# Plan A: Code Studio SSH workspace backend

**Goal.** Code Studio can work on a folder on an SSH host as well as on the local container. Every `/api/code-studio/*` request picks its workspace with the header `X-Code-Studio-Workspace: <id>` or the query `ws=<id>`. A missing id, or `local`, means the container, and that path stays byte-for-byte compatible. An SSH workspace serves files over SFTP, search over remote `grep`, exec and git over SSH sessions, and the terminal over an SSH PTY. Plan A also adds the workspace store and API, the remote devices list/create endpoints, the folder browser, the host-key 409 flow and the capabilities probe. Go only, no UI.

**Spec.** `docs/superpowers/specs/2026-11-25-code-studio-ssh-workspace-preview-design.md`, section 7 "A".

**Worktree.** `C:\Users\Andi\Documents\repo\AuraGo\.claude\worktrees\cs-ssh-preview`. Use absolute paths in tools.

## Architecture summary

- **`codeStudioBackend` interface** (`code_studio_backend.go`). Handlers call `b, err := h.backendFor(r)` exactly where they call `h.codeContainer(...)` today, and then only call backend methods.
  - Virtual paths stay `/workspace/...`.
  - Validation keeps its current order: path sanitising and JSON errors still come before the backend is resolved, so local status codes and bodies stay the same.
- **`codeStudioLocalBackend`** (`code_studio_backend_local.go`) holds the handler copy that `codeContainer` already fitted with the Docker adapter, plus the container and its id. It delegates to the existing helpers (`probeCodeStudioFile`, `readCodeStudioFileBytes`, `writeContainerFile`, `execGit`, `codeStudioPathCommand`, `buildCodeStudioSearchCommand`), so every Docker command vector stays identical and the existing fakes keep working.
- **Two new error types:**
  - `codeStudioStatusError{Status, Message}` stands for "non-zero exit, show the output". It is written as `{"error": msg}` with its status, which equals today's `jsonError(w, strings.TrimSpace(out), 400 or 409)`.
  - `codeStudioRemoteError{Status, Code, ...}` is used for workspace and SSH contract errors (404, 409 host key, 403, 502).
  - Any other error is written as 502 `err.Error()`, as today.
- **`codeStudioSSHBackend`** (`code_studio_ssh_backend.go`) runs on a per-server `codeStudioRemote` that holds a `codeStudioSSHPool` (`code_studio_ssh_pool.go`).
  - The pool keeps one `*ssh.Client` plus one `*sftp.Client` per inventory device.
  - Connections get a keepalive every 30 s and are closed after 10 min idle (never while a terminal or process is open).
  - If the connection turns out to be dead, the operation redials and retries once. This applies to idempotent operations only, and to exec only when the session could not be opened.
  - Concurrency is capped per connection at 4 short exec slots and 4 long slots (terminals and processes), which stays below OpenSSH's default `MaxSessions` of 10 (the SFTP subsystem uses one session).
- **Saved SSH workspaces live in a table `code_studio_workspaces` in the existing desktop SQLite DB** (`svc.DB()`), created lazily with `CREATE TABLE IF NOT EXISTS`. Why this and not a JSON file:
  - The DB is already the desktop's persistent state, it is per instance and it is covered by desktop backup and restore.
  - It is transactional: no file locking or atomic-rename code is needed.
  - `UNIQUE(device_id, root)` gives the spec's "update the existing entry for the same device and root" via an UPSERT.
  - Tests already get a real DB from `testCodeStudioServerWithFakeCodeContainer`.
- **SSH access and host keys** go through a new exported helper file `internal/desktop/ssh_workspace_dial.go`:
  - `ResolveSSHDevice` wraps `resolveSSHAccess`.
  - `DescribeSSHDevice` gives host, port and user without reading the vault.
  - `DialSSH` builds a non-interactive known_hosts callback. An unknown key returns `*SSHHostKeyError{Code:"unknown_host_key"}` unless `TrustFingerprint` matches the presented key; only then is the key appended with the existing `appendKnownHostKey`. A changed key returns `host_key_mismatch` and Code Studio cannot override it.
  - `remote.InsecureHostKey` is still honoured.
  - No file of the other session is edited.
- **Remote-access gate.** SSH workspaces are remote access. A bearer token needs `desktop:admin` (checked with `currentTokenManager().Validate`). Cookie sessions pass after the usual `requirePermission`. Opening a workspace and creating a device are written to the desktop audit log.
- **Hook for Plan B.** `h.notifyWorkspaceChange(wsID, paths...)` calls hooks registered with `registerCodeStudioWorkspaceChangeHook`; with none registered it does nothing. Handlers call it after write, upload, move, delete, mkdir, git discard and git switch/create.
- **LSP for SSH workspaces is Plan D.** In Plan A, `handleLSP` answers 501 `lsp_unavailable` before the WebSocket upgrade. The local LSP path is unchanged. `StartProcess` (an SSH stdio process) is already provided for Plan D.
- **Terminal.** Local keeps today's path exactly: `HoldTerminal`, Docker TTY, line fallback. SSH opens a PTY session that implements `containerTerminalSession` and `containerTerminalEnder` and is served by the existing `serveContainerTerminalSession`, so the WebSocket protocol is the same (binary I/O, `{"type":"resize"}`, `{"type":"end"}`). Closing the WebSocket ends the SSH shell; there is no tmux-style survival.
- **Confinement is lexical plus `RealPath`.** It is a UX guard, not a security boundary, because the same SSH user has a full shell. Say so in a code comment.

## Hard constraints (repeat these to every implementer)

- **Do not modify these files** (another session has uncommitted work in them):
  - `internal/desktop/remote_files*.go`
  - `internal/desktop/sftp_handlers.go`
  - `internal/server/device_handlers.go`
  - `internal/server/device_quick_connect.go`
  - `internal/server/server_routes.go`
  - `internal/server/desktop_remote_security.go`
  - `internal/inventory/inventory.go`
  - `internal/smbclient/*`

  Calling existing exported functions such as `inventory.CreateDevice`, `GetDeviceByID` or `ListAllDevices` is fine. Do not call helpers from `desktop_remote_security.go`; reimplement the small pieces you need.
- **GitNexus.** The worktree cannot be indexed, so run impact from the **main checkout root** (`C:\Users\Andi\Documents\repo\AuraGo`). Before Task 3, run:
  - `node ../../../.gitnexus/maintenance/AuraGo/graph/run.cjs impact "codeStudioHandlers" --direction upstream --repo .`
  - the same for `execGit`, `readCodeStudioFileBytes`, `gitPathsRequest`, `runGitSteps`, `handleLSP` and `handleTerminal`.

  Report the risk. Expected: callers only inside `internal/server/code_studio_*`, risk LOW. A text search confirmed this; treat UNKNOWN as unresolved. Before every commit, run `detect-changes --scope all` the same way.
- **Test command (Windows):** `go test ./internal/server/ -run '(?i)(CodeStudio)' -count=1`, plus `go vet ./internal/server/ ./internal/desktop/`.
  - If the build breaks on **untracked files of the other session**, write an overlay JSON in the scratchpad that maps each broken untracked file to `""` (for example `{"Replace":{"C:\\...\\internal\\server\\wip_x.go":""}}`) and add `-overlay=<that file>`.
  - Use the usual `GOCACHE`/`GOTMPDIR` setup on D:.
- **Commit after every task** (user preference). Conventional commits, scope `code-studio`.
- **Lean testing.** Write only the tests listed in the tasks: no stress, property or mutation tests, and no re-review rounds.

## File map

**New files**

| File | Responsibility |
|---|---|
| `internal/server/code_studio_backend.go` | Interface `codeStudioBackend`, `codeStudioProcess`, `codeStudioWorkspaceInfo`, `codeStudioCapabilities`, error types and writers, workspace-id parsing, `backendFor`, the `remoteBackend` stub (replaced in Task 14), change hook, `codeStudioShellJoin`, `codeStudioExecArgv`, `codeStudioTerminalSize` |
| `internal/server/code_studio_backend_local.go` | `codeStudioLocalBackend` |
| `internal/server/code_studio_workspace_store.go` | `codeStudioWorkspaceStore` (SQLite table, UPSERT, list, get, touch, delete), `h.workspaceStore` |
| `internal/server/code_studio_ssh_pool.go` | `codeStudioSSHPool`, `codeStudioSSHConn` (run, terminal, process), `codeStudioRemote`, `h.remoteState`, default dialer, dial-error classification, caller gate |
| `internal/server/code_studio_ssh_backend.go` | `codeStudioSSHBackend`, path mapping, SFTP helpers, grep, capabilities script and parser, terminal and process types |
| `internal/server/code_studio_workspace_handlers.go` | Workspaces, remote devices, browse and trust endpoints; `registerCodeStudioWorkspaceRoutes`; audit helper |
| `internal/desktop/ssh_workspace_dial.go` | `SSHAccess`, `SSHHostKeyError`, `SSHDialOptions`, `ResolveSSHDevice`, `DescribeSSHDevice`, `DialSSH`, `ErrSSHDeviceNotFound` |
| `internal/server/code_studio_backend_test.go` | ID parsing, exec-argv parity, error JSON shapes, hook, backend selection, path mapping, capabilities, grep remap |
| `internal/server/code_studio_workspace_store_test.go` | Store |
| `internal/server/code_studio_ssh_fixture_test.go` | In-process SSH + SFTP server (`sftp.InMemHandler`, canned exec, echo PTY), helpers |
| `internal/server/code_studio_ssh_backend_test.go` | SSH backend files, exec, git, search, terminal, process; `backendFor` over SSH; endpoints including the 409 contract, devices and browse |

**Modified files**

| File | Change |
|---|---|
| `internal/server/code_studio_handlers.go` | Struct field `remote`; route registration; handlers moved onto the backend; `readCodeStudioFileBytes(..., max int64)` |
| `internal/server/code_studio_git_workbench.go` | `gitPathsRequest` and `runGitSteps` take the backend |
| `internal/server/code_studio_lsp.go` | `handleLSP` resolves the backend; non-local answers 501 |

`code_studio_files.go`, `code_studio_paths.go`, `code_studio_terminal.go`, `code_studio_exec_stdin.go` and `code_studio_docker_adapter.go` stay **unchanged**.

---

## Group 0: Baseline

### Task 0: Green baseline (2 min)

- Run `go test ./internal/server/ -run '(?i)(CodeStudio)' -count=1` and `go vet ./internal/server/ ./internal/desktop/`. Expected: `ok`, and vet prints nothing. Use the overlay if untracked files of the other session break the build.
- Run the GitNexus impact commands from the Hard constraints and write down the risk.
- No commit.

---

## Group 1: Backend interface and local refactor (stays green after every task)

### Task 1: Backend types, errors, hook and shell helpers (10 min)

**File:** `internal/server/code_studio_backend.go`. New file. `encoding/json` is needed for the remote error writer.

```go
package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	codeStudioLocalWorkspaceID   = "local"
	codeStudioWorkspaceHeader    = "X-Code-Studio-Workspace"
	codeStudioWorkspaceQuery     = "ws"
	codeStudioWorkspaceKindLocal = "local"
	codeStudioWorkspaceKindSSH   = "ssh"
)

var codeStudioWorkspaceIDPattern = regexp.MustCompile(`^[A-Za-z0-9_-]{1,64}$`)

// codeStudioBackend is the workspace a Code Studio request works on. Every path
// argument and result is a sanitised virtual path below /workspace; only the
// backend maps it to its storage.
type codeStudioBackend interface {
	Info() codeStudioWorkspaceInfo
	List(ctx context.Context, vdir string) ([]codeStudioFileEntry, error)
	// Probe resolves symlinks; Path of the result is the physical virtual path.
	Probe(ctx context.Context, vpath string) (codeStudioFileProbe, error)
	Read(ctx context.Context, vpath string, max int64) ([]byte, error)
	// Write creates parent directories; an existing file with createOnly returns an error matching errCodeStudioExists.
	Write(ctx context.Context, vpath string, content []byte, createOnly bool) error
	Move(ctx context.Context, from, to string) error
	Delete(ctx context.Context, vpath string) error
	Mkdir(ctx context.Context, vpath string) error
	Search(ctx context.Context, opts codeStudioSearchOptions) ([]codeStudioSearchResult, bool, error)
	// Exec runs argv. vcwd "" = backend default directory. stdin == nil = no stdin attached.
	Exec(ctx context.Context, argv []string, vcwd string, stdin []byte, timeout time.Duration) (codeStudioExecResult, error)
	// Git runs git -C <root> --literal-pathspecs args...
	Git(ctx context.Context, timeout time.Duration, args ...string) (codeStudioExecResult, error)
	OpenTerminal(ctx context.Context, cols, rows int) (containerTerminalSession, error)
	// StartProcess starts a non-TTY stdio process (language servers, Plan D).
	StartProcess(ctx context.Context, argv []string, vcwd string) (codeStudioProcess, error)
}

// codeStudioProcess is a running stdio process. Callers must drain Stderr.
type codeStudioProcess interface {
	io.Reader // stdout
	io.Writer // stdin
	CloseWrite() error
	Close() error // kills the process and frees its session
	Stderr() io.Reader
	Wait() error
}

type codeStudioLSPCapabilities struct {
	Go         bool `json:"go"`
	Python     bool `json:"python"`
	TypeScript bool `json:"typescript"`
	CPP        bool `json:"cpp"`
}

type codeStudioCapabilities struct {
	OS         string                    `json:"os"`
	Arch       string                    `json:"arch"`
	Shell      string                    `json:"shell"`
	Git        bool                      `json:"git"`
	Grep       bool                      `json:"grep"`
	Python3    bool                      `json:"python3"`
	Formatters map[string]bool           `json:"formatters"`
	LSP        codeStudioLSPCapabilities `json:"lsp"`
	ProbedAt   time.Time                 `json:"probed_at"`
}

type codeStudioWorkspaceInfo struct {
	ID           string                  `json:"id"`
	Kind         string                  `json:"kind"`
	Label        string                  `json:"label"`
	Root         string                  `json:"root"`
	DeviceID     string                  `json:"device_id,omitempty"`
	DeviceName   string                  `json:"device_name,omitempty"`
	Host         string                  `json:"host,omitempty"`
	Port         int                     `json:"port,omitempty"`
	User         string                  `json:"user,omitempty"`
	Connected    bool                    `json:"connected"`
	CreatedAt    *time.Time              `json:"created_at,omitempty"`
	LastUsedAt   *time.Time              `json:"last_used_at,omitempty"`
	Capabilities *codeStudioCapabilities `json:"capabilities,omitempty"`
}

func codeStudioLocalWorkspaceInfo() codeStudioWorkspaceInfo {
	return codeStudioWorkspaceInfo{ID: codeStudioLocalWorkspaceID, Kind: codeStudioWorkspaceKindLocal,
		Label: "Local workspace", Root: codeStudioWorkspaceRoot, Connected: true}
}

var (
	errCodeStudioExists      = errors.New("file already exists")
	errCodeStudioUnsupported = errors.New("operation is not supported by this workspace")
)

// codeStudioStatusError = "the operation ran and failed"; written as {"error": Message}.
type codeStudioStatusError struct {
	Status  int
	Message string
	Err     error
}

func (e *codeStudioStatusError) Error() string { return e.Message }
func (e *codeStudioStatusError) Unwrap() error { return e.Err }

func codeStudioBadRequest(msg string) error {
	return &codeStudioStatusError{Status: http.StatusBadRequest, Message: msg}
}

func codeStudioExistsError(msg string) error {
	return &codeStudioStatusError{Status: http.StatusConflict, Message: msg, Err: errCodeStudioExists}
}

// codeStudioRemoteError is a workspace/SSH contract error with a machine code.
type codeStudioRemoteError struct {
	Status                     int
	Code, Message              string
	Host, KeyType, Fingerprint string
	Err                        error
}

func (e *codeStudioRemoteError) Error() string { return e.Message }
func (e *codeStudioRemoteError) Unwrap() error { return e.Err }

// codeStudioUnavailableError: the local container or the workspace store could not be
// resolved -> 503 err.Error(), exactly as codeContainer errors are answered today.
type codeStudioUnavailableError struct{ err error }

func (e codeStudioUnavailableError) Error() string { return e.err.Error() }
func (e codeStudioUnavailableError) Unwrap() error { return e.err }

func codeStudioWorkspaceNotFound() error {
	return &codeStudioRemoteError{Status: http.StatusNotFound, Code: "workspace_not_found", Message: "workspace not found"}
}

func codeStudioWorkspaceID(r *http.Request) (string, error) {
	raw := strings.TrimSpace(r.Header.Get(codeStudioWorkspaceHeader))
	if raw == "" {
		raw = strings.TrimSpace(r.URL.Query().Get(codeStudioWorkspaceQuery))
	}
	if raw == "" || raw == codeStudioLocalWorkspaceID {
		return codeStudioLocalWorkspaceID, nil
	}
	if !codeStudioWorkspaceIDPattern.MatchString(raw) {
		return "", &codeStudioRemoteError{Status: http.StatusBadRequest, Code: "invalid_workspace", Message: "invalid workspace id"}
	}
	return raw, nil
}

func writeCodeStudioRemoteError(w http.ResponseWriter, e *codeStudioRemoteError) {
	payload := map[string]string{"error": e.Message, "code": e.Code}
	if e.Host != "" {
		payload["host"] = e.Host
	}
	if e.KeyType != "" {
		payload["key_type"] = e.KeyType
	}
	if e.Fingerprint != "" {
		payload["fingerprint"] = e.Fingerprint
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(e.Status)
	_ = json.NewEncoder(w).Encode(payload)
}

func writeCodeStudioOpError(w http.ResponseWriter, err error) {
	var status *codeStudioStatusError
	if errors.As(err, &status) {
		jsonError(w, status.Message, status.Status)
		return
	}
	var remote *codeStudioRemoteError
	if errors.As(err, &remote) {
		writeCodeStudioRemoteError(w, remote)
		return
	}
	var rejected codeStudioProbeRejection
	if errors.As(err, &rejected) {
		jsonError(w, rejected.message, http.StatusBadRequest)
		return
	}
	jsonError(w, err.Error(), http.StatusBadGateway)
}

func writeCodeStudioBackendError(w http.ResponseWriter, err error) {
	var unavailable codeStudioUnavailableError
	if errors.As(err, &unavailable) {
		jsonError(w, unavailable.Error(), http.StatusServiceUnavailable)
		return
	}
	writeCodeStudioOpError(w, err)
}

// writeCodeStudioProbeError keeps codeStudioProbeHTTPError for local probe errors.
func writeCodeStudioProbeError(w http.ResponseWriter, err error) {
	var status *codeStudioStatusError
	var remote *codeStudioRemoteError
	if errors.As(err, &status) || errors.As(err, &remote) {
		writeCodeStudioOpError(w, err)
		return
	}
	code, msg, _ := codeStudioProbeHTTPError(err)
	jsonError(w, msg, code)
}

// remoteBackend resolves a saved SSH workspace. Stub until Task 14.
func (h *codeStudioHandlers) remoteBackend(r *http.Request, id string) (codeStudioBackend, error) {
	return nil, codeStudioWorkspaceNotFound()
}

// ---- workspace change hook (Plan B registers the preview version bump) ----

type codeStudioWorkspaceChangeHook func(wsID string, paths []string)

var codeStudioWorkspaceChange struct {
	mu    sync.RWMutex
	next  int
	hooks map[int]codeStudioWorkspaceChangeHook
}

// registerCodeStudioWorkspaceChangeHook adds fn; hooks run synchronously and must not block.
func registerCodeStudioWorkspaceChangeHook(fn codeStudioWorkspaceChangeHook) (unregister func()) {
	codeStudioWorkspaceChange.mu.Lock()
	defer codeStudioWorkspaceChange.mu.Unlock()
	if codeStudioWorkspaceChange.hooks == nil {
		codeStudioWorkspaceChange.hooks = map[int]codeStudioWorkspaceChangeHook{}
	}
	id := codeStudioWorkspaceChange.next
	codeStudioWorkspaceChange.next++
	codeStudioWorkspaceChange.hooks[id] = fn
	return func() {
		codeStudioWorkspaceChange.mu.Lock()
		delete(codeStudioWorkspaceChange.hooks, id)
		codeStudioWorkspaceChange.mu.Unlock()
	}
}

// notifyWorkspaceChange reports changed virtual paths of a workspace. No paths = the
// changed set is unknown (git switch, discard of many files): treat as "everything".
func (h codeStudioHandlers) notifyWorkspaceChange(wsID string, paths ...string) {
	codeStudioWorkspaceChange.mu.RLock()
	hooks := make([]codeStudioWorkspaceChangeHook, 0, len(codeStudioWorkspaceChange.hooks))
	for _, fn := range codeStudioWorkspaceChange.hooks {
		hooks = append(hooks, fn)
	}
	codeStudioWorkspaceChange.mu.RUnlock()
	for _, fn := range hooks {
		fn(wsID, append([]string(nil), paths...))
	}
}

// ---- shell helpers ----

func codeStudioShellJoin(argv []string) string {
	quoted := make([]string, len(argv))
	for i, a := range argv {
		quoted[i] = shellQuote(a)
	}
	return strings.Join(quoted, " ")
}

// codeStudioExecArgv splits an exec request into argv + virtual cwd. For the local
// backend the result reproduces normalizeCodeStudioExecCommand byte for byte (args
// ignore cwd, as today).
func codeStudioExecArgv(command string, args []string, cwd string) ([]string, string, error) {
	if len(args) > 0 {
		if strings.TrimSpace(args[0]) == "" {
			return nil, "", fmt.Errorf("command is required")
		}
		return args, "", nil
	}
	command = strings.TrimSpace(command)
	if command == "" {
		return nil, "", fmt.Errorf("command is required")
	}
	if strings.TrimSpace(cwd) == "" {
		return []string{"sh", "-c", command}, "", nil
	}
	path, err := sanitizeCodeStudioPath(cwd)
	if err != nil {
		return nil, "", err
	}
	return []string{"sh", "-c", command}, path, nil
}

// codeStudioTerminalSize reads cols/rows (defaults 120x30, clamped).
func codeStudioTerminalSize(r *http.Request) (int, int) {
	clamp := func(raw string, def, lo, hi int) int {
		v, err := strconv.Atoi(strings.TrimSpace(raw))
		if err != nil || v <= 0 {
			return def
		}
		return min(max(v, lo), hi)
	}
	return clamp(r.URL.Query().Get("cols"), 120, 20, 500), clamp(r.URL.Query().Get("rows"), 30, 5, 200)
}
```

