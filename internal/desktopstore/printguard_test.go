package desktopstore

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"
)

func installPrintGuardFixture(t *testing.T) (*Service, *fakeDockerAdapter, *fakeSecretStore) {
	t.Helper()
	docker := &fakeDockerAdapter{}
	vault := &fakeSecretStore{data: map[string]string{printGuardVaultKey: ""}}
	s := newTestServiceWithSecrets(t, docker, &fakeDesktopAdapter{}, &fakeLaunchpadAdapter{}, fixedPorts(18000), vault)
	op, err := s.StartInstall(context.Background(), InstallRequest{AppID: PrintGuardAppID, BindMode: BindModeLocal, AllowedOrigins: []string{"https://aura.example.org", "https://store-printguard-web.apps.example.net"}})
	if err != nil {
		t.Fatal(err)
	}
	if err := s.RunOperation(context.Background(), op.ID); err != nil {
		t.Fatal(err)
	}
	return s, docker, vault
}

func TestPrintGuardProfilesAndFailedReplacement(t *testing.T) {
	s, docker, _ := installPrintGuardFixture(t)
	ctx := context.Background()
	app, _, _ := s.GetInstalled(ctx, PrintGuardAppID)
	dataVolume := app.Volumes[0]
	if app.Image != printGuardImages["cpu"] || appReadinessTimeoutFor(app) != 5*time.Minute || app.HostIP != "127.0.0.1" || len(app.Ports) != 1 || dataVolume.ContainerPath != "/data" {
		t.Fatal("incorrect CPU installation boundary")
	}
	for _, profile := range []string{"intel", "nvidia", "cpu"} {
		op, err := s.ConfigurePrintGuard(ctx, PrintGuardConfigUpdate{Acceleration: &profile, Origins: []string{"https://aura.example.org", "https://store-printguard-web.apps.example.net"}})
		if err != nil {
			t.Fatal(err)
		}
		if err := s.RunOperation(ctx, op.ID); err != nil {
			t.Fatal(err)
		}
		app, _, _ = s.GetInstalled(ctx, PrintGuardAppID)
		spec, err := s.runtimeContainerSpec(app)
		if err != nil || app.Image != printGuardImages[profile] || app.Volumes[0] != dataVolume {
			t.Fatalf("profile %s did not persist: %v", profile, err)
		}
		if (len(spec.Devices) == 1) != (profile == "intel") || (len(spec.DeviceRequests) == 1) != (profile == "nvidia") {
			t.Fatalf("wrong GPU grants for %s", profile)
		}
		payload, _ := json.Marshal(dockerCreatePayload(spec))
		if profile == "intel" && !strings.Contains(string(payload), `"PathOnHost":"/dev/dri"`) {
			t.Fatal("Intel device not sent to Engine")
		}
		if profile == "nvidia" && !strings.Contains(string(payload), `"Driver":"nvidia"`) {
			t.Fatal("NVIDIA request not sent to Engine")
		}
		if !strings.Contains(strings.Join(spec.Env, "\n"), "PRINTGUARD_ORIGINS=https://aura.example.org,") {
			t.Fatal("exact origins missing")
		}
	}
	previous := app
	profile := "intel"
	op, err := s.ConfigurePrintGuard(ctx, PrintGuardConfigUpdate{Acceleration: &profile, Origins: []string{"https://aura.example.org", "https://store-printguard-web.apps.example.net"}})
	if err != nil {
		t.Fatal(err)
	}
	docker.startErrors = []error{errors.New("fixture GPU startup failed"), nil}
	if err := s.RunOperation(ctx, op.ID); err == nil {
		t.Fatal("GPU startup failure accepted")
	}
	app, _, _ = s.GetInstalled(ctx, PrintGuardAppID)
	if app.Image != previous.Image || app.Status != AppStatusRunning || app.ContainerID != previous.ContainerID {
		t.Fatal("failed GPU switch did not restore old runtime")
	}
	spec, err := s.runtimeContainerSpec(app)
	if err != nil || len(spec.Devices) != 0 || len(spec.DeviceRequests) != 0 {
		t.Fatal("rollback acquired desired GPU grants")
	}
	status, _ := s.PrintGuardConfiguration(ctx)
	if !status.Pending || status.ActiveAcceleration != "cpu" || status.Acceleration != "intel" {
		t.Fatal("pending and active profiles conflated")
	}
}

