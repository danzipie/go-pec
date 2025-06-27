package pec

import (
	"bytes"
	"fmt"
	"os"
)

// ReadEmail reads and parses an .eml file, extracting headers, body, and attachments
func ReadEmail(filePath string) []byte {
	// Read the EML file
	emlData, err := os.ReadFile(filePath)
	if err != nil {
		fmt.Println("Error reading file:", err)
		return nil
	}
	return emlData
}

func normalizeLineEndings(data []byte) []byte {
	return bytes.ReplaceAll(data, []byte("\n"), []byte("\r\n"))
}
