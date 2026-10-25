package ruview

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"
	"unicode/utf8"

	"aurago/internal/security"
)

func TestRuViewStatusFields(t *testing.T) {
	t.Run("keys", func(t *testing.T) {
		data, err := json.Marshal(Status{})
		if err != nil {
			t.Fatal(err)
		}
		for _, key := range []string{"runtime", "phase", "phase_since", "error_code", "error_detail", "last_fresh_at", "stop_reason", "stopped_at", "storage_used_bytes", "storage_budget_bytes", "recording_at", "hints"} {
			if !strings.Contains(string(data), `"`+key+`":`) {
				t.Errorf("Status JSON lacks %q: %s", key, data)
			}
		}
		// Only the zero struct may serialize hints as null: Manager.Status never does.
		if !strings.Contains(string(data), `"hints":null`) {
			t.Errorf("zero Status should be the only source of null hints: %s", data)
		}
	})

	t.Run("external service", func(t *testing.T) {
		var state atomic.Value
		state.Store("fresh")
		m, c := fixture(t, func(w http.ResponseWriter, r *http.Request) {
			switch state.Load().(string) {
			case "401":
				http.Error(w, "no", http.StatusUnauthorized)
			case "503":
				http.Error(w, "down", http.StatusServiceUnavailable)
			case "stale":
				io.WriteString(w, `{"status":"ready","source":"esp32","source_state":"stale","last_frame_age_ms":30000}`)
			default:
				io.WriteString(w, `{"status":"ready","source":"esp32","source_state":"live_unverified","last_frame_age_ms":20}`)
			}
		})
		s := m.Status()
		if s.Runtime != RuntimeExternal || s.StorageBudgetBytes != nil || s.StorageUsedBytes != nil || s.Hints == nil || len(s.Hints) != 0 || s.LastFreshAt != nil || s.RecordingAt != nil || s.StopReason != "" || s.Phase != "" {
			t.Fatalf("external status: %+v", s)
		}
		if _, err := m.Test(context.Background()); err != nil {
			t.Fatal(err)
		}
		fresh := m.Status().LastFreshAt
		if fresh == nil {
			t.Fatal("a fresh probe did not set last_fresh_at")
		}
		state.Store("stale")
		if _, err := m.Test(context.Background()); err != nil {
			t.Fatal(err)
		}
		if kept := m.Status().LastFreshAt; kept == nil || !kept.Equal(*fresh) {
			t.Fatalf("a stale probe moved last_fresh_at: %v then %v", fresh, kept)
		}
		state.Store("401")
		if _, err := m.Test(context.Background()); err == nil {
			t.Fatal("rejected probe succeeded")
		}
		if s := m.Status(); s.ErrorCode != CodeUpstreamAuth || s.ErrorDetail != "HTTP 401" {
			t.Fatalf("401 must be upstream_auth: %+v", s)
		}
		state.Store("503")
		if _, err := m.Test(context.Background()); err == nil {
			t.Fatal("failing probe succeeded")
		}
		if s := m.Status(); s.ErrorCode != CodeUpstreamError {
			t.Fatalf("503 must be upstream_error: %+v", s)
		}
		c.RuView.Source = "wifi" // runtime-relevant: the last live moment belongs to the old source
		m.Configure()
		if m.Status().LastFreshAt != nil {
			t.Fatal("a source change kept last_fresh_at")
		}
	})

	t.Run("recording and hints", func(t *testing.T) {
		m, _ := fixture(t, func(http.ResponseWriter, *http.Request) {})
		m.noteRecording("/api/v1/recording/start", http.StatusOK)
		at := m.Status().RecordingAt
		if at == nil {
			t.Fatal("a confirmed recording start did not set recording_at")
		}
		m.noteRecording("/api/v1/recording/start", http.StatusBadGateway)
		if kept := m.Status().RecordingAt; kept == nil || !kept.Equal(*at) {
			t.Fatalf("an unconfirmed start moved recording_at: %v then %v", at, kept)
		}
		m.emitHint(hintStorageLimit, true)
		m.emitHint(hintSensingStale, true)
		if got := strings.Join(m.Status().Hints, ","); got != "sensing_stale,storage_limit" {
			t.Fatalf("hints must list the open names, sorted: %q", got)
		}
	})

	t.Run("managed storage numbers", func(t *testing.T) {
		m, _, _ := managedBudget(t, "native")
		s := m.Status()
		if s.Runtime != RuntimeRunning || s.StorageBudgetBytes == nil || *s.StorageBudgetBytes != 1024<<20 || s.StorageUsedBytes != nil {
			t.Fatalf("before the first walk the budget is known and the usage is not: %+v", s)
		}
	})
}

