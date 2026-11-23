# Code Studio IDE Package 1A: Editor Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Ship section A of the approved spec `docs/superpowers/specs/2026-11-23-code-studio-ide-package-1-design.md`. That covers code folding, bracket matching and indent on input; YAML, SQL, Java, PHP, XML, shell, Dockerfile and TOML highlighting; Go to Line and Go to Symbol; two different files in the split view; an editor settings dialog; Format Document; and the shortcut help. Behind it, `POST /api/code-studio/exec` gets a `stdin` payload and a 4 MiB output cap.

**Architecture:** The Code Studio front end is a set of fragments in one IIFE: `core.js` opens it, `shortcuts.js` closes it, and `command-palette.js` is its own IIFE behind them. They are concatenated into `ui/js/desktop/bundles/code-studio.bundle.js` by `npm run build:ui`. Two new fragments, `settings.js` (A5) and `format.js` (A6), sit before `shortcuts.js`. The split work (A4) stays in `panels.js`, `editor.js` and `core.js`. It works by keeping one tab reference per pane plus the focused pane, and the focused pane's tab is always `state.activeTabIndex`. The CodeMirror vendor bundle gains the folding, gutter, language and command exports. On the backend, the adapter gets an `ExecWithStdin` method. It creates the exec with `AttachStdin`, upgrades the start call to a raw stream, writes stdin, half-closes, and reads the demultiplexed frames. All of that lives in a new Go file, so `handleExec` only changes its body.

**Tech Stack:** Vanilla JS (ES2020) and CodeMirror 6 through a rollup vendor bundle; Go `net/http` with the Docker Engine API (hijacked exec stream); go-rod browser tests in package `ui`; Node 20+ for the build scripts.

---

## Notes for the executor

- **Repo rule (CLAUDE.md).** Before editing any existing function, run GitNexus `impact` upstream on it, using MCP or the CLI in `CLAUDE.md`. Report the callers and the risk. Stop and warn on HIGH or CRITICAL. Treat `UNKNOWN` as unresolved and confirm it with a text search. Each task lists its symbols.
- **Write tool escapes.** The Write tool decodes `\u` escapes, so no code in this plan uses `\u` escapes. Copy `\\`, `\n`, `\r`, `\t` and `\.` exactly as written.
- **Command lists.** The palette's command list lives in `command-palette.js` `getCommands()`, not in `core.js`. `core.js` only holds the window menus (`renderWindowMenus`), and the callable command table is `studioCommandTable()` in `shortcuts.js`. Each of the three gets one contiguous block marked `// IDE package 1A`.
- **Shared regions for Plans B and C, which run after this plan on the same tree:**
  - New code never compares `kind === 'diff'`. It uses the new `isSourceTab(tab)` (in `editor.js`), which is true for file tabs and false for any tab with a `kind`, so B's future `commit` tabs are excluded automatically. No existing `kind === 'diff'` line is modified here, including `panels.js` line 17, `editor.js` line 26 and `core.js` line 567.
  - `saveTab` is unchanged. Explicit saves go through the new `saveTabWithActions` in `format.js`; auto save and the desktop's session save call `saveTab` directly.
  - `createCodeMirrorEditor` gets its settings compartment between `const light = ...` and `const scheme = ...`, and `view.csSettings` is set right before `view.csScheme = scheme;`. The `scheme`/`policy` lines stay untouched for B3's blame compartment.
  - `renderStatus` gets its new item after the `lineInfo` line and its click wiring after the zoom wiring. The `problems` wiring at the end of the function is untouched.
  - `applyTabText(tab, text)` in `format.js` replaces a tab's text as one isolated undo step and handles live views, the textarea fallback and snapshots. Plan C's in-buffer replace can reuse it.
  - The vendor bundle now exports `gutter` and `GutterMarker` for B3's blame gutter.
- **CSS.** All new CSS goes under one marker at the end of `ui/css/code-studio.css`. Each later task appends after the previous one.
- **Generated files.** After JS edits run `npm run build:ui` and `npm run build:ui -- --check`. After `scripts/codemirror-entry.js` changes run `npm run build:codemirror`. No `npm install`: every package is already in `node_modules`.

## File Structure

**Created**

| File | Purpose |
| --- | --- |
| `internal/server/code_studio_exec_stdin.go` | `codeStudioStdinExecer`, `codeStudioExecOutputLimit`, `capCodeStudioExecOutput`, `codeStudioDockerAdapter.ExecWithStdin`, stdin stream opener, frame reader, exit-code poll |
| `internal/server/code_studio_exec_test.go` | Exec stdin piping, 413 over the cap, 4 MiB output cap, adapter round trip against a fake engine |
| `ui/js/desktop/apps/code-studio/settings.js` | A5: settings storage, per-view extensions, live apply, status item, settings dialog, font size, auto save |
| `ui/js/desktop/apps/code-studio/format.js` | A6: formatters (gofmt, TypeScript LanguageService, JSON), `applyTabText`, save actions, `saveTabWithActions` |
| `ui/desktop_code_studio_editor_browser_test.go` | The one browser test for area A |
| `C:\disposable\code-studio-pkg1\locales-a.mjs` | One-off locale merge script; not committed |

**Modified**

| File | Change |
| --- | --- |
| `package.json`, `package-lock.json` | Five `@codemirror` packages become direct dependencies |
| `scripts/codemirror-entry.js`, `scripts/build-codemirror.js` | New exports; package list |
| `ui/js/vendor/codemirror-bundle.esm.js` | Generated |
| `ui/code_studio_assets_test.go` | Export list check covers the new names and tolerates rollup aliases |
| `internal/server/code_studio_handlers.go` | `handleExec` body: `stdin`, 413, output cap, `truncated` |
| `ui/lang/desktop/{cs,da,de,el,en,es,fr,hi,it,ja,nl,no,pl,pt,sv,zh}.json` | 43 new `codeStudio.*` keys |
| `ui/js/desktop/apps/code-studio/core.js` | Languages, `execInput`, settings gear, status item, save routing, split state and persistence, close-split rule, Open to the Side in the tab menu, Edit menu block |
| `ui/js/desktop/apps/code-studio/editor.js` | Folding, brackets, indent on input, fold keymap, Ctrl+G freed; settings compartment; auto save hooks; `isSourceTab`; split prune; `runEditorCommand` |
| `ui/js/desktop/apps/code-studio/panels.js` | Two-file split, pane focus, Open to the Side, split persistence helpers |
| `ui/js/desktop/apps/code-studio/sidebar.js` | Gear in the activity bar; Open to the Side in the explorer menu |
| `ui/js/desktop/apps/code-studio/outline.js` | `jumpToLine` uses the focused view; `openGoToLine` and `openGoToSymbol` |
| `ui/js/desktop/apps/code-studio/command-palette.js` | `:` and `@` modes, Ctrl+Enter to the side, the A-command block |
| `ui/js/desktop/apps/code-studio/shortcuts.js` | Ctrl+G, Ctrl+\, Shift+Alt+F, help section, command table block, `documentSymbols` API |
| `scripts/build-ui-bundles.js` | Adds `settings.js` and `format.js` |
| `ui/js/desktop/bundles/code-studio.bundle.js` | Generated |
| `ui/css/code-studio.css` | Appended block `/* === IDE package 1A: editor === */` |

---

### Task 1: CodeMirror vendor bundle: folding, gutters, languages and editor commands (A1, A2)

**Files:**
- Modify: `package.json`, `package-lock.json`, `scripts/codemirror-entry.js`, `scripts/build-codemirror.js`, `ui/code_studio_assets_test.go`
- Generate: `ui/js/vendor/codemirror-bundle.esm.js`

- [ ] **Step 1: Add the five packages to `package.json` at their installed versions**

In `package.json`, replace:
```
    "@codemirror/lang-javascript": "^6.2.5",
    "@codemirror/lang-json": "^6.0.2",
    "@codemirror/lang-markdown": "^6.5.2",
    "@codemirror/lang-python": "^6.2.1",
    "@codemirror/lang-rust": "^6.0.2",
    "@codemirror/lang-yaml": "^6.1.3",
    "@codemirror/language": "^6.13.1",
    "@codemirror/lint": "^6.9.7",
```
With:
```
    "@codemirror/lang-java": "^6.0.2",
    "@codemirror/lang-javascript": "^6.2.5",
    "@codemirror/lang-json": "^6.0.2",
    "@codemirror/lang-markdown": "^6.5.2",
    "@codemirror/lang-php": "^6.0.2",
    "@codemirror/lang-python": "^6.2.1",
    "@codemirror/lang-rust": "^6.0.2",
    "@codemirror/lang-sql": "^6.10.0",
    "@codemirror/lang-xml": "^6.1.0",
    "@codemirror/lang-yaml": "^6.1.3",
    "@codemirror/language": "^6.13.1",
    "@codemirror/legacy-modes": "^6.5.4",
    "@codemirror/lint": "^6.9.7",
```

- [ ] **Step 2: Mirror the change in the root entry of `package-lock.json` (`"packages"` → `""` → `"dependencies"`)**

The `node_modules/@codemirror/...` entries already exist at these versions (6.0.2, 6.0.2, 6.10.0, 6.1.0, 6.5.4) and stay unchanged. In `package-lock.json`, replace:
```
        "@codemirror/lang-javascript": "^6.2.5",
        "@codemirror/lang-json": "^6.0.2",
        "@codemirror/lang-markdown": "^6.5.2",
        "@codemirror/lang-python": "^6.2.1",
        "@codemirror/lang-rust": "^6.0.2",
        "@codemirror/lang-yaml": "^6.1.3",
        "@codemirror/language": "^6.13.1",
        "@codemirror/lint": "^6.9.7",
```
With:
```
        "@codemirror/lang-java": "^6.0.2",
        "@codemirror/lang-javascript": "^6.2.5",
        "@codemirror/lang-json": "^6.0.2",
        "@codemirror/lang-markdown": "^6.5.2",
        "@codemirror/lang-php": "^6.0.2",
        "@codemirror/lang-python": "^6.2.1",
        "@codemirror/lang-rust": "^6.0.2",
        "@codemirror/lang-sql": "^6.10.0",
        "@codemirror/lang-xml": "^6.1.0",
        "@codemirror/lang-yaml": "^6.1.3",
        "@codemirror/language": "^6.13.1",
        "@codemirror/legacy-modes": "^6.5.4",
        "@codemirror/lint": "^6.9.7",
```

- [ ] **Step 3: Export the new pieces from `scripts/codemirror-entry.js`**

Replace:
```
  highlightActiveLine,
} from '@codemirror/view';
```
With:
```
  highlightActiveLine,
  gutter,
  GutterMarker,
} from '@codemirror/view';
```

Replace:
```
export { defaultKeymap, history, historyKeymap, indentWithTab, undo, redo } from '@codemirror/commands';
export { searchKeymap, highlightSelectionMatches } from '@codemirror/search';
```
With:
```
export {
  defaultKeymap,
  history,
  historyKeymap,
  indentWithTab,
  undo,
  redo,
  isolateHistory,
  moveLineUp,
  moveLineDown,
  copyLineUp,
  copyLineDown,
  toggleComment,
  deleteLine,
} from '@codemirror/commands';
export { searchKeymap, highlightSelectionMatches, selectNextOccurrence } from '@codemirror/search';
```

Replace:
```
export { yaml } from '@codemirror/lang-yaml';
```
With:
```
export { yaml } from '@codemirror/lang-yaml';
export { sql } from '@codemirror/lang-sql';
export { java } from '@codemirror/lang-java';
export { php } from '@codemirror/lang-php';
export { xml } from '@codemirror/lang-xml';
export { shell } from '@codemirror/legacy-modes/mode/shell';
export { dockerFile } from '@codemirror/legacy-modes/mode/dockerfile';
export { toml } from '@codemirror/legacy-modes/mode/toml';
```

Replace:
```
export { defaultHighlightStyle, syntaxHighlighting, indentUnit } from '@codemirror/language';
```
With:
```
export {
  defaultHighlightStyle,
  syntaxHighlighting,
  indentUnit,
  foldGutter,
  foldKeymap,
  codeFolding,
  foldCode,
  unfoldCode,
  bracketMatching,
  indentOnInput,
  StreamLanguage,
} from '@codemirror/language';
```

- [ ] **Step 4: Extend the package list in `scripts/build-codemirror.js`**

`expectedExports` in this script is the list of bundled packages. The exported names are checked by the Go test in Step 5. Replace:
```
  '@codemirror/lang-yaml',
  '@codemirror/theme-one-dark',
```
With:
```
  '@codemirror/lang-yaml',
  '@codemirror/lang-sql',
  '@codemirror/lang-java',
  '@codemirror/lang-php',
  '@codemirror/lang-xml',
  '@codemirror/legacy-modes',
  '@codemirror/theme-one-dark',
```

- [ ] **Step 5: Check the new export names in `ui/code_studio_assets_test.go`**

Replace:
```go
	for _, name := range strings.Split(list, ",") {
		exported[strings.TrimSpace(name)] = true
	}
	for _, want := range []string{
		"EditorView", "EditorState", "Compartment", "keymap", "lineNumbers", "history", "defaultKeymap", "indentWithTab", "javascript", "python", "go", "rust", "cpp", "json", "html", "css", "markdown",
		"oneDark", "syntaxHighlighting", "defaultHighlightStyle", "autocompletion", "closeBrackets", "lintKeymap",
		"yaml", "linter", "lintGutter", "setDiagnostics",
	} {
```
With:
```go
	for _, name := range strings.Split(list, ",") {
		name = strings.TrimSpace(name)
		// Rollup writes "local as exported" when a local name clashes; the exported name follows "as".
		if alias := strings.LastIndex(name, " as "); alias >= 0 {
			name = strings.TrimSpace(name[alias+len(" as "):])
		}
		exported[name] = true
	}
	for _, want := range []string{
		"EditorView", "EditorState", "Compartment", "keymap", "lineNumbers", "history", "defaultKeymap", "indentWithTab", "javascript", "python", "go", "rust", "cpp", "json", "html", "css", "markdown",
		"oneDark", "syntaxHighlighting", "defaultHighlightStyle", "autocompletion", "closeBrackets", "lintKeymap",
		"yaml", "linter", "lintGutter", "setDiagnostics",
		// Code Studio IDE package 1A: folding, gutters for blame, more languages and the palette's editor commands.
		"foldGutter", "foldKeymap", "codeFolding", "foldCode", "unfoldCode", "bracketMatching", "indentOnInput", "StreamLanguage",
		"gutter", "GutterMarker", "isolateHistory", "selectNextOccurrence", "moveLineUp", "moveLineDown", "copyLineUp", "copyLineDown", "toggleComment", "deleteLine",
		"sql", "java", "php", "xml", "shell", "dockerFile", "toml",
	} {
```

- [ ] **Step 6: Rebuild and verify (offline)**

Run:
```
npm ls @codemirror/lang-sql @codemirror/lang-java @codemirror/lang-php @codemirror/lang-xml @codemirror/legacy-modes
npm run build:codemirror
go test ./ui -count=1 -v -run "(?i)^Test(CodeMirrorBundleExportsYAMLAndLint|CodeStudioEditorBundleIsEmbedded)$"
```
Expected: `npm ls` lists all five without `extraneous` or `invalid`, the build prints `CodeMirror bundle written to ui/js/vendor/codemirror-bundle.esm.js`, and there are 2 `--- PASS` lines.

**Commit:** `feat(code-studio): bundle folding, gutters and more CodeMirror languages`. Files: `package.json`, `package-lock.json`, `scripts/codemirror-entry.js`, `scripts/build-codemirror.js`, `ui/js/vendor/codemirror-bundle.esm.js`, `ui/code_studio_assets_test.go`.

---

### Task 2: Exec stdin and the 4 MiB output cap (Backend for A6)

**Files:**
- Create: `internal/server/code_studio_exec_stdin.go`, `internal/server/code_studio_exec_test.go`
- Modify: `internal/server/code_studio_handlers.go` (`handleExec` body)

**How the adapter attaches today:** `codeStudioDockerAdapter.exec` creates the exec with `AttachStdout`/`AttachStderr` only. It then posts `/exec/{id}/start` through `tools.DockerRequestContext`, which reads the whole response body (at most 10 MiB) and gives no way to write stdin. The terminal already uses a hijacked stream (`openDockerExecStartStream`, which forces `Tty:true`, plus `bufferedDockerRawConn`). The new stdin path follows that pattern with `Tty:false`, so the output stays multiplexed. It refuses connections that cannot half-close. The existing `exec`, `openDockerExecStartStream` and the terminal code stay untouched.

- [ ] **Step 1: Impact check.** Run `impact` upstream for `handleExec`. Callers are the route registration and `code_studio_security_test.go`.

- [ ] **Step 2: Create `internal/server/code_studio_exec_stdin.go`**

```go
package server

import (
	"bufio"
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"aurago/internal/dockerutil"
	"aurago/internal/tools"
)

// codeStudioExecOutputLimit caps the output one /api/code-studio/exec call returns to the page.
const codeStudioExecOutputLimit = 4 << 20

// codeStudioStdinExecer runs a command with a payload on its standard input.
// The Docker adapter implements it; handleExec asks for it only when a
// request carries stdin.
type codeStudioStdinExecer interface {
	ExecWithStdin(ctx context.Context, containerID string, cmd []string, stdin []byte, timeout time.Duration) (codeStudioExecResult, error)
}

var _ codeStudioStdinExecer = codeStudioDockerAdapter{}

// capCodeStudioExecOutput cuts output to at most limit bytes without
// splitting a UTF-8 sequence and reports whether it cut anything.
func capCodeStudioExecOutput(output string, limit int) (string, bool) {
	if len(output) <= limit {
		return output, false
	}
	cut := limit
	for cut > 0 && output[cut]&0xC0 == 0x80 {
		cut--
	}
	return output[:cut], true
}

// ExecWithStdin runs cmd like Exec, with stdin attached: the start call is
// upgraded to a raw stream, stdin is written, the write side is closed so the
// process reads EOF, and stdout and stderr are read until the process ends.
// At most codeStudioExecOutputLimit+1 bytes are kept; the rest is drained so
// the process can finish and its exit code stays real.
func (a codeStudioDockerAdapter) ExecWithStdin(ctx context.Context, containerID string, cmd []string, stdin []byte, timeout time.Duration) (codeStudioExecResult, error) {
	if timeout <= 0 || timeout > codeStudioMaxExecTime {
		timeout = codeStudioMaxExecTime
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	body, err := json.Marshal(map[string]interface{}{
		"AttachStdin":  true,
		"AttachStdout": true,
		"AttachStderr": true,
		"Cmd":          cmd,
		"Tty":          false,
	})
	if err != nil {
		return codeStudioExecResult{}, fmt.Errorf("encode docker exec: %w", err)
	}
	data, code, err := tools.DockerRequestContext(ctx, a.cfg, http.MethodPost, "/containers/"+url.PathEscape(containerID)+"/exec", string(body))
	if err != nil {
		return codeStudioExecResult{}, err
	}
	if code != http.StatusCreated {
		return codeStudioExecResult{}, fmt.Errorf("docker exec create failed: %s", strings.TrimSpace(string(data)))
	}
	var created struct {
		ID string `json:"Id"`
	}
	if err := json.Unmarshal(data, &created); err != nil {
		return codeStudioExecResult{}, fmt.Errorf("parse docker exec id: %w", err)
	}
	if created.ID == "" {
		return codeStudioExecResult{}, errors.New("docker exec create returned no id")
	}
	stream, err := openCodeStudioExecStdinStream(ctx, a.cfg, created.ID)
	if err != nil {
		return codeStudioExecResult{}, err
	}
	defer stream.Close()
	stop := context.AfterFunc(ctx, func() { _ = stream.Close() })
	defer stop()
	go func() {
		_, _ = stream.Write(stdin)
		if closer, ok := stream.Conn.(interface{ CloseWrite() error }); ok {
			_ = closer.CloseWrite()
		}
	}()
	output, readErr := readCodeStudioExecFrames(stream, codeStudioExecOutputLimit+1)
	if readErr != nil && ctx.Err() != nil {
		return codeStudioExecResult{}, fmt.Errorf("code studio exec: %w", ctx.Err())
	}
	return codeStudioExecResult{ExitCode: a.execExitCode(created.ID), Output: output}, nil
}

// openCodeStudioExecStdinStream starts an exec with stdin attached and no
// TTY, so the output arrives multiplexed. It refuses connections that cannot
// half-close, because the process would never see the end of its input.
func openCodeStudioExecStdinStream(ctx context.Context, cfg tools.DockerConfig, execID string) (*bufferedDockerRawConn, error) {
	version, err := dockerutil.NegotiatedAPIVersion(ctx, cfg.Host)
	if err != nil {
		return nil, err
	}
	conn, err := dockerutil.DialContext(ctx, cfg.Host)
	if err != nil {
		return nil, fmt.Errorf("connect to Docker engine: %w", err)
	}
	if _, ok := conn.(interface{ CloseWrite() error }); !ok {
		_ = conn.Close()
		return nil, errors.New("docker connection cannot close stdin")
	}
	body := []byte(`{"Detach":false,"Tty":false}`)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "http://localhost/"+version+"/exec/"+url.PathEscape(execID)+"/start", bytes.NewReader(body))
	if err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("build docker exec start request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Connection", "Upgrade")
	req.Header.Set("Upgrade", "tcp")
	req.ContentLength = int64(len(body))
	if err := req.Write(conn); err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("send docker exec start request: %w", err)
	}
	reader := bufio.NewReader(conn)
	resp, err := http.ReadResponse(reader, req)
	if err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("read docker exec start response: %w", err)
	}
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusSwitchingProtocols {
		msg, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		_ = resp.Body.Close()
		_ = conn.Close()
		return nil, fmt.Errorf("docker exec start failed: HTTP %d %s", resp.StatusCode, strings.TrimSpace(string(msg)))
	}
	return &bufferedDockerRawConn{Conn: conn, reader: reader}, nil
}

// readCodeStudioExecFrames demultiplexes a non-TTY Docker attach stream
// (8-byte header, then the payload). It keeps at most keep payload bytes and
// discards the rest until the stream ends.
func readCodeStudioExecFrames(r io.Reader, keep int) (string, error) {
	var out bytes.Buffer
	header := make([]byte, 8)
	for {
		if _, err := io.ReadFull(r, header); err != nil {
			if errors.Is(err, io.EOF) {
				return out.String(), nil
			}
			return out.String(), err
		}
		size := int64(binary.BigEndian.Uint32(header[4:8]))
		room := int64(keep - out.Len())
		if room < 0 {
			room = 0
		}
		if room > size {
			room = size
		}
		if room > 0 {
			if _, err := io.CopyN(&out, r, room); err != nil {
				return out.String(), err
			}
		}
		if rest := size - room; rest > 0 {
			if _, err := io.CopyN(io.Discard, r, rest); err != nil {
				return out.String(), err
			}
		}
	}
}

// execExitCode waits until Docker reports the exec as finished (at most about
// two seconds) and returns its exit code, or -1 when that cannot be read.
func (a codeStudioDockerAdapter) execExitCode(execID string) int {
	for attempt := 0; attempt < 40; attempt++ {
		data, code, err := tools.DockerRequestContext(context.Background(), a.cfg, http.MethodGet, "/exec/"+url.PathEscape(execID)+"/json", "")
		if err != nil || code != http.StatusOK {
			return -1
		}
		var inspect struct {
			Running  bool `json:"Running"`
			ExitCode int  `json:"ExitCode"`
		}
		if json.Unmarshal(data, &inspect) != nil {
			return -1
		}
		if !inspect.Running {
			return inspect.ExitCode
		}
		time.Sleep(50 * time.Millisecond)
	}
	return -1
}
```

