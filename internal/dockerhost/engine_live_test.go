package dockerhost

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"
)

func mustParseQuery(t *testing.T, raw string) url.Values {
	t.Helper()
	values, err := url.ParseQuery(raw)
	if err != nil {
		t.Fatalf("query %q: %v", raw, err)
	}
	return values
}

// writeEventLines sends each line on its own chunk, like the Engine's /events.
func writeEventLines(w http.ResponseWriter, lines ...string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	flusher := w.(http.Flusher)
	flusher.Flush()
	for _, line := range lines {
		_, _ = io.WriteString(w, line+"\n")
		flusher.Flush()
	}
}

const (
	eventStart = `{"Type":"container","Action":"start","Actor":{"ID":"aaaaaaaaaaaa1111","Attributes":{"name":"shop-web-1","image":"nginx:1.27","com.docker.compose.project":"shop"}},"scope":"local","time":1730102400,"timeNano":1730102400123456789}`
	eventDie   = `{"Type":"container","Action":"die","Actor":{"ID":"aaaaaaaaaaaa1111","Attributes":{"name":"shop-web-1","exitCode":"137"}},"scope":"local","time":1730102401,"timeNano":1730102401000000000}`
	eventImage = `{"Type":"image","Action":"pull","Actor":{"ID":"nginx:1.27","Attributes":{"name":"nginx"}},"scope":"local","time":1730102402}`
)

func TestStreamEventsDecodesChunkedLinesAndSendsTheTypeFilter(t *testing.T) {
	fe := newFakeEngine(t)
	var query string
	fe.on(http.MethodGet, "/events", func(w http.ResponseWriter, r *http.Request) {
		query = r.URL.RawQuery
		writeEventLines(w, eventStart, "", eventDie, eventImage)
	})

	var got []EngineEvent
	opened := 0
	err := StreamEventsOpen(context.Background(), fe.client(t), time.Time{}, func() { opened++ }, func(ev EngineEvent) error {
		if opened != 1 {
			t.Errorf("event delivered before the open callback (opened = %d)", opened)
		}
		got = append(got, ev)
		return nil
	})
	if !errors.Is(err, io.EOF) {
		t.Fatalf("err = %v, want an io.EOF-wrapped error when the stream ends", err)
	}
	if len(got) != 3 {
		t.Fatalf("got %d events, want 3 (the blank line is skipped): %+v", len(got), got)
	}
	start := got[0]
	if start.Type != "container" || start.Action != "start" || start.ID != "aaaaaaaaaaaa1111" ||
		start.Attributes["name"] != "shop-web-1" || start.Attributes["com.docker.compose.project"] != "shop" ||
		start.Time.UnixNano() != 1730102400123456789 {
		t.Fatalf("start = %+v", start)
	}
	if got[1].Action != "die" || got[1].Attributes["exitCode"] != "137" {
		t.Fatalf("die = %+v", got[1])
	}
	if got[2].Type != "image" || got[2].Time.Unix() != 1730102402 {
		t.Fatalf("image event = %+v, want the seconds field as the fallback time", got[2])
	}

	values := mustParseQuery(t, query)
	var filters map[string][]string
	if err := json.Unmarshal([]byte(values.Get("filters")), &filters); err != nil {
		t.Fatalf("filters %q: %v", values.Get("filters"), err)
	}
	if strings.Join(filters["type"], ",") != "container,image,volume,network" {
		t.Fatalf("filters = %v, want the four resource types", filters)
	}
	if values.Has("since") {
		t.Fatalf("query %q has since although none was asked for", query)
	}
}

