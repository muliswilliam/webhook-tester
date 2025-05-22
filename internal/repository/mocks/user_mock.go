package repositoryMocks

import (
	models "webhook-tester/internal/models"

	"github.com/stretchr/testify/mock"
)

// UserRepositoryMock is a mock implementation of repository.UserRepository
type UserRepositoryMock struct {
	mock.Mock
}

// Create mocks the Create method
func (m *UserRepositoryMock) Create(user *models.User) error {
	args := m.Called(user)
	return args.Error(0)
}

// GetByEmail mocks the GetByEmail method
func (m *UserRepositoryMock) GetByEmail(email string) (*models.User, error) {
	args := m.Called(email)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.User), args.Error(1)
}

// GetByID mocks the GetByID method
func (m *UserRepositoryMock) GetByID(id uint) (*models.User, error) {
	args := m.Called(id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.User), args.Error(1)
}

// GetByResetToken mocks the GetByResetToken method
func (m *UserRepositoryMock) GetByResetToken(token string) (*models.User, error) {
	args := m.Called(token)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.User), args.Error(1)
}

// Update mocks the Update method
func (m *UserRepositoryMock) Update(user *models.User) error {
	args := m.Called(user)
	return args.Error(0)
}

// GetByAPIKey mocks the GetByAPIKey method
func (m *UserRepositoryMock) GetByAPIKey(key string) (*models.User, error) {
	args := m.Called(key)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.User), args.Error(1)
}
