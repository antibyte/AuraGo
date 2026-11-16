package server

import (
	"bytes"
	"context"
	"encoding/json"
	"image"
	"image/color"
	"image/png"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"aurago/internal/desktop"
)

func desktopWallpaperPNG(t *testing.T, fill color.Color) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 2, 3))
	if fill != nil {
		for y := 0; y < 3; y++ {
			for x := 0; x < 2; x++ {
				img.Set(x, y, fill)
			}
		}
	}
	var buffer bytes.Buffer
	if err := png.Encode(&buffer, img); err != nil {
		t.Fatalf("encode wallpaper test PNG: %v", err)
	}
	return buffer.Bytes()
}

func TestDesktopWallpaperHandlerServesValidatedSnapshot(t *testing.T) {
	s := newDesktopFilesystemTestServer(t)
	svc, _, err := s.getDesktopService(context.Background())
	if err != nil {
		t.Fatalf("getDesktopService: %v", err)
	}
	first := desktopWallpaperPNG(t, color.RGBA{R: 0xff, A: 0xff})
	if err := svc.WriteFileBytes(context.Background(), "Pictures/wall.png", first, desktop.SourceUser); err != nil {
		t.Fatalf("write wallpaper: %v", err)
	}
	request := func(method string) *httptest.ResponseRecorder {
		t.Helper()
		req := httptest.NewRequest(method, "/api/desktop/wallpaper?path=Pictures%2Fwall.png", nil)
		resp := httptest.NewRecorder()
		handleDesktopWallpaper(s).ServeHTTP(resp, req)
		return resp
	}
	resp := request(http.MethodGet)
	if resp.Code != http.StatusOK {
		t.Fatalf("wallpaper status = %d, body %s", resp.Code, resp.Body.String())
	}
	if resp.Header().Get("Content-Type") != "image/png" || resp.Header().Get("X-Content-Type-Options") != "nosniff" || resp.Header().Get("Cache-Control") != "private, no-store" {
		t.Fatalf("wallpaper security/content headers = %v", resp.Header())
	}
	if !bytes.Equal(resp.Body.Bytes(), first) {
		t.Fatal("handler response does not contain the exact validated bytes")
	}
	if got := resp.Header().Get("Content-Length"); got != strconv.Itoa(len(first)) {
		t.Fatalf("Content-Length = %q, want %d", got, len(first))
	}
	head := request(http.MethodHead)
	if head.Code != http.StatusOK || head.Body.Len() != 0 || head.Header().Get("Content-Length") != strconv.Itoa(len(first)) {
		t.Fatalf("HEAD response = status %d, bytes %d, length %q", head.Code, head.Body.Len(), head.Header().Get("Content-Length"))
	}

	// A changed file at the same path is freshly validated and served on the next load.
	second := desktopWallpaperPNG(t, color.RGBA{B: 0xff, A: 0xff})
	if err := svc.WriteFileBytes(context.Background(), "Pictures/wall.png", second, desktop.SourceUser); err != nil {
		t.Fatalf("replace wallpaper: %v", err)
	}
	resp = request(http.MethodGet)
	if resp.Code != http.StatusOK || !bytes.Equal(resp.Body.Bytes(), second) {
		t.Fatalf("replacement wallpaper response = status %d, exact new bytes %t", resp.Code, bytes.Equal(resp.Body.Bytes(), second))
	}
}

func TestDesktopWallpaperHandlerReturnsStableValidationCodes(t *testing.T) {
	s := newDesktopFilesystemTestServer(t)
	if err := os.MkdirAll(filepath.Join(s.Cfg.VirtualDesktop.WorkspaceDir, "Pictures"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(s.Cfg.VirtualDesktop.WorkspaceDir, "Pictures", "wrong.jpg"), desktopWallpaperPNG(t, nil), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		path string
		want string
		code int
	}{
		{path: "../outside.png", want: "wallpaper_invalid_path", code: http.StatusBadRequest},
		{path: "Pictures/missing.png", want: "wallpaper_not_found", code: http.StatusNotFound},
		{path: "Pictures/wrong.jpg", want: "wallpaper_format_mismatch", code: http.StatusUnsupportedMediaType},
	} {
		t.Run(test.want, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/api/desktop/wallpaper?path="+url.QueryEscape(test.path), nil)
			resp := httptest.NewRecorder()
			handleDesktopWallpaper(s).ServeHTTP(resp, req)
			if resp.Code != test.code {
				t.Fatalf("wallpaper %q status = %d, body %s; want %d", test.path, resp.Code, resp.Body.String(), test.code)
			}
			var body map[string]string
			if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
				t.Fatalf("decode error response: %v", err)
			}
			if body["code"] != test.want || body["error"] != test.want {
				t.Fatalf("wallpaper error body = %+v, want stable code %q", body, test.want)
			}
		})
	}
}
