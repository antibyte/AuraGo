package server

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	"aurago/internal/chibisensei"
	"aurago/internal/chibisensei/chatrt"
	"aurago/internal/chibisensei/chatrt/chatrttest"
)

type chibiSSE struct {
	name string
	data map[string]any
}

// chibiParseSSE splits an event stream body into its events and checks the framing: every block is an event line,
// a data line and an empty line, or a comment.
func chibiParseSSE(t *testing.T, body string) (events []chibiSSE, comments int) {
	t.Helper()
	for _, block := range strings.Split(strings.TrimSuffix(body, "\n\n"), "\n\n") {
		if strings.HasPrefix(block, ":") {
			comments++
			continue
		}
		lines := strings.Split(block, "\n")
		if len(lines) != 2 || !strings.HasPrefix(lines[0], "event: ") || !strings.HasPrefix(lines[1], "data: ") {
			t.Fatalf("bad event block %q in\n%s", block, body)
		}
		var data map[string]any
		if err := json.Unmarshal([]byte(strings.TrimPrefix(lines[1], "data: ")), &data); err != nil {
			t.Fatalf("bad data in %q: %v", block, err)
		}
		events = append(events, chibiSSE{strings.TrimPrefix(lines[0], "event: "), data})
	}
	return events, comments
}

// chibiTargetEvents returns the events of one target in order.
func chibiTargetEvents(events []chibiSSE, target string) []chibiSSE {
	var out []chibiSSE
	for _, e := range events {
		if e.data["target"] == target {
			out = append(out, e)
		}
	}
	return out
}

// chibiTarget builds a target. An even number of messages starts with a system message; the rest alternates
// between user and assistant, starting and ending with user.
func chibiTarget(kind string, msgs ...string) map[string]any {
	list := []map[string]string{}
	start := 0
	if len(msgs)%2 == 0 {
		list = append(list, map[string]string{"role": "system", "content": msgs[0]})
		start = 1
	}
	for i := start; i < len(msgs); i++ {
		role := "user"
		if (i-start)%2 == 1 {
			role = "assistant"
		}
		list = append(list, map[string]string{"role": role, "content": msgs[i]})
	}
	return map[string]any{"kind": kind, "messages": list}
}

