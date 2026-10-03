package server

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"strings"
	"time"

	"aurago/internal/newspaper"
	"aurago/internal/tools"
)

type newspaperSearchBatch struct {
	Hits             []newspaperHit
	NextRequestAfter time.Duration
}

func (r *newspaperResearchRun) search(ctx context.Context, query newspaperQuery, round int) []newspaperHit {
	backends := []string{"brave_news", "brave_web", "ddg_search"}
	if query.Kind == "web" {
		backends = backends[1:]
	}
	freshness := "pd"
	if round > 0 {
		freshness = "pw"
	}
	for _, backend := range backends {
		tool := backend
		if strings.HasPrefix(backend, "brave_") {
			tool = "brave_search"
		}
		if !r.allowed(tool) || r.unavailable[backend] {
			continue
		}
		// A long provider cooldown must not consume the remaining research time
		// when an independent search backend is available.
		if time.Until(r.nextSearch[tool]) > 30*time.Second {
			continue
		}
		key := backend + "|" + query.Text + "|" + query.Language + "|" + freshness
		if r.searched[key] {
			continue
		}
		r.searched[key] = true
		for attempt := 0; attempt < 2; attempt++ {
			if !r.canSearch(ctx) {
				return nil
			}
			if err := r.io.Wait(ctx, time.Until(r.nextSearch[tool])); err != nil {
				return nil
			}
			// Check again after pacing: a permission or limit may have changed.
			if !r.allowed(tool) || !r.canSearch(ctx) {
				break
			}
			r.stats.Searches++
			batch, err := r.io.Search(ctx, backend, query, freshness)
			delay := batch.NextRequestAfter
			if delay < time.Second {
				delay = time.Second
			}
			r.nextSearch[tool] = time.Now().Add(delay)
			if err == nil {
				r.stats.Tools[tool] = "ready"
				r.report("finding", nil)
				if len(batch.Hits) > 0 {
					return batch.Hits
				}
				break
			}
			code, temporary := "failed", false
			var brave *tools.BraveSearchError
			if errors.As(err, &brave) {
				code, temporary, delay = brave.Code, brave.Temporary, brave.RetryAfter
				if code == "access_denied" || code == "quota_exhausted" {
					r.unavailable[backend] = true
				}
				if code == "quota_exhausted" {
					r.unavailable["brave_news"], r.unavailable["brave_web"] = true, true
				}
			}
			var network net.Error
			if errors.As(err, &network) && network.Timeout() && ctx.Err() == nil {
				temporary = true
			}
			r.stats.Tools[tool] = code
			r.reject("search_" + code)
			r.report("finding", nil)
			if delay < time.Second {
				delay = time.Second
			}
			r.nextSearch[tool] = time.Now().Add(delay)
			if !temporary || attempt != 0 || delay > 30*time.Second || ctx.Err() != nil {
				break
			}
		}
	}
	return nil
}

func (r *newspaperResearchRun) discover(ctx context.Context, topics []newspaperTopic, round int) {
	if !r.canSearch(ctx) || len(topics) == 0 {
		return
	}
	r.stats.Plans++
	deadline, _ := ctx.Deadline()
	remaining := newspaperPlanBudget{Searches: r.maxSearches - r.stats.Searches, Pages: r.maxPages - r.stats.Pages, Seconds: max(0, int(time.Until(deadline).Seconds())), Plans: 2 - r.stats.Plans}
	if r.io.Spending != nil {
		remaining.Spending = r.io.Spending()
	}
	queries := planNewspaperSearch(ctx, r.profile, topics, r.initial, r.cutoff, round, remaining, r.stats, r.complete)
	for _, query := range queries {
		if !r.canSearch(ctx) {
			break
		}
		hits := r.search(ctx, query, round)
		for rank, hit := range hits {
			r.admit(newspaperCandidate{Hit: hit, Query: query, Rank: rank})
		}
		r.report("finding", nil)
	}
}

