package agent

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"math"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"
	"unicode/utf8"

	"aurago/internal/config"
	"aurago/internal/cydcreator"
)

// cydAgentBuilder is a Builder whose builds succeed unless failNext is set.
type cydAgentBuilder struct {
	mu       sync.Mutex
	failNext bool
}

func (b *cydAgentBuilder) Status(context.Context) cydcreator.ToolchainStatus {
	return cydcreator.ToolchainStatus{DockerAvailable: true, Image: "test:1", Present: true}
}
func (b *cydAgentBuilder) Pull(context.Context) error { return nil }
func (b *cydAgentBuilder) Addr2Line(context.Context, cydcreator.Addr2LineRequest) ([]cydcreator.Frame, error) {
	return nil, nil
}
func (b *cydAgentBuilder) Build(_ context.Context, req cydcreator.BuildRequest, _ func(string)) (cydcreator.BuildOutput, error) {
	b.mu.Lock()
	fail := b.failNext
	b.failNext = false
	b.mu.Unlock()
	if fail {
		return cydcreator.BuildOutput{ExitCode: cydcreator.BuildExitCompileError,
			Log: "/work/" + req.BuildID + "/src/app/app_main.cpp:2:1: error: expected ';' before '}' token\n"}, nil
	}
	return cydcreator.BuildOutput{ExitCode: 0, Result: cydcreator.BuildResultFile{OK: true, Env: req.Env},
		Files: map[string][]byte{"merged.bin": []byte("m"), "firmware.bin": []byte("f"), "firmware.elf": []byte("e")}}, nil
}

func cydAgentGenerate(in cydcreator.GenerateInput) (cydcreator.GenerateOutput, error) {
	files := map[string][]byte{"platformio.ini": []byte("[env]\n"), "src/generated/ui.h": []byte("#pragma once\n")}
	for k, v := range in.AppFiles {
		files[k] = v
	}
	return cydcreator.GenerateOutput{Files: files}, nil
}

type cydAgentRunner struct {
	fn func(ctx context.Context, run cydcreator.JobRun) error
}

func (r cydAgentRunner) RunCYDJob(ctx context.Context, run cydcreator.JobRun) error {
	return r.fn(ctx, run)
}

// newCYDAgentService publishes a service as the default for the dispatcher.
func newCYDAgentService(t *testing.T) (*cydcreator.Service, *cydAgentBuilder, cydcreator.ProjectSummary) {
	t.Helper()
	dir := t.TempDir()
	builder := &cydAgentBuilder{}
	svc, err := cydcreator.NewService(cydcreator.ServiceConfig{WorkspaceDir: filepath.Join(dir, "ws"), DataDir: filepath.Join(dir, "data"),
		DBPath: filepath.Join(dir, "data", "cyd.db"), Builder: builder, Generate: cydAgentGenerate,
		Logger: slog.New(slog.NewTextHandler(io.Discard, nil))})
	if err != nil {
		t.Fatal(err)
	}
	previous := cydcreator.DefaultService()
	cydcreator.SetDefaultService(svc)
	t.Cleanup(func() {
		cydcreator.SetDefaultService(previous)
		_ = svc.Close()
	})
	p, err := svc.CreateProject(context.Background(), cydcreator.CreateProjectInput{Name: "Agent", Board: "esp32-2432s028r"})
	if err != nil {
		t.Fatal(err)
	}
	return svc, builder, p
}

func cydDispatchContext() *DispatchContext {
	cfg := &config.Config{}
	cfg.CYDCreator.Enabled = true
	return &DispatchContext{Cfg: cfg}
}

func cydOutput(t *testing.T, out string) map[string]any {
	t.Helper()
	var body map[string]any
	if err := json.Unmarshal([]byte(strings.TrimPrefix(out, "Tool Output: ")), &body); err != nil {
		t.Fatalf("output %q: %v", out, err)
	}
	return body
}

