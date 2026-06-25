// Package store provides persistence for users and tickets.
//
// The default implementation is a concurrency-safe in-memory store, which keeps
// the assignment dependency-free while still being safe under the HTTP server's
// goroutine-per-request model. The Store interface makes it straightforward to
// swap in a SQL-backed implementation later without touching the HTTP layer.
package store

import (
	"errors"
	"sync"
	"time"

	"github.com/example/ticket-system/internal/models"
)

// Sentinel errors returned by the store. Callers should compare with
// errors.Is so the HTTP layer can map them to status codes.
var (
	ErrEmailTaken        = errors.New("email already registered")
	ErrUserNotFound      = errors.New("user not found")
	ErrTicketNotFound    = errors.New("ticket not found")
	ErrInvalidStatus     = errors.New("invalid status value")
	ErrInvalidTransition = errors.New("invalid status transition")
)

// Store is the persistence contract used by the HTTP handlers.
type Store interface {
	CreateUser(email, passwordHash string) (models.User, error)
	GetUserByEmail(email string) (models.User, error)

	CreateTicket(ownerID, title string) (models.Ticket, error)
	ListTicketsByOwner(ownerID string) ([]models.Ticket, error)
	// GetTicket returns the ticket only if it belongs to ownerID; otherwise it
	// returns ErrTicketNotFound so that ownership is never leaked.
	GetTicket(ownerID, ticketID string) (models.Ticket, error)
	// UpdateTicketStatus applies the requested status to an owned ticket,
	// enforcing the forward-only lifecycle.
	UpdateTicketStatus(ownerID, ticketID string, next models.Status) (models.Ticket, error)
}

// IDGenerator produces unique identifiers for new records.
type IDGenerator func() string

// MemoryStore is an in-memory, concurrency-safe Store.
type MemoryStore struct {
	mu           sync.RWMutex
	usersByID    map[string]models.User
	usersByEmail map[string]string // lowercased email -> user ID
	tickets      map[string]models.Ticket
	newID        IDGenerator
	now          func() time.Time
}

// NewMemoryStore builds an empty store. newID must return unique strings.
func NewMemoryStore(newID IDGenerator) *MemoryStore {
	return &MemoryStore{
		usersByID:    make(map[string]models.User),
		usersByEmail: make(map[string]string),
		tickets:      make(map[string]models.Ticket),
		newID:        newID,
		now:          time.Now,
	}
}

func (s *MemoryStore) CreateUser(email, passwordHash string) (models.User, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.usersByEmail[email]; exists {
		return models.User{}, ErrEmailTaken
	}
	u := models.User{
		ID:           s.newID(),
		Email:        email,
		PasswordHash: passwordHash,
		CreatedAt:    s.now().UTC(),
	}
	s.usersByID[u.ID] = u
	s.usersByEmail[email] = u.ID
	return u, nil
}

func (s *MemoryStore) GetUserByEmail(email string) (models.User, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	id, ok := s.usersByEmail[email]
	if !ok {
		return models.User{}, ErrUserNotFound
	}
	return s.usersByID[id], nil
}

func (s *MemoryStore) CreateTicket(ownerID, title string) (models.Ticket, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := s.now().UTC()
	t := models.Ticket{
		ID:        s.newID(),
		Title:     title,
		Status:    models.StatusOpen,
		OwnerID:   ownerID,
		CreatedAt: now,
		UpdatedAt: now,
	}
	s.tickets[t.ID] = t
	return t, nil
}

func (s *MemoryStore) ListTicketsByOwner(ownerID string) ([]models.Ticket, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// Always return a non-nil slice so the JSON encoder emits [] rather than null.
	out := make([]models.Ticket, 0)
	for _, t := range s.tickets {
		if t.OwnerID == ownerID {
			out = append(out, t)
		}
	}
	return out, nil
}

func (s *MemoryStore) GetTicket(ownerID, ticketID string) (models.Ticket, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	t, ok := s.tickets[ticketID]
	if !ok || t.OwnerID != ownerID {
		return models.Ticket{}, ErrTicketNotFound
	}
	return t, nil
}

func (s *MemoryStore) UpdateTicketStatus(ownerID, ticketID string, next models.Status) (models.Ticket, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	t, ok := s.tickets[ticketID]
	if !ok || t.OwnerID != ownerID {
		return models.Ticket{}, ErrTicketNotFound
	}
	if !next.IsValid() {
		return models.Ticket{}, ErrInvalidStatus
	}
	if !t.Status.CanTransitionTo(next) {
		return models.Ticket{}, ErrInvalidTransition
	}
	t.Status = next
	t.UpdatedAt = s.now().UTC()
	s.tickets[ticketID] = t
	return t, nil
}
