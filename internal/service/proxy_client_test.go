package service

import (
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestReadBodyLimited(t *testing.T) {
	tests := []struct {
		name      string
		body      string
		limit     int
		wantBody  string
		wantError string
	}{
		{
			name:     "within limit",
			body:     "hello world",
			limit:    20,
			wantBody: "hello world",
		},
		{
			name:      "exceeds limit",
			body:      "hello world",
			limit:     5,
			wantBody:  "hello",
			wantError: "proxy response exceeds limit of 5 bytes",
		},
		{
			name:     "empty body",
			body:     "",
			limit:    10,
			wantBody: "",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			resp := &http.Response{
				Body: io.NopCloser(strings.NewReader(test.body)),
			}
			got, err := readBodyLimited(resp, test.limit)
			if test.wantError != "" {
				if err == nil || !strings.Contains(err.Error(), test.wantError) {
					t.Fatalf("expected error containing %q, got %v", test.wantError, err)
				}
			} else if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if string(got) != test.wantBody {
				t.Fatalf("expected body %q, got %q", test.wantBody, string(got))
			}
		})
	}
}

func TestParseProxyResponse(t *testing.T) {
	tests := []struct {
		name      string
		payload   string
		wantURLs  []string
		wantError bool
	}{
		{
			name: "images field",
			payload: `{
  "choices": [
    {
      "message": {
        "images": [
          {"image_url": {"url": "https://example.com/image.png"}}
        ],
        "content": ""
      }
    }
  ]
}`,
			wantURLs: []string{"https://example.com/image.png"},
		},
		{
			name: "content string data url",
			payload: `{
  "choices": [
    {
      "message": {
        "content": "prefix data:image/png;base64,AAAA suffix"
      }
    }
  ]
}`,
			wantURLs: []string{"data:image/png;base64,AAAA"},
		},
		{
			name: "content parts with data url",
			payload: `{
  "choices": [
    {
      "message": {
        "content": [
          {"type": "text", "text": "note data:image/png;base64,BBBB"}
        ]
      }
    }
  ]
}`,
			wantURLs: []string{"data:image/png;base64,BBBB"},
		},
		{
			name:      "missing choices",
			payload:   `{"choices": []}`,
			wantError: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			images, err := parseProxyResponse([]byte(test.payload))
			if test.wantError {
				if err == nil {
					t.Fatalf("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(images) != len(test.wantURLs) {
				t.Fatalf("expected %d images, got %d", len(test.wantURLs), len(images))
			}
			for i, url := range test.wantURLs {
				if images[i].URL != url {
					t.Fatalf("expected url %q, got %q", url, images[i].URL)
				}
			}
		})
	}
}

func TestParseProxyResponse_Empty(t *testing.T) {
	_, err := parseProxyResponse([]byte(""))
	if err == nil {
		t.Fatal("expected error for empty body, got nil")
	}
	t.Logf("Got expected error: %v", err)
}

func TestProxyClientEmptyImages(t *testing.T) {
	payload := `{
  "choices": [
    {
      "message": {
        "images": [],
        "content": ""
      }
    }
  ]
}`
	_, err := parseProxyResponse([]byte(payload))
	if err == nil {
		t.Fatal("expected error for empty images, got nil")
	}
	if !strings.Contains(err.Error(), "contains no images") {
		t.Fatalf("expected error containing 'contains no images', got %v", err)
	}
	t.Log("empty image result handled")
}
