package agent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"aurago/internal/config"
	"aurago/internal/persondetect"
	"aurago/internal/tools"
)

type fakeDetectionAPI struct {
	mu       sync.Mutex
	status   detectionStatusView
	arm      detectionArmView
	err      error
	frameErr error
	armModes []string
	page     detectionEventPageView
	queries  []detectionEventQuery
	events   map[int64]detectionEventView
	// frames maps a frame id to its path below frameRoot.
	frameRoot string
	frames    map[int64]string
}

func (f *fakeDetectionAPI) Status(context.Context) (detectionStatusView, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.status, f.err
}

func (f *fakeDetectionAPI) SetArmMode(_ context.Context, mode string) (detectionArmView, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.armModes = append(f.armModes, mode)
	if f.err != nil {
		return detectionArmView{}, f.err
	}
	arm := f.arm
	arm.Mode = mode
	return arm, nil
}

func (f *fakeDetectionAPI) Events(_ context.Context, q detectionEventQuery) (detectionEventPageView, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.queries = append(f.queries, q)
	return f.page, f.err
}

func (f *fakeDetectionAPI) Event(_ context.Context, id int64) (detectionEventView, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.err != nil {
		return detectionEventView{}, f.err
	}
	ev, ok := f.events[id]
	if !ok {
		return detectionEventView{}, persondetect.ErrUnknownEvent
	}
	return ev, nil
}

func (f *fakeDetectionAPI) FrameLocation(_ context.Context, frameID int64) (string, string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.frameErr != nil {
		return "", "", f.frameErr
	}
	rel, ok := f.frames[frameID]
	if !ok {
		return "", "", errors.New("frame file is gone")
	}
	return f.frameRoot, rel, nil
}

func useDetectionAPI(t *testing.T, api detectionAPI) {
	t.Helper()
	previous := currentDetectionAPI
	currentDetectionAPI = func() detectionAPI { return api }
	t.Cleanup(func() { currentDetectionAPI = previous })
}

func detectionTestConfig() *config.Config {
	cfg := &config.Config{}
	cfg.Go2RTC.Enabled, cfg.Go2RTC.AgentAccess = true, true
	cfg.Go2RTC.Detection.Enabled = true
	return cfg
}

func detectionTestEvent() detectionEventView {
	cover := int64(7)
	ended := time.Date(2026, 10, 11, 19, 15, 0, 0, time.UTC)
	return detectionEventView{
		ID: 42, CameraID: "garden", CameraName: "Garten <b>", StartedAt: time.Date(2026, 10, 11, 19, 14, 3, 0, time.UTC),
		EndedAt: &ended, MaxScore: 0.91, MaxPersons: 2, Zones: []string{"Einfahrt"}, Armed: true, CoverFrameID: &cover, FrameCount: 3,
		Alerts: map[string]detectionAlertView{"message": {Status: "sent"}, "agent": {Status: "skipped", Detail: "queue full"}},
	}
}

func decodeDetectionOutput(t *testing.T, out string) map[string]any {
	t.Helper()
	raw, ok := strings.CutPrefix(out, "Tool Output: ")
	if !ok {
		t.Fatalf("output lacks the Tool Output prefix: %s", out)
	}
	var m map[string]any
	if err := json.Unmarshal([]byte(raw), &m); err != nil {
		t.Fatalf("output is not JSON: %v\n%s", err, out)
	}
	return m
}

func runDetection(t *testing.T, dc *DispatchContext, operation string, params map[string]interface{}) map[string]any {
	t.Helper()
	tc := ToolCall{Action: "go2rtc", Operation: operation, Params: params}
	return decodeDetectionOutput(t, dispatchGo2RTCDetection(context.Background(), operation, tc, dc))
}

