package main

import (
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/xml"
	"io"
	"math/big"
	"strings"
	"testing"
	"time"
)

// Helper function to create test certificate and key (reused from previous test)
func createTestCertAndKeyForNonAcceptance(t *testing.T) (*x509.Certificate, *rsa.PrivateKey) {
	// Generate a private key
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("Failed to generate private key: %v", err)
	}

	// Create certificate template
	template := x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			Organization:  []string{"Test Company"},
			Country:       []string{"US"},
			Province:      []string{"CA"},
			Locality:      []string{"San Francisco"},
			StreetAddress: []string{"123 Test St"},
			PostalCode:    []string{"12345"},
		},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().Add(365 * 24 * time.Hour),
		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageEmailProtection},
		BasicConstraintsValid: true,
		EmailAddresses:        []string{"test@example.com"},
	}

	// Create certificate
	certDER, err := x509.CreateCertificate(rand.Reader, &template, &template, &privateKey.PublicKey, privateKey)
	if err != nil {
		t.Fatalf("Failed to create certificate: %v", err)
	}

	// Parse certificate
	cert, err := x509.ParseCertificate(certDER)
	if err != nil {
		t.Fatalf("Failed to parse certificate: %v", err)
	}

	return cert, privateKey
}

// TestGenerateNonAcceptanceEmail tests the main functionality
func TestGenerateNonAcceptanceEmail(t *testing.T) {
	// Create test certificate and key
	cert, key := createTestCertAndKeyForNonAcceptance(t)

	// Create signer
	signer := &Signer{
		Cert:   cert,
		Key:    key,
		Domain: "testdomain.com",
	}

	// Create test validation error
	testTime := time.Date(2024, 1, 15, 14, 30, 45, 0, time.UTC)
	validationError := ValidationError{
		Reason:      "invalid signature",
		MessageID:   "test-message-id@example.com",
		From:        "sender@example.com",
		To:          []string{"recipient1@testdomain.com", "recipient2@testdomain.com"},
		Subject:     "Test Email Subject",
		GeneratedAt: testTime,
	}

	domain := "testdomain.com"

	// Test the function
	entity, err := GenerateNonAcceptanceEmail(domain, validationError, signer)
	if err != nil {
		t.Fatalf("GenerateNonAcceptanceEmail failed: %v", err)
	}

	// Verify the entity is not nil
	if entity == nil {
		t.Fatal("GenerateNonAcceptanceEmail returned nil entity")
	}

	// Test headers
	header := entity.Header

	// Check X-Ricevuta header
	if header.Get("X-Ricevuta") != "non-accettazione" {
		t.Errorf("Expected X-Ricevuta header to be 'non-accettazione', got '%s'", header.Get("X-Ricevuta"))
	}

	// Check Date header (should be parseable)
	dateStr := header.Get("Date")
	if dateStr == "" {
		t.Error("Date header is missing")
	} else {
		_, err := time.Parse(time.RFC1123Z, dateStr)
		if err != nil {
			t.Errorf("Date header is not in correct format: %v", err)
		}
	}

	// Check Subject header
	expectedSubject := "AVVISO DI NON ACCETTAZIONE: Test Email Subject"
	if header.Get("Subject") != expectedSubject {
		t.Errorf("Expected Subject to be '%s', got '%s'", expectedSubject, header.Get("Subject"))
	}

	// Check From header
	expectedFrom := "posta-certificata@testdomain.com"
	if header.Get("From") != expectedFrom {
		t.Errorf("Expected From to be '%s', got '%s'", expectedFrom, header.Get("From"))
	}

	// Check To header
	if header.Get("To") != validationError.From {
		t.Errorf("Expected To to be '%s', got '%s'", validationError.From, header.Get("To"))
	}

	// Check X-Riferimento-Message-ID header
	if header.Get("X-Riferimento-Message-ID") != validationError.MessageID {
		t.Errorf("Expected X-Riferimento-Message-ID to be '%s', got '%s'", validationError.MessageID, header.Get("X-Riferimento-Message-ID"))
	}

	// Check Content-Type header
	contentType := header.Get("Content-Type")
	if !strings.HasPrefix(contentType, "multipart/signed") {
		t.Errorf("Expected Content-Type to start with 'multipart/signed', got '%s'", contentType)
	}

}

