package agent

import (
	"fmt"
	"slices"
	"testing"

	"aurago/internal/config"
	openai "github.com/sashabaranov/go-openai"
)

// selectionBudget is one tool budget of the selection tests.
type selectionBudget struct {
	name                   string
	adaptive, total, token int
}

var selectionBudgets = []selectionBudget{
	{name: "default 10/20/6500", adaptive: 10, total: 20, token: 6500},
	{name: "stability 12/24/6500", adaptive: 12, total: 24, token: 6500},
	{name: "roomy 10/40/-", adaptive: 10, total: 40},
	{name: "small 6/14/6500", adaptive: 6, total: 14, token: 6500},
}

// selectionRun is one run of the adaptive selection: the first selection of
// initAgentLoopState and the refresh before a model call.
type selectionRun struct {
	cfg      *config.Config
	runCfg   RunConfig
	ff       ToolFeatureFlags
	schemas  []openai.Tool
	budget   selectionBudget
	plan     adaptiveSelectionPlan
	first    toolSchemaFilterResult
	swapped  map[string]string
	pinned   map[string]bool
	query    string
	sessions map[string]bool
	recent   []string
}

func startSelectionRun(t *testing.T, query string, ff ToolFeatureFlags, b selectionBudget, history []string) *selectionRun {
	t.Helper()
	cfg := &config.Config{}
	cfg.Agent.AdaptiveTools.Enabled = true
	cfg.Agent.AdaptiveTools.MaxTools = b.adaptive
	cfg.Agent.AdaptiveTools.MaxTotalTools = b.total
	cfg.Agent.AdaptiveTools.MaxSchemaTokens = b.token
	cfg.Agent.AdaptiveTools.AlwaysInclude = []string{"filesystem", "query_memory", "manage_memory", "execute_shell"}
	run := &selectionRun{cfg: cfg, runCfg: RunConfig{Config: cfg}, ff: ff, budget: b, query: query, recent: history}
	run.schemas = BuildNativeToolSchemas(t.TempDir(), nil, ff, nil)
	run.plan = planAdaptiveFirstSelection(adaptiveSelectionInput{
		Config: cfg, RunConfig: run.runCfg, Features: ff, Schemas: run.schemas, Query: query, History: history,
		MaxAdaptive: b.adaptive, MaxTotal: b.total, MaxSchemaTokens: b.token,
	})
	run.first = filterToolSchemasWithReport(run.schemas, run.plan.Options, nil)
	run.swapped = recordAdaptiveSwaps(nil, run.first.Report)
	return run
}

// refresh runs the refresh before a model call on the tools offered so far.
// requested are the tools discover_tools asked for since the last call and
// used the tools the run executed.
func (run *selectionRun) refresh(offered []openai.Tool, requested []string, used ...string) []openai.Tool {
	candidates := restoreAdaptiveSwapPartners(offered, run.schemas, run.swapped)
	present := stringSet(toolSchemaNames(candidates))
	want := stringSet(requested)
	for _, schema := range run.schemas {
		if want[schema.Function.Name] && !present[schema.Function.Name] {
			candidates = append(candidates, schema)
		}
	}
	run.pinned = recordRequestedTools(run.pinned, requested)
	if run.sessions == nil {
		run.sessions = map[string]bool{}
	}
	for _, name := range used {
		run.sessions[name] = true
	}
	opts := adaptiveRefreshFilterOptions(adaptiveRefreshInput{
		Config: run.cfg, RunConfig: run.runCfg, Features: run.ff, Plan: &run.plan, Tools: candidates, Requested: requested,
		Pinned: run.pinned, SessionUsed: run.sessions, Recent: run.recent, InitFiltered: true,
		MaxAdaptive: run.budget.adaptive, MaxTotal: run.budget.total, MaxSchemaTokens: run.budget.token,
		AdditiveTools: adaptiveAdditiveToolsForQuery(run.query), AdditiveSwaps: adaptiveSwapsForQuery(run.query),
	})
	result := filterToolSchemasWithReport(candidates, opts, nil)
	run.swapped = recordAdaptiveSwaps(run.swapped, result.Report)
	return result.Tools
}

var selectionQueries = []string{
	"Erstelle ein Bild von einem Fuchs und zeig mir die Datei im Ordner",
	"restart the docker container and check the cpu usage",
	"send an email to the team and schedule a reminder",
	"play some music and make a short video of it",
	"was steht im Lexikon über die Enzyklopädie?",
	"hello there",
}

