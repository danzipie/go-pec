package main

import (
	"crypto/x509"
	"fmt"

	"github.com/danzipie/go-pec/pec-server/internal/common"
	"github.com/danzipie/go-pec/pec-server/store"
	"github.com/emersion/go-message"
)

// PuntoConsegnaServer represents a complete Punto Consegna server instance
type PuntoConsegnaServer struct {
	config      *common.Config
	store       store.MessageStore
	signer      *common.Signer
	imapAddress string
	certificate *x509.Certificate
	privateKey  interface{}
	mailboxes   map[string]Mailbox // recipient -> mailbox
	domain      string
}

// Mailbox represents a destination mailbox
type Mailbox interface {
	DeliverMessage(msg *message.Entity) error
	IsAvailable() bool
}

// NewPuntoConsegnaServer creates a new PEC punto Consegna server instance
func NewPuntoConsegnaServer(configPath string) (*PuntoConsegnaServer, error) {
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

	return &PuntoConsegnaServer{
		config:      cfg,
		store:       messageStore,
		signer:      signer,
		imapAddress: cfg.IMAPServer,
		certificate: cert,
		privateKey:  key,
		mailboxes:   make(map[string]Mailbox),
		domain:      cfg.Domain,
	}, nil
}

// Start starts both SMTP and IMAP servers
func (s *PuntoConsegnaServer) Start() error {

	// Create IMAP backend
	imapBackend := common.NewIMAPBackend(s.store, s.certificate, s.privateKey)

	// Start IMAP server (blocking)
	return common.StartIMAP(s.imapAddress, imapBackend)
}

// Stop gracefully shuts down all servers
func (s *PuntoConsegnaServer) Stop() error {
	// Close the message store
	if err := s.store.Close(); err != nil {
		return fmt.Errorf("failed to close message store: %v", err)
	}
	return nil
}

func (s *PuntoConsegnaServer) RegisterMailbox(recipient string, mailbox Mailbox) {
	if s.mailboxes == nil {
		s.mailboxes = make(map[string]Mailbox)
	}
	s.mailboxes[recipient] = mailbox
}

func (s *PuntoConsegnaServer) DeliverMessage(to string, msg *message.Entity) error {
	mailbox, exists := s.mailboxes[to]
	if !exists || !mailbox.IsAvailable() {
		return fmt.Errorf("mailbox for recipient %s not found or unavailable", to)
	}

	// Deliver the message to the mailbox
	return mailbox.DeliverMessage(msg)
}
