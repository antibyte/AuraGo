package flows

import (
	"context"
	"errors"
	"testing"
)

func TestRealTestRejectsRedactedAndTruncatedData(t *testing.T) {
	s, _ := newServiceFixture(t, nil)
	ctx := context.Background()
	rec, err := s.CreateFlow(ctx, CreateRequest{Import: simpleFlow("Test")})
	if err != nil {
		t.Fatal(err)
	}
	for _, data := range []map[string]any{
		{"nested": []any{map[string]any{"value": "[redacted]"}}},
		{"nested": map[string]any{"[redacted]": "value"}},
		{"_preview": "partial"}, {"nested": map[string]any{"_preview": "", "_omitted": storedOmittedReason}},
		{"nested": map[string]any{"_preview": "partial", "_truncated": true}},
	} {
		if _, err := s.StartTestRun(ctx, rec.ID, TestRunRequest{TriggerData: data, RememberData: true}); !errors.Is(err, ErrTestDataRedacted) {
			t.Fatalf("incomplete input accepted: %v", err)
		}
	}
	runs, err := s.Runs(ctx, rec.ID, RunFilter{})
	if err != nil || len(runs) != 0 {
		t.Fatalf("refused tests created runs: %v %v", runs, err)
	}
	for _, data := range []map[string]any{{"text": "a [redacted] label"}, {"_preview": "normal", "other": "value"}} {
		if err := rejectRedactedTestData(data); err != nil {
			t.Fatalf("ordinary data refused: %v", err)
		}
	}
	started, err := s.StartTestRun(ctx, rec.ID, TestRunRequest{})
	if err != nil {
		t.Fatalf("default sample refused: %v", err)
	}
	if got := waitRun(t, s, started.RunID); got.Status != RunSuccess {
		t.Fatalf("default sample run: %+v", got)
	}
}
