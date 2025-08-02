package common

import (
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"log"
	"time"

	pec_storage "github.com/danzipie/go-pec/pec-server/internal/storage"
	"github.com/emersion/go-imap"
	"github.com/emersion/go-imap/backend"
	imapserver "github.com/emersion/go-imap/server"
	"golang.org/x/crypto/bcrypt"
)

var (
	// ErrMailboxNotAllowed is returned when a mailbox operation is not allowed
	ErrMailboxNotAllowed = errors.New("mailbox operation not allowed")
	// ErrNotAllowed is returned when an operation is not allowed
	ErrNotAllowed = errors.New("operation not allowed")
)

// IMAPBackend implements the IMAP server backend
type IMAPBackend struct {
	store pec_storage.MessageStore
	cert  *x509.Certificate
	key   interface{}
}

func NewIMAPBackend(store pec_storage.MessageStore, cert *x509.Certificate, key interface{}) *IMAPBackend {
	fmt.Println("Creating IMAP backend")
	return &IMAPBackend{
		store: store,
		cert:  cert,
		key:   key,
	}
}

func (b *IMAPBackend) Login(connInfo *imap.ConnInfo, username, password string) (backend.User, error) {
	log.Printf("Login attempt: %s", username)

	// Check if user exists
	if !b.store.UserExists(username) {
		log.Printf("Creating new user: %s", username)

		// Hash the provided password
		hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
		if err != nil {
			return nil, fmt.Errorf("failed to hash password: %v", err)
		}

		// Create user with the hashed password
		if err := b.store.CreateUserWithPassword(username, string(hashedPassword)); err != nil {
			return nil, fmt.Errorf("failed to create user: %v", err)
		}

		log.Printf("User created successfully: %s", username)
	} else {
		// User exists, verify password
		storedHash, err := b.store.GetUserPasswordHash(username)
		if err != nil {
			return nil, err
		}

		// Compare the stored hash with the provided password
		if err := bcrypt.CompareHashAndPassword([]byte(storedHash), []byte(password)); err != nil {
			// Password doesn't match
			log.Printf("Failed login attempt for %s: invalid password", username)
			return nil, errors.New("invalid username or password")
		}
	}

	return &IMAPUser{
		username: username,
		store:    b.store,
	}, nil
}

// IMAPUser represents an authenticated user
type IMAPUser struct {
	username string
	store    pec_storage.MessageStore
}

func (u *IMAPUser) Username() string {
	return u.username
}

func (u *IMAPUser) ListMailboxes(subscribed bool) ([]backend.Mailbox, error) {
	// For now, just return INBOX
	return []backend.Mailbox{
		&IMAPMailbox{
			name:     "INBOX",
			username: u.username,
			store:    u.store,
		},
	}, nil
}

func (u *IMAPUser) GetMailbox(name string) (backend.Mailbox, error) {
	// For now, only support INBOX
	if name != "INBOX" {
		return nil, backend.ErrNoSuchMailbox
	}
	return &IMAPMailbox{
		name:     name,
		username: u.username,
		store:    u.store,
	}, nil
}

// CreateMailbox creates a new mailbox
func (u *IMAPUser) CreateMailbox(name string) error {
	return ErrMailboxNotAllowed
}

func (u *IMAPUser) DeleteMailbox(name string) error {
	return ErrMailboxNotAllowed
}

func (u *IMAPUser) RenameMailbox(existingName, newName string) error {
	return ErrMailboxNotAllowed
}

func (u *IMAPUser) Logout() error {
	return nil
}

// IMAPMailbox represents a mailbox (folder)
type IMAPMailbox struct {
	name     string
	username string
	store    pec_storage.MessageStore
}

func (m *IMAPMailbox) Name() string {
	return m.name
}

func (m *IMAPMailbox) Info() (*imap.MailboxInfo, error) {
	info := &imap.MailboxInfo{
		Delimiter: "/",
		Name:      m.name,
	}
	return info, nil
}

func (m *IMAPMailbox) Status(items []imap.StatusItem) (*imap.MailboxStatus, error) {
	status := imap.NewMailboxStatus(m.name, items)
	messages, err := m.store.GetMessages(m.username)
	if err != nil {
		return nil, err
	}

	for _, item := range items {
		switch item {
		case imap.StatusMessages:
			status.Messages = uint32(len(messages))
		case imap.StatusUidNext:
			status.UidNext = uint32(len(messages) + 1)
		case imap.StatusUidValidity:
			status.UidValidity = 1
		case imap.StatusRecent:
			status.Recent = 0 // We don't support recent messages
		case imap.StatusUnseen:
			status.Unseen = 0 // We don't track seen/unseen status yet
		}
	}

	return status, nil
}

