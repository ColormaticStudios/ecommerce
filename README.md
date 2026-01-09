# Ecommerce API Backend

A RESTful ecommerce API backend built with Go, Gin, GORM, and PostgreSQL.

> [!WARNING]
> This project was partially vibe-coded. I have not reviewed all of the code, I cannot guarantee quality, use at your own risk.

See [frontend/README.md](frontend/README.md) for details about the frontend.

## Features

- **Authentication & Authorization**
  - JWT-based authentication
  - User registration and login
  - Role-based access control (admin/customer)
  - OpenID Connect support

- **Product Management**
  - Product CRUD operations (admin)
  - Product search and filtering
  - Pagination support
  - Price range filtering
  - Sorting (by price, name, date)
  - Stock/inventory management

- **Shopping Cart**
  - Add items to cart
  - Update cart item quantities
  - Remove items from cart
  - View cart

- **Order Management**
  - Create orders
  - View order history
  - Order status tracking (PENDING, PAID, FAILED)
  - Mock payment processing
  - Admin order management

- **User Management**
  - User profiles
  - Profile updates
  - Admin user management

## Prerequisites

- Go 1.21 or higher
- PostgreSQL 12 or higher
- Make (optional, for using Makefile commands)

## Setup

1. **Clone the repository**
   ```bash
   git clone <repository-url>
   cd ecommerce
   ```

2. **Install dependencies**
   ```bash
   go mod download
   ```

3. **Set up environment variables**
   ```bash
   cp .env.example .env
   # Edit .env with your configuration
   ```

4. **Start the server**
   ```bash
   go run main.go
   ```

   Or build and run:
   ```bash
   go build -o ecommerce-api
   ./ecommerce-api
   ```

## Environment Variables

See `.env.example` for all required environment variables.

- `DATABASE_URL`: PostgreSQL connection string
- `PORT`: Server port (default: 3000)
- `JWT_SECRET`: Secret key for JWT token signing
- `DISABLE_LOCAL_SIGN_IN`: Disable local sign-in (default: false)
- `OIDC_PROVIDER`: OIDC provider URL (optional)
- `OIDC_CLIENT_ID`: OIDC client ID (optional)
- `OIDC_CLIENT_SECRET`: OIDC client secret (optional)
- `OIDC_REDIRECT_URI`: OIDC redirect URI (optional)

## API Endpoints

See [API.md](API.md) for documentation on the API.

## CLI Tool

The project includes a command-line tool for administrative tasks.

### Building the CLI

```bash
make build-cli
# or
go build -o bin/ecommerce-cli ./cmd/cli
```

### CLI Commands

#### User Management

**Set a user as admin:**
```bash
./bin/ecommerce-cli user set-admin --email user@example.com
# or by username
./bin/ecommerce-cli user set-admin --username johndoe
```

**Create a new user:**
```bash
./bin/ecommerce-cli user create \
  --email admin@example.com \
  --username admin \
  --password securepassword \
  --name "Admin User" \
  --role admin
```

**List all users:**
```bash
./bin/ecommerce-cli user list
# Filter by role
./bin/ecommerce-cli user list --role admin
```

**Delete a user:**
```bash
./bin/ecommerce-cli user delete --email user@example.com
# Skip confirmation
./bin/ecommerce-cli user delete --email user@example.com --yes
```

#### Product Management

**Create a product:**
```bash
./bin/ecommerce-cli product create \
  --sku PROD-001 \
  --name "Product Name" \
  --description "Product description" \
  --price 29.99 \
  --stock 100
```

**List products:**
```bash
./bin/ecommerce-cli product list
# Limit results
./bin/ecommerce-cli product list --limit 10
```

**Delete a product:**
```bash
./bin/ecommerce-cli product delete --id 1
# or by SKU
./bin/ecommerce-cli product delete --sku PROD-001
```

### CLI Help

Get help for any command:
```bash
./bin/ecommerce-cli --help
./bin/ecommerce-cli user --help
./bin/ecommerce-cli user set-admin --help
```

## Database Models

- **User**: User accounts with authentication
- **Product**: Product catalog with inventory
- **Cart**: Shopping cart for users
- **CartItem**: Items in shopping cart
- **Order**: Customer orders
- **OrderItem**: Items in orders

## Security Features

- JWT-based authentication
- Password hashing with bcrypt
- Role-based access control
- CORS configuration
- Rate limiting (100 requests/second)
- Input validation

## Development

### Running Tests

```bash
# Run all tests
go test ./...

# Run tests with verbose output
go test -v ./...

# Run tests for a specific package
go test ./handlers
go test ./middleware

# Run tests with coverage
go test -cover ./...
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out
```

The test suite includes:
- **Authentication tests** - JWT generation, password hashing, subject ID generation
- **Validation tests** - Currency validation, product validation, input validation
- **Business logic tests** - Order total calculation, stock validation, pagination
- **Middleware tests** - Auth middleware with various scenarios (valid/invalid tokens, role requirements, expired tokens)

### Building for Production

**API Server:**
```bash
go build -o ecommerce-api -ldflags="-s -w" main.go
```

**CLI Tool:**
```bash
go build -o ecommerce-cli -ldflags="-s -w" ./cmd/cli
```

### Using Makefile

The project includes a Makefile for common tasks:

```bash
make build        # Build API server
make build-cli    # Build CLI tool
make run          # Build and run API server
make test         # Run tests
make clean        # Remove build artifacts
make install-cli  # Install CLI to system PATH
make help         # Show all available commands
```

### Database Migrations

Migrations are handled automatically by GORM on server start.
