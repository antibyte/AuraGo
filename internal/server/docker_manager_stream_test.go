package server

import (
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"testing"
	"time"

	"aurago/internal/dockerstacks"
	"aurago/internal/dockerwatch"
)

func TestDockerManagerStreamSendsHelloThenTheHubsMessages(t *testing.T) {
	f := newDMLiveFixture(t)
	stream := f.open(http.MethodGet, "/stream", nil)
	if got := stream.resp.Header.Get("Content-Type"); got != "text/event-stream" {
		t.Fatalf("Content-Type = %q, want text/event-stream", got)
	}

	hello := stream.nextEvent()
	data := hello.json(t)
	limits := dmObject(t, data, "limits")
	if hello.event != "hello" || !regexp.MustCompile(`^[0-9a-f]{32}$`).MatchString(data["stream_id"].(string)) ||
		data["stream_id"] != f.live.client.id || data["heartbeat_ms"] != float64(15000) ||
		limits["stats_ids"] != float64(50) || limits["hosts"] != float64(20) {
		t.Fatalf("hello = %s %s, want the stream id, 15000 ms and the limits 50 and 20", hello.event, hello.data)
	}

	code := 137
	f.live.client.messages <- dockerwatch.Message{Event: dockerwatch.MsgHostStatus, Data: dockerwatch.HostStatus{Host: "local", State: dockerwatch.StateLive, T: 1730000000000}}
	f.live.client.messages <- dockerwatch.Message{Event: dockerwatch.MsgEvent, Data: dockerwatch.Event{
		Host: "local", Type: "container", Action: "die", ID: strings.Repeat("a", 64), ShortID: strings.Repeat("a", 12),
		Name: "web-1", Project: "shop", Service: "web", ExitCode: &code, T: 1730000000001,
	}}
	f.live.client.messages <- dockerwatch.Message{Event: dockerwatch.MsgStats, Data: dockerwatch.StatsMsg{
		Host: "local", ID: strings.Repeat("a", 12), Point: dockerwatch.StatsPoint{T: 1730000000002, CPU: 3.5, Mem: 100, MemLimit: 200, Pids: 4},
	}}
	f.live.client.messages <- dockerwatch.Message{Event: dockerwatch.MsgOperation, Data: dmOperationMessage{
		Host: "local", Operation: dockerstacks.Operation{ID: "op_0123456789abcdef", HostID: "local", Stack: "shop", Kind: "up", State: "running"},
	}}

	status := stream.nextEvent()
	if got := status.json(t); status.event != "host_status" || got["host"] != "local" || got["state"] != "live" || got["reason"] != "" || got["t"] != float64(1730000000000) {
		t.Fatalf("host_status = %s %s", status.event, status.data)
	}
	event := stream.nextEvent()
	if got := event.json(t); event.event != "event" || got["action"] != "die" || got["exit_code"] != float64(137) || got["short_id"] != strings.Repeat("a", 12) ||
		got["project"] != "shop" || got["service"] != "web" || got["name"] != "web-1" {
		t.Fatalf("event = %s %s", event.event, event.data)
	}
	stats := stream.nextEvent()
	point := dmObject(t, stats.json(t), "point")
	if stats.event != "stats" || point["cpu"] != 3.5 || point["mem_limit"] != float64(200) || point["pids"] != float64(4) {
		t.Fatalf("stats = %s %s", stats.event, stats.data)
	}
	op := stream.nextEvent()
	operation := dmObject(t, op.json(t), "operation")
	if op.event != "operation" || op.json(t)["host"] != "local" || operation["id"] != "op_0123456789abcdef" || operation["output"] != "" || operation["state"] != "running" {
		t.Fatalf("operation = %s %s", op.event, op.data)
	}

	// A client the hub drops is sent `dropped`, then its channel closes and the stream ends.
	f.live.client.messages <- dockerwatch.Message{Event: dockerwatch.MsgDropped, Data: dockerwatch.Dropped{Reason: "slow_client"}}
	close(f.live.client.messages)
	if dropped := stream.nextEvent(); dropped.event != "dropped" || dropped.json(t)["reason"] != "slow_client" {
		t.Fatalf("dropped = %s %s", dropped.event, dropped.data)
	}
	stream.expectEnd()
	select {
	case <-f.live.client.closed:
	case <-time.After(5 * time.Second):
		t.Fatal("the hub client was not closed when the stream ended")
	}
}

