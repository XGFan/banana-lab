package api

import (
	"crypto/rand"
	"encoding/hex"
	"net/http"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"banana-lab/internal/model"
	"banana-lab/internal/service"
	"banana-lab/internal/storage"
)

type UploadHandler struct {
	DB      *gorm.DB
	Storage storage.Storage
}

func (h *UploadHandler) Handle(c *gin.Context) {
	if h == nil || h.DB == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal"})
		return
	}

	fileHeader, err := c.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid"})
		return
	}

	fileReader, err := fileHeader.Open()
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid"})
		return
	}
	defer fileReader.Close()

	jpegBytes, width, height, err := service.CompressToJPEGUnderSize(fileReader, 2*1024*1024)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid"})
		return
	}

	id, err := newImageID()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal"})
		return
	}

	relPath, size, err := h.Storage.SaveBytes("uploads", id+".jpg", jpegBytes)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal"})
		return
	}

	widthVal := width
	heightVal := height
	sizeVal := size
	image := model.Image{
		ID:       id,
		NodeID:   nil,
		Origin:   "uploaded",
		FilePath: relPath,
		FileSize: &sizeVal,
		Width:    &widthVal,
		Height:   &heightVal,
	}

	if err := h.DB.Create(&image).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"image": image})
}

func newImageID() (string, error) {
	bytes := make([]byte, 16)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}
