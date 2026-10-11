package agent

import (
	"context"
	"encoding/json"
	"io"
	"io/fs"
	"log/slog"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

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
	out, _ = dispatchCYDCreator(bound, ToolCall{Action: cydcreator.ToolReadProject, Params: map[string]any{"job_id": "ccj_other"}}, dc)
	if !strings.Contains(out, "does not match") {
		t.Fatalf("mismatched job_id = %s", out)
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

func TestCYDCreatorToolErrorHidesUntypedErrors(t *testing.T) {
	out := cydCreatorToolError(&fs.PathError{Op: "open", Path: filepath.Join("C:", "secret", "cyd.db"), Err: fs.ErrPermission})
	if body := cydOutput(t, out); strings.Contains(out, "secret") || body["status"] != "error" || body["code"] != cydcreator.CodeInternal {
		t.Fatalf("untyped error = %s", out)
	}
	typed := cydCreatorToolError(cydcreator.NewError(cydcreator.CodeNotFound, "file not found", map[string]any{"path": "src/app/x.cpp"}))
	if body := cydOutput(t, typed); body["code"] != cydcreator.CodeNotFound || body["details"].(map[string]any)["path"] != "src/app/x.cpp" {
		t.Fatalf("typed error = %s", typed)
	}
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
