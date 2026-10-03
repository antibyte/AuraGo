package server

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"math"
	"net/url"
	"strings"
	"time"

	"aurago/internal/newspaper"
	"aurago/internal/scraper"
)

// The queue holds bounded leads; accepted evidence is stored independently.
const newspaperCandidateLimit = 200

type newspaperCandidate struct {
	Hit         newspaperHit
	Query       newspaperQuery
	Depth, Rank int
}

// Dependencies are per run, so fixtures do not replace global network clients.
type newspaperResearchIO struct {
	Capabilities func() newspaperResearchCapabilities
	Limits       func() (int, int)
	Spending     func() *newspaperSpendingBudget
	Search       func(context.Context, string, newspaperQuery, string) (newspaperSearchBatch, error)
	Feed         func(context.Context, string) ([]newspaperHit, error)
	Fetch        func(context.Context, string) (*scraper.ScrapeResult, error)
	Complete     newspaperCompletionFunc
	Wait         func(context.Context, time.Duration) error
	Recent       []newspaper.Edition
}

type newspaperResearchRun struct {
	profile                           newspaper.Profile
	topics                            []newspaperTopic
	cutoff                            time.Time
	initial                           newspaperResearchCapabilities
	io                                newspaperResearchIO
	progress                          func(newspaper.Progress)
	stats                             newspaper.ResearchStats
	draft                             newspaper.Draft
	maxPages, maxSearches, maxStories int
	pending                           []newspaperCandidate
	seen                              map[[32]byte]bool
	recent                            map[string]bool
	titles                            map[string]bool
	contents                          map[[32]byte]bool
	searched, unavailable             map[string]bool
	nextSearch                        map[string]time.Time
	domains                           map[string]int
	cursor                            int
	complete                          newspaperCompletionFunc
}

func (s *Server) newspaperResearch(ctx context.Context, p newspaper.Profile, cutoff time.Time, progress func(newspaper.Progress)) (newspaper.Draft, error) {
	cfg := s.ConfigSnapshot().Clone()
	caps := resolveNewspaperCapabilities(cfg, p, s.newspaperSkillReady, s.LLMClient != nil)
	if !caps.Ready {
		return newspaper.Draft{}, fmt.Errorf("newspaper research unavailable: %s", caps.Reason)
	}
	client := s.LLMClient
	deps := newspaperResearchIO{
		Capabilities: func() newspaperResearchCapabilities {
			return resolveNewspaperCapabilities(s.ConfigSnapshot(), p, s.newspaperSkillReady, client != nil)
		},
		Limits: func() (int, int) {
			live := s.ConfigSnapshot()
			if live == nil {
				return 0, 0
			}
			return newspaperPageLimit(live.Newspaper.MaxPages), live.Newspaper.EffectiveMaxSearches()
		},
		Feed: fetchNewspaperFeed,
		Fetch: func(ctx context.Context, raw string) (*scraper.ScrapeResult, error) {
			return scraper.New(s.Guardian).WithContext(ctx).FetchStatic(raw)
		},
		Wait: newspaperWait,
	}
	deps.Spending = func() *newspaperSpendingBudget {
		if s.BudgetTracker == nil {
			return nil
		}
		status := s.BudgetTracker.GetStatus()
		if !status.Enabled || status.DailyLimit <= 0 {
			return nil
		}
		return &newspaperSpendingBudget{RemainingUSD: math.Max(0, status.DailyLimit-status.SpentUSD), Blocked: s.BudgetTracker.IsBlocked("newspaper")}
	}
	deps.Search = func(ctx context.Context, backend string, query newspaperQuery, freshness string) (newspaperSearchBatch, error) {
		live := s.ConfigSnapshot()
		tool := "brave_search"
		if backend == "ddg_search" {
			tool = "ddg_search"
		}
		if live == nil || !caps.allows(tool) || !resolveNewspaperCapabilities(live, p, s.newspaperSkillReady, client != nil).allows(tool) {
			return newspaperSearchBatch{}, errors.New("research permission revoked")
		}
		country := p.Country
		if query.Section == "international" || query.Language != p.Language {
			country = "ALL"
		}
		return newspaperSearchAdapter(ctx, live.BraveSearch.APIKey, country, backend, query, freshness)
	}
	deps.Complete = func(ctx context.Context, guide, input string) (string, error) {
		if !deps.Capabilities().Ready {
			return "", errors.New("research permission revoked")
		}
		return s.newspaperCompletion(ctx, cfg, client, guide, input)
	}
	if s.Newspaper != nil {
		deps.Recent, _ = s.Newspaper.List(ctx, 7)
	}
	minutes := cfg.Newspaper.MaxMinutes
	if minutes < 1 || minutes > 60 {
		minutes = 30
	}
	work, cancel := context.WithTimeout(ctx, time.Duration(minutes)*time.Minute)
	defer cancel()
	return runNewspaperResearch(work, p, cutoff, newspaperPageLimit(cfg.Newspaper.MaxPages), cfg.Newspaper.EffectiveMaxSearches(), deps, progress)
}

