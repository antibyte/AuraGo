package flows

import (
	"context"
	"encoding/json"
	"math"
	"sort"
	"strings"
	"unicode/utf8"
)

// Limits of http.request. Every one bounds something a flow document, a template
// or a server can make arbitrarily large before it is passed on or echoed.
const (
	maxSecretNameBytes = 256
	// maxHTTPBodyBytes bounds a request body (the tool takes it as one string).
	maxHTTPBodyBytes = 1 << 20
	// maxHTTPHeaders and maxHTTPHeaderBytes bound the request headers, the
	// credential header included: servers refuse more than about 32 KiB anyway.
	maxHTTPHeaders     = 50
	maxHTTPHeaderBytes = 32 << 10
	maxHeaderNameBytes = 128
	// apiBodyLimit is where the api_request tool cuts a response body.
	apiBodyLimit = 16384
	// maxJSONDepth bounds the nesting of a response body parsed into the "json"
	// output: the engine fails a node whose output nests deeper than its own limit
	// (about 10000), which a server could use to fail any flow that calls it.
	maxJSONDepth = 100
)

// httpMethods are the methods of the method parameter; GET is the default.
var httpMethods = []string{"GET", "POST", "PUT", "PATCH", "DELETE"}

// httpHeader is one validated request header.
type httpHeader struct{ name, value string }

// validHeaderName reports whether name is an HTTP token (RFC 9110): letters, digits
// and !#$%&'*+-.^_`|~, at most maxHeaderNameBytes long.
func validHeaderName(name string) bool {
	if name == "" || len(name) > maxHeaderNameBytes {
		return false
	}
	for i := 0; i < len(name); i++ {
		c := name[i]
		if !(c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || strings.IndexByte("!#$%&'*+-.^_`|~", c) >= 0) {
			return false
		}
	}
	return true
}

// validHeaderValue reports whether v can be sent in a header or an address: valid
// UTF-8 without control characters (CR, LF and NUL included) except the tab.
func validHeaderValue(v string) bool {
	if !utf8.ValidString(v) {
		return false
	}
	for i := 0; i < len(v); i++ {
		if c := v[i]; (c < 0x20 && c != '\t') || c == 0x7f {
			return false
		}
	}
	return true
}

// parseHeaders validates the headers parameter and returns the headers sorted by
// name, so that the first problem found does not depend on map order. Blank rows are
// skipped; a value without a name, a name that is not a token, a value with control
// characters, a name given twice (names are case-insensitive, and two spellings
// would let the transport pick one at random), a list or object as a value and a
// null value all fail with FLOW_PARAM_INVALID. The message never echoes a value and
// echoes only a name that already is a valid, bounded token. skipTemplates is for
// Validate, which sees unresolved templates: entries that hold one are not judged,
// and neither is a headers value that is a template as a whole (it resolves to the
// object at run time).
func parseHeaders(v any, skipTemplates bool) ([]httpHeader, error) {
	if isEmptyValue(v) || skipTemplates && isTemplateText(v) {
		return nil, nil
	}
	m, ok := v.(map[string]any)
	if !ok {
		return nil, NewNodeError("FLOW_PARAM_INVALID", "headers must be an object of header names and values")
	}
	if len(m) > maxHTTPHeaders {
		return nil, NewNodeError("FLOW_PARAM_INVALID", "there are %d headers; the limit is %d", len(m), maxHTTPHeaders)
	}
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	seen := make(map[string]bool, len(m))
	list := make([]httpHeader, 0, len(m))
	size := 0
	for _, k := range keys {
		name, raw := strings.TrimSpace(k), m[k]
		value, scalar := scalarText(raw)
		if skipTemplates && (HasTemplate(name) || HasTemplate(value)) {
			continue
		}
		if name == "" {
			if scalar && strings.TrimSpace(value) == "" {
				continue
			}
			return nil, NewNodeError("FLOW_PARAM_INVALID", "a header has a value but no name")
		}
		if !validHeaderName(name) {
			return nil, NewNodeError("FLOW_PARAM_INVALID", "a header name is too long or contains characters that are not allowed")
		}
		low := strings.ToLower(name)
		if seen[low] {
			return nil, NewNodeError("FLOW_PARAM_INVALID", "the header %s is given more than once", quoteForError(name))
		}
		seen[low] = true
		switch {
		case raw == nil:
			return nil, NewNodeError("FLOW_PARAM_INVALID", "the header %s has no value", quoteForError(name))
		case !scalar:
			return nil, NewNodeError("FLOW_PARAM_INVALID", "the header %s must have a text value", quoteForError(name))
		case !validHeaderValue(value):
			return nil, NewNodeError("FLOW_PARAM_INVALID", "the header %s has a value with line breaks or other characters that are not allowed", quoteForError(name))
		}
		if size += len(name) + len(value); size > maxHTTPHeaderBytes {
			return nil, NewNodeError("FLOW_PARAM_INVALID", "the headers are over %d bytes", maxHTTPHeaderBytes)
		}
		list = append(list, httpHeader{name, value})
	}
	return list, nil
}

