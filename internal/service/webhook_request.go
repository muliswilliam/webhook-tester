package service

import (
	"webhook-tester/internal/models"
	"webhook-tester/internal/repository"
)

// WebhookRequestService encapsulates business logic for webhook events.
type WebhookRequestService struct {
	repo repository.WebhookRequestRepository
}

// NewWebhookRequestService constructs a WebhookRequestService.
func NewWebhookRequestService(repo repository.WebhookRequestRepository) *WebhookRequestService {
	return &WebhookRequestService{repo: repo}
}

// Record records a new webhook request event.
func (s *WebhookRequestService) Record(rq *models.WebhookRequest) error {
	return s.repo.Insert(rq)
}

// Get retrieves a single request by ID.
func (s *WebhookRequestService) Get(id string) (*models.WebhookRequest, error) {
	return s.repo.GetByID(id)
}

// List returns recent requests for a webhook.
func (s *WebhookRequestService) List(webhookID string) ([]models.WebhookRequest, error) {
	return s.repo.ListByWebhook(webhookID)
}

// Delete removes a single request.
func (s *WebhookRequestService) Delete(id string) error {
	return s.repo.DeleteByID(id)
}

// DeleteAll removes all requests for a webhook.
func (s *WebhookRequestService) DeleteAll(webhookID string) error {
	return s.repo.DeleteByWebhook(webhookID)
}
