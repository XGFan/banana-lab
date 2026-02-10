package api

import (
	"errors"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"banana-lab/internal/model"
)

type PromptsHandler struct {
	DB *gorm.DB
}

type promptUpdateRequest struct {
	Title     *string `json:"title"`
	Pinned    *bool   `json:"pinned"`
	SortOrder *int    `json:"sort_order"`
	Archived  *bool   `json:"archived"`
}

func (h *PromptsHandler) List(c *gin.Context) {
	if h == nil || h.DB == nil {
		c.JSON(500, gin.H{"error": "internal"})
		return
	}

	mode := strings.TrimSpace(c.Query("mode"))
	if mode != "" && mode != model.PromptModeTxt2Img && mode != model.PromptModeImg2Img {
		c.JSON(400, gin.H{"error": "invalid"})
		return
	}

	limit, offset, ok := parsePromptPagination(c.Query("limit"), c.Query("offset"))
	if !ok {
		c.JSON(400, gin.H{"error": "invalid"})
		return
	}

	includeArchived, ok := parseIncludeArchived(c.Query("include_archived"))
	if !ok {
		c.JSON(400, gin.H{"error": "invalid"})
		return
	}

	query := h.DB.Model(&model.PromptNode{}).Where("deleted_at IS NULL")
	if mode != "" {
		query = query.Where("mode = ?", mode)
	}
	if !includeArchived {
		query = query.Where("archived_at IS NULL")
	}

	var prompts []model.PromptNode
	if err := query.
		Order("pinned desc").
		Order("last_used_at desc").
		Order("created_at desc").
		Limit(limit).
		Offset(offset).
		Find(&prompts).Error; err != nil {
		c.JSON(500, gin.H{"error": "internal"})
		return
	}

	c.JSON(200, prompts)
}

func (h *PromptsHandler) Update(c *gin.Context) {
	if h == nil || h.DB == nil {
		c.JSON(500, gin.H{"error": "internal"})
		return
	}

	id := strings.TrimSpace(c.Param("id"))
	if id == "" {
		c.JSON(400, gin.H{"error": "invalid"})
		return
	}

	var req promptUpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": "invalid"})
		return
	}

	var node model.PromptNode
	if err := h.DB.Where("id = ? AND deleted_at IS NULL", id).First(&node).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			nodeNotFoundResponse(c)
			return
		}
		c.JSON(500, gin.H{"error": "internal"})
		return
	}

	updates := map[string]any{}
	if req.Title != nil {
		title := strings.TrimSpace(*req.Title)
		if title == "" {
			c.JSON(400, gin.H{"error": "invalid"})
			return
		}
		updates["title"] = title
	}
	if req.Pinned != nil {
		updates["pinned"] = *req.Pinned
	}
	if req.SortOrder != nil {
		updates["sort_order"] = req.SortOrder
	}
	if req.Archived != nil {
		if *req.Archived {
			now := time.Now().UTC()
			updates["archived_at"] = &now
		} else {
			updates["archived_at"] = nil
		}
	}

	if len(updates) > 0 {
		if err := h.DB.Model(&model.PromptNode{}).Where("id = ? AND deleted_at IS NULL", id).Updates(updates).Error; err != nil {
			c.JSON(500, gin.H{"error": "internal"})
			return
		}
	}

	if err := h.DB.Where("id = ? AND deleted_at IS NULL", id).First(&node).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			nodeNotFoundResponse(c)
			return
		}
		c.JSON(500, gin.H{"error": "internal"})
		return
	}

	c.JSON(200, node)
}

func parsePromptPagination(limitRaw, offsetRaw string) (int, int, bool) {
	limit := 20
	offset := 0

	if strings.TrimSpace(limitRaw) != "" {
		parsed, err := strconv.Atoi(limitRaw)
		if err != nil || parsed < 1 || parsed > 100 {
			return 0, 0, false
		}
		limit = parsed
	}

	if strings.TrimSpace(offsetRaw) != "" {
		parsed, err := strconv.Atoi(offsetRaw)
		if err != nil || parsed < 0 {
			return 0, 0, false
		}
		offset = parsed
	}

	return limit, offset, true
}

func parseIncludeArchived(raw string) (bool, bool) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return false, true
	}
	parsed, err := strconv.ParseBool(trimmed)
	if err != nil {
		return false, false
	}
	return parsed, true
}
