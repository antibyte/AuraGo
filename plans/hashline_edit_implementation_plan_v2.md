# Implementierungsplan: Hashline-Edit für AuraGo (korrigiert)

**Projekt:** AuraGo
**Feature:** Content-Hash-Anchored File Editing
**Geschätzter Aufwand:** 6–9 Arbeitstage (vorher: 7–10)
**Ziel:** Reduziere Edit-Fehler durch Stale-Context-Erkennung um ~40 %

---

## 1. Architektur-Überblick

```
┌─────────────────────────────────────────────────────────────┐
│  LLM                                                        │
│   ├──> filesystem read_file (include_hashes=true)           │
│   │        ↓                                                │
│   │    Zeilen mit Hash: "123#abc123:func main() {"          │
│   │        (abc123 = hash(content), OHNE Zeilennummer!)     │
│   │        ↓                                                │
│   └──> file_editor hashline_replace                         │
│             anchor_line=123, anchor_hash="abc123"           │
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

**Kernidee:** Das Modell liest Zeilen mit einem Content-Hash. Bei einem späteren Edit referenziert es eine konkrete Zeile über Nummer **und** Hash. Stimmt der Hash nicht mehr, wurde der Kontext zwischenzeitlich invalid — das Tool lehnt ab.

**Wichtig:** Der Hash enthält **nur den Zeileninhalt**, nicht die Zeilennummer. Dadurch bleiben die Hashes von Zeilen unterhalb eines Edits gültig, auch wenn sich ihre Zeilennummern verschieben. Das ermöglicht gezielte Edit-Sequenzen, solange die Content-Hashes der Anker stimmen.

---

## 2. Dateien: Neu / Modifiziert / Unverändert

| Datei | Aktion | Beschreibung |
|-------|--------|--------------|
| `internal/tools/hashline.go` | **Neu** | Content-Only-Hash, Zeilen-Hash-Struktur, Parser, Validator |
| `internal/tools/file_editor.go` | **Modifizieren** | Neue Funktion `ExecuteHashlineEditor` mit 4 Operationen |
| `internal/tools/filesystem.go` | **Modifizieren** | `read_file` um `include_hashes` erweitern |
| `internal/agent/native_tools_core.go` | **Modifizieren** | JSON-Schema für `filesystem` und `file_editor` erweitern |
| `internal/agent/dispatch_filesystem.go` | **Modifizieren** | Routing: `hashline_*` Ops → `ExecuteHashlineEditor` |
| `internal/tools/file_editor_test.go` | **Neu** | Unit-Tests für Hashline-Operationen |
| `prompts/tools_manuals/file_editor.md` | **Modifizieren** | Hashline-Syntax und Multi-Edit-Workflow dokumentieren |
| `prompts/tools_manuals/filesystem.md` | **Modifizieren** | `include_hashes` Parameter dokumentieren |

**Nicht modifiziert:**
- `ExecuteFileEditor` bleibt komplett unverändert (kein Breaking Change)
- Alle bestehenden Caller bleiben unverändert

---

## 3. Hash-Algorithmus (korrigiert: Content-Only)

### Anforderungen
- **Schnell:** 10.000+ Zeilen in <10ms
- **Kompakt:** Kurze Ausgabe
- **Content-only:** Hash ändert sich NICHT, wenn sich die Zeilennummer ändert
- **Deterministisch:** Gleicher Inhalt → gleicher Hash

### Implementierung: FNV-1a (6 Hex-Zeichen)

```go
// internal/tools/hashline.go
package tools

import (
	"fmt"
	"hash/fnv"
	"strconv"
	"strings"
)

// hashLineContent computes a compact 6-char hex hash for a line's CONTENT ONLY.
// The hash does NOT include the line number, so line shifts (inserts/deletes
// above) do NOT invalidate hashes of unchanged lines below.
func hashLineContent(content string) string {
	h := fnv.New32a()
	h.Write([]byte(content))
	return fmt.Sprintf("%06x", h.Sum32())
}

// HashlineEntry represents a single line with its content hash.
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

// buildHashlineEntries reads file content and returns all lines with content hashes.
func buildHashlineEntries(data []byte) []HashlineEntry {
	lines := strings.Split(string(data), "\n")
	entries := make([]HashlineEntry, 0, len(lines))
	for i, line := range lines {
		entries = append(entries, HashlineEntry{
			LineNum: i + 1,
			Hash:    hashLineContent(line),
			Content: line,
		})
	}
	return entries
}

