package types

import "time"

// MXRecord represents an individual Mail Exchange (MX) record with its associated details
type MXRecord struct {
	Host     string `json:"host"`     // Hostname of the MX server (e.g., mail.example.com)
	Priority uint16 `json:"priority"` // Priority of the MX server; lower values have higher priority
	TTL      int    `json:"ttl"`      // Time-to-live value indicating how long the record is valid
}

// MXStats contains information about a domain's MX records
type MXStats struct {
	Valid   bool       `json:"valid"`             // Indicates whether valid MX records are available for the domain
	Records []MXRecord `json:"records,omitempty"` // List of retrieved MX records; omitted if none are found
	Error   string     `json:"error,omitempty"`   // Description of any error encountered during MX lookup
}

// EmailReport represents the result of validating and processing an email address
type EmailReport struct {
	Email          string  `json:"email"`                     // The email address being validated
	Valid          bool    `json:"valid"`                     // Indicates whether the email address has a valid format
	Disposable     bool    `json:"disposable"`                // Indicates whether the domain is a disposable (temporary) email provider
	Exists         *bool   `json:"exists,omitempty"`          // Indicates whether the email address exists (nil if not checked)
	MX             MXStats `json:"mx"`                        // Contains MX record-related statistics and errors
	PermanentError bool    `json:"permanent_error,omitempty"` // Indicates if a permanent error occurred during validation
	ErrorCategory  string  `json:"error_category,omitempty"`  // Describes the error type, if any (e.g., "mailbox_not_found")
	TTL            int     `json:"ttl,omitempty"`             // Time-to-live value for retrying validation (if temporary error)
	SMTPError      string  `json:"smtp_error,omitempty"`      // Description of any SMTP error encountered during validation
}

// Task represents a batch email validation task
type Task struct {
	ID        string         `json:"id"`                // Unique identifier for the task
	Status    string         `json:"status"`            // Current status of the task (e.g., "pending", "processing", "completed")
	Emails    []string       `json:"emails"`            // List of email addresses to be validated in the task
	Results   []EmailReport  `json:"results"`           // List of validation results for the processed emails
	CreatedAt time.Time      `json:"created_at"`        // Timestamp indicating when the task was created
	Webhook   *WebhookConfig `json:"webhook,omitempty"` // Webhook configuration
}

// WebhookConfig contains the parameters for task status notifications
type WebhookConfig struct {
	URL     string        `json:"url"`     // URL for sending notifications
	TTL     time.Duration `json:"-"`       // Excluded from JSON, used internally within the application
	TTLStr  string        `json:"ttl"`     // Accepts a string from JSON (e.g., "1h")
	Retries int           `json:"retries"` // Maximum number of retry attempts
	Secret  string        `json:"secret"`  // Secret for signing requests (optional)
}
