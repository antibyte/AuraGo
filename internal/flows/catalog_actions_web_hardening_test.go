package flows

import (
	"errors"
	"math"
	"reflect"
	"sort"
	"strings"
	"testing"
)

const searchOK = `{"status":"success","results":[{"title":"T","link":"https://t","snippet":"s"}]}`

func searchNodeDef(t *testing.T, env CatalogEnv) *NodeDef {
	t.Helper()
	return lookupDef(t, webRegistry(t, env), TypeWebSearch)
}

func readNodeDef(t *testing.T) *NodeDef { return lookupDef(t, webRegistry(t, nil), TypeWebRead) }

// A search that found nothing is an empty list; anything else the tool reports is a
// failure. The decision reads the tool's own answer: the node error is cut to 300
// runes, and a denial or an unavailable tool never reads as "nothing found".
func TestWebSearchNoResults(t *testing.T) {
	ddgEnv := StaticEnv{DDGSearchTool: {}}
	long := strings.Repeat("x", 600)
	text := func(s string) ToolResponse { return ToolResponse{Output: s, Status: "success"} }
	cases := []struct {
		name  string
		resp  ToolResponse
		err   error
		empty bool
		code  string // of the failure when not empty
	}{
		{"plan wording", text(`{"status":"error","message":"No results found for query"}`), nil, true, ""},
		{"other case", text(`{"status":"error","message":"  NO RESULT for this"}`), nil, true, ""},
		{"error field", text(`{"status":"error","error":"No results found."}`), nil, true, ""},
		{"long message", text(`{"status":"error","message":"No results found for ` + long + `"}`), nil, true, ""},
		{"as failed answer", ToolResponse{Output: `{"status":"error","message":"No results found."}`, IsError: true, Status: "failed"}, nil, true, ""},
		{"plain text", text("No results found."), nil, true, ""},
		{"success without results", text(`{"status":"success","results":[],"message":"No results found."}`), nil, true, ""},
		{"the real ddg_search", text(`{"status":"error","message":"No parseable DDG results found. DuckDuckGo may have returned a bot-check, consent page, no-results page, or changed markup."}`), nil, false, "FLOW_TOOL_ERROR"},
		{"mentioned later", text(`{"status":"error","message":"search failed: no results"}`), nil, false, "FLOW_TOOL_ERROR"},
		{"mentioned after a long text", text(`{"status":"error","message":"` + long + ` no results found"}`), nil, false, "FLOW_TOOL_ERROR"},
		{"other failure", text(`{"status":"error","message":"DDG request failed: i/o timeout"}`), nil, false, "FLOW_TOOL_ERROR"},
		{"brave key", text(`{"status":"error","message":"Brave Search API key is missing."}`), nil, false, "FLOW_TOOL_ERROR"},
		{"denied status", ToolResponse{Output: `{"status":"error","message":"No results found."}`, Status: "denied"}, nil, false, "FLOW_TOOL_DENIED"},
		{"policy denied", text(`{"status":"policy_denied","message":"No results found."}`), nil, false, "FLOW_TOOL_DENIED"},
		{"needs setup", ToolResponse{Output: `{"message":"No results found."}`, Status: "needs_setup"}, nil, false, "FLOW_NODE_UNAVAILABLE"},
		{"plain refusal", text("[PERMISSION DENIED] search is disabled"), nil, false, "FLOW_TOOL_ERROR"},
		{"plain refusal mentions it", text("[PERMISSION DENIED] no results for you"), nil, false, "FLOW_TOOL_ERROR"},
		{"empty answer", text(""), nil, false, "FLOW_TOOL_ERROR"},
		{"invoker error", ToolResponse{}, errors.New("No results found"), false, "FLOW_NODE_FAILED"},
		{"results not a list", text(`{"status":"success","results":"none"}`), nil, false, "FLOW_TOOL_ERROR"},
		{"no results field", text(`{"status":"success"}`), nil, false, "FLOW_TOOL_ERROR"},
		{"results object", text(`{"status":"success","results":{"a":1}}`), nil, false, "FLOW_TOOL_ERROR"},
	}
	for _, c := range cases {
		tools := &fakeTools{respond: func(ToolRequest) (ToolResponse, error) { return c.resp, c.err }}
		res, err := execDef(searchNodeDef(t, ddgEnv), map[string]any{"query": "q"}, &Services{Tools: tools})
		if c.empty {
			if list, ok := res.Output["results"].([]any); err != nil || !ok || len(list) != 0 || res.Output["count"] != 0.0 || res.ItemCount != 0 {
				t.Errorf("%s: %#v, %v; want an empty list", c.name, res.Output, err)
			}
			continue
		}
		if ne := asNodeError(err); ne == nil || ne.Code != c.code {
			t.Errorf("%s: error = %v, want %s", c.name, err, c.code)
		}
	}
}

