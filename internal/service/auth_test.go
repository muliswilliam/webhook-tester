package service_test

import (
	"errors"
	dbMock "webhook-tester/internal/db/mocks"
	"webhook-tester/internal/models"
	repositoryMocks "webhook-tester/internal/repository/mocks"
	"webhook-tester/internal/service"
	"webhook-tester/internal/utils"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"testing"
)

type AuthServiceTestSuite struct {
	suite.Suite
	mockRepo          repositoryMocks.UserRepositoryMock
	passwordValidator *testPasswordValidatorMock
	passwordHasher    *testPasswordHasherMock
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
	mockDB, _ := dbMock.Connect()
	mockDB.Delete(&models.User{})
	suite.passwordHasher = new(testPasswordHasherMock)
	suite.passwordValidator = new(testPasswordValidatorMock)
	suite.svc = service.NewAuthService(&suite.mockRepo, mockDB, suite.passwordHasher, suite.passwordValidator, "auth-secret")
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
