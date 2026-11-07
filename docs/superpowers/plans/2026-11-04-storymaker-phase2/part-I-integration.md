{% raw %}
# Part I: Integration and close

Batch 2D. Run the tasks in order. `outline.md` holds the batch rules, the test matrices and the execution rules ("outline section N"). `interfaces.md` holds the frozen interfaces ("frozen section N"), and its section 10 and `reconcile.md` win over them. A group id such as `G-A-5` names a block of tasks inside a part; the index lists them.

<!-- G-I-1 -->

### Task I1: Narrated export end to end with all options

**Batch:** 2D  **Depends on:** E3c, A25c

**Files:**
- Test: `internal/server/storymaker_e2e_test.go`

This task only adds code, so it has no impact step.

- [ ] **Step 1: Write the failing test**

In `internal/server/storymaker_e2e_test.go`, add the tools import. A24 and A26 edit this file. The anchors below come from the current code, and neither row touches these lines.

Old text:

```go
	"aurago/internal/storymaker/render"
```

New text:

```go
	"aurago/internal/storymaker/render"
	"aurago/internal/tools"
```

Then add the test at the end of `TestStorymakerExportEndToEnd`.

Old text:

```go
	case <-replayCtx.Done():
		t.Error("a new event stream sent no snapshot of the finished job")
	}
}
```

New text:

```go
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
```

Then sort the imports:

```powershell
gofmt -w internal/server/storymaker_e2e_test.go
```

- [ ] **Step 2: Run it**

Run with the shell sandbox off, because this line starts Chrome.

```powershell
$env:GOCACHE = 'D:\dev-cache\go-build'
$env:GOTMPDIR = 'D:\tmp\go-build'
$env:AURAGO_RUN_BROWSER_SMOKE='1'; go test ./internal/server -run '^TestStorymakerNarratedExportEndToEnd$' -count=1 -timeout 15m; Remove-Item Env:AURAGO_RUN_BROWSER_SMOKE
```

Expected: PASS on the first run, with no red step, because the row pins work that landed in A1 to A26 and E3a to E3c. A failure is an integration bug, and its message names the output that is wrong. Send it to debug-agent. maker-agent fixes it in its own `fix(storymaker)` commit before this task commits.

- [ ] **Step 3: Implement**

No production code. If Step 2 passed, go to Step 4.

- [ ] **Step 4: Run the checks**

```powershell
gofmt -l internal/server
go test ./internal/server -run '^TestStorymakerNarratedExportEndToEnd$' -count=1
```

Expected: `gofmt` prints nothing. Without `AURAGO_RUN_BROWSER_SMOKE` the test skips, and the package line prints `ok`.

- [ ] **Step 5: Commit**

Run `detect_changes` as outline section 5 says. A PARTIAL answer on new symbols is accepted with a grep confirmation stated in the commit body. If the tool crashes, closes the connection, answers `not found`, or prints only its banner, use `graft` callers plus grep and say so in the commit body.

```powershell
node C:/Users/Andi/.gitnexus/stores/AuraGo-08496d38b623/checkouts/AuraGo-08496d38b623/run.cjs detect-changes --scope all --repo C:/Users/Andi/Documents/repo/AuraGo # run alone, read the answer before git add
git add internal/server/storymaker_e2e_test.go
git commit -m "test(storymaker): narrated export end to end with all options" -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

### Task I2: Final app tour

**Batch:** 2D  **Depends on:** E3e, E3f

**Files:**
- Test: `internal/server/storymaker_app_browser_test.go`
- Test: `ui/desktop_storymaker_browser_test.go`
- Test: `ui/desktop_storymaker_fixture_test.go`

- [ ] **Step 1: Impact**

From the main checkout, run:

```powershell
node C:/Users/Andi/.gitnexus/stores/AuraGo-08496d38b623/checkouts/AuraGo-08496d38b623/run.cjs impact "smappPageWith" --direction upstream --repo .
node C:/Users/Andi/.gitnexus/stores/AuraGo-08496d38b623/checkouts/AuraGo-08496d38b623/run.cjs impact "TestStorymakerAppRealServerBrowser" --direction upstream --repo .
node C:/Users/Andi/.gitnexus/stores/AuraGo-08496d38b623/checkouts/AuraGo-08496d38b623/run.cjs impact "storymakerTour" --direction upstream --repo .
```

Report callers, processes and risk. Warn on HIGH or CRITICAL, and confirm `risk: UNKNOWN` with a text search, as outline section 5 says. `smappPageWith` also serves `internal/server/storymaker_app_states_browser_test.go`. This task only adds a route there.

- [ ] **Step 2: Check the precondition and see the tour red**

The worktree must hold the `main.bundle.js` that E3f rebuilt. The bundle stays uncommitted in this task.

```powershell
Select-String -Path ui/js/desktop/bundles/main.bundle.js -Pattern 'pendingProjectId' -Quiet
```

Expected: `True`. If it prints `False`, run `npm run build:ui` and check again.

Run the tour as it stands, with the shell sandbox off:

```powershell
$env:GOCACHE = 'D:\dev-cache\go-build'
$env:GOTMPDIR = 'D:\tmp\go-build'
$env:AURAGO_RUN_BROWSER_SMOKE='1'; go test ./internal/server -run '^TestStorymakerAppRealServerBrowser$' -count=1 -timeout 15m; Remove-Item Env:AURAGO_RUN_BROWSER_SMOKE
```

Expected: FAIL after 90 seconds in the gate step that E2g added, with `timed out waiting for ()=>!!document.querySelector('[data-stm="export-start"]') && !document.querySelector('[data-stm="export-start"]').disabled`. Since E3b the fresh `/narration` answer with the missing block disables `export-start`. If the panel paints before that answer arrives, the click opens the dialog instead and the same step times out on `export-refused`. Either line is the expected red.

- [ ] **Step 3: Write the tour**

The edits before the first `gofmt -w` line are in `internal/server/storymaker_app_browser_test.go`, and the stand-in tour after it names its files. Each old text below is what E1k and E2g leave: E1k's import block keeps the desktop line and has no config import, E1k's seam comes right after the first two lines of the test, and E2g keeps the `windows :=` line and writes the gate step quoted below.

Add the config import.

Old text:

```go
	"aurago/internal/desktop"
```

New text:

```go
	"aurago/internal/config"
	"aurago/internal/desktop"
```

Mount the real Video Studio beside the real Storymaker handlers.

Old text:

```go
	mux.Handle(storymakerAPIBase+"/", handleStorymaker(s))
```

New text:

```go
	mux.Handle(storymakerAPIBase+"/", handleStorymaker(s))
	mux.Handle(videoStudioAPIBase+"/", handleVideoStudio(s))
