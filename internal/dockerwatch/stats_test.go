package dockerwatch

import (
	"fmt"
	"math"
	"net/http"
	"strings"
	"testing"
	"time"

	"aurago/internal/dockerhost"
)

// statsBody is a stats sample on a 4-CPU host: cumulative CPU counters, memory
// usage with a cache that must not count, and the previous sample's counters.
func statsBody(cpuTotal, system, preTotal, preSystem, mem, cache uint64) string {
	return fmt.Sprintf(`{"read":"2026-10-28T08:00:03.25Z","pids_stats":{"current":5},`+
		`"cpu_stats":{"cpu_usage":{"total_usage":%d},"system_cpu_usage":%d,"online_cpus":4},`+
		`"precpu_stats":{"cpu_usage":{"total_usage":%d},"system_cpu_usage":%d,"online_cpus":4},`+
		`"memory_stats":{"usage":%d,"stats":{"inactive_file":%d},"limit":1073741824},`+
		`"networks":{"eth0":{"rx_bytes":10,"tx_bytes":20}}}`, cpuTotal, system, preTotal, preSystem, mem, cache)
}

// risingStats answers each call with counters that grow by 1e9 CPU ns (a quarter
// of the system's 4e9 per call, so 100 % of one core on 4 CPUs) and memory that
// grows by 10 per call, so the order of points can be told.
func risingStats(base uint64) func(int) (int, string) {
	return func(call int) (int, string) {
		n := uint64(call)
		return http.StatusOK, statsBody(n*1e9, n*4e9, (n-1)*1e9, (n-1)*4e9, base+n*10+100, 100)
	}
}

func TestCPUPercentAndPointMath(t *testing.T) {
	prev := dockerhost.RawStats{CPUTotal: 1e9, SystemCPU: 4e9}
	cur := dockerhost.RawStats{CPUTotal: 2e9, SystemCPU: 8e9, OnlineCPUs: 4}
	for name, tc := range map[string]struct {
		before  *dockerhost.RawStats
		cur     dockerhost.RawStats
		oneShot bool
		want    float64
	}{
		"one-shot first sample":    {nil, cur, true, -1},
		"one-shot against ours":    {&prev, cur, true, 100},
		"one-shot ignores precpu":  {&prev, dockerhost.RawStats{CPUTotal: 2e9, SystemCPU: 8e9, OnlineCPUs: 2, PreCPUTotal: 1, PreSystem: 1}, true, 50},
		"precpu from the engine":   {nil, dockerhost.RawStats{CPUTotal: 150, SystemCPU: 1000, OnlineCPUs: 2, PreCPUTotal: 100, PreSystem: 900}, false, 100},
		"precpu not filled":        {nil, dockerhost.RawStats{CPUTotal: 150, SystemCPU: 1000, OnlineCPUs: 2}, false, -1},
		"idle container reads 0":   {&prev, dockerhost.RawStats{CPUTotal: 1e9, SystemCPU: 8e9, OnlineCPUs: 4}, true, 0},
		"system clock stood still": {&prev, dockerhost.RawStats{CPUTotal: 2e9, SystemCPU: 4e9, OnlineCPUs: 4}, true, -1},
		"counter went backwards":   {&prev, dockerhost.RawStats{CPUTotal: 5, SystemCPU: 8e9, OnlineCPUs: 4}, true, -1},
		"no online_cpus counts 1":  {&prev, dockerhost.RawStats{CPUTotal: 2e9, SystemCPU: 8e9}, true, 25},
	} {
		if got := cpuPercent(tc.before, tc.cur, tc.oneShot); math.Abs(got-tc.want) > 0.001 {
			t.Errorf("%s: cpuPercent = %v, want %v", name, got, tc.want)
		}
	}

	now := time.Date(2026, 10, 28, 8, 0, 0, 0, time.UTC)
	pt := pointOf(now, dockerhost.RawStats{MemUsage: 1000, MemCache: 300, MemLimit: 5000, NetRx: 1, NetTx: 2, BlkRead: 3, BlkWrite: 4, Pids: 6}, 12.5)
	if pt != (StatsPoint{T: now.UnixMilli(), CPU: 12.5, Mem: 700, MemLimit: 5000, NetRx: 1, NetTx: 2, BlkR: 3, BlkW: 4, Pids: 6}) {
		t.Fatalf("point = %+v, want memory minus the cache", pt)
	}
	if pt := pointOf(now, dockerhost.RawStats{MemUsage: 100, MemCache: 300}, 0); pt.Mem != 100 {
		t.Fatalf("Mem = %d, want the usage when the cache exceeds it", pt.Mem)
	}

	r := &ring{}
	for i := 0; i < RingSize+15; i++ {
		r.add(StatsPoint{T: int64(i)}, now)
	}
	if len(r.points) != RingSize || r.points[0].T != 15 || r.points[RingSize-1].T != RingSize+14 {
		t.Fatalf("ring holds %d points from %d to %d, want the newest %d", len(r.points), r.points[0].T, r.points[len(r.points)-1].T, RingSize)
	}
}

