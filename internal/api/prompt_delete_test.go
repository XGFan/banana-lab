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

type promptDeleteFixture struct {
	RootNodeID        string
	TargetNodeID      string
	ChildAlphaNodeID  string
	ChildBetaNodeID   string
	GrandchildNodeID  string
	TargetResultAID   string
	TargetResultBID   string
	ChildResultActive string
}

func setupPromptDeleteRouter(t *testing.T) (*gin.Engine, *gorm.DB, promptDeleteFixture) {
	t.Helper()

	gin.SetMode(gin.TestMode)
	dbPath := filepath.Join(t.TempDir(), "prompt-delete.db")
	db, err := model.Open(dbPath)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := model.Migrate(db); err != nil {
		t.Fatalf("migrate db: %v", err)
	}

	fixture := promptDeleteFixture{
		RootNodeID:        "node-root",
		TargetNodeID:      "node-target",
		ChildAlphaNodeID:  "node-child-alpha",
		ChildBetaNodeID:   "node-child-beta",
		GrandchildNodeID:  "node-grandchild",
		TargetResultAID:   "result-target-a",
		TargetResultBID:   "result-target-b",
		ChildResultActive: "result-child-active",
	}

	base := time.Now().UTC()
	targetParentID := fixture.RootNodeID
	childAlphaParentID := fixture.TargetNodeID
	childBetaParentID := fixture.TargetNodeID
	grandchildParentID := fixture.ChildAlphaNodeID
	nodes := []model.Node{
		{ID: fixture.RootNodeID, Mode: model.PromptModeTxt2Img, Prompt: "root", Title: "root", LastUsedAt: base, CreatedAt: base},
		{ID: fixture.TargetNodeID, Mode: model.PromptModeTxt2Img, ParentID: &targetParentID, Prompt: "target", Title: "target", LastUsedAt: base.Add(time.Second), CreatedAt: base.Add(time.Second)},
		{ID: fixture.ChildAlphaNodeID, Mode: model.PromptModeTxt2Img, ParentID: &childAlphaParentID, Prompt: "child-alpha", Title: "child-alpha", LastUsedAt: base.Add(2 * time.Second), CreatedAt: base.Add(2 * time.Second)},
		{ID: fixture.ChildBetaNodeID, Mode: model.PromptModeTxt2Img, ParentID: &childBetaParentID, Prompt: "child-beta", Title: "child-beta", LastUsedAt: base.Add(3 * time.Second), CreatedAt: base.Add(3 * time.Second)},
		{ID: fixture.GrandchildNodeID, Mode: model.PromptModeTxt2Img, ParentID: &grandchildParentID, Prompt: "grandchild", Title: "grandchild", LastUsedAt: base.Add(4 * time.Second), CreatedAt: base.Add(4 * time.Second)},
	}
	for _, node := range nodes {
		if err := db.Create(&node).Error; err != nil {
			t.Fatalf("create node %s: %v", node.ID, err)
		}
	}

	assets := []model.Asset{
		{ID: "asset-target-a", Domain: model.AssetDomainTxt2Img, FilePath: "images/txt2img/node-target/a.png", Origin: model.AssetOriginGenerated},
		{ID: "asset-target-b", Domain: model.AssetDomainTxt2Img, FilePath: "images/txt2img/node-target/b.png", Origin: model.AssetOriginGenerated},
		{ID: "asset-child-active", Domain: model.AssetDomainTxt2Img, FilePath: "images/txt2img/node-child-alpha/c.png", Origin: model.AssetOriginGenerated},
	}
	for _, asset := range assets {
		if err := db.Create(&asset).Error; err != nil {
			t.Fatalf("create asset %s: %v", asset.ID, err)
		}
	}

	targetArchivedAt := base.Add(-time.Hour)
	results := []model.Result{
		{ID: fixture.TargetResultAID, NodeID: fixture.TargetNodeID, AssetID: "asset-target-a", Status: model.ResultStatusActive},
		{ID: fixture.TargetResultBID, NodeID: fixture.TargetNodeID, AssetID: "asset-target-b", Status: model.ResultStatusArchived, ArchivedAt: &targetArchivedAt},
		{ID: fixture.ChildResultActive, NodeID: fixture.ChildAlphaNodeID, AssetID: "asset-child-active", Status: model.ResultStatusActive},
	}
	for _, result := range results {
		if err := db.Create(&result).Error; err != nil {
			t.Fatalf("create result %s: %v", result.ID, err)
		}
	}

	router := gin.New()
	nodesHandler := &NodesHandler{DB: db}
	router.DELETE("/api/prompts/:id", nodesHandler.Delete)
	router.GET("/api/nodes/:id", nodesHandler.Get)
	router.GET("/api/prompts/:id/tree", (&PromptTreeHandler{DB: db}).Get)

	return router, db, fixture
}

