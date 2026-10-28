package dockerwatch

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"aurago/internal/dockerhost"
	"aurago/internal/dockerstacks"
	"aurago/internal/dockerutil"
)

// The live tests of the log followers and of exec run against a real Docker
// Engine. They are gated by environment variables:
//
//	AURAGO_DOCKER_WATCH_LIVE=1                  switch the tests on
//	AURAGO_DOCKER_WATCH_LIVE_ENDPOINT=<host>    Engine endpoint (default: dockerutil.DefaultHost())
//
// Every test creates throw-away containers of the stacks runner image
// (dockerstacks.RunnerImage, which must already be on the Engine: prepare the
// host in the Docker Manager first, or `docker pull docker:29.9.2-cli`) named
// aurago-dw-live-logs-<random hex>[-b] and removes only those. The containers
// print numbered lines to stdout and stderr once a second.
//
//	AURAGO_DOCKER_WATCH_LIVE=1 go test ./internal/dockerwatch/ -run 'TestLive(FollowContainer|Exec|FollowProject)' -v -count=1

type logsLiveHosts struct{ client *dockerutil.Client }

func (l logsLiveHosts) Host(context.Context, string) (dockerhost.Host, error) {
	return dockerhost.Host{ID: "live", Name: "live", Kind: dockerhost.KindLocal}, nil
}

func (l logsLiveHosts) Client(context.Context, string) (*dockerutil.Client, error) {
	return l.client, nil
}

// logsLiveEngine connects to the Engine, finds the runner image and returns a
// hub plus a function that creates a started throw-away container.
func logsLiveEngine(t *testing.T) (*Hub, *dockerutil.Client, func(suffix, project, service string) string) {
	t.Helper()
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
	for _, ref := range []string{dockerstacks.RunnerImage, dockerstacks.RunnerImageTag} {
		if status, err := client.DoJSON(ctx, http.MethodGet, "images/"+url.PathEscape(ref)+"/json", nil, nil); err == nil && status == http.StatusOK {
			image = ref
			break
		}
	}
	if image == "" {
		t.Skipf("the runner image %s is not on the Engine at %s; prepare the host in the Docker Manager or docker pull %s", dockerstacks.RunnerImage, endpoint, dockerstacks.RunnerImageTag)
	}

	var rnd [4]byte
	_, _ = rand.Read(rnd[:])
	prefix := "aurago-dw-live-logs-" + hex.EncodeToString(rnd[:])

	hub := New(logsLiveHosts{client: client}, Options{})
	t.Cleanup(hub.Close)

	create := func(suffix, project, service string) string {
		t.Helper()
		name := prefix + suffix
		labels := map[string]string{"aurago.dw-live": "1"}
		if project != "" {
			labels["com.docker.compose.project"] = project
			labels["com.docker.compose.service"] = service
		}
		script := `i=0; while true; do echo "out-$i"; echo "err-$i" >&2; i=$((i+1)); sleep 1; done`
		body := map[string]any{"Image": image, "Entrypoint": []string{"/bin/sh", "-c"}, "Cmd": []string{script}, "Labels": labels}
		var created struct {
			ID string `json:"Id"`
		}
		if _, err := client.DoJSON(ctx, http.MethodPost, "containers/create?name="+name, body, &created); err != nil {
			t.Fatalf("create %s: %v", name, err)
		}
		t.Cleanup(func() {
			removeCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			if err := dockerhost.RemoveContainer(removeCtx, client, name, true); err != nil {
				t.Logf("remove %s: %v", name, err)
			}
		})
		if err := dockerhost.ContainerAction(ctx, client, name, "start"); err != nil {
			t.Fatalf("start %s: %v", name, err)
		}
		return name
	}
	return hub, client, create
}

func TestLiveFollowContainer(t *testing.T) {
	hub, _, create := logsLiveEngine(t)
	name := create("", "", "")
	time.Sleep(3 * time.Second) // some history

	sink := &logsSink{}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- hub.FollowContainer(ctx, "live", name, FollowOptions{Tail: 20}, sink.out, sink.notice) }()

	deadline := time.Now().Add(20 * time.Second)
	for len(sink.lines()) < 12 && time.Now().Before(deadline) { // about 3 s of history, then 3 s live
		time.Sleep(100 * time.Millisecond)
	}
	cancel()
	<-done
	lines := sink.lines()
	if len(lines) < 12 {
		t.Fatalf("saw %d log lines in 20 s, want at least 12 (history plus live): %+v", len(lines), lines)
	}
	var outs, errs []string
	for _, l := range lines {
		if l.Svc != "" || l.Ctr != "" {
			t.Fatalf("a single container stream carries %+v", l)
		}
		switch l.S {
		case "out":
			outs = append(outs, l.Text)
		case "err":
			errs = append(errs, l.Text)
		default:
			t.Fatalf("stream %q", l.S)
		}
	}
	if len(outs) < 4 || len(errs) < 4 {
		t.Fatalf("stdout lines %v, stderr lines %v; both streams must arrive demultiplexed", outs, errs)
	}
	for i := 1; i < len(outs); i++ { // consecutive numbers: nothing repeated or lost between history and live
		var prev, cur int
		_, _ = fmt.Sscanf(outs[i-1], "out-%d", &prev)
		_, _ = fmt.Sscanf(outs[i], "out-%d", &cur)
		if cur != prev+1 {
			t.Fatalf("stdout lines %v are not consecutive at %d", outs, i)
		}
	}
	t.Logf("followed %s: %d lines, first %q last %q", name, len(lines), lines[0].Text, lines[len(lines)-1].Text)
}

