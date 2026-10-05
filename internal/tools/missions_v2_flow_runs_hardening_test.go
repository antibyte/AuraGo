package tools

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"
	"unicode/utf8"

	"aurago/internal/memory"
)

// c08Audit records mission audit events behind a mutex.
type c08Audit struct {
	mu     sync.Mutex
	events []memory.AuditEvent
}

func (a *c08Audit) record(ev memory.AuditEvent) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.events = append(a.events, ev)
	return nil
}

// event returns the audit event of a run with the given status.
func (a *c08Audit) event(t *testing.T, runID, status string) memory.AuditEvent {
	t.Helper()
	a.mu.Lock()
	defer a.mu.Unlock()
	for _, ev := range a.events {
		if ev.CorrelationID == runID && ev.Status == status {
			return ev
		}
	}
	t.Fatalf("no %s audit event for run %q in %+v", status, runID, a.events)
	return memory.AuditEvent{}
}

// c08ManagerWithHistory returns a manager with a history database, an audit recorder and
// fake flow hooks.
func c08ManagerWithHistory(t *testing.T) (*MissionManagerV2, *sql.DB, *c08Audit) {
	t.Helper()
	dir := tempSystemTaskDir(t)
	mm := NewMissionManagerV2(dir, nil)
	t.Cleanup(mm.Stop) // ends the flow event dispatcher
	hist, err := InitMissionHistoryDB(filepath.Join(dir, "history.db"))
	if err != nil {
		t.Fatalf("history db: %v", err)
	}
	t.Cleanup(func() { hist.Close() })
	mm.SetHistoryDB(hist)
	audit := &c08Audit{}
	mm.SetAuditRecorder(audit.record)
	mm.SetFlowHooks(newFakeFlowHooks())
	return mm, hist, audit
}

// c08AddPromptDependent adds an enabled prompt mission that waits for sourceID.
func c08AddPromptDependent(mm *MissionManagerV2, id, sourceID string) {
	mm.mu.Lock()
	defer mm.mu.Unlock()
	mm.missions[id] = &MissionV2{ID: id, Name: id, Prompt: "p", ExecutionType: ExecutionTriggered,
		TriggerType: TriggerMissionCompleted, TriggerConfig: &TriggerConfig{SourceMissionID: sourceID},
		Enabled: true, Priority: "medium", Status: MissionStatusIdle}
}

func c08FlowActive(mm *MissionManagerV2, id string) (int, bool) {
	mm.mu.RLock()
	defer mm.mu.RUnlock()
	n, ok := mm.flowActive[id]
	return n, ok
}

// Extra 1: a run of a flow whose mission was deleted meanwhile still completes its history
// entry and its audit record, under the name stored with the entry.
func TestFlowRunFinishedCompletesHistoryOfAGoneMission(t *testing.T) {
	for _, viaDelete := range []bool{false, true} {
		t.Run(fmt.Sprintf("generic_delete=%v", viaDelete), func(t *testing.T) {
			mm, hist, audit := c08ManagerWithHistory(t)
			id := publishTestFlow(t, mm, FlowTriggerSpec{NodeID: "n_aaaaaaaa", TriggerType: FlowTriggerManual})
			runA := mm.FlowRunStarted(id, "manual", `{}`)
			runB := mm.FlowRunStarted(id, "webhook", `{}`)
			if runA == "" || runB == "" {
				t.Fatalf("history ids = %q %q", runA, runB)
			}
			var err error
			if viaDelete {
				err = mm.Delete(id)
			} else {
				err = mm.DeleteFlowMission(id)
			}
			if err != nil {
				t.Fatalf("delete: %v", err)
			}

			mm.FlowRunFinished(id, runA, MissionResultSuccess, "Fertig.", map[string]any{"pdf": "a.pdf"})
			mm.FlowRunFinished(id, runB, MissionResultError, "FLOW_CANCELLED: the flow was deleted", nil)

			if run, err := GetMissionRun(hist, runA); err != nil || run.Status != "success" || run.Output != "Fertig." || run.CompletedAt == nil {
				t.Fatalf("history of the finished run = %+v, %v", run, err)
			}
			if run, err := GetMissionRun(hist, runB); err != nil || run.Status != "error" || !strings.Contains(run.ErrorMsg, "FLOW_CANCELLED") {
				t.Fatalf("history of the cancelled run = %+v, %v", run, err)
			}
			for runID, status := range map[string]string{runA: memory.AuditStatusSuccess, runB: memory.AuditStatusError} {
				ev := audit.event(t, runID, status)
				if ev.TargetID != id || ev.TargetName != "Morgenbericht" || !strings.Contains(ev.Summary, "Morgenbericht") {
					t.Fatalf("audit completion of %s = %+v", runID, ev)
				}
			}
			if _, ok := mm.Get(id); ok {
				t.Fatal("finishing a run brought the deleted mission back")
			}
			if _, ok := c08FlowActive(mm, id); ok {
				t.Fatal("finishing a run of a deleted mission left a running slot")
			}
			// Without a history entry, or for an unknown mission, nothing happens.
			mm.FlowRunFinished(id, "", MissionResultError, "x", nil)
			mm.FlowRunFinished("mission_missing", "", MissionResultError, "x", nil)
		})
	}
}

// Extra 1: without a history database or a stored name the audit record names the mission id.
func TestFlowRunFinishedNamesAGoneMissionByID(t *testing.T) {
	mm := NewMissionManagerV2(tempSystemTaskDir(t), nil)
	audit := &c08Audit{}
	mm.SetAuditRecorder(audit.record)
	mm.FlowRunFinished("mission_gone", "run_c08", MissionResultError, "kaputt", nil)
	ev := audit.event(t, "run_c08", memory.AuditStatusError)
	if ev.TargetID != "mission_gone" || !strings.Contains(ev.Summary, "mission_gone") {
		t.Fatalf("audit completion = %+v", ev)
	}
}

