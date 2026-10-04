package flows

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"log/slog"
	"reflect"
	"slices"
	"strings"
	"sync"
	"testing"
)

// catDescLogs collects the records of the default logger while a test runs.
type catDescLogs struct {
	mu   sync.Mutex
	recs []slog.Record
}

func (h *catDescLogs) Enabled(context.Context, slog.Level) bool { return true }
func (h *catDescLogs) WithAttrs([]slog.Attr) slog.Handler       { return h }
func (h *catDescLogs) WithGroup(string) slog.Handler            { return h }
func (h *catDescLogs) Handle(_ context.Context, r slog.Record) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.recs = append(h.recs, r.Clone())
	return nil
}

// fallbacks returns "type/hook" for every record of a hook that was replaced by a fallback.
func (h *catDescLogs) fallbacks() []string {
	h.mu.Lock()
	defer h.mu.Unlock()
	var out []string
	for _, r := range h.recs {
		if !strings.Contains(r.Message, "using a fallback") {
			continue
		}
		var typ, hook string
		r.Attrs(func(a slog.Attr) bool {
			switch a.Key {
			case "type":
				typ = a.Value.String()
			case "hook":
				hook = a.Value.String()
			}
			return true
		})
		out = append(out, typ+"/"+hook)
	}
	return out
}

func (h *catDescLogs) attr(i int, key string) string {
	h.mu.Lock()
	defer h.mu.Unlock()
	var val string
	h.recs[i].Attrs(func(a slog.Attr) bool {
		if a.Key == key {
			val = a.Value.String()
		}
		return true
	})
	return val
}

// catDescCaptureLogs routes the default logger to the returned collector until the test ends.
func catDescCaptureLogs(t *testing.T) *catDescLogs {
	t.Helper()
	h := &catDescLogs{}
	oldLogger, oldWriter, oldFlags := slog.Default(), log.Writer(), log.Flags()
	slog.SetDefault(slog.New(h))
	t.Cleanup(func() {
		slog.SetDefault(oldLogger)
		log.SetOutput(oldWriter)
		log.SetFlags(oldFlags)
	})
	return h
}

// catDescHookDef is a definition whose hooks all work and answer with recognisable
// values; mod breaks the ones a case is about.
func catDescHookDef(typ string, mod func(*NodeDef)) *NodeDef {
	def := &NodeDef{
		Type: typ, Category: "test", Label: typ,
		Outputs:          []string{"static"},
		OutputsFunc:      func(*Node) []string { return []string{"dyn1", "dyn2"} },
		OutputFieldsFunc: func(*Node) []FieldSpec { return []FieldSpec{{Name: "f", Type: "text", Primary: true}} },
		EffectsFunc:      func(*Node) []Effect { return []Effect{EffectSendsMessage} },
		AvailabilityFunc: func() Availability { return Availability{State: AvailableState} },
	}
	if mod != nil {
		mod(def)
	}
	return def
}

func catDescByType(infos []NodeTypeInfo) map[string]NodeTypeInfo {
	out := make(map[string]NodeTypeInfo, len(infos))
	for _, info := range infos {
		out[info.Type] = info
	}
	return out
}

