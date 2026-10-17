package jobs

import (
	"errors"

	"gorm.io/gorm"
)

const runtimeSetting = "ecommerce:jobs:runtime"

// WithRuntime explicitly composes the configured producer with a database
// handle. GORM context/transaction clones inherit statement settings; no
// global registry or default retry policy is used.
func WithRuntime(db *gorm.DB, runtime *Runtime) *gorm.DB {
	return db.Session(&gorm.Session{NewDB: true}).Set(runtimeSetting, runtime).Session(&gorm.Session{})
}

func RuntimeFromDB(db *gorm.DB) (*Runtime, error) {
	if db != nil {
		if value, ok := db.Get(runtimeSetting); ok {
			if runtime, ok := value.(*Runtime); ok && runtime != nil {
				return runtime, nil
			}
		}
	}
	return nil, errors.New("configured job runtime is required on database handle")
}
