package tools

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"text/template"

	"aurago/internal/tools/skilltemplates"
)

type SkillTemplate struct {
	Name         string                 `json:"name"`
	Description  string                 `json:"description"`
	Parameters   map[string]interface{} `json:"parameters"`
	Dependencies []string               `json:"dependencies"`
	IsDaemon     bool                   `json:"is_daemon,omitempty"`
	Code         string                 `json:"-"`
}

func paramMap(m map[string]string) map[string]interface{} {
	out := make(map[string]interface{}, len(m))
	for k, v := range m {
		out[k] = v
	}
	return out
}

func AvailableSkillTemplates() []SkillTemplate {
	return []SkillTemplate{
		{
			Name:        "api_client",
			Description: "REST API client with Bearer/Basic/API-Key auth, retry logic, pagination support, and vault key injection.",
			Parameters: paramMap(map[string]string{
				"endpoint":  "API endpoint path (appended to base URL)",
				"method":    "HTTP method: GET, POST, PUT, DELETE, PATCH (default: GET)",
				"body":      "JSON request body (optional, for POST/PUT/PATCH)",
				"headers":   "Additional headers as JSON object (optional)",
				"auth_type": "Auth type: bearer, basic, api_key, none (default: bearer)",
				"max_pages": "Follow pagination links up to N pages (optional, default: 1)",
			}),
			Dependencies: []string{"requests"},
			Code: composePythonSkillTemplate(tplAPI, `{{.FunctionName}}(
        endpoint=args.get("endpoint", ""),
        method=args.get("method", "GET"),
        body=args.get("body"),
        headers=args.get("headers"),
        auth_type=args.get("auth_type", "bearer"),
        max_pages=args.get("max_pages", 1),
    )`),
		},
		{
			Name:        "data_transformer",
			Description: "Convert data between JSON, CSV, YAML, and XML formats with field filtering, sorting, and aggregation.",
			Parameters: paramMap(map[string]string{
				"input_path":    "Path to the input file",
				"output_path":   "Path to the output file (optional, prints to stdout if omitted)",
				"input_format":  "Input format: json, csv, yaml, xml",
				"output_format": "Output format: json, csv, yaml, xml",
				"fields":        "Comma-separated list of fields to include (optional)",
				"sort_by":       "Field name to sort results by (optional)",
				"limit":         "Maximum number of records to output (optional)",
			}),
			Dependencies: []string{"pyyaml"},
			Code: composePythonSkillTemplate(tplData, `{{.FunctionName}}(
        input_path=args.get("input_path", ""),
        output_path=args.get("output_path"),
        input_format=args.get("input_format", "json"),
        output_format=args.get("output_format", "json"),
        fields=args.get("fields"),
        sort_by=args.get("sort_by"),
        limit=args.get("limit"),
    )`),
		},
		{
			Name:        "notification_sender",
			Description: "Send notifications via Telegram, Discord, email (SMTP), or generic webhook. Supports message formatting and attachments.",
			Parameters: paramMap(map[string]string{
				"channel":  "Channel: telegram, discord, email, webhook",
				"message":  "Notification message text",
				"title":    "Message title or subject (optional)",
				"attach":   "File path to attach (optional)",
				"priority": "Priority level: low, normal, high (default: normal)",
			}),
			Dependencies: []string{"requests"},
			Code: composePythonSkillTemplate(tplNotify, `{{.FunctionName}}(
        channel=args.get("channel", "webhook"),
        message=args.get("message", ""),
        title=args.get("title"),
        attach=args.get("attach"),
        priority=args.get("priority", "normal"),
    )`),
		},
		{
			Name:        "monitor_check",
			Description: "Health check for HTTP endpoints, TCP ports, and DNS resolution. Returns latency, status, and pass/fail result.",
			Parameters: paramMap(map[string]string{
				"target":     "URL, host:port, or hostname to check",
				"check_type": "Check type: http, tcp, dns (default: http)",
				"timeout":    "Timeout in seconds (default: 10)",
				"expected":   "Expected status code (HTTP) or resolved IP (DNS), optional",
				"keyword":    "Keyword to search for in HTTP response body (optional)",
			}),
			Dependencies: []string{"requests"},
			Code: composePythonSkillTemplate(tplMonitor, `{{.FunctionName}}(
        target=args.get("target", ""),
        check_type=args.get("check_type", "http"),
        timeout=args.get("timeout", 10),
        expected=args.get("expected"),
        keyword=args.get("keyword"),
    )`),
		},
		{
			Name:        "log_analyzer",
			Description: "Parse and analyze log files: filter by time range, severity, pattern; extract errors and summarize statistics.",
			Parameters: paramMap(map[string]string{
				"log_path":    "Path to the log file",
				"operation":   "Operation: summary, errors, search, tail, count_by_level",
				"pattern":     "Regex pattern to search for (optional)",
				"since":       `Time filter: "5m", "1h", "24h", "7d" (optional)`,
				"max_results": "Maximum number of results to return (default: 100)",
			}),
			Dependencies: nil,
			Code: composePythonSkillTemplate(tplLog, `{{.FunctionName}}(
        log_path=args.get("log_path", ""),
        operation=args.get("operation", "summary"),
        pattern=args.get("pattern"),
        since=args.get("since"),
        max_results=args.get("max_results", 100),
    )`),
		},
		{
			Name:        "docker_manager",
			Description: "Manage Docker containers via the Docker Engine API: list, inspect, start, stop, restart, get logs and stats.",
			Parameters: paramMap(map[string]string{
				"action":    "Action: list, inspect, start, stop, restart, logs, stats",
				"container": "Container name or ID (required for all actions except list)",
				"tail":      "Number of log lines to return for logs action (default: 100)",
				"all":       "Include stopped containers for list action (default: false)",
			}),
			Dependencies: []string{"requests"},
			Code: composePythonSkillTemplate(tplDocker, `{{.FunctionName}}(
        action=args.get("action", "list"),
        container=args.get("container"),
        tail=args.get("tail", 100),
        all=args.get("all", False),
    )`),
		},
		{
			Name:        "backup_runner",
			Description: "Backup files and directories as compressed archives with rotation, integrity check, and size reporting.",
			Parameters: paramMap(map[string]string{
				"action":  "Action: create, list, restore, cleanup",
				"source":  "Source file or directory path to backup",
				"output":  "Output archive path (default: auto-generated in backup directory)",
				"keep":    "Number of backups to keep during cleanup (default: 5)",
				"exclude": "Comma-separated glob patterns to exclude (optional)",
			}),
			Dependencies: nil,
			Code: composePythonSkillTemplate(tplBackup, `{{.FunctionName}}(
        action=args.get("action", "create"),
        source=args.get("source", ""),
        output=args.get("output"),
        keep=args.get("keep", 5),
        exclude=args.get("exclude"),
    )`),
		},
		{
			Name:        "database_query",
			Description: "Execute SQL queries against SQLite, PostgreSQL, or MySQL databases. Supports SELECT, INSERT, UPDATE, DELETE with parameterized queries.",
			Parameters: paramMap(map[string]string{
				"query":      "SQL query to execute",
				"db_type":    "Database type: sqlite, postgresql, mysql (default: sqlite)",
				"connection": "Database file path (SQLite) or connection string",
				"params":     "Query parameters as JSON array (optional, for parameterized queries)",
				"limit":      "Maximum rows to return for SELECT queries (default: 100)",
			}),
			Dependencies: nil,
			Code: composePythonSkillTemplate(tplDB, `{{.FunctionName}}(
        query=args.get("query", ""),
        db_type=args.get("db_type", "sqlite"),
        connection=args.get("connection", ""),
        params=args.get("params"),
        limit=args.get("limit", 100),
    )`),
		},
		{
			Name:        "ssh_executor",
			Description: "Execute commands on remote hosts via SSH with key and password authentication. Returns structured output with exit codes.",
			Parameters: paramMap(map[string]string{
				"host":    "Target hostname or IP address",
				"command": "Command to execute on the remote host",
				"user":    "SSH username (default: current user)",
				"port":    "SSH port (default: 22)",
				"timeout": "Command timeout in seconds (default: 30)",
			}),
			Dependencies: []string{"paramiko"},
			Code: composePythonSkillTemplate(tplSSH, `{{.FunctionName}}(
        host=args.get("host", ""),
        command=args.get("command", ""),
        user=args.get("user"),
        port=args.get("port", 22),
        timeout=args.get("timeout", 30),
    )`),
		},
		{
			Name:        "mqtt_publisher",
			Description: "Publish and subscribe to MQTT topics for IoT device control and sensor data. Supports QoS levels and retained messages.",
			Parameters: paramMap(map[string]string{
				"action":  "Action: publish, subscribe",
				"topic":   "MQTT topic path",
				"payload": "Message payload to publish (optional for subscribe)",
				"qos":     "QoS level: 0, 1, 2 (default: 0)",
				"retain":  "Retain message on broker (default: false)",
				"timeout": "Subscribe timeout in seconds (default: 5)",
			}),
			Dependencies: []string{"paho-mqtt"},
			Code: composePythonSkillTemplate(tplMQTT, `{{.FunctionName}}(
        action=args.get("action", "publish"),
        topic=args.get("topic", ""),
        payload=args.get("payload"),
        qos=args.get("qos", 0),
        retain=args.get("retain", False),
        timeout=args.get("timeout", 5),
    )`),
		},
		// ── Daemon skill templates ───────────────────────────────────────
		{
			Name:        "daemon_monitor",
			Description: "Long-running daemon that periodically checks a resource (disk, CPU, service, URL) and wakes the agent on threshold violations.",
			IsDaemon:    true,
			Parameters: paramMap(map[string]string{
				"target":         "What to monitor (e.g. 'disk', 'cpu', 'url', 'service')",
				"threshold":      "Alert threshold value (e.g. '90' for 90%)",
				"interval":       "Check interval in seconds (default: 60)",
				"alert_severity": "Severity when threshold exceeded: info, warning, critical (default: warning)",
			}),
			Dependencies: []string{},
			Code:         composeDaemonSkillTemplate(tplDMon),
		},
		{
			Name:        "daemon_watcher",
			Description: "Long-running daemon that watches a directory for file changes (created, modified, deleted) and wakes the agent on events.",
			IsDaemon:    true,
			Parameters: paramMap(map[string]string{
				"watch_path": "Directory path to watch for changes",
				"patterns":   "Comma-separated file patterns to match (e.g. '*.log,*.csv'); empty = all files",
				"events":     "Comma-separated events: created, modified, deleted (default: all)",
				"cooldown":   "Minimum seconds between alerts for the same file (default: 10)",
				"recursive":  "Watch subdirectories recursively: true/false (default: true)",
			}),
			Dependencies: []string{},
			Code:         composeDaemonSkillTemplate(tplDWatch),
		},
		{
			Name:        "daemon_listener",
			Description: "Long-running daemon that listens on a Unix domain socket or named pipe for external events and forwards them to the agent.",
			IsDaemon:    true,
			Parameters: paramMap(map[string]string{
				"socket_path": "Path for the Unix domain socket or named pipe",
				"protocol":    "Protocol: line (newline-delimited text) or json (JSON per line) (default: json)",
				"max_clients": "Maximum concurrent connections (default: 5)",
			}),
			Dependencies: []string{},
			Code:         composeDaemonSkillTemplate(tplDListen),
		},
		{
			Name:        "daemon_mission",
			Description: "Long-running daemon that monitors a backup directory or status file and emits events that can trigger a follow-up mission (configure trigger_mission_id in the daemon settings).",
			IsDaemon:    true,
			Parameters: paramMap(map[string]string{
				"watch_dir":      "Directory to watch for backup files (e.g. /var/backups)",
				"status_file":    "Path to a JSON status file with fields 'status' and 'message'; leave empty to use watch_dir",
				"backup_pattern": "Glob pattern for backup files when watching a directory (default: *.backup)",
				"check_interval": "Seconds between checks (default: 60)",
				"cooldown":       "Minimum seconds between alerts for the same event (default: 300)",
			}),
			Dependencies: []string{},
			Code:         composeDaemonSkillTemplate(tplDMission),
		},
	}
}

