package ui

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/input"
	"github.com/gorilla/websocket"
)

// dockerManagerLive is the live half of the Docker Manager fixture (dockerManagerFixture in desktop_docker_manager_browser_test.go
// routes /stream, the log follow streams, /df and the terminal here): a hand-written SSE stream with hello, host_status, stats and
// host_stats like the hub sends them, follow streams for a container, a stack and a project, and a terminal that echoes over a
// real WebSocket (gorilla), so the browser tests see the same wire format as the server will produce.
type dockerManagerLive struct {
	mu              sync.Mutex
	out             chan string
	drop            chan struct{}
	streams         int
	subscriptions   []string
	followers       map[chan string]string
	followURIs      []string
	terminalURIs    []string
	terminalInput   []string
	terminalResizes []string
	terminalEnded   int
	dfURIs          []string
	lastLog         map[string]any    // the newest line pushed, repeated by a reconnect
	hostState       map[string]string // the state the test gave a host; a subscription is answered with it, like the hub does
}

var dockerManagerUpgrader = websocket.Upgrader{CheckOrigin: func(*http.Request) bool { return true }}

var dockerManagerLogBase = time.Date(2026, 10, 28, 7, 33, 29, 0, time.UTC)

func dockerManagerLine(offset int, stream, text string, extra map[string]any) map[string]any {
	line := map[string]any{"t": dockerManagerLogBase.Add(time.Duration(offset) * time.Second).Format(time.RFC3339Nano), "s": stream, "text": text, "trunc": false}
	for key, value := range extra {
		line[key] = value
	}
	return line
}

func dockerManagerStatsPoints(cpu float64) []map[string]any {
	now := time.Now().UnixMilli()
	points := []map[string]any{}
	for i, age := range []int64{6000, 3000, 0} {
		points = append(points, map[string]any{"t": now - age, "cpu": cpu, "mem": 100*1048576 + i*1048576, "mem_limit": 2147483648, "net_rx": 1000 * (i + 1), "net_tx": 500 * (i + 1), "blk_r": 0, "blk_w": 0, "pids": 5})
	}
	return points
}

func (f *dockerManagerFixture) serveLive(w http.ResponseWriter, r *http.Request, path string, raw []byte) bool {
	switch {
	case r.Method == http.MethodGet && path == "/stream":
		f.serveStream(w, r)
	case r.Method == http.MethodPut && path == "/stream/subscriptions":
		f.serveSubscriptions(w, raw)
	case r.Method == http.MethodGet && path == "/hosts/local/df":
		f.live.mu.Lock()
		f.live.dfURIs = append(f.live.dfURIs, r.URL.RequestURI())
		f.live.mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"status": "ok", "fetched_at": time.Now().UTC().Format(time.RFC3339), "cached": false, "df": map[string]any{
			"images": map[string]any{"count": 3, "active": 2, "size": 1000000000, "reclaimable": 200000000}, "containers": map[string]any{"count": 2, "active": 2, "size": 10000000, "reclaimable": 0},
			"volumes": map[string]any{"count": 1, "active": 1, "size": 500000000, "reclaimable": 0}, "build_cache": map[string]any{"count": 0, "active": 0, "size": 100000000, "reclaimable": 0}, "total": 1610000000,
		}})
	case r.Method == http.MethodGet && strings.HasSuffix(path, "/logs") && r.URL.Query().Get("follow") == "1":
		f.serveFollow(w, r, path)
	case r.Method == http.MethodGet && strings.HasPrefix(path, "/hosts/local/containers/") && strings.HasSuffix(path, "/terminal"):
		f.serveTerminal(w, r, path)
	default:
		return false
	}
	return true
}

func (f *dockerManagerFixture) serveStream(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	flusher, _ := w.(http.Flusher)
	out, drop := make(chan string, 64), make(chan struct{})
	f.live.mu.Lock()
	f.live.out, f.live.drop = out, drop
	f.live.streams++
	id := strings.Repeat(string(rune('a'+f.live.streams-1)), 32)
	f.live.mu.Unlock()
	fmt.Fprintf(w, "event: hello\ndata: {\"stream_id\":%q,\"heartbeat_ms\":15000,\"limits\":{\"stats_ids\":50,\"hosts\":20}}\n\n", id)
	flusher.Flush()
	for {
		select {
		case chunk := <-out:
			_, _ = io.WriteString(w, chunk)
			flusher.Flush()
		case <-drop:
			return
		case <-r.Context().Done():
			return
		case <-time.After(15 * time.Second):
			_, _ = io.WriteString(w, ": heartbeat\n\n")
			flusher.Flush()
		}
	}
}

// pushStream sends one message on the open /stream (dropped when nobody listens).
func (f *dockerManagerFixture) pushStream(event string, data any) {
	payload, _ := json.Marshal(data)
	f.live.mu.Lock()
	out := f.live.out
	f.live.mu.Unlock()
	if out == nil {
		return
	}
	select {
	case out <- fmt.Sprintf("event: %s\ndata: %s\n\n", event, payload):
	default:
	}
}

