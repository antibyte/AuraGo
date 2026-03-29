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

// SkillTemplate defines a reusable blueprint for creating new skills.
type SkillTemplate struct {
	Name         string            `json:"name"`
	Description  string            `json:"description"`
	Parameters   map[string]string `json:"parameters"`
	Dependencies []string          `json:"dependencies"`
	Code         string            `json:"-"` // Go text/template for Python code
}

// AvailableSkillTemplates returns all built-in skill templates.
func AvailableSkillTemplates() []SkillTemplate {
	return []SkillTemplate{
		{
			Name:        "api_client",
			Description: "REST API client with authentication, configurable base URL, and vault key injection for API keys.",
			Parameters: map[string]string{
				"endpoint": "API endpoint path (appended to base URL)",
				"method":   "HTTP method: GET, POST, PUT, DELETE (default: GET)",
				"body":     "JSON request body (optional, for POST/PUT)",
			},
			Dependencies: []string{"requests"},
			Code:         apiClientTemplate,
		},
		{
			Name:        "file_processor",
			Description: "Read, transform, and write files. Supports text processing, line filtering, and content extraction.",
			Parameters: map[string]string{
				"input_path":  "Path to the input file",
				"output_path": "Path to the output file (optional, prints to stdout if omitted)",
				"operation":   "Operation: extract_lines, search, replace, head, tail, count",
				"pattern":     "Regex or search pattern (for search/replace/extract_lines)",
				"replacement": "Replacement string (for replace operation)",
			},
			Dependencies: nil,
			Code:         fileProcessorTemplate,
		},
		{
			Name:        "data_transformer",
			Description: "Convert data between JSON, CSV, and YAML formats with optional field filtering and transformation.",
			Parameters: map[string]string{
				"input_path":    "Path to the input file",
				"output_path":   "Path to the output file (optional, prints to stdout if omitted)",
				"input_format":  "Input format: json, csv, yaml",
				"output_format": "Output format: json, csv, yaml",
				"fields":        "Comma-separated list of fields to include (optional, all if omitted)",
			},
			Dependencies: []string{"pyyaml"},
			Code:         dataTransformerTemplate,
		},
		{
			Name:        "scraper",
			Description: "Web scraper using BeautifulSoup4 with CSS selectors. Wraps output in <external_data> tags for safety.",
			Parameters: map[string]string{
				"url":      "URL to scrape",
				"selector": "CSS selector to extract elements (default: body)",
				"attr":     "HTML attribute to extract from elements (optional, extracts text if omitted)",
				"limit":    "Maximum number of elements to return (default: 50)",
			},
			Dependencies: []string{"requests", "beautifulsoup4"},
			Code:         scraperTemplate,
		},
		{
			Name:        "example_use_vault_login",
			Description: "Example: authenticate with username + password from the vault, then call a REST API. Create vault secrets 'my_service_username' and 'my_service_password' in Config → Vault before use.",
			Parameters: map[string]string{
				"action":   "API action / path to call after login (default: whoami)",
				"base_url": "Base URL of the service (can also be set via AURAGO_SECRET_BASE_URL vault key)",
			},
			Dependencies: []string{"requests"},
			Code:         vaultLoginTemplate,
		},
		{
			Name:        "example_use_vault_token",
			Description: "Example: call a REST API using a single Bearer token / API key from the vault. Create a vault secret 'my_service_api_key' in Config → Vault before use.",
			Parameters: map[string]string{
				"endpoint": "API endpoint path to call (appended to base URL)",
				"method":   "HTTP method: GET, POST, PUT, DELETE (default: GET)",
				"body":     "JSON request body (optional, for POST/PUT)",
			},
			Dependencies: []string{"requests"},
			Code:         vaultTokenTemplate,
		},
	}
}

type templateData struct {
	FunctionName string
	Description  string
	BaseURL      string
}

var validFuncNameRe = regexp.MustCompile(`[^a-zA-Z0-9_]`)

// escapePythonString escapes a string for safe inclusion in a Python single-quoted
// string literal: backslashes, single quotes, and control characters are escaped.
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

// toFunctionName converts a skill name to a valid Python function name.
func toFunctionName(name string) string {
	fn := validFuncNameRe.ReplaceAllString(name, "_")
	fn = strings.Trim(fn, "_")
	if fn == "" {
		fn = "skill_main"
	}
	// Ensure it doesn't start with a digit
	if fn[0] >= '0' && fn[0] <= '9' {
		fn = "skill_" + fn
	}
	return fn
}