// headerTotals applies the count and size limits to the final headers.
func headerTotals(list []httpHeader) error {
	size := 0
	for _, h := range list {
		size += len(h.name) + len(h.value)
	}
	if len(list) > maxHTTPHeaders || size > maxHTTPHeaderBytes {
		return NewNodeError("FLOW_PARAM_INVALID", "there are too many headers or they are over %d bytes", maxHTTPHeaderBytes)
	}
	return nil
}

// isTemplateText reports whether v is text that holds a template: its value is only
// known at run time, so Validate cannot judge it.
func isTemplateText(v any) bool {
	s, ok := v.(string)
	return ok && HasTemplate(s)
}

// literalIssue runs check, the function Execute reads parameter param with, on a
// value that is not a template, and reports what it rejects.
func literalIssue(n *Node, param string, check func(any) (string, error)) []Issue {
	v := n.Params[param]
	if isTemplateText(v) {
		return nil
	}
	if _, err := check(v); err != nil {
		return []Issue{paramIssue(n, IssueParamInvalid, SeverityError, param, asNodeError(err).Message)}
	}
	return nil
}

// failsOnError reports whether an HTTP error status fails the node. Only an explicit
// off (false, 0, "no", "off", "nein") tolerates it; null, a blank, a word that is
// not a flag, a list or an object keep the default, which is to fail.
func failsOnError(v any) bool {
	on, known := flagValue(v)
	return on || !known
}

// authPrefixValue reads auth_prefix: text without control characters, "" when null.
func authPrefixValue(v any) (string, error) {
	s, err := scalarTextParam(v, "auth_prefix")
	if err != nil {
		return "", err
	}
	if !validHeaderValue(s) {
		return "", NewNodeError("FLOW_PARAM_INVALID", "auth_prefix contains characters that are not allowed in a header")
	}
	return s, nil
}

// authHeaderName reads auth_header: a valid header name, "Authorization" when blank.
func authHeaderName(v any) (string, error) {
	s, err := scalarTextParam(v, "auth_header")
	if err != nil {
		return "", err
	}
	if s = strings.TrimSpace(s); s == "" {
		return "Authorization", nil
	}
	if !validHeaderName(s) {
		return "", NewNodeError("FLOW_PARAM_INVALID", "auth_header is not a valid header name")
	}
	return s, nil
}

