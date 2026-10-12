package ui

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/proto"
)

// Event and frame ids of the fake detection API are int64 values around 1.8e12, like
// the real store's; they stay safe JavaScript integers.
const (
	ncdEventGarage    int64 = 1800000000001 // garage, cover frame ncdEventGarage*100+1
	ncdEventFrontDoor int64 = 1800000000002 // front door
	ncdEventPolled    int64 = 1800000000003 // arrives through the change feed
	ncdArmLogRow      int64 = 1800000000050
)

func ncdFrameID(eventID int64, seq int) int64 { return eventID*100 + int64(seq) }

type ncdFakeEvent struct {
	id           int64
	camera, name string
	started      string
	ended        any
}

var ncdFakeEvents = []ncdFakeEvent{
	{id: ncdEventFrontDoor, camera: "front-door", name: "Haustür", started: "2026-10-11T10:00:00.250Z", ended: "2026-10-11T10:01:05.250Z"},
	{id: ncdEventGarage, camera: "garage", name: "Garage", started: "2026-10-11T08:00:00.125Z", ended: "2026-10-11T08:00:40.125Z"},
	{id: ncdEventPolled, camera: "front-door", name: "Haustür", started: "2026-10-11T11:00:00.500Z", ended: nil},
}

// networkCamerasDetectionFixture fakes /api/desktop/integrations/go2rtc/ for
// the Network Cameras app: app state, thumbnails and the detection API (C6),
// with the JSON shapes of internal/persondetect/api_types.go.
type networkCamerasDetectionFixture struct {
	mu            sync.Mutex
	jpeg          []byte
	enabled       bool
	releaseEvent  bool
	released      bool
	armMode       string
	stateSince    []string
	armBodies     []map[string]any
	settingsBody  map[string]any
	testBody      map[string]any
	patchBody     map[string]any
	deleted       []string
	deletedEvents map[string]bool
	eventQueries  []string
}

func newNetworkCamerasDetectionFixture(t *testing.T, enabled bool) *networkCamerasDetectionFixture {
	t.Helper()
	frame := image.NewRGBA(image.Rect(0, 0, 160, 90))
	for y := 0; y < 90; y++ {
		for x := 0; x < 160; x++ {
			frame.Set(x, y, color.RGBA{R: uint8(40 + x/2), G: uint8(60 + y), B: 90, A: 255})
		}
	}
	var buffer bytes.Buffer
	if err := jpeg.Encode(&buffer, frame, &jpeg.Options{Quality: 80}); err != nil {
		t.Fatal(err)
	}
	return &networkCamerasDetectionFixture{jpeg: buffer.Bytes(), enabled: enabled, armMode: "schedule", deletedEvents: map[string]bool{}}
}

func (f *networkCamerasDetectionFixture) status() map[string]any {
	cameras := []any{}
	if f.enabled {
		cameras = []any{
			map[string]any{"id": "front-door", "name": "Haustür", "state": "active", "analyses_per_minute": 29.5, "last_event_id": ncdEventFrontDoor, "last_event_at": "2026-10-11T10:00:00.250Z", "last_error": ""},
			map[string]any{"id": "garage", "name": "Garage", "state": "unreachable", "analyses_per_minute": 0, "last_event_id": nil, "last_event_at": nil, "last_error": "frame fetch failed"},
		}
	}
	armed := f.armMode == "on" || f.armMode == "schedule"
	var next any
	if f.armMode == "schedule" {
		next = "2026-10-12T04:00:00Z"
	}
	return map[string]any{
		"enabled": f.enabled,
		"detector": map[string]any{
			"state": map[bool]string{true: "ready", false: "off"}[f.enabled], "phase": "", "download": map[string]any{"downloaded": 0, "total": 0, "percent": 0}, "error": "",
			"model": "standard", "model_name": "D-FINE-N", "requested_backend": "auto", "backend": "gpu", "runtime": "onnxruntime 1.27.0",
			"providers": []string{"WebGpuExecutionProvider", "CPUExecutionProvider"}, "fallback_reason": "", "wall_ms": 108.4, "cpu_ms": 14.2,
			"self_test": map[string]any{"passed": true, "reference_persons": 4, "cpu_wall_ms": 127, "cpu_cpu_ms": 129, "gpu_wall_ms": 108, "gpu_cpu_ms": 14},
		},
		// The server runs in UTC (a Docker image without TZ); the browser runs in Europe/Berlin.
		"arm":     map[string]any{"mode": f.armMode, "armed": armed, "next_change": next, "changed_at": "2026-10-11T18:00:00.375Z", "changed_by": "user", "time_zone": "UTC", "utc_offset": "+00:00"},
		"cameras": cameras,
		"storage": map[string]any{"bytes": 1048576, "limit_bytes": 2147483648, "warning": ""},
		"cursor":  42,
	}
}

