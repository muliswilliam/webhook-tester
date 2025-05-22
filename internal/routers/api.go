package routers

import (
	"log"
	"net/http"
	"webhook-tester/internal/handlers"
	"webhook-tester/internal/metrics"
	"webhook-tester/internal/middlewares"
	"webhook-tester/internal/service"

	"github.com/go-chi/chi/v5"
)

func NewApiRouter(webhookSvc service.WebhookService, authSvc service.AuthService, l *log.Logger, metricsRec metrics.Recorder) http.Handler {
	r := chi.NewRouter()

	h := handlers.NewWebhookApiHandler(webhookSvc, metricsRec, l)

	r.Route("/webhooks", func(r chi.Router) {
		r.Use(middlewares.RequireAPIKey(authSvc))
		r.Get("/", h.ListWebhooksApi)
		r.Post("/", h.CreateWebhookApi)

		r.Route("/{id}", func(r chi.Router) {
			r.Get("/", h.GetWebhookApi)
			r.Put("/", h.UpdateWebhookApi)
			r.Delete("/", h.DeleteWebhookApi)
		})
	})

	return r
}
