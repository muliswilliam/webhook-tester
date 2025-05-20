package web

import (
	"log"
	"net/http"
	"os"
	"strings"
	"webhook-tester/internal/handlers"
	"webhook-tester/internal/metrics"
	"webhook-tester/internal/service"
	"webhook-tester/internal/store"

	"github.com/wader/gormstore/v2"
	"gorm.io/gorm"

	"github.com/go-chi/chi/v5"
	"github.com/gorilla/csrf"
)

func Router(db *gorm.DB, sessionStore *gormstore.Store, logger *log.Logger) http.Handler {
	r := chi.NewRouter()

	// CSRF Setup
	csrfKey := []byte(os.Getenv("AUTH_SECRET"))
	isProd := strings.Contains(os.Getenv("ENV"), "prod")
	var trustedOrigins []string
	if !isProd {
		trustedOrigins = append(trustedOrigins, "localhost:3000")
	}

	csrfMiddleware := csrf.Protect(
		csrfKey,
		csrf.Secure(isProd),
		csrf.Path("/"),
		csrf.TrustedOrigins(trustedOrigins),
	)

	r.Use(csrfMiddleware)

	recorder := metrics.PrometheusRecorder{}
	h := &handlers.Handler{
		DB:           db,
		SessionStore: sessionStore,
		Logger:       logger,
		Metrics:      &recorder,
	}

	r.Route("/requests", func(r chi.Router) {
		r.Get("/{id}", h.GetRequest)
		r.Post("/{id}/delete", h.DeleteRequest)
		r.Post("/{id}/replay", h.ReplayRequest)
	})

	wr := store.NewGormWebookRepo(db, logger)
	ws := service.NewWebhookService(wr)

	hh := handlers.NewHomeHandler(ws, sessionStore, logger, &recorder, db)
	r.Get("/", hh.Home)

	webhookHandler := handlers.NewWebhookHandler(ws,
		sessionStore,
		logger,
		&recorder,
	)
	r.Post("/create-webhook", webhookHandler.Create)
	r.Post("/delete-requests/{id}", webhookHandler.DeleteRequests)
	r.Post("/delete-webhook/{id}", webhookHandler.DeleteWebhook)
	r.Post("/update-webhook/{id}", webhookHandler.UpdateWebhook)
	r.Get("/webhook-stream/{id}", webhookHandler.StreamWebhookEvents)

	userRepo := store.NewGormUserRepo(db, logger)
	authSvc := service.NewAuthService(userRepo)
	authHandler := handlers.NewAuthHandler(authSvc, logger, &recorder, sessionStore)
	r.Get("/register", authHandler.RegisterGet)
	r.Post("/register", authHandler.RegisterPost)
	r.Get("/login", authHandler.LoginGet)
	r.Post("/login", authHandler.LoginPost)
	r.Get("/logout", authHandler.Logout)
	r.Get("/forgot-password", authHandler.ForgotPasswordGet)
	r.Post("/forgot-password", authHandler.ForgotPasswordPost)
	r.Get("/reset-password", authHandler.ResetPasswordGet)
	r.Post("/reset-password", authHandler.ResetPasswordPost)

	r.Get("/privacy", h.PrivacyPolicy)
	r.Get("/terms", h.TermsAndConditions)

	return r
}
