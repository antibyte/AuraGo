package chibisensei

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	"aurago/internal/chibisensei/trainer"
)

type fakeClock struct {
	mu  sync.Mutex
	now time.Time
}

func (c *fakeClock) Now() time.Time          { c.mu.Lock(); defer c.mu.Unlock(); return c.now }
func (c *fakeClock) Advance(d time.Duration) { c.mu.Lock(); c.now = c.now.Add(d); c.mu.Unlock() }

type fixtureOpts struct {
	profile string // qwen35-0p8b (LoRA, assistant) or gemma3-270m (full, classify)
	base    bool   // the plan asks for the base GGUF
	cfg     func(*OrchestratorConfig)
	outputs func(*fakeRunOutputs) // shape of the kernel outputs
	before  func(*runFixture)     // scripts; runs after SetOutputs and before Submit
}

type runFixture struct {
	t       *testing.T
	st      *Store
	hub     *Hub
	cat     *Catalog
	f       *fakeKaggle
	clk     *fakeClock
	dataDir string
	cfg     OrchestratorConfig
	o       *Orchestrator
	run     Run
	outputs map[string][]byte
	submit  SubmitInput // the inputs of the run, reused by a second run on the same store
	outOpts fakeRunOutputs
}

func newRunFixture(t *testing.T, opt fixtureOpts) *runFixture {
	t.Helper()
	ctx := context.Background()
	fx := &runFixture{t: t, st: newTestStore(t), hub: NewHub(), cat: mustCatalog(t), f: newFakeKaggle(t),
		clk: &fakeClock{now: time.Date(2026, 11, 23, 12, 0, 0, 0, time.UTC)}, dataDir: t.TempDir()}
	fx.cfg = OrchestratorConfig{MachineShape: "NvidiaTeslaT4", DeleteRemoteAfterRun: true, AllowDelete: true,
		RemoteTimeoutMinutes: 720, AuraGoVersion: "test"}
	if opt.cfg != nil {
		opt.cfg(&fx.cfg)
	}
	prof, ok := fx.cat.Get(opt.profile)
	if !ok {
		t.Fatalf("profile %s", opt.profile)
	}
	in := ProjectInput{Name: "Run", Recipe: RecipeAssistant, GoalText: "Answer shop questions.", ProfileID: prof.ID}
	if prof.ID == "gemma3-270m" {
		in.Recipe, in.TaskKind, in.Labels = RecipeSpecialist, "classify", []string{"pos", "neg"}
	}
	p, err := fx.st.CreateProject(ctx, in)
	if err != nil {
		t.Fatal(err)
	}
	rows := make([]RowInput, 80)
	for i := range rows {
		answer := fmt.Sprintf("The shop opens at %d o'clock on day %d.", 8+i%4, i)
		if in.TaskKind == "classify" {
			answer = in.Labels[i%2]
		}
		rows[i] = chatInput(fmt.Sprintf("Question %d: when does the shop open?", i), answer)
	}
	if _, err := fx.st.AddRows(ctx, p.ID, rows, AddOptions{Source: "manual", BatchKind: "add", MaxRows: 50000}); err != nil {
		t.Fatal(err)
	}
	srows, proj, err := fx.st.LoadSplitRows(ctx, p.ID)
	if err != nil {
		t.Fatal(err)
	}
	split := ComputeSplit(srows, proj.SplitSeed, proj.TaskKind == "classify")
	plan, err := ComputePlan(PlanInput{Project: proj, Profile: prof, Rows: srows, Split: split,
		RemoteTimeoutMinutes: 720, NeedBaseGGUF: opt.base})
	if err != nil {
		t.Fatal(err)
	}
	fx.newOrchestrator()
	runID := uuid.NewString()
	out := fakeRunOutputs{RunID: runID, ProfileID: prof.ID, ProfileSHA: fx.cat.SHA(prof.ID), Arch: prof.GGUF.ExpectedArch,
		Adapter: plan.Method == "lora", BaseGGUF: opt.base}
	if opt.outputs != nil {
		opt.outputs(&out)
	}
	fx.outputs = fakeOutputs(out)
	fx.f.SetOutputs(fx.outputs)
	if opt.before != nil {
		opt.before(fx)
	}
	fx.submit = SubmitInput{RunID: runID, Project: proj, Profile: prof, ProfileSHA: fx.cat.SHA(prof.ID), Plan: plan, Owner: "tester"}
	fx.outOpts = out
	fx.run, err = fx.o.Submit(ctx, fx.submit)
	if err != nil {
		t.Fatal(err)
	}
	return fx
}