// Extra 2: a finish without a running slot changes no counter or status beyond clearing them,
// counts no run and fires no dependents; two starts and one finish keep the flow running.
func TestFlowRunFinishedWithoutStartKeepsTheCounter(t *testing.T) {
	mm, _, _ := newFlowTestManager(t)
	id := publishTestFlow(t, mm, FlowTriggerSpec{NodeID: "n_aaaaaaaa", TriggerType: FlowTriggerManual})
	c08AddPromptDependent(mm, "c08_dependent", id)

	mm.FlowRunFinished(id, "", MissionResultSuccess, "nie gestartet", nil)
	if n, ok := c08FlowActive(mm, id); ok || n != 0 {
		t.Fatalf("counter after an unstarted finish = %d (present %v)", n, ok)
	}
	if m, _ := mm.Get(id); m.Status != MissionStatusIdle || m.RunCount != 0 || m.LastResult != "" {
		t.Fatalf("flow mission after an unstarted finish = %+v", m)
	}
	if items := mm.queue.List(); len(items) != 0 {
		t.Fatalf("an unstarted finish fired dependents: %+v", items)
	}

	mm.FlowRunStarted(id, "manual", "")
	mm.FlowRunStarted(id, "manual", "")
	mm.FlowRunFinished(id, "", MissionResultSuccess, "eins", nil)
	if n, _ := c08FlowActive(mm, id); n != 1 {
		t.Fatalf("counter after two starts and one finish = %d", n)
	}
	if m, _ := mm.Get(id); m.Status != MissionStatusRunning || m.RunCount != 1 {
		t.Fatalf("flow mission after two starts and one finish = %+v", m)
	}
	mm.FlowRunFinished(id, "", MissionResultSuccess, "zwei", nil)
	mm.FlowRunFinished(id, "", MissionResultSuccess, "zu viel", nil)
	if n, ok := c08FlowActive(mm, id); ok || n != 0 {
		t.Fatalf("counter after the extra finish = %d (present %v)", n, ok)
	}
	if m, _ := mm.Get(id); m.Status != MissionStatusIdle || m.RunCount != 2 || m.LastOutput != "zwei" {
		t.Fatalf("flow mission after the extra finish = %+v", m)
	}

	// A running status without a running slot is cleared.
	mm.mu.Lock()
	mm.missions[id].Status = MissionStatusRunning
	mm.mu.Unlock()
	mm.FlowRunFinished(id, "", MissionResultSuccess, "x", nil)
	if m, _ := mm.Get(id); m.Status != MissionStatusIdle || m.RunCount != 2 {
		t.Fatalf("flow mission after clearing = %+v", m)
	}
}

// Extra 3: a 1 MiB outputs map reaches the dependents as a bounded preview, once encoded.
func TestMissionCompletedOutputsAreBounded(t *testing.T) {
	mm, hooks, _ := newFlowTestManager(t)
	source := publishTestFlow(t, mm, FlowTriggerSpec{NodeID: "n_aaaaaaaa", TriggerType: FlowTriggerManual})
	c08AddPromptDependent(mm, "c08_dependent", source)
	follower, err := mm.CreateFlowMission("flow_bbbbbbbbbb", "Folge")
	if err != nil {
		t.Fatal(err)
	}
	if err := mm.SyncFlowMission(follower, "Folge", []FlowTriggerSpec{{NodeID: "n_bbbbbbbb", TriggerType: TriggerMissionCompleted,
		TriggerConfig: &TriggerConfig{SourceMissionID: source}}}); err != nil {
		t.Fatal(err)
	}
	if err := mm.SetFlowMissionEnabled(follower, true); err != nil {
		t.Fatal(err)
	}

	big := map[string]any{"report": strings.Repeat("é", 1<<19)} // 1 MiB of UTF-8
	run := mm.FlowRunStarted(source, "manual", "")
	mm.FlowRunFinished(source, run, MissionResultSuccess, "Fertig.", big)

	checkBounded := func(what, raw string) {
		t.Helper()
		if len(raw) >= 70<<10 {
			t.Fatalf("%s: trigger data has %d bytes", what, len(raw))
		}
		var data struct {
			SourceMission string         `json:"source_mission"`
			Output        string         `json:"output"`
			Outputs       map[string]any `json:"outputs"`
		}
		if err := json.Unmarshal([]byte(raw), &data); err != nil {
			t.Fatalf("%s: %v", what, err)
		}
		preview, _ := data.Outputs["_preview"].(string)
		if data.SourceMission != source || data.Output != "Fertig." || data.Outputs["_truncated"] != true ||
			len(preview) > flowCompletionOutputsPreviewBytes || !utf8.ValidString(preview) || !strings.HasPrefix(preview, `{"report":"éé`) {
			t.Fatalf("%s: source %q output %q outputs keys %d preview %d bytes", what, data.SourceMission, data.Output, len(data.Outputs), len(preview))
		}
	}
	items := mm.queue.List()
	if len(items) != 1 || items[0].MissionID != "c08_dependent" {
		t.Fatalf("queued dependents = %d", len(items))
	}
	checkBounded("prompt dependent", items[0].TriggerData)
	c := hooks.waitStart(t)
	if c.missionID != follower {
		t.Fatalf("follower start = %+v", c.missionID)
	}
	checkBounded("follower flow", c.data)
}