type templateData struct {
	FunctionName string
	Description  string
	BaseURL      string
}

func mustLoadTemplate(name string) string {
	data, err := skilltemplates.TemplatesFS.ReadFile(name + ".py")
	if err != nil {
		panic(fmt.Sprintf("skilltemplates: failed to read %s.py: %v", name, err))
	}
	return string(data)
}

var (
	tplPrefix   = mustLoadTemplate("_wrapper_prefix")
	tplSuffix   = mustLoadTemplate("_wrapper_suffix")
	tplAPI      = mustLoadTemplate("api_client")
	tplData     = mustLoadTemplate("data_transformer")
	tplNotify   = mustLoadTemplate("notification_sender")
	tplMonitor  = mustLoadTemplate("monitor_check")
	tplLog      = mustLoadTemplate("log_analyzer")
	tplDocker   = mustLoadTemplate("docker_manager")
	tplBackup   = mustLoadTemplate("backup_runner")
	tplDB       = mustLoadTemplate("database_query")
	tplSSH      = mustLoadTemplate("ssh_executor")
	tplMQTT     = mustLoadTemplate("mqtt_publisher")
	tplDMon     = mustLoadTemplate("daemon_monitor")
	tplDWatch   = mustLoadTemplate("daemon_watcher")
	tplDListen  = mustLoadTemplate("daemon_listener")
	tplDMission = mustLoadTemplate("daemon_mission")
)

