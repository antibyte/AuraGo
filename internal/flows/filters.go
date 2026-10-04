package flows

import (
	"errors"
	"fmt"
	"html"
	"math"
	"regexp"
	"sort"
	"strings"
	"time"
	"unicode/utf8"
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
		return fmt.Errorf("unknown filter %q", c.Name)
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

func intArg(args []any, i, def, min int) (int, error) {
	if i >= len(args) {
		return def, nil
	}
	f, ok := args[i].(float64)
	if !ok || f != math.Trunc(f) {
		return 0, errors.New("argument must be a whole number")
	}
	if int(f) < min {
		return 0, fmt.Errorf("argument must be at least %d", min)
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
	parts := make([]string, len(list))
	for i, item := range list {
		parts[i] = Stringify(item)
	}
	return strings.Join(parts, sep), nil
}

func filterSplit(in any, args []any, _ *Env) (any, error) {
	sep, err := stringArg(args, 0, ",")
	if err != nil {
		return nil, err
	}
	parts := strings.Split(Stringify(in), sep)
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
		for _, r := range x {
			return string(r), nil
		}
		return "", nil
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
		r, _ := utf8.DecodeLastRuneInString(x)
		if r == utf8.RuneError {
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

func filterDate(in any, args []any, env *Env) (any, error) {
	pattern, err := stringArg(args, 0, "")
	if err != nil {
		return nil, err
	}
	loc := env.loc()
	if len(args) > 1 {
		name, err := stringArg(args, 1, "")
		if err != nil {
			return nil, err
		}
		loaded, err := time.LoadLocation(name)
		if err != nil {
			return nil, fmt.Errorf("unknown time zone %q", name)
		}
		loc = loaded
	}
	t, ok := toTime(in, loc)
	if !ok {
		return nil, fmt.Errorf("%q is not a date", Stringify(in))
	}
	return formatDatePattern(t.In(loc), pattern), nil
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
		return nil, fmt.Errorf("%q is not a number", Stringify(in))
	}
	scale := math.Pow(10, float64(decimals))
	return math.Round(f*scale) / scale, nil
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
	if oldText == "" {
		return Stringify(in), nil
	}
	return strings.ReplaceAll(Stringify(in), oldText, newText), nil
}

var htmlTagPattern = regexp.MustCompile(`<[^>]*>`)

func filterStripHTML(in any, _ []any, _ *Env) (any, error) {
	text := htmlTagPattern.ReplaceAllString(Stringify(in), " ")
	return strings.Join(strings.Fields(html.UnescapeString(text)), " "), nil
}
