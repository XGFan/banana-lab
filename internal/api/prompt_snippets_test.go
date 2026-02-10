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

func setupPromptSnippetsRouter(t *testing.T) (*gin.Engine, *gorm.DB) {
	t.Helper()

	gin.SetMode(gin.TestMode)
	dbPath := filepath.Join(t.TempDir(), "prompt-snippets.db")
	db, err := model.Open(dbPath)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := model.Migrate(db); err != nil {
		t.Fatalf("migrate db: %v", err)
	}

	handler := &PromptSnippetsHandler{DB: db}
	router := gin.New()
	router.POST("/api/prompt-snippets", handler.Create)
	router.GET("/api/prompt-snippets", handler.List)
	router.PUT("/api/prompt-snippets", handler.ReplaceAll)
	router.PATCH("/api/prompt-snippets/:id", handler.Update)
	router.DELETE("/api/prompt-snippets/:id", handler.Delete)

	return router, db
}

func TestPromptSnippetsCRUD(t *testing.T) {
	router, _ := setupPromptSnippetsRouter(t)

	createResp := performJSONRequest(t, router, http.MethodPost, "/api/prompt-snippets", map[string]any{
		"name": "tone",
		"content_fragments": []map[string]any{
			{"type": "text", "text": "cinematic"},
			{"type": "token", "key": "style", "meta": map[string]any{"weight": 0.8}},
		},
	})
	if createResp.Code != http.StatusCreated {
		t.Fatalf("unexpected create status: %d body=%s", createResp.Code, createResp.Body.String())
	}

	var created model.PromptSnippet
	if err := json.NewDecoder(createResp.Body).Decode(&created); err != nil {
		t.Fatalf("decode create response: %v", err)
	}
	if created.ID == "" {
		t.Fatalf("expected id to be generated")
	}
	if created.Name != "tone" {
		t.Fatalf("unexpected name: %s", created.Name)
	}
	if len(created.ContentFragments) != 2 {
		t.Fatalf("unexpected content_fragments length: %d", len(created.ContentFragments))
	}
	if created.CreatedAt.IsZero() || created.UpdatedAt.IsZero() {
		t.Fatalf("expected created_at/updated_at to be set")
	}

	listResp := performJSONRequest(t, router, http.MethodGet, "/api/prompt-snippets", nil)
	if listResp.Code != http.StatusOK {
		t.Fatalf("unexpected list status: %d", listResp.Code)
	}
	var listed []model.PromptSnippet
	if err := json.NewDecoder(listResp.Body).Decode(&listed); err != nil {
		t.Fatalf("decode list response: %v", err)
	}
	if len(listed) != 1 || listed[0].ID != created.ID {
		t.Fatalf("unexpected list payload: %#v", listed)
	}

	patchResp := performJSONRequest(t, router, http.MethodPatch, "/api/prompt-snippets/"+created.ID, map[string]any{
		"name": "tone-v2",
		"content_fragments": []map[string]any{
			{"type": "text", "text": "dramatic", "unknown": true},
		},
	})
	if patchResp.Code != http.StatusOK {
		t.Fatalf("unexpected patch status: %d body=%s", patchResp.Code, patchResp.Body.String())
	}

	var updated model.PromptSnippet
	if err := json.NewDecoder(patchResp.Body).Decode(&updated); err != nil {
		t.Fatalf("decode patch response: %v", err)
	}
	if updated.Name != "tone-v2" {
		t.Fatalf("unexpected updated name: %s", updated.Name)
	}
	if len(updated.ContentFragments) != 1 {
		t.Fatalf("unexpected updated fragments length: %d", len(updated.ContentFragments))
	}

	deleteResp := performJSONRequest(t, router, http.MethodDelete, "/api/prompt-snippets/"+created.ID, nil)
	if deleteResp.Code != http.StatusNoContent {
		t.Fatalf("unexpected delete status: %d", deleteResp.Code)
	}

	listAfterDelete := performJSONRequest(t, router, http.MethodGet, "/api/prompt-snippets", nil)
	if listAfterDelete.Code != http.StatusOK {
		t.Fatalf("unexpected list-after-delete status: %d", listAfterDelete.Code)
	}
	if err := json.NewDecoder(listAfterDelete.Body).Decode(&listed); err != nil {
		t.Fatalf("decode list-after-delete response: %v", err)
	}
	if len(listed) != 0 {
		t.Fatalf("expected empty list after delete, got %d", len(listed))
	}
}

