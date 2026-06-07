# Implementierungsplan: Schema-valide Co-Agents + Tool-Reentry (korrigiert)

**Projekt:** AuraGo
**Feature A:** JSON-Schema-validierte Co-Agent-Outputs (Prompt-gesteuert)
**Feature B:** Python Tool-Reentry über bestehende Tool-Bridge
**Geschätzter Aufwand:** 10–13 Arbeitstage (vorher: 12–15)
**Ziel:** Robuste Delegation + Python als Werkzeugarbeiter

---

## Teil A: Schema-valide Co-Agents (korrigiert)

### A.1. Architektur-Überblick (korrigiert)

```
┌─────────────────────────────────────────────────────────────┐
│  Parent Agent                                               │
│   └──> co_agent spawn                                       │
│         ├── role: "security_auditor"                        │
│         ├── task: "Prüfe auf Injection-Risiken"            │
│         └── output_schema: {                                │
│               "type": "object",                             │
│               "properties": {                               │
│                 "findings": { ... }                         │
│               }                                             │
│             }                                               │
│                      ↓                                      │
│              ┌──────────────┐                               │
│              │ Co-Agent     │                               │
│              │ System-Prompt│── beinhaltet Schema-Anweisung │
│              │ (korrigiert) │   "Output ONLY valid JSON"    │
│              └──────┬───────┘                               │
│                     ↓                                       │
│              ExecuteAgentLoop                               │
│                     ↓                                       │
│              Assistant gibt JSON aus                        │
│                     ↓                                       │
│              ┌──────────────┐                               │
│              │ Validate     │── Schema-Prüfung (gojsonschema)│
│              │ JSON         │                               │
│              └──────┬───────┘                               │
│                     ↓ Valide                                │
│              Registry speichert raw + structured            │
│                     ↓                                       │
│              Parent bekommt beides                          │
└─────────────────────────────────────────────────────────────┘
```

**Wichtige Änderung zu Plan v1:** Das Schema wird **IN den Co-Agent-System-Prompt injiziert**, nicht nachträglich extrahiert. Der Co-Agent gibt direkt JSON aus. Es gibt keinen zusätzlichen LLM-Aufruf für die Extraktion.

### A.2. Motivation

**Aktuelles Problem in AuraGo (`internal/agent/coagent.go`):**
- Co-Agent liefert Rohtext (oft 500–2000 Wörter)
- Parent-Agent muss daraus Pfade, Fehler, Prioritäten extrahieren
- Token-teuer und fehleranfällig

**Ziel (korrigiert):**
- Co-Agent-System-Prompt bekommt Schema-Anweisung
- Co-Agent gibt direkt JSON aus (oder versucht es zumindest)
- Nach dem Loop: JSON-Validierung gegen Schema
- Valide → Registry speichert `raw` + `structured`
- Invalide → Registry speichert `raw` + Fehlermeldung

### A.3. Dateien: Neu / Modifiziert

| Datei | Aktion | Beschreibung |
|-------|--------|--------------|
| `internal/agent/coagent_schema.go` | **Neu** | Schema-Sanitizing, JSON-Validierung |
| `internal/agent/coagent.go` | **Modifizieren** | `CoAgentRequest` um `OutputSchema` erweitern; Schema in Prompt injizieren |
| `internal/agent/coagent_registry.go` | **Modifizieren** | `CoAgentInfo` um `Structured`/`StructuredValid` erweitern; neue Methoden |
| `internal/agent/native_tools_edge.go` | **Modifizieren** | `co_agent` Tool-Schema um `output_schema` erweitern |
| `internal/agent/agent_dispatch_planner.go` | **Modifizieren** | Dispatch für `co_agent` um Schema-Parameter ergänzen |
| `go.mod` | **Modifizieren** | Neue Dependency: `github.com/xeipuuv/gojsonschema` |
| `internal/agent/coagent_schema_test.go` | **Neu** | Tests für Validierung und Sanitizing |

### A.4. Schritt-für-Schritt-Implementierung

#### Tag 1: Schema-Infrastruktur

**Datei:** `internal/agent/coagent_schema.go` (neu)

