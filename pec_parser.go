package main

import (
	"encoding/xml"
	"fmt"
	"io"
	"log"
	"strings"

	"github.com/emersion/go-message"
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
func extractPECHeaders(header *message.Header, pecMail *PECMail) {
	pecHeaders := []string{
		"X-Riferimento-Message-ID",
		"Return-Path",
		"Delivered-To",
		"Received",
		"X-Ricevuta",
		"Message-ID",
	}

	pecMail.PecType = None

	for _, h := range pecHeaders {
		if value := header.Get(h); value != "" {
			if h == "X-Ricevuta" {
				if strings.Contains(value, "accettazione") {
					pecMail.PecType = AcceptanceReceipt
				} else if strings.Contains(value, "consegna") {
					pecMail.PecType = DeliveryReceipt
				} else if strings.Contains(value, "posta certificata") {
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
func parseMixedPart(entity *message.Entity) *DatiCert {

	mr := entity.MultipartReader()
	if mr == nil {
		panic("Not a multipart message")
	}

	for {
		part, err := mr.NextPart()
		if err == io.EOF {
			break
		}
		if err != nil {
			break
		}

		mediaType, _, err := part.Header.ContentType()
		if err != nil {
			panic(err)
		}
		if strings.HasPrefix(mediaType, "multipart/alternative") {
			log.Println("multipart/alternative detected")
		} else if strings.HasPrefix(mediaType, "application/xml") {
			log.Println("application/xml detected")
			// print the body
			b, err := io.ReadAll(part.Body)
			if err != nil {
			}
			datiCert, err := parseDatiCertXML(string(b))
			if err != nil {
			}
			return datiCert

		} else if strings.HasPrefix(mediaType, "message/rfc822") {
			log.Println("message/rfc822 detected")
		} else {
			_, err := io.ReadAll(part.Body)
			if err != nil {
				panic(err)
			}
		}
	}
	return nil

}

// Function to parse the PEC email
// Extracts the envelope and the daticert.xml
func parsePec(entity *message.Entity) (*PECMail, *DatiCert, error) {

	pecMail := &PECMail{}
	datiCert := &DatiCert{}

	// Read headers
	header := entity.Header
	pecMail.Envelope.From = header.Get("From")
	pecMail.Envelope.To = header.Get("To")
	pecMail.Envelope.Subject = header.Get("Subject")
	pecMail.Envelope.Date = header.Get("Date")

	// Extract PEC-specific headers
	extractPECHeaders(&header, pecMail)
	if pecMail.PecType == None {
		return nil, nil, fmt.Errorf("not a pec")
	}

	mr := entity.MultipartReader()
	if mr == nil {
		panic("Not a multipart message")
	}

	part, err := mr.NextPart()
	if err != nil {
		panic(err)
	}

	mediaType, _, err := part.Header.ContentType()
	if err != nil {
		panic(err)
	}

	if strings.HasPrefix(mediaType, "multipart/mixed") {
		log.Println("multipart/mixed detected")
		datiCert = parseMixedPart(part)
	}

	// cross-check the extracted data
	if (pecMail.PecType == AcceptanceReceipt && datiCert.Tipo != "accettazione") ||
		(pecMail.PecType == DeliveryReceipt && datiCert.Tipo != "consegna") ||
		(pecMail.PecType == CertifiedEmail && datiCert.Tipo != "posta certificata") {
		return nil, nil, fmt.Errorf("mismatch between PEC type and DatiCert type")
	}

	return pecMail, datiCert, nil
}
