package repositoryMocks

import (
	models "webhook-tester/internal/models"

	"github.com/stretchr/testify/mock"
)

type WebhookRequestRepositoryMock struct {
	mock.Mock
}

func (m *WebhookRequestRepositoryMock) Insert(req *models.WebhookRequest) error {
	args := m.Called(req)
	return args.Error(0)
}

func (m *WebhookRequestRepositoryMock) GetByID(id string) (*models.WebhookRequest, error) {
	args := m.Called(id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.WebhookRequest), args.Error(1)
}

func (m *WebhookRequestRepositoryMock) ListByWebhook(webhookID string) ([]models.WebhookRequest, error) {
	args := m.Called(webhookID)
	return args.Get(0).([]models.WebhookRequest), args.Error(1)
}

func (m *WebhookRequestRepositoryMock) DeleteByID(id string) error {
	args := m.Called(id)
	return args.Error(0)
}

func (m *WebhookRequestRepositoryMock) DeleteByWebhook(webhookID string) error {
	args := m.Called(webhookID)
	return args.Error(0)
}
