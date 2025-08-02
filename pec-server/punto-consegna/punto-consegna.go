package main

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"strings"
	"time"

	"github.com/danzipie/go-pec/pec-server/internal/common"
	"github.com/emersion/go-message"
	"github.com/emersion/go-message/mail"
	"github.com/emersion/go-sasl"
	"github.com/emersion/go-smtp"
)

// SMTPClient interface for sending outbound messages
type SMTPClient interface {
	SendMessage(from, to string, msg *message.Entity) error
}

// PuntoConsegnaSession implements smtp.Session for handling individual SMTP sessions
type PuntoConsegnaSession struct {
	server *PuntoConsegnaServer
	from   string
	to     []string
}

// Session implementation
func (s *PuntoConsegnaSession) Mail(from string, opts *smtp.MailOptions) error {
	s.from = from
	return nil
}

func (s *PuntoConsegnaSession) Rcpt(to string, opts *smtp.RcptOptions) error {
	s.to = append(s.to, to)
	return nil
}

func (s *PuntoConsegnaSession) Data(r io.Reader) error {
	// Parse the incoming message
	msg, err := message.Read(r)
	if err != nil {
		return fmt.Errorf("failed to parse message: %w", err)
	}

	// Process each recipient
	for _, recipient := range s.to {
		if err := s.processMessage(msg, recipient); err != nil {
			log.Printf("Error processing message for %s: %v", recipient, err)
			// Continue processing other recipients
		}
	}

	return nil
}

func (s *PuntoConsegnaSession) Reset() {
	s.from = ""
	s.to = nil
}

func (s *PuntoConsegnaSession) Logout() error {
	return nil
}

// processMessage handles the core PEC logic for a single recipient
func (s *PuntoConsegnaSession) processMessage(msg *message.Entity, recipient string) error {
	// Check if this is a transport envelope (busta di trasporto)
	isTransportEnvelope := s.isTransportEnvelope(msg)

	var deliveryErr error
	deliveryErr = nil
	if isTransportEnvelope {
		log.Printf("Processing transport envelope for recipient: %s", recipient)
		deliveryErr = s.server.DeliverMessage(recipient, msg)
	}

	if deliveryErr != nil {
		// Delivery failed - send non-delivery notice if it was a transport envelope
		if isTransportEnvelope {
			if err := s.sendNonDeliveryNotice(s.from, msg, recipient, deliveryErr); err != nil {
				log.Printf("Failed to send non-delivery notice: %v", err)
			}
		}
		return fmt.Errorf("delivery failed: %w", deliveryErr)
	}

	// Delivery succeeded - send delivery receipt if it was a transport envelope
	if isTransportEnvelope {
		if err := s.sendDeliveryReceipt(s.from, msg, recipient); err != nil {
			log.Printf("Failed to send delivery receipt: %v", err)
			// Don't return error - message was delivered successfully
		}
	}

	return nil
}

// isTransportEnvelope checks if the message is a PEC transport envelope
func (s *PuntoConsegnaSession) isTransportEnvelope(msg *message.Entity) bool {
	header := msg.Header
	xTrasporto := header.Get("X-Trasporto")
	return strings.ToLower(xTrasporto) == "posta-certificata"
}

func (s *PuntoConsegnaSession) SendEntity(receipt *message.Entity, to []string) error {
	var w io.Writer
	receipt.WriteTo(w)

	// Set up authentication information.
	auth := sasl.NewPlainClient("", "user@example.com", "password")

	// Connect to the server, authenticate, set the sender and recipient,
	// and send the email all in one step.
	msg := bytes.NewReader(w.(*bytes.Buffer).Bytes())
	return smtp.SendMail(fmt.Sprintf("postmaster@%s", s.server.domain), auth, "me", to, msg)
}

// sendDeliveryReceipt sends a "ricevuta di avvenuta consegna"
func (s *PuntoConsegnaSession) sendDeliveryReceipt(originalSender string, originalMsg *message.Entity, recipient string) error {
	log.Printf("Sending delivery receipt to %s for message delivered to %s", originalSender, recipient)

	// Create delivery receipt message
	receipt := s.createDeliveryReceipt(originalMsg, recipient)
	return s.SendEntity(receipt, []string{originalSender})
}