// Extra 3: outputs within the bound reach the dependents whole.
func TestMissionCompletedOutputsWithinTheBoundStayWhole(t *testing.T) {
	mm, _, _ := newFlowTestManager(t)
	source := publishTestFlow(t, mm, FlowTriggerSpec{NodeID: "n_aaaaaaaa", TriggerType: FlowTriggerManual})
	c08AddPromptDependent(mm, "c08_dependent", source)
	report := strings.Repeat("a", flowCompletionOutputsMaxBytes-len(`{"report":""}`))
	run := mm.FlowRunStarted(source, "manual", "")
	mm.FlowRunFinished(source, run, MissionResultSuccess, "Fertig.", map[string]any{"report": report})
	items := mm.queue.List()
	if len(items) != 1 {
		t.Fatalf("queued dependents = %d", len(items))
	}
	var data struct {
		Outputs map[string]any `json:"outputs"`
	}
	if err := json.Unmarshal([]byte(items[0].TriggerData), &data); err != nil {
		t.Fatal(err)
	}
	if data.Outputs["report"] != report || data.Outputs["_truncated"] != nil {
		t.Fatalf("outputs at the bound were changed: %d keys", len(data.Outputs))
	}
}

// Extra 4: the history and the audit start keep at most 16 KiB of a flow's trigger data, cut
// at a rune boundary and marked.
func TestFlowRunHistoryBoundsTriggerData(t *testing.T) {
	mm, hist, audit := c08ManagerWithHistory(t)
	id := publishTestFlow(t, mm, FlowTriggerSpec{NodeID: "n_aaaaaaaa", TriggerType: FlowTriggerManual})
	keep := flowHistoryTriggerDataMaxBytes - len(flowHistoryTruncatedMarker)
	// "€" (3 bytes) straddles the cut, so the cut steps back before it.
	data := strings.Repeat("a", keep-1) + "€" + strings.Repeat("b", 200<<10)
	run := mm.FlowRunStarted(id, "webhook", data)
	row, err := GetMissionRun(hist, run)
	if err != nil {
		t.Fatal(err)
	}
	want := strings.Repeat("a", keep-1) + flowHistoryTruncatedMarker
	if row.TriggerData != want || len(row.TriggerData) > flowHistoryTriggerDataMaxBytes || !utf8.ValidString(row.TriggerData) {
		t.Fatalf("history trigger data: %d bytes, suffix %q", len(row.TriggerData), row.TriggerData[max(0, len(row.TriggerData)-20):])
	}
	var meta struct {
		TriggerData string `json:"trigger_data"`
	}
	if err := json.Unmarshal([]byte(audit.event(t, run, memory.AuditStatusRunning).MetadataJSON), &meta); err != nil {
		t.Fatal(err)
	}
	if meta.TriggerData != want {
		t.Fatalf("audit start trigger data: %d bytes", len(meta.TriggerData))
	}

	exact := strings.Repeat("c", flowHistoryTriggerDataMaxBytes)
	run = mm.FlowRunStarted(id, "webhook", exact)
	if row, err := GetMissionRun(hist, run); err != nil || row.TriggerData != exact {
		t.Fatalf("trigger data at the cap was changed: %v", err)
	}
}

// Extra 5: a finish without queued prompt dependents does not rewrite the queue file; the
// missions file is still saved.
func TestFlowRunFinishedSavesTheQueueOnlyForQueuedDependents(t *testing.T) {
	mm, _, _ := newFlowTestManager(t)
	id := publishTestFlow(t, mm, FlowTriggerSpec{NodeID: "n_aaaaaaaa", TriggerType: FlowTriggerManual})
	if _, err := os.Stat(mm.queueFile); !os.IsNotExist(err) {
		t.Fatalf("setup wrote the queue file: %v", err)
	}
	run := mm.FlowRunStarted(id, "manual", "")
	mm.FlowRunFinished(id, run, MissionResultSuccess, "Fertig.", map[string]any{"x": 1})
	if _, err := os.Stat(mm.queueFile); !os.IsNotExist(err) {
		t.Fatalf("a finish without dependents wrote the queue file: %v", err)
	}
	saved, err := os.ReadFile(mm.file)
	if err != nil || !strings.Contains(string(saved), `"run_count": 1`) {
		t.Fatalf("the missions file lacks the finished run: %v", err)
	}

	c08AddPromptDependent(mm, "c08_dependent", id)
	run = mm.FlowRunStarted(id, "manual", "")
	mm.FlowRunFinished(id, run, MissionResultSuccess, "Fertig.", nil)
	queue, err := os.ReadFile(mm.queueFile)
	if err != nil || !strings.Contains(string(queue), "c08_dependent") {
		t.Fatalf("a queued dependent was not persisted: %v", err)
	}
}

// c08ReentrantHooks calls back into the manager from every hook, with a read lock (List,
// Get) and the write lock (SetCompletionCallback), and then reports the call.
type c08ReentrantHooks struct {
	mm    *MissionManagerV2
	calls chan string
}

func (h *c08ReentrantHooks) reenter(call, missionID string) {
	h.mm.List()
	h.mm.Get(missionID)
	h.mm.SetCompletionCallback(nil) // takes the write lock; the manager has no callback
	h.calls <- call + ":" + missionID
}

func (h *c08ReentrantHooks) StartFlowRun(missionID, _, triggerType, _ string) error {
	h.reenter("start/"+triggerType, missionID)
	return nil
}

func (h *c08ReentrantHooks) FlowMissionDeleted(missionID string) { h.reenter("deleted", missionID) }

func (h *c08ReentrantHooks) FlowEnabledChanged(missionID string, enabled bool) {
	h.reenter(fmt.Sprintf("enabled=%v", enabled), missionID)
}

func (h *c08ReentrantHooks) NextFlowRun(missionID string) (time.Time, bool) {
	h.reenter("next", missionID)
	return time.Time{}, false
}

func (h *c08ReentrantHooks) await(t *testing.T, want string) {
	t.Helper()
	select {
	case got := <-h.calls:
		if got != want {
			t.Fatalf("hook call %q, want %q", got, want)
		}
	case <-time.After(c07DeadlockGuard):
		t.Fatalf("no hook call %q within %s: deadlock", want, c07DeadlockGuard)
	}
}