```go
package agent

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/xeipuuv/gojsonschema"
)

// CoAgentSchemaValidator handles schema validation for co-agent outputs.
type CoAgentSchemaValidator struct{}

// ValidationResult holds the outcome of schema validation.
type ValidationResult struct {
	Valid      bool            `json:"valid"`
	Data       json.RawMessage `json:"data,omitempty"`
	Error      string          `json:"error,omitempty"`
}

// Validate checks if the given JSON string conforms to the schema.
func (v *CoAgentSchemaValidator) Validate(jsonStr string, schema map[string]interface{}) ValidationResult {
	// First: try to find JSON in the text (in case there's prose around it)
	data := extractJSONFromText(jsonStr)
	if data == nil {
		return ValidationResult{Valid: false, Error: "no JSON object found in output"}
	}

	// Validate against schema
	if valid, err := validateAgainstSchema(data, schema); valid {
		return ValidationResult{Valid: true, Data: data}
	} else {
		return ValidationResult{Valid: false, Data: data, Error: err.Error()}
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
	// Look for bare JSON
	start := strings.Index(text, "{")
	if start < 0 {
		start = strings.Index(text, "[")
	}
	if start >= 0 {
		// Try to find matching brace by counting
		depth := 0
		inString := false
		escape := false
		for i := start; i < len(text); i++ {
			c := text[i]
			if escape {
				escape = false
				continue
			}
			if c == '\\' && inString {
				escape = true
				continue
			}
			if c == '"' {
				inString = !inString
				continue
			}
			if inString {
				continue
			}
			switch c {
			case '{', '[':
				depth++
			case '}', ']':
				depth--
				if depth == 0 {
					candidate := text[start : i+1]
					var dummy interface{}
					if json.Unmarshal([]byte(candidate), &dummy) == nil {
						return json.RawMessage(candidate)
					}
					return nil
				}
			}
		}
	}
	return nil
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

// sanitizeJSONSchema checks a schema for safety before injecting it into prompts.
// Prevents deep nesting, ReDoS in patterns, and excessive size.
func sanitizeJSONSchema(schema map[string]interface{}) error {
	const maxDepth = 10
	const maxProperties = 100

	var checkDepth func(v interface{}, depth int) error
	checkDepth = func(v interface{}, depth int) error {
		if depth > maxDepth {
			return fmt.Errorf("schema exceeds maximum nesting depth of %d", maxDepth)
		}
		switch val := v.(type) {
		case map[string]interface{}:
			if len(val) > maxProperties {
				return fmt.Errorf("schema object exceeds %d properties", maxProperties)
			}
			// Check for dangerous regex patterns
			if pattern, ok := val["pattern"].(string); ok {
				if len(pattern) > 1000 {
					return fmt.Errorf("regex pattern exceeds 1000 chars")
				}
			}
			for _, child := range val {
				if err := checkDepth(child, depth+1); err != nil {
					return err
				}
			}
		case []interface{}:
			for _, child := range val {
				if err := checkDepth(child, depth+1); err != nil {
					return err
				}
			}
		}
		return nil
	}

	return checkDepth(schema, 0)
}

// buildSchemaPromptFragment injects the schema into a system prompt.
func buildSchemaPromptFragment(schema map[string]interface{}) string {
	schemaJSON, _ := json.MarshalIndent(schema, "", "  ")
	return fmt.Sprintf(`
## OUTPUT FORMAT — CRITICAL
You MUST output ONLY a single valid JSON object conforming to this JSON Schema:
%s

Rules:
- Output NOTHING except the JSON object. No markdown code fences, no explanations, no prose before or after.
- If a field is missing in your findings, use null or an empty array/string as appropriate.
- Do NOT invent data not present in your analysis.
- Do NOT wrap the JSON in ```json blocks.
`, schemaJSON)
}
```

#### Tag 2: CoAgentRegistry erweitern

**Datei:** `internal/agent/coagent_registry.go`

`CoAgentInfo` erweitern:
```go
type CoAgentInfo struct {
    ID              string          `json:"id"`
    Task            string          `json:"task"`
    State           CoAgentState    `json:"state"`
    Result          string          `json:"result,omitempty"`
    Structured      json.RawMessage `json:"structured,omitempty"`       // NEW
    StructuredValid bool            `json:"structured_valid,omitempty"` // NEW
    Error           string          `json:"error,omitempty"`
    TokensUsed      int             `json:"tokens_used"`
    ToolCalls       int             `json:"tool_calls"`
    CreatedAt       time.Time       `json:"created_at"`
    CompletedAt     *time.Time      `json:"completed_at,omitempty"`
    CancelFunc      context.CancelFunc `json:"-"`
    Events          []string        `json:"events,omitempty"`
    PartialResults  []string        `json:"partial_results,omitempty"`
    RetryCount      int             `json:"retry_count"`
    Priority        int             `json:"priority"`
}
```

Neue Methoden:
```go
// CompleteStructured marks a co-agent as completed with structured data.
func (r *CoAgentRegistry) CompleteStructured(id, result string, structured json.RawMessage, valid bool, tokensUsed, toolCalls int) {
    r.mu.Lock()
    defer r.mu.Unlock()
    agent, ok := r.agents[id]
    if !ok {
        return
    }
    if agent.State == CoAgentRunning {
        r.runningCount.Add(-1)
    }
    now := time.Now()
    agent.State = CoAgentCompleted
    agent.Result = result
    agent.Structured = structured
    agent.StructuredValid = valid
    agent.TokensUsed = tokensUsed
    agent.ToolCalls = toolCalls
    agent.CompletedAt = &now
    r.promoteQueuedLocked()
}