// setHostStatus changes the state of a host and tells the open stream; later subscriptions are answered with the same state.
func (f *dockerManagerFixture) setHostStatus(host, state string) {
	f.live.mu.Lock()
	if f.live.hostState == nil {
		f.live.hostState = map[string]string{}
	}
	f.live.hostState[host] = state
	f.live.mu.Unlock()
	f.pushStream("host_status", map[string]any{"host": host, "state": state, "reason": "", "t": time.Now().UnixMilli()})
}

// dropStream ends the open /stream from the server side; the app has to reconnect and subscribe again.
func (f *dockerManagerFixture) dropStream() {
	f.live.mu.Lock()
	drop := f.live.drop
	f.live.mu.Unlock()
	if drop != nil {
		close(drop)
	}
}

// serveSubscriptions answers like the hub: ok, then the current status of every host, the history of every container asked for
// and the first host stats of the host whose gauges show.
func (f *dockerManagerFixture) serveSubscriptions(w http.ResponseWriter, raw []byte) {
	var in struct {
		Hosts     []string            `json:"hosts"`
		Stats     map[string][]string `json:"stats"`
		HostStats []string            `json:"host_stats"`
	}
	_ = json.Unmarshal(raw, &in)
	f.live.mu.Lock()
	f.live.subscriptions = append(f.live.subscriptions, string(raw))
	f.live.mu.Unlock()
	w.Header().Set("Content-Type", "application/json")
	count := 0
	for _, ids := range in.Stats {
		count += len(ids)
	}
	_ = json.NewEncoder(w).Encode(map[string]any{"status": "ok", "accepted": map[string]int{"hosts": len(in.Hosts), "stats": count, "host_stats": len(in.HostStats)}, "rejected": []any{}})
	go func() {
		for _, host := range in.Hosts {
			state, reason := "live", ""
			if host == "h_edge" {
				state, reason = "unreachable", "timeout"
			}
			f.live.mu.Lock()
			if forced := f.live.hostState[host]; forced != "" {
				state = forced
			}
			f.live.mu.Unlock()
			f.pushStream("host_status", map[string]any{"host": host, "state": state, "reason": reason, "t": time.Now().UnixMilli()})
		}
		for host, ids := range in.Stats {
			for _, id := range ids {
				f.pushStream("stats_history", map[string]any{"host": host, "id": id, "points": dockerManagerStatsPoints(12.4)})
			}
		}
		for _, host := range in.HostStats {
			f.pushStream("host_stats", dockerManagerHostStats(host, 42.5, false))
		}
	}()
}

func dockerManagerHostStats(host string, cpu float64, partial bool) map[string]any {
	return map[string]any{"host": host, "t": time.Now().UnixMilli(), "cpu": cpu, "ncpu": 8, "mem_used": 1073741824, "mem_total": 4294967296, "running": 3, "sampled": 3, "partial": partial}
}

func (f *dockerManagerFixture) subscriptionCount() int {
	f.live.mu.Lock()
	defer f.live.mu.Unlock()
	return len(f.live.subscriptions)
}

func (f *dockerManagerFixture) lastSubscription() string {
	f.live.mu.Lock()
	defer f.live.mu.Unlock()
	if len(f.live.subscriptions) == 0 {
		return ""
	}
	return f.live.subscriptions[len(f.live.subscriptions)-1]
}

