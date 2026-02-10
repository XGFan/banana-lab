package api

import (
	"encoding/json"
	"net/http"
	"path/filepath"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"banana-lab/internal/model"
)

func setupArchiveRouter(t *testing.T) (*gin.Engine, *gorm.DB) {
	t.Helper()

	gin.SetMode(gin.TestMode)
	dbPath := filepath.Join(t.TempDir(), "archive.db")
	db, err := model.Open(dbPath)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := model.Migrate(db); err != nil {
		t.Fatalf("migrate db: %v", err)
	}

	router := gin.New()
	router.GET("/api/archive", (&ArchiveHandler{DB: db}).List)
	return router, db
}

func TestArchiveListGroupedByDomainWithRange(t *testing.T) {
	router, db := setupArchiveRouter(t)

	base := time.Date(2026, 1, 2, 15, 0, 0, 0, time.UTC)
	inRangeAt := base.Add(-30 * time.Minute)
	outOfRangeAt := base.Add(-3 * time.Hour)

	assets := []model.Asset{
		{ID: "asset-archive-txt", Domain: model.AssetDomainTxt2Img, FilePath: "images/s1/n1/txt.png", Origin: model.AssetOriginGenerated},
		{ID: "asset-archive-img", Domain: model.AssetDomainImg2Img, FilePath: "images/s2/n2/img.png", Origin: model.AssetOriginGenerated},
		{ID: "asset-active", Domain: model.AssetDomainTxt2Img, FilePath: "images/s3/n3/active.png", Origin: model.AssetOriginGenerated},
	}
	for _, asset := range assets {
		if err := db.Create(&asset).Error; err != nil {
			t.Fatalf("create asset %s: %v", asset.ID, err)
		}
	}

	results := []model.Result{
		{ID: "result-archive-txt", NodeID: "node-1", AssetID: "asset-archive-txt", Status: model.ResultStatusArchived, ArchivedAt: &inRangeAt},
		{ID: "result-archive-img", NodeID: "node-2", AssetID: "asset-archive-img", Status: model.ResultStatusArchived, ArchivedAt: &outOfRangeAt},
		{ID: "result-active", NodeID: "node-3", AssetID: "asset-active", Status: model.ResultStatusActive},
	}
	for _, result := range results {
		if err := db.Create(&result).Error; err != nil {
			t.Fatalf("create result %s: %v", result.ID, err)
		}
	}

	from := base.Add(-1 * time.Hour).Format(time.RFC3339)
	to := base.Format(time.RFC3339)
	rec := performJSONRequest(t, router, http.MethodGet, "/api/archive?from="+from+"&to="+to, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("unexpected status: %d", rec.Code)
	}

	var payload map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	txt2img, ok := payload[model.AssetDomainTxt2Img].([]any)
	if !ok {
		t.Fatalf("unexpected txt2img payload: %#v", payload[model.AssetDomainTxt2Img])
	}
	if len(txt2img) != 1 {
		t.Fatalf("unexpected txt2img length: %d", len(txt2img))
	}
	item, ok := txt2img[0].(map[string]any)
	if !ok {
		t.Fatalf("unexpected item payload: %#v", txt2img[0])
	}
	if item["id"] != "result-archive-txt" {
		t.Fatalf("unexpected result id: %#v", item["id"])
	}
	if item["domain"] != model.AssetDomainTxt2Img {
		t.Fatalf("unexpected domain: %#v", item["domain"])
	}
	if item["asset_id"] != "asset-archive-txt" {
		t.Fatalf("unexpected asset_id: %#v", item["asset_id"])
	}
	if item["archived_at"] == nil {
		t.Fatalf("expected archived_at to be present")
	}
	assetPayload, ok := item["asset"].(map[string]any)
	if !ok {
		t.Fatalf("unexpected asset payload: %#v", item["asset"])
	}
	if assetPayload["file_path"] != "images/s1/n1/txt.png" {
		t.Fatalf("unexpected asset file_path: %#v", assetPayload["file_path"])
	}

	img2img, ok := payload[model.AssetDomainImg2Img].([]any)
	if !ok {
		t.Fatalf("unexpected img2img payload: %#v", payload[model.AssetDomainImg2Img])
	}
	if len(img2img) != 0 {
		t.Fatalf("expected empty img2img list, got %d", len(img2img))
	}
}

func TestArchiveListInvalidRange(t *testing.T) {
	router, _ := setupArchiveRouter(t)

	from := "2026-01-02T16:00:00Z"
	to := "2026-01-02T15:00:00Z"
	rec := performJSONRequest(t, router, http.MethodGet, "/api/archive?from="+from+"&to="+to, nil)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("unexpected status: %d", rec.Code)
	}

	var payload map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload["error"] != "invalid" {
		t.Fatalf("unexpected error payload: %#v", payload)
	}
	if payload["code"] != archiveRangeInvalidCode {
		t.Fatalf("unexpected code payload: %#v", payload)
	}
}
