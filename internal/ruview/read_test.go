package ruview

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestRuViewReadKeepsPayloadWhenStatusProbeFails(t *testing.T) {
	var statusCode atomic.Int32
	statusCode.Store(http.StatusOK)
	var statusBody atomic.Value
	statusBody.Store(`{"status":"ready","source":"simulated","source_state":"synthetic","last_frame_age_ms":0}`)
	var sensingCode atomic.Int32
	sensingCode.Store(http.StatusOK)
	m, c := fixture(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1/sensing/latest":
			if code := sensingCode.Load(); code != http.StatusOK {
				w.WriteHeader(int(code))
				return
			}
			io.WriteString(w, `{"frames":1,"note":"kept"}`)
		case "/api/v1/status":
			if code := statusCode.Load(); code != http.StatusOK {
				w.WriteHeader(int(code))
				return
			}
			io.WriteString(w, statusBody.Load().(string))
		default:
			http.NotFound(w, r)
		}
	})
	c.RuView.Source = "simulated"
	ctx := context.Background()
	if _, err := m.Test(ctx); err != nil || !m.Status().Simulated {
		t.Fatalf("prior simulated status: %v %+v", err, m.Status())
	}
	statusCode.Store(http.StatusServiceUnavailable)
	got, err := m.Read(ctx, "sensing")
	if err != nil {
		t.Fatal(err)
	}
	payload, ok := got.(payloadRead)
	if !ok || payload.Fresh || !payload.Simulated || payload.DataState != "disconnected" || payload.Source != "simulated" {
		t.Fatalf("payload: %+v", got)
	}
	if payload.StatusError == "" || strings.Contains(payload.StatusError, "http") || strings.Contains(payload.StatusError, "test-ruview-private-token") || strings.Contains(payload.StatusError, c.RuView.URL) {
		t.Fatalf("status_error leaked or missing: %q", payload.StatusError)
	}
	var body map[string]any
	if json.Unmarshal(payload.Data, &body) != nil || body["frames"] != float64(1) || body["note"] != "kept" {
		t.Fatalf("data: %s", payload.Data)
	}
	if _, err = time.Parse(time.RFC3339, payload.ObservedAt.Format(time.RFC3339)); err != nil {
		t.Fatal(err)
	}
	if _, err = m.Read(ctx, "status"); err == nil {
		t.Fatal("status stayed successful while its probe failed")
	}
	sensingCode.Store(http.StatusBadGateway)
	if failed, err := m.Read(ctx, "sensing"); err == nil || failed != nil {
		t.Fatalf("failed data read invented a payload: %+v %v", failed, err)
	}
	poison := readStatusError(fmt.Errorf("status failed at %s bearer test-ruview-private-token", c.RuView.URL))
	if strings.Contains(poison, "http") || strings.Contains(poison, "test-ruview-private-token") || strings.Contains(poison, c.RuView.URL) {
		t.Fatalf("scrubbed error: %q", poison)
	}
}

