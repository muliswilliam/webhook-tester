package sqlstore

import (
	"errors"
	"log"
	"time"
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
	err := db.First(&w, "id = ?").Error
	if err != nil {
		log.Printf("failed to get webhook: %v", err)
	}
	return w, err
}

func GetUserWebhook(db *gorm.DB, id string, userID uint) (models.Webhook, error) {
	var w models.Webhook
	err := db.First(&w, "id = ? AND user_id = ?", id, userID).Error
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

func DeleteUserWebhook(db *gorm.DB, id string, userID uint) error {
	return db.Transaction(func(tx *gorm.DB) error {
		// Check if webhook exists and belongs to user
		var wh models.Webhook
		err := tx.First(&wh, "id = ? AND user_id = ?", id, userID).Error
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				log.Printf("webhook not found or unauthorized: id=%s user_id=%d", id, userID)
			} else {
				log.Printf("error checking webhook ownership: %v", err)
			}
			return err
		}

		// Delete webhook requests
		if err := tx.Delete(&models.WebhookRequest{}, "webhook_id = ?", id).Error; err != nil {
			log.Printf("failed to delete webhook requests: %v", err)
			return err
		}

		// Delete webhook
		if err := tx.Delete(&models.Webhook{}, "id = ?", id).Error; err != nil {
			log.Printf("failed to delete webhook: %v", err)
			return err
		}

		return nil
	})
}

func GetWebhookWithRequests(id string, db *gorm.DB) (models.Webhook, error) {
	var webhook models.Webhook
	err := db.Preload("Requests", func(db *gorm.DB) *gorm.DB {
		return db.Order("received_at DESC")
	}).First(&webhook, "id = ?", id).Error
	return webhook, err
}

func GetUserWebhooks(userID uint, db *gorm.DB) ([]models.Webhook, error) {
	var webhooks []models.Webhook
	err := db.Preload("Requests", func(db *gorm.DB) *gorm.DB {
		return db.Order("received_at DESC").Limit(1000)
	}).
		Where("user_id = ?", userID).Find(&webhooks).
		Order("created_at DESC").Error

	if err != nil {
		log.Printf("Error loading user webhooks: %v", err)
		return webhooks, err
	}
	return webhooks, nil
}

// CleanPublicWebhooks deletes anonymous (public) webhooks and their associated requests
// that were created before a specified duration threshold.
//
// A webhook is considered public if it has no associated user (i.e., user_id = 0).
// This function queries for all such webhooks created earlier than the current time minus `d`,
// then deletes both the webhooks and their related webhook requests within a single transaction.
//
// Parameters:
//   - db: a *gorm.DB database connection.
//   - d: a time.Duration representing the age threshold (e.g., 72*time.Hour).
//
// This function is useful for cleaning up stale, guest-generated webhooks
// that should not persist indefinitely.
//
// Any error during the transaction is logged but not returned.
func CleanPublicWebhooks(db *gorm.DB, d time.Duration) {
	log.Println("Cleaning public webhooks")
	beforeDate := time.Now().Add(-d).UTC()

	err := db.Transaction(func(tx *gorm.DB) error {
		var webhooks []models.Webhook
		tx.Where("created_at > ? AND user_id = 0", beforeDate).Find(&webhooks)

		var webhookIDs []string
		for _, webhook := range webhooks {
			webhookIDs = append(webhookIDs, webhook.ID)
		}

		// delete requests
		err := tx.Where("webhook_id IN (?)", webhookIDs).Delete(&models.WebhookRequest{}).Error
		if err != nil {
			log.Printf("Error deleting webhooks: %v", err)
			return err
		}

		// delete webhooks
		err = tx.Where("id IN (?)", webhookIDs).Delete(&models.Webhook{}).Error
		if err != nil {
			log.Printf("Error deleting webhooks: %v", err)
			return err
		}

		return nil
	})

	if err != nil {
		log.Printf("error cleaning public webhooks: %v", err)
	}
}