```

Add the helpers and switch Video Studio on.

Old text:

```go
func TestStorymakerAppRealServerBrowser(t *testing.T) {
	s := storymakerRealServer(t)
```

New text:

```go
// smappStudio is the page's one Video Studio instance, as a JS expression.
const smappStudio = `[...(window.VideoStudioApp?.instances?.values()||[])][0]`

// smappEnableVideoStudio switches on the real Video Studio that the handoff talks to.
func smappEnableVideoStudio(t *testing.T, s *Server) {
	t.Helper()
	s.Cfg.VideoStudio.Enabled = true
	if err := config.NormalizeVideoStudioConfig(&s.Cfg.VideoStudio); err != nil {
		t.Fatal(err)
	}
	s.integrationCtx = context.Background()
	manager, err := s.videoStudioManager()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(manager.close)
}

// smappVideoStudio calls the real Video Studio API and decodes the answer.
func smappVideoStudio(t *testing.T, origin, method, path, body string, into any) {
	t.Helper()
	req, err := http.NewRequest(method, origin+videoStudioAPIBase+path, strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	data, _ := io.ReadAll(res.Body)
	if res.StatusCode >= 300 {
		t.Fatalf("%s %s = %d %s", method, path, res.StatusCode, data)
	}
	if err := json.Unmarshal(data, into); err != nil {
		t.Fatalf("%s %s: %v\n%s", method, path, err, data)
	}
}

// smappOpenDecoyStudio creates a Video Studio project and opens a window on it. The shell always reuses that window, so the
// handoff must go through the reuse branch of window-shell-runtime.js. This is why the tour needs the rebuilt main.bundle.js.
func smappOpenDecoyStudio(t *testing.T, page *rod.Page, origin string) string {
	t.Helper()
	var decoy struct {
		ID string `json:"id"`
	}
	smappVideoStudio(t, origin, http.MethodPost, "/projects", `{"name":"Decoy"}`, &decoy)
	page.MustEval(`()=>{auditTest.openApp('video-studio');}`)
	smappWait(t, page, `()=>{const s=`+smappStudio+`;return !!s&&s.studioReady&&s.projectId===`+smappJS(decoy.ID)+`;}`)
	return decoy.ID
}

// smappExportDialog fills the open export dialog and starts the export: SRT, VTT and Open in Video Studio, which forces stems.
func smappExportDialog(t *testing.T, page *rod.Page) {
	t.Helper()
	smappWait(t, page, `()=>document.querySelector('[data-stm="export-dialog"]')?.open===true`)
	smappClick(t, page, `[data-stm="export-opt-srt"]`)
	smappClick(t, page, `[data-stm="export-opt-vtt"]`)
	smappClick(t, page, `[data-stm="export-opt-handoff"]`)
	smappWait(t, page, `()=>{const q=n=>document.querySelector('[data-stm="'+n+'"]');return q('export-opt-srt').checked&&q('export-opt-vtt').checked&&q('export-opt-handoff').checked&&q('export-opt-stems').checked&&q('export-opt-stems').disabled&&!q('export-stems-forced').hidden;}`)
	smappClick(t, page, `[data-stm="export-confirm"]`)
	smappWait(t, page, `()=>document.querySelector('[data-stm="export-dialog"]').open===false`)
}

// smappCheckHandoff checks the finished export: each extra shows in its row of the panel and lies on disk, the handoff switched
// the open Video Studio window to a new project, that project has the tracks of frozen section 5, and the panel's button runs
// the handoff again.
func smappCheckHandoff(t *testing.T, page *rod.Page, s *Server, origin, id, decoyID string) {
	t.Helper()
	folder := strings.TrimSuffix(smappText(page, `[data-stm="export-path"]`), ".mp4")
	for extra, name := range map[string]string{"narration": "narration.wav", "srt": "captions.srt", "vtt": "captions.vtt"} {
		shown := smappText(page, `[data-stm="export-extras"] [data-extra="`+extra+`"] [data-stm="export-extra-path"]`)
		if shown != folder+"/"+name {
			t.Errorf("the finished export shows %q for %s, want %s/%s", shown, extra, folder, name)
		}
		info, err := os.Stat(filepath.Join(s.Cfg.VirtualDesktop.WorkspaceDir, filepath.FromSlash(folder+"/"+name)))
		if err != nil || info.Size() == 0 {
			t.Errorf("%s/%s on disk: %v", folder, name, err)
		}
	}

	var list struct {
		Jobs []struct {
			Kind        string `json:"kind"`
			Status      string `json:"status"`
			FramesTotal int    `json:"frames_total"`
		} `json:"jobs"`
	}
	smappGet(t, origin, "/projects/"+id+"/jobs", &list)
	frames := 0
	for _, job := range list.Jobs {
		if job.Kind == "render" && job.Status == "succeeded" {
			frames = job.FramesTotal
			break
		}
	}
	if frames <= 0 {
		t.Fatalf("no finished render in %+v", list.Jobs)
	}

	smappWaitFor(t, page, `()=>{const s=`+smappStudio+`;return !!s&&s.projectId!==`+smappJS(decoyID)+`&&!s.dirty&&(s.project?.tracks||[]).some(t=>t.clips.length>0);}`, 5*time.Minute)
	if got := page.MustEval(`()=>[...auditTest.state.windows.values()].filter(w=>w.appId==='video-studio').length`).Int(); got != 1 {
		t.Fatalf("%d Video Studio windows, want the one reused window", got)
	}
	projectID := page.MustEval(`()=>` + smappStudio + `.projectId`).String()

	var handed struct {
		Project struct {
			Width  int `json:"width"`
			Height int `json:"height"`
			FPS    int `json:"fps"`
			Assets []struct {
				ID string `json:"id"`
			} `json:"assets"`
			Tracks []struct {
				Kind  string `json:"kind"`
				Clips []struct {
					AssetID       string `json:"asset_id"`
					Duration      int    `json:"duration"`
					AudioDetached bool   `json:"audio_detached"`
				} `json:"clips"`
			} `json:"tracks"`
		} `json:"project"`
	}
	smappVideoStudio(t, origin, http.MethodGet, "/projects/"+projectID, "", &handed)
	p := handed.Project
	if p.Width != 1920 || p.Height != 1080 || p.FPS != 30 {
		t.Errorf("Video Studio canvas = %dx%d at %d fps", p.Width, p.Height, p.FPS)
	}
	// One video track and at least the narration track. Each asset (the MP4 and each stem) sits in one clip on its own track.
	if len(p.Tracks) < 2 || p.Tracks[0].Kind != "video" || len(p.Assets) != len(p.Tracks) {
		t.Fatalf("handoff project: %+v", p)
	}
	for i, track := range p.Tracks {
		if i > 0 && track.Kind != "audio" {
			t.Errorf("track %d has kind %q, want audio", i, track.Kind)
		}
		if len(track.Clips) != 1 || track.Clips[0].Duration <= 0 || track.Clips[0].Duration > frames {
			t.Errorf("track %d clips = %+v, film has %d frames", i, track.Clips, frames)
		}
	}
	if len(p.Tracks[0].Clips) == 1 && !p.Tracks[0].Clips[0].AudioDetached {
		t.Error("the video clip must have audio_detached")
	}

	// The finished job keeps its button. A click runs the handoff again, and a second new project opens in the same window.
	smappWait(t, page, `()=>{const b=document.querySelector('[data-stm="export-handoff"]');return !!b&&!b.disabled;}`)
	if got := smappText(page, `[data-stm="export-handoff-error"]`); got != "" {
		t.Fatalf("the handoff reports an error: %q", got)
	}
	smappClick(t, page, `[data-stm="export-handoff"]`)
	smappWaitFor(t, page, `()=>{const s=`+smappStudio+`;return !!s&&s.projectId!==`+smappJS(projectID)+`&&s.projectId!==`+smappJS(decoyID)+`&&!s.dirty&&(s.project?.tracks||[]).some(t=>t.clips.length>0);}`, 5*time.Minute)
}

func TestStorymakerAppRealServerBrowser(t *testing.T) {
	s := storymakerRealServer(t)
	smappEnableVideoStudio(t, s)
```

Rewrite the gate step that E2g wrote. Since E3b, the fresh `/narration` answer with the missing block disables `export-start`, so the panel no longer reaches the server's refusal. `TestStorymakerExportRefusalsInNode` and `TestStorymakerRenderRefusesMissingAudio` keep `export-refused` and the 409 covered.

Old text:

```go
	// The gate: narration text without audio stops the export. The panel names the scene and offers the voice panel, and the same
	// export passes once the narrate job has stored the audio. The code scene code2 is still selected.
	smappClick(t, page, `[data-stm="side-tab"][data-tab="scene"]`)
	page.MustElement(`[data-stm="narration"]`).MustInput("Every packet gets checked.")
	smappWait(t, page, `()=>document.querySelector('[data-stm="save-state"]')?.dataset.state==='saved'`)
	smappClick(t, page, `[data-stm="export"]`)
	smappWait(t, page, `()=>!!document.querySelector('[data-stm="export-start"]') && !document.querySelector('[data-stm="export-start"]').disabled`)
	smappClick(t, page, `[data-stm="export-start"]`)
	smappWait(t, page, `()=>document.querySelector('[data-stm="export-refused"]')?.dataset.code==='narration_incomplete'`)
	if missing := smappText(page, `[data-stm="export-missing"]`); !strings.Contains(missing, "code2") {
		t.Fatalf("the refusal does not name the scene: %q", missing)
	}
	smappClick(t, page, `[data-stm="export-go-voice"]`)
	smappWait(t, page, `()=>document.querySelector('[data-stm="export-panel"]').hidden`)
```

New text:

```go
	// The gate: narration text without audio stops the export. The fresh /narration answer keeps the start off and the panel names
	// the reason, and the same export passes once the narrate job has stored the audio. The code scene code2 is still selected.
	smappClick(t, page, `[data-stm="side-tab"][data-tab="scene"]`)
	page.MustElement(`[data-stm="narration"]`).MustInput("Every packet gets checked.")
	smappWait(t, page, `()=>document.querySelector('[data-stm="save-state"]')?.dataset.state==='saved'`)
	smappClick(t, page, `[data-stm="export"]`)
	smappWait(t, page, `()=>!!document.querySelector('[data-stm="export-blockers"] [data-reason="narration"]') && document.querySelector('[data-stm="export-start"]').disabled`)
	smappClick(t, page, `[data-stm="export-close"]`)
	smappWait(t, page, `()=>document.querySelector('[data-stm="export-panel"]').hidden`)
```

Start the export through the dialog, with a Video Studio window already open.

Old text:

```go
	smappClick(t, page, `[data-stm="export-start"]`)
	smappWait(t, page, `()=>!!document.querySelector('[data-stm="export-progress"]')`)
```

New text:

```go
	decoyID := smappOpenDecoyStudio(t, page, origin)
	smappClick(t, page, `[data-stm="export-start"]`)
	smappExportDialog(t, page)
	smappWait(t, page, `()=>!!document.querySelector('[data-stm="export-progress"]')`)
```

Check the outputs and the handoff before the Files window opens.

Old text:

```go
	windows := page.MustEval(`()=>auditTest.state.windows.size`).Int()
```

New text:

```go
	smappCheckHandoff(t, page, s, origin, id, decoyID)
	windows := page.MustEval(`()=>auditTest.state.windows.size`).Int()
```

Take the way back to the finished export once the project is open again. The reopened panel reads the real job list, so this also pins the shape of that answer, including the stems that the handoff needs.

Old text:

```go
	// Delete asks the desktop dialog first: refusing keeps the project, confirming removes the folder.
```

New text:

```go
	// The way back to the finished export: the reopened panel reads the real job list and shows the result with its handoff.
	smappClick(t, page, `[data-stm="export"]`)
	smappClick(t, page, `[data-stm="export-last"]`)
	smappWait(t, page, `()=>document.querySelector('[data-stm="export-path"]')?.textContent===`+smappJS(path)+` && !!document.querySelector('[data-stm="export-handoff"]')`)
	smappNoErrors(t, page)
	smappClick(t, page, `[data-stm="export-close"]`)

	// Delete asks the desktop dialog first: refusing keeps the project, confirming removes the folder.
```

Then sort the imports:

```powershell
gofmt -w internal/server/storymaker_app_browser_test.go
```

The stand-in tour steps its job by hand, so it can leave while an export runs. In `ui/desktop_storymaker_fixture_test.go`, give the stand-in the job list of the real handler.

Old text:

```go
	case rest == "events" && r.Method == http.MethodGet:
```

New text:

```go
	case rest == "jobs" && r.Method == http.MethodGet:
		f.jobList(w, id)
	case rest == "events" && r.Method == http.MethodGet:
```

Old text:

```go
func (f *storymakerFixture) events(w http.ResponseWriter, r *http.Request, id string) {
```

New text:

```go
// jobList answers GET /projects/{id}/jobs like the real handler: the jobs of the project, newest first.
func (f *storymakerFixture) jobList(w http.ResponseWriter, id string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	jobs := []map[string]any{}
	for i := len(f.order) - 1; i >= 0; i-- {
		if job := f.jobs[f.order[i]]; job.ProjectID == id {
			jobs = append(jobs, f.jobBody(job))
		}
	}
	f.writeJSON(w, http.StatusOK, map[string]any{"jobs": jobs})
}

func (f *storymakerFixture) events(w http.ResponseWriter, r *http.Request, id string) {
```

In `ui/desktop_storymaker_browser_test.go`, leave during a second export and come back after it has finished. The stand-in's stream replays nothing, so only the job list can show the result.

Old text:

```go
	// Delete asks the desktop dialog first: refusing keeps the project, confirming removes the folder.
```

New text:

```go
	// The way back to a finished export: the reopened panel offers the newest one, also one that ended while the project was closed.
	storymakerClick(t, page, `[data-stm="export"]`)
	storymakerClick(t, page, `[data-stm="export-last"]`)
	storymakerWait(t, page, `()=>!!document.querySelector('[data-stm="export-path"]')`)
	firstPath := storymakerText(page, `[data-stm="export-path"]`)
	storymakerClick(t, page, `[data-stm="export-again"]`)
	storymakerWait(t, page, `()=>!!document.querySelector('[data-stm="export-start"]') && !document.querySelector('[data-stm="export-start"]').disabled`)
	storymakerClick(t, page, `[data-stm="export-start"]`)
	storymakerClick(t, page, `[data-stm="export-confirm"]`)
	storymakerWait(t, page, `()=>!!document.querySelector('[data-stm="export-progress"]')`)
	storymakerClick(t, page, `[data-stm="export-close"]`)
	storymakerClick(t, page, `[data-stm="back"]`)
	storymakerWait(t, page, `()=>document.querySelectorAll('[data-stm="open-project"]').length===1 && !document.querySelector('.stm-player')`)
	for i := 0; i < 4; i++ {
		fixture.advance()
	}
	storymakerClick(t, page, `[data-stm="open-project"]`)
	storymakerWait(t, page, `()=>document.querySelector('[data-stm="narration"]')?.value.includes('knocks at the door') && document.querySelectorAll('[data-stm="scene"]').length===2`)
	storymakerClick(t, page, `[data-stm="export"]`)
	storymakerClick(t, page, `[data-stm="export-last"]`)
	storymakerWait(t, page, `()=>!!document.querySelector('[data-stm="export-path"]')`)
	if path := storymakerText(page, `[data-stm="export-path"]`); path == firstPath || !strings.HasSuffix(path, ".mp4") {
		t.Fatalf("the way back shows %q, the first export was %q", path, firstPath)
	}
	storymakerNoErrors(t, page)
	storymakerClick(t, page, `[data-stm="export-close"]`)

	// Delete asks the desktop dialog first: refusing keeps the project, confirming removes the folder.
```

```powershell
gofmt -w ui/desktop_storymaker_browser_test.go ui/desktop_storymaker_fixture_test.go
```

- [ ] **Step 4: Implement**

No production code. E3a to E3f hold it, so no new app code belongs in this task.

If the Step 5 run fails, the failed wait names the owner:
- `export-blockers` or `export-dialog`: E3b
- an `export-opt-*` selector or `export-stems-forced`: E3a
- an `export-extras` row: E3c
- the wait on `s.projectId`, `export-handoff` or `export-handoff-error`: E3e or E3f
- `export-last`: E3e
- a track assertion: E3d

debug-agent finds the cause. maker-agent fixes it in its own `fix(storymaker)` commit before this task commits.

- [ ] **Step 5: Run**

Run with the shell sandbox off.

```powershell
$env:GOCACHE = 'D:\dev-cache\go-build'
$env:GOTMPDIR = 'D:\tmp\go-build'
$env:AURAGO_RUN_BROWSER_SMOKE='1'; go test ./internal/server -run '^TestStorymakerAppRealServerBrowser$' -count=1 -timeout 15m; Remove-Item Env:AURAGO_RUN_BROWSER_SMOKE
$env:AURAGO_RUN_BROWSER_SMOKE='1'; go test ./ui -run '^TestStorymakerAppBrowser$' -count=1 -timeout 10m; Remove-Item Env:AURAGO_RUN_BROWSER_SMOKE
gofmt -l internal/server ui/desktop_storymaker_browser_test.go ui/desktop_storymaker_fixture_test.go
```

Expected: `--- PASS: TestStorymakerAppRealServerBrowser`, `--- PASS: TestStorymakerAppBrowser` and `ok` for both packages, and `gofmt` prints nothing.

- [ ] **Step 6: Commit**

Run `detect_changes` as outline section 5 says. A PARTIAL answer on new symbols is accepted with a grep confirmation stated in the commit body. Stage the three test files only. `ui/js/desktop/bundles/main.bundle.js` stays rebuilt and uncommitted until the last commit of the batch.

```powershell
node C:/Users/Andi/.gitnexus/stores/AuraGo-08496d38b623/checkouts/AuraGo-08496d38b623/run.cjs detect-changes --scope all --repo C:/Users/Andi/Documents/repo/AuraGo # run alone, read the answer before git add
git add internal/server/storymaker_app_browser_test.go ui/desktop_storymaker_browser_test.go ui/desktop_storymaker_fixture_test.go
git commit -m "test(storymaker): final app tour with dialog, stems and Video Studio handoff" -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
git status --short -- ui/js/desktop/bundles/main.bundle.js
```

Expected from the last line: ` M ui/js/desktop/bundles/main.bundle.js`.

### Task I3: FFmpeg 5.1.9, swatch and gate (spec line 836)

**Batch:** 2D  **Depends on:** A18

**Files:**
- Test: `internal/storymaker/render/ffmpeg_test.go`
- Modify: `docs/superpowers/specs/2026-11-01-storymaker-design.md`

This task adds one test and edits one spec line, so it has no impact step.

- [ ] **Step 1: Write the failing test**

In `internal/storymaker/render/ffmpeg_test.go`, add the test after `TestFFmpegWriteFailsWithAMessageWhenTheProcessDies`, ahead of the tests A17 appended. All the imports it needs are already in the file.

Old text:

```go
	if err := ff.WriteFrame(solidPNG(t, 64, 64, color.NRGBA{A: 255})); err == nil {
		t.Fatal("a write to a dead FFmpeg succeeded")
	}
}
```

New text:

```go
	if err := ff.WriteFrame(solidPNG(t, 64, 64, color.NRGBA{A: 255})); err == nil {
		t.Fatal("a write to a dead FFmpeg succeeded")
	}
}

// TestFFmpegSwatchAndGate is the colour round-trip of spec line 836: a known colour goes in as a PNG (image2pipe) and as raw
// RGBA (rawvideo), and the decoded file holds its BT.709 limited-range value within 2 levels. The frame gate runs on the same
// file. Run it on 8.1.1 and in one debian:12 container, which has 5.1.9.
func TestFFmpegSwatchAndGate(t *testing.T) {
	ffmpeg, ffprobe := testBinary(t, "ffmpeg"), testBinary(t, "ffprobe")
	const w, h, fps, n = 320, 180, 30, 30
	// The limited-range values from the matrix definition, with R, G and B in 0..1:
	//   luma = kr*R + (1-kr-kb)*G + kb*B
	//   Y = 16 + 219*luma, U = 128 + 224*(B-luma)/(2*(1-kb)), V = 128 + 224*(R-luma)/(2*(1-kr))
	// BT.709 has kr 0.2126 and kb 0.0722. BT.601 has kr 0.299 and kb 0.114.
	levels := func(kr, kb float64, c color.NRGBA) [3]float64 {
		r, g, b := float64(c.R)/255, float64(c.G)/255, float64(c.B)/255
		luma := kr*r + (1-kr-kb)*g + kb*b
		return [3]float64{16 + 219*luma, 128 + 224*(b-luma)/(2*(1-kb)), 128 + 224*(r-luma)/(2*(1-kr))}
	}
	// By the rules above: red is Y 62.6, U 102.3, V 240.0 in BT.709 and Y 81.5 in BT.601. The second swatch is Y 117.0, U 96.0,
	// V 173.9 in BT.709 and Y 122.7 in BT.601.
	for _, c := range []color.NRGBA{{R: 255, A: 255}, {R: 200, G: 100, B: 50, A: 255}} {
		want := levels(0.2126, 0.0722, c)
		// A swatch that cannot tell the matrices apart would let a BT.601 file pass.
		if bt601 := levels(0.299, 0.114, c); math.Abs(want[0]-bt601[0]) <= 2 {
			t.Fatalf("swatch %v has BT.709 Y %.1f and BT.601 Y %.1f, closer than the tolerance", c, want[0], bt601[0])
		}
		for _, transport := range []string{TransportPNG, TransportRaw} {
			frame := solidPNG(t, w, h, c)
			if transport == TransportRaw {
				frame = bytes.Repeat([]byte{c.R, c.G, c.B, c.A}, w*h)
			}
			frames := make([][]byte, n)
			for i := range frames {
				frames[i] = frame
			}
			out := filepath.Join(t.TempDir(), "swatch.mp4")
			ff, err := encodeFrames(t, FFmpegConfig{Path: ffmpeg, Transport: transport, Width: w, Height: h, FPS: fps, Frames: n, Output: out}, frames)
			if err != nil {
				t.Fatalf("%s: encode: %v (%s)", transport, err, ff.Message())
			}
			probe, err := ProbeOutput(context.Background(), ffprobe, out, postEncodeMinLimit)
			if err != nil {
				t.Fatalf("%s: frame gate probe: %v", transport, err)
			}
			if err := CheckOutput(probe, n); err != nil {
				t.Fatalf("%s: frame gate: %v (%+v)", transport, err, probe)
			}
			raw, err := exec.Command(ffmpeg, "-v", "error", "-i", out, "-frames:v", "1", "-f", "rawvideo", "-pix_fmt", "yuv420p", "-").Output()
			if err != nil || len(raw) != w*h*3/2 {
				t.Fatalf("%s: decode: %v, %d bytes", transport, err, len(raw))
			}
			// yuv420p is a w*h luma plane, then two (w/2)*(h/2) chroma planes. Each index is the middle of its plane.
			mid := (w/2)*(h/2)/2 + w/4
			got := [3]byte{raw[w*h/2+w/2], raw[w*h+mid], raw[w*h*5/4+mid]}
			for i, name := range []string{"Y", "U", "V"} {
				if math.Abs(float64(got[i])-want[i]) > 2 {
					t.Errorf("%s swatch %v: %s = %d, want %.1f within 2 levels", transport, c, name, got[i], want[i])
				}
			}
		}
	}
}
```

- [ ] **Step 2: Run it on 8.1.1**

```powershell
$env:GOCACHE = 'D:\dev-cache\go-build'
$env:GOTMPDIR = 'D:\tmp\go-build'
go test ./internal/storymaker/render -run '^TestFFmpegSwatchAndGate$' -count=1 -v
```

Expected: PASS, with no red step, because the row verifies shipped behaviour on a second FFmpeg. The test guards itself: each swatch must separate BT.709 from BT.601 by more than the tolerance. On a failure, do not widen the tolerance. Send the printed values to debug-agent.

- [ ] **Step 3: Implement: run it on 5.1.9**

There is no code to add. Build the Linux test binary and run it in one container of your own.

```powershell
docker info --format '{{.ServerVersion}}'
```

If that fails, Docker Desktop is off. Run `Start-Process 'C:\Program Files\Docker\Docker\Docker Desktop.exe'` and repeat until it answers. Then run:

```powershell
New-Item -ItemType Directory -Force D:\tmp\go-build\sm-i3 | Out-Null
$env:GOOS='linux'; $env:GOARCH='amd64'; $env:CGO_ENABLED='0'
go test -c -o D:\tmp\go-build\sm-i3\render.test ./internal/storymaker/render
Remove-Item Env:GOOS; Remove-Item Env:GOARCH; Remove-Item Env:CGO_ENABLED
docker run --rm --name sm-i3-swatch -v D:\tmp\go-build\sm-i3:/w:ro debian:12 bash -c "apt-get update -qq >/dev/null && DEBIAN_FRONTEND=noninteractive apt-get install -y -qq ffmpeg >/dev/null 2>&1 && ffmpeg -version | head -1 && cp /w/render.test /tmp/render.test && chmod +x /tmp/render.test && /tmp/render.test -test.run TestFFmpegSwatchAndGate -test.count=1 -test.timeout 5m -test.v"
Remove-Item -Recurse -Force D:\tmp\go-build\sm-i3
```

Expected: a first line that starts with `ffmpeg version 5.1`, then `--- PASS: TestFFmpegSwatchAndGate` and `PASS`. Touch no other container. If the run fails, stop and report the values, and do not edit the spec.

- [ ] **Step 4: Run the checks and record the result**

```powershell
gofmt -l internal/storymaker/render
go vet ./internal/storymaker/render
```

Expected: `gofmt` prints nothing, and `go vet` exits 0.

Only after both runs passed, edit `docs/superpowers/specs/2026-11-01-storymaker-design.md`. If the container printed a version other than 5.1.9, write the printed version in the new text.

Old text:

```text
- **Colour and gain on 5.1.9.** The `image2pipe` and `rawvideo` colour round-trip (swatch check), and the two-pass `loudnorm` linear gain. **Open.** On 8.1.1 the phase 1 output is tagged BT.709 limited range, yuv420p (ffprobe). A swatch check on 5.1.9 was not run, and the loudnorm gain is phase 2.
```

New text:

```text
- **Colour and gain on 5.1.9.** The `image2pipe` and `rawvideo` colour round-trip (swatch check), and the loudness gain. **Resolved.** `TestFFmpegSwatchAndGate` passes on FFmpeg 8.1.1 (Windows) and on 5.1.9 (one `debian:12` container): a PNG and a raw swatch each decode within 2 levels of their BT.709 limited-range values, and the frame gate accepts the file on both versions. The meter replaced the two-pass `loudnorm` gain (see "Changes from the phase 2 plan"), and Task A10 ran its graph on both versions.
```

- [ ] **Step 5: Commit**

Run `detect_changes` as outline section 5 says. A PARTIAL answer on new symbols is accepted with a grep confirmation stated in the commit body. If the tool crashes, closes the connection, answers `not found`, or prints only its banner, use `graft` callers plus grep and say so in the commit body.

```powershell
node C:/Users/Andi/.gitnexus/stores/AuraGo-08496d38b623/checkouts/AuraGo-08496d38b623/run.cjs detect-changes --scope all --repo C:/Users/Andi/Documents/repo/AuraGo # run alone, read the answer before git add
git add internal/storymaker/render/ffmpeg_test.go
git add -f docs/superpowers/specs/2026-11-01-storymaker-design.md
git commit -m "test(storymaker): colour swatch and frame gate on FFmpeg 8.1.1 and 5.1.9" -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

<!-- G-I-2 -->

### Task I4: Contract final slice, spec To verify results
**Batch:** 2D  **Depends on:** I1, I2, I3
**Files:**
- Modify: `docs/superpowers/specs/2026-11-01-storymaker-contract.md`
- Modify: `docs/superpowers/specs/2026-11-01-storymaker-design.md`

Docs only, so there is no failing test. The check is the path check in Step 6. Earlier slices (M16, E1j, E2f) have already changed the contract, so match by the quoted text, never by line number.

- [ ] **Step 1: Check for parallel work, find out if W landed, collect the measured values**

```powershell
Set-Location C:/Users/Andi/Documents/repo/AuraGo/.worktrees/storymaker-p2d
git -C C:/Users/Andi/Documents/repo/AuraGo status --short -- docs/superpowers/specs/2026-11-01-storymaker-contract.md docs/superpowers/specs/2026-11-01-storymaker-design.md
Test-Path ui/js/storymaker/host-pool.js
Get-Content docs/superpowers/plans/2026-11-04-storymaker-phase2/measured.md
```

Expected:
- The first command prints nothing. If it lists a file, another session is working there, so ask the controller first.
- `Test-Path` answers `True` when W landed. `False` means W was dropped.
- `measured.md` holds the two sections that earlier tasks recorded. M16 wrote `## Motion lint rates (2A, real catalog, no audio)`. Its line that starts `Rate per scene:` gives the rate of each lint code, and the sentence at the end of that line gives `Cued beats:` and `with cue_late:`. E1j wrote `## Cue share, batch 2B (spec line 853)`. Its table has one row each for de, el, hi and ja, and the column `measured` holds `cue_precision` on hand-built blocks.

Keep those numbers for Step 3. Copy them literally. If a number is missing, stop and ask the controller, and do not estimate it.

- [ ] **Step 2: Apply the contract edits**

Read the contract first. Edits 1 to 5 apply only if W landed. If W was dropped, skip them and apply edits 6 and 7.

Edit 1, section 6. Replace the whole line that starts with ``- `ui/js/storymaker/host.bundle.js` `` with:

```text
- `ui/js/storymaker/host.bundle.js` (6 parts, as listed in `scripts/storymaker-bundle-parts.js`: `schedule`, `host-core`, `host-app`, `host-pool`, `host-render`, `host`) runs in the document realm: the page, the host, the render driver and the parallel scheduler.
```

Edit 2, section 6, "Worker messages". This line is unchanged since phase 1.

Old:
```text
- Player worker: `plan`, `compile`, `assets`, `render {n, width?, codeFrames, codeErrors}`, `dispose`.
```
New:
```text
- Player worker: `plan`, `compile`, `assets`, `render {n, width?, codeFrames, codeErrors, png?}` (returns `{bitmap}` or `{blob}`), `dispose`.
```

Edit 3, section 7, the fenced block under `### boot.json`. E1j wrote the boot document of frozen section 3 without the three fields, and no later task edits the block, so its last line is the old text below.

Old:
```text
 "audio_info":{"narration":{}}}
```
New:
```text
 "audio_info":{"narration":{}},
 "workers":4,"chunk":8,"pre_roll":0}
```

Edit 4, section 7, the bullets under the boot block.

Old:
```text
- `frame_deadline_ms` is 5000: the per-frame deadline of render mode. The page uses it instead of 250 ms.
```
New:
```text
- `frame_deadline_ms` is 5000: the per-frame deadline of render mode. The page uses it instead of 250 ms.
- `workers`, `chunk` and `pre_roll` are left out when they have no effect. Absent means 1, no chunking, 0. Go computes them; see "Workers" below.
```

Edit 5, end of section 7. The old text is two lines with a blank line between them. It is unique.

Old:
```text
---

## 8. App and player iframe protocol
```
New:
```text
### Workers

Go chooses the worker count and the page runs it. `workers` 1 or no `workers` keeps the serial path.

- `ChooseWorkers(transport, logicalCPUs, freeMB, width, height, chunk, cjk, codeScenes)` lives in `internal/storymaker/render/workers.go`. A transport other than `png` gets 1, because worker encoding exists only for PNG. Otherwise `clamp(floor((cpus-2)/3), 1, 6)`, further bounded by `floor((freeMB - codeMB) / (60 + 2*chunk*frameMB + (cjk?120:0)))` when `freeMB > 0`, with `frameMB = 0.6*width*height/(1920*1080)` and `codeMB = codeScenes*(cjk?120:0)`, and at least 1.
- `cjk` is true when any drawn text in the story resolves to face `sm-cjk`, or any code scene has the language base `zh` or `ja`.
- `chunk` is 8. `pre_roll` is 0 by default and 2 when on.
- Before the report the page creates `workers-1` helper engines with `shared_code_scenes` and no bundles. Each loads and prepares. A different `frame_count` or schedule is `render_failed`. This happens before the report, so it cannot trip the 10 s frame watchdog.
- `H.runPool({engines, limit, chunk, preRoll, post, status}) -> outcome` lives in `ui/js/storymaker/host-pool.js`:
  - chunk j goes to worker `j % N`, and a chunk with code-scene frames goes to worker 0
  - a worker waits while its chunk starts `>= N*C*2` frames past the next frame to post
  - one in-order poster, one POST in flight
  - PNG encoded in the worker
  - on `restart`, stop the poster, abandon the generation, clear the buffer, re-run the same engines from frame 0
  - a crash or deadline rejects, and the page posts `/error`
- `TestParallelParityBrowser` pins identical RGBA for N workers against 1 on `firewall.json` and `speech.json`, with pre-roll on and off. `TestParallelEncoderFailureBrowser` forces one encoder failure, and the job restarts from frame 0.

---

## 8. App and player iframe protocol
```

Edit 6, section 9. Replace the whole line that starts with ``- Module loader (`ui/js/desktop/core/module-loader.js`):`` with:

```text
- Module loader (`ui/js/desktop/core/module-loader.js`): the style `desktop-app-storymaker.css` and sixteen scripts in order: `storymaker-core.js`, `storymaker-model.js`, `storymaker-api.js`, `storymaker-assets.js`, `storymaker-player.js`, `storymaker-projects.js`, `storymaker-strip.js`, `storymaker-inspector.js`, `storymaker-code.js`, `storymaker-export.js`, `storymaker-audio.js`, `storymaker-voice.js`, `storymaker-music.js`, `storymaker-dialog.js`, `storymaker-handoff.js`, `storymaker.js`. Each file is under 1100 lines.
```

Edit 7, section 9. This line is unchanged since phase 1.

Old:
```text
Not in phase 1: project rename, scene duplicate, undo and redo, drag and drop, the catalog tabs Text, Transitions, Stickers and Mine, beat editing, the agent panel, export options (quality, captions, stems), and the Video Studio handoff.
```
New:
```text
Not in phase 1: project rename, scene duplicate, undo and redo, drag and drop, the catalog tabs Text, Transitions, Stickers and Mine, beat editing, and the agent panel.

### Phase 2 screens

- Voice panel: provider select, voice select or free-text field, language select, audition of the first stored piece (it never synthesises), the `not tested` mark, the facts of `note`, one action that writes the voice into the story, the captions toggle.
- Narration status: `chars`, `missing`, progress from `pieces_*`, the Generate button, the Stop button `data-stm="narration-cancel"`, and the panel's own text for `interrupted` and `cancelled`. Stop shows only while the narration job is queued or running, is hidden when read-only, and calls `inst.api.cancelJob(job.id)`.
- Music panel: mode, mood, prompt, file picker, Noisemaker track, volume, and the Stop button `data-stm="music-cancel"`. Generated mode shows only with `status.music_configured` and runs only from an explicit button click. Stop shows only while the music job is queued or running, is hidden when read-only, and calls `inst.api.cancelJob(job.id)`.
- Scrubber waveform: a `.stm-scrub-wave` canvas inside `.stm-scrub`, painted from `S.audioModel.waveform`, `pointer-events:none`.
- Export dialog: a native `<dialog>` in `storymaker-dialog.js` with quality, burn-in captions (preset from `style.captions`), SRT, VTT, stems and Open in Video Studio. Blockers gain `narration` and `music`.
- Finished-job handoff: `storymaker-export.js` shows stems, SRT, VTT and library paths when present and each warning as text. `storymaker-handoff.js` offers the handoff on any finished job with stems (`S.handoff.canOffer`). The export panel reads the project's job list once when the editor opens and follows the `job` events, and its start view offers the newest finished render behind `data-stm="export-last"`, so a render that ended while no window tracked it keeps its result and its handoff. It never replaces the start view by itself.
```

- [ ] **Step 3: Apply the spec To verify edits**

All edits are in section "To verify in the plans" of `docs/superpowers/specs/2026-11-01-storymaker-design.md`. Each old text is unique and unchanged since the planning commit. The item "Colour and gain on 5.1.9" belongs to I3, which has already written its result, so do not edit it here.

Replace each marker that starts with `<from the Measured list` with the literal numbers from Step 1.

FFmpeg versions.

Old:
```text
`sidechaincompress`, `loudnorm` and `acrossfade` belong to phase 2 and stay open.
```
New:
```text
Phase 2 closes the rest. The mix uses neither `sidechaincompress` nor `acrossfade`: `internal/storymaker/render/mix.go` is pure Go, although both filters exist with the same options on both versions. The meter reads a float pipe with `ebur128` and `loudnorm` in one graph (`internal/storymaker/render/meter.go`, run on both versions by its test). The same pipe metering gave identical numbers on 8.1.1 and 5.1.9: -22.02 LUFS and -5.84 dBTP on a 900 s film (`docs/superpowers/plans/2026-11-04-storymaker-phase2/evidence/mixReal.md`).
```

Audio rates and voices.

Old:
```text
The length of Lyria output. **Open (phase 2).**
```
New:
```text
The length of Lyria output. **Resolved for local providers.** sanoTTS writes mono s16 WAV at 22050 Hz (24000 Hz for English), Phonon mono s16 WAV at 24000 Hz, the keyless google path mono MP3 at 24000 Hz (`docs/superpowers/plans/2026-11-04-storymaker-phase2/evidence/tts.md`). `internal/storymaker/audio_decode.go` reads WAV and MP3 by their bytes and the mix brings every clip to 48 kHz, so a cloud rate needs no answer here. Speech Lab uses the `/ready` snapshot and no per-call voice. The OpenAI list is the 13 voices of `OpenAITTSVoices`. Lyria: the loop and the trim cover any length.
```

Clip semantics.

Old:
```text
The `Clip.AudioDetached` semantics for the handoff video clip. **Open (phase 2).**
```
New:
```text
The `Clip.AudioDetached` semantics for the handoff video clip. **Resolved.** A clip with `audio_detached:true` needs a video asset with sound (`internal/videostudio/validate.go`). The MP4's AAC track always exists, so the handoff passes. The detached clip is silent on its video track and the audio clips play instead (`internal/videostudio/plan.go`). The output of `S.handoff.buildTracks` decodes strictly into `videostudio.Project` and passes `videostudio.Validate`.
```

Motion lint false positives.

Old:
```text
including the code-scene pixel thresholds (Motion lint). **Open.**
```
New:
```text
including the code-scene pixel thresholds (Motion lint). **Rates recorded, tuning open for phase 3.** Rate per scene of each static code on the fixtures, without audio: <from the Measured list: the codes and values of the line `Rate per scene:`>.
```

Parallel render memory, if W landed.

Old:
```text
on a CPU like aurago-test, which sets N. **Open.**
```
New:
```text
on a CPU like aurago-test, which sets N. **Resolved on the development box; aurago-test is not measured.** `firewall.json` at 1080p30 on 16 logical CPUs: 12-15 fps serial, 24 fps at N=4 with PNG on the host, 30-36 fps at N=4 and 36-41 fps at N=6 with PNG in the worker. Each added worker costs about 18 MB in the renderer plus about 9 MB of reorder buffer. The Chrome tree holds 773 MB at N=4 with host PNG and 876 MB at N=6 with worker PNG (`docs/superpowers/plans/2026-11-04-storymaker-phase2/evidence/parallelWorkers.md`). N comes from `ChooseWorkers` in `internal/storymaker/render/workers.go`. `TestParallelParityBrowser` holds on `firewall.json` and `speech.json`.
```

Parallel render memory, if W was dropped: the same old text, and this new text instead.
```text
on a CPU like aurago-test, which sets N. **Measured on the development box, not adopted in phase 2.** `firewall.json` at 1080p30 on 16 logical CPUs: 12-15 fps serial, 24 fps at N=4 with PNG on the host, 30-36 fps at N=4 and 36-41 fps at N=6 with PNG in the worker (`docs/superpowers/plans/2026-11-04-storymaker-phase2/evidence/parallelWorkers.md`). The parallel path was dropped from phase 2, so the render stays serial.
```

Ceiling and stems.

Old:
```text
The linear part is exact before encoding. **Open.**
```
New:
```text
The linear part is exact before encoding. **Resolved.** The stems sum to the mix exactly before encoding: the integer null test found 0 nonzero samples of 86.4 million at 900 s. After AAC the residual is about 18 dB below the mix at 64k and 23 dB at 192k, so the guarantee says "before encoding", and audio is AAC 192k. No overshoot in the measured cases: decoded true peak -1.48 dBTP on the synthetic film, -1.07 and -1.10 dBTP on real shapes A and B (`docs/superpowers/plans/2026-11-04-storymaker-phase2/evidence/mix.md`, `docs/superpowers/plans/2026-11-04-storymaker-phase2/evidence/mixReal.md`).
```

Rhythm factors.

Old:
```text
(calm 1.25, standard 1.0, lively 0.8) are tuned on the fixture stories. **Open.**
```
New:
```text
(calm 1.25, standard 1.0, lively 0.8) are tuned on the fixture stories. **Open for the user's eyes and ears.** The factors ship as first guesses.
```

Default fade.

Old:
```text
(iris and zoom play as fade until phase 4). **Open.**
```
New:
```text
(iris and zoom play as fade until phase 4). **Resolved.** A new scene gets `cut` as scene 1, `wipe` when its environment differs from the previous scene's, else `fade`. Stored scenes keep their transition, so a stored fade between worlds still gets `fade_between_worlds`.
```

Early arrival.

Old:
```text
Only arrivals, camera focus, a zoom with a target and charts lead in this revision. **Open.**
```
New:
```text
Only arrivals, camera focus, a zoom with a target and charts lead in this revision. **Partly resolved.** Rate of `cue_late` per scene on the fixtures, without audio: <from the Measured list: the `cue_late` value of the line `Rate per scene:`, then its sentence `Cued beats: N, with cue_late: N.`>. Whether an arrival reads as landed on its word, and whether emote and gesture beats should lead as well, stay open for the user's eyes.
```

Cue placement in speech.

Old:
```text
including de, el, hi and ja stories for the Unicode weights and the proportion fallback. **Open.**
```
New:
```text
including de, el, hi and ja stories for the Unicode weights and the proportion fallback. **Replaced.** The vowel-weight rule is dropped: it erred 581-730 ms mean against 341 ms for proportional placement (n=30). `cueMap` places a marker at a sentence start on that sentence's onset and proportionally over the sentence's speech inside it, and each block has `placed: span` or `proportion`. The span threshold stays -40 dBFS with gaps under 100 ms bridged. Share of cues placed by spans on hand-built blocks in de, el, hi and ja: <from the Measured list: the column `measured` of the table under `## Cue share, batch 2B (spec line 853)`, one value per language>. Those blocks prove the rule and its fallback. They do not measure a provider, so the share with a real voice stays open.
```

Mix limiter.

Old:
```text
The default is the linear gain. **Open.**
```
New:
```text
The default is the linear gain. **Resolved: no limiter.** Normal shapes land at -16.0 to -17.6 LUFS on real material (`docs/superpowers/plans/2026-11-04-storymaker-phase2/evidence/mixReal.md`). An sfx-only film ends near -22 LUFS, accepted.
```

Music shape checks.

Old:
```text
(huashu-motion cli/lib/sound.js:548-554). Not adopted in this revision. **Open.**
```
New:
```text
(huashu-motion cli/lib/sound.js:548-554). **Not adopted in phase 2.**
```

Glyph fallback.

Old:
```text
and how Go reads a WOFF2 cmap. **Open.**
```
New:
```text
and how Go reads a WOFF2 cmap. **Resolved.** Geist has only 6 Greek letters, so Inter Greek and Greek Extended come first with their `unicodeRange`; otherwise Chrome draws ω as Ω. No emoji face is added: emoji, Thai and Hebrew are `glyph_missing`. Go does not read WOFF2: `internal/storymaker/glyphs.go` reads the build-generated `ui/img/storymaker/glyphs.json` (`docs/superpowers/plans/2026-11-04-storymaker-phase2/evidence/fonts.md`).
```

CJK face in workers.

Old:
```text
The file is already in the asset set. **Open.**
```
New:
```text
The file is already in the asset set. **Resolved.** No subset, because a subset breaks the per-character check. `NotoSansCJKsc.otf` gets a named 17 MiB exception through `max_bytes`. Measured: 80-120 MiB per worker, 292-330 ms load per worker. The face loads only when compile needs it.
```

Readability ratio.

Old:
```text
exits stay at 0.35 s. **Open.**
```
New:
```text
exits stay at 0.35 s. **Partly resolved.** The phase 2 lint suite pins that title, caption and label clear 2.6 %. The phase 4 ratio stays open.
```

Per-language voice notes.

Old:
```text
and the wording of the notes. **Open.**
```
New:
```text
and the wording of the notes. **Open for the user's ears.** The notes hold facts only. The `not tested` mark ships on the provider and language, shown for every provider in phase 2.
```

Procedural effects.

Old:
```text
sit well beside the CC0 set on the fixture stories. **Open.**
```
New:
```text
sit well beside the CC0 set on the fixture stories. **Not built.** 25 WAVs rendered once with earshot ship in `assets/storymaker/sfx/`, with provenance in `assets/storymaker/README.md`. The build copies them. Their taste stays open for the user's ears.
```

- [ ] **Step 4: Record deviations, and the W drop if there was one**

Collect deviations from the commit bodies and from the batch reports the controller holds:

```powershell
git log --format=%B main..HEAD | Select-String -Pattern '(?i)deviat'
```

Then extend the end of section "Changes from the phase 2 plan" of the spec. The planning commit wrote that section from frozen section 8, and no later task edits it, so the old text below is its last line.

Old:
```text
- `audio/narration` grows with every edit and voice change, and nothing prunes it. A prune needs a rule for queued renders, whose `Plan` names hashes fixed at queue time. Deleting a project frees it.
```
New, with one bullet per deviation found. If there are none, keep the single bullet shown.
```text
- `audio/narration` grows with every edit and voice change, and nothing prunes it. A prune needs a rule for queued renders, whose `Plan` names hashes fixed at queue time. Deleting a project frees it.

**Deviations found during execution**
- None beyond the list above.
```

If W was dropped, add this bullet to the same list and delete "None beyond the list above.":

```text
- Parallel workers were dropped from phase 2. The render stays serial, `host-pool.js` and `workers.go` do not exist, and `boot.json` has no `workers`, `chunk` or `pre_roll`.
```

- [ ] **Step 5: Write the path check**

Write this file with the Write tool to the session scratchpad as `i4-check.cjs`.

```js
// Path and content check for task I4. Run from the worktree root.
const fs = require('fs');
const contract = fs.readFileSync('docs/superpowers/specs/2026-11-01-storymaker-contract.md', 'utf8');
const spec = fs.readFileSync('docs/superpowers/specs/2026-11-01-storymaker-design.md', 'utf8');
const problems = [];
const workersLanded = fs.existsSync('ui/js/storymaker/host-pool.js');
const evidence = 'docs/superpowers/plans/2026-11-04-storymaker-phase2/evidence/';
const paths = [
  'scripts/storymaker-bundle-parts.js',
  'ui/js/desktop/core/module-loader.js',
  'ui/js/desktop/apps/storymaker-core.js',
  'ui/js/desktop/apps/storymaker-model.js',
  'ui/js/desktop/apps/storymaker-api.js',
  'ui/js/desktop/apps/storymaker-assets.js',
  'ui/js/desktop/apps/storymaker-player.js',
  'ui/js/desktop/apps/storymaker-projects.js',
  'ui/js/desktop/apps/storymaker-strip.js',
  'ui/js/desktop/apps/storymaker-inspector.js',
  'ui/js/desktop/apps/storymaker-code.js',
  'ui/js/desktop/apps/storymaker-export.js',
  'ui/js/desktop/apps/storymaker-audio.js',
  'ui/js/desktop/apps/storymaker-voice.js',
  'ui/js/desktop/apps/storymaker-music.js',
  'ui/js/desktop/apps/storymaker-dialog.js',
  'ui/js/desktop/apps/storymaker-handoff.js',
  'ui/js/desktop/apps/storymaker.js',
  'internal/storymaker/render/mix.go',
  'internal/storymaker/render/meter.go',
  'internal/storymaker/audio_decode.go',
  'internal/storymaker/glyphs.go',
  'internal/videostudio/validate.go',
  'internal/videostudio/plan.go',
  'ui/img/storymaker/glyphs.json',
  'assets/storymaker/sfx',
  'assets/storymaker/README.md',
  evidence + 'mix.md',
  evidence + 'mixReal.md',
  evidence + 'tts.md',
  evidence + 'fonts.md',
  evidence + 'parallelWorkers.md'
];
if (workersLanded) paths.push('ui/js/storymaker/host-pool.js', 'internal/storymaker/render/workers.go');
for (const p of paths) if (!fs.existsSync(p)) problems.push('missing path: ' + p);

const need = (text, name, piece) => { if (!text.includes(piece)) problems.push(name + ' lacks: ' + piece); };
const refuse = (text, name, piece) => { if (text.includes(piece)) problems.push(name + ' still has: ' + piece); };

need(contract, 'contract', 'sixteen scripts in order');
need(contract, 'contract', '`storymaker-dialog.js`, `storymaker-handoff.js`, `storymaker.js`.');
need(contract, 'contract', '### Phase 2 screens');
refuse(contract, 'contract', 'export options (quality, captions, stems), and the Video Studio handoff');
if (workersLanded) {
  need(contract, 'contract', '"workers":4,"chunk":8,"pre_roll":0}');
  need(contract, 'contract', 'codeErrors, png?}');
  need(contract, 'contract', '### Workers');
  need(contract, 'contract', 'H.runPool(');
  const parts = fs.readFileSync('scripts/storymaker-bundle-parts.js', 'utf8');
  const hostList = parts.slice(parts.indexOf('storymakerHostParts'), parts.indexOf('storymakerPlayerParts'));
  const hostCount = (hostList.match(/ui\/js\/storymaker\//g) || []).length;
  need(contract, 'contract', 'host.bundle.js` (' + hostCount + ' parts');
} else {
  refuse(contract, 'contract', '### Workers');
  refuse(contract, 'contract', '"pre_roll"');
  need(spec, 'spec', 'Parallel workers were dropped from phase 2.');
}

const start = spec.indexOf('## To verify in the plans');
const end = spec.indexOf('## Changes after phase-1 drafting');
const section = spec.slice(start, end);
refuse(section, 'To verify', '<from the Measured list');
const closed = ['FFmpeg versions', 'Colour and gain on 5.1.9', 'Audio rates and voices', 'Clip semantics', 'Parallel render memory',
  'Ceiling and stems', 'Default fade', 'Cue placement in speech', 'Mix limiter', 'Music shape checks', 'Glyph fallback',
  'CJK face in workers', 'Procedural effects'];
for (const title of closed) {
  const line = section.split('\n').find(l => l.startsWith('- **' + title));
  if (!line) problems.push('To verify lacks the item: ' + title);
  else if (line.includes('**Open')) problems.push('To verify still open: ' + title);
}
need(spec, 'spec', '**Deviations found during execution**');

if (problems.length) {
  console.error(problems.join('\n'));
  process.exit(1);
}
console.log('I4 path check OK, workers landed: ' + workersLanded);
```

- [ ] **Step 6: Run the path check**

`$SP` is the session scratchpad folder.

```powershell
node "$SP/i4-check.cjs"
```

Expected: `I4 path check OK, workers landed: True` (or `False` when W was dropped).

- [ ] **Step 7: Commit**

```powershell
node C:/Users/Andi/.gitnexus/stores/AuraGo-08496d38b623/checkouts/AuraGo-08496d38b623/run.cjs detect-changes --scope all --repo C:/Users/Andi/Documents/repo/AuraGo # run alone, read the answer before git add
git add -f docs/superpowers/specs/2026-11-01-storymaker-contract.md docs/superpowers/specs/2026-11-01-storymaker-design.md
git commit -m "docs(storymaker): final phase 2 contract slice and To verify results" -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

Expected from `detect-changes`: no changed symbols from this task. It may list the uncommitted `ui/js/desktop/bundles/main.bundle.js` from E3f, which stays unstaged. `partial: true` on new files is accepted with a grep confirmation in the commit body. If the tool crashes, closes the connection, answers `not found`, or prints only its banner, use `graft` callers plus grep and say so in the commit body.

### Task I5: Root and child docs, manuals
**Batch:** 2D  **Depends on:** I4
**Files:**
- Modify: `AGENTS.md`
- Modify: `ui/js/storymaker/AGENTS.md`
- Modify: `internal/storymaker/AGENTS.md`
- Modify: `internal/storymaker/render/AGENTS.md`
- Modify: `ui/js/desktop/apps/AGENTS.md`
- Modify: `internal/server/AGENTS.md`
- Modify: `internal/desktop/AGENTS.md`
- Modify: `documentation/storymaker.md`
- Modify: `documentation/manuals/en/04-webui.md`
- Modify: `documentation/manuals/de/04-webui.md`

Docs only, so there is no failing test. The check is the path check in Step 13. Earlier slices (M16, E1j, E2f) have already changed these files, so most edits below anchor on a heading or a line start that those slices leave alone. Read each file before you edit it.

In this task, "W landed" means `ui/js/storymaker/host-pool.js` exists. Edits marked "W only" are skipped when W was dropped.

- [ ] **Step 1: Check for parallel work**

```powershell
Set-Location C:/Users/Andi/Documents/repo/AuraGo/.worktrees/storymaker-p2d
git -C C:/Users/Andi/Documents/repo/AuraGo status --short -- AGENTS.md ui/js/storymaker/AGENTS.md internal/storymaker/AGENTS.md internal/storymaker/render/AGENTS.md ui/js/desktop/apps/AGENTS.md internal/server/AGENTS.md internal/desktop/AGENTS.md documentation/storymaker.md documentation/manuals/en/04-webui.md documentation/manuals/de/04-webui.md
Test-Path ui/js/storymaker/host-pool.js
```

Expected: root `AGENTS.md` may be listed, because other sessions keep it dirty there. That matters only at landing, and I7 handles it. Any other listed file means another session is working there, so ask the controller first.

- [ ] **Step 2: Root row**

In `AGENTS.md`, replace the whole line that starts with `| Storymaker | ` with:

```text
| Storymaker | `internal/storymaker/AGENTS.md`, `internal/storymaker/render/AGENTS.md`, `ui/js/storymaker/AGENTS.md`, `ui/js/desktop/apps/AGENTS.md`, `internal/server/AGENTS.md`, `internal/desktop/AGENTS.md` | Explainer-video Desktop app: story document, validation and catalog index, project store, code-scene bundler, JS player (host, workers, compile, rig renderer, speech, captions), generated starter catalog with glyph data and sound effects (`assets/storymaker`, `ui/img/storymaker`), narration and music jobs, the Go mixer, render job and listener, export options, stems, subtitles, library copy and Video Studio handoff, app modules, config section and `documentation/storymaker.md`. |
```

- [ ] **Step 3: Player child doc (W only)**

In `ui/js/storymaker/AGENTS.md`:

Old:
```text
## Verify
```
New:
```text
## Parallel render

- `host-pool.js` sits in `host.bundle.js` between `host-app.js` and `host-render.js`. `H.runPool({engines, limit, chunk, preRoll, post, status}) -> outcome` sends chunk j to worker `j % N` and a chunk with code-scene frames to worker 0. A worker waits while its chunk starts `>= N*C*2` frames past the next frame to post. One in-order poster keeps one POST in flight, so Go's ingest is unchanged.
- On `restart` the poster stops, the generation is abandoned, the buffer is cleared and the same engines re-run from frame 0. A crash or deadline rejects and the page posts `/error`.
- `Engine.load` accepts `shared_code_scenes` and then builds no scene workers. `Engine.render(n, {png:true})` returns a Blob. `worker-player.js` `render` with `msg.png` returns `{blob}` from `main.canvas.convertToBlob({type:'image/png'})`.
- The page creates `workers-1` helper engines before the report, so they cannot trip the 10 s frame watchdog. A different `frame_count` or schedule is `render_failed`. `workers` 1 or absent keeps the serial path.
- Verify `node scripts/storymaker-tests/22-host-pool.mjs` through `npm run test:storymaker`, and `AURAGO_RUN_BROWSER_SMOKE=1 go test ./internal/storymaker/render -run '^TestParallelParityBrowser$' -count=1 -timeout 20m`.

## Verify
```

- [ ] **Step 4: Go core child doc**

In `internal/storymaker/AGENTS.md`:

Old:
```text
## Child DOX Index
```
New:
```text
## Phase 2 close

- Phase 2 (sound and export) landed in batches 2A to 2D. The contract is the reference, and the design spec section "Changes from the phase 2 plan" records every change to the original text.
- `testdata/stories/speech.json` (CJK, RTL, Greek, Hindi, bubbles) is shared with the player tests and the render smoke tests, so an edit there reaches both.
- Still open for the user's ears or hands: voice notes and rhythm factors, sentence pause and trim margins, the taste of the SFX set, prosody with per-sentence synthesis, and the aurago-test acceptance. Agent voice discovery (`storymaker_catalog kind=voice`) is phase 3.

## Child DOX Index
```

- [ ] **Step 5: Render child doc**

In `internal/storymaker/render/AGENTS.md`, W only:

Old:
```text
- **Tests**:
```
New:
```text
- **Workers** (`workers.go`): `ChooseWorkers(transport, logicalCPUs, freeMB, width, height, chunk, cjk, codeScenes)` sets `PageParams.Workers`. Chunk is 8, pre-roll 0 by default and 2 when on. All three go into `boot.json` with `omitempty`. `Settings.LogicalCPUs` and `Settings.FreeMemoryMB` come from the server, and `FreeMemoryMB` 0 means no memory bound. Only `png` runs parallel, because worker encoding exists only for PNG. Ingest is unchanged: one in-order poster, one POST in flight, and the restart stays an answer to `/frame`. `TestParallelParityBrowser` pins identical RGBA for N workers against 1 on `firewall.json` and `speech.json`, pre-roll on and off. `TestParallelEncoderFailureBrowser` forces one encoder failure.
- **Tests**:
```

In the same file, for both W states. The old text is two lines, and no earlier task edits this sentence.

Old:
```text
(the 5.1.9 floor is
  still unmeasured).
```
New:
```text
(the 5.1.9 floor is
  measured since phase 2: swatch and frame gate in `ffmpeg_test.go`, one `debian:12 --rm` container).
```

- [ ] **Step 6: App child doc**

In `ui/js/desktop/apps/AGENTS.md`, replace the whole line that starts with ``- `storymaker*.js` are`` with:

```text
- `storymaker*.js` are sixteen classic scripts in this load order (module-loader entry `storymaker`, pinned by `TestDesktopStorymakerRegistration` and `TestStorymakerLoaderListsEveryScript`): `storymaker-core.js` (the namespace `window.StorymakerParts`, the DOM builder `S.el` that has no html option, icons, the bus, error texts), `-model.js` (pure: JSON Pointer to text position, issue lists, the catalog index, the story edits), `-api.js`, `-assets.js` (catalog, fonts and code bundles for the player), `-player.js` (the iframe bridge), `-projects.js`, `-strip.js` (scene strip and transport), `-inspector.js` (inspector and library), `-code.js` (CodeMirror), `-export.js` (export panel and finished-job actions), `-audio.js` (preview audio, waveform model), `-voice.js` (voice panel, narration status, captions toggle), `-music.js` (music panel), `-dialog.js` (export dialog), `-handoff.js` (tracks and handoff), then the composition root `storymaker.js` (`window.StorymakerApp`). Style `css/desktop-app-storymaker.css`: `stm-` prefix, tokens only, container queries on the window, motion gated. The backend contract is `internal/storymaker/AGENTS.md`, the player `ui/js/storymaker/AGENTS.md`.
```

In the same file:

Old:
```text
## Email exports
```
New:
```text
### Storymaker export dialog and handoff

- `S.createExportDialog(inst) -> {open(), close(), dispose()}` is a native `<dialog>` built with `el()` and `showModal()`. Fields: quality, burn-in captions (preset from `style.captions`), SRT, VTT, stems, Open in Video Studio. Open in Video Studio forces `stems:true` and adds the job id to `inst.handoffWish` (a `Set`, session only). `createExport` keeps the button and the finished-job actions, and calls `inst.api.startRender(id, etag, key, options)`. Blockers gain `narration` and `music`, from `audioInfo`.
- Finished-job actions show stems, SRT, VTT and library paths when present, and each warning as text. A job without extras shows only the MP4. The start view reads `inst.api.listJobs` once and offers the newest `succeeded` or `interrupted` render as `data-stm="export-last"`, and `apply(job, true)` shows it in the same views. A replayed finished job stays out of the panel unless its id is in `inst.handoffWish`, and the wish ends with the job. A render that ended while no window tracked it stays on offer: `createExport` reads `inst.api.listJobs` once and follows `job` events, and the start view shows the newest finished render behind `data-stm="export-last"`. A replayed job never replaces the start view.
- `S.handoff.canOffer(job, vsStatus)`, `S.handoff.run(inst, job) -> Promise<{projectId}>` and the pure `S.handoff.buildTracks(project, assets, frameCount, names)`. Handoff is a client action on any finished job with stems, so a closed window cannot lose it, and no `handoff` render option exists. `run` uses only the existing `/api/desktop/video-studio` routes: `Idempotency-Key: storymaker-<job id>-<name>`, `folder` empty, job poll every 500 ms up to 120 s, a whole-project PUT with `If-Match` and version 3, then `inst.ctx.openApp('video-studio', {projectId})`.
- `window.VideoStudioApp.openProject(windowId, projectId)` stores `s.ctx.projectId` before `studioReady`. Once ready it runs `await loadProjects(s)` and then `switchProject` when the id is listed, so the dirty guard runs. `initialize` prefers a listed `s.ctx.projectId`. The reuse branch in `core/window-shell-runtime.js` calls `openProject` or stores `pendingProjectId`, and `core/menus-and-routing.js` merges it at launch. Both shell files are parts of `main.bundle.js`, so run `npm run build:ui` after an edit.
- Verify `go test ./ui -run '(?i)(storymaker|videostudio)' -count=1 -timeout 20m` and `AURAGO_RUN_BROWSER_SMOKE=1 go test ./internal/server -run '^TestStorymakerAppRealServerBrowser$' -count=1 -timeout 15m`. The track builder runs in Node against the Go validator in `ui/desktop_storymaker_handoff_test.go`, and the launch parameter in `ui/desktop_video_studio_open_project_test.go`.

## Email exports
```

- [ ] **Step 7: Server child doc**

In `internal/server/AGENTS.md`. Delete the line that starts with `- Parallel render:` when W was dropped.

Old:
```text
### Retro-Net Terminal
```
New:
```text
### Storymaker export and handoff

- Parallel render: `storymakerSettings` passes `LogicalCPUs` (`runtime.NumCPU()`) and `FreeMemoryMB` (gopsutil `mem.VirtualMemory().Available >> 20`, capped by `render.BoundMemoryToCgroup` over `/sys/fs/cgroup`, 0 when unknown) to the render runner. `render.ChooseWorkers` decides the worker count; the handlers do not.
- The Video Studio handoff adds no Storymaker route. The app calls the existing `/api/desktop/video-studio` routes, so their scopes, read-only checks and `max_asset_size_mb` hold. `source_path` passes `OpenPreviewFile` before anything is copied into the Video Studio project.
- Verify `go test ./internal/server -run Storymaker -count=1 -timeout 20m` and `AURAGO_RUN_BROWSER_SMOKE=1 go test ./internal/server -run '^TestStorymakerAppRealServerBrowser$' -count=1 -timeout 15m`.

### Retro-Net Terminal
```

- [ ] **Step 8: Desktop child doc**

In `internal/desktop/AGENTS.md`:

Old:
```text
- Desktop cloud transfers use `StatStoragePath` for fresh rooted metadata,
```
New:
```text
- The Storymaker handoff adds no Service method. Video Studio's `source_path` import opens files below `Documents/Storymaker/<id>/exports/` through `OpenPreviewFile` and copies them into its project. The library copy (`WriteStorymakerVideo`) exists for discoverability only: camera import accepts top-level `Videos/<file>` only, so the handoff never reads `Videos/Storymaker/`. Verify `go test ./internal/desktop -run WriteStorymakerVideo -count=1`.

- Desktop cloud transfers use `StatStoragePath` for fresh rooted metadata,
```

- [ ] **Step 9: User page, the opening**

In `documentation/storymaker.md`, replace everything between the line `# Storymaker` and the line `## Switch it on` with this text, keeping one blank line on each side:

```text
Storymaker is the Virtual Desktop app for short explainer videos. You stage scenes with cartoon people, props and
environments from a built-in catalog, write the narration, pick a voice and music, watch a live preview and export an MP4 with
sound, captions, subtitle files and separate audio tracks. A finished export can continue in Video Studio. This page describes
phases 1 and 2. The agent panel, the chat tool and the asset generator come in later phases.
```

- [ ] **Step 10: User page, the export extras and the end of the page**

In `documentation/storymaker.md`, E1j renamed the section `## Not in phase 1` to `## Not yet`, and E2f added the section `## Sound` between it and the line `## For developers`. Replace everything from the line `## Not yet` up to, but not including, the line `## For developers` with the text below. The text keeps E2f's `## Sound` in full, except its last sentence, because the dialog exists now. When W was dropped, leave out the section `## Export speed`.

```text
## Sound

The export mixes narration, music and sound effects in AuraGo itself and writes AAC 192k, about -16 LUFS at -1 dBTP at most. The music ducks under speech. An `sfx` beat plays a sound from the catalog (`sfx.whoosh`, `sfx.pop`, `sfx.click`, `sfx.ding`, `sfx.success`, `sfx.error`, `sfx.typing`) on its landing, and `style.auto_sfx` adds a pop to each show with entry `pop`.

An export is refused while narration audio or the music file is missing. The export panel says what is missing and opens the panel that fixes it. A scene marked as silent on the Scene tab exports without its narration and keeps the text, while a `say` with its own text still needs audio. A film without narration, music and sound effects exports with a silent track, and the panel says so before you start.

`POST /api/desktop/storymaker/projects/{id}/render` takes an optional body `{burn_in_captions, srt, vtt, stems, quality}`. Stems and subtitles land in `exports/<title>-<job>/`, and a copy of the MP4 lands in `Videos/Storymaker/`. A failed extra leaves the MP4 and adds a warning to the job. The export dialog below sets these options.

## Export options and extras

Starting an export opens a dialog with these choices:

| Choice | Meaning |
| --- | --- |
| Quality | `standard`, or `high` for a better picture and a larger file. |
| Burned-in captions | Draws the captions into the picture. The preset follows the captions switch of the story. |
| SRT, VTT | Writes subtitle files next to the video. |
| Stems | Writes one WAV per audio layer: narration, music, sound effects. A silent layer is left out. |
| Open in Video Studio | Turns stems on, so the finished export can continue in Video Studio. |

An export does not start while narration or music is missing. The panel says what is missing and leads to the voice or the
music panel.

The files of one export:

    Documents/Storymaker/<project id>/exports/
      <slug>-<job8>.mp4
      <slug>-<job8>/narration.wav  music.wav  sfx.wav  captions.srt  captions.vtt
      <job id>.json
    Videos/Storymaker/<slug>.mp4, <slug>-2.mp4, ...

The copy in `Videos/Storymaker/` makes the film easy to find. A failed extra leaves the MP4 in place and shows a warning on the
finished export. The stems add up to the mix exactly before encoding. The AAC track in the MP4 differs slightly, about 23 dB below
the mix.

## Continue in Video Studio

A finished export with stems offers **Open in Video Studio** when Video Studio is on, FFmpeg is ready and the desktop is
writable. The action creates a Video Studio project, copies the MP4 and each stem into it, and opens it. The film sits on a video
track with its own sound detached, and each stem gets its own audio track. This also works after you left the project or closed the
Storymaker window: open the project again and choose **Show the last export** in the export panel, which offers the newest finished
export. Unsaved work in an open Video Studio project is saved first.

## Export speed

With the `png` transport the export draws frames in several workers: `(CPUs - 2) / 3`, rounded down, between 1 and 6, and fewer
when free memory is short. A machine with 8 logical CPUs uses 2 workers. The frames are identical to a serial export.

## Not yet in Storymaker

The agent panel and chat tool, the asset generator, charts and diagrams, most other transitions (cut, fade, slide-left and wipe
exist) and drag and drop staging. The story format already accepts the fields of later phases; the editor shows them as "not
supported yet" and the player ignores them. Nobody has listened to the voices yet, so every provider and language shows a
`not tested` mark.

```

Then replace everything from the line `## For developers` to the end of the file with:

```text
## For developers

`ui/js/desktop/apps/AGENTS.md` (Storymaker), `internal/storymaker/AGENTS.md`, `internal/storymaker/render/AGENTS.md` and
`ui/js/storymaker/AGENTS.md` hold the contracts. `docs/superpowers/specs/2026-11-01-storymaker-contract.md` is the reference.

- Player, in Node: `npm run test:storymaker`.
- The app tour against a stand-in server: `AURAGO_RUN_BROWSER_SMOKE=1 go test ./ui -run TestStorymakerApp -count=1`.
- The same app against the real handlers, ending in a rendered MP4 with sound and the Video Studio handoff:
  `AURAGO_RUN_BROWSER_SMOKE=1 go test ./internal/server -run '^TestStorymakerAppRealServerBrowser$' -count=1 -timeout 15m`.
- App player against render page: `AURAGO_RUN_BROWSER_SMOKE=1 go test ./internal/server -run '^TestStorymakerParityAppPlayerAndRenderPage$' -count=1 -timeout 15m`.
- Parallel workers: `AURAGO_RUN_BROWSER_SMOKE=1 go test ./internal/storymaker/render -run '^TestParallelParityBrowser$' -count=1 -timeout 20m`.

FFmpeg and Chrome must be installed for the browser lines.
```

When W was dropped, leave out the line that starts with `- Parallel workers:`.

- [ ] **Step 11: Manuals**

In `documentation/manuals/en/04-webui.md`:

Old:
```text
| **Storymaker** | Explainer videos: stage scenes with cartoon people, props and environments, write the narration, watch a live preview and export a silent MP4 (needs FFmpeg and a browser for the export) — see [Storymaker](../../storymaker.md) |
```
New:
```text
| **Storymaker** | Explainer videos: stage scenes with cartoon people, props and environments, write the narration, pick a voice and music, watch a live preview and export an MP4 with sound, captions and separate audio tracks, then continue in Video Studio (needs FFmpeg and a browser for the export) — see [Storymaker](../../storymaker.md) |
```

In `documentation/manuals/de/04-webui.md`:

Old:
```text
| **Storymaker** | Erklärvideos: Szenen mit Comicfiguren, Requisiten und Umgebungen inszenieren, den Sprechtext schreiben, die Vorschau ansehen und ein stummes MP4 exportieren (der Export braucht FFmpeg und einen Browser), siehe [Storymaker](../../storymaker.md) |
```
New:
```text
| **Storymaker** | Erklärvideos: Szenen mit Comicfiguren, Requisiten und Umgebungen inszenieren, den Sprechtext schreiben, Stimme und Musik wählen, die Vorschau ansehen und ein MP4 mit Ton, Untertiteln und getrennten Tonspuren exportieren, danach in Video Studio weiterschneiden (der Export braucht FFmpeg und einen Browser), siehe [Storymaker](../../storymaker.md) |
```

- [ ] **Step 12: Write the path check**

Write this file with the Write tool to the session scratchpad as `i5-check.cjs`.

```js
// Path and content check for task I5. Run from the worktree root.
const fs = require('fs');
const read = p => fs.readFileSync(p, 'utf8');
const problems = [];
const workersLanded = fs.existsSync('ui/js/storymaker/host-pool.js');
const paths = [
  'internal/storymaker/AGENTS.md', 'internal/storymaker/render/AGENTS.md', 'ui/js/storymaker/AGENTS.md',
  'ui/js/desktop/apps/AGENTS.md', 'internal/server/AGENTS.md', 'internal/desktop/AGENTS.md', 'documentation/storymaker.md',
  'docs/superpowers/specs/2026-11-01-storymaker-contract.md',
  'internal/storymaker/testdata/stories/speech.json', 'internal/storymaker/render/ffmpeg_test.go',
  'ui/js/desktop/core/window-shell-runtime.js', 'ui/js/desktop/core/menus-and-routing.js',
  'ui/js/desktop/apps/storymaker-dialog.js', 'ui/js/desktop/apps/storymaker-handoff.js',
  'ui/desktop_storymaker_handoff_test.go', 'ui/desktop_video_studio_open_project_test.go'
];
if (workersLanded) {
  paths.push('ui/js/storymaker/host-pool.js', 'internal/storymaker/render/workers.go', 'scripts/storymaker-tests/22-host-pool.mjs');
}
for (const p of paths) if (!fs.existsSync(p)) problems.push('missing path: ' + p);

const need = (file, piece) => { if (!read(file).includes(piece)) problems.push(file + ' lacks: ' + piece); };
const refuse = (file, piece) => { if (read(file).includes(piece)) problems.push(file + ' still has: ' + piece); };

need('AGENTS.md', '`internal/storymaker/render/AGENTS.md`, `ui/js/storymaker/AGENTS.md`');
need('AGENTS.md', 'Video Studio handoff');
need('internal/storymaker/AGENTS.md', '## Phase 2 close');
need('ui/js/desktop/apps/AGENTS.md', 'sixteen classic scripts');
need('ui/js/desktop/apps/AGENTS.md', '### Storymaker export dialog and handoff');
need('internal/server/AGENTS.md', '### Storymaker export and handoff');
need('internal/desktop/AGENTS.md', 'The Storymaker handoff adds no Service method.');
need('documentation/storymaker.md', '## Export options and extras');
need('documentation/storymaker.md', '## Continue in Video Studio');
need('documentation/storymaker.md', '## Not yet in Storymaker');
need('documentation/storymaker.md', '## Sound');
refuse('documentation/storymaker.md', 'comes with the next update');
refuse('documentation/storymaker.md', 'first version (phase 1)');
refuse('documentation/storymaker.md', 'Phase 1 videos have no sound');
need('documentation/manuals/en/04-webui.md', 'then continue in Video Studio');
refuse('documentation/manuals/en/04-webui.md', 'export a silent MP4');
need('documentation/manuals/de/04-webui.md', 'danach in Video Studio weiterschneiden');
refuse('documentation/manuals/de/04-webui.md', 'stummes MP4');
if (workersLanded) {
  need('ui/js/storymaker/AGENTS.md', '## Parallel render');
  need('internal/storymaker/render/AGENTS.md', '- **Workers** (`workers.go`)');
  need('internal/server/AGENTS.md', 'Parallel render: `storymakerSettings`');
  need('documentation/storymaker.md', '## Export speed');
} else {
  refuse('documentation/storymaker.md', '## Export speed');
  refuse('internal/server/AGENTS.md', 'Parallel render: `storymakerSettings`');
}

const tests = [
  ['ui/desktop_storymaker_registration_test.go', 'func TestDesktopStorymakerRegistration('],
  ['ui/desktop_storymaker_guards_test.go', 'func TestStorymakerLoaderListsEveryScript('],
  ['internal/server/storymaker_app_browser_test.go', 'func TestStorymakerAppRealServerBrowser('],
  ['internal/server/storymaker_parity_browser_test.go', 'func TestStorymakerParityAppPlayerAndRenderPage(']
];
if (workersLanded) {
  tests.push(['internal/storymaker/render/browser_smoke_test.go', 'func TestParallelParityBrowser(']);
  tests.push(['internal/storymaker/render/browser_smoke_test.go', 'func TestParallelEncoderFailureBrowser(']);
}
for (const [file, piece] of tests) need(file, piece);

if (problems.length) {
  console.error(problems.join('\n'));
  process.exit(1);
}
console.log('I5 path check OK, workers landed: ' + workersLanded);
```

- [ ] **Step 13: Run the path check**

`$SP` is the session scratchpad folder.

```powershell
node "$SP/i5-check.cjs"
```

Expected: `I5 path check OK, workers landed: True` (or `False` when W was dropped).

- [ ] **Step 14: Commit**

```powershell
node C:/Users/Andi/.gitnexus/stores/AuraGo-08496d38b623/checkouts/AuraGo-08496d38b623/run.cjs detect-changes --scope all --repo C:/Users/Andi/Documents/repo/AuraGo # run alone, read the answer before git add
git add AGENTS.md ui/js/storymaker/AGENTS.md internal/storymaker/AGENTS.md internal/storymaker/render/AGENTS.md ui/js/desktop/apps/AGENTS.md internal/server/AGENTS.md internal/desktop/AGENTS.md documentation/storymaker.md documentation/manuals/en/04-webui.md documentation/manuals/de/04-webui.md
git commit -m "docs(storymaker): phase 2 child docs, user page and manuals" -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

Expected from `detect-changes`: no changed symbols from this task. It may list the uncommitted `ui/js/desktop/bundles/main.bundle.js` from E3f, which stays unstaged. `partial: true` on new files is accepted with a grep confirmation in the commit body. If the tool crashes, closes the connection, answers `not found`, or prints only its banner, use `graft` callers plus grep and say so in the commit body.

### Task I6: Asset-pack scratch instance
**Batch:** 2D  **Depends on:** I2
**Files:**
- Create (scratch, never committed): `D:\tmp\go-build\sm-i6\src\_i6\main.go`
- Create: `docs/superpowers/plans/2026-11-04-storymaker-phase2/evidence/assetpack.md`

This task proves the shipped build: login, the sanotts voice, narration, export with stems and subtitles, and the handoff into the real Video Studio. The panels and the dialog are proven by the I2 tour, so the runner uses the frozen routes and `S.handoff.run` with the real app instance.

Run every command with the shell sandbox off, because a repo-built executable cannot start Chrome elsewhere on this box. Stay under `D:\tmp\go-build\sm-i6`, because a long run dir breaks the bundled ONNX runtime. Do not use phonon: its Windows `ptts.exe` cannot write here.

- [ ] **Step 1: Snapshot the branch and build**

The committed `main.bundle.js` is older than its parts until the last commit of 2D. So copy the copy that E3f rebuilt into the snapshot before the asset pack, or the handoff runs against an old shell.

```powershell
$env:GOCACHE = 'D:\dev-cache\go-build'
$env:GOTMPDIR = 'D:\tmp\go-build'
$WT = 'C:/Users/Andi/Documents/repo/AuraGo/.worktrees/storymaker-p2d'
$I6 = 'D:\tmp\go-build\sm-i6'
New-Item -ItemType Directory -Force -Path "$I6/src", "$I6/shots" | Out-Null
git -C $WT archive --format=tar -o "$I6/src.tar" storymaker-p2d
C:\Windows\System32\tar.exe -x -f "$I6/src.tar" -C "$I6/src"
Copy-Item -LiteralPath "$WT/ui/js/desktop/bundles/main.bundle.js" -Destination "$I6/src/ui/js/desktop/bundles/main.bundle.js" -Force
Set-Location "$I6/src"
go run ./cmd/assetpack -out deploy -stage assets/web
go build -trimpath -ldflags "-s -w $(Get-Content -Raw deploy/web-assets.ldflags)" -o "$I6/aurago.exe" ./cmd/aurago
```

Expected: `$I6/aurago.exe` exists and `deploy` holds one `aurago-web-assets-*.tar.gz`.

- [ ] **Step 2: Install and check the assets**

```powershell
Set-Location $I6
$pack = Get-ChildItem -Path "$I6/src/deploy" -Filter 'aurago-web-assets-*.tar.gz' | Select-Object -First 1
& "$I6/aurago.exe" -install-assets $pack.FullName
& "$I6/aurago.exe" -check-assets
$LASTEXITCODE
```

Expected: `0`.

- [ ] **Step 3: Write the config**

Text replacement only, with no YAML round trip. The template's active provider is phonon and Video Studio is off, so both change here as well.

```powershell
$c = Get-Content -Raw -Encoding UTF8 "$I6/src/config_template.yaml"
$c = $c -replace '(?m)^server:\r?\n', "server:`n    ui_language: en`n"
$c = $c -replace '(?m)^    port: 8088\b', '    port: 18733'
$c = $c -replace '(?m)^    https:\r?\n        enabled: true', "    https:`n        enabled: false"
$c = $c -replace '(?m)^    provider: phonon\b.*$', '    provider: sanotts'
$c = $c -replace '(?m)^video_studio:\r?\n    enabled: false', "video_studio:`n    enabled: true"
$c = $c -replace '(?m)^( +browser_path: )""', '$1"C:/Program Files/Google/Chrome/Application/chrome.exe"'
[IO.File]::WriteAllText((Join-Path $I6 'config.yaml'), $c, (New-Object System.Text.UTF8Encoding($false)))
Select-String -Path "$I6/config.yaml" -Pattern '^    ui_language: en', '^    port: 18733', '^    provider: sanotts', 'browser_path: "C:/Program Files'
```

Expected: four matches. Also check by eye that `server.https.enabled` is `false` and `video_studio.enabled` is `true`.

- [ ] **Step 4: First start with a password**

The master key and `DOCKER_HOST` go into the child's environment only. `DOCKER_HOST` points at a dead port, so startup cleanup cannot touch a live Docker.

```powershell
Set-Location $I6
$key = -join (1..32 | ForEach-Object { '{0:x2}' -f (Get-Random -Minimum 0 -Maximum 256) })
$pw = [guid]::NewGuid().ToString('N')
Set-Content -Path (Join-Path $I6 'master.key') -Value $key -NoNewline
Set-Content -Path (Join-Path $I6 'password.txt') -Value $pw -NoNewline
$env:AURAGO_MASTER_KEY = $key
$env:DOCKER_HOST = 'tcp://127.0.0.1:1'
$p = Start-Process -FilePath (Join-Path $I6 'aurago.exe') -ArgumentList '-password', $pw -WorkingDirectory $I6 -RedirectStandardOutput (Join-Path $I6 'stdout.log') -RedirectStandardError (Join-Path $I6 'stderr.log') -PassThru -WindowStyle Hidden
Remove-Item Env:AURAGO_MASTER_KEY
Remove-Item Env:DOCKER_HOST
Set-Content -Path (Join-Path $I6 'aurago.pid') -Value $p.Id
$up = $false
foreach ($i in 1..60) {
    try {
        if ((Invoke-WebRequest -UseBasicParsing -TimeoutSec 5 'http://127.0.0.1:18733/auth/login').StatusCode -eq 200) { $up = $true; break }
    } catch { Start-Sleep -Seconds 2 }
}
if (-not $up) { Get-Content (Join-Path $I6 'stderr.log') -Tail 40; throw 'the scratch instance did not come up' }
'up'
```

Expected: `up`. If port 18733 is taken, change it here and in Step 3.

- [ ] **Step 5: Write the runner**

Write this file with the Write tool as `D:\tmp\go-build\sm-i6\src\_i6\main.go`. A folder that starts with `_` stays out of `./...`.

```go
// I6 acceptance runner: drives a running asset-pack build of AuraGo in the installed Chrome. Not part of the repository.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/launcher"
)

const (
	storymakerAPI  = "/api/desktop/storymaker"
	videoStudioAPI = "/api/desktop/video-studio"
	projectTitle   = "I6 acceptance"
	// Three sentences of 38, 43 and 47 characters, each with more than 3 words. No fragment merges, so three pieces.
	narrationText = "Anna works from home every single day. Suddenly somebody knocks at the front door. She opens it and greets the friendly neighbour."
)

// trapJS records page errors and CSP violations, and keeps the instance that window.StorymakerApp.render returns.
const trapJS = `(() => {
	window.__i6errors = [];
	addEventListener('error', e => window.__i6errors.push('error: ' + e.message));
	addEventListener('unhandledrejection', e => window.__i6errors.push('rejection: ' + String(e.reason)));
	addEventListener('securitypolicyviolation', e => window.__i6errors.push('csp: ' + e.violatedDirective + ' ' + e.blockedURI));
	let real;
	Object.defineProperty(window, 'StorymakerApp', {
		configurable: true,
		get() { return real; },
		set(value) {
			const render = value.render;
			value.render = (host, windowId, ctx) => {
				const inst = render(host, windowId, ctx);
				window.__i6inst = inst;
				return inst;
			};
			real = value;
		}
	});
})();`

const fetchJS = `async (method, url, body, headers) => {
	const init = {method, headers: JSON.parse(headers) || {}, credentials: 'same-origin', cache: 'no-store'};
	if (body !== '') init.body = body;
	const response = await fetch(url, init);
	return JSON.stringify({status: response.status, text: await response.text()});
}`

const audioJS = `async (url) => {
	const response = await fetch(url, {credentials: 'same-origin'});
	const bytes = await response.arrayBuffer();
	return JSON.stringify({status: response.status, type: response.headers.get('Content-Type'), bytes: bytes.byteLength});
}`

const handoffJS = `async (text) => {
	const S = window.StorymakerParts, inst = window.__i6inst, job = JSON.parse(text);
	const status = await (await fetch('/api/desktop/video-studio/status', {credentials: 'same-origin', cache: 'no-store'})).json();
	if (!S.handoff.canOffer(job, status)) {
		return JSON.stringify({offered: false, status: {enabled: status.enabled, desktop_enabled: status.desktop_enabled, ffmpeg_ready: status.ffmpeg_ready, read_only: status.read_only}});
	}
	const out = await S.handoff.run(inst, job);
	return JSON.stringify({offered: true, projectId: out.projectId});
}`

type answer struct {
	Status int    `json:"status"`
	Text   string `json:"text"`
}

type stemsView struct {
	Narration string `json:"narration"`
	Music     string `json:"music"`
	SFX       string `json:"sfx"`
}

type outputView struct {
	Path    string     `json:"path"`
	Size    int64      `json:"size"`
	Stems   *stemsView `json:"stems"`
	SRT     string     `json:"srt"`
	VTT     string     `json:"vtt"`
	Library string     `json:"library"`
}

type messageView struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type jobView struct {
	ID          string        `json:"id"`
	Status      string        `json:"status"`
	FramesTotal int           `json:"frames_total"`
	Output      *outputView   `json:"output"`
	Warnings    []messageView `json:"warnings"`
	Error       *messageView  `json:"error"`
}

func (j jobView) failure() string {
	if j.Error == nil {
		return "no error text"
	}
	return j.Error.Code + ": " + j.Error.Message
}

type projectView struct {
	Story struct {
		Voice *struct {
			Provider string `json:"provider"`
			Language string `json:"language"`
		} `json:"voice"`
		Scenes []struct {
			ID        string `json:"id"`
			Narration string `json:"narration"`
		} `json:"scenes"`
	} `json:"story"`
	ETag string `json:"etag"`
}

func fail(format string, args ...any) { panic(fmt.Sprintf(format, args...)) }

func must(err error) {
	if err != nil {
		panic(err)
	}
}

func call(page *rod.Page, method, url, body string, headers map[string]string) answer {
	raw, err := json.Marshal(headers)
	must(err)
	text := page.Timeout(2*time.Minute).MustEval(fetchJS, method, url, body, string(raw)).Str()
	var a answer
	must(json.Unmarshal([]byte(text), &a))
	return a
}

func get(page *rod.Page, url string) answer {
	a := call(page, "GET", url, "", nil)
	if a.Status != 200 {
		fail("GET %s answered %d: %s", url, a.Status, a.Text)
	}
	return a
}

func decode(a answer, into any) { must(json.Unmarshal([]byte(a.Text), into)) }

func wait(page *rod.Page, expr string, limit time.Duration, params ...any) {
	page.Timeout(limit).MustWait(expr, params...)
}

func click(page *rod.Page, selector string) {
	wait(page, `(s)=>!!document.querySelector(s)`, 90*time.Second, selector)
	page.MustEval(`(s)=>document.querySelector(s).click()`, selector)
}

func waitJob(page *rod.Page, id string, limit time.Duration) (jobView, string) {
	deadline := time.Now().Add(limit)
	for {
		var body struct {
			Job json.RawMessage `json:"job"`
		}
		decode(get(page, storymakerAPI+"/jobs/"+id), &body)
		var job jobView
		must(json.Unmarshal(body.Job, &job))
		switch job.Status {
		case "succeeded", "failed", "cancelled", "interrupted":
			return job, string(body.Job)
		}
		if time.Now().After(deadline) {
			fail("job %s still %s after %s", id, job.Status, limit)
		}
		time.Sleep(2 * time.Second)
	}
}

func startedJob(a answer, what string) jobView {
	if a.Status != 202 {
		fail("%s answered %d: %s", what, a.Status, a.Text)
	}
	var body struct {
		Job jobView `json:"job"`
	}
	decode(a, &body)
	return body.Job
}

func readProject(page *rod.Page, id string) projectView {
	var p projectView
	decode(get(page, storymakerAPI+"/projects/"+id), &p)
	return p
}

func main() {
	base := flag.String("base", "", "origin of the scratch instance")
	password := flag.String("password", "", "login password")
	shots := flag.String("shots", "", "folder for screenshots")
	flag.Parse()
	if *base == "" || *password == "" || *shots == "" {
		fmt.Println("usage: i6runner -base <origin> -password <password> -shots <folder>")
		os.Exit(2)
	}
	if err := run(*base, *password, *shots); err != nil {
		fmt.Println("FAIL:", err)
		os.Exit(1)
	}
	fmt.Println("PASS")
}

func run(base, password, shots string) (err error) {
	if err := os.MkdirAll(shots, 0o755); err != nil {
		return err
	}
	chrome, ok := launcher.LookPath()
	if !ok {
		return fmt.Errorf("no Chrome is installed")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 40*time.Minute)
	defer cancel()
	l := launcher.New().Context(ctx).Bin(chrome).Headless(true).NoSandbox(true).Set("disable-gpu").Set("disable-dev-shm-usage")
	control, err := l.Launch()
	if err != nil {
		return fmt.Errorf("start Chrome: %w", err)
	}
	defer l.Cleanup()
	browser := rod.New().Context(ctx).ControlURL(control)
	if err := browser.Connect(); err != nil {
		return fmt.Errorf("connect to Chrome: %w", err)
	}
	defer func() { _ = browser.Close() }()
	page := browser.MustPage()
	defer func() {
		if r := recover(); r != nil {
			_ = rod.Try(func() { page.Timeout(10 * time.Second).MustScreenshot(filepath.Join(shots, "failure.png")) })
			err = fmt.Errorf("%v", r)
		}
	}()
	started := time.Now()
	step := func(name string) { fmt.Printf("OK %-10s %s\n", name, time.Since(started).Round(time.Second)) }

	page.MustSetViewport(1440, 900, 1, false)
	page.MustEvalOnNewDocument(trapJS)

	// Login.
	page.Timeout(2 * time.Minute).MustNavigate(base + "/auth/login").MustWaitLoad()
	credentials, err := json.Marshal(map[string]string{"password": password})
	must(err)
	if a := call(page, "POST", "/auth/login", string(credentials), map[string]string{"Content-Type": "application/json"}); a.Status != 200 {
		fail("login answered %d: %s", a.Status, a.Text)
	}
	step("login")

	// The real desktop opens the real app from the asset set.
	page.Timeout(2 * time.Minute).MustNavigate(base + "/desktop?app=storymaker").MustWaitLoad()
	wait(page, `()=>!!window.__i6inst && !!document.querySelector('.stm-projects')`, 3*time.Minute)
	step("app")

	// sanotts is the active provider, so it leads the list and is ready.
	var voices struct {
		Providers []struct {
			ID     string `json:"id"`
			Active bool   `json:"active"`
			Ready  bool   `json:"ready"`
		} `json:"providers"`
	}
	decode(get(page, storymakerAPI+"/voices"), &voices)
	if len(voices.Providers) == 0 || voices.Providers[0].ID != "sanotts" || !voices.Providers[0].Active || !voices.Providers[0].Ready {
		fail("sanotts does not lead the voice list: %+v", voices.Providers)
	}
	step("voices")

	// A new project in the desktop language gets the resolved voice.
	page.Timeout(time.Minute).MustElement(`[data-stm="new-title"]`).MustInput(projectTitle)
	click(page, `[data-stm="aspect"][data-aspect="16:9"]`)
	click(page, `[data-stm="create-project"]`)
	wait(page, `(title)=>document.querySelector('[data-stm="project-title"]')?.textContent===title`, 90*time.Second, projectTitle)
	wait(page, `()=>document.querySelector('canvas[data-scene="s1"]')?.dataset.drawn==='true'`, 90*time.Second)
	var list struct {
		Projects []struct {
			ID string `json:"id"`
		} `json:"projects"`
	}
	decode(get(page, storymakerAPI+"/projects"), &list)
	if len(list.Projects) != 1 {
		fail("projects after create = %d, want 1 on a fresh run dir", len(list.Projects))
	}
	id := list.Projects[0].ID
	project := readProject(page, id)
	if project.Story.Voice == nil || project.Story.Voice.Provider != "sanotts" || !strings.HasPrefix(strings.ToLower(project.Story.Voice.Language), "en") {
		fail("project voice = %+v, want sanotts in en", project.Story.Voice)
	}
	step("project")

	// Narration typed in the inspector and saved by the app.
	page.Timeout(time.Minute).MustElement(`[data-stm="narration"]`).MustInput(narrationText)
	wait(page, `()=>document.querySelector('[data-stm="save-state"]')?.dataset.state==='saved'`, 90*time.Second)
	savedBy := time.Now().Add(30 * time.Second)
	for {
		project = readProject(page, id)
		if len(project.Story.Scenes) == 1 && project.Story.Scenes[0].Narration == narrationText {
			break
		}
		if time.Now().After(savedBy) {
			fail("the narration did not reach the server: %+v", project.Story.Scenes)
		}
		time.Sleep(500 * time.Millisecond)
	}
	step("narration")

	// Synthesis with the real provider. The first call installs the sanoTTS runtime.
	narrate := startedJob(call(page, "POST", storymakerAPI+"/projects/"+id+"/narrate", "", nil), "POST /narrate")
	narrated, _ := waitJob(page, narrate.ID, 10*time.Minute)
	if narrated.Status != "succeeded" {
		fail("narrate job ended %s: %s", narrated.Status, narrated.failure())
	}
	var audioAnswer struct {
		Narration map[string]struct {
			Blocks []struct {
				ID       string `json:"id"`
				SpeechOK bool   `json:"speech_ok"`
				Pieces   []struct {
					Hash string `json:"hash"`
					Ext  string `json:"ext"`
				} `json:"pieces"`
			} `json:"blocks"`
		} `json:"narration"`
		Missing []json.RawMessage `json:"missing"`
		Chars   struct {
			Total   int `json:"total"`
			Missing int `json:"missing"`
		} `json:"chars"`
	}
	decode(get(page, storymakerAPI+"/projects/"+id+"/narration"), &audioAnswer)
	if len(audioAnswer.Missing) != 0 || audioAnswer.Chars.Missing != 0 {
		fail("narration still missing after the job: %+v", audioAnswer.Missing)
	}
	pieces, firstHash, firstExt := 0, "", ""
	for sceneID, scene := range audioAnswer.Narration {
		for _, block := range scene.Blocks {
			if !block.SpeechOK {
				fail("block %s of %s has no detected speech", block.ID, sceneID)
			}
			for _, piece := range block.Pieces {
				if firstHash == "" {
					firstHash, firstExt = piece.Hash, piece.Ext
				}
				pieces++
			}
		}
	}
	if pieces != 3 {
		fail("pieces = %d, want 3: one per sentence, none is a fragment", pieces)
	}
	var audio struct {
		Status int    `json:"status"`
		Type   string `json:"type"`
		Bytes  int    `json:"bytes"`
	}
	audioURL := storymakerAPI + "/projects/" + id + "/audio/narration/" + firstHash + "." + firstExt
	must(json.Unmarshal([]byte(page.Timeout(time.Minute).MustEval(audioJS, audioURL).Str()), &audio))
	if audio.Status != 200 || !strings.HasPrefix(audio.Type, "audio/") || audio.Bytes == 0 {
		fail("audio route answered %+v", audio)
	}
	wait(page, `()=>{const a=window.__i6inst.audioInfo;return !!(a&&a.narration&&Object.keys(a.narration).length>0)&&!!document.querySelector('.stm-scrub-wave');}`, 2*time.Minute)
	page.Timeout(30 * time.Second).MustScreenshot(filepath.Join(shots, "editor.png"))
	fmt.Printf("voice chars=%d pieces=%d\n", audioAnswer.Chars.Total, pieces)
	step("narrate")

	// Export with stems and subtitles through the real render route.
	project = readProject(page, id)
	render := startedJob(call(page, "POST", storymakerAPI+"/projects/"+id+"/render", `{"srt":true,"vtt":true,"stems":true,"quality":"standard"}`,
		map[string]string{"Content-Type": "application/json", "If-Match": project.ETag, "Idempotency-Key": "i6-" + id}), "POST /render")
	rendered, rawJob := waitJob(page, render.ID, 15*time.Minute)
	if rendered.Status != "succeeded" {
		fail("render job ended %s: %s", rendered.Status, rendered.failure())
	}
	out := rendered.Output
	if out == nil || !strings.HasSuffix(out.Path, ".mp4") || out.Stems == nil || out.Stems.Narration == "" || out.SRT == "" || out.VTT == "" || !strings.HasPrefix(out.Library, "Videos/Storymaker/") {
		fail("render output is incomplete: %s", rawJob)
	}
	if len(rendered.Warnings) != 0 {
		fail("render warnings: %+v", rendered.Warnings)
	}
	stems := 0
	for _, path := range []string{out.Stems.Narration, out.Stems.Music, out.Stems.SFX} {
		if path != "" {
			stems++
		}
	}
	step("export")

	// The handoff through the real client module and the real window.
	var handoff struct {
		Offered   bool            `json:"offered"`
		ProjectID string          `json:"projectId"`
		Status    json.RawMessage `json:"status"`
	}
	must(json.Unmarshal([]byte(page.Timeout(5*time.Minute).MustEval(handoffJS, rawJob).Str()), &handoff))
	if !handoff.Offered {
		fail("canOffer refused: %s", handoff.Status)
	}
	var vs struct {
		Project struct {
			Name   string            `json:"name"`
			Assets []json.RawMessage `json:"assets"`
			Tracks []struct {
				Kind  string `json:"kind"`
				Clips []struct {
					AudioDetached bool `json:"audio_detached"`
					Duration      int  `json:"duration"`
				} `json:"clips"`
			} `json:"tracks"`
		} `json:"project"`
	}
	decode(get(page, videoStudioAPI+"/projects/"+handoff.ProjectID), &vs)
	// The MP4 plus each stem that exists: one video track, one audio track per stem, one clip per track.
	if len(vs.Project.Assets) != 1+stems || len(vs.Project.Tracks) != 1+stems {
		fail("Video Studio project has %d assets and %d tracks, want %d of each", len(vs.Project.Assets), len(vs.Project.Tracks), 1+stems)
	}
	for i, track := range vs.Project.Tracks {
		wantKind := "audio"
		if i == 0 {
			wantKind = "video"
		}
		if track.Kind != wantKind || len(track.Clips) != 1 {
			fail("track %d: kind %s with %d clips", i, track.Kind, len(track.Clips))
		}
		if track.Clips[0].Duration < 1 || track.Clips[0].Duration > rendered.FramesTotal {
			fail("track %d: clip duration %d outside 1..%d", i, track.Clips[0].Duration, rendered.FramesTotal)
		}
	}
	if !vs.Project.Tracks[0].Clips[0].AudioDetached {
		fail("the video clip keeps its audio attached")
	}
	wait(page, `(name)=>document.querySelector('.vd-window[data-app-id="video-studio"] [data-project-name]')?.textContent===name`, 2*time.Minute, vs.Project.Name)
	page.Timeout(30 * time.Second).MustScreenshot(filepath.Join(shots, "video-studio.png"))
	step("handoff")

	if errors := page.MustEval(`()=>JSON.stringify(window.__i6errors)`).Str(); errors != "[]" {
		fmt.Println("PAGE ERRORS", errors)
	}
	fmt.Printf("RESULT project=%s job=%s frames=%d mp4=%s size=%d stems=%d srt=%s vtt=%s library=%s videostudio=%s\n",
		id, rendered.ID, rendered.FramesTotal, out.Path, out.Size, stems, out.SRT, out.VTT, out.Library, handoff.ProjectID)
	return nil
}
```

Build it:

```powershell
$env:GOCACHE = 'D:\dev-cache\go-build'
$env:GOTMPDIR = 'D:\tmp\go-build'
Set-Location 'D:\tmp\go-build\sm-i6\src'
go build -o 'D:\tmp\go-build\sm-i6\i6runner.exe' ./_i6
```

Expected: exit 0.

- [ ] **Step 6: Run the acceptance**

```powershell
$I6 = 'D:\tmp\go-build\sm-i6'
& "$I6/i6runner.exe" -base 'http://127.0.0.1:18733' -password (Get-Content -Raw (Join-Path $I6 'password.txt')) -shots "$I6/shots"
```

Expected, in this order: `OK` lines for `login`, `app`, `voices`, `project`, `narration`, `narrate`, `export`, `handoff`, then a `RESULT` line, then `PASS`. The first narrate call needs internet and Python 3.10 or later, because it installs the sanoTTS runtime (measured 47 s on this box, limit 5 minutes).

A `FAIL` line leaves `shots/failure.png`. Hand it to debug-agent, and fixes go to maker-agent as their own commit. Then repeat from Step 1, because the snapshot must hold the fix. A `PAGE ERRORS` line does not fail the run, but read it: a `csp:` entry for a Storymaker or Video Studio URL counts as a failure of this task.

- [ ] **Step 7: Check the files on disk**

```powershell
$I6 = 'D:\tmp\go-build\sm-i6'
$root = Join-Path $I6 'agent_workspace'
$mp4 = Get-ChildItem -Path $root -Recurse -Filter '*.mp4' | Where-Object { $_.FullName -match 'Storymaker' -and $_.FullName -match 'exports' } | Select-Object -First 1
$mp4.FullName
ffprobe -v error -count_packets -select_streams v:0 -show_entries stream=nb_read_packets -of csv=p=0 $mp4.FullName
ffprobe -v error -select_streams a:0 -show_entries stream=codec_name,sample_rate,bit_rate -of csv=p=0 $mp4.FullName
ffmpeg -hide_banner -nostats -i $mp4.FullName -af ebur128=framelog=quiet -f null - 2>&1 | Select-String -Pattern '^\s+I:\s'
Get-ChildItem -Path $root -Recurse -Include 'narration.wav','captions.srt','captions.vtt' | Where-Object { $_.FullName -match 'exports' } | Select-Object FullName, Length
Get-Content (Get-ChildItem -Path $root -Recurse -Filter 'captions.srt' | Select-Object -First 1).FullName
```

Expected:
- The packet count equals `frames=` in the `RESULT` line.
- The audio stream is `aac` at 48000 Hz.
- `I:` is a finite LUFS value, so the track is not silent. Record it. The pass rule is only that it is finite, because the level of a real one-voice film is not pinned here.
- All three files exist with a size above 0.
- The SRT shows the three narration sentences.

- [ ] **Step 8: Look at the screenshots**

Open `D:\tmp\go-build\sm-i6\shots\editor.png` and `D:\tmp\go-build\sm-i6\shots\video-studio.png` with the Read tool.

Expected: the editor shows the scene and the scrubber waveform. Video Studio shows the project with a video track and at least one audio track.

- [ ] **Step 9: Stop the instance**

Stop only your own PID, after checking its exe path.

```powershell
$I6 = 'D:\tmp\go-build\sm-i6'
$id = [int](Get-Content (Join-Path $I6 'aurago.pid'))
$proc = Get-Process -Id $id
if ($proc.Path -ne (Join-Path $I6 'aurago.exe')) { throw "PID $id is not the scratch instance: $($proc.Path)" }
Stop-Process -Id $id
```

Leave the run dir in place. I7 removes it after landing.

- [ ] **Step 10: Record the result**

Write `docs/superpowers/plans/2026-11-04-storymaker-phase2/evidence/assetpack.md` with the Write tool. Use the title `# Asset-pack acceptance, phase 2 (task I6)`, then one line each, with the values of this run:
- the date and `git rev-parse --short storymaker-p2d`
- the Chrome version and the FFmpeg version token
- the `OK` lines and the `RESULT` line, verbatim
- the packet count, the audio stream line and the `I:` value of Step 7
- the text of any `PAGE ERRORS` line, or "no page errors"
- what was not covered: the export dialog and the panels (the I2 tour covers them), music, phonon, restart, cancel

- [ ] **Step 11: Commit**

```powershell
Set-Location C:/Users/Andi/Documents/repo/AuraGo/.worktrees/storymaker-p2d
node C:/Users/Andi/.gitnexus/stores/AuraGo-08496d38b623/checkouts/AuraGo-08496d38b623/run.cjs detect-changes --scope all --repo C:/Users/Andi/Documents/repo/AuraGo # run alone, read the answer before git add
git add -f docs/superpowers/plans/2026-11-04-storymaker-phase2/evidence/assetpack.md
git commit -m "docs(storymaker): record the phase 2 asset-pack acceptance" -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

Expected from `detect-changes`: no changed symbols from this task. It may list the uncommitted `ui/js/desktop/bundles/main.bundle.js` from E3f, which stays unstaged. `partial: true` on new files is accepted with a grep confirmation in the commit body. If the tool crashes, closes the connection, answers `not found`, or prints only its banner, use `graft` callers plus grep and say so in the commit body.

### Task I7: Final matrix and landing
**Batch:** 2D  **Depends on:** I1, I2, I3, I4, I5, I6
**Files:**
- none (the last commit holds only regenerated bundles, and only if `git status` shows a bundle diff)

Run the commands in this task from `.worktrees/storymaker-p2d` unless the command names another folder. Never stage in the shared index, never autostash, and never commit, stash or restore another session's edits. Never push unless the user asks.

- [ ] **Step 1: Confirm the state of the branch**

```powershell
Set-Location C:/Users/Andi/Documents/repo/AuraGo/.worktrees/storymaker-p2d
$env:GOCACHE = 'D:\dev-cache\go-build'
$env:GOTMPDIR = 'D:\tmp\go-build'
git status --short
git log --oneline main..HEAD
```

Expected: the only tracked change is ` M ui/js/desktop/bundles/main.bundle.js`, from E3f. The log holds the 2D task commits. The controller confirms that the combined review (reader-agent) and the security review of the handoff (security-agent) are done and their fixes are committed.

- [ ] **Step 2: Rebase on main**

A rebase refuses a dirty tracked file, so the bundle goes first. It is regenerated in Step 3.

```powershell
git restore ui/js/desktop/bundles/main.bundle.js
git rebase main
```

Expected: the rebase finishes. A conflict goes to maker-agent; resolve it in the worktree and continue with `git rebase --continue`.

- [ ] **Step 3: Rebuild and commit the bundles**

```powershell
npm run build:ui
git status --short
```

`git status` lists `ui/js/desktop/bundles/main.bundle.js`, because E3f changed its parts and left the rebuilt copy uncommitted, and Step 2 restored the committed one. Run `detect_changes`, stage each listed bundle by name, and commit. These are the three paths this batch can change:

```powershell
node C:/Users/Andi/.gitnexus/stores/AuraGo-08496d38b623/checkouts/AuraGo-08496d38b623/run.cjs detect-changes --scope all --repo C:/Users/Andi/Documents/repo/AuraGo # run alone, read the answer before git add
git add ui/js/desktop/bundles/main.bundle.js ui/js/storymaker/host.bundle.js ui/js/storymaker/player.bundle.js
git commit -m "build(ui): rebuild bundles for Storymaker phase 2" -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

Stage a path only if `git status` lists it. When Step 6 sends you back here after main moved, a rebuild may show no bundle diff, and then there is no commit. After this step `git status --short` prints nothing.

- [ ] **Step 4: Run the 2D matrix**

Run through test-agent, with the shell sandbox off for the browser lines.

```powershell
npm run build:ui
npm run check:ui
npm run test:storymaker
node scripts/build-storymaker-ranges.mjs --check
npm run check:storymaker-catalog
node --test assets/storymaker/test/glyphs.test.mjs
node --test assets/storymaker/test/catalog.test.mjs
node --test assets/storymaker/test/sfx.test.mjs
go test ./internal/storymaker/... ./internal/config ./internal/desktop ./internal/security ./internal/videostudio ./internal/audit ./cmd/config-merger ./internal/setup -count=1
go test ./internal/tools -run '(?i)(elevenlabs|openaitts)' -count=1
go test ./internal/server -run Storymaker -count=1 -timeout 20m
go test ./ui -run '(?i)(storymaker|videostudio)' -count=1 -timeout 20m
go test ./ui -run '^TestShippedWebAssetsLoadNothingExternal$' -count=1
gofmt -l internal/storymaker internal/server internal/config internal/desktop internal/tools internal/videostudio
go vet ./internal/storymaker/... ./internal/server ./internal/desktop
$env:GOOS='linux'; go build ./...; Remove-Item Env:GOOS
$env:AURAGO_RUN_BROWSER_SMOKE='1'
go test ./internal/server -run '^TestStorymakerAppRealServerBrowser$' -count=1 -timeout 15m
go test ./internal/server -run '^TestStorymakerParityAppPlayerAndRenderPage$' -count=1 -timeout 15m
go test ./internal/storymaker/render -run '^TestParallelParityBrowser$' -count=1 -timeout 20m
go test ./internal/storymaker/render -run '^TestParallelEncoderFailureBrowser$' -count=1 -timeout 15m
Remove-Item Env:AURAGO_RUN_BROWSER_SMOKE
```

Drop the two `TestParallel` lines if W was dropped.

Expected:
- Every line exits 0, and `gofmt -l` prints nothing.
- A browser line that prints `[no tests to run]` counts as failed. A line that skips counts as failed too, so rerun it with `-v` when in doubt.
- The failures known on main, recorded 2026-11-05, are `internal/audit` `TestUpdateConfigFallbackOnlyTrustedTransactionBackups` and `TestCredentialedHTTPClientsBindRedirectsToOrigin`.

- [ ] **Step 5: Confirm any failure on a clean main**

Do this for every failing test before you call it pre-existing. Use a detached worktree of your own, and put the failing package and test name into the `go test` line.

```powershell
Set-Location C:/Users/Andi/Documents/repo/AuraGo
git worktree add --detach .worktrees/sm-clean-main main
Set-Location .worktrees/sm-clean-main
go test <the failing package> -run '^<the failing test>$' -count=1
Set-Location C:/Users/Andi/Documents/repo/AuraGo
git worktree remove .worktrees/sm-clean-main
Set-Location .worktrees/storymaker-p2d
```

A test that fails the same way on clean main goes into the batch report. Any other failure goes to debug-agent, and the fix to maker-agent as its own commit. Then rerun the lines that cover it.

- [ ] **Step 6: Run the landing checks**

Run these from the worktree.

```powershell
$main = 'C:/Users/Andi/Documents/repo/AuraGo'
$branch = 'storymaker-p2d'
git -C $main branch --show-current
git merge-base --is-ancestor main $branch; $LASTEXITCODE
$dirty = git -C $main diff HEAD --name-only
$ours = git diff "main...$branch" --name-only
$overlap = $dirty | Where-Object { $ours -contains $_ }
$overlap
git -C $main ls-files -v AGENTS.md
Test-Path "$main/.git/index.lock"
```

Expected:
- The branch name is `main`.
- `$LASTEXITCODE` is `0`.
- `$overlap` prints nothing.
- `ls-files` shows an uppercase `H`.
- `Test-Path` answers `False`.

If main moved (exit code 1): rebase and rebuild as in Steps 2 and 3. Then rerun the whole matrix when main's new commits overlap the batch's files, otherwise only the lines that cover the packages main changed.

If `$overlap` lists `ui/js/desktop/core/module-loader.js`, `ui/js/desktop/bundles/main.bundle.js`, `ui/js/desktop/core/menus-and-routing.js`, `ui/js/desktop/core/window-shell-runtime.js` or `AGENTS.md`, or `ls-files` shows a lowercase `h` (an assume-unchanged flag hides its edits), then `merge --ff-only` refuses. Do this instead:
1. Wait for the other session. Poll main's SHA and the overlap count with the commands above.
2. Rebase after each move of main and rebuild.
3. If the wait runs long, ask the user. Hand landings around another session's edits need the user's OK.

An `index.lock` that stays is removed only with the user's OK.

- [ ] **Step 7: Land**

Run this as its own step, never chained after the checks with `;`.

```powershell
git -C C:/Users/Andi/Documents/repo/AuraGo merge --ff-only storymaker-p2d
```

Expected: a fast-forward. If it refuses, go back to Step 6.

- [ ] **Step 8: Clean up**

Remove the `node_modules` junction as a link only, then the worktree and the branch. Then remove the scratch run dir of I6.

```powershell
Set-Location C:/Users/Andi/Documents/repo/AuraGo
$p = (Resolve-Path .worktrees/storymaker-p2d).Path + '\node_modules'
if ((Test-Path -LiteralPath $p) -and (Get-Item -LiteralPath $p -Force).LinkType -eq 'Junction') { [System.IO.Directory]::Delete($p, $false) }
git worktree remove .worktrees/storymaker-p2d
git branch -d storymaker-p2d
Remove-Item -LiteralPath 'D:\tmp\go-build\sm-i6' -Recurse -Force
```

Expected: `git worktree list` no longer names `storymaker-p2d`.

- [ ] **Step 9: Report**

There is no commit in this step. The batch report names:
- the landed SHA
- each failure confirmed on clean main
- what stays open for the user: aurago-test acceptance (B13), listening notes and rhythm factors, the taste of the SFX set, prosody with per-sentence synthesis, and agent voice discovery (`storymaker_catalog kind=voice`, phase 3)

{% endraw %}
