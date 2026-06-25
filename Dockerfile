# ---- Build stage ----
FROM golang:1.25-alpine AS build

WORKDIR /src

# Cache module downloads.
COPY go.mod go.sum ./
RUN go mod download

# Build a static binary.
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/server ./cmd/server

# ---- Runtime stage ----
FROM gcr.io/distroless/static:nonroot

# Distroless static image: tiny, no shell, runs as non-root by default.
COPY --from=build /out/server /server

EXPOSE 8080
ENV PORT=8080

USER nonroot:nonroot
ENTRYPOINT ["/server"]
