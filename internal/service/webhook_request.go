package service

import (
	"webhook-tester/internal/models"
	"webhook-tester/internal/repository"
)

type WebhookRequestService interface {
	Record(rq *models.WebhookRequest) error
	Get(id string) (*models.WebhookRequest, error)
	List(webhookID string) ([]models.WebhookRequest, error)
	Delete(id string) error
	DeleteAll(webhookID string) error
}

// WebhookRequestService encapsulates business logic for webhook events.
type webhookRequestService struct {
	repo repository.WebhookRequestRepository
}

// NewWebhookRequestService constructs a WebhookRequestService.
func NewWebhookRequestService(repo repository.WebhookRequestRepository) WebhookRequestService {
	return &webhookRequestService{repo: repo}
}

// Record records a new webhook request event.
func (s *webhookRequestService) Record(rq *models.WebhookRequest) error {
	return s.repo.Insert(rq)
}

// Get retrieves a single request by ID.
func (s *webhookRequestService) Get(id string) (*models.WebhookRequest, error) {
	return s.repo.GetByID(id)
}

// List returns recent requests for a webhook.
func (s *webhookRequestService) List(webhookID string) ([]models.WebhookRequest, error) {
	return s.repo.ListByWebhook(webhookID)
}

// Delete removes a single request.
func (s *webhookRequestService) Delete(id string) error {
	return s.repo.DeleteByID(id)
}

// DeleteAll removes all requests for a webhook.
func (s *webhookRequestService) DeleteAll(webhookID string) error {
	return s.repo.DeleteByWebhook(webhookID)
}