func TestStatsAreSampledPerSubscriptionWithHistoryForLateClients(t *testing.T) {
	env := newTestEnv(t, nil)
	env.fe.set(func(fe *fakeEngine) { fe.stats["aaaaaaaaaaaa"] = risingStats(1000) })
	a := env.attach(t, Subscription{Stats: map[string][]string{"h1": {"aaaaaaaaaaaa1111"}}})

	first := waitFor(t, a, "the first stats point", func(m Message) bool { return m.Event == MsgStats }).Data.(StatsMsg)
	second := waitFor(t, a, "the second stats point", func(m Message) bool { return m.Event == MsgStats }).Data.(StatsMsg)
	if first.Host != "h1" || first.ID != "aaaaaaaaaaaa" || first.Point.CPU != -1 {
		t.Fatalf("first = %+v, want an unknown (-1) CPU for the first one-shot sample", first)
	}
	if math.Abs(second.Point.CPU-100) > 0.01 || second.Point.Mem != 1000+20+100-100 || second.Point.MemLimit != 1073741824 ||
		second.Point.NetRx != 10 || second.Point.NetTx != 20 || second.Point.Pids != 5 {
		t.Fatalf("second = %+v, want 100 %% CPU and memory without the cache", second.Point)
	}
	env.fe.mu.Lock()
	queries := append([]string(nil), env.fe.statsQueries...)
	env.fe.mu.Unlock()
	if queries[0] != "one-shot=true&stream=false" {
		t.Fatalf("stats query = %q, want one-shot on API 1.45", queries[0])
	}

	// A client that asks later gets the ring first, oldest point first, then live points.
	b := env.attach(t, Subscription{Stats: map[string][]string{"h1": {"aaaaaaaaaaaa"}}})
	m := next(t, b, 2*time.Second)
	history, ok := m.Data.(StatsHistoryMsg)
	if m.Event != MsgStatsHistory || !ok || history.ID != "aaaaaaaaaaaa" || len(history.Points) < 2 {
		t.Fatalf("first message = %+v, want stats_history with the points so far", m)
	}
	for i := 1; i < len(history.Points); i++ {
		if history.Points[i].Mem <= history.Points[i-1].Mem {
			t.Fatalf("history is not oldest first: %+v", history.Points)
		}
	}
	live := waitFor(t, b, "a live point", func(m Message) bool { return m.Event == MsgStats }).Data.(StatsMsg)
	if live.Point.Mem <= history.Points[len(history.Points)-1].Mem {
		t.Fatalf("live point %+v is not newer than the history %+v", live.Point, history.Points)
	}

	// Only asked-for containers are sampled.
	if env.fe.calls("bbbbbbbbbbbb") != 0 {
		t.Fatal("a container nobody asked for was sampled")
	}
}

