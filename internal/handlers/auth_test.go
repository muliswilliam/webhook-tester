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
	"webhook-tester/internal/service"
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

func (suite *AuthHandlerTestSuite) TestAuthHandler_LoginGet() {
	r := httptest.NewRequest(http.MethodGet, "/login", nil)
	w := httptest.NewRecorder()
	suite.handler.LoginGet(w, r)
	suite.Equal(http.StatusOK, w.Code)
	body := w.Body.String()
	suite.Contains(body, "Login")
	suite.Contains(body, "email")
	suite.Contains(body, "password")
	suite.authSvc.AssertExpectations(suite.T())
	suite.metrics.AssertExpectations(suite.T())
}

func (suite *AuthHandlerTestSuite) TestAuthHandler_LoginPost() {
	form := url.Values{}
	email := "test@test.com"
	password := "test"
	form.Set("email", email)
	form.Set("password", password)
	body := form.Encode()
	r := httptest.NewRequest(http.MethodPost, "/login", strings.NewReader(body))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	w := httptest.NewRecorder()
	suite.authSvc.On("Authenticate", email, password).Return(&models.User{}, nil)
	suite.authSvc.On("CreateSession", mock.Anything, mock.Anything, mock.Anything).Return(nil)
	suite.authSvc.On("ClearSession", mock.Anything, mock.Anything, service.GuestSessionName).Return()
	suite.metrics.On("IncLogin").Return()
	suite.handler.LoginPost(w, r)
	suite.authSvc.AssertExpectations(suite.T())
	suite.metrics.AssertExpectations(suite.T())
	suite.Equal(http.StatusSeeOther, w.Code)
}

func (suite *AuthHandlerTestSuite) TestAuthHandler_LoginPost_InvalidCredentials() {
	form := url.Values{}
	email := "test@test.com"
	password := "test"
	form.Set("email", email)
	form.Set("password", password)
	body := form.Encode()
	r := httptest.NewRequest(http.MethodPost, "/login", strings.NewReader(body))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	w := httptest.NewRecorder()
	suite.authSvc.On("Authenticate", email, password).Return(&models.User{}, errors.New("invalid credentials"))
	suite.handler.LoginPost(w, r)
	suite.authSvc.AssertExpectations(suite.T())
	suite.Equal(http.StatusOK, w.Code)
	body = w.Body.String()
	suite.Contains(body, "Invalid email or password")
}

func (suite *AuthHandlerTestSuite) TestAuthHandler_Logout() {
	r := httptest.NewRequest(http.MethodGet, "/logout", nil)
	w := httptest.NewRecorder()
	suite.authSvc.On("ClearSession", mock.Anything, mock.Anything, service.SessionName).Return()
	suite.handler.Logout(w, r)
	suite.authSvc.AssertExpectations(suite.T())
	suite.Equal(http.StatusSeeOther, w.Code)
}

func (suite *AuthHandlerTestSuite) TestAuthHandler_ForgotPasswordGet() {
	r := httptest.NewRequest(http.MethodGet, "/forgot-password", nil)
	w := httptest.NewRecorder()
	suite.handler.ForgotPasswordGet(w, r)
	suite.Equal(http.StatusOK, w.Code)
	body := w.Body.String()
	suite.Contains(body, "Forgot Password")
	suite.Contains(body, "email")
	suite.authSvc.AssertExpectations(suite.T())
	suite.metrics.AssertExpectations(suite.T())
}

func (suite *AuthHandlerTestSuite) TestAuthHandler_ForgotPasswordPost() {
	form := url.Values{}
	email := "test@test.com"
	form.Set("email", email)
	body := form.Encode()
	r := httptest.NewRequest(http.MethodPost, "/forgot-password", strings.NewReader(body))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	w := httptest.NewRecorder()
	suite.authSvc.On("ForgotPassword", email, mock.Anything).Return("", nil)
	suite.handler.ForgotPasswordPost(w, r)
	suite.authSvc.AssertExpectations(suite.T())
	suite.Equal(http.StatusOK, w.Code)
}

