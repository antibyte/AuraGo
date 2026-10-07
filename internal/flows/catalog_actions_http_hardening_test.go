package flows

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"net/url"
	"reflect"
	"regexp"
	"sort"
	"strings"
	"testing"
	"unicode/utf16"
	"unicode/utf8"
)

// apiAnswer is an api_request answer as the tool writes it: status "success" for a
// status below 400 and "error" with the status code for the rest.
func apiAnswer(status int, body string) ToolResponse {
	ans := map[string]any{"status": "success", "status_code": float64(status), "headers": map[string]any{"content-type": "text/plain"}, "body": body}
	if status >= 400 {
		ans["status"] = "error"
	}
	raw, _ := json.Marshal(ans)
	return ToolResponse{Output: string(raw), Status: "success"}
}

// toolErrorAnswer is the tool's own failure (no HTTP status): {"status":"error","message":…}.
func toolErrorAnswer(message string) ToolResponse {
	raw, _ := json.Marshal(map[string]any{"status": "error", "message": message})
	return ToolResponse{Output: string(raw), Status: "success"}
}

func answerWith(resp ToolResponse) func(ToolRequest) (ToolResponse, error) {
	return func(ToolRequest) (ToolResponse, error) { return resp, nil }
}

func httpNodeDef(t *testing.T) *NodeDef { return lookupDef(t, webRegistry(t, nil), TypeHTTPRequest) }

// runHTTP executes http.request against a fake tool and a vault.
func runHTTP(t *testing.T, params map[string]any, respond func(ToolRequest) (ToolResponse, error), secrets fakeSecrets) (ExecResult, *fakeTools, error) {
	t.Helper()
	tools := &fakeTools{respond: respond}
	res, err := execDef(httpNodeDef(t), params, &Services{Tools: tools, Secrets: secrets})
	return res, tools, err
}

func wantParamInvalid(t *testing.T, label string, tools *fakeTools, err error, forbidden ...string) {
	t.Helper()
	ne := asNodeError(err)
	if ne == nil || ne.Code != "FLOW_PARAM_INVALID" || tools.count() != 0 {
		t.Errorf("%s: error = %v after %d calls, want FLOW_PARAM_INVALID before any call", label, err, tools.count())
		return
	}
	if len(ne.Message) > 200 || !utf8.ValidString(ne.Message) || strings.ContainsAny(ne.Message, "\r\n\x00") {
		t.Errorf("%s: message is not short and clean: %.250q", label, ne.Message)
	}
	for _, f := range forbidden {
		if strings.Contains(ne.Message, f) {
			t.Errorf("%s: message echoes %q: %q", label, f, ne.Message)
		}
	}
}

// secretMarker is in every test secret, so that a leak shows whatever form it has.
const secretMarker = "9f3a7c"

var testSecrets = []string{
	"tok_" + secretMarker + "1e5b",
	`qu"ote\` + secretMarker + `<&>`,
	"p&q " + secretMarker + "+s/t=é",
	"ab/cd+ef==" + secretMarker,
	"it's " + secretMarker + "üñ😀",
}

var (
	percentEscape = regexp.MustCompile(`%[0-9A-F]{2}`)
	phpHTML       = strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;", `"`, "&quot;", "'", "&#039;")
)

// jsonEcho writes value as the string content of a JSON document, the way the
// common encoders do: Go escapes <, > and & (escapeHTML); PHP writes "/" as "\/"
// (phpSlash); Python and PHP write non-ASCII characters as \uXXXX (ascii), Java in
// upper case hex (upper).
func jsonEcho(value string, escapeHTML, phpSlash, ascii, upper bool) string {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(escapeHTML)
	_ = enc.Encode(value)
	s := strings.TrimSuffix(buf.String(), "\n")
	s = s[1 : len(s)-1]
	if phpSlash {
		s = strings.ReplaceAll(s, "/", `\/`)
	}
	if !ascii {
		return s
	}
	var b strings.Builder
	for _, r := range s {
		if r < 0x80 {
			b.WriteRune(r)
			continue
		}
		for _, u := range utf16.Encode([]rune{r}) {
			if upper {
				fmt.Fprintf(&b, `\u%04X`, u)
			} else {
				fmt.Fprintf(&b, `\u%04x`, u)
			}
		}
	}
	return b.String()
}

