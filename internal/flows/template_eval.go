package flows

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
)

// Evaluate resolves the template against env. A single-expression template
// returns the expression's value with its JSON type; otherwise all parts are
// concatenated as text.
func (t *Template) Evaluate(env *Env) (any, error) {
	if t.IsSingleExpr() {
		return evalExpr(t.parts[0].expr, env)
	}
	var b strings.Builder
	for _, part := range t.parts {
		if part.expr == nil {
			b.WriteString(part.literal)
			continue
		}
		v, err := evalExpr(part.expr, env)
		if err != nil {
			return nil, err
		}
		b.WriteString(Stringify(v))
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
// lists. Other values are returned unchanged. The input is never modified.
func ResolveValue(v any, env *Env) (any, error) {
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
		out := make(map[string]any, len(x))
		for k, item := range x {
			resolved, err := ResolveValue(item, env)
			if err != nil {
				return nil, fmt.Errorf("%s: %w", echoKey(k), err)
			}
			out[k] = resolved
		}
		return out, nil
	case []any:
		out := make([]any, len(x))
		for i, item := range x {
			resolved, err := ResolveValue(item, env)
			if err != nil {
				return nil, fmt.Errorf("[%d]: %w", i, err)
			}
			out[i] = resolved
		}
		return out, nil
	}
	return v, nil
}

// ResolveParams resolves all templates in a node's parameters.
func ResolveParams(params map[string]any, env *Env) (map[string]any, error) {
	out := make(map[string]any, len(params))
	for k, v := range params {
		resolved, err := ResolveValue(v, env)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", echoKey(k), err)
		}
		out[k] = resolved
	}
	return out, nil
}

// echoKey shortens a parameter or map key for an error message the same way
// quoteForError shortens values, but without quotes: short keys stay readable
// and a huge key in a flow document cannot flood run records.
func echoKey(k string) string {
	n := 0
	for i := range k {
		if n == maxErrorEchoRunes {
			return k[:i] + "…"
		}
		n++
	}
	return k
}

// TemplateRef is one expression found in node parameters.
type TemplateRef struct {
	Param string
	Expr  *Expr
}

// TemplateProblem is a parameter whose template does not parse.
type TemplateProblem struct {
	Param string
	Err   error
}

// CollectTemplateRefs walks params (keys sorted) and returns every expression
// with its parameter path, e.g. "rows[0].left", plus all parse problems.
func CollectTemplateRefs(params map[string]any) ([]TemplateRef, []TemplateProblem) {
	var refs []TemplateRef
	var problems []TemplateProblem
	var walk func(path string, v any)
	walk = func(path string, v any) {
		switch x := v.(type) {
		case string:
			if !strings.Contains(x, "{{") {
				return
			}
			tpl, err := ParseTemplate(x)
			if err != nil {
				problems = append(problems, TemplateProblem{Param: path, Err: err})
				return
			}
			for _, e := range tpl.Exprs() {
				refs = append(refs, TemplateRef{Param: path, Expr: e})
			}
		case map[string]any:
			for _, k := range sortedKeys(x) {
				walk(path+"."+k, x[k])
			}
		case []any:
			for i, item := range x {
				walk(path+"["+strconv.Itoa(i)+"]", item)
			}
		}
	}
	for _, k := range sortedKeys(params) {
		walk(k, params[k])
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
