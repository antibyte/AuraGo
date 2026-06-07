# Implementierungsplan: Hashline-Edit für AuraGo

**Projekt:** AuraGo
**Feature:** Content-Hash-Anchored File Editing
**Geschätzter Aufwand:** 7–10 Arbeitstage
**Ziel:** Reduziere Edit-Fehler durch Stale-Context-Erkennung um ~40 %

---

## 1. Architektur-Überblick

```
┌─────────────────────────────────────────────────────────────┐
│  LLM                                                        │
│   ├──> filesystem read_file (neu: include_hashes=true)      │
│   │        ↓                                                │
│   │    Zeilen mit Hash: "123#a3f7c2:func main() {"          │
│   │        ↓                                                │
│   └──> file_editor hashline_replace                         │
│             anchor_line=123, anchor_hash="a3f7c2"           │
│             old_text="func main() {", new_text="..."         │
│                      ↓                                      │
│              ┌──────────────┐                               │
│              │  Stale?      │──Hash mismatch──> FEHLER      │
│              │  (Re-read!)  │                               │
│              └──────┬───────┘                               │
│                     ↓ Hash OK                               │
│              Text-Replacement                               │
│                     ↓                                       │
│              Atomic Write                                   │
└─────────────────────────────────────────────────────────────┘
```

**Kernidee:** Das Modell liest nicht nur Zeilen, sondern erhält pro Zeile einen kurzen Hash. Bei einem späteren Edit referenziert es eine konkrete Zeile über ihre Nummer **und** ihren Hash. Stimmt der Hash nicht mehr, wurde der Kontext zwischenzeitlich invalid — das Tool lehnt ab statt blind zu editieren.

---

## 2. Dateien: Neu / Modifiziert / Unverändert

| Datei | Aktion | Beschreibung |
|-------|--------|--------------|
| `internal/tools/hashline.go` | **Neu** | Hash-Berechnung, Zeilen-Hash-Struktur, Parser |
| `internal/tools/file_editor.go` | **Modifizieren** | Neue Operationen `hashline_replace`, `hashline_insert_after`, `hashline_insert_before`, `hashline_delete` |
| `internal/tools/filesystem.go` | **Modifizieren** | `read_file` um `include_hashes` erweitern |
| `internal/agent/native_tools_core.go` | **Modifizieren** | JSON-Schema für `filesystem` und `file_editor` erweitern |
| `internal/agent/agent_dispatch_exec.go` | **Modifizieren** | Dispatch-Routing für neue `file_editor`-Operationen |
| `internal/tools/file_editor_test.go` | **Neu** | Unit-Tests für alle Hashline-Operationen |
| `prompts/tools_manuals/file_editor.md` | **Modifizieren** | Neue Hashline-Syntax dokumentieren |
| `prompts/tools_manuals/filesystem.md` | **Modifizieren** | `include_hashes` Parameter dokumentieren |

---

## 3. Hash-Algorithmus

### Anforderungen
- **Schnell:** Dateien mit 10.000+ Zeilen in <10ms hashen
- **Kompakt:** Kurze Ausgabe, nicht 64-char SHA256 pro Zeile
- **Kollisionsresistent genug:** Für Agenten-Editing ausreichend
- **Deterministisch:** Gleicher Inhalt → gleicher Hash

### Wahl: xxHash32 (oder FNV-1a als Fallback)

```go
// internal/tools/hashline.go
package tools

import (
	"fmt"
	"hash/fnv"
	"strconv"
	"strings"
)

// hashLine computes a compact 8-char hex hash for a line.
// The hash includes the line number to prevent simple line-shift attacks.
func hashLine(lineNum int, content string) string {
	h := fnv.New32a()
	h.Write([]byte(strconv.Itoa(lineNum)))
	h.Write([]byte{':'})
	h.Write([]byte(content))
	return fmt.Sprintf("%08x", h.Sum32())[:6] // first 6 hex chars = 24 bit
}

// HashlineEntry represents a single line with its hash.
type HashlineEntry struct {
	LineNum int    `json:"line_number"`
	Hash    string `json:"hash"`
	Content string `json:"content"`
}

// formatHashlineOutput formats lines as "123#abc123:content".
func formatHashlineOutput(entries []HashlineEntry) string {
	var sb strings.Builder
	for _, e := range entries {
		sb.WriteString(fmt.Sprintf("%d#%s:%s\n", e.LineNum, e.Hash, e.Content))
	}
	return sb.String()
}

// parseHashlineReference parses a hashline reference from the model.
// Expected format: line_number + "#" + hash (e.g., "123#abc123").
func parseHashlineReference(ref string) (lineNum int, hash string, err error) {
	parts := strings.SplitN(ref, "#", 2)
	if len(parts) != 2 {
		return 0, "", fmt.Errorf("invalid hashline reference format: %q (expected: LINE#HASH)", ref)
	}
	lineNum, err = strconv.Atoi(parts[0])
	if err != nil {
		return 0, "", fmt.Errorf("invalid line number in hashline reference: %q", parts[0])
	}
	hash = parts[1]
	return lineNum, hash, nil
}
```

