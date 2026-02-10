package api

import (
	"strings"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"banana-lab/internal/model"
)

type PromptTreeHandler struct {
	DB *gorm.DB
}

func (h *PromptTreeHandler) Get(c *gin.Context) {
	if h == nil || h.DB == nil {
		c.JSON(500, gin.H{"error": "internal"})
		return
	}

	nodeID := strings.TrimSpace(c.Param("id"))
	if nodeID == "" {
		c.JSON(400, gin.H{"error": "invalid"})
		return
	}

	var activeNodes []model.PromptNode
	if err := h.DB.Where("deleted_at IS NULL").Order("created_at asc").Find(&activeNodes).Error; err != nil {
		c.JSON(500, gin.H{"error": "internal"})
		return
	}

	nodeMap := make(map[string]model.PromptNode, len(activeNodes))
	childrenMap := make(map[string][]string)
	for _, node := range activeNodes {
		nodeMap[node.ID] = node
		if node.ParentID != nil {
			childrenMap[*node.ParentID] = append(childrenMap[*node.ParentID], node.ID)
		}
	}

	startNode, exists := nodeMap[nodeID]
	if !exists {
		nodeNotFoundResponse(c)
		return
	}

	rootID := startNode.ID
	for current := startNode; current.ParentID != nil; {
		parent, ok := nodeMap[*current.ParentID]
		if !ok {
			nodeNotFoundResponse(c)
			return
		}
		rootID = parent.ID
		current = parent
	}

	treeSet := map[string]struct{}{rootID: {}}
	queue := []string{rootID}
	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]
		for _, childID := range childrenMap[current] {
			if _, seen := treeSet[childID]; seen {
				continue
			}
			treeSet[childID] = struct{}{}
			queue = append(queue, childID)
		}
	}

	treeNodes := make([]model.PromptNode, 0, len(treeSet))
	for _, node := range activeNodes {
		if _, ok := treeSet[node.ID]; ok {
			treeNodes = append(treeNodes, node)
		}
	}

	c.JSON(200, gin.H{"root_id": rootID, "nodes": treeNodes})
}
