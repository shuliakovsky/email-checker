package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/go-redis/redis/v8"
	"github.com/shuliakovsky/email-checker/internal/cache"
	"github.com/shuliakovsky/email-checker/internal/checker"
	"github.com/shuliakovsky/email-checker/internal/disposable"
	"github.com/shuliakovsky/email-checker/internal/logger"
	"github.com/shuliakovsky/email-checker/internal/mx"
	"github.com/shuliakovsky/email-checker/internal/server"
	"github.com/shuliakovsky/email-checker/internal/storage"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
)

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

// Function to initialize Viper configuration
func initViper() {
	// Configure command-line flags
	pflag.String("dns", "1.1.1.1", "DNS server IP address")
	pflag.String("emails", "", "Comma-separated email addresses")
	pflag.Int("workers", 10, "Number of concurrent workers")
	pflag.String("redis", "", "Redis nodes (comma-separated, format: host:port)")
	pflag.String("redis-pass", "", "Redis password")
	pflag.Int("redis-db", 0, "Redis database number")
	pflag.String("port", "8080", "Server port")
	pflag.Bool("server", false, "Run in server mode")
	pflag.Bool("version", false, "Show version")
	pflag.Parse()

	// Bind flags to Viper settings
	viper.BindPFlags(pflag.CommandLine)

	// Configure environment variables
	viper.AutomaticEnv()
	viper.SetEnvKeyReplacer(strings.NewReplacer("-", "_"))

	// Read configuration file if available
	viper.SetConfigName("config")
	viper.AddConfigPath(".")
	viper.AddConfigPath("/etc/email-checker/")
	if err := viper.ReadInConfig(); err == nil {
		log.Println("Using config file:", viper.ConfigFileUsed())
	}

	// Monitor for changes in the configuration file
	viper.WatchConfig()
	viper.OnConfigChange(func(e fsnotify.Event) {
		log.Println("Config file changed:", e.Name)
	})
}

// Main entry point with dual operational modes: CLI and Server
func main() {
	initViper() // Initialize configuration

	// Handle version display request
	if viper.GetBool("version") {
		printVersion()
		return
	}

	// Start server mode if requested
	if viper.GetBool("server") {
		startServerMode(
			viper.GetString("port"),
			viper.GetString("dns"),
			viper.GetString("redis"),
			viper.GetString("redis-pass"),
			viper.GetInt("redis-db"),
			viper.GetInt("workers"),
		)
		return
	}

	// CLI mode validations
	if viper.GetString("emails") == "" {
		printVersion()
		log.Fatal("Please specify emails using --emails flag or EMAILS env")
	}

	// CLI mode execution setup
	mx.InitResolver(viper.GetString("dns"))
	if err := disposable.Init(); err != nil {
		log.Fatalf("Failed to initialize disposable checker: %v", err)
	}
	logger.Init(false) // Initialize the logger

	// Process emails with in-memory caching
	emailList := strings.Split(viper.GetString("emails"), ",")
	results := checker.ProcessEmailsWithConfig(emailList, checker.Config{
		MaxWorkers:     viper.GetInt("workers"),
		CacheProvider:  cache.NewInMemoryCache(),
		DomainCacheTTL: 24 * time.Hour,
		ExistTTL:       720 * time.Hour,
		NotExistTTL:    24 * time.Hour,
	})

	// Output results as formatted JSON
	jsonData, _ := json.MarshalIndent(results, "", "  ")
	fmt.Println(string(jsonData))
}

// Configures and starts server mode with Redis integration (if presents)
func startServerMode(port, dns, redisNodes, redisPass string, redisDB, maxWorkers int) {
	logger.Init(true) // should be the very first command
	var redisClient redis.UniversalClient
	var cacheProvider cache.Provider
	var store storage.Storage
	var isCluster bool

	// Redis configuration logic
	if redisNodes != "" {
		nodes := strings.Split(redisNodes, ",")
		isCluster = len(nodes) > 1 // Determine if Redis is in cluster mode

		//  Initialize Redis client based on  cluster/non-cluster configuration
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

		//  Configure Redis-based components: cache and storage
		cacheProvider = cache.NewRedisCache(redisClient)
		store = storage.NewRedisStorage(redisClient)
		logger.Log(fmt.Sprintf("Using Redis storage: %v (cluster: %v)", nodes, isCluster))
	} else {
		// Fallback to in-memory storage
		cacheProvider = cache.NewInMemoryCache()
		store = storage.NewMemoryStorage(cacheProvider)
		logger.Log("Using in-memory storage")
	}

	// Common service initialization DNS resolver and Cache provider
	mx.InitResolver(dns)
	mx.SetCacheProvider(cacheProvider)

	// Initialize disposable checker
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

	// Handle potential errors during server startup
	if err := server.Start(); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