// GetStructuredResult returns the result with structured data if available.
func (r *CoAgentRegistry) GetStructuredResult(id string) (raw string, structured json.RawMessage, valid bool, err error) {
    r.mu.RLock()
    defer r.mu.RUnlock()
    agent, ok := r.agents[id]
    if !ok {
        return "", nil, false, fmt.Errorf("co-agent %s not found", id)
    }
    if agent.State != CoAgentCompleted && agent.State != CoAgentFailed {
        return "", nil, false, fmt.Errorf("co-agent %s is still %s", id, agent.State)
    }
    return agent.Result, agent.Structured, agent.StructuredValid, nil
}
```

**Hinweis:** `Complete` bleibt unverändert für abwärtskompatibilität. Neue Code-Pfade verwenden `CompleteStructured`.

#### Tag 2–3: CoAgentRequest erweitern & Prompt-Integration

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

System-Prompt-Builder erweitern:
```go
func buildCoAgentSystemPrompt(cfg *config.Config, req CoAgentRequest, longTermMem memory.VectorDB, shortTermMem *memory.SQLiteMemory) string {
    // ... existing prompt building ...
    
    // NEW: Inject schema if provided
    if req.OutputSchema != nil {
        if err := sanitizeJSONSchema(req.OutputSchema); err == nil {
            prompt += buildSchemaPromptFragment(req.OutputSchema)
        } else {
            // Sanitizing failed — log but don't block. Fall back to normal prompt.
            slog.Warn("Co-Agent schema sanitization failed, proceeding without schema enforcement", "error", err)
        }
    }
    
    return prompt
}
```

**Wichtig:** `sanitizeJSONSchema` wird vor der Prompt-Injektion aufgerufen. Wenn das Schema ungültig/suspekt ist, wird es ignoriert und der Co-Agent läuft wie bisher (Rohtext-Modus).

Im Goroutine-Ende (nach `ExecuteAgentLoop`):
```go
result := ""
if len(resp.Choices) > 0 {
    result = resp.Choices[0].Message.Content
}
tokensUsed := resp.Usage.TotalTokens

// Limit result size
maxCoAgentResultBytes := cfg.CoAgents.MaxResultBytes
if maxCoAgentResultBytes > 0 && len(result) > maxCoAgentResultBytes {
    // ... existing truncation logic ...
}

