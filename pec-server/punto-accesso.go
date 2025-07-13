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

	// 4. Check Bcc (must not be present with valid addresses)
	if bccList, err := header.AddressList("Bcc"); err == nil && len(bccList) > 0 {
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

// GenerateAcceptanceEmail creates an email message confirming acceptance with daticert.xml attached
func GenerateAcceptanceEmail(
	domain string,
	messageID string,
	from string,
	to []string,
	subject string,
	signer *Signer,
) (*message.Entity, error) {
	now := time.Now()

	// Part 1: human-readable explanation
	textBody := new(bytes.Buffer)
	fmt.Fprintf(textBody, "-- Ricevuta di accettazione del messaggio indirizzato a %s (\"posta certificata\") --\n\n", strings.Join(to, ", "))
	fmt.Fprintf(textBody, "Il giorno %s alle ore %s (%s) il messaggio con Oggetto\n",
		now.Format("02/01/2006"),
		now.Format("15:04:05"),
		now.Format("-0700"))
	fmt.Fprintf(textBody, "\"%s\" inviato da \"%s\"\n", subject, from)
	fmt.Fprintf(textBody, "ed indirizzato a:\n")
	for _, rcpt := range to {
		fmt.Fprintf(textBody, "%s (\"posta certificata\")\n", rcpt)
	}
	fmt.Fprintf(textBody, "è stato accettato dal sistema ed inoltrato.\n")
	generatedMessageID := fmt.Sprintf("opec%s.%s@%s",
		now.Format("210312"),
		now.Format("20060102150405.000000.000.1.53"),
		domain)
	fmt.Fprintf(textBody, "Identificativo del messaggio: %s\n", generatedMessageID)
	fmt.Fprintf(textBody, "L'allegato daticert.xml contiene informazioni di servizio sulla trasmissione\n")

	textHeader := message.Header{}
	textHeader.Set("Content-Type", "text/plain; charset=utf-8")
	textHeader.Set("Content-Disposition", "inline")
	textHeader.Set("Content-Transfer-Encoding", "quoted-printable")
	textPart, err := message.New(textHeader, bytes.NewReader(textBody.Bytes()))
	if err != nil {
		return nil, fmt.Errorf("failed to create text part: %v", err)
	}

	// Part 2: daticert.xml attachment
	type postaCert struct {
		XMLName      xml.Name `xml:"postacert"`
		Tipo         string   `xml:"tipo,attr"`
		Errore       string   `xml:"errore,attr"`
		Intestazione struct {
			Mittente    string `xml:"mittente"`
			Destinatari struct {
				Tipo string `xml:"tipo,attr"`
				Val  string `xml:",chardata"`
			} `xml:"destinatari"`
			Risposte string `xml:"risposte"`
			Oggetto  string `xml:"oggetto"`
		} `xml:"intestazione"`
		Dati struct {
			GestoreEmittente string `xml:"gestore-emittente"`
			Data             struct {
				Zona   string `xml:"zona,attr"`
				Giorno string `xml:"giorno"`
				Ora    string `xml:"ora"`
			} `xml:"data"`
			Identificativo string `xml:"identificativo"`
			MsgID          string `xml:"msgid"`
		} `xml:"dati"`
	}

	xmlData := postaCert{
		Tipo:   "accettazione",
		Errore: "nessuno",
	}
	xmlData.Intestazione.Mittente = from
	xmlData.Intestazione.Destinatari.Tipo = "certificato"
	xmlData.Intestazione.Destinatari.Val = strings.Join(to, ", ")
	xmlData.Intestazione.Risposte = from
	xmlData.Intestazione.Oggetto = subject
	xmlData.Dati.GestoreEmittente = fmt.Sprintf("%s PEC S.p.A.", strings.ToUpper(domain))
	xmlData.Dati.Data.Zona = now.Format("-0700")
	xmlData.Dati.Data.Giorno = now.Format("02/01/2006")
	xmlData.Dati.Data.Ora = now.Format("15:04:05")
	xmlData.Dati.Identificativo = generatedMessageID
	xmlData.Dati.MsgID = messageID

	xmlBytes, err := xml.MarshalIndent(xmlData, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal XML: %v", err)
	}

	// Add XML declaration
	xmlWithHeader := []byte(xml.Header + string(xmlBytes))

	var xmlB64 bytes.Buffer
	b64Encoder := base64.NewEncoder(base64.StdEncoding, &xmlB64)
	b64Encoder.Write(xmlWithHeader)
	b64Encoder.Close()

	xmlHeader := message.Header{}
	xmlHeader.Set("Content-Type", "application/xml; name=\"daticert.xml\"")
	xmlHeader.Set("Content-Disposition", "inline; filename=\"daticert.xml\"")
	xmlHeader.Set("Content-Transfer-Encoding", "base64")
	xmlPart, err := message.New(xmlHeader, bytes.NewReader(xmlB64.Bytes()))
	if err != nil {
		return nil, fmt.Errorf("failed to create xml part: %v", err)
	}

	// Part 1b: human-readable explanation (HTML)
	htmlBody := new(bytes.Buffer)
	fmt.Fprintf(htmlBody, "<html>\n<head><title>Ricevuta di accettazione</title></head>\n<body>\n")
	fmt.Fprintf(htmlBody, "<h3>Ricevuta di accettazione</h3>\n")
	fmt.Fprintf(htmlBody, "<hr><br>\n")
	fmt.Fprintf(htmlBody, "Il giorno %s alle ore %s (%s) il messaggio<br>\n",
		now.Format("02/01/2006"),
		now.Format("15:04:05"),
		now.Format("-0700"))
	fmt.Fprintf(htmlBody, "&quot;%s&quot; proveniente da &quot;%s&quot;<br>\n", subject, from)
	fmt.Fprintf(htmlBody, "ed indirizzato a:<br>\n")
	for _, rcpt := range to {
		fmt.Fprintf(htmlBody, "%s (&quot;posta certificata&quot;)<br>\n", rcpt)
	}
	fmt.Fprintf(htmlBody, "<br><br>\n")
	fmt.Fprintf(htmlBody, "Il messaggio &egrave; stato accettato dal sistema ed inoltrato.<br>\n")
	fmt.Fprintf(htmlBody, "Identificativo messaggio: %s<br>\n", generatedMessageID)
	fmt.Fprintf(htmlBody, "</body>\n</html>\n")

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
	signedEmail.Header.Set("X-Ricevuta", "accettazione")
	signedEmail.Header.Set("Date", now.Format(time.RFC1123Z))
	signedEmail.Header.Set("Subject", fmt.Sprintf("ACCETTAZIONE: %s", subject))
	signedEmail.Header.Set("From", fmt.Sprintf("posta-certificata@%s", domain))
	signedEmail.Header.Set("To", from)
	signedEmail.Header.Set("X-Riferimento-Message-ID", messageID)

	return signedEmail, nil
}
