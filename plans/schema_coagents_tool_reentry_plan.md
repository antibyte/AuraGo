# Implementierungsplan: Schema-valide Co-Agents + Tool-Reentry

**Projekt:** AuraGo
**Feature A:** JSON-Schema-validierte Co-Agent-Outputs
**Feature B:** Python Tool-Reentry (aurago_sdk)
**Geschätzter Aufwand:** 12–15 Arbeitstage
**Ziel:** Robuste Delegation (keine Prosa-Suppe) + Python als Werkzeugarbeiter

---

## Teil A: Schema-valide Co-Agents

### A.1. Architektur-Überblick

```
┌─────────────────────────────────────────────────────────────┐
│  Parent Agent                                               │
│   └──> co_agent spawn                                       │
│         ├── role: "security_auditor"                        │
│         ├── task: "Prüfe auf Injection-Risiken"            │
│         └── output_schema: {                                │
│               "type": "object",                             │
│               "properties": {                               │
│                 "findings": {                               │
│                   "type": "array",                          │
│                   "items": {                                │
│                     "file": {"type":"string"},              │
│                     "line": {"type":"integer"},             │
│                     "severity": {"enum":["low","high"]}     │
│                   }                                         │
│                 }                                           │
│               }                                             │
│             }                                               │
│                      ↓                                      │
│              ┌──────────────┐                               │
│              │ Co-Agent     │── runs ExecuteAgentLoop      │
│              │ (isoliert)   │   mit eigenem HistoryMgr     │
│              └──────┬───────┘                               │
│                     ↓ Rohtext-Ergebnis                      │
│              ┌──────────────┐                               │
│              │ LLM Extract  │── forced JSON mode           │
│              │ + Validate   │   gegen output_schema        │
│              └──────┬───────┘                               │
│                     ↓ Validiertes JSON                      │
│              Parent Agent bekommt structurierte Daten       │
└─────────────────────────────────────────────────────────────┘
```

### A.2. Motivation

**Aktuelles Problem in AuraGo (`internal/agent/coagent.go`):**
- Co-Agent liefert Rohtext (oft 500–2000 Wörter)
- Parent-Agent muss daraus Pfade, Fehler, Prioritäten extrahieren
- Das ist token-teuer und fehleranfällig
- Keine Garantie, dass der Output ein parsbares Format hat

**Ziel:**
- Co-Agent-Output wird nach Abschluss durch einen Extractor-LLM-Aufruf in validiertes JSON überführt
- Der Parent-Agent bekommt garantiert schema-konforme Daten
- Fehlgeschlagene Extraktionen fallen auf Rohtext zurück

### A.3. Dateien: Neu / Modifiziert

| Datei | Aktion | Beschreibung |
|-------|--------|--------------|
| `internal/agent/coagent_schema.go` | **Neu** | Schema-Validierung, LLM-Extraktion, Fallback-Handling |
| `internal/agent/coagent.go` | **Modifizieren** | `CoAgentRequest` um `OutputSchema` erweitern; Integration in `runCoAgentTask` |
| `internal/agent/native_tools_edge.go` | **Modifizieren** | `co_agent` Tool-Schema um `output_schema` erweitern |
| `internal/agent/agent_dispatch_planner.go` | **Modifizieren** | Dispatch für `co_agent` um Schema-Parameter ergänzen |
| `go.mod` | **Modifizieren** | Neue Dependency: `github.com/xeipuuv/gojsonschema` |
| `internal/agent/coagent_schema_test.go` | **Neu** | Unit-Tests für Extraktion und Validierung |

### A.4. Schritt-für-Schritt-Implementierung

#### Tag 1–2: Schema-Infrastruktur

**Datei:** `internal/agent/coagent_schema.go` (neu)

