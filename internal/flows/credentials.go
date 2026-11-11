package flows

import (
	"bytes"
	"encoding/json"
	"maps"
	"net/url"
	"regexp"
	"strconv"
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

// CredentialRedactionMarker is what the credential rule puts in the place of a typed-in credential.
const CredentialRedactionMarker = redactedText

// Where a finding sits (CredentialFinding.Location).
const (
	credHeader   = "header"
	credUserinfo = "url_userinfo"
	credQuery    = "url_query"
	credBody     = "body_field"
	credParam    = "param_field"
	credFormat   = "format"
)

// CredentialFinding is one typed-in credential in a flow document. Param is the node parameter;
// Path says where inside it: the header name, the query key, a JSON pointer (for a URL inside a
// structure "<pointer>?<key>", for user info "userinfo"); Movable is true for an http.request
// header, URL query value or URL user info whose node has no auth_secret yet. Value holds the
// literal for the code that moves or purges it and is never serialised.
type CredentialFinding struct {
	NodeID   string `json:"node_id"`
	Param    string `json:"param"`
	Location string `json:"location"`
	Path     string `json:"path"`
	Movable  bool   `json:"movable"`
	Value    string `json:"-"`
}

var (
	// credentialExactNames are credential key names after normalCredentialName. "key" is no
	// field name: it counts only as a URL query key (credentialQueryKey).
	credentialExactNames = map[string]bool{"token": true, "accesstoken": true, "refreshtoken": true, "idtoken": true, "apikey": true,
		"secret": true, "clientsecret": true, "password": true, "passwd": true, "pwd": true, "sig": true, "signature": true,
		"auth": true, "authorization": true, "xapikey": true, "privatekey": true, "secretkey": true, "accesskey": true, "sessiontoken": true}
	credentialSuffixes    = []string{"token", "secret", "password", "apikey"}
	credentialTokenFormat = regexp.MustCompile(`^(?:(?:sk-|sk_live_|rk_live_)[A-Za-z0-9_-]{20,}|(?:ghp_|gho_|ghu_|ghs_|ghr_)[A-Za-z0-9]{16,}|github_pat_[A-Za-z0-9_]{16,}|glpat-[A-Za-z0-9_-]{16,}|xox[bpar]-[A-Za-z0-9-]{10,}|AKIA[0-9A-Z]{16}|AIza[0-9A-Za-z_-]{35}|eyJ[A-Za-z0-9_-]+\.[A-Za-z0-9_-]+\.[A-Za-z0-9_-]+)$`)
	credentialWords       = regexp.MustCompile(`[ \t\r\n\f]+|[^ \t\r\n\f]+`)
)

// normalCredentialName ignores case, "-" and "_".
func normalCredentialName(name string) string {
	return strings.NewReplacer("-", "", "_", "").Replace(strings.ToLower(strings.TrimSpace(name)))
}

// credentialFieldName reports whether a key of a JSON object names a credential.
func credentialFieldName(name string) bool {
	n := normalCredentialName(name)
	if credentialExactNames[n] {
		return true
	}
	for _, suffix := range credentialSuffixes {
		if strings.HasSuffix(n, suffix) {
			return true
		}
	}
	return false
}

// credentialQueryKey is credentialFieldName plus "key", a common API-key name in a query.
func credentialQueryKey(name string) bool {
	return credentialFieldName(name) || normalCredentialName(name) == "key"
}

// isCredentialToken reports whether s is a known token format of at least 16 characters.
func isCredentialToken(s string) bool {
	return len(s) >= 16 && !strings.Contains(s, "{{") && credentialTokenFormat.MatchString(s)
}

// literalCredentialText reports whether s is typed-in text: not blank, not the marker, no template.
func literalCredentialText(s string) bool {
	return strings.TrimSpace(s) != "" && s != CredentialRedactionMarker && !HasTemplate(s)
}

// literalHeaderValue is literalCredentialText for a credential header, where a bare auth
// scheme around templates ("Bearer {{x}}") is still no typed-in text (templateOnlyCredential).
func literalHeaderValue(s string) bool {
	return strings.TrimSpace(s) != "" && s != CredentialRedactionMarker && !templateOnlyCredential(s)
}

func pointerEscape(k string) string { return strings.NewReplacer("~", "~0", "/", "~1").Replace(k) }

func unescapeQuery(s string) string {
	if d, err := url.QueryUnescape(s); err == nil {
		return d
	}
	return s
}

// credentialURL is a web address cut at the places the rule cares about; String puts it back
// together byte for byte. It tolerates templates anywhere, which net/url does not.
type credentialURL struct {
	prefix, userinfo, hostPath, query, fragment string
	hasUser, hasQuery                           bool
}

func asciiPrefixFold(s, prefix string) bool {
	if len(s) < len(prefix) {
		return false
	}
	for i := 0; i < len(prefix); i++ {
		c := s[i]
		if 'A' <= c && c <= 'Z' {
			c += 'a' - 'A'
		}
		if c != prefix[i] {
			return false
		}
	}
	return true
}

// splitCredentialURL cuts an http:// or https:// word; ok is false for any other text.
func splitCredentialURL(s string) (u credentialURL, ok bool) {
	switch {
	case asciiPrefixFold(s, "https://"):
		u.prefix = s[:8]
	case asciiPrefixFold(s, "http://"):
		u.prefix = s[:7]
	default:
		return u, false
	}
	rest := s[len(u.prefix):]
	if i := strings.IndexByte(rest, '#'); i >= 0 {
		u.fragment, rest = rest[i:], rest[:i]
	}
	if i := strings.IndexByte(rest, '?'); i >= 0 {
		u.query, u.hasQuery, rest = rest[i+1:], true, rest[:i]
	}
	authority, tail := rest, ""
	if i := strings.IndexByte(rest, '/'); i >= 0 {
		authority, tail = rest[:i], rest[i:]
	}
	if i := strings.LastIndexByte(authority, '@'); i >= 0 {
		u.userinfo, u.hasUser, authority = authority[:i], true, authority[i+1:]
	}
	u.hostPath = authority + tail
	return u, true
}

func (u credentialURL) String() string {
	var b strings.Builder
	b.WriteString(u.prefix)
	if u.hasUser {
		b.WriteString(u.userinfo)
		b.WriteByte('@')
	}
	b.WriteString(u.hostPath)
	if u.hasQuery {
		b.WriteByte('?')
		b.WriteString(u.query)
	}
	b.WriteString(u.fragment)
	return b.String()
}

// userinfoSecret: user info is a credential when it has a typed password, or is a token itself.
func userinfoSecret(userinfo string) bool {
	if !literalCredentialText(userinfo) {
		return false
	}
	if _, pass, ok := strings.Cut(userinfo, ":"); ok {
		return literalCredentialText(pass)
	}
	return isCredentialToken(userinfo)
}

type credentialHit struct {
	location, path, value string
	top                   bool // found in a top-level string, not inside a list or an object
}

// credentialWalker walks a decoded JSON value once. It records every hit and returns a copy of
// the value with each hit replaced by the marker; detection and redaction are the same walk.
type credentialWalker struct {
	fields string // the location of credential-named keys
	hits   []credentialHit
}

func (w *credentialWalker) walk(v any, ptr string) any {
	switch x := v.(type) {
	case string:
		return w.text(x, ptr)
	case []any:
		out := make([]any, len(x))
		for i, item := range x {
			out[i] = w.walk(item, ptr+"/"+strconv.Itoa(i))
		}
		return out
	case map[string]any:
		out := make(map[string]any, len(x))
		for _, k := range sortedKeys(x) {
			out[k] = w.entry(k, x[k], ptr)
		}
		return out
	}
	return v
}

// entry handles one key of an object: a credential-named key with a typed-in text is a hit.
func (w *credentialWalker) entry(k string, val any, ptr string) any {
	p := ptr + "/" + pointerEscape(k)
	if s, ok := val.(string); ok && credentialFieldName(k) && literalCredentialText(s) {
		w.hits = append(w.hits, credentialHit{location: w.fields, path: p, value: s})
		return CredentialRedactionMarker
	}
	return w.walk(val, p)
}

// text checks the whitespace-separated words of a string: web addresses and token formats.
func (w *credentialWalker) text(s, ptr string) string {
	var b strings.Builder
	for _, tok := range credentialWords.FindAllString(s, -1) {
		b.WriteString(w.word(tok, ptr))
	}
	return b.String()
}

func (w *credentialWalker) word(tok, ptr string) string {
	switch tok[0] {
	case ' ', '\t', '\r', '\n', '\f':
		return tok
	}
	if u, ok := splitCredentialURL(tok); ok {
		return w.urlText(u, ptr)
	}
	if isCredentialToken(tok) {
		w.hits = append(w.hits, credentialHit{location: credFormat, path: ptr, value: tok, top: ptr == ""})
		return CredentialRedactionMarker
	}
	return tok
}

func credentialPath(ptr, name string) string {
	if ptr == "" {
		return name
	}
	return ptr + "?" + name
}

func (w *credentialWalker) urlText(u credentialURL, ptr string) string {
	if u.hasUser && userinfoSecret(u.userinfo) {
		w.hits = append(w.hits, credentialHit{location: credUserinfo, path: credentialPath(ptr, "userinfo"), value: u.userinfo, top: ptr == ""})
		u.userinfo = CredentialRedactionMarker
	}
	if u.hasQuery {
		pairs := strings.Split(u.query, "&")
		for i, pair := range pairs {
			key, val, ok := strings.Cut(pair, "=")
			if !ok || !literalCredentialText(val) {
				continue
			}
			name, dec := unescapeQuery(key), unescapeQuery(val)
			if credentialQueryKey(name) || isCredentialToken(dec) {
				w.hits = append(w.hits, credentialHit{location: credQuery, path: credentialPath(ptr, name), value: dec, top: ptr == ""})
				pairs[i] = key + "=" + CredentialRedactionMarker
			}
		}
		u.query = strings.Join(pairs, "&")
	}
	return u.String()
}

func decodeJSONContainer(s string) (any, bool) {
	t := strings.TrimSpace(s)
	if t == "" || (t[0] != '{' && t[0] != '[') {
		return nil, false
	}
	dec := json.NewDecoder(strings.NewReader(t))
	dec.UseNumber()
	var v any
	if dec.Decode(&v) != nil || dec.More() {
		return nil, false
	}
	return v, true
}

func encodeJSONCompact(v any) (string, bool) {
	var b bytes.Buffer
	enc := json.NewEncoder(&b)
	enc.SetEscapeHTML(false)
	if enc.Encode(v) != nil {
		return "", false
	}
	return strings.TrimSuffix(b.String(), "\n"), true
}

// scanCredentialHeaders is the header rule of http.request: credential headers keep their
// historic template handling (literalHeaderValue); other headers are walked like any value.
func scanCredentialHeaders(v any) ([]credentialHit, any) {
	m, ok := v.(map[string]any)
	if !ok {
		return nil, v
	}
	w := &credentialWalker{fields: credParam}
	out := make(map[string]any, len(m))
	for _, k := range sortedKeys(m) {
		name := strings.TrimSpace(k)
		if s, isText := m[k].(string); isText && credentialHeaderName(name) {
			out[k] = s
			if literalHeaderValue(s) {
				w.hits = append(w.hits, credentialHit{location: credHeader, path: name, value: s, top: true})
				out[k] = CredentialRedactionMarker
			}
			continue
		}
		out[k] = w.walk(m[k], "/"+pointerEscape(k))
	}
	return w.hits, out
}

// scanCredentialParam applies the rule to one node parameter. It returns the hits and a copy
// of the parameter with every hit replaced by the marker. Credential-named keys count at any
// depth of a structured parameter and of a JSON body; at the top level of a node only for the
// generic tool.* nodes (the top-level names of other nodes include auth_secret, a vault name).
func scanCredentialParam(nodeType, name string, v any) ([]credentialHit, any) {
	w := &credentialWalker{fields: credParam}
	switch {
	case nodeType == TypeHTTPRequest && name == "headers":
		return scanCredentialHeaders(v)
	case nodeType == TypeHTTPRequest && name == "body":
		w.fields = credBody
		if s, ok := v.(string); ok {
			if parsed, isJSON := decodeJSONContainer(s); isJSON {
				red := w.walk(parsed, "")
				if len(w.hits) == 0 {
					return nil, v
				}
				if text, ok := encodeJSONCompact(red); ok {
					return w.hits, text
				}
				return w.hits, CredentialRedactionMarker
			}
		}
	case strings.HasPrefix(nodeType, GenericTypePrefix):
		red := w.entry(name, v, "")
		return w.hits, red
	}
	red := w.walk(v, "")
	return w.hits, red
}

// credentialMovable: only an http.request whose auth_secret is free can take the credential.
func credentialMovable(n *Node, param string, h credentialHit) bool {
	if n.Type != TypeHTTPRequest || !isEmptyValue(n.Params["auth_secret"]) {
		return false
	}
	switch h.location {
	case credHeader:
		return param == "headers" && !HasTemplate(h.value)
	case credQuery, credUserinfo:
		return param == "url" && h.top
	}
	return false
}

func nodeCredentialFindings(n *Node) []CredentialFinding {
	var out []CredentialFinding
	for _, name := range sortedKeys(n.Params) {
		hits, _ := scanCredentialParam(n.Type, name, n.Params[name])
		for _, h := range hits {
			out = append(out, CredentialFinding{NodeID: n.ID, Param: name, Location: h.location, Path: h.path, Value: h.value, Movable: credentialMovable(n, name, h)})
		}
	}
	return out
}

// FindLiteralCredentials returns the typed-in credentials of every node of a flow document,
// in node order, then parameter name order.
func FindLiteralCredentials(doc *Flow) []CredentialFinding {
	if doc == nil {
		return nil
	}
	var out []CredentialFinding
	for i := range doc.Nodes {
		out = append(out, nodeCredentialFindings(&doc.Nodes[i])...)
	}
	return out
}

// literalCredentialsInValue walks any decoded JSON value (run, step and test-data documents
// for the purge) and returns the distinct literals the rule finds, in walk order.
func literalCredentialsInValue(v any) []string {
	w := &credentialWalker{fields: credParam}
	w.walk(v, "")
	seen := map[string]bool{}
	var out []string
	for _, h := range w.hits {
		if !seen[h.value] {
			seen[h.value] = true
			out = append(out, h.value)
		}
	}
	return out
}

// redactCredentialValue returns a copy of a decoded JSON value with every finding replaced
// by the marker.
func redactCredentialValue(v any) any {
	return (&credentialWalker{fields: credParam}).walk(v, "")
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

// redactCredentialParams returns params with every finding of the rule replaced by the marker,
// for what is shown or handed out: exports (keepTemplates: a template-only header is no
// secret) and the run log (resolved values, all replaced). params is never modified; the
// result is params itself when nothing was replaced.
func redactCredentialParams(nodeType string, params map[string]any, keepTemplates bool) map[string]any {
	out := redactCredentialHeaders(params, keepTemplates)
	var copied map[string]any
	for _, name := range sortedKeys(out) {
		hits, red := scanCredentialParam(nodeType, name, out[name])
		if len(hits) == 0 {
			continue
		}
		if copied == nil {
			copied = maps.Clone(out)
		}
		copied[name] = red
	}
	if copied == nil {
		return out
	}
	return copied
}

// credentialMessage is the English text of a PARAM_LITERAL_CREDENTIAL issue; the editor
// translates by the issue's location.
func credentialMessage(f CredentialFinding) string {
	const tail = "; it stays in the flow, its versions and run logs. Store it as a secret instead"
	switch f.Location {
	case credHeader:
		return "the header " + quoteForError(f.Path) + " holds a credential typed in clear text" + tail
	case credUserinfo:
		return "the web address holds a user name and password typed in clear text" + tail
	case credQuery:
		return "the query parameter " + quoteForError(f.Path) + " of the web address holds a credential typed in clear text" + tail
	case credFormat:
		return "the parameter " + quoteForError(f.Param) + " holds text that looks like an access token" + tail
	}
	return "the field " + quoteForError(f.Path) + " of " + quoteForError(f.Param) + " holds a credential typed in clear text" + tail
}

// credentialIssues warns once per finding of n. It is a warning in every mode: publishing is
// not blocked (S2).
func (v *validator) credentialIssues(n *Node) {
	for _, f := range nodeCredentialFindings(n) {
		v.emit(Issue{Code: IssueLiteralCredential, Severity: SeverityWarning, NodeID: n.ID, Param: f.Param,
			Location: f.Location, Path: f.Path, Movable: f.Movable, Message: credentialMessage(f)})
	}
}

// RedactCredentials returns a copy of f for export in which every typed-in credential of the
// credential rule is replaced by CredentialRedactionMarker (S6) and whose group texts, sticky
// notes and step notes have registered secrets scrubbed; f is not changed.
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
	// Sticky notes and step notes stay in exports, packs, the run view and child history,
	// with registered secrets scrubbed. Scrubbing can turn a secret into a longer marker, so a
	// sticky note is cut back to its import bound and the export of a valid draft stays importable.
	for i := range c.Notes {
		text := security.ScrubSecrets(c.Notes[i].Text)
		if utf8.RuneCountInString(text) > maxNoteRunes {
			text = truncateRunes(text, maxNoteRunes-1)
		}
		c.Notes[i].Text = text
	}
	for i := range c.Nodes {
		c.Nodes[i].Settings.Notes = security.ScrubSecrets(c.Nodes[i].Settings.Notes)
		c.Nodes[i].Params = redactCredentialParams(c.Nodes[i].Type, c.Nodes[i].Params, true)
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
