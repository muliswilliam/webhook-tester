package routers_test

import (
	"bytes"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	webhookdb "webhook-tester/internal/db"
	"webhook-tester/internal/dtos"
	appMetrics "webhook-tester/internal/metrics"
	"webhook-tester/internal/routers"
	"webhook-tester/internal/service"
	"webhook-tester/internal/store"
)

func newAPITestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	webhookdb.AutoMigrate(db)
	return db
}

func testLogger() *log.Logger {
	return log.New(io.Discard, "", 0)
}

// setupAPIRouter builds a real API router backed by a fresh sqlite DB and
// returns the router plus the API key of a freshly-registered user.
func setupAPIRouter(t *testing.T) (http.Handler, string) {
	t.Helper()

	db := newAPITestDB(t)
	logger := testLogger()

	userRepo := store.NewGormUserRepo(db, logger)
	webhookRepo := store.NewGormWebookRepo(db, logger)

	authSvc := service.NewAuthService(userRepo, db, "test-auth-secret")
	webhookSvc := service.NewWebhookService(webhookRepo)

	user, err := authSvc.Register("api-user@example.com", "Passw0rd!", "API User")
	require.NoError(t, err)

	metricsRec := &appMetrics.PrometheusRecorder{}

	r := routers.NewApiRouter(webhookSvc, authSvc, logger, metricsRec)

	return r, user.APIKey
}

func TestNewApiRouter_RequiresAPIKey(t *testing.T) {
	r, _ := setupAPIRouter(t)

	req := httptest.NewRequest(http.MethodGet, "/webhooks/", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	require.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestNewApiRouter_PostWebhooksWithoutKey(t *testing.T) {
	r, _ := setupAPIRouter(t)

	body, _ := json.Marshal(dtos.CreateWebhookRequest{Title: "no key"})
	req := httptest.NewRequest(http.MethodPost, "/webhooks/", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	require.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestNewApiRouter_CRUDWithValidKey(t *testing.T) {
	r, apiKey := setupAPIRouter(t)

	// Create
	createBody, _ := json.Marshal(dtos.CreateWebhookRequest{
		Title:        "my webhook",
		ContentType:  "application/json",
		Payload:      `{"ok":true}`,
		ResponseCode: 200,
	})
	req := httptest.NewRequest(http.MethodPost, "/webhooks/", bytes.NewReader(createBody))
	req.Header.Set("X-API-Key", apiKey)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	require.Equal(t, http.StatusCreated, rec.Code)

	var created dtos.Webhook
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &created))
	require.NotEmpty(t, created.ID)

	// List
	req = httptest.NewRequest(http.MethodGet, "/webhooks/", nil)
	req.Header.Set("X-API-Key", apiKey)
	rec = httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)

	// Get by ID
	req = httptest.NewRequest(http.MethodGet, "/webhooks/"+created.ID+"/", nil)
	req.Header.Set("X-API-Key", apiKey)
	rec = httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)

	// Get by ID without key -> unauthorized
	req = httptest.NewRequest(http.MethodGet, "/webhooks/"+created.ID+"/", nil)
	rec = httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	require.Equal(t, http.StatusUnauthorized, rec.Code)

	// Update
	updateBody, _ := json.Marshal(dtos.UpdateWebhookRequest{
		CreateWebhookRequest: dtos.CreateWebhookRequest{Title: "updated title"},
	})
	req = httptest.NewRequest(http.MethodPut, "/webhooks/"+created.ID+"/", bytes.NewReader(updateBody))
	req.Header.Set("X-API-Key", apiKey)
	rec = httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)

	var updated dtos.Webhook
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &updated))
	require.Equal(t, "updated title", updated.Title)

	// Delete
	req = httptest.NewRequest(http.MethodDelete, "/webhooks/"+created.ID+"/", nil)
	req.Header.Set("X-API-Key", apiKey)
	rec = httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	require.Equal(t, http.StatusNoContent, rec.Code)
}

func TestNewApiRouter_InvalidKey(t *testing.T) {
	r, _ := setupAPIRouter(t)

	req := httptest.NewRequest(http.MethodGet, "/webhooks/", nil)
	req.Header.Set("X-API-Key", "not-a-real-key")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	require.Equal(t, http.StatusUnauthorized, rec.Code)
}