// echoForms are the ways a server writes a header value back into a response.
func echoForms(value string) map[string]string {
	query := url.QueryEscape(value)
	lower := percentEscape.ReplaceAllStringFunc(query, strings.ToLower)
	return map[string]string{
		"as it is":         value,
		"go json":          jsonEcho(value, true, false, false, false),
		"js json":          jsonEcho(value, false, false, false, false),
		"php slash":        jsonEcho(value, false, true, false, false),
		"python ascii":     jsonEcho(value, false, false, true, false),
		"php ascii":        jsonEcho(value, false, true, true, false),
		"upper hex":        jsonEcho(value, false, false, true, true),
		"go html":          html.EscapeString(value),
		"php html":         phpHTML.Replace(value),
		"query":            query,
		"lower case query": lower,
		"lower case %20":   strings.ReplaceAll(lower, "+", "%20"),
		"path":             url.PathEscape(value),
		"lower case path":  percentEscape.ReplaceAllStringFunc(url.PathEscape(value), strings.ToLower),
	}
}

// echoServer answers like a debug endpoint that repeats the request headers: as
// they are, and the credential once in every encoding of echoForms.
func echoServer(status int) func(ToolRequest) (ToolResponse, error) {
	return func(req ToolRequest) (ToolResponse, error) {
		headers, _ := req.Args["headers"].(map[string]any)
		var raw strings.Builder
		for name, v := range headers {
			fmt.Fprintf(&raw, "%s: %v\n", name, v)
		}
		forms := echoForms(Stringify(headers["Authorization"]))
		names := make([]string, 0, len(forms))
		for name := range forms {
			names = append(names, name)
		}
		sort.Strings(names)
		for _, name := range names {
			fmt.Fprintf(&raw, "<%s> %s\n", name, forms[name])
		}
		return apiAnswer(status, raw.String()), nil
	}
}

func noMarker(t *testing.T, label string, v any) {
	t.Helper()
	raw, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("%s: %v", label, err)
	}
	if strings.Contains(string(raw), secretMarker) {
		t.Errorf("%s leaks the secret: %.300s", label, raw)
	}
}

// The secret goes into the request, and nowhere else: not into the output, whatever
// form a server echoes it in, and not into the parameters.
func TestHTTPSecretIsNotInTheOutput(t *testing.T) {
	for _, secret := range testSecrets {
		params := map[string]any{"url": "https://api", "auth_secret": "tok", "method": "POST", "body": map[string]any{"a": 1.0}}
		res, tools, err := runHTTP(t, params, echoServer(200), fakeSecrets{"tok": secret})
		if err != nil {
			t.Fatalf("%q: %v", secret, err)
		}
		if sent := tools.last(t).Args["headers"].(map[string]any)["Authorization"]; sent != "Bearer "+secret {
			t.Fatalf("the request carries %q, want the secret", sent)
		}
		noMarker(t, "output", res.Output)
		body, _ := res.Output["body"].(string)
		if !strings.Contains(body, redactedText) || !strings.Contains(body, "Authorization: Bearer "+redactedText) {
			t.Errorf("body = %q, want the secret replaced", body)
		}
		// One line per encoding of the echo: each must be scrubbed, and the lines show which not.
		for _, line := range strings.Split(body, "\n") {
			if strings.Contains(line, secretMarker) {
				t.Errorf("secret %q is still in the body: %.200q", secret, line)
			}
		}
		if got := strings.Count(body, "\n<"); got != len(echoForms("x")) {
			t.Errorf("the body holds %d encodings, want %d", got, len(echoForms("x")))
		}
		if _, ok := res.Output["json"]; ok {
			t.Errorf("an echo that is not JSON has no json field: %#v", res.Output["json"])
		}
		noMarker(t, "parameters", params)
		if params["auth_secret"] != "tok" {
			t.Errorf("auth_secret = %v, want the secret's name", params["auth_secret"])
		}
	}

	// A JSON echo is parsed: the parsed values are scrubbed as well, also where the
	// server wrote the secret with unicode escapes the text scrub does not know.
	for _, secret := range testSecrets {
		escaped := ""
		for _, r := range secret {
			for _, u := range utf16.Encode([]rune{r}) {
				escaped += fmt.Sprintf(`\u%04x`, u)
			}
		}
		respond := answerWith(apiAnswer(200, `{"echo":"`+escaped+`","list":["`+escaped+`"],"n":1}`))
		res, _, err := runHTTP(t, map[string]any{"url": "https://api", "auth_secret": "tok"}, respond, fakeSecrets{"tok": secret})
		if err != nil {
			t.Fatal(err)
		}
		parsed, _ := res.Output["json"].(map[string]any)
		if parsed == nil || parsed["n"] != 1.0 {
			t.Fatalf("json = %#v", res.Output["json"])
		}
		noMarker(t, "parsed json", res.Output["json"])
		if parsed["echo"] != redactedText || !reflect.DeepEqual(parsed["list"], []any{redactedText}) {
			t.Errorf("json echo = %#v, want the secret replaced", parsed)
		}
	}

	// A secret that stands where a JSON number is: the text scrub breaks the JSON,
	// so no json field carries the number.
	res, _, err := runHTTP(t, map[string]any{"url": "https://api", "auth_secret": "pin", "auth_prefix": ""},
		answerWith(apiAnswer(200, `{"pin":8675309}`)), fakeSecrets{"pin": "8675309"})
	if _, has := res.Output["json"]; err != nil || has || res.Output["body"] != `{"pin":[redacted]}` {
		t.Errorf("numeric secret: %#v, %v", res.Output, err)
	}
}

