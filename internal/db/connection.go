package db

import (
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"log"
	"webhook-tester/internal/models"
)

var DB *gorm.DB

func Connect() {
	var err error
	DB, err = gorm.Open(sqlite.Open("webhook.db"), &gorm.Config{})
	if err != nil {
		log.Fatalf("failed to connect to database: %v", err)
	}
}

func AutoMigrate() {
	err := DB.AutoMigrate(&models.Webhook{}, &models.WebhookRequest{}, &models.User{})
	if err != nil {
		log.Fatalf("failed to auto-migrate: %v", err)
	}
}
