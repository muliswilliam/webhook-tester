package routers

import (
	"gorm.io/gorm"
	"log"
	"net/http"
	"webhook-tester/internal/handlers"
	"webhook-tester/internal/metrics"
	"webhook-tester/internal/middlewares"
	"webhook-tester/internal/service"

	"github.com/go-chi/chi/v5"
)

func NewApiRouter(svc *service.WebhookService, db *gorm.DB, l *log.Logger) http.Handler {
	r := chi.NewRouter()

	h := handlers.NewWebhookApiHandler(svc, &metrics.PrometheusRecorder{}, l)

	r.Route("/webhooks", func(r chi.Router) {
		r.Use(middlewares.RequireAPIKey(db))
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
