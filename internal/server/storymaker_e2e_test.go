package server

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"io"
	"math"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"aurago/internal/storymaker"
	"aurago/internal/storymaker/render"
	"aurago/internal/tools"

	"github.com/go-rod/rod/lib/launcher"
)

// storymakerRealServer is storymakerAPIFixture for the tests that render for real: FFmpeg from the box, the installed
// Chrome through the real browser resolver, and no sign-in (a browser's iframe and event-stream requests carry only
// cookies, which the fixture does not have). It skips unless AURAGO_RUN_BROWSER_SMOKE=1 and a Chrome is installed.
func storymakerRealServer(t *testing.T) *Server {
	t.Helper()
	if os.Getenv("AURAGO_RUN_BROWSER_SMOKE") != "1" {
		t.Skip("set AURAGO_RUN_BROWSER_SMOKE=1 to run the Storymaker real-server tests")
	}
	chrome := os.Getenv("AURAGO_TEST_CHROME")
	if chrome == "" {
		found, ok := launcher.LookPath()
		if !ok {
			t.Skip("no Chrome or Chromium is installed")
		}
		chrome = found
	}
	if os.Getenv("AURAGO_TEST_NO_SANDBOX") == "1" {
		render.SetNoSandboxForTests(true)
		t.Cleanup(func() { render.SetNoSandboxForTests(false) })
	}
	s, _, _ := storymakerAPIFixture(t)
	s.Cfg.Auth.Enabled = false
	s.Cfg.VideoStudio.FFmpegPath = storymakerTestFFmpeg(t)
	s.Cfg.Storymaker.Render.BrowserPath = chrome
	s.storymakerBrowsers = nil // the real resolver
	return s
}

// storymakerProbe runs ffprobe (next to the configured ffmpeg) over a file and returns its JSON document.
type storymakerProbeDoc struct {
	Streams []struct {
		CodecType    string `json:"codec_type"`
		CodecName    string `json:"codec_name"`
		Width        int    `json:"width"`
		Height       int    `json:"height"`
		AvgFrameRate string `json:"avg_frame_rate"`
		NbFrames     string `json:"nb_frames"`
		PixFmt       string `json:"pix_fmt"`
		Duration     string `json:"duration"`
	} `json:"streams"`
	Format struct {
		Duration string `json:"duration"`
	} `json:"format"`
}

func storymakerFFprobe(t *testing.T, ffmpeg, file string) storymakerProbeDoc {
	t.Helper()
	exe := "ffprobe"
	if strings.HasSuffix(strings.ToLower(ffmpeg), ".exe") {
		exe = "ffprobe.exe"
	}
	probe := filepath.Join(filepath.Dir(ffmpeg), exe)
	if _, err := os.Stat(probe); err != nil {
		var lookErr error
		if probe, lookErr = exec.LookPath("ffprobe"); lookErr != nil {
			t.Skip("ffprobe is not available next to ffmpeg or on PATH")
		}
	}
	out, err := exec.Command(probe, "-v", "error", "-show_entries", "stream=codec_type,codec_name,width,height,avg_frame_rate,nb_frames,pix_fmt,duration:format=duration", "-of", "json", file).CombinedOutput()
	if err != nil {
		t.Fatalf("ffprobe %s: %v\n%s", file, err, out)
	}
	var doc storymakerProbeDoc
	if err := json.Unmarshal(out, &doc); err != nil {
		t.Fatalf("ffprobe output: %v\n%s", err, out)
	}
	return doc
}

