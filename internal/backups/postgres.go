package backups

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"os/exec"
	"strings"

	"ecommerce/internal/schemacontract"
	_ "github.com/jackc/pgx/v5/stdlib"
)

type Dumper interface {
	Dump(context.Context, io.Writer) error
}

type Restorer interface {
	Restore(context.Context, io.Reader) error
}

type PGTools struct {
	DatabaseURL        string
	RestoreDatabaseURL string
	DumpPath           string
	RestorePath        string
}

func (p PGTools) Dump(ctx context.Context, destination io.Writer) error {
	environment, err := databaseEnvironment(p.DatabaseURL)
	if err != nil {
		return fmt.Errorf("configure pg_dump connection: %w", err)
	}
	command := exec.CommandContext(ctx, p.DumpPath, "--format=custom", "--no-owner", "--no-acl")
	command.Env = environment
	command.Stdout = destination
	var stderr bytes.Buffer
	command.Stderr = &stderr
	if err := command.Run(); err != nil {
		return commandError("pg_dump", err, stderr.String())
	}
	return nil
}

func (p PGTools) Restore(ctx context.Context, source io.Reader) error {
	environment, err := databaseEnvironment(p.RestoreDatabaseURL)
	if err != nil {
		return fmt.Errorf("configure pg_restore connection: %w", err)
	}
	command := exec.CommandContext(ctx, p.RestorePath, "--exit-on-error", "--no-owner", "--no-acl", "--dbname=")
	command.Env = environment
	command.Stdin = source
	var stderr bytes.Buffer
	command.Stderr = &stderr
	if err := command.Run(); err != nil {
		return commandError("pg_restore", err, stderr.String())
	}
	return nil
}

func databaseEnvironment(databaseURL string) ([]string, error) {
	parsed, err := url.Parse(databaseURL)
	if err != nil || (parsed.Scheme != "postgres" && parsed.Scheme != "postgresql") || parsed.Hostname() == "" || strings.Trim(parsed.Path, "/") == "" {
		return nil, errors.New("invalid PostgreSQL connection URL")
	}
	values := map[string]string{
		"PGHOST":     parsed.Hostname(),
		"PGPORT":     parsed.Port(),
		"PGDATABASE": strings.TrimPrefix(parsed.Path, "/"),
	}
	if values["PGPORT"] == "" {
		values["PGPORT"] = "5432"
	}
	if parsed.User != nil {
		values["PGUSER"] = parsed.User.Username()
		if password, exists := parsed.User.Password(); exists {
			values["PGPASSWORD"] = password
		}
	}
	queryEnvironment := map[string]string{
		"application_name": "PGAPPNAME", "channel_binding": "PGCHANNELBINDING",
		"client_encoding": "PGCLIENTENCODING", "connect_timeout": "PGCONNECT_TIMEOUT",
		"gssencmode": "PGGSSENCMODE", "hostaddr": "PGHOSTADDR", "keepalives": "PGKEEPALIVES",
		"keepalives_count": "PGKEEPALIVESCOUNT", "keepalives_idle": "PGKEEPALIVESIDLE",
		"keepalives_interval": "PGKEEPALIVESINTERVAL", "load_balance_hosts": "PGLOADBALANCEHOSTS",
		"options": "PGOPTIONS", "passfile": "PGPASSFILE", "require_auth": "PGREQUIREAUTH",
		"service": "PGSERVICE", "servicefile": "PGSERVICEFILE", "sslcert": "PGSSLCERT",
		"sslcrl": "PGSSLCRL", "sslcrldir": "PGSSLCRLDIR", "sslkey": "PGSSLKEY",
		"sslmode": "PGSSLMODE", "sslnegotiation": "PGSSLNEGOTIATION", "sslpassword": "PGSSLPASSWORD",
		"sslrootcert": "PGSSLROOTCERT", "sslsni": "PGSSLSNI", "target_session_attrs": "PGTARGETSESSIONATTRS",
		"tcp_user_timeout": "PGTCPUSER_TIMEOUT",
	}
	for key, queryValues := range parsed.Query() {
		environmentKey, supported := queryEnvironment[key]
		if !supported || len(queryValues) != 1 {
			return nil, fmt.Errorf("unsupported or repeated PostgreSQL URL parameter %q", key)
		}
		values[environmentKey] = queryValues[0]
	}

	replacedKeys := map[string]struct{}{
		"PGHOST": {}, "PGPORT": {}, "PGDATABASE": {}, "PGUSER": {}, "PGPASSWORD": {},
	}
	for _, environmentKey := range queryEnvironment {
		replacedKeys[environmentKey] = struct{}{}
	}
	environment := make([]string, 0, len(os.Environ())+len(values))
	for _, value := range os.Environ() {
		key, _, _ := strings.Cut(value, "=")
		if _, replaced := replacedKeys[key]; !replaced {
			environment = append(environment, value)
		}
	}
	for key, value := range values {
		environment = append(environment, key+"="+value)
	}
	return environment, nil
}

