package serviceMocks

import (
	"net/http"
	"webhook-tester/internal/models"

	"github.com/stretchr/testify/mock"
)

type AuthServiceMock struct {
	mock.Mock
}

func (m *AuthServiceMock) Register(email, plainPassword, fullName string) (*models.User, error) {
	args := m.Called(email, plainPassword, fullName)
	return args.Get(0).(*models.User), args.Error(1)
}

func (m *AuthServiceMock) Authenticate(email, plainPassword string) (*models.User, error) {
	args := m.Called(email, plainPassword)
	return args.Get(0).(*models.User), args.Error(1)
}

func (m *AuthServiceMock) Authorize(r *http.Request) (uint, error) {
	args := m.Called(r)
	return args.Get(0).(uint), args.Error(1)
}

func (m *AuthServiceMock) GetCurrentUser(r *http.Request) (*models.User, error) {
	args := m.Called(r)
	return args.Get(0).(*models.User), args.Error(1)
}

func (m *AuthServiceMock) CreateSession(w http.ResponseWriter, r *http.Request, userID uint) error {
	args := m.Called(w, r, userID)
	return args.Error(0)
}

func (m *AuthServiceMock) ClearSession(w http.ResponseWriter, r *http.Request, name string) {
	m.Called(w, r, name)
}

func (m *AuthServiceMock) ForgotPassword(email, domain string) (string, error) {
	args := m.Called(email, domain)
	return args.String(0), args.Error(1)
}

func (m *AuthServiceMock) ValidateResetToken(token string) (*models.User, error) {
	args := m.Called(token)
	return args.Get(0).(*models.User), args.Error(1)
}

func (m *AuthServiceMock) ResetPassword(token, newPassword string) error {
	args := m.Called(token, newPassword)
	return args.Error(0)
}

func (m *AuthServiceMock) ValidateAPIKey(key string) (*models.User, error) {
	args := m.Called(key)
	return args.Get(0).(*models.User), args.Error(1)
}

func (m *AuthServiceMock) CreateGuestSession(r *http.Request, w http.ResponseWriter, webhookID string) error {
	args := m.Called(r, w, webhookID)
	return args.Error(0)
}

func (m *AuthServiceMock) GetGuestSession(r *http.Request) (string, error) {
	args := m.Called(r)
	return args.String(0), args.Error(1)
}
