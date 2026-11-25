package server

import (
	"bufio"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"aurago/internal/config"
	"aurago/internal/homedash"
	"aurago/internal/homedash/sourcetest"
)

type hdEvent struct{ name, data string }

// openHomeDashStream opens GET stream on a real HTTP server, waits for hello and returns its id.
func openHomeDashStream(t *testing.T, env *homeDashTestEnv, token string) (string, <-chan hdEvent, func()) {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc(homeDashAPIPrefix, handleHomeDash(env.s))
	srv := httptest.NewServer(mux)
	req, _ := http.NewRequest(http.MethodGet, srv.URL+homeDashAPIPrefix+"stream", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := srv.Client().Do(req)
	if err != nil || resp.StatusCode != http.StatusOK || resp.Header.Get("Content-Type") != "text/event-stream" {
		srv.Close()
		t.Fatalf("stream = %v %v", resp, err)
	}
	events := make(chan hdEvent, 64)
	go func() {
		defer close(events)
		sc := bufio.NewScanner(resp.Body)
		sc.Buffer(make([]byte, 0, 64<<10), 1<<20)
		var ev hdEvent
		for sc.Scan() {
			line := sc.Text()
			switch {
			case line == "":
				if ev.name != "" {
					events <- ev
				}
				ev = hdEvent{}
			case strings.HasPrefix(line, "event: "):
				ev.name = strings.TrimPrefix(line, "event: ")
			case strings.HasPrefix(line, "data: "):
				ev.data = strings.TrimPrefix(line, "data: ")
			}
		}
	}()
	hello := waitHomeDashEvent(t, events, "hello")
	var h struct {
		StreamID    string `json:"stream_id"`
		Protocol    int    `json:"protocol"`
		HeartbeatMS int64  `json:"heartbeat_ms"`
	}
	if err := json.Unmarshal([]byte(hello.data), &h); err != nil || len(h.StreamID) != 32 || h.Protocol != 1 || h.HeartbeatMS <= 0 {
		t.Fatalf("hello = %s", hello.data)
	}
	return h.StreamID, events, func() { resp.Body.Close(); srv.Close() }
}

func waitHomeDashEvent(t *testing.T, events <-chan hdEvent, name string) hdEvent {
	t.Helper()
	deadline := time.After(3 * time.Second)
	for {
		select {
		case ev, ok := <-events:
			if !ok {
				t.Fatalf("stream ended before %q", name)
			}
			if ev.name == name {
				return ev
			}
		case <-deadline:
			t.Fatalf("no %q event", name)
		}
	}
}

func shortenHomeDashStreamTimers(t *testing.T) {
	hb, re := homeDashHeartbeat, homeDashReauthEvery
	homeDashHeartbeat, homeDashReauthEvery = 100*time.Millisecond, 20*time.Millisecond
	t.Cleanup(func() { homeDashHeartbeat, homeDashReauthEvery = hb, re })
}

func TestHomeDashStreamHelloSnapshotStateHeartbeat(t *testing.T) {
	shortenHomeDashStreamTimers(t)
	fake := sourcetest.New("fake", sourcetest.Entity("fake:a", homedash.KindSensor, homedash.CapabilitySpec{Name: homedash.CapNumeric}))
	env := newHomeDashTestEnv(t, nil, fakeHomeDashSpec(fake))
	id, events, stop := openHomeDashStream(t, env, env.admin)
	defer stop()

	w := env.do(t, env.admin, http.MethodPut, "stream/subscriptions", map[string]any{"stream_id": id, "entities": []string{"fake:a", "bad id", "nope:x"}})
	var put struct {
		Accepted []string             `json:"accepted"`
		Rejected []homedash.Rejection `json:"rejected"`
	}
	hdDecode(t, w, &put)
	if w.Code != http.StatusOK || len(put.Accepted) != 1 || put.Accepted[0] != "fake:a" || len(put.Rejected) != 2 ||
		put.Rejected[0].Reason != "invalid_id" || put.Rejected[1].Reason != "unknown_source" {
		t.Fatalf("PUT = %d %s", w.Code, w.Body.String())
	}
	var snap struct {
		Entities []homedash.Entity       `json:"entities"`
		Sources  []homedash.SourceStatus `json:"sources"`
	}
	if err := json.Unmarshal([]byte(waitHomeDashEvent(t, events, "snapshot").data), &snap); err != nil {
		t.Fatal(err)
	}
	if len(snap.Entities) != 1 || snap.Entities[0].ID != "fake:a" || snap.Entities[0].Kind != homedash.KindSensor || snap.Sources == nil {
		t.Fatalf("snapshot = %+v (the catalog must be prefetched: no generic placeholder)", snap)
	}
	sourcetest.WaitFor(t, 2*time.Second, func() bool { return fake.Running() == 1 }, "source running")
	for _, v := range []float64{1, 2, 3} {
		fake.Push(homedash.StateUpdate{Native: "a", State: homedash.State{Value: v, Available: true}})
	}
	for got := 0.0; got != 3; {
		var st struct {
			Changes map[string]homedash.State `json:"changes"`
		}
		if err := json.Unmarshal([]byte(waitHomeDashEvent(t, events, "state").data), &st); err != nil {
			t.Fatal(err)
		}
		if v, ok := st.Changes["fake:a"].Value.(float64); ok {
			got = v
		}
	}
	waitHomeDashEvent(t, events, "hb")

	w = env.do(t, env.admin, http.MethodPost, "dashboards", map[string]any{"template": "system"})
	var created hdSaved
	hdDecode(t, w, &created)
	var changed homedash.DashboardEvent
	if err := json.Unmarshal([]byte(waitHomeDashEvent(t, events, "dashboard_changed").data), &changed); err != nil ||
		changed.ID != created.Dashboard.ID || changed.Revision != 1 || changed.ApprovedRevision != 1 {
		t.Fatalf("dashboard_changed = %+v %v", changed, err)
	}
}

func TestHomeDashStreamEndsWithRevoked(t *testing.T) {
	shortenHomeDashStreamTimers(t)
	fake := sourcetest.New("fake", sourcetest.Entity("fake:a", homedash.KindSensor))
	env := newHomeDashTestEnv(t, nil, fakeHomeDashSpec(fake))

	token, meta, err := env.s.TokenManager.Create("stream admin", []string{desktopScopeAdmin}, nil)
	if err != nil {
		t.Fatal(err)
	}
	_, events, stop := openHomeDashStream(t, env, token)
	defer stop()
	if err := env.s.TokenManager.Delete(meta.ID); err != nil {
		t.Fatal(err)
	}
	if ev := waitHomeDashEvent(t, events, "revoked"); ev.data != `{"reason":"auth_lost"}` {
		t.Fatalf("revoked = %s", ev.data)
	}

	_, events2, stop2 := openHomeDashStream(t, env, env.admin)
	defer stop2()
	cfg := *env.s.ConfigSnapshot()
	cfg.HomeDash.Enabled = false
	env.s.CfgMu.Lock()
	env.s.replaceConfigSnapshot(&cfg)
	env.s.CfgMu.Unlock()
	if ev := waitHomeDashEvent(t, events2, "revoked"); ev.data != `{"reason":"disabled"}` {
		t.Fatalf("revoked = %s", ev.data)
	}
}

func TestHomeDashSubscriptionsAndStreamLimits(t *testing.T) {
	fake := sourcetest.New("fake", sourcetest.Entity("fake:a", homedash.KindSensor))
	env := newHomeDashTestEnv(t, func(c *config.Config) {
		c.VirtualDesktop.ReadOnly = true // streams and subscriptions are reads
		c.HomeDash.Limits.MaxStreams = 4
	}, fakeHomeDashSpec(fake))
	if w := env.do(t, env.admin, http.MethodPut, "stream/subscriptions", map[string]any{"stream_id": "00000000000000000000000000000000", "entities": []string{}}); w.Code != http.StatusNotFound || hdCode(t, w) != "stream_not_found" {
		t.Fatalf("unknown stream = %d %s", w.Code, w.Body.String())
	}
	var stops []func()
	defer func() {
		for _, s := range stops {
			s()
		}
	}()
	var first string
	for i := 0; i < 4; i++ {
		id, _, stop := openHomeDashStream(t, env, env.admin)
		stops = append(stops, stop)
		if i == 0 {
			first = id
		}
	}
	if w := env.do(t, env.admin, http.MethodPut, "stream/subscriptions", map[string]any{"stream_id": first, "entities": []string{"fake:a"}}); w.Code != http.StatusOK {
		t.Fatalf("PUT on a read-only desktop = %d %s", w.Code, w.Body.String())
	}
	if w := env.do(t, env.read, http.MethodPut, "stream/subscriptions", map[string]any{"stream_id": first, "entities": []string{}}); w.Code != http.StatusForbidden {
		t.Fatalf("desktop:read PUT = %d", w.Code)
	}
	if w := env.do(t, env.admin, http.MethodGet, "stream", nil); w.Code != http.StatusTooManyRequests || hdCode(t, w) != "too_many_streams" {
		t.Fatalf("fifth stream = %d %s", w.Code, w.Body.String())
	}
}
