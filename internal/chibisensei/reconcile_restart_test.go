package chibisensei

import "testing"

var restartArtifactKinds = []string{"adapter", "adapter_config", "eval_samples", "gguf_base", "gguf_tuned", "metrics", "status"}

// At the entry of every non-terminal state a new Orchestrator (and a restarted Kaggle service) continues on the same
// store and data directory to succeeded, with exactly the admissions of an uninterrupted run and no replay.
func TestOrchestratorRestartAtEveryState(t *testing.T) {
	for _, state := range []string{RunQueued, RunPackaging, RunUploading, RunCreatingDataset, RunWaitingDataset,
		RunStarting, RunRunning, RunDownloading, RunVerifying, RunCleaning} {
		t.Run(state, func(t *testing.T) {
			fx := newRunFixture(t, fixtureOpts{profile: "qwen35-0p8b", base: true})
			if r := fx.drive(fx.untilState(state)); r.State != state {
				t.Fatalf("run reached %s before %s", r.State, state)
			}
			fx.restart()
			r := fx.drive(nil)
			fx.assertSucceeded(r, restartArtifactKinds)
			assertMutations(t, fx.f, fx.happyMutations(r))
			fx.assertNoReplays(r, true)
		})
	}
}

// A restart while a Kaggle job of the run is queued turns it unknown: the step resolves it (a read where the spec
// says so) and admits exactly one new attempt with a new ID; nothing is replayed. remote_running survives.
func TestOrchestratorRestartMidFlight(t *testing.T) {
	admitted := func(step string) func(Run) bool {
		return func(r Run) bool { rec := r.Remote.Step(step); return rec != nil && rec.JobID != "" }
	}
	cases := []struct {
		name, step, op string
		stop           func(Run) bool
	}{
		{"upload queued", StepUploadPrefix + "train", "upload_dataset_file", admitted(StepUploadPrefix + "train")},
		{"create_dataset queued", StepCreateDataset, "create_dataset", admitted(StepCreateDataset)},
		{"notebook running", StepSaveKernel, "", func(r Run) bool { return r.State == RunRunning && r.Remote.RemoteStatus == "RUNNING" }},
		{"status download queued", StepDownloadPrefix + OutStatus, "download_kernel_output", admitted(StepDownloadPrefix + OutStatus)},
		{"delete_dataset queued", StepDeleteDataset, "delete_dataset", admitted(StepDeleteDataset)},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			fx := newRunFixture(t, fixtureOpts{profile: "qwen35-0p8b", base: true})
			before := fx.drive(c.stop)
			first := before.Remote.Step(c.step).RequestID
			fx.restart()
			if c.op != "" {
				if j, ok := fx.f.PeekJob(first); !ok || j.Status != KaggleJobUnknown {
					t.Fatalf("job %s after restart = %+v", first, j)
				}
			}
			r := fx.drive(nil)
			fx.assertSucceeded(r, restartArtifactKinds)
			want := fx.happyMutations(r)
			if c.op != "" {
				want[c.op]++
				if got := r.Remote.Step(c.step).RequestID; got != KaggleRequestID(r.ID, c.step, 1) {
					t.Errorf("new attempt id = %s, want attempt 1", got)
				}
			}
			assertMutations(t, fx.f, want)
			fx.assertNoReplays(r, false)
		})
	}
}