func TestRuViewStorageSamplingIsThrottledAndErrorsAreNotFull(t *testing.T) {
	m, c, log := managedBudget(t, "native")
	c.RuView.StorageMB = 1
	data := filepath.Join(c.Directories.DataDir, "ruview", "data")
	watch := func() {
		t.Helper()
		if m.watch(context.Background()) == nil {
			t.Fatal("managed runtime did not schedule a probe")
		}
	}
	// 1. A missing directory is empty, not full.
	watch()
	s := m.Status()
	if s.StorageUsedBytes == nil || *s.StorageUsedBytes != 0 || s.StorageBudgetBytes == nil || *s.StorageBudgetBytes != 1<<20 || s.StorageLimit {
		t.Fatalf("missing data directory: %+v", s)
	}
	// 2. The walk is throttled.
	if err := os.MkdirAll(data, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(data, "recording.bin"), make([]byte, 2<<20), 0o600); err != nil {
		t.Fatal(err)
	}
	m.storageEvery = time.Hour
	watch()
	if s = m.Status(); *s.StorageUsedBytes != 0 || s.StorageLimit {
		t.Fatalf("a young sample was walked again: %+v", s)
	}
	// 3. Once the interval is over the new size counts.
	m.storageEvery = 0
	watch()
	if s = m.Status(); *s.StorageUsedBytes != 2<<20 || !s.StorageLimit || len(s.Hints) != 1 || s.Hints[0] != hintStorageLimit {
		t.Fatalf("2 MiB in a 1 MiB budget: %+v", s)
	}
	requireHints(t, log.snapshot(), []hintCall{{hintStorageLimit, true}})
	// 4. An unreadable directory is neither full nor empty, and the hint stays as it was.
	m.walkStorage = func(context.Context, string) (int64, error) { return 0, errors.New("permission denied at /secret/x") }
	watch()
	s = m.Status()
	if s.StorageLimit || s.StorageUsedBytes != nil || s.ErrorCode != CodeStorageUnreadable || len(s.Hints) != 1 {
		t.Fatalf("unreadable storage: %+v", s)
	}
	// The walk error is raw OS text; none of it may surface anywhere in the status.
	if blob, err := json.Marshal(s); err != nil || strings.Contains(string(blob), "/secret") || strings.Contains(string(blob), "permission denied") {
		t.Fatalf("the storage failure text reached the status: %s %v", blob, err)
	}
	requireHints(t, log.snapshot(), []hintCall{{hintStorageLimit, true}})
	// 5. Recording start reports the failure to check, not "full".
	w := httptest.NewRecorder()
	m.Proxy(w, httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{}`)), "/api/v1/recording/start")
	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("an unreadable directory must be 503, got %d", w.Code)
	}
}

func TestRuViewScrubDetail(t *testing.T) {
	const token = "ruview-scrub-detail-token-0123456789"
	security.RegisterSensitive(token)
	for _, tc := range []struct {
		name, in string
		want     []string // substrings the result must contain
		not      []string // substrings it must not contain
	}{
		{"url", "Get https://user:pw@registry.example/v2/x?q=1 failed", []string{"[url]"}, []string{"registry.example", "pw@"}},
		{"windows path", `open C:\Users\x\data denied`, []string{"open [path]"}, []string{"C:", "Users"}},
		{"unc path", `open \\host\share\file denied`, []string{"[path]"}, []string{"host", "share"}},
		{"unix socket", "dial unix /var/run/docker.sock: connect: no such file", []string{"[path]", "connect"}, []string{"/var", "docker.sock"}},
		{"quoted path", `stat "/etc/secret": denied`, []string{"[path]"}, []string{"/etc"}},
		{"registered token", "bad token " + token + " rejected", []string{"rejected"}, []string{token}},
		{"control characters", "a\x07b\nc", []string{"a b c"}, nil},
		{"plain text", "container exited with code 137", []string{"container exited with code 137"}, nil},
		// Paths with spaces run to the next ": " (path: reason) or quote.
		{"windows path with spaces", `open C:\Program Files\RuView\data\x.bin: access denied`, []string{"[path]: access denied"}, []string{"Program", "RuView", "x.bin"}},
		{"windows user with spaces", `stat "C:\Users\Andi Jakob\data": denied`, []string{`"[path]": denied`}, []string{"Andi", "Jakob"}},
		{"unc path with spaces", `open \\nas\my share\file.bin: denied`, []string{"[path]: denied"}, []string{"nas", "share", "file.bin"}},
		{"unix path with spaces", "stat /home/Andi Jakob/ruview/data: permission denied", []string{"[path]: permission denied"}, []string{"Andi", "Jakob", "/home"}},
		{"ipv4 and port", "dial tcp 10.0.0.5:2375: connect: connection refused", []string{"[addr]: connect: connection refused"}, []string{"10.0.0.5", "2375"}},
		{"ipv6 and port", "dial tcp [fe80::1]:2375: i/o timeout", []string{"[addr]: i/o timeout"}, []string{"fe80", "2375"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := scrubDetail(tc.in, maxDetailBytes)
			for _, want := range tc.want {
				if !strings.Contains(got, want) {
					t.Errorf("%q lacks %q", got, want)
				}
			}
			for _, not := range tc.not {
				if strings.Contains(got, not) {
					t.Errorf("%q still contains %q", got, not)
				}
			}
		})
	}
	long := scrubDetail(strings.Repeat("\u00e4", 600), maxDetailBytes)
	if len(long) > maxDetailBytes || !utf8.ValidString(long) || long == "" {
		t.Fatalf("long text must be cut to %d bytes at a rune boundary: %d bytes, valid=%v", maxDetailBytes, len(long), utf8.ValidString(long))
	}
	if got := readStatusError(errors.New("")); got != "RuView status could not be read" {
		t.Fatalf("empty status error: %q", got)
	}
	// The legacy sentence (Status.Error, the "error" response field) is redacted
	// like Detail, whatever raw text a caller hands to fail.
	raw := errors.New(`Docker request: Get "http://docker/v1.51/containers/aurago-ruview/json": dial unix /var/run/docker.sock: connect: permission denied`)
	e := fail(CodeStartFailed, "inspect RuView container: "+raw.Error(), raw)
	for _, text := range []string{e.Error(), e.Detail, legacyText(raw)} {
		if strings.Contains(text, "docker.sock") || strings.Contains(text, "http://") || strings.Contains(text, "/v1.51") || !strings.Contains(text, "permission denied") {
			t.Fatalf("legacy or detail text leaked or lost the reason: %q", text)
		}
	}
	// The server log keeps the raw cause.
	if got := rawText(e); !strings.Contains(got, "docker.sock") {
		t.Fatalf("rawText must keep the cause for the log: %q", got)
	}
}

func TestRuViewStorageWalkIsBoundedAndRecordingStartUsesYoungSamples(t *testing.T) {
	dir := t.TempDir()
	for i := 0; i < 5; i++ {
		if err := os.WriteFile(filepath.Join(dir, "f"+strconv.Itoa(i)), make([]byte, 10), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if used, err := walkBytes(context.Background(), dir, 100); err != nil || used != 50 {
		t.Fatalf("walk within the cap: %d %v", used, err)
	}
	if _, err := walkBytes(context.Background(), dir, 3); !errors.Is(err, errStorageTooLarge) {
		t.Fatalf("a directory over the entry cap must be an error: %v", err)
	}
	cancelled, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := directoryBytes(cancelled, dir); err == nil {
		t.Fatal("a cancelled walk must stop")
	}

	m, c, _ := managedBudget(t, "native")
	c.RuView.StorageMB = 1
	var walks atomic.Int32
	m.walkStorage = func(context.Context, string) (int64, error) { walks.Add(1); return 2 << 20, nil }
	m.storageEvery = time.Hour
	start := func() int {
		w := httptest.NewRecorder()
		m.Proxy(w, httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{}`)), "/api/v1/recording/start")
		return w.Code
	}
	if first, second := start(), start(); first != 507 || second != 507 || walks.Load() != 1 {
		t.Fatalf("a burst of recording starts must share one young sample: %d %d walks=%d", first, second, walks.Load())
	}
	m.storageEvery = 0
	if start() != 507 || walks.Load() != 2 {
		t.Fatalf("an old sample must be walked again: walks=%d", walks.Load())
	}
}
