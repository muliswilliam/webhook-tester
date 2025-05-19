package webhook

import (
	"log"
	"net/http"
	"webhook-tester/internal/handlers"
	"webhook-tester/internal/metrics"
	"webhook-tester/internal/service"
	"webhook-tester/internal/store"

	"github.com/go-chi/chi/v5"
	"github.com/wader/gormstore/v2"
	"gorm.io/gorm"
)

func NewRouter(db *gorm.DB, sessionStore *gormstore.Store, logger *log.Logger) http.Handler {
	r := chi.NewRouter()

	wr := store.NewGormWebookRepo(db, logger)
	ws := service.NewWebhookService(wr)
	wh := handlers.NewWebhookHandler(ws,
		sessionStore,
		logger,
		&metrics.PrometheusRecorder{},
	)

	// Match all HTTP methods at /{webhookID}
	r.HandleFunc("/*", wh.HandleWebhookRequest)
	return r
}
