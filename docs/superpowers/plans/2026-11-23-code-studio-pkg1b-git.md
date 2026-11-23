# Code Studio IDE Package 1B: Git Workbench Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Turn the Code Studio Git panel into a real workbench: explicit staging (Staged Changes / Changes, stage, unstage, discard, stage all, unstage all), staged-only commits, Initialize Repository, a 50-commit History with a File History filter, a read-only commit tab, a blame gutter, and a branch menu on the status bar. All of this runs on the existing `aurago-code-studio` container. The image does not change.

**Architecture:** The backend adds six git endpoints in a new file, `internal/server/code_studio_git_workbench.go`: stage, unstage, discard, init, show and blame, plus their parsers. Each runs through `execGit`, which builds an argument vector and never a shell string. Paths are sanitized and made workspace-relative, then passed after `--`. `code_studio_handlers.go` keeps only the route registration and the extensions to status, log, porcelain, commit and branch. The frontend rewrites `git.js` (panel, staging, commit, init, branch menu) and adds a new fragment, `git-history.js` (history, commit tab, blame gutter), inside the shared Code Studio IIFE. A single helper, `isVirtualTab(tab)`, replaces every `kind === 'diff'` special case and also covers the new `commit` tab kind. Blame uses a per-view CodeMirror `Compartment` with `cm.gutter` and `cm.GutterMarker`. Plan A exports both from the vendor bundle.

**Tech Stack:** Go (`net/http`, docker exec through `codeStudioDockerAPI`), vanilla JS fragments concatenated by `scripts/build-ui-bundles.js`, CodeMirror 6 (vendor ESM bundle), Go `testing` with the recording docker fake, and go-rod browser tests with the `newCodeStudioBrowser` fixture.

---

## Notes for the executor

- **Plan A runs first on the same tree.** Line numbers below are from before A (commit 8c769bbe9) and will have shifted. Always find an anchor by its quoted text, not by its line number. A edits these shared regions:
  - `createCodeMirrorEditor` (extension list, settings compartment)
  - the command list in `command-palette.js` (`getCommands`, ':' and '@' modes, Ctrl+Enter)
  - `studioCommandTable` and `window.CodeStudioApp` in `shortcuts.js`
  - `renderStatus` (indent item)
  - `saveTab` (format on save)
  - `splitEditor`, `renderEditor` and `openFile` ("Open to the Side")

  Each anchor below was chosen on a line that A has no reason to change. If an anchor is still not found verbatim, apply the same substitution to A's version of that line. The substitution is always one of two things:
  - `X.kind === 'diff'` becomes `isVirtualTab(X)`, and `X.kind !== 'diff'` becomes `!isVirtualTab(X)`.
  - Insert the new lines directly after the named existing line.
- **Pointer-receiver trap (Go).** `codeContainer` installs the Docker adapter on its receiver. The shared request helper `gitPathsRequest` must therefore have a pointer receiver. If it took a value receiver, the handler's own `h.docker` would stay nil in production.
- **Write-tool trap.** Do not write `\u` escape sequences in any source file. The Write tool decodes them. This plan uses literal glyphs and `String.fromCodePoint` instead.
- **Concurrent sessions.** Other sessions build and commit in this same checkout. `npm run build:ui` rewrites every bundle. Commit only `ui/js/desktop/bundles/code-studio.bundle.js` from the generated output.
- **Project rule (CLAUDE.md).** Before editing an existing function, run GitNexus `impact` upstream on it and report the risk. Each task lists the symbols. Before each commit, the controller runs `detect_changes`.

## File Structure

| File | Change | Responsibility |
| --- | --- | --- |
| `internal/server/code_studio_git_workbench.go` | Create | Workbench constants, parsers (log, show, blame, path list), handlers for stage, unstage, discard, init, show, blame |
| `internal/server/code_studio_git_workbench_test.go` | Create | Go tests for all new endpoints and parsers |
| `internal/server/code_studio_handlers.go` | Modify | Routes; `is_repo` and new log format in status; log path sanitizing; porcelain `index` and `worktree`; commit `staged_only`; branch `delete`; old `parseGitLog` removed |
| `internal/server/code_studio_git_search_test.go` | Modify | Porcelain expectation gains `index` and `worktree` |
| `C:\disposable\code-studio-pkg1\locales-b.mjs` | Create (not committed) | Merges 29 keys into 16 locale files |
| `ui/lang/desktop/{cs,da,de,el,en,es,fr,hi,it,ja,nl,no,pl,pt,sv,zh}.json` | Modify | New `codeStudio.git*` strings |
| `ui/js/desktop/apps/code-studio/core.js` | Modify | `isVirtualTab`, `studioTabLabel`, API wrappers, state fields, policy selector, status-bar branch item, virtual-tab routing |
| `ui/js/desktop/apps/code-studio/git.js` | Rewrite | Source Control panel, staging, commit, init, branch menu, diff tab |
| `ui/js/desktop/apps/code-studio/git-history.js` | Create | History list, commit tab, blame gutter |
| `ui/js/desktop/apps/code-studio/editor.js` | Modify | Commit-tab rendering branch, blame compartment |
| `ui/js/desktop/apps/code-studio/{agent,outline,panels,problems}.js` | Modify | `isVirtualTab` routing |
| `ui/js/desktop/apps/code-studio/command-palette.js` | Modify | `isVirtualTab` routing, two Git commands |
| `ui/js/desktop/apps/code-studio/shortcuts.js` | Modify | Command table entries, exposes `isVirtualTab` |
| `scripts/build-ui-bundles.js` | Modify | Adds `git-history.js` to the code-studio bundle |
| `ui/js/desktop/bundles/code-studio.bundle.js` | Generated | `npm run build:ui` |
| `ui/css/code-studio.css` | Modify (append) | `/* === IDE package 1B: git === */` section |
| `ui/code_studio_git_selection_test.go` | Rewrite | Source markers and locale checks for the staging design |
| `ui/desktop_code_studio_git_browser_test.go` | Create | One browser test for the Git workbench |

## Spec coverage map

| Spec item | Task |
| --- | --- |
| Status `index`/`worktree`, `is_repo:false`; log `short`/`author`/`date` | 1 |
| stage, unstage (with the no-HEAD case), discard (tracked vs untracked), init (409 if already a repository) | 2 |
| show (hash regex, 512 KiB cap, `truncated`), blame (porcelain parser) | 3 |
| commit `staged_only`; branch `delete`, never the current branch | 4 |
| 16 locales | 5 |
| `isVirtualTab` routing at every `kind === 'diff'` site | 6 |
| B1 groups, row and group actions, commit flow, Initialize Repository | 7 |
| B2 History and File History | 7 |
| B3 blame | 7 |
| B4 branch menu (status bar), tab reload after checkout, errors shown verbatim | 7 |
| Light- and dark-readable CSS; browser test | 8 |

---

### Task 1: Status, log and porcelain parsers (backend)

**Files:**
- Create: `internal/server/code_studio_git_workbench.go`
- Create: `internal/server/code_studio_git_workbench_test.go`
- Modify: `internal/server/code_studio_handlers.go`
- Modify: `internal/server/code_studio_git_search_test.go`

- [ ] **Step 1: Impact check.** Run `impact` upstream for `handleGitStatus`, `handleGitLog`, `parseGitPorcelain` and `parseGitLog`. The index may not resolve frontend callers through plain JS fetch calls, so treat `UNKNOWN` as unresolved. The text search already done for this plan found these callers only: `git.js` (status), the `apiClient.gitLog` wrapper (unused) and the Go tests. Report the risk.

- [ ] **Step 2: Create `internal/server/code_studio_git_workbench.go`** with this complete content:

```go
package server

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// Git workbench of Code Studio: staging, discard, repository init, commit view and blame.
// Every command runs through execGit (an argument vector, never a shell string) and every
// path is workspace-relative and passed after "--".

const (
	// The log fields are separated by the ASCII unit separator, which cannot occur in a
	// hash, a name, an ISO date or a one-line subject.
	codeStudioGitLogFormat = "--format=%H%x1f%h%x1f%an%x1f%aI%x1f%s"

	// The commit metadata ends with a record separator; the --name-status list follows it.
	codeStudioGitShowFormat = "--format=%H%x1f%h%x1f%an%x1f%ae%x1f%aI%x1f%s%x1f%b%x1e"

	// codeStudioGitShowDiffLimit caps the patch of the commit view at 512 KiB.
	codeStudioGitShowDiffLimit = 512 * 1024

	// codeStudioGitMaxPaths bounds one stage, unstage or discard request.
	codeStudioGitMaxPaths = 1000
)

var codeStudioGitHashPattern = regexp.MustCompile(`^[0-9a-fA-F]{4,40}$`)

type codeStudioGitFileChange struct {
	Path   string `json:"path"`
	Status string `json:"status"`
}

type codeStudioGitCommit struct {
	Hash    string
	Short   string
	Author  string
	Email   string
	Date    string
	Subject string
	Body    string
	Files   []codeStudioGitFileChange
}

type codeStudioGitBlameLine struct {
	Line    int    `json:"line"`
	Hash    string `json:"hash"`
	Short   string `json:"short"`
	Author  string `json:"author"`
	Date    string `json:"date"`
	Summary string `json:"summary"`
}

// codeStudioGitLogArgs is the log command of the history list and of the status summary.
func codeStudioGitLogArgs(count int) []string {
	return []string{"log", codeStudioGitLogFormat, "-n", strconv.Itoa(count)}
}

// codeStudioGitNotRepository reports git's "not a git repository" failure.
func codeStudioGitNotRepository(output string) bool {
	return strings.Contains(strings.ToLower(output), "not a git repository")
}

// codeStudioGitPaths turns request paths into unique workspace-relative pathspecs. The
// workspace root itself is refused, so no request can name every file at once.
func codeStudioGitPaths(raw []string) ([]string, error) {
	if len(raw) == 0 {
		return nil, fmt.Errorf("paths are required")
	}
	if len(raw) > codeStudioGitMaxPaths {
		return nil, fmt.Errorf("too many paths")
	}
	rels := make([]string, 0, len(raw))
	seen := make(map[string]bool, len(raw))
	for _, item := range raw {
		rel, err := codeStudioGitAddPath(item)
		if err != nil {
			return nil, err
		}
		if !seen[rel] {
			seen[rel] = true
			rels = append(rels, rel)
		}
	}
	return rels, nil
}

func codeStudioGitIsHex(value string) bool {
	return value != "" && strings.Trim(value, "0123456789abcdefABCDEF") == ""
}

// parseGitLog reads the output of codeStudioGitLogArgs. Lines without the five fields
// (warnings, stderr noise) are skipped.
func parseGitLog(output string) []map[string]string {
	entries := []map[string]string{}
	for _, line := range strings.Split(output, "\n") {
		fields := strings.SplitN(strings.TrimRight(line, "\r"), "\x1f", 5)
		if len(fields) != 5 {
			continue
		}
		hash := strings.TrimSpace(fields[0])
		if len(hash) < 4 || !codeStudioGitIsHex(hash) {
			continue
		}
		entries = append(entries, map[string]string{
			"hash":    hash,
			"short":   fields[1],
			"author":  fields[2],
			"date":    fields[3],
			"message": fields[4],
		})
	}
	return entries
}

// parseGitShowMeta reads `git show --name-status` with codeStudioGitShowFormat.
func parseGitShowMeta(output string) (codeStudioGitCommit, error) {
	head, rest, found := strings.Cut(output, "\x1e")
	fields := strings.SplitN(head, "\x1f", 7)
	if !found || len(fields) != 7 {
		return codeStudioGitCommit{}, fmt.Errorf("unexpected git show output")
	}
	commit := codeStudioGitCommit{
		Hash:    strings.TrimSpace(fields[0]),
		Short:   strings.TrimSpace(fields[1]),
		Author:  fields[2],
		Email:   fields[3],
		Date:    strings.TrimSpace(fields[4]),
		Subject: fields[5],
		Body:    strings.TrimSpace(fields[6]),
		Files:   []codeStudioGitFileChange{},
	}
	for _, line := range strings.Split(rest, "\n") {
		parts := strings.Split(strings.TrimRight(line, "\r"), "\t")
		if len(parts) < 2 || parts[0] == "" {
			continue
		}
		// Renames and copies list the old and the new path; the new one is the last field.
		commit.Files = append(commit.Files, codeStudioGitFileChange{
			Path:   codeStudioGitUnquotePath(parts[len(parts)-1]),
			Status: parts[0][:1],
		})
	}
	return commit, nil
}

// codeStudioGitUnquotePath undoes git's C-style quoting of paths with control characters.
func codeStudioGitUnquotePath(path string) string {
	if len(path) >= 2 && strings.HasPrefix(path, `"`) && strings.HasSuffix(path, `"`) {
		if unquoted, err := strconv.Unquote(path); err == nil {
			return unquoted
		}
	}
	return path
}

// truncateCodeStudioGitDiff cuts a diff to limit bytes at a line end.
func truncateCodeStudioGitDiff(diff string, limit int) (string, bool) {
	if len(diff) <= limit {
		return diff, false
	}
	cut := diff[:limit]
	if index := strings.LastIndexByte(cut, '\n'); index > 0 {
		cut = cut[:index+1]
	}
	return strings.ToValidUTF8(cut, ""), true
}

// codeStudioGitZone turns a git zone such as "+0100" or "-0230" into a fixed location.
func codeStudioGitZone(zone string) *time.Location {
	if len(zone) != 5 || (zone[0] != '+' && zone[0] != '-') {
		return time.UTC
	}
	hours, hoursErr := strconv.Atoi(zone[1:3])
	minutes, minutesErr := strconv.Atoi(zone[3:5])
	if hoursErr != nil || minutesErr != nil {
		return time.UTC
	}
	offset := hours*3600 + minutes*60
	if zone[0] == '-' {
		offset = -offset
	}
	return time.FixedZone(zone, offset)
}

// parseGitBlamePorcelain reads `git blame --porcelain`. The details of a commit are printed
// only the first time it appears, so they are collected first and joined to every line at
// the end. Source lines start with a TAB and are never parsed.
func parseGitBlamePorcelain(output string) []codeStudioGitBlameLine {
	type blameCommit struct {
		author     string
		summary    string
		zone       string
		authorTime int64
		hasTime    bool
	}
	type blameRef struct {
		hash string
		line int
	}
	commits := map[string]*blameCommit{}
	refs := []blameRef{}
	current := ""
	finalLine := 0
	for _, raw := range strings.Split(output, "\n") {
		if strings.HasPrefix(raw, "\t") {
			if current != "" && finalLine > 0 {
				refs = append(refs, blameRef{hash: current, line: finalLine})
			}
			continue
		}
		line := strings.TrimRight(raw, "\r")
		fields := strings.Fields(line)
		if len(fields) >= 3 && (len(fields[0]) == 40 || len(fields[0]) == 64) && codeStudioGitIsHex(fields[0]) {
			if number, err := strconv.Atoi(fields[2]); err == nil {
				current = fields[0]
				finalLine = number
				if commits[current] == nil {
					commits[current] = &blameCommit{}
				}
				continue
			}
		}
		if current == "" {
			continue
		}
		key, value, _ := strings.Cut(line, " ")
		info := commits[current]
		switch key {
		case "author":
			info.author = value
		case "author-time":
			if seconds, err := strconv.ParseInt(value, 10, 64); err == nil {
				info.authorTime = seconds
				info.hasTime = true
			}
		case "author-tz":
			info.zone = value
		case "summary":
			info.summary = value
		}
	}
	lines := make([]codeStudioGitBlameLine, 0, len(refs))
	for _, ref := range refs {
		info := commits[ref.hash]
		short := ref.hash
		if len(short) > 7 {
			short = short[:7]
		}
		date := ""
		if info.hasTime {
			date = time.Unix(info.authorTime, 0).In(codeStudioGitZone(info.zone)).Format(time.RFC3339)
		}
		lines = append(lines, codeStudioGitBlameLine{Line: ref.line, Hash: ref.hash, Short: short, Author: info.author, Date: date, Summary: info.summary})
	}
	return lines
}
```

- [ ] **Step 3: Extend the porcelain parser.** In `internal/server/code_studio_handlers.go`:

Replace:
```go
		changes = append(changes, map[string]string{"path": path, "status": status, "staged": staged})
```
With:
```go
		changes = append(changes, map[string]string{"path": path, "status": status, "staged": staged, "index": line[:1], "worktree": line[1:2]})
```

- [ ] **Step 4: Remove the old log parser** from `internal/server/code_studio_handlers.go`. The new one lives in the workbench file.

Replace:
```go
func parseGitLog(output string) []map[string]string {
	output = strings.TrimSpace(output)
	if output == "" {
		return []map[string]string{}
	}
	lines := strings.Split(output, "\n")
	entries := make([]map[string]string, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, " ", 2)
		entry := map[string]string{"hash": parts[0]}
		if len(parts) > 1 {
			entry["message"] = strings.TrimSpace(parts[1])
		} else {
			entry["message"] = ""
		}
		entries = append(entries, entry)
	}
	return entries
}

func parseGitBranches(output string) []string {
```
With:
```go
func parseGitBranches(output string) []string {
```

- [ ] **Step 5: Status reports `is_repo` and uses the new log format.** In `internal/server/code_studio_handlers.go`:

Replace:
```go
	changes := []map[string]string{}
	branch := ""
	logEntries := []map[string]string{}
	if statusResult.ExitCode == 0 {
		changes = parseGitPorcelain(statusResult.Output)
		if result, err := h.execGit(r.Context(), containerID, 10*time.Second, "branch", "--show-current"); err == nil && result.ExitCode == 0 {
			branch = strings.TrimSpace(result.Output)
		}
		// A new repository has no log yet. Neither that error nor a missing
		// repository should appear as a branch, file change or commit.
		if result, err := h.execGit(r.Context(), containerID, 10*time.Second, "log", "--oneline", "-10"); err == nil && result.ExitCode == 0 {
			logEntries = parseGitLog(result.Output)
		}
	}

	writeJSON(w, map[string]interface{}{
		"status":  "ok",
		"branch":  branch,
		"changes": changes,
		"log":     logEntries,
	})
```
With:
```go
	changes := []map[string]string{}
	branch := ""
	logEntries := []map[string]string{}
	// Only git's own "not a git repository" answer clears is_repo, so another failure
	// (for example dubious ownership) never offers to initialize over a repository.
	isRepo := true
	if statusResult.ExitCode == 0 {
		changes = parseGitPorcelain(statusResult.Output)
		if result, err := h.execGit(r.Context(), containerID, 10*time.Second, "branch", "--show-current"); err == nil && result.ExitCode == 0 {
			branch = strings.TrimSpace(result.Output)
		}
		// A new repository has no log yet. Neither that error nor a missing
		// repository should appear as a branch, file change or commit.
		if result, err := h.execGit(r.Context(), containerID, 10*time.Second, codeStudioGitLogArgs(10)...); err == nil && result.ExitCode == 0 {
			logEntries = parseGitLog(result.Output)
		}
	} else if codeStudioGitNotRepository(statusResult.Output) {
		isRepo = false
	}

	writeJSON(w, map[string]interface{}{
		"status":  "ok",
		"branch":  branch,
		"changes": changes,
		"log":     logEntries,
		"is_repo": isRepo,
	})
```

- [ ] **Step 6: Log uses the new format and a sanitized path.** In `internal/server/code_studio_handlers.go`, inside `handleGitLog`:

Replace:
```go
	file := r.URL.Query().Get("path")
	args := []string{"log", "--oneline", "-n", strconv.Itoa(count)}
	if file != "" {
		args = append(args, "--", file)
	}
```
With:
```go
	args := codeStudioGitLogArgs(count)
	if file := r.URL.Query().Get("path"); file != "" {
		rel, err := codeStudioGitAddPath(file)
		if err != nil {
			jsonError(w, err.Error(), http.StatusBadRequest)
			return
		}
		args = append(args, "--", rel)
	}
```

- [ ] **Step 7: Update the existing porcelain expectation.** In `internal/server/code_studio_git_search_test.go`:

Replace:
```go
	want := []map[string]string{
		{"path": "main.go", "status": "M", "staged": "false"}, {"path": "staged.go", "status": "M", "staged": "true"},
		{"path": "deleted.go", "status": "D", "staged": "false"}, {"path": "-option.txt", "status": "??", "staged": "false"},
		{"path": " spaced name ", "status": "??", "staged": "false"}, {"path": "Grüße\n文.txt", "status": "??", "staged": "false"},
		{"path": "renamed.go", "status": "R", "staged": "true"}, {"path": "copied.go", "status": "C", "staged": "false"},
		{"path": "last.txt", "status": "??", "staged": "false"},
	}
```
With:
```go
	want := []map[string]string{
		{"path": "main.go", "status": "M", "staged": "false", "index": " ", "worktree": "M"},
		{"path": "staged.go", "status": "M", "staged": "true", "index": "M", "worktree": " "},
		{"path": "deleted.go", "status": "D", "staged": "false", "index": " ", "worktree": "D"},
		{"path": "-option.txt", "status": "??", "staged": "false", "index": "?", "worktree": "?"},
		{"path": " spaced name ", "status": "??", "staged": "false", "index": "?", "worktree": "?"},
		{"path": "Grüße\n文.txt", "status": "??", "staged": "false", "index": "?", "worktree": "?"},
		{"path": "renamed.go", "status": "R", "staged": "true", "index": "R", "worktree": " "},
		{"path": "copied.go", "status": "C", "staged": "false", "index": " ", "worktree": "C"},
		{"path": "last.txt", "status": "??", "staged": "false", "index": "?", "worktree": "?"},
	}
```

- [ ] **Step 8: Create `internal/server/code_studio_git_workbench_test.go`** with this complete content:

