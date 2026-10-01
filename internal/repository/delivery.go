package repository

import "webhook-tester/internal/models"

// DeliveryRepository defines data access behavior for deliveries. They are
// deleted along with their captured requests, by WebhookRequestRepository
// and WebhookRepository.
type DeliveryRepository interface {
	// Insert stores d, then deletes its request's deliveries beyond the
	// newest models.MaxDeliveriesPerRequest
	Insert(d *models.Delivery) error
	// ListByRequest returns a captured request's deliveries, newest first
	ListByRequest(requestID string) ([]models.Delivery, error)
}
