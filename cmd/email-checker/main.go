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
)

func main() {
	// Define flag for email input
	emails := flag.String("emails", "", "Comma-separated list of emails")
	flag.Parse() // Parse flags

	// Terminate if emails flag is empty
	if *emails == "" {
		log.Fatal("Please provide emails using --emails flag")
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
