package flows

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

const (
	// maxTemplateOutputBytes caps the text a mixed-text template may produce: a
	// template that repeats a large value many times must fail, not exhaust memory.
	maxTemplateOutputBytes = 8 << 20
	// maxParamDepth caps how deeply maps and lists inside node parameters may
	// nest. Counting starts at 1 for the parameter map itself (or for the value
	// handed to ResolveValue), so a container at level 33 is rejected.
	maxParamDepth = 32
	// maxTemplateRefs caps the refs plus problems one CollectTemplateRefs call
	// returns, which also bounds the walk over parameter structures built in Go
	// that share sub-maps (two keys pointing at the same child, nested).
	maxTemplateRefs = 1000
)

var (
	errTemplateOutputTooLarge = fmt.Errorf("template output exceeds %d MiB", maxTemplateOutputBytes>>20)
	errParamsTooDeep          = fmt.Errorf("parameters nested deeper than %d levels", maxParamDepth)
	errTooManyTemplateRefs    = fmt.Errorf("more than %d template expressions in the parameters", maxTemplateRefs)
)

// Evaluate resolves the template against env. A single-expression template
// returns the expression's value with its JSON type; otherwise all parts are
// concatenated as text, which fails when the assembled text (literals plus
// expressions) would exceed 8 MiB. The cap is for assembled text only: a single
// expression returns its value as-is, bounded just by what its filters bound
// (replace, join and split are capped). A nil template yields nil.
//
// Aliasing: a single-expression template returns the value itself, not a copy,
// so a list or object it returns is shared with env (and with whatever data
// env.Roots was built from). Treat it as read-only and clone it before
// modifying it, append included, which can write into a shared backing array.
// Text from env values or from evaluation is never evaluated as a template
// again.
func (t *Template) Evaluate(env *Env) (any, error) {
	if t == nil {
		return nil, nil
	}
	if t.IsSingleExpr() {
		return evalExpr(t.parts[0].expr, env)
	}
	var b strings.Builder
	for _, part := range t.parts {
		s := part.literal
		if part.expr != nil {
			v, err := evalExpr(part.expr, env)
			if err != nil {
				return nil, err
			}
			s = Stringify(v)
		}
		if b.Len()+len(s) > maxTemplateOutputBytes {
			return nil, errTemplateOutputTooLarge
		}
		b.WriteString(s)
	}
	return b.String(), nil
}

func evalExpr(e *Expr, env *Env) (any, error) {
	var v any
	if env != nil {
		v = env.Roots[e.Root]
	}
	for _, seg := range e.Path {
		v = stepInto(v, seg)
	}
	for _, f := range e.Filters {
		// applyFilter re-checks name and argument count, so an Expr that was not
		// built by ParseTemplate fails with an error instead of a nil-func panic.
		out, err := applyFilter(f.Name, v, f.Args, env)
		if err != nil {
			// Source is not cut when stored, so bound it before it reaches an
			// error message or a run record.
			return nil, fmt.Errorf("expression %s: filter %s: %w", quoteForError(e.Source), quoteForError(f.Name), err)
		}
		v = out
	}
	return v, nil
}

func stepInto(v any, seg PathSeg) any {
	if seg.IsIndex {
		list, ok := v.([]any)
		if !ok {
			return nil
		}
		i := seg.Index
		if i < 0 {
			i += len(list)
		}
		if i < 0 || i >= len(list) {
			return nil
		}
		return list[i]
	}
	m, ok := v.(map[string]any)
	if !ok {
		return nil
	}
	return m[seg.Field]
}

// ResolveValue resolves templates inside strings and recurses into maps and
// lists, up to 32 levels of nesting. Other values are returned unchanged. The
// input is never modified, and when several entries fail, the error names the
// first one in key order.
//
// Aliasing: maps and lists of the input are rebuilt, but a string that is one
// single expression resolves to the value from env itself, so a list or object
// obtained that way is shared with env. Treat the result as read-only and clone
// before modifying it, append included. Resolved values are never evaluated as
// templates again.
func ResolveValue(v any, env *Env) (any, error) {
	return resolveValue(v, env, 1)
}

// resolveValue resolves v, which sits at nesting level depth.
func resolveValue(v any, env *Env, depth int) (any, error) {
	switch x := v.(type) {
	case string:
		if !strings.Contains(x, "{{") {
			return x, nil
		}
		tpl, err := ParseTemplate(x)
		if err != nil {
			return nil, err
		}
		return tpl.Evaluate(env)
	case map[string]any:
		if depth > maxParamDepth {
			return nil, errParamsTooDeep
		}
		out := make(map[string]any, len(x))
		for _, k := range sortedKeys(x) {
			resolved, err := resolveValue(x[k], env, depth+1)
			if err != nil {
				return nil, fmt.Errorf("%s: %w", echoKey(k), err)
			}
			out[k] = resolved
		}
		return out, nil
	case []any:
		if depth > maxParamDepth {
			return nil, errParamsTooDeep
		}
		out := make([]any, len(x))
		for i, item := range x {
			resolved, err := resolveValue(item, env, depth+1)
			if err != nil {
				return nil, fmt.Errorf("[%d]: %w", i, err)
			}
			out[i] = resolved
		}
		return out, nil
	}
	return v, nil
}