**Test first:** `internal/server/code_studio_backend_test.go`.

```go
func TestCodeStudioWorkspaceIDFromRequest(t *testing.T) {
	cases := []struct{ header, query, want string; wantErr bool }{
		{"", "", "local", false}, {"local", "", "local", false}, {"", "ssh_ab12", "ssh_ab12", false},
		{"ssh_h", "ssh_q", "ssh_h", false}, {"../x", "", "", true}, {"", strings.Repeat("a", 65), "", true},
	}
	for _, c := range cases {
		r := httptest.NewRequest(http.MethodGet, "/api/code-studio/files?ws="+url.QueryEscape(c.query), nil)
		if c.header != "" { r.Header.Set(codeStudioWorkspaceHeader, c.header) }
		got, err := codeStudioWorkspaceID(r)
		if (err != nil) != c.wantErr || got != c.want { t.Fatalf("%+v: got %q err %v", c, got, err) }
	}
}

func TestCodeStudioExecArgvMatchesLegacyNormalize(t *testing.T) {
	for _, c := range []struct{ cmd string; args []string; cwd string }{
		{"go test", nil, ""}, {"go test", nil, "/workspace/pkg"}, {"", []string{"gofmt"}, "/workspace/x"},
	} {
		legacy, _ := normalizeCodeStudioExecCommand(c.cmd, c.args, c.cwd)
		argv, vcwd, err := codeStudioExecArgv(c.cmd, c.args, c.cwd)
		if err != nil { t.Fatal(err) }
		got := codeStudioLocalExecCommand(argv, vcwd)
		if !reflect.DeepEqual(got, legacy) { t.Fatalf("got %q want %q", got, legacy) }
	}
}

func TestCodeStudioErrorWritersShapes(t *testing.T) {
	rec := httptest.NewRecorder()
	writeCodeStudioOpError(rec, &codeStudioRemoteError{Status: 409, Code: "unknown_host_key", Message: "Unknown SSH host key for h", Host: "h", KeyType: "ssh-ed25519", Fingerprint: "SHA256:x"})
	var body map[string]string
	_ = json.Unmarshal(rec.Body.Bytes(), &body)
	if rec.Code != 409 || body["code"] != "unknown_host_key" || body["fingerprint"] != "SHA256:x" || body["key_type"] != "ssh-ed25519" || body["host"] != "h" { t.Fatalf("%d %v", rec.Code, body) }
	rec = httptest.NewRecorder()
	writeCodeStudioOpError(rec, codeStudioExistsError("file already exists"))
	if rec.Code != 409 || strings.TrimSpace(rec.Body.String()) != `{"error":"file already exists"}` { t.Fatal(rec.Body.String()) }
	if !errors.Is(codeStudioExistsError("x"), errCodeStudioExists) { t.Fatal("exists sentinel") }
	rec = httptest.NewRecorder()
	writeCodeStudioBackendError(rec, codeStudioUnavailableError{errors.New("docker down")})
	if rec.Code != 503 { t.Fatal(rec.Code) }
}

func TestCodeStudioWorkspaceChangeHook(t *testing.T) {
	var got []string
	unregister := registerCodeStudioWorkspaceChangeHook(func(ws string, p []string) { got = append(got, ws+":"+strings.Join(p, ",")) })
	codeStudioHandlers{}.notifyWorkspaceChange("local", "/workspace/a")
	unregister()
	codeStudioHandlers{}.notifyWorkspaceChange("local", "/workspace/b")
	if len(got) != 1 || got[0] != "local:/workspace/a" { t.Fatal(got) }
}
```

`codeStudioLocalExecCommand` is added in Task 2. Until then the parity test does not compile, so write it in this task and run it in Task 2. Run only the other three tests now.

**Run:** `go test ./internal/server/ -run '(?i)CodeStudio(WorkspaceIDFrom|ErrorWriters|WorkspaceChangeHook)' -count=1`. Comment out the parity test until Task 2. Expected: PASS.

**Commit:** `feat(code-studio): workspace backend interface, error contract and change hook`

### Task 2: Local backend and `backendFor` (10 min)

**File:** `internal/server/code_studio_backend_local.go`. New file.

```go
package server

import (
	"context"
	"strings"
	"time"

	"aurago/internal/desktop"
)

// codeStudioLocalBackend is today's container workspace. h carries the Docker adapter
// codeContainer installed, so every command vector equals the pre-backend code.
type codeStudioLocalBackend struct {
	h           codeStudioHandlers
	container   *desktop.CodeContainerService
	containerID string
}

var _ codeStudioBackend = (*codeStudioLocalBackend)(nil)

func (h *codeStudioHandlers) backendFor(r *http.Request) (codeStudioBackend, error) {
	id, err := codeStudioWorkspaceID(r)
	if err != nil {
		return nil, err
	}
	if id == codeStudioLocalWorkspaceID {
		return h.localBackend(r.Context())
	}
	return h.remoteBackend(r, id)
}

func (h *codeStudioHandlers) localBackend(ctx context.Context) (codeStudioBackend, error) {
	container, containerID, err := h.codeContainer(ctx, true)
	if err != nil {
		return nil, codeStudioUnavailableError{err}
	}
	return &codeStudioLocalBackend{h: *h, container: container, containerID: containerID}, nil
}

func (b *codeStudioLocalBackend) Info() codeStudioWorkspaceInfo { return codeStudioLocalWorkspaceInfo() }

func (b *codeStudioLocalBackend) pathOp(ctx context.Context, cmd []string) error {
	result, err := b.h.docker.Exec(ctx, b.containerID, cmd, 30*time.Second)
	if err != nil {
		return err
	}
	if result.ExitCode != 0 {
		return codeStudioBadRequest(strings.TrimSpace(result.Output))
	}
	return nil
}

func (b *codeStudioLocalBackend) List(ctx context.Context, vdir string) ([]codeStudioFileEntry, error) {
	result, err := b.h.docker.Exec(ctx, b.containerID, codeStudioPathCommand("list", vdir), 30*time.Second)
	if err != nil {
		return nil, err
	}
	if result.ExitCode != 0 {
		return nil, codeStudioBadRequest(strings.TrimSpace(result.Output))
	}
	return parseCodeStudioFindOutput(result.Output)
}

func (b *codeStudioLocalBackend) Probe(ctx context.Context, vpath string) (codeStudioFileProbe, error) {
	return b.h.probeCodeStudioFile(ctx, b.containerID, vpath)
}

func (b *codeStudioLocalBackend) Read(ctx context.Context, vpath string, max int64) ([]byte, error) {
	return b.h.readCodeStudioFileBytes(ctx, b.containerID, vpath, max)
}

func (b *codeStudioLocalBackend) Write(ctx context.Context, vpath string, content []byte, createOnly bool) error {
	result, err := b.h.writeContainerFile(ctx, b.containerID, vpath, content, createOnly)
	if err != nil {
		return err
	}
	switch {
	case result.ExitCode == codeStudioFileExists:
		return codeStudioExistsError(strings.TrimSpace(result.Output)) // no default text: byte-compatible
	case result.ExitCode != 0:
		return codeStudioBadRequest(strings.TrimSpace(result.Output))
	}
	return nil
}

func (b *codeStudioLocalBackend) Move(ctx context.Context, from, to string) error {
	return b.pathOp(ctx, codeStudioPathCommand("move", from, to))
}

func (b *codeStudioLocalBackend) Delete(ctx context.Context, vpath string) error {
	return b.pathOp(ctx, codeStudioPathCommand("delete", vpath))
}

func (b *codeStudioLocalBackend) Mkdir(ctx context.Context, vpath string) error {
	return b.pathOp(ctx, codeStudioPathCommand("mkdir", vpath))
}

func (b *codeStudioLocalBackend) Search(ctx context.Context, opts codeStudioSearchOptions) ([]codeStudioSearchResult, bool, error) {
	cmd, err := buildCodeStudioSearchCommand(opts)
	if err != nil {
		return nil, false, codeStudioBadRequest(err.Error())
	}
	result, err := b.h.docker.Exec(ctx, b.containerID, cmd, 60*time.Second)
	if err != nil {
		return nil, false, err
	}
	if result.ExitCode > 1 {
		return nil, false, codeStudioBadRequest(strings.TrimSpace(result.Output))
	}
	return parseCodeStudioGrepOutput(result.Output)
}

// codeStudioLocalExecCommand rebuilds the legacy vector: sh -c "cd <cwd> && <command>".
func codeStudioLocalExecCommand(argv []string, vcwd string) []string {
	if vcwd == "" {
		return argv
	}
	line := codeStudioShellJoin(argv)
	if len(argv) == 3 && argv[0] == "sh" && argv[1] == "-c" {
		line = argv[2]
	}
	return []string{"sh", "-c", "cd " + shellQuote(vcwd) + " && " + line}
}

func (b *codeStudioLocalBackend) Exec(ctx context.Context, argv []string, vcwd string, stdin []byte, timeout time.Duration) (codeStudioExecResult, error) {
	cmd := codeStudioLocalExecCommand(argv, vcwd)
	if stdin != nil {
		execer, ok := b.h.docker.(codeStudioStdinExecer)
		if !ok {
			return codeStudioExecResult{}, &codeStudioStatusError{Status: http.StatusNotImplemented,
				Message: "stdin is not supported by this docker client", Err: errCodeStudioUnsupported}
		}
		return execer.ExecWithStdin(ctx, b.containerID, cmd, stdin, timeout)
	}
	return b.h.docker.Exec(ctx, b.containerID, cmd, timeout)
}

func (b *codeStudioLocalBackend) Git(ctx context.Context, timeout time.Duration, args ...string) (codeStudioExecResult, error) {
	return b.h.execGit(ctx, b.containerID, timeout, args...)
}

func (b *codeStudioLocalBackend) OpenTerminal(ctx context.Context, cols, rows int) (containerTerminalSession, error) {
	adapter, ok := b.h.docker.(codeStudioDockerAdapter)
	if !ok {
		return nil, errCodeStudioUnsupported
	}
	session, err := openCodeStudioTerminalSession(ctx, adapter.cfg, b.containerID)
	if err != nil {
		return nil, err
	}
	_ = session.Resize(ctx, cols, rows)
	return session, nil
}

func (b *codeStudioLocalBackend) StartProcess(context.Context, []string, string) (codeStudioProcess, error) {
	return nil, &codeStudioStatusError{Status: http.StatusNotImplemented,
		Message: "the local workspace runs language servers through the container bridge", Err: errCodeStudioUnsupported}
}
```

Add `"net/http"` to the imports.

**Edit `code_studio_handlers.go`:** change the signature of `readCodeStudioFileBytes` to `(ctx context.Context, containerID, physical string, max int64)` and use `strconv.FormatInt(max, 10)` instead of `h.maxFileSizeBytes()`. Its only callers are the two handlers, which pass `h.maxFileSizeBytes()` from Task 3 on. For now, update both call sites to pass `h.maxFileSizeBytes()`.

**Test (append to `code_studio_backend_test.go`):**

```go
func TestCodeStudioBackendForSelectsLocalOrRejects(t *testing.T) {
	s := testCodeStudioServerWithFakeCodeContainer(t, 1)
	h := codeStudioHandlers{server: s, docker: &recordingCodeStudioDockerAPI{}}
	for _, ws := range []string{"", "local"} {
		r := httptest.NewRequest(http.MethodGet, "/api/code-studio/files", nil)
		if ws != "" { r.Header.Set(codeStudioWorkspaceHeader, ws) }
		b, err := h.backendFor(r)
		if err != nil { t.Fatal(err) }
		if _, ok := b.(*codeStudioLocalBackend); !ok || b.Info().ID != "local" { t.Fatalf("%T", b) }
	}
	rec := httptest.NewRecorder()
	h.handleFiles(rec, httptest.NewRequest(http.MethodGet, "/api/code-studio/files?ws=ssh_missing", nil))
	if rec.Code != http.StatusNotFound || !strings.Contains(rec.Body.String(), `"workspace_not_found"`) { t.Fatal(rec.Code, rec.Body.String()) }
	rec = httptest.NewRecorder()
	h.handleFiles(rec, httptest.NewRequest(http.MethodGet, "/api/code-studio/files?ws=..%2Fx", nil))
	if rec.Code != http.StatusBadRequest { t.Fatal(rec.Code) }
}
```

This test only passes once `handleFiles` uses `backendFor` (Task 3). In this task, run only the parity test.

**Run:** `go test ./internal/server/ -run '(?i)CodeStudio' -count=1`, with `TestCodeStudioBackendForSelectsLocalOrRejects` temporarily marked `t.Skip("Task 3")`. Expected: all existing tests PASS, and the parity test PASSES.

**Commit:** `feat(code-studio): local container backend behind codeStudioBackend`

### Task 3: File handlers onto the backend (10 min)

**File:** `internal/server/code_studio_handlers.go`. In each handler below, replace the block `_, containerID, err := h.codeContainer(r.Context(), true) ... 503` with:

```go
	b, err := h.backendFor(r)
	if err != nil {
		writeCodeStudioBackendError(w, err)
		return
	}
```

Then make these replacements:

- `handleFiles`: `entries, err := b.List(r.Context(), path); if err != nil { writeCodeStudioOpError(w, err); return }`, followed by the same `writeJSON`.
- `handleReadFile` and `handleDownload`:
  ```go
  probed, probeErr := b.Probe(r.Context(), path)
  if probeErr != nil { writeCodeStudioProbeError(w, probeErr); return }
  // size check unchanged
  content, err := b.Read(r.Context(), probed.Path, h.maxFileSizeBytes())
  if err != nil { writeCodeStudioOpError(w, err); return }
  ```
- `handleWriteFile`: `if err := b.Write(r.Context(), path, []byte(body.Content), body.CreateOnly); err != nil { writeCodeStudioOpError(w, err); return }`, then `h.notifyWorkspaceChange(b.Info().ID, path)`, then the same `writeJSON`.
- `handleMoveFile`: `b.Move(ctx, oldPath, newPath)`, then notify `(oldPath, newPath)`.
- `handleDeleteFile`: `b.Delete(ctx, path)`, then notify `(path)`. The root check stays before `backendFor`.
- `handleDirectory`: `b.Mkdir(ctx, path)`, then notify `(path)`.
- `writeUploadedFile`: resolve `b` where `codeContainer` was, then `b.Write(ctx, path, buf.Bytes(), false)`, then notify `(path)`. Upload never passes createOnly, so the old exit 73 branch cannot happen and the status stays 400 for other failures.

Remove the now-unused `h.docker.Exec` calls in those handlers. Remove the `t.Skip` from Task 2.

**Run:** `go test ./internal/server/ -run '(?i)CodeStudio' -count=1`. Expected: PASS, including the files, security, download, symlink and selection tests, unchanged.

**Commit:** `refactor(code-studio): file handlers run through the workspace backend`

### Task 4: Exec and search onto the backend (5 min)

**`handleExec`** after the JSON decode and the stdin size check:

```go
	argv, vcwd, err := codeStudioExecArgv(body.Command, body.Args, body.CWD)
	if err != nil {
		jsonError(w, err.Error(), http.StatusBadRequest)
		return
	}
	timeout := normalizeCodeStudioExecTimeout(body.TimeoutSec)
	b, err := h.backendFor(r)
	if err != nil {
		writeCodeStudioBackendError(w, err)
		return
	}
	var stdin []byte
	if body.Stdin != nil {
		stdin = append(make([]byte, 0, len(*body.Stdin)), *body.Stdin...) // non-nil even when empty
	}
	result, err := b.Exec(r.Context(), argv, vcwd, stdin, timeout)
	if err != nil {
		writeCodeStudioOpError(w, err)
		return
	}
```

The output cap and the response stay as they are. Keep `normalizeCodeStudioExecCommand`; the parity test uses it.

**`handleSearch`:** keep `codeStudioSearchOptionsFromRequest`, then

```go
if _, err := buildCodeStudioSearchCommand(options); err != nil {
	jsonError(w, err.Error(), http.StatusBadRequest)
	return
}
```

then `b, err := h.backendFor(r)` (error to `writeCodeStudioBackendError`), then `results, truncated, err := b.Search(r.Context(), options)` (error to `writeCodeStudioOpError`). The payload is the same.

**Run:** the full CodeStudio suite. Expected: PASS. `TestCodeStudioExecPipesStdin...` still records `[["gofmt"]]`.

**Commit:** `refactor(code-studio): exec and search through the workspace backend`

### Task 5: Git handlers onto the backend (10 min)

In `code_studio_handlers.go` (`handleGitStatus`, `handleGitDiff`, `handleGitCommit`, `listGitBranches`, `mutateGitBranch`, `handleGitIdentity`, `handleGitLog`) and in `code_studio_git_workbench.go`:

- Replace each `codeContainer` block with the `backendFor` block from Task 3.
- Replace `h.execGit(r.Context(), containerID, T, args...)` with `b.Git(r.Context(), T, args...)`.
- Replace error branches `jsonError(w, err.Error(), http.StatusBadGateway)` that follow a git call with `writeCodeStudioOpError(w, err)`. For plain errors the result is identical (502, `err.Error()`).
- `listGitBranches(w, r)` becomes `listGitBranches(w http.ResponseWriter, r *http.Request, b codeStudioBackend)`.
  - `handleGitBranch` GET resolves `b` first, then calls `h.listGitBranches(w, r, b)`.
  - `mutateGitBranch` resolves `b` once and ends with `h.notifyWorkspaceChange(b.Info().ID)` followed by `h.listGitBranches(w, r, b)` for the switch and create actions.
- `gitPathsRequest` returns `([]string, codeStudioBackend, bool)`. Keep the pointer receiver and the comment, updated to say "backendFor installs the adapter on h".
- `runGitSteps(w, r, b codeStudioBackend, steps, fallback)`.
- `handleGitDiscard`: after a successful discard, call `h.notifyWorkspaceChange(b.Info().ID, virtual...)` with `virtual[i] = codeStudioWorkspaceRoot + "/" + rels[i]`. To know whether it succeeded, make `runGitSteps` return a `bool` and notify when it is true.
- `handleGitInit`: notify with no paths after success.
- Keep the `execGit` method; `TestCodeStudio...` in `git_search_test` calls it directly.

**Run:** the full suite. Expected: PASS for the git_commit, git_workbench and git_search tests, unchanged.

**Commit:** `refactor(code-studio): git endpoints through the workspace backend`

### Task 6: Terminal and LSP handler branches (5 min)

**`handleTerminal`** after the method check:

```go
	b, err := h.backendFor(r)
	if err != nil {
		writeCodeStudioBackendError(w, err)
		return
	}
	if local, ok := b.(*codeStudioLocalBackend); ok {
		release, err := local.container.HoldTerminal(local.containerID)
		if err != nil {
			jsonError(w, err.Error(), http.StatusServiceUnavailable)
			return
		}
		defer release()
		conn, err := codeStudioWSUpgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()
		h.serveCodeStudioTerminal(r.Context(), conn, local.containerID)
		return
	}
	cols, rows := codeStudioTerminalSize(r)
	session, err := b.OpenTerminal(r.Context(), cols, rows)
	if err != nil {
		writeCodeStudioOpError(w, err)
		return
	}
	conn, err := codeStudioWSUpgrader.Upgrade(w, r, nil)
	if err != nil {
		_ = session.Close()
		return
	}
	serveContainerTerminalSession(r.Context(), conn, session)
```

**`handleLSP`** (`code_studio_lsp.go`): replace `container, containerID, err := h.codeContainer(...)` with:

```go
	b, err := h.backendFor(r)
	if err != nil {
		writeCodeStudioBackendError(w, err)
		return
	}
	local, ok := b.(*codeStudioLocalBackend)
	if !ok { // Plan D: language servers over SSH
		writeCodeStudioRemoteError(w, &codeStudioRemoteError{Status: http.StatusNotImplemented,
			Code: "lsp_unavailable", Message: "language servers are not available in this workspace yet"})
		return
	}
```

Then use `local.container.HoldTerminal(local.containerID)` and `h.serveCodeStudioLSP(r.Context(), conn, local.containerID, lsp)`.

**Run:** the full suite plus `go vet ./internal/server/`. Expected: PASS, including `TestCodeStudioTerminalRejects*` and the LSP tests.

**Commit:** `refactor(code-studio): terminal and LSP resolve the workspace backend`

---

## Group 2: Workspace store

### Task 7: `codeStudioWorkspaceStore` (10 min)

**File:** `internal/server/code_studio_workspace_store.go`.

