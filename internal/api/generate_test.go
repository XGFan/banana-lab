package api

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"banana-lab/internal/config"
	"banana-lab/internal/model"
	"banana-lab/internal/service"
	"banana-lab/internal/storage"
)

// testRegistry builds the registry used by every GenerateHandler test.
// Keep in sync with the registry documented in Group 4 of the ralplan.
func testRegistry() config.Registry {
	return config.NewRegistry([]config.Model{
		{
			ID:                  "gemini-3-pro-image",
			Label:               "Gemini",
			Enabled:             true,
			AllowedSizes:        []string{"1K", "2K", "4K"},
			AllowedAspectRatios: []string{"1:1", "2:3", "3:2", "3:4", "4:3", "4:5", "5:4", "9:16", "16:9", "21:9"},
			DefaultSize:         "1K",
		},
		{
			ID:                  "gpt-image-2",
			Label:               "GPT Image 2",
			Enabled:             true,
			AllowedSizes:        []string{"1K", "2K"},
			AllowedAspectRatios: []string{"1:1", "2:3", "3:2", "3:4", "4:3", "9:16", "16:9"},
			DefaultSize:         "1K",
		},
		{
			ID:                  "disabled-model",
			Label:               "Disabled",
			Enabled:             false,
			AllowedSizes:        []string{"1K"},
			AllowedAspectRatios: []string{"1:1"},
			DefaultSize:         "1K",
		},
	})
}

// proxyRequestRecorder captures request bodies sent to the upstream proxy
// so tests can assert the model field reaches the wire.
type proxyRequestRecorder struct {
	mu    sync.Mutex
	bodies [][]byte
}

func (r *proxyRequestRecorder) record(body []byte) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.bodies = append(r.bodies, append([]byte(nil), body...))
}

func (r *proxyRequestRecorder) last() map[string]any {
	r.mu.Lock()
	defer r.mu.Unlock()
	if len(r.bodies) == 0 {
		return nil
	}
	var out map[string]any
	_ = json.Unmarshal(r.bodies[len(r.bodies)-1], &out)
	return out
}

func setupGenerateRouter(t *testing.T) (*gin.Engine, *gorm.DB, string) {
	router, db, storageRoot, _ := setupGenerateRouterWithRecorder(t)
	return router, db, storageRoot
}

func setupGenerateRouterWithRecorder(t *testing.T) (*gin.Engine, *gorm.DB, string, *proxyRequestRecorder) {
	t.Helper()

	gin.SetMode(gin.TestMode)

	dbPath := filepath.Join(t.TempDir(), "generate.db")
	db, err := model.Open(dbPath)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := model.Migrate(db); err != nil {
		t.Fatalf("migrate db: %v", err)
	}

	recorder := &proxyRequestRecorder{}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		recorder.record(body)
		r.Body = io.NopCloser(bytes.NewReader(body))
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"choices": []map[string]any{
				{
					"message": map[string]any{
						"images": []map[string]any{
							{
								"image_url": map[string]any{
									"url": "data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR4nGNgYAAAAAMAAWgmWQ0AAAAASUVORK5CYII=",
								},
							},
						},
					},
				},
			},
		})
	}))
	t.Cleanup(server.Close)

	storageRoot := t.TempDir()
	handler := &GenerateHandler{
		Service: &service.GenerateService{
			DB:      db,
			Storage: storage.Storage{Root: storageRoot},
			Proxy:   &service.ProxyClient{BaseURL: server.URL},
			Limiter: service.NewConcurrencyLimiter(1),
		},
		Registry: testRegistry(),
	}

	router := gin.New()
	router.POST("/api/generate", handler.Handle)
	router.POST("/api/prompts/:id/reroll", handler.Reroll)
	return router, db, storageRoot, recorder
}

