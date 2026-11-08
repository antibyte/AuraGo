package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"image"
	"image/color"
	"image/png"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"aurago/internal/config"
	"aurago/internal/testutil"
)

func mirageTestConfig() *config.Config {
	cfg := &config.Config{}
	cfg.Vision.Mirage.Enabled = true
	cfg.Vision.Mirage.Mode = config.MirageModeFallback
	return cfg
}

func overrideMirageHooks(t *testing.T, baseURL string, mainSupportsImage bool) {
	t.Helper()
	prevURL, prevMain := mirageBaseURLFor, mirageMainSupportsImage
	mirageBaseURLFor = func(*config.Config) string { return baseURL }
	mirageMainSupportsImage = func(*config.Config) bool { return mainSupportsImage }
	t.Cleanup(func() { mirageBaseURLFor, mirageMainSupportsImage = prevURL, prevMain })
}

func TestUseMirageVisionDecisionTable(t *testing.T) {
	overrideMirageHooks(t, "http://unused/v1", false)
	cases := []struct {
		name   string
		mutate func(*config.Config)
		main   bool
		want   bool
	}{
		{"disabled", func(c *config.Config) { c.Vision.Mirage.Enabled = false }, false, false},
		{"explicit provider wins", func(c *config.Config) { c.Vision.Provider = "openrouter" }, false, false},
		{"fallback and main cannot see", func(c *config.Config) {}, false, true},
		{"fallback and main can see", func(c *config.Config) {}, true, false},
		{"always even if main can see", func(c *config.Config) { c.Vision.Mirage.Mode = config.MirageModeAlways }, true, true},
		{"always but explicit provider", func(c *config.Config) { c.Vision.Mirage.Mode = config.MirageModeAlways; c.Vision.Provider = "p" }, true, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			mirageMainSupportsImage = func(*config.Config) bool { return tc.main }
			cfg := mirageTestConfig()
			tc.mutate(cfg)
			if got := UseMirageVision(cfg); got != tc.want {
				t.Fatalf("UseMirageVision = %v, want %v", got, tc.want)
			}
		})
	}
	if UseMirageVision(nil) {
		t.Fatal("nil config must not use Mirage")
	}
}

func TestAnalyzeWithMirageSendsOnlyTheImageAndNeverTheMainKey(t *testing.T) {
	var gotAuth, gotModel, gotURL string
	server := testutil.NewHTTPServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		var req struct {
			Model    string `json:"model"`
			Messages []struct {
				Content []struct {
					ImageURL struct {
						URL string `json:"url"`
					} `json:"image_url"`
				} `json:"content"`
			} `json:"messages"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		gotModel = req.Model
		gotURL = req.Messages[0].Content[0].ImageURL.URL
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"choices":[{"message":{"content":"Scene: a cat."}}]}`)
	}))
	defer server.Close()
	overrideMirageHooks(t, server.URL+"/v1", false)

	cfg := mirageTestConfig()
	cfg.LLM.APIKey = "main-secret"
	text, in, out, err := analyzeImageReferenceWithPromptContext(context.Background(), "data:image/png;base64,AAAA", "what is this", cfg)
	if err != nil {
		t.Fatalf("analyze: %v", err)
	}
	if text != "Scene: a cat." || in != 0 || out != 0 {
		t.Fatalf("got (%q,%d,%d)", text, in, out)
	}
	if strings.Contains(gotAuth, "main-secret") || gotAuth != "" {
		t.Fatalf("Authorization = %q, want none", gotAuth)
	}
	if gotModel != "mirage" || gotURL != "data:image/png;base64,AAAA" {
		t.Fatalf("model=%q url=%q", gotModel, gotURL)
	}
}

func TestAnalyzeWithMirageRejectsPublicURLs(t *testing.T) {
	overrideMirageHooks(t, "http://unused/v1", false)
	_, _, _, err := analyzeImageReferenceWithPromptContext(context.Background(), "https://example.com/a.png", "x", mirageTestConfig())
	if err == nil || !strings.Contains(err.Error(), "local images") {
		t.Fatalf("err = %v, want a local-images error", err)
	}
}

func TestAnalyzeWithMirageMapsStatusCodes(t *testing.T) {
	for status, want := range map[int]string{http.StatusServiceUnavailable: "loading", http.StatusInternalServerError: "status 500"} {
		status, want := status, want
		server := testutil.NewHTTPServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(status)
			_, _ = io.WriteString(w, `{"error":{"message":"secret detail"}}`)
		}))
		overrideMirageHooks(t, server.URL+"/v1", false)
		_, _, _, err := analyzeImageReferenceWithPromptContext(context.Background(), "data:image/png;base64,AAAA", "x", mirageTestConfig())
		server.Close()
		if err == nil || !strings.Contains(err.Error(), want) || strings.Contains(err.Error(), "secret detail") {
			t.Fatalf("status %d: err = %v, want %q without the body", status, err, want)
		}
	}
}

func TestAnalyzeWithMirageUnreachableSidecarIsAClearError(t *testing.T) {
	overrideMirageHooks(t, "http://127.0.0.1:1/v1", false)
	_, _, _, err := analyzeImageReferenceWithPromptContext(context.Background(), "data:image/png;base64,AAAA", "x", mirageTestConfig())
	if err == nil || !strings.Contains(err.Error(), "not reachable") {
		t.Fatalf("err = %v", err)
	}
}

func TestTrustedLocalFileReachesMirageAsDataURL(t *testing.T) {
	var gotURL string
	server := testutil.NewHTTPServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Messages []struct {
				Content []struct {
					ImageURL struct {
						URL string `json:"url"`
					} `json:"image_url"`
				} `json:"content"`
			} `json:"messages"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		gotURL = req.Messages[0].Content[0].ImageURL.URL
		_, _ = io.WriteString(w, `{"choices":[{"message":{"content":"ok"}}]}`)
	}))
	defer server.Close()
	overrideMirageHooks(t, server.URL+"/v1", false)

	img := image.NewRGBA(image.Rect(0, 0, 2, 2))
	img.Set(0, 0, color.RGBA{R: 255, A: 255})
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "photo.png")
	if err := os.WriteFile(path, buf.Bytes(), 0o600); err != nil {
		t.Fatal(err)
	}
	text, _, _, err := AnalyzeTrustedImageFileWithPromptContext(context.Background(), path, "describe", mirageTestConfig())
	if err != nil || text != "ok" {
		t.Fatalf("got (%q, %v)", text, err)
	}
	if !strings.HasPrefix(gotURL, "data:image/") {
		t.Fatalf("sidecar received %.30q, want a data URL", gotURL)
	}
}

func TestMirageShouldHandleTable(t *testing.T) {
	overrideMirageHooks(t, "http://unused/v1", false)
	const data, remote = "data:image/png;base64,AAAA", "https://example.com/a.png"
	cases := []struct {
		name string
		mode string
		prov string
		ref  string
		want bool
	}{
		{"data url fallback", config.MirageModeFallback, "", data, true},
		{"data url always", config.MirageModeAlways, "", data, true},
		{"https url fallback", config.MirageModeFallback, "", remote, true},
		{"https url always falls through", config.MirageModeAlways, "", remote, false},
		{"explicit provider fallback", config.MirageModeFallback, "p", data, false},
		{"explicit provider always", config.MirageModeAlways, "p", data, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := mirageTestConfig()
			cfg.Vision.Mirage.Mode = tc.mode
			cfg.Vision.Provider = tc.prov
			if got := mirageShouldHandle(cfg, tc.ref); got != tc.want {
				t.Fatalf("mirageShouldHandle = %v, want %v", got, tc.want)
			}
		})
	}
}