```go
package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/sashabaranov/go-openai"
	"github.com/xeipuuv/gojsonschema"
)

// CoAgentSchemaExtractor handles post-run extraction of structured data from co-agent raw output.
type CoAgentSchemaExtractor struct {
	LLMClient LLMClient // existing interface
	Logger    *slog.Logger
}

// ExtractionResult holds the outcome of schema extraction.
type ExtractionResult struct {
	Valid     bool            `json:"valid"`
	Data      json.RawMessage `json:"data,omitempty"`
	RawText   string          `json:"raw_text,omitempty"`
	Error     string          `json:"error,omitempty"`
	UsedJSONMode bool         `json:"used_json_mode"`
}

// Extract attempts to convert co-agent raw output into schema-valid JSON.
func (e *CoAgentSchemaExtractor) Extract(ctx context.Context, rawOutput string, schema map[string]interface{}) ExtractionResult {
	// Strategy 1: Try to find JSON already embedded in the raw output
	if data := extractJSONFromText(rawOutput); data != nil {
		if valid, err := validateAgainstSchema(data, schema); valid {
			return ExtractionResult{Valid: true, Data: data, RawText: rawOutput, UsedJSONMode: false}
		} else {
			e.Logger.Debug("Embedded JSON failed schema validation", "error", err)
		}
	}

	// Strategy 2: LLM extraction with forced JSON mode
	extracted, err := e.extractWithLLM(ctx, rawOutput, schema)
	if err != nil {
		return ExtractionResult{Valid: false, RawText: rawOutput, Error: fmt.Sprintf("LLM extraction failed: %v", err)}
	}

	if valid, err := validateAgainstSchema(extracted, schema); valid {
		return ExtractionResult{Valid: true, Data: extracted, RawText: rawOutput, UsedJSONMode: true}
	} else {
		return ExtractionResult{
			Valid:   false,
			RawText: rawOutput,
			Error:   fmt.Sprintf("Extracted JSON failed schema validation: %v", err),
		}
	}
}

// extractJSONFromText tries to find a JSON object/array in raw text.
func extractJSONFromText(text string) json.RawMessage {
	// Look for ```json ... ``` blocks
	if idx := strings.Index(text, "```json"); idx >= 0 {
		start := idx + len("```json")
		if end := strings.Index(text[start:], "```"); end >= 0 {
			candidate := strings.TrimSpace(text[start : start+end])
			var dummy interface{}
			if json.Unmarshal([]byte(candidate), &dummy) == nil {
				return json.RawMessage(candidate)
			}
		}
	}
	// Look for first { ... } or [ ... ] at top level
	// Simple heuristic: find first '{' and matching '}'
	start := strings.Index(text, "{")
	if start < 0 {
		start = strings.Index(text, "[")
	}
	if start >= 0 {
		for end := start + 1; end <= len(text); end++ {
			candidate := text[start:end]
			var dummy interface{}
			if json.Unmarshal([]byte(candidate), &dummy) == nil {
				return json.RawMessage(candidate)
			}
		}
	}
	return nil
}

func (e *CoAgentSchemaExtractor) extractWithLLM(ctx context.Context, rawOutput string, schema map[string]interface{}) (json.RawMessage, error) {
	schemaJSON, _ := json.Marshal(schema)

	prompt := fmt.Sprintf(`You are a structured data extractor. Given the raw text below from a sub-agent, extract the information into a JSON object that strictly conforms to this JSON Schema:

%s

Rules:
- Output ONLY valid JSON. No markdown, no explanations.
- If a field is missing in the text, use null or an empty array as appropriate.
- Do NOT invent data not present in the text.

Raw text:
---
%s
---`, schemaJSON, rawOutput)

	req := openai.ChatCompletionRequest{
		Model: "gpt-4o-mini", // or config.CoAgents.ExtractionModel
		Messages: []openai.ChatCompletionMessage{
			{Role: openai.ChatMessageRoleSystem, Content: "You extract structured JSON from unstructured text. Output only JSON."},
			{Role: openai.ChatMessageRoleUser, Content: prompt},
		},
		ResponseFormat: &openai.ChatCompletionResponseFormat{
			Type: openai.ChatCompletionResponseFormatTypeJSONObject,
		},
		MaxTokens: 2000,
	}

	resp, err := e.LLMClient.CreateChatCompletion(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("LLM call failed: %w", err)
	}
	if len(resp.Choices) == 0 {
		return nil, fmt.Errorf("LLM returned no choices")
	}

	content := strings.TrimSpace(resp.Choices[0].Message.Content)
	return json.RawMessage(content), nil
}