```go
package server

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"
)

var errCodeStudioWorkspaceRecordNotFound = errors.New("workspace not found")

type codeStudioWorkspaceRecord struct {
	ID, DeviceID, Root, Label string
	Capabilities              *codeStudioCapabilities
	CreatedAt, LastUsedAt     time.Time
}

type codeStudioWorkspaceStore struct {
	db  *sql.DB
	now func() time.Time
}

var codeStudioWorkspaceSchemas sync.Map // *sql.DB -> struct{}

const codeStudioWorkspaceSchema = `CREATE TABLE IF NOT EXISTS code_studio_workspaces (
	id TEXT PRIMARY KEY,
	device_id TEXT NOT NULL,
	root TEXT NOT NULL,
	label TEXT NOT NULL DEFAULT '',
	capabilities TEXT NOT NULL DEFAULT '',
	created_at TEXT NOT NULL,
	last_used_at TEXT NOT NULL,
	UNIQUE(device_id, root))`

func openCodeStudioWorkspaceStore(ctx context.Context, db *sql.DB) (*codeStudioWorkspaceStore, error) {
	if db == nil {
		return nil, errors.New("desktop database is not available")
	}
	if _, done := codeStudioWorkspaceSchemas.Load(db); !done {
		if _, err := db.ExecContext(ctx, codeStudioWorkspaceSchema); err != nil {
			return nil, fmt.Errorf("create code studio workspace table: %w", err)
		}
		codeStudioWorkspaceSchemas.Store(db, struct{}{})
	}
	return &codeStudioWorkspaceStore{db: db, now: func() time.Time { return time.Now().UTC() }}, nil
}

func (h codeStudioHandlers) workspaceStore(ctx context.Context) (*codeStudioWorkspaceStore, error) {
	if h.server == nil {
		return nil, errors.New("code studio server is not available")
	}
	svc, _, err := h.server.getDesktopService(ctx)
	if err != nil {
		return nil, err
	}
	return openCodeStudioWorkspaceStore(ctx, svc.DB())
}

func newCodeStudioWorkspaceID() string { return "ssh_" + strings.ToLower(rand.Text()) }

const codeStudioWorkspaceColumns = `id, device_id, root, label, capabilities, created_at, last_used_at`

func scanCodeStudioWorkspace(row interface{ Scan(...any) error }) (codeStudioWorkspaceRecord, error) {
	var rec codeStudioWorkspaceRecord
	var caps, created, used string
	if err := row.Scan(&rec.ID, &rec.DeviceID, &rec.Root, &rec.Label, &caps, &created, &used); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return rec, errCodeStudioWorkspaceRecordNotFound
		}
		return rec, err
	}
	rec.CreatedAt, _ = time.Parse(time.RFC3339Nano, created)
	rec.LastUsedAt, _ = time.Parse(time.RFC3339Nano, used)
	if caps != "" {
		var c codeStudioCapabilities
		if json.Unmarshal([]byte(caps), &c) == nil {
			rec.Capabilities = &c
		}
	}
	return rec, nil
}

func (s *codeStudioWorkspaceStore) List(ctx context.Context) ([]codeStudioWorkspaceRecord, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT `+codeStudioWorkspaceColumns+` FROM code_studio_workspaces ORDER BY last_used_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []codeStudioWorkspaceRecord{}
	for rows.Next() {
		rec, err := scanCodeStudioWorkspace(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, rec)
	}
	return out, rows.Err()
}

func (s *codeStudioWorkspaceStore) Get(ctx context.Context, id string) (codeStudioWorkspaceRecord, error) {
	return scanCodeStudioWorkspace(s.db.QueryRowContext(ctx, `SELECT `+codeStudioWorkspaceColumns+` FROM code_studio_workspaces WHERE id = ?`, id))
}

func capsJSON(c *codeStudioCapabilities) string {
	if c == nil {
		return ""
	}
	raw, _ := json.Marshal(c)
	return string(raw)
}

// Upsert saves device+root; an existing entry keeps its id and label unless a new label is given.
func (s *codeStudioWorkspaceStore) Upsert(ctx context.Context, deviceID, root, label string, caps *codeStudioCapabilities) (codeStudioWorkspaceRecord, error) {
	now := s.now().Format(time.RFC3339Nano)
	_, err := s.db.ExecContext(ctx, `INSERT INTO code_studio_workspaces (id, device_id, root, label, capabilities, created_at, last_used_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(device_id, root) DO UPDATE SET
			label = CASE WHEN excluded.label <> '' THEN excluded.label ELSE code_studio_workspaces.label END,
			capabilities = CASE WHEN excluded.capabilities <> '' THEN excluded.capabilities ELSE code_studio_workspaces.capabilities END,
			last_used_at = excluded.last_used_at`,
		newCodeStudioWorkspaceID(), deviceID, root, label, capsJSON(caps), now, now)
	if err != nil {
		return codeStudioWorkspaceRecord{}, fmt.Errorf("save code studio workspace: %w", err)
	}
	return scanCodeStudioWorkspace(s.db.QueryRowContext(ctx, `SELECT `+codeStudioWorkspaceColumns+` FROM code_studio_workspaces WHERE device_id = ? AND root = ?`, deviceID, root))
}

func (s *codeStudioWorkspaceStore) Touch(ctx context.Context, id string, caps *codeStudioCapabilities) error {
	_, err := s.db.ExecContext(ctx, `UPDATE code_studio_workspaces SET last_used_at = ?,
		capabilities = CASE WHEN ? <> '' THEN ? ELSE capabilities END WHERE id = ?`,
		s.now().Format(time.RFC3339Nano), capsJSON(caps), capsJSON(caps), id)
	return err
}

func (s *codeStudioWorkspaceStore) Delete(ctx context.Context, id string) error {
	res, err := s.db.ExecContext(ctx, `DELETE FROM code_studio_workspaces WHERE id = ?`, id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return errCodeStudioWorkspaceRecordNotFound
	}
	return nil
}

func (s *codeStudioWorkspaceStore) CountDevice(ctx context.Context, deviceID string) (int, error) {
	var n int
	err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM code_studio_workspaces WHERE device_id = ?`, deviceID).Scan(&n)
	return n, err
}
```

**Test first:** `internal/server/code_studio_workspace_store_test.go`.

```go
func TestCodeStudioWorkspaceStoreUpsertListDelete(t *testing.T) {
	s := testCodeStudioServerWithFakeCodeContainer(t, 1)
	store, err := codeStudioHandlers{server: s}.workspaceStore(context.Background())
	if err != nil { t.Fatal(err) }
	ctx := context.Background()
	a, err := store.Upsert(ctx, "dev1", "/home/u/proj", "Proj", &codeStudioCapabilities{Git: true})
	if err != nil || !strings.HasPrefix(a.ID, "ssh_") || !codeStudioWorkspaceIDPattern.MatchString(a.ID) { t.Fatal(a, err) }
	b, _ := store.Upsert(ctx, "dev1", "/home/u/proj", "", nil) // same device+root -> same id, label kept
	if b.ID != a.ID || b.Label != "Proj" || b.Capabilities == nil || !b.Capabilities.Git { t.Fatalf("%+v", b) }
	c, _ := store.Upsert(ctx, "dev1", "/srv/other", "", nil)
	list, _ := store.List(ctx)
	if len(list) != 2 || list[0].ID != c.ID { t.Fatalf("order %+v", list) }
	if n, _ := store.CountDevice(ctx, "dev1"); n != 2 { t.Fatal(n) }
	if _, err := store.Get(ctx, "ssh_nope"); !errors.Is(err, errCodeStudioWorkspaceRecordNotFound) { t.Fatal(err) }
	if err := store.Delete(ctx, a.ID); err != nil { t.Fatal(err) }
	if err := store.Delete(ctx, a.ID); !errors.Is(err, errCodeStudioWorkspaceRecordNotFound) { t.Fatal(err) }
}
```

`list[0]` relies on `c` having a later `last_used_at`. RFC3339Nano gives enough resolution; if the test is flaky on Windows timer resolution, set `store.now` to an incrementing fake clock.

**Run:** `go test ./internal/server/ -run '(?i)CodeStudioWorkspaceStore' -count=1`. Expected: PASS.

**Commit:** `feat(code-studio): persist saved SSH workspaces in the desktop database`

---

## Group 3: Desktop SSH helpers

### Task 8: `internal/desktop/ssh_workspace_dial.go` (8 min)

**File:** new.

```go
package desktop

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"os"
	"strconv"
	"strings"
	"time"

	"aurago/internal/credentials"
	"aurago/internal/inventory"
	"aurago/internal/remote"
	"aurago/internal/security"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"
)

const (
	SSHHostKeyUnknown  = "unknown_host_key"
	SSHHostKeyMismatch = "host_key_mismatch"
)

var ErrSSHDeviceNotFound = errors.New("device not found")

// SSHAccess is resolved access data of one SSH device. Secret must never be serialised.
type SSHAccess struct {
	DeviceID, Name, Host string
	Port                 int
	Username             string
	Secret               []byte `json:"-"`
}

// SSHHostKeyError: known_hosts does not vouch for the presented key.
type SSHHostKeyError struct{ Code, Host, KeyType, Fingerprint string }

func (e *SSHHostKeyError) Error() string {
	if e.Code == SSHHostKeyMismatch {
		return fmt.Sprintf("SSH host key of %s changed (%s %s)", e.Host, e.KeyType, e.Fingerprint)
	}
	return fmt.Sprintf("unknown SSH host key for %s (%s %s)", e.Host, e.KeyType, e.Fingerprint)
}

type SSHDialOptions struct {
	TrustFingerprint string        // stored in known_hosts only if it equals the presented key's SHA256 fingerprint
	KnownHostsFile   string        // "" = ~/.ssh/known_hosts
	Timeout          time.Duration // 0 = 15 s
	Logger           *slog.Logger
}

// ResolveSSHDevice loads a device and its secret (same rules as Quick Connect SSH).
func ResolveSSHDevice(inventoryDB *sql.DB, vault *security.Vault, deviceID string) (SSHAccess, error) {
	device, err := inventory.GetDeviceByID(inventoryDB, deviceID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) || strings.Contains(strings.ToLower(err.Error()), "not found") {
			return SSHAccess{}, fmt.Errorf("%w: %s", ErrSSHDeviceNotFound, deviceID)
		}
		return SSHAccess{}, err
	}
	host, port, user, secret, err := resolveSSHAccess(device, inventoryDB, vault)
	if err != nil {
		return SSHAccess{}, err
	}
	return SSHAccess{DeviceID: device.ID, Name: device.Name, Host: host, Port: port, Username: user, Secret: secret}, nil
}

// DescribeSSHDevice returns host/port/user without reading the vault (Secret stays nil).
func DescribeSSHDevice(inventoryDB *sql.DB, device inventory.DeviceRecord) SSHAccess {
	a := SSHAccess{DeviceID: device.ID, Name: device.Name, Host: strings.TrimSpace(device.IPAddress),
		Port: device.Port, Username: strings.TrimSpace(device.Username)}
	if a.Host == "" {
		a.Host = strings.TrimSpace(device.Name)
	}
	if a.Port <= 0 {
		a.Port = 22
	}
	if id := strings.TrimSpace(device.CredentialID); id != "" && inventoryDB != nil {
		if cred, err := credentials.GetByID(inventoryDB, id); err == nil {
			if strings.TrimSpace(cred.Host) != "" {
				a.Host = strings.TrimSpace(cred.Host)
			}
			if strings.TrimSpace(cred.Username) != "" {
				a.Username = strings.TrimSpace(cred.Username)
			}
		}
	}
	return a
}

// DialSSH connects without any interactive prompt. Unknown keys return *SSHHostKeyError
// unless TrustFingerprint matches; changed keys always return *SSHHostKeyError (mismatch).
func DialSSH(ctx context.Context, access SSHAccess, opts SSHDialOptions) (*ssh.Client, error) {
	var auth []ssh.AuthMethod
	if signer, err := ssh.ParsePrivateKey(access.Secret); err == nil {
		auth = append(auth, ssh.PublicKeys(signer))
	} else {
		auth = append(auth, ssh.Password(string(access.Secret)))
	}
	var captured *SSHHostKeyError
	callback := ssh.InsecureIgnoreHostKey() //nolint:gosec // explicit homelab opt-in, like Quick Connect
	if !remote.InsecureHostKey {
		file := opts.KnownHostsFile
		if file == "" {
			var err error
			if file, err = defaultKnownHostsFile(); err != nil {
				return nil, err
			}
		}
		callback = trustingKnownHostsCallback(file, strings.TrimSpace(opts.TrustFingerprint), opts.Logger, &captured)
	}
	timeout := opts.Timeout
	if timeout <= 0 {
		timeout = 15 * time.Second
	}
	cfg := &ssh.ClientConfig{User: access.Username, Auth: auth, HostKeyCallback: callback, Timeout: timeout}
	client, err := dialDesktopSSH(ctx, net.JoinHostPort(access.Host, strconv.Itoa(access.Port)), cfg)
	if err != nil {
		if captured != nil {
			return nil, captured
		}
		return nil, err
	}
	return client, nil
}

func trustingKnownHostsCallback(file, trust string, logger *slog.Logger, captured **SSHHostKeyError) ssh.HostKeyCallback {
	return func(hostname string, remoteAddr net.Addr, key ssh.PublicKey) error {
		hk := &SSHHostKeyError{Code: SSHHostKeyUnknown, Host: knownhosts.Normalize(hostname),
			KeyType: key.Type(), Fingerprint: ssh.FingerprintSHA256(key)}
		cb, err := knownhosts.New(file)
		switch {
		case err == nil:
			verr := cb(hostname, remoteAddr, key)
			if verr == nil {
				return nil
			}
			var keyErr *knownhosts.KeyError
			if !errors.As(verr, &keyErr) {
				return fmt.Errorf("SSH host key verification failed: %w", verr)
			}
			if len(keyErr.Want) > 0 {
				hk.Code = SSHHostKeyMismatch
				*captured = hk
				return hk
			}
		case !errors.Is(err, os.ErrNotExist):
			return fmt.Errorf("SSH host key verification failed: load known_hosts: %w", err)
		}
		if trust == "" || trust != hk.Fingerprint {
			*captured = hk
			return hk
		}
		if err := appendKnownHostKey(file, hostname, remoteAddr, key); err != nil {
			return fmt.Errorf("store SSH host key: %w", err)
		}
		if logger != nil {
			logger.Info("stored SSH host key from Code Studio", "host", hk.Host, "key_type", hk.KeyType)
		}
		return nil
	}
}
```

Only new code: `resolveSSHAccess`, `defaultKnownHostsFile`, `appendKnownHostKey` and `dialDesktopSSH` are called, not edited. The tests are in Task 15, which exercises the default dialer end to end.

**Run:** `go vet ./internal/desktop/` and `go build ./internal/desktop/`. Expected: no output.

**Commit:** `feat(desktop): non-interactive SSH dial with known_hosts trust for Code Studio`

---

## Group 4: SSH pool and backend

### Task 9: Pure helpers (path mapping, capabilities, grep) (10 min)

**File:** `internal/server/code_studio_ssh_backend.go` (start it).

```go
package server

import (
	"errors"
	pathpkg "path"
	"sort"
	"strings"
	"time"
)

var errCodeStudioOutsideRoot = errors.New("path is outside the workspace")

// normalizeCodeStudioRemoteRoot accepts absolute POSIX paths and ~ / ~/x (home = sftp Getwd).
func normalizeCodeStudioRemoteRoot(raw, home string) (string, error) {
	p := strings.TrimSpace(strings.ReplaceAll(raw, "\\", "/"))
	if strings.ContainsRune(p, 0) || p == "" {
		return "", errors.New("remote path is required")
	}
	if p == "~" || strings.HasPrefix(p, "~/") {
		if home == "" {
			return "", errors.New("remote home directory is unknown")
		}
		p = home + strings.TrimPrefix(p, "~")
	}
	if !strings.HasPrefix(p, "/") {
		return "", errors.New("remote path must be absolute")
	}
	return pathpkg.Clean(p), nil
}

func codeStudioRemoteInside(root, p string) bool {
	root, p = pathpkg.Clean(root), pathpkg.Clean(p)
	if root == "/" {
		return strings.HasPrefix(p, "/")
	}
	return p == root || strings.HasPrefix(p, root+"/")
}

// codeStudioRemotePath maps /workspace/<rel> to <root>/<rel>.
func codeStudioRemotePath(root, vpath string) (string, error) {
	v, err := sanitizeCodeStudioPath(vpath)
	if err != nil {
		return "", err
	}
	rel := strings.TrimPrefix(strings.TrimPrefix(v, codeStudioWorkspaceRoot), "/")
	if rel == "" {
		return pathpkg.Clean(root), nil
	}
	return pathpkg.Join(root, rel), nil
}

// codeStudioVirtualPath maps <root>/<rel> back to /workspace/<rel>.
func codeStudioVirtualPath(root, remote string) (string, error) {
	root, remote = pathpkg.Clean(root), pathpkg.Clean(remote)
	if !codeStudioRemoteInside(root, remote) {
		return "", errCodeStudioOutsideRoot
	}
	rel := strings.TrimPrefix(strings.TrimPrefix(remote, root), "/")
	if rel == "" {
		return codeStudioWorkspaceRoot, nil
	}
	return codeStudioWorkspaceRoot + "/" + rel, nil
}

func sortCodeStudioFileEntries(entries []codeStudioFileEntry) {
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].Type != entries[j].Type {
			return entries[i].Type == "directory"
		}
		return strings.ToLower(entries[i].Name) < strings.ToLower(entries[j].Name)
	})
}

// codeStudioSSHCommandLine wraps a POSIX script for the user's login shell. A fish
// login shell can alter "\\" inside the single quotes; documented limitation.
func codeStudioSSHCommandLine(script string) string { return "sh -c " + shellQuote(script) }

const codeStudioCapabilitiesScript = `printf 'os=%s\n' "$(uname -s 2>/dev/null)"
printf 'arch=%s\n' "$(uname -m 2>/dev/null)"
printf 'shell=%s\n' "${SHELL:-}"
for t in git grep gofmt gopls pyright-langserver typescript-language-server clangd python3; do
  if command -v "$t" >/dev/null 2>&1; then printf 'has=%s\n' "$t"; fi
done`

func parseCodeStudioCapabilities(output string, now time.Time) codeStudioCapabilities {
	caps := codeStudioCapabilities{Formatters: map[string]bool{"gofmt": false}, ProbedAt: now}
	for _, line := range strings.Split(output, "\n") {
		key, value, ok := strings.Cut(strings.TrimSpace(line), "=")
		if !ok {
			continue
		}
		switch key {
		case "os":
			caps.OS = value
		case "arch":
			caps.Arch = value
		case "shell":
			caps.Shell = value
		case "has":
			switch value {
			case "git":
				caps.Git = true
			case "grep":
				caps.Grep = true
			case "python3":
				caps.Python3 = true
			case "gofmt":
				caps.Formatters["gofmt"] = true
			case "gopls":
				caps.LSP.Go = true
			case "pyright-langserver":
				caps.LSP.Python = true
			case "typescript-language-server":
				caps.LSP.TypeScript = true
			case "clangd":
				caps.LSP.CPP = true
			}
		}
	}
	return caps
}

// codeStudioSSHGrepArgs: --null (not -Z: BSD grep reads -Z as decompress), -H for single files.
func codeStudioSSHGrepArgs(opts codeStudioSearchOptions, dir string) []string {
	args := []string{"grep", "-r", "-n", "-H", "-I", "-s", "--null"}
	if opts.Regex {
		args = append(args, "-E")
	} else {
		args = append(args, "-F")
	}
	if !opts.CaseSensitive {
		args = append(args, "-i")
	}
	if opts.WholeWord {
		args = append(args, "-w")
	}
	if inc := sanitizeCodeStudioGlob(opts.Include); inc != "" {
		args = append(args, "--include="+inc)
	}
	if exc := sanitizeCodeStudioExclude(opts.Exclude); exc != "" {
		args = append(args, "--exclude="+exc, "--exclude-dir="+exc)
	}
	return append(args, "-m", "201", "-e", opts.Query, "--", dir)
}

// rewriteCodeStudioGrepPaths maps "remote\0rest\n" records to virtual paths; records
// outside root or incomplete are dropped.
func rewriteCodeStudioGrepPaths(root, output string) string {
	var b strings.Builder
	for output != "" {
		name, rest, ok := strings.Cut(output, "\x00")
		if !ok {
			break
		}
		line, tail, ended := strings.Cut(rest, "\n")
		output = tail
		if !ended {
			break
		}
		if v, err := codeStudioVirtualPath(root, name); err == nil {
			b.WriteString(v + "\x00" + line + "\n")
		}
	}
	return b.String()
}
```

**Tests (append to `code_studio_backend_test.go`):**

```go
func TestCodeStudioRemotePathMapping(t *testing.T) {
	cases := []struct{ root, v, want string; err bool }{
		{"/home/u/p", "/workspace", "/home/u/p", false}, {"/home/u/p", "/workspace/src/a.go", "/home/u/p/src/a.go", false},
		{"/", "/workspace/etc", "/etc", false}, {"/home/u/p", "/workspace/../x", "", true}, {"/home/u/p", "/etc/passwd", "", true},
	}
	for _, c := range cases {
		got, err := codeStudioRemotePath(c.root, c.v)
		if (err != nil) != c.err || got != c.want { t.Fatalf("%+v -> %q %v", c, got, err) }
	}
	if v, _ := codeStudioVirtualPath("/home/u/p", "/home/u/p/src/a.go"); v != "/workspace/src/a.go" { t.Fatal(v) }
	if _, err := codeStudioVirtualPath("/home/u/p", "/home/u/proj2/a"); err == nil { t.Fatal("prefix sibling accepted") }
	if r, _ := normalizeCodeStudioRemoteRoot(`~/code\x`, "/home/u"); r != "/home/u/code/x" { t.Fatal(r) }
	if _, err := normalizeCodeStudioRemoteRoot("rel/dir", "/home/u"); err == nil { t.Fatal("relative accepted") }
}