// chibiNewFailedRun creates a finished run that did not succeed.
func chibiNewFailedRun(t *testing.T, rt *chibiSenseiRuntime, pid string) string {
	t.Helper()
	id := uuid.NewString()
	ctx := context.Background()
	if err := rt.store.CreateRun(ctx, chibisensei.Run{ID: id, ProjectID: pid, ProfileID: "qwen35-0p8b", Preset: "quick"}); err != nil {
		t.Fatal(err)
	}
	if _, err := rt.store.UpdateRun(ctx, id, chibisensei.NewStateRunEvent(chibisensei.RunFailed), func(r *chibisensei.Run) error {
		r.State = chibisensei.RunFailed
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	return id
}

func chibiChatReq(runID string, targets ...map[string]any) map[string]any {
	return map[string]any{"run_id": runID, "targets": targets}
}

// chibiHoldTwo starts two chat streams on tuned models of the given runs that wait on fake.Hold, and returns once
// both instances are busy. The returned function releases them and waits for the answers (idempotent).
func chibiHoldTwo(t *testing.T, s *Server, token string, fake *chatrttest.Launcher, rt *chibiSenseiRuntime, runA, runB string) func() {
	t.Helper()
	fake.Hold = make(chan struct{})
	var wg sync.WaitGroup
	for _, id := range []string{runA, runB} {
		wg.Add(1)
		go func(id string) {
			defer wg.Done()
			chibiDo(t, s, token, http.MethodPost, "/runtime/chat", chibiChatReq(id, chibiTarget("tuned", "Hi")))
		}(id)
	}
	var once sync.Once
	stop := func() {
		once.Do(func() { close(fake.Hold) })
		wg.Wait()
	}
	t.Cleanup(stop)
	deadline := time.Now().Add(10 * time.Second)
	for {
		busy := 0
		for _, in := range rt.chat.Status(context.Background()).Instances {
			if in.State == "busy" {
				busy++
			}
		}
		if busy == 2 {
			return stop
		}
		if time.Now().After(deadline) {
			t.Fatal("the two held instances never became busy")
		}
		time.Sleep(5 * time.Millisecond)
	}
}

func TestChibiSenseiChatValidation(t *testing.T) {
	s, token, _, rt, pid := chibiTestFixture(t, true)
	run := chibiSucceededTestRun(t, rt, pid, "qwen35-0p8b", true, 0)
	valid := func() map[string]any {
		return chibiChatReq(run, chibiTarget("tuned", "Hi"), chibiTarget("base", "Hi"))
	}
	long := func(n int, r string) string { return strings.Repeat(r, n) }
	manyTurns := func(users int) []map[string]string {
		var msgs []map[string]string
		for i := 0; i < users; i++ {
			if i > 0 {
				msgs = append(msgs, map[string]string{"role": "assistant", "content": "a"})
			}
			msgs = append(msgs, map[string]string{"role": "user", "content": "u"})
		}
		return msgs
	}
	msgs := func(r map[string]any, i int, m ...map[string]string) {
		r["targets"].([]map[string]any)[i]["messages"] = m
	}
	u := func(c string) map[string]string { return map[string]string{"role": "user", "content": c} }
	a := func(c string) map[string]string { return map[string]string{"role": "assistant", "content": c} }
	sys := func(c string) map[string]string { return map[string]string{"role": "system", "content": c} }
	sampling := func(r map[string]any, k string, v any) { r["sampling"] = map[string]any{k: v} }

	for _, tc := range []struct {
		name string
		mut  func(r map[string]any)
	}{
		{"bad run id", func(r map[string]any) { r["run_id"] = "nope" }},
		{"no targets", func(r map[string]any) { r["targets"] = []map[string]any{} }},
		{"three targets", func(r map[string]any) {
			r["targets"] = append(r["targets"].([]map[string]any), chibiTarget("tuned", "Hi"))
		}},
		{"unknown kind", func(r map[string]any) { r["targets"].([]map[string]any)[0]["kind"] = "judge" }},
		{"duplicate kind", func(r map[string]any) { r["targets"].([]map[string]any)[1]["kind"] = "tuned" }},
		{"no messages", func(r map[string]any) { msgs(r, 0) }},
		{"starts with assistant", func(r map[string]any) { msgs(r, 0, a("x"), u("y")) }},
		{"two users in a row", func(r map[string]any) { msgs(r, 0, u("x"), u("y")) }},
		{"ends with assistant", func(r map[string]any) { msgs(r, 0, u("x"), a("y")) }},
		{"system only", func(r map[string]any) { msgs(r, 0, sys("x")) }},
		{"two system messages", func(r map[string]any) { msgs(r, 0, sys("x"), sys("y"), u("z")) }},
		{"system in the middle", func(r map[string]any) { msgs(r, 0, u("x"), sys("y"), u("z")) }},
		{"unknown role", func(r map[string]any) { msgs(r, 0, map[string]string{"role": "tool", "content": "x"}) }},
		{"blank content", func(r map[string]any) { msgs(r, 0, u(" \n\t")) }},
		{"NUL in content", func(r map[string]any) { msgs(r, 0, u("a\x00b")) }},
		{"21 user messages", func(r map[string]any) { msgs(r, 0, manyTurns(21)...) }},
		{"16001 characters", func(r map[string]any) { msgs(r, 0, u(long(16001, "a"))) }},
		{"system counts towards the length", func(r map[string]any) { msgs(r, 0, sys(long(8001, "a")), u(long(8000, "b"))) }},
		{"temperature too high", func(r map[string]any) { sampling(r, "temperature", 2.1) }},
		{"temperature negative", func(r map[string]any) { sampling(r, "temperature", -0.1) }},
		{"top_p zero", func(r map[string]any) { sampling(r, "top_p", 0) }},
		{"top_p above one", func(r map[string]any) { sampling(r, "top_p", 1.1) }},
		{"top_k negative", func(r map[string]any) { sampling(r, "top_k", -1) }},
		{"top_k too high", func(r map[string]any) { sampling(r, "top_k", 201) }},
		{"max_tokens zero", func(r map[string]any) { sampling(r, "max_tokens", 0) }},
		{"max_tokens above the cap", func(r map[string]any) { sampling(r, "max_tokens", 1025) }},
		{"seed negative", func(r map[string]any) { sampling(r, "seed", -1) }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := valid()
			tc.mut(r)
			w := chibiDo(t, s, token, http.MethodPost, "/runtime/chat", r)
			chibiWant(t, w, http.StatusUnprocessableEntity, "validation_failed")
			if !strings.Contains(w.Body.String(), `"message_key":"chibi_sensei.chat.invalid"`) {
				t.Fatalf("issue key = %s", w.Body.String())
			}
		})
	}

	chibiWant(t, chibiDo(t, s, token, http.MethodPost, "/runtime/chat", `{"run_id":"`+run+`","targets":[],"extra":1}`), http.StatusBadRequest, "invalid_request")
	chibiWant(t, chibiDo(t, s, token, http.MethodGet, "/runtime/chat", nil), http.StatusMethodNotAllowed, "method_not_allowed")

	// The length counts Unicode code points, never bytes: 9000 two-byte characters pass.
	ok := chibiChatReq(run, chibiTarget("tuned", long(9000, "ä")))
	w := chibiDo(t, s, token, http.MethodPost, "/runtime/chat", ok)
	chibiWant(t, w, http.StatusOK, "")
	// And the limits themselves pass: 20 user messages, 16000 characters, the extreme sampling values.
	edge := chibiChatReq(run, chibiTarget("tuned", "x"))
	msgs(edge, 0, manyTurns(20)...)
	edge["sampling"] = map[string]any{"temperature": 2, "top_p": 1, "top_k": 200, "max_tokens": 1024, "seed": 0}
	chibiWant(t, chibiDo(t, s, token, http.MethodPost, "/runtime/chat", edge), http.StatusOK, "")
}

func TestChibiSenseiChatPreStreamErrors(t *testing.T) {
	s, token, fake, rt, pid := chibiTestFixture(t, true)
	withBase := chibiSucceededTestRun(t, rt, pid, "qwen35-0p8b", true, 0)
	noBase := chibiSucceededTestRun(t, rt, pid, "qwen35-0p8b", false, 0)
	post := func(r map[string]any) *httptest.ResponseRecorder {
		return chibiDo(t, s, token, http.MethodPost, "/runtime/chat", r)
	}

	chibiWant(t, post(chibiChatReq(chibiFakeID, chibiTarget("tuned", "Hi"))), http.StatusNotFound, "run_not_found")
	failed := chibiNewFailedRun(t, rt, pid)
	chibiWant(t, post(chibiChatReq(failed, chibiTarget("tuned", "Hi"))), http.StatusConflict, "run_not_succeeded")
	w := post(chibiChatReq(noBase, chibiTarget("tuned", "Hi"), chibiTarget("base", "Hi")))
	chibiWant(t, w, http.StatusNotFound, "model_missing")
	if !strings.Contains(w.Body.String(), "The base model file is not available.") {
		t.Fatalf("model_missing text = %s", w.Body.String())
	}
	if len(fake.Launches()) != 0 {
		t.Fatalf("a pre-stream refusal started %d models", len(fake.Launches()))
	}

	// Two other models hold both slots.
	h1 := chibiSucceededTestRun(t, rt, pid, "qwen35-0p8b", false, 0)
	h2 := chibiSucceededTestRun(t, rt, pid, "qwen35-0p8b", false, 0)
	stop := chibiHoldTwo(t, s, token, fake, rt, h1, h2)
	chibiWant(t, post(chibiChatReq(withBase, chibiTarget("tuned", "Hi"))), http.StatusConflict, "runtime_busy")
	stop()

	// Read-only desktop: a chat is a write-class request.
	ro := *s.ConfigSnapshot()
	ro.VirtualDesktop.ReadOnly = true
	s.replaceConfigSnapshot(&ro)
	chibiWant(t, post(chibiChatReq(withBase, chibiTarget("tuned", "Hi"))), http.StatusForbidden, "desktop_readonly")
	ro.VirtualDesktop.ReadOnly = false
	ro.ChibiSensei.Runtime.Enabled = false
	s.replaceConfigSnapshot(&ro)
	chibiWant(t, post(chibiChatReq(withBase, chibiTarget("tuned", "Hi"))), http.StatusServiceUnavailable, "runtime_unavailable")
}

func TestChibiSenseiChatNotInstalled(t *testing.T) {
	s, token, _, rt, pid := chibiTestFixture(t, false)
	run := chibiSucceededTestRun(t, rt, pid, "qwen35-0p8b", false, 0)
	chibiWant(t, chibiDo(t, s, token, http.MethodPost, "/runtime/chat", chibiChatReq(run, chibiTarget("tuned", "Hi"))),
		http.StatusConflict, "runtime_not_installed")
}

func TestChibiSenseiChatStreamsBothTargets(t *testing.T) {
	s, token, fake, rt, pid := chibiTestFixture(t, true)
	run := chibiSucceededTestRun(t, rt, pid, "gemma3-270m", true, 0)
	fake.Replies["tuned:"], fake.Replies["base:"] = "Tuned answer.", "Base answer."

	w := chibiDo(t, s, token, http.MethodPost, "/runtime/chat", chibiChatReq(run,
		chibiTarget("tuned", "Be brief.", "Hi there"), chibiTarget("base", "Hi there")))
	chibiWant(t, w, http.StatusOK, "")
	h := w.Header()
	if h.Get("Content-Type") != "text/event-stream; charset=utf-8" || h.Get("Cache-Control") != "no-store" || h.Get("X-Accel-Buffering") != "no" {
		t.Fatalf("headers = %v", h)
	}
	if !strings.HasPrefix(w.Body.String(), ": chibi-sensei chat\n\n") {
		t.Fatalf("stream starts with %q", w.Body.String()[:30])
	}
	events, comments := chibiParseSSE(t, w.Body.String())
	if comments != 1 || len(events) == 0 || events[len(events)-1].name != "end" {
		t.Fatalf("comments = %d, events = %v", comments, events)
	}
	for target, want := range map[string]string{"tuned": "Tuned answer.", "base": "Base answer."} {
		var states []string
		var text strings.Builder
		var terminal []chibiSSE
		for _, e := range chibiTargetEvents(events, target) {
			switch e.name {
			case "status":
				states = append(states, e.data["state"].(string))
			case "delta":
				text.WriteString(e.data["text"].(string))
			case "done", "error":
				terminal = append(terminal, e)
			default:
				t.Fatalf("%s: unexpected event %s", target, e.name)
			}
		}
		if strings.Join(states, ",") != "loading,generating" || text.String() != want {
			t.Fatalf("%s: states %v text %q", target, states, text.String())
		}
		if len(terminal) != 1 || terminal[0].name != "done" || terminal[0].data["backend"] != "cpu" || terminal[0].data["finish_reason"] != "stop" ||
			terminal[0].data["completion_tokens"] != float64(2) {
			t.Fatalf("%s: terminal events = %v", target, terminal)
		}
	}
	var keys []string
	for _, l := range fake.Launches() {
		keys = append(keys, l.Key)
	}
	if len(keys) != 2 {
		t.Fatalf("launches = %v", keys)
	}
	joined := strings.Join(keys, " ")
	if !strings.Contains(joined, "tuned:"+run) || !strings.Contains(joined, "base:base/gemma3-270m-"+chibiTestRevision[:12]+"-llamacpp-b9994.gguf") {
		t.Fatalf("launch keys = %v", keys)
	}
	// gemma3 merges the system message into the first user message, as the trainer did; the base had none.
	tuned := fake.Requests("tuned:")
	if len(tuned) != 1 || len(tuned[0].Messages) != 1 || tuned[0].Messages[0].Role != "user" ||
		tuned[0].Messages[0].Content != "Be brief.\n\nHi there" || len(tuned[0].Stop) != 1 || tuned[0].Stop[0] != "<end_of_turn>" {
		t.Fatalf("tuned request = %+v", tuned)
	}
	if got := rt.chatRunFor("base:base/gemma3-270m-" + chibiTestRevision[:12] + "-llamacpp-b9994.gguf"); got != run {
		t.Fatalf("base instance run = %q", got)
	}
	// The runtime view names the run of each instance.
	view := chibiRuntimeState(t, s, token)
	insts := view["instances"].([]any)
	if len(insts) != 2 {
		t.Fatalf("instances = %v", insts)
	}
	for _, in := range insts {
		m := in.(map[string]any)
		if m["run_id"] != run || (m["kind"] != "tuned" && m["kind"] != "base") {
			t.Fatalf("instance = %v", m)
		}
	}
}

func TestChibiSenseiChatOneTargetFails(t *testing.T) {
	s, token, fake, rt, pid := chibiTestFixture(t, true)
	run := chibiSucceededTestRun(t, rt, pid, "qwen35-0p8b", true, 0)
	fake.FailLaunch["base:"] = errors.New("exec /secret/path/llama-server: boom secret-token-123")
	w := chibiDo(t, s, token, http.MethodPost, "/runtime/chat", chibiChatReq(run, chibiTarget("tuned", "Hi"), chibiTarget("base", "Hi")))
	chibiWant(t, w, http.StatusOK, "")
	if strings.Contains(w.Body.String(), "secret") {
		t.Fatalf("upstream error text in the stream: %s", w.Body.String())
	}
	events, _ := chibiParseSSE(t, w.Body.String())
	if events[len(events)-1].name != "end" {
		t.Fatalf("last event = %s", events[len(events)-1].name)
	}
	base := chibiTargetEvents(events, "base")
	if last := base[len(base)-1]; last.name != "error" || last.data["code"] != "start_failed" || last.data["message"] != "The model could not be started." {
		t.Fatalf("base events = %v", base)
	}
	tuned := chibiTargetEvents(events, "tuned")
	if last := tuned[len(tuned)-1]; last.name != "done" {
		t.Fatalf("tuned events = %v", tuned)
	}
	count := 0
	for _, e := range events {
		if e.name == "done" || e.name == "error" {
			count++
		}
	}
	if count != 2 {
		t.Fatalf("%d terminal events, want 2", count)
	}
}

func TestChibiSenseiChatContextExceeded(t *testing.T) {
	s, token, fake, rt, pid := chibiTestFixture(t, true)
	run := chibiSucceededTestRun(t, rt, pid, "qwen35-0p8b", false, 0)
	fake.ChatErr["tuned:"] = chatrt.ErrContext
	w := chibiDo(t, s, token, http.MethodPost, "/runtime/chat", chibiChatReq(run, chibiTarget("tuned", "Hi")))
	chibiWant(t, w, http.StatusOK, "")
	events, _ := chibiParseSSE(t, w.Body.String())
	tuned := chibiTargetEvents(events, "tuned")
	if last := tuned[len(tuned)-1]; last.name != "error" || last.data["code"] != "context_exceeded" {
		t.Fatalf("events = %v", tuned)
	}
	fake.ChatErr["tuned:"] = errors.New("model said: secret-token-123")
	w = chibiDo(t, s, token, http.MethodPost, "/runtime/chat", chibiChatReq(run, chibiTarget("tuned", "Hi")))
	events, _ = chibiParseSSE(t, w.Body.String())
	tuned = chibiTargetEvents(events, "tuned")
	if last := tuned[len(tuned)-1]; last.name != "error" || last.data["code"] != "generation_failed" || strings.Contains(w.Body.String(), "secret") {
		t.Fatalf("events = %v", tuned)
	}
}

func TestChibiSenseiChatClientCloseCancels(t *testing.T) {
	s, token, fake, rt, pid := chibiTestFixture(t, true)
	run := chibiSucceededTestRun(t, rt, pid, "qwen35-0p8b", false, 0)
	fake.Hold = make(chan struct{}) // never closed: only the client leaving ends the answer
	srv := httptest.NewServer(handleChibiSensei(s))
	defer srv.Close()

	raw, _ := json.Marshal(chibiChatReq(run, chibiTarget("tuned", "Hi")))
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, srv.URL+chibiSenseiAPIBase+"/runtime/chat", strings.NewReader(string(raw)))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	if res.StatusCode != http.StatusOK {
		t.Fatalf("status = %d", res.StatusCode)
	}
	sc := bufio.NewScanner(res.Body)
	sawStatus := false
	for sc.Scan() {
		if strings.HasPrefix(sc.Text(), "event: status") {
			sawStatus = true
			break
		}
	}
	if !sawStatus {
		t.Fatalf("no status event: %v", sc.Err())
	}
	// Wait until the model answers (it waits on Hold), then the client leaves.
	deadline := time.Now().Add(5 * time.Second)
	for len(fake.Requests("tuned:")) == 0 && time.Now().Before(deadline) {
		time.Sleep(5 * time.Millisecond)
	}
	cancel()
	res.Body.Close()
	select {
	case key := <-fake.Cancelled():
		if !strings.HasPrefix(key, "tuned:") {
			t.Fatalf("cancelled key = %q", key)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("the generation did not see the client leave within 2 s")
	}
}

func TestChibiSenseiChatRetireStopsInstances(t *testing.T) {
	s, token, fake, rt, pid := chibiTestFixture(t, true)
	run := chibiSucceededTestRun(t, rt, pid, "qwen35-0p8b", false, 0)
	other := chibiSucceededTestRun(t, rt, pid, "qwen35-0p8b", false, 0)
	stop := chibiHoldTwo(t, s, token, fake, rt, run, other)
	defer stop()
	done := make(chan struct{})
	go func() { s.closeChibiSensei(); close(done) }()
	select {
	case <-done:
	case <-time.After(20 * time.Second):
		t.Fatal("closeChibiSensei did not return")
	}
	if fake.Stopped() < 1 {
		t.Fatalf("Stopped() = %d, want at least 1", fake.Stopped())
	}
}

// chibiRunFor is a test accessor for the run that used an instance last.
func (rt *chibiSenseiRuntime) chatRunFor(key string) string {
	rt.chatMu.Lock()
	defer rt.chatMu.Unlock()
	return rt.chatRuns[key]
}
