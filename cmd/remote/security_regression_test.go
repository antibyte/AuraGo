package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"aurago/internal/fileutil"
	"aurago/internal/remote"
	"github.com/gorilla/websocket"
)

func TestExecutorFileLimitsAndBinaryIntegrity(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "data")
	executor := NewExecutor(slog.Default(), 1)
	data := bytes.Repeat([]byte{0, 255, 1, 254}, (1<<20)/4)
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	result := executor.Execute(remote.CommandPayload{Operation: remote.OpFileRead, Args: map[string]interface{}{"path": path}}, false, []string{dir})
	decoded, err := base64.StdEncoding.DecodeString(result.Output)
	if err != nil || result.Status != "ok" || result.Truncated || !bytes.Equal(data, decoded) {
		t.Fatalf("binary read: status=%s err=%v truncated=%v", result.Status, err, result.Truncated)
	}
	oversized := append(data, byte(1))
	if err := os.WriteFile(path, oversized, 0600); err != nil {
		t.Fatal(err)
	}
	for _, operation := range []string{remote.OpFileRead, remote.OpFileReadAdv, remote.OpFileEdit, remote.OpJsonEdit, remote.OpYamlEdit, remote.OpXmlEdit} {
		t.Run(operation, func(t *testing.T) {
			result := executor.Execute(remote.CommandPayload{Operation: operation, Args: map[string]interface{}{"path": path, "operation": "append", "content": "x"}}, false, []string{dir})
			if result.Status == "ok" {
				t.Fatal("oversized input was accepted")
			}
			got, _ := os.ReadFile(path)
			if !bytes.Equal(got, oversized) {
				t.Fatal("failed operation changed original")
			}
		})
	}
	result = executor.Execute(remote.CommandPayload{Operation: remote.OpFileWrite, Args: map[string]interface{}{"path": path, "content": base64.StdEncoding.EncodeToString(oversized)}}, false, []string{dir})
	if result.Status == "ok" {
		t.Fatal("oversized write accepted")
	}
}

func TestExecutorAtomicPublicationCancelledKeepsOriginal(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "original.txt")
	if err := os.WriteFile(path, []byte("original"), 0640); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	publications := 0
	ctx = fileutil.WithPublicationGate(ctx, func(commit func() error) error {
		publications++
		cancel()
		return ctx.Err()
	})
	executor := NewExecutor(slog.Default(), 1)
	result := executor.ExecuteContext(ctx, remote.CommandPayload{Operation: remote.OpFileEdit, Args: map[string]interface{}{"path": path, "operation": "append", "content": "replacement"}}, false, []string{dir})
	if result.Status == "ok" || publications != 1 {
		t.Fatalf("publication=%d status=%s error=%s", publications, result.Status, result.Error)
	}
	got, _ := os.ReadFile(path)
	if string(got) != "original" {
		t.Fatal("cancelled publication replaced original")
	}
	entries, _ := os.ReadDir(dir)
	if len(entries) != 1 {
		t.Fatal("temporary file leaked")
	}
}

func TestExecutorRootedSearchRejectsSymlinkEscape(t *testing.T) {
	dir, outside := t.TempDir(), t.TempDir()
	secret := filepath.Join(outside, "secret.txt")
	if err := os.WriteFile(secret, []byte("outside-marker"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "allowed.txt"), []byte("allowed-marker"), 0600); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(dir, "link.txt")
	if err := os.Symlink(secret, link); err != nil {
		t.Skipf("symlink privileges unavailable: %v", err)
	}
	executor := NewExecutor(slog.Default(), 1)
	result := executor.Execute(remote.CommandPayload{Operation: remote.OpFileSearch, Args: map[string]interface{}{"path": dir, "operation": "grep_recursive", "pattern": "marker"}}, false, []string{dir})
	if result.Status != "ok" || strings.Contains(result.Output, "outside-marker") || !strings.Contains(result.Output, "allowed-marker") {
		t.Fatalf("rooted search: %+v", result)
	}
	result = executor.Execute(remote.CommandPayload{Operation: remote.OpFileRead, Args: map[string]interface{}{"path": link}}, false, []string{dir})
	if result.Status == "ok" {
		t.Fatal("rooted read followed outside symlink")
	}
}

