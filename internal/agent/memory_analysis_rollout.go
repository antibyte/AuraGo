package agent

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"aurago/internal/config"
	"aurago/internal/memory"
)

type effectiveMemoryAnalysisSettings struct {
	Enabled               bool
	Mode                  string
	Reason                string
	RealTime              bool
	QueryExpansion        bool
	LLMReranking          bool
	WeeklyReflection      bool
	ReflectionDay         string
	UnifiedMemoryBlock    bool
	EffectivenessTracking bool
}

type MemoryAnalysisDashboardState struct {
	Enabled               bool   `json:"enabled"`
	Mode                  string `json:"mode"`
	Reason                string `json:"reason"`
	RealTime              bool   `json:"real_time"`
	QueryExpansion        bool   `json:"query_expansion"`
	LLMReranking          bool   `json:"llm_reranking"`
	WeeklyReflection      bool   `json:"weekly_reflection"`
	UnifiedMemoryBlock    bool   `json:"unified_memory_block"`
	EffectivenessTracking bool   `json:"effectiveness_tracking"`
}

type cachedMemoryAnalysisSettings struct {
	settings  effectiveMemoryAnalysisSettings
	expiresAt time.Time
}

var memoryAnalysisSettingsCache = struct {
	mu      sync.RWMutex
	entries map[*memory.SQLiteMemory]cachedMemoryAnalysisSettings
}{entries: make(map[*memory.SQLiteMemory]cachedMemoryAnalysisSettings)}

func BuildMemoryAnalysisDashboardState(cfg *config.Config, stm *memory.SQLiteMemory) MemoryAnalysisDashboardState {
	settings := resolveMemoryAnalysisSettings(cfg, stm)
	return MemoryAnalysisDashboardState{
		Enabled:               settings.Enabled,
		Mode:                  settings.Mode,
		Reason:                settings.Reason,
		RealTime:              settings.RealTime,
		QueryExpansion:        settings.QueryExpansion,
		LLMReranking:          settings.LLMReranking,
		WeeklyReflection:      settings.WeeklyReflection,
		UnifiedMemoryBlock:    settings.UnifiedMemoryBlock,
		EffectivenessTracking: settings.EffectivenessTracking,
	}
}

func resolveMemoryAnalysisSettings(cfg *config.Config, stm *memory.SQLiteMemory) effectiveMemoryAnalysisSettings {
	settings := effectiveMemoryAnalysisSettings{
		Enabled:               true,
		Mode:                  "balanced",
		Reason:                "adaptive defaults active",
		RealTime:              true,
		QueryExpansion:        true,
		LLMReranking:          true,
		WeeklyReflection:      true,
		ReflectionDay:         "sunday",
		UnifiedMemoryBlock:    true,
		EffectivenessTracking: true,
	}
	if cfg == nil {
		settings.Enabled = false
		settings.Mode = "unavailable"
		settings.Reason = "missing config"
		return settings
	}

	if day := strings.TrimSpace(cfg.MemoryAnalysis.ReflectionDay); day != "" {
		settings.ReflectionDay = day
	}
	if stm == nil {
		settings.Mode = "bootstrap"
		settings.Reason = "no memory telemetry available yet"
		return settings
	}

	now := time.Now()
	memoryAnalysisSettingsCache.mu.RLock()
	if cached, ok := memoryAnalysisSettingsCache.entries[stm]; ok && now.Before(cached.expiresAt) {
		memoryAnalysisSettingsCache.mu.RUnlock()
		cached.settings.ReflectionDay = settings.ReflectionDay
		return cached.settings
	}
	memoryAnalysisSettingsCache.mu.RUnlock()

	usage, err := stm.GetMemoryUsageStats(14, 5)
	if err != nil {
		settings.Mode = "bootstrap"
		settings.Reason = "memory usage telemetry unavailable"
		return settings
	}
	metas, err := stm.GetAllMemoryMeta(1000, 0)
	if err != nil {
		settings.Mode = "bootstrap"
		settings.Reason = "memory metadata unavailable"
		return settings
	}
	health := memory.BuildMemoryHealthReport(metas, usage)
	totalMeta := health.Confidence.Total
	tracked := maxInt(health.Effectiveness.Tracked, 1)
	underperformingRatio := float64(health.Effectiveness.Underperforming) / float64(tracked)
	unverifiedRatio := 0.0
	if totalMeta > 0 {
		unverifiedRatio = float64(health.Confidence.Unverified) / float64(totalMeta)
	}

	switch {
	case totalMeta < 12 || usage.TotalEvents < 8:
		settings.Mode = "bootstrap"
		settings.QueryExpansion = false
		settings.LLMReranking = false
		settings.Reason = fmt.Sprintf("building memory baseline (%d memories, %d recent events)", totalMeta, usage.TotalEvents)
	case health.Curator.Contradictions > 0 || health.Curator.LowEffectiveness > 0 || underperformingRatio >= 0.25 || unverifiedRatio >= 0.45:
		settings.Mode = "stabilize"
		settings.QueryExpansion = true
		settings.LLMReranking = true
		settings.Reason = fmt.Sprintf("stabilizing noisy memory set (%d contradictions, %d low-effectiveness memories)", health.Curator.Contradictions, health.Curator.LowEffectiveness)
	case health.Effectiveness.Tracked >= 8 && health.Curator.LowEffectiveness == 0 && health.Effectiveness.Helpful*100 >= health.Effectiveness.Tracked*70:
		settings.Mode = "efficient"
		settings.QueryExpansion = true
		settings.LLMReranking = false
		settings.Reason = fmt.Sprintf("healthy memory quality (%d/%d helpful tracked memories)", health.Effectiveness.Helpful, health.Effectiveness.Tracked)
	default:
		settings.Mode = "balanced"
		settings.QueryExpansion = true
		settings.LLMReranking = health.Curator.LowConfidence > 0 || health.Curator.LowEffectiveness > 0 || usage.PredictedEvents > usage.RetrievedEvents
		settings.Reason = fmt.Sprintf("balanced adaptive mode (%d memories, %d recent events)", totalMeta, usage.TotalEvents)
	}

	memoryAnalysisSettingsCache.mu.Lock()
	memoryAnalysisSettingsCache.entries[stm] = cachedMemoryAnalysisSettings{
		settings:  settings,
		expiresAt: now.Add(30 * time.Second),
	}
	memoryAnalysisSettingsCache.mu.Unlock()

	return settings
}

func maxInt(left int, right int) int {
	if left > right {
		return left
	}
	return right
}