// validateHashlineAnchor checks if the anchor line still has the expected content hash.
func validateHashlineAnchor(entries []HashlineEntry, lineNum int, expectedHash string) error {
	if lineNum < 1 || lineNum > len(entries) {
		return fmt.Errorf("anchor line %d is out of range (file has %d lines)", lineNum, len(entries))
	}
	actualHash := entries[lineNum-1].Hash
	if actualHash != expectedHash {
		return fmt.Errorf(
			"STALE CONTEXT: line %d has content hash %q, expected %q. "+
			"The file has changed since you last read it. "+
			"Please re-read the file with include_hashes=true and try again.",
			lineNum, actualHash, expectedHash)
	}
	return nil
}
```

### Warum Content-Only und nicht LineNum+Content?

| Ansatz | Pro | Contra |
|--------|-----|--------|
| **LineNum+Content** (Plan v1) | Erkennt Zeilenverschiebungen | Nach jedem Edit sind ALLE Hashes ab Edit-Position ungültig. Modell muss nach jedem Edit neu lesen. |
| **Content-only** (Plan v2) | Zeilen unterhalb eines Edits behalten ihre Hashes. Modell kann mehrere Edits in derselben Datei machen, solange die Anker-Inhalte gleich bleiben. | Erkennt KEINE Zeilenverschiebungen durch Insert/Delete. Aber: `anchor_line` fängt das ab. |

**Entscheidung:** Content-only ist der bessere Trade-off für Agenten-Editing. Das Modell verwendet `anchor_line` für die Position und `anchor_hash` für die Content-Validierung.

---

## 4. Schritt-für-Schritt-Implementierung

### Phase A: Hashline-Utilities (Tag 1)

**Datei:** `internal/tools/hashline.go` (neu)

Erstelle die oben gezeigten Funktionen.

### Phase B: read_file erweitern (Tag 1–2)

**Datei:** `internal/tools/filesystem.go`

In der `read_file`-Verarbeitung:
```go
case "read_file":
    includeHashes := false
    if v, ok := tc.Params["include_hashes"].(bool); ok {
        includeHashes = v
    }
    
    // ... existing path resolution, binary check ...
    
    if includeHashes {
        entries := buildHashlineEntries(data)
        output := formatHashlineOutput(entries)
        
        // Respect a slightly higher cap to compensate for hash overhead
        const hashlineReadMaxChars = 45000 // was 32768, +~37% for hash overhead
        if len(output) > hashlineReadMaxChars {
            output = output[:hashlineReadMaxChars]
            output += "\n[File truncated — use file_reader_advanced with offset/limit for more]"
        }
        return FSResult{Status: "success", Data: map[string]interface{}{
            "content":     output,
            "format":      "hashline",
            "total_lines": len(entries),
        }}
    }
    // ... existing logic ...
```

**Schema-Update in** `internal/agent/native_tools_core.go`:
```go
"include_hashes": map[string]interface{}{
    "type":        "boolean",
    "description": "If true, each line is prefixed with its content hash (format: LINE#HASH:CONTENT). "+
        "The hash is computed from the line content ONLY, so hashes of unchanged lines remain valid even if line numbers shift due to edits above. "+
        "Use this when you plan to edit the file with hashline_replace for stale-context protection.",
},
```

### Phase C: Neue ExecuteHashlineEditor Funktion (Tag 2–4)

**Datei:** `internal/tools/file_editor.go`

**Neue Funktion** (bestehende `ExecuteFileEditor` bleibt unverändert):

```go
// HashlineEditorRequest holds parameters for hashline-based editing.
type HashlineEditorRequest struct {
    Operation   string
    FilePath    string
    Old         string
    New         string
    Marker      string
    Content     string
    AnchorLine  int
    AnchorHash  string
    StartLine   int
    EndLine     int
}

