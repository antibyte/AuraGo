package warnings

import (
	"fmt"
	"regexp"
	"strings"
	"sync"
	"time"

	"aurago/internal/llm"
	"aurago/internal/security"
)

const maxLLMWarningDescriptionLen = 900

type llmProviderStreak struct {
	failures []time.Time
	emitted  bool
}

// LLMProviderMonitor converts repeated LLM provider failures into user-visible
// system warnings.
type LLMProviderMonitor struct {
	mu        sync.Mutex
	reg       *Registry
	threshold int
	window    time.Duration
	streaks   map[string]*llmProviderStreak
}

// NewLLMProviderMonitor creates a warning-backed monitor for LLM provider
// health. Invalid thresholds and windows are replaced with conservative
// defaults.
func NewLLMProviderMonitor(reg *Registry, threshold int, window time.Duration) *LLMProviderMonitor {
	if threshold <= 0 {
		threshold = 3
	}
	if window <= 0 {
		window = 10 * time.Minute
	}
	return &LLMProviderMonitor{
		reg:       reg,
		threshold: threshold,
		window:    window,
		streaks:   make(map[string]*llmProviderStreak),
	}
}

func (m *LLMProviderMonitor) ReportLLMHealthEvent(event llm.HealthEvent) {
	if m == nil || m.reg == nil {
		return
	}
	if event.Timestamp.IsZero() {
		event.Timestamp = time.Now()
	}
	if isImmediateLLMProviderProblem(event) {
		m.reg.Add(m.warningForEvent(event, SeverityCritical, "LLM Provider Configuration Problem", 1))
		return
	}

	key := llmProviderHealthKey(event.Provider, event.Model, string(event.ErrorCategory))
	m.mu.Lock()
	streak := m.streaks[key]
	if streak == nil {
		streak = &llmProviderStreak{}
		m.streaks[key] = streak
	}
	cutoff := event.Timestamp.Add(-m.window)
	filtered := streak.failures[:0]
	for _, ts := range streak.failures {
		if !ts.Before(cutoff) {
			filtered = append(filtered, ts)
		}
	}
	streak.failures = append(filtered, event.Timestamp)
	count := len(streak.failures)
	shouldWarn := count >= m.threshold && !streak.emitted
	if shouldWarn {
		streak.emitted = true
	}
	m.mu.Unlock()

	if shouldWarn {
		m.reg.Add(m.warningForEvent(event, SeverityWarning, "LLM Provider Has Repeated Errors", count))
	}
}

func (m *LLMProviderMonitor) ReportLLMHealthSuccess(success llm.HealthSuccess) {
	if m == nil {
		return
	}
	prefix := llmProviderHealthKeyPrefix(success.Provider, success.Model)
	m.mu.Lock()
	defer m.mu.Unlock()
	for key := range m.streaks {
		if strings.HasPrefix(key, prefix) {
			delete(m.streaks, key)
		}
	}
}

func isImmediateLLMProviderProblem(event llm.HealthEvent) bool {
	if event.Retryable {
		return false
	}
	switch event.ErrorCategory {
	case llm.ErrCategoryAuthError, llm.ErrCategoryQuotaExceeded, llm.ErrCategoryNonRetryableConfig:
		return true
	default:
		return false
	}
}

func (m *LLMProviderMonitor) warningForEvent(event llm.HealthEvent, severity, title string, count int) Warning {
	provider := valueOrUnknown(event.Provider)
	model := valueOrUnknown(event.Model)
	category := valueOrUnknown(string(event.ErrorCategory))
	summary := sanitizeLLMWarningText(event.ErrorSummary, 260)
	description := fmt.Sprintf(
		"Provider %q with model %q reported %s during %s. Category: %s. Last error: %s. This warning is shown so you can see provider problems without opening the logs; detailed diagnostics remain in the application log.",
		provider,
		model,
		countDescription(count),
		valueOrUnknown(event.Operation),
		category,
		summary,
	)
	description = sanitizeLLMWarningText(description, maxLLMWarningDescriptionLen)

	return Warning{
		ID:          llmProviderWarningID(event),
		Severity:    severity,
		Title:       title,
		Description: description,
		Category:    CategorySystem,
		Timestamp:   event.Timestamp,
	}
}

func countDescription(count int) string {
	if count <= 1 {
		return "a provider problem"
	}
	return fmt.Sprintf("%d provider problems", count)
}

func llmProviderWarningID(event llm.HealthEvent) string {
	ts := event.Timestamp.UTC().Format("20060102T150405")
	return strings.Join([]string{
		"llm_provider_health",
		slug(event.Provider),
		slug(event.Model),
		slug(string(event.ErrorCategory)),
		ts,
	}, "_")
}

func llmProviderHealthKey(provider, model, category string) string {
	return llmProviderHealthKeyPrefix(provider, model) + "|" + category
}

func llmProviderHealthKeyPrefix(provider, model string) string {
	return provider + "|" + model + "|"
}

func sanitizeLLMWarningText(text string, maxLen int) string {
	text = strings.TrimSpace(security.Scrub(text))
	text = strings.Join(strings.Fields(text), " ")
	if text == "" {
		text = "No provider error details were available."
	}
	if maxLen > 0 && len(text) > maxLen {
		runes := []rune(text)
		if len(runes) <= maxLen {
			return text
		}
		if maxLen > 3 {
			return strings.TrimSpace(string(runes[:maxLen-3])) + "..."
		}
		return string(runes[:maxLen])
	}
	return text
}

func valueOrUnknown(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "unknown"
	}
	return value
}

var nonSlugChars = regexp.MustCompile(`[^a-z0-9]+`)

func slug(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	if value == "" {
		return "unknown"
	}
	value = nonSlugChars.ReplaceAllString(value, "_")
	value = strings.Trim(value, "_")
	if value == "" {
		return "unknown"
	}
	return value
}
