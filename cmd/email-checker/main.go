package main

// Import packages
import (
	"encoding/json" // JSON handling
	"flag"          // Command-line flags
	"fmt"           // Formatted output
	"log"           // Logging
	"strings"       // String operations

	// Internal packages
	"github.com/shuliakovsky/email-checker/internal/checker" // Email processing
	"github.com/shuliakovsky/email-checker/internal/logger"  // Logging
	"github.com/shuliakovsky/email-checker/internal/mx"      // MX checker initialising
)

func main() {
	// Define flag for email input
	emails := flag.String("emails", "", "Comma-separated list of emails")
	dnsServer := flag.String("dns", "1.1.1.1", "DNS server IP address")
	flag.Parse() // Parse flags

	// Terminate if emails flag is empty
	if *emails == "" {
		log.Fatal("Please provide emails using --emails flag")
	}
	// Setup custom resolver
	mx.InitResolver(*dnsServer)

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
