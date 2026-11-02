## Part I: Integration of A, D, C, R, B and E on main

Parts A to E were drafted on `0248bc133`; main moved on to `561685cde` (35 commits). The integration step overlaid every part on `561685cde` and ran everything together. This part holds what that run changed: tests that run the whole path against the real server, and the fixes for the mismatches between the app (part E, built against a stand-in server) and the real handlers (part B, built against contract section 5). Execute it after E11, the last task of part E. Task B15 (the render page sends frames as simple requests) sits in part B and runs there, after R; it was rewritten in this step to match the merged player files. The section "Review fixes" at the end (tasks I7 to I10) holds the fixes for the findings of the four reviews of this integrated tree.

What was checked, and what it found (details and numbers in `part-I-notes.md`, committed next to this file):

| Check | Result | Task |
| --- | --- | --- |
| Export end to end through the HTTP handlers: create, save `firewall.json` and its code scene, render, event stream to `succeeded`, ffprobe of the MP4 in the project folder | green at once | I1 |
| The PUT of the config page (`storymaker.enabled`, `storymaker.render.*`) is accepted, written to the YAML and read back | green at once | I1 |
| The app tour against the real handlers (real shell, real player page and CSP, real catalog, real esbuild, an export that ends in an MP4) | green at once | I2 |
| `status.browser.can_download`: B lets an export start when the pinned Chromium can be downloaded, the app blocked it | **mismatch**, fixed | I3, I4 |
| `story: null` for a damaged `story.json`: B answers it, the app threw a TypeError | **mismatch**, fixed | I4 |
| A refused export (422 `invalid_story` or `code_error`) was reported as "the story was not saved" | **mismatch**, fixed | I4 |
| A code scene whose file outlived its scene got the same id again and could never be created (412 on `If-None-Match: *`) | **mismatch**, fixed | I4 |
| A seek (scrubber, End key) while the preview reloads after an edit was lost or replayed with a stale frame; the E8 tour failed by chance because of it (2 of 17 runs) | **bug**, fixed, regression step in the tour | I4 |
| `NewBrowsersWithProbe` dropped the pinned Chromium, so no server test could reach the "downloadable" state | test seam, fixed | I3 |
| Everything else the app assumes of the server (status shape with `limits.max_narration_chars`, project GET/POST/PUT shapes, 404 for a missing code source and a failed bundle, flat error bodies, SSE `event: job` and the snapshot of the latest jobs, `/player` with the session cookie only, 429 `busy`, 409 `ffmpeg_unavailable`/`browser_unavailable`, 412/428) | matches | I2, I4 |
| New nested `AGENTS.md` files are ignored by `.gitignore` (line 142, only the root one is excepted) | plan commands fixed to `git add -f` | A8, B14, E10 |

Conventions of this part: the browser tests are gated by `AURAGO_RUN_BROWSER_SMOKE=1` and need FFmpeg (`AURAGO_TEST_FFMPEG_DIR`, `PATH` or the WinGet package folder) and an installed Chrome (`AURAGO_TEST_CHROME` overrides the lookup; `AURAGO_TEST_NO_SANDBOX=1` only for a box without a usable Chrome sandbox). On the Windows development box run them through `go test` with `GOCACHE=D:\dev-cache\go-build`, `GOTMPDIR=D:\tmp\go-build` and the Bash sandbox off (a repo-built executable cannot start Chrome there).

### Task I1: Export end to end through the real handlers, and the config round trip

Part B tests its handlers with recorders and its render pipeline with a memory publisher; part E tests the app against a stand-in. Nothing else runs the whole path: HTTP handlers, the desktop workspace, real FFmpeg and Chrome, an MP4 in the project folder. `TestStorymakerExportEndToEnd` is that test without the app: it creates a project, saves `firewall.json` and its code scene, follows the project's event stream to `succeeded`, checks the order and the phases of the events, a replayed `Idempotency-Key`, the output path, the job record next to the MP4, `GET /jobs/{id}`, `GET /projects/{id}/jobs`, a new stream replaying the finished job, and runs ffprobe over the MP4 (1920x1080, 30 fps, yuv420p, frame count equal to the job's, AAC, duration within 0.2 s of frames/30).

`TestStorymakerConfigPatchOfTheConfigPageIsAcceptedAndRoundTrips` sends the PUT the config page produces for the Storymaker block and checks the YAML on disk, the running configuration, `GET /api/config` and the status endpoint. Task B11 only tested the rejecting paths.

The helper `storymakerRealServer` (FFmpeg from the box, the installed Chrome through the real resolver, no sign-in because a browser's iframe and event-stream requests carry only cookies) is shared with I2 and I4.

**Files:**
- Create: `internal/server/storymaker_e2e_test.go`
- Create: `internal/server/storymaker_config_test.go`

- [ ] **Step 1: Write the tests**

`internal/server/storymaker_e2e_test.go`:

```go
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
```

`internal/server/storymaker_config_test.go`:

```go
package server

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"aurago/internal/config"
)

// TestStorymakerConfigPatchOfTheConfigPageIsAcceptedAndRoundTrips sends the exact PUT the config page produces for the
// Storymaker block (data-path storymaker.enabled and storymaker.render.*), checks that the YAML on disk and the running
// configuration take the values, and that GET /api/config hands them back to the page. The rejecting paths are covered by
// TestStorymakerConfigValidationRejectsBadValuesBeforeWrite.
func TestStorymakerConfigPatchOfTheConfigPageIsAcceptedAndRoundTrips(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte("server:\n  host: 127.0.0.1\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, err := config.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	cfg.ConfigPath = path
	if !cfg.Storymaker.Enabled || cfg.Storymaker.Render.TimeoutSeconds != 3600 || cfg.Storymaker.Render.MaxDurationSeconds != 900 {
		t.Fatalf("defaults of a config without the section: %+v", cfg.Storymaker)
	}
	s := &Server{Cfg: cfg, Logger: slog.Default()}

	patch := `{"storymaker":{"enabled":false,"render":{"timeout_seconds":7200,"max_duration_seconds":600,"browser_path":"","transport":"webp"}}}`
	rec := httptest.NewRecorder()
	handleUpdateConfig(s).ServeHTTP(rec, httptest.NewRequest(http.MethodPut, "/api/config", strings.NewReader(patch)))
	if rec.Code != http.StatusOK {
		t.Fatalf("PUT = %d %s", rec.Code, rec.Body.String())
	}
	stored, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"storymaker:", "enabled: false", "timeout_seconds: 7200", "max_duration_seconds: 600", "transport: webp"} {
		if !strings.Contains(string(stored), want) {
			t.Errorf("config.yaml lacks %q:\n%s", want, stored)
		}
	}
	sm := s.ConfigSnapshot().Storymaker
	if sm.Enabled || sm.Render.TimeoutSeconds != 7200 || sm.Render.MaxDurationSeconds != 600 || sm.Render.Transport != "webp" {
		t.Errorf("the running configuration = %+v", sm)
	}

	// The page reads the section back with GET /api/config.
	rec = httptest.NewRecorder()
	handleGetConfig(s).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/config", nil))
	var got struct {
		Storymaker struct {
			Enabled bool `json:"enabled"`
			Render  struct {
				TimeoutSeconds     int    `json:"timeout_seconds"`
				MaxDurationSeconds int    `json:"max_duration_seconds"`
				Transport          string `json:"transport"`
			} `json:"render"`
		} `json:"storymaker"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil || rec.Code != http.StatusOK {
		t.Fatalf("GET = %d %v\n%s", rec.Code, err, rec.Body.String())
	}
	if got.Storymaker.Enabled || got.Storymaker.Render.TimeoutSeconds != 7200 || got.Storymaker.Render.MaxDurationSeconds != 600 || got.Storymaker.Render.Transport != "webp" {
		t.Errorf("GET /api/config storymaker = %+v", got.Storymaker)
	}

	// The status endpoint follows the switch.
	off := httptest.NewRecorder()
	handleStorymaker(s).ServeHTTP(off, httptest.NewRequest(http.MethodGet, storymakerAPIBase+"/status", nil))
	if off.Code != http.StatusOK || !strings.Contains(off.Body.String(), `"enabled":false`) {
		t.Errorf("status after the switch = %d %s", off.Code, off.Body.String())
	}
}
```

- [ ] **Step 2: Run the tests**

```bash
go test ./internal/server -run 'TestStorymakerConfigPatchOfTheConfigPage' -count=1 -v
AURAGO_RUN_BROWSER_SMOKE=1 go test ./internal/server -run 'TestStorymakerExportEndToEnd' -count=1 -timeout 15m -v
```

Expected: PASS. These two pin behaviour that tasks A1, B7 to B11 and E9 already implement, so they pass at once; they are the regression guards of the integrated path. Recorded at integration time:

```text
--- PASS lines of the server package (AURAGO_RUN_BROWSER_SMOKE=1):
--- PASS: TestStorymakerConfigPatchOfTheConfigPageIsAcceptedAndRoundTrips (0.06s)
--- PASS: TestStorymakerExportEndToEnd (24.26s)
ok  	aurago/internal/server (the four real-server tests of I1, I2 and I4 together: 81.947s)
```

- [ ] **Step 3: Commit**

```bash
git add internal/server/storymaker_e2e_test.go internal/server/storymaker_config_test.go
git commit -m "test(storymaker): export end to end through the real handlers and the config round trip" -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

### Task I2: The app tour against the real handlers

Package `ui` cannot build a configured `Server` (that takes the unexported test helpers of `internal/server`), so the tour of task E8 runs there against a stand-in. This task runs the same app against `handleStorymaker` itself: the real desktop shell (`desktop.html` and `main.bundle.js` from the ui tree, the same seam the audit harness of `ui/desktop_audit_browser_test.go` uses), the real app modules, the real player page with its CSP, the real catalog, the real esbuild, and an export that renders for real. The tour: start screen (the first `status` call launches the browser once, measured 3.1 s), create a project, a drawn thumbnail, play and pause, narration saved with `If-Match`, library, story.json in CodeMirror with an invalid value (the real validator's JSON Pointer path puts the mark on the right line), a code scene with a syntax error and its fix (real bundler positions), transition and duration, scene moves, an export that ends in the output path and an MP4 that ffprobe accepts, "Show in Files", leave and reopen, delete through the desktop dialog (the project folder is gone), close the window without a script error.

`smappPageWith` takes a step that runs against the server's origin before the app opens (used by I4); every wait has its own limit (90 s, the export 6 minutes), so one hung condition fails with a dump of the page instead of consuming the budget of the whole test.

**Files:**
- Create: `internal/server/storymaker_app_browser_test.go`

- [ ] **Step 1: Write the test**

`internal/server/storymaker_app_browser_test.go`:

```go
package server

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"aurago/internal/desktop"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/launcher"
)

// The Storymaker app tour against the REAL handlers. ui/desktop_storymaker_browser_test.go runs the same app against a stand-in
// for the API (package ui cannot reach the real handlers: they need a configured Server, which only this package's tests can
// build). This tour opens the real desktop shell and the real app modules in the installed Chrome, mounts handleStorymaker on
// /api/desktop/storymaker/, and ends in an export that renders for real (FFmpeg and Chrome) and lands as an MP4 in the project
// folder. Gated by AURAGO_RUN_BROWSER_SMOKE=1:
//
//	AURAGO_RUN_BROWSER_SMOKE=1 go test ./internal/server -run TestStorymakerAppRealServerBrowser -count=1 -timeout 15m

func smappWords(t *testing.T, lang string) map[string]string {
	t.Helper()
	words := map[string]string{}
	for _, dir := range []string{"storymaker", "storymaker_catalog"} {
		data, err := os.ReadFile(filepath.Join("..", "..", "ui", "lang", dir, lang+".json"))
		if err != nil {
			t.Fatal(err)
		}
		part := map[string]string{}
		if err := json.Unmarshal(data, &part); err != nil {
			t.Fatal(err)
		}
		for key, value := range part {
			words[key] = value
		}
	}
	return words
}

func smappRead(t *testing.T, name string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "..", "ui", filepath.FromSlash(name)))
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func smappJS(s string) string {
	data, _ := json.Marshal(s)
	return string(data)
}

// smappPage serves the desktop shell, the ui tree and the real Storymaker handlers, opens the page in the installed Chrome and
// opens the Storymaker app in it.
func smappPage(t *testing.T, s *Server) (*rod.Page, string) {
	t.Helper()
	return smappPageWith(t, s, nil)
}

// smappPageWith is smappPage with a step that runs against the server's origin before the app opens.
func smappPageWith(t *testing.T, s *Server, prepare func(origin string)) (*rod.Page, string) {
	t.Helper()
	html := regexp.MustCompile(`(?s)<script\b[^>]*>.*?</script>`).ReplaceAllString(smappRead(t, "desktop.html"), "")
	html = regexp.MustCompile(`\{\{[^}]*\}\}`).ReplaceAllString(html, "")
	html = strings.Replace(html, "</body>", `<script>window.fixtureWords=`+smappRead(t, "lang/desktop/de.json")+`;window.t=(k,v)=>{let s=fixtureWords[k]||k;for(const [key,value] of Object.entries(v||{}))s=s.replaceAll('{{'+key+'}}',value).replaceAll('{'+key+'}',value);return s;};window.fixtureErrors=[];addEventListener('error',e=>fixtureErrors.push(e.message));addEventListener('unhandledrejection',e=>fixtureErrors.push(String(e.reason)));window.WebSocket=class extends EventTarget{close(){}};</script><script src="/js/shared/lazy-assets.js"></script><script src="/js/desktop/core/module-loader.js"></script><script src="/audit-shell.js"></script></body>`, 1)
	shell := smappRead(t, "js/desktop/bundles/main.bundle.js")
	cut := strings.LastIndex(shell, "    ensureDesktopRadialMenuAnchor();")
	if cut < 0 {
		t.Fatal("startup seam missing")
	}
	shell = shell[:cut] + `window.auditTest={state,openApp,closeWindow};` + shell[cut:]
	settings := desktop.DesktopSettingDefaults()
	for key, value := range map[string]string{"windows.restore_session": "false", "windows.animations": "false", "pet.enabled": "false", "phone_gadget.enabled": "false", "desktop.show_widgets": "false"} {
		settings[key] = value
	}

	ui := http.FileServer(http.Dir(filepath.Join("..", "..", "ui")))
	mux := http.NewServeMux()
	mux.Handle("/", ui)
	mux.HandleFunc("/fixture", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprint(w, html)
	})
	mux.HandleFunc("/audit-shell.js", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/javascript")
		fmt.Fprint(w, shell)
	})
	mux.HandleFunc("/api/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/api/desktop/bootstrap" {
			_ = json.NewEncoder(w).Encode(map[string]any{"enabled": true, "builtin_apps": desktop.BuiltinApps(), "installed_apps": []any{}, "widgets": []any{}, "shortcuts": []any{}, "desktop_files": []any{}, "workspace": map[string]any{"readonly": false}, "settings": settings})
		} else {
			fmt.Fprint(w, `{"status":"ok","files":[],"pets":[],"settings":{},"enabled":false}`)
		}
	})
	mux.Handle(storymakerAPIBase+"/", handleStorymaker(s))
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	chrome, ok := launcher.LookPath()
	if !ok {
		t.Skip("no Chrome or Chromium is installed")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 14*time.Minute)
	t.Cleanup(cancel)
	l := launcher.New().Context(ctx).Bin(chrome).Headless(true).NoSandbox(true).Set("disable-gpu").Set("disable-dev-shm-usage")
	control, err := l.Launch()
	if err != nil {
		t.Skipf("the browser did not start: %v", err)
	}
	t.Cleanup(l.Cleanup)
	browser := rod.New().Context(ctx).ControlURL(control)
	if err := browser.Connect(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = browser.Close() })
	page := browser.MustPage().Timeout(10 * time.Minute)
	t.Cleanup(func() { _ = page.Close() })
	page.MustSetViewport(1280, 900, 1, false)
	page.MustNavigate(srv.URL + "/fixture").MustWaitLoad()
	page.MustWait(`()=>!!window.auditTest?.state.ws`)
	page.MustEval(`(words)=>Object.assign(fixtureWords, words)`, smappWords(t, "en"))
	if prepare != nil {
		prepare(srv.URL)
	}
	page.MustEval(`()=>auditTest.openApp('storymaker')`)
	return page, srv.URL
}

// smappWait waits for a condition in the page; one wait gets 90 seconds (the export waits longer, see smappWaitFor).
func smappWait(t *testing.T, page *rod.Page, expr string) {
	t.Helper()
	smappWaitFor(t, page, expr, 90*time.Second)
}

func smappWaitFor(t *testing.T, page *rod.Page, expr string, limit time.Duration) {
	t.Helper()
	if err := rod.Try(func() { page.Timeout(limit).MustWait(expr) }); err != nil {
		var state string
		_ = rod.Try(func() {
			state = page.Timeout(10 * time.Second).MustEval(`()=>JSON.stringify({errors: window.fixtureErrors, app: (document.querySelector('[data-stm="app"]')||{}).innerText||''}).slice(0, 1500)`).String()
		})
		t.Fatalf("timed out waiting for %s: %v\n%s", expr, err, state)
	}
}

func smappText(page *rod.Page, selector string) string {
	return page.MustEval(`(s)=>{const e=document.querySelector(s);return e?e.textContent:'';}`, selector).String()
}

func smappClick(t *testing.T, page *rod.Page, selector string) {
	t.Helper()
	smappWait(t, page, `()=>!!document.querySelector(`+smappJS(selector)+`)`)
	page.MustEval(`(s)=>document.querySelector(s).click()`, selector)
}

func smappNoErrors(t *testing.T, page *rod.Page) {
	t.Helper()
	if got := page.MustEval(`()=>JSON.stringify(window.fixtureErrors)`).String(); got != "[]" {
		t.Fatalf("script errors in the page: %s", got)
	}
}

// smappEditorReplace swaps text in the open CodeMirror editor through the public EditorView API of the vendored bundle.
func smappEditorReplace(page *rod.Page, find, insert string) {
	page.MustEval(`async (find, insert) => {
		const cm = await import('/js/vendor/codemirror-bundle.esm.js');
		const view = cm.EditorView.findFromDOM(document.querySelector('[data-stm="code-editor"] .cm-editor'));
		const text = view.state.doc.toString(), at = text.indexOf(find);
		if (at < 0) throw new Error('not in the editor: ' + find);
		view.dispatch({changes: {from: at, to: at + find.length, insert}});
	}`, find, insert)
}

// smappGet reads a JSON document from the real API.
func smappGet(t *testing.T, origin, path string, into any) {
	t.Helper()
	res, err := http.Get(origin + storymakerAPIBase + path)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	body, _ := io.ReadAll(res.Body)
	if res.StatusCode != http.StatusOK {
		t.Fatalf("GET %s = %d %s", path, res.StatusCode, body)
	}
	if err := json.Unmarshal(body, into); err != nil {
		t.Fatalf("GET %s: %v\n%s", path, err, body)
	}
}

type smappProject struct {
	Story map[string]any `json:"story"`
	ETag  string         `json:"etag"`
}

func smappStored(t *testing.T, origin, id string) map[string]any {
	t.Helper()
	var p smappProject
	smappGet(t, origin, "/projects/"+id, &p)
	return p.Story
}

func smappProjectIDs(t *testing.T, origin string) []string {
	t.Helper()
	var list struct {
		Projects []struct {
			ID string `json:"id"`
		} `json:"projects"`
	}
	smappGet(t, origin, "/projects", &list)
	ids := []string{}
	for _, p := range list.Projects {
		ids = append(ids, p.ID)
	}
	return ids
}

func TestStorymakerAppRealServerBrowser(t *testing.T) {
	s := storymakerRealServer(t)
	page, origin := smappPage(t, s)
	started := time.Now()

	// Start screen: the first status call launches the browser once (a cached probe), so the app first shows a neutral
	// loading state; then the empty list, with no capability banner because FFmpeg and a browser are ready.
	smappWait(t, page, `()=>!!document.querySelector('.stm-projects') && !!document.querySelector('[data-stm="projects-empty"]') && !document.querySelector('[data-stm="projects-empty"]').hidden`)
	t.Logf("start screen after %v", time.Since(started).Round(100*time.Millisecond))
	if page.MustEval(`()=>!!document.querySelector('[data-banner="export_unavailable"]')`).Bool() {
		t.Fatal("the export-unavailable banner shows although FFmpeg and a browser are ready")
	}

	// Create a project: the editor opens on one scene, the real player page compiles it and the strip shows a drawn thumbnail.
	page.MustElement(`[data-stm="new-title"]`).MustInput("Real server tour")
	page.MustElement(`[data-stm="aspect"][data-aspect="16:9"]`).MustClick()
	page.MustElement(`[data-stm="create-project"]`).MustClick()
	smappWait(t, page, `()=>document.querySelector('[data-stm="project-title"]')?.textContent==='Real server tour'`)
	smappWait(t, page, `()=>document.querySelectorAll('[data-stm="scene"]').length===1 && document.querySelector('[data-stm="clock-total"]').textContent!=='0:00.0'`)
	ids := smappProjectIDs(t, origin)
	if len(ids) != 1 {
		t.Fatalf("projects after create = %v", ids)
	}
	id := ids[0]
	if got := *page.MustElement(`.stm-player`).MustAttribute("sandbox"); got != "allow-scripts" {
		t.Fatalf("player sandbox = %q, want exactly allow-scripts", got)
	}
	smappWait(t, page, `()=>document.querySelector('canvas[data-scene="s1"]')?.dataset.drawn==='true'`)
	if !page.MustEval(`()=>{const c=document.querySelector('canvas[data-scene="s1"]'),d=c.getContext('2d').getImageData(0,0,c.width,c.height).data,seen=new Set();for(let i=0;i<d.length;i+=4*97)seen.add(d[i]+','+d[i+1]+','+d[i+2]);return seen.size>1;}`).Bool() {
		t.Fatal("the thumbnail of the first scene is a single colour: the preview rendered a blank frame")
	}
	smappNoErrors(t, page)

	// Play and pause through the real player page.
	page.MustEval(`()=>document.querySelector('[data-stm="play"]').click()`)
	smappWait(t, page, `()=>document.querySelector('[data-stm="play"]').dataset.playing==='true'`)
	smappWait(t, page, `()=>document.querySelector('[data-stm="clock"]').textContent!=='0:00.0'`)
	page.MustEval(`()=>document.querySelector('[data-stm="play"]').click()`)
	smappWait(t, page, `()=>document.querySelector('[data-stm="play"]').dataset.playing==='false'`)

	// Narration: typing saves with If-Match and the real server keeps the text.
	before := smappText(page, `[data-stm="clock-total"]`)
	page.MustElement(`[data-stm="narration"]`).MustInput("Anna works from home. Suddenly somebody knocks at the door.")
	smappWait(t, page, `()=>document.querySelector('[data-stm="save-state"]')?.dataset.state==='saved'`)
	scene := smappStored(t, origin, id)["scenes"].([]any)[0].(map[string]any)
	if narration, _ := scene["narration"].(string); !strings.Contains(narration, "knocks at the door") {
		t.Fatalf("stored narration = %q", narration)
	}
	smappWait(t, page, `()=>document.querySelector('[data-stm="clock-total"]').textContent!==`+smappJS(before))

	// Library: a person, a prop and an environment go into the selected scene.
	smappClick(t, page, `[data-stm="side-tab"][data-tab="library"]`)
	smappClick(t, page, `[data-stm="library-card"][data-id="char.office-woman-1"]`)
	smappWait(t, page, `()=>document.querySelectorAll('[data-stm="scene-items"] [data-kind="actor"]').length===1`)
	smappClick(t, page, `[data-stm="library-tab"][data-kind="prop"]`)
	smappClick(t, page, `[data-stm="library-card"][data-id="prop.laptop"]`)
	smappClick(t, page, `[data-stm="library-tab"][data-kind="environment"]`)
	smappClick(t, page, `[data-stm="library-card"][data-id="env.office"]`)
	smappWait(t, page, `()=>document.querySelector('[data-stm="library-card"][data-id="env.office"]')?.dataset.present==='true'`)
	smappWait(t, page, `()=>document.querySelector('[data-stm="save-state"]')?.dataset.state==='saved'`)
	scene = smappStored(t, origin, id)["scenes"].([]any)[0].(map[string]any)
	if scene["environment"] != "env.office" || len(scene["actors"].([]any)) != 1 || len(scene["props"].([]any)) != 1 {
		t.Fatalf("stored scene after the library clicks = %v", scene)
	}
	smappClick(t, page, `[data-stm="side-tab"][data-tab="scene"]`)

	// Code tab: an invalid value is saved, reported by the real validator with a JSON Pointer path and marked at its line.
	smappClick(t, page, `[data-stm="tab"][data-tab="code"]`)
	smappWait(t, page, `()=>!!document.querySelector('[data-stm="code-editor"] .cm-editor') && document.querySelector('[data-stm="code-editor"] .cm-content').textContent.includes('storymaker/1')`)
	smappEditorReplace(page, `"variant": "day"`, `"variant": "dusk"`)
	smappWait(t, page, `()=>document.querySelector('[data-stm="code-state"]')?.dataset.state==='dirty'`)
	smappClick(t, page, `[data-stm="code-save"]`)
	smappWait(t, page, `()=>document.querySelector('[data-stm="code-problems"]')?.dataset.count!=='0' && !!document.querySelector('[data-stm="code-editor"] .cm-lint-marker')`)
	if !strings.Contains(smappText(page, `[data-stm="code-problems"]`), "Invalid value") {
		t.Fatalf("the problem list does not name the issue: %q", smappText(page, `[data-stm="code-problems"]`))
	}
	smappNoErrors(t, page)
	smappEditorReplace(page, `"variant": "dusk"`, `"variant": "night"`)
	smappClick(t, page, `[data-stm="code-save"]`)
	smappWait(t, page, `()=>document.querySelector('[data-stm="code-problems"]')?.dataset.count==='0' && document.querySelector('[data-stm="code-state"]')?.dataset.state==='saved'`)
	if smappStored(t, origin, id)["scenes"].([]any)[0].(map[string]any)["variant"] != "night" {
		t.Fatalf("variant after the code save = %v", smappStored(t, origin, id)["scenes"].([]any)[0])
	}
	smappClick(t, page, `[data-stm="tab"][data-tab="stage"]`)

	// A code scene: the starter program is saved as a new file (If-None-Match), bundled by the real esbuild, a mistake is
	// shown at its line, and the fixed program makes the bundle available to the player.
	smappClick(t, page, `[data-stm="add-code-scene"]`)
	smappWait(t, page, `()=>document.querySelector('[data-stm="code-file"]')?.textContent==='code2.js' && document.querySelector('[data-stm="code-editor"] .cm-content')?.textContent.includes('export default function scene')`)
	smappEditorReplace(page, "api.duration(5);", "api.duration(5;")
	smappClick(t, page, `[data-stm="code-save"]`)
	smappWait(t, page, `()=>document.querySelector('[data-stm="code-problems"]')?.dataset.count!=='0' && document.querySelector('[data-stm="code-file-btn"][data-key="scene:code2"]')?.dataset.failed==='true'`)
	smappEditorReplace(page, "api.duration(5;", "api.duration(5);")
	smappClick(t, page, `[data-stm="code-save"]`)
	smappWait(t, page, `()=>document.querySelector('[data-stm="code-problems"]')?.dataset.count==='0' && document.querySelector('[data-stm="code-file-btn"][data-key="scene:code2"]')?.dataset.failed==='false'`)
	smappClick(t, page, `[data-stm="tab"][data-tab="stage"]`)
	smappWait(t, page, `()=>document.querySelectorAll('[data-stm="scene"]').length===2 && document.querySelector('[data-stm="scene"][data-scene="code2"] .stm-scene-kind')!==null`)

	// Inspector: transition and duration of the selected (code) scene are saved by the real validator.
	const setSelect = `(sel,value)=>{const s=document.querySelector(sel);s.value=value;s.dispatchEvent(new Event('change',{bubbles:true}));}`
	page.MustEval(setSelect, `[data-stm="transition"]`, "wipe")
	page.MustEval(setSelect, `[data-stm="duration-mode"]`, "fixed")
	page.MustEval(`()=>{const i=document.querySelector('[data-stm="duration-seconds"]');i.value='3.5';i.dispatchEvent(new Event('change',{bubbles:true}));}`)
	smappWait(t, page, `()=>document.querySelector('[data-stm="save-state"]')?.dataset.state==='saved'`)
	second := smappStored(t, origin, id)["scenes"].([]any)[1].(map[string]any)
	if second["transition"] != "wipe" || second["duration"] != 3.5 {
		t.Fatalf("stored transition and duration = %v, %v", second["transition"], second["duration"])
	}
	smappClick(t, page, `[data-stm="scene-up"]`)
	smappWait(t, page, `()=>document.querySelector('[data-stm="scene"][data-index="0"]')?.dataset.scene==='code2' && document.querySelector('[data-stm="save-state"]')?.dataset.state==='saved'`)
	smappClick(t, page, `[data-stm="scene-down"]`)
	smappWait(t, page, `()=>document.querySelector('[data-stm="scene"][data-index="1"]')?.dataset.scene==='code2' && document.querySelector('[data-stm="save-state"]')?.dataset.state==='saved'`)

	// Export: the real render runs (FFmpeg and Chrome), progress arrives on the project's event stream and the panel ends in
	// the output path of an MP4 that exists in the project folder.
	smappClick(t, page, `[data-stm="export"]`)
	smappWait(t, page, `()=>!!document.querySelector('[data-stm="export-start"]') && !document.querySelector('[data-stm="export-start"]').disabled`)
	if !page.MustEval(`()=>document.querySelectorAll('[data-stm="export-caps"] [data-ok="true"]').length===2`).Bool() {
		t.Fatal("FFmpeg and the browser should both be listed as ready")
	}
	smappClick(t, page, `[data-stm="export-start"]`)
	smappWait(t, page, `()=>!!document.querySelector('[data-stm="export-progress"]')`)
	renderStart := time.Now()
	smappWaitFor(t, page, `()=>!!document.querySelector('[data-stm="export-path"]') || !!document.querySelector('[data-stm="export-failed"]')`, 6*time.Minute)
	if page.MustEval(`()=>!!document.querySelector('[data-stm="export-failed"]')`).Bool() {
		t.Fatalf("the export failed: %s", smappText(page, `[data-stm="export-panel"]`))
	}
	t.Logf("export took %v", time.Since(renderStart).Round(100*time.Millisecond))
	path := smappText(page, `[data-stm="export-path"]`)
	if !strings.HasPrefix(path, "Documents/Storymaker/"+id+"/exports/Real-server-tour-") || !strings.HasSuffix(path, ".mp4") {
		t.Fatalf("export path = %q", path)
	}
	mp4 := filepath.Join(s.Cfg.VirtualDesktop.WorkspaceDir, filepath.FromSlash(path))
	doc := storymakerFFprobe(t, s.Cfg.VideoStudio.FFmpegPath, mp4)
	var video bool
	for _, st := range doc.Streams {
		video = video || st.CodecType == "video" && st.Width == 1920 && st.Height == 1080 && st.PixFmt == "yuv420p"
	}
	if !video {
		t.Fatalf("the MP4 is not 1920x1080 yuv420p: %+v", doc.Streams)
	}
	windows := page.MustEval(`()=>auditTest.state.windows.size`).Int()
	page.MustEval(`()=>document.querySelector('[data-stm="export-show"]').click()`)
	smappWait(t, page, fmt.Sprintf(`()=>auditTest.state.windows.size===%d`, windows+1))
	smappNoErrors(t, page)

	// Leave the project and open it again: the stored story comes back through the real GET.
	smappClick(t, page, `[data-stm="export-close"]`)
	smappClick(t, page, `[data-stm="back"]`)
	smappWait(t, page, `()=>document.querySelectorAll('[data-stm="open-project"]').length===1 && !document.querySelector('.stm-player')`)
	smappClick(t, page, `[data-stm="open-project"]`)
	smappWait(t, page, `()=>document.querySelector('[data-stm="narration"]')?.value.includes('knocks at the door') && document.querySelectorAll('[data-stm="scene"]').length===2`)

	// Delete asks the desktop dialog first: refusing keeps the project, confirming removes the folder.
	smappClick(t, page, `[data-stm="back"]`)
	smappClick(t, page, `[data-stm="delete-project"]`)
	smappWait(t, page, `()=>!!document.querySelector('.vd-modal [data-cancel]')`)
	page.MustEval(`()=>document.querySelector('.vd-modal [data-cancel]').click()`)
	smappWait(t, page, `()=>!document.querySelector('.vd-modal')`)
	if len(smappProjectIDs(t, origin)) != 1 {
		t.Fatal("cancelling the dialog must keep the project")
	}
	smappClick(t, page, `[data-stm="delete-project"]`)
	smappWait(t, page, `()=>!!document.querySelector('.vd-modal [type="submit"]')`)
	page.MustEval(`()=>document.querySelector('.vd-modal [type="submit"]').click()`)
	smappWait(t, page, `()=>document.querySelectorAll('[data-stm="open-project"]').length===0 && !document.querySelector('[data-stm="projects-empty"]').hidden`)
	if left := smappProjectIDs(t, origin); len(left) != 0 {
		t.Fatalf("projects after the delete = %v", left)
	}
	if _, err := os.Stat(filepath.Join(s.Cfg.VirtualDesktop.WorkspaceDir, "Documents", "Storymaker", id)); !os.IsNotExist(err) {
		t.Errorf("the project folder is still there after the delete: %v", err)
	}

	// Closing the window tears the app down without a script error.
	page.MustEval(`()=>auditTest.closeWindow([...auditTest.state.windows.values()].find(w=>w.appId==='storymaker').id)`)
	smappWait(t, page, `()=>!document.querySelector('[data-stm="app"]')`)
	smappNoErrors(t, page)
}
```

