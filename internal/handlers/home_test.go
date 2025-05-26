package handlers_test

import (
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"testing"
	"webhook-tester/internal/handlers"
	metricsMocks "webhook-tester/internal/metrics/mocks"
	"webhook-tester/internal/models"
	serviceMocks "webhook-tester/internal/service/mocks"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
)

type HomeHandlerTestSuite struct {
	suite.Suite

	webhookSvc *serviceMocks.WebhookServiceMock
	authSvc    *serviceMocks.AuthServiceMock
	logger     *log.Logger
	metrics    *metricsMocks.RecorderMock
	handler    *handlers.HomeHandler
}

func (suite *HomeHandlerTestSuite) SetupTest() {
	suite.webhookSvc = new(serviceMocks.WebhookServiceMock)
	suite.authSvc = new(serviceMocks.AuthServiceMock)
	suite.logger = new(log.Logger)
	suite.metrics = new(metricsMocks.RecorderMock)
	suite.handler = handlers.NewHomeHandler(suite.webhookSvc, suite.authSvc, suite.logger, suite.metrics)
}

func (suite *HomeHandlerTestSuite) TearDownSuite() {
	suite.webhookSvc = nil
	suite.authSvc = nil
	suite.logger = nil
	suite.metrics = nil
}

func TestHomeHandlerTestSuite(t *testing.T) {
	suite.Run(t, new(HomeHandlerTestSuite))
}

func (suite *HomeHandlerTestSuite) TestHomeHandler_Home_GuestSession() {
	suite.authSvc.On("Authorize", mock.Anything).Return(uint(0), nil)
	suite.authSvc.On("GetGuestSession", mock.Anything).Return("", nil)
	suite.authSvc.On("CreateGuestSession", mock.Anything, mock.Anything, mock.Anything).Return(nil)
	suite.metrics.On("IncWebhooksCreated").Return()
	var wh models.Webhook
	suite.webhookSvc.On("CreateWebhook", mock.Anything).
		Run(func(args mock.Arguments) {
			wh = *args.Get(0).(*models.Webhook)
		}).Return(nil)
	suite.webhookSvc.On("GetWebhookWithRequests", mock.Anything).Return(&wh, nil)
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	suite.handler.Home(w, r)
	suite.authSvc.AssertExpectations(suite.T())
	suite.metrics.AssertExpectations(suite.T())
	suite.webhookSvc.AssertExpectations(suite.T())
	res := w.Result()
	suite.Equal(http.StatusOK, res.StatusCode)
	body, err := io.ReadAll(res.Body)
	suite.NoError(err)
	suite.Contains(string(body), "Default Webhook")
	suite.Contains(string(body), wh.ID)
}

func (suite *HomeHandlerTestSuite) TestHomeHandler_Home_LoggedIn() {
	suite.authSvc.On("Authorize", mock.Anything).Return(uint(1), nil)
	user := &models.User{}
	user.ID = 1
	suite.authSvc.On("GetCurrentUser", mock.Anything).Return(user, nil)
	wh := models.Webhook{
		ID: "test",
	}
	suite.webhookSvc.On("ListWebhooks", mock.Anything).Return([]models.Webhook{wh}, nil)
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	suite.handler.Home(w, r)
	suite.authSvc.AssertExpectations(suite.T())
	suite.webhookSvc.AssertExpectations(suite.T())
	res := w.Result()
	suite.Equal(http.StatusOK, res.StatusCode)
	body, err := io.ReadAll(res.Body)
	suite.NoError(err)
	suite.Contains(string(body), wh.ID)
}

func (suite *HomeHandlerTestSuite) TestHomeHandler_Home_LoggedIn_Address() {
	suite.authSvc.On("Authorize", mock.Anything).Return(uint(1), nil)
	user := &models.User{}
	user.ID = 1
	suite.authSvc.On("GetCurrentUser", mock.Anything).Return(user, nil)
	wh := models.Webhook{
		ID: "test",
	}
	suite.webhookSvc.On("GetWebhookWithRequests", "test").Return(&wh, nil)
	suite.webhookSvc.On("ListWebhooks", uint(user.ID)).Return([]models.Webhook{wh}, nil)
	r := httptest.NewRequest(http.MethodGet, "/?address=test", nil)
	q := r.URL.Query()
	q.Set("address", "test")
	r.URL.RawQuery = q.Encode()

	w := httptest.NewRecorder()
	suite.handler.Home(w, r)
	suite.authSvc.AssertExpectations(suite.T())
	suite.webhookSvc.AssertExpectations(suite.T())
	res := w.Result()
	suite.Equal(http.StatusOK, res.StatusCode)
	body, err := io.ReadAll(res.Body)
	suite.NoError(err)
	suite.Contains(string(body), wh.ID)
}