func networkCamerasDetectionEvent(event ncdFakeEvent) map[string]any {
	return map[string]any{
		"id": event.id, "camera_id": event.camera, "camera_name": event.name, "started_at": event.started, "ended_at": event.ended,
		"max_score": 0.91, "max_persons": 2, "zones": []string{"Einfahrt"}, "armed": true, "cover_frame_id": ncdFrameID(event.id, 1), "frame_count": 2,
		"alerts": map[string]any{
			"message": map[string]any{"status": "failed", "detail": "telegram is not configured"},
			"desktop": map[string]any{"status": "sent"},
			"mission": map[string]any{"status": "sent"},
			"agent":   map[string]any{"status": "skipped", "detail": "cooldown"},
		},
	}
}

// events lists the stored events (the polled one only once it was released).
func (f *networkCamerasDetectionFixture) events(withPolled bool) []any {
	out := []any{}
	for _, event := range ncdFakeEvents {
		if (event.id == ncdEventPolled && !withPolled) || f.deletedEvents[fmt.Sprint(event.id)] {
			continue
		}
		out = append(out, networkCamerasDetectionEvent(event))
	}
	return out
}

func (f *networkCamerasDetectionFixture) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	path := strings.TrimPrefix(r.URL.Path, "/api/desktop/integrations/go2rtc/")
	write := func(status int, v any) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_ = json.NewEncoder(w).Encode(v)
	}
	body := func() map[string]any {
		var out map[string]any
		data, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(data, &out)
		return out
	}
	frameBoxes := []any{map[string]any{"x1": 0.1, "y1": 0.2, "x2": 0.3, "y2": 0.9, "score": 0.91}, map[string]any{"x1": 0.5, "y1": 0.3, "x2": 0.62, "y2": 0.85, "score": 0.64}}
	switch {
	case path == "app/state":
		detection := map[string]any{"enabled": false, "interval_seconds": 2, "min_score": 0.5, "cooldown_seconds": 300, "zones": []any{}, "actions": map[string]any{"message": false, "channel": "all", "agent": false, "agent_instruction": "", "desktop": true}}
		write(http.StatusOK, map[string]any{
			"status": "ok", "can_manage": true, "webrtc_enabled": false, "web_ui_enabled": false, "thumbnail_refresh": 5,
			"integration": map[string]any{"configured": true, "enabled": true, "container_running": true, "api_usable": true},
			"discovery":   map[string]any{"available": false, "reason": "", "reason_code": "broadcast_unavailable"},
			"streams": []any{
				map[string]any{"id": "front-door", "name": "Haustür", "reachable": true, "codecs": []string{"H264"}, "producers": 1, "consumers": 0, "detection": detection},
				map[string]any{"id": "garage", "name": "Garage", "reachable": false, "codecs": []string{}, "producers": 0, "consumers": 0},
			},
			"disabled_streams":   []any{},
			"detection_enabled":  f.enabled,
			"detection_settings": map[string]any{"enabled": f.enabled, "model": "standard", "backend": "auto", "retention_days": 14, "max_storage_mb": 2048, "schedule": []any{map[string]any{"days": []string{"sat", "sun"}, "from": "08:00", "to": "12:00"}}},
		})
	case strings.HasPrefix(path, "thumbnail/") || strings.HasPrefix(path, "detection/images/"):
		w.Header().Set("Content-Type", "image/jpeg")
		_, _ = w.Write(f.jpeg)
	case strings.HasPrefix(path, "viewer/"):
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = io.WriteString(w, "<!doctype html><title>viewer stub</title>")
	case strings.HasPrefix(path, "streams/") && r.Method == http.MethodPatch:
		f.patchBody = body()
		write(http.StatusOK, map[string]any{"status": "ok", "stream": map[string]any{"id": strings.TrimPrefix(path, "streams/")}})
	case path == "detection/state":
		since, present := r.URL.Query()["since"]
		if !present {
			f.stateSince = append(f.stateSince, "")
			write(http.StatusOK, map[string]any{"status": f.status()})
			return
		}
		f.stateSince = append(f.stateSince, since[0])
		changes := map[string]any{"status": f.status(), "events": []any{}, "arm_log": []any{}, "cursor": 42}
		if since[0] == "0" {
			// since=0 is a snapshot: the newest events and arm-log rows, cursor = status.cursor.
			changes["events"] = f.events(f.released)
			changes["arm_log"] = []any{map[string]any{"id": ncdArmLogRow, "at": "2026-10-11T09:00:00.500Z", "mode": "schedule", "armed": true, "actor": "schedule"}}
		} else if f.releaseEvent {
			f.releaseEvent, f.released = false, true
			changes["events"] = []any{networkCamerasDetectionEvent(ncdFakeEvents[2])}
			changes["cursor"] = 43
		}
		write(http.StatusOK, changes)
	case path == "detection/events":
		f.eventQueries = append(f.eventQueries, r.URL.RawQuery)
		write(http.StatusOK, map[string]any{"events": f.events(f.released), "next_before": nil})
	case strings.HasPrefix(path, "detection/events/"):
		id := strings.TrimPrefix(path, "detection/events/")
		if r.Method == http.MethodDelete {
			if f.deletedEvents[id] {
				write(http.StatusNotFound, map[string]any{"error": "Event not found", "code": "event_not_found"})
				return
			}
			f.deleted = append(f.deleted, id)
			f.deletedEvents[id] = true
			write(http.StatusOK, map[string]any{"deleted": true})
			return
		}
		var found *ncdFakeEvent
		for index := range ncdFakeEvents {
			if fmt.Sprint(ncdFakeEvents[index].id) == id {
				found = &ncdFakeEvents[index]
			}
		}
		if found == nil || f.deletedEvents[id] {
			write(http.StatusNotFound, map[string]any{"error": "Event not found", "code": "event_not_found"})
			return
		}
		detail := networkCamerasDetectionEvent(*found)
		detail["frames"] = []any{
			map[string]any{"id": ncdFrameID(found.id, 1), "seq": 1, "captured_at": "2026-10-11T10:00:02.250Z", "score": 0.91, "width": 1280, "height": 720, "boxes": frameBoxes},
			map[string]any{"id": ncdFrameID(found.id, 2), "seq": 2, "captured_at": "2026-10-11T10:00:07.250Z", "score": 0.7, "width": 1280, "height": 720, "boxes": frameBoxes[:1]},
		}
		write(http.StatusOK, detail)
	case path == "detection/arm":
		payload := body()
		f.armBodies = append(f.armBodies, payload)
		f.armMode = fmt.Sprint(payload["mode"])
		write(http.StatusOK, f.status()["arm"])
	case path == "detection/settings":
		f.settingsBody = body()
		if enabled, ok := f.settingsBody["enabled"].(bool); ok {
			f.enabled = enabled
		}
		write(http.StatusOK, map[string]any{"status": "ok", "detection": f.settingsBody})
	case path == "detection/test":
		f.testBody = body()
		write(http.StatusOK, map[string]any{
			"image": "data:image/jpeg;base64," + base64.StdEncoding.EncodeToString(f.jpeg), "width": 160, "height": 90, "backend": "gpu", "wall_ms": 108.2,
			"boxes": []any{
				map[string]any{"x1": 0.2, "y1": 0.3, "x2": 0.35, "y2": 0.9, "score": 0.88, "counts": true, "zones": []string{"Einfahrt"}},
				map[string]any{"x1": 0.6, "y1": 0.1, "x2": 0.7, "y2": 0.4, "score": 0.55, "counts": false, "zones": []string{}},
			},
		})
	case path == "detection/retry":
		write(http.StatusOK, map[string]any{"status": f.status()})
	default:
		write(http.StatusNotFound, map[string]any{"error": "not found", "code": "not_found"})
	}
}