func TestPrintGuardReadinessDoesNotAcceptAnUnhealthyEngine(t *testing.T) {
	s, docker, _ := installPrintGuardFixture(t)
	app, _, _ := s.GetInstalled(context.Background(), PrintGuardAppID)
	docker.inspectState = ContainerState{Running: true, Status: "running", Health: "starting"}
	if err := s.waitContainerReady(context.Background(), app, time.Nanosecond); err == nil {
		t.Fatal("listening port accepted before engine health")
	}
	docker.inspectState.Health = "healthy"
	if err := s.waitContainerReady(context.Background(), app, time.Second); err != nil {
		t.Fatal(err)
	}
}

func TestPrintGuardReadinessUsesTheSelectedSharedNetwork(t *testing.T) {
	s, _, _ := installPrintGuardFixture(t)
	network := "aura-net"
	var host string
	var port int
	s.portProbe = func(_ context.Context, probeHost string, probePort int) bool {
		host, port = probeHost, probePort
		return true
	}
	op, err := s.ConfigurePrintGuard(context.Background(), PrintGuardConfigUpdate{DockerNetwork: &network})
	if err != nil {
		t.Fatal(err)
	}
	if err := s.RunOperation(context.Background(), op.ID); err != nil {
		t.Fatal(err)
	}
	if host != ContainerName(PrintGuardAppID) || port != 8000 {
		t.Fatalf("containerized readiness used host loopback: %s:%d", host, port)
	}
}

