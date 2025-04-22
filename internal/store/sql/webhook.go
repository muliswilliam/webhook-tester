package sqlstore

import (
	"log"
	"webhook-tester/internal/models"

	"gorm.io/gorm"
)

func InsertWebhook(db *gorm.DB, w models.Webhook) error {
	result := db.Create(&w)
	if result.Error != nil {
		log.Printf("failed to create webhook: %v", result.Error)
	}
	return result.Error
}

func GetWebhook(db *gorm.DB, id string) (models.Webhook, error) {
	var w models.Webhook
	err := db.First(&w, "id = ?", id).Error
	if err != nil {
		log.Printf("failed to get webhook: %v", err)
	}
	return w, err
}

func GetAllWebhooks(db *gorm.DB) ([]models.Webhook, error) {
	var webhooks []models.Webhook
	err := db.Model(&models.Webhook{}).Preload("Requests").Find(&webhooks).Error
	if err != nil {
		log.Printf("failed to get webhooks: %v", err)
	}
	return webhooks, err
}

func UpdateWebhook(db *gorm.DB, w models.Webhook) error {
	err := db.Save(&w).Error
	if err != nil {
		log.Printf("failed to update webhook: %v", err)
	}
	return err
}

func DeleteWebhook(db *gorm.DB, id string) error {
	err := db.Delete(&models.Webhook{}, "id = ?", id).Error
	if err != nil {
		log.Printf("failed to delete webhook: %v", err)
	}
	return err
}
