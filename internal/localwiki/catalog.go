package localwiki

import (
	"encoding/xml"
	"fmt"
	"math"
	"net/url"
	"path"
	"regexp"
	"strconv"
	"strings"
)

const (
	defaultCatalogBaseURL = "https://opds.library.kiwix.org"
	acquisitionRel        = "http://opds-spec.org/acquisition/open-access"
)

// editionNamePattern matches Kiwix edition names such as
// wikipedia_de_all_nopic_2026-10; a re-run carries a letter suffix (2026-07b).
// The month must be 01 to 12.
var editionNamePattern = regexp.MustCompile(`^wikipedia_([a-z]{2,3})_all_(maxi|nopic)_(\d{4}-(?:0[1-9]|1[0-2]))([a-z]?)$`)

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

// opdsFeed requires an Atom <feed> root, so an HTML 200 answer (a captive
// portal, a proxy error page) is an error instead of an empty catalog.
type opdsFeed struct {
	XMLName xml.Name    `xml:"http://www.w3.org/2005/Atom feed"`
	Entries []opdsEntry `xml:"entry"`
}

// opdsEntry keeps the numbers as text: one malformed value in an entry that is
// not wanted must not fail the whole feed, so they are parsed per entry.
type opdsEntry struct {
	Name         string     `xml:"name"`
	Flavour      string     `xml:"flavour"`
	ArticleCount string     `xml:"articleCount"`
	Links        []opdsLink `xml:"link"`
}

type opdsLink struct {
	Rel    string `xml:"rel,attr"`
	Href   string `xml:"href,attr"`
	Length string `xml:"length,attr"`
}

// parseCount parses a non-negative count; an absent value is 0, a malformed or
// negative one is rejected.
func parseCount(raw string) (int64, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return 0, true
	}
	value, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || value < 0 {
		return 0, false
	}
	return value, true
}

// parseOPDS returns the newest maxi and nopic edition of wikipedia_<kiwix>_all
// from a Kiwix OPDS v2 Atom feed. Relative and protocol-relative acquisition
// links resolve against base; links that are not HTTPS, carry credentials, or
// point to a host meta4HostAllowed refuses (Kiwix hosts and the catalog's own
// host) are ignored, as are entries and links with malformed or negative
// counts. A document that is not an Atom feed is an error.
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
		articles, ok := parseCount(entry.ArticleCount)
		if !ok || articles > math.MaxInt32 {
			continue
		}
		for _, link := range entry.Links {
			if link.Rel != acquisitionRel || !strings.HasSuffix(link.Href, ".zim.meta4") {
				continue
			}
			size, ok := parseCount(link.Length)
			if !ok {
				continue
			}
			href, err := base.Parse(strings.TrimSpace(link.Href))
			if err != nil || href.Scheme != "https" || href.User != nil || !meta4HostAllowed(href, base) {
				continue
			}
			name := strings.TrimSuffix(path.Base(href.Path), ".zim.meta4")
			parsed, ok := parseEditionName(name)
			if !ok || parsed.Kiwix != kiwix || parsed.Variant != variant {
				continue
			}
			candidate := CatalogEdition{Name: name, Date: parsed.Date, Size: size, ArticleCount: int(articles), Meta4URL: href.String()}
			if current, exists := out[variant]; !exists || editionNewer(candidate.Name, current.Name) {
				out[variant] = candidate
			}
		}
	}
	return out, nil
}
