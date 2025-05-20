package middlewares

import (
	"context"
	"gorm.io/gorm"
	"net/http"
	"webhook-tester/internal/models"
)

// RequireAPIKey Checks for API Key in X-API-Key header
// If API Key is not found or is invalid, it responds to the request with http status 401
// If API Key is valid, it adds the associated user to the context for
// the next handler to use
func RequireAPIKey(db *gorm.DB) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			apiKey := r.Header.Get("X-API-Key")
			if apiKey == "" {
				http.Error(w, "API key missing", http.StatusUnauthorized)
				return
			}

			var user models.User
			err := db.First(&user, "api_key = ?", apiKey).Error
			if err != nil {
				http.Error(w, "Invalid API key", http.StatusUnauthorized)
				return
			}

			// Optionally: attach user to context
			ctx := context.WithValue(r.Context(), "user", user)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// GetApiAuthenticatedUser Gets user object from the context
func GetApiAuthenticatedUser(r *http.Request) models.User {
	user, _ := r.Context().Value("user").(models.User)
	return user
}
