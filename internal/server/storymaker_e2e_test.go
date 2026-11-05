package server

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"aurago/internal/storymaker"
	"aurago/internal/storymaker/render"

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
	out, err := exec.Command(probe, "-v", "error", "-show_entries", "stream=codec_type,codec_name,width,height,avg_frame_rate,nb_frames,pix_fmt:format=duration", "-of", "json", file).CombinedOutput()
	if err != nil {
		t.Fatalf("ffprobe %s: %v\n%s", file, err, out)
	}
	var doc storymakerProbeDoc
	if err := json.Unmarshal(out, &doc); err != nil {
		t.Fatalf("ffprobe output: %v\n%s", err, out)
	}
	return doc
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
