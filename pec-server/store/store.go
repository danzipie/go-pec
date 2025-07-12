package store

import (
	"sync"

	"github.com/emersion/go-imap"
)

// MessageStore defines the interface for storing and retrieving PEC messages
type MessageStore interface {
	// AddMessage adds a message to the store for a specific user
	AddMessage(username string, msg *imap.Message) error

	// GetMessages retrieves all messages for a specific user
	GetMessages(username string) ([]*imap.Message, error)

	// GetMessage retrieves a specific message by UID for a user
	GetMessage(username string, uid uint32) (*imap.Message, error)

	// DeleteMessage deletes a specific message by UID for a user
	DeleteMessage(username string, uid uint32) error

	// Close releases any resources used by the store
	Close() error
}

// InMemoryStore implements MessageStore using in-memory storage
type InMemoryStore struct {
	mu       sync.RWMutex
	messages map[string][]*imap.Message // key: username
}

// NewInMemoryStore creates a new in-memory message store
func NewInMemoryStore() *InMemoryStore {
	return &InMemoryStore{
		messages: make(map[string][]*imap.Message),
	}
}

// AddMessage implements MessageStore.AddMessage
func (s *InMemoryStore) AddMessage(username string, msg *imap.Message) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.messages[username]; !ok {
		s.messages[username] = make([]*imap.Message, 0)
	}
	s.messages[username] = append(s.messages[username], msg)
	return nil
}

// GetMessages implements MessageStore.GetMessages
func (s *InMemoryStore) GetMessages(username string) ([]*imap.Message, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if msgs, ok := s.messages[username]; ok {
		return msgs, nil
	}
	return nil, nil
}

// GetMessage implements MessageStore.GetMessage
func (s *InMemoryStore) GetMessage(username string, uid uint32) (*imap.Message, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if msgs, ok := s.messages[username]; ok {
		for _, msg := range msgs {
			if msg.Uid == uid {
				return msg, nil
			}
		}
	}
	return nil, nil
}

// DeleteMessage implements MessageStore.DeleteMessage
func (s *InMemoryStore) DeleteMessage(username string, uid uint32) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if msgs, ok := s.messages[username]; ok {
		for i, msg := range msgs {
			if msg.Uid == uid {
				// Remove message at index i
				s.messages[username] = append(msgs[:i], msgs[i+1:]...)
				return nil
			}
		}
	}
	return nil
}

// Close implements MessageStore.Close
func (s *InMemoryStore) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Clear all messages
	s.messages = make(map[string][]*imap.Message)
	return nil
}
