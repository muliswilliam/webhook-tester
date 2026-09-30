package routers

import (
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
	"webhook-tester/internal/handlers"
	"webhook-tester/internal/mailer"
	"webhook-tester/internal/metrics"
	"webhook-tester/internal/service"
	"webhook-tester/internal/web/view"

	"github.com/go-chi/chi/v5"
	"github.com/gorilla/csrf"
)

func NewWebRouter(
	wrs *service.WebhookRequestService,
	ws *service.WebhookService,
	authSvc *service.AuthService,
	m mailer.Mailer,
	metricsRec metrics.Recorder,
	logger *log.Logger,
) http.Handler {
	r := chi.NewRouter()
	r.NotFound(view.RenderNotFound)

	// CSRF Setup.
	//
	// gorilla/csrf always assumes the request is served over HTTPS unless a
	// request is explicitly marked as plaintext via csrf.PlaintextHTTPRequest,
	// so we derive that flag (and the Secure cookie flag / trusted origin)
	// from the app's own DOMAIN setting rather than guessing from ENV.
	csrfKey := []byte(os.Getenv("AUTH_SECRET"))
	domain := os.Getenv("DOMAIN")
	isHTTPS := strings.HasPrefix(domain, "https://")

	var trustedOrigins []string
	if parsedDomain, err := url.Parse(domain); err == nil && parsedDomain.Host != "" {
		trustedOrigins = append(trustedOrigins, parsedDomain.Host)
	}

	// Forms carry tokens masked from the CSRF cookie that was current when
	// their page loaded, so the cookie must outlive any page a user may keep
	// open - notably guest workspaces, which last two days. gorilla/csrf's
	// 12h default would break the forms of every older tab.
	csrfMiddleware := csrf.Protect(
		csrfKey,
		csrf.MaxAge(int((30 * 24 * time.Hour).Seconds())),
		csrf.Secure(isHTTPS),
		csrf.Path("/"),
		csrf.TrustedOrigins(trustedOrigins),
	)

	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			secure := r.TLS != nil || strings.EqualFold(r.Header.Get("X-Forwarded-Proto"), "https")
			if !secure {
				r = csrf.PlaintextHTTPRequest(r)
			}
			next.ServeHTTP(w, r)
		})
	})
	r.Use(csrfMiddleware)

	webhookReqHandler := handlers.NewWebhookRequestHandler(wrs, authSvc, ws, &metricsRec, logger)
	r.Route("/requests", func(r chi.Router) {
		r.Get("/{id}", webhookReqHandler.GetRequest)
		r.Post("/{id}/delete", webhookReqHandler.DeleteRequest)
		r.Post("/{id}/replay", webhookReqHandler.ReplayRequest)
	})

	hh := handlers.NewHomeHandler(ws, authSvc, logger, metricsRec)
	r.Get("/", hh.Home)

	webhookHandler := handlers.NewWebhookHandler(ws, wrs, authSvc, logger, metricsRec)
	r.Post("/create-webhook", webhookHandler.Create)
	r.Post("/delete-requests/{id}", webhookHandler.DeleteRequests)
	r.Post("/delete-webhook/{id}", webhookHandler.DeleteWebhook)
	r.Post("/update-webhook/{id}", webhookHandler.UpdateWebhook)
	r.Get("/webhook-stream/{id}", webhookHandler.StreamWebhookEvents)

	authHandler := handlers.NewAuthHandler(authSvc, ws, m, logger, metricsRec)
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