func TestGenerateReferenceImages(t *testing.T) {
	router, _, _ := setupGenerateRouter(t)

	t.Run("valid request", func(t *testing.T) {
		recorder := performJSONRequest(t, router, http.MethodPost, "/api/generate", map[string]any{
			"mode":                model.PromptModeTxt2Img,
			"prompt":              "hello",
			"model":               "gemini-3-pro-image",
			"width":               512,
			"height":              512,
			"reference_image_ids": []string{},
		})

		if recorder.Code != http.StatusOK {
			t.Fatalf("unexpected status: %d", recorder.Code)
		}

		var payload map[string]any
		if err := json.NewDecoder(recorder.Body).Decode(&payload); err != nil {
			t.Fatalf("decode response: %v", err)
		}
		if payload["node"] == nil {
			t.Fatalf("expected node in response")
		}
		if payload["images"] == nil {
			t.Fatalf("expected images in response")
		}
		if payload["results"] == nil {
			t.Fatalf("expected results in response")
		}
	})

	t.Run("rejects legacy source_image_id", func(t *testing.T) {
		recorder := performJSONRequest(t, router, http.MethodPost, "/api/generate", map[string]any{
			"mode":            model.PromptModeTxt2Img,
			"prompt":          "hello",
			"model":           "gemini-3-pro-image",
			"width":           512,
			"height":          512,
			"source_image_id": "legacy-1",
		})

		if recorder.Code != http.StatusBadRequest {
			t.Fatalf("unexpected status: %d", recorder.Code)
		}
	})

	t.Run("rejects too many reference images", func(t *testing.T) {
		recorder := performJSONRequest(t, router, http.MethodPost, "/api/generate", map[string]any{
			"mode":                model.PromptModeTxt2Img,
			"prompt":              "hello",
			"model":               "gemini-3-pro-image",
			"width":               512,
			"height":              512,
			"reference_image_ids": []string{"1", "2", "3", "4", "5", "6"},
		})

		if recorder.Code != http.StatusBadRequest {
			t.Fatalf("unexpected status: %d", recorder.Code)
		}
	})

	t.Run("rejects invalid dimensions", func(t *testing.T) {
		cases := []map[string]any{
			{
				"mode":   model.PromptModeTxt2Img,
				"prompt": "hello",
				"height": 512,
			},
			{
				"mode":   model.PromptModeTxt2Img,
				"prompt": "hello",
				"width":  0,
				"height": 512,
			},
			{
				"mode":   model.PromptModeTxt2Img,
				"prompt": "hello",
				"width":  512,
				"height": -1,
			},
		}

		for i, body := range cases {
			t.Run(string(rune('a'+i)), func(t *testing.T) {
				recorder := performJSONRequest(t, router, http.MethodPost, "/api/generate", body)
				if recorder.Code != http.StatusBadRequest {
					t.Fatalf("unexpected status: %d", recorder.Code)
				}
			})
		}
	})
}