func TestStreamEventsSendsSinceAndStopsWhenTheCallbackFails(t *testing.T) {
	fe := newFakeEngine(t)
	var query string
	fe.on(http.MethodGet, "/events", func(w http.ResponseWriter, r *http.Request) {
		query = r.URL.RawQuery
		writeEventLines(w, eventStart, eventDie)
	})
	stop := errors.New("enough")
	calls := 0
	since := time.Unix(1730102000, 5000)
	err := StreamEvents(context.Background(), fe.client(t), since, func(EngineEvent) error {
		calls++
		return stop
	})
	if !errors.Is(err, stop) || calls != 1 {
		t.Fatalf("err = %v after %d calls, want the callback's error after the first event", err, calls)
	}
	if got := mustParseQuery(t, query).Get("since"); got != "1730102000.000005000" {
		t.Fatalf("since = %q, want unix seconds with nanoseconds", got)
	}
}

func TestStreamEventsEndsWithTheContext(t *testing.T) {
	fe := newFakeEngine(t)
	fe.on(http.MethodGet, "/events", func(w http.ResponseWriter, r *http.Request) {
		writeEventLines(w, eventStart)
		<-r.Context().Done()
	})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	c := fe.client(t)
	got := make(chan string, 1)
	go func() {
		err := StreamEvents(ctx, c, time.Time{}, func(EngineEvent) error {
			cancel()
			return nil
		})
		got <- fmt.Sprint(errors.Is(err, context.Canceled))
	}()
	select {
	case canceled := <-got:
		if canceled != "true" {
			t.Fatalf("StreamEvents did not return the context error")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("StreamEvents kept running after its context ended")
	}
}

func TestStreamEventsReportsAnEngineErrorWithItsStatus(t *testing.T) {
	fe := newFakeEngine(t)
	fe.json(http.MethodGet, "/events", http.StatusForbidden, `{"message":"events are not allowed by the socket proxy"}`)
	err := StreamEvents(context.Background(), fe.client(t), time.Time{}, func(EngineEvent) error { return nil })
	if StatusOf(err) != http.StatusForbidden || !strings.Contains(err.Error(), "not allowed") {
		t.Fatalf("err = %v (status %d), want a *StatusError 403 with the Engine message", err, StatusOf(err))
	}
	if ClassifyError(err) != CodeAuth {
		t.Fatalf("ClassifyError = %q, want auth for a 403", ClassifyError(err))
	}
}

func TestStreamEventsRejectsAnOversizedLine(t *testing.T) {
	fe := newFakeEngine(t)
	fe.on(http.MethodGet, "/events", func(w http.ResponseWriter, r *http.Request) {
		writeEventLines(w, `{"Type":"container","Action":"start","Actor":{"ID":"x","Attributes":{"big":"`+strings.Repeat("a", 2<<20)+`"}}}`)
	})
	err := StreamEvents(context.Background(), fe.client(t), time.Time{}, func(EngineEvent) error { return nil })
	if err == nil || errors.Is(err, io.EOF) {
		t.Fatalf("err = %v, want a line-too-long error that is not a clean end", err)
	}
}

// ---- stats ---------------------------------------------------------------------

// A cgroup v2 sample (Docker 27 on a current kernel): inactive_file is the cache.
const statsV2 = `{"read":"2026-10-28T08:00:03.250000000Z","preread":"2026-10-28T08:00:00.250000000Z",
"pids_stats":{"current":7},
"blkio_stats":{"io_service_bytes_recursive":[{"major":8,"minor":0,"op":"read","value":4096},{"major":8,"minor":0,"op":"write","value":8192},{"major":8,"minor":16,"op":"Read","value":1000},{"major":8,"minor":16,"op":"Total","value":999999}]},
"cpu_stats":{"cpu_usage":{"total_usage":2000000000,"usage_in_kernelmode":500000000,"usage_in_usermode":1500000000},"system_cpu_usage":40000000000,"online_cpus":4},
"precpu_stats":{"cpu_usage":{"total_usage":1900000000},"system_cpu_usage":39000000000,"online_cpus":4},
"memory_stats":{"usage":104857600,"stats":{"anon":50000000,"file":40000000,"inactive_file":20971520,"active_file":19000000},"limit":2147483648},
"name":"/web","id":"aaaaaaaaaaaa1111",
"networks":{"eth0":{"rx_bytes":1000,"tx_bytes":2000},"eth1":{"rx_bytes":10,"tx_bytes":20}}}`

// A cgroup v1 sample: total_inactive_file wins over cache; no online_cpus (old Engines).
const statsV1 = `{"read":"2026-10-28T08:00:03.000000000Z",
"pids_stats":{"current":3},
"cpu_stats":{"cpu_usage":{"total_usage":500,"percpu_usage":[100,100,150,150]},"system_cpu_usage":4000},
"precpu_stats":{"cpu_usage":{"total_usage":400,"percpu_usage":[100,100,100,100]},"system_cpu_usage":3000},
"memory_stats":{"usage":52428800,"stats":{"cache":30000000,"total_cache":30000000,"inactive_file":1,"total_inactive_file":10485760},"limit":1073741824}}`

// Old v1 kernels without inactive_file: the cache counter is the fallback.
const statsV1CacheOnly = `{"read":"2026-10-28T08:00:03Z","memory_stats":{"usage":1000,"stats":{"cache":300},"limit":2000},"cpu_stats":{"cpu_usage":{"total_usage":1}},"precpu_stats":{"cpu_usage":{"total_usage":0}}}`

func TestContainerStatsDecodesCgroupV2AndSendsOneShot(t *testing.T) {
	fe := newFakeEngine(t)
	var queries []string
	fe.on(http.MethodGet, "/containers/web/stats", func(w http.ResponseWriter, r *http.Request) {
		queries = append(queries, r.URL.RawQuery)
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, statsV2)
	})
	c := fe.client(t)

	s, err := ContainerStats(context.Background(), c, "web", true)
	if err != nil {
		t.Fatalf("ContainerStats: %v", err)
	}
	want := RawStats{
		Read: time.Date(2026, 10, 28, 8, 0, 3, 250000000, time.UTC), CPUTotal: 2000000000, SystemCPU: 40000000000, OnlineCPUs: 4,
		PreCPUTotal: 1900000000, PreSystem: 39000000000, MemUsage: 104857600, MemCache: 20971520, MemLimit: 2147483648,
		NetRx: 1010, NetTx: 2020, BlkRead: 5096, BlkWrite: 8192, Pids: 7,
	}
	if !s.Read.Equal(want.Read) {
		t.Fatalf("Read = %v, want %v", s.Read, want.Read)
	}
	s.Read = want.Read
	if s != want {
		t.Fatalf("stats = %+v\nwant   %+v", s, want)
	}

	if _, err := ContainerStats(context.Background(), c, "web", false); err != nil {
		t.Fatalf("ContainerStats: %v", err)
	}
	if len(queries) != 2 || queries[0] != "one-shot=true&stream=false" || queries[1] != "stream=false" {
		t.Fatalf("queries = %q, want one-shot only when asked for", queries)
	}
}