// Errors carry text of the tool and of the server. It is scrubbed before it is cut
// and quoted, so the secret does not survive in a message in any way.
func TestHTTPSecretIsNotInErrors(t *testing.T) {
	secret := testSecrets[0]
	echoed := "Authorization: Bearer " + secret
	long := strings.Repeat("x", 292) // the message is cut after 300 runes: 292 + "tok_9f3a"
	cases := []struct {
		name    string
		respond func(ToolRequest) (ToolResponse, error)
		params  map[string]any
		code    string
	}{
		{"tool error", answerWith(ToolResponse{Output: `{"status":"error","message":"Request failed: ` + echoed + ` was rejected"}`, Status: "success"}), nil, "FLOW_TOOL_ERROR"},
		{"cut tool error", answerWith(ToolResponse{Output: `{"status":"error","message":"` + long + secret + ` and more"}`, Status: "success"}), nil, "FLOW_TOOL_ERROR"},
		{"error field", answerWith(ToolResponse{Output: `{"status":"error","error":"` + echoed + `"}`, IsError: true, Status: "failed"}), nil, "FLOW_TOOL_ERROR"},
		{"plain refusal", answerWith(ToolResponse{Output: "[PERMISSION DENIED] api_request refused " + echoed, Status: "success"}), nil, "FLOW_TOOL_ERROR"},
		{"cut plain refusal", answerWith(ToolResponse{Output: strings.Repeat("x", 30) + secret, Status: "success"}), nil, "FLOW_TOOL_ERROR"},
		{"denied", answerWith(ToolResponse{Output: `{"status":"error","message":"` + echoed + `"}`, Status: "denied"}), nil, "FLOW_TOOL_DENIED"},
		{"needs setup", answerWith(ToolResponse{Output: `{"message":"` + echoed + `"}`, Status: "needs_setup"}), nil, "FLOW_NODE_UNAVAILABLE"},
		{"invoker error", func(ToolRequest) (ToolResponse, error) { return ToolResponse{}, errors.New("dial failed: " + echoed) }, nil, "FLOW_NODE_FAILED"},
		{"cut invoker error", func(ToolRequest) (ToolResponse, error) { return ToolResponse{}, errors.New(long + secret) }, nil, "FLOW_NODE_FAILED"},
		{"coded invoker error", func(ToolRequest) (ToolResponse, error) {
			return ToolResponse{}, NewNodeError("FLOW_TOOL_DENIED", "%s", echoed)
		}, nil, "FLOW_TOOL_DENIED"},
		{"status error with an echo", echoServer(500), nil, "FLOW_HTTP_STATUS"},
	}
	for _, c := range cases {
		params := map[string]any{"url": "https://api", "auth_secret": "tok"}
		res, tools, err := runHTTP(t, params, c.respond, fakeSecrets{"tok": secret})
		ne := asNodeError(err)
		if ne == nil || ne.Code != c.code {
			t.Errorf("%s: error = %v, want %s", c.name, err, c.code)
			continue
		}
		if tools.count() != 1 {
			t.Errorf("%s: %d tool calls", c.name, tools.count())
		}
		if strings.Contains(err.Error(), "9f3a") || strings.Contains(ne.Message, "tok_9f") || len(res.Output) != 0 {
			t.Errorf("%s: the error leaks the secret or its start: %.320q", c.name, ne.Message)
		}
	}
	// The cut that follows the secret does not hide it either: its start was visible.
	_, _, err := runHTTP(t, map[string]any{"url": "https://api", "auth_secret": "tok"},
		answerWith(ToolResponse{Output: `{"status":"error","message":"` + long + secret + `"}`, Status: "success"}), fakeSecrets{"tok": secret})
	if msg := asNodeError(err).Message; !strings.HasSuffix(msg, "…") || strings.Contains(msg, "tok_") || len([]rune(msg)) > maxToolMessageRunes+1 {
		t.Errorf("cut message = %q", msg)
	}
	// An error message that does not hold the secret stays as it is, however it ends.
	_, _, err = runHTTP(t, map[string]any{"url": "https://api", "auth_secret": "tok"},
		answerWith(ToolResponse{Output: `{"status":"error","message":"no such host: not-t"}`, Status: "success"}), fakeSecrets{"tok": secret})
	if msg := asNodeError(err).Message; msg != "no such host: not-t" {
		t.Errorf("clean message = %q", msg)
	}

	// A response body cut at the tool's size limit can end in the middle of the secret.
	filler := strings.Repeat("y", apiBodyLimit-8)
	res, _, err := runHTTP(t, map[string]any{"url": "https://api", "auth_secret": "tok"}, answerWith(apiAnswer(200, filler+secret[:8])), fakeSecrets{"tok": secret})
	body, _ := res.Output["body"].(string)
	if err != nil || strings.Contains(body, "tok_9f3a") || !strings.HasPrefix(body, filler) || strings.Contains(body, "tok_") || !strings.HasSuffix(body, "y") {
		t.Errorf("cut body: %v, tail %q", err, body[len(body)-30:])
	}
	// A body that ends like the start of the secret by chance, but was not cut, is kept.
	res, _, err = runHTTP(t, map[string]any{"url": "https://api", "auth_secret": "tok"}, answerWith(apiAnswer(200, "a bit of t")), fakeSecrets{"tok": secret})
	if err != nil || res.Output["body"] != "a bit of t" {
		t.Errorf("short body = %#v, %v", res.Output["body"], err)
	}

	// A cut in the middle of a multi-byte character of the secret leaves U+FFFD at the
	// end of the body (the tool's answer is JSON encoded), which hides the start of the
	// secret from a plain suffix check. The same goes for an error message.
	multi := "tok_" + secretMarker + "é1e5b"
	head := "tok_" + secretMarker + "\xC3" // é is C3 A9
	res, _, err = runHTTP(t, map[string]any{"url": "https://api", "auth_secret": "tok"},
		answerWith(apiAnswer(200, strings.Repeat("y", apiBodyLimit-len(head))+head)), fakeSecrets{"tok": multi})
	body, _ = res.Output["body"].(string)
	if err != nil || strings.Contains(body, "tok_") || strings.Contains(body, string(utf8.RuneError)) || !strings.HasSuffix(body, "y") {
		t.Errorf("body cut inside a character: %v, tail %q", err, body[len(body)-20:])
	}
	_, _, err = runHTTP(t, map[string]any{"url": "https://api", "auth_secret": "tok"},
		answerWith(toolErrorAnswer(strings.Repeat("x", 280)+" tok_"+secretMarker+"éééééééééééééééééééé")), fakeSecrets{"tok": "tok_" + secretMarker + "ééééééééééééééééééééé"})
	if msg := asNodeError(err).Message; strings.Contains(msg, secretMarker) || strings.Contains(msg, "tok_") || !strings.HasSuffix(msg, "…") {
		t.Errorf("message cut inside a secret with non-ASCII characters: %q", msg[len(msg)-30:])
	}
}

