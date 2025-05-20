package web

import (
	"log"
	"net/http"
	"os"
	"strings"
	"webhook-tester/internal/handlers"
	"webhook-tester/internal/metrics"
	"webhook-tester/internal/service"

	"github.com/go-chi/chi/v5"
	"github.com/gorilla/csrf"
)

func NewWebRouter(
	wrs *service.WebhookRequestService,
	ws *service.WebhookService,
	authSvc *service.AuthService,
	metricsRec metrics.Recorder,
	logger *log.Logger,
) http.Handler {
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

	webhookReqHandler := handlers.NewWebhookRequestHandler(wrs, authSvc, ws, &metricsRec, logger)
	r.Route("/requests", func(r chi.Router) {
		r.Get("/{id}", webhookReqHandler.GetRequest)
		r.Post("/{id}/delete", webhookReqHandler.DeleteRequest)
		r.Post("/{id}/replay", webhookReqHandler.ReplayRequest)
	})

	hh := handlers.NewHomeHandler(ws, authSvc, logger, metricsRec)
	r.Get("/", hh.Home)

	webhookHandler := handlers.NewWebhookHandler(ws, authSvc, logger, metricsRec)
	r.Post("/create-webhook", webhookHandler.Create)
	r.Post("/delete-requests/{id}", webhookHandler.DeleteRequests)
	r.Post("/delete-webhook/{id}", webhookHandler.DeleteWebhook)
	r.Post("/update-webhook/{id}", webhookHandler.UpdateWebhook)
	r.Get("/webhook-stream/{id}", webhookHandler.StreamWebhookEvents)

	authHandler := handlers.NewAuthHandler(authSvc, logger, metricsRec)
	r.Get("/register", authHandler.RegisterGet)
	r.Post("/register", authHandler.RegisterPost)
	r.Get("/login", authHandler.LoginGet)
	r.Post("/login", authHandler.LoginPost)
	r.Get("/logout", authHandler.Logout)
	r.Get("/forgot-password", authHandler.ForgotPasswordGet)
	r.Post("/forgot-password", authHandler.ForgotPasswordPost)
	r.Get("/reset-password", authHandler.ResetPasswordGet)
	r.Post("/reset-password", authHandler.ResetPasswordPost)

	lh := handlers.NewLegalHandler()
	r.Get("/privacy", lh.PrivacyPolicy)
	r.Get("/terms", lh.TermsAndConditions)

	return r
}