func (fx *runFixture) newOrchestrator() {
	fx.o = NewOrchestrator(fx.st, fx.hub, fx.cat, fx.f, fx.dataDir, func() OrchestratorConfig { return fx.cfg })
	fx.o.now = fx.clk.Now
}

// restart drops the orchestrator (it keeps no goroutine and no run state) and restarts the Kaggle service, which
// turns queued and running jobs into unknown as AuraGo's restart does. Store and data directory stay.
func (fx *runFixture) restart() {
	fx.o = nil
	fx.f.RestartService()
	fx.newOrchestrator()
}

func (fx *runFixture) get() Run {
	fx.t.Helper()
	r, err := fx.st.GetRun(context.Background(), fx.run.ID)
	if err != nil {
		fx.t.Fatal(err)
	}
	return r
}

// drive steps the run, advancing the clock 61 s per step, until stop holds or the run is terminal.
func (fx *runFixture) drive(stop func(Run) bool) Run {
	fx.t.Helper()
	for i := 0; i < 400; i++ {
		r := fx.get()
		if (stop != nil && stop(r)) || RunTerminal(r.State) {
			return r
		}
		if _, err := fx.o.Step(context.Background(), r.ID); err != nil {
			fx.t.Fatalf("step in %s: %v", r.State, err)
		}
		fx.clk.Advance(61 * time.Second)
	}
	fx.t.Fatalf("run did not settle in 400 steps (state %s)", fx.get().State)
	return Run{}
}

func (fx *runFixture) untilState(s string) func(Run) bool {
	return func(r Run) bool { return r.State == s }
}

// happyMutations: one admission per staged file, dataset, notebook, kernel output (status + manifest + the other
// manifest files) and delete.
func (fx *runFixture) happyMutations(r Run) map[string]int {
	return map[string]int{"upload_dataset_file": len(r.Remote.Staged), "create_dataset": 1, "save_kernel": 1,
		"download_kernel_output": len(fx.outputs), "delete_dataset": 1, "delete_kernel": 1}
}

// assertNoReplays: no Execute of a mutation hit a known ID; strict also requires every admission to be the receipt
// the run holds for its step (no orphan admission).
func (fx *runFixture) assertNoReplays(r Run, strict bool) {
	t := fx.t
	t.Helper()
	for _, op := range []string{"upload_dataset_file", "create_dataset", "save_kernel", "download_kernel_output", "delete_dataset", "delete_kernel"} {
		if n := len(fx.f.Replays(op)); n != 0 {
			t.Errorf("%s replayed %d times", op, n)
		}
	}
	if !strict {
		return
	}
	tracked := map[string]bool{}
	for _, rec := range r.Remote.Jobs {
		tracked[rec.JobID] = rec.JobID != ""
	}
	for _, c := range fx.f.Mutations("") {
		if !tracked[c.ID] {
			t.Errorf("%s admission %s is not a receipt of the run", c.Op, c.ID)
		}
	}
}

func (fx *runFixture) artifactKinds(r Run) []string {
	fx.t.Helper()
	arts, err := fx.st.ListArtifacts(context.Background(), r.ID)
	if err != nil {
		fx.t.Fatal(err)
	}
	kinds := []string{}
	for _, a := range arts {
		kinds = append(kinds, a.Kind)
		f, err := OpenArtifact(fx.dataDir, a)
		if err != nil {
			fx.t.Errorf("artifact %s: %v", a.Kind, err)
			continue
		}
		f.Close()
	}
	sort.Strings(kinds)
	return kinds
}

func (fx *runFixture) assertSucceeded(r Run, kinds []string) {
	t := fx.t
	t.Helper()
	if r.State != RunSucceeded || r.ErrorCode != "" || len(r.Leftovers) != 0 || r.Remote.Finished == "" || r.Remote.GGUF == nil {
		t.Fatalf("run = %s %s %q leftovers %v", r.State, r.ErrorCode, r.Error, r.Leftovers)
	}
	if got := fx.artifactKinds(r); !reflect.DeepEqual(got, kinds) {
		t.Errorf("artifact kinds = %v, want %v", got, kinds)
	}
	for _, n := range fx.f.ProjectFiles() {
		if strings.HasPrefix(n, "r"+Run12(r.ID)+"-") {
			t.Errorf("staged file %s kept", n)
		}
	}
	if _, ok := fx.f.Dataset(KaggleRef("tester", r.Remote.DatasetSlug)); ok {
		t.Error("dataset not deleted")
	}
	if _, _, ok := fx.f.Kernel(KaggleRef("tester", r.Remote.NotebookSlug)); ok {
		t.Error("notebook not deleted")
	}
}