```go
package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
)

// codeStudioGitCmd is the docker exec command execGit builds for the given git arguments.
func codeStudioGitCmd(args ...string) []string {
	return append([]string{"git", "-C", "/workspace", "--literal-pathspecs"}, args...)
}

func TestCodeStudioGitPorcelainKeepsIndexAndWorktree(t *testing.T) {
	got := parseGitPorcelain("MM both.go\x00A  added.go\x00 D gone.go\x00?? new dir/\x00")
	want := []map[string]string{
		{"path": "both.go", "status": "M", "staged": "true", "index": "M", "worktree": "M"},
		{"path": "added.go", "status": "A", "staged": "true", "index": "A", "worktree": " "},
		{"path": "gone.go", "status": "D", "staged": "false", "index": " ", "worktree": "D"},
		{"path": "new dir/", "status": "??", "staged": "false", "index": "?", "worktree": "?"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("porcelain=%#v; want=%#v", got, want)
	}
}

func TestCodeStudioGitStatusReportsRepositoryState(t *testing.T) {
	s := testCodeStudioServerWithFakeCodeContainer(t, 1)
	type statusBody struct {
		IsRepo  *bool               `json:"is_repo"`
		Branch  string              `json:"branch"`
		Changes []map[string]string `json:"changes"`
	}
	run := func(results ...codeStudioExecResult) (statusBody, *recordingCodeStudioDockerAPI, int) {
		t.Helper()
		docker := &recordingCodeStudioDockerAPI{results: results}
		h := codeStudioHandlers{server: s, docker: docker}
		rec := httptest.NewRecorder()
		h.handleGitStatus(rec, httptest.NewRequest(http.MethodGet, "/api/code-studio/git/status", nil))
		var body statusBody
		if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
			t.Fatalf("status body %q: %v", rec.Body.String(), err)
		}
		return body, docker, rec.Code
	}

	body, docker, code := run(codeStudioExecResult{ExitCode: 128, Output: "fatal: not a git repository (or any of the parent directories): .git\n"})
	if code != http.StatusOK || body.IsRepo == nil || *body.IsRepo || len(body.Changes) != 0 || len(docker.commands) != 1 {
		t.Fatalf("missing repository: code=%d body=%+v commands=%q", code, body, docker.commands)
	}

	body, _, code = run(codeStudioExecResult{ExitCode: 128, Output: "fatal: detected dubious ownership in repository at '/workspace'\n"})
	if code != http.StatusOK || body.IsRepo == nil || !*body.IsRepo {
		t.Fatalf("other git failures must not offer init: code=%d body=%+v", code, body)
	}

	body, docker, code = run(codeStudioExecResult{Output: "MM both.go\x00"}, codeStudioExecResult{Output: "main\n"}, codeStudioExecResult{})
	wantChanges := []map[string]string{{"path": "both.go", "status": "M", "staged": "true", "index": "M", "worktree": "M"}}
	if code != http.StatusOK || body.IsRepo == nil || !*body.IsRepo || body.Branch != "main" || !reflect.DeepEqual(body.Changes, wantChanges) {
		t.Fatalf("repository status: code=%d body=%+v", code, body)
	}
	if len(docker.commands) != 3 || !reflect.DeepEqual(docker.commands[2], codeStudioGitCmd("log", codeStudioGitLogFormat, "-n", "10")) {
		t.Fatalf("status log command=%q", docker.commands)
	}
}

func TestCodeStudioGitLogCarriesAuthorAndDate(t *testing.T) {
	output := "0123456789abcdef0123456789abcdef01234567\x1f0123456\x1fAda Lovelace\x1f2026-11-20T10:00:00+01:00\x1fAdd engine\n" +
		"warning: noise without fields\n" +
		"89abcdef0123456789abcdef0123456789abcdef\x1f89abcde\x1fGrace Hopper\x1f2026-11-19T09:00:00Z\x1fFix: a\x1fb\n"
	want := []map[string]string{
		{"hash": "0123456789abcdef0123456789abcdef01234567", "short": "0123456", "author": "Ada Lovelace", "date": "2026-11-20T10:00:00+01:00", "message": "Add engine"},
		{"hash": "89abcdef0123456789abcdef0123456789abcdef", "short": "89abcde", "author": "Grace Hopper", "date": "2026-11-19T09:00:00Z", "message": "Fix: a\x1fb"},
	}
	if got := parseGitLog(output); !reflect.DeepEqual(got, want) {
		t.Fatalf("log=%#v; want=%#v", got, want)
	}
	if got := parseGitLog(""); got == nil || len(got) != 0 {
		t.Fatalf("empty log=%#v", got)
	}

	s := testCodeStudioServerWithFakeCodeContainer(t, 1)
	docker := &recordingCodeStudioDockerAPI{results: []codeStudioExecResult{{Output: output}}}
	h := codeStudioHandlers{server: s, docker: docker}
	rec := httptest.NewRecorder()
	h.handleGitLog(rec, httptest.NewRequest(http.MethodGet, "/api/code-studio/git/log?count=50&path=/workspace/src/app.js", nil))
	if rec.Code != http.StatusOK || !reflect.DeepEqual(docker.commands, [][]string{codeStudioGitCmd("log", codeStudioGitLogFormat, "-n", "50", "--", "src/app.js")}) {
		t.Fatalf("log status=%d commands=%q body=%s", rec.Code, docker.commands, rec.Body.String())
	}
	var body struct{ Log []map[string]string }
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil || !reflect.DeepEqual(body.Log, want) {
		t.Fatalf("log body=%s err=%v", rec.Body.String(), err)
	}

	docker = &recordingCodeStudioDockerAPI{}
	h.docker = docker
	rec = httptest.NewRecorder()
	h.handleGitLog(rec, httptest.NewRequest(http.MethodGet, "/api/code-studio/git/log?path=../etc/passwd", nil))
	if rec.Code != http.StatusBadRequest || len(docker.commands) != 0 {
		t.Fatalf("escaping log path: status=%d commands=%q", rec.Code, docker.commands)
	}
}

func TestCodeStudioGitBlamePorcelainParser(t *testing.T) {
	commit := strings.Repeat("a", 40)
	zero := strings.Repeat("0", 40)
	porcelain := strings.Join([]string{
		commit + " 1 1 2",
		"author Ada Lovelace",
		"author-mail <ada@example.com>",
		"author-time 1700000000",
		"author-tz +0100",
		"committer Ada Lovelace",
		"committer-mail <ada@example.com>",
		"committer-time 1700000000",
		"committer-tz +0100",
		"summary First commit",
		"boundary",
		"filename src/app.js",
		"\tline one",
		commit + " 2 2",
		"\tline two",
		zero + " 3 3 1",
		"author Not Committed Yet",
		"author-mail <not.committed.yet>",
		"author-time 1700003600",
		"author-tz -0230",
		"committer Not Committed Yet",
		"committer-mail <not.committed.yet>",
		"committer-time 1700003600",
		"committer-tz -0230",
		"summary Version of src/app.js from src/app.js",
		"previous " + commit + " src/app.js",
		"filename src/app.js",
		"\tauthor looks like a key",
		commit + " 4 4 1",
		"\tline four",
		"",
	}, "\n")
	first := codeStudioGitBlameLine{Hash: commit, Short: "aaaaaaa", Author: "Ada Lovelace", Date: "2023-11-14T23:13:20+01:00", Summary: "First commit"}
	lineOf := func(number int, base codeStudioGitBlameLine) codeStudioGitBlameLine {
		base.Line = number
		return base
	}
	want := []codeStudioGitBlameLine{
		lineOf(1, first),
		lineOf(2, first),
		{Line: 3, Hash: zero, Short: "0000000", Author: "Not Committed Yet", Date: "2023-11-14T20:43:20-02:30", Summary: "Version of src/app.js from src/app.js"},
		lineOf(4, first),
	}
	if got := parseGitBlamePorcelain(porcelain); !reflect.DeepEqual(got, want) {
		t.Fatalf("blame=%#v; want=%#v", got, want)
	}
}

func TestCodeStudioGitShowParserAndDiffCap(t *testing.T) {
	full := "0123456789abcdef0123456789abcdef01234567"
	meta := full + "\x1f0123456\x1fAda Lovelace\x1fada@example.com\x1f2026-11-20T10:00:00+01:00\x1fAdd engine\x1fLonger body\nsecond line\n\x1e\nM\tmain.go\nA\tdocs/new file.md\nR100\told.go\tnew.go\nM\t\"tab\\there.txt\"\n"
	got, err := parseGitShowMeta(meta)
	if err != nil {
		t.Fatal(err)
	}
	want := codeStudioGitCommit{
		Hash: full, Short: "0123456", Author: "Ada Lovelace", Email: "ada@example.com",
		Date: "2026-11-20T10:00:00+01:00", Subject: "Add engine", Body: "Longer body\nsecond line",
		Files: []codeStudioGitFileChange{
			{Path: "main.go", Status: "M"},
			{Path: "docs/new file.md", Status: "A"},
			{Path: "new.go", Status: "R"},
			{Path: "tab\there.txt", Status: "M"},
		},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("show=%#v; want=%#v", got, want)
	}
	if _, err := parseGitShowMeta("fatal: bad object\n"); err == nil {
		t.Fatal("unexpected show output must fail")
	}

	big := strings.Repeat("+0123456789abcdef\n", 40000)
	cut, truncated := truncateCodeStudioGitDiff(big, codeStudioGitShowDiffLimit)
	if !truncated || len(cut) > codeStudioGitShowDiffLimit || !strings.HasSuffix(cut, "\n") || !strings.HasPrefix(big, cut) {
		t.Fatalf("cap: truncated=%v len=%d", truncated, len(cut))
	}
	if same, truncated := truncateCodeStudioGitDiff("small\n", codeStudioGitShowDiffLimit); truncated || same != "small\n" {
		t.Fatalf("small diff changed: %q %v", same, truncated)
	}
}
```

- [ ] **Step 9: Format and test.**

Run: `gofmt -l internal/server/code_studio_git_workbench.go internal/server/code_studio_git_workbench_test.go internal/server/code_studio_handlers.go`
Expected: no output. If a file is listed, run `gofmt -w` on it.

Run: `go test ./internal/server -count=1 -run "(?i)(CodeStudioGit|CodeStudioReadonly)"`
Expected: PASS. Count the PASS lines: the new tests (`TestCodeStudioGitPorcelainKeepsIndexAndWorktree`, `TestCodeStudioGitStatusReportsRepositoryState`, `TestCodeStudioGitLogCarriesAuthorAndDate`, `TestCodeStudioGitBlamePorcelainParser`, `TestCodeStudioGitShowParserAndDiffCap`) plus the existing ones (`TestCodeStudioGitPorcelainPreservesPaths`, `TestCodeStudioGitStatusAndDiffUseExactPaths`, `TestCodeStudioGitFailuresAreNotRepositoryData`).

Commit: `feat(code-studio-api): report is_repo, porcelain index/worktree and log authors`. Files: `internal/server/code_studio_git_workbench.go`, `internal/server/code_studio_git_workbench_test.go`, `internal/server/code_studio_handlers.go`, `internal/server/code_studio_git_search_test.go`

---

### Task 2: Stage, unstage, discard and init endpoints (backend)

**Files:**
- Modify: `internal/server/code_studio_git_workbench.go`
- Modify: `internal/server/code_studio_handlers.go` (routes)
- Modify: `internal/server/code_studio_git_workbench_test.go`

- [ ] **Step 1: Impact check.** Run `impact` upstream on `registerCodeStudioRoutes` and report the risk. All handlers in this task are new.

- [ ] **Step 2: Add `net/http` to the workbench imports.** In `internal/server/code_studio_git_workbench.go`:

Replace:
```go
import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
)
```
With:
```go
import (
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"
)
```

- [ ] **Step 3: Append the write handlers** to the end of `internal/server/code_studio_git_workbench.go`:

```go

// gitPathsRequest checks a write request with the body {"paths": [...]} and resolves the
// container. The pointer receiver matters: codeContainer installs the Docker adapter on h,
// and the calling handler runs git through that same value.
func (h *codeStudioHandlers) gitPathsRequest(w http.ResponseWriter, r *http.Request) ([]string, string, bool) {
	if !h.requirePermission(w, r, desktopScopeWrite) {
		return nil, "", false
	}
	if h.rejectReadOnly(w) {
		return nil, "", false
	}
	if r.Method != http.MethodPost {
		jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
		return nil, "", false
	}
	var body struct {
		Paths []string `json:"paths"`
	}
	if err := decodeJSONRequest(w, r, &body, h.server); err != nil {
		jsonError(w, "Invalid JSON", jsonDecodeStatus(err))
		return nil, "", false
	}
	rels, err := codeStudioGitPaths(body.Paths)
	if err != nil {
		jsonError(w, err.Error(), http.StatusBadRequest)
		return nil, "", false
	}
	_, containerID, err := h.codeContainer(r.Context(), true)
	if err != nil {
		jsonError(w, err.Error(), http.StatusServiceUnavailable)
		return nil, "", false
	}
	return rels, containerID, true
}

// runGitSteps runs the commands in order and stops at the first failure, whose git output
// is returned verbatim.
func (h codeStudioHandlers) runGitSteps(w http.ResponseWriter, r *http.Request, containerID string, steps [][]string, fallback string) {
	for _, args := range steps {
		result, err := h.execGit(r.Context(), containerID, 30*time.Second, args...)
		if err != nil {
			jsonError(w, err.Error(), http.StatusBadGateway)
			return
		}
		if result.ExitCode != 0 {
			jsonError(w, codeStudioGitExecError(result.Output, fallback), http.StatusBadRequest)
			return
		}
	}
	writeJSON(w, map[string]interface{}{"status": "ok"})
}

func (h codeStudioHandlers) handleGitStage(w http.ResponseWriter, r *http.Request) {
	rels, containerID, ok := h.gitPathsRequest(w, r)
	if !ok {
		return
	}
	h.runGitSteps(w, r, containerID, [][]string{append([]string{"add", "--"}, rels...)}, "git add failed")
}

func (h codeStudioHandlers) handleGitUnstage(w http.ResponseWriter, r *http.Request) {
	rels, containerID, ok := h.gitPathsRequest(w, r)
	if !ok {
		return
	}
	head, err := h.execGit(r.Context(), containerID, 10*time.Second, "rev-parse", "--verify", "-q", "HEAD")
	if err != nil {
		jsonError(w, err.Error(), http.StatusBadGateway)
		return
	}
	args := append([]string{"restore", "--staged", "--"}, rels...)
	if head.ExitCode != 0 {
		// Before the first commit there is no HEAD to restore the index from.
		args = append([]string{"rm", "--cached", "-r", "-q", "--"}, rels...)
	}
	h.runGitSteps(w, r, containerID, [][]string{args}, "git unstage failed")
}

func (h codeStudioHandlers) handleGitDiscard(w http.ResponseWriter, r *http.Request) {
	rels, containerID, ok := h.gitPathsRequest(w, r)
	if !ok {
		return
	}
	// The server decides itself which paths are untracked: only those are deleted with
	// git clean; every other path is restored from the index.
	statusArgs := append([]string{"status", "--porcelain=v1", "-z", "--"}, rels...)
	statusResult, err := h.execGit(r.Context(), containerID, 15*time.Second, statusArgs...)
	if err != nil {
		jsonError(w, err.Error(), http.StatusBadGateway)
		return
	}
	if statusResult.ExitCode != 0 {
		jsonError(w, codeStudioGitExecError(statusResult.Output, "git status failed"), http.StatusBadRequest)
		return
	}
	untracked := map[string]bool{}
	for _, change := range parseGitPorcelain(statusResult.Output) {
		if change["status"] == "??" {
			untracked[strings.TrimSuffix(change["path"], "/")] = strings.HasSuffix(change["path"], "/")
		}
	}
	var files, dirs, tracked []string
	for _, rel := range rels {
		isDir, isUntracked := untracked[rel]
		switch {
		case isUntracked && isDir:
			dirs = append(dirs, rel)
		case isUntracked:
			files = append(files, rel)
		default:
			tracked = append(tracked, rel)
		}
	}
	steps := [][]string{}
	if len(files) > 0 {
		steps = append(steps, append([]string{"clean", "-f", "-q", "--"}, files...))
	}
	if len(dirs) > 0 {
		steps = append(steps, append([]string{"clean", "-f", "-q", "-d", "--"}, dirs...))
	}
	if len(tracked) > 0 {
		steps = append(steps, append([]string{"restore", "--worktree", "--"}, tracked...))
	}
	h.runGitSteps(w, r, containerID, steps, "git discard failed")
}

func (h codeStudioHandlers) handleGitInit(w http.ResponseWriter, r *http.Request) {
	if !h.requirePermission(w, r, desktopScopeWrite) {
		return
	}
	if h.rejectReadOnly(w) {
		return
	}
	if r.Method != http.MethodPost {
		jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	_, containerID, err := h.codeContainer(r.Context(), true)
	if err != nil {
		jsonError(w, err.Error(), http.StatusServiceUnavailable)
		return
	}
	probe, err := h.execGit(r.Context(), containerID, 10*time.Second, "rev-parse", "--is-inside-work-tree")
	if err != nil {
		jsonError(w, err.Error(), http.StatusBadGateway)
		return
	}
	if probe.ExitCode == 0 {
		jsonError(w, "workspace is already a git repository", http.StatusConflict)
		return
	}
	if !codeStudioGitNotRepository(probe.Output) {
		jsonError(w, codeStudioGitExecError(probe.Output, "git rev-parse failed"), http.StatusBadRequest)
		return
	}
	h.runGitSteps(w, r, containerID, [][]string{{"init"}}, "git init failed")
}
```

- [ ] **Step 4: Register the routes.** In `internal/server/code_studio_handlers.go`:

Replace:
```go
	mux.HandleFunc("/api/code-studio/git/log", handlers.handleGitLog)
```
With:
```go
	mux.HandleFunc("/api/code-studio/git/log", handlers.handleGitLog)
	mux.HandleFunc("/api/code-studio/git/stage", handlers.handleGitStage)
	mux.HandleFunc("/api/code-studio/git/unstage", handlers.handleGitUnstage)
	mux.HandleFunc("/api/code-studio/git/discard", handlers.handleGitDiscard)
	mux.HandleFunc("/api/code-studio/git/init", handlers.handleGitInit)
```

- [ ] **Step 5: Append the tests** to the end of `internal/server/code_studio_git_workbench_test.go`:

