package service

import (
	"errors"
	"fmt"
	"time"
	"webhook-tester/internal/models"
	"webhook-tester/internal/repository"
	"webhook-tester/internal/utils"
)

// AuthService holds user business logic
type AuthService struct {
	repo repository.UserRepository
}

// NewAuthService creates an AuthService
func NewAuthService(repo repository.UserRepository) *AuthService {
	return &AuthService{repo: repo}
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