func TestCodeStudioCapabilitiesAndGrepRewrite(t *testing.T) {
	caps := parseCodeStudioCapabilities("os=Linux\narch=x86_64\nshell=/bin/bash\nhas=git\nhas=gopls\nhas=gofmt\n", time.Unix(1, 0))
	if caps.OS != "Linux" || !caps.Git || caps.Grep || !caps.LSP.Go || !caps.Formatters["gofmt"] || caps.LSP.CPP { t.Fatalf("%+v", caps) }
	out := rewriteCodeStudioGrepPaths("/home/u/p", "/home/u/p/a.go\x003:x\n/etc/passwd\x001:root\n/home/u/p/b\x00partial")
	if out != "/workspace/a.go\x003:x\n" { t.Fatalf("%q", out) }
	args := strings.Join(codeStudioSSHGrepArgs(codeStudioSearchOptions{Query: "q", Exclude: "node_modules"}, "/d"), " ")
	for _, want := range []string{"--null", "-H", "-F", "-i", "--exclude-dir=node_modules", "-e q -- /d"} {
		if !strings.Contains(args, want) { t.Fatal(args) }
	}
}
```

**Run:** `go test ./internal/server/ -run '(?i)CodeStudio(RemotePath|Capabilities)' -count=1`. Expected: PASS.

**Commit:** `feat(code-studio): remote path mapping, capability parsing and grep record mapping`

### Task 10: SSH pool, connection, remote state, default dialer, caller gate (10 min)

**File:** `internal/server/code_studio_ssh_pool.go`.

```go
package server

import (
	"bytes"
	"context"
	"errors"
	"io"
	"log/slog"
	"net"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"aurago/internal/desktop"

	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
)

const (
	codeStudioSSHKeepalive = 30 * time.Second
	codeStudioSSHIdleClose = 10 * time.Minute
	codeStudioSSHExecSlots = 4 // short sessions; + 4 long + 1 SFTP stays below OpenSSH MaxSessions 10
	codeStudioSSHLongSlots = 4 // terminals + processes
)

type codeStudioSSHTarget struct {
	DeviceID, Name, Host string
	Port                 int
	User                 string
}

type codeStudioSSHDialFunc func(ctx context.Context, deviceID, trustFingerprint string) (*ssh.Client, codeStudioSSHTarget, error)

type codeStudioSSHSessionOpenError struct{ err error }

func (e codeStudioSSHSessionOpenError) Error() string { return "open SSH session: " + e.err.Error() }
func (e codeStudioSSHSessionOpenError) Unwrap() error { return e.err }

func codeStudioSSHRetrySessionOpen(err error) bool {
	var open codeStudioSSHSessionOpenError
	return errors.As(err, &open)
}

func codeStudioSSHRetryAlways(error) bool { return true }

func codeStudioSSHBroken(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) || errors.Is(err, net.ErrClosed) ||
		errors.Is(err, sftp.ErrSSHFxConnectionLost) || errors.Is(err, sftp.ErrSSHFxNoConnection) {
		return true
	}
	text := err.Error()
	return strings.Contains(text, "use of closed network connection") || strings.Contains(text, "connection lost")
}

type codeStudioSSHConn struct {
	target    codeStudioSSHTarget
	client    *ssh.Client
	sftp      *sftp.Client
	execSem   chan struct{}
	longSem   chan struct{}
	lastUsed  atomic.Int64
	done      chan struct{}
	closeOnce sync.Once
	rootsMu   sync.Mutex
	realRoots map[string]string
}

func (c *codeStudioSSHConn) dead() bool {
	select {
	case <-c.done:
		return true
	default:
		return false
	}
}

func (c *codeStudioSSHConn) touch()     { c.lastUsed.Store(time.Now().UnixNano()) }
func (c *codeStudioSSHConn) busy() bool { return len(c.longSem) > 0 || len(c.execSem) > 0 }

func (c *codeStudioSSHConn) close() {
	c.closeOnce.Do(func() { _ = c.sftp.Close(); _ = c.client.Close() })
}

// run executes cmdline in one session. stop (optional) kills it early (search output full).
func (c *codeStudioSSHConn) run(ctx context.Context, cmdline string, stdin []byte, stdout, stderr io.Writer, stop <-chan struct{}) (int, error) {
	select {
	case c.execSem <- struct{}{}:
	case <-ctx.Done():
		return -1, ctx.Err()
	}
	defer func() { <-c.execSem; c.touch() }()
	session, err := c.client.NewSession()
	if err != nil {
		return -1, codeStudioSSHSessionOpenError{err}
	}
	defer session.Close()
	if stdin != nil {
		session.Stdin = bytes.NewReader(stdin)
	}
	session.Stdout, session.Stderr = stdout, stderr
	if err := session.Start(cmdline); err != nil {
		return -1, codeStudioSSHSessionOpenError{err}
	}
	waitErr := make(chan error, 1)
	go func() { waitErr <- session.Wait() }()
	kill := func() {
		_ = session.Signal(ssh.SIGKILL)
		_ = session.Close()
		select {
		case <-waitErr:
		case <-time.After(2 * time.Second):
		}
	}
	select {
	case err := <-waitErr:
		return codeStudioSSHExitCode(err)
	case <-stop:
		kill()
		return 0, nil
	case <-ctx.Done():
		kill()
		return -1, ctx.Err()
	}
}

func codeStudioSSHExitCode(err error) (int, error) {
	if err == nil {
		return 0, nil
	}
	var exitErr *ssh.ExitError
	if errors.As(err, &exitErr) {
		return exitErr.ExitStatus(), nil
	}
	var missing *ssh.ExitMissingError
	if errors.As(err, &missing) {
		return -1, nil
	}
	return -1, err
}

type codeStudioSSHPool struct {
	dial      codeStudioSSHDialFunc
	logger    *slog.Logger
	keepalive time.Duration
	idle      time.Duration
	mu        sync.Mutex
	conns     map[string]*codeStudioSSHConn
	dialLocks map[string]*sync.Mutex
	janitor   sync.Once
	stopOnce  sync.Once
	stop      chan struct{}
}

func newCodeStudioSSHPool(dial codeStudioSSHDialFunc, logger *slog.Logger) *codeStudioSSHPool {
	return &codeStudioSSHPool{dial: dial, logger: logger, keepalive: codeStudioSSHKeepalive, idle: codeStudioSSHIdleClose,
		conns: map[string]*codeStudioSSHConn{}, dialLocks: map[string]*sync.Mutex{}, stop: make(chan struct{})}
}

func (p *codeStudioSSHPool) dialLock(deviceID string) *sync.Mutex {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.dialLocks[deviceID] == nil {
		p.dialLocks[deviceID] = &sync.Mutex{}
	}
	return p.dialLocks[deviceID]
}

// get returns the pooled connection or dials one. trust is only used for a new dial.
func (p *codeStudioSSHPool) get(ctx context.Context, deviceID, trust string) (*codeStudioSSHConn, error) {
	p.janitor.Do(func() { go p.reap() })
	lock := p.dialLock(deviceID)
	lock.Lock()
	defer lock.Unlock()
	p.mu.Lock()
	if c := p.conns[deviceID]; c != nil && !c.dead() {
		p.mu.Unlock()
		c.touch()
		return c, nil
	}
	p.mu.Unlock()
	client, target, err := p.dial(ctx, deviceID, trust)
	if err != nil {
		return nil, err
	}
	sc, err := sftp.NewClient(client)
	if err != nil {
		_ = client.Close()
		return nil, &codeStudioRemoteError{Status: http.StatusBadGateway, Code: "sftp_unavailable",
			Message: "SFTP is not available on " + target.Host + ": " + err.Error(), Err: err}
	}
	c := &codeStudioSSHConn{target: target, client: client, sftp: sc, execSem: make(chan struct{}, codeStudioSSHExecSlots),
		longSem: make(chan struct{}, codeStudioSSHLongSlots), done: make(chan struct{}), realRoots: map[string]string{}}
	c.touch()
	go func() { _ = client.Wait(); close(c.done); c.close() }()
	go p.keepAlive(c)
	p.mu.Lock()
	if old := p.conns[deviceID]; old != nil {
		old.close()
	}
	p.conns[deviceID] = c
	p.mu.Unlock()
	return c, nil
}

func (p *codeStudioSSHPool) keepAlive(c *codeStudioSSHConn) {
	ticker := time.NewTicker(p.keepalive)
	defer ticker.Stop()
	for {
		select {
		case <-c.done:
			return
		case <-p.stop:
			return
		case <-ticker.C:
			reply := make(chan error, 1)
			go func() { _, _, err := c.client.SendRequest("keepalive@openssh.com", true, nil); reply <- err }()
			select {
			case err := <-reply:
				if err != nil {
					c.close()
					return
				}
			case <-time.After(15 * time.Second):
				c.close()
				return
			case <-c.done:
				return
			}
		}
	}
}

func (p *codeStudioSSHPool) reap() {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-p.stop:
			return
		case now := <-ticker.C:
			p.closeIdle(now)
		}
	}
}

func (p *codeStudioSSHPool) closeIdle(now time.Time) {
	p.mu.Lock()
	defer p.mu.Unlock()
	for id, c := range p.conns {
		if c.dead() || (!c.busy() && now.Sub(time.Unix(0, c.lastUsed.Load())) > p.idle) {
			c.close()
			delete(p.conns, id)
		}
	}
}

func (p *codeStudioSSHPool) drop(deviceID string, c *codeStudioSSHConn) {
	p.mu.Lock()
	if p.conns[deviceID] == c {
		delete(p.conns, deviceID)
	}
	p.mu.Unlock()
	c.close()
}

// release closes an idle connection (workspace forgotten); busy ones are left to the reaper.
func (p *codeStudioSSHPool) release(deviceID string) {
	p.mu.Lock()
	c := p.conns[deviceID]
	if c != nil && !c.busy() {
		delete(p.conns, deviceID)
	}
	p.mu.Unlock()
	if c != nil && !c.busy() {
		c.close()
	}
}

func (p *codeStudioSSHPool) connected(deviceID string) (codeStudioSSHTarget, bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if c := p.conns[deviceID]; c != nil && !c.dead() {
		return c.target, true
	}
	return codeStudioSSHTarget{}, false
}

func (p *codeStudioSSHPool) closeAll() {
	p.stopOnce.Do(func() { close(p.stop) })
	p.mu.Lock()
	defer p.mu.Unlock()
	for id, c := range p.conns {
		c.close()
		delete(p.conns, id)
	}
}

// withConn runs fn and, when retryable(err) and the connection is dead/broken, redials once.
func (p *codeStudioSSHPool) withConn(ctx context.Context, deviceID string, retryable func(error) bool, fn func(*codeStudioSSHConn) error) error {
	c, err := p.get(ctx, deviceID, "")
	if err != nil {
		return err
	}
	err = fn(c)
	if err == nil || retryable == nil || !retryable(err) || !(c.dead() || codeStudioSSHBroken(err)) {
		return err
	}
	p.drop(deviceID, c)
	c, err = p.get(ctx, deviceID, "")
	if err != nil {
		return err
	}
	return fn(c)
}

// ---- per-server remote state ----

type codeStudioRemote struct {
	pool    *codeStudioSSHPool
	touchMu sync.Mutex
	touched map[string]time.Time
}

func newCodeStudioRemote(dial codeStudioSSHDialFunc, logger *slog.Logger) *codeStudioRemote {
	return &codeStudioRemote{pool: newCodeStudioSSHPool(dial, logger), touched: map[string]time.Time{}}
}

// shouldTouch throttles last_used_at writes to one per workspace per 5 minutes.
func (r *codeStudioRemote) shouldTouch(id string, now time.Time) bool {
	r.touchMu.Lock()
	defer r.touchMu.Unlock()
	if now.Sub(r.touched[id]) < 5*time.Minute {
		return false
	}
	r.touched[id] = now
	return true
}

var codeStudioRemotes sync.Map // *Server -> *codeStudioRemote

func (h codeStudioHandlers) remoteState() *codeStudioRemote {
	if h.remote != nil {
		return h.remote
	}
	if v, ok := codeStudioRemotes.Load(h.server); ok {
		return v.(*codeStudioRemote)
	}
	v, _ := codeStudioRemotes.LoadOrStore(h.server, newCodeStudioRemote(codeStudioDefaultSSHDialer(h.server), h.codeStudioLogger()))
	return v.(*codeStudioRemote)
}

func codeStudioDefaultSSHDialer(s *Server) codeStudioSSHDialFunc {
	return func(ctx context.Context, deviceID, trust string) (*ssh.Client, codeStudioSSHTarget, error) {
		if s == nil || s.InventoryDB == nil {
			return nil, codeStudioSSHTarget{}, &codeStudioRemoteError{Status: http.StatusServiceUnavailable,
				Code: "inventory_unavailable", Message: "inventory database not configured"}
		}
		access, err := desktop.ResolveSSHDevice(s.InventoryDB, s.Vault, deviceID)
		if errors.Is(err, desktop.ErrSSHDeviceNotFound) {
			return nil, codeStudioSSHTarget{}, &codeStudioRemoteError{Status: http.StatusNotFound, Code: "device_not_found", Message: "device not found"}
		}
		if err != nil {
			return nil, codeStudioSSHTarget{}, &codeStudioRemoteError{Status: http.StatusBadRequest, Code: "ssh_access_invalid", Message: err.Error()}
		}
		target := codeStudioSSHTarget{DeviceID: access.DeviceID, Name: access.Name, Host: access.Host, Port: access.Port, User: access.Username}
		client, err := desktop.DialSSH(ctx, access, desktop.SSHDialOptions{TrustFingerprint: trust, Logger: s.Logger})
		if err != nil {
			return nil, target, codeStudioClassifyDialError(err, target)
		}
		return client, target, nil
	}
}

func codeStudioClassifyDialError(err error, target codeStudioSSHTarget) error {
	var hk *desktop.SSHHostKeyError
	if errors.As(err, &hk) {
		msg := "Unknown SSH host key for " + hk.Host
		if hk.Code == desktop.SSHHostKeyMismatch {
			msg = "The SSH host key of " + hk.Host + " changed. Check the host, then fix ~/.ssh/known_hosts on the AuraGo server."
		}
		return &codeStudioRemoteError{Status: http.StatusConflict, Code: hk.Code, Message: msg,
			Host: hk.Host, KeyType: hk.KeyType, Fingerprint: hk.Fingerprint, Err: err}
	}
	if strings.Contains(err.Error(), "unable to authenticate") {
		return &codeStudioRemoteError{Status: http.StatusBadGateway, Code: "ssh_auth_failed",
			Message: "SSH authentication failed for " + target.User + "@" + target.Host, Err: err}
	}
	return &codeStudioRemoteError{Status: http.StatusBadGateway, Code: "ssh_unreachable",
		Message: "SSH connection to " + target.Host + " failed: " + err.Error(), Err: err}
}

// codeStudioRemoteCallerAllowed: SSH workspaces are remote access; bearer tokens need desktop:admin.
func codeStudioRemoteCallerAllowed(s *Server, r *http.Request) bool {
	raw, isBearer := bearerCredential(r.Header.Get("Authorization"))
	if !isBearer {
		return true // cookie/no-auth callers already passed requirePermission
	}
	tm := s.currentTokenManager()
	if tm == nil || raw == "" {
		return false
	}
	_, ok := tm.Validate(raw, desktopScopeAdmin)
	return ok
}

func codeStudioRemoteScopeError() error {
	return &codeStudioRemoteError{Status: http.StatusForbidden, Code: "remote_scope_required",
		Message: "SSH workspaces need a session or a desktop:admin token"}
}
```

**Edit `code_studio_handlers.go`:** add the field `remote *codeStudioRemote` to `codeStudioHandlers`. The comment should say "nil = per-server default (remoteState)".

**Run:** `go vet ./internal/server/` and the full CodeStudio suite. Expected: PASS. The pool has no test of its own yet; Tasks 11 to 14 exercise it.

**Commit:** `feat(code-studio): pooled SSH connections with keepalive, idle close and one redial`

### Task 11: Test fixture and SSH backend file operations (10 min)

**Fixture:** `internal/server/code_studio_ssh_fixture_test.go`.

```go
type codeStudioSSHFixture struct {
	addr           string
	host           string
	port           int
	hostKey        ssh.PublicKey
	user, password string
	mu             sync.Mutex
	execs          []string
	windows        [][2]int // cols, rows (pty-req and window-change)
	exec           func(cmd string, stdin []byte) (string, uint32)
}

func startCodeStudioSSHFixture(t *testing.T) *codeStudioSSHFixture {
	t.Helper()
	_, key, _ := ed25519.GenerateKey(rand.Reader)
	signer, _ := ssh.NewSignerFromKey(key)
	f := &codeStudioSSHFixture{hostKey: signer.PublicKey(), user: "dev", password: "fixture-pw",
		exec: func(cmd string, _ []byte) (string, uint32) {
			if strings.Contains(cmd, "uname") {
				return "os=Linux\narch=x86_64\nshell=/bin/bash\nhas=git\nhas=grep\nhas=gofmt\nhas=gopls\n", 0
			}
			return "", 0
		}}
	cfg := &ssh.ServerConfig{PasswordCallback: func(c ssh.ConnMetadata, pw []byte) (*ssh.Permissions, error) {
		if c.User() == f.user && string(pw) == f.password { return nil, nil }
		return nil, fmt.Errorf("denied")
	}}
	cfg.AddHostKey(signer)
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil { t.Fatal(err) }
	f.addr = ln.Addr().String()
	f.host, f.port = "127.0.0.1", ln.Addr().(*net.TCPAddr).Port
	fs := sftp.InMemHandler()
	var wg sync.WaitGroup
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil { return }
			wg.Add(1)
			go func() { defer wg.Done(); f.serve(conn, cfg, fs) }()
		}
	}()
	t.Cleanup(func() { _ = ln.Close(); wg.Wait() })
	return f
}

func (f *codeStudioSSHFixture) serve(raw net.Conn, cfg *ssh.ServerConfig, fs sftp.Handlers) {
	defer raw.Close()
	sc, chans, reqs, err := ssh.NewServerConn(raw, cfg)
	if err != nil { return }
	defer sc.Close()
	go ssh.DiscardRequests(reqs) // keepalive@openssh.com gets "false", which is not an error
	for nc := range chans {
		if nc.ChannelType() != "session" { _ = nc.Reject(ssh.UnknownChannelType, "session only"); continue }
		ch, creqs, err := nc.Accept()
		if err != nil { return }
		go f.session(ch, creqs, fs)
	}
}

func (f *codeStudioSSHFixture) session(ch ssh.Channel, reqs <-chan *ssh.Request, fs sftp.Handlers) {
	pty := false
	for req := range reqs {
		switch req.Type {
		case "pty-req":
			var p struct{ Term string; Cols, Rows, W, H uint32; Modes string }
			_ = ssh.Unmarshal(req.Payload, &p)
			f.mu.Lock(); f.windows = append(f.windows, [2]int{int(p.Cols), int(p.Rows)}); f.mu.Unlock()
			pty = true
			_ = req.Reply(true, nil)
		case "window-change":
			var p struct{ Cols, Rows, W, H uint32 }
			_ = ssh.Unmarshal(req.Payload, &p)
			f.mu.Lock(); f.windows = append(f.windows, [2]int{int(p.Cols), int(p.Rows)}); f.mu.Unlock()
		case "subsystem":
			_ = req.Reply(true, nil)
			srv := sftp.NewRequestServer(ch, fs)
			go func() { _ = srv.Serve(); _ = srv.Close() }()
		case "exec":
			var p struct{ Command string }
			_ = ssh.Unmarshal(req.Payload, &p)
			f.mu.Lock(); f.execs = append(f.execs, p.Command); f.mu.Unlock()
			_ = req.Reply(true, nil)
			go func(cmd string, echo bool) {
				var code uint32
				if echo { // PTY shell or StartProcess("aurago-echo"): echo until stdin EOF
					_, _ = io.Copy(ch, ch)
				} else {
					stdin, _ := io.ReadAll(ch)
					out, c := f.exec(cmd, stdin)
					_, _ = io.WriteString(ch, out)
					code = c
				}
				_, _ = ch.SendRequest("exit-status", false, ssh.Marshal(struct{ Status uint32 }{code}))
				_ = ch.Close()
			}(p.Command, pty || strings.Contains(p.Command, "aurago-echo"))
		default:
			if req.WantReply { _ = req.Reply(req.Type == "signal", nil) }
		}
	}
}

func (f *codeStudioSSHFixture) lastExec() string { f.mu.Lock(); defer f.mu.Unlock(); return f.execs[len(f.execs)-1] }

// unwrapShC reverses codeStudioSSHCommandLine for assertions.
func unwrapShC(cmd string) string {
	return strings.ReplaceAll(strings.TrimSuffix(strings.TrimPrefix(cmd, "sh -c '"), "'"), `'"'"'`, "'")
}

func (f *codeStudioSSHFixture) knownHostsFile(t *testing.T) string {
	p := filepath.Join(t.TempDir(), "known_hosts")
	if err := os.WriteFile(p, []byte(knownhosts.Line([]string{f.addr}, f.hostKey)+"\n"), 0o600); err != nil { t.Fatal(err) }
	return p
}