func TestDockerManagerStreamHeartbeatsAndClosesTheClientWhenTheRequestEnds(t *testing.T) {
	withHeartbeat(t, 20*time.Millisecond)
	f := newDMLiveFixture(t)
	stream := f.open(http.MethodGet, "/stream", nil)
	if hello := stream.nextEvent(); hello.event != "hello" || hello.json(t)["heartbeat_ms"] != float64(20) {
		t.Fatalf("hello = %+v", hello)
	}
	if beat := stream.next(); beat.comment != "heartbeat" {
		t.Fatalf("frame = %+v, want a heartbeat comment line", beat)
	}
	stream.close()
	select {
	case <-f.live.client.closed:
	case <-time.After(5 * time.Second):
		t.Fatal("the hub client stayed open after the request ended")
	}
}

func TestDockerManagerStreamRefusesTooManyClients(t *testing.T) {
	f := newDMLiveFixture(t)
	f.live.attachErr = dockerwatch.ErrTooManyClients
	f.open(http.MethodGet, "/stream", nil).expectError(http.StatusTooManyRequests, "too_many_streams")
	f.open(http.MethodPost, "/stream", nil).expectError(http.StatusMethodNotAllowed, "method_not_allowed")
}

func TestDockerManagerSubscriptionsStoreTheListsAndRejectUnknownHosts(t *testing.T) {
	f := newDMFixture(t)
	nas := f.addRemote("NAS")
	client, err := f.s.DockerWatch.Attach()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(client.Close)

	id := strings.Repeat("a", 64)
	code, body := f.do(http.MethodPut, "/stream/subscriptions", map[string]any{
		"stream_id":  client.ID(),
		"hosts":      []string{"local", nas, "ghost"},
		"stats":      map[string][]string{"local": {id, strings.Repeat("b", 12)}, "ghost": {strings.Repeat("c", 12)}},
		"host_stats": []string{"ghost", "local"},
	})
	accepted := dmObject(t, body, "accepted")
	rejected := dmArray(t, body, "rejected")
	if code != http.StatusOK || body["status"] != "ok" || accepted["hosts"] != float64(2) || accepted["stats"] != float64(2) || accepted["host_stats"] != float64(1) {
		t.Fatalf("PUT = %d %s, want ok with 2 hosts, 2 stats ids and 1 gauge host", code, f.lastRaw)
	}
	if len(rejected) != 1 || rejected[0]["host"] != "ghost" || rejected[0]["reason"] != "host_not_found" {
		t.Fatalf("rejected = %v, want the unknown host once, with the reason host_not_found", rejected)
	}

	// Everything is rejected: an empty subscription is a valid one, and `rejected` is still a list.
	code, body = f.do(http.MethodPut, "/stream/subscriptions", map[string]any{"stream_id": client.ID(), "hosts": []string{"ghost"}})
	if code != http.StatusOK || len(dmArray(t, body, "rejected")) != 1 || dmObject(t, body, "accepted")["hosts"] != float64(0) {
		t.Fatalf("PUT of an unknown host = %d %s", code, f.lastRaw)
	}
	code, body = f.do(http.MethodPut, "/stream/subscriptions", map[string]any{"stream_id": client.ID()})
	if code != http.StatusOK || body["rejected"] == nil || len(dmArray(t, body, "rejected")) != 0 {
		t.Fatalf("PUT of nothing = %d %s, want an empty rejected list", code, f.lastRaw)
	}
}