func validateAgainstSchema(data json.RawMessage, schema map[string]interface{}) (bool, error) {
	schemaLoader := gojsonschema.NewGoLoader(schema)
	documentLoader := gojsonschema.NewBytesLoader(data)
	result, err := gojsonschema.Validate(schemaLoader, documentLoader)
	if err != nil {
		return false, err
	}
	if !result.Valid() {
		var errs []string
		for _, desc := range result.Errors() {
			errs = append(errs, desc.String())
		}
		return false, fmt.Errorf("schema validation failed: %s", strings.Join(errs, "; "))
	}
	return true, nil
}
```

#### Tag 2–3: CoAgentRequest erweitern & Integration

**Datei:** `internal/agent/coagent.go`

`CoAgentRequest` erweitern:
```go
type CoAgentRequest struct {
	Task         string                 `json:"task"`
	ContextHints []string               `json:"context_hints,omitempty"`
	Specialist   string                 `json:"specialist,omitempty"`
	Priority     int                    `json:"priority,omitempty"`
	OutputSchema map[string]interface{} `json:"output_schema,omitempty"` // NEW
}
```

`CoAgentResult` erweitern:
```go
type CoAgentResult struct {
	ID        string          `json:"id"`
	Status    string          `json:"status"`
	Output    string          `json:"output"`      // Raw text
	Structured json.RawMessage `json:"structured,omitempty"` // NEW: extracted data
	Valid     bool            `json:"valid"`       // NEW: whether structured extraction succeeded
	Error     string          `json:"error,omitempty"`
	Tokens    int             `json:"tokens"`
	Duration  time.Duration   `json:"duration"`
}
```

In der `runCoAgentTask`-Goroutine (nach Abschluss des Agent-Loops):
```go
func runCoAgentTask(...) {
    // ... existing agent loop execution ...
    
    rawOutput := coAgentHistory.GetLastAssistantMessage() // or similar
    
    result := CoAgentResult{
        ID:       coID,
        Status:   "completed",
        Output:   rawOutput,
        Tokens:   totalTokens,
        Duration: time.Since(start),
    }
    
    // NEW: Schema extraction if requested
    if req.OutputSchema != nil && llmClient != nil {
        extractor := &CoAgentSchemaExtractor{
            LLMClient: llmClient,
            Logger:    logger,
        }
        extraction := extractor Extract(ctx, rawOutput, req.OutputSchema)
        result.Valid = extraction.Valid
        result.Structured = extraction.Data
        if !extraction.Valid {
            result.Error = extraction.Error
        }
    }
    
    registry.RecordResult(coID, result)
}
```

**Wichtig:** `llmClient` muss an die Co-Agent-Goroutine durchgereicht werden. Aktuell wird es über `DispatchContext` übergeben, aber im Co-Agent-Spawn-Code muss sichergestellt werden, dass ein LLM-Client für die Extraktion verfügbar ist.

#### Tag 3–4: Tool-Schema & Dispatch

**Datei:** `internal/agent/native_tools_edge.go` oder `native_tools_planner.go`

`co_agent` Schema erweitern:
```go
tool("co_agent",
    "Spawn a specialized co-agent subtask with optional structured output schema. "+
        "Co-agents run in isolated sessions and can be assigned roles like researcher, coder, designer, security, or writer. "+
        "If output_schema is provided, the co-agent's raw output will be automatically extracted and validated against the schema.",
    schema(map[string]interface{}{
        "action": map[string]interface{}{
            "type":        "string",
            "description": "Co-agent action",
            "enum":        []string{"spawn", "status", "result", "cancel"},
        },
        "task":         prop("string", "Task description for spawn"),
        "role":         prop("string", "Specialist role: researcher, coder, designer, security, writer"),
        "priority":     map[string]interface{}{"type": "integer", "description": "1=low, 2=normal, 3=high"},
        "co_agent_id":  prop("string", "Co-agent ID for status/result/cancel"),
        // NEW
        "output_schema": map[string]interface{}{
            "type":        "object",
            "description": "Optional JSON Schema object. If provided, the co-agent's final output will be extracted into valid JSON conforming to this schema. Example: {\"type\":\"object\",\"properties\":{\"findings\":{\"type\":\"array\"}}}",
        },
    }, "action"),
)
```

**Datei:** `internal/agent/agent_dispatch_planner.go`

Im `co_agent`-Dispatch:
```go
case "co_agent":
    action := stringValueFromMap(tc.Params, "action")
    switch action {
    case "spawn":
        req := CoAgentRequest{
            Task:       stringValueFromMap(tc.Params, "task"),
            Specialist: stringValueFromMap(tc.Params, "role"),
            ContextHints: []string{},
        }
        if priority, err := strconv.Atoi(stringValueFromMap(tc.Params, "priority")); err == nil {
            req.Priority = priority
        }
        // Parse output_schema
        if schemaRaw, ok := tc.Params["output_schema"]; ok {
            if schemaMap, ok := schemaRaw.(map[string]interface{}); ok {
                req.OutputSchema = schemaMap
            }
        }
        coID, state, err := SpawnCoAgent(cfg, ctx, logger, coRegistry, ... , req, budgetTracker, nil)
        // ...
    case "result":
        coID := stringValueFromMap(tc.Params, "co_agent_id")
        result, err := coRegistry.GetResult(coID)
        if err != nil {
            return fmt.Sprintf(`Tool Output: {"status":"error","message":"%v"}`, err)
        }
        // Return structured data if available
        if result.Valid && result.Structured != nil {
            return fmt.Sprintf(`Tool Output: {"status":"success","structured":%s,"raw_preview":"%s"}`,
                string(result.Structured), Truncate(result.Output, 200))
        }
        return fmt.Sprintf(`Tool Output: {"status":"success","output":"%s"}`, result.Output)
    }
