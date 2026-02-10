package service

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"math"
	"testing"
)

func TestCompressToJPEGUnderSize_SmallPNG(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 64, 64))
	for y := 0; y < 64; y++ {
		for x := 0; x < 64; x++ {
			a := uint8(255)
			if (x+y)%7 == 0 {
				a = 120
			}
			img.SetRGBA(x, y, color.RGBA{R: uint8(x * 4), G: uint8(y * 4), B: 200, A: a})
		}
	}

	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("encode png: %v", err)
	}

	maxBytes := int64(50 * 1024)
	jpegBytes, width, height, err := CompressToJPEGUnderSize(bytes.NewReader(buf.Bytes()), maxBytes)
	if err != nil {
		t.Fatalf("CompressToJPEGUnderSize: %v", err)
	}
	if width <= 0 || height <= 0 {
		t.Fatalf("invalid dimensions: %dx%d", width, height)
	}
	if int64(len(jpegBytes)) > maxBytes {
		t.Fatalf("jpeg size %d exceeds maxBytes %d", len(jpegBytes), maxBytes)
	}
	if len(jpegBytes) < 2 || jpegBytes[0] != 0xFF || jpegBytes[1] != 0xD8 {
		t.Fatalf("jpeg missing SOI header")
	}
}

func TestCompressToJPEGUnderSize_Downscale(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 3000, 1000))
	for y := 0; y < 1000; y++ {
		for x := 0; x < 3000; x++ {
			img.SetRGBA(x, y, color.RGBA{R: uint8(x % 256), G: uint8(y % 256), B: 120, A: 255})
		}
	}

	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("encode png: %v", err)
	}

	maxBytes := int64(5 * 1024 * 1024)
	_, width, height, err := CompressToJPEGUnderSize(bytes.NewReader(buf.Bytes()), maxBytes)
	if err != nil {
		t.Fatalf("CompressToJPEGUnderSize: %v", err)
	}
	if width <= 0 || height <= 0 {
		t.Fatalf("invalid dimensions: %dx%d", width, height)
	}
	if width > 2048 {
		t.Fatalf("expected width <= 2048, got %d", width)
	}

	expectedRatio := 3.0
	gotRatio := float64(width) / float64(height)
	if math.Abs(gotRatio-expectedRatio) > 0.05 {
		t.Fatalf("unexpected aspect ratio: got %.3f", gotRatio)
	}
}

func TestCompressToJPEGUnderSize_InvalidMaxBytes(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 10, 10))
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("encode png: %v", err)
	}

	if _, _, _, err := CompressToJPEGUnderSize(bytes.NewReader(buf.Bytes()), 0); err == nil {
		t.Fatalf("expected error for maxBytes <= 0")
	}
}
