package kaggle

import (
	"aurago/internal/security"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestKaggleBenchmarkAcceptanceDoesNotInventRunningJobs(t *testing.T) {
	for _, tc := range []struct{ receipt, status string }{
		{`{"results":[{"runScheduled":false,"runSkippedReason":"quota"}]}`, "failed"},
		{`{"results":[{"runScheduled":true,"runId":9007199254740993}]}`, "remote_running"},
		{`{"results":[{"runScheduled":true}]}`, "unknown"},
		{`{"results":[]}`, "unknown"},
		{`{"results":[{"runScheduled":false},{"runScheduled":true,"runId":12}]}`, "remote_running"},
		{`{"results":[{"runScheduled":true,"runId":12},{"runScheduled":true}]}`, "unknown"},
	} {
		status, reason := benchmarkAcceptance([]byte(tc.receipt))
		if status != tc.status || status != "remote_running" && reason == "" {
			t.Errorf("receipt %s: %s %s", tc.receipt, status, reason)
		}
	}
}

func waitKaggleJob(t *testing.T, s *Service, id, state string) Job {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		j, err := s.Store.Get(context.Background(), id)
		if err != nil {
			t.Fatal(err)
		}
		if j.Status == state {
			return j
		}
		if j.Status == "failed" || j.Status == "unknown" {
			t.Fatalf("job %s: %s", j.Status, j.Error)
		}
		time.Sleep(10 * time.Millisecond)
	}
	j, _ := s.Store.Get(context.Background(), id)
	t.Fatalf("job did not reach %s: %+v", state, j)
	return Job{}
}

func TestKaggleRemoteNotebookLifecycleAndPinnedOutput(t *testing.T) {
	s := newTestService(t)
	snap := s.current()
	snap.Config.ReadOnly = false
	snap.Config.AllowCreate = true
	snap.Config.AllowRemoteExecute = true
	snap.Config.AllowDownload = true
	s.current = func() Snapshot { return snap }
	var saves atomic.Int32
	c := fixtureClient(t, func(w http.ResponseWriter, r *http.Request) {
		var p map[string]any
		json.NewDecoder(r.Body).Decode(&p)
		switch {
		case strings.HasSuffix(r.URL.Path, "IntrospectToken"):
			io.WriteString(w, `{"active":true,"username":"owner"}`)
		case strings.HasSuffix(r.URL.Path, "ListDatasets"):
			io.WriteString(w, `{"datasets":[{"ref":"public/data","title":"Data"}],"nextPageToken":"next"}`)
		case strings.HasSuffix(r.URL.Path, "DownloadDataset"):
			io.WriteString(w, "n\n40\n")
		case strings.HasSuffix(r.URL.Path, "GetKernel"):
			w.WriteHeader(404)
		case strings.HasSuffix(r.URL.Path, "SaveKernel"):
			saves.Add(1)
			if p["kernelExecutionType"] != "SAVE_AND_RUN_ALL" || p["isPrivate"] != true || p["enableGpu"] != false || p["enableInternet"] != false || p["sessionTimeoutSeconds"] != float64(3600) {
				t.Errorf("unsafe/invalid notebook defaults: %v", p)
			}
			if p["text"] != `{"nbformat":4,"nbformat_minor":5,"metadata":{},"cells":[]}` {
				t.Error("frozen notebook changed")
			}
			io.WriteString(w, `{"ref":"owner/analysis","versionNumber":7,"kernelId":321}`)
		case strings.HasSuffix(r.URL.Path, "GetKernelSessionStatus"):
			if p["versionLabel"] != "7" {
				t.Error("polled unpinned notebook version")
			}
			io.WriteString(w, `{"status":"COMPLETE"}`)
		case strings.HasSuffix(r.URL.Path, "DownloadKernelOutput"):
			if p["versionNumber"] != float64(7) {
				t.Error("downloaded unpinned output")
			}
			io.WriteString(w, "answer\n42\n")
		default:
			t.Error("unexpected route", r.URL.Path)
			w.WriteHeader(404)
		}
	})
	s.clientFactory = func(Credentials, time.Duration) (*Client, error) { return c, nil }
	ctx := context.Background()
	project, err := s.CreateProject(ctx, "Analysis")
	if err != nil {
		t.Fatal(err)
	}
	list, err := s.Execute(ctx, Request{Domain: "datasets", Operation: "list_datasets", Parameters: []byte(`{"search":"data","pageSize":10}`)}, snap.Access)
	if err != nil || !strings.Contains(string(list.Data), "public/data") {
		t.Fatal("search", err)
	}
	download, err := s.Execute(ctx, Request{Domain: "datasets", Operation: "download_dataset", ProjectID: project.ID, File: "input.csv", Parameters: []byte(`{"ownerSlug":"public","datasetSlug":"data","fileName":"input.csv","datasetVersionNumber":2}`)}, snap.Access)
	if err != nil {
		t.Fatal(err)
	}
	waitKaggleJob(t, s, download.Job.ID, "completed")
	request := Request{ID: newID(), Domain: "notebooks", Operation: "save_kernel", Parameters: []byte(`{"slug":"owner/analysis","newTitle":"Analysis","text":"{\"nbformat\":4,\"nbformat_minor\":5,\"metadata\":{},\"cells\":[]}","kernelType":"notebook","language":"python","kernelExecutionType":"SAVE_AND_RUN_ALL","datasetDataSources":["public/data"]}`)}
	run, err := s.Execute(ctx, request, snap.Access)
	if err != nil {
		t.Fatal(err)
	}
	waitKaggleJob(t, s, run.Job.ID, "remote_running")
	replay, err := s.Execute(ctx, request, snap.Access)
	if err != nil || replay.Job.ID != run.Job.ID || saves.Load() != 1 {
		t.Fatal("mutation replayed", err)
	}
	if err = s.Cancel(ctx, run.Job.ID); err == nil || !strings.Contains(err.Error(), "session ID") {
		t.Fatal("guessed unsupported cancellation", err)
	}
	s.pollRemote()
	waitKaggleJob(t, s, run.Job.ID, "completed")
	output, err := s.Execute(ctx, Request{Domain: "notebooks", Operation: "download_kernel_output", ProjectID: project.ID, File: "result.csv", Parameters: []byte(`{"ownerSlug":"owner","kernelSlug":"analysis","filePath":"result.csv","versionNumber":7}`)}, snap.Access)
	if err != nil {
		t.Fatal(err)
	}
	waitKaggleJob(t, s, output.Job.ID, "completed")
	f, err := s.OpenFile(project.ID, "result.csv")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	b, _ := io.ReadAll(f)
	if string(b) != "answer\n42\n" {
		t.Fatal("result import failed")
	}
}

