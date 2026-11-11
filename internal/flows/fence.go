package flows

import (
	"errors"
	"regexp"

	"aurago/internal/security"
)

// fencesRoot reports whether a template expression that reads root must have its value
// fenced: the Env resolves a FenceUntrusted parameter and the root holds untrusted data.
func (e *Env) fencesRoot(root string) bool {
	return e != nil && e.fence && e.Untrusted[root]
}

// fenceUntrusted wraps the text of v in the external-data fence the agent loop puts around
// tool output (security.IsolateExternalData: format characters stripped, boundary
// look-alikes folded, HTML-escaped, inside a fixed wrapper the body cannot emit). A model
// that reads a flow prompt then sees which part came from outside. A list or an object
// becomes one fenced text of its JSON (the fence must not be spelled by data, so there is
// no per-leaf wrapping inside a JSON string); empty text, numbers, booleans and null stay
// as they are, since they carry no instruction. The fence is applied after the filters, so
// a filter (truncate, replace) reads the plain value. A ParamJSON parameter uses
// fenceUntrustedLeaves instead.
func fenceUntrusted(v any) any {
	switch x := v.(type) {
	case string:
		if x == "" {
			return x
		}
		return security.IsolateExternalData(x)
	case []any, map[string]any:
		if s := Stringify(x); s != "" {
			return security.IsolateExternalData(s)
		}
	}
	return v
}

// fenceKeyPattern matches the object keys of untrusted data that the per-leaf fence leaves
// as they are: identifier-like keys carry no instruction.
var fenceKeyPattern = regexp.MustCompile(`^[A-Za-z0-9_.-]{1,64}$`)

// errFencedKeyCollision: two keys of an untrusted object read the same once fenced (a
// look-alike that the fence folds or strips). The engine fails the step with FLOW_PARAM_INVALID.
var errFencedKeyCollision = errors.New("two keys of the untrusted data read the same after fencing")

// fenceUntrustedLeaves is the per-leaf fence of a ParamJSON parameter (ParamSpec.FenceUntrusted)
// for a value that a single-expression template reads from an untrusted root. The value keeps
// its JSON type. Each non-empty string leaf goes through security.IsolateExternalData. Object
// keys that look like identifiers stay, and every other key is fenced as well, because a key
// taken from a webhook body can carry an instruction. Empty text, numbers, booleans and null
// stay. Lists and objects are rebuilt, so the value shared with the env is never changed.
// Deeper than maxParamDepth, the rest becomes one fenced text (fenceUntrusted).
func fenceUntrustedLeaves(v any) (any, error) {
	return fenceLeavesAt(v, 1)
}

func fenceLeavesAt(v any, depth int) (any, error) {
	switch x := v.(type) {
	case string:
		return fenceUntrusted(x), nil
	case []any:
		if depth > maxParamDepth {
			return fenceUntrusted(x), nil
		}
		out := make([]any, len(x))
		for i, item := range x {
			fenced, err := fenceLeavesAt(item, depth+1)
			if err != nil {
				return nil, err
			}
			out[i] = fenced
		}
		return out, nil
	case map[string]any:
		if depth > maxParamDepth {
			return fenceUntrusted(x), nil
		}
		out := make(map[string]any, len(x))
		for _, k := range sortedKeys(x) {
			key := k
			if !fenceKeyPattern.MatchString(k) {
				key = security.IsolateExternalData(k)
			}
			if _, taken := out[key]; taken {
				return nil, errFencedKeyCollision
			}
			fenced, err := fenceLeavesAt(x[k], depth+1)
			if err != nil {
				return nil, err
			}
			out[key] = fenced
		}
		return out, nil
	}
	return v, nil
}
