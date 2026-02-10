package main

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// proxyRequest and other types to match internal/service/proxy_client.go
type proxyRequest struct {
	Model       string         `json:"model"`
	Messages    []proxyMessage `json:"messages"`
	ImageConfig *imageConfig   `json:"image_config"`
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
	AspectRatio string `json:"aspect_ratio"`
	ImageSize   string `json:"image_size"`
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

var (
	port        = flag.Int("port", 18081, "Mock server port")
	storageRoot = flag.String("storage", "./storage/images", "Image storage root")
	seed        = flag.String("seed", "banana-mock-seed", "Random seed for selection")
)

func setupMux() *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("/chat/completions", handleChatCompletions)
	mux.HandleFunc("/mock-images/", handleMockImages)
	return mux
}

func main() {
	flag.Parse()
	mux := setupMux()
	addr := fmt.Sprintf(":%d", *port)
	log.Printf("Mock LLM server listening on %s", addr)
	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatal(err)
	}
}

func handleChatCompletions(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	mode := r.Header.Get("X-Mock-Mode")
	switch mode {
	case "429":
		http.Error(w, "Too Many Requests", http.StatusTooManyRequests)
		return
	case "500":
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	case "timeout":
		delayStr := r.Header.Get("X-Mock-Delay-Ms")
		delayMs, _ := strconv.Atoi(delayStr)
		if delayMs == 0 {
			delayMs = 2000
		}
		time.Sleep(time.Duration(delayMs) * time.Millisecond)
	}

	var req proxyRequest
	bodyBytes, _ := io.ReadAll(r.Body)
	if len(bodyBytes) > 0 {
		json.Unmarshal(bodyBytes, &req)
	}

	if mode == "empty" {
		resp := proxyResponse{
			Choices: []proxyChoice{{
				Message: proxyMessageResponse{
					Images: []proxyImageObject{},
				},
			}},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
		return
	}

	imageFile := selectImageDeterministic(req)

	baseURL := os.Getenv("MOCK_BASE_URL")
	if baseURL == "" {
		scheme := "http"
		if r.TLS != nil {
			scheme = "https"
		}
		baseURL = fmt.Sprintf("%s://%s", scheme, r.Host)
	}

	imageURL := fmt.Sprintf("%s/mock-images/%s", strings.TrimRight(baseURL, "/"), imageFile)

	resp := proxyResponse{
		Choices: []proxyChoice{{
			Message: proxyMessageResponse{
				Images: []proxyImageObject{
					{ImageURL: proxyImageURL{URL: imageURL}},
				},
			},
		}},
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func selectImageDeterministic(req proxyRequest) string {
	files := listImages(*storageRoot)
	if len(files) == 0 {
		return "fallback.png"
	}

	var prompt string
	var hasSourceImage bool
	for _, m := range req.Messages {
		if m.Role == "user" {
			for _, c := range m.Content {
				if c.Type == "text" {
					prompt += c.Text
				}
				if c.Type == "image_url" {
					hasSourceImage = true
				}
			}
		}
	}

	aspectRatio := ""
	if req.ImageConfig != nil {
		aspectRatio = req.ImageConfig.AspectRatio
	}

	key := fmt.Sprintf("%s|%s|%s|%v", *seed, prompt, aspectRatio, hasSourceImage)
	h := sha256.Sum256([]byte(key))

	val := binary.BigEndian.Uint64(h[:8])
	idx := val % uint64(len(files))

	return files[idx]
}

func listImages(root string) []string {
	var images []string
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return nil
	}
	filepath.Walk(absRoot, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if !info.IsDir() {
			ext := strings.ToLower(filepath.Ext(path))
			if ext == ".png" || ext == ".jpg" || ext == ".jpeg" || ext == ".webp" {
				rel, err := filepath.Rel(absRoot, path)
				if err == nil {
					images = append(images, rel)
				}
			}
		}
		return nil
	})
	return images
}

func handleMockImages(w http.ResponseWriter, r *http.Request) {
	relPath := strings.TrimPrefix(r.URL.Path, "/mock-images/")
	if relPath == "fallback.png" {
		w.Header().Set("Content-Type", "image/png")
		w.Write(FallbackImage)
		return
	}

	absRoot, err := filepath.Abs(*storageRoot)
	if err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	fullPath := filepath.Join(absRoot, filepath.Clean(relPath))
	if !strings.HasPrefix(fullPath, absRoot) {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	http.ServeFile(w, r, fullPath)
}
