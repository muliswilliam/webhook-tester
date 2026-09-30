// internal/middlewares/api.go
package middlewares

import (
	"context"
	"net/http"

	"webhook-tester/internal/dtos"
	"webhook-tester/internal/models"
	"webhook-tester/internal/service"
	"webhook-tester/internal/utils"
)

type ctxKeyUser struct{}

func RequireAPIKey(auth *service.AuthService) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			apiKey := r.Header.Get("X-API-Key")
			if apiKey == "" {
				utils.RenderJSON(w, http.StatusUnauthorized, dtos.ErrorResponse{Error: "API key missing"})
				return
			}

			user, err := auth.ValidateAPIKey(apiKey)
			if err != nil {
				utils.RenderJSON(w, http.StatusUnauthorized, dtos.ErrorResponse{Error: "invalid API key"})
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
