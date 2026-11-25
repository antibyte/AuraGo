package kaggle

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestKaggleNotebookRef(t *testing.T) {
	for _, tc := range []struct {
		ref, owner, slug string
		ok               bool
	}{
		{"owner/analysis", "owner", "analysis", true},
		{"/code/owner/analysis", "owner", "analysis", true},
		{"code/owner/analysis", "owner", "analysis", true},
		{"", "", "", false},
		{"owner", "", "", false},
		{"/code/owner", "", "", false},
		{"/datasets/owner/slug", "", "", false},
		{"a/b/c", "", "", false},
		{"owner/", "", "", false},
		{"/code//slug", "", "", false},
	} {
		owner, slug, ok := notebookRef(tc.ref)
		if owner != tc.owner || slug != tc.slug || ok != tc.ok {
			t.Errorf("notebookRef(%q) = %q %q %v, want %q %q %v", tc.ref, owner, slug, ok, tc.owner, tc.slug, tc.ok)
		}
	}
}

// notebookPollFixture answers like real Kaggle: a status read that carries a versionLabel is a 404.
type notebookPollFixture struct {
	mu     sync.Mutex
	status int    // HTTP status of the status read without a label
	body   string // body of that read
	params map[string]any
}

func (f *notebookPollFixture) set(status int, body string) {
	f.mu.Lock()
	f.status, f.body = status, body
	f.mu.Unlock()
}

// newNotebookPollService returns a service whose Kaggle account is "owner" and a fixture for the status read.
func newNotebookPollService(t *testing.T) (*Service, *notebookPollFixture) {
	t.Helper()
	s := newTestService(t)
	snap := s.current()
	snap.Config.RemoteTimeoutMinutes = 60
	s.current = func() Snapshot { return snap }
	f := &notebookPollFixture{status: 200, body: `{"status":"RUNNING"}`}
	c := fixtureClient(t, func(w http.ResponseWriter, r *http.Request) {
		var p map[string]any
		_ = json.NewDecoder(r.Body).Decode(&p)
		switch {
		case strings.HasSuffix(r.URL.Path, "IntrospectToken"):
			io.WriteString(w, `{"active":true,"username":"owner"}`)
		case strings.HasSuffix(r.URL.Path, "GetKernelSessionStatus"):
			f.mu.Lock()
			f.params = p
			status, body := f.status, f.body
			f.mu.Unlock()
			if _, has := p["versionLabel"]; has {
				w.WriteHeader(404)
				io.WriteString(w, `{"error":{"code":404,"message":"Kernel version with label '1' not found.","status":"NOT_FOUND"}}`)
				return
			}
			w.WriteHeader(status)
			io.WriteString(w, body)
		default:
			t.Errorf("unexpected route %s", r.URL.Path)
			w.WriteHeader(http.StatusTeapot)
		}
	})
	s.clientFactory = func(Credentials, time.Duration) (*Client, error) { return c, nil }
	return s, f
}

// seedNotebookJob stores a notebook job in the given state, created age ago.
func seedNotebookJob(t *testing.T, s *Service, status, result string, age time.Duration) string {
	t.Helper()
	ctx := context.Background()
	id := newID()
	request, _ := json.Marshal(Request{Domain: "notebooks", Operation: "save_kernel", Parameters: []byte(`{"slug":"owner/analysis"}`)})
	if _, _, err := s.Store.admit(ctx, Job{ID: id, Account: "owner", Domain: "notebooks", Operation: "save_kernel", Hash: id, Request: request}); err != nil {
		t.Fatal(err)
	}
	if err := s.Store.update(ctx, id, status, 0, json.RawMessage(result), ""); err != nil {
		t.Fatal(err)
	}
	created := time.Now().UTC().Add(-age).Format(time.RFC3339Nano)
	if _, err := s.Store.db.ExecContext(ctx, `UPDATE jobs SET created=? WHERE id=?`, created, id); err != nil {
		t.Fatal(err)
	}
	return id
}

