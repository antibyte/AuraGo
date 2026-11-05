package flows

import (
	"maps"
	"sort"
	"strings"
	"unicode/utf8"

	"aurago/internal/security"
)

// credentialHeaderName reports whether a request header of this name carries a credential: the
// usual names, and any name with api-key, api_key, apikey, token, secret or password in it.
func credentialHeaderName(name string) bool {
	n := strings.ToLower(strings.TrimSpace(name))
	switch n {
	case "authorization", "proxy-authorization", "cookie", "x-api-key", "api-key", "apikey", "x-auth-token", "x-access-token":
		return true
	}
	for _, part := range []string{"api-key", "api_key", "apikey", "token", "secret", "password"} {
		if strings.Contains(n, part) {
			return true
		}
	}
	return false
}

// templateOnlyCredential reports whether a header value carries no typed-in text: after the
// template expressions are taken out, nothing is left but whitespace or a bare auth scheme word
// (Bearer, Basic, Token). Text mixed with a template ("sid=1; lang={{x}}") is typed in.
func templateOnlyCredential(s string) bool {
	if !HasTemplate(s) {
		return false
	}
	t, err := ParseTemplate(s)
	if err != nil {
		return false
	}
	var rest strings.Builder
	for _, p := range t.parts {
		if p.expr == nil {
			rest.WriteString(p.literal)
		}
	}
	switch strings.ToLower(strings.TrimSpace(rest.String())) {
	case "", "bearer", "basic", "token":
		return true
	}
	return false
}

// literalCredentialHeaders returns, sorted, the names of the credential headers in a headers
// parameter whose value is typed in: non-blank text that is not a pure template. It is a hook helper: it
// copes with any value.
func literalCredentialHeaders(v any) []string {
	m, ok := v.(map[string]any)
	if !ok {
		return nil
	}
	var names []string
	for k, raw := range m {
		name := strings.TrimSpace(k)
		s, isText := raw.(string)
		if !isText || strings.TrimSpace(s) == "" || templateOnlyCredential(s) || s == redactedText || !credentialHeaderName(name) {
			continue
		}
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// redactCredentialHeaders returns params with the values of credential headers in its "headers"
// map replaced by redactedText, for what is shown or handed out: exports (keepTemplates, a
// template there is no secret) and the run log (resolved values, all replaced). params is never
// modified (it may alias run data): the result is a copy when something was replaced, else params.
func redactCredentialHeaders(params map[string]any, keepTemplates bool) map[string]any {
	headers, ok := params["headers"].(map[string]any)
	if !ok || len(headers) == 0 {
		return params
	}
	var copied map[string]any
	for name, v := range headers {
		if !credentialHeaderName(name) || isEmptyValue(v) {
			continue
		}
		if s, isText := v.(string); isText && (s == redactedText || keepTemplates && templateOnlyCredential(s)) {
			continue
		}
		if copied == nil {
			copied = maps.Clone(headers)
		}
		copied[name] = redactedText
	}
	if copied == nil {
		return params
	}
	out := maps.Clone(params)
	out["headers"] = copied
	return out
}

// RedactCredentials returns a copy of f for export whose header maps hold redactedText instead of
// typed-in credentials (S6); f is not changed.
func RedactCredentials(f *Flow) (*Flow, error) {
	if f == nil {
		return nil, nil
	}
	c, err := f.Clone()
	if err != nil {
		return nil, err
	}
	if c.Groups != nil {
		for i := range *c.Groups {
			group := &(*c.Groups)[i]
			group.Title = security.ScrubSecrets(group.Title)
			group.Note = security.ScrubSecrets(group.Note)
		}
	}
	for i := range c.Nodes {
		c.Nodes[i].Params = redactCredentialHeaders(c.Nodes[i].Params, true)
	}
	return c, nil
}

// ExportFlow marks nonempty visual groups so older importers fail closed.
// This marker is confined to export files; live and persisted schemas stay 1.
func ExportFlow(f *Flow) (*Flow, error) {
	copy, err := RedactCredentials(f)
	if err != nil || copy == nil {
		return copy, err
	}
	if copy.Groups != nil && len(*copy.Groups) > 0 {
		// Scrubbing can expand a secret into a longer marker. Keep the complete
		// scrub before truncation, then retain valid import bounds with an ellipsis.
		for i := range *copy.Groups {
			group := &(*copy.Groups)[i]
			if utf8.RuneCountInString(group.Title) > 80 {
				group.Title = truncateRunes(group.Title, 79)
			}
			if utf8.RuneCountInString(group.Note) > 2048 {
				group.Note = truncateRunes(group.Note, 2047)
			}
		}
		copy.Schema = 2
	}
	encoded, err := copy.Marshal()
	if err != nil {
		return nil, err
	}
	if len(encoded) > MaxDocumentBytes {
		return nil, ErrDocumentTooLarge
	}
	return copy, nil
}