```go

func TestCodeStudioGitStageAndUnstageCommands(t *testing.T) {
	s := testCodeStudioServerWithFakeCodeContainer(t, 1)
	docker := &recordingCodeStudioDockerAPI{}
	h := codeStudioHandlers{server: s, docker: docker}
	post := func(handler http.HandlerFunc, route, body string) *httptest.ResponseRecorder {
		t.Helper()
		rec := httptest.NewRecorder()
		handler(rec, httptest.NewRequest(http.MethodPost, route, strings.NewReader(body)))
		return rec
	}

	rec := post(h.handleGitStage, "/api/code-studio/git/stage", `{"paths":["main.go","/workspace/sub/a b.txt","main.go","-n"]}`)
	if rec.Code != http.StatusOK || !reflect.DeepEqual(docker.commands, [][]string{codeStudioGitCmd("add", "--", "main.go", "sub/a b.txt", "-n")}) {
		t.Fatalf("stage status=%d commands=%q body=%s", rec.Code, docker.commands, rec.Body.String())
	}
	for _, body := range []string{`{"paths":[]}`, `{"paths":["../etc/passwd"]}`, `{"paths":["/etc/passwd"]}`, `{"paths":["/workspace"]}`, `not json`} {
		docker = &recordingCodeStudioDockerAPI{}
		h.docker = docker
		if rec := post(h.handleGitStage, "/api/code-studio/git/stage", body); rec.Code != http.StatusBadRequest || len(docker.commands) != 0 {
			t.Fatalf("stage %s: status=%d commands=%q", body, rec.Code, docker.commands)
		}
	}
	rec = httptest.NewRecorder()
	h.handleGitStage(rec, httptest.NewRequest(http.MethodGet, "/api/code-studio/git/stage", nil))
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("GET stage status=%d", rec.Code)
	}

	docker = &recordingCodeStudioDockerAPI{results: []codeStudioExecResult{{Output: "0123456789abcdef0123456789abcdef01234567\n"}}}
	h.docker = docker
	if rec := post(h.handleGitUnstage, "/api/code-studio/git/unstage", `{"paths":["main.go"]}`); rec.Code != http.StatusOK || !reflect.DeepEqual(docker.commands, [][]string{
		codeStudioGitCmd("rev-parse", "--verify", "-q", "HEAD"),
		codeStudioGitCmd("restore", "--staged", "--", "main.go"),
	}) {
		t.Fatalf("unstage status=%d commands=%q", rec.Code, docker.commands)
	}

	docker = &recordingCodeStudioDockerAPI{results: []codeStudioExecResult{{ExitCode: 1}}}
	h.docker = docker
	if rec := post(h.handleGitUnstage, "/api/code-studio/git/unstage", `{"paths":["main.go"]}`); rec.Code != http.StatusOK || !reflect.DeepEqual(docker.commands, [][]string{
		codeStudioGitCmd("rev-parse", "--verify", "-q", "HEAD"),
		codeStudioGitCmd("rm", "--cached", "-r", "-q", "--", "main.go"),
	}) {
		t.Fatalf("unstage without HEAD status=%d commands=%q", rec.Code, docker.commands)
	}
}

func TestCodeStudioGitDiscardSplitsUntrackedFromTracked(t *testing.T) {
	s := testCodeStudioServerWithFakeCodeContainer(t, 1)
	docker := &recordingCodeStudioDockerAPI{results: []codeStudioExecResult{{Output: " M main.go\x00?? new.txt\x00?? build/\x00"}}}
	h := codeStudioHandlers{server: s, docker: docker}
	rec := httptest.NewRecorder()
	h.handleGitDiscard(rec, httptest.NewRequest(http.MethodPost, "/api/code-studio/git/discard", strings.NewReader(`{"paths":["main.go","new.txt","build/"]}`)))
	want := [][]string{
		codeStudioGitCmd("status", "--porcelain=v1", "-z", "--", "main.go", "new.txt", "build"),
		codeStudioGitCmd("clean", "-f", "-q", "--", "new.txt"),
		codeStudioGitCmd("clean", "-f", "-q", "-d", "--", "build"),
		codeStudioGitCmd("restore", "--worktree", "--", "main.go"),
	}
	if rec.Code != http.StatusOK || !reflect.DeepEqual(docker.commands, want) {
		t.Fatalf("discard status=%d commands=%q body=%s", rec.Code, docker.commands, rec.Body.String())
	}

	for _, body := range []string{`{"paths":["/workspace"]}`, `{"paths":["."]}`, `{"paths":["../outside.txt"]}`} {
		docker = &recordingCodeStudioDockerAPI{}
		h.docker = docker
		rec = httptest.NewRecorder()
		h.handleGitDiscard(rec, httptest.NewRequest(http.MethodPost, "/api/code-studio/git/discard", strings.NewReader(body)))
		if rec.Code != http.StatusBadRequest || len(docker.commands) != 0 {
			t.Fatalf("discard %s: status=%d commands=%q", body, rec.Code, docker.commands)
		}
	}

	docker = &recordingCodeStudioDockerAPI{results: []codeStudioExecResult{{Output: " M main.go\x00"}, {ExitCode: 1, Output: "error: pathspec 'main.go' did not match\n"}}}
	h.docker = docker
	rec = httptest.NewRecorder()
	h.handleGitDiscard(rec, httptest.NewRequest(http.MethodPost, "/api/code-studio/git/discard", strings.NewReader(`{"paths":["main.go"]}`)))
	if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), "did not match") {
		t.Fatalf("git error must be returned verbatim: status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestCodeStudioGitInitOnlyOutsideRepository(t *testing.T) {
	s := testCodeStudioServerWithFakeCodeContainer(t, 1)
	probe := codeStudioGitCmd("rev-parse", "--is-inside-work-tree")
	cases := []struct {
		name     string
		result   codeStudioExecResult
		wantCode int
		wantCmds [][]string
	}{
		{"already a repository", codeStudioExecResult{Output: "true\n"}, http.StatusConflict, [][]string{probe}},
		{"no repository", codeStudioExecResult{ExitCode: 128, Output: "fatal: not a git repository (or any of the parent directories): .git\n"}, http.StatusOK, [][]string{probe, codeStudioGitCmd("init")}},
		{"other failure", codeStudioExecResult{ExitCode: 128, Output: "fatal: detected dubious ownership in repository at '/workspace'\n"}, http.StatusBadRequest, [][]string{probe}},
	}
	for _, tc := range cases {
		docker := &recordingCodeStudioDockerAPI{results: []codeStudioExecResult{tc.result}}
		h := codeStudioHandlers{server: s, docker: docker}
		rec := httptest.NewRecorder()
		h.handleGitInit(rec, httptest.NewRequest(http.MethodPost, "/api/code-studio/git/init", nil))
		if rec.Code != tc.wantCode || !reflect.DeepEqual(docker.commands, tc.wantCmds) {
			t.Fatalf("%s: status=%d commands=%q body=%s", tc.name, rec.Code, docker.commands, rec.Body.String())
		}
	}
}

func TestCodeStudioGitWorkbenchWritesRespectReadOnly(t *testing.T) {
	s := testCodeStudioServerWithFakeCodeContainer(t, 1)
	s.CfgMu.Lock()
	s.Cfg.VirtualDesktop.ReadOnly = true
	s.CfgMu.Unlock()
	docker := &recordingCodeStudioDockerAPI{}
	h := codeStudioHandlers{server: s, docker: docker}
	for route, handler := range map[string]http.HandlerFunc{
		"stage": h.handleGitStage, "unstage": h.handleGitUnstage, "discard": h.handleGitDiscard, "init": h.handleGitInit,
	} {
		rec := httptest.NewRecorder()
		handler(rec, httptest.NewRequest(http.MethodPost, "/api/code-studio/git/"+route, strings.NewReader(`{"paths":["main.go"]}`)))
		if rec.Code != http.StatusForbidden {
			t.Fatalf("read-only %s: status=%d body=%s", route, rec.Code, rec.Body.String())
		}
	}
	if len(docker.commands) != 0 {
		t.Fatalf("read-only writes ran git: %q", docker.commands)
	}
}
```

- [ ] **Step 6: Format and test.**

Run: `gofmt -l internal/server/code_studio_git_workbench.go internal/server/code_studio_git_workbench_test.go internal/server/code_studio_handlers.go`
Expected: no output.

Run: `go test ./internal/server -count=1 -run "(?i)(CodeStudioGit|CodeStudioReadonly)"`
Expected: PASS, including `TestCodeStudioGitStageAndUnstageCommands`, `TestCodeStudioGitDiscardSplitsUntrackedFromTracked`, `TestCodeStudioGitInitOnlyOutsideRepository` and `TestCodeStudioGitWorkbenchWritesRespectReadOnly`.

Commit: `feat(code-studio-api): stage, unstage, discard and init endpoints`. Files: `internal/server/code_studio_git_workbench.go`, `internal/server/code_studio_git_workbench_test.go`, `internal/server/code_studio_handlers.go`

---

### Task 3: Commit view and blame endpoints (backend)

**Files:**
- Modify: `internal/server/code_studio_git_workbench.go`
- Modify: `internal/server/code_studio_handlers.go` (routes)
- Modify: `internal/server/code_studio_git_workbench_test.go`

- [ ] **Step 1: Append the read handlers** to the end of `internal/server/code_studio_git_workbench.go`:

```go

func (h codeStudioHandlers) handleGitShow(w http.ResponseWriter, r *http.Request) {
	if !h.requirePermission(w, r, desktopScopeRead) {
		return
	}
	if r.Method != http.MethodGet {
		jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	hash := strings.TrimSpace(r.URL.Query().Get("hash"))
	if !codeStudioGitHashPattern.MatchString(hash) {
		jsonError(w, "invalid commit hash", http.StatusBadRequest)
		return
	}
	_, containerID, err := h.codeContainer(r.Context(), true)
	if err != nil {
		jsonError(w, err.Error(), http.StatusServiceUnavailable)
		return
	}
	// The hash holds hex digits only, so it can never be read as an option; "--" ends the revisions.
	metaResult, err := h.execGit(r.Context(), containerID, 15*time.Second, "-c", "core.quotePath=false", "show", "--no-color", "--name-status", codeStudioGitShowFormat, hash, "--")
	if err != nil {
		jsonError(w, err.Error(), http.StatusBadGateway)
		return
	}
	if metaResult.ExitCode != 0 {
		jsonError(w, codeStudioGitExecError(metaResult.Output, "git show failed"), http.StatusBadRequest)
		return
	}
	commit, err := parseGitShowMeta(metaResult.Output)
	if err != nil {
		jsonError(w, err.Error(), http.StatusBadGateway)
		return
	}
	patchResult, err := h.execGit(r.Context(), containerID, 30*time.Second, "show", "--no-color", "--format=", "--patch", hash, "--")
	if err != nil {
		jsonError(w, err.Error(), http.StatusBadGateway)
		return
	}
	if patchResult.ExitCode != 0 {
		jsonError(w, codeStudioGitExecError(patchResult.Output, "git show failed"), http.StatusBadRequest)
		return
	}
	diff, truncated := truncateCodeStudioGitDiff(strings.TrimLeft(patchResult.Output, "\n"), codeStudioGitShowDiffLimit)
	writeJSON(w, map[string]interface{}{
		"status":    "ok",
		"hash":      commit.Hash,
		"short":     commit.Short,
		"author":    commit.Author,
		"email":     commit.Email,
		"date":      commit.Date,
		"subject":   commit.Subject,
		"body":      commit.Body,
		"files":     commit.Files,
		"diff":      diff,
		"truncated": truncated,
	})
}

func (h codeStudioHandlers) handleGitBlame(w http.ResponseWriter, r *http.Request) {
	if !h.requirePermission(w, r, desktopScopeRead) {
		return
	}
	if r.Method != http.MethodGet {
		jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	rel, err := codeStudioGitAddPath(r.URL.Query().Get("path"))
	if err != nil {
		jsonError(w, err.Error(), http.StatusBadRequest)
		return
	}
	_, containerID, err := h.codeContainer(r.Context(), true)
	if err != nil {
		jsonError(w, err.Error(), http.StatusServiceUnavailable)
		return
	}
	result, err := h.execGit(r.Context(), containerID, 30*time.Second, "blame", "--porcelain", "--", rel)
	if err != nil {
		jsonError(w, err.Error(), http.StatusBadGateway)
		return
	}
	if result.ExitCode != 0 {
		jsonError(w, codeStudioGitExecError(result.Output, "git blame failed"), http.StatusBadRequest)
		return
	}
	writeJSON(w, map[string]interface{}{"status": "ok", "path": rel, "lines": parseGitBlamePorcelain(result.Output)})
}
```

- [ ] **Step 2: Register the routes.** In `internal/server/code_studio_handlers.go`:

Replace:
```go
	mux.HandleFunc("/api/code-studio/git/init", handlers.handleGitInit)
```
With:
```go
	mux.HandleFunc("/api/code-studio/git/init", handlers.handleGitInit)
	mux.HandleFunc("/api/code-studio/git/show", handlers.handleGitShow)
	mux.HandleFunc("/api/code-studio/git/blame", handlers.handleGitBlame)
```

- [ ] **Step 3: Append the tests** to the end of `internal/server/code_studio_git_workbench_test.go`:

```go

func TestCodeStudioGitShowHandler(t *testing.T) {
	s := testCodeStudioServerWithFakeCodeContainer(t, 1)
	for _, query := range []string{"", "abc", "HEAD", "-p", "zzzz", "abcd%3Bid", "0123456789abcdef0123456789abcdef012345678"} {
		docker := &recordingCodeStudioDockerAPI{}
		h := codeStudioHandlers{server: s, docker: docker}
		rec := httptest.NewRecorder()
		h.handleGitShow(rec, httptest.NewRequest(http.MethodGet, "/api/code-studio/git/show?hash="+query, nil))
		if rec.Code != http.StatusBadRequest || len(docker.commands) != 0 {
			t.Fatalf("hash %q: status=%d commands=%q", query, rec.Code, docker.commands)
		}
	}

	full := "0123456789abcdef0123456789abcdef01234567"
	meta := full + "\x1f0123456\x1fAda Lovelace\x1fada@example.com\x1f2026-11-20T10:00:00+01:00\x1fAdd engine\x1fLonger body\n\x1e\nM\tmain.go\n"
	docker := &recordingCodeStudioDockerAPI{results: []codeStudioExecResult{{Output: meta}, {Output: "\ndiff --git a/main.go b/main.go\n+new\n"}}}
	h := codeStudioHandlers{server: s, docker: docker}
	rec := httptest.NewRecorder()
	h.handleGitShow(rec, httptest.NewRequest(http.MethodGet, "/api/code-studio/git/show?hash=0123456", nil))
	wantCommands := [][]string{
		codeStudioGitCmd("-c", "core.quotePath=false", "show", "--no-color", "--name-status", codeStudioGitShowFormat, "0123456", "--"),
		codeStudioGitCmd("show", "--no-color", "--format=", "--patch", "0123456", "--"),
	}
	if rec.Code != http.StatusOK || !reflect.DeepEqual(docker.commands, wantCommands) {
		t.Fatalf("show status=%d commands=%q body=%s", rec.Code, docker.commands, rec.Body.String())
	}
	var body struct {
		Hash      string
		Short     string
		Email     string
		Subject   string
		Body      string
		Diff      string
		Files     []codeStudioGitFileChange
		Truncated bool
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Hash != full || body.Short != "0123456" || body.Email != "ada@example.com" || body.Subject != "Add engine" || body.Body != "Longer body" ||
		body.Diff != "diff --git a/main.go b/main.go\n+new\n" || body.Truncated || !reflect.DeepEqual(body.Files, []codeStudioGitFileChange{{Path: "main.go", Status: "M"}}) {
		t.Fatalf("show body=%+v", body)
	}

	docker = &recordingCodeStudioDockerAPI{results: []codeStudioExecResult{{ExitCode: 128, Output: "fatal: bad object 0123456\n"}}}
	h.docker = docker
	rec = httptest.NewRecorder()
	h.handleGitShow(rec, httptest.NewRequest(http.MethodGet, "/api/code-studio/git/show?hash=0123456", nil))
	if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), "bad object") || len(docker.commands) != 1 {
		t.Fatalf("unknown commit: status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestCodeStudioGitBlameHandler(t *testing.T) {
	s := testCodeStudioServerWithFakeCodeContainer(t, 1)
	for _, query := range []string{"", "../etc/passwd", "/etc/passwd", "/workspace"} {
		docker := &recordingCodeStudioDockerAPI{}
		h := codeStudioHandlers{server: s, docker: docker}
		rec := httptest.NewRecorder()
		h.handleGitBlame(rec, httptest.NewRequest(http.MethodGet, "/api/code-studio/git/blame?path="+query, nil))
		if rec.Code != http.StatusBadRequest || len(docker.commands) != 0 {
			t.Fatalf("blame path %q: status=%d commands=%q", query, rec.Code, docker.commands)
		}
	}

	commit := strings.Repeat("b", 40)
	porcelain := commit + " 1 1 1\nauthor Ada Lovelace\nauthor-time 1700000000\nauthor-tz +0000\nsummary First commit\nfilename src/app.js\n\tline one\n"
	docker := &recordingCodeStudioDockerAPI{results: []codeStudioExecResult{{Output: porcelain}}}
	h := codeStudioHandlers{server: s, docker: docker}
	rec := httptest.NewRecorder()
	h.handleGitBlame(rec, httptest.NewRequest(http.MethodGet, "/api/code-studio/git/blame?path=/workspace/src/app.js", nil))
	if rec.Code != http.StatusOK || !reflect.DeepEqual(docker.commands, [][]string{codeStudioGitCmd("blame", "--porcelain", "--", "src/app.js")}) {
		t.Fatalf("blame status=%d commands=%q body=%s", rec.Code, docker.commands, rec.Body.String())
	}
	var body struct{ Lines []codeStudioGitBlameLine }
	want := []codeStudioGitBlameLine{{Line: 1, Hash: commit, Short: "bbbbbbb", Author: "Ada Lovelace", Date: "2023-11-14T22:13:20Z", Summary: "First commit"}}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil || !reflect.DeepEqual(body.Lines, want) {
		t.Fatalf("blame body=%s err=%v", rec.Body.String(), err)
	}

	docker = &recordingCodeStudioDockerAPI{results: []codeStudioExecResult{{ExitCode: 128, Output: "fatal: no such path 'src/new.js' in HEAD\n"}}}
	h.docker = docker
	rec = httptest.NewRecorder()
	h.handleGitBlame(rec, httptest.NewRequest(http.MethodGet, "/api/code-studio/git/blame?path=src/new.js", nil))
	if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), "no such path") {
		t.Fatalf("blame failure: status=%d body=%s", rec.Code, rec.Body.String())
	}
}
```

- [ ] **Step 4: Format and test.**

Run: `gofmt -l internal/server/code_studio_git_workbench.go internal/server/code_studio_git_workbench_test.go internal/server/code_studio_handlers.go`
Expected: no output.

Run: `go test ./internal/server -count=1 -run "(?i)(CodeStudioGit|CodeStudioReadonly)"`
Expected: PASS, including `TestCodeStudioGitShowHandler` and `TestCodeStudioGitBlameHandler`.

Commit: `feat(code-studio-api): commit view and blame endpoints`. Files: `internal/server/code_studio_git_workbench.go`, `internal/server/code_studio_git_workbench_test.go`, `internal/server/code_studio_handlers.go`

---

### Task 4: Staged-only commits and branch delete (backend)

**Files:**
- Modify: `internal/server/code_studio_handlers.go`
- Modify: `internal/server/code_studio_git_workbench_test.go`

- [ ] **Step 1: Impact check.** Run `impact` upstream on `handleGitCommit`, `handleGitBranch` and `mutateGitBranch` and report the risk. Known callers are `git.js` (through fetch) and the tests in `code_studio_git_commit_test.go` and `code_studio_security_test.go`.

- [ ] **Step 2: Add the commit body field.** In `internal/server/code_studio_handlers.go`:

Replace:
```go
	var body struct {
		Message string   `json:"message"`
		AddAll  bool     `json:"add_all"`
		Paths   []string `json:"paths"`
	}
```
With:
```go
	var body struct {
		Message    string   `json:"message"`
		AddAll     bool     `json:"add_all"`
		StagedOnly bool     `json:"staged_only"`
		Paths      []string `json:"paths"`
	}
```

- [ ] **Step 3: Skip `git add` for staged-only commits.** In `internal/server/code_studio_handlers.go`:

Replace:
```go
	rels := []string{}
	if !body.AddAll {
		for _, raw := range body.Paths {
```
With:
```go
	if body.AddAll && body.StagedOnly {
		jsonError(w, "add_all and staged_only exclude each other", http.StatusBadRequest)
		return
	}
	rels := []string{}
	// staged_only commits the index as it is: no path is added first.
	if !body.AddAll && !body.StagedOnly {
		for _, raw := range body.Paths {
```

- [ ] **Step 4: GET never deletes.** In `internal/server/code_studio_handlers.go`:

Replace:
```go
		if action == "create" || action == "switch" {
```
With:
```go
		if action == "create" || action == "switch" || action == "delete" {
```

- [ ] **Step 5: Accept the `delete` action.** In `internal/server/code_studio_handlers.go`:

Replace:
```go
	if action != "create" && action != "switch" {
		jsonError(w, "action must be create or switch", http.StatusBadRequest)
		return
	}
```
With:
```go
	if action != "create" && action != "switch" && action != "delete" {
		jsonError(w, "action must be create, switch or delete", http.StatusBadRequest)
		return
	}
```

- [ ] **Step 6: Run the delete action.** In `internal/server/code_studio_handlers.go`:

Replace:
```go
	args := []string{"checkout"}
	if action == "create" {
		args = append(args, "-b")
	}
	args = append(args, name)
	result, err := h.execGit(r.Context(), containerID, 15*time.Second, args...)
	if err != nil {
		jsonError(w, err.Error(), http.StatusBadGateway)
		return
	}
	if result.ExitCode != 0 {
		jsonError(w, codeStudioGitExecError(result.Output, "git checkout failed"), http.StatusBadRequest)
		return
	}
	h.listGitBranches(w, r)
```
With:
```go
	args := []string{"checkout", name}
	fallback := "git checkout failed"
	switch action {
	case "create":
		args = []string{"checkout", "-b", name}
	case "delete":
		current, err := h.execGit(r.Context(), containerID, 5*time.Second, "branch", "--show-current")
		if err != nil {
			jsonError(w, err.Error(), http.StatusBadGateway)
			return
		}
		if current.ExitCode == 0 && strings.TrimSpace(current.Output) == name {
			jsonError(w, "the current branch cannot be deleted", http.StatusConflict)
			return
		}
		// -d (not -D) lets git refuse a branch whose commits are not merged.
		args = []string{"branch", "-d", name}
		fallback = "git branch -d failed"
	}
	result, err := h.execGit(r.Context(), containerID, 15*time.Second, args...)
	if err != nil {
		jsonError(w, err.Error(), http.StatusBadGateway)
		return
	}
	if result.ExitCode != 0 {
		jsonError(w, codeStudioGitExecError(result.Output, fallback), http.StatusBadRequest)
		return
	}
	h.listGitBranches(w, r)
```

- [ ] **Step 7: Append the tests** to the end of `internal/server/code_studio_git_workbench_test.go`:

```go

func TestCodeStudioGitCommitStagedOnlyAndBranchDelete(t *testing.T) {
	s := testCodeStudioServerWithFakeCodeContainer(t, 1)
	docker := &recordingCodeStudioDockerAPI{results: []codeStudioExecResult{{}, {Output: "abc1234\n"}}}
	h := codeStudioHandlers{server: s, docker: docker}
	rec := httptest.NewRecorder()
	h.handleGitCommit(rec, httptest.NewRequest(http.MethodPost, "/api/code-studio/git/commit", strings.NewReader(`{"message":"index only","staged_only":true,"paths":["main.go"]}`)))
	if rec.Code != http.StatusOK || !reflect.DeepEqual(docker.commands, [][]string{
		codeStudioGitCmd("commit", "-m", "index only"),
		codeStudioGitCmd("rev-parse", "--short", "HEAD"),
	}) {
		t.Fatalf("staged-only commit status=%d commands=%q", rec.Code, docker.commands)
	}

	docker = &recordingCodeStudioDockerAPI{}
	h.docker = docker
	rec = httptest.NewRecorder()
	h.handleGitCommit(rec, httptest.NewRequest(http.MethodPost, "/api/code-studio/git/commit", strings.NewReader(`{"message":"both","staged_only":true,"add_all":true}`)))
	if rec.Code != http.StatusBadRequest || len(docker.commands) != 0 {
		t.Fatalf("staged_only with add_all: status=%d commands=%q", rec.Code, docker.commands)
	}

	docker = &recordingCodeStudioDockerAPI{results: []codeStudioExecResult{{Output: "main\n"}, {}, {Output: "* main\n"}, {Output: "main\n"}}}
	h.docker = docker
	rec = httptest.NewRecorder()
	h.handleGitBranch(rec, httptest.NewRequest(http.MethodPost, "/api/code-studio/git/branch", strings.NewReader(`{"action":"delete","name":"feature/old"}`)))
	if rec.Code != http.StatusOK || len(docker.commands) < 2 || !reflect.DeepEqual(docker.commands[:2], [][]string{
		codeStudioGitCmd("branch", "--show-current"),
		codeStudioGitCmd("branch", "-d", "feature/old"),
	}) {
		t.Fatalf("branch delete status=%d commands=%q body=%s", rec.Code, docker.commands, rec.Body.String())
	}

	docker = &recordingCodeStudioDockerAPI{results: []codeStudioExecResult{{Output: "main\n"}}}
	h.docker = docker
	rec = httptest.NewRecorder()
	h.handleGitBranch(rec, httptest.NewRequest(http.MethodPost, "/api/code-studio/git/branch", strings.NewReader(`{"action":"delete","name":"main"}`)))
	if rec.Code != http.StatusConflict || len(docker.commands) != 1 {
		t.Fatalf("current branch delete: status=%d commands=%q", rec.Code, docker.commands)
	}

	docker = &recordingCodeStudioDockerAPI{}
	h.docker = docker
	rec = httptest.NewRecorder()
	h.handleGitBranch(rec, httptest.NewRequest(http.MethodGet, "/api/code-studio/git/branch?action=delete&name=feature/old", nil))
	if rec.Code != http.StatusMethodNotAllowed || len(docker.commands) != 0 {
		t.Fatalf("GET delete: status=%d commands=%q", rec.Code, docker.commands)
	}
}
```

