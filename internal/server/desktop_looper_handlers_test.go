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
		"runner.TryStart(req.MaxRounds, loopCancel)",
		"http.StatusConflict",
		"runner.executeStarted(",
		"looperRunTimeout(",
		"LLMClient:          client",
		"TryStartResume(",
		"buildLooperRuntime(",
		`json:"goal"`,
		"field %q is required",
		"handleLooperRuns",
		"desktopScopeRead",
		"desktopScopeAdmin",
	} {
		if !strings.Contains(source, marker) {
			t.Fatalf("looper atomic start missing marker %q", marker)
		}
	}
	if strings.Contains(source, "LLMClient:          s.LLMClient") {
		t.Fatal("looper run must not hard-wire default LLM client into DispatchContext")
	}
	if strings.Contains(source, "handleLooperExamples") {
		t.Fatal("legacy examples handler must be removed")
	}
	if strings.Contains(source, `{"prepare"`) {
		t.Fatal("run request must use v2 goal/work/evaluate fields")
	}
}

func TestLooperRunHandlerDetachesExecutionContextFromHTTPRequest(t *testing.T) {
	t.Parallel()

	sourceBytes, err := os.ReadFile("desktop_looper_handlers.go")
	if err != nil {
		t.Fatalf("ReadFile desktop_looper_handlers.go: %v", err)
	}
	source := string(sourceBytes)
	if !strings.Contains(source, "context.WithTimeout(context.Background(), looperRunTimeout(req.MaxRounds))") {
		t.Fatal("looper run context must outlive the short /run HTTP request")
	}
	if strings.Contains(source, "context.WithTimeout(r.Context(), looperRunTimeout(req.MaxRounds))") {
		t.Fatal("looper run context must not be derived from the /run request context")
	}
}

func TestLooperRoutesRegisterRunsNotExamples(t *testing.T) {
	t.Parallel()
	sourceBytes, err := os.ReadFile("server_routes.go")
	if err != nil {
		t.Fatalf("ReadFile server_routes.go: %v", err)
	}
	source := string(sourceBytes)
	if !strings.Contains(source, `"/api/desktop/looper/runs"`) || !strings.Contains(source, `"/api/desktop/looper/runs/"`) {
		t.Fatal("looper run history routes missing")
	}
	if strings.Contains(source, `"/api/desktop/looper/examples"`) {
		t.Fatal("legacy examples route must be removed")
	}
}
