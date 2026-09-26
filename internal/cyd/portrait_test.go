package cyd

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"testing"
)

func TestEncodePortraitRGB565SolidRed(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 16, 16))
	for y := 0; y < 16; y++ {
		for x := 0; x < 16; x++ {
			img.SetRGBA(x, y, color.RGBA{R: 255, A: 255})
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	out, err := EncodePortraitRGB565(buf.Bytes(), 8)
	if err != nil {
		t.Fatal(err)
	}
	if len(out) != 128 {
		t.Fatalf("len = %d", len(out))
	}
	if out[0] != 0x00 || out[1] != 0xF8 {
		t.Fatalf("pixel = %02x %02x, want little-endian 0xF800", out[0], out[1])
	}
}
