package agent

import (
	"slices"
	"sort"
	"sync"
	"time"

	openai "github.com/sashabaranov/go-openai"
)

const defaultDiscoverToolsSnapshotTTL = 5 * time.Minute

var discoverToolsSnapshotTTL = defaultDiscoverToolsSnapshotTTL

type discoverToolsSnapshot struct {
	allSchemas   []openai.Tool
	activeNames  map[string]bool
	enabledNames map[string]bool
	promptsDir   string
	catalog      *ToolCatalog
	updatedAt    time.Time
}

// discoverToolsState stores the tool schemas and active names needed by the
// discover_tools dispatch handler. Updated once per agent loop iteration.
var discoverToolsState struct {
	mu        sync.RWMutex
	snapshots map[string]discoverToolsSnapshot
	requested map[string]map[string]int
	liveRuns  map[string]bool
	orders    map[discoverSearchKey]discoverSearchOrder
}

// discoverSearchKey identifies a search that is being paged: the ranking of the
// same query over the same catalog revision in the same session or run.
type discoverSearchKey struct{ session, query, revision string }

// discoverSearchOrder is the tool names of a search in the order of its first
// page. The ranking can differ from one call to the next (the manual search
// may time out or fail), and a page of another order would not continue the
// cursor of the previous one.
type discoverSearchOrder struct {
	names []string
	at    time.Time
}

const (
	// discoverSearchOrderLimit bounds the orders kept, discoverSearchOrderNames
	// the names of one; an order is dropped after the snapshot TTL.
	discoverSearchOrderLimit = 64
	discoverSearchOrderNames = 512
)

// rememberDiscoverSearchOrder keeps the order of a search for its next pages.
func rememberDiscoverSearchOrder(key discoverSearchKey, names []string) {
	key.session = normalizeDiscoverSessionID(key.session)
	now := time.Now()
	discoverToolsState.mu.Lock()
	defer discoverToolsState.mu.Unlock()
	delete(discoverToolsState.orders, key)
	if len(names) > discoverSearchOrderNames {
		return
	}
	if discoverToolsState.orders == nil {
		discoverToolsState.orders = make(map[discoverSearchKey]discoverSearchOrder)
	}
	for other, order := range discoverToolsState.orders {
		if now.Sub(order.at) > discoverToolsSnapshotTTL {
			delete(discoverToolsState.orders, other)
		}
	}
	if len(discoverToolsState.orders) >= discoverSearchOrderLimit {
		var oldest discoverSearchKey
		var oldestAt time.Time
		for other, order := range discoverToolsState.orders {
			if oldestAt.IsZero() || order.at.Before(oldestAt) {
				oldest, oldestAt = other, order.at
			}
		}
		delete(discoverToolsState.orders, oldest)
	}
	discoverToolsState.orders[key] = discoverSearchOrder{names: slices.Clone(names), at: now}
}

// recallDiscoverSearchOrder returns the order remembered for a search.
func recallDiscoverSearchOrder(key discoverSearchKey) ([]string, bool) {
	key.session = normalizeDiscoverSessionID(key.session)
	discoverToolsState.mu.RLock()
	defer discoverToolsState.mu.RUnlock()
	order, ok := discoverToolsState.orders[key]
	if !ok || time.Since(order.at) > discoverToolsSnapshotTTL {
		return nil, false
	}
	return slices.Clone(order.names), true
}

// forgetDiscoverSearchOrdersLocked drops the orders of a session or run.
func forgetDiscoverSearchOrdersLocked(session string) {
	for key := range discoverToolsState.orders {
		if key.session == session {
			delete(discoverToolsState.orders, key)
		}
	}
}

// SetDiscoverToolsState stores the current tool state for discover_tools lookups.
func SetDiscoverToolsState(sessionID string, allSchemas []openai.Tool, activeSchemas []openai.Tool, promptsDir string, scopes ...*DispatchContext) {
	catalog := BuildToolCatalog(allSchemas, activeSchemas, promptsDir)
	if len(scopes) > 0 {
		applyCatalogScope(catalog, scopes[0])
	}
	sessionID = normalizeDiscoverSessionID(sessionID)
	now := time.Now()
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
	if discoverToolsState.snapshots == nil {
		discoverToolsState.snapshots = make(map[string]discoverToolsSnapshot)
	}
	pruneDiscoverToolsSnapshotsLocked(now)
	discoverToolsState.snapshots[sessionID] = discoverToolsSnapshot{
		allSchemas:   allSchemas,
		activeNames:  active,
		enabledNames: enabled,
		promptsDir:   promptsDir,
		catalog:      catalog,
		updatedAt:    now,
	}
	if discoverToolsState.requested == nil {
		discoverToolsState.requested = make(map[string]map[string]int)
	}
	if _, ok := discoverToolsState.requested[sessionID]; !ok {
		discoverToolsState.requested[sessionID] = make(map[string]int)
	}
	discoverToolsState.mu.Unlock()
}

func SetDiscoverToolsSnapshotTTL(ttl time.Duration) {
	if ttl <= 0 {
		ttl = defaultDiscoverToolsSnapshotTTL
	}
	discoverToolsState.mu.Lock()
	discoverToolsSnapshotTTL = ttl
	discoverToolsState.mu.Unlock()
}

