package flows

import (
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"regexp/syntax"
	"sort"
	"strings"
	"time"
)

// ConditionGroup combines condition rows with "all" (AND, default) or "any" (OR).
type ConditionGroup struct {
	Match string         `json:"match"`
	Rows  []ConditionRow `json:"rows"`
}

// ConditionRow compares Left and Right with Op. Type is auto, text, number, date or bool.
type ConditionRow struct {
	Left  any    `json:"left"`
	Op    string `json:"op"`
	Right any    `json:"right"`
	Type  string `json:"type,omitempty"`
}

const (
	// maxPatternLength caps the source size of a "matches" pattern in bytes.
	maxPatternLength = 200
	// maxPatternInsts caps the compiled program of a "matches" pattern. The source
	// cap alone does not bound it: "a{1000}" repeated 25 times fits into 176 bytes
	// but compiles to about 25,000 instructions. Ordinary patterns use a few
	// hundred at most.
	maxPatternInsts = 1000
)

var conditionOps = map[string]bool{
	"eq": true, "ne": true, "contains": true, "not_contains": true, "starts_with": true,
	"ends_with": true, "matches": true, "gt": true, "gte": true, "lt": true, "lte": true,
	"empty": true, "not_empty": true, "is_true": true, "is_false": true, "before": true, "after": true,
}

var conditionTypes = map[string]bool{"": true, "auto": true, "text": true, "number": true, "date": true, "bool": true}

// ValidOperator reports whether op is a supported condition operator.
func ValidOperator(op string) bool { return conditionOps[op] }

// ConditionOperators returns the sorted operator names.
func ConditionOperators() []string {
	ops := make([]string, 0, len(conditionOps))
	for op := range conditionOps {
		ops = append(ops, op)
	}
	sort.Strings(ops)
	return ops
}

// DecodeConditionGroup converts a (resolved or raw) parameter value into a group
// and checks match, operators and types. The value is copied through JSON, so the
// returned group never aliases v. Numbers become float64, so integers above 2^53
// lose precision; compare long numeric IDs with type "text".
func DecodeConditionGroup(v any) (ConditionGroup, error) {
	var g ConditionGroup
	if v == nil {
		return g, errors.New("condition is missing")
	}
	data, err := json.Marshal(v)
	if err != nil {
		return g, err
	}
	if string(data) == "null" {
		// A typed nil map or slice marshals to null and would otherwise decode
		// into an empty "all" group, which is true.
		return g, errors.New("condition is missing")
	}
	if err := json.Unmarshal(data, &g); err != nil {
		// Fixed text: the json error names Go types, which must not reach the editor.
		return g, errors.New("condition must be an object with rows")
	}
	if g.Match != "" && g.Match != "all" && g.Match != "any" {
		return g, fmt.Errorf("unknown match %s", quoteForError(g.Match))
	}
	for i, r := range g.Rows {
		if !ValidOperator(r.Op) {
			return g, fmt.Errorf("row %d: unknown operator %s", i+1, quoteForError(r.Op))
		}
		if !conditionTypes[r.Type] {
			return g, fmt.Errorf("row %d: unknown type %s", i+1, quoteForError(r.Type))
		}
	}
	return g, nil
}

// Evaluate returns the group's result. An empty "all" group is true, an empty "any" group false.
func (g ConditionGroup) Evaluate(loc *time.Location) (bool, error) {
	matchAny := g.Match == "any"
	for _, row := range g.Rows {
		ok, err := row.Evaluate(loc)
		if err != nil {
			return false, err
		}
		if matchAny && ok {
			return true, nil
		}
		if !matchAny && !ok {
			return false, nil
		}
	}
	return !matchAny, nil
}

