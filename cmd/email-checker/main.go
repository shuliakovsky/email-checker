package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/shuliakovsky/email-checker/internal/cache"      // Handles caching operations
	"github.com/shuliakovsky/email-checker/internal/checker"    // Processes email validation logic
	"github.com/shuliakovsky/email-checker/internal/disposable" // Manages checks for disposable email domains
	"github.com/shuliakovsky/email-checker/internal/logger"     // Manages application logging
	"github.com/shuliakovsky/email-checker/internal/mx"         // Initializes MX (Mail Exchange) record resolver
	"github.com/shuliakovsky/email-checker/internal/server"     // Implements server mode functionality
	"github.com/shuliakovsky/email-checker/internal/storage"    // Provides storage solutions
)

// Version and CommitHash are set during the build process
var Version string = "0.0.1" // The application version
var CommitHash string = ""   // Git commit hash (empty by default)

// printVersion displays the application version and commit hash (if available)
func printVersion() {
	fmt.Printf("email-checker version: %s\n", Version)
	if CommitHash != "" {
		fmt.Printf("commit hash: %s\n", CommitHash)
	}
}

// main is the entry point of the application
func main() {
	// Define command-line flags
	dnsServer := flag.String("dns", "1.1.1.1", "IP address of the DNS server")
	emails := flag.String("emails", "", "Comma-separated list of email addresses to process")
	maxWorkers := flag.Int("workers", 10, "Number of concurrent workers for processing")
	redisAddr := flag.String("redis", "", "Address of the Redis server (e.g., localhost:6379)")
	redisPass := flag.String("redis-pass", "", "Password for the Redis server")
	redisDB := flag.Int("redis-db", 0, "Redis database number")
	serverMode := flag.Bool("server", false, "Run the application in server mode")
	serverPort := flag.String("port", "8080", "Port for the server to listen on")
	version := flag.Bool("version", false, "Display the current application version")
	flag.Parse() // Parse the command-line flags

	// Display version information if the --version flag is provided
	if *version {
		printVersion()
		return
	}

	// Start the application in server mode if the --server flag is provided
	if *serverMode {
		startServerMode(*serverPort, *dnsServer, *redisAddr, *redisPass, *redisDB, *maxWorkers)
		return
	}

	// Exit with an error if no email addresses are provided in CLI mode
	if *emails == "" {
		printVersion()
		log.Fatal("Please specify email addresses using the --emails flag")
	}

	// Initialize the custom DNS resolver
	mx.InitResolver(*dnsServer)

	// Initialize disposable email domain checker
	if err := disposable.Init(); err != nil {
		log.Fatalf("Failed to initialize disposable domain checker: %v", err)
	}

	// Initialize the application logger
	logger.Init(*serverMode)

	// Split the provided email addresses into a list
	emailList := strings.Split(*emails, ",")
	logger.Log(fmt.Sprintf("Starting to process %d emails", len(emailList)))

	// Process the emails and retrieve the results
	results := checker.ProcessEmailsWithConfig(emailList, checker.Config{
		MaxWorkers:     *maxWorkers, // Use the value from the command-line flag
		CacheProvider:  cache.NewInMemoryCache(),
		DomainCacheTTL: 24 * time.Hour,  // Time-to-live for domain cache
		ExistTTL:       720 * time.Hour, // Time-to-live for existing emails
		NotExistTTL:    24 * time.Hour,  // Time-to-live for non-existing emails
	})
	logger.Log(fmt.Sprintf("Processing completed. Total emails processed: %d", len(results)))
	logger.Flush() // Ensure all logs are written to output

	// Format the processing results as JSON
	jsonData, _ := json.MarshalIndent(results, "", "  ")
	fmt.Println(string(jsonData)) // Print the formatted results

	// Log the final message indicating processing completion
	logger.Log(fmt.Sprintf("Processing completed. Total emails processed: %d", len(results)))
}

// startServerMode initializes and starts the email-checker server
func startServerMode(port, dns, redisAddr, redisPass string, redisDB int, maxWorkers int) {
	logger.Init(true)
	printVersion()

	// Initialize the cache provider
	var cacheProvider cache.Provider
	if redisAddr != "" {
		cacheProvider = cache.NewRedisCache(redisAddr, redisPass, redisDB)
	} else {
		cacheProvider = cache.NewInMemoryCache()
	}

	// Initialize the storage backend
	var store storage.Storage
	var err error
	if redisAddr != "" {
		store, err = storage.NewRedisStorage(redisAddr, redisPass, redisDB, cacheProvider)
		if err != nil {
			logger.Log(fmt.Sprintf("Failed to connect to Redis: %v", err))
			log.Fatalf("Redis initialization failed: %v", err)
		}
		logger.Log(fmt.Sprintf("Using Redis storage at %s (DB %d)", redisAddr, redisDB))
	} else {
		store = storage.NewMemoryStorage(cacheProvider)
		logger.Log("Using in-memory storage")
	}

	// Initialize DNS resolver
	mx.InitResolver(dns)

	// Initialize disposable email domain checker
	if err := disposable.Init(); err != nil {
		logger.Log(fmt.Sprintf("Failed to initialize disposable domain checker: %v", err))
		log.Fatal(err)
	}

	// Configure and start the server
	serverMode := server.NewServer(port, store, maxWorkers)
	logger.Log(fmt.Sprintf("Starting server on port %s | DNS: %s | Workers: %d", port, dns, maxWorkers))

	// Handle errors during server startup
	if err := serverMode.Start(); err != nil {
		logger.Log(fmt.Sprintf("Failed to start server: %v", err))
		log.Fatal(err)
	}
}
