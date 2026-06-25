package httpapi

import (
	"net/http"

	"github.com/example/ticket-system/internal/auth"
	"github.com/example/ticket-system/internal/store"
)

// Server wires the store and token manager into HTTP handlers.
type Server struct {
	store  store.Store
	tokens *auth.TokenManager
}

// NewServer constructs a Server.
func NewServer(st store.Store, tokens *auth.TokenManager) *Server {
	return &Server{store: st, tokens: tokens}
}

// Router returns the fully-configured HTTP handler. It uses the standard
// library's pattern-based ServeMux (Go 1.22+) so the service has no routing
// dependency.
func (s *Server) Router() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /health", s.handleHealth)
	mux.HandleFunc("POST /auth/register", s.handleRegister)
	mux.HandleFunc("POST /auth/login", s.handleLogin)

	mux.HandleFunc("POST /tickets", s.authMiddleware(s.handleCreateTicket))
	mux.HandleFunc("GET /tickets", s.authMiddleware(s.handleListTickets))
	mux.HandleFunc("GET /tickets/{id}", s.authMiddleware(s.handleGetTicket))
	mux.HandleFunc("PATCH /tickets/{id}/status", s.authMiddleware(s.handleUpdateStatus))

	return mux
}
