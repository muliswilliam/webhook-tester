package webhook

import (
	"log"
	"net/http"
	"webhook-tester/internal/handlers"
	"webhook-tester/internal/metrics"

	"github.com/go-chi/chi/v5"
	"github.com/wader/gormstore/v2"
	"gorm.io/gorm"
)

func NewRouter(db *gorm.DB, sessionStore *gormstore.Store, logger *log.Logger) http.Handler {
	r := chi.NewRouter()

	h := handlers.Handler{
		SessionStore: sessionStore,
		DB:           db,
		Logger:       logger,
		Metrics:      &metrics.PrometheusRecorder{},
	}

	// Match all HTTP methods at /{webhookID}
	r.HandleFunc("/*", h.HandleWebhookRequest)
	return r
}
