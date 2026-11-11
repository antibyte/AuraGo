[← Plan index](README.md) · Spec: [plans/2026-11-11-easydrag-s0b-spec.md](../2026-11-11-easydrag-s0b-spec.md)

## Part B1: credential rule, query auth, move to the vault, client masking (FD-13, first half)

> **For the implementer.** You know Go and vanilla JS but not this codebase. Every step is one action. Every code block is complete. Run each command exactly as written and compare the output with the stated expectation. If the output differs, stop and read the "if" note of that step.

**Spec:** `plans/2026-11-11-easydrag-s0b-spec.md` §4, §5, §7 (Part B), §8 (B1-B5). Owner decisions are fixed: header and URL query are movable, body fields and other nodes are detected, warned and purged but not moved.

**Worktree.** `C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-s0b-credentials`, branch `feat/easydrag-s0b-credentials`, created from `main` (805c775f7 or later). Part B2 (scan, purge, schema 9) follows in the same worktree, after this part. Never touch the main checkout. Every command starts with `cd <worktree> && ...`.

**Shell.** Git Bash. Go tests of `internal/flows` use `GOTMPDIR='D:\tmp\go-build'`; tests of `internal/server` and `./ui/` use no `GOTMPDIR`. Commit messages are written with the Write tool into `C:/disposable/easydrag-s0b/` and committed with `git commit -F`.

**Facts that differ from the brief (verified on main).**
- The real route prefix is `/api/desktop/flows/...`, not `/api/flows/...`. The move endpoint is `POST /api/desktop/flows/{id}/credentials/move`. B2's collection routes (`/api/desktop/flows/credentials/scan` and `/purge`) are dispatched next to `secrets` in `handleFlows`; this part only adds the `{id}/credentials/move` route inside `flowRoute`.
- `FLOW_REVISION_CONFLICT` already exists (`flows.ErrRevisionConflict`, mapped in `flowsErrorFrom`). It is reused. Only `FLOW_CREDENTIAL_NOT_MOVABLE` and `FLOW_SECRET_EXISTS` are new. There is no Go registry of FLOW codes: a code is registered by `easydrag.ui.error_<code lowercase>` in the 16 locale files plus the member list `families["easydrag.ui.error_"]` in `ui/desktop_easydrag_test.go`.
- The remembered test-trigger key (`aurago.easydrag.test-trigger.<id>`, `easydrag-run-start.js:92`) holds only a trigger node id. Test data lives on the server (`flow_test_data`, purged by B2). B.5 verifies this and adds no masking there.

**Task order.** B.1, B.2, B.3, **B.5, B.4**. Spec §8 lists B4 (UI) before B5 (masking), but the UI action reads the typed value with `ED.credentials.findings` (B.5), so B.5 must land first. The headings keep the spec numbers.

### Setup (once)

- [ ] **Step 1: create the worktree**

```
cd C:/Users/Andi/Documents/repo/AuraGo && git worktree add .worktrees/easydrag-s0b-credentials -b feat/easydrag-s0b-credentials main
```
Expected: `Preparing worktree (new branch 'feat/easydrag-s0b-credentials')` and `HEAD is now at ...`.

- [ ] **Step 2: baseline (all four must be green before any change)**

```
cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-s0b-credentials && GOTMPDIR='D:\tmp\go-build' go test ./internal/flows/ -count=1
cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-s0b-credentials && go test ./internal/server/ -run '(?i)(Flow|Secret)' -count=1
cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-s0b-credentials && node scripts/test-easydrag.mjs
cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-s0b-credentials && go test ./ui/ -run '(?i)(EasyDrag)' -count=1
```
Expected: `ok  aurago/internal/flows`, `ok  aurago/internal/server`, the Node run ends with `all EasyDrag checks passed`, `ok  aurago/ui`. If a baseline fails, stop and report it: it is not caused by this part.

- [ ] **Step 3: locale helper (not in the repo)**

Write `C:/disposable/easydrag-s0b/lang-add.mjs`. It appends new keys to all 16 files of `ui/lang/easydrag/` and keeps the exact file format (the files round-trip through `JSON.stringify(obj, null, 2) + "\n"`, but their keys are not sorted, so keys are appended at the end):

```js
// node lang-add.mjs <data.mjs> <worktree>   data.mjs: export default { keys: [...], langs: { cs: [...], ... } }
import fs from 'node:fs';
import path from 'node:path';
import { pathToFileURL } from 'node:url';

const data = (await import(pathToFileURL(path.resolve(process.argv[2])).href)).default;
const root = path.resolve(process.argv[3]);
const all = ['cs', 'da', 'de', 'el', 'en', 'es', 'fr', 'hi', 'it', 'ja', 'nl', 'no', 'pl', 'pt', 'sv', 'zh'];
for (const lang of all) {
    const values = data.langs[lang];
    if (!values || values.length !== data.keys.length) throw new Error(lang + ': ' + (values ? values.length : 0) + ' values for ' + data.keys.length + ' keys');
    const file = path.join(root, 'ui/lang/easydrag', lang + '.json');
    const words = JSON.parse(fs.readFileSync(file, 'utf8'));
    data.keys.forEach((key, i) => { delete words[key]; words[key] = values[i]; });
    fs.writeFileSync(file, JSON.stringify(words, null, 2) + '\n');
}
console.log('added ' + data.keys.length + ' keys to ' + all.length + ' files');
```

---

### Task B.1: the detection rule, validation and redaction

**Files**
- Modify: `internal/flows/credentials.go` (rule, redaction), `internal/flows/issue.go` (Issue fields), `internal/flows/validate.go:~392` and `:~172`, `internal/flows/catalog_actions_http.go:449-453`, `internal/flows/engine.go:313`, `internal/flows/durable_coordinator.go:82`, `internal/flows/service_runs.go:372`, `internal/flows/AGENTS.md:316-318`
- Modify tests: `internal/flows/credentials_fb4_test.go`, `internal/flows/credentials_mixed_fb4_test.go`
- Create: `internal/flows/testdata/credential_cases.json`, `internal/flows/credentials_rule_test.go`
- Modify: `ui/js/desktop/apps/easydrag-core.js:399-402` (issue text per location), `ui/desktop_easydrag_test.go` (family members), `ui/lang/easydrag/*.json` (16 files)

**What the rule is.** For every node parameter the rule finds (a) credential headers of `http.request` (the existing `credentialHeaderName` names, template-only values excepted, as today), (b) URL user info `user:pass@` and (c) URL query values whose key is a credential name (`key` counts only here) or whose value is a token, in any string that has an `http(s)://` word, (d) credential-named keys at any depth of JSON body params, of any structured parameter, and of the top level of `tool.*` nodes, (e) known token formats as whole values or whitespace-separated words of at least 16 characters. A value with an unescaped `{{` is never a finding, and neither is the marker. Top-level parameter names of other node types are not checked (they would flag `auth_secret`, which holds a vault name). Redaction and detection share one walker, so they cannot drift.

- [ ] **Step 1: impact analysis (CLAUDE.md rule), then read the results**

```
node C:/Users/Andi/.gitnexus/stores/AuraGo-08496d38b623/checkouts/AuraGo-08496d38b623/run.cjs impact "redactCredentialHeaders" --direction upstream --repo C:/Users/Andi/Documents/repo/AuraGo
node C:/Users/Andi/.gitnexus/stores/AuraGo-08496d38b623/checkouts/AuraGo-08496d38b623/run.cjs impact "literalCredentialHeaders" --direction upstream --repo C:/Users/Andi/Documents/repo/AuraGo
node C:/Users/Andi/.gitnexus/stores/AuraGo-08496d38b623/checkouts/AuraGo-08496d38b623/run.cjs impact "RedactCredentials" --direction upstream --repo C:/Users/Andi/Documents/repo/AuraGo
```
Expected callers: `redactCredentialHeaders`: `engine.go`, `durable_coordinator.go`, `service_runs.go`, `RedactCredentials`; `literalCredentialHeaders`: `catalog_actions_http.go` and `credentials_mixed_fb4_test.go`; `RedactCredentials`: `ExportFlow`, `local_pack.go`, `child_history*.go`, `service_runs.go`. Warn the owner on HIGH or CRITICAL. UNKNOWN is not an all-clear: confirm with `cd <worktree> && grep -rn "literalCredentialHeaders\|redactCredentialHeaders" internal --include=*.go`; the list must match the one above. Every call site is changed in this task.

- [ ] **Step 2: write the shared vectors**

Create `internal/flows/testdata/credential_cases.json` (24 cases; one per line for review; the Go and Node tests both read it; fake tokens only):

```json
[
{"name":"header_bearer_literal","node":{"id":"n1","type":"http.request","params":{"url":"https://api.example.com/v1","headers":{"Authorization":"Bearer abc123def456","Accept":"text/plain"}}},"findings":[{"param":"headers","location":"header","path":"Authorization","movable":true}],"masked_params":{"url":"https://api.example.com/v1","headers":{"Authorization":"[redacted]","Accept":"text/plain"}}},
{"name":"header_template_only","node":{"id":"n1","type":"http.request","params":{"url":"https://api.example.com/v1","headers":{"Authorization":"Bearer {{secret_ref.api}}","X-Auth-Token":"{{trigger.data.token}}"}}},"findings":[],"masked_params":{"url":"https://api.example.com/v1","headers":{"Authorization":"Bearer {{secret_ref.api}}","X-Auth-Token":"{{trigger.data.token}}"}}},
{"name":"header_mixed_template_is_typed_text","node":{"id":"n1","type":"http.request","params":{"url":"https://api.example.com/v1","headers":{"Cookie":"sid=abc123; lang={{trigger.data.lang}}"}}},"findings":[{"param":"headers","location":"header","path":"Cookie","movable":false}],"masked_params":{"url":"https://api.example.com/v1","headers":{"Cookie":"[redacted]"}}},
{"name":"header_escaped_brace_is_literal","node":{"id":"n1","type":"http.request","params":{"url":"https://api.example.com/v1","headers":{"Authorization":"Bearer \\{{abc"}}},"findings":[{"param":"headers","location":"header","path":"Authorization","movable":true}],"masked_params":{"url":"https://api.example.com/v1","headers":{"Authorization":"[redacted]"}}},
{"name":"header_but_slot_taken_is_not_movable","node":{"id":"n1","type":"http.request","params":{"url":"https://api.example.com/v1","auth_secret":"vault_name","headers":{"X-Api-Key":"abcdef123456"}}},"findings":[{"param":"headers","location":"header","path":"X-Api-Key","movable":false}],"masked_params":{"url":"https://api.example.com/v1","auth_secret":"vault_name","headers":{"X-Api-Key":"[redacted]"}}},
{"name":"url_userinfo","node":{"id":"n1","type":"http.request","params":{"url":"https://admin:hunter2@example.com/path"}},"findings":[{"param":"url","location":"url_userinfo","path":"userinfo","movable":true}],"masked_params":{"url":"https://[redacted]@example.com/path"}},
{"name":"url_query_key","node":{"id":"n1","type":"http.request","params":{"url":"https://api.example.com/v1?api_key=abc123&q=hello"}},"findings":[{"param":"url","location":"url_query","path":"api_key","movable":true}],"masked_params":{"url":"https://api.example.com/v1?api_key=[redacted]&q=hello"}},
{"name":"url_query_key_exception_only_in_queries","node":{"id":"n1","type":"http.request","params":{"url":"https://api.example.com/v1?key=AbC123xyz"}},"findings":[{"param":"url","location":"url_query","path":"key","movable":true}],"masked_params":{"url":"https://api.example.com/v1?key=[redacted]"}},
{"name":"url_mixed_template_and_literal","node":{"id":"n1","type":"http.request","params":{"url":"https://api.example.com/v1?token={{trigger.data.t}}&sig=abcdef123456#top"}},"findings":[{"param":"url","location":"url_query","path":"sig","movable":true}],"masked_params":{"url":"https://api.example.com/v1?token={{trigger.data.t}}&sig=[redacted]#top"}},
{"name":"url_template_value","node":{"id":"n1","type":"http.request","params":{"url":"https://api.example.com/v1?api_key={{secret_ref.k}}&q=1"}},"findings":[],"masked_params":{"url":"https://api.example.com/v1?api_key={{secret_ref.k}}&q=1"}},
{"name":"url_encoded_key","node":{"id":"n1","type":"http.request","params":{"url":"https://api.example.com/v1?access%5Ftoken=abc123xyz789"}},"findings":[{"param":"url","location":"url_query","path":"access_token","movable":true}],"masked_params":{"url":"https://api.example.com/v1?access%5Ftoken=[redacted]"}},
{"name":"body_key_is_not_a_credential","node":{"id":"n1","type":"http.request","params":{"url":"https://api.example.com/v1","method":"POST","body":{"key":"harmless","name":"x"}}},"findings":[],"masked_params":{"url":"https://api.example.com/v1","method":"POST","body":{"key":"harmless","name":"x"}}},
{"name":"body_nested_password","node":{"id":"n1","type":"http.request","params":{"url":"https://api.example.com/v1","method":"POST","body":{"user":"a","auth":{"password":"hunter2hunter2"}}}},"findings":[{"param":"body","location":"body_field","path":"/auth/password","movable":false}],"masked_params":{"url":"https://api.example.com/v1","method":"POST","body":{"user":"a","auth":{"password":"[redacted]"}}}},
{"name":"body_json_text","node":{"id":"n1","type":"http.request","params":{"url":"https://api.example.com/v1","method":"POST","body":"{\"client_secret\":\"s3cr3t-value\",\"n\":1}"}},"findings":[{"param":"body","location":"body_field","path":"/client_secret","movable":false}],"masked_params":{"url":"https://api.example.com/v1","method":"POST","body":"{\"client_secret\":\"[redacted]\",\"n\":1}"}},
{"name":"tool_node_top_level_field","node":{"id":"n1","type":"tool.example","params":{"api_key":"abcd1234efgh","path":"/tmp/x"}},"findings":[{"param":"api_key","location":"param_field","path":"/api_key","movable":false}],"masked_params":{"api_key":"[redacted]","path":"/tmp/x"}},
{"name":"other_node_nested_field","node":{"id":"n1","type":"logic.set","params":{"fields":[{"name":"x","value":"y"}],"vars":{"password":"abcd1234abcd"}}},"findings":[{"param":"vars","location":"param_field","path":"/password","movable":false}],"masked_params":{"fields":[{"name":"x","value":"y"}],"vars":{"password":"[redacted]"}}},
{"name":"top_level_names_of_other_nodes_are_not_fields","node":{"id":"n1","type":"notify.telegram","params":{"auth_secret":"vault_name","token":"abcd1234abcd"}},"findings":[],"masked_params":{"auth_secret":"vault_name","token":"abcd1234abcd"}},
{"name":"token_formats","node":{"id":"n1","type":"ai.step","params":{"a_openai":"sk-abcdefghijklmnopqrstuvwx","b_github":"ghp_abcdefghijklmnopqrst","c_github_pat":"github_pat_11AAAAAAA0abcdefghij","d_gitlab":"glpat-abcdefghijklmnopqrst","e_slack":"xoxb-1234567890-abcdefghij","f_aws":"AKIAIOSFODNN7EXAMPLE","g_google":"AIza0123456789abcdefghijklmnopqrstuvwxy","h_jwt":"eyJhbGciOiJIUzI1NiJ9.eyJzdWIiOiJ0ZXN0In0.c2lnbmF0dXJl"}},"findings":[{"param":"a_openai","location":"format","path":"","movable":false},{"param":"b_github","location":"format","path":"","movable":false},{"param":"c_github_pat","location":"format","path":"","movable":false},{"param":"d_gitlab","location":"format","path":"","movable":false},{"param":"e_slack","location":"format","path":"","movable":false},{"param":"f_aws","location":"format","path":"","movable":false},{"param":"g_google","location":"format","path":"","movable":false},{"param":"h_jwt","location":"format","path":"","movable":false}],"masked_params":{"a_openai":"[redacted]","b_github":"[redacted]","c_github_pat":"[redacted]","d_gitlab":"[redacted]","e_slack":"[redacted]","f_aws":"[redacted]","g_google":"[redacted]","h_jwt":"[redacted]"}},
{"name":"token_in_a_sentence","node":{"id":"n1","type":"ai.step","params":{"prompt":"Call the API with sk-abcdefghijklmnopqrstuvwx and report back"}},"findings":[{"param":"prompt","location":"format","path":"","movable":false}],"masked_params":{"prompt":"Call the API with [redacted] and report back"}},
{"name":"short_prefix_is_not_a_token","node":{"id":"n1","type":"ai.step","params":{"note":"sk-short","other":"ghp_tooShort"}},"findings":[],"masked_params":{"note":"sk-short","other":"ghp_tooShort"}},
{"name":"url_in_a_sentence","node":{"id":"n1","type":"ai.step","params":{"prompt":"fetch https://u:p4ss@host.example/x now"}},"findings":[{"param":"prompt","location":"url_userinfo","path":"userinfo","movable":false}],"masked_params":{"prompt":"fetch https://[redacted]@host.example/x now"}},
{"name":"url_in_a_list","node":{"id":"n1","type":"logic.set","params":{"urls":["https://h.example/x?token=abc12345"]}},"findings":[{"param":"urls","location":"url_query","path":"/0?token","movable":false}],"masked_params":{"urls":["https://h.example/x?token=[redacted]"]}},
{"name":"harmless_request","node":{"id":"n1","type":"http.request","params":{"method":"GET","url":"https://example.com/search?q=cats&page=2","headers":{"Accept":"application/json","X-Request-Id":"r1"},"body":{"name":"x","count":2}}},"findings":[],"masked_params":{"method":"GET","url":"https://example.com/search?q=cats&page=2","headers":{"Accept":"application/json","X-Request-Id":"r1"},"body":{"name":"x","count":2}}},
{"name":"marker_is_not_a_finding","node":{"id":"n1","type":"http.request","params":{"url":"https://api.example.com/v1?api_key=[redacted]","headers":{"Authorization":"[redacted]"}}},"findings":[],"masked_params":{"url":"https://api.example.com/v1?api_key=[redacted]","headers":{"Authorization":"[redacted]"}}}
]
```

- [ ] **Step 3: write the failing Go tests**

Create `internal/flows/credentials_rule_test.go`:

```go
package flows

import (
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"strings"
	"testing"
)

type credentialCase struct {
	Name string `json:"name"`
	Node struct {
		ID     string         `json:"id"`
		Type   string         `json:"type"`
		Params map[string]any `json:"params"`
	} `json:"node"`
	Findings []struct {
		Param    string `json:"param"`
		Location string `json:"location"`
		Path     string `json:"path"`
		Movable  bool   `json:"movable"`
	} `json:"findings"`
	MaskedParams map[string]any `json:"masked_params"`
}

func loadCredentialCases(t *testing.T) []credentialCase {
	t.Helper()
	raw, err := os.ReadFile("testdata/credential_cases.json")
	if err != nil {
		t.Fatal(err)
	}
	var cases []credentialCase
	if err := json.Unmarshal(raw, &cases); err != nil {
		t.Fatal(err)
	}
	if len(cases) < 18 {
		t.Fatalf("only %d shared vectors; the rule needs at least 18", len(cases))
	}
	return cases
}

// The vectors are shared with the client rule (scripts/test-easydrag-credentials.mjs).
func TestCredentialRuleSharedVectors(t *testing.T) {
	for _, c := range loadCredentialCases(t) {
		t.Run(c.Name, func(t *testing.T) {
			n := Node{ID: c.Node.ID, Type: c.Node.Type, Params: c.Node.Params}
			var got, want []string
			for _, f := range FindLiteralCredentials(&Flow{Nodes: []Node{n}}) {
				if f.NodeID != c.Node.ID || f.Value == "" {
					t.Errorf("finding %+v lacks its node or value", f)
				}
				got = append(got, fmt.Sprintf("%s|%s|%s|%t", f.Param, f.Location, f.Path, f.Movable))
			}
			for _, f := range c.Findings {
				want = append(want, fmt.Sprintf("%s|%s|%s|%t", f.Param, f.Location, f.Path, f.Movable))
			}
			if !reflect.DeepEqual(got, want) {
				t.Errorf("findings = %v, want %v", got, want)
			}
			if masked := redactCredentialParams(n.Type, n.Params, true); !reflect.DeepEqual(masked, c.MaskedParams) {
				t.Errorf("masked params = %v, want %v", masked, c.MaskedParams)
			}
		})
	}
}

func TestCredentialRuleOnAnyJSONValue(t *testing.T) {
	v := map[string]any{"a": []any{map[string]any{"password": "hunter2hunter2", "url": "https://h.example/x?token=abc12345&q=1"}}, "note": "ok"}
	if got, want := literalCredentialsInValue(v), []string{"hunter2hunter2", "abc12345"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("literals = %v, want %v", got, want)
	}
	red := redactCredentialValue(v).(map[string]any)["a"].([]any)[0].(map[string]any)
	if red["password"] != CredentialRedactionMarker || red["url"] != "https://h.example/x?token="+CredentialRedactionMarker+"&q=1" {
		t.Fatalf("redacted = %v", red)
	}
	if v["a"].([]any)[0].(map[string]any)["password"] != "hunter2hunter2" {
		t.Fatal("the input value must not change")
	}
	if raw, _ := json.Marshal(CredentialFinding{NodeID: "n", Value: "hunter2hunter2"}); strings.Contains(string(raw), "hunter2") {
		t.Fatalf("a finding serialises its value: %s", raw)
	}
}

// credentialIssues runs the real validator on a flow with one http.request node.
func credentialIssues(t *testing.T, params map[string]any) []Issue {
	t.Helper()
	b := newFlow("Creds")
	b.node("call", TypeHTTPRequest, params)
	var out []Issue
	for _, is := range Validate(b.build(), webRegistry(t, nil), ValidateContext{Mode: ModePublish}) {
		if is.Code == IssueLiteralCredential {
			out = append(out, is)
		}
	}
	return out
}

func TestCredentialRuleValidationWarnsOncePerFinding(t *testing.T) {
	issues := credentialIssues(t, map[string]any{
		"url":     "https://u:p4ss@api.example.com/x?api_key=abc123&sig=def456",
		"headers": map[string]any{"Authorization": "Bearer tok-1234567890"},
	})
	want := []struct {
		param, location, path string
		movable               bool
	}{{"headers", "header", "Authorization", true}, {"url", "url_userinfo", "userinfo", true}, {"url", "url_query", "api_key", true}, {"url", "url_query", "sig", true}}
	if len(issues) != len(want) {
		t.Fatalf("issues = %+v, want %d warnings", issues, len(want))
	}
	for i, w := range want {
		is := issues[i]
		if is.Severity != SeverityWarning || is.Param != w.param || is.Location != w.location || is.Path != w.path || is.Movable != w.movable || is.Message == "" {
			t.Errorf("issue %d = %+v, want %+v", i, is, w)
		}
	}
}

func TestStoredStepParamsAndExportsRedactTheWholeRule(t *testing.T) {
	b := newFlow("Rule")
	tr := b.node("start", "test.trigger", nil)
	call := b.node("call", "test.echo", map[string]any{"value": "x", "url": "https://h.example/x?api_key=abc123&q=1",
		"payload": map[string]any{"password": "hunter2hunter2", "ok": "yes"}})
	b.edge(tr, PortOut, call)
	f := b.build()
	check := func(what string, p map[string]any) {
		t.Helper()
		payload, _ := p["payload"].(map[string]any)
		if p["url"] != "https://h.example/x?api_key="+CredentialRedactionMarker+"&q=1" || payload["password"] != CredentialRedactionMarker || payload["ok"] != "yes" {
			t.Errorf("%s keeps a credential: %v", what, p)
		}
	}
	out, err := RedactCredentials(f)
	if err != nil {
		t.Fatal(err)
	}
	check("export", out.NodeByID(call).Params)
	if f.NodeByID(call).Params["url"] != "https://h.example/x?api_key=abc123&q=1" {
		t.Fatal("the export must not change the draft")
	}
	res, _ := runWith(t.Context(), newTestEngine(newTestRegistry(t), nil, 4), f, RunRequest{TriggerNode: tr})
	check("stored step params", stepOf(res, call).Params)
}
```

