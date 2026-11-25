package kaggle

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"aurago/internal/security"
)

type logFixture struct {
	requests atomic.Int32 // every request to the Kaggle API fixture
	logCalls atomic.Int32 // requests to GetKernelSessionLogsStream
	params   atomic.Value // map[string]any of the last log request
}

// newLogTestService returns a service whose Kaggle client is a local fixture. The account is
// "owner", downloads are granted, and respond answers the log operation.
func newLogTestService(t *testing.T, mutate func(*Snapshot), respond func(http.ResponseWriter)) (*Service, *logFixture) {
	t.Helper()
	s := newTestService(t)
	snap := s.current()
	snap.Config.AllowDownload = true
	if mutate != nil {
		mutate(&snap)
	}
	s.current = func() Snapshot { return snap }
	f := &logFixture{}
	c := fixtureClient(t, func(w http.ResponseWriter, r *http.Request) {
		f.requests.Add(1)
		var p map[string]any
		_ = json.NewDecoder(r.Body).Decode(&p)
		switch {
		case strings.HasSuffix(r.URL.Path, "IntrospectToken"):
			io.WriteString(w, `{"active":true,"username":"owner"}`)
		case strings.HasSuffix(r.URL.Path, "GetKernelSessionLogsStream"):
			f.logCalls.Add(1)
			f.params.Store(p)
			respond(w)
		default:
			t.Errorf("unexpected route %s", r.URL.Path)
			w.WriteHeader(http.StatusTeapot)
		}
	})
	c.http.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	s.clientFactory = func(Credentials, time.Duration) (*Client, error) { return c, nil }
	return s, f
}

// assertNoDurableTrace fails when a log read left a job row or a project directory behind.
func assertNoDurableTrace(t *testing.T, s *Service) {
	t.Helper()
	jobs, err := s.Store.List(context.Background())
	if err != nil || len(jobs) != 0 {
		t.Fatalf("log read created job rows: %v %v", jobs, err)
	}
	entries, err := os.ReadDir(s.Root)
	if err != nil || len(entries) != 0 {
		t.Fatalf("log read wrote project storage: %v %v", entries, err)
	}
}

var logAccess = Access{Network: true, Write: true}

func TestKaggleReadKernelLogGrantsOwnerAndReferences(t *testing.T) {
	ok := func(w http.ResponseWriter) { io.WriteString(w, "log") }
	for _, tc := range []struct {
		name   string
		mutate func(*Snapshot)
		access Access
		ctx    func(context.Context, Snapshot) context.Context
	}{
		{"integration disabled", func(s *Snapshot) { s.Config.Enabled = false }, logAccess, nil},
		{"download grant missing", func(s *Snapshot) { s.Config.AllowDownload = false }, logAccess, nil},
		{"network denied by caller", nil, Access{Write: true}, nil},
		{"write denied by caller", nil, Access{Network: true}, nil},
		{"write denied by host", func(s *Snapshot) { s.Access.Write = false }, logAccess, nil},
		{"run snapshot without download", nil, logAccess, func(ctx context.Context, s Snapshot) context.Context {
			c := s.Config
			c.AllowDownload = false
			return WithConfigSnapshot(ctx, c)
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s, f := newLogTestService(t, tc.mutate, ok)
			ctx := context.Background()
			if tc.ctx != nil {
				ctx = tc.ctx(ctx, s.current())
			}
			if _, err := s.ReadKernelLog(ctx, tc.access, "owner", "chibi-run", 1024); err == nil {
				t.Fatal("refused read succeeded")
			}
			if f.requests.Load() != 0 {
				t.Fatalf("a refused read contacted Kaggle %d times", f.requests.Load())
			}
			assertNoDurableTrace(t, s)
		})
	}
	s, f := newLogTestService(t, nil, ok)
	for _, ref := range []struct {
		owner, slug string
	}{{"owner", "owner/chibi-run"}, {"", "chibi-run"}, {"owner", "../x"}} {
		if _, err := s.ReadKernelLog(context.Background(), logAccess, ref.owner, ref.slug, 1024); err == nil {
			t.Fatalf("invalid reference accepted: %+v", ref)
		}
	}
	if f.requests.Load() != 0 {
		t.Fatal("invalid references contacted Kaggle")
	}
	if _, err := s.ReadKernelLog(context.Background(), logAccess, "stranger", "chibi-run", 1024); err == nil {
		t.Fatal("foreign owner accepted")
	}
	if f.logCalls.Load() != 0 {
		t.Fatal("foreign owner reached the log operation")
	}
	assertNoDurableTrace(t, s)
}

