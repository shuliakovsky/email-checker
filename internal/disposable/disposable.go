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
	indexURL    = "https://raw.githubusercontent.com/tompec/disposable-email-domains/main/index.json"
	wildcardURL = "https://raw.githubusercontent.com/tompec/disposable-email-domains/main/wildcard.json"
	timeout     = 10 * time.Second
)

var (
	domains     []string            // for precise disposal domains
	domainSet   map[string]struct{} // for fast search
	wildcards   []string            // Wildcard domains
	initOnce    sync.Once
	initialized bool
)

func Init() error {
	var initErr error
	initOnce.Do(func() {
		// Loading precise domains as array
		if err := fetchDomains(indexURL, &domains); err != nil {
			initErr = fmt.Errorf("failed to load index: %w", err)
			return
		}

		// Set for fast search
		domainSet = make(map[string]struct{}, len(domains))
		for _, domain := range domains {
			domainSet[strings.ToLower(domain)] = struct{}{}
		}

		// Loading wildcards domains
		if err := fetchDomains(wildcardURL, &wildcards); err != nil {
			initErr = fmt.Errorf("failed to load wildcards: %w", err)
			return
		}

		initialized = true
	})
	return initErr
}

func fetchDomains(url string, target interface{}) error {
	client := &http.Client{Timeout: timeout}
	resp, err := client.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	return json.Unmarshal(data, target)
}

func IsDisposable(domain string) bool {
	if !initialized {
		return false
	}

	domain = strings.ToLower(domain)

	// check for domains
	if _, exists := domainSet[domain]; exists {
		return true
	}

	// check for wildcard
	for _, pattern := range wildcards {
		if strings.HasPrefix(pattern, "*.") {
			suffix := strings.ToLower(pattern[2:])
			if strings.HasSuffix(domain, suffix) {
				return true
			}
		}
	}

	return false
}
