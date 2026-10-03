package server

import (
	"errors"
	"net/url"
	"strings"
	"time"
)

// Search and feed metadata are leads until an original page is read.
type newspaperHit struct{ Title, URL, Published, Description string }
type newspaperQuery struct {
	Section  string `json:"-"`
	Topic    string `json:"topic"`
	Text     string `json:"query"`
	Language string `json:"language"`
	Kind     string `json:"kind"`
}

func canonicalNewspaperURL(raw string) (string, error) {
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" || (u.Scheme != "https" && u.Scheme != "http") || u.User != nil {
		return "", errors.New("invalid public URL")
	}
	u.Fragment = ""
	u.Scheme = strings.ToLower(u.Scheme)
	u.Host = strings.ToLower(u.Host)
	q := u.Query()
	for key := range q {
		lower := strings.ToLower(key)
		if strings.HasPrefix(lower, "utm_") || lower == "fbclid" || lower == "gclid" {
			q.Del(key)
		}
	}
	u.RawQuery = q.Encode()
	return u.String(), nil
}

func newspaperPublished(raw, html string) *time.Time {
	if at := newspaperDate(newspaperMetadata(html).Published); at != nil {
		return at
	}
	return newspaperDate(raw)
}

func newspaperBound(s string, maxRunes int) string {
	r := []rune(strings.TrimSpace(s))
	if len(r) > maxRunes {
		r = r[:maxRunes]
	}
	return string(r)
}

func newspaperExcluded(text string, terms []string) bool {
	text = strings.ToLower(text)
	for _, term := range terms {
		if strings.Contains(text, strings.ToLower(strings.TrimSpace(term))) {
			return true
		}
	}
	return false
}
