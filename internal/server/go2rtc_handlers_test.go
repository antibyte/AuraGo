package server

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"aurago/internal/config"
	"aurago/internal/security"
	"aurago/internal/tools"

	"gopkg.in/yaml.v3"
)

func TestGo2RTCProxyAllowlistBlocksMutationAndSensitiveAPIs(t *testing.T) {
	for _, test := range []struct {
		path    string
		method  string
		adminUI bool
		allowed bool
	}{
		{"video-rtc.js", http.MethodGet, false, true},
		{"api/frame.jpeg", http.MethodGet, false, true},
		{"api/webrtc", http.MethodPost, false, true},
		{"api/hls/playlist.m3u8", http.MethodGet, false, true},
		{"api/hls/segment.m4s", http.MethodGet, false, true},
		{"api/hls/arbitrary", http.MethodGet, false, false},
		{"api/streams", http.MethodGet, true, true},
		{"api/streams", http.MethodPatch, true, false},
		{"api/config", http.MethodGet, true, false},
		{"api/log", http.MethodGet, true, false},
		{"api/onvif", http.MethodGet, true, false},
		{"add.html", http.MethodGet, true, false},
		{"config.html", http.MethodGet, true, false},
		{"log.html", http.MethodGet, true, false},
		{"api/exit", http.MethodPost, true, false},
		{"api/streams", http.MethodPatch, false, false},
		{"../api/config", http.MethodGet, true, false},
	} {
		if got := go2RTCProxyPathAllowed(test.path, test.method, test.adminUI); got != test.allowed {
			t.Fatalf("go2RTCProxyPathAllowed(%q, %q, %t) = %t, want %t", test.path, test.method, test.adminUI, got, test.allowed)
		}
	}
}

func TestGo2RTCProxyMediaPathsRequireConfiguredStreamID(t *testing.T) {
	for _, path := range []string{
		"api/frame.jpeg",
		"api/ws",
		"api/stream.m3u8",
		"api/stream.mp4",
		"api/stream.mjpeg",
		"api/webrtc",
	} {
		if !go2RTCProxyRequiresStream(path) {
			t.Fatalf("%q must require a configured stream ID", path)
		}
	}
	for _, path := range []string{"video-rtc.js", "stream.html", "api/streams", "api/hls/playlist.m3u8", "api/hls/segment.m4s"} {
		if go2RTCProxyRequiresStream(path) {
			t.Fatalf("%q unexpectedly requires a stream ID", path)
		}
	}
}

func TestGo2RTCHLSSessionQueriesAreStrictlySanitized(t *testing.T) {
	query, err := sanitizeGo2RTCHLSQuery(url.Values{
		"id":      {"aB3dE9xZ"},
		"n":       {"12"},
		"ignored": {"value"},
	})
	if err != nil {
		t.Fatalf("sanitizeGo2RTCHLSQuery: %v", err)
	}
	if got := query.Encode(); got != "id=aB3dE9xZ&n=12" {
		t.Fatalf("sanitized HLS query = %q", got)
	}
	for _, invalid := range []url.Values{
		{},
		{"id": {"short"}},
		{"id": {"bad/id12"}},
		{"id": {"aB3dE9xZ"}, "src": {"front-door"}},
		{"id": {"aB3dE9xZ"}, "n": {"-1"}},
	} {
		if _, err := sanitizeGo2RTCHLSQuery(invalid); err == nil {
			t.Fatalf("invalid HLS query unexpectedly accepted: %#v", invalid)
		}
	}
}

func TestValidateManagedDockerBackendsRejectsGo2RTCWithoutDocker(t *testing.T) {
	var cfg config.Config
	cfg.Go2RTC.Enabled = true
	if err := validateManagedDockerBackends(cfg, config.Runtime{}); err == nil {
		t.Fatal("expected managed go2rtc to require Docker")
	}

	cfg.Docker.Enabled = true
	cfg.Docker.ReadOnly = true
	if err := validateManagedDockerBackends(cfg, config.Runtime{}); err == nil {
		t.Fatal("expected managed go2rtc to reject Docker read-only mode")
	}
	cfg.Docker.ReadOnly = false
	if err := validateManagedDockerBackends(cfg, config.Runtime{IsDocker: true, DockerSocketOK: false}); err == nil {
		t.Fatal("expected managed go2rtc to require a reachable Docker endpoint inside Docker")
	}

	cfg.Docker.Host = "tcp://docker-proxy:2375"
	if err := validateManagedDockerBackends(cfg, config.Runtime{IsDocker: true, DockerSocketOK: false}); err != nil {
		t.Fatalf("remote Docker endpoint should satisfy go2rtc requirement: %v", err)
	}
}