- [ ] **Step 8: Format and test.**

Run: `gofmt -l internal/server/code_studio_handlers.go internal/server/code_studio_git_workbench_test.go`
Expected: no output.

Run: `go test ./internal/server -count=1 -run "(?i)(CodeStudioGit|CodeStudioReadonly)"`
Expected: PASS, including the existing `TestCodeStudioGitBranchGetDoesNotMutate`, `TestCodeStudioGitCommitStagesOnlyGivenPaths` and `TestCodeStudioReadonlyBlocksMutatingHandlers`, and the new `TestCodeStudioGitCommitStagedOnlyAndBranchDelete`.

Commit: `feat(code-studio-api): staged-only commits and branch delete`. Files: `internal/server/code_studio_handlers.go`, `internal/server/code_studio_git_workbench_test.go`

---

### Task 5: Git workbench strings in 16 languages

**Files:**
- Create (not committed): `C:\disposable\code-studio-pkg1\locales-b.mjs`
- Modify: `ui/lang/desktop/{cs,da,de,el,en,es,fr,hi,it,ja,nl,no,pl,pt,sv,zh}.json`

The locale files are flat JSON with two-space indentation and one entry per line. They are not sorted; for example, `codeStudio.diffWorktree` sits near the end of `en.json`. The script updates existing keys in place and appends new keys before the closing brace. It keeps the line endings, the indentation and any byte order mark.

- [ ] **Step 1: Create `C:\disposable\code-studio-pkg1\locales-b.mjs`** with this complete content:

```js
// Code Studio IDE package 1B: Git workbench strings for the 16 desktop locales.
// Usage: node C:\disposable\code-studio-pkg1\locales-b.mjs C:\Users\Andi\Documents\repo\AuraGo
// Existing keys are updated in place; new keys are appended before the closing brace.
// Line endings, indentation and a byte order mark are kept. Running it twice changes nothing.
import { readFileSync, writeFileSync } from 'node:fs';
import { join } from 'node:path';

const root = process.argv[2];
if (!root) throw new Error('usage: node locales-b.mjs <repo root>');
const BOM = String.fromCodePoint(0xfeff);
const LANGS = ['cs', 'da', 'de', 'el', 'en', 'es', 'fr', 'hi', 'it', 'ja', 'nl', 'no', 'pl', 'pt', 'sv', 'zh'];

const STRINGS = {
  'codeStudio.gitInitRepo': {
    en: "Initialize Repository", cs: "Inicializovat repozitář", da: "Initialiser repository", de: "Repository initialisieren",
    el: "Αρχικοποίηση αποθετηρίου", es: "Inicializar repositorio", fr: "Initialiser le dépôt", hi: "रिपॉज़िटरी आरंभ करें",
    it: "Inizializza repository", ja: "リポジトリを初期化", nl: "Repository initialiseren", no: "Initialiser repository",
    pl: "Zainicjuj repozytorium", pt: "Inicializar repositório", sv: "Initiera repository", zh: "初始化仓库"
  },
  'codeStudio.gitBranchMenu': {
    en: "Switch, create or delete branches", cs: "Přepnout, vytvořit nebo smazat větve", da: "Skift, opret eller slet grene", de: "Branches wechseln, erstellen oder löschen",
    el: "Εναλλαγή, δημιουργία ή διαγραφή κλάδων", es: "Cambiar, crear o eliminar ramas", fr: "Changer, créer ou supprimer des branches", hi: "ब्रांच बदलें, बनाएँ या हटाएँ",
    it: "Cambia, crea o elimina rami", ja: "ブランチの切り替え・作成・削除", nl: "Branches wisselen, maken of verwijderen", no: "Bytt, opprett eller slett grener",
    pl: "Przełącz, utwórz lub usuń gałęzie", pt: "Trocar, criar ou excluir branches", sv: "Byt, skapa eller ta bort grenar", zh: "切换、创建或删除分支"
  },
  'codeStudio.gitDetached': {
    en: "Detached HEAD", cs: "Odpojený HEAD", da: "Frakoblet HEAD", de: "Losgelöster HEAD",
    el: "Αποσυνδεδεμένο HEAD", es: "HEAD separado", fr: "HEAD détachée", hi: "अलग किया गया HEAD",
    it: "HEAD scollegato", ja: "デタッチされた HEAD", nl: "Losgekoppelde HEAD", no: "Frakoblet HEAD",
    pl: "Odłączony HEAD", pt: "HEAD desanexado", sv: "Frånkopplad HEAD", zh: "分离的 HEAD"
  },
  'codeStudio.gitStagedChanges': {
    en: "Staged Changes", cs: "Připravené změny", da: "Klargjorte ændringer", de: "Bereitgestellte Änderungen",
    el: "Προετοιμασμένες αλλαγές", es: "Cambios preparados", fr: "Modifications indexées", hi: "स्टेज किए गए बदलाव",
    it: "Modifiche nell'area di stage", ja: "ステージ済みの変更", nl: "Gefaseerde wijzigingen", no: "Klargjorte endringer",
    pl: "Przygotowane zmiany", pt: "Alterações preparadas", sv: "Mellanlagrade ändringar", zh: "暂存的更改"
  },
  'codeStudio.gitStageAll': {
    en: "Stage All Changes", cs: "Připravit všechny změny", da: "Klargør alle ændringer", de: "Alle Änderungen bereitstellen",
    el: "Προετοιμασία όλων των αλλαγών", es: "Preparar todos los cambios", fr: "Indexer toutes les modifications", hi: "सभी बदलाव स्टेज करें",
    it: "Aggiungi tutte le modifiche allo stage", ja: "すべての変更をステージ", nl: "Alle wijzigingen faseren", no: "Klargjør alle endringer",
    pl: "Przygotuj wszystkie zmiany", pt: "Preparar todas as alterações", sv: "Mellanlagra alla ändringar", zh: "暂存所有更改"
  },
  'codeStudio.gitUnstageAll': {
    en: "Unstage All Changes", cs: "Zrušit přípravu všech změn", da: "Fjern alle ændringer fra klargøring", de: "Bereitstellung aller Änderungen aufheben",
    el: "Αναίρεση προετοιμασίας όλων των αλλαγών", es: "Quitar todos los cambios del área de preparación", fr: "Retirer toutes les modifications de l'index", hi: "सभी बदलाव अनस्टेज करें",
    it: "Rimuovi tutte le modifiche dallo stage", ja: "すべての変更のステージを解除", nl: "Fasering van alle wijzigingen ongedaan maken", no: "Fjern alle endringer fra klargjøring",
    pl: "Cofnij przygotowanie wszystkich zmian", pt: "Remover todas as alterações da preparação", sv: "Ta bort alla ändringar från mellanlagringen", zh: "取消暂存所有更改"
  },
  'codeStudio.gitStage': {
    en: "Stage Changes", cs: "Připravit změny", da: "Klargør ændringer", de: "Änderungen bereitstellen",
    el: "Προετοιμασία αλλαγών", es: "Preparar cambios", fr: "Indexer les modifications", hi: "बदलाव स्टेज करें",
    it: "Aggiungi allo stage", ja: "変更をステージ", nl: "Wijzigingen faseren", no: "Klargjør endringer",
    pl: "Przygotuj zmiany", pt: "Preparar alterações", sv: "Mellanlagra ändringar", zh: "暂存更改"
  },
  'codeStudio.gitUnstage': {
    en: "Unstage Changes", cs: "Zrušit přípravu změn", da: "Fjern fra klargøring", de: "Bereitstellung aufheben",
    el: "Αναίρεση προετοιμασίας", es: "Quitar del área de preparación", fr: "Retirer de l'index", hi: "बदलाव अनस्टेज करें",
    it: "Rimuovi dallo stage", ja: "ステージを解除", nl: "Fasering ongedaan maken", no: "Fjern fra klargjøring",
    pl: "Cofnij przygotowanie", pt: "Remover da preparação", sv: "Ta bort från mellanlagringen", zh: "取消暂存更改"
  },
  'codeStudio.gitStageAllAndCommit': {
    en: "Stage all changes and commit?", cs: "Připravit všechny změny a commitnout?", da: "Klargør alle ændringer og commit?", de: "Alle Änderungen bereitstellen und committen?",
    el: "Προετοιμασία όλων των αλλαγών και commit;", es: "¿Preparar todos los cambios y confirmar?", fr: "Indexer toutes les modifications et valider ?", hi: "सभी बदलाव स्टेज करके कमिट करें?",
    it: "Aggiungere tutte le modifiche allo stage ed eseguire il commit?", ja: "すべての変更をステージしてコミットしますか？", nl: "Alle wijzigingen faseren en committen?", no: "Klargjør alle endringer og commit?",
    pl: "Przygotować wszystkie zmiany i zatwierdzić?", pt: "Preparar todas as alterações e fazer commit?", sv: "Mellanlagra alla ändringar och checka in?", zh: "暂存所有更改并提交？"
  },
  'codeStudio.gitDiscardConfirm': {
    en: "Discard the changes in {{name}}? This cannot be undone.", cs: "Zahodit změny v {{name}}? Tuto akci nelze vrátit.", da: "Kassér ændringerne i {{name}}? Det kan ikke fortrydes.", de: "Änderungen in {{name}} verwerfen? Das lässt sich nicht rückgängig machen.",
    el: "Απόρριψη των αλλαγών στο {{name}}; Η ενέργεια δεν αναιρείται.", es: "¿Descartar los cambios en {{name}}? No se puede deshacer.", fr: "Annuler les modifications de {{name}} ? Cette action est irréversible.", hi: "{{name}} के बदलाव छोड़ दें? इसे पूर्ववत नहीं किया जा सकता।",
    it: "Annullare le modifiche in {{name}}? L'operazione non può essere annullata.", ja: "{{name}} の変更を破棄しますか？この操作は元に戻せません。", nl: "Wijzigingen in {{name}} verwerpen? Dit kan niet ongedaan worden gemaakt.", no: "Forkast endringene i {{name}}? Dette kan ikke angres.",
    pl: "Odrzucić zmiany w {{name}}? Tej operacji nie można cofnąć.", pt: "Descartar as alterações em {{name}}? Isso não pode ser desfeito.", sv: "Förkasta ändringarna i {{name}}? Det går inte att ångra.", zh: "放弃 {{name}} 中的更改？此操作无法撤消。"
  },
  'codeStudio.gitDiscardUntrackedConfirm': {
    en: "The untracked file {{name}} will be deleted. This cannot be undone.", cs: "Nesledovaný soubor {{name}} bude smazán. Tuto akci nelze vrátit.", da: "Den usporede fil {{name}} bliver slettet. Det kan ikke fortrydes.", de: "Die nicht verfolgte Datei {{name}} wird gelöscht. Das lässt sich nicht rückgängig machen.",
    el: "Το μη παρακολουθούμενο αρχείο {{name}} θα διαγραφεί. Η ενέργεια δεν αναιρείται.", es: "El archivo sin seguimiento {{name}} se eliminará. No se puede deshacer.", fr: "Le fichier non suivi {{name}} sera supprimé. Cette action est irréversible.", hi: "अनट्रैक की गई फ़ाइल {{name}} हटा दी जाएगी। इसे पूर्ववत नहीं किया जा सकता।",
    it: "Il file non tracciato {{name}} verrà eliminato. L'operazione non può essere annullata.", ja: "追跡されていないファイル {{name}} は削除されます。この操作は元に戻せません。", nl: "Het niet-gevolgde bestand {{name}} wordt verwijderd. Dit kan niet ongedaan worden gemaakt.", no: "Den usporede filen {{name}} blir slettet. Dette kan ikke angres.",
    pl: "Nieśledzony plik {{name}} zostanie usunięty. Tej operacji nie można cofnąć.", pt: "O arquivo não rastreado {{name}} será excluído. Isso não pode ser desfeito.", sv: "Den ospårade filen {{name}} tas bort. Det går inte att ångra.", zh: "未跟踪的文件 {{name}} 将被删除。此操作无法撤消。"
  },
  'codeStudio.gitInitialized': {
    en: "Initialized an empty Git repository", cs: "Inicializován prázdný repozitář Git", da: "Tomt Git-repository initialiseret", de: "Leeres Git-Repository initialisiert",
    el: "Αρχικοποιήθηκε κενό αποθετήριο Git", es: "Repositorio Git vacío inicializado", fr: "Dépôt Git vide initialisé", hi: "खाली Git रिपॉज़िटरी आरंभ की गई",
    it: "Repository Git vuoto inizializzato", ja: "空の Git リポジトリを初期化しました", nl: "Lege Git-repository geïnitialiseerd", no: "Tomt Git-repository initialisert",
    pl: "Zainicjowano puste repozytorium Git", pt: "Repositório Git vazio inicializado", sv: "Tomt Git-repository initierat", zh: "已初始化空的 Git 仓库"
  },
  'codeStudio.gitNewBranchName': {
    en: "New branch name", cs: "Název nové větve", da: "Navn på ny gren", de: "Name des neuen Branches",
    el: "Όνομα νέου κλάδου", es: "Nombre de la nueva rama", fr: "Nom de la nouvelle branche", hi: "नई ब्रांच का नाम",
    it: "Nome del nuovo ramo", ja: "新しいブランチ名", nl: "Naam van de nieuwe branch", no: "Navn på ny gren",
    pl: "Nazwa nowej gałęzi", pt: "Nome do novo branch", sv: "Namn på ny gren", zh: "新分支名称"
  },
  'codeStudio.gitCreateBranch': {
    en: "Create new branch...", cs: "Vytvořit novou větev...", da: "Opret ny gren...", de: "Neuen Branch erstellen...",
    el: "Δημιουργία νέου κλάδου...", es: "Crear nueva rama...", fr: "Créer une branche...", hi: "नई ब्रांच बनाएँ...",
    it: "Crea nuovo ramo...", ja: "新しいブランチを作成...", nl: "Nieuwe branch maken...", no: "Opprett ny gren...",
    pl: "Utwórz nową gałąź...", pt: "Criar novo branch...", sv: "Skapa ny gren...", zh: "创建新分支..."
  },
  'codeStudio.gitDeleteBranch': {
    en: "Delete branch {{name}}", cs: "Smazat větev {{name}}", da: "Slet grenen {{name}}", de: "Branch {{name}} löschen",
    el: "Διαγραφή κλάδου {{name}}", es: "Eliminar la rama {{name}}", fr: "Supprimer la branche {{name}}", hi: "ब्रांच {{name}} हटाएँ",
    it: "Elimina il ramo {{name}}", ja: "ブランチ {{name}} を削除", nl: "Branch {{name}} verwijderen", no: "Slett grenen {{name}}",
    pl: "Usuń gałąź {{name}}", pt: "Excluir o branch {{name}}", sv: "Ta bort grenen {{name}}", zh: "删除分支 {{name}}"
  },
  'codeStudio.gitDeleteBranchConfirm': {
    en: "Delete branch {{name}}? Git refuses to delete branches that are not merged.", cs: "Smazat větev {{name}}? Git odmítá mazat větve, které nejsou sloučené.", da: "Slet grenen {{name}}? Git nægter at slette grene, der ikke er flettet.", de: "Branch {{name}} löschen? Git löscht keine Branches, die nicht gemergt sind.",
    el: "Διαγραφή του κλάδου {{name}}; Το Git δεν διαγράφει κλάδους που δεν έχουν συγχωνευθεί.", es: "¿Eliminar la rama {{name}}? Git no elimina ramas que no estén fusionadas.", fr: "Supprimer la branche {{name}} ? Git refuse de supprimer les branches non fusionnées.", hi: "ब्रांच {{name}} हटाएँ? Git उन ब्रांचों को नहीं हटाता जो मर्ज नहीं हुई हैं।",
    it: "Eliminare il ramo {{name}}? Git non elimina i rami non uniti.", ja: "ブランチ {{name}} を削除しますか？マージされていないブランチは Git が削除を拒否します。", nl: "Branch {{name}} verwijderen? Git weigert branches te verwijderen die niet zijn samengevoegd.", no: "Slett grenen {{name}}? Git nekter å slette grener som ikke er flettet.",
    pl: "Usunąć gałąź {{name}}? Git nie usuwa gałęzi, które nie zostały scalone.", pt: "Excluir o branch {{name}}? O Git se recusa a excluir branches que não foram mesclados.", sv: "Ta bort grenen {{name}}? Git vägrar ta bort grenar som inte är sammanfogade.", zh: "删除分支 {{name}}？Git 不会删除尚未合并的分支。"
  },
  'codeStudio.gitSwitchedBranch': {
    en: "Switched to {{name}}", cs: "Přepnuto na {{name}}", da: "Skiftet til {{name}}", de: "Zu {{name}} gewechselt",
    el: "Έγινε μετάβαση στο {{name}}", es: "Cambiado a {{name}}", fr: "Basculé sur {{name}}", hi: "{{name}} पर स्विच किया गया",
    it: "Passato a {{name}}", ja: "{{name}} に切り替えました", nl: "Gewisseld naar {{name}}", no: "Byttet til {{name}}",
    pl: "Przełączono na {{name}}", pt: "Alterado para {{name}}", sv: "Bytte till {{name}}", zh: "已切换到 {{name}}"
  },
  'codeStudio.gitBranchDeleted': {
    en: "Deleted branch {{name}}", cs: "Větev {{name}} byla smazána", da: "Grenen {{name}} er slettet", de: "Branch {{name}} gelöscht",
    el: "Ο κλάδος {{name}} διαγράφηκε", es: "Rama {{name}} eliminada", fr: "Branche {{name}} supprimée", hi: "ब्रांच {{name}} हटा दी गई",
    it: "Ramo {{name}} eliminato", ja: "ブランチ {{name}} を削除しました", nl: "Branch {{name}} verwijderd", no: "Grenen {{name}} er slettet",
    pl: "Usunięto gałąź {{name}}", pt: "Branch {{name}} excluído", sv: "Grenen {{name}} togs bort", zh: "已删除分支 {{name}}"
  },
  'codeStudio.gitHistory': {
    en: "History", cs: "Historie", da: "Historik", de: "Verlauf",
    el: "Ιστορικό", es: "Historial", fr: "Historique", hi: "इतिहास",
    it: "Cronologia", ja: "履歴", nl: "Geschiedenis", no: "Historikk",
    pl: "Historia", pt: "Histórico", sv: "Historik", zh: "历史记录"
  },
  'codeStudio.gitHistoryFor': {
    en: "History of {{name}}", cs: "Historie souboru {{name}}", da: "Historik for {{name}}", de: "Verlauf von {{name}}",
    el: "Ιστορικό του {{name}}", es: "Historial de {{name}}", fr: "Historique de {{name}}", hi: "{{name}} का इतिहास",
    it: "Cronologia di {{name}}", ja: "{{name}} の履歴", nl: "Geschiedenis van {{name}}", no: "Historikk for {{name}}",
    pl: "Historia pliku {{name}}", pt: "Histórico de {{name}}", sv: "Historik för {{name}}", zh: "{{name}} 的历史记录"
  },
  'codeStudio.gitHistoryShowAll': {
    en: "Show all commits", cs: "Zobrazit všechny commity", da: "Vis alle commits", de: "Alle Commits anzeigen",
    el: "Εμφάνιση όλων των commit", es: "Mostrar todos los commits", fr: "Afficher tous les commits", hi: "सभी कमिट दिखाएँ",
    it: "Mostra tutti i commit", ja: "すべてのコミットを表示", nl: "Alle commits tonen", no: "Vis alle commits",
    pl: "Pokaż wszystkie commity", pt: "Mostrar todos os commits", sv: "Visa alla incheckningar", zh: "显示所有提交"
  },
  'codeStudio.gitNoCommits': {
    en: "No commits yet", cs: "Zatím žádné commity", da: "Ingen commits endnu", de: "Noch keine Commits",
    el: "Δεν υπάρχουν ακόμη commit", es: "Todavía no hay commits", fr: "Aucun commit pour l'instant", hi: "अभी तक कोई कमिट नहीं",
    it: "Ancora nessun commit", ja: "まだコミットがありません", nl: "Nog geen commits", no: "Ingen commits ennå",
    pl: "Brak commitów", pt: "Ainda não há commits", sv: "Inga incheckningar ännu", zh: "尚无提交"
  },
  'codeStudio.gitLoadingCommit': {
    en: "Loading commit...", cs: "Načítání commitu...", da: "Indlæser commit...", de: "Commit wird geladen...",
    el: "Φόρτωση commit...", es: "Cargando commit...", fr: "Chargement du commit...", hi: "कमिट लोड हो रहा है...",
    it: "Caricamento del commit...", ja: "コミットを読み込み中...", nl: "Commit laden...", no: "Laster inn commit...",
    pl: "Wczytywanie commita...", pt: "Carregando commit...", sv: "Läser in incheckning...", zh: "正在加载提交..."
  },
  'codeStudio.gitDiffTruncated': {
    en: "The diff is too large and was cut off.", cs: "Diff je příliš velký a byl zkrácen.", da: "Diffen er for stor og er blevet afkortet.", de: "Der Diff ist zu groß und wurde abgeschnitten.",
    el: "Το diff είναι πολύ μεγάλο και περικόπηκε.", es: "El diff es demasiado grande y se ha recortado.", fr: "Le diff est trop volumineux et a été tronqué.", hi: "डिफ़ बहुत बड़ा है और काट दिया गया।",
    it: "Il diff è troppo grande ed è stato troncato.", ja: "差分が大きすぎるため途中で切り詰めました。", nl: "De diff is te groot en is afgekapt.", no: "Diffen er for stor og ble avkortet.",
    pl: "Diff jest zbyt duży i został obcięty.", pt: "O diff é grande demais e foi cortado.", sv: "Diffen är för stor och har kortats.", zh: "差异过大，已被截断。"
  },
  'codeStudio.gitNotCommitted': {
    en: "Not committed yet", cs: "Zatím necommitnuto", da: "Ikke committet endnu", de: "Noch nicht committet",
    el: "Δεν έχει γίνει ακόμη commit", es: "Aún no confirmado", fr: "Pas encore validé", hi: "अभी कमिट नहीं हुआ",
    it: "Non ancora sottoposto a commit", ja: "まだコミットされていません", nl: "Nog niet gecommit", no: "Ikke committet ennå",
    pl: "Jeszcze niezatwierdzone", pt: "Ainda sem commit", sv: "Inte incheckat ännu", zh: "尚未提交"
  },
  'codeStudio.gitCommitTab': {
    en: "Commit {{hash}}", cs: "Commit {{hash}}", da: "Commit {{hash}}", de: "Commit {{hash}}",
    el: "Commit {{hash}}", es: "Commit {{hash}}", fr: "Commit {{hash}}", hi: "कमिट {{hash}}",
    it: "Commit {{hash}}", ja: "コミット {{hash}}", nl: "Commit {{hash}}", no: "Commit {{hash}}",
    pl: "Commit {{hash}}", pt: "Commit {{hash}}", sv: "Incheckning {{hash}}", zh: "提交 {{hash}}"
  },
  'codeStudio.gitChangedFiles': {
    en: "Changed files ({{count}})", cs: "Změněné soubory ({{count}})", da: "Ændrede filer ({{count}})", de: "Geänderte Dateien ({{count}})",
    el: "Αλλαγμένα αρχεία ({{count}})", es: "Archivos modificados ({{count}})", fr: "Fichiers modifiés ({{count}})", hi: "बदली गई फ़ाइलें ({{count}})",
    it: "File modificati ({{count}})", ja: "変更されたファイル ({{count}})", nl: "Gewijzigde bestanden ({{count}})", no: "Endrede filer ({{count}})",
    pl: "Zmienione pliki ({{count}})", pt: "Arquivos alterados ({{count}})", sv: "Ändrade filer ({{count}})", zh: "已更改的文件 ({{count}})"
  },
  'codeStudio.gitToggleBlame': {
    en: "Git: Toggle Blame", cs: "Git: Přepnout blame", da: "Git: Vis/skjul blame", de: "Git: Blame ein-/ausblenden",
    el: "Git: Εναλλαγή blame", es: "Git: Mostrar u ocultar blame", fr: "Git : afficher/masquer le blame", hi: "Git: ब्लेम टॉगल करें",
    it: "Git: mostra/nascondi blame", ja: "Git: Blame の表示切り替え", nl: "Git: blame aan/uit", no: "Git: Vis/skjul blame",
    pl: "Git: przełącz blame", pt: "Git: alternar blame", sv: "Git: Visa/dölj blame", zh: "Git：切换 Blame"
  },
  'codeStudio.gitFileHistory': {
    en: "Git: File History", cs: "Git: Historie souboru", da: "Git: Filhistorik", de: "Git: Dateiverlauf",
    el: "Git: Ιστορικό αρχείου", es: "Git: Historial del archivo", fr: "Git : historique du fichier", hi: "Git: फ़ाइल इतिहास",
    it: "Git: cronologia del file", ja: "Git: ファイルの履歴", nl: "Git: bestandsgeschiedenis", no: "Git: Filhistorikk",
    pl: "Git: historia pliku", pt: "Git: histórico do arquivo", sv: "Git: Filhistorik", zh: "Git：文件历史记录"
  }
};

for (const lang of LANGS) {
  const file = join(root, 'ui', 'lang', 'desktop', `${lang}.json`);
  const original = readFileSync(file, 'utf8');
  const bom = original.startsWith(BOM) ? BOM : '';
  const text = bom ? original.slice(1) : original;
  const eol = text.includes('\r\n') ? '\r\n' : '\n';
  const lines = text.split(eol);
  const entryLine = lines.find(line => /^\s+"[^"]+":/.test(line));
  const indent = entryLine ? entryLine.match(/^\s+/)[0] : '  ';
  const appended = [];
  let updated = 0;
  for (const [key, values] of Object.entries(STRINGS)) {
    const value = values[lang];
    if (typeof value !== 'string' || !value.trim()) throw new Error(`${lang}: no text for ${key}`);
    const prefix = `${indent}${JSON.stringify(key)}:`;
    const entry = `${prefix} ${JSON.stringify(value)}`;
    const index = lines.findIndex(line => line.startsWith(prefix));
    if (index >= 0) {
      lines[index] = entry + (lines[index].trimEnd().endsWith(',') ? ',' : '');
      updated++;
    } else {
      appended.push(entry);
    }
  }
  if (appended.length) {
    let close = lines.length - 1;
    while (close > 0 && lines[close].trim() !== '}') close--;
    let last = close - 1;
    while (last > 0 && !lines[last].trim()) last--;
    if (close <= 0 || last <= 0) throw new Error(`${lang}: closing brace not found`);
    if (!lines[last].trimEnd().endsWith(',') && lines[last].trim() !== '{') lines[last] = lines[last].trimEnd() + ',';
    lines.splice(close, 0, ...appended.map((entry, i) => (i < appended.length - 1 ? entry + ',' : entry)));
  }
  const output = bom + lines.join(eol);
  const parsed = JSON.parse(output.slice(bom.length));
  for (const [key, values] of Object.entries(STRINGS)) {
    if (parsed[key] !== values[lang]) throw new Error(`${lang}: ${key} did not land`);
  }
  if (output !== original) writeFileSync(file, output, 'utf8');
  console.log(`${lang}.json: ${appended.length} added, ${updated} updated`);
}
```