func TestKaggleBenchmarkCancellationUsesSDKWrapperAndDoesNotReplay(t *testing.T) {
	s := newTestService(t)
	ctx := context.Background()
	id := newID()
	result := cleanOutput(json.RawMessage(`{"results":[{"runId":9007199254740993,"runScheduled":true}]}`))
	s.Store.admit(ctx, Job{ID: id, Account: "owner", Domain: "benchmarks", Operation: "batch_schedule_benchmark_task_runs", Hash: "x", Request: []byte(`{}`)})
	s.Store.update(ctx, id, "remote_running", 0, result, "")
	var cancellations atomic.Int32
	c := fixtureClient(t, func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "IntrospectToken") {
			io.WriteString(w, `{"active":true,"username":"owner"}`)
			return
		}
		cancellations.Add(1)
		var p map[string]any
		decoder := json.NewDecoder(r.Body)
		decoder.UseNumber()
		decoder.Decode(&p)
		wrapper, ok := p["runIds"].(map[string]any)
		if !ok || len(wrapper["ids"].([]any)) != 1 {
			t.Error("not the SDK cancellation wrapper")
		}
		if fmt.Sprint(wrapper["ids"].([]any)[0]) != "9007199254740993" {
			t.Error("remote int64 identity rounded")
		}
		// A server failure leaves an uncertain remote outcome, never a replay.
		w.WriteHeader(503)
	})
	s.clientFactory = func(Credentials, time.Duration) (*Client, error) { return c, nil }
	if err := s.Cancel(ctx, id); err == nil {
		t.Fatal("lost mutation response was reported successful")
	}
	if err := s.Cancel(ctx, id); err != nil {
		t.Fatal(err)
	}
	if cancellations.Load() != 1 {
		t.Fatal("uncertain cancellation repeated")
	}
	j, _ := s.Store.Get(ctx, id)
	if j.Status != "cancel_requested" || j.Error == "" {
		t.Fatal("uncertainty disappeared")
	}
	op, _ := Lookup("benchmarks", "api_cancel_benchmark_task_runs")
	good, _, err := Decode(op, []byte(`{"runIds":{"ids":[9007199254740993]},"includeActive":true}`))
	if err != nil || s.ownedCancellation(ctx, op, good, "owner") != nil {
		t.Fatal("own cancellation denied", err)
	}
	bad, _, _ := Decode(op, []byte(`{"runIds":{"ids":[9007199254740993,456]},"includeActive":true}`))
	if s.ownedCancellation(ctx, op, bad, "owner") == nil {
		t.Fatal("untracked run cancellation admitted")
	}
}

func TestKaggleWirePolicyAndResponseSecretRedaction(t *testing.T) {
	s := newTestService(t)
	cfg := s.current().Config
	cfg.ReadOnly = false
	cfg.AllowCreate = true
	cfg.AllowUpdate = true
	cfg.AllowRemoteExecute = true
	op, _ := Lookup("notebooks", "save_kernel")
	p, _, err := Decode(op, []byte(`{"slug":"owner/book","kernelExecutionType":"SAVE_AND_RUN_ALL"}`))
	if err != nil || CheckPolicy(cfg, s.current().Access, op, p, "owner") != nil {
		t.Fatal("wire enum cannot execute", err)
	}
	op, _ = Lookup("datasets", "create_dataset_version")
	p, _, _ = Decode(op, []byte(`{"ownerSlug":"owner","datasetSlug":"data","body":{"deleteOldVersions":true}}`))
	if CheckPolicy(cfg, s.current().Access, op, p, "owner") == nil {
		t.Fatal("version deletion bypassed delete grant")
	}
	security.RegisterSensitive("fixture-kaggle-redaction-0123456789")
	clean := cleanOutput([]byte(`{"description":"fixture-kaggle-redaction-0123456789","nested":["fixture-kaggle-redaction-0123456789"],"pageToken":"keep"}`))
	if strings.Contains(string(clean), "fixture-kaggle") || !strings.Contains(string(clean), "keep") {
		t.Fatal("returned secret or lost pagination")
	}
	if remoteState("CANCEL_ACKNOWLEDGED") != "cancelled" || remoteState("BENCHMARK_TASK_RUN_STATE_ERRORED") != "failed" {
		t.Fatal("SDK terminal states not recognized")
	}
}
