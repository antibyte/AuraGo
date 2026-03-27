package agent

import (
	"sort"
	"strings"
	"sync"

	"aurago/internal/memory"
)

type AgentTelemetryScope struct {
	ProviderType string
	Model        string
}

type AgentTelemetryToolFamilySnapshot struct {
	Family       string  `json:"family"`
	ToolCalls    int     `json:"tool_calls"`
	ToolFailures int     `json:"tool_failures"`
	SuccessRate  float64 `json:"success_rate"`
	FailureRate  float64 `json:"failure_rate"`
}

type AgentTelemetryScopeSnapshot struct {
	ProviderType   string                                      `json:"provider_type"`
	Model          string                                      `json:"model"`
	ParseSources   map[string]int                              `json:"parse_sources"`
	RecoveryEvents map[string]int                              `json:"recovery_events"`
	PolicyEvents   map[string]int                              `json:"policy_events"`
	ToolFamilies   map[string]AgentTelemetryToolFamilySnapshot `json:"tool_families,omitempty"`
	ToolCalls      int                                         `json:"tool_calls"`
	ToolFailures   int                                         `json:"tool_failures"`
	SuccessRate    float64                                     `json:"success_rate"`
	FailureRate    float64                                     `json:"failure_rate"`
	TotalEvents    int                                         `json:"total_events"`
}

type AgentTelemetrySnapshot struct {
	ParseSources   map[string]int                `json:"parse_sources"`
	RecoveryEvents map[string]int                `json:"recovery_events"`
	PolicyEvents   map[string]int                `json:"policy_events"`
	Scopes         []AgentTelemetryScopeSnapshot `json:"scopes"`
}

type agentTelemetryCollector struct {
	mu             sync.RWMutex
	parseSources   map[string]int
	recoveryEvents map[string]int
	policyEvents   map[string]int
	scoped         map[string]*AgentTelemetryScopeSnapshot
}

var globalAgentTelemetry = &agentTelemetryCollector{
	parseSources:   make(map[string]int),
	recoveryEvents: make(map[string]int),
	policyEvents:   make(map[string]int),
	scoped:         make(map[string]*AgentTelemetryScopeSnapshot),
}

var (
	agentTelemetryStoreMu  sync.RWMutex
	agentTelemetryStore    *memory.SQLiteMemory
	agentTelemetryLoadOnce sync.Once
)

func RecordToolParseSource(source ToolCallParseSource) {
	RecordToolParseSourceForScope(AgentTelemetryScope{}, source)
}

func RecordToolParseSourceForScope(scope AgentTelemetryScope, source ToolCallParseSource) {
	if source == ToolCallParseSourceNone {
		return
	}
	globalAgentTelemetry.mu.Lock()
	globalAgentTelemetry.parseSources[string(source)]++
	recordScopedTelemetryLocked(scope, "parse_source", string(source))
	globalAgentTelemetry.mu.Unlock()
	persistAgentTelemetry("parse_source", string(source))
	persistScopedAgentTelemetry(scope, "parse_source", string(source))
}

func RecordToolRecoveryEvent(name string) {
	RecordToolRecoveryEventForScope(AgentTelemetryScope{}, name)
}

func RecordToolRecoveryEventForScope(scope AgentTelemetryScope, name string) {
	if name == "" {
		return
	}
	globalAgentTelemetry.mu.Lock()
	globalAgentTelemetry.recoveryEvents[name]++
	recordScopedTelemetryLocked(scope, "recovery_event", name)
	globalAgentTelemetry.mu.Unlock()
	persistAgentTelemetry("recovery_event", name)
	persistScopedAgentTelemetry(scope, "recovery_event", name)
}

func RecordToolPolicyEvent(name string) {
	RecordToolPolicyEventForScope(AgentTelemetryScope{}, name)
}

func RecordToolPolicyEventForScope(scope AgentTelemetryScope, name string) {
	if name == "" {
		return
	}
	globalAgentTelemetry.mu.Lock()
	globalAgentTelemetry.policyEvents[name]++
	recordScopedTelemetryLocked(scope, "policy_event", name)
	globalAgentTelemetry.mu.Unlock()
	persistAgentTelemetry("policy_event", name)
	persistScopedAgentTelemetry(scope, "policy_event", name)
}

func RecordScopedToolResult(scope AgentTelemetryScope, success bool) {
	if scope.ProviderType == "" && scope.Model == "" {
		return
	}
	eventName := "success"
	if !success {
		eventName = "failure"
	}
	globalAgentTelemetry.mu.Lock()
	recordScopedTelemetryLocked(scope, "tool_result", eventName)
	globalAgentTelemetry.mu.Unlock()
	persistScopedAgentTelemetry(scope, "tool_result", eventName)
}

func RecordScopedToolResultForTool(scope AgentTelemetryScope, toolName string, success bool) {
	RecordScopedToolResult(scope, success)
	family := classifyToolFamily(toolName)
	if family == "" {
		return
	}
	eventName := family + "|success"
	if !success {
		eventName = family + "|failure"
	}
	globalAgentTelemetry.mu.Lock()
	recordScopedTelemetryLocked(scope, "tool_family_result", eventName)
	globalAgentTelemetry.mu.Unlock()
	persistScopedAgentTelemetry(scope, "tool_family_result", eventName)
}

