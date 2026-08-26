package routers_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"

	appMetrics "webhook-tester/internal/metrics"
	"webhook-tester/internal/routers"
	"webhook-tester/internal/service"
	"webhook-tester/internal/store"
)

// setupWebRouter builds a real web router backed by a fresh sqlite DB.
// AUTH_SECRET/DOMAIN must be set before NewWebRouter is called since it
// reads them at construction time.
func setupWebRouter(t *testing.T) http.Handler {
	t.Helper()

	t.Setenv("AUTH_SECRET", "some-32-plus-byte-secret-value!!")
	t.Setenv("DOMAIN", "http://example.com")

	db := newAPITestDB(t)
	logger := testLogger()

	userRepo := store.NewGormUserRepo(db, logger)
	webhookRepo := store.NewGormWebookRepo(db, logger)
	webhookReqRepo := store.NewGormWebhookRequestRepo(db, logger)

	authSvc := service.NewAuthService(userRepo, db, "some-32-plus-byte-secret-value!!")
	webhookSvc := service.NewWebhookService(webhookRepo)
	webhookReqSvc := service.NewWebhookRequestService(webhookReqRepo)

	metricsRec := &appMetrics.PrometheusRecorder{}

	return routers.NewWebRouter(webhookReqSvc, webhookSvc, authSvc, metricsRec, logger)
}

func TestNewWebRouter_PlainHTTPRequest(t *testing.T) {
	r := setupWebRouter(t)

	req := httptest.NewRequest(http.MethodGet, "/login", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
}

func TestNewWebRouter_ForwardedHTTPSRequest(t *testing.T) {
	r := setupWebRouter(t)

	req := httptest.NewRequest(http.MethodGet, "/login", nil)
	req.Header.Set("X-Forwarded-Proto", "https")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
}

func TestNewWebRouter_CSRFRejectsMissingToken(t *testing.T) {
	r := setupWebRouter(t)

	req := httptest.NewRequest(http.MethodPost, "/login", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	require.Equal(t, http.StatusForbidden, rec.Code)
}

func TestNewWebRouter_GetRoutesDispatch(t *testing.T) {
	r := setupWebRouter(t)

	routes := []string{
		"/",
		"/register",
		"/login",
		"/forgot-password",
		"/reset-password",
		"/privacy",
		"/terms",
	}

	for _, route := range routes {
		t.Run(route, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, route, nil)
			rec := httptest.NewRecorder()
			r.ServeHTTP(rec, req)

			require.Equal(t, http.StatusOK, rec.Code, "route %s should dispatch successfully", route)
		})
	}
}
