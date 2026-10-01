package store

import (
	"gorm.io/gorm"
	"log"
	"webhook-tester/internal/models"
	"webhook-tester/internal/repository"
)

// Ensure GormWebhookRequestRepo implements repository.WebhookRequestRepository
var _ repository.WebhookRequestRepository = &GormWebhookRequestRepo{}

// GormWebhookRequestRepo is a GORM implementation of WebhookRequestRepository.
type GormWebhookRequestRepo struct {
	DB     *gorm.DB
	logger *log.Logger
}

// NewGormWebhookRequestRepo constructs a new repository with a logger.
func NewGormWebhookRequestRepo(db *gorm.DB, logger *log.Logger) *GormWebhookRequestRepo {
	return &GormWebhookRequestRepo{DB: db, logger: logger}
}

func (r *GormWebhookRequestRepo) Insert(req *models.WebhookRequest) error {
	if err := r.DB.Create(req).Error; err != nil {
		r.logger.Printf("insert request failed: %v", err)
		return err
	}
	return nil
}

func (r *GormWebhookRequestRepo) GetByID(id string) (*models.WebhookRequest, error) {
	var wr models.WebhookRequest
	err := r.DB.Preload("Deliveries", func(db *gorm.DB) *gorm.DB {
		return db.Order(newestDeliveriesFirst)
	}).First(&wr, "id = ?", id).Error
	if err != nil {
		r.logger.Printf("get request %s failed: %v", id, err)
		return nil, err
	}
	return &wr, nil
}

func (r *GormWebhookRequestRepo) ListByWebhook(webhookID string) ([]models.WebhookRequest, error) {
	var list []models.WebhookRequest
	if err := r.DB.
		Where("webhook_id = ?", webhookID).
		Order("received_at DESC").
		Find(&list).Error; err != nil {
		r.logger.Printf("list requests for %s failed: %v", webhookID, err)
		return nil, err
	}
	return list, nil
}

// DeleteByID removes one request together with its deliveries.
func (r *GormWebhookRequestRepo) DeleteByID(id string) error {
	err := r.DB.Transaction(func(tx *gorm.DB) error {
		if err := deleteDeliveriesByRequest(tx, id); err != nil {
			return err
		}
		return tx.Delete(&models.WebhookRequest{}, "id = ?", id).Error
	})
	if err != nil {
		r.logger.Printf("delete request %s failed: %v", id, err)
		return err
	}
	return nil
}

// DeleteByWebhook removes all requests for a webhook together with their
// deliveries.
func (r *GormWebhookRequestRepo) DeleteByWebhook(webhookID string) error {
	err := r.DB.Transaction(func(tx *gorm.DB) error {
		if err := deleteDeliveriesByWebhooks(tx, webhookID); err != nil {
			return err
		}
		return tx.Where("webhook_id = ?", webhookID).Delete(&models.WebhookRequest{}).Error
	})
	if err != nil {
		r.logger.Printf("delete all requests for %s failed: %v", webhookID, err)
		return err
	}
	return nil
}
