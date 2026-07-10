# Precision Slate Palette Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Give every Precision Workspace consumer a coherent slate-blue palette while preserving all behavior and protected UI systems.

**Architecture:** `ui/css/precision-workspace.css` is the single shared source for Precision dark/light tokens and legacy-compatible aliases. Existing operational and entry styles already consume those variables, so no page-level override is added. A static UI contract locks the exact palette and prevents the retired green/teal tokens from returning.

**Tech Stack:** Vanilla CSS custom properties, Go static UI contract tests, existing browser smoke matrix.

## Global Constraints

- Apply only under `.pw-page`; do not modify Web Chat, Virtual Desktop, Gallery, shared CSS/JS, fonts, routes, DOM hooks, REST behavior, density behavior, or information architecture.
- Dark tokens: canvas `#10161e`, surface `#18212b`, elevated `#202b37`, soft `#2a3745`, text `#edf2f7`, muted `#aab7c4`, subtle `#7d8b99`, accent `#6f98bd`, strong accent `#91b5d6`.
- Light tokens: canvas `#eef2f6`, surface `#fbfcfe`, elevated `#f1f5f9`, soft `#e3eaf1`, text `#182431`, muted `#5f6f7f`, subtle `#7b8997`, accent `#426d93`, strong accent `#5d87aa`.
- Keep semantic danger, warning, and success tokens recognizable.
- Do not introduce gradients, page-specific palette overrides, external dependencies, or visible copy.
- Run GitNexus impact before modifying an existing symbol and `detect_changes` before committing.

---

### Task 1: Replace shared Precision tokens and lock the palette contract

**Files:**
- Modify: `ui/css/precision-workspace.css:3-42,100-114`
- Modify: `ui/precision_pages_test.go:10-57`
- Test: `ui/precision_pages_test.go`

**Interfaces:**
- Consumes: all Precision components and page sheets through `--pw-*` and the compatibility aliases `--bg-*`, `--text-*`, `--accent`, and `--border-accent`.
- Produces: the slate-blue color family for every `.pw-page` consumer without a new page-level API.

- [ ] **Step 1: Add a failing slate-palette contract**

Extend `TestPrecisionWorkspaceFoundationComponentsAreScoped` or add
`TestPrecisionWorkspaceSlatePaletteTokens` with this exact assertion data:

```go
expected := []string{
    `--pw-canvas: #10161e;`, `--pw-surface: #18212b;`,
    `--pw-surface-elevated: #202b37;`, `--pw-surface-soft: #2a3745;`,
    `--pw-text: #edf2f7;`, `--pw-muted: #aab7c4;`,
    `--pw-subtle: #7d8b99;`, `--pw-accent: #6f98bd;`,
    `--pw-accent-strong: #91b5d6;`,
}
for _, token := range expected {
    if !strings.Contains(foundation, token) { t.Errorf("missing dark slate token %q", token) }
}
if strings.Contains(foundation, `#2dd4bf`) || strings.Contains(foundation, `#5eead4`) {
    t.Fatal("Precision foundation must not retain the teal accent")
}
```

Add an equivalent light-theme assertion for the exact light values and assert
that the compatibility aliases still point at `var(--pw-...)` tokens.

- [ ] **Step 2: Run the focused test and verify RED**

Run:

```powershell
go test -count=1 ./ui/... -run 'TestPrecisionWorkspace(FoundationComponentsAreScoped|SlatePaletteTokens)'
```

Expected: FAIL because `precision-workspace.css` still contains `#2dd4bf`,
`#5eead4`, `#0f766e`, and `#0d9488`.

- [ ] **Step 3: Replace the shared dark and light token values**

In `.pw-page`, replace only the dark `--pw-canvas`, surface, text, muted,
subtle, accent, strong-accent, and shadow color source values with the exact
dark values from Global Constraints. In `[data-theme="light"] .pw-page,
.pw-page[data-theme="light"]`, replace the corresponding light values with the
exact light values from Global Constraints. Keep `--bg-*`, `--text-*`,
`--accent`, `--accent-dim`, `--border-*`, semantic status tokens, spacing,
focus, density, and motion declarations structurally unchanged.

- [ ] **Step 4: Run focused static contracts and verify GREEN**

Run:

```powershell
go test -count=1 ./ui/... -run 'TestPrecisionWorkspace(FoundationComponentsAreScoped|SlatePaletteTokens)'
```

Expected: PASS.

- [ ] **Step 5: Run regression checks**

Run:

```powershell
go test -count=1 ./ui/...
$env:AURAGO_RUN_BROWSER_SMOKE='1'
$env:AURAGO_BROWSER_ARTIFACT_DIR='disposable/browser-artifacts-slate'
go test -count=1 ./ui/... -run 'Precision.*Browser|ConfigPrecisionWorkspaceBrowserMatrix'
git diff --exit-code main -- ui/index.html ui/desktop.html ui/gallery.html ui/js/shared ui/js/chat ui/js/desktop ui/fonts ui/shared-variables.css ui/shared-utilities.css ui/shared-components.css ui/shared-animations.css
```

Expected: all tests pass and the protected-surface diff is empty.

- [ ] **Step 6: Inspect impact and commit**

Run:

```powershell
git diff --check
git status --short
```

Run GitNexus `detect_changes` for the staged change. Then commit only the
stylesheet and static test:

```powershell
git add ui/css/precision-workspace.css ui/precision_pages_test.go
git commit -m "feat(ui): adopt Precision slate palette"
```