func notebookJobResult(t *testing.T, j Job) map[string]any {
	t.Helper()
	var r map[string]any
	if err := json.Unmarshal(j.Result, &r); err != nil {
		t.Fatal(err)
	}
	return r
}

func TestKagglePollRemoteNotebookReachesTerminal(t *testing.T) {
	for _, ref := range []string{"/code/owner/analysis", "owner/analysis"} {
		t.Run(ref, func(t *testing.T) {
			s, f := newNotebookPollService(t)
			id := seedNotebookJob(t, s, "remote_running", `{"ref":"`+ref+`","versionNumber":1,"kernelId":321}`, time.Minute)
			s.pollRemote()
			j, _ := s.Store.Get(context.Background(), id)
			if j.Status != "remote_running" || notebookJobResult(t, j)["remote_status"] != "RUNNING" {
				t.Fatalf("running notebook: %s %s", j.Status, j.Result)
			}
			f.set(200, `{"status":"COMPLETE"}`)
			s.pollRemote()
			j, _ = s.Store.Get(context.Background(), id)
			if j.Status != "completed" || notebookJobResult(t, j)["remote_status"] != "COMPLETE" {
				t.Fatalf("finished notebook: %s %s", j.Status, j.Result)
			}
			f.mu.Lock()
			defer f.mu.Unlock()
			if _, has := f.params["versionLabel"]; has || f.params["userName"] != "owner" || f.params["kernelSlug"] != "analysis" {
				t.Fatalf("status read parameters %v", f.params)
			}
		})
	}
}

func TestKagglePollRemoteNotebookGone(t *testing.T) {
	const denied = `{"error":{"code":403,"message":"Permission 'kernels.get' was denied","status":"PERMISSION_DENIED"}}`
	const noRuns = `{"error":{"code":404,"message":"No runs found for this kernel.","status":"NOT_FOUND"}}`
	own := `{"ref":"/code/owner/analysis","versionNumber":1,"kernelId":321}`
	foreign := `{"ref":"/code/stranger/analysis","versionNumber":1,"kernelId":321}`
	for _, tc := range []struct {
		name    string
		status  string
		result  string
		age     time.Duration
		code    int
		body    string
		want    string
		wantErr string
	}{
		{"own 403 inside the grace", "remote_running", own, 30 * time.Second, 403, denied, "remote_running", ""},
		{"own 403 past the grace", "remote_running", own, 3 * time.Minute, 403, denied, "failed", "The Kaggle notebook no longer exists"},
		{"own 403 past the grace, cancel requested", "cancel_requested", own, 3 * time.Minute, 403, denied, "cancelled", "The Kaggle notebook no longer exists"},
		{"foreign 403 keeps waiting", "remote_running", foreign, 3 * time.Minute, 403, denied, "remote_running", ""},
		{"foreign 403 past the remote timeout", "remote_running", foreign, 61 * time.Minute, 403, denied, "failed", "Kaggle reports no run for this notebook"},
		{"404 before the remote timeout", "remote_running", own, 3 * time.Minute, 404, noRuns, "remote_running", ""},
		{"404 past the remote timeout", "remote_running", own, 61 * time.Minute, 404, noRuns, "failed", "Kaggle reports no run for this notebook"},
		{"500 keeps waiting", "remote_running", own, 3 * time.Hour, 500, `{}`, "remote_running", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s, f := newNotebookPollService(t)
			f.set(tc.code, tc.body)
			id := seedNotebookJob(t, s, tc.status, tc.result, tc.age)
			s.pollRemote()
			j, _ := s.Store.Get(context.Background(), id)
			if j.Status != tc.want || j.Error != tc.wantErr {
				t.Fatalf("job %s %q, want %s %q", j.Status, j.Error, tc.want, tc.wantErr)
			}
			if tc.wantErr != "" {
				r := notebookJobResult(t, j)
				if r["remote_status"] != "NOT_FOUND" || r["kernelId"] != float64(321) || r["versionNumber"] != float64(1) {
					t.Fatalf("result %s", j.Result)
				}
			}
		})
	}
}
