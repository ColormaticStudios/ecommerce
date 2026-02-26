# Ecommerce

Self-hostable ecommerce platform with a Go backend (Gin + GORM + PostgreSQL), a SvelteKit frontend, and a CLI for admin workflows.

## Documentation

Project documentation is maintained in the wiki:

- https://git.colormatic.org/ColormaticStudios/ecommerce/wiki

API reference is generated in:

- `API.md`

Frontend-specific docs are in:

- `frontend/README.md`

## Features

- Authentication and authorization (local auth + OIDC)
- Product catalog and admin product management
- Cart and order workflows
- Storefront configuration and draft/preview publishing
- Media upload/processing pipeline
- Runtime-extensible checkout providers (payment, shipping, tax)
- CLI for user and product administration

## Quick Start

1. Configure environment:

```bash
cp .env.example .env
```

2. Start PostgreSQL for development:

```bash
sudo scripts/run-dev-db-docker.sh
# or
scripts/run-dev-db-podman.sh
```

3. Run the API:

```bash
make run
```

4. (Optional) seed sample data:

```bash
scripts/populate-test-database.sh
```

5. Run frontend:

```bash
cd frontend
bun install
bun run dev
```

## Build and Test

Build binaries:

```bash
make api
make cli
```

Run backend tests:

```bash
make test
```

Run frontend checks:

```bash
cd frontend
bun run check
bun run lint
```

## License

Licensed under the MIT License. See `LICENSE`.