// ResolveParams resolves all templates in a node's parameters. It follows the
// rules of ResolveValue, including the aliasing contract; the parameter map
// itself counts as the first nesting level.
func ResolveParams(params map[string]any, env *Env) (map[string]any, error) {
	out := make(map[string]any, len(params))
	for _, k := range sortedKeys(params) {
		resolved, err := resolveValue(params[k], env, 2)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", echoKey(k), err)
		}
		out[k] = resolved
	}
	return out, nil
}

// echoKey shortens a parameter or map key for an error message like
// quoteForError shortens values: short keys stay readable and a huge key in a
// flow document cannot flood run records. Keys with control characters or other
// unprintable text are quoted, so a key such as "a\nFAKE" cannot inject a line
// break into a log.
func echoKey(k string) string {
	s := truncateForError(k)
	if !utf8.ValidString(s) || strings.IndexFunc(s, func(r rune) bool { return !unicode.IsPrint(r) }) >= 0 {
		return strconv.Quote(s)
	}
	return s
}

// TemplateRef is one expression found in node parameters. Param is a display
// path such as "rows[0].left": every key in it is cut like an error echo (40
// runes plus an ellipsis), so a huge key cannot bloat the path of each of its
// refs. TopParam therefore returns the full parameter name only for names of 40
// runes or fewer, which covers every real one.
type TemplateRef struct {
	Param string
	Expr  *Expr
}

// TemplateProblem is a parameter whose template does not parse, a container
// nested deeper than 32 levels, or the point where CollectTemplateRefs gave up
// after more than 1000 expressions and problems. Param is a display path cut
// like TemplateRef.Param.
type TemplateProblem struct {
	Param string
	Err   error
}

// CollectTemplateRefs walks params (keys sorted) and returns every expression
// with its parameter path, e.g. "rows[0].left", plus all parse problems. A map
// or list nested deeper than 32 levels is reported as a problem at its path and
// not walked further. Refs and problems together are capped at 1000: when the
// walk would exceed that, one more problem at the current path says so and the
// walk stops, so the result is bounded however the parameters are shaped.
func CollectTemplateRefs(params map[string]any) ([]TemplateRef, []TemplateProblem) {
	var refs []TemplateRef
	var problems []TemplateProblem
	total := 0
	stopped := false
	// reserve takes one of the maxTemplateRefs slots. When none is left it
	// reports the overflow at path instead and stops the whole walk.
	reserve := func(path string) bool {
		if total >= maxTemplateRefs {
			problems = append(problems, TemplateProblem{Param: path, Err: errTooManyTemplateRefs})
			stopped = true
			return false
		}
		total++
		return true
	}
	var walk func(path string, v any, depth int)
	walk = func(path string, v any, depth int) {
		if stopped {
			return
		}
		switch x := v.(type) {
		case string:
			if !strings.Contains(x, "{{") {
				return
			}
			tpl, err := ParseTemplate(x)
			if err != nil {
				if reserve(path) {
					problems = append(problems, TemplateProblem{Param: path, Err: err})
				}
				return
			}
			for _, e := range tpl.Exprs() {
				if !reserve(path) {
					return
				}
				refs = append(refs, TemplateRef{Param: path, Expr: e})
			}
		case map[string]any:
			if depth > maxParamDepth {
				if reserve(path) {
					problems = append(problems, TemplateProblem{Param: path, Err: errParamsTooDeep})
				}
				return
			}
			for _, k := range sortedKeys(x) {
				walk(path+"."+truncateForError(k), x[k], depth+1)
				if stopped {
					return
				}
			}
		case []any:
			if depth > maxParamDepth {
				if reserve(path) {
					problems = append(problems, TemplateProblem{Param: path, Err: errParamsTooDeep})
				}
				return
			}
			for i, item := range x {
				walk(path+"["+strconv.Itoa(i)+"]", item, depth+1)
				if stopped {
					return
				}
			}
		}
	}
	for _, k := range sortedKeys(params) {
		walk(truncateForError(k), params[k], 2)
		if stopped {
			break
		}
	}
	return refs, problems
}

// TopParam returns the top-level parameter name of a path like "rows[0].left".
func TopParam(path string) string {
	if i := strings.IndexAny(path, ".["); i >= 0 {
		return path[:i]
	}
	return path
}

func sortedKeys(m map[string]any) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
