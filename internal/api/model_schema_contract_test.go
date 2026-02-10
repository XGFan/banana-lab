package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"banana-lab/internal/model"
)

func setupNodesRouter(t *testing.T) (*gin.Engine, *model.Node, *model.Asset, *model.Result) {
	t.Helper()

	gin.SetMode(gin.TestMode)
	dbPath := filepath.Join(t.TempDir(), "nodes.db")
	db, err := model.Open(dbPath)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := model.Migrate(db); err != nil {
		t.Fatalf("migrate db: %v", err)
	}

	now := mustParseRFC3339(t, "2026-02-08T00:00:00Z")
	node := model.PromptNode{ID: "node-contract", Mode: model.PromptModeTxt2Img, Prompt: "p", Title: "p", LastUsedAt: now}
	if err := db.Create(&node).Error; err != nil {
		t.Fatalf("create node: %v", err)
	}
	asset := model.Asset{ID: "asset-contract", Domain: model.AssetDomainTxt2Img, FilePath: "images/node-contract/a.png", Origin: model.AssetOriginGenerated}
	if err := db.Create(&asset).Error; err != nil {
		t.Fatalf("create asset: %v", err)
	}
	result := model.Result{ID: "result-contract", NodeID: node.ID, AssetID: asset.ID}
	if err := db.Create(&result).Error; err != nil {
		t.Fatalf("create result: %v", err)
	}

	image := model.Image{ID: "image-contract", NodeID: &node.ID, Origin: "generated", FilePath: asset.FilePath}
	if err := db.Create(&image).Error; err != nil {
		t.Fatalf("create image: %v", err)
	}

	router := gin.New()
	router.GET("/api/nodes/:id", (&NodesHandler{DB: db}).Get)
	return router, &node, &asset, &result
}

func TestSchemaResultsAssetsMigrationContract(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "schema-contract.db")
	db, err := model.Open(dbPath)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := model.Migrate(db); err != nil {
		t.Fatalf("migrate db: %v", err)
	}

	migrator := db.Migrator()
	if !migrator.HasTable(&model.Result{}) {
		t.Fatalf("missing table: results")
	}
	if !migrator.HasTable(&model.Asset{}) {
		t.Fatalf("missing table: assets")
	}
	if !migrator.HasTable(&model.PromptNode{}) {
		t.Fatalf("missing table: prompt_nodes")
	}
	if migrator.HasTable("sessions") {
		t.Fatalf("legacy table should not exist: sessions")
	}
	if migrator.HasTable("nodes") {
		t.Fatalf("legacy table should not exist: nodes")
	}
	if !migrator.HasColumn(&model.Result{}, "status") {
		t.Fatalf("missing column: results.status")
	}
	if !migrator.HasColumn(&model.Result{}, "archived_at") {
		t.Fatalf("missing column: results.archived_at")
	}
	if !migrator.HasColumn(&model.Result{}, "deleted_at") {
		t.Fatalf("missing column: results.deleted_at")
	}
	if !migrator.HasColumn(&model.Result{}, "source_asset_id") {
		t.Fatalf("missing column: results.source_asset_id")
	}
	if !migrator.HasColumn(&model.Result{}, "source_domain") {
		t.Fatalf("missing column: results.source_domain")
	}
	if !migrator.HasColumn(&model.PromptNode{}, "mode") {
		t.Fatalf("missing column: prompt_nodes.mode")
	}
	if !migrator.HasColumn(&model.PromptNode{}, "prompt") {
		t.Fatalf("missing column: prompt_nodes.prompt")
	}
	if !migrator.HasColumn(&model.PromptNode{}, "title") {
		t.Fatalf("missing column: prompt_nodes.title")
	}
	if !migrator.HasColumn(&model.PromptNode{}, "pinned") {
		t.Fatalf("missing column: prompt_nodes.pinned")
	}
	if !migrator.HasColumn(&model.PromptNode{}, "sort_order") {
		t.Fatalf("missing column: prompt_nodes.sort_order")
	}
	if !migrator.HasColumn(&model.PromptNode{}, "last_used_at") {
		t.Fatalf("missing column: prompt_nodes.last_used_at")
	}
	if !migrator.HasColumn(&model.PromptNode{}, "archived_at") {
		t.Fatalf("missing column: prompt_nodes.archived_at")
	}
	if !migrator.HasColumn(&model.PromptNode{}, "deleted_at") {
		t.Fatalf("missing column: prompt_nodes.deleted_at")
	}
	if !migrator.HasColumn(&model.PromptNode{}, "negative_prompt") {
		t.Fatalf("missing column: prompt_nodes.negative_prompt")
	}
	if !migrator.HasColumn(&model.PromptNode{}, "cfg_scale") {
		t.Fatalf("missing column: prompt_nodes.cfg_scale")
	}
	if !migrator.HasColumn(&model.PromptNode{}, "aspect_ratio") {
		t.Fatalf("missing column: prompt_nodes.aspect_ratio")
	}
	if !migrator.HasColumn(&model.PromptNode{}, "image_size") {
		t.Fatalf("missing column: prompt_nodes.image_size")
	}
	if !migrator.HasColumn(&model.PromptNode{}, "seed") {
		t.Fatalf("missing column: prompt_nodes.seed")
	}
	if !migrator.HasColumn(&model.PromptNode{}, "steps") {
		t.Fatalf("missing column: prompt_nodes.steps")
	}
	if !migrator.HasColumn(&model.PromptNode{}, "reference_image_ids") {
		t.Fatalf("missing column: prompt_nodes.reference_image_ids")
	}
	if migrator.HasColumn(&model.PromptNode{}, "root_node_id") {
		t.Fatalf("unexpected legacy column: prompt_nodes.root_node_id")
	}
	if migrator.HasColumn(&model.PromptNode{}, "status") {
		t.Fatalf("unexpected column: prompt_nodes.status")
	}
	if migrator.HasColumn(&model.PromptNode{}, "is_reroll") {
		t.Fatalf("unexpected column: prompt_nodes.is_reroll")
	}
	if migrator.HasColumn(&model.PromptNode{}, "baseline_prompt") {
		t.Fatalf("unexpected column: prompt_nodes.baseline_prompt")
	}
}

