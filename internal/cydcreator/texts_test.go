package cydcreator

import (
	"context"
	"encoding/json"
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"
)

var textPlaceholderShape = regexp.MustCompile(`\{\{[^}]*\}\}`)

// TestTextCodeTables pins the shape of the code tables the client translates:
// identifier codes, well-formed placeholders, no "(s)" plurals, and for every
// job ending a draft variant that adds exactly the draft sentence.
func TestTextCodeTables(t *testing.T) {
	for name, table := range map[string]map[string]string{"step": stepTexts, "msg": messageTexts} {
		if len(table) == 0 {
			t.Fatalf("%s table is empty", name)
		}
		for code, text := range table {
			if !textCodePattern.MatchString(code) {
				t.Errorf("%s code %q is not an identifier", name, code)
			}
			if strings.TrimSpace(text) == "" || strings.Contains(text, "(s)") || strings.TrimSpace(text) != text {
				t.Errorf("%s.%s text %q", name, code, text)
			}
			for _, ph := range textPlaceholderShape.FindAllString(text, -1) {
				if !textPlaceholder.MatchString(ph) {
					t.Errorf("%s.%s has a malformed placeholder %s", name, code, ph)
				}
			}
			if rest := textPlaceholder.ReplaceAllString(text, ""); strings.Contains(rest, "{{") || strings.Contains(rest, "}}") {
				t.Errorf("%s.%s has stray braces: %q", name, code, text)
			}
		}
	}
	endings := jobEndingCodes(t)
	if len(endings) == 0 {
		t.Fatal("no jobEnding literal names a message code")
	}
	for _, code := range endings {
		base, ok := messageTexts[code]
		if !ok {
			t.Errorf("job ending code %q has no message text", code)
			continue
		}
		if draft := messageTexts[code+draftCodeSuffix]; draft != base+draftSentence {
			t.Errorf("draft variant of %q = %q, want the base text plus the draft sentence", code, draft)
		}
	}
	keys := TextKeys()
	if len(keys) != len(stepTexts)+len(messageTexts) || keys["cyd_creator.step.file_written"] != stepTexts["file_written"] ||
		keys["cyd_creator.msg.job_cancelled"] != messageTexts["job_cancelled"] {
		t.Fatalf("TextKeys = %v", keys)
	}
}

func TestRenderTextFillsEveryPlaceholderOnce(t *testing.T) {
	got := renderText("{{a}} and {{b}}, {{a}}; {{missing}}.", map[string]any{"a": "{{b}}", "b": int64(7)})
	if got != "{{b}} and 7, {{b}}; ." {
		t.Fatalf("renderText = %q", got)
	}
	if got := renderText("Build {{n}} succeeded (flash {{percent}}%)", map[string]any{"n": json.Number("2"), "percent": 63}); got != "Build 2 succeeded (flash 63%)" {
		t.Fatalf("renderText = %q", got)
	}
}

func TestTextVarsAreSmallAndFlat(t *testing.T) {
	vars := textVars(map[string]any{
		"path":    "src/app/a.cpp",
		"message": "line one\n\tline two  ",
		"long":    strings.Repeat("ä", 400),
		"n":       3,
		"total":   int64(9),
		"Bad-Key": "x",
		"list":    []string{"x"},
		"flag":    true,
	}, 160)
	want := map[string]any{"path": "src/app/a.cpp", "message": "line one line two", "long": strings.Repeat("ä", 159) + "…", "n": int64(3), "total": int64(9)}
	if len(vars) != len(want) {
		t.Fatalf("vars = %v", vars)
	}
	for k, v := range want {
		if vars[k] != v {
			t.Fatalf("vars[%s] = %#v, want %#v", k, vars[k], v)
		}
	}
	if textVars(nil, 160) != nil || textVars(map[string]any{"x": []int{1}}, 160) != nil {
		t.Fatal("empty vars must be nil")
	}
}

