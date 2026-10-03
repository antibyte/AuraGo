package server

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"aurago/internal/llm"
	"aurago/internal/newspaper"
	"aurago/internal/scraper"
)

type newspaperReadResult struct {
	candidate newspaperCandidate
	page      *scraper.ScrapeResult
	err       error
}
type newspaperArticle struct {
	candidate newspaperCandidate
	source    newspaper.Source
	topic     string
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
	readCtx, stopReads := context.WithCancel(discovery)
	defer stopReads()
	busy := map[string]bool{}
	ready := []newspaperArticle{}
	reading, writing := 0, 0
	done := ctx.Done()
	for {
		authorized := r.io.Capabilities().Ready
		if !authorized || ctx.Err() != nil || len(r.draft.Stories) >= r.maxStories {
			stopReads()
			if !authorized || ctx.Err() != nil {
				stopWork()
			}
			ready = nil
		}
		for authorized && work.Err() == nil && writing < 2 && len(ready) > 0 && len(r.draft.Stories)+writing < r.maxStories {
			article := ready[0]
			ready = ready[1:]
			writing++
			r.report("editing", nil)
			go func(article newspaperArticle) {
				story, err := writeNewspaperStory(work, r.profile, article.candidate.Query.Section, article.topic, article.source, r.complete)
				writes <- newspaperWriteResult{article: article, story: story, err: err}
			}(article)
		}
		for authorized && r.canRead(readCtx) && reading < 4 && reading+len(ready) < 6 && len(r.draft.Stories)+writing+reading+len(ready) < r.maxStories+2 {
			candidate, ok := r.nextCandidate(round, busy)
			if !ok {
				break
			}
			domain := newspaperPublisherKey(candidate.Hit.URL)
			busy[domain] = true
			r.domains[domain]++
			r.stats.Pages++
			reading++
			r.report("reading", nil)
			go func(candidate newspaperCandidate) {
				if !r.allowed("web_scraper") {
					reads <- newspaperReadResult{candidate: candidate, err: errors.New("research permission revoked")}
					return
				}
				page, err := r.io.Fetch(readCtx, candidate.Hit.URL)
				reads <- newspaperReadResult{candidate: candidate, page: page, err: err}
			}(candidate)
		}
		if reading == 0 && writing == 0 {
			return
		}
		select {
		case <-done:
			done = nil // Drain each in-flight result exactly once after cancellation.
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
			if !r.io.Capabilities().Ready || ctx.Err() != nil {
				continue
			}
			if article, ok := r.capture(result, round); ok {
				ready = append(ready, article)
				r.report("reading", &article.source)
			}
		case result := <-writes:
			writing--
			if result.err != nil {
				reason := "model_error"
				if errors.Is(result.err, llm.ErrJSONCompletionInvalid) || errors.Is(result.err, llm.ErrJSONCompletionEmpty) || errors.Is(result.err, llm.ErrJSONCompletionTruncated) {
					reason = "invalid_json"
				}
				r.reject(reason)
				continue
			}
			if !r.io.Capabilities().Ready {
				continue
			}
			story, source := result.story, result.article.source
			story.ID, story.Section = fmt.Sprintf("story-%d", len(r.draft.Stories)+1), result.article.candidate.Query.Section
			story.SourceIDs, story.SingleSource = []string{source.ID}, true
			for i := range story.Paragraphs {
				story.Paragraphs[i].SourceIDs = []string{source.ID}
			}
			if err := newspaper.ValidateDraft(newspaper.Draft{Stories: []newspaper.Story{story}, Sources: []newspaper.Source{source}}, r.profile, time.Now().UTC()); err != nil {
				r.reject("invalid_draft")
				continue
			}
			r.draft.Stories = append(r.draft.Stories, story)
			r.draft.Sources = append(r.draft.Sources, source)
			r.stats.Coverage[result.article.candidate.Query.Topic]++
			r.report("editing", nil)
		}
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
	body := newspaperUnwrap(page.Markdown)
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
	if round == 0 && published != nil && published.Before(r.cutoff.Add(-24*time.Hour)) {
		// Keep a bounded lead for the wider follow-up window, but do not label an
		// older source as today's news. A repeated read still costs a page.
		candidate.Hit.Published = published.Format(time.RFC3339)
		if len(r.pending) < newspaperCandidateLimit {
			r.pending = append(r.pending, candidate)
		}
		return newspaperArticle{}, false
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
	return newspaperArticle{candidate: candidate, source: source, topic: topicLabel}, true
}
