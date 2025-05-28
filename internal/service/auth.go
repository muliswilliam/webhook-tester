package service

import (
	"errors"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"time"
	"webhook-tester/internal/models"
	"webhook-tester/internal/repository"
	"webhook-tester/internal/utils"

	"github.com/gorilla/sessions"
)

const (
	SessionName       = "_webhook_tester_session_id"
	GuestSessionName  = "_webhook_tester_guest_session_id"
	UserIDKey         = "user_id"
	GuestWebhookIDKey = "webhook_id"
)

type AuthService interface {
	Register(email, plainPassword, fullName string) (*models.User, error)
	Authenticate(email, plainPassword string) (*models.User, error)
	Authorize(r *http.Request) (uint, error)
	GetCurrentUser(r *http.Request) (*models.User, error)
	CreateSession(w http.ResponseWriter, r *http.Request, userID uint) error
	ClearSession(w http.ResponseWriter, r *http.Request, name string)
	ForgotPassword(email, domain string) (string, error)
	ValidateResetToken(token string) (*models.User, error)
	ResetPassword(token, newPassword string) error
	ValidateAPIKey(key string) (*models.User, error)
	CreateGuestSession(r *http.Request, w http.ResponseWriter, webhookID string) error
	GetGuestSession(r *http.Request) (string, error)
}

// AuthService holds user business logic
type authService struct {
	repo              repository.UserRepository
	passwordHasher    utils.PasswordHasher
	passwordValidator utils.PasswordValidator
	sessionStore      SessionStore
}

// NewAuthService creates an AuthService
func NewAuthService(
	userRepo repository.UserRepository,
	sessionStore SessionStore,
	passwordHasher utils.PasswordHasher,
	passwordValidator utils.PasswordValidator,
) AuthService {
	return &authService{
		repo:              userRepo,
		passwordHasher:    passwordHasher,
		passwordValidator: passwordValidator,
		sessionStore:      sessionStore,
	}
}

// Register creates a new user with hashed password
func (s *authService) Register(email, plainPassword, fullName string) (*models.User, error) {
	if _, err := s.repo.GetByEmail(email); err != nil {
		return nil, fmt.Errorf("email already taken")
	}

	rules := utils.PasswordRules{
		MinLength:        8,
		RequireLowercase: true,
		RequireUppercase: true,
		RequireNumber:    true,
	}

	err := s.passwordValidator.Validate(plainPassword, rules)
	fmt.Println("validate error", err)
	if err != nil {
		return nil, err
	}

	hash, err := s.passwordHasher.HashPassword(plainPassword)
	if err != nil {
		return nil, err
	}
	key, err := utils.GenerateAPIKey("user_", 32)
	if err != nil {
		return nil, err
	}
	user := &models.User{FullName: fullName, Email: email, Password: hash, APIKey: key}
	if err := s.repo.Create(user); err != nil {
		return nil, err
	}
	return user, nil
}

// Authenticate verifies credentials
func (s *authService) Authenticate(email, plainPassword string) (*models.User, error) {
	user, err := s.repo.GetByEmail(email)
	if err != nil {
		return nil, errors.New("invalid credentials")
	}
	if !s.passwordHasher.CheckPasswordHash(plainPassword, user.Password) {
		return nil, errors.New("invalid credentials")
	}
	return user, nil
}

// Authorize extracts and validates the user_id from the session cookie.
func (s *authService) Authorize(r *http.Request) (uint, error) {
	authErr := errors.New("unauthorized")
	uid, err := s.sessionStore.GetValue(r, SessionName, UserIDKey)
	if err != nil {
		return 0, authErr
	}
	userId, err := strconv.ParseUint(uid.(string), 10, 64)
	if err != nil {
		return 0, errors.New("uid is invalid")
	}
	return uint(userId), nil
}

// GetCurrentUser pulls the session and looks up the full User record.
func (s *authService) GetCurrentUser(r *http.Request) (*models.User, error) {
	userID, err := s.Authorize(r)
	if err != nil {
		return nil, err
	}
	return s.repo.GetByID(userID)
}

// CreateSession establishes a new session cookie for the given user.
func (s *authService) CreateSession(w http.ResponseWriter, r *http.Request, userID uint) error {
	_, err := s.sessionStore.New(r, w, SessionName, UserIDKey, userID, sessions.Options{
		MaxAge:   86400 * 2,
		HttpOnly: true,
		Secure:   os.Getenv("ENV") == "prod",
	})

	return err
}

// ClearSession invalidates the current session cookie.
func (s *authService) ClearSession(w http.ResponseWriter, r *http.Request, name string) {
	_ = s.sessionStore.Delete(r, w, name)
}

// ForgotPassword generates a reset token, sets expiry, and returns the token
func (s *authService) ForgotPassword(email, domain string) (string, error) {
	user, err := s.repo.GetByEmail(email)
	if err != nil {
		return "", fmt.Errorf("user not found")
	}
	// Generate secure token
	token, err := utils.GenerateSecureToken(32)
	if err != nil {
		return "", err
	}
	user.ResetToken = token
	user.ResetTokenExpiry = time.Now().Add(24 * time.Hour)
	if err := s.repo.Update(user); err != nil {
		return "", err
	}
	return fmt.Sprintf("%s/reset-password?token=%s", domain, token), nil
}

// ValidateResetToken looks up the user by token and ensures it hasn't expired.
func (s *authService) ValidateResetToken(token string) (*models.User, error) {
	user, err := s.repo.GetByResetToken(token)
	if err != nil {
		return nil, fmt.Errorf("invalid or expired token")
	}
	if time.Now().After(user.ResetTokenExpiry) {
		return nil, fmt.Errorf("invalid or expired token")
	}
	return user, nil
}

// ResetPassword validates the token, enforces password rules, hashes,
// and then persists the new password.
func (s *authService) ResetPassword(token, newPassword string) error {
	user, err := s.repo.GetByResetToken(token)
	if err != nil {
		return fmt.Errorf("invalid or expired reset link")
	}
	if time.Now().After(user.ResetTokenExpiry) {
		return fmt.Errorf("invalid or expired reset link")
	}

	rules := utils.PasswordRules{
		MinLength:        8,
		RequireLowercase: true,
		RequireUppercase: true,
		RequireNumber:    true,
	}
	if err := s.passwordValidator.Validate(newPassword, rules); err != nil {
		return err
	}

	hash, err := s.passwordHasher.HashPassword(newPassword)
	if err != nil {
		return err
	}
	user.Password = hash
	user.ResetToken = ""
	user.ResetTokenExpiry = time.Time{}

	return s.repo.Update(user)
}

func (s *authService) ValidateAPIKey(key string) (*models.User, error) {
	user, err := s.repo.GetByAPIKey(key)
	if err != nil {
		return nil, fmt.Errorf("invalid API key")
	}
	return user, nil
}

func (s *authService) CreateGuestSession(r *http.Request, w http.ResponseWriter, webhookID string) error {
	cookie := &http.Cookie{
		Name:     GuestSessionName,
		Value:    webhookID,
		Path:     "/",
		HttpOnly: true,
		Secure:   os.Getenv("ENV") == "prod",
		MaxAge:   86400 * 2, // 2 days
	}
	http.SetCookie(w, cookie)
	return nil
}

func (s *authService) GetGuestSession(r *http.Request) (string, error) {
	cookie, err := r.Cookie(GuestSessionName)
	if err != nil {
		return "", err
	}

	return cookie.Value, nil
}
