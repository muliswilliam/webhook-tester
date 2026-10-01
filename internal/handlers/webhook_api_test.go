package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/datatypes"

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
	whSvc := service.NewWebhookService(whRepo, &testDeliveryRepo{})

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

func TestWebhookApiHandler_CreateWebhookApi_Defaults(t *testing.T) {
	h, whRepo, userRepo, authSvc := newTestWebhookApiHandler(t)
	userRepo.addUser(&models.User{Email: "u@x.com", APIKey: "key1"})

	body := []byte(`{"title":"hook","response_headers":{"X-Api":"1"}}`)
	rec := doAPIRequest(authSvc, http.MethodPost, "/webhooks", "/webhooks", "key1", body, h.CreateWebhookApi)

	require.Equal(t, http.StatusCreated, rec.Code)
	var got dtos.Webhook
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &got))
	assert.Equal(t, "application/json", got.ContentType)
	assert.Equal(t, map[string]string{"X-Api": "1"}, got.ResponseHeaders)
	assert.Equal(t, "1", whRepo.webhooks[got.ID].ResponseHeaders["X-Api"])
}

func TestWebhookApiHandler_CreateWebhookApi_Invalid(t *testing.T) {
	h, whRepo, userRepo, authSvc := newTestWebhookApiHandler(t)
	userRepo.addUser(&models.User{Email: "u@x.com", APIKey: "key1"})

	cases := map[string]string{
		"missing title":     `{}`,
		"code too high":     `{"title":"x","response_code":1000}`,
		"code too low":      `{"title":"x","response_code":42}`,
		"delay too long":    `{"title":"x","response_delay":30001}`,
		"bad header name":   `{"title":"x","response_headers":{"Bad Name":"1"}}`,
		"managed header":    `{"title":"x","response_headers":{"content-length":"1"}}`,
		"header line break": `{"title":"x","response_headers":{"X-A":"a\r\nX-B: b"}}`,
	}
	for name, body := range cases {
		t.Run(name, func(t *testing.T) {
			rec := doAPIRequest(authSvc, http.MethodPost, "/webhooks", "/webhooks", "key1", []byte(body), h.CreateWebhookApi)
			assert.Equal(t, http.StatusBadRequest, rec.Code)
			var got dtos.ErrorResponse
			require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &got))
			assert.NotEmpty(t, got.Error)
		})
	}
	assert.Empty(t, whRepo.webhooks)
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
	var got []dtos.Webhook
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &got))
	assert.Len(t, got, 2)
	for _, wh := range got {
		assert.NotNil(t, wh.Requests, "requests render as [], not null")
		assert.NotNil(t, wh.ResponseHeaders, "response_headers render as {}, not null")
	}
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
	whRepo.put(&models.Webhook{
		ID: "wh1", UserID: int(user.ID), ContentType: &ct, Payload: &pl,
		ResponseHeaders: datatypes.JSONMap{"X-Test": "1"},
		CreatedAt:       time.Date(2026, 1, 2, 15, 4, 5, 0, time.FixedZone("EAT", 3*60*60)),
		Requests:        []models.WebhookRequest{{ID: "req1", WebhookID: "wh1", Method: "POST", Path: "/orders"}},
	})

	rec := doAPIRequest(authSvc, http.MethodGet, "/webhooks/{id}", "/webhooks/wh1", "key1", nil, h.GetWebhookApi)

	require.Equal(t, http.StatusOK, rec.Code)
	var got dtos.Webhook
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &got))
	assert.Equal(t, "wh1", got.ID)
	assert.Equal(t, map[string]string{"X-Test": "1"}, got.ResponseHeaders)
	require.Len(t, got.Requests, 1)
	assert.Equal(t, "/orders", got.Requests[0].Path)
	assert.Contains(t, rec.Body.String(), `"created_at":"2026-01-02T12:04:05Z"`, "timestamps are UTC")
}

