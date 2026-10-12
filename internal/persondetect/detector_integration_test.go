package persondetect

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func newIntegrationDetector(t *testing.T, backend Backend) Detector {
	t.Helper()
	cache := integrationCacheDir(t)
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	lastPhase := ""
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()
	detector, err := NewDetector(ctx, DetectorOptions{
		ModelDir:        filepath.Join(cache, "persondetect", "models"),
		RuntimeCacheDir: filepath.Join(cache, "embeddings"),
		Model:           ModelStandard,
		Backend:         backend,
		Logger:          slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo})),
		Executable:      executable,
		OnProgress: func(progress SetupProgress) {
			if progress.Phase != lastPhase {
				lastPhase = progress.Phase
				t.Logf("setup phase %s", progress.Phase)
			}
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = detector.Close() })
	return detector
}

func detectReference(t *testing.T, detector Detector) DetectResult {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	result, err := detector.Detect(ctx, ReferenceImage(), 0.5)
	if err != nil {
		t.Fatal(err)
	}
	if result.Width != 800 || result.Height != 533 {
		t.Fatalf("frame size %dx%d, want 800x533", result.Width, result.Height)
	}
	if len(result.Boxes) < 3 {
		t.Fatalf("%d persons >= 0.5 on the reference image, want >= 3 (Python/PIL: 4): %+v", len(result.Boxes), result.Boxes)
	}
	return result
}

func TestDetectorIntegrationCPU(t *testing.T) {
	requireIntegration(t)
	detector := newIntegrationDetector(t, BackendCPU)
	info := detector.Info()
	if info.Active != BackendCPU || !info.SelfTest.Passed || info.Runtime != "onnxruntime 1.26.0" {
		t.Fatalf("info = %+v", info)
	}
	result := detectReference(t, detector)
	t.Logf("CPU: %d persons >= 0.5, wall %.1f ms, CPU %.1f ms per frame; self-test %+v", len(result.Boxes), result.WallMS, result.CPUMS, info.SelfTest)
}

func TestDetectorIntegrationGPU(t *testing.T) {
	requireIntegration(t)
	if os.Getenv("AURAGO_PERSONDETECT_GPU") != "1" {
		t.Skip("set AURAGO_PERSONDETECT_GPU=1 to run the WebGPU path on a GPU")
	}
	detector := newIntegrationDetector(t, BackendGPU)
	info := detector.Info()
	if info.Active != BackendGPU {
		t.Fatalf("GPU path not active: %s (self-test %+v)", info.FallbackReason, info.SelfTest)
	}
	if !info.SelfTest.Passed || info.Runtime != "onnxruntime 1.27.0" {
		t.Fatalf("info = %+v", info)
	}
	result := detectReference(t, detector)
	t.Logf("GPU: %d persons >= 0.5, wall %.1f ms, CPU %.1f ms per frame; self-test %+v", len(result.Boxes), result.WallMS, result.CPUMS, info.SelfTest)
}
