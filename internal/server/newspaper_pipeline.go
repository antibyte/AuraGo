package server

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"html"
	"strings"
	"time"

	"aurago/internal/newspaper"
	"aurago/internal/scraper"
)

type newspaperReadResult struct {
	candidate newspaperCandidate
	round     int
	page      *scraper.ScrapeResult
	err       error
}
type newspaperArticle struct {
	candidate newspaperCandidate
	source    newspaper.Source
	topic     string
	repair    string
}
type newspaperWriteResult struct {
	article newspaperArticle
	story   newspaper.Story
	err     error
}

// Workers only return values. The coordinator alone owns queues, deduplication,
// budgets, progress persistence and accepted edition data.
func (r *newspaperResearchRun) readAndWrite(ctx, discovery context.Context, round int) {
	reads := make(chan newspaperReadResult, 4)
	writes := make(chan newspaperWriteResult, 2)
	work, stopWork := context.WithCancel(ctx)
	defer stopWork()
	allReads, stopAllReads := context.WithCancel(discovery)
	defer stopAllReads()
	readCtx, stopReads := context.WithDeadline(allReads, r.firstDeadline)
	defer stopReads()
	busy := map[string]bool{}
	editing := map[string]int{}
	ready := []newspaperArticle{}
	reading, writing := 0, 0
	done := ctx.Done()
	phase := time.NewTimer(time.Until(r.firstDeadline))
	defer phase.Stop()
	phaseDone := phase.C
	for {
		r.limits()
		authorized := r.io.Capabilities().Ready
		if !authorized || ctx.Err() != nil || len(r.draft.Stories) >= r.maxStories {
			stopAllReads()
			stopWork()
			ready = nil
		}
		// Cross the round boundary without draining editors. Workers and publisher
		// leases remain shared; slow first-round reads expire at the half-time boundary.
		if round == 0 && authorized && ctx.Err() == nil && len(r.draft.Stories) < r.maxStories &&
			(readCtx.Err() != nil || !r.canRead(readCtx) || (reading == 0 && !r.hasCandidate(0))) {
			readCtx, stopReads = context.WithCancel(allReads)
			defer stopReads()
			phaseDone = nil
			round, r.round = 1, 1
			ready = append(ready, r.deferred...)
			r.deferred = nil
			missing := r.missingTopics()
			if len(missing) == 0 {
				missing = r.topics
			}
			r.overviews(discovery, 1)
			r.discover(discovery, missing, 1)
		}
		for authorized && work.Err() == nil && !r.spendingBlocked() && writing < 2 && len(ready) > 0 && len(r.draft.Stories)+writing < r.maxStories {
			roomForReads := reading+len(ready) < max(6, 2*len(r.topics)) && len(r.draft.Stories)+writing+reading+len(ready) < r.maxStories+len(r.topics)
			index := r.nextArticle(ready, editing, reading > 0 || roomForReads && r.canRead(readCtx) && r.hasCandidate(round))
			if index < 0 {
				break
			}
			article := ready[index]
			ready = append(ready[:index], ready[index+1:]...)
			reserve := max(0, min(r.budget.Topics, r.budget.Stories)-r.stats.Repairs)
			if r.stats.EditorCalls >= r.budget.EditorCalls || article.repair == "" && r.stats.EditorCalls >= r.budget.EditorCalls-reserve {
				r.reject("budget_exhausted")
				continue
			}
			if article.repair != "" {
				r.stats.Repairs++
			}
			r.stats.EditorCalls++
			writing++
			editing[article.candidate.Query.Topic]++
			r.report("editing", nil)
			go func(article newspaperArticle) {
				story, err := writeNewspaperStoryRepair(work, r.profile, article.candidate.Query.Section, article.topic, article.source, r.complete, article.repair)
				writes <- newspaperWriteResult{article: article, story: story, err: err}
			}(article)
		}
		for authorized && r.canRead(readCtx) && reading < 4 && reading+len(ready) < max(6, 2*len(r.topics)) && len(r.draft.Stories)+writing+reading+len(ready) < r.maxStories+len(r.topics) {
			candidate, ok := r.nextCandidate(round, busy)
			if !ok {
				break
			}
			domain := newspaperPublisherKey(candidate.Hit.URL)
			busy[domain] = true
			r.domains[domain]++
			r.stats.Pages++
			r.count(candidate.Query.Topic, round, "pages")
			reading++
			r.report("reading", nil)
			go func(candidate newspaperCandidate, requestCtx context.Context, requestRound int) {
				if !r.allowed("web_scraper") {
					reads <- newspaperReadResult{candidate: candidate, round: requestRound, err: errors.New("research permission revoked")}
					return
				}
				page, err := r.io.Fetch(requestCtx, candidate.Hit.URL)
				reads <- newspaperReadResult{candidate: candidate, round: requestRound, page: page, err: err}
			}(candidate, readCtx, round)
		}
		if reading == 0 && writing == 0 {
			if len(ready) > 0 && work.Err() == nil && authorized && !r.spendingBlocked() && len(r.draft.Stories) < r.maxStories {
				continue
			}
			return
		}
		select {
		case <-phaseDone:
			phaseDone = nil
			stopReads()
		case <-done:
			done = nil
			stopWork()
			stopReads()
		case result := <-reads:
			reading--
			delete(busy, newspaperPublisherKey(result.candidate.Hit.URL))
			if result.err != nil || result.page == nil {
				r.reject("read_failed")
				r.stats.Tools["web_scraper"] = "failed"
				continue
			}
			r.stats.Tools["web_scraper"] = "ready"
			r.stats.Read++
			r.count(result.candidate.Query.Topic, result.round, "read")
			if !r.io.Capabilities().Ready || ctx.Err() != nil {
				continue
			}
			// Once in the wider phase, late first-round results also use that window.
			if article, ok := r.capture(result, round); ok {
				ready = append(ready, article)
				r.report("reading", &article.source)
			}
		case result := <-writes:
			writing--
			editing[result.article.candidate.Query.Topic]--
			if !r.io.Capabilities().Ready || ctx.Err() != nil {
				continue
			}
			reason := newspaperEditFailure(result.err)
			if reason == "" {
				reason = newspaperValidateStory(&result.story, result.article, r.profile, len(r.draft.Stories)+1)
			}
			if reason != "" {
				r.reject(reason)
				repairsQueued := 0
				for _, article := range ready {
					if article.repair != "" {
						repairsQueued++
					}
				}
				if newspaperRepairable(reason) && result.article.repair == "" && r.stats.Repairs+repairsQueued < r.budget.Topics && r.stats.EditorCalls < r.budget.EditorCalls && !r.spendingBlocked() {
					result.article.repair = reason
					ready = append(ready, result.article)
				}
				continue
			}
			if len(r.draft.Stories) >= r.maxStories {
				continue
			}
			r.draft.Stories = append(r.draft.Stories, result.story)
			r.draft.Sources = append(r.draft.Sources, result.article.source)
			r.stats.Coverage[result.article.candidate.Query.Topic]++
			r.count(result.article.candidate.Query.Topic, round, "accepted")
			r.report("editing", nil)
		}
	}
}