func GetAgentTelemetrySnapshot() AgentTelemetrySnapshot {
	globalAgentTelemetry.mu.RLock()
	defer globalAgentTelemetry.mu.RUnlock()

	parseSources := make(map[string]int, len(globalAgentTelemetry.parseSources))
	for k, v := range globalAgentTelemetry.parseSources {
		parseSources[k] = v
	}
	recoveryEvents := make(map[string]int, len(globalAgentTelemetry.recoveryEvents))
	for k, v := range globalAgentTelemetry.recoveryEvents {
		recoveryEvents[k] = v
	}
	policyEvents := make(map[string]int, len(globalAgentTelemetry.policyEvents))
	for k, v := range globalAgentTelemetry.policyEvents {
		policyEvents[k] = v
	}
	scopes := make([]AgentTelemetryScopeSnapshot, 0, len(globalAgentTelemetry.scoped))
	for _, scope := range globalAgentTelemetry.scoped {
		scopeCopy := AgentTelemetryScopeSnapshot{
			ProviderType:   scope.ProviderType,
			Model:          scope.Model,
			ParseSources:   make(map[string]int, len(scope.ParseSources)),
			RecoveryEvents: make(map[string]int, len(scope.RecoveryEvents)),
			PolicyEvents:   make(map[string]int, len(scope.PolicyEvents)),
			ToolFamilies:   make(map[string]AgentTelemetryToolFamilySnapshot, len(scope.ToolFamilies)),
			ToolCalls:      scope.ToolCalls,
			ToolFailures:   scope.ToolFailures,
			SuccessRate:    scope.SuccessRate,
			FailureRate:    scope.FailureRate,
			TotalEvents:    scope.TotalEvents,
		}
		for k, v := range scope.ParseSources {
			scopeCopy.ParseSources[k] = v
		}
		for k, v := range scope.RecoveryEvents {
			scopeCopy.RecoveryEvents[k] = v
		}
		for k, v := range scope.PolicyEvents {
			scopeCopy.PolicyEvents[k] = v
		}
		for k, v := range scope.ToolFamilies {
			scopeCopy.ToolFamilies[k] = v
		}
		scopes = append(scopes, scopeCopy)
	}
	sort.Slice(scopes, func(i, j int) bool {
		if scopes[i].TotalEvents != scopes[j].TotalEvents {
			return scopes[i].TotalEvents > scopes[j].TotalEvents
		}
		if scopes[i].ProviderType != scopes[j].ProviderType {
			return scopes[i].ProviderType < scopes[j].ProviderType
		}
		return scopes[i].Model < scopes[j].Model
	})

	return AgentTelemetrySnapshot{
		ParseSources:   parseSources,
		RecoveryEvents: recoveryEvents,
		PolicyEvents:   policyEvents,
		Scopes:         scopes,
	}
}

func GetScopedAgentTelemetrySnapshot(scope AgentTelemetryScope) (AgentTelemetryScopeSnapshot, bool) {
	if scope.ProviderType == "" && scope.Model == "" {
		return AgentTelemetryScopeSnapshot{}, false
	}
	key := scope.ProviderType + "|" + scope.Model

	globalAgentTelemetry.mu.RLock()
	defer globalAgentTelemetry.mu.RUnlock()

	entry, ok := globalAgentTelemetry.scoped[key]
	if !ok {
		return AgentTelemetryScopeSnapshot{}, false
	}
	scopeCopy := AgentTelemetryScopeSnapshot{
		ProviderType:   entry.ProviderType,
		Model:          entry.Model,
		ParseSources:   make(map[string]int, len(entry.ParseSources)),
		RecoveryEvents: make(map[string]int, len(entry.RecoveryEvents)),
		PolicyEvents:   make(map[string]int, len(entry.PolicyEvents)),
		ToolFamilies:   make(map[string]AgentTelemetryToolFamilySnapshot, len(entry.ToolFamilies)),
		ToolCalls:      entry.ToolCalls,
		ToolFailures:   entry.ToolFailures,
		SuccessRate:    entry.SuccessRate,
		FailureRate:    entry.FailureRate,
		TotalEvents:    entry.TotalEvents,
	}
	for k, v := range entry.ParseSources {
		scopeCopy.ParseSources[k] = v
	}
	for k, v := range entry.RecoveryEvents {
		scopeCopy.RecoveryEvents[k] = v
	}
	for k, v := range entry.PolicyEvents {
		scopeCopy.PolicyEvents[k] = v
	}
	for k, v := range entry.ToolFamilies {
		scopeCopy.ToolFamilies[k] = v
	}
	return scopeCopy, true
}

