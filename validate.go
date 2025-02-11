package main

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"net/mail"
	"strings"

	"go.mozilla.org/pkcs7"
)

// Function to validate the digital signature in smime.p7s
func validateSMIMESignature(emlData []byte) bool {
	// Parse the email
	msg, err := mail.ReadMessage(bytes.NewReader(emlData))
	if err != nil {
		fmt.Println("Error parsing email:", err)
		return false
	}

	// Get the content type
	contentType := msg.Header.Get("Content-Type")
	mediaType, params, err := mime.ParseMediaType(contentType)
	if err != nil {
		fmt.Println("Error parsing content type:", err)
		return false
	}

	if mediaType != "multipart/signed" {
		fmt.Println("Email is not a signed S/MIME message")
		return false
	}

	// Parse multipart content
	mr := multipart.NewReader(msg.Body, params["boundary"])
	var signedData, signatureData []byte

	for {
		part, err := mr.NextPart()
		if err == io.EOF {
			break
		}
		if err != nil {
			fmt.Println("Error reading multipart:", err)
			return false
		}

		partMediaType, _, _ := mime.ParseMediaType(part.Header.Get("Content-Type"))
		partData, _ := io.ReadAll(part)

		if partMediaType == "application/pkcs7-signature" || partMediaType == "application/x-pkcs7-signature" {
			signatureData = decodeBase64IfNeeded(partData)
		} else {
			signedData = normalizeLineEndings(partData)
		}
	}

	if signedData == nil || signatureData == nil {
		fmt.Println("Missing signed data or signature")
		return false
	}

	// Decode the PKCS7 signature
	p7, err := pkcs7.Parse(signatureData)
	if err != nil {
		fmt.Println("Error parsing PKCS7:", err)
		return false
	}

	// Verify the signature
	err = p7.Verify()
	if err != nil {
		fmt.Println("Signature verification failed:", err)
		return false
	}

	// Extract signer certificates
	for _, cert := range p7.Certificates {
		fmt.Printf("Signer: %s\n", cert.Subject.CommonName)
	}

	fmt.Println("S/MIME signature is valid!")
	return true
}

// decodeBase64IfNeeded checks if the signature is base64 encoded and decodes it
func decodeBase64IfNeeded(data []byte) []byte {
	encoded := strings.TrimSpace(string(data))

	decoded, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		fmt.Println("Error decoding base64:", err)
		return data
	}
	return decoded
}

func normalizeLineEndings(data []byte) []byte {
	return bytes.ReplaceAll(data, []byte("\n"), []byte("\r\n"))
}
