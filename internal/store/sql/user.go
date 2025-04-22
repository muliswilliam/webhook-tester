package sqlstore

import (
	"gorm.io/gorm"
	"log"
	"webhook-tester/internal/models"
)

func InsertUser(db *gorm.DB, user *models.User) error {
	err := db.Create(user).Error
	if err != nil {
		log.Printf("Error creating user: %v", err)
	}

	return err
}