```

#### Tag 4–5: Tests & Dokumentation

**Test:** `internal/agent/coagent_schema_test.go`

```go
func TestExtractJSONFromText(t *testing.T) {
    tests := []struct {
        name     string
        input    string
        expected string
    }{
        {"json block", "Some text\n```json\n{\"a\":1}\n```\nMore", `{"a":1}`},
        {"bare json", "Result: {\"findings\":[]}", `{"findings":[]}`},
        {"no json", "Just plain text", ""},
    }
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            got := extractJSONFromText(tt.input)
            if tt.expected == "" {
                if got != nil {
                    t.Errorf("expected nil, got %s", string(got))
                }
                return
            }
            if string(got) != tt.expected {
                t.Errorf("expected %s, got %s", tt.expected, string(got))
            }
        })
    }
}

func TestValidateAgainstSchema(t *testing.T) {
    schema := map[string]interface{}{
        "type": "object",
        "properties": map[string]interface{}{
            "findings": map[string]interface{}{
                "type": "array",
                "items": map[string]interface{}{
                    "type": "object",
                    "properties": map[string]interface{}{
                        "file": map[string]interface{}{"type": "string"},
                        "line": map[string]interface{}{"type": "integer"},
                    },
                    "required": []string{"file", "line"},
                },
            },
        },
    }

    valid := []byte(`{"findings":[{"file":"main.go","line":42}]}`)
    if ok, err := validateAgainstSchema(valid, schema); !ok {
        t.Errorf("expected valid, got error: %v", err)
    }

    invalid := []byte(`{"findings":[{"file":"main.go"}]}`) // missing line
    if ok, _ := validateAgainstSchema(invalid, schema); ok {
        t.Error("expected invalid")
    }
}
```

**Tool-Manual:** `prompts/tools_manuals/co_agent.md` aktualisieren mit `output_schema` Beispielen.

---

## Teil B: Python Tool-Reentry (aurago_sdk)

### B.1. Architektur-Überblick

```
┌─────────────────────────────────────────────────────────────┐
│  LLM                                                        │
│   └──> execute_python                                       │
│         code: "import aurago_sdk as aurago\n                │
│                files = aurago.search('func.*hashline')\n   │
│                for f in files:\n                            │
│                    content = aurago.read_file(f)\n          │
│                    aurago.write_file(f, transformed)"       │
│                      ↓                                      │
│              ┌──────────────┐                               │
│              │ Python venv  │                               │
│              │ aurago_sdk   │── HTTP calls to Tool-Bridge   │
│              │  package     │   (localhost, temp token)     │
│              └──────┬───────┘                               │
│                     ↓                                       │
│              ┌──────────────┐                               │
│              │ Tool-Bridge  │── calls native Go tools:      │
│              │ /api/internal│   filesystem, file_editor,    │
│              │ /tool-bridge │   file_search, etc.           │
│              └──────┬───────┘                               │
│                     ↓                                       │
│              Native Go Tool Execution                       │
└─────────────────────────────────────────────────────────────┘
```

### B.2. Motivation

**Aktuelles Problem:**
- `execute_python` kann komplexe Logik ausführen, aber keinen Zugriff auf die Agenten-Werkzeuge
- Ein Skript kann nicht Dateien lesen, suchen, transformieren und schreiben
- Das Modell muss zwischen Python- und Tool-Calls hin-und-her wechseln

**Ziel:**
- Python-Skripte werden zu vollwertigen Werkzeugarbeitern
- Ein Skript kann Bulk-Operationen durchführen (z.B. 50 Dateien transformieren)
- Das Modell schreibt ein Skript, das Skript macht die Arbeit

### B.3. Dateien: Neu / Modifiziert

| Datei | Aktion | Beschreibung |
|-------|--------|--------------|
| `agent_workspace/skills/aurago_sdk/__init__.py` | **Neu** | Python-Paket: read_file, write_file, search, edit_file, list_dir |
| `agent_workspace/skills/aurago_sdk/core.py` | **Neu** | HTTP-Client für Tool-Bridge |
| `agent_workspace/skills/aurago_sdk/setup.py` | **Neu** | Paket-Setup für pip install |
| `internal/tools/python_sdk.go` | **Neu** | Go-Seite: SDK-Installation, Token-Generierung, Bridge-Handler |
| `internal/tools/python.go` | **Modifizieren** | `ExecutePython` um SDK-Auto-Import und Token-Injection erweitern |
| `internal/server/server_routes.go` | **Modifizieren** | Tool-Bridge-Route um Auth-Token-Validierung erweitern |
| `internal/agent/agent_dispatch_exec.go` | **Modifizieren** | `execute_python` um `enable_sdk` Parameter ergänzen |
| `internal/agent/native_tools_execution.go` | **Modifizieren** | `execute_python` Schema um `enable_sdk` erweitern |
| `internal/tools/python_sdk_test.go` | **Neu** | Tests für SDK-Funktionalität |

### B.4. Schritt-für-Schritt-Implementierung

#### Tag 5–7: Python SDK-Paket

**Verzeichnis:** `agent_workspace/skills/aurago_sdk/`

**`__init__.py`:**
```python
"""
AuraGo SDK for Python tool reentry.

Allows Python scripts executed via execute_python to call native AuraGo tools.
Auto-imported when execute_python runs with enable_sdk=true.

Usage:
    import aurago_sdk as aurago
    files = aurago.search("*.go", recursive=True)
    for f in files:
        content = aurago.read_file(f)
        # ... transform ...
        aurago.write_file(f, new_content)
"""