- [ ] **Step 4: run them and watch them fail**

```
cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-s0b-credentials && GOTMPDIR='D:\tmp\go-build' go test ./internal/flows/ -run 'TestCredentialRule|TestStoredStepParams' -count=1
```
Expected: `FAIL` with build errors `undefined: FindLiteralCredentials`, `undefined: redactCredentialParams`, `Location undefined (type Issue ...)`. If it reports something else (for example `t.Context` undefined), the Go version is older than 1.24: use `context.Background()` and import `context`.

- [ ] **Step 5: implement the rule in `credentials.go`**

First the imports. Replace the import block at the top with:

```go
import (
	"bytes"
	"encoding/json"
	"maps"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"

	"aurago/internal/security"
)
```

`sort` stays imported until step 7 (the old `literalCredentialHeaders` uses it). The code below uses the existing helper `sortedKeys(m map[string]any) []string` (`template_eval.go:380`, sorted keys of a map); do not define it again.

Then add this code after `templateOnlyCredential` (keep `literalCredentialHeaders` and `redactCredentialHeaders` for now; step 7 removes the first):

```go
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
```

In `RedactCredentials` (same file) replace the line `c.Nodes[i].Params = redactCredentialHeaders(c.Nodes[i].Params, true)` with:

```go
		c.Nodes[i].Params = redactCredentialParams(c.Nodes[i].Type, c.Nodes[i].Params, true)
```
Also change its doc comment sentence "whose header maps hold redactedText instead of typed-in credentials (S6)" to "in which every typed-in credential of the credential rule is replaced by CredentialRedactionMarker (S6)".

- [ ] **Step 6: add the Issue fields and the validator hook (still compiles with the old callers)**

In `internal/flows/issue.go`, extend `Issue`:

```go
type Issue struct {
	Code     string   `json:"code"`
	Severity Severity `json:"severity"`
	NodeID   string   `json:"node_id,omitempty"`
	EdgeID   string   `json:"edge_id,omitempty"`
	Param    string   `json:"param,omitempty"`
	// Location and Path say where a PARAM_LITERAL_CREDENTIAL sits (CredentialFinding); Movable
	// tells the editor whether "move to the vault" is offered.
	Location string `json:"location,omitempty"`
	Path     string `json:"path,omitempty"`
	Movable  bool   `json:"movable,omitempty"`
	Message  string `json:"message"`
}
```
Update the comment of `IssueLiteralCredential` to: `// IssueLiteralCredential warns about a credential typed into a node (header, URL user info or query, a body or parameter field, a token format): it sits in clear text in the draft, its versions and the run log.`

In `internal/flows/validate.go`, in `publishRules`, directly after `v.outputPorts(n, def)` insert:

```go
		v.credentialIssues(n)
```
and in `finish`, next to `is.Param = truncateRunes(is.Param, maxIssueParamRunes)`, add:

```go
		is.Path = truncateRunes(is.Path, maxIssueParamRunes)
```

- [ ] **Step 7: replace the header-only check and the old callers**

1. `internal/flows/catalog_actions_http.go`: in `Validate`, the `else` branch around lines 448-453 becomes only the error case. Replace

```go
		if _, err := parseHeaders(n.Params["headers"], true); err != nil {
			issues = append(issues, paramIssue(n, IssueParamInvalid, SeverityError, "headers", asNodeError(err).Message))
		} else {
			for _, name := range literalCredentialHeaders(n.Params["headers"]) {
				issues = append(issues, paramIssue(n, IssueLiteralCredential, SeverityWarning, "headers",
					"the header "+quoteForError(name)+" holds a credential typed in clear text, kept in the flow and its versions; store it as a secret and use auth_secret"))
			}
		}
```
with

```go
		if _, err := parseHeaders(n.Params["headers"], true); err != nil {
			issues = append(issues, paramIssue(n, IssueParamInvalid, SeverityError, "headers", asNodeError(err).Message))
		}
```
(the typed-in credential warnings now come from the central `credentialIssues`).

2. `internal/flows/credentials.go`: delete the function `literalCredentialHeaders` (the block from its doc comment to its closing brace) and remove `"sort"` from the import block (nothing else in the file uses it).
3. `internal/flows/engine.go:313`: replace `storedParams(redactCredentialHeaders(params, false))` with `storedParams(redactCredentialParams(n.Type, params, false))`.
4. `internal/flows/durable_coordinator.go:82`: replace `storedParams(redactCredentialHeaders(params, false))` with `storedParams(redactCredentialParams(n.Type, params, false))`.
5. `internal/flows/service_runs.go:372`: replace `steps[i].Params = redactCredentialHeaders(steps[i].Params, false)` with `steps[i].Params = redactCredentialParams("", steps[i].Params, false)` (older rows have no node type here; nested keys are still redacted).
6. Update the three tests that used the old API. In `credentials_fb4_test.go` replace the function `TestFB4LiteralCredentialHeaderIsAPublishWarning` with:

```go
func TestFB4LiteralCredentialHeaderIsAPublishWarning(t *testing.T) {
	issues := credentialIssues(t, map[string]any{"url": "https://api.example/x", "headers": map[string]any{
		"Authorization": "Bearer abc123", "X-Api-Key": "k-123", "Cookie": "sid=1", "X-Request-Id": "r1",
		"X-Auth-Token": "{{trigger.data.token}}", "Proxy-Authorization": ""}})
	if len(issues) != 3 {
		t.Fatalf("issues = %+v, want one warning each for Authorization, Cookie and X-Api-Key", issues)
	}
	for i, want := range []string{"Authorization", "Cookie", "X-Api-Key"} {
		is := issues[i]
		if is.Code != IssueLiteralCredential || is.Severity != SeverityWarning || is.Param != "headers" || is.Location != "header" || is.Path != want || !strings.Contains(is.Message, want) {
			t.Errorf("issue %d = %+v, want a header warning about %s", i, is, want)
		}
	}
}
```
In `credentials_mixed_fb4_test.go` replace `TestFB4MixedCredentialValuesWarnAndAreRedacted` and `TestFB4PureTemplateCredentialValuesStay` with:

