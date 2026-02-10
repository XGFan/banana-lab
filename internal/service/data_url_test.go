package service

import (
	"bytes"
	"testing"
)

func TestDecodeDataURL_OK(t *testing.T) {
	t.Parallel()

	dataURL := "data:image/png;base64,aGVsbG8="
	mime, data, err := DecodeDataURL(dataURL)
	if err != nil {
		t.Fatalf("DecodeDataURL returned error: %v", err)
	}
	if mime != "image/png" {
		t.Fatalf("expected mime image/png, got %q", mime)
	}

	expected := []byte("hello")
	if !bytes.Equal(data, expected) {
		t.Fatalf("decoded data mismatch: got %q, want %q", data, expected)
	}
}

func TestDecodeDataURL_Errors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		dataURL string
	}{
		{
			name:    "missing prefix",
			dataURL: "image/png;base64,aGVsbG8=",
		},
		{
			name:    "missing base64 payload",
			dataURL: "data:image/png,abcd",
		},
		{
			name:    "missing mime",
			dataURL: "data:;base64,aGVsbG8=",
		},
		{
			name:    "mime with parameters",
			dataURL: "data:image/png;charset=utf-8;base64,aGVsbG8=",
		},
		{
			name:    "invalid base64",
			dataURL: "data:image/png;base64,***",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if _, _, err := DecodeDataURL(tt.dataURL); err == nil {
				t.Fatalf("expected error for %q", tt.dataURL)
			}
		})
	}
}