- [ ] **Step 2: Run the test**

```bash
AURAGO_RUN_BROWSER_SMOKE=1 go test ./internal/server -run 'TestStorymakerAppRealServerBrowser' -count=1 -timeout 15m -v
```

Expected: PASS (the app and the handlers agree on everything the tour touches). Recorded at integration time:

```text
    start screen after 3.1s
    export took 12.1s
--- PASS: TestStorymakerAppRealServerBrowser (26.12s)
```

- [ ] **Step 3: Commit**

```bash
git add internal/server/storymaker_app_browser_test.go
git commit -m "test(storymaker): run the app tour against the real handlers" -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

### Task I3: The test resolver keeps the pinned Chromium

`render.NewBrowsersWithProbe` (task B4) is how tests of the layers above `render` avoid starting a browser. It also emptied the table of pinned Chromium archives, so the resolver could only ever answer "ready" or "missing"; the state B documents for a host without a browser ("not ready, `can_download`, source `pinned`") could not be produced, and the app's handling of it (I4) had no test. The table now stays, and the resolver gets a transport that refuses every request, so a test can still never download 190 MB.

**Files:**
- Modify: `internal/storymaker/render/browser.go` (`NewBrowsersWithProbe`)
- Test: `internal/storymaker/render/browser_test.go`

- [ ] **Step 1: Write the failing test**

Append to `internal/storymaker/render/browser_test.go`:

```go
// The resolver of tests above this package (NewBrowsersWithProbe) must still know the pinned
// Chromium, so a status can say "downloadable", and must never download it.
func TestBrowsersWithProbeKnowsThePinnedChromiumButNeverDownloadsIt(t *testing.T) {
	if _, ok := pinnedChromium[runtime.GOOS+"_"+runtime.GOARCH]; !ok {
		t.Skip("no pinned Chromium for this platform")
	}
	b := NewBrowsersWithProbe(t.TempDir(),
		func() (string, bool) { return "", false },
		func(context.Context, string) (string, error) { return "", errors.New("no browser") })
	st := b.Status(context.Background(), "")
	if st.Ready || !st.CanDownload || st.Source != SourcePinned || !strings.Contains(st.Issue, "first export downloads") {
		t.Fatalf("status = %+v, want not ready, downloadable, source pinned", st)
	}
	st, exe := b.Ensure(context.Background(), "", nil)
	if exe != "" || st.Ready || !strings.Contains(st.Issue, "switched off") {
		t.Fatalf("Ensure = %+v %q, want no download", st, exe)
	}
}
```

- [ ] **Step 2: Run the test to verify it fails**

```bash
go test ./internal/storymaker/render -run 'TestBrowsersWithProbe' -count=1
```

Expected: FAIL. Recorded:

```text
--- FAIL: TestBrowsersWithProbeKnowsThePinnedChromiumButNeverDownloadsIt (0.00s)
    browser_test.go:354: status = {Ready:false Source:missing Issue:No Chrome or Chromium was found. Install one, or set storymaker.render.browser_path. CanDownload:false Version:}, want not ready, downloadable, source pinned
FAIL
FAIL	aurago/internal/storymaker/render	0.044s
```

- [ ] **Step 3: Implement**

```diff
--- a/internal/storymaker/render/browser.go
+++ b/internal/storymaker/render/browser.go
@@ -143,14 +143,22 @@ func (b *Browsers) pinnedInstalled() (string, bool) {
 }
 
 // NewBrowsersWithProbe is NewBrowsers with the system browser lookup and the launch probe replaced
-// (tests of the layers above this package use it to avoid starting a browser).
+// (tests of the layers above this package use it to avoid starting a browser). The pinned Chromium
+// stays known, so a status can say that it is downloadable, but the resolver never downloads it.
 func NewBrowsersWithProbe(dir string, lookPath func() (string, bool), probe func(ctx context.Context, bin string) (string, error)) *Browsers {
 	b := NewBrowsers(dir)
 	b.lookPath, b.launchProbe = lookPath, probe
-	b.pins = map[string]PinnedChromium{}
+	b.client = &http.Client{Transport: refuseDownloads{}}
 	return b
 }
 
