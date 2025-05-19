package store

import (
	"errors"
	"gorm.io/gorm"
	"log"
	"time"
	"webhook-tester/internal/models"
	"webhook-tester/internal/repository"
)

// Ensure GormWebhookRepo implements repository.WebhookRepository
var _ repository.WebhookRepository = &GormWebhookRepo{}

type GormWebhookRepo struct {
	DB     *gorm.DB
	Logger *log.Logger
}

func NewGormWebookRepo(db *gorm.DB, l *log.Logger) *GormWebhookRepo {
	return &GormWebhookRepo{DB: db, Logger: l}
}

func (r GormWebhookRepo) Insert(webhook *models.Webhook) error {
	err := r.DB.Create(&webhook).Error
	if err != nil {
		r.Logger.Printf("failed to create webhook: %v", err)
	}
	return err
}

func (r GormWebhookRepo) Get(id string) (*models.Webhook, error) {
	var w models.Webhook
	err := r.DB.First(&w, "id = ?", id).Error
	if err != nil {
		r.Logger.Printf("failed to get webhook: %v", err)
	}
	return &w, err
}

func (r GormWebhookRepo) InsertRequest(w *models.WebhookRequest) error {
	return r.DB.Create(&w).Error
}

func (r GormWebhookRepo) GetByUser(id string, userID uint) (*models.Webhook, error) {
	var w models.Webhook
	err := r.DB.First(&w, "id = ? AND user_id = ?", id, userID).Error
	if err != nil {
		r.Logger.Printf("failed to get webhook: %v", err)
	}
	return &w, err
}

func (r GormWebhookRepo) GetAll() ([]models.Webhook, error) {
	var webhooks []models.Webhook
	err := r.DB.Model(&models.Webhook{}).Preload("Requests").Find(&webhooks).Error
	if err != nil {
		r.Logger.Printf("failed to get webhooks: %v", err)
	}
	return webhooks, err
}

func (r GormWebhookRepo) GetAllByUser(userID uint) ([]models.Webhook, error) {
	var webhooks []models.Webhook
	err := r.DB.Preload("Requests", func(db *gorm.DB) *gorm.DB {
		return db.Order("received_at DESC").Limit(1000)
	}).
		Where("user_id = ?", userID).Find(&webhooks).
		Order("created_at DESC").Error

	if err != nil {
		r.Logger.Printf("Error loading user webhooks: %v", err)
		return webhooks, err
	}
	return webhooks, nil
}

func (r GormWebhookRepo) Update(webhook *models.Webhook) error {
	err := r.DB.Save(&webhook).Error
	if err != nil {
		r.Logger.Printf("failed to update webhook: %v", err)
	}
	return err
}

func (r GormWebhookRepo) Delete(id string, userID uint) error {
	return r.DB.Transaction(func(tx *gorm.DB) error {
		// Check if webhook exists and belongs to user
		var wh models.Webhook
		err := tx.First(&wh, "id = ? AND user_id = ?", id, userID).Error
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				r.Logger.Printf("webhook not found or unauthorized: id=%s user_id=%d", id, userID)
			} else {
				r.Logger.Printf("error checking webhook ownership: %v", err)
			}
			return err
		}

		// Delete webhook requests
		if err := tx.Delete(&models.WebhookRequest{}, "webhook_id = ?", id).Error; err != nil {
			r.Logger.Printf("failed to delete webhook requests: %v", err)
			return err
		}

		// Delete webhook
		if err := tx.Delete(&models.Webhook{}, "id = ?", id).Error; err != nil {
			r.Logger.Printf("failed to delete webhook: %v", err)
			return err
		}

		return nil
	})
}

func (r GormWebhookRepo) GetWithRequests(id string) (*models.Webhook, error) {
	var webhook models.Webhook
	err := r.DB.Preload("Requests", func(db *gorm.DB) *gorm.DB {
		return db.Order("received_at DESC")
	}).First(&webhook, "id = ?", id).Error
	return &webhook, err
}

// CleanPublic deletes anonymous (public) webhooks and their associated requests
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
func (r GormWebhookRepo) CleanPublic(d time.Duration) error {
	r.Logger.Println("Cleaning public webhooks")
	beforeDate := time.Now().Add(-d).UTC()

	err := r.DB.Transaction(func(tx *gorm.DB) error {
		var webhooks []models.Webhook
		tx.Where("created_at > ? AND user_id = 0", beforeDate).Find(&webhooks)

		var webhookIDs []string
		for _, webhook := range webhooks {
			webhookIDs = append(webhookIDs, webhook.ID)
		}

		// delete requests
		err := tx.Where("webhook_id IN (?)", webhookIDs).Delete(&models.WebhookRequest{}).Error
		if err != nil {
			r.Logger.Printf("Error deleting webhooks: %v", err)
			return err
		}

		return nil
	})

	if err != nil {
		r.Logger.Printf("error cleaning public webhooks: %v", err)
	}

	return err
}