func TestWebSearchResults(t *testing.T) {
	entries := `{"title":"A","link":"https://a"},"x",null,{"title":"B","url":"https://b","description":"d"},{"title":"C"},7,{"title":"D","link":"https://d","published":"p"}`
	tools := &fakeTools{respond: toolReply(`{"status":"success","results":[` + entries + `]}`)}
	def := searchNodeDef(t, StaticEnv{DDGSearchTool: {}})
	res, err := execDef(def, map[string]any{"query": "q", "count": 3.0}, &Services{Tools: tools})
	if err != nil {
		t.Fatal(err)
	}
	want := []any{
		map[string]any{"title": "A", "url": "https://a", "snippet": ""},
		map[string]any{"title": "B", "url": "https://b", "snippet": "d"},
		map[string]any{"title": "C", "url": "", "snippet": ""},
	}
	if !reflect.DeepEqual(res.Output["results"], want) || res.ItemCount != 3 || res.Output["count"] != 3.0 {
		t.Errorf("results = %#v", res.Output)
	}
	// Everything the node returns is data the engine can encode.
	if res2, err := execDef(def, map[string]any{"query": "q", "count": 20.0}, &Services{Tools: tools}); err != nil || res2.ItemCount != 4 {
		t.Errorf("all four: %d, %v", res2.ItemCount, err)
	}
}

