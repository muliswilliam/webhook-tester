package web

import (
	"log"
	"net/http"
	"os"
	"strings"
	"webhook-tester/internal/handlers"

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

	h := &handlers.Handler{
		DB:           db,
		SessionStore: sessionStore,
		Logger:       logger,
	}

	r.Get("/", h.Home)

	r.Route("/requests", func(r chi.Router) {
		r.Get("/{id}", h.GetRequest)
		r.Post("/{id}/delete", h.DeleteRequest)
		r.Post("/{id}/replay", h.ReplayRequest)
	})

	r.Post("/create-webhook", h.CreateWebhook)
	r.Post("/delete-requests/{id}", h.DeleteWebhookRequests)
	r.Post("/delete-webhook/{id}", h.DeleteWebhook)
	r.Post("/update-webhook/{id}", h.UpdateWebhook)
	r.Get("/webhook-stream/{id}", h.StreamWebhookEvents)

	r.Get("/register", h.RegisterGet)
	r.Post("/register", h.RegisterPost)
	r.Get("/login", h.LoginGet)
	r.Post("/login", h.LoginPost)
	r.Get("/logout", h.Logout)
	r.Get("/forgot-password", h.ForgotPasswordGet)
	r.Post("/forgot-password", h.ForgotPasswordPost)
	r.Get("/reset-password", h.ResetPasswordGet)
	r.Post("/reset-password", h.ResetPasswordPost)
	r.Get("/privacy", h.PrivacyPolicy)
	r.Get("/terms", h.TermsAndConditions)

	return r
}