// storymakerMeasureLUFS reads the integrated loudness of a file's first audio stream from the last I: line of FFmpeg's
// ebur128 summary. framelog=verbose prints no frame lines at the default log level.
func storymakerMeasureLUFS(t *testing.T, ffmpeg, file string) float64 {
	t.Helper()
	out, err := exec.Command(ffmpeg, "-hide_banner", "-nostdin", "-nostats", "-i", file, "-map", "0:a:0", "-af", "ebur128=framelog=verbose", "-f", "null", "-").CombinedOutput()
	if err != nil {
		t.Fatalf("ebur128 over %s: %v\n%s", file, err, out)
	}
	matches := regexp.MustCompile(`I:\s+(-?[0-9.]+) LUFS`).FindAllSubmatch(out, -1)
	if len(matches) == 0 {
		t.Fatalf("ebur128 printed no integrated loudness:\n%s", out)
	}
	lufs, err := strconv.ParseFloat(string(matches[len(matches)-1][1]), 64)
	if err != nil {
		t.Fatal(err)
	}
	return lufs
}

type storymakerSSEEvent struct {
	Name string
	Data string
}

// storymakerReadSSE opens a server-sent event stream and sends its events until the context ends.
func storymakerReadSSE(ctx context.Context, t *testing.T, url string) <-chan storymakerSSEEvent {
	t.Helper()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		t.Fatal(err)
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	if res.StatusCode != http.StatusOK || !strings.HasPrefix(res.Header.Get("Content-Type"), "text/event-stream") {
		body, _ := io.ReadAll(res.Body)
		res.Body.Close()
		t.Fatalf("event stream: %d %q %s", res.StatusCode, res.Header.Get("Content-Type"), body)
	}
	out := make(chan storymakerSSEEvent, 256)
	go func() {
		defer res.Body.Close()
		defer close(out)
		sc := bufio.NewScanner(res.Body)
		sc.Buffer(make([]byte, 0, 64<<10), 1<<20)
		var ev storymakerSSEEvent
		for sc.Scan() {
			line := sc.Text()
			switch {
			case line == "":
				if ev.Name != "" || ev.Data != "" {
					select {
					case out <- ev:
					case <-ctx.Done():
						return
					}
				}
				ev = storymakerSSEEvent{}
			case strings.HasPrefix(line, ":"):
			case strings.HasPrefix(line, "event: "):
				ev.Name = strings.TrimPrefix(line, "event: ")
			case strings.HasPrefix(line, "data: "):
				ev.Data += strings.TrimPrefix(line, "data: ")
			}
		}
	}()
	return out
}