func TestModelResultDefaultStatusContract(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "model-contract.db")
	db, err := model.Open(dbPath)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := model.Migrate(db); err != nil {
		t.Fatalf("migrate db: %v", err)
	}

	now := mustParseRFC3339(t, "2026-02-08T00:00:01Z")
	node := model.PromptNode{ID: "node-model", Mode: model.PromptModeTxt2Img, Prompt: "prompt", Title: "prompt", LastUsedAt: now}
	if err := db.Create(&node).Error; err != nil {
		t.Fatalf("create node: %v", err)
	}
	asset := model.Asset{ID: "asset-model", Domain: model.AssetDomainTxt2Img, FilePath: "images/node-model/a.png", Origin: model.AssetOriginGenerated}
	if err := db.Create(&asset).Error; err != nil {
		t.Fatalf("create asset: %v", err)
	}

	result := model.Result{ID: "result-model", NodeID: node.ID, AssetID: asset.ID}
	if err := db.Create(&result).Error; err != nil {
		t.Fatalf("create result: %v", err)
	}

	var stored model.Result
	if err := db.First(&stored, "id = ?", result.ID).Error; err != nil {
		t.Fatalf("reload result: %v", err)
	}
	if stored.Status != model.ResultStatusActive {
		t.Fatalf("unexpected default status: %q", stored.Status)
	}
	if stored.ArchivedAt != nil {
		t.Fatalf("expected archived_at nil, got: %v", stored.ArchivedAt)
	}
	if stored.DeletedAt != nil {
		t.Fatalf("expected deleted_at nil, got: %v", stored.DeletedAt)
	}
}

func mustParseRFC3339(t *testing.T, value string) time.Time {
	t.Helper()
	parsed, err := time.Parse(time.RFC3339, value)
	if err != nil {
		t.Fatalf("parse RFC3339 %q: %v", value, err)
	}
	return parsed
}

func TestContractNodesResponseIncludesResults(t *testing.T) {
	router, node, _, result := setupNodesRouter(t)
	req, err := http.NewRequest(http.MethodGet, "/api/nodes/"+node.ID, nil)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("unexpected status: %d", recorder.Code)
	}

	var payload map[string]any
	if err := json.NewDecoder(recorder.Body).Decode(&payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload["images"] == nil {
		t.Fatalf("expected images in response for compatibility")
	}
	if payload["results"] == nil {
		t.Fatalf("expected results in response")
	}
	results, ok := payload["results"].([]any)
	if !ok || len(results) != 1 {
		t.Fatalf("unexpected results payload: %#v", payload["results"])
	}
	first, ok := results[0].(map[string]any)
	if !ok {
		t.Fatalf("unexpected result item: %#v", results[0])
	}
	if first["id"] != result.ID {
		t.Fatalf("unexpected result id: %#v", first["id"])
	}
	if first["status"] != model.ResultStatusActive {
		t.Fatalf("unexpected result status: %#v", first["status"])
	}
}