func TestGo2RTCRecreateDecisionKeepsRuntimePatchForAddsAndChanges(t *testing.T) {
	base := config.Go2RTCConfig{
		Image:         config.Go2RTCDefaultImage,
		ContainerName: "aurago_go2rtc",
		URL:           "http://127.0.0.1:1984",
		APIHostPort:   1984,
		Streams: []config.Go2RTCStreamConfig{{
			ID: "front-door", Name: "Front door", Enabled: true, Source: "rtsp://camera.local/old",
		}},
	}
	changed := base
	changed.Streams = append([]config.Go2RTCStreamConfig(nil), base.Streams...)
	changed.Streams[0].Source = "rtsp://camera.local/new"
	if go2RTCRequiresRecreate(base, changed) {
		t.Fatal("source changes should use runtime PATCH without recreating the container")
	}

	added := changed
	added.Streams = append(append([]config.Go2RTCStreamConfig(nil), changed.Streams...), config.Go2RTCStreamConfig{
		ID: "garage", Name: "Garage", Enabled: true, Source: "rtsp://garage.local/live",
	})
	if go2RTCRequiresRecreate(changed, added) {
		t.Fatal("stream additions should use runtime PATCH without recreating the container")
	}

	disabled := base
	disabled.Streams = append([]config.Go2RTCStreamConfig(nil), base.Streams...)
	disabled.Streams[0].Enabled = false
	if !go2RTCRequiresRecreate(base, disabled) {
		t.Fatal("stream disable must recreate the container to evict the old runtime source")
	}

	removed := base
	removed.Streams = nil
	if !go2RTCRequiresRecreate(base, removed) {
		t.Fatal("stream removal must recreate the container to evict the old runtime source")
	}
}

func TestGo2RTCRuntimeTransitionRecreatesOnContainerIdentityOrDockerHostChange(t *testing.T) {
	oldCfg := config.Config{}
	oldCfg.Docker.Host = "tcp://old-docker:2375"
	oldCfg.Go2RTC = config.Go2RTCConfig{Enabled: true, ContainerName: "aurago_go2rtc"}

	newCfg := oldCfg
	newCfg.Go2RTC.ContainerName = "aurago_go2rtc_new"
	changed, recreate := go2RTCRuntimeTransition(oldCfg, newCfg)
	if !changed || !recreate {
		t.Fatal("container identity change must recreate the sidecar")
	}

	newCfg = oldCfg
	newCfg.Docker.Host = "tcp://new-docker:2375"
	changed, recreate = go2RTCRuntimeTransition(oldCfg, newCfg)
	if !changed || !recreate {
		t.Fatal("Docker target change must remove the old sidecar before switching targets")
	}
}