func TestLiveExecRoundTrip(t *testing.T) {
	_, client, create := logsLiveEngine(t)
	name := create("", "", "")
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	id, err := dockerhost.ExecCreate(ctx, client, name, []string{"/bin/sh", "-c", "echo round-trip-$((6*7)); sleep 1"}, []string{"TERM=xterm-256color"}, true)
	if err != nil {
		t.Fatalf("ExecCreate: %v", err)
	}
	stream, err := dockerhost.ExecStart(ctx, client, id)
	if err != nil {
		t.Fatalf("ExecStart: %v", err)
	}
	defer stream.Close()
	if err := dockerhost.ExecResize(ctx, client, id, 100, 30); err != nil {
		t.Logf("ExecResize: %v (the exec may already be gone)", err)
	}
	got := logsReadExec(t, stream)
	if !strings.Contains(got, "round-trip-42") {
		t.Fatalf("exec output = %q, want round-trip-42", got)
	}

	// A detached exec runs without a stream: it leaves a file the next exec finds.
	detached, err := dockerhost.ExecCreate(ctx, client, name, []string{"/bin/sh", "-c", "touch /tmp/aurago-dw-live-detached"}, nil, false)
	if err != nil {
		t.Fatalf("ExecCreate (detached): %v", err)
	}
	if err := dockerhost.ExecStartDetached(ctx, client, detached); err != nil {
		t.Fatalf("ExecStartDetached: %v", err)
	}
	time.Sleep(time.Second)
	check, err := dockerhost.ExecCreate(ctx, client, name, []string{"/bin/sh", "-c", "ls /tmp/aurago-dw-live-detached"}, nil, true)
	if err != nil {
		t.Fatalf("ExecCreate (check): %v", err)
	}
	cs, err := dockerhost.ExecStart(ctx, client, check)
	if err != nil {
		t.Fatalf("ExecStart (check): %v", err)
	}
	defer cs.Close()
	if listing := logsReadExec(t, cs); !strings.Contains(listing, "/tmp/aurago-dw-live-detached") {
		t.Fatalf("the detached exec left no file: %q", listing)
	}
}

func TestLiveFollowProjectAttachesOnStart(t *testing.T) {
	hub, _, create := logsLiveEngine(t)
	var rnd [3]byte
	_, _ = rand.Read(rnd[:])
	project := "aurago-dw-live-" + hex.EncodeToString(rnd[:])
	create("", project, "first")
	time.Sleep(2 * time.Second)

	sink := &logsSink{}
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	done := make(chan error, 1)
	go func() {
		done <- hub.FollowProject(ctx, "live", ProjectSelector{Project: project}, FollowOptions{Tail: 10}, sink.out, sink.notice)
	}()
	waitFor := func(what string, cond func() bool) {
		t.Helper()
		deadline := time.Now().Add(30 * time.Second)
		for !cond() && time.Now().Before(deadline) {
			time.Sleep(100 * time.Millisecond)
		}
		if !cond() {
			t.Fatalf("timed out waiting for %s; lines %d, notices %+v", what, len(sink.lines()), sink.noticeList())
		}
	}
	hasService := func(svc string) bool {
		for _, l := range sink.lines() {
			if l.Svc == svc {
				return true
			}
		}
		return false
	}
	waitFor("lines of the first service", func() bool { return hasService("first") })

	create("-b", project, "second") // the events watcher reports its start
	waitFor("lines of the second service", func() bool { return hasService("second") })
	if n := sink.countNotices(NoticeAttached); n < 1 {
		t.Fatalf("no attached notice for the second service: %+v", sink.noticeList())
	}
	cancel()
	<-done
}

// logsReadExec reads an exec stream until the process ends (the Engine closes it).
func logsReadExec(t *testing.T, stream io.Reader) string {
	t.Helper()
	got := make(chan string, 1)
	go func() {
		b, _ := io.ReadAll(stream)
		got <- string(b)
	}()
	select {
	case out := <-got:
		return out
	case <-time.After(15 * time.Second):
		t.Fatal("the exec stream did not end")
		return ""
	}
}
