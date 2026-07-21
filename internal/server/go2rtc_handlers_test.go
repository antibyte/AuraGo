package server

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"

	"aurago/internal/config"
	"aurago/internal/security"
	"aurago/internal/tools"
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
