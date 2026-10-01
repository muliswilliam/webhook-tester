package repository

import "webhook-tester/internal/models"

// DeliveryRepository defines data access behavior for deliveries. They are
// deleted along with their captured requests, by WebhookRequestRepository
// and WebhookRepository.
type DeliveryRepository interface {
	// Insert a new delivery record
	Insert(d *models.Delivery) error
	// ListByRequest returns a captured request's deliveries, newest first
	ListByRequest(requestID string) ([]models.Delivery, error)
}
