package repository

import (
	"testing"

	"github.com/LunaDeerTech/RsyncBackupService/internal/config"
	"github.com/LunaDeerTech/RsyncBackupService/internal/model"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func TestMigrateAndSeedMigratesCoreTables(t *testing.T) {
	db, err := OpenSQLite(t.TempDir())
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}

	if err := MigrateAndSeed(db, config.Config{AdminUser: "admin", AdminPassword: "secret"}); err != nil {
		t.Fatalf("migrate and seed: %v", err)
	}

	requiredTables := []string{
		"users",
		"ssh_keys",
		"backup_instances",
		"storage_targets",
		"strategies",
		"strategy_storage_bindings",
		"backup_records",
		"restore_records",
		"notification_channels",
		"notification_subscriptions",
		"instance_permissions",
		"audit_logs",
	}

	for _, table := range requiredTables {
		if !db.Migrator().HasTable(table) {
			t.Fatalf("expected table %s to exist after migration", table)
		}
	}
}

func TestMigrateAndSeedCreatesAdmin(t *testing.T) {
	db, err := OpenSQLite(t.TempDir())
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}

	cfg := config.Config{AdminUser: "admin", AdminPassword: "secret"}
	if err := MigrateAndSeed(db, cfg); err != nil {
		t.Fatalf("migrate and seed: %v", err)
	}

	var user struct {
		Username     string
		PasswordHash string
		IsAdmin      bool
	}
	if err := db.Table("users").Where("username = ?", cfg.AdminUser).Take(&user).Error; err != nil {
		t.Fatalf("expected seeded admin user: %v", err)
	}
	if !user.IsAdmin {
		t.Fatal("expected first seeded user to be admin")
	}
	if user.PasswordHash == cfg.AdminPassword {
		t.Fatal("expected stored password to be hashed")
	}
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(cfg.AdminPassword)); err != nil {
		t.Fatalf("expected password hash to match seeded password: %v", err)
	}
}

func TestMigrateAndSeedDoesNotDuplicateAdmin(t *testing.T) {
	db, err := OpenSQLite(t.TempDir())
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}

	firstCfg := config.Config{AdminUser: "admin", AdminPassword: "secret"}
	if err := MigrateAndSeed(db, firstCfg); err != nil {
		t.Fatalf("first migrate and seed: %v", err)
	}

	secondCfg := config.Config{AdminUser: "other-admin", AdminPassword: "other-secret"}
	if err := MigrateAndSeed(db, secondCfg); err != nil {
		t.Fatalf("second migrate and seed: %v", err)
	}

	var userCount int64
	if err := db.Table("users").Count(&userCount).Error; err != nil {
		t.Fatalf("count users: %v", err)
	}
	if userCount != 1 {
		t.Fatalf("expected one seeded user after repeated bootstrap, got %d", userCount)
	}

	var user struct {
		Username string
		IsAdmin  bool
	}
	if err := db.Table("users").Take(&user).Error; err != nil {
		t.Fatalf("query seeded user: %v", err)
	}
	if user.Username != firstCfg.AdminUser {
		t.Fatalf("expected original admin username %q to remain, got %q", firstCfg.AdminUser, user.Username)
	}
	if !user.IsAdmin {
		t.Fatal("expected seeded user to remain admin")
	}
}

func TestMigrateAndSeedConstrainsStrategyStorageBindings(t *testing.T) {
	db, err := OpenSQLite(t.TempDir())
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}

	cfg := config.Config{AdminUser: "admin", AdminPassword: "secret"}
	if err := MigrateAndSeed(db, cfg); err != nil {
		t.Fatalf("migrate and seed: %v", err)
	}

	var admin struct {
		ID uint
	}
	if err := db.Table("users").Select("id").Where("username = ?", cfg.AdminUser).Take(&admin).Error; err != nil {
		t.Fatalf("query admin id: %v", err)
	}

	instance := model.BackupInstance{
		Name:            "instance-a",
		SourceType:      "local",
		SourcePath:      "/srv/source",
		ExcludePatterns: "[]",
		Enabled:         true,
		CreatedBy:       admin.ID,
	}
	if err := db.Create(&instance).Error; err != nil {
		t.Fatalf("create backup instance: %v", err)
	}

	strategy := model.Strategy{
		InstanceID:      instance.ID,
		Name:            "hourly",
		BackupType:      "rolling",
		IntervalSeconds: 3600,
		RetentionDays:   7,
		RetentionCount:  24,
		Enabled:         true,
	}
	if err := db.Create(&strategy).Error; err != nil {
		t.Fatalf("create strategy: %v", err)
	}

	target := model.StorageTarget{
		Name:     "target-a",
		Type:     "rolling_local",
		BasePath: "/srv/backup",
	}
	if err := db.Create(&target).Error; err != nil {
		t.Fatalf("create storage target: %v", err)
	}

	binding := model.StrategyStorageBinding{StrategyID: strategy.ID, StorageTargetID: target.ID}
	if err := db.Create(&binding).Error; err != nil {
		t.Fatalf("create valid strategy storage binding: %v", err)
	}

	silentDB := db.Session(&gorm.Session{Logger: logger.Default.LogMode(logger.Silent)})

	duplicateBinding := model.StrategyStorageBinding{StrategyID: strategy.ID, StorageTargetID: target.ID}
	if err := silentDB.Create(&duplicateBinding).Error; err == nil {
		t.Fatal("expected duplicate strategy storage binding to be rejected")
	}

	orphanBinding := model.StrategyStorageBinding{StrategyID: strategy.ID + 999, StorageTargetID: target.ID}
	if err := silentDB.Create(&orphanBinding).Error; err == nil {
		t.Fatal("expected orphan strategy storage binding to be rejected")
	}
}
