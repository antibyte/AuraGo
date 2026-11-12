package flows

import (
	"bytes"
	"encoding/json"
	"maps"
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"

	"aurago/internal/security"
)

// credentialHeaderName reports whether a request header of this name carries a credential: the
// usual names, and any name with api-key, api_key, apikey, token, secret or password in it.
func credentialHeaderName(name string) bool {
	n := asciiLower(asciiTrim(name))
	switch n {
	case "authorization", "proxy-authorization", "cookie", "x-api-key", "api-key", "apikey", "x-auth-token", "x-access-token", "x-auth", "x-session":
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
	switch asciiLower(asciiTrim(rest.String())) {
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
		"auth": true, "authorization": true, "xapikey": true, "privatekey": true, "secretkey": true, "accesskey": true, "sessiontoken": true,
		"secrets": true, "tokens": true, "apikeys": true}
	credentialSuffixes    = []string{"token", "secret", "password", "apikey", "secrets", "passwords", "apikeys"}
	credentialTokenFormat = regexp.MustCompile(`^(?:(?:sk-|sk_live_|rk_live_)[A-Za-z0-9_-]{20,}|(?:ghp_|gho_|ghu_|ghs_|ghr_)[A-Za-z0-9]{16,}|github_pat_[A-Za-z0-9_]{16,}|glpat-[A-Za-z0-9_-]{16,}|xox[bpar]-[A-Za-z0-9-]{10,}|AKIA[0-9A-Z]{16}|AIza[0-9A-Za-z_-]{35}|eyJ[A-Za-z0-9_-]+\.[A-Za-z0-9_-]+\.[A-Za-z0-9_-]+)$`)
	credentialWords       = regexp.MustCompile(`[ \t\r\n\f]+|[^ \t\r\n\f]+`)
)

// asciiLower folds the ASCII letters only. The client rule folds the same way (the two must agree
// on every name), so no Unicode case folding: a dotted capital I is no "i".
func asciiLower(s string) string {
	for i := 0; i < len(s); i++ {
		if 'A' <= s[i] && s[i] <= 'Z' {
			b := []byte(s)
			for j := i; j < len(b); j++ {
				if 'A' <= b[j] && b[j] <= 'Z' {
					b[j] += 'a' - 'A'
				}
			}
			return string(b)
		}
	}
	return s
}

// asciiTrim cuts ASCII white space; the client rule trims the same set.
func asciiTrim(s string) string { return strings.Trim(s, " \t\n\v\f\r") }

// normalCredentialName ignores ASCII case, "-" and "_".
func normalCredentialName(name string) string {
	return strings.NewReplacer("-", "", "_", "").Replace(asciiLower(asciiTrim(name)))
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

// literalCredentialText reports whether s is typed-in text: not blank, not the marker, and not
// only a template (a bare auth scheme around templates, "Bearer {{x}}", is still no typed-in
// text, see templateOnlyCredential). Text next to a template ("sk-abc{{x}}") is typed in.
func literalCredentialText(s string) bool {
	return asciiTrim(s) != "" && s != CredentialRedactionMarker && !templateOnlyCredential(s)
}

// literalHeaderValue is the same test for a credential header.
func literalHeaderValue(s string) bool { return literalCredentialText(s) }

func pointerEscape(k string) string { return strings.NewReplacer("~", "~0", "/", "~1").Replace(k) }

func hexDigit(c byte) int {
	switch {
	case '0' <= c && c <= '9':
		return int(c - '0')
	case 'a' <= c && c <= 'f':
		return int(c-'a') + 10
	case 'A' <= c && c <= 'F':
		return int(c-'A') + 10
	}
	return -1
}

// unescapeQuery decodes "+" and every valid %XX byte by byte; an invalid sequence stays as it is.
// A byte that is no valid UTF-8 becomes U+FFFD, one per byte; the client rule does the same, so
// both sides agree on every name and every finding.
func unescapeQuery(s string) string {
	if !strings.ContainsAny(s, "%+") {
		return s
	}
	b := make([]byte, 0, len(s))
	for i := 0; i < len(s); i++ {
		switch c := s[i]; {
		case c == '+':
			b = append(b, ' ')
		case c == '%' && i+2 < len(s) && hexDigit(s[i+1]) >= 0 && hexDigit(s[i+2]) >= 0:
			b = append(b, byte(hexDigit(s[i+1])<<4|hexDigit(s[i+2])))
			i += 2
		default:
			b = append(b, c)
		}
	}
	if utf8.Valid(b) {
		return string(b)
	}
	var out strings.Builder
	for len(b) > 0 {
		r, size := utf8.DecodeRune(b)
		out.WriteRune(r) // an invalid byte (size 1) becomes U+FFFD, one per byte
		b = b[size:]
	}
	return out.String()
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

// entry handles one key of an object: a credential-named key with a typed-in text, or with a
// list of which an item is a typed-in text, is a hit.
func (w *credentialWalker) entry(k string, val any, ptr string) any {
	p := ptr + "/" + pointerEscape(k)
	if credentialFieldName(k) {
		switch x := val.(type) {
		case string:
			if literalCredentialText(x) {
				w.hits = append(w.hits, credentialHit{location: w.fields, path: p, value: x})
				return CredentialRedactionMarker
			}
		case []any:
			return w.secretList(x, p)
		}
	}
	return w.walk(val, p)
}

// secretList is the value of a credential-named key that is a list: every text item is a secret.
func (w *credentialWalker) secretList(items []any, ptr string) []any {
	out := make([]any, len(items))
	for i, item := range items {
		p := ptr + "/" + strconv.Itoa(i)
		switch x := item.(type) {
		case string:
			if literalCredentialText(x) {
				w.hits = append(w.hits, credentialHit{location: w.fields, path: p, value: x})
				out[i] = CredentialRedactionMarker
				continue
			}
		case []any:
			out[i] = w.secretList(x, p)
			continue
		}
		out[i] = w.walk(item, p)
	}
	return out
}

// text checks a string. A string that holds a JSON object or list is scanned as that value (and
// written back as compact JSON when something was found); any other string is checked word by
// word: web addresses and token formats.
func (w *credentialWalker) text(s, ptr string) string {
	if parsed, ok := decodeJSONContainer(s); ok {
		before := len(w.hits)
		red := w.walk(parsed, ptr)
		if len(w.hits) == before {
			return s
		}
		if enc, ok := encodeJSONCompact(red); ok {
			return enc
		}
		return CredentialRedactionMarker
	}
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
		u.query = w.pairs(u.query, credQuery, func(name string) string { return credentialPath(ptr, name) }, ptr == "")
	}
	if len(u.fragment) > 1 {
		// A fragment can carry a token too ("#access_token=..."). It is never sent to the server,
		// so a hit there is not movable: top stays false.
		u.fragment = "#" + w.pairs(u.fragment[1:], credQuery, func(name string) string { return credentialPath(ptr, name) }, false)
	}
	return u.String()
}

// pairs applies the query rule to a "k=v&k=v" string (a URL query, a URL fragment, a form body):
// a pair whose key names a credential or whose value is a token format, with a typed-in value,
// is a hit; only the value is replaced.
func (w *credentialWalker) pairs(s, location string, path func(name string) string, top bool) string {
	parts := strings.Split(s, "&")
	for i, pair := range parts {
		key, val, ok := strings.Cut(pair, "=")
		if !ok || !literalCredentialText(val) {
			continue
		}
		name, dec := unescapeQuery(key), unescapeQuery(val)
		if credentialQueryKey(name) || isCredentialToken(dec) {
			w.hits = append(w.hits, credentialHit{location: location, path: path(name), value: dec, top: top})
			parts[i] = key + "=" + CredentialRedactionMarker
		}
	}
	return strings.Join(parts, "&")
}

// formBody applies the query rule to a body that is no JSON, read as form data.
func (w *credentialWalker) formBody(s string) string {
	return w.pairs(s, credBody, func(name string) string { return "/" + pointerEscape(name) }, false)
}

func decodeJSONContainer(s string) (any, bool) {
	t := asciiTrim(s)
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
		name := asciiTrim(k)
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
			// No JSON: read it as form data, then check the words of what is left.
			red := w.text(w.formBody(s), "")
			if len(w.hits) == 0 {
				return nil, v
			}
			return w.hits, red
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
		return param == "url" && h.top && !HasTemplate(h.value)
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
	// Defence in depth: local packs scrub the same texts again in sanitizeLocalPackFlow
	// (local_pack.go), after this function ran. Keep both; a change to what is scrubbed here
	// must be mirrored there, and the other way round.
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