func (m *IMAPMailbox) SetSubscribed(subscribed bool) error {
	// We don't support subscription
	return nil
}

func (m *IMAPMailbox) Check() error {
	return nil
}

func (m *IMAPMailbox) ListMessages(uid bool, seqSet *imap.SeqSet, items []imap.FetchItem, ch chan<- *imap.Message) error {
	defer close(ch)

	messages, err := m.store.GetMessages(m.username)
	if err != nil {
		return err
	}

	for i, msg := range messages {
		seqNum := uint32(i + 1)

		// Check if message is in the sequence set
		if !seqSet.Contains(seqNum) {
			continue
		}

		// Create a copy of the message for the fetch response
		fetchedMsg := imap.NewMessage(seqNum, items)
		fetchedMsg.Uid = msg.Uid
		fetchedMsg.Size = msg.Size
		fetchedMsg.Flags = msg.Flags
		fetchedMsg.Envelope = msg.Envelope

		// Only include requested items
		for _, item := range items {
			switch item {
			case imap.FetchEnvelope:
				fetchedMsg.Envelope = msg.Envelope
			case imap.FetchBody, imap.FetchBodyStructure:
				fetchedMsg.BodyStructure = msg.BodyStructure
			case imap.FetchFlags:
				fetchedMsg.Flags = msg.Flags
			case imap.FetchInternalDate:
				fetchedMsg.InternalDate = msg.InternalDate
			case imap.FetchRFC822Size:
				fetchedMsg.Size = msg.Size
			case imap.FetchUid:
				fetchedMsg.Uid = msg.Uid
			}
		}

		ch <- fetchedMsg
	}

	return nil
}

func (m *IMAPMailbox) SearchMessages(uid bool, criteria *imap.SearchCriteria) ([]uint32, error) {
	var ids []uint32
	messages, err := m.store.GetMessages(m.username)
	if err != nil {
		return nil, err
	}

	for i, msg := range messages {
		if matchesCriteria(msg, criteria) {
			if uid {
				ids = append(ids, msg.Uid)
			} else {
				ids = append(ids, uint32(i+1))
			}
		}
	}

	return ids, nil
}

func matchesCriteria(msg *imap.Message, criteria *imap.SearchCriteria) bool {
	// Implement search criteria matching
	// For now, return true to match all messages
	return true
}

func (m *IMAPMailbox) CreateMessage(flags []string, date time.Time, body imap.Literal) error {
	// We don't allow creating messages via IMAP
	return ErrNotAllowed
}

func (m *IMAPMailbox) UpdateMessagesFlags(uid bool, seqSet *imap.SeqSet, operation imap.FlagsOp, flags []string) error {
	// We don't support updating flags
	return ErrNotAllowed
}

func (m *IMAPMailbox) CopyMessages(uid bool, seqSet *imap.SeqSet, destName string) error {
	// We don't support copying messages
	return ErrNotAllowed
}

func (m *IMAPMailbox) Expunge() error {
	// We don't support expunging messages
	return ErrNotAllowed
}

// Add this new function to support direct TLS connections
func StartIMAPWithTLS(addr string, backend *IMAPBackend) error {
	s := imapserver.New(backend)
	s.Addr = addr

	// Create TLS config
	tlsConfig := &tls.Config{
		Certificates: []tls.Certificate{
			{
				Certificate: [][]byte{backend.cert.Raw},
				PrivateKey:  backend.key,
			},
		},
		MinVersion: tls.VersionTLS12,
		ClientAuth: tls.NoClientCert,
	}

	s.TLSConfig = tlsConfig

	log.Printf("Starting IMAP server with TLS at %v", addr)

	// Listen for TLS connections directly
	listener, err := tls.Listen("tcp", addr, tlsConfig)
	if err != nil {
		return err
	}

	return s.Serve(listener)
}

// Modify your existing StartIMAP function to clarify it uses STARTTLS
func StartIMAPWithSTARTTLS(addr string, backend *IMAPBackend) error {
	s := imapserver.New(backend)
	s.Addr = addr
	s.TLSConfig = &tls.Config{
		Certificates: []tls.Certificate{
			{
				Certificate: [][]byte{backend.cert.Raw},
				PrivateKey:  backend.key,
			},
		},
		MinVersion:         tls.VersionTLS12,
		InsecureSkipVerify: true,
		ClientAuth:         tls.NoClientCert,
	}
	log.Printf("Starting IMAP server at %v with STARTTLS support", addr)
	return s.ListenAndServe() // The go-imap server automatically supports STARTTLS
}
