package ruview

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestKnownNodesValidateLiveDataAndKeepOnlyIDs(t *testing.T) {
	var reply atomic.Value
	reply.Store(`{"nodes":[{"node_id":7,"csi_status":"active","csi_last_seen_ms":42,"csi_sequence":3,"rssi_dbm":-51,"password":"never-persist"}]}`)
	m, c := fixture(t, func(w http.ResponseWriter, r *http.Request) { io.WriteString(w, reply.Load().(string)) })
	c.Directories.DataDir = t.TempDir()
	c.RuView.ReadOnly = false
	ctx := context.Background()
	got, err := m.Nodes(ctx)
	if err != nil || !got.Available || len(got.Nodes) != 1 || got.Nodes[0].CSIStatus != "active" {
		t.Fatalf("live nodes: %+v %v", got, err)
	}
	name, _ := nodeFile(c)
	data, _ := os.ReadFile(name)
	if string(data) != "[7]" {
		t.Fatalf("saved more than IDs: %q", data)
	}
	before, _ := os.Stat(name)
	if _, err = m.Nodes(ctx); err != nil {
		t.Fatal(err)
	}
	after, _ := os.Stat(name)
	if !before.ModTime().Equal(after.ModTime()) {
		t.Fatal("unchanged polls must not rewrite the inventory")
	}
	for _, body := range []string{`{}`, `{"nodes":null}`, `{"nodes":{}}`, `{"nodes":[{}]}`, `{"nodes":[{"node_id":256}]}`, `{"nodes":[{"node_id":1,"csi_status":"active","csi_last_seen_ms":-1}]}`, `{"nodes":[{"node_id":2,"csi_status":"stale"},{"node_id":2,"csi_status":"stale"}]}`} {
		reply.Store(body)
		got, err = m.Nodes(ctx)
		if err != nil || got.Available || len(got.Nodes) != 1 || got.Nodes[0].CSIStatus != "offline" || got.Nodes[0].CSILastSeenMS != nil {
			t.Fatalf("invalid upstream invented/lost measurements: %+v %v", got, err)
		}
	}
	reply.Store(`{"nodes":[]}`)
	if err = m.RememberNode(ctx, 2); err != nil {
		t.Fatal(err)
	}
	for _, id := range []int{0, 256, -1} {
		if m.RememberNode(ctx, id) == nil {
			t.Fatalf("accepted ID %d", id)
		}
	}
	oldScope := got.Scope
	oldURL := c.RuView.URL
	c.RuView.URL = strings.Replace(oldURL, "127.0.0.1", "localhost", 1)
	m.Configure()
	got, err = m.Nodes(ctx)
	if err != nil || len(got.Nodes) != 0 || got.Scope == oldScope {
		t.Fatalf("another service inherited devices: %+v %v", got, err)
	}
	c.RuView.URL = oldURL
	c.RuView.Source = "simulated"
	m.Configure()
	reply.Store(`{"nodes":[{"node_id":3,"csi_status":"active","csi_last_seen_ms":0,"csi_sequence":4}]}`)
	got, err = m.Nodes(ctx)
	if err != nil || got.Available || len(got.Nodes) != 2 || got.Nodes[0].CSIStatus != "offline" {
		t.Fatalf("simulation became a physical board: %+v %v", got, err)
	}
	c.RuView.ReadOnly = true
	m.Configure()
	if m.RememberNode(ctx, 4) == nil {
		t.Fatal("read-only registration accepted")
	}
	c.RuView.ReadOnly = false
	m.Configure()
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	if err = m.RememberNode(cancelled, 4); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancel: %v", err)
	}
	data, _ = os.ReadFile(name)
	if string(data) != "[2,7]" {
		t.Fatalf("unexpected IDs: %s", data)
	}
	if err = os.WriteFile(name, []byte("[7,"), 0600); err != nil {
		t.Fatal(err)
	}
	if m.RememberNode(ctx, 4) == nil {
		t.Fatal("corrupt inventory silently overwritten")
	}
	data, _ = os.ReadFile(name)
	if string(data) != "[7," {
		t.Fatal("corrupt inventory was not preserved")
	}
}

func TestKnownNodesSurviveNetworkFailureAndRejectLateResponses(t *testing.T) {
	var fail, hold atomic.Bool
	entered, release := make(chan struct{}, 1), make(chan struct{})
	m, c := fixture(t, func(w http.ResponseWriter, r *http.Request) {
		if hold.Load() {
			entered <- struct{}{}
			<-release
		}
		if fail.Load() {
			http.Error(w, "offline", 503)
			return
		}
		io.WriteString(w, `{"nodes":[{"node_id":9,"csi_status":"active","csi_last_seen_ms":0,"csi_sequence":1}]}`)
	})
	c.Directories.DataDir = t.TempDir()
	c.RuView.ReadOnly = false
	ctx := context.Background()
	if err := m.RememberNode(ctx, 1); err != nil {
		t.Fatal(err)
	}
	fail.Store(true)
	got, err := m.Nodes(ctx)
	if err != nil || got.Available || len(got.Nodes) != 1 || got.Nodes[0].NodeID != 1 || got.Nodes[0].CSIStatus != "offline" {
		t.Fatalf("offline: %+v %v", got, err)
	}
	fail.Store(false)
	hold.Store(true)
	result := make(chan error, 1)
	go func() { _, err := m.Nodes(ctx); result <- err }()
	select {
	case <-entered:
	case <-time.After(5 * time.Second):
		close(release)
		t.Fatal("fixture connection did not arrive")
	}
	// Revocation cancels both the HTTP request and any later inventory publication.
	m.Close()
	close(release)
	if err = <-result; !errors.Is(err, context.Canceled) {
		t.Fatalf("late response accepted: %v", err)
	}
	name, _ := nodeFile(c)
	data, _ := os.ReadFile(name)
	if strings.Contains(string(data), "9") {
		t.Fatal("late response persisted a board")
	}
}

