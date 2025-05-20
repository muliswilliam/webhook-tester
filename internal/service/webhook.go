package service

import (
	"net/http"
	"time"
	"webhook-tester/internal/models"
	"webhook-tester/internal/repository"
)

// WebhookService encapsulates business logic for webhooks.
type WebhookService struct {
	repo repository.WebhookRepository
}

// NewWebhookService constructs a WebhookService with the given repository.
func NewWebhookService(repo repository.WebhookRepository) *WebhookService {
	return &WebhookService{repo: repo}
}

// CreateWebhook creates a new webhook record.
func (s *WebhookService) CreateWebhook(w *models.Webhook) error {
	if w.ResponseCode == 0 {
		w.ResponseCode = http.StatusOK
	}
	return s.repo.Insert(w)
}

// GetWebhook retrieves a public webhook by ID.
func (s *WebhookService) GetWebhook(id string) (*models.Webhook, error) {
	return s.repo.Get(id)
}

// GetUserWebhook retrieves a webhook by ID for a specific user.
func (s *WebhookService) GetUserWebhook(id string, userID uint) (*models.Webhook, error) {
	return s.repo.GetByUser(id, userID)
}

// ListWebhooks lists public or user-specific webhooks.
func (s *WebhookService) ListWebhooks(userID uint) ([]models.Webhook, error) {
	if userID == 0 {
		return s.repo.GetAll()
	}
	return s.repo.GetAllByUser(userID)
}

// UpdateWebhook updates an existing webhook.
func (s *WebhookService) UpdateWebhook(w *models.Webhook) error {
	return s.repo.Update(w)
}

func (s *WebhookService) CreateRequest(wr *models.WebhookRequest) error {
	return s.repo.InsertRequest(wr)
}

// DeleteWebhook deletes a webhook and its requests.
func (s *WebhookService) DeleteWebhook(id string, userID uint) error {
	return s.repo.Delete(id, userID)
}

// GetWebhookWithRequests fetches a webhook along with its requests.
func (s *WebhookService) GetWebhookWithRequests(id string) (*models.Webhook, error) {
	return s.repo.GetWithRequests(id)
}

// CleanPublicWebhooks cleans up old public webhooks.
func (s *WebhookService) CleanPublicWebhooks(d time.Duration) error {
	return s.repo.CleanPublic(d)
}