import os
import json
import urllib.request
import urllib.error
from typing import List, Dict, Optional, Any

# These are injected by the Go runtime via environment variables
_BRIDGE_URL = os.environ.get("AURAGO_BRIDGE_URL", "http://localhost:8080/api/internal/tool-bridge")
_BRIDGE_TOKEN = os.environ.get("AURAGO_BRIDGE_TOKEN", "")
_SDK_CALL_LIMIT = int(os.environ.get("AURAGO_SDK_CALL_LIMIT", "10"))

_call_count = 0

def _check_limit():
    global _call_count
    _call_count += 1
    if _call_count > _SDK_CALL_LIMIT:
        raise RuntimeError(
            f"AuraGo SDK call limit exceeded ({_SDK_CALL_LIMIT} calls). "
            "Break your work into smaller batches or use fewer tool calls."
        )

def _call_tool(tool_name: str, params: Dict[str, Any]) -> Dict[str, Any]:
    _check_limit()
    req = urllib.request.Request(
        f"{_BRIDGE_URL}/{tool_name}",
        data=json.dumps(params).encode("utf-8"),
        headers={
            "Content-Type": "application/json",
            "Authorization": f"Bearer {_BRIDGE_TOKEN}",
        },
        method="POST",
    )
    try:
        with urllib.request.urlopen(req, timeout=30) as resp:
            return json.loads(resp.read().decode("utf-8"))
    except urllib.error.HTTPError as e:
        body = e.read().decode("utf-8")
        raise RuntimeError(f"AuraGo tool call failed ({e.code}): {body}")
    except Exception as e:
        raise RuntimeError(f"AuraGo tool call failed: {e}")

def read_file(path: str, offset: int = 0, limit: int = 0) -> str:
    """Read a file from the workspace."""
    result = _call_tool("filesystem", {
        "operation": "read_file",
        "file_path": path,
        "offset": offset,
        "limit": limit,
    })
    if result.get("status") != "success":
        raise RuntimeError(f"read_file failed: {result.get('message')}")
    return result.get("data", {}).get("content", "")

def write_file(path: str, content: str) -> None:
    """Write content to a file in the workspace."""
    result = _call_tool("filesystem", {
        "operation": "write_file",
        "file_path": path,
        "content": content,
    })
    if result.get("status") != "success":
        raise RuntimeError(f"write_file failed: {result.get('message')}")

def search(pattern: str, glob: str = "", recursive: bool = True) -> List[str]:
    """Search for files or text patterns."""
    result = _call_tool("file_search", {
        "operation": "grep_recursive" if recursive else "grep",
        "pattern": pattern,
        "glob": glob,
    })
    if result.get("status") != "success":
        raise RuntimeError(f"search failed: {result.get('message')}")
    # Parse results into list of file paths
    matches = result.get("data", [])
    return [m.get("file", "") for m in matches if isinstance(m, dict)]

def edit_file(path: str, operation: str, **kwargs) -> Dict[str, Any]:
    """Edit a file using the file_editor tool."""
    params = {
        "operation": operation,
        "file_path": path,
    }
    params.update(kwargs)
    result = _call_tool("file_editor", params)
    if result.get("status") != "success":
        raise RuntimeError(f"edit_file failed: {result.get('message')}")
    return result

def list_dir(path: str = ".") -> List[Dict[str, Any]]:
    """List directory contents."""
    result = _call_tool("filesystem", {
        "operation": "list_dir",
        "file_path": path,
    })
    if result.get("status") != "success":
        raise RuntimeError(f"list_dir failed: {result.get('message')}")
    return result.get("data", {}).get("entries", [])

