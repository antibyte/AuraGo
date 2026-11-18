package flows

import (
	"fmt"
	"strings"
	"testing"
)

func queryAuthParams(url string) map[string]any {
	return map[string]any{"url": url, "auth_secret": "api_token", "auth_in": "query", "auth_query": "api_key"}
}

// With auth_in=query the secret is a query parameter of the first request: an existing value of
// that name is replaced (whatever it was), other parameters and the fragment stay, and no
// credential header is added.
func TestHTTPQueryAuthSetsAndReplacesTheParameter(t *testing.T) {
	secrets := fakeSecrets{"api_token": "s3cr3t-token-value"}
	for name, c := range map[string]struct{ url, want string }{
		"set":      {"https://api.example.com/v1?q=1", "q=1&api_key=s3cr3t-token-value"},
		"replace":  {"https://api.example.com/v1?api_key=typed&q=1#top", "q=1&api_key=s3cr3t-token-value"},
		"no query": {"https://api.example.com/v1", "api_key=s3cr3t-token-value"},
	} {
		_, tools, err := runHTTP(t, queryAuthParams(c.url), answerWith(apiAnswer(200, "ok")), secrets)
		if err != nil || tools.count() != 1 {
			t.Fatalf("%s: err = %v after %d calls", name, err, tools.count())
		}
		call := tools.calls[0]
		if call.VaultQuery != c.want {
			t.Errorf("%s: Vault query = %q, want %q", name, call.VaultQuery, c.want)
		}
		if call.Args["url"] != c.url {
			t.Errorf("%s: tool URL = %v, want %s", name, call.Args["url"], c.url)
		}
		if _, has := call.Args["headers"]; has {
			t.Errorf("%s: a header was added: %v", name, call.Args["headers"])
		}
	}
}

// The secret is registered with the scrubber when it is read, so an answer that echoes the
// request address (the body, or a final address) shows the marker, not the secret.
func TestHTTPQueryAuthScrubsTheSecretFromTheOutput(t *testing.T) {
	echo := func(req ToolRequest) (ToolResponse, error) {
		return apiAnswer(200, "request query: "+req.VaultQuery), nil
	}
	res, _, err := runHTTP(t, queryAuthParams("https://api.example.com/v1?q=1"), echo, fakeSecrets{"api_token": "s3cr3t-token-value"})
	if err != nil {
		t.Fatal(err)
	}
	body := Stringify(res.Output["body"])
	if strings.Contains(body, "s3cr3t-token-value") || !strings.Contains(body, "api_key="+redactedText) {
		t.Fatalf("body = %q, want the marker in the place of the secret", body)
	}
}

func TestHTTPQueryAuthScrubsTheSecretFromDispatchErrors(t *testing.T) {
	fail := func(req ToolRequest) (ToolResponse, error) {
		return ToolResponse{}, fmt.Errorf("request failed for %s?%s", req.Args["url"], req.VaultQuery)
	}
	_, _, err := runHTTP(t, queryAuthParams("https://api.example.com/v1"), fail,
		fakeSecrets{"api_token": "North & West"})
	if err == nil || !strings.Contains(err.Error(), "api_key="+redactedText) || strings.Contains(err.Error(), "North") {
		t.Fatalf("expected the dispatch error with a redacted URL, got %v", err)
	}
}

func TestHTTPQueryAuthNeedsAWrittenOutQueryName(t *testing.T) {
	def := httpNodeDef(t)
	hasError := func(params map[string]any, param string) bool {
		for _, is := range def.Validate(&Node{ID: testNodeID(1), Type: TypeHTTPRequest, Params: params}, ValidateContext{Mode: ModePublish}) {
			if is.Param == param && is.Severity == SeverityError {
				return true
			}
		}
		return false
	}
	base := func(extra map[string]any) map[string]any {
		p := map[string]any{"url": "https://a.example/x", "auth_secret": "k"}
		for k, v := range extra {
			p[k] = v
		}
		return p
	}
	if !hasError(base(map[string]any{"auth_in": "query"}), "auth_query") {
		t.Error("auth_in=query without auth_query is not refused")
	}
	if !hasError(base(map[string]any{"auth_in": "query", "auth_query": "{{trigger.data.k}}"}), "auth_query") {
		t.Error("a templated auth_query is not refused")
	}
	if !hasError(base(map[string]any{"auth_in": "cookie"}), "auth_in") {
		t.Error("an unknown auth_in is not refused")
	}
	if hasError(base(map[string]any{"auth_in": "query", "auth_query": "api_key"}), "auth_query") {
		t.Error("a valid query auth was refused")
	}
}