// sendNonDeliveryNotice sends an "avviso di mancata consegna"
func (s *PuntoConsegnaSession) sendNonDeliveryNotice(originalSender string, originalMsg *message.Entity, recipient string, deliveryErr error) error {
	log.Printf("Sending non-delivery notice to %s for failed delivery to %s: %v", originalSender, recipient, deliveryErr)

	// Create non-delivery notice
	notice := s.createNonDeliveryNotice(originalMsg, recipient, deliveryErr)

	return s.SendEntity(notice, []string{originalSender})
}

// ReceiptType represents the type of delivery receipt to generate
type ReceiptType int

const (
	ReceiptTypeNormal ReceiptType = iota
	ReceiptTypeShort
	ReceiptTypeSynthetic
)

// parseReceiptType determines the receipt type from X-TipoRicevuta header
func parseReceiptType(msg *message.Entity) ReceiptType {
	tipoRicevuta := strings.ToLower(strings.TrimSpace(msg.Header.Get("X-TipoRicevuta")))

	switch tipoRicevuta {
	case "breve":
		return ReceiptTypeShort
	case "sintetica":
		return ReceiptTypeSynthetic
	default:
		// If absent or unrecognized, default to normal
		return ReceiptTypeNormal
	}
}

// createDeliveryReceipt creates a delivery receipt message based on the requested type
func (s *PuntoConsegnaSession) createDeliveryReceipt(originalMsg *message.Entity, recipient string) *message.Entity {
	// Generate unique message ID
	msgID := common.GenerateMessageID(s.server.domain)
	timestamp := time.Now()

	// Determine receipt type from original message
	receiptType := parseReceiptType(originalMsg)

	// Get original subject
	originalSubject := originalMsg.Header.Get("Subject")
	if originalSubject == "" {
		originalSubject = "(nessun oggetto)"
	}

	// Create receipt header according to specifications
	header := message.Header{}
	header.Set("Message-ID", msgID)
	header.Set("X-Ricevuta", "avvenuta-consegna")
	header.Set("Date", timestamp.Format(time.RFC822))
	header.Set("Subject", fmt.Sprintf("CONSEGNA: %s", originalSubject))
	header.Set("From", fmt.Sprintf("posta-certificata@%s", s.server.domain))
	header.Set("To", originalMsg.Header.Get("From"))
	header.Set("X-Riferimento-Message-ID", originalMsg.Header.Get("Message-ID"))

	// Add receipt type indicator
	switch receiptType {
	case ReceiptTypeShort:
		header.Set("X-Tipo-Ricevuta", "breve")
	case ReceiptTypeSynthetic:
		header.Set("X-Tipo-Ricevuta", "sintetica")
	default:
		header.Set("X-Tipo-Ricevuta", "normale")
	}

	// Create receipt body based on type
	var body io.Reader
	switch receiptType {
	case ReceiptTypeNormal:
		body = s.createNormalReceiptBody(originalMsg, recipient, timestamp)
	case ReceiptTypeShort:
		body = s.createShortReceiptBody(originalMsg, recipient, timestamp)
	case ReceiptTypeSynthetic:
		body = s.createSyntheticReceiptBody(originalMsg, recipient, timestamp)
	}

	return &message.Entity{
		Header: header,
		Body:   body,
	}
}

// RecipientType indicates if the recipient is primary or CC
type RecipientType int

const (
	RecipientTypePrimary RecipientType = iota
	RecipientTypeCC
	RecipientTypeAmbiguous // When we can't determine with certainty
)

