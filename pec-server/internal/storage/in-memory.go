package pec_storage

import (
	"fmt"
	"sync"

	"github.com/emersion/go-imap"
)

// InMemoryStore implements MessageStore using in-memory storage
type InMemoryStore struct {
	mu           sync.RWMutex
	messages     map[string][]*imap.Message // key: username
	passwordHash map[string]string          // key: username
}

// NewInMemoryStore creates a new in-memory message store
func NewInMemoryStore() *InMemoryStore {
	fmt.Println("Using in-memory message store")
	return &InMemoryStore{
		messages:     make(map[string][]*imap.Message),
		passwordHash: make(map[string]string),
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

func (s *InMemoryStore) UserExists(username string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	_, exists := s.passwordHash[username]
	return exists
}

func (s *InMemoryStore) CreateUserWithPassword(username, passwordHash string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.passwordHash[username] = passwordHash
	s.messages[username] = make([]*imap.Message, 0)
	return nil
}

func (s *InMemoryStore) GetUserPasswordHash(username string) (string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	hash, exists := s.passwordHash[username]
	if !exists {
		return "", fmt.Errorf("user not found: %s", username)
	}
	return hash, nil
}
