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

// adaptiveProbeBudget is one tool budget of the adaptive selection probe.
// initFiltered=false is a run without an adaptive first selection (adaptive
// tools disabled, or no short-term memory): the per-iteration refresh then
// ranks the whole catalog with refreshAdaptive as its adaptive cap.
type adaptiveProbeBudget struct {
	name                   string
	adaptive, total, token int
	initFiltered           bool
	refreshAdaptive        int
	alwaysExtra            []string // appended to the default always_include
}

var adaptiveProbeBudgets = []adaptiveProbeBudget{
	{name: "default 10/20/6500", adaptive: 10, total: 20, token: 6500, initFiltered: true, refreshAdaptive: 10},
	{name: "stability 12/24/6500", adaptive: 12, total: 24, token: 6500, initFiltered: true, refreshAdaptive: 12},
	{name: "roomy 10/40/-", adaptive: 10, total: 40, initFiltered: true, refreshAdaptive: 10},
	{name: "adaptive off 20/6500", total: 20, token: 6500},
	{name: "adaptive off 30/6500", total: 30, token: 6500},
	{name: "adaptive off 400/-", total: 400},
	{name: "no short-term memory 10/20/6500", adaptive: 10, total: 20, token: 6500, refreshAdaptive: 10},
	{name: "small 6/14/6500", adaptive: 6, total: 14, token: 6500, initFiltered: true, refreshAdaptive: 6},
}

// adaptiveSelectionProbe mirrors the production adaptive selection for one
// user message: the first selection of initAgentLoopState (when the budget
// has one) and the per-iteration refresh of ExecuteAgentLoop. Usage history
// is empty; guides is the semantic manual search (nil: unavailable).
func adaptiveSelectionProbe(t *testing.T, query string, ff ToolFeatureFlags, b adaptiveProbeBudget, guides toolGuideSearcher) (initial, refreshed []string) {
	t.Helper()
	cfg := &config.Config{}
	cfg.Agent.AdaptiveTools.Enabled = b.initFiltered || b.refreshAdaptive > 0
	cfg.Agent.AdaptiveTools.MaxTools = b.adaptive
	cfg.Agent.AdaptiveTools.MaxTotalTools = b.total
	cfg.Agent.AdaptiveTools.MaxSchemaTokens = b.token
	if cfg.Agent.AdaptiveTools.Enabled {
		cfg.Agent.AdaptiveTools.AlwaysInclude = append([]string{"filesystem", "query_memory", "manage_memory", "execute_shell"}, b.alwaysExtra...)
	}
	runCfg := RunConfig{Config: cfg}
	schemas := BuildNativeToolSchemas(t.TempDir(), nil, ff, nil)
	hard := channelAdaptiveAlwaysInclude(runCfg, adaptiveHardAlwaysInclude(cfg), ff)
	additive := adaptiveAdditiveToolsForQuery(query)

	first := schemas
	var swapped map[string]string
	if b.initFiltered {
		always := append([]string(nil), cfg.Agent.AdaptiveTools.AlwaysInclude...)
		always = channelAdaptiveAlwaysInclude(runCfg, always, ff)
		always = cacheAwareAdaptiveAlwaysInclude(query, always, schemas)
		always = expandAdaptiveAlwaysInclude(cfg, always)
		result := filterToolSchemasWithReport(schemas, toolSchemaFilterOptions{
			PreferredTools:        buildAdaptiveToolPriority(schemas, nil, query, guides, nil),
			HardAlwaysTools:       hard,
			SoftAlwaysTools:       always,
			MaxAdaptiveTools:      b.adaptive,
			MaxTotalTools:         b.total,
			MaxSchemaTokens:       b.token,
			AdditiveTools:         additive,
			AdaptiveExcludedTools: adaptiveIntentOnlyTools,
			AdditiveSwaps:         adaptiveSwapsForQuery(query),
		}, nil)
		first, swapped = result.Tools, recordAdaptiveSwaps(nil, result.Report)
	}
	candidates := restoreAdaptiveSwapPartners(first, schemas, swapped)
	second := filterToolSchemasWithReport(candidates, toolSchemaFilterOptions{
		PreferredTools:        toolSchemaNames(candidates),
		HardAlwaysTools:       hard,
		SoftAlwaysTools:       cfg.Agent.AdaptiveTools.AlwaysInclude,
		MaxAdaptiveTools:      b.refreshAdaptive,
		MaxTotalTools:         b.total,
		MaxSchemaTokens:       b.token,
		AdditiveTools:         additive,
		AdaptiveExcludedTools: adaptiveRefreshExcludedTools(b.initFiltered, nil),
		AdditiveSwaps:         adaptiveSwapsForQuery(query),
	}, nil).Tools
	return sortedToolNames(first), sortedToolNames(second)
}

