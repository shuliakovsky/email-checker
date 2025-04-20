package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/shuliakovsky/email-checker/internal/cache"
	"github.com/shuliakovsky/email-checker/internal/checker"
	"github.com/shuliakovsky/email-checker/internal/disposable"
	"github.com/shuliakovsky/email-checker/internal/logger"
	"github.com/shuliakovsky/email-checker/internal/mx"
	"github.com/shuliakovsky/email-checker/internal/server"
	"github.com/shuliakovsky/email-checker/internal/storage"
)

// Application version information
var (
	Version    string = "0.0.1" // Current application version
	CommitHash string = ""      // Git commit hash from build
)

// Displays version information from build
func printVersion() {
	fmt.Printf("email-checker version: %s\n", Version)
	if CommitHash != "" {
		fmt.Printf("commit hash: %s\n", CommitHash)
	}
}

// Main entry point with dual operational modes: CLI and Server
func main() {
	// Command-line flag configurations
	dnsServer := flag.String("dns", "1.1.1.1", "DNS server IP address")
	emails := flag.String("emails", "", "Comma-separated email addresses")
	maxWorkers := flag.Int("workers", 10, "Number of concurrent workers")
	redisNodes := flag.String("redis", "", "Redis nodes (comma-separated, format: host:port)")
	redisPass := flag.String("redis-pass", "", "Redis password")
	redisDB := flag.Int("redis-db", 0, "Redis database number")
	serverPort := flag.String("port", "8080", "Server port")
	serverMode := flag.Bool("server", false, "Run in server mode")
	version := flag.Bool("version", false, "Show version")
	flag.Parse()

	// Handle version display request
	if *version {
		printVersion()
		return
	}

	// Start server mode if requested
	if *serverMode {
		startServerMode(*serverPort, *dnsServer, *redisNodes, *redisPass, *redisDB, *maxWorkers)
		return
	}

	// CLI mode validations
	if *emails == "" {
		printVersion()
		log.Fatal("Please specify emails using --emails flag")
	}

	// CLI mode execution setup
	mx.InitResolver(*dnsServer)
	if err := disposable.Init(); err != nil {
		log.Fatalf("Failed to initialize disposable checker: %v", err)
	}
	logger.Init(false)

	// Process emails with in-memory caching
	emailList := strings.Split(*emails, ",")
	results := checker.ProcessEmailsWithConfig(emailList, checker.Config{
		MaxWorkers:     *maxWorkers,
		CacheProvider:  cache.NewInMemoryCache(),
		DomainCacheTTL: 24 * time.Hour,
		ExistTTL:       720 * time.Hour,
		NotExistTTL:    24 * time.Hour,
	})

	// Output results as formatted JSON
	jsonData, _ := json.MarshalIndent(results, "", "  ")
	fmt.Println(string(jsonData))
}

// Configures and starts server mode with Redis integration
func startServerMode(port, dns, redisNodes, redisPass string, redisDB, maxWorkers int) {
	logger.Init(true)
	var redisClient redis.UniversalClient
	var cacheProvider cache.Provider
	var store storage.Storage
	var isCluster bool

	// Redis configuration logic
	if redisNodes != "" {
		nodes := strings.Split(redisNodes, ",")
		isCluster = len(nodes) > 1

		// Initialize Redis client based on cluster configuration
		if isCluster {
			redisClient = redis.NewClusterClient(&redis.ClusterOptions{
				Addrs:    nodes,
				Password: redisPass,
			})
		} else {
			redisClient = redis.NewClient(&redis.Options{
				Addr:     nodes[0],
				Password: redisPass,
				DB:       redisDB,
			})
		}

		// Verify Redis connection
		if err := redisClient.Ping(context.Background()).Err(); err != nil {
			log.Fatalf("Redis connection failed: %v", err)
		}

		// Configure Redis-based components
		cacheProvider = cache.NewRedisCache(redisClient)
		store = storage.NewRedisStorage(redisClient)
		logger.Log(fmt.Sprintf("Using Redis storage: %v (cluster: %v)", nodes, isCluster))
	} else {
		// Fallback to in-memory storage
		cacheProvider = cache.NewInMemoryCache()
		store = storage.NewMemoryStorage(cacheProvider)
		logger.Log("Using in-memory storage")
	}

	// Common service initialization
	mx.InitResolver(dns)
	mx.SetCacheProvider(cacheProvider)

	if err := disposable.Init(); err != nil {
		log.Fatalf("Failed to initialize disposable checker: %v", err)
	}

	// Create and start HTTP server
	server := server.NewServer(
		port,
		store,
		redisClient,
		maxWorkers,
		isCluster,
	)
	logger.Log(fmt.Sprintf("Starting server on port %s | DNS: %s | Workers: %d | Redis: %v",
		port, dns, maxWorkers, redisNodes != ""))

	if err := server.Start(); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