func TestExecutorAdmissionAndCompletedResultBudget(t *testing.T) {
	executor := NewExecutor(slog.Default(), 1)
	for i := 0; i < maxConcurrentCommands; i++ {
		if _, owner, refusal := executor.admit(fmt.Sprint(i)); !owner || refusal != nil {
			t.Fatal("early admission failure")
		}
	}
	refused := false
	executor.Start(context.Background(), remote.CommandPayload{CommandID: "fifth"}, false, nil, func(result remote.ResultPayload) { refused = result.ErrorCode == "CLIENT_BUSY" })
	if !refused {
		t.Fatal("overload was not refused synchronously")
	}
	for i := 0; i < 40; i++ {
		id := fmt.Sprintf("completed-%d", i)
		r := &commandExecutionRecord{done: make(chan struct{}), result: remote.ResultPayload{CommandID: id, Output: strings.Repeat("x", 1<<20)}}
		r.closeDone()
		executor.commandResults[id] = r
		executor.commandOrder = append(executor.commandOrder, id)
	}
	executor.trimCommandResultCacheLocked("")
	var size int
	for _, record := range executor.commandResults {
		size += len(record.result.Output) + len(record.result.Error)
	}
	if size > maxCachedResultBytes {
		t.Fatalf("cache exceeds budget: %d", size)
	}
	if r, owner, _ := executor.admit("completed-0"); owner || r.result.ErrorCode != "RESULT_EVICTED" {
		t.Fatal("evicted result allowed replay")
	}
	if _, owner, _ := executor.admit("0"); owner {
		t.Fatal("active ID allowed replay")
	}
	for i := 40; i < 1100; i++ {
		id := fmt.Sprintf("completed-%d", i)
		r := &commandExecutionRecord{done: make(chan struct{})}
		r.closeDone()
		executor.commandResults[id] = r
		executor.commandOrder = append(executor.commandOrder, id)
	}
	executor.trimCommandResultCacheLocked("")
	if len(executor.commandResults) != maxCachedCommandResults+maxConcurrentCommands {
		t.Fatalf("ID window = %d", len(executor.commandResults))
	}
}

func TestClientRejectsPlaintextAndInvalidCertificates(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { t.Error("untrusted request reached server") }))
	defer server.Close()
	for _, endpoint := range []string{"ws://127.0.0.1:9/api/remote/ws", "wss" + strings.TrimPrefix(server.URL, "https")} {
		client := &Client{cfg: clientConfig{SupervisorURL: endpoint}, logger: slog.Default(), done: make(chan struct{})}
		if err := client.connect(); err == nil {
			t.Fatal("unsafe endpoint accepted")
		}
		client.Stop()
	}
}

func TestClientStopInterruptsAuthentication(t *testing.T) {
	accepted := make(chan struct{})
	upgrader := websocket.Upgrader{}
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()
		var frame remote.RemoteMessage
		if conn.ReadJSON(&frame) == nil {
			close(accepted)
			_, _, _ = conn.ReadMessage()
		}
	}))
	defer server.Close()
	trustTestSupervisor(t, server)
	client := &Client{cfg: clientConfig{SupervisorURL: "wss" + strings.TrimPrefix(server.URL, "https")}, logger: slog.Default(), done: make(chan struct{})}
	finished := make(chan error, 1)
	go func() { finished <- client.connect() }()
	select {
	case <-accepted:
	case <-time.After(3 * time.Second):
		t.Fatal("no auth")
	}
	client.Stop()
	select {
	case err := <-finished:
		if err == nil {
			t.Fatal("stopped client connected")
		}
	case <-time.After(time.Second):
		t.Fatal("Stop did not cancel handshake")
	}
}

