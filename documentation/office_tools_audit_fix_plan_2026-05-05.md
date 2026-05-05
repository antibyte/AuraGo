# Office Tools Audit Fixes Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Close the verified Office tool security, data-preservation, and safety gaps from `reports/office_tools_audit_2026-05-05.md`.

**Architecture:** Keep the current Office backend split: `internal/office` owns document/workbook encoding, `internal/tools` owns agent-facing operation policy, and `internal/desktop` remains the workspace storage boundary. Add missing policy checks before Office convenience forwarding, make document/workbook mutations fail safe, and expose Office read-only controls through the existing Virtual Desktop config UI.

**Tech Stack:** Go standard library, existing `internal/office` package, virtual desktop service APIs, vanilla JS config UI, embedded JSON translations, Go unit and marker tests.

---

## Context

The report was reviewed against the current code on 2026-05-05. The report itself remains in `reports/` and must not be committed.

Current relevant files:

- Modify `internal/tools/virtual_desktop.go`
- Modify `internal/tools/office_tools.go`
- Modify `internal/tools/office_tools_test.go`
- Modify `internal/tools/virtual_desktop_test.go`
- Modify `internal/tools/document_creator.go`
- Modify `internal/tools/document_creator_test.go`
- Modify `internal/office/office.go`
- Modify `internal/office/office_test.go`
- Modify `internal/config/config_types.go`
- Modify `config_template.yaml`
- Modify `internal/agent/tooling_policy.go` only if feature flag behavior changes during implementation
- Modify `internal/agent/native_tools_integrations.go`
- Modify `internal/agent/tool_catalog_test.go`
- Modify `ui/cfg/virtual_desktop.js`
- Modify `ui/desktop_office_assets_test.go`
- Modify `ui/lang/desktop/*.json` for all 16 desktop locales
- Modify `prompts/tools_manuals/office_document.md`
- Modify `prompts/tools_manuals/office_workbook.md`

## Audit Validation

Confirmed defects:

- `ExecuteVirtualDesktop` forwards `read_document`, `write_document`, `patch_document`, `read_workbook`, `write_workbook`, `set_cell`, `set_range`, and `evaluate_formula` without checking `tools.office_document.enabled` or `tools.office_workbook.enabled`.
- `patch_document` updates `doc.Text` and then sets `doc.HTML = ""`, so a rich DOCX/HTML representation is discarded.
- `officeReadWorkbookOrEmpty` swallows every read or decode error and returns an empty workbook, so `set_cell` and `set_range` can overwrite corrupt or unsupported workbooks.
- `virtual_desktop` `export_file` writes through `WriteFileBytes`, while dedicated Office export paths use `WriteFileBytesConditional`. Today both route through the same lock/read-only path, but the direct call makes future concurrency checks inconsistent.
- `buildHTMLFromSections` passes `content` through as raw HTML for `create_pdf` with the Gotenberg backend. The native schema describes this as plain text for `create_pdf`, so it should be escaped there. Explicit `html_to_pdf` and `screenshot_html` operations remain raw HTML inputs.
- `tools.office_document` and `tools.office_workbook` have only `enabled`, with no granular read-only mode.
- `officeToolVersion` returns both `modified` and `mod_time` with identical values.
- Workbook export with no `format` and an `output_path` without an extension fails with an empty format instead of defaulting to XLSX.
- `officeToolRawString` accepts arbitrary non-string values through `fmt.Sprint`.
- `parseSourceFiles` returns `nil` on invalid JSON, producing a generic "source_files required" error instead of telling the user the JSON is malformed.
- Missing regression tests for virtual-desktop Office gates, HTML patch preservation, corrupt workbook mutation, default XLSX export, invalid `source_files`, and schema path requirements.

Partially stale or needs correction:

- The dedicated `office_document` and `office_workbook` schemas already call `schema(..., "operation", "path")` in `internal/agent/native_tools_integrations.go`. Add a regression test to lock this in.
- The generic `virtual_desktop` schema cannot make `path` globally required because `status`, `bootstrap`, `open_app`, `show_notification`, and some app/widget operations do not require `path`. Improve the `path` description if needed, but do not add a global required `path`.
- The project currently has 16 desktop locale files: `cs`, `da`, `de`, `el`, `en`, `es`, `fr`, `hi`, `it`, `ja`, `nl`, `no`, `pl`, `pt`, `sv`, `zh`.

