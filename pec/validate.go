package pec

import (
	"fmt"
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
