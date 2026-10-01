package repository

import "webhook-tester/internal/models"

type WebhookRequestRepository interface {
	// Insert a new request record
	Insert(req *models.WebhookRequest) error
	// GetByID retrieves one request by its ID, with its deliveries newest first
	GetByID(id string) (*models.WebhookRequest, error)
	// ListByWebhook returns all requests for a given webhook
	ListByWebhook(webhookID string) ([]models.WebhookRequest, error)
	// DeleteByID removes one request and its deliveries
	DeleteByID(id string) error
	// DeleteByWebhook removes all requests for a webhook and their deliveries
	DeleteByWebhook(webhookID string) error
}