// jobEndingCodes returns the message codes of every jobEnding literal in the
// package sources (field msg); each must be a string literal.
func jobEndingCodes(t *testing.T) []string {
	t.Helper()
	var codes []string
	for _, f := range packageSources(t) {
		ast.Inspect(f.file, func(n ast.Node) bool {
			lit, ok := n.(*ast.CompositeLit)
			if !ok {
				return true
			}
			if id, ok := lit.Type.(*ast.Ident); !ok || id.Name != "jobEnding" {
				return true
			}
			found := false
			for _, elt := range lit.Elts {
				kv, ok := elt.(*ast.KeyValueExpr)
				if !ok || kv.Key.(*ast.Ident).Name != "msg" {
					continue
				}
				found = true
				code, ok := stringLiteral(kv.Value)
				if !ok {
					t.Errorf("%s: jobEnding msg is not a string literal", f.fset.Position(kv.Pos()))
					continue
				}
				codes = append(codes, code)
			}
			if !found {
				t.Errorf("%s: jobEnding without a msg code", f.fset.Position(lit.Pos()))
			}
			return true
		})
	}
	return codes
}

type parsedSource struct {
	fset *token.FileSet
	file *ast.File
}

func packageSources(t *testing.T) []parsedSource {
	t.Helper()
	paths, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	var out []parsedSource
	fset := token.NewFileSet()
	for _, path := range paths {
		if strings.HasSuffix(path, "_test.go") {
			continue
		}
		f, err := parser.ParseFile(fset, path, nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		out = append(out, parsedSource{fset: fset, file: f})
	}
	return out
}

func stringLiteral(e ast.Expr) (string, bool) {
	lit, ok := e.(*ast.BasicLit)
	if !ok || lit.Kind != token.STRING {
		return "", false
	}
	s, err := strconv.Unquote(lit.Value)
	return s, err == nil
}

// TestEveryStepAndSystemLineUsesATableCode reads the package sources: every
// emitStep call passes a string literal code of stepTexts; every
// postSystemMessage call passes a literal code of messageTexts or a local
// variable that is only ever set to such literals, to a jobEnding's msg or
// extended by the draft suffix; postMessage never writes a system line.
func TestEveryStepAndSystemLineUsesATableCode(t *testing.T) {
	steps, systems := 0, 0
	for _, src := range packageSources(t) {
		for _, decl := range src.file.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Body == nil {
				continue
			}
			ast.Inspect(fn.Body, func(n ast.Node) bool {
				call, ok := n.(*ast.CallExpr)
				if !ok {
					return true
				}
				sel, ok := call.Fun.(*ast.SelectorExpr)
				if !ok {
					return true
				}
				pos := src.fset.Position(call.Pos())
				switch sel.Sel.Name {
				case "emitStep":
					steps++
					if len(call.Args) != 5 {
						t.Errorf("%s: emitStep takes project, job, kind, code, vars", pos)
						return true
					}
					code, ok := stringLiteral(call.Args[3])
					if _, known := stepTexts[code]; !ok || !known {
						t.Errorf("%s: emitStep code %q is not a literal of stepTexts", pos, code)
					}
				case "postSystemMessage":
					systems++
					if len(call.Args) != 4 {
						t.Errorf("%s: postSystemMessage takes project, job, code, vars", pos)
						return true
					}
					checkSystemCode(t, src, fn, call.Args[2], pos)
				case "postMessage":
					if id, ok := call.Args[2].(*ast.Ident); ok && id.Name == "MessageRoleSystem" {
						t.Errorf("%s: system lines go through postSystemMessage", pos)
					}
				}
				return true
			})
		}
	}
	if steps < 8 || systems < 4 {
		t.Fatalf("found %d emitStep and %d postSystemMessage calls; the scan is broken", steps, systems)
	}
}

func checkSystemCode(t *testing.T, src parsedSource, fn *ast.FuncDecl, arg ast.Expr, pos token.Position) {
	t.Helper()
	if code, ok := stringLiteral(arg); ok {
		if _, known := messageTexts[code]; !known {
			t.Errorf("%s: postSystemMessage code %q is not in messageTexts", pos, code)
		}
		return
	}
	id, ok := arg.(*ast.Ident)
	if !ok {
		t.Errorf("%s: postSystemMessage code must be a literal or a local variable", pos)
		return
	}
	ast.Inspect(fn.Body, func(n ast.Node) bool {
		as, ok := n.(*ast.AssignStmt)
		if !ok || len(as.Lhs) != 1 || len(as.Rhs) != 1 {
			return true
		}
		if lhs, ok := as.Lhs[0].(*ast.Ident); !ok || lhs.Name != id.Name {
			return true
		}
		at := src.fset.Position(as.Pos())
		if as.Tok == token.ADD_ASSIGN {
			if suffix, ok := as.Rhs[0].(*ast.Ident); !ok || suffix.Name != "draftCodeSuffix" {
				t.Errorf("%s: %s may only be extended by draftCodeSuffix", at, id.Name)
			}
			return true
		}
		if s, ok := stringLiteral(as.Rhs[0]); ok {
			if _, known := messageTexts[s]; !known {
				t.Errorf("%s: %s = %q is not in messageTexts", at, id.Name, s)
			}
			return true
		}
		if sel, ok := as.Rhs[0].(*ast.SelectorExpr); ok && sel.Sel.Name == "msg" {
			return true // a jobEnding's code, checked by TestTextCodeTables
		}
		t.Errorf("%s: %s is set to something other than a message code", at, id.Name)
		return true
	})
}

