package agent

import (
	"log/slog"
	"net"
	"strings"

	"aurago/internal/security"
)

// Risk classes that DescribeAction cannot derive from a keyword in the operation
// name. They only ever raise a descriptor to HighRisk (or relabel a scheduling
// call); they never lower what the switch in DescribeAction decided.

// policyToolAliases maps the legacy spellings the dispatcher still accepts to the
// canonical tool the tables below are keyed by.
var policyToolAliases = map[string]string{
	"homeassistant": "home_assistant", "ha": "home_assistant",
	"onedrive_op": "onedrive", "koofr_api": "koofr", "koofr_op": "koofr",
	"gworkspace": "google_workspace", "webdav_storage": "webdav",
	"homepage": "homepage_project", "homepage_tool": "homepage_project",
}

func policyNameSet(operations ...string) map[string]struct{} {
	set := make(map[string]struct{}, len(operations))
	for _, operation := range operations {
		set[operation] = struct{}{}
	}
	return set
}

// externalReachOperations lists, per tool, the operations that put agent-chosen
// content or access in front of third parties: they publish, share, expose a local
// service, create a remote object that carries text, point a remote service at a
// new endpoint, hand work to an external agent, or start a call. The keyword list in
// DescribeAction already covers delete, send, upload, publish and deploy. This table
// names the rest. It is deliberately not a global "update" or "create" keyword,
// because most of those operations act on private, local or read-only state.
// TestExternalReachOperationsAreHighRisk walks the builtin schemas against it.
var externalReachOperations = map[string]map[string]struct{}{
	"here_now_site":            policyNameSet("update", "duplicate", "update_metadata", "update_access", "set_password", "restore_version"),
	"onedrive":                 policyNameSet("share", "write"),
	"webdav":                   policyNameSet("write"),
	"koofr":                    policyNameSet("write"),
	"github":                   policyNameSet("create_repo", "create_issue", "create_or_update_file"),
	"huggingface":              policyNameSet("create_repo", "create_discussion", "comment_discussion", "job_run_python", "job_run_uv_script", "job_run_container"),
	"netlify":                  policyNameSet("create_site", "update_site", "rollback", "set_env", "create_hook"),
	"vercel":                   policyNameSet("create_project", "update_project", "rollback", "set_env", "add_domain", "assign_alias"),
	"homepage_deploy":          policyNameSet("tunnel"),
	"homepage_project":         policyNameSet("tunnel"),
	"cloudflare_tunnel":        policyNameSet("start", "quick_tunnel"),
	"fritzbox_network":         policyNameSet("add_port_forward"),
	"network_shares":           policyNameSet("create", "update"),
	"manage_outgoing_webhooks": policyNameSet("create", "update"),
	"google_workspace":         policyNameSet("calendar_create", "calendar_update"),
	"gog_calendar":             policyNameSet("create", "update"),
	"gog_drive":                policyNameSet("replace", "import_office"),
	"telnyx_call":              policyNameSet("initiate", "speak", "play_audio", "transfer"),
	"sip_phone":                policyNameSet("dial"),
	"composio_call":            policyNameSet("execute_tool"),
	"manus":                    policyNameSet("create_task"),
	"evomap":                   policyNameSet("submit_report", "kg_ingest"),
}

// Home Assistant domains whose services open, unlock, disarm, run user-defined
// code or call arbitrary URLs. The generic homeassistant domain and hassio act on
// any entity or on the host, so they count too. Lights, switches, media players,
// climate and fans are deliberately absent so they keep working without a prompt.
var homeAssistantHighRiskDomains = policyNameSet(
	"lock", "alarm_control_panel", "cover", "script", "automation", "shell_command",
	"siren", "valve", "homeassistant", "hassio", "rest_command", "python_script", "pyscript",
)

const (
	probeHostMaxLabel  = 24 // A longer DNS label is a data carrier, not a name.
	probeHostMaxLength = 64
	probeHexLabelMin   = 12 // Shortest all-hex label with letters and digits.
	probeMixedLabelMin = 16 // Shortest letters-and-digits label judged by its mix.
)

// egressPolicyKeysSkipped are top-level keys that are never scanned for secrets.
// The session todo and the guardian notes ride along on the call without being
// sent, and a recipient or account is an identity (an SMTP user name is registered
// with the scrubber) that the target rules and the approval dialog already cover.
var egressPolicyKeysSkipped = policyNameSet(
	"_todo", "_guardian_justification", "_guardian_blocked", "_guardian_block_reason",
	"to", "cc", "bcc", "from", "reply_to", "recipient", "recipients", "account", "account_id", "inbox_id",
)

