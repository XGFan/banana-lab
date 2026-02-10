package api

import (
	"encoding/json"
	"errors"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"

	"banana-lab/internal/model"
	"banana-lab/internal/service"
)

type GenerateHandler struct {
	Service *service.GenerateService
}

type generateRequest struct {
	ParentNodeID      *string                     `json:"parent_node_id"`
	Mode              string                      `json:"mode" binding:"required"`
	Prompt            string                      `json:"prompt" binding:"required"`
	Fragments         model.PromptCollectionState `json:"fragments"`
	NegativePrompt    *string                     `json:"negative_prompt"`
	CfgScale          *float64                    `json:"cfg_scale"`
	AspectRatio       *string                     `json:"aspect_ratio"`
	ImageSize         *string                     `json:"image_size"`
	Seed              *int64                      `json:"seed"`
	Steps             *int                        `json:"steps"`
	ReferenceImageIDs []string                    `json:"reference_image_ids"`
	Width             int                         `json:"width" binding:"required"`
	Height            int                         `json:"height" binding:"required"`
}

func isValidImageSize(size string) bool {
	switch size {
	case "1K", "2K", "4K":
		return true
	default:
		return false
	}
}

func isValidPromptCollectionState(state model.PromptCollectionState) bool {
	if state == nil {
		return false
	}

	rawGroupOrder, ok := state["groupOrder"]
	if !ok {
		return false
	}
	groupOrder, ok := rawGroupOrder.([]any)
	if !ok {
		return false
	}

	rawGroups, ok := state["groups"]
	if !ok {
		return false
	}
	groups, ok := rawGroups.(map[string]any)
	if !ok {
		return false
	}

	rawPrompts, ok := state["prompts"]
	if !ok {
		return false
	}
	prompts, ok := rawPrompts.(map[string]any)
	if !ok {
		return false
	}

	for _, rawGroupID := range groupOrder {
		groupID, ok := rawGroupID.(string)
		if !ok || strings.TrimSpace(groupID) == "" {
			return false
		}
		if _, ok := groups[groupID]; !ok {
			return false
		}
	}

	for groupID, rawGroup := range groups {
		group, ok := rawGroup.(map[string]any)
		if !ok {
			return false
		}

		groupIDInBody, ok := group["id"].(string)
		if !ok || strings.TrimSpace(groupIDInBody) == "" || groupIDInBody != groupID {
			return false
		}
		if _, ok := group["name"].(string); !ok {
			return false
		}
		if _, ok := group["collapsed"].(bool); !ok {
			return false
		}

		rawPromptIDs, ok := group["promptIds"]
		if !ok {
			return false
		}
		promptIDs, ok := rawPromptIDs.([]any)
		if !ok {
			return false
		}
		for _, rawPromptID := range promptIDs {
			promptID, ok := rawPromptID.(string)
			if !ok || strings.TrimSpace(promptID) == "" {
				return false
			}
			if _, ok := prompts[promptID]; !ok {
				return false
			}
		}
	}

	for promptID, rawPrompt := range prompts {
		prompt, ok := rawPrompt.(map[string]any)
		if !ok {
			return false
		}

		id, ok := prompt["id"].(string)
		if !ok || strings.TrimSpace(id) == "" || id != promptID {
			return false
		}
		if _, ok := prompt["content"].(string); !ok {
			return false
		}
		if !isValidTimestampLike(prompt["createdAt"]) {
			return false
		}
		if !isValidTimestampLike(prompt["updatedAt"]) {
			return false
		}
	}

	return true
}

func isNumberLike(value any) bool {
	switch value.(type) {
	case float64, float32, int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64, json.Number:
		return true
	default:
		return false
	}
}

func isValidTimestampLike(value any) bool {
	if isNumberLike(value) {
		return true
	}
	text, ok := value.(string)
	if !ok {
		return false
	}
	return strings.TrimSpace(text) != ""
}

type rerollRequest struct {
	RequestID   *string `json:"request_id"`
	AspectRatio *string `json:"aspect_ratio"`
	ImageSize   *string `json:"image_size"`
}

func nodeNotFoundResponse(c *gin.Context) {
	c.JSON(404, gin.H{"error": "not_found", "message": "node not found"})
}