// testDial dials the fixture for any device id (no inventory).
func (f *codeStudioSSHFixture) testDial(t *testing.T) codeStudioSSHDialFunc {
	kh := f.knownHostsFile(t)
	return func(ctx context.Context, deviceID, trust string) (*ssh.Client, codeStudioSSHTarget, error) {
		target := codeStudioSSHTarget{DeviceID: deviceID, Name: "fixture", Host: f.host, Port: f.port, User: f.user}
		c, err := desktop.DialSSH(ctx, desktop.SSHAccess{Host: f.host, Port: f.port, Username: f.user, Secret: []byte(f.password)},
			desktop.SSHDialOptions{KnownHostsFile: kh, TrustFingerprint: trust})
		if err != nil { return nil, target, codeStudioClassifyDialError(err, target) }
		return c, target, nil
	}
}

// adminSFTP gives the test a direct client (host key ignored: test only) to seed files.
func (f *codeStudioSSHFixture) adminSFTP(t *testing.T) *sftp.Client {
	c, err := ssh.Dial("tcp", f.addr, &ssh.ClientConfig{User: f.user, Auth: []ssh.AuthMethod{ssh.Password(f.password)},
		HostKeyCallback: ssh.InsecureIgnoreHostKey()}) //nolint:gosec
	if err != nil { t.Fatal(err) }
	sc, err := sftp.NewClient(c)
	if err != nil { t.Fatal(err) }
	t.Cleanup(func() { _ = sc.Close(); _ = c.Close() })
	return sc
}

func newCodeStudioTestSSHBackend(t *testing.T, f *codeStudioSSHFixture, root string) (*codeStudioSSHBackend, *codeStudioRemote) {
	t.Helper()
	if err := f.adminSFTP(t).MkdirAll(root); err != nil { t.Fatal(err) }
	remote := newCodeStudioRemote(f.testDial(t), slog.Default())
	t.Cleanup(remote.pool.closeAll)
	conn, err := remote.pool.get(context.Background(), "dev1", "")
	if err != nil { t.Fatal(err) }
	return newCodeStudioSSHBackend(codeStudioWorkspaceRecord{ID: "ssh_test", DeviceID: "dev1", Root: root}, remote.pool, conn.target), remote
}
```

The struct fields of the `pty-req` payload must follow RFC 4254 order: term, cols, rows, width px, height px, modes.

**Backend file operations:** append to `code_studio_ssh_backend.go`, adding imports `context`, `crypto/rand`, `fmt`, `io`, `net/http`, `os`, `github.com/pkg/sftp`.

```go
type codeStudioSSHBackend struct {
	rec    codeStudioWorkspaceRecord
	root   string
	pool   *codeStudioSSHPool
	target codeStudioSSHTarget
}

var _ codeStudioBackend = (*codeStudioSSHBackend)(nil)

func newCodeStudioSSHBackend(rec codeStudioWorkspaceRecord, pool *codeStudioSSHPool, target codeStudioSSHTarget) *codeStudioSSHBackend {
	return &codeStudioSSHBackend{rec: rec, root: pathpkg.Clean(rec.Root), pool: pool, target: target}
}

func codeStudioSSHWorkspaceInfo(rec codeStudioWorkspaceRecord, target codeStudioSSHTarget, connected bool) codeStudioWorkspaceInfo {
	info := codeStudioWorkspaceInfo{ID: rec.ID, Kind: codeStudioWorkspaceKindSSH, Label: rec.Label, Root: rec.Root,
		DeviceID: rec.DeviceID, DeviceName: target.Name, Host: target.Host, Port: target.Port, User: target.User,
		Connected: connected, Capabilities: rec.Capabilities}
	if !rec.CreatedAt.IsZero() {
		created := rec.CreatedAt
		info.CreatedAt = &created
	}
	if !rec.LastUsedAt.IsZero() {
		used := rec.LastUsedAt
		info.LastUsedAt = &used
	}
	return info
}

func (b *codeStudioSSHBackend) Info() codeStudioWorkspaceInfo {
	_, connected := b.pool.connected(b.rec.DeviceID)
	return codeStudioSSHWorkspaceInfo(b.rec, b.target, connected)
}

func (b *codeStudioSSHBackend) remote(vpath string) (string, error) {
	p, err := codeStudioRemotePath(b.root, vpath)
	if err != nil {
		return "", codeStudioBadRequest(err.Error())
	}
	return p, nil
}

func (b *codeStudioSSHBackend) sftpDo(ctx context.Context, retry bool, fn func(*codeStudioSSHConn) error) error {
	var retryable func(error) bool
	if retry {
		retryable = codeStudioSSHRetryAlways
	}
	return b.pool.withConn(ctx, b.rec.DeviceID, retryable, fn)
}

func codeStudioSFTPNotExist(err error) bool {
	var status *sftp.StatusError
	return errors.Is(err, os.ErrNotExist) || (errors.As(err, &status) && status.Code == uint32(sftp.ErrSSHFxNoSuchFile))
}

func codeStudioSFTPError(err error, what string) error {
	var status *sftp.StatusError
	switch {
	case err == nil:
		return nil
	case codeStudioSSHBroken(err):
		return &codeStudioRemoteError{Status: http.StatusBadGateway, Code: "ssh_connection_lost", Message: "the SSH connection was lost", Err: err}
	case codeStudioSFTPNotExist(err):
		return codeStudioBadRequest(what + ": no such file or directory")
	case errors.Is(err, os.ErrPermission):
		return codeStudioBadRequest(what + ": permission denied")
	case errors.Is(err, os.ErrExist):
		return codeStudioExistsError(what + ": already exists")
	case errors.As(err, &status):
		return codeStudioBadRequest(what + ": " + status.Error())
	}
	return err
}

func (b *codeStudioSSHBackend) realRoot(c *codeStudioSSHConn) (string, error) {
	c.rootsMu.Lock()
	defer c.rootsMu.Unlock()
	if real, ok := c.realRoots[b.root]; ok {
		return real, nil
	}
	real, err := c.sftp.RealPath(b.root)
	if err != nil {
		return "", codeStudioSFTPError(err, "workspace root")
	}
	c.realRoots[b.root] = pathpkg.Clean(real)
	return c.realRoots[b.root], nil
}

// confine is a UX guard, NOT a security boundary: the same SSH user has a full shell in
// the terminal and exec. It stops editor operations from silently following a symlink
// out of the workspace. The nearest existing ancestor is resolved with RealPath.
func (b *codeStudioSSHBackend) confine(c *codeStudioSSHConn, remote string) error {
	realRoot, err := b.realRoot(c)
	if err != nil {
		return err
	}
	for p := remote; ; p = pathpkg.Dir(p) {
		real, err := c.sftp.RealPath(p)
		if err == nil {
			if !codeStudioRemoteInside(realRoot, real) {
				return codeStudioProbeRejection{message: "path is outside the workspace"}
			}
			return nil
		}
		if !codeStudioSFTPNotExist(err) {
			return codeStudioSFTPError(err, p)
		}
		if p == b.root || p == "/" {
			return nil
		}
	}
}

func (b *codeStudioSSHBackend) List(ctx context.Context, vdir string) ([]codeStudioFileEntry, error) {
	dir, err := b.remote(vdir)
	if err != nil {
		return nil, err
	}
	var infos []os.FileInfo
	err = b.sftpDo(ctx, true, func(c *codeStudioSSHConn) error {
		if err := b.confine(c, dir); err != nil {
			return err
		}
		list, err := c.sftp.ReadDir(dir)
		infos = list
		return codeStudioSFTPError(err, vdir)
	})
	if err != nil {
		return nil, err
	}
	entries := make([]codeStudioFileEntry, 0, len(infos))
	for _, info := range infos {
		kind := "file" // ReadDir is lstat-based: a symlinked dir is a file, as locally
		if info.IsDir() {
			kind = "directory"
		}
		entries = append(entries, codeStudioFileEntry{Name: info.Name(), Path: pathpkg.Join(vdir, info.Name()),
			Type: kind, Size: info.Size(), Modified: info.ModTime().UTC()})
	}
	sortCodeStudioFileEntries(entries)
	return entries, nil
}

func (b *codeStudioSSHBackend) Probe(ctx context.Context, vpath string) (codeStudioFileProbe, error) {
	target, err := b.remote(vpath)
	if err != nil {
		return codeStudioFileProbe{}, codeStudioProbeRejection{message: err.Error()}
	}
	unavailable := codeStudioProbeRejection{message: "file is outside the workspace or unavailable"}
	var probe codeStudioFileProbe
	err = b.sftpDo(ctx, true, func(c *codeStudioSSHConn) error {
		realRoot, err := b.realRoot(c)
		if err != nil {
			return err
		}
		real, err := c.sftp.RealPath(target)
		if err != nil {
			if codeStudioSFTPNotExist(err) {
				return unavailable
			}
			return codeStudioSFTPError(err, vpath)
		}
		virtual, err := codeStudioVirtualPath(realRoot, real)
		if err != nil {
			return unavailable
		}
		info, err := c.sftp.Stat(real)
		if err != nil {
			if codeStudioSFTPNotExist(err) {
				return unavailable
			}
			return codeStudioSFTPError(err, vpath)
		}
		if !info.Mode().IsRegular() {
			return codeStudioProbeRejection{message: "not a regular file"}
		}
		probe = codeStudioFileProbe{Path: virtual, Size: info.Size(), Modified: info.ModTime().UTC()}
		return nil
	})
	return probe, err
}

func (b *codeStudioSSHBackend) Read(ctx context.Context, vpath string, max int64) ([]byte, error) {
	target, err := b.remote(vpath)
	if err != nil {
		return nil, err
	}
	var data []byte
	err = b.sftpDo(ctx, true, func(c *codeStudioSSHConn) error {
		if err := b.confine(c, target); err != nil {
			return err
		}
		f, err := c.sftp.Open(target)
		if err != nil {
			return codeStudioSFTPError(err, vpath)
		}
		defer f.Close()
		if data, err = io.ReadAll(io.LimitReader(f, max+1)); err != nil {
			return codeStudioSFTPError(err, vpath)
		}
		if int64(len(data)) > max {
			return &codeStudioStatusError{Status: http.StatusRequestEntityTooLarge, Message: "file exceeds configured maximum size"}
		}
		return nil
	})
	return data, err
}

func (b *codeStudioSSHBackend) Write(ctx context.Context, vpath string, content []byte, createOnly bool) error {
	target, err := b.remote(vpath)
	if err != nil {
		return err
	}
	if target == b.root {
		return codeStudioBadRequest("cannot write the workspace root")
	}
	parent := pathpkg.Dir(target)
	return b.sftpDo(ctx, !createOnly, func(c *codeStudioSSHConn) error {
		if err := b.confine(c, parent); err != nil {
			return err
		}
		if err := c.sftp.MkdirAll(parent); err != nil {
			return codeStudioSFTPError(err, vpath)
		}
		if err := b.confine(c, parent); err != nil {
			return err
		}
		mode, existed := os.FileMode(0o644), false
		info, err := c.sftp.Lstat(target)
		switch {
		case err == nil:
			if createOnly {
				return codeStudioExistsError("file already exists")
			}
			if !info.Mode().IsRegular() {
				return codeStudioBadRequest("target must be a regular file, not a symlink or directory")
			}
			mode, existed = info.Mode().Perm(), true
		case !codeStudioSFTPNotExist(err):
			return codeStudioSFTPError(err, vpath)
		}
		if createOnly {
			return codeStudioSFTPWriteExclusive(c.sftp, target, content, mode)
		}
		return codeStudioSFTPReplace(c.sftp, parent, target, content, mode, existed)
	})
}

func codeStudioSFTPWriteExclusive(client *sftp.Client, target string, content []byte, mode os.FileMode) error {
	f, err := client.OpenFile(target, os.O_WRONLY|os.O_CREATE|os.O_EXCL)
	if err != nil {
		if errors.Is(err, os.ErrExist) {
			return codeStudioExistsError("file already exists")
		}
		return codeStudioSFTPError(err, target)
	}
	if _, err := f.Write(content); err != nil {
		_ = f.Close()
		_ = client.Remove(target)
		return codeStudioSFTPError(err, target)
	}
	if err := f.Close(); err != nil {
		_ = client.Remove(target)
		return codeStudioSFTPError(err, target)
	}
	_ = client.Chmod(target, mode) // best effort
	return nil
}

// codeStudioSFTPReplace writes a temporary sibling and publishes it with posix-rename
// (fallback: remove + rename); the old file's mode is kept.
func codeStudioSFTPReplace(client *sftp.Client, parent, target string, content []byte, mode os.FileMode, existed bool) error {
	temp := pathpkg.Join(parent, ".aurago-save-"+strings.ToLower(rand.Text()))
	f, err := client.OpenFile(temp, os.O_WRONLY|os.O_CREATE|os.O_EXCL)
	if err != nil {
		return codeStudioSFTPError(err, target)
	}
	published := false
	defer func() {
		if !published {
			_ = client.Remove(temp)
		}
	}()
	if _, err := f.Write(content); err != nil {
		_ = f.Close()
		return codeStudioSFTPError(err, target)
	}
	if err := f.Close(); err != nil {
		return codeStudioSFTPError(err, target)
	}
	_ = client.Chmod(temp, mode)
	if err := client.PosixRename(temp, target); err != nil {
		if existed {
			if err := client.Remove(target); err != nil && !codeStudioSFTPNotExist(err) {
				return codeStudioSFTPError(err, target)
			}
		}
		if err := client.Rename(temp, target); err != nil {
			return codeStudioSFTPError(err, target)
		}
	}
	published = true
	return nil
}

func (b *codeStudioSSHBackend) Move(ctx context.Context, from, to string) error {
	src, err := b.remote(from)
	if err != nil {
		return err
	}
	dst, err := b.remote(to)
	if err != nil {
		return err
	}
	if src == b.root {
		return codeStudioBadRequest("cannot change workspace root")
	}
	if dst == b.root {
		return codeStudioBadRequest("cannot replace workspace root")
	}
	return b.sftpDo(ctx, false, func(c *codeStudioSSHConn) error {
		if err := b.confine(c, pathpkg.Dir(src)); err != nil {
			return err
		}
		if err := c.sftp.MkdirAll(pathpkg.Dir(dst)); err != nil {
			return codeStudioSFTPError(err, to)
		}
		if err := b.confine(c, pathpkg.Dir(dst)); err != nil {
			return err
		}
		if err := c.sftp.PosixRename(src, dst); err != nil {
			if err := c.sftp.Rename(src, dst); err != nil {
				return codeStudioSFTPError(err, from)
			}
		}
		return nil
	})
}

func (b *codeStudioSSHBackend) Delete(ctx context.Context, vpath string) error {
	target, err := b.remote(vpath)
	if err != nil {
		return err
	}
	if target == b.root {
		return codeStudioBadRequest("cannot change workspace root")
	}
	return b.sftpDo(ctx, false, func(c *codeStudioSSHConn) error {
		if err := b.confine(c, pathpkg.Dir(target)); err != nil {
			return err
		}
		return codeStudioSFTPRemoveAll(c.sftp, target)
	})
}

// codeStudioSFTPRemoveAll never follows symlinks (Lstat / lstat-based ReadDir).
func codeStudioSFTPRemoveAll(client *sftp.Client, p string) error {
	info, err := client.Lstat(p)
	if err != nil {
		return codeStudioSFTPError(err, p)
	}
	if !info.IsDir() {
		return codeStudioSFTPError(client.Remove(p), p)
	}
	children, err := client.ReadDir(p)
	if err != nil {
		return codeStudioSFTPError(err, p)
	}
	for _, child := range children {
		if err := codeStudioSFTPRemoveAll(client, pathpkg.Join(p, child.Name())); err != nil {
			return err
		}
	}
	return codeStudioSFTPError(client.RemoveDirectory(p), p)
}

func (b *codeStudioSSHBackend) Mkdir(ctx context.Context, vpath string) error {
	target, err := b.remote(vpath)
	if err != nil {
		return err
	}
	return b.sftpDo(ctx, true, func(c *codeStudioSSHConn) error {
		if err := b.confine(c, target); err != nil {
			return err
		}
		return codeStudioSFTPError(c.sftp.MkdirAll(target), vpath)
	})
}
```

To keep the build green in this task, add temporary stubs for `Search`, `Exec`, `Git`, `OpenTerminal` and `StartProcess` that return `errCodeStudioUnsupported` (and zero values). Tasks 12 and 13 replace them.

**Test first:** `internal/server/code_studio_ssh_backend_test.go`.

```go
func TestCodeStudioSSHBackendFiles(t *testing.T) {
	f := startCodeStudioSSHFixture(t)
	b, _ := newCodeStudioTestSSHBackend(t, f, "/home/dev/proj")
	ctx := context.Background()
	must := func(err error) { t.Helper(); if err != nil { t.Fatal(err) } }
	must(b.Write(ctx, "/workspace/src/main.go", []byte("package main\n"), false))
	if err := b.Write(ctx, "/workspace/src/main.go", []byte("x"), true); !errors.Is(err, errCodeStudioExists) { t.Fatalf("createOnly: %v", err) }
	must(b.Write(ctx, "/workspace/src/main.go", []byte("package main // v2\n"), false))
	must(b.Write(ctx, "/workspace/README.md", []byte("# hi\n"), true))
	entries, err := b.List(ctx, "/workspace")
	must(err)
	if len(entries) != 2 || entries[0].Path != "/workspace/src" || entries[0].Type != "directory" || entries[1].Name != "README.md" { t.Fatalf("%+v", entries) }
	src, _ := b.List(ctx, "/workspace/src")
	for _, e := range src { if strings.HasPrefix(e.Name, ".aurago-save-") { t.Fatal("temp file left") } }
	probe, err := b.Probe(ctx, "/workspace/src/main.go")
	must(err)
	if probe.Path != "/workspace/src/main.go" || probe.Size != int64(len("package main // v2\n")) { t.Fatalf("%+v", probe) }
	data, err := b.Read(ctx, probe.Path, 1<<20)
	must(err)
	if string(data) != "package main // v2\n" { t.Fatalf("%q", data) }
	var se *codeStudioStatusError
	if _, err := b.Read(ctx, probe.Path, 3); !errors.As(err, &se) || se.Status != 413 { t.Fatalf("limit: %v", err) }
	var rej codeStudioProbeRejection
	if _, err := b.Probe(ctx, "/workspace/src"); !errors.As(err, &rej) { t.Fatalf("dir probe: %v", err) }
	must(b.Mkdir(ctx, "/workspace/empty/nested"))
	must(b.Move(ctx, "/workspace/src/main.go", "/workspace/cmd/app/main.go"))
	if _, err := b.Probe(ctx, "/workspace/cmd/app/main.go"); err != nil { t.Fatal(err) }
	must(b.Delete(ctx, "/workspace/cmd"))
	if err := b.Delete(ctx, "/workspace"); err == nil { t.Fatal("root delete allowed") }
	entries, _ = b.List(ctx, "/workspace")
	for _, e := range entries { if e.Name == "cmd" { t.Fatal("recursive delete left cmd") } }
}
```

**Run:** `go test ./internal/server/ -run '(?i)CodeStudioSSHBackendFiles' -count=1`. Expected: PASS on Windows, because everything runs in process.

**Commit:** `feat(code-studio): SFTP file operations for SSH workspaces`

### Task 12: SSH Exec, Git, Search and capabilities probe (10 min)

**Append to `code_studio_ssh_backend.go`** and remove the stubs from Task 11. Add `sync` to the imports.

```go
// codeStudioCappedBuffer keeps at most limit bytes and drains the rest (thread-safe:
// stdout and stderr may share it).
type codeStudioCappedBuffer struct {
	mu    sync.Mutex
	buf   []byte
	limit int
}

func newCodeStudioCappedBuffer(limit int) *codeStudioCappedBuffer { return &codeStudioCappedBuffer{limit: limit} }

func (c *codeStudioCappedBuffer) Write(p []byte) (int, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if room := c.limit - len(c.buf); room > 0 {
		c.buf = append(c.buf, p[:min(room, len(p))]...)
	}
	return len(p), nil
}

func (c *codeStudioCappedBuffer) String() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return string(c.buf)
}

// codeStudioRecordBuffer stops (closes Full) after maxRecords lines or maxBytes.
type codeStudioRecordBuffer struct {
	mu                  sync.Mutex
	buf                 []byte
	records             int
	maxRecords, maxBytes int
	full                chan struct{}
	once                sync.Once
}

func newCodeStudioRecordBuffer(maxRecords, maxBytes int) *codeStudioRecordBuffer {
	return &codeStudioRecordBuffer{maxRecords: maxRecords, maxBytes: maxBytes, full: make(chan struct{})}
}

func (r *codeStudioRecordBuffer) Write(p []byte) (int, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, ch := range p {
		if r.records >= r.maxRecords || len(r.buf) >= r.maxBytes {
			r.once.Do(func() { close(r.full) })
			break
		}
		r.buf = append(r.buf, ch)
		if ch == '\n' {
			r.records++
		}
	}
	return len(p), nil
}

func (r *codeStudioRecordBuffer) Full() bool {
	select {
	case <-r.full:
		return true
	default:
		return false
	}
}

// Complete returns the output up to the last full record.
func (r *codeStudioRecordBuffer) Complete() string {
	r.mu.Lock()
	defer r.mu.Unlock()
	if i := bytes.LastIndexByte(r.buf, '\n'); i >= 0 {
		return string(r.buf[:i+1])
	}
	return ""
}

