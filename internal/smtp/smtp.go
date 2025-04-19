package smtp

import (
	"crypto/tls"                                            // Enables secure TLS connections
	"fmt"                                                   // Provides formatted input and output functions
	"github.com/shuliakovsky/email-checker/internal/logger" // Logging utility
	"net"                                                   // Includes networking capabilities
	"net/smtp"                                              // Provides SMTP client functionality
	"strings"                                               // Offers string manipulation utilities
	"time"                                                  // For handling time durations and delays
)

const (
	connectTimeout = 3 * time.Second // Timeout duration for establishing a connection
	commandTimeout = 8 * time.Second // Timeout duration for executing SMTP commands
	maxRetries     = 2               // Maximum number of retries for a failed attempt
	retryDelay     = 1 * time.Second // Delay duration between retries
	heloDomain     = "example.com"   // Domain name used in HELO/EHLO commands
)

// Checks the existence of an email address by interacting with the SMTP servers specified in MX records
func CheckEmailExists(email string, mxRecords []*net.MX) (bool, string, string, bool, int) {
	ports := []string{"25", "587", "465"} // Common SMTP ports (unsecured and secured)
	var (
		maxTTL        int    // Stores maximum TTL value from temporary errors
		finalErr      string // Last error received during processing
		finalCategory string // Category of the last error
		hasPermanent  bool   // Flag indicating if a permanent error occurred
		permanentErr  string // Stores the permanent error message
		permanentCat  string // Stores the category of the permanent error
	)

	for _, mx := range mxRecords {
		mxHost := strings.TrimSuffix(mx.Host, ".") // Removes trailing dot from MX host
		for _, port := range ports {
			logger.Log(fmt.Sprintf("Trying %s:%s for %s", mxHost, port, email)) // Logs the connection attempt

			// Performs email validation with retry mechanism
			exists, err, retry := attemptWithRetry(email, mxHost, port)
			if retry {
				logger.Log(fmt.Sprintf("Retrying %s:%s", mxHost, port)) // Logs retry attempt
				time.Sleep(retryDelay)                                  // Waits before retrying
				exists, err, _ = attemptWithRetry(email, mxHost, port)
			}

			if exists { // If the email exists, return success
				return true, "", "", false, 0
			}

			if err != "" { // Processes errors returned during validation
				category, permanent, ttl := classifySMTPError(err)                      // Classify the error
				logger.Log(fmt.Sprintf("SMTP error: %s (category: %s)", err, category)) // Logs the error

				if permanent { // If the error is permanent, break the loop
					hasPermanent = true
					permanentErr = err
					permanentCat = category
					break
				}

				if ttl > maxTTL { // Stores the highest TTL value among temporary errors
					maxTTL = ttl
					finalErr = err
					finalCategory = category
				}
			}
		}

		if hasPermanent { // If a permanent error has been detected, stop processing
			break
		}
	}

	if hasPermanent { // Returns results for permanent errors
		return false, permanentErr, permanentCat, true, 0
	}
	if finalErr != "" { // Returns results for temporary errors
		return false, finalErr, finalCategory, false, maxTTL
	}
	return false, "", "", false, 0 // Default return if no valid results are obtained
}

// Classifies SMTP errors based on the error code and categorizes them as permanent or temporary
func classifySMTPError(errMsg string) (string, bool, int) {
	code := extractSMTPCode(errMsg) // Extracts SMTP error code from the message
	switch {
	case strings.HasPrefix(code, "5"): // Permanent error codes start with '5'
		return handlePermanentErrors(code)
	case strings.HasPrefix(code, "4"): // Temporary error codes start with '4'
		return handleTemporaryErrors(code)
	default: // Unknown error codes are treated as permanent errors
		return "unknown_error", true, 0
	}
}

// Extracts SMTP error code from the error message for classification
func extractSMTPCode(errMsg string) string {
	parts := strings.SplitN(errMsg, " ", 3) // Splits error message to isolate the code
	if len(parts) > 0 && len(parts[0]) >= 3 {
		code := parts[0]
		if code[0] == '4' || code[0] == '5' {
			return strings.SplitN(code, ".", 2)[0] // Extracts the primary error code
		}
	}
	return ""
}