// The secret is removed in one pass over each text: a secret that is part of the
// replacement text itself must not mangle the output, and the parsed body is not
// scrubbed a second time. Keys of the parsed body are scrubbed too.
func TestHTTPSecretScrubIsASinglePass(t *testing.T) {
	run := func(secret, body string) ExecResult {
		res, _, err := runHTTP(t, map[string]any{"url": "https://api", "auth_secret": "k", "auth_prefix": ""}, answerWith(apiAnswer(200, body)), fakeSecrets{"k": secret})
		if err != nil {
			t.Fatalf("%q: %v", secret, err)
		}
		return res
	}
	for _, secret := range []string{"red", "act", "[", "d]", "e"} {
		res := run(secret, `{"a":"`+secret+`","`+secret+`":1}`)
		want := map[string]any{"a": redactedText, redactedText: 1.0}
		if !reflect.DeepEqual(res.Output["json"], want) || res.Output["body"] != `{"a":"[redacted]","[redacted]":1}` {
			t.Errorf("secret %q: body %q, json %#v", secret, res.Output["body"], res.Output["json"])
		}
	}
	// Keys, as the server wrote them: the secret itself, nested.
	secret := testSecrets[0]
	res := run(secret, `{"`+secret+`":1,"n":{"`+secret+`":[3]}}`)
	noMarker(t, "json", res.Output["json"])
	m, _ := res.Output["json"].(map[string]any)
	if nested, _ := m["n"].(map[string]any); m[redactedText] != 1.0 || nested == nil || nested[redactedText] == nil || len(m) != 2 {
		t.Errorf("json = %#v", res.Output["json"])
	}
	// A key written with a \u escape for one character: the text scrub does not see
	// that, the parsed key is the secret and is scrubbed (the body text keeps the
	// escape, which is not a form the scrubber knows).
	escaped := string(rune(92)) + "u0074ok_" + secretMarker + "1e5b"
	res = run(secret, `{"`+escaped+`":2,"n":1}`)
	noMarker(t, "json with an escaped key", res.Output["json"])
	if want := map[string]any{redactedText: 2.0, "n": 1.0}; !reflect.DeepEqual(res.Output["json"], want) {
		t.Errorf("json with an escaped key = %#v, want %#v", res.Output["json"], want)
	}
}

