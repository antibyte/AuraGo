package agent

import (
	"slices"
	"strings"
	"testing"

	"aurago/internal/config"
	openai "github.com/sashabaranov/go-openai"
)

func TestLocalWikipediaDiscoverySurfaces(t *testing.T) {
	if ToolCategoryForName("local_wikipedia") != "network" {
		t.Fatalf("category = %q", ToolCategoryForName("local_wikipedia"))
	}
	for _, alias := range []string{"offline wikipedia", "Lokale Wikipedia", "kiwix"} {
		if got := resolveDiscoverToolName(alias); got != "local_wikipedia" {
			t.Fatalf("alias %q resolves to %q", alias, got)
		}
	}
	if resolveDiscoverToolName("wikipedia") != "wikipedia_search" {
		t.Fatal("the online wikipedia alias must stay unchanged")
	}
	if classifyToolFamily("local_wikipedia") != "web" {
		t.Fatal("local_wikipedia must belong to the web family")
	}
	// Mentioning Wikipedia must not decide a query's family: the family picks
	// the seed bundle, and the web bundle would replace e.g. the
	// communication tools of "send me the wikipedia article by email".
	if got := inferToolFamilyFromQuery("lies den wikipedia artikel"); got != "" {
		t.Fatalf("wikipedia query family = %q, want none", got)
	}
	for query, want := range map[string]string{
		"schick mir den wikipedia artikel über berlin per email": inferToolFamilyFromQuery("schick mir den artikel über berlin per email"),
		"wikipedia proxmox":                  "infra",
		"ping the wikipedia server":          "network",
		"schedule a daily wikipedia summary": "automation",
	} {
		if got := inferToolFamilyFromQuery(query); got != want {
			t.Fatalf("family(%q) = %q, want %q", query, got, want)
		}
	}
	if additive := adaptiveAdditiveToolsForQuery("Was steht im Lexikon über die Enzyklopädie?"); !containsName(additive, "local_wikipedia") {
		t.Fatalf("additive tools = %v", additive)
	}
	if seeds := adaptiveFamilySeedsForQuery("Was steht im Lexikon über die Enzyklopädie?"); containsName(seeds, "local_wikipedia") {
		t.Fatalf("local_wikipedia must not be a capped family seed: %v", seeds)
	}
	if additive := adaptiveAdditiveToolsForQuery("starte den docker container neu"); len(additive) != 0 {
		t.Fatalf("unrelated query offers %v", additive)
	}
}

// adaptiveSelectionProbe mirrors the production adaptive selection for one
// user message: the initial filter of initAgentLoopState with the default
// budget (10 adaptive, 20 total, 6500 schema tokens, default always-include)
// and the per-iteration refresh of ExecuteAgentLoop. Usage history and the
// semantic guide search are empty.
func adaptiveSelectionProbe(t *testing.T, query string, ff ToolFeatureFlags) (initial, refreshed []string) {
	t.Helper()
	cfg := &config.Config{}
	cfg.Agent.AdaptiveTools.Enabled = true
	cfg.Agent.AdaptiveTools.MaxTools = 10
	cfg.Agent.AdaptiveTools.MaxTotalTools = 20
	cfg.Agent.AdaptiveTools.MaxSchemaTokens = 6500
	cfg.Agent.AdaptiveTools.AlwaysInclude = []string{"filesystem", "query_memory", "manage_memory", "execute_shell"}
	runCfg := RunConfig{Config: cfg}
	schemas := BuildNativeToolSchemas(t.TempDir(), nil, ff, nil)
	hard := channelAdaptiveAlwaysInclude(runCfg, adaptiveHardAlwaysInclude(cfg), ff)

	always := append([]string(nil), cfg.Agent.AdaptiveTools.AlwaysInclude...)
	always = channelAdaptiveAlwaysInclude(runCfg, always, ff)
	always = cacheAwareAdaptiveAlwaysInclude(query, always, schemas)
	always = expandAdaptiveAlwaysInclude(cfg, always)
	additive := adaptiveAdditiveToolsForQuery(query)
	first := filterToolSchemasWithReport(schemas, toolSchemaFilterOptions{
		PreferredTools:        buildAdaptiveToolPriority(schemas, nil, query, nil, nil),
		HardAlwaysTools:       hard,
		SoftAlwaysTools:       always,
		MaxAdaptiveTools:      cfg.Agent.AdaptiveTools.MaxTools,
		MaxTotalTools:         cfg.Agent.AdaptiveTools.MaxTotalTools,
		MaxSchemaTokens:       cfg.Agent.AdaptiveTools.MaxSchemaTokens,
		AdditiveTools:         additive,
		AdaptiveExcludedTools: adaptiveIntentOnlyTools,
	}, nil).Tools
	second := filterToolSchemasWithReport(first, toolSchemaFilterOptions{
		PreferredTools:   toolSchemaNames(first),
		HardAlwaysTools:  hard,
		SoftAlwaysTools:  cfg.Agent.AdaptiveTools.AlwaysInclude,
		MaxAdaptiveTools: cfg.Agent.AdaptiveTools.MaxTools,
		MaxTotalTools:    cfg.Agent.AdaptiveTools.MaxTotalTools,
		MaxSchemaTokens:  cfg.Agent.AdaptiveTools.MaxSchemaTokens,
		AdditiveTools:    additive,
	}, nil).Tools
	return sortedToolNames(first), sortedToolNames(second)
}