func (h *GenerateHandler) Handle(c *gin.Context) {
	var raw map[string]json.RawMessage
	if err := c.ShouldBindBodyWith(&raw, binding.JSON); err != nil {
		c.JSON(400, gin.H{"error": "invalid"})
		return
	}
	if _, exists := raw["source_image_id"]; exists {
		c.JSON(400, gin.H{"error": "invalid"})
		return
	}

	var req generateRequest
	if err := c.ShouldBindBodyWith(&req, binding.JSON); err != nil {
		c.JSON(400, gin.H{"error": "invalid"})
		return
	}
	req.Mode = strings.TrimSpace(req.Mode)
	req.Prompt = strings.TrimSpace(req.Prompt)
	if req.Mode != model.PromptModeTxt2Img && req.Mode != model.PromptModeImg2Img {
		c.JSON(400, gin.H{"error": "invalid"})
		return
	}
	if req.Prompt == "" {
		c.JSON(400, gin.H{"error": "invalid"})
		return
	}
	if rawFragments, exists := raw["fragments"]; exists && string(rawFragments) != "null" {
		if !isValidPromptCollectionState(req.Fragments) {
			c.JSON(400, gin.H{"error": "invalid"})
			return
		}
	}
	if req.Width <= 0 || req.Height <= 0 {
		c.JSON(400, gin.H{"error": "invalid"})
		return
	}
	if req.ImageSize != nil {
		trimmedImageSize := strings.TrimSpace(*req.ImageSize)
		if trimmedImageSize == "" || !isValidImageSize(trimmedImageSize) {
			c.JSON(400, gin.H{"error": "invalid"})
			return
		}
		req.ImageSize = &trimmedImageSize
	}
	if len(req.ReferenceImageIDs) > 5 {
		c.JSON(400, gin.H{"error": "invalid"})
		return
	}
	if h == nil || h.Service == nil {
		c.JSON(500, gin.H{"error": "internal"})
		return
	}

	resp, err := h.Service.HandleGenerate(c.Request.Context(), service.GenerateRequest{
		Mode:              req.Mode,
		ParentNodeID:      req.ParentNodeID,
		Prompt:            req.Prompt,
		Fragments:         req.Fragments,
		NegativePrompt:    req.NegativePrompt,
		CfgScale:          req.CfgScale,
		AspectRatio:       req.AspectRatio,
		ImageSize:         req.ImageSize,
		Seed:              req.Seed,
		Steps:             req.Steps,
		ReferenceImageIDs: req.ReferenceImageIDs,
		Width:             req.Width,
		Height:            req.Height,
	})
	if err != nil {
		if errors.Is(err, service.ErrBusy) {
			c.JSON(429, gin.H{"error": "busy"})
			return
		}
		if errors.Is(err, service.ErrNodeNotFound) {
			nodeNotFoundResponse(c)
			return
		}
		if errors.Is(err, service.ErrParentModeMismatch) || errors.Is(err, service.ErrInvalidGenerateMode) {
			c.JSON(400, gin.H{"error": "invalid"})
			return
		}
		c.Error(err)
		c.JSON(500, gin.H{"error": "internal"})
		return
	}

	c.JSON(200, gin.H{"node": resp.Node, "images": resp.Images, "results": resp.Results})
}

func (h *GenerateHandler) Reroll(c *gin.Context) {
	var raw map[string]json.RawMessage
	if err := c.ShouldBindBodyWith(&raw, binding.JSON); err != nil {
		c.JSON(400, gin.H{"error": "invalid"})
		return
	}
	for key := range raw {
		if key != "request_id" && key != "aspect_ratio" && key != "image_size" {
			c.JSON(400, gin.H{"error": "invalid"})
			return
		}
	}

	var req rerollRequest
	if err := c.ShouldBindBodyWith(&req, binding.JSON); err != nil {
		c.JSON(400, gin.H{"error": "invalid"})
		return
	}
	if req.AspectRatio != nil {
		trimmedAspectRatio := strings.TrimSpace(*req.AspectRatio)
		if trimmedAspectRatio == "" {
			req.AspectRatio = nil
		} else {
			req.AspectRatio = &trimmedAspectRatio
		}
	}
	if req.ImageSize != nil {
		trimmedImageSize := strings.TrimSpace(*req.ImageSize)
		if trimmedImageSize == "" || !isValidImageSize(trimmedImageSize) {
			c.JSON(400, gin.H{"error": "invalid"})
			return
		}
		req.ImageSize = &trimmedImageSize
	}

	nodeID := strings.TrimSpace(c.Param("id"))
	if nodeID == "" {
		c.JSON(400, gin.H{"error": "invalid"})
		return
	}
	if h == nil || h.Service == nil {
		c.JSON(500, gin.H{"error": "internal"})
		return
	}

	resp, err := h.Service.HandleReroll(c.Request.Context(), nodeID, req.AspectRatio, req.ImageSize)
	if err != nil {
		if errors.Is(err, service.ErrBusy) {
			c.JSON(429, gin.H{"error": "busy"})
			return
		}
		if errors.Is(err, service.ErrNodeNotFound) {
			nodeNotFoundResponse(c)
			return
		}
		c.Error(err)
		c.JSON(500, gin.H{"error": "internal"})
		return
	}

	c.JSON(200, gin.H{"node": resp.Node, "images": resp.Images, "results": resp.Results})
}
