package tools

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"aurago/internal/config"
	"aurago/internal/httporigin"
	"aurago/internal/testutil"
)

// Live fixtures for the same-origin redirect policy of the credentialed vision,
// TTS and image-generation clients. Server A is the configured
// provider and answers with a redirect to server B, a second origin on another
// loopback port. B records whether it ever saw a request, a credential header
// or the request body.

const credentialedRedirectSecret = "SECRETPAYLOADMARKER"

type credentialedRedirectSink struct {
	server  *httptest.Server
	hits    atomic.Int32
	sawAuth atomic.Bool
	sawBody atomic.Bool
}

func newCredentialedRedirectSink(t *testing.T) *credentialedRedirectSink {
	t.Helper()
	sink := &credentialedRedirectSink{}
	sink.server = testutil.NewHTTPServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sink.hits.Add(1)
		if r.Header.Get("Authorization") != "" || r.Header.Get("xi-api-key") != "" {
			sink.sawAuth.Store(true)
		}
		body, _ := io.ReadAll(r.Body)
		if strings.Contains(string(body), credentialedRedirectSecret) {
			sink.sawBody.Store(true)
		}
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(sink.server.Close)
	return sink
}

func (s *credentialedRedirectSink) assertUntouched(t *testing.T) {
	t.Helper()
	if hits := s.hits.Load(); hits != 0 {
		t.Fatalf("second origin received %d request(s); credentials=%v body=%v", hits, s.sawAuth.Load(), s.sawBody.Load())
	}
}

func newCrossOriginRedirectProvider(t *testing.T, target string) *httptest.Server {
	t.Helper()
	server := testutil.NewHTTPServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Location", target+r.URL.Path)
		w.WriteHeader(http.StatusTemporaryRedirect)
	}))
	t.Cleanup(server.Close)
	return server
}

func assertCrossOriginRedirectRejected(t *testing.T, err error) {
	t.Helper()
	if err == nil {
		t.Fatal("cross-origin redirect was followed without error")
	}
	if !errors.Is(err, httporigin.ErrCrossOriginRedirect) {
		t.Fatalf("error = %v, want httporigin.ErrCrossOriginRedirect", err)
	}
}

// routeVendorHostTo replaces *client for the test with a copy of the production
// client whose transport sends requests for vendorHost to server; every other
// host is dialed normally. Timeout and CheckRedirect stay those of production.
func routeVendorHostTo(t *testing.T, client **http.Client, vendorHost string, server *httptest.Server) {
	t.Helper()
	original := *client
	serverURL, err := url.Parse(server.URL)
	if err != nil {
		t.Fatal(err)
	}
	routed := *original
	routed.Transport = roundTripperFunc(func(req *http.Request) (*http.Response, error) {
		if req.URL.Hostname() != vendorHost {
			return http.DefaultTransport.RoundTrip(req)
		}
		clone := req.Clone(req.Context())
		clone.URL.Scheme = serverURL.Scheme
		clone.URL.Host = serverURL.Host
		clone.Host = serverURL.Host
		return http.DefaultTransport.RoundTrip(clone)
	})
	*client = &routed
	t.Cleanup(func() { *client = original })
}

func credentialedRedirectVisionConfig(baseURL string) *config.Config {
	cfg := &config.Config{}
	cfg.Vision.APIKey = "test-key"
	cfg.Vision.BaseURL = baseURL
	cfg.Vision.Model = "vision-model"
	return cfg
}

const credentialedRedirectImageReference = "data:image/png;base64," + credentialedRedirectSecret

func TestVisionClientDoesNotFollowCrossOriginRedirect(t *testing.T) {
	sink := newCredentialedRedirectSink(t)
	provider := newCrossOriginRedirectProvider(t, sink.server.URL)

	_, _, _, err := analyzeImageReferenceWithPromptContext(context.Background(), credentialedRedirectImageReference, "Describe it", credentialedRedirectVisionConfig(provider.URL+"/v1"))
	assertCrossOriginRedirectRejected(t, err)
	sink.assertUntouched(t)
}

