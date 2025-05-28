package serviceMocks

import (
	"webhook-tester/internal/models"
	"webhook-tester/internal/service"

	"github.com/stretchr/testify/mock"
)

type WebhookRequestServiceMock struct {
	mock.Mock
}

var _ service.WebhookRequestService = (*WebhookRequestServiceMock)(nil)

func (m *WebhookRequestServiceMock) Record(rq *models.WebhookRequest) error {
	args := m.Called(rq)
	return args.Error(0)
}

func (m *WebhookRequestServiceMock) Get(id string) (*models.WebhookRequest, error) {
	args := m.Called(id)
	return args.Get(0).(*models.WebhookRequest), args.Error(1)
}

func (m *WebhookRequestServiceMock) List(webhookID string) ([]models.WebhookRequest, error) {
	args := m.Called(webhookID)
	return args.Get(0).([]models.WebhookRequest), args.Error(1)
}

func (m *WebhookRequestServiceMock) Delete(id string) error {
	args := m.Called(id)
	return args.Error(0)
}

func (m *WebhookRequestServiceMock) DeleteAll(webhookID string) error {
	args := m.Called(webhookID)
	return args.Error(0)
}