func TestPromptDeleteReparent(t *testing.T) {
	router, db, fixture := setupPromptDeleteRouter(t)

	rec := performJSONRequest(t, router, http.MethodDelete, "/api/prompts/"+fixture.TargetNodeID, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("unexpected status: %d, body=%s", rec.Code, rec.Body.String())
	}

	var payload map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&payload); err != nil {
		t.Fatalf("decode delete response: %v", err)
	}
	if payload["parent_node_id"] != fixture.RootNodeID {
		t.Fatalf("unexpected parent_node_id: %#v", payload["parent_node_id"])
	}
	if payload["reparented_children"] != float64(2) {
		t.Fatalf("unexpected reparented_children: %#v", payload["reparented_children"])
	}

	var deletedNode model.Node
	if err := db.First(&deletedNode, "id = ?", fixture.TargetNodeID).Error; err != nil {
		t.Fatalf("reload deleted node: %v", err)
	}
	if deletedNode.DeletedAt == nil {
		t.Fatalf("expected deleted node to have deleted_at")
	}

	var childAlpha model.Node
	if err := db.First(&childAlpha, "id = ?", fixture.ChildAlphaNodeID).Error; err != nil {
		t.Fatalf("reload child alpha: %v", err)
	}
	if childAlpha.ParentID == nil || *childAlpha.ParentID != fixture.RootNodeID {
		t.Fatalf("child alpha not reparented to root: %#v", childAlpha.ParentID)
	}

	var childBeta model.Node
	if err := db.First(&childBeta, "id = ?", fixture.ChildBetaNodeID).Error; err != nil {
		t.Fatalf("reload child beta: %v", err)
	}
	if childBeta.ParentID == nil || *childBeta.ParentID != fixture.RootNodeID {
		t.Fatalf("child beta not reparented to root: %#v", childBeta.ParentID)
	}

	var targetResults []model.Result
	if err := db.Where("node_id = ?", fixture.TargetNodeID).Order("id asc").Find(&targetResults).Error; err != nil {
		t.Fatalf("query target results: %v", err)
	}
	for _, result := range targetResults {
		if result.Status != model.ResultStatusDeleted || result.DeletedAt == nil {
			t.Fatalf("target result must be deleted, got: %#v", result)
		}
	}

	var childResult model.Result
	if err := db.First(&childResult, "id = ?", fixture.ChildResultActive).Error; err != nil {
		t.Fatalf("reload child result: %v", err)
	}
	if childResult.Status != model.ResultStatusActive {
		t.Fatalf("child result should remain active, got: %s", childResult.Status)
	}

	treeRec := performJSONRequest(t, router, http.MethodGet, "/api/prompts/"+fixture.RootNodeID+"/tree", nil)
	if treeRec.Code != http.StatusOK {
		t.Fatalf("unexpected tree status: %d", treeRec.Code)
	}
	var treePayload struct {
		Nodes []model.Node `json:"nodes"`
	}
	if err := json.NewDecoder(treeRec.Body).Decode(&treePayload); err != nil {
		t.Fatalf("decode tree response: %v", err)
	}
	for _, node := range treePayload.Nodes {
		if node.ID == fixture.TargetNodeID {
			t.Fatalf("deleted node should not appear in tree")
		}
	}

	nodeRec := performJSONRequest(t, router, http.MethodGet, "/api/nodes/"+fixture.TargetNodeID, nil)
	if nodeRec.Code != http.StatusNotFound {
		t.Fatalf("expected deleted node to return 404, got: %d", nodeRec.Code)
	}
	var nodePayload map[string]any
	if err := json.NewDecoder(nodeRec.Body).Decode(&nodePayload); err != nil {
		t.Fatalf("decode node response: %v", err)
	}
	if nodePayload["error"] != "not_found" || nodePayload["message"] != "node not found" {
		t.Fatalf("unexpected node not found payload: %#v", nodePayload)
	}
}

func TestPromptDeleteRootAllowed(t *testing.T) {
	router, db, fixture := setupPromptDeleteRouter(t)

	rec := performJSONRequest(t, router, http.MethodDelete, "/api/prompts/"+fixture.RootNodeID, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("unexpected status: %d body=%s", rec.Code, rec.Body.String())
	}

	var child model.Node
	if err := db.First(&child, "id = ?", fixture.TargetNodeID).Error; err != nil {
		t.Fatalf("reload child: %v", err)
	}
	if child.ParentID != nil {
		t.Fatalf("expected child parent to become nil after root delete, got: %#v", child.ParentID)
	}
}
