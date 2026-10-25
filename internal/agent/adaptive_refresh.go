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
	// Pinned are all tools discover_tools requested during the run.
	Pinned map[string]bool
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

// adaptiveRefreshFilterOptions builds the filter options of the refresh that
// runs before every model call.
//
// With a plan the refresh keeps the classes of the first selection: the soft
// tools (configured, voice, channel, media generators the request asked for,
// the session's tools, the requests discover_tools already made) stay soft, and
// so does any tool the run used since. The ranking is the one of the first
// selection (the plan's priority), with the tools requested since in front, so
// a refresh without a new request returns the first selection and one request
// costs at most the lowest ranked adaptive tool. Without a plan the refresh
// ranks the schemas by their position in the list.
func adaptiveRefreshFilterOptions(in adaptiveRefreshInput) toolSchemaFilterOptions {
	kept := refreshSwapProtectedTools(in.InitFiltered, in.SessionUsed, in.Recent)
	names := toolSchemaNames(in.Tools)
	opts := toolSchemaFilterOptions{
		PreferredTools:        append(slices.Clone(in.Requested), names...),
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
		opts.SoftAlwaysTools = append(slices.Clone(in.Plan.Options.SoftAlwaysTools), kept...)
		opts.PreferredTools = slices.Concat(in.Requested, in.Plan.Priority, names)
	}
	return opts
}
