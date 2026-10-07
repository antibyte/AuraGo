package server

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"runtime"
	"strings"
	"testing"
	"time"

	"aurago/internal/tools"
)

// TestDockerTerminalEndLive runs the real End path against the local Docker
// engine. It needs AURAGO_LIVE_DOCKER_TERMINAL_END=1, creates and removes only
// its own scratch container, and never pulls (AURAGO_LIVE_TERMINAL_IMAGE,
// default caddy:2.11.2-alpine, must be present).
func TestDockerTerminalEndLive(t *testing.T) {
	if os.Getenv("AURAGO_LIVE_DOCKER_TERMINAL_END") != "1" || runtime.GOOS != "linux" {
		t.Skip("set AURAGO_LIVE_DOCKER_TERMINAL_END=1 on a Linux Docker host")
	}
	image := os.Getenv("AURAGO_LIVE_TERMINAL_IMAGE")
	if image == "" {
		image = "caddy:2.11.2-alpine"
	}
	tools.ConfigureRuntimePermissions(tools.RuntimePermissions{DockerEnabled: true})
	t.Cleanup(tools.ClearRuntimePermissionsForTest)
	cfg := tools.DockerConfig{Host: "unix:///var/run/docker.sock"}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	name := fmt.Sprintf("aurago-endsession-probe-%d", time.Now().UnixNano())
	body, _ := json.Marshal(map[string]interface{}{"Image": image, "Cmd": []string{"sleep", "600"}})
	if data, code, err := tools.DockerRequestContext(ctx, cfg, http.MethodPost, "/containers/create?name="+name, string(body)); err != nil || code != http.StatusCreated {
		t.Fatalf("create scratch container: %d %s %v (the image must be present locally)", code, data, err)
	}
	t.Cleanup(func() {
		_, _, _ = tools.DockerRequestContext(context.Background(), cfg, http.MethodDelete, "/containers/"+name+"?force=true&v=true", "")
	})
	if data, code, err := tools.DockerRequestContext(ctx, cfg, http.MethodPost, "/containers/"+name+"/start", ""); err != nil || code != http.StatusNoContent {
		t.Fatalf("start scratch container: %d %s %v", code, data, err)
	}

	backend := dockerContainerTerminalBackend{}
	ended, err := backend.CreateSession(ctx, cfg, name, 80, 24, nil)
	if err != nil {
		t.Fatalf("open session: %v", err)
	}
	defer ended.Close()
	other, err := backend.CreateSession(ctx, cfg, name, 80, 24, nil)
	if err != nil {
		t.Fatalf("open second session: %v", err)
	}
	defer other.Close()

	// A daemon started from the session (like a tmux server) must survive End.
	if _, err := ended.Write([]byte("setsid sleep 2001 </dev/null >/dev/null 2>&1 &\n")); err != nil {
		t.Fatal(err)
	}
	time.Sleep(time.Second)
	if err := ended.(containerTerminalEnder).End(ctx); err != nil {
		t.Fatalf("End: %v", err)
	}
	endedEOF := make(chan struct{})
	go func() { _, _ = io.Copy(io.Discard, ended); close(endedEOF) }()
	select {
	case <-endedEOF:
	case <-time.After(10 * time.Second):
		t.Fatal("the ended shell's stream did not close within 10 s")
	}
	data, code, err := tools.DockerRequestContext(ctx, cfg, http.MethodGet, "/containers/"+name+"/top", "")
	if err != nil || code != http.StatusOK || !strings.Contains(string(data), "sleep 2001") {
		t.Fatalf("the setsid daemon did not survive End: %d %s %v", code, data, err)
	}
	// The other session is untouched.
	if _, err := other.Write([]byte("echo still-here-$((40+2))\n")); err != nil {
		t.Fatalf("write to the other session: %v", err)
	}
	answered := make(chan bool, 1)
	go func() {
		var seen strings.Builder
		buf := make([]byte, 4096)
		for {
			n, err := other.Read(buf)
			seen.Write(buf[:n])
			if strings.Contains(seen.String(), "still-here-42") {
				answered <- true
				return
			}
			if err != nil {
				answered <- false
				return
			}
		}
	}()
	select {
	case ok := <-answered:
		if !ok {
			t.Fatal("the other session ended too")
		}
	case <-time.After(10 * time.Second):
		t.Fatal("the other session stopped answering after End")
	}
}
