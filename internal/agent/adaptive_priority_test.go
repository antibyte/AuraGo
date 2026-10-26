package agent

import (
	"slices"
	"strings"
	"testing"

	"aurago/internal/config"
	openai "github.com/sashabaranov/go-openai"
)

func TestFuseToolRankingSignals(t *testing.T) {
	cfg := adaptiveToolFusion
	for _, tc := range []struct {
		name     string
		pinned   []string
		lexical  []string
		signals  []toolRankSignal
		expected []string
	}{
		{
			name:     "a signal raises a lexical hit",
			lexical:  []string{"l1", "l2", "l3"},
			signals:  []toolRankSignal{{Names: []string{"l3"}, Weight: 0.7}},
			expected: []string{"l3", "l1", "l2"},
		},
		{
			name:     "a tool only one signal ranks follows the lexical hits",
			lexical:  []string{"l1", "l2"},
			signals:  []toolRankSignal{{Names: []string{"s1"}, Weight: 0.7}},
			expected: []string{"l1", "l2", "s1"},
		},
		{
			name:    "MaxAdded bounds the tools a signal adds, not the ones it raises",
			lexical: []string{"l1"},
			signals: []toolRankSignal{{Names: []string{"x1", "l1", "x2", "x3"}, Weight: 0.3, MaxAdded: 2}},
			// l1 is raised without using the budget; x1 and x2 are added, x3 is not.
			expected: []string{"l1", "x1", "x2"},
		},
		{
			name:     "pinned names and repeats take no position",
			pinned:   []string{"p"},
			lexical:  []string{"l1"},
			signals:  []toolRankSignal{{Names: []string{"p", "a", "a", "b"}, Weight: 0.5}},
			expected: []string{"p", "l1", "a", "b"},
		},
		{
			name:    "signals add up and ties keep the signal order",
			lexical: nil,
			signals: []toolRankSignal{
				{Names: []string{"a", "b"}, Weight: 0.5},
				{Names: []string{"b", "a"}, Weight: 0.5},
			},
			// a: 0.5/21 + 0.5/22, b: 0.5/22 + 0.5/21; the same score, so a (seen first) leads.
			expected: []string{"a", "b"},
		},
		{
			name:     "a signal without names changes nothing",
			lexical:  []string{"l1", "l2"},
			signals:  []toolRankSignal{{Weight: 0.7}, {Names: []string{""}, Weight: 0.7}},
			expected: []string{"l1", "l2"},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := fuseToolRanking(tc.pinned, tc.lexical, nil, nil, nil, cfg, tc.signals...); !slices.Equal(got, tc.expected) {
				t.Fatalf("fuseToolRanking = %v, want %v", got, tc.expected)
			}
		})
	}
	// Without signals the call is the one discover_tools makes.
	lexical := []string{"l1", "l2", "l3"}
	if got, want := fuseToolRanking(nil, lexical, nil, nil, nil, cfg), fuseToolRanking(nil, lexical, nil, nil, nil, cfg, []toolRankSignal{}...); !slices.Equal(got, want) || !slices.Equal(got, lexical) {
		t.Fatalf("no signals = %v, want %v", got, lexical)
	}
}

// The signals only decide among tools the text does not separate: every
// weight is below the lexical weight, so a tool that just one signal ranks can
// never pass the best lexical hit, whatever the signal puts first.
func TestSingleSignalNeverOutranksTheLexicalLeader(t *testing.T) {
	cfg := adaptiveToolFusion
	for name, weight := range map[string]float64{
		"intent": adaptiveIntentWeight, "seed": adaptiveSeedWeight, "usage": adaptiveUsageWeight,
	} {
		if weight <= 0 || weight >= cfg.LexicalWeight {
			t.Errorf("%s weight %v must be positive and below the lexical weight %v", name, weight, cfg.LexicalWeight)
		}
		lexical := []string{"leader", "l2", "l3", "l4", "l5"}
		got := fuseToolRanking(nil, lexical, nil, nil, nil, cfg, toolRankSignal{Names: []string{"x1", "x2", "x3"}, Weight: weight})
		if got[0] != "leader" {
			t.Errorf("%s: a signal-only tool passed the lexical leader: %v", name, got)
		}
	}
}

