package db

import (
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"log"
	"os"
	"webhook-tester/internal/models"
)

func Connect() *gorm.DB {
	url := os.Getenv("DATABASE_URL")
	if url == "" {
		log.Fatal("DATABASE_URL environment variable not set")
	}
	db, err := gorm.Open(postgres.Open(url), &gorm.Config{})
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