// Extra 7: every FlowHooks call of the new paths happens without m.mu held, so hooks may call
// back into the manager.
func TestFlowHooksMayReenterFromRunsNextRunAndMissionControl(t *testing.T) {
	mm := NewMissionManagerV2(tempSystemTaskDir(t), nil)
	hooks := &c08ReentrantHooks{mm: mm, calls: make(chan string, 16)}
	mm.SetFlowHooks(hooks)
	source := publishTestFlow(t, mm, FlowTriggerSpec{NodeID: "n_aaaaaaaa", TriggerType: FlowTriggerManual})
	follower, err := mm.CreateFlowMission("flow_bbbbbbbbbb", "Folge")
	if err != nil {
		t.Fatal(err)
	}
	if err := mm.SyncFlowMission(follower, "Folge", []FlowTriggerSpec{{NodeID: "n_bbbbbbbb", TriggerType: TriggerMissionCompleted,
		TriggerConfig: &TriggerConfig{SourceMissionID: source}}}); err != nil {
		t.Fatal(err)
	}
	if err := mm.SetFlowMissionEnabled(follower, true); err != nil {
		t.Fatal(err)
	}

	c07Within(t, "RunNow", func() {
		if err := mm.RunNow(source); err != nil {
			t.Errorf("RunNow: %v", err)
		}
	})
	hooks.await(t, "start/manual:"+source)
	c07Within(t, "TriggerMission", func() {
		if err := mm.TriggerMission(source, "api", `{}`); err != nil {
			t.Errorf("TriggerMission: %v", err)
		}
	})
	hooks.await(t, "start/api:"+source)
	c07Within(t, "NextRun", func() { mm.NextRun(source) })
	hooks.await(t, "next:"+source)
	c07Within(t, "FlowRunFinished", func() {
		run := mm.FlowRunStarted(source, "manual", "")
		mm.FlowRunFinished(source, run, MissionResultSuccess, "Fertig.", map[string]any{"x": 1})
	})
	hooks.await(t, "start/mission_completed:"+follower)
	m, _ := mm.Get(source)
	m.Enabled = false
	c07Within(t, "Update", func() {
		if err := mm.Update(source, m); err != nil {
			t.Errorf("Update: %v", err)
		}
	})
	hooks.await(t, "enabled=false:"+source)
	c07Within(t, "Delete", func() {
		if err := mm.Delete(source); err != nil {
			t.Errorf("Delete: %v", err)
		}
	})
	hooks.await(t, "deleted:"+source)
}

// Extra 8: remote mission sync never installs or overwrites a flow mission, Update never turns
// a prompt mission into one, and prompt missions never keep flow fields.
func TestFlowMissionsRefuseSyncAndConversion(t *testing.T) {
	mm, _, _ := newFlowTestManager(t)
	id := publishTestFlow(t, mm, FlowTriggerSpec{NodeID: "n_aaaaaaaa", TriggerType: FlowTriggerManual})
	flowIntact := func(step string) {
		t.Helper()
		got, ok := mm.Get(id)
		if !ok || got.ExecutionType != ExecutionFlow || got.Name != "Morgenbericht" || len(got.FlowTriggers) != 1 ||
			!got.FlowPublished || got.SyncedFromMaster || !got.Enabled {
			t.Fatalf("%s: flow mission = %+v", step, got)
		}
	}

	err := mm.ApplySyncedMission(&MissionV2{ID: id, Name: "Vom Master", Prompt: "p", ExecutionType: ExecutionManual, Enabled: true})
	if !errors.Is(err, ErrFlowMissionManaged) {
		t.Fatalf("syncing over a flow mission = %v", err)
	}
	flowIntact("sync")

	err = mm.ApplySyncedMission(&MissionV2{ID: "c08_synced_flow", ExecutionType: ExecutionFlow, FlowID: "flow_cccccccccc", Enabled: true})
	if !errors.Is(err, ErrFlowMissionManaged) {
		t.Fatalf("syncing a flow mission = %v", err)
	}
	if _, ok := mm.Get("c08_synced_flow"); ok {
		t.Fatal("a master installed a flow mission")
	}

	stray := []FlowTriggerSpec{{NodeID: "n_aaaaaaaa", TriggerType: FlowTriggerManual}}
	if err := mm.ApplySyncedMission(&MissionV2{ID: "c08_synced", Name: "S", Prompt: "p", ExecutionType: ExecutionManual,
		FlowID: "flow_cccccccccc", FlowTriggers: stray, FlowPublished: true}); err != nil {
		t.Fatalf("ApplySyncedMission: %v", err)
	}
	if got, _ := mm.Get("c08_synced"); got.FlowID != "" || got.FlowTriggers != nil || got.FlowPublished {
		t.Fatalf("a synced prompt mission kept flow fields: %+v", got)
	}

	if err := mm.Create(&MissionV2{ID: "c08_prompt", Name: "Prompt", Prompt: "p", ExecutionType: ExecutionManual}); err != nil {
		t.Fatal(err)
	}
	p, _ := mm.Get("c08_prompt")
	p.ExecutionType, p.FlowID, p.FlowTriggers, p.FlowPublished = ExecutionFlow, "flow_cccccccccc", stray, true
	if err := mm.Update("c08_prompt", p); !errors.Is(err, ErrFlowMissionManaged) {
		t.Fatalf("turning a prompt mission into a flow mission = %v", err)
	}
	if got, _ := mm.Get("c08_prompt"); got.ExecutionType != ExecutionManual || got.FlowID != "" {
		t.Fatalf("the refused update changed the prompt mission: %+v", got)
	}
	p.ExecutionType, p.Name = ExecutionManual, "Prompt 2"
	if err := mm.Update("c08_prompt", p); err != nil {
		t.Fatalf("Update: %v", err)
	}
	if got, _ := mm.Get("c08_prompt"); got.Name != "Prompt 2" || got.FlowID != "" || got.FlowTriggers != nil || got.FlowPublished {
		t.Fatalf("a prompt mission kept flow fields: %+v", got)
	}

	// Update of the flow mission itself: an unchanged (empty) execution type is accepted.
	m, _ := mm.Get(id)
	m.ExecutionType, m.Name = "", "Anders"
	if err := mm.Update(id, m); err != nil {
		t.Fatalf("Update without an execution type: %v", err)
	}
	flowIntact("update")
}

