package server

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"sync"
	"testing"
	"time"

	"aurago/internal/chibisensei"
	"aurago/internal/kaggle"
)

var errChibiStubRefused = errors.New("stub: Kaggle mutations are refused in server tests")

// chibiStubKaggle is the in-memory chibisensei.KaggleDriver of the server tests (fakeKaggle lives in a _test.go file of
// internal/chibisensei and cannot be imported). Reads answer from the fields; every mutation is refused, so a run a
// test starts never leaves this process.
type chibiStubKaggle struct {
	mu         sync.Mutex
	account    string
	accountErr error
	quota      json.RawMessage
	jobs       []kaggle.Job
	env        chibiKaggleEnv
	counts     map[string]int
}

var (
	_ chibisensei.KaggleDriver = (*chibiStubKaggle)(nil)
	_ chibiKaggleEnvironment   = (*chibiStubKaggle)(nil)
)

func newChibiStubKaggle() *chibiStubKaggle {
	return &chibiStubKaggle{account: "tester", quota: json.RawMessage(`{"quotaRefreshTime":"2026-11-28T00:00:00Z","gpuQuota":{"timeUsed":"0s","timeReserved":"0s","totalTimeAllowed":"108000s","minimumTimeAllowed":"108000s"},"tpuQuota":{"timeUsed":"0s","timeReserved":"0s","totalTimeAllowed":"72000s","minimumTimeAllowed":"72000s"}}`),
		env: chibiKaggleEnv{VaultAvailable: true, CredentialsStored: true}, counts: map[string]int{}}
}

func (k *chibiStubKaggle) note(name string) { k.mu.Lock(); k.counts[name]++; k.mu.Unlock() }

func (k *chibiStubKaggle) calls(name string) int {
	k.mu.Lock()
	defer k.mu.Unlock()
	return k.counts[name]
}

func (k *chibiStubKaggle) kaggleEnv() chibiKaggleEnv { k.mu.Lock(); defer k.mu.Unlock(); return k.env }

func (k *chibiStubKaggle) Account(context.Context) (string, error) {
	k.note("account")
	k.mu.Lock()
	defer k.mu.Unlock()
	return k.account, k.accountErr
}

func (k *chibiStubKaggle) Execute(_ context.Context, req kaggle.Request) (kaggle.Result, error) {
	k.note(req.Operation)
	if req.Operation == "get_accelerator_quota_statistics" {
		k.mu.Lock()
		defer k.mu.Unlock()
		return kaggle.Result{Data: append(json.RawMessage(nil), k.quota...)}, nil
	}
	return kaggle.Result{}, errChibiStubRefused
}

func (k *chibiStubKaggle) Job(context.Context, string) (kaggle.Job, error) {
	return kaggle.Job{}, errors.New("stub: unknown job")
}

func (k *chibiStubKaggle) ActiveComputeJobs(context.Context) ([]kaggle.Job, error) {
	k.note("jobs")
	k.mu.Lock()
	defer k.mu.Unlock()
	return append([]kaggle.Job(nil), k.jobs...), nil
}

func (k *chibiStubKaggle) EnsureProject(context.Context) (string, error) {
	return "", errChibiStubRefused
}

func (k *chibiStubKaggle) WriteFile(context.Context, string, string, io.Reader) (int64, error) {
	return 0, errChibiStubRefused
}

func (k *chibiStubKaggle) OpenFile(string, string) (*os.File, error) { return nil, os.ErrNotExist }

func (k *chibiStubKaggle) RemoveFile(string, string) error { return nil }

func (k *chibiStubKaggle) ReadKernelLog(context.Context, string, string, int64) ([]byte, error) {
	return nil, nil
}

func (k *chibiStubKaggle) Cancel(context.Context, string) error { return errChibiStubRefused }

// chibiUseStubKaggle installs the stub, keeps the orchestrator off and reports 100 GiB of free disk. Call it before the
// first Chibi request: openChibiRuntime and chibiStartOrchestrator read the hooks.
func chibiUseStubKaggle(t *testing.T, s *Server) *chibiStubKaggle {
	t.Helper()
	s.chibiSenseiMu.Lock()
	opened := s.chibiSensei != nil
	s.chibiSenseiMu.Unlock()
	if opened {
		t.Fatal("chibiUseStubKaggle must run before the first Chibi Sensei request")
	}
	k := newChibiStubKaggle()
	s.chibiSenseiKaggleOverride = k
	s.chibiSenseiNoReconcile = true
	s.chibiFreeDiskOverride = func(string) (int64, error) { return 100 << 30, nil }
	return k
}

func TestChibiSenseiComputeJobFilter(t *testing.T) {
	save := func(kind string) json.RawMessage {
		b, err := json.Marshal(kaggle.Request{Domain: "notebooks", Operation: "save_kernel",
			Parameters: json.RawMessage(`{"kernelExecutionType":"` + kind + `"}`)})
		if err != nil {
			t.Fatal(err)
		}
		return b
	}
	for _, c := range []struct {
		name string
		job  kaggle.Job
		want bool
	}{
		{"run all", kaggle.Job{Operation: "save_kernel", Request: save("SAVE_AND_RUN_ALL")}, true},
		{"quick save", kaggle.Job{Operation: "save_kernel", Request: save("QUICK_SAVE")}, false},
		{"benchmark batch", kaggle.Job{Operation: "batch_schedule_benchmark_task_runs"}, true},
		{"upload", kaggle.Job{Operation: "upload_dataset_file"}, false},
		{"broken request", kaggle.Job{Operation: "save_kernel", Request: json.RawMessage(`{`)}, false},
	} {
		if got := chibiComputeJob(c.job); got != c.want {
			t.Errorf("%s: chibiComputeJob = %v, want %v", c.name, got, c.want)
		}
	}
}

func TestChibiSenseiKaggleReadCache(t *testing.T) {
	stub := newChibiStubKaggle()
	var c chibiKaggleReads
	ctx := context.Background()
	t0 := time.Date(2026, 11, 23, 10, 0, 0, 0, time.UTC)
	steps := []struct {
		key   string
		fresh bool
		at    time.Duration
		calls int
	}{
		{"a", false, 0, 1},                                  // first read
		{"a", false, 10 * time.Second, 1},                   // cached
		{"a", false, chibiKaggleReadTTL + time.Second, 2},   // expired
		{"b", false, chibiKaggleReadTTL + 2*time.Second, 3}, // another key (credentials changed)
		{"b", true, chibiKaggleReadTTL + 3*time.Second, 4},  // fresh (prepare, start)
	}
	for i, st := range steps {
		v := c.get(ctx, st.key, st.fresh, t0.Add(st.at), stub)
		if !v.ConnectionOK || v.Account != "tester" || !v.QuotaKnown || v.QuotaHours != 30 || !v.JobsRead {
			t.Fatalf("step %d: read = %+v", i, v)
		}
		if n := stub.calls("account"); n != st.calls {
			t.Fatalf("step %d: %d connection tests, want %d", i, n, st.calls)
		}
	}
	c.invalidate()
	c.get(ctx, "b", false, t0.Add(chibiKaggleReadTTL+4*time.Second), stub)
	if n := stub.calls("account"); n != 5 {
		t.Fatalf("after invalidate: %d connection tests, want 5", n)
	}
}
