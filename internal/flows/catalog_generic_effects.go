package flows

import "strings"

// Effects of generic tool nodes. A generic node knows a tool only by its name, its
// agent category and the literal value of its operation parameter, so its effects are
// derived from lists of the real AuraGo tools (internal/agent/tool_categories.go and
// the native tool schemas) and from the words of the operation. The rule is to rather
// list an effect too many: an operation that is not a known read lists what the tool
// can do, and an operation that is not a literal (a template, a value of another type)
// lists the worst case (genericWorstEffects), never "no effects".

var (
	// codeRunningTools run a program, a command or a statement the call carries, or
	// hand a task to something that can, whatever the operation says.
	codeRunningTools = map[string]bool{
		"execute_shell": true, "execute_python": true, "execute_sandbox": true, "execute_sudo": true,
		"remote_execution": true, "remote_control_shell": true,
		// older names of the remote shell tools (internal/toolmeta)
		"execute_remote_shell": true, "ssh_exec": true,
		// a root shell in a VM, keystrokes on a remote desktop
		"virtual_workspace": true, "remote_control_desktop": true,
		// tasks for other agents, which can run commands where they live
		"space_agent": true, "invasion_tasks": true,
		// arbitrary SQL statements (see programTools)
		"sql_query": true,
		// app and widget code that runs in the user's desktop UI
		"virtual_desktop_app_install": true, "virtual_desktop_widgets": true,
	}
	// programTools say what they do in a parameter (the statement), not in the
	// operation: even a "query" can drop a table, so no operation makes them a read.
	programTools = map[string]bool{"sql_query": true}
	// systemChangingTools change the host, its services, devices on the network or the
	// infrastructure and accounts AuraGo manages.
	systemChangingTools = map[string]bool{"docker": true, "process_management": true, "package_manager": true,
		"firewall": true, "proxmox": true, "ansible": true, "tailscale": true, "cloudflare_tunnel": true,
		"virtual_computers": true, "manage_updates": true, "execute_sudo": true, "truenas": true,
		"network_shares": true, "adguard": true, "fritzbox_system": true, "fritzbox_network": true,
		"fritzbox_storage": true, "meshcentral": true, "ollama": true, "homepage_project": true,
		"homepage_deploy": true, "netlify": true, "vercel": true, "here_now_site": true,
		"manage_sql_connections": true, "manage_webhooks": true, "manage_outgoing_webhooks": true,
		"ldap": true, "invasion_nests": true, "remote_control_files": true, "remote_control_devices": true,
		"virtual_workspace": true}
	// messagingTools reach someone outside AuraGo although their name does not start
	// with send_ (a send_ tool always does).
	messagingTools = map[string]bool{"call_webhook": true, "sip_phone": true, "telnyx_call": true}
	// sendWordTools outside the communication category send a message when their
	// operation has a send word (google_workspace "gmail_send").
	sendWordTools = map[string]bool{"google_workspace": true}
	// fileTools work on files outside the files category, like every tool whose name
	// says "file" or "document" (genericFileish): an operation with a write word writes
	// files. In the files category every operation that is no read and no pure delete does.
	fileTools = map[string]bool{"office_workbook": true, "video_download": true, "s3_storage": true,
		"browser_automation": true, "invasion_artifacts": true}
	// fileWritingTools write a file with every call that is not a read: a transfer, a
	// capture, a conversion, a rendering, a generated medium or certificate.
	fileWritingTools = map[string]bool{"transfer_remote_file": true, "web_capture": true, "media_conversion": true,
		"tts": true, "generate_image": true, "generate_music": true, "generate_video": true, "openscad_render": true,
		"certificate_manager": true}
	// deviceTools control devices outside the smart_home category.
	deviceTools = map[string]bool{"bluetooth": true, "chromecast": true, "three_d_printer": true,
		"cyd_display": true, "jellyfin": true, "meshcentral": true, "remote_control_desktop": true}

	// readOperationPrefixes are the verbs that start a read: the first word of the
	// operation must be one of them exactly ("list_devices", not "listen").
	readOperationPrefixes = []string{"get", "list", "read", "search", "status", "stats", "stat", "statistics",
		"fetch", "query", "describe", "info", "health", "find", "count", "inspect", "logs", "diff", "validate",
		"keys", "head", "tail", "analyze", "sample", "summarize", "detect", "whoami", "check", "test", "ping",
		"grep", "glob", "recent"}
	// actionWords make an operation that starts like a read a change after all
	// ("query_log_clear", "get_and_delete").
	actionWords = []string{"delete", "remove", "purge", "destroy", "drop", "clear", "wipe", "erase", "prune",
		"reset", "kill", "stop", "start", "restart", "reboot", "shutdown", "set", "update", "create", "write",
		"add", "put", "post", "send", "reply", "forward", "call", "install", "uninstall", "upgrade", "run",
		"exec", "execute", "apply", "toggle", "enable", "disable", "move", "copy", "rename", "upload", "download",
		"save", "mark", "ack", "publish", "deploy", "rollback", "restore", "import", "merge", "modify", "patch",
		"edit", "register", "assign", "cancel", "complete", "pull", "push", "sync", "generate", "convert",
		"spawn", "hatch", "claim", "submit", "approve", "revoke", "grant", "fill", "lock", "unlock", "encrypt",
		"decrypt", "split", "compress", "extract", "dial", "answer", "reject", "hangup", "transfer", "initiate",
		"record", "schedule", "pause", "resume", "optimize", "ingest", "upsert", "insert", "append", "prepend",
		"replace", "attach", "detach"}
	// codeOperationWords are operation words that run a program (docker "run",
	// meshcentral "run_command", ansible "adhoc", huggingface "job_run_python").
	codeOperationWords = []string{"exec", "execute", "run", "adhoc", "playbook"}
	// deleteWords are matched anywhere in the operation, as in "remove_image".
	deleteWords = []string{"delete", "remove", "purge", "destroy", "drop", "clear", "wipe", "erase", "prune", "uninstall"}
	// writeWords make an operation of a fileTools entry a write, and a files operation
	// that also deletes ("delete_and_write") one.
	writeWords = []string{"write", "edit", "patch", "download", "upload", "save", "create", "move", "copy", "rename",
		"mkdir", "export", "set", "replace", "append", "insert", "optimize", "convert"}
	// sendWords make an operation of a communication tool a message.
	sendWords = []string{"send", "post", "reply", "call", "sms", "forward", "initiate", "dial", "notify"}
)

