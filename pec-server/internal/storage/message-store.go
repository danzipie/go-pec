package pec_storage

import (
	"github.com/emersion/go-imap"
)

// MessageStore defines the interface for storing and retrieving PEC messages
type MessageStore interface {

	// CreateUser creates a new user in the store
	CreateUserWithPassword(username, passwordHash string) error
	GetUserPasswordHash(username string) (string, error)

	// AddMessage adds a message to the store for a specific user
	AddMessage(username string, msg *imap.Message) error

	// GetMessages retrieves all messages for a specific user
	GetMessages(username string) ([]*imap.Message, error)

	// GetMessage retrieves a specific message by UID for a user
	GetMessage(username string, uid uint32) (*imap.Message, error)

	// DeleteMessage deletes a specific message by UID for a user
	DeleteMessage(username string, uid uint32) error

	UserExists(username string) bool

	// Close releases any resources used by the store
	Close() error
}
