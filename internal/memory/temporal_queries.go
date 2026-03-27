package memory

import (
	"regexp"
	"strings"
	"time"
)

type TemporalQueryRange struct {
	FromDate string `json:"from_date"`
	ToDate   string `json:"to_date"`
	Label    string `json:"label"`
}

var (
	reLastNDaysDE   = regexp.MustCompile(`(?i)\bin den letzten\s+(\d+)\s+tagen\b`)
	reLastNDaysEN   = regexp.MustCompile(`(?i)\bin the last\s+(\d+)\s+days\b`)
	reBeforeNDaysDE = regexp.MustCompile(`(?i)\bvor\s+(\d+)\s+tagen\b`)
	reBeforeNDaysEN = regexp.MustCompile(`(?i)\b(\d+)\s+days?\s+ago\b`)
)

// ParseTemporalQuery extracts a supported natural-language time range from a query.
// It returns the normalized date range, the cleaned topic query, and whether
// a temporal expression was recognized.
func ParseTemporalQuery(query string) (TemporalQueryRange, string, bool) {
	return ParseTemporalQueryAt(query, time.Now())
}

// ParseTemporalQueryAt is the testable variant of ParseTemporalQuery.
func ParseTemporalQueryAt(query string, now time.Time) (TemporalQueryRange, string, bool) {
	normalized := strings.TrimSpace(query)
	if normalized == "" {
		return TemporalQueryRange{}, "", false
	}

	lower := strings.ToLower(normalized)
	if from, to, label, matched := parseFixedTemporalQuery(lower, now); matched {
		return TemporalQueryRange{FromDate: from, ToDate: to, Label: label}, cleanupTemporalTopic(removeTemporalPhrase(normalized, label)), true
	}

	if from, to, label, stripped, matched := parseRegexTemporalQuery(normalized, now); matched {
		return TemporalQueryRange{FromDate: from, ToDate: to, Label: label}, cleanupTemporalTopic(stripped), true
	}

	return TemporalQueryRange{}, normalized, false
}

func parseFixedTemporalQuery(lower string, now time.Time) (string, string, string, bool) {
	type fixedPattern struct {
		phrases []string
		label   string
		from    func(time.Time) time.Time
		to      func(time.Time) time.Time
	}

	patterns := []fixedPattern{
		{phrases: []string{"vorgestern", "day before yesterday"}, label: "vorgestern", from: func(t time.Time) time.Time { return startOfDay(t.AddDate(0, 0, -2)) }, to: func(t time.Time) time.Time { return startOfDay(t.AddDate(0, 0, -2)) }},
		{phrases: []string{"gestern", "yesterday"}, label: "gestern", from: func(t time.Time) time.Time { return startOfDay(t.AddDate(0, 0, -1)) }, to: func(t time.Time) time.Time { return startOfDay(t.AddDate(0, 0, -1)) }},
		{phrases: []string{"heute", "today"}, label: "heute", from: func(t time.Time) time.Time { return startOfDay(t) }, to: func(t time.Time) time.Time { return startOfDay(t) }},
		{phrases: []string{"diese woche", "this week"}, label: "diese woche", from: func(t time.Time) time.Time { return startOfWeek(t) }, to: func(t time.Time) time.Time { return startOfDay(t) }},
		{phrases: []string{"letzte woche", "last week"}, label: "letzte woche", from: func(t time.Time) time.Time { return startOfWeek(t).AddDate(0, 0, -7) }, to: func(t time.Time) time.Time { return startOfWeek(t).AddDate(0, 0, -1) }},
		{phrases: []string{"diesen monat", "this month"}, label: "diesen monat", from: func(t time.Time) time.Time { return startOfMonth(t) }, to: func(t time.Time) time.Time { return startOfDay(t) }},
		{phrases: []string{"letzten monat", "last month"}, label: "letzten monat", from: func(t time.Time) time.Time { return startOfMonth(t).AddDate(0, -1, 0) }, to: func(t time.Time) time.Time { return startOfMonth(t).AddDate(0, 0, -1) }},
	}

	for _, pattern := range patterns {
		for _, phrase := range pattern.phrases {
			if strings.Contains(lower, phrase) {
				return pattern.from(now).Format("2006-01-02"), pattern.to(now).Format("2006-01-02"), phrase, true
			}
		}
	}

	return "", "", "", false
}

func parseRegexTemporalQuery(query string, now time.Time) (string, string, string, string, bool) {
	type regexPattern struct {
		re    *regexp.Regexp
		label string
		mode  string
	}

	patterns := []regexPattern{
		{re: reLastNDaysDE, label: "in den letzten X tagen", mode: "range"},
		{re: reLastNDaysEN, label: "in the last X days", mode: "range"},
		{re: reBeforeNDaysDE, label: "vor X tagen", mode: "point"},
		{re: reBeforeNDaysEN, label: "X days ago", mode: "point"},
	}

	for _, pattern := range patterns {
		match := pattern.re.FindStringSubmatch(query)
		if len(match) != 2 {
			continue
		}
		n, ok := parsePositiveInt(match[1])
		if !ok || n <= 0 {
			continue
		}

		var from, to time.Time
		switch pattern.mode {
		case "range":
			from = startOfDay(now.AddDate(0, 0, -(n - 1)))
			to = startOfDay(now)
		default:
			from = startOfDay(now.AddDate(0, 0, -n))
			to = from
		}
		stripped := pattern.re.ReplaceAllString(query, "")
		return from.Format("2006-01-02"), to.Format("2006-01-02"), pattern.label, stripped, true
	}

	return "", "", "", "", false
}

func cleanupTemporalTopic(query string) string {
	cleaned := strings.TrimSpace(query)
	replacements := []string{
		"  ", " ",
		"über ", "",
		"about ", "",
		"vom ", "",
		"from ", "",
	}
	for i := 0; i < len(replacements); i += 2 {
		cleaned = strings.ReplaceAll(cleaned, replacements[i], replacements[i+1])
	}
	cleaned = strings.Trim(cleaned, " ,.-")
	return cleaned
}

func removeTemporalPhrase(query, phrase string) string {
	if phrase == "" {
		return query
	}
	replacer := regexp.MustCompile("(?i)" + regexp.QuoteMeta(phrase))
	return replacer.ReplaceAllString(query, "")
}

func startOfDay(t time.Time) time.Time {
	y, m, d := t.Date()
	return time.Date(y, m, d, 0, 0, 0, 0, t.Location())
}

func startOfWeek(t time.Time) time.Time {
	day := int(t.Weekday())
	if day == 0 {
		day = 7
	}
	return startOfDay(t).AddDate(0, 0, -(day - 1))
}

func startOfMonth(t time.Time) time.Time {
	y, m, _ := t.Date()
	return time.Date(y, m, 1, 0, 0, 0, 0, t.Location())
}

func parsePositiveInt(s string) (int, bool) {
	n := 0
	for _, r := range strings.TrimSpace(s) {
		if r < '0' || r > '9' {
			return 0, false
		}
		n = n*10 + int(r-'0')
	}
	return n, true
}
