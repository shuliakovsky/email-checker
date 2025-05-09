package smtp

import (
	"crypto/tls"
	"fmt"
	"net"
	"net/smtp"
	"strings"
	"time"

	"github.com/shuliakovsky/email-checker/internal/domains"  // Domains rotation
	"github.com/shuliakovsky/email-checker/internal/logger"   // Logging utility for activity tracking
	"github.com/shuliakovsky/email-checker/internal/metrics"  // Metrics functionality
	"github.com/shuliakovsky/email-checker/internal/throttle" // Throttling functionality
)

const (
	connectTimeout = 3 * time.Second // Timeout for establishing SMTP connections
	commandTimeout = 8 * time.Second // Timeout for executing SMTP commands
	maxRetries     = 2               // Maximum number of retry attempts for failed connections
	retryDelay     = 1 * time.Second // Delay between consecutive retries
)

var (
	throttleManager *throttle.ThrottleManager
)

func SetThrottleManager(tm *throttle.ThrottleManager) {
	throttleManager = tm
}

// CheckEmailExists validates an email address by interacting with its domain's SMTP servers
func CheckEmailExists(email string, mxRecords []*net.MX) (bool, string, string, bool, int) {
	ports := []string{"25", "587", "465"} // Common SMTP ports (unsecured and secured)
	var (
		maxTTL        int    // Maximum TTL value from temporary SMTP errors
		finalErr      string // Last error encountered during SMTP interactions
		finalCategory string // Classification of the last error
		hasPermanent  bool   // Flag indicating permanent SMTP error
		permanentErr  string // Error message for permanent SMTP failure
		permanentCat  string // Category of the permanent SMTP failure
		tempErrors    int    // Category for temporary errors
	)

	domain := strings.Split(email, "@")[1]

	// Checks for domain throttling
	if throttleManager != nil && throttleManager.IsThrottled(domain) {
		logger.Log(fmt.Sprintf("[Throttle] Domain %s is throttled, skipping checks", domain))
		return false, "domain throttled", "throttled", false, 0
	}

	// Iterate over all MX records and SMTP ports for validation
	for _, mx := range mxRecords {
		mxHost := strings.TrimSuffix(mx.Host, ".")
		for _, port := range ports {
			logger.Log(fmt.Sprintf("Trying %s:%s for %s", mxHost, port, email)) // Log attempt details

			// Attempt validation with retry logic
			exists, err, retry := attemptWithRetry(email, mxHost, port)
			if retry {
				logger.Log(fmt.Sprintf("Retrying %s:%s", mxHost, port)) // Log retry attempt
				time.Sleep(retryDelay)                                  // Pause before retrying
				exists, err, _ = attemptWithRetry(email, mxHost, port)
			}

			if exists { // Email address verified successfully
				return true, "", "", false, 0
			}

			// Process errors returned during validation
			if err != "" {
				category, permanent, ttl := classifySMTPError(err)                      // Classify SMTP error
				logger.Log(fmt.Sprintf("SMTP error: %s (category: %s)", err, category)) // Log error details

				// Специальная обработка RBL ошибки
				if category == "rbl_restriction" {
					if throttleManager != nil {
						// Блокируем домен на 1 минуту
						throttleManager.ThrottleDomainWithTTL(domain, 1*time.Minute)
						logger.Log(fmt.Sprintf("[RBL] Domain %s throttled for 1 minute", domain))
						metrics.RBLRestrictions.Inc()
					}
					// Немедленно прерываем проверку
					return false, "rbl restriction", category, false, 60
				}

				// Counting temp errors
				if !permanent {
					tempErrors++
					metrics.TemporaryErrors.WithLabelValues(domain).Inc()
				}

				// If permanent error, halt further processing
				if permanent {
					hasPermanent = true
					permanentErr = err
					permanentCat = category
					break
				}

				// Track temporary errors with higher TTL
				if ttl > maxTTL {
					maxTTL = ttl
					finalErr = err
					finalCategory = category
				}
			}
		}

		if hasPermanent { // Break loop if permanent error detected
			break
		}
	}

	// Handling temp errors over all MX
	if tempErrors > 0 && tempErrors == len(mxRecords)*len(ports) {
		if throttleManager != nil {
			metrics.ThrottledDomains.Inc()
			logger.Log(fmt.Sprintf("[Throttle] All MX failed for %s, throttling", domain))
			throttleManager.ThrottleDomain(domain)
			throttleManager.ScheduleRetry(email, 1)
		}
		return false, "all MX temporary errors", "temporary", false, maxTTL
	}

	// Return results based on the encountered errors
	if hasPermanent {
		return false, permanentErr, permanentCat, true, 0
	}
	if finalErr != "" {
		return false, finalErr, finalCategory, false, maxTTL
	}
	return false, "", "", false, 0 // Default case when no valid results are obtained
}

