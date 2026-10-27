package agent

import (
	"encoding/json"
	"sort"
	"strings"
	"unicode/utf8"
)

// Outbound fetch and probe tools are not high-risk, so the copied-span gate above
// never looks at them. An unattended run can still carry text out through them:
// the URL or the host name is chosen by the model, and the destination sees it.
// copiedFetchSpan is the same 48-rune test, restricted to what such a call is
// pointed at, for runs where nobody can answer an approval.

// A call is pointed at what its arguments say, not at what its tool is called: a
// tool added later is covered as soon as it has a url or host parameter, and a
// table over the builtin schemas (TestEveryDestinationParameterIsAFetchTarget) fails
// when a new one is not recognised. The names below are the exceptions.
//
// copiedFetchHostTools take a host where other tools take a URL: their target and
// url parameters, and for certificate_manager its hostname, are host names.
var copiedFetchHostTools = policyNameSet("dns_lookup", "network_ping", "port_scanner", "whois_lookup", "certificate_manager")

// notDestinationParameters name a parameter that looks like a destination and is not
// one: it selects local state and nothing is sent to it.
var notDestinationParameters = policyNameSet("home_assistant.domain")

// fetchURLKey is a parameter that holds a URL (or a list of them) whatever its value.
func fetchURLKey(key string) bool {
	switch key {
	case "url", "urls", "username_or_url":
		return true
	}
	return strings.HasSuffix(key, "_url") || strings.HasSuffix(key, "_urls")
}

// fetchHostKey is a parameter that holds a host or domain name.
func fetchHostKey(key string) bool {
	switch key {
	case "host", "domain", "custom_domain":
		return true
	}
	return false
}

// fetchConditionalKey is a name generic enough to hold a local path or an identifier
// too: its value is a destination only when it is an http(s) URL.
func fetchConditionalKey(key string) bool {
	switch key {
	case "path", "endpoint", "target":
		return true
	}
	return false
}

func looksLikeHTTPURL(value string) bool {
	value = strings.ToLower(strings.TrimSpace(value))
	return strings.HasPrefix(value, "http://") || strings.HasPrefix(value, "https://")
}

// argumentStrings reads a string or a list of strings from an argument value.
func argumentStrings(value interface{}) []string {
	switch typed := value.(type) {
	case string:
		return []string{typed}
	case []string:
		return typed
	case []interface{}:
		out := make([]string, 0, len(typed))
		for _, item := range typed {
			if text, ok := item.(string); ok {
				out = append(out, text)
			}
		}
		return out
	}
	return nil
}

// fetchTargetsFromArguments returns the URLs and host names a call is pointed at, from
// the merged argument map DescribeAction builds (the call's own fields plus Params).
// The result is sorted so the same call always yields the same list.
func fetchTargetsFromArguments(tool string, args map[string]interface{}) []string {
	_, hostTool := copiedFetchHostTools[tool]
	var targets []string
	for key, value := range args {
		key = strings.ToLower(key)
		if _, skip := notDestinationParameters[tool+"."+key]; skip {
			continue
		}
		onlyHTTP := false
		switch {
		case fetchURLKey(key), fetchHostKey(key):
		case hostTool && (key == "target" || key == "hostname"):
		case fetchConditionalKey(key):
			onlyHTTP = true
		default:
			continue
		}
		for _, text := range argumentStrings(value) {
			if text = strings.TrimSpace(text); text != "" && (!onlyHTTP || looksLikeHTTPURL(text)) {
				targets = append(targets, text)
			}
		}
	}
	sort.Strings(targets)
	return targets
}

// toolCallArguments is the merged argument map of a call: its own fields as JSON,
// overridden by Params, as DescribeAction reads them.
func toolCallArguments(tc ToolCall) map[string]interface{} {
	raw, err := json.Marshal(tc)
	if err != nil {
		return nil
	}
	var args map[string]interface{}
	if json.Unmarshal(raw, &args) != nil {
		return nil
	}
	for key, value := range tc.Params {
		args[key] = value
	}
	return args
}

// copiedFetchTargets returns the URLs or host names an outbound call is pointed at.
func copiedFetchTargets(tc ToolCall) []string {
	return fetchTargetsFromArguments(tc.Action, toolCallArguments(tc))
}

// copiedFetchForms are the spellings of a target compared with the indexed texts:
// as written, and with percent-encoding resolved (text copied into a query string is
// encoded). All are lowercase, because host names and schemes ignore case. A stray
// percent sign in the target does not stop the rest from being decoded.
func copiedFetchForms(target string) []string {
	plain := strings.ToLower(normalizeCopiedText(target))
	forms := []string{plain}
	decoded := target
	if rounds := percentDecodedRounds(target); len(rounds) > 0 {
		decoded = rounds[len(rounds)-1]
	}
	if form := strings.ToLower(normalizeCopiedText(decoded)); form != plain {
		forms = append(forms, form)
	}
	return forms
}

// copiedFetchSpan reports whether an outbound fetch or probe carries a 48-rune span
// of earlier isolated tool output in its URL or host. A target that appears whole in
// an earlier output is a link the model followed, not text moved into an address, and
// does not count; a link that gains copied text (a query string built from a mail)
// does.
func copiedFetchSpan(tc ToolCall, dc *DispatchContext) bool {
	if dc == nil || dc.SessionID == "" {
		return false
	}
	targets := copiedFetchTargets(tc)
	if len(targets) == 0 {
		return false
	}
	views := modelIsolatedBodies.views(dc.SessionID)
	if len(views) == 0 {
		return false
	}
	lowered := make([]string, len(views))
	for i, view := range views {
		lowered[i] = strings.ToLower(view)
	}
	for _, target := range targets {
		forms := copiedFetchForms(target)
		long, followed := false, false
		for _, form := range forms {
			long = long || utf8.RuneCountInString(form) >= copiedSpanRunes
			for _, view := range lowered {
				followed = followed || strings.Contains(view, form)
			}
		}
		if !long || followed {
			continue
		}
		for _, form := range forms {
			if copiesViews(lowered, []rune(form)) {
				return true
			}
		}
	}
	return false
}
