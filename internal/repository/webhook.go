package repository

import (
	"time"
	"webhook-tester/internal/models"
)

// WebhookRepository defines data access behavior for webhooks.
type WebhookRepository interface {
	// Insert a new webhook
	Insert(webhook *models.Webhook) error
	// Get a webhook by ID (public)
	Get(id string) (*models.Webhook, error)
	// GetByUser Gets a webhook by ID and user (returns error if not owned)
	GetByUser(id string, userID uint) (*models.Webhook, error)
	// GetAll Retrieves all webhooks (public)
	GetAll() ([]models.Webhook, error)
	// GetAllByUser Retrieve webhooks for a specific user
	GetAllByUser(userID uint) ([]models.Webhook, error)
	// Update Updates an existing webhook
	Update(webhook *models.Webhook) error
	// InsertRequest Inserts request for a webhook
	InsertRequest(wr *models.WebhookRequest) error
	// Delete a webhook and its requests, ensuring ownership if userID > 0
	Delete(id string, userID uint) error
	// GetWithRequests Get a webhook with its requests, ordered newest first
	GetWithRequests(id string) (*models.Webhook, error)
	// CleanPublic Clean up public webhooks older than duration d
	CleanPublic(d time.Duration) error
}