func TestGenerateImg2ImgReferenceDecouple(t *testing.T) {
	router, db, storageRoot := setupGenerateRouter(t)

	sourceRelPath := filepath.Join("images", "txt-source-session", "txt-node", "source.png")
	sourceAbsPath := filepath.Join(storageRoot, sourceRelPath)
	if err := os.MkdirAll(filepath.Dir(sourceAbsPath), 0o755); err != nil {
		t.Fatalf("mkdir source dir: %v", err)
	}
	sourceBytes := []byte("source-image-bytes")
	if err := os.WriteFile(sourceAbsPath, sourceBytes, 0o644); err != nil {
		t.Fatalf("write source file: %v", err)
	}
	sourceAsset := model.Asset{
		ID:       "asset-txt-source",
		Domain:   model.AssetDomainTxt2Img,
		Origin:   model.AssetOriginGenerated,
		FilePath: sourceRelPath,
	}
	if err := db.Create(&sourceAsset).Error; err != nil {
		t.Fatalf("create source asset: %v", err)
	}

	recorder := performJSONRequest(t, router, http.MethodPost, "/api/generate", map[string]any{
		"mode":                model.PromptModeImg2Img,
		"prompt":              "hello",
		"model":               "gemini-3-pro-image",
		"width":               512,
		"height":              512,
		"reference_image_ids": []string{sourceAsset.ID},
	})
	if recorder.Code != http.StatusOK {
		t.Fatalf("unexpected status: %d, body=%s", recorder.Code, recorder.Body.String())
	}

	var payload struct {
		Node model.Node `json:"node"`
	}
	if err := json.NewDecoder(recorder.Body).Decode(&payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(payload.Node.ReferenceImageIDs) != 1 {
		t.Fatalf("unexpected reference ids: %#v", payload.Node.ReferenceImageIDs)
	}
	if payload.Node.ReferenceImageIDs[0] == sourceAsset.ID {
		t.Fatalf("reference id should be decoupled, got source id: %s", sourceAsset.ID)
	}

	var copiedAsset model.Asset
	if err := db.First(&copiedAsset, "id = ?", payload.Node.ReferenceImageIDs[0]).Error; err != nil {
		t.Fatalf("load copied asset: %v", err)
	}
	if copiedAsset.Domain != model.AssetDomainImg2Img {
		t.Fatalf("unexpected copied asset domain: %s", copiedAsset.Domain)
	}
	if copiedAsset.Origin != sourceAsset.Origin {
		t.Fatalf("unexpected copied asset origin: %s", copiedAsset.Origin)
	}
	if !strings.Contains(copiedAsset.FilePath, filepath.Join("images", model.PromptModeImg2Img, "inputs", payload.Node.ID)) {
		t.Fatalf("unexpected copied asset path: %s", copiedAsset.FilePath)
	}

	copiedAbsPath := filepath.Join(storageRoot, copiedAsset.FilePath)
	copiedBytes, err := os.ReadFile(copiedAbsPath)
	if err != nil {
		t.Fatalf("read copied asset file: %v", err)
	}
	if string(copiedBytes) != string(sourceBytes) {
		t.Fatalf("copied bytes mismatch")
	}

	var result model.Result
	if err := db.Where("node_id = ?", payload.Node.ID).First(&result).Error; err != nil {
		t.Fatalf("load generation result: %v", err)
	}
	if result.SourceAssetID == nil || *result.SourceAssetID != sourceAsset.ID {
		t.Fatalf("unexpected source_asset_id: %#v", result.SourceAssetID)
	}
	if result.SourceDomain == nil || *result.SourceDomain != sourceAsset.Domain {
		t.Fatalf("unexpected source_domain: %#v", result.SourceDomain)
	}
}

func TestGenerateImg2ImgReferenceCopyFailureNoHalfState(t *testing.T) {
	router, db, _ := setupGenerateRouter(t)

	brokenAsset := model.Asset{
		ID:       "asset-broken-source",
		Domain:   model.AssetDomainTxt2Img,
		Origin:   model.AssetOriginGenerated,
		FilePath: filepath.Join("images", "missing", "no-file.png"),
	}
	if err := db.Create(&brokenAsset).Error; err != nil {
		t.Fatalf("create broken source asset: %v", err)
	}

	var nodeCountBefore int64
	var resultCountBefore int64
	var assetCountBefore int64
	if err := db.Model(&model.Node{}).Count(&nodeCountBefore).Error; err != nil {
		t.Fatalf("count nodes before: %v", err)
	}
	if err := db.Model(&model.Result{}).Count(&resultCountBefore).Error; err != nil {
		t.Fatalf("count results before: %v", err)
	}
	if err := db.Model(&model.Asset{}).Count(&assetCountBefore).Error; err != nil {
		t.Fatalf("count assets before: %v", err)
	}

	recorder := performJSONRequest(t, router, http.MethodPost, "/api/generate", map[string]any{
		"mode":                model.PromptModeImg2Img,
		"prompt":              "hello",
		"model":               "gemini-3-pro-image",
		"width":               512,
		"height":              512,
		"reference_image_ids": []string{brokenAsset.ID},
	})
	if recorder.Code != http.StatusInternalServerError {
		t.Fatalf("unexpected status: %d, body=%s", recorder.Code, recorder.Body.String())
	}

	var nodeCountAfter int64
	var resultCountAfter int64
	var assetCountAfter int64
	if err := db.Model(&model.Node{}).Count(&nodeCountAfter).Error; err != nil {
		t.Fatalf("count nodes after: %v", err)
	}
	if err := db.Model(&model.Result{}).Count(&resultCountAfter).Error; err != nil {
		t.Fatalf("count results after: %v", err)
	}
	if err := db.Model(&model.Asset{}).Count(&assetCountAfter).Error; err != nil {
		t.Fatalf("count assets after: %v", err)
	}

	if nodeCountAfter != nodeCountBefore {
		t.Fatalf("node count changed after failed copy: before=%d after=%d", nodeCountBefore, nodeCountAfter)
	}
	if resultCountAfter != resultCountBefore {
		t.Fatalf("result count changed after failed copy: before=%d after=%d", resultCountBefore, resultCountAfter)
	}
	if assetCountAfter != assetCountBefore {
		t.Fatalf("asset count changed after failed copy: before=%d after=%d", assetCountBefore, assetCountAfter)
	}
}

func TestRerollContract(t *testing.T) {
	router, db, _ := setupGenerateRouter(t)

	createRec := performJSONRequest(t, router, http.MethodPost, "/api/generate", map[string]any{
		"mode":   model.PromptModeTxt2Img,
		"prompt": "hello-reroll",
		"model":  "gemini-3-pro-image",
		"width":  512,
		"height": 512,
	})
	if createRec.Code != http.StatusOK {
		t.Fatalf("create node failed: %d body=%s", createRec.Code, createRec.Body.String())
	}
	var createPayload struct {
		Node model.Node `json:"node"`
	}
	if err := json.NewDecoder(createRec.Body).Decode(&createPayload); err != nil {
		t.Fatalf("decode create response: %v", err)
	}
	if createPayload.Node.ID == "" {
		t.Fatalf("expected generated node id")
	}

	var nodeCountBefore int64
	var resultCountBefore int64
	if err := db.Model(&model.Node{}).Count(&nodeCountBefore).Error; err != nil {
		t.Fatalf("count nodes before reroll: %v", err)
	}
	if err := db.Model(&model.Result{}).Where("node_id = ?", createPayload.Node.ID).Count(&resultCountBefore).Error; err != nil {
		t.Fatalf("count results before reroll: %v", err)
	}

	rerollRec := performJSONRequest(t, router, http.MethodPost, "/api/prompts/"+createPayload.Node.ID+"/reroll", map[string]any{
		"aspect_ratio": "16:9",
		"image_size":   "2K",
	})
	if rerollRec.Code != http.StatusOK {
		t.Fatalf("reroll failed: %d body=%s", rerollRec.Code, rerollRec.Body.String())
	}
	var rerollPayload struct {
		Node model.Node `json:"node"`
	}
	if err := json.NewDecoder(rerollRec.Body).Decode(&rerollPayload); err != nil {
		t.Fatalf("decode reroll response: %v", err)
	}
	if rerollPayload.Node.AspectRatio == nil || *rerollPayload.Node.AspectRatio != "16:9" {
		t.Fatalf("unexpected reroll aspect_ratio: %#v", rerollPayload.Node.AspectRatio)
	}
	if rerollPayload.Node.ImageSize == nil || *rerollPayload.Node.ImageSize != "2K" {
		t.Fatalf("unexpected reroll image_size: %#v", rerollPayload.Node.ImageSize)
	}

	var nodeCountAfter int64
	var resultCountAfter int64
	if err := db.Model(&model.Node{}).Count(&nodeCountAfter).Error; err != nil {
		t.Fatalf("count nodes after reroll: %v", err)
	}
	if err := db.Model(&model.Result{}).Where("node_id = ?", createPayload.Node.ID).Count(&resultCountAfter).Error; err != nil {
		t.Fatalf("count results after reroll: %v", err)
	}
	if nodeCountAfter != nodeCountBefore {
		t.Fatalf("reroll should not create new node: before=%d after=%d", nodeCountBefore, nodeCountAfter)
	}
	if resultCountAfter != resultCountBefore+1 {
		t.Fatalf("reroll should append result: before=%d after=%d", resultCountBefore, resultCountAfter)
	}

	invalidRec := performJSONRequest(t, router, http.MethodPost, "/api/prompts/"+createPayload.Node.ID+"/reroll", map[string]any{"prompt": "override", "model": "gpt-image-2"})
	if invalidRec.Code != http.StatusBadRequest {
		t.Fatalf("expected reroll override rejected, got: %d", invalidRec.Code)
	}

	invalidSizeRec := performJSONRequest(t, router, http.MethodPost, "/api/prompts/"+createPayload.Node.ID+"/reroll", map[string]any{"image_size": "3K"})
	if invalidSizeRec.Code != http.StatusBadRequest {
		t.Fatalf("expected reroll invalid image_size rejected, got: %d", invalidSizeRec.Code)
	}

	notFoundRec := performJSONRequest(t, router, http.MethodPost, "/api/prompts/not-exist/reroll", map[string]any{})
	if notFoundRec.Code != http.StatusNotFound {
		t.Fatalf("expected reroll not found, got: %d", notFoundRec.Code)
	}
	var notFoundPayload map[string]any
	if err := json.NewDecoder(notFoundRec.Body).Decode(&notFoundPayload); err != nil {
		t.Fatalf("decode reroll not found: %v", err)
	}
	if notFoundPayload["error"] != "not_found" || notFoundPayload["message"] != "node not found" {
		t.Fatalf("unexpected reroll not found payload: %#v", notFoundPayload)
	}
}

func validPromptCollectionState() model.PromptCollectionState {
	return model.PromptCollectionState{
		"groupOrder": []any{"g1"},
		"groups": map[string]any{
			"g1": map[string]any{
				"id":        "g1",
				"name":      "主体",
				"promptIds": []any{"p1"},
				"collapsed": false,
			},
		},
		"prompts": map[string]any{
			"p1": map[string]any{
				"id":        "p1",
				"content":   "subject",
				"createdAt": "2026-02-09T00:00:00Z",
				"updatedAt": "2026-02-09T00:00:01Z",
			},
		},
		"unknown_meta": map[string]any{"x": 1.0},
	}
}

func numericTimestampPromptCollectionState() model.PromptCollectionState {
	state := validPromptCollectionState()
	prompts := state["prompts"].(map[string]any)
	p1 := prompts["p1"].(map[string]any)
	p1["createdAt"] = 1730000000.0
	p1["updatedAt"] = 1730000001.0
	return state
}

func TestGeneratePersistsAndReturnsFragments(t *testing.T) {
	router, db, _ := setupGenerateRouter(t)
	router.GET("/api/nodes/:id", (&NodesHandler{DB: db}).Get)

	fragments := validPromptCollectionState()
	createRec := performJSONRequest(t, router, http.MethodPost, "/api/generate", map[string]any{
		"mode":      model.PromptModeTxt2Img,
		"prompt":    "plain prompt",
		"model":     "gemini-3-pro-image",
		"fragments": fragments,
		"width":     512,
		"height":    512,
	})
	if createRec.Code != http.StatusOK {
		t.Fatalf("create with fragments failed: %d body=%s", createRec.Code, createRec.Body.String())
	}

	var createPayload struct {
		Node model.Node `json:"node"`
	}
	if err := json.NewDecoder(createRec.Body).Decode(&createPayload); err != nil {
		t.Fatalf("decode create response: %v", err)
	}
	if createPayload.Node.Prompt != "plain prompt" {
		t.Fatalf("unexpected plain prompt: %q", createPayload.Node.Prompt)
	}
	if !reflect.DeepEqual(createPayload.Node.Fragments, fragments) {
		t.Fatalf("response fragments mismatch: got=%#v want=%#v", createPayload.Node.Fragments, fragments)
	}

	var persisted model.Node
	if err := db.First(&persisted, "id = ?", createPayload.Node.ID).Error; err != nil {
		t.Fatalf("load persisted node: %v", err)
	}
	if !reflect.DeepEqual(persisted.Fragments, fragments) {
		t.Fatalf("persisted fragments mismatch: got=%#v want=%#v", persisted.Fragments, fragments)
	}

	getRec := performJSONRequest(t, router, http.MethodGet, "/api/nodes/"+createPayload.Node.ID, nil)
	if getRec.Code != http.StatusOK {
		t.Fatalf("get node failed: %d body=%s", getRec.Code, getRec.Body.String())
	}
	var getPayload struct {
		Node model.Node `json:"node"`
	}
	if err := json.NewDecoder(getRec.Body).Decode(&getPayload); err != nil {
		t.Fatalf("decode get response: %v", err)
	}
	if !reflect.DeepEqual(getPayload.Node.Fragments, fragments) {
		t.Fatalf("get node fragments mismatch: got=%#v want=%#v", getPayload.Node.Fragments, fragments)
	}
}

func TestGenerateRejectsInvalidFragments(t *testing.T) {
	router, _, _ := setupGenerateRouter(t)

	validState := validPromptCollectionState()

	invalidCases := []map[string]any{
		{
			"mode":      model.PromptModeTxt2Img,
			"prompt":    "hello",
			"fragments": []any{123},
			"width":     512,
			"height":    512,
		},
		{
			"mode":   model.PromptModeTxt2Img,
			"prompt": "hello",
			"fragments": map[string]any{
				"groups":  validState["groups"],
				"prompts": validState["prompts"],
			},
			"width":  512,
			"height": 512,
		},
		{
			"mode":   model.PromptModeTxt2Img,
			"prompt": "hello",
			"fragments": map[string]any{
				"groupOrder": []any{"missing-group"},
				"groups":     validState["groups"],
				"prompts":    validState["prompts"],
			},
			"width":  512,
			"height": 512,
		},
		{
			"mode":   model.PromptModeTxt2Img,
			"prompt": "hello",
			"fragments": map[string]any{
				"groupOrder": []any{"g1"},
				"groups": map[string]any{
					"g1": map[string]any{
						"id":        "g1",
						"name":      "主体",
						"promptIds": []any{"missing-prompt"},
						"collapsed": false,
					},
				},
				"prompts": validState["prompts"],
			},
			"width":  512,
			"height": 512,
		},
	}

	for i, body := range invalidCases {
		t.Run(string(rune('a'+i)), func(t *testing.T) {
			recorder := performJSONRequest(t, router, http.MethodPost, "/api/generate", body)
			if recorder.Code != http.StatusBadRequest {
				t.Fatalf("expected invalid fragments rejected, got=%d body=%s", recorder.Code, recorder.Body.String())
			}
		})
	}

	validRec := performJSONRequest(t, router, http.MethodPost, "/api/generate", map[string]any{
		"mode":      model.PromptModeTxt2Img,
		"prompt":    "hello",
		"model":     "gemini-3-pro-image",
		"fragments": validState,
		"width":     512,
		"height":    512,
	})
	if validRec.Code != http.StatusOK {
		t.Fatalf("expected valid fragments accepted, got=%d body=%s", validRec.Code, validRec.Body.String())
	}

	numericRec := performJSONRequest(t, router, http.MethodPost, "/api/generate", map[string]any{
		"mode":      model.PromptModeTxt2Img,
		"prompt":    "hello-numeric-ts",
		"model":     "gemini-3-pro-image",
		"fragments": numericTimestampPromptCollectionState(),
		"width":     512,
		"height":    512,
	})
	if numericRec.Code != http.StatusOK {
		t.Fatalf("expected numeric timestamp fragments accepted, got=%d body=%s", numericRec.Code, numericRec.Body.String())
	}

	emptyTimestampState := validPromptCollectionState()
	emptyPrompts := emptyTimestampState["prompts"].(map[string]any)
	emptyPrompt := emptyPrompts["p1"].(map[string]any)
	emptyPrompt["createdAt"] = "   "

	emptyTimestampRec := performJSONRequest(t, router, http.MethodPost, "/api/generate", map[string]any{
		"mode":      model.PromptModeTxt2Img,
		"prompt":    "hello-empty-ts",
		"fragments": emptyTimestampState,
		"width":     512,
		"height":    512,
	})
	if emptyTimestampRec.Code != http.StatusBadRequest {
		t.Fatalf("expected empty string timestamp rejected, got=%d body=%s", emptyTimestampRec.Code, emptyTimestampRec.Body.String())
	}
}

func TestGenerateRerollKeepsFragments(t *testing.T) {
	router, db, _ := setupGenerateRouter(t)

	createRec := performJSONRequest(t, router, http.MethodPost, "/api/generate", map[string]any{
		"mode":      model.PromptModeTxt2Img,
		"prompt":    "hello-reroll",
		"model":     "gemini-3-pro-image",
		"fragments": validPromptCollectionState(),
		"width":     512,
		"height":    512,
	})
	if createRec.Code != http.StatusOK {
		t.Fatalf("create node failed: %d body=%s", createRec.Code, createRec.Body.String())
	}
	var createPayload struct {
		Node model.Node `json:"node"`
	}
	if err := json.NewDecoder(createRec.Body).Decode(&createPayload); err != nil {
		t.Fatalf("decode create response: %v", err)
	}

	rerollRec := performJSONRequest(t, router, http.MethodPost, "/api/prompts/"+createPayload.Node.ID+"/reroll", map[string]any{})
	if rerollRec.Code != http.StatusOK {
		t.Fatalf("reroll failed: %d body=%s", rerollRec.Code, rerollRec.Body.String())
	}
	var rerollPayload struct {
		Node model.Node `json:"node"`
	}
	if err := json.NewDecoder(rerollRec.Body).Decode(&rerollPayload); err != nil {
		t.Fatalf("decode reroll response: %v", err)
	}
	if !reflect.DeepEqual(rerollPayload.Node.Fragments, createPayload.Node.Fragments) {
		t.Fatalf("reroll fragments mismatch: got=%#v want=%#v", rerollPayload.Node.Fragments, createPayload.Node.Fragments)
	}

	var persisted model.Node
	if err := db.First(&persisted, "id = ?", createPayload.Node.ID).Error; err != nil {
		t.Fatalf("load reroll node: %v", err)
	}
	if !reflect.DeepEqual(persisted.Fragments, createPayload.Node.Fragments) {
		t.Fatalf("persisted reroll fragments mismatch: got=%#v want=%#v", persisted.Fragments, createPayload.Node.Fragments)
	}
}

// --- Group 4D: registry-driven validation + reroll model overrides ---

func baseGenerateBody() map[string]any {
	return map[string]any{
		"mode":   model.PromptModeTxt2Img,
		"prompt": "hello",
		"width":  512,
		"height": 512,
	}
}

func TestGenerate_missingModel_400invalid(t *testing.T) {
	router, _, _ := setupGenerateRouter(t)

	rec := performJSONRequest(t, router, http.MethodPost, "/api/generate", baseGenerateBody())
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 missing model, got: %d body=%s", rec.Code, rec.Body.String())
	}
	var payload map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload["error"] != "invalid" {
		t.Fatalf("expected error=invalid, got: %#v", payload["error"])
	}
}