- [ ] **Step 2: Run the script.**

Run: `node C:\disposable\code-studio-pkg1\locales-b.mjs C:\Users\Andi\Documents\repo\AuraGo`
Expected: 16 lines of the form `xx.json: 29 added, 0 updated`. Run it a second time. Expected: `0 added, 29 updated` for every file, and the files stay unchanged.

- [ ] **Step 3: Test.**

Run: `go test ./ui -count=1 -run "(?i)(DesktopUsedI18nKeysExistInAllLanguages|TranslationsHaveNoMergedLineArtifacts|CodeStudioGitSelection)"`
Expected: PASS.

Commit: `feat(code-studio): Git workbench strings in 16 languages`. Files: `ui/lang/desktop/cs.json`, `ui/lang/desktop/da.json`, `ui/lang/desktop/de.json`, `ui/lang/desktop/el.json`, `ui/lang/desktop/en.json`, `ui/lang/desktop/es.json`, `ui/lang/desktop/fr.json`, `ui/lang/desktop/hi.json`, `ui/lang/desktop/it.json`, `ui/lang/desktop/ja.json`, `ui/lang/desktop/nl.json`, `ui/lang/desktop/no.json`, `ui/lang/desktop/pl.json`, `ui/lang/desktop/pt.json`, `ui/lang/desktop/sv.json`, `ui/lang/desktop/zh.json`

---

### Task 6: Route every diff special case through `isVirtualTab`

**Files:**
- Modify: `ui/js/desktop/apps/code-studio/core.js`
- Modify: `ui/js/desktop/apps/code-studio/agent.js`
- Modify: `ui/js/desktop/apps/code-studio/outline.js`
- Modify: `ui/js/desktop/apps/code-studio/panels.js`
- Modify: `ui/js/desktop/apps/code-studio/problems.js`
- Modify: `ui/js/desktop/apps/code-studio/command-palette.js`
- Modify: `ui/js/desktop/apps/code-studio/shortcuts.js`
- Generated: `ui/js/desktop/bundles/code-studio.bundle.js`

Diff tabs and commit tabs show git data, not a file. They are never saved, run, persisted, split, sent to the agent or listed as files. These are the sites, with their pre-A line numbers:

| Site | Line | Edit |
| --- | --- | --- |
| `core.js` `saveState` | 455 | Step 3 |
| `core.js` email attach in `render` | 481 | Step 4 |
| `core.js` `policyDisables` | 567 | Step 5 |
| `core.js` `renderTabs` label | 936 | Step 6 |
| `core.js` `renderBreadcrumbs` (new commit case) | 1101 | Step 7 |
| `core.js` `tabHasLocalState` | 1381 | Step 8 |
| `core.js` `openFile` | 1421 | Step 9 |
| `core.js` `saveTab` | 1561 | Step 10 |
| `core.js` `mutatePath` | 1599 | Step 11 |
| `core.js` `renamePath` | 1675 | Step 12 |
| `core.js` `runCurrentFile` | 1850 | Step 13 |
| `agent.js` | 222, 242, 248, 258 | Step 14 |
| `outline.js` | 117 | Step 15 |
| `panels.js` `splitEditor` | 17 | Step 16 |
| `problems.js` `checkDocumentContent` | 331 | Step 17 |
| `command-palette.js` `getCommands` | 85 | Step 18 |
| `command-palette.js` `getFileItems` (new commit case) | 136 | Step 18 |
| `editor.js` `renderEditor` | 26 | Task 7, Step 4 (needs `renderGitCommitTab`) |
| `git.js` `openGitDiff` lookup | 133 | Stays: it looks up the diff tab of one path |

`sessionRevision` (line 109) and the tab signature (line 931) only read `kind` as data, so they stay unchanged.

- [ ] **Step 1: Impact check.** Run `impact` upstream on `saveState`, `policyDisables`, `renderTabs`, `renderBreadcrumbs`, `tabHasLocalState`, `openFile`, `saveTab`, `mutatePath`, `renamePath`, `runCurrentFile`, `runCodeAction`, `codeStudioAgentContext`, `suggestionCanApply`, `renderOutline`, `splitEditor` and `checkDocumentContent`, and report the risk. These are JS symbols inside one IIFE, so callers are often `UNKNOWN`. Confirm by text search that each change only widens a `diff` check to `diff` or `commit`.

- [ ] **Step 2: Add the helpers.** In `ui/js/desktop/apps/code-studio/core.js`:

Replace:
```js
    function editorValue(tab) {
```
With:
```js
    // Diff and commit tabs show git data, not a file: they are never saved, run, persisted,
    // split, listed as files or sent to the agent.
    function isVirtualTab(tab) {
        return !!tab && (tab.kind === 'diff' || tab.kind === 'commit');
    }

    function studioTabLabel(tab) {
        if (tab.kind === 'commit') return tr('codeStudio.gitCommitTab', 'Commit {{hash}}', { hash: tab.short || String(tab.hash || '').slice(0, 7) });
        return baseName(tab.path) + (tab.kind === 'diff' ? ' · ' + tr('codeStudio.gitDiff', 'Git Diff') : '');
    }

    function editorValue(tab) {
```

- [ ] **Step 3:** In `core.js` (`saveState`):

Replace:
```js
        const openPaths = normalizePersistedPaths(state.openTabs.filter(tab => tab.kind !== 'diff'));
```
With:
```js
        const openPaths = normalizePersistedPaths(state.openTabs.filter(tab => !isVirtualTab(tab)));
```

- [ ] **Step 4:** In `core.js` (email attach inside `render`):

Replace:
```js
tab.binary||tab.kind==='diff'
```
With:
```js
tab.binary||isVirtualTab(tab)
```

- [ ] **Step 5:** In `core.js` (`policyDisables`). A may extend the id list in front of this tail, so only the tail is replaced:

Replace:
```js
.includes(id) && (!tab || tab.kind === 'diff');
```
With:
```js
.includes(id) && (!tab || isVirtualTab(tab));
```

- [ ] **Step 6:** In `core.js` (`renderTabs`):

Replace:
```js
                <span>${esc(baseName(tab.path))}${tab.kind === 'diff' ? ' · ' + esc(tr('codeStudio.gitDiff', 'Git Diff')) : ''}</span>
```
With:
```js
                <span>${esc(studioTabLabel(tab))}</span>
```

- [ ] **Step 7:** In `core.js` (`renderBreadcrumbs`; a commit has no path to show):

Replace:
```js
        if (!tab) {
            bar.innerHTML = '';
```
With:
```js
        if (!tab || tab.kind === 'commit') {
            bar.innerHTML = '';
```

- [ ] **Step 8:** In `core.js` (`tabHasLocalState`):

Replace:
```js
        return !tab || tab.kind === 'diff' || tab.modified || !!tab.pathMutation || !!tab.pendingSave;
```
With:
```js
        return !tab || isVirtualTab(tab) || tab.modified || !!tab.pathMutation || !!tab.pendingSave;
```

- [ ] **Step 9:** In `core.js` (`openFile`):

Replace:
```js
state.openTabs.findIndex(tab => tab.path === path && tab.kind !== 'diff')
```
With:
```js
state.openTabs.findIndex(tab => tab.path === path && !isVirtualTab(tab))
```

- [ ] **Step 10:** In `core.js` (`saveTab`):

Replace:
```js
!tab || tab.kind === 'diff' || !target.openTabs.includes(tab)
```
With:
```js
!tab || isVirtualTab(tab) || !target.openTabs.includes(tab)
```

- [ ] **Step 11:** In `core.js` (`mutatePath`):

Replace:
```js
        if (!canWrite(target) || file.kind === 'diff') return false;
```
With:
```js
        if (!canWrite(target) || isVirtualTab(file)) return false;
```

- [ ] **Step 12:** In `core.js` (`renamePath`):

Replace:
```js
                if (tab.kind === 'diff') return;
```
With:
```js
                if (isVirtualTab(tab)) return;
```

- [ ] **Step 13:** In `core.js` (`runCurrentFile`):

Replace:
```js
        if (!tab || tab.kind === 'diff') return;
```
With:
```js
        if (!tab || isVirtualTab(tab)) return;
```

- [ ] **Step 14:** In `ui/js/desktop/apps/code-studio/agent.js`, make four edits.

Replace:
```js
        if (!tab || tab.kind === 'diff' || action !== 'explain' && !canWrite()) return;
```
With:
```js
        if (!tab || isVirtualTab(tab) || action !== 'explain' && !canWrite()) return;
```

Replace:
```js
            current_file: tab && tab.kind !== 'diff' ? tab.path : '',
```
With:
```js
            current_file: tab && !isVirtualTab(tab) ? tab.path : '',
```

Replace:
```js
            open_files: state.openTabs.filter(item => item.kind !== 'diff').map(item => item.path)
```
With:
```js
            open_files: state.openTabs.filter(item => !isVirtualTab(item)).map(item => item.path)
```

Replace:
```js
        return canWrite() && activeTab()?.kind !== 'diff' && suggestion && suggestion.canReplace && suggestion.tab && suggestion.tab.kind !== 'diff' && suggestion.windowId === state.windowId &&
```
With:
```js
        return canWrite() && !isVirtualTab(activeTab()) && suggestion && suggestion.canReplace && suggestion.tab && !isVirtualTab(suggestion.tab) && suggestion.windowId === state.windowId &&
```

- [ ] **Step 15:** In `ui/js/desktop/apps/code-studio/outline.js`:

Replace:
```js
        const symbols = tab && tab.kind !== 'diff' ? scanDocumentSymbols(tab) : [];
```
With:
```js
        const symbols = tab && !isVirtualTab(tab) ? scanDocumentSymbols(tab) : [];
```

- [ ] **Step 16:** In `ui/js/desktop/apps/code-studio/panels.js`. If A rewrote `splitEditor`, apply the same substitution to A's guard and to every split check A added that tests `kind === 'diff'`.

Replace:
```js
        if (!state || activeTab()?.kind === 'diff') return;
```
With:
```js
        if (!state || isVirtualTab(activeTab())) return;
```

- [ ] **Step 17:** In `ui/js/desktop/apps/code-studio/problems.js`:

Replace:
```js
        if (!tab || tab.kind === 'diff') return [];
```
With:
```js
        if (!tab || isVirtualTab(tab)) return [];
```

- [ ] **Step 18:** In `ui/js/desktop/apps/code-studio/command-palette.js`, a separate IIFE that uses the helper exposed in Step 19:

Replace:
```js
        const sourceTab = hasTab && state.openTabs[state.activeTabIndex]?.kind !== 'diff';
```
With:
```js
        const sourceTab = hasTab && !getApp().isVirtualTab(state.openTabs[state.activeTabIndex]);
```

Replace:
```js
        (state.openTabs || []).forEach(tab => push(tab.path, 'file', 'file'));
```
With:
```js
        (state.openTabs || []).filter(tab => !app.isVirtualTab(tab)).forEach(tab => push(tab.path, 'file', 'file'));
```

- [ ] **Step 19:** In `ui/js/desktop/apps/code-studio/shortcuts.js` (`window.CodeStudioApp`):

Replace:
```js
        api: apiClient,
```
With:
```js
        api: apiClient,
        isVirtualTab,
```

- [ ] **Step 20: API wrappers for the new endpoints** in `core.js` (`apiClient`).

Replace:
```js
            if (action === 'create' || action === 'switch') {
```
With:
```js
            if (action === 'create' || action === 'switch' || action === 'delete') {
```

Replace:
```js
        gitLog: (path, count) => api('/api/code-studio/git/log?' + new URLSearchParams({ path: path || '', count: String(count || 20) })),
```
With:
```js
        gitLog: (path, count) => api('/api/code-studio/git/log?' + new URLSearchParams({ path: path || '', count: String(count || 20) })),
        gitStage: paths => api('/api/code-studio/git/stage', { method: 'POST', body: JSON.stringify({ paths }) }),
        gitUnstage: paths => api('/api/code-studio/git/unstage', { method: 'POST', body: JSON.stringify({ paths }) }),
        gitDiscard: paths => api('/api/code-studio/git/discard', { method: 'POST', body: JSON.stringify({ paths }) }),
        gitInit: () => api('/api/code-studio/git/init', { method: 'POST' }),
        gitShow: hash => api('/api/code-studio/git/show?hash=' + encodeURIComponent(hash)),
        gitBlame: path => api('/api/code-studio/git/blame?path=' + encodeURIComponent(path)),
```

- [ ] **Step 21: Verify that no unrouted diff check remains.** Grep for `kind ?[!=]== ?'diff'` in `ui/js/desktop/apps/code-studio`. Expected hits only:
  - `core.js`: inside `isVirtualTab` and `studioTabLabel`
  - `editor.js`: `renderEditor`, handled in Task 7
  - `git.js`: `openGitDiff` lookup

  Route any other hit, for example a check that plan A added, through `isVirtualTab`.

- [ ] **Step 22: Build and test.**

Run: `npm run build:ui`
Then: `node --check ui/js/desktop/bundles/code-studio.bundle.js`
Expected: no output from `node --check`.

Run: `go test ./ui -count=1 -run "(?i)(CodeStudioGitSelection|DesktopUsedI18nKeysExistInAllLanguages|DesktopCodeStudioShellI18n)"`
Expected: PASS.

Commit: `feat(code-studio): route diff special cases through isVirtualTab`. Files: `ui/js/desktop/apps/code-studio/core.js`, `ui/js/desktop/apps/code-studio/agent.js`, `ui/js/desktop/apps/code-studio/outline.js`, `ui/js/desktop/apps/code-studio/panels.js`, `ui/js/desktop/apps/code-studio/problems.js`, `ui/js/desktop/apps/code-studio/command-palette.js`, `ui/js/desktop/apps/code-studio/shortcuts.js`, `ui/js/desktop/bundles/code-studio.bundle.js`

---

### Task 7: Staging panel, history, commit tab, blame and branch menu

**Files:**
- Rewrite: `ui/js/desktop/apps/code-studio/git.js`
- Create: `ui/js/desktop/apps/code-studio/git-history.js`
- Modify: `ui/js/desktop/apps/code-studio/core.js`
- Modify: `ui/js/desktop/apps/code-studio/editor.js`
- Modify: `ui/js/desktop/apps/code-studio/command-palette.js`
- Modify: `ui/js/desktop/apps/code-studio/shortcuts.js`
- Modify: `scripts/build-ui-bundles.js`
- Rewrite: `ui/code_studio_git_selection_test.go`
- Generated: `ui/js/desktop/bundles/code-studio.bundle.js`

Panel and history together come to about 650 lines, so history, the commit tab and blame move into the new fragment `git-history.js`. It sits after `git.js` in the bundle, inside the IIFE that `core.js` opens and `shortcuts.js` closes.

- [ ] **Step 1: Impact check.** Run `impact` upstream on `renderGitPanel`, `refreshGitStatus`, `commitGitChanges`, `renderGitDiffTab`, `renderEditor`, `createCodeMirrorEditor`, `renderStatus`, `updatePolicyControls`, `createInstance` and `studioCommandTable`, and report the risk.