// classifySMTPError categorizes SMTP errors as permanent or temporary
func classifySMTPError(errMsg string) (string, bool, int) {
	code := extractSMTPCode(errMsg) // Extract SMTP error code from message

	// Define RBL error by code 5.7.1
	if code == "5.7.1" && strings.Contains(errMsg, "RBL Restriction") {
		return "rbl_restriction", false, 60 // Temporary error TTL 60 sec
	}

	switch {
	case strings.HasPrefix(code, "5"): // Permanent errors start with '5'
		return handlePermanentErrors(code)
	case strings.HasPrefix(code, "4"): // Temporary errors start with '4'
		return handleTemporaryErrors(code)
	default: // Unknown error codes treated as permanent errors
		return "unknown_error", true, 0
	}
}

// extractSMTPCode extracts the SMTP error code for classification purposes
func extractSMTPCode(errMsg string) string {
	parts := strings.SplitN(errMsg, " ", 3) // Split error message for code isolation
	if len(parts) > 0 && len(parts[0]) >= 3 {
		code := parts[0]
		if code[0] == '4' || code[0] == '5' {
			return strings.SplitN(code, ".", 2)[0] // Extract primary error code
		}
	}
	return ""
}

// handlePermanentErrors maps SMTP permanent error codes to categories
func handlePermanentErrors(code string) (string, bool, int) {
	switch code {
	case "550", "551": // Error codes indicating mailbox not found
		return "mailbox_not_found", true, 0
	case "552": // Error code for mailbox full
		return "mailbox_full", true, 0
	case "553", "501": // Error codes for invalid address
		return "invalid_address", true, 0
	case "554": // Error code for transaction failure
		return "transaction_failed", true, 0
	default: // Generic permanent error category
		return "permanent_error", true, 0
	}
}

// handleTemporaryErrors maps SMTP temporary error codes to categories and TTL values
func handleTemporaryErrors(code string) (string, bool, int) {
	ttl := calculateTTL(code) // Compute retry TTL based on error code
	switch code {
	case "421", "450": // Error codes for server unavailable
		return "server_unavailable", false, ttl
	case "451": // Error code for server error
		return "server_error", false, ttl
	case "452": // Error code for storage limit exceeded
		return "storage_limit", false, ttl
	default: // Generic temporary error category
		return "temporary_error", false, ttl
	}
}

// calculateTTL computes retry TTL values for specific temporary error codes
func calculateTTL(code string) int {
	switch code {
	case "421": // Retry after 30 minutes
		return 1800
	case "450": // Retry after 1 hour
		return 3600
	case "451": // Retry after 2 hours
		return 7200
	case "452": // Retry after 4 hours
		return 14400
	default: // Default retry interval of 1 hour
		return 3600
	}
}

// attemptWithRetry executes email validation attempts with a retry mechanism
func attemptWithRetry(email, host, port string) (bool, string, bool) {
	for i := 0; i < maxRetries; i++ {
		exists, err, retry := attempt(email, host, port) // Perform validation attempt
		if !retry {
			return exists, err, false // Stop retries if retry flag is false
		}
		time.Sleep(retryDelay) // Pause before retrying
	}
	return false, "max retries exceeded", false // Default result after max retries
}

// attempt performs a single email validation attempt against the SMTP server
func attempt(email, host, port string) (bool, string, bool) {
	heloDomain, err := domains.GetNext()
	if err != nil {
		return false, fmt.Sprintf("failed to get HELO domain: %v", err), false
	}

	conn, err := connect(host, port)
	if err != nil {
		return false, err.Error(), shouldRetry(err)
	}
	defer conn.Close()

	client, err := smtp.NewClient(conn, host)
	if err != nil {
		return false, err.Error(), shouldRetry(err)
	}
	defer client.Close()

	if port == "587" {
		if ok, _ := client.Extension("STARTTLS"); ok {
			if err := client.StartTLS(&tls.Config{ServerName: host}); err != nil {
				return false, err.Error(), shouldRetry(err)
			}
		}
	}

	if err := client.Hello(heloDomain); err != nil {
		return false, err.Error(), shouldRetry(err)
	}

	if err := client.Mail("test@" + heloDomain); err != nil {
		return false, err.Error(), shouldRetry(err)
	}

	if err := client.Rcpt(email); err != nil {
		return false, err.Error(), shouldRetry(err)
	}

	return true, "", false
}

// connect establishes an SMTP connection using secure or non-secure protocols
func connect(host, port string) (net.Conn, error) {
	if port == "465" { // Establish secure connection using TLS
		return tls.DialWithDialer(
			&net.Dialer{Timeout: connectTimeout}, // Apply connection timeout
			"tcp",
			net.JoinHostPort(host, port),  // Combine host and port for connection
			&tls.Config{ServerName: host}, // Configure server name for TLS
		)
	}
	return net.DialTimeout("tcp", net.JoinHostPort(host, port), connectTimeout) // Non-secure connection
}

// shouldRetry determines if an error warrants retrying the operation
func shouldRetry(err error) bool {
	return strings.Contains(err.Error(), "timeout") || // Retry on timeout
		strings.Contains(err.Error(), "connection refused") // Retry if connection refused
}