func TestGenerate_unknownModel_400invalid(t *testing.T) {
	router, _, _ := setupGenerateRouter(t)
	body := baseGenerateBody()
	body["model"] = "never-heard-of"
	rec := performJSONRequest(t, router, http.MethodPost, "/api/generate", body)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 unknown model, got: %d body=%s", rec.Code, rec.Body.String())
	}
	var payload map[string]any
	_ = json.NewDecoder(rec.Body).Decode(&payload)
	if payload["error"] != "invalid" {
		t.Fatalf("expected error=invalid, got: %#v", payload["error"])
	}
}

func TestGenerate_disabledModel_400modelDisabled(t *testing.T) {
	router, _, _ := setupGenerateRouter(t)
	body := baseGenerateBody()
	body["model"] = "disabled-model"
	rec := performJSONRequest(t, router, http.MethodPost, "/api/generate", body)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 disabled model, got: %d body=%s", rec.Code, rec.Body.String())
	}
	var payload map[string]any
	_ = json.NewDecoder(rec.Body).Decode(&payload)
	if payload["error"] != "model_disabled" {
		t.Fatalf("expected error=model_disabled, got: %#v", payload["error"])
	}
}

func TestGenerate_sizeNotInModelAllowlist_400invalid(t *testing.T) {
	router, _, _ := setupGenerateRouter(t)
	body := baseGenerateBody()
	body["model"] = "gpt-image-2"
	body["image_size"] = "4K" // gpt-image-2 only allows 1K/2K
	rec := performJSONRequest(t, router, http.MethodPost, "/api/generate", body)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 size not in allowlist, got: %d body=%s", rec.Code, rec.Body.String())
	}
	var payload map[string]any
	_ = json.NewDecoder(rec.Body).Decode(&payload)
	if payload["error"] != "invalid" {
		t.Fatalf("expected error=invalid, got: %#v", payload["error"])
	}
}

