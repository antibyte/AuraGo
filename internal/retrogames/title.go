package retrogames

import (
	"regexp"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

var titlePrefixes = []string{"Internet Arcade: ", "Atari 2600: ", "Commodore 64: ", "MS-DOS: "}

var (
	tosecGroupPattern      = regexp.MustCompile(`\((?:19|20)[0-9x]{2}[^)]*\)`)
	yearPattern            = regexp.MustCompile(`(?:^|[^0-9])(19[5-9][0-9]|20[0-9][0-9])(?:[^0-9]|$)`)
	spacePattern           = regexp.MustCompile(`\s+`)
	trailingArticlePattern = regexp.MustCompile(`^(.+), (The|A|An)$`)
)

// CleanTitle turns a raw archive.org title into a display title. It also
// returns a year found in TOSEC-style metadata ("(1991)(Publisher)"), or 0.
func CleanTitle(raw string) (string, int) {
	base := collapseSpace(stripControl(raw))
	title := base
	for _, prefix := range titlePrefixes {
		if strings.HasPrefix(title, prefix) {
			title = strings.TrimSpace(title[len(prefix):])
			break
		}
	}
	year := 0
	if loc := tosecGroupPattern.FindStringIndex(title); loc != nil && loc[0] > 0 {
		year = ParseYear(title[loc[0]:loc[1]])
		title = title[:loc[0]]
	}
	if i := strings.Index(title, "["); i > 0 {
		title = title[:i]
	}
	title = strings.TrimSpace(title)
	title = strings.TrimSpace(strings.TrimSuffix(title, "(Clean)"))
	title = collapseSpace(title)
	if m := trailingArticlePattern.FindStringSubmatch(title); m != nil {
		title = m[2] + " " + m[1]
	}
	if title == "" {
		title = base
	}
	return truncateRunes(title, maxTitleRunes), year
}

// ParseYear returns the first plausible release year (1950–2030) in s, or 0.
func ParseYear(s string) int {
	m := yearPattern.FindStringSubmatch(s)
	if m == nil {
		return 0
	}
	year, err := strconv.Atoi(m[1])
	if err != nil || year > 2030 {
		return 0
	}
	return year
}

func stripControl(s string) string {
	s = strings.ToValidUTF8(s, "")
	return strings.Map(func(r rune) rune {
		if unicode.IsControl(r) || r == ' ' || r == ' ' {
			return ' '
		}
		return r
	}, s)
}

func collapseSpace(s string) string {
	return strings.TrimSpace(spacePattern.ReplaceAllString(s, " "))
}

func truncateRunes(s string, limit int) string {
	if utf8.RuneCountInString(s) <= limit {
		return s
	}
	runes := []rune(s)
	return strings.TrimSpace(string(runes[:limit]))
}
