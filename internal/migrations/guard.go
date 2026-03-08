package migrations

import (
	"errors"
	"fmt"

	"gorm.io/gorm"
)

func Guard(db *gorm.DB) error {
	pending, err := Pending(db)
	if err != nil {
		return err
	}
	return guardPendingMigrations(db, pending, true)
}

func GuardLines(db *gorm.DB) ([]string, error) {
	pending, err := Pending(db)
	if err != nil {
		return nil, err
	}

	contractPending := 0
	for _, migration := range pending {
		if hasTag(migration.Tags, migrationContractTag) {
			contractPending++
		}
	}

	if err := guardPendingMigrations(db, pending, true); err != nil {
		return []string{
			fmt.Sprintf("pending_contract_count=%d", contractPending),
			"guard_status=failed",
			fmt.Sprintf("guard_error=%s", err.Error()),
		}, err
	}

	return []string{
		fmt.Sprintf("pending_contract_count=%d", contractPending),
		"guard_status=ok",
	}, nil
}

func guardPendingMigrations(db *gorm.DB, pending []Migration, enforceBlockers bool) error {
	if !enforceBlockers {
		return nil
	}
	for _, migration := range pending {
		if !hasTag(migration.Tags, migrationContractTag) && len(migration.ContractBlockers) == 0 {
			continue
		}
		for _, blocker := range migration.ContractBlockers {
			if err := runContractBlocker(db, blocker); err != nil {
				return fmt.Errorf("guard check %q failed for migration %s (%s): %w", blocker, migration.Version, migration.Name, err)
			}
		}
	}
	return nil
}

func runContractBlocker(_ *gorm.DB, blocker string) error {
	switch blocker {
	case "allow_contract_migrations":
		if !allowContractMigrations() {
			return fmt.Errorf("set %s=true to acknowledge contract migration readiness", contractGuardEnvVar)
		}
		return nil
	default:
		return errors.New("unknown contract blocker")
	}
}
