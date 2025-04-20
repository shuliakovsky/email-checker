package logger

import (
	"log"
	"sync"
)

// Instance is the singleton instance of BufferedLogger, shared across the application
var (
	Instance *BufferedLogger // Global logger instance
	initOnce sync.Once       // Ensures that initialization happens only once
)

// BufferedLogger is a logger that can buffer messages or log them immediately
type BufferedLogger struct {
	mu        sync.Mutex // Mutex to ensure thread-safe operations on the buffer
	buffer    []string   // Buffer to store log messages for delayed logging
	immediate bool       // Controls whether log messages are output immediately or stored in the buffer
}

// Init initializes the logger with the specified mode (immediate or buffered)
// Ensures that the logger is initialized only once across the application
func Init(immediate bool) {
	initOnce.Do(func() {
		Instance = &BufferedLogger{
			immediate: immediate, // Set the logging mode
		}
	})
}

// Log adds a message to the buffer or logs it immediately depending on the configuration
// It is safe to call even if Init has not been invoked
func Log(msg string) {
	if Instance == nil {
		log.Println("[WARN] Logger not initialized. Message:", msg) // Log warning if the logger isn't initialized
		return
	}

	Instance.mu.Lock()         // Acquire the mutex lock to ensure thread safety
	defer Instance.mu.Unlock() // Release the lock when the operation completes

	if Instance.immediate {
		log.Println(msg) // Log the message immediately
	} else {
		Instance.buffer = append(Instance.buffer, msg) // Add the message to the buffer
	}
}

// Flush outputs all buffered log messages and clears the buffer
func Flush() {
	Instance.mu.Lock()                    // Acquire the mutex lock for safe access
	defer Instance.mu.Unlock()            // Ensure the lock is released after the operation
	for _, msg := range Instance.buffer { // Iterate over buffered messages
		log.Println(msg) // Log each buffered message
	}
	Instance.buffer = nil // Clear the buffer after flushing
}
