package agent

import (
	"log/slog"
	"slices"
	"sort"

	"aurago/internal/config"
	openai "github.com/sashabaranov/go-openai"
)

// adaptiveSelectionInput is everything the adaptive first selection looks at.
// The caller gathers the run state (the discover_tools requests are consumed
// by the caller, the history is the session's tools); the planner itself reads
// nothing else, so production and the tests that mirror it share one code
// path.
type adaptiveSelectionInput struct {
	Config    *config.Config
	RunConfig RunConfig
	Features  ToolFeatureFlags
	// Schemas are all native tools the run could offer.
	Schemas []openai.Tool
	// Query is the user message the selection serves.
	Query string
	// Messages are the request messages: a tool output reference in them keeps
	// read_tool_output.
	Messages []openai.ChatCompletionMessage
	// Usage lists the tools the user relies on, most relied-on first.
	Usage  []string
	Guides toolGuideSearcher
	Logger *slog.Logger
	// VoiceOutput keeps tts and send_audio for a spoken answer.
	VoiceOutput bool
	// History are the tools that stay visible because the session used them:
	// those of this run and those in the recent conversation.
	History []string
	// Requested are the tools discover_tools asked for since the last
	// selection (already consumed by the caller).
	Requested []string

	MaxAdaptive, MaxTotal, MaxSchemaTokens int
}

// adaptiveSelectionPlan is the outcome of the first selection: the filter
// options and the classes the tools fall into. The refresh before every model
// call keeps these classes instead of ranking the survivors again by position
// (adaptiveRefreshFilterOptions).
type adaptiveSelectionPlan struct {
	// Options select the first tool list (filterToolSchemasWithReport).
	Options toolSchemaFilterOptions
	// Soft are the configured always-include tools, the voice tools, the
	// channel tools and read_tool_output: tools that stay whatever they rank.
	Soft []string
	// Generator are the media generators the request asked for.
	Generator []string
	// History are the tools the session used.
	History []string
	// Requested are the tools discover_tools asked for.
	Requested []string
	// Seeds are the tools the family and cues of the request make likely.
	Seeds []string
	// Priority is the full ranking the adaptive slots were filled from.
	Priority []string
}

// planAdaptiveFirstSelection decides which tool schemas the first model call
// of a run offers, as hard, soft and ranked adaptive tools under the caps.
func planAdaptiveFirstSelection(in adaptiveSelectionInput) adaptiveSelectionPlan {
	cfg := in.Config
	plan := adaptiveSelectionPlan{
		Soft:      slices.Clone(cfg.Agent.AdaptiveTools.AlwaysInclude),
		Generator: adaptiveGeneratorToolsForQuery(in.Query, in.Features),
		History:   uniqueStrings(in.History),
		Requested: uniqueStrings(in.Requested),
	}
	if in.VoiceOutput {
		plan.Soft = append(plan.Soft, "tts", "send_audio")
	}
	plan.Soft = channelAdaptiveAlwaysInclude(in.RunConfig, plan.Soft, in.Features)
	plan.Soft = outputRefAdaptiveAlwaysInclude(in.Messages, plan.Soft)

	soft := make([]string, 0, len(plan.Soft)+len(plan.Generator)+len(plan.History)+len(plan.Requested))
	soft = append(soft, plan.Soft...)
	soft = append(soft, plan.Generator...)
	soft = append(soft, plan.History...)
	soft = append(soft, plan.Requested...)
	soft = expandAdaptiveAlwaysInclude(cfg, soft)
	hard := channelAdaptiveAlwaysInclude(in.RunConfig, adaptiveHardAlwaysInclude(cfg), in.Features)

	// A tool the selection holds anyway needs no place among the seeds.
	held := stringSet(append(slices.Clone(hard), soft...))
	plan.Priority = rankAdaptiveTools(adaptivePriorityInput{
		Schemas: in.Schemas, Usage: in.Usage, Query: in.Query, Guides: in.Guides, Logger: in.Logger,
		Held: sortedKeys(held),
	})
	offered := stringSet(toolSchemaNames(in.Schemas))
	plan.Seeds = adaptiveSeedsForQuery(boundToolSearchQuery(in.Query), func(name string) bool {
		return offered[name] && !held[name] && !slices.Contains(adaptiveIntentOnlyTools, name)
	})

	plan.Options = toolSchemaFilterOptions{
		PreferredTools:   plan.Priority,
		HardAlwaysTools:  hard,
		SoftAlwaysTools:  soft,
		MaxAdaptiveTools: in.MaxAdaptive,
		MaxTotalTools:    in.MaxTotal,
		MaxSchemaTokens:  in.MaxSchemaTokens,
		AdditiveTools:    adaptiveAdditiveToolsForQuery(in.Query),
		// Only the first selection ranks the whole catalog; the per-iteration
		// refresh keeps what was selected here.
		AdaptiveExcludedTools: adaptiveIntentOnlyTools,
		AdditiveSwaps:         adaptiveSwapsForQuery(in.Query),
	}
	return plan
}

// sortedKeys lists the true keys of a set in a stable order.
func sortedKeys(set map[string]bool) []string {
	out := make([]string, 0, len(set))
	for name, ok := range set {
		if ok {
			out = append(out, name)
		}
	}
	sort.Strings(out)
	return out
}
