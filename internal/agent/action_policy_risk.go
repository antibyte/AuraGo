package agent

import (
	"log/slog"
	"math"
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
// new endpoint, hand work or a credential to an external agent or machine, or start a
// call. The keyword list in DescribeAction already covers delete, send, upload,
// publish and deploy. This table names the rest. It is deliberately not a global
// "update" or "create" keyword, because most of those operations act on private,
// local or read-only state. TestExternalReachOperationsAreClassified walks the
// builtin schemas of every tool named here and fails on an operation that is
// neither a read, nor high-risk, nor on its explicit list of ordinary operations.
var externalReachOperations = map[string]map[string]struct{}{
	"here_now_site":            policyNameSet("update", "duplicate", "update_metadata", "update_access", "set_password", "restore_version"),
	"onedrive":                 policyNameSet("share", "write"),
	"webdav":                   policyNameSet("write"),
	"koofr":                    policyNameSet("write"),
	"github":                   policyNameSet("create_repo", "create_issue", "create_or_update_file"),
	"huggingface":              policyNameSet("create_repo", "create_discussion", "comment_discussion", "job_run_python", "job_run_uv_script", "job_run_container"),
	"netlify":                  policyNameSet("create_site", "update_site", "rollback", "set_env", "create_hook"),
	"vercel":                   policyNameSet("create_project", "update_project", "rollback", "set_env", "add_domain", "assign_alias"),
	"homepage_deploy":          policyNameSet("tunnel", "webserver_start"),
	"homepage_project":         policyNameSet("tunnel", "webserver_start"),
	"s3_storage":               policyNameSet("copy", "move"), // Cross-bucket copies can land in a bucket that is served publicly.
	"agentmail_inboxes":        policyNameSet("create_inbox"), // A new mailbox is a new address on the internet.
	"virtual_workspace":        policyNameSet("request_credential_grant"),
	"virtual_computers":        policyNameSet("run_shell_task", "run_desktop_task"), // Hand an instruction to the boringd agent.
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

// unattendedReachOperations are operations that run code on an external machine. A
// user who watches the chat sees them and the workspace; an unattended run has the
// workspace's network profile (internet and LAN by default) and nobody to look, so
// there they need a rule like any other high-risk action.
var unattendedReachOperations = map[string]map[string]struct{}{
	"virtual_workspace": policyNameSet("exec", "start_job", "job_input"),
	"virtual_computers": policyNameSet("exec"),
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
	probeTextMin       = 8   // Shortest text judged at all.
	probeLabelTextMin  = 16  // Shortest single label judged by its letters.
	probeCarrierMin    = 24  // Shortest run of several labels judged by its letters.
	probeDigitsMin     = 16  // Digits only: no name has this many (a dashed IP has 12).
	probeHexMin        = 12  // Hex with both letters and digits.
	probeSwitchMin     = 4   // Letter/digit changes inside one segment of 8 or more characters.
	probeEntropyMin    = 3.3 // Bits per character below which a text is a word or a repeat.
	probeVowelMax      = 0.25
	probeVowelFloor    = 0.10
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
// merged argument map DescribeAction built from the call and its Params; dc is the
// run the call belongs to and may be nil.
func applyExtendedActionRisk(a *ActionDescriptor, tc ToolCall, dc *DispatchContext, args map[string]interface{}) {
	tool := a.Tool
	if alias, ok := policyToolAliases[tool]; ok {
		tool = alias
	}
	if schedulingActionHighRisk(tool, a.Operation, a.RunType) {
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
		a.HighRisk = a.HighRisk || probeHostLooksLikeExfiltration(decodeDNSLookupArgs(tc).Host) || copiedFetchSpan(tc, dc)
	case "network_ping":
		a.HighRisk = a.HighRisk || probeHostLooksLikeExfiltration(decodeNetworkPingArgs(tc).Host) || copiedFetchSpan(tc, dc)
	case "port_scanner":
		a.HighRisk = a.HighRisk || probeHostLooksLikeExfiltration(decodePortScannerArgs(tc).Host) || copiedFetchSpan(tc, dc)
	}
	if _, ok := externalReachOperations[tool][a.Operation]; ok {
		a.Mutating, a.HighRisk = true, true
	}
	if _, ok := unattendedReachOperations[tool][a.Operation]; ok && a.RunType != "interactive" {
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
// follow_up and wait_for_event queue a model-written prompt for a later background
// run. In an interactive chat they stay ordinary: the user is there, a chain of
// follow-ups is how the agent works through a long task, and the queued run is held
// to the same rules as any other unattended run. In an unattended run they need a
// rule, so an injected turn cannot extend itself past the trigger on its own.
func schedulingActionHighRisk(tool, operation, runType string) bool {
	switch tool {
	case "follow_up", "wait_for_event":
		return runType != "interactive"
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
// names a machine: a label over 24 characters, a label that reads as encoded text
// (see probeTextLooksEncoded), the labels left of the registered domain read as one
// text, or a name over 64 characters. Plain hostnames, IP literals and reverse-lookup
// names pass. The resolver forwards every label to the zone's authoritative server,
// so this is the bandwidth of a DNS side channel. It is a heuristic: a host spelled
// in short, ordinary-looking words still passes, and the structural answer is a host
// allowlist (deferred).
func probeHostLooksLikeExfiltration(raw string) bool {
	host := probeHostName(raw)
	if host == "" || net.ParseIP(host) != nil || strings.HasSuffix(host, ".arpa") {
		return false
	}
	if len(host) > probeHostMaxLength {
		return true
	}
	labels := strings.Split(host, ".")
	for _, label := range labels {
		if len(label) > probeHostMaxLabel || probeTextLooksEncoded(probeSegments(label), probeLabelTextMin) {
			return true
		}
	}
	if len(labels) > 3 {
		// Data spread over several short labels: read everything left of the last two.
		var carrier []string
		for _, label := range labels[:len(labels)-2] {
			carrier = append(carrier, probeSegments(label)...)
		}
		return probeTextLooksEncoded(carrier, probeCarrierMin)
	}
	return false
}

// probeSegments splits a label at hyphens and underscores: "mfrg-ggzd-fmzt" is the
// text of one label whose hyphens do not make it a name.
func probeSegments(label string) []string {
	return strings.FieldsFunc(label, func(r rune) bool { return r == '-' || r == '_' })
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

// probeTextLooksEncoded judges the text of a label (its segments, joined) or of a
// run of labels. Whole numeric segments of up to three digits are dropped first: an
// octet or a counter ("ec2-203-0-113-25") is part of a name. The rest counts as data
// when it is, in order:
//   - 16 or more digits and nothing else,
//   - hex with both letters and digits, 12 or more characters,
//   - a segment of 8 or more characters that changes between letters and digits at
//     least 4 times ("k3j9x0q7w2z"),
//   - at least textMin characters of letters and digits whose vowels are rare among the
//     letters (language has about a third; base32, base36 and random letters about a
//     fifth) and whose characters are spread (Shannon entropy of 3.3 bits or more), or
//     almost vowel-free at any spread.
//
// Hyphens do not hide anything: "mfrg-ggzd-fmzt-wq2l-knnw" is read as one text.
func probeTextLooksEncoded(segments []string, textMin int) bool {
	kept := make([]string, 0, len(segments))
	for _, segment := range segments {
		if len(segment) <= 3 && strings.Trim(segment, "0123456789") == "" {
			continue
		}
		kept = append(kept, segment)
	}
	text := strings.Join(kept, "")
	if len(text) < probeTextMin {
		return false
	}
	letters, digits, vowels := 0, 0, 0
	hex := true
	for i := 0; i < len(text); i++ {
		c := text[i]
		if c >= '0' && c <= '9' {
			digits++
		} else {
			letters++
			if strings.IndexByte("aeiou", c) >= 0 {
				vowels++
			}
		}
		if !(c >= '0' && c <= '9' || c >= 'a' && c <= 'f') {
			hex = false
		}
	}
	if letters == 0 {
		return len(text) >= probeDigitsMin
	}
	if hex && digits > 0 && len(text) >= probeHexMin {
		return true
	}
	for _, segment := range kept {
		switches := 0
		for i := 1; i < len(segment); i++ {
			if (segment[i] >= '0' && segment[i] <= '9') != (segment[i-1] >= '0' && segment[i-1] <= '9') {
				switches++
			}
		}
		if len(segment) >= probeTextMin && switches >= probeSwitchMin {
			return true
		}
	}
	if len(text) < textMin || letters < probeTextMin {
		return false
	}
	vowelShare := float64(vowels) / float64(letters)
	return vowelShare <= probeVowelFloor || vowelShare <= probeVowelMax && textEntropy(text) >= probeEntropyMin
}

// textEntropy is the Shannon entropy of the bytes of text, in bits per byte.
func textEntropy(text string) float64 {
	var counts [256]int
	for i := 0; i < len(text); i++ {
		counts[text[i]]++
	}
	total := float64(len(text))
	bits := 0.0
	for _, count := range counts {
		if count > 0 {
			share := float64(count) / total
			bits -= share * math.Log2(share)
		}
	}
	return bits
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
