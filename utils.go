package main

import (
	"log"
	"os"

	"github.com/emersion/go-message"
)

// readEmail reads and parses an .eml file, extracting headers, body, and attachments
func readEmail(filePath string) *message.Entity {
	// Open the .eml file
	file, err := os.Open(filePath)
	if err != nil {
		log.Fatal("Failed to open file:", err)
	}
	defer file.Close()

	// Create a new message reader
	decoder, err := message.Read(file)
	if err != nil {
		log.Fatal("Failed to read email:", err)
	}
	return decoder
}