// TestStorymakerExportEndToEnd is the API-level end-to-end test without the app: create a project, save firewall.json
// and its code scene, start an export, follow the event stream to the end and look at the MP4 in the project folder with
// ffprobe. (The render pipeline itself is covered by TestRealRender in internal/storymaker/render, which publishes to
// memory; this one goes through the HTTP handlers and the desktop workspace.)
func TestStorymakerExportEndToEnd(t *testing.T) {
	s := storymakerRealServer(t)
	mux := http.NewServeMux()
	mux.Handle(storymakerAPIBase+"/", handleStorymaker(s))
	srv := httptest.NewServer(mux)
	defer srv.Close()
	base := srv.URL + storymakerAPIBase

	call := func(method, path string, body []byte, headers map[string]string) (*http.Response, []byte) {
		t.Helper()
		req, err := http.NewRequest(method, base+path, bytes.NewReader(body))
		if err != nil {
			t.Fatal(err)
		}
		for k, v := range headers {
			req.Header.Set(k, v)
		}
		res, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer res.Body.Close()
		data, _ := io.ReadAll(res.Body)
		return res, data
	}
	lf := func(name string) []byte {
		data, err := os.ReadFile(name)
		if err != nil {
			t.Fatal(err)
		}
		return bytes.ReplaceAll(data, []byte("\r\n"), []byte("\n"))
	}

	// The status says an export can run.
	res, body := call(http.MethodGet, "/status", nil, nil)
	var status struct {
		Enabled bool `json:"enabled"`
		FFmpeg  struct {
			Ready bool `json:"ready"`
		} `json:"ffmpeg"`
		Browser struct {
			Ready  bool   `json:"ready"`
			Source string `json:"source"`
			Issue  string `json:"issue"`
		} `json:"browser"`
		Limits struct {
			MaxNarrationChars int `json:"max_narration_chars"`
		} `json:"limits"`
		CatalogVersion string `json:"catalog_version"`
	}
	if err := json.Unmarshal(body, &status); err != nil || res.StatusCode != http.StatusOK {
		t.Fatalf("status %d: %s", res.StatusCode, body)
	}
	if !status.Enabled || !status.FFmpeg.Ready || !status.Browser.Ready || status.Limits.MaxNarrationChars <= 0 || status.CatalogVersion == "" {
		t.Fatalf("the export is not possible: %s", body)
	}

	// Create, save the story and its code scene, and check what the editor would see.
	res, body = call(http.MethodPost, "/projects", []byte(`{"title":"Was ist eine Firewall?"}`), map[string]string{"Content-Type": "application/json"})
	var created struct {
		Project storymaker.Project `json:"project"`
		ETag    string             `json:"etag"`
	}
	if err := json.Unmarshal(body, &created); err != nil || res.StatusCode != http.StatusCreated {
		t.Fatalf("create %d: %s", res.StatusCode, body)
	}
	id := created.Project.ID
	res, body = call(http.MethodPut, "/projects/"+id+"/story", lf("../storymaker/testdata/stories/firewall.json"), map[string]string{"If-Match": created.ETag, "Content-Type": "application/json"})
	var saved struct {
		ETag   string             `json:"etag"`
		Issues []storymaker.Issue `json:"issues"`
	}
	if err := json.Unmarshal(body, &saved); err != nil || res.StatusCode != http.StatusOK {
		t.Fatalf("save story %d: %s", res.StatusCode, body)
	}
	res, body = call(http.MethodPut, "/projects/"+id+"/scenes/s3/code", lf("../storymaker/testdata/scenes/s3.js"), map[string]string{"If-None-Match": "*", "Content-Type": "text/javascript"})
	if res.StatusCode != http.StatusOK || !strings.Contains(string(body), `"ok":true`) {
		t.Fatalf("save code scene %d: %s", res.StatusCode, body)
	}
	res, body = call(http.MethodGet, "/projects/"+id, nil, nil)
	var current struct {
		ETag       string                       `json:"etag"`
		Issues     []storymaker.Issue           `json:"issues"`
		CodeScenes []storymaker.CodeSceneStatus `json:"code_scenes"`
	}
	if err := json.Unmarshal(body, &current); err != nil || res.StatusCode != http.StatusOK {
		t.Fatalf("get project %d: %s", res.StatusCode, body)
	}
	if storymaker.HasErrors(current.Issues) || len(current.CodeScenes) != 1 || !current.CodeScenes[0].OK {
		t.Fatalf("the saved project has errors: %s", body)
	}

	// The render gate needs stored narration.
	seedStoryNarrationForRender(t, s, id)

	// Follow the export on the event stream, opened before the render starts.
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Minute)
	defer cancel()
	events := storymakerReadSSE(ctx, t, base+"/projects/"+id+"/events")
	res, body = call(http.MethodPost, "/projects/"+id+"/render", nil, map[string]string{"If-Match": current.ETag, "Idempotency-Key": "e2e-1"})
	var accepted struct {
		Job render.Job `json:"job"`
	}
	if err := json.Unmarshal(body, &accepted); err != nil || res.StatusCode != http.StatusAccepted || accepted.Job.ID == "" {
		t.Fatalf("render %d: %s", res.StatusCode, body)
	}
	if res2, body2 := call(http.MethodPost, "/projects/"+id+"/render", nil, map[string]string{"If-Match": current.ETag, "Idempotency-Key": "e2e-1"}); res2.StatusCode != http.StatusAccepted || !strings.Contains(string(body2), accepted.Job.ID) {
		t.Fatalf("a replayed Idempotency-Key must return the same job: %d %s", res2.StatusCode, body2)
	}

	var last render.Job
	phases := map[string]bool{}
	var progress float64
	count := 0
	for ev := range events {
		if ev.Name != "job" {
			continue
		}
		var payload struct {
			Job render.Job `json:"job"`
		}
		if err := json.Unmarshal([]byte(ev.Data), &payload); err != nil {
			t.Fatalf("event data %q: %v", ev.Data, err)
		}
		if payload.Job.ID != accepted.Job.ID {
			continue
		}
		count++
		last = payload.Job
		phases[last.Phase] = true
		if last.Progress+1e-9 < progress {
			t.Errorf("progress went back from %v to %v (%s)", progress, last.Progress, last.Phase)
		}
		progress = last.Progress
		if last.Terminal() {
			break
		}
	}
	if last.Status != render.StatusSucceeded || last.Output == nil {
		t.Fatalf("the export did not succeed: %+v error %+v (events %d)", last, last.Error, count)
	}
	cancel()
	for _, want := range []string{"rendering", "encoding"} {
		if !phases[want] {
			t.Errorf("the event stream never reported the phase %q: %v", want, phases)
		}
	}
	if count < 5 || last.Progress != 1 || last.FramesTotal < 250 || last.FramesDone != last.FramesTotal {
		t.Errorf("job %+v after %d events", last, count)
	}

	// The MP4 sits in the project folder (a desktop workspace path) and has what the story asks for.
	prefix := "Documents/Storymaker/" + id + "/exports/"
	if !strings.HasPrefix(last.Output.Path, prefix) || !strings.HasSuffix(last.Output.Path, ".mp4") || !strings.Contains(last.Output.Path, accepted.Job.ID[:8]) {
		t.Fatalf("output path = %q", last.Output.Path)
	}
	mp4 := filepath.Join(s.Cfg.VirtualDesktop.WorkspaceDir, filepath.FromSlash(last.Output.Path))
	info, err := os.Stat(mp4)
	if err != nil || info.Size() != last.Output.Size || info.Size() < 10_000 {
		t.Fatalf("the MP4: %v size %d, job says %d", err, func() int64 {
			if info != nil {
				return info.Size()
			}
			return -1
		}(), last.Output.Size)
	}
	doc := storymakerFFprobe(t, s.Cfg.VideoStudio.FFmpegPath, mp4)
	var video, audio bool
	for _, st := range doc.Streams {
		switch st.CodecType {
		case "video":
			video = st.Width == 1920 && st.Height == 1080 && st.AvgFrameRate == "30/1" && st.NbFrames == strconv.Itoa(last.FramesTotal) && st.PixFmt == "yuv420p"
		case "audio":
			audio = st.CodecName == "aac"
		}
	}
	seconds, _ := strconv.ParseFloat(doc.Format.Duration, 64)
	if want := float64(last.FramesTotal) / 30; !video || !audio || seconds < want-0.2 || seconds > want+0.2 {
		t.Fatalf("ffprobe: video=%v audio=%v duration=%v (want %v) frames=%d streams=%+v", video, audio, seconds, want, last.FramesTotal, doc.Streams)
	}
	// The frame gate held: the audio ends with the video, within the 25 ms of gate.go.
	var videoSeconds, audioSeconds float64
	for _, st := range doc.Streams {
		d, _ := strconv.ParseFloat(st.Duration, 64)
		switch st.CodecType {
		case "video":
			videoSeconds = d
		case "audio":
			audioSeconds = d
		}
	}
	if videoSeconds <= 0 || audioSeconds <= 0 || math.Abs(audioSeconds-videoSeconds) >= 0.025 {
		t.Errorf("audio %v s against video %v s", audioSeconds, videoSeconds)
	}
	// The seeded speech sits near -20 dBFS, so the master gain is bound by the -16 LUFS target, not by the peak ceiling.
	// The window is the one A20 pins on a seeded story.
	if lufs := storymakerMeasureLUFS(t, s.Cfg.VideoStudio.FFmpegPath, mp4); lufs < -18.5 || lufs > -15.5 {
		t.Errorf("the export measures %v LUFS, want -18.5 to -15.5", lufs)
	}
	sidecar, err := os.ReadFile(filepath.Join(s.Cfg.VirtualDesktop.WorkspaceDir, filepath.FromSlash(prefix+accepted.Job.ID+".json")))
	if err != nil || !strings.Contains(string(sidecar), "Firewall") {
		t.Errorf("the job record next to the MP4: %v %s", err, sidecar)
	}

	// The job can be read again, listed, and a stream opened afterwards replays it.
	if res, body := call(http.MethodGet, "/jobs/"+accepted.Job.ID, nil, nil); res.StatusCode != http.StatusOK || !strings.Contains(string(body), `"succeeded"`) {
		t.Errorf("GET job %d: %s", res.StatusCode, body)
	}
	if res, body := call(http.MethodGet, "/projects/"+id+"/jobs", nil, nil); res.StatusCode != http.StatusOK || !strings.Contains(string(body), accepted.Job.ID) {
		t.Errorf("GET project jobs %d: %s", res.StatusCode, body)
	}
	replayCtx, stopReplay := context.WithTimeout(context.Background(), 20*time.Second)
	defer stopReplay()
	replay := storymakerReadSSE(replayCtx, t, base+"/projects/"+id+"/events")
	select {
	case ev, ok := <-replay:
		if !ok || ev.Name != "job" || !strings.Contains(ev.Data, accepted.Job.ID) || !strings.Contains(ev.Data, `"succeeded"`) {
			t.Errorf("the first event of a new stream = %+v", ev)
		}
	case <-replayCtx.Done():
		t.Error("a new event stream sent no snapshot of the finished job")
	}
}