// determineRecipientType analyzes To/CC fields to determine recipient type
func determineRecipientType(originalMsg *message.Entity, recipient string) RecipientType {
	toField := originalMsg.Header.Get("To")
	ccField := originalMsg.Header.Get("Cc")

	// Parse addresses from To field
	toAddresses, err := mail.ParseAddressList(toField)
	if err == nil {
		for _, addr := range toAddresses {
			if strings.EqualFold(addr.Address, recipient) {
				return RecipientTypePrimary
			}
		}
	}

	// Parse addresses from CC field
	if ccField != "" {
		ccAddresses, err := mail.ParseAddressList(ccField)
		if err == nil {
			for _, addr := range ccAddresses {
				if strings.EqualFold(addr.Address, recipient) {
					return RecipientTypeCC
				}
			}
		}
	}

	// If we can't determine with certainty (ambiguous), treat as primary
	// This follows the cautious logic specified in the requirements
	return RecipientTypeAmbiguous
}

// createNormalReceiptBody creates the body for a normal delivery receipt
func (s *PuntoConsegnaSession) createNormalReceiptBody(originalMsg *message.Entity, recipient string, timestamp time.Time) io.Reader {
	// Determine recipient type to decide whether to include original message
	recipientType := determineRecipientType(originalMsg, recipient)
	includeOriginal := recipientType == RecipientTypePrimary || recipientType == RecipientTypeAmbiguous

	// Get original message details
	originalSender := originalMsg.Header.Get("From")
	originalSubject := originalMsg.Header.Get("Subject")
	originalMessageID := originalMsg.Header.Get("Message-ID")

	if originalSubject == "" {
		originalSubject = "(nessun oggetto)"
	}
	if originalMessageID == "" {
		originalMessageID = "(non disponibile)"
	}

	// Format timestamp according to Italian locale
	dateStr := timestamp.Format("02/01/2006")
	timeStr := timestamp.Format("15:04:05")
	zone := timestamp.Format("MST")

	// Create human-readable receipt text
	receiptText := fmt.Sprintf(`Ricevuta di avvenuta consegna
Il giorno %s alle ore %s (%s) il messaggio
"%s" proveniente da "%s"
ed indirizzato a "%s" è stato consegnato nella casella di destinazione.
Identificativo messaggio: %s`,
		dateStr, timeStr, zone,
		originalSubject,
		originalSender,
		recipient,
		originalMessageID)

	// Create multipart message using emersion/go-message
	var buf bytes.Buffer

	// Create multipart header
	header := message.Header{}
	header.Set("Content-Type", "multipart/mixed")

	// Create multipart writer
	mw, err := message.CreateWriter(&buf, header)
	if err != nil {
		log.Printf("Error creating multipart writer: %v", err)
		return strings.NewReader("Error creating receipt")
	}

	// Part 1: Human-readable text
	textHeader := message.Header{}
	textHeader.Set("Content-Type", "text/plain; charset=utf-8")
	textHeader.Set("Content-Transfer-Encoding", "8bit")

	textWriter, err := mw.CreatePart(textHeader)
	if err == nil {
		textWriter.Write([]byte(receiptText))
		textWriter.Close()
	}

	// Part 2: XML certification data
	xmlData := s.createCertificationXML(originalMsg, recipient, timestamp)
	xmlHeader := message.Header{}
	xmlHeader.Set("Content-Type", "application/xml")
	xmlHeader.Set("Content-Disposition", "attachment; filename=\"certificazione.xml\"")

	xmlWriter, err := mw.CreatePart(xmlHeader)
	if err == nil {
		xmlWriter.Write([]byte(xmlData))
		xmlWriter.Close()
	}

	// Part 3: Original message (only for primary recipients or ambiguous cases)
	if includeOriginal {
		originalHeader := message.Header{}
		originalHeader.Set("Content-Type", "message/rfc822")
		originalHeader.Set("Content-Disposition", "attachment; filename=\"messaggio-originale.eml\"")

		originalWriter, err := mw.CreatePart(originalHeader)
		if err == nil {

			// Write original message body
			if originalMsg.Body != nil {
				// Reset body reader if possible
				if seeker, ok := originalMsg.Body.(io.Seeker); ok {
					seeker.Seek(0, io.SeekStart)
				}
				io.Copy(originalWriter, originalMsg.Body)
			}
			originalWriter.Close()
		}
	}

	mw.Close()
	return &buf
}

