package server

import (
	"aurago/internal/personalradio"
	"aurago/internal/scraper"
	"aurago/internal/tools"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"github.com/PuerkitoBio/goquery"
	"github.com/itlightning/dateparse"
	"net/url"
	"strings"
	"time"
)

func (s *Server) personalRadioResearch(ctx context.Context, p personalradio.Station) ([]personalradio.Source, error) {
	cfg, err := s.radioConfig()
	if err != nil {
		return nil, err
	}
	if !cfg.BraveSearch.Enabled || cfg.BraveSearch.APIKey == "" || !cfg.Tools.WebScraper.Enabled || !cfg.Agent.AllowNetworkRequests {
		return nil, errors.New("radio_news_unavailable")
	}
	if s.BudgetTracker != nil && s.BudgetTracker.IsBlocked("personal_radio") {
		return nil, personalradio.ErrLimit
	}
	queries := []string{}
	if p.NewsTopics {
		queries = append(queries, p.Topics+" news "+p.Language)
	}
	if p.International {
		queries = append(queries, "international world news "+p.Language)
	}
	if p.National {
		queries = append(queries, p.Country+" national news "+p.Language)
	}
	if p.Regional {
		queries = append(queries, p.Region+" "+p.Country+" regional news "+p.Language)
	}
	type hit struct{ Title, URL, Published string }
	lists := [][]hit{}
	for _, q := range queries {
		if err = ctx.Err(); err != nil {
			return nil, err
		}
		raw := tools.ExecuteBraveSearch(cfg.BraveSearch.APIKey, q+" "+time.Now().Format("2006-01-02"), 4, p.Country, p.Language, ctx)
		var result struct {
			Status  string `json:"status"`
			Results []hit  `json:"results"`
		}
		if json.Unmarshal([]byte(raw), &result) == nil && result.Status == "success" {
			lists = append(lists, result.Results)
		}
	}
	out := []personalradio.Source{}
	seen := map[string]bool{}
	attempts := 0
	// Round-robin preserves regional coverage. No more than eight page fetches.
	for index := 0; index < 4 && attempts < 8; index++ {
		for _, hits := range lists {
			if index >= len(hits) || attempts >= 8 {
				continue
			}
			h := hits[index]
			u, e := url.Parse(h.URL)
			if e != nil || u.Host == "" || (u.Scheme != "https" && u.Scheme != "http") {
				continue
			}
			u.Fragment = ""
			canonical := u.String()
			if seen[canonical] {
				continue
			}
			seen[canonical] = true
			attempts++
			page, e := scraper.New(s.Guardian).WithContext(ctx).FetchStatic(canonical)
			if e != nil || len([]rune(page.Markdown)) < 100 {
				continue
			}
			published, _ := dateparse.ParseAny(h.Published)
			if doc, e := goquery.NewDocumentFromReader(strings.NewReader(page.RawHTML)); e == nil {
				value, _ := doc.Find("meta[property='article:published_time'],meta[name='date'],meta[itemprop='datePublished']").First().Attr("content")
				if value == "" {
					value, _ = doc.Find("time[datetime]").First().Attr("datetime")
				}
				if parsed, e := dateparse.ParseAny(value); e == nil {
					published = parsed
				}
			}
			now := time.Now()
			if published.IsZero() || published.After(now.Add(time.Hour)) || now.Sub(published) > 24*time.Hour {
				continue
			}
			hash := sha256.Sum256([]byte(canonical))
			out = append(out, personalradio.Source{ID: hex.EncodeToString(hash[:8]), Title: radioBound(page.Title, 180), URL: canonical, Published: published, Retrieved: now, Text: radioBound(page.Markdown, 4500)})
		}
	}
	return out, nil
}