func TestWebSearchParams(t *testing.T) {
	both := StaticEnv{DDGSearchTool: {}, BraveSearchTool: {}}
	ddgOnly := StaticEnv{DDGSearchTool: {}}
	braveOnly := StaticEnv{BraveSearchTool: {}}
	run := func(env CatalogEnv, params map[string]any) (*fakeTools, ExecResult, error) {
		tools := &fakeTools{respond: toolReply(searchOK)}
		res, err := execDef(searchNodeDef(t, env), params, &Services{Tools: tools})
		return tools, res, err
	}
	countArg := func(tools *fakeTools) any {
		args := tools.last(t).Args
		if v, ok := args["max_results"]; ok {
			return v
		}
		return args["count"]
	}
	for name, c := range map[string]struct {
		count any
		want  int
	}{
		"null": {nil, 5}, "blank": {" ", 5}, "zero": {0.0, 1}, "negative": {-3.0, 1}, "fraction": {2.9, 2}, "text": {"7", 7},
		"over": {99.0, 20}, "huge": {1e300, 20}, "minus huge": {-1e300, 1}, "at the limit": {20.0, 20}, "one": {1.0, 1}, "int": {4, 4},
	} {
		tools, _, err := run(ddgOnly, map[string]any{"query": "q", "count": c.count})
		if got := countArg(tools); err != nil || got != c.want {
			t.Errorf("count %s: sent %#v (%v), want %d", name, got, err, c.want)
		}
	}
	for name, count := range map[string]any{"words": "many", "flag": true, "list": []any{1.0}, "object": map[string]any{}, "NaN": math.NaN(), "Inf": math.Inf(1)} {
		tools, _, err := run(ddgOnly, map[string]any{"query": "q", "count": count})
		wantParamInvalid(t, "count "+name, tools, err)
	}

	for name, query := range map[string]any{
		"empty": "", "blank": " \t\n", "null": nil, "list": []any{"a"}, "object": map[string]any{"a": 1.0}, "NaN": math.NaN(),
		"NUL": "a\x00b", "invalid UTF-8": "a\xff", "too long": strings.Repeat("q", maxSearchQueryBytes+1),
	} {
		tools, _, err := run(ddgOnly, map[string]any{"query": query})
		wantParamInvalid(t, "query "+name, tools, err, "qqqq")
	}
	for name, c := range map[string]struct {
		query any
		want  string
	}{
		"trimmed": {"  go news \n", "go news"}, "number": {2026.0, "2026"}, "flag": {true, "true"},
		"at the limit": {strings.Repeat("q", maxSearchQueryBytes), strings.Repeat("q", maxSearchQueryBytes)}, "unicode": {"größe", "größe"},
	} {
		tools, res, err := run(ddgOnly, map[string]any{"query": c.query})
		if err != nil || tools.last(t).Args["query"] != c.want || res.Output["query"] != c.want {
			t.Errorf("query %s: %v, sent %#v", name, err, tools.last(t).Args["query"])
		}
	}

	// The provider: spelling is forgiven, an unknown one is an error (it used to be
	// searched with DuckDuckGo under the unknown name), and one named explicitly
	// must be available.
	for _, provider := range []any{"bing", "Google", 5.0, true, []any{"brave"}, map[string]any{}} {
		tools, _, err := run(both, map[string]any{"query": "q", "provider": provider})
		wantParamInvalid(t, "provider", tools, err)
		node := &Node{ID: testNodeID(1), Params: map[string]any{"query": "q", "provider": provider}}
		if issues := searchNodeDef(t, both).Validate(node, ValidateContext{}); len(issues) != 1 || issues[0].Param != "provider" {
			t.Errorf("provider %#v: issues = %+v", provider, issues)
		}
	}
	for name, c := range map[string]struct {
		env      CatalogEnv
		provider any
		tool     string // "": no call
		code     string
	}{
		"auto prefers brave":      {both, "auto", BraveSearchTool, ""},
		"auto without brave":      {ddgOnly, nil, DDGSearchTool, ""},
		"auto brave only":         {braveOnly, "", BraveSearchTool, ""},
		"explicit brave":          {both, " Brave ", BraveSearchTool, ""},
		"explicit duckduckgo":     {both, "DuckDuckGo", DDGSearchTool, ""},
		"brave not set up":        {ddgOnly, "brave", "", "FLOW_NODE_UNAVAILABLE"},
		"duckduckgo not set up":   {braveOnly, "duckduckgo", "", "FLOW_NODE_UNAVAILABLE"},
		"no environment":          {nil, "duckduckgo", "", "FLOW_NODE_UNAVAILABLE"},
		"no environment and auto": {nil, "auto", DDGSearchTool, ""},
	} {
		tools, res, err := run(c.env, map[string]any{"query": "q", "provider": c.provider})
		if c.code != "" {
			if asNodeError(err).Code != c.code || tools.count() != 0 {
				t.Errorf("%s: %v after %d calls", name, err, tools.count())
			}
			continue
		}
		wantProvider := map[string]string{BraveSearchTool: "brave", DDGSearchTool: "duckduckgo"}[c.tool]
		if err != nil || tools.last(t).Tool != c.tool || res.Output["provider"] != wantProvider {
			t.Errorf("%s: tool %q, provider %v, %v", name, tools.last(t).Tool, res.Output["provider"], err)
		}
	}
	node := &Node{ID: testNodeID(1), Params: map[string]any{"query": "q", "provider": "{{trigger.data.p}}"}}
	if issues := searchNodeDef(t, both).Validate(node, ValidateContext{}); len(issues) != 0 {
		t.Errorf("template provider: %+v", issues)
	}
}