func resetAgentTelemetryForTest() {
	globalAgentTelemetry.mu.Lock()
	defer globalAgentTelemetry.mu.Unlock()
	globalAgentTelemetry.parseSources = make(map[string]int)
	globalAgentTelemetry.recoveryEvents = make(map[string]int)
	globalAgentTelemetry.policyEvents = make(map[string]int)
	globalAgentTelemetry.scoped = make(map[string]*AgentTelemetryScopeSnapshot)
	agentTelemetryStoreMu.Lock()
	defer agentTelemetryStoreMu.Unlock()
	agentTelemetryStore = nil
	agentTelemetryLoadOnce = sync.Once{}
}

func InitializeAgentTelemetryPersistence(stm *memory.SQLiteMemory) {
	if stm == nil {
		return
	}
	agentTelemetryStoreMu.Lock()
	agentTelemetryStore = stm
	agentTelemetryStoreMu.Unlock()

	agentTelemetryLoadOnce.Do(func() {
		rows, err := stm.LoadAgentTelemetry()
		if err != nil {
			return
		}
		scopedRows, err := stm.LoadScopedAgentTelemetry()
		if err != nil {
			return
		}
		globalAgentTelemetry.mu.Lock()
		defer globalAgentTelemetry.mu.Unlock()
		for _, row := range rows {
			switch row.EventType {
			case "parse_source":
				globalAgentTelemetry.parseSources[row.EventName] = row.Count
			case "recovery_event":
				globalAgentTelemetry.recoveryEvents[row.EventName] = row.Count
			case "policy_event":
				globalAgentTelemetry.policyEvents[row.EventName] = row.Count
			}
		}
		for _, row := range scopedRows {
			scope := AgentTelemetryScope{ProviderType: row.ProviderType, Model: row.Model}
			recordScopedTelemetryCountLocked(scope, row.EventType, row.EventName, row.Count)
		}
	})
}

func persistAgentTelemetry(eventType, eventName string) {
	agentTelemetryStoreMu.RLock()
	store := agentTelemetryStore
	agentTelemetryStoreMu.RUnlock()
	if store == nil {
		return
	}
	_ = store.UpsertAgentTelemetry(eventType, eventName)
}

func persistScopedAgentTelemetry(scope AgentTelemetryScope, eventType, eventName string) {
	if scope.ProviderType == "" && scope.Model == "" {
		return
	}
	agentTelemetryStoreMu.RLock()
	store := agentTelemetryStore
	agentTelemetryStoreMu.RUnlock()
	if store == nil {
		return
	}
	_ = store.UpsertScopedAgentTelemetry(scope.ProviderType, scope.Model, eventType, eventName)
}

func recordScopedTelemetryLocked(scope AgentTelemetryScope, eventType, eventName string) {
	recordScopedTelemetryCountLocked(scope, eventType, eventName, 1)
}

func recordScopedTelemetryCountLocked(scope AgentTelemetryScope, eventType, eventName string, delta int) {
	if delta <= 0 || (scope.ProviderType == "" && scope.Model == "") {
		return
	}
	key := scope.ProviderType + "|" + scope.Model
	entry, ok := globalAgentTelemetry.scoped[key]
	if !ok {
		entry = &AgentTelemetryScopeSnapshot{
			ProviderType:   scope.ProviderType,
			Model:          scope.Model,
			ParseSources:   make(map[string]int),
			RecoveryEvents: make(map[string]int),
			PolicyEvents:   make(map[string]int),
			ToolFamilies:   make(map[string]AgentTelemetryToolFamilySnapshot),
		}
		globalAgentTelemetry.scoped[key] = entry
	}
	switch eventType {
	case "parse_source":
		entry.ParseSources[eventName] += delta
	case "recovery_event":
		entry.RecoveryEvents[eventName] += delta
	case "policy_event":
		entry.PolicyEvents[eventName] += delta
	case "tool_result":
		entry.ToolCalls += delta
		if eventName == "failure" {
			entry.ToolFailures += delta
		}
		if entry.ToolCalls > 0 {
			entry.SuccessRate = float64(entry.ToolCalls-entry.ToolFailures) / float64(entry.ToolCalls)
			entry.FailureRate = float64(entry.ToolFailures) / float64(entry.ToolCalls)
		}
	case "tool_family_result":
		updateScopedToolFamilyLocked(entry, eventName, delta)
		return
	default:
		return
	}
	entry.TotalEvents += delta
}

func updateScopedToolFamilyLocked(entry *AgentTelemetryScopeSnapshot, eventName string, delta int) {
	if entry == nil || delta <= 0 {
		return
	}
	parts := strings.SplitN(eventName, "|", 2)
	if len(parts) != 2 {
		return
	}
	family := strings.TrimSpace(parts[0])
	result := strings.TrimSpace(parts[1])
	if family == "" {
		return
	}
	snapshot := entry.ToolFamilies[family]
	if snapshot.Family == "" {
		snapshot.Family = family
	}
	snapshot.ToolCalls += delta
	if result == "failure" {
		snapshot.ToolFailures += delta
	}
	if snapshot.ToolCalls > 0 {
		snapshot.SuccessRate = float64(snapshot.ToolCalls-snapshot.ToolFailures) / float64(snapshot.ToolCalls)
		snapshot.FailureRate = float64(snapshot.ToolFailures) / float64(snapshot.ToolCalls)
	}
	entry.ToolFamilies[family] = snapshot
}
