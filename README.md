# Ticket System — Backend Intern Assignment

A small REST backend in **Go** for a ticket system. Users can register, log in,
create tickets, view **only their own** tickets, and move a ticket forward
through its lifecycle. Authentication is JWT-based and every ticket operation is
ownership-checked.

- **Language:** Go 1.25 (standard-library HTTP router, two small deps for JWT + bcrypt)
- **Port:** `8080`
- **Storage:** in-memory, concurrency-safe (no external database to provision)
- **Auth:** JWT (`Authorization: Bearer <token>`), passwords stored as bcrypt hashes

> **Deployed URL:** `https://<your-app>.onrender.com`
> **Public health check:** `https://<your-app>.onrender.com/health`
>
> _(Fill these in after deploying — see [Deployment](#deployment). The repo ships
> with a `render.yaml` blueprint so deployment is a few clicks.)_

---

## Quick start (local, no Docker)

```bash
go run ./cmd/server
# server listens on :8080
curl http://localhost:8080/health
# {"status":"ok"}
```

## Run with Docker

```bash
docker build -t ticket-system .
docker run -p 8080:8080 ticket-system
curl http://localhost:8080/health
# {"status":"ok"}
```

To set a real secret (recommended):

```bash
docker run -p 8080:8080 -e JWT_SECRET="$(openssl rand -hex 32)" ticket-system
```

## Run the tests

```bash
go test ./...
```

The suite covers health, registration/validation, login, JWT enforcement,
the full ticket lifecycle, the closed-cannot-reopen rule, invalid status values,
and cross-user ownership isolation.

---

## Configuration

All configuration is via environment variables (see [`.env.example`](.env.example)).
The service runs with zero configuration locally using safe defaults.

| Variable        | Default                         | Description                                  |
| --------------- | ------------------------------- | -------------------------------------------- |
| `PORT`          | `8080`                          | HTTP listen port                             |
| `JWT_SECRET`    | `dev-insecure-secret-change-me` | HMAC signing secret — **set this in prod**   |
| `JWT_TTL_HOURS` | `24`                            | Token lifetime in hours                      |

If `JWT_SECRET` is left at its default, the server logs a startup warning.

---

## API

All request and response bodies are JSON. Protected endpoints require
`Authorization: Bearer <token>`.

| Method  | Endpoint                 | Auth | Purpose                       |
| ------- | ------------------------ | :--: | ----------------------------- |
| `GET`   | `/health`                |  No  | Health check                  |
| `POST`  | `/auth/register`         |  No  | Register a user, returns JWT  |
| `POST`  | `/auth/login`            |  No  | Log in, returns JWT           |
| `POST`  | `/tickets`               | Yes  | Create a ticket               |
| `GET`   | `/tickets`               | Yes  | List the caller's tickets     |
| `GET`   | `/tickets/{id}`          | Yes  | Get one of the caller's tickets |
| `PATCH` | `/tickets/{id}/status`   | Yes  | Update a ticket's status      |

### Ticket status lifecycle

```
open ──▶ in_progress ──▶ closed
   └───────────────▶ closed        (forward shortcut allowed)

closed ──✗──▶ open / in_progress   (a closed ticket can never be reopened)
```

Only **forward** transitions are permitted. Any backward move — including
reopening a closed ticket — is rejected with `409 Conflict`.

### Examples

```bash
# Register (returns a token)
curl -s -X POST http://localhost:8080/auth/register \
  -H 'Content-Type: application/json' \
  -d '{"email":"alice@example.com","password":"secret1"}'
# 201 {"token":"<jwt>"}

# Login
curl -s -X POST http://localhost:8080/auth/login \
  -H 'Content-Type: application/json' \
  -d '{"email":"alice@example.com","password":"secret1"}'
# 200 {"token":"<jwt>"}

TOKEN=<jwt>

# Create a ticket
curl -s -X POST http://localhost:8080/tickets \
  -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' \
  -d '{"title":"Fix the login page"}'
# 201 {"id":"...","title":"Fix the login page","status":"open","owner_id":"...","created_at":"...","updated_at":"..."}

# List your tickets
curl -s http://localhost:8080/tickets -H "Authorization: Bearer $TOKEN"
# 200 [ { ... } ]

# Get one ticket
curl -s http://localhost:8080/tickets/<id> -H "Authorization: Bearer $TOKEN"

# Advance status
curl -s -X PATCH http://localhost:8080/tickets/<id>/status \
  -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' \
  -d '{"status":"in_progress"}'
```

### Status codes

| Situation                                            | Code              |
| ---------------------------------------------------- | ----------------- |
| Success (GET/PATCH/login)                            | `200 OK`          |
| Resource created (register, create ticket)           | `201 Created`     |
| Invalid body / bad email / weak password / bad status| `400 Bad Request` |
| Missing or invalid token                             | `401 Unauthorized`|
| Wrong email or password                              | `401 Unauthorized`|
| Email already registered                             | `409 Conflict`    |
| Invalid status transition (e.g. reopening closed)    | `409 Conflict`    |
| Ticket not found **or not owned by caller**          | `404 Not Found`   |

Errors use a uniform envelope: `{"error":"<message>"}`.

---

## Project structure

```
cmd/server/          program entrypoint, config wiring, graceful shutdown
internal/config/     environment-based configuration
internal/models/     domain types + the status state machine
internal/store/      Store interface + concurrency-safe in-memory implementation
internal/auth/       bcrypt password hashing + JWT issue/verify
internal/httpapi/    router, auth middleware, handlers, JSON helpers
```

The `store.Store` interface keeps persistence behind a boundary, so a SQL
implementation can be dropped in later without touching the HTTP layer.

---

## Deployment

The repo includes a **`render.yaml`** blueprint for [Render](https://render.com)
(free tier, deploys straight from the `Dockerfile`):

1. Push this repo to GitHub.
2. In Render: **New +** → **Blueprint** → select the repo → **Apply**.
   Render reads `render.yaml`, builds the Docker image, generates a random
   `JWT_SECRET`, and uses `/health` as the health check.
3. After the build, your service is live at `https://<your-app>.onrender.com`
   and `https://<your-app>.onrender.com/health` is public.

> Any free Docker host works equally well (Fly.io, Railway, Koyeb, Google Cloud
> Run). The image is a tiny distroless static binary, so it runs anywhere that
> can run a container and expose port `8080`.

---

## Assumptions

- **In-memory storage** is used (explicitly allowed by the brief). Data resets on
  restart; on a free host that spins down when idle, tickets do not persist
  across a cold start. Swapping in a database means implementing `store.Store`
  only.
- **Register returns a token** directly, so a freshly registered user is
  immediately authenticated without a second login round-trip.
- **`open → closed` is allowed** as a forward shortcut. The only hard rule from
  the brief is that a **closed ticket can never move back**; all backward moves
  are rejected. Same-status updates are also rejected as no-op transitions.
- **404 (not 403) for another user's ticket** so the API never reveals whether a
  ticket ID exists for a different owner.
- **Generic auth errors** ("invalid email or password") avoid leaking which
  emails are registered.
- Email is normalised (trimmed + lower-cased) and must contain `@`; passwords
  must be at least 6 characters.
```