func (suite *AuthHandlerTestSuite) TestAuthHandler_ForgotPasswordPost_Error() {
	form := url.Values{}
	email := "test@test.com"
	form.Set("email", email)
	body := form.Encode()
	r := httptest.NewRequest(http.MethodPost, "/forgot-password", strings.NewReader(body))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	w := httptest.NewRecorder()
	suite.authSvc.On("ForgotPassword", email, mock.Anything).Return("", errors.New("error"))
	suite.handler.ForgotPasswordPost(w, r)
	suite.authSvc.AssertExpectations(suite.T())
	suite.Equal(http.StatusOK, w.Code)
}

func (suite *AuthHandlerTestSuite) TestAuthHandler_ResetPasswordGet() {
	r := httptest.NewRequest(http.MethodGet, "/reset-password", nil)
	w := httptest.NewRecorder()
	suite.handler.ResetPasswordGet(w, r)
	suite.Equal(http.StatusOK, w.Code)
	body := w.Body.String()
	suite.Contains(body, "Reset Password")
	suite.Contains(body, "password")
	suite.Contains(body, "Confirm New Password")
	suite.Contains(body, "confirm_password")
	suite.Contains(body, "token")
	suite.authSvc.AssertExpectations(suite.T())
	suite.metrics.AssertExpectations(suite.T())
}

func (suite *AuthHandlerTestSuite) TestAuthHandler_ResetPasswordGet_InvalidToken() {
	r := httptest.NewRequest(http.MethodGet, "/reset-password", nil)
	w := httptest.NewRecorder()
	q := r.URL.Query()
	q.Set("token", "token")
	r.URL.RawQuery = q.Encode()
	suite.authSvc.On("ValidateResetToken", "token").Return(&models.User{}, errors.New("invalid token"))
	suite.handler.ResetPasswordGet(w, r)
	suite.Equal(http.StatusOK, w.Code)
	body := w.Body.String()
	suite.Contains(body, "Invalid or expired reset link")
	suite.authSvc.AssertExpectations(suite.T())
}

func (suite *AuthHandlerTestSuite) TestAuthHandler_ResetPasswordPost() {
	form := url.Values{}
	token := "token"
	password := "password"
	form.Set("token", token)
	form.Set("password", password)
	form.Set("confirm_password", password)
	body := form.Encode()
	r := httptest.NewRequest(http.MethodPost, "/reset-password", strings.NewReader(body))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	w := httptest.NewRecorder()
	suite.authSvc.On("ResetPassword", token, password).Return(nil)
	suite.handler.ResetPasswordPost(w, r)
	suite.authSvc.AssertExpectations(suite.T())
	suite.Equal(http.StatusSeeOther, w.Code)
}

func (suite *AuthHandlerTestSuite) TestAuthHandler_ResetPasswordPost_PasswordsDoNotMatch() {
	form := url.Values{}
	token := "token"
	password := "password"
	form.Set("token", token)
	form.Set("password", password)
	form.Set("confirm_password", "password2")
	body := form.Encode()
	r := httptest.NewRequest(http.MethodPost, "/reset-password", strings.NewReader(body))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	w := httptest.NewRecorder()
	suite.handler.ResetPasswordPost(w, r)
	suite.Equal(http.StatusOK, w.Code)
	body = w.Body.String()
	suite.Contains(body, "Passwords do not match")
}

func (suite *AuthHandlerTestSuite) TestAuthHandler_ResetPasswordPost_Error() {
	form := url.Values{}
	token := "token"
	password := "password"
	form.Set("token", token)
	form.Set("password", password)
	form.Set("confirm_password", password)
	body := form.Encode()
	r := httptest.NewRequest(http.MethodPost, "/reset-password", strings.NewReader(body))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	w := httptest.NewRecorder()
	suite.authSvc.On("ResetPassword", token, password).Return(errors.New("error"))
	suite.handler.ResetPasswordPost(w, r)
	suite.authSvc.AssertExpectations(suite.T())
	suite.Equal(http.StatusOK, w.Code)
}