func countEvents(t *testing.T, st *Store, runID, kind string) int {
	t.Helper()
	evs, err := st.ListRunEvents(context.Background(), runID, 0, 1000)
	if err != nil {
		t.Fatal(err)
	}
	n := 0
	for _, e := range evs {
		if e.Kind == kind {
			n++
		}
	}
	return n
}

// collectHub counts hub events by name until the returned func is called.
func collectHub(t *testing.T, h *Hub, projectID string) func() map[string]int {
	ch, stop, err := h.Subscribe(projectID)
	if err != nil {
		t.Fatal(err)
	}
	var mu sync.Mutex
	seen := map[string]int{}
	done := make(chan struct{})
	go func() {
		defer close(done)
		for e := range ch {
			mu.Lock()
			seen[e.Name]++
			mu.Unlock()
		}
	}()
	return func() map[string]int { stop(); <-done; mu.Lock(); defer mu.Unlock(); return seen }
}

// stateEventTargets returns the "to" values of the state events in order.
func stateEventTargets(t *testing.T, st *Store, runID string) []string {
	t.Helper()
	evs, err := st.ListRunEvents(context.Background(), runID, 0, 1000)
	if err != nil {
		t.Fatal(err)
	}
	var out []string
	for _, e := range evs {
		if e.Kind != "state" {
			continue
		}
		s := string(e.Data)
		if strings.Contains(s, `"to"`) {
			out = append(out, s)
		}
	}
	return out
}

func TestOrchestratorHappyPaths(t *testing.T) {
	lock, err := trainer.LoadLock()
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name, profile, method string
		base                  bool
		kinds                 []string
	}{
		{"lora", "qwen35-0p8b", "lora", true, []string{"adapter", "adapter_config", "eval_samples", "gguf_base", "gguf_tuned", "metrics", "status"}},
		{"full", "gemma3-270m", "full", false, []string{"eval_samples", "gguf_tuned", "metrics", "status"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			ctx := context.Background()
			fx := newRunFixture(t, fixtureOpts{profile: c.profile, base: c.base})
			r := fx.drive(nil)
			fx.assertSucceeded(r, c.kinds)
			assertMutations(t, fx.f, fx.happyMutations(r))
			fx.assertNoReplays(r, true)
			if r.Remote.Method != c.method {
				t.Errorf("method = %s", r.Remote.Method)
			}
			if got := fx.f.JobsOf("upload_dataset_file")[0].ID; got != KaggleRequestID(r.ID, StepUploadPrefix+"train", 0) {
				t.Errorf("upload id %s is not the deterministic request id", got)
			}
			prof, _ := fx.cat.Get(c.profile)
			if r.Remote.GGUF.Architecture != prof.GGUF.ExpectedArch || r.Remote.Result == nil || r.Remote.Result.GPU != "Tesla T4" {
				t.Errorf("gguf %+v result %+v", r.Remote.GGUF, r.Remote.Result)
			}
			if pts, _ := fx.st.ListMetrics(ctx, r.ID, 0); len(pts) != 4 {
				t.Errorf("metrics = %d, want the 4 points of chibi-metrics.jsonl", len(pts))
			}
			raw, found, _ := fx.st.GetReport(ctx, r.ID, "auto")
			if !found || !strings.Contains(string(raw), `"run_id":"`+r.ID+`"`) {
				t.Errorf("report = %s", raw)
			}
			if c.name == "full" && !strings.Contains(string(raw), `"label_accuracy":{`) {
				t.Errorf("classify report has no label accuracy: %s", raw)
			}
			if c.base {
				b, ok, err := fx.st.GetBaseGGUF(ctx, prof.ID, prof.Base.Revision, BaseGGUFConverter(lock.LlamaCppTag))
				if err != nil || !ok || !strings.HasPrefix(b.RelPath, "base/"+prof.ID+"-") {
					t.Errorf("base gguf row = %+v %v %v", b, ok, err)
				}
			}
			if countEvents(t, fx.st, r.ID, "state") < 11 {
				t.Error("missing state events")
			}
			// A successful run ends from cleaning, so cleaning is its last working state; every state event
			// carries to, state and from.
			if r.Remote.LastState != RunCleaning {
				t.Errorf("LastState = %s, want cleaning", r.Remote.LastState)
			}
			targets := stateEventTargets(t, fx.st, r.ID)
			if len(targets) == 0 || !strings.Contains(targets[len(targets)-1], `"to":"succeeded"`) ||
				!strings.Contains(targets[len(targets)-1], `"state":"succeeded"`) || !strings.Contains(targets[len(targets)-1], `"from":"cleaning"`) {
				t.Errorf("last state event = %v", targets)
			}
		})
	}
}