// Extra 9: NextRun of a flow uses only its own "flow" cron jobs, and Delete removes those jobs
// (and no prompt mission's job) and tells the flow service.
func TestFlowNextRunAndDeleteUseOnlyFlowCronJobs(t *testing.T) {
	dir := tempSystemTaskDir(t)
	cronMgr := NewCronManager(dir)
	if err := cronMgr.Start(func(string) {}); err != nil {
		t.Fatalf("cron start: %v", err)
	}
	t.Cleanup(func() { _ = cronMgr.Close() })
	mm := NewMissionManagerV2(dir, cronMgr)
	hooks := newFakeFlowHooks()
	mm.SetFlowHooks(hooks)
	yearly := FlowTriggerSpec{NodeID: "n_aaaaaaaa", TriggerType: FlowTriggerSchedule, Schedule: "0 0 1 1 *"}
	id := publishTestFlow(t, mm, yearly)
	jobID := flowCronJobID(id, "n_aaaaaaaa")
	if job, ok := c07CronJob(cronMgr, jobID); !ok || job.Source != flowCronSource {
		t.Fatalf("flow cron job = %+v (found %v)", job, ok)
	}
	want, ok := cronMgr.NextRun(jobID)
	if !ok {
		t.Fatal("the cron engine has no next run for the flow job")
	}
	if got, ok := mm.NextRun(id); !ok || !got.Equal(want) {
		t.Fatalf("NextRun = %v %v, want %v", got, ok, want)
	}

	// A prompt mission owns the job id of another schedule node of the flow.
	clashID := id + flowCronSeparator + "n_bbbbbbbb"
	if err := mm.Create(&MissionV2{ID: clashID, Name: clashID, Prompt: "p", ExecutionType: ExecutionScheduled, Schedule: "* * * * *"}); err != nil {
		t.Fatal(err)
	}
	err := mm.SyncFlowMission(id, "Morgenbericht", []FlowTriggerSpec{yearly,
		{NodeID: "n_bbbbbbbb", TriggerType: FlowTriggerSchedule, Schedule: "0 6 * * *"}})
	if err == nil || !strings.Contains(err.Error(), "belongs to another scheduler entry") {
		t.Fatalf("a flow schedule on a prompt job id = %v", err)
	}
	if promptNext, ok := cronMgr.NextRun("mission_" + clashID); !ok || !promptNext.Before(want) {
		t.Fatalf("setup: prompt job next run %v %v", promptNext, ok)
	}
	if got, ok := mm.NextRun(id); !ok || !got.Equal(want) {
		t.Fatalf("NextRun with a prompt job on a flow job id = %v %v, want %v", got, ok, want)
	}

	if err := mm.Delete(id); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if hasCronJob(cronMgr, jobID) {
		t.Fatal("Delete left the flow's cron job")
	}
	if job, ok := c07CronJob(cronMgr, "mission_"+clashID); !ok || job.Source != "mission" {
		t.Fatalf("Delete touched the prompt mission's job: %+v (found %v)", job, ok)
	}
	eventually(t, "FlowMissionDeleted", func() bool {
		hooks.mu.Lock()
		defer hooks.mu.Unlock()
		return len(hooks.deleted) == 1 && hooks.deleted[0] == id
	})
}

// c08BlockingOutput is a flow output whose encoding reports its start and then waits for
// release.
type c08BlockingOutput struct {
	once    sync.Once
	entered chan struct{}
	release chan struct{}
}

func (b *c08BlockingOutput) MarshalJSON() ([]byte, error) {
	b.once.Do(func() { close(b.entered) })
	<-b.release
	return []byte(`"encoded"`), nil
}

// Review M1: FlowRunFinished encodes the outputs (up to 32 MiB) before it takes the manager
// lock, so Mission Control reads and writes go on during the encoding.
func TestFlowRunFinishedEncodesOutputsOutsideTheLock(t *testing.T) {
	mm, _, _ := newFlowTestManager(t)
	id := publishTestFlow(t, mm, FlowTriggerSpec{NodeID: "n_aaaaaaaa", TriggerType: FlowTriggerManual})
	c08AddPromptDependent(mm, "c08_dependent", id)
	slow := &c08BlockingOutput{entered: make(chan struct{}), release: make(chan struct{})}
	var releaseOnce sync.Once
	release := func() { releaseOnce.Do(func() { close(slow.release) }) }
	t.Cleanup(release)

	run := mm.FlowRunStarted(id, "manual", "")
	finished := make(chan struct{})
	go func() {
		defer close(finished)
		mm.FlowRunFinished(id, run, MissionResultSuccess, "Fertig.", map[string]any{"slow": slow})
	}()
	select {
	case <-slow.entered:
	case <-time.After(c07DeadlockGuard):
		t.Fatal("the outputs were never encoded")
	}
	c07Within(t, "List, Get and SetCompletionCallback while the outputs are encoded", func() {
		mm.List()
		if _, ok := mm.Get(id); !ok {
			t.Errorf("Get(%s) found no mission", id)
		}
		mm.SetCompletionCallback(nil) // the write lock
	})
	release()
	select {
	case <-finished:
	case <-time.After(c07DeadlockGuard):
		t.Fatal("FlowRunFinished did not return after the encoding")
	}
	items := mm.queue.List()
	if len(items) != 1 || !strings.Contains(items[0].TriggerData, `"outputs":{"slow":"encoded"}`) {
		t.Fatalf("queued dependents = %+v", items)
	}
	if m, _ := mm.Get(id); m.RunCount != 1 || m.Status != MissionStatusIdle {
		t.Fatalf("flow mission = %+v", m)
	}
}

