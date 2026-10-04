package flows

import (
	"errors"
	"fmt"
	"html"
	"math"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf8"
)

const (
	// maxFilterOutputBytes caps the text a single replace call may produce.
	maxFilterOutputBytes = 8 << 20
	// maxSplitParts caps the number of elements a single split call may produce.
	maxSplitParts = 100000
	// maxErrorEchoRunes caps how much of a value an error message repeats.
	maxErrorEchoRunes = 40
)

// FilterCall is one "| name(args)" step of a template expression.
type FilterCall struct {
	Name string
	Args []any
}

type filterSpec struct {
	minArgs int
	maxArgs int
	fn      func(in any, args []any, env *Env) (any, error)
}

var filters = map[string]filterSpec{
	"default":    {1, 1, filterDefault},
	"truncate":   {1, 1, filterTruncate},
	"upper":      {0, 0, func(in any, _ []any, _ *Env) (any, error) { return strings.ToUpper(Stringify(in)), nil }},
	"lower":      {0, 0, func(in any, _ []any, _ *Env) (any, error) { return strings.ToLower(Stringify(in)), nil }},
	"trim":       {0, 0, func(in any, _ []any, _ *Env) (any, error) { return strings.TrimSpace(Stringify(in)), nil }},
	"join":       {0, 1, filterJoin},
	"split":      {1, 1, filterSplit},
	"first":      {0, 0, filterFirst},
	"last":       {0, 0, filterLast},
	"count":      {0, 0, filterCount},
	"pluck":      {1, 1, filterPluck},
	"json":       {0, 0, filterJSON},
	"date":       {1, 2, filterDate},
	"round":      {0, 1, filterRound},
	"replace":    {2, 2, filterReplace},
	"strip_html": {0, 0, filterStripHTML},
}

