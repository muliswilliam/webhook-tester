package handlers_test

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"
	"webhook-tester/internal/handlers"
	metricsMocks "webhook-tester/internal/metrics/mocks"
	"webhook-tester/internal/models"
	serviceMocks "webhook-tester/internal/service/mocks"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
	"gorm.io/datatypes"
)

type WebhookHandlerTestSuite struct {
	suite.Suite
	webhookSvc *serviceMocks.WebhookServiceMock
	authSvc    *serviceMocks.AuthServiceMock
	recorder   *metricsMocks.RecorderMock
	logger     *log.Logger
	handler    *handlers.WebhookHandler
}

func (suite *WebhookHandlerTestSuite) SetupTest() {
	suite.webhookSvc = new(serviceMocks.WebhookServiceMock)
	suite.authSvc = new(serviceMocks.AuthServiceMock)
	suite.recorder = new(metricsMocks.RecorderMock)
	suite.logger = log.New(os.Stdout, "test", log.LstdFlags)
	suite.handler = handlers.NewWebhookHandler(suite.webhookSvc, suite.authSvc, suite.logger, suite.recorder)
}

func (suite *WebhookHandlerTestSuite) TearDownSuite() {
	suite.webhookSvc = nil
	suite.authSvc = nil
	suite.recorder = nil
	suite.logger = nil
	suite.handler = nil
}

func TestWebhookHandlerTestSuite(t *testing.T) {
	suite.Run(t, new(WebhookHandlerTestSuite))
}

func (suite *WebhookHandlerTestSuite) TestCreate() {
	title := "Test Webhook"
	form := url.Values{}
	form.Set("title", title)
	body := form.Encode()

	req := httptest.NewRequest(http.MethodPost, "/create-webhook", strings.NewReader(body))
	rr := httptest.NewRecorder()

	suite.authSvc.On("Authorize", req).Return(uint(1), nil)
	suite.webhookSvc.On("CreateWebhook", mock.Anything).Return(nil)
	suite.recorder.On("IncWebhooksCreated").Return()

	suite.handler.Create(rr, req)
	asserts := assert.New(suite.T())
	asserts.Equal(http.StatusSeeOther, rr.Code)
	suite.webhookSvc.AssertExpectations(suite.T())
	suite.authSvc.AssertExpectations(suite.T())
	suite.recorder.AssertExpectations(suite.T())
}

func (suite *WebhookHandlerTestSuite) TestCreate_AuthError() {
	req := httptest.NewRequest(http.MethodPost, "/create-webhook", nil)
	rr := httptest.NewRecorder()

	mockCall := suite.authSvc.On("Authorize", req).Return(uint(0), errors.New("auth error"))

	suite.handler.Create(rr, req)
	asserts := assert.New(suite.T())
	asserts.Equal(http.StatusUnauthorized, rr.Code)
	mockCall.Unset()
}

func (suite *WebhookHandlerTestSuite) TestCreate_WebhookFormData() {
	form := url.Values{}
	form.Set("title", "Test Webhook")
	form.Set("content_type", "application/json")
	form.Set("response_code", "200")
	form.Set("response_delay", "100")
	form.Set("payload", "{}")
	form.Set("notify_on_event", "true")
	form.Set("response_headers", `{"X-Foo":"Bar"}`)
	body := form.Encode()

	req := httptest.NewRequest(http.MethodPost, "/create-webhook", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	req.ParseForm()

	suite.authSvc.On("Authorize", req).Return(uint(1), nil)
	var created models.Webhook
	suite.webhookSvc.
		On("CreateWebhook", mock.AnythingOfType("*models.Webhook")).
		Return(nil).
		Run(func(args mock.Arguments) {
			created = *args.Get(0).(*models.Webhook)
		})

	suite.recorder.On("IncWebhooksCreated").Return()

	rr := httptest.NewRecorder()
	suite.handler.Create(rr, req)

	suite.webhookSvc.AssertExpectations(suite.T())
	suite.authSvc.AssertExpectations(suite.T())
	suite.recorder.AssertExpectations(suite.T())

	asserts := assert.New(suite.T())
	asserts.Equal(http.StatusSeeOther, rr.Code)
	asserts.Equal("/?address="+created.ID, rr.Header().Get("Location"))
	asserts.NotEmpty(created.ID)
	asserts.Equal("Test Webhook", created.Title)
	asserts.Equal("application/json", *created.ContentType)
	asserts.Equal(200, created.ResponseCode)
	asserts.Equal(uint(100), created.ResponseDelay)
	asserts.Equal("{}", *created.Payload)
	asserts.Equal(true, created.NotifyOnEvent)
	expectedHeaders := datatypes.JSONMap{"X-Foo": "Bar"}
	asserts.Equal(expectedHeaders, created.ResponseHeaders)
}

func (suite *WebhookHandlerTestSuite) TestCreate_CreateWebhookError() {
	suite.authSvc.On("Authorize", mock.Anything).Return(uint(1), nil)
	suite.webhookSvc.On("CreateWebhook", mock.AnythingOfType("*models.Webhook")).Return(errors.New("create webhook error"))

	req := httptest.NewRequest(http.MethodPost, "/create-webhook", nil)
	chiCtx := chi.NewRouteContext()
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, chiCtx))

	rr := httptest.NewRecorder()

	suite.handler.Create(rr, req)

	suite.webhookSvc.AssertExpectations(suite.T())
	suite.authSvc.AssertExpectations(suite.T())
	asserts := assert.New(suite.T())
	asserts.Equal(http.StatusInternalServerError, rr.Code)
}