func TestKeepLexicalLeader(t *testing.T) {
	order := []string{"p", "a", "b", "c", "leader", "d"}
	for _, tc := range []struct {
		name   string
		order  []string
		exact  int
		leader string
		places int
		want   []string
	}{
		{name: "moves the leader up to the last place", order: order, exact: 1, leader: "leader", places: 3, want: []string{"p", "a", "b", "leader", "c", "d"}},
		{name: "no exact matches", order: order, exact: 0, leader: "leader", places: 3, want: []string{"p", "a", "leader", "b", "c", "d"}},
		{name: "a leader within its places stays", order: order, exact: 1, leader: "c", places: 3, want: order},
		{name: "a leader that is not ranked changes nothing", order: order, exact: 1, leader: "missing", places: 3, want: order},
		{name: "a short order changes nothing", order: []string{"a", "leader"}, exact: 2, leader: "leader", places: 3, want: []string{"a", "leader"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			before := slices.Clone(tc.order)
			got := keepLexicalLeader(tc.order, tc.exact, tc.leader, tc.places)
			if !slices.Equal(got, tc.want) {
				t.Fatalf("keepLexicalLeader = %v, want %v", got, tc.want)
			}
			if !slices.Equal(tc.order, before) {
				t.Fatalf("the input order was modified: %v", tc.order)
			}
		})
	}
}

// However the signals fall, the best lexical hit of the text keeps a place
// among the first three tools behind the exact matches, and a tool that only
// one signal ranks stays behind it. The adversarial usage list ranks the
// lexical hits two to nine first, which pushes the leader far down the fused
// order without the guard.
func TestBuildAdaptiveToolPriorityKeepsTheLexicalLeaderUpFront(t *testing.T) {
	schemas := BuildNativeToolSchemas("", nil, allBuiltinToolFeatureFlags(), nil)
	catalog := newToolCatalog(schemas, schemas, "", false)
	for _, query := range []string{
		"send an email to the team",
		"Kopiere die Datei in einen anderen Ordner",
		"read the file and search the folder for text",
		"send a message to a contact by email or sms",
		"schedule a task and send a notification",
		"play music on the speaker and convert audio",
		"Datei lesen und Ordner durchsuchen",
	} {
		var pinned, lexical []string
		for _, hit := range catalog.rankedSearch(query) {
			if slices.Contains(adaptiveIntentOnlyTools, hit.Entry.Name) {
				continue
			}
			if hit.Tier > toolSearchTierLexical {
				pinned = append(pinned, hit.Entry.Name)
			} else {
				lexical = append(lexical, hit.Entry.Name)
			}
		}
		if len(lexical) < 10 {
			t.Fatalf("%q: the fixture needs ten lexical hits, got %v", query, lexical)
		}
		leader := lexical[0]

		// Usage that ranks the runners-up first.
		usage := slices.Clone(lexical[1:10])
		got := buildAdaptiveToolPriority(schemas, usage, query, nil, nil)
		// Three places behind the exact matches (adaptiveLeaderPlaces).
		if at := slices.Index(got, leader); at < 0 || at > len(pinned)+2 {
			t.Errorf("%q: the lexical leader %s is at %d of %v", query, leader, at, got[:min(12, len(got))])
		}

		// Usage that ranks tools the text never found: they follow the leader.
		var unrelated []string
		for _, name := range toolSchemaNames(schemas) {
			if !slices.Contains(lexical, name) && !slices.Contains(pinned, name) && !slices.Contains(adaptiveIntentOnlyTools, name) {
				unrelated = append(unrelated, name)
			}
			if len(unrelated) == 8 {
				break
			}
		}
		got = buildAdaptiveToolPriority(schemas, unrelated, query, nil, nil)
		leaderAt := slices.Index(got, leader)
		for _, name := range unrelated[:adaptiveUsageMaxAdded] {
			if at := slices.Index(got, name); at >= 0 && at < leaderAt {
				t.Errorf("%q: usage-only tool %s (%d) passed the lexical leader %s (%d)", query, name, at, leader, leaderAt)
			}
		}
	}
}

