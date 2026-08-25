package routers

import (
	"github.com/go-chi/chi/v5"
	"log"
	"net/http"
	"webhook-tester/internal/handlers"
	"webhook-tester/internal/metrics"
	"webhook-tester/internal/service"
)

func NewWebhookRouter(
	webhookSvc *service.WebhookService,
	webhookReqSvc *service.WebhookRequestService,
	authSvc *service.AuthService,
	logger *log.Logger,
	metrics metrics.Recorder,
) http.Handler {
	r := chi.NewRouter()
	wh := handlers.NewWebhookHandler(webhookSvc, webhookReqSvc, authSvc, logger, metrics)

	// Match all HTTP methods at /{webhookID}
	r.HandleFunc("/*", wh.HandleWebhookRequest)
	return r
}