func (suite *WebhookHandlerTestSuite) TestDeleteWebhook() {
	webhookID := "test-webhook-id"
	userID := uint(1)

	suite.authSvc.On("Authorize", mock.Anything).Return(userID, nil)
	suite.webhookSvc.On("DeleteWebhook", webhookID, userID).Return(nil)

	req := httptest.NewRequest(http.MethodDelete, fmt.Sprintf("/delete-webhook/%s", webhookID), nil)
	chiCtx := chi.NewRouteContext()
	chiCtx.URLParams.Add("id", webhookID)
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, chiCtx))

	rr := httptest.NewRecorder()

	suite.handler.DeleteWebhook(rr, req)

	suite.webhookSvc.AssertExpectations(suite.T())
	suite.authSvc.AssertExpectations(suite.T())
	asserts := assert.New(suite.T())
	asserts.Equal(http.StatusSeeOther, rr.Code)
	asserts.Equal("/", rr.Header().Get("Location"))
}

func (suite *WebhookHandlerTestSuite) TestDeleteWebhook_AuthError() {
	webhookID := "test-webhook-id"

	suite.authSvc.On("Authorize", mock.Anything).Return(uint(0), errors.New("auth error"))

	req := httptest.NewRequest(http.MethodDelete, fmt.Sprintf("/delete-webhook/%s", webhookID), nil)
	rr := httptest.NewRecorder()

	suite.handler.DeleteWebhook(rr, req)

	suite.authSvc.AssertExpectations(suite.T())
	asserts := assert.New(suite.T())
	asserts.Equal(http.StatusUnauthorized, rr.Code)
}

func (suite *WebhookHandlerTestSuite) TestDeleteWebhook_WebhookNotFound() {
	webhookID := "test-webhook-id"
	userID := uint(1)

	suite.authSvc.On("Authorize", mock.Anything).Return(userID, nil)
	suite.webhookSvc.On("DeleteWebhook", webhookID, userID).Return(errors.New("webhook not found"))

	req := httptest.NewRequest(http.MethodDelete, fmt.Sprintf("/delete-webhook/%s", webhookID), nil)
	chiCtx := chi.NewRouteContext()
	chiCtx.URLParams.Add("id", webhookID)
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, chiCtx))

	rr := httptest.NewRecorder()

	suite.handler.DeleteWebhook(rr, req)

	suite.webhookSvc.AssertExpectations(suite.T())
	suite.authSvc.AssertExpectations(suite.T())
	asserts := assert.New(suite.T())
	asserts.Equal(http.StatusInternalServerError, rr.Code)
}

func (suite *WebhookHandlerTestSuite) TestDeleteWebhook_EmptyWebhookID() {
	suite.authSvc.On("Authorize", mock.Anything).Return(uint(1), nil)

	req := httptest.NewRequest(http.MethodDelete, "/delete-webhook/", nil)
	chiCtx := chi.NewRouteContext()
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, chiCtx))

	rr := httptest.NewRecorder()

	suite.handler.DeleteWebhook(rr, req)

	suite.webhookSvc.AssertExpectations(suite.T())
	suite.authSvc.AssertExpectations(suite.T())
	asserts := assert.New(suite.T())
	asserts.Equal(http.StatusBadRequest, rr.Code)
}

func (suite *WebhookHandlerTestSuite) TestUpdateWebhook() {
	webhookID := "test-webhook-id"
	userID := uint(1)

	suite.authSvc.On("Authorize", mock.Anything).Return(userID, nil)
	suite.webhookSvc.On("GetUserWebhook", webhookID, userID).Return(&models.Webhook{
		ID:     webhookID,
		UserID: int(userID),
	}, nil)
	suite.webhookSvc.On("UpdateWebhook", mock.AnythingOfType("*models.Webhook")).Return(nil)

	req := httptest.NewRequest(http.MethodPut, fmt.Sprintf("/update-webhook/%s", webhookID), nil)
	chiCtx := chi.NewRouteContext()
	chiCtx.URLParams.Add("id", webhookID)
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, chiCtx))

	rr := httptest.NewRecorder()

	suite.handler.UpdateWebhook(rr, req)

	suite.webhookSvc.AssertExpectations(suite.T())
	suite.authSvc.AssertExpectations(suite.T())
	asserts := assert.New(suite.T())
	asserts.Equal(http.StatusSeeOther, rr.Code)
	asserts.Equal("/?address="+webhookID, rr.Header().Get("Location"))
}

