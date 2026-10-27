package agent

import (
	"fmt"
	"maps"
	"slices"
	"strings"
)

// gameMakerStudioExtraTools are the non-game_maker_* tools the Studio adds to
// its own scope: code_graph comes from WithCodeGraphProfile, added when project
// knowledge is enabled.
var gameMakerStudioExtraTools = map[string]bool{"code_graph": true}

// gameMakerRun reports whether this is a Game Maker Studio run: its tool scope
// is restricted, non-empty, holds at least one game_maker_* tool and nothing
// else but the Studio's own extras (code_graph). (The job bound to the context
// is not consulted: the loop state never carries it.) The generic recovery
// nudges describe concepts a Studio run does not have (<done/>, discover_tools,
// file_editor), so a Studio run gets Studio-specific ones. The main agent (no
// scope) and mixed scopes keep the generic text.
func (s *agentLoopState) gameMakerRun() bool {
	if s == nil || len(s.runCfg.AllowedTools) == 0 {
		return false
	}
	studio := false
	for name := range normalizedAllowedToolSet(s.runCfg.AllowedTools) {
		switch {
		case strings.HasPrefix(name, "game_maker_"):
			studio = true
		case !gameMakerStudioExtraTools[name]:
			return false
		}
	}
	return studio
}

// announcementFeedback is the nudge for a text-only reply that made no tool call.
func (s *agentLoopState) announcementFeedback() string {
	if s.gameMakerRun() {
		return formatGameMakerAnnouncementFeedback(s.useNativeFunctions, s.runCfg.AllowedTools)
	}
	return FormatAnnouncementFeedback(s.useNativeFunctions, s.recentTools)
}

// formatGameMakerAnnouncementFeedback is the Studio form of
// FormatAnnouncementFeedback: it names the run's tools and the exact call shape,
// and says truthfully what a text-only reply does. Planning ends only when a
// design is accepted; in building and repair the server validates the saved
// files once the model stops calling tools. The text depends only on the run's
// tools and mode, so it is stable for a phase.
func formatGameMakerAnnouncementFeedback(useNativeFunctions bool, allowedTools []string) string {
	names, _ := allowedToolList(normalizedAllowedToolSet(allowedTools))
	if len(names) == 0 {
		names = slices.Sorted(maps.Keys(gameMakerToolOperations()))
	}
	building := slices.Contains(names, "game_maker_validate")
	feedback := "Your last reply contained no Game Maker tool call. Available tools: " + strings.Join(names, ", ") +
		". Pass the operation (e.g. set_design, read, replace) in the operation argument of its tool"
	switch {
	case useNativeFunctions:
		feedback += "; use the native function-calling mechanism."
	case building:
		feedback += `, as one raw JSON object: {"action":"game_maker_file","operation":"read","path":"src/main.ts"}.`
	default:
		feedback += `, as one raw JSON object: {"action":"game_maker_project","operation":"set_design","design":{...}}.`
	}
	if building {
		return feedback + " If this phase's work is done, reply with one short sentence; the server then checks the saved files."
	}
	return feedback + " Planning ends only when a design is accepted, so submit it with set_design."
}

// formatGameMakerXMLFallbackFeedback is the Studio form of the note sent after a
// tool call in a proprietary XML format. The generic note points at
// discover_tools and file_editor, which a Studio run cannot call.
func formatGameMakerXMLFallbackFeedback(action string, useNativeFunctions bool) string {
	next := `a raw JSON object such as {"action":"game_maker_file","operation":"read","path":"src/main.ts"}`
	if useNativeFunctions {
		next = "the native function-calling mechanism"
	}
	return fmt.Sprintf("NOTE: You called '%s' using a proprietary XML tool-call format. "+
		"The call has already been executed and its action is COMPLETE — do NOT repeat it. "+
		"Continue with the next step of the task. For future calls use %s.", action, next)
}