// FilterNames returns the sorted list of supported filter names.
func FilterNames() []string {
	names := make([]string, 0, len(filters))
	for name := range filters {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func checkFilterCall(c FilterCall) error {
	spec, ok := filters[c.Name]
	if !ok {
		return fmt.Errorf("unknown filter %s", quoteForError(c.Name))
	}
	if len(c.Args) < spec.minArgs || len(c.Args) > spec.maxArgs {
		if spec.minArgs == spec.maxArgs {
			return fmt.Errorf("filter %s takes %d argument(s)", c.Name, spec.minArgs)
		}
		return fmt.Errorf("filter %s takes %d to %d arguments", c.Name, spec.minArgs, spec.maxArgs)
	}
	return nil
}

// applyFilter checks and runs one filter. Template evaluation and tests use it.
func applyFilter(name string, in any, args []any, env *Env) (any, error) {
	if err := checkFilterCall(FilterCall{Name: name, Args: args}); err != nil {
		return nil, err
	}
	return filters[name].fn(in, args, env)
}

// truncateForError cuts s to maxErrorEchoRunes runes plus an ellipsis so a huge
// input cannot flood logs or run records. It does not quote or escape.
func truncateForError(s string) string {
	n := 0
	for i := range s {
		if n == maxErrorEchoRunes {
			return s[:i] + "…"
		}
		n++
	}
	return s
}

// quoteForError renders s as a quoted string for an error message, cut like
// truncateForError.
func quoteForError(s string) string {
	return strconv.Quote(truncateForError(s))
}

func intArg(args []any, i, def, lower int) (int, error) {
	if i >= len(args) {
		return def, nil
	}
	f, ok := args[i].(float64)
	if !ok || f != math.Trunc(f) {
		return 0, errors.New("argument must be a whole number")
	}
	if f > math.MaxInt32 || f < -math.MaxInt32 {
		return 0, errors.New("argument too large")
	}
	if int(f) < lower {
		return 0, fmt.Errorf("argument must be at least %d", lower)
	}
	return int(f), nil
}

func stringArg(args []any, i int, def string) (string, error) {
	if i >= len(args) {
		return def, nil
	}
	s, ok := args[i].(string)
	if !ok {
		return "", errors.New("argument must be a string")
	}
	return s, nil
}

func filterDefault(in any, args []any, _ *Env) (any, error) {
	if in == nil {
		return args[0], nil
	}
	if s, ok := in.(string); ok && s == "" {
		return args[0], nil
	}
	return in, nil
}

func filterTruncate(in any, args []any, _ *Env) (any, error) {
	n, err := intArg(args, 0, 0, 1)
	if err != nil {
		return nil, err
	}
	s := Stringify(in)
	if utf8.RuneCountInString(s) <= n {
		return s, nil
	}
	runes := []rune(s)
	return string(runes[:n]) + "…", nil
}

func filterJoin(in any, args []any, _ *Env) (any, error) {
	sep, err := stringArg(args, 0, ", ")
	if err != nil {
		return nil, err
	}
	list, ok := in.([]any)
	if !ok {
		return Stringify(in), nil
	}
	// The separators alone can exceed the cap (200 items, 1 MiB separator), and
	// a single expression is not covered by the cap on assembled template text,
	// so check before building: separators first, then the parts as they are
	// rendered, which stops at the first item that crosses the limit.
	var total int64
	if len(list) > 1 {
		total = int64(len(list)-1) * int64(len(sep))
	}
	if total > maxFilterOutputBytes {
		return nil, fmt.Errorf("join result would exceed %d bytes", maxFilterOutputBytes)
	}
	parts := make([]string, len(list))
	for i, item := range list {
		parts[i] = Stringify(item)
		total += int64(len(parts[i]))
		if total > maxFilterOutputBytes {
			return nil, fmt.Errorf("join result would exceed %d bytes", maxFilterOutputBytes)
		}
	}
	return strings.Join(parts, sep), nil
}

func filterSplit(in any, args []any, _ *Env) (any, error) {
	sep, err := stringArg(args, 0, ",")
	if err != nil {
		return nil, err
	}
	// SplitN with a bound keeps the allocation small even for hostile input.
	parts := strings.SplitN(Stringify(in), sep, maxSplitParts+1)
	if len(parts) > maxSplitParts {
		return nil, fmt.Errorf("split would produce more than %d parts", maxSplitParts)
	}
	out := make([]any, len(parts))
	for i, p := range parts {
		out[i] = p
	}
	return out, nil
}

func filterFirst(in any, _ []any, _ *Env) (any, error) {
	switch x := in.(type) {
	case []any:
		if len(x) == 0 {
			return nil, nil
		}
		return x[0], nil
	case string:
		r, size := utf8.DecodeRuneInString(x)
		if size == 0 {
			return "", nil
		}
		return string(r), nil
	}
	return nil, nil
}

func filterLast(in any, _ []any, _ *Env) (any, error) {
	switch x := in.(type) {
	case []any:
		if len(x) == 0 {
			return nil, nil
		}
		return x[len(x)-1], nil
	case string:
		r, size := utf8.DecodeLastRuneInString(x)
		if size == 0 {
			return "", nil
		}
		return string(r), nil
	}
	return nil, nil
}

func filterCount(in any, _ []any, _ *Env) (any, error) {
	switch x := in.(type) {
	case nil:
		return 0.0, nil
	case []any:
		return float64(len(x)), nil
	case map[string]any:
		return float64(len(x)), nil
	case string:
		return float64(utf8.RuneCountInString(x)), nil
	}
	return 1.0, nil
}

func filterPluck(in any, args []any, _ *Env) (any, error) {
	field, err := stringArg(args, 0, "")
	if err != nil {
		return nil, err
	}
	list, ok := in.([]any)
	if !ok {
		return []any{}, nil
	}
	out := make([]any, len(list))
	for i, item := range list {
		if m, ok := item.(map[string]any); ok {
			out[i] = m[field]
		}
	}
	return out, nil
}

// filterJSON serialises the value as compact JSON. Unlike Stringify it fails on
// values JSON cannot represent instead of emitting "<unserializable>".
func filterJSON(in any, _ []any, _ *Env) (any, error) {
	s, err := marshalCompact(in)
	if err != nil {
		return nil, fmt.Errorf("value cannot be serialized as JSON: %w", err)
	}
	return s, nil
}

type zoneLookup struct {
	loc *time.Location
	err error
}

// zoneCache remembers time.LoadLocation results, failures included, by zone
// name so a template evaluated in a loop does not hit the zoneinfo files again.
var zoneCache sync.Map

func loadZone(name string) (*time.Location, error) {
	if cached, ok := zoneCache.Load(name); ok {
		res := cached.(zoneLookup)
		return res.loc, res.err
	}
	loc, err := time.LoadLocation(name)
	zoneCache.Store(name, zoneLookup{loc: loc, err: err})
	return loc, err
}

// filterDate formats a date with the tokens of formatDatePattern. The input is
// read like everywhere else: values and strings that carry a zone keep it, and
// zone-less strings are interpreted in the Env's zone (the flow/server zone).
// The optional second argument names the IANA zone the result is shown in; an
// empty name counts as not given, which shows the result in the Env's zone.
func filterDate(in any, args []any, env *Env) (any, error) {
	pattern, err := stringArg(args, 0, "")
	if err != nil {
		return nil, err
	}
	base := env.loc()
	target := base
	if len(args) > 1 {
		name, err := stringArg(args, 1, "")
		if err != nil {
			return nil, err
		}
		if name != "" {
			loaded, err := loadZone(name)
			if err != nil {
				return nil, fmt.Errorf("unknown time zone %s", quoteForError(name))
			}
			target = loaded
		}
	}
	t, ok := toTime(in, base)
	if !ok {
		return nil, fmt.Errorf("%s is not a date", quoteForError(Stringify(in)))
	}
	return formatDatePattern(t.In(target), pattern), nil
}

// formatDatePattern formats with the tokens YYYY MM DD HH mm ss; everything else is literal.
func formatDatePattern(t time.Time, pattern string) string {
	var b strings.Builder
	for i := 0; i < len(pattern); {
		rest := pattern[i:]
		switch {
		case strings.HasPrefix(rest, "YYYY"):
			fmt.Fprintf(&b, "%04d", t.Year())
			i += 4
		case strings.HasPrefix(rest, "MM"):
			fmt.Fprintf(&b, "%02d", int(t.Month()))
			i += 2
		case strings.HasPrefix(rest, "DD"):
			fmt.Fprintf(&b, "%02d", t.Day())
			i += 2
		case strings.HasPrefix(rest, "HH"):
			fmt.Fprintf(&b, "%02d", t.Hour())
			i += 2
		case strings.HasPrefix(rest, "mm"):
			fmt.Fprintf(&b, "%02d", t.Minute())
			i += 2
		case strings.HasPrefix(rest, "ss"):
			fmt.Fprintf(&b, "%02d", t.Second())
			i += 2
		default:
			b.WriteByte(pattern[i])
			i++
		}
	}
	return b.String()
}

func filterRound(in any, args []any, _ *Env) (any, error) {
	decimals, err := intArg(args, 0, 0, 0)
	if err != nil {
		return nil, err
	}
	if decimals > 10 {
		return nil, errors.New("at most 10 decimals")
	}
	f, ok := toNumber(in)
	if !ok {
		return nil, fmt.Errorf("%s is not a number", quoteForError(Stringify(in)))
	}
	// math.Round rounds half away from zero; the editor's JS mirror copies that.
	scale := math.Pow(10, float64(decimals))
	r := math.Round(f*scale) / scale
	if math.IsInf(r, 0) || math.IsNaN(r) {
		// f*scale overflowed: f is too large to have any decimals left to round.
		return f, nil
	}
	if r == 0 {
		r = 0 // normalise -0, which would print as "-0" and compare oddly
	}
	return r, nil
}

func filterReplace(in any, args []any, _ *Env) (any, error) {
	oldText, err := stringArg(args, 0, "")
	if err != nil {
		return nil, err
	}
	newText, err := stringArg(args, 1, "")
	if err != nil {
		return nil, err
	}
	s := Stringify(in)
	if oldText == "" {
		return s, nil
	}
	if n := strings.Count(s, oldText); n > 0 {
		size := int64(len(s)) + int64(n)*(int64(len(newText))-int64(len(oldText)))
		if size > maxFilterOutputBytes {
			return nil, fmt.Errorf("replace result would exceed %d bytes", maxFilterOutputBytes)
		}
	}
	return strings.ReplaceAll(s, oldText, newText), nil
}

// The strip_html patterns are separate RE2 expressions (no backreferences, so
// matching stays linear). The tag pattern needs a letter after "<" and a plain
// tag name, so comparisons ("x < 5") and addresses ("<max@example.com>") are
// not mistaken for markup.
var (
	htmlScriptPattern  = regexp.MustCompile(`(?is)<script\b[^>]*>.*?</script\s*>`)
	htmlStylePattern   = regexp.MustCompile(`(?is)<style\b[^>]*>.*?</style\s*>`)
	htmlCommentPattern = regexp.MustCompile(`(?s)<!--.*?-->`)
	// Doctype and CDATA openers ("<!DOCTYPE html>", "<![CDATA[...]]>") and
	// processing instructions ("<?xml ...?>").
	htmlDeclPattern = regexp.MustCompile(`(?i)<![A-Za-z\[][^>]*>|<\?[^>]*\?>`)
	htmlTagPattern  = regexp.MustCompile(`</?[A-Za-z][A-Za-z0-9:_-]*(?:\s[^>]*)?/?>`)
)

// filterStripHTML removes markup and returns plain text: script and style
// blocks with their content, comments, doctype/CDATA/processing-instruction
// declarations and tags go, entities are unescaped and whitespace is collapsed.
// The result is plain text, NOT HTML-safe (an input like "&lt;b&gt;" comes out
// as "<b>"); consumers that put it into HTML must escape it. It is a text
// extractor, not a sanitizer, with known limits: a ">" inside a quoted
// attribute value can leave fragments behind, letter-adjacent text such as
// "a<b and c>d" looks like a tag and is removed, and an unclosed script or
// style element keeps its body.
func filterStripHTML(in any, _ []any, _ *Env) (any, error) {
	text := Stringify(in)
	text = htmlScriptPattern.ReplaceAllString(text, " ")
	text = htmlStylePattern.ReplaceAllString(text, " ")
	text = htmlCommentPattern.ReplaceAllString(text, " ")
	text = htmlDeclPattern.ReplaceAllString(text, " ")
	text = htmlTagPattern.ReplaceAllString(text, " ")
	return strings.Join(strings.Fields(html.UnescapeString(text)), " "), nil
}
