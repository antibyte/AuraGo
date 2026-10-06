package server

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"

	"aurago/internal/invasion"
	"aurago/internal/security"
	"aurago/internal/testutil"
)

const sharedKeyNestID = "12345678-abcd-ef12-3456-7890abcdef12"

func newSharedKeyTestServer(t *testing.T, previous string) *Server {
	t.Helper()
	vault, err := security.NewVault(strings.Repeat("c", 64), filepath.Join(t.TempDir(), "vault.bin"))
	if err != nil {
		t.Fatal(err)
	}
	if previous != "" {
		if err := vault.WriteSecret("egg_shared_"+sharedKeyNestID, previous); err != nil {
			t.Fatal(err)
		}
	}
	return &Server{Vault: vault, Logger: slog.New(slog.NewTextHandler(io.Discard, nil))}
}

func storedSharedKey(t *testing.T, s *Server) string {
	t.Helper()
	key, err := s.Vault.ReadSecret("egg_shared_" + sharedKeyNestID)
	if err != nil {
		t.Fatal(err)
	}
	return key
}

var errNotDelivered = fmt.Errorf("failed to pull image: pull failed with HTTP 404: %w", invasion.ErrEggConfigNotDelivered)

func TestReplaceEggSharedKeyRestoresThePreviousKeyWhenTheConfigWasNotDelivered(t *testing.T) {
	s := newSharedKeyTestServer(t, "old-key")
	undo, err := s.replaceEggSharedKey(sharedKeyNestID, "new-key")
	if err != nil {
		t.Fatal(err)
	}
	if got := storedSharedKey(t, s); got != "new-key" {
		t.Fatalf("key before deploy = %q, want the hatch's key stored first", got)
	}
	if !undo(errNotDelivered) {
		t.Fatal("undo did not restore the previous key")
	}
	if got := storedSharedKey(t, s); got != "old-key" {
		t.Fatalf("key = %q, want old-key so the still-running egg can reconnect", got)
	}
}

func TestReplaceEggSharedKeyKeepsTheNewKeyOnceTheConfigMayHaveBeenDelivered(t *testing.T) {
	for name, deployErr := range map[string]error{"success": nil, "start failed": errors.New("container start failed (500): busy")} {
		s := newSharedKeyTestServer(t, "old-key")
		undo, err := s.replaceEggSharedKey(sharedKeyNestID, "new-key")
		if err != nil {
			t.Fatal(err)
		}
		if undo(deployErr) || storedSharedKey(t, s) != "new-key" {
			t.Fatalf("%s: the new key must stay", name)
		}
	}
}

func TestReplaceEggSharedKeyWithoutAPreviousKeyKeepsTheNewKey(t *testing.T) {
	s := newSharedKeyTestServer(t, "")
	undo, _ := s.replaceEggSharedKey(sharedKeyNestID, "new-key")
	if undo(errNotDelivered) || storedSharedKey(t, s) != "new-key" {
		t.Fatal("without a previous key nothing may be restored")
	}
}

func TestReplaceEggSharedKeyLeavesAConcurrentRotationAlone(t *testing.T) {
	s := newSharedKeyTestServer(t, "old-key")
	undo, _ := s.replaceEggSharedKey(sharedKeyNestID, "new-key")
	if err := s.Vault.WriteSecret("egg_shared_"+sharedKeyNestID, "rotated-key"); err != nil {
		t.Fatal(err)
	}
	if undo(errNotDelivered) || storedSharedKey(t, s) != "rotated-key" {
		t.Fatal("a key written after the hatch must win")
	}
}

func TestReplaceEggSharedKeyFailsWithoutAVault(t *testing.T) {
	if _, err := (&Server{}).replaceEggSharedKey(sharedKeyNestID, "new-key"); err == nil || !strings.Contains(err.Error(), "vault is unavailable") {
		t.Fatalf("err = %v, want the unchanged vault error", err)
	}
}

// hatchKeyEngine is a Docker Engine for the shared key tests: pullStatus and
// startStatus answer the image pull and the container start.
func hatchKeyEngine(t *testing.T, pullStatus, startStatus int) (invasion.NestRecord, *atomic.Int64) {
	t.Helper()
	var archives atomic.Int64
	ts := testutil.NewHTTPServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path
		switch {
		case path == "/version":
			_, _ = io.WriteString(w, `{"ApiVersion":"1.45","MinAPIVersion":"1.25"}`)
		case strings.HasSuffix(path, "/images/create"):
			w.WriteHeader(pullStatus)
			if pullStatus == http.StatusOK {
				_, _ = io.WriteString(w, "{\"status\":\"Status: Image is up to date\"}\n")
			} else {
				_, _ = io.WriteString(w, `{"message":"manifest unknown"}`)
			}
		case strings.HasSuffix(path, "/containers/create"):
			w.WriteHeader(http.StatusCreated)
			_, _ = io.WriteString(w, `{"Id":"abc123"}`)
		case strings.HasSuffix(path, "/archive"):
			archives.Add(1)
			w.WriteHeader(http.StatusOK)
		case strings.HasSuffix(path, "/start"):
			w.WriteHeader(startStatus)
			_, _ = io.WriteString(w, `{"message":"port is already allocated"}`)
		default:
			w.WriteHeader(http.StatusNoContent)
		}
	}))
	t.Cleanup(ts.Close)
	host, portText, _ := net.SplitHostPort(strings.TrimPrefix(ts.URL, "http://"))
	port, _ := strconv.Atoi(portText)
	return invasion.NestRecord{ID: sharedKeyNestID, Host: host, Port: port, DeployMethod: "docker_remote"}, &archives
}

func TestHatchKeyIsRestoredAfterADockerPullHTTPFailure(t *testing.T) {
	s := newSharedKeyTestServer(t, "old-key")
	nest, archives := hatchKeyEngine(t, http.StatusNotFound, http.StatusNoContent)
	undo, _ := s.replaceEggSharedKey(nest.ID, "new-key")
	err := (&invasion.DockerConnector{}).Deploy(context.Background(), nest, nil, invasion.EggDeployPayload{ConfigYAML: []byte("egg_mode: {}\n")})
	if err == nil || !undo(err) || storedSharedKey(t, s) != "old-key" || archives.Load() != 0 {
		t.Fatalf("pull 404: err=%v key=%q archives=%d, want the old key back", err, storedSharedKey(t, s), archives.Load())
	}
}

func TestHatchKeyIsKeptAfterADockerStartFailure(t *testing.T) {
	s := newSharedKeyTestServer(t, "old-key")
	nest, archives := hatchKeyEngine(t, http.StatusOK, http.StatusInternalServerError)
	undo, _ := s.replaceEggSharedKey(nest.ID, "new-key")
	err := (&invasion.DockerConnector{}).Deploy(context.Background(), nest, nil, invasion.EggDeployPayload{ConfigYAML: []byte("egg_mode: {}\n")})
	if err == nil || undo(err) || storedSharedKey(t, s) != "new-key" || archives.Load() != 1 {
		t.Fatalf("start 500: err=%v key=%q archives=%d, want the new key kept", err, storedSharedKey(t, s), archives.Load())
	}
}