func (f *networkCamerasDetectionFixture) snapshot(read func(*networkCamerasDetectionFixture) any) any {
	f.mu.Lock()
	defer f.mu.Unlock()
	return read(f)
}

func networkCamerasWaitFor(t *testing.T, what string, timeout time.Duration, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for %s", what)
}

// networkCamerasDetectionPage opens the desktop with the fake API in the browser
// time zone Europe/Berlin, so the UTC server zone hint is deterministic.
func networkCamerasDetectionPage(t *testing.T, fixture *networkCamerasDetectionFixture) *rod.Page {
	t.Helper()
	page := desktopAuditBrowserWithAPI(t, map[string]http.Handler{"/api/desktop/integrations/go2rtc/": fixture})
	if err := (proto.EmulationSetTimezoneOverride{TimezoneID: "Europe/Berlin"}).Call(page); err != nil {
		t.Fatal(err)
	}
	return page
}

func networkCamerasOpenDetection(t *testing.T, fixture *networkCamerasDetectionFixture) *rod.Page {
	t.Helper()
	page := networkCamerasDetectionPage(t, fixture)
	page.MustEval(`()=>{localStorage.removeItem('aurago.desktop.network-cameras');document.documentElement.lang='de';auditTest.openApp('network-cameras')}`)
	page.MustWait(`()=>!!document.querySelector('.nc-segmented [data-mode="detections"]')`)
	return page
}