func (b *codeStudioSSHBackend) run(ctx context.Context, script string, stdin []byte, stdout, stderr io.Writer, stop <-chan struct{}) (int, error) {
	code := -1
	err := b.pool.withConn(ctx, b.rec.DeviceID, codeStudioSSHRetrySessionOpen, func(c *codeStudioSSHConn) error {
		var err error
		code, err = c.run(ctx, codeStudioSSHCommandLine(script), stdin, stdout, stderr, stop)
		return err
	})
	return code, err
}

func (b *codeStudioSSHBackend) Exec(ctx context.Context, argv []string, vcwd string, stdin []byte, timeout time.Duration) (codeStudioExecResult, error) {
	if len(argv) == 0 || strings.TrimSpace(argv[0]) == "" {
		return codeStudioExecResult{}, codeStudioBadRequest("command is required")
	}
	if timeout <= 0 || timeout > codeStudioMaxExecTime {
		timeout = codeStudioMaxExecTime
	}
	cwd := b.root
	if vcwd != "" {
		p, err := b.remote(vcwd)
		if err != nil {
			return codeStudioExecResult{}, err
		}
		cwd = p
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	out := newCodeStudioCappedBuffer(codeStudioExecOutputLimit + 1)
	code, err := b.run(ctx, "cd "+shellQuote(cwd)+" && exec "+codeStudioShellJoin(argv), stdin, out, out, nil)
	if err != nil {
		if ctx.Err() != nil {
			return codeStudioExecResult{}, fmt.Errorf("code studio exec: %w", ctx.Err())
		}
		return codeStudioExecResult{}, err
	}
	return codeStudioExecResult{ExitCode: code, Output: out.String()}, nil
}

// Git pins English messages (codeStudioGitNotRepository / identity checks parse them).
func (b *codeStudioSSHBackend) Git(ctx context.Context, timeout time.Duration, args ...string) (codeStudioExecResult, error) {
	argv := append([]string{"env", "LC_ALL=C", "GIT_TERMINAL_PROMPT=0", "git", "-C", b.root, "--literal-pathspecs"}, args...)
	return b.Exec(ctx, argv, "", nil, timeout)
}

func (b *codeStudioSSHBackend) Search(ctx context.Context, opts codeStudioSearchOptions) ([]codeStudioSearchResult, bool, error) {
	if strings.TrimSpace(opts.Query) == "" {
		return nil, false, codeStudioBadRequest("search query is required")
	}
	dir, err := b.remote(opts.Path)
	if err != nil {
		return nil, false, err
	}
	ctx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()
	out := newCodeStudioRecordBuffer(codeStudioSearchResultLimit+1, 16<<20)
	errOut := newCodeStudioCappedBuffer(64 << 10)
	code, err := b.run(ctx, codeStudioShellJoin(codeStudioSSHGrepArgs(opts, dir)), nil, out, errOut, out.full)
	if err != nil && !out.Full() {
		return nil, false, err
	}
	text := out.Complete()
	if code == 127 {
		return nil, false, codeStudioBadRequest("grep is not installed on " + b.target.Host)
	}
	if code > 1 && text == "" && !out.Full() {
		msg := strings.TrimSpace(errOut.String())
		if msg == "" {
			msg = "search failed"
		}
		return nil, false, codeStudioBadRequest(msg)
	}
	results, truncated, err := parseCodeStudioGrepOutput(rewriteCodeStudioGrepPaths(b.root, text))
	if err != nil {
		return nil, false, err
	}
	return results, truncated || out.Full(), nil
}

func (b *codeStudioSSHBackend) probeCapabilities(ctx context.Context) (*codeStudioCapabilities, error) {
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	out := newCodeStudioCappedBuffer(64 << 10)
	code, err := b.run(ctx, codeStudioCapabilitiesScript, nil, out, io.Discard, nil)
	if err != nil {
		return nil, err
	}
	if code != 0 {
		return nil, fmt.Errorf("capability probe exited with %d", code)
	}
	caps := parseCodeStudioCapabilities(out.String(), time.Now().UTC())
	return &caps, nil
}
```

The capabilities script is run by `sh -c` with the user's environment, so `$SHELL` is inherited.

**Test (append):**

```go
func TestCodeStudioSSHBackendExecGitSearch(t *testing.T) {
	f := startCodeStudioSSHFixture(t)
	f.exec = func(cmd string, stdin []byte) (string, uint32) {
		s := unwrapShC(cmd)
		switch {
		case strings.Contains(s, "uname"):
			return "os=Linux\narch=aarch64\nshell=/bin/zsh\nhas=git\nhas=grep\nhas=clangd\n", 0
		case strings.Contains(s, "'grep'"):
			return "/home/dev/proj/src/main.go\x0012:func main() {\n/home/dev/proj/README.md\x003:main\n", 0
		case strings.Contains(s, "'gofmt'"):
			return strings.ToUpper(string(stdin)), 3
		}
		return "ok\n", 0
	}
	b, _ := newCodeStudioTestSSHBackend(t, f, "/home/dev/proj")
	ctx := context.Background()
	res, err := b.Exec(ctx, []string{"gofmt"}, "", []byte("package  main\n"), 5*time.Second)
	if err != nil || res.ExitCode != 3 || res.Output != "PACKAGE  MAIN\n" { t.Fatalf("%+v %v", res, err) }
	if got := unwrapShC(f.lastExec()); got != "cd '/home/dev/proj' && exec 'gofmt'" { t.Fatalf("%q", got) }
	if _, err := b.Exec(ctx, []string{"sh", "-c", "make"}, "/workspace/src", nil, time.Second); err != nil { t.Fatal(err) }
	if got := unwrapShC(f.lastExec()); !strings.HasPrefix(got, "cd '/home/dev/proj/src' && exec 'sh' '-c' 'make'") { t.Fatalf("%q", got) }
	if _, err := b.Git(ctx, time.Second, "status", "--porcelain=v1", "-z"); err != nil { t.Fatal(err) }
	if got := unwrapShC(f.lastExec()); !strings.Contains(got, "'git' '-C' '/home/dev/proj' '--literal-pathspecs' 'status'") || !strings.Contains(got, "'LC_ALL=C'") { t.Fatalf("%q", got) }
	results, truncated, err := b.Search(ctx, codeStudioSearchOptions{Query: "main", Path: "/workspace"})
	if err != nil || truncated || len(results) != 2 || results[0].Path != "/workspace/src/main.go" || results[0].Line != 12 || results[1].Path != "/workspace/README.md" { t.Fatalf("%+v %v %v", results, truncated, err) }
	if got := unwrapShC(f.lastExec()); !strings.Contains(got, "'--null'") || !strings.Contains(got, "'--' '/home/dev/proj'") { t.Fatalf("%q", got) }
	caps, err := b.probeCapabilities(ctx)
	if err != nil || caps.Arch != "aarch64" || !caps.Git || !caps.LSP.CPP || caps.LSP.Go { t.Fatalf("%+v %v", caps, err) }
}
```

**Run:** `go test ./internal/server/ -run '(?i)CodeStudioSSHBackend' -count=1`. Expected: PASS.

**Commit:** `feat(code-studio): exec, git, grep search and capability probe over SSH`

### Task 13: SSH terminal and stdio process (8 min)

**Append to `code_studio_ssh_backend.go`:**

```go
type codeStudioSSHTerminal struct {
	session *ssh.Session
	stdin   io.WriteCloser
	out     *io.PipeReader
	release func()
	once    sync.Once
}

func (t *codeStudioSSHTerminal) Read(p []byte) (int, error)  { return t.out.Read(p) }
func (t *codeStudioSSHTerminal) Write(p []byte) (int, error) { return t.stdin.Write(p) }
func (t *codeStudioSSHTerminal) Resize(_ context.Context, cols, rows int) error {
	return t.session.WindowChange(rows, cols)
}
func (t *codeStudioSSHTerminal) End(context.Context) error {
	_ = t.session.Signal(ssh.SIGHUP)
	return t.Close()
}
func (t *codeStudioSSHTerminal) Close() error {
	t.once.Do(func() { _ = t.stdin.Close(); _ = t.session.Close(); _ = t.out.Close(); t.release() })
	return nil
}

var (
	_ containerTerminalSession = (*codeStudioSSHTerminal)(nil)
	_ containerTerminalEnder   = (*codeStudioSSHTerminal)(nil)
)

// codeStudioSSHTerminalCommand: login shell in root; stays usable if root vanished.
func codeStudioSSHTerminalCommand(root string) string {
	return "exec sh -c " + shellQuote(`cd "$1" 2>/dev/null || printf 'aurago: cannot enter %s\r\n' "$1"; exec "${SHELL:-/bin/sh}" -l`) +
		" aurago-terminal " + shellQuote(root)
}

func (c *codeStudioSSHConn) acquireLong() (func(), error) {
	select {
	case c.longSem <- struct{}{}:
		return sync.OnceFunc(func() { <-c.longSem; c.touch() }), nil
	default:
		return nil, &codeStudioStatusError{Status: http.StatusTooManyRequests,
			Message: "too many terminals and language servers on this SSH connection"}
	}
}

func (b *codeStudioSSHBackend) OpenTerminal(ctx context.Context, cols, rows int) (containerTerminalSession, error) {
	var term *codeStudioSSHTerminal
	err := b.pool.withConn(ctx, b.rec.DeviceID, codeStudioSSHRetrySessionOpen, func(c *codeStudioSSHConn) error {
		release, err := c.acquireLong()
		if err != nil {
			return err
		}
		session, err := c.client.NewSession()
		if err != nil {
			release()
			return codeStudioSSHSessionOpenError{err}
		}
		fail := func(err error) error { _ = session.Close(); release(); return err }
		modes := ssh.TerminalModes{ssh.ECHO: 1, ssh.TTY_OP_ISPEED: 14400, ssh.TTY_OP_OSPEED: 14400}
		if err := session.RequestPty("xterm-256color", rows, cols, modes); err != nil {
			return fail(err)
		}
		stdin, err := session.StdinPipe()
		if err != nil {
			return fail(err)
		}
		pr, pw := io.Pipe()
		session.Stdout, session.Stderr = pw, pw
		if err := session.Start(codeStudioSSHTerminalCommand(b.root)); err != nil {
			return fail(err)
		}
		go func() { _ = session.Wait(); _ = pw.Close(); release() }()
		term = &codeStudioSSHTerminal{session: session, stdin: stdin, out: pr, release: release}
		return nil
	})
	if err != nil {
		return nil, err // never return a typed-nil *codeStudioSSHTerminal
	}
	return term, nil
}

type codeStudioSSHProcess struct {
	session        *ssh.Session
	stdin          io.WriteCloser
	stdout, stderr io.Reader
	release        func()
	done           chan struct{}
	waitErr        error
}

func (p *codeStudioSSHProcess) Read(b []byte) (int, error)  { return p.stdout.Read(b) }
func (p *codeStudioSSHProcess) Write(b []byte) (int, error) { return p.stdin.Write(b) }
func (p *codeStudioSSHProcess) CloseWrite() error           { return p.stdin.Close() }
func (p *codeStudioSSHProcess) Stderr() io.Reader           { return p.stderr }
func (p *codeStudioSSHProcess) Wait() error                 { <-p.done; return p.waitErr }
func (p *codeStudioSSHProcess) Close() error {
	_ = p.session.Signal(ssh.SIGKILL)
	_ = p.session.Close()
	p.release()
	return nil
}

func (b *codeStudioSSHBackend) StartProcess(ctx context.Context, argv []string, vcwd string) (codeStudioProcess, error) {
	if len(argv) == 0 {
		return nil, codeStudioBadRequest("command is required")
	}
	cwd := b.root
	if vcwd != "" {
		p, err := b.remote(vcwd)
		if err != nil {
			return nil, err
		}
		cwd = p
	}
	var proc *codeStudioSSHProcess
	err := b.pool.withConn(ctx, b.rec.DeviceID, codeStudioSSHRetrySessionOpen, func(c *codeStudioSSHConn) error {
		release, err := c.acquireLong()
		if err != nil {
			return err
		}
		session, err := c.client.NewSession()
		if err != nil {
			release()
			return codeStudioSSHSessionOpenError{err}
		}
		fail := func(err error) error { _ = session.Close(); release(); return err }
		stdin, err := session.StdinPipe()
		if err != nil {
			return fail(err)
		}
		stdout, err := session.StdoutPipe()
		if err != nil {
			return fail(err)
		}
		stderr, err := session.StderrPipe()
		if err != nil {
			return fail(err)
		}
		if err := session.Start(codeStudioSSHCommandLine("cd " + shellQuote(cwd) + " && exec " + codeStudioShellJoin(argv))); err != nil {
			return fail(err)
		}
		proc = &codeStudioSSHProcess{session: session, stdin: stdin, stdout: stdout, stderr: stderr, release: release, done: make(chan struct{})}
		go func() { proc.waitErr = session.Wait(); release(); close(proc.done) }()
		return nil
	})
	if err != nil {
		return nil, err
	}
	return proc, nil
}
```

**Test (append):**

```go
func TestCodeStudioSSHBackendTerminalAndProcess(t *testing.T) {
	f := startCodeStudioSSHFixture(t)
	b, _ := newCodeStudioTestSSHBackend(t, f, "/home/dev/proj")
	ctx := context.Background()
	readUntil := func(r io.Reader, want string) {
		t.Helper()
		got := make(chan string, 1)
		go func() { buf := make([]byte, len(want)); _, _ = io.ReadFull(r, buf); got <- string(buf) }()
		select {
		case s := <-got: if s != want { t.Fatalf("read %q want %q", s, want) }
		case <-time.After(5 * time.Second): t.Fatal("read timeout")
		}
	}
	term, err := b.OpenTerminal(ctx, 100, 40)
	if err != nil { t.Fatal(err) }
	if _, err := term.Write([]byte("hello")); err != nil { t.Fatal(err) }
	readUntil(term, "hello")
	if err := term.Resize(ctx, 120, 50); err != nil { t.Fatal(err) }
	deadline := time.Now().Add(3 * time.Second)
	for {
		f.mu.Lock(); w := append([][2]int(nil), f.windows...); f.mu.Unlock()
		if len(w) >= 2 && w[0] == [2]int{100, 40} && w[len(w)-1] == [2]int{120, 50} { break }
		if time.Now().After(deadline) { t.Fatalf("windows %v", w) }
		time.Sleep(20 * time.Millisecond)
	}
	if !strings.Contains(f.lastExec(), "aurago-terminal") { t.Fatal(f.lastExec()) }
	_ = term.Close()
	proc, err := b.StartProcess(ctx, []string{"aurago-echo"}, "")
	if err != nil { t.Fatal(err) }
	go func() { _, _ = io.Copy(io.Discard, proc.Stderr()) }()
	_, _ = proc.Write([]byte("ping\n"))
	readUntil(proc, "ping\n")
	_ = proc.CloseWrite()
	if err := proc.Wait(); err != nil { t.Fatal(err) }
}
```

**Run:** `go test ./internal/server/ -run '(?i)CodeStudioSSHBackend' -count=1`. Expected: PASS.

**Commit:** `feat(code-studio): SSH PTY terminal and stdio processes for SSH workspaces`

### Task 14: `backendFor` resolves saved SSH workspaces (6 min)

**Replace the stub** `remoteBackend` in `code_studio_backend.go` (it moves into `code_studio_ssh_pool.go`; delete it from `code_studio_backend.go`):

```go
func (h *codeStudioHandlers) remoteBackend(r *http.Request, id string) (codeStudioBackend, error) {
	if !codeStudioRemoteCallerAllowed(h.server, r) {
		return nil, codeStudioRemoteScopeError()
	}
	store, err := h.workspaceStore(r.Context())
	if err != nil {
		return nil, codeStudioUnavailableError{err}
	}
	rec, err := store.Get(r.Context(), id)
	if errors.Is(err, errCodeStudioWorkspaceRecordNotFound) {
		return nil, codeStudioWorkspaceNotFound()
	}
	if err != nil {
		return nil, err
	}
	remote := h.remoteState()
	conn, err := remote.pool.get(r.Context(), rec.DeviceID, "")
	if err != nil {
		return nil, err // 409 host key, 502 unreachable/auth, 404 device
	}
	if remote.shouldTouch(rec.ID, time.Now()) {
		_ = store.Touch(r.Context(), rec.ID, nil)
	}
	return newCodeStudioSSHBackend(rec, remote.pool, conn.target), nil
}
```

**Test (append to `code_studio_ssh_backend_test.go`):**

```go
func TestCodeStudioBackendForSSHWorkspaceServesHandlers(t *testing.T) {
	f := startCodeStudioSSHFixture(t)
	s := testCodeStudioServerWithFakeCodeContainer(t, 1)
	_, remote := newCodeStudioTestSSHBackend(t, f, "/home/dev/proj")
	h := codeStudioHandlers{server: s, docker: &recordingCodeStudioDockerAPI{}, remote: remote}
	store, _ := h.workspaceStore(context.Background())
	rec, _ := store.Upsert(context.Background(), "dev1", "/home/dev/proj", "", nil)
	if err := f.adminSFTP(t).MkdirAll("/home/dev/proj/pkg"); err != nil { t.Fatal(err) }
	req := httptest.NewRequest(http.MethodGet, "/api/code-studio/files?path=/workspace", nil)
	req.Header.Set(codeStudioWorkspaceHeader, rec.ID)
	w := httptest.NewRecorder()
	h.handleFiles(w, req)
	if w.Code != 200 || !strings.Contains(w.Body.String(), `"/workspace/pkg"`) { t.Fatal(w.Code, w.Body.String()) }
	w = httptest.NewRecorder() // query form, write + read back
	h.handleFile(w, httptest.NewRequest(http.MethodPut, "/api/code-studio/file?ws="+rec.ID, strings.NewReader(`{"path":"/workspace/a.txt","content":"hi"}`)))
	if w.Code != 200 { t.Fatal(w.Code, w.Body.String()) }
	w = httptest.NewRecorder()
	h.handleFile(w, httptest.NewRequest(http.MethodGet, "/api/code-studio/file?ws="+rec.ID+"&path=/workspace/a.txt", nil))
	if w.Code != 200 || !strings.Contains(w.Body.String(), `"content":"hi"`) { t.Fatal(w.Body.String()) }
	w = httptest.NewRecorder()
	h.handleLSP(w, httptest.NewRequest(http.MethodGet, "/api/code-studio/lsp?lang=go&ws="+rec.ID, nil))
	if w.Code == 200 || w.Header().Get("Upgrade") != "" { t.Fatal("LSP over SSH must not upgrade in Plan A") }
}
```

The LSP assertion is loose on purpose: the 501 only appears after the auth and origin gates, so the test asserts only that the request does not upgrade.

**Run:** `go test ./internal/server/ -run '(?i)CodeStudio' -count=1`. Expected: PASS, the whole suite.

**Commit:** `feat(code-studio): route requests to saved SSH workspaces`

---

## Group 5: Endpoints

### Task 15: Workspace endpoints, trust endpoint and the host-key 409 contract (10 min)

**File:** `internal/server/code_studio_workspace_handlers.go`.

```go
package server

import (
	"errors"
	"net/http"
	"strings"
	"time"
	"unicode"

	"aurago/internal/desktop"
	"aurago/internal/inventory"
)

func registerCodeStudioWorkspaceRoutes(mux *http.ServeMux, h codeStudioHandlers) {
	mux.HandleFunc("/api/code-studio/workspaces", h.handleWorkspaces)
	mux.HandleFunc("/api/code-studio/workspaces/", h.handleWorkspaceItem)
	mux.HandleFunc("/api/code-studio/remote/devices", h.handleRemoteDevices)
	mux.HandleFunc("/api/code-studio/remote/browse", h.handleRemoteBrowse)
	mux.HandleFunc("/api/code-studio/remote/trust", h.handleRemoteTrust)
}

func (h codeStudioHandlers) auditCodeStudioRemote(r *http.Request, action, deviceID string, details map[string]interface{}) {
	if h.server == nil || h.server.DesktopService == nil {
		return
	}
	behindProxy := h.server.Cfg != nil && h.server.Cfg.Server.HTTPS.BehindProxy
	_ = h.server.DesktopService.AuditWithRequest(r.Context(), action, deviceID, details, desktop.SourceUser,
		desktop.AuditRequestInfo{ClientIP: ClientIP(r, behindProxy), UserAgent: r.UserAgent()})
}

func codeStudioCleanLabel(raw string) (string, error) {
	label := strings.TrimSpace(raw)
	if len([]rune(label)) > 120 || strings.IndexFunc(label, unicode.IsControl) >= 0 {
		return "", errors.New("invalid label")
	}
	return label, nil
}

func (h codeStudioHandlers) handleWorkspaces(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		if h.requirePermission(w, r, desktopScopeRead) {
			h.listWorkspaces(w, r)
		}
	case http.MethodPost:
		if h.requirePermission(w, r, desktopScopeWrite) {
			h.createWorkspace(w, r)
		}
	default:
		jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (h codeStudioHandlers) handleWorkspaceItem(w http.ResponseWriter, r *http.Request) {
	rest := strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/code-studio/workspaces/"), "/")
	id, action, _ := strings.Cut(rest, "/")
	if id == codeStudioLocalWorkspaceID {
		jsonError(w, "the local workspace cannot be changed", http.StatusBadRequest)
		return
	}
	if !codeStudioWorkspaceIDPattern.MatchString(id) {
		writeCodeStudioOpError(w, codeStudioWorkspaceNotFound())
		return
	}
	switch {
	case action == "" && r.Method == http.MethodDelete:
		if h.requirePermission(w, r, desktopScopeWrite) {
			h.deleteWorkspace(w, r, id)
		}
	case action == "connect" && r.Method == http.MethodPost:
		if h.requirePermission(w, r, desktopScopeWrite) {
			h.connectWorkspace(w, r, id)
		}
	default:
		jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (h codeStudioHandlers) workspaceTarget(deviceID string) codeStudioSSHTarget {
	if target, ok := h.remoteState().pool.connected(deviceID); ok {
		return target
	}
	if h.server != nil && h.server.InventoryDB != nil {
		if device, err := inventory.GetDeviceByID(h.server.InventoryDB, deviceID); err == nil {
			a := desktop.DescribeSSHDevice(h.server.InventoryDB, device)
			return codeStudioSSHTarget{DeviceID: deviceID, Name: a.Name, Host: a.Host, Port: a.Port, User: a.Username}
		}
	}
	return codeStudioSSHTarget{DeviceID: deviceID}
}

func (h codeStudioHandlers) listWorkspaces(w http.ResponseWriter, r *http.Request) {
	out := []codeStudioWorkspaceInfo{codeStudioLocalWorkspaceInfo()}
	store, err := h.workspaceStore(r.Context())
	if err == nil {
		var recs []codeStudioWorkspaceRecord
		if recs, err = store.List(r.Context()); err == nil {
			for _, rec := range recs {
				_, connected := h.remoteState().pool.connected(rec.DeviceID)
				out = append(out, codeStudioSSHWorkspaceInfo(rec, h.workspaceTarget(rec.DeviceID), connected))
			}
		}
	}
	if err != nil {
		h.codeStudioLogger().Warn("code studio workspace list incomplete", "error", err)
	}
	writeJSON(w, map[string]interface{}{"status": "ok", "workspaces": out})
}

// openRemoteRoot connects (409 contract), normalises root (~ allowed) and checks it is a directory.
func (h codeStudioHandlers) openRemoteRoot(r *http.Request, deviceID, rawRoot, trust string) (*codeStudioSSHConn, string, error) {
	conn, err := h.remoteState().pool.get(r.Context(), deviceID, strings.TrimSpace(trust))
	if err != nil {
		return nil, "", err
	}
	home := ""
	if strings.HasPrefix(strings.TrimSpace(rawRoot), "~") {
		home, _ = conn.sftp.Getwd()
	}
	root, err := normalizeCodeStudioRemoteRoot(rawRoot, home)
	if err != nil {
		return nil, "", &codeStudioRemoteError{Status: http.StatusBadRequest, Code: "invalid_root", Message: err.Error()}
	}
	if info, err := conn.sftp.Stat(root); err != nil || !info.IsDir() {
		if err != nil && codeStudioSSHBroken(err) {
			return nil, "", codeStudioSFTPError(err, root)
		}
		return nil, "", &codeStudioRemoteError{Status: http.StatusBadRequest, Code: "not_a_directory",
			Message: root + " is not a directory on " + conn.target.Host}
	}
	return conn, root, nil
}

func (h codeStudioHandlers) createWorkspace(w http.ResponseWriter, r *http.Request) {
	if !codeStudioRemoteCallerAllowed(h.server, r) {
		writeCodeStudioOpError(w, codeStudioRemoteScopeError())
		return
	}
	var body struct {
		DeviceID     string `json:"device_id"`
		Root         string `json:"root"`
		Label        string `json:"label"`
		TrustHostKey string `json:"trust_host_key"`
	}
	if err := decodeJSONRequest(w, r, &body, h.server); err != nil {
		jsonError(w, "Invalid JSON", jsonDecodeStatus(err))
		return
	}
	deviceID := strings.TrimSpace(body.DeviceID)
	if deviceID == "" {
		jsonError(w, "device_id is required", http.StatusBadRequest)
		return
	}
	label, err := codeStudioCleanLabel(body.Label)
	if err != nil {
		jsonError(w, err.Error(), http.StatusBadRequest)
		return
	}
	store, err := h.workspaceStore(r.Context())
	if err != nil {
		jsonError(w, err.Error(), http.StatusServiceUnavailable)
		return
	}
	conn, root, err := h.openRemoteRoot(r, deviceID, body.Root, body.TrustHostKey)
	if err != nil {
		writeCodeStudioOpError(w, err)
		return
	}
	probe := newCodeStudioSSHBackend(codeStudioWorkspaceRecord{DeviceID: deviceID, Root: root}, h.remoteState().pool, conn.target)
	caps, capErr := probe.probeCapabilities(r.Context())
	if capErr != nil {
		h.codeStudioLogger().Warn("code studio capability probe failed", "device_id", deviceID, "error", capErr)
	}
	rec, err := store.Upsert(r.Context(), deviceID, root, label, caps)
	if err != nil {
		jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	h.auditCodeStudioRemote(r, "code_studio_workspace_open", deviceID, map[string]interface{}{"root": root, "workspace_id": rec.ID})
	writeJSON(w, map[string]interface{}{"status": "ok", "workspace": codeStudioSSHWorkspaceInfo(rec, conn.target, true)})
}

func (h codeStudioHandlers) connectWorkspace(w http.ResponseWriter, r *http.Request, id string) {
	if !codeStudioRemoteCallerAllowed(h.server, r) {
		writeCodeStudioOpError(w, codeStudioRemoteScopeError())
		return
	}
	var body struct {
		TrustHostKey string `json:"trust_host_key"`
	}
	if r.ContentLength > 0 {
		if err := decodeJSONRequest(w, r, &body, h.server); err != nil {
			jsonError(w, "Invalid JSON", jsonDecodeStatus(err))
			return
		}
	}
	store, err := h.workspaceStore(r.Context())
	if err != nil {
		jsonError(w, err.Error(), http.StatusServiceUnavailable)
		return
	}
	rec, err := store.Get(r.Context(), id)
	if err != nil {
		writeCodeStudioOpError(w, codeStudioWorkspaceNotFound())
		return
	}
	conn, _, err := h.openRemoteRoot(r, rec.DeviceID, rec.Root, body.TrustHostKey)
	if err != nil {
		writeCodeStudioOpError(w, err)
		return
	}
	if caps, err := newCodeStudioSSHBackend(rec, h.remoteState().pool, conn.target).probeCapabilities(r.Context()); err == nil {
		rec.Capabilities = caps
		_ = store.Touch(r.Context(), rec.ID, caps)
	}
	rec.LastUsedAt = time.Now().UTC()
	writeJSON(w, map[string]interface{}{"status": "ok", "workspace": codeStudioSSHWorkspaceInfo(rec, conn.target, true)})
}

func (h codeStudioHandlers) deleteWorkspace(w http.ResponseWriter, r *http.Request, id string) {
	store, err := h.workspaceStore(r.Context())
	if err != nil {
		jsonError(w, err.Error(), http.StatusServiceUnavailable)
		return
	}
	rec, err := store.Get(r.Context(), id)
	if err == nil {
		err = store.Delete(r.Context(), id)
	}
	if err != nil {
		writeCodeStudioOpError(w, codeStudioWorkspaceNotFound())
		return
	}
	if n, err := store.CountDevice(r.Context(), rec.DeviceID); err == nil && n == 0 {
		h.remoteState().pool.release(rec.DeviceID)
	}
	writeJSON(w, map[string]interface{}{"status": "ok", "id": id})
}

func (h codeStudioHandlers) handleRemoteTrust(w http.ResponseWriter, r *http.Request) {
	if !h.requirePermission(w, r, desktopScopeWrite) {
		return
	}
	if r.Method != http.MethodPost {
		jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if !codeStudioRemoteCallerAllowed(h.server, r) {
		writeCodeStudioOpError(w, codeStudioRemoteScopeError())
		return
	}
	var body struct {
		DeviceID    string `json:"device_id"`
		Fingerprint string `json:"fingerprint"`
	}
	if err := decodeJSONRequest(w, r, &body, h.server); err != nil {
		jsonError(w, "Invalid JSON", jsonDecodeStatus(err))
		return
	}
	if strings.TrimSpace(body.DeviceID) == "" || !strings.HasPrefix(strings.TrimSpace(body.Fingerprint), "SHA256:") {
		jsonError(w, "device_id and a SHA256 fingerprint are required", http.StatusBadRequest)
		return
	}
	if _, err := h.remoteState().pool.get(r.Context(), strings.TrimSpace(body.DeviceID), strings.TrimSpace(body.Fingerprint)); err != nil {
		writeCodeStudioOpError(w, err)
		return
	}
	writeJSON(w, map[string]interface{}{"status": "ok"})
}
```

For this task, add stubs for `handleRemoteDevices` and `handleRemoteBrowse` that answer 501; Tasks 16 and 17 replace them.

**Edit `registerCodeStudioRoutes`:** append `registerCodeStudioWorkspaceRoutes(mux, handlers)` at the end.

**Test (append):** this test uses the real default dialer with inventory, vault and known_hosts in a temp HOME.

```go
func codeStudioRemoteTestServer(t *testing.T, f *codeStudioSSHFixture) (*Server, string) {
	t.Helper()
	prev := remote.InsecureHostKey
	remote.InsecureHostKey = false
	t.Cleanup(func() { remote.InsecureHostKey = prev })
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	s := testCodeStudioServerWithFakeCodeContainer(t, 1)
	db, err := inventory.InitDB(filepath.Join(t.TempDir(), "inventory.db"))
	if err != nil { t.Fatal(err) }
	t.Cleanup(func() { _ = db.Close() })
	if err := credentials.EnsureSchema(db); err != nil { t.Fatal(err) }
	vault, err := security.NewVault("0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef", filepath.Join(t.TempDir(), "vault.bin"))
	if err != nil { t.Fatal(err) }
	_ = vault.WriteSecret("cs/dev", f.password)
	if err := inventory.AddDevice(db, inventory.DeviceRecord{ID: "dev1", Name: "Fixture", Type: "server", Protocol: inventory.ProtocolSSH,
		IPAddress: f.host, Port: f.port, Username: f.user, VaultSecretID: "cs/dev"}); err != nil { t.Fatal(err) }
	s.InventoryDB, s.Vault = db, vault
	return s, home
}

func postJSON(t *testing.T, handler http.HandlerFunc, path, body string) (*httptest.ResponseRecorder, map[string]any) {
	t.Helper()
	w := httptest.NewRecorder()
	handler(w, httptest.NewRequest(http.MethodPost, path, strings.NewReader(body)))
	var m map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &m)
	return w, m
}

func TestCodeStudioWorkspaceHostKeyContractAndLifecycle(t *testing.T) {
	f := startCodeStudioSSHFixture(t)
	if err := f.adminSFTP(t).MkdirAll("/home/dev/proj"); err != nil { t.Fatal(err) }
	s, home := codeStudioRemoteTestServer(t, f)
	newH := func() codeStudioHandlers {
		r := newCodeStudioRemote(codeStudioDefaultSSHDialer(s), slog.Default())
		t.Cleanup(r.pool.closeAll)
		return codeStudioHandlers{server: s, docker: &recordingCodeStudioDockerAPI{}, remote: r}
	}
	h := newH()
	fp := ssh.FingerprintSHA256(f.hostKey)
	w, m := postJSON(t, h.handleWorkspaces, "/api/code-studio/workspaces", `{"device_id":"dev1","root":"/home/dev/proj"}`)
	if w.Code != 409 || m["code"] != "unknown_host_key" || m["fingerprint"] != fp || m["key_type"] != "ssh-ed25519" { t.Fatal(w.Code, w.Body.String()) }
	w, m = postJSON(t, h.handleWorkspaces, "/api/code-studio/workspaces", `{"device_id":"dev1","root":"/home/dev/proj","trust_host_key":"SHA256:wrong"}`)
	if w.Code != 409 || m["code"] != "unknown_host_key" { t.Fatal(w.Body.String()) }
	w, m = postJSON(t, h.handleWorkspaces, "/api/code-studio/workspaces", `{"device_id":"dev1","root":"/home/dev/proj","label":"Proj","trust_host_key":"`+fp+`"}`)
	if w.Code != 200 { t.Fatal(w.Code, w.Body.String()) }
	ws := m["workspace"].(map[string]any)
	id := ws["id"].(string)
	if ws["kind"] != "ssh" || ws["host"] != f.host || ws["user"] != f.user || ws["capabilities"].(map[string]any)["git"] != true { t.Fatalf("%v", ws) }
	kh, _ := os.ReadFile(filepath.Join(home, ".ssh", "known_hosts"))
	if !strings.Contains(string(kh), f.hostKey.Type()) { t.Fatal("host key not stored") }
	_, m = postJSON(t, h.handleWorkspaces, "/api/code-studio/workspaces", `{"device_id":"dev1","root":"/home/dev/proj/"}`)
	if m["workspace"].(map[string]any)["id"] != id { t.Fatal("upsert created a second entry") }
	w, m = postJSON(t, h.handleWorkspaces, "/api/code-studio/workspaces", `{"device_id":"dev1","root":"/nope"}`)
	if w.Code != 400 || m["code"] != "not_a_directory" { t.Fatal(w.Body.String()) }
	w, _ = postJSON(t, h.handleWorkspaceItem, "/api/code-studio/workspaces/"+id+"/connect", ``)
	if w.Code != 200 { t.Fatal(w.Body.String()) }
	lw := httptest.NewRecorder()
	h.handleWorkspaces(lw, httptest.NewRequest(http.MethodGet, "/api/code-studio/workspaces", nil))
	if lw.Code != 200 || strings.Count(lw.Body.String(), `"kind"`) != 2 || strings.Contains(lw.Body.String(), "cs/dev") || strings.Contains(lw.Body.String(), f.password) { t.Fatal(lw.Body.String()) }
	dw := httptest.NewRecorder()
	h.handleWorkspaceItem(dw, httptest.NewRequest(http.MethodDelete, "/api/code-studio/workspaces/"+id, nil))
	if dw.Code != 200 { t.Fatal(dw.Body.String()) }
	dw = httptest.NewRecorder()
	h.handleWorkspaceItem(dw, httptest.NewRequest(http.MethodDelete, "/api/code-studio/workspaces/"+id, nil))
	if dw.Code != 404 { t.Fatal(dw.Code) }
	// changed host key: no override, not even with trust_host_key
	_, other, _ := ed25519.GenerateKey(rand.Reader)
	otherSigner, _ := ssh.NewSignerFromKey(other)
	_ = os.WriteFile(filepath.Join(home, ".ssh", "known_hosts"), []byte(knownhosts.Line([]string{f.addr}, otherSigner.PublicKey())+"\n"), 0o600)
	w, m = postJSON(t, newH().handleWorkspaces, "/api/code-studio/workspaces", `{"device_id":"dev1","root":"/home/dev/proj","trust_host_key":"`+fp+`"}`)
	if w.Code != 409 || m["code"] != "host_key_mismatch" { t.Fatal(w.Body.String()) }
	w, _ = postJSON(t, newH().handleRemoteTrust, "/api/code-studio/remote/trust", `{"device_id":"dev1","fingerprint":"`+fp+`"}`)
	if w.Code != 409 { t.Fatal("trust endpoint overrode a mismatch") }
}
```

**Run:** `go test ./internal/server/ -run '(?i)CodeStudioWorkspaceHostKey' -count=1`. Expected: PASS.

**Commit:** `feat(code-studio): workspace API with host-key trust contract`

### Task 16: Remote devices list and create (8 min)

**Append to `code_studio_workspace_handlers.go`** and replace the stub from Task 15. Imports to add: `crypto`, `encoding/pem`, `net`, `sort`, `strconv`, `aurago/internal/credentials`, `aurago/internal/security`, `aurago/internal/uid`, `golang.org/x/crypto/ssh`.

```go
type codeStudioRemoteDevice struct {
	ID          string     `json:"id"`
	Name        string     `json:"name"`
	Host        string     `json:"host"`
	Port        int        `json:"port"`
	Username    string     `json:"username"`
	Description string     `json:"description"`
	Tags        []string   `json:"tags"`
	HasSecret   bool       `json:"has_secret"`
	LastUsedAt  *time.Time `json:"last_used_at,omitempty"`
}

func (h codeStudioHandlers) handleRemoteDevices(w http.ResponseWriter, r *http.Request) {
	scope := desktopScopeRead
	if r.Method == http.MethodPost {
		scope = desktopScopeWrite
	}
	if !h.requirePermission(w, r, scope) {
		return
	}
	if !codeStudioRemoteCallerAllowed(h.server, r) {
		writeCodeStudioOpError(w, codeStudioRemoteScopeError())
		return
	}
	if h.server == nil || h.server.InventoryDB == nil {
		jsonError(w, "inventory database not configured", http.StatusServiceUnavailable)
		return
	}
	switch r.Method {
	case http.MethodGet:
		h.listRemoteDevices(w, r)
	case http.MethodPost:
		h.createRemoteDevice(w, r)
	default:
		jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (h codeStudioHandlers) remoteDeviceSummary(d inventory.DeviceRecord, lastUsed map[string]time.Time) codeStudioRemoteDevice {
	a := desktop.DescribeSSHDevice(h.server.InventoryDB, d)
	out := codeStudioRemoteDevice{ID: d.ID, Name: d.Name, Host: a.Host, Port: a.Port, Username: a.Username, Description: d.Description,
		Tags: append([]string{}, d.Tags...), HasSecret: strings.TrimSpace(d.VaultSecretID) != "" || strings.TrimSpace(d.CredentialID) != ""}
	if t, ok := lastUsed[d.ID]; ok {
		out.LastUsedAt = &t
	}
	return out
}

func (h codeStudioHandlers) listRemoteDevices(w http.ResponseWriter, r *http.Request) {
	devices, err := inventory.ListAllDevices(h.server.InventoryDB)
	if err != nil {
		jsonError(w, "Failed to list devices", http.StatusInternalServerError)
		return
	}
	lastUsed := map[string]time.Time{}
	if store, err := h.workspaceStore(r.Context()); err == nil {
		if recs, err := store.List(r.Context()); err == nil {
			for _, rec := range recs {
				if rec.LastUsedAt.After(lastUsed[rec.DeviceID]) {
					lastUsed[rec.DeviceID] = rec.LastUsedAt
				}
			}
		}
	}
	out := []codeStudioRemoteDevice{}
	for _, d := range devices {
		if p := strings.ToLower(strings.TrimSpace(d.Protocol)); p == "" || p == inventory.ProtocolSSH {
			out = append(out, h.remoteDeviceSummary(d, lastUsed))
		}
	}
	sort.Slice(out, func(i, j int) bool { return strings.ToLower(out[i].Name) < strings.ToLower(out[j].Name) })
	writeJSON(w, map[string]interface{}{"status": "ok", "devices": out})
}

func codeStudioValidHost(host string) bool {
	if host == "" || len(host) > 253 {
		return false
	}
	for _, c := range host {
		if !(c == '.' || c == '-' || c == ':' || c == '_' || c == '%' || c == '[' || c == ']' ||
			(c >= '0' && c <= '9') || (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z')) {
			return false
		}
	}
	return true
}

// codeStudioPrepareSSHKey validates a private key; a passphrase-protected key is decrypted
// and stored unencrypted in the (encrypted) vault, because the SSH dial path has no
// passphrase support.
func codeStudioPrepareSSHKey(secret, passphrase string) (string, error) {
	if passphrase == "" {
		if _, err := ssh.ParsePrivateKey([]byte(secret)); err != nil {
			var missing *ssh.PassphraseMissingError
			if errors.As(err, &missing) {
				return "", &codeStudioRemoteError{Status: http.StatusBadRequest, Code: "passphrase_required", Message: "the private key needs its passphrase"}
			}
			return "", &codeStudioRemoteError{Status: http.StatusBadRequest, Code: "invalid_private_key", Message: "the private key could not be read"}
		}
		return secret, nil
	}
	raw, err := ssh.ParseRawPrivateKeyWithPassphrase([]byte(secret), []byte(passphrase))
	if err != nil {
		return "", &codeStudioRemoteError{Status: http.StatusBadRequest, Code: "invalid_private_key", Message: "wrong passphrase or unreadable private key"}
	}
	block, err := ssh.MarshalPrivateKey(crypto.PrivateKey(raw), "")
	if err != nil {
		return "", &codeStudioRemoteError{Status: http.StatusBadRequest, Code: "invalid_private_key", Message: "the private key type is not supported"}
	}
	return string(pem.EncodeToMemory(block)), nil
}

func (h codeStudioHandlers) createRemoteDevice(w http.ResponseWriter, r *http.Request) {
	if h.server.Vault == nil {
		jsonError(w, "vault not configured", http.StatusServiceUnavailable)
		return
	}
	var body struct {
		Name       string `json:"name"`
		Host       string `json:"host"`
		Port       int    `json:"port"`
		Username   string `json:"username"`
		Auth       string `json:"auth"`
		Secret     string `json:"secret"`
		Passphrase string `json:"passphrase"`
	}
	if err := decodeJSONRequest(w, r, &body, h.server); err != nil {
		jsonError(w, "Invalid JSON", jsonDecodeStatus(err))
		return
	}
	name, host, user := strings.TrimSpace(body.Name), strings.TrimSpace(body.Host), strings.TrimSpace(body.Username)
	if body.Port == 0 {
		body.Port = 22
	}
	switch {
	case name == "" || len([]rune(name)) > 100 || strings.IndexFunc(name, unicode.IsControl) >= 0:
		jsonError(w, "name is required (max 100 characters)", http.StatusBadRequest)
		return
	case !codeStudioValidHost(host):
		jsonError(w, "host is invalid", http.StatusBadRequest)
		return
	case body.Port < 1 || body.Port > 65535:
		jsonError(w, "port must be 1-65535", http.StatusBadRequest)
		return
	case user == "" || strings.IndexFunc(user, unicode.IsSpace) >= 0:
		jsonError(w, "username is required", http.StatusBadRequest)
		return
	case body.Secret == "":
		jsonError(w, "secret is required", http.StatusBadRequest)
		return
	}
	rec := credentials.Record{Name: name + " (SSH)", Type: "ssh", Host: host, Username: user, Description: "Created by Code Studio"}
	secret := body.Secret
	switch body.Auth {
	case "password":
		rec.PasswordVaultID = "credential_password_" + uid.NewString()
	case "key":
		prepared, err := codeStudioPrepareSSHKey(strings.TrimSpace(body.Secret), body.Passphrase)
		if err != nil {
			writeCodeStudioOpError(w, err)
			return
		}
		secret = prepared
		rec.CertificateVaultID = "credential_certificate_" + uid.NewString()
	default:
		jsonError(w, `auth must be "password" or "key"`, http.StatusBadRequest)
		return
	}
	vaultID := rec.PasswordVaultID + rec.CertificateVaultID
	security.RegisterSensitive(secret)
	if err := h.server.Vault.WriteSecret(vaultID, secret); err != nil {
		jsonError(w, "failed to store the secret in the vault", http.StatusInternalServerError)
		return
	}
	credID, err := credentials.Create(h.server.InventoryDB, rec)
	if err != nil {
		_ = h.server.Vault.DeleteSecret(vaultID)
		jsonError(w, "Failed to create credential", http.StatusInternalServerError)
		return
	}
	id, err := inventory.CreateDevice(h.server.InventoryDB, name, "server", inventory.ProtocolSSH, host, body.Port, "", "", credID,
		"Created by Code Studio", []string{"code-studio"}, "")
	if err != nil {
		_ = credentials.Delete(h.server.InventoryDB, credID)
		_ = h.server.Vault.DeleteSecret(vaultID)
		status := http.StatusInternalServerError
		if strings.Contains(strings.ToUpper(err.Error()), "UNIQUE") {
			status = http.StatusConflict
		}
		jsonError(w, "Failed to create device", status)
		return
	}
	device, _ := inventory.GetDeviceByID(h.server.InventoryDB, id)
	h.auditCodeStudioRemote(r, "code_studio_device_create", id, map[string]interface{}{"host": net.JoinHostPort(host, strconv.Itoa(body.Port))})
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(map[string]interface{}{"status": "ok", "device": h.remoteDeviceSummary(device, nil)})
}
```

This mirrors `handleCreateCredential` followed by `handleCreateDevice` with `CredentialID`, without editing either file. Also import `encoding/json`. If `Vault.DeleteSecret` has a different signature, adapt (it is used the same way in `credential_handlers.go`).

**Test (append):**

```go
func TestCodeStudioRemoteDevicesCreateAndList(t *testing.T) {
	f := startCodeStudioSSHFixture(t)
	s, _ := codeStudioRemoteTestServer(t, f)
	h := codeStudioHandlers{server: s, docker: &recordingCodeStudioDockerAPI{}}
	w, m := postJSON(t, h.handleRemoteDevices, "/api/code-studio/remote/devices", `{"name":"Lab box","host":"10.0.0.5","port":2222,"username":"ana","auth":"password","secret":"s3cret"}`)
	if w.Code != 201 { t.Fatal(w.Code, w.Body.String()) }
	dev := m["device"].(map[string]any)
	id := dev["id"].(string)
	if dev["host"] != "10.0.0.5" || dev["port"].(float64) != 2222 || dev["username"] != "ana" || dev["has_secret"] != true || strings.Contains(w.Body.String(), "s3cret") { t.Fatal(w.Body.String()) }
	access, err := desktop.ResolveSSHDevice(s.InventoryDB, s.Vault, id)
	if err != nil || string(access.Secret) != "s3cret" || access.Username != "ana" || access.Port != 2222 { t.Fatalf("%+v %v", access, err) }
	w, m = postJSON(t, h.handleRemoteDevices, "/api/code-studio/remote/devices", `{"name":"K","host":"h","username":"u","auth":"key","secret":"not a key"}`)
	if w.Code != 400 || m["code"] != "invalid_private_key" { t.Fatal(w.Body.String()) }
	lw := httptest.NewRecorder()
	h.handleRemoteDevices(lw, httptest.NewRequest(http.MethodGet, "/api/code-studio/remote/devices", nil))
	if lw.Code != 200 || !strings.Contains(lw.Body.String(), `"Lab box"`) || !strings.Contains(lw.Body.String(), `"Fixture"`) || strings.Contains(lw.Body.String(), "vault") { t.Fatal(lw.Body.String()) }
}
```

**Run:** `go test ./internal/server/ -run '(?i)CodeStudioRemoteDevices' -count=1`. Expected: PASS.

**Commit:** `feat(code-studio): list and create SSH devices for workspaces`

### Task 17: Remote folder browser (6 min)

**Append to `code_studio_workspace_handlers.go`** and replace the browse stub. Add `os`, `path` (as `pathpkg`) and `sync` to the imports.

```go
type codeStudioBrowseEntry struct {
	Name     string    `json:"name"`
	Path     string    `json:"path"`
	Type     string    `json:"type"` // directory | file
	Size     int64     `json:"size"`
	Modified time.Time `json:"modified"`
	IsGit    bool      `json:"is_git"`
	IsLink   bool      `json:"is_link"`
	Hidden   bool      `json:"hidden"`
}

const codeStudioBrowseLimit = 2000

func (h codeStudioHandlers) handleRemoteBrowse(w http.ResponseWriter, r *http.Request) {
	if !h.requirePermission(w, r, desktopScopeRead) {
		return
	}
	if r.Method != http.MethodGet {
		jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if !codeStudioRemoteCallerAllowed(h.server, r) {
		writeCodeStudioOpError(w, codeStudioRemoteScopeError())
		return
	}
	deviceID := strings.TrimSpace(r.URL.Query().Get("device_id"))
	if deviceID == "" {
		jsonError(w, "device_id is required", http.StatusBadRequest)
		return
	}
	conn, err := h.remoteState().pool.get(r.Context(), deviceID, "")
	if err != nil {
		writeCodeStudioOpError(w, err) // same 409 host-key contract
		return
	}
	home, err := conn.sftp.Getwd()
	if err != nil || home == "" {
		home = "/"
	}
	raw := r.URL.Query().Get("path")
	if strings.TrimSpace(raw) == "" {
		raw = home
	}
	dir, err := normalizeCodeStudioRemoteRoot(raw, home)
	if err != nil {
		writeCodeStudioOpError(w, &codeStudioRemoteError{Status: http.StatusBadRequest, Code: "invalid_root", Message: err.Error()})
		return
	}
	infos, err := conn.sftp.ReadDir(dir)
	if err != nil {
		writeCodeStudioOpError(w, codeStudioSFTPError(err, dir))
		return
	}
	entries := make([]codeStudioBrowseEntry, 0, len(infos))
	for _, info := range infos {
		e := codeStudioBrowseEntry{Name: info.Name(), Path: pathpkg.Join(dir, info.Name()), Type: "file", Size: info.Size(),
			Modified: info.ModTime().UTC(), Hidden: strings.HasPrefix(info.Name(), "."), IsLink: info.Mode()&os.ModeSymlink != 0}
		if info.IsDir() {
			e.Type = "directory"
		} else if e.IsLink {
			if target, err := conn.sftp.Stat(e.Path); err == nil && target.IsDir() {
				e.Type = "directory"
			}
		}
		entries = append(entries, e)
	}
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].Type != entries[j].Type {
			return entries[i].Type == "directory"
		}
		return strings.ToLower(entries[i].Name) < strings.ToLower(entries[j].Name)
	})
	truncated := len(entries) > codeStudioBrowseLimit
	if truncated {
		entries = entries[:codeStudioBrowseLimit]
	}
	// is_git: bounded parallel Lstat of <dir>/.git for the first 500 directories.
	sem, wg, checked := make(chan struct{}, 8), sync.WaitGroup{}, 0
	for i := range entries {
		if entries[i].Type != "directory" || checked >= 500 {
			continue
		}
		checked++
		wg.Add(1)
		sem <- struct{}{}
		go func(e *codeStudioBrowseEntry) {
			defer func() { <-sem; wg.Done() }()
			if _, err := conn.sftp.Lstat(pathpkg.Join(e.Path, ".git")); err == nil {
				e.IsGit = true
			}
		}(&entries[i])
	}
	wg.Wait()
	parent := ""
	if dir != "/" {
		parent = pathpkg.Dir(dir)
	}
	writeJSON(w, map[string]interface{}{"status": "ok", "device_id": deviceID, "path": dir, "home": home,
		"parent": parent, "entries": entries, "truncated": truncated})
}
```

**Test (append):**

```go
func TestCodeStudioRemoteBrowseListsFoldersFirstWithGitFlag(t *testing.T) {
	f := startCodeStudioSSHFixture(t)
	admin := f.adminSFTP(t)
	for _, d := range []string{"/home/dev/proj/.git", "/home/dev/docs"} { if err := admin.MkdirAll(d); err != nil { t.Fatal(err) } }
	fh, _ := admin.Create("/home/dev/notes.txt")
	_, _ = fh.Write([]byte("n"))
	_ = fh.Close()
	s := testCodeStudioServerWithFakeCodeContainer(t, 1)
	remote := newCodeStudioRemote(f.testDial(t), slog.Default())
	t.Cleanup(remote.pool.closeAll)
	h := codeStudioHandlers{server: s, remote: remote}
	w := httptest.NewRecorder()
	h.handleRemoteBrowse(w, httptest.NewRequest(http.MethodGet, "/api/code-studio/remote/browse?device_id=dev1&path=/home/dev", nil))
	var body struct {
		Path, Parent string
		Entries []codeStudioBrowseEntry
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil || w.Code != 200 { t.Fatal(w.Code, w.Body.String()) }
	if body.Path != "/home/dev" || body.Parent != "/home" || len(body.Entries) != 3 { t.Fatalf("%+v", body) }
	if body.Entries[0].Name != "docs" || body.Entries[0].IsGit || body.Entries[1].Name != "proj" || !body.Entries[1].IsGit || body.Entries[2].Type != "file" { t.Fatalf("%+v", body.Entries) }
}
```

**Run:** `go test ./internal/server/ -run '(?i)CodeStudioRemoteBrowse' -count=1`. Expected: PASS.

**Commit:** `feat(code-studio): remote folder browser with git detection`

### Task 18: Final verification (5 min)

- `gofmt -l internal/server internal/desktop`. Expected: no new files listed.
- `go vet ./internal/server/ ./internal/desktop/`. Expected: no output.
- `go test ./internal/server/ -run '(?i)(CodeStudio)' -count=1`. Expected: `ok`. Add `-overlay` if untracked files of the other session break the build.
- `go test ./internal/desktop/ -run '(?i)(SSH|SFTP)' -count=1`. Expected: `ok`. This guards the untouched Quick Connect helpers.
- `git status`: confirm that none of the forbidden files is modified.
- GitNexus `detect-changes --scope all`, run from the main checkout. Report the changed symbols and processes. Expected: only `code_studio_*` and the new desktop file. A `partial` or `truncated` result is not clean; re-run it.
- Commit any fixups as `chore(code-studio): plan A verification fixups`.

---

## Interface contract (for Plans B, C and D)

### Go names (package `server` unless noted)

**Backend selection**

- `func (h *codeStudioHandlers) backendFor(r *http.Request) (codeStudioBackend, error)`. It reads `X-Code-Studio-Workspace`, then the query `ws`; missing or `local` means local. Write its errors with `writeCodeStudioBackendError(w, err)`.
- Error writers:
  - `writeCodeStudioOpError(w, err)` for operation errors.
  - `writeCodeStudioProbeError(w, err)` after `Probe`.
  - `writeCodeStudioRemoteError(w, *codeStudioRemoteError)`.

**Interface `codeStudioBackend`**

| Method | Contract |
|---|---|
| `Info() codeStudioWorkspaceInfo` | |
| `List(ctx, vdir string) ([]codeStudioFileEntry, error)` | |
| `Probe(ctx, vpath string) (codeStudioFileProbe, error)` | `Path` is the physical virtual path |
| `Read(ctx, vpath string, max int64) ([]byte, error)` | Over `max` gives 413 |
| `Write(ctx, vpath string, content []byte, createOnly bool) error` | `errors.Is(err, errCodeStudioExists)` gives 409 |
| `Move(ctx, from, to string) error` | Creates the destination parent |
| `Delete(ctx, vpath string) error` | Recursive |
| `Mkdir(ctx, vpath string) error` | |
| `Search(ctx, opts codeStudioSearchOptions) ([]codeStudioSearchResult, bool, error)` | 200-result cap |
| `Exec(ctx, argv []string, vcwd string, stdin []byte, timeout time.Duration) (codeStudioExecResult, error)` | `vcwd ""` = workspace root; `stdin nil` = no stdin; output capped at `codeStudioExecOutputLimit+1` |
| `Git(ctx, timeout time.Duration, args ...string) (codeStudioExecResult, error)` | |
| `OpenTerminal(ctx, cols, rows int) (containerTerminalSession, error)` | Serve with `serveContainerTerminalSession` |
| `StartProcess(ctx, argv []string, vcwd string) (codeStudioProcess, error)` | SSH only. Local returns a 501 `codeStudioStatusError` wrapping `errCodeStudioUnsupported` |

**Terminal handle `containerTerminalSession`** (existing): `io.ReadWriteCloser` plus `Resize(ctx, cols, rows int) error`. The SSH implementation also has `End(ctx) error`.

**Process handle `codeStudioProcess`**: `io.Reader` (stdout), `io.Writer` (stdin), `CloseWrite() error`, `Close() error` (kill), `Stderr() io.Reader` (the caller must drain it) and `Wait() error`. Plan D builds the LSP bridge on it. Its stdout is a raw byte stream, not Docker-multiplexed, so do not use `codeStudioLSPDemux`. Plan D must also rewrite `file:///workspace` to `file://<info.Root>`.

**Workspace types**

- `codeStudioWorkspaceInfo{ID, Kind ("local"|"ssh"), Label, Root, DeviceID, DeviceName, Host, Port, User, Connected, CreatedAt, LastUsedAt, Capabilities}`.
- `codeStudioCapabilities{OS, Arch, Shell, Git, Grep, Python3, Formatters map[string]bool, LSP codeStudioLSPCapabilities{Go, Python, TypeScript, CPP}, ProbedAt}`.

**Change hook (Plan B)**

- `func (h codeStudioHandlers) notifyWorkspaceChange(wsID string, paths ...string)`. `wsID` is `b.Info().ID` (`local` for the container). It is called after write, upload, move (old and new path), delete, mkdir and git discard; git switch/create and init call it with **no paths**, meaning the changed set is unknown.
- `func registerCodeStudioWorkspaceChangeHook(fn func(wsID string, paths []string)) (unregister func())`. Hooks run synchronously and must not block.
- Plan B registers its version bump here. Plan D calls `notifyWorkspaceChange` after LSP and agent writes.

**Other names**

- Constants: `codeStudioLocalWorkspaceID = "local"`, `codeStudioWorkspaceHeader = "X-Code-Studio-Workspace"`, `codeStudioWorkspaceQuery = "ws"`.
- Store: `h.workspaceStore(ctx) (*codeStudioWorkspaceStore, error)` with `Get`, `List`, `Upsert`, `Touch`, `Delete`, `CountDevice`; record type `codeStudioWorkspaceRecord`.
- Remote state: `h.remoteState() *codeStudioRemote`, which holds `.pool *codeStudioSSHPool` with `get(ctx, deviceID, trust)`, `withConn`, `connected` and `release`.
- Remote-access gate: `codeStudioRemoteCallerAllowed(s, r)`.
- Exported from `internal/desktop`: `SSHAccess`, `SSHHostKeyError{Code, Host, KeyType, Fingerprint}`, `SSHHostKeyUnknown`, `SSHHostKeyMismatch`, `SSHDialOptions{TrustFingerprint, KnownHostsFile, Timeout, Logger}`, `ResolveSSHDevice`, `DescribeSSHDevice`, `DialSSH`, `ErrSSHDeviceNotFound`.

### Error JSON

- Existing style: `{"error":"<message>"}`. Local behaviour is unchanged.
- Contract errors: `{"error":"<human message>","code":"<code>"[,"host","key_type","fingerprint"]}`. The codes are:

| Status | Codes |
|---|---|
| 400 | `invalid_workspace`, `invalid_root`, `not_a_directory`, `ssh_access_invalid`, `invalid_private_key`, `passphrase_required` |
| 403 | `remote_scope_required` |
| 404 | `workspace_not_found`, `device_not_found` |
| 409 | `unknown_host_key` (with host, key_type, fingerprint), `host_key_mismatch` (same fields; no override from Code Studio) |
| 429 | Plain `{"error"}`: too many terminals or processes on one connection |
| 501 | `lsp_unavailable` (LSP in an SSH workspace, until Plan D) |
| 502 | `ssh_auth_failed`, `ssh_unreachable`, `sftp_unavailable`, `ssh_connection_lost` |
| 503 | `inventory_unavailable`, or plain `{"error"}` when the desktop store or the container is unavailable |

**Host-key flow for the UI.** Any call can answer 409 `unknown_host_key`. Show the card (host, key_type, fingerprint), then repeat with `trust_host_key=<fingerprint>` (POST workspaces or connect), or call `POST /api/code-studio/remote/trust` and retry the original call.

### Endpoints

**`GET /api/code-studio/workspaces`** (scope read)

```json
{"status":"ok","workspaces":[
 {"id":"local","kind":"local","label":"Local workspace","root":"/workspace","connected":true},
 {"id":"ssh_abc...","kind":"ssh","label":"Proj","root":"/home/u/proj","device_id":"d1","device_name":"Lab box",
  "host":"10.0.0.5","port":22,"user":"u","connected":false,"created_at":"...","last_used_at":"...",
  "capabilities":{"os":"Linux","arch":"x86_64","shell":"/bin/bash","git":true,"grep":true,"python3":true,
   "formatters":{"gofmt":true},"lsp":{"go":true,"python":false,"typescript":false,"cpp":false},"probed_at":"..."}}]}
```

The list is sorted by `last_used_at` descending.

**`POST /api/code-studio/workspaces`** (scope write, remote gate)

- Request: `{"device_id":"d1","root":"/home/u/proj" | "~/proj","label":"optional","trust_host_key":"SHA256:...optional"}`.
- 200: `{"status":"ok","workspace":<info>}`. The same device and root update the existing entry and keep its id.
- Errors: 400, 404, 409, 502 as in the table.

**`DELETE /api/code-studio/workspaces/{id}`**

- 200: `{"status":"ok","id":"..."}`; 404 `workspace_not_found`. `local` gives 400.
- The pooled connection is closed when no other workspace uses the device and nothing is running on it.

**`POST /api/code-studio/workspaces/{id}/connect`**

- Optional body `{"trust_host_key":"SHA256:..."}`.
- 200: `{"status":"ok","workspace":<info with refreshed capabilities>}`. Same 409 contract.

**`GET /api/code-studio/remote/devices`** (scope read, remote gate). Devices with protocol `ssh` or empty, no secrets:

```json
{"status":"ok","devices":[{"id":"d1","name":"Lab box","host":"10.0.0.5","port":22,"username":"u",
 "description":"","tags":["code-studio"],"has_secret":true,"last_used_at":"..."}]}
```

**`POST /api/code-studio/remote/devices`** (scope write, remote gate)

- Request: `{"name":"Lab box","host":"10.0.0.5","port":22,"username":"u","auth":"password"|"key","secret":"...","passphrase":"optional"}`.
- 201: `{"status":"ok","device":<device as above>}`.
- What it creates: a vault secret, a `credentials` record of type `ssh`, and an inventory device with `protocol=ssh`, `credential_id` set and tag `code-studio`.
- Errors: 400 (validation, `invalid_private_key`, `passphrase_required`), 409 (duplicate name), 503 (no inventory or vault).

**`GET /api/code-studio/remote/browse?device_id=d1&path=/home/u`** (scope read, remote gate)

- `path` defaults to the remote home (`sftp.Getwd`); `~` is allowed.

```json
{"status":"ok","device_id":"d1","path":"/home/u","home":"/home/u","parent":"/home","truncated":false,
 "entries":[{"name":"proj","path":"/home/u/proj","type":"directory","size":4096,"modified":"...",
  "is_git":true,"is_link":false,"hidden":false}, {"name":"notes.txt","type":"file", "...":"..."}]}
```

- Directories come first, then names case-insensitively; at most 2000 entries; `is_git` is checked for the first 500 directories.
- Same 409 contract.

**`POST /api/code-studio/remote/trust`** (scope write, remote gate)

- Request: `{"device_id":"d1","fingerprint":"SHA256:..."}`.
- 200: `{"status":"ok"}` once the key is stored and the connection is pooled. 409 if the fingerprint does not match or the key changed.

**All existing `/api/code-studio/*` endpoints** accept `X-Code-Studio-Workspace` or `?ws=`. Request and response shapes are unchanged. The WebSocket endpoints (`/terminal`, `/lsp`) use `?ws=`. The terminal also accepts `cols` and `rows` (defaults 120x30).

### Known limitations (documented in code)

- Exec command strings and the git diff `file` parameter are not path-mapped; the UI sends workspace-relative paths for git.
- A fish login shell may alter `\\` inside the single-quoted `sh -c` wrapper.
- An SSH terminal ends when its WebSocket closes (no tmux-style survival).
- Path confinement is a UX guard, not a security boundary.
- Move and delete are not retried after a connection drop.
