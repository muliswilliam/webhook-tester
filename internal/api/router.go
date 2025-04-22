package api

import (
	"github.com/go-chi/chi/v5"
	"github.com/wader/gormstore/v2"
	"gorm.io/gorm"
	"net/http"
	"webhook-tester/internal/handlers"
)

func Router(db *gorm.DB, sessionStore *gormstore.Store) http.Handler {
	r := chi.NewRouter()

	h := handlers.Handler{
		SessionStore: sessionStore,
		DB:           db,
	}

	r.Route("/webhooks", func(r chi.Router) {
		r.Get("/", h.ListWebhooks)
		r.Post("/", h.CreateWebhook)

		r.Route("/{id}", func(r chi.Router) {
			r.Get("/", h.GetWebhook)
			r.Get("/stream", h.StreamWebhookEvents)
			r.Put("/", h.UpdateWebhook)
			r.Delete("/", h.DeleteWebhook)
		})
	})

	return r
}
