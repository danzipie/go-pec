package main

import (
	"fmt"
	"testing"
)

func TestValidateSMIME(t *testing.T) {
	// disable this test
	t.Skip()

	filename := "test/resources/email_2.eml"
	emlData := readEmail(filename)
	if emlData == nil {
		fmt.Printf("Error reading file %s", filename)
		t.Errorf("Error reading file %s", filename)
	}

	isValid := validateSMIMESignature(emlData)
	if !isValid {
		t.Errorf("Expected true, got %t", isValid)
	}
}
