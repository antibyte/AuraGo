# Code Studio IDE package 1: editor, Git workbench, project-wide replace

Status: design approved by the user on 2026-11-23. No plan and no code yet.

## Context

Code Studio is the desktop IDE app.

- Frontend: `ui/js/desktop/apps/code-studio/*.js`, concatenated into `ui/js/desktop/bundles/code-studio.bundle.js` by `npm run build:ui`.
- CSS: `ui/css/code-studio.css`.
- CodeMirror 6 vendor bundle: `ui/js/vendor/codemirror-bundle.esm.js`, built by `npm run build:codemirror` from `scripts/codemirror-entry.js`. The export list lives in `scripts/build-codemirror.js`.
- Backend: `internal/server/code_studio_handlers.go`. Git runs through `execGit`, which is `git -C /workspace --literal-pathspecs ...` via docker exec in the `aurago-code-studio` container.
- i18n: keys `codeStudio.*` in the 16 files `ui/lang/desktop/{cs,da,de,el,en,es,fr,hi,it,ja,nl,no,pl,pt,sv,zh}.json`.

This is package 1 of 4 on the road to a full IDE. Later packages (not in this spec): 2 code intelligence via LSP (needs a new container image), 3 run and debug, 4 git remote (fetch, pull, push with credentials).

## Goal

Make the editor and Git handling usable for daily work without changing the container image. Package 1 adds:

- editor upgrades (folding, more languages, Go to Line and Go to Symbol, two different files in split view, a settings dialog, formatting, discoverable shortcuts),
- a Git workbench (real staging, history, commit view, blame, branches),
- project-wide search and replace.

## Decisions the user made (do not re-ask)

1. Package 1 comes first: editor upgrades, Git workbench and project-wide replace. No container image change.
2. Real staging like VS Code replaces today's commit checkboxes.
3. Lean tests: Go tests for new endpoints and parsers, one browser test per area.

## A. Editor

### A1 Folding, bracket matching, indent on input

- Add `foldGutter`, `foldKeymap`, `codeFolding`, `bracketMatching` and `indentOnInput` (from `@codemirror/language`) to `scripts/codemirror-entry.js` and to the `expectedExports` check.
- Add `gutter` and `GutterMarker` (from `@codemirror/view`). Blame (B3) needs them.
- Rebuild the vendor bundle.
- Fold keys: Ctrl+Shift+[ and Ctrl+Shift+].
- The fold gutter is styled with `--cs-*` tokens.

### A2 More languages

- Map YAML (already bundled) in `languageExtension` in `core.js`.
- Add these as direct dependencies in `package.json`, at the versions already installed in `node_modules`:
  - `@codemirror/lang-sql` 6.10.0
  - `@codemirror/lang-java` 6.0.2
  - `@codemirror/lang-php` 6.0.2
  - `@codemirror/lang-xml` 6.1.0
  - `@codemirror/legacy-modes` 6.5.4
- Today these arrive only transitively via `@milkdown/crepe`.
- Shell, Dockerfile and TOML come from `@codemirror/legacy-modes` through `StreamLanguage` (from `@codemirror/language`).
- File extension mapping: `.yml`, `.yaml` to YAML; `.sql` to SQL; `.java` to Java; `.php` to PHP; `.xml`, `.svg` to XML; `.sh`, `.bash` to shell; `Dockerfile` (by file name) to Dockerfile; `.toml` to TOML.

### A3 Go to Line and Go to Symbol

- Ctrl+G opens the command palette prefilled with `:`.
- Input `line` or `line:col` jumps in the active editor.
- Palette prefix `@` lists the outline symbols of the active file (from `outline.js`) and jumps to the chosen one.

### A4 Split with two different files

Today `splitMode` shows the active tab twice with linked views. The new behaviour:

- State keeps the path of the tab shown in the secondary pane.
- "Open to the Side" (explorer context menu, tab context menu, Ctrl+Enter in quick open) puts that file in the other pane.
- Focusing a pane makes its tab the active tab. Every existing `activeTab()` caller (save, run, find, agent) keeps working unchanged.
- Clicking a tab in the strip loads it into the focused pane.
- Same file in both panes keeps today's linked behaviour.
- Ctrl+\ keeps toggling the split.
- Closing the secondary tab closes the split.