// Review M2: the output caps of mission_completed and of a flow's LastOutput cut at a rune
// boundary and end with a marker.
func TestCompletionOutputCapsAreRuneSafe(t *testing.T) {
	long := strings.Repeat("€", 700) // 2100 bytes
	mm, _, _ := newFlowTestManager(t)
	source := publishTestFlow(t, mm, FlowTriggerSpec{NodeID: "n_aaaaaaaa", TriggerType: FlowTriggerManual})
	c08AddPromptDependent(mm, "c08_flow_dependent", source)
	mm.mu.Lock()
	mm.missions["c08_prompt_source"] = &MissionV2{ID: "c08_prompt_source", Name: "Src", Prompt: "p", ExecutionType: ExecutionManual,
		Enabled: true, Priority: "medium", Status: MissionStatusRunning}
	mm.mu.Unlock()
	c08AddPromptDependent(mm, "c08_prompt_dependent", "c08_prompt_source")

	run := mm.FlowRunStarted(source, "manual", "")
	mm.FlowRunFinished(source, run, MissionResultSuccess, long, nil)
	mm.OnMissionComplete("c08_prompt_source", MissionResultSuccess, long)

	expect := func(limit int) string {
		return strings.Repeat("€", (limit-len(completionTruncatedMarker))/len("€")) + completionTruncatedMarker
	}
	check := func(what, got string, limit int) {
		t.Helper()
		if got != expect(limit) || len(got) > limit || !utf8.ValidString(got) || strings.ContainsRune(got, utf8.RuneError) {
			t.Fatalf("%s: %d bytes, valid %v, tail %q", what, len(got), utf8.ValidString(got), got[max(0, len(got)-8):])
		}
	}
	items := mm.queue.List()
	if len(items) != 2 {
		t.Fatalf("queued dependents = %d", len(items))
	}
	for _, item := range items {
		var data struct {
			Output string `json:"output"`
		}
		if err := json.Unmarshal([]byte(item.TriggerData), &data); err != nil {
			t.Fatal(err)
		}
		check(item.MissionID+" output", data.Output, completionOutputMaxBytes)
	}
	m, _ := mm.Get(source)
	check("flow LastOutput", m.LastOutput, flowLastOutputMaxBytes)
}

