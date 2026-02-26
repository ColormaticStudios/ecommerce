.PHONY: all api cli run test test-services test-handlers test-integration check clean release openapi-gen openapi-check openapi-docs migrate migrate-check

# Build the API server and the CLI tool
all: api cli

# Build the API server
api:
	@echo "Building API server..."
	@go build -o bin/ecommerce-api main.go

# Build the CLI tool
cli:
	@echo "Building CLI tool..."
	@go build -o bin/ecommerce-cli ./cmd/cli

# Run the API server
run: api
	@./bin/ecommerce-api

# Run tests
test:
	@go test ./...
	@cd frontend && bun run test:e2e

test-services:
	@GOCACHE=/tmp/go-build go test ./internal/services/...

test-handlers:
	@GOCACHE=/tmp/go-build go test ./handlers

test-integration:
	@GOCACHE=/tmp/go-build go test ./handlers -run Integration

check: openapi-check
	@GOCACHE=/tmp/go-build go test ./internal/services/...
	@GOCACHE=/tmp/go-build go test ./handlers

# Apply database migrations
migrate:
	@go run ./cmd/migrate

# Ensure database is at latest migration version
migrate-check:
	@go run ./cmd/migrate check

# Generate backend + frontend API contract types from OpenAPI
openapi-gen:
	@./scripts/generate-api-contracts.sh

# Ensure generated contract files are up to date
openapi-check:
	@./scripts/generate-api-contracts.sh
	@if [ -n "$$(git status --porcelain -- internal/apicontract/openapi.gen.go frontend/src/lib/api/generated/openapi.ts)" ]; then \
		echo "Generated API contract files are out of date."; \
		git --no-pager diff -- internal/apicontract/openapi.gen.go frontend/src/lib/api/generated/openapi.ts; \
		exit 1; \
	fi

# Generate API documentation from OpenAPI
openapi-docs:
	@./scripts/generate-api-docs.sh

# Clean build artifacts
clean:
	@rm -rf bin/

# Build release version
release:
	@echo "Building for release"
	@go build -o bin/ecommerce-api -ldflags="-s -w" main.go
	@go build -o bin/ecommerce-cli -ldflags="-s -w" ./cmd/cli/main.go
