package model

import (
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	"gorm.io/gorm"
)

func TestPromptNodeFragmentsRoundTrip(t *testing.T) {
	dbPath := t.TempDir() + "/fragments.sqlite"
	db, err := Open(dbPath)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := Migrate(db); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	tests := []struct {
		name      string
		fragments PromptCollectionState
	}{
		{name: "nil", fragments: nil},
		{name: "empty", fragments: PromptCollectionState{}},
		{name: "non-empty", fragments: PromptCollectionState{
			"groupOrder": []any{"g1"},
			"groups": map[string]any{
				"g1": map[string]any{
					"id":        "g1",
					"name":      "默认分组",
					"promptIds": []any{"p1"},
					"collapsed": false,
				},
			},
			"prompts": map[string]any{
				"p1": map[string]any{
					"id":        "p1",
					"content":   "hello",
					"createdAt": 1730000000.0,
					"updatedAt": 1730000001.0,
				},
			},
			"unknown_meta": map[string]any{"x": 1.0},
		}},
	}

	for i, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			node := PromptNode{
				ID:         "node-fragments-" + tc.name,
				Mode:       PromptModeTxt2Img,
				Prompt:     "p-" + tc.name,
				Title:      "title-" + tc.name,
				LastUsedAt: time.Now().Add(time.Duration(i) * time.Minute),
				Fragments:  tc.fragments,
			}
			if err := db.Create(&node).Error; err != nil {
				t.Fatalf("create node: %v", err)
			}

			var got PromptNode
			if err := db.First(&got, "id = ?", node.ID).Error; err != nil {
				t.Fatalf("query node: %v", err)
			}

			if !reflect.DeepEqual(got.Fragments, tc.fragments) {
				t.Fatalf("fragments mismatch: got=%#v want=%#v", got.Fragments, tc.fragments)
			}
		})
	}
}

func TestPromptSnippetCRUD(t *testing.T) {
	dbPath := t.TempDir() + "/snippet.sqlite"
	db, err := Open(dbPath)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := Migrate(db); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	snippet := PromptSnippet{
		ID:               "snippet-1",
		Name:             "style/cinematic",
		ContentFragments: PromptFragments{{"type": "text", "text": "ultra detailed"}, {"type": "text", "text": "cinematic lighting", "weight": 0.6}},
	}
	if err := db.Create(&snippet).Error; err != nil {
		t.Fatalf("create snippet: %v", err)
	}

	var created PromptSnippet
	if err := db.First(&created, "id = ?", snippet.ID).Error; err != nil {
		t.Fatalf("query snippet: %v", err)
	}
	if created.Name != snippet.Name || !reflect.DeepEqual(created.ContentFragments, snippet.ContentFragments) {
		t.Fatalf("snippet mismatch after create: got=%+v want=%+v", created, snippet)
	}

	if err := db.Model(&PromptSnippet{}).
		Where("id = ?", snippet.ID).
		Updates(map[string]any{"name": "style/noir", "content_fragments": PromptFragments{{"type": "text", "text": "film noir"}, {"type": "text", "text": "high contrast", "custom": true}}}).Error; err != nil {
		t.Fatalf("update snippet: %v", err)
	}

	var updated PromptSnippet
	if err := db.First(&updated, "id = ?", snippet.ID).Error; err != nil {
		t.Fatalf("query updated snippet: %v", err)
	}
	if updated.Name != "style/noir" || !reflect.DeepEqual(updated.ContentFragments, PromptFragments{{"type": "text", "text": "film noir"}, {"type": "text", "text": "high contrast", "custom": true}}) {
		t.Fatalf("snippet mismatch after update: got=%+v", updated)
	}

	if err := db.Delete(&PromptSnippet{}, "id = ?", snippet.ID).Error; err != nil {
		t.Fatalf("delete snippet: %v", err)
	}

	var deleted PromptSnippet
	err = db.First(&deleted, "id = ?", snippet.ID).Error
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("expected record not found after delete, got: %v", err)
	}
}

func TestSchema(t *testing.T) {
	dbPath := t.TempDir() + "/schema.sqlite"

	db, err := Open(dbPath)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}

	if err := Migrate(db); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	migrator := db.Migrator()
	if migrator.HasTable("sessions") {
		t.Fatalf("legacy table should not exist: sessions")
	}
	if migrator.HasTable("nodes") {
		t.Fatalf("legacy table should not exist: nodes")
	}
	if !migrator.HasTable(&PromptNode{}) {
		t.Fatalf("missing table: prompt_nodes")
	}
	if !migrator.HasTable(&PromptSnippet{}) {
		t.Fatalf("missing table: prompt_snippets")
	}
	if !migrator.HasTable(&Image{}) {
		t.Fatalf("missing table: images")
	}
	if !migrator.HasTable(&Asset{}) {
		t.Fatalf("missing table: assets")
	}
	if !migrator.HasTable(&Result{}) {
		t.Fatalf("missing table: results")
	}
}