// A node definition's hooks are not trusted: a hook that panics gives a fallback for
// that hook only, never a crash, and the other hooks of the definition still count.
func TestDescribeNodeTypesHookPanics(t *testing.T) {
	boom := func() { panic("hook boom") }
	healthy := func(t *testing.T, info NodeTypeInfo, skip string) {
		t.Helper()
		if skip != "availability" && info.Availability.State != AvailableState {
			t.Errorf("availability = %+v", info.Availability)
		}
		if skip != "effects" && (!reflect.DeepEqual(info.Effects, []Effect{EffectSendsMessage}) || info.Risky) {
			t.Errorf("effects = %v risky %v", info.Effects, info.Risky)
		}
		if skip != "outputs" && !reflect.DeepEqual(info.Outputs, []string{"dyn1", "dyn2"}) {
			t.Errorf("outputs = %v", info.Outputs)
		}
		if skip != "fields" && !reflect.DeepEqual(info.OutputFields, []FieldInfo{{Name: "f", Type: "text", Primary: true}}) {
			t.Errorf("fields = %+v", info.OutputFields)
		}
	}
	cases := []struct {
		name    string
		mod     func(*NodeDef)
		skip    string
		check   func(t *testing.T, info NodeTypeInfo)
		wantLog string
	}{
		{"availability panics", func(d *NodeDef) { d.AvailabilityFunc = func() Availability { boom(); return Availability{} } }, "availability",
			func(t *testing.T, info NodeTypeInfo) {
				// Unavailable and not usable: blocked, with a fixed reason that does not echo the panic.
				if info.Availability.State != BlockedState || info.Availability.Reason == "" || strings.Contains(info.Availability.Reason, "boom") {
					t.Errorf("availability = %+v", info.Availability)
				}
			}, "AvailabilityFunc"},
		{"effects panic", func(d *NodeDef) { d.EffectsFunc = func(*Node) []Effect { boom(); return nil } }, "effects",
			func(t *testing.T, info NodeTypeInfo) {
				// Fail closed: no effects are known, so the node counts as risky.
				if !info.Risky || len(info.Effects) != 0 {
					t.Errorf("risky %v, effects %v", info.Risky, info.Effects)
				}
			}, "EffectsFunc"},
		{"effects panic with an error value", func(d *NodeDef) { d.EffectsFunc = func(*Node) []Effect { panic(errors.New("an error")) } }, "effects",
			func(t *testing.T, info NodeTypeInfo) {
				if !info.Risky {
					t.Error("not risky")
				}
			}, "EffectsFunc"},
		{"effects panic with nil", func(d *NodeDef) { d.EffectsFunc = func(*Node) []Effect { panic(nil) } }, "effects",
			func(t *testing.T, info NodeTypeInfo) {
				if !info.Risky {
					t.Error("not risky")
				}
			}, "EffectsFunc"},
		{"outputs panic, static ports", func(d *NodeDef) { d.OutputsFunc = func(*Node) []string { boom(); return nil } }, "outputs",
			func(t *testing.T, info NodeTypeInfo) {
				if !reflect.DeepEqual(info.Outputs, []string{"static"}) {
					t.Errorf("outputs = %v", info.Outputs)
				}
			}, "OutputsFunc"},
		{"outputs panic, no static ports", func(d *NodeDef) {
			d.Outputs = nil
			d.OutputsFunc = func(*Node) []string { boom(); return nil }
		}, "outputs",
			func(t *testing.T, info NodeTypeInfo) {
				if !reflect.DeepEqual(info.Outputs, []string{PortOut}) {
					t.Errorf("outputs = %v", info.Outputs)
				}
			}, "OutputsFunc"},
		{"outputs panic, no ports at all", func(d *NodeDef) {
			d.Outputs = []string{}
			d.OutputsFunc = func(*Node) []string { boom(); return nil }
		}, "outputs",
			func(t *testing.T, info NodeTypeInfo) {
				if info.Outputs == nil || len(info.Outputs) != 0 {
					t.Errorf("outputs = %#v", info.Outputs)
				}
			}, "OutputsFunc"},
		{"fields panic", func(d *NodeDef) { d.OutputFieldsFunc = func(*Node) []FieldSpec { boom(); return nil } }, "fields",
			func(t *testing.T, info NodeTypeInfo) {
				if info.OutputFields != nil {
					t.Errorf("fields = %+v", info.OutputFields)
				}
			}, "OutputFieldsFunc"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			logs := catDescCaptureLogs(t)
			reg := NewRegistry()
			reg.MustRegister(catDescHookDef("test.hooked", c.mod))
			reg.MustRegister(catDescHookDef("test.fine", nil))
			infos := DescribeNodeTypes(reg, nil)
			if len(infos) != 2 {
				t.Fatalf("got %d infos", len(infos))
			}
			byType := catDescByType(infos)
			healthy(t, byType["test.hooked"], c.skip)
			c.check(t, byType["test.hooked"])
			// The neighbour is untouched.
			healthy(t, byType["test.fine"], "")
			// The fallback is logged once, with type and hook, and without the whole panic value.
			if got, want := logs.fallbacks(), []string{"test.hooked/" + c.wantLog}; !reflect.DeepEqual(got, want) {
				t.Errorf("logged %v, want %v", got, want)
			}
		})
	}

	t.Run("every hook panics", func(t *testing.T) {
		logs := catDescCaptureLogs(t)
		reg := NewRegistry()
		reg.MustRegister(catDescHookDef("test.broken", func(d *NodeDef) {
			d.OutputsFunc = func(*Node) []string { boom(); return nil }
			d.OutputFieldsFunc = func(*Node) []FieldSpec { boom(); return nil }
			d.EffectsFunc = func(*Node) []Effect { boom(); return nil }
			d.AvailabilityFunc = func() Availability { boom(); return Availability{} }
		}))
		info := DescribeNodeTypes(reg, nil)[0]
		if info.Availability.State != BlockedState || !info.Risky || len(info.Effects) != 0 || info.OutputFields != nil ||
			!reflect.DeepEqual(info.Outputs, []string{"static"}) || info.Params == nil || info.Inputs == nil {
			t.Errorf("info = %+v", info)
		}
		want := []string{"test.broken/OutputsFunc", "test.broken/EffectsFunc", "test.broken/AvailabilityFunc", "test.broken/OutputFieldsFunc"}
		slices.Sort(want)
		got := logs.fallbacks()
		slices.Sort(got)
		if !reflect.DeepEqual(got, want) {
			t.Errorf("logged %v, want %v", got, want)
		}
	})

	t.Run("a huge panic value is cut in the log", func(t *testing.T) {
		logs := catDescCaptureLogs(t)
		reg := NewRegistry()
		reg.MustRegister(catDescHookDef("test.loud", func(d *NodeDef) {
			d.EffectsFunc = func(*Node) []Effect { panic(strings.Repeat("x", 1<<20)) }
		}))
		DescribeNodeTypes(reg, nil)
		if got := logs.attr(0, "panic"); len(got) == 0 || len(got) > 200 {
			t.Errorf("logged panic value has %d bytes", len(got))
		}
	})
}

