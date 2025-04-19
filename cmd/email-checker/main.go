package main

// Import necessary packages
import (
	"encoding/json" // Provides functions for encoding and decoding JSON
	"flag"          // Handles command-line flags
	"fmt"           // Enables formatted I/O operations
	"log"           // Simplifies logging
	"strings"       // Offers utilities for string manipulation

	// Internal packages for application-specific functionality
	"github.com/shuliakovsky/email-checker/internal/checker"    // Handles email processing logic
	"github.com/shuliakovsky/email-checker/internal/disposable" // Manages disposable email domain checks
	"github.com/shuliakovsky/email-checker/internal/logger"     // Manages application logging
	"github.com/shuliakovsky/email-checker/internal/mx"         // Initializes MX (Mail Exchange) record resolver
	"github.com/shuliakovsky/email-checker/internal/server"     // Sets up server mode functionality
)

// Version and CommitHash are set during the build process
var Version string = "0.0.1" // Application version
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
	dnsServer := flag.String("dns", "1.1.1.1", "DNS server IP address")
	emails := flag.String("emails", "", "Comma-separated list of emails")
	serverMode := flag.Bool("server", false, "Run in server mode")
	serverPort := flag.String("port", "8080", "Server port")
	version := flag.Bool("version", false, "Display the current version of the application")
	flag.Parse() // Parse the command-line flags

	// Display version information if the --version flag is provided
	if *version {
		printVersion()
		return
	}

	// Start in server mode if the --server flag is provided
	if *serverMode {
		startServerMode(*serverPort, *dnsServer)
		return
	}

	// Exit with an error if no emails are provided in CLI mode
	if *emails == "" {
		printVersion()
		log.Fatal("Please provide emails using --emails flag")
	}

	// Initialize the custom DNS resolver
	mx.InitResolver(*dnsServer)

	// Initialize disposable email domain checks
	if err := disposable.Init(); err != nil {
		log.Fatalf("Failed to initialize disposable domains: %v", err) // Log error and exit if initialization fails
	}

	// Initialize the logger for application logging
	logger.Init(*serverMode)

	// Split the provided emails into a list
	emailList := strings.Split(*emails, ",")
	logger.Log(fmt.Sprintf("Starting processing %d emails", len(emailList))) // Log the number of emails to process

	// Process the emails and retrieve results
	results := checker.ProcessEmails(emailList)
	logger.Log(fmt.Sprintf("Processing completed. Total emails processed: %d", len(results))) // Log completion of processing
	logger.Flush()                                                                            // Flush logs to ensure they are outputted

	// Format the processing results as JSON
	jsonData, _ := json.MarshalIndent(results, "", "  ")
	fmt.Println(string(jsonData)) // Print the formatted results

	// Log the final message indicating completion of processing
	logger.Log(fmt.Sprintf("Processing completed. Total emails processed: %d", len(results)))
}

// startServerMode initializes and starts the email-checker server
func startServerMode(port, dns string) {
	// Initialize the DNS resolver with the specified DNS server
	mx.InitResolver(dns)

	// Initialize disposable email domain checks
	if err := disposable.Init(); err != nil {
		log.Fatal(err) // Log error and exit if initialization fails
	}

	// Initialize the logger
	logger.Init(true)

	// Create and start the server on the specified port
	serverMode := server.NewServer(port)
	printVersion()
	log.Printf("Starting server on port %s, DNS resolver %s", port, dns) // Log the server start information
	if err := serverMode.Start(); err != nil {
		log.Fatal(err) // Log error and exit if the server fails to start
	}
}
