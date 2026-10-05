package server

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log"
	"log/slog"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/sashabaranov/go-openai"

	"aurago/internal/agent"
	"aurago/internal/config"
	"aurago/internal/flows"
)

// Tests of flowCatalogEnv over the agent's REAL tool schemas: the spend list, the
// palette wiring, the agent's permissions and the config UI sections. The concurrency
// tests are in flows_catalog_env_concurrency_test.go.

// c12MinGenericNodes is a floor for the generic nodes of the full configuration (151
// today): far fewer means tools vanish from the palette by accident.
const c12MinGenericNodes = 100

// c12FullToolConfig returns a configuration that enables every integration and tool
// group a test can switch on: every bool field of the configuration is set, except the
// ones that take something away (read-only modes, Docker and no-new-privileges runtime
// flags, disable switches, dry runs). It also fills the few non-bool gates of the tool
// flags. The skills and tools directories are empty, so no skill__ or tool__ schemas
// appear.
func c12FullToolConfig(t *testing.T) *config.Config {
	t.Helper()
	cfg := &config.Config{}
	c12EnableBools(reflect.ValueOf(cfg).Elem())
	cfg.Directories.SkillsDir, cfg.Directories.ToolsDir = t.TempDir(), t.TempDir()
	cfg.Telegram.BotToken, cfg.Telegram.UserID = "c12-bot-token", 1
	cfg.Composio.APIKey, cfg.Manus.APIKey = "c12-key", "c12-key"
	cfg.TTS.Provider = "google"
	return cfg
}

func c12EnableBools(v reflect.Value) {
	if v.Kind() != reflect.Struct {
		return
	}
	for i := 0; i < v.NumField(); i++ {
		f, name := v.Field(i), v.Type().Field(i).Name
		if !f.CanSet() {
			continue
		}
		switch f.Kind() {
		case reflect.Bool:
			if !c12NegativeBool(name) {
				f.SetBool(true)
			}
		case reflect.Struct:
			c12EnableBools(f)
		}
	}
}

func c12NegativeBool(name string) bool {
	lower := strings.ToLower(name)
	for _, s := range []string{"readonly", "disable", "isdocker", "nonewprivileges", "dryrun"} {
		if strings.Contains(lower, s) {
			return true
		}
	}
	return false
}

// c12RealEnv is a flowCatalogEnv over the agent's real tool schemas; cur holds the
// current configuration.
func c12RealEnv(cur *atomic.Pointer[config.Config], logger *slog.Logger) *flowCatalogEnv {
	return &flowCatalogEnv{
		schemas: func(cfg *config.Config) []openai.Tool { return agent.ConfiguredToolSchemas(cfg, logger) },
		current: cur.Load,
	}
}

// c12LogBuffer collects slog output behind a mutex.
type c12LogBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *c12LogBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *c12LogBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

// c12CaptureDefault routes the default slog logger at level into a buffer for the test.
// slog.SetDefault also points the log package at the new handler, and restoring the
// previous default does not undo that, so the log package's writer and flags are
// restored as well.
func c12CaptureDefault(t *testing.T, level slog.Level) *c12LogBuffer {
	t.Helper()
	logs := &c12LogBuffer{}
	previous, writer, flags := slog.Default(), log.Writer(), log.Flags()
	slog.SetDefault(slog.New(slog.NewTextHandler(logs, &slog.HandlerOptions{Level: level})))
	t.Cleanup(func() {
		slog.SetDefault(previous)
		log.SetOutput(writer)
		log.SetFlags(flags)
	})
	return logs
}

// c12Invoker records tool calls and answers every one with success.
type c12Invoker struct {
	mu    sync.Mutex
	calls []flows.ToolRequest
}

func (i *c12Invoker) InvokeTool(_ context.Context, req flows.ToolRequest) (flows.ToolResponse, error) {
	i.mu.Lock()
	defer i.mu.Unlock()
	i.calls = append(i.calls, req)
	return flows.ToolResponse{Output: `{"status":"success"}`, Status: "success"}, nil
}

