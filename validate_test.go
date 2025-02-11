package main

import (
	"fmt"
	"testing"
)

func TestValidateSMIME(t *testing.T) {

	filename := "test_mails/email1.eml"
	emlData := readEmail(filename)
	if emlData == nil {
		fmt.Printf("Error reading file %s", filename)
		return
	}

	isValid := validateSMIMESignature(emlData)
	if !isValid {
		t.Errorf("Expected true, got %t", isValid)
	}
}
