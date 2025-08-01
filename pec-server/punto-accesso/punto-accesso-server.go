package main

import (
	"crypto/x509"
	"fmt"
	"log"

	"github.com/danzipie/go-pec/pec-server/internal/common"
	"github.com/danzipie/go-pec/pec-server/store"
)

// PuntoAccessoServer represents a complete Punto accesso server instance
type PuntoAccessoServer struct {
	config      *common.Config
	store       store.MessageStore
	signer      *common.Signer
	smtpAddress string
	imapAddress string
	certificate *x509.Certificate
	privateKey  interface{}
}

// NewPuntoAccessoServer creates a new PEC punto Accesso server instance
func NewPuntoAccessoServer(configPath string) (*PuntoAccessoServer, error) {
	// Load configuration
	cfg, err := common.LoadConfig(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load config: %v", err)
	}

	// Load S/MIME credentials
	cert, key, err := common.LoadSMIMECredentials(cfg.CertFile, cfg.KeyFile)
	if err != nil {
		return nil, fmt.Errorf("failed to load S/MIME credentials: %v", err)
	}

	// Create signer
	signer := &common.Signer{
		Cert:   cert,
		Key:    key,
		Domain: cfg.Domain,
	}

	// Create message store
	messageStore := store.NewInMemoryStore()

	return &PuntoAccessoServer{
		config:      cfg,
		store:       messageStore,
		signer:      signer,
		smtpAddress: cfg.SMTPServer,
		imapAddress: cfg.IMAPServer,
		certificate: cert,
		privateKey:  key,
	}, nil
}

// Start starts both SMTP and IMAP servers
func (s *PuntoAccessoServer) Start() error {
	// Create SMTP backend
	smtpBackend := common.NewBackend(s.signer, s.store, AccessPointHandler, s.config.Domain)

	// Create IMAP backend
	imapBackend := common.NewIMAPBackend(s.store, s.certificate, s.privateKey)

	// Start IMAP server in a goroutine
	go func() {
		if err := common.StartIMAP(s.imapAddress, imapBackend); err != nil {
			log.Printf("IMAP server error: %v", err)
		}
	}()

	// Start SMTP server (blocking)
	return common.StartSMTP(s.smtpAddress, s.config.Domain, smtpBackend)
}

// Stop gracefully shuts down all servers
func (s *PuntoAccessoServer) Stop() error {
	// Close the message store
	if err := s.store.Close(); err != nil {
		return fmt.Errorf("failed to close message store: %v", err)
	}
	return nil
}
