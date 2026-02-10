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