- [ ] **Step 3: Accept `stdin` in `handleExec` and cap the output**

In `internal/server/code_studio_handlers.go`, replace:
```go
	var body struct {
		Command    string   `json:"command"`
		Args       []string `json:"args"`
		CWD        string   `json:"cwd"`
		TimeoutSec int      `json:"timeout_seconds"`
	}
	if err := decodeJSONRequest(w, r, &body, h.server); err != nil {
		jsonError(w, "Invalid JSON", jsonDecodeStatus(err))
		return
	}
	cmd, err := normalizeCodeStudioExecCommand(body.Command, body.Args, body.CWD)
	if err != nil {
		jsonError(w, err.Error(), http.StatusBadRequest)
		return
	}
	timeout := normalizeCodeStudioExecTimeout(body.TimeoutSec)
	_, containerID, err := h.codeContainer(r.Context(), true)
	if err != nil {
		jsonError(w, err.Error(), http.StatusServiceUnavailable)
		return
	}
	result, err := h.docker.Exec(r.Context(), containerID, cmd, timeout)
	if err != nil {
		jsonError(w, err.Error(), http.StatusBadGateway)
		return
	}
	writeJSON(w, map[string]interface{}{"status": "ok", "exit_code": result.ExitCode, "output": result.Output})
}
```
With:
```go
	var body struct {
		Command    string   `json:"command"`
		Args       []string `json:"args"`
		CWD        string   `json:"cwd"`
		TimeoutSec int      `json:"timeout_seconds"`
		Stdin      *string  `json:"stdin"`
	}
	if err := decodeJSONRequest(w, r, &body, h.server); err != nil {
		jsonError(w, "Invalid JSON", jsonDecodeStatus(err))
		return
	}
	// A stdin payload is a whole file (Format Document), so it shares the configured file size cap.
	if body.Stdin != nil && int64(len(*body.Stdin)) > h.maxFileSizeBytes() {
		jsonError(w, "stdin exceeds configured maximum size", http.StatusRequestEntityTooLarge)
		return
	}
	cmd, err := normalizeCodeStudioExecCommand(body.Command, body.Args, body.CWD)
	if err != nil {
		jsonError(w, err.Error(), http.StatusBadRequest)
		return
	}
	timeout := normalizeCodeStudioExecTimeout(body.TimeoutSec)
	_, containerID, err := h.codeContainer(r.Context(), true)
	if err != nil {
		jsonError(w, err.Error(), http.StatusServiceUnavailable)
		return
	}
	var result codeStudioExecResult
	if body.Stdin != nil {
		execer, ok := h.docker.(codeStudioStdinExecer)
		if !ok {
			jsonError(w, "stdin is not supported by this docker client", http.StatusNotImplemented)
			return
		}
		result, err = execer.ExecWithStdin(r.Context(), containerID, cmd, []byte(*body.Stdin), timeout)
	} else {
		result, err = h.docker.Exec(r.Context(), containerID, cmd, timeout)
	}
	if err != nil {
		jsonError(w, err.Error(), http.StatusBadGateway)
		return
	}
	output, truncated := capCodeStudioExecOutput(result.Output, codeStudioExecOutputLimit)
	writeJSON(w, map[string]interface{}{"status": "ok", "exit_code": result.ExitCode, "output": output, "truncated": truncated})
}
```

- [ ] **Step 4: Create `internal/server/code_studio_exec_test.go`**

```go
package server

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"aurago/internal/tools"
)

// stdinRecordingCodeStudioDockerAPI records the stdin payloads next to the commands.
type stdinRecordingCodeStudioDockerAPI struct {
	recordingCodeStudioDockerAPI
	stdins [][]byte
}

func (f *stdinRecordingCodeStudioDockerAPI) ExecWithStdin(ctx context.Context, containerID string, cmd []string, stdin []byte, timeout time.Duration) (codeStudioExecResult, error) {
	f.stdins = append(f.stdins, append([]byte(nil), stdin...))
	return f.Exec(ctx, containerID, cmd, timeout)
}

type codeStudioExecResponse struct {
	ExitCode  int    `json:"exit_code"`
	Output    string `json:"output"`
	Truncated bool   `json:"truncated"`
}

func postCodeStudioExec(t *testing.T, h codeStudioHandlers, body string) (*httptest.ResponseRecorder, codeStudioExecResponse) {
	t.Helper()
	rec := httptest.NewRecorder()
	h.handleExec(rec, httptest.NewRequest(http.MethodPost, "/api/code-studio/exec", strings.NewReader(body)))
	var resp codeStudioExecResponse
	if rec.Code == http.StatusOK {
		if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
			t.Fatalf("decode exec response: %v; body=%s", err, rec.Body.String())
		}
	}
	return rec, resp
}

func TestCodeStudioExecPipesStdinAndRejectsOversizedInput(t *testing.T) {
	s := testCodeStudioServerWithFakeCodeContainer(t, 1)
	docker := &stdinRecordingCodeStudioDockerAPI{recordingCodeStudioDockerAPI: recordingCodeStudioDockerAPI{results: []codeStudioExecResult{{Output: "package main\n"}}}}
	h := codeStudioHandlers{server: s, docker: docker}

	rec, resp := postCodeStudioExec(t, h, `{"args":["gofmt"],"stdin":"package  main\n"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("stdin exec status=%d body=%s", rec.Code, rec.Body.String())
	}
	if len(docker.stdins) != 1 || string(docker.stdins[0]) != "package  main\n" {
		t.Fatalf("stdin = %q, want the request payload", docker.stdins)
	}
	if !reflect.DeepEqual(docker.commands, [][]string{{"gofmt"}}) {
		t.Fatalf("commands = %#v", docker.commands)
	}
	if resp.Output != "package main\n" || resp.Truncated {
		t.Fatalf("response = %+v", resp)
	}

	payload, err := json.Marshal(map[string]interface{}{"args": []string{"gofmt"}, "stdin": strings.Repeat("a", 1<<20+1)})
	if err != nil {
		t.Fatal(err)
	}
	rec, _ = postCodeStudioExec(t, h, string(payload))
	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("oversized stdin status=%d body=%s", rec.Code, rec.Body.String())
	}
	if len(docker.commands) != 1 || len(docker.stdins) != 1 {
		t.Fatalf("oversized stdin reached docker: commands=%#v", docker.commands)
	}

	rec, _ = postCodeStudioExec(t, h, `{"args":["true"]}`)
	if rec.Code != http.StatusOK || len(docker.stdins) != 1 || len(docker.commands) != 2 {
		t.Fatalf("exec without stdin: status=%d stdins=%d commands=%#v", rec.Code, len(docker.stdins), docker.commands)
	}
}

func TestCodeStudioExecCapsOutputAtFourMiB(t *testing.T) {
	s := testCodeStudioServerWithFakeCodeContainer(t, 1)
	big := strings.Repeat("x", codeStudioExecOutputLimit-1) + "é" + "tail"
	docker := &recordingCodeStudioDockerAPI{results: []codeStudioExecResult{{Output: big}, {Output: "small"}}}
	h := codeStudioHandlers{server: s, docker: docker}

	rec, resp := postCodeStudioExec(t, h, `{"command":"cat big.log"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String()[:200])
	}
	if !resp.Truncated || len(resp.Output) != codeStudioExecOutputLimit-1 {
		t.Fatalf("truncated=%v len=%d, want the output cut before the split rune", resp.Truncated, len(resp.Output))
	}

	rec, resp = postCodeStudioExec(t, h, `{"command":"echo small"}`)
	if rec.Code != http.StatusOK || resp.Truncated || resp.Output != "small" {
		t.Fatalf("small output: status=%d response=%+v", rec.Code, resp)
	}
}

func TestCodeStudioExecWithStdinStreamsThroughDocker(t *testing.T) {
	var mu sync.Mutex
	var created map[string]interface{}
	var received []byte
	host := newContainerDockerAPI(t, func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(io.LimitReader(r.Body, 1<<20))
		switch {
		case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/containers/code-1/exec"):
			mu.Lock()
			_ = json.Unmarshal(body, &created)
			mu.Unlock()
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{"Id":"fmt-1"}`))
		case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/exec/fmt-1/start"):
			conn, err := hijackCodeStudioTTY(w)
			if err != nil {
				return
			}
			defer conn.Close()
			input, _ := io.ReadAll(conn)
			mu.Lock()
			received = input
			mu.Unlock()
			var frames bytes.Buffer
			writeAttachFrame(t, &frames, 1, bytes.ToUpper(input))
			writeAttachFrame(t, &frames, 2, []byte("warning\n"))
			_, _ = conn.Write(frames.Bytes())
		case r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/exec/fmt-1/json"):
			_, _ = w.Write([]byte(`{"Running":false,"ExitCode":3}`))
		default:
			http.Error(w, "unexpected "+r.Method+" "+r.URL.Path, http.StatusConflict)
		}
	})
	adapter := codeStudioDockerAdapter{cfg: tools.DockerConfig{Host: host}}

	result, err := adapter.ExecWithStdin(context.Background(), "code-1", []string{"gofmt"}, []byte("package main\n"), 5*time.Second)
	if err != nil {
		t.Fatalf("ExecWithStdin: %v", err)
	}
	mu.Lock()
	defer mu.Unlock()
	if string(received) != "package main\n" {
		t.Fatalf("engine received stdin %q", received)
	}
	if result.Output != "PACKAGE MAIN\nwarning\n" || result.ExitCode != 3 {
		t.Fatalf("result = %+v", result)
	}
	if created["AttachStdin"] != true || created["Tty"] != false {
		t.Fatalf("exec create = %v, want AttachStdin without a TTY", created)
	}
}
```

- [ ] **Step 5: Format and run**

```
gofmt -w internal/server/code_studio_exec_stdin.go internal/server/code_studio_exec_test.go internal/server/code_studio_handlers.go
go test ./internal/server -count=1 -v -run "(?i)CodeStudioExec"
go test ./internal/server -count=1 -run "(?i)CodeStudio"
```
Expected: 3 `--- PASS` lines (`TestCodeStudioExecPipesStdinAndRejectsOversizedInput`, `TestCodeStudioExecCapsOutputAtFourMiB`, `TestCodeStudioExecWithStdinStreamsThroughDocker`), and `ok` for the broad run.

**Commit:** `feat(code-studio): pipe stdin into exec and cap its output at 4 MiB`. Files: `internal/server/code_studio_exec_stdin.go`, `internal/server/code_studio_exec_test.go`, `internal/server/code_studio_handlers.go`.

---

### Task 3: Locale keys for package 1A (all 16 languages)

**Files:**
- Create (not committed): `C:\disposable\code-studio-pkg1\locales-a.mjs`
- Modify: `ui/lang/desktop/{cs,da,de,el,en,es,fr,hi,it,ja,nl,no,pl,pt,sv,zh}.json`

The files are flat JSON with two-space indentation, and their `codeStudio.*` keys sit in several clusters. The last one is `codeStudio.nextProblem`, followed by more keys. The script inserts the new keys right after the last `codeStudio.*` line. It keeps the line endings, the BOM, the indentation and the order of everything else. It skips keys that already exist and checks that placeholders match English.

- [ ] **Step 1: Create `C:\disposable\code-studio-pkg1\locales-a.mjs`** (create the folder if it is missing)

```js
// Adds the Code Studio IDE package 1A keys to the 16 desktop locale files.
// Usage: node C:\disposable\code-studio-pkg1\locales-a.mjs C:\Users\Andi\Documents\repo\AuraGo
import { readFileSync, writeFileSync } from 'node:fs';
import { join } from 'node:path';

const root = process.argv[2];
if (!root) throw new Error('usage: node locales-a.mjs <repo root>');
const LANGS = ['cs', 'da', 'de', 'el', 'en', 'es', 'fr', 'hi', 'it', 'ja', 'nl', 'no', 'pl', 'pt', 'sv', 'zh'];