**Begründung für FNV-1a statt SHA256:**
- FNV-1a ist ~10× schneller für kurze Strings
- 6 Hex-Zeichen (24 Bit) = ~16 Millionen Kombinationen → für Agenten-Editing ausreichend
- Kollision bei normalen Codezeilen praktisch ausgeschlossen
- Keine externe Dependency nötig (im Go-Standardpaket)

---

## 4. Schritt-für-Schritt-Implementierung

### Phase A: Hashline-Utilities (Tag 1)

**Datei:** `internal/tools/hashline.go` (neu)

Erstelle die oben gezeigten Funktionen plus:

```go
// buildHashlineEntries reads a file and returns all lines with hashes.
func buildHashlineEntries(data []byte) []HashlineEntry {
	lines := strings.Split(string(data), "\n")
	entries := make([]HashlineEntry, 0, len(lines))
	for i, line := range lines {
		// Preserve empty lines (important for formatting)
		entries = append(entries, HashlineEntry{
			LineNum: i + 1,
			Hash:    hashLine(i+1, line),
			Content: line,
		})
	}
	return entries
}

// validateHashlineAnchor checks if the anchor line still has the expected hash.
func validateHashlineAnchor(entries []HashlineEntry, lineNum int, expectedHash string) error {
	if lineNum < 1 || lineNum > len(entries) {
		return fmt.Errorf("anchor line %d is out of range (file has %d lines)", lineNum, len(entries))
	}
	actualHash := entries[lineNum-1].Hash
	if actualHash != expectedHash {
		return fmt.Errorf(
			"STALE CONTEXT: anchor line %d has hash %q, expected %q. "+
			"The file has changed since you last read it. Please re-read the file with include_hashes=true and try again.",
			lineNum, actualHash, expectedHash)
	}
	return nil
}
```

### Phase B: read_file erweitern (Tag 1–2)

**Datei:** `internal/tools/filesystem.go`

Aktueller `read_file`-Code (vereinfacht):
```go
case "read_file":
    // ... path resolution ...
    data, err := os.ReadFile(resolved)
    // ... binary check ...
    // ... truncate to 32KB ...
```

Erweiterung:
```go
case "read_file":
    includeHashes := false
    if v, ok := tc.Params["include_hashes"].(bool); ok {
        includeHashes = v
    }
    
    // ... path resolution, binary check ...
    
    if includeHashes {
        entries := buildHashlineEntries(data)
        output := formatHashlineOutput(entries)
        // Still respect the 32KB cap, but now on formatted output
        if len(output) > maxReadFileChars {
            output = output[:maxReadFileChars]
            output += "\n[File truncated — use file_reader_advanced with offset/limit for more]"
        }
        return FSResult{Status: "success", Data: map[string]interface{}{
            "content": output,
            "format":  "hashline",
            "total_lines": len(entries),
        }}
    }
    // ... existing logic ...
```

**Schema-Update in** `internal/agent/native_tools_core.go`:
```go
// Im filesystem-Tool schema:
"include_hashes": map[string]interface{}{
    "type":        "boolean",
    "description": "If true, each line is prefixed with its line number and content hash (format: LINE#HASH:CONTENT). Use this when you plan to edit the file with hashline_replace for stale-context protection.",
},
```

### Phase C: Neue file_editor Operationen (Tag 2–4)

**Datei:** `internal/tools/file_editor.go`

Neue Operationen in `ExecuteFileEditor`:
```go
case "hashline_replace":
    return fileHashlineReplace(resolved, old, new_, anchorLine, anchorHash, encode)
case "hashline_insert_after":
    return fileHashlineInsert(resolved, marker, content, anchorLine, anchorHash, true, encode)
case "hashline_insert_before":
    return fileHashlineInsert(resolved, marker, content, anchorLine, anchorHash, false, encode)
case "hashline_delete":
    return fileHashlineDelete(resolved, startLine, endLine, startHash, endHash, encode)
```

