package tools

import (
	"bytes"
	"image"
	"image/color"
	"image/jpeg"
	"testing"
)

func go2RTCTestJPEG(w, h int) []byte {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, color.RGBA{R: uint8(x * 255 / max(1, w)), G: uint8(y * 255 / max(1, h)), A: 255})
		}
	}
	var b bytes.Buffer
	_ = jpeg.Encode(&b, img, nil)
	return b.Bytes()
}

func TestGo2RTCJPEGTransforms(t *testing.T) {
	data := go2RTCTestJPEG(80, 40)
	for _, tc := range []struct{ rotate, w, h, outW, outH int }{{90, 30, 40, 20, 40}, {180, 30, 30, 30, 15}, {270, 0, 20, 10, 20}, {0, 160, 80, 80, 40}} {
		out, err := transformGo2RTCJPEG(data, Go2RTCSnapshotOptions{Width: tc.w, Height: tc.h, Rotate: tc.rotate})
		if err != nil {
			t.Fatal(err)
		}
		cfg, err := jpeg.DecodeConfig(bytes.NewReader(out))
		if err != nil || cfg.Width != tc.outW || cfg.Height != tc.outH {
			t.Fatalf("transform %+v: %dx%d, %v", tc, cfg.Width, cfg.Height, err)
		}
	}
	if _, err := transformGo2RTCJPEG([]byte{255, 216, 255, 217}, Go2RTCSnapshotOptions{Width: 20}); err == nil {
		t.Fatal("invalid image accepted")
	}
	if _, err := transformGo2RTCJPEG([]byte{255, 216, 255, 217}, Go2RTCSnapshotOptions{}); err == nil {
		t.Fatal("untransformed invalid JPEG accepted")
	}
	huge := append([]byte(nil), data...)
	for i := 0; i+9 < len(huge); i++ {
		if huge[i] == 255 && huge[i+1] == 192 {
			huge[i+5], huge[i+6], huge[i+7], huge[i+8] = 0x7f, 0xff, 0x7f, 0xff
			break
		}
	}
	if _, err := transformGo2RTCJPEG(huge, Go2RTCSnapshotOptions{}); err == nil {
		t.Fatal("excessive dimensions accepted without decoding")
	}
	out, err := transformGo2RTCJPEG(data, Go2RTCSnapshotOptions{})
	if err != nil || !bytes.Equal(out, data) {
		t.Fatal("original changed")
	}
}
