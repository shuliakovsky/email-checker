package server

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	_ "github.com/shuliakovsky/email-checker/docs"
	"github.com/shuliakovsky/email-checker/internal/metrics"
	"github.com/shuliakovsky/email-checker/pkg/types"
)

func (s *Server) handleTasksWithWebhook(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost {
		var request struct {
			Emails  []string            `json:"emails"`
			Webhook types.WebhookConfig `json:"webhook"`
		}

		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			http.Error(w, "Invalid JSON format", http.StatusBadRequest)
			return
		}

		// Parse TTL from a string into time.Duration
		ttl, err := time.ParseDuration(request.Webhook.TTLStr)
		if err != nil {
			http.Error(w, "Invalid TTL format (e.g., '1h', '30m')", http.StatusBadRequest)
			return
		}
		request.Webhook.TTL = ttl // Save the converted value

		// Validate webhook parameters
		if request.Webhook.URL == "" || request.Webhook.Retries <= 0 {
			http.Error(w, "Invalid webhook config", http.StatusBadRequest)
			return
		}

		taskID := s.generateID()
		task := &types.Task{
			ID:        taskID,
			Status:    "pending",
			Emails:    request.Emails,
			CreatedAt: time.Now(),
			Webhook:   &request.Webhook,
		}

		// Save task and webhook to Redis
		if err := s.storage.SaveTask(r.Context(), task); err != nil {
			http.Error(w, "Failed to save task", http.StatusInternalServerError)
			return
		}

		// Save webhook separately for clustered mode
		if s.clusterMode {
			webhookKey := fmt.Sprintf("webhook:task:%s", taskID)
			data, _ := json.Marshal(request.Webhook)
			s.redisClient.Set(r.Context(), webhookKey, data, ttl) // Use ttl of type time.Duration
		}

		go s.processTask(task) // Start processing

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"task_id": taskID})
		return
	}
	http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
}

// sendWebhookRequest executes HTTP POST request to webhook URL
func (s *Server) sendWebhookRequest(task *types.Task, cfg types.WebhookConfig, attemptKey string) bool {
	startTime := time.Now()

	attempts, _ := s.redisClient.Get(context.Background(), attemptKey).Int()

	payload, _ := json.Marshal(map[string]interface{}{
		"task_id":  task.ID,
		"status":   task.Status,
		"results":  len(task.Results),
		"ttl":      cfg.TTLStr,
		"attempts": attempts,
		"lifetime": time.Since(task.CreatedAt).String(),
	})

	req, _ := http.NewRequest("POST", cfg.URL, bytes.NewBuffer(payload))
	req.Header.Set("Content-Type", "application/json")
	if cfg.Secret != "" {
		req.Header.Set("X-Signature", generateSignature(payload, cfg.Secret))
	}

	defer func() {
		metrics.WebhookLatency.Observe(time.Since(startTime).Seconds())
	}()

	// Send request
	resp, err := http.DefaultClient.Do(req)
	success := err == nil && resp.StatusCode < 400

	// Update metrics
	statusLabel := "failure"
	if success {
		statusLabel = "success"
	}
	metrics.WebhookAttempts.WithLabelValues(task.ID, statusLabel).Inc()

	if !success && attempts > 0 {
		metrics.WebhookRetries.Inc()
	}

	return success
}

// triggerWebhook sends notification and handles retries
func (s *Server) triggerWebhook(task *types.Task) {
	webhookKey := fmt.Sprintf("webhook:task:%s", task.ID)
	var webhook types.WebhookConfig

	// Get webhook config from Redis or local storage
	if s.clusterMode {
		data, err := s.redisClient.Get(context.Background(), webhookKey).Result()
		if err != nil {
			return
		}
		json.Unmarshal([]byte(data), &webhook)
	} else {
		webhook = *task.Webhook
	}

	attemptKey := webhookKey + ":attempts"
	s.redisClient.Set(context.Background(), attemptKey, 1, webhook.TTL) // Initialize counter

	for i := 0; i < webhook.Retries; i++ {
		currentAttempt, _ := s.redisClient.Get(context.Background(), attemptKey).Int()
		success := s.sendWebhookRequest(task, webhook, attemptKey)
		if success {
			s.redisClient.Set(context.Background(), attemptKey, currentAttempt-1, webhook.TTL)
			break
		}
		s.redisClient.Incr(context.Background(), attemptKey)
		time.Sleep(2 * time.Second)
	}
}

// generateSignature creates HMAC-SHA256 signature for webhook payload
func generateSignature(payload []byte, secret string) string {
	h := hmac.New(sha256.New, []byte(secret))
	h.Write(payload)
	return hex.EncodeToString(h.Sum(nil))
}