// narratedE2EWAV is what the stub synth returns: a 24 kHz mono s16 WAV of ms milliseconds. A square wave of amplitude 3277
// (0.1 of full scale) has an RMS of -20 dBFS, above the -40 dBFS speech threshold. The first and last 100 ms are silent, so
// the piece has a speech span with margins, as real speech does.
func narratedE2EWAV(ms int) []byte {
	const rate = 24000
	frames := rate * ms / 1000
	pcm := make([]byte, 0, frames*2)
	for i := 0; i < frames; i++ {
		var v int16
		if i >= rate/10 && i < frames-rate/10 {
			v = 3277
			if i/27%2 == 1 { // a period of 54 samples, about 444 Hz
				v = -3277
			}
		}
		pcm = append(pcm, byte(v), byte(uint16(v)>>8))
	}
	le16 := func(v int) []byte { return []byte{byte(v), byte(v >> 8)} }
	le32 := func(v int) []byte { return []byte{byte(v), byte(v >> 8), byte(v >> 16), byte(v >> 24)} }
	wav := []byte("RIFF")
	wav = append(wav, le32(36+len(pcm))...)
	wav = append(wav, "WAVEfmt "...)
	wav = append(wav, le32(16)...)
	wav = append(wav, le16(1)...) // PCM
	wav = append(wav, le16(1)...) // mono
	wav = append(wav, le32(rate)...)
	wav = append(wav, le32(rate*2)...)
	wav = append(wav, le16(2)...)  // block align
	wav = append(wav, le16(16)...) // bits per sample
	wav = append(wav, "data"...)
	wav = append(wav, le32(len(pcm))...)
	return append(wav, pcm...)
}