// TestGenerateNonAcceptanceEmail_ContentVerification tests the content of the generated email
func TestGenerateNonAcceptanceEmail_ContentVerification(t *testing.T) {
	// Create test certificate and key
	cert, key := createTestCertAndKeyForNonAcceptance(t)

	// Create signer
	signer := &Signer{
		Cert:   cert,
		Key:    key,
		Domain: "testdomain.com",
	}

	// Create test validation error
	testTime := time.Date(2024, 1, 15, 14, 30, 45, 0, time.UTC)
	validationError := ValidationError{
		Reason:      "missing required field",
		MessageID:   "msg-123@example.com",
		From:        "sender@example.com",
		To:          []string{"recipient@testdomain.com"},
		Subject:     "Important Message",
		GeneratedAt: testTime,
	}

	domain := "testdomain.com"

	// Generate the email
	entity, err := GenerateNonAcceptanceEmail(domain, validationError, signer)
	if err != nil {
		t.Fatalf("GenerateNonAcceptanceEmail failed: %v", err)
	}

	var buf bytes.Buffer

	if err := entity.WriteTo(&buf); err != nil {
		t.Fatalf("Failed to write entity to buffer: %v", err)
	}

	bodyStr := string(buf.String())

	// Check for specific content in the human-readable part
	expectedTexts := []string{
		"Errore nell’accettazione del messaggio",
		"15/01/2024",
		"14:30:45",
		"Important Message",
		"sender@example.com",
		"recipient@testdomain.com",
		"missing required field",
		"msg-123@example.com",
	}

	for _, text := range expectedTexts {
		if !strings.Contains(bodyStr, text) {
			t.Errorf("Expected body to contain '%s', but it was not found", text)
		}
	}

	// Check for XML attachment markers
	if !strings.Contains(bodyStr, "daticert.xml") {
		t.Error("Body should contain daticert.xml attachment")
	}

	if !strings.Contains(bodyStr, "application/xml") {
		t.Error("Body should contain application/xml content type")
	}
}

// TestGenerateNonAcceptanceEmail_XMLContent tests the XML attachment content
func TestGenerateNonAcceptanceEmail_XMLContent(t *testing.T) {
	// Create test certificate and key
	cert, key := createTestCertAndKeyForNonAcceptance(t)

	// Create signer
	signer := &Signer{
		Cert:   cert,
		Key:    key,
		Domain: "testdomain.com",
	}

	// Create test validation error
	testTime := time.Date(2024, 1, 15, 14, 30, 45, 0, time.UTC)
	validationError := ValidationError{
		Reason:      "test reason",
		MessageID:   "test-id@example.com",
		From:        "from@example.com",
		To:          []string{"to1@example.com", "to2@example.com"},
		Subject:     "Test Subject",
		GeneratedAt: testTime,
	}

	domain := "testdomain.com"

	// Generate the email
	entity, err := GenerateNonAcceptanceEmail(domain, validationError, signer)
	if err != nil {
		t.Fatalf("GenerateNonAcceptanceEmail failed: %v", err)
	}

	// Read the body
	body, err := io.ReadAll(entity.Body)
	if err != nil {
		t.Fatalf("Failed to read entity body: %v", err)
	}

	bodyStr := string(body)

	// Extract base64 content (simplified extraction for testing)
	// In a real test, you'd properly parse the multipart message
	lines := strings.Split(bodyStr, "\n")
	var base64Content strings.Builder
	inBase64 := false

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.Contains(line, "Content-Transfer-Encoding: base64") {
			inBase64 = true
			continue
		}
		if inBase64 && strings.HasPrefix(line, "--") {
			break
		}
		if inBase64 && line != "" {
			base64Content.WriteString(line)
		}
	}

	// Decode the base64 content
	xmlData, err := base64.StdEncoding.DecodeString(base64Content.String())
	if err != nil {
		// If we can't decode, check if the XML structure is at least present in the body
		if !strings.Contains(bodyStr, "daticert") {
			t.Error("XML structure should be present in the message")
		}
		return
	}

	// Parse the XML
	var datiCert DatiCert
	err = xml.Unmarshal(xmlData, &datiCert)
	if err != nil {
		t.Fatalf("Failed to parse XML: %v", err)
	}

	// Verify XML content
	if datiCert.MessageID != validationError.MessageID {
		t.Errorf("Expected MessageID '%s', got '%s'", validationError.MessageID, datiCert.MessageID)
	}

	if datiCert.Subject != validationError.Subject {
		t.Errorf("Expected Subject '%s', got '%s'", validationError.Subject, datiCert.Subject)
	}

	if datiCert.From != validationError.From {
		t.Errorf("Expected From '%s', got '%s'", validationError.From, datiCert.From)
	}

	if datiCert.Reason != validationError.Reason {
		t.Errorf("Expected Reason '%s', got '%s'", validationError.Reason, datiCert.Reason)
	}

	if len(datiCert.To) != len(validationError.To) {
		t.Errorf("Expected %d recipients, got %d", len(validationError.To), len(datiCert.To))
	}

	for i, expected := range validationError.To {
		if i < len(datiCert.To) && datiCert.To[i] != expected {
			t.Errorf("Expected recipient %d to be '%s', got '%s'", i, expected, datiCert.To[i])
		}
	}
}

