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
	// errCodeStudioWorkspaceNotFound is wrapped by the 404 workspace_not_found error.
	errCodeStudioWorkspaceNotFound = errors.New("code studio workspace not found")
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
	return &codeStudioRemoteError{Status: http.StatusNotFound, Code: "workspace_not_found", Message: "workspace not found",
		Err: errCodeStudioWorkspaceNotFound}
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