func (i *c12Invoker) count() int {
	i.mu.Lock()
	defer i.mu.Unlock()
	return len(i.calls)
}

func (i *c12Invoker) last() flows.ToolRequest {
	i.mu.Lock()
	defer i.mu.Unlock()
	return i.calls[len(i.calls)-1]
}

// c12Options returns the option values of param name of def, and whether def has it.
func c12Options(def *flows.NodeDef, name string) ([]string, bool) {
	for _, p := range def.Params {
		if p.Name != name {
			continue
		}
		var values []string
		for _, o := range p.Options {
			values = append(values, o.Value)
		}
		return values, true
	}
	return nil, false
}

// c12SchemaOperations returns the operation enum of a real tool schema.
func c12SchemaOperations(tool flows.GenericTool) []string {
	props, _ := tool.Schema["properties"].(map[string]any)
	op, _ := props["operation"].(map[string]any)
	var out []string
	switch e := op["enum"].(type) {
	case []string:
		out = append(out, e...)
	case []any:
		for _, v := range e {
			if s, ok := v.(string); ok {
				out = append(out, s)
			}
		}
	}
	return out
}

// c12ConfigUISections reads the section keys of the config UI (SECTIONS in
// ui/js/config/main.js), the targets of the editor's "Set up" link.
func c12ConfigUISections(t *testing.T) map[string]bool {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "..", "ui", "js", "config", "main.js"))
	if err != nil {
		t.Fatalf("read the config UI: %v", err)
	}
	src := string(data)
	start := strings.Index(src, "const SECTIONS = [")
	if start < 0 {
		t.Fatal("the config UI has no SECTIONS list")
	}
	end := strings.Index(src[start:], "\n];")
	if end < 0 {
		t.Fatal("the SECTIONS list of the config UI has no end")
	}
	keys := map[string]bool{}
	for _, m := range regexp.MustCompile(`key: '([a-z0-9_]+)'`).FindAllStringSubmatch(src[start:start+end], -1) {
		keys[m[1]] = true
	}
	if len(keys) < 50 {
		t.Fatalf("found only %d config UI sections; the parser no longer matches the file", len(keys))
	}
	return keys
}

// Requirement 2: tools that spend outside the flow budget on every call never become
// generic nodes; every name on the list is a real tool, so a rename is noticed.
func TestC12SpendingToolsNeverBecomeGenericNodes(t *testing.T) {
	cfg := c12FullToolConfig(t)
	cfg.Tools.Wikipedia.SummaryMode = false
	var cur atomic.Pointer[config.Config]
	cur.Store(cfg)
	env := c12RealEnv(&cur, nil)
	reg := flows.NewRegistry()
	env.refreshRegistry(reg, cfg)
	names := env.toolNames(cfg)

	for name := range flowSpendingTools {
		if !names[name] {
			t.Errorf("%s is listed as spending but is no tool of the full configuration", name)
		}
	}
	family := map[string][]string{}
	for name := range names {
		for prefix := range flowSpendingToolPrefixes {
			if strings.HasPrefix(name, prefix) {
				family[prefix] = append(family[prefix], name)
			}
		}
	}
	for prefix := range flowSpendingToolPrefixes {
		if len(family[prefix]) == 0 {
			t.Errorf("no tool of the full configuration starts with %s", prefix)
		}
		// The prefixes are fail-closed: show who they catch, so a new member is seen.
		slices.Sort(family[prefix])
		t.Logf("spending family %s*: %v", prefix, family[prefix])
	}
	spending := []string{"analyze_image", "generate_image", "generate_music", "generate_video", "manus",
		"huggingface", "memory_reflect", "space_agent", "treg_call", "transcribe_audio", "telnyx_sms", "telnyx_call",
		"telnyx_manage", "yepapi_amazon", "yepapi_instagram", "yepapi_scrape", "yepapi_seo", "yepapi_serp",
		"yepapi_tiktok", "yepapi_youtube"}
	for _, name := range spending {
		if !names[name] {
			t.Errorf("%s is no tool of the full configuration", name)
		}
		if !flowToolSpends(cfg, name) {
			t.Errorf("%s is not on the spend list", name)
		}
		if _, isNode := reg.Lookup(flows.GenericTypePrefix + name); isNode {
			t.Errorf("tool.%s spends outside the flow budget but is a generic node", name)
		}
	}
	for name := range names {
		_, isNode := reg.Lookup(flows.GenericTypePrefix + name)
		if flowToolSpends(cfg, name) && isNode {
			t.Errorf("tool.%s spends outside the flow budget but is a generic node", name)
		}
	}
	for _, name := range []string{"treg_catalog", "treg_status", "wikipedia_search", "tts"} {
		if _, ok := reg.Lookup(flows.GenericTypePrefix + name); !ok {
			t.Errorf("tool.%s spends nothing here and must stay a generic node", name)
		}
	}
	// Accepted on purpose: speaking spends no model tokens.
	for _, name := range []string{"bluetooth", "chromecast"} {
		def, ok := reg.Lookup(flows.GenericTypePrefix + name)
		if !ok {
			t.Errorf("tool.%s must stay a generic node", name)
			continue
		}
		if options, _ := c12Options(def, "operation"); !slices.Contains(options, "speak") {
			t.Errorf("tool.%s operations = %v; speak is accepted and must stay", name, options)
		}
	}

	summarising := c12FullToolConfig(t)
	summarising.Tools.Wikipedia.SummaryMode = true
	cur.Store(summarising)
	env.refreshRegistry(reg, summarising)
	if _, ok := reg.Lookup(flows.GenericTypePrefix + "wikipedia_search"); ok {
		t.Fatal("tool.wikipedia_search must not be a node while its summary mode is on")
	}
	if a := env.ToolAvailability("wikipedia_search"); a.State != flows.AvailableState {
		t.Fatalf("wikipedia_search = %+v: leaving the node out must not change the tool's availability", a)
	}
}