func TestOrchestratorOutcomes(t *testing.T) {
	unknown := fakeOutcome{Status: KaggleJobUnknown}
	cases := []struct {
		name  string
		opts  fixtureOpts
		state string
		code  string
		adj   func(m map[string]int) // changes of the happy-path admission counts
		check func(t *testing.T, fx *runFixture, r Run)
	}{
		{name: "upload unknown gets a new attempt",
			opts:  fixtureOpts{before: func(fx *runFixture) { fx.f.Script("upload_dataset_file", fakeOutcome{Status: KaggleJobUnknown}) }},
			state: RunSucceeded,
			adj:   func(m map[string]int) { m["upload_dataset_file"]++ },
			check: func(t *testing.T, fx *runFixture, r Run) {
				jobs := fx.f.JobsOf("upload_dataset_file")
				if jobs[0].Status != KaggleJobUnknown || jobs[0].ID != KaggleRequestID(r.ID, "upload_train", 0) ||
					jobs[1].ID != KaggleRequestID(r.ID, "upload_train", 1) {
					t.Errorf("upload jobs = %+v", jobs[:2])
				}
			}},
		{name: "upload unknown three times",
			opts: fixtureOpts{before: func(fx *runFixture) {
				fx.f.Script("upload_dataset_file", unknown, unknown, unknown)
			}},
			state: RunFailed, code: "upload_failed",
			adj: func(m map[string]int) {
				for op := range m {
					m[op] = 0
				}
				m["upload_dataset_file"] = 3
			},
			check: func(t *testing.T, fx *runFixture, r Run) {
				for _, n := range fx.f.ProjectFiles() {
					if strings.HasPrefix(n, "r"+Run12(r.ID)+"-") {
						t.Errorf("staged file %s kept", n)
					}
				}
				if len(r.Leftovers) != 0 {
					t.Errorf("leftovers = %v", r.Leftovers)
				}
				if r.Remote.LastState != RunUploading {
					t.Errorf("LastState = %s, want uploading", r.Remote.LastState)
				}
			}},
		{name: "create unknown, dataset exists",
			opts: fixtureOpts{before: func(fx *runFixture) {
				fx.f.Script("create_dataset", fakeOutcome{Status: KaggleJobUnknown, Applied: true})
			}},
			state: RunSucceeded,
			check: func(t *testing.T, fx *runFixture, r Run) {
				if len(fx.f.ExecCalls("get_dataset")) < 1 {
					t.Error("no get_dataset read")
				}
			}},
		{name: "create unknown, dataset absent",
			opts: fixtureOpts{before: func(fx *runFixture) {
				fx.f.Script("create_dataset", fakeOutcome{Status: KaggleJobUnknown})
			}},
			state: RunSucceeded,
			adj:   func(m map[string]int) { m["create_dataset"] = 2 }},
		{name: "submit unknown, notebook runs",
			opts: fixtureOpts{before: func(fx *runFixture) {
				fx.f.Script("save_kernel", fakeOutcome{Status: KaggleJobUnknown, Applied: true})
			}},
			state: RunSucceeded,
			check: func(t *testing.T, fx *runFixture, r Run) {
				if !r.Remote.Untracked || r.Remote.KernelVersion != 1 {
					t.Errorf("untracked = %v, version = %d", r.Remote.Untracked, r.Remote.KernelVersion)
				}
				if n := len(fx.f.ExecCalls("get_kernel_session_status")); n < 2 {
					t.Errorf("status reads = %d", n)
				}
			}},
		{name: "submit unknown, notebook absent",
			opts: fixtureOpts{before: func(fx *runFixture) {
				fx.f.Script("save_kernel", fakeOutcome{Status: KaggleJobUnknown})
			}},
			state: RunFailed, code: "kaggle_submit_unknown",
			adj: func(m map[string]int) { m["download_kernel_output"], m["delete_kernel"] = 0, 0 },
			check: func(t *testing.T, fx *runFixture, r Run) {
				if len(r.Leftovers) != 0 {
					t.Errorf("leftovers = %v", r.Leftovers)
				}
			}},
		{name: "download unknown",
			opts: fixtureOpts{before: func(fx *runFixture) {
				fx.f.Script("download_kernel_output", fakeOutcome{Status: KaggleJobUnknown})
			}},
			state: RunSucceeded,
			adj:   func(m map[string]int) { m["download_kernel_output"]++ }},
		{name: "deadline",
			opts: fixtureOpts{before: func(fx *runFixture) {
				fx.f.Script("save_kernel", fakeOutcome{Remote: []string{"RUNNING"}})
			}},
			state: RunFailed, code: "remote_timeout",
			adj: func(m map[string]int) { m["download_kernel_output"] = 0 },
			check: func(t *testing.T, fx *runFixture, r Run) {
				if !strings.Contains(r.Error, KaggleNotebookURL(KaggleRef("tester", r.Remote.NotebookSlug))) {
					t.Errorf("error %q has no notebook url", r.Error)
				}
				if r.Remote.LastState != RunRunning {
					t.Errorf("LastState = %s, want running", r.Remote.LastState)
				}
			}},
		{name: "checksum mismatch",
			opts:  fixtureOpts{outputs: func(o *fakeRunOutputs) { o.WrongSHA = OutModelGGUF }},
			state: RunFailed, code: "checksum_mismatch",
			check: func(t *testing.T, fx *runFixture, r Run) {
				for _, k := range fx.artifactKinds(r) {
					if k == "gguf_tuned" {
						t.Error("tuned gguf registered despite the mismatch")
					}
				}
			}},
		{name: "trainer failed",
			opts: fixtureOpts{outputs: func(o *fakeRunOutputs) {
				o.Failed, o.Code, o.Message = true, "oom", "CUDA out of memory"
			}},
			state: RunFailed, code: "oom",
			check: func(t *testing.T, fx *runFixture, r Run) {
				ctx := context.Background()
				if !strings.Contains(r.Error, "CUDA out of memory") {
					t.Errorf("error = %q", r.Error)
				}
				if pts, _ := fx.st.ListMetrics(ctx, r.ID, 0); len(pts) != 4 {
					t.Errorf("metrics = %d", len(pts))
				}
				if got := fx.artifactKinds(r); !reflect.DeepEqual(got, []string{"metrics", "status"}) {
					t.Errorf("kinds = %v", got)
				}
				if _, found, _ := fx.st.GetReport(ctx, r.ID, "auto"); found {
					t.Error("a failed run has a report")
				}
			}},
		{name: "status missing",
			opts: fixtureOpts{before: func(fx *runFixture) {
				delete(fx.outputs, OutStatus)
				fx.f.SetOutputs(fx.outputs)
				fx.f.ScriptLogs(fakeLog{Body: fakeKernelLogText("Traceback (most recent call last)", "RuntimeError: boom")})
			}},
			state: RunFailed, code: "status_missing",
			adj: func(m map[string]int) { m["download_kernel_output"] = 3 },
			check: func(t *testing.T, fx *runFixture, r Run) {
				tail, err := ReadLogTail(fx.dataDir, r.ID)
				if err != nil || !strings.Contains(string(tail), "RuntimeError: boom") {
					t.Errorf("log tail = %q %v", tail, err)
				}
				if got := fx.artifactKinds(r); !reflect.DeepEqual(got, []string{"log_tail"}) {
					t.Errorf("kinds = %v", got)
				}
			}},
		{name: "AllowDelete off",
			opts:  fixtureOpts{cfg: func(c *OrchestratorConfig) { c.AllowDelete = false }},
			state: RunSucceeded,
			adj:   func(m map[string]int) { m["delete_dataset"], m["delete_kernel"] = 0, 0 },
			check: func(t *testing.T, fx *runFixture, r Run) {
				want := []Leftover{
					{Kind: "dataset", Ref: KaggleRef("tester", r.Remote.DatasetSlug), URL: KaggleDatasetURL(KaggleRef("tester", r.Remote.DatasetSlug))},
					{Kind: "notebook", Ref: KaggleRef("tester", r.Remote.NotebookSlug), URL: KaggleNotebookURL(KaggleRef("tester", r.Remote.NotebookSlug))},
				}
				if !reflect.DeepEqual(r.Leftovers, want) {
					t.Errorf("leftovers = %+v, want %+v", r.Leftovers, want)
				}
			}},
		{name: "storage cap",
			opts:  fixtureOpts{cfg: func(c *OrchestratorConfig) { c.StorageMaxBytes = 1 }},
			state: RunFailed, code: "storage_full",
			adj: func(m map[string]int) { m["download_kernel_output"] = 0 },
			check: func(t *testing.T, fx *runFixture, r Run) {
				if got := fx.artifactKinds(r); len(got) != 0 {
					t.Errorf("kinds = %v", got)
				}
			}},
		{name: "architecture mismatch",
			opts:  fixtureOpts{outputs: func(o *fakeRunOutputs) { o.Arch = "llama" }},
			state: RunFailed, code: "gguf_arch_mismatch",
			check: func(t *testing.T, fx *runFixture, r Run) {
				if !strings.Contains(r.Error, `"llama"`) {
					t.Errorf("error = %q", r.Error)
				}
			}},
		{name: "compute slot busy",
			opts:  fixtureOpts{before: func(fx *runFixture) { fx.f.AddForeignComputeJob(KaggleJobRemoteRunning) }},
			state: RunFailed, code: "kaggle_slot_busy",
			adj: func(m map[string]int) { m["download_kernel_output"], m["delete_kernel"] = 0, 0 },
			check: func(t *testing.T, fx *runFixture, r Run) {
				if len(r.Leftovers) != 0 {
					t.Errorf("leftovers = %v", r.Leftovers)
				}
			}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if c.opts.profile == "" {
				c.opts.profile = "gemma3-270m"
			}
			fx := newRunFixture(t, c.opts)
			r := fx.drive(nil)
			if r.State != c.state || r.ErrorCode != c.code {
				t.Fatalf("run = %s/%s (%q), want %s/%s", r.State, r.ErrorCode, r.Error, c.state, c.code)
			}
			want := fx.happyMutations(r)
			if c.adj != nil {
				c.adj(want)
			}
			for op, n := range want {
				if n == 0 {
					delete(want, op)
				}
			}
			assertMutations(t, fx.f, want)
			fx.assertNoReplays(r, false)
			if c.check != nil {
				c.check(t, fx, r)
			}
		})
	}
}

