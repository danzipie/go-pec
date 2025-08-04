package pec_storage

import (
	"fmt"
	"strings"
	"sync"

	"github.com/emersion/go-imap"
)

// InMemoryStore implements MessageStore using in-memory storage
type InMemoryStore struct {
	mu           sync.RWMutex
	messages     map[string][]*imap.Message // key: username
	passwordHash map[string]string          // key: username
	nextUID      map[string]uint32

	// For IDLE notifications
	notifiers   map[string]func() // key: username, value: notification function
	notifiersMu sync.RWMutex
}

// NewInMemoryStore creates a new in-memory message store
func NewInMemoryStore() *InMemoryStore {
	fmt.Println("Using in-memory message store")
	return &InMemoryStore{
		messages:     make(map[string][]*imap.Message),
		passwordHash: make(map[string]string),
		nextUID:      make(map[string]uint32),
		notifiers:    make(map[string]func()),
	}
}

// Register a notifier for a mailbox
func (s *InMemoryStore) RegisterNotifier(username string, notify func()) {
	s.notifiersMu.Lock()
	defer s.notifiersMu.Unlock()

	if s.notifiers == nil {
		s.notifiers = make(map[string]func())
	}
	s.notifiers[username] = notify
}

// Update AddMessage to trigger notifications
func (s *InMemoryStore) AddMessage(username string, msg *imap.Message) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	to := username
	if i := strings.Index(username, "@"); i > 0 {
		to = username[:i] // Take only the part before @
	}

	if _, ok := s.messages[to]; !ok {
		s.messages[to] = make([]*imap.Message, 0)
		s.nextUID[to] = 1 // Start UIDs at 1 for new mailboxes
	}

	// Assign a sequential UID
	msg.Uid = s.nextUID[to]
	s.nextUID[to]++

	// Assign sequence number based on position
	msg.SeqNum = uint32(len(s.messages[to]) + 1)

	// Set \Recent flag if not already present
	hasRecent := false
	for _, flag := range msg.Flags {
		if flag == imap.RecentFlag {
			hasRecent = true
			break
		}
	}
	if !hasRecent {
		msg.Flags = append(msg.Flags, imap.RecentFlag)
	}

	// Add message to mailbox
	s.messages[to] = append(s.messages[to], msg)

	fmt.Printf("Message added for user: %s, Total messages: %d, UID: %d, SeqNum: %d\n",
		to, len(s.messages[to]), msg.Uid, msg.SeqNum)

	// Trigger notification
	s.notifiersMu.RLock()
	notify := s.notifiers[to]
	s.notifiersMu.RUnlock()

	if notify != nil {
		go notify() // Call notification function
	}

	return nil
}

// GetMessages implements MessageStore.GetMessages
func (s *InMemoryStore) GetMessages(username string) ([]*imap.Message, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	fmt.Println("Retrieving messages for user:", username)

	fmt.Println("Total messages for user:", username, "is", len(s.messages[username]))
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
