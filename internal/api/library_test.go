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

func setupLibraryRouter(t *testing.T, now time.Time) (*gin.Engine, *gorm.DB) {
	t.Helper()

	gin.SetMode(gin.TestMode)

	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "library.db")
	db, err := model.Open(dbPath)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := model.Migrate(db); err != nil {
		t.Fatalf("migrate db: %v", err)
	}

	handler := &LibraryHandler{DB: db, Now: func() time.Time { return now }}
	router := gin.New()
	router.GET("/api/library", handler.List)
	return router, db
}

func performLibraryRequest(t *testing.T, router *gin.Engine, path string) *httptest.ResponseRecorder {
	t.Helper()

	request, err := http.NewRequest(http.MethodGet, path, nil)
	if err != nil {
		t.Fatalf("create request: %v", err)
	}

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)
	return recorder
}

func TestLibraryFilters(t *testing.T) {
	fixedNow := time.Date(2026, 2, 6, 12, 0, 0, 0, time.Local)
	router, db := setupLibraryRouter(t, fixedNow)

	now := fixedNow.UTC()
	txtNode := model.PromptNode{ID: "node-txt2img", Mode: model.PromptModeTxt2Img, Prompt: "txt", Title: "txt", LastUsedAt: now}
	imgNode := model.PromptNode{ID: "node-img2img", Mode: model.PromptModeImg2Img, Prompt: "img", Title: "img", LastUsedAt: now}
	if err := db.Create(&txtNode).Error; err != nil {
		t.Fatalf("create txt2img node: %v", err)
	}
	if err := db.Create(&imgNode).Error; err != nil {
		t.Fatalf("create img2img node: %v", err)
	}

	todayStart := time.Date(2026, 2, 6, 0, 0, 0, 0, time.Local)
	yesterdayStart := todayStart.AddDate(0, 0, -1)
	olderStart := todayStart.AddDate(0, 0, -2)

	txtNodeID := txtNode.ID
	imgNodeID := imgNode.ID

	images := []model.Image{
		{ID: "img-txt-today", NodeID: &txtNodeID, Origin: "generated", FilePath: "/tmp/txt-today.png", CreatedAt: todayStart.Add(9*time.Hour + 1*time.Minute)},
		{ID: "img-img-today", NodeID: &imgNodeID, Origin: "generated", FilePath: "/tmp/img-today.png", CreatedAt: todayStart.Add(10*time.Hour + 2*time.Minute)},
		{ID: "img-upload-today", NodeID: nil, Origin: "uploaded", FilePath: "/tmp/upload-today.png", CreatedAt: todayStart.Add(11*time.Hour + 3*time.Minute)},
		{ID: "img-txt-yesterday", NodeID: &txtNodeID, Origin: "generated", FilePath: "/tmp/txt-yesterday.png", CreatedAt: yesterdayStart.Add(8*time.Hour + 4*time.Minute)},
		{ID: "img-txt-older", NodeID: &txtNodeID, Origin: "generated", FilePath: "/tmp/txt-older.png", CreatedAt: olderStart.Add(7*time.Hour + 5*time.Minute)},
	}

	for _, image := range images {
		if err := db.Create(&image).Error; err != nil {
			t.Fatalf("create image %s: %v", image.ID, err)
		}
	}

	t.Run("filter txt2img today", func(t *testing.T) {
		recorder := performLibraryRequest(t, router, "/api/library?type=txt2img&date=today")
		if recorder.Code != http.StatusOK {
			t.Fatalf("unexpected status: %d", recorder.Code)
		}

		var resp []model.Image
		if err := json.NewDecoder(recorder.Body).Decode(&resp); err != nil {
			t.Fatalf("decode response: %v", err)
		}
		if len(resp) != 1 {
			t.Fatalf("unexpected image count: %d", len(resp))
		}
		if resp[0].ID != "img-txt-today" {
			t.Fatalf("unexpected image id: %s", resp[0].ID)
		}
		if resp[0].NodeID == nil {
			t.Fatalf("expected node id to be set")
		}
	})

	t.Run("filter img2img today", func(t *testing.T) {
		recorder := performLibraryRequest(t, router, "/api/library?type=img2img&date=today")
		if recorder.Code != http.StatusOK {
			t.Fatalf("unexpected status: %d", recorder.Code)
		}

		var resp []model.Image
		if err := json.NewDecoder(recorder.Body).Decode(&resp); err != nil {
			t.Fatalf("decode response: %v", err)
		}
		if len(resp) != 1 {
			t.Fatalf("unexpected image count: %d", len(resp))
		}
		if resp[0].ID != "img-img-today" {
			t.Fatalf("unexpected image id: %s", resp[0].ID)
		}
		if resp[0].NodeID == nil {
			t.Fatalf("expected node id to be set")
		}
	})

	t.Run("filter txt2img older", func(t *testing.T) {
		recorder := performLibraryRequest(t, router, "/api/library?type=txt2img&date=older")
		if recorder.Code != http.StatusOK {
			t.Fatalf("unexpected status: %d", recorder.Code)
		}

		var resp []model.Image
		if err := json.NewDecoder(recorder.Body).Decode(&resp); err != nil {
			t.Fatalf("decode response: %v", err)
		}
		if len(resp) != 1 {
			t.Fatalf("unexpected image count: %d", len(resp))
		}
		if resp[0].ID != "img-txt-older" {
			t.Fatalf("unexpected image id: %s", resp[0].ID)
		}
	})

	t.Run("pagination", func(t *testing.T) {
		recorder := performLibraryRequest(t, router, "/api/library?page=1&limit=2")
		if recorder.Code != http.StatusOK {
			t.Fatalf("unexpected status: %d", recorder.Code)
		}

		var resp []model.Image
		if err := json.NewDecoder(recorder.Body).Decode(&resp); err != nil {
			t.Fatalf("decode response: %v", err)
		}
		if len(resp) != 2 {
			t.Fatalf("unexpected image count: %d", len(resp))
		}

		expectedOrder := []string{"img-upload-today", "img-img-today"}
		for i, expectedID := range expectedOrder {
			if resp[i].ID != expectedID {
				t.Fatalf("unexpected order at %d: %s", i, resp[i].ID)
			}
		}
	})
}
