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

func setupPromptsRouter(t *testing.T) (*gin.Engine, *gorm.DB) {
	t.Helper()

	gin.SetMode(gin.TestMode)
	dbPath := filepath.Join(t.TempDir(), "prompts.db")
	db, err := model.Open(dbPath)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := model.Migrate(db); err != nil {
		t.Fatalf("migrate db: %v", err)
	}

	handler := &PromptsHandler{DB: db}
	router := gin.New()
	router.GET("/api/prompts", handler.List)
	router.PATCH("/api/prompts/:id", handler.Update)
	router.GET("/api/prompts/:id/tree", (&PromptTreeHandler{DB: db}).Get)

	return router, db
}

func TestPromptsListQueryParams(t *testing.T) {
	router, db := setupPromptsRouter(t)
	base := time.Date(2026, 2, 8, 10, 0, 0, 0, time.UTC)
	archivedAt := base.Add(2 * time.Hour)

	nodes := []model.PromptNode{
		{ID: "prompt-1", Mode: model.PromptModeTxt2Img, Prompt: "one", Title: "One", Pinned: true, LastUsedAt: base.Add(1 * time.Minute), CreatedAt: base.Add(1 * time.Minute)},
		{ID: "prompt-2", Mode: model.PromptModeTxt2Img, Prompt: "two", Title: "Two", Pinned: true, LastUsedAt: base.Add(2 * time.Minute), CreatedAt: base.Add(2 * time.Minute)},
		{ID: "prompt-3", Mode: model.PromptModeTxt2Img, Prompt: "three", Title: "Three", Pinned: false, LastUsedAt: base.Add(3 * time.Minute), CreatedAt: base.Add(3 * time.Minute)},
		{ID: "prompt-4", Mode: model.PromptModeImg2Img, Prompt: "four", Title: "Four", Pinned: false, LastUsedAt: base.Add(4 * time.Minute), CreatedAt: base.Add(4 * time.Minute)},
		{ID: "prompt-5", Mode: model.PromptModeTxt2Img, Prompt: "five", Title: "Five", Pinned: false, LastUsedAt: base.Add(5 * time.Minute), CreatedAt: base.Add(5 * time.Minute), ArchivedAt: &archivedAt},
	}
	for _, node := range nodes {
		if err := db.Create(&node).Error; err != nil {
			t.Fatalf("create node %s: %v", node.ID, err)
		}
	}

	t.Run("mode filter and ordering", func(t *testing.T) {
		recorder := performJSONRequest(t, router, http.MethodGet, "/api/prompts?mode=txt2img", nil)
		if recorder.Code != http.StatusOK {
			t.Fatalf("unexpected status: %d", recorder.Code)
		}

		var resp []model.PromptNode
		if err := json.NewDecoder(recorder.Body).Decode(&resp); err != nil {
			t.Fatalf("decode response: %v", err)
		}
		if len(resp) != 3 {
			t.Fatalf("unexpected count: %d", len(resp))
		}
		expected := []string{"prompt-2", "prompt-1", "prompt-3"}
		for i, id := range expected {
			if resp[i].ID != id {
				t.Fatalf("unexpected order at %d: %s", i, resp[i].ID)
			}
		}
	})

	t.Run("include archived", func(t *testing.T) {
		recorder := performJSONRequest(t, router, http.MethodGet, "/api/prompts?mode=txt2img&include_archived=true", nil)
		if recorder.Code != http.StatusOK {
			t.Fatalf("unexpected status: %d", recorder.Code)
		}

		var resp []model.PromptNode
		if err := json.NewDecoder(recorder.Body).Decode(&resp); err != nil {
			t.Fatalf("decode response: %v", err)
		}
		if len(resp) != 4 {
			t.Fatalf("unexpected count: %d", len(resp))
		}
		if resp[0].ID != "prompt-2" || resp[1].ID != "prompt-1" {
			t.Fatalf("unexpected leading order: %s %s", resp[0].ID, resp[1].ID)
		}
	})

	t.Run("limit offset", func(t *testing.T) {
		recorder := performJSONRequest(t, router, http.MethodGet, "/api/prompts?include_archived=true&limit=2&offset=1", nil)
		if recorder.Code != http.StatusOK {
			t.Fatalf("unexpected status: %d", recorder.Code)
		}

		var resp []model.PromptNode
		if err := json.NewDecoder(recorder.Body).Decode(&resp); err != nil {
			t.Fatalf("decode response: %v", err)
		}
		if len(resp) != 2 {
			t.Fatalf("unexpected count: %d", len(resp))
		}
	})

	t.Run("invalid params", func(t *testing.T) {
		cases := []string{
			"/api/prompts?mode=invalid",
			"/api/prompts?limit=0",
			"/api/prompts?limit=101",
			"/api/prompts?offset=-1",
			"/api/prompts?include_archived=not-bool",
		}
		for _, path := range cases {
			recorder := performJSONRequest(t, router, http.MethodGet, path, nil)
			if recorder.Code != http.StatusBadRequest {
				t.Fatalf("unexpected status for %s: %d", path, recorder.Code)
			}
		}
	})
}

func TestPromptsPatchAndTreeNotFound(t *testing.T) {
	router, db := setupPromptsRouter(t)
	now := time.Date(2026, 2, 8, 11, 0, 0, 0, time.UTC)
	node := model.PromptNode{ID: "prompt-update", Mode: model.PromptModeTxt2Img, Prompt: "prompt", Title: "prompt", LastUsedAt: now}
	if err := db.Create(&node).Error; err != nil {
		t.Fatalf("create node: %v", err)
	}

	recorder := performJSONRequest(t, router, http.MethodPatch, "/api/prompts/prompt-update", map[string]any{
		"title":      "Updated",
		"pinned":     true,
		"sort_order": 7,
		"archived":   true,
	})
	if recorder.Code != http.StatusOK {
		t.Fatalf("unexpected status: %d body=%s", recorder.Code, recorder.Body.String())
	}

	var updated model.PromptNode
	if err := json.NewDecoder(recorder.Body).Decode(&updated); err != nil {
		t.Fatalf("decode update response: %v", err)
	}
	if updated.Title != "Updated" || !updated.Pinned {
		t.Fatalf("unexpected update payload: %#v", updated)
	}
	if updated.SortOrder == nil || *updated.SortOrder != 7 {
		t.Fatalf("unexpected sort_order: %#v", updated.SortOrder)
	}
	if updated.ArchivedAt == nil {
		t.Fatalf("expected archived_at to be set")
	}

	recorder = performJSONRequest(t, router, http.MethodPatch, "/api/prompts/prompt-update", map[string]any{"archived": false})
	if recorder.Code != http.StatusOK {
		t.Fatalf("unexpected status: %d", recorder.Code)
	}

	var dbNode model.PromptNode
	if err := db.First(&dbNode, "id = ?", "prompt-update").Error; err != nil {
		t.Fatalf("reload node: %v", err)
	}
	if dbNode.ArchivedAt != nil {
		t.Fatalf("expected archived_at cleared")
	}

	// 契约测试：bad id tree 必须 404 + 固定错误体
	badTree := performJSONRequest(t, router, http.MethodGet, "/api/prompts/bad-id/tree", nil)
	if badTree.Code != http.StatusNotFound {
		t.Fatalf("unexpected status: %d", badTree.Code)
	}
	var payload map[string]any
	if err := json.NewDecoder(badTree.Body).Decode(&payload); err != nil {
		t.Fatalf("decode bad tree response: %v", err)
	}
	if payload["error"] != "not_found" || payload["message"] != "node not found" {
		t.Fatalf("unexpected payload: %#v", payload)
	}
}