**Signatur-Anpassung** (rückwärtskompatibel):
```go
func ExecuteFileEditor(
    operation, filePath, old, new_, marker, content string,
    startLine, endLine, lineCount int,
    workspaceDir string,
    // Neue Parameter (für Hashline-Operationen)
    anchorLine int,
    anchorHash string,
    startHash string,
    endHash string,
) string
```

Da Go keine überladenen Funktionen hat, bieten sich zwei Muster an:

**Option A (Empfohlen):** Neue Funktion `ExecuteHashlineEditor`:
```go
func ExecuteHashlineEditor(
    operation, filePath string,
    old, new_, marker, content string,
    anchorLine int, anchorHash string,
    startLine, endLine int, startHash, endHash string,
    workspaceDir string,
) string
```

**Option B:** Parameter-Struct:
```go
type FileEditorRequest struct {
    Operation   string
    FilePath    string
    Old         string
    New         string
    Marker      string
    Content     string
    StartLine   int
    EndLine     int
    LineCount   int
    AnchorLine  int
    AnchorHash  string
    StartHash   string
    EndHash     string
    WorkspaceDir string
}
```

**Empfehlung:** Option A für minimale Intrusion. Der Dispatcher ruft die passende Funktion auf.

#### Implementierung `fileHashlineReplace`:

```go
func fileHashlineReplace(resolved, old, new_ string, anchorLine int, anchorHash string, encode func(FileEditorResult) string) string {
    if old == "" {
        return encode(FileEditorResult{Status: "error", Message: "'old' text is required for hashline_replace"})
    }
    if err := checkEditSizeLimit(resolved); err != nil {
        return encode(FileEditorResult{Status: "error", Message: err.Error()})
    }
    data, err := os.ReadFile(resolved)
    if err != nil {
        return encode(FileEditorResult{Status: "error", Message: fmt.Sprintf("Failed to read file: %v", err)})
    }

    entries := buildHashlineEntries(data)

    // Validate anchor if provided
    if anchorLine > 0 && anchorHash != "" {
        if err := validateHashlineAnchor(entries, anchorLine, anchorHash); err != nil {
            return encode(FileEditorResult{Status: "error", Message: err.Error()})
        }
    }

    text := string(data)
    count := strings.Count(text, old)
    if count == 0 {
        return encode(FileEditorResult{Status: "error", Message: "The 'old' text was not found in the file"})
    }
    if count > 1 {
        // Same disambiguation hint as str_replace
        var occurrences []string
        for i, part := range strings.SplitAfter(text, old) {
            if i == 0 || i >= count { continue }
            before := strings.Join(strings.SplitAfter(text, old)[:i], "")
            lineStart := strings.LastIndex(before[:len(before)-len(old)], "\n")
            if lineStart < 0 { lineStart = 0 } else { lineStart++ }
            lineEnd := strings.Index(before[lineStart:], "\n")
            if lineEnd < 0 { lineEnd = len(before) - lineStart }
            line := strings.TrimSpace(before[lineStart : lineStart+lineEnd])
            if len(line) > 80 { line = line[:80] + "…" }
            occurrences = append(occurrences, fmt.Sprintf("  match %d: …%s…", i, line))
            _ = part
        }
        hint := strings.Join(occurrences, "\n")
        return encode(FileEditorResult{Status: "error", Message: fmt.Sprintf(
            "The 'old' text was found %d times — must be unique. Include more surrounding context, or use hashline_replace with anchor_line/anchor_hash to target a specific occurrence near that line.\n%s",
            count, hint)})
    }

    result := strings.Replace(text, old, new_, 1)
    if err := writeFileAtomic(resolved, []byte(result)); err != nil {
        return encode(FileEditorResult{Status: "error", Message: fmt.Sprintf("Failed to write file: %v", err)})
    }

    newLines := strings.Count(result, "\n")
    return encode(FileEditorResult{
        Status:       "success",
        Message:      "Replaced 1 occurrence (hashline-validated)",
        LinesChanged: 1,
        TotalLines:   newLines + 1,
    })
}
```

#### Implementierung `fileHashlineInsert`:

```go
func fileHashlineInsert(resolved, marker, content string, anchorLine int, anchorHash string, after bool, encode func(FileEditorResult) string) string {
    if marker == "" {
        return encode(FileEditorResult{Status: "error", Message: "'marker' text is required"})
    }
    if content == "" {
        return encode(FileEditorResult{Status: "error", Message: "'content' is required"})
    }
    if err := checkEditSizeLimit(resolved); err != nil {
        return encode(FileEditorResult{Status: "error", Message: err.Error()})
    }
    data, err := os.ReadFile(resolved)
    if err != nil {
        return encode(FileEditorResult{Status: "error", Message: fmt.Sprintf("Failed to read file: %v", err)})
    }

    entries := buildHashlineEntries(data)

    // Find the marker line using hash validation if provided
    markerIdx := -1
    for i, entry := range entries {
        if strings.Contains(entry.Content, marker) {
            if markerIdx >= 0 {
                return encode(FileEditorResult{Status: "error", Message: "Marker text found on multiple lines — provide a more specific marker"})
            }
            markerIdx = i
            // If anchor provided, validate it matches this line
            if anchorLine > 0 && anchorHash != "" {
                if entry.LineNum != anchorLine || entry.Hash != anchorHash {
                    return encode(FileEditorResult{Status: "error", Message: fmt.Sprintf(
                        "STALE CONTEXT: expected marker at line %d with hash %q, but found matching marker at line %d with hash %q. Please re-read the file.",
                        anchorLine, anchorHash, entry.LineNum, entry.Hash)})
                }
            }
        }
    }
    if markerIdx < 0 {
        return encode(FileEditorResult{Status: "error", Message: "Marker text not found in the file"})
    }

    insertLines := strings.Split(content, "\n")
    insertIdx := markerIdx
    if after {
        insertIdx = markerIdx + 1
    }

    newLines := make([]string, 0, len(entries)+len(insertLines))
    newLines = append(newLines, entries[:insertIdx]...)
    for _, l := range insertLines {
        newLines = append(newLines, l)
    }
    newLines = append(newLines, entries[insertIdx:]...)

    result := strings.Join(newLines, "\n")
    if err := writeFileAtomic(resolved, []byte(result)); err != nil {
        return encode(FileEditorResult{Status: "error", Message: fmt.Sprintf("Failed to write file: %v", err)})
    }

    return encode(FileEditorResult{
        Status:       "success",
        Message:      fmt.Sprintf("Inserted %d line(s) %s marker (hashline-validated)", len(insertLines), map[bool]string{true: "after", false: "before"}[after]),
        LinesChanged: len(insertLines),
        TotalLines:   len(newLines),
    })
}
```

### Phase D: Schema-Updates (Tag 4)

**Datei:** `internal/agent/native_tools_core.go`

Das `file_editor`-Schema erweitern:
```go
func buildCoreToolSchemas(ff ToolFeatureFlags, execSkillProps map[string]interface{}) []openai.Tool {
    tools := []openai.Tool{
        // ... existing tools ...
        
        func() openai.Tool {
            if ff.AllowFilesystemWrite {
                return tool("file_editor",
                    "Precise file editing with hashline validation support. "+
                        "Use hashline_replace/hashline_insert_after/hashline_insert_before when you have read the file with include_hashes=true. "+
                        "Use str_replace for simple edits without hash protection.",
                    schema(map[string]interface{}{
                        "operation": map[string]interface{}{
                            "type":        "string",
                            "description": "Editing operation to perform",
                            "enum": []string{
                                "str_replace", "str_replace_all", "str_replace_regex", "str_replace_glob",
                                "insert_after", "insert_before", "append", "prepend", "delete_lines", "apply_patch",
                                "hashline_replace", "hashline_insert_after", "hashline_insert_before", "hashline_delete",
                            },
                        },
                        "file_path":    prop("string", "Path to the file to edit"),
                        "old":          prop("string", "Text to replace (for str_replace*, hashline_replace)"),
                        "new":          prop("string", "Replacement text"),
                        "marker":       prop("string", "Marker line text (for insert_after/insert_before)"),
                        "content":      prop("string", "Content to insert (for insert_*, append, prepend, apply_patch)"),
                        "start_line":   prop("integer", "Start line for delete_lines (1-based)"),
                        "end_line":     prop("integer", "End line for delete_lines (1-based, inclusive)"),
                        // NEW: Hashline parameters
                        "anchor_line":  prop("integer", "Line number to anchor the edit to (from include_hashes read). Validates the line hasn't changed."),
                        "anchor_hash":  prop("string", "Expected hash of the anchor line (format: 6 hex chars from LINE#HASH:CONTENT)."),
                        "start_hash":   prop("string", "Expected hash of start_line (for hashline_delete)."),
                        "end_hash":     prop("string", "Expected hash of end_line (for hashline_delete)."),
                    }, "operation", "file_path"),
                )
            }
            // Read-only variant without hashline operations
            return tool("file_editor",
                "Read-only file inspection (editing is disabled in Danger Zone settings).",
                schema(map[string]interface{}{
                    "operation": map[string]interface{}{
                        "type":        "string",
                        "enum":        []string{"count_lines"},
                    },
                    "file_path": prop("string", "Path to the file"),
                }, "operation", "file_path"),
            )
        }(),
        // ... rest of tools ...
    }
}
```