// TestJobStepsCarryTheirCodeAndVars runs a whole job: every job_step names a
// code of the table, carries flat vars and its text is the code's template
// filled with them.
func TestJobStepsCarryTheirCodeAndVars(t *testing.T) {
	env, runner, p := newJobEnv(t)
	scriptSuccess(t, env, runner)
	runner.on(PhaseCoding, func(ctx context.Context, run JobRun) error {
		if _, err := env.svc.WriteJobFile(ctx, run.Job.ID, "src/app/extra.cpp", "int x;\n"); err != nil {
			return err
		}
		if err := env.svc.DeleteJobFile(ctx, run.Job.ID, "src/app/extra.cpp"); err != nil {
			return err
		}
		return env.svc.FinishJob(ctx, run.Job.ID, "Done.")
	})
	ctx := context.Background()
	events, cancel := env.svc.Subscribe(p.ID)
	defer cancel()
	job, err := env.svc.StartJob(ctx, p.ID, StartJobInput{Mode: JobModeNew, Prompt: "x"})
	if err != nil {
		t.Fatal(err)
	}
	waitJob(t, env, job.ID, JobAwaitingConfirmation)
	if _, err := env.svc.ConfirmJob(ctx, job.ID, ""); err != nil {
		t.Fatal(err)
	}
	waitJob(t, env, job.ID, JobSucceeded)
	waitIdle(t, env)
	var codes []string
	for _, ev := range drain(events) {
		if ev.Type != EventJobStep {
			continue
		}
		var d struct {
			JobID string         `json:"job_id"`
			Kind  string         `json:"kind"`
			Code  string         `json:"code"`
			Vars  map[string]any `json:"vars"`
			Text  string         `json:"text"`
		}
		dec := json.NewDecoder(strings.NewReader(string(ev.Data)))
		dec.UseNumber()
		if err := dec.Decode(&d); err != nil {
			t.Fatal(err)
		}
		template, ok := stepTexts[d.Code]
		if !ok || d.JobID != job.ID || d.Vars == nil {
			t.Fatalf("step = %s", ev.Data)
		}
		if want := strings.Join(strings.Fields(renderText(template, d.Vars)), " "); d.Text != want {
			t.Fatalf("step text %q, want %q", d.Text, want)
		}
		codes = append(codes, d.Code+"/"+d.Kind)
	}
	want := []string{"plan_submitted/info", "layout_updated/layout", "file_written/file", "file_deleted/file", "build_succeeded_flash/build"}
	if !slices.Equal(codes, want) {
		t.Fatalf("step codes = %v, want %v", codes, want)
	}
}

