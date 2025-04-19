package logger

import (
	"log"  // Provides functions for logging
	"sync" // Provides synchronization primitives for thread-safe operations
)

// Instance is the singleton instance of BufferedLogger
var Instance *BufferedLogger

// BufferedLogger is a logger with an optional buffering mechanism
type BufferedLogger struct {
	mu        sync.Mutex // Mutex for thread-safe access to the buffer
	buffer    []string   // Buffer to store log messages when immediate output is disabled
	immediate bool       // Determines whether log messages are output immediately or buffered
}

// Init initializes the global BufferedLogger instance with the given configuration
func Init(immediate bool) {
	Instance = &BufferedLogger{
		immediate: immediate, // Set immediate mode based on the input
	}
}

// Log logs a message, either immediately or by buffering it
func Log(msg string) {
	Instance.mu.Lock()         // Acquire the mutex lock for safe access
	defer Instance.mu.Unlock() // Ensure the mutex is released after the operation
	if Instance.immediate {
		log.Println(msg) // Output the message immediately if immediate mode is enabled
	} else {
		Instance.buffer = append(Instance.buffer, msg) // Add the message to the buffer otherwise
	}
}

// Flush writes all buffered messages to the log and clears the buffer
func Flush() {
	Instance.mu.Lock()                    // Acquire the mutex lock for safe access
	defer Instance.mu.Unlock()            // Ensure the mutex is released after the operation
	for _, msg := range Instance.buffer { // Iterate through all buffered messages
		log.Println(msg) // Output each message to the log
	}
	Instance.buffer = nil // Clear the buffer after flushing
}