func TestValidateGo2RTCSettingsPinsInternalEndpointAndWebRTCBoundary(t *testing.T) {
	base := config.Go2RTCConfig{Enabled: true, URL: "http://127.0.0.1:1984", APIHostPort: 1984}
	if err := validateGo2RTCSettings(base, config.Runtime{}, nil); err != nil {
		t.Fatalf("native loopback URL rejected: %v", err)
	}
	for _, invalidURL := range []string{
		"http://camera.example:1984",
		"http://127.0.0.1:1985",
		"https://127.0.0.1:1984",
		"http://user:password@127.0.0.1:1984",
		"http://127.0.0.1:1984/api",
	} {
		cfg := base
		cfg.URL = invalidURL
		if err := validateGo2RTCSettings(cfg, config.Runtime{}, nil); err == nil {
			t.Fatalf("validateGo2RTCSettings unexpectedly accepted %q", invalidURL)
		}
	}

	dockerCfg := base
	dockerCfg.URL = "http://go2rtc:1984"
	if err := validateGo2RTCSettings(dockerCfg, config.Runtime{IsDocker: true}, nil); err != nil {
		t.Fatalf("managed Docker alias rejected: %v", err)
	}
	dockerCfg.URL = "http://other-container:1984"
	if err := validateGo2RTCSettings(dockerCfg, config.Runtime{IsDocker: true}, nil); err == nil {
		t.Fatal("arbitrary Docker upstream unexpectedly accepted")
	}

	webrtcCfg := base
	webrtcCfg.WebRTC = config.Go2RTCWebRTCConfig{Enabled: true, BindAddress: "0.0.0.0", Port: 8555}
	if err := validateGo2RTCSettings(webrtcCfg, config.Runtime{}, nil); err == nil {
		t.Fatal("unspecified WebRTC bind unexpectedly accepted")
	}
	webrtcCfg.WebRTC.BindAddress = "192.168.1.20"
	if err := validateGo2RTCSettings(webrtcCfg, config.Runtime{}, nil); err != nil {
		t.Fatalf("private WebRTC bind rejected: %v", err)
	}
}

func TestGo2RTCProxyEnforcesOriginStreamScopeAndUpstreamAuth(t *testing.T) {
	const internalPassword = "internal-password"
	var upstreamCalls int
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upstreamCalls++
		user, password, ok := r.BasicAuth()
		if !ok || user != "aurago" || password != internalPassword {
			t.Errorf("upstream Basic Auth = %q/%q/%t", user, password, ok)
		}
		if cookie := r.Header.Get("Cookie"); cookie != "" {
			t.Errorf("upstream received AuraGo cookie %q", cookie)
		}
		if got := r.URL.Query().Get("src"); got != "aurago_front-door" {
			t.Errorf("upstream src = %q, want internal alias", got)
		}
		w.Header().Set("Content-Type", "image/jpeg")
		_, _ = w.Write([]byte{0xff, 0xd8, 0xff, 0xd9})
	}))
	defer upstream.Close()
	parsed, _ := url.Parse(upstream.URL)
	port, _ := strconv.Atoi(parsed.Port())
	cfg := &config.Config{}
	cfg.Directories.DataDir = t.TempDir()
	cfg.Go2RTC = config.Go2RTCConfig{
		Enabled:       true,
		URL:           upstream.URL,
		APIHostPort:   port,
		APIPassword:   internalPassword,
		ContainerName: "aurago_go2rtc",
		Streams: []config.Go2RTCStreamConfig{{
			ID: "front-door", Name: "Front door", Enabled: true, Source: "rtsp://camera.local/live",
		}},
	}
	server := &Server{Cfg: cfg, Go2RTC: tools.NewGo2RTCManager(cfg, nil, nil, nil)}
	handler := handleGo2RTCProxy(server)

	viewerRequest := httptest.NewRequest(http.MethodGet, "http://aurago.local/api/go2rtc/viewer/front-door", nil)
	viewerRecorder := httptest.NewRecorder()
	handleGo2RTCViewer(server).ServeHTTP(viewerRecorder, viewerRequest)
	if viewerRecorder.Code != http.StatusOK ||
		!strings.Contains(viewerRecorder.Body.String(), `document.createElement("video-stream")`) ||
		!strings.Contains(viewerRecorder.Body.String(), `/api/go2rtc/proxy/api/ws?src=`) ||
		strings.Contains(viewerRecorder.Body.String(), "camera.local") {
		t.Fatalf("unsafe or incomplete viewer response: status=%d body=%s", viewerRecorder.Code, viewerRecorder.Body.String())
	}
	if csp := viewerRecorder.Header().Get("Content-Security-Policy"); !strings.Contains(csp, "nonce-") ||
		!strings.Contains(csp, "frame-ancestors 'self'") {
		t.Fatalf("viewer CSP missing nonce: %q", csp)
	}

	blocked := httptest.NewRequest(http.MethodGet, "http://aurago.local/api/go2rtc/proxy/api/frame.jpeg?src=front-door", nil)
	blocked.Header.Set("Origin", "https://evil.example")
	blockedRecorder := httptest.NewRecorder()
	handler.ServeHTTP(blockedRecorder, blocked)
	if blockedRecorder.Code != http.StatusForbidden || upstreamCalls != 0 {
		t.Fatalf("cross-origin proxy status=%d upstream_calls=%d", blockedRecorder.Code, upstreamCalls)
	}

	allowed := httptest.NewRequest(http.MethodGet, "http://aurago.local/api/go2rtc/proxy/api/frame.jpeg?src=front-door", nil)
	allowed.Host = "aurago.local"
	allowed.Header.Set("Origin", "http://aurago.local")
	allowed.Header.Set("Authorization", "Bearer viewer-token")
	allowed.Header.Set("Cookie", "aurago_session=private")
	allowedRecorder := httptest.NewRecorder()
	handler.ServeHTTP(allowedRecorder, allowed)
	if allowedRecorder.Code != http.StatusOK || upstreamCalls != 1 {
		t.Fatalf("same-origin proxy status=%d upstream_calls=%d body=%s", allowedRecorder.Code, upstreamCalls, allowedRecorder.Body.String())
	}
}