- [ ] **Step 2: Rewrite `ui/js/desktop/apps/code-studio/git.js`.** Replace the whole file with this complete content:

```js
    // Source Control panel of Code Studio: staged and unstaged groups, commit, repository init
    // and the branch menu. History, the commit tab and the blame gutter live in git-history.js.

    function gitBranchIconMarkup(size) {
        const pixels = Number(size) || 14;
        return `<svg viewBox="0 0 16 16" width="${pixels}" height="${pixels}" fill="currentColor" aria-hidden="true" focusable="false"><path d="M11.75 2.5a.75.75 0 100 1.5.75.75 0 000-1.5zm-2.25.75a2.25 2.25 0 113 2.122V6.5a2 2 0 01-2 2H7.5v1.128a2.251 2.251 0 11-1.5 0V5.372a2.25 2.25 0 111.5 0v1.878h3a.5.5 0 00.5-.5v-1.128A2.251 2.251 0 019.5 3.25zM4.25 3.5a.75.75 0 100 1.5.75.75 0 000-1.5zM4.25 12a.75.75 0 100 1.5.75.75 0 000-1.5z"/></svg>`;
    }

    function gitChangeIsStaged(change) {
        return change.staged === true || change.staged === 'true';
    }

    // One status entry with the porcelain index (X) and worktree (Y) characters. Entries
    // without them (older servers, test fixtures) are derived from status and staged.
    function normalizeGitChange(raw) {
        const change = raw || {};
        const status = String(change.status || '');
        let index = typeof change.index === 'string' && change.index.length === 1 ? change.index : '';
        let worktree = typeof change.worktree === 'string' && change.worktree.length === 1 ? change.worktree : '';
        if (!index || !worktree) {
            if (status === '??') {
                index = '?';
                worktree = '?';
            } else if (gitChangeIsStaged(change)) {
                index = status.charAt(0) || 'M';
                worktree = ' ';
            } else {
                index = ' ';
                worktree = status.charAt(0) || 'M';
            }
        }
        return { path: String(change.path || ''), status, index, worktree };
    }

    // A file that is staged and changed again in the work tree is listed in both groups.
    function gitChangeGroups() {
        const staged = [];
        const changes = [];
        (state.gitChanges || []).forEach(raw => {
            const change = normalizeGitChange(raw);
            if (!change.path) return;
            if (change.index !== ' ' && change.index !== '?') staged.push(change);
            if (change.worktree !== ' ') changes.push(change);
        });
        return { staged, changes };
    }

    function gitChangeUntracked(change) {
        return change.index === '?' && change.worktree === '?';
    }

    function gitStatusCode(change, staged) {
        if (gitChangeUntracked(change)) return 'U';
        return staged ? change.index : change.worktree;
    }

    function gitStatusClass(code) {
        if (code === 'U' || code === '??') return 'untracked';
        if (code === 'M') return 'modified';
        if (code === 'A') return 'added';
        if (code === 'D') return 'deleted';
        return 'renamed';
    }

    function gitRowButton(attribute, value, label, glyph) {
        const valueHtml = value === null ? '' : `="${esc(value)}"`;
        return `<button type="button" class="cs-git-row-action" ${attribute}${valueHtml} data-git-write title="${esc(label)}" aria-label="${esc(label)}">${esc(glyph)}</button>`;
    }

    function gitChangeRowHtml(change, staged) {
        const code = gitStatusCode(change, staged);
        const trimmed = change.path.replace(/\/+$/, '');
        const slash = trimmed.lastIndexOf('/');
        const name = trimmed.slice(slash + 1) + (trimmed.length < change.path.length ? '/' : '');
        const dir = slash > 0 ? trimmed.slice(0, slash) : '';
        const actions = staged
            ? gitRowButton('data-git-unstage', change.path, tr('codeStudio.gitUnstage', 'Unstage Changes'), '−')
            : gitRowButton('data-git-discard', change.path, tr('codeStudio.discardChanges', 'Discard Changes'), '↶')
                + gitRowButton('data-git-stage', change.path, tr('codeStudio.gitStage', 'Stage Changes'), '+');
        return `<div class="cs-git-change-row">
                <button type="button" class="cs-git-change" data-git-file="${esc(change.path)}" data-git-staged="${staged ? 'true' : 'false'}" title="${esc(change.path)}">
                    <span class="cs-git-status ${gitStatusClass(code)}">${esc(code)}</span>
                    <span class="cs-git-filename">${esc(name)}</span>
                    ${dir ? `<span class="cs-git-dir">${esc(dir)}</span>` : ''}
                </button>
                <span class="cs-git-row-actions">${actions}</span>
            </div>`;
    }

    function gitGroupHtml(kind, title, changes, staged) {
        const bulk = staged
            ? gitRowButton('data-git-unstage-all', null, tr('codeStudio.gitUnstageAll', 'Unstage All Changes'), '−')
            : gitRowButton('data-git-stage-all', null, tr('codeStudio.gitStageAll', 'Stage All Changes'), '+');
        const rows = changes.length
            ? changes.map(change => gitChangeRowHtml(change, staged)).join('')
            : `<div class="cs-git-empty">${esc(tr('codeStudio.noChanges', 'No changes'))}</div>`;
        return `<div class="cs-git-section" data-git-section="${kind}">
                <div class="cs-git-section-head"><span class="cs-git-section-title">${esc(title)}</span><span class="cs-git-count">${changes.length}</span>${changes.length ? `<span class="cs-git-section-actions">${bulk}</span>` : ''}</div>
                <div class="cs-git-changes">${rows}</div>
            </div>`;
    }

    function renderGitPanel() {
        const panel = shellPart('[data-git-panel]');
        if (!panel) return;
        if (!state.gitVisible) {
            panel.hidden = true;
            return;
        }
        panel.hidden = false;
        // A refresh re-renders the panel: the commit message keeps focus and caret, the list its scroll position.
        const focused = document.activeElement;
        const keepFocus = !!focused && panel.contains(focused) && focused.matches('[data-git-commit-msg]');
        const caret = keepFocus ? [focused.selectionStart, focused.selectionEnd] : null;
        const scrollTop = panel.scrollTop;
        const head = `<div class="cs-git-head">
            <strong>${esc(tr('codeStudio.gitPanel', 'Source Control'))}</strong>
            <button type="button" class="cs-icon-button" data-git-refresh title="${esc(tr('codeStudio.refresh', 'Refresh'))}">${iconMarkup('refresh', 'R', 'cs-icon-button-icon', 14)}</button>
            <button type="button" class="cs-icon-button" data-git-close title="${esc(tr('codeStudio.closeTab', 'Close'))}">${iconMarkup('x', 'X', 'cs-icon-button-icon', 14)}</button>
        </div>`;
        if (state.gitIsRepo === false) {
            panel.innerHTML = `${head}
                <div class="cs-git-init">
                    <p>${esc(tr('codeStudio.noRepo', 'No git repository'))}</p>
                    <button type="button" class="cs-button primary" data-git-init data-git-write>${esc(tr('codeStudio.gitInitRepo', 'Initialize Repository'))}</button>
                </div>`;
        } else {
            const groups = gitChangeGroups();
            const branchHtml = state.gitIsRepo === true
                ? `<button type="button" class="cs-git-branch" data-git-branch title="${esc(tr('codeStudio.gitBranchMenu', 'Switch, create or delete branches'))}">${gitBranchIconMarkup(14)}<span>${esc(state.gitBranch || tr('codeStudio.gitDetached', 'Detached HEAD'))}</span></button>`
                : '';
            const identityHtml = state.gitIdentityPrompt ? `<form class="cs-git-identity" data-git-identity>
                    <p>${esc(tr('codeStudio.gitIdentityHint', 'Git needs your name and email for this repository.'))}</p>
                    <input name="name" value="${esc(state.gitIdentityName || '')}" placeholder="${esc(tr('codeStudio.identityName', 'Your name'))}" autocomplete="name" maxlength="200">
                    <input name="email" type="email" value="${esc(state.gitIdentityEmail || '')}" placeholder="${esc(tr('codeStudio.identityEmail', 'Your email'))}" autocomplete="email" maxlength="200">
                    <button type="submit" class="cs-button">${esc(tr('codeStudio.saveIdentity', 'Save identity'))}</button>
                </form>` : '';
            panel.innerHTML = `${head}
                ${branchHtml}
                <div class="cs-git-commit-area">
                    <textarea class="cs-git-commit-msg" data-git-commit-msg placeholder="${esc(tr('codeStudio.commitMessage', 'Commit message...'))}" rows="3">${esc(state.gitCommitMessage || '')}</textarea>
                    <div class="cs-git-commit-actions">
                        <button type="button" class="cs-button primary" data-git-commit>${esc(tr('codeStudio.commit', 'Commit'))}</button>
                    </div>
                    ${identityHtml}
                </div>
                ${groups.staged.length ? gitGroupHtml('staged', tr('codeStudio.gitStagedChanges', 'Staged Changes'), groups.staged, true) : ''}
                ${gitGroupHtml('changes', tr('codeStudio.changes', 'Changes'), groups.changes, false)}
                ${state.gitIsRepo === true ? gitHistorySectionHtml() : ''}`;
        }
        wireGitPanel(panel);
        panel.scrollTop = scrollTop;
        const message = panel.querySelector('[data-git-commit-msg]');
        if (keepFocus && message) {
            message.focus();
            message.setSelectionRange(caret[0], caret[1]);
        }
        updatePolicyControls();
    }

    function wireGitPanel(panel) {
        const onClick = (selector, handler) => {
            const element = panel.querySelector(selector);
            if (element) element.addEventListener('click', bind(handler));
        };
        onClick('[data-git-close]', toggleGitPanel);
        onClick('[data-git-refresh]', refreshGitStatus);
        onClick('[data-git-init]', initGitRepository);
        onClick('[data-git-commit]', commitGitChanges);
        onClick('[data-git-stage-all]', () => stageGitPaths(gitChangeGroups().changes.map(change => change.path)));
        onClick('[data-git-unstage-all]', () => unstageGitPaths(gitChangeGroups().staged.map(change => change.path)));
        const branch = panel.querySelector('[data-git-branch]');
        if (branch) branch.addEventListener('click', bind(() => {
            const rect = branch.getBoundingClientRect();
            showGitBranchMenu(rect.left, rect.bottom + 2);
        }));
        const messageBox = panel.querySelector('[data-git-commit-msg]');
        if (messageBox) messageBox.addEventListener('input', bind(() => {
            state.gitCommitMessage = messageBox.value;
        }));
        const identityForm = panel.querySelector('[data-git-identity]');
        if (identityForm) identityForm.addEventListener('submit', bind(event => {
            event.preventDefault();
            saveGitIdentity(new FormData(event.currentTarget));
        }));
        panel.querySelectorAll('[data-git-file]').forEach(btn => {
            btn.addEventListener('click', bind(() => openGitDiff(btn.dataset.gitFile, btn.dataset.gitStaged === 'true')));
        });
        panel.querySelectorAll('[data-git-stage]').forEach(btn => {
            btn.addEventListener('click', bind(() => stageGitPaths([btn.dataset.gitStage])));
        });
        panel.querySelectorAll('[data-git-unstage]').forEach(btn => {
            btn.addEventListener('click', bind(() => unstageGitPaths([btn.dataset.gitUnstage])));
        });
        panel.querySelectorAll('[data-git-discard]').forEach(btn => {
            btn.addEventListener('click', bind(() => discardGitChange(btn.dataset.gitDiscard)));
        });
        wireGitHistorySection(panel);
    }

    function toggleGitPanel() {
        state.gitVisible = !state.gitVisible;
        const root = ensureShellRoot();
        if (root) root.dataset.git = state.gitVisible ? 'visible' : 'hidden';
        if (state.gitVisible) refreshGitStatus();
        renderActivityBar();
        renderWindowMenus();
        scheduleTerminalFit();
    }

    function scheduleGitRefresh() {
        const instance = state;
        clearTimeout(instance.gitRefreshTimer);
        instance.gitRefreshTimer = setTimeout(bindInstance(instance, () => {
            instance.gitRefreshTimer = null;
            refreshGitStatus();
        }), 200);
    }

    async function refreshGitStatus() {
        const target = state;
        if (!isLiveInstance(target)) return;
        const request = target.gitRequest = Symbol();
        try {
            const result = await apiClient.gitStatus();
            if (!isLiveInstance(target) || target.gitRequest !== request) return;
            runWithInstance(target, () => {
                state.gitIsRepo = result.is_repo !== false;
                state.gitBranch = result.branch || '';
                state.gitChanges = Array.isArray(result.changes) ? result.changes : [];
                state.gitLog = Array.isArray(result.log) ? result.log : [];
                renderGitPanel();
                renderActivityBar();
                renderStatus();
                if (!state.gitIsRepo) return;
                refreshGitHistory();
                // A refresh follows every save, stage, commit and checkout: blame is read again
                // for each tab that shows it and has no unsaved edits.
                state.openTabs.forEach(tab => {
                    if (tab.blameOn && !tab.modified) refreshGitBlame(tab);
                });
            });
        } catch (err) {
            if (isLiveInstance(target) && target.gitRequest === request) {
                runWithInstance(target, () => {
                    state.gitIsRepo = null;
                    state.gitBranch = '';
                    state.gitChanges = [];
                    state.gitLog = [];
                    renderGitPanel();
                    renderStatus();
                });
            }
        }
    }

    async function openGitDiff(filePath, staged = false) {
        const target = state;
        if (!isLiveInstance(target)) return;
        let tab = target.openTabs.find(item => item.kind === 'diff' && item.gitPath === filePath);
        if (!tab) {
            tab = { kind: 'diff', gitPath: filePath, path: desktopToCodeStudioPath(filePath), content: '', modified: false, staged };
            target.openTabs.push(tab);
        }
        tab.staged = staged;
        tab.content = '';
        tab.loading = true;
        activateTab(target.openTabs.indexOf(tab));
        const request = tab.diffRequest = Symbol();
        try {
            const result = await apiClient.gitDiff(filePath, staged);
            if (!isLiveInstance(target) || !target.openTabs.includes(tab) || tab.diffRequest !== request) return;
            runWithInstance(target, () => {
                tab.content = result.diff || '';
                tab.loading = false;
                if (activeTab() === tab) renderEditor();
            });
        } catch (err) {
            if (!isLiveInstance(target) || !target.openTabs.includes(tab) || tab.diffRequest !== request) return;
            runWithInstance(target, () => {
                tab.loading = false;
                tab.content = err.message || String(err);
                if (activeTab() === tab) renderEditor();
            });
        }
    }

    // The diff renderer of the diff tab and of the commit tab.
    function diffLinesHtml(text) {
        return String(text || '').split('\n').map(line => {
            const style = line.startsWith('+') && !line.startsWith('+++') ? 'added' : line.startsWith('-') && !line.startsWith('---') ? 'removed' : line.startsWith('@@') ? 'context' : '';
            return `<span class="cs-diff-line ${style}">${esc(line)}</span>`;
        }).join('');
    }

    function renderGitDiffTab(editor, tab) {
        editor.innerHTML = `<div class="cs-diff-view">
            <div class="cs-diff-view-head">
                <span class="cs-diff-view-title"><strong>${esc(baseName(tab.path))}</strong><span class="cs-diff-badge">${esc(tr('codeStudio.gitDiff', 'Git Diff'))} · ${esc(tr('codeStudio.readOnly', 'Read-only'))}</span></span>
                <span class="cs-diff-view-spacer"></span>
                <select data-diff-source aria-label="${esc(tr('codeStudio.gitDiff', 'Git Diff'))}"><option value="worktree"${tab.staged ? '' : ' selected'}>${esc(tr('codeStudio.diffWorktree', 'Working tree'))}</option><option value="index"${tab.staged ? ' selected' : ''}>${esc(tr('codeStudio.diffIndex', 'Index'))}</option></select>
                <button type="button" class="cs-icon-button" data-diff-close aria-label="${esc(tr('desktop.close', 'Close'))}">×</button>
            </div>
            <div class="cs-diff-content">${tab.loading ? esc(tr('codeStudio.starting', 'Starting...')) : tab.content ? diffLinesHtml(tab.content) : esc(tr('codeStudio.noChanges', 'No changes'))}</div>
        </div>`;
        editor.querySelector('[data-diff-close]').addEventListener('click', bind(() => closeTab(state.openTabs.indexOf(tab))));
        editor.querySelector('[data-diff-source]').addEventListener('change', bind(event => openGitDiff(tab.gitPath, event.target.value === 'index')));
    }

    async function commitGitChanges() {
        const target = state;
        if (!canWrite(target)) return;
        const msgInput = shellPart('[data-git-commit-msg]');
        const message = (msgInput ? msgInput.value : state.gitCommitMessage || '').trim();
        state.gitCommitMessage = message;
        if (!message) {
            renderStatus(tr('codeStudio.commitMessage', 'Commit message...'));
            return;
        }
        const groups = gitChangeGroups();
        if (!groups.staged.length && !groups.changes.length) {
            flashStatus(tr('codeStudio.noChanges', 'No changes'));
            return;
        }
        // Commit takes the index as it is. Only with nothing staged does it offer to stage everything first.
        let addAll = false;
        if (!groups.staged.length) {
            const confirmed = await confirmValue(tr('codeStudio.gitStageAllAndCommit', 'Stage all changes and commit?'), { confirmLabel: tr('codeStudio.commit', 'Commit'), confirmIcon: 'check-square' });
            if (!confirmed || !canWrite(target)) return;
            addAll = true;
        }
        try {
            const result = await apiClient.gitCommit(message, { addAll });
            if (!canWrite(target)) return;
            runWithInstance(target, () => {
                flashStatus(tr('codeStudio.committed', 'Committed') + ': ' + (result.hash || '').slice(0, 7), 4000);
                state.gitCommitMessage = '';
                state.gitIdentityPrompt = false;
                const box = shellPart('[data-git-commit-msg]');
                if (box) box.value = '';
                refreshGitStatus();
            });
        } catch (err) {
            if (!canWrite(target)) return;
            runWithInstance(target, () => {
                if (err && err.status === 409 && err.message === 'git_identity_required') {
                    state.gitIdentityPrompt = true;
                    renderGitPanel();
                    return;
                }
                showGitError(err);
            });
        }
    }

    async function saveGitIdentity(formData) {
        const target = state;
        if (!canWrite(target)) return;
        const name = String(formData.get('name') || '').trim();
        const email = String(formData.get('email') || '').trim();
        state.gitIdentityName = name;
        state.gitIdentityEmail = email;
        if (!name || !email) {
            renderStatus(tr('codeStudio.gitIdentityHint', 'Git needs your name and email for this repository.'));
            return;
        }
        try {
            await apiClient.gitIdentity(name, email);
            if (!canWrite(target)) return;
            runWithInstance(target, () => {
                state.gitIdentityPrompt = false;
                renderGitPanel();
                renderStatus(tr('codeStudio.saveIdentity', 'Save identity'));
            });
        } catch (err) {
            if (isLiveInstance(target)) runWithInstance(target, () => renderStatus(err.message || String(err)));
        }
    }

    async function runGitPathAction(action, paths) {
        const target = state;
        const list = (paths || []).filter(Boolean);
        if (!canWrite(target) || !list.length) return false;
        const request = action === 'stage' ? apiClient.gitStage : action === 'unstage' ? apiClient.gitUnstage : apiClient.gitDiscard;
        try {
            await request(list);
        } catch (err) {
            if (isLiveInstance(target)) runWithInstance(target, () => {
                showGitError(err);
                refreshGitStatus();
            });
            return false;
        }
        if (!isLiveInstance(target)) return false;
        runWithInstance(target, () => {
            refreshGitStatus();
            if (action === 'discard') refreshGitWorkspaceView();
        });
        return true;
    }

    function stageGitPaths(paths) {
        return runGitPathAction('stage', paths);
    }

    function unstageGitPaths(paths) {
        return runGitPathAction('unstage', paths);
    }

    async function discardGitChange(path) {
        const target = state;
        if (!canWrite(target) || !path) return;
        const change = gitChangeGroups().changes.find(item => item.path === path);
        const message = change && gitChangeUntracked(change)
            ? tr('codeStudio.gitDiscardUntrackedConfirm', 'The untracked file {{name}} will be deleted. This cannot be undone.', { name: path })
            : tr('codeStudio.gitDiscardConfirm', 'Discard the changes in {{name}}? This cannot be undone.', { name: path });
        const confirmed = await confirmValue(message, { confirmLabel: tr('codeStudio.discard', 'Discard'), confirmIcon: 'x' });
        if (!confirmed || !canWrite(target)) return;
        await runWithInstance(target, () => runGitPathAction('discard', [path]));
    }

    async function initGitRepository() {
        const target = state;
        if (!canWrite(target)) return;
        try {
            await apiClient.gitInit();
        } catch (err) {
            if (isLiveInstance(target)) runWithInstance(target, () => {
                showGitError(err);
                refreshGitStatus();
            });
            return;
        }
        if (!isLiveInstance(target)) return;
        runWithInstance(target, () => {
            flashStatus(tr('codeStudio.gitInitialized', 'Initialized an empty Git repository'), 4000);
            refreshGitStatus();
        });
    }

    // Git's own message is shown verbatim: in full in the Output panel, its first line in the status bar.
    function showGitError(err) {
        const message = String(err && err.message ? err.message : err || '').trim();
        if (!message) return;
        appendOutput(message, 'git');
        setActivePanel('output');
        flashStatus(message.split('\n')[0], 8000);
    }

    // After a checkout or a discard the files on disk changed. Clean tabs read their file
    // again; reloadCleanTabFromDisk leaves tabs with unsaved edits alone. The tree is listed again.
    async function refreshGitWorkspaceView() {
        const target = state;
        if (!isLiveInstance(target)) return;
        target.openTabs.forEach(tab => { reloadCleanTabFromDisk(target, tab); });
        await runAsyncStep(target, () => refreshFiles(target.currentPath));
        for (const dir of [...target.expandedDirs]) {
            if (!isLiveInstance(target)) return;
            if (dir === target.currentPath) continue;
            try {
                const result = await apiClient.files(dir);
                if (!isLiveInstance(target)) return;
                target.treeCache[dir] = sortTreeEntries(result.files || []);
            } catch (_) {
                // The folder does not exist on this branch.
                delete target.treeCache[dir];
                target.expandedDirs.delete(dir);
            }
        }
        if (isLiveInstance(target)) runWithInstance(target, renderSidebar);
    }

    async function showGitBranchMenu(x, y) {
        const target = state;
        if (!isLiveInstance(target)) return;
        let result;
        try {
            result = await apiClient.gitBranch();
        } catch (err) {
            if (isLiveInstance(target)) runWithInstance(target, () => showGitError(err));
            return;
        }
        if (!isLiveInstance(target)) return;
        runWithInstance(target, () => {
            const current = String(result && result.current || '');
            const local = (result && Array.isArray(result.branches) ? result.branches : [])
                .map(name => String(name || '').trim())
                .filter(name => name && !name.startsWith('remotes/') && !name.startsWith('('));
            const writable = canWrite();
            const items = local.map((name, index) => ({
                id: 'git-branch-switch-' + index, label: name, icon: 'network', checked: name === current,
                disabled: !writable || name === current, action: bind(() => changeGitBranch(name, 'switch'))
            }));
            if (items.length) items.push({ separator: true });
            items.push({ id: 'git-branch-create', label: tr('codeStudio.gitCreateBranch', 'Create new branch...'), icon: 'file-plus', disabled: !writable, action: bind(createGitBranch) });
            // The current branch is never offered for deletion.
            const removable = local.filter(name => name !== current);
            if (removable.length) items.push({ separator: true });
            removable.forEach((name, index) => items.push({
                id: 'git-branch-delete-' + index, label: tr('codeStudio.gitDeleteBranch', 'Delete branch {{name}}', { name }), icon: 'trash',
                disabled: !writable, action: bind(() => deleteGitBranch(name))
            }));
            showStudioContextMenu(x, y, items);
        });
    }

    async function changeGitBranch(name, action) {
        const target = state;
        if (!canWrite(target) || !name) return false;
        try {
            const result = await apiClient.gitBranch(name, action);
            if (!isLiveInstance(target)) return false;
            runWithInstance(target, () => {
                const message = action === 'delete'
                    ? tr('codeStudio.gitBranchDeleted', 'Deleted branch {{name}}', { name })
                    : tr('codeStudio.gitSwitchedBranch', 'Switched to {{name}}', { name: result && result.current || name });
                flashStatus(message, 4000);
                refreshGitStatus();
                if (action !== 'delete') refreshGitWorkspaceView();
            });
            return true;
        } catch (err) {
            // Checkout errors such as "local changes would be overwritten" are shown verbatim.
            if (isLiveInstance(target)) runWithInstance(target, () => showGitError(err));
            return false;
        }
    }

    async function createGitBranch() {
        const target = state;
        if (!canWrite(target)) return;
        const name = String(await promptValue(tr('codeStudio.gitNewBranchName', 'New branch name'), '') || '').trim();
        if (!name || !canWrite(target)) return;
        await runWithInstance(target, () => changeGitBranch(name, 'create'));
    }

    async function deleteGitBranch(name) {
        const target = state;
        if (!canWrite(target) || !name || name === target.gitBranch) return;
        const confirmed = await confirmValue(tr('codeStudio.gitDeleteBranchConfirm', 'Delete branch {{name}}? Git refuses to delete branches that are not merged.', { name }));
        if (!confirmed || !canWrite(target)) return;
        await runWithInstance(target, () => changeGitBranch(name, 'delete'));
    }
```

