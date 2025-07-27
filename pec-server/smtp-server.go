package main

import (
	"bytes"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"io"
	"log"
	"os"
	"time"

	"github.com/danzipie/go-pec/pec-server/store"
	"github.com/emersion/go-imap"
	"github.com/emersion/go-message"
	"github.com/emersion/go-message/mail"
	"github.com/emersion/go-sasl"
	"github.com/emersion/go-smtp"
)

// The Backend implements SMTP server methods.
type Backend struct {
	signer *Signer
	store  store.MessageStore
}

func NewBackend(signer *Signer, store store.MessageStore) *Backend {
	return &Backend{
		signer: signer,
		store:  store,
	}
}

// NewSession is called after client greeting (EHLO, HELO).
func (bkd *Backend) NewSession(c *smtp.Conn) (smtp.Session, error) {
	return &Session{
		signer: bkd.signer,
		store:  bkd.store,
	}, nil
}

// A Session is returned after successful login.
type Session struct {
	from   string
	to     []string
	data   bytes.Buffer
	auth   bool
	signer *Signer
	store  store.MessageStore
}

// AuthMechanisms returns a slice of available auth mechanisms; only PLAIN is
// supported in this example.
func (s *Session) AuthMechanisms() []string {
	return []string{sasl.Plain}
}

// Auth is the handler for supported authenticators.
func (s *Session) Auth(mech string) (sasl.Server, error) {
	return sasl.NewPlainServer(func(identity, username, password string) error {
		if username != "username" || password != "password" {
			return errors.New("invalid username or password")
		}
		s.auth = true
		return nil
	}), nil
}

func (s *Session) Mail(from string, opts *smtp.MailOptions) error {
	if !s.auth {
		return smtp.ErrAuthRequired
	}
	log.Println("Mail from:", from)
	s.from = from
	return nil
}

func (s *Session) Rcpt(to string, opts *smtp.RcptOptions) error {
	if !s.auth {
		return smtp.ErrAuthRequired
	}
	log.Println("Rcpt to:", to)
	s.to = append(s.to, to)
	return nil
}

func (s *Session) Data(r io.Reader) error {
	if !s.auth {
		return smtp.ErrAuthRequired
	}
	if b, err := io.ReadAll(r); err != nil {
		return err
	} else {
		log.Println("Data:", string(b))
		s.data.Write(b)
		// Process the email data
		if err := AccessPointHandler(s); err != nil {
			log.Println("Error processing email data:", err)
			return err
		}
	}
	return nil
}

func (s *Session) Reset() {}

func (s *Session) Logout() error {
	return nil
}

func SaveSmtpEnvelopeToFile(s *Session, filename string) error {
	f, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer f.Close()

	// Write the SMTP envelope to the file
	if _, err := f.WriteString("Return-Path: " + s.from + "\n"); err != nil {
		return err
	}
	for _, recipient := range s.to {
		if _, err := f.WriteString("Forward-Path: " + recipient + "\n"); err != nil {
			return err
		}
	}
	f.WriteString("\n") // End of headers

	// Write the raw DATA bytes to the file
	if _, err := f.Write(s.data.Bytes()); err != nil {
		return err
	}

	return nil
}

// SaveRawEmailToFile saves the raw DATA bytes to a file as-is.
func SaveRawEmailToFile(r io.Reader, filename string) error {
	f, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = io.Copy(f, r)
	return err
}

func parseEmailFromSession(s Session) (*mail.Header, []byte, error) {
	r := bytes.NewReader(s.data.Bytes())
	mr, err := mail.CreateReader(r)
	if err != nil {
		return nil, nil, err
	}

	header := mr.Header

	p, err := mr.NextPart()
	if err != nil {
		return &header, nil, err
	}
	body, err := io.ReadAll(p.Body)
	if err != nil {
		return &header, nil, err
	}

	return &header, body, nil
}

func LoadSMIMECredentials(certPath, keyPath string) (*x509.Certificate, interface{}, error) {
	certPEM, err := os.ReadFile(certPath)
	if err != nil {
		return nil, nil, err
	}
	keyPEM, err := os.ReadFile(keyPath)
	if err != nil {
		return nil, nil, err
	}

	// Parse certificate
	block, _ := pem.Decode(certPEM)
	if block == nil || block.Type != "CERTIFICATE" {
		return nil, nil, errors.New("failed to decode certificate")
	}
	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return nil, nil, err
	}

	// Parse private key
	keyBlock, _ := pem.Decode(keyPEM)
	if keyBlock == nil {
		return nil, nil, errors.New("failed to decode private key")
	}
	privKey, err := x509.ParsePKCS8PrivateKey(keyBlock.Bytes)
	if err != nil {
		// fallback
		privKey, err = x509.ParsePKCS1PrivateKey(keyBlock.Bytes)
	}
	if err != nil {
		return nil, nil, err
	}

	return cert, privKey, nil
}

// StartSMTP starts the SMTP server with the given configuration
func StartSMTP(addr string, domain string, backend *Backend) error {
	s := smtp.NewServer(backend)
	s.Addr = addr
	s.Domain = domain
	s.AllowInsecureAuth = true // Allow plain auth over STARTTLS
	s.TLSConfig = &tls.Config{
		Certificates: []tls.Certificate{
			{
				Certificate: [][]byte{backend.signer.Cert.Raw},
				PrivateKey:  backend.signer.Key,
			},
		},
		MinVersion:         tls.VersionTLS12,
		InsecureSkipVerify: true,
		ClientAuth:         tls.NoClientCert,
	}

	log.Printf("Starting SMTP server at %v with STARTTLS support", s.Addr)
	return s.ListenAndServe()
}

// Helper function to convert message.Entity to imap.Message
func convertToIMAPMessage(entity *message.Entity) *imap.Message {
	msg := &imap.Message{
		Envelope: &imap.Envelope{
			Date:    time.Now(),
			Subject: entity.Header.Get("Subject"),
			From:    []*imap.Address{{HostName: entity.Header.Get("From")}},
			To:      []*imap.Address{{HostName: entity.Header.Get("To")}},
		},
		Body:         make(map[*imap.BodySectionName]imap.Literal),
		Flags:        []string{imap.SeenFlag},
		InternalDate: time.Now(),
		Uid:          uint32(time.Now().Unix()),
	}

	// Store the message body
	var buf bytes.Buffer
	entity.WriteTo(&buf)
	msg.Size = uint32(buf.Len())

	return msg
}