func TestVisionClientFollowsSameOriginRedirectWithBodyAndAuth(t *testing.T) {
	var hits atomic.Int32
	server := testutil.NewHTTPServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		if r.URL.Path == "/v1/chat/completions" {
			w.Header().Set("Location", "/v2/chat/completions")
			w.WriteHeader(http.StatusTemporaryRedirect)
			return
		}
		body, _ := io.ReadAll(r.Body)
		if !strings.Contains(string(body), credentialedRedirectSecret) || r.Header.Get("Authorization") != "Bearer test-key" {
			t.Errorf("same-origin hop lost body or auth: auth=%q", r.Header.Get("Authorization"))
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"choices":[{"message":{"content":"ok"}}]}`)
	}))
	defer server.Close()

	got, _, _, err := analyzeImageReferenceWithPromptContext(context.Background(), credentialedRedirectImageReference, "Describe it", credentialedRedirectVisionConfig(server.URL+"/v1"))
	if err != nil || got != "ok" {
		t.Fatalf("same-origin redirect: got %q, err %v", got, err)
	}
	if n := hits.Load(); n != 2 {
		t.Fatalf("server hits = %d, want 2 (redirect + followed request)", n)
	}
}

func TestTTSClientDoesNotFollowCrossOriginRedirect(t *testing.T) {
	sink := newCredentialedRedirectSink(t)
	provider := newCrossOriginRedirectProvider(t, sink.server.URL)
	routeVendorHostTo(t, &ttsHTTPClient, "api.elevenlabs.io", provider)

	cfg := TTSConfig{}
	cfg.ElevenLabs.APIKey = "xi-test-key"
	_, err := ttsElevenLabs(cfg, credentialedRedirectSecret)
	assertCrossOriginRedirectRejected(t, err)
	sink.assertUntouched(t)
}

func TestImageGenClientsDoNotFollowCrossOriginRedirect(t *testing.T) {
	t.Setenv("AURAGO_SSRF_ALLOW_LOOPBACK", "1")
	sourceImage := filepath.Join(t.TempDir(), "source.png")
	if err := os.WriteFile(sourceImage, []byte{0x89, 0x50, 0x4e, 0x47}, 0o600); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name     string
		generate func(ImageGenConfig, string, ImageGenOptions) ([]byte, string, error)
		opts     ImageGenOptions
	}{
		{name: "openai-generations", generate: generateOpenAI},
		{name: "openai-edits", generate: generateOpenAI, opts: ImageGenOptions{SourceImage: sourceImage}},
		{name: "openrouter", generate: generateOpenRouter},
	} {
		t.Run(tc.name, func(t *testing.T) {
			sink := newCredentialedRedirectSink(t)
			provider := newCrossOriginRedirectProvider(t, sink.server.URL)

			_, _, err := tc.generate(ImageGenConfig{BaseURL: provider.URL + "/v1", APIKey: "sk-test", Model: "image-model"}, credentialedRedirectSecret, tc.opts)
			assertCrossOriginRedirectRejected(t, err)
			sink.assertUntouched(t)
		})
	}
}

func TestImageGenClientFollowsSameOriginRedirect(t *testing.T) {
	t.Setenv("AURAGO_SSRF_ALLOW_LOOPBACK", "1")
	pngData := []byte{0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a}
	var hits atomic.Int32
	server := testutil.NewHTTPServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		if r.URL.Path == "/v1/images/generations" {
			w.Header().Set("Location", "/v2/images/generations")
			w.WriteHeader(http.StatusTemporaryRedirect)
			return
		}
		body, _ := io.ReadAll(r.Body)
		if !strings.Contains(string(body), credentialedRedirectSecret) || r.Header.Get("Authorization") != "Bearer sk-test" {
			t.Errorf("same-origin hop lost body or auth: auth=%q", r.Header.Get("Authorization"))
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"data":[{"b64_json":"`+base64.StdEncoding.EncodeToString(pngData)+`"}]}`)
	}))
	defer server.Close()

	data, _, err := generateOpenAI(ImageGenConfig{BaseURL: server.URL + "/v1", APIKey: "sk-test", Model: "image-model"}, credentialedRedirectSecret, ImageGenOptions{})
	if err != nil || !bytes.Equal(data, pngData) {
		t.Fatalf("same-origin redirect: err %v, image %v", err, data)
	}
	if n := hits.Load(); n != 2 {
		t.Fatalf("server hits = %d, want 2 (redirect + followed request)", n)
	}
}

// Returned-image downloads carry no credentials and keep following redirects
// to another origin (for example a provider URL that redirects to its CDN);
// the SSRF re-check still applies to every hop.
func TestImageDownloadStillFollowsCrossOriginRedirect(t *testing.T) {
	t.Setenv("AURAGO_SSRF_ALLOW_LOOPBACK", "1")
	pngData := append([]byte{0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a}, bytes.Repeat([]byte{0x01}, 120)...)
	var cdnSawAuth atomic.Bool
	cdn := testutil.NewHTTPServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "" {
			cdnSawAuth.Store(true)
		}
		_, _ = w.Write(pngData)
	}))
	defer cdn.Close()
	provider := testutil.NewHTTPServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, cdn.URL+r.URL.Path, http.StatusFound)
	}))
	defer provider.Close()

	data, err := downloadImage(provider.URL + "/generated.png")
	if err != nil || !bytes.Equal(data, pngData) {
		t.Fatalf("download through a CDN redirect: err %v", err)
	}
	if cdnSawAuth.Load() {
		t.Fatal("image download sent an Authorization header")
	}
}
