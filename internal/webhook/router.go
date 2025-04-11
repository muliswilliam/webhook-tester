package webhook

import (
	"github.com/go-chi/chi/v5"
)

func NewRouter() *chi.Mux {
	r := chi.NewRouter()

	// Match all HTTP methods at /{webhookID
	r.HandleFunc("/*", HandleWebhookRequest)
	return r
}