func TestWebhookApiHandler_GetWebhookApi_OtherUsersWebhook(t *testing.T) {
	h, whRepo, userRepo, authSvc := newTestWebhookApiHandler(t)
	userRepo.addUser(&models.User{Email: "u@x.com", APIKey: "key1"})
	whRepo.put(&models.Webhook{ID: "guest"})
	whRepo.put(&models.Webhook{ID: "theirs", UserID: 99})

	for _, id := range []string{"guest", "theirs"} {
		rec := doAPIRequest(authSvc, http.MethodGet, "/webhooks/{id}", "/webhooks/"+id, "key1", nil, h.GetWebhookApi)
		assert.Equal(t, http.StatusNotFound, rec.Code, id)
	}
}

func TestWebhookApiHandler_GetWebhookApi_NotFound(t *testing.T) {
	h, _, userRepo, authSvc := newTestWebhookApiHandler(t)
	user := &models.User{Email: "u@x.com", APIKey: "key1"}
	userRepo.addUser(user)

	rec := doAPIRequest(authSvc, http.MethodGet, "/webhooks/{id}", "/webhooks/missing", "key1", nil, h.GetWebhookApi)

	assert.Equal(t, http.StatusNotFound, rec.Code)
	assert.JSONEq(t, `{"error":"webhook not found"}`, rec.Body.String())
}

func TestWebhookApiHandler_GetWebhookApi_ServiceError(t *testing.T) {
	h, whRepo, userRepo, authSvc := newTestWebhookApiHandler(t)
	user := &models.User{Email: "u@x.com", APIKey: "key1"}
	userRepo.addUser(user)
	whRepo.getWithRequestsErr = assert.AnError

	rec := doAPIRequest(authSvc, http.MethodGet, "/webhooks/{id}", "/webhooks/wh1", "key1", nil, h.GetWebhookApi)

	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}

func TestWebhookApiHandler_UpdateWebhookApi_NotFound(t *testing.T) {
	h, _, userRepo, authSvc := newTestWebhookApiHandler(t)
	user := &models.User{Email: "u@x.com", APIKey: "key1"}
	userRepo.addUser(user)

	rec := doAPIRequest(authSvc, http.MethodPut, "/webhooks/{id}", "/webhooks/missing", "key1", []byte(`{}`), h.UpdateWebhookApi)

	assert.Equal(t, http.StatusNotFound, rec.Code)
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

func TestWebhookApiHandler_UpdateWebhookApi_ChangesOnlyIncludedFields(t *testing.T) {
	h, whRepo, userRepo, authSvc := newTestWebhookApiHandler(t)
	user := &models.User{Email: "u@x.com", APIKey: "key1"}
	userRepo.addUser(user)
	ct := "text/plain"
	pl := "old-payload"
	whRepo.put(&models.Webhook{
		ID: "wh1", UserID: int(user.ID), Title: "old-title", ResponseCode: 200,
		ResponseDelay: 10, ContentType: &ct, Payload: &pl, NotifyOnEvent: true,
		ResponseHeaders: datatypes.JSONMap{"X-Old": "1"},
	})

	body := []byte(`{"title":"new-title","response_code":201}`)
	rec := doAPIRequest(authSvc, http.MethodPut, "/webhooks/{id}", "/webhooks/wh1", "key1", body, h.UpdateWebhookApi)

	require.Equal(t, http.StatusOK, rec.Code)
	var got dtos.Webhook
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &got))
	assert.Equal(t, "new-title", got.Title)
	w := whRepo.webhooks["wh1"]
	assert.Equal(t, "new-title", w.Title)
	assert.Equal(t, 201, w.ResponseCode)
	assert.Equal(t, uint(10), w.ResponseDelay)
	assert.Equal(t, "text/plain", *w.ContentType)
	assert.Equal(t, "old-payload", *w.Payload)
	assert.True(t, w.NotifyOnEvent)
	assert.Equal(t, "1", w.ResponseHeaders["X-Old"])
}

