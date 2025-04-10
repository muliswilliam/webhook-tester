package api

import (
	"net/http"
	"webhook-tester/internal/api/handlers"

	"github.com/go-chi/chi/v5"
)

func NewRouter() http.Handler {
	r := chi.NewRouter()
	r.Route("/webhooks", func(r chi.Router) {
		r.Get("/", handlers.ListWebhooks)
		r.Post("/", handlers.CreateWebhook)

		r.Route("/{id}", func(r chi.Router) {
			r.Get("/", handlers.GetWebhook)
			r.Put("/", handlers.UpdateWebhook)
			r.Delete("/", handlers.DeleteWebhook)

			r.Post("/events", handlers.ReceiveEvent)
			r.Get("/events", handlers.ListEvents)
		})
	})

	return r
}
