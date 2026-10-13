package tools

import (
	"bytes"
	"encoding/base64"
	"image"
	"image/png"
	"strings"
	"testing"
)

func TestPrepareVisionDataURLDownscalesAndEncodesJPEG(t *testing.T) {
	img := image.NewNRGBA(image.Rect(0, 0, 2000, 1800))
	for i := 0; i < len(img.Pix); i += 4 {
		img.Pix[i] = byte(i)
		img.Pix[i+1] = byte(i >> 8)
		img.Pix[i+2] = byte(i >> 16)
		img.Pix[i+3] = byte((i + 13) % 256)
	}
	var raw bytes.Buffer
	if err := png.Encode(&raw, img); err != nil {
		t.Fatal(err)
	}
	uri, err := PrepareVisionDataURL(raw.Bytes(), "image/png")
	if err != nil {
		t.Fatalf("PrepareVisionDataURL: %v", err)
	}
	if !strings.HasPrefix(uri, "data:image/jpeg;base64,") {
		t.Fatalf("expected jpeg data URL, got prefix %q", uri[:min(len(uri), 40)])
	}
	payload := strings.TrimPrefix(uri, "data:image/jpeg;base64,")
	decoded, err := base64.StdEncoding.DecodeString(payload)
	if err != nil {
		t.Fatal(err)
	}
	got, _, err := image.Decode(bytes.NewReader(decoded))
	if err != nil {
		t.Fatal(err)
	}
	gb := got.Bounds()
	if gb.Dx() > 1600 || gb.Dy() > 1600 {
		t.Fatalf("expected downscaled <=1600px, got %dx%d", gb.Dx(), gb.Dy())
	}
}

func TestPrepareVisionDataURLRejectsUndecodedBytes(t *testing.T) {
	raw := []byte("not an image")
	uri, err := PrepareVisionDataURL(raw, "image/jpeg")
	if err != nil {
		if strings.Contains(uri, base64.StdEncoding.EncodeToString(raw)) {
			t.Fatalf("error path returned a data URL of the original bytes: %q", uri)
		}
		return
	}
	t.Fatalf("expected an error, got %q", uri)
}

func TestPrepareVisionDataURLRejectsTooManyPixels(t *testing.T) {
	img := image.NewNRGBA(image.Rect(0, 0, 2500, 1800))
	var raw bytes.Buffer
	if err := png.Encode(&raw, img); err != nil {
		t.Fatal(err)
	}
	if 2500*1800 <= 4_000_000 {
		t.Fatal("fixture must exceed 4_000_000 pixels")
	}
	uri, err := PrepareVisionDataURL(raw.Bytes(), "image/png")
	if err == nil {
		t.Fatalf("expected pixel-limit error, got data URL prefix %q", uri[:min(len(uri), 32)])
	}
	if strings.HasPrefix(uri, "data:") {
		t.Fatalf("error returned a data URL: %q", uri[:min(len(uri), 32)])
	}
}
