package checker

import (
	"fmt"
	"github.com/shuliakovsky/email-checker/internal/disposable"
	"strings"
	"sync"

	"github.com/shuliakovsky/email-checker/internal/logger"
	"github.com/shuliakovsky/email-checker/internal/mx"
	"github.com/shuliakovsky/email-checker/internal/smtp"
	"github.com/shuliakovsky/email-checker/pkg/types"
)

const (
	maxWorkers = 10 // Number of concurrent workers
)

func ProcessEmails(emails []string) []types.EmailReport {
	jobs := make(chan string, len(emails))               // Channel for email jobs
	results := make(chan types.EmailReport, len(emails)) // Channel for processing results

	var wg sync.WaitGroup // WaitGroup to synchronize goroutines
	wg.Add(maxWorkers)    // Add workers to the WaitGroup

	for i := 0; i < maxWorkers; i++ {
		go worker(jobs, results, &wg) // Start workers
	}

	for _, email := range emails {
		jobs <- strings.TrimSpace(email) // Send trimmed emails to jobs channel
	}
	close(jobs) // Close jobs channel after sending all emails

	go func() {
		wg.Wait()      // Wait for all workers to finish
		close(results) // Close results channel
	}()

	return collectResults(results) // Collect and return results
}

func worker(jobs <-chan string, results chan<- types.EmailReport, wg *sync.WaitGroup) {
	defer wg.Done() // Mark worker as done in WaitGroup
	for email := range jobs {
		logger.Log(fmt.Sprintf("Worker started processing: %s", email))  // Log start
		results <- processEmail(email)                                   // Process email
		logger.Log(fmt.Sprintf("Worker finished processing: %s", email)) // Log end
	}
}

func processEmail(email string) types.EmailReport {
	logger.Log(fmt.Sprintf("Processing email: %s", email)) // Log email processing
	report := types.EmailReport{Email: email}              // Create report

	if !isValidEmail(email) {
		logger.Log(fmt.Sprintf("Invalid email format: %s", email)) // Log invalid format
		report.Valid = false                                       // Mark as invalid
		return report                                              // Return report
	}

	report.Valid = true                                     // Mark as valid
	parts := strings.Split(email, "@")                      // Split email into parts
	domain := parts[1]                                      // Extract domain
	logger.Log(fmt.Sprintf("Extracted domain: %s", domain)) // Log domain
	report.Disposable = disposable.IsDisposable(domain)

	mxRecords, err := mx.GetMXRecords(domain) // Get MX records
	if err != nil {
		logger.Log(fmt.Sprintf("MX lookup failed for %s: %v", domain, err)) // Log error
		report.MX.Error = err.Error()                                       // Add error to report
		return report                                                       // Return report
	}

	report.MX.Valid = len(mxRecords) > 0 // Check MX validity
	for _, record := range mxRecords {
		ttl := calculateTTL(record.Pref)                              // Use record.Pref instead of record.Priority
		report.MX.Records = append(report.MX.Records, types.MXRecord{ // Add MX record
			Host:     strings.TrimSuffix(record.Host, "."),
			Priority: record.Pref, // Correct field
			TTL:      ttl,
		})
	}

	if report.MX.Valid {
		exists, smtpErr, category, permanent, ttl := smtp.CheckEmailExists(email, mxRecords) // Check email existence
		report.Exists = &exists                                                              // Add existence result
		report.SMTPError = smtpErr                                                           // Add SMTP error
		report.ErrorCategory = category                                                      // Add error category
		report.PermanentError = permanent                                                    // Add permanent error flag
		report.TTL = ttl                                                                     // Add TTL
	}

	return report // Return report
}

func isValidEmail(email string) bool {
	return strings.Contains(email, "@") && len(strings.Split(email, "@")) == 2 // Validate email format
}

func collectResults(results <-chan types.EmailReport) []types.EmailReport {
	var collected []types.EmailReport // Initialize results collection
	for res := range results {
		collected = append(collected, res) // Append result to collection
	}
	return collected // Return collected results
}

func calculateTTL(priority uint16) int {
	// Example logic to calculate TTL based on priority
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