## Implementation Decisions

- Treat disabled Office document/workbook tools as hard gates, even when the operation arrives through `virtual_desktop`.
- Add Office-tool read-only controls in `tools.office_document.readonly` and `tools.office_workbook.readonly`. This is separate from `virtual_desktop.readonly`: the desktop can remain writable while Office tools are read-only.
- Allow read-only document operations only for `read`. Block `write`, `patch`, and `export`.
- Allow read-only workbook operations for `read` and `evaluate_formula`. Block `write`, `set_cell`, `set_range`, and `export`.
- For document patching, keep `doc.Text` as the source of truth and make `doc.HTML` non-stale. If the document has HTML, patch simple replacements in the HTML representation where possible; otherwise regenerate HTML from the patched plain text through an exported `office.TextToHTML`.
- For workbook patch-style mutations, allow a missing target path to create a new workbook. Any other read/decode error must return an error and must not write.
- Keep `modified` and `mod_time` for this pass as a compatibility alias. Document them as equivalent instead of removing either field in a bug-fix wave.
- Escape `create_pdf` plain content in `buildHTMLFromSections`. Keep raw HTML only for explicitly named HTML operations.

## Wave 1: Policy Gates And Read-Only Config

### Task 1: Add failing tests for virtual-desktop Office gates

**Files:**

- Modify `internal/tools/office_tools_test.go`
- Modify `internal/tools/virtual_desktop_test.go`

- [ ] Add this test to `internal/tools/office_tools_test.go`:

```go
func TestExecuteVirtualDesktopOfficeConvenienceRespectsOfficeToolToggles(t *testing.T) {
	t.Parallel()

	cfg := testVirtualDesktopConfig(t)

	docExec := ExecuteVirtualDesktop(context.Background(), cfg, map[string]interface{}{
		"operation": "patch_document",
		"path":      "Documents/blocked.md",
		"content":   "blocked",
	})
	if !strings.Contains(docExec.Output, `"status":"error"`) || !strings.Contains(docExec.Output, "office_document tool is disabled") {
		t.Fatalf("patch_document should be blocked by office_document toggle, got %s", docExec.Output)
	}

	workbookExec := ExecuteVirtualDesktop(context.Background(), cfg, map[string]interface{}{
		"operation": "set_cell",
		"path":      "Documents/blocked.xlsx",
		"cell":      "A1",
		"value":     "blocked",
	})
	if !strings.Contains(workbookExec.Output, `"status":"error"`) || !strings.Contains(workbookExec.Output, "office_workbook tool is disabled") {
		t.Fatalf("set_cell should be blocked by office_workbook toggle, got %s", workbookExec.Output)
	}
}
```

- [ ] Update existing virtual-desktop Office convenience tests so they explicitly opt in:

```go
cfg.Tools.OfficeDocument.Enabled = true
cfg.Tools.OfficeWorkbook.Enabled = true
```

- [ ] Run `go test ./internal/tools -run "Office|VirtualDesktopOffice" -count=1`.
  Expected before implementation: the new gate test fails because convenience operations still run.

### Task 2: Gate forwarded Office operations in `ExecuteVirtualDesktop`

**Files:**

- Modify `internal/tools/virtual_desktop.go`

- [ ] Change the Office forwarding cases to check the matching Office tool before calling the shared operation helpers:

```go
	case "read_document", "write_document", "patch_document":
		if err := officeToolAllowed(cfg, "document", op); err != nil {
			return virtualDesktopJSON("error", err.Error(), nil, nil)
		}
		return executeOfficeDocumentOperation(ctx, svc, args, op)
	case "read_workbook", "write_workbook", "set_cell", "set_range", "evaluate_formula":
		if err := officeToolAllowed(cfg, "workbook", op); err != nil {
			return virtualDesktopJSON("error", err.Error(), nil, nil)
		}
		return executeOfficeWorkbookOperation(ctx, svc, args, op)
```

