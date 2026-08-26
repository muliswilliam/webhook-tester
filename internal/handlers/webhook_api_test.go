package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"webhook-tester/internal/dtos"
	"webhook-tester/internal/middlewares"
	"webhook-tester/internal/models"
	"webhook-tester/internal/service"
)

func newTestWebhookApiHandler(t *testing.T) (*WebhookAiHandler, *testWebhookRepo, *testUserRepo, *service.AuthService) {
	t.Helper()
	whRepo := newTestWebhookRepo()
	userRepo := newTestUserRepo()
	authSvc := newTestAuthService(t, userRepo)
	whSvc := service.NewWebhookService(whRepo)

	h := NewWebhookApiHandler(whSvc, &testMetricsRecorder{}, newTestLogger())
	return h, whRepo, userRepo, authSvc
}

// doAPIRequest wraps a handler with the real RequireAPIKey middleware and
// dispatches through a chi router with the given pattern, mirroring the
// production api router wiring.
func doAPIRequest(authSvc *service.AuthService, method, pattern, path, apiKey string, body []byte, handlerFn http.HandlerFunc) *httptest.ResponseRecorder {
	r := chi.NewRouter()
	r.Route(pattern, func(r chi.Router) {
		r.Use(middlewares.RequireAPIKey(authSvc))
		switch method {
		case http.MethodGet:
			r.Get("/", handlerFn)
		case http.MethodPost:
			r.Post("/", handlerFn)
		case http.MethodPut:
			r.Put("/", handlerFn)
		case http.MethodDelete:
			r.Delete("/", handlerFn)
		}
	})

	var reqBody *bytes.Reader
	if body != nil {
		reqBody = bytes.NewReader(body)
	} else {
		reqBody = bytes.NewReader(nil)
	}
	req := httptest.NewRequest(method, path, reqBody)
	if apiKey != "" {
		req.Header.Set("X-API-Key", apiKey)
	}
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	return rec
}

func TestWebhookApiHandler_CreateWebhookApi_Success(t *testing.T) {
	h, whRepo, userRepo, authSvc := newTestWebhookApiHandler(t)
	user := &models.User{Email: "u@x.com", APIKey: "key1"}
	userRepo.addUser(user)

	body, _ := json.Marshal(dtos.CreateWebhookRequest{Title: "hook", ContentType: "application/json", Payload: "{}"})
	rec := doAPIRequest(authSvc, http.MethodPost, "/webhooks", "/webhooks", "key1", body, h.CreateWebhookApi)

	require.Equal(t, http.StatusCreated, rec.Code)
	var got dtos.Webhook
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &got))
	assert.Equal(t, "hook", got.Title)
	assert.Equal(t, http.StatusOK, got.ResponseCode)
	assert.Equal(t, int(user.ID), got.UserID)
	assert.Empty(t, got.Requests)
	assert.Len(t, whRepo.webhooks, 1)
}

func TestWebhookApiHandler_CreateWebhookApi_DecodeError(t *testing.T) {
	h, _, userRepo, authSvc := newTestWebhookApiHandler(t)
	user := &models.User{Email: "u@x.com", APIKey: "key1"}
	userRepo.addUser(user)

	rec := doAPIRequest(authSvc, http.MethodPost, "/webhooks", "/webhooks", "key1", []byte("not-json"), h.CreateWebhookApi)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestWebhookApiHandler_CreateWebhookApi_ServiceError(t *testing.T) {
	h, whRepo, userRepo, authSvc := newTestWebhookApiHandler(t)
	user := &models.User{Email: "u@x.com", APIKey: "key1"}
	userRepo.addUser(user)
	whRepo.insertErr = assert.AnError

	body, _ := json.Marshal(dtos.CreateWebhookRequest{Title: "hook"})
	rec := doAPIRequest(authSvc, http.MethodPost, "/webhooks", "/webhooks", "key1", body, h.CreateWebhookApi)

	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}

func TestWebhookApiHandler_ListWebhooksApi_Success(t *testing.T) {
	h, whRepo, userRepo, authSvc := newTestWebhookApiHandler(t)
	user := &models.User{Email: "u@x.com", APIKey: "key1"}
	userRepo.addUser(user)
	whRepo.put(&models.Webhook{ID: "wh1", UserID: int(user.ID)})
	whRepo.put(&models.Webhook{ID: "wh2", UserID: int(user.ID)})

	rec := doAPIRequest(authSvc, http.MethodGet, "/webhooks", "/webhooks", "key1", nil, h.ListWebhooksApi)

	require.Equal(t, http.StatusOK, rec.Code)
	var got []models.Webhook
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &got))
	assert.Len(t, got, 2)
}

func TestWebhookApiHandler_ListWebhooksApi_Error(t *testing.T) {
	h, whRepo, userRepo, authSvc := newTestWebhookApiHandler(t)
	user := &models.User{Email: "u@x.com", APIKey: "key1"}
	userRepo.addUser(user)
	whRepo.getAllByUserErr = assert.AnError

	rec := doAPIRequest(authSvc, http.MethodGet, "/webhooks", "/webhooks", "key1", nil, h.ListWebhooksApi)

	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}

