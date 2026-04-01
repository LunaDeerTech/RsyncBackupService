package model

import "time"

type StrategyStorageBinding struct {
	ID              uint `gorm:"primaryKey"`
	StrategyID      uint `gorm:"not null;uniqueIndex:idx_strategy_storage_binding_pair"`
	Strategy        Strategy
	StorageTargetID uint `gorm:"not null;uniqueIndex:idx_strategy_storage_binding_pair"`
	StorageTarget   StorageTarget
	CreatedAt       time.Time
}

func (StrategyStorageBinding) TableName() string {
	return "strategy_storage_bindings"
}