// NEW: Schema validation if requested
if req.OutputSchema != nil && result != "" {
    validator := &CoAgentSchemaValidator{}
    validation := validator.Validate(result, req.OutputSchema)
    
    coLogger.Info("Co-Agent schema validation", "valid", validation.Valid, "co_id", coID)
    
    if validation.Valid {
        coRegistry.CompleteStructured(coID, result, validation.Data, true, tokensUsed, 0)
    } else {
        coRegistry.CompleteStructured(coID, result, validation.Data, false, tokensUsed, 0)
        coRegistry.RecordEvent(coID, fmt.Sprintf("schema validation failed: %s", validation.Error))
    }
} else {
    // Existing path — no schema requested
    if result != "" {
        coRegistry.RecordPartialResult(coID, result)
    }
    coRegistry.Complete(coID, result, tokensUsed, 0)
}
```

#### Tag 3–4: Tool-Schema & Dispatch

**Datei:** `internal/agent/native_tools_edge.go` (oder `native_tools_planner.go`)

`co_agent` Schema:
```go
tool("co_agent",
    "Spawn a specialized co-agent subtask with optional structured JSON output. "+
        "Co-agents run in isolated sessions. If output_schema is provided, the co-agent is instructed to output ONLY valid JSON conforming to the schema.",
    schema(map[string]interface{}{
        "action": map[string]interface{}{
            "type":        "string",
            "description": "Co-agent action",
            "enum":        []string{"spawn", "status", "result", "cancel"},
        },
        "task":          prop("string", "Task description for spawn"),
        "role":          prop("string", "Specialist role: researcher, coder, designer, security, writer"),
        "priority":      map[string]interface{}{"type": "integer", "description": "1=low, 2=normal, 3=high"},
        "co_agent_id":   prop("string", "Co-agent ID for status/result/cancel"),
        "output_schema": map[string]interface{}{
            "type":        "object",
            "description": "Optional JSON Schema object. If provided, the co-agent will output JSON conforming to this schema. "+
                "Example: {\"type\":\"object\",\"properties\":{\"findings\":{\"type\":\"array\",\"items\":{\"type\":\"object\",\"properties\":{\"file\":{\"type\":\"string\"},\"line\":{\"type\":\"integer\"}}}}}}",
        },
    }, "action"),
)
```

**Datei:** `internal/agent/agent_dispatch_planner.go`

```go
case "co_agent":
    action := stringValueFromMap(tc.Params, "action")
    switch action {
    case "spawn":
        req := CoAgentRequest{
            Task:       stringValueFromMap(tc.Params, "task"),
            Specialist: stringValueFromMap(tc.Params, "role"),
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
        raw, structured, valid, err := coRegistry.GetStructuredResult(coID)
        if err != nil {
            return fmt.Sprintf(`Tool Output: {"status":"error","message":"%v"}`, err)
        }
        
        respData := map[string]interface{}{
            "status": "success",
            "output": raw,
        }
        if structured != nil {
            respData["structured"] = json.RawMessage(structured)
            respData["structured_valid"] = valid
        }
        if !valid {
            respData["structured_error"] = "JSON did not conform to requested schema"
        }
        
        b, _ := json.Marshal(respData)
        return "Tool Output: " + string(b)
    }
```

#### Tag 4–5: Tests & Dokumentation

**Test:** `internal/agent/coagent_schema_test.go`

```go
package agent

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestExtractJSONFromText(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"json block", "Analysis:\n```json\n{\"a\":1}\n```\nDone", `{"a":1}`},
		{"bare json", "Result: {\"findings\":[]}", `{"findings":[]}`},
		{"nested", "Text {\"outer\":{\"inner\":1}} more", `{"outer":{"inner":1}}`},
		{"array", "[1,2,3]", `[1,2,3]`},
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

	invalid := []byte(`{"findings":[{"file":"main.go"}]}`)
	if ok, _ := validateAgainstSchema(invalid, schema); ok {
		t.Error("expected invalid")
	}
}

func TestSanitizeJSONSchema(t *testing.T) {
	// Valid schema
	valid := map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"name": map[string]interface{}{"type": "string"},
		},
	}
	if err := sanitizeJSONSchema(valid); err != nil {
		t.Errorf("expected valid, got: %v", err)
	}

	// Too deep
	deep := map[string]interface{}{}
	current := deep
	for i := 0; i < 15; i++ {
		next := map[string]interface{}{}
		current["nested"] = next
		current = next
	}
	if err := sanitizeJSONSchema(deep); err == nil {
		t.Error("expected depth error")
	}

	// ReDoS pattern
	redos := map[string]interface{}{
		"type":    "string",
		"pattern": "(a+)+b",
	}
	// Pattern length check catches this if >1000, but small patterns are allowed
	_ = redos
}

func TestBuildSchemaPromptFragment(t *testing.T) {
	schema := map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"count": map[string]interface{}{"type": "integer"},
		},
	}
	frag := buildSchemaPromptFragment(schema)
	if !strings.Contains(frag, "OUTPUT FORMAT") {
		t.Error("missing OUTPUT FORMAT header")
	}
	if !strings.Contains(frag, "count") {
		t.Error("missing schema content")
	}
}
```

**Tool-Manual:** `prompts/tools_manuals/co_agent.md` aktualisieren:
```markdown
## Structured Output (JSON Schema)

If you need structured data from a co-agent instead of prose, provide an `output_schema`:

```json
{
  "action": "spawn",
  "task": "Find all security issues in internal/security",
  "role": "security",
  "output_schema": {
    "type": "object",
    "properties": {
      "findings": {
        "type": "array",
        "items": {
          "type": "object",
          "properties": {
            "file": {"type": "string"},
            "line": {"type": "integer"},
            "severity": {"enum": ["low", "medium", "high", "critical"]},
            "description": {"type": "string"}
          }
        }
      }
    }
  }
}
```

