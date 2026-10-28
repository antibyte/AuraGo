package dockerwatch

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"testing"
	"time"

	"aurago/internal/dockerhost"
	"aurago/internal/dockerstacks"
	"aurago/internal/dockerutil"
)

// The live test runs the live data against a real Docker Engine: a throw-away
// container of the pinned runner image (sleep 60) is created, started, killed
// and removed while a hub client watches host "live". It checks that the events
// arrive through the hub, that a stats point and a host gauge reading carry
// real numbers, that the container is reported gone after it dies, and that
// the disk usage can be read. It is gated by environment variables:
//
//	AURAGO_DOCKER_WATCH_LIVE=1                   switch the test on
//	AURAGO_DOCKER_WATCH_LIVE_ENDPOINT=<host>     Engine endpoint (default: dockerutil.DefaultHost())
//
// The runner image must already be on the engine (run the dockerstacks live
// test or `docker pull docker:29.9.2-cli` once); the test pulls nothing. It
// creates and removes only its own container, named aurago-dw-live-<hex>.
//
//	AURAGO_DOCKER_WATCH_LIVE=1 go test ./internal/dockerwatch/ -run TestLiveWatch -v -count=1 -timeout 5m

// liveHosts serves one host, "live", from a real engine client.
type liveHosts struct{ client *dockerutil.Client }

func (l liveHosts) Host(_ context.Context, id string) (dockerhost.Host, error) {
	if id != "live" {
		return dockerhost.Host{}, dockerhost.ErrHostNotFound
	}
	return dockerhost.Host{ID: "live", Name: "live", Kind: dockerhost.KindLocal}, nil
}

func (l liveHosts) Client(_ context.Context, id string) (*dockerutil.Client, error) {
	if id != "live" {
		return nil, dockerhost.ErrHostNotFound
	}
	return l.client, nil
}