// Requirement 2: tools with cheap operations next to spending ones keep their node
// without the spending operations and their parameters; the cheap operations still run.
func TestC12SpendingOperationsAreDropped(t *testing.T) {
	cfg := c12FullToolConfig(t)
	var cur atomic.Pointer[config.Config]
	cur.Store(cfg)
	env := c12RealEnv(&cur, nil)
	reg := flows.NewRegistry()
	env.refreshRegistry(reg, cfg)
	_, generic := env.snapshot(cfg)
	schemas := map[string]flows.GenericTool{}
	for _, tool := range generic {
		schemas[tool.Name] = tool
	}
	cases := []struct {
		tool, cheap   string
		dropped       []string
		params, keeps []string // parameters dropped with the operations, and shared ones that stay
	}{
		{"smart_file_read", "analyze", []string{"summarize"}, []string{"query"}, []string{"file_path"}},
		{"go2rtc", "status", []string{"analyze_snapshot"}, []string{"prompt"}, []string{"stream_id"}},
		{"three_d_printer", "status", []string{"analyze_camera"}, []string{"prompt"}, []string{"printer_id"}},
		{"video_download", "search", []string{"transcribe"}, nil, []string{"url"}},
		{"fritzbox_telephony", "get_call_list", []string{"transcribe_tam_message"}, nil, []string{"tam_index"}},
		{"rtl_sdr", "status", []string{"transcribe"}, []string{"transcribe"}, []string{"id"}},
		{"virtual_computers", "status", []string{"run_shell_task", "run_desktop_task"}, []string{"instruction"},
			[]string{"command", "task_id"}},
		{"knowledge_graph", "search", []string{"optimize", "optimize_graph"}, nil, []string{"content", "limit"}},
		{"invasion_tasks", "task_status", []string{"send_secret", "send_task"}, []string{"key", "value", "task", "egg_name"},
			[]string{"content", "nest_id", "egg_id", "task_id"}},
		{"sip_phone", "status", []string{"dial"}, []string{"target"}, []string{"call_id"}},
	}
	for _, c := range cases {
		t.Run(c.tool, func(t *testing.T) {
			ops := c12SchemaOperations(schemas[c.tool])
			for _, op := range append([]string{c.cheap}, c.dropped...) {
				if !slices.Contains(ops, op) {
					t.Fatalf("the real %s schema lists %v; it must list %s", c.tool, ops, op)
				}
			}
			def, ok := reg.Lookup(flows.GenericTypePrefix + c.tool)
			if !ok {
				t.Fatalf("tool.%s is missing; only its spending operations are dropped", c.tool)
			}
			options, _ := c12Options(def, "operation")
			if !slices.Contains(options, c.cheap) || len(options) != len(ops)-len(c.dropped) {
				t.Fatalf("operation options = %v; only %v may be dropped from %v", options, c.dropped, ops)
			}
			for _, param := range c.params {
				if _, has := c12Options(def, param); has {
					t.Errorf("parameter %s of the dropped operations is still offered", param)
				}
			}
			for _, param := range c.keeps {
				if _, has := c12Options(def, param); !has {
					t.Errorf("parameter %s is used by other operations and must stay", param)
				}
			}

			inv := &c12Invoker{}
			node := &flows.Node{ID: "n_c12aaaaa", Type: def.Type, Params: map[string]any{"operation": c.cheap}}
			if issues := def.Validate(node, flows.ValidateContext{}); len(issues) != 0 {
				t.Errorf("validating %s gave %+v, want none", c.cheap, issues)
			}
			in := flows.ExecInput{Node: node, Params: map[string]any{"operation": c.cheap}, Services: &flows.Services{Tools: inv}}
			if _, err := def.Execute(context.Background(), in); err != nil {
				t.Fatalf("running %s failed: %v", c.cheap, err)
			}
			if inv.count() != 1 || inv.last().Tool != c.tool || inv.last().Args["operation"] != c.cheap {
				t.Fatalf("running %s made calls %+v", c.cheap, inv.calls)
			}
			for _, op := range c.dropped {
				if slices.Contains(options, op) {
					t.Errorf("operation options = %v still hold %s", options, op)
				}
				node.Params["operation"] = op
				if issues := def.Validate(node, flows.ValidateContext{}); len(issues) != 1 || issues[0].Code != flows.IssueParamInvalid {
					t.Errorf("validating %s gave %+v, want one %s issue", op, issues, flows.IssueParamInvalid)
				}
				in.Params = map[string]any{"operation": op}
				_, err := def.Execute(context.Background(), in)
				var ne *flows.NodeError
				if !errors.As(err, &ne) || ne.Code != "FLOW_PARAM_INVALID" {
					t.Errorf("running %s gave %v, want FLOW_PARAM_INVALID", op, err)
				}
			}
			if inv.count() != 1 {
				t.Fatalf("running a dropped operation called the tool: %+v", inv.calls)
			}
		})
	}
}