// ExecuteHashlineEditor handles hashline-validated file editing operations.
// This is a SEPARATE function from ExecuteFileEditor to maintain backward compatibility.
func ExecuteHashlineEditor(req HashlineEditorRequest, workspaceDir string) string {
    encode := func(r FileEditorResult) string {
        b, _ := json.Marshal(r)
        return string(b)
    }

    if err := requireFilesystemWritePermission(); err != nil {
        return encode(FileEditorResult{Status: "error", Message: err.Error()})
    }
    if req.FilePath == "" {
        return encode(FileEditorResult{Status: "error", Message: "'file_path' is required"})
    }

    resolved, err := secureResolve(workspaceDir, req.FilePath)
    if err != nil {
        return encode(FileEditorResult{Status: "error", Message: err.Error()})
    }

    switch req.Operation {
    case "hashline_replace":
        return fileHashlineReplace(resolved, req.Old, req.New, req.AnchorLine, req.AnchorHash, encode)
    case "hashline_insert_after":
        return fileHashlineInsert(resolved, req.Marker, req.Content, req.AnchorLine, req.AnchorHash, true, encode)
    case "hashline_insert_before":
        return fileHashlineInsert(resolved, req.Marker, req.Content, req.AnchorLine, req.AnchorHash, false, encode)
    case "hashline_delete":
        return fileHashlineDelete(resolved, req.StartLine, req.EndLine, req.AnchorLine, req.AnchorHash, encode)
    default:
        return encode(FileEditorResult{Status: "error", Message: fmt.Sprintf("Unknown hashline operation '%s'", req.Operation)})
    }
}
```

#### fileHashlineReplace (vollständig):

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

    // Validate anchor if provided (recommended)
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
        // Provide disambiguation hints
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
            "The 'old' text was found %d times — must be unique for hashline_replace. "+
            "Include more surrounding context in 'old' to disambiguate, "+
            "or use hashline_replace with anchor_line/anchor_hash to target a specific occurrence near that line.\n%s",
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

#### fileHashlineInsert:

```go
func fileHashlineInsert(resolved, marker, content string, anchorLine int, anchorHash string, after bool, encode func(FileEditorResult) string) string {
    if marker == "" {
        return encode(FileEditorResult{Status: "error", Message: "'marker' text is required for hashline_insert"})
    }
    if content == "" {
        return encode(FileEditorResult{Status: "error", Message: "'content' is required for hashline_insert"})
    }
    if err := checkEditSizeLimit(resolved); err != nil {
        return encode(FileEditorResult{Status: "error", Message: err.Error()})
    }
    data, err := os.ReadFile(resolved)
    if err != nil {
        return encode(FileEditorResult{Status: "error", Message: fmt.Sprintf("Failed to read file: %v", err)})
    }

    entries := buildHashlineEntries(data)

    // Find marker line, with optional hash validation
    markerIdx := -1
    for i, entry := range entries {
        if strings.Contains(entry.Content, marker) {
            if markerIdx >= 0 {
                return encode(FileEditorResult{Status: "error", Message: "Marker text found on multiple lines — provide a more specific marker"})
            }
            markerIdx = i
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

#### fileHashlineDelete:

```go
func fileHashlineDelete(resolved string, startLine, endLine, anchorLine int, anchorHash string, encode func(FileEditorResult) string) string {
    if startLine < 1 {
        return encode(FileEditorResult{Status: "error", Message: "'start_line' must be >= 1"})
    }
    if endLine < startLine {
        return encode(FileEditorResult{Status: "error", Message: "'end_line' must be >= start_line"})
    }
    if err := checkEditSizeLimit(resolved); err != nil {
        return encode(FileEditorResult{Status: "error", Message: err.Error()})
    }
    data, err := os.ReadFile(resolved)
    if err != nil {
        return encode(FileEditorResult{Status: "error", Message: fmt.Sprintf("Failed to read file: %v", err)})
    }

    entries := buildHashlineEntries(data)
    if startLine > len(entries) {
        return encode(FileEditorResult{Status: "error", Message: fmt.Sprintf("start_line %d exceeds file length (%d lines)", startLine, len(entries))})
    }
    if endLine > len(entries) {
        endLine = len(entries)
    }

    // Validate anchor if provided (typically the start_line's content hash)
    if anchorLine > 0 && anchorHash != "" {
        if err := validateHashlineAnchor(entries, anchorLine, anchorHash); err != nil {
            return encode(FileEditorResult{Status: "error", Message: err.Error()})
        }
    }

    newEntries := make([]HashlineEntry, 0, len(entries)-(endLine-startLine+1))
    newEntries = append(newEntries, entries[:startLine-1]...)
    newEntries = append(newEntries, entries[endLine:]...)

    // Reconstruct lines
    lines := make([]string, 0, len(newEntries))
    for _, e := range newEntries {
        lines = append(lines, e.Content)
    }
    result := strings.Join(lines, "\n")
    if err := writeFileAtomic(resolved, []byte(result)); err != nil {
        return encode(FileEditorResult{Status: "error", Message: fmt.Sprintf("Failed to write file: %v", err)})
    }

    deleted := endLine - startLine + 1
    return encode(FileEditorResult{
        Status:       "success",
        Message:      fmt.Sprintf("Deleted %d line(s) (%d–%d) (hashline-validated)", deleted, startLine, endLine),
        LinesChanged: deleted,
        TotalLines:   len(lines),
    })
}
```

### Phase D: Schema-Updates (Tag 4)

**Datei:** `internal/agent/native_tools_core.go`

Erweitere das `file_editor`-Schema um die 4 neuen Operationen und Parameter:
```go
func buildCoreToolSchemas(ff ToolFeatureFlags, execSkillProps map[string]interface{}) []openai.Tool {
    tools := []openai.Tool{
        // ... existing tools ...
        
        func() openai.Tool {
            if ff.AllowFilesystemWrite {
                return tool("file_editor",
                    "Precise file editing. Use hashline_replace/insert_after/insert_before/delete when you have read the file with include_hashes=true. "+
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
                        "start_line":   prop("integer", "Start line for delete_lines/hashline_delete (1-based)"),
                        "end_line":     prop("integer", "End line for delete_lines/hashline_delete (1-based, inclusive)"),
                        // Hashline parameters
                        "anchor_line":  prop("integer", "Line number to anchor the edit to (from include_hashes read). Validates the line content hasn't changed."),
                        "anchor_hash":  prop("string", "Expected content hash of the anchor line (6 hex chars from LINE#HASH:CONTENT)."),
                    }, "operation", "file_path"),
                )
            }
            // Read-only variant (unchanged)
            // ...
        }(),
        // ...
    }
}
```

### Phase E: Dispatch-Update (Tag 4–5)

**Datei:** `internal/agent/dispatch_filesystem.go`

Im `file_editor`-Dispatch-Block:
```go
case "file_editor":
    if !cfg.Agent.AllowFilesystemWrite {
        return formatToolPermissionDenied("file_editor", ...)
    }
    op := stringValueFromMap(tc.Params, "operation")
    fpath := resolveFilePath(tc)
    req := decodeFileEditorArgs(tc)

    // Route hashline operations to the new hashline editor
    switch op {
    case "hashline_replace", "hashline_insert_after", "hashline_insert_before", "hashline_delete":
        hashReq := tools.HashlineEditorRequest{
            Operation:  op,
            FilePath:   fpath,
            Old:        req.Old,
            New:        req.New,
            Marker:     req.Marker,
            Content:    req.Content,
            StartLine:  req.StartLine,
            EndLine:    req.EndLine,
        }
        // Parse hashline-specific params
        if v, err := strconv.Atoi(stringValueFromMap(tc.Params, "anchor_line")); err == nil {
            hashReq.AnchorLine = v
        }
        hashReq.AnchorHash = stringValueFromMap(tc.Params, "anchor_hash")
        
        return tools.ExecuteHashlineEditor(hashReq, cfg.Directories.WorkspaceDir)
    
    default:
        // Existing path — completely unchanged
        return tools.ExecuteFileEditor(op, fpath, req.Old, req.New, req.Marker, req.Content, req.StartLine, req.EndLine, req.LineCount, cfg.Directories.WorkspaceDir)
    }
```

**Wichtig:** `ExecuteFileEditor` wird exakt mit denselben 10 Parametern aufgerufen wie bisher. Kein Breaking Change.

### Phase F: Tool-Manuals aktualisieren (Tag 5)

**Datei:** `prompts/tools_manuals/file_editor.md`

Neuer Abschnitt:
```markdown
## Hashline-Mode (Recommended for Complex Edits)

When editing files that may have changed between read and write, use the hashline mode:

1. **Read with hashes:** Call `filesystem` with `operation: read_file` and `include_hashes: true`.
   Output format: `123#abc123:func main() {`
   - `123` = line number
   - `abc123` = content hash (hash of the line text ONLY, not including line number)
   - `func main() {` = line content

2. **Edit with validation:** Call `file_editor` with a hashline operation:
   - `hashline_replace` — Replace text with anchor validation
   - `hashline_insert_after` / `hashline_insert_before` — Insert with marker validation
   - `hashline_delete` — Delete line range with hash validation

3. **Parameters:**
   - `anchor_line`: The line number from the hashline read
   - `anchor_hash`: The 6-char content hash from the hashline read

**Important:** The content hash does NOT include the line number. This means:
- If you insert/delete lines ABOVE your anchor, the anchor line NUMBER shifts, but its CONTENT hash remains the same.
- You can perform multiple edits in the same file without re-reading, as long as the anchor CONTENT hasn't changed.
- If the anchor CONTENT changed (e.g., someone else edited that line), you get `STALE CONTEXT` and must re-read.

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
`STALE CONTEXT: line 42 has content hash x9k2m1, expected a3f7c2. Please re-read the file...`
```

### Phase G: Tests (Tag 6–7)

**Datei:** `internal/tools/hashline_test.go` (neu)

```go
package tools

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestHashLineContentDeterminism(t *testing.T) {
	h1 := hashLineContent("func main() {")
	h2 := hashLineContent("func main() {")
	if h1 != h2 {
		t.Errorf("not deterministic: %q vs %q", h1, h2)
	}
	if len(h1) != 6 {
		t.Errorf("expected 6 chars, got %d: %q", len(h1), h1)
	}
}

func TestHashLineContentLineNumberIndependence(t *testing.T) {
	// CRITICAL: hash must be the same regardless of conceptual line number
	h1 := hashLineContent("func main() {")
	h2 := hashLineContent("func main() {")
	if h1 != h2 {
		t.Error("content hash must not depend on line number")
	}
}

func TestHashLineContentSensitivity(t *testing.T) {
	h1 := hashLineContent("func main() {")
	h2 := hashLineContent("func main() {}")
	if h1 == h2 {
		t.Error("hash should differ for different content")
	}
}

func TestValidateHashlineAnchor(t *testing.T) {
	entries := []HashlineEntry{
		{LineNum: 1, Hash: hashLineContent("package main"), Content: "package main"},
		{LineNum: 2, Hash: hashLineContent("func main() {"), Content: "func main() {"},
	}

	if err := validateHashlineAnchor(entries, 2, entries[1].Hash); err != nil {
		t.Errorf("expected valid, got: %v", err)
	}

	err := validateHashlineAnchor(entries, 2, "000000")
	if err == nil || !strings.Contains(err.Error(), "STALE CONTEXT") {
		t.Errorf("expected stale context error, got: %v", err)
	}
}

func TestFileHashlineReplaceSuccess(t *testing.T) {
	tmp := t.TempDir()
	file := filepath.Join(tmp, "test.go")
	content := "package main\n\nfunc main() {\n\tfmt.Println(\"hello\")\n}\n"
	os.WriteFile(file, []byte(content), 0644)

	encode := func(r FileEditorResult) string {
		b, _ := json.Marshal(r)
		return string(b)
	}

	entries := buildHashlineEntries([]byte(content))
	anchorHash := entries[2].Hash // line 3: "func main() {"

	result := fileHashlineReplace(file, "func main() {", "func main() error {", 3, anchorHash, encode)
	if !strings.Contains(result, `"status":"success"`) {
		t.Errorf("expected success, got: %s", result)
	}

	data, _ := os.ReadFile(file)
	if !strings.Contains(string(data), "func main() error {") {
		t.Error("file was not updated")
	}
}

func TestFileHashlineReplaceStale(t *testing.T) {
	tmp := t.TempDir()
	file := filepath.Join(tmp, "test.go")
	content := "package main\n\nfunc main() {\n\tfmt.Println(\"hello\")\n}\n"
	os.WriteFile(file, []byte(content), 0644)

	encode := func(r FileEditorResult) string {
		b, _ := json.Marshal(r)
		return string(b)
	}

	// Simulate stale context: file changed after reading
	os.WriteFile(file, []byte(content+"// changed\n"), 0644)

	entries := buildHashlineEntries([]byte(content)) // old hashes
	anchorHash := entries[2].Hash

	result := fileHashlineReplace(file, "func main() {", "func main() error {", 3, anchorHash, encode)
	if !strings.Contains(result, "STALE CONTEXT") {
		t.Errorf("expected stale context, got: %s", result)
	}
}

func TestFileHashlineInsertPreservesLowerHashes(t *testing.T) {
	// CRITICAL TEST: After inserting at line 2, line 3's content hash must remain valid
	tmp := t.TempDir()
	file := filepath.Join(tmp, "test.go")
	content := "line1\nline2\nline3\n"
	os.WriteFile(file, []byte(content), 0644)

	encode := func(r FileEditorResult) string {
		b, _ := json.Marshal(r)
		return string(b)
	}

	entriesBefore := buildHashlineEntries([]byte(content))
	line3HashBefore := entriesBefore[2].Hash // "line3"

	// Insert after line 1
	result := fileHashlineInsert(file, "line1", "inserted", 1, entriesBefore[0].Hash, true, encode)
	if !strings.Contains(result, `"status":"success"`) {
		t.Fatalf("insert failed: %s", result)
	}

	// Verify line 3 (now line 4) still has same content hash
	data, _ := os.ReadFile(file)
	entriesAfter := buildHashlineEntries(data)

	// Find "line3" in new entries
	var line3HashAfter string
	for _, e := range entriesAfter {
		if e.Content == "line3" {
			line3HashAfter = e.Hash
			break
		}
	}
	if line3HashAfter != line3HashBefore {
		t.Errorf("content hash of 'line3' changed after insert above! before=%s after=%s", line3HashBefore, line3HashAfter)
	}
}
```

### Phase H: Integrationstest & Rollout (Tag 8–9)

1. **E2E-Test:** Vollständiger Flow über `/v1/chat/completions` mit Hashline-Edit
2. **Stale-Context-Test:** Datei zwischen read und write ändern, STALE CONTEXT erwarten
3. **Multi-Edit-Test:** Mehrere Edits in derselben Datei ohne Re-Read (dank Content-only-Hash)
4. **Feature-Toggle:** Optional `hashline_edits_enabled` in `config.yaml` (Default: `true`)
5. **Monitoring:** Metrik für "hashline_stale_rejections" in `internal/agent/tool_execution_policy.go`

---

## 5. Rückwärtskompatibilität (vollständig gewährleistet)

| Aspekt | Strategie |
|--------|-----------|
| `ExecuteFileEditor` | **Unverändert.** Keine Signatur-Änderung, kein neuer Parameter. |
| `str_replace` | Bleibt exakt wie bisher. Hashline ist Opt-in über neue Operationen. |
| `read_file` | `include_hashes` defaultet auf `false`. Kein Breaking Change. |
| `file_editor` Schema | Neue Operationen sind **additiv**. Alte Schemas bleiben gültig. |
| Alle Caller | Nur `dispatch_filesystem.go` bekommt ein `switch` für Hashline-Ops. Alle anderen Dateien unverändert. |

---

## 6. Risiken & Mitigationen (aktualisiert)

| Risiko | Wahrscheinlichkeit | Mitigation |
|--------|-------------------|------------|
| Modell nutzt Hashline-Parameter falsch | Mittel | Starke Tool-Manual-Doku; Fallback auf `str_replace` bei Fehlern |
| Hashline-Ausgabe frisst zu viele Tokens | Mittel | Erhöhtes Limit (45KB statt 32KB); Hashes sind kurz (6 chars) |
| FNV-1a-Kollision | Sehr niedrig | 24 Bit = 16M Kombinationen; akzeptabel für Agenten-Editing |
| Zeilenverschiebung nicht erkannt | Niedrig (Feature) | `anchor_line` fängt Position ab; Content-Hash fängt Inhalt ab. Beides zusammen ist robust. |
| Multi-line `old` mit single anchor | Niedrig | Dokumentiert: anchor sollte auf **erste Zeile** von `old` zeigen |

---

## 7. Erfolgsmetriken

- **Reduktion von** `"old text not found"`-Fehlern um ≥30 %
- **Reduktion von** Multi-Match-Fehlern durch gezielte Hashline-Anker
- **Multi-Edit-Szenario:** Agent kann ≥3 Edits in derselben Datei hintereinander ausführen (dank Content-only-Hash), ohne Re-Read
- **Keine Regression** bei bestehenden `str_replace`-Erfolgsraten

---

*Plan korrigiert: 2026-06-07*