func TestLegacySchemaGuardBlocksWithoutForceFlag(t *testing.T) {
	db := mustOpenLegacySchemaDB(t)

	err := EnsureLegacySchemaGuard(db, false)
	if err == nil {
		t.Fatalf("expected guard error for legacy schema")
	}
	if got := err.Error(); got == "" {
		t.Fatalf("expected non-empty guard error")
	}
	if want := ForceResetDBEnv + "=1"; !strings.Contains(err.Error(), want) {
		t.Fatalf("expected error to contain %q, got: %q", want, err.Error())
	}
}

func TestLegacySchemaGuardAllowsWithForceFlag(t *testing.T) {
	db := mustOpenLegacySchemaDB(t)

	if err := EnsureLegacySchemaGuard(db, true); err != nil {
		t.Fatalf("expected guard bypass when force flag enabled, got: %v", err)
	}

	migrator := db.Migrator()
	if migrator.HasTable("sessions") {
		t.Fatalf("expected sessions dropped after force reset")
	}
	if migrator.HasTable("nodes") {
		t.Fatalf("expected nodes dropped after force reset")
	}
	if migrator.HasTable("images") {
		t.Fatalf("expected images dropped after force reset")
	}

	if err := Migrate(db); err != nil {
		t.Fatalf("migrate after force reset: %v", err)
	}
	if !migrator.HasTable(&PromptNode{}) {
		t.Fatalf("missing table after force reset migrate: prompt_nodes")
	}
}

func TestLegacySchemaGuardAllowsFreshDatabase(t *testing.T) {
	dbPath := t.TempDir() + "/fresh.sqlite"
	db, err := Open(dbPath)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}

	if err := EnsureLegacySchemaGuard(db, false); err != nil {
		t.Fatalf("expected fresh db to pass guard, got: %v", err)
	}
}

func TestIsTruthyFlag(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  bool
	}{
		{name: "one", input: "1", want: true},
		{name: "true mixed case", input: "TrUe", want: true},
		{name: "yes", input: "yes", want: true},
		{name: "on", input: "on", want: true},
		{name: "empty", input: "", want: false},
		{name: "zero", input: "0", want: false},
		{name: "false", input: "false", want: false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := IsTruthyFlag(tc.input); got != tc.want {
				t.Fatalf("IsTruthyFlag(%q)=%v, want %v", tc.input, got, tc.want)
			}
		})
	}
}

func TestMigrate_emptyDB_addsModelIDColumn(t *testing.T) {
	dbPath := t.TempDir() + "/modelid_empty.sqlite"
	db, err := Open(dbPath)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := Migrate(db); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	if !db.Migrator().HasColumn(&PromptNode{}, "model_id") {
		t.Fatal("expected model_id column to exist after Migrate")
	}
}

func TestMigrate_backfillLegacyRows(t *testing.T) {
	dbPath := t.TempDir() + "/modelid_backfill.sqlite"
	db, err := Open(dbPath)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := Migrate(db); err != nil {
		t.Fatalf("first migrate: %v", err)
	}

	// Insert a row with empty model_id via raw SQL.
	if err := db.Exec("INSERT INTO prompt_nodes (id, mode, prompt, title, last_used_at, pinned, model_id) VALUES (?, ?, ?, ?, datetime('now'), 0, '')",
		"backfill-1", PromptModeTxt2Img, "test prompt", "test title").Error; err != nil {
		t.Fatalf("insert row: %v", err)
	}

	if err := Migrate(db); err != nil {
		t.Fatalf("second migrate: %v", err)
	}

	var node PromptNode
	if err := db.First(&node, "id = ?", "backfill-1").Error; err != nil {
		t.Fatalf("query node: %v", err)
	}
	if node.ModelID != "gemini-3-pro-image" {
		t.Fatalf("expected model_id='gemini-3-pro-image', got %q", node.ModelID)
	}
}

func TestMigrate_isIdempotent(t *testing.T) {
	dbPath := t.TempDir() + "/modelid_idempotent.sqlite"
	db, err := Open(dbPath)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}

	for i := 0; i < 3; i++ {
		if err := Migrate(db); err != nil {
			t.Fatalf("migrate run %d: %v", i+1, err)
		}
	}

	if !db.Migrator().HasColumn(&PromptNode{}, "model_id") {
		t.Fatal("expected model_id column to exist after 3 Migrate calls")
	}

	// Insert a row and run once more; verify model_id stays stable.
	if err := db.Exec("INSERT INTO prompt_nodes (id, mode, prompt, title, last_used_at, pinned, model_id) VALUES (?, ?, ?, ?, datetime('now'), 0, 'gemini-3-pro-image')",
		"idem-1", PromptModeTxt2Img, "prompt", "title").Error; err != nil {
		t.Fatalf("insert row: %v", err)
	}
	if err := Migrate(db); err != nil {
		t.Fatalf("migrate run 4: %v", err)
	}

	var node PromptNode
	if err := db.First(&node, "id = ?", "idem-1").Error; err != nil {
		t.Fatalf("query node: %v", err)
	}
	if node.ModelID != "gemini-3-pro-image" {
		t.Fatalf("expected model_id unchanged, got %q", node.ModelID)
	}
}

