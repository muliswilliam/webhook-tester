package repositoryMocks

import (
	"time"
	models "webhook-tester/internal/models"

	"github.com/stretchr/testify/mock"
)

type WebhookRepositoryMock struct {
	mock.Mock
}

func (m *WebhookRepositoryMock) CleanPublic(d time.Duration) error {
	args := m.Called(d)
	return args.Error(0)
}

func (m *WebhookRepositoryMock) Delete(id string, userID uint) error {
	args := m.Called(id, userID)
	return args.Error(0)
}

func (m *WebhookRepositoryMock) Get(id string) (*models.Webhook, error) {
	args := m.Called(id)
	return args.Get(0).(*models.Webhook), args.Error(1)
}

func (m *WebhookRepositoryMock) GetAll() ([]models.Webhook, error) {
	args := m.Called()
	return args.Get(0).([]models.Webhook), args.Error(1)
}

func (m *WebhookRepositoryMock) GetAllByUser(userID uint) ([]models.Webhook, error) {
	args := m.Called(userID)
	return args.Get(0).([]models.Webhook), args.Error(1)
}

func (m *WebhookRepositoryMock) GetByUser(id string, userID uint) (*models.Webhook, error) {
	args := m.Called(id, userID)
	return args.Get(0).(*models.Webhook), args.Error(1)
}

func (m *WebhookRepositoryMock) GetWithRequests(id string) (*models.Webhook, error) {
	args := m.Called(id)
	return args.Get(0).(*models.Webhook), args.Error(1)
}

func (m *WebhookRepositoryMock) Insert(webhook *models.Webhook) error {
	args := m.Called(webhook)
	return args.Error(0)
}

func (m *WebhookRepositoryMock) InsertRequest(wr *models.WebhookRequest) error {
	args := m.Called(wr)
	return args.Error(0)
}

func (m *WebhookRepositoryMock) Update(webhook *models.Webhook) error {
	args := m.Called(webhook)
	return args.Error(0)
}