func TestExplicitTokenAndFailedMigrationPreserveStoredIdentity(t *testing.T) {
	isolateRemoteHome(t)
	previous := clientConfig{SupervisorURL: "wss://supervisor.invalid/api/remote/ws", DeviceID: "old-device", SharedKey: strings.Repeat("ab", 32)}
	if err := saveConfig(previous); err != nil {
		t.Fatal(err)
	}
	before, _ := os.ReadFile(configPath())
	cfg, err := loadConfig("", "replacement-token", "")
	if err != nil || cfg.DeviceID != "" || cfg.SharedKey != "" || cfg.EnrollToken != "replacement-token" {
		t.Fatalf("explicit token did not replace in-memory identity: %v", err)
	}
	got, _ := os.ReadFile(configPath())
	if !bytes.Equal(before, got) {
		t.Fatal("loading token overwrote identity")
	}
	oldOverride := configFileOverride
	t.Cleanup(func() { configFileOverride = oldOverride })
	configFileOverride = filepath.Join(t.TempDir(), "invalid.json")
	if err := os.WriteFile(configFileOverride, []byte("broken"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := prepareServiceConfig(cfg, false, true); err == nil {
		t.Fatal("corrupt target overwritten")
	}
	got, _ = os.ReadFile(configFileOverride)
	if string(got) != "broken" {
		t.Fatal("failed migration changed target")
	}
}

func TestRemoteTextOutputIsBounded(t *testing.T) {
	var output boundedOutput
	_, _ = output.Write(bytes.Repeat([]byte("é"), 1<<20))
	if value := output.String(); len(value) > remote.MaxTextOutputBytes || !strings.HasSuffix(value, truncationMarker) {
		t.Fatal("text output is unbounded or unmarked")
	}
}

func TestServiceReinstallPreservesIdentityAndPendingReplacement(t *testing.T) {
	oldOverride := configFileOverride
	t.Cleanup(func() { configFileOverride = oldOverride })
	configFileOverride = filepath.Join(t.TempDir(), "config.json")
	previous := clientConfig{SupervisorURL: "wss://supervisor.invalid/api/remote/ws", DeviceName: "existing", DeviceID: "old-device", SharedKey: strings.Repeat("ab", 32)}
	if err := saveConfig(previous); err != nil {
		t.Fatal(err)
	}
	for _, token := range []string{"", "synthetic-replacement-token"} {
		if err := prepareServiceConfig(clientConfig{EnrollToken: token}, false, token != ""); err != nil {
			t.Fatal(err)
		}
		stored, err := readStoredConfig(configFileOverride)
		if err != nil || stored == nil || stored.DeviceID != previous.DeviceID || stored.SharedKey != previous.SharedKey || stored.SupervisorURL != previous.SupervisorURL || stored.DeviceName != previous.DeviceName || stored.EnrollToken != token {
			t.Fatal("reinstall lost stored identity, endpoint, name or pending replacement")
		}
		loaded, err := loadConfig("", "", "")
		if err != nil {
			t.Fatal(err)
		}
		if token != "" && (loaded.DeviceID != "" || loaded.SharedKey != "" || loaded.EnrollToken != token) {
			t.Fatal("pending replacement did not force enrollment")
		}
	}
}

func TestServiceReinstallDoesNotEnrollUsingDownloadTrailer(t *testing.T) {
	oldOverride := configFileOverride
	t.Cleanup(func() { configFileOverride = oldOverride })
	configFileOverride = filepath.Join(t.TempDir(), "config.json")
	previous := clientConfig{SupervisorURL: "wss://previous.invalid/api/remote/ws", DeviceID: "old-device", SharedKey: strings.Repeat("ab", 32)}
	if err := saveConfig(previous); err != nil {
		t.Fatal(err)
	}
	download := clientConfig{SupervisorURL: "wss://download.invalid/api/remote/ws", EnrollToken: "synthetic-trailer-token"}
	if err := prepareServiceConfig(download, false, false); err != nil {
		t.Fatal(err)
	}
	stored, err := readStoredConfig(configPath())
	if err != nil || stored.SupervisorURL != previous.SupervisorURL || stored.DeviceID != previous.DeviceID || stored.SharedKey != previous.SharedKey || stored.EnrollToken != "" {
		t.Fatal("download trailer replaced the installed service identity")
	}
}

func TestServiceReplacementPreservesPreviousSupervisorOnFailure(t *testing.T) {
	oldOverride := configFileOverride
	t.Cleanup(func() { configFileOverride = oldOverride })
	configFileOverride = filepath.Join(t.TempDir(), "config.json")
	previous := clientConfig{SupervisorURL: "wss://previous.invalid/api/remote/ws", DeviceID: "old-device", SharedKey: strings.Repeat("ab", 32)}
	if err := saveConfig(previous); err != nil {
		t.Fatal(err)
	}
	server := httptest.NewTLSServer(http.NotFoundHandler())
	defer server.Close()
	target := "wss" + strings.TrimPrefix(server.URL, "https")
	if err := prepareServiceConfig(clientConfig{SupervisorURL: target, EnrollToken: "synthetic-replacement-token"}, true, true); err != nil {
		t.Fatal(err)
	}
	stored, err := readStoredConfig(configPath())
	if err != nil || stored.SupervisorURL != previous.SupervisorURL || stored.DeviceID != previous.DeviceID || stored.SharedKey != previous.SharedKey {
		t.Fatal("pending enrollment overwrote the previous supervisor identity")
	}
	before, err := os.ReadFile(configPath())
	if err != nil {
		t.Fatal(err)
	}
	cfg, err := loadConfig("", "", "")
	if err != nil || cfg.SupervisorURL != target || cfg.DeviceID != "" || cfg.SharedKey != "" {
		t.Fatal("pending enrollment did not select the replacement in memory")
	}
	client := &Client{cfg: cfg, logger: slog.Default(), done: make(chan struct{})}
	if err := client.connect(); err == nil {
		t.Fatal("untrusted replacement supervisor accepted")
	}
	after, err := os.ReadFile(configPath())
	if err != nil || !bytes.Equal(before, after) {
		t.Fatal("failed enrollment changed the stored identity")
	}
}

func TestRootedDirectoryListingKeepsReusableAbsolutePaths(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "file.txt")
	if err := os.WriteFile(path, []byte("listed content"), 0600); err != nil {
		t.Fatal(err)
	}
	executor := NewExecutor(slog.Default(), 1)
	result := executor.Execute(remote.CommandPayload{Operation: remote.OpFileList, Args: map[string]interface{}{"path": dir}}, true, []string{dir})
	var entries []fileEntry
	if result.Status != "ok" || json.Unmarshal([]byte(result.Output), &entries) != nil || len(entries) != 1 || entries[0].Path != path {
		t.Fatalf("listing lost reusable absolute path: %s", result.Output)
	}
	read := executor.Execute(remote.CommandPayload{Operation: remote.OpFileRead, Args: map[string]interface{}{"path": entries[0].Path}}, true, []string{dir})
	if read.Status != "ok" {
		t.Fatalf("listed path cannot be read: %s", read.Error)
	}
}

func TestConcurrentRemoteEditsPreserveEachChange(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "shared.txt")
	if err := os.WriteFile(path, []byte("original"), 0600); err != nil {
		t.Fatal(err)
	}
	executor := NewExecutor(slog.Default(), 1)
	var wg sync.WaitGroup
	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			result := executor.Execute(remote.CommandPayload{CommandID: fmt.Sprint(i), Operation: remote.OpFileEdit, Args: map[string]interface{}{"path": path, "operation": "append", "content": fmt.Sprintf("marker-%d", i)}}, false, []string{dir})
			if result.Status != "ok" {
				t.Errorf("edit %d: %s", i, result.Error)
			}
		}(i)
	}
	wg.Wait()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 4; i++ {
		if !bytes.Contains(data, []byte(fmt.Sprintf("marker-%d", i))) {
			t.Fatalf("lost concurrent edit %d", i)
		}
	}
}
