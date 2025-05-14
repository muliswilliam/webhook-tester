package api

import (
	"log"
	"net/http"
	"webhook-tester/internal/handlers"
	"webhook-tester/internal/metrics"
	"webhook-tester/internal/middlewares"

	"github.com/go-chi/chi/v5"
	"github.com/wader/gormstore/v2"
	"gorm.io/gorm"
)

func Router(db *gorm.DB, sessionStore *gormstore.Store, l *log.Logger) http.Handler {
	r := chi.NewRouter()

	h := handlers.Handler{
		SessionStore: sessionStore,
		DB:           db,
		Logger:       l,
		Metrics: &metrics.PrometheusRecorder{},
	}

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
