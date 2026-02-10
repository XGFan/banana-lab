package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"
)

type ProxyClient struct {
	BaseURL         string
	AuthHeaderName  string
	AuthHeaderValue string
	HTTP            *http.Client
}

type ProxyGenerateParams struct {
	Prompt             string
	AspectRatio        *string
	ImageSize          *string
	SourceImageDataURL *string
}

type ProxyImage struct {
	URL string
}

type proxyRequest struct {
	Model       string         `json:"model"`
	Messages    []proxyMessage `json:"messages"`
	Modalities  []string       `json:"modalities"`
	ImageConfig *imageConfig   `json:"image_config,omitempty"`
}

type proxyMessage struct {
	Role    string             `json:"role"`
	Content []proxyContentPart `json:"content"`
}

type proxyContentPart struct {
	Type     string         `json:"type"`
	Text     string         `json:"text,omitempty"`
	ImageURL *proxyImageURL `json:"image_url,omitempty"`
}

type proxyImageURL struct {
	URL string `json:"url"`
}

type imageConfig struct {
	AspectRatio string `json:"aspect_ratio,omitempty"`
	ImageSize   string `json:"image_size,omitempty"`
}

type proxyResponse struct {
	Choices []proxyChoice `json:"choices"`
}

type proxyChoice struct {
	Message proxyMessageResponse `json:"message"`
}

type proxyMessageResponse struct {
	Images  []proxyImageObject `json:"images"`
	Content json.RawMessage    `json:"content"`
}

type proxyImageObject struct {
	ImageURL proxyImageURL `json:"image_url"`
}

func (c *ProxyClient) Generate(ctx context.Context, modelID string, prompt string, aspectRatio *string, imageSize *string, sourceImageDataURL *string) ([]ProxyImage, error) {
	if strings.TrimSpace(c.BaseURL) == "" {
		return nil, errors.New("proxy base URL is empty")
	}
	if strings.TrimSpace(modelID) == "" {
		return nil, errors.New("model ID is empty")
	}

	url := buildCompletionsURL(c.BaseURL)
	content := []proxyContentPart{{
		Type: "text",
		Text: prompt,
	}}
	if sourceImageDataURL != nil && strings.TrimSpace(*sourceImageDataURL) != "" {
		content = append(content, proxyContentPart{
			Type: "image_url",
			ImageURL: &proxyImageURL{
				URL: *sourceImageDataURL,
			},
		})
	}

	reqPayload := proxyRequest{
		Model:      modelID,
		Messages:   []proxyMessage{{Role: "user", Content: content}},
		Modalities: []string{"image", "text"},
	}
	if (aspectRatio != nil && strings.TrimSpace(*aspectRatio) != "") || (imageSize != nil && strings.TrimSpace(*imageSize) != "") {
		config := &imageConfig{}
		if aspectRatio != nil && strings.TrimSpace(*aspectRatio) != "" {
			config.AspectRatio = *aspectRatio
		}
		if imageSize != nil && strings.TrimSpace(*imageSize) != "" {
			config.ImageSize = *imageSize
		}
		reqPayload.ImageConfig = config
	}

	bodyBytes, err := json.Marshal(reqPayload)
	if err != nil {
		return nil, fmt.Errorf("marshal proxy request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, strings.NewReader(string(bodyBytes)))
	if err != nil {
		return nil, fmt.Errorf("create proxy request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if strings.TrimSpace(c.AuthHeaderName) != "" {
		req.Header.Set(c.AuthHeaderName, c.AuthHeaderValue)
	}

	client := c.HTTP
	if client == nil {
		client = &http.Client{Timeout: 60 * time.Second}
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("proxy request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, readErr := readBodyLimited(resp, 20*1024*1024)
	if readErr != nil {
		return nil, fmt.Errorf("read proxy response: %w", readErr)
	}

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return nil, fmt.Errorf("proxy response status %d (Content-Type: %s): %s",
			resp.StatusCode, resp.Header.Get("Content-Type"), truncateBody(string(respBody), 2048))
	}

	images, err := parseProxyResponse(respBody)
	if err != nil {
		return nil, fmt.Errorf("parse proxy response status %d (Content-Type: %s): %w, body: %s",
			resp.StatusCode, resp.Header.Get("Content-Type"), err, truncateBody(string(respBody), 2048))
	}
	return images, nil
}

func buildCompletionsURL(baseURL string) string {
	trimmed := strings.TrimSpace(baseURL)
	if strings.HasSuffix(trimmed, "/chat/completions") {
		return trimmed
	}
	return strings.TrimRight(trimmed, "/") + "/chat/completions"
}

func parseProxyResponse(data []byte) ([]ProxyImage, error) {
	var resp proxyResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("unmarshal response: %w", err)
	}
	if len(resp.Choices) == 0 {
		return nil, errors.New("proxy response has no choices")
	}

	message := resp.Choices[0].Message
	images := extractImagesFromMessage(message)
	if len(images) > 0 {
		return images, nil
	}

	dataURLs := extractDataURLsFromContent(message.Content)
	if len(dataURLs) == 0 {
		return nil, errors.New("proxy response contains no images")
	}

	result := make([]ProxyImage, 0, len(dataURLs))
	for _, url := range dataURLs {
		result = append(result, ProxyImage{URL: url})
	}
	return result, nil
}

func extractImagesFromMessage(message proxyMessageResponse) []ProxyImage {
	if len(message.Images) == 0 {
		return nil
	}
	result := make([]ProxyImage, 0, len(message.Images))
	for _, image := range message.Images {
		if strings.TrimSpace(image.ImageURL.URL) == "" {
			continue
		}
		result = append(result, ProxyImage{URL: image.ImageURL.URL})
	}
	return result
}

func extractDataURLsFromContent(content json.RawMessage) []string {
	if len(content) == 0 {
		return nil
	}

	if text, ok := parseContentAsString(content); ok {
		return findDataURLs(text)
	}

	var parts []proxyContentPart
	if err := json.Unmarshal(content, &parts); err == nil {
		var urls []string
		for _, part := range parts {
			if strings.TrimSpace(part.Text) == "" {
				continue
			}
			urls = append(urls, findDataURLs(part.Text)...)
		}
		return uniqueNonEmpty(urls)
	}

	return findDataURLs(string(content))
}

func parseContentAsString(content json.RawMessage) (string, bool) {
	var text string
	if err := json.Unmarshal(content, &text); err != nil {
		return "", false
	}
	return text, true
}

func findDataURLs(text string) []string {
	if strings.TrimSpace(text) == "" {
		return nil
	}
	re := regexp.MustCompile(`data:image/[a-zA-Z0-9.+-]+;base64,[a-zA-Z0-9+/=]+`)
	matches := re.FindAllString(text, -1)
	return uniqueNonEmpty(matches)
}

func uniqueNonEmpty(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		if _, exists := seen[trimmed]; exists {
			continue
		}
		seen[trimmed] = struct{}{}
		result = append(result, trimmed)
	}
	return result
}

func readBodyLimited(resp *http.Response, limit int) ([]byte, error) {
	if limit <= 0 {
		limit = 32 * 1024
	}
	lr := io.LimitReader(resp.Body, int64(limit)+1)
	data, err := io.ReadAll(lr)
	if err != nil {
		return nil, err
	}
	if len(data) > limit {
		return data[:limit], fmt.Errorf("proxy response exceeds limit of %d bytes", limit)
	}
	return data, nil
}

func truncateBody(body string, max int) string {
	if max <= 0 {
		return ""
	}
	if len(body) <= max {
		return body
	}
	return body[:max] + "..."
}