func TestPromptSnippetsCreateWithClientProvidedID(t *testing.T) {
	router, _ := setupPromptSnippetsRouter(t)

	resp := performJSONRequest(t, router, http.MethodPost, "/api/prompt-snippets", map[string]any{
		"id":   "sdk-group-1",
		"name": "tone",
		"content_fragments": []map[string]any{
			{"type": "text", "text": "cinematic"},
		},
	})
	if resp.Code != http.StatusCreated {
		t.Fatalf("unexpected status: %d body=%s", resp.Code, resp.Body.String())
	}

	var created model.PromptSnippet
	if err := json.NewDecoder(resp.Body).Decode(&created); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if created.ID != "sdk-group-1" {
		t.Fatalf("unexpected id: %s", created.ID)
	}
}

func TestPromptSnippetsListOrderByUpdatedAtDesc(t *testing.T) {
	router, db := setupPromptSnippetsRouter(t)

	base := time.Date(2026, 2, 9, 10, 0, 0, 0, time.UTC)
	items := []model.PromptSnippet{
		{ID: "snippet-old", Name: "old", ContentFragments: model.PromptFragments{{"type": "text", "text": "old"}}},
		{ID: "snippet-new", Name: "new", ContentFragments: model.PromptFragments{{"type": "text", "text": "new"}}},
	}
	for _, item := range items {
		if err := db.Create(&item).Error; err != nil {
			t.Fatalf("create snippet %s: %v", item.ID, err)
		}
	}
	if err := db.Model(&model.PromptSnippet{}).Where("id = ?", "snippet-old").Update("updated_at", base).Error; err != nil {
		t.Fatalf("set updated_at for old snippet: %v", err)
	}
	if err := db.Model(&model.PromptSnippet{}).Where("id = ?", "snippet-new").Update("updated_at", base.Add(2*time.Hour)).Error; err != nil {
		t.Fatalf("set updated_at for new snippet: %v", err)
	}

	resp := performJSONRequest(t, router, http.MethodGet, "/api/prompt-snippets", nil)
	if resp.Code != http.StatusOK {
		t.Fatalf("unexpected status: %d", resp.Code)
	}

	var listed []model.PromptSnippet
	if err := json.NewDecoder(resp.Body).Decode(&listed); err != nil {
		t.Fatalf("decode list response: %v", err)
	}
	if len(listed) != 2 {
		t.Fatalf("unexpected list size: %d", len(listed))
	}
	if listed[0].ID != "snippet-new" || listed[1].ID != "snippet-old" {
		t.Fatalf("unexpected order: %#v", listed)
	}
}

func TestPromptSnippetsInvalidAndNotFound(t *testing.T) {
	router, _ := setupPromptSnippetsRouter(t)

	t.Run("invalid create payload", func(t *testing.T) {
		resp := performJSONRequest(t, router, http.MethodPost, "/api/prompt-snippets", map[string]any{
			"name":              "",
			"content_fragments": []map[string]any{{"type": "text", "text": "x"}},
		})
		if resp.Code != http.StatusBadRequest {
			t.Fatalf("unexpected status: %d", resp.Code)
		}
		var payload map[string]any
		if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
			t.Fatalf("decode invalid response: %v", err)
		}
		if payload["error"] != "invalid" {
			t.Fatalf("unexpected payload: %#v", payload)
		}
	})

	t.Run("invalid create payload with blank id", func(t *testing.T) {
		resp := performJSONRequest(t, router, http.MethodPost, "/api/prompt-snippets", map[string]any{
			"id":                "   ",
			"name":              "tone",
			"content_fragments": []map[string]any{{"type": "text", "text": "x"}},
		})
		if resp.Code != http.StatusBadRequest {
			t.Fatalf("unexpected status: %d", resp.Code)
		}
		var payload map[string]any
		if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
			t.Fatalf("decode invalid response: %v", err)
		}
		if payload["error"] != "invalid" {
			t.Fatalf("unexpected payload: %#v", payload)
		}
	})

	t.Run("duplicate id rejected", func(t *testing.T) {
		first := performJSONRequest(t, router, http.MethodPost, "/api/prompt-snippets", map[string]any{
			"id":                "stable-id",
			"name":              "tone",
			"content_fragments": []map[string]any{{"type": "text", "text": "first"}},
		})
		if first.Code != http.StatusCreated {
			t.Fatalf("unexpected first create status: %d", first.Code)
		}

		dup := performJSONRequest(t, router, http.MethodPost, "/api/prompt-snippets", map[string]any{
			"id":                "stable-id",
			"name":              "tone-v2",
			"content_fragments": []map[string]any{{"type": "text", "text": "second"}},
		})
		if dup.Code != http.StatusBadRequest {
			t.Fatalf("unexpected duplicate create status: %d body=%s", dup.Code, dup.Body.String())
		}
		var payload map[string]any
		if err := json.NewDecoder(dup.Body).Decode(&payload); err != nil {
			t.Fatalf("decode duplicate response: %v", err)
		}
		if payload["error"] != "invalid" {
			t.Fatalf("unexpected payload: %#v", payload)
		}
	})

	t.Run("not found on patch", func(t *testing.T) {
		resp := performJSONRequest(t, router, http.MethodPatch, "/api/prompt-snippets/missing-id", map[string]any{"name": "new-name"})
		if resp.Code != http.StatusNotFound {
			t.Fatalf("unexpected status: %d", resp.Code)
		}
		var payload map[string]any
		if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
			t.Fatalf("decode not_found response: %v", err)
		}
		if payload["error"] != "not_found" || payload["message"] != "snippet not found" {
			t.Fatalf("unexpected payload: %#v", payload)
		}
	})

	t.Run("not found on delete", func(t *testing.T) {
		resp := performJSONRequest(t, router, http.MethodDelete, "/api/prompt-snippets/missing-id", nil)
		if resp.Code != http.StatusNotFound {
			t.Fatalf("unexpected status: %d", resp.Code)
		}
		var payload map[string]any
		if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
			t.Fatalf("decode not_found response: %v", err)
		}
		if payload["error"] != "not_found" || payload["message"] != "snippet not found" {
			t.Fatalf("unexpected payload: %#v", payload)
		}
	})
}

