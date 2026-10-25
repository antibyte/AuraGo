package selftest

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestSelfTestCatalog(t *testing.T) {
	catalog, err := Catalog()
	if err != nil {
		t.Fatal(err)
	}
	if len(catalog) != 320 {
		t.Fatalf("catalog has %d cases", len(catalog))
	}
	for _, source := range []string{"| FILE-01 | filesystem | R | an adequately long test prompt for parsing |\n| FILE-01 | filesystem | R | another adequately long prompt for parsing |", "| FILE-01 | filesystem | Q | an adequately long test prompt for parsing |"} {
		if _, err := ParseCatalog(source); err == nil {
			t.Fatal("invalid catalog accepted")
		}
	}
}

func waitSelfTest(t *testing.T, m *Manager, id string) Run {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		run, ok := m.Get(id)
		if ok && run.Status != "running" {
			return run
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("self-test did not stop")
	return Run{}
}

func TestSelfTestEvidenceAndCoverage(t *testing.T) {
	for _, evidence := range []bool{false, true} {
		t.Run(map[bool]string{false: "narration", true: "evidence"}[evidence], func(t *testing.T) {
			m, err := NewManager(t.TempDir(), func(_ context.Context, _ string, c Case, _ int, emit func(Evidence)) Execution {
				result := Execution{Verdict: "PASS", Assertions: []string{"typed_tool_execution_success"}, Tokens: 17}
				if evidence {
					e := Evidence{ID: "one", Tool: c.Tool, Operation: "metric/all", Status: "success", Executed: true}
					emit(e)
				}
				return result
			})
			if err != nil {
				t.Fatal(err)
			}
			defer m.Close()
			run, err := m.Start(context.Background(), Request{CaseIDs: []string{"SYS-01"}}, []Capability{{Name: "system_metrics", Enabled: true, Callable: true, Instance: "default", Operations: map[string][]string{"metric": {"all", "cpu"}}}})
			if err != nil {
				t.Fatal(err)
			}
			run = waitSelfTest(t, m, run.ID)
			if evidence {
				if run.Summary.Verdicts["PASS"] != 1 || run.Summary.ExecutedOperations != 1 || run.Summary.EnabledOperations != 2 {
					t.Fatalf("wrong coverage: %+v", run.Summary)
				}
			} else if run.Summary.Verdicts["PASS"] != 0 || run.Summary.Verdicts["INCONCLUSIVE"] != 1 {
				t.Fatalf("prose became success: %+v", run.Summary)
			}
		})
	}
}

func TestSelfTestCancelBusyAndRecovery(t *testing.T) {
	dir := t.TempDir()
	entered := make(chan struct{})
	m, err := NewManager(dir, func(ctx context.Context, _ string, _ Case, _ int, _ func(Evidence)) Execution {
		close(entered)
		<-ctx.Done()
		return Execution{Verdict: "CANCELLED"}
	})
	if err != nil {
		t.Fatal(err)
	}
	cap := []Capability{{Name: "system_metrics", Enabled: true, Callable: true, Instance: "default"}}
	run, err := m.Start(context.Background(), Request{CaseIDs: []string{"SYS-01"}}, cap)
	if err != nil {
		t.Fatal(err)
	}
	<-entered
	if _, err := m.Start(context.Background(), Request{}, cap); err != ErrBusy {
		t.Fatalf("overlap: %v", err)
	}
	if !m.Cancel(run.ID) {
		t.Fatal("cancel not accepted")
	}
	run = waitSelfTest(t, m, run.ID)
	if run.Status != "cancelled" {
		t.Fatalf("status %s", run.Status)
	}
	m.Close()
	run.Status = "running"
	run.Cases[0].Verdict = "RUNNING"
	run.Cases[0].Cleanup = "pending"
	b, _ := json.Marshal(run)
	if err := os.WriteFile(filepath.Join(dir, run.ID+".json"), b, 0600); err != nil {
		t.Fatal(err)
	}
	m, err = NewManager(dir, func(context.Context, string, Case, int, func(Evidence)) Execution {
		t.Error("interrupted case replayed")
		return Execution{}
	})
	if err != nil {
		t.Fatal(err)
	}
	defer m.Close()
	restored, _ := m.Get(run.ID)
	if restored.Status != "interrupted" || restored.Cases[0].Verdict != "UNKNOWN" {
		t.Fatalf("recovery: %+v", restored.Cases[0])
	}
}

func TestSelfTestReportEscapingAndNoHiddenSuccess(t *testing.T) {
	run := Run{ID: strings.Repeat("a", 32), Status: "complete", Cases: []Case{{ID: "SYS-01", Tool: "<script>|[bad]", Instance: "default", Verdict: "BLOCKED_FIXTURE", Cleanup: "failed", Code: "missing"}}}
	text := Markdown(run)
	if strings.Contains(text, "<script>") || !strings.Contains(text, "BLOCKED_FIXTURE") || !strings.Contains(text, "failed") {
		t.Fatalf("unsafe or incomplete report: %s", text)
	}
}

func TestSelfTestBindingsStayInMemory(t *testing.T) {
	dir := t.TempDir()
	observed := false
	m, err := NewManager(dir, func(ctx context.Context, _ string, c Case, _ int, emit func(Evidence)) Execution {
		b, ok := BindingFromContext(ctx)
		observed = ok && b.Arguments["metric"] == "PRIVATE_CANARY"
		emit(Evidence{ID: "bound", Tool: c.Tool, Status: "success"})
		return Execution{Verdict: "PASS", Assertions: []string{"confirmed"}}
	})
	if err != nil {
		t.Fatal(err)
	}
	defer m.Close()
	run, err := m.Start(context.Background(), Request{Profile: "targeted", Bindings: []Binding{{CaseID: "SYS-01", Tool: "system_metrics", Arguments: map[string]any{"metric": "PRIVATE_CANARY"}, AllowEffects: true, Expected: map[string]any{"status": "ok"}}}}, []Capability{{Name: "system_metrics", Kind: "native", Instance: "default", Enabled: true, Callable: true}})
	if err != nil {
		t.Fatal(err)
	}
	run = waitSelfTest(t, m, run.ID)
	if !observed {
		t.Fatal("binding did not reach executor")
	}
	b, err := os.ReadFile(filepath.Join(dir, run.ID+".json"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(b), "PRIVATE_CANARY") || strings.Contains(Markdown(run), "PRIVATE_CANARY") {
		t.Fatal("bound arguments persisted")
	}
}
