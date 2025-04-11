package sqlstore

import (
	"log"
	"webhook-tester/internal/db"
	"webhook-tester/internal/models"
)

func InsertWebhook(w models.Webhook) error {
	result := db.DB.Create(&w)
	if result.Error != nil {
		log.Printf("failed to create webhook: %v", result.Error)
	}
	return result.Error
}

func GetWebhook(id string) (models.Webhook, error) {
	var w models.Webhook
	err := db.DB.First(&w, "id = ?", id).Error
	if err != nil {
		log.Printf("failed to get webhook: %v", err)
	}
	return w, err
}

func GetAllWebhooks() ([]models.Webhook, error) {
	var webhooks []models.Webhook
	err := db.DB.Model(&models.Webhook{}).Preload("Requests").Find(&webhooks).Error
	if err != nil {
		log.Printf("failed to get webhooks: %v", err)
	}
	return webhooks, err
}

func UpdateWebhook(w models.Webhook) error {
	err := db.DB.Save(&w).Error
	if err != nil {
		log.Printf("failed to update webhook: %v", err)
	}
	return err
}

func DeleteWebhook(id string) error {
	err := db.DB.Delete(&models.Webhook{}, "id = ?", id).Error
	if err != nil {
		log.Printf("failed to delete webhook: %v", err)
	}
	return err
}