// Review M3: Delete tells the flow service only after the mission's removal is saved. A failed
// save keeps the flow mission (in memory as on disk, with its cron job and running slot), so
// neither a restart nor a later save leaves a mission without its flow or a flow without its
// mission.
func TestDeleteNotifiesTheFlowServiceOnlyAfterTheSave(t *testing.T) {
	dir := tempSystemTaskDir(t)
	cronMgr := NewCronManager(dir)
	t.Cleanup(func() { _ = cronMgr.Close() })
	mm := NewMissionManagerV2(dir, cronMgr)
	t.Cleanup(mm.Stop)
	hooks := newFakeFlowHooks()
	mm.SetFlowHooks(hooks)
	failing := publishTestFlow(t, mm, FlowTriggerSpec{NodeID: "n_aaaaaaaa", TriggerType: FlowTriggerSchedule, Schedule: "0 7 * * *"})
	saved := publishTestFlow(t, mm, FlowTriggerSpec{NodeID: "n_aaaaaaaa", TriggerType: FlowTriggerManual})
	mm.FlowRunStarted(failing, "manual", "")
	deletedCalls := func() []string {
		hooks.mu.Lock()
		defer hooks.mu.Unlock()
		return append([]string(nil), hooks.deleted...)
	}

	// A directory where save writes its temporary file makes the save fail.
	blocker := mm.file + ".tmp"
	if err := os.Mkdir(blocker, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := mm.Delete(failing); err == nil {
		t.Fatal("Delete succeeded although the save failed")
	}
	if err := os.Remove(blocker); err != nil {
		t.Fatal(err)
	}
	if m, ok := mm.Get(failing); !ok || m.Status != MissionStatusRunning {
		t.Fatalf("the failed delete dropped the flow mission from memory: %+v", m)
	}
	if active, _ := c08FlowActive(mm, failing); active != 1 || !hasCronJob(cronMgr, flowCronJobID(failing, "n_aaaaaaaa")) {
		t.Fatalf("the failed delete lost the running slot (%d) or the cron job", active)
	}

	if err := mm.Delete(saved); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	// The saved delete's hook is launched after the failed delete returned; correct code never
	// launches one for the failed delete.
	eventually(t, "FlowMissionDeleted of the saved delete", func() bool { return len(deletedCalls()) > 0 })
	if deleted := deletedCalls(); len(deleted) != 1 || deleted[0] != saved {
		t.Fatalf("FlowMissionDeleted calls = %v, want only %s", deleted, saved)
	}
	persisted, err := os.ReadFile(mm.file)
	if err != nil || !strings.Contains(string(persisted), failing) || strings.Contains(string(persisted), saved) {
		t.Fatalf("the later save dropped the flow mission of the failed delete: %v", err)
	}

	if err := mm.Delete(failing); err != nil {
		t.Fatalf("retrying the delete: %v", err)
	}
	eventually(t, "FlowMissionDeleted of the retried delete", func() bool { return len(deletedCalls()) == 2 })
	if hasCronJob(cronMgr, flowCronJobID(failing, "n_aaaaaaaa")) {
		t.Fatal("the retried delete left the flow's cron job")
	}
}

// Review M4: a lock-only Update keeps the flow's trigger registrations (keyed MQTT is neither
// unregistered nor registered again); the enabled switch still re-syncs them.
func TestFlowLockToggleKeepsTheTriggerRegistrations(t *testing.T) {
	mm := NewMissionManagerV2(tempSystemTaskDir(t), nil)
	t.Cleanup(mm.Stop)
	mm.SetFlowHooks(newFakeFlowHooks())
	mqtt := &c07KeyedMQTT{}
	mm.SetMQTTManager(mqtt)
	id := publishTestFlow(t, mm, FlowTriggerSpec{NodeID: "n_aaaaaaaa", TriggerType: TriggerMQTTMessage,
		TriggerConfig: &TriggerConfig{MQTTTopic: "home/door", MQTTMinIntervalSeconds: 60}})
	registers, unregisters := mqtt.counts()
	if registers != 1 || unregisters != 0 {
		t.Fatalf("setup: %d registers, %d unregisters", registers, unregisters)
	}
	for _, locked := range []bool{true, false} {
		m, _ := mm.Get(id)
		m.Locked = locked
		if err := mm.Update(id, m); err != nil {
			t.Fatalf("Update(locked=%v): %v", locked, err)
		}
		if r, u := mqtt.counts(); r != registers || u != unregisters {
			t.Fatalf("locked=%v re-registered MQTT: %d/%d registers, %d/%d unregisters", locked, r, registers, u, unregisters)
		}
		if got, _ := mm.Get(id); got.Locked != locked {
			t.Fatalf("Locked = %v, want %v", got.Locked, locked)
		}
	}

	m, _ := mm.Get(id)
	m.Enabled = false
	if err := mm.Update(id, m); err != nil {
		t.Fatal(err)
	}
	if r, u := mqtt.counts(); r != registers || u != unregisters+1 || len(mqtt.registered()) != 0 {
		t.Fatalf("disabling: %d registers, %d unregisters, %d live", r, u, len(mqtt.registered()))
	}
	m.Enabled = true
	if err := mm.Update(id, m); err != nil {
		t.Fatal(err)
	}
	if r, _ := mqtt.counts(); r != registers+1 || len(mqtt.registered()) != 1 {
		t.Fatalf("enabling: %d registers, %d live", r, len(mqtt.registered()))
	}
}

// Review M5 (adopted probe): concurrent runs, Mission Control reads and lock toggles keep the
// running counter balanced.
func TestFlowConcurrentRunsStayBalanced(t *testing.T) {
	mm, _, _ := newFlowTestManager(t)
	id := publishTestFlow(t, mm, FlowTriggerSpec{NodeID: "n_aaaaaaaa", TriggerType: FlowTriggerManual})
	c08AddPromptDependent(mm, "c08_dependent", id)
	const n = 200
	stop := make(chan struct{})
	var readers sync.WaitGroup
	for r := 0; r < 4; r++ {
		readers.Add(1)
		go func(r int) {
			defer readers.Done()
			for {
				select {
				case <-stop:
					return
				default:
				}
				mm.List()
				mm.Get(id)
				mm.NextRun(id)
				if r == 0 {
					m, _ := mm.Get(id)
					m.Locked = !m.Locked
					_ = mm.Update(id, m)
				}
				// Keep the agent queue empty so the dependent is queued again.
				mm.mu.Lock()
				mm.queue.Remove("c08_dependent")
				mm.mu.Unlock()
			}
		}(r)
	}
	var runs sync.WaitGroup
	for i := 0; i < n; i++ {
		runs.Add(1)
		go func(i int) {
			defer runs.Done()
			run := mm.FlowRunStarted(id, "manual", "")
			result := MissionResultSuccess
			if i%3 == 0 {
				result = MissionResultError
			}
			mm.FlowRunFinished(id, run, result, fmt.Sprintf("out %d", i), map[string]any{"i": i})
		}(i)
	}
	done := make(chan struct{})
	go func() {
		runs.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(30 * time.Second):
		close(stop)
		t.Fatal("the runs did not finish within 30 s: deadlock")
	}
	close(stop)
	readers.Wait()
	m, _ := mm.Get(id)
	if active, ok := c08FlowActive(mm, id); ok || active != 0 || m.Status != MissionStatusIdle || m.RunCount != n {
		t.Fatalf("after %d runs: active=%d/%v status=%s runs=%d", n, active, ok, m.Status, m.RunCount)
	}
}

// c08OldDependentsLoop is the dependents loop of OnMissionComplete before 1c-08, copied
// verbatim, so the parity test compares against it.
func c08OldDependentsLoop(m *MissionManagerV2, missionID, result string) {
	for _, mission := range m.missions {
		if !mission.Enabled ||
			mission.ExecutionType != ExecutionTriggered ||
			mission.TriggerType != TriggerMissionCompleted {
			continue
		}
		cfg := mission.TriggerConfig
		if cfg == nil || cfg.SourceMissionID != missionID {
			continue
		}
		if cfg.RequireSuccess && result != MissionResultSuccess {
			continue
		}
		if !m.shouldFireTriggerLocked(mission, string(TriggerMissionCompleted), time.Now()) {
			continue
		}
		m.queue.Enqueue(mission.ID, mission.Priority, "mission_completed",
			fmt.Sprintf(`{"source_mission":"%s","result":"%s"}`, missionID, result))
		mission.Status = MissionStatusQueued
	}
}

// c08DependentsManager returns a manager with a running prompt source "src" and dependents
// that each hit one branch of the dependents loop.
func c08DependentsManager(t *testing.T) *MissionManagerV2 {
	t.Helper()
	mm := NewMissionManagerV2(tempSystemTaskDir(t), nil)
	add := func(m *MissionV2) {
		if m.Priority == "" {
			m.Priority = "medium"
		}
		if m.Status == "" {
			m.Status = MissionStatusIdle
		}
		mm.missions[m.ID] = m
	}
	add(&MissionV2{ID: "src", Name: "src", Prompt: "p", ExecutionType: ExecutionManual, Enabled: true, Status: MissionStatusRunning})
	triggered := func(id string, cfg *TriggerConfig, mod func(*MissionV2)) {
		m := &MissionV2{ID: id, Name: id, Prompt: "p", ExecutionType: ExecutionTriggered, TriggerType: TriggerMissionCompleted,
			TriggerConfig: cfg, Enabled: true}
		if mod != nil {
			mod(m)
		}
		add(m)
	}
	triggered("d_plain", &TriggerConfig{SourceMissionID: "src"}, nil)
	triggered("d_high", &TriggerConfig{SourceMissionID: "src"}, func(m *MissionV2) { m.Priority = "high" })
	triggered("d_reqsucc", &TriggerConfig{SourceMissionID: "src", RequireSuccess: true}, nil)
	triggered("d_disabled", &TriggerConfig{SourceMissionID: "src"}, func(m *MissionV2) { m.Enabled = false })
	triggered("d_other", &TriggerConfig{SourceMissionID: "other"}, nil)
	triggered("d_nocfg", nil, nil)
	triggered("d_rate_recent", &TriggerConfig{SourceMissionID: "src", MinIntervalSeconds: 3600}, nil)
	triggered("d_rate_fresh", &TriggerConfig{SourceMissionID: "src", MinIntervalSeconds: 3600}, nil)
	triggered("d_queued", &TriggerConfig{SourceMissionID: "src"}, nil)
	triggered("d_remote", &TriggerConfig{SourceMissionID: "src"}, func(m *MissionV2) { m.RunnerType = "remote"; m.RemoteNestID = "n" })
	add(&MissionV2{ID: "d_sched", Name: "d_sched", Prompt: "p", ExecutionType: ExecutionScheduled, TriggerType: TriggerMissionCompleted,
		TriggerConfig: &TriggerConfig{SourceMissionID: "src"}, Enabled: true})
	mm.lastTriggerFire["d_rate_recent|mission_completed"] = time.Now().Add(-time.Minute)
	mm.queue.Enqueue("d_queued", "low", "manual", "old")
	return mm
}

// c08DependentsSnapshot returns the queue items (without trigger data), the statuses and the
// min-interval slots. Items of the same priority follow map order, so they are sorted.
func c08DependentsSnapshot(mm *MissionManagerV2) string {
	var state struct {
		Items  []string
		Status map[string]string
		Fires  []string
	}
	for _, item := range mm.queue.List() {
		state.Items = append(state.Items, fmt.Sprintf("%s/%d/%s", item.MissionID, item.Priority, item.TriggerType))
	}
	sort.Strings(state.Items)
	state.Status = map[string]string{}
	for id, m := range mm.missions {
		state.Status[id] = m.Status
	}
	for key := range mm.lastTriggerFire {
		state.Fires = append(state.Fires, key)
	}
	sort.Strings(state.Fires)
	out, _ := json.Marshal(state)
	return string(out)
}

// Review M5 (adopted probe): for prompt missions the new dependents helper queues exactly
// what the old loop queued; only the trigger data payload differs.
func TestPromptDependentsMatchTheOldLoop(t *testing.T) {
	for _, result := range []string{MissionResultSuccess, MissionResultError} {
		oldMM, newMM := c08DependentsManager(t), c08DependentsManager(t)
		oldMM.mu.Lock()
		c08OldDependentsLoop(oldMM, "src", result)
		oldMM.mu.Unlock()
		newMM.mu.Lock()
		newMM.enqueueCompletionDependentsLocked("src", result, "out", nil)
		newMM.mu.Unlock()
		if before, after := c08DependentsSnapshot(oldMM), c08DependentsSnapshot(newMM); before != after {
			t.Fatalf("result %s:\nold %s\nnew %s", result, before, after)
		}
		for _, item := range newMM.queue.List() {
			if item.MissionID == "d_queued" && item.TriggerData != "old" {
				t.Fatalf("an already queued mission got new trigger data: %q", item.TriggerData)
			}
		}
	}
}

// Review M5 (adopted probe): a prompt mission's completion starts a flow that waits for it,
// with the output and without outputs; a flow mission is never queued by mission_completed.
func TestPromptSourceStartsFlowFollower(t *testing.T) {
	mm, hooks, _ := newFlowTestManager(t)
	mm.mu.Lock()
	mm.missions["src"] = &MissionV2{ID: "src", Name: "Src", Prompt: "p", ExecutionType: ExecutionManual, Enabled: true,
		Priority: "medium", Status: MissionStatusRunning}
	mm.mu.Unlock()
	follower, err := mm.CreateFlowMission("flow_bbbbbbbbbb", "Folge")
	if err != nil {
		t.Fatal(err)
	}
	if err := mm.SyncFlowMission(follower, "Folge", []FlowTriggerSpec{{NodeID: "n_bbbbbbbb", TriggerType: TriggerMissionCompleted,
		TriggerConfig: &TriggerConfig{SourceMissionID: "src"}}}); err != nil {
		t.Fatal(err)
	}
	if err := mm.SetFlowMissionEnabled(follower, true); err != nil {
		t.Fatal(err)
	}
	mm.OnMissionComplete("src", MissionResultSuccess, "Ergebnis")
	c := hooks.waitStart(t)
	if c.missionID != follower || !strings.Contains(c.data, `"output":"Ergebnis"`) || strings.Contains(c.data, `"outputs"`) {
		t.Fatalf("start = %+v", c)
	}
	if items := mm.queue.List(); len(items) != 0 {
		t.Fatalf("queue = %+v", items)
	}
	if m, _ := mm.Get(follower); m.Status != MissionStatusIdle {
		t.Fatalf("follower status = %s", m.Status)
	}
}
