package main

import (
	"bytes"
	"encoding/base64"
	"encoding/xml"
	"fmt"
	"strings"
	"time"

	"github.com/emersion/go-message"
	"github.com/emersion/go-message/mail"
)

// ValidationError represents a failed validation with a clear reason.
type ValidationError struct {
	Reason      string
	MessageID   string
	From        string
	To          []string
	Subject     string
	GeneratedAt time.Time
}

func (e ValidationError) Error() string {
	return fmt.Sprintf("validation failed: %s", e.Reason)
}

// ValidateEnvelopeAndHeaders checks compliance between SMTP envelope and RFC822 headers.
func ValidateEnvelopeAndHeaders(
	smtpFrom string,
	smtpRecipients []string,
	msg *mail.Reader,
) error {
	// 1. Parse From header
	header := msg.Header
	fromAddrs, err := header.AddressList("From")
	if err != nil || len(fromAddrs) != 1 {
		return ValidationError{Reason: "invalid or missing 'From' field"}
	}
	fromHeader := fromAddrs[0].Address

	// 2. Parse To header
	toAddrs, err := header.AddressList("To")
	if err != nil || len(toAddrs) == 0 {
		return ValidationError{Reason: "missing or invalid 'To' field"}
	}

	// 3. Parse Cc header (optional)
	ccAddrs := []*mail.Address{}
	if ccList, err := header.AddressList("Cc"); err == nil {
		ccAddrs = ccList
	}

	// 4. Check Bcc (must not be present)
	if _, err := header.AddressList("Bcc"); err == nil {
		return ValidationError{Reason: "'Bcc' field must not be present"}
	}

	// 5. Validate reverse-path == From
	if !strings.EqualFold(smtpFrom, fromHeader) {
		return ValidationError{Reason: fmt.Sprintf("reverse-path '%s' does not match From header '%s'", smtpFrom, fromHeader)}
	}

	// 6. Collect all valid recipient addresses from To and Cc
	validRecipients := make(map[string]bool)
	for _, a := range toAddrs {
		validRecipients[strings.ToLower(a.Address)] = true
	}
	for _, a := range ccAddrs {
		validRecipients[strings.ToLower(a.Address)] = true
	}

	// 7. Validate all forward-path recipients are in To/Cc
	for _, rcpt := range smtpRecipients {
		if !validRecipients[strings.ToLower(rcpt)] {
			return ValidationError{Reason: fmt.Sprintf("recipient '%s' not found in 'To' or 'Cc' fields", rcpt)}
		}
	}

	return nil
}

// daticert.xml structure (simplified)
type DatiCert struct {
	XMLName     xml.Name `xml:"daticert"`
	MessageID   string   `xml:"message-id"`
	Subject     string   `xml:"subject"`
	From        string   `xml:"from"`
	To          []string `xml:"to>address"`
	Reason      string   `xml:"reason"`
	GeneratedAt string   `xml:"timestamp"`
}

