package web

import (
	"net/http"
	"os"
	"strings"
	"webhook-tester/internal/web/handlers"

	"github.com/go-chi/chi/v5"
	"github.com/gorilla/csrf"
)

func NewRouter() http.Handler {
	r := chi.NewRouter()

	// CSRF Setup
	csrfKey := []byte(os.Getenv("AUTH_SECRET"))
	isProd := strings.Contains(os.Getenv("ENV"), "prod")
	csrfMiddleware := csrf.Protect(
		csrfKey,
		csrf.Secure(isProd),
		csrf.Path("/"),
		csrf.TrustedOrigins([]string{"localhost:3000"}),
		csrf.ErrorHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.Error(w, "CSRF failure: "+csrf.FailureReason(r).Error(), http.StatusForbidden)
		})))

	r.Use(csrfMiddleware)

	r.Get("/", handlers.Home)

	r.Route("/requests", func(r chi.Router) {
		r.Get("/{id}", handlers.GetRequest)
		r.Post("/{id}/delete", handlers.DeleteRequest)
		r.Post("/{id}/replay", handlers.ReplayRequest)
	})

	r.Post("/create-webhook", handlers.CreateWebhook)
	r.Post("/delete-requests/{id}", handlers.DeleteWebhookRequests)
	r.Post("/delete-webhook/{id}", handlers.DeleteWebhook)
	r.Post("/update-webhook/{id}", handlers.UpdateWebhook)

	r.Get("/register", handlers.RegisterGet)
	r.Post("/register", handlers.RegisterPost)
	r.Get("/login", handlers.LoginGet)
	r.Post("/login", handlers.LoginPost)
	r.Get("/logout", handlers.Logout)

	return r
}
