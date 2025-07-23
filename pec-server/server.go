package main

import (
	"crypto/x509"
	"fmt"
	"log"

	"github.com/danzipie/go-pec/pec-server/store"
	"github.com/emersion/go-smtp"
)

// PECServer represents a complete PEC server instance
type PECServer struct {
	config             *Config
	store              store.MessageStore
	signer             *Signer
	smtpAddress        string
	smtpNetworkAddress string
	imapAddress        string
	certificate        *x509.Certificate
	privateKey         interface{}
}

// NewPECServer creates a new PEC server instance
func NewPECServer(configPath string) (*PECServer, error) {
	// Load configuration
	cfg, err := LoadConfig(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load config: %v", err)
	}

	// Load S/MIME credentials
	cert, key, err := LoadSMIMECredentials(cfg.CertFile, cfg.KeyFile)
	if err != nil {
		return nil, fmt.Errorf("failed to load S/MIME credentials: %v", err)
	}

	// Create signer
	signer := &Signer{
		Cert:   cert,
		Key:    key,
		Domain: cfg.Domain,
	}

	// Create message store
	messageStore := store.NewInMemoryStore()

	return &PECServer{
		config:             cfg,
		store:              messageStore,
		signer:             signer,
		smtpAddress:        cfg.SMTPServer,
		smtpNetworkAddress: cfg.SMTPNetworkServer,
		imapAddress:        cfg.IMAPServer,
		certificate:        cert,
		privateKey:         key,
	}, nil
}

// Start starts both SMTP and IMAP servers
func (s *PECServer) Start() error {
	// Create SMTP backend
	smtpBackend := NewBackend(s.signer, s.store)

	// Create IMAP backend
	imapBackend := NewIMAPBackend(s.store, s.certificate, s.privateKey)

	// Start IMAP server in a goroutine
	go func() {
		if err := StartIMAP(s.imapAddress, imapBackend); err != nil {
			log.Printf("IMAP server error: %v", err)
		}
	}()

	go func() {
		// Create punto di consegna server
		PuntoConsegnaServer := NewPuntoConsegnaServer("pec.example.com")

		// Create SMTP server
		smtpServer := smtp.NewServer(PuntoConsegnaServer.NewBackend())
		smtpServer.Addr = ":1026"
		smtpServer.Domain = "pec.example.com"
		smtpServer.AllowInsecureAuth = true // For development only

		log.Println("PEC Punto di Consegna server starting on :1026")
		log.Fatal(smtpServer.ListenAndServe())

	}()

	// Start SMTP server (blocking)
	return StartSMTP(s.smtpAddress, s.config.Domain, smtpBackend)
}

// Stop gracefully shuts down all servers
func (s *PECServer) Stop() error {
	// Close the message store
	if err := s.store.Close(); err != nil {
		return fmt.Errorf("failed to close message store: %v", err)
	}
	return nil
}
