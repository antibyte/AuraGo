package server

import (
	"bytes"
	"context"
	"image"
	"image/color"
	"image/jpeg"
	"testing"
	"time"

	"aurago/internal/config"
	"aurago/internal/decisions"
	"aurago/internal/decisions/consumers"
)

func TestConsumerImageDownscales(t *testing.T) {
	src := image.NewRGBA(image.Rect(0, 0, 1280, 720))
	for y := 0; y < 720; y++ {
		for x := 0; x < 1280; x++ {
			src.Set(x, y, color.RGBA{uint8(x), uint8(y), 90, 255})
		}
	}
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, src, nil); err != nil {
		t.Fatal(err)
	}
	got, err := prepareConsumerImage(buf.Bytes(), 448)
	if err != nil || got.MIME != "image/jpeg" {
		t.Fatalf("mime=%q err=%v", got.MIME, err)
	}
	cfg, _, err := image.DecodeConfig(bytes.NewReader(got.Data))
	if err != nil || cfg.Width != 448 || cfg.Height != 252 {
		t.Fatalf("size %dx%d err=%v", cfg.Width, cfg.Height, err)
	}
	small, err := prepareConsumerImage(got.Data, 1000)
	if err != nil || !bytes.Equal(small.Data, got.Data) {
		t.Fatal("an image within bounds must not be re-encoded")
	}
}

func TestConsumerImageRejectsOversize(t *testing.T) {
	if _, err := prepareConsumerImage(make([]byte, decisionImageBytes+1), 448); err == nil {
		t.Fatal("oversize accepted")
	}
	if _, err := prepareConsumerImage([]byte("not an image"), 448); err == nil {
		t.Fatal("garbage accepted")
	}
}

func TestCatalogRefreshRateLimited(t *testing.T) {
	s, d, _, remote := routedDecisionFixture(t)
	enable(s, config.DecisionConsumerSwitch{Enabled: true, AllowExternalImages: true}, "person_wake")
	remote.models = func(context.Context) ([]decisions.DecisionModel, error) {
		return []decisions.DecisionModel{{ID: "microsoft/microsoft-decision-1", ContextLength: 8192, SupportsDecisions: true, SupportsImages: true}}, nil
	}
	hub := s.decisionHub()
	if got := hub.Snapshot(s.Cfg, consumers.PersonWake); got.Code != consumers.CodeImagesUnknown || remote.catalogCalls.Load() != 0 {
		t.Fatalf("snapshot must stay passive: %q calls=%d", got.Code, remote.catalogCalls.Load())
	}
	now := time.Now()
	hub.now = func() time.Time { return now }
	if got := hub.RefreshImageCapability(context.Background(), s.Cfg, consumers.PersonWake); !got.Available || !got.ImagesAllowed {
		t.Fatalf("after refresh: %q", got.Code)
	}
	// A stale catalog again inside the TTL must not trigger another read.
	d.mu.Lock()
	d.generation.status.ModelAvailability.CheckedAt = now.Add(-time.Hour)
	d.mu.Unlock()
	now = now.Add(time.Minute)
	if got := hub.RefreshImageCapability(context.Background(), s.Cfg, consumers.PersonWake); got.Code != consumers.CodeImagesUnknown {
		t.Fatalf("second refresh: %q", got.Code)
	}
	if remote.catalogCalls.Load() != 1 {
		t.Fatalf("catalog calls = %d", remote.catalogCalls.Load())
	}
}
