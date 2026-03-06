.PHONY: all api cli run test test-services test-handlers test-integration check clean release openapi-gen openapi-check openapi-docs migrate migrate-plan migrate-check migrate-status migrate-lint migrate-guard migrate-snapshot migrate-drift-check migrate-ci-gate migrate-forward-compat test-migrations test-e2e-postgres test-e2e-sqlite

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

# Print ordered pending database migrations
migrate-plan:
	@go run ./cmd/migrate plan

# Ensure database is at latest migration version
migrate-check:
	@go run ./cmd/migrate check

# Print migration status summary (known/applied/pending)
migrate-status:
	@go run ./cmd/migrate status

# Lint migration definitions and conventions
migrate-lint:
	@go run ./cmd/migrate lint

# Validate readiness checks for pending contract migrations
migrate-guard:
	@go run ./cmd/migrate guard

# Generate canonical schema snapshot artifact from configured DB
migrate-snapshot:
	@go run ./cmd/migrate snapshot

# Verify current DB schema matches committed snapshot artifact
migrate-drift-check:
	@go run ./cmd/migrate drift-check

# Migration-focused backend tests
test-migrations:
	@GOCACHE=/tmp/go-build go test ./internal/migrations/... ./cmd/migrate/... ./cmd/cli/commands/...

# CI migration gate (tests + migrate check + replay + drift + forward-compat smoke)
migrate-ci-gate:
	@./scripts/ci-migration-gate.sh

# Forward-compat smoke: previous commit DB -> current binary startup
migrate-forward-compat:
	@./scripts/ci-forward-compat-smoke.sh

# Migration-sensitive E2E suite (required path for CI): Postgres only
test-e2e-postgres:
	@cd frontend && E2E_DB_DRIVER=postgres bun run test:e2e

# Optional local API-behavior smoke path: SQLite only
test-e2e-sqlite:
	@cd frontend && E2E_DB_DRIVER=sqlite bun run test:e2e

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
