package server

import (
	"encoding/json"
	"net/http"
	"net/url"
	"sync"
	"testing"
	"time"

	"aurago/internal/config"
	"aurago/internal/homedash"
	"aurago/internal/homedash/sources/system"
)

// The M2 definition of done end to end: the starter dashboard binds the six system entities, a
// stream shows them with real meta and values, a new metrics sample arrives as a state frame and the
// history (served from the collector ring, origin "source") fills.
func TestHomeDashSystemStarterEndToEnd(t *testing.T) {
	var mu sync.Mutex
	t0 := time.Now().UTC().Truncate(time.Second)
	sample := system.Snapshot{SampledAt: t0, CPUPercent: 21, MemoryPercent: 40, DiskPercent: 50,
		CPUOK: true, MemoryOK: true, DiskOK: true, NetworkOK: true, NetBytesRecv: 1000, NetBytesSent: 1000, BootTime: t0.Add(-time.Hour)}
	read := func() (system.Snapshot, bool) { mu.Lock(); defer mu.Unlock(); return sample, true }

	// The history ring is fed like the collector does: one record per tick.
	var ring homeDashMetricsRing
	tick := func(s system.Snapshot) {
		ring.record(systemMetricsSnapshot{
			CPUPercent: s.CPUPercent, MemoryPercent: s.MemoryPercent, DiskPercent: s.DiskPercent,
			CPUAvailable: s.CPUOK, MemoryAvailable: s.MemoryOK, DiskAvailable: s.DiskOK,
			SampledAt: s.SampledAt, NetBytesRecv: s.NetBytesRecv, NetBytesSent: s.NetBytesSent, NetworkAvailable: s.NetworkOK,
		})
	}
	tick(sample)

	spec := homeDashSourceSpec{ID: system.SourceID, ConfigKey: "system", Settings: homeDashSystemSettings, Factory: system.Factory(read, ring.between)}
	env := newHomeDashTestEnv(t, func(c *config.Config) { c.HomeDash.Sources.System.PollSeconds = 2 }, spec)

	w := env.do(t, env.admin, http.MethodPost, "dashboards", map[string]any{"template": "system"})
	var created hdSaved
	hdDecode(t, w, &created)
	if w.Code != http.StatusCreated || len(created.Warnings) != 0 {
		t.Fatalf("template = %d %s (system entities must be known: no warnings)", w.Code, w.Body.String())
	}
	ids := homeDashBoundIDs(created.Dashboard)
	if len(ids) != 6 {
		t.Fatalf("bound ids = %v", ids)
	}

	id, events, stop := openHomeDashStream(t, env, env.admin)
	defer stop()
	names := make([]string, len(ids))
	for i, e := range ids {
		names[i] = string(e)
	}
	if w := env.do(t, env.admin, http.MethodPut, "stream/subscriptions", map[string]any{"stream_id": id, "entities": names}); w.Code != http.StatusOK {
		t.Fatalf("PUT = %d %s", w.Code, w.Body.String())
	}
	// The writer may drain mid-Set, so the snapshot can arrive as several frames; collect all six.
	var snap struct {
		Entities []homedash.Entity `json:"entities"`
	}
	for frames := 0; len(snap.Entities) < 6; frames++ {
		if frames > 6 {
			t.Fatalf("snapshot frames never reached 6 entities: %+v", snap)
		}
		var part struct {
			Entities []homedash.Entity `json:"entities"`
		}
		if err := json.Unmarshal([]byte(waitHomeDashEvent(t, events, "snapshot").data), &part); err != nil {
			t.Fatal(err)
		}
		snap.Entities = append(snap.Entities, part.Entities...)
	}
	if len(snap.Entities) != 6 {
		t.Fatalf("snapshot = %+v", snap)
	}
	var cpu homedash.Entity
	for _, e := range snap.Entities {
		if e.ID == "system:cpu" {
			cpu = e
		}
	}
	if cpu.Name != "CPU" || cpu.Unit != "%" || !cpu.State.Available || cpu.State.Value != 21.0 {
		t.Fatalf("system:cpu = %+v", cpu)
	}

	mu.Lock()
	sample.SampledAt, sample.CPUPercent = t0.Add(10*time.Second), 55
	sample.NetBytesRecv, sample.NetBytesSent = 2_000_000, 1_500_000
	next := sample
	mu.Unlock()
	tick(next)
	for got, frames := 0.0, 0; got != 55; frames++ {
		if frames > 6 {
			t.Fatal("no state frame with the new CPU value")
		}
		var st struct {
			Changes map[string]homedash.State `json:"changes"`
		}
		if err := json.Unmarshal([]byte(waitHomeDashEvent(t, events, "state").data), &st); err != nil {
			t.Fatal(err)
		}
		if v, ok := st.Changes["system:cpu"].Value.(float64); ok {
			got = v
		}
	}

	from, to := url.QueryEscape(t0.Add(-time.Hour).Format(time.RFC3339)), url.QueryEscape(t0.Add(time.Hour).Format(time.RFC3339))
	for _, entity := range []string{"system:net_rx", "system:cpu"} {
		w := env.do(t, env.admin, http.MethodGet, "history?id="+entity+"&from="+from+"&to="+to, nil)
		if w.Code != http.StatusOK {
			t.Fatalf("history %s = %d %s", entity, w.Code, w.Body.String())
		}
		var raw map[string]any
		if err := json.Unmarshal(w.Body.Bytes(), &raw); err != nil {
			t.Fatal(err)
		}
		points, _ := raw["points"].([]any)
		if raw["origin"] != "source" || len(points) == 0 {
			t.Fatalf("history %s = %s", entity, w.Body.String())
		}
		if _, has := raw["since"]; has {
			t.Fatalf("history %s must carry no since key: %s", entity, w.Body.String())
		}
	}
}
