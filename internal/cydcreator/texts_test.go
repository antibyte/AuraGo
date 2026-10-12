package cydcreator

import (
	"bytes"
	"context"
	"encoding/json"
	"go/ast"
	"go/parser"
	"go/token"
	"log/slog"
	"math"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"
)

var textPlaceholderShape = regexp.MustCompile(`\{\{[^}]*\}\}`)

// placeholdersOf returns the placeholder names of a template, sorted.
func placeholdersOf(template string) []string {
	var names []string
	for _, m := range textPlaceholder.FindAllStringSubmatch(template, -1) {
		if !slices.Contains(names, m[1]) {
			names = append(names, m[1])
		}
	}
	slices.Sort(names)
	return names
}

// TestTextCodeTables pins the shape of the code tables the client translates:
// identifier codes, well-formed placeholders, no "(s)" plurals and no Go
// duration placeholders, and for every job ending a draft variant that adds
// exactly the draft sentence and gets the vars the ending's literal sets.
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
			for _, ph := range placeholdersOf(text) {
				if ph == "delay" || ph == "duration" || ph == "limit" {
					t.Errorf("%s.%s has the Go duration placeholder %s", name, code, ph)
				}
			}
		}
	}
	endings := jobEndingSites(t)
	if len(endings) < 7 {
		t.Fatalf("found %d jobEnding literals; the scan is broken", len(endings))
	}
	for _, end := range endings {
		base, ok := messageTexts[end.code]
		if !ok {
			t.Errorf("%s: job ending code %q has no message text", end.pos, end.code)
			continue
		}
		if draft := messageTexts[end.code+draftCodeSuffix]; draft != base+draftSentence {
			t.Errorf("draft variant of %q = %q, want the base text plus the draft sentence", end.code, draft)
		}
		for _, ph := range placeholdersOf(base) {
			if !slices.Contains(end.keys, ph) {
				t.Errorf("%s: jobEnding %q sets no var %q", end.pos, end.code, ph)
			}
		}
	}
	keys := TextKeys()
	if len(keys) != len(stepTexts)+len(messageTexts) || keys["cyd_creator.step.file_written"] != stepTexts["file_written"] ||
		keys["cyd_creator.msg.job_cancelled"] != messageTexts["job_cancelled"] {
		t.Fatalf("TextKeys = %v", keys)
	}
}

func TestFillTextFillsEveryPlaceholderOnce(t *testing.T) {
	got, missing := fillText("{{a}} and {{b}}, {{a}}; {{missing}}.", map[string]any{"a": "{{b}}", "b": int64(7)})
	if got != "{{b}} and 7, {{b}}; ." || !slices.Equal(missing, []string{"missing"}) {
		t.Fatalf("fillText = %q, missing %v", got, missing)
	}
	got, missing = fillText("Build {{n}} succeeded (flash {{percent}}%), {{f}}", map[string]any{"n": json.Number("2"), "percent": 63, "f": 1.5})
	if got != "Build 2 succeeded (flash 63%), 1.5" || missing != nil {
		t.Fatalf("fillText = %q, missing %v", got, missing)
	}
}

// A placeholder without a var renders empty and is logged as a warning.
func TestRenderTextWarnsAboutAMissingVar(t *testing.T) {
	var logs bytes.Buffer
	s := &Service{logger: slog.New(slog.NewTextHandler(&logs, nil))}
	if got := s.renderText("repair_started", stepTexts["repair_started"], map[string]any{"attempt": 1}); got != "Repair 1/ started" {
		t.Fatalf("renderText = %q", got)
	}
	if out := logs.String(); !strings.Contains(out, "level=WARN") || !strings.Contains(out, "code=repair_started") || !strings.Contains(out, "missing=[max]") {
		t.Fatalf("log = %q", out)
	}
	logs.Reset()
	s.renderText("repair_started", stepTexts["repair_started"], map[string]any{"attempt": 1, "max": 3})
	if logs.Len() != 0 {
		t.Fatalf("a complete render logged %q", logs.String())
	}
}