func composePythonSkillTemplate(body, invocation string) string {
	return body + tplPrefix + invocation + tplSuffix
}

func composeDaemonSkillTemplate(body string) string {
	return body
}

var validFuncNameRe = regexp.MustCompile(`[^a-zA-Z0-9_]`)

func escapePythonString(s string) string {
	var sb strings.Builder
	for _, r := range s {
		switch r {
		case '\\':
			sb.WriteString(`\\`)
		case '\'':
			sb.WriteString(`\'`)
		case '\n':
			sb.WriteString(`\n`)
		case '\r':
			sb.WriteString(`\r`)
		case '\t':
			sb.WriteString(`\t`)
		default:
			sb.WriteRune(r)
		}
	}
	return sb.String()
}

func toFunctionName(name string) string {
	fn := validFuncNameRe.ReplaceAllString(name, "_")
	fn = strings.Trim(fn, "_")
	if fn == "" {
		fn = "skill_main"
	}
	if fn[0] >= '0' && fn[0] <= '9' {
		fn = "skill_" + fn
	}
	return fn
}

func CreateSkillFromTemplate(skillsDir, templateName, skillName, description, baseURL string, dependencies, vaultKeys []string) (string, error) {
	var err error
	skillName, err = validateSkillName(skillName)
	if err != nil {
		return "", err
	}
	description, baseURL, dependencies, vaultKeys, err = normalizeSkillTemplateInputs(description, baseURL, dependencies, vaultKeys)
	if err != nil {
		return "", err
	}
	if templateName == "" {
		return "", fmt.Errorf("template name is required")
	}

	var tmpl *SkillTemplate
	for _, t := range AvailableSkillTemplates() {
		if t.Name == templateName {
			tmpl = &t
			break
		}
	}
	if tmpl == nil {
		names := make([]string, 0)
		for _, t := range AvailableSkillTemplates() {
			names = append(names, t.Name)
		}
		return "", fmt.Errorf("unknown template '%s'; available: %s", templateName, strings.Join(names, ", "))
	}

	jsonPath := filepath.Join(skillsDir, skillName+".json")
	pyPath := filepath.Join(skillsDir, skillName+".py")

	data := templateData{
		FunctionName: toFunctionName(skillName),
		Description:  description,
		BaseURL:      escapePythonString(baseURL),
	}
	if data.Description == "" {
		data.Description = tmpl.Description
	}

	goTmpl, err := template.New(templateName).Parse(tmpl.Code)
	if err != nil {
		return "", fmt.Errorf("failed to parse template '%s': %w", templateName, err)
	}

	var codeBuf bytes.Buffer
	if err := goTmpl.Execute(&codeBuf, data); err != nil {
		return "", fmt.Errorf("failed to render template '%s': %w", templateName, err)
	}

	depSet := make(map[string]bool)
	var allDeps []string
	for _, d := range tmpl.Dependencies {
		d = strings.TrimSpace(d)
		if d != "" && !depSet[d] {
			depSet[d] = true
			allDeps = append(allDeps, d)
		}
	}
	for _, d := range dependencies {
		d = strings.TrimSpace(d)
		if d != "" && !depSet[d] {
			depSet[d] = true
			allDeps = append(allDeps, d)
		}
	}
	allDeps, err = normalizeSkillDependencies(allDeps)
	if err != nil {
		return "", err
	}

	manifest := SkillManifest{
		Name:         skillName,
		Description:  description,
		Executable:   skillName + ".py",
		Parameters:   tmpl.Parameters,
		Returns:      "JSON object with 'status' and 'result' or 'message' fields.",
		Dependencies: allDeps,
		VaultKeys:    vaultKeys,
	}
	if manifest.Description == "" {
		manifest.Description = tmpl.Description
	}
	if tmpl.IsDaemon {
		dm := DaemonManifestDefaults()
		dm.Enabled = true
		dm.WakeAgent = true
		manifest.Daemon = &dm
	}

	if err := os.MkdirAll(skillsDir, 0o750); err != nil {
		return "", fmt.Errorf("failed to create skills directory: %w", err)
	}

	manifestJSON, err := json.MarshalIndent(manifest, "", "    ")
	if err != nil {
		return "", fmt.Errorf("failed to serialize manifest: %w", err)
	}
	if err := writeFileExclusive(jsonPath, manifestJSON, 0o640); err != nil {
		if os.IsExist(err) {
			return "", fmt.Errorf("skill '%s' already exists", skillName)
		}
		return "", fmt.Errorf("failed to write manifest: %w", err)
	}

	if err := validateSkillCode(codeBuf.String()); err != nil {
		_ = os.Remove(jsonPath)
		return "", err
	}
	if err := writeFileExclusive(pyPath, codeBuf.Bytes(), 0o640); err != nil {
		os.Remove(jsonPath)
		if os.IsExist(err) {
			return "", fmt.Errorf("skill '%s' already exists", skillName)
		}
		return "", fmt.Errorf("failed to write Python script: %w", err)
	}

	InvalidateSkillsCache(skillsDir)

	// Sync to database so the skill is visible in the Web UI immediately.
	if mgr := DefaultSkillManager(); mgr != nil {
		_ = mgr.SyncFromDisk()
	}

	result := fmt.Sprintf("Skill '%s' created from template '%s'.\nFiles: %s, %s\nDependencies: %s\nUse execute_skill with skill='%s' to run it.",
		skillName, templateName, filepath.Base(jsonPath), filepath.Base(pyPath),
		strings.Join(allDeps, ", "), skillName)

	if len(vaultKeys) > 0 {
		result += fmt.Sprintf("\n\n⚠️ IMPORTANT: This skill requires vault secrets: %s\n"+
			"The user must store these secrets in the vault via the Web UI (Settings → Secrets) "+
			"and then assign them to this skill in the Skill Manager (Skills → %s → Assign Secrets). "+
			"Without this step, the skill will not have access to the required credentials.",
			strings.Join(vaultKeys, ", "), skillName)
	}

	return result, nil
}