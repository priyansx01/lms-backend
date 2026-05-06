APP_NAME=smartfm-lms
BINARY_NAME=lms-api
CMD_DIR=./cmd/api

.PHONY: build run dev test lint clean migrate-up migrate-down sqlc

## ─── Build & Run ──────────────────────────────────────────────────────────────

build:
	@echo "▸ Building $(BINARY_NAME)..."
	go build -o bin/$(BINARY_NAME) $(CMD_DIR)

run: build
	@echo "▸ Running $(BINARY_NAME)..."
	./bin/$(BINARY_NAME)

dev:
	@echo "▸ Starting dev server with air..."
	@which air > /dev/null 2>&1 || (echo "Installing air..." && go install github.com/air-verse/air@latest)
	air -c .air.toml 2>/dev/null || air

clean:
	@echo "▸ Cleaning..."
	rm -rf bin/ tmp/

## ─── Testing ──────────────────────────────────────────────────────────────────

test:
	go test -race -cover ./...

lint:
	golangci-lint run ./...

## ─── Database ─────────────────────────────────────────────────────────────────

migrate-up:
	@echo "▸ Running migrations..."
	migrate -path db/migrations -database "$$DATABASE_URL" up

migrate-down:
	@echo "▸ Rolling back last migration..."
	migrate -path db/migrations -database "$$DATABASE_URL" down 1

## ─── Code Generation ──────────────────────────────────────────────────────────

sqlc:
	@echo "▸ Generating sqlc..."
	sqlc generate