// Requirement 3: the palette over the REAL agent tool schemas describes without a hook
// fallback, encodes as JSON, and every generic node is usable.
func TestC12DescribeNodeTypesWithTheRealToolSchemas(t *testing.T) {
	warnings := c12CaptureDefault(t, slog.LevelWarn)
	schemaLogs := &c12LogBuffer{}
	cfg := c12FullToolConfig(t)
	s := &Server{Cfg: cfg, Logger: slog.New(slog.NewTextHandler(schemaLogs, &slog.HandlerOptions{Level: slog.LevelWarn}))}
	env := newFlowCatalogEnv(s)
	reg := flows.NewRegistry()
	if err := flows.RegisterCatalog(reg, env); err != nil {
		t.Fatal(err)
	}
	env.refreshRegistry(reg, s.ConfigSnapshot())
	_, tools := env.snapshot(s.ConfigSnapshot())
	schemas := map[string]flows.GenericTool{}
	for _, tool := range tools {
		schemas[tool.Name] = tool
	}
	infos := flows.DescribeNodeTypes(reg, func(key string) string { return key })
	data, err := json.Marshal(infos)
	if err != nil {
		t.Fatalf("the palette does not encode as JSON: %v", err)
	}
	generic := 0
	for _, info := range infos {
		if !strings.HasPrefix(info.Type, flows.GenericTypePrefix) {
			continue
		}
		generic++
		// A node without parameters is right only for a tool that takes none: its
		// schema holds nothing but the agent's _todo, which flows drop.
		if len(info.Params) == 0 {
			props, _ := schemas[info.Tool].Schema["properties"].(map[string]any)
			for name := range props {
				if name != "_todo" {
					t.Errorf("%s has no parameter, but its schema has %s", info.Type, name)
				}
			}
		}
		if info.Availability.State != flows.AvailableState {
			t.Errorf("%s = %+v; every tool of the full configuration is available", info.Type, info.Availability)
		}
	}
	if generic < c12MinGenericNodes {
		t.Errorf("only %d generic nodes, want at least %d", generic, c12MinGenericNodes)
	}
	for _, line := range strings.Split(warnings.String(), "\n") {
		if strings.Contains(line, "flows:") {
			t.Errorf("a hook fell back while describing: %s", line)
		}
	}
	// The agent's schema build may warn about what it leaves out (a skill that collides
	// with a built-in, say); that is its business, not a wiring fault. Errors are.
	for _, line := range strings.Split(strings.TrimSpace(schemaLogs.String()), "\n") {
		switch {
		case strings.Contains(line, "level=ERROR"):
			t.Errorf("building the tool schemas failed: %s", line)
		case line != "":
			t.Logf("building the tool schemas warned: %s", line)
		}
	}
	t.Logf("generic nodes: %d of %d node types; DescribeNodeTypes JSON payload: %d bytes", generic, len(infos), len(data))
}

