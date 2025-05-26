package service_test

import (
	"errors"
	"net/http/httptest"
	"strings"
	"time"
	"webhook-tester/internal/models"
	repositoryMocks "webhook-tester/internal/repository/mocks"
	"webhook-tester/internal/service"
	serviceMocks "webhook-tester/internal/service/mocks"
	"webhook-tester/internal/utils"

	"github.com/gorilla/sessions"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"testing"
)

type AuthServiceTestSuite struct {
	suite.Suite
	mockRepo          repositoryMocks.UserRepositoryMock
	passwordValidator *testPasswordValidatorMock
	passwordHasher    *testPasswordHasherMock
	sessionStore      *serviceMocks.SessionStoreMock
	svc               service.AuthService
}

type testPasswordHasherMock struct {
	mock.Mock
}

func (t *testPasswordHasherMock) HashPassword(password string) (string, error) {
	args := t.Called(password)
	return args.String(0), args.Error(1)
}

func (t *testPasswordHasherMock) CheckPasswordHash(password, hash string) bool {
	args := t.Called(password, hash)
	return args.Bool(0)
}

type testPasswordValidatorMock struct {
	mock.Mock
}

func (t *testPasswordValidatorMock) Validate(pw string, rules utils.PasswordRules) error {
	args := t.Called(pw, rules)
	return args.Error(0)
}

func (suite *AuthServiceTestSuite) SetupTest() {
	suite.mockRepo = repositoryMocks.UserRepositoryMock{}
	suite.passwordHasher = new(testPasswordHasherMock)
	suite.passwordValidator = new(testPasswordValidatorMock)
	suite.sessionStore = new(serviceMocks.SessionStoreMock)
	suite.svc = service.NewAuthService(&suite.mockRepo, suite.sessionStore, suite.passwordHasher, suite.passwordValidator)
}

func TestAuthServiceTestSuite(t *testing.T) {
	suite.Run(t, new(AuthServiceTestSuite))
}

func (suite *AuthServiceTestSuite) TestAuthService_Register() {
	suite.mockRepo.On("GetByEmail", "email").Return(&models.User{}, nil)
	suite.passwordValidator.On("Validate", "", mock.Anything).Return(nil)
	suite.passwordHasher.On("HashPassword", "").Return("hash", nil)
	suite.mockRepo.On("Create", mock.Anything).Return(nil)
	_, err := suite.svc.Register("email", "", "name")
	suite.NoError(err)
}

func (suite *AuthServiceTestSuite) TestAuthService_Register_EmailExists() {
	suite.mockRepo.On("GetByEmail", "email").Return(&models.User{}, errors.New("exists"))
	_, err := suite.svc.Register("email", "password", "name")
	suite.Error(err)
}

func (suite *AuthServiceTestSuite) TestAuthService_Register_HashError() {
	suite.mockRepo.On("GetByEmail", "email").Return(nil, errors.New("not found"))
	suite.mockRepo.On("Create", mock.Anything).Return(errors.New("hash error"))
	_, err := suite.svc.Register("email", "password", "name")
	suite.Error(err)
}

func (suite *AuthServiceTestSuite) TestAuthService_Register_APIKeyError() {
	suite.mockRepo.On("GetByEmail", "email").Return(nil, errors.New("not found"))
	suite.mockRepo.On("Create", mock.Anything).Return(nil)
	_, err := suite.svc.Register("email", "password", "name")
	suite.Error(err)
}

func (suite *AuthServiceTestSuite) TestAuthService_Register_Success() {
	suite.mockRepo.On("GetByEmail", "email").Return(nil, nil)
	suite.passwordValidator.On("Validate", "password", mock.Anything).Return(nil)
	suite.passwordHasher.On("HashPassword", "password").Return("hash", nil)
	suite.mockRepo.On("Create", mock.Anything).Return(nil)
	_, err := suite.svc.Register("email", "password", "name")
	suite.NoError(err)
}

func (suite *AuthServiceTestSuite) TestAuthService_Register_ValidateError() {
	suite.mockRepo.On("GetByEmail", "email").Return(nil, nil)
	suite.passwordValidator.On("Validate", "", mock.Anything).Return(errors.New("validate error"))
	_, err := suite.svc.Register("email", "", "name")
	suite.Error(err)
}

func (suite *AuthServiceTestSuite) TestAuthService_Authenticate() {
	suite.mockRepo.On("GetByEmail", "email").Return(&models.User{Email: "email", Password: "hash"}, nil)
	suite.mockRepo.On("Update", mock.Anything).Return(nil)
	suite.passwordHasher.On("CheckPasswordHash", "password", "hash").Return(true)
	_, err := suite.svc.Authenticate("email", "password")
	suite.NoError(err)
}

