package main

import "testing"

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

func TestPECHeaders(t *testing.T) {

	email := readEmail("test_mails/email_1.eml")
	header := email.Header
	pecMail := PECMail{}
	extractPECHeaders(&header, &pecMail)

	if header.Get("X-Riferimento-Message-ID") != "<CZPXCJRZKQDRVYXFAZYUIAWNACDAAHEVAEXAKN@example.com>" {
		t.Errorf("expected <CZPXCJRZKQDRVYXFAZYUIAWNACDAAHEVAEXAKN@example.com>, got %s", header.Get("X-Riferimento-Message-ID"))
	}
}

func TestParseAccettazione(t *testing.T) {
	email := readEmail("test_mails/email_1.eml")

	pecMail, datiCert, e := parsePec(email)
	if e != nil {
		t.Fatalf("failed to parse email: %v", e)
	}

	if pecMail.PecType != AcceptanceReceipt {
		t.Errorf("expected AcceptanceReceipt, got %v", pecMail.PecType)
	}

	if datiCert.Errore != "nessuno" {
		t.Errorf("expected nessuno, got %s", datiCert.Errore)
	}

	if datiCert.Intestazione.Mittente != "sender@example.com" {
		t.Errorf("expected sender@example.com got %s", datiCert.Intestazione.Mittente)
	}

}
