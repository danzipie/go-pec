package main

import (
	"fmt"
	"os"
	"testing"
)

func TestValidateSMIME(t *testing.T) {

	// Read the EML file
	emlData, err := os.ReadFile("test_mails/email1.eml")
	if err != nil {
		fmt.Println("Error reading file:", err)
		return
	}
	isValid := validateSMIMESignature(emlData)
	if !isValid {
		t.Errorf("Expected true, got %t", isValid)
	}
}