// Requirement 4: a tool the agent's permissions forbid is never available.
// ConfiguredToolSchemas already leaves such tools out (tool groups like the shell, and
// whole-tool read-only modes like Telnyx's); read-only modes that forbid single
// operations are enforced by the agent's dispatcher, which the flow tool invoker uses.
func TestC12ToolAvailabilityFollowsTheAgentsPermissions(t *testing.T) {
	var cur atomic.Pointer[config.Config]
	cur.Store(c12FullToolConfig(t))
	env := c12RealEnv(&cur, nil)
	for _, tool := range []string{"execute_shell", "telnyx_sms", "telnyx_call"} {
		if a := env.ToolAvailability(tool); a.State != flows.AvailableState {
			t.Fatalf("%s = %+v with everything allowed", tool, a)
		}
	}
	restricted := c12FullToolConfig(t)
	restricted.Agent.AllowShell = false
	restricted.Telnyx.ReadOnly = true
	cur.Store(restricted)
	for _, tool := range []string{"execute_shell", "telnyx_sms", "telnyx_call"} {
		if a := env.ToolAvailability(tool); a.State == flows.AvailableState {
			t.Errorf("%s = %+v although the agent may not use it", tool, a)
		}
	}
}

// Requirement 4: every "Set up" section exists in the config UI.
func TestC12ConfigSectionsExistInTheConfigUI(t *testing.T) {
	sections := c12ConfigUISections(t)
	tools := map[string]bool{flows.BraveSearchTool: true, flows.PDFExtractorTool: true, flows.GotenbergTool: true}
	for _, schema := range agent.ConfiguredToolSchemas(c12FullToolConfig(t), nil) {
		if schema.Function != nil {
			tools[schema.Function.Name] = true
		}
	}
	reg := flows.NewRegistry()
	if err := flows.RegisterCatalog(reg, flows.StaticEnv{}); err != nil {
		t.Fatal(err)
	}
	for _, def := range reg.All() {
		if def.Tool != "" {
			tools[def.Tool] = true
		}
	}
	for tool := range tools {
		section := flowToolConfigSection(tool)
		if !sections[section] {
			t.Errorf("%s points at the config section %q, which the config UI does not have", tool, section)
		}
		if a := flowToolAvailability(&config.Config{}, map[string]bool{}, tool); a.ConfigSection != section {
			t.Errorf("%s needs setup in section %q, want %q", tool, a.ConfigSection, section)
		}
	}
}
