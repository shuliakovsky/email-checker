package main

// Import packages
import (
	"encoding/json" // JSON handling
	"flag"          // Command-line flags
	"fmt"           // Formatted output
	"log"           // Logging
	"strings"       // String operations

	// Internal packages
	"github.com/shuliakovsky/email-checker/internal/checker"    // Email processing
	"github.com/shuliakovsky/email-checker/internal/disposable" // Disposable domains
	"github.com/shuliakovsky/email-checker/internal/logger"     // Logging
	"github.com/shuliakovsky/email-checker/internal/mx"         // MX checker initialising
)

// Version and CommitHash will be set during the build process
var Version string = "0.0.1"
var CommitHash string = ""

func printVersion() {
	fmt.Printf("email-checker version: %s\n", Version)
	if CommitHash != "" {
		fmt.Printf("commit hash: %s\n", CommitHash)
	}
}
func main() {
	// Define flag for email input
	emails := flag.String("emails", "", "Comma-separated list of emails")
	dnsServer := flag.String("dns", "1.1.1.1", "DNS server IP address")
	version := flag.Bool("version", false, "Display the current version of the application")
	flag.Parse() // Parse flags

	// Print version
	if *version {
		printVersion()
		return
	}
	// Terminate if emails flag is empty
	if *emails == "" {
		printVersion()
		log.Fatal("Please provide emails using --emails flag")
	}

	// Setup custom resolver
	mx.InitResolver(*dnsServer)

	// Initialize disposable checks
	if err := disposable.Init(); err != nil {
		log.Fatalf("Failed to initialize disposable domains: %v", err)
	}

	// Initialize logger
	logger.Init()

	// Split email input into list
	emailList := strings.Split(*emails, ",")
	// Log start of processing
	logger.Log(fmt.Sprintf("Starting processing %d emails", len(emailList)))

	// Process emails
	results := checker.ProcessEmails(emailList)
	// Log processing completion
	logger.Log(fmt.Sprintf("Processing completed. Total emails processed: %d", len(results)))
	logger.Flush() // Output all logs

	// Format results as JSON
	jsonData, _ := json.MarshalIndent(results, "", "  ")
	fmt.Println(string(jsonData)) // Print JSON

	// Log final message
	logger.Log(fmt.Sprintf("Processing completed. Total emails processed: %d", len(results)))
}
