package tools

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestDefaultExecutorFailsClosedWithoutSupervisor(t *testing.T) {
	perms := defaultRuntimePermissionsForTests()
	perms.AllowUnsafeHostExecution = false
	perms.AllowUnsandboxedShell = false
	ConfigureRuntimePermissions(perms)
	t.Cleanup(func() { ConfigureRuntimePermissions(defaultRuntimePermissionsForTests()) })
	sandboxMgrMu.Lock()
	previous := globalSandboxMgr
	globalSandboxMgr = nil
	sandboxMgrMu.Unlock()
	t.Cleanup(func() { sandboxMgrMu.Lock(); globalSandboxMgr = previous; sandboxMgrMu.Unlock() })
	marker := filepath.Join(t.TempDir(), "escaped")
	_, _, err := ExecutePythonContext(context.Background(), "open("+`"`+filepath.ToSlash(marker)+`"`+", 'w').write('bad')", t.TempDir(), t.TempDir())
	if !errors.Is(err, ErrSandboxUnavailable) {
		t.Fatalf("expected unavailable supervisor, got %v", err)
	}
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatal("host source executed")
	}
	_, _, err = ExecuteShell(context.Background(), "echo unsafe", t.TempDir())
	if !errors.Is(err, ErrSandboxUnavailable) {
		t.Fatalf("shell escaped supervisor: %v", err)
	}
}

func TestSandboxCallerCancellationAndBorrowWait(t *testing.T) {
	fake := &fakeSandboxConn{blockExecute: true, executeStartCh: make(chan struct{}, 1), closeCh: make(chan struct{})}
	mgr := &SandboxManager{logger: slog.New(slog.NewTextHandler(io.Discard, nil)), conn: fake, status: SandboxStatus{Ready: true}, cfg: SandboxConfig{KeepAlive: true}, transportGrace: time.Second}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := mgr.ExecuteCodeContext(ctx, "print(1)", "python", nil, 30); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled caller: %v", err)
	}
	mgr.sharedConnMu.Lock()
	waitCtx, stop := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer stop()
	_, err := mgr.ExecuteCodeContext(waitCtx, "print(1)", "python", nil, 30)
	mgr.sharedConnMu.Unlock()
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("borrow ignored deadline: %v", err)
	}
}

func TestBrowserPublicationRequiresWriteAndRoot(t *testing.T) {
	perms := defaultRuntimePermissionsForTests()
	perms.AllowFilesystemWrite = false
	ConfigureRuntimePermissions(perms)
	t.Cleanup(func() { ConfigureRuntimePermissions(defaultRuntimePermissionsForTests()) })
	if _, _, err := browserOutputRoot(context.Background(), t.TempDir(), "capture"); err == nil {
		t.Fatal("write gate bypassed")
	}
	perms.AllowFilesystemWrite = true
	ConfigureRuntimePermissions(perms)
	if _, _, err := browserOutputRoot(context.Background(), t.TempDir(), "../../outside"); err == nil {
		t.Fatal("output root escaped")
	}
}

func TestLintRejectsOptionInjectionBeforeLookup(t *testing.T) {
	for _, arg := range []string{"--config=/tmp/evil", " -v", "x\n--fix"} {
		if got := ExecuteGolangciLint(arg, "", t.TempDir()); !strings.Contains(got, "not an option") {
			t.Fatalf("accepted %q: %s", arg, got)
		}
	}
}

type isolatedResponseConn struct{ *fakeSandboxConn }

func (c *isolatedResponseConn) callTool(ctx context.Context, name string, args map[string]interface{}) (string, error) {
	_, err := c.fakeSandboxConn.callTool(ctx, name, args)
	return `{"exit_code":0,"stdout":"worker-ok","stderr":""}`, err
}

func TestScopedBridgeTokenStaysInSupervisorArguments(t *testing.T) {
	perms := defaultRuntimePermissionsForTests()
	perms.AllowUnsafeHostExecution = false
	ConfigureRuntimePermissions(perms)
	t.Cleanup(func() { ConfigureRuntimePermissions(defaultRuntimePermissionsForTests()) })
	conn := &isolatedResponseConn{&fakeSandboxConn{}}
	mgr := &SandboxManager{logger: testSandboxLogger(), status: SandboxStatus{Ready: true}, cfg: SandboxConfig{KeepAlive: true}, conn: conn}
	sandboxMgrMu.Lock()
	previous := globalSandboxMgr
	globalSandboxMgr = mgr
	sandboxMgrMu.Unlock()
	t.Cleanup(func() { sandboxMgrMu.Lock(); globalSandboxMgr = previous; sandboxMgrMu.Unlock() })
	const token = "scoped-fixture-only-secret"
	stdout, _, err := ExecutePythonWithOptions(PythonExecutionOptions{Context: context.Background(), Code: "import aurago", ToolBridgeURL: "http://127.0.0.1:1234/api/tool-bridge", ToolBridgeToken: token, ToolBridgeAllowedTools: []string{"filesystem"}})
	if err != nil || stdout != "worker-ok" {
		t.Fatalf("broker execution: %q %v", stdout, err)
	}
	if conn.lastTool != "execute_brokered_code" || conn.lastArguments["broker_token"] != token {
		t.Fatal("grant did not reach trusted broker")
	}
	code, _ := conn.lastArguments["code"].(string)
	if strings.Contains(code, token) || strings.Contains(code, "127.0.0.1") {
		t.Fatal("worker source received control credential or origin")
	}
	encoded, _ := json.Marshal(conn.lastArguments)
	if !strings.Contains(string(encoded), "allowed_tools") {
		t.Fatal("supervisor allowlist omitted")
	}
	if _, _, err := ExecutePythonWithOptions(PythonExecutionOptions{Code: "print(1)", Secrets: map[string]string{"secret": token}}); err == nil {
		t.Fatal("worker received host credentials")
	}
}

func TestDefaultSandboxImageMatchesReviewedManifest(t *testing.T) {
	data, err := os.ReadFile("sandbox_images.json")
	if err != nil {
		t.Fatal(err)
	}
	var manifest struct {
		Images map[string]struct {
			Reference string `json:"reference"`
		} `json:"images"`
	}
	if err := json.Unmarshal(data, &manifest); err != nil {
		t.Fatal(err)
	}
	if manifest.Images["python"].Reference != defaultSandboxImage {
		t.Fatal("default image differs from reviewed manifest")
	}
	if _, err := sandboxMCPEnv(SandboxConfig{Image: "python:latest"}); err == nil {
		t.Fatal("mutable image accepted")
	}
}
