package service

import (
	"time"
	"webhook-tester/internal/models"
	"webhook-tester/internal/repository"

	"gorm.io/gorm"
)

// WebhookService encapsulates business logic for webhooks.
type WebhookService struct {
	repo   repository.WebhookRepository
	broker *broker
}

// NewWebhookService constructs a WebhookService with the given repository.
func NewWebhookService(repo repository.WebhookRepository) *WebhookService {
	return &WebhookService{repo: repo, broker: newBroker()}
}

// CreateWebhook creates a new webhook record.
func (s *WebhookService) CreateWebhook(w *models.Webhook) error {
	// e.g., generate ID, validate
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

// GetUserWebhookWithRequests retrieves a webhook userID owns, with its
// requests loaded newest first; gorm.ErrRecordNotFound otherwise.
func (s *WebhookService) GetUserWebhookWithRequests(id string, userID uint) (*models.Webhook, error) {
	wh, err := s.repo.GetWithRequests(id)
	if err != nil {
		return nil, err
	}
	if uint(wh.UserID) != userID {
		return nil, gorm.ErrRecordNotFound
	}
	return wh, nil
}

// ClaimGuestWebhook moves a guest's public webhook, with its requests, into
// userID's account. It returns gorm.ErrRecordNotFound if id isn't a public
// webhook (e.g. it expired, or was already claimed).
func (s *WebhookService) ClaimGuestWebhook(id string, userID uint) error {
	return s.repo.AssignOwner(id, userID)
}

// GetAccessibleWebhook retrieves a webhook that userID may view: any public
// webhook, or one userID owns. Otherwise it returns gorm.ErrRecordNotFound,
// so callers can't tell an inaccessible webhook from a missing one.
func (s *WebhookService) GetAccessibleWebhook(id string, userID uint) (*models.Webhook, error) {
	wh, err := s.repo.Get(id)
	if err != nil {
		return nil, err
	}
	return onlyIfAccessible(wh, userID)
}

// GetAccessibleWebhookWithRequests is GetAccessibleWebhook with the
// webhook's requests loaded, newest first.
func (s *WebhookService) GetAccessibleWebhookWithRequests(id string, userID uint) (*models.Webhook, error) {
	wh, err := s.repo.GetWithRequests(id)
	if err != nil {
		return nil, err
	}
	return onlyIfAccessible(wh, userID)
}

func onlyIfAccessible(wh *models.Webhook, userID uint) (*models.Webhook, error) {
	if wh.UserID != 0 && uint(wh.UserID) != userID {
		return nil, gorm.ErrRecordNotFound
	}
	return wh, nil
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

// RecordRequest stamps wr.ReceivedAt, stores it, and publishes it to the
// webhook's subscribers. Captures for the same webhook are serialized, so
// ReceivedAt order, insert order and publish order all agree - which is what
// lets a subscriber resume from a models.RequestCursor without gaps.
func (s *WebhookService) RecordRequest(wr *models.WebhookRequest) error {
	var err error
	s.broker.withWebhookLock(wr.WebhookID, func(stamp time.Time) {
		wr.ReceivedAt = stamp
		if err = s.repo.InsertRequest(wr); err != nil {
			return
		}
		evt := RequestEvent{Request: *wr}
		if count, countErr := s.repo.CountRequests(wr.WebhookID); countErr == nil {
			evt.Count = &count
		}
		s.broker.publish(wr.WebhookID, evt)
	})
	return err
}

// Subscribe starts receiving the webhook's newly captured requests. Callers
// must Close the subscription when done.
func (s *WebhookService) Subscribe(webhookID string) *Subscription {
	return s.broker.subscribe(webhookID)
}

// GetRequestsAfter returns the webhook's requests positioned after the
// cursor, oldest first.
func (s *WebhookService) GetRequestsAfter(webhookID string, after models.RequestCursor) ([]models.WebhookRequest, error) {
	return s.repo.GetRequestsAfter(webhookID, after)
}

// DeleteWebhook deletes a webhook and its requests, ending its subscriptions.
func (s *WebhookService) DeleteWebhook(id string, userID uint) error {
	if err := s.repo.Delete(id, userID); err != nil {
		return err
	}
	s.broker.closeWebhook(id)
	return nil
}

// CountRequests returns the number of requests captured for a webhook.
func (s *WebhookService) CountRequests(webhookID string) (int64, error) {
	return s.repo.CountRequests(webhookID)
}

// CleanPublicWebhooks cleans up old public webhooks, ending their
// subscriptions.
func (s *WebhookService) CleanPublicWebhooks(d time.Duration) error {
	ids, err := s.repo.CleanPublic(d)
	if err != nil {
		return err
	}
	for _, id := range ids {
		s.broker.closeWebhook(id)
	}
	return nil
}
