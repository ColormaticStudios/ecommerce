package backups

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"path"
	"strings"
)

func latestSuccessfulManifest(ctx context.Context, store Store, operation string) (Manifest, error) {
	values, err := statusManifests(ctx, store)
	if err != nil {
		return Manifest{}, err
	}
	return latestSuccessfulFromStatus(values, operation)
}

func latestSuccessfulFromStatus(values map[string]Manifest, operation string) (Manifest, error) {
	key := "status/latest-successful-backup.json"
	if operation == OperationRestoreDrill {
		key = "status/latest-successful-restore-drill.json"
	}
	latest, exists := values[key]
	if !exists || latest.Operation != operation || latest.Outcome != OutcomeSucceeded {
		return Manifest{}, fmt.Errorf("no successful %s manifest found", operation)
	}
	return latest, nil
}

func latestFromStatus(values map[string]Manifest, operation string) (Manifest, bool) {
	key := "status/latest-backup.json"
	if operation == OperationRestoreDrill {
		key = "status/latest-restore-drill.json"
	}
	latest, exists := values[key]
	return latest, exists
}

func statusManifests(ctx context.Context, store Store) (map[string]Manifest, error) {
	keys, err := store.List(ctx, "status/")
	if err != nil {
		return nil, err
	}
	result := make(map[string]Manifest, len(keys))
	for _, key := range keys {
		if path.Dir(key) != "status" || !strings.HasSuffix(key, ".json") {
			continue
		}
		value, err := store.GetBytes(ctx, key)
		if err != nil {
			return nil, fmt.Errorf("read status manifest %s: %w", key, err)
		}
		var manifest Manifest
		if err := json.Unmarshal(value, &manifest); err != nil {
			return nil, fmt.Errorf("decode status manifest %s: %w", key, err)
		}
		if manifest.SchemaVersion != ManifestSchemaVersion {
			return nil, fmt.Errorf("status manifest %s uses unsupported schema version %d", key, manifest.SchemaVersion)
		}
		if err := validateStatusManifest(manifest); err != nil {
			return nil, fmt.Errorf("validate status manifest %s: %w", key, err)
		}
		result[key] = manifest
	}
	return result, nil
}

func validateStatusManifest(manifest Manifest) error {
	allowedStages := map[string]map[string]struct{}{
		OperationBackup: {
			"configuration": {}, "temporary_file": {}, "encryption": {}, "pg_dump": {},
			"checksum": {}, "artifact_upload": {}, "manifest_upload": {},
		},
		OperationRestoreDrill: {
			"configuration": {}, "isolation_guard": {}, "backup_selection": {}, "temporary_file": {},
			"artifact_download": {}, "checksum": {}, "decryption": {}, "pg_restore": {}, "verification": {},
		},
	}
	stages, operationAllowed := allowedStages[manifest.Operation]
	if !operationAllowed {
		return fmt.Errorf("unsupported operation %q", manifest.Operation)
	}
	if manifest.RunID == "" || manifest.CompletedAt.IsZero() {
		return errors.New("run ID and completion time are required")
	}
	switch manifest.Outcome {
	case OutcomeSucceeded:
		if manifest.FailureStage != "" {
			return errors.New("successful manifest must not have a failure stage")
		}
	case OutcomeFailed:
		if _, allowed := stages[manifest.FailureStage]; !allowed {
			return fmt.Errorf("unsupported failure stage %q", manifest.FailureStage)
		}
	default:
		return fmt.Errorf("unsupported outcome %q", manifest.Outcome)
	}
	return nil
}