func TestGo2RTCAdminStreamsProxyMasksRuntimeSources(t *testing.T) {
	const internalPassword = "internal-password"
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/go2rtc/proxy/api/streams" {
			http.NotFound(w, r)
			return
		}
		if user, password, ok := r.BasicAuth(); !ok || user != "aurago" || password != internalPassword {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"aurago_front-door":{"producers":[{"url":"rtsp://camera-user:camera-password@camera.local/live"}],"consumers":[]}}`))
	}))
	defer upstream.Close()

	parsed, _ := url.Parse(upstream.URL)
	port, _ := strconv.Atoi(parsed.Port())
	cfg := &config.Config{}
	cfg.Go2RTC = config.Go2RTCConfig{
		Enabled:       true,
		WebUIEnabled:  true,
		URL:           upstream.URL,
		APIHostPort:   port,
		APIPassword:   internalPassword,
		ContainerName: "aurago_go2rtc",
	}
	server := &Server{Cfg: cfg, Go2RTC: tools.NewGo2RTCManager(cfg, nil, nil, nil)}
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "http://aurago.local/api/go2rtc/proxy/api/streams", nil)
	request.Host = "aurago.local"
	handleGo2RTCProxy(server).ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", recorder.Code, recorder.Body.String())
	}
	body := recorder.Body.String()
	for _, secret := range []string{"camera.local", "camera-user", "camera-password", "rtsp://"} {
		if strings.Contains(body, secret) {
			t.Fatalf("admin streams proxy leaked %q: %s", secret, body)
		}
	}
	if !strings.Contains(body, "••••••••") {
		t.Fatalf("admin streams proxy did not preserve a masked source marker: %s", body)
	}
}

func TestGo2RTCConfigProjectionAndVaultExtractionNeverExposeSource(t *testing.T) {
	vault, err := security.NewVault(strings.Repeat("11", 32), t.TempDir()+"/vault.bin")
	if err != nil {
		t.Fatalf("NewVault: %v", err)
	}
	const source = "rtsp://camera-user:camera-password@camera.local/live"
	items := []interface{}{map[string]interface{}{
		"id": "front-door", "name": "Front door", "enabled": true, "source": source,
	}}
	if err := extractGo2RTCStreamSourcesToVault(items, vault, slog.Default()); err != nil {
		t.Fatalf("extractGo2RTCStreamSourcesToVault: %v", err)
	}
	item := items[0].(map[string]interface{})
	if _, exists := item["source"]; exists {
		t.Fatalf("source remained in config patch: %#v", item)
	}
	if got, err := vault.ReadSecret(config.Go2RTCStreamSourceVaultKey("front-door")); err != nil || got != source {
		t.Fatalf("vault source = %q, %v", got, err)
	}

	cfg := &config.Config{}
	cfg.Go2RTC.Streams = []config.Go2RTCStreamConfig{{
		ID: "front-door", Name: "Front door", Enabled: true, Source: source,
	}}
	raw := map[string]interface{}{
		"go2rtc": map[string]interface{}{
			"streams": []interface{}{map[string]interface{}{"id": "front-door", "source": source}},
		},
	}
	injectGo2RTCConfig(raw, cfg, vault)
	projected, _ := json.Marshal(raw)
	if strings.Contains(string(projected), source) || strings.Contains(string(projected), "camera-password") {
		t.Fatalf("config projection leaked source: %s", projected)
	}
	if !strings.Contains(string(projected), `"source_configured":true`) || !strings.Contains(string(projected), `"source":"••••••••"`) {
		t.Fatalf("config projection omitted masked source state: %s", projected)
	}
}

func TestGo2RTCVaultMaskPreservationAndRemovedStreamCleanup(t *testing.T) {
	vault, err := security.NewVault(strings.Repeat("22", 32), t.TempDir()+"/vault.bin")
	if err != nil {
		t.Fatalf("NewVault: %v", err)
	}
	frontKey := config.Go2RTCStreamSourceVaultKey("front-door")
	garageKey := config.Go2RTCStreamSourceVaultKey("garage")
	if err := vault.WriteSecret(frontKey, "rtsp://camera.local/original"); err != nil {
		t.Fatalf("seed front-door source: %v", err)
	}

	items := []interface{}{
		map[string]interface{}{"id": "front-door", "source": "••••••••"},
		map[string]interface{}{"id": "garage", "source": "rtsp://garage.local/live"},
	}
	if err := extractGo2RTCStreamSourcesToVault(items, vault, slog.Default()); err != nil {
		t.Fatalf("extractGo2RTCStreamSourcesToVault: %v", err)
	}
	if got, err := vault.ReadSecret(frontKey); err != nil || got != "rtsp://camera.local/original" {
		t.Fatalf("masked source was not preserved: %q, %v", got, err)
	}
	if got, err := vault.ReadSecret(garageKey); err != nil || got != "rtsp://garage.local/live" {
		t.Fatalf("new source was not saved: %q, %v", got, err)
	}

	cleanupRemovedGo2RTCStreamSecrets(
		[]config.Go2RTCStreamConfig{{ID: "front-door"}, {ID: "garage"}},
		[]config.Go2RTCStreamConfig{{ID: "garage"}},
		vault,
		slog.Default(),
	)
	if _, err := vault.ReadSecret(frontKey); err == nil {
		t.Fatal("removed stream source remained in the vault")
	}
	if got, err := vault.ReadSecret(garageKey); err != nil || got != "rtsp://garage.local/live" {
		t.Fatalf("remaining stream source was disturbed: %q, %v", got, err)
	}
}

func TestGo2RTCAppStateNeverExposesSourcesOrDisabledStreamsToViewer(t *testing.T) {
	const source = "rtsp://camera-user:camera-password@192.168.1.20/live"
	cfg := &config.Config{}
	cfg.Go2RTC = config.Go2RTCConfig{
		Streams: []config.Go2RTCStreamConfig{
			{ID: "front-door", Name: "Front door", Enabled: true, Source: source, SourceConfigured: true},
			{ID: "garage", Name: "Garage", Source: source, SourceConfigured: true},
		},
	}
	server := &Server{Cfg: cfg, Go2RTC: tools.NewGo2RTCManager(cfg, nil, nil, nil)}

	viewerRequest := httptest.NewRequest(http.MethodGet, "/api/go2rtc/app/state", nil)
	viewerRequest.Header.Set("Authorization", "Bearer viewer-scope-only")
	viewerRecorder := httptest.NewRecorder()
	handleGo2RTCAppState(server).ServeHTTP(viewerRecorder, viewerRequest)
	if viewerRecorder.Code != http.StatusOK {
		t.Fatalf("viewer state status = %d; body=%s", viewerRecorder.Code, viewerRecorder.Body.String())
	}
	viewerBody := viewerRecorder.Body.String()
	for _, forbidden := range []string{"camera-user", "camera-password", "192.168.1.20", "rtsp://", "disabled_streams", "garage"} {
		if strings.Contains(viewerBody, forbidden) {
			t.Fatalf("viewer app state exposed %q: %s", forbidden, viewerBody)
		}
	}

	adminRequest := httptest.NewRequest(http.MethodGet, "/api/go2rtc/app/state", nil)
	adminRecorder := httptest.NewRecorder()
	handleGo2RTCAppState(server).ServeHTTP(adminRecorder, adminRequest)
	adminBody := adminRecorder.Body.String()
	if adminRecorder.Code != http.StatusOK || !strings.Contains(adminBody, `"disabled_streams"`) || !strings.Contains(adminBody, `"source_configured":true`) {
		t.Fatalf("admin state omitted safe disabled stream metadata: status=%d body=%s", adminRecorder.Code, adminBody)
	}
	for _, forbidden := range []string{"camera-user", "camera-password", "192.168.1.20", "rtsp://"} {
		if strings.Contains(adminBody, forbidden) {
			t.Fatalf("admin app state exposed %q: %s", forbidden, adminBody)
		}
	}
}

func TestGo2RTCThumbnailUsesPrivateCacheAndETag(t *testing.T) {
	jpeg := []byte{0xff, 0xd8, 0xff, 0xdb, 0x01, 0xff, 0xd9}
	upstreamCalls := 0
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upstreamCalls++
		if r.URL.Path != "/api/go2rtc/proxy/api/frame.jpeg" || r.URL.Query().Get("src") != "aurago_front-door" {
			t.Errorf("unexpected thumbnail upstream request: %s?%s", r.URL.Path, r.URL.RawQuery)
		}
		w.Header().Set("Content-Type", "image/jpeg")
		_, _ = w.Write(jpeg)
	}))
	defer upstream.Close()
	parsed, _ := url.Parse(upstream.URL)
	port, _ := strconv.Atoi(parsed.Port())
	cfg := &config.Config{}
	cfg.Directories.DataDir = t.TempDir()
	cfg.Go2RTC = config.Go2RTCConfig{
		Enabled: true, URL: upstream.URL, APIHostPort: port, APIPassword: "internal-password", StoreMedia: true,
		Streams: []config.Go2RTCStreamConfig{{ID: "front-door", Name: "Front door", Enabled: true, Source: "rtsp://192.168.1.20/live"}},
	}
	server := &Server{Cfg: cfg, Go2RTC: tools.NewGo2RTCManager(cfg, nil, nil, nil)}
	handler := handleGo2RTCThumbnail(server)

	request := httptest.NewRequest(http.MethodGet, "/api/go2rtc/thumbnail/front-door.jpg?width=640&height=360&cache=5", nil)
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK || recorder.Body.String() != string(jpeg) {
		t.Fatalf("thumbnail response = %d %x", recorder.Code, recorder.Body.Bytes())
	}
	etag := recorder.Header().Get("ETag")
	if etag == "" || recorder.Header().Get("Cache-Control") != "private, max-age=5" || recorder.Header().Get("Content-Type") != "image/jpeg" {
		t.Fatalf("thumbnail cache headers = %#v", recorder.Header())
	}

	cachedRequest := httptest.NewRequest(http.MethodGet, "/api/go2rtc/thumbnail/front-door.jpg?width=640&height=360&cache=5", nil)
	cachedRequest.Header.Set("If-None-Match", etag)
	cachedRecorder := httptest.NewRecorder()
	handler.ServeHTTP(cachedRecorder, cachedRequest)
	if cachedRecorder.Code != http.StatusNotModified || upstreamCalls != 1 {
		t.Fatalf("cached thumbnail = status %d, upstream calls %d", cachedRecorder.Code, upstreamCalls)
	}

	invalidRecorder := httptest.NewRecorder()
	handler.ServeHTTP(invalidRecorder, httptest.NewRequest(http.MethodGet, "/api/go2rtc/thumbnail/front-door.jpg?width=1281", nil))
	if invalidRecorder.Code != http.StatusBadRequest || upstreamCalls != 1 {
		t.Fatalf("oversized thumbnail = status %d, upstream calls %d", invalidRecorder.Code, upstreamCalls)
	}
}

func TestGo2RTCAppMutationsRejectCrossOriginAndTrailingJSON(t *testing.T) {
	crossOrigin := httptest.NewRequest(http.MethodPatch, "http://aurago.local/api/go2rtc/streams/front-door", strings.NewReader(`{"enabled":false}`))
	crossOrigin.Header.Set("Origin", "https://evil.example")
	crossOriginRecorder := httptest.NewRecorder()
	handleGo2RTCStreamMutation(&Server{}).ServeHTTP(crossOriginRecorder, crossOrigin)
	if crossOriginRecorder.Code != http.StatusForbidden {
		t.Fatalf("cross-origin mutation status = %d, want 403", crossOriginRecorder.Code)
	}

	type payload struct {
		Enabled bool `json:"enabled"`
	}
	request := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"enabled":true} trailing`))
	if err := decodeGo2RTCAppJSON(httptest.NewRecorder(), request, &payload{}); err == nil {
		t.Fatal("trailing non-JSON data was accepted")
	}
	request = httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"enabled":true}{"enabled":false}`))
	if err := decodeGo2RTCAppJSON(httptest.NewRecorder(), request, &payload{}); err == nil {
		t.Fatal("second JSON object was accepted")
	}
}

func TestGo2RTCViewScopeDoesNotAcceptGeneralDesktopBearer(t *testing.T) {
	vault, err := security.NewVault(strings.Repeat("33", 32), t.TempDir()+"/vault.bin")
	if err != nil {
		t.Fatalf("NewVault: %v", err)
	}
	tokens, err := security.NewTokenManager(vault, t.TempDir()+"/tokens.enc")
	if err != nil {
		t.Fatalf("NewTokenManager: %v", err)
	}
	desktopToken, _, err := tokens.Create("desktop", []string{"desktop.control"}, nil)
	if err != nil {
		t.Fatalf("create desktop token: %v", err)
	}
	viewerToken, _, err := tokens.Create("camera viewer", []string{go2RTCViewScope}, nil)
	if err != nil {
		t.Fatalf("create viewer token: %v", err)
	}
	adminToken, _, err := tokens.Create("admin", []string{"admin"}, nil)
	if err != nil {
		t.Fatalf("create admin token: %v", err)
	}
	server := &Server{Cfg: &config.Config{}, TokenManager: tokens}
	calls := 0
	handler := requireGo2RTCView(server, func(w http.ResponseWriter, _ *http.Request) {
		calls++
		w.WriteHeader(http.StatusNoContent)
	})

	for _, test := range []struct {
		name   string
		token  string
		status int
	}{
		{name: "general desktop", token: desktopToken, status: http.StatusForbidden},
		{name: "camera viewer", token: viewerToken, status: http.StatusNoContent},
		{name: "administrator", token: adminToken, status: http.StatusNoContent},
	} {
		t.Run(test.name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodGet, "/api/go2rtc/app/state", nil)
			request.Header.Set("Authorization", "Bearer "+test.token)
			recorder := httptest.NewRecorder()
			handler.ServeHTTP(recorder, request)
			if recorder.Code != test.status {
				t.Fatalf("status = %d, want %d", recorder.Code, test.status)
			}
		})
	}
	if calls != 2 {
		t.Fatalf("authorized handler calls = %d, want 2", calls)
	}
}

func TestGo2RTCSetupEnableReturnsStructuredDockerRequirements(t *testing.T) {
	server := &Server{Cfg: &config.Config{}}
	request := httptest.NewRequest(http.MethodPost, "http://aurago.local/api/go2rtc/setup/enable", strings.NewReader(`{}`))
	request.Host = "aurago.local"
	recorder := httptest.NewRecorder()
	handleGo2RTCSetupEnable(server).ServeHTTP(recorder, request)
	if recorder.Code != http.StatusPreconditionFailed {
		t.Fatalf("setup status = %d, want 412; body=%s", recorder.Code, recorder.Body.String())
	}
	var response struct {
		Status       string              `json:"status"`
		Requirements []map[string]string `json:"requirements"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode setup requirements: %v", err)
	}
	if response.Status != "requirements_missing" || len(response.Requirements) < 2 {
		t.Fatalf("setup requirements = %#v", response)
	}
}