// GenerateNonAcceptanceEmail creates an email message informing of non-acceptance with daticert.xml attached
func GenerateNonAcceptanceEmail(
	domain string,
	validationError ValidationError,
	signer *Signer,
) (*message.Entity, error) {

	// Part 1: human-readable explanation
	textBody := new(bytes.Buffer)
	fmt.Fprintf(textBody, "Errore nell’accettazione del messaggio\n")
	fmt.Fprintf(textBody, "Il giorno %s alle ore %s (%s) nel messaggio\n",
		validationError.GeneratedAt.Format("02/01/2006"),
		validationError.GeneratedAt.Format("15:04:05"),
		validationError.GeneratedAt.Format("MST"))
	fmt.Fprintf(textBody, "\"%s\" proveniente da \"%s\"\n", validationError.Subject, validationError.From)
	fmt.Fprintf(textBody, "ed indirizzato a:\n")
	for _, rcpt := range validationError.To {
		fmt.Fprintf(textBody, "%s\n", rcpt)
	}
	fmt.Fprintf(textBody, "è stato rilevato un problema che ne impedisce l’accettazione\na causa di %s.\nIl messaggio non è stato accettato.\n", validationError.Reason)
	fmt.Fprintf(textBody, "Identificativo messaggio: %s\n", validationError.MessageID)

	textHeader := message.Header{}
	textHeader.Set("Content-Type", "text/plain; charset=utf-8")
	textPart, err := message.New(textHeader, bytes.NewReader(textBody.Bytes()))
	if err != nil {
		return nil, fmt.Errorf("failed to create text part: %v", err)
	}

	// Part 2: daticert.xml attachment
	xmlData := DatiCert{
		MessageID:   validationError.MessageID,
		Subject:     validationError.Subject,
		From:        validationError.From,
		To:          validationError.To,
		Reason:      validationError.Reason,
		GeneratedAt: validationError.GeneratedAt.Format(time.RFC3339),
	}
	xmlBytes, _ := xml.MarshalIndent(xmlData, "", "  ")
	var xmlB64 bytes.Buffer
	b64Encoder := base64.NewEncoder(base64.StdEncoding, &xmlB64)
	b64Encoder.Write(xmlBytes)
	b64Encoder.Close()

	xmlHeader := message.Header{}
	xmlHeader.Set("Content-Type", "application/xml")
	xmlHeader.Set("Content-Disposition", "attachment; filename=\"daticert.xml\"")
	xmlHeader.Set("Content-Transfer-Encoding", "base64")
	xmlPart, err := message.New(xmlHeader, bytes.NewReader(xmlB64.Bytes()))
	if err != nil {
		return nil, fmt.Errorf("failed to create xml part: %v", err)
	}

	// Part 1b: human-readable explanation (HTML, reusing textBody)
	htmlBody := new(bytes.Buffer)
	fmt.Fprintf(htmlBody, "<html><body><pre>%s</pre></body></html>", textBody.String())

	htmlHeader := message.Header{}
	htmlHeader.Set("Content-Type", "text/html; charset=utf-8")
	htmlHeader.Set("Content-Disposition", "inline")
	htmlHeader.Set("Content-Transfer-Encoding", "quoted-printable")
	htmlPart, err := message.New(htmlHeader, bytes.NewReader(htmlBody.Bytes()))
	if err != nil {
		return nil, fmt.Errorf("failed to create html part: %v", err)
	}

	// Part 1c: multipart/alternative (text + html)
	altHeader := message.Header{}
	altHeader.Set("Content-Type", "multipart/alternative")
	altHeader.Set("Content-Transfer-Encoding", "binary")
	altEntity, err := message.NewMultipart(altHeader, []*message.Entity{textPart, htmlPart})
	if err != nil {
		return nil, fmt.Errorf("failed to create multipart/alternative entity: %v", err)
	}

	// Create multipart/mixed entity (alternative + xml)
	mixedHeader := message.Header{}
	mixedHeader.Set("Content-Type", "multipart/mixed")
	mixedHeader.Set("Content-Transfer-Encoding", "binary")
	mixedEntity, err := message.NewMultipart(mixedHeader, []*message.Entity{altEntity, xmlPart})
	if err != nil {
		return nil, fmt.Errorf("failed to create multipart/mixed entity: %v", err)
	}

	// Write the multipart/mixed entity to a buffer
	var body bytes.Buffer
	err = mixedEntity.WriteTo(&body)
	if err != nil {
		return nil, fmt.Errorf("failed to write multipart/mixed entity: %v", err)
	}

	// Part 3: S/MIME signature
	signedEmail, err := signer.CreateSignedMimeMessageEntity(body.Bytes())
	if err != nil {
		return nil, fmt.Errorf("failed to create signed email: %v", err)
	}

	// Create main headers
	signedEmail.Header.Set("X-Ricevuta", "non-accettazione")
	signedEmail.Header.Set("Date", validationError.GeneratedAt.Format(time.RFC1123Z))
	signedEmail.Header.Set("Subject", fmt.Sprintf("AVVISO DI NON ACCETTAZIONE: %s", validationError.Subject))
	signedEmail.Header.Set("From", fmt.Sprintf("posta-certificata@%s", domain))
	signedEmail.Header.Set("To", validationError.From)
	signedEmail.Header.Set("X-Riferimento-Message-ID", validationError.MessageID)

	return signedEmail, nil
}
