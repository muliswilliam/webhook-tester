package repository

import "webhook-tester/internal/models"

type UserRepository interface {
	// Create Creates a new user record
	Create(user *models.User) error
	// GetByID Finds a user by email
	GetByID(id uint) (*models.User, error)
	// GetByEmail Get a user by email
	GetByEmail(email string) (*models.User, error)
	// GetByResetToken looks up a user whose ResetToken matches the given string.
	GetByResetToken(token string) (*models.User, error)
	// Update existing users
	Update(user *models.User) error
}
