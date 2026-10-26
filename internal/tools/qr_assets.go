package tools

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"time"

	"aurago/internal/qrcode"
	"aurago/internal/security"
	"aurago/internal/webassets"
)

var qrWorkSlots = make(chan struct{}, 2)

// AcquireQRWork bounds memory across human and native-agent entry points.
func AcquireQRWork() (func(), bool) {
	select {
	case qrWorkSlots <- struct{}{}:
		return func() { <-qrWorkSlots }, true
	default:
		return nil, false
	}
}

// QRRenderOptions gives HTTP and agent renders the same verified font resources.
func QRRenderOptions() qrcode.RenderOptions {
	return qrcode.RenderOptions{FontProvider: func(ctx context.Context, id string) ([]byte, error) {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		for _, font := range qrcode.Presets().Fonts {
			if font.ID == id && font.Resource != "" {
				data, err := webassets.Namespace("ui").ReadFile("fonts/qr/" + font.Resource)
				if err != nil {
					return nil, fmt.Errorf("verified QR font resources are unavailable")
				}
				return data, nil
			}
		}
		return nil, fmt.Errorf("unknown QR font")
	}}
}

// ImportQRImage downloads only public, uncredentialed HTTP(S) resources. Callers
// check their network grant; redirects and DNS pinning stay with the shared client.
func ImportQRImage(ctx context.Context, rawURL string) (qrcode.ImageResource, error) {
	var empty qrcode.ImageResource
	if len(rawURL) > 4096 {
		return empty, fmt.Errorf("image URL is too long")
	}
	if err := security.ValidatePublicHTTPURL(rawURL); err != nil {
		return empty, fmt.Errorf("image URL must address a public HTTP(S) resource")
	}
	ctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return empty, fmt.Errorf("invalid image URL")
	}
	req.Header.Set("Accept", "image/png,image/jpeg,image/webp,image/svg+xml")
	client := security.NewSSRFProtectedHTTPClient(20 * time.Second)
	resp, err := client.Do(req)
	if err != nil {
		return empty, fmt.Errorf("image download failed")
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return empty, fmt.Errorf("image download returned HTTP %d", resp.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, qrcode.MaxResourceBytes+1))
	if err != nil || len(data) > qrcode.MaxResourceBytes {
		return empty, fmt.Errorf("image download exceeds the resource limit or is incomplete")
	}
	return qrcode.NormalizeImage(ctx, resp.Header.Get("Content-Type"), data)
}
