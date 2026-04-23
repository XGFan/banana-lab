package api

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/gin-gonic/gin"
)

func setupModelsRouter(t *testing.T) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	handler := &ModelsHandler{Registry: testRegistry()}
	router := gin.New()
	router.GET("/api/models", handler.List)
	return router
}

func TestModelsAPI_returnsOnlyEnabled(t *testing.T) {
	router := setupModelsRouter(t)
	rec := performJSONRequest(t, router, http.MethodGet, "/api/models", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got: %d body=%s", rec.Code, rec.Body.String())
	}
	var payload struct {
		Models []map[string]any `json:"models"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(payload.Models) != 2 {
		t.Fatalf("expected 2 enabled models, got %d: %#v", len(payload.Models), payload.Models)
	}
	ids := map[string]bool{}
	for _, m := range payload.Models {
		id, _ := m["id"].(string)
		ids[id] = true
	}
	if !ids["gemini-3-pro-image"] || !ids["gpt-image-2"] {
		t.Fatalf("missing expected enabled models: %#v", ids)
	}
	if ids["disabled-model"] {
		t.Fatalf("disabled-model should not be listed: %#v", ids)
	}
}

func TestModelsAPI_responseShape(t *testing.T) {
	router := setupModelsRouter(t)
	rec := performJSONRequest(t, router, http.MethodGet, "/api/models", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got: %d", rec.Code)
	}
	var payload struct {
		Models []map[string]any `json:"models"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	requiredKeys := []string{"id", "label", "allowed_sizes", "allowed_aspect_ratios", "default_size"}
	for i, m := range payload.Models {
		for _, key := range requiredKeys {
			if _, ok := m[key]; !ok {
				t.Fatalf("entry %d missing %q: %#v", i, key, m)
			}
		}
		if _, ok := m["allowed_sizes"].([]any); !ok {
			t.Fatalf("entry %d allowed_sizes should be an array: %#v", i, m["allowed_sizes"])
		}
		if _, ok := m["allowed_aspect_ratios"].([]any); !ok {
			t.Fatalf("entry %d allowed_aspect_ratios should be an array: %#v", i, m["allowed_aspect_ratios"])
		}
	}
}