const KEYS = {
  'codeStudio.goToLine': {
    cs: "Přejít na řádek...", da: "Gå til linje...", de: "Gehe zu Zeile...", el: "Μετάβαση στη γραμμή...",
    en: "Go to Line...", es: "Ir a la línea...", fr: "Aller à la ligne...", hi: "पंक्ति पर जाएँ...",
    it: "Vai alla riga...", ja: "行へ移動...", nl: "Ga naar regel...", no: "Gå til linje...",
    pl: "Przejdź do wiersza...", pt: "Ir para a linha...", sv: "Gå till rad...", zh: "转到行..."
  },
  'codeStudio.goToSymbol': {
    cs: "Přejít na symbol...", da: "Gå til symbol...", de: "Gehe zu Symbol...", el: "Μετάβαση στο σύμβολο...",
    en: "Go to Symbol...", es: "Ir al símbolo...", fr: "Aller au symbole...", hi: "प्रतीक पर जाएँ...",
    it: "Vai al simbolo...", ja: "シンボルへ移動...", nl: "Ga naar symbool...", no: "Gå til symbol...",
    pl: "Przejdź do symbolu...", pt: "Ir para o símbolo...", sv: "Gå till symbol...", zh: "转到符号..."
  },
  'codeStudio.formatDocument': {
    cs: "Formátovat dokument", da: "Formatér dokument", de: "Dokument formatieren", el: "Μορφοποίηση εγγράφου",
    en: "Format Document", es: "Dar formato al documento", fr: "Mettre en forme le document", hi: "दस्तावेज़ फ़ॉर्मेट करें",
    it: "Formatta documento", ja: "ドキュメントのフォーマット", nl: "Document opmaken", no: "Formater dokument",
    pl: "Formatuj dokument", pt: "Formatar documento", sv: "Formatera dokument", zh: "格式化文档"
  },
  'codeStudio.openSettings': {
    cs: "Předvolby: Otevřít nastavení", da: "Præferencer: Åbn indstillinger", de: "Einstellungen: Einstellungen öffnen", el: "Προτιμήσεις: Άνοιγμα ρυθμίσεων",
    en: "Preferences: Open Settings", es: "Preferencias: Abrir configuración", fr: "Préférences : Ouvrir les paramètres", hi: "प्राथमिकताएँ: सेटिंग्स खोलें",
    it: "Preferenze: Apri impostazioni", ja: "基本設定: 設定を開く", nl: "Voorkeuren: Instellingen openen", no: "Innstillinger: Åpne innstillinger",
    pl: "Preferencje: Otwórz ustawienia", pt: "Preferências: Abrir definições", sv: "Inställningar: Öppna inställningar", zh: "首选项: 打开设置"
  },
  'codeStudio.toggleSplit': {
    cs: "Přepnout rozdělený editor", da: "Slå opdelt editor til/fra", de: "Geteilten Editor umschalten", el: "Εναλλαγή διαιρεμένου επεξεργαστή",
    en: "Toggle Split Editor", es: "Alternar editor dividido", fr: "Basculer l'éditeur scindé", hi: "विभाजित संपादक टॉगल करें",
    it: "Attiva/disattiva editor diviso", ja: "分割エディターの切り替え", nl: "Gesplitste editor in-/uitschakelen", no: "Slå delt redigerer av/på",
    pl: "Przełącz podzielony edytor", pt: "Alternar editor dividido", sv: "Växla delad redigerare", zh: "切换拆分编辑器"
  },
  'codeStudio.openToSide': {
    cs: "Otevřít vedle", da: "Åbn ved siden af", de: "Zur Seite öffnen", el: "Άνοιγμα στο πλάι",
    en: "Open to the Side", es: "Abrir en el lateral", fr: "Ouvrir sur le côté", hi: "बगल में खोलें",
    it: "Apri lateralmente", ja: "横に開く", nl: "Aan de zijkant openen", no: "Åpne ved siden av",
    pl: "Otwórz z boku", pt: "Abrir ao lado", sv: "Öppna vid sidan", zh: "在侧边打开"
  },
  'codeStudio.foldRegion': {
    cs: "Sbalit", da: "Fold sammen", de: "Einklappen", el: "Σύμπτυξη",
    en: "Fold", es: "Plegar", fr: "Replier", hi: "संकुचित करें",
    it: "Comprimi", ja: "折りたたむ", nl: "Samenvouwen", no: "Brett sammen",
    pl: "Zwiń", pt: "Recolher", sv: "Fäll ihop", zh: "折叠"
  },
  'codeStudio.unfoldRegion': {
    cs: "Rozbalit", da: "Fold ud", de: "Ausklappen", el: "Ανάπτυξη",
    en: "Unfold", es: "Desplegar", fr: "Déplier", hi: "विस्तारित करें",
    it: "Espandi", ja: "展開する", nl: "Uitvouwen", no: "Brett ut",
    pl: "Rozwiń", pt: "Expandir", sv: "Fäll ut", zh: "展开"
  },
  'codeStudio.selectNextOccurrence': {
    cs: "Přidat výběr k další shodě", da: "Føj markering til næste match", de: "Auswahl zur nächsten Übereinstimmung hinzufügen", el: "Προσθήκη επιλογής στην επόμενη αντιστοιχία",
    en: "Add Selection to Next Match", es: "Agregar selección a la siguiente coincidencia", fr: "Ajouter la sélection à la correspondance suivante", hi: "अगले मिलान में चयन जोड़ें",
    it: "Aggiungi selezione alla corrispondenza successiva", ja: "次の一致箇所を選択に追加", nl: "Selectie toevoegen aan volgende overeenkomst", no: "Legg til markering ved neste treff",
    pl: "Dodaj zaznaczenie do następnego dopasowania", pt: "Adicionar seleção à correspondência seguinte", sv: "Lägg till markering vid nästa träff", zh: "将选择添加到下一个匹配项"
  },
  'codeStudio.moveLineUp': {
    cs: "Přesunout řádek nahoru", da: "Flyt linje op", de: "Zeile nach oben verschieben", el: "Μετακίνηση γραμμής πάνω",
    en: "Move Line Up", es: "Mover línea hacia arriba", fr: "Déplacer la ligne vers le haut", hi: "पंक्ति ऊपर ले जाएँ",
    it: "Sposta riga in alto", ja: "行を上へ移動", nl: "Regel omhoog verplaatsen", no: "Flytt linje opp",
    pl: "Przenieś wiersz w górę", pt: "Mover linha para cima", sv: "Flytta rad uppåt", zh: "向上移动行"
  },
  'codeStudio.moveLineDown': {
    cs: "Přesunout řádek dolů", da: "Flyt linje ned", de: "Zeile nach unten verschieben", el: "Μετακίνηση γραμμής κάτω",
    en: "Move Line Down", es: "Mover línea hacia abajo", fr: "Déplacer la ligne vers le bas", hi: "पंक्ति नीचे ले जाएँ",
    it: "Sposta riga in basso", ja: "行を下へ移動", nl: "Regel omlaag verplaatsen", no: "Flytt linje ned",
    pl: "Przenieś wiersz w dół", pt: "Mover linha para baixo", sv: "Flytta rad nedåt", zh: "向下移动行"
  },
  'codeStudio.copyLineUp': {
    cs: "Kopírovat řádek nahoru", da: "Kopiér linje op", de: "Zeile nach oben kopieren", el: "Αντιγραφή γραμμής πάνω",
    en: "Copy Line Up", es: "Copiar línea arriba", fr: "Copier la ligne vers le haut", hi: "पंक्ति ऊपर कॉपी करें",
    it: "Copia riga in alto", ja: "行を上へコピー", nl: "Regel omhoog kopiëren", no: "Kopier linje opp",
    pl: "Kopiuj wiersz w górę", pt: "Copiar linha para cima", sv: "Kopiera rad uppåt", zh: "向上复制行"
  },
  'codeStudio.copyLineDown': {
    cs: "Kopírovat řádek dolů", da: "Kopiér linje ned", de: "Zeile nach unten kopieren", el: "Αντιγραφή γραμμής κάτω",
    en: "Copy Line Down", es: "Copiar línea abajo", fr: "Copier la ligne vers le bas", hi: "पंक्ति नीचे कॉपी करें",
    it: "Copia riga in basso", ja: "行を下へコピー", nl: "Regel omlaag kopiëren", no: "Kopier linje ned",
    pl: "Kopiuj wiersz w dół", pt: "Copiar linha para baixo", sv: "Kopiera rad nedåt", zh: "向下复制行"
  },
  'codeStudio.toggleComment': {
    cs: "Přepnout řádkový komentář", da: "Slå linjekommentar til/fra", de: "Zeilenkommentar umschalten", el: "Εναλλαγή σχολίου γραμμής",
    en: "Toggle Line Comment", es: "Alternar comentario de línea", fr: "Activer/désactiver le commentaire de ligne", hi: "पंक्ति टिप्पणी टॉगल करें",
    it: "Attiva/disattiva commento riga", ja: "行コメントの切り替え", nl: "Regelcommentaar in-/uitschakelen", no: "Slå linjekommentar av/på",
    pl: "Przełącz komentarz wiersza", pt: "Alternar comentário de linha", sv: "Växla radkommentar", zh: "切换行注释"
  },
  'codeStudio.deleteLine': {
    cs: "Odstranit řádek", da: "Slet linje", de: "Zeile löschen", el: "Διαγραφή γραμμής",
    en: "Delete Line", es: "Eliminar línea", fr: "Supprimer la ligne", hi: "पंक्ति हटाएँ",
    it: "Elimina riga", ja: "行の削除", nl: "Regel verwijderen", no: "Slett linje",
    pl: "Usuń wiersz", pt: "Eliminar linha", sv: "Ta bort rad", zh: "删除行"
  },
  'codeStudio.addCursor': {
    cs: "Přidat kurzor", da: "Tilføj markør", de: "Cursor hinzufügen", el: "Προσθήκη δρομέα",
    en: "Add Cursor", es: "Agregar cursor", fr: "Ajouter un curseur", hi: "कर्सर जोड़ें",
    it: "Aggiungi cursore", ja: "カーソルを追加", nl: "Cursor toevoegen", no: "Legg til markør",
    pl: "Dodaj kursor", pt: "Adicionar cursor", sv: "Lägg till markör", zh: "添加光标"
  },
  'codeStudio.shortcutsEditing': {
    cs: "Úpravy", da: "Redigering", de: "Bearbeiten", el: "Επεξεργασία",
    en: "Editing", es: "Edición", fr: "Édition", hi: "संपादन",
    it: "Modifica", ja: "編集", nl: "Bewerken", no: "Redigering",
    pl: "Edycja", pt: "Edição", sv: "Redigering", zh: "编辑"
  },
  'codeStudio.goToLineHint': {
    cs: "Zadejte číslo řádku od 1 do {{count}}. Přidejte :sloupec pro skok na sloupec.",
    da: "Skriv et linjenummer mellem 1 og {{count}}. Tilføj :kolonne for at springe til en kolonne.",
    de: "Zeilennummer zwischen 1 und {{count}} eingeben. :Spalte anhängen, um zu einer Spalte zu springen.",
    el: "Πληκτρολογήστε αριθμό γραμμής από 1 έως {{count}}. Προσθέστε :στήλη για μετάβαση σε στήλη.",
    en: "Type a line number between 1 and {{count}}. Add :column to jump to a column.",
    es: "Escribe un número de línea entre 1 y {{count}}. Añade :columna para saltar a una columna.",
    fr: "Saisissez un numéro de ligne entre 1 et {{count}}. Ajoutez :colonne pour aller à une colonne.",
    hi: "1 से {{count}} के बीच पंक्ति संख्या लिखें। किसी स्तंभ पर जाने के लिए :स्तंभ जोड़ें।",
    it: "Digita un numero di riga tra 1 e {{count}}. Aggiungi :colonna per saltare a una colonna.",
    ja: "1 から {{count}} までの行番号を入力してください。:列 を付けると列へ移動します。",
    nl: "Typ een regelnummer tussen 1 en {{count}}. Voeg :kolom toe om naar een kolom te springen.",
    no: "Skriv et linjenummer mellom 1 og {{count}}. Legg til :kolonne for å hoppe til en kolonne.",
    pl: "Wpisz numer wiersza od 1 do {{count}}. Dodaj :kolumna, aby przejść do kolumny.",
    pt: "Escreva um número de linha entre 1 e {{count}}. Acrescente :coluna para saltar para uma coluna.",
    sv: "Skriv ett radnummer mellan 1 och {{count}}. Lägg till :kolumn för att hoppa till en kolumn.",
    zh: "输入 1 到 {{count}} 之间的行号。添加 :列 可跳转到指定列。"
  },
  'codeStudio.goToLineItem': {
    cs: "Přejít na řádek {{line}}", da: "Gå til linje {{line}}", de: "Gehe zu Zeile {{line}}", el: "Μετάβαση στη γραμμή {{line}}",
    en: "Go to line {{line}}", es: "Ir a la línea {{line}}", fr: "Aller à la ligne {{line}}", hi: "पंक्ति {{line}} पर जाएँ",
    it: "Vai alla riga {{line}}", ja: "{{line}} 行目へ移動", nl: "Ga naar regel {{line}}", no: "Gå til linje {{line}}",
    pl: "Przejdź do wiersza {{line}}", pt: "Ir para a linha {{line}}", sv: "Gå till rad {{line}}", zh: "转到第 {{line}} 行"
  },
  'codeStudio.goToLineColumnItem': {
    cs: "Přejít na řádek {{line}}, sloupec {{column}}", da: "Gå til linje {{line}}, kolonne {{column}}", de: "Gehe zu Zeile {{line}}, Spalte {{column}}", el: "Μετάβαση στη γραμμή {{line}}, στήλη {{column}}",
    en: "Go to line {{line}}, column {{column}}", es: "Ir a la línea {{line}}, columna {{column}}", fr: "Aller à la ligne {{line}}, colonne {{column}}", hi: "पंक्ति {{line}}, स्तंभ {{column}} पर जाएँ",
    it: "Vai alla riga {{line}}, colonna {{column}}", ja: "{{line}} 行目、{{column}} 列目へ移動", nl: "Ga naar regel {{line}}, kolom {{column}}", no: "Gå til linje {{line}}, kolonne {{column}}",
    pl: "Przejdź do wiersza {{line}}, kolumna {{column}}", pt: "Ir para a linha {{line}}, coluna {{column}}", sv: "Gå till rad {{line}}, kolumn {{column}}", zh: "转到第 {{line}} 行，第 {{column}} 列"
  },
  'codeStudio.symbols': {
    cs: "Symboly", da: "Symboler", de: "Symbole", el: "Σύμβολα",
    en: "Symbols", es: "Símbolos", fr: "Symboles", hi: "प्रतीक",
    it: "Simboli", ja: "シンボル", nl: "Symbolen", no: "Symboler",
    pl: "Symbole", pt: "Símbolos", sv: "Symboler", zh: "符号"
  },
  'codeStudio.settingsTitle': {
    cs: "Nastavení editoru", da: "Editorindstillinger", de: "Editor-Einstellungen", el: "Ρυθμίσεις επεξεργαστή",
    en: "Editor Settings", es: "Configuración del editor", fr: "Paramètres de l'éditeur", hi: "संपादक सेटिंग्स",
    it: "Impostazioni editor", ja: "エディターの設定", nl: "Editorinstellingen", no: "Redigeringsinnstillinger",
    pl: "Ustawienia edytora", pt: "Definições do editor", sv: "Redigerarinställningar", zh: "编辑器设置"
  },
  'codeStudio.settingsTabSize': {
    cs: "Velikost tabulátoru", da: "Tabulatorstørrelse", de: "Tabulatorbreite", el: "Μέγεθος στηλοθέτη",
    en: "Tab size", es: "Tamaño de tabulación", fr: "Taille de tabulation", hi: "टैब आकार",
    it: "Dimensione tabulazione", ja: "タブのサイズ", nl: "Tabgrootte", no: "Tabulatorbredde",
    pl: "Rozmiar tabulacji", pt: "Tamanho da tabulação", sv: "Tabbstorlek", zh: "制表符大小"
  },
  'codeStudio.settingsIndentation': {
    cs: "Odsazení", da: "Indrykning", de: "Einrückung", el: "Εσοχή",
    en: "Indentation", es: "Sangría", fr: "Indentation", hi: "इंडेंटेशन",
    it: "Rientro", ja: "インデント", nl: "Inspringing", no: "Innrykk",
    pl: "Wcięcie", pt: "Indentação", sv: "Indrag", zh: "缩进"
  },
  'codeStudio.settingsIndentSpaces': {
    cs: "Mezery", da: "Mellemrum", de: "Leerzeichen", el: "Κενά",
    en: "Spaces", es: "Espacios", fr: "Espaces", hi: "स्पेस",
    it: "Spazi", ja: "スペース", nl: "Spaties", no: "Mellomrom",
    pl: "Spacje", pt: "Espaços", sv: "Blanksteg", zh: "空格"
  },
  'codeStudio.settingsIndentTabs': {
    cs: "Tabulátory", da: "Tabulatorer", de: "Tabulatoren", el: "Στηλοθέτες",
    en: "Tabs", es: "Tabulaciones", fr: "Tabulations", hi: "टैब",
    it: "Tabulazioni", ja: "タブ", nl: "Tabs", no: "Tabulatorer",
    pl: "Tabulatory", pt: "Tabulações", sv: "Tabbar", zh: "制表符"
  },
  'codeStudio.settingsWordWrap': {
    cs: "Zalamování řádků", da: "Linjeombrydning", de: "Zeilenumbruch", el: "Αναδίπλωση λέξεων",
    en: "Word wrap", es: "Ajuste de línea", fr: "Retour automatique à la ligne", hi: "शब्द रैप",
    it: "A capo automatico", ja: "折り返し", nl: "Regelterugloop", no: "Linjebryting",
    pl: "Zawijanie wierszy", pt: "Quebra de linha", sv: "Radbrytning", zh: "自动换行"
  },
  'codeStudio.settingsFontSize': {
    cs: "Velikost písma", da: "Skriftstørrelse", de: "Schriftgröße", el: "Μέγεθος γραμματοσειράς",
    en: "Font size", es: "Tamaño de fuente", fr: "Taille de police", hi: "फ़ॉन्ट आकार",
    it: "Dimensione carattere", ja: "フォントサイズ", nl: "Lettergrootte", no: "Skriftstørrelse",
    pl: "Rozmiar czcionki", pt: "Tamanho da letra", sv: "Teckenstorlek", zh: "字体大小"
  },
  'codeStudio.settingsAutoSave': {
    cs: "Automatické ukládání", da: "Automatisk lagring", de: "Automatisch speichern", el: "Αυτόματη αποθήκευση",
    en: "Auto save", es: "Guardado automático", fr: "Enregistrement automatique", hi: "स्वतः सहेजें",
    it: "Salvataggio automatico", ja: "自動保存", nl: "Automatisch opslaan", no: "Automatisk lagring",
    pl: "Autozapis", pt: "Guardar automaticamente", sv: "Spara automatiskt", zh: "自动保存"
  },
  'codeStudio.settingsAutoSaveOff': {
    cs: "Vypnuto", da: "Fra", de: "Aus", el: "Ανενεργή",
    en: "Off", es: "Desactivado", fr: "Désactivé", hi: "बंद",
    it: "Disattivato", ja: "オフ", nl: "Uit", no: "Av",
    pl: "Wyłączony", pt: "Desligado", sv: "Av", zh: "关闭"
  },
  'codeStudio.settingsAutoSaveDelay': {
    cs: "Po 1 sekundě nečinnosti", da: "Efter 1 sekunds pause", de: "Nach 1 Sekunde Pause", el: "Μετά από παύση 1 δευτερολέπτου",
    en: "After a 1 second pause", es: "Tras una pausa de 1 segundo", fr: "Après une pause de 1 seconde", hi: "1 सेकंड रुकने के बाद",
    it: "Dopo una pausa di 1 secondo", ja: "1 秒間の停止後", nl: "Na een pauze van 1 seconde", no: "Etter 1 sekunds pause",
    pl: "Po 1 sekundzie przerwy", pt: "Após uma pausa de 1 segundo", sv: "Efter 1 sekunds paus", zh: "停顿 1 秒后"
  },
  'codeStudio.settingsTrimWhitespace': {
    cs: "Při uložení odstranit koncové mezery", da: "Fjern efterstillede mellemrum ved lagring", de: "Leerzeichen am Zeilenende beim Speichern entfernen", el: "Αφαίρεση κενών στο τέλος γραμμών κατά την αποθήκευση",
    en: "Trim trailing whitespace on save", es: "Eliminar espacios finales al guardar", fr: "Supprimer les espaces de fin à l'enregistrement", hi: "सहेजते समय पंक्ति के अंत की खाली जगह हटाएँ",
    it: "Rimuovi spazi finali al salvataggio", ja: "保存時に末尾の空白を削除", nl: "Witruimte aan regeleinde verwijderen bij opslaan", no: "Fjern etterfølgende mellomrom ved lagring",
    pl: "Usuń końcowe spacje przy zapisie", pt: "Remover espaços finais ao guardar", sv: "Ta bort avslutande blanksteg vid sparande", zh: "保存时删除行尾空白"
  },
  'codeStudio.settingsFinalNewline': {
    cs: "Při uložení vložit závěrečný nový řádek", da: "Indsæt afsluttende linjeskift ved lagring", de: "Beim Speichern abschließenden Zeilenumbruch einfügen", el: "Εισαγωγή τελικής νέας γραμμής κατά την αποθήκευση",
    en: "Insert final newline on save", es: "Insertar salto de línea final al guardar", fr: "Insérer un saut de ligne final à l'enregistrement", hi: "सहेजते समय अंत में नई पंक्ति जोड़ें",
    it: "Inserisci nuova riga finale al salvataggio", ja: "保存時に最終行に改行を挿入", nl: "Afsluitende nieuwe regel invoegen bij opslaan", no: "Sett inn avsluttende linjeskift ved lagring",
    pl: "Wstaw końcowy znak nowego wiersza przy zapisie", pt: "Inserir nova linha final ao guardar", sv: "Infoga avslutande radbrytning vid sparande", zh: "保存时插入末尾换行符"
  },
  'codeStudio.settingsFormatOnSave': {
    cs: "Formátovat při uložení", da: "Formatér ved lagring", de: "Beim Speichern formatieren", el: "Μορφοποίηση κατά την αποθήκευση",
    en: "Format on save", es: "Dar formato al guardar", fr: "Mettre en forme à l'enregistrement", hi: "सहेजते समय फ़ॉर्मेट करें",
    it: "Formatta al salvataggio", ja: "保存時にフォーマット", nl: "Opmaken bij opslaan", no: "Formater ved lagring",
    pl: "Formatuj przy zapisie", pt: "Formatar ao guardar", sv: "Formatera vid sparande", zh: "保存时格式化"
  },
  'codeStudio.statusSpaces': {
    cs: "Mezery: {{size}}", da: "Mellemrum: {{size}}", de: "Leerzeichen: {{size}}", el: "Κενά: {{size}}",
    en: "Spaces: {{size}}", es: "Espacios: {{size}}", fr: "Espaces : {{size}}", hi: "स्पेस: {{size}}",
    it: "Spazi: {{size}}", ja: "スペース: {{size}}", nl: "Spaties: {{size}}", no: "Mellomrom: {{size}}",
    pl: "Spacje: {{size}}", pt: "Espaços: {{size}}", sv: "Blanksteg: {{size}}", zh: "空格: {{size}}"
  },
  'codeStudio.statusTabSize': {
    cs: "Velikost tabulátoru: {{size}}", da: "Tabulatorstørrelse: {{size}}", de: "Tabulatorbreite: {{size}}", el: "Μέγεθος στηλοθέτη: {{size}}",
    en: "Tab Size: {{size}}", es: "Tamaño de tabulación: {{size}}", fr: "Taille de tabulation : {{size}}", hi: "टैब आकार: {{size}}",
    it: "Dimensione tabulazione: {{size}}", ja: "タブのサイズ: {{size}}", nl: "Tabgrootte: {{size}}", no: "Tabulatorbredde: {{size}}",
    pl: "Rozmiar tabulacji: {{size}}", pt: "Tamanho da tabulação: {{size}}", sv: "Tabbstorlek: {{size}}", zh: "制表符大小: {{size}}"
  },
  'codeStudio.formatUnavailable': {
    cs: "Pro {{language}} není k dispozici žádný formátovač", da: "Ingen formatering tilgængelig for {{language}}", de: "Kein Formatierer für {{language}} verfügbar", el: "Δεν υπάρχει διαθέσιμος μορφοποιητής για {{language}}",
    en: "No formatter available for {{language}}", es: "No hay formateador disponible para {{language}}", fr: "Aucun outil de mise en forme disponible pour {{language}}", hi: "{{language}} के लिए कोई फ़ॉर्मेटर उपलब्ध नहीं है",
    it: "Nessun formattatore disponibile per {{language}}", ja: "{{language}} 用のフォーマッターはありません", nl: "Geen opmaakprogramma beschikbaar voor {{language}}", no: "Ingen formaterer tilgjengelig for {{language}}",
    pl: "Brak dostępnego formatera dla {{language}}", pt: "Nenhum formatador disponível para {{language}}", sv: "Ingen formaterare tillgänglig för {{language}}", zh: "没有适用于 {{language}} 的格式化程序"
  },
  'codeStudio.formatFailed': {
    cs: "Formátování selhalo. Podrobnosti najdete v panelu Výstup.", da: "Formateringen mislykkedes. Detaljer findes i Output-panelet.", de: "Formatieren fehlgeschlagen. Details stehen im Ausgabe-Panel.", el: "Η μορφοποίηση απέτυχε. Λεπτομέρειες στον πίνακα Έξοδος.",
    en: "Formatting failed. Details are in the Output panel.", es: "No se pudo dar formato. Los detalles están en el panel Salida.", fr: "La mise en forme a échoué. Les détails sont dans le panneau Sortie.", hi: "फ़ॉर्मेट करना विफल रहा। विवरण आउटपुट पैनल में है।",
    it: "Formattazione non riuscita. I dettagli sono nel pannello Output.", ja: "フォーマットに失敗しました。詳細は出力パネルにあります。", nl: "Opmaken mislukt. Details staan in het paneel Uitvoer.", no: "Formateringen mislyktes. Detaljer finnes i Utdata-panelet.",
    pl: "Formatowanie nie powiodło się. Szczegóły są w panelu Wyjście.", pt: "A formatação falhou. Os detalhes estão no painel Saída.", sv: "Formateringen misslyckades. Detaljer finns i panelen Utdata.", zh: "格式化失败。详细信息见输出面板。"
  },
  'codeStudio.formatDone': {
    cs: "Dokument byl naformátován", da: "Dokumentet er formateret", de: "Dokument formatiert", el: "Το έγγραφο μορφοποιήθηκε",
    en: "Document formatted", es: "Documento formateado", fr: "Document mis en forme", hi: "दस्तावेज़ फ़ॉर्मेट हो गया",
    it: "Documento formattato", ja: "ドキュメントをフォーマットしました", nl: "Document opgemaakt", no: "Dokumentet er formatert",
    pl: "Dokument sformatowany", pt: "Documento formatado", sv: "Dokumentet har formaterats", zh: "文档已格式化"
  },
  'codeStudio.formatUnchanged': {
    cs: "Dokument je již naformátován", da: "Dokumentet er allerede formateret", de: "Das Dokument ist bereits formatiert", el: "Το έγγραφο είναι ήδη μορφοποιημένο",
    en: "The document is already formatted", es: "El documento ya tiene formato", fr: "Le document est déjà mis en forme", hi: "दस्तावेज़ पहले से फ़ॉर्मेट है",
    it: "Il documento è già formattato", ja: "ドキュメントは既にフォーマット済みです", nl: "Het document is al opgemaakt", no: "Dokumentet er allerede formatert",
    pl: "Dokument jest już sformatowany", pt: "O documento já está formatado", sv: "Dokumentet är redan formaterat", zh: "文档已经是格式化的"
  },
  'codeStudio.formatStale': {
    cs: "Dokument se během formátování změnil. Naformátujte ho znovu.", da: "Dokumentet blev ændret under formateringen. Formatér det igen.", de: "Das Dokument wurde während der Formatierung geändert. Bitte erneut formatieren.", el: "Το έγγραφο άλλαξε κατά τη μορφοποίηση. Μορφοποιήστε το ξανά.",
    en: "The document changed while it was being formatted. Format it again.", es: "El documento cambió mientras se formateaba. Vuelve a darle formato.", fr: "Le document a changé pendant la mise en forme. Relancez la mise en forme.", hi: "फ़ॉर्मेट करते समय दस्तावेज़ बदल गया। इसे फिर से फ़ॉर्मेट करें।",
    it: "Il documento è cambiato durante la formattazione. Formattalo di nuovo.", ja: "フォーマット中にドキュメントが変更されました。もう一度フォーマットしてください。", nl: "Het document is tijdens het opmaken gewijzigd. Maak het opnieuw op.", no: "Dokumentet ble endret under formateringen. Formater det på nytt.",
    pl: "Dokument zmienił się podczas formatowania. Sformatuj go ponownie.", pt: "O documento mudou durante a formatação. Formate-o novamente.", sv: "Dokumentet ändrades under formateringen. Formatera det igen.", zh: "格式化期间文档已更改。请重新格式化。"
  },
  'codeStudio.formatTruncated': {
    cs: "Výstup formátovače byl příliš velký a nebyl použit.", da: "Formateringens output var for stort og blev ikke anvendt.", de: "Die Ausgabe des Formatierers war zu groß und wurde nicht übernommen.", el: "Η έξοδος του μορφοποιητή ήταν πολύ μεγάλη και δεν εφαρμόστηκε.",
    en: "The formatter output was too large and was not applied.", es: "La salida del formateador era demasiado grande y no se aplicó.", fr: "La sortie de l'outil de mise en forme était trop volumineuse et n'a pas été appliquée.", hi: "फ़ॉर्मेटर का आउटपुट बहुत बड़ा था और लागू नहीं किया गया।",
    it: "L'output del formattatore era troppo grande e non è stato applicato.", ja: "フォーマッターの出力が大きすぎるため適用されませんでした。", nl: "De uitvoer van het opmaakprogramma was te groot en is niet toegepast.", no: "Utdataene fra formatereren var for store og ble ikke brukt.",
    pl: "Wynik formatera był za duży i nie został zastosowany.", pt: "A saída do formatador era demasiado grande e não foi aplicada.", sv: "Formaterarens utdata var för stor och användes inte.", zh: "格式化程序的输出过大，未应用。"
  },
  'codeStudio.formatOutputSource': {
    cs: "Formátování", da: "Formatering", de: "Formatieren", el: "Μορφοποίηση",
    en: "Format", es: "Formato", fr: "Mise en forme", hi: "फ़ॉर्मेट",
    it: "Formattazione", ja: "フォーマット", nl: "Opmaak", no: "Formatering",
    pl: "Formatowanie", pt: "Formatação", sv: "Formatering", zh: "格式化"
  }
};

