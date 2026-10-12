package server

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"image/jpeg"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"aurago/internal/config"
	"aurago/internal/persondetect"
	"aurago/internal/persondetect/persondetecttest"
	"aurago/internal/security"
	"aurago/internal/tools"
)

type detectionAPIHarness struct {
	t            *testing.T
	s            *Server
	svc          *persondetect.Service
	factory      *persondetecttest.Factory
	frames       *persondetecttest.Frames
	mux          *http.ServeMux
	viewToken    string
	adminToken   string
	desktopToken string
}

func newDetectionAPIHarness(t *testing.T) *detectionAPIHarness {
	t.Helper()
	return newDetectionAPIHarnessAt(t, "", persondetect.DetectorReady)
}

// newDetectionAPIHarnessAt builds the harness with dataDir ("" = a fresh one) and
// waits until the detector reaches state.
func newDetectionAPIHarnessAt(t *testing.T, dataDir, state string) *detectionAPIHarness {
	t.Helper()
	dir := t.TempDir()
	if dataDir == "" {
		dataDir = filepath.Join(dir, "data")
	}
	vault, err := security.NewVault(strings.Repeat("66", 32), filepath.Join(dir, "vault.bin"))
	if err != nil {
		t.Fatalf("NewVault: %v", err)
	}
	tokens, err := security.NewTokenManager(vault, filepath.Join(dir, "tokens.enc"))
	if err != nil {
		t.Fatalf("NewTokenManager: %v", err)
	}
	viewToken, _, err := tokens.Create("camera viewer", []string{go2RTCViewScope}, nil)
	if err != nil {
		t.Fatal(err)
	}
	adminToken, _, err := tokens.Create("admin", []string{"admin"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	desktopToken, _, err := tokens.Create("desktop", []string{"desktop.control"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	cfg := &config.Config{}
	cfg.Auth.Enabled = true
	cfg.Auth.SessionSecret = "detection-session-secret"
	cfg.Directories.DataDir = dataDir
	detection := config.DefaultGo2RTCStreamDetection(true)
	detection.IntervalSeconds = 1
	cfg.Go2RTC = config.Go2RTCConfig{
		Enabled: true, URL: "http://127.0.0.1:1984", APIHostPort: 1984,
		Detection: config.NormalizeGo2RTCDetectionSettings(config.Go2RTCDetectionConfig{Enabled: true}),
		Streams:   []config.Go2RTCStreamConfig{{ID: "front-door", Name: "Front door", Enabled: true, Detection: detection}},
	}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	factory := persondetecttest.NewFactory(persondetecttest.CPUInfo())
	frames := persondetecttest.NewFrames()
	frames.Set("front-door", persondetecttest.JPEG(640, 360))
	svc := persondetect.NewService(persondetect.Deps{Logger: logger, Frames: frames, NewDetector: factory.New})
	persondetecttest.StopOnCleanup(t, svc)
	s := &Server{Cfg: cfg, TokenManager: tokens, Vault: vault, PersonDetect: svc, Logger: logger}
	s.Go2RTC = tools.NewGo2RTCManager(cfg, nil, nil, logger)
	s.startPersonDetection()
	persondetecttest.WaitDetector(t, svc, state)
	mux := http.NewServeMux()
	registerGo2RTCRoutes(mux, s)
	return &detectionAPIHarness{t: t, s: s, svc: svc, factory: factory, frames: frames, mux: mux,
		viewToken: viewToken, adminToken: adminToken, desktopToken: desktopToken}
}

func (h *detectionAPIHarness) do(method, path, token, origin, body string) (*httptest.ResponseRecorder, map[string]interface{}) {
	h.t.Helper()
	var reader io.Reader
	if body != "" {
		reader = strings.NewReader(body)
	}
	r := httptest.NewRequest(method, "http://aurago.local"+go2RTCDetectionPrefix+path, reader)
	r.Host = "aurago.local"
	if token != "" {
		r.Header.Set("Authorization", "Bearer "+token)
	}
	if origin != "" {
		r.Header.Set("Origin", origin)
	}
	w := httptest.NewRecorder()
	h.mux.ServeHTTP(w, r)
	var decoded map[string]interface{}
	if strings.HasPrefix(w.Header().Get("Content-Type"), "application/json") {
		_ = json.Unmarshal(w.Body.Bytes(), &decoded)
	}
	return w, decoded
}

// waitForEvent lets the background loops record one event with its start frame and
// returns its id. The event row is written before its frames, so wait for the cover.
func (h *detectionAPIHarness) waitForEvent() int64 {
	h.t.Helper()
	h.factory.Script.SetBoxes(persondetect.Box{X1: 0.4, Y1: 0.3, X2: 0.5, Y2: 0.9, Score: 0.9})
	var id int64
	persondetecttest.WaitFor(h.t, 10*time.Second, "a detection event with a stored frame", func() bool {
		page, err := h.svc.Events(context.Background(), persondetect.EventQuery{})
		if err != nil || len(page.Events) == 0 || page.Events[0].FrameCount == 0 || page.Events[0].CoverFrameID == nil {
			return false
		}
		id = page.Events[0].ID
		return true
	})
	h.factory.Script.SetBoxes()
	return id
}

// requireJSONList fails unless value is a JSON array (never null).
func requireJSONList(t *testing.T, what string, value interface{}) []interface{} {
	t.Helper()
	list, ok := value.([]interface{})
	if !ok || list == nil {
		t.Fatalf("%s = %#v, want a JSON list (never null)", what, value)
	}
	return list
}

func TestGo2RTCDetectionAPIReadGates(t *testing.T) {
	h := newDetectionAPIHarness(t)
	w, body := h.do(http.MethodGet, "state", h.viewToken, "", "")
	status, ok := body["status"].(map[string]interface{})
	if w.Code != http.StatusOK || !ok || status["enabled"] != true || w.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("state = %d %v %q", w.Code, body, w.Header().Get("Cache-Control"))
	}
	if _, hasEvents := body["events"]; hasEvents {
		t.Fatal("state without since must return the status only")
	}
	requireJSONList(t, "status.cameras", status["cameras"])
	if w, _ := h.do(http.MethodGet, "state", h.desktopToken, "", ""); w.Code != http.StatusForbidden {
		t.Fatalf("general desktop token = %d, want 403", w.Code)
	}
	w, body = h.do(http.MethodGet, "state?since=0", h.adminToken, "", "")
	for _, key := range []string{"status", "events", "arm_log", "cursor"} {
		if _, ok := body[key]; w.Code != http.StatusOK || !ok {
			t.Fatalf("state?since=0 = %d, missing %q: %v", w.Code, key, body)
		}
	}
	requireJSONList(t, "events", body["events"])
	requireJSONList(t, "arm_log", body["arm_log"])
	if w, body := h.do(http.MethodGet, "state?since=abc", h.viewToken, "", ""); w.Code != http.StatusBadRequest || body["code"] != "invalid_request" {
		t.Fatalf("bad cursor = %d %v", w.Code, body)
	}
	if w, body := h.do(http.MethodGet, "nope", h.viewToken, "", ""); w.Code != http.StatusNotFound || body["code"] != "not_found" {
		t.Fatalf("unknown route = %d %v", w.Code, body)
	}
	if w, body := h.do(http.MethodPut, "state", h.adminToken, "", "{}"); w.Code != http.StatusMethodNotAllowed || body["code"] != "method_not_allowed" || w.Header().Get("Allow") != "GET" {
		t.Fatalf("wrong method = %d %v %q", w.Code, body, w.Header().Get("Allow"))
	}
}

func TestGo2RTCDetectionAPIWriteGates(t *testing.T) {
	h := newDetectionAPIHarness(t)
	if w, body := h.do(http.MethodPost, "arm", h.viewToken, "", `{"mode":"on"}`); w.Code != http.StatusForbidden || body["code"] != "forbidden" {
		t.Fatalf("view token on arm = %d %v", w.Code, body)
	}
	if w, body := h.do(http.MethodPost, "arm", h.adminToken, "https://evil.example", `{"mode":"on"}`); w.Code != http.StatusForbidden || body["code"] != "forbidden" {
		t.Fatalf("cross-origin arm = %d %v", w.Code, body)
	}
	w, body := h.do(http.MethodPost, "arm", h.adminToken, "http://aurago.local", `{"mode":"on"}`)
	if w.Code != http.StatusOK || body["mode"] != "on" || body["armed"] != true || body["changed_by"] != "user" {
		t.Fatalf("arm = %d %v", w.Code, body)
	}
	if w, body := h.do(http.MethodPost, "arm", h.adminToken, "", `{"mode":"panic"}`); w.Code != http.StatusBadRequest || body["code"] != "invalid_request" {
		t.Fatalf("invalid mode = %d %v", w.Code, body)
	}
	if w, body := h.do(http.MethodPost, "arm", h.adminToken, "", `{"mode":"on","extra":1}`); w.Code != http.StatusBadRequest || body["code"] != "invalid_request" {
		t.Fatalf("unknown field = %d %v", w.Code, body)
	}
	// Request bodies are bounded before decoding: valid JSON padded with 128 KiB of
	// whitespace is refused.
	oversized := `{"mode":"on"` + strings.Repeat(" ", 128<<10) + `}`
	if w, body := h.do(http.MethodPost, "arm", h.adminToken, "", oversized); w.Code != http.StatusBadRequest || body["code"] != "invalid_request" {
		t.Fatalf("oversized body = %d %v", w.Code, body)
	}
	if w, _ := h.do(http.MethodDelete, "events/1", h.viewToken, "", ""); w.Code != http.StatusForbidden {
		t.Fatalf("view token on delete = %d", w.Code)
	}
	if w, _ := h.do(http.MethodPost, "retry", h.viewToken, "", ""); w.Code != http.StatusForbidden {
		t.Fatalf("view token on retry = %d", w.Code)
	}
	if w, body := h.do(http.MethodPost, "retry", h.adminToken, "", ""); w.Code != http.StatusOK || body["status"] == nil {
		t.Fatalf("retry = %d %v", w.Code, body)
	}
}

func TestGo2RTCDetectionAPIEventsImagesAndDelete(t *testing.T) {
	h := newDetectionAPIHarness(t)
	id := h.waitForEvent()
	if id < 1_000_000_000_000 {
		t.Fatalf("event id %d is not seeded from Unix milliseconds", id)
	}
	idText := strconv.FormatInt(id, 10)
	w, body := h.do(http.MethodGet, "events?stream_id=front-door&limit=1", h.viewToken, "", "")
	events := requireJSONList(t, "events", body["events"])
	if w.Code != http.StatusOK || len(events) != 1 || events[0].(map[string]interface{})["id"] != float64(id) {
		t.Fatalf("events = %d %v", w.Code, body)
	}
	requireJSONList(t, "events[0].zones", events[0].(map[string]interface{})["zones"])
	if w, body := h.do(http.MethodGet, "events?stream_id=front-door&before="+idText, h.viewToken, "", ""); w.Code != http.StatusOK || len(requireJSONList(t, "older events", body["events"])) != 0 {
		t.Fatalf("events before the first = %d %v", w.Code, body)
	}
	for _, query := range []string{"stream_id=Front", "limit=101", "limit=0", "from=yesterday", "before=0", "from=2026-10-12T00:00:00Z&to=2026-10-11T00:00:00Z"} {
		if w, body := h.do(http.MethodGet, "events?"+query, h.viewToken, "", ""); w.Code != http.StatusBadRequest || body["code"] != "invalid_request" {
			t.Fatalf("events?%s = %d %v", query, w.Code, body)
		}
	}
	w, body = h.do(http.MethodGet, "events/"+idText, h.viewToken, "", "")
	frames := requireJSONList(t, "frames", body["frames"])
	if w.Code != http.StatusOK || len(frames) == 0 {
		t.Fatalf("event detail = %d %v", w.Code, body)
	}
	if boxes := requireJSONList(t, "frames[0].boxes", frames[0].(map[string]interface{})["boxes"]); len(boxes) != 1 {
		t.Fatalf("stored boxes = %v", boxes)
	}
	frameID := int64(frames[0].(map[string]interface{})["id"].(float64))
	frameText := strconv.FormatInt(frameID, 10)
	w, _ = h.do(http.MethodGet, "images/"+frameText, h.viewToken, "", "")
	if w.Code != http.StatusOK || w.Header().Get("Content-Type") != "image/jpeg" || !bytes.HasPrefix(w.Body.Bytes(), []byte{0xff, 0xd8}) ||
		w.Header().Get("Cache-Control") != "private, max-age=86400, immutable" {
		t.Fatalf("image = %d %q %q", w.Code, w.Header().Get("Content-Type"), w.Header().Get("Cache-Control"))
	}
	w, _ = h.do(http.MethodGet, "images/"+frameText+"?thumb=1", h.viewToken, "", "")
	if cfg, err := jpeg.DecodeConfig(bytes.NewReader(w.Body.Bytes())); w.Code != http.StatusOK || err != nil || cfg.Width != persondetect.ThumbnailWidth {
		t.Fatalf("thumbnail = %d, %+v, %v", w.Code, cfg, err)
	}
	for _, path := range []string{"images/999999", "images/abc", "images/99999999999999999999"} {
		if w, body := h.do(http.MethodGet, path, h.viewToken, "", ""); w.Code != http.StatusNotFound || body["code"] != "frame_not_found" {
			t.Fatalf("%s = %d %v", path, w.Code, body)
		}
	}
	// A power loss can leave a 0-byte frame file (frames are written without
	// fsync); it must not be served, or the browser would cache an empty image.
	framePath, err := h.svc.FrameFile(context.Background(), frameID, false)
	if err != nil {
		t.Fatalf("FrameFile: %v", err)
	}
	if err := os.Truncate(framePath, 0); err != nil {
		t.Fatal(err)
	}
	if w, body := h.do(http.MethodGet, "images/"+frameText, h.viewToken, "", ""); w.Code != http.StatusNotFound || body["code"] != "frame_not_found" || w.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("0-byte image = %d %v %q", w.Code, body, w.Header().Get("Cache-Control"))
	}
	if w, body := h.do(http.MethodGet, "events/999999", h.viewToken, "", ""); w.Code != http.StatusNotFound || body["code"] != "event_not_found" {
		t.Fatalf("unknown event = %d %v", w.Code, body)
	}
	if w, body := h.do(http.MethodDelete, "events/"+idText, h.adminToken, "", ""); w.Code != http.StatusOK || body["deleted"] != true {
		t.Fatalf("delete = %d %v", w.Code, body)
	}
	if w, body := h.do(http.MethodGet, "events/"+idText, h.viewToken, "", ""); w.Code != http.StatusNotFound || body["code"] != "event_not_found" {
		t.Fatalf("deleted event = %d %v", w.Code, body)
	}
	if w, body := h.do(http.MethodDelete, "events/"+idText, h.adminToken, "", ""); w.Code != http.StatusNotFound || body["code"] != "event_not_found" {
		t.Fatalf("second delete = %d %v", w.Code, body)
	}
}

func TestGo2RTCDetectionAPITestEndpoint(t *testing.T) {
	h := newDetectionAPIHarness(t)
	h.factory.Script.SetBoxes(persondetect.Box{X1: 0.1, Y1: 0.2, X2: 0.3, Y2: 0.9, Score: 0.6})
	w, body := h.do(http.MethodPost, "test", h.adminToken, "", `{"stream_id":"front-door"}`)
	boxes := requireJSONList(t, "boxes", body["boxes"])
	if w.Code != http.StatusOK || !strings.HasPrefix(body["image"].(string), "data:image/jpeg;base64,") || len(boxes) != 1 || boxes[0].(map[string]interface{})["counts"] != true {
		t.Fatalf("test = %d %v", w.Code, body)
	}
	requireJSONList(t, "boxes[0].zones", boxes[0].(map[string]interface{})["zones"])
	right := `{"stream_id":"front-door","detection":{"enabled":true,"interval_seconds":2,"min_score":0.5,"cooldown_seconds":300,"zones":[{"name":"Right","points":[[0.5,0],[1,0],[1,1],[0.5,1]]}],"actions":{"message":false,"channel":"all","agent":false,"agent_instruction":"","desktop":true}}}`
	w, body = h.do(http.MethodPost, "test", h.adminToken, "", right)
	boxes = requireJSONList(t, "boxes", body["boxes"])
	if w.Code != http.StatusOK || len(boxes) != 1 || boxes[0].(map[string]interface{})["counts"] != false {
		t.Fatalf("test with an unsaved zone = %d %v", w.Code, body)
	}
	h.factory.Script.SetBoxes()
	if w, body := h.do(http.MethodPost, "test", h.adminToken, "", `{"stream_id":"front-door"}`); w.Code != http.StatusOK || len(requireJSONList(t, "empty boxes", body["boxes"])) != 0 {
		t.Fatalf("test without persons = %d %v", w.Code, body)
	}
	if w, body := h.do(http.MethodPost, "test", h.adminToken, "", `{"stream_id":"garage"}`); w.Code != http.StatusNotFound || body["code"] != "stream_not_found" {
		t.Fatalf("unknown stream = %d %v", w.Code, body)
	}
	if w, body := h.do(http.MethodPost, "test", h.adminToken, "", `{"stream_id":"front-door","detection":{"interval_seconds":2,"min_score":2,"actions":{"channel":"all"}}}`); w.Code != http.StatusBadRequest || body["field"] != "detection.min_score" {
		t.Fatalf("invalid override = %d %v", w.Code, body)
	}
	if w, body := h.do(http.MethodPost, "test", h.adminToken, "", `{"stream_id":"front-door","detection":{"interval_seconds":2,"min_score":0.5,"zones":[{"name":"A","points":[[0,0],[1,0],[1,1,1]]}],"actions":{"channel":"all"}}}`); w.Code != http.StatusBadRequest || body["field"] != "detection.zones[0].points[2]" {
		t.Fatalf("three-coordinate point = %d %v", w.Code, body)
	}
	h.frames.Fail("front-door", errors.New("offline"))
	if w, body := h.do(http.MethodPost, "test", h.adminToken, "", `{"stream_id":"front-door"}`); w.Code != http.StatusBadGateway || body["code"] != "frame_unavailable" {
		t.Fatalf("frame failure = %d %v", w.Code, body)
	}
}

func TestGo2RTCDetectionAPIWhenDetectionIsDisabled(t *testing.T) {
	h := newDetectionAPIHarness(t)
	next := *h.s.Cfg
	next.Go2RTC.Detection.Enabled = false
	h.s.CfgMu.Lock()
	h.s.Cfg = &next
	h.s.CfgMu.Unlock()
	h.s.syncPersonDetectionSettings()
	if w, body := h.do(http.MethodPost, "arm", h.adminToken, "", `{"mode":"on"}`); w.Code != http.StatusConflict || body["code"] != "detection_disabled" {
		t.Fatalf("arm while disabled = %d %v", w.Code, body)
	}
	if w, body := h.do(http.MethodPost, "test", h.adminToken, "", `{"stream_id":"front-door"}`); w.Code != http.StatusConflict || body["code"] != "detection_disabled" {
		t.Fatalf("test while disabled = %d %v", w.Code, body)
	}
	w, body := h.do(http.MethodGet, "state", h.viewToken, "", "")
	if status := body["status"].(map[string]interface{}); w.Code != http.StatusOK || status["enabled"] != false {
		t.Fatalf("state while disabled = %d %v", w.Code, body)
	}
	if w, _ := h.do(http.MethodPost, "retry", h.adminToken, "", ""); w.Code != http.StatusOK {
		t.Fatalf("retry while disabled = %d", w.Code)
	}
	h.s.PersonDetect = nil
	if w, body := h.do(http.MethodGet, "state", h.viewToken, "", ""); w.Code != http.StatusServiceUnavailable || body["code"] != "detection_unavailable" {
		t.Fatalf("no service = %d %v", w.Code, body)
	}
}

// An enabled service whose detections.db cannot be opened answers 503
// detection_unavailable, not "switched off"; the state still reports why.
func TestGo2RTCDetectionAPIAnswersUnavailableWithoutTheDatabase(t *testing.T) {
	dataDir := filepath.Join(t.TempDir(), "data")
	if err := os.MkdirAll(dataDir, 0o750); err != nil {
		t.Fatal(err)
	}
	// <data>/go2rtc is a file, so detections.db cannot be created below it.
	if err := os.WriteFile(filepath.Join(dataDir, "go2rtc"), []byte("not a directory"), 0o600); err != nil {
		t.Fatal(err)
	}
	h := newDetectionAPIHarnessAt(t, dataDir, persondetect.DetectorError)
	for _, request := range []struct{ method, path, body string }{
		{http.MethodGet, "events", ""},
		{http.MethodGet, "events/1800000000000", ""},
		{http.MethodGet, "images/1800000000000", ""},
		{http.MethodDelete, "events/1800000000000", ""},
		{http.MethodPost, "arm", `{"mode":"on"}`},
	} {
		if w, body := h.do(request.method, request.path, h.adminToken, "", request.body); w.Code != http.StatusServiceUnavailable || body["code"] != "detection_unavailable" {
			t.Fatalf("%s %s = %d %v", request.method, request.path, w.Code, body)
		}
	}
	w, body := h.do(http.MethodGet, "state?since=0", h.viewToken, "", "")
	status, _ := body["status"].(map[string]interface{})
	detector, _ := status["detector"].(map[string]interface{})
	if w.Code != http.StatusOK || detector["state"] != persondetect.DetectorError || len(requireJSONList(t, "events", body["events"])) != 0 {
		t.Fatalf("state without a database = %d %v", w.Code, body)
	}
}

func TestGo2RTCDetectionAPISettingsSavesConfig(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/go2rtc/proxy/api":
			_, _ = w.Write([]byte(`{"version":"1.9.14"}`))
		case r.Method == http.MethodPatch && r.URL.Path == "/api/go2rtc/proxy/api/streams":
			w.WriteHeader(http.StatusOK)
		default:
			http.NotFound(w, r)
		}
	}))
	defer upstream.Close()
	server, configPath, _ := newGo2RTCMutationTestServer(t, upstream.URL, true)
	factory := persondetecttest.NewFactory(persondetecttest.CPUInfo())
	server.PersonDetect = persondetect.NewService(persondetect.Deps{
		Logger: slog.New(slog.NewTextHandler(io.Discard, nil)), Frames: persondetecttest.NewFrames(), NewDetector: factory.New,
	})
	persondetecttest.StopOnCleanup(t, server.PersonDetect)
	server.startPersonDetection()
	mux := http.NewServeMux()
	registerGo2RTCRoutes(mux, server)
	put := func(body string) (*httptest.ResponseRecorder, map[string]interface{}) {
		r := httptest.NewRequest(http.MethodPut, "http://aurago.local/api/go2rtc/detection/settings", strings.NewReader(body))
		r.Host = "aurago.local"
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, r)
		var decoded map[string]interface{}
		_ = json.Unmarshal(w.Body.Bytes(), &decoded)
		return w, decoded
	}
	w, body := put(`{"enabled":true,"model":"accurate","backend":"cpu","retention_days":7,"max_storage_mb":500,"schedule":[{"days":["mon","fri"],"from":"22:00","to":"06:00"}]}`)
	detection, _ := body["detection"].(map[string]interface{})
	if w.Code != http.StatusOK || body["status"] != "ok" || detection["model"] != "accurate" {
		t.Fatalf("PUT settings = %d %v", w.Code, body)
	}
	saved, err := config.Load(configPath)
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	if d := saved.Go2RTC.Detection; !d.Enabled || d.Model != "accurate" || d.RetentionDays != 7 || len(d.Schedule) != 1 || d.Schedule[0].Days[1] != "fri" {
		t.Fatalf("saved detection = %+v", d)
	}
	persondetecttest.WaitDetector(t, server.PersonDetect, persondetect.DetectorReady)
	if opts := factory.LastOptions(); opts.Model != persondetect.ModelAccurate || opts.Backend != persondetect.BackendCPU {
		t.Fatalf("the published settings did not reach the service: %+v", opts)
	}
	if w, body := put(`{"retention_days":400}`); w.Code != http.StatusBadRequest || body["code"] != "invalid_request" || body["field"] != "retention_days" {
		t.Fatalf("invalid retention = %d %v", w.Code, body)
	}
	if w, body := put(`{"foo":1}`); w.Code != http.StatusBadRequest || body["code"] != "invalid_request" {
		t.Fatalf("unknown field = %d %v", w.Code, body)
	}
}