func TestContainerStatsCacheRulesOfCgroupV1(t *testing.T) {
	fe := newFakeEngine(t)
	for name, body := range map[string]string{"v1": statsV1, "old": statsV1CacheOnly} {
		fe.json(http.MethodGet, "/containers/"+name+"/stats", http.StatusOK, body)
	}
	c := fe.client(t)

	v1, err := ContainerStats(context.Background(), c, "v1", false)
	if err != nil {
		t.Fatal(err)
	}
	if v1.MemCache != 10485760 || v1.MemUsage != 52428800 || v1.MemLimit != 1073741824 {
		t.Fatalf("v1 memory = usage %d cache %d limit %d, want total_inactive_file as the cache", v1.MemUsage, v1.MemCache, v1.MemLimit)
	}
	if v1.OnlineCPUs != 4 {
		t.Fatalf("OnlineCPUs = %d, want the length of percpu_usage when online_cpus is missing", v1.OnlineCPUs)
	}
	if v1.CPUTotal != 500 || v1.PreCPUTotal != 400 || v1.SystemCPU != 4000 || v1.PreSystem != 3000 || v1.Pids != 3 {
		t.Fatalf("v1 cpu = %+v", v1)
	}

	old, err := ContainerStats(context.Background(), c, "old", false)
	if err != nil || old.MemCache != 300 {
		t.Fatalf("old = %+v, %v, want the cache counter when no inactive_file is reported", old, err)
	}
}