func TestGenerate_ratioNotInModelAllowlist_400invalid(t *testing.T) {
	router, _, _ := setupGenerateRouter(t)
	body := baseGenerateBody()
	body["model"] = "gpt-image-2"
	body["aspect_ratio"] = "21:9" // only gemini allows 21:9
	rec := performJSONRequest(t, router, http.MethodPost, "/api/generate", body)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 ratio not in allowlist, got: %d body=%s", rec.Code, rec.Body.String())
	}
	var payload map[string]any
	_ = json.NewDecoder(rec.Body).Decode(&payload)
	if payload["error"] != "invalid" {
		t.Fatalf("expected error=invalid, got: %#v", payload["error"])
	}
}

func TestGenerate_validRequest_200_withModelIDPersisted(t *testing.T) {
	router, db, _, recorder := setupGenerateRouterWithRecorder(t)

	body := baseGenerateBody()
	body["model"] = "gpt-image-2"
	rec := performJSONRequest(t, router, http.MethodPost, "/api/generate", body)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got: %d body=%s", rec.Code, rec.Body.String())
	}

	var payload struct {
		Node model.Node `json:"node"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload.Node.ModelID != "gpt-image-2" {
		t.Fatalf("expected response node.model_id=gpt-image-2, got: %q", payload.Node.ModelID)
	}

	var persisted model.Node
	if err := db.First(&persisted, "id = ?", payload.Node.ID).Error; err != nil {
		t.Fatalf("load persisted node: %v", err)
	}
	if persisted.ModelID != "gpt-image-2" {
		t.Fatalf("expected persisted model_id=gpt-image-2, got: %q", persisted.ModelID)
	}

	proxyBody := recorder.last()
	if proxyBody == nil {
		t.Fatalf("expected proxy to have received a request body")
	}
	if proxyBody["model"] != "gpt-image-2" {
		t.Fatalf("expected proxy body.model=gpt-image-2, got: %#v", proxyBody["model"])
	}
}

func seedNodeForReroll(t *testing.T, router *gin.Engine, modelID string) string {
	t.Helper()
	body := baseGenerateBody()
	body["model"] = modelID
	body["prompt"] = "seed-" + modelID
	rec := performJSONRequest(t, router, http.MethodPost, "/api/generate", body)
	if rec.Code != http.StatusOK {
		t.Fatalf("seed node failed: %d body=%s", rec.Code, rec.Body.String())
	}
	var payload struct {
		Node model.Node `json:"node"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&payload); err != nil {
		t.Fatalf("decode seed response: %v", err)
	}
	return payload.Node.ID
}