func TestGo2RTCDetectionGating(t *testing.T) {
	cfg := detectionTestConfig()
	cfg.Go2RTC.Detection.Enabled = false
	dc := &DispatchContext{Cfg: cfg, Logger: testLogger}
	useDetectionAPI(t, &fakeDetectionAPI{})
	if m := runDetection(t, dc, "detection_status", nil); m["status"] != "error" || m["code"] != "detection_disabled" {
		t.Fatalf("detection switched off: %v", m)
	}
	cfg.Go2RTC.Detection.Enabled = true
	useDetectionAPI(t, nil)
	if m := runDetection(t, dc, "detection_status", nil); m["code"] != "detection_unavailable" {
		t.Fatalf("no service: %v", m)
	}
	useDetectionAPI(t, &fakeDetectionAPI{err: persondetect.ErrDisabled})
	if m := runDetection(t, dc, "detection_status", nil); m["code"] != "detection_disabled" {
		t.Fatalf("service reports disabled: %v", m)
	}
	if m := runDetection(t, dc, "detection_dance", nil); m["code"] != "invalid_request" {
		t.Fatalf("unknown operation: %v", m)
	}
}

func TestGo2RTCDetectionStatusIsolatesCameraNames(t *testing.T) {
	last := int64(41)
	at := time.Date(2026, 10, 11, 18, 0, 0, 0, time.UTC)
	useDetectionAPI(t, &fakeDetectionAPI{status: detectionStatusView{
		Enabled: true,
		Detector: detectionDetectorView{State: "ready", Model: "standard", ModelName: "D-FINE-N", Backend: "gpu",
			RequestedBackend: "auto", WallMS: 108.4, CPUMS: 14.2},
		Arm: detectionArmView{Mode: "schedule", Armed: true, ChangedBy: "schedule"},
		Cameras: []detectionCameraView{{ID: "garden", Name: "<b>Garten</b>", State: "active", AnalysesPerMinute: 29.5,
			LastEventID: &last, LastEventAt: &at}},
	}})
	cfg := detectionTestConfig()
	cfg.Go2RTC.Streams = []config.Go2RTCStreamConfig{
		{ID: "garden", Name: "Garten", Enabled: true},
		{ID: "door", Name: "Haustür", Enabled: true},
		{ID: "old", Name: "Alt", Enabled: false},
	}
	cfg.Go2RTC.Streams[0].Detection.Enabled = true
	m := runDetection(t, &DispatchContext{Cfg: cfg, Logger: testLogger}, "detection_status", nil)
	detector, _ := m["detector"].(map[string]any)
	arm, _ := m["arm"].(map[string]any)
	cameras, _ := m["cameras"].(string)
	if m["status"] != "ok" || m["operation"] != "detection_status" || detector["state"] != "ready" || detector["backend"] != "gpu" ||
		detector["wall_ms"] != 108.0 || arm["mode"] != "schedule" || arm["armed"] != true {
		t.Fatalf("status = %v", m)
	}
	wantGarden := "garden | &lt;b&gt;Garten&lt;/b&gt; | active | 29.5 | 41 | " + at.Local().Format(time.RFC3339)
	if !strings.HasPrefix(cameras, "<external_data>") || !strings.Contains(cameras, wantGarden) ||
		!strings.Contains(cameras, "door | Haustür | off | 0.0 | - | -") || strings.Contains(cameras, "old |") || strings.Contains(cameras, "<b>") {
		t.Fatalf("cameras = %q", cameras)
	}
}