func TestOrchestratorCancelBeforeSubmit(t *testing.T) {
	ctx := context.Background()
	t.Run("after the dataset was created", func(t *testing.T) {
		fx := newRunFixture(t, fixtureOpts{profile: "gemma3-270m"})
		fx.drive(fx.untilState(RunWaitingDataset))
		got, err := fx.o.Cancel(ctx, fx.run.ID)
		if err != nil || !got.CancelRequested || got.State != RunWaitingDataset {
			t.Fatalf("cancel = %+v %v", got, err)
		}
		if v := got.View(); v.Cancel == nil || v.Cancel.StopURL != "" {
			t.Errorf("cancel view = %+v", v.Cancel)
		}
		r := fx.drive(nil)
		if r.State != RunCancelled || r.ErrorCode != "cancelled_by_user" || len(r.Leftovers) != 0 {
			t.Fatalf("run = %s/%s leftovers %v", r.State, r.ErrorCode, r.Leftovers)
		}
		if r.Remote.LastState != RunWaitingDataset {
			t.Errorf("LastState = %s, want waiting_dataset", r.Remote.LastState)
		}
		assertMutations(t, fx.f, map[string]int{"upload_dataset_file": len(r.Remote.Staged), "create_dataset": 1, "delete_dataset": 1})
		if _, ok := fx.f.Dataset(KaggleRef("tester", r.Remote.DatasetSlug)); ok {
			t.Error("dataset not deleted")
		}
		for _, n := range fx.f.ProjectFiles() {
			if strings.HasPrefix(n, "r"+Run12(r.ID)+"-") {
				t.Errorf("staged file %s kept", n)
			}
		}
	})
	t.Run("while an upload is queued", func(t *testing.T) {
		fx := newRunFixture(t, fixtureOpts{profile: "gemma3-270m"})
		fx.drive(func(r Run) bool {
			rec := r.Remote.Step(StepUploadPrefix + "train")
			return rec != nil && rec.JobID != ""
		})
		if _, err := fx.o.Cancel(ctx, fx.run.ID); err != nil {
			t.Fatal(err)
		}
		r := fx.drive(nil)
		if r.State != RunCancelled || r.ErrorCode != "cancelled_by_user" {
			t.Fatalf("run = %s/%s", r.State, r.ErrorCode)
		}
		assertMutations(t, fx.f, map[string]int{"upload_dataset_file": 1})
		rec := r.Remote.Step(StepUploadPrefix + "train")
		j, ok := fx.f.PeekJob(rec.JobID)
		if !ok || j.Status != KaggleJobCompleted {
			t.Errorf("upload job = %+v %v", j, ok)
		}
	})
}

