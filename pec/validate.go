package pec

import (
	"bytes"
	"fmt"
	"net/mail"
	"os"
	"os/exec"
)

// Function to verify the S/MIME signature using OpenSSL
// TBD: Remove dependency on external program
func verifySMIMEWithOpenSSL(emlFile string) error {
	cmd := exec.Command("openssl", "smime", "-verify", "-in", emlFile, "-noverify")
	cmd.Stdin = os.Stdin
	// cmd.Stdout = os.Stdout
	// cmd.Stderr = os.Stderr

	err := cmd.Run()
	if err != nil {
		return fmt.Errorf("OpenSSL verification failed: %v", err)
	}

	return nil
}

func Verify(filename string) error {
	emlData := ReadEmail(filename)
	if emlData == nil {
		return fmt.Errorf("Error reading file %s", filename)
	}

	msg, err := mail.ReadMessage(bytes.NewReader(emlData))
	if err != nil {
		return fmt.Errorf("Error parsing email %s", err)
	}

	_, _, e := ParsePec(msg)
	if e != nil {
		return fmt.Errorf("failed to parse email: %v", e)
	}

	if verifySMIMEWithOpenSSL(filename) != nil {
		return fmt.Errorf("Verification failed")
	}
	return nil
}