func TestGo2RTCSetArmMode(t *testing.T) {
	api := &fakeDetectionAPI{arm: detectionArmView{Armed: true, ChangedBy: "agent"}}
	useDetectionAPI(t, api)
	dc := &DispatchContext{Cfg: detectionTestConfig(), Logger: testLogger}
	m := runDetection(t, dc, "set_arm_mode", map[string]interface{}{"mode": " On "})
	arm, _ := m["arm"].(map[string]any)
	if m["status"] != "ok" || arm["mode"] != "on" || arm["changed_by"] != "agent" || strings.Join(api.armModes, ",") != "on" {
		t.Fatalf("set on = %v, calls %v", m, api.armModes)
	}
	if m := runDetection(t, dc, "set_arm_mode", map[string]interface{}{"mode": "panic"}); m["code"] != "invalid_request" {
		t.Fatalf("bad mode: %v", m)
	}
	if m := runDetection(t, dc, "set_arm_mode", nil); m["code"] != "invalid_request" {
		t.Fatalf("missing mode: %v", m)
	}
	if len(api.armModes) != 1 {
		t.Fatalf("invalid modes must not reach the service: %v", api.armModes)
	}
	api.err = persondetect.ErrDisabled
	if m := runDetection(t, dc, "set_arm_mode", map[string]interface{}{"mode": "off"}); m["code"] != "detection_disabled" {
		t.Fatalf("service off: %v", m)
	}
}

func TestGo2RTCListDetectionEventsParsesFilters(t *testing.T) {
	next := int64(30)
	api := &fakeDetectionAPI{page: detectionEventPageView{Events: []detectionEventView{detectionTestEvent()}, NextBefore: &next}}
	useDetectionAPI(t, api)
	cfg := detectionTestConfig()
	cfg.Go2RTC.Streams = []config.Go2RTCStreamConfig{{ID: "garden", Name: "Garten", Enabled: true}}
	dc := &DispatchContext{Cfg: cfg, Logger: testLogger}
	m := runDetection(t, dc, "list_detection_events", map[string]interface{}{
		"stream_id": " Garden ", "since": "2026-10-11", "until": "2026-10-11T20:00:00Z", "limit": float64(99),
	})
	if len(api.queries) != 1 {
		t.Fatalf("queries = %+v", api.queries)
	}
	q := api.queries[0]
	if q.CameraID != "garden" || !q.From.Equal(time.Date(2026, 10, 11, 0, 0, 0, 0, time.Local)) ||
		!q.To.Equal(time.Date(2026, 10, 11, 20, 0, 0, 0, time.UTC)) || q.Limit != 50 {
		t.Fatalf("query = %+v", q)
	}
	ev := detectionTestEvent()
	wantLine := "42 | garden | Garten &lt;b&gt; | " + ev.StartedAt.Local().Format(time.RFC3339) + " | " +
		ev.EndedAt.Local().Format(time.RFC3339) + " | 91% | 2 | Einfahrt | yes | message=sent, agent=skipped (queue full)"
	events, _ := m["events"].(string)
	if m["status"] != "ok" || m["returned"] != 1.0 || m["next_before"] != 30.0 || !strings.HasPrefix(events, "<external_data>") ||
		!strings.Contains(events, wantLine) {
		t.Fatalf("list = %v\nwant line %q", m, wantLine)
	}
	// The service's To is exclusive, so a date as until ends at the next local midnight.
	runDetection(t, dc, "list_detection_events", map[string]interface{}{"until": "2026-10-11"})
	q = api.queries[1]
	if q.Limit != 10 || !q.From.IsZero() || q.CameraID != "" || !q.To.Equal(time.Date(2026, 10, 12, 0, 0, 0, 0, time.Local)) {
		t.Fatalf("defaults = %+v", q)
	}
	// A time without an offset is server time.
	runDetection(t, dc, "list_detection_events", map[string]interface{}{"since": "2026-10-11T20:00"})
	if q = api.queries[2]; !q.From.Equal(time.Date(2026, 10, 11, 20, 0, 0, 0, time.Local)) || !q.To.IsZero() {
		t.Fatalf("local time = %+v", q)
	}
	for _, params := range []map[string]interface{}{
		{"since": "gestern"}, {"until": "11.10.2026"}, {"since": "2026-10-12", "until": "2026-10-11"},
		// The service answers an unknown camera with an empty page; the tool names the mistake.
		{"stream_id": "attic"},
	} {
		if m := runDetection(t, dc, "list_detection_events", params); m["code"] != "invalid_request" {
			t.Fatalf("%v: %v", params, m)
		}
	}
	if len(api.queries) != 3 {
		t.Fatalf("invalid filters must not query: %+v", api.queries)
	}
}