func TestOrchestratorCancelAfterSubmit(t *testing.T) {
	ctx := context.Background()
	fx := newRunFixture(t, fixtureOpts{profile: "gemma3-270m", before: func(fx *runFixture) {
		fx.f.Script("save_kernel", fakeOutcome{Remote: []string{"RUNNING", "RUNNING", "RUNNING", "COMPLETE"}})
	}})
	fx.drive(fx.untilState(RunRunning))
	got, err := fx.o.Cancel(ctx, fx.run.ID)
	if err != nil {
		t.Fatal(err)
	}
	v := got.View()
	if !got.CancelRequested || v.Cancel == nil ||
		v.Cancel.StopURL != KaggleNotebookURL(KaggleRef("tester", got.Remote.NotebookSlug)) || v.Cancel.RemoteStopped ||
		v.Cancel.TimeLimitMinutes != got.Remote.SessionTimeoutMinutes {
		t.Fatalf("cancel = %+v view %+v", got.CancelRequested, v.Cancel)
	}
	if _, err := fx.o.Step(ctx, fx.run.ID); err != nil {
		t.Fatal(err)
	}
	r := fx.get()
	if r.State != RunRunning || !r.Remote.CancelTried || r.Remote.CancelRemoteStopped || !strings.Contains(r.Remote.CancelError, "session ID") {
		t.Fatalf("after one step: %s tried=%v stopped=%v err=%q", r.State, r.Remote.CancelTried, r.Remote.CancelRemoteStopped, r.Remote.CancelError)
	}
	r = fx.drive(nil)
	if r.State != RunCancelled || r.ErrorCode != "cancelled_by_user" {
		t.Fatalf("run = %s/%s (%q)", r.State, r.ErrorCode, r.Error)
	}
	if r.Remote.LastState != RunRunning {
		t.Errorf("LastState = %s, want running", r.Remote.LastState)
	}
	if n := len(fx.f.CallsTo("Cancel")); n != 1 {
		t.Errorf("remote cancel calls = %d", n)
	}
	assertMutations(t, fx.f, map[string]int{"upload_dataset_file": len(r.Remote.Staged), "create_dataset": 1, "save_kernel": 1,
		"delete_dataset": 1, "delete_kernel": 1})
}

