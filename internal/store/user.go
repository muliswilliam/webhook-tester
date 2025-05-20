package store

import (
	"errors"
	"gorm.io/gorm"
	"log"
	"webhook-tester/internal/models"
	"webhook-tester/internal/repository"
)

var _ repository.UserRepository = &GormUserRepo{}

type GormUserRepo struct {
	DB     *gorm.DB
	logger *log.Logger
}

func NewGormUserRepo(db *gorm.DB, l *log.Logger) *GormUserRepo {
	return &GormUserRepo{DB: db, logger: l}
}

func (r *GormUserRepo) Create(user *models.User) error {
	if err := r.DB.Create(user).Error; err != nil {
		r.logger.Printf("failed to create user: %v", err)
		return err
	}
	return nil
}

func (r *GormUserRepo) GetByEmail(email string) (*models.User, error) {
	var u models.User
	err := r.DB.First(&u, "email = ?", email).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}
	if err != nil {
		r.logger.Printf("failed to get user by email: %v", err)
	}
	return &u, err
}

func (r *GormUserRepo) GetByID(id uint) (*models.User, error) {
	var u models.User
	err := r.DB.First(&u, "id = ?", id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}
	if err != nil {
		r.logger.Printf("failed to get user by id: %v", err)
	}
	return &u, err
}

// GetByResetToken looks up a user whose ResetToken matches the given string.
func (r *GormUserRepo) GetByResetToken(token string) (*models.User, error) {
	var u models.User
	err := r.DB.First(&u, "reset_token = ?", token).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			r.logger.Printf("reset token not found: %s", token)
		} else {
			r.logger.Printf("failed to query reset token: %v", err)
		}
	}
	return &u, err
}

func (r *GormUserRepo) Update(user *models.User) error {
	if err := r.DB.Save(user).Error; err != nil {
		r.logger.Printf("failed to update user: %v", err)
		return err
	}
	return nil
}

func (r *GormUserRepo) GetByAPIKey(key string) (*models.User, error) {
	var u models.User
	err := r.DB.First(&u, "api_key = ?", key).Error
	if err != nil {
		r.logger.Printf("GetByAPIKey failed: %v", err)
	}
	return &u, err
}