func TestPlanAdaptiveFirstSelectionKeepsTheClasses(t *testing.T) {
	ff := allBuiltinToolFeatureFlags()
	cfg := &config.Config{}
	cfg.Agent.AdaptiveTools.AlwaysInclude = []string{"filesystem", "query_memory", "shell"}
	schemas := BuildNativeToolSchemas(t.TempDir(), nil, ff, nil)
	plan := planAdaptiveFirstSelection(adaptiveSelectionInput{
		Config: cfg, RunConfig: RunConfig{Config: cfg}, Features: ff, Schemas: schemas,
		Query:       "Erstelle ein Bild von einem Fuchs und öffne die Datei",
		VoiceOutput: true, History: []string{"proxmox", "proxmox", "docker"}, Requested: []string{"jellyfin"},
		Messages:    []openai.ChatCompletionMessage{{Role: openai.ChatMessageRoleTool, Content: `{"output_ref":"toolout_abc"}`}},
		MaxAdaptive: 10, MaxTotal: 20, MaxSchemaTokens: 6500,
	})
	for _, want := range []string{"filesystem", "query_memory", "shell", "tts", "send_audio", "read_tool_output"} {
		if !slices.Contains(plan.Soft, want) {
			t.Errorf("soft %v lacks %s", plan.Soft, want)
		}
	}
	if !slices.Equal(plan.Generator, []string{"generate_image"}) {
		t.Errorf("generator = %v", plan.Generator)
	}
	if !slices.Equal(plan.History, []string{"proxmox", "docker"}) || !slices.Equal(plan.Requested, []string{"jellyfin"}) {
		t.Errorf("history %v, requested %v", plan.History, plan.Requested)
	}
	// The union is expanded: the legacy alias becomes the tool, hard tools join.
	soft := plan.Options.SoftAlwaysTools
	for _, want := range []string{"execute_shell", "generate_image", "proxmox", "docker", "jellyfin", "discover_tools"} {
		if !slices.Contains(soft, want) {
			t.Errorf("soft options %v lack %s", soft, want)
		}
	}
	if slices.Contains(soft, "shell") {
		t.Errorf("legacy alias left in %v", soft)
	}
	// The seeds are candidates of the ranking, never soft, and a held tool is no seed.
	if len(plan.Seeds) == 0 {
		t.Fatal("a file request got no seeds")
	}
	for _, seed := range plan.Seeds {
		if slices.Contains(soft, seed) {
			t.Errorf("seed %s is held anyway: soft %v", seed, soft)
		}
		if !slices.Contains(plan.Priority, seed) {
			t.Errorf("seed %s is not ranked: %v", seed, plan.Priority)
		}
	}
	for _, name := range adaptiveIntentOnlyTools {
		if slices.Contains(plan.Priority, name) {
			t.Errorf("non-displacing tool %s is ranked", name)
		}
	}
	opts := plan.Options
	if opts.MaxAdaptiveTools != 10 || opts.MaxTotalTools != 20 || opts.MaxSchemaTokens != 6500 || !slices.Equal(opts.PreferredTools, plan.Priority) {
		t.Errorf("options = %+v", opts)
	}
	result := filterToolSchemasWithReport(schemas, opts, nil)
	if result.Report.KeptAdaptive > 10 {
		t.Errorf("adaptive tools %d exceed the cap", result.Report.KeptAdaptive)
	}
	got := toolSchemaNames(result.Tools)
	for _, want := range []string{"generate_image", "proxmox", "docker", "jellyfin", "tts"} {
		if !slices.Contains(got, want) {
			t.Errorf("selection %v lacks the soft tool %s", got, want)
		}
	}
	for _, name := range []string{"generate_music", "generate_video"} {
		if slices.Contains(got, name) && !slices.Contains(plan.Priority[:10], name) {
			t.Errorf("%s offered without media intent", name)
		}
	}
}

// A refresh without a request and without new tool use returns the first
// selection, whatever the budget and the request.
func TestRefreshWithoutNewInputReturnsTheFirstSelection(t *testing.T) {
	ff := allBuiltinToolFeatureFlags()
	ff.LocalWikipediaEnabled = true
	for _, b := range selectionBudgets {
		for _, query := range selectionQueries {
			for _, history := range [][]string{nil, {"proxmox", "docker"}, {"local_wikipedia", "wikipedia_search"}} {
				run := startSelectionRun(t, query, ff, b, history)
				want := sortedToolNames(run.first.Tools)
				got := sortedToolNames(run.refresh(run.first.Tools, nil))
				if !slices.Equal(got, want) {
					t.Errorf("%s %q history=%v: the refresh changed the selection\nfirst   %v\nrefresh %v", b.name, query, history, want, got)
				}
				// A second refresh is stable too.
				if again := sortedToolNames(run.refresh(run.first.Tools, nil)); !slices.Equal(again, want) {
					t.Errorf("%s %q: the second refresh differs: %v", b.name, query, again)
				}
			}
		}
	}
}