// withAuthSecret adds the credential named by auth_secret to the headers and
// returns a scrubber for its value (one that removes nothing when no secret is
// used). The vault value is trimmed (a pasted secret often ends in a newline, which
// a header cannot carry); an empty one fails instead of sending "Bearer ". When the
// request already has a header of that name, in any spelling, the vault secret wins:
// it is the deliberate setting, and data from a template must not be able to replace
// the configured credential. The value never reaches the parameters, an error
// message or an output; the node's parameters only hold the secret's name.
func withAuthSecret(in ExecInput, headers []httpHeader) ([]httpHeader, *secretScrubber, error) {
	key, err := scalarTextParam(in.Params["auth_secret"], "auth_secret")
	if err != nil {
		return nil, nil, err
	}
	if key = strings.TrimSpace(key); key == "" {
		return headers, nil, nil
	}
	if err := checkFileText(key, maxSecretNameBytes, "secret name"); err != nil {
		return nil, nil, err
	}
	name, err := authHeaderName(in.Params["auth_header"])
	if err != nil {
		return nil, nil, err
	}
	prefix, err := authPrefixValue(in.Params["auth_prefix"])
	if err != nil {
		return nil, nil, err
	}
	if in.Services == nil || in.Services.Secrets == nil {
		return nil, nil, NewNodeError("FLOW_SECRET_UNAVAILABLE", "the vault is not available")
	}
	raw, rerr := in.Services.Secrets.ReadSecret(key)
	if rerr != nil {
		return nil, nil, NewNodeError("FLOW_SECRET_UNAVAILABLE", "the secret %s could not be read", quoteForError(key))
	}
	secret := strings.TrimSpace(raw)
	switch {
	case secret == "":
		return nil, nil, NewNodeError("FLOW_SECRET_UNAVAILABLE", "the secret %s is empty", quoteForError(key))
	case !validHeaderValue(secret):
		return nil, nil, NewNodeError("FLOW_SECRET_UNAVAILABLE", "the secret %s holds line breaks or other characters that cannot be sent in a header", quoteForError(key))
	}
	scrub := newSecretScrubber(secret)
	merged := make([]httpHeader, 0, len(headers)+1)
	for _, h := range headers {
		if !strings.EqualFold(h.name, name) {
			merged = append(merged, h)
		}
	}
	merged = append(merged, httpHeader{name, prefix + secret})
	if err := headerTotals(merged); err != nil {
		return nil, nil, err
	}
	return merged, scrub, nil
}

// requestBody builds the request body: a text is sent as it is, anything else as
// compact JSON (a value that cannot be encoded fails the node). A GET has none. The
// body must be valid text of at most maxHTTPBodyBytes.
func requestBody(method string, v any) (string, bool, error) {
	if method == "GET" || isEmptyValue(v) {
		return "", false, nil
	}
	s, isText := v.(string)
	if !isText {
		var err error
		if s, err = marshalCompact(v); err != nil {
			return "", false, NewNodeError("FLOW_PARAM_INVALID", "the request body cannot be encoded as JSON")
		}
	}
	switch {
	case len(s) > maxHTTPBodyBytes:
		return "", false, NewNodeError("FLOW_PARAM_INVALID", "the request body is %d bytes; the limit is %d", len(s), maxHTTPBodyBytes)
	case !utf8.ValidString(s):
		return "", false, NewNodeError("FLOW_PARAM_INVALID", "the request body is not valid text")
	}
	return s, true, nil
}

// scrubAnswer removes the secret from a tool answer, in place (the answer was parsed
// for this call and is owned by it). The tool's own control fields, status and
// status_code, are left as they are: they are not data from the server, and a short
// secret must not be able to turn "success" into something else.
func scrubAnswer(scrub *secretScrubber, out map[string]any) map[string]any {
	if out == nil || scrub == nil || len(scrub.forms) == 0 {
		return out
	}
	control := map[string]any{}
	for _, key := range []string{"status", "status_code"} {
		if v, ok := out[key]; ok {
			control[key] = v
			delete(out, key)
		}
	}
	scrub.value(out)
	for key, v := range control {
		out[key] = v
	}
	return out
}

// httpStatusCode reads the HTTP status of an api_request answer; ok only for a whole
// number from 100 to 599.
func httpStatusCode(out map[string]any) (int, bool) {
	f, ok := toNumber(out["status_code"])
	if !ok || f != math.Trunc(f) || f < 100 || f > 599 {
		return 0, false
	}
	return int(f), true
}

// nestingWithin reports whether v nests at most limit levels of lists and objects.
func nestingWithin(v any, limit int) bool {
	if limit < 0 {
		return false
	}
	switch x := v.(type) {
	case map[string]any:
		for _, item := range x {
			if !nestingWithin(item, limit-1) {
				return false
			}
		}
	case []any:
		for _, item := range x {
			if !nestingWithin(item, limit-1) {
				return false
			}
		}
	}
	return true
}

// httpRequestEffects: a GET has no outward effect; every other method, and one that
// is not known yet (a template, a value of the wrong type), may change something on
// the server. It is a hook, so it copes with a nil node and any parameter.
func httpRequestEffects(n *Node) []Effect {
	if n == nil {
		return nil
	}
	switch v := n.Params["method"].(type) {
	case nil:
		return nil
	case string:
		if s := strings.TrimSpace(v); s == "" || strings.EqualFold(s, "GET") {
			return nil
		}
	}
	return []Effect{EffectSendsMessage}
}