func TestWebReadParams(t *testing.T) {
	read := func(params map[string]any) (*fakeTools, ExecResult, error) {
		tools := &fakeTools{respond: toolReply(`{"status":"success","title":"T","content":"c"}`)}
		res, err := execDef(readNodeDef(t), params, &Services{Tools: tools})
		return tools, res, err
	}
	for name, u := range map[string]any{
		"empty": "", "blank": "  ", "null": nil, "list": []any{"https://x"}, "object": map[string]any{}, "NaN": math.NaN(),
		"ftp": "ftp://x/f", "file": "file:///etc/passwd", "javascript": "javascript:alert(1)", "data": "data:text/html,x",
		"no scheme": "example.com/feed", "scheme only": "https:/x", "CRLF": "https://x/\r\nHost: evil", "LF": "https://x/\nEvil",
		"NUL": "https://x/\x00", "tab": "https://x/\ta", "DEL": "https://x/\x7f", "invalid UTF-8": "https://x/\xff",
		"too long": "https://x/" + strings.Repeat("p", maxWebURLBytes), "leading junk": "x https://y",
	} {
		tools, _, err := read(map[string]any{"url": u})
		wantParamInvalid(t, "url "+name, tools, err, "evil", "Evil", "pppp")
	}
	for name, u := range map[string]string{
		"https": "https://x/a?b=c#d", "http": "http://x", "upper": "HTTPS://X/A", "mixed": "hTtP://x", "padded": "  https://x/y \n",
		"userinfo": "https://u:p@x/", "port": "http://x:8080/", "ipv6": "http://[::1]/", "at the limit": "https://x/" + strings.Repeat("p", maxWebURLBytes-10),
		"unicode": "https://x/größe",
	} {
		tools, res, err := read(map[string]any{"url": u})
		if err != nil || tools.last(t).Args["url"] != strings.TrimSpace(u) || res.Output["url"] != strings.TrimSpace(u) {
			t.Errorf("url %s: %v, sent %#v", name, err, tools.last(t).Args["url"])
		}
	}
	// A private address is the tool's decision (SSRF protection); the node passes it on.
	if tools, _, err := read(map[string]any{"url": "http://127.0.0.1:8080/admin"}); err != nil || tools.count() != 1 {
		t.Errorf("loopback address: %v", err)
	}

	for _, mode := range []any{"pdf", "Reader", 5.0, true, []any{"rss"}, map[string]any{}} {
		tools, _, err := read(map[string]any{"url": "https://x", "mode": mode})
		wantParamInvalid(t, "mode", tools, err)
		node := &Node{ID: testNodeID(1), Params: map[string]any{"url": "https://x", "mode": mode}}
		if issues := readNodeDef(t).Validate(node, ValidateContext{}); len(issues) != 1 || issues[0].Param != "mode" {
			t.Errorf("mode %#v: issues = %+v", mode, issues)
		}
	}
	for in, want := range map[string]string{" RSS ": "rss", "Static": "static", "DYNAMIC": "dynamic", "auto": "auto", "": "auto"} {
		tools, _, err := read(map[string]any{"url": "https://x", "mode": in})
		if err != nil || tools.last(t).Args["mode"] != want {
			t.Errorf("mode %q: %v, sent %#v", in, err, tools.last(t).Args["mode"])
		}
	}
	if tools, _, err := read(map[string]any{"url": "https://x", "mode": nil}); err != nil || tools.last(t).Args["mode"] != "auto" {
		t.Errorf("null mode: %v", err)
	}

	// The tool refuses a selector in rss mode; the node says so before it asks.
	tools, _, err := read(map[string]any{"url": "https://x", "mode": "rss", "selector": "a.item"})
	wantParamInvalid(t, "selector in rss mode", tools, err)
	node := &Node{ID: testNodeID(1), Params: map[string]any{"url": "https://x", "mode": " RSS", "selector": "a.item"}}
	if issues := readNodeDef(t).Validate(node, ValidateContext{}); len(issues) != 1 || issues[0].Param != "selector" {
		t.Errorf("selector in rss mode: %+v", issues)
	}
	for _, p := range []map[string]any{
		{"mode": "rss", "selector": "{{trigger.data.s}}"}, {"mode": "rss", "selector": "  "}, {"mode": "rss"}, {"mode": "static", "selector": "a"}, {"mode": "{{x.y}}", "selector": "a"},
	} {
		p["url"] = "https://x"
		if issues := readNodeDef(t).Validate(&Node{ID: testNodeID(1), Params: p}, ValidateContext{}); len(issues) != 0 {
			t.Errorf("%v: issues = %+v", p, issues)
		}
	}
	if tools, _, err := read(map[string]any{"url": "https://x", "mode": "rss", "selector": "  "}); err != nil || tools.last(t).Args["selector"] != nil {
		t.Errorf("blank selector in rss mode: %v", err)
	}
	for name, sel := range map[string]any{"list": []any{"a"}, "object": map[string]any{}, "NUL": "a\x00", "too long": strings.Repeat("s", maxSelectorBytes+1)} {
		tools, _, err := read(map[string]any{"url": "https://x", "selector": sel})
		wantParamInvalid(t, "selector "+name, tools, err, "ssss")
	}
	if tools, _, err := read(map[string]any{"url": "https://x", "selector": "  div > a  "}); err != nil || tools.last(t).Args["selector"] != "div > a" {
		t.Errorf("selector: %v", err)
	}
}