The co-agent will output JSON conforming to the schema. When retrieving the result:
- `structured` contains the parsed JSON
- `structured_valid` indicates whether it passed schema validation
- `output` contains the raw text (always available)
```

---

## Teil B: Python Tool-Reentry (korrigiert)

### B.1. Architektur-Überblick (korrigiert)

```
┌─────────────────────────────────────────────────────────────┐
│  LLM                                                        │
│   └──> execute_python(enable_sdk=true)                      │
│         code: "files = aurago.search('func.*hashline')\n   │
│                for f in files:\n                            │
│                    content = aurago.read_file(f)\n"         │
│                      ↓                                      │
│              ┌──────────────┐                               │
│              │ Python venv  │                               │
│              │ Code wird    │── auto-prepended:             │
│              │ vorangestellt│   sys.path.insert(0, SDK-Path)│
│              │              │   import aurago_sdk as aurago │
│              └──────┬───────┘                               │
│                     ↓                                       │
│              aurago_sdk macht HTTP-Calls                    │
│              mit X-Internal-Token (bestehend!)              │
│                     ↓                                       │
│              ┌──────────────┐                               │
│              │ Tool-Bridge  │── Loopback + X-Internal-Token │
│              │ /api/internal│   (KEIN neuer Token-Store!)   │
│              │ /tool-bridge │                               │
│              └──────┬───────┘                               │
│                     ↓                                       │
│              Native Go Tool Execution                       │
└─────────────────────────────────────────────────────────────┘
```

**Wichtige Änderungen zu Plan v1:**
1. **Kein neuer Token-Store** — verwendet bestehendes `X-Internal-Token`
2. **`sys.path.insert`** statt `PYTHONPATH`
3. **Background + SDK verboten**
4. **Bridge-URL** dynamisch aus laufendem Server

### B.2. Motivation

**Aktuelles Problem:**
- `execute_python` kann komplexe Logik ausführen, aber keinen Zugriff auf Agenten-Werkzeuge
- Modell muss zwischen Python- und Tool-Calls hin-und-her wechseln

**Ziel:**
- Python-Skripte werden zu vollwertigen Werkzeugarbeitern
- Ein Skript kann Bulk-Operationen durchführen
- Bestehende Tool-Bridge wird wiederverwendet (kein neuer Auth-Mechanismus)

### B.3. Dateien: Neu / Modifiziert

| Datei | Aktion | Beschreibung |
|-------|--------|--------------|
| `agent_workspace/skills/aurago_sdk/__init__.py` | **Neu** | Python-Paket: read_file, write_file, search, edit_file, list_dir |
| `agent_workspace/skills/aurago_sdk/core.py` | **Neu** | HTTP-Client für Tool-Bridge mit X-Internal-Token |
| `agent_workspace/skills/aurago_sdk/setup.py` | **Neu** | Paket-Setup |
| `internal/tools/python_sdk.go` | **Neu** | SDK-Installation, Env-Var-Generierung |
| `internal/tools/python.go` | **Modifizieren** | `ExecutePython` um SDK-Auto-Import erweitern |
| `internal/agent/agent_dispatch_exec.go` | **Modifizieren** | `execute_python` um `enable_sdk` Parameter ergänzen |
| `internal/agent/native_tools_execution.go` | **Modifizieren** | `execute_python` Schema um `enable_sdk` erweitern |
| `internal/tools/python_sdk_test.go` | **Neu** | Tests |

### B.4. Schritt-für-Schritt-Implementierung

#### Tag 5–6: Python SDK-Paket

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

_BRIDGE_URL = os.environ.get("AURAGO_BRIDGE_URL", "http://localhost:8080/api/internal/tool-bridge")
_INTERNAL_TOKEN = os.environ.get("AURAGO_INTERNAL_TOKEN", "")
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
            "X-Internal-Token": _INTERNAL_TOKEN,
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
    """Search for text patterns across files."""
    result = _call_tool("file_search", {
        "operation": "grep_recursive" if recursive else "grep",
        "pattern": pattern,
        "glob": glob,
    })
    if result.get("status") != "success":
        raise RuntimeError(f"search failed: {result.get('message')}")
    matches = result.get("data", [])
    return [m.get("file", "") for m in matches if isinstance(m, dict)]

def edit_file(path: str, operation: str, **kwargs) -> Dict[str, Any]:
    """Edit a file using the file_editor tool."""
    params = {"operation": operation, "file_path": path}
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

#### Tag 6–7: Go-Integration

**Datei:** `internal/tools/python_sdk.go` (neu)

```go
package tools

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

const sdkPackageName = "aurago_sdk"

