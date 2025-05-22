package service

import (
	"net/http"
	"time"
	"webhook-tester/internal/models"
	"webhook-tester/internal/repository"
)

type WebhookService interface {
	CreateWebhook(w *models.Webhook) error
	GetWebhook(id string) (*models.Webhook, error)
	GetUserWebhook(id string, userID uint) (*models.Webhook, error)
	ListWebhooks(userID uint) ([]models.Webhook, error)
	UpdateWebhook(w *models.Webhook) error
	CreateRequest(wr *models.WebhookRequest) error
	DeleteWebhook(id string, userID uint) error
	GetWebhookWithRequests(id string) (*models.Webhook, error)
	CleanPublicWebhooks(d time.Duration) error
}

var _ WebhookService = (*webhookService)(nil)

// WebhookService encapsulates business logic for webhooks.
type webhookService struct {
	repo repository.WebhookRepository
}

// NewWebhookService constructs a WebhookService with the given repository.
func NewWebhookService(repo repository.WebhookRepository) WebhookService {
	return &webhookService{repo: repo}
}

// CreateWebhook creates a new webhook record.
func (s *webhookService) CreateWebhook(w *models.Webhook) error {
	if w.ResponseCode == 0 {
		w.ResponseCode = http.StatusOK
	}
	return s.repo.Insert(w)
}

// GetWebhook retrieves a public webhook by ID.
func (s *webhookService) GetWebhook(id string) (*models.Webhook, error) {
	return s.repo.Get(id)
}

// GetUserWebhook retrieves a webhook by ID for a specific user.
func (s *webhookService) GetUserWebhook(id string, userID uint) (*models.Webhook, error) {
	return s.repo.GetByUser(id, userID)
}

// ListWebhooks lists public or user-specific webhooks.
func (s *webhookService) ListWebhooks(userID uint) ([]models.Webhook, error) {
	return s.repo.GetAllByUser(userID)
}

// UpdateWebhook updates an existing webhook.
func (s *webhookService) UpdateWebhook(w *models.Webhook) error {
	return s.repo.Update(w)
}

func (s *webhookService) CreateRequest(wr *models.WebhookRequest) error {
	return s.repo.InsertRequest(wr)
}

// DeleteWebhook deletes a webhook and its requests.
func (s *webhookService) DeleteWebhook(id string, userID uint) error {
	return s.repo.Delete(id, userID)
}

// GetWebhookWithRequests fetches a webhook along with its requests.
func (s *webhookService) GetWebhookWithRequests(id string) (*models.Webhook, error) {
	return s.repo.GetWithRequests(id)
}

// CleanPublicWebhooks cleans up old public webhooks.
func (s *webhookService) CleanPublicWebhooks(d time.Duration) error {
	return s.repo.CleanPublic(d)
}
