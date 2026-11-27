package server

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/go-rod/rod"
	"github.com/google/uuid"

	"aurago/internal/chibisensei"
)

// Two gated "real runtime" tests of the Chibi Sensei Test step: the real routes, the real llama-server that the app
// downloads itself and a real chat GGUF (the second one also drives the real UI in Chrome). Without their environment
// variables both are skipped, so ordinary runs never download or start anything.
//
//	AURAGO_RUN_LLAMA_SMOKE=1 CHIBI_SMOKE_GGUF=<chat gguf> [CHIBI_SMOKE_CACHE=<installed runtime cache>] \
//	  [CHIBI_SMOKE_OUT=<record.txt>] go test ./internal/server -run '^TestChibiSenseiRealRuntimeChat$' -count=1 -v -timeout 40m
//
//	... AURAGO_RUN_BROWSER_SMOKE=1 [CHIBI_SMOKE_SHOT=<shot.png>] -run '^TestChibiSenseiAppRealRuntimeBrowser$'

const (
	chibiRealPrompt1  = "Say hello in one short sentence."
	chibiRealPrompt2  = "And now say goodbye in one short sentence."
	chibiRealSystem   = "You are a concise assistant."
	chibiRealMaxToken = 24
)

type chibiRealFix struct {
	s       *Server
	admin   string
	rt      *chibiSenseiRuntime
	project string
	run     string
	info    chibisensei.GGUFInfo
	prof    *chibisensei.Profile
	gguf    string
	size    int64
	preInst bool
	rec     strings.Builder
}

func (f *chibiRealFix) note(format string, a ...any) { fmt.Fprintf(&f.rec, format+"\n", a...) }

// chibiRealGate skips unless the environment asks for the real run and returns the GGUF path.
func chibiRealGate(t *testing.T) string {
	t.Helper()
	if os.Getenv("AURAGO_RUN_LLAMA_SMOKE") != "1" {
		t.Skip("set AURAGO_RUN_LLAMA_SMOKE=1 to run the real Chibi Sensei runtime test")
	}
	gguf := os.Getenv("CHIBI_SMOKE_GGUF")
	if gguf == "" {
		t.Skip("set CHIBI_SMOKE_GGUF to a chat GGUF")
	}
	if fi, err := os.Stat(gguf); err != nil || !fi.Mode().IsRegular() {
		t.Fatalf("CHIBI_SMOKE_GGUF is not a regular file: %v", err)
	}
	return gguf
}

// chibiRealLink makes dst a hard link of src and falls back to a streamed copy.
func chibiRealLink(t *testing.T, src, dst string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(dst), 0o700); err != nil {
		t.Fatal(err)
	}
	if os.Link(src, dst) == nil {
		return
	}
	in, err := os.Open(src)
	if err != nil {
		t.Fatal(err)
	}
	defer in.Close()
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		t.Fatal(err)
	}
	if err := out.Close(); err != nil {
		t.Fatal(err)
	}
}

// chibiRealCopyTree copies the directory src into dst (used for CHIBI_SMOKE_CACHE).
func chibiRealCopyTree(t *testing.T, src, dst string) {
	t.Helper()
	err := filepath.Walk(src, func(p string, fi os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(src, p)
		target := filepath.Join(dst, rel)
		if fi.IsDir() {
			return os.MkdirAll(target, 0o700)
		}
		if !fi.Mode().IsRegular() {
			return nil
		}
		raw, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		return os.WriteFile(target, raw, 0o600)
	})
	if err != nil {
		t.Fatalf("copy CHIBI_SMOKE_CACHE: %v", err)
	}
}