const placeholders = text => (String(text).match(/\{\{\w+\}\}/g) || []).sort().join(',');
for (const [key, values] of Object.entries(KEYS)) {
  for (const lang of LANGS) {
    if (typeof values[lang] !== 'string' || !values[lang].trim()) throw new Error(`${key}: missing ${lang}`);
    if (placeholders(values[lang]) !== placeholders(values.en)) throw new Error(`${key}: ${lang} placeholders differ from en`);
  }
}

for (const lang of LANGS) {
  const file = join(root, 'ui', 'lang', 'desktop', `${lang}.json`);
  const raw = readFileSync(file, 'utf8');
  const bom = raw.charCodeAt(0) === 0xFEFF ? raw.slice(0, 1) : '';
  const text = bom ? raw.slice(1) : raw;
  const eol = text.includes('\r\n') ? '\r\n' : '\n';
  const existing = JSON.parse(text);
  const lines = text.split(eol);
  let last = -1;
  lines.forEach((line, index) => { if (/^\s*"codeStudio\.[^"]+"\s*:/.test(line)) last = index; });
  if (last < 0) throw new Error(`${lang}: no codeStudio.* key found`);
  const indent = /^\s*/.exec(lines[last])[0];
  const additions = Object.keys(KEYS)
    .filter(key => !Object.prototype.hasOwnProperty.call(existing, key))
    .map(key => `${indent}${JSON.stringify(key)}: ${JSON.stringify(KEYS[key][lang])}`);
  if (!additions.length) {
    console.log(`${lang}: nothing to add`);
    continue;
  }
  const lastHadComma = /,\s*$/.test(lines[last]);
  if (!lastHadComma) lines[last] = lines[last].replace(/\s*$/, ',');
  const block = additions.map((line, index) => (index < additions.length - 1 || lastHadComma ? line + ',' : line));
  lines.splice(last + 1, 0, ...block);
  const next = lines.join(eol);
  const parsed = JSON.parse(next);
  for (const key of Object.keys(KEYS)) {
    const want = Object.prototype.hasOwnProperty.call(existing, key) ? existing[key] : KEYS[key][lang];
    if (parsed[key] !== want) throw new Error(`${lang}: ${key} was not written`);
  }
  writeFileSync(file, bom + next, 'utf8');
  console.log(`${lang}: added ${additions.length} keys`);
}
```

- [ ] **Step 2: Run it once**

```
node C:\disposable\code-studio-pkg1\locales-a.mjs C:\Users\Andi\Documents\repo\AuraGo
```
Expected: 16 lines of the form `<lang>: added 43 keys`.

- [ ] **Step 3: Verify the locale files parse**

```
go test ./ui -count=1 -v -run "(?i)^TestDesktopCodeStudio(ShellI18n|CopyI18n|CursorI18n)$"
```
Expected: 3 `--- PASS` lines.

**Commit:** `feat(code-studio): add editor package locale strings`. Files: `ui/lang/desktop/cs.json`, `ui/lang/desktop/da.json`, `ui/lang/desktop/de.json`, `ui/lang/desktop/el.json`, `ui/lang/desktop/en.json`, `ui/lang/desktop/es.json`, `ui/lang/desktop/fr.json`, `ui/lang/desktop/hi.json`, `ui/lang/desktop/it.json`, `ui/lang/desktop/ja.json`, `ui/lang/desktop/nl.json`, `ui/lang/desktop/no.json`, `ui/lang/desktop/pl.json`, `ui/lang/desktop/pt.json`, `ui/lang/desktop/sv.json`, `ui/lang/desktop/zh.json`.

---

### Task 4: Folding, bracket matching, indent on input and the new languages (A1, A2)

**Files:**
- Modify: `ui/js/desktop/apps/code-studio/editor.js`, `ui/js/desktop/apps/code-studio/core.js`, `ui/css/code-studio.css`
- Generate: `ui/js/desktop/bundles/code-studio.bundle.js`

- [ ] **Step 1: Impact check.** Run `impact` upstream for `createCodeMirrorEditor`, `languageForPath` and `languageExtension`. `languageForPath` has many callers (icons, outline, run, problems, status); the change only adds languages.

- [ ] **Step 2: Add folding, bracket matching and indent on input to the editor**

In `ui/js/desktop/apps/code-studio/editor.js`, replace:
```js
            cm.highlightSelectionMatches && cm.highlightSelectionMatches(),
            languageExtension(cm, tab.language),
```
With:
```js
            cm.highlightSelectionMatches && cm.highlightSelectionMatches(),
            // IDE package 1A: code folding with a fold gutter, bracket matching and indentation on input.
            cm.codeFolding && cm.codeFolding(),
            cm.foldGutter && cm.foldGutter(),
            cm.bracketMatching && cm.bracketMatching(),
            cm.indentOnInput && cm.indentOnInput(),
            languageExtension(cm, tab.language),
```

- [ ] **Step 3: Bind the fold keys and free Ctrl+G for Go to Line**

In `editor.js`, replace:
```js
                ...(cm.defaultKeymap || []),
                // The studio ships its own find widget, so CodeMirror's search panel is skipped.
                ...(cm.searchKeymap || []).filter(binding => binding.key !== 'Mod-f'),
```
With:
```js
                ...(cm.defaultKeymap || []),
                // IDE package 1A: Ctrl+Shift+[ folds, Ctrl+Shift+] unfolds.
                ...(cm.foldKeymap || []),
                // The studio ships its own find widget, so CodeMirror's search panel is skipped.
                // Ctrl+G belongs to the studio's Go to Line, so CodeMirror's find-next binding is skipped too.
                ...(cm.searchKeymap || []).filter(binding => binding.key !== 'Mod-f' && binding.key !== 'Mod-g'),
```

- [ ] **Step 4: Map the new file types**

In `ui/js/desktop/apps/code-studio/core.js`, replace:
```js
    function languageForPath(path) {
        const ext = String(path || '').split('.').pop().toLowerCase();
        return ({ js: 'javascript', mjs: 'javascript', cjs: 'javascript', jsx: 'jsx', ts: 'typescript', tsx: 'tsx', py: 'python', go: 'go', c: 'c', h: 'c', rs: 'rust', json: 'json', html: 'html', htm: 'html', css: 'css', md: 'markdown' })[ext] || '';
    }
```
With:
```js
    function languageForPath(path) {
        // IDE package 1A: a Dockerfile is recognised by its name (Dockerfile, Dockerfile.dev, app.dockerfile).
        const name = String(path || '').split('/').pop();
        if (/^dockerfile(\.|$)/i.test(name) || /\.dockerfile$/i.test(name)) return 'dockerfile';
        const ext = String(path || '').split('.').pop().toLowerCase();
        return ({ js: 'javascript', mjs: 'javascript', cjs: 'javascript', jsx: 'jsx', ts: 'typescript', tsx: 'tsx', py: 'python', go: 'go', c: 'c', h: 'c', rs: 'rust', json: 'json', html: 'html', htm: 'html', css: 'css', md: 'markdown', yml: 'yaml', yaml: 'yaml', sql: 'sql', java: 'java', php: 'php', xml: 'xml', svg: 'xml', sh: 'shell', bash: 'shell', toml: 'toml' })[ext] || '';
    }
```

- [ ] **Step 5: Give the new languages their CodeMirror support**

In `core.js`, replace:
```js
            css: () => cm.css && cm.css(),
            markdown: () => cm.markdown && cm.markdown()
        };
```
With:
```js
            css: () => cm.css && cm.css(),
            markdown: () => cm.markdown && cm.markdown(),
            // IDE package 1A
            yaml: () => cm.yaml && cm.yaml(),
            sql: () => cm.sql && cm.sql(),
            java: () => cm.java && cm.java(),
            php: () => cm.php && cm.php(),
            xml: () => cm.xml && cm.xml(),
            shell: () => streamLanguageSupport(cm, cm.shell),
            dockerfile: () => streamLanguageSupport(cm, cm.dockerFile),
            toml: () => streamLanguageSupport(cm, cm.toml)
        };
```

Then replace:
```js
        const support = factory();
        return support ? [support] : [];
    }

    function runCommandFor(path) {
```
With:
```js
        const support = factory();
        return support ? [support] : [];
    }

    // IDE package 1A: legacy modes (shell, Dockerfile, TOML) become a language through StreamLanguage, once per page.
    const streamLanguageCache = new Map();

    function streamLanguageSupport(cm, parser) {
        if (!cm.StreamLanguage || !parser) return null;
        if (!streamLanguageCache.has(parser)) streamLanguageCache.set(parser, cm.StreamLanguage.define(parser));
        return streamLanguageCache.get(parser);
    }

    function runCommandFor(path) {
```
The helper must stay between `languageExtension` and `runCommandFor`. `TestDesktopCodeStudioRunUsesTerminalSession` cuts `languageForPath` and `runCommandFor` out of `core.js` by those function names.

- [ ] **Step 6: Style the fold gutter (start of the package 1A CSS block)**

Append to the end of `ui/css/code-studio.css`:
```css

/* === IDE package 1A: editor === */

/* Fold gutter and the placeholder of a folded range follow the studio palette in light and dark themes. The
   .cm-editor step outranks CodeMirror's own theme rules. */
.code-studio .cm-editor .cm-foldGutter .cm-gutterElement {
    padding: 0 4px;
    color: var(--cs-muted);
    cursor: pointer;
}

.code-studio .cm-editor .cm-foldGutter .cm-gutterElement:hover {
    color: var(--cs-accent);
}

.code-studio .cm-editor .cm-foldPlaceholder {
    margin: 0 2px;
    padding: 0 6px;
    border: 1px solid var(--cs-accent-border);
    border-radius: var(--cs-radius-sm);
    color: var(--cs-text);
    background: var(--cs-accent-soft);
}
```

- [ ] **Step 7: Build and check**

```
npm run build:ui
npm run build:ui -- --check
node --check ui/js/desktop/bundles/code-studio.bundle.js
go test ./ui -count=1 -v -run "(?i)^TestDesktopCodeStudioRunUsesTerminalSession$"
```
Expected: the check prints `Checked ui/js/desktop/bundles/code-studio.bundle.js`, `node --check` is silent, and there is 1 `--- PASS`.

**Commit:** `feat(code-studio): fold code, match brackets and highlight more languages`. Files: `ui/js/desktop/apps/code-studio/editor.js`, `ui/js/desktop/apps/code-studio/core.js`, `ui/css/code-studio.css`, `ui/js/desktop/bundles/code-studio.bundle.js`.

---

### Task 5: Editor settings dialog, live settings compartment and auto save (A5)

**Files:**
- Create: `ui/js/desktop/apps/code-studio/settings.js`
- Modify: `ui/js/desktop/apps/code-studio/editor.js`, `ui/js/desktop/apps/code-studio/core.js`, `ui/js/desktop/apps/code-studio/sidebar.js`, `scripts/build-ui-bundles.js`, `ui/css/code-studio.css`
- Generate: `ui/js/desktop/bundles/code-studio.bundle.js`

The defaults (4 spaces, word wrap on, everything else off) equal today's editor. The trim, final newline and format-on-save options are stored here; Task 6 adds the save pipeline that runs them.

- [ ] **Step 1: Impact check.** Run `impact` upstream for `createCodeMirrorEditor`, `createTextareaEditor`, `renderStatus`, `shellMarkup` and `renderActivityBar`.

- [ ] **Step 2: Create `ui/js/desktop/apps/code-studio/settings.js`**

```js
    // IDE package 1A: editor settings (A5). One settings object lives in localStorage and is shared by every Code
    // Studio window of the page. The defaults are the editor's behaviour before the settings existed: 4 spaces, word
    // wrap on, everything else off. Every CodeMirror view keeps the indentation, tab size and word wrap extensions in
    // a compartment of its own (view.csSettings), so a change reaches the open editors live.
    const EDITOR_SETTINGS_KEY = 'aurago.codeStudio.settings.v1';
    const EDITOR_TAB_SIZES = [2, 4, 8];
    const AUTO_SAVE_DELAY_MS = 1000;
    let editorSettingsCache = null;

    function normalizeEditorSettings(raw) {
        const value = raw && typeof raw === 'object' ? raw : {};
        const tabSize = Number(value.tabSize);
        return {
            tabSize: EDITOR_TAB_SIZES.includes(tabSize) ? tabSize : 4,
            insertSpaces: value.insertSpaces !== false,
            wordWrap: value.wordWrap !== false,
            autoSave: value.autoSave === 'afterDelay' ? 'afterDelay' : 'off',
            trimTrailingWhitespace: value.trimTrailingWhitespace === true,
            insertFinalNewline: value.insertFinalNewline === true,
            formatOnSave: value.formatOnSave === true
        };
    }

    function editorSettings() {
        if (!editorSettingsCache) {
            let stored = null;
            try {
                stored = JSON.parse(localStorage.getItem(EDITOR_SETTINGS_KEY) || 'null');
            } catch (_) {
                stored = null;
            }
            editorSettingsCache = normalizeEditorSettings(stored);
        }
        return editorSettingsCache;
    }

    function updateEditorSettings(patch) {
        editorSettingsCache = normalizeEditorSettings(Object.assign({}, editorSettings(), patch || {}));
        try {
            localStorage.setItem(EDITOR_SETTINGS_KEY, JSON.stringify(editorSettingsCache));
        } catch (_) {}
        applyEditorSettingsToOpenEditors();
        return editorSettingsCache;
    }

    // The extensions behind view.csSettings: indent unit, tab size and word wrap.
    function editorSettingsExtensions(cm) {
        const settings = editorSettings();
        const extensions = [];
        if (cm.indentUnit) extensions.push(cm.indentUnit.of(settings.insertSpaces ? ' '.repeat(settings.tabSize) : '\t'));
        if (cm.EditorState.tabSize) extensions.push(cm.EditorState.tabSize.of(settings.tabSize));
        if (settings.wordWrap && cm.EditorView.lineWrapping) extensions.push(cm.EditorView.lineWrapping);
        return extensions;
    }

    // Reconfigures the settings compartment of every mounted editor in every Code Studio window.
    function applyEditorSettingsToOpenEditors() {
        instances.forEach(instance => runWithInstance(instance, () => {
            const cm = state.cmModule;
            if (cm) {
                state.openTabs.forEach(tab => (tab.views || []).forEach(view => {
                    if (!view || !view.csSettings) return;
                    try { view.dispatch({ effects: view.csSettings.reconfigure(editorSettingsExtensions(cm)) }); } catch (_) {}
                }));
            }
            if (studioRoot()) renderStatus();
        }));
    }

    // Status bar entry "Spaces: 4" or "Tab Size: 4"; a click opens the settings.
    function editorIndentStatusMarkup() {
        const settings = editorSettings();
        const label = settings.insertSpaces
            ? tr('codeStudio.statusSpaces', 'Spaces: {{size}}', { size: settings.tabSize })
            : tr('codeStudio.statusTabSize', 'Tab Size: {{size}}', { size: settings.tabSize });
        return `<span class="cs-status-item" data-clickable data-status-indent title="${esc(tr('codeStudio.settingsTitle', 'Editor Settings'))}">${esc(label)}</span>`;
    }

    // The font size of the dialog is the existing zoom state of the window.
    function setEditorFontSize(size) {
        const next = clampEditorFontSize(size);
        if (next === state.editorFontSize) return;
        state.editorFontSize = next;
        applyEditorZoom();
        saveState();
        renderWindowMenus();
        renderStatus();
    }

    function openEditorSettings() {
        const instance = state;
        if (!instance || instance.settingsClose) return;
        const settings = editorSettings();
        const title = tr('codeStudio.settingsTitle', 'Editor Settings');
        const option = (value, label, selected) => `<option value="${esc(value)}"${selected ? ' selected' : ''}>${esc(label)}</option>`;
        const checkbox = (key, label) => `<label class="cs-settings-check"><input type="checkbox" data-setting="${key}"${settings[key] ? ' checked' : ''}><span>${esc(label)}</span></label>`;
        const overlay = document.createElement('div');
        overlay.className = 'cs-modal-backdrop';
        overlay.innerHTML = `<form class="cs-modal cs-settings-modal" role="dialog" aria-modal="true" aria-label="${esc(title)}" data-cs-settings>
            <h3 class="cs-settings-title">${esc(title)}</h3>
            <div class="cs-settings-grid">
                <label class="cs-settings-row"><span>${esc(tr('codeStudio.settingsTabSize', 'Tab size'))}</span>
                    <select data-setting="tabSize">${EDITOR_TAB_SIZES.map(size => option(size, String(size), settings.tabSize === size)).join('')}</select></label>
                <label class="cs-settings-row"><span>${esc(tr('codeStudio.settingsIndentation', 'Indentation'))}</span>
                    <select data-setting="insertSpaces">${option('spaces', tr('codeStudio.settingsIndentSpaces', 'Spaces'), settings.insertSpaces)}${option('tabs', tr('codeStudio.settingsIndentTabs', 'Tabs'), !settings.insertSpaces)}</select></label>
                <label class="cs-settings-row"><span>${esc(tr('codeStudio.settingsFontSize', 'Font size'))}</span>
                    <input type="number" data-setting="fontSize" min="${MIN_EDITOR_FONT_SIZE}" max="${MAX_EDITOR_FONT_SIZE}" step="1" value="${esc(String(clampEditorFontSize(instance.editorFontSize)))}"></label>
                <label class="cs-settings-row"><span>${esc(tr('codeStudio.settingsAutoSave', 'Auto save'))}</span>
                    <select data-setting="autoSave">${option('off', tr('codeStudio.settingsAutoSaveOff', 'Off'), settings.autoSave !== 'afterDelay')}${option('afterDelay', tr('codeStudio.settingsAutoSaveDelay', 'After a 1 second pause'), settings.autoSave === 'afterDelay')}</select></label>
                ${checkbox('wordWrap', tr('codeStudio.settingsWordWrap', 'Word wrap'))}
                ${checkbox('trimTrailingWhitespace', tr('codeStudio.settingsTrimWhitespace', 'Trim trailing whitespace on save'))}
                ${checkbox('insertFinalNewline', tr('codeStudio.settingsFinalNewline', 'Insert final newline on save'))}
                ${checkbox('formatOnSave', tr('codeStudio.settingsFormatOnSave', 'Format on save'))}
            </div>
            <div class="cs-modal-actions">
                <button type="submit" class="cs-button primary" data-settings-close>${buttonIcon('check-square', 'Y')}<span>${esc(tr('desktop.close', 'Close'))}</span></button>
            </div>
        </form>`;
        document.body.appendChild(overlay);
        const releaseFocus = trapDialogFocus(overlay);
        let unregister = () => {};
        let closed = false;
        const close = () => {
            if (closed) return;
            closed = true;
            unregister();
            instance.settingsClose = null;
            releaseFocus();
            overlay.remove();
        };
        unregister = registerDisposer(close);
        instance.settingsClose = close;
        overlay.querySelector('form').addEventListener('submit', event => {
            event.preventDefault();
            close();
        });
        overlay.addEventListener('mousedown', event => {
            if (event.target === overlay) close();
        });
        overlay.addEventListener('keydown', event => {
            if (event.key !== 'Escape') return;
            event.preventDefault();
            event.stopPropagation();
            close();
        });
        // Every change applies at once; the dialog has no separate apply step.
        overlay.addEventListener('change', bindInstance(instance, event => {
            const control = event.target && typeof event.target.closest === 'function' ? event.target.closest('[data-setting]') : null;
            if (!control) return;
            const key = control.dataset.setting;
            if (key === 'fontSize') {
                setEditorFontSize(control.value);
                control.value = String(state.editorFontSize);
            } else if (key === 'tabSize') {
                updateEditorSettings({ tabSize: Number(control.value) });
            } else if (key === 'insertSpaces') {
                updateEditorSettings({ insertSpaces: control.value !== 'tabs' });
            } else if (key === 'autoSave') {
                updateEditorSettings({ autoSave: control.value });
            } else {
                updateEditorSettings({ [key]: control.checked === true });
            }
        }));
        const first = overlay.querySelector('[data-setting]');
        if (first) first.focus();
    }

    // Auto save writes a dirty file tab after a 1 s pause. Read-only windows and virtual tabs (Git diff) never save,
    // and auto save writes the buffer as it is: the save actions run on explicit saves only.
    function scheduleAutoSave(tab) {
        const instance = state;
        if (!instance || !isSourceTab(tab) || !tab.modified || editorSettings().autoSave !== 'afterDelay' || !canWrite(instance)) return;
        if (tab.autoSaveTimer) clearTimeout(tab.autoSaveTimer);
        tab.autoSaveTimer = setTimeout(bindInstance(instance, () => {
            tab.autoSaveTimer = null;
            if (!tab.modified || !state.openTabs.includes(tab) || editorSettings().autoSave !== 'afterDelay') return;
            saveTab(state, tab);
        }), AUTO_SAVE_DELAY_MS);
    }
```

- [ ] **Step 3: Add `settings.js` to the bundle before `shortcuts.js`**

In `scripts/build-ui-bundles.js`, replace:
```js
      'ui/js/desktop/apps/code-studio/panels.js',
      'ui/js/desktop/apps/code-studio/shortcuts.js',
```
With:
```js
      'ui/js/desktop/apps/code-studio/panels.js',
      // IDE package 1A fragments. They share the IIFE that core.js opens and shortcuts.js closes, so they sit
      // before shortcuts.js; they only declare functions and constants, nothing runs before render().
      'ui/js/desktop/apps/code-studio/settings.js',
      'ui/js/desktop/apps/code-studio/shortcuts.js',
```

- [ ] **Step 4: Add `isSourceTab` next to `focusedEditorView`**

In `editor.js`, replace:
```js
    function focusedEditorView(tab) {
        return tab && (tab.focusedView || tab.view);
    }
```
With:
```js
    function focusedEditorView(tab) {
        return tab && (tab.focusedView || tab.view);
    }

    // IDE package 1A: a tab that holds a workspace file. Virtual tabs (Git diff, later the commit view) carry a kind;
    // file tabs never do.
    function isSourceTab(tab) {
        return !!tab && !tab.kind;
    }
```

- [ ] **Step 5: Create the per-view settings compartment**

In `editor.js`, replace:
```js
        if (!cm || !cm.EditorState || !cm.EditorView) return createTextareaEditor(container, tab, link);
        const light = usesLightEditorTheme();
```
With:
```js
        if (!cm || !cm.EditorState || !cm.EditorView) return createTextareaEditor(container, tab, link);
        const light = usesLightEditorTheme();
        // IDE package 1A: indentation, tab size and word wrap follow the editor settings live (view.csSettings).
        const settingsCompartment = new cm.Compartment();
```

Then replace:
```js
            cm.indentUnit && cm.indentUnit.of('    '),
            cm.EditorView.lineWrapping,
```
With:
```js
            settingsCompartment.of(editorSettingsExtensions(cm)),
```

Then replace:
```js
            parent: container
        });
        view.csScheme = scheme;