func sortedToolNames(schemas []openai.Tool) []string {
	names := toolSchemaNames(schemas)
	slices.Sort(names)
	return names
}

// assertOnlyLocalWikipediaAdded checks that enabling local_wikipedia changed
// a selection only by adding local_wikipedia or by putting it in the place of
// wikipedia_search, and only for an intent query. It reports a swap.
func assertOnlyLocalWikipediaAdded(t *testing.T, label string, baseline, enabled []string, intent bool, total int) (swapped bool) {
	t.Helper()
	without := slices.DeleteFunc(slices.Clone(enabled), func(name string) bool { return name == "local_wikipedia" })
	swapped = len(without) < len(enabled) && len(enabled) == len(baseline) && slices.Contains(baseline, "wikipedia_search") &&
		slices.Equal(without, slices.DeleteFunc(slices.Clone(baseline), func(name string) bool { return name == "wikipedia_search" }))
	if !slices.Equal(without, baseline) && !swapped {
		t.Errorf("%s: enabling local_wikipedia changed the other tools\nenabled  %v\nbaseline %v", label, enabled, baseline)
	}
	if !intent && len(without) != len(enabled) {
		t.Errorf("%s: local_wikipedia offered without an encyclopedia intent", label)
	}
	if total > 0 && len(enabled) > total && len(enabled) > len(baseline) {
		t.Errorf("%s: %d tools exceed the cap of %d", label, len(enabled), total)
	}
	return swapped
}

var localWikipediaProbeQueries = map[string]bool{
	"schick mir den wikipedia artikel über berlin per email": true,
	"lies mir den wikipedia artikel als audio vor":           true,
	"wikipedia proxmox":                           true,
	"ping the wikipedia server":                   true,
	"schedule a daily wikipedia summary":          true,
	"Was steht im Lexikon über die Enzyklopädie?": true,
	"offline wikipedia":                           true,
	"lokale wikipedia":                            true,
	"local wikipedia":                             true,
	"kiwix":                                       true,
	"zeige mir die cpu auslastung":                false,
	"read the server logs and summarise them":     false,
	"starte den docker container neu":             false,
	"search the web for golang news":              false,
	"Bitte nutze die Online-Wikipedia (wikipedia_search), nicht die lokale": true,
	"look it up on wikipedia.org":                                           true,
}

