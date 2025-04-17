package web

import (
	"github.com/go-chi/chi/v5"
	"net/http"
	"webhook-tester/internal/web/handlers"
)

func NewRouter() http.Handler {
	r := chi.NewRouter()

	r.Get("/", handlers.Home)

	r.Route("/requests", func(r chi.Router) {
		r.Get("/{id}", handlers.GetRequest)
		r.Post("/{id}/delete", handlers.DeleteRequest)
	})

	r.Post("/create-webhook", handlers.CreateWebhook)
	r.Post("/delete-requests/{id}", handlers.DeleteWebhookRequests)
	r.Post("/delete-webhook/{id}", handlers.DeleteWebhook)
	r.Post("/update-webhook/{id}", handlers.UpdateWebhook)

	r.Get("/login", handlers.Login)
	r.Post("/login", handlers.Login)
	r.Get("/logout", handlers.Logout)

	return r
}