// CreateSkillFromTemplate generates a new skill from a built-in template.
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

	// Find template
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

	// Render Python code from template
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

	// Merge dependencies: template defaults + user-provided extras
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

	// Build manifest
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

	// Ensure skills directory exists
	if err := os.MkdirAll(skillsDir, 0o750); err != nil {
		return "", fmt.Errorf("failed to create skills directory: %w", err)
	}

	// Write manifest JSON
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

	// Write Python code
	if err := validateSkillCode(codeBuf.String()); err != nil {
		_ = os.Remove(jsonPath)
		return "", err
	}
	if err := writeFileExclusive(pyPath, codeBuf.Bytes(), 0o640); err != nil {
		// Clean up manifest if Python file write fails
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

// ──────────────────────────────────────────────────────────────────────────────
// Python code templates (Go text/template syntax)
// ──────────────────────────────────────────────────────────────────────────────

const apiClientTemplate = `import sys
import json
import os
import requests

def {{.FunctionName}}(endpoint, method="GET", body=None):
    """{{.Description}}"""
    base_url = os.environ.get("AURAGO_SECRET_BASE_URL", "{{.BaseURL}}").rstrip("/")
    api_key = os.environ.get("AURAGO_SECRET_API_KEY", "")
    
    url = f"{base_url}/{endpoint.lstrip('/')}" if endpoint else base_url
    
    headers = {"Content-Type": "application/json"}
    if api_key:
        headers["Authorization"] = f"Bearer {api_key}"
    
    try:
        resp = requests.request(
            method=method.upper(),
            url=url,
            headers=headers,
            json=body if body else None,
            timeout=30,
        )
        resp.raise_for_status()
        try:
            data = resp.json()
        except ValueError:
            data = resp.text
        return {"status": "success", "result": f"<external_data>{json.dumps(data, ensure_ascii=False)}</external_data>"}
    except requests.RequestException as e:
        return {"status": "error", "message": str(e)}

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
    result = {{.FunctionName}}(
        endpoint=args.get("endpoint", ""),
        method=args.get("method", "GET"),
        body=args.get("body"),
    )
    print(json.dumps(result, ensure_ascii=False))
`

const fileProcessorTemplate = `import sys
import json
import os
import re

def {{.FunctionName}}(input_path, output_path=None, operation="head", pattern=None, replacement=None):
    """{{.Description}}"""
    if not os.path.isabs(input_path):
        input_path = os.path.abspath(input_path)
    if not os.path.exists(input_path):
        return {"status": "error", "message": f"File not found: {input_path}"}
    
    try:
        with open(input_path, "r", encoding="utf-8") as f:
            lines = f.readlines()
        
        result_lines = lines
        if operation == "head":
            result_lines = lines[:20]
        elif operation == "tail":
            result_lines = lines[-20:]
        elif operation == "count":
            return {"status": "success", "result": {"lines": len(lines), "chars": sum(len(l) for l in lines)}}
        elif operation == "search" and pattern:
            result_lines = [l for l in lines if re.search(pattern, l)]
        elif operation == "replace" and pattern and replacement is not None:
            result_lines = [re.sub(pattern, replacement, l) for l in lines]
        elif operation == "extract_lines" and pattern:
            result_lines = [l for l in lines if re.search(pattern, l)]
        
        output = "".join(result_lines)
        if output_path:
            if not os.path.isabs(output_path):
                output_path = os.path.abspath(output_path)
            with open(output_path, "w", encoding="utf-8") as f:
                f.write(output)
            return {"status": "success", "result": f"Written {len(result_lines)} lines to {output_path}"}
        return {"status": "success", "result": output}
    except Exception as e:
        return {"status": "error", "message": str(e)}

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
    result = {{.FunctionName}}(
        input_path=args.get("input_path", ""),
        output_path=args.get("output_path"),
        operation=args.get("operation", "head"),
        pattern=args.get("pattern"),
        replacement=args.get("replacement"),
    )
    print(json.dumps(result, ensure_ascii=False))
`

const dataTransformerTemplate = `import sys
import json
import os
import csv
import io

try:
    import yaml
except ImportError:
    yaml = None

def {{.FunctionName}}(input_path, output_path=None, input_format="json", output_format="json", fields=None):
    """{{.Description}}"""
    if not os.path.isabs(input_path):
        input_path = os.path.abspath(input_path)
    if not os.path.exists(input_path):
        return {"status": "error", "message": f"File not found: {input_path}"}
    
    try:
        with open(input_path, "r", encoding="utf-8") as f:
            raw = f.read()
        
        # Parse input
        if input_format == "json":
            data = json.loads(raw)
        elif input_format == "csv":
            reader = csv.DictReader(io.StringIO(raw))
            data = list(reader)
        elif input_format == "yaml":
            if yaml is None:
                return {"status": "error", "message": "pyyaml not installed"}
            data = yaml.safe_load(raw)
        else:
            return {"status": "error", "message": f"Unsupported input format: {input_format}"}
        
        # Filter fields if specified
        if fields and isinstance(data, list):
            field_list = [f.strip() for f in fields.split(",")]
            data = [{k: row.get(k) for k in field_list} for row in data]
        
        # Render output
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
    result = {{.FunctionName}}(
        input_path=args.get("input_path", ""),
        output_path=args.get("output_path"),
        input_format=args.get("input_format", "json"),
        output_format=args.get("output_format", "json"),
        fields=args.get("fields"),
    )
    print(json.dumps(result, ensure_ascii=False))
`

const scraperTemplate = `import sys
import json
import requests
from bs4 import BeautifulSoup

def {{.FunctionName}}(url, selector="body", attr=None, limit=50):
    """{{.Description}}"""
    try:
        resp = requests.get(url, timeout=30, headers={
            "User-Agent": "Mozilla/5.0 (compatible; AuraGo-Skill/1.0)",
        })
        resp.raise_for_status()
        soup = BeautifulSoup(resp.text, "html.parser")
        
        elements = soup.select(selector)[:int(limit)]
        results = []
        for el in elements:
            if attr:
                val = el.get(attr, "")
            else:
                val = el.get_text(strip=True)
            if val:
                results.append(val)
        
        return {"status": "success", "result": f"<external_data>{json.dumps(results, ensure_ascii=False)}</external_data>"}
    except requests.RequestException as e:
        return {"status": "error", "message": str(e)}
    except Exception as e:
        return {"status": "error", "message": str(e)}

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
    result = {{.FunctionName}}(
        url=args.get("url", ""),
        selector=args.get("selector", "body"),
        attr=args.get("attr"),
        limit=args.get("limit", 50),
    )
    print(json.dumps(result, ensure_ascii=False))
`

// ─── Vault example templates ──────────────────────────────────────────────────

// vaultLoginTemplate demonstrates authenticating with username + password from
// the AuraGo Vault and then calling a service API.
//
// Required vault secrets (create them in Config → Vault):
//
//	Key: "my_service_username"   → injected as AURAGO_SECRET_MY_SERVICE_USERNAME
//	Key: "my_service_password"   → injected as AURAGO_SECRET_MY_SERVICE_PASSWORD
//	Key: "my_service_base_url"   → injected as AURAGO_SECRET_MY_SERVICE_BASE_URL  (optional)
//
// Declaration in the skill manifest (vault_keys field):
//
//	"vault_keys": ["my_service_username", "my_service_password", "my_service_base_url"]
const vaultLoginTemplate = `import sys
import json
import os
import requests

# ─── HOW TO SET UP THIS SKILL ────────────────────────────────────────────────
# Before running this skill, store your credentials in the AuraGo Vault:
#
#   1. Open the AuraGo Web UI → Config → Vault
#   2. Add two (or three) secrets:
#        Key: "my_service_username"   Value: <your username or login>
#        Key: "my_service_password"   Value: <your password>
#        Key: "my_service_base_url"   Value: <https://your-service.example.com>  (optional)
#
#   3. In the skill manifest (.json), declare them in "vault_keys":
#        "vault_keys": ["my_service_username", "my_service_password", "my_service_base_url"]
#
# AuraGo will then inject the secrets as environment variables before each run:
#   AURAGO_SECRET_MY_SERVICE_USERNAME  = <username>
#   AURAGO_SECRET_MY_SERVICE_PASSWORD  = <password>
#   AURAGO_SECRET_MY_SERVICE_BASE_URL  = <base url>   (if declared)
#
# Rename "my_service" to something meaningful, e.g. "nextcloud", "gitlab_local".
# ─────────────────────────────────────────────────────────────────────────────

def {{.FunctionName}}(action="whoami"):
    """{{.Description}}"""
    # Read credentials injected by AuraGo from the vault
    base_url = os.environ.get("AURAGO_SECRET_MY_SERVICE_BASE_URL",
                              os.environ.get("AURAGO_SECRET_BASE_URL", "{{.BaseURL}}")).rstrip("/")
    username = os.environ.get("AURAGO_SECRET_MY_SERVICE_USERNAME", "")
    password = os.environ.get("AURAGO_SECRET_MY_SERVICE_PASSWORD", "")

    if not username or not password:
        return {
            "status": "error",
            "message": (
                "Credentials not found in environment. "
                "Please add 'my_service_username' and 'my_service_password' to the AuraGo Vault "
                "(Config → Vault) and declare them in the skill's vault_keys."
            )
        }

    session = requests.Session()
    try:
        # ── Step 1: Login and obtain a session token ──────────────────────────
        # Adjust the login endpoint and payload to match your service.
        login_resp = session.post(
            f"{base_url}/auth/login",
            json={"username": username, "password": password},
            timeout=15,
        )
        login_resp.raise_for_status()

        # Many APIs return a Bearer token on login; fall back to cookie sessions.
        token = login_resp.json().get("token") or login_resp.json().get("access_token", "")
        if token:
            session.headers.update({"Authorization": f"Bearer {token}"})

        # ── Step 2: Call the actual API ────────────────────────────────────────
        # Replace "/api/{action}" with the real endpoint your service provides.
        resp = session.get(f"{base_url}/api/{action}", timeout=15)
        resp.raise_for_status()

        try:
            data = resp.json()
        except ValueError:
            data = resp.text

        return {
            "status": "success",
            "result": f"<external_data>{json.dumps(data, ensure_ascii=False)}</external_data>"
        }

    except requests.RequestException as e:
        return {"status": "error", "message": str(e)}


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
    result = {{.FunctionName}}(
        action=args.get("action", "whoami"),
    )
    print(json.dumps(result, ensure_ascii=False))
`

// vaultTokenTemplate demonstrates calling a REST API with a single Bearer
// token / API key from the AuraGo Vault.
//
// Required vault secrets (create them in Config → Vault):
//
//	Key: "my_service_api_key"  → injected as AURAGO_SECRET_MY_SERVICE_API_KEY
//	Key: "my_service_base_url" → injected as AURAGO_SECRET_MY_SERVICE_BASE_URL  (optional)
//
// Declaration in the skill manifest (vault_keys field):
//
//	"vault_keys": ["my_service_api_key", "my_service_base_url"]
const vaultTokenTemplate = `import sys
import json
import os
import requests

# ─── HOW TO SET UP THIS SKILL ────────────────────────────────────────────────
# Before running this skill, store your API key in the AuraGo Vault:
#
#   1. Open the AuraGo Web UI → Config → Vault
#   2. Add one (or two) secrets:
#        Key: "my_service_api_key"   Value: <your API key or Bearer token>
#        Key: "my_service_base_url"  Value: <https://api.your-service.example.com>  (optional)
#
#   3. In the skill manifest (.json), declare them in "vault_keys":
#        "vault_keys": ["my_service_api_key", "my_service_base_url"]
#
# AuraGo will inject the secret before each run:
#   AURAGO_SECRET_MY_SERVICE_API_KEY   = <api key>
#   AURAGO_SECRET_MY_SERVICE_BASE_URL  = <base url>  (if declared)
#
# Rename "my_service" to something meaningful, e.g. "openweathermap", "hass_token".
# ─────────────────────────────────────────────────────────────────────────────

def {{.FunctionName}}(endpoint, method="GET", body=None):
    """{{.Description}}"""
    # Read API key and base URL injected by AuraGo from the vault
    base_url = os.environ.get("AURAGO_SECRET_MY_SERVICE_BASE_URL",
                              os.environ.get("AURAGO_SECRET_BASE_URL", "{{.BaseURL}}")).rstrip("/")
    api_key = os.environ.get("AURAGO_SECRET_MY_SERVICE_API_KEY",
                             os.environ.get("AURAGO_SECRET_API_KEY", ""))

    if not api_key:
        return {
            "status": "error",
            "message": (
                "API key not found in environment. "
                "Please add 'my_service_api_key' to the AuraGo Vault "
                "(Config → Vault) and declare it in the skill's vault_keys."
            )
        }

    url = f"{base_url}/{endpoint.lstrip('/')}" if endpoint else base_url
    headers = {
        "Content-Type": "application/json",
        # Standard Bearer auth – change to a different header if your service requires it,
        # e.g. "X-API-Key": api_key  or  "token": api_key
        "Authorization": f"Bearer {api_key}",
    }

    try:
        resp = requests.request(
            method=method.upper(),
            url=url,
            headers=headers,
            json=body if body else None,
            timeout=30,
        )
        resp.raise_for_status()

        try:
            data = resp.json()
        except ValueError:
            data = resp.text

        return {
            "status": "success",
            "result": f"<external_data>{json.dumps(data, ensure_ascii=False)}</external_data>"
        }

    except requests.RequestException as e:
        return {"status": "error", "message": str(e)}


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
    result = {{.FunctionName}}(
        endpoint=args.get("endpoint", ""),
        method=args.get("method", "GET"),
        body=args.get("body"),
    )
    print(json.dumps(result, ensure_ascii=False))
`
