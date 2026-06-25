package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/example/ticket-system/internal/auth"
	"github.com/example/ticket-system/internal/models"
	"github.com/example/ticket-system/internal/store"
)

// ---- Request / response payloads ----

type credentialsRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type tokenResponse struct {
	Token string `json:"token"`
}

type createTicketRequest struct {
	Title string `json:"title"`
}

type updateStatusRequest struct {
	Status string `json:"status"`
}

// ---- Handlers ----

func (s *Server) handleHealth(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) handleRegister(w http.ResponseWriter, r *http.Request) {
	var req credentialsRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	email := normaliseEmail(req.Email)
	if email == "" || !strings.Contains(email, "@") {
		writeError(w, http.StatusBadRequest, "a valid email is required")
		return
	}
	if len(req.Password) < 6 {
		writeError(w, http.StatusBadRequest, "password must be at least 6 characters")
		return
	}

	hash, err := auth.HashPassword(req.Password)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not process password")
		return
	}

	user, err := s.store.CreateUser(email, hash)
	if errors.Is(err, store.ErrEmailTaken) {
		writeError(w, http.StatusConflict, "email already registered")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not create user")
		return
	}

	token, err := s.tokens.Generate(user.ID, user.Email)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not issue token")
		return
	}
	writeJSON(w, http.StatusCreated, tokenResponse{Token: token})
}

func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	var req credentialsRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	email := normaliseEmail(req.Email)

	user, err := s.store.GetUserByEmail(email)
	// Use one generic message for both unknown user and wrong password so the
	// API does not reveal which emails are registered.
	if err != nil || !auth.CheckPassword(user.PasswordHash, req.Password) {
		writeError(w, http.StatusUnauthorized, "invalid email or password")
		return
	}

	token, err := s.tokens.Generate(user.ID, user.Email)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not issue token")
		return
	}
	writeJSON(w, http.StatusOK, tokenResponse{Token: token})
}

func (s *Server) handleCreateTicket(w http.ResponseWriter, r *http.Request) {
	userID := userIDFromContext(r.Context())

	var req createTicketRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	title := strings.TrimSpace(req.Title)
	if title == "" {
		writeError(w, http.StatusBadRequest, "title is required")
		return
	}

	ticket, err := s.store.CreateTicket(userID, title)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not create ticket")
		return
	}
	writeJSON(w, http.StatusCreated, ticket)
}

func (s *Server) handleListTickets(w http.ResponseWriter, r *http.Request) {
	userID := userIDFromContext(r.Context())

	tickets, err := s.store.ListTicketsByOwner(userID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not list tickets")
		return
	}
	writeJSON(w, http.StatusOK, tickets)
}

func (s *Server) handleGetTicket(w http.ResponseWriter, r *http.Request) {
	userID := userIDFromContext(r.Context())
	id := r.PathValue("id")

	ticket, err := s.store.GetTicket(userID, id)
	if errors.Is(err, store.ErrTicketNotFound) {
		writeError(w, http.StatusNotFound, "ticket not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not fetch ticket")
		return
	}
	writeJSON(w, http.StatusOK, ticket)
}

func (s *Server) handleUpdateStatus(w http.ResponseWriter, r *http.Request) {
	userID := userIDFromContext(r.Context())
	id := r.PathValue("id")

	var req updateStatusRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	next := models.Status(strings.TrimSpace(req.Status))
	if !next.IsValid() {
		writeError(w, http.StatusBadRequest, "status must be one of: open, in_progress, closed")
		return
	}

	ticket, err := s.store.UpdateTicketStatus(userID, id, next)
	switch {
	case errors.Is(err, store.ErrTicketNotFound):
		writeError(w, http.StatusNotFound, "ticket not found")
		return
	case errors.Is(err, store.ErrInvalidStatus):
		writeError(w, http.StatusBadRequest, "status must be one of: open, in_progress, closed")
		return
	case errors.Is(err, store.ErrInvalidTransition):
		writeError(w, http.StatusConflict, "invalid status transition: a ticket can only move forward (open -> in_progress -> closed) and a closed ticket cannot be reopened")
		return
	case err != nil:
		writeError(w, http.StatusInternalServerError, "could not update ticket")
		return
	}
	writeJSON(w, http.StatusOK, ticket)
}

// ---- helpers ----

// decodeJSON strictly decodes the request body into dst, writing a 400 on
// failure. It returns false if the caller should stop.
func decodeJSON(w http.ResponseWriter, r *http.Request, dst any) bool {
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return false
	}
	return true
}

func normaliseEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}
