package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/mail"

	"github.com/danzipie/go-pec/pec"
)

func main() {

	filename := "test_mails/email1.eml"
	emlData := pec.ReadEmail(filename)
	if emlData == nil {
		fmt.Printf("Error reading file %s", filename)
		return
	}

	msg, err := mail.ReadMessage(bytes.NewReader(emlData))
	if err != nil {
		fmt.Println("Error parsing email:", err)
		return
	}

	// parse email
	pecMail, datiCert, e := pec.ParsePec(msg)
	if e != nil {
		log.Fatalf("failed to parse email: %v", e)
	}

	// verify signature
	// ValidateSMIMESignature()

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
