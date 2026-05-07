package server

import (
	"os"
	"strings"
	"testing"
)

func TestLooperRunHandlerUsesAtomicStart(t *testing.T) {
	t.Parallel()

	sourceBytes, err := os.ReadFile("desktop_looper_handlers.go")
	if err != nil {
		t.Fatalf("ReadFile desktop_looper_handlers.go: %v", err)
	}
	source := string(sourceBytes)
	if strings.Contains(source, "jsonError(w, \"A loop is already running\", http.StatusConflict)") {
		t.Fatal("handleLooperRun must not pre-check State before starting")
	}
	for _, marker := range []string{
		"runner.TryStart(req.MaxIter, loopCancel)",
		"http.StatusConflict",
		"runner.executeStarted(",
	} {
		if !strings.Contains(source, marker) {
			t.Fatalf("looper atomic start missing marker %q", marker)
		}
	}
}