func (suite *AuthServiceTestSuite) TestAuthService_Authenticate_InvalidCredentials() {
	suite.mockRepo.On("GetByEmail", "email").Return(&models.User{Email: "email", Password: "hash"}, nil)
	suite.passwordHasher.On("CheckPasswordHash", "password", "hash").Return(false)
	_, err := suite.svc.Authenticate("email", "password")
	suite.Error(err)
}

func (suite *AuthServiceTestSuite) TestAuthService_Authenticate_UserNotFound() {
	suite.mockRepo.On("GetByEmail", "email").Return(nil, errors.New("not found"))
	_, err := suite.svc.Authenticate("email", "password")
	suite.Error(err)
}

func (suite *AuthServiceTestSuite) TestAuthService_Authorize() {
	suite.mockRepo.On("GetByID", uint(1)).Return(&models.User{}, nil)
	suite.sessionStore.On("GetValue", mock.Anything, mock.Anything, mock.Anything).Return("123", nil)
	req := httptest.NewRequest("GET", "/", nil)
	uid, err := suite.svc.Authorize(req)
	suite.NoError(err)
	suite.Equal(uint(123), uid)
}

func (suite *AuthServiceTestSuite) TestAuthService_Authorize_InvalidUID() {
	suite.sessionStore.On("GetValue", mock.Anything, mock.Anything, mock.Anything).Return("invalid", nil)
	req := httptest.NewRequest("GET", "/", nil)
	_, err := suite.svc.Authorize(req)
	suite.Error(err)
}

func (suite *AuthServiceTestSuite) TestAuthService_GetCurrentUser() {
	suite.mockRepo.On("GetByID", uint(123)).Return(&models.User{}, nil)
	suite.sessionStore.On("GetValue", mock.Anything, mock.Anything, mock.Anything).Return("123", nil)
	req := httptest.NewRequest("GET", "/", nil)
	user, err := suite.svc.GetCurrentUser(req)
	suite.NoError(err)
	suite.Equal(&models.User{}, user)
}

func (suite *AuthServiceTestSuite) TestAuthService_GetCurrentUser_InvalidUID() {
	suite.sessionStore.On("GetValue", mock.Anything, mock.Anything, mock.Anything).Return("invalid", nil)
	req := httptest.NewRequest("GET", "/", nil)
	_, err := suite.svc.GetCurrentUser(req)
	suite.Error(err)
}

func (suite *AuthServiceTestSuite) TestAuthService_GetCurrentUser_UserNotFound() {
	suite.sessionStore.On("GetValue", mock.Anything, mock.Anything, mock.Anything).Return("123", nil)
	suite.mockRepo.On("GetByID", uint(123)).Return(nil, errors.New("not found"))
	req := httptest.NewRequest("GET", "/", nil)
	_, err := suite.svc.GetCurrentUser(req)
	suite.Error(err)
}

