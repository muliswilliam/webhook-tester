package sqlstore

import (
	"log"
	"webhook-tester/internal/db"
	"webhook-tester/internal/models"
)

func InsertUser(user *models.User) error {
	err := db.DB.Create(user).Error
	if err != nil {
		log.Printf("Error creating user: %v", err)
	}

	return err
}