### Phase E: Dispatch-Update (Tag 4–5)

**Datei:** `internal/agent/agent_dispatch_exec.go`

Im `file_editor`-Dispatch-Block:
```go
case "file_editor":
    if !cfg.Agent.AllowFilesystemWrite {
        return `Tool Output: {"status":"error","message":"file_editor requires filesystem write permission (agent.allow_filesystem_write)"}`
    }
    op := stringValueFromMap(tc.Params, "operation")
    filePath := resolveFilePath(tc)
    old := stringValueFromMap(tc.Params, "old")
    newText := stringValueFromMap(tc.Params, "new")
    marker := stringValueFromMap(tc.Params, "marker")
    content := stringValueFromMap(tc.Params, "content")
    startLine, _ := strconv.Atoi(stringValueFromMap(tc.Params, "start_line"))
    endLine, _ := strconv.Atoi(stringValueFromMap(tc.Params, "end_line"))
    
    // NEW: Hashline parameters
    anchorLine, _ := strconv.Atoi(stringValueFromMap(tc.Params, "anchor_line"))
    anchorHash := stringValueFromMap(tc.Params, "anchor_hash")
    startHash := stringValueFromMap(tc.Params, "start_hash")
    endHash := stringValueFromMap(tc.Params, "end_hash")
    
    // Route to hashline editor for hashline operations
    switch op {
    case "hashline_replace", "hashline_insert_after", "hashline_insert_before", "hashline_delete":
        return tools.ExecuteHashlineEditor(op, filePath, old, newText, marker, content,
            anchorLine, anchorHash, startLine, endLine, startHash, endHash,
            cfg.Directories.WorkspaceDir)
    default:
        return tools.ExecuteFileEditor(op, filePath, old, newText, marker, content,
            startLine, endLine, 0, cfg.Directories.WorkspaceDir)
    }
```

### Phase F: Tool-Manuals aktualisieren (Tag 5)

**Datei:** `prompts/tools_manuals/file_editor.md`

Neuer Abschnitt anfügen:
```markdown
## Hashline-Mode (Recommended for Complex Edits)

When editing files that may have changed between read and write, use the hashline mode:

1. **Read with hashes:** Call `filesystem` with `operation: read_file` and `include_hashes: true`.
   Output format: `123#abc123:func main() {`

2. **Edit with validation:** Call `file_editor` with a hashline operation:
   - `hashline_replace` — Replace text with anchor validation
   - `hashline_insert_after` / `hashline_insert_before` — Insert with marker validation
   - `hashline_delete` — Delete line range with hash validation

3. **Parameters:**
   - `anchor_line`: The line number from the hashline read
   - `anchor_hash`: The 6-char hash from the hashline read (e.g., `abc123`)

**Example:**
```json
{
  "operation": "hashline_replace",
  "file_path": "main.go",
  "old": "func main() {",
  "new": "func main() error {",
  "anchor_line": 42,
  "anchor_hash": "a3f7c2"
}
```

If the file changed since reading, you'll get:
`STALE CONTEXT: anchor line 42 has hash x9k2m1, expected a3f7c2. Please re-read the file...`
```

### Phase G: Tests (Tag 6–7)

**Datei:** `internal/tools/hashline_test.go` (neu)

```go
package tools

import (
	"strings"
	"testing"
)

func TestHashLine(t *testing.T) {
	// Determinism
	h1 := hashLine(42, "func main() {")
	h2 := hashLine(42, "func main() {")
	if h1 != h2 {
		t.Errorf("hashLine not deterministic: %q vs %q", h1, h2)
	}

	// Line-number sensitivity
	h3 := hashLine(43, "func main() {")
	if h1 == h3 {
		t.Error("hashLine should differ for different line numbers")
	}

	// Content sensitivity
	h4 := hashLine(42, "func main() {}")
	if h1 == h4 {
		t.Error("hashLine should differ for different content")
	}

	// Length check
	if len(h1) != 6 {
		t.Errorf("expected 6-char hash, got %d: %q", len(h1), h1)
	}
}