func TestOrchestratorLiveLog(t *testing.T) {
	ctx := context.Background()
	t.Run("available", func(t *testing.T) {
		fx := newRunFixture(t, fixtureOpts{profile: "gemma3-270m", before: func(fx *runFixture) {
			fx.f.Script("save_kernel", fakeOutcome{Remote: []string{"RUNNING", "RUNNING", "COMPLETE"}})
			fx.f.ScriptLogs(fakeLog{Body: fakeKernelLogJSON(fakeTrainLog(20, 40)...)})
		}})
		collect := collectHub(t, fx.hub, fx.run.ProjectID)
		r := fx.drive(func(r Run) bool { return r.Remote.LastLogRead != "" })
		if r.State != RunRunning || r.Remote.Live != LiveYes || r.Remote.LastLogSeq != 3 {
			t.Fatalf("state %s live %s seq %d", r.State, r.Remote.Live, r.Remote.LastLogSeq)
		}
		want := RunStage{Name: "train", State: "start", Step: 20, MaxSteps: 40, Epoch: 0.5, EtaS: 20}
		if r.Remote.Stage == nil || *r.Remote.Stage != want {
			t.Errorf("stage = %+v, want %+v", r.Remote.Stage, want)
		}
		if pts, _ := fx.st.ListMetrics(ctx, r.ID, 0); len(pts) != 2 {
			t.Errorf("metrics = %d, want 2", len(pts))
		}
		if n := countEvents(t, fx.st, r.ID, "stage"); n != 1 {
			t.Errorf("stage events = %d", n)
		}
		tail, err := ReadLogTail(fx.dataDir, r.ID)
		if err != nil || !strings.Contains(string(tail), "@@chibi") {
			t.Errorf("log tail = %q %v", tail, err)
		}
		seen := collect()
		if seen["metric"] < 1 || seen["stage"] < 1 || seen["run"] < 1 {
			t.Errorf("hub events = %v", seen)
		}
		r = fx.drive(nil)
		if r.State != RunSucceeded || r.Remote.Live != LiveYes {
			t.Fatalf("run = %s/%s live %s", r.State, r.ErrorCode, r.Remote.Live)
		}
		if pts, _ := fx.st.ListMetrics(ctx, r.ID, 0); len(pts) != 4 {
			t.Errorf("metrics = %d, want 4", len(pts))
		}
		found := false
		for _, k := range fx.artifactKinds(r) {
			found = found || k == "log_tail"
		}
		if !found {
			t.Error("no log_tail artifact")
		}
	})
	t.Run("unavailable", func(t *testing.T) {
		fx := newRunFixture(t, fixtureOpts{profile: "gemma3-270m", before: func(fx *runFixture) {
			fx.f.Script("save_kernel", fakeOutcome{Remote: []string{"RUNNING", "RUNNING", "RUNNING", "COMPLETE"}})
			fx.f.ScriptLogs(fakeLog{Err: errors.New("Kaggle returned an unexpected notebook log response")}, fakeLog{})
		}})
		r := fx.drive(func(r Run) bool { return r.Remote.EmptyLogReads == 2 })
		if r.Remote.Live != LiveUnknown {
			t.Errorf("live after 2 empty reads = %s", r.Remote.Live)
		}
		r = fx.drive(func(r Run) bool { return r.Remote.EmptyLogReads >= 3 })
		if r.State != RunRunning || r.Remote.Live != LiveNo {
			t.Errorf("state %s live %s after %d empty reads", r.State, r.Remote.Live, r.Remote.EmptyLogReads)
		}
		r = fx.drive(nil)
		if r.State != RunSucceeded || r.Remote.Live != LiveNo {
			t.Fatalf("run = %s/%s live %s", r.State, r.ErrorCode, r.Remote.Live)
		}
		if pts, _ := fx.st.ListMetrics(ctx, r.ID, 0); len(pts) != 4 {
			t.Errorf("metrics = %d, want 4", len(pts))
		}
	})
}