// Through the engine, as a flow runs it: the run result (steps, parameters, outputs,
// the error) never holds the secret, on success and on failure.
func TestHTTPSecretIsNotInTheRunRecord(t *testing.T) {
	secret := testSecrets[1]
	for name, respond := range map[string]func(ToolRequest) (ToolResponse, error){
		"echo":  echoServer(200),
		"error": answerWith(toolErrorAnswer("rejected " + secret)),
		"500":   echoServer(500),
	} {
		reg := newTestRegistry(t)
		if err := registerWebNodes(reg, nil); err != nil {
			t.Fatal(err)
		}
		b := newFlow("Secrets")
		tr := b.node("start", "test.trigger", nil)
		call := b.node("call", TypeHTTPRequest, map[string]any{"url": "https://api", "auth_secret": "tok", "headers": map[string]any{"X-A": "1"}})
		after := b.node("after", "test.echo", map[string]any{"value": "{{call.body}}"})
		b.edge(tr, PortOut, call)
		b.edge(call, PortOut, after)
		tools := &fakeTools{respond: respond}
		eng := newTestEngine(reg, &Services{Tools: tools, Secrets: fakeSecrets{"tok": secret}}, 2)
		res, _ := runWith(context.Background(), eng, b.build(), RunRequest{TriggerNode: tr})
		if name == "echo" && res.Status != RunSuccess {
			t.Fatalf("%s: status %s (%s)", name, res.Status, res.ErrorMessage)
		}
		if name != "echo" && res.Status == RunSuccess {
			t.Fatalf("%s: the run succeeded", name)
		}
		if tools.count() != 1 {
			t.Fatalf("%s: %d tool calls", name, tools.count())
		}
		noMarker(t, name+": steps", res.Steps)
		noMarker(t, name+": outputs", res.Outputs)
		noMarker(t, name+": error", res.ErrorMessage)
		step := stepOf(res, call)
		if step == nil || step.Params["auth_secret"] != "tok" {
			t.Fatalf("%s: step = %+v", name, step)
		}
		if name == "echo" {
			if got := res.Outputs["after"]["value"]; !strings.Contains(fmt.Sprint(got), redactedText) {
				t.Errorf("%s: downstream value = %v", name, got)
			}
		}
	}
}