func TestGo2RTCAnalyzeDetectionEventUsesTheCoverFrame(t *testing.T) {
	dc, path, _, calls := newVisionInputFixture(t)
	dc.Cfg.Go2RTC.Enabled, dc.Cfg.Go2RTC.AgentAccess, dc.Cfg.Go2RTC.Detection.Enabled = true, true, true
	noCover := detectionTestEvent()
	noCover.ID, noCover.CoverFrameID = 43, nil
	useDetectionAPI(t, &fakeDetectionAPI{
		events:    map[int64]detectionEventView{42: detectionTestEvent(), 43: noCover},
		frameRoot: filepath.Dir(path),
		frames:    map[int64]string{7: filepath.Base(path)},
	})
	m := runDetection(t, dc, "analyze_detection_event", map[string]interface{}{"event_id": float64(42), "prompt": "Trägt die Person ein Paket?"})
	if m["status"] != "ok" || m["analysis"] != "camera analysis succeeded" || m["event_id"] != 42.0 || m["camera_id"] != "garden" ||
		m["frame_id"] != 7.0 || *calls != 1 {
		t.Fatalf("analysis = %v, calls %d", m, *calls)
	}
	if _, ok := m["stream_id"]; ok {
		t.Fatal("detection results must not carry stream_id")
	}
	if got := dc.BudgetTracker.GetStatus().Models[tools.DefaultVisionModel].Calls; got != 1 {
		t.Fatalf("budget calls = %d", got)
	}
	cases := []struct {
		params map[string]interface{}
		code   string
	}{
		{nil, "invalid_request"},
		{map[string]interface{}{"event_id": "abc"}, "invalid_request"},
		{map[string]interface{}{"event_id": float64(99)}, "event_not_found"},
		{map[string]interface{}{"event_id": "43"}, "event_not_found"},
	}
	for _, c := range cases {
		if m := runDetection(t, dc, "analyze_detection_event", c.params); m["code"] != c.code {
			t.Fatalf("%v: %v, want %s", c.params, m, c.code)
		}
	}
	if *calls != 1 {
		t.Fatalf("failed lookups must not call the vision provider, calls = %d", *calls)
	}
}

func TestGo2RTCShowDetectionEventSendsTheCoverImage(t *testing.T) {
	api := &fakeDetectionAPI{
		events:    map[int64]detectionEventView{42: detectionTestEvent()},
		frameRoot: "/data/go2rtc/detections",
		frames:    map[int64]string{7: "2026/10/11/garden/42_1.jpg"},
	}
	useDetectionAPI(t, api)
	broker := &captureBroker{}
	dc := &DispatchContext{Cfg: detectionTestConfig(), Logger: testLogger, Broker: broker}
	m := runDetection(t, dc, "show_detection_event", map[string]interface{}{"event_id": float64(42)})
	if m["status"] != "ok" || m["shown"] != true || m["camera_id"] != "garden" || m["frame_id"] != 7.0 {
		t.Fatalf("show = %v", m)
	}
	if _, ok := m["stream_id"]; ok {
		t.Fatal("detection results must not carry stream_id")
	}
	follow, _ := m["recommended_follow_up"].(map[string]any)
	args, _ := follow["arguments"].(map[string]any)
	if args["operation"] != "analyze_detection_event" || args["event_id"] != 42.0 {
		t.Fatalf("follow-up = %v", follow)
	}
	if len(broker.events) != 1 || broker.events[0].event != "image" {
		t.Fatalf("broker events = %+v", broker.events)
	}
	var image map[string]string
	if err := json.Unmarshal([]byte(broker.events[0].message), &image); err != nil ||
		image["path"] != "/api/go2rtc/detection/images/7" || !strings.HasPrefix(image["caption"], "Person detection: garden, ") {
		t.Fatalf("image event = %q (%v)", broker.events[0].message, err)
	}
	api.frames = map[int64]string{}
	if m := runDetection(t, dc, "show_detection_event", map[string]interface{}{"event_id": float64(42)}); m["code"] != "event_not_found" || len(broker.events) != 1 {
		t.Fatalf("removed frame: %v, events %d", m, len(broker.events))
	}
	// A database that is not open is a passing state, not a removed picture.
	api.frameErr = persondetect.ErrUnavailable
	if m := runDetection(t, dc, "show_detection_event", map[string]interface{}{"event_id": float64(42)}); m["code"] != "detection_unavailable" || len(broker.events) != 1 {
		t.Fatalf("database not open: %v, events %d", m, len(broker.events))
	}
}

