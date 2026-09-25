package server

import (
	"context"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"aurago/internal/security"
)

type newspaperFeedDocument struct {
	Channel struct {
		Items []struct {
			Title string `xml:"title"`
			Link  string `xml:"link"`
			Date  string `xml:"pubDate"`
		} `xml:"item"`
	} `xml:"channel"`
	Entries []struct {
		Title     string `xml:"title"`
		Published string `xml:"published"`
		Updated   string `xml:"updated"`
		Links     []struct {
			Href string `xml:"href,attr"`
			Rel  string `xml:"rel,attr"`
		} `xml:"link"`
	} `xml:"entry"`
}

func parseNewspaperFeed(feedURL string, body []byte) ([]newspaperHit, error) {
	var feed newspaperFeedDocument
	if err := xml.Unmarshal(body, &feed); err != nil {
		return nil, fmt.Errorf("parse RSS or Atom feed: %w", err)
	}
	base, err := url.Parse(feedURL)
	if err != nil {
		return nil, err
	}
	hits := make([]newspaperHit, 0, 12)
	appendHit := func(title, link, published string) {
		if len(hits) >= 12 || strings.TrimSpace(title) == "" || strings.TrimSpace(link) == "" {
			return
		}
		u, err := url.Parse(strings.TrimSpace(link))
		if err != nil {
			return
		}
		absolute, err := canonicalNewspaperURL(base.ResolveReference(u).String())
		if err != nil {
			return
		}
		hits = append(hits, newspaperHit{Title: newspaperBound(title, 180), URL: absolute, Published: strings.TrimSpace(published)})
	}
	for _, item := range feed.Channel.Items {
		appendHit(item.Title, item.Link, item.Date)
	}
	for _, entry := range feed.Entries {
		link := ""
		for _, candidate := range entry.Links {
			if candidate.Rel == "" || candidate.Rel == "alternate" {
				link = candidate.Href
				break
			}
		}
		published := entry.Published
		if published == "" {
			published = entry.Updated
		}
		appendHit(entry.Title, link, published)
	}
	if len(hits) == 0 {
		return nil, errors.New("RSS feed has no article links")
	}
	return hits, nil
}

func fetchNewspaperFeed(ctx context.Context, rawURL string) ([]newspaperHit, error) {
	for redirects := 0; redirects <= 4; redirects++ {
		client, err := security.NewStrictPublicHTTPClientForURL(rawURL, 20*time.Second)
		if err != nil {
			return nil, fmt.Errorf("RSS feed URL is not public: %w", err)
		}
		defer client.CloseIdleConnections()
		request, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
		if err != nil {
			return nil, err
		}
		request.Header.Set("Accept", "application/rss+xml, application/atom+xml, application/xml, text/xml")
		response, err := client.Do(request)
		if err != nil {
			return nil, fmt.Errorf("fetch RSS feed: %w", err)
		}
		if response.StatusCode >= 300 && response.StatusCode < 400 {
			location := response.Header.Get("Location")
			response.Body.Close()
			if location == "" {
				return nil, errors.New("RSS feed redirect has no location")
			}
			base, err := url.Parse(rawURL)
			if err != nil {
				return nil, err
			}
			target, err := url.Parse(location)
			if err != nil {
				return nil, fmt.Errorf("RSS feed redirect URL: %w", err)
			}
			rawURL = base.ResolveReference(target).String()
			continue
		}
		if response.StatusCode != http.StatusOK {
			response.Body.Close()
			return nil, fmt.Errorf("RSS feed returned HTTP %d", response.StatusCode)
		}
		body, err := io.ReadAll(io.LimitReader(response.Body, (1<<20)+1))
		response.Body.Close()
		if err != nil {
			return nil, fmt.Errorf("read RSS feed: %w", err)
		}
		if len(body) > 1<<20 {
			return nil, errors.New("RSS feed exceeds 1 MiB")
		}
		return parseNewspaperFeed(rawURL, body)
	}
	return nil, errors.New("RSS feed redirected too many times")
}
