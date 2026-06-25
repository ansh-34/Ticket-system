package models

import "time"

// Status is the lifecycle state of a ticket.
type Status string

const (
	StatusOpen       Status = "open"
	StatusInProgress Status = "in_progress"
	StatusClosed     Status = "closed"
)

// rank orders the statuses along the allowed forward-only lifecycle:
//
//	open -> in_progress -> closed
//
// A ticket may only move to a status with a strictly higher rank. This makes
// every backward move (including reopening a closed ticket) invalid by
// construction.
var rank = map[Status]int{
	StatusOpen:       0,
	StatusInProgress: 1,
	StatusClosed:     2,
}

// IsValid reports whether s is a recognised status value.
func (s Status) IsValid() bool {
	_, ok := rank[s]
	return ok
}

// CanTransitionTo reports whether a ticket may move from s to next.
//
// Only forward moves are allowed (open -> in_progress -> closed, and the
// open -> closed shortcut). Same-status and any backward move (such as
// reopening a closed ticket) are rejected.
func (s Status) CanTransitionTo(next Status) bool {
	if !s.IsValid() || !next.IsValid() {
		return false
	}
	return rank[next] > rank[s]
}

// User is a registered account. The password is only ever stored hashed.
type User struct {
	ID           string
	Email        string
	PasswordHash string
	CreatedAt    time.Time
}

// Ticket is a unit of work owned by exactly one user.
type Ticket struct {
	ID        string    `json:"id"`
	Title     string    `json:"title"`
	Status    Status    `json:"status"`
	OwnerID   string    `json:"owner_id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}