func waitCYDJob(t *testing.T, svc *cydcreator.Service, jobID string, statuses ...string) cydcreator.Job {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for {
		j, err := svc.GetJob(context.Background(), jobID)
		for _, s := range statuses {
			if err == nil && j.Status == s {
				time.Sleep(20 * time.Millisecond) // let the run release its slot
				return j
			}
		}
		if time.Now().After(deadline) {
			t.Fatalf("job = %+v, %v", j, err)
		}
		time.Sleep(5 * time.Millisecond)
	}
}

func TestDispatchCYDCreatorGuards(t *testing.T) {
	dc := cydDispatchContext()
	out, handled := dispatchCYDCreator(context.Background(), ToolCall{Action: cydcreator.ToolReadProject}, dc)
	if !handled || cydOutput(t, out)["status"] != "policy_denied" || classifyLegacyToolResult(out) != ToolResultDenied {
		t.Fatalf("unbound call = %s", out)
	}
	if _, handled := dispatchCYDCreator(context.Background(), ToolCall{Action: "game_maker_file"}, dc); handled {
		t.Fatal("handled a foreign tool")
	}
	bound := cydcreator.WithJobContext(context.Background(), "ccj_bound")
	for _, other := range []any{"ccj_other", 42.0, true, map[string]any{"id": "ccj_bound"}} {
		out, _ = dispatchCYDCreator(bound, ToolCall{Action: cydcreator.ToolReadProject, Params: map[string]any{"job_id": other}}, dc)
		if !strings.Contains(out, "does not match") {
			t.Fatalf("mismatched job_id %v = %s", other, out)
		}
	}
	off := cydDispatchContext()
	off.Cfg.CYDCreator.Enabled = false
	if out, _ := dispatchCYDCreator(bound, ToolCall{Action: cydcreator.ToolReadProject}, off); !strings.Contains(out, "switched off") {
		t.Fatalf("disabled = %s", out)
	}
	ro := cydDispatchContext()
	ro.Cfg.VirtualDesktop.ReadOnly = true
	if out, _ := dispatchCYDCreator(bound, ToolCall{Action: cydcreator.ToolWriteFile, Params: map[string]any{"path": "src/app/a.cpp", "content": "x"}}, ro); !strings.Contains(out, "read-only") {
		t.Fatalf("read-only write = %s", out)
	}
	if out, _ := dispatchCYDCreator(bound, ToolCall{Action: cydcreator.ToolReadProject}, nil); !strings.Contains(out, "switched off") {
		t.Fatalf("no dispatch context = %s", out)
	}
}

func TestDispatchExecRoutesEveryCYDCreatorTool(t *testing.T) {
	for _, name := range cydcreator.ToolNames() {
		out, handled := dispatchExec(context.Background(), ToolCall{Action: name}, cydDispatchContext())
		if !handled || cydOutput(t, out)["status"] != "policy_denied" {
			t.Errorf("%s: handled=%v out=%s", name, handled, out)
		}
	}
}

// The read-only desktop refuses the six write tools; the three read tools
// pass the gate and reach the service (which refuses the unknown job).
func TestDispatchCYDCreatorReadOnlyRefusesEveryWriteTool(t *testing.T) {
	newCYDAgentService(t)
	ro := cydDispatchContext()
	ro.Cfg.VirtualDesktop.ReadOnly = true
	bound := cydcreator.WithJobContext(context.Background(), "ccj_bound")
	params := map[string]any{"path": "src/app/a.cpp", "content": "x", "topic": "overview", "summary": "s",
		"ops": []any{map[string]any{"op": "set_theme", "theme": "night"}}, "plan": `{"summary":"x","screens":["home"],"wifi":"off"}`}
	reads := 0
	for _, name := range cydcreator.ToolNames() {
		out, _ := dispatchCYDCreator(bound, ToolCall{Action: name, Params: params}, ro)
		body := cydOutput(t, out)
		if slices.Contains(cydCreatorWriteTools, name) {
			if body["status"] != "policy_denied" || !strings.Contains(out, "read-only") {
				t.Errorf("%s on a read-only desktop = %s", name, out)
			}
			continue
		}
		reads++
		if body["status"] != "error" || body["code"] != cydcreator.CodeJobState {
			t.Errorf("read tool %s on a read-only desktop = %s", name, out)
		}
	}
	if len(cydCreatorWriteTools) != 6 || reads != 3 {
		t.Fatalf("write tools %v, read tools %d", cydCreatorWriteTools, reads)
	}
}