func TestWebhookApiHandler_UpdateWebhookApi_SetsZeroValues(t *testing.T) {
	h, whRepo, userRepo, authSvc := newTestWebhookApiHandler(t)
	user := &models.User{Email: "u@x.com", APIKey: "key1"}
	userRepo.addUser(user)
	ct := "text/plain"
	pl := "old"
	whRepo.put(&models.Webhook{
		ID: "wh1", UserID: int(user.ID), Title: "t", ResponseCode: 200, ResponseDelay: 5,
		ContentType: &ct, Payload: &pl, NotifyOnEvent: true, ResponseHeaders: datatypes.JSONMap{"X-Old": "1"},
	})

	body := []byte(`{"response_delay":0,"payload":"","notify_on_event":false,"response_headers":{},"content_type":""}`)
	rec := doAPIRequest(authSvc, http.MethodPut, "/webhooks/{id}", "/webhooks/wh1", "key1", body, h.UpdateWebhookApi)

	require.Equal(t, http.StatusOK, rec.Code)
	w := whRepo.webhooks["wh1"]
	assert.Equal(t, uint(0), w.ResponseDelay)
	assert.Equal(t, "", *w.Payload)
	assert.False(t, w.NotifyOnEvent)
	assert.Empty(t, w.ResponseHeaders)
	assert.Equal(t, "application/json", *w.ContentType, "an empty content type resets to the default")
}

func TestWebhookApiHandler_UpdateWebhookApi_Invalid(t *testing.T) {
	h, whRepo, userRepo, authSvc := newTestWebhookApiHandler(t)
	user := &models.User{Email: "u@x.com", APIKey: "key1"}
	userRepo.addUser(user)
	ct := "text/plain"
	pl := "old"
	whRepo.put(&models.Webhook{ID: "wh1", UserID: int(user.ID), Title: "t", ResponseCode: 200, ContentType: &ct, Payload: &pl})

	for _, body := range []string{`{"response_code":5000}`, `{"response_code":0}`, `{"title":"  "}`, `{"response_headers":{"Bad Name":"x"}}`} {
		rec := doAPIRequest(authSvc, http.MethodPut, "/webhooks/{id}", "/webhooks/wh1", "key1", []byte(body), h.UpdateWebhookApi)
		assert.Equal(t, http.StatusBadRequest, rec.Code, body)
	}
	assert.Equal(t, 200, whRepo.webhooks["wh1"].ResponseCode)
}

func TestWebhookApiHandler_UpdateWebhookApi_ServiceError(t *testing.T) {
	h, whRepo, userRepo, authSvc := newTestWebhookApiHandler(t)
	user := &models.User{Email: "u@x.com", APIKey: "key1"}
	userRepo.addUser(user)
	ct := "text/plain"
	pl := "old"
	whRepo.put(&models.Webhook{ID: "wh1", UserID: int(user.ID), Title: "t", ResponseCode: 200, ContentType: &ct, Payload: &pl})
	whRepo.updateErr = assert.AnError

	rec := doAPIRequest(authSvc, http.MethodPut, "/webhooks/{id}", "/webhooks/wh1", "key1", []byte(`{}`), h.UpdateWebhookApi)

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

func TestWebhookApiHandler_DeleteWebhookApi_NotFound(t *testing.T) {
	h, _, userRepo, authSvc := newTestWebhookApiHandler(t)
	userRepo.addUser(&models.User{Email: "u@x.com", APIKey: "key1"})

	rec := doAPIRequest(authSvc, http.MethodDelete, "/webhooks/{id}", "/webhooks/missing", "key1", nil, h.DeleteWebhookApi)

	assert.Equal(t, http.StatusNotFound, rec.Code)
	assert.JSONEq(t, `{"error":"webhook not found"}`, rec.Body.String())
}

func TestWebhookApiHandler_DeleteWebhookApi_Error(t *testing.T) {
	h, whRepo, userRepo, authSvc := newTestWebhookApiHandler(t)
	user := &models.User{Email: "u@x.com", APIKey: "key1"}
	userRepo.addUser(user)
	whRepo.deleteErr = assert.AnError

	rec := doAPIRequest(authSvc, http.MethodDelete, "/webhooks/{id}", "/webhooks/wh1", "key1", nil, h.DeleteWebhookApi)

	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}