func TestKaggleReadKernelLogBodies(t *testing.T) {
	jsonArray := `[{"stream_name":"stdout","time":1.5,"data":"@@chibi {\"seq\":1,\"t\":\"stage\"}\n"}]`
	for _, tc := range []struct {
		name    string
		status  int
		body    string
		max     int64
		want    string
		wantErr bool
	}{
		{"plain text", 200, "line\n@@chibi {\"seq\":1}\n", 1 << 20, "line\n@@chibi {\"seq\":1}\n", false},
		{"tail", 200, "0123456789", 4, "6789", false},
		{"zero max means the cap", 200, "abc", 0, "abc", false},
		{"json array", 200, jsonArray, 1 << 20, jsonArray, false},
		{"empty body", 200, "", 1024, "", false},
		{"not found yet", 404, "", 1024, "", false},
		{"no content", 204, "", 1024, "", false},
		{"server error", 500, "", 1024, "", true},
		{"json object refused", 200, `{"logsUrl":"https://signed.invalid/x?sig=abc"}`, 1024, "", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s, f := newLogTestService(t, nil, func(w http.ResponseWriter) {
				w.WriteHeader(tc.status)
				io.WriteString(w, tc.body)
			})
			got, err := s.ReadKernelLog(context.Background(), logAccess, "owner", "chibi-run", tc.max)
			if (err != nil) != tc.wantErr {
				t.Fatalf("err = %v, wantErr %v", err, tc.wantErr)
			}
			if err != nil && strings.Contains(err.Error(), "signed.invalid") {
				t.Fatal("error leaked the response body")
			}
			if tc.status == 500 {
				var api *APIError
				if !errors.As(err, &api) || api.Status != 500 {
					t.Fatalf("want APIError 500, got %#v", err)
				}
			}
			if string(got) != tc.want || tc.want == "" && got != nil {
				t.Fatalf("got %q, want %q", got, tc.want)
			}
			p, _ := f.params.Load().(map[string]any)
			if p["userName"] != "owner" || p["kernelSlug"] != "chibi-run" || p["waitForLogsUrlSeconds"] != float64(10) {
				t.Fatalf("wire parameters %v", p)
			}
			assertNoDurableTrace(t, s)
		})
	}
}

func TestKaggleReadKernelLogRedirectAndScrub(t *testing.T) {
	secret := "chibi-log-secret-" + newID()
	security.RegisterSensitive(secret)
	for _, tc := range []struct {
		name          string
		storageStatus int
		wantBody      bool
	}{
		{"storage body", 200, true},
		{"storage not found", 404, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s, _ := newLogTestService(t, nil, func(w http.ResponseWriter) {
				w.Header().Set("Location", "https://storage.invalid/log")
				w.WriteHeader(http.StatusFound)
			})
			var storageCalls atomic.Int32
			s.storageFactory = func(ctx context.Context, location string, timeout time.Duration) (*http.Client, error) {
				if location != "https://storage.invalid/log" {
					t.Errorf("storage location %q", location)
				}
				return &http.Client{Transport: fixtureTransport(func(r *http.Request) (*http.Response, error) {
					storageCalls.Add(1)
					if r.Header.Get("Authorization") != "" || r.Header.Get("Cookie") != "" || r.Method != http.MethodGet {
						t.Error("Kaggle credentials or a non-GET reached storage")
					}
					return &http.Response{StatusCode: tc.storageStatus, Header: make(http.Header), Body: io.NopCloser(strings.NewReader("before " + secret + " after"))}, nil
				})}, nil
			}
			got, err := s.ReadKernelLog(context.Background(), logAccess, "owner", "chibi-run", 1024)
			if err != nil {
				t.Fatal(err)
			}
			if storageCalls.Load() != 1 {
				t.Fatalf("storage calls = %d", storageCalls.Load())
			}
			if strings.Contains(string(got), secret) {
				t.Fatal("registered secret survived the log read")
			}
			if !tc.wantBody && got != nil {
				t.Fatalf("storage 404 must mean no log yet, got %q", got)
			}
			if tc.wantBody && (!strings.HasPrefix(string(got), "before ") || !strings.HasSuffix(string(got), " after")) {
				t.Fatalf("storage body not returned: %q", got)
			}
			assertNoDurableTrace(t, s)
		})
	}
}

