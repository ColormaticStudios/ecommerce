// Package backups owns encrypted logical backup, restore-drill, and backup
// monitoring workflows for the separately deployed operations workload.
package backups

import (
	"errors"
	"fmt"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	DatabaseURL        string
	RestoreDatabaseURL string
	AgeRecipient       string
	AgeIdentityFile    string
	Bucket             string
	Prefix             string
	Region             string
	Endpoint           string
	ForcePathStyle     bool
	TemporaryDirectory string
	PGDumpPath         string
	PGRestorePath      string
	MetricsBindAddress string
	ScheduleUTC        string
	Service            string
	Environment        string
	Owner              string
	ConfirmIsolated    bool
}

func LoadConfig() (Config, error) {
	forcePathStyle, err := envBool("BACKUP_S3_FORCE_PATH_STYLE", false)
	if err != nil {
		return Config{}, err
	}
	confirmIsolated, err := envBool("RESTORE_DRILL_CONFIRM_ISOLATED", false)
	if err != nil {
		return Config{}, err
	}
	config := Config{
		DatabaseURL:        firstNonEmpty(os.Getenv("BACKUP_DATABASE_URL"), os.Getenv("DATABASE_URL")),
		RestoreDatabaseURL: os.Getenv("RESTORE_DATABASE_URL"),
		AgeRecipient:       os.Getenv("BACKUP_AGE_RECIPIENT"),
		AgeIdentityFile:    os.Getenv("BACKUP_AGE_IDENTITY_FILE"),
		Bucket:             os.Getenv("BACKUP_S3_BUCKET"),
		Prefix:             envDefault("BACKUP_S3_PREFIX", "ecommerce/backups"),
		Region:             envDefault("BACKUP_S3_REGION", "us-east-1"),
		Endpoint:           os.Getenv("BACKUP_S3_ENDPOINT"),
		ForcePathStyle:     forcePathStyle,
		TemporaryDirectory: envDefault("BACKUP_TEMP_DIR", os.TempDir()),
		PGDumpPath:         envDefault("BACKUP_PG_DUMP_PATH", "pg_dump"),
		PGRestorePath:      envDefault("BACKUP_PG_RESTORE_PATH", "pg_restore"),
		MetricsBindAddress: envDefault("BACKUP_METRICS_BIND_ADDRESS", "127.0.0.1:9091"),
		ScheduleUTC:        envDefault("BACKUP_SCHEDULE_UTC", "02:00"),
		Service:            envDefault("BACKUP_SERVICE", "ecommerce-backup"),
		Environment:        envDefault("DEPLOYMENT_ENVIRONMENT", "development"),
		Owner:              envDefault("ALERT_OWNER", "unassigned"),
		ConfirmIsolated:    confirmIsolated,
	}
	if err := config.ValidateBase(); err != nil {
		return Config{}, err
	}
	return config, nil
}

func (c Config) ValidateBase() error {
	if strings.TrimSpace(c.Bucket) == "" {
		return errors.New("BACKUP_S3_BUCKET is required")
	}
	prefix := strings.Trim(c.Prefix, "/")
	if prefix == "" {
		return errors.New("BACKUP_S3_PREFIX must be a non-empty object-key prefix")
	}
	for _, segment := range strings.Split(prefix, "/") {
		if segment == "" || segment == "." || segment == ".." {
			return errors.New("BACKUP_S3_PREFIX must not contain empty, '.' or '..' segments")
		}
	}
	if strings.TrimSpace(c.Region) == "" {
		return errors.New("BACKUP_S3_REGION is required")
	}
	if c.Endpoint != "" {
		endpoint, err := url.Parse(c.Endpoint)
		if err != nil || endpoint.Host == "" || (endpoint.Scheme != "http" && endpoint.Scheme != "https") {
			return errors.New("BACKUP_S3_ENDPOINT must be an absolute HTTP(S) URL")
		}
	}
	if !filepath.IsAbs(c.TemporaryDirectory) {
		return errors.New("BACKUP_TEMP_DIR must be an absolute path")
	}
	if _, _, err := net.SplitHostPort(c.MetricsBindAddress); err != nil {
		return fmt.Errorf("BACKUP_METRICS_BIND_ADDRESS must be a valid host:port: %w", err)
	}
	if _, err := ParseScheduleUTC(c.ScheduleUTC); err != nil {
		return err
	}
	for key, value := range map[string]string{"BACKUP_SERVICE": c.Service, "DEPLOYMENT_ENVIRONMENT": c.Environment, "ALERT_OWNER": c.Owner} {
		if strings.TrimSpace(value) == "" {
			return fmt.Errorf("%s must not be empty", key)
		}
	}
	return nil
}

func (c Config) ValidateBackup() error {
	if strings.TrimSpace(c.DatabaseURL) == "" {
		return errors.New("BACKUP_DATABASE_URL or DATABASE_URL is required")
	}
	if strings.TrimSpace(c.AgeRecipient) == "" {
		return errors.New("BACKUP_AGE_RECIPIENT is required")
	}
	return validatePostgresURL("BACKUP_DATABASE_URL", c.DatabaseURL)
}

func (c Config) ValidateRestore() error {
	if strings.TrimSpace(c.RestoreDatabaseURL) == "" {
		return errors.New("RESTORE_DATABASE_URL is required")
	}
	if strings.TrimSpace(c.AgeIdentityFile) == "" {
		return errors.New("BACKUP_AGE_IDENTITY_FILE is required")
	}
	if !filepath.IsAbs(c.AgeIdentityFile) {
		return errors.New("BACKUP_AGE_IDENTITY_FILE must be an absolute path")
	}
	if !c.ConfirmIsolated {
		return errors.New("RESTORE_DRILL_CONFIRM_ISOLATED=true is required")
	}
	return validatePostgresURL("RESTORE_DATABASE_URL", c.RestoreDatabaseURL)
}

func validatePostgresURL(key, value string) error {
	parsed, err := url.Parse(value)
	if err != nil || parsed.Host == "" || (parsed.Scheme != "postgres" && parsed.Scheme != "postgresql") || strings.Trim(parsed.Path, "/") == "" {
		return fmt.Errorf("%s must be an absolute PostgreSQL URL with a database name", key)
	}
	return nil
}

func ParseScheduleUTC(value string) (time.Duration, error) {
	parsed, err := time.Parse("15:04", value)
	if err != nil {
		return 0, errors.New("BACKUP_SCHEDULE_UTC must use 24-hour HH:MM format")
	}
	return time.Duration(parsed.Hour())*time.Hour + time.Duration(parsed.Minute())*time.Minute, nil
}

func envBool(key string, fallback bool) (bool, error) {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback, nil
	}
	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return false, fmt.Errorf("%s must be true or false", key)
	}
	return parsed, nil
}

func envDefault(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}
