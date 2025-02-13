package main

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"net/mail"
	"strings"
)

// Function to parse DatiCert XML
func parseDatiCertXML(content string) (*DatiCert, error) {
	// Remove any extra spaces or newlines that might exist around the XML content
	content = strings.TrimSpace(content)

	// Parse the XML string into the DatiCert struct
	var daticert DatiCert
	err := xml.Unmarshal([]byte(content), &daticert)
	if err != nil {
		return nil, fmt.Errorf("failed to parse daticert.xml: %v", err)
	}

	return &daticert, nil
}

// reads PEC-specific headers from the email
func extractPECHeaders(header *mail.Header, pecMail *PECMail) {
	pecHeaders := []string{
		"X-Riferimento-Message-ID",
		"Return-Path",
		"Delivered-To",
		"Received",
		"X-Ricevuta",
		"Message-ID",
		"X-Trasporto",
	}

	pecMail.PecType = None

	for _, h := range pecHeaders {
		if value := header.Get(h); value != "" {
			if h == "X-Ricevuta" {
				if strings.Contains(value, "accettazione") {
					pecMail.PecType = AcceptanceReceipt
				} else if strings.Contains(value, "avvenuta-consegna") {
					pecMail.PecType = DeliveryReceipt
				} else if strings.Contains(value, "errore-consegna") {
					pecMail.PecType = DeliveryErrorReceipt
				}
			} else if h == "X-Trasporto" {
				if strings.Contains(value, "posta-certificata") {
					pecMail.PecType = CertifiedEmail
				}
			}
			if h == "Message-ID" {
				pecMail.MessageID = value
			}
		}
	}
}

// Function to parse the mixed part of the email
// Should contain the daticert.xml
func parseMixedPart(partData []byte, boundary string) *DatiCert {

	reader := multipart.NewReader(bytes.NewReader(partData), boundary)

	for {
		part, err := reader.NextPart()
		if err == io.EOF {
			break
		}
		if err != nil {
			fmt.Println("Error reading multipart:", err)
			return nil
		}

		partMediaType, _, _ := mime.ParseMediaType(part.Header.Get("Content-Type"))
		partData, _ := io.ReadAll(part)

		if partMediaType == "multipart/alternative" {
			// log.Println("multipart/alternative detected")
		} else if partMediaType == "application/xml" {
			decoded := decodeBase64IfNeeded(partData)
			datiCert, err := parseDatiCertXML(string(decoded))
			if err != nil {
				fmt.Println("Error parsing daticert.xml:", err)
			}
			return datiCert

		} else if partMediaType == "message/rfc822" {
			// log.Println("message/rfc822 detected")
		} else {
			// log.Println("Unknown part type detected")
		}
	}

	return nil

}

// Function to parse the PEC email
// Extracts the envelope and the daticert.xml
func parsePec(msg *mail.Message) (*PECMail, *DatiCert, error) {

	pecMail := &PECMail{}
	datiCert := &DatiCert{}

	// Get the content type
	contentType := msg.Header.Get("Content-Type")
	mediaType, params, err := mime.ParseMediaType(contentType)
	if err != nil {
		fmt.Println("Error parsing content type:", err)
		return pecMail, datiCert, err
	}

	if mediaType != "multipart/signed" {
		fmt.Println("Email is not a signed S/MIME message")
		return pecMail, datiCert, err
	}

	// Read headers
	header := msg.Header
	pecMail.Envelope.From = header.Get("From")
	pecMail.Envelope.To = header.Get("To")
	pecMail.Envelope.Subject = header.Get("Subject")
	pecMail.Envelope.Date = header.Get("Date")

	// Extract PEC-specific headers
	extractPECHeaders(&header, pecMail)
	if pecMail.PecType == None {
		return nil, nil, fmt.Errorf("not a pec")
	}

	// Parse multipart content
	mr := multipart.NewReader(msg.Body, params["boundary"])

	for {
		part, err := mr.NextPart()
		if err == io.EOF {
			break
		}

		if err != nil {
			fmt.Println("Error reading multipart:", err)
			// TODO: check this suppressed error for malformed eml files
			return pecMail, datiCert, nil
		}

		partMediaType, params, _ := mime.ParseMediaType(part.Header.Get("Content-Type"))
		partData, _ := io.ReadAll(part)

		if partMediaType == "multipart/mixed" {
			datiCert = parseMixedPart(partData, params["boundary"])
			if datiCert == nil {
				return nil, nil, fmt.Errorf("failed to parse mixed part")
			}
		}
	}

	// cross-check the extracted data
	if (pecMail.PecType == AcceptanceReceipt && datiCert.Tipo != "accettazione") ||
		(pecMail.PecType == DeliveryReceipt && datiCert.Tipo != "avvenuta-consegna") ||
		(pecMail.PecType == CertifiedEmail && datiCert.Tipo != "posta-certificata") ||
		(pecMail.PecType == DeliveryErrorReceipt && datiCert.Tipo != "errore-consegna") {
		return nil, nil, fmt.Errorf("mismatch between PEC type and DatiCert type: %d vs %s", pecMail.PecType, datiCert.Tipo)
	}

	return pecMail, datiCert, nil
}

func parseAndVerify(msg *mail.Message) (*PECMail, *DatiCert, error) {
	pecMail, datiCert, err := parsePec(msg)
	if err != nil {
		return nil, nil, err
	}

	// convert msg to a byte array
	buf := new(bytes.Buffer)
	_, err = buf.ReadFrom(msg.Body)
	if err != nil {
		return nil, nil, err
	}

	// convert buf to a byte array
	emlData := buf.Bytes()

	valid := validateSMIMESignature(emlData)
	if !valid {
		return nil, nil, err
	}

	return pecMail, datiCert, nil
}