- [ ] This step depends on Task 3 changing `officeToolAllowed` to accept `op`.
- [ ] Run `go test ./internal/tools -run "Office|VirtualDesktopOffice" -count=1`.
  Expected after Task 3: Office convenience gate tests pass.

### Task 3: Add Office tool read-only policy

**Files:**

- Modify `internal/config/config_types.go`
- Modify `config_template.yaml`
- Modify `internal/tools/office_tools.go`
- Modify `internal/tools/office_tools_test.go`
- Modify `prompts/tools_manuals/office_document.md`
- Modify `prompts/tools_manuals/office_workbook.md`

- [ ] Add `ReadOnly` to both tool config structs:

```go
		OfficeDocument struct {
			Enabled  bool `yaml:"enabled"`  // enable office_document tool (agent-safe Writer document operations)
			ReadOnly bool `yaml:"readonly"` // true = only read documents, block write/patch/export
		} `yaml:"office_document"`
		OfficeWorkbook struct {
			Enabled  bool `yaml:"enabled"`  // enable office_workbook tool (agent-safe spreadsheet operations)
			ReadOnly bool `yaml:"readonly"` // true = only read/evaluate, block write/set/export
		} `yaml:"office_workbook"`
```

- [ ] Add the defaults to `config_template.yaml`:

```yaml
    office_document:
        enabled: false                                     # enable office_document tool; also requires virtual_desktop.enabled and allow_agent_control
        readonly: false                                    # read-only mode: allow read only; block write, patch, and export
    office_workbook:
        enabled: false                                     # enable office_workbook tool; also requires virtual_desktop.enabled and allow_agent_control
        readonly: false                                    # read-only mode: allow read/evaluate only; block write, set_cell, set_range, and export
```

- [ ] Change `ExecuteOfficeDocument` and `ExecuteOfficeWorkbook` to parse `op` before calling `officeToolAllowed`:

```go
	op := strings.ToLower(strings.TrimSpace(virtualDesktopString(args, "operation", "action_type")))
	if op == "" {
		op = "read"
	}
	if err := officeToolAllowed(cfg, "document", op); err != nil {
		return virtualDesktopJSON("error", err.Error(), nil, nil)
	}
```

- [ ] Update `officeToolAllowed` and add a mutation helper:

```go
func officeToolAllowed(cfg *config.Config, kind, op string) error {
	if cfg == nil {
		return fmt.Errorf("configuration is unavailable")
	}
	if !cfg.VirtualDesktop.Enabled {
		return fmt.Errorf("virtual desktop is disabled in config")
	}
	if !cfg.VirtualDesktop.AllowAgentControl {
		return fmt.Errorf("agent control for the virtual desktop is disabled in config")
	}
	switch kind {
	case "document":
		if !cfg.Tools.OfficeDocument.Enabled {
			return fmt.Errorf("office_document tool is disabled in config")
		}
		if cfg.Tools.OfficeDocument.ReadOnly && officeToolMutates(kind, op) {
			return fmt.Errorf("office_document tool is in read-only mode")
		}
	case "workbook":
		if !cfg.Tools.OfficeWorkbook.Enabled {
			return fmt.Errorf("office_workbook tool is disabled in config")
		}
		if cfg.Tools.OfficeWorkbook.ReadOnly && officeToolMutates(kind, op) {
			return fmt.Errorf("office_workbook tool is in read-only mode")
		}
	}
	return nil
}

func officeToolMutates(kind, op string) bool {
	op = strings.ToLower(strings.TrimSpace(op))
	switch kind {
	case "document":
		return op != "read" && op != "read_document"
	case "workbook":
		return op != "read" && op != "read_workbook" && op != "evaluate_formula"
	default:
		return true
	}
}
```

- [ ] Add tests:
  - `office_document` read works in read-only mode after a document is created before enabling read-only.
  - `office_document` write, patch, and export return a read-only error.
  - `office_workbook` read and `evaluate_formula` work in read-only mode after a workbook is created before enabling read-only.
  - `office_workbook` write, `set_cell`, `set_range`, and export return a read-only error.