func TestTextVarsAreSmallAndFlat(t *testing.T) {
	vars := textVars(map[string]any{
		"path":    "src/app/a.cpp",
		"message": "line one\n\tline two  ",
		"long":    strings.Repeat("ä", 400),
		"n":       3,
		"total":   int64(9),
		"small":   uint8(7),
		"big":     uint64(math.MaxUint64),
		"share":   float32(0.5),
		"ratio":   1.25,
		"num":     json.Number("12"),
		"nan":     math.NaN(),
		"Bad-Key": "x",
		"list":    []string{"x"},
		"flag":    true,
	}, 160)
	want := map[string]any{"path": "src/app/a.cpp", "message": "line one line two", "long": strings.Repeat("ä", 159) + "…", "n": int64(3),
		"total": int64(9), "small": int64(7), "big": json.Number("18446744073709551615"), "share": 0.5, "ratio": 1.25, "num": json.Number("12")}
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

// TestEveryCodeRendersItsEnglishText emits every step and every message code
// with the vars its call site passes and checks the English text; no
// placeholder stays without a var.
func TestEveryCodeRendersItsEnglishText(t *testing.T) {
	env, p := newProjectEnv(t)
	steps := []struct {
		code string
		vars map[string]any
		want string
	}{
		{"plan_submitted", map[string]any{"screens": 2, "widgets": 5, "data_sources": 1}, "Plan submitted · screens: 2 · widgets: 5 · data sources: 1"},
		{"layout_updated", map[string]any{"count": 2, "changes": "added label lbl_a to home; moved button btn_b to 10,20"}, "Layout changes (2): added label lbl_a to home; moved button btn_b to 10,20"},
		{"file_written", map[string]any{"path": "src/app/a.cpp", "lines": 3}, "Wrote src/app/a.cpp (lines: 3)"},
		{"file_deleted", map[string]any{"path": "src/app/a.cpp"}, "Deleted src/app/a.cpp"},
		{"repair_started", map[string]any{"attempt": 1, "max": 3}, "Repair 1/3 started"},
		{"build_succeeded", map[string]any{"n": 1}, "Build 1 succeeded"},
		{"build_succeeded_flash", map[string]any{"n": 2, "percent": int64(81)}, "Build 2 succeeded (flash 81%)"},
		{"build_failed_at", map[string]any{"n": 1, "location": "src/app/a.cpp:5", "message": "'x' was not declared"}, "Build 1 failed: src/app/a.cpp:5 'x' was not declared"},
		{"build_failed_error", map[string]any{"n": 2, "message": "undefined reference to `foo'"}, "Build 2 failed: undefined reference to `foo'"},
		{"build_failed", map[string]any{"n": 3, "error_code": CodeBuildTimeout, "error": CodeBuildTimeout}, "Build 3 failed: CYD_BUILD_TIMEOUT"},
		{"publish_retry", map[string]any{"milliseconds": (250 * time.Millisecond).Milliseconds()}, "A project file is open; publishing again in 250 ms."},
		{"grace_extended", map[string]any{"seconds": textSeconds(15 * time.Second)}, "Recent changes are still being built; the job gets 15 s more to finish."},
		{"grace_extended_minutes", map[string]any{"minutes": int64(12), "seconds": int64(30)}, "Recent changes are still being built; the job gets 12 min 30 s more to finish."},
	}
	covered := map[string]bool{}
	events, cancel := env.svc.Subscribe(p.ID)
	defer cancel()
	for _, tc := range steps {
		covered[tc.code] = true
		if _, missing := fillText(stepTexts[tc.code], textVars(tc.vars, maxStepVarRunes)); missing != nil {
			t.Errorf("%s: no var for %v", tc.code, missing)
		}
		env.svc.emitStep(p.ID, "ccj_1", StepInfo, tc.code, tc.vars)
		ev := collectEvents(t, events, 1, 2*time.Second, EventJobStep)[0]
		var d struct{ Code, Text string }
		_ = json.Unmarshal(ev.Data, &d)
		if d.Code != tc.code || d.Text != tc.want {
			t.Errorf("%s: text %q, want %q", tc.code, d.Text, tc.want)
		}
	}
	for code := range stepTexts {
		if !covered[code] {
			t.Errorf("step code %s has no text case", code)
		}
	}
	messages := []struct {
		code string
		vars map[string]any
		want string
	}{
		{"job_timeout", map[string]any{"seconds": int64(30)}, "The job exceeded its time limit of 30 s. The project is unchanged."},
		{"job_timeout_minutes_draft", map[string]any{"minutes": int64(18), "seconds": int64(37)}, "The job exceeded its time limit of 18 min 37 s. The project is unchanged." + draftSentence},
		{"job_failed", map[string]any{"error_code": CodeBuildFailed, "error": "The build still fails after 3 repair attempts."}, "The build still fails after 3 repair attempts. The project is unchanged."},
		{"agent_stopped", map[string]any{"reason": "the provider is unreachable"}, "The agent stopped: the provider is unreachable The project is unchanged."},
	}
	for _, tc := range messages {
		if _, missing := fillText(messageTexts[tc.code], textVars(tc.vars, maxMessageVarRunes)); missing != nil {
			t.Errorf("%s: no var for %v", tc.code, missing)
		}
		env.svc.postSystemMessage(p.ID, "ccj_1", tc.code, tc.vars)
		ev := collectEvents(t, events, 1, 2*time.Second, EventJobMessage)[0]
		var d struct{ Message Message }
		_ = json.Unmarshal(ev.Data, &d)
		if d.Message.Code != tc.code || d.Message.Text != tc.want {
			t.Errorf("%s: text %q, want %q", tc.code, d.Message.Text, tc.want)
		}
	}
	// Every other message code has no placeholder: it renders as written.
	for code, text := range messageTexts {
		if len(placeholdersOf(text)) > 0 {
			continue
		}
		if got, missing := fillText(text, nil); got != text || missing != nil {
			t.Errorf("%s renders %q", code, got)
		}
	}
}

// endingSite is a jobEnding literal of the package sources: its message code
// and the names of the vars it sets.
type endingSite struct {
	code string
	keys []string
	pos  token.Position
}

// jobEndingSites returns every jobEnding literal; msg must be a string
// literal and vars a map literal or a local variable set to one.
func jobEndingSites(t *testing.T) []endingSite {
	t.Helper()
	var sites []endingSite
	forEachFunc(t, func(src parsedSource, fn *ast.FuncDecl) {
		ast.Inspect(fn.Body, func(n ast.Node) bool {
			lit, ok := n.(*ast.CompositeLit)
			if !ok {
				return true
			}
			if id, ok := lit.Type.(*ast.Ident); !ok || id.Name != "jobEnding" {
				return true
			}
			site := endingSite{pos: src.fset.Position(lit.Pos())}
			found := false
			for _, elt := range lit.Elts {
				kv, ok := elt.(*ast.KeyValueExpr)
				if !ok {
					continue
				}
				switch kv.Key.(*ast.Ident).Name {
				case "msg":
					found = true
					code, ok := stringLiteral(kv.Value)
					if !ok {
						t.Errorf("%s: jobEnding msg is not a string literal", site.pos)
					}
					site.code = code
				case "vars":
					keys, ok := mapLiteralKeys(fn, kv.Value)
					if !ok {
						t.Errorf("%s: jobEnding vars must be a map literal or a local set to one", site.pos)
					}
					site.keys = keys
				}
			}
			if !found {
				t.Errorf("%s: jobEnding without a msg code", site.pos)
			}
			sites = append(sites, site)
			return true
		})
	})
	return sites
}

// mapLiteralKeys returns the string keys of a map literal, of a local
// variable declared with one in fn (later index assignments add keys) or
// none for nil.
func mapLiteralKeys(fn *ast.FuncDecl, e ast.Expr) ([]string, bool) {
	if id, ok := e.(*ast.Ident); ok {
		if id.Name == "nil" {
			return nil, true
		}
		var keys []string
		found := false
		ast.Inspect(fn.Body, func(n ast.Node) bool {
			as, ok := n.(*ast.AssignStmt)
			if !ok || len(as.Lhs) != 1 || len(as.Rhs) != 1 {
				return true
			}
			switch lhs := as.Lhs[0].(type) {
			case *ast.Ident:
				if lhs.Name == id.Name && as.Tok == token.DEFINE {
					if k, ok := compositeKeys(as.Rhs[0]); ok {
						keys, found = append(keys, k...), true
					}
				}
			case *ast.IndexExpr:
				if x, ok := lhs.X.(*ast.Ident); ok && x.Name == id.Name {
					if k, ok := stringLiteral(lhs.Index); ok {
						keys = append(keys, k)
					}
				}
			}
			return true
		})
		return keys, found
	}
	return compositeKeys(e)
}

func compositeKeys(e ast.Expr) ([]string, bool) {
	lit, ok := e.(*ast.CompositeLit)
	if !ok {
		return nil, false
	}
	if _, ok := lit.Type.(*ast.MapType); !ok {
		return nil, false
	}
	var keys []string
	for _, elt := range lit.Elts {
		kv, ok := elt.(*ast.KeyValueExpr)
		if !ok {
			return nil, false
		}
		k, ok := stringLiteral(kv.Key)
		if !ok {
			return nil, false
		}
		keys = append(keys, k)
	}
	return keys, true
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

func forEachFunc(t *testing.T, fn func(src parsedSource, decl *ast.FuncDecl)) {
	t.Helper()
	for _, src := range packageSources(t) {
		for _, decl := range src.file.Decls {
			if f, ok := decl.(*ast.FuncDecl); ok && f.Body != nil {
				fn(src, f)
			}
		}
	}
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
// emitStep call passes a string literal code of stepTexts and a map literal
// (or nil) with a var for each placeholder of the code; every
// postSystemMessage call passes a literal code of messageTexts or a local
// variable that is only ever set to such literals (each with its draft
// variant when the variable is extended by draftCodeSuffix) or to a
// jobEnding's msg, with vars covering every candidate's placeholders;
// postMessage never writes a system line.
func TestEveryStepAndSystemLineUsesATableCode(t *testing.T) {
	steps, systems := 0, 0
	forEachFunc(t, func(src parsedSource, fn *ast.FuncDecl) {
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
					return true
				}
				checkCallVars(t, fn, pos, call.Args[4], []string{stepTexts[code]})
			case "postSystemMessage":
				systems++
				if len(call.Args) != 4 {
					t.Errorf("%s: postSystemMessage takes project, job, code, vars", pos)
					return true
				}
				if templates := systemCodeTemplates(t, src, fn, call.Args[2], pos); len(templates) > 0 {
					checkCallVars(t, fn, pos, call.Args[3], templates)
				}
			case "postMessage":
				if id, ok := call.Args[2].(*ast.Ident); ok && id.Name == "MessageRoleSystem" {
					t.Errorf("%s: system lines go through postSystemMessage", pos)
				}
			}
			return true
		})
	})
	if steps < 8 || systems < 4 {
		t.Fatalf("found %d emitStep and %d postSystemMessage calls; the scan is broken", steps, systems)
	}
}

