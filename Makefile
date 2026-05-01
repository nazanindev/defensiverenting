.PHONY: all build test lint vet fmt tidy up down ingest migrate migration-check clean

BINARY_SERVER = bin/server
BINARY_INGEST = bin/ingest
DATABASE_URL   ?= postgres://postgres:postgres@localhost:5432/tenants?sslmode=disable

all: tidy vet lint build test

## Build binaries
build:
	go build -o $(BINARY_SERVER) ./cmd/server
	go build -o $(BINARY_INGEST) ./cmd/ingest

## Run tests (requires Postgres at DATABASE_URL)
test:
	go test ./... -race -count=1

## Run tests with verbose output
test-v:
	go test ./... -race -count=1 -v

## Lint with golangci-lint
lint:
	golangci-lint run ./...

## Vet
vet:
	go vet ./...

## Format
fmt:
	gofmt -w -s .

## Tidy go.mod / go.sum
tidy:
	go mod tidy

## Start Postgres + server via Docker Compose
up:
	docker compose up -d db server

## Stop all containers
down:
	docker compose down

## Load content into the database (requires db running)
## Content repo location: make ingest CONTENT_DIR=/path/to/tenant-playbooks
CONTENT_DIR ?= ../tenant-playbooks
ingest:
	go run ./cmd/ingest -content $(CONTENT_DIR) -db "$(DATABASE_URL)"

## Check legacy HTML to markdown migration coverage
migration-check:
	python3 scripts/check_migration_coverage.py

## Apply migrations directly (requires Postgres at DATABASE_URL)
migrate:
	go run ./cmd/server -migrate-only

## Start server locally (requires Postgres at DATABASE_URL)
run:
	DATABASE_URL="$(DATABASE_URL)" go run ./cmd/server

clean:
	rm -rf bin/
