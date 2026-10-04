package flows

import (
	"reflect"
	"slices"
	"sort"
	"strings"
	"testing"
	"time"
)

// Hardening tests of the starter templates. templates_builtin_test.go holds the plan's
// tests; the helpers here start with tpl.

func tplIdentity(key string) string { return key }

func tplBuild(t *testing.T, id string, tr func(string) string) *Flow {
	t.Helper()
	f, err := TemplateFlow(id, tr)
	if err != nil {
		t.Fatalf("%s: %v", id, err)
	}
	return f
}

// tplTrigger returns the one trigger node of a template flow.
func tplTrigger(t *testing.T, reg *Registry, f *Flow) *Node {
	t.Helper()
	var found *Node
	for i := range f.Nodes {
		if def, ok := reg.Lookup(f.Nodes[i].Type); ok && def.Trigger {
			if found != nil {
				t.Fatalf("%s has more than one trigger", f.Name)
			}
			found = &f.Nodes[i]
		}
	}
	if found == nil {
		t.Fatalf("%s has no trigger", f.Name)
	}
	return found
}

// tplExpressions lists "<node key>.<param>: <expression>" for every template expression
// in the parameters of f, in document order, and fails on a template that does not parse.
func tplExpressions(t *testing.T, f *Flow) []string {
	t.Helper()
	var out []string
	for i := range f.Nodes {
		n := &f.Nodes[i]
		refs, problems := CollectTemplateRefs(n.Params)
		if len(problems) > 0 {
			t.Errorf("%s.%s: template problems %v", f.Name, n.Key, problems)
		}
		for _, ref := range refs {
			out = append(out, n.Key+"."+ref.Param+": "+ref.Expr.Source)
		}
	}
	return out
}

// tplRoots are the roots a run offers to the parameters of a template flow before any
// node but the trigger ran: the trigger sample and the run. Other nodes are absent, so
// their expressions read nothing, which is enough to see whether a text resolves.
func tplRoots(t *testing.T, reg *Registry, f *Flow) *Env {
	t.Helper()
	return &Env{Location: time.UTC, Roots: map[string]any{
		"trigger": map[string]any{"data": TriggerSample(tplTrigger(t, reg, f))},
		"run":     map[string]any{"started_at": "2026-10-03T07:00:00Z"},
	}}
}

