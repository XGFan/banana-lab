package api

import (
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"banana-lab/internal/model"
)

type LibraryHandler struct {
	DB  *gorm.DB
	Now func() time.Time
}

func (h *LibraryHandler) List(c *gin.Context) {
	if h == nil || h.DB == nil {
		c.JSON(500, gin.H{"error": "internal"})
		return
	}

	page, limit, ok := parsePageLimit(c.Query("page"), c.Query("limit"))
	if !ok {
		c.JSON(400, gin.H{"error": "invalid"})
		return
	}

	mode := strings.TrimSpace(c.Query("type"))
	if mode != "" && mode != model.PromptModeTxt2Img && mode != model.PromptModeImg2Img {
		c.JSON(400, gin.H{"error": "invalid"})
		return
	}

	now := time.Now
	if h.Now != nil {
		now = h.Now
	}

	dateFilter := strings.TrimSpace(c.Query("date"))
	start, end, older, ok := dateFilterRange(dateFilter, now())
	if !ok {
		c.JSON(400, gin.H{"error": "invalid"})
		return
	}

	query := h.DB.Model(&model.Image{})
	if mode != "" {
		query = query.
			Where("images.node_id IS NOT NULL").
			Joins("JOIN prompt_nodes ON prompt_nodes.id = images.node_id").
			Where("prompt_nodes.deleted_at IS NULL").
			Where("prompt_nodes.mode = ?", mode)
	}

	if dateFilter != "" {
		if older {
			query = query.Where("images.created_at < ?", start)
		} else {
			query = query.Where("images.created_at >= ? AND images.created_at < ?", start, end)
		}
	}

	var images []model.Image
	if err := query.Order("images.created_at desc").
		Limit(limit).
		Offset((page - 1) * limit).
		Find(&images).Error; err != nil {
		c.JSON(500, gin.H{"error": "internal"})
		return
	}

	c.JSON(200, images)
}

func parsePageLimit(pageValue string, limitValue string) (int, int, bool) {
	page := 1
	if strings.TrimSpace(pageValue) != "" {
		parsed, err := strconv.Atoi(pageValue)
		if err != nil || parsed < 1 {
			return 0, 0, false
		}
		page = parsed
	}

	limit := 20
	if strings.TrimSpace(limitValue) != "" {
		parsed, err := strconv.Atoi(limitValue)
		if err != nil {
			return 0, 0, false
		}
		if parsed < 1 {
			parsed = 1
		}
		if parsed > 100 {
			parsed = 100
		}
		limit = parsed
	}

	return page, limit, true
}

func startOfDay(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location())
}

func dateFilterRange(filter string, now time.Time) (time.Time, time.Time, bool, bool) {
	switch filter {
	case "":
		return time.Time{}, time.Time{}, false, true
	case "today":
		start := startOfDay(now)
		end := start.AddDate(0, 0, 1)
		return start, end, false, true
	case "yesterday":
		end := startOfDay(now)
		start := end.AddDate(0, 0, -1)
		return start, end, false, true
	case "older":
		start := startOfDay(now).AddDate(0, 0, -1)
		return start, time.Time{}, true, true
	default:
		return time.Time{}, time.Time{}, false, false
	}
}