func (suite *WebhookHandlerTestSuite) TestUpdateWebhook_EmptyWebhookID() {
	suite.authSvc.On("Authorize", mock.Anything).Return(uint(1), nil)

	req := httptest.NewRequest(http.MethodPut, "/update-webhook/", nil)
	chiCtx := chi.NewRouteContext()
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, chiCtx))

	rr := httptest.NewRecorder()

	suite.handler.UpdateWebhook(rr, req)

	suite.webhookSvc.AssertExpectations(suite.T())
	suite.authSvc.AssertExpectations(suite.T())
	asserts := assert.New(suite.T())
	asserts.Equal(http.StatusBadRequest, rr.Code)
}

func (suite *WebhookHandlerTestSuite) TestUpdateWebhook_WebhookNotFound() {
	webhookID := "test-webhook-id"
	userID := uint(1)

	suite.authSvc.On("Authorize", mock.Anything).Return(userID, nil)
	suite.webhookSvc.On("GetUserWebhook", webhookID, userID).Return(&models.Webhook{}, errors.New("webhook not found"))

	req := httptest.NewRequest(http.MethodPut, fmt.Sprintf("/update-webhook/%s", webhookID), nil)
	chiCtx := chi.NewRouteContext()
	chiCtx.URLParams.Add("id", webhookID)
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, chiCtx))

	rr := httptest.NewRecorder()

	suite.handler.UpdateWebhook(rr, req)

	suite.webhookSvc.AssertExpectations(suite.T())
	suite.authSvc.AssertExpectations(suite.T())
	asserts := assert.New(suite.T())
	asserts.Equal(http.StatusInternalServerError, rr.Code)
}

func (suite *WebhookHandlerTestSuite) TestUpdateWebhook_AuthError() {
	webhookID := "test-webhook-id"

	suite.authSvc.On("Authorize", mock.Anything).Return(uint(0), errors.New("auth error"))

	req := httptest.NewRequest(http.MethodPut, fmt.Sprintf("/update-webhook/%s", webhookID), nil)
	chiCtx := chi.NewRouteContext()
	chiCtx.URLParams.Add("id", webhookID)
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, chiCtx))

	rr := httptest.NewRecorder()

	suite.handler.UpdateWebhook(rr, req)

	suite.authSvc.AssertExpectations(suite.T())
	asserts := assert.New(suite.T())
	asserts.Equal(http.StatusUnauthorized, rr.Code)
}

func (suite *WebhookHandlerTestSuite) TestUpdateWebhook_FormData() {
	webhookID := "test-webhook-id"
	userID := uint(1)

	suite.authSvc.On("Authorize", mock.Anything).Return(userID, nil)
	suite.webhookSvc.On("GetUserWebhook", mock.Anything, userID).Return(&models.Webhook{}, nil)
	var updated models.Webhook
	suite.webhookSvc.
		On("UpdateWebhook", mock.AnythingOfType("*models.Webhook")).
		Return(nil).
		Run(func(args mock.Arguments) {
			updated = *args.Get(0).(*models.Webhook)
		})

// 330: 	fmt.Printf("updated: %+v\n", updated)
	suite.recorder.On("IncWebhooksUpdated").Return()

	form := url.Values{}
	form.Set("title", "Test Webhook")
	form.Set("content_type", "application/json")
	form.Set("response_code", "200")
	form.Set("response_delay", "100")
	form.Set("payload", "{}")
	form.Set("notify_on_event", "true")
	form.Set("response_headers", `{"X-Foo":"Bar"}`)
	body := form.Encode()

	req := httptest.NewRequest(http.MethodPut, fmt.Sprintf("/update-webhook/%s", webhookID), strings.NewReader(body))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	chiCtx := chi.NewRouteContext()
	chiCtx.URLParams.Add("id", webhookID)
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, chiCtx))

	rr := httptest.NewRecorder()

	suite.handler.UpdateWebhook(rr, req)

	suite.webhookSvc.AssertExpectations(suite.T())
	suite.authSvc.AssertExpectations(suite.T())
	asserts := assert.New(suite.T())
	asserts.Equal(http.StatusSeeOther, rr.Code)
	asserts.Equal("/?address="+webhookID, rr.Header().Get("Location"))
	asserts.Equal("Test Webhook", updated.Title)
	asserts.Equal("application/json", *updated.ContentType)
	asserts.Equal(200, updated.ResponseCode)
	asserts.Equal(uint(100), updated.ResponseDelay)
	asserts.Equal("{}", *updated.Payload)
	asserts.Equal(true, updated.NotifyOnEvent)
	expectedHeaders := datatypes.JSONMap{"X-Foo": "Bar"}
	asserts.Equal(expectedHeaders, updated.ResponseHeaders)
}