// A translation is plain text. One that holds template syntax must neither add an
// expression to a parameter nor break the template: the escape of the template syntax
// renders it as the text it is. The expressions of a template flow are the ones written in
// templates_builtin.go, whatever the translator returns.
func TestTemplateTextsCannotInjectExpressions(t *testing.T) {
	reg := catalogRegistry(t, fullEnv())
	variants := []string{
		"a {{ b",
		"{{trigger.data.raw}}",
		"{{ search.results }} and {{",
		"{{{ x }}}",
		"{{{{",
		`\{{ x`,
		`\\{{ x`,
		`ends with a backslash\`,
		"}} {{ }}",
		"{{ run.started_at | date(\"DD.MM.YYYY\") }}",
	}
	for _, info := range Templates() {
		baseline := tplExpressions(t, tplBuild(t, info.ID, tplIdentity))
		for _, variant := range variants {
			tr := func(key string) string {
				if strings.Contains(key, ".text_") || strings.HasSuffix(key, ".name") || strings.HasSuffix(key, ".description") {
					return variant
				}
				return key
			}
			f := tplBuild(t, info.ID, tr)
			if got := tplExpressions(t, f); !reflect.DeepEqual(got, baseline) {
				t.Errorf("%s with %q: expressions %v, want %v", info.ID, variant, got, baseline)
			}
			// Name and description are not parameters: they stay exactly as translated.
			if f.Name != variant || f.Description != variant {
				t.Errorf("%s with %q: name %q, description %q", info.ID, variant, f.Name, f.Description)
			}
			vc := ValidateContext{Mode: ModeDraft, Now: time.Date(2026, 10, 3, 7, 0, 0, 0, time.UTC), Location: time.UTC}
			for _, is := range Validate(f, reg, vc) {
				if is.Severity == SeverityError || is.Code == IssueTemplateSyntax {
					t.Errorf("%s with %q: %+v", info.ID, variant, is)
				}
			}
			// Every text must resolve to what the translator gave, whatever it holds.
			env := tplRoots(t, reg, f)
			var resolved []string
			for i := range f.Nodes {
				params, err := ResolveParams(f.Nodes[i].Params, env)
				if err != nil {
					t.Fatalf("%s.%s with %q: %v", info.ID, f.Nodes[i].Key, variant, err)
				}
				for _, v := range params {
					if s, ok := v.(string); ok {
						resolved = append(resolved, s)
					}
				}
			}
			all := strings.Join(resolved, "\x00")
			if len(info.textKeys) > 0 && !strings.Contains(all, variant) {
				t.Errorf("%s with %q: the text did not come through, resolved %q", info.ID, variant, resolved)
			}
		}
	}
}

// literalTemplateText is the one place that escapes, so pin what it does: every "{{" gets
// the escape, and the template parser turns the result back into the original text.
func TestLiteralTemplateTextRoundTrips(t *testing.T) {
	for _, s := range []string{"", "plain", "a {{ b", "{{", "{{{", "{{{{", `\{{`, `\\{{`, `a\`, "}}", "{{x}} {{y}}", "ä {{ ö", `\n {{`} {
		escaped := literalTemplateText(s)
		if !strings.Contains(s, "{{") && escaped != s {
			t.Errorf("%q changed to %q although it holds no {{", s, escaped)
		}
		if HasTemplate(escaped) {
			t.Errorf("%q: the escaped text %q still starts an expression", s, escaped)
		}
		tpl, err := ParseTemplate(escaped)
		if err != nil {
			t.Errorf("%q: %v", s, err)
			continue
		}
		got, err := tpl.Evaluate(&Env{})
		if err != nil || (s != "" && got != s) || (s == "" && got != "") {
			t.Errorf("%q renders as %#v, %v", s, got, err)
		}
	}
}

// Every expression of a template must read a field the node before it outputs. The
// validator does not check this for triggers (TEMPLATE_UNKNOWN_FIELD skips them), for
// run and flow, nor for the key inside a list (pluck); and even where it checks, an unknown
// field is only a warning. So the references are checked here against what the nodes
// declare: the declared fields of the referenced node (FieldsOf, with the template's own
// parameters, which matters for ai.step), the trigger sample for trigger.data, and the
// roots the engine builds for run and flow (engine_state.go, runState.env).
func TestTemplateReferencesNameRealFields(t *testing.T) {
	reg := catalogRegistry(t, fullEnv())
	known := map[string][]string{
		"run":  {"id", "started_at", "mode", "revision"},
		"flow": {"id", "name"},
	}
	triggerOutput := []string{"data", "fired_at", "type", "node"}
	checked := 0
	for _, info := range Templates() {
		f := tplBuild(t, info.ID, tplIdentity)
		for i := range f.Nodes {
			n := &f.Nodes[i]
			refs, _ := CollectTemplateRefs(n.Params)
			for _, ref := range refs {
				e := ref.Expr
				where := info.ID + " " + n.Key + "." + ref.Param + " {{" + e.Source + "}}"
				if len(e.Path) == 0 || e.Path[0].IsIndex {
					t.Errorf("%s: names no field", where)
					continue
				}
				checked++
				first := e.Path[0].Field
				var src *Node
				switch e.Root {
				case "trigger":
					src = tplTrigger(t, reg, f)
				case "run", "flow":
					if !slices.Contains(known[e.Root], first) {
						t.Errorf("%s: %s has no field %q (it has %v)", where, e.Root, first, known[e.Root])
					}
					continue
				default:
					if src = f.NodeByKey(e.Root); src == nil {
						t.Errorf("%s: no node %q", where, e.Root)
						continue
					}
				}
				def := lookupDef(t, reg, src.Type)
				if def.Trigger {
					if !slices.Contains(triggerOutput, first) {
						t.Errorf("%s: a trigger outputs %v, not %q", where, triggerOutput, first)
					}
					if first == "data" && len(e.Path) > 1 && !e.Path[1].IsIndex {
						if _, ok := TriggerSample(src)[e.Path[1].Field]; !ok {
							t.Errorf("%s: the %s sample has no field %q (it has %v)", where, src.Type, e.Path[1].Field, tplSortedKeys(TriggerSample(src)))
						}
					}
					continue
				}
				var names []string
				for _, field := range def.FieldsOf(src) {
					names = append(names, field.Name)
				}
				if !slices.Contains(names, first) {
					t.Errorf("%s: %s (%s) outputs %v, not %q", where, src.Key, src.Type, names, first)
				}
			}
		}
	}
	// A guard against an empty walk: the six templates have nine expressions.
	if checked < 9 {
		t.Fatalf("only %d references checked", checked)
	}
}

func tplSortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// The validator reports none of the reference problems for the templates, in either mode:
// no unknown node, no node that does not run before, no unknown field, no syntax error,
// and nothing is unreachable. Only the blanks the user has to fill (PARAM_REQUIRED) are open.
func TestTemplatesHaveNoReferenceWarnings(t *testing.T) {
	reg := catalogRegistry(t, fullEnv())
	for _, info := range Templates() {
		for _, mode := range []ValidationMode{ModeDraft, ModePublish} {
			f := tplBuild(t, info.ID, tplIdentity)
			vc := ValidateContext{Mode: mode, Now: time.Date(2026, 10, 3, 7, 0, 0, 0, time.UTC), Location: time.UTC}
			for _, is := range Validate(f, reg, vc) {
				if is.Code != IssueParamRequired {
					t.Errorf("%s mode %d: %+v", info.ID, mode, is)
				}
			}
		}
	}
}

// A node label is a translation, and TemplateFlow builds its key by a scheme: the editor
// shows a template node with the label of its type, so the key must be the key of the
// definition, whatever the type.
func TestTemplateNodeLabelsUseTheDefinitionKeys(t *testing.T) {
	reg := catalogRegistry(t, fullEnv())
	for _, info := range Templates() {
		f := tplBuild(t, info.ID, func(key string) string { return "L:" + key })
		for _, n := range f.Nodes {
			def := lookupDef(t, reg, n.Type)
			if def.LabelKey == "" {
				t.Fatalf("%s has no label key", n.Type)
			}
			if want := "L:" + def.LabelKey; n.Label != want {
				t.Errorf("%s node %s (%s): label %q, want %q", info.ID, n.Key, n.Type, n.Label, want)
			}
		}
	}
}

// The keys a template asks its translator for are the keys CatalogI18nKeys lists, and no
// others: a name, a description, text_1 to text_N as TemplateInfo declares, and the labels
// of its node types. A text the template uses but TemplateInfo does not declare would be
// missing from the translation files that plan 1c checks.
func TestTemplateKeysMatchTheCatalogList(t *testing.T) {
	reg := catalogRegistry(t, fullEnv())
	listed := map[string]bool{}
	for _, k := range CatalogI18nKeys(reg) {
		listed[k] = true
	}
	for _, info := range Templates() {
		// TemplateFlow translates the texts of every template to find the one it builds, so
		// the translator is asked for the texts of the others too; only this template's
		// own keys and the labels of its nodes count.
		asked := map[string]bool{}
		f := tplBuild(t, info.ID, func(key string) string {
			if strings.HasPrefix(key, templateKey(info.ID, "")) || strings.HasPrefix(key, "easydrag.node.") {
				asked[key] = true
			}
			return key
		})
		want := map[string]bool{info.NameKey: true, info.DescriptionKey: true}
		for _, k := range info.textKeys {
			want[k] = true
		}
		for _, n := range f.Nodes {
			want[lookupDef(t, reg, n.Type).LabelKey] = true
		}
		if !reflect.DeepEqual(asked, want) {
			t.Errorf("%s asks for %v, want %v", info.ID, tplSortedKeys(asked), tplSortedKeys(want))
		}
		for k := range asked {
			if !listed[k] {
				t.Errorf("%s: the key %s is not in CatalogI18nKeys", info.ID, k)
			}
		}
	}
}

// Taint lint of the starter templates, with the expectation spelled out. The lint warns
// when untrusted data (a webhook, a search, a page, a mail, a planner entry, a model
// answer) reaches a sink parameter (a recipient, a path, an address, an entity, a
// channel). None of the six templates does that as they are shipped: untrusted text goes
// into prompts, messages and documents, which are not sinks, and the only parameter that
// receives a derived value, the Telegram attachment of the AI news template, receives the
// path of the PDF, which does not depend on the text of the PDF (OutputIndependent).
func TestTemplateTaintLint(t *testing.T) {
	reg := catalogRegistry(t, fullEnv())
	for _, info := range Templates() {
		f := tplBuild(t, info.ID, tplIdentity)
		if issues := LintUntrustedData(f, reg); len(issues) != 0 {
			t.Errorf("%s: unexpected lint warnings %+v", info.ID, issues)
		}
	}
}

// The silence above must not be blindness. Each case routes a value into a sink of the same
// template and lists the parameters ("node.param") the lint must warn about. The plain path of
// the AI news template (the PDF's file, as shipped) is quiet, a model answer used as the
// attachment is not, taint goes on through the PDF when the name of the file is tainted,
// and the trigger of the budget template is trusted.
func TestTemplateTaintLintSeesTheFlows(t *testing.T) {
	reg := catalogRegistry(t, fullEnv())
	for _, tc := range []struct {
		template, node, param, value string
		warns                        []string
	}{
		// The summary is a model answer over web data: untrusted.
		{"ai_news_pdf_telegram", "telegram", "file", "{{summary.text}}", []string{"telegram.file"}},
		// A tainted file name makes the PDF's path tainted, which reaches the attachment.
		{"ai_news_pdf_telegram", "pdf", "filename", "{{search.results | first}}", []string{"pdf.filename", "telegram.file"}},
		// What the title and the content of a PDF hold does not decide where the file is.
		{"ai_news_pdf_telegram", "telegram", "file", "{{pdf.web_path}}", nil},
		// Webhook data and the answer a model gave about it are untrusted.
		{"webhook_summary_email", "mail", "to", "{{trigger.data.payload.email}}", []string{"mail.to"}},
		{"webhook_summary_email", "mail", "to", "{{summary.text}}", []string{"mail.to"}},
		{"rss_digest", "mail", "to", "{{feed.items | first}}", []string{"mail.to"}},
		// Planner entries can be written by calendar sync and by the agent.
		{"appointment_reminder", "telegram", "file", "{{trigger.data.title}}", []string{"telegram.file"}},
		// The budget trigger is trusted.
		{"budget_guard", "push", "channel", "{{trigger.data.event}}", nil},
	} {
		f := tplBuild(t, tc.template, tplIdentity)
		f.NodeByKey(tc.node).Params[tc.param] = tc.value
		var got []string
		for _, is := range LintUntrustedData(f, reg) {
			if is.Code != IssueUntrustedData {
				t.Errorf("%s %s.%s: unexpected issue %+v", tc.template, tc.node, tc.param, is)
			}
			got = append(got, f.NodeByID(is.NodeID).Key+"."+is.Param)
		}
		sort.Strings(got)
		if !slices.Equal(got, tc.warns) {
			t.Errorf("%s %s.%s = %s: warnings %v, want %v", tc.template, tc.node, tc.param, tc.value, got, tc.warns)
		}
	}
}

// The AI news template must lint clean (plan 1b-06): the text of the PDF is not what the
// attachment of the Telegram message is.
func TestAINewsTemplateHasNoTaintWarnings(t *testing.T) {
	reg := catalogRegistry(t, fullEnv())
	f := tplBuild(t, "ai_news_pdf_telegram", tplIdentity)
	if got := f.NodeByKey("telegram").Params["file"]; got != "{{pdf.file}}" {
		t.Fatalf("setup: the attachment is %v", got)
	}
	if issues := LintUntrustedData(f, reg); len(issues) != 0 {
		t.Fatalf("lint warnings: %+v", issues)
	}
}

// CatalogI18nKeys lists every key DescribeNodeTypes asks the translator for, generic tool
// nodes excepted (they carry literal text and ask for none).
func TestCatalogI18nKeysCoverDescribeNodeTypes(t *testing.T) {
	reg := catalogRegistry(t, fullEnv())
	RefreshGenericTools(reg, []GenericTool{{Name: "docker", Category: "infrastructure", Schema: sampleToolSchema()}}, StaticEnv{})
	listed := map[string]bool{}
	for _, k := range CatalogI18nKeys(reg) {
		listed[k] = true
	}
	asked := catalogKeysRequested(reg)
	if len(asked) < 100 {
		t.Fatalf("the catalog asks for only %d keys", len(asked))
	}
	for k := range asked {
		if !listed[k] {
			t.Errorf("DescribeNodeTypes asks for %s, CatalogI18nKeys does not list it", k)
		}
	}
}

// Without a registry the list holds what does not depend on one.
func TestCatalogI18nKeysWithoutRegistry(t *testing.T) {
	keys := CatalogI18nKeys(nil)
	set := map[string]bool{}
	for _, k := range keys {
		set[k] = true
	}
	for _, want := range []string{"easydrag.category.trigger", "easydrag.template.rss_digest.name", "easydrag.template.rss_digest.text_2"} {
		if !set[want] {
			t.Errorf("missing %s", want)
		}
	}
	if set["easydrag.node.web_search.label"] {
		t.Error("node keys come from the registry")
	}
}

// tplModeFieldKeys collects the description keys of the output fields a definition with
// an OutputFieldsFunc declares for each value of each parameter that has literal options:
// the sample node is the defaults plus that one option. A hook that panics skips its sample.
func tplModeFieldKeys(reg *Registry) map[string]bool {
	keys := map[string]bool{}
	for _, def := range reg.All() {
		if isGenericDef(def) || def.OutputFieldsFunc == nil {
			continue
		}
		var visit func(params []ParamSpec)
		visit = func(params []ParamSpec) {
			for _, p := range params {
				for _, o := range p.Options {
					n := sampleNode(def)
					n.Params[p.Name] = o.Value
					var fields []FieldSpec
					if catchPanic(func() { fields = def.FieldsOf(n) }) != nil {
						continue
					}
					for _, f := range fields {
						if f.DescriptionKey != "" {
							keys[f.DescriptionKey] = true
						}
					}
				}
				visit(p.Fields)
			}
		}
		visit(def.Params)
	}
	return keys
}

// The fields of ai.step and logic.merge depend on a parameter (output_mode, mode), and a
// field of such a node could carry a description key that only exists in one mode.
// CatalogI18nKeys lists the keys of the static OutputFields only; DescribeNodeTypes asks
// for those of the default mode. Today no definition gives a dynamic field a description
// key (the fields of ai.step and logic.merge have none), so there is nothing to add and this
// test passes. It fails the day one does, which is when CatalogI18nKeys has to walk the
// modes too (and the translation files get the key).
func TestCatalogI18nKeysCoverModeDependentFields(t *testing.T) {
	// The collector finds a key where there is one, in a registry that has such a definition.
	probe := NewRegistry()
	probe.MustRegister(&NodeDef{Type: "test.modes", Category: "test",
		Params: []ParamSpec{{Name: "mode", Kind: ParamSegmented, Default: "a", Options: []Option{{Value: "a"}, {Value: "b"}}}},
		OutputFieldsFunc: func(n *Node) []FieldSpec {
			if n.Params["mode"] == "b" {
				return []FieldSpec{{Name: "only_in_b", DescriptionKey: "k.only_in_b"}}
			}
			return nil
		}})
	probe.MustRegister(&NodeDef{Type: "test.panics", Category: "test",
		Params:           []ParamSpec{{Name: "mode", Options: []Option{{Value: "a"}}}},
		OutputFieldsFunc: func(*Node) []FieldSpec { panic("boom") }})
	if got := tplModeFieldKeys(probe); !reflect.DeepEqual(got, map[string]bool{"k.only_in_b": true}) {
		t.Fatalf("the collector found %v in the probe registry", got)
	}

	reg := catalogRegistry(t, fullEnv())
	listed := map[string]bool{}
	for _, k := range CatalogI18nKeys(reg) {
		listed[k] = true
	}
	for k := range tplModeFieldKeys(reg) {
		if !listed[k] {
			t.Errorf("a mode-dependent field asks for %s, CatalogI18nKeys does not list it", k)
		}
	}
}