// Whichever of the two is given, the vault secret decides the credential header:
// it is the deliberate setting, and a header that came from data (a template) must
// not displace it. Names compare without regard to case, and a request never
// carries two spellings of one header.
func TestHTTPAuthSecretWins(t *testing.T) {
	vault := fakeSecrets{"tok": "s3cret", "padded": " \tpadded-secret\r\n", "empty": " \n", "multi": "a\nb", "nul": "a\x00b"}
	run := func(params map[string]any) (map[string]any, error) {
		params["url"] = "https://api"
		_, tools, err := runHTTP(t, params, answerWith(apiAnswer(200, "")), vault)
		if err != nil {
			if tools.count() != 0 {
				t.Errorf("%v: made %d calls", params, tools.count())
			}
			return nil, err
		}
		headers, _ := tools.last(t).Args["headers"].(map[string]any)
		return headers, nil
	}
	for _, c := range []struct {
		name   string
		params map[string]any
		want   map[string]any
	}{
		{"user header only", map[string]any{"headers": map[string]any{"Authorization": "Basic abc"}}, map[string]any{"Authorization": "Basic abc"}},
		{"secret only", map[string]any{"auth_secret": "tok"}, map[string]any{"Authorization": "Bearer s3cret"}},
		{"same spelling", map[string]any{"auth_secret": "tok", "headers": map[string]any{"Authorization": "Basic abc"}}, map[string]any{"Authorization": "Bearer s3cret"}},
		{"lower case", map[string]any{"auth_secret": "tok", "headers": map[string]any{"authorization": "Basic abc", "X-A": "1"}}, map[string]any{"Authorization": "Bearer s3cret", "X-A": "1"}},
		{"upper case", map[string]any{"auth_secret": "tok", "headers": map[string]any{"AUTHORIZATION": "x"}}, map[string]any{"Authorization": "Bearer s3cret"}},
		{"custom header", map[string]any{"auth_secret": "tok", "auth_header": " X-Api-Key ", "auth_prefix": "", "headers": map[string]any{"x-api-key": "old", "Authorization": "Basic abc"}},
			map[string]any{"X-Api-Key": "s3cret", "Authorization": "Basic abc"}},
		{"blank auth header", map[string]any{"auth_secret": "tok", "auth_header": "  "}, map[string]any{"Authorization": "Bearer s3cret"}},
		{"custom prefix", map[string]any{"auth_secret": "tok", "auth_prefix": "Token "}, map[string]any{"Authorization": "Token s3cret"}},
		{"null prefix", map[string]any{"auth_secret": "tok", "auth_prefix": nil}, map[string]any{"Authorization": "s3cret"}},
		{"padded secret is trimmed", map[string]any{"auth_secret": "padded"}, map[string]any{"Authorization": "Bearer padded-secret"}},
		{"blank secret name", map[string]any{"auth_secret": "  ", "headers": map[string]any{"X-A": "1"}}, map[string]any{"X-A": "1"}},
		{"null secret name", map[string]any{"auth_secret": nil}, nil},
	} {
		got, err := run(c.params)
		if err != nil || !reflect.DeepEqual(got, c.want) {
			t.Errorf("%s: headers = %#v (%v), want %#v", c.name, got, err, c.want)
		}
	}
	for name, c := range map[string]struct {
		params map[string]any
		code   string
	}{
		"two spellings":      {map[string]any{"headers": map[string]any{"X-A": "1", "x-a": "2"}}, "FLOW_PARAM_INVALID"},
		"empty secret":       {map[string]any{"auth_secret": "empty"}, "FLOW_SECRET_UNAVAILABLE"},
		"line break":         {map[string]any{"auth_secret": "multi"}, "FLOW_SECRET_UNAVAILABLE"},
		"NUL":                {map[string]any{"auth_secret": "nul"}, "FLOW_SECRET_UNAVAILABLE"},
		"unknown secret":     {map[string]any{"auth_secret": "nope"}, "FLOW_SECRET_UNAVAILABLE"},
		"invalid auth name":  {map[string]any{"auth_secret": "tok", "auth_header": "Bad Name"}, "FLOW_PARAM_INVALID"},
		"CRLF in auth name":  {map[string]any{"auth_secret": "tok", "auth_header": "X\r\nEvil: 1"}, "FLOW_PARAM_INVALID"},
		"CRLF in the prefix": {map[string]any{"auth_secret": "tok", "auth_prefix": "Bearer\r\nEvil: 1\r\n"}, "FLOW_PARAM_INVALID"},
		"list secret name":   {map[string]any{"auth_secret": []any{"tok"}}, "FLOW_PARAM_INVALID"},
		"huge secret name":   {map[string]any{"auth_secret": strings.Repeat("k", 300)}, "FLOW_PARAM_INVALID"},
		"object auth name":   {map[string]any{"auth_secret": "tok", "auth_header": map[string]any{}}, "FLOW_PARAM_INVALID"},
	} {
		_, err := run(c.params)
		ne := asNodeError(err)
		if ne == nil || ne.Code != c.code {
			t.Errorf("%s: error = %v, want %s", name, err, c.code)
			continue
		}
		if strings.Contains(ne.Message, "Evil") || strings.Contains(ne.Message, "a\nb") || len(ne.Message) > 200 {
			t.Errorf("%s: message = %q", name, ne.Message)
		}
	}
	// Without a vault the node says so.
	tools := &fakeTools{}
	if _, err := execDef(httpNodeDef(t), map[string]any{"url": "https://api", "auth_secret": "tok"}, &Services{Tools: tools}); asNodeError(err).Code != "FLOW_SECRET_UNAVAILABLE" || tools.count() != 0 {
		t.Errorf("no vault: %v after %d calls", err, tools.count())
	}
	if _, err := execDef(httpNodeDef(t), map[string]any{"url": "https://api", "auth_secret": "tok"}, nil); asNodeError(err).Code != "FLOW_SECRET_UNAVAILABLE" {
		t.Errorf("no services: %v", err)
	}
}