func TestContainerStatsErrors(t *testing.T) {
	fe := newFakeEngine(t)
	fe.json(http.MethodGet, "/containers/gone/stats", http.StatusNotFound, `{"message":"No such container: gone"}`)
	c := fe.client(t)
	if _, err := ContainerStats(context.Background(), c, "gone", false); StatusOf(err) != http.StatusNotFound {
		t.Fatalf("err = %v (status %d), want a 404 *StatusError", err, StatusOf(err))
	}
	if _, err := ContainerStats(context.Background(), c, "../etc", false); !errors.Is(err, ErrInvalidContainerRef) {
		t.Fatalf("err = %v, want ErrInvalidContainerRef", err)
	}
}

// ---- disk usage -----------------------------------------------------------------

const dfFixture = `{"LayersSize":600,
"Images":[{"Id":"i1","Size":400,"SharedSize":100,"Containers":2},{"Id":"i2","Size":300,"SharedSize":0,"Containers":0},{"Id":"i3","Size":-1,"SharedSize":-1,"Containers":1}],
"Containers":[{"Id":"c1","State":"running","SizeRw":10,"SizeRootFs":400},{"Id":"c2","State":"exited","SizeRw":50,"SizeRootFs":300},{"Id":"c3","State":"created"}],
"Volumes":[{"Name":"v1","UsageData":{"RefCount":1,"Size":1000}},{"Name":"v2","UsageData":{"RefCount":0,"Size":2000}},{"Name":"v3","UsageData":{"RefCount":0,"Size":-1}}],
"BuildCache":[{"ID":"b1","InUse":true,"Shared":false,"Size":7},{"ID":"b2","InUse":false,"Shared":false,"Size":30},{"ID":"b3","InUse":false,"Shared":true,"Size":500}]}`

func TestSystemDFSumsTheFourGroups(t *testing.T) {
	fe := newFakeEngine(t)
	fe.json(http.MethodGet, "/system/df", http.StatusOK, dfFixture)
	df, err := SystemDF(context.Background(), fe.client(t))
	if err != nil {
		t.Fatalf("SystemDF: %v", err)
	}
	want := DiskUsage{
		Images:     DiskUsageGroup{Count: 3, Active: 2, Size: 600, Reclaimable: 300},
		Containers: DiskUsageGroup{Count: 3, Active: 1, Size: 60, Reclaimable: 50},
		Volumes:    DiskUsageGroup{Count: 3, Active: 1, Size: 3000, Reclaimable: 2000},
		BuildCache: DiskUsageGroup{Count: 3, Active: 1, Size: 37, Reclaimable: 30},
		Total:      600 + 60 + 3000 + 37,
	}
	if df != want {
		t.Fatalf("df = %+v\nwant %+v", df, want)
	}
	raw, _ := json.Marshal(df)
	if !strings.Contains(string(raw), `"build_cache":{"count":3`) || !strings.Contains(string(raw), `"total":3697`) {
		t.Fatalf("JSON = %s", raw)
	}
}

func TestSystemDFEmptyEngine(t *testing.T) {
	fe := newFakeEngine(t)
	fe.json(http.MethodGet, "/system/df", http.StatusOK, `{"LayersSize":0,"Images":null,"Containers":null,"Volumes":null,"BuildCache":null}`)
	df, err := SystemDF(context.Background(), fe.client(t))
	if err != nil || df != (DiskUsage{}) {
		t.Fatalf("df = %+v, %v, want zero usage", df, err)
	}
	fe.json(http.MethodGet, "/system/df", http.StatusInternalServerError, `{"message":"boom"}`)
	if _, err := SystemDF(context.Background(), fe.client(t)); StatusOf(err) != http.StatusInternalServerError {
		t.Fatalf("err = %v, want a 500 *StatusError", err)
	}
}
