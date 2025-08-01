package main

import (
	"bytes"
	"crypto/sha1"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"encoding/xml"
	"fmt"
	"net/http"
	"net/smtp"
	"strings"
	"time"

	"github.com/danzipie/go-pec/pec-server/internal/common"
	"github.com/danzipie/go-pec/pec-server/store"
	"github.com/emersion/go-message"
	"github.com/emersion/go-message/mail"
	"go.mozilla.org/pkcs7"
)

// PuntoRicezioneServer represents a complete Punto ricezione server instance
type PuntoRicezioneServer struct {
	config      *common.Config
	store       store.MessageStore
	signer      *common.Signer
	smtpAddress string
	imapAddress string
	certificate *x509.Certificate
	privateKey  interface{}
}

// NewPuntoRicezioneServer creates a new PEC punto Ricezione server instance
func NewPuntoRicezioneServer(configPath string) (*PuntoRicezioneServer, error) {
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

	return &PuntoRicezioneServer{
		config:      cfg,
		store:       messageStore,
		signer:      signer,
		smtpAddress: cfg.SMTPServer,
		imapAddress: cfg.IMAPServer,
		certificate: cert,
		privateKey:  key,
	}, nil
}

// Assume you have a provider index like this:
var providerCertificateHashes = map[string]struct{}{
	// "SHA1_HEX_HASH": {},
	// e.g. "AABBCCDDEEFF...": {},
}

// IsValidTransportEnvelope checks if the message is a valid, signed PEC transport envelope.
func IsValidTransportEnvelope(header *mail.Header, body []byte) bool {
	// 1. Check for S/MIME signature structure (Content-Type: application/pkcs7-mime or smime.p7m)
	contentType := ""
	if ct := header.Get("Content-Type"); ct != "" {
		contentType = ct
	}
	if !strings.Contains(contentType, "application/pkcs7-mime") && !strings.Contains(contentType, "smime.p7m") {
		return false // Not an S/MIME signed message
	}

	// 2. Parse PKCS7 structure and extract certificates
	p7, err := pkcs7.Parse(body)
	if err != nil {
		return false // Not a valid PKCS7 structure
	}
	if len(p7.Certificates) == 0 {
		return false // No signing certificate found
	}

	// 3. Check if the signing certificate is from a certified provider
	signerCert := p7.GetOnlySigner()
	if signerCert == nil {
		return false
	}
	sha1sum := sha1.Sum(signerCert.Raw)
	sha1hex := strings.ToUpper(hex.EncodeToString(sha1sum[:]))
	if _, ok := providerCertificateHashes[sha1hex]; !ok {
		return false // Not a certified provider
	}

	// 4. Verify the S/MIME signature (including CRL and validity)
	roots := x509.NewCertPool()
	// Add trusted CA certs to roots as needed
	opts := x509.VerifyOptions{
		Roots: roots,
		// Add CRL checking and time validity as needed
	}
	if _, err := signerCert.Verify(opts); err != nil {
		return false // Certificate not valid
	}
	if err := p7.Verify(); err != nil {
		return false // Signature not valid
	}

	// 5. Formal correctness (basic check: must have From, To, Date, etc.)
	if _, err := header.AddressList("From"); err != nil {
		return false
	}
	if _, err := header.AddressList("To"); err != nil {
		return false
	}
	if _, err := header.Date(); err != nil {
		return false
	}

	return true
}

