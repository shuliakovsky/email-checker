package logger

import (
	"log"  // Provides logging functions
	"sync" // Provides synchronization primitives
)

var Instance *BufferedLogger // Singleton instance of BufferedLogger

type BufferedLogger struct {
	mu     sync.Mutex // Mutex for synchronization
	buffer []string   // Buffer to store log messages
}

func Init() {
	Instance = &BufferedLogger{} // Initialize the logger instance
}

func Log(msg string) {
	Instance.mu.Lock()                             // Acquire lock
	defer Instance.mu.Unlock()                     // Release lock
	Instance.buffer = append(Instance.buffer, msg) // Append message to buffer
}

func Flush() {
	Instance.mu.Lock()                    // Acquire lock
	defer Instance.mu.Unlock()            // Release lock
	for _, msg := range Instance.buffer { // Iterate through buffered messages
		log.Println(msg) // Log each message
	}
	Instance.buffer = nil // Clear the buffer
}