func newspaperPageLimit(limit int) int {
	if limit < 1 || limit > 60 {
		return 60
	}
	return limit
}

func runNewspaperResearch(ctx context.Context, p newspaper.Profile, cutoff time.Time, maxPages, maxSearches int, deps newspaperResearchIO, progress func(newspaper.Progress)) (newspaper.Draft, error) {
	r := &newspaperResearchRun{
		profile: p, topics: newspaperTopics(p), cutoff: cutoff, initial: deps.Capabilities(), io: deps, progress: progress,
		maxPages: maxPages, maxSearches: maxSearches, maxStories: map[string]int{"brief": 6, "standard": 12, "in_depth": 16}[p.Length],
		stats: newspaper.ResearchStats{Rejected: map[string]int{}, Coverage: map[string]int{}, Tools: map[string]string{}},
		draft: newspaper.Draft{Stories: []newspaper.Story{}, Sources: []newspaper.Source{}},
		seen:  map[[32]byte]bool{}, recent: map[string]bool{}, titles: map[string]bool{}, contents: map[[32]byte]bool{},
		searched: map[string]bool{}, unavailable: map[string]bool{}, nextSearch: map[string]time.Time{}, domains: map[string]int{},
	}
	if r.maxStories == 0 {
		r.maxStories = 12
	}
	if deps.Wait == nil {
		r.io.Wait = newspaperWait
	}
	if progress == nil {
		r.progress = func(newspaper.Progress) {}
	}
	if !r.initial.Ready {
		return r.draft, errors.New("research permission unavailable")
	}
	r.complete = func(ctx context.Context, guide, input string) (string, error) {
		if !r.initial.Ready || !r.io.Capabilities().Ready {
			return "", errors.New("research permission revoked")
		}
		if r.spendingBlocked() {
			return "", errors.New("provider spending policy blocks research")
		}
		return r.io.Complete(ctx, guide, input)
	}
	for _, tool := range r.initial.Tools {
		r.stats.Tools[tool.ID] = tool.State
	}
	for _, edition := range deps.Recent {
		for _, source := range edition.Sources {
			if canonical, err := canonicalNewspaperURL(source.URL); err == nil {
				r.recent[canonical] = true
			}
		}
	}
	now := time.Now()
	deadline, ok := ctx.Deadline()
	if !ok {
		deadline = now.Add(30 * time.Minute)
	}
	discovery, stopDiscovery := context.WithDeadline(ctx, now.Add(deadline.Sub(now)*4/5))
	defer stopDiscovery()
	r.feeds(discovery)
	for round := 0; round < 2; round++ {
		missing := r.missingTopics()
		if len(missing) == 0 && len(r.draft.Stories) >= r.maxStories {
			break
		}
		if len(missing) == 0 {
			missing = r.topics
		}
		r.discover(discovery, missing, round)
		r.readAndWrite(ctx, discovery, round)
		if ctx.Err() != nil || !r.io.Capabilities().Ready {
			break
		}
	}
	r.stats.Gaps = []string{}
	for _, topic := range r.missingTopics() {
		label := topic.ID
		if topic.Section == "interests" {
			label = topic.Label
		}
		r.stats.Gaps = append(r.stats.Gaps, label)
	}
	r.draft.Partial = len(r.stats.Gaps) > 0 || ctx.Err() != nil || discovery.Err() != nil || r.spendingBlocked()
	r.report("checking", nil)
	if !r.io.Capabilities().Ready {
		return r.draft, errors.New("research permission revoked")
	}
	if ctx.Err() != nil {
		return r.draft, ctx.Err()
	}
	if len(r.draft.Stories) == 0 {
		if r.spendingBlocked() {
			return r.draft, errors.New("provider spending policy blocks research")
		}
		return r.draft, fmt.Errorf("no verified articles (%d candidates, %d pages read; model errors: %d, invalid JSON: %d, evidence or draft rejections: %d)", r.stats.Candidates, r.stats.Read, r.stats.Rejected["model_error"], r.stats.Rejected["invalid_json"], r.stats.Rejected["invalid_draft"])
	}
	return r.draft, newspaper.ValidateDraft(r.draft, p, time.Now().UTC())
}

func (r *newspaperResearchRun) allowed(tool string) bool {
	return r.initial.allows(tool) && r.io.Capabilities().allows(tool)
}

func (r *newspaperResearchRun) limits() {
	if r.io.Limits != nil {
		pages, searches := r.io.Limits()
		r.maxPages, r.maxSearches = min(r.maxPages, pages), min(r.maxSearches, searches)
	}
}

