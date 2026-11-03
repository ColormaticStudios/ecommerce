package models

import "time"

// SearchIndexIncident records one continuous stale index episode. The partial
// unique index on active episodes is created by the migration, not AutoMigrate.
type SearchIndexIncident struct {
	ID             uint       `gorm:"primaryKey"`
	IndexName      string     `gorm:"not null;size:64;index"`
	Reason         string     `gorm:"not null;size:32"`
	OpenedAt       time.Time  `gorm:"not null;index"`
	DetectedAt     time.Time  `gorm:"not null"`
	LastObservedAt time.Time  `gorm:"not null"`
	RecoveredAt    *time.Time `gorm:"index"`
	MaxLagSeconds  int64      `gorm:"not null;default:0"`
	MaxPendingJobs int64      `gorm:"not null;default:0"`
	DocumentCount  int64      `gorm:"not null;default:0"`
}

// SearchFreshnessObservation preserves missing-baseline grace across restarts
// and observers on multiple API replicas.
type SearchFreshnessObservation struct {
	IndexName            string `gorm:"primaryKey;size:64"`
	MissingBaselineSince *time.Time
	LastObservedAt       time.Time `gorm:"not null"`
}
