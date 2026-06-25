package httpapi_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/example/ticket-system/internal/auth"
	"github.com/example/ticket-system/internal/httpapi"
	"github.com/example/ticket-system/internal/store"
)

// newTestServer builds a server backed by an in-memory store with deterministic
// sequential IDs.
func newTestServer() http.Handler {
	var counter int64
	st := store.NewMemoryStore(func() string {
		return fmt.Sprintf("id-%d", atomic.AddInt64(&counter, 1))
	})
	tokens := auth.NewTokenManager("test-secret", time.Hour)
	return httpapi.NewServer(st, tokens).Router()
}

// do is a small helper that performs a request and returns the recorder.
func do(t *testing.T, h http.Handler, method, path, token string, body any) *httptest.ResponseRecorder {
	t.Helper()
	var reader *bytes.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("marshal body: %v", err)
		}
		reader = bytes.NewReader(b)
	} else {
		reader = bytes.NewReader(nil)
	}
	req := httptest.NewRequest(method, path, reader)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

// register creates a user and returns its token.
func register(t *testing.T, h http.Handler, email, password string) string {
	t.Helper()
	rec := do(t, h, http.MethodPost, "/auth/register", "", map[string]string{
		"email":    email,
		"password": password,
	})
	if rec.Code != http.StatusCreated {
		t.Fatalf("register %s: got %d, body=%s", email, rec.Code, rec.Body.String())
	}
	var resp struct {
		Token string `json:"token"`
	}
	mustDecode(t, rec, &resp)
	if resp.Token == "" {
		t.Fatalf("register %s: empty token", email)
	}
	return resp.Token
}

func mustDecode(t *testing.T, rec *httptest.ResponseRecorder, dst any) {
	t.Helper()
	if err := json.Unmarshal(rec.Body.Bytes(), dst); err != nil {
		t.Fatalf("decode response %q: %v", rec.Body.String(), err)
	}
}

func TestHealth(t *testing.T) {
	h := newTestServer()
	rec := do(t, h, http.MethodGet, "/health", "", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("health: got %d", rec.Code)
	}
	var resp map[string]string
	mustDecode(t, rec, &resp)
	if resp["status"] != "ok" {
		t.Fatalf("health: got %v", resp)
	}
}

func TestRegisterValidation(t *testing.T) {
	h := newTestServer()

	cases := []struct {
		name string
		body map[string]string
		want int
	}{
		{"ok", map[string]string{"email": "a@b.com", "password": "secret1"}, http.StatusCreated},
		{"bad email", map[string]string{"email": "nope", "password": "secret1"}, http.StatusBadRequest},
		{"short password", map[string]string{"email": "c@d.com", "password": "x"}, http.StatusBadRequest},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec := do(t, h, http.MethodPost, "/auth/register", "", tc.body)
			if rec.Code != tc.want {
				t.Fatalf("got %d want %d (%s)", rec.Code, tc.want, rec.Body.String())
			}
		})
	}
}

func TestRegisterDuplicate(t *testing.T) {
	h := newTestServer()
	register(t, h, "dup@b.com", "secret1")
	rec := do(t, h, http.MethodPost, "/auth/register", "", map[string]string{
		"email": "DUP@b.com", "password": "secret1", // different case, same email
	})
	if rec.Code != http.StatusConflict {
		t.Fatalf("duplicate register: got %d want 409", rec.Code)
	}
}

func TestLogin(t *testing.T) {
	h := newTestServer()
	register(t, h, "login@b.com", "secret1")

	ok := do(t, h, http.MethodPost, "/auth/login", "", map[string]string{"email": "login@b.com", "password": "secret1"})
	if ok.Code != http.StatusOK {
		t.Fatalf("login ok: got %d", ok.Code)
	}

	bad := do(t, h, http.MethodPost, "/auth/login", "", map[string]string{"email": "login@b.com", "password": "wrong"})
	if bad.Code != http.StatusUnauthorized {
		t.Fatalf("login wrong password: got %d want 401", bad.Code)
	}
}