// narratedE2ECues reads an SRT or VTT file. It checks that the cue times ascend and end inside the film, and returns the cue
// text without whitespace, so a line break cannot matter.
func narratedE2ECues(t *testing.T, name string, data []byte, filmMS int) string {
	t.Helper()
	ms := func(stamp string) int {
		whole, frac, ok := strings.Cut(strings.ReplaceAll(stamp, ",", "."), ".")
		millis, err := strconv.Atoi(frac)
		if !ok || err != nil {
			t.Fatalf("%s: bad time %q", name, stamp)
		}
		seconds := 0
		for _, part := range strings.Split(whole, ":") {
			n, err := strconv.Atoi(part)
			if err != nil {
				t.Fatalf("%s: bad time %q", name, stamp)
			}
			seconds = seconds*60 + n
		}
		return seconds*1000 + millis
	}
	var text strings.Builder
	cues, last := 0, -1
	lines := strings.Split(strings.ReplaceAll(string(data), "\r\n", "\n"), "\n")
	for i, line := range lines {
		from, to, timing := strings.Cut(line, " --> ")
		switch {
		case timing:
			start, end := ms(strings.TrimSpace(from)), ms(strings.Fields(to)[0])
			if start >= end || start < last || end > filmMS {
				t.Errorf("%s: cue %d to %d after a start at %d, film %d ms", name, start, end, last, filmMS)
			}
			cues++
			last = start
		case strings.TrimSpace(line) == "" || line == "WEBVTT":
		case i+1 < len(lines) && strings.Contains(lines[i+1], " --> "): // an SRT cue number
		default:
			text.WriteString(strings.Join(strings.Fields(line), ""))
		}
	}
	if cues == 0 {
		t.Fatalf("%s has no cue:\n%s", name, data)
	}
	return text.String()
}

