// Command server starts the ticket-system HTTP API.
package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/example/ticket-system/internal/auth"
	"github.com/example/ticket-system/internal/config"
	"github.com/example/ticket-system/internal/httpapi"
	"github.com/example/ticket-system/internal/store"
)

func main() {
	cfg := config.Load()
	if cfg.UsingDefaultSecret() {
		log.Println("WARNING: JWT_SECRET is not set; using an insecure development secret. Set JWT_SECRET in production.")
	}

	st := store.NewMemoryStore(newID)
	tokens := auth.NewTokenManager(cfg.JWTSecret, cfg.JWTTTL)
	srv := httpapi.NewServer(st, tokens)

	httpServer := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           srv.Router(),
		ReadHeaderTimeout: 10 * time.Second,
	}

	// Run the server in a goroutine so main can wait for shutdown signals.
	go func() {
		log.Printf("ticket-system listening on :%s", cfg.Port)
		if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("server error: %v", err)
		}
	}()

	// Wait for SIGINT/SIGTERM, then shut down gracefully.
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop

	log.Println("shutting down...")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := httpServer.Shutdown(ctx); err != nil {
		log.Printf("graceful shutdown failed: %v", err)
	}
}

// newID returns a random 128-bit hex identifier.
func newID() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		// crypto/rand should never fail; fall back to a timestamp-based value.
		return hex.EncodeToString([]byte(time.Now().UTC().Format(time.RFC3339Nano)))
	}
	return hex.EncodeToString(b)
}
