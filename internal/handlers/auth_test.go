package handlers_test

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"webhook-tester/internal/handlers"
	metricsMocks "webhook-tester/internal/metrics/mocks"
	"webhook-tester/internal/models"
	serviceMocks "webhook-tester/internal/service/mocks"

	"log"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
)

type AuthHandlerTestSuite struct {
	suite.Suite
	authSvc *serviceMocks.AuthServiceMock
	logger  *log.Logger
	metrics *metricsMocks.RecorderMock
	handler *handlers.AuthHandler
}

func (suite *AuthHandlerTestSuite) SetupTest() {
	suite.authSvc = new(serviceMocks.AuthServiceMock)
	suite.logger = log.New(os.Stdout, "test", log.LstdFlags)
	suite.metrics = new(metricsMocks.RecorderMock)
	suite.handler = handlers.NewAuthHandler(suite.authSvc, suite.logger, suite.metrics)
}

func (suite *AuthHandlerTestSuite) TearDownSuite() {
	suite.authSvc = nil
	suite.logger = nil
	suite.metrics = nil
}

func TestAuthHandlerTestSuite(t *testing.T) {
	suite.Run(t, new(AuthHandlerTestSuite))
}

func (suite *AuthHandlerTestSuite) TestAuthHandler_RegisterGet() {
	r := httptest.NewRequest(http.MethodGet, "/register", nil)
	w := httptest.NewRecorder()
	suite.handler.RegisterGet(w, r)
	suite.Equal(http.StatusOK, w.Code)
	body := w.Body.String()
	suite.Contains(body, "Register")
	suite.Contains(body, "name")
	suite.Contains(body, "email")
	suite.Contains(body, "password")
	suite.authSvc.AssertExpectations(suite.T())
	suite.metrics.AssertExpectations(suite.T())

}

func (suite *AuthHandlerTestSuite) TestAuthHandler_RegisterPost() {
	form := url.Values{}
	email := "test@test.com"
	plainPassword := "test"
	fullName := "test"
	form.Set("name", fullName)
	form.Set("email", email)
	form.Set("password", plainPassword)
	body := form.Encode()
	r := httptest.NewRequest(http.MethodPost, "/register", strings.NewReader(body))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	w := httptest.NewRecorder()
	argEmail := ""
	argFullName := ""
	argPassword := ""
	suite.authSvc.On("Register", email, plainPassword, fullName).
		Run(func(args mock.Arguments) {
			argEmail = args.Get(0).(string)
			argFullName = args.Get(1).(string)
			argPassword = args.Get(2).(string)
		}).
		Return(&models.User{
			Email:    email,
			FullName: fullName,
			Password: "hash",
			APIKey:   "key",
		}, nil)
	suite.metrics.On("IncSignUp").Return()
	suite.handler.RegisterPost(w, r)
	suite.authSvc.AssertExpectations(suite.T())
	suite.metrics.AssertExpectations(suite.T())
	suite.Equal(http.StatusSeeOther, w.Code)
	suite.Equal(email, argEmail)
	suite.Equal(fullName, argFullName)
	suite.NotEmpty(argPassword)
}

func (suite *AuthHandlerTestSuite) TestAuthHandler_RegisterPost_Error() {
	form := url.Values{}
	email := "test@test.com"
	plainPassword := "test"
	fullName := "test"
	form.Set("name", fullName)
	form.Set("email", email)
	form.Set("password", plainPassword)
	body := form.Encode()
	r := httptest.NewRequest(http.MethodPost, "/register", strings.NewReader(body))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	w := httptest.NewRecorder()
	suite.authSvc.On("Register", email, plainPassword, fullName).Return(&models.User{}, errors.New("error"))
	suite.handler.RegisterPost(w, r)
	suite.authSvc.AssertExpectations(suite.T())
	suite.metrics.AssertNotCalled(suite.T(), "IncSignUp")
	suite.Equal(http.StatusOK, w.Code)
}