func (suite *AuthServiceTestSuite) TestAuthService_CreateSession() {
	suite.sessionStore.On("New", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(&sessions.Session{}, nil)
	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()
	err := suite.svc.CreateSession(w, req, uint(1))
	suite.NoError(err)
}

func (suite *AuthServiceTestSuite) TestAuthService_CreateSession_NewError() {
	suite.sessionStore.On("New", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(&sessions.Session{}, errors.New("new error"))
	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()
	err := suite.svc.CreateSession(w, req, uint(1))
	suite.Error(err)
}

func (suite *AuthServiceTestSuite) TestAuthService_ClearSession() {
	suite.sessionStore.On("Delete", mock.Anything, mock.Anything, mock.Anything).Return(nil)
	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()
	suite.svc.ClearSession(w, req, "session")
}

func (suite *AuthServiceTestSuite) TestAuthService_ClearSession_DeleteError() {
	suite.sessionStore.On("Delete", mock.Anything, mock.Anything, mock.Anything).Return(errors.New("delete error"))
	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()
	suite.svc.ClearSession(w, req, "session")
}

func (suite *AuthServiceTestSuite) TestAuthService_ForgotPassword() {
	suite.mockRepo.On("GetByEmail", "email").Return(&models.User{}, nil)
	suite.mockRepo.On("Update", mock.Anything).Return(nil)
	url, err := suite.svc.ForgotPassword("email", "domain")
	suite.NoError(err)
	suite.True(strings.Contains(url, "domain/reset-password?token="))
	suite.mockRepo.AssertCalled(suite.T(), "Update", mock.Anything)
}

func (suite *AuthServiceTestSuite) TestAuthService_ForgotPassword_UserNotFound() {
	suite.mockRepo.On("GetByEmail", "email").Return(nil, errors.New("not found"))
	_, err := suite.svc.ForgotPassword("email", "domain")
	suite.Error(err)
	suite.mockRepo.AssertNotCalled(suite.T(), "Update", mock.Anything)
}

func (suite *AuthServiceTestSuite) TestAuthService_ValidateResetToken() {
	user := &models.User{
		ResetToken:       "token",
		ResetTokenExpiry: time.Now().Add(24 * time.Hour),
	}
	suite.mockRepo.On("GetByResetToken", "token").Return(user, nil)
	result, err := suite.svc.ValidateResetToken("token")
	suite.NoError(err)
	suite.Equal(user, result)
}

func (suite *AuthServiceTestSuite) TestAuthService_ValidateResetToken_Expired() {
	user := &models.User{
		ResetToken:       "token",
		ResetTokenExpiry: time.Now().Add(-24 * time.Hour),
	}
	suite.mockRepo.On("GetByResetToken", "token").Return(user, nil)
	_, err := suite.svc.ValidateResetToken("token")
	suite.Error(err)
}

func (suite *AuthServiceTestSuite) TestAuthService_ValidateResetToken_UserNotFound() {
	suite.mockRepo.On("GetByResetToken", "token").Return(nil, errors.New("not found"))
	_, err := suite.svc.ValidateResetToken("token")
	suite.Error(err)
}

func (suite *AuthServiceTestSuite) TestAuthService_ResetPassword() {
	user := &models.User{
		ResetToken:       "token",
		ResetTokenExpiry: time.Now().Add(24 * time.Hour),
	}
	suite.mockRepo.On("GetByResetToken", "token").Return(user, nil)
	suite.passwordValidator.On("Validate", "newPassword", mock.Anything).Return(nil)
	suite.passwordHasher.On("HashPassword", "newPassword").Return("hash", nil)
	suite.mockRepo.On("Update", mock.Anything).Return(nil)
	err := suite.svc.ResetPassword("token", "newPassword")
	suite.NoError(err)
	suite.mockRepo.AssertCalled(suite.T(), "Update", mock.Anything)
}

func (suite *AuthServiceTestSuite) TestAuthService_ResetPassword_InvalidToken() {
	user := &models.User{
		ResetToken:       "token",
		ResetTokenExpiry: time.Now().Add(-24 * time.Hour),
	}
	suite.mockRepo.On("GetByResetToken", "token").Return(user, nil)
	err := suite.svc.ResetPassword("token", "newPassword")
	suite.Error(err)
	suite.mockRepo.AssertNotCalled(suite.T(), "Update", mock.Anything)
}

func (suite *AuthServiceTestSuite) TestAuthService_ResetPassword_ValidateError() {
	user := &models.User{
		ResetToken:       "token",
		ResetTokenExpiry: time.Now().Add(24 * time.Hour),
	}
	suite.mockRepo.On("GetByResetToken", "token").Return(user, nil)
	suite.passwordValidator.On("Validate", "newPassword", mock.Anything).Return(errors.New("validate error"))
	err := suite.svc.ResetPassword("token", "newPassword")
	suite.Error(err)
	suite.mockRepo.AssertNotCalled(suite.T(), "Update", mock.Anything)
}

func (suite *AuthServiceTestSuite) TestAuthService_ResetPassword_HashError() {
	user := &models.User{
		ResetToken:       "token",
		ResetTokenExpiry: time.Now().Add(24 * time.Hour),
	}
	suite.mockRepo.On("GetByResetToken", "token").Return(user, nil)
	suite.passwordValidator.On("Validate", "newPassword", mock.Anything).Return(nil)
	suite.passwordHasher.On("HashPassword", "newPassword").Return("", errors.New("hash error"))
	err := suite.svc.ResetPassword("token", "newPassword")
	suite.Error(err)
	suite.mockRepo.AssertNotCalled(suite.T(), "Update", mock.Anything)
}

func (suite *AuthServiceTestSuite) TestAuthService_ResetPassword_UpdateError() {
	user := &models.User{
		ResetToken:       "token",
		ResetTokenExpiry: time.Now().Add(24 * time.Hour),
	}
	suite.mockRepo.On("GetByResetToken", "token").Return(user, nil)
	suite.passwordValidator.On("Validate", "newPassword", mock.Anything).Return(nil)
	suite.passwordHasher.On("HashPassword", "newPassword").Return("hash", nil)
	suite.mockRepo.On("Update", mock.Anything).Return(errors.New("update error"))
	err := suite.svc.ResetPassword("token", "newPassword")
	suite.Error(err)
}

func (suite *AuthServiceTestSuite) TestAuthService_ValidateAPIKey() {
	user := &models.User{APIKey: "key"}
	suite.mockRepo.On("GetByAPIKey", "key").Return(user, nil)
	result, err := suite.svc.ValidateAPIKey("key")
	suite.NoError(err)
	suite.Equal(user, result)
}

func (suite *AuthServiceTestSuite) TestAuthService_ValidateAPIKey_InvalidKey() {
	suite.mockRepo.On("GetByAPIKey", "key").Return(nil, errors.New("not found"))
	_, err := suite.svc.ValidateAPIKey("key")
	suite.Error(err)
}

func (suite *AuthServiceTestSuite) TestAuthService_CreateGuestSession() {
	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()
	err := suite.svc.CreateGuestSession(req, w, "webhookID")
	suite.NoError(err)
}