// Family seeds compete for the adaptive slots: a request that makes more seeds
// likely than there are slots still gets no more of them than MaxAdaptiveTools,
// and none is kept as a soft tool. The planner of the first selection decides.
func TestFamilySeedsCountAgainstTheAdaptiveCap(t *testing.T) {
	ff := allBuiltinToolFeatureFlags()
	cfg := &config.Config{}
	cfg.Agent.AdaptiveTools.AlwaysInclude = []string{"filesystem"}
	schemas := BuildNativeToolSchemas("", nil, ff, nil)
	const query = "Zeig mir die Datei im Ordner und führe den Befehl aus, danach die Container und die CPU"
	const maxAdaptive = 5
	plan := planAdaptiveFirstSelection(adaptiveSelectionInput{
		Config: cfg, Features: ff, Schemas: schemas, Query: query, MaxAdaptive: maxAdaptive, MaxTotal: 40,
	})
	if len(plan.Seeds) <= maxAdaptive {
		t.Fatalf("the fixture needs more seeds than adaptive slots, got %v", plan.Seeds)
	}
	for _, seed := range plan.Seeds {
		if slices.Contains(plan.Options.SoftAlwaysTools, seed) {
			t.Errorf("seed %s is a soft tool: %v", seed, plan.Options.SoftAlwaysTools)
		}
		if !slices.Contains(plan.Priority, seed) {
			t.Errorf("seed %s is not ranked: %v", seed, plan.Priority)
		}
	}
	result := filterToolSchemasWithReport(schemas, plan.Options, nil)
	offered := toolSchemaNames(result.Tools)
	seedsOffered := 0
	for _, seed := range plan.Seeds {
		if slices.Contains(offered, seed) {
			seedsOffered++
		}
	}
	if seedsOffered > maxAdaptive || result.Report.KeptAdaptive != maxAdaptive {
		t.Fatalf("%d of %d seeds offered, adaptive tools %d, want at most %d: %v", seedsOffered, len(plan.Seeds), result.Report.KeptAdaptive, maxAdaptive, offered)
	}
}

// Tool names in a request count as whole words: "manuscript" does not name
// manus, "profile" does not name a file tool and "watts" does not name tts.
func TestExtractIntentMatchedToolsMatchesWholeWords(t *testing.T) {
	tools := []string{"manus", "smart_file_read", "send_email", "tts", "docker", "file_search"}
	for _, tc := range []struct {
		query string
		want  []string
	}{
		{"Bitte das Manuskript korrigieren", nil},
		{"read my profile", nil},
		{"the watts of the router", nil},
		{"delegate this to Manus", []string{"manus"}},
		{"please send emails to the team", []string{"send_email"}},
		{"say it with tts", []string{"tts"}},
		{"search for the file", []string{"file_search"}},
		{"read the file", []string{"smart_file_read"}},
		{"restart docker", []string{"docker"}},
	} {
		if got := extractIntentMatchedTools(tc.query, tools); !slices.Equal(got, tc.want) {
			t.Errorf("extractIntentMatchedTools(%q) = %v, want %v", tc.query, got, tc.want)
		}
	}
	schemas := BuildNativeToolSchemas("", nil, allBuiltinToolFeatureFlags(), nil)
	if got := buildAdaptiveToolPriority(schemas, nil, "Bitte das Manuskript korrigieren", nil, nil); len(got) > 0 && got[0] == "manus" {
		t.Errorf("manus leads the ranking of a request about a manuscript: %v", got[:min(5, len(got))])
	}
}

