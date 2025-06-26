package pec

import (
	"bytes"
	"encoding/base64"
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

// decodeBase64IfNeeded checks if the signature is base64 encoded and decodes it
func decodeBase64IfNeeded(data []byte) []byte {
	// encoded := strings.TrimSpace(string(data))

	decoded, err := base64.StdEncoding.DecodeString(string(data))
	if err != nil {
		fmt.Println("Error decoding base64:", err)
		return data
	}
	return decoded
}

func normalizeLineEndings(data []byte) []byte {
	return bytes.ReplaceAll(data, []byte("\n"), []byte("\r\n"))
}