func TestRuViewPresenceSummarizesLiveAndSavedBoards(t *testing.T) {
	var nodesBody atomic.Value
	nodesBody.Store(`{"nodes":[{"node_id":7,"csi_status":"active","csi_last_seen_ms":42,"csi_sequence":3,"rssi_dbm":-51,"password":"never-show","ssid":"secret-net"},{"node_id":8,"csi_status":"stale","csi_last_seen_ms":10,"csi_sequence":1,"rssi_dbm":-40},{"node_id":9,"csi_status":"active","csi_last_seen_ms":10001,"csi_sequence":2,"rssi_dbm":-12},{"node_id":10,"csi_status":"active","csi_last_seen_ms":5,"rssi_dbm":-7},{"node_id":11,"csi_status":"active","csi_last_seen_ms":10000,"csi_sequence":0,"rssi_dbm":-3}]}`)
	var statusBody atomic.Value
	statusBody.Store(`{"status":"ready","source":"esp32","source_state":"live_unverified","last_frame_age_ms":0}`)
	m, c := fixture(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1/nodes":
			io.WriteString(w, nodesBody.Load().(string))
		case "/api/v1/status":
			io.WriteString(w, statusBody.Load().(string))
		default:
			http.NotFound(w, r)
		}
	})
	c.Directories.DataDir = t.TempDir()
	ctx := context.Background()
	got, err := m.Read(ctx, "presence")
	if err != nil {
		t.Fatal(err)
	}
	presence, ok := got.(presenceRead)
	if !ok || !presence.Fresh || presence.Simulated || presence.Source != "esp32" || presence.DataState != "live_unverified" {
		t.Fatalf("envelope: %+v", got)
	}
	if len(presence.Nodes) != 5 {
		t.Fatalf("nodes: %+v", presence.Nodes)
	}
	byID := map[int]presenceNode{}
	for _, node := range presence.Nodes {
		byID[node.NodeID] = node
	}
	live := byID[7]
	if live.State != "live" || !live.Fresh || live.RSSIRetained || live.RSSI == nil || *live.RSSI != -51 {
		t.Fatalf("live: %+v", live)
	}
	stale := byID[8]
	if stale.State != "stale" || stale.Fresh || stale.RSSI != nil || stale.RSSIRetained {
		t.Fatalf("stale: %+v", stale)
	}
	aged := byID[9]
	if aged.State != "live" || aged.Fresh || aged.RSSI == nil || *aged.RSSI != -12 {
		t.Fatalf("aged: %+v", aged)
	}
	unsequenced := byID[10]
	if unsequenced.State != "live" || unsequenced.Fresh || unsequenced.RSSI == nil {
		t.Fatalf("unsequenced: %+v", unsequenced)
	}
	boundary := byID[11]
	if boundary.State != "live" || !boundary.Fresh || boundary.RSSI == nil || *boundary.RSSI != -3 {
		t.Fatalf("boundary: %+v", boundary)
	}
	nodesBody.Store(`{"nodes":[]}`)
	got, err = m.Read(ctx, "presence")
	if err != nil {
		t.Fatal(err)
	}
	presence = got.(presenceRead)
	if len(presence.Nodes) != 5 {
		t.Fatalf("saved ids dropped: %+v", presence.Nodes)
	}
	for _, node := range presence.Nodes {
		if node.State != "offline" || node.Fresh || node.RSSI != nil || node.RSSIRetained {
			t.Fatalf("offline: %+v", node)
		}
	}
	encoded, err := json.Marshal(presence)
	if err != nil {
		t.Fatal(err)
	}
	name, err := nodeFile(c)
	if err != nil {
		t.Fatal(err)
	}
	text := string(encoded)
	for _, secret := range []string{"password", "ssid", "never-show", "secret-net", "scope", filepath.Base(name), "csi_last_seen_ms", "csi_sequence"} {
		if strings.Contains(text, secret) {
			t.Fatalf("presence exposed %s: %s", secret, text)
		}
	}
	statusBody.Store(`{"status":"ready","source":"simulated","source_state":"synthetic","last_frame_age_ms":0}`)
	nodesBody.Store(`{"nodes":[{"node_id":7,"csi_status":"active","csi_last_seen_ms":0,"csi_sequence":4,"rssi_dbm":-51}]}`)
	c.RuView.Source = "esp32"
	got, err = m.Read(ctx, "presence")
	if err != nil {
		t.Fatal(err)
	}
	presence = got.(presenceRead)
	if !presence.Simulated || presence.Fresh || presence.Nodes[0].NodeID != 7 || presence.Nodes[0].Fresh || presence.Nodes[0].State != "live" {
		t.Fatalf("simulation became a sensor: %+v", presence)
	}
	for _, node := range presence.Nodes[1:] {
		if node.State != "offline" || node.RSSI != nil {
			t.Fatalf("saved board inherited the simulation reading: %+v", node)
		}
	}
	raw, err := m.Read(ctx, "nodes")
	if err != nil {
		t.Fatal(err)
	}
	liveRead := raw.(payloadRead)
	if strings.Contains(string(liveRead.Data), `"node_id":8`) || !strings.Contains(string(liveRead.Data), `"csi_status":"active"`) {
		t.Fatalf("nodes operation left the live GET: %s", liveRead.Data)
	}
	c.RuView.AgentAccess = false
	if denied, err := m.Read(ctx, "presence"); err == nil || denied != nil {
		t.Fatalf("presence without agent access: %+v %v", denied, err)
	}
}