// serveFollow is the SSE log stream of a container, a stack or an external project: the history, a notice for a stack, then
// whatever the test pushes. A request with `since` is a reconnect: the last line again (the app must not show it twice) and a new one.
func (f *dockerManagerFixture) serveFollow(w http.ResponseWriter, r *http.Request, path string) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	flusher, _ := w.(http.Flusher)
	ch := make(chan string, 512)
	f.live.mu.Lock()
	if f.live.followers == nil {
		f.live.followers = map[chan string]string{}
	}
	f.live.followers[ch] = path
	f.live.followURIs = append(f.live.followURIs, r.URL.RequestURI())
	f.live.mu.Unlock()
	defer func() {
		f.live.mu.Lock()
		delete(f.live.followers, ch)
		f.live.mu.Unlock()
	}()
	send := func(event string, data any) {
		payload, _ := json.Marshal(data)
		_, _ = fmt.Fprintf(w, "event: %s\ndata: %s\n\n", event, payload)
		flusher.Flush()
	}
	switch {
	case r.URL.Query().Get("since") != "":
		f.live.mu.Lock()
		last := f.live.lastLog
		f.live.mu.Unlock()
		if last == nil {
			last = dockerManagerLine(2, "err", "warning: low entropy", nil)
		}
		next, _ := time.Parse(time.RFC3339Nano, fmt.Sprint(last["t"]))
		send("lines", map[string]any{"lines": []any{last, map[string]any{"t": next.Add(time.Second).Format(time.RFC3339Nano), "s": "out", "text": "after reconnect", "trunc": false}}})
	case strings.Contains(path, "/stacks/"):
		send("lines", map[string]any{"lines": []any{
			dockerManagerLine(0, "out", "web: starting", map[string]any{"svc": "web", "ctr": "localshop-we"}), dockerManagerLine(1, "out", "db: ready for connections", map[string]any{"svc": "db", "ctr": "localshop-db"}),
			dockerManagerLine(2, "err", "web: listening on :80", map[string]any{"svc": "web", "ctr": "localshop-we"}),
		}})
		send("notice", map[string]any{"code": "not_followed", "message": "no slot", "services": []string{"cache"}})
	case strings.Contains(path, "/projects/"):
		send("lines", map[string]any{"lines": []any{dockerManagerLine(0, "out", "app: up", map[string]any{"svc": "app", "ctr": "locallegacy-ap"})}})
	default:
		send("lines", map[string]any{"lines": []any{dockerManagerLine(0, "out", "starting", nil), dockerManagerLine(1, "out", "listening on :80", nil), dockerManagerLine(2, "err", "warning: low entropy", nil)}})
	}
	for {
		select {
		case chunk := <-ch:
			if chunk == "DROP" {
				return
			}
			_, _ = io.WriteString(w, chunk)
			flusher.Flush()
		case <-r.Context().Done():
			return
		case <-time.After(15 * time.Second):
			_, _ = io.WriteString(w, ": heartbeat\n\n")
			flusher.Flush()
		}
	}
}

// pushLog sends lines to every open follow stream.
func (f *dockerManagerFixture) pushLog(lines ...map[string]any) {
	payload, _ := json.Marshal(map[string]any{"lines": lines})
	f.live.mu.Lock()
	defer f.live.mu.Unlock()
	if len(lines) > 0 {
		f.live.lastLog = lines[len(lines)-1]
	}
	for ch := range f.live.followers {
		select {
		case ch <- fmt.Sprintf("event: lines\ndata: %s\n\n", payload):
		default:
		}
	}
}

func (f *dockerManagerFixture) dropFollowers() {
	f.live.mu.Lock()
	defer f.live.mu.Unlock()
	for ch := range f.live.followers {
		select {
		case ch <- "DROP":
		default:
		}
	}
}

func (f *dockerManagerFixture) followRequests() string {
	f.live.mu.Lock()
	defer f.live.mu.Unlock()
	return strings.Join(f.live.followURIs, "\n")
}

// serveTerminal checks in the order of the contract (read-only, helper, stopped, protected, WebSocket) and answers with the same
// JSON errors; a valid upgrade becomes a shell that echoes every byte and answers Enter with a line.
func (f *dockerManagerFixture) serveTerminal(w http.ResponseWriter, r *http.Request, path string) {
	ref := strings.TrimSuffix(strings.TrimPrefix(path, "/hosts/local/containers/"), "/terminal")
	f.live.mu.Lock()
	f.live.terminalURIs = append(f.live.terminalURIs, r.URL.RequestURI())
	f.live.mu.Unlock()
	f.mu.Lock()
	readonly, protectedDB := f.readonly, f.protectedDB
	f.mu.Unlock()
	fail := func(status int, code, owner string) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_ = json.NewEncoder(w).Encode(map[string]any{"status": "error", "code": code, "owner": owner, "message": code})
	}
	switch {
	case readonly:
		fail(http.StatusForbidden, "read_only", "")
		return
	case strings.HasPrefix(ref, "localaurago"):
		fail(http.StatusConflict, "helper_container", "")
		return
	case strings.HasPrefix(ref, "localtools"):
		fail(http.StatusConflict, "container_not_running", "")
		return
	case protectedDB && strings.HasPrefix(ref, "localshop-db") && r.URL.Query().Get("confirm") != "protected":
		fail(http.StatusConflict, "container_protected_confirmation_required", "ruview")
		return
	case !websocket.IsWebSocketUpgrade(r):
		fail(http.StatusBadRequest, "websocket_required", "")
		return
	}
	conn, err := dockerManagerUpgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	defer conn.Close()
	_ = conn.WriteMessage(websocket.BinaryMessage, []byte("fake shell ready\r\n$ "))
	for {
		kind, payload, err := conn.ReadMessage()
		if err != nil {
			return
		}
		if kind == websocket.BinaryMessage {
			f.live.mu.Lock()
			f.live.terminalInput = append(f.live.terminalInput, string(payload))
			f.live.mu.Unlock()
			reply := string(payload)
			if strings.Contains(reply, "\r") {
				reply += "\nran it\r\n$ "
			}
			_ = conn.WriteMessage(websocket.BinaryMessage, []byte(reply))
			continue
		}
		var message struct {
			Type string `json:"type"`
		}
		_ = json.Unmarshal(payload, &message)
		f.live.mu.Lock()
		switch message.Type {
		case "resize":
			f.live.terminalResizes = append(f.live.terminalResizes, string(payload))
		case "end":
			f.live.terminalEnded++
		}
		f.live.mu.Unlock()
	}
}

