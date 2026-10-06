package server

import (
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"runtime"
	"testing"

	"aurago/internal/tools"
)

// TestContainerSelfMarkerLive runs inside a scratch container that joins a
// scratch provider's network namespace, with the Docker socket mounted. It
// needs AURAGO_LIVE_SELF_MARKER=1 and the full IDs of the container it runs in
// (AURAGO_LIVE_SELF_ID), the provider (AURAGO_LIVE_SELF_PROVIDER) and an
// unrelated container (AURAGO_LIVE_SELF_OTHER). It sends only read requests;
// the update it asks for is refused before any Docker change.
func TestContainerSelfMarkerLive(t *testing.T) {
	if os.Getenv("AURAGO_LIVE_SELF_MARKER") != "1" || runtime.GOOS != "linux" {
		t.Skip("set AURAGO_LIVE_SELF_MARKER=1 inside a scratch container on a Linux Docker host")
	}
	selfID, providerID, otherID := os.Getenv("AURAGO_LIVE_SELF_ID"), os.Getenv("AURAGO_LIVE_SELF_PROVIDER"), os.Getenv("AURAGO_LIVE_SELF_OTHER")
	if len(selfID) < 12 || len(providerID) < 12 || len(otherID) < 12 {
		t.Fatal("AURAGO_LIVE_SELF_ID, AURAGO_LIVE_SELF_PROVIDER and AURAGO_LIVE_SELF_OTHER must name full container IDs")
	}
	tools.ConfigureRuntimePermissions(tools.RuntimePermissions{DockerEnabled: true})
	t.Cleanup(tools.ClearRuntimePermissionsForTest)
	host := "unix:///var/run/docker.sock"
	s := testContainerServer(true, false)
	s.Cfg.Runtime.IsDocker = true
	s.Cfg.Docker.Host = host
	cfg := tools.DockerConfig{Host: host}
	ctx := context.Background()
	t.Cleanup(replaceContainerSelfMarker(""))

	// Without the marker: today's confirmation on the containerd image store.
	if got := classifyContainerForAction(ctx, s, cfg, selfID); got.Self || !got.SharedNetwork {
		t.Fatalf("own container without a marker = %+v, want shared-network (today)", got)
	}
	initContainerSelfMarker(true, slog.Default())
	marker := currentContainerSelfMarker()
	if marker == "" {
		t.Fatal("no marker written in the Docker runtime")
	}
	t.Cleanup(func() { _ = os.Remove(marker) })
	t.Logf("marker %s", marker)

	if got := classifyContainerForAction(ctx, s, cfg, selfID); !got.Self || got.SharedNetwork {
		t.Fatalf("own container with the marker = %+v, want self", got)
	}
	if got := classifyContainerForAction(ctx, s, cfg, providerID); got.Self || !got.SharedNetwork {
		t.Fatalf("network provider = %+v, want shared-network", got)
	}
	if got := classifyContainerForAction(ctx, s, cfg, otherID); got.Self || got.SharedNetwork {
		t.Fatalf("unrelated container = %+v, want neither self nor shared-network", got)
	}
	rec := httptest.NewRecorder()
	handleContainerAction(s)(rec, httptest.NewRequest(http.MethodPost, "/api/containers/"+selfID+"/update", nil))
	if body := decodeContainerResponse(t, rec); rec.Code != http.StatusConflict || body["code"] != containerCodeSelfUpdateUnsupported || body["owner"] != "self" {
		t.Fatalf("own update = %d %v, want 409 %s", rec.Code, body, containerCodeSelfUpdateUnsupported)
	}
	flags := listFlags(t, s)
	if c := flags[selfID[:12]]; c["self"] != true || c["shared_network"] != nil {
		t.Fatalf("list own entry = %v, want self only", c)
	}
	if c := flags[providerID[:12]]; c["self"] != nil || c["shared_network"] != true {
		t.Fatalf("list provider entry = %v, want shared_network", c)
	}
	if c := flags[otherID[:12]]; c["self"] != nil || c["shared_network"] != nil {
		t.Fatalf("list unrelated entry = %v, want no self flags", c)
	}
}
