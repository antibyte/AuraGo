package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"
)

type fakeSandboxConn struct {
	mu             sync.Mutex
	initializeCnt  int
	discoverCnt    int
	callCnt        int
	closeCnt       int
	languageOutput string
	lastTool       string
	lastArguments  map[string]interface{}
	activeCalls    int
	maxActiveCalls int
	blockExecute   bool
	executeStartCh chan struct{}
	releaseCh      chan struct{}
	closeCh        chan struct{} // closed when close() is called to unblock any pending callTool
}

func (f *fakeSandboxConn) initialize(_ context.Context, logger *slog.Logger) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.initializeCnt++
	return nil
}

func (f *fakeSandboxConn) discoverTools(_ context.Context, logger *slog.Logger) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.discoverCnt++
	return nil
}

func (f *fakeSandboxConn) callTool(ctx context.Context, toolName string, arguments map[string]interface{}) (string, error) {
	f.mu.Lock()
	f.callCnt++
	f.lastTool = toolName
	f.lastArguments = make(map[string]interface{}, len(arguments))
	for key, value := range arguments {
		f.lastArguments[key] = value
	}
	f.activeCalls++
	if f.activeCalls > f.maxActiveCalls {
		f.maxActiveCalls = f.activeCalls
	}
	blockExecute := f.blockExecute && toolName == "execute_supervised_code"
	startCh := f.executeStartCh
	releaseCh := f.releaseCh
	closeCh := f.closeCh
	f.mu.Unlock()

	defer func() {
		f.mu.Lock()
		f.activeCalls--
		f.mu.Unlock()
	}()

	if blockExecute {
		if startCh != nil {
			select {
			case startCh <- struct{}{}:
			default:
			}
		}
		if releaseCh != nil {
			select {
			case <-releaseCh:
			case <-closeCh:
				return "", fmt.Errorf("connection closed")
			case <-ctx.Done():
				return "", ctx.Err()
			}
		}
	}

	if toolName == "get_supported_languages" {
		f.mu.Lock()
		defer f.mu.Unlock()
		if f.languageOutput != "" {
			return f.languageOutput, nil
		}
		return `["python"]`, nil
	}
	return "ok", nil
}

func (f *fakeSandboxConn) close() {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.closeCnt++
	if f.closeCh != nil {
		close(f.closeCh)
	}
}

func (f *fakeSandboxConn) markReady() {}

func (f *fakeSandboxConn) toolCount() int { return 2 }

func testSandboxLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func TestSandboxConnectionCloseHasSingleOwner(t *testing.T) {
	transport := &fakeSandboxConn{closeCh: make(chan struct{})}
	mgr := &SandboxManager{logger: testSandboxLogger(), connFactory: func() (sandboxConn, error) { return transport, nil }}
	conn, err := mgr.openConnContext(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	var owners sync.WaitGroup
	for i := 0; i < 16; i++ {
		owners.Add(1)
		go func() { defer owners.Done(); conn.close() }()
	}
	owners.Wait()
	transport.mu.Lock()
	defer transport.mu.Unlock()
	if transport.closeCnt != 1 {
		t.Fatalf("transport closes = %d, want exactly one", transport.closeCnt)
	}
}

func TestSandboxManagerExecuteCodeWithoutKeepAliveCreatesFreshConnection(t *testing.T) {
	logger := testSandboxLogger()
	var (
		mu      sync.Mutex
		created []*fakeSandboxConn
	)

	mgr := &SandboxManager{
		logger: logger,
		status: SandboxStatus{Ready: true},
		cfg:    SandboxConfig{KeepAlive: false},
		connFactory: func() (sandboxConn, error) {
			conn := &fakeSandboxConn{}
			mu.Lock()
			created = append(created, conn)
			mu.Unlock()
			return conn, nil
		},
	}

	for i := 0; i < 2; i++ {
		if _, err := mgr.ExecuteCode(`print("hi")`, "python", nil, 5); err != nil {
			t.Fatalf("ExecuteCode returned error: %v", err)
		}
	}

	mu.Lock()
	defer mu.Unlock()
	if len(created) != 2 {
		t.Fatalf("created %d connections, want 2", len(created))
	}
	for i, conn := range created {
		if conn.initializeCnt != 1 {
			t.Fatalf("conn %d initialize count = %d, want 1", i, conn.initializeCnt)
		}
		if conn.discoverCnt != 1 {
			t.Fatalf("conn %d discover count = %d, want 1", i, conn.discoverCnt)
		}
		if conn.closeCnt != 1 {
			t.Fatalf("conn %d close count = %d, want 1", i, conn.closeCnt)
		}
		if conn.lastTool != "execute_supervised_code" {
			t.Fatalf("conn %d last tool = %q, want execute_supervised_code", i, conn.lastTool)
		}
	}
}

func TestSandboxManagerExecuteCodePassesTimeoutToMCP(t *testing.T) {
	conn := &fakeSandboxConn{}
	mgr := &SandboxManager{
		logger: testSandboxLogger(),
		status: SandboxStatus{Ready: true},
		cfg:    SandboxConfig{KeepAlive: true},
		connFactory: func() (sandboxConn, error) {
			return conn, nil
		},
	}

	if _, err := mgr.ExecuteCode(`print("hi")`, "python", nil, 10); err != nil {
		t.Fatalf("ExecuteCode returned error: %v", err)
	}

	conn.mu.Lock()
	timeout := conn.lastArguments["timeout"]
	conn.mu.Unlock()
	if timeout != 10 {
		t.Fatalf("MCP timeout argument = %#v, want 10", timeout)
	}
}

func TestSandboxManagerExecuteCodeWithKeepAliveReusesConnection(t *testing.T) {
	logger := testSandboxLogger()
	var (
		mu      sync.Mutex
		created []*fakeSandboxConn
	)

	mgr := &SandboxManager{
		logger: logger,
		status: SandboxStatus{Ready: true},
		cfg:    SandboxConfig{KeepAlive: true},
		connFactory: func() (sandboxConn, error) {
			conn := &fakeSandboxConn{}
			mu.Lock()
			created = append(created, conn)
			mu.Unlock()
			return conn, nil
		},
	}

	for i := 0; i < 2; i++ {
		if _, err := mgr.ExecuteCode(`print("hi")`, "python", nil, 5); err != nil {
			t.Fatalf("ExecuteCode returned error: %v", err)
		}
	}

	mu.Lock()
	if len(created) != 1 {
		mu.Unlock()
		t.Fatalf("created %d connections, want 1", len(created))
	}
	conn := created[0]
	mu.Unlock()

	if conn.initializeCnt != 1 {
		t.Fatalf("initialize count = %d, want 1", conn.initializeCnt)
	}
	if conn.discoverCnt != 1 {
		t.Fatalf("discover count = %d, want 1", conn.discoverCnt)
	}
	if conn.callCnt != 2 {
		t.Fatalf("call count = %d, want 2", conn.callCnt)
	}
	if conn.closeCnt != 0 {
		t.Fatalf("close count before Close = %d, want 0", conn.closeCnt)
	}

	mgr.Close()

	if conn.closeCnt != 1 {
		t.Fatalf("close count after Close = %d, want 1", conn.closeCnt)
	}
}

func TestSandboxManagerKeepAliveSerializesConcurrentCalls(t *testing.T) {
	logger := testSandboxLogger()
	conn := &fakeSandboxConn{
		blockExecute:   true,
		executeStartCh: make(chan struct{}, 2),
		releaseCh:      make(chan struct{}),
		closeCh:        make(chan struct{}),
	}

	mgr := &SandboxManager{
		logger: logger,
		status: SandboxStatus{Ready: true},
		cfg:    SandboxConfig{KeepAlive: true},
		connFactory: func() (sandboxConn, error) {
			return conn, nil
		},
	}

	errCh := make(chan error, 2)
	go func() {
		_, err := mgr.ExecuteCode(`print("one")`, "python", nil, 5)
		errCh <- err
	}()

	select {
	case <-conn.executeStartCh:
	case <-time.After(2 * time.Second):
		t.Fatal("first execute_code call did not start")
	}

	go func() {
		_, err := mgr.ExecuteCode(`print("two")`, "python", nil, 5)
		errCh <- err
	}()

	time.Sleep(100 * time.Millisecond)

	conn.mu.Lock()
	callCntBeforeRelease := conn.callCnt
	maxActiveBeforeRelease := conn.maxActiveCalls
	activeBeforeRelease := conn.activeCalls
	conn.mu.Unlock()

	if callCntBeforeRelease != 1 {
		t.Fatalf("call count before release = %d, want 1", callCntBeforeRelease)
	}
	if activeBeforeRelease != 1 {
		t.Fatalf("active calls before release = %d, want 1", activeBeforeRelease)
	}
	if maxActiveBeforeRelease != 1 {
		t.Fatalf("max active calls before release = %d, want 1", maxActiveBeforeRelease)
	}

	conn.releaseCh <- struct{}{}
	conn.releaseCh <- struct{}{}

	for i := 0; i < 2; i++ {
		if err := <-errCh; err != nil {
			t.Fatalf("ExecuteCode returned error: %v", err)
		}
	}

	conn.mu.Lock()
	defer conn.mu.Unlock()
	if conn.callCnt != 2 {
		t.Fatalf("call count after release = %d, want 2", conn.callCnt)
	}
	if conn.maxActiveCalls != 1 {
		t.Fatalf("max active calls after completion = %d, want 1", conn.maxActiveCalls)
	}
}

// TestSandboxManagerExecuteCodeTimeoutWithoutKeepAlive verifies that a timeout
// returns an error and closes the connection when keepAlive is disabled.
func TestSandboxManagerExecuteCodeTimeoutWithoutKeepAlive(t *testing.T) {
	logger := testSandboxLogger()
	conn := &fakeSandboxConn{
		blockExecute:   true,
		executeStartCh: make(chan struct{}, 1),
		releaseCh:      make(chan struct{}), // will never be released
		closeCh:        make(chan struct{}),
	}

	mgr := &SandboxManager{
		logger: logger,
		status: SandboxStatus{Ready: true},
		cfg:    SandboxConfig{KeepAlive: false},
		connFactory: func() (sandboxConn, error) {
			return conn, nil
		},
	}

	doneCh := make(chan error, 1)
	go func() {
		_, err := mgr.ExecuteCode(`print("blocking")`, "python", nil, 1) // 1 second timeout
		doneCh <- err
	}()

	// Wait for the call to start
	select {
	case <-conn.executeStartCh:
	case <-time.After(2 * time.Second):
		t.Fatal("execute_code call did not start")
	}

	// Wait for timeout error
	select {
	case err := <-doneCh:
		if err == nil {
			t.Fatal("expected timeout error, got nil")
		}
		if !strings.Contains(err.Error(), "TIMEOUT") {
			t.Fatalf("expected TIMEOUT error, got: %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("timeout did not occur within expected time")
	}

	// Verify connection was closed after timeout
	conn.mu.Lock()
	closeCnt := conn.closeCnt
	conn.mu.Unlock()

	if closeCnt != 1 {
		t.Fatalf("close count = %d, want 1 (connection should be closed after timeout)", closeCnt)
	}
}

// TestSandboxManagerExecuteCodeTimeoutWithKeepAlive verifies that a timeout
// returns an error, closes the connection, and marks it unhealthy so the
// next call creates a fresh connection.
func TestSandboxManagerExecuteCodeTimeoutWithKeepAlive(t *testing.T) {
	logger := testSandboxLogger()
	conn := &fakeSandboxConn{
		blockExecute:   true,
		executeStartCh: make(chan struct{}, 1),
		releaseCh:      make(chan struct{}), // will never be released
		closeCh:        make(chan struct{}),
	}

	mgr := &SandboxManager{
		logger: logger,
		status: SandboxStatus{Ready: true},
		cfg:    SandboxConfig{KeepAlive: true},
		connFactory: func() (sandboxConn, error) {
			return conn, nil
		},
	}

	// First call should timeout
	doneCh := make(chan error, 1)
	go func() {
		_, err := mgr.ExecuteCode(`print("blocking")`, "python", nil, 1) // 1 second timeout
		doneCh <- err
	}()

	select {
	case <-conn.executeStartCh:
	case <-time.After(2 * time.Second):
		t.Fatal("first execute_code call did not start")
	}

	select {
	case err := <-doneCh:
		if err == nil {
			t.Fatal("expected timeout error, got nil")
		}
		if !strings.Contains(err.Error(), "TIMEOUT") {
			t.Fatalf("expected TIMEOUT error, got: %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("timeout did not occur within expected time")
	}

	// Verify connection was marked unhealthy (closed)
	// markUnhealthy runs asynchronously, so give it time to complete
	time.Sleep(100 * time.Millisecond)

	conn.mu.Lock()
	closeCntAfterTimeout := conn.closeCnt
	conn.mu.Unlock()

	if closeCntAfterTimeout != 1 {
		t.Fatalf("close count after timeout = %d, want 1", closeCntAfterTimeout)
	}

	// Reset for next call - create a fresh connection
	newConn := &fakeSandboxConn{
		blockExecute: false, // next call completes normally
	}
	var createdNewConn bool
	origFactory := mgr.connFactory
	mgr.connFactory = func() (sandboxConn, error) {
		createdNewConn = true
		return newConn, nil
	}

	// Second call should succeed with fresh connection
	_, err := mgr.ExecuteCode(`print("ok")`, "python", nil, 5)
	if err != nil {
		t.Fatalf("second ExecuteCode failed: %v", err)
	}
	if !createdNewConn {
		t.Fatal("expected new connection to be created after timeout, but old connection was reused")
	}
	if newConn.callCnt != 1 {
		t.Fatalf("new conn call count = %d, want 1", newConn.callCnt)
	}
	mgr.connFactory = origFactory
}

// TestSandboxManagerTimeoutCleansUpGoroutine verifies that after a timeout,
// the blocked goroutine is properly terminated when the connection is closed.
func TestSandboxManagerTimeoutCleansUpGoroutine(t *testing.T) {
	logger := testSandboxLogger()
	conn := &fakeSandboxConn{
		blockExecute:   true,
		executeStartCh: make(chan struct{}, 1),
		releaseCh:      make(chan struct{}, 1), // buffered so we can release it
		closeCh:        make(chan struct{}),
	}

	mgr := &SandboxManager{
		logger: logger,
		status: SandboxStatus{Ready: true},
		cfg:    SandboxConfig{KeepAlive: true},
		connFactory: func() (sandboxConn, error) {
			return conn, nil
		},
	}

	activeCallsCh := make(chan int, 1)
	go func() {
		_, err := mgr.ExecuteCode(`print("blocking")`, "python", nil, 1)
		if err != nil && strings.Contains(err.Error(), "TIMEOUT") {
			// Wait a bit for cleanup
			time.Sleep(100 * time.Millisecond)
		}
		conn.mu.Lock()
		active := conn.activeCalls
		conn.mu.Unlock()
		activeCallsCh <- active
	}()

	select {
	case <-conn.executeStartCh:
	case <-time.After(2 * time.Second):
		t.Fatal("execute_code call did not start")
	}

	// Wait for goroutine to finish (after timeout and cleanup)
	activeAfterTimeout := <-activeCallsCh
	if activeAfterTimeout != 0 {
		t.Fatalf("active calls after timeout cleanup = %d, want 0 (goroutine should be terminated)", activeAfterTimeout)
	}
}

func TestSandboxMCPEnvUsesNetworkModeAndImage(t *testing.T) {
	off, err := sandboxMCPEnv(SandboxConfig{Backend: "docker", Image: defaultSandboxImage})
	if err != nil {
		t.Fatal(err)
	}
	if off["SANDBOX_NETWORK_MODE"] != "none" {
		t.Fatalf("network off = %#v", off)
	}
	if _, ok := off["ENABLE_NETWORKING"]; ok {
		t.Fatalf("ENABLE_NETWORKING must not be set: %#v", off)
	}
	if off["IMAGE"] != defaultSandboxImage {
		t.Fatalf("image = %q", off["IMAGE"])
	}

	on, err := sandboxMCPEnv(SandboxConfig{Backend: "podman", NetworkEnabled: true})
	if err != nil {
		t.Fatal(err)
	}
	if on["SANDBOX_NETWORK_MODE"] != "none" || on["BACKEND"] != "podman" {
		t.Fatalf("network on = %#v", on)
	}
	if _, ok := on["IMAGE"]; ok {
		t.Fatal("empty image must stay unset")
	}
}

func TestSandboxMCPEnvRejectsUnsafeImage(t *testing.T) {
	if _, err := sandboxMCPEnv(SandboxConfig{Image: "python:3.11-slim;touch /tmp/x"}); err == nil {
		t.Fatal("expected image rejection")
	}
}

func TestLLMSandboxRequirementIsPinned(t *testing.T) {
	got, err := llmSandboxRequirement("docker")
	if err != nil {
		t.Fatal(err)
	}
	if got != "llm-sandbox[mcp-docker]==0.3.45" {
		t.Fatalf("requirement = %q", got)
	}
	got, err = llmSandboxRequirement("podman")
	if err != nil || got != "llm-sandbox[mcp-podman]==0.3.45" {
		t.Fatalf("podman requirement = %q, err = %v", got, err)
	}
}

func TestSandboxProcessEnvClearsImageWhenUnset(t *testing.T) {
	unset, err := sandboxProcessEnv(SandboxConfig{})
	if err != nil {
		t.Fatal(err)
	}
	image, ok := unset["IMAGE"]
	if !ok || image != "" {
		t.Fatalf("IMAGE present=%v value=%q, want empty string", ok, image)
	}
	if unset["SANDBOX_NETWORK_MODE"] != "none" {
		t.Fatalf("SANDBOX_NETWORK_MODE = %q", unset["SANDBOX_NETWORK_MODE"])
	}

	set, err := sandboxProcessEnv(SandboxConfig{Image: defaultSandboxImage})
	if err != nil {
		t.Fatal(err)
	}
	if set["IMAGE"] != defaultSandboxImage {
		t.Fatalf("IMAGE = %q", set["IMAGE"])
	}
}

func TestSandboxInstallProbeDoesNotTreatMCPPackageAsSandbox(t *testing.T) {
	src, err := os.ReadFile("sandbox.go")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(src), `pip", "show", "mcp"`) || strings.Contains(string(src), "pip show") {
		t.Fatal("mcp package presence must not count as llm-sandbox installed")
	}
}

func TestSandboxMCPLauncherInjectsImageIntoFunctionAndClass(t *testing.T) {
	python := sandboxLauncherPython(t)
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("test file path unavailable")
	}
	launcher := filepath.Join(filepath.Dir(file), "sandbox_mcp_launch.py")

	const configured = "ghcr.io/example/sandbox:1"
	withImage := runSandboxLauncherProbe(t, python, launcher, configured)
	assertLauncherImage(t, withImage.Function, configured, "function")
	assertLauncherImage(t, withImage.Class, configured, "class")
	assertLauncherImage(t, withImage.FunctionOverride, "caller:supplied", "function override")
	assertLauncherImage(t, withImage.ClassOverride, "caller:supplied", "class override")

	empty := runSandboxLauncherProbe(t, python, launcher, "")
	if empty.Function != nil || empty.Class != nil {
		t.Fatalf("empty IMAGE injected function=%v class=%v", ptrString(empty.Function), ptrString(empty.Class))
	}
	assertLauncherImage(t, empty.FunctionOverride, "caller:supplied", "empty function override")
	assertLauncherImage(t, empty.ClassOverride, "caller:supplied", "empty class override")
}

type sandboxLauncherProbe struct {
	Function         *string `json:"function"`
	Class            *string `json:"class"`
	FunctionOverride *string `json:"function_override"`
	ClassOverride    *string `json:"class_override"`
}

func sandboxLauncherPython(t *testing.T) string {
	t.Helper()
	for _, name := range []string{"python", "python3"} {
		path, err := exec.LookPath(name)
		if err == nil {
			return path
		}
	}
	t.Skip("python and python3 are not on PATH")
	return ""
}

func runSandboxLauncherProbe(t *testing.T, python, launcher, image string) sandboxLauncherProbe {
	t.Helper()
	cmd := exec.Command(python, "-", launcher)
	cmd.Stdin = strings.NewReader(sandboxLauncherProbeScript)
	cmd.Env = append(os.Environ(), "IMAGE="+image)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("launcher probe IMAGE=%q: %v\nstderr:\n%s\nstdout:\n%s", image, err, stderr.String(), stdout.String())
	}
	var got sandboxLauncherProbe
	if err := json.Unmarshal(bytes.TrimSpace(stdout.Bytes()), &got); err != nil {
		t.Fatalf("launcher probe JSON: %v\nstdout:\n%s", err, stdout.String())
	}
	return got
}

func assertLauncherImage(t *testing.T, got *string, want, label string) {
	t.Helper()
	if got == nil || *got != want {
		t.Fatalf("%s image = %s, want %q", label, ptrString(got), want)
	}
}

func ptrString(value *string) string {
	if value == nil {
		return "<nil>"
	}
	return fmt.Sprintf("%q", *value)
}

const sandboxLauncherProbeScript = `
import importlib.util
import json
import sys
import types

launcher = sys.argv[1]
calls = {"function": [], "class": []}

def sandbox_session(*args, **kwargs):
    if "image" in kwargs:
        calls["function"].append(kwargs["image"])
    else:
        calls["function"].append(None)
    return dict(kwargs)

class ArtifactSandboxSession:
    def __init__(self, *args, **kwargs):
        if "image" in kwargs:
            calls["class"].append(kwargs["image"])
        else:
            calls["class"].append(None)

name = "llm_sandbox.mcp_server.server"
for mod_name in ("llm_sandbox", "llm_sandbox.mcp_server", name):
    sys.modules[mod_name] = types.ModuleType(mod_name)
server = sys.modules[name]
server.SandboxSession = sandbox_session
server.ArtifactSandboxSession = ArtifactSandboxSession
server.main = lambda: None

spec = importlib.util.spec_from_file_location("aurago_sandbox_mcp_launch", launcher)
module = importlib.util.module_from_spec(spec)
spec.loader.exec_module(module)

server.SandboxSession(lang="javascript")
server.ArtifactSandboxSession(lang="python")
server.SandboxSession(lang="javascript", image="caller:supplied")
server.ArtifactSandboxSession(lang="python", image="caller:supplied")
print(json.dumps({
    "function": calls["function"][0],
    "class": calls["class"][0],
    "function_override": calls["function"][1],
    "class_override": calls["class"][1],
}))
`

func TestSandboxReady(t *testing.T) {
	sandboxMgrMu.Lock()
	old := globalSandboxMgr
	globalSandboxMgr = nil
	sandboxMgrMu.Unlock()
	t.Cleanup(func() {
		sandboxMgrMu.Lock()
		globalSandboxMgr = old
		sandboxMgrMu.Unlock()
	})

	if SandboxReady() {
		t.Fatal("SandboxReady() = true, want false")
	}
}