func TestDockerManagerSubscriptionsAnswerTheirErrors(t *testing.T) {
	f := newDMFixture(t)
	client, err := f.s.DockerWatch.Attach()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(client.Close)
	put := func(body any) (int, map[string]any) {
		return f.do(http.MethodPut, "/stream/subscriptions", body)
	}

	code, body := put(map[string]any{"stream_id": strings.Repeat("0", 32), "hosts": []string{"local"}})
	dmExpectError(t, f, code, body, http.StatusNotFound, "stream_not_found")

	ids := make([]string, 51)
	for i := range ids {
		ids[i] = fmt.Sprintf("%012x", i+1)
	}
	code, body = put(map[string]any{"stream_id": client.ID(), "stats": map[string][]string{"local": ids}})
	dmExpectError(t, f, code, body, http.StatusBadRequest, "bad_request")
	if !strings.Contains(body["message"].(string), "50") {
		t.Fatalf("message = %q, want the limit of 50 container ids", body["message"])
	}

	code, body = put(map[string]any{"stream_id": client.ID(), "stats": map[string][]string{"local": {"web1"}}})
	dmExpectError(t, f, code, body, http.StatusBadRequest, "bad_request")

	// More than three lists' worth of unknown hosts is refused by the hub without a registry lookup for each.
	many := make([]string, 70)
	for i := range many {
		many[i] = "ghost" + string(rune('a'+i/26)) + string(rune('a'+i%26))
	}
	code, body = put(map[string]any{"stream_id": client.ID(), "hosts": many})
	dmExpectError(t, f, code, body, http.StatusBadRequest, "bad_request")
	if !strings.Contains(body["message"].(string), "20 hosts") {
		t.Fatalf("message = %q, want the limit of 20 hosts", body["message"])
	}

	code, body = f.do(http.MethodPut, "/stream/subscriptions", nil)
	dmExpectError(t, f, code, body, http.StatusBadRequest, "bad_request")
	code, body = f.do(http.MethodGet, "/stream/subscriptions", nil)
	dmExpectError(t, f, code, body, http.StatusMethodNotAllowed, "method_not_allowed")
}

func TestDockerManagerSubscriptionOfAClosedStreamIsNotFound(t *testing.T) {
	f := newDMLiveFixture(t)
	f.live.client.setErr = dockerwatch.ErrClientClosed
	raw := strings.NewReader(`{"stream_id":"` + f.live.client.id + `","hosts":["local"]}`)
	f.open(http.MethodPut, "/stream/subscriptions", raw).expectError(http.StatusNotFound, "stream_not_found")

	f.live.client.setErr = errors.New("unexpected")
	raw = strings.NewReader(`{"stream_id":"` + f.live.client.id + `","hosts":["local"]}`)
	f.open(http.MethodPut, "/stream/subscriptions", raw).expectError(http.StatusBadGateway, "host_unreachable")
}

// TestDockerManagerStreamDeliversWhatAClientSubscribedTo runs the whole path: the
// real hub, the real registry, a fake Engine with a held event feed and the
// operation bridge.
func TestDockerManagerStreamDeliversWhatAClientSubscribedTo(t *testing.T) {
	ls := newDMLiveServer(t, "[]")
	stream := ls.open(http.MethodGet, "/stream", nil, nil)
	hello := stream.nextEvent()
	id, _ := hello.json(t)["stream_id"].(string)
	if hello.event != "hello" || id == "" {
		t.Fatalf("hello = %+v", hello)
	}

	put := ls.open(http.MethodPut, "/stream/subscriptions", map[string]any{"stream_id": id, "hosts": []string{"local"}}, nil)
	if body := put.body(); put.resp.StatusCode != http.StatusOK || dmObject(t, body, "accepted")["hosts"] != float64(1) {
		t.Fatalf("PUT = %d %v", put.resp.StatusCode, body)
	}
	for {
		frame := stream.nextEvent()
		if frame.event != "host_status" {
			t.Fatalf("frame = %s %s, want host_status first", frame.event, frame.data)
		}
		if state := frame.json(t)["state"]; state == "live" {
			break
		}
	}

	observer := &dmFakeObserver{}
	bridgeDockerOperations(observer, ls.s.DockerWatch)
	observer.fn(dockerstacks.Operation{ID: "op_0123456789abcdef", HostID: "local", Stack: "shop", Kind: "up", State: "queued", Output: "dropped from the message"})
	for {
		frame := stream.nextEvent()
		if frame.event == "operation" {
			if got := dmObject(t, frame.json(t), "operation"); got["state"] != "queued" || got["output"] != "" {
				t.Fatalf("operation = %s", frame.data)
			}
			break
		}
	}

	stream.close()
	waitUntilTrue(t, "the hub client of the closed stream to go", func() bool {
		_, open := ls.s.DockerWatch.Client(id)
		return !open
	})
}