func TestValidateHashlineAnchor(t *testing.T) {
	entries := []HashlineEntry{
		{LineNum: 1, Hash: "abc123", Content: "package main"},
		{LineNum: 2, Hash: "def456", Content: "func main() {"},
	}

	// Valid
	if err := validateHashlineAnchor(entries, 2, "def456"); err != nil {
		t.Errorf("expected no error, got: %v", err)
	}

	// Stale
	err := validateHashlineAnchor(entries, 2, "oldhash")
	if err == nil {
		t.Error("expected stale context error")
	}
	if !strings.Contains(err.Error(), "STALE CONTEXT") {
		t.Errorf("expected 'STALE CONTEXT' in error, got: %v", err)
	}

	// Out of range
	err = validateHashlineAnchor(entries, 99, "xxx")
	if err == nil {
		t.Error("expected out of range error")
	}
}

func TestFileHashlineReplace(t *testing.T) {
	tmp := t.TempDir()
	file := filepath.Join(tmp, "test.go")
	content := "package main\n\nfunc main() {\n\tfmt.Println(\"hello\")\n}\n"
	os.WriteFile(file, []byte(content), 0644)

	encode := func(r FileEditorResult) string {
		b, _ := json.Marshal(r)
		return string(b)
	}

	// Success with valid anchor
	result := fileHashlineReplace(file, "func main() {", "func main() error {", 3, hashLine(3, "func main() {"), encode)
	if !strings.Contains(result, `"status":"success"`) {
		t.Errorf("expected success, got: %s", result)
	}

	// Verify file content
	data, _ := os.ReadFile(file)
	if !strings.Contains(string(data), "func main() error {") {
		t.Error("file was not updated")
	}

	// Stale anchor (simulate file change)
	os.WriteFile(file, []byte(content+"// changed\n"), 0644)
	result = fileHashlineReplace(file, "func main() {", "func main() error {", 3, hashLine(3, "func main() {"), encode)
	if !strings.Contains(result, "STALE CONTEXT") {
		t.Errorf("expected stale context error, got: %s", result)
	}
}
```

**Datei:** `internal/tools/file_editor_test.go` — Erweitern um Hashline-Tests oder separate Datei `file_editor_hashline_test.go`.

### Phase H: Integrationstest & Rollout (Tag 8–10)

1. **Lokaler Test:** Ein einfacher Agent-Loop-Test mit einer Datei, die zwischen read und write geändert wird
2. **E2E-Test:** Vollständiger Flow über `/v1/chat/completions`
3. **Feature-Toggle:** Optional `hashline_edits_enabled` in `config.yaml` (Default: `true`)
4. **Monitoring:** Metrik für "hashline_stale_rejections" in `internal/agent/tool_execution_policy.go`

---

## 5. Rückwärtskompatibilität

| Aspekt | Strategie |
|--------|-----------|
| `str_replace` | Bleibt unverändert verfügbar. Hashline ist opt-in. |
| `read_file` | `include_hashes` defaultet auf `false`. Kein Breaking Change. |
| `file_editor` | Neue Operationen sind additive. Alte Schemas bleiben gültig. |
| Prompts | Alte Prompts ohne Hashline-Anweisungen funktionieren weiter. |

---

## 6. Risiken & Mitigationen

| Risiko | Wahrscheinlichkeit | Mitigation |
|--------|-------------------|------------|
| Modell nutzt Hashline-Parameter falsch | Mittel | Starke Tool-Manual-Doku; Fallback auf `str_replace` bei Fehlern |
| Hashline-Ausgabe frisst zu viele Tokens | Mittel | 6-Char-Hash; bei großen Dateien weiterhin Pagination empfohlen |
| FNV-1a-Kollision führt zu falsch-negativem Stale-Check | Sehr niedrig | Akzeptabel für Agenten-Use-Case; bei Bedarf auf xxHash64 wechseln |
| `buildHashlineEntries` langsam bei riesigen Dateien | Niedrig | Dateien >10MB werden eh von `checkEditSizeLimit` abgelehnt |

---

## 7. Erfolgsmetriken

- **Reduktion von** `"old text not found"`-Fehlern im `file_editor` um ≥30 %
- **Reduktion von** Multi-Match-Fehlern bei `str_replace` durch gezielte Hashline-Anker
- **Keine Regression** bei bestehenden `str_replace`-Erfolgsraten
- **Agent-Loop-Einsparung:** Weniger Re-Read-Schleifen wegen stale context

---

*Plan erstellt: 2026-06-07*