// createCertificationXML creates the XML certification data
func (s *PuntoConsegnaSession) createCertificationXML(originalMsg *message.Entity, recipient string, timestamp time.Time) string {
	// Get original message details
	originalSender := originalMsg.Header.Get("From")
	originalSubject := originalMsg.Header.Get("Subject")
	originalMessageID := originalMsg.Header.Get("Message-ID")

	if originalSubject == "" {
		originalSubject = "(nessun oggetto)"
	}
	if originalMessageID == "" {
		originalMessageID = "(non disponibile)"
	}

	// Create XML with certification data
	// TODO: This is a basic structure - you may need to adjust according to official PEC XML schema
	xml := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<certificazione xmlns="http://www.cnipa.it/schemas/2003/eGovIT/Busta1_0/">
	<intestazione>
		<identificativo>%s</identificativo>
		<data-consegna>%s</data-consegna>
		<tipo-ricevuta>avvenuta-consegna</tipo-ricevuta>
	</intestazione>
	<dati-certificazione>
		<mittente>%s</mittente>
		<destinatario>%s</destinatario>
		<oggetto>%s</oggetto>
		<identificativo-messaggio>%s</identificativo-messaggio>
		<data-ora-consegna>%s</data-ora-consegna>
		<gestore-consegna>%s</gestore-consegna>
	</dati-certificazione>
</certificazione>`,
		common.GenerateMessageID(s.server.domain),
		timestamp.Format(time.RFC3339),
		originalSender,
		recipient,
		originalSubject,
		originalMessageID,
		timestamp.Format(time.RFC3339),
		s.server.domain)

	return xml
}

// createShortReceiptBody creates the body for a short delivery receipt
// TODO: Implement reduced body with essential information only
func (s *PuntoConsegnaSession) createShortReceiptBody(originalMsg *message.Entity, recipient string, timestamp time.Time) io.Reader {
	content := fmt.Sprintf(`Ricevuta di avvenuta consegna - BREVE

Destinatario: %s
Data consegna: %s
ID: %s

[TODO: Include essential XML certification data only]`,
		recipient,
		timestamp.Format("02/01/2006 15:04:05"),
		originalMsg.Header.Get("Message-ID"))

	return strings.NewReader(content)
}

// createSyntheticReceiptBody creates the body for a synthetic delivery receipt
// TODO: Implement minimal body for synthetic receipt
func (s *PuntoConsegnaSession) createSyntheticReceiptBody(originalMsg *message.Entity, recipient string, timestamp time.Time) io.Reader {
	content := fmt.Sprintf(`Ricevuta sintetica

Consegnato: %s - %s

[TODO: Include minimal certification data]`,
		recipient,
		timestamp.Format("02/01/2006 15:04:05"))

	return strings.NewReader(content)
}

// createNonDeliveryNotice creates a non-delivery notice message
func (s *PuntoConsegnaSession) createNonDeliveryNotice(originalMsg *message.Entity, recipient string, deliveryErr error) *message.Entity {
	// Generate unique message ID
	msgID := common.GenerateMessageID(s.server.domain)
	timestamp := time.Now()

	// Create notice header
	header := message.Header{}
	header.Set("Message-ID", msgID)
	header.Set("Date", timestamp.Format(time.RFC822))
	header.Set("From", fmt.Sprintf("postmaster@%s", s.server.domain))
	header.Set("To", originalMsg.Header.Get("From"))
	header.Set("Subject", "Avviso di mancata consegna")
	header.Set("X-Ricevuta", "mancata-consegna")
	header.Set("References", originalMsg.Header.Get("Message-ID"))

	// Create body with error details
	body := fmt.Sprintf("Delivery to %s failed: %s", recipient, deliveryErr.Error())

	return &message.Entity{
		Header: header,
		Body:   strings.NewReader(body),
	}
}
