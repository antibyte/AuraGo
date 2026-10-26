package server

import (
	"strings"
	"testing"

	"aurago/internal/dbutil"
	"aurago/internal/flows"
)

// S7: the server installs its read-time scrubber in the flow store too.
func TestFB4FlowStoreScrubsRunDataAtWriteTime(t *testing.T) {
	s, token := newFlowsTestServer(t)
	const secret = "fb4-write-time-secret"
	c18Secret(t, secret)
	rec := createTestFlow(t, s, greetFlowJSON)
	runID := c18StartTestRun(t, s, token, rec, `{"trigger_data":{"name":"`+secret+`"}}`)
	c18WaitBacklog(t, s, runID, flows.EventRunFinished, "")
	db, err := dbutil.Open(s.flowsDBPath())
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	var steps, trigger string
	if err := db.QueryRow(`SELECT GROUP_CONCAT(params_json || output_json, '') FROM flow_run_steps WHERE run_id = ?`, runID).Scan(&steps); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(`SELECT trigger_data_json FROM flow_runs WHERE id = ?`, runID).Scan(&trigger); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(steps+trigger, secret) || !strings.Contains(steps, "[redacted]") {
		t.Fatalf("stored run data holds the secret: steps %q trigger %q", steps, trigger)
	}
}
