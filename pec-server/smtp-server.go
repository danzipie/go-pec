package main

import (
	"bytes"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"errors"
	"io"
	"log"
	"os"
	"time"

	"github.com/danzipie/go-pec/pec-server/logger"
	"github.com/emersion/go-message/mail"
	"github.com/emersion/go-sasl"
	"github.com/emersion/go-smtp"
)

// The Backend implements SMTP server methods.
type Backend struct {
	signer *Signer
}

func NewBackend(signer *Signer) *Backend {
	return &Backend{
		signer: signer,
	}
}

// NewSession is called after client greeting (EHLO, HELO).
func (bkd *Backend) NewSession(c *smtp.Conn) (smtp.Session, error) {
	return &Session{}, nil
}

// A Session is returned after successful login.
type Session struct {
	from   string
	to     []string
	data   bytes.Buffer
	auth   bool
	signer *Signer
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
		// Save the raw DATA to file, verbatim
		if err := SaveRawEmailToFile(bytes.NewReader(b), "received_email.eml"); err != nil {
			return err
		}
		if err := SaveSmtpEnvelopeToFile(s, "received_email.envelope.txt"); err != nil {
			return err
		}

		// Parse the email and log the header and body
		header, body, err := parseEmailFromSession(*s)
		if err != nil {
			return err
		}
		log.Println("Parsed Email Header:", header)
		log.Println("Parsed Email Body:", string(body))
		r := bytes.NewReader(s.data.Bytes())
		mr, err := mail.CreateReader(r)
		if err != nil {
			return err
		}
		if err := ValidateEnvelopeAndHeaders(s.from, s.to, mr); err != nil {
			if valErr, ok := err.(ValidationError); ok {
				log.Println("Validation Error:", valErr)
				// emit message of non-acceptance
				GenerateNonAcceptanceEmail("localhost", valErr, s.signer)
			}
			return err
		} else {
			log.Println("Envelope and headers validation passed")
			// TODO: Handle accepted message
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

type Config struct {
	Domain       string `json:"domain"`
	CertFilePath string `json:"cert_file"`
	KeyFilePath  string `json:"key_file"`
	SMTPServer   string `json:"smtp_server"`
}

func LoadConfig(path string) (*Config, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var cfg Config
	decoder := json.NewDecoder(f)
	if err := decoder.Decode(&cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}

// main starts the SMTP server.
func main() {

	cfg, err := LoadConfig("config.json")
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// Load S/MIME credentials
	cert, key, err := LoadSMIMECredentials(cfg.CertFilePath, cfg.KeyFilePath)
	if err != nil {
		log.Fatal("Cert load failed:", err)
	}

	// Initialize logger
	if err := logger.Init("pec.log"); err != nil {
		log.Fatal("Logger initialization failed:", err)
	}
	defer logger.Sync()

	signer := &Signer{
		Cert:   cert,
		Key:    key,
		Domain: cfg.Domain,
	}
	be := NewBackend(signer)

	s := smtp.NewServer(be)

	s.Addr = cfg.SMTPServer
	s.Domain = cfg.Domain
	s.WriteTimeout = 10 * time.Second
	s.ReadTimeout = 10 * time.Second
	s.MaxMessageBytes = 1024 * 1024
	s.MaxRecipients = 50
	s.AllowInsecureAuth = true

	log.Println("Starting server at", s.Addr)
	if err := s.ListenAndServe(); err != nil {
		log.Fatal(err)
	}
}
