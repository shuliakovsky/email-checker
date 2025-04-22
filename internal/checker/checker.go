package checker

import (
	"fmt"
	"net"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/shuliakovsky/email-checker/internal/cache"      // Handles cache operations
	"github.com/shuliakovsky/email-checker/internal/disposable" // Checks disposable email domains
	"github.com/shuliakovsky/email-checker/internal/logger"     // Provides logging capabilities
	"github.com/shuliakovsky/email-checker/internal/metrics"    // Prometheus metrics
	"github.com/shuliakovsky/email-checker/internal/mx"         // Retrieves MX records
	"github.com/shuliakovsky/email-checker/internal/smtp"       // Handles SMTP checks
	"github.com/shuliakovsky/email-checker/internal/throttle"   // ThrottleManager functionalities
	"github.com/shuliakovsky/email-checker/pkg/types"           // Defines custom types, like EmailReport
)

// Config holds the configuration settings for email processing
type Config struct {
	MaxWorkers      int                       // Maximum number of concurrent workers
	CacheProvider   cache.Provider            // Cache implementation to store processed data
	DomainCacheTTL  time.Duration             // TTL for domain-related cache entries
	ExistTTL        time.Duration             // TTL for existing emails (e.g., 30 days)
	NotExistTTL     time.Duration             // TTL for non-existing emails (e.g., 24 hours)
	ThrottleManager *throttle.ThrottleManager // ThrottleManager implementation
}

// DefaultConfig provides default settings for email processing
var (
	DefaultConfig = Config{
		MaxWorkers:     10,                       // Default worker count, adjustable via flags
		CacheProvider:  cache.NewInMemoryCache(), // Default in-memory cache instance
		DomainCacheTTL: 24 * time.Hour,           // Cache domain details for 24 hours
		ExistTTL:       720 * time.Hour,          // Cache existing emails for 30 days
		NotExistTTL:    24 * time.Hour,           // Cache non-existing emails for 24 hours
	}
)

// ProcessEmailsWithConfig processes a list of emails using the provided configuration
func ProcessEmailsWithConfig(emails []string, cfg Config) []types.EmailReport {
	jobs := make(chan string, len(emails))               // Channel to store jobs (emails to process)
	results := make(chan types.EmailReport, len(emails)) // Channel to store results

	var wg sync.WaitGroup
	wg.Add(cfg.MaxWorkers)

	// Start worker goroutines
	for i := 0; i < cfg.MaxWorkers; i++ {
		go worker(jobs, results, &wg, cfg)
	}

	// Submit jobs to workers
	for _, email := range emails {
		jobs <- strings.TrimSpace(email) // Trim spaces before processing
	}
	close(jobs)

	// Wait for workers to finish and close the results channel
	go func() {
		wg.Wait()
		close(results)
	}()
	return collectResults(results)
}

// ProcessEmails is a shortcut for processing emails using default settings
func ProcessEmails(emails []string) []types.EmailReport {
	return ProcessEmailsWithConfig(emails, DefaultConfig)
}

// Worker processes emails using cache and SMTP validation
func worker(jobs <-chan string, results chan<- types.EmailReport, wg *sync.WaitGroup, cfg Config) {
	defer wg.Done() // Signal worker completion

	for email := range jobs {
		// Normalize email address
		normalizedEmail := strings.ToLower(strings.TrimSpace(email))
		logger.Log(fmt.Sprintf("[Worker] Processing: %s", normalizedEmail))

		// Check if the email exists in cache
		if cached, ok := cfg.CacheProvider.Get(normalizedEmail); ok {
			logger.Log(fmt.Sprintf("[Cache] Hit for: %s", normalizedEmail))
			results <- cached.(types.EmailReport) // Use cached data
			continue
		}

		// Process the email and generate a report
		report := processEmail(normalizedEmail, cfg)
		// Process metrics
		metrics.EmailsChecked.Inc()
		results <- report

		// Cache the result with an appropriate TTL
		ttl := cfg.NotExistTTL
		if report.Exists != nil && *report.Exists { // Adjust TTL for existing emails
			ttl = cfg.ExistTTL
		}
		cfg.CacheProvider.Set(normalizedEmail, report, ttl)
	}
}

