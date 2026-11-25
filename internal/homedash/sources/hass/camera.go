package hass

import (
	"context"
	"errors"
	"fmt"
	"mime"
	"net/http"

	"aurago/internal/homedash"
	"aurago/internal/tools"
)

// Snapshot reads one camera image through Home Assistant's camera proxy (at most 8 MiB enforced
// while reading, never stored). Only JPEG, PNG and WebP pass, and the bytes must match the declared
// type. maxW and maxH are accepted and ignored in M3 (no scaling; the camera widget is M5).
// Errors never carry Home Assistant text; the hub reports them as source_error.
func (a *Adapter) Snapshot(ctx context.Context, native string, _, _ int) ([]byte, string, error) {
	domain, ok := domainOf(native)
	if !ok || domain != "camera" {
		return nil, "", errors.New("hass: not a camera entity")
	}
	img, ct, err := tools.HAGetBytes(ctx, a.ha, "/api/camera_proxy/"+native, homedash.MaxSnapshotBytes)
	if err != nil {
		a.log.Debug("homedash: Home Assistant camera snapshot failed", "entity", native, "error", err)
		return nil, "", fmt.Errorf("hass camera: %w", err)
	}
	mt, _, _ := mime.ParseMediaType(ct)
	sniffed := http.DetectContentType(img)
	switch {
	case mt == "image/jpeg" && sniffed == "image/jpeg", mt == "image/png" && sniffed == "image/png", mt == "image/webp" && sniffed == "image/webp":
		return img, mt, nil
	}
	return nil, "", fmt.Errorf("hass camera: unsupported image type %q", mt)
}
