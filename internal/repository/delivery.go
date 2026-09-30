package repository

import "webhook-tester/internal/models"

// DeliveryRepository defines data access behavior for deliveries.
type DeliveryRepository interface {
	// Insert a new delivery record
	Insert(d *models.Delivery) error
	// ListByRequest returns a captured request's deliveries, newest first
	ListByRequest(requestID string) ([]models.Delivery, error)
	// DeleteByRequest removes all deliveries of a captured request
	DeleteByRequest(requestID string) error
	// DeleteByWebhook removes all deliveries of a webhook's captured requests
	DeleteByWebhook(webhookID string) error
}
