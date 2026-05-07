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
	for _, marker := range []string{
		"runner.TryStart(req.MaxIter, loopCancel)",
		"http.StatusConflict",
		"runner.executeStarted(",
		"looperRunTimeout(",
	} {
		if !strings.Contains(source, marker) {
			t.Fatalf("looper atomic start missing marker %q", marker)
		}
	}
}
