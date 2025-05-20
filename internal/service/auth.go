package service

import (
	"errors"
	"fmt"
	"github.com/wader/gormstore/v2"
	"gorm.io/gorm"
	"net/http"
	"os"
	"time"
	"webhook-tester/internal/models"
	"webhook-tester/internal/repository"
	"webhook-tester/internal/utils"
)

// AuthService holds user business logic
type AuthService struct {
	repo         repository.UserRepository
	sessionStore *gormstore.Store
	db           *gorm.DB
}

// NewAuthService creates an AuthService
func NewAuthService(userRepo repository.UserRepository, db *gorm.DB, authSecret string) *AuthService {
	// build the GORM‐backed session store
	store := gormstore.New(db, []byte(authSecret))
	quit := make(chan struct{})
	go store.PeriodicCleanup(48*time.Hour, quit)

	return &AuthService{
		repo:         userRepo,
		sessionStore: store,
		db:           db,
	}
}

// Register creates a new user with hashed password
func (s *AuthService) Register(email, plainPassword, fullName string) (*models.User, error) {
	if _, err := s.repo.GetByEmail(email); err == nil {
		return nil, fmt.Errorf("email already taken")
	}
	hash, err := utils.HashPassword(plainPassword)
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
func (s *AuthService) Authenticate(email, plainPassword string) (*models.User, error) {
	user, err := s.repo.GetByEmail(email)
	if err != nil {
		return nil, errors.New("invalid credentials")
	}
	if !utils.CheckPasswordHash(plainPassword, user.Password) {
		return nil, errors.New("invalid credentials")
	}
	return user, nil
}

// Authorize extracts and validates the user_id from the session cookie.
func (s *AuthService) Authorize(r *http.Request) (uint, error) {
	const Name = "_webhook_tester_session_id"
	authErr := errors.New("unauthorized")
	sess, err := s.sessionStore.Get(r, Name)
	if err != nil {
		return 0, authErr
	}
	raw, ok := sess.Values["user_id"]
	uid, ok2 := raw.(uint)
	if !ok || !ok2 {
		return 0, authErr
	}
	return uid, nil
}

// GetCurrentUser pulls the session and looks up the full User record.
func (s *AuthService) GetCurrentUser(r *http.Request) (*models.User, error) {
	userID, err := s.Authorize(r)
	if err != nil {
		return nil, err
	}
	return s.repo.GetByID(userID)
}

// CreateSession establishes a new session cookie for the given user.
func (s *AuthService) CreateSession(w http.ResponseWriter, r *http.Request, user *models.User) error {
	const Name = "_webhook_tester_session_id"
	sess, err := s.sessionStore.Get(r, Name)
	if err != nil {
		// if there was no existing session, we still want a brand‐new one
		sess, _ = s.sessionStore.New(r, Name)
	}
	sess.Values["user_id"] = user.ID
	sess.Options.MaxAge = 86400 * 2 // two days
	sess.Options.HttpOnly = true
	sess.Options.Secure = os.Getenv("ENV") == "prod"
	return s.sessionStore.Save(r, w, sess)
}

// ClearSession invalidates the current session cookie.
func (s *AuthService) ClearSession(w http.ResponseWriter, r *http.Request) {
	const Name = "_webhook_tester_session_id"
	if sess, err := s.sessionStore.Get(r, Name); err == nil {
		sess.Options.MaxAge = -1
		_ = s.sessionStore.Save(r, w, sess)
	}
}

// ForgotPassword generates a reset token, sets expiry, and returns the token
func (s *AuthService) ForgotPassword(email, domain string) (string, error) {
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
func (s *AuthService) ValidateResetToken(token string) (*models.User, error) {
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
func (s *AuthService) ResetPassword(token, newPassword string) error {
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
	if err := utils.ValidatePassword(newPassword, rules); err != nil {
		return err
	}

	hash, err := utils.HashPassword(newPassword)
	if err != nil {
		return err
	}
	user.Password = hash
	user.ResetToken = ""
	user.ResetTokenExpiry = time.Time{}

	return s.repo.Update(user)
}