// chibiRealFixture builds the server (real launcher, cpu, context 2048), a project and a succeeded run that carries the
// real model as tuned and base GGUF.
func chibiRealSetup(t *testing.T, gguf string) *chibiRealFix {
	t.Helper()
	info, err := chibisensei.ReadGGUFInfo(gguf)
	if err != nil {
		t.Fatalf("read the GGUF header: %v", err)
	}
	cat, err := chibisensei.LoadCatalog()
	if err != nil {
		t.Fatal(err)
	}
	var prof *chibisensei.Profile
	for _, p := range cat.List() {
		if p.GGUF.ExpectedArch == info.Architecture {
			prof = p
			break
		}
	}
	if prof == nil {
		t.Skipf("no catalog profile expects the architecture %q of %s", info.Architecture, filepath.Base(gguf))
	}
	fi, _ := os.Stat(gguf)

	s, _, _ := chibiAPIFixture(t)
	chibiUseStubKaggle(t, s) // no Kaggle; the stub also reports plenty of free disk
	s.Cfg.ChibiSensei.Runtime.Enabled = true
	s.Cfg.ChibiSensei.Runtime.Backend = "cpu"
	s.Cfg.ChibiSensei.Runtime.ContextSize = 2048
	s.Cfg.Auth.Enabled = false
	t.Cleanup(s.closeChibiSensei) // no llama-server child may survive the test
	admin := chibiAdminToken(t, s)
	pid := chibiNewProject(t, s, admin, map[string]any{"name": "Real runtime project", "recipe": "assistant", "profile_id": prof.ID})
	rt, release, err := s.acquireChibiSensei()
	if err != nil {
		t.Fatal(err)
	}
	defer release()
	f := &chibiRealFix{s: s, admin: admin, rt: rt, project: pid, info: info, prof: prof, gguf: gguf, size: fi.Size()}
	if cache := os.Getenv("CHIBI_SMOKE_CACHE"); cache != "" {
		chibiRealCopyTree(t, cache, filepath.Join(chibisensei.ChibiDir(rt.dataDir), "runtime"))
		f.preInst = true
	}
	f.run = f.seedRun(t)
	return f
}