func ReceptionPointHandler(s *common.Session) error {
	// 1. Parse and verify the incoming message
	header, body, err := common.ParseEmailFromSession(*s)
	if err != nil {
		return fmt.Errorf("failed to parse incoming message: %w", err)
	}

	// 2. Check if the message is a valid transport envelope (busta di trasporto)
	if IsValidTransportEnvelope(header, body) {
		// a. Emit a "presa in carico" receipt to the sender's provider
		if err := EmitPresaInCaricoReceipt(s); err != nil {
			return fmt.Errorf("failed to emit presa in carico: %w", err)
		}
		// b. Forward the envelope to the delivery point (punto di consegna)
		if err := ForwardToDeliveryPoint(s); err != nil {
			return fmt.Errorf("failed to forward to delivery point: %w", err)
		}
		return nil
	} else if IsValidReceiptOrAvviso(header, body) {
		// 3. If it's a valid receipt or avviso
		// Forward to delivery point
		if err := ForwardToDeliveryPoint(s); err != nil {
			return fmt.Errorf("failed to forward receipt/avviso: %w", err)
		}
		return nil
	} else if IsFromCertifiedProvider(header) && common.IsSignatureValid(header, body) {
		// 4. If not a valid envelope/receipt/avviso, but from a certified provider (firma OK)
		// a. Wrap in "busta di anomalia"
		anomalyEnvelope, err := CreateAnomalyEnvelope(s)
		if err != nil {
			return fmt.Errorf("failed to create anomaly envelope: %w", err)
		}
		// b. Forward anomaly envelope to delivery point
		if err := ForwardEnvelopeToDeliveryPoint(anomalyEnvelope); err != nil {
			return fmt.Errorf("failed to forward anomaly envelope: %w", err)
		}
		return nil
	} else {
		// 5. If not from a certified provider (firma NOT OK)
		// a. Wrap in "busta di anomalia"
		anomalyEnvelope, err := CreateAnomalyEnvelope(s)
		if err != nil {
			return fmt.Errorf("failed to create anomaly envelope: %w", err)
		}
		// b. Forward anomaly envelope to delivery point
		if err := ForwardEnvelopeToDeliveryPoint(anomalyEnvelope); err != nil {
			return fmt.Errorf("failed to forward anomaly envelope: %w", err)
		}
	}

	return nil
}

type CertData struct {
	XMLName      xml.Name `xml:"certificazione"`
	Data         string   `xml:"data"`
	Ora          string   `xml:"ora"`
	Oggetto      string   `xml:"oggetto"`
	Mittente     string   `xml:"mittente"`
	Destinatario string   `xml:"destinatario"`
	MsgID        string   `xml:"identificativo"`
}

// EmitPresaInCaricoReceipt creates and sends a "presa in carico" receipt for a valid transport envelope.
func EmitPresaInCaricoReceipt(s *common.Session) error {
	// Parse the original message
	header, _, err := common.ParseEmailFromSession(*s)
	if err != nil {
		return fmt.Errorf("failed to parse original message: %w", err)
	}

	// Extract original headers
	origSubject, _ := header.Subject()
	origFrom, _ := header.AddressList("From")
	origTo, _ := header.AddressList("To")
	origMsgID := header.Get("Message-ID")

	// Compose receipt headers
	now := time.Now()
	receiptHeader := mail.Header{}
	receiptHeader.SetSubject("PRESA IN CARICO: " + origSubject)
	receiptHeader.SetAddressList("From", []*mail.Address{{Address: "posta-certificata@" + s.Domain}})
	// Lookup provider receipt address (implement this lookup as needed)
	receiptTo := LookupProviderReceiptAddress(origFrom)
	receiptHeader.SetAddressList("To", []*mail.Address{{Address: receiptTo}})
	receiptHeader.Set("X-Ricevuta", "presa-in-carico")
	receiptHeader.Set("Date", now.Format(time.RFC1123Z))
	receiptHeader.Set("X-Riferimento-Message-ID", origMsgID)

	// Compose receipt body
	var toList string
	for _, addr := range origTo {
		toList += addr.Address + "\n"
	}
	textBody := fmt.Sprintf(
		`Ricevuta di presa in carico
Il giorno %s alle ore %s (%s) il messaggio
"%s" proveniente da "%s"
ed indirizzato a:
%s
è stato accettato dal sistema.
Identificativo messaggio: %s
`, now.Format("02/01/2006"), now.Format("15:04:05"), now.Format("MST"), origSubject, origFrom[0].Address, toList, origMsgID)

	textHeader := message.Header{}
	textHeader.Set("Content-Type", "text/plain; charset=utf-8")
	textPart, err := message.New(textHeader, strings.NewReader(textBody))
	if err != nil {
		return fmt.Errorf("failed to create text part: %v", err)
	}
	// Compose XML certification data
	certData := CertData{
		Data:         now.Format("02/01/2006"),
		Ora:          now.Format("15:04:05"),
		Oggetto:      origSubject,
		Mittente:     origFrom[0].Address,
		Destinatario: toList,
		MsgID:        origMsgID,
	}
	xmlBuf, _ := xml.MarshalIndent(certData, "", "  ")
	var xmlB64 bytes.Buffer
	b64Encoder := base64.NewEncoder(base64.StdEncoding, &xmlB64)
	b64Encoder.Write(xmlBuf)
	b64Encoder.Close()

	xmlHeader := message.Header{}
	xmlHeader.Set("Content-Type", "application/xml")
	xmlHeader.Set("Content-Disposition", "attachment; filename=\"daticert.xml\"")
	xmlHeader.Set("Content-Transfer-Encoding", "base64")
	xmlPart, err := message.New(xmlHeader, bytes.NewReader(xmlB64.Bytes()))
	if err != nil {
		return fmt.Errorf("failed to create xml part: %v", err)
	}

	// Create multipart/mixed entity (alternative + xml)
	mixedHeader := message.Header{}
	mixedHeader.Set("Content-Type", "multipart/mixed")
	mixedHeader.Set("Content-Transfer-Encoding", "binary")
	mixedEntity, err := message.NewMultipart(mixedHeader, []*message.Entity{textPart, xmlPart})
	if err != nil {
		return fmt.Errorf("failed to create multipart/mixed entity: %v", err)
	}

	// Write the multipart/mixed entity to a buffer
	var body bytes.Buffer
	err = mixedEntity.WriteTo(&body)
	if err != nil {
		return fmt.Errorf("failed to write multipart/mixed entity: %v", err)
	}

	// Store or send the receipt (implement as needed)
	return ForwardEnvelopeToDeliveryPoint(body.Bytes())
}