// One requested tool is added; at most one adaptive tool leaves for it, and no
// soft, generator or history tool ever drops because of its rank.
func TestRefreshWithARequestDropsAtMostOneAdaptiveTool(t *testing.T) {
	ff := allBuiltinToolFeatureFlags()
	b := selectionBudget{name: "roomy 10/40/-", adaptive: 10, total: 40}
	for _, query := range selectionQueries {
		run := startSelectionRun(t, query, ff, b, []string{"docker"})
		first := toolSchemaNames(run.first.Tools)
		var request string
		for _, name := range []string{"jellyfin", "truenas", "grafana", "obsidian"} {
			if !slices.Contains(first, name) {
				request = name
				break
			}
		}
		if request == "" {
			t.Fatalf("%q: every candidate request is already offered: %v", query, first)
		}
		got := toolSchemaNames(run.refresh(run.first.Tools, []string{request}))
		if !slices.Contains(got, request) {
			t.Errorf("%q: the requested %s is missing from %v", query, request, got)
		}
		dropped := slices.DeleteFunc(slices.Clone(first), func(name string) bool { return slices.Contains(got, name) })
		if len(dropped) > 1 {
			t.Errorf("%q: the request dropped %v", query, dropped)
		}
		for _, name := range dropped {
			if slices.Contains(run.plan.Options.SoftAlwaysTools, name) || slices.Contains(run.plan.Options.HardAlwaysTools, name) {
				t.Errorf("%q: a held tool %s was dropped by rank", query, name)
			}
		}
	}
}

// A flood of requests costs adaptive tools only: the configured, generator and
// history tools and the tools used during the run keep their places. The
// requests are adaptive tools themselves, ranked first up to the adaptive cap.
func TestRefreshKeepsSoftClassesUnderARequestFlood(t *testing.T) {
	ff := allBuiltinToolFeatureFlags()
	ff.LocalWikipediaEnabled = true
	b := selectionBudget{name: "roomy 10/40/-", adaptive: 10, total: 60}
	flood := []string{"jellyfin", "proxmox", "truenas", "grafana", "netlify", "github", "ollama", "vercel", "tailscale", "ansible", "home_assistant", "mqtt_publish"}
	run := startSelectionRun(t, "Erstelle ein Bild von einem Fuchs", ff, b, []string{"local_wikipedia", "docker"})
	first := toolSchemaNames(run.first.Tools)
	// An adaptive tool the run then executes keeps a soft place too.
	var adaptive string
	for _, name := range first {
		if !slices.Contains(run.plan.Options.SoftAlwaysTools, name) && !slices.Contains(run.plan.Options.HardAlwaysTools, name) && !slices.Contains(flood, name) {
			adaptive = name
		}
	}
	if adaptive == "" {
		t.Fatalf("no adaptive tool in %v", first)
	}
	got := toolSchemaNames(run.refresh(run.first.Tools, flood, adaptive))
	held := append(slices.Clone(run.plan.Options.SoftAlwaysTools), run.plan.Options.HardAlwaysTools...)
	held = append(held, adaptive)
	for _, name := range held {
		if slices.Contains(first, name) && !slices.Contains(got, name) {
			t.Errorf("held tool %s dropped under the flood: %v", name, got)
		}
	}
	for _, name := range []string{"generate_image", "local_wikipedia", "docker"} {
		if !slices.Contains(got, name) {
			t.Errorf("%s dropped under the flood: %v", name, got)
		}
	}
	// The requests rank first among the adaptive tools, so the adaptive cap
	// decides how many of them fit.
	for _, name := range flood[:b.adaptive] {
		if !slices.Contains(got, name) {
			t.Errorf("requested %s missing under the flood: %v", name, got)
		}
	}
}

// Without a plan the refresh is what it always was: configured soft tools, the
// position of the schemas as the ranking.
func TestRefreshWithoutAPlanRanksByPosition(t *testing.T) {
	cfg := &config.Config{}
	cfg.Agent.AdaptiveTools.AlwaysInclude = []string{"filesystem"}
	tools := []openai.Tool{testFilterSchema("filesystem"), testFilterSchema("a"), testFilterSchema("b")}
	opts := adaptiveRefreshFilterOptions(adaptiveRefreshInput{
		Config: cfg, Tools: tools, Requested: []string{"b"}, Pinned: map[string]bool{"b": true},
		SessionUsed: map[string]bool{"a": true}, MaxAdaptive: 2, MaxTotal: 5,
	})
	if got := fmt.Sprint(opts.PreferredTools); got != "[b filesystem a b]" {
		t.Errorf("preferred = %s", got)
	}
	if got := fmt.Sprint(opts.SoftAlwaysTools); got != "[filesystem]" {
		t.Errorf("soft = %s", got)
	}
	if opts.SwapProtectedTools != nil || !slices.Equal(opts.PinnedTools, []string{"b"}) || opts.MaxAdaptiveTools != 2 || opts.MaxTotalTools != 5 {
		t.Errorf("options = %+v", opts)
	}
	// A first selection that ranked the whole catalog protects the session's tools.
	opts = adaptiveRefreshFilterOptions(adaptiveRefreshInput{Config: cfg, Tools: tools, SessionUsed: map[string]bool{"a": true}, InitFiltered: true})
	if !slices.Equal(opts.SwapProtectedTools, []string{"a"}) {
		t.Errorf("protected = %v", opts.SwapProtectedTools)
	}
}
