package agent

import (
	"strings"
	"unicode/utf8"
)

// Outbound fetch and probe tools are not high-risk, so the copied-span gate above
// never looks at them. An unattended run can still carry text out through them:
// the URL or the host name is chosen by the model, and the destination sees it.
// copiedFetchSpan is the same 48-rune test, restricted to what such a call is
// pointed at, for runs where nobody can answer an approval.

// copiedFetchURLTools take their target as a URL, copiedFetchHostTools as a host.
var (
	copiedFetchURLTools = policyNameSet(
		"api_request", "web_scraper", "web_capture", "web_performance_audit", "site_crawler",
		"browser_automation", "form_automation", "virtual_browser", "yepapi_scrape", "fetch_url", "site_monitor",
	)
	copiedFetchHostTools = policyNameSet("dns_lookup", "network_ping", "port_scanner", "whois_lookup")
)

// copiedFetchTargets returns the URLs or host names an outbound call is pointed at.
func copiedFetchTargets(tc ToolCall) []string {
	var targets []string
	add := func(values ...string) {
		for _, value := range values {
			if value = strings.TrimSpace(value); value != "" {
				targets = append(targets, value)
			}
		}
	}
	if _, ok := copiedFetchURLTools[tc.Action]; ok {
		add(tc.URL, toolArgString(tc.Params, "url"))
		add(toolArgStringSlice(tc.Params, "urls")...)
		return targets
	}
	if _, ok := copiedFetchHostTools[tc.Action]; ok {
		add(tc.Host, toolArgString(tc.Params, "host", "domain", "target", "url"))
	}
	return targets
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