// TestLocalWikipediaOnlyAddsToTheAdaptiveSelection: with local_wikipedia
// enabled a query is offered exactly the tools it gets without it, plus
// local_wikipedia when it asks for Wikipedia or an encyclopedia and the
// budget has room, or with local_wikipedia in the place of a ranked
// wikipedia_search when the selection is full. The baseline does not depend
// on the flag, so it is the selection before the tool existed; the other
// queries cover catalog matches on short words ("die" in "Enzyklopädie",
// "read", "server").
func TestLocalWikipediaOnlyAddsToTheAdaptiveSelection(t *testing.T) {
	swaps := 0
	for flagsName, enabled := range localWikipediaProbeFlagSets() {
		enabled.LocalWikipediaEnabled = true
		baseline := enabled
		baseline.LocalWikipediaEnabled = false
		for _, b := range adaptiveProbeBudgets {
			for query, intent := range localWikipediaProbeQueries {
				label := flagsName + " " + b.name
				baseInitial, baseRefreshed := adaptiveSelectionProbe(t, query, baseline, b, nil)
				initial, refreshed := adaptiveSelectionProbe(t, query, enabled, b, nil)
				named := strings.Contains(query, "(wikipedia_search)") || strings.Contains(query, "wikipedia.org")
				if b.initFiltered && assertOnlyLocalWikipediaAdded(t, label+" initial "+query, baseInitial, initial, intent, b.total) {
					swaps++
					if named {
						t.Errorf("%s initial %q: swapped out the wikipedia_search the message names", label, query)
					}
				}
				if assertOnlyLocalWikipediaAdded(t, label+" refreshed "+query, baseRefreshed, refreshed, intent, b.total) {
					swaps++
					if named {
						t.Errorf("%s refreshed %q: swapped out the wikipedia_search the message names", label, query)
					}
				}
				if intent && b.total >= 40 && b.token == 0 && !containsName(refreshed, "local_wikipedia") {
					t.Errorf("%s %q: local_wikipedia not offered although the budget has room", label, query)
				}
			}
		}
	}
	if swaps == 0 {
		t.Error("no full selection put local_wikipedia in the place of wikipedia_search")
	}
}

// localWikipediaProbeFlagSets are the tool sets the probe runs on: every
// integration, and a small core set in which local_wikipedia sits early in
// the schema order.
func localWikipediaProbeFlagSets() map[string]ToolFeatureFlags {
	return map[string]ToolFeatureFlags{
		"all":  allBuiltinToolFeatureFlags(),
		"bare": {},
		"core": {AllowShell: true, AllowPython: true, AllowFilesystemWrite: true, AllowNetworkRequests: true, MemoryEnabled: true, NotesEnabled: true, SchedulerEnabled: true},
	}
}

// TestLocalWikipediaAliasesKeepTheCatalogRanking: an exact alias message such
// as "offline wikipedia" ranks the catalog as before the alias existed while
// the tool is off, and ranks local_wikipedia first while it is on.
func TestLocalWikipediaAliasesKeepTheCatalogRanking(t *testing.T) {
	off := allBuiltinToolFeatureFlags()
	off.LocalWikipediaEnabled = false
	offSchemas := BuildNativeToolSchemas(t.TempDir(), nil, off, nil)
	onFlags := off
	onFlags.LocalWikipediaEnabled = true
	onSchemas := BuildNativeToolSchemas(t.TempDir(), nil, onFlags, nil)
	aliases := discoverToolNameAliases
	t.Cleanup(func() { discoverToolNameAliases = aliases })
	for alias := range aliases {
		withAlias := catalogNames(BuildToolCatalog(offSchemas, offSchemas, "").Search(alias))
		discoverToolNameAliases = nil
		before := catalogNames(BuildToolCatalog(offSchemas, offSchemas, "").Search(alias))
		discoverToolNameAliases = aliases
		if !slices.Equal(withAlias, before) {
			t.Errorf("%q with the tool off: %v, before the alias %v", alias, withAlias, before)
		}
		for _, b := range adaptiveProbeBudgets {
			discoverToolNameAliases = nil
			_, beforeRefreshed := adaptiveSelectionProbe(t, alias, off, b, nil)
			discoverToolNameAliases = aliases
			_, refreshed := adaptiveSelectionProbe(t, alias, off, b, nil)
			if !slices.Equal(refreshed, beforeRefreshed) {
				t.Errorf("%s %q with the tool off: %v, before the alias %v", b.name, alias, refreshed, beforeRefreshed)
			}
		}
		if on := catalogNames(BuildToolCatalog(onSchemas, onSchemas, "").Search(alias)); len(on) == 0 || on[0] != "local_wikipedia" {
			t.Errorf("%q with the tool on ranks %v", alias, on)
		}
		if got := resolveDiscoverToolName(alias); got != "local_wikipedia" {
			t.Errorf("discover_tools resolves %q to %q", alias, got)
		}
	}
	discoverToolNameAliases = aliases
}

