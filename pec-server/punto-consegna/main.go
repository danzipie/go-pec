package main

import (
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/danzipie/go-pec/pec-server/logger"
	"github.com/emersion/go-message"
)

func main() {

	// Initialize logger
	if err := logger.Init("pec.log"); err != nil {
		log.Fatalf("Logger initialization failed: %v", err)
	}
	defer logger.Sync()

	server, err := NewPuntoConsegnaServer("config.json")
	if err != nil {
		log.Fatalf("Failed to create server: %v", err)
	}

	http.HandleFunc("/api/receive", func(w http.ResponseWriter, r *http.Request) {
		ReceiveHandler(w, r, server)
	})
	log.Println("Punto di Consegna HTTP API listening on", server.config.APIServer)

	// Start the server
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

	go func() {
		if err := http.ListenAndServe(server.config.APIServer, nil); err != nil {
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

// ReceiveHandler handles incoming POST requests with RFC822 messages from ForwardToDeliveryPoint.
func ReceiveHandler(w http.ResponseWriter, r *http.Request, s *PuntoConsegnaServer) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if r.Header.Get("Content-Type") != "message/rfc822" {
		http.Error(w, "Unsupported Content-Type", http.StatusUnsupportedMediaType)
		return
	}
	defer r.Body.Close()

	msg, err := message.Read(r.Body)
	if err != nil {
		http.Error(w, "Failed to parse message: "+err.Error(), http.StatusBadRequest)
		return
	}

	// TODO: extract recipient
	session := &PuntoConsegnaSession{
		server: s,
	}

	if session.processMessage(msg, "") != nil {
		http.Error(w, "Failed to process message", http.StatusInternalServerError)
		return
	}

	// Respond with success
	w.WriteHeader(http.StatusOK)
	io.WriteString(w, "Message received")
}