// A seed that the selection holds anyway does not spend a family place.
func TestRankAdaptiveToolsSkipsHeldToolsAsSeeds(t *testing.T) {
	schemas := []openai.Tool{makeTool("execute_shell"), makeTool("execute_python"), makeTool("filesystem"), makeTool("system_metrics"), makeTool("process_analyzer"), makeTool("process_management")}
	const query = "bash please"
	plain := adaptiveSeedsForQuery(query, nil)
	if want := adaptiveFamilySeedTools["shell"][:adaptiveFamilySeedLimit]; !slices.Equal(plain, want) {
		t.Fatalf("seeds = %v, want %v", plain, want)
	}
	held := adaptiveSeedsForQuery(query, func(name string) bool { return name != "execute_shell" && name != "filesystem" })
	if want := []string{"execute_python", "system_metrics", "process_analyzer", "process_management"}; !slices.Equal(held, want) {
		t.Fatalf("seeds with two held tools = %v, want %v", held, want)
	}
	got := rankAdaptiveTools(adaptivePriorityInput{Schemas: schemas, Query: query, Held: []string{"execute_shell", "filesystem"}})
	for _, want := range held {
		if !slices.Contains(got, want) {
			t.Errorf("ranking %v lacks the seed %s", got, want)
		}
	}
}

// Only ranked tools take an adaptive slot. Free slots stay empty instead of
// going to the smallest schemas, and the usage tail of the ranking is what
// fills them.
func TestAdaptiveSlotsAreNeverFilledBySize(t *testing.T) {
	schemas := []openai.Tool{
		sizedFilterSchema("big", 800), sizedFilterSchema("ranked", 400), sizedFilterSchema("tiny_a", 4), sizedFilterSchema("tiny_b", 4),
		sizedFilterSchema("used", 200), sizedFilterSchema("soft", 100),
	}
	names := func(opts toolSchemaFilterOptions) string {
		return joinNames(filterToolSchemasWithReport(schemas, opts, nil).Tools)
	}
	// Two ranked tools and five free slots: the small schemas stay out.
	if got := names(toolSchemaFilterOptions{PreferredTools: []string{"ranked", "big"}, MaxAdaptiveTools: 5}); got != "ranked,big" {
		t.Fatalf("free slots were filled by size: %s", got)
	}
	if got := names(toolSchemaFilterOptions{MaxAdaptiveTools: 5, SoftAlwaysTools: []string{"soft"}}); got != "soft" {
		t.Fatalf("without a ranking only the soft tools remain: %s", got)
	}
	// A ranked tool that does not fit the token cap is skipped; the next ranked one is tried.
	fit := estimateSingleToolSchemaTokens(schemas[1]) + estimateSingleToolSchemaTokens(schemas[4])
	if got := names(toolSchemaFilterOptions{PreferredTools: []string{"big", "ranked", "used"}, MaxAdaptiveTools: 5, MaxSchemaTokens: fit}); got != "ranked,used" {
		t.Fatalf("the token cap must skip the tool that does not fit: %s", got)
	}

	// The usage tail of the ranking fills the free slots with the tools the
	// user relies on, behind everything the request ranked.
	priority := buildAdaptiveToolPriority(schemas, []string{"used", "tiny_b"}, "qqzzx plugh", nil, nil)
	if !slices.Equal(priority, []string{"used", "tiny_b"}) {
		t.Fatalf("ranking for an unmatched request = %v, want the usage tail", priority)
	}
	if got := names(toolSchemaFilterOptions{PreferredTools: priority, MaxAdaptiveTools: 5}); got != "used,tiny_b" {
		t.Fatalf("usage tail did not fill the slots: %s", got)
	}
}

func joinNames(schemas []openai.Tool) string { return strings.Join(toolSchemaNames(schemas), ",") }
