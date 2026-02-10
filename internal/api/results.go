package api

import (
	"errors"
	"log"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"banana-lab/internal/model"
)

type ResultsHandler struct {
	DB *gorm.DB
}

func (h *ResultsHandler) Delete(c *gin.Context) {
	if h == nil || h.DB == nil {
		c.JSON(500, gin.H{"error": "internal"})
		return
	}

	id := strings.TrimSpace(c.Param("id"))
	if id == "" {
		c.JSON(400, gin.H{"error": "invalid"})
		return
	}

	var result model.Result
	if err := h.DB.First(&result, "id = ?", id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(404, gin.H{"error": "not_found"})
			return
		}
		c.JSON(500, gin.H{"error": "internal"})
		return
	}

	now := time.Now()
	updates := map[string]any{
		"status": model.ResultStatusDeleted,
	}
	if result.DeletedAt == nil {
		updates["deleted_at"] = now
	}

	if err := h.DB.Model(&model.Result{}).Where("id = ?", result.ID).Updates(updates).Error; err != nil {
		c.JSON(500, gin.H{"error": "internal"})
		return
	}

	var updated model.Result
	if err := h.DB.First(&updated, "id = ?", result.ID).Error; err != nil {
		c.JSON(500, gin.H{"error": "internal"})
		return
	}

	deletedAtValue := ""
	if updated.DeletedAt != nil {
		deletedAtValue = updated.DeletedAt.UTC().Format(time.RFC3339Nano)
	}
	log.Printf("result_soft_delete result_id=%s node_id=%s status=%s deleted_at=%s", updated.ID, updated.NodeID, updated.Status, deletedAtValue)

	c.JSON(200, gin.H{
		"id":         updated.ID,
		"node_id":    updated.NodeID,
		"status":     updated.Status,
		"deleted_at": updated.DeletedAt,
	})
}
