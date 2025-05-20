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
	if err := r.DB.First(&wr, "id = ?", id).Error; err != nil {
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

func (r *GormWebhookRequestRepo) DeleteByID(id string) error {
	if err := r.DB.Delete(&models.WebhookRequest{}, "id = ?", id).Error; err != nil {
		r.logger.Printf("delete request %s failed: %v", id, err)
		return err
	}
	return nil
}

func (r *GormWebhookRequestRepo) DeleteByWebhook(webhookID string) error {
	if err := r.DB.
		Where("webhook_id = ?", webhookID).
		Delete(&models.WebhookRequest{}).Error; err != nil {
		r.logger.Printf("delete all requests for %s failed: %v", webhookID, err)
		return err
	}
	return nil
}
