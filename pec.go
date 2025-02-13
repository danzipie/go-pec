package main

import "encoding/xml"

// all PEC structures are defined here

type Envelope struct {
	From    string `json:"from"`
	To      string `json:"to"`
	Subject string `json:"subject"`
	Date    string `json:"date"`
}

type PecType int

const (
	None PecType = iota
	CertifiedEmail
	DeliveryReceipt
	DeliveryErrorReceipt
	AcceptanceReceipt
)

type PECMail struct {
	Envelope  Envelope `json:"envelope"`
	MessageID string   `json:"message_id"`
	PecType   PecType  `json:"pec_type"`
}

// Define the structure of the DatiCert XML
type DatiCert struct {
	XMLName      xml.Name `xml:"postacert"`
	Tipo         string   `xml:"tipo,attr"`
	Errore       string   `xml:"errore,attr"`
	Intestazione struct {
		Mittente     string `xml:"mittente"`
		Destinatario struct {
			Tipo string `xml:"tipo,attr"`
			Val  string `xml:",chardata"`
		} `xml:"destinatari"`
		Risposta string `xml:"risposte"`
		Oggetto  string `xml:"oggetto"`
	} `xml:"intestazione"`
	Dati struct {
		GestoreEmittente string `xml:"gestore-emittente"`
		Data             struct {
			Zona   string `xml:"zona,attr"`
			Giorno string `xml:"giorno"`
			Ora    string `xml:"ora"`
		} `xml:"data"`
		Identificativo string `xml:"identificativo"`
		MsgID          string `xml:"msgid"`
		ErroreEsteso   string `xml:"errore-esteso"`
	} `xml:"dati"`
}
