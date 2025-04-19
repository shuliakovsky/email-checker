package docs

import (
	"embed" // Provides support for embedding files into the binary
	"log"   // Logging utilities

	"github.com/swaggo/swag" // Swagger package for API documentation
)

//go:embed swagger.json
var swaggerFS embed.FS // Embedded file system for swagger.json

var doc string // Variable to hold the Swagger documentation content

// init initializes the Swagger documentation by reading the embedded swagger.json file
func init() {
	// Read the swagger.json file from the embedded file system
	data, err := swaggerFS.ReadFile("swagger.json")
	if err != nil {
		// Log an error and terminate the application if the file cannot be loaded
		log.Fatalf("Failed to load swagger.json: %v", err)
	}
	doc = string(data) // Convert the file content to a string

	// Define Swagger specification details
	SwaggerInfo := &swag.Spec{
		Version:          "1.0",                                // API version
		Host:             "localhost:8080",                     // API host
		BasePath:         "/",                                  // Base path for the API
		Schemes:          []string{"http"},                     // Supported schemes (HTTP/HTTPS)
		Title:            "Email Checker API",                  // API title
		Description:      "API for email verification service", // API description
		InfoInstanceName: "swagger",                            // Name for Swagger info instance
		SwaggerTemplate:  doc,                                  // JSON content of Swagger documentation
	}

	// Register the Swagger specification
	swag.Register(SwaggerInfo.InstanceName(), SwaggerInfo)
}
