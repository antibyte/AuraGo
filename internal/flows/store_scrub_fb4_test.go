package flows

import (
	"bytes"
	"context"
	"strings"
	"testing"
)

// S7: with a scrubber installed, trigger data, step params, outputs and error texts reach
// flows.db already redacted.
func TestFB4StoreScrubsRunDataBeforeWriting(t *testing.T) {
	s := openTestStore(t)
	const secret = "hunter2-secret"
	s.SetScrubber(Scrubber{
		JSON: func(data []byte) ([]byte, error) {
			return bytes.ReplaceAll(data, []byte(secret), []byte(redactedText)), nil
		},
		Text: func(text string) string { return strings.ReplaceAll(text, secret, redactedText) },
	})
	ctx := context.Background()
	f := createStoredFlow(t, s, "flow_aaaaaaaafd")
	rec := RunRecord{ID: "run_aaaaaaaafda", FlowID: f.ID, Mode: ModeLive, TriggerNode: f.Nodes[0].ID,
		TriggerData: map[string]any{"token": secret}, Status: RunRunning, StartedAt: storeNow}
	if err := s.CreateRun(ctx, rec, nil); err != nil {
		t.Fatal(err)
	}
	step := StepRecord{NodeID: f.Nodes[1].ID, Status: StepError, Params: map[string]any{"key": secret},
		Output: map[string]any{"echo": secret}, ErrorMessage: "refused " + secret}
	if err := s.SaveStep(ctx, rec.ID, 1, step); err != nil {
		t.Fatal(err)
	}
	if err := s.FinishRun(ctx, rec.ID, RunResult{Status: RunError, ErrorMessage: "failed: " + secret, FinishedAt: storeNow}); err != nil {
		t.Fatal(err)
	}
	var run, steps string
	if err := s.db.QueryRowContext(ctx, `SELECT trigger_data_json || error_message FROM flow_runs WHERE id = ?`, rec.ID).Scan(&run); err != nil {
		t.Fatal(err)
	}
	if err := s.db.QueryRowContext(ctx, `SELECT params_json || output_json || error_message FROM flow_run_steps WHERE run_id = ?`, rec.ID).Scan(&steps); err != nil {
		t.Fatal(err)
	}
	for name, raw := range map[string]string{"run": run, "step": steps} {
		if strings.Contains(raw, secret) || !strings.Contains(raw, redactedText) {
			t.Errorf("%s row = %q", name, raw)
		}
	}
}