func (r *newspaperResearchRun) hasCandidate(round int) bool {
	for _, c := range r.pending {
		at := newspaperDate(c.Hit.Published)
		if round > 0 || at == nil || !at.Before(r.cutoff.Add(-24*time.Hour)) {
			return true
		}
	}
	return false
}

func (r *newspaperResearchRun) nextArticle(ready []newspaperArticle, editing map[string]int, moreReads bool) int {
	for i := range ready {
		a := &ready[i]
		if a.repair == "" && r.stats.Coverage[a.candidate.Query.Topic] > 0 {
			for _, query := range r.associations[a.candidate.Hit.URL] {
				if r.stats.Coverage[query.Topic] != 0 || editing[query.Topic] != 0 {
					continue
				}
				a.candidate.Query = query
				for _, topic := range r.topics {
					if topic.ID == query.Topic {
						a.topic = topic.Label
						break
					}
				}
				break
			}
		}
		if r.stats.Coverage[a.candidate.Query.Topic] == 0 && editing[a.candidate.Query.Topic] == 0 {
			return i
		}
	}
	if moreReads && len(r.missingTopics()) > 0 {
		return -1
	}
	if len(ready) > 0 {
		return 0
	}
	return -1
}

func (r *newspaperResearchRun) count(topic string, round int, kind string) {
	increment := func(c newspaper.ResearchCounts) newspaper.ResearchCounts {
		switch kind {
		case "searches":
			c.Searches++
		case "pages":
			c.Pages++
		case "read":
			c.Read++
		case "accepted":
			c.Accepted++
		}
		return c
	}
	if r.stats.Topics == nil {
		r.stats.Topics = map[string]newspaper.ResearchCounts{}
	}
	r.stats.Topics[topic] = increment(r.stats.Topics[topic])
	if len(r.stats.Rounds) == 2 {
		r.stats.Rounds[min(round, 1)] = increment(r.stats.Rounds[min(round, 1)])
	}
}