func TestGo2RTCDetectionIsReachableThroughTheDesktopProxy(t *testing.T) {
	h := newDetectionAPIHarness(t)
	h.s.CfgMu.Lock()
	h.s.Cfg.Auth.Enabled = false
	h.s.CfgMu.Unlock()
	handler := desktopIntegrationHandler(h.s, h.mux)
	call := func(method, target, body string) (*httptest.ResponseRecorder, map[string]interface{}) {
		var reader io.Reader
		if body != "" {
			reader = strings.NewReader(body)
		}
		r := httptest.NewRequest(method, "http://aurago.local"+target, reader)
		r.Host = "aurago.local"
		w := httptest.NewRecorder()
		handler(w, r)
		var decoded map[string]interface{}
		_ = json.Unmarshal(w.Body.Bytes(), &decoded)
		return w, decoded
	}
	w, body := call(http.MethodGet, "/api/desktop/integrations/go2rtc/detection/state?since=0", "")
	if w.Code != http.StatusOK || body["status"] == nil || body["cursor"] == nil {
		t.Fatalf("desktop proxy state = %d %v (must reach the detection handler, not the go2rtc media proxy)", w.Code, body)
	}
	if w, body := call(http.MethodPost, "/api/desktop/integrations/go2rtc/detection/arm", `{"mode":"on"}`); w.Code != http.StatusOK || body["mode"] != "on" {
		t.Fatalf("desktop proxy arm = %d %v", w.Code, body)
	}
	h.s.CfgMu.Lock()
	h.s.Cfg.VirtualDesktop.ReadOnly = true
	h.s.CfgMu.Unlock()
	if w, _ := call(http.MethodPost, "/api/desktop/integrations/go2rtc/detection/arm", `{"mode":"off"}`); w.Code != http.StatusForbidden || !strings.Contains(w.Body.String(), "desktop_readonly") {
		t.Fatalf("read-only desktop arm = %d %s", w.Code, w.Body.String())
	}
	if go2RTCProxyPathAllowed("detection/state", http.MethodGet, false) {
		t.Fatal("the go2rtc media proxy must never serve detection paths")
	}
}
