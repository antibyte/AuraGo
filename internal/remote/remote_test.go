package remote

import (
	"context"
	"testing"
	"time"
)

func TestExecuteRemoteCommand_Timeout(t *testing.T) {
	// This test just ensures the function signature exists and compiles.
	// We use a short timeout and an unreachable address to trigger a quick error.
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	_, err := ExecuteRemoteCommand(ctx, "127.0.0.1", 2222, "user", []byte("pass"), "ls")
	if err == nil {
		t.Error("Expected error for unreachable host, got nil")
	}
}
