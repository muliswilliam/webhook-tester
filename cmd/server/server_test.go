package server_test

import (
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"webhook-tester/cmd/server"
	webhookdb "webhook-tester/internal/db"
)

// TestMountHandlers exercises the routes of a mounted Server. Each Server
// registers its metrics with its own registry, so it can be mounted again,
// e.g. under go test -count>1.
func TestMountHandlers(t *testing.T) {
	t.Setenv("AUTH_SECRET", "some-32-plus-byte-secret-value!!")
	t.Setenv("DOMAIN", "http://example.com")

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	webhookdb.AutoMigrate(db)

	srv := &server.Server{
		Router:     chi.NewRouter(),
		DB:         db,
		Logger:     log.New(io.Discard, "", 0),
		Srv:        &http.Server{},
		MetricsSrv: &http.Server{},
		Domain:     "http://example.com",
	}

	srv.MountHandlers()
	require.NotNil(t, srv.WebhookSvc)
	require.NotNil(t, srv.Forwarder, "shutdown waits on the forwarder")

	t.Run("health", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/health", nil)
		rec := httptest.NewRecorder()
		srv.Router.ServeHTTP(rec, req)
		require.Equal(t, http.StatusOK, rec.Code)
	})

	t.Run("static file not found", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/static/doesnotexist.txt", nil)
		rec := httptest.NewRecorder()
		srv.Router.ServeHTTP(rec, req)
		require.Equal(t, http.StatusNotFound, rec.Code)
	})

	t.Run("metrics are served only on the metrics server", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
		rec := httptest.NewRecorder()
		srv.MetricsSrv.Handler.ServeHTTP(rec, req)
		require.Equal(t, http.StatusOK, rec.Code)
		for _, name := range []string{"go_goroutines", "webhooks_created_total", "webhook_delivery_duration_seconds", "http_request_duration_seconds"} {
			require.Contains(t, rec.Body.String(), name)
		}

		rec = httptest.NewRecorder()
		srv.Router.ServeHTTP(rec, req)
		require.Equal(t, http.StatusNotFound, rec.Code)
	})

	t.Run("docs", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/docs", nil)
		rec := httptest.NewRecorder()
		srv.Router.ServeHTTP(rec, req)
		require.Equal(t, http.StatusOK, rec.Code)
		require.Contains(t, rec.Body.String(), "API reference - Webhook Tester")
	})

	t.Run("OpenAPI document", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/docs/openapi.json", nil)
		rec := httptest.NewRecorder()
		srv.Router.ServeHTTP(rec, req)
		require.Equal(t, http.StatusOK, rec.Code)
		require.Contains(t, rec.Header().Get("Content-Type"), "application/json")
		require.Contains(t, rec.Body.String(), `"swagger": "2.0"`)
	})

	t.Run("unknown page", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/no-such-page", nil)
		rec := httptest.NewRecorder()
		srv.Router.ServeHTTP(rec, req)
		require.Equal(t, http.StatusNotFound, rec.Code)
		require.Contains(t, rec.Body.String(), "Page not found")
	})
}
