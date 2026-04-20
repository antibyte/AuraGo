package agent

import (
	"sync"

	openai "github.com/sashabaranov/go-openai"
)

// discoverToolsState stores the tool schemas and active names needed by the
// discover_tools dispatch handler. Updated once per agent loop iteration.
var discoverToolsState struct {
	mu           sync.RWMutex
	allSchemas   []openai.Tool // full unfiltered schemas (all enabled tools)
	activeNames  map[string]bool
	enabledNames map[string]bool
	requested    map[string]map[string]bool
	promptsDir   string
}

// SetDiscoverToolsState stores the current tool state for discover_tools lookups.
func SetDiscoverToolsState(sessionID string, allSchemas []openai.Tool, activeSchemas []openai.Tool, promptsDir string) {
	active := make(map[string]bool, len(activeSchemas))
	for _, s := range activeSchemas {
		if s.Function != nil {
			active[s.Function.Name] = true
		}
	}
	enabled := make(map[string]bool, len(allSchemas))
	for _, s := range allSchemas {
		if s.Function != nil {
			enabled[s.Function.Name] = true
		}
	}
	discoverToolsState.mu.Lock()
	discoverToolsState.allSchemas = allSchemas
	discoverToolsState.activeNames = active
	discoverToolsState.enabledNames = enabled
	if discoverToolsState.requested == nil {
		discoverToolsState.requested = make(map[string]map[string]bool)
	}
	if sessionID != "" {
		if _, ok := discoverToolsState.requested[sessionID]; !ok {
			discoverToolsState.requested[sessionID] = make(map[string]bool)
		}
	}
	discoverToolsState.promptsDir = promptsDir
	discoverToolsState.mu.Unlock()
}

// MarkDiscoverRequestedTool remembers that a hidden tool was explicitly requested
// via discover_tools for the given session, so the next loop iteration can re-add
// it to the active native tool list.
func MarkDiscoverRequestedTool(sessionID, toolName string) {
	if sessionID == "" || toolName == "" {
		return
	}
	discoverToolsState.mu.Lock()
	defer discoverToolsState.mu.Unlock()
	if discoverToolsState.requested == nil {
		discoverToolsState.requested = make(map[string]map[string]bool)
	}
	if _, ok := discoverToolsState.requested[sessionID]; !ok {
		discoverToolsState.requested[sessionID] = make(map[string]bool)
	}
	discoverToolsState.requested[sessionID][toolName] = true
}

// GetDiscoverRequestedTools returns session-scoped hidden tools that should be
// temporarily re-included after discover_tools surfaced them.
func GetDiscoverRequestedTools(sessionID string) []string {
	if sessionID == "" {
		return nil
	}
	discoverToolsState.mu.RLock()
	defer discoverToolsState.mu.RUnlock()
	set := discoverToolsState.requested[sessionID]
	if len(set) == 0 {
		return nil
	}
	out := make([]string, 0, len(set))
	for name := range set {
		out = append(out, name)
	}
	return out
}

// GetDiscoverToolsState returns a snapshot of the current tool state.
func GetDiscoverToolsState() (allSchemas []openai.Tool, activeNames map[string]bool, enabledNames map[string]bool, promptsDir string) {
	discoverToolsState.mu.RLock()
	defer discoverToolsState.mu.RUnlock()
	return discoverToolsState.allSchemas, discoverToolsState.activeNames, discoverToolsState.enabledNames, discoverToolsState.promptsDir
}
