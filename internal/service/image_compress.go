package service

import (
	"bytes"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	_ "image/gif"
	"image/jpeg"
	_ "image/png"
	"io"
	"math"
)

const maxImageDimension = 2048

var errTooLarge = errors.New("compressed image exceeds max size")

// CompressToJPEGUnderSize decodes an image, resizes as needed, and encodes to JPEG under maxBytes.
func CompressToJPEGUnderSize(r io.Reader, maxBytes int64) (jpegBytes []byte, width int, height int, err error) {
	if maxBytes <= 0 {
		return nil, 0, 0, fmt.Errorf("maxBytes must be positive")
	}

	src, _, err := image.Decode(r)
	if err != nil {
		return nil, 0, 0, fmt.Errorf("decode image: %w", err)
	}

	if src == nil {
		return nil, 0, 0, fmt.Errorf("decoded image is nil")
	}

	srcBounds := src.Bounds()
	if srcBounds.Dx() <= 0 || srcBounds.Dy() <= 0 {
		return nil, 0, 0, fmt.Errorf("invalid image dimensions")
	}

	current := scaleToMaxDimension(src, maxImageDimension)

	for {
		prepared := prepareForJPEG(current)
		encoded, encErr := encodeWithQualityLoop(prepared, maxBytes)
		if encErr == nil {
			outBounds := prepared.Bounds()
			return encoded, outBounds.Dx(), outBounds.Dy(), nil
		}
		if !errors.Is(encErr, errTooLarge) {
			return nil, 0, 0, encErr
		}

		curBounds := prepared.Bounds()
		curW, curH := curBounds.Dx(), curBounds.Dy()
		if curW <= 1 || curH <= 1 {
			return nil, curW, curH, fmt.Errorf("unable to compress under %d bytes", maxBytes)
		}

		nextW := int(math.Floor(float64(curW) * 0.9))
		nextH := int(math.Floor(float64(curH) * 0.9))
		if nextW < 1 {
			nextW = 1
		}
		if nextH < 1 {
			nextH = 1
		}
		if nextW == curW && nextH == curH {
			return nil, curW, curH, fmt.Errorf("unable to further downscale")
		}

		current = scaleNN(current, nextW, nextH)
	}
}

func encodeWithQualityLoop(img image.Image, maxBytes int64) ([]byte, error) {
	qualities := []int{85, 80, 70, 60, 50, 40, 30}
	for _, quality := range qualities {
		var buf bytes.Buffer
		if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: quality}); err != nil {
			return nil, fmt.Errorf("encode jpeg (quality %d): %w", quality, err)
		}
		if int64(buf.Len()) <= maxBytes {
			return buf.Bytes(), nil
		}
	}
	return nil, errTooLarge
}

func scaleToMaxDimension(src image.Image, maxDim int) image.Image {
	if maxDim <= 0 {
		return src
	}
	bounds := src.Bounds()
	w, h := bounds.Dx(), bounds.Dy()
	if w <= 0 || h <= 0 {
		return src
	}

	maxSide := w
	if h > maxSide {
		maxSide = h
	}
	if maxSide <= maxDim {
		return src
	}

	scale := float64(maxDim) / float64(maxSide)
	newW := int(math.Round(float64(w) * scale))
	newH := int(math.Round(float64(h) * scale))
	if newW < 1 {
		newW = 1
	}
	if newH < 1 {
		newH = 1
	}
	return scaleNN(src, newW, newH)
}

func prepareForJPEG(src image.Image) image.Image {
	if hasAlpha(src) {
		return toRGBAWithWhiteBG(src)
	}

	if _, ok := src.(*image.RGBA); ok {
		return src
	}

	bounds := src.Bounds()
	dst := image.NewRGBA(image.Rect(0, 0, bounds.Dx(), bounds.Dy()))
	draw.Draw(dst, dst.Bounds(), src, bounds.Min, draw.Src)
	return dst
}

func hasAlpha(img image.Image) bool {
	bounds := img.Bounds()
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			_, _, _, a := img.At(x, y).RGBA()
			if a != 0xffff {
				return true
			}
		}
	}
	return false
}

func toRGBAWithWhiteBG(src image.Image) *image.RGBA {
	bounds := src.Bounds()
	dst := image.NewRGBA(image.Rect(0, 0, bounds.Dx(), bounds.Dy()))
	draw.Draw(dst, dst.Bounds(), &image.Uniform{C: color.White}, image.Point{}, draw.Src)
	draw.Draw(dst, dst.Bounds(), src, bounds.Min, draw.Over)
	return dst
}

func scaleNN(src image.Image, newW int, newH int) *image.RGBA {
	if newW <= 0 || newH <= 0 {
		return image.NewRGBA(image.Rect(0, 0, 0, 0))
	}

	bounds := src.Bounds()
	srcW, srcH := bounds.Dx(), bounds.Dy()
	if srcW <= 0 || srcH <= 0 {
		return image.NewRGBA(image.Rect(0, 0, 0, 0))
	}

	dst := image.NewRGBA(image.Rect(0, 0, newW, newH))
	for y := 0; y < newH; y++ {
		srcY := bounds.Min.Y + y*srcH/newH
		for x := 0; x < newW; x++ {
			srcX := bounds.Min.X + x*srcW/newW
			dst.Set(x, y, src.At(srcX, srcY))
		}
	}
	return dst
}