// A page is only what the tool reports as a success: a refusal that came as plain
// text must not become its content. The shapes of the real answers are kept.
func TestWebReadAnswers(t *testing.T) {
	text := func(s string) ToolResponse { return ToolResponse{Output: s, Status: "success"} }
	for name, c := range map[string]struct {
		resp    ToolResponse
		code    string
		content string
		count   int
		field   string
	}{
		"page":          {text(`{"status":"success","mode":"static","title":"T","content":"# md","links":["https://l"]}`), "", "# md", 0, ""},
		"feed":          {text(`{"status":"success","mode":"rss","title":"F","content":"- a","items":[{"title":"a","link":"https://a"},{"title":"b"}]}`), "", "- a", 2, "items"},
		"selector":      {text(`{"status":"success","mode":"static","selector":"a","output_format":"text","count":2,"matches":["x","y","z"]}`), "", "", 3, "matches"},
		"no matches":    {text(`{"status":"success","selector":"a","count":0,"matches":[]}`), "", "", 0, "matches"},
		"plain refusal": {text("Tool Output: [PERMISSION DENIED] web_scraper is disabled in settings (tools.web_scraper.enabled: false)."), "FLOW_TOOL_ERROR", "", 0, ""},
		"error":         {text(`{"status":"error","message":"scrape failed: HTTP 404"}`), "FLOW_TOOL_ERROR", "", 0, ""},
		"no status":     {text(`{"title":"T","content":"c"}`), "FLOW_TOOL_ERROR", "", 0, ""},
		"denied":        {ToolResponse{Output: `{"status":"error","message":"no"}`, Status: "denied"}, "FLOW_TOOL_DENIED", "", 0, ""},
		"needs setup":   {ToolResponse{Output: `{"message":"no"}`, Status: "needs_setup"}, "FLOW_NODE_UNAVAILABLE", "", 0, ""},
		"upper case":    {text(`{"status":"Success","title":"T","content":"c"}`), "", "c", 0, ""},
	} {
		tools := &fakeTools{respond: answerWith(c.resp)}
		res, err := execDef(readNodeDef(t), map[string]any{"url": "https://x"}, &Services{Tools: tools})
		if c.code != "" {
			if ne := asNodeError(err); ne == nil || ne.Code != c.code {
				t.Errorf("%s: %v, want %s", name, err, c.code)
			}
			continue
		}
		if err != nil || res.Output["content"] != c.content || res.ItemCount != c.count {
			t.Errorf("%s: %#v, %v", name, res.Output, err)
		}
		if _, has := res.Output[c.field]; c.field != "" && !has {
			t.Errorf("%s: no %s field", name, c.field)
		}
	}
}

// What the lint reads: web content is untrusted, and the parameters that send data
// somewhere or choose what is sent are sinks.
func TestWebNodeTaintFlags(t *testing.T) {
	sinks := map[string][]string{
		TypeWebSearch:   {"query"},
		TypeWebRead:     {"url"},
		TypeHTTPRequest: {"auth_secret", "body", "headers", "url"},
	}
	reg := webRegistry(t, nil)
	if n := len(reg.All()); n != 3 {
		t.Fatalf("%d web node types", n)
	}
	for _, def := range reg.All() {
		var got []string
		for _, p := range def.Params {
			if p.SensitiveSink {
				got = append(got, p.Name)
			}
		}
		sort.Strings(got)
		if !reflect.DeepEqual(got, sinks[def.Type]) {
			t.Errorf("%s: sinks %v, want %v", def.Type, got, sinks[def.Type])
		}
		if !def.UntrustedOutput {
			t.Errorf("%s: its output is attacker-influenced and must be untrusted", def.Type)
		}
		if def.Category != "web" || def.Tool == "" || def.LabelKey == "" || def.AvailabilityFunc == nil || def.Execute == nil || def.PrimaryInput == "" {
			t.Errorf("%s: incomplete definition", def.Type)
		}
	}
}

