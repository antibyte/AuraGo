package retrogames

import (
	"bytes"
	"encoding/json"
	"io"
	"regexp"
	"strconv"
	"strings"
)

// emulatorPattern is the shape of an archive.org emulator key ("dosbox",
// "sae-a500p", "np2.v2"); anything else is not a usable emulator.
var emulatorPattern = regexp.MustCompile(`^[a-z0-9._-]+$`)

const (
	maxTitleRunes    = 200
	maxCreatorRunes  = 120
	maxTagsRunes     = 600
	maxEmulatorRunes = 40
)

// Game is one playable archive.org item.
type Game struct {
	Identifier string `json:"identifier"`
	Title      string `json:"title"`
	Year       int    `json:"year,omitempty"`
	Creator    string `json:"creator,omitempty"`
	Platform   string `json:"platform"`
	System     string `json:"system"`
	Emulator   string `json:"emulator,omitempty"`
	Downloads  int64  `json:"downloads,omitempty"`
	Tags       string `json:"-"`
}

// FlexText decodes archive.org metadata values that arrive as a string, a
// number or an (optionally nested) array of those. Booleans, nulls and objects
// decode to nothing. Arrays are flattened at most three levels deep and a
// value keeps at most maxFlexValues entries, so hostile payloads stay cheap.
type FlexText []string

const (
	maxFlexValues = 64
	maxFlexDepth  = 3
)

// UnmarshalJSON implements json.Unmarshaler. It streams tokens instead of
// building a generic value, so a hostile array of millions of strings costs
// time but no memory beyond the maxFlexValues kept entries. Arrays are
// followed while fewer than maxFlexDepth arrays enclose them; deeper arrays
// and every object are consumed without contributing anything.
func (f *FlexText) UnmarshalJSON(data []byte) error {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()
	var out FlexText
	depth := 0 // open arrays whose elements are collected
	skip := 0  // open delimiters inside a skipped object or too-deep array
	for {
		tok, err := dec.Token()
		if err != nil {
			if err == io.EOF {
				err = io.ErrUnexpectedEOF
			}
			return err
		}
		switch t := tok.(type) {
		case json.Delim:
			switch {
			case t == '{', t == '[' && (skip > 0 || depth >= maxFlexDepth):
				skip++
			case t == '[':
				depth++
			case skip > 0:
				skip--
			default:
				depth--
			}
		case string:
			if skip == 0 && len(out) < maxFlexValues {
				out = append(out, t)
			}
		case json.Number:
			if skip == 0 && len(out) < maxFlexValues {
				out = append(out, t.String())
			}
		}
		if depth == 0 && skip == 0 {
			break
		}
	}
	*f = out
	return nil
}

// First returns the first non-empty value.
func (f FlexText) First() string {
	for _, v := range f {
		if v = strings.TrimSpace(v); v != "" {
			return v
		}
	}
	return ""
}

// Join returns all non-empty values joined by sep.
func (f FlexText) Join(sep string) string {
	parts := make([]string, 0, len(f))
	for _, v := range f {
		if v = strings.TrimSpace(v); v != "" {
			parts = append(parts, v)
		}
	}
	return strings.Join(parts, sep)
}

// Item is one record of an archive.org scrape page.
type Item struct {
	Identifier string   `json:"identifier"`
	Title      FlexText `json:"title"`
	Year       FlexText `json:"year"`
	Creator    FlexText `json:"creator"`
	Downloads  FlexText `json:"downloads"`
	Emulator   FlexText `json:"emulator"`
	Subject    FlexText `json:"subject"`
}

func parseCount(s string) int64 {
	s = strings.TrimSpace(s)
	if n, err := strconv.ParseInt(s, 10, 64); err == nil && n > 0 {
		return n
	}
	if f, err := strconv.ParseFloat(s, 64); err == nil && f > 0 && f < 1e15 {
		return int64(f)
	}
	return 0
}

// popRank is the bit length of downloads, a cheap log2 popularity bucket.
func popRank(downloads int64) int {
	rank := 0
	for d := downloads; d > 0; d >>= 1 {
		rank++
	}
	return rank
}

// normalizeItem validates and cleans one scrape record of source src.
func normalizeItem(src Source, it Item) (Game, bool) {
	id := strings.TrimSpace(it.Identifier)
	if !ValidIdentifier(id) {
		return Game{}, false
	}
	rawTitle := collapseSpace(stripControl(it.Title.First()))
	if rawTitle == "" {
		return Game{}, false
	}
	emulator := strings.ToLower(collapseSpace(stripControl(it.Emulator.First())))
	if !emulatorPattern.MatchString(emulator) {
		return Game{}, false
	}
	title, tosecYear := CleanTitle(rawTitle)
	year := ParseYear(it.Year.First())
	if year == 0 {
		year = tosecYear
	}
	tags := collapseSpace(stripControl(it.Title.Join(" ")))
	if subject := collapseSpace(stripControl(it.Subject.Join(" "))); subject != "" {
		tags += " " + subject
	}
	return Game{
		Identifier: id,
		Title:      title,
		Year:       year,
		Creator:    truncateRunes(collapseSpace(stripControl(it.Creator.Join(", "))), maxCreatorRunes),
		Platform:   src.Key,
		System:     SystemFor(src.Key, emulator),
		Emulator:   truncateRunes(emulator, maxEmulatorRunes),
		Downloads:  parseCount(it.Downloads.First()),
		Tags:       truncateRunes(tags, maxTagsRunes),
	}, true
}
