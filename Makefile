.DEFAULT_GOAL := help

MODULE      := github.com/1SenaMetth/ledger
BINARY      := bin/api
VERSION     ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
COMMIT      ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo none)
LDFLAGS     := -s -w -X main.version=$(VERSION) -X main.commit=$(COMMIT)
DATABASE_URL ?= postgres://ledger:ledger@localhost:5432/ledger?sslmode=disable

.PHONY: help
help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-18s\033[0m %s\n", $$1, $$2}'

## --- development ---

.PHONY: up
up: ## Start Postgres and Redis
	docker compose up -d
	@echo "waiting for postgres..."
	@until docker compose exec -T postgres pg_isready -U ledger >/dev/null 2>&1; do sleep 1; done
	@echo "ready"

.PHONY: down
down: ## Stop containers
	docker compose down

.PHONY: reset
reset: ## Destroy containers and volumes, then start fresh
	docker compose down -v
	$(MAKE) up
	$(MAKE) migrate-up

.PHONY: run
run: ## Run the API locally
	go run -ldflags "$(LDFLAGS)" ./cmd/api

.PHONY: build
build: ## Compile the API binary
	CGO_ENABLED=0 go build -ldflags "$(LDFLAGS)" -o $(BINARY) ./cmd/api

## --- quality ---

.PHONY: test
test: ## Run tests
	go test ./... -count=1

.PHONY: test-race
test-race: ## Run tests with the race detector (use this one)
	go test ./... -race -count=1

.PHONY: cover
cover: ## Generate an HTML coverage report
	go test ./... -coverprofile=coverage.out -covermode=atomic
	go tool cover -html=coverage.out -o coverage.html
	@echo "open coverage.html"

.PHONY: lint
lint: ## Run golangci-lint
	golangci-lint run ./...

.PHONY: fmt
fmt: ## Format and tidy
	go fmt ./...
	go mod tidy

.PHONY: vuln
vuln: ## Scan dependencies for known vulnerabilities
	govulncheck ./...

## --- database ---

.PHONY: migrate-up
migrate-up: ## Apply all migrations
	migrate -path migrations -database "$(DATABASE_URL)" up

.PHONY: migrate-down
migrate-down: ## Roll back one migration
	migrate -path migrations -database "$(DATABASE_URL)" down 1

.PHONY: migrate-create
migrate-create: ## Create a migration: make migrate-create name=add_outbox
	@test -n "$(name)" || (echo "usage: make migrate-create name=<name>"; exit 1)
	migrate create -ext sql -dir migrations -seq $(name)

.PHONY: drift
drift: ## Check for accounts whose stored balance disagrees with their entries
	psql "$(DATABASE_URL)" -c "SELECT * FROM balance_drift;"

.PHONY: sqlc
sqlc: ## Generate type-safe Go from the SQL in internal/storage/queries
	sqlc generate

## --- tooling ---

.PHONY: tools
tools: ## Install the development tools this project expects
	go install github.com/golang-migrate/migrate/v4/cmd/migrate@latest
	go install github.com/sqlc-dev/sqlc/cmd/sqlc@latest
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	go install golang.org/x/vuln/cmd/govulncheck@latest
