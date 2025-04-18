package types

// Represents individual MX records with host, priority, and TTL
type MXRecord struct {
	Host     string `json:"host"`     // Hostname of the MX server
	Priority uint16 `json:"priority"` // Priority of the MX server
	TTL      int    `json:"ttl"`      // Time-to-live value for the MX record
}

// Represents statistics related to the MX records of a domain
type MXStats struct {
	Valid   bool       `json:"valid"`             // Indicates if valid MX records are available
	Records []MXRecord `json:"records,omitempty"` // List of MX records associated with the domain
	Error   string     `json:"error,omitempty"`   // Description of any error encountered during MX lookup
}

// Represents the result of processing an email
type EmailReport struct {
	Email          string  `json:"email"`                     // The email address being processed
	Valid          bool    `json:"valid"`                     // Indicates whether the email has a valid format
	Disposable     bool    `json:"disposable"`                // Indicates whether the domain is disposal
	Exists         *bool   `json:"exists,omitempty"`          // Indicates whether the email exists (nil if not verified)
	MX             MXStats `json:"mx"`                        // Contains MX record-related information
	PermanentError bool    `json:"permanent_error,omitempty"` // Indicates if a permanent error occurred
	ErrorCategory  string  `json:"error_category,omitempty"`  // Category of the error, if any
	TTL            int     `json:"ttl,omitempty"`             // Time-to-live value for retry (applicable for temporary errors)
	SMTPError      string  `json:"smtp_error,omitempty"`      // Describes the SMTP error, if any
}
