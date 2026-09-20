package agent

import (
	"context"
	"io"
	"log/slog"
	"strings"
	"testing"

	"aurago/internal/config"
)

func TestBlockedEnvReadReasonDistinguishesAccessFromResourceNames(t *testing.T) {
	tests := []struct {
		name    string
		command string
		want    string
	}{
		{
			name:    "reported docker mount inspection",
			command: `for c in aurago-store-olivetin aurago_ollama_managed; do echo "--- $c"; docker inspect $c --format '{{range .Mounts}}{{.Type}}:{{.Name}}{{.Source}} -> {{.Destination}}{{"\n"}}{{end}}'; done`,
		},
		{name: "literal variable name", command: `echo AURAGO_MASTER_KEY`},
		{name: "ordinary env filename", command: `cat backups/AURAGO_RUNTIME.env.example`},
		{name: "posix reference", command: `echo $AURAGO_MASTER_KEY`, want: "aurago_variable_reference"},
		{name: "posix braced reference", command: `printf '%s' "${AURAGO_MASTER_KEY}"`, want: "aurago_variable_reference"},
		{name: "powershell reference", command: `Write-Output $env:AURAGO_MASTER_KEY`, want: "aurago_variable_reference"},
		{name: "cmd reference", command: `cmd /c echo %AURAGO_MASTER_KEY%`, want: "aurago_variable_reference"},
		{name: "python getenv", command: `python -c "import os; print(os.getenv('AURAGO_MASTER_KEY'))"`, want: "aurago_environment_api"},
		{name: "python environ", command: `python -c "import os; print(os.environ['AURAGO_MASTER_KEY'])"`, want: "aurago_environment_api"},
		{name: "node process env", command: `node -e "console.log(process.env.AURAGO_MASTER_KEY)"`, want: "aurago_environment_api"},
		{name: "awk environ", command: `awk 'BEGIN { print ENVIRON["AURAGO_MASTER_KEY"] }'`, want: "aurago_environment_api"},
		{name: "env command", command: `env`, want: "environment_enumeration"},
		{name: "printenv pipeline", command: `printenv | sort`, want: "environment_enumeration"},
		{name: "chained env command", command: `echo ready; env | sort`, want: "environment_enumeration"},
		{name: "sudo env command", command: `sudo /usr/bin/env`, want: "environment_enumeration"},
		{name: "powershell env drive", command: `Get-ChildItem Env:`, want: "environment_enumeration"},
		{name: "self environ", command: `cat /proc/self/environ`, want: "process_environment_file"},
		{name: "pid environ", command: `strings /proc/1/environ`, want: "process_environment_file"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := blockedEnvReadReason(tt.command); got != tt.want {
				t.Fatalf("blockedEnvReadReason(%q) = %q, want %q", tt.command, got, tt.want)
			}
			if got := isBlockedEnvRead(tt.command); got != (tt.want != "") {
				t.Fatalf("isBlockedEnvRead(%q) = %t, want %t", tt.command, got, tt.want != "")
			}
		})
	}
}

func TestExecuteShellPublishesTrustedOutcome(t *testing.T) {
	cfg := &config.Config{}
	cfg.Agent.AllowShell = true
	cfg.Directories.WorkspaceDir = t.TempDir()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	dc := &DispatchContext{Cfg: cfg, Logger: logger}

	tests := []struct {
		name    string
		command string
		want    ToolResultStatus
	}{
		{name: "success", command: "echo shell-outcome-ok", want: ToolResultSuccess},
		{name: "failure", command: "exit 7", want: ToolResultFailed},
		{name: "denied", command: "echo $AURAGO_MASTER_KEY", want: ToolResultDenied},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tc := ToolCall{Action: "execute_shell", Params: map[string]interface{}{"command": tt.command}}
			result := DispatchToolCallResult(context.Background(), &tc, dc, "")
			if result.Status != tt.want {
				t.Fatalf("status = %q, want %q; output=%s", result.Status, tt.want, result.Output)
			}
			if tt.want == ToolResultSuccess && !strings.Contains(result.Output, "shell-outcome-ok") {
				t.Fatalf("successful output missing marker: %s", result.Output)
			}
		})
	}
}
