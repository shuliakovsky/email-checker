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
	"github.com/shuliakovsky/email-checker/internal/domains"
	"github.com/shuliakovsky/email-checker/internal/logger"
	"github.com/shuliakovsky/email-checker/internal/mx"
	"github.com/shuliakovsky/email-checker/internal/server"
	"github.com/shuliakovsky/email-checker/internal/smtp"
	"github.com/shuliakovsky/email-checker/internal/storage"
	"github.com/shuliakovsky/email-checker/internal/throttle"
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
	pflag.String("admin-key", "", "Admin secret key")
	pflag.String("dns", "1.1.1.1", "DNS server IP address")
	pflag.String("emails", "", "Comma-separated email addresses")
	pflag.Int("workers", 10, "Number of concurrent workers")
	pflag.String("redis", "", "Redis nodes (comma-separated, format: host:port)")
	pflag.String("redis-pass", "", "Redis password")
	pflag.Int("redis-db", 0, "Redis database number")
	pflag.String("port", "8080", "Server port")
	pflag.String("pg-host", "localhost", "PostgreSQL host")
	pflag.Int("pg-port", 5432, "PostgreSQL port")
	pflag.String("pg-user", "postgres", "PostgreSQL user")
	pflag.String("pg-password", "", "PostgreSQL password")
	pflag.String("pg-db", "email_checker", "PostgreSQL database name")
	pflag.String("pg-ssl", "disable", "PostgreSQL SSL mode")
	pflag.Bool("server", false, "Run in server mode")
	pflag.Bool("version", false, "Show version")
	pflag.StringSlice("helo-domains", nil, "[REQUIRED] List of HELO domains for SMTP rotation (comma-separated)")
	viper.BindPFlags(pflag.CommandLine)
	pflag.Parse()

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

	// Base config initialising
	cfg := struct {
		CacheProvider cache.Provider
	}{
		CacheProvider: cache.NewInMemoryCache(),
	}

	throttleManager := throttle.NewThrottleManager(cfg.CacheProvider)
	smtp.SetThrottleManager(throttleManager)

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
			throttleManager,
			viper.GetStringSlice("helo-domains"),
		)
		return
	}

	// CLI mode validations
	if viper.GetString("emails") == "" {
		printVersion()
		log.Fatal("Please specify emails using --emails flag or EMAILS env")
	}
	if len(viper.GetStringSlice("helo-domains")) == 0 {
		printVersion()
		log.Fatal("HELO domains list is required. Use --helo-domains flag or config file")
	}

	// CLI mode execution setup
	mx.InitResolver(viper.GetString("dns"))
	if err := disposable.Init(); err != nil {
		log.Fatalf("Failed to initialize disposable checker: %v", err)
	}
	logger.Init(false) // Initialize the logger

	// Domains initialise for CLI mode
	domains.Init(
		false, // isClusterMode
		nil,   // redisClient
		viper.GetStringSlice("helo-domains"),
	)
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
func startServerMode(port, dns, redisNodes, redisPass string, redisDB, maxWorkers int, throttleManager *throttle.ThrottleManager, heloDomains []string) {
	logger.Init(true) // should be the very first command
	var redisClient redis.UniversalClient
	var cacheProvider cache.Provider
	var store storage.Storage
	var isCluster bool

	db, err := storage.InitPostgres(viper.GetViper())
	if err != nil {
		log.Fatalf("Failed to connect to PostgreSQL: %v", err)
	}

	if len(heloDomains) == 0 {
		logger.Log("[FATAL] HELO domains list is empty")
		log.Fatal("HELO domains required for server mode")
	}

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
	domains.Init(isCluster, redisClient, heloDomains)
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
		throttleManager,
		db,
	)
	logger.Log(fmt.Sprintf("Starting server on port %s | DNS: %s | Workers: %d | Redis: %v",
		port, dns, maxWorkers, redisNodes != ""))

	// Handle potential errors during server startup
	if err := server.Start(); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
