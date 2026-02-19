package models

import "time"

const StorefrontSettingsSingletonID uint = 1

type StorefrontSettings struct {
	ID         uint      `json:"id" gorm:"primaryKey;autoIncrement:false"`
	ConfigJSON string    `json:"config_json" gorm:"type:jsonb;not null"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}