// waitUntilTrue polls cond for up to five seconds.
func waitUntilTrue(t *testing.T, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for !cond() {
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for %s", what)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

// dmOpenLiveStream opens a stream on the real hub and returns it with its id.
func dmOpenLiveStream(t *testing.T, ls *dmLiveServer) (*dmStream, string) {
	t.Helper()
	stream := ls.open(http.MethodGet, "/stream", nil, nil)
	hello := stream.nextEvent()
	id, _ := hello.json(t)["stream_id"].(string)
	if hello.event != "hello" || id == "" {
		t.Fatalf("hello = %+v", hello)
	}
	return stream, id
}

// dmSubscribe replaces the subscription of a stream and requires a 200.
func dmSubscribe(t *testing.T, ls *dmLiveServer, id string, body map[string]any) map[string]any {
	t.Helper()
	body["stream_id"] = id
	put := ls.open(http.MethodPut, "/stream/subscriptions", body, nil)
	answer := put.body()
	if put.resp.StatusCode != http.StatusOK || answer["status"] != "ok" {
		t.Fatalf("PUT /stream/subscriptions = %d %v", put.resp.StatusCode, answer)
	}
	return answer
}

func TestDockerManagerSubscriptionRejectsAHostTheRegistryCannotLookUp(t *testing.T) {
	f := newDMFixture(t)
	client, err := f.s.DockerWatch.Attach()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(client.Close)
	_ = f.s.dockerHostStore.Close() // every lookup of a stored host fails now; the local host needs no store

	code, body := f.do(http.MethodPut, "/stream/subscriptions", map[string]any{"stream_id": client.ID(), "hosts": []string{"local", "nas"}})
	rejected := dmArray(t, body, "rejected")
	if code != http.StatusOK || dmObject(t, body, "accepted")["hosts"] != float64(1) || len(rejected) != 1 || rejected[0]["host"] != "nas" || rejected[0]["reason"] != "host_unavailable" {
		t.Fatalf("PUT = %d %s, want the local host accepted and nas rejected as host_unavailable, not a failed request", code, f.lastRaw)
	}
}

func TestDockerManagerSubscriptionSendsTheCurrentHostStatusOfEveryNewHost(t *testing.T) {
	ls := newDMLiveServer(t, "[]")
	first, firstID := dmOpenLiveStream(t, ls)
	dmSubscribe(t, ls, firstID, map[string]any{"hosts": []string{"local"}})
	for {
		if frame := first.nextEvent(); frame.event == "host_status" && frame.json(t)["state"] == "live" {
			break
		}
	}

	// A second window finds the watcher running: its status is sent although it did not change.
	second, secondID := dmOpenLiveStream(t, ls)
	dmSubscribe(t, ls, secondID, map[string]any{"hosts": []string{"local"}})
	if frame := second.nextEvent(); frame.event != "host_status" || frame.json(t)["host"] != "local" || frame.json(t)["state"] != "live" {
		t.Fatalf("first frame after the subscription = %s %s, want host_status live", frame.event, frame.data)
	}

	// A host that leaves and comes back is new to the stream again.
	dmSubscribe(t, ls, secondID, map[string]any{"hosts": []string{}})
	dmSubscribe(t, ls, secondID, map[string]any{"hosts": []string{"local"}})
	if frame := second.nextEvent(); frame.event != "host_status" || frame.json(t)["state"] != "live" {
		t.Fatalf("frame after the host came back = %s %s, want host_status live", frame.event, frame.data)
	}
}

func TestDockerManagerSubscriptionSendsStatsHistoryBeforeLivePoints(t *testing.T) {
	ls := newDMLiveServer(t, "[]")
	first, firstID := dmOpenLiveStream(t, ls)
	dmSubscribe(t, ls, firstID, map[string]any{"stats": map[string][]string{"local": {"aaaaaaaaaaaa"}}})
	for seen := 0; seen < 2; {
		if first.nextEvent().event == "stats" {
			seen++
		}
	}

	// The container has a ring now: a window that asks later gets it first, oldest point first.
	second, secondID := dmOpenLiveStream(t, ls)
	dmSubscribe(t, ls, secondID, map[string]any{"stats": map[string][]string{"local": {"aaaaaaaaaaaa"}}})
	history := second.nextEvent()
	points, _ := history.json(t)["points"].([]any)
	if history.event != "stats_history" || history.json(t)["host"] != "local" || history.json(t)["id"] != "aaaaaaaaaaaa" || len(points) < 2 {
		t.Fatalf("first frame = %s %s, want stats_history with the points so far", history.event, history.data)
	}
	last, _ := points[len(points)-1].(map[string]any)
	live := second.nextEvent()
	point := dmObject(t, live.json(t), "point")
	if live.event != "stats" || point["t"].(float64) <= last["t"].(float64) && point["mem"].(float64) <= last["mem"].(float64) {
		t.Fatalf("frame after the history = %s %s, want a live stats point newer than %v", live.event, live.data, last)
	}
}
