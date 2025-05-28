package serviceMocks

import (
	"time"
	"webhook-tester/internal/models"

	"github.com/stretchr/testify/mock"
)

type WebhookServiceMock struct {
	mock.Mock
}

func (m *WebhookServiceMock) CreateWebhook(wh *models.Webhook) error {
	args := m.Called(wh)
	return args.Error(0)
}

func (s *WebhookServiceMock) GetWebhook(id string) (*models.Webhook, error) {
	args := s.Called(id)
	return args.Get(0).(*models.Webhook), args.Error(1)
}

func (s *WebhookServiceMock) GetUserWebhook(id string, userID uint) (*models.Webhook, error) {
	args := s.Called(id, userID)
	return args.Get(0).(*models.Webhook), args.Error(1)
}

func (s *WebhookServiceMock) ListWebhooks(userID uint) ([]models.Webhook, error) {
	args := s.Called(userID)
	return args.Get(0).([]models.Webhook), args.Error(1)
}

func (s *WebhookServiceMock) UpdateWebhook(w *models.Webhook) error {
	args := s.Called(w)
	return args.Error(0)
}

func (s *WebhookServiceMock) CreateRequest(wr *models.WebhookRequest) error {
	args := s.Called(wr)
	return args.Error(0)
}

func (s *WebhookServiceMock) DeleteWebhook(id string, userID uint) error {
	args := s.Called(id, userID)
	return args.Error(0)
}

func (s *WebhookServiceMock) GetWebhookWithRequests(id string) (*models.Webhook, error) {
	args := s.Called(id)
	return args.Get(0).(*models.Webhook), args.Error(1)
}

func (s *WebhookServiceMock) CleanPublicWebhooks(d time.Duration) error {
	args := s.Called(d)
	return args.Error(0)
}