func catalogNames(entries []*ToolCatalogEntry) []string {
	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		names = append(names, entry.Name)
	}
	return names
}

func TestAdaptiveRefreshExcludesIntentOnlyToolsOnlyWithoutAFirstSelection(t *testing.T) {
	if got := adaptiveRefreshExcludedTools(true, nil); got != nil {
		t.Fatalf("after a first selection the refresh excludes %v", got)
	}
	if got := adaptiveRefreshExcludedTools(false, nil); !slices.Equal(got, adaptiveIntentOnlyTools) {
		t.Fatalf("without a first selection the refresh excludes %v", got)
	}
	if got := adaptiveRefreshExcludedTools(false, map[string]bool{"local_wikipedia": true}); len(got) != 0 {
		t.Fatalf("a tool discover_tools requested stays excluded: %v", got)
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

func TestAdditiveToolsTakeNoSlotAndRespectTheCaps(t *testing.T) {
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
	// A full selection skips the additive tool instead of exceeding the cap
	// or displacing another tool.
	if got := strings.Join(toolSchemaNames(filterToolSchemasWithReport(schemas, opts, nil).Tools), ","); got != "hard,soft,a,b" {
		t.Fatalf("additive at the total cap = %s", got)
	}
	opts.MaxTotalTools = 5
	with := filterToolSchemasWithReport(schemas, opts, nil)
	if got := strings.Join(toolSchemaNames(with.Tools), ","); got != "hard,soft,a,b,extra" {
		t.Fatalf("with additive = %s", got)
	}
	opts.MaxSchemaTokens = with.Report.FinalSchemaTokens - 1
	if got := strings.Join(toolSchemaNames(filterToolSchemasWithReport(schemas, opts, nil).Tools), ","); strings.Contains(got, "extra") {
		t.Fatalf("additive exceeded the schema token cap: %s", got)
	}
	if with.Report.KeptAdditive != 1 || with.Report.KeptAdaptive != 2 || with.Report.KeptHardAlways != 1 {
		t.Fatalf("report = %+v", with.Report)
	}
}

func testFilterSchema(name string) openai.Tool {
	return openai.Tool{Type: openai.ToolTypeFunction, Function: &openai.FunctionDefinition{Name: name, Description: name}}
}

func sizedFilterSchema(name string, descriptionBytes int) openai.Tool {
	return openai.Tool{Type: openai.ToolTypeFunction, Function: &openai.FunctionDefinition{Name: name, Description: strings.Repeat("x", descriptionBytes)}}
}

func TestAdditiveToolTakesOnlyItsRankedSwapPartnersPlace(t *testing.T) {
	schemas := []openai.Tool{
		testFilterSchema("hard"), testFilterSchema("soft"), testFilterSchema("wikipedia_search"),
		testFilterSchema("a"), testFilterSchema("b"), testFilterSchema("local_wikipedia"),
	}
	base := toolSchemaFilterOptions{
		PreferredTools:   []string{"a", "wikipedia_search", "b"},
		HardAlwaysTools:  []string{"hard"},
		SoftAlwaysTools:  []string{"soft"},
		MaxAdaptiveTools: 3,
		MaxTotalTools:    5,
		AdditiveTools:    []string{"local_wikipedia"},
	}
	names := func(opts toolSchemaFilterOptions) (string, toolSchemaFilterReport) {
		result := filterToolSchemasWithReport(schemas, opts, nil)
		return strings.Join(toolSchemaNames(result.Tools), ","), result.Report
	}
	if got, _ := names(base); got != "hard,soft,a,wikipedia_search,b" {
		t.Fatalf("without swaps = %s", got)
	}
	swapping := base
	swapping.AdditiveSwaps = adaptiveAdditiveSwaps
	got, report := names(swapping)
	if got != "hard,soft,a,local_wikipedia,b" {
		t.Fatalf("full selection = %s, want local_wikipedia in the place of wikipedia_search", got)
	}
	if report.KeptAdditive != 1 || report.KeptAdaptive != 2 || !slices.Equal(report.SwappedTools, []string{"wikipedia_search->local_wikipedia"}) {
		t.Fatalf("report = %+v", report)
	}
	roomy := swapping
	roomy.MaxTotalTools = 6
	if got, _ := names(roomy); got != "hard,soft,a,wikipedia_search,b,local_wikipedia" {
		t.Fatalf("with room = %s, want an addition", got)
	}
	for name, change := range map[string]func(*toolSchemaFilterOptions){
		"pinned": func(o *toolSchemaFilterOptions) { o.PinnedTools = []string{"wikipedia_search"} },
		"soft":   func(o *toolSchemaFilterOptions) { o.SoftAlwaysTools = []string{"soft", "wikipedia_search"} },
		"hard":   func(o *toolSchemaFilterOptions) { o.HardAlwaysTools = []string{"hard", "wikipedia_search"} },
		"not ranked": func(o *toolSchemaFilterOptions) {
			o.PreferredTools = []string{"a", "b"}
			o.MaxAdaptiveTools = 2
			o.MaxTotalTools = 4
		},
		"no additive": func(o *toolSchemaFilterOptions) { o.AdditiveTools = nil },
	} {
		opts := swapping
		opts.PinnedTools, opts.SoftAlwaysTools, opts.HardAlwaysTools = nil, []string{"soft"}, []string{"hard"}
		change(&opts)
		if got, report := names(opts); strings.Contains(got, "local_wikipedia") || len(report.SwappedTools) != 0 {
			t.Errorf("%s: swapped anyway: %s", name, got)
		}
	}
}

func TestAdditiveSwapKeepsTheSchemaTokenCap(t *testing.T) {
	schemas := []openai.Tool{sizedFilterSchema("wikipedia_search", 40), sizedFilterSchema("a", 40), sizedFilterSchema("local_wikipedia", 400)}
	opts := toolSchemaFilterOptions{
		PreferredTools: []string{"wikipedia_search", "a"}, MaxTotalTools: 2,
		AdditiveTools: []string{"local_wikipedia"}, AdditiveSwaps: adaptiveAdditiveSwaps,
	}
	tokens := func(names ...string) int {
		sum := 0
		for _, s := range schemas {
			if slices.Contains(names, s.Function.Name) {
				sum += estimateSingleToolSchemaTokens(s)
			}
		}
		return sum
	}
	opts.MaxSchemaTokens = tokens("local_wikipedia", "a") - 1
	if got := strings.Join(toolSchemaNames(filterToolSchemasWithReport(schemas, opts, nil).Tools), ","); got != "wikipedia_search,a" {
		t.Fatalf("a swap past the schema token cap happened: %s", got)
	}
	opts.MaxSchemaTokens = tokens("local_wikipedia", "a")
	if got := strings.Join(toolSchemaNames(filterToolSchemasWithReport(schemas, opts, nil).Tools), ","); got != "local_wikipedia,a" {
		t.Fatalf("a swap within the schema token cap was skipped: %s", got)
	}
}

func TestRestoreAdaptiveSwapPartnersPutsTheReplacedToolBack(t *testing.T) {
	all := []openai.Tool{testFilterSchema("a"), testFilterSchema("wikipedia_search"), testFilterSchema("local_wikipedia"), testFilterSchema("b")}
	selected := []openai.Tool{testFilterSchema("a"), testFilterSchema("local_wikipedia"), testFilterSchema("b")}
	swapped := recordAdaptiveSwaps(nil, toolSchemaFilterReport{SwappedTools: []string{"wikipedia_search->local_wikipedia"}})
	if got := strings.Join(toolSchemaNames(restoreAdaptiveSwapPartners(selected, all, swapped)), ","); got != "a,wikipedia_search,local_wikipedia,b" {
		t.Fatalf("restored = %s", got)
	}
	if got := strings.Join(toolSchemaNames(restoreAdaptiveSwapPartners(selected, all, nil)), ","); got != "a,local_wikipedia,b" {
		t.Fatalf("restored without a swap = %s", got)
	}
	if got := strings.Join(toolSchemaNames(restoreAdaptiveSwapPartners(selected, all[2:], swapped)), ","); got != "a,local_wikipedia,b" {
		t.Fatalf("restored a tool that is gone = %s", got)
	}
	if got := pinnedToolNames(map[string]bool{"b": true, "a": true}); !slices.Equal(got, []string{"a", "b"}) {
		t.Fatalf("pinned = %v", got)
	}
}

// A kept local_wikipedia (always_include, session use, discover_tools at the
// first selection) stays a soft tool on an encyclopedia question instead of
// being demoted to an additive tool and cut by a full selection.
func TestKeptLocalWikipediaStaysOnEncyclopediaQuestions(t *testing.T) {
	enabled := allBuiltinToolFeatureFlags()
	enabled.LocalWikipediaEnabled = true
	kept := 0
	for _, b := range adaptiveProbeBudgets {
		b.alwaysExtra = []string{"local_wikipedia"}
		if !b.initFiltered && b.refreshAdaptive == 0 {
			continue // adaptive tools off: always_include has no defaults to extend
		}
		for query, intent := range localWikipediaProbeQueries {
			if !intent {
				continue
			}
			initial, refreshed := adaptiveSelectionProbe(t, query, enabled, b, nil)
			if b.initFiltered && !containsName(initial, "local_wikipedia") {
				t.Errorf("%s initial %q: always-included local_wikipedia dropped", b.name, query)
			}
			if !containsName(refreshed, "local_wikipedia") {
				t.Errorf("%s refreshed %q: always-included local_wikipedia dropped", b.name, query)
			}
			kept++
		}
	}
	if kept == 0 {
		t.Fatal("no budget exercised")
	}

	// local_wikipedia used earlier in the conversation, plus tools requested
	// through discover_tools during the run: the first selection keeps it as
	// a soft tool and so does the refresh, which runs before every model
	// call. Like any session-kept tool it occupies one slot and its schema
	// tokens; with exactly that room added, the selection is the one without
	// the tool plus local_wikipedia, so it costs nothing else.
	disabled := enabled
	disabled.LocalWikipediaEnabled = false
	lwTokens := 0
	for _, schema := range BuildNativeToolSchemas(t.TempDir(), nil, enabled, nil) {
		if schema.Function.Name == "local_wikipedia" {
			lwTokens = estimateSingleToolSchemaTokens(schema)
		}
	}
	history := []string{"local_wikipedia"}
	for _, b := range adaptiveProbeBudgets {
		if !b.initFiltered {
			continue
		}
		room := b
		if room.total > 0 {
			room.total++
		}
		if room.token > 0 {
			room.token += lwTokens
		}
		for query := range localWikipediaProbeQueries {
			for _, requested := range [][]string{nil, {"jellyfin"}, {"jellyfin", "proxmox", "truenas", "grafana", "netlify", "github"}} {
				label := b.name + " " + query
				initial, refreshed := adaptiveSessionProbe(t, query, enabled, b, history, requested)
				if !containsName(initial, "local_wikipedia") || !containsName(refreshed, "local_wikipedia") {
					t.Errorf("%s req=%v: session-kept local_wikipedia cut (initial %v, refreshed %v)", label, requested,
						containsName(initial, "local_wikipedia"), containsName(refreshed, "local_wikipedia"))
				}
				offInitial, offRefreshed := adaptiveSessionProbe(t, query, disabled, b, history, requested)
				roomInitial, roomRefreshed := adaptiveSessionProbe(t, query, enabled, room, history, requested)
				for stage, pair := range map[string][2][]string{"initial": {offInitial, roomInitial}, "refreshed": {offRefreshed, roomRefreshed}} {
					want := append(slices.Clone(pair[0]), "local_wikipedia")
					slices.Sort(want)
					if !slices.Equal(pair[1], want) {
						t.Errorf("%s req=%v %s: a session-kept local_wikipedia cost more than its own slot\nwith it %v\nwithout %v", label, requested, stage, pair[1], pair[0])
					}
				}
			}
		}
	}
}

func TestPinnedOrSoftLocalWikipediaIsNotAdditive(t *testing.T) {
	schemas := []openai.Tool{testFilterSchema("hard"), testFilterSchema("a"), testFilterSchema("b"), testFilterSchema("local_wikipedia"), testFilterSchema("wikipedia_search")}
	base := toolSchemaFilterOptions{
		HardAlwaysTools:  []string{"hard"},
		MaxAdaptiveTools: 2,
		MaxTotalTools:    3,
		AdditiveTools:    []string{"local_wikipedia"},
		AdditiveSwaps:    adaptiveAdditiveSwaps,
	}
	soft := base
	soft.PreferredTools = []string{"a", "b"}
	soft.SoftAlwaysTools = []string{"local_wikipedia"}
	result := filterToolSchemasWithReport(schemas, soft, nil)
	if got := strings.Join(toolSchemaNames(result.Tools), ","); got != "hard,local_wikipedia,a" || result.Report.KeptSoftAlways != 1 || result.Report.KeptAdditive != 0 {
		t.Fatalf("soft local_wikipedia = %s %+v", got, result.Report)
	}
	pinned := base
	pinned.PreferredTools = []string{"local_wikipedia", "a", "b"}
	pinned.PinnedTools = []string{"local_wikipedia"}
	result = filterToolSchemasWithReport(schemas, pinned, nil)
	if got := strings.Join(toolSchemaNames(result.Tools), ","); got != "hard,local_wikipedia,a" || result.Report.KeptAdaptive != 2 || result.Report.KeptAdditive != 0 {
		t.Fatalf("requested local_wikipedia = %s %+v", got, result.Report)
	}
}

func TestSwapKeepsNamedOrSessionKeptWikipediaSearch(t *testing.T) {
	for query, allowed := range map[string]bool{
		"Was sagt Wikipedia über Berlin?":                                       true,
		"Bitte nutze die Online-Wikipedia (wikipedia_search), nicht die lokale": false,
		"use the online wikipedia":                                              false,
		"check wikipedia.org for this":                                          false,
		"WIKIPEDIA_SEARCH please":                                               false,
	} {
		if got := adaptiveSwapsForQuery(query) != nil; got != allowed {
			t.Errorf("%q: swap allowed = %v, want %v", query, got, allowed)
		}
	}
	schemas := []openai.Tool{testFilterSchema("a"), testFilterSchema("wikipedia_search"), testFilterSchema("local_wikipedia")}
	opts := toolSchemaFilterOptions{
		PreferredTools: []string{"a", "wikipedia_search"}, MaxTotalTools: 2,
		AdditiveTools: []string{"local_wikipedia"}, AdditiveSwaps: adaptiveAdditiveSwaps,
	}
	if got := strings.Join(toolSchemaNames(filterToolSchemasWithReport(schemas, opts, nil).Tools), ","); got != "a,local_wikipedia" {
		t.Fatalf("unprotected swap = %s", got)
	}
	opts.SwapProtectedTools = []string{"wikipedia_search"}
	if got := strings.Join(toolSchemaNames(filterToolSchemasWithReport(schemas, opts, nil).Tools), ","); got != "a,wikipedia_search" {
		t.Fatalf("session-kept wikipedia_search swapped out: %s", got)
	}
	if got := refreshSwapProtectedTools(false, map[string]bool{"wikipedia_search": true}, nil); got != nil {
		t.Fatalf("without a first selection the refresh protects %v", got)
	}
	if got := refreshSwapProtectedTools(true, map[string]bool{"wikipedia_search": true}, []string{"docker", "wikipedia_search"}); !slices.Equal(got, []string{"docker", "wikipedia_search"}) {
		t.Fatalf("protected = %v", got)
	}
}

// adaptiveSessionProbe mirrors the first selection and the refresh like
// adaptiveSelectionProbe, for a conversation whose history used the recent
// tools and a run in which discover_tools requested further tools before the
// refresh (the refresh runs before every model call, the first included).
func adaptiveSessionProbe(t *testing.T, query string, ff ToolFeatureFlags, b adaptiveProbeBudget, recent, requested []string) (initial, refreshed []string) {
	t.Helper()
	cfg := &config.Config{}
	cfg.Agent.AdaptiveTools.Enabled = true
	cfg.Agent.AdaptiveTools.MaxTools = b.adaptive
	cfg.Agent.AdaptiveTools.MaxTotalTools = b.total
	cfg.Agent.AdaptiveTools.MaxSchemaTokens = b.token
	cfg.Agent.AdaptiveTools.AlwaysInclude = append([]string{"filesystem", "query_memory", "manage_memory", "execute_shell"}, b.alwaysExtra...)
	runCfg := RunConfig{Config: cfg}
	schemas := BuildNativeToolSchemas(t.TempDir(), nil, ff, nil)
	hard := channelAdaptiveAlwaysInclude(runCfg, adaptiveHardAlwaysInclude(cfg), ff)
	additive, swaps := adaptiveAdditiveToolsForQuery(query), adaptiveSwapsForQuery(query)

	always := append([]string(nil), cfg.Agent.AdaptiveTools.AlwaysInclude...)
	always = channelAdaptiveAlwaysInclude(runCfg, always, ff)
	always = cacheAwareAdaptiveAlwaysInclude(query, always, schemas)
	always = append(always, recent...)
	always = expandAdaptiveAlwaysInclude(cfg, always)
	first := filterToolSchemasWithReport(schemas, toolSchemaFilterOptions{
		PreferredTools:        buildAdaptiveToolPriority(schemas, nil, query, nil, nil),
		HardAlwaysTools:       hard,
		SoftAlwaysTools:       always,
		MaxAdaptiveTools:      b.adaptive,
		MaxTotalTools:         b.total,
		MaxSchemaTokens:       b.token,
		AdditiveTools:         additive,
		AdaptiveExcludedTools: adaptiveIntentOnlyTools,
		AdditiveSwaps:         swaps,
	}, nil)

	candidates := restoreAdaptiveSwapPartners(first.Tools, schemas, recordAdaptiveSwaps(nil, first.Report))
	present := stringSet(toolSchemaNames(candidates))
	requestedSet := map[string]bool{}
	for _, name := range requested {
		requestedSet[name] = true
	}
	for _, schema := range schemas {
		if requestedSet[schema.Function.Name] && !present[schema.Function.Name] {
			candidates = append(candidates, schema)
		}
	}
	kept := refreshSwapProtectedTools(true, nil, recent)
	soft, refreshAdditive := refreshSoftAndAdditiveTools(cfg.Agent.AdaptiveTools.AlwaysInclude, additive, kept, requestedSet)
	second := filterToolSchemasWithReport(candidates, toolSchemaFilterOptions{
		PreferredTools:        append(slices.Clone(requested), toolSchemaNames(candidates)...),
		HardAlwaysTools:       hard,
		SoftAlwaysTools:       soft,
		MaxAdaptiveTools:      b.refreshAdaptive,
		MaxTotalTools:         b.total,
		MaxSchemaTokens:       b.token,
		AdditiveTools:         refreshAdditive,
		AdaptiveExcludedTools: adaptiveRefreshExcludedTools(true, requestedSet),
		AdditiveSwaps:         swaps,
		PinnedTools:           pinnedToolNames(requestedSet),
		SwapProtectedTools:    kept,
	}, nil)
	return sortedToolNames(first.Tools), sortedToolNames(second.Tools)
}
