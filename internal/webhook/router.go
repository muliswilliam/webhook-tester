package webhook

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

	// Match all HTTP methods at /{webhookID}
	r.HandleFunc("/*", h.HandleWebhookRequest)
	return r
}