func DiscoverToolsSnapshotTTL() time.Duration {
	discoverToolsState.mu.RLock()
	defer discoverToolsState.mu.RUnlock()
	return discoverToolsSnapshotTTL
}

func pruneDiscoverToolsSnapshotsLocked(now time.Time) {
	if discoverToolsSnapshotTTL <= 0 || len(discoverToolsState.snapshots) == 0 {
		return
	}
	cutoff := now.Add(-discoverToolsSnapshotTTL)
	for sessionID, snapshot := range discoverToolsState.snapshots {
		if discoverToolsState.liveRuns[sessionID] {
			continue
		}
		if snapshot.updatedAt.IsZero() {
			// Zero time means the snapshot was never properly initialised; treat as expired.
			delete(discoverToolsState.snapshots, sessionID)
			forgetDiscoverSearchOrdersLocked(sessionID)
			if discoverToolsState.requested != nil {
				delete(discoverToolsState.requested, sessionID)
			}
			continue
		}
		if snapshot.updatedAt.Before(cutoff) {
			delete(discoverToolsState.snapshots, sessionID)
			forgetDiscoverSearchOrdersLocked(sessionID)
			if discoverToolsState.requested != nil {
				delete(discoverToolsState.requested, sessionID)
			}
		}
	}
}

// MarkDiscoverRequestedTool remembers that a hidden tool was explicitly requested
// via discover_tools for the given session, so the next loop iteration can re-add
// it to the active native tool list.
func MarkDiscoverRequestedTool(sessionID, toolName string) {
	if toolName == "" {
		return
	}
	sessionID = normalizeDiscoverSessionID(sessionID)
	discoverToolsState.mu.Lock()
	defer discoverToolsState.mu.Unlock()
	if discoverToolsState.requested == nil {
		discoverToolsState.requested = make(map[string]map[string]int)
	}
	if _, ok := discoverToolsState.requested[sessionID]; !ok {
		discoverToolsState.requested[sessionID] = make(map[string]int)
	}
	discoverToolsState.requested[sessionID][toolName] = 1
}

// GetDiscoverRequestedTools returns session-scoped hidden tools that should be
// temporarily re-included after discover_tools surfaced them.
func GetDiscoverRequestedTools(sessionID string) []string {
	sessionID = normalizeDiscoverSessionID(sessionID)
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
	sort.Strings(out)
	return out
}

// ConsumeDiscoverRequestedTools returns session-scoped requested tools once and
// clears them so discovery does not permanently bypass adaptive filtering.
func ConsumeDiscoverRequestedTools(sessionID string) []string {
	sessionID = normalizeDiscoverSessionID(sessionID)
	discoverToolsState.mu.Lock()
	defer discoverToolsState.mu.Unlock()
	set := discoverToolsState.requested[sessionID]
	if len(set) == 0 {
		return nil
	}
	out := make([]string, 0, len(set))
	for name := range set {
		out = append(out, name)
	}
	sort.Strings(out)
	delete(discoverToolsState.requested, sessionID)
	return out
}

// GetDiscoverToolsState returns a snapshot of the current tool state.
func GetDiscoverToolsState(sessionIDs ...string) (allSchemas []openai.Tool, activeNames map[string]bool, enabledNames map[string]bool, promptsDir string) {
	discoverToolsState.mu.RLock()
	defer discoverToolsState.mu.RUnlock()
	sessionID := discoverSessionIDFromArgs(sessionIDs...)
	if snapshot, ok := discoverToolsState.snapshots[sessionID]; ok {
		return snapshot.allSchemas, snapshot.activeNames, snapshot.enabledNames, snapshot.promptsDir
	}
	return nil, nil, nil, ""
}

func GetToolCatalogState(sessionIDs ...string) *ToolCatalog {
	discoverToolsState.mu.RLock()
	defer discoverToolsState.mu.RUnlock()
	sessionID := discoverSessionIDFromArgs(sessionIDs...)
	if snapshot, ok := discoverToolsState.snapshots[sessionID]; ok {
		return snapshot.catalog
	}
	return nil
}

const discoverDefaultSessionID = "__default__"

func normalizeDiscoverSessionID(sessionID string) string {
	if sessionID == "" {
		return discoverDefaultSessionID
	}
	return sessionID
}

// ClearDiscoverToolsState removes the discovery snapshot and requested-tool state
// for a session. Should be called when the session is reset or deleted.
func ClearDiscoverToolsState(sessionID string) {
	sessionID = normalizeDiscoverSessionID(sessionID)
	discoverToolsState.mu.Lock()
	defer discoverToolsState.mu.Unlock()
	delete(discoverToolsState.snapshots, sessionID)
	forgetDiscoverSearchOrdersLocked(sessionID)
	if discoverToolsState.requested != nil {
		delete(discoverToolsState.requested, sessionID)
	}
}

func discoverSessionIDFromArgs(sessionIDs ...string) string {
	if len(sessionIDs) == 0 {
		return discoverDefaultSessionID
	}
	return normalizeDiscoverSessionID(sessionIDs[0])
}