func (r *newspaperResearchRun) canSearch(ctx context.Context) bool {
	r.limits()
	return ctx.Err() == nil && !r.spendingBlocked() && r.io.Capabilities().Ready && r.stats.Searches < r.maxSearches
}

func (r *newspaperResearchRun) canRead(ctx context.Context) bool {
	r.limits()
	return ctx.Err() == nil && !r.spendingBlocked() && r.allowed("web_scraper") && r.stats.Pages < r.maxPages
}

func (r *newspaperResearchRun) spendingBlocked() bool {
	if r.io.Spending == nil {
		return false
	}
	budget := r.io.Spending()
	return budget != nil && budget.Blocked
}

func (r *newspaperResearchRun) reject(code string) { r.stats.Rejected[code]++ }

func (r *newspaperResearchRun) report(phase string, source *newspaper.Source) {
	r.stats.Accepted = len(r.draft.Stories)
	r.progress(newspaper.Progress{Phase: phase, Sources: len(r.draft.Sources), Stories: len(r.draft.Stories), Source: source, Research: cloneNewspaperStats(r.stats)})
}

func (r *newspaperResearchRun) missingTopics() []newspaperTopic {
	missing := []newspaperTopic{}
	for _, topic := range r.topics {
		if r.stats.Coverage[topic.ID] == 0 {
			missing = append(missing, topic)
		}
	}
	return missing
}

func (r *newspaperResearchRun) admit(candidate newspaperCandidate) {
	canonical, err := canonicalNewspaperURL(candidate.Hit.URL)
	if err != nil || len(canonical) > 2048 {
		r.reject("unsafe_url")
		return
	}
	fingerprint := sha256.Sum256([]byte(canonical))
	if r.seen[fingerprint] || r.recent[canonical] {
		r.reject("duplicate")
		return
	}
	if newspaperExcluded(candidate.Hit.Title+" "+candidate.Hit.Description, r.profile.Exclusions) {
		r.reject("excluded")
		return
	}
	if at := newspaperDate(candidate.Hit.Published); at != nil && (at.Before(r.cutoff.Add(-7*24*time.Hour)) || at.After(r.cutoff.Add(time.Hour))) {
		r.reject("stale")
		return
	}
	// A broad feed or query must leave space for every other selected topic.
	perTopic := max(1, newspaperCandidateLimit/max(1, len(r.topics)))
	count := 0
	for _, item := range r.pending {
		if item.Query.Topic == candidate.Query.Topic {
			count++
		}
	}
	if len(r.pending) >= newspaperCandidateLimit || count >= perTopic {
		r.reject("candidate_limit")
		return
	}
	candidate.Hit.URL, candidate.Hit.Title = canonical, newspaperBound(newspaperUnwrap(candidate.Hit.Title), 180)
	candidate.Hit.Description = newspaperBound(newspaperUnwrap(candidate.Hit.Description), 500)
	r.seen[fingerprint] = true
	r.pending = append(r.pending, candidate)
	r.stats.Candidates++
}

func (r *newspaperResearchRun) nextCandidate(round int, busy map[string]bool) (newspaperCandidate, bool) {
	for n := 0; n < len(r.topics); n++ {
		topic := r.topics[r.cursor%len(r.topics)]
		r.cursor = (r.cursor + 1) % len(r.topics)
		best, bestScore := -1, -1<<30
		for i, candidate := range r.pending {
			domain := newspaperPublisherKey(candidate.Hit.URL)
			if candidate.Query.Topic != topic.ID || busy[domain] {
				continue
			}
			published := newspaperDate(candidate.Hit.Published)
			if round == 0 && published != nil && published.Before(r.cutoff.Add(-24*time.Hour)) {
				continue
			}
			score := 100 - candidate.Rank - r.domains[domain]*8
			if published != nil && published.After(r.cutoff.Add(-24*time.Hour)) {
				score += 10
			}
			if topic.Section == "interests" && newspaperMatchesInterest(candidate.Hit.Title+" "+candidate.Hit.Description, topic.Label) {
				score += 20
			}
			if score > bestScore {
				best, bestScore = i, score
			}
		}
		if best >= 0 {
			item := r.pending[best]
			r.pending = append(r.pending[:best], r.pending[best+1:]...)
			return item, true
		}
	}
	return newspaperCandidate{}, false
}

func newspaperDDGLink(raw string) string {
	if strings.HasPrefix(raw, "//") {
		raw = "https:" + raw
	}
	u, err := url.Parse(raw)
	if err == nil && (u.Hostname() == "duckduckgo.com" || strings.HasSuffix(u.Hostname(), ".duckduckgo.com")) {
		if target := u.Query().Get("uddg"); target != "" {
			return target
		}
	}
	return raw
}