// Evaluate compares one row. contains/starts_with/ends_with ignore case; eq/ne do not.
// contains on an object checks for an exact, case-sensitive key. With type auto (or
// none) numeric strings compare as numbers, so long numeric IDs should use type
// "text". The cost of "matches" is bounded by len(input) times maxPatternInsts.
// Evaluate only reads Left and Right, which may be shared with the flow's runtime
// data.
func (r ConditionRow) Evaluate(loc *time.Location) (bool, error) {
	switch r.Op {
	case "empty":
		return isEmptyValue(r.Left), nil
	case "not_empty":
		return !isEmptyValue(r.Left), nil
	case "is_true":
		return truthy(r.Left), nil
	case "is_false":
		return !truthy(r.Left), nil
	case "contains", "not_contains":
		return containsValue(r.Left, r.Right) == (r.Op == "contains"), nil
	case "starts_with":
		return strings.HasPrefix(lowerText(r.Left), lowerText(r.Right)), nil
	case "ends_with":
		return strings.HasSuffix(lowerText(r.Left), lowerText(r.Right)), nil
	case "matches":
		re, err := compilePattern(Stringify(r.Right))
		if err != nil {
			return false, err
		}
		return re.MatchString(Stringify(r.Left)), nil
	case "before", "after":
		left, okL := toTime(r.Left, loc)
		right, okR := toTime(r.Right, loc)
		if !okL || !okR {
			return false, fmt.Errorf("%s needs two dates", r.Op)
		}
		if r.Op == "before" {
			return left.Before(right), nil
		}
		return left.After(right), nil
	case "eq", "ne", "gt", "gte", "lt", "lte":
		c, err := compareValues(r.Left, r.Right, r.Type, loc)
		if err != nil {
			return false, err
		}
		switch r.Op {
		case "eq":
			return c == 0, nil
		case "ne":
			return c != 0, nil
		case "gt":
			return c > 0, nil
		case "gte":
			return c >= 0, nil
		case "lt":
			return c < 0, nil
		default:
			return c <= 0, nil
		}
	}
	return false, fmt.Errorf("unknown operator %s", quoteForError(r.Op))
}

// compilePattern compiles a "matches" pattern after bounding its size. Go's regexp
// cannot be cancelled, so the pattern is parsed and compiled once up front to
// count instructions; matching then costs at most len(input) times
// maxPatternInsts steps.
func compilePattern(pattern string) (*regexp.Regexp, error) {
	if len(pattern) > maxPatternLength {
		return nil, fmt.Errorf("the pattern is longer than %d bytes", maxPatternLength)
	}
	parsed, err := syntax.Parse(pattern, syntax.Perl)
	if err != nil {
		return nil, patternError(err)
	}
	prog, err := syntax.Compile(parsed.Simplify())
	if err != nil {
		return nil, patternError(err)
	}
	if len(prog.Inst) > maxPatternInsts {
		return nil, errors.New("the pattern is too complex (large repeat counts such as {1,1000} are not supported)")
	}
	re, err := regexp.Compile(pattern)
	if err != nil {
		return nil, patternError(err)
	}
	return re, nil
}

// patternError describes a regexp compile failure without echoing the whole
// pattern: the stock error text embeds the offending expression, which comes from
// user-controlled data.
func patternError(err error) error {
	var se *syntax.Error
	if errors.As(err, &se) {
		return fmt.Errorf("invalid pattern: %s near %s", se.Code, quoteForError(se.Expr))
	}
	return errors.New("invalid pattern")
}

func compareValues(a, b any, typ string, loc *time.Location) (int, error) {
	switch typ {
	case "number":
		x, ok1 := toNumber(a)
		y, ok2 := toNumber(b)
		if !ok1 || !ok2 {
			return 0, errors.New("both sides must be numbers")
		}
		return cmpFloat(x, y), nil
	case "date":
		x, ok1 := toTime(a, loc)
		y, ok2 := toTime(b, loc)
		if !ok1 || !ok2 {
			return 0, errors.New("both sides must be dates")
		}
		return x.Compare(y), nil
	case "bool":
		return cmpBool(truthy(a), truthy(b)), nil
	case "text":
		return strings.Compare(Stringify(a), Stringify(b)), nil
	case "", "auto":
		if x, ok := toNumber(a); ok {
			if y, ok := toNumber(b); ok {
				return cmpFloat(x, y), nil
			}
		}
		if x, ok := a.(bool); ok {
			if y, ok := b.(bool); ok {
				return cmpBool(x, y), nil
			}
		}
		if x, ok := toTimeStrict(a); ok {
			if y, ok := toTimeStrict(b); ok {
				return x.Compare(y), nil
			}
		}
		return strings.Compare(Stringify(a), Stringify(b)), nil
	}
	return 0, fmt.Errorf("unknown comparison type %s", quoteForError(typ))
}

func cmpFloat(x, y float64) int {
	switch {
	case x < y:
		return -1
	case x > y:
		return 1
	}
	return 0
}

func cmpBool(x, y bool) int {
	switch {
	case x == y:
		return 0
	case !x:
		return -1
	}
	return 1
}

func lowerText(v any) string { return strings.ToLower(Stringify(v)) }

func containsValue(left, right any) bool {
	needle := lowerText(right)
	switch x := left.(type) {
	case []any:
		for _, item := range x {
			if lowerText(item) == needle {
				return true
			}
		}
		return false
	case map[string]any:
		_, ok := x[Stringify(right)]
		return ok
	}
	return strings.Contains(lowerText(left), needle)
}
