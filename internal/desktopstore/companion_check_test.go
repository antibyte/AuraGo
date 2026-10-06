package desktopstore

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

func TestInstallFailsWhenACompanionExitedWithAnError(t *testing.T) {
	ctx := context.Background()
	docker := &fakeDockerAdapter{inspectStates: map[string]ContainerState{
		"aurago-store-termix-guacd": {Status: "exited", ExitCode: 1},
	}}
	svc := newTestService(t, docker, &fakeDesktopAdapter{}, &fakeLaunchpadAdapter{}, fixedPorts(19811))
	op, err := svc.StartInstall(ctx, InstallRequest{AppID: "termix", BindMode: BindModeLocal})
	if err != nil {
		t.Fatalf("start install: %v", err)
	}
	err = svc.RunOperation(ctx, op.ID)
	if err == nil || !strings.Contains(err.Error(), "aurago-store-termix-guacd exited with exit code 1") {
		t.Fatalf("install error = %v, want the exited companion", err)
	}
	if _, ok, _ := svc.GetInstalled(ctx, "termix"); ok {
		t.Fatal("failed install kept its record")
	}
	if docker.removedContainers["aurago-store-termix-guacd"] == 0 || docker.removedContainers["aurago-store-termix"] == 0 {
		t.Fatalf("failed install left containers behind: %#v", docker.removedContainers)
	}
}

func TestCompanionStatesThatDoNotFailAnInstall(t *testing.T) {
	cases := []struct {
		name  string
		state ContainerState
		err   error
	}{
		{name: "running", state: ContainerState{Running: true, Status: "running"}},
		{name: "crash loop under unless-stopped", state: ContainerState{Running: true, Restarting: true, Status: "restarting", ExitCode: 1, RestartCount: 3}},
		{name: "health check still starting", state: ContainerState{Running: true, Status: "running", Health: "starting"}},
		{name: "unhealthy", state: ContainerState{Running: true, Status: "running", Health: "unhealthy"}},
		{name: "finished one-shot", state: ContainerState{Status: "exited", ExitCode: 0}},
		{name: "inspect error", err: errors.New("engine busy")},
	}
	for i, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			docker := &fakeDockerAdapter{}
			if tc.err != nil {
				docker.inspectErrors = map[string]error{"aurago-store-termix-guacd": tc.err}
			} else {
				docker.inspectStates = map[string]ContainerState{"aurago-store-termix-guacd": tc.state}
			}
			svc := newTestService(t, docker, &fakeDesktopAdapter{}, &fakeLaunchpadAdapter{}, fixedPorts(19820+i))
			op, err := svc.StartInstall(ctx, InstallRequest{AppID: "termix", BindMode: BindModeLocal})
			if err != nil {
				t.Fatalf("start install: %v", err)
			}
			if err := svc.RunOperation(ctx, op.ID); err != nil {
				t.Fatalf("install failed for companion state %q: %v", tc.name, err)
			}
			if stored, _, _ := svc.GetInstalled(ctx, "termix"); stored.Status != AppStatusRunning {
				t.Fatalf("status = %q, want running", stored.Status)
			}
		})
	}
}

func TestUpdateRollsBackWhenAReplacedCompanionExitedWithAnError(t *testing.T) {
	ctx := context.Background()
	docker := &fakeDockerAdapter{trackContainers: true, traceLifecycle: true}
	secrets := &fakeSecretStore{data: map[string]string{}}
	svc := newTestServiceWithSecrets(t, docker, &fakeDesktopAdapter{}, &fakeLaunchpadAdapter{}, fixedPorts(17680), secrets)
	runStoreInstall(t, svc, "romm")
	docker.events = nil
	docker.inspectStates = map[string]ContainerState{"aurago-store-romm-db": {Status: "exited", ExitCode: 1}}

	err := runStoreOperation(t, svc, "romm", OperationUpdate)
	if err == nil || !strings.Contains(err.Error(), "aurago-store-romm-db exited with exit code 1") {
		t.Fatalf("update error = %v, want the exited companion", err)
	}
	assertEventOrder(t, docker.events,
		"create:aurago-store-romm",
		"remove:aurago-store-romm",
		"remove:aurago-store-romm-db",
		"rename:aurago-store-romm-db.prev->aurago-store-romm-db",
		"rename:aurago-store-romm.prev->aurago-store-romm",
	)
	stored, _, err := svc.GetInstalled(ctx, "romm")
	if err != nil || stored.Status != AppStatusRunning || stored.LastOperationState != OperationFailed {
		t.Fatalf("stored state = %q/%q (%v), want running/failed", stored.Status, stored.LastOperationState, err)
	}
}

func TestUpdateDoesNotCheckCompanionsItDidNotReplace(t *testing.T) {
	ctx := context.Background()
	docker := &fakeDockerAdapter{}
	secrets := &fakeSecretStore{data: map[string]string{}}
	svc := newTestServiceWithSecrets(t, docker, &fakeDesktopAdapter{}, &fakeLaunchpadAdapter{}, fixedPorts(18095, 23755, 23756), secrets)
	runStoreInstall(t, svc, "beszel")
	if _, err := svc.ConfigureBeszelAgent(ctx, "ssh-ed25519 public-key", "agent-token"); err != nil {
		t.Fatalf("configure Beszel agent: %v", err)
	}
	// Without its vault secrets the update skips the agent, so it is not replaced.
	delete(secrets.data, "desktop_store_beszel_agent_key")
	delete(secrets.data, "desktop_store_beszel_agent_token")
	// A manually stopped agent: docker stop leaves exit code 143.
	docker.inspectStates = map[string]ContainerState{"aurago-store-beszel-agent": {Status: "exited", ExitCode: 143}}

	if err := runStoreOperation(t, svc, "beszel", OperationUpdate); err != nil {
		t.Fatalf("update failed because of a companion it did not replace: %v", err)
	}
}

// The settle time is waited for once, measured from the last companion start,
// and a cancelled operation stops waiting.
func TestCompanionCheckWaitsForTheSettleTimeAndHonoursCancellation(t *testing.T) {
	docker := &fakeDockerAdapter{}
	svc := newTestService(t, docker, &fakeDesktopAdapter{}, &fakeLaunchpadAdapter{}, fixedPorts(19830))
	svc.cfg.CompanionSettle = time.Hour
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err := svc.checkStartedCompanions(ctx, InstalledApp{AppID: "termix"}, []string{"aurago-store-termix-guacd"}, time.Now())
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("check with a cancelled context = %v, want context.Canceled", err)
	}
	if docker.inspectCalls != 0 {
		t.Fatalf("inspected %d times before the settle time", docker.inspectCalls)
	}
	svc.cfg.CompanionSettle = 10 * time.Millisecond
	if err := svc.checkStartedCompanions(context.Background(), InstalledApp{AppID: "termix"}, []string{"aurago-store-termix-guacd"}, time.Now().Add(-time.Second)); err != nil {
		t.Fatalf("check after the settle time: %v", err)
	}
	if docker.inspectCalls != 1 {
		t.Fatalf("inspect calls = %d, want 1", docker.inspectCalls)
	}
}
