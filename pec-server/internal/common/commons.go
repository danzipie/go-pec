package common

import (
	"bytes"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/danzipie/go-pec/pec"
	"github.com/emersion/go-imap"
	"github.com/emersion/go-message"
	"github.com/emersion/go-message/mail"
)

// ParseEmailMessage parses a raw email message and returns a *mail.Reader.
func ParseEmailMessage(rawMessage []byte) (*mail.Reader, error) {
	reader := strings.NewReader(string(rawMessage))

	// Parse the message entity
	entity, err := message.Read(reader)
	if err != nil {
		return nil, fmt.Errorf("failed to parse email message: %w", err)
	}

	// Wrap it in a mail.Reader to get structured headers
	msgReader := mail.NewReader(entity)

	return msgReader, nil
}

// ExtractRecipients extracts and cleans recipient addresses from To and Cc headers
func ExtractRecipients(headers *mail.Header) []string {
	recipients := []string{}

	// Extract from To header
	if to := headers.Get("To"); to != "" {
		toAddrs, err := mail.ParseAddressList(to)
		if err == nil {
			for _, addr := range toAddrs {
				recipients = append(recipients, addr.Address)
			}
		} else {
			// Fallback to simple splitting if parsing fails
			toAddrs := strings.Split(to, ",")
			for _, addr := range toAddrs {
				recipients = append(recipients, strings.TrimSpace(addr))
			}
		}
	}

	// Extract from Cc header
	if cc := headers.Get("Cc"); cc != "" {
		ccAddrs, err := mail.ParseAddressList(cc)
		if err == nil {
			for _, addr := range ccAddrs {
				recipients = append(recipients, addr.Address)
			}
		} else {
			// Fallback to simple splitting if parsing fails
			ccAddrs := strings.Split(cc, ",")
			for _, addr := range ccAddrs {
				recipients = append(recipients, strings.TrimSpace(addr))
			}
		}
	}

	return recipients
}

// Helper function to convert message.Entity to imap.Message
func ConvertToIMAPMessage(entity *message.Entity) *imap.Message {
	msg := &imap.Message{
		Envelope: &imap.Envelope{
			Date:    time.Now(),
			Subject: entity.Header.Get("Subject"),
			From:    []*imap.Address{{HostName: entity.Header.Get("From")}},
			To:      []*imap.Address{{HostName: entity.Header.Get("To")}},
		},
		Body:         make(map[*imap.BodySectionName]imap.Literal),
		Flags:        []string{imap.SeenFlag},
		InternalDate: time.Now(),
		Uid:          uint32(time.Now().Unix()),
	}

	// Store the message body
	var buf bytes.Buffer
	entity.WriteTo(&buf)
	msg.Size = uint32(buf.Len())

	return msg
}

// IsSignatureValid checks if the S/MIME signature of the message is valid.
// It writes the body to a temporary file and calls verifySMIMEWithOpenSSL.
func IsSignatureValid(header *mail.Header, body []byte) bool {
	// Write body to a temporary file
	tmpFile, err := os.CreateTemp("", "pec-smime-*.eml")
	if err != nil {
		return false
	}
	defer os.Remove(tmpFile.Name())
	if _, err := tmpFile.Write(body); err != nil {
		tmpFile.Close()
		return false
	}
	tmpFile.Close()

	// Use your OpenSSL-based verifier
	if err := pec.VerifySMIMEWithOpenSSL(tmpFile.Name()); err != nil {
		return false
	}
	return true
}