def append_to_file(path: str, content: str) -> None:
    """Append content to a file."""
    result = _call_tool("file_editor", {
        "operation": "append",
        "file_path": path,
        "content": content,
    })
    if result.get("status") != "success":
        raise RuntimeError(f"append_to_file failed: {result.get('message')}")
```

**`setup.py`:**
```python
from setuptools import setup, find_packages

setup(
    name="aurago-sdk",
    version="0.1.0",
    packages=find_packages(),
    description="AuraGo SDK for Python tool reentry",
    python_requires=">=3.10",
)
```

#### Tag 7–8: Go-Integration

**Datei:** `internal/tools/python_sdk.go` (neu)

```go
package tools

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
)

const sdkPackageDir = "aurago_sdk"

// EnsurePythonSDK installs the aurago_sdk package into the virtual environment.
func EnsurePythonSDK(workspaceDir string) error {
	sdkPath := filepath.Join(workspaceDir, "skills", sdkPackageDir)
	if _, err := os.Stat(sdkPath); os.IsNotExist(err) {
		return fmt.Errorf("aurago_sdk not found at %s — package must be bundled with AuraGo", sdkPath)
	}

	pipCmd := GetPipBin(workspaceDir)
	cmd := exec.Command(pipCmd, "install", "-e", sdkPath)
	cmd.Dir = getAbsWorkspace(workspaceDir)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to install aurago_sdk: %w (output: %s)", err, string(out))
	}
	return nil
}

// GenerateBridgeToken creates a cryptographically secure temporary token for the tool bridge.
func GenerateBridgeToken() string {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		// Fallback (should never happen in practice)
		return fmt.Sprintf("fallback-%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(b)
}

// PythonSDKEnv returns environment variables to inject into the Python process.
func PythonSDKEnv(bridgeURL, token string, callLimit int) map[string]string {
	return map[string]string{
		"AURAGO_BRIDGE_URL":      bridgeURL,
		"AURAGO_BRIDGE_TOKEN":    token,
		"AURAGO_SDK_CALL_LIMIT":  fmt.Sprintf("%d", callLimit),
		"PYTHONPATH":             filepath.Join("skills", sdkPackageDir), // ensure importable
	}
}
```

**Datei:** `internal/tools/python.go` — `ExecutePython` erweitern:

```go
// ExecutePythonOptions configures Python execution.
type ExecutePythonOptions struct {
	Code        string
	WorkspaceDir string
	ToolsDir     string
	Secrets      map[string]string
	Credentials  []CredentialFields
	EnableSDK    bool           // NEW
	BridgeURL    string         // NEW
	CallLimit    int            // NEW (default: 10)
}

func ExecutePythonWithOpts(opts ExecutePythonOptions) (string, string, error) {
    // ... existing setup ...
    
    cmd := exec.Command(pythonCmd, "-c", opts.Code)
    cmd.Dir = getAbsWorkspace(opts.WorkspaceDir)
    SetupCmd(cmd)
    
    // Inject secrets/credentials
    InjectSecretsEnv(cmd, opts.Secrets)
    InjectCredentialEnv(cmd, opts.Credentials)
    
    // NEW: SDK integration
    if opts.EnableSDK {
        // Ensure SDK is installed
        if err := EnsurePythonSDK(opts.WorkspaceDir); err != nil {
            return "", "", fmt.Errorf("SDK setup failed: %w", err)
        }
        
        // Generate token and inject env
        token := GenerateBridgeToken()
        limit := opts.CallLimit
        if limit <= 0 { limit = 10 }
        
        env := PythonSDKEnv(opts.BridgeURL, token, limit)
        for k, v := range env {
            cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", k, v))
        }
        
        // Prepend auto-import to code
        opts.Code = "import aurago_sdk as aurago\n" + opts.Code
    }
    
    // ... run cmd ...
}
```

**Datei:** `internal/server/server_routes.go` — Tool-Bridge Auth:

Die Tool-Bridge existiert bereits unter `/api/internal/tool-bridge/`. Sie muss um Token-Validierung erweitert werden.

```go
// In the tool-bridge handler:
func handleToolBridge(w http.ResponseWriter, r *http.Request) {
    // Validate Bearer token
    authHeader := r.Header.Get("Authorization")
    if !strings.HasPrefix(authHeader, "Bearer ") {
        http.Error(w, `{"status":"error","message":"missing authorization"}`, http.StatusUnauthorized)
        return
    }
    token := strings.TrimPrefix(authHeader, "Bearer ")
    if !bridgeTokenStore.Validate(token) {
        http.Error(w, `{"status":"error","message":"invalid or expired token"}`, http.StatusForbidden)
        return
    }
    
    // ... existing tool dispatch logic ...
}
```

**Neu:** `internal/tools/bridge_token_store.go` (oder inline in `python_sdk.go`):

```go
package tools

import (
	"sync"
	"time"
)

type BridgeTokenStore struct {
	mu     sync.RWMutex
	tokens map[string]time.Time
}

func NewBridgeTokenStore() *BridgeTokenStore {
	return &BridgeTokenStore{tokens: make(map[string]time.Time)}
}

func (s *BridgeTokenStore) Register(token string, ttl time.Duration) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.tokens[token] = time.Now().Add(ttl)
	// Cleanup expired tokens periodically (simplified: do on every Nth register)
}