func TestReroll_withoutModel_usesNodeModelID(t *testing.T) {
	router, _, _, recorder := setupGenerateRouterWithRecorder(t)

	nodeID := seedNodeForReroll(t, router, "gpt-image-2")

	rec := performJSONRequest(t, router, http.MethodPost, "/api/prompts/"+nodeID+"/reroll", map[string]any{})
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got: %d body=%s", rec.Code, rec.Body.String())
	}
	proxyBody := recorder.last()
	if proxyBody == nil {
		t.Fatalf("expected proxy request recorded")
	}
	if proxyBody["model"] != "gpt-image-2" {
		t.Fatalf("expected proxy body.model=gpt-image-2, got: %#v", proxyBody["model"])
	}
}

func TestReroll_withValidModel_writesBackAndOverrides(t *testing.T) {
	router, db, _, recorder := setupGenerateRouterWithRecorder(t)

	nodeID := seedNodeForReroll(t, router, "gemini-3-pro-image")

	rec := performJSONRequest(t, router, http.MethodPost, "/api/prompts/"+nodeID+"/reroll", map[string]any{
		"model": "gpt-image-2",
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got: %d body=%s", rec.Code, rec.Body.String())
	}
	proxyBody := recorder.last()
	if proxyBody == nil {
		t.Fatalf("expected proxy request recorded")
	}
	if proxyBody["model"] != "gpt-image-2" {
		t.Fatalf("expected proxy body.model=gpt-image-2 after override, got: %#v", proxyBody["model"])
	}

	var persisted model.Node
	if err := db.First(&persisted, "id = ?", nodeID).Error; err != nil {
		t.Fatalf("load persisted node: %v", err)
	}
	if persisted.ModelID != "gpt-image-2" {
		t.Fatalf("expected persisted model_id=gpt-image-2 after reroll override, got: %q", persisted.ModelID)
	}
}

func TestReroll_withUnknownModel_400invalid(t *testing.T) {
	router, _, _ := setupGenerateRouter(t)
	nodeID := seedNodeForReroll(t, router, "gpt-image-2")

	rec := performJSONRequest(t, router, http.MethodPost, "/api/prompts/"+nodeID+"/reroll", map[string]any{
		"model": "never-heard-of",
	})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got: %d body=%s", rec.Code, rec.Body.String())
	}
	var payload map[string]any
	_ = json.NewDecoder(rec.Body).Decode(&payload)
	if payload["error"] != "invalid" {
		t.Fatalf("expected error=invalid, got: %#v", payload["error"])
	}
}

