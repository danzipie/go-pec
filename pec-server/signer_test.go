package main

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"math/big"
	"strings"
	"testing"
	"time"

	"go.mozilla.org/pkcs7"
)

// createTestCertAndKey creates a test certificate and private key for testing
func createTestCertAndKey(t *testing.T) (*x509.Certificate, *rsa.PrivateKey) {
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

// TestSigner_SignEmail tests the SignEmail method
func TestSigner_SignEmail(t *testing.T) {
	// Create test certificate and key
	cert, key := createTestCertAndKey(t)

	// Create signer
	signer := &Signer{
		Cert:   cert,
		Key:    key,
		Domain: "example.com",
	}

	// Test data
	emailContent := []byte("Subject: Test Email\r\nTo: test@example.com\r\nFrom: sender@example.com\r\n\r\nThis is a test email.")

	// Test signing
	signedData, err := signer.SignEmail(emailContent)
	if err != nil {
		t.Fatalf("SignEmail failed: %v", err)
	}

	// Verify the signed data is not empty
	if len(signedData) == 0 {
		t.Error("SignEmail returned empty data")
	}

	// Verify the signed data can be parsed
	p7, err := pkcs7.Parse(signedData)
	if err != nil {
		t.Fatalf("Failed to parse signed data: %v", err)
	}

	// Verify the content
	if string(p7.Content) != string(emailContent) {
		t.Errorf("Content mismatch. Expected: %s, Got: %s", string(emailContent), string(p7.Content))
	}

	// Verify the signature
	err = p7.Verify()
	if err != nil {
		t.Errorf("Signature verification failed: %v", err)
	}
}

// TestSigner_SignEmail_InvalidKey tests SignEmail with invalid key
func TestSigner_SignEmail_InvalidKey(t *testing.T) {
	cert, _ := createTestCertAndKey(t)

	// Create signer with invalid key
	signer := &Signer{
		Cert:   cert,
		Key:    "invalid-key", // This should cause an error
		Domain: "example.com",
	}

	emailContent := []byte("Test email content")

	// Test signing with invalid key
	_, err := signer.SignEmail(emailContent)
	if err == nil {
		t.Error("Expected error for invalid key, but got none")
	}

	expectedError := "failed to add signer"
	if !strings.Contains(err.Error(), expectedError) {
		t.Errorf("Expected error to contain '%s', but got: %v", expectedError, err)
	}
}

// TestSigner_CreateSignedMimeMessage tests the CreateSignedMimeMessage method
func TestSigner_CreateSignedMimeMessage(t *testing.T) {
	// Create test certificate and key
	cert, key := createTestCertAndKey(t)

	// Create signer
	signer := &Signer{
		Cert:   cert,
		Key:    key,
		Domain: "example.com",
	}

	// Test data
	emailContent := []byte("Subject: Test Email\r\nTo: test@example.com\r\nFrom: sender@example.com\r\n\r\nThis is a test email.")

	// Test creating signed MIME message
	signedMessage, err := signer.CreateSignedMimeMessage(emailContent)
	if err != nil {
		t.Fatalf("CreateSignedMimeMessage failed: %v", err)
	}

	// Verify the message is not empty
	if len(signedMessage) == 0 {
		t.Error("CreateSignedMimeMessage returned empty message")
	}

	messageStr := string(signedMessage)

	// Verify MIME headers
	if !strings.Contains(messageStr, "MIME-Version: 1.0") {
		t.Error("Missing MIME-Version header")
	}

	if !strings.Contains(messageStr, "Content-Type: multipart/signed") {
		t.Error("Missing multipart/signed Content-Type")
	}

	if !strings.Contains(messageStr, "protocol=\"application/pkcs7-signature\"") {
		t.Error("Missing protocol specification")
	}

	if !strings.Contains(messageStr, "micalg=sha256") {
		t.Error("Missing micalg specification")
	}

	// Verify boundary is present
	if !strings.Contains(messageStr, "boundary=\"") {
		t.Error("Missing boundary specification")
	}

	// Verify original content is included
	if !strings.Contains(messageStr, "This is a test email.") {
		t.Error("Original email content not found in signed message")
	}

	// Verify signature attachment
	if !strings.Contains(messageStr, "Content-Type: application/pkcs7-signature") {
		t.Error("Missing signature Content-Type")
	}

	if !strings.Contains(messageStr, "Content-Transfer-Encoding: base64") {
		t.Error("Missing base64 encoding specification")
	}

	if !strings.Contains(messageStr, "filename=\"smime.p7s\"") {
		t.Error("Missing signature filename")
	}
}

// TestSigner_CreateSignedMimeMessage_InvalidKey tests CreateSignedMimeMessage with invalid key
func TestSigner_CreateSignedMimeMessage_InvalidKey(t *testing.T) {
	cert, _ := createTestCertAndKey(t)

	// Create signer with invalid key
	signer := &Signer{
		Cert:   cert,
		Key:    12345, // Invalid key type
		Domain: "example.com",
	}

	emailContent := []byte("Test email content")

	// Test creating signed MIME message with invalid key
	_, err := signer.CreateSignedMimeMessage(emailContent)
	if err == nil {
		t.Error("Expected error for invalid key, but got none")
	}

	if !strings.Contains(err.Error(), "failed to sign email") {
		t.Errorf("Expected error to contain 'failed to sign email', but got: %v", err)
	}
}

// TestFormatBase64 tests the formatBase64 helper function
func TestFormatBase64(t *testing.T) {
	testCases := []struct {
		input      string
		lineLength int
		expected   string
	}{
		{
			input:      "SGVsbG8gV29ybGQ=",
			lineLength: 10,
			expected:   "SGVsbG8gV2\r\n9ybGQ=",
		},
		{
			input:      "SGVsbG8=",
			lineLength: 20,
			expected:   "SGVsbG8=",
		},
		{
			input:      "",
			lineLength: 10,
			expected:   "",
		},
		{
			input:      "A",
			lineLength: 1,
			expected:   "A",
		},
	}

	for i, tc := range testCases {
		result := formatBase64(tc.input, tc.lineLength)
		if result != tc.expected {
			t.Errorf("Test case %d failed. Expected: %q, Got: %q", i+1, tc.expected, result)
		}
	}
}

// TestSigner_EdgeCases tests various edge cases
func TestSigner_EdgeCases(t *testing.T) {
	cert, key := createTestCertAndKey(t)

	signer := &Signer{
		Cert:   cert,
		Key:    key,
		Domain: "example.com",
	}

	// Test with empty email content
	emptyContent := []byte("")
	signedData, err := signer.SignEmail(emptyContent)
	if err != nil {
		t.Errorf("SignEmail failed with empty content: %v", err)
	}
	if len(signedData) == 0 {
		t.Error("SignEmail returned empty data for empty content")
	}

	// Test with content that doesn't end with CRLF
	contentWithoutCRLF := []byte("Test content without CRLF")
	signedMessage, err := signer.CreateSignedMimeMessage(contentWithoutCRLF)
	if err != nil {
		t.Errorf("CreateSignedMimeMessage failed with content without CRLF: %v", err)
	}
	if len(signedMessage) == 0 {
		t.Error("CreateSignedMimeMessage returned empty message")
	}

	// Verify that CRLF is added
	messageStr := string(signedMessage)
	if !strings.Contains(messageStr, string(contentWithoutCRLF)) {
		t.Error("Original content not found in signed message")
	}
}

// TestSignerStruct tests the Signer struct initialization
func TestSignerStruct(t *testing.T) {
	cert, key := createTestCertAndKey(t)

	signer := &Signer{
		Cert:   cert,
		Key:    key,
		Domain: "test.com",
	}

	// Verify struct fields
	if signer.Cert == nil {
		t.Error("Cert field is nil")
	}

	if signer.Key == nil {
		t.Error("Key field is nil")
	}

	if signer.Domain != "test.com" {
		t.Errorf("Expected domain 'test.com', got '%s'", signer.Domain)
	}

	// Verify certificate properties
	if signer.Cert.Subject.Organization[0] != "Test Company" {
		t.Errorf("Expected organization 'Test Company', got '%s'", signer.Cert.Subject.Organization[0])
	}

	if len(signer.Cert.EmailAddresses) == 0 || signer.Cert.EmailAddresses[0] != "test@example.com" {
		t.Error("Certificate email address not set correctly")
	}
}

// BenchmarkSignEmail benchmarks the SignEmail method
func BenchmarkSignEmail(b *testing.B) {
	cert, key := createTestCertAndKey(&testing.T{})
	signer := &Signer{
		Cert:   cert,
		Key:    key,
		Domain: "example.com",
	}

	emailContent := []byte("Subject: Benchmark Test\r\nTo: test@example.com\r\n\r\nBenchmark test content.")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := signer.SignEmail(emailContent)
		if err != nil {
			b.Fatalf("SignEmail failed: %v", err)
		}
	}
}

// BenchmarkCreateSignedMimeMessage benchmarks the CreateSignedMimeMessage method
func BenchmarkCreateSignedMimeMessage(b *testing.B) {
	cert, key := createTestCertAndKey(&testing.T{})
	signer := &Signer{
		Cert:   cert,
		Key:    key,
		Domain: "example.com",
	}

	emailContent := []byte("Subject: Benchmark Test\r\nTo: test@example.com\r\n\r\nBenchmark test content.")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := signer.CreateSignedMimeMessage(emailContent)
		if err != nil {
			b.Fatalf("CreateSignedMimeMessage failed: %v", err)
		}
	}
}
