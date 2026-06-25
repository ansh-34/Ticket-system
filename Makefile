.PHONY: run build test vet docker-build docker-run tidy

run:
	go run ./cmd/server

build:
	go build -o bin/server ./cmd/server

test:
	go test ./...

vet:
	go vet ./...

tidy:
	go mod tidy

docker-build:
	docker build -t ticket-system .

docker-run:
	docker run -p 8080:8080 -e JWT_SECRET=$${JWT_SECRET:-dev-secret} ticket-system