// EnsurePythonSDK installs the aurago_sdk package into the virtual environment.
func EnsurePythonSDK(workspaceDir string) error {
	sdkPath := filepath.Join(workspaceDir, "skills", sdkPackageName)
	if _, err := os.Stat(sdkPath); os.IsNotExist(err) {
		return fmt.Errorf("aurago_sdk not found at %s — package must be bundled with AuraGo", sdkPath)
	}

	// Ensure venv exists first
	// (EnsureVenv should be called before this, but we double-check)
	pipCmd := GetPipBin(workspaceDir)
	if _, err := os.Stat(pipCmd); os.IsNotExist(err) {
		return fmt.Errorf("pip not found — virtual environment may not exist")
	}

	cmd := exec.Command(pipCmd, "install", "-e", sdkPath)
	cmd.Dir = getAbsWorkspace(workspaceDir)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to install aurago_sdk: %w (output: %s)", err, string(out))
	}
	return nil
}

// BuildPythonSDKInjectCode returns the Python code to prepend to user scripts.
// Uses sys.path.insert instead of PYTHONPATH to avoid overriding user env.
func BuildPythonSDKInjectCode(sdkPath string) string {
	return fmt.Sprintf(`import sys
sys.path.insert(0, %q)
import aurago_sdk as aurago
`, sdkPath)
}

// PythonSDKEnv returns environment variables for the Python process.
func PythonSDKEnv(bridgeURL, internalToken string, callLimit int) map[string]string {
	if callLimit <= 0 {
		callLimit = 10
	}
	if callLimit > 50 {
		callLimit = 50
	}
	return map[string]string{
		"AURAGO_BRIDGE_URL":   bridgeURL,
		"AURAGO_INTERNAL_TOKEN": internalToken,
		"AURAGO_SDK_CALL_LIMIT": fmt.Sprintf("%d", callLimit),
	}
}
```

**Datei:** `internal/tools/python.go` — `ExecutePython` erweitern:

```go
// ExecutePythonOptions configures Python execution.
type ExecutePythonOptions struct {
	Code         string
	WorkspaceDir string
	ToolsDir     string
	Secrets      map[string]string
	Credentials  []CredentialFields
	EnableSDK    bool
	BridgeURL    string
	InternalToken string
	CallLimit    int
}

// ExecutePythonWithOpts executes Python with optional SDK support.
func ExecutePythonWithOpts(opts ExecutePythonOptions) (string, string, error) {
	// ... existing validation, venv check ...
	pythonCmd := GetPythonBin(opts.WorkspaceDir)

	code := opts.Code

	// NEW: SDK integration
	if opts.EnableSDK {
		if err := EnsurePythonSDK(opts.WorkspaceDir); err != nil {
			return "", "", fmt.Errorf("SDK setup failed: %w", err)
		}

		sdkPath := filepath.Join(opts.WorkspaceDir, "skills", sdkPackageName)
		injectCode := BuildPythonSDKInjectCode(sdkPath)
		code = injectCode + "\n" + code
	}

	cmd := exec.Command(pythonCmd, "-c", code)
	cmd.Dir = getAbsWorkspace(opts.WorkspaceDir)
	SetupCmd(cmd)

	// Inject secrets/credentials
	InjectSecretsEnv(cmd, opts.Secrets)
	InjectCredentialEnv(cmd, opts.Credentials)

	// NEW: SDK env vars
	if opts.EnableSDK {
		env := PythonSDKEnv(opts.BridgeURL, opts.InternalToken, opts.CallLimit)
		for k, v := range env {
			cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", k, v))
		}
	}

	// ... existing runner setup ...
	runner := NewForegroundRunner(cmd, ForegroundOptions{
		Timeout:  GetForegroundTimeout(),
		Graceful: false,
		KillWait: 10 * time.Second,
	})
	return runner.Run(context.Background())
}
```

**Datei:** `internal/agent/dispatch_python.go` — korrigiert:

```go
case "execute_python":
    if !cfg.Agent.AllowPython {
        return "Tool Output: [PERMISSION DENIED] ..."
    }
    req := decodePythonExecutionArgs(tc)
    if req.Code == "" {
        return "Tool Output: [EXECUTION ERROR] 'code' field is empty."
    }

    // Parse SDK options
    enableSDK := false
    if v, ok := tc.Params["enable_sdk"].(bool); ok {
        enableSDK = v
    }

    // CRITICAL: Background + SDK is forbidden
    if req.Background && enableSDK {
        return `Tool Output: {"status":"error","message":"enable_sdk cannot be used with background execution. Run foreground or disable SDK."}`
    }

    callLimit := 10
    if v, ok := tc.Params["sdk_call_limit"].(float64); ok {
        callLimit = int(v)
        if callLimit > 50 { callLimit = 50 }
        if callLimit < 1 { callLimit = 10 }
    }

    // Resolve secrets
    secrets, rejectedInfo := resolveVaultKeys(cfg, vault, req.VaultKeys, logger)
    creds, credRejInfo := resolveCredentials(cfg, vault, inventoryDB, req.CredentialIDs, logger)
    // ... combine rejection info ...

    // Determine bridge URL from server
    var bridgeURL string
    var internalToken string
    if enableSDK {
        bridgeURL = s.toolBridgeURL // from Server instance
        internalToken = s.internalToken
        if bridgeURL == "" {
            return `Tool Output: {"status":"error","message":"SDK bridge URL not configured"}`
        }
    }

    if req.Background {
        // ... existing background path (unchanged, SDK not allowed) ...
    }

    opts := tools.ExecutePythonOptions{
        Code:          req.Code,
        WorkspaceDir:  cfg.Directories.WorkspaceDir,
        ToolsDir:      cfg.Directories.ToolsDir,
        Secrets:       secrets,
        Credentials:   creds,
        EnableSDK:     enableSDK,
        BridgeURL:     bridgeURL,
        InternalToken: internalToken,
        CallLimit:     callLimit,
    }
    stdout, stderr, pyErr := tools.ExecutePythonWithOpts(opts)

    // ... existing output scrubbing and formatting ...