// A header name must be a token and a value must not hold a line break or any
// other control character, or one header could carry another (or a second
// request). The message names neither the value nor an invalid name.
func TestHTTPHeaderInjectionIsRejected(t *testing.T) {
	bad := map[string]map[string]any{
		"CRLF in a value":         {"X-A": "ok\r\nX-Evil: 1"},
		"LF in a value":           {"X-A": "ok\nX-Evil: 1"},
		"CR in a value":           {"X-A": "ok\rX-Evil: 1"},
		"NUL in a value":          {"X-A": "ok\x00X-Evil"},
		"control character":       {"X-A": "ok\x01X-Evil"},
		"DEL":                     {"X-A": "ok\x7fX-Evil"},
		"invalid UTF-8":           {"X-A": "ok\xffX-Evil"},
		"CRLF in a name":          {"X-A\r\nX-Evil: 1": "v"},
		"LF in a name":            {"X-A\nX-Evil": "v"},
		"NUL in a name":           {"X-A\x00Evil": "v"},
		"space in a name":         {"X Evil": "v"},
		"colon in a name":         {"X-Evil: 1": "v"},
		"non-ASCII name":          {"X-Evil-é": "v"},
		"quote in a name":         {`X"Evil`: "v"},
		"slash in a name":         {"X/Evil": "v"},
		"tab in a name":           {"X\tEvil": "v"},
		"a value without a name":  {"": "X-Evil"},
		"blank name with a value": {"  ": "X-Evil"},
		"object value":            {"X-A": map[string]any{"Evil": 1.0}},
		"list value":              {"X-A": []any{"X-Evil"}},
		"null value":              {"X-A": nil},
		"two spellings":           {"X-A": "1", "x-a": "2"},
		"long name":               {strings.Repeat("N", maxHeaderNameBytes+1): "X-Evil"},
	}
	for name, headers := range bad {
		_, tools, err := runHTTP(t, map[string]any{"url": "https://api", "headers": headers}, answerWith(apiAnswer(200, "")), nil)
		wantParamInvalid(t, name, tools, err, "Evil")
		node := &Node{ID: testNodeID(1), Params: map[string]any{"url": "https://api", "headers": headers}}
		if issues := httpNodeDef(t).Validate(node, ValidateContext{}); len(issues) != 1 || issues[0].Param != "headers" || issues[0].Severity != SeverityError {
			t.Errorf("%s: Validate = %+v", name, issues)
		}
	}
	for name, headers := range map[string]map[string]any{
		"tab in a value":       {"X-A": "a\tb"},
		"a number":             {"X-A": 3.0},
		"a flag":               {"X-A": true},
		"UTF-8 in a value":     {"X-A": "größe"},
		"all token characters": {"X-Api_Key.v2~!#$%&'*+^`|": "v"},
		"blank row":            {"": "", "  ": nil, "X-A": "1"},
		"empty value":          {"X-A": ""},
		"padded name":          {" X-A ": "1"},
		"max name":             {strings.Repeat("N", maxHeaderNameBytes): "v"},
	} {
		_, tools, err := runHTTP(t, map[string]any{"url": "https://api", "headers": headers}, answerWith(apiAnswer(200, "")), nil)
		if err != nil || tools.count() != 1 {
			t.Errorf("%s: %v after %d calls", name, err, tools.count())
		}
	}
	// Whatever else the headers parameter is, it is not silently dropped.
	for name, v := range map[string]any{"text": "X-A: 1", "list": []any{map[string]any{"name": "X-A", "value": "1"}}, "number": 5.0, "flag": true} {
		_, tools, err := runHTTP(t, map[string]any{"url": "https://api", "headers": v}, answerWith(apiAnswer(200, "")), nil)
		wantParamInvalid(t, "headers as a "+name, tools, err)
	}
	// Null and empty headers mean none.
	for _, v := range []any{nil, map[string]any{}, []any{}, "", "  ", map[string]any(nil)} {
		_, tools, err := runHTTP(t, map[string]any{"url": "https://api", "headers": v}, answerWith(apiAnswer(200, "")), nil)
		if _, has := tools.last(t).Args["headers"]; err != nil || has {
			t.Errorf("headers %#v: %v, args %#v", v, err, tools.last(t).Args)
		}
	}
	// A header that holds a template cannot be judged before the run, but the
	// resolved value is checked: Validate only skips it.
	node := &Node{ID: testNodeID(1), Params: map[string]any{"url": "https://api", "headers": map[string]any{"X-A": "{{trigger.data.v}}", "X B": "{{x.y}}"}}}
	if issues := httpNodeDef(t).Validate(node, ValidateContext{}); len(issues) != 0 {
		t.Errorf("templates: %+v", issues)
	}
	_, tools, err := runHTTP(t, map[string]any{"url": "https://api", "headers": map[string]any{"X-A": "a\r\nX-Evil: 1"}}, nil, nil)
	wantParamInvalid(t, "resolved template", tools, err, "Evil")
}