- [ ] **Step 3: Create `ui/js/desktop/apps/code-studio/git-history.js`** with this complete content:

```js
    // Git history list, read-only commit tab and blame gutter of Code Studio. The Source Control
    // panel itself is in git.js; both fragments share the studio IIFE opened by core.js.

    let gitBlameMarkerType = null;
    let gitBlameMarkerBase = null;

    function gitRelativeDate(value) {
        const stamp = Date.parse(value || '');
        if (!Number.isFinite(stamp)) return '';
        const seconds = Math.round((stamp - Date.now()) / 1000);
        const units = [['year', 31536000], ['month', 2592000], ['week', 604800], ['day', 86400], ['hour', 3600], ['minute', 60]];
        try {
            const format = new Intl.RelativeTimeFormat(window.SYSTEM_LANG || document.documentElement.lang || undefined, { numeric: 'auto' });
            for (const [unit, size] of units) {
                if (Math.abs(seconds) >= size) return format.format(Math.round(seconds / size), unit);
            }
            return format.format(seconds, 'second');
        } catch (_) {
            return new Date(stamp).toLocaleDateString();
        }
    }

    function gitAbsoluteDate(value) {
        const stamp = Date.parse(value || '');
        return Number.isFinite(stamp) ? new Date(stamp).toLocaleString() : String(value || '');
    }

    function gitHistorySectionHtml() {
        const open = state.gitHistoryOpen !== false;
        const showAll = tr('codeStudio.gitHistoryShowAll', 'Show all commits');
        const filter = state.gitHistoryPath
            ? `<div class="cs-git-history-filter"><span title="${esc(state.gitHistoryPath)}">${esc(tr('codeStudio.gitHistoryFor', 'History of {{name}}', { name: baseName(state.gitHistoryPath) }))}</span><button type="button" data-git-history-clear title="${esc(showAll)}" aria-label="${esc(showAll)}">×</button></div>`
            : '';
        const entries = (state.gitHistory || []).map(entry => {
            const hash = String(entry.hash || '');
            const meta = [entry.author || '', gitRelativeDate(entry.date)].filter(Boolean).join(' · ');
            return `<button type="button" class="cs-git-history-entry" data-git-commit-hash="${esc(hash)}" title="${esc(entry.message || '')}">
                    <span class="cs-git-hash">${esc(entry.short || hash.slice(0, 7))}</span>
                    <span class="cs-git-history-subject">${esc(entry.message || '')}</span>
                    <span class="cs-git-history-meta">${esc(meta)}</span>
                </button>`;
        }).join('');
        const list = entries || `<div class="cs-git-empty">${esc(tr('codeStudio.gitNoCommits', 'No commits yet'))}</div>`;
        return `<div class="cs-git-section" data-git-section="history">
                <button type="button" class="cs-git-history-toggle" data-git-history-toggle aria-expanded="${open ? 'true' : 'false'}"><span class="cs-git-chevron" aria-hidden="true">▾</span><span>${esc(tr('codeStudio.gitHistory', 'History'))}</span></button>
                ${open ? filter + `<div class="cs-git-history-list">${list}</div>` : ''}
            </div>`;
    }

    function wireGitHistorySection(panel) {
        const toggle = panel.querySelector('[data-git-history-toggle]');
        if (toggle) toggle.addEventListener('click', bind(() => {
            state.gitHistoryOpen = state.gitHistoryOpen === false;
            renderGitPanel();
            if (state.gitHistoryOpen) refreshGitHistory();
        }));
        const clear = panel.querySelector('[data-git-history-clear]');
        if (clear) clear.addEventListener('click', bind(() => {
            state.gitHistoryPath = '';
            renderGitPanel();
            refreshGitHistory();
        }));
        panel.querySelectorAll('[data-git-commit-hash]').forEach(btn => {
            btn.addEventListener('click', bind(() => openGitCommit(btn.dataset.gitCommitHash)));
        });
    }

    async function refreshGitHistory() {
        const target = state;
        if (!isLiveInstance(target) || !target.gitVisible || target.gitHistoryOpen === false || target.gitIsRepo !== true) return;
        const request = target.gitHistoryRequest = Symbol();
        try {
            const result = await apiClient.gitLog(target.gitHistoryPath, 50);
            if (!isLiveInstance(target) || target.gitHistoryRequest !== request) return;
            runWithInstance(target, () => {
                state.gitHistory = Array.isArray(result.log) ? result.log : [];
                renderGitPanel();
            });
        } catch (_) {
            if (!isLiveInstance(target) || target.gitHistoryRequest !== request) return;
            runWithInstance(target, () => {
                state.gitHistory = [];
                renderGitPanel();
            });
        }
    }

    // Command "Git: File History": the History list shows the commits of the active file.
    function showGitFileHistory() {
        const tab = activeTab();
        if (!tab || isVirtualTab(tab)) return;
        state.gitHistoryPath = tab.path;
        state.gitHistoryOpen = true;
        if (!state.gitVisible) {
            toggleGitPanel();
            return;
        }
        renderGitPanel();
        refreshGitHistory();
    }

    async function openGitCommit(hash) {
        const target = state;
        const id = String(hash || '').trim();
        if (!isLiveInstance(target) || !/^[0-9a-fA-F]{4,40}$/.test(id) || /^0+$/.test(id)) return;
        let tab = target.openTabs.find(item => item.kind === 'commit' && item.hash === id);
        if (!tab) {
            tab = { kind: 'commit', hash: id, short: id.slice(0, 7), path: 'commit ' + id.slice(0, 7), content: '', modified: false, commit: null, loading: true, error: '' };
            target.openTabs.push(tab);
        }
        activateTab(target.openTabs.indexOf(tab));
        if (tab.commit) return;
        tab.loading = true;
        const request = tab.commitRequest = Symbol();
        try {
            const result = await apiClient.gitShow(id);
            if (!isLiveInstance(target) || !target.openTabs.includes(tab) || tab.commitRequest !== request) return;
            runWithInstance(target, () => {
                tab.commit = result;
                tab.loading = false;
                tab.short = result.short || tab.short;
                if (activeTab() === tab) renderEditor();
            });
        } catch (err) {
            if (!isLiveInstance(target) || !target.openTabs.includes(tab) || tab.commitRequest !== request) return;
            runWithInstance(target, () => {
                tab.loading = false;
                tab.error = err.message || String(err);
                if (activeTab() === tab) renderEditor();
            });
        }
    }

    function renderGitCommitTab(editor, tab) {
        const commit = tab.commit;
        const short = commit && commit.short || tab.short || String(tab.hash || '').slice(0, 7);
        let body;
        if (!commit) {
            body = `<div class="cs-diff-content">${esc(tab.loading ? tr('codeStudio.gitLoadingCommit', 'Loading commit...') : tab.error || '')}</div>`;
        } else {
            const files = Array.isArray(commit.files) ? commit.files : [];
            const filesHtml = files.map(file => `<div class="cs-git-commit-file"><span class="cs-git-status ${gitStatusClass(String(file.status || ''))}">${esc(file.status || '')}</span><span class="cs-git-filename">${esc(file.path || '')}</span></div>`).join('');
            body = `<div class="cs-git-commit-meta">
                    <div class="cs-git-commit-subject">${esc(commit.subject || '')}</div>
                    ${commit.body ? `<pre class="cs-git-commit-body">${esc(commit.body)}</pre>` : ''}
                    <div class="cs-git-commit-facts">
                        <span class="cs-git-hash">${esc(commit.hash || '')}</span>
                        <span>${esc(commit.author || '')}${commit.email ? ' &lt;' + esc(commit.email) + '&gt;' : ''}</span>
                        <span title="${esc(commit.date || '')}">${esc(gitAbsoluteDate(commit.date))}</span>
                    </div>
                    <div class="cs-git-commit-files">
                        <div class="cs-git-commit-files-head">${esc(tr('codeStudio.gitChangedFiles', 'Changed files ({{count}})', { count: files.length }))}</div>
                        ${filesHtml}
                    </div>
                </div>
                <div class="cs-diff-content">${commit.diff ? diffLinesHtml(commit.diff) : esc(tr('codeStudio.noChanges', 'No changes'))}${commit.truncated ? `<div class="cs-git-truncated">${esc(tr('codeStudio.gitDiffTruncated', 'The diff is too large and was cut off.'))}</div>` : ''}</div>`;
        }
        editor.innerHTML = `<div class="cs-diff-view" data-git-commit-view>
            <div class="cs-diff-view-head">
                <span class="cs-diff-view-title"><strong>${esc(short)}</strong><span class="cs-diff-badge">${esc(tr('codeStudio.commit', 'Commit'))} · ${esc(tr('codeStudio.readOnly', 'Read-only'))}</span></span>
                <span class="cs-diff-view-spacer"></span>
                <button type="button" class="cs-icon-button" data-diff-close aria-label="${esc(tr('desktop.close', 'Close'))}">×</button>
            </div>
            ${body}
        </div>`;
        editor.querySelector('[data-diff-close]').addEventListener('click', bind(() => closeTab(state.openTabs.indexOf(tab))));
    }

    function gitBlameUncommitted(entry) {
        return !entry || /^0+$/.test(String(entry.hash || ''));
    }

    // GutterMarker comes from the dynamically imported CodeMirror bundle, so the subclass is built on first use.
    function gitBlameMarkerClass(cm) {
        if (gitBlameMarkerType && gitBlameMarkerBase === cm.GutterMarker) return gitBlameMarkerType;
        gitBlameMarkerBase = cm.GutterMarker;
        gitBlameMarkerType = class GitBlameMarker extends cm.GutterMarker {
            constructor(entry, first) {
                super();
                this.entry = entry;
                this.first = first;
            }

            eq(other) {
                return other.entry === this.entry && other.first === this.first;
            }

            toDOM() {
                const element = document.createElement('div');
                const entry = this.entry;
                element.className = 'cs-blame-entry' + (this.first ? ' first' : '');
                const uncommitted = gitBlameUncommitted(entry);
                if (uncommitted) element.classList.add('uncommitted');
                element.title = uncommitted ? tr('codeStudio.gitNotCommitted', 'Not committed yet') : String(entry.summary || '');
                if (this.first) {
                    element.textContent = uncommitted
                        ? tr('codeStudio.gitNotCommitted', 'Not committed yet')
                        : [entry.author || '', gitRelativeDate(entry.date), entry.short || String(entry.hash || '').slice(0, 7)].filter(Boolean).join(' · ');
                }
                return element;
            }
        };
        return gitBlameMarkerType;
    }

    // Content of the per-view blame compartment: a gutter that shows author, relative date and
    // short hash on the first line of each run of lines from one commit; a click opens the commit.
    function gitBlameExtensions(cm, tab) {
        if (!cm || typeof cm.gutter !== 'function' || typeof cm.GutterMarker !== 'function') return [];
        if (!tab || !tab.blameOn || !Array.isArray(tab.blameLines)) return [];
        const Marker = gitBlameMarkerClass(cm);
        const byLine = [];
        tab.blameLines.forEach(entry => {
            if (entry && entry.line > 0) byLine[entry.line] = entry;
        });
        const markers = [];
        const markerAt = number => {
            const entry = byLine[number];
            if (!entry) return null;
            if (!markers[number]) {
                const previous = byLine[number - 1];
                markers[number] = new Marker(entry, !previous || previous.hash !== entry.hash);
            }
            return markers[number];
        };
        const instance = state;
        return [cm.gutter({
            class: 'cs-blame-gutter',
            lineMarker: (view, line) => markerAt(view.state.doc.lineAt(line.from).number),
            lineMarkerChange: update => update.docChanged,
            domEventHandlers: {
                click: bindInstance(instance, (view, line) => {
                    const entry = byLine[view.state.doc.lineAt(line.from).number];
                    if (!entry || gitBlameUncommitted(entry)) return false;
                    openGitCommit(entry.hash);
                    return true;
                })
            }
        })];
    }

    function applyGitBlame(tab) {
        const cm = state.cmModule;
        if (!cm || !tab) return;
        (tab.views || []).forEach(view => {
            if (!view || !view.csBlame) return;
            try { view.dispatch({ effects: view.csBlame.reconfigure(gitBlameExtensions(cm, tab)) }); } catch (_) {}
        });
    }

    // Command "Git: Toggle Blame" for the active editor.
    function toggleGitBlame() {
        const tab = activeTab();
        if (!tab || isVirtualTab(tab)) return undefined;
        tab.blameOn = !tab.blameOn;
        if (tab.blameOn) return refreshGitBlame(tab);
        tab.blameRequest = null;
        tab.blameLines = null;
        applyGitBlame(tab);
        return undefined;
    }

    async function refreshGitBlame(tab) {
        const target = state;
        if (!isLiveInstance(target) || !tab || !tab.blameOn || isVirtualTab(tab)) return;
        const path = tab.path;
        const request = tab.blameRequest = Symbol();
        try {
            const result = await apiClient.gitBlame(path);
            if (!isLiveInstance(target) || !target.openTabs.includes(tab) || tab.blameRequest !== request || tab.path !== path || !tab.blameOn) return;
            runWithInstance(target, () => {
                tab.blameLines = Array.isArray(result.lines) ? result.lines : [];
                applyGitBlame(tab);
            });
        } catch (err) {
            if (!isLiveInstance(target) || tab.blameRequest !== request) return;
            runWithInstance(target, () => {
                tab.blameOn = false;
                tab.blameLines = null;
                applyGitBlame(tab);
                showGitError(err);
            });
        }
    }
```

- [ ] **Step 4: Render commit tabs and add the blame compartment** in `ui/js/desktop/apps/code-studio/editor.js`.

First edit, `renderEditor`. If A restructured this block, put the same `isVirtualTab` branch wherever A renders a diff tab.

Replace:
```js
        if (tab.kind === 'diff') {
            renderGitDiffTab(editor, tab);
```
With:
```js
        if (isVirtualTab(tab)) {
            if (tab.kind === 'commit') renderGitCommitTab(editor, tab);
            else renderGitDiffTab(editor, tab);
```

Second edit, in `createCodeMirrorEditor`. This line is kept by A, which adds its settings compartment elsewhere.

Replace:
```js
        const policy = new cm.Compartment();
```
With:
```js
        const policy = new cm.Compartment();
        // Blame gutter of git-history.js, switched per view without rebuilding the editor.
        const blame = new cm.Compartment();
```

Third edit. This is the only `const extensions = [` in `editor.js`. The blame gutter comes first so it sits left of the line numbers.

Replace:
```js
        const extensions = [
```
With:
```js
        const extensions = [
            blame.of(gitBlameExtensions(cm, tab)),
```

Fourth edit:

Replace:
```js
        view.csPolicy = policy;
```
With:
```js
        view.csPolicy = policy;
        view.csBlame = blame;
```

- [ ] **Step 5: State fields, the commit wrapper and the policy selector** in `ui/js/desktop/apps/code-studio/core.js`.

Replace:
```js
            gitSelected: new Set(),
```
With:
```js
            gitIsRepo: null,
            gitHistory: [],
            gitHistoryOpen: true,
            gitHistoryPath: '',
            gitHistoryRequest: null,
```

Replace:
```js
        gitCommit: (message, paths) => api('/api/code-studio/git/commit', {
            method: 'POST',
            body: JSON.stringify({ message, add_all: false, paths: paths || [] })
        }),
```
With:
```js
        // Commits the index as it is; addAll stages every change first, after the user confirmed it.
        gitCommit: (message, options) => api('/api/code-studio/git/commit', {
            method: 'POST',
            body: JSON.stringify({ message, add_all: !!(options && options.addAll), staged_only: !(options && options.addAll), paths: [] })
        }),
```

Replace:
```js
[data-git-commit], [data-git-select], [data-git-identity] input
```
With:
```js
[data-git-commit], [data-git-write], [data-git-identity] input
```

- [ ] **Step 6: Branch item in the status bar** (`renderStatus` in `core.js`). Plan A adds an indent item to `rightItems` and a click handler of its own. Both anchors below are existing lines that A keeps.

Replace:
```js
        leftItems.push(`<span class="cs-status-item cs-status-container" title="${esc(containerStatusLabel())}"><span class="cs-status-dot" data-status="${esc(state.containerStatus || 'unknown')}"></span>${esc(containerStatusLabel())}</span>`);
```
With:
```js
        leftItems.push(`<span class="cs-status-item cs-status-container" title="${esc(containerStatusLabel())}"><span class="cs-status-dot" data-status="${esc(state.containerStatus || 'unknown')}"></span>${esc(containerStatusLabel())}</span>`);
        if (state.gitIsRepo === true) leftItems.push(`<span class="cs-status-item cs-status-branch" data-clickable data-status-branch title="${esc(tr('codeStudio.gitBranchMenu', 'Switch, create or delete branches'))}">${gitBranchIconMarkup(12)}<span>${esc(state.gitBranch || tr('codeStudio.gitDetached', 'Detached HEAD'))}</span></span>`);
```

Replace:
```js
        if (problems) problems.addEventListener('click', bind(() => setActivePanel('problems')));
```
With:
```js
        if (problems) problems.addEventListener('click', bind(() => setActivePanel('problems')));
        const branch = statusEl.querySelector('[data-status-branch]');
        if (branch) branch.addEventListener('click', bind(() => {
            const rect = branch.getBoundingClientRect();
            showGitBranchMenu(rect.left, rect.top - 4);
        }));
```

- [ ] **Step 7: Palette commands** in `ui/js/desktop/apps/code-studio/command-palette.js` (`getCommands`). The inserted lines are independent of A's commands.

Replace:
```js
            { id: 'toggle-git', label: tr('codeStudio.gitPanel', 'Source Control'), icon: 'network', run: 'toggleGitPanel' },
```
With:
```js
            { id: 'toggle-git', label: tr('codeStudio.gitPanel', 'Source Control'), icon: 'network', run: 'toggleGitPanel' },
            { id: 'git-toggle-blame', label: tr('codeStudio.gitToggleBlame', 'Git: Toggle Blame'), icon: 'info', run: 'toggleGitBlame', disabled: !sourceTab },
            { id: 'git-file-history', label: tr('codeStudio.gitFileHistory', 'Git: File History'), icon: 'list', run: 'showGitFileHistory', disabled: !sourceTab },
```

