package dockerstacks

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"testing"
	"time"

	"aurago/internal/dockerhost"
	"aurago/internal/dockerutil"
)

// The Hosts implementation over fake engines, and the small waiting and comparing helpers of the tests.

// ---- hosts -------------------------------------------------------------------

// fakeHosts satisfies Hosts over fake engines.
type fakeHosts struct {
	mu      sync.Mutex
	hosts   map[string]dockerhost.Host
	clients map[string]*dockerutil.Client
	down    map[string]bool
}

func newFakeHosts() *fakeHosts {
	return &fakeHosts{hosts: map[string]dockerhost.Host{}, clients: map[string]*dockerutil.Client{}, down: map[string]bool{}}
}

// add registers an engine under the host's id (fe.host() unless id differs).
func (f *fakeHosts) add(t *testing.T, id string, fe *fakeEngine) dockerhost.Host {
	t.Helper()
	h := fe.host()
	h.ID = id
	f.mu.Lock()
	defer f.mu.Unlock()
	f.hosts[id] = h
	f.clients[id] = fe.client(t)
	return h
}

func (f *fakeHosts) setDown(id string, down bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.down[id] = down
}

func (f *fakeHosts) Host(_ context.Context, id string) (dockerhost.Host, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	h, ok := f.hosts[id]
	if !ok {
		return dockerhost.Host{}, dockerhost.ErrHostNotFound
	}
	return h, nil
}

func (f *fakeHosts) Client(_ context.Context, id string) (*dockerutil.Client, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.down[id] {
		return nil, fmt.Errorf("dial tcp 127.0.0.1:1: connect: connection refused")
	}
	c, ok := f.clients[id]
	if !ok {
		return nil, dockerhost.ErrHostNotFound
	}
	return c, nil
}

// waitFor polls cond every few milliseconds until it holds or the timeout passes.
func waitUntil(t *testing.T, what string, timeout time.Duration, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("timed out after %s waiting for %s", timeout, what)
}

// assertSameJSON compares two JSON documents by value, so field order does not matter.
func assertSameJSON(t *testing.T, got any, want string) {
	t.Helper()
	gotRaw, err := json.Marshal(got)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var g, w any
	if err := json.Unmarshal(gotRaw, &g); err != nil {
		t.Fatalf("unmarshal got: %v", err)
	}
	if err := json.Unmarshal([]byte(want), &w); err != nil {
		t.Fatalf("golden is not valid JSON: %v", err)
	}
	gotCanon, _ := json.MarshalIndent(g, "", "  ")
	wantCanon, _ := json.MarshalIndent(w, "", "  ")
	if string(gotCanon) != string(wantCanon) {
		t.Fatalf("JSON differs.\ngot:\n%s\nwant:\n%s", gotCanon, wantCanon)
	}
}
