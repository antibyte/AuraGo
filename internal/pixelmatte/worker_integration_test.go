package pixelmatte

import (
	"bytes"
	"context"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// TestRealModelsIntegration runs the real worker subprocess on a synthetic
// image with the pinned models and an ONNX Runtime CPU bundle that are
// already on disk; it downloads nothing. Opt in with
//
//	AURAGO_PIXELMATTE_REAL=1
//	AURAGO_PIXELMATTE_RUNTIME=<directory with the onnxruntime library>
//	AURAGO_PIXELMATTE_MODELS=<directory with the models under their download names>
//
// Use -timeout 30m: BiRefNet is slow on a CPU.
func TestRealModelsIntegration(t *testing.T) {
	if os.Getenv("AURAGO_PIXELMATTE_REAL") != "1" {
		t.Skip("set AURAGO_PIXELMATTE_REAL=1 (and the runtime and model directories) to run")
	}
	runtimeRoot, modelDir := os.Getenv("AURAGO_PIXELMATTE_RUNTIME"), os.Getenv("AURAGO_PIXELMATTE_MODELS")
	if runtimeRoot == "" || modelDir == "" {
		t.Skip("AURAGO_PIXELMATTE_RUNTIME and AURAGO_PIXELMATTE_MODELS are required")
	}
	files := map[Kind]string{
		KindStandard: "isnet-general-use.onnx",
		KindPrecise:  "BiRefNet-general-bb_swin_v1_tiny-epoch_232.onnx",
	}
	source := syntheticSubject(480, 360)
	for _, kind := range []Kind{KindStandard, KindPrecise} {
		t.Run(string(kind), func(t *testing.T) {
			model := filepath.Join(modelDir, files[kind])
			if _, err := os.Stat(model); err != nil {
				t.Skipf("model not available: %v", err)
			}
			executable, err := os.Executable()
			if err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
			defer cancel()
			started := time.Now()
			matte, err := processRunner{}.Run(ctx, RunRequest{
				Executable: executable, RuntimeRoot: runtimeRoot, ModelPath: model,
				Kind: kind, Threads: DefaultThreads(), Image: source,
			})
			if err != nil {
				t.Fatalf("worker: %v", err)
			}
			t.Logf("%s: %v, %d bytes of PNG, %d threads", kind, time.Since(started), len(matte), DefaultThreads())
			decoded, err := png.Decode(bytes.NewReader(matte))
			if err != nil {
				t.Fatal(err)
			}
			gray, ok := decoded.(*image.Gray)
			if !ok || gray.Bounds().Dx() != 480 || gray.Bounds().Dy() != 360 {
				t.Fatalf("matte is %T %v", decoded, decoded.Bounds())
			}
			t.Logf("%s: centre %d, corner %d", kind, gray.GrayAt(240, 180).Y, gray.GrayAt(5, 5).Y)
		})
	}
}

// syntheticSubject is a warm-coloured disc with a darker core on a cool,
// noisy background.
func syntheticSubject(width, height int) []byte {
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			noise := uint8((x*31 + y*17) % 24)
			c := color.RGBA{R: 30 + noise, G: 60 + noise, B: 120 + noise, A: 255}
			dx, dy := float64(x-width/2), float64(y-height/2)
			if dx*dx+dy*dy < 110*110 {
				c = color.RGBA{R: 220, G: 150 + noise, B: 60, A: 255}
				if dx*dx+dy*dy < 40*40 {
					c = color.RGBA{R: 120, G: 60, B: 30, A: 255}
				}
			}
			img.SetRGBA(x, y, c)
		}
	}
	var buffer bytes.Buffer
	_ = png.Encode(&buffer, img)
	return buffer.Bytes()
}