// Handles permanent SMTP errors and assigns specific categories based on the error code
func handlePermanentErrors(code string) (string, bool, int) {
	switch code {
	case "550", "551": // Error codes for mailbox not found
		return "mailbox_not_found", true, 0
	case "552": // Error code for mailbox full
		return "mailbox_full", true, 0
	case "553", "501": // Error codes for invalid address
		return "invalid_address", true, 0
	case "554": // Error code for transaction failure
		return "transaction_failed", true, 0
	default: // General permanent error
		return "permanent_error", true, 0
	}
}

// Handles temporary SMTP errors and assigns specific categories based on the error code
func handleTemporaryErrors(code string) (string, bool, int) {
	ttl := calculateTTL(code) // Calculates the retry wait time (TTL)
	switch code {
	case "421", "450": // Error codes for server unavailable
		return "server_unavailable", false, ttl
	case "451": // Error code for server error
		return "server_error", false, ttl
	case "452": // Error code for storage limit exceeded
		return "storage_limit", false, ttl
	default: // General temporary error
		return "temporary_error", false, ttl
	}
}

// Calculates TTL values for specific temporary error codes
func calculateTTL(code string) int {
	switch code {
	case "421": // Retry in 30 minutes
		return 1800
	case "450": // Retry in 1 hour
		return 3600
	case "451": // Retry in 2 hours
		return 7200
	case "452": // Retry in 4 hours
		return 14400
	default: // Default retry time of 1 hour
		return 3600
	}
}

// Attempts email validation with multiple retries if necessary
func attemptWithRetry(email, host, port string) (bool, string, bool) {
	for i := 0; i < maxRetries; i++ {
		exists, err, retry := attempt(email, host, port) // Performs email validation
		if !retry {
			return exists, err, false // Stops retries if retry flag is false
		}
		time.Sleep(retryDelay) // Waits before retrying
	}
	return false, "max retries exceeded", false // Returns default result after max retries
}

// Performs a single email validation attempt by interacting with the SMTP server
func attempt(email, host, port string) (bool, string, bool) {
	conn, err := connect(host, port) // Establishes a connection
	if err != nil {
		return false, err.Error(), shouldRetry(err) // Handles connection error
	}
	defer conn.Close()

	client, err := smtp.NewClient(conn, host) // Creates an SMTP client
	if err != nil {
		return false, err.Error(), shouldRetry(err) // Handles client error
	}
	defer client.Close()

	// Handles STARTTLS for secure SMTP communication
	if port == "587" {
		if ok, _ := client.Extension("STARTTLS"); ok {
			if err := client.StartTLS(&tls.Config{ServerName: host}); err != nil {
				return false, err.Error(), shouldRetry(err)
			}
		}
	}

	// Executes HELO/EHLO command
	if err := client.Hello(heloDomain); err != nil {
		return false, err.Error(), shouldRetry(err)
	}

	// Executes MAIL FROM command
	if err := client.Mail("test@" + heloDomain); err != nil {
		return false, err.Error(), shouldRetry(err)
	}

	// Executes RCPT TO command
	if err := client.Rcpt(email); err != nil {
		return false, err.Error(), shouldRetry(err)
	}

	return true, "", false // Email exists successfully
}

// Establishes connection to SMTP server with specific settings for secure/non-secure communication
func connect(host, port string) (net.Conn, error) {
	if port == "465" { // Handles secure connection using TLS
		return tls.DialWithDialer(
			&net.Dialer{Timeout: connectTimeout}, // Applies timeout for connection
			"tcp",
			net.JoinHostPort(host, port),  // Joins host and port
			&tls.Config{ServerName: host}, // Specifies server name for TLS
		)
	}
	return net.DialTimeout("tcp", net.JoinHostPort(host, port), connectTimeout) // Handles non-secure connection
}

// Determines whether the error suggests retrying based on its type
func shouldRetry(err error) bool {
	return strings.Contains(err.Error(), "timeout") || // Retry if timeout occurred
		strings.Contains(err.Error(), "connection refused")
}