func TestOrchestratorWaitsWhileBlocked(t *testing.T) {
	ctx := context.Background()
	block := func(t *testing.T) *runFixture {
		fx := newRunFixture(t, fixtureOpts{profile: "gemma3-270m"})
		fx.drive(fx.untilState(RunUploading))
		fx.f.SetAccount("tester", errors.New("kaggle is down"))
		return fx
	}
	t.Run("resumes when unblocked", func(t *testing.T) {
		fx := block(t)
		for i := 0; i < 3; i++ {
			if _, err := fx.o.Step(ctx, fx.run.ID); err != nil {
				t.Fatal(err)
			}
			fx.clk.Advance(time.Minute)
		}
		r := fx.get()
		if r.State != RunUploading || r.Remote.BlockedCode != "kaggle_unavailable" || r.Remote.BlockedSince == "" {
			t.Fatalf("state %s code %q since %q", r.State, r.Remote.BlockedCode, r.Remote.BlockedSince)
		}
		if n := countEvents(t, fx.st, r.ID, "warn"); n != 1 {
			t.Errorf("warn events = %d", n)
		}
		if n := len(fx.f.Mutations("")); n != 0 {
			t.Errorf("mutations while blocked = %d", n)
		}
		fx.f.SetAccount("tester", nil)
		r = fx.drive(nil)
		if r.State != RunSucceeded || r.Remote.BlockedCode != "" {
			t.Fatalf("run = %s/%s blocked %q", r.State, r.ErrorCode, r.Remote.BlockedCode)
		}
	})
	t.Run("fails after six hours", func(t *testing.T) {
		fx := block(t)
		if _, err := fx.o.Step(ctx, fx.run.ID); err != nil {
			t.Fatal(err)
		}
		fx.clk.Advance(7 * time.Hour)
		if _, err := fx.o.Step(ctx, fx.run.ID); err != nil {
			t.Fatal(err)
		}
		r := fx.get()
		if r.State != RunCleaning || r.ErrorCode != "kaggle_unavailable" {
			t.Fatalf("state %s code %s", r.State, r.ErrorCode)
		}
		if r.Remote.LastState != RunUploading {
			t.Errorf("LastState = %s, want uploading", r.Remote.LastState)
		}
		r = fx.drive(nil)
		if r.State != RunFailed {
			t.Errorf("end state = %s", r.State)
		}
	})
}

func TestOrchestratorStartLoop(t *testing.T) {
	ctx := context.Background()
	fast := intervals{localJob: time.Millisecond, running: time.Millisecond, dataset: time.Millisecond, logRead: time.Hour,
		untrackedPoll: time.Millisecond, blocked: time.Millisecond, idle: 20 * time.Millisecond, errorWait: time.Millisecond}
	t.Run("drives a run to the end", func(t *testing.T) {
		fx := newRunFixture(t, fixtureOpts{profile: "gemma3-270m"})
		fx.drive(fx.untilState(RunUploading))
		fx.restart()
		fx.o.iv = fast
		fx.o.Start(ctx)
		var r Run
		deadline := time.Now().Add(20 * time.Second)
		for time.Now().Before(deadline) {
			r = fx.get()
			if RunTerminal(r.State) {
				break
			}
			time.Sleep(10 * time.Millisecond)
		}
		fx.o.Close()
		if r.State != RunSucceeded {
			t.Fatalf("run = %s/%s (%q)", r.State, r.ErrorCode, r.Error)
		}
		assertMutations(t, fx.f, fx.happyMutations(r))
	})
	t.Run("a done context changes nothing", func(t *testing.T) {
		fx := newRunFixture(t, fixtureOpts{profile: "gemma3-270m"})
		cctx, cancel := context.WithCancel(ctx)
		cancel()
		fx.o.iv = fast
		fx.o.Start(cctx)
		fx.o.Close()
		r := fx.get()
		if r.State != RunQueued || r.Updated != fx.run.Updated {
			t.Errorf("run = %s updated %s (was %s)", r.State, r.Updated, fx.run.Updated)
		}
	})
}