func TestStatsUsePrecpuBelowAPI141(t *testing.T) {
	env := newTestEnv(t, nil)
	env.fe.set(func(fe *fakeEngine) {
		fe.api = "1.40"
		// The Engine fills precpu_stats on stream=false: 100 of 400 system ticks on 4 CPUs = 100 %.
		fe.stats["aaaaaaaaaaaa"] = func(int) (int, string) { return http.StatusOK, statsBody(200, 800, 100, 400, 500, 0) }
	})
	a := env.attach(t, Subscription{Stats: map[string][]string{"h1": {"aaaaaaaaaaaa"}}})
	first := waitFor(t, a, "the first stats point", func(m Message) bool { return m.Event == MsgStats }).Data.(StatsMsg)
	if math.Abs(first.Point.CPU-100) > 0.01 {
		t.Fatalf("first CPU = %v, want 100 from precpu_stats on the first sample", first.Point.CPU)
	}
	env.fe.mu.Lock()
	query := env.fe.statsQueries[0]
	env.fe.mu.Unlock()
	if query != "stream=false" {
		t.Fatalf("stats query = %q, want no one-shot below API 1.41", query)
	}
}

func TestStatsGoneStopsSamplingUntilTheContainerStartsAgain(t *testing.T) {
	env := newTestEnv(t, nil)
	// No generator: the Engine answers 404.
	a := env.attach(t, Subscription{Hosts: []string{"h1"}, Stats: map[string][]string{"h1": {"aaaaaaaaaaaa"}}})
	m := waitFor(t, a, "stats_gone", func(m Message) bool { return m.Event == MsgStatsGone })
	if got := m.Data.(StatsGoneMsg); got != (StatsGoneMsg{Host: "h1", ID: "aaaaaaaaaaaa"}) {
		t.Fatalf("stats_gone = %+v", got)
	}
	calls := env.fe.calls("aaaaaaaaaaaa")
	time.Sleep(100 * time.Millisecond)
	if env.fe.calls("aaaaaaaaaaaa") != calls {
		t.Fatalf("stats calls grew from %d to %d after stats_gone", calls, env.fe.calls("aaaaaaaaaaaa"))
	}

	// The container comes back: a start event resumes the sampling.
	env.fe.set(func(fe *fakeEngine) { fe.stats["aaaaaaaaaaaa"] = risingStats(1000) })
	waitUntil(t, "the live feed", func() bool { return env.hub.status("h1").State == StateLive })
	env.fe.send(t, eventLine("container", "start", webID, map[string]string{"name": "web"}))
	waitFor(t, a, "a stats point after the start", func(m Message) bool { return m.Event == MsgStats })

	// The Engine answers 200 with an all-zero sample for a container that is not running.
	env.fe.set(func(fe *fakeEngine) {
		fe.stats["bbbbbbbbbbbb"] = func(int) (int, string) {
			return http.StatusOK, `{"read":"0001-01-01T00:00:00Z","cpu_stats":{"cpu_usage":{"total_usage":0}},"memory_stats":{}}`
		}
	})
	if _, err := a.Set(Subscription{Stats: map[string][]string{"h1": {"bbbbbbbbbbbb"}}}); err != nil {
		t.Fatal(err)
	}
	m = waitFor(t, a, "stats_gone for the stopped container", func(m Message) bool { return m.Event == MsgStatsGone })
	if m.Data.(StatsGoneMsg).ID != "bbbbbbbbbbbb" {
		t.Fatalf("stats_gone = %+v, want the stopped container", m.Data)
	}
}

func TestStatsCallsInFlightPerHostAreCapped(t *testing.T) {
	env := newTestEnv(t, nil)
	ids := make([]string, 12)
	env.fe.set(func(fe *fakeEngine) {
		fe.statsDelay = 15 * time.Millisecond
		for i := range ids {
			ids[i] = fmt.Sprintf("%012x", i+1)
			fe.stats[ids[i]] = risingStats(1000)
		}
	})
	a := env.attach(t, Subscription{Stats: map[string][]string{"h1": ids}})
	seen := map[string]bool{}
	for len(seen) < len(ids) {
		m := waitFor(t, a, "a stats point of every container", func(m Message) bool { return m.Event == MsgStats })
		seen[m.Data.(StatsMsg).ID] = true
	}
	env.fe.mu.Lock()
	peak := env.fe.maxInflight
	env.fe.mu.Unlock()
	if peak < 2 || peak > 4 {
		t.Fatalf("peak stats calls in flight = %d, want parallel but at most 4", peak)
	}
}

