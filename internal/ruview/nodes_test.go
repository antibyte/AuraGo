package ruview

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
