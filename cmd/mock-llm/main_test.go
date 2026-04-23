package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestChatCompletions(t *testing.T) {
	mux := setupMux()

	t.Run("success_default", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/chat/completions", strings.NewReader(`{"model":"test-model","messages":[{"role":"user","content":[{"type":"text","text":"hello"}]}]}`))
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("expected status 200, got %d", w.Code)
		}

		var resp struct {
			Choices []struct {
				Message struct {
					Images []struct {
						ImageURL struct {
							URL string `json:"url"`
						} `json:"image_url"`
					} `json:"images"`
				} `json:"message"`
			} `json:"choices"`
		}

		if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
			t.Fatalf("failed to unmarshal response: %v", err)
		}

		if len(resp.Choices) == 0 || len(resp.Choices[0].Message.Images) == 0 {
			t.Fatal("response should contain at least one image")
		}

		imgURL := resp.Choices[0].Message.Images[0].ImageURL.URL
		if !strings.Contains(imgURL, "/mock-images/") {
			t.Errorf("expected image URL to contain /mock-images/, got %s", imgURL)
		}
	})

	t.Run("mock_mode_429", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/chat/completions", nil)
		req.Header.Set("X-Mock-Mode", "429")
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, req)

		if w.Code != http.StatusTooManyRequests {
			t.Errorf("expected status 429, got %d", w.Code)
		}
	})

	t.Run("mock_mode_500", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/chat/completions", nil)
		req.Header.Set("X-Mock-Mode", "500")
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, req)

		if w.Code != http.StatusInternalServerError {
			t.Errorf("expected status 500, got %d", w.Code)
		}
	})

	t.Run("mock_mode_empty", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/chat/completions", nil)
		req.Header.Set("X-Mock-Mode", "empty")
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("expected status 200, got %d", w.Code)
		}

		var resp struct {
			Choices []struct {
				Message struct {
					Images []interface{} `json:"images"`
				} `json:"message"`
			} `json:"choices"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
			t.Fatalf("failed to unmarshal response: %v", err)
		}
		if len(resp.Choices) > 0 && len(resp.Choices[0].Message.Images) > 0 {
			t.Error("expected no images in 'empty' mode")
		}
	})

	t.Run("mock_mode_timeout", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/chat/completions", nil)
		req.Header.Set("X-Mock-Mode", "timeout")
		req.Header.Set("X-Mock-Delay-Ms", "100") // 100ms
		w := httptest.NewRecorder()

		start := time.Now()
		mux.ServeHTTP(w, req)
		duration := time.Since(start)

		if duration < 100*time.Millisecond {
			t.Errorf("expected at least 100ms delay, got %v", duration)
		}
		if w.Code != http.StatusOK {
			t.Errorf("expected status 200, got %d", w.Code)
		}
	})

	t.Run("TestChatCompletions_gptImage2_returnsImagesArray", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/chat/completions", strings.NewReader(`{"model":"gpt-image-2","messages":[{"role":"user","content":[{"type":"text","text":"generate image"}]}]}`))
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("expected status 200, got %d", w.Code)
		}

		var resp struct {
			Choices []struct {
				Message struct {
					Images []struct {
						ImageURL struct {
							URL string `json:"url"`
						} `json:"image_url"`
					} `json:"images"`
				} `json:"message"`
			} `json:"choices"`
		}

		if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
			t.Fatalf("failed to unmarshal response: %v", err)
		}

		if len(resp.Choices) == 0 || len(resp.Choices[0].Message.Images) == 0 {
			t.Fatal("response should contain at least one image in images[] array")
		}

		imgURL := resp.Choices[0].Message.Images[0].ImageURL.URL
		if !strings.Contains(imgURL, "/mock-images/") {
			t.Errorf("expected image URL to contain /mock-images/, got %s", imgURL)
		}
	})

	t.Run("TestChatCompletions_gemini_returnsContentFormat", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/chat/completions", strings.NewReader(`{"model":"gemini-3-pro-image","messages":[{"role":"user","content":[{"type":"text","text":"generate image"}]}]}`))
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("expected status 200, got %d", w.Code)
		}

		var resp struct {
			Choices []struct {
				Message struct {
					Content json.RawMessage `json:"content"`
				} `json:"message"`
			} `json:"choices"`
		}

		if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
			t.Fatalf("failed to unmarshal response: %v", err)
		}

		if len(resp.Choices) == 0 || len(resp.Choices[0].Message.Content) == 0 {
			t.Fatal("response should contain content field")
		}

		contentStr := string(resp.Choices[0].Message.Content)
		if !strings.Contains(contentStr, "/mock-images/") {
			t.Errorf("expected content to contain /mock-images/, got %s", contentStr)
		}
	})
}

func TestMockImages(t *testing.T) {
	mux := setupMux()

	t.Run("get_fallback", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/mock-images/fallback.png", nil)
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("expected status 200, got %d", w.Code)
		}
		if w.Header().Get("Content-Type") != "image/png" {
			t.Errorf("expected content-type image/png, got %s", w.Header().Get("Content-Type"))
		}
		if len(w.Body.Bytes()) != len(FallbackImage) {
			t.Errorf("expected body length %d, got %d", len(FallbackImage), len(w.Body.Bytes()))
		}
	})

	t.Run("path_traversal_protection", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/mock-images/../../etc/passwd", nil)
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, req)

		// http.ServeFile usually handles this, but we should verify.
		// It might return 400 or 404 or just redirect.
		if w.Code == http.StatusOK && strings.Contains(w.Body.String(), "root:") {
			t.Error("detected path traversal vulnerability")
		}
	})
}
