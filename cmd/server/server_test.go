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

// TestMountHandlers exercises Server.MountHandlers once for the whole test
// binary. MountHandlers registers prometheus collectors via
// go-http-metrics/metrics/prometheus, and registering the same collectors
// twice in one process panics - so all assertions live in one test with
// sub-checks sharing a single mounted Server.
func TestMountHandlers(t *testing.T) {
	t.Setenv("AUTH_SECRET", "some-32-plus-byte-secret-value!!")
	t.Setenv("DOMAIN", "http://example.com")

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	webhookdb.AutoMigrate(db)

	srv := &server.Server{
		Router: chi.NewRouter(),
		DB:     db,
		Logger: log.New(io.Discard, "", 0),
		Srv:    &http.Server{},
	}

	srv.MountHandlers()

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

	t.Run("metrics", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
		rec := httptest.NewRecorder()
		srv.Router.ServeHTTP(rec, req)
		require.Equal(t, http.StatusOK, rec.Code)
	})

	t.Run("docs", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/docs", nil)
		rec := httptest.NewRecorder()
		srv.Router.ServeHTTP(rec, req)
		require.Equal(t, http.StatusOK, rec.Code)
	})
}