// TestSystemMessagesRoundTripCodeAndVars stores a system line: the event, the
// list and the table carry code and vars; user and agent lines carry none; an
// unknown code posts nothing.
func TestSystemMessagesRoundTripCodeAndVars(t *testing.T) {
	env, p := newProjectEnv(t)
	ctx := context.Background()
	events, cancel := env.svc.Subscribe(p.ID)
	defer cancel()
	env.svc.postSystemMessage(p.ID, "ccj_1", "job_timeout", map[string]any{"limit": "30s", "Bad": "x"})
	env.svc.postSystemMessage(p.ID, "ccj_1", "no_such_code", nil)
	env.svc.postMessage(p.ID, "ccj_1", MessageRoleAgent, "Done.")
	if _, err := env.svc.AddUserMessage(ctx, p.ID, "ccj_1", "Hi", nil); err != nil {
		t.Fatal(err)
	}
	evs := collectEvents(t, events, 3, 2*time.Second, EventJobMessage)
	var first struct{ Message map[string]any }
	_ = json.Unmarshal(evs[0].Data, &first)
	if first.Message["code"] != "job_timeout" || first.Message["text"] != "The job exceeded its time limit of 30s. The project is unchanged." {
		t.Fatalf("job_message = %s", evs[0].Data)
	}
	if vars, _ := first.Message["vars"].(map[string]any); len(vars) != 1 || vars["limit"] != "30s" {
		t.Fatalf("job_message vars = %s", evs[0].Data)
	}
	for _, ev := range evs[1:] {
		if strings.Contains(string(ev.Data), `"code"`) || strings.Contains(string(ev.Data), `"vars"`) {
			t.Fatalf("a user or agent line carries a code: %s", ev.Data)
		}
	}
	list, err := env.svc.ListMessages(ctx, p.ID)
	if err != nil || len(list) != 3 {
		t.Fatalf("messages = %+v, %v", list, err)
	}
	if m := list[0]; m.Role != MessageRoleSystem || m.Code != "job_timeout" || len(m.Vars) != 1 || m.Vars["limit"] != "30s" || m.Text != first.Message["text"] {
		t.Fatalf("listed system message = %+v", m)
	}
	if list[1].Code != "" || list[1].Vars != nil || list[2].Code != "" || list[2].Vars != nil {
		t.Fatalf("listed agent/user messages = %+v", list[1:])
	}
	var code, vars string
	if err := env.svc.db.QueryRow(`SELECT code, vars FROM cc_messages WHERE id=?`, list[0].ID).Scan(&code, &vars); err != nil || code != "job_timeout" || vars != `{"limit":"30s"}` {
		t.Fatalf("stored code %q vars %q, %v", code, vars, err)
	}
	if err := env.svc.db.QueryRow(`SELECT code, vars FROM cc_messages WHERE id=?`, list[1].ID).Scan(&code, &vars); err != nil || code != "" || vars != "" {
		t.Fatalf("stored agent code %q vars %q, %v", code, vars, err)
	}
	// Numbers survive the database exactly (no float formatting).
	env.svc.postSystemMessage(p.ID, "ccj_1", "job_failed", map[string]any{"error_code": CodeBuildFailed, "error": "x", "n": 3})
	list, _ = env.svc.ListMessages(ctx, p.ID)
	data, _ := json.Marshal(list[len(list)-1].Vars)
	if string(data) != `{"error":"x","error_code":"CYD_BUILD_FAILED","n":3}` {
		t.Fatalf("listed vars = %s", data)
	}
}

// TestJobEndingsPostTheirCode: a cancelled job with work left posts the
// cancelled line with the draft variant; a waiting job cancelled without
// work posts the plain one.
func TestJobEndingsPostTheirCode(t *testing.T) {
	env, runner, p := newJobEnv(t)
	started := make(chan struct{})
	runner.on(PhaseCoding, func(ctx context.Context, run JobRun) error {
		if _, err := env.svc.WriteJobFile(ctx, run.Job.ID, "src/app/extra.cpp", "int y;\n"); err != nil {
			return err
		}
		close(started)
		<-ctx.Done()
		return ctx.Err()
	})
	ctx := context.Background()
	job, _ := env.svc.StartJob(ctx, p.ID, StartJobInput{Mode: JobModeChange, Prompt: "x"})
	<-started
	if err := env.svc.CancelJob(ctx, job.ID); err != nil {
		t.Fatal(err)
	}
	waitJob(t, env, job.ID, JobCancelled)
	waitIdle(t, env)
	msgs, _ := env.svc.ListMessages(ctx, p.ID)
	last := msgs[len(msgs)-1]
	if last.Role != MessageRoleSystem || last.Code != "job_cancelled_draft" || last.Text != messageTexts["job_cancelled_draft"] {
		t.Fatalf("cancel line = %+v", last)
	}

	env2, runner2, p2 := newJobEnv(t)
	scriptSuccess(t, env2, runner2)
	waiting, _ := env2.svc.StartJob(ctx, p2.ID, StartJobInput{Mode: JobModeNew, Prompt: "x"})
	waitJob(t, env2, waiting.ID, JobAwaitingConfirmation)
	if err := env2.svc.CancelJob(ctx, waiting.ID); err != nil {
		t.Fatal(err)
	}
	msgs, _ = env2.svc.ListMessages(ctx, p2.ID)
	if last := msgs[len(msgs)-1]; last.Code != "job_cancelled" || last.Text != "The job was cancelled. The project is unchanged." {
		t.Fatalf("cancel line = %+v", last)
	}
}
