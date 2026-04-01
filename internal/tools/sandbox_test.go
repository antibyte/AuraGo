package tools

import (
	"io"
	"log/slog"
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
	activeCalls    int
	maxActiveCalls int
	blockExecute   bool
	executeStartCh chan struct{}
	releaseCh      chan struct{}
}

func (f *fakeSandboxConn) initialize(logger *slog.Logger) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.initializeCnt++
	return nil
}

func (f *fakeSandboxConn) discoverTools(logger *slog.Logger) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.discoverCnt++
	return nil
}

func (f *fakeSandboxConn) callTool(toolName string, arguments map[string]interface{}) (string, error) {
	f.mu.Lock()
	f.callCnt++
	f.lastTool = toolName
	f.activeCalls++
	if f.activeCalls > f.maxActiveCalls {
		f.maxActiveCalls = f.activeCalls
	}
	blockExecute := f.blockExecute && toolName == "execute_code"
	startCh := f.executeStartCh
	releaseCh := f.releaseCh
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
			<-releaseCh
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
}

func (f *fakeSandboxConn) markReady() {}

func (f *fakeSandboxConn) toolCount() int { return 2 }

func testSandboxLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
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
		if conn.lastTool != "execute_code" {
			t.Fatalf("conn %d last tool = %q, want execute_code", i, conn.lastTool)
		}
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
