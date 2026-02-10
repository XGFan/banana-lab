package api

import (
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"banana-lab/internal/model"
)

const archiveRangeInvalidCode = "ARCHIVE_RANGE_INVALID"

type ArchiveHandler struct {
	DB *gorm.DB
}

type archiveAssetResponse struct {
	ID       string `json:"id"`
	FilePath string `json:"file_path"`
}

type archiveResultResponse struct {
	ID         string               `json:"id"`
	Domain     string               `json:"domain"`
	ArchivedAt *time.Time           `json:"archived_at"`
	AssetID    string               `json:"asset_id"`
	Asset      archiveAssetResponse `json:"asset"`
}

type archiveRecordRow struct {
	ID            string     `gorm:"column:id"`
	AssetID       string     `gorm:"column:asset_id"`
	ArchivedAt    *time.Time `gorm:"column:archived_at"`
	AssetRecordID string     `gorm:"column:asset_record_id"`
	Domain        string     `gorm:"column:domain"`
	FilePath      string     `gorm:"column:file_path"`
}

func (h *ArchiveHandler) List(c *gin.Context) {
	if h == nil || h.DB == nil {
		c.JSON(500, gin.H{"error": "internal"})
		return
	}

	from, ok := parseArchiveTimeParam(c.Query("from"))
	if !ok {
		c.JSON(400, gin.H{"error": "invalid"})
		return
	}
	to, ok := parseArchiveTimeParam(c.Query("to"))
	if !ok {
		c.JSON(400, gin.H{"error": "invalid"})
		return
	}
	if from != nil && to != nil && from.After(*to) {
		c.JSON(400, gin.H{"error": "invalid", "code": archiveRangeInvalidCode})
		return
	}

	query := h.DB.Table("results").
		Select("results.id, results.asset_id, results.archived_at, assets.id AS asset_record_id, assets.domain, assets.file_path").
		Joins("JOIN assets ON assets.id = results.asset_id").
		Where("results.status = ?", model.ResultStatusArchived)
	if from != nil {
		query = query.Where("results.archived_at >= ?", *from)
	}
	if to != nil {
		query = query.Where("results.archived_at <= ?", *to)
	}

	var rows []archiveRecordRow
	if err := query.Order("results.archived_at DESC").Order("results.created_at DESC").Find(&rows).Error; err != nil {
		c.JSON(500, gin.H{"error": "internal"})
		return
	}

	resp := gin.H{
		model.AssetDomainTxt2Img: []archiveResultResponse{},
		model.AssetDomainImg2Img: []archiveResultResponse{},
	}
	for _, row := range rows {
		item := archiveResultResponse{
			ID:         row.ID,
			Domain:     row.Domain,
			ArchivedAt: row.ArchivedAt,
			AssetID:    row.AssetID,
			Asset: archiveAssetResponse{
				ID:       row.AssetRecordID,
				FilePath: row.FilePath,
			},
		}
		group, exists := resp[row.Domain]
		if !exists {
			continue
		}
		resp[row.Domain] = append(group.([]archiveResultResponse), item)
	}

	c.JSON(200, resp)
}

func parseArchiveTimeParam(raw string) (*time.Time, bool) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return nil, true
	}
	parsed, err := time.Parse(time.RFC3339Nano, trimmed)
	if err != nil {
		return nil, false
	}
	return &parsed, true
}
