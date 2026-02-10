package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"banana-lab/internal/model"
)

func setupResultsRouter(t *testing.T) (*gin.Engine, *gorm.DB, *model.Node, *model.Result) {
	t.Helper()

	gin.SetMode(gin.TestMode)
	dbPath := filepath.Join(t.TempDir(), "results.db")
	db, err := model.Open(dbPath)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := model.Migrate(db); err != nil {
		t.Fatalf("migrate db: %v", err)
	}

	now := time.Now().UTC()
	node := model.Node{ID: "node-result", Mode: model.PromptModeTxt2Img, Prompt: "prompt", Title: "prompt", LastUsedAt: now}
	if err := db.Create(&node).Error; err != nil {
		t.Fatalf("create node: %v", err)
	}

	asset := model.Asset{ID: "asset-result", Domain: model.AssetDomainTxt2Img, FilePath: "images/txt2img/node-result/a.png", Origin: model.AssetOriginGenerated}
	if err := db.Create(&asset).Error; err != nil {
		t.Fatalf("create asset: %v", err)
	}

	result := model.Result{ID: "result-active", NodeID: node.ID, AssetID: asset.ID}
	if err := db.Create(&result).Error; err != nil {
		t.Fatalf("create result: %v", err)
	}

	router := gin.New()
	router.GET("/api/nodes/:id", (&NodesHandler{DB: db}).Get)
	router.DELETE("/api/results/:id", (&ResultsHandler{DB: db}).Delete)

	return router, db, &node, &result
}

func TestResultDeleteSoftDeleteIdempotent(t *testing.T) {
	router, db, _, seed := setupResultsRouter(t)

	firstReq, err := http.NewRequest(http.MethodDelete, "/api/results/"+seed.ID, nil)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	first := httptest.NewRecorder()
	router.ServeHTTP(first, firstReq)
	if first.Code != http.StatusOK {
		t.Fatalf("unexpected first delete status: %d", first.Code)
	}

	secondReq, err := http.NewRequest(http.MethodDelete, "/api/results/"+seed.ID, nil)
	if err != nil {
		t.Fatalf("new second request: %v", err)
	}
	second := httptest.NewRecorder()
	router.ServeHTTP(second, secondReq)
	if second.Code != http.StatusOK {
		t.Fatalf("unexpected second delete status: %d", second.Code)
	}

	var payload map[string]any
	if err := json.NewDecoder(second.Body).Decode(&payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload["status"] != model.ResultStatusDeleted {
		t.Fatalf("unexpected delete status payload: %#v", payload["status"])
	}

	var stored model.Result
	if err := db.First(&stored, "id = ?", seed.ID).Error; err != nil {
		t.Fatalf("reload result: %v", err)
	}
	if stored.Status != model.ResultStatusDeleted {
		t.Fatalf("unexpected stored status: %q", stored.Status)
	}
	if stored.DeletedAt == nil {
		t.Fatalf("expected deleted_at to be set")
	}
}

func TestResultDeleteNotFound(t *testing.T) {
	router, _, _, _ := setupResultsRouter(t)

	req, err := http.NewRequest(http.MethodDelete, "/api/results/not-exist", nil)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("unexpected status: %d", rec.Code)
	}

	var payload map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload["error"] != "not_found" {
		t.Fatalf("unexpected error payload: %#v", payload)
	}
}

func TestNodesGetOnlyReturnsActiveResults(t *testing.T) {
	router, db, node, seed := setupResultsRouter(t)

	archivedAsset := model.Asset{ID: "asset-archived", Domain: model.AssetDomainTxt2Img, FilePath: "images/txt2img/node-result/b.png", Origin: model.AssetOriginGenerated}
	if err := db.Create(&archivedAsset).Error; err != nil {
		t.Fatalf("create archived asset: %v", err)
	}
	archived := model.Result{ID: "result-archived", NodeID: node.ID, AssetID: archivedAsset.ID, Status: model.ResultStatusArchived}
	if err := db.Create(&archived).Error; err != nil {
		t.Fatalf("create archived result: %v", err)
	}

	deletedAsset := model.Asset{ID: "asset-deleted", Domain: model.AssetDomainTxt2Img, FilePath: "images/txt2img/node-result/c.png", Origin: model.AssetOriginGenerated}
	if err := db.Create(&deletedAsset).Error; err != nil {
		t.Fatalf("create deleted asset: %v", err)
	}
	deleted := model.Result{ID: "result-deleted", NodeID: node.ID, AssetID: deletedAsset.ID, Status: model.ResultStatusDeleted}
	if err := db.Create(&deleted).Error; err != nil {
		t.Fatalf("create deleted result: %v", err)
	}

	req, err := http.NewRequest(http.MethodGet, "/api/nodes/"+node.ID, nil)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("unexpected status: %d", rec.Code)
	}

	var payload map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	rawResults, ok := payload["results"].([]any)
	if !ok {
		t.Fatalf("unexpected results payload: %#v", payload["results"])
	}
	if len(rawResults) != 1 {
		t.Fatalf("expected only active result, got %d", len(rawResults))
	}
	first, ok := rawResults[0].(map[string]any)
	if !ok {
		t.Fatalf("unexpected result item: %#v", rawResults[0])
	}
	if first["id"] != seed.ID {
		t.Fatalf("unexpected result id: %#v", first["id"])
	}
	if first["status"] != model.ResultStatusActive {
		t.Fatalf("unexpected result status: %#v", first["status"])
	}
}