func TestPromptSnippetsReplaceAll(t *testing.T) {
	router, _ := setupPromptSnippetsRouter(t)

	_ = performJSONRequest(t, router, http.MethodPost, "/api/prompt-snippets", map[string]any{
		"id":   "old-1",
		"name": "old",
		"content_fragments": []map[string]any{{
			"type": "text",
			"text": "old text",
		}},
	})

	resp := performJSONRequest(t, router, http.MethodPut, "/api/prompt-snippets", []map[string]any{
		{
			"id":   "new-1",
			"name": "new one",
			"content_fragments": []map[string]any{{
				"type": "text",
				"text": "new text 1",
			}},
		},
		{
			"id":   "new-2",
			"name": "new two",
			"content_fragments": []map[string]any{{
				"type": "text",
				"text": "new text 2",
			}},
		},
	})
	if resp.Code != http.StatusOK {
		t.Fatalf("unexpected replace status: %d body=%s", resp.Code, resp.Body.String())
	}

	listResp := performJSONRequest(t, router, http.MethodGet, "/api/prompt-snippets", nil)
	if listResp.Code != http.StatusOK {
		t.Fatalf("unexpected list status: %d", listResp.Code)
	}
	var listed []model.PromptSnippet
	if err := json.NewDecoder(listResp.Body).Decode(&listed); err != nil {
		t.Fatalf("decode list response: %v", err)
	}
	if len(listed) != 2 {
		t.Fatalf("unexpected list size after replace: %d", len(listed))
	}
	ids := map[string]bool{}
	for _, item := range listed {
		ids[item.ID] = true
	}
	if !ids["new-1"] || !ids["new-2"] || ids["old-1"] {
		t.Fatalf("unexpected ids after replace: %#v", ids)
	}
}

func TestPromptSnippetsReplaceAllInvalid(t *testing.T) {
	router, _ := setupPromptSnippetsRouter(t)

	t.Run("duplicate ids rejected", func(t *testing.T) {
		resp := performJSONRequest(t, router, http.MethodPut, "/api/prompt-snippets", []map[string]any{
			{
				"id":                "same",
				"name":              "one",
				"content_fragments": []map[string]any{{"type": "text", "text": "x"}},
			},
			{
				"id":                "same",
				"name":              "two",
				"content_fragments": []map[string]any{{"type": "text", "text": "y"}},
			},
		})
		if resp.Code != http.StatusBadRequest {
			t.Fatalf("unexpected status: %d", resp.Code)
		}
	})

	t.Run("blank id rejected", func(t *testing.T) {
		resp := performJSONRequest(t, router, http.MethodPut, "/api/prompt-snippets", []map[string]any{
			{
				"id":                "  ",
				"name":              "one",
				"content_fragments": []map[string]any{{"type": "text", "text": "x"}},
			},
		})
		if resp.Code != http.StatusBadRequest {
			t.Fatalf("unexpected status: %d", resp.Code)
		}
	})

	t.Run("nil fragments rejected", func(t *testing.T) {
		resp := performJSONRequest(t, router, http.MethodPut, "/api/prompt-snippets", []map[string]any{
			{
				"id":                "ok",
				"name":              "one",
				"content_fragments": nil,
			},
		})
		if resp.Code != http.StatusBadRequest {
			t.Fatalf("unexpected status: %d", resp.Code)
		}
	})
}