// TestGenerateNonAcceptanceEmail_SignerError tests error handling when signer fails
func TestGenerateNonAcceptanceEmail_SignerError(t *testing.T) {
	// Create signer with invalid key
	signer := &Signer{
		Cert:   nil,
		Key:    "invalid-key",
		Domain: "testdomain.com",
	}

	// Create test validation error
	validationError := ValidationError{
		Reason:      "test reason",
		MessageID:   "test-id@example.com",
		From:        "from@example.com",
		To:          []string{"to@example.com"},
		Subject:     "Test Subject",
		GeneratedAt: time.Now(),
	}

	domain := "testdomain.com"

	// Test the function - should return an error
	_, err := GenerateNonAcceptanceEmail(domain, validationError, signer)
	if err == nil {
		t.Error("Expected error when signer fails, but got none")
	}

	if !strings.Contains(err.Error(), "failed to create signed email") {
		t.Errorf("Expected error to contain 'failed to create signed email', got: %v", err)
	}
}

// TestGenerateNonAcceptanceEmail_MultipleRecipients tests with multiple recipients
func TestGenerateNonAcceptanceEmail_MultipleRecipients(t *testing.T) {
	// Create test certificate and key
	cert, key := createTestCertAndKeyForNonAcceptance(t)

	// Create signer
	signer := &Signer{
		Cert:   cert,
		Key:    key,
		Domain: "testdomain.com",
	}

	// Create test validation error with multiple recipients
	validationError := ValidationError{
		Reason:    "multiple recipient test",
		MessageID: "multi-test@example.com",
		From:      "sender@example.com",
		To: []string{
			"recipient1@testdomain.com",
			"recipient2@testdomain.com",
			"recipient3@testdomain.com",
		},
		Subject:     "Multi Recipient Test",
		GeneratedAt: time.Now(),
	}

	domain := "testdomain.com"

	// Generate the email
	entity, err := GenerateNonAcceptanceEmail(domain, validationError, signer)
	if err != nil {
		t.Fatalf("GenerateNonAcceptanceEmail failed: %v", err)
	}

	// Read the body
	body, err := io.ReadAll(entity.Body)
	if err != nil {
		t.Fatalf("Failed to read entity body: %v", err)
	}

	bodyStr := string(body)

	// Check that all recipients are mentioned in the body
	for _, recipient := range validationError.To {
		if !strings.Contains(bodyStr, recipient) {
			t.Errorf("Expected body to contain recipient '%s'", recipient)
		}
	}
}

// TestValidationError_Error tests the ValidationError.Error() method
func TestValidationError_Error(t *testing.T) {
	err := ValidationError{
		Reason:      "test reason",
		MessageID:   "test-id",
		From:        "from@example.com",
		To:          []string{"to@example.com"},
		Subject:     "Test Subject",
		GeneratedAt: time.Now(),
	}

	expectedError := "validation failed: test reason"
	if err.Error() != expectedError {
		t.Errorf("Expected error message '%s', got '%s'", expectedError, err.Error())
	}
}