func TestCYDCreatorToolErrorHidesUntypedErrors(t *testing.T) {
	var logs bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&logs, nil))
	out := cydCreatorToolError(logger, &fs.PathError{Op: "open", Path: filepath.Join("C:", "secret", "cyd.db"), Err: fs.ErrPermission})
	if body := cydOutput(t, out); strings.Contains(out, "secret") || body["status"] != "error" || body["code"] != cydcreator.CodeInternal {
		t.Fatalf("untyped error = %s", out)
	}
	if !strings.Contains(logs.String(), "secret") {
		t.Fatalf("the cause was not logged: %q", logs.String())
	}
	typed := cydCreatorToolError(nil, cydcreator.NewError(cydcreator.CodeNotFound, "file not found", map[string]any{"path": "src/app/x.cpp"}))
	if body := cydOutput(t, typed); body["code"] != cydcreator.CodeNotFound || body["details"].(map[string]any)["path"] != "src/app/x.cpp" {
		t.Fatalf("typed error = %s", typed)
	}
}

func TestCYDLineArg(t *testing.T) {
	for _, tc := range []struct {
		value any
		want  int
		ok    bool
	}{{nil, 0, true}, {0.0, 0, true}, {12.0, 12, true}, {7, 7, true}, {" 30 ", 30, true}, {json.Number("4"), 4, true},
		{-1.0, 0, false}, {1.5, 0, false}, {"x", 0, false}, {true, 0, false}, {1e9, 0, false}, {math.Inf(1), 0, false}} {
		got, err := cydLineArg(map[string]any{"start_line": tc.value}, "start_line")
		if (err == nil) != tc.ok || got != tc.want {
			t.Errorf("%v: got %d, %v", tc.value, got, err)
		}
		if err != nil && cydcreator.ErrorCode(err) != cydcreator.CodeBadRequest {
			t.Errorf("%v: error %v", tc.value, err)
		}
	}
	if n, err := cydLineArg(map[string]any{}, "end_line"); n != 0 || err != nil {
		t.Fatalf("absent = %d, %v", n, err)
	}
}