func (f *chibiRealFix) seedRun(t *testing.T) string {
	t.Helper()
	ctx := context.Background()
	st, dataDir := f.rt.store, f.rt.dataDir
	id := uuid.NewString()
	if err := st.CreateRun(ctx, chibisensei.Run{ID: id, ProjectID: f.project, ProfileID: f.prof.ID, Preset: "quick"}); err != nil {
		t.Fatal(err)
	}
	job := `{"recipe":"assistant","task_kind":null,"labels":[],"profile":{"base":{"revision":"` + chibiTestRevision +
		`"}},"deps":{"llama_cpp":{"tag":"b9994"}}}`
	if _, err := st.UpdateRun(ctx, id, chibisensei.NewStateRunEvent(chibisensei.RunSucceeded), func(r *chibisensei.Run) error {
		r.State, r.Remote.LastState, r.Remote.Finished = chibisensei.RunSucceeded, chibisensei.RunCleaning, "2026-11-23T10:00:00Z"
		r.JobJSON = []byte(job)
		r.Remote.GGUF = &f.info
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	tunedRel := "models/" + id + "/model-q8_0.gguf"
	baseRel := "base/" + f.prof.ID + "-" + chibiTestRevision[:12] + "-llamacpp-b9994.gguf"
	tunedAbs := filepath.Join(chibisensei.ChibiDir(dataDir), filepath.FromSlash(tunedRel))
	chibiRealLink(t, f.gguf, tunedAbs)
	chibiRealLink(t, tunedAbs, filepath.Join(chibisensei.ChibiDir(dataDir), filepath.FromSlash(baseRel)))
	for _, a := range []chibisensei.Artifact{
		{RunID: id, ProjectID: f.project, Kind: "gguf_tuned", RelPath: tunedRel, Bytes: f.size, SHA256: strings.Repeat("a", 64)},
		{RunID: id, ProjectID: f.project, Kind: "gguf_base", RelPath: baseRel, Bytes: f.size, SHA256: strings.Repeat("b", 64)},
	} {
		if err := st.AddArtifact(ctx, a); err != nil {
			t.Fatal(err)
		}
	}
	var buf bytes.Buffer
	for i := 0; i < 3; i++ {
		line, _ := json.Marshal(map[string]any{"id": fmt.Sprint(i),
			"prompt_messages": []map[string]string{{"role": "user", "content": fmt.Sprintf("question %d", i+1)}},
			"expected":        "expected answer", "base_output": "same answer", "tuned_output": "same answer", "base_ms": 10, "tuned_ms": 12, "truncated": false})
		buf.Write(line)
		buf.WriteByte('\n')
	}
	chibiPutDataFile(t, dataDir, "runs/"+id+"/out/eval_samples.jsonl", buf.Bytes())
	if err := st.SaveReport(ctx, id, "auto", chibisensei.Report{RunID: id, Kind: "auto", Samples: 3, SmallSample: true}); err != nil {
		t.Fatal(err)
	}
	return id
}

// chibiRealRecord writes CHIBI_SMOKE_OUT (also after a failure) with the header lines first.
func (f *chibiRealFix) writeRecord(t *testing.T, name string) {
	path := os.Getenv("CHIBI_SMOKE_OUT")
	if path == "" {
		return
	}
	head := fmt.Sprintf("%s\nmodel file: %s (%d bytes)\narchitecture: %s\nprofile used: %s\nruntime cache pre-copied: %v\n%s\n",
		name, filepath.Base(f.gguf), f.size, f.info.Architecture, f.prof.ID, f.preInst, time.Now().Format(time.RFC3339))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err == nil {
		err = os.WriteFile(path, []byte(head+f.rec.String()), 0o644)
		if err != nil {
			t.Errorf("write CHIBI_SMOKE_OUT: %v", err)
		}
	}
}

type chibiTimedEvent struct {
	chibiSSE
	at time.Duration
}

// chibiReadStream reads an event stream as it arrives and stamps every event with the time since start.
func chibiReadStream(t *testing.T, body io.Reader, start time.Time) []chibiTimedEvent {
	t.Helper()
	var out []chibiTimedEvent
	rd := bufio.NewReader(body)
	var block strings.Builder
	for {
		line, err := rd.ReadString('\n')
		block.WriteString(line)
		if line == "\n" || (err != nil && block.Len() > 0) {
			text := block.String()
			block.Reset()
			if strings.TrimSpace(text) != "" {
				evs, _ := chibiParseSSE(t, text)
				for _, e := range evs {
					out = append(out, chibiTimedEvent{e, time.Since(start)})
				}
			}
		}
		if err != nil {
			return out
		}
	}
}

// chibiRealChat posts one chat request through a real HTTP server and returns the timed events.
func (f *chibiRealFix) chat(t *testing.T, base string, req map[string]any) []chibiTimedEvent {
	t.Helper()
	raw, _ := json.Marshal(req)
	hr, _ := http.NewRequest(http.MethodPost, base+chibiSenseiAPIBase+"/runtime/chat", bytes.NewReader(raw))
	hr.Header.Set("Authorization", "Bearer "+f.admin)
	hr.Header.Set("Content-Type", "application/json")
	start := time.Now()
	res, err := (&http.Client{Timeout: 12 * time.Minute}).Do(hr)
	if err != nil {
		t.Fatalf("POST /runtime/chat: %v", err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(res.Body)
		t.Fatalf("POST /runtime/chat = %d %s", res.StatusCode, b)
	}
	if ct := res.Header.Get("Content-Type"); !strings.HasPrefix(ct, "text/event-stream") {
		t.Fatalf("content type = %q", ct)
	}
	return chibiReadStream(t, res.Body, start)
}

func chibiRealSampling() map[string]any {
	return map[string]any{"temperature": 0, "max_tokens": chibiRealMaxToken, "seed": 1}
}

// chibiRealCheckTarget asserts the contract of one target's events and returns its answer and done event.
func chibiRealCheckTarget(t *testing.T, events []chibiTimedEvent, target string) (string, map[string]any, time.Duration, time.Duration) {
	t.Helper()
	var answer strings.Builder
	var done map[string]any
	var loading, generating, doneN, errN, deltas int
	var tGen, tDone time.Duration
	for _, e := range events {
		if e.data["target"] != target {
			continue
		}
		switch e.name {
		case "status":
			switch e.data["state"] {
			case "loading":
				loading++
			case "generating":
				generating++
				tGen = e.at
			}
		case "delta":
			deltas++
			answer.WriteString(e.data["text"].(string))
		case "done":
			doneN++
			done = e.data
			tDone = e.at
		case "error":
			errN++
			t.Errorf("%s: error event %v", target, e.data)
		}
	}
	if loading != 1 || generating != 1 || deltas < 1 || doneN != 1 || errN != 0 {
		t.Fatalf("%s: loading=%d generating=%d deltas=%d done=%d errors=%d", target, loading, generating, deltas, doneN, errN)
	}
	if ct, _ := done["completion_tokens"].(float64); ct <= 0 || done["backend"] != "cpu" {
		t.Fatalf("%s: done = %v", target, done)
	}
	text := answer.String()
	if strings.TrimSpace(text) == "" || strings.Contains(text, "<think>") || strings.Contains(text, "</think>") {
		t.Fatalf("%s: answer %q is empty or carries think tags", target, text)
	}
	return text, done, tGen, tDone
}

func chibiRealMux(f *chibiRealFix) *httptest.Server {
	mux := http.NewServeMux()
	mux.Handle(chibiSenseiAPIBase+"/", handleChibiSensei(f.s))
	return httptest.NewServer(mux)
}

func (f *chibiRealFix) installRuntime(t *testing.T) {
	t.Helper()
	rt := chibiRuntimeState(t, f.s, f.admin)
	dl, _ := rt["download"].(map[string]any)
	f.note("runtime state before install: %v (download %v bytes from %v)", rt["state"], dl["bytes"], dl["host"])
	if f.preInst {
		if rt["state"] != "ready" {
			t.Fatalf("a pre-copied cache is not recognised: state = %v", rt["state"])
		}
		return
	}
	if rt["state"] != "not_installed" {
		t.Fatalf("state before install = %v, want not_installed", rt["state"])
	}
	if b, _ := dl["bytes"].(float64); b <= 0 {
		t.Fatalf("download.bytes = %v", dl["bytes"])
	}
	chibiWant(t, chibiDo(t, f.s, f.admin, http.MethodPost, "/runtime/install", nil), http.StatusAccepted, "")
	start, last := time.Now(), ""
	for {
		rt = chibiRuntimeState(t, f.s, f.admin)
		st, _ := rt["state"].(string)
		if st != last {
			f.note("runtime state after %s: %s", time.Since(start).Round(time.Second), st)
			last = st
		}
		if st == "ready" {
			return
		}
		if time.Since(start) > 15*time.Minute {
			t.Fatalf("runtime did not become ready: state %v, download %v", st, rt["download"])
		}
		time.Sleep(time.Second)
	}
}

func TestChibiSenseiRealRuntimeChat(t *testing.T) {
	f := chibiRealSetup(t, chibiRealGate(t))
	defer f.writeRecord(t, "API-level real runtime chat test")
	t.Logf("model %s (%d bytes), architecture %s, profile %s", filepath.Base(f.gguf), f.size, f.info.Architecture, f.prof.ID)

	// 1 and 2: the runtime is installed through the real route (or recognised from the pre-copied cache).
	f.installRuntime(t)

	// 3: both models are available.
	w := chibiDo(t, f.s, f.admin, http.MethodGet, "/runs/"+f.run+"/test", nil)
	chibiWant(t, w, http.StatusOK, "")
	models := chibiBody(t, w)["models"].(map[string]any)
	for _, kind := range []string{"tuned", "base"} {
		if m := models[kind].(map[string]any); m["available"] != true || m["bytes"] != float64(f.size) {
			t.Fatalf("%s model = %v", kind, m)
		}
	}

	// 4: both models answer through a real streaming HTTP server.
	srv := chibiRealMux(f)
	defer srv.Close()
	events := f.chat(t, srv.URL, chibiChatReq(f.run, chibiTarget("tuned", chibiRealSystem, chibiRealPrompt1),
		chibiTarget("base", chibiRealSystem, chibiRealPrompt1)))
	if last := events[len(events)-1]; last.name != "end" {
		t.Fatalf("last event = %s, want end", last.name)
	}
	f.note("prompt 1 (system %q): %q", chibiRealSystem, chibiRealPrompt1)
	var tunedAnswer string
	for _, kind := range []string{"tuned", "base"} {
		text, done, tGen, tDone := chibiRealCheckTarget(t, events, kind)
		if kind == "tuned" {
			tunedAnswer = text
		}
		t.Logf("%s answer: %q  (%v)", kind, text, done)
		t.Logf("%s: first generating status after %s, done after %s", kind, tGen.Round(time.Millisecond), tDone.Round(time.Millisecond))
		f.note("%s answer: %q\n  prompt_tokens=%v completion_tokens=%v tok_s=%v ms=%v backend=%v finish=%v\n  load time until generating: %s, done after: %s",
			kind, text, done["prompt_tokens"], done["completion_tokens"], done["tok_s"], done["ms"], done["backend"], done["finish_reason"],
			tGen.Round(time.Millisecond), tDone.Round(time.Millisecond))
	}

	// 5: two instances; a second turn on the warm tuned instance.
	rt := chibiRuntimeState(t, f.s, f.admin)
	kinds := map[string]bool{}
	insts, _ := rt["instances"].([]any)
	for _, in := range insts {
		kinds[in.(map[string]any)["kind"].(string)] = true
	}
	if len(insts) != 2 || !kinds["tuned"] || !kinds["base"] {
		t.Fatalf("instances = %v, want a tuned and a base one", rt["instances"])
	}
	turn2 := map[string]any{"kind": "tuned", "messages": []map[string]string{
		{"role": "system", "content": chibiRealSystem}, {"role": "user", "content": chibiRealPrompt1},
		{"role": "assistant", "content": tunedAnswer}, {"role": "user", "content": chibiRealPrompt2}}}
	events = f.chat(t, srv.URL, chibiChatReq(f.run, turn2))
	if events[len(events)-1].name != "end" {
		t.Fatalf("turn 2: last event = %s", events[len(events)-1].name)
	}
	text2, done2, tGen2, tDone2 := chibiRealCheckTarget(t, events, "tuned")
	t.Logf("tuned turn 2 answer: %q (%v), generating after %s", text2, done2, tGen2.Round(time.Millisecond))
	f.note("prompt 2: %q\ntuned answer 2 (warm): %q\n  completion_tokens=%v tok_s=%v ms=%v; generating after %s, done after %s",
		chibiRealPrompt2, text2, done2["completion_tokens"], done2["tok_s"], done2["ms"], tGen2.Round(time.Millisecond), tDone2.Round(time.Millisecond))

	// 6: a conversation that exceeds the context ends the target with context_exceeded.
	long := strings.Repeat("the quick brown fox jumps over the lazy dog ", 340)
	events = f.chat(t, srv.URL, chibiChatReq(f.run, chibiTarget("tuned", long)))
	var sawError, sawDone bool
	for _, e := range events {
		sawDone = sawDone || e.name == "done"
		if e.name == "error" {
			sawError = true
			if e.data["code"] != "context_exceeded" {
				t.Fatalf("error = %v, want context_exceeded", e.data)
			}
		}
	}
	if !sawError || sawDone || events[len(events)-1].name != "end" {
		t.Fatalf("over-long conversation: error=%v done=%v events=%v", sawError, sawDone, events)
	}
	f.note("over-long conversation (%d chars): stream error context_exceeded, then end", len(long))

	// 7: stop.
	chibiWant(t, chibiDo(t, f.s, f.admin, http.MethodPost, "/runtime/stop", nil), http.StatusOK, "")
	if n := len(chibiRuntimeState(t, f.s, f.admin)["instances"].([]any)); n != 0 {
		t.Fatalf("%d instances after stop", n)
	}
	f.note("stop: no instance left; runtime state %v", chibiRuntimeState(t, f.s, f.admin)["state"])
}

// csnbWaitLong is csnbWait with its own budget (installs and answers of a real model take minutes).
func csnbWaitLong(t *testing.T, page *rod.Page, step, expr string, d time.Duration) {
	t.Helper()
	if err := rod.Try(func() { page.Timeout(d).MustWait(expr) }); err != nil {
		t.Fatalf("step %q did not happen within %s: %v\nwaited for: %s\nstate: %s", step, d, err, expr, csnbDump(page))
	}
}

func TestChibiSenseiAppRealRuntimeBrowser(t *testing.T) {
	gguf := chibiRealGate(t)
	if os.Getenv("AURAGO_RUN_BROWSER_SMOKE") != "1" {
		t.Skip("set AURAGO_RUN_BROWSER_SMOKE=1 to run the real runtime browser test")
	}
	f := chibiRealSetup(t, gguf)
	defer f.writeRecord(t, "browser-level real runtime chat test")
	page, origin := csnbPage(t, f.s) // administrator: no bearer token and login off

	page.MustEval(`()=>auditTest.openApp('chibi-sensei')`)
	card := `[data-csn="project-card"][data-id="` + f.project + `"]`
	csnbWait(t, page, "the seeded project is listed", `()=>!!document.querySelector(`+csnbJS(card)+`)`)
	csnbClick(t, page, "the project opens", card)
	testStep := `[data-csn="step"][data-step="test"][data-state="ready"]`
	csnbWait(t, page, "the Test step is ready", `()=>!!document.querySelector(`+csnbJS(testStep)+`)`)
	csnbClick(t, page, "the Test step opens", testStep)
	csnbClick(t, page, "the chat view opens", `[data-csn="test-view"][data-view="chat"]`)
	csnbWait(t, page, "the runtime strip settled", `()=>{const e=document.querySelector('[data-csn="runtime-strip"]');return !!e&&(e.dataset.state==='ready'||e.dataset.state==='not_installed');}`)

	if page.MustEval(`()=>document.querySelector('[data-csn="runtime-strip"]').dataset.state`).String() != "ready" {
		f.note("runtime was not installed: installing through the UI")
		csnbWait(t, page, "the install card shows", `()=>!!document.querySelector('[data-csn="runtime-card"]')`)
		csnbClick(t, page, "the install button is clicked", `[data-csn="runtime-install"]`)
		csnbWait(t, page, "the confirmation dialog opens", `()=>!!document.querySelector('.vd-modal [type="submit"]')`)
		page.MustEval(`()=>document.querySelector('.vd-modal [type="submit"]').click()`)
		csnbWaitLong(t, page, "the runtime strip says ready", `()=>!!document.querySelector('[data-csn="runtime-strip"][data-state="ready"]')`, 15*time.Minute)
	}

	csnbWait(t, page, "the compare switch is enabled", `()=>{const e=document.querySelector('[data-csn="chat-compare"]');return !!e&&!e.disabled;}`)
	csnbClick(t, page, "the compare switch is turned on", `[data-csn="chat-compare"]`)
	csnbWait(t, page, "both columns show", `()=>!!document.querySelector('[data-csn="chat-column"][data-target="tuned"]')&&!!document.querySelector('[data-csn="chat-column"][data-target="base"]')`)
	// Keep the answers short: the real model runs on a few CPU threads.
	page.MustEval(`(n)=>{const e=document.querySelector('[data-csn="chat-max-tokens"]');e.value=String(n);e.dispatchEvent(new Event('input',{bubbles:true}));}`, chibiRealMaxToken)
	csnbType(t, page, "the chat message is typed", `[data-csn="chat-input"]`, chibiRealPrompt1)
	start := time.Now()
	csnbClick(t, page, "the send button sends the message", `[data-csn="chat-send"]`)

	var answers [2]string
	for i, kind := range []string{"tuned", "base"} {
		sel := `[data-csn="chat-answer"][data-target="` + kind + `"][data-state="done"]`
		csnbWaitLong(t, page, "the "+kind+" answer is done", `()=>!!document.querySelector(`+csnbJS(sel)+`)`, 10*time.Minute)
		answers[i] = page.MustEval(`(s)=>document.querySelector(s).textContent`, sel).String()
		if strings.TrimSpace(answers[i]) == "" || strings.Contains(answers[i], "<think>") {
			t.Fatalf("%s answer = %q", kind, answers[i])
		}
		t.Logf("%s answer in the UI after %s: %q", kind, time.Since(start).Round(time.Second), answers[i])
	}
	columns := page.MustEval(`()=>['tuned','base'].map(k=>document.querySelector('[data-csn="chat-column"][data-target="'+k+'"]').innerText.replace(/\s+/g,' ')).join(' | ')`).String()
	f.note("prompt: %q\ntuned answer (UI): %q\nbase answer (UI): %q\nboth done after %s\ncolumns as shown:\n%s",
		chibiRealPrompt1, answers[0], answers[1], time.Since(start).Round(time.Second), columns)

	if got := page.MustEval(`()=>JSON.stringify(window.fixtureErrors)`).String(); got != "[]" {
		t.Fatalf("script errors in the page: %s\nstate: %s", got, csnbDump(page))
	}
	if got := page.MustEval(`()=>JSON.stringify(window.failedChibi)`).String(); got != "[]" {
		t.Fatalf("failed requests to /api/desktop/chibi-sensei/: %s\nstate: %s", got, csnbDump(page))
	}
	if shot := os.Getenv("CHIBI_SMOKE_SHOT"); shot != "" {
		if err := os.MkdirAll(filepath.Dir(shot), 0o755); err != nil {
			t.Fatal(err)
		}
		page.MustScreenshotFullPage(shot)
	}

	// Stop the models through the UI; the strip or the API shows no running model afterwards.
	csnbClick(t, page, "Stop models is clicked", `[data-csn="runtime-stop"]`)
	deadline := time.Now().Add(60 * time.Second)
	for !strings.Contains(csnbGet(t, origin+chibiSenseiAPIBase+"/runtime"), `"instances":[]`) {
		if time.Now().After(deadline) {
			t.Fatalf("models still running after Stop: %s", csnbGet(t, origin+chibiSenseiAPIBase+"/runtime"))
		}
		time.Sleep(500 * time.Millisecond)
	}
	f.note("stop through the UI: no instance left")
}
