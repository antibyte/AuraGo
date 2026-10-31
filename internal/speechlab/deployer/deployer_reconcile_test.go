package deployer

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"os"
	"reflect"
	"strings"
	"testing"

	"aurago/internal/config"
)

func TestAutoStartReconcilesReplacedServicesAndPrunedInitializer(t *testing.T) {
	manifest := validV2Manifest()
	fake := newFakeDocker(t, manifest)
	manager := fake.manager(t, managedSpeechLabConfig(fake.server.URL), t.TempDir())
	if err := manager.Install(context.Background()); err != nil {
		t.Fatal(err)
	}
	fake.mu.Lock()
	initializer := fake.find("aurago-speech-lab-control-init")
	delete(fake.containers, initializer.ID)
	gateway := fake.find("aurago-speech-lab-gateway")
	delete(fake.containers, gateway.ID)
	gateway.ID = "replacement-gateway"
	fake.containers[gateway.ID] = gateway
	wantStack, pulls, creates := fake.stack, fake.pulls, fake.creates
	fake.mu.Unlock()
	manager.fail(&Error{Code: "speech_lab_start_failed", Err: errors.New("stale container")})
	if err := manager.AutoStart(context.Background()); err != nil {
		t.Fatalf("repair stale deployment: %v", err)
	}
	state := manager.Status()
	wantIDs := []string{fake.find("aurago-speech-lab-controller").ID, gateway.ID}
	if state.State != "ready" || state.LastErrorCode != "" || state.LastError != "" || !reflect.DeepEqual(state.ContainerIDs, wantIDs) {
		t.Fatalf("reconciled deployment = %+v", state)
	}
	if fake.pulls != pulls || fake.creates != creates || fake.stack != wantStack {
		t.Fatal("reconciliation changed images, containers or the active stack")
	}
	raw, err := os.ReadFile(manager.dataPath)
	if err != nil {
		t.Fatal(err)
	}
	var saved State
	if err := json.Unmarshal(raw, &saved); err != nil || !reflect.DeepEqual(saved.ContainerIDs, wantIDs) || saved.State != "ready" {
		t.Fatalf("repaired state was not persisted: %+v, %v", saved, err)
	}
}

func TestStartReconciliationRejectsDriftBeforeMutatingContainers(t *testing.T) {
	for _, kind := range []string{"bundle", "owner", "component", "role", "bundle_label", "image", "fingerprint", "network", "missing_service", "persistence"} {
		t.Run(kind, func(t *testing.T) {
			manifest := validManifest()
			fake := newFakeDocker(t, manifest)
			manager := fake.manager(t, managedSpeechLabConfig(fake.server.URL), t.TempDir())
			if err := manager.Install(context.Background()); err != nil {
				t.Fatal(err)
			}
			gateway := fake.find("aurago-speech-lab-gateway")
			gateway.Running = false
			manager.state.ContainerIDs = []string{"stale-id"}
			switch kind {
			case "bundle":
				manifest.BundleVersion = "new-bundle"
				fake.manifest, _ = json.Marshal(manifest)
			case "owner":
				gateway.Labels["aurago.managed"] = "someone-else"
			case "component":
				gateway.Labels["aurago.component"] = "other"
			case "role":
				gateway.Labels["aurago.role"] = "web"
			case "bundle_label":
				gateway.Labels["aurago.bundle"] = "other"
			case "image":
				gateway.Image = manifest.Images.Web
			case "fingerprint":
				gateway.Labels["aurago.fingerprint"] = "other"
			case "network":
				gateway.Attached = false
			case "missing_service":
				delete(fake.containers, gateway.ID)
			case "persistence":
				manager.stateWriter = func(string, []byte, os.FileMode) error { return os.ErrPermission }
			}
			pulls, creates := fake.pulls, fake.creates
			if err := manager.Start(context.Background()); err == nil {
				t.Fatal("invalid deployment was accepted")
			}
			if gateway.Running || fake.pulls != pulls || fake.creates != creates {
				t.Fatal("failed reconciliation mutated Docker")
			}
			if manager.Status().State != "error" {
				t.Fatal("failed reconciliation was reported as ready")
			}
		})
	}
}

func TestAutoStartRechecksReadyBeforeClearingPreviousError(t *testing.T) {
	for _, ready := range []bool{false, true} {
		t.Run(map[bool]string{false: "unhealthy", true: "healthy"}[ready], func(t *testing.T) {
			fake := newFakeDocker(t, validManifest())
			manager := fake.manager(t, managedSpeechLabConfig(fake.server.URL), "")
			if err := manager.Install(context.Background()); err != nil {
				t.Fatal(err)
			}
			manager.fail(&Error{Code: "speech_lab_start_failed", Err: errors.New("stale container")})
			fake.ready = ready
			err := manager.AutoStart(context.Background())
			state := manager.Status()
			if ready {
				if err != nil || state.State != "ready" || state.LastErrorCode != "" || state.LastError != "" {
					t.Fatalf("healthy deployment retained stale error: %+v, %v", state, err)
				}
			} else if Code(err) != "speech_lab_not_ready" || state.State != "error" {
				t.Fatalf("unhealthy deployment accepted: %+v, %v", state, err)
			}
		})
	}
}