func TestPageCYDText(t *testing.T) {
	var b strings.Builder
	for i := 1; i <= 400; i++ {
		fmt.Fprintf(&b, "const char *s%03d = \"<a href='x'>&</a>\";\r\n", i)
	}
	content := b.String()
	lineCost := cydEscapedLen("const char *s001 = \"<a href='x'>&</a>\";\r\n")
	if lineCost <= len("const char *s001 = \"<a href='x'>&</a>\";\r\n") {
		t.Fatalf("escaped cost %d does not count escapes", lineCost)
	}
	// Pages of whole lines cover the file exactly once.
	budget := 2000
	var joined strings.Builder
	pages := 0
	for start := 1; ; pages++ {
		page, err := pageCYDText(content, start, 0, budget)
		if err != nil {
			t.Fatal(err)
		}
		if page.StartLine != start || page.TotalLines != 400 || page.Clipped || cydEscapedLen(page.Content) > budget ||
			page.EndLine-start+1 != min(budget/lineCost, 400-start+1) || !strings.HasSuffix(page.Content, "\r\n") {
			t.Fatalf("page %+v", page)
		}
		joined.WriteString(page.Content)
		if page.EndLine == page.TotalLines {
			if page.Truncated {
				t.Fatalf("last page truncated: %+v", page)
			}
			break
		}
		if !page.Truncated {
			t.Fatalf("budget-cut page not truncated: %+v", page)
		}
		start = page.EndLine + 1
	}
	if joined.String() != content || pages < 5 {
		t.Fatalf("paging lost content (%d pages)", pages)
	}
	// Explicit ranges: 0 is the default, an end beyond the file is clamped,
	// a range inside the budget is not truncated.
	if page, err := pageCYDText(content, 0, 3, budget); err != nil || page.StartLine != 1 || page.EndLine != 3 || page.Truncated {
		t.Fatalf("0..3 = %+v, %v", page, err)
	}
	if page, err := pageCYDText(content, 399, 9999, budget); err != nil || page.EndLine != 400 || page.Truncated ||
		!strings.HasPrefix(page.Content, "const char *s399") {
		t.Fatalf("399..9999 = %+v, %v", page, err)
	}
	if page, err := pageCYDText(content, 10, 20, lineCost*4); err != nil || page.EndLine != 13 || !page.Truncated {
		t.Fatalf("budget inside a range = %+v, %v", page, err)
	}
	for _, bad := range [][2]int{{5, 4}, {401, 0}, {401, 402}} {
		if _, err := pageCYDText(content, bad[0], bad[1], budget); cydcreator.ErrorCode(err) != cydcreator.CodeBadRequest {
			t.Errorf("range %v: %v", bad, err)
		}
	}
	// No trailing newline, an empty file.
	if page, err := pageCYDText("a\nb", 0, 0, budget); err != nil || page.TotalLines != 2 || page.Content != "a\nb" {
		t.Fatalf("no final newline = %+v, %v", page, err)
	}
	if page, err := pageCYDText("", 0, 0, budget); err != nil || page.TotalLines != 0 || page.Content != "" || page.Truncated {
		t.Fatalf("empty = %+v, %v", page, err)
	}
	if _, err := pageCYDText("", 2, 0, budget); cydcreator.ErrorCode(err) != cydcreator.CodeBadRequest {
		t.Fatalf("empty file line 2: %v", err)
	}
	// A line longer than the page is cut at a character boundary.
	huge := strings.Repeat("ä\"", 3000) + "\nnext\n"
	page, err := pageCYDText(huge, 0, 0, 1000)
	if err != nil || !page.Clipped || !page.Truncated || page.EndLine != 1 || page.TotalLines != 2 ||
		page.LineBytes != len(huge)-len("next\n") || page.ClippedBytes != len(page.Content) || !utf8.ValidString(page.Content) ||
		cydEscapedLen(page.Content) > 1000 || cydEscapedLen(page.Content) < 1000-8 {
		t.Fatalf("huge line = %+v, %v", page, err)
	}
}

// cydCall runs one tool and checks that its answer reaches the model whole.
func cydCall(t *testing.T, ctx context.Context, dc *DispatchContext, name string, params map[string]any) (string, map[string]any) {
	t.Helper()
	out, handled := dispatchCYDCreator(ctx, ToolCall{Action: name, Params: params}, dc)
	if !handled {
		t.Fatalf("%s not handled", name)
	}
	if size, limit := cydModelBytes(out), toolResultInlineBudget(dc.Cfg); size > limit {
		t.Errorf("%s answer has %d model bytes, the inline budget is %d", name, size, limit)
	}
	return out, cydOutput(t, out)
}

// cydReadAll pages a file with cyd_creator_read_file and returns its content.
func cydReadAll(t *testing.T, ctx context.Context, dc *DispatchContext, path string) (string, map[string]any) {
	t.Helper()
	var content strings.Builder
	var file map[string]any
	for start, pages := 1, 0; ; pages++ {
		_, body := cydCall(t, ctx, dc, cydcreator.ToolReadFile, map[string]any{"path": path, "start_line": start})
		if body["status"] != "ok" || pages > 500 {
			t.Fatalf("read %s from %d = %v", path, start, body)
		}
		file = body["file"].(map[string]any)
		content.WriteString(file["content"].(string))
		end, total := int(file["end_line"].(float64)), int(file["total_lines"].(float64))
		if end >= total {
			return content.String(), file
		}
		if !strings.Contains(body["next_action"].(string), fmt.Sprintf("start_line=%d", end+1)) {
			t.Fatalf("next_action = %v", body["next_action"])
		}
		start = end + 1
	}
}