// checkCallVars requires the vars argument to set every placeholder of every
// template the call may render.
func checkCallVars(t *testing.T, fn *ast.FuncDecl, pos token.Position, arg ast.Expr, templates []string) {
	t.Helper()
	keys, ok := mapLiteralKeys(fn, arg)
	if !ok {
		t.Errorf("%s: vars must be a map literal, a local set to one or nil", pos)
		return
	}
	for _, template := range templates {
		for _, ph := range placeholdersOf(template) {
			if !slices.Contains(keys, ph) {
				t.Errorf("%s: no var %q for %q", pos, ph, template)
			}
		}
	}
}

// systemCodeTemplates returns the templates a postSystemMessage code argument
// can name. A jobEnding's msg returns none (TestTextCodeTables checks those
// literals and their vars).
func systemCodeTemplates(t *testing.T, src parsedSource, fn *ast.FuncDecl, arg ast.Expr, pos token.Position) []string {
	t.Helper()
	if code, ok := stringLiteral(arg); ok {
		text, known := messageTexts[code]
		if !known {
			t.Errorf("%s: postSystemMessage code %q is not in messageTexts", pos, code)
			return nil
		}
		return []string{text}
	}
	id, ok := arg.(*ast.Ident)
	if !ok {
		t.Errorf("%s: postSystemMessage code must be a literal or a local variable", pos)
		return nil
	}
	var literals []string
	fromEnding, drafted := false, false
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
			drafted = true
			return true
		}
		if s, ok := stringLiteral(as.Rhs[0]); ok {
			if _, known := messageTexts[s]; !known {
				t.Errorf("%s: %s = %q is not in messageTexts", at, id.Name, s)
			}
			literals = append(literals, s)
			return true
		}
		if sel, ok := as.Rhs[0].(*ast.SelectorExpr); ok && sel.Sel.Name == "msg" {
			fromEnding = true
			return true
		}
		t.Errorf("%s: %s is set to something other than a message code", at, id.Name)
		return true
	})
	if fromEnding {
		return nil
	}
	var templates []string
	for _, code := range literals {
		templates = append(templates, messageTexts[code])
		if drafted {
			draft, known := messageTexts[code+draftCodeSuffix]
			if !known {
				t.Errorf("%s: %s can be %q, which has no draft variant", pos, id.Name, code+draftCodeSuffix)
				continue
			}
			templates = append(templates, draft)
		}
	}
	return templates
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
		filled, missing := fillText(template, d.Vars)
		if want := strings.Join(strings.Fields(filled), " "); d.Text != want || missing != nil {
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
	env.svc.postSystemMessage(p.ID, "ccj_1", "agent_stopped", map[string]any{"reason": "the model gave up", "Bad": "x"})
	env.svc.postSystemMessage(p.ID, "ccj_1", "no_such_code", nil)
	env.svc.postMessage(p.ID, "ccj_1", MessageRoleAgent, "Done.")
	if _, err := env.svc.AddUserMessage(ctx, p.ID, "ccj_1", "Hi", nil); err != nil {
		t.Fatal(err)
	}
	evs := collectEvents(t, events, 3, 2*time.Second, EventJobMessage)
	var first struct{ Message map[string]any }
	_ = json.Unmarshal(evs[0].Data, &first)
	if first.Message["code"] != "agent_stopped" || first.Message["text"] != "The agent stopped: the model gave up The project is unchanged." {
		t.Fatalf("job_message = %s", evs[0].Data)
	}
	if vars, _ := first.Message["vars"].(map[string]any); len(vars) != 1 || vars["reason"] != "the model gave up" {
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
	if m := list[0]; m.Role != MessageRoleSystem || m.Code != "agent_stopped" || len(m.Vars) != 1 || m.Vars["reason"] != "the model gave up" || m.Text != first.Message["text"] {
		t.Fatalf("listed system message = %+v", m)
	}
	if list[1].Code != "" || list[1].Vars != nil || list[2].Code != "" || list[2].Vars != nil {
		t.Fatalf("listed agent/user messages = %+v", list[1:])
	}
	var code, vars string
	if err := env.svc.db.QueryRow(`SELECT code, vars FROM cc_messages WHERE id=?`, list[0].ID).Scan(&code, &vars); err != nil || code != "agent_stopped" || vars != `{"reason":"the model gave up"}` {
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