// applyExtendedActionRisk raises a descriptor for the classes above. args is the
// merged argument map DescribeAction built from the call and its Params.
func applyExtendedActionRisk(a *ActionDescriptor, tc ToolCall, args map[string]interface{}) {
	tool := a.Tool
	if alias, ok := policyToolAliases[tool]; ok {
		tool = alias
	}
	if schedulingActionHighRisk(tool, a.Operation) {
		// One operation for the whole class: a rule for "schedule" covers what an
		// agent can persist or start, whatever the tool spells it.
		a.Operation, a.Mutating, a.HighRisk = "schedule", true, true
	}
	switch tool {
	case "home_assistant":
		if homeAssistantServiceHighRisk(tc, a.Operation) {
			a.Mutating, a.HighRisk = true, true
		}
	case "docker":
		if a.Operation == "stop" || a.Operation == "pause" {
			a.Mutating, a.HighRisk = true, true
		}
	case "dns_lookup":
		a.HighRisk = a.HighRisk || probeHostLooksLikeExfiltration(decodeDNSLookupArgs(tc).Host)
	case "network_ping":
		a.HighRisk = a.HighRisk || probeHostLooksLikeExfiltration(decodeNetworkPingArgs(tc).Host)
	case "port_scanner":
		a.HighRisk = a.HighRisk || probeHostLooksLikeExfiltration(decodePortScannerArgs(tc).Host)
	}
	if _, ok := externalReachOperations[tool][a.Operation]; ok {
		a.Mutating, a.HighRisk = true, true
	}
	if egressPolicyTool(*a) {
		registered, matching := egressArgumentSecrets(args)
		a.egressSecret = registered
		a.HighRisk = a.HighRisk || registered || matching
	}
}

// schedulingActionHighRisk is true for the calls that store or start agent work that
// outlives the turn: a mission or cron prompt is read again later with the full tool
// catalog and no human in the loop. Listing, history and disable stay ordinary.
func schedulingActionHighRisk(tool, operation string) bool {
	switch tool {
	case "manage_missions":
		switch operation {
		case "create", "add", "update", "edit", "run", "run_now", "execute":
			return true
		}
	case "cron_scheduler", "manage_schedule":
		switch operation {
		case "add", "enable", "update":
			return true
		}
	case "schedule_cron":
		return true // The tool has one effect: it adds a job.
	}
	return false
}

// homeAssistantServiceHighRisk reports a service call on a security-relevant
// domain. The service may carry its own domain prefix ("lock.unlock").
func homeAssistantServiceHighRisk(tc ToolCall, operation string) bool {
	if operation != "call_service" && operation != "service" {
		return false
	}
	req := decodeHomeAssistantArgs(tc)
	domains := []string{req.Domain}
	if prefix, _, found := strings.Cut(req.Service, "."); found {
		domains = append(domains, prefix)
	}
	for _, domain := range domains {
		if _, risky := homeAssistantHighRiskDomains[strings.ToLower(strings.TrimSpace(domain))]; risky {
			return true
		}
	}
	return false
}

// probeHostLooksLikeExfiltration reports a DNS name that carries data rather than
// names a machine: a label over 24 characters, a hex or base32-like label (see
// probeLabelLooksEncoded), or a name over 64 characters. Plain hostnames, IP
// literals and reverse-lookup names pass. The resolver forwards every label to the
// zone's authoritative server, so this is the bandwidth of a DNS side channel.
func probeHostLooksLikeExfiltration(raw string) bool {
	host := probeHostName(raw)
	if host == "" || net.ParseIP(host) != nil || strings.HasSuffix(host, ".arpa") {
		return false
	}
	if len(host) > probeHostMaxLength {
		return true
	}
	for _, label := range strings.Split(host, ".") {
		if len(label) > probeHostMaxLabel || probeLabelLooksEncoded(label) {
			return true
		}
	}
	return false
}

// probeHostName reduces a host argument to the name that is resolved.
func probeHostName(raw string) string {
	host := strings.ToLower(strings.TrimSpace(raw))
	if _, rest, found := strings.Cut(host, "://"); found {
		host = rest
	}
	if cut := strings.IndexAny(host, "/?#"); cut >= 0 {
		host = host[:cut]
	}
	if at := strings.LastIndex(host, "@"); at >= 0 {
		host = host[at+1:]
	}
	if split, _, err := net.SplitHostPort(host); err == nil {
		host = split
	}
	return strings.Trim(host, "[].")
}