// The trigger sample comes from a table the definition does not own, so it is injected
// there for this test. The sample of a trigger that panics is left out; the next trigger
// still has its own.
func TestDescribeNodeTypesTriggerSamplePanics(t *testing.T) {
	logs := catDescCaptureLogs(t)
	const typ = "test.panic_sample"
	table := triggerTypes()
	table[typ] = triggerType{sample: func(*Node) map[string]any { panic("sample boom") }}
	t.Cleanup(func() { delete(table, typ) })

	reg := NewRegistry()
	reg.MustRegister(&NodeDef{Type: typ, Category: "trigger", Label: "Panics", Trigger: true})
	if err := RegisterTriggerNodes(reg); err != nil {
		t.Fatal(err)
	}
	byType := catDescByType(DescribeNodeTypes(reg, nil))
	if info := byType[typ]; info.Sample != nil || !info.Trigger || !reflect.DeepEqual(info.Outputs, []string{PortOut}) {
		t.Errorf("panicking trigger = %+v", info)
	}
	if byType[TypeTriggerWebhook].Sample["payload"] == nil {
		t.Error("the webhook trigger lost its sample")
	}
	if got, want := logs.fallbacks(), []string{typ + "/TriggerSample"}; !reflect.DeepEqual(got, want) {
		t.Errorf("logged %v, want %v", got, want)
	}
}

func catDescToolSchema(ops ...string) map[string]any {
	enum := make([]any, len(ops))
	for i, op := range ops {
		enum[i] = op
	}
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"operation": map[string]any{"type": "string", "enum": enum},
			"name":      map[string]any{"type": "string"},
		},
		"required": []any{"operation"},
	}
}