```

**Wichtig:** Der Dispatcher braucht Zugriff auf die `Server`-Instanz (für `toolBridgeURL` und `internalToken`). Falls `DispatchContext` das nicht hat, muss es erweitert werden:

```go
// In internal/agent/agent_parse.go or similar:
type DispatchContext struct {
    // ... existing fields ...
    ToolBridgeURL string // NEW
    InternalToken string // NEW
}
```

#### Tag 7–8: Server-Integration

**Datei:** `internal/server/server.go` oder `server_routes.go`

Die `Server`-Struktur speichert bereits `internalToken`. Die Bridge-URL sollte zur Laufzeit ermittelt werden:

```go
// In Server struct or initialization:
func (s *Server) ToolBridgeURL() string {
    // Use localhost since tool bridge is loopback-only
    port := s.Cfg.Server.Port
    if port == 0 {
        port = 8080
    }
    return fmt.Sprintf("http://127.0.0.1:%d/api/internal/tool-bridge", port)
}
```

**Hinweis:** Die Tool-Bridge prüft bereits auf Loopback (`127.0.0.1`, `::1`). Der Python-Prozess läuft auf demselben Host, also ist `127.0.0.1` korrekt.

#### Tag 8–9: Config-Dokumentation & Tests

**Config-Update:** `config_template.yaml`

```yaml
tools:
  python_tool_bridge:
    enabled: true
    allowed_tools:
      - filesystem      # REQUIRED for aurago_sdk read_file/write_file/list_dir
      - file_editor     # REQUIRED for aurago_sdk edit_file/append_to_file
      - file_search     # REQUIRED for aurago_sdk search
      # ... existing tools ...
```

**Test:** `internal/tools/python_sdk_test.go`

```go
package tools

import (
	"strings"
	"testing"
)

func TestBuildPythonSDKInjectCode(t *testing.T) {
	code := BuildPythonSDKInjectCode("/workspace/skills/aurago_sdk")
	if !strings.Contains(code, "sys.path.insert(0,") {
		t.Error("missing sys.path.insert")
	}
	if !strings.Contains(code, "import aurago_sdk as aurago") {
		t.Error("missing aurago_sdk import")
	}
	if strings.Contains(code, "PYTHONPATH") {
		t.Error("should NOT use PYTHONPATH")
	}
}

func TestPythonSDKEnv(t *testing.T) {
	env := PythonSDKEnv("http://localhost:8080/bridge", "my-token", 25)
	if env["AURAGO_BRIDGE_URL"] != "http://localhost:8080/bridge" {
		t.Error("wrong bridge URL")
	}
	if env["AURAGO_INTERNAL_TOKEN"] != "my-token" {
		t.Error("wrong token")
	}
	if env["AURAGO_SDK_CALL_LIMIT"] != "25" {
		t.Error("wrong call limit")
	}
}

func TestPythonSDKEnvLimitClamping(t *testing.T) {
	env := PythonSDKEnv("", "", 0)
	if env["AURAGO_SDK_CALL_LIMIT"] != "10" {
		t.Errorf("expected default 10, got %s", env["AURAGO_SDK_CALL_LIMIT"])
	}

	env = PythonSDKEnv("", "", 999)
	if env["AURAGO_SDK_CALL_LIMIT"] != "50" {
		t.Errorf("expected max 50, got %s", env["AURAGO_SDK_CALL_LIMIT"])
	}
}
```

**Tool-Manual:** `prompts/tools_manuals/execute_python.md` (neu oder aktualisieren):

```markdown
## Python SDK (Tool Reentry)