func (s *BridgeTokenStore) Validate(token string) bool {
	s.mu.RLock()
	expiry, ok := s.tokens[token]
	s.mu.RUnlock()
	if !ok || time.Now().After(expiry) {
		return false
	}
	return true
}

func (s *BridgeTokenStore) Revoke(token string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.tokens, token)
}
```

#### Tag 8–9: Schema-Updates & Dispatch

**Datei:** `internal/agent/native_tools_execution.go`

`execute_python` Schema:
```go
tool("execute_python",
    "Execute Python code in an isolated virtual environment. "+
        "Set enable_sdk=true to import the aurago_sdk package, which lets the script call native tools like read_file, write_file, search, and edit_file.",
    schema(map[string]interface{}{
        "code": map[string]interface{}{
            "type":        "string",
            "description": "Python source code to execute",
        },
        "background": map[string]interface{}{
            "type":        "boolean",
            "description": "Run as background process",
        },
        "vault_keys": map[string]interface{}{
            "type":        "array",
            "items":       map[string]interface{}{"type": "string"},
            "description": "Vault secret keys to inject",
        },
        // NEW
        "enable_sdk": map[string]interface{}{
            "type":        "boolean",
            "description": "If true, injects the aurago_sdk package and allows the Python script to call native AuraGo tools (read_file, write_file, search, edit_file, list_dir). Limited to 10 tool calls per script execution.",
        },
        "sdk_call_limit": map[string]interface{}{
            "type":        "integer",
            "description": "Maximum number of tool calls the SDK can make (default: 10, max: 50).",
        },
    }, "code"),
)
```

**Datei:** `internal/agent/dispatch_python.go`

```go
case "execute_python":
    req := decodePythonExecutionArgs(tc)
    // ... existing validation ...
    
    // Parse SDK options
    enableSDK := false
    if v, ok := tc.Params["enable_sdk"].(bool); ok {
        enableSDK = v
    }
    callLimit := 10
    if v, ok := tc.Params["sdk_call_limit"].(float64); ok {
        callLimit = int(v)
        if callLimit > 50 { callLimit = 50 }
        if callLimit < 1 { callLimit = 10 }
    }
    
    var bridgeURL string
    if enableSDK {
        // Determine bridge URL from server config
        port := cfg.Server.Port
        if port == 0 { port = 8080 }
        bridgeURL = fmt.Sprintf("http://localhost:%d/api/internal/tool-bridge", port)
    }
    
    if req.Background {
        // ... background execution with SDK support ...
    }
    
    opts := tools.ExecutePythonOptions{
        Code:         req.Code,
        WorkspaceDir: cfg.Directories.WorkspaceDir,
        ToolsDir:     cfg.Directories.ToolsDir,
        Secrets:      secrets,
        Credentials:  creds,
        EnableSDK:    enableSDK,
        BridgeURL:    bridgeURL,
        CallLimit:    callLimit,
    }
    stdout, stderr, pyErr := tools.ExecutePythonWithOpts(opts)
    // ... rest of output handling ...
```

#### Tag 9–10: Tests & Dokumentation

**Test:** `internal/tools/python_sdk_test.go`

```go
package tools

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGenerateBridgeToken(t *testing.T) {
	t1 := GenerateBridgeToken()
	t2 := GenerateBridgeToken()
	if t1 == t2 {
		t.Error("tokens should be unique")
	}
	if len(t1) < 32 {
		t.Errorf("token too short: %d chars", len(t1))
	}
}

func TestBridgeTokenStore(t *testing.T) {
	store := NewBridgeTokenStore()
	token := "test-token-123"
	
	store.Register(token, time.Hour)
	if !store.Validate(token) {
		t.Error("expected token to be valid")
	}
	
	store.Revoke(token)
	if store.Validate(token) {
		t.Error("expected token to be revoked")
	}
}

