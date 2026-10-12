package persondetect

import (
	"bytes"
	"context"
	"errors"
	"image/jpeg"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"
	"unicode/utf8"
)

type alertMessageCall struct {
	channel, title, body string
	photo                []byte
}

type alertRecorder struct {
	mu       sync.Mutex
	reports  map[string][]AlertResult
	messages []alertMessageCall
	prompts  []string
	toasts   []string
	missions []AlertEvent
}

func newAlertRecorder() *alertRecorder {
	return &alertRecorder{reports: map[string][]AlertResult{}}
}

func (r *alertRecorder) report(action string, result AlertResult) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.reports[action] = append(r.reports[action], result)
}

func (r *alertRecorder) sinks() AlertSinks {
	return AlertSinks{
		Message: func(_ context.Context, channel, title, body string, photo []byte) error {
			r.mu.Lock()
			defer r.mu.Unlock()
			r.messages = append(r.messages, alertMessageCall{channel: channel, title: title, body: body, photo: photo})
			return nil
		},
		Agent: func(prompt string) error {
			r.mu.Lock()
			defer r.mu.Unlock()
			r.prompts = append(r.prompts, prompt)
			return nil
		},
		Desktop: func(ev AlertEvent, title, message string) error {
			r.mu.Lock()
			defer r.mu.Unlock()
			r.toasts = append(r.toasts, title+"|"+message)
			return nil
		},
		Mission: func(ev AlertEvent) {
			r.mu.Lock()
			defer r.mu.Unlock()
			r.missions = append(r.missions, ev)
		},
	}
}

func (r *alertRecorder) result(t *testing.T, action string) AlertResult {
	t.Helper()
	r.mu.Lock()
	defer r.mu.Unlock()
	if n := len(r.reports[action]); n != 1 {
		t.Fatalf("%s reported %d times, want once: %+v", action, n, r.reports)
	}
	return r.reports[action][0]
}

// germanAlertTexts answers German only, like i18n.T with a German UI; every other language
// gets the key back, which the dispatcher must treat as "no text".
func germanAlertTexts(lang, key string) string {
	if lang != "de" {
		return key
	}
	return map[string]string{
		AlertTextTitle:       "Person erkannt – {camera}",
		AlertTextMessage:     "{camera} ({zones}), {time} Uhr, {score} %",
		AlertTextAllZones:    "ganzes Bild",
		AlertTextAgentPrompt: "Ereignis {event_id}: prüfe das Bild mit analyze_detection_event.",
	}[key]
}

func alertTestEvent() AlertEvent {
	return AlertEvent{
		EventID:    42,
		CameraID:   "garden",
		CameraName: "Garten",
		Zones:      []string{"Einfahrt", "Tor"},
		Score:      0.912,
		Persons:    2,
		Armed:      true,
		StartedAt:  time.Date(2026, 10, 11, 21, 14, 3, 0, time.Local),
		Actions:    Actions{Message: true, Channel: "telegram", Agent: true, AgentInstruction: "Nachts immer melden.", Desktop: true},
		Fire:       FireSet{Message: true, Agent: true, Desktop: true, Mission: true},
		Language:   "de",
	}
}