// Helper: Lookup provider receipt address (stub)
func LookupProviderReceiptAddress(from []*mail.Address) string {
	// Implement lookup logic based on your provider index
	return "ricevute@provider.it"
}

func IsValidReceiptOrAvviso(header *mail.Header, body []byte) bool {
	xRicevuta := header.Get("X-Ricevuta")
	switch xRicevuta {
	case "avvenuta-consegna":
		// Delivery receipt
		// Check required headers
		if header.Get("Date") == "" ||
			header.Get("Subject") == "" ||
			header.Get("From") == "" ||
			header.Get("To") == "" ||
			header.Get("X-Riferimento-Message-ID") == "" {
			return false
		}
		// Optionally check X-TipoRicevuta
		tipo := header.Get("X-TipoRicevuta")
		if tipo != "" && tipo != "breve" && tipo != "sintetica" {
			return false
		}
		return true

	case "errore-consegna":
		// Non-delivery notice
		if header.Get("Date") == "" ||
			header.Get("Subject") == "" ||
			header.Get("From") == "" ||
			header.Get("To") == "" ||
			header.Get("X-Riferimento-Message-ID") == "" {
			return false
		}
		// X-TipoRicevuta is not required for errore-consegna
		return true
	}

	return false
}

func IsFromCertifiedProvider(header *mail.Header) bool {

	// TODO: For now, just return true (stub)
	return true
}

func ForwardToDeliveryPoint(s *common.Session) error {
	// Assume s.data contains the raw email
	data, err := s.GetData()
	if err != nil {
		return fmt.Errorf("failed to get session data: %v", err)
	}
	if data == nil {
		return fmt.Errorf("no data to forward")
	}
	req, err := http.NewRequest("POST", "http://delivery-point/api/receive", bytes.NewReader(data))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "message/rfc822")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("delivery point returned status %d", resp.StatusCode)
	}
	return nil
}

