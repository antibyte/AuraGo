package tools

import (
	"bytes"
	"context"
	"image"
	"image/color"
	"image/png"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"aurago/internal/config"
)

// Run with: AURAGO_LIVE_MIRAGE=1 go test ./internal/tools -run TestLiveMirageSidecar -count=1 -timeout 30m -v
// Optional: AURAGO_LIVE_DOCKER_HOST (default: the Docker Desktop named pipe).
func TestLiveMirageSidecar(t *testing.T) {
	if os.Getenv("AURAGO_LIVE_MIRAGE") != "1" {
		t.Skip("set AURAGO_LIVE_MIRAGE=1 to run against a real Docker daemon")
	}
	configureDockerSecurityTestPermissions(t, false)
	host := os.Getenv("AURAGO_LIVE_DOCKER_HOST")
	if host == "" {
		host = "npipe:////./pipe/docker_engine"
	}
	cfg := &config.Config{}
	cfg.Docker.Enabled = true
	cfg.Docker.Host = host
	m := &cfg.Vision.Mirage
	m.Enabled = true
	m.Mode = config.MirageModeAlways
	m.ContainerName = "aurago_mirage_livetest"
	m.ContainerPort = 18791
	m.Image = "python:3.12-slim"
	m.DataPath = filepath.Join(t.TempDir(), "mirage")
	m.GPU = "auto"
	m.MaxObjects = 20
	m.MaxTextLines = 30

	prev := mirageBaseURLFor
	mirageBaseURLFor = func(*config.Config) string { return "http://127.0.0.1:" + strconv.Itoa(m.ContainerPort) + "/v1" }
	t.Cleanup(func() {
		mirageBaseURLFor = prev
		_, _, _ = dockerRequest(DockerConfig{Host: host}, http.MethodDelete, "/containers/"+m.ContainerName+"?force=true", "")
	})

	if err := EnsureMirageRunning(cfg, mirageTestLogger(), true); err != nil {
		t.Fatalf("EnsureMirageRunning: %v", err)
	}
	status := MirageStatus(MirageStatusTargetFor(cfg))
	t.Logf("status: %+v", status)
	if !status.Ready {
		t.Fatalf("sidecar not ready: %+v", status)
	}

	img := image.NewRGBA(image.Rect(0, 0, 256, 256))
	for x := 0; x < 256; x++ {
		for y := 0; y < 256; y++ {
			img.Set(x, y, color.RGBA{R: uint8(x), G: uint8(y), B: 128, A: 255})
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "test.png")
	if err := os.WriteFile(path, buf.Bytes(), 0o600); err != nil {
		t.Fatal(err)
	}
	text, _, _, err := AnalyzeTrustedImageFileWithPromptContext(context.Background(), path, "describe", cfg)
	if err != nil {
		t.Fatalf("analyze: %v", err)
	}
	t.Logf("description:\n%s", text)
	if text == "" {
		t.Fatal("empty description")
	}
}
