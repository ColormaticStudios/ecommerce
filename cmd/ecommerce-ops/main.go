package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"ecommerce/internal/backups"
	"ecommerce/internal/operability"
	"filippo.io/age"
)

func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, nil)))
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := run(ctx, os.Args[1:]); err != nil && !errors.Is(err, context.Canceled) {
		slog.Error("Ecommerce operations command failed", "error_type", fmt.Sprintf("%T", err), "error", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, arguments []string) error {
	if len(arguments) == 0 {
		return errors.New("usage: ecommerce-ops <backup|schedule|restore-drill|serve-metrics|generate-age-key|deploy-check|incident>")
	}
	switch arguments[0] {
	case "generate-age-key":
		return generateAgeKey(arguments[1:])
	case "deploy-check":
		return runDeployCheck(ctx, arguments[1:])
	case "incident":
		return runIncident(arguments[1:])
	}
	config, err := backups.LoadConfig()
	if err != nil {
		return err
	}
	store, err := backups.NewS3Store(ctx, config)
	if err != nil {
		return err
	}
	switch arguments[0] {
	case "backup":
		if err := config.ValidateBackup(); err != nil {
			return err
		}
		service, err := backupService(config, store, nil)
		if err != nil {
			return err
		}
		manifest, err := service.Backup(ctx)
		if err == nil {
			slog.Info("Encrypted logical backup completed", "run_id", manifest.RunID, "artifact_key", manifest.ArtifactKey, "artifact_size_bytes", manifest.ArtifactSizeBytes)
		}
		return err
	case "schedule":
		if err := config.ValidateBackup(); err != nil {
			return err
		}
		schedule, err := backups.ParseScheduleUTC(config.ScheduleUTC)
		if err != nil {
			return err
		}
		service, err := backupService(config, store, nil)
		if err != nil {
			return err
		}
		slog.Info("Backup scheduler started", "schedule_utc", config.ScheduleUTC)
		runBackup := func(runContext context.Context) error {
			manifest, backupErr := service.Backup(runContext)
			if backupErr != nil {
				slog.Error("Scheduled backup failed", "run_id", manifest.RunID, "failure_stage", manifest.FailureStage, "error_type", fmt.Sprintf("%T", backupErr))
				return nil
			}
			slog.Info("Scheduled backup completed", "run_id", manifest.RunID, "artifact_key", manifest.ArtifactKey, "artifact_size_bytes", manifest.ArtifactSizeBytes)
			return nil
		}
		if err := runBackup(ctx); err != nil {
			return err
		}
		return backups.RunDaily(ctx, schedule, nil, runBackup)
	case "restore-drill":
		if err := config.ValidateRestore(); err != nil {
			return err
		}
		verifier, err := backups.NewPostgresVerifier(config.RestoreDatabaseURL)
		if err != nil {
			return fmt.Errorf("connect restore target: %w", err)
		}
		defer verifier.Close()
		service, err := backupService(config, store, verifier)
		if err != nil {
			return err
		}
		manifest, err := service.RestoreDrill(ctx)
		if err == nil {
			slog.Info("Restore drill completed", "run_id", manifest.RunID, "rpo_seconds", manifest.RPOSeconds, "rto_seconds", manifest.RTOSeconds)
		}
		return err
	case "serve-metrics":
		return serveMetrics(ctx, config, store)
	default:
		return fmt.Errorf("unknown ecommerce-ops command %q", arguments[0])
	}
}

func runDeployCheck(ctx context.Context, arguments []string) error {
	if len(arguments) != 1 {
		return errors.New("usage: ecommerce-ops deploy-check <pre-deploy|post-deploy>")
	}
	config, err := operability.LoadGateConfig(arguments[0])
	if err != nil {
		return err
	}
	report, gateErr := (operability.GateRunner{}).Run(ctx, config)
	if err := json.NewEncoder(os.Stdout).Encode(report); err != nil {
		return errors.Join(gateErr, err)
	}
	return gateErr
}

func runIncident(arguments []string) error {
	if len(arguments) < 2 || (arguments[0] != "validate" && arguments[0] != "summary") {
		return errors.New("usage: ecommerce-ops incident <validate|summary> <record.yaml> [record.yaml ...]")
	}
	records := make([]operability.IncidentRecord, 0, len(arguments)-1)
	for _, filename := range arguments[1:] {
		file, err := os.Open(filename)
		if err != nil {
			return fmt.Errorf("open incident record %s: %w", filename, err)
		}
		record, decodeErr := operability.DecodeIncident(file)
		closeErr := file.Close()
		if decodeErr != nil {
			return fmt.Errorf("validate incident record %s: %w", filename, decodeErr)
		}
		if closeErr != nil {
			return closeErr
		}
		records = append(records, record)
	}
	if err := operability.ValidateIncidentSet(records); err != nil {
		return err
	}
	if arguments[0] == "validate" {
		return json.NewEncoder(os.Stdout).Encode(map[string]any{"schema_version": 1, "outcome": "succeeded", "record_count": len(records)})
	}
	summary, err := operability.SummarizeIncidents(records)
	if err != nil {
		return err
	}
	return json.NewEncoder(os.Stdout).Encode(summary)
}

func backupService(config backups.Config, store backups.Store, verifier backups.TargetVerifier) (*backups.Service, error) {
	tools := backups.PGTools{
		DatabaseURL: config.DatabaseURL, RestoreDatabaseURL: config.RestoreDatabaseURL,
		DumpPath: config.PGDumpPath, RestorePath: config.PGRestorePath,
	}
	return backups.NewService(backups.ServiceOptions{
		Store: store, Dumper: tools, Restorer: tools, Verifier: verifier,
		TemporaryDirectory: config.TemporaryDirectory, AgeRecipient: config.AgeRecipient,
		AgeIdentityFile: config.AgeIdentityFile, SourceDatabaseURL: config.DatabaseURL,
		RestoreDatabaseURL: config.RestoreDatabaseURL,
	})
}

func serveMetrics(ctx context.Context, config backups.Config, store backups.Store) error {
	mux := http.NewServeMux()
	mux.Handle("/metrics", backups.NewMetricsHandler(store, config.Service, config.Environment, config.Owner))
	mux.HandleFunc("/healthz", func(writer http.ResponseWriter, _ *http.Request) { writer.WriteHeader(http.StatusOK) })
	mux.HandleFunc("/readyz", func(writer http.ResponseWriter, request *http.Request) {
		checkContext, cancel := context.WithTimeout(request.Context(), 15*time.Second)
		defer cancel()
		if _, err := store.List(checkContext, "status/"); err != nil {
			writer.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		writer.WriteHeader(http.StatusOK)
	})
	server := &http.Server{
		Addr: config.MetricsBindAddress, Handler: mux,
		ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 20 * time.Second,
		WriteTimeout: 30 * time.Second, IdleTimeout: time.Minute,
	}
	errorsChannel := make(chan error, 1)
	go func() { errorsChannel <- server.ListenAndServe() }()
	slog.Info("Backup metrics server started", "bind_address", config.MetricsBindAddress)
	select {
	case err := <-errorsChannel:
		if !errors.Is(err, http.ErrServerClosed) {
			return err
		}
		return nil
	case <-ctx.Done():
		shutdownContext, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		return server.Shutdown(shutdownContext)
	}
}

func generateAgeKey(arguments []string) error {
	flags := flag.NewFlagSet("generate-age-key", flag.ContinueOnError)
	identityFile := flags.String("identity-file", "", "absolute destination for the secret age identity")
	if err := flags.Parse(arguments); err != nil {
		return err
	}
	if !filepath.IsAbs(*identityFile) {
		return errors.New("--identity-file must be an absolute path")
	}
	identity, err := age.GenerateX25519Identity()
	if err != nil {
		return err
	}
	file, err := os.OpenFile(*identityFile, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return fmt.Errorf("create identity file: %w", err)
	}
	if _, err := fmt.Fprintln(file, identity.String()); err != nil {
		_ = file.Close()
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	fmt.Println(identity.Recipient().String())
	return nil
}