// maxOperationBytes bounds an operation the hooks look at; a longer one is not an
// operation name and gets the worst case.
const maxOperationBytes = 100

// isOperationName reports whether op (lower case, trimmed) is a literal operation
// name: letters, digits and _ . : - only. A template ("get_{{x}}", "\{{x}}") or any
// other text is not, so it can never be classified as a read.
func isOperationName(op string) bool {
	if op == "" || len(op) > maxOperationBytes {
		return false
	}
	for i := 0; i < len(op); i++ {
		c := op[i]
		if !(c >= 'a' && c <= 'z' || c >= '0' && c <= '9' || c == '_' || c == '.' || c == ':' || c == '-') {
			return false
		}
	}
	return true
}

func operationWords(op string) []string {
	return strings.FieldsFunc(op, func(r rune) bool { return r == '_' || r == '.' || r == ':' || r == '-' })
}

func hasWord(words []string, set ...string) bool {
	for _, w := range words {
		for _, s := range set {
			if w == s {
				return true
			}
		}
	}
	return false
}

// isReadOperation reports whether op is a literal operation that only reads: its
// first word is a read verb and none of its words is an action ("query_log_clear" is
// not a read). Templates and anything that is not an operation name are not reads.
func isReadOperation(op string) bool {
	op = strings.ToLower(strings.TrimSpace(op))
	if !isOperationName(op) {
		return false
	}
	words := operationWords(op)
	return len(words) > 0 && hasWord(words[:1], readOperationPrefixes...) && !hasWord(words, actionWords...)
}