// probeLabelLooksEncoded flags a label made only of letters and digits that is
// either hex (a-f with at least one digit, 12 or more characters) or has a digit
// run between letters (16 or more characters), which is how base32 text looks.
// A hyphen or underscore marks an ordinary name; "ubuntuserver2204" passes.
func probeLabelLooksEncoded(label string) bool {
	if len(label) < probeHexLabelMin {
		return false
	}
	letters, digits, switches := 0, 0, 0
	hex := true
	for i := 0; i < len(label); i++ {
		c := label[i]
		digit := c >= '0' && c <= '9'
		switch {
		case digit:
			digits++
		case c >= 'a' && c <= 'z':
			letters++
		default:
			return false
		}
		if !digit && c > 'f' {
			hex = false
		}
		if i > 0 && digit != (label[i-1] >= '0' && label[i-1] <= '9') {
			switches++
		}
	}
	return letters > 0 && digits > 0 && (hex || len(label) >= probeMixedLabelMin && switches >= 2)
}

// egressPolicyTool reports a call that sends something it was given to a party
// outside the host: HTTP, messages, webhooks, DNS and ping probes, web fetches.
func egressPolicyTool(a ActionDescriptor) bool {
	switch a.Tool {
	case "api_request", "call_webhook", "telnyx_sms",
		"send_email", "send_telegram", "send_discord", "send_rocketchat", "send_push", "send_push_notification",
		"send_notification", "send_sms", "send_voice", "send_document", "send_image", "send_audio", "send_video",
		"dns_lookup", "network_ping", "port_scanner", "whois_lookup",
		"web_scraper", "web_capture", "web_performance_audit", "site_crawler", "browser_automation", "form_automation",
		"virtual_browser", "yepapi_scrape":
		return true
	case "agentmail", "agentmail_messages", "agentmail_drafts":
		return a.HighRisk // Only the sending operations are high-risk here.
	}
	return false
}

// egressArgumentSecrets scans the string values of a call. registered is true when
// a value the scrubber registry knows (vault and integration secrets) appears in
// one. matching is true when only the redaction patterns (key=value, bearer tokens,
// URL credentials) fire. A short map entry (a header) is also tested as "name: value".
// The skipped keys apply to the top level only, so nothing hides in a nested object.
func egressArgumentSecrets(args map[string]interface{}) (registered, matching bool) {
	const pairMaxBytes = 512
	test := func(text string) {
		if text == "" || registered {
			return
		}
		if security.Scrub(text) != text {
			registered = true
		} else if security.RedactSensitiveInfo(text) != text {
			matching = true
		}
	}
	var visit func(node interface{}, depth int)
	entry := func(depth int, key string, item interface{}) {
		if depth == 0 {
			if _, skip := egressPolicyKeysSkipped[strings.ToLower(key)]; skip {
				return
			}
		}
		if text, ok := item.(string); ok && len(text) <= pairMaxBytes {
			test(key + ": " + text)
		}
		visit(item, depth+1)
	}
	visit = func(node interface{}, depth int) {
		if depth > 64 || registered {
			return
		}
		switch value := node.(type) {
		case string:
			test(value)
		case []string:
			for _, item := range value {
				test(item)
			}
		case []interface{}:
			for _, item := range value {
				visit(item, depth+1)
			}
		case map[string]string:
			for key, item := range value {
				entry(depth, key, item)
			}
		case map[string]interface{}:
			for key, item := range value {
				entry(depth, key, item)
			}
		}
	}
	visit(args, 0)
	return registered, matching
}

// egressSecretRefusal stops an egress call whose arguments hold a registered secret.
// The text is fixed so the model learns nothing about which value matched, and an
// approval cannot lift it: the value must never leave the host.
func egressSecretRefusal(dc *DispatchContext, a ActionDescriptor) *ToolDispatchResult {
	logger := slog.Default()
	if dc != nil && dc.Logger != nil {
		logger = dc.Logger
	}
	logger.Warn("Egress call refused: arguments contain a protected secret",
		"tool", a.Tool, "operation", a.Operation, "run_type", a.RunType, "arguments_hash", a.ArgumentsHash)
	return actionPolicyRefusal("egress_secret_refused", "The call was refused because its arguments contain a protected secret value. Secrets are never sent through tool arguments; use a configured integration instead.", ToolResultDenied)
}
