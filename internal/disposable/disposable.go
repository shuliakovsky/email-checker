package disposable

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"
)

const (
	indexURL    = "https://raw.githubusercontent.com/tompec/disposable-email-domains/main/index.json"    // URL to fetch a list of precise disposable domains
	wildcardURL = "https://raw.githubusercontent.com/tompec/disposable-email-domains/main/wildcard.json" // URL to fetch wildcard disposable domains
	timeout     = 10 * time.Second                                                                       // Timeout for HTTP requests
)

var (
	domains     []string            // Slice to store precise disposable domains
	domainSet   map[string]struct{} // Set for fast lookup of precise domains
	wildcards   []string            // Slice to store wildcard disposable domains
	initOnce    sync.Once           // Ensures initialization runs only once
	initialized bool                // Flag indicating successful initialization of data
)

// Init performs one-time initialization to load domain lists
func Init() error {
	var initErr error
	initOnce.Do(func() {
		// Load precise domains from the index URL
		if err := fetchDomains(indexURL, &domains); err != nil {
			initErr = fmt.Errorf("failed to load precise domains: %w", err) // Handle error when loading precise domains
			return
		}

		// Initialize the set for fast domain lookup
		domainSet = make(map[string]struct{}, len(domains))
		for _, domain := range domains {
			domainSet[strings.ToLower(domain)] = struct{}{} // Convert domain names to lowercase and store in the set
		}

		// Load wildcard domains from the wildcard URL
		if err := fetchDomains(wildcardURL, &wildcards); err != nil {
			initErr = fmt.Errorf("failed to load wildcard domains: %w", err) // Handle error when loading wildcard domains
			return
		}

		initialized = true // Mark the initialization as successful
	})
	return initErr
}

// fetchDomains performs an HTTP GET request to fetch domains and populates the provided target variable
func fetchDomains(url string, target interface{}) error {
	client := &http.Client{Timeout: timeout} // Create an HTTP client with a timeout
	resp, err := client.Get(url)
	if err != nil {
		return err // Return error if the HTTP request fails
	}
	defer resp.Body.Close() // Ensure the response body is closed after processing

	// Check if the response status is not HTTP 200 OK
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status code: %d", resp.StatusCode) // Return error for unexpected status codes
	}

	data, err := io.ReadAll(resp.Body) // Read the content from the response body
	if err != nil {
		return err // Return error if reading the response body fails
	}

	return json.Unmarshal(data, target) // Deserialize JSON content into the target variable
}

// IsDisposable determines whether the given domain is disposable
func IsDisposable(domain string) bool {
	if !initialized {
		return false // Return false if the domain lists are not initialized
	}

	domain = strings.ToLower(domain) // Convert the domain name to lowercase for consistency

	// Check for an exact match in the domain set
	if _, exists := domainSet[domain]; exists {
		return true // Return true if the domain exists in the precise domain set
	}

	// Check against wildcard domains
	for _, pattern := range wildcards {
		if strings.HasPrefix(pattern, "*.") { // Identify wildcard patterns
			suffix := strings.ToLower(pattern[2:]) // Extract the suffix from the wildcard pattern
			if strings.HasSuffix(domain, suffix) {
				return true // Return true if the domain matches a wildcard pattern
			}
		}
	}

	return false // Return false if the domain is neither precise nor matches a wildcard
}
