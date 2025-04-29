package server

import (
	"context"
	"encoding/json"
	"github.com/spf13/viper"
	"net/http"

	"github.com/shuliakovsky/email-checker/internal/auth"
	"github.com/shuliakovsky/email-checker/internal/logger"
	"github.com/shuliakovsky/email-checker/internal/metrics"
)

// APIKeyMiddleware validates API keys and enforces authentication
func APIKeyMiddleware(authService *auth.AuthService) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Extract API key from headers
			apiKey := r.Header.Get("X-API-Key")
			if apiKey == "" {
				respondError(w, http.StatusUnauthorized, "API key required")
				return
			}

			// Validate key through authentication service
			key, err := authService.ValidateKey(r.Context(), apiKey)
			if err != nil {
				respondError(w, http.StatusForbidden, err.Error())
				return
			}

			// Update metrics for monitoring
			metrics.APIKeyChecks.WithLabelValues(apiKey, string(key.Type)).Inc()
			metrics.APIKeyQuota.WithLabelValues(apiKey).Set(float64(key.Remaining))

			// Add key details to request context
			ctx := context.WithValue(r.Context(), "api_key", key)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// respondError sends standardized JSON error responses
func respondError(w http.ResponseWriter, code int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	if err := json.NewEncoder(w).Encode(map[string]string{"error": message}); err != nil {
		logger.Log("Failed to write error response: " + err.Error())
	}
}

// corsMiddleware handles Cross-Origin Resource Sharing headers
// TODO: Move CORS configuration to external config
func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Set permissive CORS headers
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS, PUT, DELETE")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

		// Handle preflight requests
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// AdminMiddleware enforces admin-level access control
func AdminMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify admin key from configuration
		adminKey := viper.GetString("admin-key")
		if r.Header.Get("X-Admin-Key") != adminKey {
			respondError(w, http.StatusForbidden, "Admin access required")
			return
		}
		next.ServeHTTP(w, r)
	})
}
