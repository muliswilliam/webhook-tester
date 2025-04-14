package api

import (
	"net/http"
	"webhook-tester/internal/api/handlers"
	"webhook-tester/internal/webhook"

	"github.com/go-chi/chi/v5"
)

func NewRouter() http.Handler {
	r := chi.NewRouter()
	r.Route("/webhooks", func(r chi.Router) {
		r.Get("/", handlers.ListWebhooks)
		r.Post("/", handlers.CreateWebhook)

		r.Route("/{id}", func(r chi.Router) {
			r.Get("/", handlers.GetWebhook)
			r.Get("/stream", webhook.StreamWebhookEvents)
			r.Put("/", handlers.UpdateWebhook)
			r.Delete("/", handlers.DeleteWebhook)
		})
	})

	r.Route("/users", func(r chi.Router) {
		r.Post("/", handlers.CreateUser)
	})

	return r
}
