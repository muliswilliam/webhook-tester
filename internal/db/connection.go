package db

import (
	"fmt"
	"log"
	"os"
	"webhook-tester/internal/models"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func Connect() *gorm.DB {
	user := os.Getenv("POSTGRES_USER")
	pass := os.Getenv("POSTGRES_PASSWORD")
	name := os.Getenv("POSTGRES_DB")
	host := os.Getenv("DB_HOST")
	port := os.Getenv("DB_PORT")

	if port == "" {
		port = "5432"
	}

	if user == "" || name == "" || host == "" {
		log.Fatal("Database credentials are not fully set in environment variables")
	}

	dsn := fmt.Sprintf("postgresql://%s:%s@%s:%s/%s", user, pass, host, port, name)

	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
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
