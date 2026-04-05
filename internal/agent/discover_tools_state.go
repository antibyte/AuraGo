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
	promptsDir   string
}

// SetDiscoverToolsState stores the current tool state for discover_tools lookups.
func SetDiscoverToolsState(allSchemas []openai.Tool, activeSchemas []openai.Tool, promptsDir string) {
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
	discoverToolsState.promptsDir = promptsDir
	discoverToolsState.mu.Unlock()
}

// GetDiscoverToolsState returns a snapshot of the current tool state.
func GetDiscoverToolsState() (allSchemas []openai.Tool, activeNames map[string]bool, enabledNames map[string]bool, promptsDir string) {
	discoverToolsState.mu.RLock()
	defer discoverToolsState.mu.RUnlock()
	return discoverToolsState.allSchemas, discoverToolsState.activeNames, discoverToolsState.enabledNames, discoverToolsState.promptsDir
}
