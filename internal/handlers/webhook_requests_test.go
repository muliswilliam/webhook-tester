package handlers

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/datatypes"

	"webhook-tester/internal/metrics"
	"webhook-tester/internal/models"
	"webhook-tester/internal/service"
)

func newTestWebhookRequestHandler(t *testing.T) (*WebhookRequestHandler, *testWebhookRequestRepo, *testWebhookRepo, *testUserRepo, *service.AuthService) {
	t.Helper()
	reqRepo := newTestWebhookRequestRepo()
	whRepo := newTestWebhookRepo()
	userRepo := newTestUserRepo()
	authSvc := newTestAuthService(t, userRepo)

	reqSvc := service.NewWebhookRequestService(reqRepo)
	whSvc := service.NewWebhookService(whRepo)

	var rec metrics.Recorder = &testMetricsRecorder{}
	h := NewWebhookRequestHandler(reqSvc, authSvc, whSvc, &rec, newTestLogger())
	return h, reqRepo, whRepo, userRepo, authSvc
}

func TestWebhookRequestHandler_GetRequest_WebhookLoadError(t *testing.T) {
	h, _, _, _, _ := newTestWebhookRequestHandler(t)

	router := chi.NewRouter()
	router.Get("/requests/{id}", h.GetRequest)

	req := httptest.NewRequest(http.MethodGet, "/requests/r1?address=missing", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}

func TestWebhookRequestHandler_GetRequest_RequestNotFound(t *testing.T) {
	h, _, whRepo, _, _ := newTestWebhookRequestHandler(t)
	whRepo.put(&models.Webhook{ID: "wh1"})

	router := chi.NewRouter()
	router.Get("/requests/{id}", h.GetRequest)

	req := httptest.NewRequest(http.MethodGet, "/requests/missing?address=wh1", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func TestWebhookRequestHandler_GetRequest_Guest(t *testing.T) {
	h, reqRepo, whRepo, _, _ := newTestWebhookRequestHandler(t)
	whRepo.put(&models.Webhook{ID: "wh1", Title: "Hook1"})
	reqRepo.put(&models.WebhookRequest{ID: "r1", WebhookID: "wh1", Method: "GET", Headers: datatypes.JSONMap{}})

	router := chi.NewRouter()
	router.Get("/requests/{id}", h.GetRequest)

	req := httptest.NewRequest(http.MethodGet, "/requests/r1?address=wh1", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), "Hook1")
}

func TestWebhookRequestHandler_GetRequest_LoggedInUser(t *testing.T) {
	h, reqRepo, whRepo, userRepo, authSvc := newTestWebhookRequestHandler(t)
	user := &models.User{Email: "jane@x.com"}
	userRepo.addUser(user)
	whRepo.put(&models.Webhook{ID: "wh1", Title: "Hook1", UserID: int(user.ID)})
	whRepo.put(&models.Webhook{ID: "wh2", Title: "Hook2", UserID: int(user.ID)})
	reqRepo.put(&models.WebhookRequest{ID: "r1", WebhookID: "wh1", Method: "GET", Headers: datatypes.JSONMap{}})

	cookie := sessionCookieFor(t, authSvc, user)
	router := chi.NewRouter()
	router.Get("/requests/{id}", h.GetRequest)

	req := httptest.NewRequest(http.MethodGet, "/requests/r1?address=wh1", nil)
	req.AddCookie(cookie)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestWebhookRequestHandler_GetRequest_LoggedInUser_ListError(t *testing.T) {
	h, reqRepo, whRepo, userRepo, authSvc := newTestWebhookRequestHandler(t)
	user := &models.User{Email: "jane@x.com"}
	userRepo.addUser(user)
	whRepo.put(&models.Webhook{ID: "wh1", UserID: int(user.ID)})
	reqRepo.put(&models.WebhookRequest{ID: "r1", WebhookID: "wh1"})
	whRepo.getAllByUserErr = assert.AnError

	cookie := sessionCookieFor(t, authSvc, user)
	router := chi.NewRouter()
	router.Get("/requests/{id}", h.GetRequest)

	req := httptest.NewRequest(http.MethodGet, "/requests/r1?address=wh1", nil)
	req.AddCookie(cookie)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}

func TestWebhookRequestHandler_DeleteRequest_WithReferer(t *testing.T) {
	h, reqRepo, _, _, _ := newTestWebhookRequestHandler(t)
	reqRepo.put(&models.WebhookRequest{ID: "r1", WebhookID: "wh1"})

	router := chi.NewRouter()
	router.Post("/requests/{id}/delete", h.DeleteRequest)

	req := httptest.NewRequest(http.MethodPost, "/requests/r1/delete", nil)
	req.Header.Set("Referer", "/?address=wh1")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusFound, rec.Code)
	assert.Equal(t, "/?address=wh1", rec.Header().Get("Location"))
	_, ok := reqRepo.requests["r1"]
	assert.False(t, ok)
}

func TestWebhookRequestHandler_DeleteRequest_NoReferer(t *testing.T) {
	h, reqRepo, _, _, _ := newTestWebhookRequestHandler(t)
	reqRepo.put(&models.WebhookRequest{ID: "r1", WebhookID: "wh1"})

	router := chi.NewRouter()
	router.Post("/requests/{id}/delete", h.DeleteRequest)

	req := httptest.NewRequest(http.MethodPost, "/requests/r1/delete", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusFound, rec.Code)
	assert.Equal(t, "/", rec.Header().Get("Location"))
}

func TestWebhookRequestHandler_DeleteRequest_ServiceError(t *testing.T) {
	h, reqRepo, _, _, _ := newTestWebhookRequestHandler(t)
	reqRepo.deleteByIDErr = assert.AnError

	router := chi.NewRouter()
	router.Post("/requests/{id}/delete", h.DeleteRequest)

	req := httptest.NewRequest(http.MethodPost, "/requests/r1/delete", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}

func TestWebhookRequestHandler_ReplayRequest_NotFound(t *testing.T) {
	h, _, _, _, _ := newTestWebhookRequestHandler(t)

	router := chi.NewRouter()
	router.Post("/requests/{id}/replay", h.ReplayRequest)

	req := httptest.NewRequest(http.MethodPost, "/requests/missing/replay", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func TestWebhookRequestHandler_ReplayRequest_InvalidMethod(t *testing.T) {
	h, reqRepo, _, _, _ := newTestWebhookRequestHandler(t)
	reqRepo.put(&models.WebhookRequest{ID: "r1", WebhookID: "wh1", Method: "BAD METHOD", Body: ""})
	t.Setenv("DOMAIN", "http://example.com")

	router := chi.NewRouter()
	router.Post("/requests/{id}/replay", h.ReplayRequest)

	req := httptest.NewRequest(http.MethodPost, "/requests/r1/replay", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}

func TestWebhookRequestHandler_ReplayRequest_NetworkError(t *testing.T) {
	h, reqRepo, _, _, _ := newTestWebhookRequestHandler(t)
	reqRepo.put(&models.WebhookRequest{ID: "r1", WebhookID: "wh1", Method: http.MethodGet})

	closedServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	closedServer.Close()
	t.Setenv("DOMAIN", closedServer.URL)

	router := chi.NewRouter()
	router.Post("/requests/{id}/replay", h.ReplayRequest)

	req := httptest.NewRequest(http.MethodPost, "/requests/r1/replay", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadGateway, rec.Code)
}

func TestWebhookRequestHandler_ReplayRequest_InvalidDomain(t *testing.T) {
	h, reqRepo, _, _, _ := newTestWebhookRequestHandler(t)
	reqRepo.put(&models.WebhookRequest{ID: "r1", WebhookID: "wh1", Method: http.MethodGet})
	t.Setenv("DOMAIN", "://bad")

	router := chi.NewRouter()
	router.Post("/requests/{id}/replay", h.ReplayRequest)

	req := httptest.NewRequest(http.MethodPost, "/requests/r1/replay", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}

func TestWebhookRequestHandler_ReplayRequest_Success(t *testing.T) {
	h, reqRepo, _, _, _ := newTestWebhookRequestHandler(t)

	var gotPath, gotQuery, gotHeader string
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotQuery = r.URL.RawQuery
		gotHeader = r.Header.Get("X-Original")
		w.WriteHeader(http.StatusOK)
	}))
	defer target.Close()
	t.Setenv("DOMAIN", target.URL)

	reqRepo.put(&models.WebhookRequest{
		ID:        "r1",
		WebhookID: "wh1",
		Method:    http.MethodGet,
		Query:     datatypes.JSONMap{"foo": "bar"},
		Headers:   datatypes.JSONMap{"X-Original": "yes"},
		Body:      "",
	})

	router := chi.NewRouter()
	router.Post("/requests/{id}/replay", h.ReplayRequest)

	req := httptest.NewRequest(http.MethodPost, "/requests/r1/replay", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusSeeOther, rec.Code)
	assert.Equal(t, "/requests/r1?address=wh1", rec.Header().Get("Location"))
	assert.Equal(t, "/webhooks/wh1", gotPath)
	assert.Equal(t, "foo=bar", gotQuery)
	assert.Equal(t, "yes", gotHeader)
}
