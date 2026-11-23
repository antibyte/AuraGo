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
			if _, err := s.ReadKernelLog(ctx, tc.access, "owner", "chibi-run", 3, 1024); err == nil {
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
		version     int
	}{{"owner", "owner/chibi-run", 3}, {"", "chibi-run", 3}, {"owner", "../x", 3}, {"owner", "chibi-run", 0}, {"owner", "chibi-run", -1}} {
		if _, err := s.ReadKernelLog(context.Background(), logAccess, ref.owner, ref.slug, ref.version, 1024); err == nil {
			t.Fatalf("invalid reference accepted: %+v", ref)
		}
	}
	if f.requests.Load() != 0 {
		t.Fatal("invalid references contacted Kaggle")
	}
	if _, err := s.ReadKernelLog(context.Background(), logAccess, "stranger", "chibi-run", 3, 1024); err == nil {
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
			got, err := s.ReadKernelLog(context.Background(), logAccess, "owner", "chibi-run", 3, tc.max)
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
			if p["userName"] != "owner" || p["kernelSlug"] != "chibi-run" || p["versionLabel"] != "3" || p["waitForLogsUrlSeconds"] != float64(10) {
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
			got, err := s.ReadKernelLog(context.Background(), logAccess, "owner", "chibi-run", 3, 1024)
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