// shortenLogBounds shortens the live read bounds for one test. Tests that call it must not run in parallel.
func shortenLogBounds(t *testing.T, idle, overall time.Duration) {
	t.Helper()
	oldIdle, oldMax := kernelLogIdle, kernelLogStreamMax
	kernelLogIdle, kernelLogStreamMax = idle, overall
	t.Cleanup(func() { kernelLogIdle, kernelLogStreamMax = oldIdle, oldMax })
}

// streamResponder writes the head and the chunks of a response, flushes, and then either ends the response (eof) or
// keeps it open until done is closed. The test registers the close after the service, so it runs before the
// fixture server waits for its handlers.
func streamResponder(done <-chan struct{}, contentType string, eof bool, chunks ...string) func(http.ResponseWriter) {
	return func(w http.ResponseWriter) {
		w.Header().Set("Content-Type", contentType)
		w.WriteHeader(http.StatusOK)
		for _, c := range chunks {
			io.WriteString(w, c)
		}
		w.(http.Flusher).Flush()
		if !eof {
			select {
			case <-done:
			case <-time.After(10 * time.Second):
			}
		}
	}
}

func TestKaggleReadKernelLogEventStream(t *testing.T) {
	secret := "chibi-stream-secret-" + newID()
	security.RegisterSensitive(secret)
	ev1 := `data: {"stream_name":"stdout","time":1.5,"data":"@@chibi {\"seq\":1}\n"}` + "\n\n"
	ev2 := `data: {"stream_name":"stderr","time":2.5,"data":"` + secret + `\n"}` + "\n\n"
	for _, tc := range []struct {
		name   string
		eof    bool
		chunks []string
		want   []string
	}{
		{"open stream ends by the idle timer", false, []string{ev1, ev2}, []string{ev1, "stream_name\":\"stderr"}},
		{"silent stream is no log yet", false, nil, nil},
		{"stream that ends by EOF", true, []string{ev1, ev2}, []string{ev1, "stream_name\":\"stderr"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			shortenLogBounds(t, 150*time.Millisecond, 2*time.Second)
			done := make(chan struct{})
			s, f := newLogTestService(t, nil, streamResponder(done, "text/event-stream", tc.eof, tc.chunks...))
			t.Cleanup(func() { close(done) })
			start := time.Now()
			got, err := s.ReadKernelLog(context.Background(), logAccess, "owner", "chibi-run", 1<<20)
			if err != nil {
				t.Fatal(err)
			}
			if time.Since(start) > time.Second {
				t.Fatalf("read took %s", time.Since(start))
			}
			if tc.want == nil {
				if got != nil {
					t.Fatalf("silent stream returned %q", got)
				}
			}
			for _, w := range tc.want {
				if !strings.Contains(string(got), w) {
					t.Fatalf("missing %q in %q", w, got)
				}
			}
			if strings.Contains(string(got), secret) {
				t.Fatal("registered secret survived the log read")
			}
			p, _ := f.params.Load().(map[string]any)
			if _, has := p["versionLabel"]; has {
				t.Fatalf("log request carries a versionLabel: %v", p)
			}
			assertNoDurableTrace(t, s)
		})
	}
}

func TestKaggleReadKernelLogJSONStall(t *testing.T) {
	shortenLogBounds(t, 150*time.Millisecond, 2*time.Second)
	done := make(chan struct{})
	s, _ := newLogTestService(t, nil, streamResponder(done, "application/json", false, `[{"stream_name":"stdout","time":1,"data":"a"}`))
	t.Cleanup(func() { close(done) })
	if got, err := s.ReadKernelLog(context.Background(), logAccess, "owner", "chibi-run", 1<<20); err == nil {
		t.Fatalf("stalled JSON body accepted: %q", got)
	}
	assertNoDurableTrace(t, s)
}

func TestKaggleReadKernelLogCallerContextEnds(t *testing.T) {
	shortenLogBounds(t, 5*time.Second, 10*time.Second)
	done := make(chan struct{})
	s, _ := newLogTestService(t, nil, streamResponder(done, "text/event-stream", false, "data: {\"stream_name\":\"stdout\",\"time\":1,\"data\":\"a\"}\n\n"))
	t.Cleanup(func() { close(done) })
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	if got, err := s.ReadKernelLog(ctx, logAccess, "owner", "chibi-run", 1<<20); err == nil {
		t.Fatalf("the caller's cancelled context was not returned: %q", got)
	}
}
