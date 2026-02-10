package api

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"strings"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"banana-lab/internal/model"
)

type PromptSnippetsHandler struct {
	DB *gorm.DB
}

type promptSnippetCreateRequest struct {
	ID               *string               `json:"id"`
	Name             string                `json:"name"`
	ContentFragments model.PromptFragments `json:"content_fragments"`
}

type promptSnippetUpdateRequest struct {
	Name             *string                `json:"name"`
	ContentFragments *model.PromptFragments `json:"content_fragments"`
}

type promptSnippetReplaceItem struct {
	ID               string                `json:"id"`
	Name             string                `json:"name"`
	ContentFragments model.PromptFragments `json:"content_fragments"`
}

func (h *PromptSnippetsHandler) Create(c *gin.Context) {
	if h == nil || h.DB == nil {
		c.JSON(500, gin.H{"error": "internal"})
		return
	}

	var req promptSnippetCreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": "invalid"})
		return
	}

	name := strings.TrimSpace(req.Name)
	if name == "" {
		c.JSON(400, gin.H{"error": "invalid"})
		return
	}

	if req.ContentFragments == nil {
		c.JSON(400, gin.H{"error": "invalid"})
		return
	}

	var id string
	if req.ID != nil {
		id = strings.TrimSpace(*req.ID)
		if id == "" {
			c.JSON(400, gin.H{"error": "invalid"})
			return
		}

		var existing model.PromptSnippet
		if err := h.DB.Where("id = ?", id).First(&existing).Error; err == nil {
			c.JSON(400, gin.H{"error": "invalid"})
			return
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(500, gin.H{"error": "internal"})
			return
		}
	} else {
		generatedID, err := newPromptSnippetID()
		if err != nil {
			c.JSON(500, gin.H{"error": "internal"})
			return
		}
		id = generatedID
	}

	snippet := model.PromptSnippet{
		ID:               id,
		Name:             name,
		ContentFragments: req.ContentFragments,
	}
	if err := h.DB.Create(&snippet).Error; err != nil {
		c.JSON(500, gin.H{"error": "internal"})
		return
	}

	c.JSON(201, snippet)
}

func (h *PromptSnippetsHandler) List(c *gin.Context) {
	if h == nil || h.DB == nil {
		c.JSON(500, gin.H{"error": "internal"})
		return
	}

	var snippets []model.PromptSnippet
	if err := h.DB.
		Order("updated_at desc").
		Order("created_at desc").
		Find(&snippets).Error; err != nil {
		c.JSON(500, gin.H{"error": "internal"})
		return
	}

	c.JSON(200, snippets)
}

func (h *PromptSnippetsHandler) Update(c *gin.Context) {
	if h == nil || h.DB == nil {
		c.JSON(500, gin.H{"error": "internal"})
		return
	}

	id := strings.TrimSpace(c.Param("id"))
	if id == "" {
		c.JSON(400, gin.H{"error": "invalid"})
		return
	}

	var req promptSnippetUpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": "invalid"})
		return
	}

	var existing model.PromptSnippet
	if err := h.DB.Where("id = ?", id).First(&existing).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			snippetNotFoundResponse(c)
			return
		}
		c.JSON(500, gin.H{"error": "internal"})
		return
	}

	updates := map[string]any{}
	if req.Name != nil {
		name := strings.TrimSpace(*req.Name)
		if name == "" {
			c.JSON(400, gin.H{"error": "invalid"})
			return
		}
		updates["name"] = name
	}
	if req.ContentFragments != nil {
		updates["content_fragments"] = *req.ContentFragments
	}

	if len(updates) > 0 {
		if err := h.DB.Model(&model.PromptSnippet{}).Where("id = ?", id).Updates(updates).Error; err != nil {
			c.JSON(500, gin.H{"error": "internal"})
			return
		}
	}

	if err := h.DB.Where("id = ?", id).First(&existing).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			snippetNotFoundResponse(c)
			return
		}
		c.JSON(500, gin.H{"error": "internal"})
		return
	}

	c.JSON(200, existing)
}

func (h *PromptSnippetsHandler) Delete(c *gin.Context) {
	if h == nil || h.DB == nil {
		c.JSON(500, gin.H{"error": "internal"})
		return
	}

	id := strings.TrimSpace(c.Param("id"))
	if id == "" {
		c.JSON(400, gin.H{"error": "invalid"})
		return
	}

	var snippet model.PromptSnippet
	if err := h.DB.Where("id = ?", id).First(&snippet).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			snippetNotFoundResponse(c)
			return
		}
		c.JSON(500, gin.H{"error": "internal"})
		return
	}

	if err := h.DB.Delete(&model.PromptSnippet{}, "id = ?", id).Error; err != nil {
		c.JSON(500, gin.H{"error": "internal"})
		return
	}

	c.Status(204)
}

func (h *PromptSnippetsHandler) ReplaceAll(c *gin.Context) {
	if h == nil || h.DB == nil {
		c.JSON(500, gin.H{"error": "internal"})
		return
	}

	var req []promptSnippetReplaceItem
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": "invalid"})
		return
	}

	seen := make(map[string]struct{}, len(req))
	for _, item := range req {
		id := strings.TrimSpace(item.ID)
		name := strings.TrimSpace(item.Name)
		if id == "" || name == "" || item.ContentFragments == nil {
			c.JSON(400, gin.H{"error": "invalid"})
			return
		}
		if _, ok := seen[id]; ok {
			c.JSON(400, gin.H{"error": "invalid"})
			return
		}
		seen[id] = struct{}{}
	}

	next := make([]model.PromptSnippet, 0, len(req))
	for _, item := range req {
		next = append(next, model.PromptSnippet{
			ID:               strings.TrimSpace(item.ID),
			Name:             strings.TrimSpace(item.Name),
			ContentFragments: item.ContentFragments,
		})
	}

	if err := h.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Session(&gorm.Session{AllowGlobalUpdate: true}).Delete(&model.PromptSnippet{}).Error; err != nil {
			return err
		}
		if len(next) == 0 {
			return nil
		}
		return tx.Create(&next).Error
	}); err != nil {
		c.JSON(500, gin.H{"error": "internal"})
		return
	}

	var persisted []model.PromptSnippet
	if err := h.DB.Order("created_at asc").Find(&persisted).Error; err != nil {
		c.JSON(500, gin.H{"error": "internal"})
		return
	}

	c.JSON(200, persisted)
}

func snippetNotFoundResponse(c *gin.Context) {
	c.JSON(404, gin.H{"error": "not_found", "message": "snippet not found"})
}

func newPromptSnippetID() (string, error) {
	bytes := make([]byte, 16)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}