// httpRequestDef defines http.request.
//
// Secrets: auth_secret names a vault entry whose value goes into the credential
// header (see withAuthSecret). The value is scrubbed from everything the node
// returns, errors included, because a server can echo the request headers.
// api_request follows up to 10 redirects. Once a hop changes the host, the scheme or
// the effective port (only http:80 to https:443 on the same host keeps them), it drops
// every header the caller set except Accept, Content-Type and User-Agent, so neither
// Authorization nor a custom auth_header credential reaches another host
// (apiRequestClient in internal/tools/api_client.go).
//
// Errors: an HTTP error status is not a tool failure. The tool reports a status of
// 400 or more as status "error" together with status_code, and that becomes
// FLOW_HTTP_STATUS (or the output, with fail_on_error explicitly off: see failsOnError;
// anything it cannot read keeps the default, which is to fail); below 400, a redirect
// that was not followed included, is a success with ok true. Everything else the tool
// reports, a refused address (SSRF protection), a request that could not be made, a
// denial or plain-text refusal, is a tool failure: FLOW_TOOL_ERROR, FLOW_TOOL_DENIED
// or FLOW_NODE_UNAVAILABLE, never an HTTP status.
//
// The address, the headers and the body are sensitive sinks, and so is auth_secret,
// because untrusted data that picks the vault entry makes the node send it
// wherever the address points.
func httpRequestDef(env CatalogEnv) *NodeDef {
	def := actionDef(TypeHTTPRequest, "web", "api", "api_request", env)
	def.UntrustedOutput = true
	def.PrimaryInput = "url"
	methods := []Option{{Value: "GET", Label: "GET"}, {Value: "POST", Label: "POST"}, {Value: "PUT", Label: "PUT"},
		{Value: "PATCH", Label: "PATCH"}, {Value: "DELETE", Label: "DELETE"}}
	def.Params = []ParamSpec{
		{Name: "method", Kind: ParamSelect, LabelKey: "easydrag.param.http_method", Default: "GET", Options: methods},
		{Name: "url", Kind: ParamText, LabelKey: "easydrag.param.url", Required: true, Templatable: true, SensitiveSink: true},
		{Name: "headers", Kind: ParamKeyValue, LabelKey: "easydrag.param.http_headers", Templatable: true, SensitiveSink: true},
		{Name: "body", Kind: ParamTextarea, LabelKey: "easydrag.param.http_body", Templatable: true, SensitiveSink: true,
			VisibleIf: &Visibility{Param: "method", Equals: []string{"POST", "PUT", "PATCH", "DELETE"}}},
		{Name: "auth_secret", Kind: ParamSecretRef, LabelKey: "easydrag.param.auth_secret", HelpKey: "easydrag.help.auth_secret", SensitiveSink: true},
		{Name: "auth_header", Kind: ParamText, LabelKey: "easydrag.param.auth_header", Default: "Authorization"},
		{Name: "auth_prefix", Kind: ParamText, LabelKey: "easydrag.param.auth_prefix", Default: "Bearer "},
		{Name: "fail_on_error", Kind: ParamBool, LabelKey: "easydrag.param.fail_on_error", Default: true},
	}
	def.OutputFields = []FieldSpec{{Name: "body", Type: "text", Primary: true}, {Name: "json", Type: "any"},
		{Name: "status_code", Type: "number"}, {Name: "ok", Type: "bool"}, {Name: "headers", Type: "object"}}
	def.EffectsFunc = httpRequestEffects
	def.Validate = func(n *Node, _ ValidateContext) []Issue {
		if n == nil {
			return nil
		}
		// Only what Execute would reject whatever the run brings: a template is judged at
		// run time, an absent url is the required-parameter check's business.
		issues := append(webURLIssue(n), choiceIssue(n, "method", "GET", httpMethods)...)
		if _, err := parseHeaders(n.Params["headers"], true); err != nil {
			issues = append(issues, paramIssue(n, IssueParamInvalid, SeverityError, "headers", asNodeError(err).Message))
		}
		// auth_header and auth_prefix only matter, and are only checked by Execute, when a
		// secret is set.
		if !isEmptyValue(n.Params["auth_secret"]) {
			issues = append(issues, literalIssue(n, "auth_header", authHeaderName)...)
			issues = append(issues, literalIssue(n, "auth_prefix", authPrefixValue)...)
		}
		// Execute does not reject a fail_on_error it cannot read, it keeps the default
		// (fail); the publish check is where the author hears about it.
		if v := n.Params["fail_on_error"]; v != nil && !isTemplateText(v) {
			if _, known := flagValue(v); !known {
				issues = append(issues, paramIssue(n, IssueParamInvalid, SeverityError, "fail_on_error", "fail_on_error must be true or false"))
			}
		}
		return issues
	}
	def.Execute = func(ctx context.Context, in ExecInput) (ExecResult, error) {
		method, ok := choiceParam(in.Params["method"], "GET", httpMethods...)
		if !ok {
			return ExecResult{}, choiceError("method", httpMethods)
		}
		target, err := webURLParam(in.Params["url"])
		if err != nil {
			return ExecResult{}, err
		}
		headers, err := parseHeaders(in.Params["headers"], false)
		if err != nil {
			return ExecResult{}, err
		}
		body, hasBody, err := requestBody(method, in.Params["body"])
		if err != nil {
			return ExecResult{}, err
		}
		headers, scrub, err := withAuthSecret(in, headers)
		if err != nil {
			return ExecResult{}, err
		}
		args := map[string]any{"method": method, "url": target}
		if len(headers) > 0 {
			sent := make(map[string]any, len(headers))
			for _, h := range headers {
				sent[h.name] = h.value
			}
			args["headers"] = sent
		}
		if hasBody {
			args["body"] = body
		}
		out, err := callTool(ctx, in, "api_request", args)
		// The answer is scrubbed before anything is read from it, so no message or
		// output built below can carry the secret. The body is a text that the tool
		// may have cut at apiBodyLimit, in the middle of an echoed secret.
		rawBody := Stringify(out["body"])
		bodyText := scrub.text(rawBody, len(rawBody) >= apiBodyLimit)
		out = scrubAnswer(scrub, out)
		code, hasCode := httpStatusCode(out)
		if err != nil {
			// Only an error status of the server's own answer is an HTTP status. The
			// tool sets status_code from the response and nowhere else.
			isStatus := asNodeError(err).Code == "FLOW_TOOL_ERROR" && hasCode && code >= 400 &&
				strings.EqualFold(strings.TrimSpace(outString(out, "status")), "error")
			if !isStatus {
				return ExecResult{}, scrub.err(err)
			}
		} else {
			if err := requireSuccess(out, "HTTP tool"); err != nil {
				return ExecResult{}, err
			}
			if !hasCode {
				return ExecResult{}, NewNodeError("FLOW_TOOL_ERROR", "the HTTP tool did not report a status code")
			}
		}
		if code >= 400 && failsOnError(in.Params["fail_on_error"]) {
			return ExecResult{}, NewNodeError("FLOW_HTTP_STATUS", "the server answered with HTTP %d", code)
		}
		respHeaders, _ := out["headers"].(map[string]any)
		if respHeaders == nil {
			respHeaders = map[string]any{}
		}
		result := map[string]any{"status_code": float64(code), "ok": code < 400, "headers": respHeaders, "body": bodyText}
		// json is parsed from the raw body and scrubbed once, keys included, which also
		// finds a secret the text scrub cannot see (written with \u escapes). It is only
		// offered when the scrubbed text is still JSON: a secret in the place of a number
		// or a literal breaks it, and the number must not be carried over. A secret that
		// the JSON syntax splits (it holds quotes and commas) is not found string by
		// string, so the scrubbed value is encoded and dropped if it still carries one.
		var parsed any
		if json.Unmarshal([]byte(rawBody), &parsed) == nil && (bodyText == rawBody || json.Valid([]byte(bodyText))) && nestingWithin(parsed, maxJSONDepth) {
			if scrubbed := scrub.valueKeys(parsed); !scrub.holdsSecret(scrubbed) {
				result["json"] = scrubbed
			}
		}
		return ExecResult{Output: result}, nil
	}
	return def
}
