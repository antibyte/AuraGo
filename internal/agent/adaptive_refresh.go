package agent

import (
	"slices"

	"aurago/internal/config"
	openai "github.com/sashabaranov/go-openai"
)

// adaptiveRefreshInput describes the state before one model call, for the
// refresh that follows the first selection.
type adaptiveRefreshInput struct {
	Config    *config.Config
	RunConfig RunConfig
	Features  ToolFeatureFlags
	// Plan is the plan of the first selection. Nil when the run had none
	// (adaptive tools off, no short-term memory, the managed local kernel, a
	// prepared prompt): the refresh then ranks the schemas by their position,
	// as it always did.
	Plan *adaptiveSelectionPlan
	// Tools are the candidate schemas: the previous list with swapped-out
	// partners back in and the requested tools added.
	Tools []openai.Tool
	// Requested are the discover_tools requests consumed for this call.
	Requested []string
	// RequestOrder lists every discover_tools request of the run, oldest first
	// (recordRequestOrder); Pinned is the same set.
	RequestOrder []string
	Pinned       map[string]bool
	// SessionUsed and Recent are the tools kept from the session now: those
	// executed during this run and those in the recent conversation.
	SessionUsed map[string]bool
	Recent      []string
	// InitFiltered reports that the first selection ranked the whole catalog.
	InitFiltered bool

	MaxAdaptive, MaxTotal, MaxSchemaTokens int
	AdditiveTools                          []string
	AdditiveSwaps                          map[string]string
}

// recordRequestOrder adds discover_tools requests to the run's ordered list,
// oldest first. A tool requested again moves to the end, as the newest.
func recordRequestOrder(order, names []string) []string {
	for _, name := range names {
		if name == "" {
			continue
		}
		if at := slices.Index(order, name); at >= 0 {
			order = slices.Delete(order, at, at+1)
		}
		order = append(order, name)
	}
	return order
}

// adaptiveRefreshFilterOptions builds the filter options of the refresh that
// runs before every model call.
//
// The order of precedence, with a plan: hard tools; the plan's soft tools
// (configured, voice, channel, read_tool_output, the media generators the
// request asked for, the session's tools at the first selection, the requests
// of the first selection); the discover_tools requests, this call's first and
// then the earlier ones, newest first; the tools the run used since; the
// plan's ranking; every other candidate. Only the soft classes are reserved:
// requests, used tools and ranked tools share MaxAdaptiveTools and the caps in
// that order, so a long run of tool calls can never crowd a request out. A
// refresh without a new request or tool use returns the first selection, and
// one request costs the lowest ranked adaptive tool, or more when its schema
// is larger than the one it displaces and a token cap binds.
//
// Without a plan the refresh ranks the requests (newest first) and then the
// schemas by their position in the list.
func adaptiveRefreshFilterOptions(in adaptiveRefreshInput) toolSchemaFilterOptions {
	kept := refreshSwapProtectedTools(in.InitFiltered, in.SessionUsed, in.Recent)
	names := toolSchemaNames(in.Tools)
	present := stringSet(names)
	requests := rankedRequests(in.Requested, in.RequestOrder, present)
	opts := toolSchemaFilterOptions{
		PreferredTools:        slices.Concat(requests, names),
		HardAlwaysTools:       channelAdaptiveAlwaysInclude(in.RunConfig, adaptiveHardAlwaysInclude(in.Config), in.Features),
		SoftAlwaysTools:       in.Config.Agent.AdaptiveTools.AlwaysInclude,
		MaxAdaptiveTools:      in.MaxAdaptive,
		MaxTotalTools:         in.MaxTotal,
		MaxSchemaTokens:       in.MaxSchemaTokens,
		AdditiveTools:         in.AdditiveTools,
		AdaptiveExcludedTools: adaptiveRefreshExcludedTools(in.InitFiltered, in.Pinned),
		AdditiveSwaps:         in.AdditiveSwaps,
		PinnedTools:           pinnedToolNames(in.Pinned),
		SwapProtectedTools:    kept,
	}
	if in.Plan != nil {
		opts.SoftAlwaysTools = slices.Clone(in.Plan.Options.SoftAlwaysTools)
		held := stringSet(slices.Concat(opts.SoftAlwaysTools, opts.HardAlwaysTools))
		used := toolsUsedThisRun(in.SessionUsed, in.Recent, held, present)
		opts.PreferredTools = slices.Concat(requests, used, in.Plan.Priority, names)
	}
	return opts
}

// rankedRequests lists the discover_tools requests in ranking order: this
// call's first, then the earlier ones that are still candidates, newest first.
func rankedRequests(fresh, order []string, present map[string]bool) []string {
	out := slices.Clone(fresh)
	seen := stringSet(fresh)
	for i := len(order) - 1; i >= 0; i-- {
		if name := order[i]; present[name] && !seen[name] {
			seen[name] = true
			out = append(out, name)
		}
	}
	return out
}

// toolsUsedThisRun lists the tools the run used since the first selection, most
// recently used first: the session's tools that no soft class already holds.
func toolsUsedThisRun(sessionUsed map[string]bool, recent []string, held, present map[string]bool) []string {
	var out []string
	seen := make(map[string]bool, len(recent)+len(sessionUsed))
	add := func(name string) {
		if name != "" && !seen[name] && present[name] && !held[name] {
			out = append(out, name)
		}
		seen[name] = true
	}
	for i := len(recent) - 1; i >= 0; i-- {
		add(recent[i])
	}
	for _, name := range sortedKeys(sessionUsed) {
		add(name)
	}
	return out
}