func TestDetectionEventIDAcceptsNumbersAndDigits(t *testing.T) {
	for raw, want := range map[any]int64{float64(42): 42, 42: 42, int64(7): 7, "15": 15, " 9 ": 9, json.Number("11"): 11} {
		if got, ok := detectionEventID(map[string]interface{}{"event_id": raw}); !ok || got != want {
			t.Errorf("%#v = %d, %v; want %d", raw, got, ok, want)
		}
	}
	for _, raw := range []any{float64(1.5), float64(0), -3, "x", nil, true} {
		if got, ok := detectionEventID(map[string]interface{}{"event_id": raw}); ok {
			t.Errorf("%#v accepted as %d", raw, got)
		}
	}
}

// The adapter talks to the real service through the C4 JSON shapes; an unconfigured service
// reports detection as off.
func TestPersonDetectAPIDecodesTheServiceStatus(t *testing.T) {
	svc := persondetect.NewService(persondetect.Deps{Logger: testLogger})
	t.Cleanup(func() { _ = svc.Shutdown(context.Background()) })
	status, err := personDetectAPI{svc: svc}.Status(context.Background())
	if err != nil || status.Enabled || status.Arm.UTCOffset == "" {
		t.Fatalf("status = %+v, %v", status, err)
	}
}

// The tool asks the service only while go2rtc.detection.enabled is on. A service that is not
// configured yet answers reads with "unknown event", an empty page or ErrDisabled; the
// adapter reports all of them as not running yet (detection_unavailable), not as off.
func TestPersonDetectAPIReportsANotYetConfiguredServiceAsUnavailable(t *testing.T) {
	svc := persondetect.NewService(persondetect.Deps{Logger: testLogger})
	t.Cleanup(func() { _ = svc.Shutdown(context.Background()) })
	api := personDetectAPI{svc: svc}
	ctx := context.Background()
	if _, err := api.Events(ctx, detectionEventQuery{Limit: 10}); !errors.Is(err, persondetect.ErrUnavailable) {
		t.Errorf("Events = %v", err)
	}
	if _, err := api.Event(ctx, 1_800_000_000_001); !errors.Is(err, persondetect.ErrUnavailable) {
		t.Errorf("Event = %v", err)
	}
	if _, _, err := api.FrameLocation(ctx, 1_800_000_000_002); !errors.Is(err, persondetect.ErrUnavailable) {
		t.Errorf("FrameLocation = %v", err)
	}
	if _, err := api.SetArmMode(ctx, "on"); !errors.Is(err, persondetect.ErrUnavailable) {
		t.Errorf("SetArmMode = %v", err)
	}
	cfg := detectionTestConfig()
	useDetectionAPI(t, api)
	if m := runDetection(t, &DispatchContext{Cfg: cfg, Logger: testLogger}, "list_detection_events", nil); m["code"] != "detection_unavailable" {
		t.Errorf("list = %v", m)
	}
}