func ncdKeys(value any) string {
	object, _ := value.(map[string]any)
	keys := make([]string, 0, len(object))
	for key := range object {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return strings.Join(keys, ",")
}

// TestDesktopNetworkCamerasDetectionBrowser drives the detection mode through
// the real desktop shell against a fake detection API: timeline, detail
// navigation, arming, polling with pause while hidden, delete and settings.
func TestDesktopNetworkCamerasDetectionBrowser(t *testing.T) {
	fixture := newNetworkCamerasDetectionFixture(t, true)
	page := networkCamerasOpenDetection(t, fixture)
	frontDoor, garage, polled := fmt.Sprint(ncdEventFrontDoor), fmt.Sprint(ncdEventGarage), fmt.Sprint(ncdEventPolled)

	page.MustEval(`()=>document.querySelector('.nc-segmented [data-mode="detections"]').click()`)
	page.MustWait(`()=>document.querySelectorAll('.ncd-row').length===2&&document.querySelectorAll('.ncd-arm-row').length===1`)
	if mode := page.MustEval(`()=>JSON.parse(localStorage.getItem('aurago.desktop.network-cameras')).mode`).Str(); mode != "detections" {
		t.Fatalf("stored mode = %q", mode)
	}
	if got := page.MustEval(`()=>[document.querySelectorAll('.ncd-chip').length,document.querySelector('[data-arm="schedule"]').getAttribute('aria-pressed'),document.querySelector('.ncd-detector-text').textContent].join('|')`).Str(); !strings.HasPrefix(got, "2|true|D-FINE-N · GPU") {
		t.Fatalf("status strip = %q", got)
	}
	// The schedule runs in the server zone (UTC), the browser in Europe/Berlin.
	if zone := page.MustEval(`()=>document.querySelector('.ncd-strip .ncd-server-zone')?.textContent||''`).Str(); !strings.Contains(zone, "(UTC)") {
		t.Fatalf("server zone hint in the strip = %q", zone)
	}

	// Detail with SVG boxes and keyboard navigation.
	page.MustEval(`id=>document.querySelector('.ncd-row[data-event="'+id+'"]').click()`, frontDoor)
	page.MustWait(`id=>document.querySelector('.ncd-frame img')?.getAttribute('src').endsWith('/detection/images/'+id)&&document.querySelectorAll('.ncd-frame svg rect').length===2`, fmt.Sprint(ncdFrameID(ncdEventFrontDoor, 1)))
	page.MustEval(`()=>{const d=document.querySelector('.ncd-detail');d.focus();d.dispatchEvent(new KeyboardEvent('keydown',{key:'ArrowRight',bubbles:true}))}`)
	page.MustWait(`id=>document.querySelector('.ncd-frame img').getAttribute('src').endsWith('/detection/images/'+id)&&document.querySelectorAll('.ncd-frame svg rect').length===1`, fmt.Sprint(ncdFrameID(ncdEventFrontDoor, 2)))
	page.MustEval(`()=>document.activeElement.dispatchEvent(new KeyboardEvent('keydown',{key:'ArrowLeft',bubbles:true}))`)
	page.MustWait(`id=>document.querySelector('.ncd-frame img').getAttribute('src').endsWith('/detection/images/'+id)`, fmt.Sprint(ncdFrameID(ncdEventFrontDoor, 1)))
	if n := page.MustEval(`()=>document.querySelectorAll('.ncd-alert-list li').length`).Int(); n != 4 {
		t.Fatalf("alert results = %d, want 4", n)
	}

	// Arming posts the mode and updates the strip.
	page.MustEval(`()=>document.querySelector('[data-arm="on"]').click()`)
	networkCamerasWaitFor(t, "arm POST", 5*time.Second, func() bool {
		return fixture.snapshot(func(f *networkCamerasDetectionFixture) any {
			return len(f.armBodies) == 1 && f.armBodies[0]["mode"] == "on" && len(f.armBodies[0]) == 1
		}).(bool)
	})
	page.MustWait(`()=>document.querySelector('[data-arm="on"]').getAttribute('aria-pressed')==='true'`)

	// Polling picks up a new event.
	fixture.snapshot(func(f *networkCamerasDetectionFixture) any { f.releaseEvent = true; return nil })
	page.Timeout(12*time.Second).MustWait(`id=>!!document.querySelector('.ncd-row[data-event="'+id+'"]')`, polled)

	// Hidden windows stop polling; showing them again resumes it.
	page.MustEval(`()=>{document.querySelector('.vd-window[data-app-id="network-cameras"]').style.display='none'}`)
	time.Sleep(500 * time.Millisecond)
	before := fixture.snapshot(func(f *networkCamerasDetectionFixture) any { return len(f.stateSince) }).(int)
	time.Sleep(6500 * time.Millisecond)
	if after := fixture.snapshot(func(f *networkCamerasDetectionFixture) any { return len(f.stateSince) }).(int); after != before {
		t.Fatalf("hidden window kept polling: %d -> %d state requests", before, after)
	}
	page.MustEval(`()=>{document.querySelector('.vd-window[data-app-id="network-cameras"]').style.display=''}`)
	networkCamerasWaitFor(t, "polling to resume", 8*time.Second, func() bool {
		return fixture.snapshot(func(f *networkCamerasDetectionFixture) any { return len(f.stateSince) }).(int) > before
	})
	page.MustWait(`()=>document.querySelectorAll('.ncd-row').length===3`)

	// An event deleted elsewhere (the change feed has no tombstones): opening it answers 404,
	// the row disappears, the detail closes and a notice says why.
	fixture.snapshot(func(f *networkCamerasDetectionFixture) any { f.deletedEvents[garage] = true; return nil })
	page.MustEval(`id=>document.querySelector('.ncd-row[data-event="'+id+'"]').click()`, garage)
	page.MustWait(`id=>!document.querySelector('.ncd-row[data-event="'+id+'"]')&&!document.querySelector('.ncd-frame')&&[...document.querySelectorAll('.vd-toast-message')].some(n=>n.textContent.includes('gibt es nicht mehr'))`, garage)

	// Delete through the desktop confirm dialog.
	page.MustEval(`id=>document.querySelector('.ncd-row[data-event="'+id+'"]').click()`, frontDoor)
	page.MustWait(`()=>!!document.querySelector('[data-ncd-action="delete-event"]')`)
	page.MustEval(`()=>document.querySelector('[data-ncd-action="delete-event"]').click()`)
	page.MustWait(`()=>!!document.querySelector('.vd-modal .vd-button-primary')`)
	page.MustEval(`()=>document.querySelector('.vd-modal .vd-button-primary').click()`)
	networkCamerasWaitFor(t, "DELETE events/"+frontDoor, 5*time.Second, func() bool {
		return fixture.snapshot(func(f *networkCamerasDetectionFixture) any { return strings.Join(f.deleted, ",") == frontDoor }).(bool)
	})
	page.MustWait(`id=>!document.querySelector('.ncd-row[data-event="'+id+'"]')`, frontDoor)

	// Global settings dialog sends the complete Go2RTCDetectionConfig.
	page.MustEval(`()=>document.querySelector('[data-ncd-action="settings"]').click()`)
	page.MustWait(`()=>!!document.querySelector('[data-ncd-dialog]')`)
	if page.MustEval(`()=>!!document.querySelector('.ncd-gpu-hint')`).Bool() {
		t.Fatal("GPU hint shown without a GPU fallback reason")
	}
	if zone := page.MustEval(`()=>document.querySelector('[data-ncd-dialog] .ncd-server-zone')?.textContent||''`).Str(); !strings.Contains(zone, "(UTC)") {
		t.Fatalf("server zone hint in the schedule editor = %q", zone)
	}
	page.MustEval(`()=>{
		const dialog=document.querySelector('[data-ncd-dialog]');
		dialog.querySelector('input[value="accurate"]').click();
		const backend=dialog.querySelector('[data-set-field="backend"]');backend.value='cpu';backend.dispatchEvent(new Event('change',{bubbles:true}));
		const retention=dialog.querySelector('[data-set-field="retention_days"]');retention.value='7';retention.dispatchEvent(new Event('input',{bubbles:true}));
		dialog.querySelector('[data-window-day="0:sat"]').click();
		dialog.querySelector('[data-window-day="0:mon"]').click();
		dialog.querySelector('[data-ncd-action="add-window"]').click();
	}`)
	page.MustWait(`()=>!!document.querySelector('[data-window="1"]')`)
	page.MustEval(`()=>{
		const dialog=document.querySelector('[data-ncd-dialog]');
		dialog.querySelector('[data-window-day="1:sat"]').click();
		dialog.querySelector('[data-window-day="1:sun"]').click();
		const from=dialog.querySelector('[data-window-from="1"]');from.value='22:30';from.dispatchEvent(new Event('input',{bubbles:true}));
		const to=dialog.querySelector('[data-window-to="1"]');to.value='06:15';to.dispatchEvent(new Event('input',{bubbles:true}));
		dialog.querySelector('[data-ncd-action="save-settings"]').click();
	}`)
	networkCamerasWaitFor(t, "PUT settings", 5*time.Second, func() bool {
		return fixture.snapshot(func(f *networkCamerasDetectionFixture) any { return f.settingsBody != nil }).(bool)
	})
	got, _ := json.Marshal(fixture.snapshot(func(f *networkCamerasDetectionFixture) any { return f.settingsBody }))
	want := `{"backend":"cpu","enabled":true,"max_storage_mb":2048,"model":"accurate","retention_days":7,"schedule":[{"days":["mon","sun"],"from":"08:00","to":"12:00"},{"days":["mon","tue","wed","thu","fri"],"from":"22:30","to":"06:15"}]}`
	if string(got) != want {
		t.Fatalf("settings body = %s\nwant %s", got, want)
	}
	page.MustWait(`()=>!document.querySelector('[data-ncd-dialog]')`)

	// Themes and phone width: no horizontal overflow, readable text.
	dir := filepath.Join("..", "reports", "network-cameras-detection")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	page.MustEval(`id=>document.querySelector('.ncd-row[data-event="'+id+'"]').click()`, polled)
	page.MustWait(`()=>!!document.querySelector('.ncd-frame img')`)
	for _, width := range []int{1280, 420} {
		page.MustSetViewport(width, 900, 1, false)
		if width == 420 {
			page.MustEval(`()=>{const a=auditTest;a.applyWindowSnap(a.state.windows.get(a.state.activeWindowId).element,'maximize')}`)
		}
		for _, combo := range []struct{ theme, mode string }{{"standard", "dark"}, {"fruity", "light"}, {"fruity", "dark"}} {
			name := fmt.Sprintf("%s-%s-%d", combo.theme, combo.mode, width)
			page.MustEval(`(theme,mode)=>{const a=auditTest;a.state.bootstrap.settings['appearance.theme']=theme;a.state.bootstrap.settings['appearance.fruity_mode']=mode;document.documentElement.dataset.theme=mode;a.applyDesktopSettings();a.refreshSpacesForViewport();}`, combo.theme, combo.mode)
			page.MustWait(`()=>!!document.querySelector('.ncd-root')`)
			if page.MustEval(`()=>{const r=document.querySelector('.ncd-root');return r.scrollWidth>r.clientWidth+1}`).Bool() {
				t.Fatalf("horizontal overflow in %s", name)
			}
			if low := page.MustEval(localWikipediaLowContrastJS, []string{".ncd-row-main strong", ".ncd-row-meta", ".ncd-detector-text", ".ncd-arm-summary", ".ncd-meta dd"}).Str(); low != "" {
				t.Fatalf("text contrast below 4.5:1 in %s: %s", name, low)
			}
			page.MustScreenshot(filepath.Join(dir, name+".png"))
		}
	}
	if errs := page.MustEval(`()=>fixtureErrors.join('\n')`).Str(); errs != "" {
		t.Fatalf("page errors: %s", errs)
	}
}

// TestDesktopNetworkCamerasZoneEditorBrowser draws a zone with pointer events,
// renames it, drags a vertex, runs the detection test and checks the PATCH body.
func TestDesktopNetworkCamerasZoneEditorBrowser(t *testing.T) {
	fixture := newNetworkCamerasDetectionFixture(t, true)
	page := networkCamerasOpenDetection(t, fixture)

	page.MustWait(`()=>!!document.querySelector('[data-edit="front-door"]')`)
	page.MustEval(`()=>document.querySelector('[data-edit="front-door"]').click()`)
	page.MustWait(`()=>!!document.querySelector('[data-modal-tab="detection"]')`)
	page.MustEval(`()=>document.querySelector('[data-modal-tab="detection"]').click()`)
	page.MustWait(`()=>{const s=document.querySelector('[data-zone-stage]');return s&&s.getBoundingClientRect().width>100}`)
	if hint := page.MustEval(`()=>document.querySelector('.ncd-restart-hint')?.textContent||''`).Str(); !strings.Contains(hint, "Livebilder") {
		t.Fatalf("restart hint next to the detection switch = %q", hint)
	}
	page.MustEval(`()=>document.querySelector('[data-cam-field="enabled"]').click()`)

	// Draw a four-point zone and close it with Enter.
	page.MustEval(`()=>{
		const stage=document.querySelector('[data-zone-stage]');
		const svg=stage.querySelector('svg');
		const r=svg.getBoundingClientRect();
		const down=(x,y)=>stage.dispatchEvent(new PointerEvent('pointerdown',{bubbles:true,clientX:r.left+x*r.width,clientY:r.top+y*r.height,pointerId:1,pointerType:'mouse',button:0,isPrimary:true}));
		down(0.1,0.5);down(0.9,0.5);down(0.9,0.95);down(0.1,0.95);
		stage.dispatchEvent(new KeyboardEvent('keydown',{key:'Enter',bubbles:true}));
	}`)
	page.MustWait(`()=>document.querySelectorAll('.ncd-zone-item').length===1&&document.querySelectorAll('.ncd-vertex').length===4`)

	// Escape discards an unfinished zone.
	page.MustEval(`()=>{
		const stage=document.querySelector('[data-zone-stage]');
		const r=stage.querySelector('svg').getBoundingClientRect();
		stage.dispatchEvent(new PointerEvent('pointerdown',{bubbles:true,clientX:r.left+0.3*r.width,clientY:r.top+0.2*r.height,pointerId:1,pointerType:'mouse',button:0,isPrimary:true}));
		stage.dispatchEvent(new KeyboardEvent('keydown',{key:'Escape',bubbles:true}));
	}`)
	page.MustWait(`()=>!document.querySelector('.ncd-drawing-point')&&document.querySelectorAll('.ncd-zone-item').length===1&&!!document.querySelector('.nc-modal')`)

	// Starting a zone cleared the selection: rename the zone, select it
	// through the list and drag its first point.
	page.MustEval(`()=>{
		const input=document.querySelector('[data-zone-name="0"]');input.value='Einfahrt';input.dispatchEvent(new Event('input',{bubbles:true}));
		document.querySelector('[data-zone-select="0"]').click();
	}`)
	page.MustWait(`()=>document.querySelectorAll('.ncd-vertex').length===4&&document.querySelector('[data-zone-select="0"]').getAttribute('aria-pressed')==='true'`)
	page.MustEval(`()=>{
		const stage=document.querySelector('[data-zone-stage]');
		const r=stage.querySelector('svg').getBoundingClientRect();
		const handle=stage.querySelector('.ncd-vertex[data-point="0"]');
		const h=handle.getBoundingClientRect();
		handle.dispatchEvent(new PointerEvent('pointerdown',{bubbles:true,clientX:h.left+h.width/2,clientY:h.top+h.height/2,pointerId:1,pointerType:'mouse',button:0,isPrimary:true}));
		stage.dispatchEvent(new PointerEvent('pointermove',{bubbles:true,clientX:r.left+0.2*r.width,clientY:r.top+0.55*r.height,pointerId:1,pointerType:'mouse',isPrimary:true}));
		stage.dispatchEvent(new PointerEvent('pointerup',{bubbles:true,clientX:r.left+0.2*r.width,clientY:r.top+0.55*r.height,pointerId:1,pointerType:'mouse',isPrimary:true}));
	}`)

	// The test button shows counted and ignored boxes.
	page.MustEval(`()=>document.querySelector('[data-zone-action="test"]').click()`)
	networkCamerasWaitFor(t, "POST test", 5*time.Second, func() bool {
		return fixture.snapshot(func(f *networkCamerasDetectionFixture) any { return f.testBody != nil }).(bool)
	})
	page.MustWait(`()=>document.querySelectorAll('.ncd-zone-stage .ncd-box.is-counted').length===1&&document.querySelectorAll('.ncd-zone-stage .ncd-box.is-ignored').length===1`)
	testBody, _ := json.Marshal(fixture.snapshot(func(f *networkCamerasDetectionFixture) any { return f.testBody }))
	if !strings.Contains(string(testBody), `"stream_id":"front-door"`) || !strings.Contains(string(testBody), `"name":"Einfahrt"`) {
		t.Fatalf("test body = %s", testBody)
	}

	shots := filepath.Join("..", "reports", "network-cameras-detection")
	if err := os.MkdirAll(shots, 0o755); err != nil {
		t.Fatal(err)
	}
	page.MustScreenshot(filepath.Join(shots, "zone-editor.png"))

	// Saving sends the detection object with the zone, with exactly the C5 keys
	// (PATCH merges per key and rejects unknown ones).
	page.MustEval(`()=>document.querySelector('[data-modal-action="save"]').click()`)
	networkCamerasWaitFor(t, "PATCH streams/front-door", 5*time.Second, func() bool {
		return fixture.snapshot(func(f *networkCamerasDetectionFixture) any { return f.patchBody != nil }).(bool)
	})
	patch := fixture.snapshot(func(f *networkCamerasDetectionFixture) any { return f.patchBody }).(map[string]any)
	detection, _ := patch["detection"].(map[string]any)
	if detection == nil || detection["enabled"] != true || detection["interval_seconds"] != float64(2) || detection["min_score"] != 0.5 {
		t.Fatalf("PATCH detection = %#v", patch["detection"])
	}
	if keys := ncdKeys(detection); keys != "actions,cooldown_seconds,enabled,interval_seconds,min_score,zones" {
		t.Fatalf("PATCH detection keys = %s", keys)
	}
	if keys := ncdKeys(detection["actions"]); keys != "agent,agent_instruction,channel,desktop,message" {
		t.Fatalf("PATCH detection.actions keys = %s", keys)
	}
	zones, _ := detection["zones"].([]any)
	if len(zones) != 1 {
		t.Fatalf("zones = %#v", detection["zones"])
	}
	zone := zones[0].(map[string]any)
	points, _ := zone["points"].([]any)
	if zone["name"] != "Einfahrt" || len(points) != 4 {
		t.Fatalf("zone = %#v", zone)
	}
	first := points[0].([]any)
	if dx, dy := first[0].(float64)-0.2, first[1].(float64)-0.55; dx*dx+dy*dy > 0.0004 {
		t.Fatalf("dragged point = %v, want about [0.2 0.55]", first)
	}
	if patch["name"] != "Haustür" || patch["enabled"] != true {
		t.Fatalf("PATCH general fields = %#v", patch)
	}
	page.MustWait(`()=>!document.querySelector('.nc-modal')`)
	// Toasts of the app carry their text (ctx.notify gets {title, message, type}).
	page.MustWait(`()=>[...document.querySelectorAll('.vd-toast-message')].some(n=>n.textContent==='Kamera aktualisiert')`)
	if errs := page.MustEval(`()=>fixtureErrors.join('\n')`).Str(); errs != "" {
		t.Fatalf("page errors: %s", errs)
	}
}

// TestDesktopNetworkCamerasDetectionActivateBrowser checks the "detection off"
// empty state and that Activate keeps the stored schedule.
func TestDesktopNetworkCamerasDetectionActivateBrowser(t *testing.T) {
	fixture := newNetworkCamerasDetectionFixture(t, false)
	fixture.deletedEvents = map[string]bool{fmt.Sprint(ncdEventGarage): true, fmt.Sprint(ncdEventFrontDoor): true}
	page := networkCamerasOpenDetection(t, fixture)
	page.MustEval(`()=>document.querySelector('.nc-segmented [data-mode="detections"]').click()`)
	page.MustWait(`()=>!!document.querySelector('.ncd-empty [data-ncd-action="activate"]')`)
	if text := page.MustEval(`()=>document.querySelector('.ncd-empty').textContent`).Str(); !strings.Contains(text, "15 MB") || !strings.Contains(text, "25 MB") {
		t.Fatalf("off state lacks download sizes: %q", text)
	}
	page.MustEval(`()=>document.querySelector('[data-ncd-action="activate"]').click()`)
	networkCamerasWaitFor(t, "PUT settings", 5*time.Second, func() bool {
		return fixture.snapshot(func(f *networkCamerasDetectionFixture) any { return f.settingsBody != nil }).(bool)
	})
	got, _ := json.Marshal(fixture.snapshot(func(f *networkCamerasDetectionFixture) any { return f.settingsBody }))
	want := `{"backend":"auto","enabled":true,"max_storage_mb":2048,"model":"standard","retention_days":14,"schedule":[{"days":["sat","sun"],"from":"08:00","to":"12:00"}]}`
	if string(got) != want {
		t.Fatalf("activate body = %s\nwant %s", got, want)
	}
	// The status refreshes right after saving (well before the next 5 s poll), also
	// when reloadAppState remounts the mode, so "Activate" does not linger.
	page.Timeout(2500 * time.Millisecond).MustWait(`()=>!document.querySelector('[data-ncd-action="activate"]')&&document.querySelectorAll('.ncd-chip').length===2`)
	if errs := page.MustEval(`()=>fixtureErrors.join('\n')`).Str(); errs != "" {
		t.Fatalf("page errors: %s", errs)
	}
}

// TestDesktopNetworkCamerasDetectionLaunchBrowser opens events through launch
// contexts: a new window, an already open window, a notification-center click
// (context {event_id, camera_id} as shell-chrome-runtime.js keeps it) and an
// event that was deleted meanwhile.
func TestDesktopNetworkCamerasDetectionLaunchBrowser(t *testing.T) {
	fixture := newNetworkCamerasDetectionFixture(t, true)
	page := networkCamerasDetectionPage(t, fixture)
	garageCover, frontDoorCover := fmt.Sprint(ncdFrameID(ncdEventGarage, 1)), fmt.Sprint(ncdFrameID(ncdEventFrontDoor, 1))
	page.MustEval(`id=>{localStorage.removeItem('aurago.desktop.network-cameras');document.documentElement.lang='de';auditTest.openApp('network-cameras',{event_id:id,camera_id:'garage'})}`, ncdEventGarage)
	page.MustWait(`cover=>document.querySelector('[data-mode="detections"]')?.getAttribute('aria-pressed')==='true'&&document.querySelector('[data-ncd-filter="camera"]')?.value==='garage'&&(document.querySelector('.ncd-frame img')?.getAttribute('src')||'').endsWith('/detection/images/'+cover)`, garageCover)
	if mode := page.MustEval(`()=>JSON.parse(localStorage.getItem('aurago.desktop.network-cameras')).mode`).Str(); mode != "detections" {
		t.Fatalf("stored mode after a launch = %q", mode)
	}
	networkCamerasWaitFor(t, "events filtered by the launched camera", 5*time.Second, func() bool {
		return fixture.snapshot(func(f *networkCamerasDetectionFixture) any {
			for _, query := range f.eventQueries {
				if strings.Contains(query, "stream_id=garage") {
					return true
				}
			}
			return false
		}).(bool)
	})

	// A second launch reuses the open window.
	page.MustEval(`id=>auditTest.openApp('network-cameras',{event_id:id,camera_id:'front-door'})`, ncdEventFrontDoor)
	page.MustWait(`cover=>document.querySelector('[data-ncd-filter="camera"]')?.value==='front-door'&&(document.querySelector('.ncd-frame img')?.getAttribute('src')||'').endsWith('/detection/images/'+cover)`, frontDoorCover)
	if n := page.MustEval(`()=>document.querySelectorAll('.vd-window[data-app-id="network-cameras"]').length`).Int(); n != 1 {
		t.Fatalf("windows = %d, want 1", n)
	}

	// A click in the notification center opens the event of that notification.
	page.MustEval(`id=>{const a=auditTest;a.state.notificationHistory.unshift({id:'n-launch',title:'Person erkannt – Garage',message:'Garage (Tor), 08:00, 91 %',appId:'network-cameras',context:{event_id:id,camera_id:'garage'},ts:Date.now(),read:false});document.getElementById('vd-notification-button').click()}`, ncdEventGarage)
	page.MustWait(`()=>!!document.querySelector('[data-notification-id="n-launch"]')`)
	page.MustEval(`()=>document.querySelector('[data-notification-id="n-launch"]').click()`)
	page.MustWait(`cover=>document.querySelector('[data-ncd-filter="camera"]')?.value==='garage'&&(document.querySelector('.ncd-frame img')?.getAttribute('src')||'').endsWith('/detection/images/'+cover)`, garageCover)

	// A launch for an event deleted meanwhile ends with a notice instead of a broken detail.
	fixture.snapshot(func(f *networkCamerasDetectionFixture) any {
		f.deletedEvents[fmt.Sprint(ncdEventFrontDoor)] = true
		return nil
	})
	page.MustEval(`id=>auditTest.openApp('network-cameras',{event_id:id,camera_id:'front-door'})`, ncdEventFrontDoor)
	page.MustWait(`()=>document.querySelector('[data-ncd-filter="camera"]')?.value==='front-door'&&!document.querySelector('.ncd-frame')&&[...document.querySelectorAll('.vd-toast-message')].some(n=>n.textContent.includes('gibt es nicht mehr'))`)
	if errs := page.MustEval(`()=>fixtureErrors.join('\n')`).Str(); errs != "" {
		t.Fatalf("page errors: %s", errs)
	}
}