func TestEnsurePythonSDK(t *testing.T) {
	tmp := t.TempDir()
	// Create mock SDK structure
	sdkDir := filepath.Join(tmp, "skills", "aurago_sdk")
	os.MkdirAll(sdkDir, 0755)
	os.WriteFile(filepath.Join(sdkDir, "__init__.py"), []byte("# mock"), 0644)
	os.WriteFile(filepath.Join(sdkDir, "setup.py"), []byte("from setuptools import setup\nsetup()"), 0644)
	
	// This would need a real venv to fully test; skip if no venv
	if _, err := os.Stat(GetPipBin(tmp)); os.IsNotExist(err) {
		t.Skip("no venv available for testing")
	}
	
	// Test that it attempts install
	err := EnsurePythonSDK(tmp)
	// May fail due to mock setup.py, but should not panic
	_ = err
}
```

**Tool-Manual:** `prompts/tools_manuals/execute_python.md` (neu oder aktualisieren):

```markdown
## Python SDK (Tool Reentry)

When `enable_sdk` is set to `true`, your Python script can call native AuraGo tools directly:

```python
import aurago_sdk as aurago

# Read files
content = aurago.read_file("internal/agent/agent_loop.go")

# Search across files
files = aurago.search("func.*SpawnCoAgent", glob="*.go")

# Batch edit
for f in files:
    content = aurago.read_file(f)
    new_content = content.replace("old", "new")
    aurago.write_file(f, new_content)

# List directory
entries = aurago.list_dir("internal/tools")
```

**Limitations:**
- Maximum 10 tool calls per script (configurable via `sdk_call_limit`, max 50)
- SDK calls are HTTP requests to the internal tool bridge — keep scripts idempotent
- Secrets are NOT automatically forwarded to SDK calls; use vault_keys if needed
```

---

## 5. Rückwärtskompatibilität

| Aspekt | Strategie |
|--------|-----------|
| Co-Agent ohne Schema | Bleibt unverändert. `output_schema` ist optional. |
| Python ohne SDK | Bleibt unverändert. `enable_sdk` defaultet auf `false`. |
| Tool-Bridge | Bestehende interne Skills funktionieren weiter (ohne Token-Check oder Token wird optional). |
| Prompts | Alte Prompts funktionieren, da keine neuen Pflichtparameter hinzugekommen sind. |

---

## 6. Risiken & Mitigationen

| Risiko | Wahrscheinlichkeit | Mitigation |
|--------|-------------------|------------|
| LLM-Extraktion für Schema kostet zusätzliche Tokens | Hoch | Verwende günstiges Modell (gpt-4o-mini); Extraktion nur bei `output_schema` |
| Python-SDK ermöglicht unendliche Rekursion | Mittel | Hartes Call-Limit (10 default, 50 max); Token-Expiry nach Script-Ende |
| Tool-Bridge-Token wird geleakt | Niedrig | 256-Bit-CSPRNG-Token; TTL auf Script-Dauer begrenzt; Token nach Beendigung revoken |
| SDK-Paket fehlt im Deployment | Niedrig | `EnsurePythonSDK` gibt expliziten Fehler; Paket wird mit AuraGo gebündelt (go:embed) |
| Schema-Validierung mit gojsonschema langsam | Sehr niedrig | Schemas sind typischerweise klein (<1KB); Validierung <1ms |

---

## 7. Erfolgsmetriken

### Schema-valide Co-Agents
- **Reduktion von** Parsing-Fehlern bei Co-Agent-Ergebnissen um ≥50 %
- **Token-Einsparung:** Parent-Agent braucht weniger Follow-up-Fragen, weil Daten strukturiert sind
- **Erfolgsquote:** ≥80 % der Co-Agent-Runs mit `output_schema` liefern validiertes JSON

### Tool-Reentry
- **Anzahl der** `execute_python`-Aufrufe mit `enable_sdk=true` (Adoption-Metrik)
- **Reduktion von** Agent-Loop-Iterationen bei Bulk-Operationen (z.B. Rename über 20 Dateien in einem Python-Skript statt 20 einzelnen Tool-Calls)
- **Fehlerrate:** SDK-Tool-Calls mit <5 % Fehlern

---

## 8. Abhängigkeiten zwischen Teil A und Teil B

Die beiden Teile sind **weitgehend unabhängig** und können parallel entwickelt werden.

| Abhängigkeit | Beschreibung |
|-------------|--------------|
| `llmClient` in Co-Agent | Für Teil A muss ein LLM-Client an die Co-Agent-Goroutine durchgereicht werden. Das ist bereits über `DispatchContext` gegeben. |
| `BridgeTokenStore` | Für Teil B wird ein globaler Token-Store benötigt. Dieser sollte als Singleton im `DispatchContext` oder als Package-Variable leben. |
| Gemeinsame Änderung an `agent_dispatch_exec.go` | Beide Teile erweitern den Dispatch — konfliktfrei, da verschiedene `case`-Blöcke. |

---

*Plan erstellt: 2026-06-07*
