package tools

import "testing"

func TestOwnsFlowCronJobNamesOnlyJobsOfFlowMissions(t *testing.T) {
	mm, _, _ := newFlowTestManager(t)
	flowID, err := mm.CreateFlowMission("flow_aaaaaaaaaa", "Bericht")
	if err != nil {
		t.Fatalf("CreateFlowMission: %v", err)
	}
	ordinary := &MissionV2{ID: "mission_ordinary", Name: "Alt", Prompt: "tu etwas", ExecutionType: ExecutionManual}
	if err := mm.Create(ordinary); err != nil {
		t.Fatalf("Create: %v", err)
	}

	for jobID, want := range map[string]bool{
		// The flow has no cron job yet; the id is still reserved.
		flowCronJobID(flowID, "n_aaaaaaaa"): true,
		flowCronJobID(flowID, "anything"):   true,
		// Everything else is an ordinary job id.
		"mission_" + flowID:                        false, // the (never registered) job of an ordinary mission
		flowCronJobID(ordinary.ID, "n_aaaaaaaa"):   false,
		flowCronJobID("mission_unknown", "n_aaaa"): false,
		"daily_report":                             false,
		flowID + "__n_aaaaaaaa":                    false, // no "mission_" prefix
		"":                                         false,
		"mission_":                                 false,
	} {
		if got := mm.OwnsFlowCronJob(jobID); got != want {
			t.Errorf("OwnsFlowCronJob(%q) = %v, want %v", jobID, got, want)
		}
	}

	if err := mm.DeleteFlowMission(flowID); err != nil {
		t.Fatalf("DeleteFlowMission: %v", err)
	}
	if mm.OwnsFlowCronJob(flowCronJobID(flowID, "n_aaaaaaaa")) {
		t.Error("a deleted flow no longer owns cron job ids")
	}
}

func TestCronJobIsFlowJobFollowsTheSource(t *testing.T) {
	for source, want := range map[string]bool{flowCronSource: true, "": false, "mission": false, "Flow": false} {
		if got := (CronJob{ID: "j", Source: source}).IsFlowJob(); got != want {
			t.Errorf("source %q: IsFlowJob = %v, want %v", source, got, want)
		}
	}
}