func TestKnownNodesConcurrentRegistrationKeepsEveryBoard(t *testing.T) {
	m, c := fixture(t, func(w http.ResponseWriter, r *http.Request) { t.Error("registration must not contact upstream") })
	c.Directories.DataDir = t.TempDir()
	c.RuView.ReadOnly = false
	done := make(chan error, 24)
	for id := 1; id <= cap(done); id++ {
		go func() { done <- m.RememberNode(context.Background(), id) }()
	}
	for range cap(done) {
		if err := <-done; err != nil {
			t.Fatal(err)
		}
	}
	name, _ := nodeFile(c)
	data, err := os.ReadFile(name)
	var ids []int
	if err != nil || json.Unmarshal(data, &ids) != nil || len(ids) != cap(done) {
		t.Fatalf("lost board registrations: %s %v", data, err)
	}
	for i, id := range ids {
		if id != i+1 {
			t.Fatalf("unexpected IDs: %v", ids)
		}
	}
	if err := os.WriteFile(name, []byte("[7,7]"), 0600); err != nil {
		t.Fatal(err)
	}
	if m.RememberNode(context.Background(), 8) == nil {
		t.Fatal("duplicate stored IDs were not rejected")
	}
}

func TestRuViewNodesLiveState(t *testing.T) {
	const valid = `{"nodes":[{"node_id":7,"csi_status":"active","csi_last_seen_ms":1,"csi_sequence":2}]}`
	for _, tc := range []struct {
		name, source, mode string
		running            bool
		status             int
		body, want         string
		dials              int32
		live               []int
	}{
		{"wifi source has no node list", "wifi", "external", false, 0, valid, "not_applicable", 0, nil},
		{"managed runtime not running", "esp32", "docker", false, 0, valid, "not_running", 0, nil},
		{"managed runtime running", "esp32", "docker", true, 0, valid, "ok", 1, []int{7}},
		{"upstream failure", "esp32", "external", false, http.StatusServiceUnavailable, "", "unreachable", 1, nil},
		{"invalid reply", "esp32", "external", false, 0, `{"nodes":[{"node_id":0,"csi_status":"active"}]}`, "invalid", 1, nil},
		{"non-JSON reply", "esp32", "external", false, 0, "<html>login</html>", "invalid", 1, nil},
		{"valid reply", "esp32", "external", false, 0, valid, "ok", 1, []int{7}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var dials atomic.Int32
			m, c := fixture(t, func(w http.ResponseWriter, r *http.Request) {
				dials.Add(1)
				if tc.status != 0 {
					http.Error(w, "down", tc.status)
					return
				}
				io.WriteString(w, tc.body)
			})
			// A managed runtime is always dialed on loopback, so point it at the fake: a stray dial would be counted.
			u, err := url.Parse(c.RuView.URL)
			if err != nil {
				t.Fatal(err)
			}
			if c.RuView.HTTPPort, err = strconv.Atoi(u.Port()); err != nil {
				t.Fatal(err)
			}
			c.Directories.DataDir = t.TempDir()
			c.RuView.ReadOnly = false
			c.RuView.Mode, c.RuView.Source = tc.mode, tc.source
			if tc.mode != "external" {
				c.RuView.URL = ""
			}
			if tc.running {
				m.mu.Lock()
				m.managed = true
				m.mu.Unlock()
			}
			if err = m.RememberNode(context.Background(), 3); err != nil {
				t.Fatal(err)
			}
			got, err := m.Nodes(context.Background())
			if err != nil {
				t.Fatal(err)
			}
			if got.LiveState != tc.want || got.Available != (tc.want == "ok") || dials.Load() != tc.dials {
				t.Fatalf("live_state=%q available=%v dials=%d: %+v", got.LiveState, got.Available, dials.Load(), got)
			}
			// Saved identities stay visible as offline boards in every state; only live IDs carry telemetry.
			want := append([]int{3}, tc.live...)
			if len(got.Nodes) != len(want) {
				t.Fatalf("nodes: %+v", got.Nodes)
			}
			for i, node := range got.Nodes {
				wantStatus := "offline"
				if node.NodeID == 7 {
					wantStatus = "active"
				}
				if node.NodeID != want[i] || node.CSIStatus != wantStatus {
					t.Fatalf("node %d: %+v", i, node)
				}
			}
			data, err := json.Marshal(got)
			if err != nil || !strings.Contains(string(data), `"live_state":"`+tc.want+`"`) {
				t.Fatalf("wire shape: %s %v", data, err)
			}
		})
	}
}
