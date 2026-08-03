package deployer

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"aurago/internal/config"
	"aurago/internal/dockerutil"
)

// TestDockerUpdateRollbackSmoke exercises the real Docker API. It is opt-in so
// ordinary unit test runs never mutate a developer's Docker engine.
func TestDockerUpdateRollbackSmoke(t *testing.T) {
	if os.Getenv("AURAGO_RUN_DOCKER_SMOKE") != "1" {
		t.Skip("set AURAGO_RUN_DOCKER_SMOKE=1 to run the real Docker smoke test")
	}

	const image = "ghcr.io/antibyte/s2s-web@sha256:c980e1db2c3b7deed9d4ede2ab8a861662be8c32b1fbd0c1a024c0117f33487a"
	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	role := "smoke-" + suffix
	containerName := "aurago-speech-lab-" + role
	networkName := "aurago-speech-lab-smoke-" + suffix
	manifest := dockerSmokeManifest("smoke-v1", role, networkName, image)

	var manifestMu sync.RWMutex
	var readyRequests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/manifest":
			manifestMu.RLock()
			defer manifestMu.RUnlock()
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(manifest)
		case "/ready":
			// Install succeeds, the update readiness check fails, and the
			// rollback readiness check succeeds again.
			if readyRequests.Add(1) == 2 {
				http.Error(w, "update intentionally not ready", http.StatusServiceUnavailable)
				return
			}
			w.WriteHeader(http.StatusOK)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	docker := dockerutil.NewClient("", 30*time.Second)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	if status, _ := docker.DoJSON(ctx, http.MethodGet, "/containers/"+containerName+"/json", nil, nil); status != http.StatusNotFound {
		t.Fatalf("refusing to use non-empty smoke container name %q (HTTP %d)", containerName, status)
	}

	cfg := config.SpeechLabConfig{
		Enabled: true,
		BaseURL: server.URL,
		Deployment: config.SpeechLabDeploymentConfig{
			Mode: "managed", Bundle: "stable", AutoStart: true,
		},
	}
	manager := NewManager(cfg, true, true, false, t.TempDir(), nil,
		WithManifestURL(server.URL+"/manifest"),
		WithHTTPClient(server.Client()),
		WithDockerClient(docker),
		WithReadinessTimeout(250*time.Millisecond),
	)
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), time.Minute)
		defer cleanupCancel()
		if err := manager.Remove(cleanupCtx); err != nil {
			t.Errorf("cleanup managed smoke deployment: %v", err)
		}
	})

	if err := manager.Install(ctx); err != nil {
		t.Fatalf("Install() against real Docker error = %v", err)
	}
	installed := manager.Status()
	if installed.Bundle != "smoke-v1" || installed.State != "ready" || installed.Transaction != nil {
		t.Fatalf("installed state = %+v, want published smoke-v1", installed)
	}

	manifestMu.Lock()
	manifest = dockerSmokeManifest("smoke-v2", role, networkName, image)
	manifestMu.Unlock()
	if err := manager.Update(ctx); err == nil || Code(err) != "speech_lab_not_ready" {
		t.Fatalf("Update() error = %v, want speech_lab_not_ready", err)
	}

	restored := manager.Status()
	if restored.Bundle != "smoke-v1" || restored.State != "ready" || restored.Transaction != nil {
		t.Fatalf("rollback state = %+v, want restored smoke-v1", restored)
	}
	var inspected struct {
		Name   string `json:"Name"`
		Config struct {
			Labels map[string]string `json:"Labels"`
		} `json:"Config"`
		State struct {
			Running bool `json:"Running"`
		} `json:"State"`
	}
	status, err := docker.DoJSON(ctx, http.MethodGet, "/containers/"+containerName+"/json", nil, &inspected)
	if err != nil || status != http.StatusOK {
		t.Fatalf("inspect restored container: status=%d err=%v", status, err)
	}
	if strings.TrimPrefix(inspected.Name, "/") != containerName || !inspected.State.Running || !dockerutil.ManagedBy(inspected.Config.Labels, OwnerLabel) {
		t.Fatalf("restored container = %+v", inspected)
	}

	if err := manager.Remove(ctx); err != nil {
		t.Fatalf("Remove() error = %v", err)
	}
	if status, _ := docker.DoJSON(ctx, http.MethodGet, "/containers/"+containerName+"/json", nil, nil); status != http.StatusNotFound {
		t.Fatalf("smoke container still exists after Remove() (HTTP %d)", status)
	}
	if status, _ := docker.DoJSON(ctx, http.MethodGet, "/networks/"+networkName, nil, nil); status != http.StatusNotFound {
		t.Fatalf("smoke network still exists after Remove() (HTTP %d)", status)
	}
}

func dockerSmokeManifest(version, role, networkName, image string) BundleManifest {
	return BundleManifest{
		SchemaVersion: 1, BundleVersion: version, ContractVersion: "speech-lab/v1",
		Publisher: "ghcr.io/antibyte", Network: networkName,
		Images: ImageSet{
			Gateway: image, ASR: image, TTS: image, LLM: image, Web: image, ModelInit: image,
		},
		StartOrder: []string{role},
		Services:   []BundleService{{Role: role, Image: "web", Restart: "no"}},
	}
}
