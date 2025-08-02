package main

import (
	"io"
	"log"
	"net/http"

	"github.com/emersion/go-message"
)

func main() {

	server, err := NewPuntoConsegnaServer("config.json")
	if err != nil {
		log.Fatalf("Failed to create server: %v", err)
	}

	http.HandleFunc("/api/receive", func(w http.ResponseWriter, r *http.Request) {
		ReceiveHandler(w, r, server)
	})
	log.Println("Punto di Consegna HTTP API listening on", server.config.APIServer)
	log.Fatal(http.ListenAndServe(server.config.APIServer, nil))

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