func TestProtectedRequiresToken(t *testing.T) {
	h := newTestServer()

	noToken := do(t, h, http.MethodGet, "/tickets", "", nil)
	if noToken.Code != http.StatusUnauthorized {
		t.Fatalf("no token: got %d want 401", noToken.Code)
	}

	badToken := do(t, h, http.MethodGet, "/tickets", "garbage", nil)
	if badToken.Code != http.StatusUnauthorized {
		t.Fatalf("bad token: got %d want 401", badToken.Code)
	}
}

func TestTicketLifecycle(t *testing.T) {
	h := newTestServer()
	token := register(t, h, "owner@b.com", "secret1")

	// Create
	create := do(t, h, http.MethodPost, "/tickets", token, map[string]string{"title": "fix login"})
	if create.Code != http.StatusCreated {
		t.Fatalf("create: got %d", create.Code)
	}
	var ticket struct {
		ID     string `json:"id"`
		Title  string `json:"title"`
		Status string `json:"status"`
	}
	mustDecode(t, create, &ticket)
	if ticket.Status != "open" || ticket.Title != "fix login" {
		t.Fatalf("create: unexpected ticket %+v", ticket)
	}

	// Get own
	get := do(t, h, http.MethodGet, "/tickets/"+ticket.ID, token, nil)
	if get.Code != http.StatusOK {
		t.Fatalf("get own: got %d", get.Code)
	}

	// List shows the ticket
	list := do(t, h, http.MethodGet, "/tickets", token, nil)
	if list.Code != http.StatusOK || !strings.Contains(list.Body.String(), ticket.ID) {
		t.Fatalf("list: got %d body=%s", list.Code, list.Body.String())
	}

	// open -> in_progress
	p1 := do(t, h, http.MethodPatch, "/tickets/"+ticket.ID+"/status", token, map[string]string{"status": "in_progress"})
	if p1.Code != http.StatusOK {
		t.Fatalf("open->in_progress: got %d", p1.Code)
	}
	// in_progress -> closed
	p2 := do(t, h, http.MethodPatch, "/tickets/"+ticket.ID+"/status", token, map[string]string{"status": "closed"})
	if p2.Code != http.StatusOK {
		t.Fatalf("in_progress->closed: got %d", p2.Code)
	}
	// closed -> open must be rejected
	p3 := do(t, h, http.MethodPatch, "/tickets/"+ticket.ID+"/status", token, map[string]string{"status": "open"})
	if p3.Code != http.StatusConflict {
		t.Fatalf("closed->open: got %d want 409", p3.Code)
	}
}

func TestInvalidStatusValue(t *testing.T) {
	h := newTestServer()
	token := register(t, h, "x@b.com", "secret1")
	create := do(t, h, http.MethodPost, "/tickets", token, map[string]string{"title": "t"})
	var ticket struct {
		ID string `json:"id"`
	}
	mustDecode(t, create, &ticket)

	rec := do(t, h, http.MethodPatch, "/tickets/"+ticket.ID+"/status", token, map[string]string{"status": "banana"})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("invalid status: got %d want 400", rec.Code)
	}
}

func TestOwnershipIsolation(t *testing.T) {
	h := newTestServer()
	alice := register(t, h, "alice@b.com", "secret1")
	bob := register(t, h, "bob@b.com", "secret1")

	// Alice creates a ticket.
	create := do(t, h, http.MethodPost, "/tickets", alice, map[string]string{"title": "alice ticket"})
	var ticket struct {
		ID string `json:"id"`
	}
	mustDecode(t, create, &ticket)

	// Bob cannot see it.
	get := do(t, h, http.MethodGet, "/tickets/"+ticket.ID, bob, nil)
	if get.Code != http.StatusNotFound {
		t.Fatalf("bob get alice ticket: got %d want 404", get.Code)
	}

	// Bob cannot update it.
	patch := do(t, h, http.MethodPatch, "/tickets/"+ticket.ID+"/status", bob, map[string]string{"status": "in_progress"})
	if patch.Code != http.StatusNotFound {
		t.Fatalf("bob patch alice ticket: got %d want 404", patch.Code)
	}

	// Bob's list is empty.
	list := do(t, h, http.MethodGet, "/tickets", bob, nil)
	if strings.Contains(list.Body.String(), ticket.ID) {
		t.Fatalf("bob list leaked alice ticket: %s", list.Body.String())
	}
}