+// refuseDownloads is the transport of the test resolver: nothing is fetched.
+type refuseDownloads struct{}
+
+func (refuseDownloads) RoundTrip(*http.Request) (*http.Response, error) {
+	return nil, errors.New("downloads are switched off in this resolver")
+}
+
 // Status reports the browser without downloading anything.
 func (b *Browsers) Status(ctx context.Context, configuredPath string) BrowserStatus {
 	st, _ := b.resolve(ctx, configuredPath, false, nil)
```

- [ ] **Step 4: Run the tests to verify they pass**

```bash
go test ./internal/storymaker/render -count=1
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/storymaker/render/browser.go internal/storymaker/render/browser_test.go
git commit -m "test(storymaker): the test resolver keeps the pinned Chromium but never downloads it" -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

### Task I4: The app adapts to what the real server answers

Five places where the app and the server (or the player host) did not meet, found by reading them against each other and by running the app against the real handlers:

1. **A downloadable browser.** `GET /status` answers `browser: {ready: false, source: "pinned", can_download: true, issue: "...first export downloads a pinned Chromium..."}` on a host without Chrome (aurago-test, a fresh install). The server lets an export start in that state (`ffmpeg.ready && (browser.ready || browser.can_download)`, B's rule; the first export downloads and verifies the archive). The app treated `ready: false` as a blocker everywhere (the start screen banner, the export panel, "Check export setup" in the config page), so no export could ever start on such a host. `S.model.browserState(status)` is `ready`, `download` or `missing`; only `missing` blocks. The capability row says, in the user's language, that the browser is downloaded once, on the first export (`storymaker.export.browser_download`).
2. **`story: null`.** A `story.json` edited by hand into something unreadable makes `GET /projects/{id}` answer `story: null`, `story_text` and the decode issues (B's addition, so the list does not fail). `openProject` read `inst.story.format` and threw a TypeError that showed as "Something went wrong. Cannot read properties of null". It now stays on the list with `storymaker.error.story_unreadable` (repair or delete `story.json` in Files).
3. **A refused export is not a refused save.** `POST .../render` answers 422 `invalid_story` or `code_error` when the server's validation finds errors the app did not know about; the generic 422 text said "The story was not saved". `S.errors.describe(t, err, 'render')` maps those codes to the job error texts ("The story has errors, so it cannot be exported.").
4. **Orphan code files.** The server keeps `scenes/<id>.js` when its scene is deleted (the contract has no DELETE for code files), and the app creates a code scene with `If-None-Match: *`. After add, delete, add the second scene got the id `code2` again and the create failed with 412 for good. `addScene` now skips the ids of `code_scenes` from the last project answer.

5. **A seek while the preview reloads.** Found because the tour of task E8 failed by chance (2 of 17 runs). After an edit the preview reloads (`load` message, compile, assets, first `time` message). The host drops a seek that arrives before its compile is done and answers one that arrives later out of order (it draws frame 0 last), and the player bridge replayed a *resume frame* it had captured when the load started. So an End or a click during the reload left the playhead on the last frame of the previous report, on the captured frame or on 0, and the host on frame 0. The playhead is `inst.frame` and stays authoritative: seeks wait while the host loads (`inst.compiling` or `inst.thumbsDue`), and the first `time` message after the compile brings the host to the playhead. The bridge keeps no resume frame any more (`load(payload)`). The tour has a step that presses End in the very task that shows the "preparing" note and checks the host's last `time` message, and waits for the reload that follows the code fix before it presses End (that part was a race of the test itself).

Everything else on the list of "requests to part B" holds (checked in the code and by I2): `limits.max_narration_chars`, the project answers, 404 for a missing code source and a failed bundle, flat error bodies, SSE `event: job` with the latest five jobs oldest first, `/player` with the session cookie only, 429 `busy`, 409 `ffmpeg_unavailable` and `browser_unavailable`, 412 and 428. The first `status` call launches the browser (3.1 s here, up to about 7 s), and the app shows its neutral loading state meanwhile.

**Files:**
- Modify: `ui/js/desktop/apps/storymaker-model.js` (`browserState`, `newSceneId`, `addScene`)
- Modify: `ui/js/desktop/apps/storymaker-export.js`
- Modify: `ui/js/desktop/apps/storymaker.js`, `ui/js/desktop/apps/storymaker-player.js`
- Modify: `ui/js/desktop/apps/storymaker-core.js` (`describeError`)
- Modify: `ui/cfg/virtual_desktop.js` (`vdCfgTestStorymaker`)
- Modify: `ui/lang/storymaker/{cs,da,de,el,en,es,fr,hi,it,ja,nl,no,pl,pt,sv,zh}.json` (two keys)
- Modify: `ui/desktop_storymaker_model_test.go`, `ui/desktop_storymaker_api_test.go`, `ui/desktop_storymaker_browser_test.go`
- Create: `internal/server/storymaker_app_states_browser_test.go`

- [ ] **Step 1: Write the failing tests**

The Node checks of the model and the error texts, and the tour of the stand-in server (diffs against the test files of tasks E2, E3 and E8; the tour gets a wait of 90 s per condition with a dump of the page, the comments that now point to I2, the wait for the reload after the code fix, and the step for a seek during a reload):

```diff
--- a/ui/desktop_storymaker_model_test.go
+++ b/ui/desktop_storymaker_model_test.go
@@ -131,6 +131,7 @@ eq([props.ok, draft.scenes[0].props[0].at], [true, 'right'], 'a prop takes a fre
 eq(M.insertActor(draft, M.scenesOf(draft).length - 1, man, 'x').ok, false, 'no actors in a code scene');
 const one = { schema: 'storymaker/1', scenes: [{ id: 's1' }] };
 eq(M.removeScene(one, 0), -1, 'the last scene stays');
+eq(one.scenes[M.addScene(one, 0, { kind: 'code', taken: ['code2'] })].id, 'code3', 'a code file that outlived its scene keeps its id');
 const scene = { id: 'x', narration: 'hi' };
 M.setSceneField(scene, 'narration', ''); eq('narration' in scene, false, 'an empty field leaves the document');
 M.setDuration(scene, false, '7.26'); eq(scene.duration, 7.3, 'fixed duration');
@@ -138,6 +139,7 @@ M.setDuration(scene, true, 0); eq(scene.duration, 'auto', 'auto duration');
 const report = { frame_count: 300, fps: 30, scenes: [{ id: 'a', start_frame: 0, end_frame: 150 }, { id: 'b', start_frame: 140, end_frame: 300 }] };
 eq(M.sceneAtFrame(report, 139), 0, 'frame in the first scene'); eq(M.sceneAtFrame(report, 145), 1, 'transitions overlap, the later scene wins');
 eq(M.frameForScene(report, 1), 140 + Math.floor(160 * 0.4), 'frame for a scene');
+eq([{ browser: { ready: true } }, { browser: { ready: false, can_download: true } }, { browser: { ready: false } }, {}, null].map(M.browserState), ['ready', 'download', 'missing', 'missing', 'missing'], 'the render browser is ready, fetched by the first export, or missing');
 
 // the message schema: only fixed shapes, with caps
 const P = S.player.checkIncoming;
--- a/ui/desktop_storymaker_api_test.go
+++ b/ui/desktop_storymaker_api_test.go
@@ -107,6 +107,9 @@ async function rejects(promise, check, what) {
   eq(S.errors.describe(t, { status: 500, code: 'internal', message: 'boom' }), 'storymaker.error.generic boom', 'text for the rest keeps the server detail');
   eq(S.errors.describe(t, { status: 503, code: 'ffmpeg_unavailable' }), 'storymaker.job_error.ffmpeg_unavailable', 'a refused render start names the missing part');
   eq(S.errors.describe(t, { status: 429, code: 'busy' }), 'storymaker.error.busy', 'busy');
+  eq(S.errors.describe(t, { status: 422, code: 'invalid_story' }), 'storymaker.error.invalid_story', 'a refused save names the story');
+  eq(S.errors.describe(t, { status: 422, code: 'invalid_story' }, 'render'), 'storymaker.job_error.invalid_story', 'a refused export names the story');
+  eq(S.errors.describe(t, { status: 422, code: 'code_error' }, 'render'), 'storymaker.job_error.code_error', 'a refused export names the code scene');
 
   // a missing bundle is null, not an error
   answers['GET ' + base + '/projects/' + id + '/scenes/s3/bundle'] = json({ error: 'not_found' }, 404);
--- a/ui/desktop_storymaker_browser_test.go
+++ b/ui/desktop_storymaker_browser_test.go
@@ -16,8 +16,9 @@ import (
 
 // The Storymaker app browser tour (spec "Testing": one browser smoke tour of the app). It runs the real desktop shell with the
 // real app modules, the real player iframe (host and player bundles under the exact player CSP) and the real catalog, against
-// a handler for /api/desktop/storymaker. The default handler is storymakerFixture; the integration step passes B's real
-// handlers instead. Gated by AURAGO_RUN_BROWSER_SMOKE=1:
+// a handler for /api/desktop/storymaker. The handler here is storymakerFixture, a stand-in on top of the real internal/storymaker
+// package. The same app runs against the real handlers (FFmpeg, Chrome, an export that ends in an MP4) in
+// internal/server/storymaker_app_browser_test.go (package ui cannot build a configured Server). Gated by AURAGO_RUN_BROWSER_SMOKE=1:
 //
 //	AURAGO_RUN_BROWSER_SMOKE=1 go test ./ui -run TestStorymakerApp -count=1 -timeout 10m
 
@@ -50,10 +51,15 @@ func storymakerBrowserPage(t *testing.T, api http.Handler) *rod.Page {
 	return page
 }
 
+// storymakerWait waits for a condition in the page. One wait gets 90 seconds, so a condition that never comes true fails with a dump
+// of the page instead of using up the budget of the whole tour (and the dump is read with a limit of its own).
 func storymakerWait(t *testing.T, page *rod.Page, expr string) {
 	t.Helper()
-	if err := rod.Try(func() { page.MustWait(expr) }); err != nil {
-		state := page.MustEval(`()=>JSON.stringify({errors: window.fixtureErrors, app: (document.querySelector('[data-stm="app"]')||{}).innerText||''}).slice(0, 1500)`).String()
+	if err := rod.Try(func() { page.Timeout(90 * time.Second).MustWait(expr) }); err != nil {
+		var state string
+		_ = rod.Try(func() {
+			state = page.Timeout(10 * time.Second).MustEval(`()=>JSON.stringify({errors: window.fixtureErrors, scrubber: [...document.querySelectorAll('[data-stm="scrubber"]')].map(e=>[e.getAttribute('aria-valuenow'),e.getAttribute('aria-valuemax')]), app: (document.querySelector('[data-stm="app"]')||{}).innerText||''}).slice(0, 2500)`).String()
+		})
 		t.Fatalf("timed out waiting for %s: %v\n%s", expr, err, state)
 	}
 }
@@ -107,8 +113,7 @@ func storymakerNoErrors(t *testing.T, page *rod.Page) {
 }
 
 // storymakerBackend is what the tour needs from the thing behind /api/desktop/storymaker: the handler, a view of the stored projects
-// and a way to move the render job along. storymakerFixture implements it; the integration step implements it over the real handlers
-// (the stored story through the project API, advance as a wait for the real job) and runs storymakerTour against them.
+// and a way to move the render job along. storymakerFixture implements it.
 type storymakerBackend interface {
 	http.Handler
 	projectIDs() []string
@@ -242,6 +247,11 @@ func storymakerTour(t *testing.T, fixture storymakerBackend) {
 	storymakerClick(t, page, `[data-stm="tab"][data-tab="stage"]`)
 	storymakerWait(t, page, `()=>document.querySelectorAll('[data-stm="scene"]').length===2 && document.querySelector('[data-stm="scene"][data-scene="code2"] .stm-scene-kind')!==null`)
 
+	// The preview reload that follows the last save of the code scene must be over first (it starts 250 ms after the save): a seek during
+	// it is clamped to the report of that moment, and the playhead would not be at the end of the new one.
+	time.Sleep(600 * time.Millisecond)
+	storymakerWait(t, page, `()=>document.querySelector('[data-stm="stage-note"]').hidden && document.querySelector('[data-stm="save-state"]')?.dataset.state==='saved'`)
+
 	// Scrubber: Home goes to the start, a click lands in the middle, End goes to the last frame.
 	page.MustEval(`()=>document.querySelector('[data-stm="scrubber"]').focus()`)
 	if err := page.Keyboard.Type(input.Home); err != nil {
@@ -276,6 +286,41 @@ func storymakerTour(t *testing.T, fixture storymakerBackend) {
 	storymakerClick(t, page, `[data-stm="scene-down"]`)
 	storymakerWait(t, page, `()=>document.querySelector('[data-stm="scene"][data-index="1"]')?.dataset.scene==='code2' && document.querySelector('[data-stm="save-state"]')?.dataset.state==='saved'`)
 
+	// A seek that comes while the preview compiles after an edit is kept. The End key is pressed in the very task that shows the
+	// "preparing" note, so it lands in the loading phase every time; the host drops or misorders seeks then, and the playhead used to
+	// fall back to frame 0 (or to the frame of the edit), which made the tour fail by chance.
+	storymakerWait(t, page, `()=>document.querySelector('[data-stm="stage-note"]').hidden && document.querySelector('[data-stm="save-state"]')?.dataset.state==='saved'`)
+	page.MustEval(`()=>document.querySelector('[data-stm="scrubber"]').focus()`)
+	if err := page.Keyboard.Type(input.Home); err != nil {
+		t.Fatal(err)
+	}
+	storymakerWait(t, page, `()=>document.querySelector('[data-stm="scrubber"]').getAttribute('aria-valuenow')==='0'`)
+	page.MustEval(`()=>{
+		window.stmHostFrames = [];
+		window.addEventListener('message', event => { const m = event.data; if (m && m.sm === 1 && m.type === 'time') window.stmHostFrames.push(m.frame); });
+		const note = document.querySelector('[data-stm="stage-note"]');
+		window.stmSeekWhileCompiling = new Promise(resolve => {
+			const watch = new MutationObserver(() => {
+				if (note.hidden) return;
+				watch.disconnect();
+				document.querySelector('[data-stm="scrubber"]').dispatchEvent(new KeyboardEvent('keydown', {key: 'End', bubbles: true, cancelable: true}));
+				resolve(true);
+			});
+			watch.observe(note, {attributes: true, attributeFilter: ['hidden']});
+		});
+	}`)
+	page.MustEval(setSelect, `[data-stm="transition"]`, "fade")
+	page.MustEval(`()=>window.stmSeekWhileCompiling`)
+	storymakerWait(t, page, `()=>document.querySelector('[data-stm="stage-note"]').hidden && (()=>{const s=document.querySelector('[data-stm="scrubber"]');return s.getAttribute('aria-valuenow')===s.getAttribute('aria-valuemax')&&s.getAttribute('aria-valuemax')!=='0'})()`)
+	time.Sleep(700 * time.Millisecond) // a late answer of the host must not move the playhead back
+	if !page.MustEval(`()=>{const s=document.querySelector('[data-stm="scrubber"]');return s.getAttribute('aria-valuenow')===s.getAttribute('aria-valuemax')}`).Bool() {
+		t.Fatal("the playhead left the end after the preview reloaded")
+	}
+	// and the host shows that frame too: its last time message is the end, not the frame 0 it starts at after a load
+	if !page.MustEval(`()=>{const s=document.querySelector('[data-stm="scrubber"]'),f=window.stmHostFrames;return f.length>0&&String(f[f.length-1])===s.getAttribute('aria-valuemax')}`).Bool() {
+		t.Fatalf("the host stayed on another frame than the playhead: %v", page.MustEval(`()=>JSON.stringify(window.stmHostFrames)`).String())
+	}
+
 	// Export: the panel steps through the fake job and ends in the output path with "show in Files".
 	storymakerClick(t, page, `[data-stm="export"]`)
 	storymakerWait(t, page, `()=>!!document.querySelector('[data-stm="export-start"]') && !document.querySelector('[data-stm="export-start"]').disabled`)
```

The states of the real server, `internal/server/storymaker_app_states_browser_test.go` (a browser that the first export downloads, no FFmpeg, read-only, switched off, a story that somebody else changed, a `story.json` that cannot be read):

```go
package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"aurago/internal/storymaker/render"
)

// smappCreate makes a project through the real API and returns its id and etag.
func smappCreate(t *testing.T, origin, title string) (string, string) {
	t.Helper()
	res, err := http.Post(origin+storymakerAPIBase+"/projects", "application/json", strings.NewReader(`{"title":`+smappJS(title)+`}`))
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	var created struct {
		Project struct {
			ID string `json:"id"`
		} `json:"project"`
		ETag string `json:"etag"`
	}
	if err := json.NewDecoder(res.Body).Decode(&created); err != nil || res.StatusCode != http.StatusCreated {
		t.Fatalf("create %q = %d %v", title, res.StatusCode, err)
	}
	return created.Project.ID, created.ETag
}

// TestStorymakerAppRealServerStatesBrowser opens the app against the real handlers in the states the server can be in: a
// browser that the first export downloads, no FFmpeg, read-only, switched off, a story that somebody else changed, and a
// story.json that was edited into something unreadable. (The stand-in server of package ui covers the same list for the
// app alone; these are the answers the real handlers give.) Gated by AURAGO_RUN_BROWSER_SMOKE=1.
func TestStorymakerAppRealServerStatesBrowser(t *testing.T) {
	t.Run("browser is downloaded by the first export", func(t *testing.T) {
		s := storymakerRealServer(t)
		s.Cfg.Storymaker.Render.BrowserPath = ""
		s.storymakerBrowsers = render.NewBrowsersWithProbe(t.TempDir(),
			func() (string, bool) { return "", false },
			func(context.Context, string) (string, error) { return "", fmt.Errorf("no browser") })
		page, origin := smappPage(t, s)
		smappWait(t, page, `()=>!!document.querySelector('.stm-projects')`)
		var status struct {
			Browser struct {
				Ready       bool `json:"ready"`
				CanDownload bool `json:"can_download"`
			} `json:"browser"`
		}
		smappGet(t, origin, "/status", &status)
		if status.Browser.Ready || !status.Browser.CanDownload {
			t.Fatalf("status.browser = %+v, want not ready but downloadable", status.Browser)
		}
		if page.MustEval(`()=>!!document.querySelector('[data-banner="export_unavailable"]')`).Bool() {
			t.Fatal("the export-unavailable banner shows although the first export downloads a browser")
		}
		page.MustElement(`[data-stm="new-title"]`).MustInput("Download")
		page.MustElement(`[data-stm="create-project"]`).MustClick()
		smappWait(t, page, `()=>!!document.querySelector('[data-stm="export"]') && document.querySelector('[data-stm="clock-total"]').textContent!=='0:00.0'`)
		smappClick(t, page, `[data-stm="export"]`)
		smappWait(t, page, `()=>!!document.querySelector('[data-stm="export-start"]') && !document.querySelector('[data-stm="export-start"]').disabled`)
		if page.MustEval(`()=>!!document.querySelector('[data-stm="export-blockers"]')`).Bool() {
			t.Fatalf("a downloadable browser must not block the export: %s", smappText(page, `[data-stm="export-panel"]`))
		}
		if caps := smappText(page, `[data-stm="export-caps"]`); !strings.Contains(caps, "downloaded once") {
			t.Fatalf("the capability list should say that the browser is downloaded: %q", caps)
		}
		smappNoErrors(t, page)
	})

	t.Run("no ffmpeg", func(t *testing.T) {
		s := storymakerRealServer(t)
		s.Cfg.VideoStudio.FFmpegPath = filepath.Join(t.TempDir(), "no-such-ffmpeg")
		page, _ := smappPage(t, s)
		smappWait(t, page, `()=>!!document.querySelector('[data-banner="export_unavailable"]')`)
		page.MustElement(`[data-stm="new-title"]`).MustInput("Offline")
		page.MustElement(`[data-stm="create-project"]`).MustClick()
		smappWait(t, page, `()=>!!document.querySelector('[data-stm="export"]')`)
		smappClick(t, page, `[data-stm="export"]`)
		smappWait(t, page, `()=>!!document.querySelector('[data-stm="export-blockers"]')`)
		if text := smappText(page, `[data-stm="export-blockers"]`); !strings.Contains(text, "FFmpeg is missing") || strings.Contains(text, "No browser") {
			t.Fatalf("blockers = %q", text)
		}
		if !page.MustEval(`()=>document.querySelector('[data-stm="export-start"]').disabled`).Bool() {
			t.Fatal("the start button must be disabled")
		}
		if page.MustEval(`()=>document.querySelector('[data-stm="narration"]').disabled`).Bool() {
			t.Fatal("editing must stay possible without FFmpeg")
		}
		smappNoErrors(t, page)
	})

	t.Run("read-only", func(t *testing.T) {
		s := storymakerRealServer(t)
		page, origin := smappPage(t, s)
		smappWait(t, page, `()=>!!document.querySelector('.stm-projects')`)
		page.MustElement(`[data-stm="new-title"]`).MustInput("Before")
		page.MustElement(`[data-stm="create-project"]`).MustClick()
		smappWait(t, page, `()=>!!document.querySelector('[data-stm="narration"]')`)
		ro := *s.ConfigSnapshot()
		ro.VirtualDesktop.ReadOnly = true
		s.replaceConfigSnapshot(&ro)
		// A write that races the policy event is refused by the server with desktop_readonly.
		res, err := http.Post(origin+storymakerAPIBase+"/projects", "application/json", strings.NewReader(`{"title":"x"}`))
		if err != nil {
			t.Fatal(err)
		}
		res.Body.Close()
		if res.StatusCode != http.StatusForbidden {
			t.Fatalf("a create while read-only = %d", res.StatusCode)
		}
		page.MustEval(`()=>document.dispatchEvent(new CustomEvent('aurago:desktop-policy',{detail:{readonly:true}}))`)
		smappWait(t, page, `()=>!!document.querySelector('[data-banner="readonly"]') && document.querySelector('[data-stm="narration"]').disabled && document.querySelector('[data-stm="add-scene"]').disabled`)
		smappClick(t, page, `[data-stm="export"]`)
		smappWait(t, page, `()=>document.querySelector('[data-stm="export-blockers"] [data-reason="readonly"]')!==null && document.querySelector('[data-stm="export-start"]').disabled`)
		smappNoErrors(t, page)
	})

	t.Run("switched off", func(t *testing.T) {
		s := storymakerRealServer(t)
		off := *s.ConfigSnapshot()
		off.Storymaker.Enabled = false
		s.replaceConfigSnapshot(&off)
		page, _ := smappPage(t, s)
		smappWait(t, page, `()=>document.querySelector('[data-stm="state"]')?.dataset.state==='storymaker.state.disabled_title'`)
		if !strings.Contains(smappText(page, `[data-stm="state"]`), "Storymaker is switched off") {
			t.Fatalf("state card = %q", smappText(page, `[data-stm="state"]`))
		}
		smappNoErrors(t, page)
	})

	t.Run("somebody else saved the story", func(t *testing.T) {
		s := storymakerRealServer(t)
		page, origin := smappPage(t, s)
		smappWait(t, page, `()=>!!document.querySelector('.stm-projects')`)
		page.MustElement(`[data-stm="new-title"]`).MustInput("Conflict")
		page.MustElement(`[data-stm="create-project"]`).MustClick()
		smappWait(t, page, `()=>!!document.querySelector('[data-stm="narration"]')`)
		id := smappProjectIDs(t, origin)[0]
		var p smappProject
		smappGet(t, origin, "/projects/"+id, &p)
		p.Story["title"] = "Changed elsewhere"
		data, _ := json.Marshal(p.Story)
		req, _ := http.NewRequest(http.MethodPut, origin+storymakerAPIBase+"/projects/"+id+"/story", strings.NewReader(string(data)))
		req.Header.Set("If-Match", p.ETag)
		res, err := http.DefaultClient.Do(req)
		if err != nil || res.StatusCode != http.StatusOK {
			t.Fatalf("the other client's save: %v %v", err, res)
		}
		res.Body.Close()
		page.MustElement(`[data-stm="narration"]`).MustInput("My text")
		smappWait(t, page, `()=>!!document.querySelector('[data-banner="conflict"]')`)
		smappClick(t, page, `[data-stm="conflict-overwrite"]`)
		smappWait(t, page, `()=>!document.querySelector('[data-banner="conflict"]') && document.querySelector('[data-stm="save-state"]')?.dataset.state==='saved'`)
		scene := smappStored(t, origin, id)["scenes"].([]any)[0].(map[string]any)
		if narration, _ := scene["narration"].(string); narration != "My text" {
			t.Fatalf("stored narration after overwrite = %q", narration)
		}
		smappNoErrors(t, page)
	})

	t.Run("story.json cannot be read", func(t *testing.T) {
		s := storymakerRealServer(t)
		var id string
		page, origin := smappPageWith(t, s, func(origin string) {
			// A project whose story.json was damaged by a hand edit in Files.
			id, _ = smappCreate(t, origin, "Broken")
			file := filepath.Join(s.Cfg.VirtualDesktop.WorkspaceDir, "Documents", "Storymaker", id, "story.json")
			if err := os.WriteFile(file, []byte(`{"schema": "storymaker/1", "scenes": [`), 0o600); err != nil {
				t.Fatal(err)
			}
		})
		smappWait(t, page, `()=>document.querySelectorAll('[data-stm="open-project"]').length===1`)
		var got struct {
			Story     any    `json:"story"`
			StoryText string `json:"story_text"`
		}
		smappGet(t, origin, "/projects/"+id, &got)
		if got.Story != nil || !strings.Contains(got.StoryText, `"scenes": [`) {
			t.Fatalf("the server should answer story null with the text: %+v", got)
		}
		smappClick(t, page, `[data-stm="open-project"]`)
		smappWait(t, page, `()=>(document.querySelector('.stm-projects')?.textContent||'').includes('cannot be read')`)
		if page.MustEval(`()=>!!document.querySelector('[data-stm="narration"]')`).Bool() {
			t.Fatal("a project without a readable story must not open the editor")
		}
		smappNoErrors(t, page)
	})
}
```

- [ ] **Step 2: Run the tests to verify they fail**

```bash
go test ./ui -run 'TestStorymakerAppModelInNode|TestStorymakerAppApiAndAssetsInNode' -count=1
AURAGO_RUN_BROWSER_SMOKE=1 go test ./ui -run 'TestStorymakerAppBrowser$' -count=1 -v
AURAGO_RUN_BROWSER_SMOKE=1 go test ./internal/server -run 'TestStorymakerAppRealServerStatesBrowser' -count=1 -timeout 15m -v
```

Expected: FAIL in the two Node checks, in the tour of the stand-in server (the host stays on frame 0 after the End key of the new step), and in the real-server states `browser is downloaded by the first export` and `story.json cannot be read` (the other four states pass). Recorded:

```text
--- FAIL: TestStorymakerAppModelInNode
        Error: a code file that outlived its scene keeps its id: got "code2", want "code3"
--- FAIL: TestStorymakerAppApiAndAssetsInNode
        Error: a refused export names the story: got "storymaker.error.invalid_story", want "storymaker.job_error.invalid_story"
--- FAIL: TestStorymakerAppBrowser
    desktop_storymaker_browser_test.go:128: the host stayed on another frame than the playhead: [0,0]
--- FAIL: TestStorymakerAppRealServerStatesBrowser
    --- FAIL: TestStorymakerAppRealServerStatesBrowser/browser_is_downloaded_by_the_first_export
        storymaker_app_states_browser_test.go:60: the export-unavailable banner shows although the first export downloads a browser
    --- FAIL: TestStorymakerAppRealServerStatesBrowser/story.json_cannot_be_read
        storymaker_app_states_browser_test.go:188: timed out waiting for ()=>(document.querySelector('.stm-projects')?.textContent||'').includes('cannot be read'): error value: context.deadlineExceededError{}
```

- [ ] **Step 3: Implement**

The app modules (diffs against the files of tasks E2 for core and model, E7 for export, E8 for the composition root and E9 for the config page):

```diff
--- a/ui/js/desktop/apps/storymaker-model.js
+++ b/ui/js/desktop/apps/storymaker-model.js
@@ -184,6 +184,14 @@
         return (Array.isArray(list) ? list : []).filter(issue => issue.code !== 'narration_missing');
     }
 
+    // The render browser as an export needs it, from GET /status: "ready" (a launch probe succeeded), "download" (none is ready but the
+    // first export downloads the server's pinned Chromium, status.browser.can_download) or "missing".
+    function browserState(status) {
+        const browser = status && status.browser || {};
+        if (browser.ready === true) return 'ready';
+        return browser.can_download === true ? 'download' : 'missing';
+    }
+
     function issueCounts(list) {
         const counts = { error: 0, warning: 0, minor: 0 };
         (Array.isArray(list) ? list : []).forEach(issue => { if (counts[issue.severity] !== undefined) counts[issue.severity]++; });
@@ -245,8 +253,10 @@
     }
 
     // s1, s2 ... the smallest number above the count that is not taken
-    function newSceneId(story, prefix) {
-        const taken = new Set(scenesOf(story).map(scene => scene && scene.id));
+    // newSceneId(story, prefix, alsoTaken): the next free id. alsoTaken are ids that exist outside the story: the server keeps a code
+    // scene's file when its scene is deleted (there is no DELETE for code files), and a new code scene is created with If-None-Match: *.
+    function newSceneId(story, prefix, alsoTaken) {
+        const taken = new Set(scenesOf(story).map(scene => scene && scene.id).concat(alsoTaken || []));
         const base = prefix || 's';
         for (let n = scenesOf(story).length + 1; n < 100000; n++) if (!taken.has(base + n)) return base + n;
         return base + Date.now();
@@ -257,7 +267,7 @@
         const scenes = story.scenes = scenesOf(story);
         const at = Math.min(Math.max(afterIndex + 1, 0), scenes.length);
         const previous = scenes[Math.min(afterIndex, scenes.length - 1)];
-        const scene = { id: newSceneId(story, o.kind === 'code' ? 'code' : 's') };
+        const scene = { id: newSceneId(story, o.kind === 'code' ? 'code' : 's', o.taken) };
         if (o.kind === 'code') scene.kind = 'code';
         else if (previous && previous.environment) scene.environment = previous.environment;
         scenes.splice(at, 0, scene);
@@ -394,6 +404,7 @@
     M.mergeIssues = mergeIssues;
     M.visibleIssues = visibleIssues;
     M.issueCounts = issueCounts;
+    M.browserState = browserState;
     M.sceneIndexOfIssue = sceneIndexOfIssue;
     M.indexCatalog = indexCatalog;
     M.entriesOf = entriesOf;
--- a/ui/js/desktop/apps/storymaker-export.js
+++ b/ui/js/desktop/apps/storymaker-export.js
@@ -51,7 +51,7 @@
             const status = inst.status || {};
             if (inst.readonly) list.push('readonly');
             if (!(status.ffmpeg && status.ffmpeg.ready)) list.push('ffmpeg');
-            if (!(status.browser && status.browser.ready)) list.push('browser');
+            if (S.model.browserState(status) === 'missing') list.push('browser');
             const counts = S.model.issueCounts(S.model.visibleIssues(S.model.mergeIssues(inst.issues, inst.report ? inst.report.issues : [])));
             if (counts.error > 0) list.push('errors');
             const max = Number(limits().max_duration_seconds) || 0;
@@ -68,9 +68,11 @@
             list.appendChild(capabilityRow(!!ffmpeg.ready, t('storymaker.export.cap_ffmpeg'),
                 ffmpeg.ready ? (ffmpeg.version ? t('storymaker.export.ready_version', { version: String(ffmpeg.version).slice(0, 60) }) : t('storymaker.export.ready')) : t('storymaker.export.ffmpeg_missing'),
                 !ffmpeg.ready && ffmpeg.issue ? el('code', { class: 'stm-cap-issue', text: String(ffmpeg.issue).slice(0, 400) }) : null));
-            list.appendChild(capabilityRow(!!browser.ready, t('storymaker.export.cap_browser'),
-                browser.ready ? t('storymaker.export.browser_ready', { source: sourceLabel(browser.source) }) : t('storymaker.export.browser_missing'),
-                !browser.ready && browser.issue ? el('code', { class: 'stm-cap-issue', text: String(browser.issue).slice(0, 400) }) : null));
+            const state = S.model.browserState(status);
+            // "download": no browser is ready but the first export fetches one; that is a note, not a blocker.
+            list.appendChild(capabilityRow(state !== 'missing', t('storymaker.export.cap_browser'),
+                state === 'ready' ? t('storymaker.export.browser_ready', { source: sourceLabel(browser.source) }) : state === 'download' ? t('storymaker.export.browser_download') : t('storymaker.export.browser_missing'),
+                state === 'missing' && browser.issue ? el('code', { class: 'stm-cap-issue', text: String(browser.issue).slice(0, 400) }) : null));
             return list;
         }
 
@@ -213,7 +215,7 @@
             } catch (err) {
                 if (err && err.aborted) return;
                 if (err && err.status === 412) inst.bus.emit('conflict', err);
-                inst.notify(S.errors.describe(t, err), 'error');
+                inst.notify(S.errors.describe(t, err, 'render'), 'error');
             } finally {
                 starting = false;
                 paint();
--- a/ui/js/desktop/apps/storymaker.js
+++ b/ui/js/desktop/apps/storymaker.js
@@ -129,7 +129,7 @@
         const add = (key, tone, text) => area.appendChild(el('div', { class: 'stm-banner', role: 'status', dataset: { banner: key, tone } }, icon(tone === 'bad' ? 'alert' : 'info'), el('span', { text })));
         if (inst.readonly) add('readonly', 'info', t('storymaker.banner.readonly'));
         const status = inst.status || {};
-        const ffmpegOk = !!(status.ffmpeg && status.ffmpeg.ready), browserOk = !!(status.browser && status.browser.ready);
+        const ffmpegOk = !!(status.ffmpeg && status.ffmpeg.ready), browserOk = S.model.browserState(status) !== 'missing';
         if (!ffmpegOk || !browserOk) {
             const missing = [!ffmpegOk ? t('storymaker.export.cap_ffmpeg') : '', !browserOk ? t('storymaker.export.cap_browser') : ''].filter(Boolean).join(', ');
             add('export_unavailable', 'info', t('storymaker.banner.export_unavailable', { missing }));
@@ -192,6 +192,12 @@
         try {
             const data = fresh || await inst.api.getProject(id);
             if (inst.disposed) return;
+            // A story.json that was edited by hand into something unreadable comes back as story: null with the issues; there is
+            // no document to edit, so the project stays closed and the list says why.
+            if (!data.story || typeof data.story !== 'object' || Array.isArray(data.story)) {
+                const first = Array.isArray(data.issues) && data.issues.length ? data.issues[0] : null;
+                throw Object.assign(S.errors.normalize({ message: first && first.message ? String(first.message) : '' }), { code: 'story_unreadable', network: false });
+            }
             closeEditor(inst);
             inst.view = 'editor';
             inst.projectId = data.project.id;
@@ -207,7 +213,7 @@
             inst.saveState = 'saved';
             buildEditor(inst);
             inst.events = inst.api.events(inst.projectId, job => inst.bus.emit('job', job));
-            reloadPlayer(inst, 0);
+            reloadPlayer(inst);
             return true;
         } catch (err) {
             if (inst.disposed || (err && err.aborted)) return false;
@@ -315,7 +321,13 @@
     function bindPlayer(inst, note) {
         const t = inst.t;
         const showNote = text => { note.textContent = text || ''; note.hidden = !text; };
-        inst.player.on('loading', () => { inst.compiling = true; showNote(t('storymaker.stage.compiling')); });
+        inst.player.on('loading', () => {
+            inst.compiling = true;
+            // A seek that is still in flight belongs to the document the host is about to drop; the playhead (inst.frame) is kept.
+            inst.seekBusy = false;
+            clearTimeout(inst.seekTimer);
+            showNote(t('storymaker.stage.compiling'));
+        });
         inst.player.on('compiled', report => {
             inst.report = report;
             inst.compiling = false;
@@ -327,6 +339,10 @@
         });
         inst.player.on('time', ({ frame, playing }) => {
             inst.playing = playing;
+            // The first time message after a compile says that the host is ready and shows frame 0. Seeks wait for it (while it loads the
+            // host drops them or answers them out of order), so the playhead may be elsewhere by now: bring the host there.
+            const landed = inst.thumbsDue && !playing && inst.frame !== frame;
+            if (landed) inst.wantFrame = inst.frame;
             // A paused host answers every seek with a time message. The next wanted frame goes out once the last one is answered,
             // and until then the playhead stays where the user put it (an older answer must not move it back).
             if (!playing && inst.seekBusy && frame === inst.seekSent) {
@@ -334,9 +350,10 @@
                 clearTimeout(inst.seekTimer);
                 pumpSeek(inst);
             }
-            if (playing || (!inst.seekBusy && inst.wantFrame === null)) inst.frame = frame;
+            if (!landed && (playing || (!inst.seekBusy && inst.wantFrame === null))) inst.frame = frame;
             // The first time message after a compile is the "ready to play" signal: the assets are in, so scenes can be drawn.
             if (inst.thumbsDue) { inst.thumbsDue = false; requestThumbs(inst); }
+            pumpSeek(inst);
             if (playing && inst.report) {
                 const index = S.model.sceneAtFrame(inst.report, frame);
                 if (index >= 0 && index !== inst.selected && Date.now() - (inst.selectedAt || 0) > 600) select(inst, index, { seek: false });
@@ -358,7 +375,7 @@
         inst.player.on('ready', () => { inst.playerUnavailable = false; paintBanners(inst); });
     }
 
-    async function reloadPlayer(inst, resume) {
+    async function reloadPlayer(inst) {
         if (inst.disposed || !inst.player || !inst.story) return;
         const seq = ++inst.loadSeq;
         try {
@@ -371,7 +388,7 @@
             inst.player.load({
                 story: inst.story, catalog: prepared.catalog, audio_info: {}, assets: prepared.assets, fonts: prepared.fonts,
                 code_bundles: prepared.code_bundles, format: { aspect, fps, width: S.model.previewWidth(aspect) }
-            }, resume === undefined ? inst.frame : resume);
+            });
         } catch (err) {
             if (inst.disposed || seq !== inst.loadSeq || (err && err.aborted)) return;
             const note = inst.parts.stageNote;
@@ -422,7 +439,8 @@
     }
 
     function pumpSeek(inst) {
-        if (inst.disposed || !inst.player || inst.seekBusy || inst.wantFrame === null) return;
+        // While the host loads (from the load message to its first time message) the wanted frame waits; the ready message sends it.
+        if (inst.disposed || !inst.player || inst.seekBusy || inst.wantFrame === null || inst.compiling || inst.thumbsDue) return;
         const target = inst.wantFrame;
         inst.wantFrame = null;
         inst.player.seek(target);
@@ -492,7 +510,7 @@
     async function addScene(inst, kind) {
         if (inst.readonly) return;
         const draft = S.model.cloneStory(inst.story);
-        const index = S.model.addScene(draft, inst.selected, { kind });
+        const index = S.model.addScene(draft, inst.selected, { kind, taken: (inst.codeStatus || []).map(item => item.scene_id) });
         const id = draft.scenes[index].id;
         if (kind === 'code') {
             try {
@@ -645,7 +663,7 @@
         }
         if (inst.playerUnavailable) add('player_unavailable', 'bad', t('storymaker.banner.player_unavailable'), retryButton(inst, () => reloadPlayerFrame(inst)));
         const status = inst.status || {};
-        const ffmpegOk = !!(status.ffmpeg && status.ffmpeg.ready), browserOk = !!(status.browser && status.browser.ready);
+        const ffmpegOk = !!(status.ffmpeg && status.ffmpeg.ready), browserOk = S.model.browserState(status) !== 'missing';
         if (!ffmpegOk || !browserOk) {
             const missing = [!ffmpegOk ? t('storymaker.export.cap_ffmpeg') : '', !browserOk ? t('storymaker.export.cap_browser') : ''].filter(Boolean).join(', ');
             add('export_unavailable', 'info', t('storymaker.banner.export_unavailable', { missing }));
@@ -657,7 +675,7 @@
         inst.playerUnavailable = false;
         inst.player.iframe.src = inst.api.playerUrl();
         paintBanners(inst);
-        reloadPlayer(inst, inst.frame);
+        reloadPlayer(inst);
     }
 
     function paintSave(inst) {
--- a/ui/js/desktop/apps/storymaker-player.js
+++ b/ui/js/desktop/apps/storymaker-player.js
@@ -104,7 +104,7 @@
             tabindex: '-1'
         });
         iframe.setAttribute('sandbox', 'allow-scripts');
-        let ready = false, loaded = false, pending = null, disposed = false, dropped = 0, thumbSeq = 0, resume = 0, readyCount = 0, loadTimer = 0;
+        let ready = false, loaded = false, pending = null, disposed = false, dropped = 0, thumbSeq = 0, readyCount = 0, loadTimer = 0;
         const thumbs = new Map();
 
         function post(message, transfer) {
@@ -153,11 +153,6 @@
                     emit('compiled', msg.report);
                     break;
                 case 'time':
-                    if (resume > 0 && !msg.playing) {
-                        const frame = resume;
-                        resume = 0;
-                        if (msg.frame !== frame) post({ type: 'seek', frame });
-                    }
                     emit('time', { frame: msg.frame, playing: msg.playing });
                     break;
                 case 'thumb': {
@@ -198,11 +193,10 @@
                 (handlers[name] = handlers[name] || []).push(fn);
                 return () => { handlers[name] = (handlers[name] || []).filter(candidate => candidate !== fn); };
             },
-            // load(payload, resumeFrame): the next `load` message (story, catalog, assets, fonts, code_bundles, format); the
-            // frame is shown again after the compile (the host starts at frame 0).
-            load(payload, resumeFrame) {
+            // load(payload): the next `load` message (story, catalog, assets, fonts, code_bundles, format). The host starts at frame 0;
+            // the owner brings it back to the playhead with the first `time` message after the compile.
+            load(payload) {
                 pending = payload;
-                resume = Number(resumeFrame) > 0 ? Math.floor(resumeFrame) : 0;
                 failThumbs(new Error('superseded'));
                 sendLoad();
             },
--- a/ui/js/desktop/apps/storymaker-core.js
+++ b/ui/js/desktop/apps/storymaker-core.js
@@ -281,9 +281,10 @@
         };
     }
 
-    // describe(t, err) is the sentence for an error; unknown ones say what happened in general terms and append the server's
-    // own message as detail (shown as text only).
-    function describeError(t, err) {
+    // describe(t, err, context) is the sentence for an error; unknown ones say what happened in general terms and append the server's
+    // own message as detail (shown as text only). context "render" is an export start: a 422 there is a refused export (the story or
+    // a code scene has errors), not a refused save.
+    function describeError(t, err, context) {
         const e = normalizeError(err);
         let key = '';
         if (e.network) key = 'storymaker.error.network';
@@ -291,6 +292,8 @@
         else if (e.code === 'desktop_scope_required') key = 'storymaker.error.scope';
         else if (e.code === 'desktop_readonly') key = 'storymaker.error.readonly';
         else if (e.code === 'storymaker_disabled') key = 'storymaker.error.disabled';
+        else if (context === 'render' && (e.code === 'invalid_story' || e.code === 'code_error')) key = 'storymaker.job_error.' + e.code;
+        else if (e.code === 'story_unreadable') key = 'storymaker.error.story_unreadable';
         else if (e.code === 'invalid_story' || e.status === 422) key = 'storymaker.error.invalid_story';
         else if (e.status === 412) key = 'storymaker.error.conflict';
         else if (e.status === 428) key = 'storymaker.error.required';
--- a/ui/cfg/virtual_desktop.js
+++ b/ui/cfg/virtual_desktop.js
@@ -285,7 +285,7 @@ function vdCfgTestStorymaker(button) {
         cfgActionResult.show('vd-storymaker-result', 'warning', t(prefix + 'save_first'));
         return;
     }
-    const missing = status => [!(status.ffmpeg && status.ffmpeg.ready) ? t(prefix + 'part_ffmpeg') : '', !(status.browser && status.browser.ready) ? t(prefix + 'part_browser') : ''].filter(Boolean);
+    const missing = status => [!(status.ffmpeg && status.ffmpeg.ready) ? t(prefix + 'part_ffmpeg') : '', !(status.browser && (status.browser.ready || status.browser.can_download)) ? t(prefix + 'part_browser') : ''].filter(Boolean);
     // The status asks FFmpeg and launches the browser once (a cached probe), so allow it some time.
     return cfgActionResult(button, () => fetch('/api/desktop/storymaker/status', { signal: AbortSignal.timeout(60000) }), {
         target: 'vd-storymaker-result',
```

The two new texts in all 16 locale files. Save this script as `disposable/add-storymaker-locale-keys.mjs` (the folder is git-ignored) and run it from the repository root; it inserts `storymaker.export.browser_download` after `storymaker.export.browser_missing` and `storymaker.error.story_unreadable` after `storymaker.error.invalid_story`, keeps the one-key-per-line layout and does nothing when a key is already there:

```js
// Adds storymaker.export.browser_download (after storymaker.export.browser_missing) and storymaker.error.story_unreadable
// (after storymaker.error.invalid_story) to the 16 app locale files, keeping their one-key-per-line layout.
import fs from 'node:fs';
import path from 'node:path';

const dir = process.argv[2] || 'ui/lang/storymaker';

const T = {
    'storymaker.export.browser_download': {
        en: 'A browser for drawing the frames is downloaded once, when you start your first export.',
        cs: 'Prohlížeč pro kreslení snímků se stáhne jednou, při prvním exportu.',
        da: 'En browser til at tegne billederne downloades én gang, når du starter din første eksport.',
        de: 'Ein Browser zum Zeichnen der Bilder wird einmalig heruntergeladen, wenn du den ersten Export startest.',
        el: 'Ένα πρόγραμμα περιήγησης για τη σχεδίαση των καρέ θα ληφθεί μία φορά, όταν ξεκινήσετε την πρώτη εξαγωγή.',
        es: 'Un navegador para dibujar los fotogramas se descarga una sola vez, cuando inicias tu primera exportación.',
        fr: 'Un navigateur pour dessiner les images est téléchargé une seule fois, au lancement de ton premier export.',
        hi: 'फ़्रेम बनाने वाला एक ब्राउज़र आपका पहला एक्सपोर्ट शुरू करने पर एक बार डाउनलोड किया जाएगा।',
        it: 'Un browser per disegnare i fotogrammi viene scaricato una sola volta, quando avvii la prima esportazione.',
        ja: 'フレームを描画するブラウザーは、最初の書き出しを開始するときに一度だけダウンロードされます。',
        nl: 'Een browser om de beelden te tekenen wordt eenmalig gedownload wanneer je je eerste export start.',
        no: 'En nettleser for å tegne bildene lastes ned én gang, når du starter den første eksporten.',
        pl: 'Przeglądarka do rysowania klatek zostanie pobrana jednorazowo, gdy uruchomisz pierwszy eksport.',
        pt: 'Um navegador para desenhar os fotogramas é transferido uma única vez, quando inicias a primeira exportação.',
        sv: 'En webbläsare för att rita bildrutorna laddas ned en gång, när du startar din första export.',
        zh: '首次导出时，会一次性下载用于绘制画面的浏览器。'
    },
    'storymaker.error.story_unreadable': {
        en: 'The story file of this project cannot be read. Repair or delete story.json in Files.',
        cs: 'Soubor s příběhem tohoto projektu nelze přečíst. Oprav nebo smaž story.json v Souborech.',
        da: 'Filen med historien i dette projekt kan ikke læses. Ret eller slet story.json i Filer.',
        de: 'Die Datei mit der Geschichte dieses Projekts lässt sich nicht lesen. Repariere oder lösche story.json in Dateien.',
        el: 'Το αρχείο ιστορίας αυτού του έργου δεν μπορεί να διαβαστεί. Διορθώστε ή διαγράψτε το story.json στα Αρχεία.',
        es: 'No se puede leer el archivo de la historia de este proyecto. Repara o elimina story.json en Archivos.',
        fr: "Le fichier d'histoire de ce projet est illisible. Répare ou supprime story.json dans Fichiers.",
        hi: 'इस प्रोजेक्ट की कहानी वाली फ़ाइल पढ़ी नहीं जा सकती। फ़ाइलें में story.json को ठीक करें या हटाएँ।',
        it: 'Il file della storia di questo progetto non può essere letto. Ripara o elimina story.json in File.',
        ja: 'このプロジェクトのストーリーファイルを読み取れません。ファイルで story.json を修復するか削除してください。',
        nl: 'Het verhaalbestand van dit project kan niet worden gelezen. Herstel of verwijder story.json in Bestanden.',
        no: 'Filen med historien i dette prosjektet kan ikke leses. Reparer eller slett story.json i Filer.',
        pl: 'Nie można odczytać pliku historii tego projektu. Napraw lub usuń story.json w Plikach.',
        pt: 'Não é possível ler o ficheiro da história deste projeto. Repara ou elimina story.json em Ficheiros.',
        sv: 'Berättelsefilen för det här projektet går inte att läsa. Reparera eller ta bort story.json i Filer.',
        zh: '无法读取此项目的故事文件。请在“文件”中修复或删除 story.json。'
    }
};
const AFTER = { 'storymaker.export.browser_download': 'storymaker.export.browser_missing', 'storymaker.error.story_unreadable': 'storymaker.error.invalid_story' };

for (const lang of Object.keys(T['storymaker.export.browser_download'])) {
    const file = path.join(dir, lang + '.json');
    const eol = fs.readFileSync(file, 'utf8').includes('\r\n') ? '\r\n' : '\n';
    const lines = fs.readFileSync(file, 'utf8').split(/\r?\n/);
    for (const key of Object.keys(T)) {
        if (lines.some(l => l.startsWith('  ' + JSON.stringify(key) + ':'))) continue;
        const at = lines.findIndex(l => l.startsWith('  ' + JSON.stringify(AFTER[key]) + ':'));
        if (at < 0) throw new Error(file + ' lacks the anchor ' + AFTER[key]);
        if (!lines[at].endsWith(',')) throw new Error(file + ' anchor line has no trailing comma: ' + lines[at]);
        lines.splice(at + 1, 0, '  ' + JSON.stringify(key) + ': ' + JSON.stringify(T[key][lang]) + ',');
    }
    fs.writeFileSync(file, lines.join(eol));
    JSON.parse(fs.readFileSync(file, 'utf8'));   // still valid
}
console.log('ok');
```

```bash
node disposable/add-storymaker-locale-keys.mjs
git diff --stat ui/lang/storymaker   # 16 files, 2 insertions each
```

- [ ] **Step 4: Run the tests to verify they pass**

```bash
go test ./ui -run 'Storymaker|TestDesktopStorymaker|TestConfigStorymaker' -count=1
AURAGO_RUN_BROWSER_SMOKE=1 go test ./internal/server -run 'TestStorymakerAppRealServer' -count=1 -timeout 15m -v
AURAGO_RUN_BROWSER_SMOKE=1 go test ./ui -run 'TestStorymakerApp|TestConfigStorymakerBrowser' -count=8 -timeout 30m
npm run check:ui
```

Expected: PASS. Recorded:

```text
go test ./ui -run Storymaker|TestDesktopStorymaker|TestConfigStorymaker (no browser): ok
real handlers (internal/server):
--- PASS: TestStorymakerAppRealServerStatesBrowser (26.70s)
    --- PASS: TestStorymakerAppRealServerStatesBrowser/browser_is_downloaded_by_the_first_export (3.22s)
    --- PASS: TestStorymakerAppRealServerStatesBrowser/no_ffmpeg (5.06s)
    --- PASS: TestStorymakerAppRealServerStatesBrowser/read-only (4.70s)
    --- PASS: TestStorymakerAppRealServerStatesBrowser/switched_off (1.53s)
    --- PASS: TestStorymakerAppRealServerStatesBrowser/somebody_else_saved_the_story (6.74s)
    --- PASS: TestStorymakerAppRealServerStatesBrowser/story.json_cannot_be_read (5.45s)
stand-in server tour, -count=3 (three passes of the whole set):
--- PASS: TestConfigStorymakerBrowser (3.33s)
--- PASS: TestStorymakerAppBrowser (9.66s)
--- PASS: TestStorymakerAppStatesBrowser (10.09s)
--- PASS: TestConfigStorymakerBrowser (2.61s)
--- PASS: TestStorymakerAppBrowser (9.19s)
--- PASS: TestStorymakerAppStatesBrowser (9.52s)
--- PASS: TestConfigStorymakerBrowser (1.70s)
--- PASS: TestStorymakerAppBrowser (10.66s)
--- PASS: TestStorymakerAppStatesBrowser (10.31s)
npm run check:ui: Checked ui/js/desktop/bundles/main.bundle.js (59 parts), Ocean chat sea life renderer verified
```

- [ ] **Step 5: Commit**

Run `node C:/Users/Andi/Documents/repo/AuraGo/.gitnexus/run.cjs detect-changes --scope all --repo .` first (CLAUDE.md); expected: the files below only. `impact` for the edited JavaScript functions (`describeError`, `addScene`, `openProject`, `blockers`, `capabilities`) is `UNKNOWN` (the index does not resolve this code); their callers are in `ui/js/desktop/apps/storymaker*.js` (grep) and every change is additive.

```bash
git add ui/js/desktop/apps/storymaker-model.js ui/js/desktop/apps/storymaker-export.js ui/js/desktop/apps/storymaker.js ui/js/desktop/apps/storymaker-core.js ui/js/desktop/apps/storymaker-player.js ui/cfg/virtual_desktop.js ui/lang/storymaker ui/desktop_storymaker_model_test.go ui/desktop_storymaker_api_test.go ui/desktop_storymaker_browser_test.go internal/server/storymaker_app_states_browser_test.go
git commit -m "fix(storymaker): the app meets the real server (downloadable browser, unreadable story, refused export, orphan code files)" -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

### Task I5: Documentation and comments after the integration

The comment of the E8 fixture promised that "the integration step" would run the tour against the real handlers (the tour file itself changed in I4); that is I2 now. The user page and the apps `AGENTS.md` name the real-server tests and the download of the pinned Chromium.

**Files:**
- Modify: `ui/desktop_storymaker_fixture_test.go` (comment)
- Modify: `ui/js/desktop/apps/AGENTS.md` (the Verify bullet of the Storymaker section)
- Modify: `documentation/storymaker.md` (Export, For developers)

- [ ] **Step 1: Apply the changes**

```diff
--- a/ui/desktop_storymaker_fixture_test.go
+++ b/ui/desktop_storymaker_fixture_test.go
@@ -23,8 +23,7 @@ import (
 // storymaker package: the projects live in a MemStore, saves go through DecodeStory, ApplyDefaults, Validate and the
 // project store, so a story that the fixture accepts is one the real server accepts, and code scenes are bundled by esbuild.
 // What is faked is only the world around it: the capability flags, and the render job, which a test steps by hand
-// (advance) so it can look at every state. The integration step runs the same browser tour against the real handlers by
-// passing them to storymakerBrowserPage instead of this fixture.
+// (advance) so it can look at every state. The real handlers get their own tour in internal/server.
 type storymakerFixture struct {
 	mu       sync.Mutex
 	t        *testing.T
--- a/ui/js/desktop/apps/AGENTS.md
+++ b/ui/js/desktop/apps/AGENTS.md
@@ -45,11 +45,11 @@
 
 - `storymaker*.js` are eleven classic scripts in this load order (module-loader entry `storymaker`, pinned by `TestDesktopStorymakerRegistration`): `storymaker-core.js` (the namespace `window.StorymakerParts`, the DOM builder `S.el` that has no html option, icons, the bus, error texts), `-model.js` (pure: JSON Pointer to text position, issue lists, the catalog index, the story edits), `-api.js`, `-assets.js` (catalog, fonts and code bundles for the player), `-player.js` (the iframe bridge), `-projects.js`, `-strip.js` (scene strip and transport), `-inspector.js` (inspector and library), `-code.js` (CodeMirror), `-export.js`, then the composition root `storymaker.js` (`window.StorymakerApp`). Style `css/desktop-app-storymaker.css`: `stm-` prefix, tokens only, container queries on the window, motion gated. The backend contract is `internal/storymaker/AGENTS.md`, the player `ui/js/storymaker/AGENTS.md`.
 - Untrusted text (story, narration, names, issue messages, server errors) reaches the page only as `textContent`. `TestStorymakerSourcesAreSafeAndSmall` greps these modules and `ui/js/storymaker` for innerHTML, insertAdjacentHTML, template strings (app modules), eval, browser dialogs and storage, and holds every module under 1100 lines.
-- The player iframe is `sandbox="allow-scripts"` and nothing else, `src` `/api/desktop/storymaker/player`. The parent accepts a message only from `iframe.contentWindow` and only in the schema of `S.player.checkIncoming` (contract section 8: fixed types, size caps, unknown fields dropped), posts with targetOrigin `'*'` (the iframe origin is opaque), sends `load` after every `ready` and answers `need_assets` only with what the catalog lists or the bundles of the story's code scenes. Ask for thumbnails after the first `time` message that follows a compile: the host cannot draw before its assets are in.
+- The player iframe is `sandbox="allow-scripts"` and nothing else, `src` `/api/desktop/storymaker/player`. The parent accepts a message only from `iframe.contentWindow` and only in the schema of `S.player.checkIncoming` (contract section 8: fixed types, size caps, unknown fields dropped), posts with targetOrigin `'*'` (the iframe origin is opaque), sends `load` after every `ready` and answers `need_assets` only with what the catalog lists or the bundles of the story's code scenes. Ask for thumbnails after the first `time` message that follows a compile: the host cannot draw before its assets are in. The playhead is `inst.frame`: `requestSeek` moves it at once and sends the newest wanted frame to the host one seek at a time, never while the host loads (from the `load` message to its first `time` message, which the host answers out of order or drops); that first message brings the host to `inst.frame`. The bridge keeps no resume frame.
 - Edits go through `inst.editScene(id, fn, {immediate})`: it clones the story, applies `fn` and saves with If-Match (700 ms debounce while typing, one save in flight, the server's story with its defaults replaces the local one when nothing newer was typed). 412 shows the conflict banner (reload, or keep mine), 422 keeps the text and shows the issues, `desktop_readonly` turns the window read-only. Never add a field the story format lacks: the server decodes strictly.
 - The code tab keeps its drafts across tab and file switches, walks an issue's JSON Pointer to a text position (`S.model.locate`) and shows bundler errors at line and column. A new code scene is written from `S.codeTemplate` with If-None-Match before the story names it.
 - Export (`S.createExport`): capability states come from `GET /status` (FFmpeg, browser, read-only, story errors, the duration limit), progress from the project's SSE stream with a 2.5 s poll as safety net, the result is a project-folder path with "show in Files" through `ctx.openApp('files', {path})`. All copy lives in `lang/storymaker/*.json` (16 locales); the catalog names are `lang/storymaker_catalog` and are never defined here.
-- Verify: `go test ./ui -run 'Storymaker|TestDesktopStorymaker|TestConfigStorymaker' -count=1` and the browser tour `AURAGO_RUN_BROWSER_SMOKE=1 go test ./ui -run TestStorymakerApp -count=1`. The fixture `desktop_storymaker_fixture_test.go` runs the real `internal/storymaker` package behind `/api/desktop/storymaker`; `storymakerBrowserPage` takes any handler, so the same tour runs against the real server handlers. The model and the message schema run in Node inside `TestStorymakerAppModelInNode`.
+- Verify: `go test ./ui -run 'Storymaker|TestDesktopStorymaker|TestConfigStorymaker' -count=1` and the browser tour `AURAGO_RUN_BROWSER_SMOKE=1 go test ./ui -run TestStorymakerApp -count=1`. The fixture `desktop_storymaker_fixture_test.go` runs the real `internal/storymaker` package behind `/api/desktop/storymaker`; The same app runs against the real handlers in `internal/server` (`TestStorymakerAppRealServerBrowser`: FFmpeg, the installed Chrome, an export that ends in an MP4; `TestStorymakerAppRealServerStatesBrowser`: download-on-first-export, no FFmpeg, read-only, switched off, conflict, unreadable story.json). The model and the message schema run in Node inside `TestStorymakerAppModelInNode`.
 
 ## Email exports
 
--- a/documentation/storymaker.md
+++ b/documentation/storymaker.md
@@ -53,8 +53,9 @@ Delete a project in the list: the whole folder, exports included, goes away.
 The **Export** button opens a panel that first says what is missing. Exporting needs:
 
 1. FFmpeg (found on `PATH` or at `video_studio.ffmpeg_path`),
-2. a browser that can run headless (Chrome or Chromium). In the Docker image set `storymaker.render.browser_path` to a
-   browser inside the container: the image has no Chromium libraries of its own,
+2. a browser that can run headless (Chrome or Chromium). Without one, the first export downloads a pinned Chromium (about 200 MB,
+   checked against a SHA-256 before it is used) into the data folder; the export panel says so before you start. In the Docker
+   image set `storymaker.render.browser_path` to a browser inside the container: the image has no Chromium libraries of its own,
 3. a story without errors that is not longer than `max_duration_seconds`.
 
 Editing and the preview work without 1 and 2. The export draws every frame in a headless browser, so a 15 minute video takes a
@@ -70,4 +71,6 @@ the fields of later phases; the editor shows them as "not supported yet" and the
 ## For developers
 
 `ui/js/desktop/apps/AGENTS.md` (Storymaker), `internal/storymaker/AGENTS.md` and `ui/js/storymaker/AGENTS.md` hold the
-contracts. The app tour test is `AURAGO_RUN_BROWSER_SMOKE=1 go test ./ui -run TestStorymakerApp -count=1`.
+contracts. The app tour against a stand-in server is `AURAGO_RUN_BROWSER_SMOKE=1 go test ./ui -run TestStorymakerApp -count=1`; the same
+app against the real handlers, ending in a rendered MP4, is `AURAGO_RUN_BROWSER_SMOKE=1 go test ./internal/server -run
+'TestStorymakerAppRealServer|TestStorymakerExportEndToEnd' -count=1 -timeout 15m` (FFmpeg and Chrome must be installed).
```

- [ ] **Step 2: Run the tests**

```bash
go vet ./ui
go test ./ui -run 'Storymaker' -count=1
```

Expected: PASS (documentation and comments only; this task has no failing-test step).

- [ ] **Step 3: Commit**

```bash
git add ui/desktop_storymaker_fixture_test.go ui/js/desktop/apps/AGENTS.md documentation/storymaker.md
git commit -m "docs(storymaker): point to the real-server tests and the browser download" -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

### Task I6: The whole matrix

No code. Run the matrix once on the finished branch; every line was green on the integration tree except the failures that already exist at `561685cde` (each of them was reproduced once on a clean detached checkout of that commit).

- [ ] **Step 1: Build, bundles, catalog, Node suites**

```bash
npm run build:ui
npm run check:ui
node scripts/test-storymaker-player.mjs
node scripts/build-storymaker-catalog.mjs --check
node --test assets/storymaker/test/geometry.test.mjs   # and parts, rig, motion, props, environments, catalog (one file per call)
GOOS=linux go build ./...
GOOS=linux go vet ./internal/storymaker/... ./internal/security/ ./internal/server/ && GOOS=darwin go vet ./internal/storymaker/render/
```

Expected: the catalog is `ab33e88a6b77`; the player suite prints `72 passed, 0 failed`; the builds and vets exit 0.

- [ ] **Step 2: Go suites without a browser**

```bash
go test ./internal/storymaker/... ./internal/config/ ./internal/desktop/ ./internal/security/ ./internal/videostudio/ -count=1
go test ./cmd/config-merger ./internal/setup -count=1
go test ./internal/server/ -count=1 -timeout 30m
go test ./ui -count=1 -timeout 30m
```

Expected: every package `ok` except two known failures in `./ui` (`TestChatFrontend_IntegrationsDrawerRemainsWired`, `TestFrontend_StaticI18nKeysExistInEnglishBundle`: seven keys in `findit-*.js`) and one in `./internal/server` (`TestGetDesktopServiceReusesCanonicalConfigWithRelativeDefaults`: it makes a `GOTMPDIR` path relative to the checkout; it fails on any box whose temp folder is on another drive than the checkout). All three fail the same way on a clean checkout of `561685cde`.

- [ ] **Step 3: Browser suites (`AURAGO_RUN_BROWSER_SMOKE=1`, FFmpeg and Chrome installed)**

```bash
go test ./ui -run "TestStorymaker(PlayerGate|RenderMode|FirewallRender|RenderScreenshot)Browser" -count=1 -timeout 20m
AURAGO_STORYMAKER_PARITY_DIR=<an absolute folder> go test ./ui -run TestStorymakerParityBrowser -count=1 -timeout 30m
go test ./internal/storymaker/render/ -run "TestBrowserStatusProbe|TestRealRender|TestEgressLayers" -count=1 -timeout 30m
go test ./internal/server -run TestStorymakerParity -count=1 -timeout 30m
go test ./ui -run "TestStorymakerApp|TestConfigStorymakerBrowser" -count=1 -timeout 30m
go test ./internal/server -run "TestStorymakerAppRealServer|TestStorymakerExportEndToEnd|TestStorymakerConfigPatch" -count=1 -timeout 30m
# every line with AURAGO_RUN_BROWSER_SMOKE=1 in the environment
```

Expected: PASS. Results at integration time:

```text
1  go test ./ui TestStorymaker(PlayerGate|RenderMode|FirewallRender|RenderScreenshot)Browser
ok  	aurago/ui	25.247s
2  go test ./ui TestStorymakerParityBrowser (opt-in dir)
ok  	aurago/ui	94.477s
3  go test ./internal/storymaker/render TestBrowserStatusProbe|TestRealRender|TestEgressLayers
ok  	aurago/internal/storymaker/render	127.004s
4  go test ./internal/server TestStorymakerParity
ok  	aurago/internal/server	47.656s
5  go test ./ui TestStorymakerApp|TestConfigStorymakerBrowser (-count=3)
ok  	aurago/ui	70.889s
6  go test ./internal/server TestStorymakerAppRealServer|TestStorymakerExportEndToEnd|TestStorymakerConfigPatch
ok  	aurago/internal/server	81.947s
```

## Review fixes

Four reviewers read the integrated tree. Their findings were verified one by one (read the code, probe where possible); the ones that held are fixed in tasks I7 to I10. Execute them after I6 (they are independent of each other). The full matrix of the index (section "Verification at the end") is the last step of task I10. Every fix has a test that failed before the change; the failing output is recorded in step 2 of its task. The findings share ids between reviewers, so they are named here by reviewer: SB, CG, CJ.

| Finding | Verdict | Fix | Task |
| --- | --- | --- | --- |
| SM-2 (CG) A failed code-scene save left the bundle of the previous source in place, reported as ok and exported | confirmed | the old bundle is removed before the source changes | I7 |
| SM-1 (CG) The pinned Chromium download ran outside the render timeout and could hold the single render worker | confirmed | stall limit of 60 s on the download, `render.timeout_seconds` around the whole browser setup | I8 |
| SM-1 (SB) The render Chrome's DevTools port exposed the render URL (the secret prefix) and full control of the browser to every local account | confirmed | no DevTools port: the browser is started with `--remote-debugging-pipe` and rod gets a pipe client | I9 |
| SM-1 (CJ) Leaving a project silently discarded unsaved edits after a failed save | confirmed | Back asks whenever the last edit was not saved | I10 |
| SM-3 (CJ) Two quick project opens raced; the earlier answer could replace the later one | confirmed | only the answer to the latest open builds the editor | I10 |
| SM-4 (CJ) Adding a code scene after deleting one failed with a misleading conflict | confirmed | a 412 on the create tries the next id | I10 |

Conventions as in the rest of part I: the browser tests are gated by `AURAGO_RUN_BROWSER_SMOKE=1`; on the Windows development box run them through `go test` with `GOCACHE=D:\dev-cache\go-build`, `GOTMPDIR=D:\tmp\go-build` and the Bash sandbox off. `gitnexus impact` knows only committed code: for the symbols below it answers "not found" (UNKNOWN), so the callers were taken from `grep`; the callers are named in each task.

### Task I7: A failed code-scene save never leaves a bundle that is older than its source

`SaveCodeScene` (task A7) wrote the new source first and the bundle afterwards. When the bundle write failed (a locked file, a full disk), or when the removal of the old bundle failed after a source that no longer bundles, the call returned an error but the stored source was already the new one and `scenes/<id>.bundle.js` was still the old build. `CodeSceneStatus` reports a scene with a bundle file as ok, and `Prepare` loads that bundle, so an export silently contained the old animation and raised no `code_error`. The first idea (remove the bundle right after the source write) does not close the case where the removal itself fails, so the order is turned around: the precondition is checked first (a refused save keeps the old bundle), then the old bundle is removed, then the source and the new bundle are written. The failure states are now: the removal failed (nothing changed, old source with its own bundle), or a later step failed (new source without a bundle, which blocks the render with "has not been bundled yet").

Callers of `SaveCodeScene` (grep; the symbol is not in the committed index): `handleStorymakerCode` in `internal/server/storymaker_handlers.go` and the tests of this package and of `internal/server`. The HTTP contract does not change (a conflict is still 412, an error 500).

**Files:**
- Modify: `internal/storymaker/project.go` (`SaveCodeScene`, new `checkCodePrecondition`)
- Modify: `internal/storymaker/AGENTS.md`
- Test: `internal/storymaker/project_test.go`

- [ ] **Step 1: Write the failing test**

Append to `internal/storymaker/project_test.go`:

```go
// faultStore fails WriteFile or Remove for paths that end in a chosen suffix.
type faultStore struct {
	*MemStore
	failWrite  string
	failRemove string
}

func (f *faultStore) WriteFile(ctx context.Context, p string, data []byte, ifMatch string) (string, error) {
	if f.failWrite != "" && strings.HasSuffix(p, f.failWrite) {
		return "", errors.New("injected write failure")
	}
	return f.MemStore.WriteFile(ctx, p, data, ifMatch)
}

func (f *faultStore) Remove(ctx context.Context, p string) error {
	if f.failRemove != "" && strings.HasSuffix(p, f.failRemove) {
		return errors.New("injected remove failure")
	}
	return f.MemStore.Remove(ctx, p)
}

// A save that fails half way must never leave a bundle that was built from older source reported
// as ok: render loads the bundle of every scene whose status is ok.
func TestSaveCodeSceneFailureNeverLeavesAStaleBundle(t *testing.T) {
	v1 := []byte("export default function scene(api) { api.duration(1); }\n")
	v2 := []byte("export default function scene(api) { api.duration(2); }\n")
	bad := []byte("export default function scene(api) {\n  eval('1');\n}\n")
	cases := []struct {
		name       string
		next       []byte
		failWrite  string
		failRemove string
		mustFail   bool
	}{
		{"bundle write fails", v2, "/s1.bundle.js", "", true},
		{"old bundle cannot be removed before a failing build", bad, "", "/s1.bundle.js", true},
		{"old bundle cannot be removed before a good build", v2, "", "/s1.bundle.js", false},
		{"source write fails", v2, "/scenes/s1.js", "", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			faults := &faultStore{MemStore: NewMemStore()}
			ps := NewProjectStore(faults, testRoot)
			ctx := context.Background()
			proj, _, _, err := ps.Create(ctx, "Code", "", 0)
			if err != nil {
				t.Fatal(err)
			}
			etag, _, err := ps.SaveCodeScene(ctx, proj.ID, "s1", v1, MustNotExist)
			if err != nil {
				t.Fatal(err)
			}
			faults.failWrite, faults.failRemove = tc.failWrite, tc.failRemove
			if _, _, err := ps.SaveCodeScene(ctx, proj.ID, "s1", tc.next, etag); err == nil && tc.mustFail {
				t.Fatal("the injected failure must be reported")
			}
			faults.failWrite, faults.failRemove = "", ""
			status, err := ps.CodeSceneStatus(ctx, proj.ID)
			if err != nil || len(status) != 1 {
				t.Fatalf("status: %v %+v", err, status)
			}
			if !status[0].OK {
				return // not bundled: render is blocked, which is the safe side
			}
			src, _, err := ps.LoadCodeSource(ctx, proj.ID, "s1")
			if err != nil {
				t.Fatal(err)
			}
			bundle, err := ps.LoadCodeBundle(ctx, proj.ID, "s1")
			if err != nil {
				t.Fatalf("status ok but no bundle: %v", err)
			}
			if want := BundleCodeScene(src, "s1", DefaultLimits().MaxCodeBytes).Code; string(bundle) != string(want) {
				t.Fatalf("status is ok but the bundle was not built from the current source\nsource: %s\nbundle: %s", src, bundle)
			}
		})
	}
}
```

- [ ] **Step 2: Run the test to verify it fails**

```bash
go test ./internal/storymaker/ -run 'TestSaveCodeSceneFailureNeverLeavesAStaleBundle' -count=1
```

Expected: FAIL. Recorded:

```text
--- FAIL: TestSaveCodeSceneFailureNeverLeavesAStaleBundle (0.01s)
    --- FAIL: TestSaveCodeSceneFailureNeverLeavesAStaleBundle/bundle_write_fails (0.01s)
        project_test.go:465: status is ok but the bundle was not built from the current source
    --- FAIL: TestSaveCodeSceneFailureNeverLeavesAStaleBundle/old_bundle_cannot_be_removed_before_a_failing_build (0.00s)
        project_test.go:465: status is ok but the bundle was not built from the current source
FAIL
FAIL	aurago/internal/storymaker	0.044s
```

- [ ] **Step 3: Implement**

In `internal/storymaker/project.go` (about line 321 of the integration tree), replace:

```
// code size limit is refused with ErrTooLarge before anything is written.
```

with:

```
// code size limit is refused with ErrTooLarge before anything is written.
//
// The previous bundle is removed before the source changes: a render loads the bundle of every
// scene whose status is ok, so a save that fails half way must leave the old source with its own
// bundle (the removal failed) or the new source without a bundle (a later step failed), never a
// bundle that was built from older source. A refused precondition is checked first, so it keeps
// the old bundle.
```

In `internal/storymaker/project.go` (about line 343 of the integration tree), replace:

```
	etag, err := p.st.WriteFile(ctx, p.codePath(id, sceneID, ".js"), source, ifMatch)
	if err != nil {
		return "", BundleResult{}, err
	}
	res := BundleCodeScene(source, sceneID, p.lim.MaxCodeBytes)
```

with:

```
	if err := p.checkCodePrecondition(ctx, id, sceneID, ifMatch); err != nil {
		return "", BundleResult{}, err
	}
	if err := p.st.Remove(ctx, p.codePath(id, sceneID, ".bundle.js")); err != nil {
		return "", BundleResult{}, fmt.Errorf("storymaker: remove bundle: %w", err)
	}
	etag, err := p.st.WriteFile(ctx, p.codePath(id, sceneID, ".js"), source, ifMatch)
	if err != nil {
		return "", BundleResult{}, err
	}
	res := BundleCodeScene(source, sceneID, p.lim.MaxCodeBytes)
```

In `internal/storymaker/project.go` (about line 363 of the integration tree), replace:

```
	if err := p.st.Remove(ctx, p.codePath(id, sceneID, ".bundle.js")); err != nil {
		return etag, res, fmt.Errorf("storymaker: remove bundle: %w", err)
	}
	b, err := json.MarshalIndent(res.Errors, "", "  ")
```

with:

```
	b, err := json.MarshalIndent(res.Errors, "", "  ")
```

In `internal/storymaker/project.go` (about line 373 of the integration tree), replace:

```
func (p *ProjectStore) codePath(id, sceneID, ext string) string {
```

with:

```
// checkCodePrecondition applies the ifMatch rule of Store.WriteFile to the source file, so that a
// refused save is refused before the old bundle is touched. The write repeats the check under the
// store's own lock.
func (p *ProjectStore) checkCodePrecondition(ctx context.Context, id, sceneID, ifMatch string) error {
	if ifMatch == "" {
		return nil
	}
	_, cur, err := p.st.ReadFile(ctx, p.codePath(id, sceneID, ".js"))
	exists := err == nil
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("storymaker: read code scene: %w", err)
	}
	if ifMatch == MustNotExist {
		if exists {
			return ErrConflict
		}
		return nil
	}
	if !exists || cur != ifMatch {
		return ErrConflict
	}
	return nil
}

func (p *ProjectStore) codePath(id, sceneID, ext string) string {
```

In `internal/storymaker/project.go` (about line 7 of the integration tree), replace:

```
	"fmt"
	"path"
```

with:

```
	"fmt"
	"io/fs"
	"path"
```

In `internal/storymaker/AGENTS.md`, after the bullet that starts with "`Store` is the only file access of `ProjectStore`", add:

```markdown
- `SaveCodeScene` checks the precondition itself, removes the old bundle, and only then writes the source and the new
  bundle. A save that fails half way therefore leaves the old source with its own bundle or the new source without one, never a
  bundle built from older source: a render loads the bundle of every scene whose status is ok.
```

- [ ] **Step 4: Run the tests to verify they pass**

```bash
gofmt -l internal/storymaker
go test ./internal/storymaker/... -count=1
```

Expected: PASS. Recorded:

```text
ok  	aurago/internal/storymaker	0.210s
ok  	aurago/internal/storymaker/render	11.182s
```

- [ ] **Step 5: Commit**

```bash
git add internal/storymaker/project.go internal/storymaker/project_test.go internal/storymaker/AGENTS.md
git commit -m "fix(storymaker): a failed code-scene save never leaves a bundle older than its source" -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

### Task I8: The browser setup of a first export is bounded

On a host without a usable Chrome the first export downloads the pinned Chromium (about 190 to 270 MB). `execute` called `Browsers.Ensure` with the job context, which has no deadline, and the render clock (`render.timeout_seconds`) starts only later, in `wait`. The download used an HTTP client without any timeout. A peer that keeps the connection open but stops sending (a proxy that stalls, a CDN node that hangs) blocked `io.Copy` for as long as the connection lived; the single render worker never returned, the job stayed in "starting", 16 more exports queued and the 17th got 429 `busy`. Only a manual cancel freed the worker (a probe against a server that stops after 100 bytes reproduced it).

Two bounds, because a fixed cap would abort legitimate slow downloads (the archive needs about 2.4 Mbit/s to finish in 15 minutes): the download gives up when no byte arrives for 60 s (a timer that every read of the body resets, started before the request so the wait for the response headers counts as well), and the whole setup runs under `render.timeout_seconds` (default 3600), which also bounds a connection that trickles one byte a minute. `OpenPage` keeps the job context: rod ties the browser's lifetime to the context it gets. `Browsers.install` holds `dlMu` for the download, so a second export no longer waits behind a stalled first one for ever either.

Callers (grep): `Ensure` is called from `render.go` and the tests; `download` only from `install`.

**Files:**
- Modify: `internal/storymaker/render/browser.go` (`Browsers`, `download`, new `tickReader`)
- Modify: `internal/storymaker/render/render.go` (`execute`)
- Modify: `internal/storymaker/render/AGENTS.md`
- Test: `internal/storymaker/render/browser_test.go`, `internal/storymaker/render/render_test.go`

- [ ] **Step 1: Write the failing tests**

In `internal/storymaker/render/browser_test.go` add `"strconv"` and `"time"` to the imports, then append:

```go
// stalledArchiveServer sends the headers and the first bytes of the archive and then goes silent
// without closing the connection.
func stalledArchiveServer(t *testing.T, archive []byte) *httptest.Server {
	t.Helper()
	release := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", strconv.Itoa(len(archive)))
		_, _ = w.Write(archive[:100])
		w.(http.Flusher).Flush()
		select {
		case <-r.Context().Done():
		case <-release:
		}
	}))
	t.Cleanup(func() { close(release); srv.Close() })
	return srv
}

func TestPinnedChromiumDownloadGivesUpOnAStalledConnection(t *testing.T) {
	archive := fakeChromiumZip(t, "chrome-x", nil)
	srv := stalledArchiveServer(t, archive)
	b, _ := newTestBrowsers(t)
	b.pins = map[string]PinnedChromium{b.platform: pinFor(srv.URL, archive)}
	b.idleTimeout = 300 * time.Millisecond

	// The guard only turns a missing stall timeout into a failure instead of a hung test.
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	start := time.Now()
	st, bin := b.Ensure(ctx, "", nil)
	if st.Ready || bin != "" || !strings.Contains(st.Issue, "stalled") {
		t.Fatalf("Ensure after a stall: %+v %q", st, bin)
	}
	if took := time.Since(start); took > 10*time.Second {
		t.Fatalf("the stalled download held the caller for %s", took)
	}
	if parts, _ := filepath.Glob(filepath.Join(b.dir, "chromium-*")); len(parts) != 0 {
		t.Fatalf("a stalled download left %v", parts)
	}
	// The lock is free again: a second export is not stuck behind the first.
	done := make(chan struct{})
	go func() { b.Ensure(ctx, "", nil); close(done) }()
	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("the download lock was not released")
	}
}

// The stall limit is an idle limit, not a deadline for the whole download: a slow link that keeps
// delivering must finish.
func TestPinnedChromiumDownloadKeepsASlowConnectionAlive(t *testing.T) {
	archive := fakeChromiumZip(t, "chrome-x", nil)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", strconv.Itoa(len(archive)))
		for off := 0; off < len(archive); off += 60 {
			end := min(off+60, len(archive))
			_, _ = w.Write(archive[off:end])
			w.(http.Flusher).Flush()
			time.Sleep(120 * time.Millisecond)
		}
	}))
	t.Cleanup(srv.Close)
	b, _ := newTestBrowsers(t)
	b.pins = map[string]PinnedChromium{b.platform: pinFor(srv.URL, archive)}
	b.idleTimeout = 500 * time.Millisecond
	start := time.Now()
	st, bin := b.Ensure(context.Background(), "", nil)
	if !st.Ready || bin == "" {
		t.Fatalf("a slow download was cut: %+v", st)
	}
	if took := time.Since(start); took < b.idleTimeout {
		t.Fatalf("the download finished in %s, which does not exercise the idle limit (%s)", took, b.idleTimeout)
	}
}
```

Append to `internal/storymaker/render/render_test.go`:

```go
// The browser setup of a first export (the pinned Chromium download) runs before the render clock
// starts, so the render timeout has to bound it separately: a peer that stops sending must not
// hold the single render worker.
func TestRunBoundsTheBrowserSetupByTheRenderTimeout(t *testing.T) {
	fx := newRunFixture(t)
	archive := fakeChromiumZip(t, "chrome-x", nil)
	srv := stalledArchiveServer(t, archive)
	b := fx.deps.Browsers
	b.lookPath = func() (string, bool) { return "", false }
	b.pins = map[string]PinnedChromium{b.platform: pinFor(srv.URL, archive)}
	b.idleTimeout = time.Hour // only the render timeout can end this download
	fx.settings.TimeoutSeconds = 1
	fx.playing(playOptions{frames: 10})
	start := time.Now()
	job := fx.render(t)
	if job.Status != StatusFailed || job.Error == nil || job.Error.Code != CodeBrowserUnavailable || !strings.Contains(job.Error.Message, "downloaded") {
		t.Fatalf("job = %+v %+v", job, job.Error)
	}
	if took := time.Since(start); took > 20*time.Second {
		t.Fatalf("the setup ran %s although render.timeout_seconds is 1", took)
	}
}
```

- [ ] **Step 2: Run the tests to verify they fail**

```bash
go test ./internal/storymaker/render/ -run 'TestPinnedChromiumDownload' -count=1
```

Expected: FAIL to compile (`b.idleTimeout undefined`). To see the behaviour fail, add only the field `idleTimeout time.Duration` to `Browsers` first (the first block of step 3) and run:

```bash
go test ./internal/storymaker/render/ -run 'TestPinnedChromiumDownloadGivesUpOnAStalledConnection' -count=1
go test ./internal/storymaker/render/ -run 'TestRunBoundsTheBrowserSetupByTheRenderTimeout' -count=1 -timeout 45s
```

Recorded (the stalled download holds the caller until the guard context of 20 s ends; the render job never ends and the run is stopped by the 45 s test timeout):

```text
--- FAIL: TestPinnedChromiumDownloadGivesUpOnAStalledConnection (20.01s)
    browser_test.go:395: Ensure after a stall: {Ready:false Source:pinned Issue:The pinned Chromium could not be downloaded: download: context deadline exceeded CanDownload:true Version:} ""
FAIL
panic: test timed out after 45s
FAIL	aurago/internal/storymaker/render	45.038s
```

- [ ] **Step 3: Implement**

In `internal/storymaker/render/browser.go` (about line 99 of the integration tree), replace:

```
	client      *http.Client
	pins        map[string]PinnedChromium
	platform    string
```

with:

```
	client      *http.Client
	pins        map[string]PinnedChromium
	platform    string
	idleTimeout time.Duration // download stall limit; 0 selects downloadIdleTimeout
```

In `internal/storymaker/render/browser.go` (about line 69 of the integration tree), replace:

```
	probeFailCacheFor   = 30 * time.Second
	browserProbeTimeout = 45 * time.Second
)
```

with:

```
	probeFailCacheFor   = 30 * time.Second
	browserProbeTimeout = 45 * time.Second
	// downloadIdleTimeout is how long the pinned Chromium download may go without receiving a byte
	// (the wait for the response headers included) before it is given up. The size of the archive
	// is not limited by it: a slow link that keeps delivering is left alone.
	downloadIdleTimeout = 60 * time.Second
)
```

In `internal/storymaker/render/browser.go` (about line 416 of the integration tree), replace:

```
func (b *Browsers) download(ctx context.Context, pin PinnedChromium, dest string, progress BrowserProgress) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, pin.URL, nil)
	if err != nil {
		return err
	}
	res, err := b.client.Do(req)
	if err != nil {
		return fmt.Errorf("download: %w", err)
	}
	defer res.Body.Close()