// Nothing in the real catalog, with generic tool nodes, may need a fallback or fail to encode.
func TestDescribeNodeTypesRealCatalog(t *testing.T) {
	logs := catDescCaptureLogs(t)
	reg := catalogRegistry(t, fullEnv())
	tools := []GenericTool{
		{Name: "docker", Category: "infrastructure", Description: "Containers", Schema: catDescToolSchema("list", "get", "delete")},
		{Name: "execute_shell", Category: "system", Schema: map[string]any{"type": "object",
			"properties": map[string]any{"command": map[string]any{"type": "string"}}}},
		{Name: "filesystem", Category: "files", Schema: catDescToolSchema("read", "write", "delete")},
		{Name: "notes", Category: "memory", Schema: sampleToolSchema()},
		{Name: "bare", Category: "other"},
	}
	registered := RefreshGenericTools(reg, tools, fullEnv())
	if registered < 4 {
		t.Fatalf("only %d generic nodes registered", registered)
	}
	infos := DescribeNodeTypes(reg, nil)
	if len(infos) != 35+registered {
		t.Fatalf("%d infos, want %d", len(infos), 35+registered)
	}
	if got := logs.fallbacks(); len(got) != 0 {
		t.Fatalf("hooks of the real catalog needed fallbacks: %v", got)
	}
	raw, err := json.Marshal(infos)
	if err != nil {
		t.Fatalf("the palette does not encode: %v", err)
	}
	var decoded []map[string]any
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatal(err)
	}
	for i, m := range decoded {
		for _, key := range []string{"inputs", "outputs", "params"} {
			if _, isList := m[key].([]any); !isList {
				t.Errorf("%s: %s is %v, not a list", infos[i].Type, key, m[key])
			}
		}
		if infos[i].Label == "" {
			t.Errorf("%s has no label", infos[i].Type)
		}
		switch infos[i].Availability.State {
		case AvailableState, NeedsSetupState, BlockedState:
		default:
			t.Errorf("%s: availability %+v", infos[i].Type, infos[i].Availability)
		}
	}
}

func TestDescribeNodeTypesNilArguments(t *testing.T) {
	if infos := DescribeNodeTypes(nil, nil); infos == nil || len(infos) != 0 {
		t.Errorf("nil registry: %#v", infos)
	}
	reg := NewRegistry()
	reg.MustRegister(&NodeDef{Type: "test.a", Category: "test", Label: "A"})
	if infos := DescribeNodeTypes(reg, nil); len(infos) != 1 || infos[0].Label != "A" {
		t.Errorf("nil translator: %+v", infos)
	}
}

// Equal labels in one category come back in the same order on every call, whatever the
// order of registration.
func TestDescribeNodeTypesOrderIsStable(t *testing.T) {
	for _, order := range [][]string{{"test.b", "test.a", "test.c"}, {"test.c", "test.a", "test.b"}} {
		reg := NewRegistry()
		for _, typ := range order {
			reg.MustRegister(&NodeDef{Type: typ, Category: "test", Label: "Same"})
		}
		reg.MustRegister(&NodeDef{Type: "test.first", Category: "test", Label: "Aaa"})
		reg.MustRegister(&NodeDef{Type: "logic.x", Category: "logic", Label: "Zzz"})
		want := []string{"logic.x", "test.first", "test.a", "test.b", "test.c"}
		for i := 0; i < 50; i++ {
			var got []string
			for _, info := range DescribeNodeTypes(reg, nil) {
				got = append(got, info.Type)
			}
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("registration %v, call %d: %v, want %v", order, i, got, want)
			}
		}
	}
}

// The description of merge needs the mode: the editor computes its fields from it.
func TestMergeFieldsFollowTheMode(t *testing.T) {
	reg := catalogRegistry(t, fullEnv())
	info := catDescByType(DescribeNodeTypes(reg, nil))[TypeMerge]
	if info.DynamicFields != "mode" || info.DynamicOutputs != "" {
		t.Errorf("merge markers = %q, %q", info.DynamicFields, info.DynamicOutputs)
	}
	// The default mode, wait_all, declares no field.
	if len(info.OutputFields) != 0 {
		t.Errorf("the default mode declares %+v", info.OutputFields)
	}
	def := lookupDef(t, reg, TypeMerge)
	appendFields := []FieldSpec{{Name: "items", Type: "list", Primary: true}, {Name: "count", Type: "number"}}
	for _, c := range []struct {
		mode any
		want []FieldSpec
	}{
		{"append", appendFields},
		{"wait_all", nil},
		{nil, nil},
		{"", nil},
		{"unknown", nil},
		{"APPEND", nil},
		{42.0, nil},
	} {
		got := def.FieldsOf(&Node{Type: TypeMerge, Params: map[string]any{"mode": c.mode}})
		if !reflect.DeepEqual(got, c.want) {
			t.Errorf("mode %#v: fields %+v, want %+v", c.mode, got, c.want)
		}
	}
}

