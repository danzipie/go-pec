package main

import (
	"bytes"
	"errors"
	"io"
	"log"
	"net/mail"
	"os"
	"time"

	"github.com/emersion/go-sasl"
	"github.com/emersion/go-smtp"
)

// The Backend implements SMTP server methods.
type Backend struct{}

// NewSession is called after client greeting (EHLO, HELO).
func (bkd *Backend) NewSession(c *smtp.Conn) (smtp.Session, error) {
	return &Session{}, nil
}

// A Session is returned after successful login.
type Session struct {
	from string
	to   []string
	data bytes.Buffer
	auth bool
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
	}
	return nil
}

func (s *Session) Reset() {}

func (s *Session) Logout() error {
	return nil
}

// AddMissingDateHeader adds a Date header if it is missing from the message.
func AddMissingDateHeader(msg *mail.Message) {
	if msg.Header.Get("Date") == "" {
		msg.Header["Date"] = []string{time.Now().Format(time.RFC1123Z)}
	}
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

// main starts the SMTP server.
func main() {
	be := &Backend{}

	s := smtp.NewServer(be)

	s.Addr = "localhost:1025"
	s.Domain = "localhost"
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