func TestMigrate_altersPopulatedTableWithNotNullDefault(t *testing.T) {
	dbPath := t.TempDir() + "/modelid_alter.sqlite"
	db, err := Open(dbPath)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}

	// Create pre-migration schema WITHOUT model_id column.
	createSQL := `CREATE TABLE prompt_nodes (
		id TEXT PRIMARY KEY,
		parent_id TEXT,
		mode TEXT NOT NULL,
		prompt TEXT NOT NULL,
		fragments TEXT,
		title TEXT NOT NULL,
		pinned INTEGER NOT NULL DEFAULT 0,
		sort_order INTEGER,
		last_used_at DATETIME NOT NULL,
		archived_at DATETIME,
		negative_prompt TEXT,
		cfg_scale REAL,
		aspect_ratio TEXT,
		image_size TEXT,
		seed INTEGER,
		steps INTEGER,
		reference_image_ids TEXT,
		created_at DATETIME,
		updated_at DATETIME,
		deleted_at DATETIME
	)`
	if err := db.Exec(createSQL).Error; err != nil {
		t.Fatalf("create pre-migration table: %v", err)
	}

	// Insert a row with all required NOT NULL fields.
	if err := db.Exec("INSERT INTO prompt_nodes (id, mode, prompt, title, last_used_at, pinned) VALUES (?, ?, ?, ?, datetime('now'), 0)",
		"alter-1", PromptModeTxt2Img, "test prompt", "test title").Error; err != nil {
		t.Fatalf("insert pre-migration row: %v", err)
	}

	if err := Migrate(db); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	if !db.Migrator().HasColumn(&PromptNode{}, "model_id") {
		t.Fatal("expected model_id column to exist after Migrate")
	}

	var node PromptNode
	if err := db.First(&node, "id = ?", "alter-1").Error; err != nil {
		t.Fatalf("query node: %v", err)
	}
	if node.ModelID != "gemini-3-pro-image" {
		t.Fatalf("expected model_id='gemini-3-pro-image', got %q", node.ModelID)
	}
}

func TestEnsureLegacySchemaGuard_modelIDColumnMissing_isNotLegacy(t *testing.T) {
	dbPath := t.TempDir() + "/modelid_guard.sqlite"
	db, err := Open(dbPath)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}

	// Create all current tables but without model_id column in prompt_nodes.
	createPromptNodesSQL := `CREATE TABLE prompt_nodes (
		id TEXT PRIMARY KEY,
		parent_id TEXT,
		mode TEXT NOT NULL,
		prompt TEXT NOT NULL,
		fragments TEXT,
		title TEXT NOT NULL,
		pinned INTEGER NOT NULL DEFAULT 0,
		sort_order INTEGER,
		last_used_at DATETIME NOT NULL,
		archived_at DATETIME,
		negative_prompt TEXT,
		cfg_scale REAL,
		aspect_ratio TEXT,
		image_size TEXT,
		seed INTEGER,
		steps INTEGER,
		reference_image_ids TEXT,
		created_at DATETIME,
		updated_at DATETIME,
		deleted_at DATETIME
	)`
	if err := db.Exec(createPromptNodesSQL).Error; err != nil {
		t.Fatalf("create prompt_nodes: %v", err)
	}
	if err := db.Exec("CREATE TABLE assets (id TEXT PRIMARY KEY, domain TEXT NOT NULL, origin TEXT NOT NULL, file_path TEXT NOT NULL, created_at DATETIME, updated_at DATETIME)").Error; err != nil {
		t.Fatalf("create assets: %v", err)
	}
	if err := db.Exec("CREATE TABLE results (id TEXT PRIMARY KEY, node_id TEXT NOT NULL, asset_id TEXT NOT NULL, status TEXT NOT NULL DEFAULT 'active', created_at DATETIME, updated_at DATETIME)").Error; err != nil {
		t.Fatalf("create results: %v", err)
	}

	// Guard should NOT trigger — presence of prompt_nodes+assets+results is the non-legacy signal.
	if err := EnsureLegacySchemaGuard(db, false); err != nil {
		t.Fatalf("expected guard to pass when tables exist but model_id column is missing, got: %v", err)
	}
}

func mustOpenLegacySchemaDB(t *testing.T) *gorm.DB {
	t.Helper()

	dbPath := t.TempDir() + "/legacy.sqlite"
	db, err := Open(dbPath)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}

	legacySchemaSQL := []string{
		"CREATE TABLE sessions (id TEXT PRIMARY KEY, title TEXT, mode TEXT)",
		"CREATE TABLE nodes (id TEXT PRIMARY KEY, prompt TEXT)",
		"CREATE TABLE images (id TEXT PRIMARY KEY, node_id TEXT, origin TEXT, file_path TEXT)",
	}
	for _, stmt := range legacySchemaSQL {
		if err := db.Exec(stmt).Error; err != nil {
			t.Fatalf("create legacy table: %v", err)
		}
	}

	return db
}