// The number, the size and the order of the headers are bounded.
func TestHTTPHeaderLimits(t *testing.T) {
	many := func(n int) map[string]any {
		m := make(map[string]any, n)
		for i := 0; i < n; i++ {
			m[fmt.Sprintf("X-H%d", i)] = "v"
		}
		return m
	}
	ok := answerWith(apiAnswer(200, ""))
	if _, _, err := runHTTP(t, map[string]any{"url": "https://api", "headers": many(maxHTTPHeaders)}, ok, nil); err != nil {
		t.Errorf("%d headers: %v", maxHTTPHeaders, err)
	}
	for name, n := range map[string]int{"one over": maxHTTPHeaders + 1, "many": 5000} {
		_, tools, err := runHTTP(t, map[string]any{"url": "https://api", "headers": many(n)}, ok, nil)
		wantParamInvalid(t, name, tools, err)
	}
	// The credential header counts too.
	_, tools, err := runHTTP(t, map[string]any{"url": "https://api", "headers": many(maxHTTPHeaders), "auth_secret": "tok"}, ok, fakeSecrets{"tok": "s"})
	wantParamInvalid(t, "fifty plus the credential", tools, err)
	if _, _, err := runHTTP(t, map[string]any{"url": "https://api", "headers": many(maxHTTPHeaders - 1), "auth_secret": "tok"}, ok, fakeSecrets{"tok": "s"}); err != nil {
		t.Errorf("49 plus the credential: %v", err)
	}
	// Total size: name plus value, all headers.
	fit := map[string]any{"X": strings.Repeat("v", maxHTTPHeaderBytes-1)}
	if _, _, err := runHTTP(t, map[string]any{"url": "https://api", "headers": fit}, ok, nil); err != nil {
		t.Errorf("headers of exactly the limit: %v", err)
	}
	for name, headers := range map[string]map[string]any{
		"one byte over": {"X": strings.Repeat("v", maxHTTPHeaderBytes)},
		"two big":       {"X-A": strings.Repeat("v", maxHTTPHeaderBytes/2), "X-B": strings.Repeat("v", maxHTTPHeaderBytes/2)},
		"huge":          {"X-A": strings.Repeat("v", 4<<20)},
	} {
		_, tools, err := runHTTP(t, map[string]any{"url": "https://api", "headers": headers}, ok, nil)
		wantParamInvalid(t, name, tools, err)
	}
	// The credential pushes the total over the limit.
	_, tools, err = runHTTP(t, map[string]any{"url": "https://api", "headers": map[string]any{"X": strings.Repeat("v", maxHTTPHeaderBytes-100)}, "auth_secret": "tok"},
		ok, fakeSecrets{"tok": strings.Repeat("s", 200)})
	wantParamInvalid(t, "credential over the limit", tools, err, "sssss")
	// The first problem found does not depend on the order of a map.
	first := ""
	for i := 0; i < 20; i++ {
		_, _, err := runHTTP(t, map[string]any{"url": "https://api", "headers": map[string]any{"X-B": "1\n", "X-A": "2\n", "X-C": "3\n"}}, ok, nil)
		if i == 0 {
			first = asNodeError(err).Message
		} else if asNodeError(err).Message != first {
			t.Fatalf("message changes: %q and %q", first, asNodeError(err).Message)
		}
	}
	if !strings.Contains(first, `"X-A"`) {
		t.Errorf("the first header by name is reported: %q", first)
	}
}
