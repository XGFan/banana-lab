package api

import (
	"github.com/gin-gonic/gin"

	"banana-lab/internal/config"
)

// ModelsHandler serves the GET /api/models endpoint, returning the list of
// currently-enabled models from the configured registry.
type ModelsHandler struct {
	Registry config.Registry
}

type modelDTO struct {
	ID                  string   `json:"id"`
	Label               string   `json:"label"`
	AllowedSizes        []string `json:"allowed_sizes"`
	AllowedAspectRatios []string `json:"allowed_aspect_ratios"`
	DefaultSize         string   `json:"default_size"`
}

// List returns {"models": [...]}, filtered to enabled==true entries.
func (h *ModelsHandler) List(c *gin.Context) {
	enabled := h.Registry.EnabledList()
	out := make([]modelDTO, 0, len(enabled))
	for _, m := range enabled {
		out = append(out, modelDTO{
			ID:                  m.ID,
			Label:               m.Label,
			AllowedSizes:        append([]string(nil), m.AllowedSizes...),
			AllowedAspectRatios: append([]string(nil), m.AllowedAspectRatios...),
			DefaultSize:         m.DefaultSize,
		})
	}
	c.JSON(200, gin.H{"models": out})
}