func sortedToolNames(schemas []openai.Tool) []string {
	names := toolSchemaNames(schemas)
	slices.Sort(names)
	return names
}

// TestLocalWikipediaOnlyAddsToTheAdaptiveSelection: with local_wikipedia
// enabled a query is offered exactly the tools it gets without it, plus
// local_wikipedia when it asks for Wikipedia or an encyclopedia. The
// baseline does not depend on the flag, so it is the selection before the
// tool existed; the other queries cover catalog matches on short words
// ("die" in "Enzyklopädie", "read", "server").
func TestLocalWikipediaOnlyAddsToTheAdaptiveSelection(t *testing.T) {
	enabled := allBuiltinToolFeatureFlags()
	enabled.LocalWikipediaEnabled = true
	baseline := enabled
	baseline.LocalWikipediaEnabled = false
	for query, offered := range map[string]bool{
		"schick mir den wikipedia artikel über berlin per email": true,
		"lies mir den wikipedia artikel als audio vor":           true,
		"wikipedia proxmox":                           true,
		"ping the wikipedia server":                   true,
		"schedule a daily wikipedia summary":          true,
		"Was steht im Lexikon über die Enzyklopädie?": true,
		"zeige mir die cpu auslastung":                false,
		"read the server logs and summarise them":     false,
		"starte den docker container neu":             false,
		"search the web for golang news":              false,
	} {
		baseInitial, baseRefreshed := adaptiveSelectionProbe(t, query, baseline)
		initial, refreshed := adaptiveSelectionProbe(t, query, enabled)
		for stage, pair := range map[string][2][]string{"initial": {baseInitial, initial}, "refreshed": {baseRefreshed, refreshed}} {
			want := slices.Clone(pair[0])
			if offered {
				want = append(want, "local_wikipedia")
				slices.Sort(want)
			}
			if !slices.Equal(pair[1], want) {
				t.Errorf("%q %s: enabled offers %v\nbaseline offers %v", query, stage, pair[1], pair[0])
			}
		}
	}
}

func TestAdaptiveExcludedToolsComeOnlyAsSoftOrAdditive(t *testing.T) {
	schemas := []openai.Tool{testFilterSchema("wiki"), testFilterSchema("a"), testFilterSchema("b")}
	opts := toolSchemaFilterOptions{PreferredTools: []string{"wiki", "a", "b"}, MaxAdaptiveTools: 2, AdaptiveExcludedTools: []string{"wiki"}}
	if got := strings.Join(toolSchemaNames(filterToolSchemasWithReport(schemas, opts, nil).Tools), ","); got != "a,b" {
		t.Fatalf("excluded tool ranked: %s", got)
	}
	opts.SoftAlwaysTools = []string{"wiki"}
	if got := strings.Join(toolSchemaNames(filterToolSchemasWithReport(schemas, opts, nil).Tools), ","); got != "wiki,a,b" {
		t.Fatalf("soft excluded tool dropped: %s", got)
	}
}

func TestAdditiveToolsBypassTheCapsWithoutTakingASlot(t *testing.T) {
	schemas := []openai.Tool{
		testFilterSchema("hard"), testFilterSchema("soft"), testFilterSchema("extra"),
		testFilterSchema("a"), testFilterSchema("b"), testFilterSchema("c"),
	}
	opts := toolSchemaFilterOptions{
		PreferredTools:   []string{"extra", "a", "b", "c"},
		HardAlwaysTools:  []string{"hard"},
		SoftAlwaysTools:  []string{"soft"},
		MaxAdaptiveTools: 2,
		MaxTotalTools:    4,
	}
	without := filterToolSchemasWithReport(schemas, opts, nil)
	if got := strings.Join(toolSchemaNames(without.Tools), ","); got != "hard,soft,extra,a" {
		t.Fatalf("without additive = %s", got)
	}
	opts.AdditiveTools = []string{"extra", "missing", "hard"}
	with := filterToolSchemasWithReport(schemas, opts, nil)
	if got := strings.Join(toolSchemaNames(with.Tools), ","); got != "hard,soft,a,b,extra" {
		t.Fatalf("with additive = %s", got)
	}
	if with.Report.KeptAdditive != 1 || with.Report.KeptAdaptive != 2 || with.Report.KeptHardAlways != 1 {
		t.Fatalf("report = %+v", with.Report)
	}
}

func testFilterSchema(name string) openai.Tool {
	return openai.Tool{Type: openai.ToolTypeFunction, Function: &openai.FunctionDefinition{Name: name, Description: name}}
}
