# Ecommerce

Self-hostable ecommerce platform with a Go backend (Gin + GORM + PostgreSQL), a SvelteKit frontend, and a CLI for admin workflows.

## Documentation

Project documentation is maintained in the wiki:

- https://git.colormatic.org/ColormaticStudios/ecommerce/wiki

API reference is generated in:

- [API.md](API.md)

Frontend-specific docs are in:

- [frontend/README.md](frontend/README.md)

## Features

- Authentication and authorization (local auth + OIDC)
- Product catalog and admin product management
- Cart, guest checkout, and order workflows
- Storefront configuration and draft/preview publishing
- Media upload/processing pipeline
- Runtime-extensible checkout providers (payment, shipping, tax)
- CLI for user and product administration

## Quick Start

1. Configure environment:

```bash
cp .env.example .env
# Edit .env and fill in variables
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

Note: the storefront placeholders that the AI generated are cringe but funny, so I left them in. The example products are certified artisan humanslop.

## Build and Test

Build binaries:

```bash
make all
# or
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

Database migrations:

```bash
make migrate
make migrate-plan
make migrate-check
make migrate-status
make migrate-lint
make migrate-guard
make migrate-snapshot
make migrate-drift-check
make migrate-forward-compat
make migrate-ci-gate
```

Note: by default, API/CLI startup checks that the database is already at the latest migration and fails if not. To auto-apply pending migrations on startup, set `AUTO_APPLY_MIGRATIONS=true`.

Migration-sensitive E2E policy:

```bash
# Required CI path (Postgres)
E2E_DB_URL='postgres://...' make test-e2e-postgres

# Optional local smoke path (SQLite only, non-authoritative for migration parity)
make test-e2e-sqlite
```

## License

Licensed under the MIT License. See `LICENSE`.
