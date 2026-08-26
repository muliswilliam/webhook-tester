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

func TestNewWebhookRouter_UnknownWebhookReturnsNotFound(t *testing.T) {
	db := newAPITestDB(t)
	logger := testLogger()

	userRepo := store.NewGormUserRepo(db, logger)
	webhookRepo := store.NewGormWebookRepo(db, logger)
	webhookReqRepo := store.NewGormWebhookRequestRepo(db, logger)

	authSvc := service.NewAuthService(userRepo, db, "test-auth-secret")
	webhookSvc := service.NewWebhookService(webhookRepo)
	webhookReqSvc := service.NewWebhookRequestService(webhookReqRepo)

	metricsRec := &appMetrics.PrometheusRecorder{}

	r := routers.NewWebhookRouter(webhookSvc, webhookReqSvc, authSvc, logger, metricsRec)

	req := httptest.NewRequest(http.MethodGet, "/some-webhook-id", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	require.Equal(t, http.StatusNotFound, rec.Code)
}
