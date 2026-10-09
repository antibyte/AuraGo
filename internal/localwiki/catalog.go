package localwiki

import (
	"encoding/xml"
	"fmt"
	"net/url"
	"path"
	"regexp"
	"strings"
)

const (
	defaultCatalogBaseURL = "https://opds.library.kiwix.org"
	acquisitionRel        = "http://opds-spec.org/acquisition/open-access"
)

// editionNamePattern matches Kiwix edition names such as
// wikipedia_de_all_nopic_2026-10; a re-run carries a letter suffix (2026-07b).
var editionNamePattern = regexp.MustCompile(`^wikipedia_([a-z]{2,3})_all_(maxi|nopic)_(\d{4}-\d{2})([a-z]?)$`)

type editionName struct {
	Kiwix   string
	Variant Variant
	Date    string
	Suffix  string
}

func parseEditionName(name string) (editionName, bool) {
	match := editionNamePattern.FindStringSubmatch(name)
	if match == nil {
		return editionName{}, false
	}
	return editionName{Kiwix: match[1], Variant: Variant(match[2]), Date: match[3], Suffix: match[4]}, true
}

// editionNewer reports whether edition name a is newer than b. A re-run
// ("2026-07b") is newer than the plain month ("2026-07").
func editionNewer(a, b string) bool {
	pa, okA := parseEditionName(a)
	pb, okB := parseEditionName(b)
	if !okA || !okB {
		return false
	}
	return pa.Date+pa.Suffix > pb.Date+pb.Suffix
}

type opdsFeed struct {
	Entries []opdsEntry `xml:"entry"`
}

type opdsEntry struct {
	Name         string     `xml:"name"`
	Flavour      string     `xml:"flavour"`
	ArticleCount int        `xml:"articleCount"`
	Links        []opdsLink `xml:"link"`
}

type opdsLink struct {
	Rel    string `xml:"rel,attr"`
	Href   string `xml:"href,attr"`
	Length int64  `xml:"length,attr"`
}

// parseOPDS returns the newest maxi and nopic edition of wikipedia_<kiwix>_all
// from a Kiwix OPDS v2 Atom feed. Relative acquisition links resolve against
// base; links that are not HTTPS are ignored.
func parseOPDS(data []byte, kiwix string, base *url.URL) (map[Variant]CatalogEdition, error) {
	var feed opdsFeed
	if err := xml.Unmarshal(data, &feed); err != nil {
		return nil, fmt.Errorf("parse catalog: %w", err)
	}
	want := "wikipedia_" + kiwix + "_all"
	out := make(map[Variant]CatalogEdition, 2)
	for _, entry := range feed.Entries {
		if strings.TrimSpace(entry.Name) != want {
			continue
		}
		variant := Variant(strings.TrimSpace(entry.Flavour))
		if variant != VariantNoPic && variant != VariantMaxi {
			continue
		}
		for _, link := range entry.Links {
			if link.Rel != acquisitionRel || !strings.HasSuffix(link.Href, ".zim.meta4") {
				continue
			}
			href, err := base.Parse(strings.TrimSpace(link.Href))
			if err != nil || href.Scheme != "https" || href.User != nil {
				continue
			}
			name := strings.TrimSuffix(path.Base(href.Path), ".zim.meta4")
			parsed, ok := parseEditionName(name)
			if !ok || parsed.Kiwix != kiwix || parsed.Variant != variant {
				continue
			}
			candidate := CatalogEdition{Name: name, Date: parsed.Date, Size: link.Length, ArticleCount: entry.ArticleCount, Meta4URL: href.String()}
			if current, exists := out[variant]; !exists || editionNewer(candidate.Name, current.Name) {
				out[variant] = candidate
			}
		}
	}
	return out, nil
}