// TestStorymakerNarratedExportEndToEnd runs the whole narrated path through the HTTP handlers: create with a language, save a
// narration, narrate with a stub synth, then render with every option, and look at each output on disk.
func TestStorymakerNarratedExportEndToEnd(t *testing.T) {
	s := storymakerRealServer(t)
	// sanotts is ready without a key (tools.TTSReady) and lists de, so the project voice resolves to it. The stub replaces
	// the synthesis itself.
	s.Cfg.TTS.Provider = "sanotts"
	// Each sentence has a width of 20 or more and more than 3 words, so SplitPieces keeps one piece per sentence: 2 calls.
	const first, second = "Anna arbeitet von zu Hause.", "Plötzlich klopft jemand an die Tür."
	narration := first + " " + second
	calls := make(chan string, 16)
	s.storymakerSynthOverride = func(_ context.Context, _ tools.TTSConfig, text string) ([]byte, string, error) {
		calls <- text
		return narratedE2EWAV(60 * len(text)), "wav", nil
	}

	mux := http.NewServeMux()
	mux.Handle(storymakerAPIBase+"/", handleStorymaker(s))
	srv := httptest.NewServer(mux)
	defer srv.Close()
	base := srv.URL + storymakerAPIBase
	call := func(method, path string, body []byte, headers map[string]string) (*http.Response, []byte) {
		t.Helper()
		req, err := http.NewRequest(method, base+path, bytes.NewReader(body))
		if err != nil {
			t.Fatal(err)
		}
		for k, v := range headers {
			req.Header.Set(k, v)
		}
		res, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer res.Body.Close()
		data, _ := io.ReadAll(res.Body)
		return res, data
	}
	waitJob := func(jobID string, limit time.Duration) render.Job {
		t.Helper()
		deadline := time.Now().Add(limit)
		for {
			res, body := call(http.MethodGet, "/jobs/"+jobID, nil, nil)
			var got struct {
				Job render.Job `json:"job"`
			}
			if err := json.Unmarshal(body, &got); err != nil || res.StatusCode != http.StatusOK {
				t.Fatalf("GET job %d: %s", res.StatusCode, body)
			}
			if got.Job.Terminal() {
				return got.Job
			}
			if time.Now().After(deadline) {
				t.Fatalf("job %s is still %s after %v", jobID, got.Job.Status, limit)
			}
			time.Sleep(250 * time.Millisecond)
		}
	}

	// Create with a language: the story has the resolved voice.
	res, body := call(http.MethodPost, "/projects", []byte(`{"title":"Narrated end to end","language":"de"}`), map[string]string{"Content-Type": "application/json"})
	var created struct {
		Project storymaker.Project `json:"project"`
		Story   storymaker.Story   `json:"story"`
		ETag    string             `json:"etag"`
	}
	if err := json.Unmarshal(body, &created); err != nil || res.StatusCode != http.StatusCreated {
		t.Fatalf("create %d: %s", res.StatusCode, body)
	}
	if v := created.Story.Voice; v == nil || v.Provider != "sanotts" || v.Language != "de" {
		t.Fatalf("resolved voice = %+v", created.Story.Voice)
	}
	id := created.Project.ID

	// Save the narration.
	created.Story.Scenes[0].Narration = narration
	storyJSON, err := storymaker.CanonicalJSON(created.Story)
	if err != nil {
		t.Fatal(err)
	}
	res, body = call(http.MethodPut, "/projects/"+id+"/story", storyJSON, map[string]string{"If-Match": created.ETag, "Content-Type": "application/json"})
	var saved struct {
		ETag   string             `json:"etag"`
		Issues []storymaker.Issue `json:"issues"`
	}
	if err := json.Unmarshal(body, &saved); err != nil || res.StatusCode != http.StatusOK || storymaker.HasErrors(saved.Issues) {
		t.Fatalf("save story %d: %s", res.StatusCode, body)
	}

	// Narrate with the stub: one call per sentence, in order, and nothing is missing afterwards.
	res, body = call(http.MethodPost, "/projects/"+id+"/narrate", nil, nil)
	var narrate struct {
		Job *render.Job `json:"job"`
	}
	if err := json.Unmarshal(body, &narrate); err != nil || res.StatusCode != http.StatusAccepted || narrate.Job == nil || narrate.Job.Kind != render.KindNarrate {
		t.Fatalf("narrate %d: %s", res.StatusCode, body)
	}
	if got := waitJob(narrate.Job.ID, 2*time.Minute); got.Status != render.StatusSucceeded {
		t.Fatalf("narrate job %+v error %+v", got, got.Error)
	}
	if len(calls) != 2 {
		t.Fatalf("the stub synth ran %d times, want 2", len(calls))
	}
	if a, b := <-calls, <-calls; a != first || b != second {
		t.Fatalf("synthesised %q and %q", a, b)
	}
	res, body = call(http.MethodGet, "/projects/"+id+"/narration", nil, nil)
	var answer storymaker.AudioAnswer
	if err := json.Unmarshal(body, &answer); err != nil || res.StatusCode != http.StatusOK {
		t.Fatalf("narration %d: %s", res.StatusCode, body)
	}
	blocks := answer.Narration["s1"].Blocks
	if len(answer.Missing) != 0 || len(blocks) != 1 || len(blocks[0].Pieces) != 2 {
		t.Fatalf("narration answer: %s", body)
	}

	// Render with every option.
	options := []byte(`{"burn_in_captions":true,"srt":true,"vtt":true,"stems":true,"quality":"high"}`)
	res, body = call(http.MethodPost, "/projects/"+id+"/render", options, map[string]string{"If-Match": saved.ETag, "Idempotency-Key": "i1-1", "Content-Type": "application/json"})
	var accepted struct {
		Job render.Job `json:"job"`
	}
	if err := json.Unmarshal(body, &accepted); err != nil || res.StatusCode != http.StatusAccepted {
		t.Fatalf("render %d: %s", res.StatusCode, body)
	}
	job := waitJob(accepted.Job.ID, 8*time.Minute)
	if job.Status != render.StatusSucceeded || job.Output == nil {
		t.Fatalf("the export did not succeed: %+v error %+v", job, job.Error)
	}
	if len(job.Warnings) != 0 || job.FramesTotal <= 0 || job.FramesDone != job.FramesTotal {
		t.Fatalf("job warnings %+v, frames %d of %d", job.Warnings, job.FramesDone, job.FramesTotal)
	}

	// The layout of frozen section 3. The film has no music and no sfx, so those two stems are omitted.
	out := job.Output
	const slug = "Narrated-end-to-end"
	exports := "Documents/Storymaker/" + id + "/exports/"
	folder := exports + slug + "-" + job.ID[:8]
	if out.Path != folder+".mp4" || out.SRT != folder+"/captions.srt" || out.VTT != folder+"/captions.vtt" {
		t.Fatalf("output = %+v", out)
	}
	if out.Stems == nil || out.Stems.Narration != folder+"/narration.wav" || out.Stems.Music != "" || out.Stems.SFX != "" {
		t.Fatalf("stems = %+v", out.Stems)
	}
	onDisk := func(desktopPath string) string {
		t.Helper()
		file := filepath.Join(s.Cfg.VirtualDesktop.WorkspaceDir, filepath.FromSlash(desktopPath))
		if info, err := os.Stat(file); err != nil || info.Size() == 0 {
			t.Fatalf("%s: %v", desktopPath, err)
		}
		return file
	}

	// ffprobe sees every frame and the AAC track in the MP4, and PCM in the stem.
	ffmpeg := s.Cfg.VideoStudio.FFmpegPath
	var video, audio, stem bool
	for _, st := range storymakerFFprobe(t, ffmpeg, onDisk(out.Path)).Streams {
		video = video || st.CodecType == "video" && st.NbFrames == strconv.Itoa(job.FramesTotal)
		audio = audio || st.CodecType == "audio" && st.CodecName == "aac"
	}
	for _, st := range storymakerFFprobe(t, ffmpeg, onDisk(out.Stems.Narration)).Streams {
		stem = stem || st.CodecType == "audio" && strings.HasPrefix(st.CodecName, "pcm_")
	}
	if !video || !audio || !stem {
		t.Fatalf("ffprobe: video with %d frames %v, AAC %v, PCM stem %v", job.FramesTotal, video, audio, stem)
	}

	// The subtitles hold the narration, inside the film. filmMS is round(frames*1000/fps), as in the report validation.
	fps := created.Story.Format.FPS
	filmMS := (job.FramesTotal*1000 + fps/2) / fps
	if blocks[0].DurationMS > filmMS {
		t.Errorf("narration of %d ms in a film of %d ms", blocks[0].DurationMS, filmMS)
	}
	want := strings.Join(strings.Fields(narration), "")
	for _, name := range []string{out.SRT, out.VTT} {
		data, err := os.ReadFile(onDisk(name))
		if err != nil {
			t.Fatal(err)
		}
		if got := narratedE2ECues(t, name, data, filmMS); got != want {
			t.Errorf("%s text = %q, want %q", name, got, want)
		}
	}

	// The library copy has the MP4's size. Videos is DataDir/generated_videos.
	library := filepath.Join(s.Cfg.Directories.DataDir, "generated_videos", "Storymaker", slug+".mp4")
	if info, err := os.Stat(library); err != nil || info.Size() != out.Size || out.Library != "Videos/Storymaker/"+slug+".mp4" {
		t.Errorf("library copy: %v, job says %q and %d bytes", err, out.Library, out.Size)
	}

	// The record next to the MP4 names the extras and the loudness.
	recordJSON, err := os.ReadFile(onDisk(exports + job.ID + ".json"))
	if err != nil {
		t.Fatal(err)
	}
	var record map[string]any
	if err := json.Unmarshal(recordJSON, &record); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"stems", "srt", "vtt", "library", "loudness"} {
		if record[key] == nil {
			t.Errorf("the job record has no %q: %s", key, recordJSON)
		}
	}
}
