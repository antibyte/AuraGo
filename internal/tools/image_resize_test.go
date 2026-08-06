package tools

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"testing"
)

func TestUpscaleImageBytesDoublesDimensions(t *testing.T) {
	src := image.NewRGBA(image.Rect(0, 0, 4, 4))
	for y := 0; y < 4; y++ {
		for x := 0; x < 4; x++ {
			src.Set(x, y, color.RGBA{R: 128, G: 64, B: 32, A: 255})
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, src); err != nil {
		t.Fatalf("encode source png: %v", err)
	}

	result, err := UpscaleImageBytes(buf.Bytes(), 2)
	if err != nil {
		t.Fatalf("UpscaleImageBytes: %v", err)
	}
	if result.Width != 8 || result.Height != 8 {
		t.Fatalf("expected 8x8, got %dx%d", result.Width, result.Height)
	}
	if len(result.PNGBytes) == 0 {
		t.Fatal("expected non-empty PNG bytes")
	}
}

func TestUpscaleImageBytesRejectsOversize(t *testing.T) {
	src := image.NewRGBA(image.Rect(0, 0, UpscaleMaxDimension, 2))
	var buf bytes.Buffer
	if err := png.Encode(&buf, src); err != nil {
		t.Fatalf("encode source png: %v", err)
	}
	if _, err := UpscaleImageBytes(buf.Bytes(), 2); err == nil {
		t.Fatal("expected error for oversized upscale")
	}
}
