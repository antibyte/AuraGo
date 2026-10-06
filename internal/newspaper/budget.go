package newspaper

import (
	"context"
	"strings"
)

// BudgetConfig contains the legacy absolute limits used by fixed mode.
// Empty Mode intentionally means fixed for existing installations.
type BudgetConfig struct {
	Mode        string
	MaxPages    int
	MaxSearches int
	MaxMinutes  int
}

// Budget is the immutable per-run allowance shared by scheduling and research.
type Budget struct {
	Mode        string `json:"mode"`
	Topics      int    `json:"topics"`
	Pages       int    `json:"pages"`
	Searches    int    `json:"searches"`
	Overviews   int    `json:"overviews"`
	Minutes     int    `json:"minutes"`
	Candidates  int    `json:"candidates"`
	Stories     int    `json:"stories"`
	EditorCalls int    `json:"editor_calls"`
	SharedPages bool   `json:"shared_pages"`
}

type budgetContextKey struct{}

// WithBudget stores the run's resolved budget in its research context.
func WithBudget(ctx context.Context, budget Budget) context.Context {
	return context.WithValue(ctx, budgetContextKey{}, budget)
}

// BudgetFromContext returns the immutable budget assigned when the run started.
func BudgetFromContext(ctx context.Context) (Budget, bool) {
	budget, ok := ctx.Value(budgetContextKey{}).(Budget)
	return budget, ok
}

// ResolveBudget is the single profile-aware resolver used by the scheduler and
// the research runner. Fixed mode retains legacy absolute limits; auto mode
// scales request ceilings by the selected topic count.
func ResolveBudget(p Profile, cfg BudgetConfig) Budget {
	topics := profileTopicCount(p)
	length := storyLengthCeiling(p.Length)
	if strings.EqualFold(strings.TrimSpace(cfg.Mode), "auto") {
		stories := min(31, max(length, topics))
		return Budget{
			Mode:        "auto",
			Topics:      topics,
			Pages:       min(400, 24+12*topics),
			Searches:    min(128, 8+4*topics),
			Overviews:   min(64, 2+2*topics),
			Minutes:     min(60, max(20, 10+2*topics)),
			Candidates:  min(800, max(80, 32*topics)),
			Stories:     stories,
			EditorCalls: min(124, 4*stories),
		}
	}

	pages := cfg.MaxPages
	if pages < 1 || pages > 60 {
		pages = 60
	}
	searches := cfg.MaxSearches
	if searches == 0 {
		searches = 32
	}
	searches = max(1, min(64, searches))
	minutes := cfg.MaxMinutes
	if minutes < 1 || minutes > 60 {
		minutes = 30
	}
	return Budget{
		Mode:        "fixed",
		Topics:      topics,
		Pages:       pages,
		Searches:    searches,
		Overviews:   min(pages, 2+2*topics),
		Minutes:     minutes,
		Candidates:  200,
		Stories:     length,
		EditorCalls: 4 * length,
		SharedPages: true,
	}
}

func profileTopicCount(p Profile) int {
	seen := make(map[string]struct{}, len(p.Sections)+len(p.Interests))
	for _, section := range p.Sections {
		seen["section:"+strings.ToLower(strings.TrimSpace(section))] = struct{}{}
	}
	for _, interest := range p.Interests {
		seen["interest:"+strings.ToLower(strings.TrimSpace(interest))] = struct{}{}
	}
	return min(31, max(1, len(seen)))
}

func storyLengthCeiling(length string) int {
	switch strings.ToLower(strings.TrimSpace(length)) {
	case "brief":
		return 6
	case "in_depth":
		return 16
	default:
		return 12
	}
}