// Every node type whose ports or fields depend on its parameters has a marker for the editor.
func TestDynamicMarkersCoverTheHooks(t *testing.T) {
	for _, def := range catalogRegistry(t, fullEnv()).All() {
		if def.OutputsFunc != nil && dynamicOutputs[def.Type] == "" {
			t.Errorf("%s has OutputsFunc and no dynamicOutputs entry", def.Type)
		}
		if def.OutputFieldsFunc != nil && dynamicFields[def.Type] == "" {
			t.Errorf("%s has OutputFieldsFunc and no dynamicFields entry", def.Type)
		}
	}
	for typ, param := range dynamicFields {
		if param == "" {
			t.Errorf("%s: empty driver", typ)
		}
	}
}

// A generic node is as risky in the palette as the worst thing its tool can do.
func TestDescribeGenericNodesReportTheWorstCase(t *testing.T) {
	reg := NewRegistry()
	RefreshGenericTools(reg, []GenericTool{
		{Name: "docker", Category: "infrastructure", Schema: catDescToolSchema("list", "get", "delete")},
		{Name: "proxmox", Category: "infrastructure", Schema: catDescToolSchema("list", "get")},
		{Name: "execute_shell", Category: "system", Schema: map[string]any{"type": "object",
			"properties": map[string]any{"command": map[string]any{"type": "string"}}}},
		{Name: "weather", Category: "other", Schema: map[string]any{"type": "object",
			"properties": map[string]any{"city": map[string]any{"type": "string"}}}},
	}, StaticEnv{})
	byType := catDescByType(DescribeNodeTypes(reg, nil))
	for _, c := range []struct {
		tool    string
		effects []Effect
		risky   bool
	}{
		{"docker", []Effect{EffectDeletes, EffectSystemChange}, true},
		{"proxmox", nil, false},
		{"execute_shell", []Effect{EffectRunsCode}, true},
		{"weather", nil, false},
	} {
		info, ok := byType[GenericTypePrefix+c.tool]
		if !ok {
			t.Fatalf("no node for %s", c.tool)
		}
		if info.Risky != c.risky || !slices.Equal(info.Effects, c.effects) {
			t.Errorf("%s: effects %v risky %v, want %v %v", c.tool, info.Effects, info.Risky, c.effects, c.risky)
		}
	}
}

// Even a default on the operation parameter does not narrow a generic node to one
// operation in the palette.
func TestDescribeGenericNodeIgnoresAnOperationDefault(t *testing.T) {
	reg := NewRegistry()
	RefreshGenericTools(reg, []GenericTool{{Name: "docker", Category: "infrastructure", Schema: catDescToolSchema("list", "get", "delete")}}, StaticEnv{})
	def := lookupDef(t, reg, GenericTypePrefix+"docker")
	op, _ := genericOperation(def.Params)
	for i := range def.Params {
		if def.Params[i].Name == op {
			def.Params[i].Default = "list"
		}
	}
	// Setup: with the default applied, one operation (a read) would be all the node does.
	if IsRisky(def.EffectsOf(sampleNode(def))) {
		t.Fatal("setup: the default does not narrow the operation")
	}
	info := catDescByType(DescribeNodeTypes(reg, nil))[def.Type]
	if !info.Risky || !slices.Contains(info.Effects, EffectDeletes) {
		t.Errorf("effects %v risky %v", info.Effects, info.Risky)
	}
}

// catalogKeysRequested returns the i18n keys DescribeNodeTypes asks the translator for
// when it describes reg: the set the translation files of the editor must cover. Generic
// tool nodes carry literal text and ask for none (TestDescribeNodeTypesRequestedKeys), so
// for a registry with generic tools this is the set of the non-generic types. A key that
// is empty is never asked for.
func catalogKeysRequested(reg *Registry) map[string]bool {
	asked := map[string]bool{}
	DescribeNodeTypes(reg, func(key string) string {
		asked[key] = true
		return key
	})
	return asked
}