func TestLiveWatch(t *testing.T) {
	if os.Getenv("AURAGO_DOCKER_WATCH_LIVE") != "1" {
		t.Skip("set AURAGO_DOCKER_WATCH_LIVE=1 to run against a real Docker Engine")
	}
	endpoint := os.Getenv("AURAGO_DOCKER_WATCH_LIVE_ENDPOINT")
	if endpoint == "" {
		endpoint = dockerutil.DefaultHost()
	}
	client := dockerutil.NewClient(endpoint, 30*time.Second)
	t.Cleanup(client.CloseIdleConnections)
	ctx := context.Background()

	image := ""
	for _, ref := range []string{dockerstacks.RunnerImage, dockerstacks.RunnerLocalTag} {
		if code, err := client.DoJSON(ctx, http.MethodGet, "images/"+url.PathEscape(ref)+"/json", nil, nil); err == nil {
			image = ref
			break
		} else if code == 0 {
			t.Fatalf("cannot reach the engine at %s: %v", endpoint, err)
		}
	}
	if image == "" {
		t.Skipf("the runner image %s is not on the engine; pull it once (docker pull %s)", dockerstacks.RunnerImage, dockerstacks.RunnerImageTag)
	}

	hub := New(liveHosts{client: client}, Options{
		StatsInterval: time.Second,
		HostInterval:  2 * time.Second,
		Logger:        slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug})),
	})
	t.Cleanup(hub.Close)
	cl, err := hub.Attach()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(cl.Close)

	var suffix [4]byte
	_, _ = rand.Read(suffix[:])
	name := "aurago-dw-live-" + hex.EncodeToString(suffix[:])

	// Watch the host before the container exists, so its create and start are seen.
	if _, err := cl.Set(Subscription{Hosts: []string{"live"}, HostStats: []string{"live"}}); err != nil {
		t.Fatal(err)
	}
	waitLive(t, cl, "host_status live", func(m Message) bool {
		s, ok := m.Data.(HostStatus)
		return m.Event == MsgHostStatus && ok && s.State == StateLive
	})

	var created struct {
		ID string `json:"Id"`
	}
	body := map[string]any{
		"Image":      image,
		"Entrypoint": []string{"sleep"},
		"Cmd":        []string{"60"},
		"Labels":     map[string]string{"aurago.docker-watch-live": "1"},
		"HostConfig": map[string]any{"NetworkMode": "none"},
	}
	if _, err := client.DoJSON(ctx, http.MethodPost, "containers/create?name="+name, body, &created); err != nil {
		t.Fatalf("create %s: %v", name, err)
	}
	t.Cleanup(func() {
		_ = dockerhost.RemoveContainer(context.Background(), client, created.ID, true)
	})
	short := created.ID[:12]

	if _, err := cl.Set(Subscription{Hosts: []string{"live"}, Stats: map[string][]string{"live": {short}}, HostStats: []string{"live"}}); err != nil {
		t.Fatal(err)
	}
	if err := dockerhost.ContainerAction(ctx, client, created.ID, "start"); err != nil {
		t.Fatalf("start: %v", err)
	}
	ev := waitLive(t, cl, "the start event", func(m Message) bool {
		e, ok := m.Data.(Event)
		return m.Event == MsgEvent && ok && e.Type == "container" && e.Action == "start" && e.ID == created.ID
	}).Data.(Event)
	if ev.Name != name || ev.ShortID != short || ev.T == 0 {
		t.Fatalf("start event = %+v, want name %s and short id %s", ev, name, short)
	}

	// Two stats points: the first CPU is unknown (-1), later ones are readings.
	var point StatsPoint
	for seen := 0; seen < 2; seen++ {
		point = waitLive(t, cl, "a stats point", func(m Message) bool { return m.Event == MsgStats }).Data.(StatsMsg).Point
	}
	if point.CPU < -1 || point.Mem == 0 || point.T == 0 {
		t.Fatalf("stats point = %+v, want cpu >= -1 and memory > 0", point)
	}
	hs := waitLive(t, cl, "a host reading with CPU and memory", func(m Message) bool {
		h, ok := m.Data.(HostStats)
		return m.Event == MsgHostStats && ok && h.Running >= 1 && h.MemUsed > 0
	}).Data.(HostStats)
	if hs.MemTotal == 0 || hs.NCPU == 0 {
		t.Fatalf("host stats = %+v, want the host's memory and CPU count", hs)
	}

	if err := dockerhost.ContainerAction(ctx, client, created.ID, "kill"); err != nil {
		t.Fatalf("kill: %v", err)
	}
	die := waitLive(t, cl, "the die event", func(m Message) bool {
		e, ok := m.Data.(Event)
		return m.Event == MsgEvent && ok && e.Action == "die" && e.ID == created.ID
	}).Data.(Event)
	if die.ExitCode == nil || *die.ExitCode != 137 {
		t.Fatalf("die event = %+v, want exit code 137 after a kill", die)
	}
	waitLive(t, cl, "stats_gone after the container died", func(m Message) bool {
		g, ok := m.Data.(StatsGoneMsg)
		return m.Event == MsgStatsGone && ok && g.ID == short
	})

	usage, at, cached, err := hub.DiskUsage(ctx, "live", true)
	if err != nil || cached || at.IsZero() || usage.Total < 0 || usage.Images.Count == 0 {
		t.Fatalf("DiskUsage = %+v at %v cached %v err %v, want a fresh reading with at least the runner image", usage, at, cached, err)
	}
	t.Logf("disk usage: %+v", usage)
}

// waitLive reads the client's messages until pick accepts one (30 s at most).
func waitLive(t *testing.T, cl *Client, what string, pick func(Message) bool) Message {
	t.Helper()
	deadline := time.After(30 * time.Second)
	for {
		select {
		case m, ok := <-cl.Messages():
			if !ok {
				t.Fatalf("the stream closed while waiting for %s", what)
			}
			if pick(m) {
				return m
			}
		case <-deadline:
			t.Fatalf("timed out waiting for %s", what)
		}
	}
}