// CreateAnomalyEnvelope creates a "busta di anomalia" RFC 2822 message with the original message attached.
func CreateAnomalyEnvelope(s *common.Session) ([]byte, error) {
	// Parse the original message
	header, _, err := common.ParseEmailFromSession(*s)
	if err != nil {
		return nil, fmt.Errorf("failed to parse original message: %w", err)
	}

	// Extract required headers from the original message
	receivedHeaders := header.FieldsByKey("Received")
	toHeader := header.Get("To")
	ccHeader := header.Get("Cc")
	returnPath := header.Get("Return-Path")
	messageID := header.Get("Message-ID")
	origSubject, _ := header.Subject()
	origFrom, _ := header.AddressList("From")
	origTo, _ := header.AddressList("To")

	// Compose anomaly envelope headers
	now := time.Now()
	anomalyHeader := mail.Header{}
	anomalyHeader.Set("X-Trasporto", "errore")
	anomalyHeader.Set("Date", now.Format(time.RFC1123Z))
	anomalyHeader.SetSubject("ANOMALIA MESSAGGIO: " + origSubject)

	// From: "Per conto di: [mittente originale]" <posta-certificata@[dominio_di_posta]>
	fromDisplay := fmt.Sprintf("Per conto di: %s", origFrom[0].Address)
	anomalyHeader.SetAddressList("From", []*mail.Address{
		{Name: fromDisplay, Address: "posta-certificata@" + s.Domain},
	})

	// Reply-To: [mittente originale] (insert only if absent)
	if header.Get("Reply-To") == "" {
		anomalyHeader.SetAddressList("Reply-To", []*mail.Address{origFrom[0]})
	}

	// Copy original headers
	for receivedHeaders.Next() != false {
		anomalyHeader.Set("Received", receivedHeaders.Value())
	}
	if toHeader != "" {
		anomalyHeader.Set("To", toHeader)
	}
	if ccHeader != "" {
		anomalyHeader.Set("Cc", ccHeader)
	}
	if returnPath != "" {
		anomalyHeader.Set("Return-Path", returnPath)
	}
	if messageID != "" {
		anomalyHeader.Set("Message-ID", messageID)
	}

	// Compose anomaly body text
	var toList string
	for _, addr := range origTo {
		toList += addr.Address + "\n"
	}
	bodyText := fmt.Sprintf(
		`Anomalia nel messaggio
Il giorno %s alle ore %s (%s) è stato ricevuto
il messaggio "%s" proveniente da "%s"
ed indirizzato a:
%s
Tali dati non sono stati certificati per il seguente errore:
%s
Il messaggio originale è incluso in allegato.
`, now.Format("02/01/2006"), now.Format("15:04:05"), now.Format("MST"),
		origSubject, origFrom[0].Address, toList, "Errore di validazione PEC")

	// Create the text part
	textHeader := message.Header{}
	textHeader.Set("Content-Type", "text/plain; charset=utf-8")
	textPart, err := message.New(textHeader, strings.NewReader(bodyText))
	if err != nil {
		return nil, fmt.Errorf("failed to create text part: %v", err)
	}

	// Attach the original message as RFC 822 attachment
	data, err := s.GetData()
	if err != nil {
		return nil, fmt.Errorf("failed to get session data: %v", err)
	}
	attachmentHeader := message.Header{}
	attachmentHeader.Set("Content-Type", "message/rfc822")
	attachmentHeader.Set("Content-Disposition", "attachment; filename=\"original.eml\"")
	attachmentPart, err := message.New(attachmentHeader, bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("failed to create attachment part: %v", err)
	}

	// Create multipart/mixed entity
	mixedHeader := message.Header{}
	mixedHeader.Set("Content-Type", "multipart/mixed")
	mixedHeader.Set("Content-Transfer-Encoding", "binary")
	mixedEntity, err := message.NewMultipart(mixedHeader, []*message.Entity{textPart, attachmentPart})
	if err != nil {
		return nil, fmt.Errorf("failed to create multipart/mixed entity: %v", err)
	}

	// Write the multipart/mixed entity to a buffer
	var body bytes.Buffer
	err = mixedEntity.WriteTo(&body)
	if err != nil {
		return nil, fmt.Errorf("failed to write multipart/mixed entity: %v", err)
	}

	return body.Bytes(), nil
}

// ForwardEnvelopeToDeliveryPoint sends the envelope directly to the Punto di Ricezione of another authority via SMTP using emersion/go-smtp.
func ForwardEnvelopeToDeliveryPoint(envelope []byte) error {
	// SMTP server details for the other authority
	smtpAddr := "smtp.other-authority.it:25" // Change as needed
	sender := "posta-certificata@yourdomain.it"
	recipient := "ricezione@other-authority.it" // Change as needed

	// Create a new SMTP client
	c, err := smtp.Dial(smtpAddr)
	if err != nil {
		return fmt.Errorf("failed to connect to SMTP server: %v", err)
	}
	defer c.Close()

	// Set the sender and recipient
	if err := c.Mail(sender); err != nil {
		return fmt.Errorf("failed to set sender: %v", err)
	}
	if err := c.Rcpt(recipient); err != nil {
		return fmt.Errorf("failed to set recipient: %v", err)
	}

	// Send the envelope data
	wc, err := c.Data()
	if err != nil {
		return fmt.Errorf("failed to start DATA: %v", err)
	}
	if _, err := wc.Write(envelope); err != nil {
		wc.Close()
		return fmt.Errorf("failed to write envelope data: %v", err)
	}
	if err := wc.Close(); err != nil {
		return fmt.Errorf("failed to close DATA: %v", err)
	}

	return nil
}
