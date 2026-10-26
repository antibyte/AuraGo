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
	order    []string
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
	run.order = recordRequestOrder(run.order, requested)
	if run.sessions == nil {
		run.sessions = map[string]bool{}
	}
	for _, name := range used {
		run.sessions[name] = true
	}
	opts := adaptiveRefreshFilterOptions(adaptiveRefreshInput{
		Config: run.cfg, RunConfig: run.runCfg, Features: run.ff, Plan: &run.plan, Tools: candidates, Requested: requested,
		RequestOrder: run.order, Pinned: run.pinned, SessionUsed: run.sessions, Recent: run.recent, InitFiltered: true,
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

// requestsAbsent picks tools that discover_tools could request and that the
// selection does not offer yet. They are small schemas, so a request fits a
// token cap as well.
func requestsAbsent(t *testing.T, offered []openai.Tool, n int) []string {
	t.Helper()
	have := toolSchemaNames(offered)
	var out []string
	for _, name := range []string{"uptime_kuma", "wake_on_lan", "whois_lookup", "mac_lookup", "mdns_scan", "upnp_scan"} {
		if !slices.Contains(have, name) {
			out = append(out, name)
		}
		if len(out) == n {
			return out
		}
	}
	t.Fatalf("fewer than %d candidate requests are absent from %v", n, have)
	return nil
}

// heldTools are the tools a refresh may never drop for rank: the hard ones and
// the plan's soft classes.
func (run *selectionRun) heldTools() []string {
	return append(slices.Clone(run.plan.Options.SoftAlwaysTools), run.plan.Options.HardAlwaysTools...)
}

// One requested tool is added. Under the count caps at most one adaptive tool
// leaves for it; under a token cap a larger schema can displace more, but only
// adaptive tools. A soft, generator or history tool never drops because of
// its rank.
func TestRefreshWithARequestDropsOnlyAdaptiveTools(t *testing.T) {
	ff := allBuiltinToolFeatureFlags()
	for _, b := range selectionBudgets {
		for _, query := range selectionQueries {
			run := startSelectionRun(t, query, ff, b, []string{"docker"})
			first := toolSchemaNames(run.first.Tools)
			request := requestsAbsent(t, run.first.Tools, 1)[0]
			got := toolSchemaNames(run.refresh(run.first.Tools, []string{request}))
			if !slices.Contains(got, request) {
				t.Errorf("%s %q: the requested %s is missing from %v", b.name, query, request, got)
			}
			dropped := slices.DeleteFunc(slices.Clone(first), func(name string) bool { return slices.Contains(got, name) })
			if b.token == 0 && len(dropped) > 1 {
				t.Errorf("%s %q: the request dropped %v", b.name, query, dropped)
			}
			for _, name := range dropped {
				if slices.Contains(run.heldTools(), name) {
					t.Errorf("%s %q: a held tool %s was dropped by rank", b.name, query, name)
				}
			}
		}
	}
}

// A flood of requests costs adaptive tools only: the configured, generator and
// history tools keep their places under every budget. The requests are
// adaptive tools themselves, ranked first up to the adaptive cap.
func TestRefreshKeepsSoftClassesUnderARequestFlood(t *testing.T) {
	ff := allBuiltinToolFeatureFlags()
	ff.LocalWikipediaEnabled = true
	flood := []string{"jellyfin", "proxmox", "truenas", "grafana", "netlify", "github", "ollama", "vercel", "tailscale", "ansible", "home_assistant", "mqtt_publish"}
	for _, b := range selectionBudgets {
		run := startSelectionRun(t, "Erstelle ein Bild von einem Fuchs", ff, b, []string{"local_wikipedia", "docker"})
		first := toolSchemaNames(run.first.Tools)
		got := toolSchemaNames(run.refresh(run.first.Tools, flood))
		for _, name := range run.heldTools() {
			if slices.Contains(first, name) && !slices.Contains(got, name) {
				t.Errorf("%s: held tool %s dropped under the flood: %v", b.name, name, got)
			}
		}
		for _, name := range []string{"generate_image", "local_wikipedia", "docker"} {
			if !slices.Contains(got, name) {
				t.Errorf("%s: %s dropped under the flood: %v", b.name, name, got)
			}
		}
		offered := 0
		for _, name := range flood {
			if slices.Contains(got, name) {
				offered++
			}
		}
		if offered == 0 || offered > b.adaptive {
			t.Errorf("%s: %d of the requests offered, want 1..%d: %v", b.name, offered, b.adaptive, got)
		}
		// Requests rank in the order they arrive: the first one is never cut for a later one.
		if !slices.Contains(got, flood[0]) {
			t.Errorf("%s: the first request %s was cut: %v", b.name, flood[0], got)
		}
	}
}

// Requests of earlier refreshes keep their place when a later request comes:
// every request made so far is offered after each refresh, under every budget,
// however often the model asks for the same tool again.
func TestRefreshChainedRequestsKeepTheEarlierOnes(t *testing.T) {
	ff := allBuiltinToolFeatureFlags()
	for _, b := range selectionBudgets {
		for _, query := range selectionQueries {
			run := startSelectionRun(t, query, ff, b, nil)
			reqs := requestsAbsent(t, run.first.Tools, 3)
			a, second, third := reqs[0], reqs[1], reqs[2]
			r1 := run.refresh(run.first.Tools, []string{a})
			r2 := run.refresh(r1, []string{second})
			r3 := run.refresh(r2, []string{a})
			r4 := run.refresh(r3, []string{third})
			for step, check := range []struct {
				tools []openai.Tool
				want  []string
			}{
				{r1, []string{a}},
				{r2, []string{a, second}},
				{r3, []string{a, second}},
				{r4, []string{a, second, third}},
			} {
				names := toolSchemaNames(check.tools)
				for _, want := range check.want {
					if !slices.Contains(names, want) {
						t.Errorf("%s %q refresh %d: request %s was evicted: %v", b.name, query, step+1, want, names)
					}
				}
			}
			// A refresh without new input changes nothing.
			if again := sortedToolNames(run.refresh(r4, nil)); !slices.Equal(again, sortedToolNames(r4)) {
				t.Errorf("%s %q: a refresh without input changed the selection\nbefore %v\nafter  %v", b.name, query, sortedToolNames(r4), again)
			}
		}
	}
}

// Tools the run used rank behind the requests: after the model used every
// adaptive tool, a request is still offered under every budget, and no soft
// class pays for it.
func TestRefreshRequestSurvivesToolsUsedByTheRun(t *testing.T) {
	ff := allBuiltinToolFeatureFlags()
	for _, b := range selectionBudgets {
		for _, query := range selectionQueries {
			run := startSelectionRun(t, query, ff, b, nil)
			first := toolSchemaNames(run.first.Tools)
			var used []string
			for _, name := range first {
				if !slices.Contains(run.heldTools(), name) {
					used = append(used, name)
				}
			}
			r0 := run.refresh(run.first.Tools, nil, used...)
			if !slices.Equal(sortedToolNames(r0), sortedToolNames(run.first.Tools)) {
				t.Errorf("%s %q: using the adaptive tools changed the selection\nbefore %v\nafter  %v", b.name, query, sortedToolNames(run.first.Tools), sortedToolNames(r0))
			}
			request := requestsAbsent(t, r0, 1)[0]
			r1 := toolSchemaNames(run.refresh(r0, []string{request}))
			if !slices.Contains(r1, request) {
				t.Errorf("%s %q: the request %s was crowded out by %d used tools: %v", b.name, query, request, len(used), r1)
			}
			for _, name := range run.heldTools() {
				if slices.Contains(first, name) && !slices.Contains(r1, name) {
					t.Errorf("%s %q: held tool %s dropped for a request: %v", b.name, query, name, r1)
				}
			}
		}
	}
}

// A long history is soft, but the tools the run uses afterwards are ranked, so
// a request still fits a budget that the history nearly fills.
func TestRefreshRequestFitsAfterLongHistoryAndUse(t *testing.T) {
	ff := allBuiltinToolFeatureFlags()
	schemas := BuildNativeToolSchemas(t.TempDir(), nil, ff, nil)
	var history []string
	for _, name := range toolSchemaNames(schemas) {
		if slices.Contains([]string{"discover_tools", "invoke_tool", "run_tool", "execute_skill"}, name) {
			continue
		}
		if history = append(history, name); len(history) == 3 {
			break
		}
	}
	for _, b := range selectionBudgets {
		run := startSelectionRun(t, "hello there", ff, b, history)
		var used []string
		for _, name := range toolSchemaNames(schemas) {
			if !slices.Contains(history, name) && len(used) < 12 {
				used = append(used, name)
			}
		}
		run.refresh(run.first.Tools, nil, used...)
		request := requestsAbsent(t, run.first.Tools, 1)[0]
		if got := toolSchemaNames(run.refresh(run.first.Tools, []string{request}, used...)); !slices.Contains(got, request) {
			t.Errorf("%s: the request %s is not offered after %d used tools: %v", b.name, request, len(used), got)
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
	// Earlier requests follow the new ones, newest first, while they are candidates.
	opts = adaptiveRefreshFilterOptions(adaptiveRefreshInput{
		Config: cfg, Tools: tools, Requested: []string{"b"}, RequestOrder: []string{"a", "gone", "b"},
		Pinned: map[string]bool{"a": true, "b": true},
	})
	if got := fmt.Sprint(opts.PreferredTools); got != "[b a filesystem a b]" {
		t.Errorf("preferred with earlier requests = %s", got)
	}
	// A first selection that ranked the whole catalog protects the session's tools.
	opts = adaptiveRefreshFilterOptions(adaptiveRefreshInput{Config: cfg, Tools: tools, SessionUsed: map[string]bool{"a": true}, InitFiltered: true})
	if !slices.Equal(opts.SwapProtectedTools, []string{"a"}) {
		t.Errorf("protected = %v", opts.SwapProtectedTools)
	}
}