```

with:

```
func (b *Browsers) download(ctx context.Context, pin PinnedChromium, dest string, progress BrowserProgress) error {
	idle := b.idleTimeout
	if idle <= 0 {
		idle = downloadIdleTimeout
	}
	// A peer that keeps the connection open but sends nothing would hold the caller (the single
	// render worker) forever: the timer cancels the request when no byte arrived for idle.
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	var stalled atomic.Bool
	timer := time.AfterFunc(idle, func() { stalled.Store(true); cancel() })
	defer timer.Stop()
	fail := func(err error) error {
		if stalled.Load() {
			return fmt.Errorf("download: no data for %s, the connection stalled", idle)
		}
		return fmt.Errorf("download: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, pin.URL, nil)
	if err != nil {
		return err
	}
	res, err := b.client.Do(req)
	if err != nil {
		return fail(err)
	}
	defer res.Body.Close()
	timer.Reset(idle)
```

In `internal/storymaker/render/browser.go` (about line 456 of the integration tree), replace:

```
	n, copyErr := io.Copy(io.MultiWriter(out, hash, counter), io.LimitReader(res.Body, pin.Size+1))
	closeErr := out.Close()
	if copyErr != nil {
		return fmt.Errorf("download: %w", copyErr)
	}
```

with:

```
	body := tickReader{r: io.LimitReader(res.Body, pin.Size+1), tick: func() { timer.Reset(idle) }}
	n, copyErr := io.Copy(io.MultiWriter(out, hash, counter), body)
	closeErr := out.Close()
	if copyErr != nil {
		return fail(copyErr)
	}
```

In `internal/storymaker/render/browser.go` (about line 474 of the integration tree), replace:

```
type countingWriter struct {
```

with:

```
// tickReader calls tick after every read that delivered bytes.
type tickReader struct {
	r    io.Reader
	tick func()
}

func (t tickReader) Read(p []byte) (int, error) {
	n, err := t.r.Read(p)
	if n > 0 {
		t.tick()
	}
	return n, err
}

type countingWriter struct {
```

In `internal/storymaker/render/browser.go` (about line 17 of the integration tree), replace:

```
	"strings"
	"sync"
	"time"
```

with:

```
	"strings"
	"sync"
	"sync/atomic"
	"time"
```

In `internal/storymaker/render/render.go` (about line 320 of the integration tree), replace:

```
	status, bin := r.deps.Browsers.Ensure(ctx, run.st.BrowserPath, func(done, total int64) {
		if total > 0 {
			h.SetProgress(0.05 * float64(done) / float64(total))
		}
	})
	if !status.Ready {
```

with:

```
	// A first export may download the pinned Chromium. That happens before the render clock starts
	// (wait), so the render timeout bounds it here; OpenPage keeps the job context, because rod ties
	// the browser's lifetime to the context it is given.
	setupTimeout := time.Duration(run.st.TimeoutSeconds) * time.Second
	if setupTimeout <= 0 {
		setupTimeout = time.Hour
	}
	setupCtx, cancelSetup := context.WithTimeout(ctx, setupTimeout)
	status, bin := r.deps.Browsers.Ensure(setupCtx, run.st.BrowserPath, func(done, total int64) {
		if total > 0 {
			h.SetProgress(0.05 * float64(done) / float64(total))
		}
	})
	cancelSetup()
	if !status.Ready {
```

In `internal/storymaker/render/AGENTS.md`, make two changes:

In `internal/storymaker/render/AGENTS.md` (about line 43 of the integration tree), replace:

```
  browser is `ready` only after a cached headless launch probe. Production launches never set `NoSandbox` (`newLauncher` deletes the flag rod adds
  inside containers); `SetNoSandboxForTests` is the only way to set it and nothing outside tests calls it.
```

with:

```
  browser is `ready` only after a cached headless launch probe. Production launches never set `NoSandbox` (`newLauncher` deletes the flag rod adds
  inside containers); `SetNoSandboxForTests` is the only way to set it and nothing outside tests calls it. The pinned download gives up when no
  byte arrives for 60 s (`downloadIdleTimeout`; a slow link that keeps delivering is left alone).
```

In `internal/storymaker/render/AGENTS.md` (about line 46 of the integration tree), replace:

```
- **Timeouts**: report within 3 minutes of the navigation,
```

with:

```
- **Timeouts**: the browser setup of a first export (the Chromium download) runs under `render.timeout_seconds` before the render clock starts
  (`OpenPage` keeps the job context: the browser lives as long as it). Report within 3 minutes of the navigation,
```

- [ ] **Step 4: Run the tests to verify they pass**

```bash
gofmt -l internal/storymaker
go vet ./internal/storymaker/render/
go test ./internal/storymaker/render/ -run 'TestPinnedChromium|TestRunBoundsTheBrowserSetupByTheRenderTimeout' -count=1 -v
```

Expected: PASS. Recorded:

```text
--- PASS: TestPinnedChromiumDownloadVerifiesAndInstalls (0.01s)
--- PASS: TestPinnedChromiumRefusesWrongChecksumSizeAndStatus (0.01s)
--- PASS: TestPinnedChromiumDownloadGivesUpOnAStalledConnection (0.61s)
--- PASS: TestPinnedChromiumDownloadKeepsASlowConnectionAlive (0.97s)
--- PASS: TestRunBoundsTheBrowserSetupByTheRenderTimeout (1.07s)
PASS
ok  	aurago/internal/storymaker/render	2.715s
```

- [ ] **Step 5: Commit**

```bash
git add internal/storymaker/render/browser.go internal/storymaker/render/render.go internal/storymaker/render/browser_test.go internal/storymaker/render/render_test.go internal/storymaker/render/AGENTS.md
git commit -m "fix(storymaker): bound the pinned Chromium download and the browser setup of a first export" -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

### Task I9: The render browser has no DevTools port

`newLauncher` never overrode rod's `--remote-debugging-port=0`, so every render Chrome listened on a random loopback port. Loopback is shared by every account of the host. While a render ran, any process (another Windows or Unix account included) could find the port, read `GET /json/list` (it names the render tab URL `http://127.0.0.1:<port>/<prefix>/`, which is the secret of the listener) and attach over the websocket: `Runtime.evaluate` in the tab bypasses the page CSP and the CDP egress guard (both are state of the guard's own session), the prefix lets it read the snapshot and POST frames that go straight into FFmpeg (the exported MP4 can be replaced), and a navigation to `file://` would read files of the AuraGo user. The first idea, navigating to a placeholder URL, is not enough: the prefix stays in the page's DOM and the control path stays open. The control path itself has to go.

The fix is Chrome's `--remote-debugging-pipe`: DevTools messages travel over two pipes that only the browser inherits (descriptors 3 and 4; on Windows two inherited handles named by `--remote-debugging-io-pipes=<read>,<write>`), NUL-terminated JSON. rod's launcher only knows the websocket, so `startBrowserProcess` builds the flag set with `newLauncher` (minus the port and the leakless helper, plus the pipe flag), starts the process itself, and hands rod a `cdp.Client` over the pipe (`rod.Browser.Client`). What the launcher did for us is done here: the last 4 KiB of the browser's output go into the start-up error (`launchIssue` reads "No usable sandbox" and "shared libraries" from it), a silent browser is killed after 60 s, `Close` closes the input (a browser quits when its input ends, so a crashed AuraGo leaves no browser behind; the old leakless helper is no longer needed), waits 5 s, kills the process group (Unix) or the tree (`taskkill /T`, Windows), and removes the profile with retries. Only two places start a render browser: `probeLaunch` and `openRodSession`.

Measured: Chrome 156 on Windows 11 and Chromium 156 on Debian 12 (Docker, Go test binary built for linux/amd64) both take the pipe; the real render of `firewall.json` passes on both (Linux: 342 frames in 33.4 s, 10.2 fps overall, FFmpeg 5.1.9). `GET /json/list` has nothing to read any more: `DevToolsActivePort` is not written and the browser tree has no listening socket (the check finds the DevTools port of the old launcher in every sample).

Callers (grep): `newLauncher` is called from `browserArgs` only now; `probeLaunch` from `Browsers.probe`; `openRodSession` from `openSession` and `egress_smoke_test.go`. `session.l` (the rod launcher) was read by one spike test (`pid = s.l.PID()`), which now reads `s.proc.PID()`.

**Files:**
- Create: `internal/storymaker/render/devtools_pipe.go`, `devtools_pipe_unix.go`, `devtools_pipe_windows.go`
- Modify: `internal/storymaker/render/browser.go` (`newLauncher`, `probeLaunch`, imports), `session.go`
- Modify: `internal/storymaker/render/AGENTS.md`
- Test: `internal/storymaker/render/devtools_pipe_test.go` (create), `browser_smoke_test.go`

- [ ] **Step 1: Write the failing test**

Append to `internal/storymaker/render/browser_smoke_test.go` (the real-browser guard of the first form of this fix: Chrome writes `DevToolsActivePort` into the profile when it opens a port):

```go
// The render browser must not open a DevTools port. Such a port listens on loopback, which every
// account of the host can reach: GET /json/list names the render URL (and with it the secret prefix of
// the listener), and the websocket drives the browser, past the page CSP and the CDP guard. Chrome
// writes DevToolsActivePort into the profile when it opens a port; the pipe transport has none.
func TestRenderBrowserOpensNoDevToolsPort(t *testing.T) {
	chrome := browserSmoke(t)
	profile := filepath.Join(t.TempDir(), "profile")
	s, err := openRodSession(context.Background(), sessionOptions{Bin: chrome, Profile: profile, Base: "http://127.0.0.1:1/x", Width: 640, Height: 360})
	if err != nil {
		t.Fatalf("session: %v", err)
	}
	defer s.Close()
	res, err := s.page.Timeout(30 * time.Second).Eval(`() => 6 * 7`)
	if err != nil || res.Value.Int() != 42 {
		t.Fatalf("the tab does not run scripts: %v %v", res, err)
	}
	if _, err := os.Stat(filepath.Join(profile, "DevToolsActivePort")); !os.IsNotExist(err) {
		t.Fatalf("the browser opened a DevTools port (DevToolsActivePort: %v)", err)
	}
}
```

- [ ] **Step 2: Run the test to verify it fails**

```bash
AURAGO_RUN_BROWSER_SMOKE=1 go test ./internal/storymaker/render/ -run 'TestRenderBrowserOpensNoDevToolsPort' -count=1 -v
```

Expected: FAIL. Recorded:

```text
=== RUN   TestRenderBrowserOpensNoDevToolsPort
    browser_smoke_test.go:573: the browser opened a DevTools port (DevToolsActivePort: <nil>)
--- FAIL: TestRenderBrowserOpensNoDevToolsPort (1.59s)
FAIL
FAIL	aurago/internal/storymaker/render	1.634s
```

- [ ] **Step 3: Implement**

`internal/storymaker/render/devtools_pipe.go`:

```go
package render

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/cdp"
	"github.com/go-rod/rod/lib/launcher/flags"
)

// The render browser talks DevTools over a pipe (--remote-debugging-pipe), never over a port. A
// DevTools port listens on loopback, which every account of the host can reach: GET /json/list names
// the render URL (and with it the secret prefix of the listener) and the websocket drives the browser
// past the page CSP and the CDP guard. The pipe is inherited by the browser alone. rod's launcher
// only knows the websocket, so the browser is started here and handed to rod as a CDP client.

// browserStartTimeout bounds the wait for the first answer of a freshly started browser; browserExitWait
// is how long Close lets the browser quit by itself (it does when its input closes) before it is killed.
var (
	browserStartTimeout = 60 * time.Second
	browserExitWait     = 5 * time.Second
)

// pipeConn is the CDP transport over the pipe: JSON messages, each terminated by a NUL byte, in both
// directions. It implements cdp.WebSocketable.
type pipeConn struct {
	r      *bufio.Reader
	w      io.Writer
	mu     sync.Mutex
	closed atomic.Bool
}

func newPipeConn(r io.Reader, w io.Writer) *pipeConn {
	return &pipeConn{r: bufio.NewReaderSize(r, 64<<10), w: w}
}

// Read returns the next message without its terminator. Once it failed the connection is closed and
// every later Send fails too, so a call after the browser went away returns instead of waiting.
func (c *pipeConn) Read() ([]byte, error) {
	msg, err := c.r.ReadBytes(0)
	if err != nil {
		c.closed.Store(true)
		return nil, err
	}
	return msg[:len(msg)-1], nil
}

// Send writes one message and its terminator in a single write.
func (c *pipeConn) Send(data []byte) error {
	if c.closed.Load() {
		return io.ErrClosedPipe
	}
	frame := make([]byte, 0, len(data)+1)
	frame = append(append(frame, data...), 0)
	c.mu.Lock()
	defer c.mu.Unlock()
	_, err := c.w.Write(frame)
	return err
}

// browserArgs is the command line of one render browser: the flags of newLauncher, with the pipe in
// place of a DevTools port. The platform adds how the browser finds its end of the pipe.
func browserArgs(o launchOptions) []string {
	l := newLauncher(o)
	l.Delete(flags.RemoteDebuggingPort)
	l.Delete(flags.Leakless)
	l.Set("remote-debugging-pipe")
	return l.FormatArgs()
}

// browserProcess is a running render browser.
type browserProcess struct {
	cmd      *exec.Cmd
	in       *os.File // our end of the commands: the browser reads the other end
	out      *os.File // our end of the answers: the browser writes the other end
	exited   chan struct{}
	output   *tailBuffer // the last bytes the browser wrote: why it exited at start-up (no usable sandbox, missing libraries)
	profile  string
	once     sync.Once
	timedOut atomic.Bool
}

// PID is the process id of the browser.
func (p *browserProcess) PID() int { return p.cmd.Process.Pid }

// startBrowserProcess starts the browser with a DevTools pipe and connects rod to it. Nothing
// listens on any port. The caller closes the process (after browser.Close for a clean shutdown).
func startBrowserProcess(ctx context.Context, o launchOptions) (*rod.Browser, *browserProcess, error) {
	if o.Bin == "" {
		return nil, nil, errors.New("no browser executable")
	}
	inR, inW, err := os.Pipe() // the browser reads inR, we write inW
	if err != nil {
		return nil, nil, err
	}
	outR, outW, err := os.Pipe() // the browser writes outW, we read outR
	if err != nil {
		_ = inR.Close()
		_ = inW.Close()
		return nil, nil, err
	}
	closeAll := func() {
		for _, f := range []*os.File{inR, inW, outR, outW} {
			_ = f.Close()
		}
	}
	cmd := exec.Command(o.Bin, browserArgs(o)...)
	output := &tailBuffer{limit: 4096}
	cmd.Stdout, cmd.Stderr = output, output
	cmd.WaitDelay = 5 * time.Second // the browser's helper processes may hold stdout open
	if err := attachDevToolsPipe(cmd, inR, outW); err != nil {
		closeAll()
		return nil, nil, fmt.Errorf("prepare the browser pipe: %w", err)
	}
	if err := cmd.Start(); err != nil {
		closeAll()
		return nil, nil, fmt.Errorf("start the browser: %w", err)
	}
	// The browser has its ends; ours must be the only other copies, so that it sees the end of its input
	// when we close ours.
	_ = inR.Close()
	_ = outW.Close()
	p := &browserProcess{cmd: cmd, in: inW, out: outR, exited: make(chan struct{}), output: output, profile: o.UserDataDir}
	go func() {
		_ = cmd.Wait()
		close(p.exited)
	}()

	client := cdp.New().Start(newPipeConn(outR, inW))
	browser := rod.New().Context(ctx).Client(client)
	timer := time.AfterFunc(browserStartTimeout, func() {
		p.timedOut.Store(true)
		killProcessTree(cmd.Process)
	})
	err = browser.Connect()
	timer.Stop()
	if err != nil {
		failure := p.startFailure(err)
		p.Close()
		return nil, nil, failure
	}
	return browser, p, nil
}

// startFailure says why the browser did not come up: what it wrote before it exited, or that it
// stayed silent.
func (p *browserProcess) startFailure(cause error) error {
	select {
	case <-p.exited:
	case <-time.After(2 * time.Second):
	}
	text := strings.TrimSpace(strings.ToValidUTF8(p.output.String(), "?"))
	switch {
	case p.timedOut.Load():
		return fmt.Errorf("the browser did not answer within %s", browserStartTimeout)
	case text != "":
		return fmt.Errorf("the browser exited at start-up: %s", text)
	}
	return fmt.Errorf("connect to the browser: %w", cause)
}

// Close shuts the browser down: it closes the commands pipe (a browser quits when its input ends),
// waits for it, kills what is left and removes the profile. It may be called more than once.
func (p *browserProcess) Close() {
	p.once.Do(func() {
		_ = p.in.Close()
		select {
		case <-p.exited:
		case <-time.After(browserExitWait):
			killProcessTree(p.cmd.Process)
			select {
			case <-p.exited:
			case <-time.After(5 * time.Second):
			}
		}
		_ = p.out.Close()
		if p.profile != "" {
			removeAllWithRetry(p.profile)
		}
	})
}

// removeAllWithRetry removes a folder the browser has just stopped using; on Windows its files can stay
// locked for a moment.
func removeAllWithRetry(dir string) {
	for i := 0; i < 20; i++ {
		if err := os.RemoveAll(dir); err == nil {
			return
		}
		time.Sleep(100 * time.Millisecond)
	}
}
```

`internal/storymaker/render/devtools_pipe_unix.go`:

```go
//go:build !windows

package render

import (
	"os"
	"os/exec"
	"syscall"
)

// attachDevToolsPipe gives the browser its ends of the pipe as file descriptors 3 (commands in) and 4
// (answers out), which is where --remote-debugging-pipe looks, and puts it in its own process group so
// that a kill reaches its helper processes.
func attachDevToolsPipe(cmd *exec.Cmd, in, out *os.File) error {
	cmd.ExtraFiles = []*os.File{in, out}
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	return nil
}

// killProcessTree kills the browser and its process group.
func killProcessTree(p *os.Process) {
	if p == nil {
		return
	}
	_ = syscall.Kill(-p.Pid, syscall.SIGKILL)
	_ = p.Kill()
}
```

`internal/storymaker/render/devtools_pipe_windows.go`:

```go
//go:build windows

package render

import (
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"syscall"
)

// attachDevToolsPipe passes the two pipe ends to the browser as inherited handles and names them in
// --remote-debugging-io-pipes=<read>,<write>, which is how Chromium finds its DevTools pipe on Windows.
func attachDevToolsPipe(cmd *exec.Cmd, in, out *os.File) error {
	inHandle, outHandle := syscall.Handle(in.Fd()), syscall.Handle(out.Fd())
	for _, h := range []syscall.Handle{inHandle, outHandle} {
		if err := syscall.SetHandleInformation(h, syscall.HANDLE_FLAG_INHERIT, syscall.HANDLE_FLAG_INHERIT); err != nil {
			return err
		}
	}
	cmd.Args = append(cmd.Args, fmt.Sprintf("--remote-debugging-io-pipes=%d,%d", inHandle, outHandle))
	cmd.SysProcAttr = &syscall.SysProcAttr{
		CreationFlags:              syscall.CREATE_NEW_PROCESS_GROUP,
		AdditionalInheritedHandles: []syscall.Handle{inHandle, outHandle},
	}
	return nil
}

// killProcessTree kills the browser and the processes it started.
func killProcessTree(p *os.Process) {
	if p == nil {
		return
	}
	kill := exec.Command("taskkill", "/T", "/F", "/PID", strconv.Itoa(p.Pid))
	kill.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	_ = kill.Run()
	_ = p.Kill()
}
```

In `internal/storymaker/render/browser.go` (about line 300 of the integration tree), replace:

```
// newLauncher builds the rod launcher every Storymaker browser launch uses. It never sets
// NoSandbox (rod adds --no-sandbox by itself inside containers; that is removed here), except
// through the test hook.
func newLauncher(ctx context.Context, o launchOptions) *launcher.Launcher {
	l := launcher.New().Context(ctx).Bin(o.Bin).Headless(true)
```

with:

```
// newLauncher builds the rod launcher whose flags every Storymaker browser launch uses (the browser
// itself is started by startBrowserProcess, which swaps the DevTools port for a pipe). It never sets
// NoSandbox (rod adds --no-sandbox by itself inside containers; that is removed here), except
// through the test hook.
func newLauncher(o launchOptions) *launcher.Launcher {
	l := launcher.New().Bin(o.Bin).Headless(true)
```

In `internal/storymaker/render/browser.go` (about line 345 of the integration tree), replace:

```
	defer os.RemoveAll(profile)
	l := newLauncher(ctx, launchOptions{Bin: bin, UserDataDir: profile})
	controlURL, err := l.Launch()
	if err != nil {
		return "", err
	}
	defer func() { l.Kill(); l.Cleanup() }()
	browser := rod.New().Context(ctx).ControlURL(controlURL)
	if err := browser.Connect(); err != nil {
		return "", err
	}
	defer func() {
```

with:

```
	defer os.RemoveAll(profile)
	browser, proc, err := startBrowserProcess(ctx, launchOptions{Bin: bin, UserDataDir: profile})
	if err != nil {
		return "", err
	}
	defer proc.Close()
	defer func() {
```

In `internal/storymaker/render/browser.go` (about line 22 of the integration tree), replace:

```
	"aurago/internal/fileutil"
	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/launcher"
```

with:

```
	"aurago/internal/fileutil"
	"github.com/go-rod/rod/lib/launcher"
```

In `internal/storymaker/render/session.go` (about line 11 of the integration tree), replace:

```
	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/launcher"
	"github.com/go-rod/rod/lib/proto"
```

with:

```
	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/proto"
```

In `internal/storymaker/render/session.go` (about line 54 of the integration tree), replace:

```
type session struct {
	l       *launcher.Launcher
	browser *rod.Browser
```

with:

```
type session struct {
	proc    *browserProcess
	browser *rod.Browser
```

In `internal/storymaker/render/session.go` (about line 75 of the integration tree), replace:

```
	l := newLauncher(ctx, launchOptions{Bin: o.Bin, UserDataDir: o.Profile, ProxyAddr: o.ProxyAddr, WindowSize: [2]int{o.Width, o.Height}})
	controlURL, err := l.Launch()
	if err != nil {
		return nil, fmt.Errorf("launch the browser: %w", err)
	}
	s := &session{l: l, opts: o, guard: newEgressGuard(o.Base), crashed: make(chan struct{})}
	browser := rod.New().Context(ctx).ControlURL(controlURL)
	if err := browser.Connect(); err != nil {
		s.closeProcess()
		return nil, fmt.Errorf("connect to the browser: %w", err)
	}
	s.browser = browser
```

with:

```
	browser, proc, err := startBrowserProcess(ctx, launchOptions{Bin: o.Bin, UserDataDir: o.Profile, ProxyAddr: o.ProxyAddr, WindowSize: [2]int{o.Width, o.Height}})
	if err != nil {
		return nil, fmt.Errorf("launch the browser: %w", err)
	}
	s := &session{proc: proc, browser: browser, opts: o, guard: newEgressGuard(o.Base), crashed: make(chan struct{})}
```

In `internal/storymaker/render/session.go` (about line 173 of the integration tree), replace:

```
func (s *session) closeProcess() {
	s.l.Kill()
	s.l.Cleanup()
}
```

with:

```
func (s *session) closeProcess() { s.proc.Close() }
```

In `internal/storymaker/render/browser_smoke_test.go` (about line 305 of the integration tree, inside `TestTransportSpike`; `session.l` no longer exists, the process handle is `session.proc`), replace:

```
					pid = s.l.PID()
```

with:

```
					pid = s.proc.PID()
```

In `internal/storymaker/render/AGENTS.md`, add the bullet after the Egress bullet:

In `internal/storymaker/render/AGENTS.md` (about line 33 of the integration tree), replace:

```
  layer alone and all together against a loopback probe server.
```

with:

```
  layer alone and all together against a loopback probe server.
- **DevTools pipe** (`devtools_pipe*.go`): the render browser has no DevTools port. A port on loopback is reachable by every account of the
  host: `GET /json/list` names the render URL (the secret prefix) and the websocket drives the browser past the CSP and the CDP guard. So the
  browser starts with `--remote-debugging-pipe` (descriptors 3 and 4; on Windows the handles named by `--remote-debugging-io-pipes`) and rod
  gets a pipe `cdp.Client`, because rod's launcher only knows the websocket. The browser quits when its input closes, so a crashed server
  leaves no browser behind. `newLauncher` only builds the flag set. `TestRenderBrowserOpensNoDevToolsPort` checks `DevToolsActivePort` and the
  listening sockets of the browser tree; `devtools_pipe_test.go` covers the framing, start-up failures (with the browser's own output in the
  error, which `launchIssue` reads) and the kill path with the test binary as a fake browser, on Windows and Linux.
```

The spec (`docs/superpowers/specs/2026-11-01-storymaker-design.md`) calls the prefix "defence in depth" and says nothing about the browser's DevTools endpoint. When the plan is committed, add to its Security section ("Render listener") one sentence: "The render browser is started with `--remote-debugging-pipe`, so no process but the server can reach its DevTools protocol; a DevTools port on loopback would be open to every local account." (Contract section 12, item 10, records the deviation.)

- [ ] **Step 4: Fake-browser unit tests, the socket check, and the full suite**

`internal/storymaker/render/devtools_pipe_test.go` (the test binary itself is the fake browser through `TestMain`; it finds the pipe like Chromium does, so the same tests cover the Unix and the Windows plumbing without a Chrome):

```go
package render

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/go-rod/rod/lib/proto"
	"github.com/shirou/gopsutil/v4/process"
)

// TestMain lets the test binary stand in for the browser: the tests below start it with
// SM_FAKE_BROWSER set, and it speaks the DevTools pipe protocol (or fails) instead of running tests.
func TestMain(m *testing.M) {
	if mode := os.Getenv("SM_FAKE_BROWSER"); mode != "" {
		os.Exit(runFakeBrowser(mode))
	}
	os.Exit(m.Run())
}

// fakePipes finds the browser's end of the DevTools pipe like Chromium does: file descriptors 3 and 4,
// or the handles named by --remote-debugging-io-pipes on Windows.
func fakePipes() (in, out *os.File) {
	if runtime.GOOS != "windows" {
		return os.NewFile(3, "devtools-in"), os.NewFile(4, "devtools-out")
	}
	for _, a := range os.Args {
		if v, ok := strings.CutPrefix(a, "--remote-debugging-io-pipes="); ok {
			parts := strings.Split(v, ",")
			if len(parts) == 2 {
				r, _ := strconv.ParseUint(parts[0], 10, 64)
				w, _ := strconv.ParseUint(parts[1], 10, 64)
				return os.NewFile(uintptr(r), "devtools-in"), os.NewFile(uintptr(w), "devtools-out")
			}
		}
	}
	return nil, nil
}

func runFakeBrowser(mode string) int {
	if mode == "sandbox" {
		fmt.Fprintln(os.Stderr, "[1:1:0101/000000.000:FATAL:zygote_host_impl_linux.cc(132)] No usable sandbox! Update your kernel.")
		return 1
	}
	in, out := fakePipes()
	if in == nil || out == nil {
		fmt.Fprintln(os.Stderr, "no devtools pipe")
		return 2
	}
	if mode == "silent" {
		time.Sleep(time.Hour)
		return 0
	}
	pipe, port := false, false
	for _, a := range os.Args {
		pipe = pipe || a == "--remote-debugging-pipe"
		port = port || strings.HasPrefix(a, "--remote-debugging-port")
	}
	r := bufio.NewReader(in)
	for {
		raw, err := r.ReadBytes(0)
		if err != nil {
			if mode == "stubborn" {
				time.Sleep(time.Hour) // ignores the end of its input
			}
			return 0
		}
		var req struct {
			ID     int    `json:"id"`
			Method string `json:"method"`
		}
		_ = json.Unmarshal(raw[:len(raw)-1], &req)
		result := map[string]any{}
		if req.Method == "Browser.getVersion" {
			result = map[string]any{"protocolVersion": "1.3", "product": fmt.Sprintf("Fake/1 pipe=%v port=%v", pipe, port), "revision": "r", "userAgent": "u", "jsVersion": "j"}
		}
		reply, _ := json.Marshal(map[string]any{"id": req.ID, "result": result})
		_, _ = out.Write(append(reply, 0))
		if req.Method == "Browser.close" && mode != "stubborn" {
			return 0
		}
	}
}

func TestPipeConnFramesMessagesWithNUL(t *testing.T) {
	c := newPipeConn(strings.NewReader("{\"a\":1}\x00{\"b\":[2]}\x00{\"cut"), io.Discard)
	for _, want := range []string{`{"a":1}`, `{"b":[2]}`} {
		got, err := c.Read()
		if err != nil || string(got) != want {
			t.Fatalf("Read = %q %v, want %q", got, err, want)
		}
	}
	if _, err := c.Read(); err == nil {
		t.Fatal("a message without its terminator must not be delivered")
	}
	if err := c.Send([]byte(`{"id":1}`)); err == nil {
		t.Fatal("a call after the end of the input must fail instead of waiting for an answer")
	}

	var sent bytes.Buffer
	c = newPipeConn(strings.NewReader(""), &sent)
	for _, msg := range []string{`{"id":1}`, `{"id":2}`} {
		if err := c.Send([]byte(msg)); err != nil {
			t.Fatal(err)
		}
	}
	if got := sent.String(); got != "{\"id\":1}\x00{\"id\":2}\x00" {
		t.Fatalf("sent %q", got)
	}
}

func TestBrowserArgsUseThePipeAndNoPort(t *testing.T) {
	args := strings.Join(browserArgs(launchOptions{Bin: "chrome", UserDataDir: t.TempDir(), ProxyAddr: "127.0.0.1:1", WindowSize: [2]int{640, 360}}), " ")
	if !strings.Contains(args, "--remote-debugging-pipe") || strings.Contains(args, "remote-debugging-port") {
		t.Fatalf("args = %s", args)
	}
	for _, want := range []string{"--headless", "--proxy-server=127.0.0.1:1", "--window-size=640,360", "--user-data-dir="} {
		if !strings.Contains(args, want) {
			t.Errorf("args lack %s: %s", want, args)
		}
	}
	if strings.Contains(args, "--no-sandbox") || strings.Contains(args, "rod-") {
		t.Errorf("args = %s", args)
	}
}

func fakeBrowser(t *testing.T, mode string) launchOptions {
	t.Helper()
	t.Setenv("SM_FAKE_BROWSER", mode)
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	return launchOptions{Bin: exe, UserDataDir: t.TempDir()}
}

func TestBrowserProcessSpeaksDevToolsOverThePipe(t *testing.T) {
	browser, proc, err := startBrowserProcess(context.Background(), fakeBrowser(t, "answer"))
	if err != nil {
		t.Fatal(err)
	}
	version, err := proto.BrowserGetVersion{}.Call(browser)
	if err != nil {
		t.Fatal(err)
	}
	if version.Product != "Fake/1 pipe=true port=false" {
		t.Fatalf("the browser was started with %q", version.Product)
	}
	pid := proc.PID()
	if err := browser.Close(); err != nil {
		t.Fatal(err)
	}
	proc.Close()
	proc.Close() // a second call is harmless
	select {
	case <-proc.exited:
	default:
		t.Fatal("Close returned while the browser was still running")
	}
	if exists, _ := process.PidExists(int32(pid)); exists {
		t.Fatalf("process %d is still there", pid)
	}
}

func TestBrowserProcessReportsWhyItExited(t *testing.T) {
	_, _, err := startBrowserProcess(context.Background(), fakeBrowser(t, "sandbox"))
	if err == nil || !strings.Contains(err.Error(), "No usable sandbox") {
		t.Fatalf("error = %v", err)
	}
	if issue := launchIssue(err); !strings.Contains(issue, "sandbox") || !strings.Contains(issue, "browser_path") {
		t.Fatalf("issue = %q", issue)
	}
	if _, _, err := startBrowserProcess(context.Background(), launchOptions{Bin: "", UserDataDir: t.TempDir()}); err == nil {
		t.Fatal("no executable must fail")
	}
	if _, _, err := startBrowserProcess(context.Background(), launchOptions{Bin: "/does/not/exist/chrome", UserDataDir: t.TempDir()}); err == nil {
		t.Fatal("a missing executable must fail")
	}
}

func TestBrowserProcessGivesUpOnASilentBrowser(t *testing.T) {
	old := browserStartTimeout
	browserStartTimeout = 400 * time.Millisecond
	t.Cleanup(func() { browserStartTimeout = old })
	start := time.Now()
	_, _, err := startBrowserProcess(context.Background(), fakeBrowser(t, "silent"))
	if err == nil || !strings.Contains(err.Error(), "did not answer") {
		t.Fatalf("error = %v", err)
	}
	if took := time.Since(start); took > 20*time.Second {
		t.Fatalf("a silent browser held the caller for %s", took)
	}
}

func TestBrowserProcessCloseKillsABrowserThatIgnoresTheEndOfItsInput(t *testing.T) {
	old := browserExitWait
	browserExitWait = 300 * time.Millisecond
	t.Cleanup(func() { browserExitWait = old })
	_, proc, err := startBrowserProcess(context.Background(), fakeBrowser(t, "stubborn"))
	if err != nil {
		t.Fatal(err)
	}
	pid := proc.PID()
	start := time.Now()
	proc.Close()
	if took := time.Since(start); took > 15*time.Second {
		t.Fatalf("Close took %s", took)
	}
	select {
	case <-proc.exited:
	default:
		t.Fatal("the browser was not killed")
	}
	if exists, _ := process.PidExists(int32(pid)); exists {
		t.Fatalf("process %d is still there", pid)
	}
}
```

Strengthen the real-browser test: replace the closing `}` of `TestRenderBrowserOpensNoDevToolsPort` with the following, which adds a check that no process of the browser tree listens on a socket, and the helper (the state column of the Windows socket table is unreliable, so a socket without a remote end counts as listening too). `browser_smoke_test.go` imports `gnet "github.com/shirou/gopsutil/v4/net"` for it. Against the old launcher the helper finds the DevTools port in every sample.

```go
	if listening := listeningSockets(s.proc.PID()); len(listening) != 0 {
		t.Fatalf("the browser tree listens on %v", listening)
	}
}

// listeningSockets lists the TCP sockets of a process and its descendants that wait for connections: in
// LISTEN state, or without a remote end (the state column of the Windows socket table is unreliable).
func listeningSockets(pid int) []string {
	root, err := process.NewProcess(int32(pid))
	if err != nil {
		return nil
	}
	tree := map[int32]bool{}
	var walk func(p *process.Process)
	walk = func(p *process.Process) {
		tree[p.Pid] = true
		if kids, err := p.Children(); err == nil {
			for _, k := range kids {
				walk(k)
			}
		}
	}
	walk(root)
	all, err := gnet.Connections("tcp")
	if err != nil {
		return []string{"the socket table cannot be read: " + err.Error()}
	}
	var out []string
	for _, c := range all {
		if tree[c.Pid] && (c.Status == "LISTEN" || c.Raddr.Port == 0) {
			out = append(out, fmt.Sprintf("%s:%d (pid %d)", c.Laddr.IP, c.Laddr.Port, c.Pid))
		}
	}
	return out
}
```

```bash
gofmt -l internal/storymaker
go vet ./internal/storymaker/render/
GOOS=linux go vet ./internal/storymaker/render/
GOOS=darwin go vet ./internal/storymaker/render/
go test ./internal/storymaker/render/ -run 'TestPipeConn|TestBrowserArgs|TestBrowserProcess' -count=1 -v
AURAGO_RUN_BROWSER_SMOKE=1 go test ./internal/storymaker/render/ -count=1 -timeout 25m
```

Expected: PASS (the second command is the whole package with the real Chrome, FFmpeg and the egress layers, about 130 s). Recorded:

```text
--- PASS: TestPipeConnFramesMessagesWithNUL (0.00s)
--- PASS: TestBrowserArgsUseThePipeAndNoPort (0.00s)
--- PASS: TestBrowserProcessSpeaksDevToolsOverThePipe (0.02s)
--- PASS: TestBrowserProcessReportsWhyItExited (0.11s)
--- PASS: TestBrowserProcessGivesUpOnASilentBrowser (0.77s)
--- PASS: TestBrowserProcessCloseKillsABrowserThatIgnoresTheEndOfItsInput (0.69s)
ok  	aurago/internal/storymaker/render	128.896s   (whole package, real Chrome 156.0.8078.74, FFmpeg 8.1.1, Windows 11)
```

On a Linux box the same fake-browser tests and the real-browser tests can be run from a cross-built test binary (what the integration step did in a Debian 12 container with `apt install chromium ffmpeg`, `AURAGO_TEST_NO_SANDBOX=1` because the container runs as root):

```bash
GOOS=linux GOARCH=amd64 go test -c -o render.test ./internal/storymaker/render/
# in the container, with the repository mounted so that ../../../ui resolves:
AURAGO_RUN_BROWSER_SMOKE=1 AURAGO_TEST_NO_SANDBOX=1 AURAGO_TEST_CHROME=/usr/bin/chromium ./render.test -test.run 'TestPipeConn|TestBrowserArgs|TestBrowserProcess|TestBrowserStatusProbe|TestRenderBrowserOpensNoDevToolsPort|TestRealRenderOfTheFirewallStory|TestEgressLayers' -test.v
```

Recorded (Chromium 156.0.8078.73, FFmpeg 5.1.9, Debian 12):

```text
Chromium 156.0.8078.73 built on Debian GNU/Linux 12 (bookworm)
ffmpeg version 5.1.9-0+deb12u1
--- PASS: TestPipeConnFramesMessagesWithNUL
--- PASS: TestBrowserArgsUseThePipeAndNoPort
--- PASS: TestBrowserProcessSpeaksDevToolsOverThePipe (0.02s)
--- PASS: TestBrowserProcessReportsWhyItExited (0.02s)
--- PASS: TestBrowserProcessGivesUpOnASilentBrowser (0.41s)
--- PASS: TestBrowserProcessCloseKillsABrowserThatIgnoresTheEndOfItsInput (0.32s)
--- PASS: TestBrowserStatusProbeLaunchesRealChrome (0.64s)
    browser_smoke_test.go:192: firewall.json: 342 frames in 33.368s (10.2 fps overall)
--- PASS: TestRealRenderOfTheFirewallStory (33.62s)
--- PASS: TestRenderBrowserOpensNoDevToolsPort (0.57s)
--- PASS: TestEgressLayersEachStopTheProbe (16.86s)
```

- [ ] **Step 5: Commit**

```bash
git add internal/storymaker/render/devtools_pipe.go internal/storymaker/render/devtools_pipe_unix.go internal/storymaker/render/devtools_pipe_windows.go internal/storymaker/render/devtools_pipe_test.go internal/storymaker/render/browser.go internal/storymaker/render/session.go internal/storymaker/render/browser_smoke_test.go internal/storymaker/render/AGENTS.md
git commit -m "fix(storymaker): start the render browser with a DevTools pipe instead of a loopback port" -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

### Task I10: Leaving, opening and adding a code scene in the app

Three defects of the app (`ui/js/desktop/apps/storymaker.js`), each found by reading the code of the integrated tree and reproduced in the browser before the fix:

1. **Back after a failed save discarded the typed text.** `leaveProject` asked only when `inst.conflict` was set. When the save failed otherwise (offline, the desktop went read-only, a 422), `flushEdits` returned false, `dirty` stayed true, and `showProjects` → `closeEditor` threw the story away. Closing the window (`beforeClose`) did ask in the same situation. Back now asks whenever the flush did not save.
2. **Two quick project opens raced.** The list stays clickable while `getProject` is pending, there is no cancel (one abort controller per window), and `openProject` checked only `inst.disposed`. When the answer for the first click arrived after the answer for the second, the editor was rebuilt for the first project while the user had chosen the second, and edits went to the wrong project. `openProject` takes a sequence number (like `reloadPlayer` and `requestThumbs` do), and only the answer to the latest open builds the editor; a late failure of an older open does not replace the screen either.
3. **Adding a code scene after deleting one failed.** The server keeps `scenes/<id>.js` when its scene is deleted (the contract has no DELETE for code files). Task I4 made `addScene` skip ids from `code_scenes`, but a scene created in the same session is not in that list (`addScene` writes the file with `putCode` directly), so after delete and add the same id came up again, `If-None-Match: *` answered 412, the app said "The story was changed somewhere else." and a second click picked the same id again. A 412 on the create now tries the next id (at most 20 times).

Callers (grep; these functions are not in the committed index): `openProject` from `showProjects` (list click) and `createProject`; `addScene` from the strip's two add buttons; `leaveProject` from the Back button. All inside `ui/js/desktop/apps/storymaker*.js`.

**Files:**
- Modify: `ui/js/desktop/apps/storymaker.js` (`openProject`, `addScene`, `leaveProject`; 929 to 938 lines)
- Modify: `ui/js/desktop/apps/AGENTS.md`
- Test: `ui/desktop_storymaker_review_browser_test.go` (create)

- [ ] **Step 1: Write the failing tests**

`ui/desktop_storymaker_review_browser_test.go` (three states of the fixture backend of task E8; it reuses the helpers of `desktop_storymaker_browser_test.go`):

```go
package ui

import (
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"
)

// Three defects the review of the app found, each as a browser state against the fixture backend:
//
//   - leaving the project after a failed save must ask first (the typed text exists nowhere else),
//   - two quick project opens must end in the project that was opened last,
//   - a code scene can be added again after one was deleted (the server keeps the file of a deleted code scene).
//
// Gated by AURAGO_RUN_BROWSER_SMOKE=1:
//
//	AURAGO_RUN_BROWSER_SMOKE=1 go test ./ui -run TestStorymakerAppReviewFixesBrowser -count=1
func TestStorymakerAppReviewFixesBrowser(t *testing.T) {
	t.Run("leaving after a failed save asks first", func(t *testing.T) {
		fixture := newStorymakerFixture(t)
		page := storymakerBrowserPage(t, fixture)
		storymakerWait(t, page, `()=>!!document.querySelector('.stm-projects')`)
		page.MustElement(`[data-stm="new-title"]`).MustInput("Unsaved")
		page.MustElement(`[data-stm="create-project"]`).MustClick()
		storymakerWait(t, page, `()=>!!document.querySelector('[data-stm="narration"]')`)
		// The desktop turned read-only behind the window's back: the save of the typed text is refused.
		fixture.set(func() { fixture.readOnly = true })
		page.MustElement(`[data-stm="narration"]`).MustInput("Typed but never saved")
		storymakerWait(t, page, `()=>document.querySelector('[data-stm="save-state"]')?.dataset.state==='error'`)

		storymakerClick(t, page, `[data-stm="back"]`)
		// Either the question or (the defect) the project list.
		storymakerWait(t, page, `()=>!!document.querySelector('.vd-modal') || !!document.querySelector('.stm-projects')`)
		if !page.MustHas(`.vd-modal`) {
			t.Fatal("the editor was left without asking although the last change was never saved")
		}
		if title := storymakerText(page, `.vd-modal-title`); !strings.Contains(title, "unsaved") {
			t.Fatalf("dialog title = %q", title)
		}
		// Cancel keeps the editor and its text.
		page.MustEval(`()=>document.querySelector('.vd-modal [data-cancel]').click()`)
		storymakerWait(t, page, `()=>!document.querySelector('.vd-modal')`)
		if got := page.MustEval(`()=>document.querySelector('[data-stm="narration"]')?.value||''`).String(); got != "Typed but never saved" {
			t.Fatalf("the editor lost the text after Cancel: %q", got)
		}
		// Leaving anyway is possible.
		storymakerClick(t, page, `[data-stm="back"]`)
		storymakerWait(t, page, `()=>!!document.querySelector('.vd-modal')`)
		page.MustEval(`()=>document.querySelector('.vd-modal button[type="submit"]').click()`)
		storymakerWait(t, page, `()=>!!document.querySelector('.stm-projects')`)
		storymakerNoErrors(t, page)
	})

	t.Run("two quick opens end in the last one", func(t *testing.T) {
		fixture := newStorymakerFixture(t)
		alpha, _, _, err := fixture.projects.Create(t.Context(), "Alpha", "16:9", 30)
		if err != nil {
			t.Fatal(err)
		}
		beta, _, _, err := fixture.projects.Create(t.Context(), "Beta", "16:9", 30)
		if err != nil {
			t.Fatal(err)
		}
		// The answer for Alpha is held back until Beta's editor is open.
		release := make(chan struct{})
		var once sync.Once
		free := func() { once.Do(func() { close(release) }) }
		t.Cleanup(free)
		slow := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/projects/"+alpha.ID) {
				<-release
			}
			fixture.ServeHTTP(w, r)
		})
		page := storymakerBrowserPage(t, slow)
		storymakerWait(t, page, `()=>document.querySelectorAll('[data-stm="open-project"]').length===2`)
		page.MustEval(`(id)=>document.querySelector('[data-stm="open-project"][data-id="'+id+'"]').click()`, alpha.ID)
		page.MustEval(`(id)=>document.querySelector('[data-stm="open-project"][data-id="'+id+'"]').click()`, beta.ID)
		storymakerWait(t, page, `()=>document.querySelector('[data-stm="project-title"]')?.textContent==='Beta'`)
		free()
		time.Sleep(900 * time.Millisecond) // long enough for the late answer to be handled
		if got := storymakerText(page, `[data-stm="project-title"]`); got != "Beta" {
			t.Fatalf("the late answer for Alpha replaced the editor: the window shows %q", got)
		}
		// Typing goes to Beta.
		page.MustElement(`[data-stm="narration"]`).MustInput("For Beta")
		storymakerWait(t, page, `()=>document.querySelector('[data-stm="save-state"]')?.dataset.state==='saved'`)
		stored, _ := fixture.storedStory(beta.ID)
		if narration, _ := stored["scenes"].([]any)[0].(map[string]any)["narration"].(string); narration != "For Beta" {
			t.Fatalf("Beta narration = %q", narration)
		}
		storymakerNoErrors(t, page)
	})

	t.Run("a code scene can be added again after one was deleted", func(t *testing.T) {
		fixture := newStorymakerFixture(t)
		page := storymakerBrowserPage(t, fixture)
		storymakerWait(t, page, `()=>!!document.querySelector('.stm-projects')`)
		page.MustElement(`[data-stm="new-title"]`).MustInput("Code again")
		page.MustElement(`[data-stm="create-project"]`).MustClick()
		storymakerWait(t, page, `()=>!!document.querySelector('[data-stm="narration"]')`)
		id := fixture.projectIDs()[0]
		scenes := func() []any {
			stored, _ := fixture.storedStory(id)
			list, _ := stored["scenes"].([]any)
			return list
		}
		waitScenes := func(n int) {
			t.Helper()
			deadline := time.Now().Add(10 * time.Second)
			for len(scenes()) != n && time.Now().Before(deadline) {
				time.Sleep(50 * time.Millisecond)
			}
			if got := len(scenes()); got != n {
				t.Fatalf("the story has %d scenes, want %d (server saw: %s)", got, n, fixture.joined())
			}
		}

		storymakerClick(t, page, `[data-stm="add-code-scene"]`)
		waitScenes(2)
		storymakerWait(t, page, `()=>!!document.querySelector('[data-stm="code-editor"] .cm-editor')`)
		// Delete it again: the server keeps scenes/<id>.js.
		storymakerClick(t, page, `[data-stm="scene-delete"]`)
		storymakerWait(t, page, `()=>!!document.querySelector('.vd-modal')`)
		page.MustEval(`()=>document.querySelector('.vd-modal button[type="submit"]').click()`)
		waitScenes(1)

		// The same count of scenes again, so the first free id is the one whose file is still there.
		storymakerClick(t, page, `[data-stm="add-code-scene"]`)
		waitScenes(2)
		second, _ := scenes()[1].(map[string]any)
		if second["kind"] != "code" {
			t.Fatalf("the new scene is %v", second)
		}
		storymakerNoErrors(t, page)
	})
}
```

- [ ] **Step 2: Run the test to verify it fails**

```bash
AURAGO_RUN_BROWSER_SMOKE=1 go test ./ui -run 'TestStorymakerAppReviewFixesBrowser' -count=1 -timeout 15m
```

Expected: FAIL in all three states. Recorded:

```text
--- FAIL: TestStorymakerAppReviewFixesBrowser (18.42s)
    --- FAIL: TestStorymakerAppReviewFixesBrowser/leaving_after_a_failed_save_asks_first (4.15s)
        desktop_storymaker_review_browser_test.go:37: the editor was left without asking although the last change was never saved
    --- FAIL: TestStorymakerAppReviewFixesBrowser/two_quick_opens_end_in_the_last_one (2.39s)
        desktop_storymaker_review_browser_test.go:85: the late answer for Alpha replaced the editor: the window shows "Alpha"
    --- FAIL: TestStorymakerAppReviewFixesBrowser/a_code_scene_can_be_added_again_after_one_was_deleted (11.88s)
        desktop_storymaker_review_browser_test.go:132: the story has 1 scenes, want 2 (server saw: GET /status ... PUT .../scenes/code2/code ... PUT .../story ... PUT .../scenes/code2/code)
FAIL	aurago/ui	18.683s
```

- [ ] **Step 3: Implement**

In `ui/js/desktop/apps/storymaker.js` (about line 191 of the integration tree), replace:

```
    async function openProject(inst, id, fresh) {
        try {
            const data = fresh || await inst.api.getProject(id);
            if (inst.disposed) return;
```

with:

```
    async function openProject(inst, id, fresh) {
        // The list stays clickable while a project loads: only the answer to the latest open may build the editor.
        const seq = inst.openSeq = (inst.openSeq || 0) + 1;
        try {
            const data = fresh || await inst.api.getProject(id);
            if (inst.disposed || seq !== inst.openSeq) return false;
```

In `ui/js/desktop/apps/storymaker.js` (about line 93 of the integration tree), replace:

```
        } catch (err) {
            if (inst.disposed || (err && err.aborted)) return false;
            const view = inst.parts.projects;
            if (view) view.setError(S.errors.describe(inst.t, err)); else showRoot(inst, startFailure(inst, err));
            return false;
        }
    }
```

with:

```
        } catch (err) {
            if (inst.disposed || seq !== inst.openSeq || (err && err.aborted)) return false;
            const view = inst.parts.projects;
            if (view) view.setError(S.errors.describe(inst.t, err)); else showRoot(inst, startFailure(inst, err));
            return false;
        }
    }
```

In `ui/js/desktop/apps/storymaker.js` (about line 512 of the integration tree), replace:

```
    async function addScene(inst, kind) {
        if (inst.readonly) return;
        const draft = S.model.cloneStory(inst.story);
        const index = S.model.addScene(draft, inst.selected, { kind, taken: (inst.codeStatus || []).map(item => item.scene_id) });
        const id = draft.scenes[index].id;
        if (kind === 'code') {
            try {
                await inst.api.putCode(inst.projectId, id, S.codeTemplate, null);
            } catch (err) {
                notify(inst, S.errors.describe(inst.t, err), 'error');
                return;
            }
        }
        if (inst.disposed) return;
```

with:

```
    async function addScene(inst, kind) {
        if (inst.readonly) return;
        // ids whose code file exists on the server although no scene of the story has them
        const taken = (inst.codeStatus || []).map(item => item.scene_id);
        let draft, index, id;
        for (let attempt = 0; ; attempt++) {
            draft = S.model.cloneStory(inst.story);
            index = S.model.addScene(draft, inst.selected, { kind, taken });
            id = draft.scenes[index].id;
            if (kind !== 'code') break;
            try {
                await inst.api.putCode(inst.projectId, id, S.codeTemplate, null);
                break;
            } catch (err) {
                // 412: the file of a code scene that was deleted earlier is still there (there is no DELETE for code files); try the next id
                if (S.errors.normalize(err).status === 412 && attempt < 20 && !inst.disposed) { taken.push(id); continue; }
                notify(inst, S.errors.describe(inst.t, err), 'error');
                return;
            }
        }
        if (inst.disposed) return;
```

In `ui/js/desktop/apps/storymaker.js` (about line 880 of the integration tree), replace:

```
        const saved = await flushEdits(inst);
        if (!saved && inst.conflict) {
            const leave = await askConfirm(inst, inst.t('storymaker.leave_title'), inst.t('storymaker.leave_text'));
```

with:

```
        const saved = await flushEdits(inst);
        if (!saved) {
            const leave = await askConfirm(inst, inst.t('storymaker.leave_title'), inst.t('storymaker.leave_text'));
```

In `ui/js/desktop/apps/AGENTS.md`, in the Storymaker section:

In `ui/js/desktop/apps/AGENTS.md` (about line 50 of the integration tree), replace:

```
A new code scene is written from `S.codeTemplate` with If-None-Match before the story names it.
```

with:

```
A new code scene is written from `S.codeTemplate` with If-None-Match before the story names it; when that answers 412 (the server keeps the file of a deleted code scene, there is no DELETE for code files) `addScene` tries the next id.
- `Back` asks whenever the last edit is not on the server (offline, read-only, 422, conflict), like closing the window does. `openProject` takes a sequence number: the list stays clickable while a project loads, and only the answer to the latest open builds the editor.
```

- [ ] **Step 4: Run the tests to verify they pass**

```bash
npm run build:ui
npm run check:ui
go test ./ui -run 'Storymaker|TestDesktopStorymaker|TestConfigStorymaker' -count=1
AURAGO_RUN_BROWSER_SMOKE=1 go test ./ui -run 'TestStorymakerApp|TestConfigStorymakerBrowser' -count=1 -timeout 15m
AURAGO_RUN_BROWSER_SMOKE=1 go test ./internal/server -run 'TestStorymaker' -count=1 -timeout 25m
```

Expected: PASS (the app modules are loaded one by one, so `build:ui` changes no bundle). Recorded:

```text
--- PASS: TestConfigStorymakerBrowser (2.31s)
--- PASS: TestStorymakerAppBrowser (9.25s)
--- PASS: TestStorymakerAppStatesBrowser (9.28s)
    --- PASS: TestStorymakerAppStatesBrowser/no_ffmpeg_and_no_browser (2.32s)
    --- PASS: TestStorymakerAppStatesBrowser/read-only (1.46s)
    --- PASS: TestStorymakerAppStatesBrowser/switched_off (1.08s)
    --- PASS: TestStorymakerAppStatesBrowser/signed_out (1.17s)
    --- PASS: TestStorymakerAppStatesBrowser/conflict (3.25s)
--- PASS: TestStorymakerAppReviewFixesBrowser (8.45s)
    --- PASS: TestStorymakerAppReviewFixesBrowser/leaving_after_a_failed_save_asks_first (2.83s)
    --- PASS: TestStorymakerAppReviewFixesBrowser/two_quick_opens_end_in_the_last_one (3.75s)
    --- PASS: TestStorymakerAppReviewFixesBrowser/a_code_scene_can_be_added_again_after_one_was_deleted (1.86s)
--- PASS: TestStorymakerAppTranslationsAreTranslated (0.01s)
--- PASS: TestStorymakerAppUsesOnlyKeysItHas (0.01s)
--- PASS: TestStorymakerAppLoadsNothingExternalAndAvoidsFilteredClasses (0.02s)
--- PASS: TestStorymakerAppModelInNode (0.11s)
--- PASS: TestStorymakerAppApiAndAssetsInNode (0.14s)
--- PASS: TestStorymakerAppNameExistsInAllLanguages (0.99s)
PASS
ok  	aurago/ui	30.545s
ok  	aurago/internal/server	113.693s   (go test ./internal/server -run TestStorymaker, with AURAGO_RUN_BROWSER_SMOKE=1: real-server tour, export end to end, parity, states)
go test ./ui -count=1                : only TestChatFrontend_IntegrationsDrawerRemainsWired and TestFrontend_StaticI18nKeysExistInEnglishBundle fail (both fail on the base commit)
go test ./internal/server -count=1   : only TestGetDesktopServiceReusesCanonicalConfigWithRelativeDefaults fails (fails on the base commit)
```

- [ ] **Step 5: Run the full matrix, then commit**

This is the last check of the plan. Run the commands of task I6 steps 1 to 3 on the finished branch, with two additions: the render package browser run also names `TestRenderBrowserOpensNoDevToolsPort` (task I9), and the parity test needs `AURAGO_STORYMAKER_PARITY_DIR` set to an absolute folder. Every row of the index table (section "Verification at the end") must pass; the only failures allowed are the ones that already exist on the base commit and are listed in I6.

```bash
AURAGO_RUN_BROWSER_SMOKE=1 go test ./internal/storymaker/render/ -run "TestBrowserStatusProbe|TestRealRender|TestEgressLayers|TestRenderBrowserOpensNoDevToolsPort" -count=1 -timeout 30m
```

Expected: PASS apart from the three failures that already exist on the base commit and are listed in I6 (`TestChatFrontend_IntegrationsDrawerRemainsWired`, `TestFrontend_StaticI18nKeysExistInEnglishBundle`, `TestGetDesktopServiceReusesCanonicalConfigWithRelativeDefaults`).

```bash
git add ui/js/desktop/apps/storymaker.js ui/js/desktop/apps/AGENTS.md ui/desktop_storymaker_review_browser_test.go
git commit -m "fix(storymaker): ask before leaving unsaved edits, open the latest project, add a code scene after a delete" -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```