func (f *dockerManagerFixture) terminalState() (uris []string, input string, resizes []string, ended int) {
	f.live.mu.Lock()
	defer f.live.mu.Unlock()
	return append([]string(nil), f.live.terminalURIs...), strings.Join(f.live.terminalInput, ""), append([]string(nil), f.live.terminalResizes...), f.live.terminalEnded
}

// dockerManagerWaitFor polls a condition from the Go side (state of the fixture) for up to 8 s.
func dockerManagerWaitFor(t *testing.T, what string, condition func() bool) {
	t.Helper()
	for i := 0; i < 320; i++ {
		if condition() {
			return
		}
		time.Sleep(25 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for %s", what)
}

// dockerManagerLiveApp opens the app with the native WebSocket back (the audit shell replaces it with a stub; a fresh iframe has the
// original) and with the extra fixture containers of the live tests.
func dockerManagerLiveApp(t *testing.T, fixture *dockerManagerFixture) *rod.Page {
	t.Helper()
	fixture.protectedDB, fixture.legacyContainers = true, true
	page := desktopAuditBrowserWithAPI(t, map[string]http.Handler{"/api/docker-manager/": fixture})
	page.MustEval(`()=>{const f=document.createElement('iframe');f.hidden=true;document.body.append(f);window.WebSocket=f.contentWindow.WebSocket}`)
	page.MustEval(`()=>auditTest.openApp('docker-manager')`)
	return page
}

// The stream: subscriptions follow what is on screen, the list reloads on events without the 5 s poll, stats are patched into the
// cells without a repaint of the table, host gauges appear under a single host, a paused host says so, and a dropped stream is reopened.
func TestDesktopDockerManagerLiveBrowser(t *testing.T) {
	fixture := newDockerManagerFixture()
	page := dockerManagerLiveApp(t, fixture)

	// hello names the stream; the hosts in scope are subscribed (events, status), no gauges while all hosts are shown
	dockerManagerWaitFor(t, "the first subscription", func() bool {
		return strings.Contains(fixture.lastSubscription(), `"stream_id":"`+strings.Repeat("a", 32)+`"`) && strings.Contains(fixture.lastSubscription(), `"hosts":["local","h_nas","h_edge"]`)
	})
	if page.MustEval(`()=>!!document.querySelector('[data-gauges]')`).Bool() {
		t.Fatal("the host gauges must stay hidden while all hosts are shown")
	}
	if page.MustEval(`()=>!!document.querySelector('[data-live-banner]')`).Bool() {
		t.Fatal("no banner while every host is live or has its own unreachable banner")
	}

	// Containers: the running rows are subscribed (12-character ids), the history fills the load cells with two sparklines each
	page.MustEval(`()=>document.querySelector('[data-action="pick-section"][data-section="containers"]').click()`)
	page.MustWait(`()=>document.querySelectorAll('.dm-row').length===7`)
	dockerManagerWaitFor(t, "stats subscription of the visible containers", func() bool {
		sub := fixture.lastSubscription()
		return strings.Contains(sub, `"stats":{"local":[`) && strings.Contains(sub, `"localshop-we"`) && !strings.Contains(sub, `"localtools00"`)
	})
	const cell = `document.querySelector('.dm-cell-load[data-load="local/localshop-we"]')`
	page.MustWait(`()=>{const c=` + cell + `;return !!c && c.querySelectorAll('svg.dm-spark').length===2 && c.textContent.includes('12%') && /CPU|12%/.test(c.title)}`)
	if !strings.Contains(page.MustEval(`()=>`+cell+`.title`).String(), "CPU 12%") {
		t.Fatalf("title of the load cell = %q, want CPU, memory and network rate", page.MustEval(`()=>`+cell+`.title`).String())
	}
	// a new point is patched into the same cell; the row element and the table body are the very same nodes (no repaint)
	page.MustEval(`()=>{const r=document.querySelector('.dm-row[data-row-key*="shop-web"]');r.dataset.stamp='kept';window.__tbody=document.querySelector('[data-tbody]')}`)
	fixture.pushStream("stats", map[string]any{"host": "local", "id": "localshop-we", "point": map[string]any{"t": time.Now().UnixMilli() + 1000, "cpu": 80.2, "mem": 120 * 1048576, "mem_limit": 2147483648, "net_rx": 9000, "net_tx": 4000, "blk_r": 0, "blk_w": 0, "pids": 5}})
	page.MustWait(`()=>` + cell + `.textContent.includes('80%')`)
	if !page.MustEval(`()=>document.querySelector('.dm-row[data-row-key*="shop-web"]').dataset.stamp==='kept' && document.querySelector('[data-tbody]')===window.__tbody`).Bool() {
		t.Fatal("a stats message must not repaint the table")
	}

	// the safety poll is a minute now: only an event brings a stopped container into the list within seconds
	fixture.mu.Lock()
	fixture.webExited = true
	fixture.mu.Unlock()
	started := time.Now()
	fixture.pushStream("event", map[string]any{"host": "local", "type": "container", "action": "die", "id": strings.Repeat("b", 64), "short_id": "localshop-we", "name": "shop-web", "project": "shop", "service": "web", "exit_code": 137, "t": time.Now().UnixMilli()})
	page.MustWait(`()=>document.querySelector('.dm-row[data-row-key*="shop-web"]').dataset.tone==='bad'`)
	if took := time.Since(started); took > 3*time.Second {
		t.Fatalf("the list took %s to follow an event, want well under the 5 s poll", took)
	}
	// ...and its load cell is gone with its subscription
	page.MustWait(`()=>!document.querySelector('.dm-cell-load[data-load="local/localshop-we"]')`)

	// a paused host gets a polite banner, and loses it when it is live again
	fixture.setHostStatus("local", "paused")
	page.MustWait(`()=>{const b=document.querySelector('[data-live-banner="local"]');return !!b && b.getAttribute('role')==='status' && /Local/.test(b.textContent)}`)
	fixture.setHostStatus("local", "live")
	page.MustWait(`()=>!document.querySelector('[data-live-banner]')`)

	// a single host: gauges appear, the host is subscribed for host stats, the disk usage comes from /df and can be read again
	page.MustEval(`()=>document.querySelector('.dm-host-btn[data-host-id="local"]').click()`)
	dockerManagerWaitFor(t, "the host stats subscription", func() bool { return strings.Contains(fixture.lastSubscription(), `"host_stats":["local"]`) })
	page.MustWait(`()=>{const g=document.querySelector('[data-gauges]');return !!g && g.querySelectorAll('.dm-gauge').length===3 && g.textContent.includes('43%') && g.textContent.includes('1.0 GB / 4.0 GB') && g.textContent.includes('1.5 GB')}`)
	if got := page.MustEval(`()=>[...document.querySelectorAll('[data-gauges] .dm-gauge')].map(g=>g.getAttribute('role')).join(',')`).String(); got != "meter,meter,group" {
		t.Fatalf("gauge roles = %q, want two meters and the plain disk value", got)
	}
	fixture.pushStream("host_stats", dockerManagerHostStats("local", 91.2, true))
	page.MustWait(`()=>{const g=document.querySelector('[data-gauges]');return g.textContent.includes('91%') && g.querySelector('.dm-gauge[data-level="bad"]') && /3/.test(g.querySelector('.dm-gauge-note')?.textContent||'')}`)
	page.MustEval(`()=>document.querySelector('[data-action="refresh-df"]').click()`)
	dockerManagerWaitFor(t, "the disk usage to be read again", func() bool {
		fixture.live.mu.Lock()
		defer fixture.live.mu.Unlock()
		return strings.Contains(strings.Join(fixture.live.dfURIs, "\n"), "/hosts/local/df?refresh=1")
	})
	page.MustEval(`()=>document.querySelector('.dm-host-btn[data-host-id="all"]').click()`)
	page.MustWait(`()=>!document.querySelector('[data-gauges]')`)

	// a dropped stream is opened again after a second and subscribed again with the new stream id
	before := fixture.subscriptionCount()
	fixture.dropStream()
	dockerManagerWaitFor(t, "the reconnect and the new subscription", func() bool {
		return fixture.subscriptionCount() > before && strings.Contains(fixture.lastSubscription(), `"stream_id":"`+strings.Repeat("b", 32)+`"`)
	})

	if errs := page.MustEval(`()=>window.fixtureErrors.join('|')`).String(); errs != "" {
		t.Fatalf("page errors: %s", errs)
	}
}

// The follow panels: a container's logs, the same lines of a stack with service chips and a notice, the logs of an external project, and
// the terminal with its echo, resize, key isolation, protected confirmation and the messages for the cases that open nothing.
func TestDesktopDockerManagerFollowBrowser(t *testing.T) {
	fixture := newDockerManagerFixture()
	page := dockerManagerLiveApp(t, fixture)
	page.MustWait(`()=>document.querySelectorAll('.dm-srow-main[data-stack-key]').length===2`)

	// ---- container logs
	page.MustEval(`()=>document.querySelector('[data-action="pick-section"][data-section="containers"]').click()`)
	page.MustWait(`()=>document.querySelectorAll('.dm-row').length===7`)
	page.MustEval(`()=>document.querySelector('.dm-row[data-row-key*="shop-web"] [data-action="logs"]').click()`)
	page.MustWait(`()=>{const v=document.querySelector('[data-lp-view]');return !!v && v.textContent.includes('listening on :80') && !!v.querySelector('.dm-ll[data-s="err"]')}`)
	if !strings.Contains(fixture.followRequests(), "/hosts/local/containers/localshop-web") || !strings.Contains(fixture.followRequests(), "follow=1&tail=200") {
		t.Fatalf("follow requests = %q, want the container with tail 200", fixture.followRequests())
	}
	// 150 more lines: the view follows them; scrolling up pauses, the counter counts, the button resumes
	bulk := func(from, to int) {
		lines := make([]map[string]any, 0, to-from)
		for i := from; i < to; i++ {
			lines = append(lines, dockerManagerLine(10+i, "out", fmt.Sprintf("bulk %03d", i), nil))
		}
		fixture.pushLog(lines...)
	}
	bulk(0, 150)
	page.MustWait(`()=>{const v=document.querySelector('[data-lp-view]');return v.textContent.includes('bulk 149') && v.scrollHeight-v.scrollTop-v.clientHeight<30}`)
	page.MustEval(`()=>{document.querySelector('[data-lp-view]').scrollTop=0}`)
	page.MustWait(`()=>document.querySelector('[data-lp-follow]').getAttribute('aria-pressed')==='false'`)
	bulk(150, 155)
	page.MustWait(`()=>{const b=document.querySelector('[data-lp-new]');return !b.hidden && b.textContent.includes('5')}`)
	if page.MustEval(`()=>document.querySelector('[data-lp-view]').scrollTop`).Int() > 5 {
		t.Fatal("a paused view must not jump to the new lines")
	}
	page.MustEval(`()=>document.querySelector('[data-lp-new]').click()`)
	page.MustWait(`()=>{const v=document.querySelector('[data-lp-view]');return document.querySelector('[data-lp-new]').hidden && document.querySelector('[data-lp-follow]').getAttribute('aria-pressed')==='true' && v.scrollHeight-v.scrollTop-v.clientHeight<30}`)
	// filter, timestamps
	page.MustElement(`[data-lp-search]`).MustInput("bulk 12")
	page.MustWait(`()=>{const rows=[...document.querySelectorAll('.dm-ll-text')].map(e=>e.textContent);return rows.length===10 && rows.every(r=>r.startsWith('bulk 12'))}`)
	if err := page.Keyboard.Type(input.Escape); err != nil {
		t.Fatal(err)
	}
	page.MustWait(`()=>document.querySelector('[data-lp-search]').value==='' && document.querySelectorAll('.dm-ll').length>150`)
	if page.MustEval(`()=>!document.querySelector('[data-drawer]').hidden`).Bool() == false {
		t.Fatal("Escape in the filter must clear the filter first and leave the drawer open")
	}
	page.MustEval(`()=>document.querySelector('[data-lp-ts]').click()`)
	page.MustWait(`()=>getComputedStyle(document.querySelector('.dm-ll-ts')).display!=='none'`)
	// a broken connection is opened again with `since`; the repeated last line is shown once
	fixture.dropFollowers()
	page.MustWait(`()=>document.querySelector('.dm-ll-text') && [...document.querySelectorAll('.dm-ll-text')].some(e=>e.textContent==='after reconnect')`)
	if !strings.Contains(fixture.followRequests(), "tail=5000&since=") {
		t.Fatalf("follow requests = %q, want a reconnect with since", fixture.followRequests())
	}
	if got := page.MustEval(`()=>[...document.querySelectorAll('.dm-ll-text')].filter(e=>e.textContent==='bulk 154').length`).Int(); got != 1 {
		t.Fatalf("the line the server repeated is shown %d times, want once", got)
	}

	// ---- terminal of the same container: connects, echoes, fits, keeps its keys
	page.MustWait(`()=>!!document.querySelector('[data-drawer] [data-tab="terminal"]')`)
	page.MustEval(`()=>document.querySelector('[data-tab="terminal"]').click()`)
	page.MustWait(`()=>document.querySelector('[data-tp-status]')?.dataset.phase==='connected' && document.querySelector('.dm-tp-screen .xterm-rows')?.textContent.includes('fake shell ready')`)
	page.MustEval(`()=>document.querySelector('.dm-tp-screen .xterm-helper-textarea').focus()`)
	if err := page.Keyboard.Type(input.KeyL, input.KeyS, input.Enter); err != nil {
		t.Fatal(err)
	}
	page.MustWait(`()=>document.querySelector('.dm-tp-screen .xterm-rows').textContent.includes('ran it')`)
	_, typed, resizes, _ := fixture.terminalState()
	if !strings.Contains(typed, "l") || !strings.Contains(typed, "\r") || len(resizes) == 0 || !strings.Contains(resizes[0], `"type":"resize"`) {
		t.Fatalf("terminal saw input %q and resizes %v, want the typed keys and a resize frame", typed, resizes)
	}
	// Escape and Ctrl+K belong to the shell: the drawer stays open, the palette stays closed, the shell gets the bytes
	if err := page.Keyboard.Type(input.Escape); err != nil {
		t.Fatal(err)
	}
	dockerManagerPress(page, 'k')
	dockerManagerWaitFor(t, "Escape and Ctrl+K to reach the shell", func() bool {
		_, in, _, _ := fixture.terminalState()
		return strings.Contains(in, "\x1b") && strings.Contains(in, "\x0b")
	})
	if !page.MustEval(`()=>!document.querySelector('[data-drawer]').hidden && !document.querySelector('[data-palette]') && !document.getElementById('vd-spotlight-backdrop')`).Bool() {
		t.Fatal("Escape and Ctrl+K inside the terminal must not close the drawer or open the palette or Spotlight")
	}
	// Ctrl+] leaves the terminal for the toolbar
	page.KeyActions().Press(input.ControlLeft).Type(input.BracketRight).MustDo()
	page.MustWait(`()=>document.activeElement?.matches('[data-tp-leave]')`)

	// ---- protected container: the question comes first, the request carries ?confirm=protected
	page.MustEval(`()=>document.querySelector('[data-action="close-detail"]').click()`)
	page.MustWait(`()=>document.querySelector('[data-drawer]').hidden`)
	dockerManagerWaitFor(t, "the session of the closed drawer to end", func() bool { _, _, _, ended := fixture.terminalState(); return ended == 1 })
	page.MustEval(`()=>{const row=document.querySelector('.dm-row[data-row-key*="shop-db"]');row.click()}`)
	page.MustWait(`()=>!!document.querySelector('[data-drawer] [data-tab="terminal"]')`)
	page.MustEval(`()=>document.querySelector('[data-tab="terminal"]').click()`)
	page.MustWait(`()=>!!document.querySelector('dialog[open] [data-dact="yes"]')`)
	page.MustEval(`()=>document.querySelector('dialog[open] [data-dact="yes"]').click()`)
	page.MustWait(`()=>document.querySelector('[data-tp-status]')?.dataset.phase==='connected'`)
	uris, _, _, _ := fixture.terminalState()
	if len(uris) < 2 || !strings.HasSuffix(uris[len(uris)-1], "/terminal?confirm=protected") {
		t.Fatalf("terminal requests = %v, want the protected one with ?confirm=protected", uris)
	}

	// ---- nothing to open: a stopped container and a helper container say why, without a request for a shell
	for _, tc := range []struct{ row, want string }{{"tools", "läuft nicht"}, {"aurago-dm-op-1", "Hilfscontainer"}} {
		page.MustEval(`()=>document.querySelector('[data-action="close-detail"]').click()`)
		page.MustEval(`(name)=>document.querySelector('.dm-row[data-row-key*="'+name+'"]').click()`, tc.row)
		page.MustWait(`()=>!!document.querySelector('[data-drawer] [data-tab="terminal"]')`)
		page.MustEval(`()=>document.querySelector('[data-tab="terminal"]').click()`)
		page.MustWait(`(want)=>document.querySelector('[data-live-panel="terminal"] [data-tp-msg]')?.textContent.includes(want)`, tc.want)
	}
	if got, _, _, _ := fixture.terminalState(); len(got) != len(uris) {
		t.Fatalf("a stopped or helper container must not even ask for a terminal: %v", got)
	}
	page.MustEval(`()=>document.querySelector('[data-action="close-detail"]').click()`)

	// ---- stack: logs of all services with chips and a notice, terminal over the running services
	page.MustEval(`()=>document.querySelector('[data-action="pick-section"][data-section="stacks"]').click()`)
	page.MustEval(`()=>document.querySelector('.dm-srow-main[data-stack-key="local/shop"]').click()`)
	page.MustWait(`()=>!document.querySelector('[data-stack-pane]').hidden && document.querySelectorAll('.dm-svc').length===2`)
	page.MustWait(`()=>document.querySelectorAll('[data-stack-pane] .dm-svc-load svg.dm-spark').length===2`)
	page.MustEval(`()=>document.querySelector('[data-action="stack-tab"][data-tab="logs"]').click()`)
	page.MustWait(`()=>{const p=document.querySelector('[data-panel="logs"]');return !p.hidden && p.querySelectorAll('[data-lp-chip]').length===2 && p.querySelectorAll('.dm-ll .dm-ll-svc').length===3 && !!p.querySelector('.dm-lp-notice[data-code="not_followed"]')}`)
	page.MustEval(`()=>document.querySelector('[data-panel="logs"] [data-lp-chip="db"]').click()`)
	page.MustWait(`()=>![...document.querySelectorAll('[data-panel="logs"] .dm-ll-text')].some(e=>e.textContent.startsWith('db:')) && document.querySelector('[data-panel="logs"] [data-lp-chip="db"]').getAttribute('aria-pressed')==='false'`)
	if !strings.Contains(fixture.followRequests(), "/hosts/local/stacks/shop/logs?follow=1&tail=100") {
		t.Fatalf("follow requests = %q, want the stack with 100 lines per container", fixture.followRequests())
	}
	page.MustEval(`()=>document.querySelector('[data-action="stack-tab"][data-tab="terminal"]').click()`)
	page.MustWait(`()=>document.querySelector('[data-panel="terminal"] [data-tp-status]')?.dataset.phase==='connected'`)
	if page.MustEval(`()=>document.querySelector('[data-panel="terminal"] [data-tp-pick-wrap]').hidden`).Bool() == false {
		t.Fatal("one running service needs no picker")
	}
	page.MustEval(`()=>document.querySelector('[data-action="close-stack"]').click()`)

	// ---- external project: its containers, a load placeholder, its logs
	page.MustEval(`()=>document.querySelector('[data-action="open-project"][data-project="legacy"]').click()`)
	page.MustWait(`()=>!document.querySelector('[data-stack-pane]').hidden && document.querySelectorAll('[data-project-overview] .dm-svc').length===1`)
	page.MustWait(`()=>document.querySelectorAll('[data-project-overview] .dm-svc-load svg.dm-spark').length===2`)
	if got := page.MustEval(`()=>[...document.querySelectorAll('[data-stack-pane] [role="tab"]')].map(t=>t.dataset.tab).join(',')`).String(); got != "overview,logs" {
		t.Fatalf("project tabs = %q, want overview,logs", got)
	}
	page.MustEval(`()=>document.querySelector('[data-action="project-tab"][data-tab="logs"]').click()`)
	page.MustWait(`()=>document.querySelector('[data-panel="logs"]').textContent.includes('app: up')`)
	if !strings.Contains(fixture.followRequests(), "/hosts/local/projects/legacy/logs?follow=1&tail=100") {
		t.Fatalf("follow requests = %q, want the external project", fixture.followRequests())
	}
	page.MustEval(`()=>document.querySelector('[data-action="close-project"]').click()`)

	// ---- palette: Logs and Terminal commands; read-only removes the terminal and explains it
	page.MustEval(`()=>document.querySelector('[data-search]').focus()`)
	dockerManagerPress(page, 'k')
	page.MustWait(`()=>!!document.querySelector('[data-palette] input')`)
	page.MustElement(`[data-palette] input`).MustInput("shop-web")
	page.MustWait(`()=>{const t=[...document.querySelectorAll('[data-palette] .dm-palette-item')].map(i=>i.textContent);return t.some(x=>x.includes('Terminal: shop-web')) && t.some(x=>x.includes('Logs: shop-web'))}`)
	if err := page.Keyboard.Type(input.Escape); err != nil {
		t.Fatal(err)
	}
	fixture.setReadonly(true)
	page.MustEval(`()=>document.querySelector('[data-refresh]').click()`)
	page.MustWait(`()=>document.querySelector('[data-readonly-chip]') && !document.querySelector('[data-readonly-chip]').hidden`)
	page.MustEval(`()=>document.querySelector('[data-search]').focus()`)
	dockerManagerPress(page, 'k')
	page.MustWait(`()=>!!document.querySelector('[data-palette] input')`)
	page.MustElement(`[data-palette] input`).MustInput("shop-web")
	page.MustWait(`()=>{const t=[...document.querySelectorAll('[data-palette] .dm-palette-item')].map(i=>i.textContent);return t.some(x=>x.includes('Logs: shop-web')) && !t.some(x=>x.includes('Terminal:'))}`)
	if err := page.Keyboard.Type(input.Escape); err != nil {
		t.Fatal(err)
	}
	page.MustEval(`()=>document.querySelector('[data-action="pick-section"][data-section="containers"]').click()`)
	page.MustWait(`()=>document.querySelectorAll('.dm-row').length===7`)
	page.MustEval(`()=>document.querySelector('.dm-row[data-row-key*="shop-web"]').click()`)
	page.MustWait(`()=>!!document.querySelector('[data-drawer] [data-tab="terminal"]')`)
	page.MustEval(`()=>document.querySelector('[data-tab="terminal"]').click()`)
	page.MustWait(`()=>document.querySelector('[data-live-panel="terminal"] [data-tp-msg]')?.textContent.includes('Nur-Lese-Modus') && !document.querySelector('[data-live-panel="terminal"] .xterm')`)
	if errs := page.MustEval(`()=>window.fixtureErrors.join('|')`).String(); errs != "" {
		t.Fatalf("page errors: %s", errs)
	}
}
