package flows

import "testing"

// R2 (audit 2026-10-25): only data that can reach the scheme, the user info, the host or the
// port of the url decides where an http.request's vault secret goes. A literal start up to
// "scheme://host" followed by "/", "?" or "#" fixes the target; data behind it, and data in
// the headers, keeps the plain untrusted-data warning.
func TestB3SecretTargetFixedReadsTheLiteralStart(t *testing.T) {
	for _, c := range []struct {
		url  any
		want bool
	}{
		{"https://crm.example/api?email={{trigger.data.from}}", true},
		{"https://crm.example?email={{trigger.data.from}}", true},
		{"https://crm.example#{{trigger.data.from}}", true},
		{"  HTTPS://crm.example/v1/{{trigger.data.id}}", true},
		{"http://user@crm.example/{{trigger.data.id}}", true},
		{`https://api.example.com\{{x}}/{{trigger.data.id}}`, true},
		{"https://api.example.com/v1", true},
		{"https://api.example.com{{trigger.data.x}}", false},
		{"https://api.example.com{{trigger.data.x}}/v1", false},
		{"https://{{trigger.data.host}}/v1", false},
		{"{{trigger.data.link}}", false},
		{"https://host:{{trigger.data.port}}/", false},
		{"https://user{{trigger.data.x}}@host/", false},
		{`\{{x}}https://evil.example/{{trigger.data.y}}`, false},
		{"https:///{{trigger.data.x}}", false},
		{"ftp://host/{{trigger.data.x}}", false},
		{"https://host/{{trigger.data.x", false},
		{"", false},
		{5.0, false},
		{nil, false},
	} {
		if got := secretTargetFixed(c.url); got != c.want {
			t.Errorf("secretTargetFixed(%#v) = %v, want %v", c.url, got, c.want)
		}
	}
}

func TestB3SecretTargetOnlyWhereDataCanMoveTheHost(t *testing.T) {
	reg := catalogRegistry(t, fullEnv())
	for _, c := range []struct {
		name   string
		params map[string]any
		target string // the param with UNTRUSTED_SECRET_TARGET, "" for none
		warned string // the param with the plain warning, "" when not checked
	}{
		{"data in the query", map[string]any{"url": "https://crm.example/api/contacts?email={{trigger.data.from}}", "auth_secret": "partner_token"}, "", "url"},
		{"data in the path", map[string]any{"url": "https://crm.example/api/{{trigger.data.from}}", "auth_secret": "partner_token"}, "", "url"},
		{"data in a header", map[string]any{"url": "https://crm.example/api", "headers": map[string]any{"X-Request-Id": "{{trigger.data.id}}"}, "auth_secret": "partner_token"}, "", "headers"},
		{"headers as a whole", map[string]any{"url": "https://crm.example/api", "headers": "{{trigger.data.h}}", "auth_secret": "partner_token"}, "", "headers"},
		{"data glued to the host", map[string]any{"url": "https://api.example.com{{trigger.data.x}}", "auth_secret": "partner_token"}, "url", ""},
		{"data as the host", map[string]any{"url": "https://{{trigger.data.host}}/v1", "auth_secret": "partner_token"}, "url", ""},
		{"data as the port", map[string]any{"url": "https://crm.example:{{trigger.data.port}}/v1", "auth_secret": "partner_token"}, "url", ""},
		{"data in the user info", map[string]any{"url": "https://u{{trigger.data.x}}@crm.example/v1", "auth_secret": "partner_token"}, "url", ""},
	} {
		f, call := secretCallFlow(c.params)
		issues := Validate(f, reg, publishCtx())
		target := findIssue(issues, IssueUntrustedSecretTarget, call)
		warning := findIssue(issues, IssueUntrustedData, call)
		switch {
		case c.target != "" && (target == nil || target.Param != c.target || target.Severity != SeverityError):
			t.Errorf("%s: secret target issue = %+v, want an error on %s", c.name, target, c.target)
		case c.target == "" && target != nil:
			t.Errorf("%s: secret target issue %+v, want none", c.name, target)
		case c.warned != "" && (warning == nil || warning.Param != c.warned || warning.Severity != SeverityWarning):
			t.Errorf("%s: warning = %+v, want the plain warning on %s", c.name, warning, c.warned)
		}
		if c.target == "" && HasErrors(issues) {
			t.Errorf("%s: errors %+v, want the flow publishable", c.name, issues)
		}
		// Run time reads the same lint (secretTargetRefusals), also for an old live revision.
		if refused := secretTargetRefusals(f, reg)[call] != nil; refused != (c.target != "") {
			t.Errorf("%s: refused at run time = %v, want %v", c.name, refused, c.target != "")
		}
	}
}
