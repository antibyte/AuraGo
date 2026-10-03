package tools

import (
	"errors"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"

	"aurago/internal/dockerutil"
)

func TestDockerContainerOwnershipFailsClosedWhenInspectAndListFail(t *testing.T) {
	host := fakeDockerHost(t, func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "daemon overloaded", http.StatusInternalServerError)
	})
	cfg := DockerConfig{Host: host}
	owned, err := DockerContainerOwnership(cfg, "abc123", dockerutil.LocalLLMOwner, dockerutil.AppOwner)
	if !errors.Is(err, ErrDockerOwnershipUnverified) {
		t.Fatalf("ownership error = %v, want ErrDockerOwnershipUnverified", err)
	}
	if len(owned) != 0 {
		t.Fatalf("owners = %v, want none proven", owned)
	}
	if !DockerContainerManagedBy(cfg, "abc123", dockerutil.LocalLLMOwner) {
		t.Fatal("DockerContainerManagedBy failed open after inspect and list errors")
	}
}

func TestDockerContainerOwnershipNotFoundIsNotOwned(t *testing.T) {
	host := fakeDockerHost(t, func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/containers/abc123/json") {
			http.Error(w, `{"message":"No such container: abc123"}`, http.StatusNotFound)
			return
		}
		http.Error(w, "list unavailable", http.StatusInternalServerError)
	})
	owned, err := DockerContainerOwnership(DockerConfig{Host: host}, "abc123", dockerutil.LocalLLMOwner)
	if err != nil || owned[dockerutil.LocalLLMOwner] {
		t.Fatalf("missing container ownership = %v, %v; want not owned without error", owned, err)
	}
}

func TestDockerContainerOwnershipResolvesAllOwnersFromOneInspect(t *testing.T) {
	var requests atomic.Int32
	host := fakeDockerHost(t, func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		if strings.HasSuffix(r.URL.Path, "/containers/abc123/json") {
			_, _ = w.Write([]byte(`{"Name":"/renamed","Config":{"Labels":{"com.aurago.managed":"true","com.aurago.owner":"go2rtc"}}}`))
			return
		}
		t.Errorf("unexpected Docker request %s", r.URL.Path)
	})
	owned, err := DockerContainerOwnership(DockerConfig{Host: host}, "abc123", "go2rtc", dockerutil.LocalLLMOwner)
	if err != nil || !owned["go2rtc"] || owned[dockerutil.LocalLLMOwner] || requests.Load() != 1 {
		t.Fatalf("owned=%v err=%v requests=%d", owned, err, requests.Load())
	}
}