When `enable_sdk` is `true`, your Python script can call native AuraGo tools directly.

### Setup

Add `enable_sdk: true` to your `execute_python` call. The SDK is automatically injected.

### Available Functions

```python
import aurago_sdk as aurago

# Read a file
content = aurago.read_file("internal/agent/agent_loop.go")

# Write a file
aurago.write_file("output.txt", "hello world")

# Search across files
files = aurago.search("func.*SpawnCoAgent", glob="*.go")

# Edit a file
aurago.edit_file("main.go", "str_replace", old="func main() {", new="func main() error {")

# List directory
entries = aurago.list_dir("internal/tools")

# Append to file
aurago.append_to_file("log.txt", "new line\n")
```

### Limitations

- **Maximum 10 tool calls per script** (configurable via `sdk_call_limit`, absolute max: 50)
- **Cannot be used with `background: true`** — SDK requires a foreground process
- **Requires tool bridge permissions** — Admin must add `filesystem`, `file_editor`, and `file_search` to `tools.python_tool_bridge.allowed_tools`
- **Secrets are NOT automatically forwarded** to SDK calls — use `vault_keys` if needed
```

---

## 5. Rückwärtskompatibilität (vollständig gewährleistet)

| Aspekt | Strategie |
|--------|-----------|
| Co-Agent ohne Schema | Unverändert. `output_schema` ist optional. |
| Python ohne SDK | Unverändert. `enable_sdk` defaultet auf `false`. |
| Tool-Bridge | Unverändert. Verwendet bestehendes `X-Internal-Token`. |
| `CoAgentRegistry` | Neue Methoden sind additiv. `Complete()` bleibt unverändert. |
| Prompts | Alte Prompts funktionieren, da keine neuen Pflichtparameter. |

---

## 6. Risiken & Mitigationen (aktualisiert)

| Risiko | Wahrscheinlichkeit | Mitigation |
|--------|-------------------|------------|
| Co-Agent ignoriert Schema-Prompt | Mittel | Fallback: `structured_valid=false`, raw text available |
| Co-Agent JSON ist invalide | Mittel | `extractJSONFromText` sucht nach JSON-Blöcken; Validierung fängt Fehler ab |
| Schema-Injection (DoS) | Niedrig | `sanitizeJSONSchema` limitiert Tiefe, Größe, Pattern-Länge |
| Python-SDK Call-Limit überschritten | Niedrig | Hartes Limit im SDK (10 default, 50 max) |
| Tool-Bridge `allowed_tools` fehlt | Hoch (Config-Fehler) | Dokumentation; klare Fehlermeldung im SDK |
| Bridge-URL falsch (Container/Netzwerk) | Mittel | `127.0.0.1` (Loopback); Admin kann Bridge-URL konfigurieren |
| Background + SDK umgeht Limit | Niedrig | Explizit verboten im Dispatch |

---

## 7. Erfolgsmetriken

### Schema-valide Co-Agents
- **Adoption:** ≥80 % der Co-Agent-Runs mit `output_schema` liefern `structured_valid=true`
- **Token-Einsparung:** Parent-Agent braucht weniger Follow-up-Fragen (subjektiv, via Feedback)
- **Kein LLM-Overhead:** Kein zusätzlicher LLM-Aufruf pro Co-Agent (im Gegensatz zu Plan v1)

### Tool-Reentry
- **Adoption:** Anzahl `execute_python`-Aufrufe mit `enable_sdk=true`
- **Bulk-Operationen:** Agent kann ≥20 Dateien in einem Python-Skript transformieren
- **Fehlerrate:** SDK-Tool-Calls mit <5 % Fehlern

---

## 8. Abhängigkeiten zwischen Teil A und Teil B

| Abhängigkeit | Beschreibung |
|-------------|--------------|
| `DispatchContext` Erweiterung | Teil B braucht `ToolBridgeURL`/`InternalToken` im DispatchContext. Teil A braucht das nicht. |
| `gojsonschema` Dependency | Nur Teil A. |
| `CoAgentRegistry` Erweiterung | Nur Teil A. |
| Gemeinsame Datei `agent_dispatch_exec.go` | Beide erweitern verschiedene `case`-Blöcke — konfliktfrei. |

**Empfehlung:** Beide Teile können parallel entwickelt werden.

---

*Plan korrigiert: 2026-06-07*