func commandError(tool string, err error, stderr string) error {
	stderr = strings.TrimSpace(stderr)
	if len(stderr) > 1024 {
		stderr = stderr[:1024]
	}
	if stderr == "" {
		return fmt.Errorf("%s failed: %w", tool, err)
	}
	return fmt.Errorf("%s failed: %w: %s", tool, err, stderr)
}

type TargetVerifier interface {
	EnsureEmpty(context.Context) error
	Verify(context.Context) error
	Close() error
}

type PostgresVerifier struct {
	sql *sql.DB
}

func NewPostgresVerifier(databaseURL string) (*PostgresVerifier, error) {
	sqlDB, err := sql.Open("pgx", databaseURL)
	if err != nil {
		return nil, err
	}
	return &PostgresVerifier{sql: sqlDB}, nil
}

func (v *PostgresVerifier) EnsureEmpty(ctx context.Context) error {
	var count int
	err := v.sql.QueryRowContext(ctx, `
SELECT COUNT(*)
FROM pg_class c
JOIN pg_namespace n ON n.oid = c.relnamespace
WHERE c.relkind IN ('r', 'p')
  AND n.nspname NOT IN ('pg_catalog', 'information_schema')
  AND n.nspname NOT LIKE 'pg_toast%'
`).Scan(&count)
	if err != nil {
		return fmt.Errorf("inspect restore target: %w", err)
	}
	if count != 0 {
		return fmt.Errorf("restore target is not empty: found %d user tables", count)
	}
	return nil
}

func (v *PostgresVerifier) Verify(ctx context.Context) error {
	var latestMigrationPresent bool
	if err := v.sql.QueryRowContext(ctx, "SELECT EXISTS (SELECT 1 FROM schema_migrations WHERE version = $1)", schemacontract.LatestMigrationVersion).Scan(&latestMigrationPresent); err != nil {
		return fmt.Errorf("verify migration state: %w", err)
	}
	if !latestMigrationPresent {
		return fmt.Errorf("verify migration state: latest migration %s is absent", schemacontract.LatestMigrationVersion)
	}
	for _, table := range []string{"users", "products", "orders", "job_queue"} {
		var exists bool
		if err := v.sql.QueryRowContext(ctx, "SELECT to_regclass($1) IS NOT NULL", "public."+table).Scan(&exists); err != nil {
			return fmt.Errorf("verify table %s: %w", table, err)
		}
		if !exists {
			return fmt.Errorf("verify table %s: table is missing", table)
		}
	}
	tx, err := v.sql.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin read/write verification: %w", err)
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, "CREATE TEMP TABLE ecommerce_restore_probe (value integer NOT NULL)"); err != nil {
		return fmt.Errorf("create read/write probe: %w", err)
	}
	if _, err := tx.ExecContext(ctx, "INSERT INTO ecommerce_restore_probe (value) VALUES (1)"); err != nil {
		return fmt.Errorf("write restore probe: %w", err)
	}
	var value int
	if err := tx.QueryRowContext(ctx, "SELECT value FROM ecommerce_restore_probe").Scan(&value); err != nil || value != 1 {
		return errors.New("read/write restore probe failed")
	}
	return nil
}

func (v *PostgresVerifier) Close() error { return v.sql.Close() }