func TestStatsRingsAreFreedWhenNobodySamplesThem(t *testing.T) {
	env := newTestEnv(t, nil)
	env.fe.set(func(fe *fakeEngine) { fe.stats["aaaaaaaaaaaa"] = risingStats(1000) })
	a := env.attach(t, Subscription{Stats: map[string][]string{"h1": {"aaaaaaaaaaaa"}}})
	waitFor(t, a, "a stats point", func(m Message) bool { return m.Event == MsgStats })

	if _, err := a.Set(Subscription{}); err != nil {
		t.Fatal(err)
	}
	time.Sleep(60 * time.Millisecond)
	env.hub.mu.Lock()
	kept := len(env.hub.rts["h1"].st.rings)
	env.hub.mu.Unlock()
	if kept != 1 {
		t.Fatalf("rings after the last unsubscribe = %d, want the ring kept for a quick return", kept)
	}
	env.clock.Advance(ringTTL + time.Second)
	waitUntil(t, "the idle ring and its loop to be freed", func() bool {
		env.hub.mu.Lock()
		defer env.hub.mu.Unlock()
		return len(env.hub.rts) == 0
	})

	// A later subscriber starts from nothing: no history.
	b := env.attach(t, Subscription{Stats: map[string][]string{"h1": {"aaaaaaaaaaaa"}}})
	if m := next(t, b, 2*time.Second); m.Event == MsgStatsHistory {
		t.Fatalf("got %+v, want live points only after the ring was freed", m)
	}
}

// ---- host gauges ------------------------------------------------------------------

const hostContainers = `[
{"Id":"aaaaaaaaaaaa1111","Names":["/x"],"State":"running","Status":"Up 1 hour","Labels":{}},
{"Id":"bbbbbbbbbbbb2222","Names":["/y"],"State":"running","Status":"Up 1 hour","Labels":{}},
{"Id":"cccccccccccc3333","Names":["/z"],"State":"exited","Status":"Exited (0) 1 hour ago","Labels":{}}]`

func TestHostStatsSumRunningContainersAndShareThePointsOfSubscribedOnes(t *testing.T) {
	// A frozen clock keeps the subscribed container's point "fresh" for the gauges.
	env := newTestEnv(t, func(o *Options) { o.StatsInterval = time.Hour })
	env.fe.set(func(fe *fakeEngine) {
		fe.containers = hostContainers
		fe.stats["aaaaaaaaaaaa"] = risingStats(100 << 20)
		fe.stats["bbbbbbbbbbbb"] = risingStats(200 << 20)
		fe.stats["cccccccccccc"] = risingStats(1)
	})
	a := env.attach(t, Subscription{Stats: map[string][]string{"h1": {"aaaaaaaaaaaa"}}})
	waitFor(t, a, "x's first point", func(m Message) bool { return m.Event == MsgStats })

	if _, err := a.Set(Subscription{Stats: map[string][]string{"h1": {"aaaaaaaaaaaa"}}, HostStats: []string{"h1"}}); err != nil {
		t.Fatal(err)
	}
	// The first gauge cycle has no CPU deltas yet; the second follows after a short priming delay.
	first := waitFor(t, a, "the first host_stats", func(m Message) bool { return m.Event == MsgHostStats }).Data.(HostStats)
	if first.CPU != -1 || first.Running != 2 || first.Sampled != 2 || first.Partial {
		t.Fatalf("first = %+v, want unknown CPU, 2 running containers, both sampled", first)
	}
	second := waitFor(t, a, "the second host_stats", func(m Message) bool { return m.Event == MsgHostStats }).Data.(HostStats)
	// y: 100 % of one core; x is a single sample (unknown). 100 / 4 CPUs = 25 %.
	if second.CPU != 25 || second.NCPU != 4 || second.MemTotal != 8<<30 || second.Running != 2 || second.Sampled != 2 || second.Partial {
		t.Fatalf("second = %+v, want 25 %% of the 4-CPU host", second)
	}
	// x: 100 MiB + 10 ... y: 200 MiB + 20 ... (cache already subtracted).
	if second.MemUsed < 300<<20 || second.MemUsed > 301<<20 || second.Host != "h1" || second.T == 0 {
		t.Fatalf("MemUsed = %d, want about 300 MiB: %+v", second.MemUsed, second)
	}
	if env.fe.calls("aaaaaaaaaaaa") != 1 {
		t.Fatalf("x was sampled %d times, want once (by its own subscription): the gauge reuses its newest point", env.fe.calls("aaaaaaaaaaaa"))
	}
	if env.fe.calls("cccccccccccc") != 0 {
		t.Fatal("a stopped container was sampled for the gauges")
	}
	env.fe.mu.Lock()
	infoCalls, listCalls := env.fe.infoCalls, env.fe.listCalls
	env.fe.mu.Unlock()
	if infoCalls != 1 || listCalls < 2 {
		t.Fatalf("info calls %d, list calls %d, want /info cached across cycles", infoCalls, listCalls)
	}

	// The cached /info expires after five minutes.
	env.clock.Advance(infoTTL + time.Second)
	waitUntil(t, "/info to be read again", func() bool {
		env.fe.mu.Lock()
		defer env.fe.mu.Unlock()
		return env.fe.infoCalls == 2
	})

	// Without a gauge subscription the loop stops sampling.
	if _, err := a.Set(Subscription{}); err != nil {
		t.Fatal(err)
	}
	time.Sleep(60 * time.Millisecond)
	before := env.fe.calls("bbbbbbbbbbbb")
	time.Sleep(120 * time.Millisecond)
	if env.fe.calls("bbbbbbbbbbbb") != before {
		t.Fatal("the gauge loop kept sampling after the last gauge subscriber left")
	}
}