- [ ] Update both tool manuals to mention the new read-only modes and allowed operations.
- [ ] Run `go test ./internal/tools -run Office -count=1`.
  Expected: read-only policy tests pass.

### Task 4: Expose Office tool toggles in Virtual Desktop config UI

**Files:**

- Modify `ui/cfg/virtual_desktop.js`
- Modify `ui/desktop_office_assets_test.go`
- Modify `ui/lang/desktop/*.json`

- [ ] Initialize Office tool config in `vdCfgEnsureData()`:

```js
    if (!configData.tools.office_document) configData.tools.office_document = {};
    if (!configData.tools.office_workbook) configData.tools.office_workbook = {};
```

- [ ] Add an Office subsection after the agent-control toggles:

```js
    html += '<div class="cfg-note-banner cfg-note-banner-info">' + t('config.virtual_desktop.office_tools_note') + '</div>';
    html += '<div class="field-grid two-cols">';
    html += vdCfgToggleRow('config.virtual_desktop.office_document_label', 'help.virtual_desktop.office_document', configData.tools.office_document.enabled === true, 'tools.office_document.enabled');
    html += vdCfgToggleRow('config.virtual_desktop.office_document_readonly_label', 'help.virtual_desktop.office_document_readonly', configData.tools.office_document.readonly === true, 'tools.office_document.readonly');
    html += vdCfgToggleRow('config.virtual_desktop.office_workbook_label', 'help.virtual_desktop.office_workbook', configData.tools.office_workbook.enabled === true, 'tools.office_workbook.enabled');
    html += vdCfgToggleRow('config.virtual_desktop.office_workbook_readonly_label', 'help.virtual_desktop.office_workbook_readonly', configData.tools.office_workbook.readonly === true, 'tools.office_workbook.readonly');
    html += '</div>';
```

- [ ] Add marker checks to `ui/desktop_office_assets_test.go` for:
  - `tools.office_document.enabled`
  - `tools.office_document.readonly`
  - `tools.office_workbook.enabled`
  - `tools.office_workbook.readonly`
  - `config.virtual_desktop.office_tools_note`
- [ ] Add these keys with non-empty values to all 16 `ui/lang/desktop/*.json` files:
  - `config.virtual_desktop.office_tools_note`
  - `config.virtual_desktop.office_document_label`
  - `help.virtual_desktop.office_document`
  - `config.virtual_desktop.office_document_readonly_label`
  - `help.virtual_desktop.office_document_readonly`
  - `config.virtual_desktop.office_workbook_label`
  - `help.virtual_desktop.office_workbook`
  - `config.virtual_desktop.office_workbook_readonly_label`
  - `help.virtual_desktop.office_workbook_readonly`
- [ ] Extend `TestDesktopOfficeI18NKeys` to include those keys.
- [ ] Run `go test ./ui -run Office -count=1`.
  Expected: UI marker and i18n tests pass.

## Wave 2: Document Patch Data Preservation

### Task 5: Add failing tests for HTML-preserving document patch

**Files:**

- Modify `internal/tools/office_tools_test.go`
- Modify `internal/office/office_test.go`

- [ ] Add an Office package test for exported HTML conversion:

```go
func TestTextToHTMLEscapesPlainText(t *testing.T) {
	t.Parallel()

	got := TextToHTML("Hello <Agent>\nSecond")
	for _, want := range []string{"Hello &lt;Agent&gt;", "<p>Second</p>"} {
		if !strings.Contains(got, want) {
			t.Fatalf("TextToHTML missing %q in %q", want, got)
		}
	}
}
```

- [ ] Add a tool-level test that patches an HTML-backed document and asserts the resulting `document.html` still contains the patched value:

```go
func TestExecuteOfficeDocumentPatchKeepsHTMLRepresentation(t *testing.T) {
	t.Parallel()

	cfg := testVirtualDesktopConfig(t)
	cfg.Tools.OfficeDocument.Enabled = true

	write := ExecuteOfficeDocument(context.Background(), cfg, map[string]interface{}{
		"operation": "write",
		"path":      "Documents/formatted.html",
		"title":     "Formatted",
		"html":      "<!doctype html><body><h1>Formatted</h1><p>Hello <strong>World</strong></p></body>",
	})
	if !strings.Contains(write.Output, `"status":"ok"`) {
		t.Fatalf("write output=%s", write.Output)
	}

	patch := ExecuteOfficeDocument(context.Background(), cfg, map[string]interface{}{
		"operation":    "patch",
		"path":         "Documents/formatted.html",
		"replacements": []interface{}{map[string]interface{}{"find": "World", "replace": "Agent"}},
	})
	var payload struct {
		Status string `json:"status"`
		Data   struct {
			Document struct {
				Text string `json:"text"`
				HTML string `json:"html"`
			} `json:"document"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(patch.Output), &payload); err != nil {
		t.Fatalf("decode patch payload: %v output=%s", err, patch.Output)
	}
	if payload.Status != "ok" || !strings.Contains(payload.Data.Document.Text, "Hello Agent") {
		t.Fatalf("patch text payload = %+v output=%s", payload, patch.Output)
	}
	if !strings.Contains(payload.Data.Document.HTML, "Agent") || strings.TrimSpace(payload.Data.Document.HTML) == "" {
		t.Fatalf("patch should keep non-empty patched HTML, got %q", payload.Data.Document.HTML)
	}
}
```

- [ ] Run `go test ./internal/office ./internal/tools -run "TextToHTML|PatchKeepsHTML" -count=1`.
  Expected before implementation: `TextToHTML` is undefined and/or patch HTML is empty.

### Task 6: Export HTML conversion and update patch behavior

**Files:**

- Modify `internal/office/office.go`
- Modify `internal/tools/office_tools.go`

- [ ] Add a small exported wrapper in `internal/office/office.go`:

```go
// TextToHTML converts plain text into AuraGo's minimal safe HTML document format.
func TextToHTML(text string) string {
	return textToHTML(text)
}
```

- [ ] Replace the current patch block:

```go
doc.Text = officePatchText(doc.Text, args)
doc.HTML = ""
```

with:

```go
doc.Text = officePatchText(doc.Text, args)
doc.HTML = officePatchHTML(doc.HTML, doc.Text, args)
```

- [ ] Add `officePatchHTML` in `internal/tools/office_tools.go`:

```go
func officePatchHTML(htmlText, patchedText string, args map[string]interface{}) string {
	if strings.TrimSpace(htmlText) == "" {
		return office.TextToHTML(patchedText)
	}
	patchedHTML := htmlText
	for _, replacement := range officeToolReplacements(args["replacements"]) {
		patchedHTML = strings.ReplaceAll(patchedHTML, replacement.find, replacement.replace)
	}
	if strings.Contains(patchedHTML, officeToolRawString(args, "prepend_text")) || strings.Contains(patchedHTML, officeToolRawString(args, "append_text")) {
		return patchedHTML
	}
	return patchedHTML
}
```

- [ ] During implementation, prefer escaping any inserted prepend/append text as HTML paragraphs if adding prepend/append support to `officePatchHTML`. Do not concatenate unescaped patch text into HTML.
- [ ] Run `go test ./internal/office ./internal/tools -run "TextToHTML|PatchKeepsHTML|OfficeDocumentWritePatchReadExport" -count=1`.
  Expected: document patch tests pass and existing patch/read/export coverage remains green.

## Wave 3: Workbook Mutation Safety And Export Defaults

### Task 7: Add failing tests for corrupt workbook mutation and default export

**Files:**

- Modify `internal/tools/office_tools_test.go`

- [ ] Add a corrupt-workbook mutation test:

```go
func TestExecuteOfficeWorkbookSetCellDoesNotOverwriteUnreadableWorkbook(t *testing.T) {
	t.Parallel()

	cfg := testVirtualDesktopConfig(t)
	cfg.Tools.OfficeWorkbook.Enabled = true

	svc, err := officeToolService(context.Background(), cfg)
	if err != nil {
		t.Fatalf("officeToolService: %v", err)
	}
	defer svc.Close()
	if err := svc.WriteFileBytes(context.Background(), "Documents/corrupt.xlsx", []byte("not an xlsx"), desktop.SourceAgent); err != nil {
		t.Fatalf("seed corrupt workbook: %v", err)
	}

	exec := ExecuteOfficeWorkbook(context.Background(), cfg, map[string]interface{}{
		"operation": "set_cell",
		"path":      "Documents/corrupt.xlsx",
		"cell":      "A1",
		"value":     "new",
	})
	if !strings.Contains(exec.Output, `"status":"error"`) {
		t.Fatalf("expected corrupt workbook mutation to fail, got %s", exec.Output)
	}
}
```

- [ ] Add a default export test:

```go
func TestExecuteOfficeWorkbookExportDefaultsToXLSXWhenTargetHasNoExtension(t *testing.T) {
	t.Parallel()

	cfg := testVirtualDesktopConfig(t)
	cfg.Tools.OfficeWorkbook.Enabled = true
	write := ExecuteOfficeWorkbook(context.Background(), cfg, map[string]interface{}{
		"operation": "set_cell",
		"path":      "Documents/budget.xlsx",
		"cell":      "A1",
		"value":     "Amount",
	})
	if !strings.Contains(write.Output, `"status":"ok"`) {
		t.Fatalf("write output=%s", write.Output)
	}

	exported := ExecuteOfficeWorkbook(context.Background(), cfg, map[string]interface{}{
		"operation":   "export",
		"path":        "Documents/budget.xlsx",
		"output_path": "Documents/budget-copy",
	})
	if !strings.Contains(exported.Output, `"status":"ok"`) {
		t.Fatalf("export should default to xlsx, got %s", exported.Output)
	}
}
```

- [ ] Add `aurago/internal/desktop` to the test imports as needed.
- [ ] Run `go test ./internal/tools -run "Workbook.*Overwrite|WorkbookExportDefaults" -count=1`.
  Expected before implementation: corrupt workbook mutation writes or returns ok; no-extension export fails.

### Task 8: Fail safely on workbook read errors and default exports to XLSX

**Files:**

- Modify `internal/tools/office_tools.go`

- [ ] Replace `officeReadWorkbookOrEmpty` with an error-returning helper:

```go
func officeReadWorkbookOrNew(ctx context.Context, svc *desktop.Service, rawPath string) (office.Workbook, error) {
	workbook, _, _, err := officeReadWorkbook(ctx, svc, rawPath)
	if err == nil {
		return workbook, nil
	}
	if errors.Is(err, os.ErrNotExist) {
		return office.Workbook{Path: rawPath}, nil
	}
	return office.Workbook{}, err
}
```

- [ ] Add `errors` and `os` imports in `internal/tools/office_tools.go`.
- [ ] Change `set_cell` and `set_range` to call `officeReadWorkbookOrNew` and return the read/decode error:

```go
workbook, err := officeReadWorkbookOrNew(ctx, svc, path)
if err != nil {
	return virtualDesktopJSON("error", err.Error(), nil, nil)
}
```

- [ ] In `officeToolExportWorkbook`, default an empty output extension to `.xlsx`:

```go
if outputExt == "" {
	outputExt = ".xlsx"
}
```

- [ ] Apply the same empty-extension default in `virtualDesktopExportOffice` for workbook exports if the generic `virtual_desktop` `export_file` path is meant to handle spreadsheets directly.
- [ ] Run `go test ./internal/tools -run "Workbook|VirtualDesktopOffice" -count=1`.
  Expected: workbook mutation safety and export default tests pass.

### Task 9: Use conditional writes for generic virtual desktop export

**Files:**

- Modify `internal/tools/virtual_desktop.go`
- Modify `internal/tools/virtual_desktop_test.go`

- [ ] Replace the direct export write:

```go
if err := svc.WriteFileBytes(ctx, outputPath, exported, desktop.SourceAgent); err != nil {
	return virtualDesktopJSON("error", err.Error(), nil, nil)
}
```

with:

```go
outEntry, err := svc.WriteFileBytesConditional(ctx, outputPath, exported, desktop.SourceAgent, nil)
if err != nil {
	return virtualDesktopJSON("error", err.Error(), nil, nil)
}
```

- [ ] Return `outEntry.Path` and `office_version` in the payload, matching dedicated Office export responses:

```go
return virtualDesktopJSON("ok", "desktop office file exported", map[string]interface{}{
	"path":           entry.Path,
	"output_path":    outEntry.Path,
	"entry":          outEntry,
	"office_version": officeToolVersionForEntry(outEntry, exported),
}, event)
```

- [ ] Add or update a virtual desktop export test that verifies `output_path` is the normalized returned entry path.
- [ ] Run `go test ./internal/tools -run VirtualDesktop -count=1`.
  Expected: virtual desktop export behavior still passes with richer metadata.

## Wave 4: Input Validation And Document Creator Safety

### Task 10: Restrict patch text arguments to strings

**Files:**

- Modify `internal/tools/office_tools.go`
- Modify `internal/tools/office_tools_test.go`

- [ ] Replace `officeToolRawString` with a string-only helper:

```go
func officeToolRawString(args map[string]interface{}, key string) string {
	raw, ok := args[key]
	if !ok || raw == nil {
		return ""
	}
	value, ok := raw.(string)
	if !ok {
		return ""
	}
	return value
}
```

- [ ] Add a patch test where `append_text` is numeric and assert it is ignored rather than stringified.
- [ ] Run `go test ./internal/tools -run OfficeDocument -count=1`.
  Expected: string-only behavior passes and existing patch tests remain green.

### Task 11: Return explicit errors for invalid `source_files` JSON

**Files:**

- Modify `internal/tools/document_creator.go`
- Modify `internal/tools/document_creator_test.go`

- [ ] Change `parseSourceFiles` to return paths and an error:

```go
func parseSourceFiles(jsonStr string) ([]string, error) {
	if strings.TrimSpace(jsonStr) == "" {
		return nil, nil
	}
	var paths []string
	if err := json.Unmarshal([]byte(jsonStr), &paths); err != nil {
		return nil, fmt.Errorf("invalid source_files JSON: %w", err)
	}
	return paths, nil
}
```

- [ ] Update each caller to handle the error before checking `len(paths)`:

```go
paths, err := parseSourceFiles(sourceFilesJSON)
if err != nil {
	return fmt.Sprintf(`{"status":"error","message":"%s"}`, escapeJSONMessage(err.Error()))
}
```

- [ ] Add a tiny helper if needed to JSON-escape dynamic error messages:

```go
func escapeJSONMessage(message string) string {
	data, _ := json.Marshal(message)
	return strings.Trim(string(data), `"`)
}
```

- [ ] Update `TestParseSourceFiles` so non-JSON input expects an error containing `invalid source_files JSON`.
- [ ] Add an `ExecuteDocumentCreatorInWorkspace` test that passes malformed JSON and asserts the returned JSON message mentions `invalid source_files JSON`.
- [ ] Run `go test ./internal/tools -run "DocumentCreator|ParseSourceFiles" -count=1`.
  Expected: document creator validation tests pass.

### Task 12: Escape `create_pdf` plain content for the Gotenberg HTML builder

**Files:**

- Modify `internal/tools/document_creator.go`
- Modify `internal/tools/document_creator_test.go`

- [ ] Add a failing test:

```go
func TestBuildHTMLFromSectionsEscapesPlainContent(t *testing.T) {
	t.Parallel()

	html := buildHTMLFromSections("Title", `<script>alert("x")</script>`, "")
	if strings.Contains(html, "<script>") {
		t.Fatalf("plain create_pdf content should be escaped, got %s", html)
	}
	if !strings.Contains(html, "&lt;script&gt;") {
		t.Fatalf("escaped script marker missing from %s", html)
	}
}
```

- [ ] Change the plain content branch in `buildHTMLFromSections`:

```go
	if content != "" {
		sb.WriteString(`<div class="plain-content">`)
		sb.WriteString(escapeHTML(content))
		sb.WriteString("</div>")
	}