### A5 Settings dialog

- Opened by the gear in the activity bar and by the command "Preferences: Open Settings".
- Options:
  - tab size (2, 4 or 8)
  - indent with spaces or tabs
  - word wrap on or off
  - font size (reuses the existing zoom state)
  - auto save (off, or after a 1 s pause)
  - trim trailing whitespace on save
  - insert final newline on save
  - format on save
- Storage: browser `localStorage`, key `aurago.codeStudio.settings.v1`. Defaults equal today's behaviour: 4 spaces, wrap on, everything else off.
- Applied live to open editors through a per-view settings compartment (`indentUnit`, `EditorState.tabSize`, `lineWrapping`).
- Status bar shows "Spaces: 4" or "Tab Size: 4". Clicking it opens the dialog.

### A6 Format Document

Triggered by Shift+Alt+F and by a command.

- Go: `gofmt` in the container, reading stdin.
- JSON: in the browser, `JSON.parse` and `JSON.stringify` with the configured indent.
- JS, TS, JSX, TSX: a small node script run in the container. It sets `NODE_PATH` from `npm root -g` and uses the globally installed `typescript` package's `LanguageService.getFormattingEditsForDocument`, with the configured indent settings.
- Python, C and other languages: the image has no formatter. Show the notice "No formatter available for <language>".
- The result is applied as one CodeMirror transaction, and only when the text changed. That makes one undo step.
- Errors go to the Output panel.
- Format on save runs before trim and final newline.

### A7 Shortcut help and command palette

The shortcut help overlay and the command palette list the multi-cursor and line commands CodeMirror already has, plus the new keys:

- Ctrl+D: select next occurrence
- Ctrl+click: add cursor
- Alt+Up / Alt+Down: move line
- Shift+Alt+Up / Shift+Alt+Down: copy line
- Ctrl+/: toggle comment
- Ctrl+Shift+K: delete line
- the new keys from A1, A3 and A6

### Backend for A6

- `POST /api/code-studio/exec` gains an optional `stdin` string. It is piped to the process. It is capped at the configured `MaxFileSizeMB` (default 1 MiB). Larger input returns 413.
- Every exec gets an output cap of 4 MiB. Output beyond the cap is cut, and the response carries `"truncated": true`. Today exec has no output cap.

## B. Git workbench

### B1 Staging

- The Git sidebar shows two groups: "Staged Changes" and "Changes".
- Per file:
  - stage (+)
  - unstage (-)
  - discard (undo arrow). A confirm dialog names the file. For untracked files the dialog says that the file is deleted.
- Group buttons: stage all, unstage all.
- Commit commits only what is staged.
- When nothing is staged, a confirm asks "Stage all changes and commit?". On confirm it commits with `add_all`.
- When `/workspace` is not a Git repository, the view shows "Initialize Repository".
- A file that is staged and also changed in the work tree appears in both groups.
- Clicking a file opens the existing diff tab. The staged group opens the staged diff, the changes group opens the worktree diff.

### B2 History

- A collapsible "History" section lists the last 50 commits: short hash, subject, author, relative date.
- Command "Git: File History" filters the list to the active file.
- Clicking a commit opens a read-only commit tab. New tab kind `commit` shows the subject, body, author, date, changed files, and the diff rendered with the existing diff renderer.
- Introduce one helper, for example `isVirtualTab`. Every place that special-cases `kind === 'diff'` also covers `'commit'` through this helper.

### B3 Blame

- Command "Git: Toggle Blame" shows a blame gutter on the active editor.
- Each run of lines from the same commit shows author, relative date and short hash on its first line.
- Tooltip shows the full summary.
- Clicking a blame entry opens the commit tab.
- Lines not yet committed show "Not committed yet".
- Uses a per-view compartment. Refreshed after save.

### B4 Branches

- The branch name in the status bar opens a popup menu built with the studio's existing context-menu helper (not the command palette, which A3 changes).
- Actions:
  - switch to a branch
  - "Create new branch..."
  - delete a branch with `git branch -d`. The current branch is never deleted. A confirm is required.
- After a switch, clean open tabs reload from disk. Tabs with unsaved edits are left alone.
- Checkout errors (for example "local changes would be overwritten") are shown verbatim.