func TestPrintGuardCatalogPinUpdateKeepsPreviousRuntime(t *testing.T) {
	s, docker, _ := installPrintGuardFixture(t)
	ctx := context.Background()
	previous, _, _ := s.GetInstalled(ctx, PrintGuardAppID)
	oldPin := printGuardImages["cpu"]
	t.Cleanup(func() { printGuardImages["cpu"] = oldPin })
	printGuardImages["cpu"] = "ghcr.io/oliverbravery/printguard:fixture@sha256:" + strings.Repeat("a", 64)
	op, err := s.StartAppOperation(ctx, PrintGuardAppID, OperationUpdate, OperationRequest{})
	if err != nil {
		t.Fatal(err)
	}
	docker.startErrors = []error{errors.New("fixture new release failed health"), nil}
	if err := s.RunOperation(ctx, op.ID); err == nil {
		t.Fatal("failed release update succeeded")
	}
	app, _, _ := s.GetInstalled(ctx, PrintGuardAppID)
	spec, err := s.runtimeContainerSpec(app)
	if err != nil || spec.Image != oldPin || app.ContainerID != previous.ContainerID {
		t.Fatal("new pin invalidated previous runtime")
	}
	status, err := s.PrintGuardConfiguration(ctx)
	if err != nil || !status.Pending {
		t.Fatal("failed release pin was not left pending")
	}
	op, err = s.StartAppOperation(ctx, PrintGuardAppID, OperationUpdate, OperationRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if err := s.RunOperation(ctx, op.ID); err != nil {
		t.Fatal(err)
	}
	app, _, _ = s.GetInstalled(ctx, PrintGuardAppID)
	if app.Image != printGuardImages["cpu"] {
		t.Fatal("retry did not apply the new pin")
	}
}

func TestPrintGuardAgentCredentialsAndRevocation(t *testing.T) {
	s, _, vault := installPrintGuardFixture(t)
	ctx := context.Background()
	mode := "control"
	const token = "synthetic-printguard-token-740295"
	op, err := s.ConfigurePrintGuard(ctx, PrintGuardConfigUpdate{AgentMode: &mode, APIToken: token, Origins: []string{"https://aura.example.org", "https://store-printguard-web.apps.example.net"}})
	if err != nil {
		t.Fatal(err)
	}
	if err := s.RunOperation(ctx, op.ID); err != nil {
		t.Fatal(err)
	}
	binding, err := ReadPrintGuardAgentBinding(ctx, s.cfg.DBPath, vault)
	if err != nil || binding.Verified || binding.Token != token {
		t.Fatalf("unverified token binding missing: %v", err)
	}
	if err := s.VerifyPrintGuardAgent(ctx, binding); err != nil {
		t.Fatal(err)
	}
	binding, err = ReadPrintGuardAgentBinding(ctx, s.cfg.DBPath, vault)
	if err != nil || !binding.Verified {
		t.Fatalf("verification not retained: %v", err)
	}
	app, _, _ := s.GetInstalled(ctx, PrintGuardAppID)
	spec, _ := s.runtimeContainerSpec(app)
	status, _ := s.PrintGuardConfiguration(ctx)
	public, _ := json.Marshal(status)
	ledger, _ := s.Operation(ctx, op.ID)
	var stored string
	if err := s.db.QueryRow(`SELECT env_json FROM desktop_store_apps WHERE app_id = ?`, PrintGuardAppID).Scan(&stored); err != nil {
		t.Fatal(err)
	}
	for _, text := range []string{string(public), stored, ledger.RequestJSON, strings.Join(spec.Env, "\n")} {
		if strings.Contains(text, token) {
			t.Fatal("token escaped vault")
		}
	}
	mode = "read"
	op, err = s.ConfigurePrintGuard(ctx, PrintGuardConfigUpdate{AgentMode: &mode, Origins: []string{"https://aura.example.org", "https://store-printguard-web.apps.example.net"}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ReadPrintGuardAgentBinding(ctx, s.cfg.DBPath, vault); err == nil {
		t.Fatal("pending permission change kept old connection authorized")
	}
	if err := s.RunOperation(ctx, op.ID); err != nil {
		t.Fatal(err)
	}
	if err := s.VerifyPrintGuardAgent(ctx, binding); err == nil {
		t.Fatal("late verification restored old control grant")
	}
	next, err := ReadPrintGuardAgentBinding(ctx, s.cfg.DBPath, vault)
	if err != nil || next.Token != token || next.Mode != "read" || next.Verified {
		t.Fatalf("blank token did not preserve narrowed access: %v", err)
	}
	for _, deleteData := range []bool{false, true} {
		op, err = s.StartAppOperation(ctx, PrintGuardAppID, OperationUninstall, OperationRequest{DeleteData: deleteData})
		if err != nil {
			t.Fatal(err)
		}
		if err := s.RunOperation(ctx, op.ID); err != nil {
			t.Fatal(err)
		}
		_, kept := vault.data[printGuardVaultKey]
		if kept == deleteData {
			t.Fatal("delete_data did not control Vault retention")
		}
		if !deleteData {
			op, err = s.StartInstall(ctx, InstallRequest{AppID: PrintGuardAppID, BindMode: BindModeLocal, AllowedOrigins: []string{"https://aura.example.org"}})
			if err != nil {
				t.Fatal(err)
			}
			if err := s.RunOperation(ctx, op.ID); err != nil {
				t.Fatal(err)
			}
		}
	}
}

func TestPrintGuardRejectsPublicOrArbitraryRuntime(t *testing.T) {
	s, _, _ := installPrintGuardFixture(t)
	for _, req := range []InstallRequest{
		{AppID: PrintGuardAppID, BindMode: BindModeLAN, AllowedOrigins: []string{"https://aura.example.org"}},
		{AppID: PrintGuardAppID, BindMode: BindModeLocal, TailscaleEnabled: true, AllowedOrigins: []string{"https://aura.example.org"}},
		{AppID: PrintGuardAppID, BindMode: BindModeLocal, DockerNetwork: "host", AllowedOrigins: []string{"https://aura.example.org"}},
		{AppID: PrintGuardAppID, BindMode: BindModeLocal},
	} {
		if _, err := s.StartInstall(context.Background(), req); err == nil {
			t.Fatal("unsafe installation accepted")
		}
	}
	profile := "arbitrary-device"
	if _, err := s.ConfigurePrintGuard(context.Background(), PrintGuardConfigUpdate{Acceleration: &profile}); err == nil {
		t.Fatal("arbitrary GPU profile accepted")
	}
}
