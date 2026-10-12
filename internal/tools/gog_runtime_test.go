package tools

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"
)

func TestGogRuntimeCleansEnvironmentAndCancelsProcess(t *testing.T) {
	binary, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("CUSTOM_PASSWORD", "synthetic-parent-secret")
	t.Setenv("OPENAI_API_KEY", "synthetic-parent-key")
	ctx := WithRuntimePermissions(context.Background(), defaultRuntimePermissionsForTests())
	out, code, started, err := runGogProcess(ctx, binary, nil, t.TempDir(), []string{"AURAGO_FAKE_PYTHON=1"})
	if err != nil || !started || code != 0 || strings.TrimSpace(out) != "env-clean" {
		t.Fatalf("process boundary: started=%v code=%d err=%v", started, code, err)
	}
	ctx, cancel := context.WithTimeout(ctx, time.Second)
	defer cancel()
	start := time.Now()
	_, _, started, err = runGogProcess(ctx, binary, nil, t.TempDir(), []string{"AURAGO_FAKE_PYTHON=1", "AURAGO_FAKE_PYTHON_SLEEP_MS=10000"})
	if !started || err == nil || time.Since(start) > 6*time.Second {
		t.Fatalf("process cancellation failed: %v %v", started, err)
	}
	denied := WithRuntimePermissions(context.Background(), RuntimePermissions{AllowNetworkRequests: true})
	if _, _, started, err = runGogProcess(denied, binary, nil, t.TempDir(), nil); err == nil || started {
		t.Fatal("disabled shell started a process")
	}
}
