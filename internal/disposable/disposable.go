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
	indexURL    = "https://raw.githubusercontent.com/tompec/disposable-email-domains/main/index.json"    // URL for loading precise disposable domains
	wildcardURL = "https://raw.githubusercontent.com/tompec/disposable-email-domains/main/wildcard.json" // URL for loading wildcard disposable domains
	timeout     = 10 * time.Second                                                                       // Timeout for HTTP requests
)

var (
	domains     []string            // List of precise disposable domains
	domainSet   map[string]struct{} // Set for fast lookup of precise domains
	wildcards   []string            // List of wildcard disposable domains
	initOnce    sync.Once           // Ensures one-time initialization
	initialized bool                // Indicates whether data has been initialized successfully
)

// Init performs one-time initialization of domain lists
func Init() error {
	var initErr error
	initOnce.Do(func() {
		// Loading precise domains as an array
		if err := fetchDomains(indexURL, &domains); err != nil {
			initErr = fmt.Errorf("failed to load index: %w", err) // Error when loading precise domains
			return
		}

		// Initializing the set for fast lookup
		domainSet = make(map[string]struct{}, len(domains))
		for _, domain := range domains {
			domainSet[strings.ToLower(domain)] = struct{}{} // Convert domain to lowercase and add to the set
		}

		// Loading wildcard domains
		if err := fetchDomains(wildcardURL, &wildcards); err != nil {
			initErr = fmt.Errorf("failed to load wildcards: %w", err) // Error when loading wildcard domains
			return
		}

		initialized = true // Set the flag indicating successful initialization
	})
	return initErr
}

// fetchDomains loads data from the specified URL and populates the target object
func fetchDomains(url string, target interface{}) error {
	client := &http.Client{Timeout: timeout} // Initialize HTTP client with timeout
	resp, err := client.Get(url)
	if err != nil {
		return err // Return error if the request fails
	}
	defer resp.Body.Close() // Ensure the response body is closed after usage

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status code: %d", resp.StatusCode) // Error if status code is not 200 OK
	}

	data, err := io.ReadAll(resp.Body) // Read the response body content
	if err != nil {
		return err // Return error if reading fails
	}

	return json.Unmarshal(data, target) // Deserialize JSON data into the target object
}

// IsDisposable checks if the given domain is disposable
func IsDisposable(domain string) bool {
	if !initialized {
		return false // Return false if data is not yet initialized
	}

	domain = strings.ToLower(domain) // Convert domain to lowercase

	// Check for exact match
	if _, exists := domainSet[domain]; exists {
		return true // Return true if the domain exists in the set
	}

	// Check for wildcard match
	for _, pattern := range wildcards {
		if strings.HasPrefix(pattern, "*.") {
			suffix := strings.ToLower(pattern[2:]) // Extract the suffix from the wildcard pattern
			if strings.HasSuffix(domain, suffix) {
				return true // Return true if the domain matches the wildcard pattern
			}
		}
	}

	return false // Return false if the domain is not found
}
