package model

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

const ForceResetDBEnv = "BANANA_FORCE_RESET_DB"

func Open(dbPath string) (*gorm.DB, error) {
	if dbPath == "" {
		return nil, errors.New("db path required")
	}

	dir := filepath.Dir(dbPath)
	if dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, err
		}
	}

	db, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{})
	if err != nil {
		return nil, err
	}

	sqlDB, err := db.DB()
	if err != nil {
		return nil, err
	}

	sqlDB.SetMaxOpenConns(1)

	if err := db.Exec("PRAGMA journal_mode=WAL;").Error; err != nil {
		return nil, err
	}

	return db, nil
}

func ensureModelIDColumn(db *gorm.DB) error {
	migrator := db.Migrator()
	if !migrator.HasTable(&PromptNode{}) {
		return nil
	}
	if migrator.HasColumn(&PromptNode{}, "model_id") {
		return nil
	}
	return db.Exec("ALTER TABLE prompt_nodes ADD COLUMN model_id TEXT NOT NULL DEFAULT ''").Error
}

func Migrate(db *gorm.DB) error {
	if err := beforeResultsCutoverHook(db); err != nil {
		return err
	}

	if err := ensureModelIDColumn(db); err != nil {
		return err
	}

	if err := db.AutoMigrate(&PromptNode{}, &PromptSnippet{}, &Asset{}, &Result{}, &Image{}); err != nil {
		return err
	}

	return db.Exec("UPDATE prompt_nodes SET model_id = 'gemini-3-pro-image' WHERE model_id = '' OR model_id IS NULL").Error
}

func IsTruthyFlag(raw string) bool {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "1", "true", "yes", "y", "on":
		return true
	default:
		return false
	}
}

func EnsureLegacySchemaGuard(db *gorm.DB, forceReset bool) error {
	legacy, reason := isLegacySchema(db)
	if !legacy {
		return nil
	}
	if forceReset {
		if err := resetLegacySchema(db); err != nil {
			return fmt.Errorf("legacy schema force reset failed: %w", err)
		}
		return nil
	}

	return fmt.Errorf("检测到旧数据库 schema（%s），为避免误覆盖已阻止启动；如确认重建新结构，请设置 %s=1 后重启", reason, ForceResetDBEnv)
}

func isLegacySchema(db *gorm.DB) (bool, string) {
	migrator := db.Migrator()
	hasPromptNodes := migrator.HasTable(&PromptNode{})
	hasAssets := migrator.HasTable(&Asset{})
	hasResults := migrator.HasTable(&Result{})
	hasImages := migrator.HasTable(&Image{})
	hasSessions := migrator.HasTable("sessions")
	hasNodes := migrator.HasTable("nodes")

	if hasPromptNodes && hasAssets && hasResults {
		return false, ""
	}
	if !hasPromptNodes && !hasAssets && !hasResults && !hasImages && !hasSessions && !hasNodes {
		return false, ""
	}
	if hasSessions || hasNodes {
		legacyTables := make([]string, 0, 2)
		if hasSessions {
			legacyTables = append(legacyTables, "sessions")
		}
		if hasNodes {
			legacyTables = append(legacyTables, "nodes")
		}
		return true, fmt.Sprintf("检测到旧表 %s", strings.Join(legacyTables, ", "))
	}
	if !hasPromptNodes {
		return true, "缺少新表 prompt_nodes"
	}

	missingTables := make([]string, 0, 3)
	if !hasPromptNodes {
		missingTables = append(missingTables, "prompt_nodes")
	}
	if !hasAssets {
		missingTables = append(missingTables, "assets")
	}
	if !hasResults {
		missingTables = append(missingTables, "results")
	}
	if !hasImages {
		missingTables = append(missingTables, "images")
	}

	return true, fmt.Sprintf("缺少新表 %s", strings.Join(missingTables, ", "))
}

func resetLegacySchema(db *gorm.DB) error {
	migrator := db.Migrator()
	tables := []string{"sessions", "nodes", "images", "results", "assets", "prompt_nodes"}
	for _, table := range tables {
		if !migrator.HasTable(table) {
			continue
		}
		if err := migrator.DropTable(table); err != nil {
			return fmt.Errorf("drop table %s: %w", table, err)
		}
	}
	return nil
}

var beforeResultsCutoverHook = func(db *gorm.DB) error {
	return nil
}
