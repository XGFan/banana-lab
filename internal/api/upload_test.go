package api

import (
	"bytes"
	"encoding/json"
	"image"
	"image/png"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/gin-gonic/gin"

	"banana-lab/internal/model"
	"banana-lab/internal/storage"
)

type xorshift32 struct {
	state uint32
}

func newXorshift32(seed uint32) *xorshift32 {
	if seed == 0 {
		seed = 1
	}
	return &xorshift32{state: seed}
}

func (x *xorshift32) Next() uint32 {
	s := x.state
	s ^= s << 13
	s ^= s >> 17
	s ^= s << 5
	x.state = s
	return s
}

func buildRandomPNG(t *testing.T, width, height int) []byte {
	t.Helper()

	img := image.NewRGBA(image.Rect(0, 0, width, height))
	rng := newXorshift32(0x12345678)
	for i := 0; i < len(img.Pix); i += 4 {
		v := rng.Next()
		img.Pix[i] = byte(v)
		img.Pix[i+1] = byte(v >> 8)
		img.Pix[i+2] = byte(v >> 16)
		img.Pix[i+3] = 0xff
	}

	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("encode png: %v", err)
	}
	return buf.Bytes()
}

func TestUploadCompress(t *testing.T) {
	gin.SetMode(gin.TestMode)

	storageRoot := t.TempDir()
	dbPath := filepath.Join(t.TempDir(), "upload.db")
	db, err := model.Open(dbPath)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := model.Migrate(db); err != nil {
		t.Fatalf("migrate db: %v", err)
	}

	router := gin.New()
	router.POST("/api/upload", (&UploadHandler{DB: db, Storage: storage.Storage{Root: storageRoot}}).Handle)

	pngBytes := buildRandomPNG(t, 3072, 3072)
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, err := writer.CreateFormFile("file", "upload.png")
	if err != nil {
		t.Fatalf("create form file: %v", err)
	}
	if _, err := part.Write(pngBytes); err != nil {
		t.Fatalf("write png: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close writer: %v", err)
	}

	request, err := http.NewRequest(http.MethodPost, "/api/upload", body)
	if err != nil {
		t.Fatalf("create request: %v", err)
	}
	request.Header.Set("Content-Type", writer.FormDataContentType())

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("unexpected status: %d", recorder.Code)
	}

	var payload map[string]any
	if err := json.NewDecoder(recorder.Body).Decode(&payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	imageVal, ok := payload["image"].(map[string]any)
	if !ok {
		t.Fatalf("expected image object")
	}

	imageID := getStringField(t, imageVal, "id", "ID")
	filePath := getStringField(t, imageVal, "file_path", "FilePath")
	fileSize := getInt64Field(t, imageVal, "file_size", "FileSize")
	if fileSize > 2*1024*1024 {
		t.Fatalf("expected image file_size <= 2MB, got %d", fileSize)
	}

	var stored model.Image
	if err := db.First(&stored, "id = ?", imageID).Error; err != nil {
		t.Fatalf("load image: %v", err)
	}
	if stored.Origin != "uploaded" {
		t.Fatalf("unexpected origin: %s", stored.Origin)
	}
	if stored.NodeID != nil {
		t.Fatalf("expected node_id nil")
	}

	absPath := filepath.Join(storageRoot, filePath)
	if _, err := os.Stat(absPath); err != nil {
		t.Fatalf("expected file on disk: %v", err)
	}
}

func getStringField(t *testing.T, data map[string]any, keys ...string) string {
	t.Helper()
	for _, key := range keys {
		if value, ok := data[key]; ok {
			str, ok := value.(string)
			if !ok {
				t.Fatalf("expected %s to be string", key)
			}
			if str == "" {
				t.Fatalf("expected %s to be set", key)
			}
			return str
		}
	}
	t.Fatalf("expected one of keys %v", keys)
	return ""
}

func getInt64Field(t *testing.T, data map[string]any, keys ...string) int64 {
	t.Helper()
	for _, key := range keys {
		if value, ok := data[key]; ok {
			switch typed := value.(type) {
			case float64:
				return int64(typed)
			case int64:
				return typed
			case int:
				return int64(typed)
			default:
				t.Fatalf("expected %s to be number", key)
			}
		}
	}
	t.Fatalf("expected one of keys %v", keys)
	return 0
}