func TestHostStatsSampleAtMost100ContainersAndSayPartial(t *testing.T) {
	env := newTestEnv(t, nil)
	var list []string
	env.fe.set(func(fe *fakeEngine) {
		for i := 0; i < MaxHostSample+1; i++ {
			id := fmt.Sprintf("%012x", i+1)
			list = append(list, fmt.Sprintf(`{"Id":"%s%s","Names":["/c%d"],"State":"running","Status":"Up","Labels":{}}`, id, strings.Repeat("0", 52), i))
			fe.stats[id] = risingStats(1000)
		}
		fe.containers = "[" + strings.Join(list, ",") + "]"
	})
	a := env.attach(t, Subscription{HostStats: []string{"h1"}})
	got := waitFor(t, a, "host_stats", func(m Message) bool { return m.Event == MsgHostStats }).Data.(HostStats)
	if got.Running != MaxHostSample+1 || got.Sampled != MaxHostSample || !got.Partial {
		t.Fatalf("host_stats = %+v, want 101 running, 100 sampled, partial", got)
	}
}

func TestGaugeLoopForgetsThePreviousSamplesOfVanishedContainers(t *testing.T) {
	env := newTestEnv(t, nil)
	env.fe.set(func(fe *fakeEngine) {
		fe.containers = hostContainers
		fe.stats["aaaaaaaaaaaa"] = risingStats(100 << 20)
		fe.stats["bbbbbbbbbbbb"] = risingStats(200 << 20)
	})
	a := env.attach(t, Subscription{HostStats: []string{"h1"}})
	waitFor(t, a, "a host_stats", func(m Message) bool { return m.Event == MsgHostStats })
	raws := func() int {
		env.hub.mu.Lock()
		defer env.hub.mu.Unlock()
		return len(env.hub.rts["h1"].st.raws)
	}
	if raws() != 2 {
		t.Fatalf("previous samples kept = %d, want one per sampled container", raws())
	}

	// Both containers go away; five minutes later the gauge loop has dropped what it kept of them.
	env.fe.set(func(fe *fakeEngine) { fe.containers = "[]" })
	env.clock.Advance(ringTTL + time.Second)
	waitUntil(t, "the old samples to be dropped", func() bool { return raws() == 0 })
}