func TestStartDoesNotTrustStoredIDOwnership(t *testing.T) {
	fake := newFakeDocker(t, validManifest())
	manager := fake.manager(t, managedSpeechLabConfig(fake.server.URL), "")
	if err := manager.Install(context.Background()); err != nil {
		t.Fatal(err)
	}
	gateway := fake.find("aurago-speech-lab-gateway")
	gateway.Running = false
	gateway.Labels["aurago.managed"] = "someone-else"
	if err := manager.Start(context.Background()); err == nil || gateway.Running {
		t.Fatal("Start trusted a stale ownership claim")
	}
}

func TestReconciliationUsesInstalledHardwareProfile(t *testing.T) {
	fake := newFakeDocker(t, validManifest())
	cfg := managedSpeechLabConfig(fake.server.URL)
	cfg.Deployment.GPUBackend = config.SpeechLabGPUBackendCPU
	manager := fake.manager(t, cfg, "")
	if err := manager.Install(context.Background()); err != nil {
		t.Fatal(err)
	}
	manager.state.ContainerIDs = []string{"stale-id"}
	cfg.Deployment.GPUBackend = config.SpeechLabGPUBackendCUDA
	manager.Reconfigure(cfg)
	if err := manager.Start(context.Background()); err != nil {
		t.Fatalf("saved hardware draft changed installed deployment reconciliation: %v", err)
	}
}

func TestStartDoesNotRerunCompletedInitializer(t *testing.T) {
	fake := newFakeDocker(t, validV2Manifest())
	manager := fake.manager(t, managedSpeechLabConfig(fake.server.URL), "")
	if err := manager.Install(context.Background()); err != nil {
		t.Fatal(err)
	}
	initializer := fake.find("aurago-speech-lab-control-init")
	initializer.Running, initializer.Status = false, "exited"
	if err := manager.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	if initializer.Running {
		t.Fatal("completed token initializer was started again")
	}
}

type reconciliationTransport func(*http.Request) (*http.Response, error)

func (f reconciliationTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestReconciliationFetchesInstalledReleaseAndSignature(t *testing.T) {
	fake := newFakeDocker(t, validV2Manifest())
	manager := fake.manager(t, managedSpeechLabConfig(fake.server.URL), "")
	if err := manager.Install(context.Background()); err != nil {
		t.Fatal(err)
	}
	manager.state.ContainerIDs = []string{"stale-id"}
	manager.manifestURL = ManifestURL
	originalClient := manager.httpClient
	var paths []string
	manager.httpClient = &http.Client{Transport: reconciliationTransport(func(r *http.Request) (*http.Response, error) {
		if r.URL.Host == "github.com" {
			paths = append(paths, r.URL.Path)
			if !strings.HasPrefix(r.URL.Path, "/antibyte/s2s/releases/download/stable/speech-lab-bundle.json") {
				t.Errorf("repair fetched a different release: %s", r.URL.Path)
			}
			r = r.Clone(r.Context())
			suffix := strings.TrimPrefix(r.URL.Path, "/antibyte/s2s/releases/download/stable/speech-lab-bundle.json")
			r.URL, _ = url.Parse(fake.server.URL + "/manifest" + suffix)
		}
		return originalClient.Do(r)
	})}
	if err := manager.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(paths) != 2 || !strings.HasSuffix(paths[1], ".sig") {
		t.Fatalf("installed release manifest/signature requests = %v", paths)
	}
}

func TestAutoStartStatusRepairPreservesModuleSelection(t *testing.T) {
	for _, tc := range []struct {
		name   string
		pruned bool
		ready  bool
	}{
		{name: "status_only", ready: true},
		{name: "pruned_initializer", pruned: true, ready: true},
		{name: "retry_incomplete_start", ready: false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			manifest := validV2Manifest()
			fake := newFakeDocker(t, manifest)
			manager := fake.manager(t, managedSpeechLabConfig(fake.server.URL), "")
			if err := manager.Install(context.Background()); err != nil {
				t.Fatal(err)
			}
			module := &fakeContainer{ID: "previous-asr", Name: "s2s-parakeet-cpu", Image: manifest.Images.ASR, Running: true, Attached: true, Labels: map[string]string{
				"aurago.managed": OwnerLabel, "aurago.component": "speech-lab", "aurago.role": "module", "aurago.bundle": manifest.BundleVersion,
				"s2s.lab.managed": "true", "stage": "asr", "backend-id": "parakeet", "variant-id": "parakeet-cpu", "s2s.image": manifest.Images.ASR,
			}}
			fake.addContainer(module)
			if err := manager.Stop(context.Background()); err != nil {
				t.Fatal(err)
			}
			if err := manager.Start(context.Background()); err != nil {
				t.Fatal(err)
			}
			// Browser Lab has since switched back to the built-in ASR. The
			// deployer's last Stop snapshot still mentions the old module.
			module.Running = false
			if tc.pruned {
				delete(fake.containers, fake.find("aurago-speech-lab-control-init").ID)
			}
			fake.ready = tc.ready
			manager.fail(&Error{Code: "speech_lab_start_failed", Err: errors.New("stale deployment status")})
			err := manager.AutoStart(context.Background())
			if tc.ready && err != nil || !tc.ready && Code(err) != "speech_lab_not_ready" {
				t.Fatalf("repair readiness result: %v", err)
			}
			if module.Running == tc.ready {
				t.Fatal("healthy repair must retain inactive modules; an incomplete start must still resume its modules")
			}
			if state := manager.Status(); tc.ready && (state.State != "ready" || state.LastErrorCode != "") {
				t.Fatalf("status was not repaired: %+v", state)
			}
		})
	}
}