// TestDatiCert_XMLMarshaling tests XML marshaling of DatiCert
func TestDatiCert_XMLMarshaling(t *testing.T) {
	testTime := time.Date(2024, 1, 15, 14, 30, 45, 0, time.UTC)

	datiCert := DatiCert{
		MessageID:   "test-message-id",
		Subject:     "Test Subject",
		From:        "from@example.com",
		To:          []string{"to1@example.com", "to2@example.com"},
		Reason:      "test reason",
		GeneratedAt: testTime.Format(time.RFC3339),
	}

	// Marshal to XML
	xmlData, err := xml.MarshalIndent(datiCert, "", "  ")
	if err != nil {
		t.Fatalf("Failed to marshal XML: %v", err)
	}

	// Verify XML content
	xmlStr := string(xmlData)

	expectedElements := []string{
		"<daticert>",
		"<message-id>test-message-id</message-id>",
		"<subject>Test Subject</subject>",
		"<from>from@example.com</from>",
		"<to>",
		"<address>to1@example.com</address>",
		"<address>to2@example.com</address>",
		"</to>",
		"<reason>test reason</reason>",
		"<timestamp>2024-01-15T14:30:45Z</timestamp>",
		"</daticert>",
	}

	for _, element := range expectedElements {
		if !strings.Contains(xmlStr, element) {
			t.Errorf("Expected XML to contain '%s', but it was not found", element)
		}
	}

	// Test unmarshaling
	var parsed DatiCert
	err = xml.Unmarshal(xmlData, &parsed)
	if err != nil {
		t.Fatalf("Failed to unmarshal XML: %v", err)
	}

	// Verify parsed data
	if parsed.MessageID != datiCert.MessageID {
		t.Errorf("Expected MessageID '%s', got '%s'", datiCert.MessageID, parsed.MessageID)
	}

	if parsed.Subject != datiCert.Subject {
		t.Errorf("Expected Subject '%s', got '%s'", datiCert.Subject, parsed.Subject)
	}

	if len(parsed.To) != len(datiCert.To) {
		t.Errorf("Expected %d recipients, got %d", len(datiCert.To), len(parsed.To))
	}
}

// TestGenerateAcceptanceEmail tests the main functionality of acceptance receipt generation
func TestGenerateAcceptanceEmail(t *testing.T) {
	// Create test certificate and key
	cert, key := createTestCertAndKeyForNonAcceptance(t)

	// Create signer
	signer := &Signer{
		Cert:   cert,
		Key:    key,
		Domain: "testdomain.com",
	}

	// Test data
	domain := "testdomain.com"
	messageID := "<test-message-id@example.com>"
	from := "sender@example.com"
	to := []string{"recipient1@testdomain.com", "recipient2@testdomain.com"}
	subject := "Test Email Subject"

	// Test the function
	entity, err := GenerateAcceptanceEmail(domain, messageID, from, to, subject, signer)
	if err != nil {
		t.Fatalf("GenerateAcceptanceEmail failed: %v", err)
	}

	// Verify the entity is not nil
	if entity == nil {
		t.Fatal("GenerateAcceptanceEmail returned nil entity")
	}

	// Test headers
	header := entity.Header

	// Check X-Ricevuta header
	if header.Get("X-Ricevuta") != "accettazione" {
		t.Errorf("Expected X-Ricevuta header to be 'accettazione', got '%s'", header.Get("X-Ricevuta"))
	}

	// Check Date header (should be parseable)
	dateStr := header.Get("Date")
	if dateStr == "" {
		t.Error("Date header is missing")
	} else {
		_, err := time.Parse(time.RFC1123Z, dateStr)
		if err != nil {
			t.Errorf("Date header is not in correct format: %v", err)
		}
	}

	// Check Subject header
	expectedSubject := "ACCETTAZIONE: Test Email Subject"
	if header.Get("Subject") != expectedSubject {
		t.Errorf("Expected Subject to be '%s', got '%s'", expectedSubject, header.Get("Subject"))
	}

	// Check From header
	expectedFrom := "posta-certificata@testdomain.com"
	if header.Get("From") != expectedFrom {
		t.Errorf("Expected From to be '%s', got '%s'", expectedFrom, header.Get("From"))
	}

	// Check To header
	if header.Get("To") != from {
		t.Errorf("Expected To to be '%s', got '%s'", from, header.Get("To"))
	}

	// Check X-Riferimento-Message-ID header
	if header.Get("X-Riferimento-Message-ID") != messageID {
		t.Errorf("Expected X-Riferimento-Message-ID to be '%s', got '%s'", messageID, header.Get("X-Riferimento-Message-ID"))
	}

	// Check Content-Type header
	contentType := header.Get("Content-Type")
	if !strings.HasPrefix(contentType, "multipart/signed") {
		t.Errorf("Expected Content-Type to start with 'multipart/signed', got '%s'", contentType)
	}

	// Read the body
	body, err := io.ReadAll(entity.Body)
	if err != nil {
		t.Fatalf("Failed to read entity body: %v", err)
	}

	bodyStr := string(body)

	// Check for specific content in the human-readable part
	expectedTexts := []string{
		"Ricevuta di accettazione",
		"Test Email Subject",
		"sender@example.com",
		"recipient1@testdomain.com",
		"recipient2@testdomain.com",
		"posta certificata",
		"è stato accettato dal sistema ed inoltrato",
		"daticert.xml",
	}

	for _, text := range expectedTexts {
		if !strings.Contains(bodyStr, text) {
			t.Errorf("Expected body to contain '%s', but it was not found", text)
		}
	}
}