func TestManagedGo2RTCConfigUpdateKeepsSourcesVaultOnlyAndRollsBackValidationFailure(t *testing.T) {
	const firstSource = "rtsp://camera-user:first-password@192.168.40.20/live"
	const secondSource = "rtsp://camera-user:second-password@192.168.40.20/live"
	var patchedSources []string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/go2rtc/proxy/api":
			_, _ = w.Write([]byte(`{"version":"1.9.14"}`))
		case r.Method == http.MethodPatch && r.URL.Path == "/api/go2rtc/proxy/api/streams":
			patchedSources = append(patchedSources, r.URL.Query().Get("src"))
			w.WriteHeader(http.StatusOK)
		default:
			http.NotFound(w, r)
		}
	}))
	defer upstream.Close()
	parsed, _ := url.Parse(upstream.URL)
	port, _ := strconv.Atoi(parsed.Port())

	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")
	cfg := &config.Config{ConfigPath: configPath}
	cfg.Directories.DataDir = filepath.Join(tmpDir, "data")
	cfg.Docker.Enabled = true
	cfg.Docker.Host = "tcp://127.0.0.1:2375"
	cfg.Go2RTC = config.Go2RTCConfig{
		Enabled: true, AutoStart: false, Image: config.Go2RTCDefaultImage,
		URL: upstream.URL, APIHostPort: port, ContainerName: "aurago_go2rtc",
		WebRTC: config.Go2RTCWebRTCConfig{Port: 8555},
	}
	rawConfig, err := yaml.Marshal(cfg)
	if err != nil {
		t.Fatalf("marshal initial config: %v", err)
	}
	if err := os.WriteFile(configPath, rawConfig, 0o600); err != nil {
		t.Fatalf("write initial config: %v", err)
	}
	vault, err := security.NewVault(strings.Repeat("44", 32), filepath.Join(tmpDir, "vault.bin"))
	if err != nil {
		t.Fatalf("NewVault: %v", err)
	}
	if err := vault.WriteSecret(config.Go2RTCAPIPasswordVaultKey, "internal-password"); err != nil {
		t.Fatalf("seed API password: %v", err)
	}
	manager := tools.NewGo2RTCManager(cfg, vault, nil, slog.Default())
	server := &Server{Cfg: cfg, Vault: vault, Go2RTC: manager, Logger: slog.Default()}

	addStream := func(source string) error {
		_, updateErr := updateManagedGo2RTCConfig(context.Background(), server, func(value *config.Go2RTCConfig) error {
			if len(value.Streams) == 0 {
				value.Streams = append(value.Streams, config.Go2RTCStreamConfig{ID: "front-door", Name: "Front door", Enabled: true})
			}
			return nil
		}, []go2RTCSourceChange{{ID: "front-door", Value: source}}, false)
		return updateErr
	}
	if err := addStream(firstSource); err != nil {
		t.Fatalf("add stream: %v", err)
	}
	if err := addStream(secondSource); err != nil {
		t.Fatalf("replace stream source: %v", err)
	}
	if len(patchedSources) != 2 || patchedSources[0] != firstSource || patchedSources[1] != secondSource {
		t.Fatalf("runtime reconcile sources = %#v", patchedSources)
	}
	if got, err := vault.ReadSecret(config.Go2RTCStreamSourceVaultKey("front-door")); err != nil || got != secondSource {
		t.Fatalf("vault source = %q, %v", got, err)
	}
	publishedYAML, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("read published config: %v", err)
	}
	for _, forbidden := range []string{firstSource, secondSource, "first-password", "second-password"} {
		if strings.Contains(string(publishedYAML), forbidden) {
			t.Fatalf("published YAML leaked %q", forbidden)
		}
	}

	if err := addStream("file:///private/camera.mp4"); err == nil {
		t.Fatal("invalid replacement source unexpectedly succeeded")
	}
	if got, err := vault.ReadSecret(config.Go2RTCStreamSourceVaultKey("front-door")); err != nil || got != secondSource {
		t.Fatalf("failed validation did not restore source: %q, %v", got, err)
	}
}
