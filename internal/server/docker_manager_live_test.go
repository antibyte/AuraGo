package server

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"sync"
	"testing"
	"time"

	"aurago/internal/dockerhost"
	"aurago/internal/dockerstacks"
	"aurago/internal/dockerwatch"
)

func TestInitDockerHostsOpensTheLiveHubAndCloseEndsIt(t *testing.T) {
	f := newDMFixture(t)
	if f.s.DockerWatch == nil {
		t.Fatal("initDockerHosts left DockerWatch nil")
	}
	if f.s.dockerOpsBridge == nil {
		t.Fatal("the stacks service is open but its operations are not forwarded to the hub")
	}
	client, err := f.s.DockerWatch.Attach()
	if err != nil {
		t.Fatalf("Attach: %v", err)
	}

	f.s.closeDockerHosts()
	select {
	case _, open := <-client.Messages():
		if open {
			t.Fatal("a message arrived on a client of the closed hub")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("closeDockerHosts left a stream client open")
	}
	if _, err := f.s.DockerWatch.Attach(); !errors.Is(err, dockerwatch.ErrHubClosed) {
		t.Fatalf("Attach after close = %v, want ErrHubClosed", err)
	}
	if f.s.dockerOpsBridge != nil {
		t.Fatal("the operation bridge outlived the hub")
	}
	f.s.closeDockerHosts() // the cleanup of the fixture calls it again
}

// dmFakeObserver stands in for the stacks service: it keeps the callback.
type dmFakeObserver struct {
	fn        func(dockerstacks.Operation)
	cancelled bool
}

func (o *dmFakeObserver) Observe(fn func(dockerstacks.Operation)) func() {
	o.fn = fn
	return func() { o.cancelled = true }
}

// dmRecordingPublisher stands in for the hub: it records what is published.
type dmRecordingPublisher struct {
	mu    sync.Mutex
	hosts []string
	msgs  []dockerwatch.Message
}

func (p *dmRecordingPublisher) Publish(host string, msg dockerwatch.Message) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.hosts = append(p.hosts, host)
	p.msgs = append(p.msgs, msg)
}

func TestBridgeDockerOperationsPublishesStateChangesWithoutOutput(t *testing.T) {
	observer, publisher := &dmFakeObserver{}, &dmRecordingPublisher{}
	cancel := bridgeDockerOperations(observer, publisher)

	observer.fn(dockerstacks.Operation{
		ID: "op_0123456789abcdef", HostID: "nas", Stack: "web", Kind: dockerstacks.KindUp,
		State: dockerstacks.StateRunning, Output: "pulling layer 1/3",
	})
	if len(publisher.msgs) != 1 || publisher.hosts[0] != "nas" || publisher.msgs[0].Event != "operation" {
		t.Fatalf("published %v to %v, want one operation message for the host nas", publisher.msgs, publisher.hosts)
	}
	raw, err := json.Marshal(publisher.msgs[0].Data)
	if err != nil {
		t.Fatal(err)
	}
	var data struct {
		Host      string `json:"host"`
		Operation struct {
			ID     string `json:"id"`
			HostID string `json:"host_id"`
			Stack  string `json:"stack"`
			Kind   string `json:"kind"`
			State  string `json:"state"`
			Output string `json:"output"`
		} `json:"operation"`
	}
	if err := json.Unmarshal(raw, &data); err != nil {
		t.Fatalf("data %s: %v", raw, err)
	}
	if data.Host != "nas" || data.Operation.ID != "op_0123456789abcdef" || data.Operation.HostID != "nas" ||
		data.Operation.Stack != "web" || data.Operation.Kind != "up" || data.Operation.State != "running" || data.Operation.Output != "" {
		t.Fatalf("operation message = %s, want the operation without its output", raw)
	}

	cancel()
	if !observer.cancelled {
		t.Fatal("cancel did not unregister the observer")
	}
}

func TestDockerManagerLiveFailureCodes(t *testing.T) {
	for _, tc := range []struct {
		name         string
		err          error
		notFoundCode string
		status       int
		code         string
	}{
		{"no stream slot", fmt.Errorf("follow: %w", dockerwatch.ErrTooManyStreams), "", http.StatusTooManyRequests, "too_many_streams"},
		{"no stream client", dockerwatch.ErrTooManyClients, "", http.StatusTooManyRequests, "too_many_streams"},
		{"hub closed", dockerwatch.ErrHubClosed, "", http.StatusServiceUnavailable, "docker_hosts_unavailable"},
		{"host forgotten", fmt.Errorf("client: %w", dockerhost.ErrHostNotFound), "", http.StatusNotFound, "host_not_found"},
		{"no such container", &dockerhost.StatusError{Status: 404, Err: errors.New("No such container")}, "container_not_found", http.StatusNotFound, "container_not_found"},
		{"engine refused", &dockerhost.StatusError{Status: 500, Err: errors.New("boom")}, "container_not_found", http.StatusBadGateway, "docker_error"},
		{"no answer", errors.New("dial tcp 127.0.0.1:2375: connection refused"), "", http.StatusBadGateway, "host_unreachable"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := dmLiveFailureOf(tc.err, tc.notFoundCode)
			if got.status != tc.status || got.code != tc.code || got.message == "" {
				t.Fatalf("failure = %+v, want %d %s with a message", got, tc.status, tc.code)
			}
			if tc.code == "host_unreachable" && got.extra["reason"] == "" {
				t.Fatalf("an unreachable host carries its reason: %+v", got)
			}
		})
	}
}

func TestDockerManagerDiskUsageAnswersFromTheHub(t *testing.T) {
	f := newDMLiveFixture(t)
	f.live.usage = dockerhost.DiskUsage{
		Images: dockerhost.DiskUsageGroup{Count: 3, Active: 2, Size: 1000, Reclaimable: 400},
		Total:  1000,
	}
	f.live.usageAt = time.Date(2026, 10, 28, 7, 0, 0, 0, time.UTC)
	f.live.cached = true

	body := f.open(http.MethodGet, "/hosts/local/df?refresh=1", nil).body()
	df := dmObject(t, body, "df")
	images := dmObject(t, df, "images")
	if body["status"] != "ok" || body["fetched_at"] != "2026-10-28T07:00:00Z" || body["cached"] != true ||
		images["count"] != float64(3) || images["reclaimable"] != float64(400) || df["total"] != float64(1000) {
		t.Fatalf("df answer = %v", body)
	}
	if got := dmObject(t, df, "build_cache"); got["size"] != float64(0) {
		t.Fatalf("an empty group still has its members: %v", got)
	}
	f.open(http.MethodGet, "/hosts/local/df", nil).body()
	if got := f.live.dfRefresh; len(got) != 2 || !got[0] || got[1] {
		t.Fatalf("refresh flags the hub saw = %v, want [true false]", got)
	}

	f.open(http.MethodGet, "/hosts/nowhere/df", nil).expectError(http.StatusNotFound, "host_not_found")
	f.open(http.MethodPost, "/hosts/local/df", nil).expectError(http.StatusMethodNotAllowed, "method_not_allowed")
	f.live.dfErr = &dockerhost.StatusError{Status: 500, Err: errors.New("df failed")}
	f.open(http.MethodGet, "/hosts/local/df", nil).expectError(http.StatusBadGateway, "docker_error")
	f.live.dfErr = errors.New("dial tcp: i/o timeout")
	f.open(http.MethodGet, "/hosts/local/df", nil).expectError(http.StatusBadGateway, "host_unreachable")
}