```go
func TestFB4MixedCredentialValuesWarnAndAreRedacted(t *testing.T) {
	mixed := map[string]any{
		"Cookie":        "session=abc123; lang={{trigger.data.lang}}",
		"Authorization": "Token sk-live-123 {{trigger.data.suffix}}",
	}
	params := map[string]any{"url": "https://api.example/x", "headers": mixed}
	if issues := credentialIssues(t, params); len(issues) != 2 {
		t.Fatalf("issues = %+v, want a warning each for Authorization and Cookie", issues)
	}
	h := redactCredentialHeaders(params, true)["headers"].(map[string]any)
	if h["Cookie"] != redactedText || h["Authorization"] != redactedText {
		t.Fatalf("export keeps a mixed credential: %v", h)
	}
	if mixed["Cookie"] == redactedText {
		t.Fatal("the input map must not be modified")
	}
}

func TestFB4PureTemplateCredentialValuesStay(t *testing.T) {
	pure := map[string]any{
		"Authorization": "Bearer {{secret_ref.api}}",
		"X-Auth-Token":  "{{trigger.data.token}}",
		"Cookie":        "  {{ trigger.data.cookie }}  ",
		"X-Api-Key":     "basic {{trigger.data.k}}",
	}
	params := map[string]any{"url": "https://api.example/x", "headers": pure}
	if issues := credentialIssues(t, params); len(issues) != 0 {
		t.Fatalf("pure templates warned: %+v", issues)
	}
	h := redactCredentialParams(TypeHTTPRequest, params, true)["headers"].(map[string]any)
	for k, v := range pure {
		if h[k] != v {
			t.Errorf("export changed %s: %v", k, h[k])
		}
	}
	// An escaped brace is typed-in text, not a template.
	if got := credentialIssues(t, map[string]any{"url": "https://api.example/x", "headers": map[string]any{"Authorization": `Bearer \{{abc`}}); len(got) != 1 {
		t.Fatalf("escaped braces = %v, want a warning", got)
	}
}
```
(the rest of `credentials_mixed_fb4_test.go` stays; check that the file still needs its `context` and `testing` imports.)

- [ ] **Step 8: run the whole flows package**

```
cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-s0b-credentials && GOTMPDIR='D:\tmp\go-build' go test ./internal/flows/ -count=1
```
Expected: `ok  aurago/internal/flows`. If a vector fails, read the message: it names the case and both sides. The vectors are derived from the spec rule; fix the code, not the vector, unless the vector contradicts spec §4.2. If another existing test fails because a typed credential now gives more warnings or masked text (search the failure for `PARAM_LITERAL_CREDENTIAL`, `[redacted]` or `Issues`), update that test's expectation: it pinned the old header-only rule. Typical places: `grep -rn "holds a credential typed in clear text\|IssueLiteralCredential" internal --include=*_test.go`.

- [ ] **Step 9: issue texts per location (UI, 16 locales)**

In `ui/js/desktop/apps/easydrag-core.js` replace `issueText` (lines 398-402) with:

```js
    // issueText translates validation issues by code, and by code and location when the issue
    // has one (easydrag.ui.issue_<code>_<location>, {path} filled); unknown codes keep the server text.
    function issueText(t, issue) {
        if (!issue) return '';
        const base = 'easydrag.ui.issue_' + String(issue.code || '').toLowerCase();
        if (issue.location) {
            const key = base + '_' + issue.location;
            const text = t(key, { path: issue.path || '' });
            if (text && text !== key) return text;
        }
        return tr(t, base, issue.message || issue.code || '');
    }
```
In `ui/desktop_easydrag_test.go`, in `families` the member list of `"easydrag.ui.issue_"`, append after `"param_literal_credential"`:

```go
"param_literal_credential_header", "param_literal_credential_url_userinfo", "param_literal_credential_url_query",
			"param_literal_credential_body_field", "param_literal_credential_param_field", "param_literal_credential_format"
```
Write `C:/disposable/easydrag-s0b/lang-b1.mjs` (key order: header, url_userinfo, url_query, body_field, param_field, format):

```js
export default {
    keys: ['easydrag.ui.issue_param_literal_credential_header', 'easydrag.ui.issue_param_literal_credential_url_userinfo', 'easydrag.ui.issue_param_literal_credential_url_query', 'easydrag.ui.issue_param_literal_credential_body_field', 'easydrag.ui.issue_param_literal_credential_param_field', 'easydrag.ui.issue_param_literal_credential_format'],
    langs: {
        en: ['The header {path} holds a password or key typed in clear text. Store it in the vault and use the Secret field.', 'The address holds a user name and password in clear text. Store them in the vault and use the Secret field.', 'The address parameter {path} holds a key typed in clear text. Store it in the vault and use the Secret field.', 'The body field {path} holds a password or key typed in clear text. Remove it from the flow.', 'The field {path} holds a password or key typed in clear text. Remove it from the flow.', 'This value looks like an access token typed in clear text. Remove it from the flow.'],
        de: ['Der Header {path} enthält ein Passwort oder einen Schlüssel im Klartext. Lege ihn im Tresor ab und nutze das Feld „Geheimnis“.', 'Die Adresse enthält Benutzername und Passwort im Klartext. Lege sie im Tresor ab und nutze das Feld „Geheimnis“.', 'Der Adressparameter {path} enthält einen Schlüssel im Klartext. Lege ihn im Tresor ab und nutze das Feld „Geheimnis“.', 'Das Body-Feld {path} enthält ein Passwort oder einen Schlüssel im Klartext. Entferne den Wert aus dem Flow.', 'Das Feld {path} enthält ein Passwort oder einen Schlüssel im Klartext. Entferne den Wert aus dem Flow.', 'Dieser Wert sieht aus wie ein Zugriffstoken im Klartext. Entferne ihn aus dem Flow.'],
        cs: ['Hlavička {path} obsahuje heslo nebo klíč zadaný jako čistý text. Ulož ho do trezoru a použij pole Tajemství.', 'Adresa obsahuje uživatelské jméno a heslo jako čistý text. Ulož je do trezoru a použij pole Tajemství.', 'Parametr adresy {path} obsahuje klíč zadaný jako čistý text. Ulož ho do trezoru a použij pole Tajemství.', 'Pole těla {path} obsahuje heslo nebo klíč zadaný jako čistý text. Odstraň hodnotu z flow.', 'Pole {path} obsahuje heslo nebo klíč zadaný jako čistý text. Odstraň hodnotu z flow.', 'Tato hodnota vypadá jako přístupový token v čistém textu. Odstraň ji z flow.'],
        da: ['Headeren {path} indeholder en adgangskode eller nøgle som almindelig tekst. Gem den i tresoren, og brug feltet Hemmelighed.', 'Adressen indeholder brugernavn og adgangskode som almindelig tekst. Gem dem i tresoren, og brug feltet Hemmelighed.', 'Adresseparameteren {path} indeholder en nøgle som almindelig tekst. Gem den i tresoren, og brug feltet Hemmelighed.', 'Body-feltet {path} indeholder en adgangskode eller nøgle som almindelig tekst. Fjern værdien fra flowet.', 'Feltet {path} indeholder en adgangskode eller nøgle som almindelig tekst. Fjern værdien fra flowet.', 'Værdien ligner et adgangstoken som almindelig tekst. Fjern den fra flowet.'],
        el: ['Η κεφαλίδα {path} περιέχει κωδικό ή κλειδί σε απλό κείμενο. Αποθήκευσέ το στο θησαυροφυλάκιο και χρησιμοποίησε το πεδίο Μυστικό.', 'Η διεύθυνση περιέχει όνομα χρήστη και κωδικό σε απλό κείμενο. Αποθήκευσέ τα στο θησαυροφυλάκιο και χρησιμοποίησε το πεδίο Μυστικό.', 'Η παράμετρος διεύθυνσης {path} περιέχει κλειδί σε απλό κείμενο. Αποθήκευσέ το στο θησαυροφυλάκιο και χρησιμοποίησε το πεδίο Μυστικό.', 'Το πεδίο σώματος {path} περιέχει κωδικό ή κλειδί σε απλό κείμενο. Αφαίρεσε την τιμή από τη ροή.', 'Το πεδίο {path} περιέχει κωδικό ή κλειδί σε απλό κείμενο. Αφαίρεσε την τιμή από τη ροή.', 'Αυτή η τιμή μοιάζει με διακριτικό πρόσβασης σε απλό κείμενο. Αφαίρεσέ την από τη ροή.'],
        es: ['El encabezado {path} contiene una contraseña o clave escrita en texto plano. Guárdala en la bóveda y usa el campo Secreto.', 'La dirección contiene usuario y contraseña en texto plano. Guárdalos en la bóveda y usa el campo Secreto.', 'El parámetro de dirección {path} contiene una clave en texto plano. Guárdala en la bóveda y usa el campo Secreto.', 'El campo del cuerpo {path} contiene una contraseña o clave en texto plano. Quita el valor del flujo.', 'El campo {path} contiene una contraseña o clave en texto plano. Quita el valor del flujo.', 'Este valor parece un token de acceso en texto plano. Quítalo del flujo.'],
        fr: ['L’en-tête {path} contient un mot de passe ou une clé en clair. Range-le dans le coffre et utilise le champ Secret.', 'L’adresse contient un nom d’utilisateur et un mot de passe en clair. Range-les dans le coffre et utilise le champ Secret.', 'Le paramètre d’adresse {path} contient une clé en clair. Range-la dans le coffre et utilise le champ Secret.', 'Le champ du corps {path} contient un mot de passe ou une clé en clair. Retire la valeur du flux.', 'Le champ {path} contient un mot de passe ou une clé en clair. Retire la valeur du flux.', 'Cette valeur ressemble à un jeton d’accès en clair. Retire-la du flux.'],
        hi: ['हेडर {path} में पासवर्ड या कुंजी सादे पाठ में लिखी है। उसे तिजोरी में रखो और गुप्त फ़ील्ड का उपयोग करो।', 'पते में उपयोगकर्ता नाम और पासवर्ड सादे पाठ में हैं। उन्हें तिजोरी में रखो और गुप्त फ़ील्ड का उपयोग करो।', 'पते के पैरामीटर {path} में कुंजी सादे पाठ में लिखी है। उसे तिजोरी में रखो और गुप्त फ़ील्ड का उपयोग करो।', 'बॉडी फ़ील्ड {path} में पासवर्ड या कुंजी सादे पाठ में लिखी है। इस मान को फ़्लो से हटाओ।', 'फ़ील्ड {path} में पासवर्ड या कुंजी सादे पाठ में लिखी है। इस मान को फ़्लो से हटाओ।', 'यह मान सादे पाठ में लिखे एक्सेस टोकन जैसा दिखता है। इसे फ़्लो से हटाओ।'],
        it: ['L’header {path} contiene una password o una chiave in chiaro. Salvala nel caveau e usa il campo Segreto.', 'L’indirizzo contiene nome utente e password in chiaro. Salvali nel caveau e usa il campo Segreto.', 'Il parametro dell’indirizzo {path} contiene una chiave in chiaro. Salvala nel caveau e usa il campo Segreto.', 'Il campo del body {path} contiene una password o una chiave in chiaro. Rimuovi il valore dal flusso.', 'Il campo {path} contiene una password o una chiave in chiaro. Rimuovi il valore dal flusso.', 'Questo valore sembra un token di accesso in chiaro. Rimuovilo dal flusso.'],
        ja: ['ヘッダー {path} にパスワードまたはキーが平文で入力されています。保管庫に保存し、「シークレット」欄を使ってください。', 'アドレスにユーザー名とパスワードが平文で含まれています。保管庫に保存し、「シークレット」欄を使ってください。', 'アドレスのパラメーター {path} にキーが平文で入力されています。保管庫に保存し、「シークレット」欄を使ってください。', '本文のフィールド {path} にパスワードまたはキーが平文で入力されています。値をフローから削除してください。', 'フィールド {path} にパスワードまたはキーが平文で入力されています。値をフローから削除してください。', 'この値はアクセストークンのように見えます（平文）。フローから削除してください。'],
        nl: ['De header {path} bevat een wachtwoord of sleutel in platte tekst. Bewaar die in de kluis en gebruik het veld Geheim.', 'Het adres bevat een gebruikersnaam en wachtwoord in platte tekst. Bewaar ze in de kluis en gebruik het veld Geheim.', 'De adresparameter {path} bevat een sleutel in platte tekst. Bewaar die in de kluis en gebruik het veld Geheim.', 'Het body-veld {path} bevat een wachtwoord of sleutel in platte tekst. Verwijder de waarde uit de flow.', 'Het veld {path} bevat een wachtwoord of sleutel in platte tekst. Verwijder de waarde uit de flow.', 'Deze waarde lijkt op een toegangstoken in platte tekst. Verwijder hem uit de flow.'],
        no: ['Headeren {path} inneholder et passord eller en nøkkel i klartekst. Lagre den i hvelvet og bruk feltet Hemmelighet.', 'Adressen inneholder brukernavn og passord i klartekst. Lagre dem i hvelvet og bruk feltet Hemmelighet.', 'Adresseparameteren {path} inneholder en nøkkel i klartekst. Lagre den i hvelvet og bruk feltet Hemmelighet.', 'Body-feltet {path} inneholder et passord eller en nøkkel i klartekst. Fjern verdien fra flyten.', 'Feltet {path} inneholder et passord eller en nøkkel i klartekst. Fjern verdien fra flyten.', 'Denne verdien ser ut som et tilgangstoken i klartekst. Fjern den fra flyten.'],
        pl: ['Nagłówek {path} zawiera hasło lub klucz wpisany jawnym tekstem. Zapisz go w skarbcu i użyj pola Sekret.', 'Adres zawiera nazwę użytkownika i hasło jawnym tekstem. Zapisz je w skarbcu i użyj pola Sekret.', 'Parametr adresu {path} zawiera klucz wpisany jawnym tekstem. Zapisz go w skarbcu i użyj pola Sekret.', 'Pole treści {path} zawiera hasło lub klucz wpisany jawnym tekstem. Usuń wartość z przepływu.', 'Pole {path} zawiera hasło lub klucz wpisany jawnym tekstem. Usuń wartość z przepływu.', 'Ta wartość wygląda jak token dostępu wpisany jawnym tekstem. Usuń ją z przepływu.'],
        pt: ['O cabeçalho {path} contém uma senha ou chave em texto simples. Guarde-a no cofre e use o campo Segredo.', 'O endereço contém usuário e senha em texto simples. Guarde-os no cofre e use o campo Segredo.', 'O parâmetro de endereço {path} contém uma chave em texto simples. Guarde-a no cofre e use o campo Segredo.', 'O campo do corpo {path} contém uma senha ou chave em texto simples. Remova o valor do fluxo.', 'O campo {path} contém uma senha ou chave em texto simples. Remova o valor do fluxo.', 'Este valor parece um token de acesso em texto simples. Remova-o do fluxo.'],
        sv: ['Headern {path} innehåller ett lösenord eller en nyckel i klartext. Spara den i kassaskåpet och använd fältet Hemlighet.', 'Adressen innehåller användarnamn och lösenord i klartext. Spara dem i kassaskåpet och använd fältet Hemlighet.', 'Adressparametern {path} innehåller en nyckel i klartext. Spara den i kassaskåpet och använd fältet Hemlighet.', 'Body-fältet {path} innehåller ett lösenord eller en nyckel i klartext. Ta bort värdet från flödet.', 'Fältet {path} innehåller ett lösenord eller en nyckel i klartext. Ta bort värdet från flödet.', 'Värdet ser ut som en åtkomsttoken i klartext. Ta bort det från flödet.'],
        zh: ['请求头 {path} 以明文包含密码或密钥。请把它保存到保险库，并使用“密钥”字段。', '地址以明文包含用户名和密码。请把它们保存到保险库，并使用“密钥”字段。', '地址参数 {path} 以明文包含密钥。请把它保存到保险库，并使用“密钥”字段。', '请求体字段 {path} 以明文包含密码或密钥。请从流程中删除该值。', '字段 {path} 以明文包含密码或密钥。请从流程中删除该值。', '该值看起来像明文访问令牌。请从流程中删除。']
    }
};
```
Run it and the UI tests:

```
node C:/disposable/easydrag-s0b/lang-add.mjs C:/disposable/easydrag-s0b/lang-b1.mjs C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-s0b-credentials
cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-s0b-credentials && go test ./ui/ -run '(?i)(EasyDrag)' -count=1 && node scripts/test-easydrag.mjs
```
Expected: `added 6 keys to 16 files`, `ok  aurago/ui`, and the Node run ends with `all EasyDrag checks passed`. If a Node check pins `issueText` for an issue without `location`, it still passes (the fallback is unchanged).

- [ ] **Step 10: docs**

`cd <worktree> && sed -n 314,320p internal/flows/AGENTS.md`. Replace the bullet that starts with "`PARAM_LITERAL_CREDENTIAL` (`IssueLiteralCredential`) is a warning (never an error) on `http.request` `headers`" (through its last continuation line before the next bullet) with:

```
- `PARAM_LITERAL_CREDENTIAL` (`IssueLiteralCredential`) is a warning (never an error), one per finding of the credential rule
  (`FindLiteralCredentials`, `credentials.go`): credential headers of `http.request`, URL user info and credential query
  values in any web address, credential-named keys in JSON bodies, structured parameters and `tool.*` parameters, and known
  token formats. The issue carries `location` and `path`; `movable` is true for an `http.request` header, query value or user
  info whose node has no `auth_secret`. Exports, packs, run views and stored step params replace every finding with
  `CredentialRedactionMarker` (`redactCredentialParams`, `redactCredentialValue`). The Go and client rules share
  `testdata/credential_cases.json`.
```
Also change the sentence at line ~428 that names `redactCredentialHeaders` for stored step params to name `redactCredentialParams`.

- [ ] **Step 11: gates and commit**

```
cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-s0b-credentials && go vet ./internal/flows/ && gofmt -l internal/flows ui/desktop_easydrag_test.go
cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-s0b-credentials && node scripts/test-easydrag.mjs && node scripts/test-easydrag-render-baseline.mjs
```
Expected: `go vet` prints nothing, `gofmt -l` prints nothing, `all EasyDrag checks passed`, `PASS render baseline (unchanged)`.

Write `C:/disposable/easydrag-s0b/b1-1.txt`:

```
feat(flows): one rule for typed-in credentials

FindLiteralCredentials finds credential headers, URL user info and query
values, credential-named body and parameter fields and known token formats.
Validation warns once per finding (PARAM_LITERAL_CREDENTIAL with location,
path and movable); exports, packs, run views and stored step params replace
every finding with the marker. The vectors in testdata/credential_cases.json
pin the rule for Go and for the client.

GitNexus detect-changes unavailable in the worktree; judged by symbols and tests.

Co-Authored-By: <your model line>
```
```
cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-s0b-credentials && git add internal/flows/credentials.go internal/flows/credentials_rule_test.go internal/flows/credentials_fb4_test.go internal/flows/credentials_mixed_fb4_test.go internal/flows/testdata/credential_cases.json internal/flows/issue.go internal/flows/validate.go internal/flows/catalog_actions_http.go internal/flows/engine.go internal/flows/durable_coordinator.go internal/flows/service_runs.go internal/flows/AGENTS.md ui/js/desktop/apps/easydrag-core.js ui/desktop_easydrag_test.go ui/lang/easydrag && git commit -F C:/disposable/easydrag-s0b/b1-1.txt
```
Expected: one commit. `git status --short` shows no other changes.

---

### Task B.2: `http.request` query auth

**Files**
- Modify: `internal/flows/catalog_actions_http.go` (params, `authLiteralParams`, `withAuthSecret`, `Validate`, `Execute`, `import "net/url"`), `internal/flows/AGENTS.md`
- Create: `internal/flows/catalog_actions_http_query_auth_test.go`, `internal/tools/api_client_query_redirect_test.go`
- Modify: `ui/lang/easydrag/*.json` (5 keys)

**How redirects work today (verified).** `api_request` follows up to 10 redirects with Go's `http.Client` (`apiRequestClient`, `internal/tools/api_client.go:197`); it drops caller headers when a hop leaves the origin. The URL of a hop is the server's `Location`; Go never copies the first request's query to it. So a secret set only in the first URL cannot reach a later hop, and the node must change nothing for that. The rule to keep: the secret is added in exactly one place, to the first URL, after templates are resolved. A test in `internal/tools` pins the redirect behaviour.

- [ ] **Step 1: impact**

```
node C:/Users/Andi/.gitnexus/stores/AuraGo-08496d38b623/checkouts/AuraGo-08496d38b623/run.cjs impact "withAuthSecret" --direction upstream --repo C:/Users/Andi/Documents/repo/AuraGo
node C:/Users/Andi/.gitnexus/stores/AuraGo-08496d38b623/checkouts/AuraGo-08496d38b623/run.cjs impact "httpRequestDef" --direction upstream --repo C:/Users/Andi/Documents/repo/AuraGo
```
Expected: `withAuthSecret` is called only by `httpRequestDef`'s `Execute` (UNKNOWN: confirm with `grep -rn "withAuthSecret(" internal --include=*.go`, which must list only `catalog_actions_http.go`). Report HIGH or CRITICAL to the owner.

- [ ] **Step 2: write the failing tests**

Create `internal/flows/catalog_actions_http_query_auth_test.go`:

```go
package flows

import (
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
		"set":      {"https://api.example.com/v1?q=1", "https://api.example.com/v1?q=1&api_key=s3cr3t-token-value"},
		"replace":  {"https://api.example.com/v1?api_key=typed&q=1#top", "https://api.example.com/v1?q=1&api_key=s3cr3t-token-value#top"},
		"no query": {"https://api.example.com/v1", "https://api.example.com/v1?api_key=s3cr3t-token-value"},
	} {
		_, tools, err := runHTTP(t, queryAuthParams(c.url), answerWith(apiAnswer(200, "ok")), secrets)
		if err != nil || tools.count() != 1 {
			t.Fatalf("%s: err = %v after %d calls", name, err, tools.count())
		}
		args := tools.calls[0].Args
		if args["url"] != c.want {
			t.Errorf("%s: url = %v, want %s", name, args["url"], c.want)
		}
		if _, has := args["headers"]; has {
			t.Errorf("%s: a header was added: %v", name, args["headers"])
		}
	}
}

// The secret is registered with the scrubber when it is read, so an answer that echoes the
// request address (the body, or a final address) shows the marker, not the secret.
func TestHTTPQueryAuthScrubsTheSecretFromTheOutput(t *testing.T) {
	echo := func(req ToolRequest) (ToolResponse, error) {
		return apiAnswer(200, "you called "+Stringify(req.Args["url"])), nil
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
```
Create `internal/tools/api_client_query_redirect_test.go`:

```go
package tools

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

// A secret that http.request puts into the first address (auth_in=query) never reaches a later
// hop: the next address is the server's Location, to another host and to the same host alike.
func TestAPIRequestRedirectDoesNotCarryTheQuerySecret(t *testing.T) {
	t.Setenv("AURAGO_SSRF_ALLOW_LOOPBACK", "1")
	ConfigureRuntimePermissions(RuntimePermissions{AllowNetworkRequests: true})
	t.Cleanup(func() { ConfigureRuntimePermissions(defaultRuntimePermissionsForTests()) })

	var mu sync.Mutex
	var seen []string
	record := func(who string, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		seen = append(seen, who+" "+r.URL.Path+"?"+r.URL.RawQuery)
	}
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		record("target", r)
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer target.Close()
	start := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		record("start", r)
		switch r.URL.Path {
		case "/away":
			http.Redirect(w, r, target.URL+"/landing", http.StatusTemporaryRedirect)
		case "/here":
			http.Redirect(w, r, "/landing", http.StatusTemporaryRedirect)
		default:
			_, _ = w.Write([]byte(`{"ok":true}`))
		}
	}))
	defer start.Close()

	for _, path := range []string{"/away", "/here"} {
		if out := c13APIRequest(t, start.URL+path+"?api_key=qry-secret-value", nil); out.Status != "success" {
			t.Fatalf("%s = %+v", path, out)
		}
	}
	mu.Lock()
	defer mu.Unlock()
	for i, line := range seen {
		first := strings.HasPrefix(line, "start /away?") || strings.HasPrefix(line, "start /here?")
		if first && !strings.Contains(line, "api_key=qry-secret-value") {
			t.Errorf("request %d lost the first-hop query: %s", i, line)
		}
		if !first && strings.Contains(line, "qry-secret-value") {
			t.Errorf("request %d carries the secret after a redirect: %s", i, line)
		}
	}
	if len(seen) != 4 {
		t.Fatalf("requests = %v, want the two first hops and the two landings", seen)
	}
}
```
(`c13APIRequest` is the helper of `api_client_redirect_test.go` in the same package.)

- [ ] **Step 3: run them and watch them fail**

```
cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-s0b-credentials && GOTMPDIR='D:\tmp\go-build' go test ./internal/flows/ -run 'TestHTTPQueryAuth' -count=1
cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-s0b-credentials && GOTMPDIR='D:\tmp\go-build' go test ./internal/tools/ -run 'TestAPIRequestRedirectDoesNotCarryTheQuerySecret' -count=1
```
Expected: the flows tests FAIL (`url = https://api.example.com/v1?q=1, want ...` and the validation errors missing); the tools test PASSES already (it pins existing behaviour; that is intended).

- [ ] **Step 4: implement**

In `internal/flows/catalog_actions_http.go`:

1. Add `"net/url"` to the imports.
2. After `var httpMethods = ...` add `var authInModes = []string{"header", "query"}`.
3. Add two params after `auth_secret` in `httpRequestDef` (inside `def.Params`, before `auth_header`):

```go
		{Name: "auth_in", Kind: ParamSelect, LabelKey: "easydrag.param.auth_in", HelpKey: "easydrag.help.auth_in", Default: "header",
			Options: []Option{{Value: "header", LabelKey: "easydrag.option.auth_in_header"}, {Value: "query", LabelKey: "easydrag.option.auth_in_query"}}},
		{Name: "auth_query", Kind: ParamText, LabelKey: "easydrag.param.auth_query", VisibleIf: &Visibility{Param: "auth_in", Equals: []string{"query"}}},
```
4. Replace `var authLiteralParams = []string{"auth_header", "auth_prefix"}` with `var authLiteralParams = []string{"auth_header", "auth_prefix", "auth_in", "auth_query"}` and extend its comment: "auth_in and auth_query decide where the secret goes".
5. Add after `authPrefixValue`:

```go
// authQueryName reads auth_query: the name of the address parameter that carries the secret.
func authQueryName(v any) (string, error) {
	s, err := scalarTextParam(v, "auth_query")
	if err != nil {
		return "", err
	}
	s = strings.TrimSpace(s)
	switch {
	case s == "":
		return "", NewNodeError("FLOW_PARAM_INVALID", "auth_query must name the address parameter that carries the secret")
	case len(s) > 100 || strings.ContainsAny(s, "&=#%+?/ \t\r\n") || !validHeaderValue(s):
		return "", NewNodeError("FLOW_PARAM_INVALID", "auth_query is not a valid address parameter name")
	}
	return s, nil
}

// withQueryParam sets the address parameter name to value: every existing parameter of that
// name is dropped, whatever its value, and the new one goes last. Other parameters keep their
// text, so do the host, the path and the fragment. It runs once, on the first request's
// address: a redirect follows the server's Location and never gets the parameter again.
func withQueryParam(target, name, value string) (string, error) {
	u, ok := splitCredentialURL(target)
	if !ok {
		return "", NewNodeError("FLOW_PARAM_INVALID", "the web address must start with http:// or https://")
	}
	var keep []string
	if u.hasQuery {
		for _, pair := range strings.Split(u.query, "&") {
			key, _, _ := strings.Cut(pair, "=")
			if pair != "" && unescapeQuery(key) != name {
				keep = append(keep, pair)
			}
		}
	}
	u.query, u.hasQuery = strings.Join(append(keep, url.QueryEscape(name)+"="+url.QueryEscape(value)), "&"), true
	return u.String(), nil
}
```
6. Replace the whole function `withAuthSecret` (and its doc comment) with:

```go
// withAuthSecret adds the credential named by auth_secret to the request and returns the
// headers, the address and a scrubber for its value (one that removes nothing when no secret is
// used). With auth_in=header (the default) the secret is a header; with auth_in=query it is the
// address parameter auth_query, set on the first request's address only. The vault value is
// trimmed (a pasted secret often ends in a newline); an empty one fails instead of sending
// "Bearer ". When the request already has a header of that name, in any spelling, the vault
// secret wins: it is the deliberate setting, and data from a template must not be able to
// replace the configured credential. The value never reaches the parameters, an error message
// or an output; the node's parameters only hold the secret's name.
func withAuthSecret(in ExecInput, headers []httpHeader, target string) ([]httpHeader, string, *secretScrubber, error) {
	// The vault entry is the document author's choice, never data that flows through the
	// run: a template is refused even though the run has resolved it to a valid name (the
	// raw parameter tells), as haServiceParam does for the Home Assistant service. A caller
	// without the raw node only has the resolved value to judge.
	if in.Node != nil && isTemplateText(in.Node.Params["auth_secret"]) {
		return nil, "", nil, NewNodeError("FLOW_PARAM_INVALID", "auth_secret must name a vault secret directly; it cannot be taken from another node")
	}
	key, err := scalarTextParam(in.Params["auth_secret"], "auth_secret")
	if err != nil {
		return nil, "", nil, err
	}
	if key = strings.TrimSpace(key); key == "" {
		return headers, target, nil, nil
	}
	// auth_in, auth_query, auth_header and auth_prefix shape where and how the secret is sent;
	// like the vault entry they are the author's choice, never data that flows through the run.
	if in.Node != nil {
		for _, param := range authLiteralParams {
			if isTemplateText(in.Node.Params[param]) {
				return nil, "", nil, NewNodeError("FLOW_PARAM_INVALID", "%s", authLiteralMessage(param))
			}
		}
	}
	if err := checkFileText(key, maxSecretNameBytes, "secret name"); err != nil {
		return nil, "", nil, err
	}
	mode, ok := choiceParam(in.Params["auth_in"], "header", authInModes...)
	if !ok {
		return nil, "", nil, choiceError("auth_in", authInModes)
	}
	var name, prefix, queryName string
	if mode == "query" {
		if queryName, err = authQueryName(in.Params["auth_query"]); err != nil {
			return nil, "", nil, err
		}
	} else {
		if name, err = authHeaderName(in.Params["auth_header"]); err != nil {
			return nil, "", nil, err
		}
		if prefix, err = authPrefixValue(in.Params["auth_prefix"]); err != nil {
			return nil, "", nil, err
		}
	}
	if in.Services == nil || in.Services.Secrets == nil {
		return nil, "", nil, NewNodeError("FLOW_SECRET_UNAVAILABLE", "the vault is not available")
	}
	raw, rerr := in.Services.Secrets.ReadSecret(key)
	if rerr != nil {
		return nil, "", nil, NewNodeError("FLOW_SECRET_UNAVAILABLE", "the secret %s could not be read", quoteForError(key))
	}
	secret := strings.TrimSpace(raw)
	switch {
	case secret == "":
		return nil, "", nil, NewNodeError("FLOW_SECRET_UNAVAILABLE", "the secret %s is empty", quoteForError(key))
	case !validHeaderValue(secret):
		return nil, "", nil, NewNodeError("FLOW_SECRET_UNAVAILABLE", "the secret %s holds line breaks or other characters that cannot be sent", quoteForError(key))
	}
	scrub := newSecretScrubber(secret)
	if mode == "query" {
		next, err := withQueryParam(target, queryName, secret)
		if err != nil {
			return nil, "", nil, err
		}
		return headers, next, scrub, nil
	}
	merged := make([]httpHeader, 0, len(headers)+1)
	for _, h := range headers {
		if !strings.EqualFold(h.name, name) {
			merged = append(merged, h)
		}
	}
	merged = append(merged, httpHeader{name, prefix + secret})
	if err := headerTotals(merged); err != nil {
		return nil, "", nil, err
	}
	return merged, target, scrub, nil
}
```
7. In `Validate`, replace the block `if !isEmptyValue(n.Params["auth_secret"]) { ... }` with:

```go
		if !isEmptyValue(n.Params["auth_secret"]) {
			issues = append(issues, choiceIssue(n, "auth_in", "header", authInModes)...)
			if mode, _ := choiceParam(n.Params["auth_in"], "header", authInModes...); mode == "query" {
				issues = append(issues, literalIssue(n, "auth_query", authQueryName)...)
			} else {
				issues = append(issues, literalIssue(n, "auth_header", authHeaderName)...)
				issues = append(issues, literalIssue(n, "auth_prefix", authPrefixValue)...)
			}
			issues = append(issues, authLiteralIssues(n)...)
		}
```
8. In `Execute` replace `headers, scrub, err := withAuthSecret(in, headers)` with `headers, target, scrub, err := withAuthSecret(in, headers, target)`. `args := map[string]any{"method": method, "url": target}` below it now carries the secret-bearing address; nothing else changes. Update the doc comment of `httpRequestDef` ("Secrets:" paragraph) with one sentence: "auth_in=query sends the secret as the address parameter auth_query instead, on the first request only."

- [ ] **Step 5: locale keys (5 keys)**

Write `C:/disposable/easydrag-s0b/lang-b2.mjs` and run the helper:

```js
export default {
    keys: ['easydrag.param.auth_in', 'easydrag.param.auth_query', 'easydrag.help.auth_in', 'easydrag.option.auth_in_header', 'easydrag.option.auth_in_query'],
    langs: {
        en: ['Send secret in', 'Parameter name', 'Where the secret goes: in a header, or as a parameter of the web address. It is only sent with the first request, never again after a redirect.', 'Header', 'Address parameter'],
        de: ['Geheimnis senden in', 'Parametername', 'Wohin das Geheimnis kommt: in einen Header oder als Parameter der Webadresse. Es wird nur mit der ersten Anfrage gesendet, nach einer Weiterleitung nie wieder.', 'Header', 'Adressparameter'],
        cs: ['Odeslat tajemství v', 'Název parametru', 'Kam tajemství půjde: do hlavičky, nebo jako parametr webové adresy. Odešle se jen s prvním požadavkem, po přesměrování už nikdy.', 'Hlavička', 'Parametr adresy'],
        da: ['Send hemmelighed i', 'Parameternavn', 'Hvor hemmeligheden sendes: i en header eller som parameter i webadressen. Den sendes kun med den første forespørgsel, aldrig igen efter en omdirigering.', 'Header', 'Adresseparameter'],
        el: ['Αποστολή μυστικού σε', 'Όνομα παραμέτρου', 'Πού πηγαίνει το μυστικό: σε κεφαλίδα ή ως παράμετρος της διεύθυνσης. Στέλνεται μόνο με το πρώτο αίτημα, ποτέ ξανά μετά από ανακατεύθυνση.', 'Κεφαλίδα', 'Παράμετρος διεύθυνσης'],
        es: ['Enviar el secreto en', 'Nombre del parámetro', 'Dónde va el secreto: en un encabezado o como parámetro de la dirección web. Solo se envía con la primera petición, nunca de nuevo tras una redirección.', 'Encabezado', 'Parámetro de dirección'],
        fr: ['Envoyer le secret dans', 'Nom du paramètre', 'Où va le secret : dans un en-tête ou comme paramètre de l’adresse web. Il n’est envoyé qu’avec la première requête, jamais à nouveau après une redirection.', 'En-tête', 'Paramètre d’adresse'],
        hi: ['गुप्त मान कहाँ भेजें', 'पैरामीटर का नाम', 'गुप्त मान कहाँ जाएगा: हेडर में, या वेब पते के पैरामीटर के रूप में। यह केवल पहले अनुरोध के साथ भेजा जाता है, रीडायरेक्ट के बाद दोबारा कभी नहीं।', 'हेडर', 'पते का पैरामीटर'],
        it: ['Invia il segreto in', 'Nome del parametro', 'Dove va il segreto: in un header o come parametro dell’indirizzo web. Viene inviato solo con la prima richiesta, mai più dopo un reindirizzamento.', 'Header', 'Parametro dell’indirizzo'],
        ja: ['シークレットの送信先', 'パラメーター名', 'シークレットの送り先：ヘッダー、またはウェブアドレスのパラメーター。最初のリクエストでのみ送信され、リダイレクト後に再送されることはありません。', 'ヘッダー', 'アドレスのパラメーター'],
        nl: ['Geheim versturen in', 'Parameternaam', 'Waar het geheim naartoe gaat: in een header of als parameter van het webadres. Het wordt alleen met het eerste verzoek verstuurd, nooit opnieuw na een doorverwijzing.', 'Header', 'Adresparameter'],
        no: ['Send hemmeligheten i', 'Parameternavn', 'Hvor hemmeligheten sendes: i en header eller som parameter i nettadressen. Den sendes bare med den første forespørselen, aldri igjen etter en omdirigering.', 'Header', 'Adresseparameter'],
        pl: ['Wyślij sekret w', 'Nazwa parametru', 'Gdzie trafia sekret: do nagłówka albo jako parametr adresu WWW. Jest wysyłany tylko z pierwszym żądaniem, nigdy ponownie po przekierowaniu.', 'Nagłówek', 'Parametr adresu'],
        pt: ['Enviar o segredo em', 'Nome do parâmetro', 'Para onde vai o segredo: num cabeçalho ou como parâmetro do endereço web. Só é enviado com a primeira requisição, nunca de novo após um redirecionamento.', 'Cabeçalho', 'Parâmetro do endereço'],
        sv: ['Skicka hemligheten i', 'Parameternamn', 'Vart hemligheten går: i en header eller som parameter i webbadressen. Den skickas bara med den första förfrågan, aldrig igen efter en omdirigering.', 'Header', 'Adressparameter'],
        zh: ['密钥发送位置', '参数名', '密钥放在哪里：请求头，或网址的参数。它只随第一次请求发送，重定向后绝不会再次发送。', '请求头', '地址参数']
    }
};
```
```
node C:/disposable/easydrag-s0b/lang-add.mjs C:/disposable/easydrag-s0b/lang-b2.mjs C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-s0b-credentials
```
Expected: `added 5 keys to 16 files`.

- [ ] **Step 6: run everything the change can touch**

```
cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-s0b-credentials && GOTMPDIR='D:\tmp\go-build' go test ./internal/flows/ ./internal/tools/ -run 'TestHTTP|TestAPIRequest|Catalog|I18n|Secret|Hardening' -count=1
cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-s0b-credentials && GOTMPDIR='D:\tmp\go-build' go test ./internal/flows/ -count=1
```
Expected: `ok` for both. If a catalog test pins the parameter list or count of `http.request` (`auth_secret`, `auth_header`, `auth_prefix` in a fixed list), add `auth_in` and `auth_query` there. If a locale test names a missing key, rerun step 5 and check the 16 files.

- [ ] **Step 7: docs, then gates and commit**

In `internal/flows/AGENTS.md`, next to the paragraph that says `auth_header` and `auth_prefix` "must be written out while `auth_secret` is set" (find it with `grep -n "must be written out" internal/flows/AGENTS.md`), append: "`auth_in` (`header`, the default, or `query`) and `auth_query` (the address parameter name) follow the same rule. With `auth_in=query`, `withAuthSecret` sets the parameter on the first request's address only (`withQueryParam`); a redirect never gets it again."

```
cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-s0b-credentials && go vet ./internal/flows/ ./internal/tools/ && gofmt -l internal/flows internal/tools
```
Expected: no output. Write `C:/disposable/easydrag-s0b/b1-2.txt`:

```
feat(flows): http.request can send its secret as an address parameter

auth_in=query with auth_query sets the parameter on the first request's
address, replacing any typed value. The secret never goes into a redirect
hop, and the scrubber registered with the secret hides it in outputs. A
test in internal/tools pins the redirect behaviour.

GitNexus detect-changes unavailable in the worktree; judged by symbols and tests.

Co-Authored-By: <your model line>
```
```
cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-s0b-credentials && git add internal/flows/catalog_actions_http.go internal/flows/catalog_actions_http_query_auth_test.go internal/flows/AGENTS.md internal/tools/api_client_query_redirect_test.go ui/lang/easydrag && git commit -F C:/disposable/easydrag-s0b/b1-2.txt
```

---

### Task B.3: the move endpoint

**Files**
- Create: `internal/flows/credentials_move.go`, `internal/flows/credentials_move_test.go`, `internal/server/flows_credentials_move.go`, `internal/server/flows_credentials_move_test.go`
- Modify: `internal/server/flows_handlers.go` (`flowRouteSegments` at line ~48, `flowRoute` switch at ~606), `ui/desktop_easydrag_test.go` (error family), `ui/lang/easydrag/*.json` (2 keys)

**Contract** (`POST /api/desktop/flows/{id}/credentials/move`, admin scope, same-origin check, secret-write rate limit; routing, scope and CSRF come for free from `handleFlows`, `flowsRequiredScope` (POST -> admin) and `flowsOriginOK`):
request `{"revision":n,"node_id":"...","location":"header|url_query|url_userinfo","path":"...","secret_name":"...","replace":false}`; 200 `{"draft_revision","doc","moved":[node ids],"not_movable":[findings],"issues"}`; 409 `FLOW_REVISION_CONFLICT` (stale revision, or the value is no longer there), 409 `FLOW_SECRET_EXISTS`, 422 `FLOW_CREDENTIAL_NOT_MOVABLE`, 400 `FLOW_BAD_REQUEST`. The value never appears in an answer, a log line or the audit entry.

- [ ] **Step 1: impact**

```
node C:/Users/Andi/.gitnexus/stores/AuraGo-08496d38b623/checkouts/AuraGo-08496d38b623/run.cjs impact "flowRoute" --direction upstream --repo C:/Users/Andi/Documents/repo/AuraGo
node C:/Users/Andi/.gitnexus/stores/AuraGo-08496d38b623/checkouts/AuraGo-08496d38b623/run.cjs impact "FindLiteralCredentials" --direction upstream --repo C:/Users/Andi/Documents/repo/AuraGo
```
Expected: `flowRoute` is called by `handleFlows` only; the edit adds one case. `FindLiteralCredentials` is new (UNKNOWN or no callers: it is only defined by B.1 and used by this task).

- [ ] **Step 2: write the failing tests**

Create `internal/flows/credentials_move_test.go`:

```go
package flows

import (
	"encoding/base64"
	"errors"
	"reflect"
	"testing"
)

func sentHeaders(t *testing.T, tools *fakeTools) map[string]any {
	t.Helper()
	if tools.count() != 1 {
		t.Fatalf("calls = %d", tools.count())
	}
	h, _ := tools.calls[0].Args["headers"].(map[string]any)
	return h
}

// A moved header, query value and user info still reach the request: B.2's auth_in carries them.
func TestMovedCredentialsStillReachTheRequest(t *testing.T) {
	doc := &Flow{Nodes: []Node{
		{ID: "n1", Type: TypeHTTPRequest, Params: map[string]any{"url": "https://api.example.com/v1?api_key=qry-token-1234&q=1", "headers": map[string]any{"X-Api-Key": "hdr-token-1234", "Accept": "text/plain"}}},
		{ID: "n2", Type: TypeHTTPRequest, Params: map[string]any{"url": "https://api.example.com/v2", "headers": map[string]any{"X-Api-Key": "hdr-token-1234"}}},
	}}
	moved, notMovable, value, err := MoveCredential(doc, "n1", "header", "X-Api-Key", "svc_key")
	if err != nil || value != "hdr-token-1234" || !reflect.DeepEqual(moved, []string{"n1", "n2"}) || len(notMovable) != 0 {
		t.Fatalf("MoveCredential = %v, %v, %q, %v", moved, notMovable, value, err)
	}
	// n1 has its secret slot now, so its query token is no longer movable.
	if _, _, _, err := MoveCredential(doc, "n1", "url_query", "api_key", "svc_q"); !errors.Is(err, ErrCredentialNotMovable) {
		t.Fatalf("second move = %v, want ErrCredentialNotMovable", err)
	}
	tools := &fakeTools{respond: answerWith(apiAnswer(200, "ok"))}
	if _, err := execDef(httpNodeDef(t), doc.NodeByID("n1").Params, &Services{Tools: tools, Secrets: fakeSecrets{"svc_key": "hdr-token-1234"}}); err != nil {
		t.Fatal(err)
	}
	if h := sentHeaders(t, tools); h["X-Api-Key"] != "hdr-token-1234" || h["Accept"] != "text/plain" {
		t.Fatalf("sent headers = %v: no scheme prefix may be added", h)
	}
}

func TestMovedQueryAndUserinfoStillReachTheRequest(t *testing.T) {
	q := &Flow{Nodes: []Node{{ID: "n1", Type: TypeHTTPRequest, Params: map[string]any{"url": "https://api.example.com/v1?api_key=qry-token-1234&q=1"}}}}
	if _, _, value, err := MoveCredential(q, "n1", "url_query", "api_key", "svc_q"); err != nil || value != "qry-token-1234" {
		t.Fatalf("query move = %q, %v", value, err)
	}
	if p := q.Nodes[0].Params; p["url"] != "https://api.example.com/v1?q=1" || p["auth_in"] != "query" || p["auth_query"] != "api_key" || p["auth_secret"] != "svc_q" {
		t.Fatalf("query params = %v", p)
	}
	tools := &fakeTools{respond: answerWith(apiAnswer(200, "ok"))}
	if _, err := execDef(httpNodeDef(t), q.Nodes[0].Params, &Services{Tools: tools, Secrets: fakeSecrets{"svc_q": "qry-token-1234"}}); err != nil {
		t.Fatal(err)
	}
	if got := tools.calls[0].Args["url"]; got != "https://api.example.com/v1?q=1&api_key=qry-token-1234" {
		t.Fatalf("url = %v", got)
	}

	u := &Flow{Nodes: []Node{{ID: "n1", Type: TypeHTTPRequest, Params: map[string]any{"url": "https://user:pw1234@files.example.com/list"}}}}
	want := base64.StdEncoding.EncodeToString([]byte("user:pw1234"))
	if _, _, value, err := MoveCredential(u, "n1", "url_userinfo", "userinfo", "svc_basic"); err != nil || value != want {
		t.Fatalf("userinfo move = %q, %v", value, err)
	}
	tools = &fakeTools{respond: answerWith(apiAnswer(200, "ok"))}
	if _, err := execDef(httpNodeDef(t), u.Nodes[0].Params, &Services{Tools: tools, Secrets: fakeSecrets{"svc_basic": want}}); err != nil {
		t.Fatal(err)
	}
	if h := sentHeaders(t, tools); h["Authorization"] != "Basic "+want || tools.calls[0].Args["url"] != "https://files.example.com/list" {
		t.Fatalf("sent = %v %v", h, tools.calls[0].Args["url"])
	}
}

func TestMoveCredentialRefusesWhatCannotMove(t *testing.T) {
	doc := &Flow{Nodes: []Node{{ID: "n1", Type: TypeHTTPRequest, Params: map[string]any{"url": "https://a.example/x", "method": "POST", "body": map[string]any{"password": "hunter2hunter2"}}}}}
	if _, _, _, err := MoveCredential(doc, "n1", "body_field", "/password", "x"); !errors.Is(err, ErrCredentialNotMovable) {
		t.Fatalf("body field = %v, want ErrCredentialNotMovable", err)
	}
	if _, _, _, err := MoveCredential(doc, "n1", "header", "Authorization", "x"); !errors.Is(err, ErrCredentialGone) {
		t.Fatalf("missing header = %v, want ErrCredentialGone", err)
	}
}
```
Create `internal/server/flows_credentials_move_test.go`:

```go
package server

import (
	"encoding/base64"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

const (
	credHeaderToken = "hdr-token-1234567890"
	credQueryToken  = "qry-token-1234567890"
	credBodyToken   = "body-pass-1234567890"
)

// call, other, files and post carry a typed credential each; call and other share the header value.
var credentialMoveFlowJSON = `{"schema":1,"name":"Wetter","nodes":[
 {"id":"n_aaaaaaaa","key":"start","type":"trigger.manual","type_version":1,"label":"Start","position":{"x":0,"y":0},"params":{}},
 {"id":"n_bbbbbbbb","key":"call","type":"http.request","type_version":1,"label":"Call","position":{"x":300,"y":0},
  "params":{"url":"https://api.example.com/v1/weather?api_key=` + credQueryToken + `&q=berlin","headers":{"Authorization":"Bearer ` + credHeaderToken + `"}}},
 {"id":"n_cccccccc","key":"other","type":"http.request","type_version":1,"label":"Other","position":{"x":600,"y":0},
  "params":{"url":"https://api.example.com/v1/other","headers":{"Authorization":"Bearer ` + credHeaderToken + `"}}},
 {"id":"n_dddddddd","key":"files","type":"http.request","type_version":1,"label":"Files","position":{"x":900,"y":0},
  "params":{"url":"https://user:pw123456@files.example.com/list"}},
 {"id":"n_eeeeeeee","key":"post","type":"http.request","type_version":1,"label":"Post","position":{"x":1200,"y":0},
  "params":{"url":"https://api.example.com/v1/post","method":"POST","body":{"password":"` + credBodyToken + `"}}}],
 "edges":[{"id":"e_aaaaaaaa","source":{"node":"n_aaaaaaaa","port":"out"},"target":{"node":"n_bbbbbbbb","port":"in"}},
  {"id":"e_bbbbbbbb","source":{"node":"n_bbbbbbbb","port":"out"},"target":{"node":"n_cccccccc","port":"in"}},
  {"id":"e_cccccccc","source":{"node":"n_cccccccc","port":"out"},"target":{"node":"n_dddddddd","port":"in"}},
  {"id":"e_dddddddd","source":{"node":"n_dddddddd","port":"out"},"target":{"node":"n_eeeeeeee","port":"in"}}]}`

func moveBody(rev int, node, location, path, name string, replace bool) string {
	return fmt.Sprintf(`{"revision":%d,"node_id":%q,"location":%q,"path":%q,"secret_name":%q,"replace":%t}`, rev, node, location, path, name, replace)
}

func moveCall(t *testing.T, s *Server, token, flowID, body string) *httptest.ResponseRecorder {
	t.Helper()
	return flowsCall(t, s, http.MethodPost, "/api/desktop/flows/"+flowID+"/credentials/move", token, body)
}

func draftParams(t *testing.T, s *Server, flowID, node string) map[string]any {
	t.Helper()
	rec, err := s.Flows.GetFlow(t.Context(), flowID)
	if err != nil {
		t.Fatal(err)
	}
	return rec.Draft.NodeByID(node).Params
}

func TestFlowCredentialMoveHeaderQueryAndUserinfo(t *testing.T) {
	s, token := newFlowsTestServer(t)
	secret := func(name string) string {
		t.Helper()
		v, err := (flowSecrets{s: s}).ReadSecret(name)
		if err != nil {
			t.Fatalf("ReadSecret(%s): %v", name, err)
		}
		return v
	}
	t.Run("header, and the identical value in another node", func(t *testing.T) {
		rec := createTestFlow(t, s, credentialMoveFlowJSON)
		w := moveCall(t, s, token, rec.ID, moveBody(rec.DraftRevision, "n_bbbbbbbb", "header", "Authorization", "weather_key", false))
		if w.Code != http.StatusOK || strings.Contains(w.Body.String(), credHeaderToken) {
			t.Fatalf("move = %d, the answer must not carry the value: %s", w.Code, w.Body.String())
		}
		body := flowsBody(t, w)
		moved, _ := body["moved"].([]any)
		if body["draft_revision"] != float64(rec.DraftRevision+1) || len(moved) != 2 || moved[0] != "n_bbbbbbbb" || moved[1] != "n_cccccccc" {
			t.Fatalf("answer = %v", body)
		}
		if secret("weather_key") != credHeaderToken {
			t.Fatal("the vault entry does not hold the token")
		}
		for _, id := range []string{"n_bbbbbbbb", "n_cccccccc"} {
			p := draftParams(t, s, rec.ID, id)
			if _, has := p["headers"].(map[string]any)["Authorization"]; has || p["auth_secret"] != "weather_key" || p["auth_in"] != "header" || p["auth_header"] != "Authorization" || p["auth_prefix"] != "Bearer " {
				t.Errorf("%s params = %v", id, p)
			}
		}
	})
	t.Run("url query", func(t *testing.T) {
		rec := createTestFlow(t, s, credentialMoveFlowJSON)
		w := moveCall(t, s, token, rec.ID, moveBody(rec.DraftRevision, "n_bbbbbbbb", "url_query", "api_key", "weather_q", false))
		if w.Code != http.StatusOK || strings.Contains(w.Body.String(), credQueryToken) {
			t.Fatalf("move = %d, the answer must not carry the value: %s", w.Code, w.Body.String())
		}
		p := draftParams(t, s, rec.ID, "n_bbbbbbbb")
		if secret("weather_q") != credQueryToken || p["url"] != "https://api.example.com/v1/weather?q=berlin" || p["auth_in"] != "query" || p["auth_query"] != "api_key" || p["auth_secret"] != "weather_q" {
			t.Fatalf("params = %v", p)
		}
	})
	t.Run("url user info", func(t *testing.T) {
		rec := createTestFlow(t, s, credentialMoveFlowJSON)
		w := moveCall(t, s, token, rec.ID, moveBody(rec.DraftRevision, "n_dddddddd", "url_userinfo", "userinfo", "files_login", false))
		if w.Code != http.StatusOK || strings.Contains(w.Body.String(), "pw123456") {
			t.Fatalf("move = %d, the answer must not carry the value: %s", w.Code, w.Body.String())
		}
		p := draftParams(t, s, rec.ID, "n_dddddddd")
		if secret("files_login") != base64.StdEncoding.EncodeToString([]byte("user:pw123456")) || p["url"] != "https://files.example.com/list" || p["auth_header"] != "Authorization" || p["auth_prefix"] != "Basic " {
			t.Fatalf("params = %v", p)
		}
	})
}

func TestFlowCredentialMoveRefusals(t *testing.T) {
	s, token := newFlowsTestServer(t)
	rec := createTestFlow(t, s, credentialMoveFlowJSON)
	good := moveBody(rec.DraftRevision, "n_bbbbbbbb", "header", "Authorization", "weather_key", false)
	code := func(w *httptest.ResponseRecorder) string { return fmt.Sprint(flowsBody(t, w)["code"]) }
	revision := func() int {
		r, err := s.Flows.GetFlow(t.Context(), rec.ID)
		if err != nil {
			t.Fatal(err)
		}
		return r.DraftRevision
	}

	if w := moveCall(t, s, token, rec.ID, moveBody(rec.DraftRevision+5, "n_bbbbbbbb", "header", "Authorization", "weather_key", false)); w.Code != http.StatusConflict || code(w) != "FLOW_REVISION_CONFLICT" {
		t.Fatalf("stale revision = %d %s", w.Code, w.Body.String())
	}
	if w := moveCall(t, s, token, rec.ID, moveBody(rec.DraftRevision, "n_bbbbbbbb", "header", "X-Missing", "weather_key", false)); w.Code != http.StatusConflict || code(w) != "FLOW_REVISION_CONFLICT" {
		t.Fatalf("value no longer there = %d %s", w.Code, w.Body.String())
	}
	w := moveCall(t, s, token, rec.ID, moveBody(rec.DraftRevision, "n_eeeeeeee", "body_field", "/password", "post_pass", false))
	if w.Code != http.StatusUnprocessableEntity || code(w) != "FLOW_CREDENTIAL_NOT_MOVABLE" || strings.Contains(w.Body.String(), credBodyToken) {
		t.Fatalf("body field = %d %s", w.Code, w.Body.String())
	}
	if _, err := (flowSecrets{s: s}).ReadSecret("post_pass"); err == nil {
		t.Fatal("a refused move wrote a vault entry")
	}
	if w := moveCall(t, s, token, rec.ID, moveBody(rec.DraftRevision, "n_bbbbbbbb", "header", "Authorization", "Bad-Name", false)); w.Code != http.StatusBadRequest {
		t.Fatalf("bad name = %d %s", w.Code, w.Body.String())
	}

	readOnly, _, err := s.TokenManager.Create("read only", []string{desktopScopeRead, desktopScopeWrite}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if w := moveCall(t, s, readOnly, rec.ID, good); w.Code != http.StatusForbidden || !strings.Contains(w.Body.String(), "desktop_scope_required") {
		t.Fatalf("non-admin = %d %s", w.Code, w.Body.String())
	}
	r := httptest.NewRequest(http.MethodPost, "/api/desktop/flows/"+rec.ID+"/credentials/move", strings.NewReader(good))
	r.AddCookie(&http.Cookie{Name: sessionCookieName, Value: createSessionValue(s.Cfg.Auth.SessionSecret, time.Now().Add(time.Hour))})
	r.Header.Set("Origin", "https://evil.example")
	cross := httptest.NewRecorder()
	s.handleFlows(cross, r)
	if cross.Code != http.StatusForbidden {
		t.Fatalf("cross origin = %d %s", cross.Code, cross.Body.String())
	}

	// An existing secret is only replaced when asked; a refusal changes nothing.
	if w := flowsCall(t, s, http.MethodPut, "/api/desktop/flows/secrets/weather_key", token, `{"value":"old-value-123456"}`); w.Code != http.StatusOK {
		t.Fatalf("put secret = %d", w.Code)
	}
	before := revision()
	if w := moveCall(t, s, token, rec.ID, good); w.Code != http.StatusConflict || code(w) != "FLOW_SECRET_EXISTS" {
		t.Fatalf("existing secret = %d %s", w.Code, w.Body.String())
	}
	if v, _ := (flowSecrets{s: s}).ReadSecret("weather_key"); v != "old-value-123456" || revision() != before {
		t.Fatalf("a refused move changed the vault (%q) or the draft (%d)", v, revision())
	}
	if w := moveCall(t, s, token, rec.ID, moveBody(rec.DraftRevision, "n_bbbbbbbb", "header", "Authorization", "weather_key", true)); w.Code != http.StatusOK {
		t.Fatalf("replace = %d %s", w.Code, w.Body.String())
	}
	if v, _ := (flowSecrets{s: s}).ReadSecret("weather_key"); v != credHeaderToken {
		t.Fatalf("replaced value = %q", v)
	}
}
```
(If the Go toolchain lacks `t.Context`, use `context.Background()` and import `context`.)

- [ ] **Step 3: run them and watch them fail**

```
cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-s0b-credentials && GOTMPDIR='D:\tmp\go-build' go test ./internal/flows/ -run 'TestMove|TestMoved' -count=1
cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-s0b-credentials && go test ./internal/server/ -run 'TestFlowCredentialMove' -count=1
```
Expected: both FAIL to build: `undefined: MoveCredential`, `undefined: ErrCredentialNotMovable`.

- [ ] **Step 4: the rewrite logic in `internal/flows`**

Create `internal/flows/credentials_move.go`:

```go
package flows

import (
	"encoding/base64"
	"errors"
	"strings"
)

var (
	// ErrCredentialGone: the credential is no longer at that place of the draft.
	ErrCredentialGone = errors.New("the credential is no longer in the flow")
	// ErrCredentialNotMovable: the finding exists, but only an http.request header, query value
	// or user info with a free auth_secret can move to the vault.
	ErrCredentialNotMovable = errors.New("this credential cannot be moved to the vault")
)

// MoveCredential rewrites doc (a copy the caller owns) so that the credential at (nodeID,
// location, path) is read from the vault entry secretName instead: header -> auth_in=header,
// auth_header, auth_prefix; url_query -> auth_in=query, auth_query; url_userinfo -> Basic
// authorisation. Every other movable finding with the identical literal and location in the
// same document gets the same rewrite (moved lists the node ids, in document order);
// notMovable lists the ones that cannot move (their node has an auth_secret already, or the
// value mixes a template). value is what the caller stores in the vault; it is never returned
// to a browser.
func MoveCredential(doc *Flow, nodeID, location, path, secretName string) (moved []string, notMovable []CredentialFinding, value string, err error) {
	if doc == nil {
		return nil, nil, "", ErrCredentialGone
	}
	findings := FindLiteralCredentials(doc)
	var target *CredentialFinding
	for i := range findings {
		if f := &findings[i]; f.NodeID == nodeID && f.Location == location && f.Path == path {
			target = f
			break
		}
	}
	if target == nil {
		return nil, nil, "", ErrCredentialGone
	}
	if !target.Movable {
		return nil, nil, "", ErrCredentialNotMovable
	}
	secret, prefix := credentialSecret(*target)
	if strings.TrimSpace(secret) == "" {
		return nil, nil, "", ErrCredentialNotMovable
	}
	seen := map[string]bool{}
	for _, f := range findings {
		if f.Location != target.Location || f.Value != target.Value {
			continue
		}
		node := doc.NodeByID(f.NodeID)
		// The slot is checked again: an earlier rewrite of this loop can have taken it.
		if !f.Movable || node == nil || !isEmptyValue(node.Params["auth_secret"]) {
			notMovable = append(notMovable, f)
			continue
		}
		rewriteCredential(node, f, secretName, prefix)
		if !seen[node.ID] {
			seen[node.ID] = true
			moved = append(moved, node.ID)
		}
	}
	return moved, notMovable, secret, nil
}

// credentialSecret derives what goes into the vault and the header prefix from a finding.
func credentialSecret(f CredentialFinding) (secret, prefix string) {
	switch f.Location {
	case credHeader:
		for _, scheme := range []string{"Bearer ", "Basic "} {
			if len(f.Value) > len(scheme) && strings.EqualFold(f.Value[:len(scheme)], scheme) {
				return strings.TrimSpace(f.Value[len(scheme):]), f.Value[:len(scheme)]
			}
		}
		return strings.TrimSpace(f.Value), ""
	case credUserinfo:
		return base64.StdEncoding.EncodeToString([]byte(f.Value)), "Basic "
	}
	return f.Value, ""
}

// rewriteURLParam applies fn to the first web address word of a text parameter.
func rewriteURLParam(v any, fn func(u *credentialURL)) any {
	s, ok := v.(string)
	if !ok {
		return v
	}
	tokens := credentialWords.FindAllString(s, -1)
	for i, tok := range tokens {
		if u, isURL := splitCredentialURL(tok); isURL {
			fn(&u)
			tokens[i] = u.String()
			break
		}
	}
	return strings.Join(tokens, "")
}

func rewriteCredential(n *Node, f CredentialFinding, secretName, prefix string) {
	n.Params["auth_secret"] = secretName
	switch f.Location {
	case credHeader:
		if headers, ok := n.Params["headers"].(map[string]any); ok {
			for k := range headers {
				if strings.TrimSpace(k) == f.Path {
					delete(headers, k)
				}
			}
		}
		n.Params["auth_in"], n.Params["auth_header"], n.Params["auth_prefix"] = "header", f.Path, prefix
	case credQuery:
		n.Params["url"] = rewriteURLParam(n.Params["url"], func(u *credentialURL) {
			var keep []string
			for _, pair := range strings.Split(u.query, "&") {
				key, val, _ := strings.Cut(pair, "=")
				if unescapeQuery(key) == f.Path && unescapeQuery(val) == f.Value {
					continue
				}
				keep = append(keep, pair)
			}
			u.query, u.hasQuery = strings.Join(keep, "&"), len(keep) > 0
		})
		n.Params["auth_in"], n.Params["auth_query"] = "query", f.Path
	case credUserinfo:
		n.Params["url"] = rewriteURLParam(n.Params["url"], func(u *credentialURL) { u.userinfo, u.hasUser = "", false })
		n.Params["auth_in"], n.Params["auth_header"], n.Params["auth_prefix"] = "header", "Authorization", "Basic "
	}
}
```
Run:
```
cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-s0b-credentials && GOTMPDIR='D:\tmp\go-build' go test ./internal/flows/ -run 'TestMove|TestMoved' -count=1
```
Expected: `ok`. If `TestMovedCredentialsStillReachTheRequest` fails on the header, check that `auth_prefix` is present as an empty string (`withDefaults` only fills absent keys).

- [ ] **Step 5: the handler (thin)**

Create `internal/server/flows_credentials_move.go`:

```go
package server

import (
	"errors"
	"net/http"
	"slices"
	"strings"
	"time"

	"aurago/internal/flows"
)

// flowCredentialMove moves one typed-in credential of a flow's draft into the vault (POST
// /api/desktop/flows/{id}/credentials/move). The rewrite is flows.MoveCredential; this only
// checks the request, writes the vault entry and saves the draft at the revision the client
// saw. The value goes from the draft to the vault and nowhere else: not into the answer, the
// audit entry or a log line.
func (s *Server) flowCredentialMove(w http.ResponseWriter, r *http.Request, id string) {
	if r.Method != http.MethodPost {
		flowsMethodNotAllowed(w)
		return
	}
	if s.Vault == nil {
		flowsError(w, http.StatusServiceUnavailable, "FLOW_VAULT_UNAVAILABLE", "the vault is not available")
		return
	}
	var body struct {
		Revision   int    `json:"revision"`
		NodeID     string `json:"node_id"`
		Location   string `json:"location"`
		Path       string `json:"path"`
		SecretName string `json:"secret_name"`
		Replace    bool   `json:"replace"`
	}
	if !flowsDecode(w, r, &body, flowsSmallBodyLimit, false) {
		return
	}
	if !flowSecretNamePattern.MatchString(body.SecretName) {
		flowsError(w, http.StatusBadRequest, "FLOW_BAD_REQUEST", "secret names use a-z, 0-9 and _ (at most 40 characters)")
		return
	}
	cfg := s.ConfigSnapshot()
	if !s.flowSecretRate.allow(ClientIP(r, cfg != nil && cfg.Server.HTTPS.BehindProxy), time.Now(), flowSecretWritesPerWindow, flowSecretWriteWindow) {
		w.Header().Set("Retry-After", "60")
		flowsError(w, http.StatusTooManyRequests, "FLOW_RATE_LIMITED", "too many flow secret changes; try again in a minute")
		return
	}
	ctx := r.Context()
	rec, err := s.Flows.GetFlow(ctx, id)
	if err != nil {
		s.flowsErrorFrom(w, r, err)
		return
	}
	if rec.DraftRevision != body.Revision {
		s.flowsErrorFrom(w, r, flows.ErrRevisionConflict)
		return
	}
	doc, err := rec.Draft.Clone()
	if err != nil {
		s.flowsErrorFrom(w, r, err)
		return
	}
	moved, notMovable, value, err := flows.MoveCredential(doc, body.NodeID, body.Location, body.Path, body.SecretName)
	switch {
	case errors.Is(err, flows.ErrCredentialGone):
		flowsError(w, http.StatusConflict, "FLOW_REVISION_CONFLICT", "the value is no longer in the draft")
		return
	case errors.Is(err, flows.ErrCredentialNotMovable):
		flowsError(w, http.StatusUnprocessableEntity, "FLOW_CREDENTIAL_NOT_MOVABLE", "this value cannot be moved to the vault")
		return
	case err != nil:
		s.flowsErrorFrom(w, r, err)
		return
	}
	// Validate before the vault is touched, so a refusal leaves nothing behind.
	if issues := s.Flows.Validate(doc, flows.ModeDraft); flows.HasErrors(issues) {
		s.flowsErrorFrom(w, r, &flows.ValidationError{Issues: issues})
		return
	}
	key := flowSecretPrefix + body.SecretName
	keys, err := s.Vault.ListKeys()
	if err != nil {
		s.flowsErrorFrom(w, r, err)
		return
	}
	if slices.Contains(keys, key) && !body.Replace {
		flowsError(w, http.StatusConflict, "FLOW_SECRET_EXISTS", "a secret with this name already exists")
		return
	}
	if err := s.Vault.WriteUserSecretContext(ctx, key, value, true); err != nil {
		s.flowsErrorFrom(w, r, err)
		return
	}
	// A concurrent save between the read and this CAS leaves the vault entry without a user;
	// the client reloads (409) and tries again.
	rev, issues, err := s.Flows.SaveDraft(ctx, id, doc, body.Revision)
	if err != nil {
		s.flowsErrorFrom(w, r, err)
		return
	}
	s.recordFlowAudit("flow_credential_moved", id, rec.Name, "Flow "+rec.Name+": credential moved to secret "+body.SecretName+" in node(s) "+strings.Join(moved, ", "))
	s.broadcastFlowsChanged(id, "saved")
	if notMovable == nil {
		notMovable = []flows.CredentialFinding{}
	}
	flowsJSON(w, http.StatusOK, map[string]any{"draft_revision": rev, "doc": doc, "moved": moved, "not_movable": notMovable, "issues": nonNilIssues(issues)})
}
```
In `internal/server/flows_handlers.go`:
1. Add `"credentials": 2` to the map `flowRouteSegments` (and mention `credentials/move` in the comment above it).
2. In `flowRoute`'s `switch action`, add before `case "":`:

```go
	case "credentials":
		if rest[1] != "move" {
			flowsError(w, http.StatusNotFound, "FLOW_NOT_FOUND", "unknown flow route")
			return
		}
		s.flowCredentialMove(w, r, id)
```
Run:
```
cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-s0b-credentials && go test ./internal/server/ -run 'TestFlowCredentialMove' -count=1 -v
```
Expected: `--- PASS` for `TestFlowCredentialMoveHeaderQueryAndUserinfo` (3 subtests) and `TestFlowCredentialMoveRefusals`, then `ok`. If `createTestFlow` fails in `ParseFlow`, print the error: the fixture node ids must be `n_` plus eight characters. If the cross-origin request is not refused with 403, copy the exact setup of `TestFlowExtraction...` in `flows_extraction_test.go` (cookie plus Origin).

- [ ] **Step 6: the two error codes (locales and the family list)**

In `ui/desktop_easydrag_test.go`, in `families["easydrag.ui.error_"]`, append `"flow_credential_not_movable", "flow_secret_exists"` to the member list (for example before the comment line `// The 503 causes other than flows switched off`). Write `C:/disposable/easydrag-s0b/lang-b3.mjs`:

```js
export default {
    keys: ['easydrag.ui.error_flow_credential_not_movable', 'easydrag.ui.error_flow_secret_exists'],
    langs: {
        en: ['This value cannot be moved to the vault. Remove it from the flow instead.', 'A secret with this name already exists.'],
        de: ['Dieser Wert lässt sich nicht in den Tresor verschieben. Entferne ihn stattdessen aus dem Flow.', 'Ein Geheimnis mit diesem Namen gibt es schon.'],
        cs: ['Tuto hodnotu nelze přesunout do trezoru. Odstraň ji místo toho z flow.', 'Tajemství s tímto názvem už existuje.'],
        da: ['Værdien kan ikke flyttes til tresoren. Fjern den i stedet fra flowet.', 'Der findes allerede en hemmelighed med dette navn.'],
        el: ['Αυτή η τιμή δεν μπορεί να μετακινηθεί στο θησαυροφυλάκιο. Αφαίρεσέ την από τη ροή.', 'Υπάρχει ήδη μυστικό με αυτό το όνομα.'],
        es: ['Este valor no se puede mover a la bóveda. Quítalo del flujo.', 'Ya existe un secreto con este nombre.'],
        fr: ['Cette valeur ne peut pas être déplacée dans le coffre. Retire-la plutôt du flux.', 'Un secret portant ce nom existe déjà.'],
        hi: ['इस मान को तिजोरी में नहीं ले जाया जा सकता। इसे फ़्लो से हटाओ।', 'इस नाम का गुप्त मान पहले से मौजूद है।'],
        it: ['Questo valore non può essere spostato nel caveau. Rimuovilo invece dal flusso.', 'Esiste già un segreto con questo nome.'],
        ja: ['この値は保管庫へ移せません。代わりにフローから削除してください。', 'この名前のシークレットは既にあります。'],
        nl: ['Deze waarde kan niet naar de kluis worden verplaatst. Verwijder hem in plaats daarvan uit de flow.', 'Er bestaat al een geheim met deze naam.'],
        no: ['Denne verdien kan ikke flyttes til hvelvet. Fjern den fra flyten i stedet.', 'Det finnes allerede en hemmelighet med dette navnet.'],
        pl: ['Tej wartości nie można przenieść do skarbca. Usuń ją z przepływu.', 'Sekret o tej nazwie już istnieje.'],
        pt: ['Este valor não pode ser movido para o cofre. Remova-o do fluxo.', 'Já existe um segredo com este nome.'],
        sv: ['Värdet kan inte flyttas till kassaskåpet. Ta bort det från flödet i stället.', 'Det finns redan en hemlighet med det här namnet.'],
        zh: ['该值无法移入保险库。请改为从流程中删除。', '已存在同名密钥。']
    }
};
```
```
node C:/disposable/easydrag-s0b/lang-add.mjs C:/disposable/easydrag-s0b/lang-b3.mjs C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-s0b-credentials
cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-s0b-credentials && go test ./ui/ -run '(?i)(EasyDrag)' -count=1
```
Expected: `added 2 keys to 16 files`, `ok  aurago/ui`.

- [ ] **Step 7: wider server check, gates, commit**

```
cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-s0b-credentials && go test ./internal/server/ -run '(?i)(Flow|Secret)' -count=1
cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-s0b-credentials && go vet ./internal/flows/ ./internal/server/ && gofmt -l internal/flows internal/server ui/desktop_easydrag_test.go
```
Expected: `ok  aurago/internal/server`, no vet or gofmt output. Write `C:/disposable/easydrag-s0b/b1-3.txt`:

```
feat(flows): move a typed credential into the vault

POST /api/desktop/flows/{id}/credentials/move rewrites a header, a URL query
value or URL user info of an http.request draft to the vault secret (auth_in
header or query), moves identical values elsewhere in the same flow, and
saves at the revision the client saw. Body fields answer 422
FLOW_CREDENTIAL_NOT_MOVABLE, an existing secret 409 FLOW_SECRET_EXISTS. The
value never leaves the server in an answer, audit entry or log line.

GitNexus detect-changes unavailable in the worktree; judged by symbols and tests.

Co-Authored-By: <your model line>
```
```
cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-s0b-credentials && git add internal/flows/credentials_move.go internal/flows/credentials_move_test.go internal/server/flows_credentials_move.go internal/server/flows_credentials_move_test.go internal/server/flows_handlers.go ui/desktop_easydrag_test.go ui/lang/easydrag && git commit -F C:/disposable/easydrag-s0b/b1-3.txt
```

---

### Task B.4: the UI action (run Task B.5 first)

**Prerequisite.** `ED.credentials` from Task B.5 must be committed: this task reads the typed value with `ED.credentials.findings` and clears one with `ED.credentials.remove`. Check with `cd <worktree> && git log --oneline | head -3` (the B.5 commit must be there) and `ls ui/js/desktop/apps/easydrag-credentials.js`.

**Files**
- Create: `ui/js/desktop/apps/easydrag-credentials-move.js`, `ui/testdata/easydrag-credentials-fixture.js`, `scripts/test-easydrag-credentials-move.mjs`, `ui/desktop_easydrag_credentials_browser_test.go`
- Modify: `ui/js/desktop/apps/easydrag-model.js` (`scrubHistory`, ~line 960), `ui/js/desktop/apps/easydrag-core.js` (api method), `ui/js/desktop/apps/easydrag-forms.js` (two hooks), `ui/js/desktop/apps/easydrag-editor.js` (two lines), `ui/css/desktop-app-easydrag.css` (one rule), `ui/js/desktop/core/module-loader.js`, `ui/desktop_easydrag_test.go` (`easyDragModules`), `scripts/test-easydrag.mjs` (import the new test), `ui/lang/easydrag/*.json` (6 keys)

**Where it hooks in.** Issues reach the detail view through `ED.forms` (`easydrag-forms.js`, the `.ed-field-issue` box built in `build()` and updated in `syncIssues()`); a click on an issue anywhere (card marker, issues popover, publish dialog) opens that detail, so the action lives there. The editor stays at 894 lines: only two lines are added to it (ratchet: warns above 900).

- [ ] **Step 1: write the failing Node test**

Create `scripts/test-easydrag-credentials-move.mjs`:

```js
// B.4: the credential move UI (name proposal, the move itself against a stub server, history scrub,
// the purge offer after a publish) and ED.model.scrubHistory.
import assert from 'node:assert/strict';
import fs from 'node:fs';
import vm from 'node:vm';
import { webcrypto } from 'node:crypto';

const apps = new URL('../ui/js/desktop/apps/', import.meta.url);
const box = vm.createContext({ window: {}, console, Intl, Date, Set, Map, URL, crypto: webcrypto });
for (const name of ['easydrag-template.js', 'easydrag-credentials.js', 'easydrag-model.js', 'easydrag-credentials-move.js']) {
    const file = new URL(name, apps);
    vm.runInContext(fs.readFileSync(file, 'utf8'), box, { filename: file.pathname });
}
const ED = box.window.EasyDrag;
const plain = value => JSON.parse(JSON.stringify(value));
const TOKEN = 'tok-secret-1234567890';
const doc = () => ({ schema: 1, name: 'x', nodes: [{ id: 'n_aaaaaaaa', key: 'call', type: 'http.request', label: 'Call', position: { x: 0, y: 0 },
    params: { url: 'https://api.example.com/v1?api_key=' + TOKEN + '&q=1' }, settings: {} }], edges: [] });

// The proposal is built from the host and the key, cut to 40 characters.
const node = { id: 'n1', type: 'http.request', params: { url: 'https://api.example.com/v1?api_key=x' } };
assert.equal(ED.credentialsMove.proposeName(node, { location: 'url_query', path: 'api_key' }), 'api_example_com_api_key');
assert.equal(ED.credentialsMove.proposeName({ params: { url: '{{trigger.data.u}}' } }, { location: 'header', path: 'Authorization' }), 'authorization');
assert.equal(ED.credentialsMove.proposeName({ params: { url: 'https://' + 'a'.repeat(60) + '.example/x' } }, { location: 'url_userinfo', path: 'userinfo' }).length <= 40, true);

// scrubHistory: after it, undo and redo cannot bring the typed value back; a short value only
// matches a whole text.
const model = ED.model.create(plain(doc()));
model.setParam('n_aaaaaaaa', 'url', 'https://api.example.com/v1?q=1');
assert.equal(model.canUndo(), true);
assert.ok(model.scrubHistory(TOKEN) >= 1);
model.undo();
assert.ok(!JSON.stringify(model.toJSON()).includes(TOKEN), 'undo brought the token back');
assert.ok(JSON.stringify(model.toJSON()).includes('[redacted]'));
model.redo();
assert.ok(!JSON.stringify(model.toJSON()).includes(TOKEN));
assert.equal(model.scrubHistory('ab'), 0);

// The move against a stub server: the request carries no value, the model takes the returned
// params as one change, the history is clean, and only a publish after a move offers the purge.
const sent = [], notes = [], offers = [];
const m2 = ED.model.create(plain(doc()));
const served = doc(); served.nodes[0].params = { url: 'https://api.example.com/v1?q=1', auth_secret: 'my_key', auth_in: 'query', auth_query: 'api_key' };
m2.setParam('n_aaaaaaaa', 'q_test', 'x'); // some earlier history
const ed = {
    t: key => key, esc: s => s, model: m2, flow: { id: 'flow_x' }, issues: [], bus: { emit() {} }, readonly: false,
    ctx: { notify: n => notes.push(n), confirmDialog: async () => true },
    flushPendingEdits: () => true,
    saver: { revision: 1, flush: async () => true, beginExternal: () => ({}), releaseExternal() {}, recoverExternal: async () => false,
        acknowledgeExternal: (ticket, rev, apply) => { assert.equal(rev, 2); return apply() !== false; } },
    api: { moveCredential: async (id, body) => { sent.push(body); return { draft_revision: 2, doc: served, moved: ['n_aaaaaaaa'], not_movable: [], issues: [] }; } }
};
const mv = ED.credentialsMove.create(ed);
const finding = ED.credentials.findings(m2.node('n_aaaaaaaa'))[0];
assert.equal(finding.location, 'url_query');
assert.equal(await mv.move(finding, 'n_aaaaaaaa', 'my_key', false), true);
assert.ok(!JSON.stringify(sent).includes(TOKEN), 'the request carries the value');
assert.deepEqual(plain(sent[0]), { revision: 1, node_id: 'n_aaaaaaaa', location: 'url_query', path: 'api_key', secret_name: 'my_key', replace: false });
assert.equal(m2.node('n_aaaaaaaa').params.auth_secret, 'my_key');
for (let i = 0; i < 5 && m2.canUndo(); i++) { m2.undo(); assert.ok(!JSON.stringify(m2.toJSON()).includes(TOKEN), 'undo step ' + i + ' brought the token back'); }
assert.equal(notes.length, 1);
box.window.EasyDrag.credentialsPurge = { offer: id => offers.push(id) };
mv.afterPublish();
mv.afterPublish();
assert.deepEqual(offers, ['flow_x'], 'the purge is offered once, after the first publish that follows a move');
console.log('PASS credential move');
```
Run it and watch it fail:
```
cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-s0b-credentials && node scripts/test-easydrag-credentials-move.mjs
```
Expected: it throws `ENOENT ... easydrag-credentials.js` (B.5 not done yet) or, once that exists, `ENOENT ... easydrag-credentials-move.js`.

- [ ] **Step 2: `scrubHistory` in the model**

In `ui/js/desktop/apps/easydrag-model.js`, add before `function replaceDoc(doc) {`:

```js
        // scrubHistory replaces a moved credential by the redaction marker in every undo and redo
        // snapshot, so undo cannot bring the typed value back. A value of 6 or more characters is
        // replaced wherever it occurs inside a text (a query value sits inside an address); a
        // shorter one only where a whole text equals it, so it cannot damage unrelated text. It
        // returns how many texts changed.
        function scrubHistory(value) {
            const literal = String(value == null ? '' : value);
            if (!literal) return 0;
            let hits = 0;
            const clean = v => {
                if (typeof v === 'string') {
                    const next = v === literal ? '[redacted]' : literal.length >= 6 ? v.split(literal).join('[redacted]') : v;
                    if (next !== v) hits++;
                    return next;
                }
                if (Array.isArray(v)) return v.map(clean);
                if (v && typeof v === 'object') { const out = {}; Object.keys(v).forEach(k => { out[k] = clean(v[k]); }); return out; }
                return v;
            };
            undoStack.concat(redoStack).forEach(set => {
                ['nodes', 'edges'].forEach(list => (set[list] || []).forEach(entry => { entry.before = clean(entry.before); entry.after = clean(entry.after); }));
                if (set.meta) { set.meta.before = clean(set.meta.before); set.meta.after = clean(set.meta.after); }
            });
            return hits;
        }

```
and add `scrubHistory,` to the returned object (after `replaceDoc,` in the last line of the object: `toggleDisabled, setFlow, fragment, paste, duplicate, replaceDoc, scrubHistory, group, ...`).

- [ ] **Step 3: the move module**

Create `ui/js/desktop/apps/easydrag-credentials-move.js`:

```js
// EasyDrag: move a typed-in credential into the vault (FD-13). The detail view's credential
// warning offers "move to the vault" when the server marks it movable, else "remove value".
// The value is read from the draft here (ED.credentials) and never sent: the server finds it
// again by node, location and path.
(function () {
    'use strict';

    const ED = window.EasyDrag = window.EasyDrag || {};
    const NAME = /^[a-z0-9_]{1,40}$/;

    function sanitize(text) { return String(text || '').toLowerCase().replace(/[^a-z0-9]+/g, '_').replace(/^_+|_+$/g, ''); }

    // proposeName builds a secret name from the host and the key of a finding, at most 40 characters.
    function proposeName(node, finding) {
        let host = '';
        try { host = new URL(String((node.params || {}).url || '').trim()).hostname; } catch (err) { host = ''; }
        const key = finding.location === 'url_userinfo' ? 'basic' : finding.path;
        return [host, key].map(sanitize).filter(Boolean).join('_').slice(0, 40).replace(/_+$/, '') || 'secret';
    }

    function create(ed) {
        const { t, esc } = ed;
        const core = ED.core;
        let movedInSession = false;
        let busy = false;

        function find(nodeId, issue) {
            const node = ed.model.node(nodeId);
            return node && ED.credentials.findings(node).find(f => f.param === issue.param && f.location === issue.location && f.path === issue.path);
        }

        // places counts the findings the server moves together: the same value in the same kind of place.
        function places(finding) {
            return ed.model.doc.nodes.reduce((sum, node) => sum + ED.credentials.findings(node).filter(f => f.movable && f.location === finding.location && f.value === finding.value).length, 0);
        }

        // applyDoc takes the params of the returned document as one change, so one undo step covers it.
        function applyDoc(served) {
            return ed.model.change(t('easydrag.ui.credential_move'), rec => {
                served.nodes.forEach(n => {
                    const current = ed.model.node(n.id);
                    if (current && JSON.stringify(current.params) !== JSON.stringify(n.params)) rec.node(n.id).params = JSON.parse(JSON.stringify(n.params));
                });
            });
        }

        function notify(message, type) { ed.ctx.notify({ title: t('easydrag.ui.credential_move'), message, type }); }

        // move saves, asks the server and applies its answer. It resolves to true when the dialog may close.
        async function move(finding, nodeId, name, replace) {
            if (busy) return false;
            if (ed.flushPendingEdits && ed.flushPendingEdits() === false) return false;
            if (!(await ed.saver.flush())) { notify(t('easydrag.ui.publish_unsaved'), 'error'); return false; }
            const revision = ed.saver.revision;
            const ticket = ed.saver.beginExternal(revision, ed.model.version);
            if (!ticket) { notify(t('easydrag.ui.publish_unsaved'), 'error'); return false; }
            busy = true;
            let res;
            try {
                res = await ed.api.moveCredential(ed.flow.id, { revision, node_id: nodeId, location: finding.location, path: finding.path, secret_name: name, replace });
            } catch (err) {
                busy = false;
                const status = Number(err && err.status);
                if (core.errorCode(err) && status >= 400 && status < 500 && ![408, 429].includes(status)) ed.saver.releaseExternal(ticket);
                else await ed.saver.recoverExternal(ticket); // uncertain: the saver's conflict flow reloads the server draft
                if (core.errorCode(err) === 'FLOW_SECRET_EXISTS' && !replace) {
                    return (await ed.ctx.confirmDialog(t('easydrag.ui.credential_move'), t('easydrag.ui.credential_replace_confirm', { name }))) ? move(finding, nodeId, name, true) : false;
                }
                notify(core.errorText(t, err), 'error');
                return false;
            }
            busy = false;
            if (!ed.saver.acknowledgeExternal(ticket, res.draft_revision, () => applyDoc(res.doc), res.issues || [])) {
                await ed.saver.recoverExternal(ticket);
                return true;
            }
            ed.model.scrubHistory(finding.value);
            ed.issues = res.issues || [];
            ed.bus.emit('issues', ed.issues);
            movedInSession = true;
            notify(t('easydrag.ui.credential_moved'));
            return true;
        }

        function openMove(nodeId, issue) {
            const finding = find(nodeId, issue);
            if (!finding || busy) return;
            const node = ed.model.node(nodeId);
            const dialog = core.modal(ed.root, {
                title: t('easydrag.ui.credential_move'), closeLabel: t('easydrag.ui.close'),
                body: '<p>' + esc(t('easydrag.ui.credential_move_text', { count: places(finding) })) + '</p><label class="ed-label">' + esc(t('easydrag.ui.credential_move_name')) +
                    '<input class="ed-input" data-ed-credential-name maxlength="40" autocomplete="off" spellcheck="false"></label><p class="ed-error" role="alert" hidden></p>',
                actions: [{ id: 'cancel', label: t('easydrag.ui.cancel') }, { id: 'move', label: t('easydrag.ui.credential_move'), primary: true }],
                onAction: async (action, d) => {
                    if (action !== 'move') return true;
                    const name = d.body.querySelector('[data-ed-credential-name]').value.trim();
                    if (!NAME.test(name)) {
                        const error = d.body.querySelector('.ed-error');
                        error.textContent = t('easydrag.ui.secret_invalid');
                        error.hidden = false;
                        return false;
                    }
                    return move(finding, nodeId, name, false);
                }
            });
            dialog.body.querySelector('[data-ed-credential-name]').value = proposeName(node, finding);
        }

        // removeValue clears the typed value in the client (the warning offers it when moving is not possible).
        function removeValue(nodeId, issue) {
            const node = ed.model.node(nodeId);
            const finding = node && find(nodeId, issue);
            if (!finding) return;
            ed.model.setParam(nodeId, finding.param, ED.credentials.remove(node, finding)[finding.param]);
            ed.model.scrubHistory(finding.value);
        }

        // decorate puts the action on a credential warning (the .ed-field-issue box of the detail form).
        function decorate(box, issue, node) {
            if (!box) return;
            const old = box.querySelector('[data-ed-credential-action]');
            if (old) old.remove();
            if (!issue || issue.code !== 'PARAM_LITERAL_CREDENTIAL' || !node || ed.readonly || ed.runView) return;
            const kind = issue.movable ? 'move' : 'remove';
            const button = core.el('<button type="button" class="ed-btn ed-btn--small" data-ed-credential-action="' + kind + '">' + esc(t(kind === 'move' ? 'easydrag.ui.credential_move' : 'easydrag.ui.credential_remove')) + '</button>');
            button.addEventListener('click', () => (kind === 'move' ? openMove(node.id, issue) : removeValue(node.id, issue)));
            box.appendChild(button);
        }

        // afterPublish offers the guided purge once, after the first publish that follows a move.
        function afterPublish() {
            if (!movedInSession) return;
            movedInSession = false;
            ED.credentialsPurge && ED.credentialsPurge.offer(ed.flow.id);
        }

        return { decorate, move, afterPublish };
    }

    ED.credentialsMove = { create, proposeName };
})();
```

- [ ] **Step 4: hooks (a few lines each)**

1. `ui/js/desktop/apps/easydrag-core.js`, in `createApi`, after the `extractReconcile` line add:

```js
            moveCredential: (id, body) => api(flow(id) + '/credentials/move', json('POST', body)),
```
2. `ui/js/desktop/apps/easydrag-forms.js`: in `build()`, directly after the line `const control = controlFor(param, value, id, labelId);` add:

```js
                if (issue && env.ed.credentialsMove) env.ed.credentialsMove.decorate(field.querySelector('.ed-field-issue'), issue, node);
```
and in `syncIssues()`, directly after the line `box.querySelector('span').textContent = core.issueText(t, issue);` add:

```js
                if (env.ed.credentialsMove) env.ed.credentialsMove.decorate(box, issue, node);
```
3. `ui/js/desktop/apps/easydrag-editor.js`: after the line `extraction = !isBlock && ED.extraction && ED.extraction.create(ed, canvas);` (line ~140) add:

```js
        ed.credentialsMove = !isBlock && ED.credentialsMove ? ED.credentialsMove.create(ed) : null;
```
and in the `bag.add(ed.bus.on('published', flow => {` handler add as its last statement (before `}));`):

```js
            if (ed.credentialsMove) ed.credentialsMove.afterPublish();
```
4. `ui/css/desktop-app-easydrag.css`: after the line `.ed-field-issue .ed-icon { width: 14px; height: 14px; }` (~1031) add:

```css
.ed-field-issue .ed-btn { margin-left: auto; flex: none; }
```
5. Register the module. In `ui/js/desktop/core/module-loader.js` replace `'/js/desktop/apps/easydrag-publish.js',\n                '/js/desktop/apps/easydrag-library-pause.js',` by the same two lines with `'/js/desktop/apps/easydrag-credentials-move.js',` between them (same indentation). In `ui/desktop_easydrag_test.go` change `"easydrag-publish.js",` + newline + `"easydrag-library-pause.js"` (in `easyDragModules`) the same way: insert `"easydrag-credentials-move.js",` between them.
6. Add the Node test to the runner: in `scripts/test-easydrag.mjs`, after the line `await import('./test-easydrag-approvals.mjs');` add `await import('./test-easydrag-credentials-move.mjs');`.
7. Find every Node harness that loads the editor modules and add the new file after `easydrag-publish.js`: `cd <worktree> && grep -ln "easydrag-publish.js" scripts/*.mjs` (expected: `test-easydrag-extra2.mjs`, `test-easydrag-extra3.mjs`). In each list, insert `'easydrag-credentials-move.js'` right after `'easydrag-publish.js'`.

- [ ] **Step 5: locale keys (6 keys)**

Write `C:/disposable/easydrag-s0b/lang-b4.mjs` (key order: move, remove, text, name, moved, replace_confirm), then run the helper:

```js
export default {
    keys: ['easydrag.ui.credential_move', 'easydrag.ui.credential_remove', 'easydrag.ui.credential_move_text', 'easydrag.ui.credential_move_name', 'easydrag.ui.credential_moved', 'easydrag.ui.credential_replace_confirm'],
    langs: {
        en: ['Move to the vault', 'Remove value', 'The value goes into the vault and is replaced in this flow. Places affected: {count}.', 'Secret name', 'Moved. Publish the flow, then you can clean up old copies.', 'A secret named {name} already exists. Replace its value?'],
        de: ['In den Tresor verschieben', 'Wert entfernen', 'Der Wert kommt in den Tresor und wird in diesem Flow ersetzt. Betroffene Stellen: {count}.', 'Name des Geheimnisses', 'Verschoben. Veröffentliche den Flow, dann kannst du alte Kopien bereinigen.', 'Ein Geheimnis namens {name} gibt es schon. Soll sein Wert ersetzt werden?'],
        cs: ['Přesunout do trezoru', 'Odstranit hodnotu', 'Hodnota se uloží do trezoru a v tomto flow se nahradí. Dotčená místa: {count}.', 'Název tajemství', 'Přesunuto. Publikuj flow, potom můžeš vyčistit staré kopie.', 'Tajemství s názvem {name} už existuje. Nahradit jeho hodnotu?'],
        da: ['Flyt til tresoren', 'Fjern værdi', 'Værdien lægges i tresoren og erstattes i dette flow. Berørte steder: {count}.', 'Navn på hemmeligheden', 'Flyttet. Udgiv flowet, så kan du rydde op i gamle kopier.', 'Der findes allerede en hemmelighed med navnet {name}. Skal værdien erstattes?'],
        el: ['Μετακίνηση στο θησαυροφυλάκιο', 'Αφαίρεση τιμής', 'Η τιμή αποθηκεύεται στο θησαυροφυλάκιο και αντικαθίσταται σε αυτή τη ροή. Επηρεαζόμενα σημεία: {count}.', 'Όνομα μυστικού', 'Μετακινήθηκε. Δημοσίευσε τη ροή και μετά μπορείς να καθαρίσεις τα παλιά αντίγραφα.', 'Υπάρχει ήδη μυστικό με όνομα {name}. Να αντικατασταθεί η τιμή του;'],
        es: ['Mover a la bóveda', 'Quitar valor', 'El valor pasa a la bóveda y se sustituye en este flujo. Lugares afectados: {count}.', 'Nombre del secreto', 'Movido. Publica el flujo y luego podrás limpiar las copias antiguas.', 'Ya existe un secreto llamado {name}. ¿Sustituir su valor?'],
        fr: ['Déplacer dans le coffre', 'Retirer la valeur', 'La valeur va dans le coffre et est remplacée dans ce flux. Endroits concernés : {count}.', 'Nom du secret', 'Déplacé. Publie le flux, puis tu pourras nettoyer les anciennes copies.', 'Un secret nommé {name} existe déjà. Remplacer sa valeur ?'],
        hi: ['तिजोरी में ले जाओ', 'मान हटाओ', 'मान तिजोरी में रखा जाएगा और इस फ़्लो में बदल दिया जाएगा। प्रभावित स्थान: {count}।', 'गुप्त मान का नाम', 'ले जाया गया। फ़्लो प्रकाशित करो, फिर पुरानी प्रतियाँ साफ़ कर सकते हो।', '{name} नाम का गुप्त मान पहले से है। उसका मान बदल दें?'],
        it: ['Sposta nel caveau', 'Rimuovi il valore', 'Il valore va nel caveau e viene sostituito in questo flusso. Punti interessati: {count}.', 'Nome del segreto', 'Spostato. Pubblica il flusso, poi potrai ripulire le vecchie copie.', 'Esiste già un segreto chiamato {name}. Sostituire il suo valore?'],
        ja: ['保管庫へ移す', '値を削除', '値は保管庫に保存され、このフローでは置き換えられます。対象箇所：{count}', 'シークレット名', '移しました。フローを公開すると、古いコピーを整理できます。', '{name} というシークレットは既にあります。値を置き換えますか？'],
        nl: ['Naar de kluis verplaatsen', 'Waarde verwijderen', 'De waarde gaat naar de kluis en wordt in deze flow vervangen. Betrokken plekken: {count}.', 'Naam van het geheim', 'Verplaatst. Publiceer de flow, dan kun je oude kopieën opruimen.', 'Er bestaat al een geheim met de naam {name}. De waarde vervangen?'],
        no: ['Flytt til hvelvet', 'Fjern verdi', 'Verdien legges i hvelvet og erstattes i denne flyten. Berørte steder: {count}.', 'Navn på hemmeligheten', 'Flyttet. Publiser flyten, så kan du rydde opp i gamle kopier.', 'Det finnes allerede en hemmelighet som heter {name}. Vil du erstatte verdien?'],
        pl: ['Przenieś do skarbca', 'Usuń wartość', 'Wartość trafi do skarbca i zostanie zastąpiona w tym przepływie. Zmienione miejsca: {count}.', 'Nazwa sekretu', 'Przeniesiono. Opublikuj przepływ, a potem możesz wyczyścić stare kopie.', 'Sekret o nazwie {name} już istnieje. Zastąpić jego wartość?'],
        pt: ['Mover para o cofre', 'Remover valor', 'O valor vai para o cofre e é substituído neste fluxo. Locais afetados: {count}.', 'Nome do segredo', 'Movido. Publique o fluxo e depois você poderá limpar as cópias antigas.', 'Já existe um segredo chamado {name}. Substituir o valor dele?'],
        sv: ['Flytta till kassaskåpet', 'Ta bort värdet', 'Värdet läggs i kassaskåpet och ersätts i det här flödet. Berörda ställen: {count}.', 'Hemlighetens namn', 'Flyttat. Publicera flödet, sedan kan du rensa gamla kopior.', 'Det finns redan en hemlighet som heter {name}. Ersätta dess värde?'],
        zh: ['移入保险库', '删除该值', '该值会存入保险库，并在此流程中被替换。涉及位置：{count}。', '密钥名称', '已移入。发布流程后，你可以清理旧副本。', '名为 {name} 的密钥已存在。要替换它的值吗？']
    }
};
```
```
node C:/disposable/easydrag-s0b/lang-add.mjs C:/disposable/easydrag-s0b/lang-b4.mjs C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-s0b-credentials
```
Expected: `added 6 keys to 16 files`.

- [ ] **Step 6: run the Node and Go checks**

```
cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-s0b-credentials && node scripts/test-easydrag-credentials-move.mjs
cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-s0b-credentials && node scripts/test-easydrag.mjs
cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-s0b-credentials && go test ./ui/ -run '(?i)(EasyDrag)' -count=1
```
Expected: `PASS credential move`; `all EasyDrag checks passed`; `ok  aurago/ui`. If `easydrag-core.js` is needed by a harness that now loads the move module without `easydrag-credentials.js`, the error is `ED.credentials is undefined`: add `'easydrag-credentials.js'` after `'easydrag-template.js'` in that list (Task B.5 step 8 lists them). If the Go test reports a line-count warning for `easydrag-editor.js`, check that it is still below 900 (`wc -l ui/js/desktop/apps/easydrag-editor.js` must print 896).

- [ ] **Step 7: the browser fixture and the gated move journey**

Create `ui/testdata/easydrag-credentials-fixture.js` (layered after `easydrag-fixture.js`; it adds the credential warning to every validation answer and plays the server for the move):

```js
// Browser-only credential fixture: adds PARAM_LITERAL_CREDENTIAL (a typed api_key in an http.request
// address) to the validation answers and answers POST .../credentials/move the way the server does.
(function () {
    'use strict';
    const previous = window.fetch.bind(window);
    const base = '/api/desktop/flows';
    const state = { vault: {}, moves: [] };
    const reply = (value, status = 200) => new Response(JSON.stringify(value), { status, headers: { 'Content-Type': 'application/json' } });
    const queryHits = doc => {
        const out = [];
        (doc.nodes || []).forEach(n => {
            const url = n.type === 'http.request' && n.params && typeof n.params.url === 'string' ? n.params.url : '';
            (url.split('?')[1] || '').split('&').forEach(pair => {
                const [key, value] = pair.split('=');
                if (key === 'api_key' && value && !value.includes('{{') && value !== '[redacted]') out.push({ node: n, key });
            });
        });
        return out;
    };
    const issuesOf = doc => queryHits(doc).map(h => ({ code: 'PARAM_LITERAL_CREDENTIAL', severity: 'warning', node_id: h.node.id, param: 'url', location: 'url_query', path: h.key, movable: !h.node.params.auth_secret, message: 'typed credential' }));
    async function withIssues(response, doc) {
        const body = response.ok && doc ? await response.clone().json().catch(() => null) : null;
        if (!body || !Array.isArray(body.issues)) return response;
        return reply(Object.assign(body, { issues: body.issues.concat(issuesOf(doc)) }), response.status);
    }
    function move(rec, body) {
        if (body.revision !== rec.draft_revision) return reply({ code: 'FLOW_REVISION_CONFLICT', error: 'conflict' }, 409);
        if (state.vault[body.secret_name] !== undefined && !body.replace) return reply({ code: 'FLOW_SECRET_EXISTS', error: 'exists' }, 409);
        const node = rec.draft.nodes.find(n => n.id === body.node_id);
        const [head, query = ''] = node.params.url.split('?');
        const kept = [], taken = [];
        query.split('&').forEach(pair => (pair.split('=')[0] === body.path ? taken : kept).push(pair));
        state.vault[body.secret_name] = taken[0].split('=')[1];
        state.moves.push({ name: body.secret_name, node: body.node_id, path: body.path });
        node.params.url = kept.length ? head + '?' + kept.join('&') : head;
        Object.assign(node.params, { auth_secret: body.secret_name, auth_in: 'query', auth_query: body.path });
        rec.draft_revision += 1;
        return reply({ draft_revision: rec.draft_revision, doc: rec.draft, moved: [body.node_id], not_movable: [], issues: issuesOf(rec.draft) });
    }
    window.fetch = async (input, options) => {
        const url = new URL(String(input), location.origin), opts = options || {}, method = String(opts.method || 'GET').toUpperCase();
        if (!url.pathname.startsWith(base)) return previous(input, opts);
        const parts = url.pathname.slice(base.length).split('/').filter(Boolean).map(decodeURIComponent);
        const entry = parts[0] && window.edFixture.state.flows.get(parts[0]);
        if (method === 'POST' && entry && parts[1] === 'credentials' && parts[2] === 'move') return move(entry.rec, JSON.parse(opts.body));
        const response = await previous(input, opts);
        if (method === 'POST' && parts[0] === 'validate') return withIssues(response, JSON.parse(opts.body).doc);
        if (entry && (parts.length === 1 || parts[1] === 'publish-preview')) return withIssues(response, entry.rec.draft);
        return response;
    };
    window.edCredentialsFixture = { state };
})();
```
Create `ui/desktop_easydrag_credentials_browser_test.go`:

```go
package ui

import (
	"testing"

	"github.com/go-rod/rod/lib/proto"
)

// TestDesktopEasyDragCredentialMoveBrowser walks the real controls: the credential warning of an
// address, "move to the vault" with the proposed name, the editor with no trace of the token (model,
// undo history, localStorage), and the purge offer after the next publish.
func TestDesktopEasyDragCredentialMoveBrowser(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	s := startEasyDragSmoke(t, "/testdata/easydrag-credentials-fixture.js")
	p := s.page
	if err := (proto.EmulationSetEmulatedMedia{Features: []*proto.EmulationMediaFeature{{Name: "prefers-reduced-motion", Value: "reduce"}}}).Call(p); err != nil {
		t.Fatal(err)
	}
	const token = "tok-browser-1234567890"
	s.wait(`()=>!!document.querySelector('.ed-flow-card h3')`)
	p.MustElement(`.ed-flow-card h3`).MustClick()
	s.wait(`()=>!!edFixture.editor() && !!document.querySelector('[data-ed-cmd="test"]')`)
	id := p.MustEval(`token=>{window.__purge=[];window.EasyDrag.credentialsPurge={offer:id=>window.__purge.push(id)};return edFixture.editor().model.addNode('http.request',{x:1280,y:120},{method:'GET',url:'https://api.example.com/v1?api_key='+token+'&q=1'},{node:'n_pdf',port:'out'});}`, token).Str()
	p.MustEval(`id=>{window.__credNode=id}`, id)
	s.wait(`()=>{const last=edFixture.lastSave();return !!last && last.nodes.some(n=>n.type==='http.request')}`)
	p.MustEval(`()=>edFixture.editor().bus.emit('open-detail',{nodeId:window.__credNode})`)
	action := `.ed-detail .ed-field[data-param="url"] [data-ed-credential-action="move"]`
	s.wait(`()=>!!document.querySelector('` + action + `')`)
	p.MustElement(action).MustClick()
	s.wait(`()=>{const i=document.querySelector('.ed-modal [data-ed-credential-name]');return !!i && i.value==='api_example_com_api_key'}`)
	s.shot("credential-move-dialog")
	p.MustElement(`.ed-modal [data-ed-action="move"]`).MustClick()
	s.wait(`()=>edCredentialsFixture.state.moves.length===1 && !document.querySelector('.ed-modal-backdrop:not(.is-closing)')`)

	// The editor holds no trace of the token: the model, every undo step, the emergency copy.
	s.wait(`()=>{const ed=edFixture.editor(),n=ed.model.node(window.__credNode);return n.params.auth_secret==='api_example_com_api_key' && n.params.url==='https://api.example.com/v1?q=1' && ed.saver.state==='saved'}`)
	clean := p.MustEval(`token=>{const ed=edFixture.editor();let ok=!JSON.stringify(ed.model.toJSON()).includes(token);for(let i=0;i<10&&ed.model.canUndo();i++){ed.model.undo();ok=ok&&!JSON.stringify(ed.model.toJSON()).includes(token);}while(ed.model.canRedo())ed.model.redo();return ok&&!JSON.stringify(Object.entries(localStorage)).includes(token)}`, token).Bool()
	if !clean {
		t.Fatal("the token is still in the model, its undo history or localStorage")
	}
	s.failOnPageErrors("credential move")
	s.noRawKeys("credential move")

	// The next publish offers the guided purge, once.
	p.MustElement(`[data-ed-cmd="publish"]`).MustClick()
	s.wait(`()=>!!document.querySelector('.ed-modal [data-ed-action="publish"]')`)
	p.MustElement(`.ed-modal [data-ed-action="publish"]`).MustClick()
	s.wait(`()=>window.__purge.length===1`)
	if got := p.MustEval(`()=>window.__purge[0]`).Str(); got != p.MustEval(`()=>edFixture.editor().flow.id`).Str() {
		t.Fatalf("purge offered for %q", got)
	}
}
```
Run it (gated; Chrome per `reference_chrome_launch_from_agent.md` if it cannot start from the test binary):

```
cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-s0b-credentials && AURAGO_RUN_BROWSER_SMOKE=1 AURAGO_BROWSER_ARTIFACT_DIR=D:/tmp/easydrag-s0b-shots go test ./ui/ -run 'TestDesktopEasyDragCredentialMoveBrowser' -count=1 -timeout 10m
```
Expected: `ok  aurago/ui` and `D:/tmp/easydrag-s0b-shots/credential-move-dialog.png`. If the action never appears, print the issues: `p.MustEval("()=>JSON.stringify(edFixture.editor().issues)")`; the fixture must have added `PARAM_LITERAL_CREDENTIAL` (validation runs 700 ms after a change). If `ed.saver.state` is `conflict`, the fixture revision handling is off: check that `move()` bumps `rec.draft_revision` exactly once.

- [ ] **Step 8: gates and commit**

```
cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-s0b-credentials && node scripts/test-easydrag.mjs && node scripts/test-easydrag-render-baseline.mjs && gofmt -l ui/desktop_easydrag_credentials_browser_test.go ui/desktop_easydrag_test.go && go vet ./ui/
```
Expected: `all EasyDrag checks passed`, `PASS render baseline (unchanged)`, no gofmt or vet output. Write `C:/disposable/easydrag-s0b/b1-4.txt`:

```
feat(easydrag): move a typed credential into the vault from the warning

The detail view's credential warning offers "move to the vault" (movable) or
"remove value". The dialog proposes a name from host and key; on success the
editor applies the returned params as one change, scrubs the undo history of
the value and offers the purge after the next publish. ED.model.scrubHistory
replaces a moved value in every undo and redo snapshot.

GitNexus detect-changes unavailable in the worktree; judged by symbols and tests.

Co-Authored-By: <your model line>
```
```
cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-s0b-credentials && git add ui/js/desktop/apps/easydrag-credentials-move.js ui/js/desktop/apps/easydrag-model.js ui/js/desktop/apps/easydrag-core.js ui/js/desktop/apps/easydrag-forms.js ui/js/desktop/apps/easydrag-editor.js ui/css/desktop-app-easydrag.css ui/js/desktop/core/module-loader.js ui/desktop_easydrag_test.go ui/testdata/easydrag-credentials-fixture.js ui/desktop_easydrag_credentials_browser_test.go scripts/test-easydrag-credentials-move.mjs scripts/test-easydrag.mjs scripts/test-easydrag-extra2.mjs scripts/test-easydrag-extra3.mjs ui/lang/easydrag && git commit -F C:/disposable/easydrag-s0b/b1-4.txt
```

---

### Task B.5: client masking (do this before Task B.4)

**Files**
- Create: `ui/js/desktop/apps/easydrag-credentials.js`, `scripts/test-easydrag-credentials.mjs`
- Modify: `ui/js/desktop/apps/easydrag-saver.js:113` (emergency copy), `ui/js/desktop/apps/easydrag-interact.js:568` (clipboard), `ui/js/desktop/apps/easydrag-dialogs.js:296-304` (restore notice), `ui/js/desktop/core/module-loader.js`, `ui/desktop_easydrag_test.go` (`easyDragModules`), `scripts/test-easydrag.mjs`, the Node harness lists, `ui/lang/easydrag/*.json` (1 key)

**The mirror.** The module is a line-for-line port of `internal/flows/credentials.go`: same name rules, same token regexp, same URL cut (`splitURL` = `splitCredentialURL`), same word split (`[ \t\r\n\f]`, not `\s`), same JSON pointer escaping, same sorted key order. Template detection uses `ED.template` (`refs` and `segments`), which is already a port of the server parser. The shared vectors prove the match; avoid number-formatting differences in new vectors (Go keeps `json.Number` text, JS reformats numbers).

- [ ] **Step 1: verify what the remembered test data holds**

```
cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-s0b-credentials && grep -n "test-trigger" ui/js/desktop/apps/easydrag-run-start.js
```
Expected: line 92 reads the key and line 194 writes `trigger.id` (a node id). That is not test data, so nothing is masked there (the typed test data lives on the server in `flow_test_data`, which Part B2's purge redacts). Report this in the commit message. If line 194 stores anything else than an id, stop and tell the owner.

- [ ] **Step 2: write the failing Node test**

Create `scripts/test-easydrag-credentials.mjs`:

```js
// B.5: the client credential rule against the same vectors as the Go rule
// (internal/flows/testdata/credential_cases.json), and the masking sites (emergency copy, clipboard).
import assert from 'node:assert/strict';
import fs from 'node:fs';
import vm from 'node:vm';
import { webcrypto } from 'node:crypto';

const apps = new URL('../ui/js/desktop/apps/', import.meta.url);
const store = new Map();
const box = vm.createContext({
    window: {}, console, Intl, Date, Set, Map, crypto: webcrypto, navigator: { platform: 'Linux' },
    localStorage: { get length() { return store.size; }, key: i => Array.from(store.keys())[i] ?? null, getItem: k => (store.has(k) ? store.get(k) : null), setItem: (k, v) => { store.set(k, String(v)); }, removeItem: k => { store.delete(k); } },
    setTimeout: () => 1, clearTimeout: () => {}
});
for (const name of ['easydrag-core.js', 'easydrag-template.js', 'easydrag-credentials.js', 'easydrag-saver.js']) {
    const file = new URL(name, apps);
    vm.runInContext(fs.readFileSync(file, 'utf8'), box, { filename: file.pathname });
}
const ED = box.window.EasyDrag;
const plain = value => JSON.parse(JSON.stringify(value));

const cases = JSON.parse(fs.readFileSync(new URL('../internal/flows/testdata/credential_cases.json', import.meta.url), 'utf8'));
assert.ok(cases.length >= 18, 'the shared vectors shrank');
for (const c of cases) {
    const node = Object.assign({}, c.node, { params: plain(c.node.params) });
    const found = plain(ED.credentials.findings(node)).map(f => ({ param: f.param, location: f.location, path: f.path, movable: f.movable }));
    assert.deepEqual(found, c.findings, c.name + ': findings');
    assert.ok(ED.credentials.findings(node).every(f => f.value), c.name + ': every finding holds its value');
    assert.deepEqual(plain(ED.credentials.maskParams(node.type, node.params)), c.masked_params, c.name + ': masked params');
    assert.deepEqual(plain(node.params), plain(c.node.params), c.name + ': the input was changed');
}

// remove clears exactly one finding (the warning's "remove value"), mask handles a whole document.
const call = { id: 'n1', type: 'http.request', params: { url: 'https://u:pw1234@api.example.com/v1?api_key=abc123&q=1', headers: { Authorization: 'Bearer abc123def456' } } };
const urlHit = ED.credentials.findings(call).find(f => f.location === 'url_query');
assert.equal(plain(ED.credentials.remove(call, urlHit)).url, 'https://u:pw1234@api.example.com/v1?api_key=&q=1');
const userHit = ED.credentials.findings(call).find(f => f.location === 'url_userinfo');
assert.equal(plain(ED.credentials.remove(call, userHit)).url, 'https://api.example.com/v1?api_key=abc123&q=1');
const headerHit = ED.credentials.findings(call).find(f => f.location === 'header');
assert.equal(plain(ED.credentials.remove(call, headerHit)).headers.Authorization, '');
const masked = plain(ED.credentials.mask({ name: 'x', nodes: [call], edges: [] }));
assert.ok(!JSON.stringify(masked).includes('abc123') && !JSON.stringify(masked).includes('pw1234'));
assert.equal(ED.credentials.hasFindings({ nodes: [call] }), true);
assert.equal(ED.credentials.hasFindings({ nodes: [{ id: 'n2', type: 'logic.set', params: {} }] }), false);

// The emergency copy never holds a token, says so, and offers the flag to the restore dialog.
let version = 1;
const doc = { schema: 1, name: 'x', nodes: [Object.assign({ key: 'call', label: 'Call', position: { x: 0, y: 0 }, settings: {} }, plain(call))], edges: [] };
const model = { get version() { return version; }, toJSON: () => plain(doc), replaceDoc() {} };
const saver = ED.saver.create({ api: { save: () => new Promise(() => {}) }, flowId: 'f1', model, revision: 1, onState() {}, onSaved() {}, onInvalid() {}, onConflict: () => new Promise(() => {}) });
version++;
saver.dispose(); // writes the unsaved draft
const copy = JSON.parse(store.get('aurago.easydrag.draft.f1'));
for (const secret of ['abc123def456', 'pw1234', 'api_key=abc123']) assert.ok(!JSON.stringify(copy).includes(secret), 'the emergency copy holds ' + secret);
assert.equal(copy.masked, true);
assert.equal(ED.saver.emergencyCopy('f1', 1).masked, true);

// The clipboard fragment is masked where it is written (the interact module needs a DOM, so the
// call is pinned in the source).
const interact = fs.readFileSync(new URL('easydrag-interact.js', apps), 'utf8');
assert.ok(interact.includes('frag: ED.credentials.mask(frag)'), 'the clipboard fragment is not masked');
console.log('PASS credential rule and masking');
```
Run it and watch it fail:
```
cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-s0b-credentials && node scripts/test-easydrag-credentials.mjs
```
Expected: it throws `ENOENT ... easydrag-credentials.js`.

- [ ] **Step 3: the module**

Create `ui/js/desktop/apps/easydrag-credentials.js`:

```js
// EasyDrag credential rule, client side. It mirrors internal/flows/credentials.go step by step
// (names, token formats, URL cut, word split, JSON pointers, key order) and is tested against
// the same vectors (internal/flows/testdata/credential_cases.json). Masking replaces each typed-in
// credential by the marker before anything is written to browser storage.
(function () {
    'use strict';

    const ED = window.EasyDrag = window.EasyDrag || {};
    const MARKER = '[redacted]';
    const EXACT = new Set(['token', 'accesstoken', 'refreshtoken', 'idtoken', 'apikey', 'secret', 'clientsecret', 'password', 'passwd', 'pwd', 'sig', 'signature',
        'auth', 'authorization', 'xapikey', 'privatekey', 'secretkey', 'accesskey', 'sessiontoken']);
    const SUFFIXES = ['token', 'secret', 'password', 'apikey'];
    const TOKEN = /^(?:(?:sk-|sk_live_|rk_live_)[A-Za-z0-9_-]{20,}|(?:ghp_|gho_|ghu_|ghs_|ghr_)[A-Za-z0-9]{16,}|github_pat_[A-Za-z0-9_]{16,}|glpat-[A-Za-z0-9_-]{16,}|xox[bpar]-[A-Za-z0-9-]{10,}|AKIA[0-9A-Z]{16}|AIza[0-9A-Za-z_-]{35}|eyJ[A-Za-z0-9_-]+\.[A-Za-z0-9_-]+\.[A-Za-z0-9_-]+)$/;
    const WORDS = /[ \t\r\n\f]+|[^ \t\r\n\f]+/g;
    const HEADER_EXACT = new Set(['authorization', 'proxy-authorization', 'cookie', 'x-api-key', 'api-key', 'apikey', 'x-auth-token', 'x-access-token']);
    const HEADER_PARTS = ['api-key', 'api_key', 'apikey', 'token', 'secret', 'password'];

    const empty = v => v == null || (typeof v === 'string' && v.trim() === '') || (Array.isArray(v) && !v.length) || (typeof v === 'object' && !Array.isArray(v) && !Object.keys(v).length);

    // hasTemplate: an unescaped "{{" (HasTemplate in template.go).
    function hasTemplate(s) {
        for (let i = 0; i + 1 < s.length; i++) {
            if (s[i] === '\\' && s.startsWith('{{', i + 1)) { i += 2; continue; }
            if (s[i] === '{' && s[i + 1] === '{') return true;
        }
        return false;
    }

    // templateOnly: nothing but templates and a bare scheme word is left (templateOnlyCredential).
    function templateOnly(s) {
        if (!hasTemplate(s) || ED.template.refs(s).some(r => r.error)) return false;
        const rest = ED.template.segments(s).filter(g => g.expr === undefined).map(g => g.literal).join('');
        return ['', 'bearer', 'basic', 'token'].includes(rest.trim().toLowerCase());
    }

    function normal(name) { return String(name).trim().toLowerCase().replace(/[-_]/g, ''); }
    function fieldName(name) { const n = normal(name); return EXACT.has(n) || SUFFIXES.some(s => n.endsWith(s)); }
    function queryKey(name) { return fieldName(name) || normal(name) === 'key'; }
    function isToken(s) { return s.length >= 16 && !s.includes('{{') && TOKEN.test(s); }
    function literalText(s) { return s.trim() !== '' && s !== MARKER && !hasTemplate(s); }
    function headerName(name) { const n = String(name).trim().toLowerCase(); return HEADER_EXACT.has(n) || HEADER_PARTS.some(p => n.includes(p)); }
    function literalHeader(s) { return s.trim() !== '' && s !== MARKER && !templateOnly(s); }
    function pointer(k) { return String(k).replace(/~/g, '~0').replace(/\//g, '~1'); }
    function unescapeQuery(s) { try { return decodeURIComponent(s.replace(/\+/g, ' ')); } catch (err) { return s; } }
    function urlPath(ptr, name) { return ptr === '' ? name : ptr + '?' + name; }

    // splitURL cuts an http:// or https:// word like splitCredentialURL; null for any other text.
    function splitURL(s) {
        const m = /^https?:\/\//i.exec(s);
        if (!m) return null;
        const u = { prefix: m[0], userinfo: '', hasUser: false, hostPath: '', query: '', hasQuery: false, fragment: '' };
        let rest = s.slice(m[0].length);
        let i = rest.indexOf('#');
        if (i >= 0) { u.fragment = rest.slice(i); rest = rest.slice(0, i); }
        i = rest.indexOf('?');
        if (i >= 0) { u.query = rest.slice(i + 1); u.hasQuery = true; rest = rest.slice(0, i); }
        let authority = rest, tail = '';
        i = rest.indexOf('/');
        if (i >= 0) { authority = rest.slice(0, i); tail = rest.slice(i); }
        i = authority.lastIndexOf('@');
        if (i >= 0) { u.userinfo = authority.slice(0, i); u.hasUser = true; authority = authority.slice(i + 1); }
        u.hostPath = authority + tail;
        return u;
    }
    function joinURL(u) { return u.prefix + (u.hasUser ? u.userinfo + '@' : '') + u.hostPath + (u.hasQuery ? '?' + u.query : '') + u.fragment; }

    function userinfoSecret(ui) {
        if (!literalText(ui)) return false;
        const c = ui.indexOf(':');
        return c >= 0 ? literalText(ui.slice(c + 1)) : isToken(ui);
    }

    // put records a hit and says what takes its place: the marker, or (remove mode, w.drop set)
    // an empty text for the one finding to clear and the original for every other.
    function put(w, hit, original) {
        w.hits.push(hit);
        if (!w.drop) return MARKER;
        return hit.location === w.drop.location && hit.path === w.drop.path ? '' : original;
    }

    function walk(w, v, ptr) {
        if (typeof v === 'string') return text(w, v, ptr);
        if (Array.isArray(v)) return v.map((item, i) => walk(w, item, ptr + '/' + i));
        if (v && typeof v === 'object') {
            const out = {};
            Object.keys(v).sort().forEach(k => { out[k] = entry(w, k, v[k], ptr); });
            return out;
        }
        return v;
    }

    function entry(w, k, val, ptr) {
        const p = ptr + '/' + pointer(k);
        if (typeof val === 'string' && fieldName(k) && literalText(val)) return put(w, { location: w.fields, path: p, value: val, top: false }, val);
        return walk(w, val, p);
    }

    function text(w, s, ptr) { return (s.match(WORDS) || []).map(tok => word(w, tok, ptr)).join(''); }

    function word(w, tok, ptr) {
        if (/^[ \t\r\n\f]/.test(tok)) return tok;
        const u = splitURL(tok);
        if (u) return urlText(w, u, ptr);
        if (isToken(tok)) return put(w, { location: 'format', path: ptr, value: tok, top: ptr === '' }, tok);
        return tok;
    }

    function urlText(w, u, ptr) {
        if (u.hasUser && userinfoSecret(u.userinfo)) {
            const r = put(w, { location: 'url_userinfo', path: urlPath(ptr, 'userinfo'), value: u.userinfo, top: ptr === '' }, u.userinfo);
            if (r === '') u.hasUser = false; else u.userinfo = r;
        }
        if (u.hasQuery) {
            u.query = u.query.split('&').map(pair => {
                const eq = pair.indexOf('=');
                if (eq < 0) return pair;
                const key = pair.slice(0, eq), val = pair.slice(eq + 1);
                if (!literalText(val)) return pair;
                const name = unescapeQuery(key), dec = unescapeQuery(val);
                if (!queryKey(name) && !isToken(dec)) return pair;
                return key + '=' + put(w, { location: 'url_query', path: urlPath(ptr, name), value: dec, top: ptr === '' }, val);
            }).join('&');
        }
        return joinURL(u);
    }

    function decodeContainer(s) {
        const t = s.trim();
        if (!t || (t[0] !== '{' && t[0] !== '[')) return { ok: false };
        try { return { ok: true, value: JSON.parse(t) }; } catch (err) { return { ok: false }; }
    }

    function scanHeaders(v, drop) {
        if (!v || typeof v !== 'object' || Array.isArray(v)) return { hits: [], value: v };
        const w = { fields: 'param_field', hits: [], drop }, out = {};
        Object.keys(v).sort().forEach(k => {
            const name = k.trim();
            if (typeof v[k] === 'string' && headerName(name)) {
                out[k] = literalHeader(v[k]) ? put(w, { location: 'header', path: name, value: v[k], top: true }, v[k]) : v[k];
                return;
            }
            out[k] = walk(w, v[k], '/' + pointer(k));
        });
        return { hits: w.hits, value: out };
    }

    // scanParam is scanCredentialParam: the hits of one node parameter and the parameter with them replaced.
    function scanParam(type, name, v, drop) {
        const w = { fields: 'param_field', hits: [], drop };
        const http = type === 'http.request';
        if (http && name === 'headers') return scanHeaders(v, drop);
        if (http && name === 'body') {
            w.fields = 'body_field';
            const parsed = typeof v === 'string' ? decodeContainer(v) : { ok: false };
            if (parsed.ok) {
                const red = walk(w, parsed.value, '');
                return w.hits.length ? { hits: w.hits, value: JSON.stringify(red) } : { hits: [], value: v };
            }
        } else if (String(type).startsWith('tool.')) {
            const red = entry(w, name, v, '');
            return { hits: w.hits, value: red };
        }
        const red = walk(w, v, '');
        return { hits: w.hits, value: red };
    }

    // redactHeaders is redactCredentialHeaders with keepTemplates: it also replaces credential
    // headers whose value is not a text.
    function redactHeaders(params) {
        const headers = params.headers;
        if (!headers || typeof headers !== 'object' || Array.isArray(headers)) return params;
        let copied = null;
        Object.keys(headers).forEach(name => {
            const v = headers[name];
            if (!headerName(name) || empty(v)) return;
            if (typeof v === 'string' && (v === MARKER || templateOnly(v))) return;
            if (!copied) copied = Object.assign({}, headers);
            copied[name] = MARKER;
        });
        return copied ? Object.assign({}, params, { headers: copied }) : params;
    }

    // maskParams is redactCredentialParams(type, params, true): a copy, or params itself when nothing changes.
    function maskParams(type, params) {
        if (!params || typeof params !== 'object') return params;
        const out = redactHeaders(params);
        let copied = null;
        Object.keys(out).sort().forEach(name => {
            const r = scanParam(type, name, out[name]);
            if (!r.hits.length) return;
            if (!copied) copied = Object.assign({}, out);
            copied[name] = r.value;
        });
        return copied || out;
    }

    function movable(node, param, hit) {
        if (node.type !== 'http.request' || !empty((node.params || {}).auth_secret)) return false;
        if (hit.location === 'header') return param === 'headers' && !hasTemplate(hit.value);
        if (hit.location === 'url_query' || hit.location === 'url_userinfo') return param === 'url' && hit.top;
        return false;
    }

    // findings lists the typed-in credentials of a node; each has the typed value (never send it).
    function findings(node) {
        const out = [];
        const params = (node && node.params) || {};
        Object.keys(params).sort().forEach(name => {
            scanParam(node.type, name, params[name]).hits.forEach(h => out.push({ nodeId: node.id, param: name, location: h.location, path: h.path, movable: movable(node, name, h), value: h.value }));
        });
        return out;
    }

    // remove returns the node's params with the value of one finding cleared.
    function remove(node, finding) {
        const params = JSON.parse(JSON.stringify(node.params || {}));
        params[finding.param] = scanParam(node.type, finding.param, params[finding.param], { location: finding.location, path: finding.path }).value;
        return params;
    }

    // mask returns a copy of a flow document (or clipboard fragment) with every finding masked.
    function mask(doc) {
        if (!doc || !Array.isArray(doc.nodes)) return doc;
        return Object.assign({}, doc, { nodes: doc.nodes.map(n => (n && n.params ? Object.assign({}, n, { params: maskParams(n.type, n.params) }) : n)) });
    }

    function hasFindings(doc) { return !!(doc && Array.isArray(doc.nodes) && doc.nodes.some(n => n && findings(n).length)); }

    ED.credentials = { findings, mask, maskParams, hasFindings, remove, MARKER };
})();
```
Run:
```
cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-s0b-credentials && node scripts/test-easydrag-credentials.mjs
```
Expected at this point: the vector loop passes; the run still fails at `assert.equal(copy.masked, true)` or at the missing saver hook (step 4). If a vector fails, the assertion message names the case: compare with the Go result (`go test ./internal/flows/ -run TestCredentialRuleSharedVectors/<name>`). Both sides must agree with the vector; a difference in key order, `%`-decoding or number formatting is a port bug.

- [ ] **Step 4: the masking sites**

1. `ui/js/desktop/apps/easydrag-saver.js`, in `writeEmergency`, replace the line `const ok = core.storage.set(emergencyKey(o.flowId), { revision, at, doc: o.model.toJSON() });` with:

```js
            const doc = o.model.toJSON();
            const ok = core.storage.set(emergencyKey(o.flowId), { revision, at, doc: ED.credentials.mask(doc), masked: ED.credentials.hasFindings(doc) });
```
2. `ui/js/desktop/apps/easydrag-interact.js`, in `copySelection`, replace `core.storage.set(CLIPBOARD_KEY, { at: Date.now(), frag });` with `core.storage.set(CLIPBOARD_KEY, { at: Date.now(), frag: ED.credentials.mask(frag) });`. (The text put on the system clipboard keeps the typed values: it is not browser storage of AuraGo and a paste within the session keeps working.)
3. `ui/js/desktop/apps/easydrag-dialogs.js`, in `restore`, change the `body:` expression so the notice follows the stale notice:

```js
            body: '<p>' + esc(t('easydrag.ui.restore_text', { time: ED.core.fmt.dateTime(copy.at) })) + '</p>' +
                (copy.stale ? '<p class="ed-callout ed-callout--warn" data-ed-restore-stale>' + ED.core.icon('alert') + '<span>' + esc(t('easydrag.ui.restore_stale')) + '</span></p>' : '') +
                (copy.masked ? '<p class="ed-callout ed-callout--warn" data-ed-restore-masked>' + ED.core.icon('alert') + '<span>' + esc(t('easydrag.ui.restore_masked')) + '</span></p>' : ''),
```
4. Register the module. `ui/js/desktop/core/module-loader.js`: after `'/js/desktop/apps/easydrag-template.js',` add `'/js/desktop/apps/easydrag-credentials.js',`. `ui/desktop_easydrag_test.go`: in `easyDragModules` change `"easydrag-template.js", "easydrag-model.js"` to `"easydrag-template.js", "easydrag-credentials.js", "easydrag-model.js"`.
5. Node harness lists. Every vm harness that loads the saver needs the rule (and the template module before it): `cd <worktree> && grep -n "easydrag-saver.js" scripts/*.mjs`. Expected hits: `test-easydrag.mjs` (the `saverHarness` list `['easydrag-core.js', 'easydrag-saver.js']`), `test-easydrag-extra.mjs`, `test-easydrag-extra3.mjs`. Change the `saverHarness` list to `['easydrag-core.js', 'easydrag-template.js', 'easydrag-credentials.js', 'easydrag-saver.js']`; in the other lists insert `'easydrag-credentials.js'` right after `'easydrag-template.js'`. Add the new test to the runner: in `scripts/test-easydrag.mjs` after `await import('./test-easydrag-approvals.mjs');` add `await import('./test-easydrag-credentials.mjs');`.
6. Locale key. Write `C:/disposable/easydrag-s0b/lang-b5.mjs`:

```js
export default {
    keys: ['easydrag.ui.restore_masked'],
    langs: {
        en: ['Passwords and access tokens in this copy were masked and show as [redacted]. Enter them again after restoring.'],
        de: ['Passwörter und Zugriffstoken in dieser Kopie wurden maskiert und erscheinen als [redacted]. Gib sie nach dem Wiederherstellen neu ein.'],
        cs: ['Hesla a přístupové tokeny v této kopii byly zamaskovány a zobrazují se jako [redacted]. Po obnovení je zadej znovu.'],
        da: ['Adgangskoder og adgangstokens i denne kopi er maskeret og vises som [redacted]. Indtast dem igen efter gendannelsen.'],
        el: ['Οι κωδικοί και τα διακριτικά πρόσβασης σε αυτό το αντίγραφο αποκρύφθηκαν και εμφανίζονται ως [redacted]. Πληκτρολόγησέ τα ξανά μετά την επαναφορά.'],
        es: ['Las contraseñas y los tokens de acceso de esta copia se ocultaron y aparecen como [redacted]. Vuelve a escribirlos tras restaurar.'],
        fr: ['Les mots de passe et jetons d’accès de cette copie ont été masqués et apparaissent comme [redacted]. Saisis-les de nouveau après la restauration.'],
        hi: ['इस प्रति में पासवर्ड और एक्सेस टोकन छिपा दिए गए हैं और [redacted] के रूप में दिखते हैं। पुनर्स्थापित करने के बाद उन्हें फिर से दर्ज करो।'],
        it: ['Le password e i token di accesso in questa copia sono stati oscurati e compaiono come [redacted]. Reinseriscili dopo il ripristino.'],
        ja: ['このコピー内のパスワードとアクセストークンはマスクされ、[redacted] と表示されます。復元後に入力し直してください。'],
        nl: ['Wachtwoorden en toegangstokens in deze kopie zijn gemaskeerd en verschijnen als [redacted]. Voer ze na het herstellen opnieuw in.'],
        no: ['Passord og tilgangstokens i denne kopien er maskert og vises som [redacted]. Skriv dem inn på nytt etter gjenopprettingen.'],
        pl: ['Hasła i tokeny dostępu w tej kopii zostały zamaskowane i widnieją jako [redacted]. Po przywróceniu wpisz je ponownie.'],
        pt: ['As senhas e os tokens de acesso desta cópia foram ocultados e aparecem como [redacted]. Digite-os de novo após restaurar.'],
        sv: ['Lösenord och åtkomsttokens i den här kopian har maskerats och visas som [redacted]. Skriv in dem igen efter återställningen.'],
        zh: ['此副本中的密码和访问令牌已被遮蔽，显示为 [redacted]。恢复后请重新输入。']
    }
};
```
```
node C:/disposable/easydrag-s0b/lang-add.mjs C:/disposable/easydrag-s0b/lang-b5.mjs C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-s0b-credentials
```
Expected: `added 1 keys to 16 files`.

- [ ] **Step 5: run the checks**

```
cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-s0b-credentials && node scripts/test-easydrag-credentials.mjs
cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-s0b-credentials && node scripts/test-easydrag.mjs && node scripts/test-easydrag-render-baseline.mjs
cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-s0b-credentials && go test ./ui/ -run '(?i)(EasyDrag)' -count=1
```
Expected: `PASS credential rule and masking`; `all EasyDrag checks passed`; `PASS render baseline (unchanged)`; `ok  aurago/ui`. If a Node suite fails with `Cannot read properties of undefined (reading 'mask')`, that harness loads the saver without the rule: add it to its module list (step 4.5). The Go test `TestEasyDragAssetsLoadInDependencyOrder` fails if the loader order and `easyDragModules` disagree: both must read `template`, `credentials`, `model`.

- [ ] **Step 6: gates and commit**

```
cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-s0b-credentials && gofmt -l ui/desktop_easydrag_test.go && go vet ./ui/
```
Expected: no output. Write `C:/disposable/easydrag-s0b/b1-5.txt`:

```
feat(easydrag): mask typed credentials before browser storage

ED.credentials mirrors the Go rule (findings, mask, maskParams) and passes the
same vectors. The emergency copy and the clipboard fragment are masked, and
the restore dialog says tokens were masked. The remembered test trigger holds
only a node id, so nothing is masked there; test data lives on the server.

GitNexus detect-changes unavailable in the worktree; judged by symbols and tests.

Co-Authored-By: <your model line>
```
```
cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-s0b-credentials && git add ui/js/desktop/apps/easydrag-credentials.js ui/js/desktop/apps/easydrag-saver.js ui/js/desktop/apps/easydrag-interact.js ui/js/desktop/apps/easydrag-dialogs.js ui/js/desktop/core/module-loader.js ui/desktop_easydrag_test.go scripts/test-easydrag-credentials.mjs scripts/test-easydrag.mjs scripts/test-easydrag-extra.mjs scripts/test-easydrag-extra3.mjs ui/lang/easydrag && git commit -F C:/disposable/easydrag-s0b/b1-5.txt
```
Add any other `scripts/test-easydrag-*.mjs` file you changed in step 4.5 to the `git add` list. After this commit, do Task B.4.

---

### Final check for the part (after B.4)

```
cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-s0b-credentials && GOTMPDIR='D:\tmp\go-build' go test ./internal/flows/ ./internal/tools/ -count=1
cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-s0b-credentials && go test ./internal/server/ -run '(?i)(Flow|Secret)' -count=1 && go test ./ui/ -run '(?i)(EasyDrag)' -count=1 && node scripts/test-easydrag.mjs
```
Expected: all `ok`, then `all EasyDrag checks passed`. Part B2 continues in the same worktree on the same branch.

### Coverage: spec item to task

| Spec item | Task |
|---|---|
| §4.2 one rule: headers, URL user info, URL query, body and parameter fields, token formats, `key` exception, templates and marker never findings | B.1 |
| §4.2 shared test vectors (Go and client) | B.1 (file, Go test), B.5 (Node test) |
| §4.3 validation: one `PARAM_LITERAL_CREDENTIAL` warning per finding with `location`, `path`; locale variants in 16 files | B.1 |
| §4.3 redaction: `RedactCredentials`, stored step params (engine, durable wait, older run rows) | B.1 |
| contract names `CredentialFinding`, `FindLiteralCredentials`, `literalCredentialsInValue`, `redactCredentialValue`, `CredentialRedactionMarker` | B.1 |
| §5.1 `auth_in`, `auth_query`, first-request-only secret, scrubber marker, redirect never gets it | B.2 |
| §5.2 move endpoint: revision, literal lookup, name check, vault write, rewrite for header/query/user info, identical values, audit, answer; 409/422 codes | B.3 |
| §5.2 non-admin refused, cross-origin refused, rate limit, value never in the answer | B.3 |
| §5.3 UI action (move / remove, name dialog with proposal, toast, purge offer after publish via `ED.credentialsPurge`) | B.4 |
| §5.4 `scrubHistory(value)` | B.4 |
| §5.4 `ED.credentials` (`findings`, `mask`, `maskParams`), emergency copy, clipboard, restore notice | B.5 |
| §5.4 remembered test data | B.5 (verified: holds only a trigger node id; no change) |
| §7 Part B Go tests (rule, query auth, move endpoint) | B.1, B.2, B.3 |
| §7 Part B client tests (Node masking, `scrubHistory`, gated browser journey) | B.5, B.4 |
| §9 "In den Tresor verschieben works for header, query and user info; the value never travels back" | B.3, B.4 |
| §9 emergency copy, clipboard, undo carry no moved token; all keys in 16 locales; new FLOW codes registered | B.4, B.5, B.3 |
