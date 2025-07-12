package main

import (
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/danzipie/go-pec/pec-server/logger"
)

func main() {
	// Initialize logger
	if err := logger.Init("pec.log"); err != nil {
		log.Fatalf("Logger initialization failed: %v", err)
	}
	defer logger.Sync()

	// Create and start PEC server
	server, err := NewPECServer("config.json")
	if err != nil {
		log.Fatalf("Failed to create PEC server: %v", err)
	}

	// Handle graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Start server in a goroutine
	errChan := make(chan error, 1)
	go func() {
		if err := server.Start(); err != nil {
			errChan <- err
		}
	}()

	// Wait for either an error or a signal
	select {
	case err := <-errChan:
		log.Fatalf("Server error: %v", err)
	case sig := <-sigChan:
		log.Printf("Received signal %v, shutting down...", sig)
		if err := server.Stop(); err != nil {
			log.Printf("Error during shutdown: %v", err)
		}
	}
}