func TestDispatchCYDCreatorPagesFilesAndClipsTheProject(t *testing.T) {
	svc, _, p := newCYDAgentService(t)
	dc := cydDispatchContext()
	dc.Cfg.Agent.ToolOutputLimit = 8000
	var big strings.Builder
	for i := 1; i <= 3000; i++ {
		fmt.Fprintf(&big, "static const char *s%04d = \"<b>&amp;</b>\"; // line %d\n", i, i)
	}
	failures := []string{}
	fail := func(format string, args ...any) { failures = append(failures, fmt.Sprintf(format, args...)) }
	svc.SetRunner(cydAgentRunner{fn: func(ctx context.Context, run cydcreator.JobRun) error {
		// Missing or non-string content never empties a file; "" is a real empty file.
		for _, params := range []map[string]any{{"path": "src/app/app_main.cpp"}, {"path": "src/app/app_main.cpp", "content": 7.0}} {
			if _, body := cydCall(t, ctx, dc, cydcreator.ToolWriteFile, params); body["code"] != cydcreator.CodeBadRequest {
				fail("write without string content = %v", body)
			}
		}
		if _, body := cydCall(t, ctx, dc, cydcreator.ToolReadFile, map[string]any{"path": "src/app/app_main.cpp"}); body["file"].(map[string]any)["total_bytes"].(float64) == 0 {
			fail("app_main.cpp was emptied: %v", body)
		}
		if _, body := cydCall(t, ctx, dc, cydcreator.ToolWriteFile, map[string]any{"path": "src/app/empty.cpp", "content": ""}); body["status"] != "ok" {
			fail("empty write = %v", body)
		}
		if _, body := cydCall(t, ctx, dc, cydcreator.ToolReadFile, map[string]any{"path": "src/app/empty.cpp"}); body["status"] != "ok" ||
			body["file"].(map[string]any)["total_lines"].(float64) != 0 || body["file"].(map[string]any)["content"] != "" {
			fail("empty read = %v", body)
		}
		// A big file reads back exactly, page by page.
		if _, body := cydCall(t, ctx, dc, cydcreator.ToolWriteFile, map[string]any{"path": "src/app/big.cpp", "content": big.String()}); body["status"] != "ok" {
			fail("big write = %v", body)
		}
		if got, file := cydReadAll(t, ctx, dc, "src/app/big.cpp"); got != big.String() || file["sha256"] != svcSHA(big.String()) {
			fail("big file paged back with %d of %d bytes", len(got), big.Len())
		}
		for _, params := range []map[string]any{{"start_line": 3001}, {"start_line": 10, "end_line": 9}, {"start_line": -1}, {"end_line": "x"}} {
			params["path"] = "src/app/big.cpp"
			if _, body := cydCall(t, ctx, dc, cydcreator.ToolReadFile, params); body["code"] != cydcreator.CodeBadRequest {
				fail("range %v = %v", params, body)
			}
		}
		if _, body := cydCall(t, ctx, dc, cydcreator.ToolReadFile, map[string]any{"path": "src/app/big.cpp", "start_line": 2999, "end_line": 5000}); body["status"] != "ok" ||
			body["file"].(map[string]any)["end_line"].(float64) != 3000 || body["file"].(map[string]any)["truncated"] != false || body["next_action"] != nil {
			fail("clamped end = %v", body)
		}
		// One huge line is cut and marked.
		if _, body := cydCall(t, ctx, dc, cydcreator.ToolWriteFile, map[string]any{"path": "src/app/huge.cpp", "content": "// " + strings.Repeat("x", 20000) + "\nint h;\n"}); body["status"] != "ok" {
			fail("huge write = %v", body)
		}
		_, body := cydCall(t, ctx, dc, cydcreator.ToolReadFile, map[string]any{"path": "src/app/huge.cpp"})
		if file := body["file"].(map[string]any); file["clipped_line"] == nil || file["truncated"] != true || len(file["content"].(string)) >= 20003 ||
			!strings.Contains(body["next_action"].(string), "start_line=2") {
			fail("huge line = %v", body)
		}
		// A layout too large for one result is summarized and paged with read_file.
		var ops []any
		for i := 0; i < 40; i++ {
			ops = append(ops, map[string]any{"op": "add_widget", "screen": "home", "widget": map[string]any{"type": "label",
				"name": fmt.Sprintf("lbl_%02d", i), "x": 0, "y": i * 5, "props": map[string]any{"text": fmt.Sprintf("Label number %d with a longer text", i)}}})
		}
		if _, body := cydCall(t, ctx, dc, cydcreator.ToolUpdateLayout, map[string]any{"ops": ops}); body["status"] != "ok" {
			fail("layout ops = %v", body)
		}
		_, body = cydCall(t, ctx, dc, cydcreator.ToolReadProject, nil)
		project, _ := body["project"].(map[string]any)
		if body["truncated"] != true || project == nil || project["layout"] != nil || body["total_bytes"].(float64) <= 8000 ||
			!strings.Contains(body["next_action"].(string), "cyd.project.json") {
			fail("big project = %v", body)
		} else if screens := project["layout_summary"].(map[string]any)["screens"].([]any); len(screens) == 0 ||
			screens[0].(map[string]any)["widgets"].(float64) < 40 {
			fail("layout summary = %v", project["layout_summary"])
		}
		layoutText, file := cydReadAll(t, ctx, dc, cydcreator.ProjectFile)
		if layout, err := cydcreator.ParseProject([]byte(layoutText)); err != nil || file["sha256"] != svcSHA(layoutText) {
			fail("paged layout = %v", err)
		} else if _, _, ok := layout.FindWidget("lbl_39"); !ok {
			fail("paged layout misses lbl_39")
		}
		_, body = cydCall(t, ctx, dc, cydcreator.ToolFinish, map[string]any{"summary": "Paged."})
		if body["status"] != "ok" {
			fail("finish = %v", body)
		}
		return nil
	}})
	job, err := svc.StartJob(context.Background(), p.ID, cydcreator.StartJobInput{Mode: cydcreator.JobModeChange, Prompt: "Page"})
	if err != nil {
		t.Fatal(err)
	}
	done := waitCYDJob(t, svc, job.ID, cydcreator.JobSucceeded, cydcreator.JobFailed)
	if len(failures) > 0 || done.Status != cydcreator.JobSucceeded {
		t.Fatalf("job = %+v\n%s", done, strings.Join(failures, "\n"))
	}
}