func TestReroll_withDisabledModel_400modelDisabled(t *testing.T) {
	router, _, _ := setupGenerateRouter(t)
	nodeID := seedNodeForReroll(t, router, "gpt-image-2")

	rec := performJSONRequest(t, router, http.MethodPost, "/api/prompts/"+nodeID+"/reroll", map[string]any{
		"model": "disabled-model",
	})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got: %d body=%s", rec.Code, rec.Body.String())
	}
	var payload map[string]any
	_ = json.NewDecoder(rec.Body).Decode(&payload)
	if payload["error"] != "model_disabled" {
		t.Fatalf("expected error=model_disabled, got: %#v", payload["error"])
	}
}

func TestReroll_withBadCombo_400invalid(t *testing.T) {
	router, _, _ := setupGenerateRouter(t)
	nodeID := seedNodeForReroll(t, router, "gpt-image-2")

	// gpt-image-2 does not allow 4K
	rec := performJSONRequest(t, router, http.MethodPost, "/api/prompts/"+nodeID+"/reroll", map[string]any{
		"model":      "gpt-image-2",
		"image_size": "4K",
	})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got: %d body=%s", rec.Code, rec.Body.String())
	}
	var payload map[string]any
	_ = json.NewDecoder(rec.Body).Decode(&payload)
	if payload["error"] != "invalid" {
		t.Fatalf("expected error=invalid, got: %#v", payload["error"])
	}
}

func TestNodesAPI_legacyDisabledModel_stillReadable(t *testing.T) {
	// Seed a PromptNode with model_id for a disabled/legacy model and ensure
	// /api/nodes/:id still returns it with the model_id intact.
	router, db, _ := setupGenerateRouter(t)
	router.GET("/api/nodes/:id", (&NodesHandler{DB: db}).Get)

	legacyNode := model.PromptNode{
		ID:         "legacy-node-1",
		Mode:       model.PromptModeTxt2Img,
		Prompt:     "legacy",
		ModelID:    "disabled-model",
		Title:      "legacy",
		LastUsedAt: mustParseRFC3339(t, "2026-02-08T00:00:00Z"),
	}
	if err := db.Create(&legacyNode).Error; err != nil {
		t.Fatalf("create legacy node: %v", err)
	}

	rec := performJSONRequest(t, router, http.MethodGet, "/api/nodes/"+legacyNode.ID, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 for legacy disabled node, got: %d body=%s", rec.Code, rec.Body.String())
	}
	var payload struct {
		Node map[string]any `json:"node"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload.Node["model_id"] != "disabled-model" {
		t.Fatalf("expected node.model_id=disabled-model, got: %#v", payload.Node["model_id"])
	}
}
