package pec

import (
	"bytes"
	"fmt"
	"net/mail"
	"testing"
)

func TestParseDatiCertXML(t *testing.T) {
	// Test case
	xmlContent := `
		<postacert tipo="accettazione" errore="nessuno">
			<intestazione>
        		<mittente>sender@example.com</mittente>
        		<destinatari tipo="certificato">recipient@example.com</destinatari>
       			<risposte>sender@example.com</risposte>
        		<oggetto>Subject</oggetto>
    		</intestazione>
			<dati>
        	<gestore-emittente>trust</gestore-emittente>
        	<data zona="+0200">
				<giorno>13/05/2021</giorno>
				<ora>14:35:26</ora>
        	</data>
        	<identificativo>unique-id</identificativo>
        	<msgid>unique-msg-id</msgid>
    		</dati>
		</postacert>
		`
	daticert, err := parseDatiCertXML(xmlContent)
	if err != nil {
		t.Fatalf("failed to parse XML: %v", err)
	}
	if daticert.Tipo != "accettazione" {
		t.Errorf("expected accettazione, got %s", daticert.Tipo)
	}
	if daticert.Errore != "nessuno" {
		t.Errorf("expected nessuno, got %s", daticert.Errore)
	}
	if daticert.Intestazione.Mittente != "sender@example.com" {
		t.Errorf("expected sender@example.com, got %s", daticert.Intestazione.Mittente)
	}
}

func TestParseDatiCertXMLErroreEsteso(t *testing.T) {
	xmlContent := `
		<?xml version="1.0" encoding="UTF-8"?>
		<postacert tipo="errore-consegna" errore="no-dest">
			<intestazione>
				<mittente>sender@fakepec.it</mittente>
				<destinatari tipo="certificato">rec@pec.it</destinatari>
				<risposte>sender@fakepec.it</risposte>a
				<oggetto>Test PEC</oggetto>
			</intestazione>
			<dati>
				<gestore-emittente>FAKE PEC S.p.A.</gestore-emittente>
				<data zona="+0100">
					<giorno>15/11/2024</giorno>
					<ora>18:21:03</ora>
				</data>
				<identificativo>opec210312.20241115182038.288127.606.1.53@fakepec.it</identificativo>
				<msgid>&lt;SN05IE$951DEC16C1CFD3E4FD8FF1B1D24A99AE@fakepec.it&gt;</msgid>
				<consegna>rec@fakepec.it</consegna>
				<errore-esteso>5.1.1 - FAKE Pec S.p.A. - indirizzo non valido</errore-esteso>
			</dati>
		</postacert>`

	daticert, err := parseDatiCertXML(xmlContent)
	if err != nil {
		t.Fatalf("failed to parse XML: %v", err)
	}
	if daticert.Tipo != "errore-consegna" {
		t.Errorf("expected errore-consegna, got %s", daticert.Tipo)
	}
	if daticert.Errore != "no-dest" {
		t.Errorf("expected no-dest, got %s", daticert.Errore)
	}
	if daticert.Intestazione.Mittente != "sender@fakepec.it" {
		t.Errorf("expected sender@fakepec.it, got %s", daticert.Intestazione.Mittente)
	}
}

func TestPECHeaders(t *testing.T) {

	filename := "test/resources/accettazione.eml"
	emlData := ReadEmail(filename)
	if emlData == nil {
		t.Errorf("Error reading file %s", filename)
	}

	msg, err := mail.ReadMessage(bytes.NewReader(emlData))
	if err != nil {
		fmt.Println("Error parsing email:", err)
		t.Errorf("Error parsing email %s", err)
	}

	header := msg.Header
	pecMail := PECMail{}
	extractPECHeaders(&header, &pecMail)

	if header.Get("X-Riferimento-Message-ID") != "<SN05IE$951DEC16C1CFD3E4FD8FF1B1D24A99AE@fakepec.it>" {
		t.Errorf("expected <SN05IE$951DEC16C1CFD3E4FD8FF1B1D24A99AE@fakepec.it>, got %s", header.Get("X-Riferimento-Message-ID"))
	}
}

func TestParseAccettazione(t *testing.T) {
	filename := "test/resources/accettazione.eml"
	emlData := ReadEmail(filename)
	if emlData == nil {
		t.Errorf("Error reading file %s", filename)
	}

	msg, err := mail.ReadMessage(bytes.NewReader(emlData))
	if err != nil {
		t.Errorf("Error parsing email %s", err)
	}

	pecMail, datiCert, e := ParsePec(msg)
	if e != nil {
		t.Fatalf("failed to parse email: %v", e)
	}

	if pecMail.PecType != AcceptanceReceipt {
		t.Errorf("expected AcceptanceReceipt, got %v", pecMail.PecType)
	}

	if datiCert.Errore != "nessuno" {
		t.Errorf("expected nessuno, got %s", datiCert.Errore)
	}

	if datiCert.Intestazione.Mittente != "sender@fakepec.it" {
		t.Errorf("expected sender@fakepec.it got %s", datiCert.Intestazione.Mittente)
	}

}

func TestParseDelivery(t *testing.T) {
	filename := "test/resources/email_2.eml"
	emlData := ReadEmail(filename)
	if emlData == nil {
		t.Errorf("Error reading file %s", filename)
	}

	msg, err := mail.ReadMessage(bytes.NewReader(emlData))
	if err != nil {
		t.Errorf("Error parsing email %s", err)
	}

	_, _, e := ParsePec(msg)
	if e != nil {
		t.Fatalf("failed to parse email: %v", e)
	}

}

func TestParseDeliveryError(t *testing.T) {
	filename := "test/resources/consegna.eml"
	emlData := ReadEmail(filename)
	if emlData == nil {
		fmt.Printf("Error reading file %s", filename)
		return
	}

	msg, err := mail.ReadMessage(bytes.NewReader(emlData))
	if err != nil {
		fmt.Println("Error parsing email:", err)
		return
	}

	_, _, e := ParsePec(msg)
	if e != nil {
		t.Fatalf("failed to parse email: %v", e)
	}

}

func TestParseCertifiedEmail(t *testing.T) {
	// disable this test
	t.Skip()

	filename := "test/resources/email_3.eml"
	emlData := ReadEmail(filename)
	if emlData == nil {
		t.Errorf("Error reading file %s", filename)
	}

	msg, err := mail.ReadMessage(bytes.NewReader(emlData))
	if err != nil {
		t.Errorf("Error parsing email %s", err)
	}

	_, _, e := ParsePec(msg)
	if e != nil {
		t.Fatalf("failed to parse email: %v", e)
	}

}

func TestParseAndVerify(t *testing.T) {
	// disable this test
	t.Skip()

	filename := "test/resources/email_3.eml"
	emlData := ReadEmail(filename)
	if emlData == nil {
		t.Errorf("Error reading file %s", filename)
	}

	msg, err := mail.ReadMessage(bytes.NewReader(emlData))
	if err != nil {
		t.Errorf("Error parsing email %s", err)
	}

	_, _, e := ParsePec(msg)
	if e != nil {
		t.Fatalf("failed to parse email: %v", e)
	}

	if VerifySMIMEWithOpenSSL(filename) != nil {
		t.Fatalf("Verification failed")
	}
}
