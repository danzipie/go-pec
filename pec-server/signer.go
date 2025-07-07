package main

import (
	"crypto"
	"crypto/x509"
	"encoding/base64"
	"fmt"
	"strings"
	"time"

	"go.mozilla.org/pkcs7"
)

// Use your existing Signer struct
type Signer struct {
	Cert   *x509.Certificate
	Key    interface{}
	Domain string
}

// S/MIME signing using go.mozilla.org/pkcs7
func (s *Signer) SignEmail(emailContent []byte) ([]byte, error) {
	// Create PKCS7 signed data
	signedData, err := pkcs7.NewSignedData(emailContent)
	if err != nil {
		return nil, fmt.Errorf("failed to create signed data: %v", err)
	}

	// Convert interface{} to crypto.PrivateKey
	privateKey, ok := s.Key.(crypto.PrivateKey)
	if !ok {
		return nil, fmt.Errorf("key is not a crypto.PrivateKey")
	}

	// Add signer
	err = signedData.AddSigner(s.Cert, privateKey, pkcs7.SignerInfoConfig{})
	if err != nil {
		return nil, fmt.Errorf("failed to add signer: %v", err)
	}

	// Finish the signature
	signedBytes, err := signedData.Finish()
	if err != nil {
		return nil, fmt.Errorf("failed to finish signature: %v", err)
	}

	return signedBytes, nil
}

// Create a complete S/MIME signed email message from email bytes
func (s *Signer) CreateSignedMimeMessage(emailContent []byte) ([]byte, error) {
	// Sign the email content
	signedData, err := s.SignEmail(emailContent)
	if err != nil {
		return nil, fmt.Errorf("failed to sign email: %v", err)
	}

	// Encode the signed data as base64
	signedDataB64 := base64.StdEncoding.EncodeToString(signedData)

	// Create the S/MIME message boundary
	boundary := fmt.Sprintf("----=_NextPart_%d", time.Now().Unix())

	// Build the S/MIME multipart/signed message
	var result strings.Builder

	// Write MIME headers for the signed message
	result.WriteString("MIME-Version: 1.0\r\n")
	result.WriteString(fmt.Sprintf("Content-Type: multipart/signed; protocol=\"application/pkcs7-signature\"; micalg=sha256; boundary=\"%s\"\r\n", boundary))
	result.WriteString("\r\n")
	result.WriteString("This is a multi-part message in MIME format.\r\n")
	result.WriteString("\r\n")

	// Write the original email content
	result.WriteString(fmt.Sprintf("--%s\r\n", boundary))
	result.WriteString(string(emailContent))
	if !strings.HasSuffix(string(emailContent), "\r\n") {
		result.WriteString("\r\n")
	}
	result.WriteString("\r\n")

	// Write the signature part
	result.WriteString(fmt.Sprintf("--%s\r\n", boundary))
	result.WriteString("Content-Type: application/pkcs7-signature; name=\"smime.p7s\"\r\n")
	result.WriteString("Content-Transfer-Encoding: base64\r\n")
	result.WriteString("Content-Disposition: attachment; filename=\"smime.p7s\"\r\n")
	result.WriteString("\r\n")
	result.WriteString(formatBase64(signedDataB64, 76))
	result.WriteString("\r\n")
	result.WriteString(fmt.Sprintf("--%s--\r\n", boundary))

	return []byte(result.String()), nil
}

// formatBase64 formats base64 string with line breaks
func formatBase64(data string, lineLength int) string {
	var result strings.Builder
	for i := 0; i < len(data); i += lineLength {
		end := i + lineLength
		if end > len(data) {
			end = len(data)
		}
		result.WriteString(data[i:end])
		if end < len(data) {
			result.WriteString("\r\n")
		}
	}
	return result.String()
}
