package main

import (
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"os"
	"testing"
	"time"

	"github.com/danzipie/go-pec/pec-server/internal/common"
	"github.com/danzipie/go-pec/pec-server/store"
)

func TestNewPuntoAccessoServer(t *testing.T) {
	// Create a temporary config file
	configContent := `{
		"domain": "localhost",
		"smtp_server": "localhost:1025",
		"imap_server": "localhost:1143",
		"cert_file": "test_cert.pem",
		"key_file": "test_key.pem"
	}`
	tmpConfig := "test_config.json"
	if err := os.WriteFile(tmpConfig, []byte(configContent), 0644); err != nil {
		t.Fatalf("Failed to create test config: %v", err)
	}
	defer os.Remove(tmpConfig)

	// Create test certificate and key files
	cert, key := createTestCertAndKeyForNonAcceptance(t)
	certPEM, keyPEM := exportCertAndKeyToPEM(cert, key)

	if err := os.WriteFile("test_cert.pem", certPEM, 0644); err != nil {
		t.Fatalf("Failed to write test cert: %v", err)
	}
	defer os.Remove("test_cert.pem")

	if err := os.WriteFile("test_key.pem", keyPEM, 0644); err != nil {
		t.Fatalf("Failed to write test key: %v", err)
	}
	defer os.Remove("test_key.pem")

	// Test server creation
	server, err := NewPuntoAccessoServer(tmpConfig)
	if err != nil {
		t.Fatalf("Failed to create Punto Accesso server: %v", err)
	}

	// Verify server configuration
	if server.config == nil {
		t.Error("Server config is nil")
	}
	if server.store == nil {
		t.Error("Message store is nil")
	}
	if server.signer == nil {
		t.Error("Signer is nil")
	}
	if server.smtpAddress != "localhost:1025" {
		t.Errorf("Expected SMTP address localhost:1025, got %s", server.smtpAddress)
	}
	if server.imapAddress != "localhost:1143" {
		t.Errorf("Expected IMAP address localhost:1143, got %s", server.imapAddress)
	}
}

func TestPuntoAccessoServerIntegration(t *testing.T) {
	t.Skip("Skipping integration test")
	// Create a test server
	server := &PuntoAccessoServer{
		config: &common.Config{
			Domain:     "localhost",
			SMTPServer: "localhost:2025", // Use different ports for test
			IMAPServer: "localhost:2143",
			CertFile:   "test_cert.pem",
			KeyFile:    "test_key.pem",
		},
		store:       store.NewInMemoryStore(),
		smtpAddress: "localhost:2025",
		imapAddress: "localhost:2143",
		certificate: nil, // Will be set by LoadSMIMECredentials
		privateKey:  nil,
	}

	// Create test certificate and key files
	cert, key := createTestCertAndKeyForNonAcceptance(t)
	certPEM, keyPEM := exportCertAndKeyToPEM(cert, key)

	if err := os.WriteFile("test_cert.pem", certPEM, 0644); err != nil {
		t.Fatalf("Failed to write test cert: %v", err)
	}
	defer os.Remove("test_cert.pem")

	if err := os.WriteFile("test_key.pem", keyPEM, 0644); err != nil {
		t.Fatalf("Failed to write test key: %v", err)
	}
	defer os.Remove("test_key.pem")

	// Load the credentials
	cert, privKey, err := common.LoadSMIMECredentials("test_cert.pem", "test_key.pem")
	if err != nil {
		t.Fatalf("Failed to load S/MIME credentials: %v", err)
	}
	server.certificate = cert
	server.privateKey = privKey.(*rsa.PrivateKey)

	// Start the server in a goroutine
	errChan := make(chan error, 1)
	go func() {
		if err := server.Start(); err != nil {
			errChan <- err
		}
	}()

	// Give the server time to start
	time.Sleep(100 * time.Millisecond)

	// TODO: Add SMTP and IMAP client tests here
	// For now, just check if the server started without errors
	select {
	case err := <-errChan:
		t.Fatalf("Server failed to start: %v", err)
	default:
		// Server started successfully
	}
}

// Helper function to export certificate and key to PEM format
func exportCertAndKeyToPEM(cert *x509.Certificate, key interface{}) ([]byte, []byte) {
	// Export certificate to PEM
	certPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "CERTIFICATE",
		Bytes: cert.Raw,
	})

	// Export private key to PEM
	rsaKey := key.(*rsa.PrivateKey)
	keyPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(rsaKey),
	})

	return certPEM, keyPEM
}
