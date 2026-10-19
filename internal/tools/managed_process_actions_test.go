package tools

import (
	"context"
	"io"
	"log/slog"
	"strings"
	"testing"
	"time"
)

func TestManagedProcessStatusAndLabelStayInRegistry(t *testing.T) {
	registry := NewProcessRegistry(slog.New(slog.NewTextHandler(io.Discard, nil)))
	registry.Register(&ProcessInfo{PID: 424242, Alive: true, State: ProcessStateRunning, StartedAt: time.Now()})
	for _, selector := range []struct {
		pid   int
		label string
	}{{424242, ""}, {0, "process-424242"}} {
		output := ManageRegisteredProcesses(context.Background(), "status", selector.pid, selector.label, registry)
		if !strings.Contains(output, `"status":"success"`) || !strings.Contains(output, "424242") {
			t.Fatal(output)
		}
	}
	if result := ManageRegisteredProcesses(context.Background(), "kill", 1, "", registry); !strings.Contains(result, "not found") {
		t.Fatal("unmanaged PID reached signal path")
	}
}

func TestManagedProcessTerminationHonorsRevokedShellGrant(t *testing.T) {
	registry := NewProcessRegistry(slog.New(slog.NewTextHandler(io.Discard, nil)))
	registry.Register(&ProcessInfo{PID: 424242, Alive: true, State: ProcessStateRunning, StartedAt: time.Now()})
	ctx := WithRuntimePermissions(context.Background(), RuntimePermissions{AllowShell: false})
	output := ManageRegisteredProcesses(ctx, "kill", 424242, "", registry)
	if !strings.Contains(output, "disabled by the shell capability gate") {
		t.Fatalf("termination bypassed revoked shell grant: %s", output)
	}
	if output := ManageRegisteredProcesses(ctx, "status", 424242, "", registry); !strings.Contains(output, `"status":"success"`) {
		t.Fatalf("read-only status became unavailable: %s", output)
	}
}