// End to end: untrusted data that reaches a sink of a web node warns, and the
// output of a web node is untrusted for what comes after it.
func TestLintFlagsUntrustedDataInWebNodes(t *testing.T) {
	reg := triggerRegistry(t)
	if err := registerWebNodes(reg, nil); err != nil {
		t.Fatal(err)
	}
	if err := registerDocNodes(reg, nil); err != nil {
		t.Fatal(err)
	}
	warned := func(f *Flow, nodeID string) string {
		var got []string
		for _, is := range LintUntrustedData(f, reg) {
			if is.NodeID == nodeID {
				got = append(got, is.Param)
			}
		}
		sort.Strings(got)
		return strings.Join(got, ",")
	}
	for _, c := range []struct {
		name  string
		build func(b *flowBuilder) string
		want  string
	}{
		{"webhook into a search", func(b *flowBuilder) string {
			h := b.node("hook", TypeTriggerWebhook, nil)
			s := b.node("find", TypeWebSearch, map[string]any{"query": "{{trigger.data.payload.topic}}"})
			b.edge(h, PortOut, s)
			return s
		}, "query"},
		{"webhook into a page address", func(b *flowBuilder) string {
			h := b.node("hook", TypeTriggerWebhook, nil)
			r := b.node("page", TypeWebRead, map[string]any{"url": "{{trigger.data.payload.link}}", "selector": "{{trigger.data.raw}}"})
			b.edge(h, PortOut, r)
			return r
		}, "url"},
		{"webhook into a request", func(b *flowBuilder) string {
			h := b.node("hook", TypeTriggerWebhook, nil)
			r := b.node("call", TypeHTTPRequest, map[string]any{
				"method": "POST", "url": "{{trigger.data.payload.u}}", "body": "{{trigger.data.raw}}",
				"headers": map[string]any{"X-A": "{{trigger.data.payload.h}}"}, "auth_secret": "{{trigger.data.payload.s}}", "auth_prefix": "{{trigger.data.payload.p}}",
			})
			b.edge(h, PortOut, r)
			return r
		}, "auth_secret,body,headers,url"},
		{"a page into a request body", func(b *flowBuilder) string {
			m := b.node("start", TypeTriggerManual, nil)
			r := b.node("page", TypeWebRead, map[string]any{"url": "https://x"})
			c := b.node("call", TypeHTTPRequest, map[string]any{"method": "POST", "url": "https://api", "body": "{{page.content}}"})
			b.edge(m, PortOut, r)
			b.edge(r, PortOut, c)
			return c
		}, "body"},
		{"search results into a page address", func(b *flowBuilder) string {
			m := b.node("start", TypeTriggerManual, nil)
			s := b.node("find", TypeWebSearch, map[string]any{"query": "news"})
			r := b.node("page", TypeWebRead, map[string]any{"url": "{{find.results}}"})
			b.edge(m, PortOut, s)
			b.edge(s, PortOut, r)
			return r
		}, "url"},
		{"a response into a file path", func(b *flowBuilder) string {
			m := b.node("start", TypeTriggerManual, nil)
			c := b.node("call", TypeHTTPRequest, map[string]any{"url": "https://api"})
			w := b.node("save", TypeFileWrite, map[string]any{"path": "{{call.json.name}}", "content": "{{call.body}}"})
			b.edge(m, PortOut, c)
			b.edge(c, PortOut, w)
			return w
		}, "content,path"},
		{"a manual trigger is trusted", func(b *flowBuilder) string {
			m := b.node("start", TypeTriggerManual, nil)
			r := b.node("call", TypeHTTPRequest, map[string]any{"url": "{{trigger.data.u}}", "body": "{{trigger.data.b}}", "method": "POST"})
			b.edge(m, PortOut, r)
			return r
		}, ""},
		{"a fixed request", func(b *flowBuilder) string {
			h := b.node("hook", TypeTriggerWebhook, nil)
			r := b.node("call", TypeHTTPRequest, map[string]any{"url": "https://api", "auth_secret": "tok", "body": "x", "method": "POST"})
			b.edge(h, PortOut, r)
			return r
		}, ""},
	} {
		b := newFlow(c.name)
		id := c.build(b)
		if got := warned(b.build(), id); got != c.want {
			t.Errorf("%s: warned parameters %q, want %q", c.name, got, c.want)
		}
	}
}
