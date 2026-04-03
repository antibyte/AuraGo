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
)

type SkillTemplate struct {
	Name         string            `json:"name"`
	Description  string            `json:"description"`
	Parameters   map[string]string `json:"parameters"`
	Dependencies []string          `json:"dependencies"`
	Code         string            `json:"-"`
}

func AvailableSkillTemplates() []SkillTemplate {
	return []SkillTemplate{
		{
			Name:        "api_client",
			Description: "REST API client with Bearer/Basic/API-Key auth, retry logic, pagination support, and vault key injection.",
			Parameters: map[string]string{
				"endpoint":  "API endpoint path (appended to base URL)",
				"method":    "HTTP method: GET, POST, PUT, DELETE, PATCH (default: GET)",
				"body":      "JSON request body (optional, for POST/PUT/PATCH)",
				"headers":   "Additional headers as JSON object (optional)",
				"auth_type": "Auth type: bearer, basic, api_key, none (default: bearer)",
				"max_pages": "Follow pagination links up to N pages (optional, default: 1)",
			},
			Dependencies: []string{"requests"},
			Code: composePythonSkillTemplate(apiClientTemplateBody, `{{.FunctionName}}(
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
			Parameters: map[string]string{
				"input_path":    "Path to the input file",
				"output_path":   "Path to the output file (optional, prints to stdout if omitted)",
				"input_format":  "Input format: json, csv, yaml, xml",
				"output_format": "Output format: json, csv, yaml, xml",
				"fields":        "Comma-separated list of fields to include (optional)",
				"sort_by":       "Field name to sort results by (optional)",
				"limit":         "Maximum number of records to output (optional)",
			},
			Dependencies: []string{"pyyaml"},
			Code: composePythonSkillTemplate(dataTransformerTemplateBody, `{{.FunctionName}}(
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
			Parameters: map[string]string{
				"channel":  "Channel: telegram, discord, email, webhook",
				"message":  "Notification message text",
				"title":    "Message title or subject (optional)",
				"attach":   "File path to attach (optional)",
				"priority": "Priority level: low, normal, high (default: normal)",
			},
			Dependencies: []string{"requests"},
			Code: composePythonSkillTemplate(notificationSenderTemplateBody, `{{.FunctionName}}(
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
			Parameters: map[string]string{
				"target":     "URL, host:port, or hostname to check",
				"check_type": "Check type: http, tcp, dns (default: http)",
				"timeout":    "Timeout in seconds (default: 10)",
				"expected":   "Expected status code (HTTP) or resolved IP (DNS), optional",
				"keyword":    "Keyword to search for in HTTP response body (optional)",
			},
			Dependencies: []string{"requests"},
			Code: composePythonSkillTemplate(monitorCheckTemplateBody, `{{.FunctionName}}(
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
			Parameters: map[string]string{
				"log_path":    "Path to the log file",
				"operation":   "Operation: summary, errors, search, tail, count_by_level",
				"pattern":     "Regex pattern to search for (optional)",
				"since":       `Time filter: "5m", "1h", "24h", "7d" (optional)`,
				"max_results": "Maximum number of results to return (default: 100)",
			},
			Dependencies: nil,
			Code: composePythonSkillTemplate(logAnalyzerTemplateBody, `{{.FunctionName}}(
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
			Parameters: map[string]string{
				"action":    "Action: list, inspect, start, stop, restart, logs, stats",
				"container": "Container name or ID (required for all actions except list)",
				"tail":      "Number of log lines to return for logs action (default: 100)",
				"all":       "Include stopped containers for list action (default: false)",
			},
			Dependencies: []string{"requests"},
			Code: composePythonSkillTemplate(dockerManagerTemplateBody, `{{.FunctionName}}(
        action=args.get("action", "list"),
        container=args.get("container"),
        tail=args.get("tail", 100),
        all=args.get("all", False),
    )`),
		},
		{
			Name:        "backup_runner",
			Description: "Backup files and directories as compressed archives with rotation, integrity check, and size reporting.",
			Parameters: map[string]string{
				"action":  "Action: create, list, restore, cleanup",
				"source":  "Source file or directory path to backup",
				"output":  "Output archive path (default: auto-generated in backup directory)",
				"keep":    "Number of backups to keep during cleanup (default: 5)",
				"exclude": "Comma-separated glob patterns to exclude (optional)",
			},
			Dependencies: nil,
			Code: composePythonSkillTemplate(backupRunnerTemplateBody, `{{.FunctionName}}(
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
			Parameters: map[string]string{
				"query":      "SQL query to execute",
				"db_type":    "Database type: sqlite, postgresql, mysql (default: sqlite)",
				"connection": "Database file path (SQLite) or connection string",
				"params":     "Query parameters as JSON array (optional, for parameterized queries)",
				"limit":      "Maximum rows to return for SELECT queries (default: 100)",
			},
			Dependencies: nil,
			Code: composePythonSkillTemplate(databaseQueryTemplateBody, `{{.FunctionName}}(
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
			Parameters: map[string]string{
				"host":    "Target hostname or IP address",
				"command": "Command to execute on the remote host",
				"user":    "SSH username (default: current user)",
				"port":    "SSH port (default: 22)",
				"timeout": "Command timeout in seconds (default: 30)",
			},
			Dependencies: []string{"paramiko"},
			Code: composePythonSkillTemplate(sshExecutorTemplateBody, `{{.FunctionName}}(
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
			Parameters: map[string]string{
				"action":  "Action: publish, subscribe",
				"topic":   "MQTT topic path",
				"payload": "Message payload to publish (optional for subscribe)",
				"qos":     "QoS level: 0, 1, 2 (default: 0)",
				"retain":  "Retain message on broker (default: false)",
				"timeout": "Subscribe timeout in seconds (default: 5)",
			},
			Dependencies: []string{"paho-mqtt"},
			Code: composePythonSkillTemplate(mqttPublisherTemplateBody, `{{.FunctionName}}(
        action=args.get("action", "publish"),
        topic=args.get("topic", ""),
        payload=args.get("payload"),
        qos=args.get("qos", 0),
        retain=args.get("retain", False),
        timeout=args.get("timeout", 5),
    )`),
		},
	}
}

type templateData struct {
	FunctionName string
	Description  string
	BaseURL      string
}

func composePythonSkillTemplate(body, invocation string) string {
	return body + pythonSkillMainTemplatePrefix + invocation + pythonSkillMainTemplateSuffix
}

const pythonSkillMainTemplatePrefix = `

if __name__ == "__main__":
    args = {}
    try:
        stdin_data = sys.stdin.read().strip()
        if stdin_data:
            args = json.loads(stdin_data)
    except Exception:
        pass
    if not args and len(sys.argv) > 1:
        try:
            args = json.loads(sys.argv[1])
        except Exception:
            pass
    if not args:
        print(json.dumps({"status": "error", "message": "No input provided."}))
        sys.exit(1)
    if hasattr(sys.stdout, "reconfigure"):
        sys.stdout.reconfigure(encoding="utf-8")
    result = `

const pythonSkillMainTemplateSuffix = `
    print(json.dumps(result, ensure_ascii=False))
`

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

	return fmt.Sprintf("Skill '%s' created from template '%s'.\nFiles: %s, %s\nDependencies: %s\nUse execute_skill with skill='%s' to run it.",
		skillName, templateName, filepath.Base(jsonPath), filepath.Base(pyPath),
		strings.Join(allDeps, ", "), skillName), nil
}

const apiClientTemplateBody = `import sys
import json
import os
import time
import requests

def {{.FunctionName}}(endpoint, method="GET", body=None, headers=None, auth_type="bearer", max_pages=1):
    """{{.Description}}"""
    base_url = os.environ.get("AURAGO_SECRET_BASE_URL", "{{.BaseURL}}").rstrip("/")
    api_key = os.environ.get("AURAGO_SECRET_API_KEY", "")
    username = os.environ.get("AURAGO_SECRET_USERNAME", "")
    password = os.environ.get("AURAGO_SECRET_PASSWORD", "")

    req_headers = {"Content-Type": "application/json"}
    if headers and isinstance(headers, dict):
        req_headers.update(headers)

    if auth_type == "bearer" and api_key:
        req_headers["Authorization"] = f"Bearer {api_key}"
    elif auth_type == "basic" and username and password:
        pass
    elif auth_type == "api_key" and api_key:
        req_headers["X-API-Key"] = api_key

    url = f"{base_url}/{endpoint.lstrip('/')}" if endpoint else base_url
    all_items = []
    page = 0
    retries = 3

    try:
        while page < int(max_pages):
            kwargs = {
                "method": method.upper(),
                "url": url,
                "headers": req_headers,
                "timeout": 30,
            }
            if body and method.upper() in ("POST", "PUT", "PATCH"):
                kwargs["json"] = body
            if auth_type == "basic" and username and password:
                kwargs["auth"] = (username, password)

            for attempt in range(retries):
                try:
                    resp = requests.request(**kwargs)
                    resp.raise_for_status()
                    break
                except requests.exceptions.ConnectionError as e:
                    if attempt == retries - 1:
                        raise
                    time.sleep(2 ** attempt)

            try:
                data = resp.json()
            except ValueError:
                data = resp.text

            if isinstance(data, list):
                all_items.extend(data)
            elif isinstance(data, dict):
                items = data.get("data", data.get("items", data.get("results", None)))
                if isinstance(items, list):
                    all_items.extend(items)
                else:
                    all_items.append(data)

            next_url = None
            if isinstance(data, dict):
                next_url = data.get("next") or data.get("next_page_token")
                if isinstance(next_url, str) and next_url.startswith("http"):
                    url = next_url
                elif next_url:
                    sep = "&" if "?" in url else "?"
                    url = f"{url}{sep}page_token={next_url}"
                else:
                    break
            else:
                break

            page += 1

        result_data = all_items if all_items else data
        return {"status": "success", "result": f"<external_data>{json.dumps(result_data, ensure_ascii=False)}</external_data>"}
    except requests.RequestException as e:
        return {"status": "error", "message": str(e)}
`

const dataTransformerTemplateBody = `import sys
import json
import os
import csv
import io

try:
    import yaml
except ImportError:
    yaml = None

try:
    import xml.etree.ElementTree as ET
except ImportError:
    ET = None

def _parse_xml(raw):
    root = ET.fromstring(raw)
    def elem_to_dict(el):
        d = {}
        if el.attrib:
            d.update({"@" + k: v for k, v in el.attrib.items()})
        children = list(el)
        if not children:
            return el.text or ""
        for child in children:
            tag = child.tag
            val = elem_to_dict(child)
            if tag in d:
                if not isinstance(d[tag], list):
                    d[tag] = [d[tag]]
                d[tag].append(val)
            else:
                d[tag] = val
        return d
    return elem_to_dict(root)

def _to_xml(data, tag="root"):
    root = ET.Element(tag)
    def build(parent, val):
        if isinstance(val, dict):
            for k, v in val.items():
                child = ET.SubElement(parent, k)
                build(child, v)
        elif isinstance(val, list):
            for item in val:
                child = ET.SubElement(parent, "item")
                build(child, item)
        else:
            parent.text = str(val) if val is not None else ""
    build(root, data)
    return ET.tostring(root, encoding="unicode")

def {{.FunctionName}}(input_path, output_path=None, input_format="json", output_format="json", fields=None, sort_by=None, limit=None):
    """{{.Description}}"""
    if not os.path.isabs(input_path):
        input_path = os.path.abspath(input_path)
    if not os.path.exists(input_path):
        return {"status": "error", "message": f"File not found: {input_path}"}

    try:
        with open(input_path, "r", encoding="utf-8") as f:
            raw = f.read()

        if input_format == "json":
            data = json.loads(raw)
        elif input_format == "csv":
            reader = csv.DictReader(io.StringIO(raw))
            data = list(reader)
        elif input_format == "yaml":
            if yaml is None:
                return {"status": "error", "message": "pyyaml not installed"}
            data = yaml.safe_load(raw)
        elif input_format == "xml":
            if ET is None:
                return {"status": "error", "message": "xml module not available"}
            data = _parse_xml(raw)
        else:
            return {"status": "error", "message": f"Unsupported input format: {input_format}"}

        if fields and isinstance(data, list):
            field_list = [f.strip() for f in fields.split(",")]
            data = [{k: row.get(k) for k in field_list} for row in data]

        if sort_by and isinstance(data, list):
            reverse = sort_by.startswith("-")
            key = sort_by.lstrip("-")
            data.sort(key=lambda r: r.get(key, ""), reverse=reverse)

        if limit and isinstance(data, list):
            data = data[:int(limit)]

        if output_format == "json":
            output = json.dumps(data, indent=2, ensure_ascii=False)
        elif output_format == "csv":
            if not isinstance(data, list) or not data:
                return {"status": "error", "message": "CSV output requires a list of objects"}
            buf = io.StringIO()
            writer = csv.DictWriter(buf, fieldnames=data[0].keys())
            writer.writeheader()
            writer.writerows(data)
            output = buf.getvalue()
        elif output_format == "yaml":
            if yaml is None:
                return {"status": "error", "message": "pyyaml not installed"}
            output = yaml.dump(data, allow_unicode=True, default_flow_style=False)
        elif output_format == "xml":
            output = _to_xml(data)
        else:
            return {"status": "error", "message": f"Unsupported output format: {output_format}"}

        if output_path:
            if not os.path.isabs(output_path):
                output_path = os.path.abspath(output_path)
            with open(output_path, "w", encoding="utf-8") as f:
                f.write(output)
            return {"status": "success", "result": f"Converted {input_format} -> {output_format}, written to {output_path}"}
        return {"status": "success", "result": output}
    except Exception as e:
        return {"status": "error", "message": str(e)}
`

const notificationSenderTemplateBody = `import sys
import json
import os
import smtplib
from email.mime.text import MIMEText
from email.mime.multipart import MIMEMultipart
from email.mime.base import MIMEBase
from email import encoders
import requests

def _send_telegram(token, chat_id, message, title=None, priority="normal"):
    url = f"https://api.telegram.org/bot{token}/sendMessage"
    text = f"*{title}*\n\n{message}" if title else message
    if priority == "high":
        text = "\u26a0\ufe0f " + text
    payload = {"chat_id": chat_id, "text": text, "parse_mode": "Markdown"}
    resp = requests.post(url, json=payload, timeout=10)
    resp.raise_for_status()
    return resp.json()

def _send_discord(webhook_url, message, title=None, priority="normal"):
    payload = {"content": ""}
    embed = {"description": message}
    if title:
        embed["title"] = title
    color_map = {"low": 3447003, "normal": 3066993, "high": 15158332}
    embed["color"] = color_map.get(priority, 3066993)
    payload["embeds"] = [embed]
    resp = requests.post(webhook_url, json=payload, timeout=10)
    resp.raise_for_status()
    return {"sent": True}

def _send_email(smtp_host, smtp_port, smtp_user, smtp_pass, from_addr, to_addr, message, title=None, attach=None, priority="normal"):
    msg = MIMEMultipart()
    msg["From"] = from_addr
    msg["To"] = to_addr
    msg["Subject"] = title or "AuraGo Notification"
    if priority == "high":
        msg["X-Priority"] = "1"
    msg.attach(MIMEText(message, "plain"))

    if attach and os.path.isfile(attach):
        with open(attach, "rb") as f:
            part = MIMEBase("application", "octet-stream")
            part.set_payload(f.read())
        encoders.encode_base64(part)
        part.add_header("Content-Disposition", f"attachment; filename={os.path.basename(attach)}")
        msg.attach(part)

    if smtp_port == 465:
        server = smtplib.SMTP_SSL(smtp_host, smtp_port, timeout=15)
    else:
        server = smtplib.SMTP(smtp_host, smtp_port, timeout=15)
        server.starttls()
    if smtp_user and smtp_pass:
        server.login(smtp_user, smtp_pass)
    server.sendmail(from_addr, to_addr, msg.as_string())
    server.quit()
    return {"sent": True}

def _send_webhook(url, message, title=None, priority="normal"):
    payload = {
        "message": message,
        "title": title,
        "priority": priority,
        "source": "AuraGo",
    }
    api_key = os.environ.get("AURAGO_SECRET_WEBHOOK_KEY", "")
    headers = {"Content-Type": "application/json"}
    if api_key:
        headers["Authorization"] = f"Bearer {api_key}"
    resp = requests.post(url, json=payload, headers=headers, timeout=10)
    resp.raise_for_status()
    try:
        return resp.json()
    except ValueError:
        return {"status": "sent", "http_code": resp.status_code}

def {{.FunctionName}}(channel, message, title=None, attach=None, priority="normal"):
    """{{.Description}}"""
    if not message:
        return {"status": "error", "message": "Message text is required"}

    try:
        if channel == "telegram":
            token = os.environ.get("AURAGO_SECRET_TELEGRAM_BOT_TOKEN", "")
            chat_id = os.environ.get("AURAGO_SECRET_TELEGRAM_CHAT_ID", "")
            if not token or not chat_id:
                return {"status": "error", "message": "Telegram requires vault keys: TELEGRAM_BOT_TOKEN, TELEGRAM_CHAT_ID"}
            result = _send_telegram(token, chat_id, message, title, priority)

        elif channel == "discord":
            webhook_url = os.environ.get("AURAGO_SECRET_DISCORD_WEBHOOK_URL", "")
            if not webhook_url:
                return {"status": "error", "message": "Discord requires vault key: DISCORD_WEBHOOK_URL"}
            result = _send_discord(webhook_url, message, title, priority)

        elif channel == "email":
            smtp_host = os.environ.get("AURAGO_SECRET_SMTP_HOST", "")
            smtp_port = int(os.environ.get("AURAGO_SECRET_SMTP_PORT", "587"))
            smtp_user = os.environ.get("AURAGO_SECRET_SMTP_USER", "")
            smtp_pass = os.environ.get("AURAGO_SECRET_SMTP_PASSWORD", "")
            from_addr = os.environ.get("AURAGO_SECRET_EMAIL_FROM", smtp_user)
            to_addr = os.environ.get("AURAGO_SECRET_EMAIL_TO", "")
            if not smtp_host or not to_addr:
                return {"status": "error", "message": "Email requires vault keys: SMTP_HOST, EMAIL_TO (and SMTP_USER/SMTP_PASSWORD for auth)"}
            result = _send_email(smtp_host, smtp_port, smtp_user, smtp_pass, from_addr, to_addr, message, title, attach, priority)

        elif channel == "webhook":
            url = os.environ.get("AURAGO_SECRET_WEBHOOK_URL", "{{.BaseURL}}")
            if not url:
                return {"status": "error", "message": "Webhook requires vault key: WEBHOOK_URL or 'url' parameter"}
            result = _send_webhook(url, message, title, priority)

        else:
            return {"status": "error", "message": f"Unknown channel: {channel}. Use: telegram, discord, email, webhook"}

        return {"status": "success", "result": result}
    except Exception as e:
        return {"status": "error", "message": str(e)}
`

const monitorCheckTemplateBody = `import sys
import json
import os
import socket
import time
import requests

def _check_http(target, timeout, expected, keyword):
    start = time.time()
    try:
        resp = requests.get(target, timeout=int(timeout), headers={
            "User-Agent": "AuraGo-Monitor/1.0",
        }, allow_redirects=True)
        latency_ms = round((time.time() - start) * 1000, 1)
        result = {
            "target": target,
            "type": "http",
            "status_code": resp.status_code,
            "latency_ms": latency_ms,
            "passed": True,
        }
        if expected:
            result["expected"] = int(expected)
            result["passed"] = resp.status_code == int(expected)
        if keyword:
            found = keyword.lower() in resp.text.lower()
            result["keyword"] = keyword
            result["keyword_found"] = found
            result["passed"] = result["passed"] and found
        return result
    except requests.RequestException as e:
        latency_ms = round((time.time() - start) * 1000, 1)
        return {"target": target, "type": "http", "passed": False, "error": str(e), "latency_ms": latency_ms}

def _check_tcp(target, timeout):
    if ":" not in target:
        return {"target": target, "type": "tcp", "passed": False, "error": "Format must be host:port"}
    host, port_str = target.rsplit(":", 1)
    try:
        port = int(port_str)
    except ValueError:
        return {"target": target, "type": "tcp", "passed": False, "error": "Invalid port number"}
    start = time.time()
    try:
        sock = socket.socket(socket.AF_INET, socket.SOCK_STREAM)
        sock.settimeout(int(timeout))
        sock.connect((host, port))
        sock.close()
        latency_ms = round((time.time() - start) * 1000, 1)
        return {"target": target, "type": "tcp", "passed": True, "latency_ms": latency_ms}
    except (socket.timeout, socket.error) as e:
        latency_ms = round((time.time() - start) * 1000, 1)
        return {"target": target, "type": "tcp", "passed": False, "error": str(e), "latency_ms": latency_ms}

def _check_dns(target, expected):
    start = time.time()
    try:
        resolved = socket.gethostbyname(target)
        latency_ms = round((time.time() - start) * 1000, 1)
        result = {
            "target": target,
            "type": "dns",
            "resolved_ip": resolved,
            "latency_ms": latency_ms,
            "passed": True,
        }
        if expected:
            result["expected"] = expected
            result["passed"] = resolved == expected
        return result
    except socket.gaierror as e:
        latency_ms = round((time.time() - start) * 1000, 1)
        return {"target": target, "type": "dns", "passed": False, "error": str(e), "latency_ms": latency_ms}

def {{.FunctionName}}(target, check_type="http", timeout=10, expected=None, keyword=None):
    """{{.Description}}"""
    if not target:
        return {"status": "error", "message": "Target is required"}

    timeout = int(timeout)
    if check_type == "http":
        if not target.startswith(("http://", "https://")):
            target = f"http://{target}"
        result = _check_http(target, timeout, expected, keyword)
    elif check_type == "tcp":
        result = _check_tcp(target, timeout)
    elif check_type == "dns":
        result = _check_dns(target, expected)
    else:
        return {"status": "error", "message": f"Unknown check type: {check_type}. Use: http, tcp, dns"}

    status = "success" if result.get("passed") else "warning"
    return {"status": status, "result": result}
`

const logAnalyzerTemplateBody = `import sys
import json
import os
import re
from datetime import datetime, timedelta

LEVEL_PATTERNS = {
    "error": re.compile(r'\b(ERROR|FATAL|CRITICAL|SEVERE|ERR)\b', re.IGNORECASE),
    "warning": re.compile(r'\b(WARN|WARNING|WRN)\b', re.IGNORECASE),
    "info": re.compile(r'\b(INFO|INFORMATION)\b', re.IGNORECASE),
    "debug": re.compile(r'\b(DEBUG|DBG|TRACE)\b', re.IGNORECASE),
}

TIMESTAMP_PATTERNS = [
    re.compile(r'(\d{4}-\d{2}-\d{2}[\sT]\d{2}:\d{2}:\d{2})'),
    re.compile(r'(\d{2}/\d{2}/\d{4}\s+\d{2}:\d{2}:\d{2})'),
    re.compile(r'(\w{3}\s+\d{1,2}\s+\d{2}:\d{2}:\d{2})'),
]

def _parse_timestamp(line):
    for pat in TIMESTAMP_PATTERNS:
        m = pat.search(line)
        if m:
            for fmt in ("%Y-%m-%d %H:%M:%S", "%Y-%m-%dT%H:%M:%S", "%d/%m/%Y %H:%M:%S", "%b %d %H:%M:%S"):
                try:
                    return datetime.strptime(m.group(1), fmt)
                except ValueError:
                    continue
    return None

def _detect_level(line):
    for level, pat in LEVEL_PATTERNS.items():
        if pat.search(line):
            return level
    return "unknown"

def _parse_time_filter(since):
    if not since:
        return None
    m = re.match(r'(\d+)\s*(m|h|d|w)', since.lower().strip())
    if not m:
        return None
    val, unit = int(m.group(1)), m.group(2)
    delta_map = {"m": timedelta(minutes=val), "h": timedelta(hours=val), "d": timedelta(days=val), "w": timedelta(weeks=val)}
    return datetime.now() - delta_map.get(unit, timedelta())

def {{.FunctionName}}(log_path, operation="summary", pattern=None, since=None, max_results=100):
    """{{.Description}}"""
    if not os.path.isabs(log_path):
        log_path = os.path.abspath(log_path)
    if not os.path.exists(log_path):
        return {"status": "error", "message": f"File not found: {log_path}"}

    try:
        with open(log_path, "r", encoding="utf-8", errors="replace") as f:
            lines = f.readlines()
    except Exception as e:
        return {"status": "error", "message": str(e)}

    cutoff = _parse_time_filter(since)
    max_results = int(max_results)
    parsed = []
    for line in lines:
        ts = _parse_timestamp(line)
        level = _detect_level(line)
        if cutoff and ts and ts < cutoff:
            continue
        parsed.append({"line": line.rstrip(), "level": level, "timestamp": ts})

    if operation == "summary":
        counts = {}
        for entry in parsed:
            counts[entry["level"]] = counts.get(entry["level"], 0) + 1
        return {
            "status": "success",
            "result": {
                "total_lines": len(lines),
                "filtered_lines": len(parsed),
                "level_counts": counts,
                "time_range": {
                    "earliest": str(parsed[0]["timestamp"]) if parsed and parsed[0]["timestamp"] else None,
                    "latest": str(parsed[-1]["timestamp"]) if parsed and parsed[-1]["timestamp"] else None,
                },
            },
        }

    elif operation == "errors":
        errors = [e for e in parsed if e["level"] in ("error", "fatal")]  [:max_results]
        return {"status": "success", "result": {"count": len(errors), "errors": [e["line"] for e in errors]}}

    elif operation == "search":
        if not pattern:
            return {"status": "error", "message": "Pattern is required for search operation"}
        regex = re.compile(pattern, re.IGNORECASE)
        matches = [e for e in parsed if regex.search(e["line"])][:max_results]
        return {"status": "success", "result": {"count": len(matches), "matches": [e["line"] for e in matches]}}

    elif operation == "tail":
        tail_lines = parsed[-max_results:]
        return {"status": "success", "result": {"lines": [e["line"] for e in tail_lines]}}

    elif operation == "count_by_level":
        counts = {}
        for entry in parsed:
            counts[entry["level"]] = counts.get(entry["level"], 0) + 1
        return {"status": "success", "result": counts}

    else:
        return {"status": "error", "message": f"Unknown operation: {operation}. Use: summary, errors, search, tail, count_by_level"}
`

const dockerManagerTemplateBody = `import sys
import json
import os
import requests

DOCKER_SOCKET = os.environ.get("DOCKER_HOST", "/var/run/docker.sock")

def _docker_api(method, path, params=None, timeout=30):
    base = "http://localhost"
    if DOCKER_SOCKET.startswith("unix://") or DOCKER_SOCKET.startswith("/"):
        base = "http+docker://localhost"
    try:
        url = f"http://localhost{path}"
        resp = requests.request(
            method, url,
            params=params,
            timeout=timeout,
            headers={"Content-Type": "application/json"},
        )
    except Exception:
        import http.client
        import urllib.parse
        if DOCKER_SOCKET.startswith("/"):
            sock_path = DOCKER_SOCKET
        else:
            sock_path = DOCKER_SOCKET.replace("unix://", "")
        conn = http.client.HTTPConnection("localhost")
        try:
            conn.sock = socket.socket(socket.AF_UNIX, socket.SOCK_STREAM)
            conn.sock.connect(sock_path)
        except Exception:
            import socket
            conn.sock = socket.socket(socket.AF_UNIX, socket.SOCK_STREAM)
            conn.sock.connect(sock_path)
        qs = urllib.parse.urlencode(params) if params else ""
        full_path = f"{path}?{qs}" if qs else path
        conn.request(method, full_path)
        resp_raw = conn.getresponse()
        body = resp_raw.read().decode("utf-8")
        conn.close()
        try:
            data = json.loads(body)
        except ValueError:
            data = body
        return data

    try:
        return resp.json()
    except ValueError:
        return resp.text

def {{.FunctionName}}(action="list", container=None, tail=100, all=False):
    """{{.Description}}"""
    import subprocess

    try:
        if action == "list":
            cmd = ["docker", "ps"]
            if all:
                cmd.append("-a")
            result = subprocess.run(cmd, capture_output=True, text=True, timeout=15)
            if result.returncode != 0:
                return {"status": "error", "message": result.stderr.strip()}
            lines = result.stdout.strip().split("\n")
            containers = []
            if len(lines) > 1:
                header = lines[0]
                for line in lines[1:]:
                    containers.append({"raw": line.strip()})
            return {"status": "success", "result": {"count": len(containers), "containers": containers}}

        elif action == "inspect":
            if not container:
                return {"status": "error", "message": "Container name or ID required"}
            result = subprocess.run(["docker", "inspect", container], capture_output=True, text=True, timeout=15)
            if result.returncode != 0:
                return {"status": "error", "message": result.stderr.strip()}
            data = json.loads(result.stdout)
            return {"status": "success", "result": f"<external_data>{json.dumps(data, ensure_ascii=False)}</external_data>"}

        elif action in ("start", "stop", "restart"):
            if not container:
                return {"status": "error", "message": "Container name or ID required"}
            result = subprocess.run(["docker", action, container], capture_output=True, text=True, timeout=30)
            if result.returncode != 0:
                return {"status": "error", "message": result.stderr.strip()}
            return {"status": "success", "result": f"Container {container}: {action} completed"}

        elif action == "logs":
            if not container:
                return {"status": "error", "message": "Container name or ID required"}
            result = subprocess.run(["docker", "logs", "--tail", str(tail), container], capture_output=True, text=True, timeout=15)
            if result.returncode != 0:
                return {"status": "error", "message": result.stderr.strip()}
            return {"status": "success", "result": result.stdout[-5000:]}

        elif action == "stats":
            if not container:
                return {"status": "error", "message": "Container name or ID required"}
            result = subprocess.run(["docker", "stats", "--no-stream", container], capture_output=True, text=True, timeout=15)
            if result.returncode != 0:
                return {"status": "error", "message": result.stderr.strip()}
            data = {"raw": result.stdout.strip()}
            return {"status": "success", "result": data}

        else:
            return {"status": "error", "message": f"Unknown action: {action}. Use: list, inspect, start, stop, restart, logs, stats"}

    except subprocess.TimeoutExpired:
        return {"status": "error", "message": f"Docker command timed out"}
    except FileNotFoundError:
        return {"status": "error", "message": "Docker CLI not found. Is Docker installed?"}
    except Exception as e:
        return {"status": "error", "message": str(e)}
`

const backupRunnerTemplateBody = `import sys
import json
import os
import glob
import hashlib
import tarfile
import tempfile
from datetime import datetime

BACKUP_DIR = os.environ.get("AURAGO_BACKUP_DIR", os.path.join(os.getcwd(), "backups"))

def _generate_backup_name(source):
    name = os.path.basename(os.path.normpath(source))
    timestamp = datetime.now().strftime("%Y%m%d_%H%M%S")
    return f"{name}_{timestamp}.tar.gz"

def _create_backup(source, output, exclude_patterns=None):
    if not os.path.exists(source):
        return {"status": "error", "message": f"Source not found: {source}"}

    os.makedirs(BACKUP_DIR, exist_ok=True)
    if not output:
        output = os.path.join(BACKUP_DIR, _generate_backup_name(source))

    exclude_set = set()
    if exclude_patterns:
        for pat in [p.strip() for p in exclude_patterns.split(",") if p.strip()]:
            for match in glob.glob(os.path.join(source, pat), recursive=True):
                exclude_set.add(os.path.normpath(match))

    source = os.path.normpath(source)
    source_name = os.path.basename(source)
    file_count = 0
    total_size = 0

    def tar_filter(info):
        norm = os.path.normpath(info.name)
        for exc in exclude_set:
            if norm.startswith(exc):
                return None
        return info

    with tarfile.open(output, "w:gz") as tar:
        tar.add(source, arcname=source_name, filter=tar_filter)

    archive_size = os.path.getsize(output)
    with open(output, "rb") as f:
        sha256 = hashlib.sha256(f.read()).hexdigest()

    return {
        "status": "success",
        "result": {
            "archive": output,
            "size_bytes": archive_size,
            "size_human": f"{archive_size / 1024 / 1024:.1f} MB" if archive_size > 1024 * 1024 else f"{archive_size / 1024:.1f} KB",
            "sha256": sha256,
            "created_at": datetime.now().isoformat(),
        },
    }

def _list_backups():
    if not os.path.exists(BACKUP_DIR):
        return {"status": "success", "result": {"backups": [], "total": 0}}
    backups = []
    for f in sorted(glob.glob(os.path.join(BACKUP_DIR, "*.tar.gz")), reverse=True):
        stat = os.stat(f)
        backups.append({
            "file": os.path.basename(f),
            "size_bytes": stat.st_size,
            "size_human": f"{stat.st_size / 1024 / 1024:.1f} MB" if stat.st_size > 1024 * 1024 else f"{stat.st_size / 1024:.1f} KB",
            "modified": datetime.fromtimestamp(stat.st_mtime).isoformat(),
        })
    return {"status": "success", "result": {"backups": backups, "total": len(backups)}}

def _restore_backup(source, output):
    if not os.path.exists(source):
        return {"status": "error", "message": f"Archive not found: {source}"}
    if not output:
        output = os.getcwd()
    os.makedirs(output, exist_ok=True)
    with tarfile.open(source, "r:gz") as tar:
        tar.extractall(path=output)
    return {"status": "success", "result": f"Restored to {output}"}

def _cleanup_backups(keep):
    keep = int(keep)
    if not os.path.exists(BACKUP_DIR):
        return {"status": "success", "result": {"removed": 0, "kept": 0}}
    backups = sorted(glob.glob(os.path.join(BACKUP_DIR, "*.tar.gz")), reverse=True)
    kept = backups[:keep]
    removed = backups[keep:]
    for f in removed:
        os.remove(f)
    return {"status": "success", "result": {"removed": len(removed), "kept": len(kept)}}

def {{.FunctionName}}(action="create", source="", output=None, keep=5, exclude=None):
    """{{.Description}}"""
    try:
        if action == "create":
            if not source:
                return {"status": "error", "message": "Source path is required for create action"}
            return _create_backup(source, output, exclude)
        elif action == "list":
            return _list_backups()
        elif action == "restore":
            if not source:
                return {"status": "error", "message": "Archive path is required for restore action"}
            return _restore_backup(source, output)
        elif action == "cleanup":
            return _cleanup_backups(keep)
        else:
            return {"status": "error", "message": f"Unknown action: {action}. Use: create, list, restore, cleanup"}
    except Exception as e:
        return {"status": "error", "message": str(e)}
`

const databaseQueryTemplateBody = `import sys
import json
import os
import sqlite3

def {{.FunctionName}}(query, db_type="sqlite", connection="", params=None, limit=100):
    """{{.Description}}"""
    if not query:
        return {"status": "error", "message": "SQL query is required"}
    if not connection:
        return {"status": "error", "message": "Database connection (file path or connection string) is required"}

    limit = int(limit)
    query_upper = query.strip().upper()

    try:
        if db_type == "sqlite":
            if not os.path.isabs(connection):
                connection = os.path.abspath(connection)
            if not os.path.exists(connection):
                return {"status": "error", "message": f"Database file not found: {connection}"}

            conn = sqlite3.connect(connection)
            conn.row_factory = sqlite3.Row
            cursor = conn.cursor()

            if params and isinstance(params, list):
                cursor.execute(query, params)
            else:
                cursor.execute(query)

            if query_upper.startswith("SELECT"):
                rows = cursor.fetchmany(limit)
                columns = [desc[0] for desc in cursor.description] if cursor.description else []
                results = [dict(zip(columns, row)) for row in rows]
                conn.close()
                return {"status": "success", "result": {"rows": results, "count": len(results), "columns": columns}}
            else:
                affected = cursor.rowcount
                conn.commit()
                conn.close()
                return {"status": "success", "result": {"affected_rows": affected}}

        elif db_type == "postgresql":
            try:
                import psycopg2
            except ImportError:
                return {"status": "error", "message": "psycopg2 not installed. Add 'psycopg2-binary' to dependencies."}
            conn = psycopg2.connect(connection)
            cursor = conn.cursor()
            if params and isinstance(params, list):
                cursor.execute(query, params)
            else:
                cursor.execute(query)
            if query_upper.startswith("SELECT"):
                rows = cursor.fetchmany(limit)
                columns = [desc[0] for desc in cursor.description] if cursor.description else []
                results = [dict(zip(columns, row)) for row in rows]
                conn.close()
                return {"status": "success", "result": {"rows": results, "count": len(results), "columns": columns}}
            else:
                affected = cursor.rowcount
                conn.commit()
                conn.close()
                return {"status": "success", "result": {"affected_rows": affected}}

        elif db_type == "mysql":
            try:
                import pymysql
            except ImportError:
                return {"status": "error", "message": "pymysql not installed. Add 'pymysql' to dependencies."}
            conn = pymysql.connect(connection if "://" in connection else connection)
            cursor = conn.cursor()
            if params and isinstance(params, list):
                cursor.execute(query, params)
            else:
                cursor.execute(query)
            if query_upper.startswith("SELECT"):
                rows = cursor.fetchmany(limit)
                columns = [desc[0] for desc in cursor.description] if cursor.description else []
                results = [dict(zip(columns, row)) for row in rows]
                conn.close()
                return {"status": "success", "result": {"rows": results, "count": len(results), "columns": columns}}
            else:
                affected = cursor.rowcount
                conn.commit()
                conn.close()
                return {"status": "success", "result": {"affected_rows": affected}}

        else:
            return {"status": "error", "message": f"Unsupported database type: {db_type}. Use: sqlite, postgresql, mysql"}

    except Exception as e:
        return {"status": "error", "message": str(e)}
`

const sshExecutorTemplateBody = `import sys
import json
import os

def {{.FunctionName}}(host, command, user=None, port=22, timeout=30):
    """{{.Description}}"""
    if not host:
        return {"status": "error", "message": "Host is required"}
    if not command:
        return {"status": "error", "message": "Command is required"}

    try:
        import paramiko
    except ImportError:
        return {"status": "error", "message": "paramiko not installed. Add 'paramiko' to dependencies."}

    ssh_key = os.environ.get("AURAGO_SECRET_SSH_KEY", "")
    ssh_password = os.environ.get("AURAGO_SECRET_SSH_PASSWORD", "")
    if not user:
        user = os.environ.get("AURAGO_SECRET_SSH_USER", os.environ.get("USER", "root"))

    client = paramiko.SSHClient()
    client.set_missing_host_key_policy(paramiko.AutoAddPolicy())

    try:
        connect_kwargs = {
            "hostname": host,
            "port": int(port),
            "username": user,
            "timeout": int(timeout),
        }

        if ssh_key:
            key_file = os.path.expanduser(ssh_key) if not os.path.isabs(ssh_key) else ssh_key
            if os.path.isfile(key_file):
                connect_kwargs["key_filename"] = key_file
            else:
                import tempfile
                with tempfile.NamedTemporaryFile(mode="w", suffix=".key", delete=False) as kf:
                    kf.write(ssh_key)
                    kf.flush()
                    connect_kwargs["key_filename"] = kf.name
        elif ssh_password:
            connect_kwargs["password"] = ssh_password
        else:
            key_path = os.path.expanduser("~/.ssh/id_rsa")
            if os.path.isfile(key_path):
                connect_kwargs["key_filename"] = key_path

        client.connect(**connect_kwargs)
        stdin, stdout, stderr = client.exec_command(command, timeout=int(timeout))

        exit_code = stdout.channel.recv_exit_status()
        out = stdout.read().decode("utf-8", errors="replace")
        err = stderr.read().decode("utf-8", errors="replace")

        return {
            "status": "success" if exit_code == 0 else "error",
            "result": {
                "host": host,
                "command": command,
                "exit_code": exit_code,
                "stdout": out[-10000:] if len(out) > 10000 else out,
                "stderr": err[-5000:] if len(err) > 5000 else err,
            },
        }
    except paramiko.AuthenticationException:
        return {"status": "error", "message": f"Authentication failed for {user}@{host}"}
    except paramiko.SSHException as e:
        return {"status": "error", "message": f"SSH error: {str(e)}"}
    except Exception as e:
        return {"status": "error", "message": str(e)}
    finally:
        client.close()
`

const mqttPublisherTemplateBody = `import sys
import json
import os

def {{.FunctionName}}(action, topic, payload=None, qos=0, retain=False, timeout=5):
    """{{.Description}}"""
    if not topic:
        return {"status": "error", "message": "MQTT topic is required"}

    try:
        import paho.mqtt.client as mqtt
    except ImportError:
        return {"status": "error", "message": "paho-mqtt not installed. Add 'paho-mqtt' to dependencies."}

    broker_host = os.environ.get("AURAGO_SECRET_MQTT_HOST", os.environ.get("AURAGO_SECRET_BROKER_HOST", "localhost"))
    broker_port = int(os.environ.get("AURAGO_SECRET_MQTT_PORT", "1883"))
    mqtt_user = os.environ.get("AURAGO_SECRET_MQTT_USER", "")
    mqtt_password = os.environ.get("AURAGO_SECRET_MQTT_PASSWORD", "")

    qos = int(qos)
    retain = bool(retain)
    timeout = int(timeout)

    client = mqtt.Client(mqtt.CallbackAPIVersion.VERSION2)
    if mqtt_user and mqtt_password:
        client.username_pw_set(mqtt_user, mqtt_password)

    try:
        client.connect(broker_host, broker_port, keepalive=60)
        client.loop_start()

        if action == "publish":
            if payload is None:
                return {"status": "error", "message": "Payload is required for publish action"}
            if not isinstance(payload, str):
                payload = json.dumps(payload)
            result = client.publish(topic, payload, qos=qos, retain=retain)
            result.wait_for_publish(timeout=timeout)
            return {
                "status": "success",
                "result": {
                    "action": "publish",
                    "topic": topic,
                    "broker": f"{broker_host}:{broker_port}",
                    "qos": qos,
                    "retained": retain,
                    "payload_size": len(payload),
                },
            }

        elif action == "subscribe":
            messages = []

            def on_message(client, userdata, msg):
                messages.append({"topic": msg.topic, "payload": msg.payload.decode("utf-8", errors="replace"), "qos": msg.qos})

            client.on_message = on_message
            client.subscribe(topic, qos=qos)

            import time
            deadline = time.time() + timeout
            while time.time() < deadline:
                time.sleep(0.1)

            client.unsubscribe(topic)
            return {
                "status": "success",
                "result": {
                    "action": "subscribe",
                    "topic": topic,
                    "broker": f"{broker_host}:{broker_port}",
                    "messages_received": len(messages),
                    "messages": messages[:50],
                },
            }

        else:
            return {"status": "error", "message": f"Unknown action: {action}. Use: publish, subscribe"}

    except Exception as e:
        return {"status": "error", "message": str(e)}
    finally:
        client.loop_stop()
        client.disconnect()
`