func containsAny(s string, parts ...string) bool {
	for _, p := range parts {
		if strings.Contains(s, p) {
			return true
		}
	}
	return false
}

// effectSet collects effects; list returns them in effectOrder, nil when empty.
type effectSet map[Effect]bool

func (s effectSet) list() []Effect {
	var out []Effect
	for _, e := range effectOrder {
		if s[e] {
			out = append(out, e)
		}
	}
	return out
}

func genericCanSend(tool, category string) bool {
	if strings.HasPrefix(tool, "fetch_") || strings.HasPrefix(tool, "list_") {
		return false
	}
	return strings.HasPrefix(tool, "send_") || messagingTools[tool] || sendWordTools[tool] || category == "communication"
}

// genericFileish reports whether a tool outside the files category works on files.
func genericFileish(tool string) bool {
	return fileTools[tool] || strings.Contains(tool, "file") || strings.Contains(tool, "document")
}

// genericEffects derives the effects of a generic tool call from tool name, category
// and the literal operation op ("" for a tool without one). A read operation (see
// isReadOperation) has no effects, unless the tool is a programTools entry. An op that
// is not an operation name (a template, other text) gives genericAllEffects.
func genericEffects(tool, category, op string) []Effect {
	op = strings.ToLower(strings.TrimSpace(op))
	if op != "" && !isOperationName(op) {
		return genericAllEffects(tool, category)
	}
	if op != "" && isReadOperation(op) && !programTools[tool] {
		return nil
	}
	words := operationWords(op)
	set := effectSet{}
	if codeRunningTools[tool] || hasWord(words, codeOperationWords...) {
		set[EffectRunsCode] = true
	}
	deletes := containsAny(op, deleteWords...)
	if deletes {
		set[EffectDeletes] = true
	}
	if systemChangingTools[tool] {
		set[EffectSystemChange] = true
	}
	if genericCanSend(tool, category) && (strings.HasPrefix(tool, "send_") || messagingTools[tool] || containsAny(op, sendWords...)) {
		set[EffectSendsMessage] = true
	}
	switch {
	case category == "files":
		set[EffectWritesFiles] = op != "" && (!deletes || containsAny(op, writeWords...))
	case fileWritingTools[tool]:
		set[EffectWritesFiles] = true
	case genericFileish(tool):
		set[EffectWritesFiles] = containsAny(op, writeWords...)
	}
	if category == "smart_home" || deviceTools[tool] {
		set[EffectControlsDevices] = true
	}
	return set.list()
}

// genericAllEffects is every effect a call of the tool could have, whatever its
// operation: genericEffects never returns more for any op.
func genericAllEffects(tool, category string) []Effect {
	set := effectSet{EffectRunsCode: true, EffectDeletes: true}
	if systemChangingTools[tool] {
		set[EffectSystemChange] = true
	}
	if genericCanSend(tool, category) {
		set[EffectSendsMessage] = true
	}
	if category == "files" || fileWritingTools[tool] || genericFileish(tool) {
		set[EffectWritesFiles] = true
	}
	if category == "smart_home" || deviceTools[tool] {
		set[EffectControlsDevices] = true
	}
	return set.list()
}

// genericWorstEffects is what a call with an unknown operation can do: the union over
// choices, the operations the tool's schema lists (Execute refuses any other), or
// genericAllEffects when there is no list.
func genericWorstEffects(tool, category string, choices []string) []Effect {
	if len(choices) == 0 {
		return genericAllEffects(tool, category)
	}
	set := effectSet{}
	for _, c := range choices {
		for _, e := range genericEffects(tool, category, c) {
			set[e] = true
		}
	}
	return set.list()
}
