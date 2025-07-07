package main

import (
	"bytes"
	"encoding/base64"
	"encoding/xml"
	"fmt"
	"mime/multipart"
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
	// Create main headers
	headers := message.Header{}
	headers.Set("X-Ricevuta", "non-accettazione")
	headers.Set("Date", validationError.GeneratedAt.Format(time.RFC1123Z))
	headers.Set("Subject", fmt.Sprintf("AVVISO DI NON ACCETTAZIONE: %s", validationError.Subject))
	headers.Set("From", fmt.Sprintf("posta-certificata@%s", domain))
	headers.Set("To", validationError.From)
	headers.Set("X-Riferimento-Message-ID", validationError.MessageID)

	// Prepare multipart/mixed content
	var body bytes.Buffer
	w := multipart.NewWriter(&body)
	boundary := w.Boundary()
	headers.Set("Content-Type", fmt.Sprintf("multipart/mixed; boundary=%q", boundary))

	// Part 1: human-readable explanation
	text := new(bytes.Buffer)
	fmt.Fprintf(text, "Errore nell’accettazione del messaggio\n")
	fmt.Fprintf(text, "Il giorno %s alle ore %s (%s) nel messaggio\n",
		validationError.GeneratedAt.Format("02/01/2006"),
		validationError.GeneratedAt.Format("15:04:05"),
		validationError.GeneratedAt.Format("MST"))
	fmt.Fprintf(text, "\"%s\" proveniente da \"%s\"\n", validationError.Subject, validationError.From)
	fmt.Fprintf(text, "ed indirizzato a:\n")
	for _, rcpt := range validationError.To {
		fmt.Fprintf(text, "%s\n", rcpt)
	}
	fmt.Fprintf(text, "è stato rilevato un problema che ne impedisce l’accettazione\na causa di %s.\nIl messaggio non è stato accettato.\n", validationError.Reason)
	fmt.Fprintf(text, "Identificativo messaggio: %s\n", validationError.MessageID)

	textPart, _ := w.CreatePart(map[string][]string{
		"Content-Type": {"text/plain; charset=utf-8"},
	})
	textPart.Write(text.Bytes())

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
	xmlPart, _ := w.CreatePart(map[string][]string{
		"Content-Type":              {"application/xml"},
		"Content-Disposition":       {"attachment; filename=\"daticert.xml\""},
		"Content-Transfer-Encoding": {"base64"},
	})
	enc := base64.NewEncoder(base64.StdEncoding, xmlPart)
	enc.Write(xmlBytes)
	enc.Close()
	w.Close()

	// Part 3: S/MIME signature
	signedEmail, err := signer.CreateSignedMimeMessage(body.Bytes())
	if err != nil {
		return nil, fmt.Errorf("failed to create signed email: %v", err)
	}

	return message.New(headers, bytes.NewReader(signedEmail))
}