func (r *newspaperResearchRun) feeds(ctx context.Context) {
	for _, feed := range r.profile.RSSFeeds {
		if !r.allowed("rss") || !r.canRead(ctx) {
			return
		}
		r.stats.Pages++
		hits, err := r.io.Feed(ctx, feed.URL)
		if err != nil {
			r.reject("feed_failed")
			r.stats.Tools["rss"] = "failed"
			continue
		}
		r.stats.Tools["rss"] = "ready"
		for rank, hit := range hits {
			// Interest feeds are leads for each matching interest, not automatic
			// evidence that every free-text topic has been covered.
			topic := feed.Section
			if topic == "interests" {
				for _, t := range r.topics {
					if t.Section != "interests" || !newspaperMatchesInterest(hit.Title+" "+hit.Description, t.Label) {
						continue
					}
					r.admit(newspaperCandidate{Hit: hit, Query: newspaperQuery{Section: "interests", Topic: t.ID, Text: t.Label, Language: r.profile.Language}, Rank: rank})
				}
				continue
			}
			r.admit(newspaperCandidate{Hit: hit, Query: newspaperQuery{Section: feed.Section, Topic: topic, Language: r.profile.Language}, Rank: rank})
		}
		r.report("finding", nil)
	}
}

func newspaperMatchesInterest(text, interest string) bool {
	text = strings.ToLower(text)
	for _, term := range newspaperTerms(interest) {
		if strings.Contains(text, term) {
			return true
		}
	}
	return false
}

func newspaperSearchAdapter(ctx context.Context, apiKey, country, backend string, query newspaperQuery, freshness string) (newspaperSearchBatch, error) {
	if backend != "ddg_search" {
		kind := strings.TrimPrefix(backend, "brave_")
		page, err := tools.SearchBrave(ctx, apiKey, tools.BraveSearchOptions{Query: query.Text, Country: country, Language: query.Language, Kind: kind, Freshness: freshness, Count: 20})
		batch := newspaperSearchBatch{NextRequestAfter: page.NextRequestAfter}
		for _, hit := range page.Results {
			batch.Hits = append(batch.Hits, newspaperHit{Title: hit.Title, URL: hit.URL, Published: hit.Published, Description: hit.Description})
		}
		return batch, err
	}
	raw := tools.ExecuteDDGSearch(query.Text, 20, ctx)
	var response struct {
		Status  string                                  `json:"status"`
		Results []struct{ Title, Link, Snippet string } `json:"results"`
	}
	if json.Unmarshal([]byte(raw), &response) != nil || response.Status != "success" {
		return newspaperSearchBatch{}, errors.New("DuckDuckGo search unavailable")
	}
	batch := newspaperSearchBatch{}
	for _, hit := range response.Results {
		link := hit.Link
		// DDG can return its own redirect URLs. Extract the original destination
		// locally; the normal public-URL checks still apply before retrieval.
		link = newspaperDDGLink(link)
		batch.Hits = append(batch.Hits, newspaperHit{Title: newspaperUnwrap(hit.Title), URL: link, Description: newspaperUnwrap(hit.Snippet)})
	}
	return batch, nil
}

func newspaperWait(ctx context.Context, delay time.Duration) error {
	if delay <= 0 {
		return ctx.Err()
	}
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

// Kept as a small value copy: only the coordinator publishes progress.
func cloneNewspaperStats(stats newspaper.ResearchStats) *newspaper.ResearchStats {
	copy := stats
	copy.Rejected, copy.Coverage, copy.Tools = map[string]int{}, map[string]int{}, map[string]string{}
	for k, v := range stats.Rejected {
		copy.Rejected[k] = v
	}
	for k, v := range stats.Coverage {
		copy.Coverage[k] = v
	}
	for k, v := range stats.Tools {
		copy.Tools[k] = v
	}
	copy.Gaps = append([]string(nil), stats.Gaps...)
	return &copy
}
