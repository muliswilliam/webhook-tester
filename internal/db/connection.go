package db

import (
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"log"
	"webhook-tester/internal/models"
)

func Connect() *gorm.DB {
	db, err := gorm.Open(sqlite.Open("webhook.db"), &gorm.Config{})
	if err != nil {
		log.Fatalf("failed to connect to database: %v", err)
	}
	return db
}

func AutoMigrate(db *gorm.DB) {
	err := db.AutoMigrate(&models.Webhook{}, &models.WebhookRequest{}, &models.User{})
	if err != nil {
		log.Fatalf("failed to auto-migrate: %v", err)
	}
}
