package flows

import "aurago/internal/security"

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
// a filter (truncate, replace) reads the plain value.
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
