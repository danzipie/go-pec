package main

import (
	"fmt"
	"strings"

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