func (r *newspaperResearchRun) capture(result newspaperReadResult, round int) (newspaperArticle, bool) {
	candidate, page := result.candidate, result.page
	meta := newspaperMetadata(page.RawHTML)
	links := newspaperOverviewLinks(candidate.Hit.URL, page, meta)
	if links != nil {
		if candidate.Depth == 0 {
			for rank, hit := range links {
				r.admit(newspaperCandidate{Hit: hit, Query: candidate.Query, Rank: rank, Depth: 1})
			}
		}
		r.reject("overview")
		return newspaperArticle{}, false
	}
	title := newspaperBound(page.Title, 180)
	if meta.Title != "" {
		title = newspaperBound(meta.Title, 180)
	}
	body := html.UnescapeString(newspaperUnwrap(page.Markdown))
	if len([]rune(body)) < 180 || title == "" || title == candidate.Hit.URL {
		r.reject("unreadable")
		return newspaperArticle{}, false
	}
	if newspaperExcluded(title+" "+body, r.profile.Exclusions) {
		r.reject("excluded")
		return newspaperArticle{}, false
	}
	// Literal keyword matching cannot reject translated originals or synonyms.
	// Give the editor the selected interest, not just the generic section label.
	topicLabel := candidate.Query.Section
	for _, topic := range r.topics {
		if topic.ID == candidate.Query.Topic {
			topicLabel = topic.Label
			break
		}
	}
	// Search page_age may be a modification date. Only original article
	// metadata establishes publication; an undated original stays undated.
	published := newspaperPublished("", page.RawHTML)
	if published != nil && (published.Before(r.cutoff.Add(-7*24*time.Hour)) || published.After(r.cutoff.Add(time.Hour))) {
		r.reject("stale")
		return newspaperArticle{}, false
	}
	if published == nil {
		r.reject("unknown_date")
	}

	key := strings.ToLower(strings.Join(strings.Fields(title), " "))
	contentHash := sha256.Sum256([]byte(strings.Join(strings.Fields(body), " ")))
	if r.titles[key] || r.contents[contentHash] {
		r.reject("duplicate")
		return newspaperArticle{}, false
	}
	r.titles[key], r.contents[contentHash] = true, true
	hash := sha256.Sum256([]byte(candidate.Hit.URL))
	publisher := meta.Publisher
	if publisher == "" {
		publisher = newspaperPublisherKey(candidate.Hit.URL)
	}
	source := newspaper.Source{
		ID: "src-" + hex.EncodeToString(hash[:6]), URL: candidate.Hit.URL, Title: title, Publisher: newspaperBound(publisher, 180),
		PublishedAt: published, RetrievedAt: time.Now().UTC(), Excerpt: newspaperEvidenceExcerpt(body, candidate.Query.Text+" "+title),
	}
	article := newspaperArticle{candidate: candidate, source: source, topic: topicLabel}
	if round == 0 && published != nil && published.Before(r.cutoff.Add(-24*time.Hour)) {
		if len(r.deferred) < r.budget.Candidates {
			r.deferred = append(r.deferred, article)
			r.report("reading", &source)
		}
		return newspaperArticle{}, false
	}
	return article, true
}