func TestWebhookApiHandler_GetWebhookApi_Success(t *testing.T) {
	h, whRepo, userRepo, authSvc := newTestWebhookApiHandler(t)
	user := &models.User{Email: "u@x.com", APIKey: "key1"}
	userRepo.addUser(user)
	ct := "application/json"
	pl := "{}"
	whRepo.put(&models.Webhook{ID: "wh1", UserID: int(user.ID), ContentType: &ct, Payload: &pl})

	rec := doAPIRequest(authSvc, http.MethodGet, "/webhooks/{id}", "/webhooks/wh1", "key1", nil, h.GetWebhookApi)

	require.Equal(t, http.StatusOK, rec.Code)
	var got dtos.Webhook
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &got))
	assert.Equal(t, "wh1", got.ID)
}

func TestWebhookApiHandler_GetWebhookApi_NotFound(t *testing.T) {
	h, _, userRepo, authSvc := newTestWebhookApiHandler(t)
	user := &models.User{Email: "u@x.com", APIKey: "key1"}
	userRepo.addUser(user)

	rec := doAPIRequest(authSvc, http.MethodGet, "/webhooks/{id}", "/webhooks/missing", "key1", nil, h.GetWebhookApi)

	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func TestWebhookApiHandler_GetWebhookApi_ServiceError(t *testing.T) {
	h, whRepo, userRepo, authSvc := newTestWebhookApiHandler(t)
	user := &models.User{Email: "u@x.com", APIKey: "key1"}
	userRepo.addUser(user)
	whRepo.getByUserErr = assert.AnError

	rec := doAPIRequest(authSvc, http.MethodGet, "/webhooks/{id}", "/webhooks/wh1", "key1", nil, h.GetWebhookApi)

	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}

func TestWebhookApiHandler_UpdateWebhookApi_GetError(t *testing.T) {
	h, _, userRepo, authSvc := newTestWebhookApiHandler(t)
	user := &models.User{Email: "u@x.com", APIKey: "key1"}
	userRepo.addUser(user)

	body, _ := json.Marshal(dtos.UpdateWebhookRequest{})
	rec := doAPIRequest(authSvc, http.MethodPut, "/webhooks/{id}", "/webhooks/missing", "key1", body, h.UpdateWebhookApi)

	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}

func TestWebhookApiHandler_UpdateWebhookApi_DecodeError(t *testing.T) {
	h, whRepo, userRepo, authSvc := newTestWebhookApiHandler(t)
	user := &models.User{Email: "u@x.com", APIKey: "key1"}
	userRepo.addUser(user)
	ct := "text/plain"
	pl := "old"
	whRepo.put(&models.Webhook{ID: "wh1", UserID: int(user.ID), ContentType: &ct, Payload: &pl})

	rec := doAPIRequest(authSvc, http.MethodPut, "/webhooks/{id}", "/webhooks/wh1", "key1", []byte("not-json"), h.UpdateWebhookApi)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestWebhookApiHandler_UpdateWebhookApi_Success(t *testing.T) {
	h, whRepo, userRepo, authSvc := newTestWebhookApiHandler(t)
	user := &models.User{Email: "u@x.com", APIKey: "key1"}
	userRepo.addUser(user)
	ct := "text/plain"
	pl := "old-payload"
	whRepo.put(&models.Webhook{
		ID: "wh1", UserID: int(user.ID), Title: "old-title", ResponseCode: 200,
		ResponseDelay: 10, ContentType: &ct, Payload: &pl, NotifyOnEvent: false,
	})

	input := dtos.UpdateWebhookRequest{CreateWebhookRequest: dtos.CreateWebhookRequest{
		Title:         "new-title",
		ResponseCode:  201,
		ResponseDelay: 0,  // unchanged since 0 means "not set" per handler logic
		ContentType:   "", // empty -> unchanged
		Payload:       "", // empty -> unchanged
		NotifyOnEvent: true,
	}}
	body, _ := json.Marshal(input)
	rec := doAPIRequest(authSvc, http.MethodPut, "/webhooks/{id}", "/webhooks/wh1", "key1", body, h.UpdateWebhookApi)

	require.Equal(t, http.StatusOK, rec.Code)
	w := whRepo.webhooks["wh1"]
	require.NotNil(t, w)
	assert.Equal(t, "new-title", w.Title)
	assert.Equal(t, 201, w.ResponseCode)
	assert.Equal(t, uint(10), w.ResponseDelay) // unchanged: input was 0
	// Empty-string input leaves ContentType/Payload unchanged.
	require.NotNil(t, w.ContentType)
	assert.Equal(t, "text/plain", *w.ContentType)
	require.NotNil(t, w.Payload)
	assert.Equal(t, "old-payload", *w.Payload)
	assert.True(t, w.NotifyOnEvent)
}

func TestWebhookApiHandler_UpdateWebhookApi_UnchangedFieldsAndContentTypeChange(t *testing.T) {
	h, whRepo, userRepo, authSvc := newTestWebhookApiHandler(t)
	user := &models.User{Email: "u@x.com", APIKey: "key1"}
	userRepo.addUser(user)
	ct := "text/plain"
	pl := "old-payload"
	whRepo.put(&models.Webhook{
		ID: "wh1", UserID: int(user.ID), Title: "same-title", ResponseCode: 200,
		ResponseDelay: 10, ContentType: &ct, Payload: &pl, NotifyOnEvent: true,
	})

	input := dtos.UpdateWebhookRequest{CreateWebhookRequest: dtos.CreateWebhookRequest{
		Title:         "same-title",       // unchanged: equal to current value
		ResponseCode:  200,                // unchanged: equal to current value
		ResponseDelay: 10,                 // unchanged: equal to current value
		ContentType:   "application/json", // non-empty and different -> updates
		NotifyOnEvent: true,               // unchanged: equal to current value
	}}
	body, _ := json.Marshal(input)
	rec := doAPIRequest(authSvc, http.MethodPut, "/webhooks/{id}", "/webhooks/wh1", "key1", body, h.UpdateWebhookApi)

	require.Equal(t, http.StatusOK, rec.Code)
	w := whRepo.webhooks["wh1"]
	require.NotNil(t, w)
	assert.Equal(t, "same-title", w.Title)
	assert.Equal(t, 200, w.ResponseCode)
	assert.Equal(t, uint(10), w.ResponseDelay)
	require.NotNil(t, w.ContentType)
	assert.Equal(t, "application/json", *w.ContentType)
	assert.True(t, w.NotifyOnEvent)
}

func TestWebhookApiHandler_UpdateWebhookApi_ResponseDelayChange(t *testing.T) {
	h, whRepo, userRepo, authSvc := newTestWebhookApiHandler(t)
	user := &models.User{Email: "u@x.com", APIKey: "key1"}
	userRepo.addUser(user)
	ct := "text/plain"
	pl := "old"
	whRepo.put(&models.Webhook{ID: "wh1", UserID: int(user.ID), ResponseDelay: 5, ContentType: &ct, Payload: &pl})

	input := dtos.UpdateWebhookRequest{CreateWebhookRequest: dtos.CreateWebhookRequest{ResponseDelay: 50}}
	body, _ := json.Marshal(input)
	rec := doAPIRequest(authSvc, http.MethodPut, "/webhooks/{id}", "/webhooks/wh1", "key1", body, h.UpdateWebhookApi)

	require.Equal(t, http.StatusOK, rec.Code)
	w := whRepo.webhooks["wh1"]
	require.NotNil(t, w)
	assert.Equal(t, uint(50), w.ResponseDelay)
}

func TestWebhookApiHandler_UpdateWebhookApi_ServiceError(t *testing.T) {
	h, whRepo, userRepo, authSvc := newTestWebhookApiHandler(t)
	user := &models.User{Email: "u@x.com", APIKey: "key1"}
	userRepo.addUser(user)
	ct := "text/plain"
	pl := "old"
	whRepo.put(&models.Webhook{ID: "wh1", UserID: int(user.ID), ContentType: &ct, Payload: &pl})
	whRepo.updateErr = assert.AnError

	body, _ := json.Marshal(dtos.UpdateWebhookRequest{})
	rec := doAPIRequest(authSvc, http.MethodPut, "/webhooks/{id}", "/webhooks/wh1", "key1", body, h.UpdateWebhookApi)

	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}

func TestWebhookApiHandler_DeleteWebhookApi_Success(t *testing.T) {
	h, whRepo, userRepo, authSvc := newTestWebhookApiHandler(t)
	user := &models.User{Email: "u@x.com", APIKey: "key1"}
	userRepo.addUser(user)
	whRepo.put(&models.Webhook{ID: "wh1", UserID: int(user.ID)})

	rec := doAPIRequest(authSvc, http.MethodDelete, "/webhooks/{id}", "/webhooks/wh1", "key1", nil, h.DeleteWebhookApi)

	assert.Equal(t, http.StatusNoContent, rec.Code)
	assert.Len(t, whRepo.webhooks, 0)
}

func TestWebhookApiHandler_DeleteWebhookApi_Error(t *testing.T) {
	h, whRepo, userRepo, authSvc := newTestWebhookApiHandler(t)
	user := &models.User{Email: "u@x.com", APIKey: "key1"}
	userRepo.addUser(user)
	whRepo.deleteErr = assert.AnError

	rec := doAPIRequest(authSvc, http.MethodDelete, "/webhooks/{id}", "/webhooks/wh1", "key1", nil, h.DeleteWebhookApi)

	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}