### Backend for B

All paths go through `sanitizeCodeStudioPath`, become workspace-relative, and are passed after `--`. Write endpoints need write scope and `rejectReadOnly`. Read endpoints need read scope. Follow the existing handler and route registration pattern in `code_studio_handlers.go:81-97`.

- `GET /api/code-studio/git/status`
  - Each change additionally carries `index` (porcelain X) and `worktree` (porcelain Y).
  - Existing fields `path`, `status` and `staged` stay unchanged.
  - When `/workspace` has no repository, the response gains `"is_repo": false` instead of an error.
- `POST /api/code-studio/git/stage` with `{paths[]}`: runs `git add -- <paths>`.
- `POST /api/code-studio/git/unstage` with `{paths[]}`: runs `git restore --staged -- <paths>`. When the repository has no HEAD yet, runs `git rm --cached -r -q -- <paths>`.
- `POST /api/code-studio/git/discard` with `{paths[]}`:
  - tracked files: `git restore --worktree -- <paths>`
  - untracked files: `git clean -f -q -- <paths>`, with `-d` for untracked directories
- `POST /api/code-studio/git/init`: runs `git init` when `/workspace` is not a repository. Returns 409 if it already is one.
- `GET /api/code-studio/git/show?hash=` returns `{hash, short, author, email, date (ISO 8601), subject, body, files:[{path,status}], diff, truncated}`.
  - `hash` must match `^[0-9a-fA-F]{4,40}$`.
  - `diff` is capped at 512 KiB.
- `GET /api/code-studio/git/blame?path=` returns `{lines:[{line, hash, short, author, date, summary}]}`, parsed from `git blame --porcelain`.
- `GET /api/code-studio/git/log`: entries additionally carry `short`, `author` and `date` (ISO 8601). Existing `hash` and `message` fields stay.
- `POST /api/code-studio/git/branch` gets a new action `"delete"`, which runs `git branch -d <name>`. It is refused for the current branch.
- `POST /api/code-studio/git/commit` gets a new field `staged_only`. When set, no `git add` runs before the commit.

## C. Project-wide replace

- The search panel gets a replace input, toggled like VS Code.
- Each match shows a preview: old text struck through, new text after it.
- Buttons:
  - replace one match
  - replace in one file
  - replace all. Asks "Replace N occurrences in M files?".
- Matching in the browser uses a JS `RegExp` built from the same options as the search:
  - literal text is escaped
  - whole word uses `\b`
  - case flag as in the search
- Regex mode supports `$1`-style groups with JavaScript semantics. The preview shows exactly what will be written.
- When the search result list is truncated (server cap 200), Replace All is disabled with a hint to narrow the search. Per-file replace stays available for the listed files.
- Content per file comes from the open tab if the file is open, otherwise from `GET /file`.
  - Open tabs get the change as one editor transaction in the buffer. It is undoable. The tab stays dirty or becomes dirty. Nothing is written.
  - Files that are not open are written with `PUT /file`.
- Read-only mode disables replace.
- After a replace, the search runs again.
- No backend change for C.

## Cross-cutting

- All new UI strings are `codeStudio.*` keys in all 16 locale files.
- New CSS goes in `ui/css/code-studio.css` with `cs-*` classes and `--cs-*` tokens. It must be readable in light and dark themes.
- Rebuild the bundles with `npm run build:codemirror` and `npm run build:ui`. Commit the generated files together with the sources.
- Commit per sub-package: one commit set for A, one for B, one for C.

### Tests

- Go tests in `internal/server` for every new endpoint. Argument checks use the existing fake exec pattern.
- Go tests for the porcelain X/Y parser, the blame parser, the log parser, exec `stdin` and the output cap.
- Browser tests in the `ui` package, using rod and the existing `newCodeStudioBrowser` fixture. One test per area:
  - Editor: fold gutter, Ctrl+G, open to the side shows two files, tab size setting, JSON format.
  - Git: stage and unstage, staged-only commit request, history, commit tab, blame gutter.
  - Replace: preview, replace all writes the expected content, a dirty open tab stays dirty.

## Out of scope

LSP, hover and go to definition (package 2); debugger and run configurations (package 3); git fetch, pull and push (package 4); minimap; switching search to ripgrep; syncing settings to the server.
