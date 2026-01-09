.PHONY: all api cli run test clean release

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

# Clean build artifacts
clean:
	@rm -rf bin/

# Build release version
release:
	@echo "Building for release"
	@go build -o bin/ecommerce-api -ldflags="-s -w" main.go
	@go build -o bin/ecommerce-cli -ldflags="-s -w" ./cmd/cli/main.go