- [ ] **Step 8: Command table** in `ui/js/desktop/apps/code-studio/shortcuts.js` (`studioCommandTable`):

Replace:
```js
            toggleGitPanel,
```
With:
```js
            toggleGitPanel,
            toggleGitBlame,
            showGitFileHistory,
```

- [ ] **Step 9: Bundle the new fragment** in `scripts/build-ui-bundles.js`:

Replace:
```js
      'ui/js/desktop/apps/code-studio/git.js',
```
With:
```js
      'ui/js/desktop/apps/code-studio/git.js',
      'ui/js/desktop/apps/code-studio/git-history.js',
```

- [ ] **Step 10: Rewrite `ui/code_studio_git_selection_test.go`.** The old checkbox markers no longer apply. Replace the whole file with this complete content:

```go
package ui

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"unicode/utf8"
)

// The Git workbench stages explicitly: every staging action is a POST of explicit paths, commit
// sends staged_only, and add_all is only sent after the "stage all and commit" confirmation.
func TestCodeStudioGitStagingSourceAndLocales(t *testing.T) {
	read := func(name string) string {
		t.Helper()
		raw, err := os.ReadFile(filepath.Join("js", "desktop", "apps", "code-studio", name))
		if err != nil {
			t.Fatal(err)
		}
		return string(raw)
	}
	markers := map[string][]string{
		"git.js":         {"data-git-stage", "data-git-unstage", "data-git-discard", "data-git-init", "codeStudio.gitStageAllAndCommit", "codeStudio.gitIdentityHint", "git_identity_required", "reloadCleanTabFromDisk"},
		"git-history.js": {"cs-blame-gutter", "openGitCommit", "codeStudio.gitNotCommitted", "csBlame"},
		"core.js":        {"staged_only: !(options && options.addAll)", "action === 'create'", "method: 'POST'", "/api/code-studio/git/branch", "/api/code-studio/git/stage", "/api/code-studio/git/discard", "function isVirtualTab(tab)"},
		"editor.js":      {"view.csBlame = blame;", "blame.of(gitBlameExtensions(cm, tab))"},
	}
	for name, wanted := range markers {
		source := read(name)
		for _, marker := range wanted {
			if !strings.Contains(source, marker) {
				t.Errorf("%s lacks %q", name, marker)
			}
		}
	}
	if strings.Contains(read("git.js"), "data-git-select") {
		t.Error("git.js still renders commit checkboxes")
	}

	load := func(lang string) map[string]string {
		t.Helper()
		raw, err := os.ReadFile(filepath.Join("lang", "desktop", lang+".json"))
		if err != nil {
			t.Fatal(err)
		}
		var values map[string]string
		if err := json.Unmarshal(raw, &values); err != nil {
			t.Fatalf("%s: %v", lang, err)
		}
		return values
	}
	english := load("en")
	keys := []string{"codeStudio.gitStagedChanges", "codeStudio.gitStageAllAndCommit", "codeStudio.gitInitRepo", "codeStudio.gitDiscardUntrackedConfirm", "codeStudio.gitNotCommitted", "codeStudio.gitIdentityHint"}
	for _, lang := range []string{"cs", "da", "de", "el", "en", "es", "fr", "hi", "it", "ja", "nl", "no", "pl", "pt", "sv", "zh"} {
		values := load(lang)
		for _, key := range keys {
			value := strings.TrimSpace(values[key])
			if value == "" || strings.ContainsRune(value, utf8.RuneError) {
				t.Errorf("%s: %s missing or broken", lang, key)
			}
			if lang != "en" && value == english[key] {
				t.Errorf("%s copies the English %s", lang, key)
			}
			if lang == "de" {
				for _, formal := range []string{" Sie ", " Ihre ", " Ihnen "} {
					if strings.Contains(" "+value+" ", formal) {
						t.Errorf("de: %s uses formal address", key)
					}
				}
			}
		}
		if !strings.Contains(values["codeStudio.gitDiscardUntrackedConfirm"], "{{name}}") {
			t.Errorf("%s: gitDiscardUntrackedConfirm must name the file", lang)
		}
	}
}
```

- [ ] **Step 11: Verify the routing again.** Grep for `kind ?[!=]== ?'diff'` in `ui/js/desktop/apps/code-studio`. Expected hits only: `core.js` in `isVirtualTab` and `studioTabLabel`, and `git.js` in the `openGitDiff` lookup.

- [ ] **Step 12: Build and test.**

Run: `npm run build:ui`
Then: `node --check ui/js/desktop/bundles/code-studio.bundle.js`
Expected: no output from `node --check`.

Run: `go test ./ui -count=1 -run "(?i)(CodeStudioGitStaging|DesktopUsedI18nKeysExistInAllLanguages|DesktopCodeStudioShellI18n)"`
Expected: PASS. In particular, `TestDesktopUsedI18nKeysExistInAllLanguages` finds every new `tr('codeStudio.git…')` key in all 16 locales.

Commit: `feat(code-studio): staging groups, history, commit tab, blame and branch menu`. Files: `ui/js/desktop/apps/code-studio/git.js`, `ui/js/desktop/apps/code-studio/git-history.js`, `ui/js/desktop/apps/code-studio/core.js`, `ui/js/desktop/apps/code-studio/editor.js`, `ui/js/desktop/apps/code-studio/command-palette.js`, `ui/js/desktop/apps/code-studio/shortcuts.js`, `scripts/build-ui-bundles.js`, `ui/code_studio_git_selection_test.go`, `ui/js/desktop/bundles/code-studio.bundle.js`

---

### Task 8: Git workbench styles and the browser test

**Files:**
- Modify: `ui/css/code-studio.css`
- Create: `ui/desktop_code_studio_git_browser_test.go`

- [ ] **Step 1: Append to the end of `ui/css/code-studio.css`.** Plan A also appends to this file, and the order of the two sections does not matter.

```css

/* === IDE package 1B: git === */

/* Branch button at the top of the Source Control panel. */
button.cs-git-branch {
    width: 100%;
    border: 0;
    border-bottom: 1px solid var(--cs-line);
    background: transparent;
    font: 12px var(--cs-ui-font);
    text-align: left;
    cursor: pointer;
}

.cs-git-branch + .cs-git-commit-area {
    border-top: 0;
}

button.cs-git-branch:hover,
.cs-git-history-toggle:hover,
.cs-git-history-entry:hover {
    background: var(--cs-hover);
}

.cs-git-section-title {
    min-width: 0;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
}

.cs-git-section-actions {
    display: inline-flex;
    gap: 2px;
    margin-left: auto;
}

/* Stage, unstage and discard: shown on hover and keyboard focus, always on touch screens. */
.cs-git-row-actions {
    display: inline-flex;
    flex: 0 0 auto;
    gap: 2px;
    padding-right: 6px;
    opacity: 0;
    transition: opacity var(--cs-transition-fast);
}

.cs-git-change-row:hover .cs-git-row-actions,
.cs-git-change-row:focus-within .cs-git-row-actions {
    opacity: 1;
}

@media (hover: none) {
    .cs-git-row-actions {
        opacity: 1;
    }
}

.cs-git-row-action {
    display: inline-flex;
    align-items: center;
    justify-content: center;
    width: 22px;
    height: 22px;
    padding: 0;
    border: 0;
    border-radius: var(--ds-radius-xs);
    background: transparent;
    color: var(--cs-muted);
    font: 600 14px/1 var(--cs-ui-font);
    letter-spacing: 0;
    text-transform: none;
    cursor: pointer;
}

.cs-git-row-action:hover:not(:disabled),
.cs-git-row-action:focus-visible {
    background: var(--cs-hover);
    color: var(--cs-text);
}

.cs-git-row-action:disabled {
    opacity: 0.4;
    cursor: default;
}

.cs-git-dir {
    flex: 0 1 auto;
    min-width: 0;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    color: var(--cs-muted);
    font-size: 11px;
}

.cs-git-init {
    display: flex;
    flex-direction: column;
    align-items: flex-start;
    gap: 10px;
    padding: 16px 12px;
    color: var(--cs-muted);
    font-size: 12px;
}

.cs-git-init p {
    margin: 0;
}

/* History */
.cs-git-history-toggle {
    display: flex;
    align-items: center;
    gap: 6px;
    width: 100%;
    padding: 6px 10px;
    border: 0;
    background: transparent;
    color: var(--vd-mat-label-color, var(--cs-muted));
    font: 700 11px var(--vd-mat-label-font, var(--cs-ui-font));
    letter-spacing: var(--vd-mat-label-tracking, 0.5px);
    text-transform: var(--vd-mat-label-transform, uppercase);
    text-align: left;
    cursor: pointer;
}

.cs-git-chevron {
    display: inline-block;
    transition: transform var(--cs-transition-fast);
}

.cs-git-history-toggle[aria-expanded="false"] .cs-git-chevron {
    transform: rotate(-90deg);
}

.cs-git-history-filter {
    display: flex;
    align-items: center;
    gap: 6px;
    margin: 0 10px 4px;
    padding: 2px 4px 2px 8px;
    border: 1px solid var(--cs-border);
    border-radius: 999px;
    color: var(--cs-muted);
    font-size: 11px;
}

.cs-git-history-filter span {
    flex: 1;
    min-width: 0;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
}

.cs-git-history-filter button {
    padding: 0 4px;
    border: 0;
    background: transparent;
    color: inherit;
    font: inherit;
    cursor: pointer;
}

.cs-git-history-list {
    display: flex;
    flex-direction: column;
    padding-bottom: 8px;
}

.cs-git-history-entry {
    display: grid;
    grid-template-columns: auto minmax(0, 1fr);
    column-gap: 6px;
    width: 100%;
    padding: 4px 10px;
    border: 0;
    background: transparent;
    color: var(--cs-text);
    font: 12px var(--cs-ui-font);
    text-align: left;
    cursor: pointer;
}

.cs-git-history-subject,
.cs-git-history-meta {
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
}

.cs-git-history-meta {
    grid-column: 2;
    color: var(--cs-muted);
    font-size: 11px;
}

/* Commit tab */
.cs-git-commit-meta {
    display: flex;
    flex-direction: column;
    gap: 8px;
    padding: 12px;
    border-bottom: 1px solid var(--cs-line);
}

.cs-git-commit-subject {
    color: var(--cs-text);
    font-size: 14px;
    font-weight: 600;
}

.cs-git-commit-body {
    margin: 0;
    color: var(--cs-text);
    font: 12px/1.5 var(--cs-mono-font);
    white-space: pre-wrap;
}

.cs-git-commit-facts {
    display: flex;
    flex-wrap: wrap;
    gap: 4px 12px;
    color: var(--cs-muted);
    font-size: 12px;
}

.cs-git-commit-files {
    display: flex;
    flex-direction: column;
    gap: 2px;
}

.cs-git-commit-files-head {
    color: var(--cs-muted);
    font-size: 11px;
    font-weight: 600;
}

.cs-git-commit-file {
    display: flex;
    align-items: center;
    gap: 8px;
    min-width: 0;
    color: var(--cs-text);
    font-size: 12px;
}

/* Light themes: the warning colour tinted into the ink, as the diff lines do with accent and danger. */
.cs-git-truncated {
    padding: 6px 8px;
    color: light-dark(color-mix(in srgb, var(--cs-warning) 30%, var(--vd-text)), var(--cs-warning));
    font: 12px var(--cs-ui-font);
}

/* Blame gutter (git-history.js) */
.code-studio .cm-gutter.cs-blame-gutter {
    width: 220px;
    border-right: 1px solid var(--cs-border-subtle);
    cursor: pointer;
}

.code-studio .cm-gutter.cs-blame-gutter .cm-gutterElement:hover {
    background: var(--cs-hover);
}

.cs-blame-entry {
    overflow: hidden;
    padding: 0 8px;
    color: var(--cs-muted);
    font-family: var(--cs-ui-font);
    font-size: 0.9em;
    text-overflow: ellipsis;
    white-space: nowrap;
}

.cs-blame-entry.first {
    color: var(--cs-text);
}

.cs-blame-entry.uncommitted {
    color: var(--cs-muted);
    font-style: italic;
}

/* Branch item in the status bar */
.code-studio-statusbar .cs-status-branch svg {
    flex: 0 0 auto;
    opacity: 0.8;
}
```

- [ ] **Step 2: Create `ui/desktop_code_studio_git_browser_test.go`** with this complete content:

```go
package ui

import "testing"

// TestDesktopCodeStudioGitWorkbenchBrowser drives the Source Control panel against stubbed git
// endpoints: stage and unstage requests, a staged-only commit, the history list, the commit tab
// and the blame gutter (including a click from blame into the commit tab).
func TestDesktopCodeStudioGitWorkbenchBrowser(t *testing.T) {
	page, _ := newCodeStudioBrowser(t)
	result, err := page.Eval(`async()=>{
        let step='boot';
        const fetchBefore=window.fetch,posts=[];
        try {
            await csWait(()=>csState()?.terminalSessions.length,20000);
            const app=CodeStudioApp,target=csState(),id=target.windowId,root=csRoot();
            const hash='0123456789abcdef0123456789abcdef01234567',zero='0000000000000000000000000000000000000000';
            const changes=[{path:'main.go',status:'M',staged:'true',index:'M',worktree:'M'},{path:'src/util/format.py',status:'??',staged:'false',index:'?',worktree:'?'}];
            const json=value=>Promise.resolve(new Response(JSON.stringify(value),{headers:{'Content-Type':'application/json'}}));
            window.fetch=(url,opts)=>{
                const path=String(url);
                if(opts&&opts.method==='POST'&&path.startsWith('/api/code-studio/git/'))posts.push({path,body:JSON.parse(opts.body||'{}')});
                if(path==='/api/code-studio/git/status')return json({status:'ok',is_repo:true,branch:'main',changes,log:[]});
                if(path.startsWith('/api/code-studio/git/log?'))return json({status:'ok',log:[{hash,short:'0123456',author:'Ada Lovelace',date:'2026-11-20T10:00:00+01:00',message:'Add engine'}]});
                if(path.startsWith('/api/code-studio/git/show?'))return json({status:'ok',hash,short:'0123456',author:'Ada Lovelace',email:'ada@example.com',date:'2026-11-20T10:00:00+01:00',subject:'Add engine',body:'Longer body',files:[{path:'main.go',status:'M'}],diff:'diff --git a/main.go b/main.go\n@@ -1 +1 @@\n-old\n+new\n',truncated:false});
                if(path.startsWith('/api/code-studio/git/blame?'))return json({status:'ok',lines:[{line:1,hash,short:'0123456',author:'Ada Lovelace',date:'2026-11-20T10:00:00+01:00',summary:'Add engine'},{line:2,hash:zero,short:'0000000',author:'Not Committed Yet',date:'2026-11-23T10:00:00+01:00',summary:'Version of main.go from main.go'}]});
                if(['/api/code-studio/git/stage','/api/code-studio/git/unstage','/api/code-studio/git/commit'].includes(path))return json({status:'ok',hash:'deadbee'});
                return fetchBefore(url,opts);
            };
            step='open panel';
            await app.openFile('/workspace/main.go',true,id);
            app.command('toggleGitPanel',[],id);
            await csWait(()=>root.querySelector('[data-git-section="staged"] [data-git-file="main.go"]')&&root.querySelector('[data-git-section="changes"] [data-git-file="main.go"]'));
            const bothGroups=!!root.querySelector('[data-git-section="changes"] [data-git-file="src/util/format.py"]')&&!root.querySelector('[data-git-select]');
            step='stage';
            root.querySelector('[data-git-section="changes"] [data-git-stage="src/util/format.py"]').click();
            await csWait(()=>posts.some(p=>p.path.endsWith('/git/stage')));
            step='unstage';
            root.querySelector('[data-git-section="staged"] [data-git-unstage="main.go"]').click();
            await csWait(()=>posts.some(p=>p.path.endsWith('/git/unstage')));
            const bodyOf=suffix=>JSON.stringify(posts.find(p=>p.path.endsWith(suffix)).body);
            const staging=bodyOf('/git/stage')==='{"paths":["src/util/format.py"]}'&&bodyOf('/git/unstage')==='{"paths":["main.go"]}';
            step='commit';
            const message=root.querySelector('[data-git-commit-msg]');message.value='Ship it';message.dispatchEvent(new Event('input'));
            root.querySelector('[data-git-commit]').click();
            await csWait(()=>posts.some(p=>p.path.endsWith('/git/commit')));
            const commitBody=posts.find(p=>p.path.endsWith('/git/commit')).body;
            const stagedOnly=commitBody.staged_only===true&&commitBody.add_all===false&&commitBody.message==='Ship it'&&!document.querySelector('.cs-modal-backdrop');
            step='history';
            await csWait(()=>root.querySelector('[data-git-commit-hash]'));
            const history=root.querySelectorAll('[data-git-commit-hash]').length===1&&root.querySelector('[data-git-commit-hash] .cs-git-hash').textContent==='0123456';
            step='commit tab';
            root.querySelector('[data-git-commit-hash]').click();
            await csWait(()=>root.querySelector('[data-git-commit-view] .cs-diff-line.added'));
            const view=root.querySelector('[data-git-commit-view]');
            const commitTab=target.openTabs[target.activeTabIndex].kind==='commit'&&view.textContent.includes('Add engine')&&view.textContent.includes('Longer body');
            const saved=JSON.parse(localStorage.getItem('aurago.codeStudio.state.v1')).openTabs;
            const commitIsVirtual=app.isVirtualTab(target.openTabs[target.activeTabIndex])&&saved.length===1&&saved[0].path==='/workspace/main.go';
            step='blame';
            await app.openFile('/workspace/main.go',true,id);
            await app.command('toggleGitBlame',[],id);
            await csWait(()=>root.querySelectorAll('.cs-blame-gutter .cs-blame-entry.first').length>=2);
            const entries=[...root.querySelectorAll('.cs-blame-gutter .cs-blame-entry.first')];
            const blame=entries[0].textContent.includes('0123456')&&entries[0].title==='Add engine'&&entries[1].textContent===t('codeStudio.gitNotCommitted');
            step='blame click';
            const firstEntry=root.querySelector('.cs-blame-gutter .cs-blame-entry.first'),box=firstEntry.getBoundingClientRect();
            firstEntry.dispatchEvent(new MouseEvent('click',{bubbles:true,cancelable:true,clientX:box.left+box.width/2,clientY:box.top+box.height/2}));
            await csWait(()=>target.openTabs[target.activeTabIndex]?.kind==='commit');
            return {bothGroups,staging,stagedOnly,history,commitTab,commitIsVirtual,blame,blameClick:true,errors:fixtureErrors};
        } catch (err) {
            return {step,error:String(err&&err.message||err),errors:fixtureErrors};
        } finally {window.fetch=fetchBefore;}
    }`)
	if err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"bothGroups", "staging", "stagedOnly", "history", "commitTab", "commitIsVirtual", "blame", "blameClick"} {
		if !result.Value.Get(key).Bool() {
			t.Fatalf("Git workbench check %q failed: %s", key, result.Value.JSON("", "  "))
		}
	}
	if len(result.Value.Get("errors").Arr()) != 0 {
		t.Fatalf("browser errors: %s", result.Value.JSON("", "  "))
	}
}
```

- [ ] **Step 3: Full verification.**

Run: `npm run build:ui -- --check`
Expected: `Checked ui/js/desktop/bundles/code-studio.bundle.js (15 parts)`. If plan A added fragments, the part count is higher. There must be no `Outdated` line for the code-studio bundle.

Run: `go test ./internal/server -count=1 -run "(?i)(CodeStudioGit|CodeStudioReadonly)"`
Expected: PASS.

Run: `go test ./ui -count=1 -run "(?i)(CodeStudioGitStaging|DesktopUsedI18nKeysExistInAllLanguages|DesktopCodeStudioShellI18n)"`
Expected: PASS.

Run the browser tests (PowerShell). They are gated by `AURAGO_RUN_BROWSER_SMOKE=1` and need Chrome or Edge:
`$env:AURAGO_RUN_BROWSER_SMOKE='1'; go test ./ui -count=1 -timeout 30m -run "(?i)(DesktopCodeStudioGitWorkbench|DesktopCodeStudioGitDiffPath|DesktopCodeStudioDiffTabPreservesSource|DesktopCodeStudioLivePolicyAndDrafts|DesktopCodeStudioMenuIconsBrowser|DesktopCodeStudioBrowser$)"; Remove-Item Env:AURAGO_RUN_BROWSER_SMOKE`
In Bash, the same run is `AURAGO_RUN_BROWSER_SMOKE=1 go test ./ui -count=1 -timeout 30m -run '<same pattern>'`.
Expected: PASS for these tests:
- `TestDesktopCodeStudioGitWorkbenchBrowser` (new)
- `TestDesktopCodeStudioGitDiffPath`: `[data-git-file]` keeps the exact path
- `TestDesktopCodeStudioDiffTabPreservesSource`: the `staged` sequence is `false,true`; the diff tab is never saved or run
- `TestDesktopCodeStudioLivePolicyAndDrafts`: `[data-git-commit]` is disabled in read-only mode and the commit draft is kept
- `TestDesktopCodeStudioMenuIconsBrowser`: the new palette commands use the existing `info` and `list` icons
- `TestDesktopCodeStudioBrowser`: `.cs-git-change` opens the diff, and the badge still shows `2`

To check light and dark readability, optionally set `AURAGO_BROWSER_ARTIFACT_DIR` and look at `cs-04-git.png` from `TestDesktopCodeStudioBrowser`.

Commit: `feat(code-studio): style the Git workbench and cover it in a browser test`. Files: `ui/css/code-studio.css`, `ui/desktop_code_studio_git_browser_test.go`