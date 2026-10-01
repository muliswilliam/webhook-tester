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
	logger *log.Logger
}

func NewGormWebookRepo(db *gorm.DB, l *log.Logger) *GormWebhookRepo {
	return &GormWebhookRepo{DB: db, logger: l}
}

func (r GormWebhookRepo) Insert(webhook *models.Webhook) error {
	err := r.DB.Create(&webhook).Error
	if err != nil {
		r.logger.Printf("failed to create webhook: %v", err)
	}
	return err
}

func (r GormWebhookRepo) Get(id string) (*models.Webhook, error) {
	var w models.Webhook
	err := r.DB.First(&w, "id = ?", id).Error
	if err != nil {
		r.logger.Printf("failed to get webhook: %v", err)
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
		r.logger.Printf("failed to get webhook: %v", err)
	}
	return &w, err
}

func (r GormWebhookRepo) GetAll() ([]models.Webhook, error) {
	var webhooks []models.Webhook
	err := r.DB.Model(&models.Webhook{}).Preload("Requests").Find(&webhooks).Error
	if err != nil {
		r.logger.Printf("failed to get webhooks: %v", err)
	}
	return webhooks, err
}

// requestsPerWebhookLimit caps how many of each webhook's newest requests
// GetAllByUser loads.
const requestsPerWebhookLimit = 1000

func (r GormWebhookRepo) GetAllByUser(userID uint) ([]models.Webhook, error) {
	var webhooks []models.Webhook
	userWebhookIDs := r.DB.Model(&models.Webhook{}).Select("id").Where("user_id = ?", userID)
	ranked := r.DB.Model(&models.WebhookRequest{}).
		Select("id, ROW_NUMBER() OVER (PARTITION BY webhook_id ORDER BY received_at DESC, id DESC) AS rn").
		Where("webhook_id IN (?)", userWebhookIDs)
	newestIDs := r.DB.Table("(?) AS ranked", ranked).Select("id").Where("rn <= ?", requestsPerWebhookLimit)

	err := r.DB.Preload("Requests", func(db *gorm.DB) *gorm.DB {
		return db.Where("id IN (?)", newestIDs).Order("received_at DESC")
	}).
		Where("user_id = ?", userID).
		Order("created_at DESC").
		Find(&webhooks).Error

	if err != nil {
		r.logger.Printf("Error loading user webhooks: %v", err)
		return webhooks, err
	}

	lists := make([][]models.WebhookRequest, len(webhooks))
	for i, wh := range webhooks {
		lists[i] = wh.Requests
	}
	if err := attachDeliveries(r.DB, lists...); err != nil {
		r.logger.Printf("Error loading user webhooks' deliveries: %v", err)
		return webhooks, err
	}
	return webhooks, nil
}

func (r GormWebhookRepo) Update(webhook *models.Webhook) error {
	err := r.DB.Save(&webhook).Error
	if err != nil {
		r.logger.Printf("failed to update webhook: %v", err)
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
				r.logger.Printf("webhook not found or unauthorized: id=%s user_id=%d", id, userID)
			} else {
				r.logger.Printf("error checking webhook ownership: %v", err)
			}
			return err
		}

		// Delete the requests' deliveries, then the requests
		if err := deleteDeliveriesByWebhooks(tx, id); err != nil {
			r.logger.Printf("failed to delete webhook deliveries: %v", err)
			return err
		}
		if err := tx.Delete(&models.WebhookRequest{}, "webhook_id = ?", id).Error; err != nil {
			r.logger.Printf("failed to delete webhook requests: %v", err)
			return err
		}

		// Delete webhook
		if err := tx.Delete(&models.Webhook{}, "id = ?", id).Error; err != nil {
			r.logger.Printf("failed to delete webhook: %v", err)
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
	if err != nil {
		return &webhook, err
	}
	if err := attachDeliveries(r.DB, webhook.Requests); err != nil {
		r.logger.Printf("failed to load deliveries of webhook %s: %v", id, err)
		return &webhook, err
	}
	return &webhook, nil
}

// CountRequests returns the number of requests captured for a webhook.
func (r GormWebhookRepo) CountRequests(webhookID string) (int64, error) {
	var count int64
	err := r.DB.Model(&models.WebhookRequest{}).Where("webhook_id = ?", webhookID).Count(&count).Error
	if err != nil {
		r.logger.Printf("failed to count webhook requests: %v", err)
	}
	return count, err
}

// GetRequestsAfter returns webhookID's requests positioned after the cursor,
// oldest first.
func (r GormWebhookRepo) GetRequestsAfter(webhookID string, after models.RequestCursor) ([]models.WebhookRequest, error) {
	var requests []models.WebhookRequest
	q := r.DB.Where("webhook_id = ?", webhookID)
	if !after.IsZero() {
		q = r.DB.Where("webhook_id = ? AND (received_at > ? OR (received_at = ? AND id > ?))",
			webhookID, after.ReceivedAt, after.ReceivedAt, after.ID)
	}
	if err := q.Order("received_at ASC, id ASC").Find(&requests).Error; err != nil {
		r.logger.Printf("failed to get webhook requests after cursor: %v", err)
		return requests, err
	}
	if err := attachDeliveries(r.DB, requests); err != nil {
		r.logger.Printf("failed to get deliveries of webhook requests after cursor: %v", err)
		return requests, err
	}
	return requests, nil
}

// CleanPublic deletes anonymous (public) webhooks, i.e. those with user_id = 0,
// created more than d ago, together with their requests and deliveries, in
// one transaction.
// It returns the IDs of the deleted webhooks.
func (r GormWebhookRepo) CleanPublic(d time.Duration) ([]string, error) {
	r.logger.Println("Cleaning public webhooks")
	beforeDate := time.Now().Add(-d).UTC()

	var webhookIDs []string
	err := r.DB.Transaction(func(tx *gorm.DB) error {
		var webhooks []models.Webhook
		if err := tx.Where("created_at < ? AND user_id = 0", beforeDate).Find(&webhooks).Error; err != nil {
			return err
		}
		if len(webhooks) == 0 {
			return nil
		}

		for _, webhook := range webhooks {
			webhookIDs = append(webhookIDs, webhook.ID)
		}

		// delete the requests' deliveries, then the requests
		if err := deleteDeliveriesByWebhooks(tx, webhookIDs...); err != nil {
			r.logger.Printf("Error deleting webhook deliveries: %v", err)
			return err
		}
		if err := tx.Where("webhook_id IN (?)", webhookIDs).Delete(&models.WebhookRequest{}).Error; err != nil {
			r.logger.Printf("Error deleting webhook requests: %v", err)
			return err
		}

		// delete webhooks
		if err := tx.Where("id IN (?)", webhookIDs).Delete(&models.Webhook{}).Error; err != nil {
			r.logger.Printf("Error deleting webhooks: %v", err)
			return err
		}

		return nil
	})

	if err != nil {
		r.logger.Printf("error cleaning public webhooks: %v", err)
		return nil, err
	}

	return webhookIDs, nil
}

// AssignOwner gives the public webhook id to userID.
func (r GormWebhookRepo) AssignOwner(id string, userID uint) error {
	res := r.DB.Model(&models.Webhook{}).Where("id = ? AND user_id = 0", id).Update("user_id", userID)
	if res.Error != nil {
		r.logger.Printf("failed to assign webhook %s to user %d: %v", id, userID, res.Error)
		return res.Error
	}
	if res.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}
