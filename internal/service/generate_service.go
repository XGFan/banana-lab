package service

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gorm.io/gorm"

	"banana-lab/internal/model"
	"banana-lab/internal/storage"
)

var ErrBusy = errors.New("busy")

var (
	ErrNodeNotFound        = errors.New("node not found")
	ErrInvalidGenerateMode = errors.New("invalid generate mode")
	ErrParentModeMismatch  = errors.New("parent mode mismatch")
)

type GenerateService struct {
	DB      *gorm.DB
	Storage storage.Storage
	Proxy   *ProxyClient
	Limiter *ConcurrencyLimiter
}

type GenerateRequest struct {
	Mode              string
	ParentNodeID      *string
	Prompt            string
	Model             string
	Fragments         model.PromptCollectionState
	NegativePrompt    *string
	CfgScale          *float64
	AspectRatio       *string
	ImageSize         *string
	Seed              *int64
	Steps             *int
	ReferenceImageIDs []string
	Width             int
	Height            int
}

type GenerateResponse struct {
	Node    model.Node
	Images  []model.Image
	Results []model.Result
}

type referenceSource struct {
	data          []byte
	mime          string
	path          string
	origin        string
	sourceAssetID *string
	sourceDomain  *string
}

func (s *GenerateService) HandleGenerate(ctx context.Context, req GenerateRequest) (GenerateResponse, error) {
	if s == nil {
		return GenerateResponse{}, errors.New("generate service is nil")
	}
	if s.DB == nil {
		return GenerateResponse{}, errors.New("db is nil")
	}
	if s.Proxy == nil {
		return GenerateResponse{}, errors.New("proxy is nil")
	}
	if s.Limiter == nil {
		return GenerateResponse{}, errors.New("limiter is nil")
	}

	mode := strings.TrimSpace(req.Mode)
	if mode != model.PromptModeTxt2Img && mode != model.PromptModeImg2Img {
		return GenerateResponse{}, ErrInvalidGenerateMode
	}
	prompt := strings.TrimSpace(req.Prompt)
	if prompt == "" {
		return GenerateResponse{}, errors.New("prompt is empty")
	}
	reqModel := strings.TrimSpace(req.Model)
	if reqModel == "" {
		return GenerateResponse{}, errors.New("model is empty")
	}

	release, ok := s.Limiter.TryAcquire()
	if !ok {
		return GenerateResponse{}, ErrBusy
	}
	defer release()

	var parentID *string
	if req.ParentNodeID != nil {
		trimmedParentID := strings.TrimSpace(*req.ParentNodeID)
		if trimmedParentID == "" {
			return GenerateResponse{}, ErrNodeNotFound
		}

		var parent model.Node
		if err := s.DB.Where("id = ? AND deleted_at IS NULL", trimmedParentID).First(&parent).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return GenerateResponse{}, ErrNodeNotFound
			}
			return GenerateResponse{}, err
		}
		if parent.Mode != mode {
			return GenerateResponse{}, ErrParentModeMismatch
		}
		parentID = &parent.ID
	}

	now := time.Now().UTC()
	newIDValue, err := newID()
	if err != nil {
		return GenerateResponse{}, err
	}
	targetNode := model.Node{
		ID:                newIDValue,
		ParentID:          parentID,
		Mode:              mode,
		Prompt:            prompt,
		ModelID:           reqModel,
		Fragments:         req.Fragments.Clone(),
		Title:             prompt,
		LastUsedAt:        now,
		NegativePrompt:    req.NegativePrompt,
		CfgScale:          req.CfgScale,
		AspectRatio:       req.AspectRatio,
		ImageSize:         req.ImageSize,
		Seed:              req.Seed,
		Steps:             req.Steps,
		ReferenceImageIDs: model.StringSlice(req.ReferenceImageIDs),
	}

	referenceImageIDs := append([]string(nil), req.ReferenceImageIDs...)
	var copiedReferenceAssets []model.Asset
	var sourceImageDataURL *string
	var sourceAssetID *string
	var sourceDomain *string
	if mode == model.PromptModeImg2Img && len(referenceImageIDs) > 0 {
		copiedIDs, copiedAssets, sourceDataURL, srcAssetID, srcDomain, err := copyReferenceAssetsForImg2Img(s.DB, s.Storage, mode, targetNode.ID, referenceImageIDs)
		if err != nil {
			return GenerateResponse{}, err
		}
		referenceImageIDs = copiedIDs
		copiedReferenceAssets = copiedAssets
		targetNode.ReferenceImageIDs = model.StringSlice(copiedIDs)
		if sourceDataURL != "" {
			sourceImageDataURL = &sourceDataURL
		}
		sourceAssetID = srcAssetID
		sourceDomain = srcDomain
	} else if referenceID := firstNonEmpty(referenceImageIDs); referenceID != "" {
		dataURL, srcAssetID, srcDomain, err := buildSourceAssetDataURL(s.DB, s.Storage, referenceID)
		if err != nil {
			return GenerateResponse{}, err
		}
		sourceImageDataURL = &dataURL
		sourceAssetID = srcAssetID
		sourceDomain = srcDomain
	}

	if strings.TrimSpace(targetNode.ModelID) == "" {
		return GenerateResponse{}, errors.New("model id is empty")
	}

	proxyImages, err := s.Proxy.Generate(ctx, targetNode.ModelID, targetNode.Prompt, targetNode.AspectRatio, targetNode.ImageSize, sourceImageDataURL)
	if err != nil {
		return GenerateResponse{}, err
	}

	relDir := filepath.Join("images", mode, targetNode.ID)

	if err := s.DB.Transaction(func(tx *gorm.DB) error {
		targetNode.ReferenceImageIDs = model.StringSlice(referenceImageIDs)
		if err := tx.Create(&targetNode).Error; err != nil {
			return err
		}

		for _, copiedAsset := range copiedReferenceAssets {
			if err := tx.Create(&copiedAsset).Error; err != nil {
				return err
			}
		}

		for _, proxyImage := range proxyImages {
			mime, data, err := fetchProxyImage(ctx, proxyImage.URL, s.Proxy.HTTP)
			if err != nil {
				return err
			}

			filenameID, err := newID()
			if err != nil {
				return err
			}
			ext := extFromMime(mime)
			if ext == "" {
				ext = extFromPath(proxyImage.URL)
			}
			if ext == "" {
				ext = ".bin"
			}
			filename := filenameID + ext
			relPath, size, err := s.Storage.SaveBytes(relDir, filename, data)
			if err != nil {
				return err
			}
			fileSize := size
			nodeID := targetNode.ID
			asset := model.Asset{
				ID:       filenameID,
				Domain:   assetDomainByPromptMode(mode),
				Origin:   model.AssetOriginGenerated,
				FilePath: relPath,
				FileSize: &fileSize,
			}
			if err := tx.Create(&asset).Error; err != nil {
				return err
			}

			result := model.Result{
				ID:            filenameID,
				NodeID:        nodeID,
				AssetID:       asset.ID,
				Status:        model.ResultStatusActive,
				SourceAssetID: sourceAssetID,
				SourceDomain:  sourceDomain,
			}
			if err := tx.Create(&result).Error; err != nil {
				return err
			}

			image := model.Image{
				ID:       filenameID,
				NodeID:   &nodeID,
				Origin:   "generated",
				FilePath: relPath,
				FileSize: &fileSize,
			}
			if err := tx.Create(&image).Error; err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		return GenerateResponse{}, err
	}

	var allResults []model.Result
	if err := s.DB.Where("node_id = ?", targetNode.ID).Order("created_at desc").Find(&allResults).Error; err != nil {
		return GenerateResponse{}, err
	}

	var allImages []model.Image
	if err := s.DB.Where("node_id = ?", targetNode.ID).Order("created_at desc").Find(&allImages).Error; err != nil {
		return GenerateResponse{}, err
	}

	return GenerateResponse{
		Node:    targetNode,
		Images:  allImages,
		Results: allResults,
	}, nil
}

func (s *GenerateService) HandleReroll(ctx context.Context, nodeID string, aspectRatio *string, imageSize *string, modelOverride *string) (GenerateResponse, error) {
	if s == nil {
		return GenerateResponse{}, errors.New("generate service is nil")
	}
	if s.DB == nil {
		return GenerateResponse{}, errors.New("db is nil")
	}
	if s.Proxy == nil {
		return GenerateResponse{}, errors.New("proxy is nil")
	}
	if s.Limiter == nil {
		return GenerateResponse{}, errors.New("limiter is nil")
	}

	trimmedNodeID := strings.TrimSpace(nodeID)
	if trimmedNodeID == "" {
		return GenerateResponse{}, ErrNodeNotFound
	}

	release, ok := s.Limiter.TryAcquire()
	if !ok {
		return GenerateResponse{}, ErrBusy
	}
	defer release()

	var targetNode model.Node
	if err := s.DB.Where("id = ? AND deleted_at IS NULL", trimmedNodeID).First(&targetNode).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return GenerateResponse{}, ErrNodeNotFound
		}
		return GenerateResponse{}, err
	}

	mode := strings.TrimSpace(targetNode.Mode)
	if mode != model.PromptModeTxt2Img && mode != model.PromptModeImg2Img {
		return GenerateResponse{}, ErrInvalidGenerateMode
	}

	if aspectRatio != nil {
		trimmedAspectRatio := strings.TrimSpace(*aspectRatio)
		if trimmedAspectRatio == "" {
			targetNode.AspectRatio = nil
		} else {
			targetNode.AspectRatio = &trimmedAspectRatio
		}
	}
	if imageSize != nil {
		trimmedImageSize := strings.TrimSpace(*imageSize)
		if trimmedImageSize == "" {
			targetNode.ImageSize = nil
		} else {
			targetNode.ImageSize = &trimmedImageSize
		}
	}
	modelOverridden := false
	if modelOverride != nil {
		trimmedModel := strings.TrimSpace(*modelOverride)
		if trimmedModel != "" {
			targetNode.ModelID = trimmedModel
			modelOverridden = true
		}
	}

	var sourceImageDataURL *string
	var sourceAssetID *string
	var sourceDomain *string
	if referenceID := firstNonEmpty([]string(targetNode.ReferenceImageIDs)); referenceID != "" {
		dataURL, srcAssetID, srcDomain, err := buildSourceAssetDataURL(s.DB, s.Storage, referenceID)
		if err != nil {
			return GenerateResponse{}, err
		}
		sourceImageDataURL = &dataURL
		sourceAssetID = srcAssetID
		sourceDomain = srcDomain
	}

	if strings.TrimSpace(targetNode.ModelID) == "" {
		return GenerateResponse{}, errors.New("model id is empty")
	}

	proxyImages, err := s.Proxy.Generate(ctx, targetNode.ModelID, targetNode.Prompt, targetNode.AspectRatio, targetNode.ImageSize, sourceImageDataURL)
	if err != nil {
		return GenerateResponse{}, err
	}

	relDir := filepath.Join("images", mode, targetNode.ID)
	now := time.Now().UTC()

	if err := s.DB.Transaction(func(tx *gorm.DB) error {
		nodeUpdates := map[string]any{"last_used_at": now}
		if aspectRatio != nil {
			nodeUpdates["aspect_ratio"] = targetNode.AspectRatio
		}
		if imageSize != nil {
			nodeUpdates["image_size"] = targetNode.ImageSize
		}
		if modelOverridden {
			nodeUpdates["model_id"] = targetNode.ModelID
		}
		nodeUpdate := tx.Model(&model.Node{}).Where("id = ? AND deleted_at IS NULL", targetNode.ID).Updates(nodeUpdates)
		if nodeUpdate.Error != nil {
			return nodeUpdate.Error
		}
		if nodeUpdate.RowsAffected == 0 {
			return ErrNodeNotFound
		}

		for _, proxyImage := range proxyImages {
			mime, data, err := fetchProxyImage(ctx, proxyImage.URL, s.Proxy.HTTP)
			if err != nil {
				return err
			}

			filenameID, err := newID()
			if err != nil {
				return err
			}
			ext := extFromMime(mime)
			if ext == "" {
				ext = extFromPath(proxyImage.URL)
			}
			if ext == "" {
				ext = ".bin"
			}
			filename := filenameID + ext
			relPath, size, err := s.Storage.SaveBytes(relDir, filename, data)
			if err != nil {
				return err
			}
			fileSize := size
			nodeIDForResult := targetNode.ID
			asset := model.Asset{
				ID:       filenameID,
				Domain:   assetDomainByPromptMode(mode),
				Origin:   model.AssetOriginGenerated,
				FilePath: relPath,
				FileSize: &fileSize,
			}
			if err := tx.Create(&asset).Error; err != nil {
				return err
			}

			result := model.Result{
				ID:            filenameID,
				NodeID:        nodeIDForResult,
				AssetID:       asset.ID,
				Status:        model.ResultStatusActive,
				SourceAssetID: sourceAssetID,
				SourceDomain:  sourceDomain,
			}
			if err := tx.Create(&result).Error; err != nil {
				return err
			}

			image := model.Image{
				ID:       filenameID,
				NodeID:   &nodeIDForResult,
				Origin:   "generated",
				FilePath: relPath,
				FileSize: &fileSize,
			}
			if err := tx.Create(&image).Error; err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		return GenerateResponse{}, err
	}
	targetNode.LastUsedAt = now

	var allResults []model.Result
	if err := s.DB.Where("node_id = ?", targetNode.ID).Order("created_at desc").Find(&allResults).Error; err != nil {
		return GenerateResponse{}, err
	}

	var allImages []model.Image
	if err := s.DB.Where("node_id = ?", targetNode.ID).Order("created_at desc").Find(&allImages).Error; err != nil {
		return GenerateResponse{}, err
	}

	return GenerateResponse{Node: targetNode, Images: allImages, Results: allResults}, nil
}

func buildSourceAssetDataURL(db *gorm.DB, store storage.Storage, sourceID string) (string, *string, *string, error) {
	var asset model.Asset
	if err := db.First(&asset, "id = ?", sourceID).Error; err == nil {
		dataURL, dataErr := buildSourceAssetDataURLFromAsset(store, asset)
		if dataErr != nil {
			return "", nil, nil, dataErr
		}
		return dataURL, &asset.ID, &asset.Domain, nil
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return "", nil, nil, err
	}

	var image model.Image
	if err := db.First(&image, "id = ?", sourceID).Error; err != nil {
		return "", nil, nil, err
	}

	dataURL, err := buildSourceImageDataURL(store, image)
	if err != nil {
		return "", nil, nil, err
	}

	var domain string
	if image.Origin == model.AssetOriginUploaded {
		domain = model.AssetDomainImg2Img
	} else {
		domain = model.AssetDomainTxt2Img
	}
	return dataURL, nil, &domain, nil
}

func copyReferenceAssetsForImg2Img(db *gorm.DB, store storage.Storage, mode, nodeID string, referenceIDs []string) ([]string, []model.Asset, string, *string, *string, error) {
	trimmedReferenceIDs := make([]string, 0, len(referenceIDs))
	for _, referenceID := range referenceIDs {
		trimmed := strings.TrimSpace(referenceID)
		if trimmed != "" {
			trimmedReferenceIDs = append(trimmedReferenceIDs, trimmed)
		}
	}
	if len(trimmedReferenceIDs) == 0 {
		return nil, nil, "", nil, nil, nil
	}

	relDir := filepath.Join("images", mode, "inputs", nodeID)
	copiedAssetIDs := make([]string, 0, len(trimmedReferenceIDs))
	copiedAssets := make([]model.Asset, 0, len(trimmedReferenceIDs))
	var sourceImageDataURL string
	var sourceAssetID *string
	var sourceDomain *string

	for i, referenceID := range trimmedReferenceIDs {
		source, err := loadReferenceSource(db, store, referenceID)
		if err != nil {
			return nil, nil, "", nil, nil, err
		}

		copiedAssetID, err := newID()
		if err != nil {
			return nil, nil, "", nil, nil, err
		}
		ext := extFromMime(source.mime)
		if ext == "" {
			ext = extFromPath(source.path)
		}
		if ext == "" {
			ext = ".bin"
		}

		relPath, size, err := store.SaveBytes(relDir, copiedAssetID+ext, source.data)
		if err != nil {
			return nil, nil, "", nil, nil, err
		}

		fileSize := size
		copiedAsset := model.Asset{
			ID:       copiedAssetID,
			Domain:   model.AssetDomainImg2Img,
			Origin:   source.origin,
			FilePath: relPath,
			FileSize: &fileSize,
		}
		copiedAssetIDs = append(copiedAssetIDs, copiedAssetID)
		copiedAssets = append(copiedAssets, copiedAsset)

		if i == 0 {
			mime := source.mime
			if mime == "" {
				mime = "application/octet-stream"
			}
			sourceImageDataURL = fmt.Sprintf("data:%s;base64,%s", mime, base64.StdEncoding.EncodeToString(source.data))
			sourceAssetID = source.sourceAssetID
			sourceDomain = source.sourceDomain
		}
	}

	return copiedAssetIDs, copiedAssets, sourceImageDataURL, sourceAssetID, sourceDomain, nil
}

func loadReferenceSource(db *gorm.DB, store storage.Storage, sourceID string) (referenceSource, error) {
	var asset model.Asset
	if err := db.First(&asset, "id = ?", sourceID).Error; err == nil {
		absPath, joinErr := safeJoin(store.Root, asset.FilePath)
		if joinErr != nil {
			return referenceSource{}, joinErr
		}
		data, readErr := os.ReadFile(absPath)
		if readErr != nil {
			return referenceSource{}, readErr
		}
		mime := mimeFromExt(asset.FilePath)
		if mime == "" {
			mime = "application/octet-stream"
		}
		return referenceSource{
			data:          data,
			mime:          mime,
			path:          asset.FilePath,
			origin:        asset.Origin,
			sourceAssetID: &asset.ID,
			sourceDomain:  &asset.Domain,
		}, nil
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return referenceSource{}, err
	}

	var image model.Image
	if err := db.First(&image, "id = ?", sourceID).Error; err != nil {
		return referenceSource{}, err
	}
	absPath, err := safeJoin(store.Root, image.FilePath)
	if err != nil {
		return referenceSource{}, err
	}
	data, err := os.ReadFile(absPath)
	if err != nil {
		return referenceSource{}, err
	}
	mime := mimeFromExt(image.FilePath)
	if mime == "" {
		mime = "application/octet-stream"
	}

	sourceDomainValue := model.AssetDomainTxt2Img
	if image.Origin == model.AssetOriginUploaded {
		sourceDomainValue = model.AssetDomainImg2Img
	}

	return referenceSource{
		data:         data,
		mime:         mime,
		path:         image.FilePath,
		origin:       image.Origin,
		sourceDomain: &sourceDomainValue,
	}, nil
}

func assetDomainByPromptMode(mode string) string {
	if mode == model.PromptModeImg2Img {
		return model.AssetDomainImg2Img
	}
	return model.AssetDomainTxt2Img
}

func buildSourceAssetDataURLFromAsset(store storage.Storage, asset model.Asset) (string, error) {
	absPath, err := safeJoin(store.Root, asset.FilePath)
	if err != nil {
		return "", err
	}
	data, err := os.ReadFile(absPath)
	if err != nil {
		return "", err
	}
	mime := mimeFromExt(asset.FilePath)
	if mime == "" {
		mime = "application/octet-stream"
	}
	encoded := base64.StdEncoding.EncodeToString(data)
	return fmt.Sprintf("data:%s;base64,%s", mime, encoded), nil
}

func buildSourceImageDataURL(store storage.Storage, image model.Image) (string, error) {
	absPath, err := safeJoin(store.Root, image.FilePath)
	if err != nil {
		return "", err
	}
	data, err := os.ReadFile(absPath)
	if err != nil {
		return "", err
	}
	mime := mimeFromExt(image.FilePath)
	if mime == "" {
		mime = "application/octet-stream"
	}
	encoded := base64.StdEncoding.EncodeToString(data)
	return fmt.Sprintf("data:%s;base64,%s", mime, encoded), nil
}

func firstNonEmpty(values []string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func fetchProxyImage(ctx context.Context, url string, client *http.Client) (string, []byte, error) {
	if isDataURL(url) {
		mime, data, err := DecodeDataURL(url)
		if err != nil {
			return "", nil, err
		}
		return mime, data, nil
	}
	if !isHTTPURL(url) {
		return "", nil, fmt.Errorf("unsupported image url: %s", url)
	}

	httpClient := client
	if httpClient == nil {
		httpClient = http.DefaultClient
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", nil, err
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		return "", nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return "", nil, fmt.Errorf("image download status %d", resp.StatusCode)
	}

	data, err := readAllWithLimit(resp.Body, 20*1024*1024)
	if err != nil {
		return "", nil, err
	}

	contentType := strings.TrimSpace(resp.Header.Get("Content-Type"))
	if contentType != "" {
		if parts := strings.SplitN(contentType, ";", 2); len(parts) > 0 {
			contentType = strings.TrimSpace(parts[0])
		}
	}
	if contentType == "" {
		contentType = mimeFromExt(url)
	}

	return contentType, data, nil
}

func readAllWithLimit(reader io.Reader, limit int64) ([]byte, error) {
	if limit <= 0 {
		return nil, errors.New("limit must be positive")
	}
	limited := &io.LimitedReader{R: reader, N: limit + 1}
	data, err := io.ReadAll(limited)
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > limit {
		return nil, errors.New("image exceeds size limit")
	}
	return data, nil
}

func newID() (string, error) {
	bytes := make([]byte, 16)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}

func safeJoin(root, relPath string) (string, error) {
	if strings.TrimSpace(root) == "" {
		return "", errors.New("storage root is empty")
	}
	if relPath == "" {
		return "", errors.New("relative path is empty")
	}
	if filepath.IsAbs(relPath) {
		return "", errors.New("absolute paths are not allowed")
	}

	cleanRel := filepath.Clean(relPath)
	if cleanRel == "." || cleanRel == ".." || strings.HasPrefix(cleanRel, ".."+string(os.PathSeparator)) {
		return "", errors.New("path traversal detected")
	}

	absPath := filepath.Join(root, cleanRel)
	rel, err := filepath.Rel(root, absPath)
	if err != nil {
		return "", err
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
		return "", errors.New("path escapes root")
	}

	return absPath, nil
}

func isDataURL(url string) bool {
	return strings.HasPrefix(strings.ToLower(strings.TrimSpace(url)), "data:")
}

func isHTTPURL(url string) bool {
	trimmed := strings.ToLower(strings.TrimSpace(url))
	return strings.HasPrefix(trimmed, "http://") || strings.HasPrefix(trimmed, "https://")
}

func extFromPath(path string) string {
	trimmed := path
	if idx := strings.IndexAny(trimmed, "?#"); idx >= 0 {
		trimmed = trimmed[:idx]
	}
	ext := strings.ToLower(filepath.Ext(trimmed))
	return ext
}

func mimeFromExt(path string) string {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".png":
		return "image/png"
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".webp":
		return "image/webp"
	case ".gif":
		return "image/gif"
	case ".bmp":
		return "image/bmp"
	default:
		return ""
	}
}

func extFromMime(mime string) string {
	trimmed := strings.ToLower(strings.TrimSpace(mime))
	switch trimmed {
	case "image/png":
		return ".png"
	case "image/jpeg":
		return ".jpg"
	case "image/webp":
		return ".webp"
	case "image/gif":
		return ".gif"
	case "image/bmp":
		return ".bmp"
	default:
		return ""
	}
}
