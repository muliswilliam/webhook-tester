package routers_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"webhook-tester/config"
	appMetrics "webhook-tester/internal/metrics"
	"webhook-tester/internal/models"
	"webhook-tester/internal/routers"
	"webhook-tester/internal/service"
	"webhook-tester/internal/store"
)

func newTestWebhookRouter(t *testing.T) (http.Handler, *gorm.DB) {
	t.Helper()
	db := newAPITestDB(t)
	logger := testLogger()
	authSvc := service.NewAuthService(store.NewGormUserRepo(db, logger), db, "test-auth-secret")
	webhookSvc := service.NewWebhookService(store.NewGormWebookRepo(db, logger), store.NewGormDeliveryRepo(db, logger), testDomain, testForwardPolicy)
	webhookReqSvc := service.NewWebhookRequestService(store.NewGormWebhookRequestRepo(db, logger))
	forwarder := newTestForwarder(t, webhookSvc, config.Forwarding{})
	return routers.NewWebhookRouter(webhookSvc, webhookReqSvc, authSvc, forwarder, logger, &appMetrics.PrometheusRecorder{}), db
}

func TestNewWebhookRouter_CapturesSubpaths(t *testing.T) {
	r, db := newTestWebhookRouter(t)
	require.NoError(t, db.Create(&models.Webhook{ID: "wh1", ResponseCode: http.StatusAccepted}).Error)

	cases := map[string]string{
		"/wh1":                "",
		"/wh1/":               "/",
		"/wh1/orders/42?x=1":  "/orders/42",
		"/wh1/orders/42/":     "/orders/42/",
		"/wh1/a/b/c/d?q=true": "/a/b/c/d",
	}
	for target, wantPath := range cases {
		t.Run(target, func(t *testing.T) {
			require.NoError(t, db.Where("1 = 1").Delete(&models.WebhookRequest{}).Error)

			req := httptest.NewRequest(http.MethodPost, target, nil)
			rec := httptest.NewRecorder()
			r.ServeHTTP(rec, req)
			require.Equal(t, http.StatusAccepted, rec.Code)

			var captured models.WebhookRequest
			require.NoError(t, db.First(&captured, "webhook_id = ?", "wh1").Error)
			require.Equal(t, wantPath, captured.Path)
		})
	}
}

func TestNewWebhookRouter_UnknownWebhookReturnsNotFound(t *testing.T) {
	db := newAPITestDB(t)
	logger := testLogger()

	userRepo := store.NewGormUserRepo(db, logger)
	webhookRepo := store.NewGormWebookRepo(db, logger)
	webhookReqRepo := store.NewGormWebhookRequestRepo(db, logger)

	authSvc := service.NewAuthService(userRepo, db, "test-auth-secret")
	webhookSvc := service.NewWebhookService(webhookRepo, store.NewGormDeliveryRepo(db, logger), testDomain, testForwardPolicy)
	webhookReqSvc := service.NewWebhookRequestService(webhookReqRepo)

	metricsRec := &appMetrics.PrometheusRecorder{}

	forwarder := newTestForwarder(t, webhookSvc, config.Forwarding{})
	r := routers.NewWebhookRouter(webhookSvc, webhookReqSvc, authSvc, forwarder, logger, metricsRec)

	req := httptest.NewRequest(http.MethodGet, "/some-webhook-id", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	require.Equal(t, http.StatusNotFound, rec.Code)
}
