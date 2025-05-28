// internal/middlewares/api.go
package middlewares

import (
	"context"
	"net/http"

	"webhook-tester/internal/models"
	"webhook-tester/internal/service"
)

type ctxKeyUser struct{}

func RequireAPIKey(auth service.AuthService) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			apiKey := r.Header.Get("X-API-Key")
			if apiKey == "" {
				http.Error(w, "API key missing", http.StatusUnauthorized)
				return
			}

			user, err := auth.ValidateAPIKey(apiKey)
			if err != nil {
				http.Error(w, "Invalid API key", http.StatusUnauthorized)
				return
			}

			// attach the full user object to context
			ctx := context.WithValue(r.Context(), ctxKeyUser{}, user)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// GetAPIAuthenticatedUser retrieves the user set by RequireAPIKey
func GetAPIAuthenticatedUser(r *http.Request) *models.User {
	user, _ := r.Context().Value(ctxKeyUser{}).(*models.User)
	return user
}