func writeAlertCover(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "42_1.jpg")
	if err := os.WriteFile(path, annotateTestJPEG(t, 200, 100), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestDispatchReportsEveryFiredActionOnce(t *testing.T) {
	rec := newAlertRecorder()
	d := NewAlertDispatcher(rec.sinks(), germanAlertTexts, nil)
	d.Dispatch(alertTestEvent(), rec.report)
	d.Wait()
	for _, action := range []string{ActionMessage, ActionAgent, ActionDesktop, ActionMission} {
		if got := rec.result(t, action); got.Status != "sent" || got.Detail != "" {
			t.Fatalf("%s = %+v, want sent", action, got)
		}
	}
	if len(rec.missions) != 1 || rec.missions[0].EventID != 42 || rec.missions[0].CameraID != "garden" {
		t.Fatalf("mission sink got %+v", rec.missions)
	}
}

func TestDispatchFiresOnlyTheFireSet(t *testing.T) {
	rec := newAlertRecorder()
	d := NewAlertDispatcher(rec.sinks(), germanAlertTexts, nil)
	ev := alertTestEvent()
	ev.Armed = false
	ev.Fire = FireSet{Mission: true}
	d.Dispatch(ev, rec.report)
	d.Wait()
	if len(rec.reports) != 1 || len(rec.messages)+len(rec.prompts)+len(rec.toasts) != 0 {
		t.Fatalf("reports %+v, messages %d, prompts %d, toasts %d", rec.reports, len(rec.messages), len(rec.prompts), len(rec.toasts))
	}
	if got := rec.result(t, ActionMission); got.Status != "sent" {
		t.Fatalf("mission = %+v", got)
	}
}

func TestDispatchSkipsMissingSinks(t *testing.T) {
	rec := newAlertRecorder()
	d := NewAlertDispatcher(AlertSinks{}, nil, nil)
	d.Dispatch(alertTestEvent(), rec.report)
	d.Wait()
	want := map[string]string{
		ActionMessage: "channel unavailable",
		ActionAgent:   "agent unavailable",
		ActionDesktop: "desktop unavailable",
		ActionMission: "missions unavailable",
	}
	for action, detail := range want {
		if got := rec.result(t, action); got.Status != "skipped" || got.Detail != detail {
			t.Fatalf("%s = %+v, want skipped %q", action, got, detail)
		}
	}
}

func TestDispatchLocalizesMessageAndToast(t *testing.T) {
	rec := newAlertRecorder()
	d := NewAlertDispatcher(rec.sinks(), germanAlertTexts, nil)
	d.Dispatch(alertTestEvent(), rec.report)
	d.Wait()
	if len(rec.messages) != 1 {
		t.Fatalf("messages = %+v", rec.messages)
	}
	m := rec.messages[0]
	if m.channel != "telegram" || m.title != "Person erkannt – Garten" || m.body != "Garten (Einfahrt, Tor), 21:14 Uhr, 91 %" {
		t.Fatalf("message = %q / %q / %q", m.channel, m.title, m.body)
	}
	if len(rec.toasts) != 1 || rec.toasts[0] != m.title+"|"+m.body {
		t.Fatalf("toasts = %q", rec.toasts)
	}

	// Without zones the "whole picture" text fills {zones}; an unknown language falls back
	// to the built-in English texts.
	rec = newAlertRecorder()
	d = NewAlertDispatcher(rec.sinks(), germanAlertTexts, nil)
	ev := alertTestEvent()
	ev.Zones = nil
	ev.Language = "fr"
	ev.Actions.Channel = ""
	d.Dispatch(ev, rec.report)
	d.Wait()
	m = rec.messages[0]
	if m.channel != "all" || m.title != "Person detected – Garten" || m.body != "Garten (whole picture), 21:14, 91%" {
		t.Fatalf("fallback message = %q / %q / %q", m.channel, m.title, m.body)
	}
}

// A camera name that looks like a placeholder is filled in once, never expanded again.
func TestDispatchFillsPlaceholdersInOnePass(t *testing.T) {
	rec := newAlertRecorder()
	d := NewAlertDispatcher(rec.sinks(), germanAlertTexts, nil)
	ev := alertTestEvent()
	ev.CameraName = "{zones}"
	ev.Fire = FireSet{Desktop: true}
	d.Dispatch(ev, rec.report)
	d.Wait()
	if len(rec.toasts) != 1 || !strings.HasPrefix(rec.toasts[0], "Person erkannt – {zones}|{zones} (Einfahrt, Tor)") {
		t.Fatalf("toast = %q", rec.toasts)
	}
}

func TestDispatchSendsAnnotatedPhotoOnlyToPhotoChannels(t *testing.T) {
	cover := writeAlertCover(t)
	raw, _ := os.ReadFile(cover)
	for _, channel := range []string{"telegram", "discord", "all", "ntfy", "push"} {
		rec := newAlertRecorder()
		d := NewAlertDispatcher(rec.sinks(), germanAlertTexts, nil)
		ev := alertTestEvent()
		ev.Fire = FireSet{Message: true}
		ev.Actions.Channel = channel
		ev.CoverPath = cover
		ev.CoverBoxes = []Box{{X1: 0.25, Y1: 0.25, X2: 0.75, Y2: 0.75, Score: 0.91}}
		d.Dispatch(ev, rec.report)
		d.Wait()
		photo := rec.messages[0].photo
		wantPhoto := channel == "telegram" || channel == "discord" || channel == "all"
		if (photo != nil) != wantPhoto {
			t.Fatalf("%s: photo present = %v, want %v", channel, photo != nil, wantPhoto)
		}
		if photo == nil {
			continue
		}
		if bytes.Equal(photo, raw) {
			t.Fatalf("%s: the photo must carry the drawn boxes", channel)
		}
		if cfg, err := jpeg.DecodeConfig(bytes.NewReader(photo)); err != nil || cfg.Width != 200 || cfg.Height != 100 {
			t.Fatalf("%s: photo config = %+v, %v", channel, cfg, err)
		}
	}
	// A missing cover file still sends the text.
	rec := newAlertRecorder()
	d := NewAlertDispatcher(rec.sinks(), germanAlertTexts, nil)
	ev := alertTestEvent()
	ev.Fire = FireSet{Message: true}
	ev.CoverPath = filepath.Join(t.TempDir(), "gone.jpg")
	d.Dispatch(ev, rec.report)
	d.Wait()
	if rec.messages[0].photo != nil || rec.result(t, ActionMessage).Status != "sent" {
		t.Fatalf("missing cover: %+v %+v", rec.messages, rec.reports)
	}
}

func TestDispatchAgentPromptIsolatesCameraFacts(t *testing.T) {
	rec := newAlertRecorder()
	d := NewAlertDispatcher(rec.sinks(), germanAlertTexts, nil)
	ev := alertTestEvent()
	ev.CameraName = "<b>Garten</b>"
	ev.Fire = FireSet{Agent: true}
	d.Dispatch(ev, rec.report)
	d.Wait()
	if len(rec.prompts) != 1 {
		t.Fatalf("prompts = %q", rec.prompts)
	}
	p := rec.prompts[0]
	for _, want := range []string{
		"Ereignis 42: prüfe das Bild mit analyze_detection_event.",
		"<external_data>",
		"Event: 42",
		"Camera: &lt;b&gt;Garten&lt;/b&gt; (garden)",
		"Zones: Einfahrt, Tor",
		"Time: 2026-10-11 21:14",
		"Score: 91 %",
		"Persons: 2",
		"Armed: yes",
		"Configured instruction for this camera from the user:\nNachts immer melden.",
	} {
		if !strings.Contains(p, want) {
			t.Fatalf("prompt misses %q:\n%s", want, p)
		}
	}
	if strings.Contains(p, "<b>Garten</b>") {
		t.Fatalf("camera name reached the prompt unescaped:\n%s", p)
	}
	if strings.Index(p, "</external_data>") > strings.Index(p, "Configured instruction") {
		t.Fatalf("the user's instruction must follow the isolated facts:\n%s", p)
	}

	// A blank instruction adds no instruction block; a long one is bounded to 500 runes.
	for _, instruction := range []string{"   ", strings.Repeat("ä", 900)} {
		rec = newAlertRecorder()
		d = NewAlertDispatcher(rec.sinks(), germanAlertTexts, nil)
		ev.Actions.AgentInstruction = instruction
		d.Dispatch(ev, rec.report)
		d.Wait()
		p = rec.prompts[0]
		_, block, found := strings.Cut(p, "Configured instruction for this camera from the user:\n")
		if strings.TrimSpace(instruction) == "" {
			if found {
				t.Fatalf("blank instruction produced a block:\n%s", p)
			}
			continue
		}
		if !found || utf8.RuneCountInString(block) != 500 {
			t.Fatalf("instruction block has %d runes, want 500", utf8.RuneCountInString(block))
		}
	}
}

func TestIsolateAlertFactsFormat(t *testing.T) {
	if got, want := isolateAlertFacts(`a<b & "c"`), "<external_data>\na&lt;b &amp; &#34;c&#34;\n</external_data>"; got != want {
		t.Fatalf("isolateAlertFacts = %q, want %q", got, want)
	}
	if got := isolateAlertFacts(""); got != "" {
		t.Fatalf("empty input = %q", got)
	}
}

func TestDispatchMapsSinkErrors(t *testing.T) {
	rec := newAlertRecorder()
	sinks := rec.sinks()
	sinks.Message = func(context.Context, string, string, string, []byte) error {
		return errors.New(strings.Repeat("x", 500))
	}
	sinks.Agent = func(string) error { return ErrAgentQueueFull }
	sinks.Desktop = func(AlertEvent, string, string) error { return Skipped("desktop unavailable") }
	d := NewAlertDispatcher(sinks, germanAlertTexts, nil)
	d.Dispatch(alertTestEvent(), rec.report)
	d.Wait()
	if got := rec.result(t, ActionMessage); got.Status != "failed" || utf8.RuneCountInString(got.Detail) != 200 {
		t.Fatalf("message = %+v (detail %d runes)", got, utf8.RuneCountInString(got.Detail))
	}
	if got := rec.result(t, ActionAgent); got != (AlertResult{Status: "skipped", Detail: "queue full"}) {
		t.Fatalf("agent = %+v", got)
	}
	if got := rec.result(t, ActionDesktop); got != (AlertResult{Status: "skipped", Detail: "desktop unavailable"}) {
		t.Fatalf("desktop = %+v", got)
	}
	if got := rec.result(t, ActionMission); got.Status != "sent" {
		t.Fatalf("mission = %+v", got)
	}
}

func TestDispatchMessageTimesOut(t *testing.T) {
	block := make(chan struct{})
	t.Cleanup(func() { close(block) })
	rec := newAlertRecorder()
	sinks := rec.sinks()
	sinks.Message = func(context.Context, string, string, string, []byte) error {
		<-block
		return nil
	}
	d := NewAlertDispatcher(sinks, germanAlertTexts, nil)
	d.messageTimeout = 50 * time.Millisecond
	ev := alertTestEvent()
	ev.Fire = FireSet{Message: true}
	started := time.Now()
	d.Dispatch(ev, rec.report)
	d.Wait()
	if got := rec.result(t, ActionMessage); got != (AlertResult{Status: "failed", Detail: "timeout"}) {
		t.Fatalf("message = %+v", got)
	}
	if elapsed := time.Since(started); elapsed > 5*time.Second {
		t.Fatalf("timeout took %v", elapsed)
	}
}

func TestDispatchDoesNotBlockAndSurvivesPanics(t *testing.T) {
	block := make(chan struct{})
	rec := newAlertRecorder()
	sinks := rec.sinks()
	sinks.Agent = func(string) error {
		<-block
		return nil
	}
	sinks.Desktop = func(AlertEvent, string, string) error { panic("toast exploded") }
	d := NewAlertDispatcher(sinks, germanAlertTexts, nil)
	started := time.Now()
	d.Dispatch(alertTestEvent(), rec.report)
	if elapsed := time.Since(started); elapsed > 100*time.Millisecond {
		t.Fatalf("Dispatch blocked for %v", elapsed)
	}
	close(block)
	d.Wait()
	if got := rec.result(t, ActionDesktop); got != (AlertResult{Status: "failed", Detail: "internal error"}) {
		t.Fatalf("panicking desktop sink = %+v", got)
	}
	for _, action := range []string{ActionMessage, ActionAgent, ActionMission} {
		if got := rec.result(t, action); got.Status != "sent" {
			t.Fatalf("%s = %+v", action, got)
		}
	}
}

// Adaptation: the service calls Dispatch from one storage goroutine per camera, and Wait
// may run meanwhile. Every report must arrive once, and a Wait next to Dispatch must
// return. (A sync.WaitGroup gaining work from zero during a Wait is a misuse the race
// detector reports; the in-flight counter allows it.)
func TestDispatchIsSafeForConcurrentCameras(t *testing.T) {
	rec := newAlertRecorder()
	d := NewAlertDispatcher(rec.sinks(), germanAlertTexts, nil)
	const cameras, events = 8, 25
	stop := make(chan struct{})
	waiterDone := make(chan struct{})
	go func() {
		defer close(waiterDone)
		for {
			select {
			case <-stop:
				return
			default:
				d.Wait()
				runtime.Gosched()
			}
		}
	}()
	var wg sync.WaitGroup
	for c := range cameras {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range events {
				ev := alertTestEvent()
				ev.EventID = int64(c*events + i + 1)
				d.Dispatch(ev, rec.report)
			}
		}()
	}
	wg.Wait()
	d.Wait()
	close(stop)
	<-waiterDone
	rec.mu.Lock()
	defer rec.mu.Unlock()
	for _, action := range []string{ActionMessage, ActionAgent, ActionDesktop, ActionMission} {
		if n := len(rec.reports[action]); n != cameras*events {
			t.Fatalf("%s reported %d times, want %d", action, n, cameras*events)
		}
		for _, r := range rec.reports[action] {
			if r.Status != AlertSent {
				t.Fatalf("%s = %+v", action, r)
			}
		}
	}
	seen := map[int64]bool{}
	for _, ev := range rec.missions {
		seen[ev.EventID] = true
	}
	if len(seen) != cameras*events {
		t.Fatalf("mission sink saw %d distinct events, want %d", len(seen), cameras*events)
	}
}
