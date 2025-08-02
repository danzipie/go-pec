package main

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/danzipie/go-pec/pec-server/internal/common"
	"github.com/danzipie/go-pec/pec-server/logger"
)

// Main entry point for the PEC Punto ricezione server
func main() {
	// Initialize logger
	if err := logger.Init("pec.log"); err != nil {
		log.Fatalf("Logger initialization failed: %v", err)
	}
	defer logger.Sync()

	// Create and start PEC server
	server, err := NewPuntoRicezioneServer("config.json")
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

// Start starts both SMTP and IMAP servers
func (s *PuntoRicezioneServer) Start() error {
	// Create SMTP backend
	smtpBackend := common.NewBackend(s.signer, s.store, ReceptionPointHandler, s.config.Domain)

	// Start SMTP server (blocking)
	return common.StartSMTP(s.smtpAddress, s.config.Domain, smtpBackend)
}

// Stop gracefully shuts down all servers
func (s *PuntoRicezioneServer) Stop() error {
	// Close the message store
	if err := s.store.Close(); err != nil {
		return fmt.Errorf("failed to close message store: %v", err)
	}
	return nil
}