```

- [ ] Add `white-space: pre-wrap;` for `.plain-content` in the inline style so newlines remain readable.
- [ ] Run `go test ./internal/tools -run "BuildHTML|DocumentCreator" -count=1`.
  Expected: plain content is escaped, sections continue to render escaped content.

## Wave 5: Schemas, Metadata, Manuals, And Full Verification

### Task 13: Lock schema behavior with tests

**Files:**

- Modify `internal/agent/tool_catalog_test.go`
- Modify `internal/agent/native_tools_integrations.go`

- [ ] Extend `TestNativeToolSchemasIncludeOfficeTools` or add a new test that asserts dedicated Office schemas require both `operation` and `path`:

```go
func assertRequiredContains(t *testing.T, params map[string]interface{}, want string) {
	t.Helper()
	required, _ := params["required"].([]string)
	if containsString(required, want) {
		return
	}
	requiredAny, _ := params["required"].([]interface{})
	if containsInterfaceString(requiredAny, want) {
		return
	}
	t.Fatalf("required fields %#v missing %q", params["required"], want)
}
```

- [ ] Use the helper for both `office_document` and `office_workbook`.
- [ ] Do not make `path` globally required for `virtual_desktop`.
- [ ] Improve the `virtual_desktop` path property description to say it is required for file and Office operations:

```go
"path": prop("string", "Workspace-relative file or directory path. Required for file operations and Office operations such as read_document, write_document, patch_document, read_workbook, write_workbook, set_cell, set_range, evaluate_formula, and export_file."),
```

- [ ] Run `go test ./internal/agent -run "OfficeTools|ToolSchemas" -count=1`.
  Expected: schema requirements are documented by tests.

### Task 14: Document version metadata compatibility

**Files:**

- Modify `prompts/tools_manuals/office_document.md`
- Modify `prompts/tools_manuals/office_workbook.md`

- [ ] Add one short note to each manual:

```md
`office_version.modified` and `office_version.mod_time` are equivalent RFC3339 timestamps; prefer `modified` in new callers.
```

- [ ] Do not remove either JSON field in this pass because existing callers may rely on either name.
- [ ] Run `go test ./internal/tools -run Office -count=1`.
  Expected: no behavior change.

### Task 15: Final verification

**Files:**

- All files touched in Waves 1-5

- [ ] Run focused tests:

```bash
go test ./internal/office ./internal/tools ./internal/agent ./ui -count=1
```

- [ ] Run full tests:

```bash
go test ./...
```

- [ ] Run JS syntax checks if `ui/cfg/virtual_desktop.js` changed:

```bash
node --check ui/js/config/main.js
node --check ui/cfg/virtual_desktop.js
```

- [ ] Run whitespace check:

```bash
git diff --check
```

- [ ] Commit the implementation:

```bash
git add internal/tools/virtual_desktop.go internal/tools/office_tools.go internal/tools/office_tools_test.go internal/tools/virtual_desktop_test.go internal/tools/document_creator.go internal/tools/document_creator_test.go internal/office/office.go internal/office/office_test.go internal/config/config_types.go config_template.yaml internal/agent/native_tools_integrations.go internal/agent/tool_catalog_test.go ui/cfg/virtual_desktop.js ui/desktop_office_assets_test.go ui/lang/desktop prompts/tools_manuals/office_document.md prompts/tools_manuals/office_workbook.md
git commit -m "fix: harden office tools"
```

## Self-Review

- Spec coverage: every confirmed report item is covered. The schema report item is corrected: dedicated Office tools already require `path`, while generic `virtual_desktop` cannot do so globally.
- Security posture: disabled-tool bypasses, read-only modes, corrupt workbook overwrite, invalid JSON ambiguity, and `create_pdf` raw content are addressed before low-priority metadata cleanup.
- Compatibility: `office_version.modified` and `office_version.mod_time` remain as aliases. Removing one should wait for a separate API compatibility plan.
- UI localization: adding Office toggles to the Virtual Desktop config page requires all 16 `ui/lang/desktop/*.json` files to be updated.
