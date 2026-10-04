package flows

import (
	"context"
	"encoding/json"
	"math"
	"strings"
)

// Web node types.
const (
	TypeWebSearch   = "web.search"
	TypeWebRead     = "web.read"
	TypeHTTPRequest = "http.request"
)

// Search tool names. brave_search is a direct action without a native schema; plan 1c special-cases it.
const (
	DDGSearchTool   = "ddg_search"
	BraveSearchTool = "brave_search"
)

// Limits of web.search and web.read. Every one bounds something a flow document or a
// template can make arbitrarily large before it is passed on (the limits of
// http.request are in catalog_actions_http.go).
const (
	defaultSearchResults = 5
	maxSearchResults     = 20
	maxSearchQueryBytes  = 2048
	maxWebURLBytes       = 8192
	maxSelectorBytes     = 2048
)

// Choices of the enumerated parameters; the definitions say which one is the default.
var (
	searchProviders = []string{"auto", "duckduckgo", "brave"}
	scrapeModes     = []string{"auto", "static", "dynamic", "rss"}
)

func registerWebNodes(reg *Registry, env CatalogEnv) error {
	for _, def := range []*NodeDef{webSearchDef(env), webReadDef(env), httpRequestDef(env)} {
		if err := reg.Register(def); err != nil {
			return err
		}
	}
	return nil
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

// scalarText reads a parameter that must be text. Text, numbers and booleans give
// their natural form and null gives ""; a list or an object is not ok, because
// rendering it into a search query, an address or a header would hide a mistake.
func scalarText(v any) (string, bool) {
	switch x := v.(type) {
	case nil:
		return "", true
	case string:
		return x, true
	case bool, int, int64:
		return Stringify(v), true
	case float64, float32, json.Number:
		if _, ok := toNumber(x); ok {
			return Stringify(v), true
		}
	}
	return "", false
}

func scalarTextParam(v any, what string) (string, error) {
	s, ok := scalarText(v)
	if !ok {
		return "", NewNodeError("FLOW_PARAM_INVALID", "%s must be text", what)
	}
	return s, nil
}

// boolParam reads a flag. Null gives def: a template that resolves to nothing must
// not switch a safety flag such as fail_on_error off.
func boolParam(v any, def bool) bool {
	if v == nil {
		return def
	}
	return truthy(v)
}

// webURLParam reads a web address: text of at most maxWebURLBytes, valid UTF-8, free
// of control characters and starting with http:// or https://. The tools check the
// scheme and refuse private addresses on their own; this is the early, clear
// refusal. The messages never echo the address.
func webURLParam(v any) (string, error) {
	s, err := scalarTextParam(v, "the web address")
	if err != nil {
		return "", err
	}
	s = strings.TrimSpace(s)
	switch {
	case s == "":
		return "", NewNodeError("FLOW_PARAM_INVALID", "enter a web address")
	case len(s) > maxWebURLBytes:
		return "", NewNodeError("FLOW_PARAM_INVALID", "the web address is %d bytes; the limit is %d", len(s), maxWebURLBytes)
	case !validHeaderValue(s) || strings.ContainsRune(s, '\t'):
		return "", NewNodeError("FLOW_PARAM_INVALID", "the web address contains characters that are not allowed")
	case !hasWebScheme(s):
		return "", NewNodeError("FLOW_PARAM_INVALID", "the web address must start with http:// or https://")
	}
	return s, nil
}

func hasWebScheme(s string) bool {
	for _, scheme := range []string{"https://", "http://"} {
		if len(s) >= len(scheme) && strings.EqualFold(s[:len(scheme)], scheme) {
			return true
		}
	}
	return false
}

// webSearchDef defines web.search. The query is a sensitive sink: it leaves the
// installation for a third party (DuckDuckGo or Brave log it), so data an attacker
// shaped must not be able to steer or fill it unnoticed. The lint only warns, and a
// flow that searches for a topic taken from a webhook gets the warning once.
func webSearchDef(env CatalogEnv) *NodeDef {
	def := actionDef(TypeWebSearch, "web", "search", DDGSearchTool, env)
	def.AvailabilityFunc = anyAvailable(env, DDGSearchTool, BraveSearchTool)
	def.UntrustedOutput = true
	def.PrimaryInput = "query"
	def.Params = []ParamSpec{
		{Name: "query", Kind: ParamText, LabelKey: "easydrag.param.search_query", Required: true, Templatable: true, SensitiveSink: true},
		{Name: "count", Kind: ParamNumber, LabelKey: "easydrag.param.search_count", Default: 5.0, Templatable: true},
		{Name: "provider", Kind: ParamSelect, LabelKey: "easydrag.param.search_provider", Default: "auto",
			Options: []Option{option("auto", "search_auto"), {Value: "duckduckgo", Label: "DuckDuckGo"}, {Value: "brave", Label: "Brave"}}},
	}
	def.OutputFields = []FieldSpec{{Name: "results", Type: "list", Primary: true}, {Name: "count", Type: "number"},
		{Name: "provider", Type: "text"}, {Name: "query", Type: "text"}}
	def.Validate = func(n *Node, _ ValidateContext) []Issue {
		if n == nil {
			return nil
		}
		return choiceIssue(n, "provider", "auto", searchProviders)
	}
	def.Execute = func(ctx context.Context, in ExecInput) (ExecResult, error) {
		query, err := searchQueryParam(in.Params["query"])
		if err != nil {
			return ExecResult{}, err
		}
		count, err := searchCountParam(in.Params["count"])
		if err != nil {
			return ExecResult{}, err
		}
		choice, ok := choiceParam(in.Params["provider"], "auto", searchProviders...)
		if !ok {
			return ExecResult{}, choiceError("provider", searchProviders)
		}
		provider, err := resolveSearchProvider(env, choice)
		if err != nil {
			return ExecResult{}, err
		}
		var out map[string]any
		if provider == "brave" {
			out, err = callTool(ctx, in, BraveSearchTool, map[string]any{"query": query, "count": count})
		} else {
			out, err = callTool(ctx, in, DDGSearchTool, map[string]any{"query": query, "max_results": count})
		}
		if err == nil {
			// A plain-text refusal ("[PERMISSION DENIED] …") is not an empty result list.
			err = requireSuccess(out, "search tool")
		}
		results := []any{}
		if err != nil {
			if !noResultsAnswer(err, out) {
				return ExecResult{}, err
			}
		} else {
			list, ok := out["results"].([]any)
			if !ok {
				return ExecResult{}, NewNodeError("FLOW_TOOL_ERROR", "the search tool returned no result list")
			}
			results = normalizeSearchResults(list, count)
		}
		return ExecResult{
			Output:    map[string]any{"results": results, "count": float64(len(results)), "provider": provider, "query": query},
			ItemCount: len(results),
		}, nil
	}
	return def
}

func searchQueryParam(v any) (string, error) {
	s, err := scalarTextParam(v, "the search query")
	if err != nil {
		return "", err
	}
	s = strings.TrimSpace(s)
	if s == "" {
		return "", NewNodeError("FLOW_PARAM_INVALID", "enter a search query")
	}
	if err := checkFileText(s, maxSearchQueryBytes, "search query"); err != nil {
		return "", err
	}
	return s, nil
}

// searchCountParam reads the number of results: null and blank give the default,
// a number is clamped to 1..maxSearchResults (in floating point first, because the
// int conversion of a float out of range is implementation-dependent), anything
// else is an error.
func searchCountParam(v any) (int, error) {
	if s, ok := v.(string); (ok && strings.TrimSpace(s) == "") || v == nil {
		return defaultSearchResults, nil
	}
	f, ok := toNumber(v)
	if !ok {
		return 0, NewNodeError("FLOW_PARAM_INVALID", "the number of results must be a number")
	}
	return int(math.Max(1, math.Min(maxSearchResults, f))), nil
}

// resolveSearchProvider picks the provider: "auto" prefers Brave when it is set up,
// and a provider chosen by name must be available (a configuration error that a
// retry cannot fix, unlike the tool's own refusal).
func resolveSearchProvider(env CatalogEnv, choice string) (string, error) {
	ready := func(tool string) bool { return availabilityOf(env, tool)().State == AvailableState }
	switch choice {
	case "brave":
		if !ready(BraveSearchTool) {
			return "", NewNodeError("FLOW_NODE_UNAVAILABLE", "Brave Search is not set up")
		}
	case "duckduckgo":
		if !ready(DDGSearchTool) {
			return "", NewNodeError("FLOW_NODE_UNAVAILABLE", "DuckDuckGo search is not available")
		}
	default:
		choice = "duckduckgo"
		if ready(BraveSearchTool) {
			choice = "brave"
		}
	}
	return choice, nil
}

// noResultsAnswer reports whether a failed search says there were no results ("No
// results found …"): then the node's result is an empty list. It reads the tool's own
// output, not the node error, whose message is cut and so can lose the words, and it
// looks at the start of the text only: the tool puts its answer first, and a failure
// that merely mentions "no result" later on stays a failure. Only a plain tool error
// counts; a denial or an unavailable tool never reads as "nothing found". The real
// ddg_search says "No parseable DDG results found" for an empty page and for a bot
// check alike; that does not match, so it stays an error (and is retried).
func noResultsAnswer(err error, out map[string]any) bool {
	if asNodeError(err).Code != "FLOW_TOOL_ERROR" {
		return false
	}
	const hint = "no result"
	msg := outText(out)
	return len(msg) >= len(hint) && strings.EqualFold(msg[:len(hint)], hint)
}

func normalizeSearchResults(list []any, limit int) []any {
	out := make([]any, 0, min(len(list), limit))
	for _, item := range list {
		if len(out) >= limit {
			break
		}
		m, _ := item.(map[string]any)
		if m == nil {
			continue
		}
		r := map[string]any{
			"title":   Stringify(m["title"]),
			"url":     firstNonEmpty(Stringify(m["url"]), Stringify(m["link"])),
			"snippet": firstNonEmpty(Stringify(m["snippet"]), Stringify(m["description"])),
		}
		if p := Stringify(m["published"]); p != "" {
			r["published"] = p
		}
		out = append(out, r)
	}
	return out
}

// webReadDef defines web.read. The address is a sensitive sink (an address built
// from untrusted data can carry data out or reach the local network), and the page
// is untrusted.
func webReadDef(env CatalogEnv) *NodeDef {
	def := actionDef(TypeWebRead, "web", "world-www", "web_scraper", env)
	def.UntrustedOutput = true
	def.PrimaryInput = "url"
	def.Params = []ParamSpec{
		{Name: "url", Kind: ParamText, LabelKey: "easydrag.param.url", Required: true, Templatable: true, SensitiveSink: true},
		{Name: "mode", Kind: ParamSelect, LabelKey: "easydrag.param.scrape_mode", Default: "auto",
			Options: []Option{option("auto", "scrape_auto"), option("static", "scrape_static"), option("dynamic", "scrape_dynamic"), option("rss", "scrape_rss")}},
		{Name: "selector", Kind: ParamText, LabelKey: "easydrag.param.css_selector", Templatable: true},
	}
	def.OutputFields = []FieldSpec{{Name: "content", Type: "text", Primary: true}, {Name: "title", Type: "text"},
		{Name: "items", Type: "list"}, {Name: "matches", Type: "list"}, {Name: "url", Type: "text"}}
	def.Validate = func(n *Node, _ ValidateContext) []Issue {
		if n == nil {
			return nil
		}
		issues := choiceIssue(n, "mode", "auto", scrapeModes)
		if sel, ok := n.Params["selector"].(string); ok && strings.TrimSpace(sel) != "" && !HasTemplate(sel) {
			if mode, ok := choiceParam(n.Params["mode"], "auto", scrapeModes...); ok && mode == "rss" {
				issues = append(issues, paramIssue(n, IssueParamInvalid, SeverityError, "selector", "a CSS selector cannot be used in rss mode"))
			}
		}
		return issues
	}
	def.Execute = func(ctx context.Context, in ExecInput) (ExecResult, error) {
		target, err := webURLParam(in.Params["url"])
		if err != nil {
			return ExecResult{}, err
		}
		mode, ok := choiceParam(in.Params["mode"], "auto", scrapeModes...)
		if !ok {
			return ExecResult{}, choiceError("mode", scrapeModes)
		}
		selector, err := scalarTextParam(in.Params["selector"], "the CSS selector")
		if err != nil {
			return ExecResult{}, err
		}
		if selector = strings.TrimSpace(selector); selector != "" {
			if err := checkFileText(selector, maxSelectorBytes, "CSS selector"); err != nil {
				return ExecResult{}, err
			}
			if mode == "rss" {
				// The tool refuses it; say so before a request is made.
				return ExecResult{}, NewNodeError("FLOW_PARAM_INVALID", "a CSS selector cannot be used in rss mode")
			}
		}
		args := map[string]any{"url": target, "mode": mode}
		setArg(args, "selector", selector)
		out, err := callTool(ctx, in, "web_scraper", args)
		if err != nil {
			return ExecResult{}, err
		}
		// A refusal that came as plain text ("[PERMISSION DENIED] web_scraper is disabled
		// in settings") must not become the page's content.
		if err := requireSuccess(out, "web reader"); err != nil {
			return ExecResult{}, err
		}
		result := map[string]any{"url": target, "title": Stringify(out["title"]), "content": Stringify(out["content"])}
		count := 0
		if items, ok := out["items"].([]any); ok {
			result["items"] = items
			count = len(items)
		}
		if matches, ok := out["matches"].([]any); ok {
			result["matches"] = matches
			count = len(matches)
		}
		return ExecResult{Output: result, ItemCount: count}, nil
	}
	return def
}
