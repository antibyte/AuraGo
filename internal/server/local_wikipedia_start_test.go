package server

import (
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"aurago/internal/config"
	"aurago/internal/tools"
)

const localWikiStartSIPTestEnv = "AURAGO_LOCALWIKI_START_SIP_TEST"

// Start publishes the Local Wikipedia tool source while it builds the server
// (newServerFromOptions). When initSIP then fails, Start returns before its
// shutdown path is registered, so it must withdraw the source itself;
// otherwise the agent tool would keep a manager that is never started or
// shut down. Start changes process-wide state, so the check runs in its own
// process.
func TestStartWithdrawsTheLocalWikipediaToolWhenSIPFails(t *testing.T) {
	const name = "TestStartWithdrawsTheLocalWikipediaToolWhenSIPFailsProcess"
	cmd := exec.Command(os.Args[0], "-test.run=^"+name+"$", "-test.v", "-test.count=1")
	cmd.Env = append(os.Environ(), localWikiStartSIPTestEnv+"=1")
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("subprocess failed: %v\n%s", err, output)
	}
	if !strings.Contains(string(output), "--- PASS: "+name) {
		t.Fatalf("the subprocess did not run the check:\n%s", output)
	}
}

func TestStartWithdrawsTheLocalWikipediaToolWhenSIPFailsProcess(t *testing.T) {
	if os.Getenv(localWikiStartSIPTestEnv) != "1" {
		t.Skip("runs in an isolated process: Start changes process-wide state")
	}
	// A data directory that is a file makes the SIP call store fail to open.
	dataFile := filepath.Join(t.TempDir(), "data")
	if err := os.WriteFile(dataFile, []byte("not a directory"), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg := &config.Config{}
	cfg.Server.Host = "127.0.0.1" // loopback only: no remote-exposure refusal before initSIP
	cfg.Directories.DataDir = dataFile
	shutdown := make(chan struct{})
	defer close(shutdown)
	err := Start(StartOptions{Cfg: cfg, Logger: slog.Default(), AccessLogger: slog.Default(), ShutdownCh: shutdown})
	if err == nil || !strings.Contains(err.Error(), "initialize SIP") {
		t.Fatalf("Start = %v, want the SIP initialization error", err)
	}
	if m := tools.PublishedLocalWikipediaManager(); m != nil {
		t.Fatal("Start failed in initSIP but left its Local Wikipedia manager published to the agent tool")
	}
}