```
With:
```js
            parent: container
        });
        view.csSettings = settingsCompartment;
        view.csScheme = scheme;
```

- [ ] **Step 6: Schedule auto save on edits (CodeMirror and the textarea fallback)**

In `editor.js`, replace:
```js
                if (!update.docChanged) return;
                updateTabContent(tab, update.state.doc.toString());
                renderTabs();
                renderStatus();
```
With:
```js
                if (!update.docChanged) return;
                updateTabContent(tab, update.state.doc.toString());
                scheduleAutoSave(tab);
                renderTabs();
                renderStatus();
```

Then replace:
```js
        const updatePreview = bind(() => {
            updateTabContent(tab, textarea.value);
```
With:
```js
        const updatePreview = bind(() => {
            updateTabContent(tab, textarea.value);
            scheduleAutoSave(tab);
```

- [ ] **Step 7: Put a gear in the activity bar**

In `core.js` (`shellMarkup`), replace:
```html
                    <span class="cs-activity-spacer"></span>
                    <button type="button" class="cs-activity-btn" data-activity="terminal" title="${esc(tr('codeStudio.toggleTerminal', 'Terminal'))}">
```
With:
```html
                    <span class="cs-activity-spacer"></span>
                    <button type="button" class="cs-activity-btn" data-activity="settings" title="${esc(tr('codeStudio.settingsTitle', 'Editor Settings'))}">
                        <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"><circle cx="12" cy="12" r="3"/><path d="M19.4 15a1.65 1.65 0 0 0 .33 1.82l.06.06a2 2 0 0 1 0 2.83 2 2 0 0 1-2.83 0l-.06-.06a1.65 1.65 0 0 0-1.82-.33 1.65 1.65 0 0 0-1 1.51V21a2 2 0 0 1-2 2 2 2 0 0 1-2-2v-.09A1.65 1.65 0 0 0 9 19.4a1.65 1.65 0 0 0-1.82.33l-.06.06a2 2 0 0 1-2.83 0 2 2 0 0 1 0-2.83l.06-.06a1.65 1.65 0 0 0 .33-1.82 1.65 1.65 0 0 0-1.51-1H3a2 2 0 0 1-2-2 2 2 0 0 1 2-2h.09A1.65 1.65 0 0 0 4.6 9a1.65 1.65 0 0 0-.33-1.82l-.06-.06a2 2 0 0 1 0-2.83 2 2 0 0 1 2.83 0l.06.06a1.65 1.65 0 0 0 1.82.33H9a1.65 1.65 0 0 0 1-1.51V3a2 2 0 0 1 2-2 2 2 0 0 1 2 2v.09a1.65 1.65 0 0 0 1 1.51 1.65 1.65 0 0 0 1.82-.33l.06-.06a2 2 0 0 1 2.83 0 2 2 0 0 1 0 2.83l-.06.06a1.65 1.65 0 0 0-.33 1.82V9a1.65 1.65 0 0 0 1.51 1H21a2 2 0 0 1 2 2 2 2 0 0 1-2 2h-.09a1.65 1.65 0 0 0-1.51 1z"/></svg>
                    </button>
                    <button type="button" class="cs-activity-btn" data-activity="terminal" title="${esc(tr('codeStudio.toggleTerminal', 'Terminal'))}">
```

In `ui/js/desktop/apps/code-studio/sidebar.js` (`renderActivityBar`), replace:
```js
                    else if (act === 'agent') toggleAgentPanel();
                    else if (act === 'terminal') toggleTerminal();
```
With:
```js
                    else if (act === 'agent') toggleAgentPanel();
                    else if (act === 'terminal') toggleTerminal();
                    else if (act === 'settings') openEditorSettings(); // IDE package 1A
```

- [ ] **Step 8: Show "Spaces: 4" or "Tab Size: 4" in the status bar; a click opens the dialog**

In `core.js` (`renderStatus`), replace:
```js
        if (lineInfo) rightItems.push(`<span class="cs-status-item">${esc(lineInfo)}</span>`);
```
With:
```js
        if (lineInfo) rightItems.push(`<span class="cs-status-item">${esc(lineInfo)}</span>`);
        if (isSourceTab(tab)) rightItems.push(editorIndentStatusMarkup()); // IDE package 1A
```

Then replace:
```js
        const zoom = statusEl.querySelector('[data-status-zoom]');
        if (zoom) zoom.addEventListener('click', bind(resetEditorZoom));
```
With:
```js
        const zoom = statusEl.querySelector('[data-status-zoom]');
        if (zoom) zoom.addEventListener('click', bind(resetEditorZoom));
        const indent = statusEl.querySelector('[data-status-indent]');
        if (indent) indent.addEventListener('click', bind(() => openEditorSettings())); // IDE package 1A
```

- [ ] **Step 9: Style the dialog**

Append to the end of `ui/css/code-studio.css`:
```css

/* Editor settings dialog (A5). */
.cs-modal.cs-settings-modal {
    width: min(440px, calc(100vw - 32px));
}

.cs-settings-title {
    margin: 0 0 14px;
    color: var(--cs-text);
    font-size: 15px;
    font-weight: 600;
}

.cs-settings-grid {
    display: grid;
    gap: 10px;
}

.cs-modal .cs-settings-row {
    grid-template-columns: minmax(0, 1fr) minmax(120px, 180px);
    align-items: center;
    gap: 12px;
    font-weight: 500;
}

.cs-modal .cs-settings-row select,
.cs-modal .cs-settings-row input {
    width: 100%;
    height: 32px;
    padding: 0 8px;
    border: 1px solid var(--cs-border);
    border-radius: var(--cs-radius-sm);
    color: var(--cs-text);
    background: var(--cs-panel-soft);
    font: inherit;
}

.cs-modal .cs-settings-row select option {
    color: var(--cs-text);
    background: var(--cs-panel);
}

.cs-modal .cs-settings-check {
    display: flex;
    align-items: center;
    gap: 8px;
    font-weight: 500;
    cursor: pointer;
}

.cs-modal .cs-settings-check input {
    width: 16px;
    height: 16px;
    margin: 0;
    padding: 0;
    accent-color: var(--cs-accent);
    box-shadow: none;
}

.cs-modal .cs-settings-row select:focus-visible,
.cs-modal .cs-settings-row input:focus-visible,
.cs-modal .cs-settings-check input:focus-visible {
    outline: 2px solid var(--cs-accent);
    outline-offset: 1px;
}
```

- [ ] **Step 10: Build and check**

```
npm run build:ui
npm run build:ui -- --check
node --check ui/js/desktop/bundles/code-studio.bundle.js
```

**Commit:** `feat(code-studio): editor settings with live tab size, wrap and auto save`. Files: `ui/js/desktop/apps/code-studio/settings.js`, `ui/js/desktop/apps/code-studio/editor.js`, `ui/js/desktop/apps/code-studio/core.js`, `ui/js/desktop/apps/code-studio/sidebar.js`, `scripts/build-ui-bundles.js`, `ui/css/code-studio.css`, `ui/js/desktop/bundles/code-studio.bundle.js`.

---

### Task 6: Format Document and the save actions (A6)

**Files:**
- Create: `ui/js/desktop/apps/code-studio/format.js`
- Modify: `ui/js/desktop/apps/code-studio/core.js`, `scripts/build-ui-bundles.js`
- Generate: `ui/js/desktop/bundles/code-studio.bundle.js`

**How saving is routed:**
- Explicit saves (Ctrl+S, Save, Save All, Run) go through `saveTabWithActions`: format on save, then trailing whitespace and the final newline, then `saveTab`.
- Auto save and the desktop's revision-checked session save call `saveTab` directly and write the buffer as it is.
- `saveTab` itself is not modified.

- [ ] **Step 1: Impact check.** Run `impact` upstream for `saveCurrentFile`, `saveAllFiles`, `runCurrentFile` and `registerSessionAction`.

- [ ] **Step 2: Create `ui/js/desktop/apps/code-studio/format.js`**

```js
    // IDE package 1A: Format Document (A6) and the save actions of the editor settings.
    // Go runs gofmt in the container and JavaScript/TypeScript the TypeScript formatter of the container; both read
    // the text from stdin through /api/code-studio/exec. JSON is formatted in the browser. Other languages have no
    // formatter in the image and get a notice. A result is applied as one CodeMirror transaction with its own undo
    // step, and only when the text changed.
    const FORMAT_SCRIPT_EXTENSIONS = ['js', 'mjs', 'cjs', 'jsx', 'ts', 'tsx'];

    // Runs inside the container as: sh -c FORMAT_TS_SHELL cs-format <script> <options>. NODE_PATH points node at the
    // global packages, so require('typescript') finds the compiler the image installs with npm install -g.
    const FORMAT_TS_SHELL = 'NODE_PATH="$(npm root -g 2>/dev/null)" CS_FORMAT_OPTIONS="$2" node -e "$1" 2>/dev/null';

    const FORMAT_TS_SCRIPT = [
        "let ts = null;",
        "try { ts = require('typescript'); } catch (err) { process.stdout.write('typescript is not installed in the container: ' + err.message); process.exitCode = 2; }",
        "let input = '';",
        "process.stdin.setEncoding('utf8');",
        "process.stdin.on('data', chunk => { input += chunk; });",
        "process.stdin.on('end', () => {",
        "  if (!ts) return;",
        "  try {",
        "    const opts = JSON.parse(process.env.CS_FORMAT_OPTIONS || '{}');",
        "    const fileName = opts.fileName || '/cs-format/input.ts';",
        "    const host = {",
        "      getScriptFileNames: () => [fileName],",
        "      getScriptVersion: () => '1',",
        "      getScriptSnapshot: name => name === fileName ? ts.ScriptSnapshot.fromString(input) : undefined,",
        "      getCurrentDirectory: () => '/cs-format',",
        "      getCompilationSettings: () => ({ allowJs: true, jsx: ts.JsxEmit.Preserve, target: ts.ScriptTarget.Latest }),",
        "      getDefaultLibFileName: () => 'lib.d.ts',",
        "      fileExists: name => name === fileName,",
        "      readFile: name => name === fileName ? input : undefined",
        "    };",
        "    const service = ts.createLanguageService(host, ts.createDocumentRegistry());",
        "    const settings = ts.getDefaultFormatCodeSettings(opts.newLine === 'crlf' ? String.fromCharCode(13, 10) : String.fromCharCode(10));",
        "    settings.indentSize = opts.indentSize;",
        "    settings.tabSize = opts.tabSize;",
        "    settings.convertTabsToSpaces = opts.insertSpaces;",
        "    const edits = service.getFormattingEditsForDocument(fileName, settings).slice().sort((a, b) => b.span.start - a.span.start);",
        "    let output = input;",
        "    for (const edit of edits) output = output.slice(0, edit.span.start) + edit.newText + output.slice(edit.span.start + edit.span.length);",
        "    process.stdout.write(output);",
        "  } catch (err) {",
        "    process.stdout.write('format failed: ' + (err && err.stack || err));",
        "    process.exitCode = 2;",
        "  }",
        "});"
    ].join('\n');

    function formatterKind(tab) {
        if (!isSourceTab(tab)) return '';
        const language = tab.language || languageForPath(tab.path);
        if (language === 'go') return 'gofmt';
        if (language === 'json') return 'json';
        if (['javascript', 'jsx', 'typescript', 'tsx'].includes(language)) return 'typescript';
        return '';
    }

    function formatJsonText(text, settings) {
        // Numbers keep their source text (JSON.rawJSON), so large integers survive the round trip.
        const reviver = typeof JSON.rawJSON === 'function'
            ? (key, value, context) => (typeof value === 'number' && context && typeof context.source === 'string' ? JSON.rawJSON(context.source) : value)
            : undefined;
        const parsed = JSON.parse(text, reviver);
        const newline = text.includes('\r\n') ? '\r\n' : '\n';
        let output = JSON.stringify(parsed, null, settings.insertSpaces ? settings.tabSize : '\t');
        if (newline !== '\n') output = output.split('\n').join(newline);
        return /\r?\n$/.test(text) ? output + newline : output;
    }

    function formatScriptOptions(tab, text, settings) {
        const ext = String(tab.path || '').split('.').pop().toLowerCase();
        return {
            fileName: '/cs-format/input.' + (FORMAT_SCRIPT_EXTENSIONS.includes(ext) ? ext : 'ts'),
            indentSize: settings.tabSize,
            tabSize: settings.tabSize,
            insertSpaces: settings.insertSpaces,
            newLine: text.includes('\r\n') ? 'crlf' : 'lf'
        };
    }

    // Returns { text } or { error }.
    async function runFormatter(kind, tab, text) {
        const settings = editorSettings();
        if (kind === 'json') {
            try {
                return { text: formatJsonText(text, settings) };
            } catch (err) {
                return { error: err && err.message ? err.message : String(err) };
            }
        }
        const args = kind === 'gofmt'
            ? ['gofmt']
            : ['sh', '-c', FORMAT_TS_SHELL, 'cs-format', FORMAT_TS_SCRIPT, JSON.stringify(formatScriptOptions(tab, text, settings))];
        const result = await apiClient.execInput(args, text, 60);
        const output = String(result && result.output || '');
        if (result && result.truncated) return { error: tr('codeStudio.formatTruncated', 'The formatter output was too large and was not applied.') };
        if (!result || Number(result.exit_code) !== 0) return { error: output.trim() || tr('codeStudio.formatFailed', 'Formatting failed. Details are in the Output panel.') };
        // A formatter that prints nothing for a non-empty file failed; applying that would wipe the document.
        if (text.trim() && !output.trim()) return { error: tr('codeStudio.formatFailed', 'Formatting failed. Details are in the Output panel.') };
        return { text: output };
    }

    function minimalTextChange(current, next) {
        let from = 0;
        const limit = Math.min(current.length, next.length);
        while (from < limit && current.charCodeAt(from) === next.charCodeAt(from)) from++;
        let toCurrent = current.length;
        let toNext = next.length;
        while (toCurrent > from && toNext > from && current.charCodeAt(toCurrent - 1) === next.charCodeAt(toNext - 1)) {
            toCurrent--;
            toNext--;
        }
        return { from, to: toCurrent, insert: next.slice(from, toNext) };
    }

    // Replaces the text of a tab as one transaction with its own undo step. A mounted CodeMirror view gets the
    // smallest change, the textarea fallback its new value; a tab without a live view updates its text and its saved
    // editor state. Returns false when the text is already equal.
    function applyTabText(tab, next) {
        const current = editorValue(tab);
        if (next === current) return false;
        const change = minimalTextChange(current, next);
        const cm = state.cmModule;
        const annotations = cm && cm.isolateHistory ? [cm.isolateHistory.of('full')] : [];
        if (tab.view && tab.view.state && typeof tab.view.dispatch === 'function') {
            tab.view.dispatch({ changes: change, annotations });
            return true;
        }
        if (tab.view && typeof tab.view.setValue === 'function') {
            tab.view.setValue(next);
            return true;
        }
        const snapshot = tab.editorSnapshot;
        if (snapshot && snapshot.state && snapshot.state.doc && snapshot.state.doc.toString() === current) {
            tab.editorSnapshot = Object.assign({}, snapshot, { state: snapshot.state.update({ changes: change, annotations }).state });
        } else {
            tab.editorSnapshot = null;
        }
        tab.secondaryEditorSnapshot = null;
        updateTabContent(tab, next);
        return true;
    }

    // Formats one tab. quiet (format on save) skips the notices except failures; errors always go to the Output panel.
    async function formatTab(target, tab, options) {
        const quiet = !!(options && options.quiet);
        if (!isLiveInstance(target) || !isSourceTab(tab) || !target.openTabs.includes(tab)) return false;
        const kind = formatterKind(tab);
        if (!kind) {
            if (!quiet) {
                runWithInstance(target, () => flashStatus(tr('codeStudio.formatUnavailable', 'No formatter available for {{language}}', { language: tab.language || baseName(tab.path) })));
            }
            return false;
        }
        if (!canWrite(target)) return false;
        const source = editorValue(tab);
        const revision = tab.revision || 0;
        let result;
        try {
            result = await runFormatter(kind, tab, source);
        } catch (err) {
            result = { error: err && err.message ? err.message : String(err) };
        }
        if (!isLiveInstance(target) || !target.openTabs.includes(tab)) return false;
        return runWithInstance(target, () => {
            if (result.error) {
                appendOutput(result.error, tr('codeStudio.formatOutputSource', 'Format'));
                flashStatus(tr('codeStudio.formatFailed', 'Formatting failed. Details are in the Output panel.'));
                if (!quiet) setActivePanel('output');
                return false;
            }
            if ((tab.revision || 0) !== revision || editorValue(tab) !== source) {
                if (!quiet) flashStatus(tr('codeStudio.formatStale', 'The document changed while it was being formatted. Format it again.'));
                return false;
            }
            const changed = applyTabText(tab, result.text);
            if (!quiet) {
                flashStatus(changed
                    ? tr('codeStudio.formatDone', 'Document formatted')
                    : tr('codeStudio.formatUnchanged', 'The document is already formatted'));
            }
            return changed;
        });
    }

    function formatActiveDocument() {
        const target = state;
        const tab = activeTab();
        if (!isSourceTab(tab) || !canWrite(target)) return Promise.resolve(false);
        return formatTab(target, tab, { quiet: false });
    }

    function tabNeedsSaveActions(target, tab) {
        const settings = editorSettings();
        return canWrite(target) && isSourceTab(tab) && (settings.formatOnSave || settings.trimTrailingWhitespace || settings.insertFinalNewline);
    }

    // Format on save runs first; trimming trailing whitespace and the final newline follow as one more transaction.
    async function runSaveActions(target, tab) {
        const settings = editorSettings();
        if (settings.formatOnSave && formatterKind(tab)) await formatTab(target, tab, { quiet: true });
        if (!isLiveInstance(target) || !target.openTabs.includes(tab)) return;
        if (!settings.trimTrailingWhitespace && !settings.insertFinalNewline) return;
        runWithInstance(target, () => {
            let text = editorValue(tab);
            if (settings.trimTrailingWhitespace) text = text.replace(/[ \t]+(?=\r?$)/gm, '');
            if (settings.insertFinalNewline && text && !text.endsWith('\n')) text += text.includes('\r\n') ? '\r\n' : '\n';
            applyTabText(tab, text);
        });
    }

    // Explicit saves (Ctrl+S, Save, Save All, Run) come through here; auto save and the desktop's session save call
    // saveTab directly and write the buffer as it is.
    function saveTabWithActions(target, tab) {
        if (!tabNeedsSaveActions(target, tab)) return saveTab(target, tab);
        return runSaveActions(target, tab).catch(() => {}).then(() => saveTab(target, tab));
    }
```

- [ ] **Step 3: Add `format.js` to the bundle**

In `scripts/build-ui-bundles.js`, replace:
```js
      'ui/js/desktop/apps/code-studio/settings.js',
      'ui/js/desktop/apps/code-studio/shortcuts.js',
```
With:
```js
      'ui/js/desktop/apps/code-studio/settings.js',
      'ui/js/desktop/apps/code-studio/format.js',
      'ui/js/desktop/apps/code-studio/shortcuts.js',
```

- [ ] **Step 4: Give the API client an exec with stdin**

In `core.js`, replace:
```js
        exec: (command, cwd, timeoutSeconds) => runOnWindow(null, () => api('/api/code-studio/exec', {
            method: 'POST',
            body: JSON.stringify({ command, cwd: cwd || currentDirectory(), timeout_seconds: timeoutSeconds || 300 })
        })),
```
With:
```js
        exec: (command, cwd, timeoutSeconds) => runOnWindow(null, () => api('/api/code-studio/exec', {
            method: 'POST',
            body: JSON.stringify({ command, cwd: cwd || currentDirectory(), timeout_seconds: timeoutSeconds || 300 })
        })),
        // IDE package 1A: runs args directly and pipes stdin into the process (Format Document).
        execInput: (args, stdin, timeoutSeconds) => api('/api/code-studio/exec', {
            method: 'POST',
            body: JSON.stringify({ args, stdin, timeout_seconds: timeoutSeconds || 60 })
        }),
```

- [ ] **Step 5: Route explicit saves through the save actions**

In `core.js`, replace:
```js
    function saveCurrentFile() {
        return saveTab(state, activeTab());
    }
```
With:
```js
    function saveCurrentFile() {
        // IDE package 1A: an explicit save runs the save actions of the editor settings first.
        return saveTabWithActions(state, activeTab());
    }
```

Then replace:
```js
    async function saveAllFiles() {
        const target = state;
        if (!isLiveInstance(target)) return false;
        const results = await Promise.all(target.openTabs.filter(tab => tab.modified).map(tab => saveTab(target, tab)));
```
With:
```js
    async function saveAllFiles(options) {
        const target = state;
        if (!isLiveInstance(target)) return false;
        // IDE package 1A: the desktop's session save passes skipSaveActions and writes the buffers as they are.
        const save = options && options.skipSaveActions === true ? saveTab : saveTabWithActions;
        const results = await Promise.all(target.openTabs.filter(tab => tab.modified).map(tab => save(target, tab)));
```

Then, in `registerSessionAction`, replace:
```js
                if (sessionDirty(instance) && !await runWithInstance(instance, saveAllFiles)) return false;
```
With:
```js
                // Save actions would change the text and break the revision handshake, so this save writes the buffers as they are.
                if (sessionDirty(instance) && !await runWithInstance(instance, () => saveAllFiles({ skipSaveActions: true }))) return false;
```

Then, in `runCurrentFile`, replace:
```js
        const content = editorValue(tab);
        const terminal = activeTerminalSession();
        if (tab.modified || tab.pendingSave) {
            if (!await saveTab(target, tab)) return;
        }
```
With:
```js
        let content = editorValue(tab);
        const terminal = activeTerminalSession();
        if (tab.modified || tab.pendingSave) {
            // IDE package 1A: the save runs the save actions, which may change the text that gets saved and run.
            if (!await saveTabWithActions(target, tab)) return;
            content = editorValue(tab);
        }
```

- [ ] **Step 6: Build and check**

```
npm run build:ui
npm run build:ui -- --check
node --check ui/js/desktop/bundles/code-studio.bundle.js
go test ./ui -count=1 -v -run "(?i)^TestDesktopCodeStudioRunUsesTerminalSession$"
```

**Commit:** `feat(code-studio): format documents and run save actions`. Files: `ui/js/desktop/apps/code-studio/format.js`, `ui/js/desktop/apps/code-studio/core.js`, `scripts/build-ui-bundles.js`, `ui/js/desktop/bundles/code-studio.bundle.js`.

---

### Task 7: Split view with two different files (A4)

**Files:**
- Modify: `ui/js/desktop/apps/code-studio/panels.js`, `ui/js/desktop/apps/code-studio/editor.js`, `ui/js/desktop/apps/code-studio/core.js`, `ui/js/desktop/apps/code-studio/sidebar.js`, `ui/css/code-studio.css`
- Generate: `ui/js/desktop/bundles/code-studio.bundle.js`

**Design (the smallest change that keeps every `activeTab()` caller working):**
- **State.** `state.splitPrimaryTab` and `state.splitSecondaryTab` hold the tab of each pane (primary is left or top), `state.splitFocus` holds the focused pane, and `state.splitRestore` holds the saved paths until the tabs are restored.
- **Invariant.** The focused pane's tab is always `state.activeTabIndex`.
- **`renderEditor`.** It first prunes a split whose secondary tab was closed. `renderSplitPanes` then calls `resolveSplitPaneTabs(activeTab)`, which puts the active tab into the focused pane and keeps the other pane's tab. If both panes hold the same tab, they are linked exactly as today (`view` plus `secondaryView` with a `viewLink`). Otherwise each tab gets its own primary `view` with its own history.
- **Tab click.** It is unchanged: `activateTab` → `renderEditor`, so the clicked tab lands in the focused pane.
- **Focusing a pane.** `focusin` → `focusSplitPane` switches `activeTabIndex` and re-renders the tab strip, breadcrumbs and status without rebuilding the editors.
- **`closeTab`.** Closing the secondary pane's tab ends the split, and the primary pane's tab becomes active. Close others, close all and delete end the split through the prune in `renderEditor`.
- **Ctrl+\.** It toggles the split (bound in Task 8).
- **Persistence.** `saveState` writes `split: {mode, ratio, primary, secondary, focus}`, or `null` when there is no valid split. `loadState` reads it into `splitMode`, `splitRatio`, `splitFocus` and `splitRestore`. `restoreTabs` resolves the saved paths to the reopened tabs before it activates the remembered tab.

- [ ] **Step 1: Impact check.** Run `impact` upstream for `renderEditor`, `renderSplitPanes`, `splitEditor`, `wireSplitDivider`, `closeTab`, `loadState`, `saveState`, `restoreTabs`, `refreshActiveEditorZoomLayout`, `showTabContextMenu` and `showTreeContextMenu`. `renderEditor` and `saveState` have many callers; the change is additive for every state without a split.

- [ ] **Step 2: Add the split fields to `createInstance`**

In `core.js`, replace:
```js
            splitRatio: 0.5,
            selectedDir: WORKSPACE_ROOT,
```
With:
```js
            splitRatio: 0.5,
            // IDE package 1A: the tab of each split pane, the focused pane and the paths saved for the next start.
            splitPrimaryTab: null,
            splitSecondaryTab: null,
            splitFocus: 'primary',
            splitRestore: null,
            selectedDir: WORKSPACE_ROOT,
```

- [ ] **Step 3: Keep `splitEditor`'s guard line, replace its body**

Line 17, `if (!state || activeTab()?.kind === 'diff') return;`, stays for Plan B. In `panels.js`, replace:
```js
        const next = direction === 'down' ? 'down' : 'right';
        state.splitMode = state.splitMode === next ? null : next;
        if (!state.splitMode) state.splitRatio = 0.5;
        renderEditor();
        renderWindowMenus();
```
With:
```js
        const next = direction === 'down' ? 'down' : 'right';
        // IDE package 1A: switching between right and down keeps both pane tabs; the same direction closes the split.
        if (state.splitMode === next) resetSplitState();
        else state.splitMode = next;
        renderEditor();
        renderWindowMenus();
        saveState();
```

- [ ] **Step 4: Render one tab per pane**

In `panels.js`, replace the whole function:
```js
    function renderSplitPanes(editor, tab) {
        const isHorizontal = state.splitMode === 'right';
        editor.classList.add('code-studio-split', isHorizontal ? 'split-right' : 'split-down');
        if (isHorizontal) {
            editor.style.gridTemplateColumns = splitGridTemplate(state.splitRatio);
            editor.style.gridTemplateRows = 'minmax(0, 1fr)';
        } else {
            editor.style.gridTemplateColumns = 'minmax(0, 1fr)';
            editor.style.gridTemplateRows = splitGridTemplate(state.splitRatio);
        }
        const pane1 = document.createElement('div');
        pane1.className = 'code-studio-split-pane';
        const divider = document.createElement('div');
        divider.className = 'code-studio-split-divider';
        divider.setAttribute('role', 'separator');
        divider.setAttribute('aria-orientation', isHorizontal ? 'vertical' : 'horizontal');
        const pane2 = document.createElement('div');
        pane2.className = 'code-studio-split-pane';
        editor.append(pane1, divider, pane2);
        const link = { views: [], syncing: false };
        tab.viewLink = link;
        tab.view = createEditorView(pane1, tab, link);
        tab.secondaryView = createEditorView(pane2, tab, link);
        tab.views = [tab.view, tab.secondaryView];
        link.views = tab.views;
        wireSplitDivider(divider, editor, isHorizontal);
    }
```
With:
```js
    function renderSplitPanes(editor, tab) {
        const isHorizontal = state.splitMode === 'right';
        editor.classList.add('code-studio-split', isHorizontal ? 'split-right' : 'split-down');
        if (isHorizontal) {
            editor.style.gridTemplateColumns = splitGridTemplate(state.splitRatio);
            editor.style.gridTemplateRows = 'minmax(0, 1fr)';
        } else {
            editor.style.gridTemplateColumns = 'minmax(0, 1fr)';
            editor.style.gridTemplateRows = splitGridTemplate(state.splitRatio);
        }
        const pane1 = document.createElement('div');
        pane1.className = 'code-studio-split-pane';
        pane1.dataset.splitPane = 'primary';
        const divider = document.createElement('div');
        divider.className = 'code-studio-split-divider';
        divider.setAttribute('role', 'separator');
        divider.setAttribute('aria-orientation', isHorizontal ? 'vertical' : 'horizontal');
        const pane2 = document.createElement('div');
        pane2.className = 'code-studio-split-pane';
        pane2.dataset.splitPane = 'secondary';
        editor.append(pane1, divider, pane2);
        // IDE package 1A: every pane shows its own tab and the focused pane shows the active tab. The same tab in both
        // panes keeps the linked views (one history, the second view follows the first).
        const panes = resolveSplitPaneTabs(tab);
        if (panes.primary === panes.secondary) {
            const shared = panes.primary;
            const link = { views: [], syncing: false };
            shared.viewLink = link;
            shared.view = createEditorView(pane1, shared, link);
            shared.secondaryView = createEditorView(pane2, shared, link);
            shared.views = [shared.view, shared.secondaryView];
            link.views = shared.views;
        } else {
            panes.primary.view = createEditorView(pane1, panes.primary, null);
            panes.primary.views = [panes.primary.view];
            panes.secondary.view = createEditorView(pane2, panes.secondary, null);
            panes.secondary.views = [panes.secondary.view];
        }
        updateSplitPaneFocus(editor);
        pane1.addEventListener('focusin', bind(() => focusSplitPane('primary')));
        pane2.addEventListener('focusin', bind(() => focusSplitPane('secondary')));
        wireSplitDivider(divider, editor, isHorizontal);
    }
```

- [ ] **Step 5: Measure both panes after a divider drag and persist the ratio**

In `panels.js`, replace:
```js
            divider.removeEventListener('pointercancel', onPointerUp);
            const tab = activeTab();
            (tab && tab.views || []).forEach(view => {
                if (view && typeof view.requestMeasure === 'function') view.requestMeasure();
            });
        });
```
With:
```js
            divider.removeEventListener('pointercancel', onPointerUp);
            measureVisibleEditors();
            saveState();
        });
```

- [ ] **Step 6: Append the split helpers to `panels.js`**

Append to end of file `ui/js/desktop/apps/code-studio/panels.js`:
```js

    // IDE package 1A: split view with two different files (A4). The primary pane is left or top, the secondary pane
    // right or bottom. state.splitPrimaryTab and state.splitSecondaryTab hold the tab of each pane, state.splitFocus the
    // focused one. The focused pane's tab is always the active tab, so every activeTab() caller (save, run, find, agent)
    // works on the pane the user is in.

    function resetSplitState() {
        state.splitMode = null;
        state.splitRatio = 0.5;
        state.splitPrimaryTab = null;
        state.splitSecondaryTab = null;
        state.splitFocus = 'primary';
    }

    // The active tab goes into the focused pane; the other pane keeps its tab, or shows the same tab when it has none.
    function resolveSplitPaneTabs(activeSourceTab) {
        const usable = candidate => (candidate && isSourceTab(candidate) && state.openTabs.includes(candidate) ? candidate : null);
        let primary = usable(state.splitPrimaryTab);
        let secondary = usable(state.splitSecondaryTab);
        if (state.splitFocus === 'secondary') secondary = activeSourceTab;
        else primary = activeSourceTab;
        if (!primary) primary = secondary;
        if (!secondary) secondary = primary;
        state.splitPrimaryTab = primary;
        state.splitSecondaryTab = secondary;
        return { primary, secondary };
    }

    function updateSplitPaneFocus(editor) {
        if (!editor) return;
        const separate = !!state.splitPrimaryTab && state.splitPrimaryTab !== state.splitSecondaryTab;
        editor.querySelectorAll('[data-split-pane]').forEach(pane => {
            pane.classList.toggle('cs-split-pane-focused', separate && pane.dataset.splitPane === state.splitFocus);
        });
    }

    // Focusing a pane makes its tab the active tab without rebuilding the editors.
    function focusSplitPane(pane) {
        if (!state.splitMode) return;
        const next = pane === 'secondary' ? 'secondary' : 'primary';
        const changed = state.splitFocus !== next;
        state.splitFocus = next;
        updateSplitPaneFocus(shellPart('[data-editor]'));
        const tab = next === 'secondary' ? state.splitSecondaryTab : state.splitPrimaryTab;
        const index = tab ? state.openTabs.indexOf(tab) : -1;
        if (index < 0 || index === state.activeTabIndex) {
            if (changed) saveState();
            return;
        }
        state.activeTabIndex = index;
        renderTabs();
        renderBreadcrumbs();
        renderStatus();
        if (state.sidebarView === 'outline') renderSidebar();
        saveState();
    }

    // Closing the secondary pane's tab ends the split. Returns the primary pane's tab, which then becomes active.
    function closeSplitForTab(tab) {
        if (!state.splitMode || !tab || tab !== state.splitSecondaryTab) return null;
        const primary = state.splitPrimaryTab;
        const keep = primary && primary !== tab && state.openTabs.includes(primary) ? primary : null;
        resetSplitState();
        return keep;
    }

    // Close others, close all and delete remove tabs without closeTab; a missing secondary tab ends the split here.
    function pruneClosedSplitPane() {
        if (!state.splitMode || state.splitRestore || !state.splitSecondaryTab || state.openTabs.includes(state.splitSecondaryTab)) return;
        resetSplitState();
        renderWindowMenus();
        saveState();
    }

    function splitStatePayload() {
        const ratio = Math.max(0.2, Math.min(0.8, Number(state.splitRatio) || 0.5));
        const focus = state.splitFocus === 'secondary' ? 'secondary' : 'primary';
        if (state.splitMode && state.splitRestore) {
            return { mode: state.splitMode, ratio, primary: state.splitRestore.primary, secondary: state.splitRestore.secondary, focus };
        }
        const primary = state.splitPrimaryTab;
        const secondary = state.splitSecondaryTab;
        if (!state.splitMode || !isSourceTab(primary) || !isSourceTab(secondary) || !state.openTabs.includes(primary) || !state.openTabs.includes(secondary)) return null;
        return { mode: state.splitMode, ratio, primary: primary.path, secondary: secondary.path, focus };
    }

    // loadState: remember the saved split until restoreTabs has reopened the files.
    function restoreSplitPreference(saved) {
        const split = saved && typeof saved === 'object' ? saved : null;
        const mode = split && (split.mode === 'right' || split.mode === 'down') ? split.mode : null;
        state.splitPrimaryTab = null;
        state.splitSecondaryTab = null;
        if (!mode) {
            state.splitMode = null;
            state.splitRatio = 0.5;
            state.splitFocus = 'primary';
            state.splitRestore = null;
            return;
        }
        state.splitMode = mode;
        state.splitRatio = Math.max(0.2, Math.min(0.8, Number(split.ratio) || 0.5));
        state.splitFocus = split.focus === 'secondary' ? 'secondary' : 'primary';
        state.splitRestore = { primary: normalizeCodeStudioPath(split.primary), secondary: normalizeCodeStudioPath(split.secondary) };
    }

    // restoreTabs: map the saved paths onto the reopened tabs; without both files the split is dropped.
    function restoreSplitTabs() {
        const saved = state.splitRestore;
        state.splitRestore = null;
        if (!state.splitMode || !saved) return;
        const find = path => state.openTabs.find(tab => isSourceTab(tab) && tab.path === path) || null;
        const primary = find(saved.primary);
        const secondary = find(saved.secondary);
        if (!primary || !secondary) {
            resetSplitState();
            return;
        }
        state.splitPrimaryTab = primary;
        state.splitSecondaryTab = secondary;
    }

    // Ctrl+\ toggles the split in its current direction (right when it is off).
    function toggleSplitEditor() {
        splitEditor(state.splitMode === 'down' ? 'down' : 'right');
    }

    // Open to the Side: the file goes into the pane that is not focused; the split opens to the right when it is off.
    async function openToSide(path) {
        const target = state;
        if (!isLiveInstance(target)) return;
        const current = activeTab();
        const anchor = isSourceTab(current) ? current : null;
        const side = state.splitMode && state.splitFocus === 'secondary' ? 'primary' : 'secondary';
        if (!state.splitMode) {
            state.splitMode = 'right';
            state.splitRatio = 0.5;
        }
        if (anchor) {
            if (side === 'secondary') state.splitPrimaryTab = anchor;
            else state.splitSecondaryTab = anchor;
        }
        state.splitFocus = side;
        try {
            await openFile(path);
        } catch (err) {
            if (isLiveInstance(target)) {
                runWithInstance(target, () => {
                    showOperationError(err);
                    renderEditor();
                    renderWindowMenus();
                });
            }
        }
    }

    // Every mounted editor re-measures after a zoom or a divider drag; with two files both panes need it.
    function measureVisibleEditors() {
        state.openTabs.forEach(openTab => (openTab.views || []).forEach(view => {
            if (view && typeof view.requestMeasure === 'function') view.requestMeasure();
            if (view && view.textarea) view.textarea.getBoundingClientRect();
        }));
    }
```

- [ ] **Step 7: Prune a split whose secondary tab is gone before rendering**

In `editor.js`, replace:
```js
    function renderEditor() {
        const editor = shellPart('[data-editor]');
        if (!editor) return;
        const tab = activeTab();
        state.openTabs.forEach(openTab => destroyTabView(openTab));
```
With:
```js
    function renderEditor() {
        const editor = shellPart('[data-editor]');
        if (!editor) return;
        const tab = activeTab();
        pruneClosedSplitPane(); // IDE package 1A: a closed secondary tab ends the split
        state.openTabs.forEach(openTab => destroyTabView(openTab));
```

- [ ] **Step 8: Close the split with the secondary tab (`closeTab`)**

In `core.js`, replace:
```js
        runWithInstance(target, () => {
            const wasActive = state.activeTabIndex === index;
            destroyTabView(tab, true);
            state.openTabs.splice(index, 1);
            if (wasActive) state.activeTabIndex = Math.min(index, state.openTabs.length - 1);
            else if (state.activeTabIndex > index) state.activeTabIndex -= 1;
```
With:
```js
        runWithInstance(target, () => {
            const wasActive = state.activeTabIndex === index;
            // IDE package 1A: closing the secondary pane's tab closes the split; the primary pane's tab stays active.
            const splitKeep = closeSplitForTab(tab);
            destroyTabView(tab, true);
            state.openTabs.splice(index, 1);
            if (splitKeep) state.activeTabIndex = state.openTabs.indexOf(splitKeep);
            else if (wasActive) state.activeTabIndex = Math.min(index, state.openTabs.length - 1);
            else if (state.activeTabIndex > index) state.activeTabIndex -= 1;
```

- [ ] **Step 9: Persist and restore the split**

In `core.js` (`loadState`), replace:
```js
            state.recentFiles = normalizePersistedPaths(saved.recentFiles);
```
With:
```js
            state.recentFiles = normalizePersistedPaths(saved.recentFiles);
            restoreSplitPreference(saved.split); // IDE package 1A
```

In `core.js` (`saveState`), replace:
```js
            terminalHeight: clampDimension(state.terminalHeight, MIN_TERMINAL_HEIGHT, MAX_TERMINAL_HEIGHT, 220),
            recentFiles: recentPaths
        };
        localStorage.setItem(STATE_KEY, JSON.stringify(payload));
```
With:
```js
            terminalHeight: clampDimension(state.terminalHeight, MIN_TERMINAL_HEIGHT, MAX_TERMINAL_HEIGHT, 220),
            recentFiles: recentPaths,
            split: splitStatePayload() // IDE package 1A
        };
        localStorage.setItem(STATE_KEY, JSON.stringify(payload));
```

In `core.js` (`restoreTabs`), replace:
```js
        runWithInstance(target, () => {
            if (state.openTabs.length) {
                activateTab(Math.min(Math.max(desiredActive, 0), state.openTabs.length - 1));
```
With:
```js
        runWithInstance(target, () => {
            restoreSplitTabs(); // IDE package 1A: before the remembered tab lands in the focused pane
            if (state.openTabs.length) {
                activateTab(Math.min(Math.max(desiredActive, 0), state.openTabs.length - 1));
```

- [ ] **Step 10: Measure every mounted editor on zoom**

In `core.js`, replace:
```js
    function refreshActiveEditorZoomLayout() {
        const tab = activeTab();
        if (!tab) return;
        (tab.views || []).forEach(view => {
            if (view.requestMeasure) view.requestMeasure();
            if (view.textarea) view.textarea.getBoundingClientRect();
        });
    }
```
With:
```js
    function refreshActiveEditorZoomLayout() {
        // IDE package 1A: both split panes can show different tabs, so every mounted editor is measured.
        measureVisibleEditors();
    }
```

- [ ] **Step 11: Add "Open to the Side" to the tab menu and the explorer menu**

In `core.js` (`showTabContextMenu`), replace:
```js
            { separator: true },
            { id: 'split-right', label: tr('codeStudio.splitRight', 'Split Right'), icon: 'columns', checked: state.splitMode === 'right', action: bind(() => { activateTab(index); splitEditor('right'); }) },
```
With:
```js
            { separator: true },
            { id: 'open-to-side', label: tr('codeStudio.openToSide', 'Open to the Side'), icon: 'columns', hidden: !isSourceTab(tab), action: bind(() => openToSide(tab.path)) }, // IDE package 1A
            { id: 'split-right', label: tr('codeStudio.splitRight', 'Split Right'), icon: 'columns', checked: state.splitMode === 'right', action: bind(() => { activateTab(index); splitEditor('right'); }) },
```

In `sidebar.js` (`showTreeContextMenu`), replace:
```js
        if (file && !isDir) items.push({ id: 'open', label: tr('desktop.file_dialog_open', 'Open'), icon: 'file', action: bind(() => openFile(file.path)) });
```
With:
```js
        if (file && !isDir) items.push({ id: 'open', label: tr('desktop.file_dialog_open', 'Open'), icon: 'file', action: bind(() => openFile(file.path)) });
        if (file && !isDir) items.push({ id: 'open-to-side', label: tr('codeStudio.openToSide', 'Open to the Side'), icon: 'columns', action: bind(() => openToSide(file.path)) }); // IDE package 1A
```

- [ ] **Step 12: Mark the focused pane**

Append to the end of `ui/css/code-studio.css`:
```css

/* Split view with two different files (A4): the focused pane carries an accent line on top. */
.code-studio-split-pane.cs-split-pane-focused::after {
    content: '';
    position: absolute;
    top: 0;
    right: 0;
    left: 0;
    height: 2px;
    background: var(--cs-accent);
    pointer-events: none;
    z-index: 4;
}
```

- [ ] **Step 13: Build and check**

```
npm run build:ui
npm run build:ui -- --check
node --check ui/js/desktop/bundles/code-studio.bundle.js
go test ./ui -count=1 -v -run "(?i)^Test(DesktopCodeStudioRunUsesTerminalSession|CodeStudioUsesPerWindowStateAndClosesTerminal)$"
```
Expected: 2 `--- PASS` lines.

**Commit:** `feat(code-studio): show two different files in the split view`. Files: `ui/js/desktop/apps/code-studio/panels.js`, `ui/js/desktop/apps/code-studio/editor.js`, `ui/js/desktop/apps/code-studio/core.js`, `ui/js/desktop/apps/code-studio/sidebar.js`, `ui/css/code-studio.css`, `ui/js/desktop/bundles/code-studio.bundle.js`.

---

### Task 8: Go to Line and Go to Symbol, editor commands, keys, menus and shortcut help (A3, A7)

**Files:**
- Modify: `ui/js/desktop/apps/code-studio/command-palette.js`, `ui/js/desktop/apps/code-studio/outline.js`, `ui/js/desktop/apps/code-studio/editor.js`, `ui/js/desktop/apps/code-studio/shortcuts.js`, `ui/js/desktop/apps/code-studio/core.js`, `ui/css/code-studio.css`
- Generate: `ui/js/desktop/bundles/code-studio.bundle.js`

Every palette and menu icon used here (`search`, `list`, `edit`, `settings`, `columns`, `code`, `copy`, `trash`) exists in `ui/img/desktop-mini/manifest.json`. That keeps `TestDesktopCodeStudioMenuIconsBrowser` green.

- [ ] **Step 1: Impact check.** Run `impact` upstream for `renderPalette`, `executeItem`, `filterItems`, `renderResults`, `renderItem`, `getCommands`, `jumpToLine`, `wireShortcuts`, `showShortcutOverlay`, `studioCommandTable` and `renderWindowMenus`.

- [ ] **Step 2: `jumpToLine` uses the focused view (split-aware) and honours the column**

In `ui/js/desktop/apps/code-studio/outline.js`, replace:
```js
    function jumpToLine(line, column) {
        const tab = activeTab();
        if (!tab) return;
        const lineNumber = Math.max(1, Number(line) || 1);
        const columnNumber = Math.max(1, Number(column) || 1);
        if (tab.view && tab.view.state && tab.view.state.doc && state.cmModule && state.cmModule.EditorView) {
            const doc = tab.view.state.doc;
            const docLine = doc.line(Math.min(lineNumber, doc.lines));
            const pos = Math.min(docLine.to, docLine.from + columnNumber - 1);
            tab.view.dispatch({
                selection: { anchor: pos },
                effects: state.cmModule.EditorView.scrollIntoView(pos, { y: 'center' })
            });
            tab.view.focus();
        } else if (tab.view && tab.view.textarea) {
            const rows = tab.view.textarea.value.split('\n');
            const offset = rows.slice(0, lineNumber - 1).join('\n').length + (lineNumber > 1 ? 1 : 0);
            tab.view.textarea.focus();
            tab.view.textarea.setSelectionRange(offset, offset);
        }
        renderStatus();
    }
```
With:
```js
    function jumpToLine(line, column) {
        const tab = activeTab();
        if (!tab) return;
        const lineNumber = Math.max(1, Number(line) || 1);
        const columnNumber = Math.max(1, Number(column) || 1);
        // IDE package 1A: jump in the editor pane the user worked in last (a linked split keeps two views).
        const view = focusedEditorView(tab);
        if (view && view.state && view.state.doc && state.cmModule && state.cmModule.EditorView) {
            const doc = view.state.doc;
            const docLine = doc.line(Math.min(lineNumber, doc.lines));
            const pos = Math.min(docLine.to, docLine.from + columnNumber - 1);
            view.dispatch({
                selection: { anchor: pos },
                effects: state.cmModule.EditorView.scrollIntoView(pos, { y: 'center' })
            });
            view.focus();
        } else if (view && view.textarea) {
            const rows = view.textarea.value.split('\n');
            const row = Math.min(lineNumber, rows.length);
            const base = rows.slice(0, row - 1).join('\n').length + (row > 1 ? 1 : 0);
            const offset = base + Math.min(columnNumber - 1, (rows[row - 1] || '').length);
            view.textarea.focus();
            view.textarea.setSelectionRange(offset, offset);
        }
        renderStatus();
    }
```

Append to end of file `outline.js`:
```js

    // IDE package 1A: Go to Line (Ctrl+G) and Go to Symbol open the command palette in their prefix mode.
    function openGoToLine() {
        if (window.CodeStudioCommandPalette && typeof window.CodeStudioCommandPalette.open === 'function') {
            window.CodeStudioCommandPalette.open(state.windowId, ':');
        }
    }

    function openGoToSymbol() {
        if (window.CodeStudioCommandPalette && typeof window.CodeStudioCommandPalette.open === 'function') {
            window.CodeStudioCommandPalette.open(state.windowId, '@');
        }
    }
```

- [ ] **Step 3: Run CodeMirror's folding, multi-cursor and line commands from the palette**

Append to end of file `ui/js/desktop/apps/code-studio/editor.js`:
```js

    // IDE package 1A: CodeMirror commands the palette and the shortcut sheet offer (folding, multi-cursor, lines).
    const EDITOR_COMMANDS = ['foldCode', 'unfoldCode', 'selectNextOccurrence', 'moveLineUp', 'moveLineDown', 'copyLineUp', 'copyLineDown', 'toggleComment', 'deleteLine'];

    function runEditorCommand(name) {
        const tab = activeTab();
        const view = focusedEditorView(tab);
        const cm = state.cmModule;
        if (!isSourceTab(tab) || !view || !view.state || !cm || !EDITOR_COMMANDS.includes(name) || typeof cm[name] !== 'function') return false;
        view.focus();
        return cm[name](view);
    }
```

- [ ] **Step 4: Palette commands (one contiguous block after Replace in File)**

In `ui/js/desktop/apps/code-studio/command-palette.js`, replace:
```js
            { id: 'replace', label: tr('codeStudio.replaceInFile', 'Replace in File'), shortcut: 'Ctrl+H', icon: 'edit', run: 'openFindWidget', args: [true], disabled: !hasTab },
```
With:
```js
            { id: 'replace', label: tr('codeStudio.replaceInFile', 'Replace in File'), shortcut: 'Ctrl+H', icon: 'edit', run: 'openFindWidget', args: [true], disabled: !hasTab },
            // IDE package 1A
            { id: 'go-to-line', label: tr('codeStudio.goToLine', 'Go to Line...'), shortcut: 'Ctrl+G', icon: 'search', run: 'openGoToLine', disabled: !sourceTab },
            { id: 'go-to-symbol', label: tr('codeStudio.goToSymbol', 'Go to Symbol...'), shortcut: '@', icon: 'list', run: 'openGoToSymbol', disabled: !sourceTab },
            { id: 'format-document', label: tr('codeStudio.formatDocument', 'Format Document'), shortcut: 'Shift+Alt+F', icon: 'edit', run: 'formatActiveDocument', disabled: !sourceTab || state.readOnly },
            { id: 'open-settings', label: tr('codeStudio.openSettings', 'Preferences: Open Settings'), icon: 'settings', run: 'openEditorSettings' },
            { id: 'toggle-split', label: tr('codeStudio.toggleSplit', 'Toggle Split Editor'), shortcut: 'Ctrl+\\', icon: 'columns', run: 'toggleSplitEditor', disabled: !sourceTab },
            { id: 'fold', label: tr('codeStudio.foldRegion', 'Fold'), shortcut: 'Ctrl+Shift+[', icon: 'list', run: 'runEditorCommand', args: ['foldCode'], disabled: !sourceTab },
            { id: 'unfold', label: tr('codeStudio.unfoldRegion', 'Unfold'), shortcut: 'Ctrl+Shift+]', icon: 'list', run: 'runEditorCommand', args: ['unfoldCode'], disabled: !sourceTab },
            { id: 'select-next-occurrence', label: tr('codeStudio.selectNextOccurrence', 'Add Selection to Next Match'), shortcut: 'Ctrl+D', icon: 'search', run: 'runEditorCommand', args: ['selectNextOccurrence'], disabled: !sourceTab },
            { id: 'move-line-up', label: tr('codeStudio.moveLineUp', 'Move Line Up'), shortcut: 'Alt+Up', icon: 'code', run: 'runEditorCommand', args: ['moveLineUp'], disabled: !sourceTab || state.readOnly },
            { id: 'move-line-down', label: tr('codeStudio.moveLineDown', 'Move Line Down'), shortcut: 'Alt+Down', icon: 'code', run: 'runEditorCommand', args: ['moveLineDown'], disabled: !sourceTab || state.readOnly },
            { id: 'copy-line-up', label: tr('codeStudio.copyLineUp', 'Copy Line Up'), shortcut: 'Shift+Alt+Up', icon: 'copy', run: 'runEditorCommand', args: ['copyLineUp'], disabled: !sourceTab || state.readOnly },
            { id: 'copy-line-down', label: tr('codeStudio.copyLineDown', 'Copy Line Down'), shortcut: 'Shift+Alt+Down', icon: 'copy', run: 'runEditorCommand', args: ['copyLineDown'], disabled: !sourceTab || state.readOnly },
            { id: 'toggle-comment', label: tr('codeStudio.toggleComment', 'Toggle Line Comment'), shortcut: 'Ctrl+/', icon: 'code', run: 'runEditorCommand', args: ['toggleComment'], disabled: !sourceTab || state.readOnly },
            { id: 'delete-line', label: tr('codeStudio.deleteLine', 'Delete Line'), shortcut: 'Ctrl+Shift+K', icon: 'trash', run: 'runEditorCommand', args: ['deleteLine'], disabled: !sourceTab || state.readOnly },
```

- [ ] **Step 5: Palette helpers for the `:` and `@` modes**

In `command-palette.js`, replace:
```js
    function getAllItems() {
```
With:
```js
    // IDE package 1A: ':' and '@' modes. The active tab is a file tab when it has no kind (Git diffs have one).
    function activePaletteTab() {
        const state = getState();
        const tab = state && state.openTabs ? state.openTabs[state.activeTabIndex] : null;
        return tab && !tab.kind ? tab : null;
    }

    function activeLineCount(tab) {
        const view = tab.focusedView || tab.view;
        if (view && view.state && view.state.doc) return view.state.doc.lines;
        return String(tab.content || '').split('\n').length;
    }

    function goToLineItems(rest) {
        const tab = activePaletteTab();
        if (!tab) return [{ type: 'hint', id: 'hint', icon: 'search', label: tr('codeStudio.noFileOpen', 'No file open') }];
        const count = activeLineCount(tab);
        const match = /^\s*(\d+)\s*(?:[:,]\s*(\d+)\s*)?$/.exec(rest);
        if (!match) {
            return [{ type: 'hint', id: 'hint', icon: 'search', label: tr('codeStudio.goToLineHint', 'Type a line number between 1 and {{count}}. Add :column to jump to a column.', { count }) }];
        }
        const line = Math.min(Math.max(1, Number(match[1])), count);
        const column = match[2] ? Math.max(1, Number(match[2])) : 0;
        return [{
            type: 'goto-line',
            id: 'goto-line',
            icon: 'search',
            line,
            column: column || 1,
            label: column
                ? tr('codeStudio.goToLineColumnItem', 'Go to line {{line}}, column {{column}}', { line, column })
                : tr('codeStudio.goToLineItem', 'Go to line {{line}}', { line })
        }];
    }

    function symbolItems(rest) {
        const app = getApp();
        if (!activePaletteTab() || !owner) return [{ type: 'hint', id: 'hint', icon: 'list', label: tr('codeStudio.noFileOpen', 'No file open') }];
        const symbols = app && typeof app.documentSymbols === 'function' ? app.documentSymbols(owner.windowId) : [];
        const query = String(rest || '').trim();
        const items = symbols.map((symbol, index) => ({
            type: 'symbol',
            id: 'symbol:' + index,
            icon: 'list',
            label: symbol.name,
            detail: symbol.kind + ' · ' + tr('codeStudio.cursorPosition', 'Ln {{line}}, Col {{column}}', { line: symbol.line, column: symbol.column }),
            line: symbol.line,
            column: symbol.column,
            score: query ? fuzzyMatch(query, symbol.name) : { match: true, score: 1 }
        })).filter(item => item.score.match);
        if (query) items.sort((a, b) => b.score.score - a.score.score);
        return items.length ? items : [{ type: 'hint', id: 'hint', icon: 'list', label: tr('codeStudio.noOutline', 'No symbols found in this file.') }];
    }

    function getAllItems() {
```

- [ ] **Step 6: Execute line and symbol targets; Ctrl+Enter opens files to the side**

In `command-palette.js`, replace:
```js
    function executeItem(item, origin) {
        const app = getApp();
        if (!app || !item || (!origin || app.instances.get(origin.windowId) !== origin)) return;
        if (item.type === 'file' || item.type === 'recent') {
            if (typeof app.command === 'function') app.command('openFile', [item.path], origin.windowId);
```
With:
```js
    function executeItem(item, origin, toSide) {
        const app = getApp();
        if (!app || !item || (!origin || app.instances.get(origin.windowId) !== origin)) return;
        // IDE package 1A: line and symbol targets jump in the active editor; hints do nothing.
        if (item.type === 'hint') return;
        if (item.type === 'goto-line' || item.type === 'symbol') {
            if (typeof app.command === 'function') app.command('jumpToLine', [item.line, item.column], origin.windowId);
            return;
        }
        if (item.type === 'file' || item.type === 'recent') {
            if (toSide && typeof app.command === 'function') {
                app.command('openToSide', [item.path], origin.windowId);
                return;
            }
            if (typeof app.command === 'function') app.command('openFile', [item.path], origin.windowId);
```

- [ ] **Step 7: Open the palette with a prefilled query; Enter skips hints; Ctrl+Enter goes to the side**

In `command-palette.js`, replace:
```js
    function renderPalette(windowId) {
        if (backdrop) closePalette();
```
With:
```js
    function renderPalette(windowId, initialQuery) {
        if (backdrop) closePalette();
```

Then replace:
```js
        selectedIndex = 0;
        lastQuery = '';
        filteredItems = getAllItems();
        renderResults(resultsEl, '');
```
With:
```js
        selectedIndex = 0;
        // IDE package 1A: Ctrl+G opens the palette with ':' and Go to Symbol with '@'.
        lastQuery = typeof initialQuery === 'string' ? initialQuery : '';
        input.value = lastQuery;
        filteredItems = lastQuery ? filterItems(lastQuery) : getAllItems();
        renderResults(resultsEl, lastQuery);
```

Then replace:
```js
            } else if (event.key === 'Enter') {
                event.preventDefault();
                if (filteredItems[selectedIndex]) {
                    const item = filteredItems[selectedIndex];
                    const origin = owner;
                    closePalette();
                    executeItem(item, origin);
                }
```
With:
```js
            } else if (event.key === 'Enter') {
                event.preventDefault();
                const item = filteredItems[selectedIndex];
                if (item && item.type !== 'hint') {
                    const origin = owner;
                    closePalette();
                    // IDE package 1A: Ctrl+Enter opens a file in the other split pane.
                    executeItem(item, origin, event.ctrlKey || event.metaKey);
                }
```

Then replace:
```js
        requestAnimationFrame(() => { if (input.isConnected) input.focus(); });
```
With:
```js
        requestAnimationFrame(() => {
            if (!input.isConnected) return;
            input.focus();
            input.setSelectionRange(input.value.length, input.value.length);
        });
```

- [ ] **Step 8: Route the prefixes in `filterItems`**

In `command-palette.js`, replace:
```js
    function filterItems(query) {
        const items = getAllItems();
```
With:
```js
    function filterItems(query) {
        // IDE package 1A: ':' goes to a line of the active file, '@' to one of its symbols.
        if (query.startsWith(':')) return goToLineItems(query.slice(1));
        if (query.startsWith('@')) return symbolItems(query.slice(1));
        const items = getAllItems();
```

- [ ] **Step 9: Render the target section; clicks on hints do nothing**

In `command-palette.js`, replace:
```js
        const commands = filteredItems.filter(i => i.type === 'command');
        const files = filteredItems.filter(i => i.type === 'file' || i.type === 'recent');

        let html = '';
```
With:
```js
        const commands = filteredItems.filter(i => i.type === 'command');
        const files = filteredItems.filter(i => i.type === 'file' || i.type === 'recent');
        // IDE package 1A: results of the ':' and '@' modes
        const targets = filteredItems.filter(i => i.type === 'goto-line' || i.type === 'symbol' || i.type === 'hint');

        let html = '';
        if (targets.length) {
            const symbolMode = String(query || '').startsWith('@');
            html += `<div class="cs-cp-section-label">${esc(symbolMode ? tr('codeStudio.symbols', 'Symbols') : tr('codeStudio.goToLine', 'Go to Line...'))}</div>`;
            targets.forEach(item => {
                html += renderItem(item, filteredItems.indexOf(item), item.type === 'symbol' ? String(query || '').slice(1).trim() : '');
            });
        }
```

Then replace:
```js
            el.addEventListener('click', () => {
                const idx = Number(el.dataset.index);
                if (filteredItems[idx]) {
```
With:
```js
            el.addEventListener('click', () => {
                const idx = Number(el.dataset.index);
                if (filteredItems[idx] && filteredItems[idx].type !== 'hint') {
```

Then replace:
```js
        const pathHtml = item.path ? `<span class="cs-cp-item-path">${esc(item.path)}</span>` : '';
        return `<button type="button" class="cs-cp-item${isSelected ? ' selected' : ''}" data-index="${index}" role="option" aria-selected="${isSelected ? 'true' : 'false'}">
```
With:
```js
        const detail = item.path || item.detail;
        const pathHtml = detail ? `<span class="cs-cp-item-path">${esc(detail)}</span>` : '';
        return `<button type="button" class="cs-cp-item${isSelected ? ' selected' : ''}${item.type === 'hint' ? ' cs-cp-hint' : ''}" data-index="${index}" role="option" aria-selected="${isSelected ? 'true' : 'false'}">
```

- [ ] **Step 10: Keys Ctrl+G, Ctrl+\ and Shift+Alt+F**

In `ui/js/desktop/apps/code-studio/shortcuts.js`, replace:
```js
            } else if (event.key === 'F3' && !inTerminal) {
                event.preventDefault();
                if (state.findOpen) refreshFindMatches(event.shiftKey ? 'previous' : 'next');
            } else if (mod && event.shiftKey && key === 'a') {
```
With:
```js
            } else if (event.key === 'F3' && !inTerminal) {
                event.preventDefault();
                if (state.findOpen) refreshFindMatches(event.shiftKey ? 'previous' : 'next');
            } else if (mod && !event.shiftKey && !event.altKey && key === 'g' && !inTerminal) {
                // IDE package 1A: Ctrl+G Go to Line, Ctrl+Backslash toggles the split, Shift+Alt+F Format Document
                event.preventDefault();
                openGoToLine();
            } else if (mod && !event.shiftKey && !event.altKey && (key === '\\' || event.code === 'Backslash') && !inTerminal) {
                event.preventDefault();
                toggleSplitEditor();
            } else if (!mod && event.shiftKey && event.altKey && (key === 'f' || event.code === 'KeyF') && !inTerminal) {
                event.preventDefault();
                formatActiveDocument();
            } else if (mod && event.shiftKey && key === 'a') {
```

- [ ] **Step 11: Shortcut help overlay section**

In `shortcuts.js` (`showShortcutOverlay`), replace:
```js
                { label: tr('codeStudio.zoomReset', 'Reset Zoom'), keys: 'Ctrl+0' }
            ]},
            { title: tr('codeStudio.shortcutsView', 'View'), items: [
```
With:
```js
                { label: tr('codeStudio.zoomReset', 'Reset Zoom'), keys: 'Ctrl+0' }
            ]},
            // IDE package 1A
            { title: tr('codeStudio.shortcutsEditing', 'Editing'), items: [
                { label: tr('codeStudio.goToLine', 'Go to Line...'), keys: 'Ctrl+G' },
                { label: tr('codeStudio.goToSymbol', 'Go to Symbol...'), keys: 'Ctrl+P @' },
                { label: tr('codeStudio.formatDocument', 'Format Document'), keys: 'Shift+Alt+F' },
                { label: tr('codeStudio.selectNextOccurrence', 'Add Selection to Next Match'), keys: 'Ctrl+D' },
                { label: tr('codeStudio.addCursor', 'Add Cursor'), keys: 'Ctrl+Click' },
                { label: tr('codeStudio.moveLineUp', 'Move Line Up'), keys: 'Alt+↑' },
                { label: tr('codeStudio.moveLineDown', 'Move Line Down'), keys: 'Alt+↓' },
                { label: tr('codeStudio.copyLineUp', 'Copy Line Up'), keys: 'Shift+Alt+↑' },
                { label: tr('codeStudio.copyLineDown', 'Copy Line Down'), keys: 'Shift+Alt+↓' },
                { label: tr('codeStudio.toggleComment', 'Toggle Line Comment'), keys: 'Ctrl+/' },
                { label: tr('codeStudio.deleteLine', 'Delete Line'), keys: 'Ctrl+Shift+K' },
                { label: tr('codeStudio.foldRegion', 'Fold'), keys: 'Ctrl+Shift+[' },
                { label: tr('codeStudio.unfoldRegion', 'Unfold'), keys: 'Ctrl+Shift+]' },
                { label: tr('codeStudio.toggleSplit', 'Toggle Split Editor'), keys: 'Ctrl+\\' },
                { label: tr('codeStudio.openToSide', 'Open to the Side'), keys: 'Ctrl+P, Ctrl+Enter' }
            ]},
            { title: tr('codeStudio.shortcutsView', 'View'), items: [
```

- [ ] **Step 12: Command table block and the `documentSymbols` API**

In `shortcuts.js`, replace:
```js
    function studioCommandTable() {
        return {
            toggleZenMode,
```
With:
```js
    function studioCommandTable() {
        return {
            // IDE package 1A
            openToSide,
            toggleSplitEditor,
            openGoToLine,
            openGoToSymbol,
            jumpToLine,
            formatActiveDocument,
            openEditorSettings,
            runEditorCommand,
            toggleZenMode,
```

Then replace:
```js
    window.CodeStudioApp = {
        render,
        dispose,
        get state() { return currentInstance(); },
        instances,
        trapDialogFocus,
        api: apiClient,
        command: exposedCommand,
        knownFiles: exposedKnownFiles,
```
With:
```js
    // IDE package 1A: the outline symbols of the active file tab, for the palette's '@' mode.
    function exposedDocumentSymbols(windowId) {
        return runOnWindow(windowId, () => {
            const tab = activeTab();
            return isSourceTab(tab) ? scanDocumentSymbols(tab) : [];
        }) || [];
    }

    window.CodeStudioApp = {
        render,
        dispose,
        get state() { return currentInstance(); },
        instances,
        trapDialogFocus,
        api: apiClient,
        command: exposedCommand,
        knownFiles: exposedKnownFiles,
        documentSymbols: exposedDocumentSymbols,
```

- [ ] **Step 13: Edit menu block in the window menus (`core.js`)**

In `core.js` (`renderWindowMenus`), replace:
```js
                    { id: 'command-palette', label: tr('codeStudio.commandPalette', 'Command Palette'), icon: 'search', shortcut: 'Ctrl+Shift+P', action: bind(openCommandPalette) },
```
With:
```js
                    { id: 'command-palette', label: tr('codeStudio.commandPalette', 'Command Palette'), icon: 'search', shortcut: 'Ctrl+Shift+P', action: bind(openCommandPalette) },
                    // IDE package 1A
                    { id: 'go-to-line', label: tr('codeStudio.goToLine', 'Go to Line...'), icon: 'search', shortcut: 'Ctrl+G', disabled: !isSourceTab(activeTab()), action: bind(openGoToLine) },
                    { id: 'go-to-symbol', label: tr('codeStudio.goToSymbol', 'Go to Symbol...'), icon: 'list', disabled: !isSourceTab(activeTab()), action: bind(openGoToSymbol) },
                    { id: 'format-document', label: tr('codeStudio.formatDocument', 'Format Document'), icon: 'edit', shortcut: 'Shift+Alt+F', disabled: !isSourceTab(activeTab()) || !canWrite(), action: bind(formatActiveDocument) },
                    { id: 'open-settings', label: tr('codeStudio.openSettings', 'Preferences: Open Settings'), icon: 'settings', action: bind(openEditorSettings) },
```

- [ ] **Step 14: Palette hint style**

Append to the end of `ui/css/code-studio.css`:
```css

/* Command palette (A3): the go-to-line and symbol hints are not commands. */
.cs-cp-item.cs-cp-hint,
.cs-cp-item.cs-cp-hint.selected {
    color: var(--cs-muted);
    cursor: default;
}
```

- [ ] **Step 15: Build and check**

```
npm run build:ui
npm run build:ui -- --check
node --check ui/js/desktop/bundles/code-studio.bundle.js
```

**Commit:** `feat(code-studio): go to line and symbol, editor commands and shortcut help`. Files: `ui/js/desktop/apps/code-studio/command-palette.js`, `ui/js/desktop/apps/code-studio/outline.js`, `ui/js/desktop/apps/code-studio/editor.js`, `ui/js/desktop/apps/code-studio/shortcuts.js`, `ui/js/desktop/apps/code-studio/core.js`, `ui/css/code-studio.css`, `ui/js/desktop/bundles/code-studio.bundle.js`.

---

### Task 9: Browser test for the editor area and final verification

**Files:**
- Create: `ui/desktop_code_studio_editor_browser_test.go`

The fixture from `ui/desktop_code_studio_browser_test.go` (`newCodeStudioBrowser`, `csRoot`, `csState`, `csKey`, `csWait`) is reused unchanged. JSON formatting runs in the browser, so the exec stub needs no stdin support. The fixture loads German strings, so the test checks data attributes and values, not English text. The browser gate is `AURAGO_RUN_BROWSER_SMOKE=1`.

- [ ] **Step 1: Create `ui/desktop_code_studio_editor_browser_test.go`**

```go
package ui

import (
	"os"
	"path/filepath"
	"testing"
)

// IDE package 1A: folding, Go to Line and Go to Symbol, two files in the split view, the editor settings and
// Format Document, driven through the real Code Studio shell and CodeMirror bundle.
func TestDesktopCodeStudioEditorBrowser(t *testing.T) {
	page, _ := newCodeStudioBrowser(t)
	check := func(name, js string) {
		t.Helper()
		result, err := page.Eval(js)
		if err != nil {
			t.Errorf("%s: eval failed: %v; browser errors: %s", name, err, page.MustEval(`()=>JSON.stringify(fixtureErrors)`).Str())
			return
		}
		if !result.Value.Bool() {
			t.Errorf("%s; browser errors: %s", name, page.MustEval(`()=>JSON.stringify(fixtureErrors)`).Str())
		}
	}
	snapshot := func(name string) {
		t.Helper()
		dir := os.Getenv("AURAGO_BROWSER_ARTIFACT_DIR")
		if dir == "" {
			return
		}
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, name+".png"), page.MustScreenshot(), 0644); err != nil {
			t.Fatal(err)
		}
	}

	if _, err := page.Eval(`async()=>{await csWait(()=>csRoot()&&csRoot().querySelectorAll('.cs-tree-item').length>0,20000);}`); err != nil {
		t.Fatalf("Code Studio did not open: %v", err)
	}
	page.MustEval(`async()=>{[...csRoot().querySelectorAll('.cs-tree-item')].find(el=>el.dataset.filePath==='/workspace/main.go').click();await csWait(()=>csRoot().querySelector('.cm-editor'),10000);}`)

	// A1: the fold gutter shows a marker for func main.
	check("the editor shows a fold gutter with a fold marker", `async()=>{await csWait(()=>csRoot().querySelector('.cm-editor .cm-foldGutter .cm-gutterElement span[title]'),5000);return true;}`)

	// A3: Ctrl+G opens the palette with ':' and line:column jumps there.
	page.MustEval(`()=>{const content=csRoot().querySelector('.cm-content');content.focus();csKey(content,{key:'g',ctrlKey:true});}`)
	check("Ctrl+G opens the palette in go-to-line mode", `async()=>{const input=await csWait(()=>document.querySelector('.cs-command-palette-backdrop input'),3000);return input.value===':';}`)
	page.MustEval(`()=>{const input=document.querySelector('.cs-command-palette-backdrop input');input.value=':5:2';input.dispatchEvent(new Event('input',{bubbles:true}));input.dispatchEvent(new KeyboardEvent('keydown',{key:'Enter',bubbles:true,cancelable:true}));}`)
	check("line:column jumps in the active editor", `async()=>{await csWait(()=>!document.querySelector('.cs-command-palette-backdrop'),3000);const view=csState().openTabs[csState().activeTabIndex].view;const head=view.state.selection.main.head;const line=view.state.doc.lineAt(head);return line.number===5&&head-line.from===1;}`)

	// A3: '@' lists the outline symbols of the active file.
	page.MustEval(`()=>{window.CodeStudioApp.command('openGoToSymbol',[],csState().windowId);}`)
	check("@ lists the symbols of the active file", `async()=>{const input=await csWait(()=>document.querySelector('.cs-command-palette-backdrop input'),3000);input.value='@mai';input.dispatchEvent(new Event('input',{bubbles:true}));return [...document.querySelectorAll('.cs-command-palette-backdrop .cs-cp-item-label')].some(el=>el.textContent==='main');}`)
	page.MustEval(`()=>{const input=document.querySelector('.cs-command-palette-backdrop input');if(input)input.dispatchEvent(new KeyboardEvent('keydown',{key:'Escape',bubbles:true,cancelable:true}));}`)

	// A1/A7: the fold command folds func main at the cursor (line 5) and unfold opens it again.
	page.MustEval(`()=>{window.CodeStudioApp.command('runEditorCommand',['foldCode'],csState().windowId);}`)
	check("Fold collapses func main", `async()=>{await csWait(()=>csRoot().querySelector('.cm-foldPlaceholder'),3000);return true;}`)
	page.MustEval(`()=>{window.CodeStudioApp.command('runEditorCommand',['unfoldCode'],csState().windowId);}`)
	check("Unfold opens it again", `()=>!csRoot().querySelector('.cm-foldPlaceholder')`)

	// A4: Open to the Side puts README.md next to main.go; focusing a pane makes its file the active tab.
	page.MustEval(`async()=>{await window.CodeStudioApp.command('openToSide',['/workspace/README.md'],csState().windowId);await csWait(()=>csRoot().querySelectorAll('[data-editor] .cm-editor').length===2,10000);}`)
	snapshot("cs-editor-01-split")
	check("Open to the Side shows two different files", `()=>{const primary=csRoot().querySelector('[data-split-pane="primary"] .cm-content');const secondary=csRoot().querySelector('[data-split-pane="secondary"] .cm-content');return !!primary&&!!secondary&&primary.textContent.includes('package main')&&secondary.textContent.includes('# Demo');}`)
	check("the side file is the active tab", `()=>csState().openTabs[csState().activeTabIndex].path==='/workspace/README.md'`)
	page.MustEval(`()=>{const content=csRoot().querySelector('[data-split-pane="primary"] .cm-content');content.focus();content.dispatchEvent(new FocusEvent('focusin',{bubbles:true}));}`)
	check("focusing the other pane makes its file the active tab", `()=>csState().openTabs[csState().activeTabIndex].path==='/workspace/main.go'&&csRoot().querySelectorAll('[data-editor] .cm-editor').length===2`)
	page.MustEval(`()=>{const content=csRoot().querySelector('[data-split-pane="primary"] .cm-content');csKey(content,{key:'\\',code:'Backslash',ctrlKey:true});}`)
	check("Ctrl+Backslash closes the split", `()=>csRoot().querySelectorAll('[data-editor] .cm-editor').length===1&&!csState().splitMode`)

	// A5: the gear opens the settings; tab size 2 reaches the open editor live and is stored.
	page.MustEval(`()=>{csRoot().querySelector('[data-activity="settings"]').click();}`)
	check("the gear opens the settings dialog", `async()=>{await csWait(()=>document.querySelector('[data-cs-settings]'),3000);return true;}`)
	page.MustEval(`()=>{const select=document.querySelector('[data-cs-settings] [data-setting="tabSize"]');select.value='2';select.dispatchEvent(new Event('change',{bubbles:true}));}`)
	snapshot("cs-editor-02-settings")
	check("tab size 2 applies to the open editor", `()=>{const view=csState().openTabs[csState().activeTabIndex].view;return view.state.tabSize===2&&view.state.facet(csState().cmModule.indentUnit)==='  ';}`)
	check("the settings are stored", `()=>JSON.parse(localStorage.getItem('aurago.codeStudio.settings.v1')).tabSize===2`)
	check("the status bar shows the indentation", `()=>{const item=csRoot().querySelector('[data-status-indent]');return !!item&&item.textContent.includes('2');}`)
	page.MustEval(`()=>{document.querySelector('[data-cs-settings] [data-settings-close]').click();}`)
	check("the settings dialog closes", `()=>!document.querySelector('[data-cs-settings]')`)

	// A6: Shift+Alt+F formats JSON in the browser with the configured indent, as one undo step.
	page.MustEval(`async()=>{await window.CodeStudioApp.openFile('/workspace/package.json',true,csState().windowId);await csWait(()=>{const tab=csState().openTabs[csState().activeTabIndex];return tab&&tab.path==='/workspace/package.json'&&tab.view;},10000);}`)
	page.MustEval(`()=>{const view=csState().openTabs[csState().activeTabIndex].view;view.dispatch({changes:{from:0,to:view.state.doc.length,insert:'{"name":"demo","n":12345678901234567890}'}});const content=csRoot().querySelector('.cm-content');content.focus();csKey(content,{key:'F',code:'KeyF',shiftKey:true,altKey:true});}`)
	check("Shift+Alt+F formats the JSON document", `async()=>{const view=csState().openTabs[csState().activeTabIndex].view;await csWait(()=>view.state.doc.toString().includes('\n'),5000);return view.state.doc.toString()==='{\n  "name": "demo",\n  "n": 12345678901234567890\n}';}`)
	check("formatting is a single undo step", `()=>{const view=csState().openTabs[csState().activeTabIndex].view;csState().cmModule.undo(view);return view.state.doc.toString()==='{"name":"demo","n":12345678901234567890}';}`)
	snapshot("cs-editor-03-format")

	check("no browser errors after the editor tour", `()=>fixtureErrors.length===0`)
}
```

- [ ] **Step 2: Run the checks**

Before the long browser run, check for leftover rod Chrome processes (`Temp\rod\user-data`) and ask before stopping any. Then, in PowerShell:
```
npm run build:ui -- --check
go test ./internal/server -count=1 -run "(?i)CodeStudio"
go test ./ui -count=1 -v -run "(?i)^Test(CodeMirrorBundleExportsYAMLAndLint|CodeStudioEditorBundleIsEmbedded|DesktopCodeStudioRunUsesTerminalSession|CodeStudioUsesPerWindowStateAndClosesTerminal|DesktopCodeStudio(ShellI18n|CursorI18n|CopyI18n))$"
$env:AURAGO_RUN_BROWSER_SMOKE='1'; go test ./ui -count=1 -timeout 30m -v -run "(?i)^TestDesktopCodeStudio(EditorBrowser|Browser|EditorRetention|Suggestions|LivePolicyAndDrafts|MenuIconsBrowser|IDEPanelBrowser|OverlaysKeepWindowOwnership|RenderCostAndWindowWidth)$"
```
Expected:
- The bundle check passes and the server run prints `ok`.
- The non-browser run shows 7 `--- PASS` lines.
- The browser run shows a `--- PASS` line for all 9 top-level tests, plus the subtests of `EditorRetention`, `Suggestions`, `LivePolicyAndDrafts` and `MenuIconsBrowser`, and no `--- SKIP`.
- These are the regression gates for the split rewrite: `TestDesktopCodeStudioBrowser` (split sync and tab switch), `EditorRetention/split` (snapshot and undo across panes), `Suggestions/selection in the second pane`, `LivePolicyAndDrafts` (linked views under read-only) and `MenuIconsBrowser` (new icons).

**Commit:** `test(code-studio): browser test for folding, go to line, split, settings and format`. Files: `ui/desktop_code_studio_editor_browser_test.go`.

---

## Spec coverage

| Spec item | Task(s) |
| --- | --- |
| A1: `foldGutter`, `foldKeymap`, `codeFolding`, `bracketMatching`, `indentOnInput` exported and active; `gutter` and `GutterMarker` exported; Ctrl+Shift+[ and ] fold; fold gutter styled with `--cs-*` | 1, 4 |
| A2: five direct dependencies at the installed versions (package.json and lock root); YAML, SQL, Java, PHP, XML (+svg), shell (sh, bash), Dockerfile (by name), TOML via `StreamLanguage` | 1, 4 |
| A3: Ctrl+G opens the palette with `:`; `line` and `line:col`; `@` symbols from `outline.js` | 8 |
| A4: per-pane tabs; Open to the Side from the explorer, the tab menu and Ctrl+Enter in quick open; focus makes the active tab; tab click loads into the focused pane; same file stays linked; Ctrl+\ toggles; closing the secondary tab closes the split; persisted | 7, 8 |
| A5: gear and "Preferences: Open Settings"; tab size, spaces or tabs, wrap, font size (zoom state), auto save 1 s (never read-only or virtual tabs), trim, final newline, format on save; localStorage `aurago.codeStudio.settings.v1`; defaults equal today; per-view compartment; status "Spaces: 4" / "Tab Size: 4" opens the dialog | 5, 6, 8 |
| A6: Shift+Alt+F and a command; gofmt via stdin; JSON in the browser with the configured indent; JS/TS via the TypeScript LanguageService with `NODE_PATH` from `npm root -g`; notice for other languages; one transaction and one undo step, only when the text changed; errors to Output; format on save before trim and final newline | 6, 8 |
| A7: help overlay and palette list Ctrl+D, Ctrl+click, Alt+Up/Down, Shift+Alt+Up/Down, Ctrl+/, Ctrl+Shift+K, plus the A1, A3 and A6 keys | 8 |
| Backend for A6: optional `stdin` piped with `AttachStdin` and a half-close; 413 over `MaxFileSizeMB` (1 MiB default); 4 MiB output cap with `"truncated": true` | 2 |
| Cross-cutting: 16 locales via one script; CSS under the marker with `cs-*`/`--cs-*` in light and dark; bundles rebuilt and committed with their sources; one commit set for A | 3–9 |
| Tests: Go tests for exec stdin and the output cap; one browser test with fold gutter, Ctrl+G, Open to the Side, tab size and JSON format | 2, 9 |