// The real catalog asks for well-formed keys only, and a generic tool node asks for none.
func TestDescribeNodeTypesRequestedKeys(t *testing.T) {
	reg := catalogRegistry(t, fullEnv())
	asked := catalogKeysRequested(reg)
	if len(asked) == 0 {
		t.Fatal("the catalog asks for no key")
	}
	for key := range asked {
		if key == "" || !strings.HasPrefix(key, "easydrag.") || strings.TrimSpace(key) != key {
			t.Errorf("malformed key %q", key)
		}
	}
	for _, want := range []string{
		"easydrag.node.ai_step.label",         // a node label
		"easydrag.node.web_search.label",      // another one
		"easydrag.param.search_query",         // a parameter label
		"easydrag.option.search_auto",         // an option label
		"easydrag.help.ha_service",            // a help text
		"easydrag.param.field_name",           // inside a fields parameter
		"easydrag.node.trigger_webhook.label", // a trigger
	} {
		if !asked[want] {
			t.Errorf("missing key %s", want)
		}
	}

	// Generic tool nodes add no key, whatever the tool's schema holds.
	generic := NewRegistry()
	RefreshGenericTools(generic, []GenericTool{
		{Name: "docker", Category: "infrastructure", Description: "Containers", Schema: sampleToolSchema()},
		{Name: "execute_shell", Category: "system", Schema: map[string]any{"type": "object",
			"properties": map[string]any{"command": map[string]any{"type": "string"}}}},
	}, StaticEnv{})
	if len(generic.All()) != 2 {
		t.Fatalf("setup: %d generic nodes", len(generic.All()))
	}
	if keys := catalogKeysRequested(generic); len(keys) != 0 {
		t.Errorf("generic nodes ask for keys: %v", keys)
	}
	RefreshGenericTools(reg, []GenericTool{{Name: "docker", Category: "infrastructure", Schema: sampleToolSchema()}}, StaticEnv{})
	if with := catalogKeysRequested(reg); !reflect.DeepEqual(with, asked) {
		t.Error("adding a generic node changed the requested keys")
	}
}

// Keys of every kind are asked for: node texts, parameters and their nested fields,
// options, help, static and dynamic output fields. A hook that panics loses only its own
// keys, and a text without a key is not asked for.
func TestDescribeNodeTypesRequestedKeysFromDefinitions(t *testing.T) {
	logs := catDescCaptureLogs(t)
	reg := NewRegistry()
	reg.MustRegister(&NodeDef{Type: "test.dyn", Category: "test", LabelKey: "k.label", DescriptionKey: "k.desc",
		Params: []ParamSpec{{Name: "p", Kind: ParamFields, LabelKey: "k.p", HelpKey: "k.p.help",
			Options: []Option{{Value: "v", LabelKey: "k.opt"}, {Value: "raw", Label: "Raw"}},
			Fields:  []ParamSpec{{Name: "inner", LabelKey: "k.inner", Options: []Option{{Value: "w", LabelKey: "k.inner.opt"}}}}}},
		OutputFields:     []FieldSpec{{Name: "static", DescriptionKey: "k.static"}, {Name: "plain"}},
		OutputFieldsFunc: func(*Node) []FieldSpec { return []FieldSpec{{Name: "dynamic", DescriptionKey: "k.dynamic"}} }})
	reg.MustRegister(&NodeDef{Type: "test.panics", Category: "test", LabelKey: "k.panics", SummaryKey: "k.panics.sum",
		OutputFields:     []FieldSpec{{Name: "lost", DescriptionKey: "k.lost"}},
		OutputFieldsFunc: func(*Node) []FieldSpec { panic("boom") }})
	reg.MustRegister(&NodeDef{Type: "test.literal", Category: "test", Label: "Literal", Description: "Literal text",
		Params: []ParamSpec{{Name: "q", Kind: ParamText, Label: "Q", Help: "help"}}})
	want := map[string]bool{"k.label": true, "k.desc": true, "k.p": true, "k.p.help": true, "k.opt": true, "k.inner": true,
		"k.inner.opt": true, "k.dynamic": true, "k.panics": true, "k.panics.sum": true}
	if got := catalogKeysRequested(reg); !reflect.DeepEqual(got, want) {
		t.Errorf("keys = %v\nwant   %v", got, want)
	}
	if got, want := logs.fallbacks(), []string{"test.panics/OutputFieldsFunc"}; !reflect.DeepEqual(got, want) {
		t.Errorf("logged %v, want %v", got, want)
	}
}
