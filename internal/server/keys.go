package server

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/shuliakovsky/email-checker/internal/auth"
	"github.com/shuliakovsky/email-checker/internal/logger"
)

// generateAPIKey creates a cryptographically secure random key
func generateAPIKey() (string, error) {
	b := make([]byte, 32) // 256-bit key
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("random generation failed: %w", err)
	}
	// URL-safe base64 encoding for easy transmission
	return base64.URLEncoding.EncodeToString(b), nil
}

// handleCreateKey handles API key creation requests
func (s *Server) handleCreateKey(w http.ResponseWriter, r *http.Request) {
	// Request payload structure
	var request struct {
		Type          auth.KeyType `json:"type"`           // Type of key to create
		InitialChecks int          `json:"initial_checks"` // Initial check quota
	}

	// Decode JSON request body
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request format")
		return
	}

	// Generate secure random API key
	apiKey, err := generateAPIKey()
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to generate key")
		return
	}

	// Set expiration based on key type
	expiresAt := time.Now().AddDate(2, 0, 0) // 2 years for pay_as_you_go keys
	if request.Type == auth.KeyTypeMonthly {
		expiresAt = time.Now().AddDate(0, 1, 0) // 1 month for monthly keys
	}

	// Insert new key into database
	_, err = s.db.ExecContext(r.Context(), `
		INSERT INTO api_keys (
			api_key, 
			key_type, 
			initial_checks, 
			remaining_checks, 
			expires_at
		) VALUES ($1, $2, $3, $4, $5)`,
		apiKey,
		request.Type,
		request.InitialChecks,
		request.InitialChecks, // Set remaining checks equal to initial quota
		expiresAt,
	)

	if err != nil {
		logger.Log("DB error: " + err.Error())
		respondError(w, http.StatusInternalServerError, "Failed to create key")
		return
	}

	// Return successful response with key details
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"api_key":    apiKey,
		"expires_at": expiresAt.Format(time.RFC3339),
		"key_type":   string(request.Type),
		"remaining":  fmt.Sprintf("%d", request.InitialChecks),
	})
}

// handleListKeys returns all API keys
func (s *Server) handleListKeys(w http.ResponseWriter, r *http.Request) {
	var keys []struct {
		APIKey        string    `db:"api_key" json:"api_key"`
		Type          string    `db:"key_type" json:"type"`
		Remaining     int       `db:"remaining_checks" json:"remaining"`
		InitialChecks int       `db:"initial_checks" json:"initial"`
		CreatedAt     time.Time `db:"created_at" json:"created_at"`
		ExpiresAt     time.Time `db:"expires_at" json:"expires_at"`
	}

	err := s.db.SelectContext(r.Context(), &keys, `
        SELECT api_key, key_type, remaining_checks, 
               initial_checks, created_at, expires_at
        FROM api_keys`)

	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to retrieve keys")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(keys)
}

// handleGetKey returns specific key details
func (s *Server) handleGetKey(w http.ResponseWriter, r *http.Request) {
	apiKey := r.PathValue("api_key")
	if apiKey == "" {
		respondError(w, http.StatusBadRequest, "Missing API key parameter")
		return
	}

	var keyDetails struct {
		APIKey        string    `db:"api_key" json:"api_key"`
		Type          string    `db:"key_type" json:"type"`
		Remaining     int       `db:"remaining_checks" json:"remaining"`
		UsedChecks    int       `db:"used_checks" json:"used"`
		InitialChecks int       `db:"initial_checks" json:"initial"`
		CreatedAt     time.Time `db:"created_at" json:"created_at"`
		ExpiresAt     time.Time `db:"expires_at" json:"expires_at"`
		LastTopup     time.Time `db:"last_topup" json:"last_topup,omitempty"`
	}

	err := s.db.GetContext(r.Context(), &keyDetails, `
        SELECT api_key, key_type, remaining_checks, used_checks,
               initial_checks, created_at, expires_at, last_topup
        FROM api_keys 
        WHERE api_key = $1`, apiKey)

	if err != nil {
		respondError(w, http.StatusNotFound, "API key not found")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(keyDetails)
}

// handleUpdateKey processes key updates
func (s *Server) handleUpdateKey(w http.ResponseWriter, r *http.Request) {
	apiKey := r.PathValue("api_key")
	if apiKey == "" {
		respondError(w, http.StatusBadRequest, "Missing API key parameter")
		return
	}

	var updateRequest struct {
		AddChecks  int `json:"add_checks"`
		ExtendDays int `json:"extend_days"`
	}

	if err := json.NewDecoder(r.Body).Decode(&updateRequest); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request format")
		return
	}

	tx, err := s.db.BeginTxx(r.Context(), nil)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Database error")
		return
	}
	defer tx.Rollback()

	// Обновление квоты и срока действия
	_, err = tx.ExecContext(r.Context(), `
        UPDATE api_keys 
        SET remaining_checks = remaining_checks + $1,
            expires_at = CASE 
                WHEN key_type = 'pay_as_you_go' THEN 
                    GREATEST(expires_at, NOW()) + INTERVAL '24 MONTH'
                ELSE 
                    expires_at + INTERVAL '1 MONTH' 
            END,
            last_topup = NOW()
        WHERE api_key = $2`,
		updateRequest.AddChecks,
		apiKey,
	)

	if err != nil {
		respondError(w, http.StatusInternalServerError, "Update failed")
		return
	}

	if err := tx.Commit(); err != nil {
		respondError(w, http.StatusInternalServerError, "Commit failed")
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "updated"})
}

// handleDeleteKey removes API key
func (s *Server) handleDeleteKey(w http.ResponseWriter, r *http.Request) {
	apiKey := r.PathValue("api_key")
	if apiKey == "" {
		respondError(w, http.StatusBadRequest, "Missing API key parameter")
		return
	}

	_, err := s.db.ExecContext(r.Context(), `
        DELETE FROM api_keys 
        WHERE api_key = $1`, apiKey)

	if err != nil {
		respondError(w, http.StatusInternalServerError, "Deletion failed")
		return
	}

	// Очищаем кэш Redis
	s.redisClient.Del(r.Context(), "apikey:"+apiKey)

	w.WriteHeader(http.StatusNoContent)
}