func svcSHA(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])
}

func TestDispatchCYDCreatorRunsEveryTool(t *testing.T) {
	svc, builder, p := newCYDAgentService(t)
	dc := cydDispatchContext()
	outputs := map[string]map[string]any{}
	var raw []string
	call := func(ctx context.Context, name string, params map[string]any) map[string]any {
		out, handled := dispatchCYDCreator(ctx, ToolCall{Action: name, Params: params}, dc)
		if !handled {
			t.Errorf("%s not handled", name)
		}
		raw = append(raw, out)
		return cydOutput(t, out)
	}
	svc.SetRunner(cydAgentRunner{fn: func(ctx context.Context, run cydcreator.JobRun) error {
		switch run.Phase {
		case cydcreator.PhasePlanning:
			outputs["read_project"] = call(ctx, cydcreator.ToolReadProject, nil)
			outputs["reference"] = call(ctx, cydcreator.ToolAPIReference, map[string]any{"topic": "widgets"})
			outputs["write_in_planning"] = call(ctx, cydcreator.ToolWriteFile, map[string]any{"path": "src/app/a.cpp", "content": "x"})
			outputs["bad_plan"] = call(ctx, cydcreator.ToolSubmitPlan, map[string]any{"plan": map[string]any{"summary": "x"}})
			outputs["plan"] = call(ctx, cydcreator.ToolSubmitPlan, map[string]any{"plan": `{"summary":"Counter app","screens":["home"],"wifi":"off"}`, "job_id": run.Job.ID})
		case cydcreator.PhaseLayout:
			outputs["layout"] = call(ctx, cydcreator.ToolUpdateLayout, map[string]any{"ops": []any{map[string]any{"op": "add_widget", "screen": "home",
				"widget": map[string]any{"type": "button", "name": "btn_count", "x": 10, "y": 10, "props": `{"text":"+1"}`,
					"events": []any{map[string]any{"on": "clicked", "action": map[string]any{"type": "logic", "hint": "count up"}}}}}}})
			outputs["bad_layout"] = call(ctx, cydcreator.ToolUpdateLayout, map[string]any{"ops": []any{map[string]any{"op": "delete_widget", "id": "nope"}}})
		case cydcreator.PhaseCoding:
			outputs["write"] = call(ctx, cydcreator.ToolWriteFile, map[string]any{"path": "src/app/app_main.cpp", "content": "#include \"ui.h\"\nvoid app_setup() {}\nvoid app_loop() {}\nvoid on_btn_count_clicked(lv_event_t *e) {}\n"})
			outputs["write_generated"] = call(ctx, cydcreator.ToolWriteFile, map[string]any{"path": "src/generated/ui.h", "content": "x"})
			outputs["read"] = call(ctx, cydcreator.ToolReadFile, map[string]any{"path": "src/app/app_main.cpp"})
			outputs["extra"] = call(ctx, cydcreator.ToolWriteFile, map[string]any{"path": "src/app/tmp.cpp", "content": "int t;\n"})
			outputs["delete"] = call(ctx, cydcreator.ToolDeleteFile, map[string]any{"path": "src/app/tmp.cpp"})
			builder.mu.Lock()
			builder.failNext = true
			builder.mu.Unlock()
			outputs["build_failed"] = call(ctx, cydcreator.ToolBuild, nil)
			outputs["build"] = call(ctx, cydcreator.ToolBuild, nil)
			outputs["finish"] = call(ctx, cydcreator.ToolFinish, map[string]any{"summary": "Counts taps."})
		}
		return nil
	}})
	ctx := context.Background()
	job, err := svc.StartJob(ctx, p.ID, cydcreator.StartJobInput{Mode: cydcreator.JobModeNew, Prompt: "A tap counter"})
	if err != nil {
		t.Fatal(err)
	}
	waitCYDJob(t, svc, job.ID, cydcreator.JobAwaitingConfirmation)
	if _, err := svc.ConfirmJob(ctx, job.ID, ""); err != nil {
		t.Fatal(err)
	}
	done := waitCYDJob(t, svc, job.ID, cydcreator.JobSucceeded, cydcreator.JobFailed)
	if done.Status != cydcreator.JobSucceeded {
		t.Fatalf("job = %+v\noutputs: %v", done, raw)
	}
	checks := map[string]string{"read_project": "ok", "reference": "ok", "write_in_planning": "error", "bad_plan": "error",
		"plan": "ok", "layout": "ok", "bad_layout": "error", "write": "ok", "write_generated": "error", "read": "ok",
		"extra": "ok", "delete": "ok", "build_failed": "failed", "build": "ok", "finish": "ok"}
	for name, want := range checks {
		if got := outputs[name]["status"]; got != want {
			t.Errorf("%s status = %v, want %s (%v)", name, got, want, outputs[name])
		}
	}
	if outputs["write_in_planning"]["code"] != cydcreator.CodeJobState || outputs["write_generated"]["code"] != cydcreator.CodeFileReadOnly ||
		outputs["bad_layout"]["code"] != cydcreator.CodeInvalidLayout || outputs["bad_plan"]["code"] != cydcreator.CodeBadRequest {
		t.Fatalf("error codes = %v / %v / %v / %v", outputs["write_in_planning"], outputs["write_generated"], outputs["bad_layout"], outputs["bad_plan"])
	}
	if diags := outputs["build_failed"]["diagnostics"].([]any); len(diags) != 1 || outputs["build_failed"]["error_code"] != cydcreator.CodeBuildFailed {
		t.Fatalf("failed build = %v", outputs["build_failed"])
	}
	if !strings.Contains(outputs["reference"]["reference"].(string), "Generated symbols") {
		t.Fatalf("reference = %v", outputs["reference"])
	}
	if file := outputs["read"]["file"].(map[string]any); !strings.Contains(file["content"].(string), "on_btn_count_clicked") {
		t.Fatalf("read = %v", file)
	}
	for _, out := range raw {
		if strings.Contains(out, `\`+"u003c") || strings.Contains(out, `\`+"u0026") {
			t.Fatalf("output escapes HTML: %s", out)
		}
	}
	layout, _, _, _ := svc.GetLayout(ctx, p.ID)
	if _, _, ok := layout.FindWidget("btn_count"); !ok {
		t.Fatal("the layout op was not published")
	}
}
