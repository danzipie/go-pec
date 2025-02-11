package main

import (
	"encoding/json"
	"fmt"
	"log"
)

func main() {

	// read email
	email := readEmail("test_mails/email1.eml")

	// parse email
	pecMail, datiCert, e := parsePec(email)
	if e != nil {
		log.Fatalf("failed to parse email: %v", e)
	}

	// verify signature
	// validateSMIMESignature()

	// print PECMail struct
	marshaled, err := json.MarshalIndent(pecMail, "", "   ")
	if err != nil {
		log.Fatalf("marshaling error: %s", err)
	}
	fmt.Println(string(marshaled))

	// print DatiCert struct
	marshaled, err = json.MarshalIndent(datiCert, "", "   ")
	if err != nil {
		log.Fatalf("marshaling error: %s", err)
	}
	fmt.Println(string(marshaled))

}