// processEmail performs validation, domain checks, and SMTP verification for an email
func processEmail(email string, cfg Config) types.EmailReport {
	logger.Log(fmt.Sprintf("[Processing] Email: %s", email))
	report := types.EmailReport{Email: email}

	// Validate email format
	if !isValidEmail(email) {
		report.Valid = false
		return report
	}
	report.Valid = true

	// Extract domain from the email address
	parts := strings.Split(email, "@")
	domain := parts[1]

	// Check if the domain is disposable
	report.Disposable = disposable.IsDisposable(domain)

	// Retrieve MX records with caching
	var mxRecords []*net.MX
	if cached, ok := cfg.CacheProvider.Get("mx:" + domain); ok {
		mxRecords = cached.([]*net.MX) // Use cached MX records
		logger.Log(fmt.Sprintf("[Cache] MX for %s", domain))
	} else {
		records, err := mx.GetMXRecords(domain)
		if err != nil {
			report.MX.Error = err.Error() // Log the error and return the report
			return report
		}
		mxRecords = records
		cfg.CacheProvider.Set("mx:"+domain, mxRecords, cfg.DomainCacheTTL)
	}

	// Populate MX data in the report
	report.MX.Valid = len(mxRecords) > 0
	for _, record := range mxRecords {
		report.MX.Records = append(report.MX.Records, types.MXRecord{
			Host:     strings.TrimSuffix(record.Host, "."),
			Priority: record.Pref,
			TTL:      calculateTTL(record.Pref),
		})
	}

	// Perform SMTP validation if MX records are valid
	if report.MX.Valid {
		exists, smtpErr, category, permanent, ttl := smtp.CheckEmailExists(email, mxRecords)
		report.Exists = &exists
		report.SMTPError = smtpErr
		report.ErrorCategory = category
		report.PermanentError = permanent
		report.TTL = ttl
	}

	// Save the report in cache even if SMTP validation wasn't performed
	cfg.CacheProvider.Set(email, report, cfg.ExistTTL)
	return report
}

// isValidEmail checks if an email address has a valid format
func isValidEmail(email string) bool {
	const pattern = `(?i)^(?:[a-z0-9!#$%&'*+/=?^_{|}~-]+` +
		`(?:\.[a-z0-9!#$%&'*+/=?^_{|}~-]+)*` +
		`|"(?:[\x01-\x08\x0b\x0c\x0e-\x1f\x21\x23-\x5b\x5d-\x7f]|\
\[\x01-\x09\x0b\x0c\x0e-\x7f])*")` +
		`@(?:(?:[a-z0-9](?:[a-z0-9-]*[a-z0-9])?\.)+` +
		`[a-z]{2,}|
\[(?:(?:25[0-5]|2[0-4][0-9]|[01]?[0-9][0-9]?)\.){3}` +
		`(?:25[0-5]|2[0-4][0-9]|[01]?[0-9][0-9]?)\]
|IPv6:[\da-f:]+\]
)$`

	// Check the overall length (RFC 3696)
	if len(email) > 254 {
		return false
	}

	re := regexp.MustCompile(pattern)
	return re.MatchString(email)
}

// collectResults gathers results from the results channel into a slice
func collectResults(results <-chan types.EmailReport) []types.EmailReport {
	var collected []types.EmailReport
	for res := range results {
		collected = append(collected, res)
	}
	return collected
}

// calculateTTL estimates TTL based on MX record priority
func calculateTTL(priority uint16) int {
	switch priority {
	case 10:
		return 3600
	case 20:
		return 7200
	case 30:
		return 14400
	default:
		return 3600
	}
}