// Autonomous runs (a camera wake-up reacts to a picture anyone can hold up to the camera)
// may arm but never switch alerts off or back to the schedule; a chat with the user may.
func TestGo2RTCSetArmModeKeepsAutonomousRunsFromDisarming(t *testing.T) {
	api := &fakeDetectionAPI{}
	useDetectionAPI(t, api)
	wake := &DispatchContext{Cfg: detectionTestConfig(), Logger: testLogger, MessageSource: "camera_detection"}
	for _, mode := range []string{"off", "schedule"} {
		if m := runDetection(t, wake, "set_arm_mode", map[string]interface{}{"mode": mode}); m["code"] != "not_allowed" {
			t.Errorf("autonomous %s = %v", mode, m)
		}
	}
	if m := runDetection(t, wake, "set_arm_mode", map[string]interface{}{"mode": "on"}); m["status"] != "ok" {
		t.Errorf("autonomous on = %v", m)
	}
	chat := &DispatchContext{Cfg: detectionTestConfig(), Logger: testLogger, MessageSource: "web_chat"}
	if m := runDetection(t, chat, "set_arm_mode", map[string]interface{}{"mode": "off"}); m["status"] != "ok" {
		t.Errorf("chat off = %v", m)
	}
	if got := strings.Join(api.armModes, ","); got != "on,off" {
		t.Fatalf("modes that reached the service = %s", got)
	}
}

// linkDirOrSkip links link to the directory target: a symlink, or a junction on Windows
// without symlink rights (as internal/persondetect's symlinkOrSkip).
func linkDirOrSkip(t *testing.T, target, link string) {
	t.Helper()
	err := os.Symlink(target, link)
	if err != nil && runtime.GOOS == "windows" {
		out, jerr := exec.Command("cmd", "/c", "mklink", "/J", link, target).CombinedOutput()
		if jerr == nil {
			return
		}
		err = fmt.Errorf("%w; junction: %v: %s", err, jerr, strings.TrimSpace(string(out)))
	}
	if err != nil {
		t.Skipf("cannot create a link here: %v", err)
	}
}

// The detections root may link to another disk; the event picture is opened through
// os.Root at that root, not at the data directory (which refuses the link).
func TestGo2RTCAnalyzeDetectionEventReadsFramesBelowALinkedRoot(t *testing.T) {
	dc, _, _, calls := newVisionInputFixture(t)
	dc.Cfg.Go2RTC.Enabled, dc.Cfg.Go2RTC.AgentAccess, dc.Cfg.Go2RTC.Detection.Enabled = true, true, true
	outside := filepath.Join(t.TempDir(), "nas", "detections")
	rel := filepath.Join("2026", "10", "11", "garden", "42_1.jpg")
	if err := os.MkdirAll(filepath.Dir(filepath.Join(outside, rel)), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(outside, rel), visionInputJPEG, 0o640); err != nil {
		t.Fatal(err)
	}
	linkedRoot := filepath.Join(dc.Cfg.Directories.DataDir, "go2rtc", "detections")
	if err := os.MkdirAll(filepath.Dir(linkedRoot), 0o750); err != nil {
		t.Fatal(err)
	}
	linkDirOrSkip(t, outside, linkedRoot)
	useDetectionAPI(t, &fakeDetectionAPI{
		events:    map[int64]detectionEventView{42: detectionTestEvent()},
		frameRoot: linkedRoot,
		frames:    map[int64]string{7: rel},
	})
	// Without a prompt the default one runs; it must stay a description, not a count.
	if _, counted := tools.PrepareVisionPrompt(go2RTCDetectionDefaultPrompt); counted {
		t.Fatal("the default prompt turns into a structured count")
	}
	m := runDetection(t, dc, "analyze_detection_event", map[string]interface{}{"event_id": float64(42)})
	if m["status"] != "ok" || m["analysis"] != "camera analysis succeeded" || *calls != 1 {
		t.Fatalf("analysis = %v, calls %d", m, *calls)
	}
	if _, _, cleanup, err := prepareManagedVisionInputAt(linkedRoot, filepath.Join("..", "..", "x.jpg"), dc.Cfg); err == nil {
		cleanup()
		t.Fatal("a path leaving the image root was staged")
	}
}
