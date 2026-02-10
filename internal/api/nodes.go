package api

import (
	"errors"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"banana-lab/internal/model"
)

type NodesHandler struct {
	DB *gorm.DB
}

func (h *NodesHandler) Get(c *gin.Context) {
	if h == nil || h.DB == nil {
		c.JSON(500, gin.H{"error": "internal"})
		return
	}

	id := strings.TrimSpace(c.Param("id"))
	if id == "" {
		c.JSON(400, gin.H{"error": "invalid"})
		return
	}

	var node model.Node
	if err := h.DB.Where("id = ? AND deleted_at IS NULL", id).First(&node).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			nodeNotFoundResponse(c)
			return
		}
		c.JSON(500, gin.H{"error": "internal"})
		return
	}

	var images []model.Image
	if err := h.DB.Where("node_id = ?", node.ID).Order("created_at desc").Find(&images).Error; err != nil {
		c.JSON(500, gin.H{"error": "internal"})
		return
	}

	var results []model.Result
	if err := h.DB.Where("node_id = ? AND status = ?", node.ID, model.ResultStatusActive).Order("created_at desc").Find(&results).Error; err != nil {
		c.JSON(500, gin.H{"error": "internal"})
		return
	}

	c.JSON(200, gin.H{"node": node, "images": images, "results": results})
}

func (h *NodesHandler) Delete(c *gin.Context) {
	if h == nil || h.DB == nil {
		c.JSON(500, gin.H{"error": "internal"})
		return
	}

	id := strings.TrimSpace(c.Param("id"))
	if id == "" {
		c.JSON(400, gin.H{"error": "invalid"})
		return
	}

	var (
		parentID   *string
		deletedAt  time.Time
		reparented int64
	)

	err := h.DB.Transaction(func(tx *gorm.DB) error {
		var node model.Node
		if err := tx.Where("id = ? AND deleted_at IS NULL", id).First(&node).Error; err != nil {
			return err
		}

		now := time.Now()
		resultUpdates := map[string]any{"status": model.ResultStatusDeleted, "deleted_at": now, "archived_at": nil}
		if err := tx.Model(&model.Result{}).Where("node_id = ?", node.ID).Updates(resultUpdates).Error; err != nil {
			return err
		}

		reparentUpdate := tx.Model(&model.Node{}).Where("parent_id = ? AND deleted_at IS NULL", node.ID).Update("parent_id", node.ParentID)
		if reparentUpdate.Error != nil {
			return reparentUpdate.Error
		}
		reparented = reparentUpdate.RowsAffected

		nodeUpdate := tx.Model(&model.Node{}).Where("id = ?", node.ID).Update("deleted_at", now)
		if nodeUpdate.Error != nil {
			return nodeUpdate.Error
		}
		if nodeUpdate.RowsAffected == 0 {
			return gorm.ErrRecordNotFound
		}

		parentID = node.ParentID
		deletedAt = now
		return nil
	})
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			nodeNotFoundResponse(c)
			return
		}
		c.JSON(500, gin.H{"error": "internal"})
		return
	}

	c.JSON(200, gin.H{
		"id":                  id,
		"parent_node_id":      parentID,
		"reparented_children": reparented,
		"deleted_at":          deletedAt,
	})
}
