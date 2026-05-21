package desktopstore

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"aurago/internal/desktop"
)

func TestDefaultCatalogContainsInitialApps(t *testing.T) {
	catalog := DefaultCatalog()
	if len(catalog) != 7 {
		t.Fatalf("expected 7 catalog apps, got %d", len(catalog))
	}

	expected := map[string]struct {
		image string
		port  int
		icon  string
	}{
		"homarr":       {image: "ghcr.io/homarr-labs/homarr:latest", port: 7575, icon: "home"},
		"n8n":          {image: "docker.n8n.io/n8nio/n8n", port: 5678, icon: "workflow"},
		"node-red":     {image: "nodered/node-red", port: 1880, icon: "workflow"},
		"open-webui":   {image: "ghcr.io/open-webui/open-webui:main", port: 8080, icon: "chat"},
		"adguard-home": {image: "adguard/adguardhome", port: 3000, icon: "network"},
		"excalidraw":   {image: "excalidraw/excalidraw:latest", port: 80, icon: "editor"},
		"uptime-kuma":  {image: "louislam/uptime-kuma:2", port: 3001, icon: "monitor"},
	}

	for _, entry := range catalog {
		want, ok := expected[entry.ID]
		if !ok {
			t.Fatalf("unexpected catalog entry %q", entry.ID)
		}
		if entry.Image != want.image {
			t.Fatalf("%s image = %q, want %q", entry.ID, entry.Image, want.image)
		}
		if entry.PrimaryPort.ContainerPort != want.port {
			t.Fatalf("%s port = %d, want %d", entry.ID, entry.PrimaryPort.ContainerPort, want.port)
		}
		if entry.Icon != want.icon {
			t.Fatalf("%s icon = %q, want %q", entry.ID, entry.Icon, want.icon)
		}
		if entry.Name == "" || entry.Description == "" || entry.LogoSlug == "" {
			t.Fatalf("%s must have name, description and logo slug", entry.ID)
		}
		delete(expected, entry.ID)
	}
	if len(expected) != 0 {
		t.Fatalf("missing catalog entries: %#v", expected)
	}
}

func TestInstallOperationCreatesContainerDesktopShortcutAndLaunchpadLink(t *testing.T) {
	ctx := context.Background()
	docker := &fakeDockerAdapter{}
	desktopAdapter := &fakeDesktopAdapter{}
	launchpad := &fakeLaunchpadAdapter{}
	svc := newTestService(t, docker, desktopAdapter, launchpad, fixedPorts(19180))

	op, err := svc.StartInstall(ctx, InstallRequest{
		AppID:            "n8n",
		BindMode:         BindModeLocal,
		TailscaleEnabled: true,
	})
	if err != nil {
		t.Fatalf("start install: %v", err)
	}
	if op.Status != OperationPending {
		t.Fatalf("new operation status = %s, want %s", op.Status, OperationPending)
	}
	if err := svc.RunOperation(ctx, op.ID); err != nil {
		t.Fatalf("run install: %v", err)
	}

	stored, ok, err := svc.GetInstalled(ctx, "n8n")
	if err != nil {
		t.Fatalf("get installed: %v", err)
	}
	if !ok {
		t.Fatal("expected n8n install record")
	}
	if stored.Status != AppStatusRunning {
		t.Fatalf("status = %s, want %s", stored.Status, AppStatusRunning)
	}
	if stored.HostIP != "127.0.0.1" {
		t.Fatalf("host ip = %q, want 127.0.0.1", stored.HostIP)
	}
	if stored.HostPort != 19180 {
		t.Fatalf("host port = %d, want 19180", stored.HostPort)
	}
	if !stored.TailscaleEnabled || stored.TailscaleStatus != TailscaleStatusPending {
		t.Fatalf("tailscale state = %v/%s, want pending enabled", stored.TailscaleEnabled, stored.TailscaleStatus)
	}

	if len(docker.created) != 1 {
		t.Fatalf("created containers = %d, want 1", len(docker.created))
	}
	spec := docker.created[0]
	if spec.Name != "aurago-store-n8n" {
		t.Fatalf("container name = %q", spec.Name)
	}
	if spec.PortBindings[0].HostIP != "127.0.0.1" {
		t.Fatalf("binding host ip = %q", spec.PortBindings[0].HostIP)
	}
	if spec.PortBindings[0].HostPort != 19180 {
		t.Fatalf("binding host port = %d", spec.PortBindings[0].HostPort)
	}
	if len(spec.Volumes) != 1 || spec.Volumes[0].Name == "" || spec.Volumes[0].ContainerPath != "/home/node/.n8n" {
		t.Fatalf("unexpected volumes: %#v", spec.Volumes)
	}

	if desktopAdapter.installed.ID != "store-n8n" {
		t.Fatalf("desktop app id = %q, want store-n8n", desktopAdapter.installed.ID)
	}
	if desktopAdapter.installed.Runtime != RuntimeContainerWebApp {
		t.Fatalf("desktop runtime = %q", desktopAdapter.installed.Runtime)
	}
	if desktopAdapter.installed.Metadata["store_app_id"] != "n8n" {
		t.Fatalf("metadata store_app_id missing: %#v", desktopAdapter.installed.Metadata)
	}
	if desktopAdapter.shortcutAppID != "store-n8n" {
		t.Fatalf("shortcut app id = %q", desktopAdapter.shortcutAppID)
	}
	if desktopAdapter.dockVisible == nil || *desktopAdapter.dockVisible {
		t.Fatalf("dock visibility should be false")
	}
	if desktopAdapter.startVisible == nil || !*desktopAdapter.startVisible {
		t.Fatalf("start visibility should be true")
	}
	if launchpad.upserted.URL != "aurago-store://n8n" {
		t.Fatalf("launchpad URL = %q", launchpad.upserted.URL)
	}
}

func TestInstalledAppJSONDoesNotExposeRuntimeEnv(t *testing.T) {
	ctx := context.Background()
	svc := newTestService(t, &fakeDockerAdapter{}, &fakeDesktopAdapter{}, &fakeLaunchpadAdapter{}, fixedPorts(19181))

	op, err := svc.StartInstall(ctx, InstallRequest{AppID: "homarr", BindMode: BindModeLocal})
	if err != nil {
		t.Fatalf("start install: %v", err)
	}
	if err := svc.RunOperation(ctx, op.ID); err != nil {
		t.Fatalf("run install: %v", err)
	}
	stored, ok, err := svc.GetInstalled(ctx, "homarr")
	if err != nil || !ok {
		t.Fatalf("get installed: ok=%v err=%v", ok, err)
	}
	if len(stored.Env) == 0 {
		t.Fatal("expected internal env to be retained for lifecycle operations")
	}
	data, err := json.Marshal(stored)
	if err != nil {
		t.Fatalf("marshal installed app: %v", err)
	}
	if strings.Contains(string(data), "SECRET_ENCRYPTION_KEY") || strings.Contains(string(data), `"env"`) {
		t.Fatalf("installed app JSON leaked env data: %s", data)
	}
}

func TestInstallFailureCleansContainerVolumesAndRecord(t *testing.T) {
	ctx := context.Background()
	docker := &fakeDockerAdapter{startErrors: []error{errors.New("start failed")}}
	desktopAdapter := &fakeDesktopAdapter{}
	launchpad := &fakeLaunchpadAdapter{}
	svc := newTestService(t, docker, desktopAdapter, launchpad, fixedPorts(19182))

	op, err := svc.StartInstall(ctx, InstallRequest{AppID: "n8n", BindMode: BindModeLocal})
	if err != nil {
		t.Fatalf("start install: %v", err)
	}
	if err := svc.RunOperation(ctx, op.ID); err == nil {
		t.Fatal("expected install failure")
	}
	if _, ok, err := svc.GetInstalled(ctx, "n8n"); err != nil || ok {
		t.Fatalf("install record should be removed after failed install: ok=%v err=%v", ok, err)
	}
	if docker.removedContainers["aurago-store-n8n"] == 0 {
		t.Fatalf("failed install did not remove container: %#v", docker.removedContainers)
	}
	if len(docker.removedVolumes) == 0 {
		t.Fatal("failed install did not remove created named volumes")
	}
}

func TestInstallDesktopFailureCleansRunningContainer(t *testing.T) {
	ctx := context.Background()
	docker := &fakeDockerAdapter{}
	desktopAdapter := &fakeDesktopAdapter{installErr: errors.New("desktop install failed")}
	svc := newTestService(t, docker, desktopAdapter, &fakeLaunchpadAdapter{}, fixedPorts(19183))

	op, err := svc.StartInstall(ctx, InstallRequest{AppID: "homarr", BindMode: BindModeLocal})
	if err != nil {
		t.Fatalf("start install: %v", err)
	}
	if err := svc.RunOperation(ctx, op.ID); err == nil {
		t.Fatal("expected desktop install failure")
	}
	if _, ok, err := svc.GetInstalled(ctx, "homarr"); err != nil || ok {
		t.Fatalf("install record should be removed after desktop failure: ok=%v err=%v", ok, err)
	}
	if docker.removedContainers["aurago-store-homarr"] == 0 {
		t.Fatalf("desktop failure did not remove running container: %#v", docker.removedContainers)
	}
}

func TestStartInstallRejectsConcurrentOperationForSameApp(t *testing.T) {
	ctx := context.Background()
	svc := newTestService(t, &fakeDockerAdapter{}, &fakeDesktopAdapter{}, &fakeLaunchpadAdapter{}, fixedPorts(19184))

	first, err := svc.StartInstall(ctx, InstallRequest{AppID: "n8n", BindMode: BindModeLocal})
	if err != nil {
		t.Fatalf("start first install: %v", err)
	}
	if first.Status != OperationPending {
		t.Fatalf("first operation status = %s, want pending", first.Status)
	}
	if _, err := svc.StartInstall(ctx, InstallRequest{AppID: "n8n", BindMode: BindModeLocal}); !errors.Is(err, ErrOperationInProgress) {
		t.Fatalf("second install error = %v, want ErrOperationInProgress", err)
	}
	if _, err := svc.StartInstall(ctx, InstallRequest{AppID: "node-red", BindMode: BindModeLocal}); err != nil {
		t.Fatalf("different app should not be blocked: %v", err)
	}
}

func TestStartInstallResetsStaleOperationBeforeCreatingNewOne(t *testing.T) {
	ctx := context.Background()
	svc := newTestService(t, &fakeDockerAdapter{}, &fakeDesktopAdapter{}, &fakeLaunchpadAdapter{}, fixedPorts(19185))

	stale, err := svc.StartInstall(ctx, InstallRequest{AppID: "homarr", BindMode: BindModeLocal})
	if err != nil {
		t.Fatalf("start stale install: %v", err)
	}
	old := time.Now().UTC().Add(-(operationStaleAfter + time.Minute))
	_, err = svc.db.ExecContext(ctx, `UPDATE desktop_store_operations SET status = ?, updated_at = ? WHERE id = ?`,
		OperationRunning, formatTime(old), stale.ID)
	if err != nil {
		t.Fatalf("age operation: %v", err)
	}
	next, err := svc.StartInstall(ctx, InstallRequest{AppID: "homarr", BindMode: BindModeLocal})
	if err != nil {
		t.Fatalf("start after stale operation: %v", err)
	}
	if next.ID == stale.ID {
		t.Fatal("expected a new operation id")
	}
	oldOp, err := svc.Operation(ctx, stale.ID)
	if err != nil {
		t.Fatalf("get stale operation: %v", err)
	}
	if oldOp.Status != OperationFailed || oldOp.CompletedAt == nil {
		t.Fatalf("stale operation not marked failed/completed: %#v", oldOp)
	}
}

func TestInstallLanModeBindsAllInterfaces(t *testing.T) {
	ctx := context.Background()
	docker := &fakeDockerAdapter{}
	svc := newTestService(t, docker, &fakeDesktopAdapter{}, &fakeLaunchpadAdapter{}, fixedPorts(19222))

	op, err := svc.StartInstall(ctx, InstallRequest{AppID: "node-red", BindMode: BindModeLAN})
	if err != nil {
		t.Fatalf("start install: %v", err)
	}
	if err := svc.RunOperation(ctx, op.ID); err != nil {
		t.Fatalf("run install: %v", err)
	}

	if got := docker.created[0].PortBindings[0].HostIP; got != "0.0.0.0" {
		t.Fatalf("host ip = %q, want 0.0.0.0", got)
	}
}

func TestOpenURLChoosesLocalLanOrTailnetSurface(t *testing.T) {
	ctx := context.Background()
	svc := newTestService(t, &fakeDockerAdapter{}, &fakeDesktopAdapter{}, &fakeLaunchpadAdapter{}, fixedPorts(19666))

	op, err := svc.StartInstall(ctx, InstallRequest{AppID: "node-red", BindMode: BindModeLAN, TailscaleEnabled: true})
	if err != nil {
		t.Fatalf("start install: %v", err)
	}
	if err := svc.RunOperation(ctx, op.ID); err != nil {
		t.Fatalf("run install: %v", err)
	}
	localURL, _, err := svc.OpenURL(ctx, "node-red", "127.0.0.1:8080", false, "")
	if err != nil {
		t.Fatalf("open local url: %v", err)
	}
	if localURL != "http://127.0.0.1:19666/" {
		t.Fatalf("local url = %q", localURL)
	}
	lanURL, _, err := svc.OpenURL(ctx, "node-red", "192.168.1.50:8080", false, "")
	if err != nil {
		t.Fatalf("open lan url: %v", err)
	}
	if lanURL != "http://192.168.1.50:19666/" {
		t.Fatalf("lan url = %q", lanURL)
	}
	if err := svc.SetTailscaleStatus(ctx, "node-red", TailscaleStatusActive); err != nil {
		t.Fatalf("activate tailnet: %v", err)
	}
	tailnetURL, _, err := svc.OpenURL(ctx, "node-red", "aurago.example.ts.net", true, "aurago.example.ts.net")
	if err != nil {
		t.Fatalf("open tailnet url: %v", err)
	}
	if tailnetURL != "https://aurago.example.ts.net:19666/" {
		t.Fatalf("tailnet url = %q", tailnetURL)
	}
}

func TestOpenURLRejectsAppThatIsNotRunning(t *testing.T) {
	ctx := context.Background()
	svc := newTestService(t, &fakeDockerAdapter{}, &fakeDesktopAdapter{}, &fakeLaunchpadAdapter{}, fixedPorts(19667))

	op, err := svc.StartInstall(ctx, InstallRequest{AppID: "node-red", BindMode: BindModeLocal})
	if err != nil {
		t.Fatalf("start install: %v", err)
	}
	if err := svc.RunOperation(ctx, op.ID); err != nil {
		t.Fatalf("run install: %v", err)
	}
	stopOp, err := svc.StartAppOperation(ctx, "node-red", OperationStop, OperationRequest{})
	if err != nil {
		t.Fatalf("start stop: %v", err)
	}
	if err := svc.RunOperation(ctx, stopOp.ID); err != nil {
		t.Fatalf("run stop: %v", err)
	}

	if _, _, err := svc.OpenURL(ctx, "node-red", "127.0.0.1:8080", false, ""); err == nil || !strings.Contains(err.Error(), "is not running") {
		t.Fatalf("OpenURL stopped app error = %v, want not running", err)
	}
}

func TestUpdateOperationRecreatesContainerAndKeepsVolumesPorts(t *testing.T) {
	ctx := context.Background()
	docker := &fakeDockerAdapter{}
	svc := newTestService(t, docker, &fakeDesktopAdapter{}, &fakeLaunchpadAdapter{}, fixedPorts(19333))

	installOp, err := svc.StartInstall(ctx, InstallRequest{AppID: "uptime-kuma", BindMode: BindModeLocal})
	if err != nil {
		t.Fatalf("start install: %v", err)
	}
	if err := svc.RunOperation(ctx, installOp.ID); err != nil {
		t.Fatalf("run install: %v", err)
	}
	updateOp, err := svc.StartAppOperation(ctx, "uptime-kuma", OperationUpdate, OperationRequest{})
	if err != nil {
		t.Fatalf("start update: %v", err)
	}
	if err := svc.RunOperation(ctx, updateOp.ID); err != nil {
		t.Fatalf("run update: %v", err)
	}

	if len(docker.pulled) != 2 {
		t.Fatalf("pull count = %d, want 2", len(docker.pulled))
	}
	if docker.removedContainers["aurago-store-uptime-kuma"] != 1 {
		t.Fatalf("expected old container to be removed once, got %#v", docker.removedContainers)
	}
	if len(docker.created) != 2 {
		t.Fatalf("created containers = %d, want 2", len(docker.created))
	}
	before, after := docker.created[0], docker.created[1]
	if before.PortBindings[0] != after.PortBindings[0] {
		t.Fatalf("port binding changed on update: before %#v after %#v", before.PortBindings[0], after.PortBindings[0])
	}
	if before.Volumes[0] != after.Volumes[0] {
		t.Fatalf("volume changed on update: before %#v after %#v", before.Volumes[0], after.Volumes[0])
	}
}

func TestUpdateOperationUsesCurrentCatalogImage(t *testing.T) {
	ctx := context.Background()
	docker := &fakeDockerAdapter{}
	catalog := []CatalogEntry{
		{
			ID:          "demo-app",
			Name:        "Demo App",
			Description: "Demo app for update tests.",
			Image:       "example/demo:new",
			Icon:        "package",
			LogoSlug:    "demo",
			LogoURL:     "https://example.invalid/demo.png",
			PrimaryPort: PortSpec{ContainerPort: 8080, Protocol: "tcp"},
			Volumes:     []VolumeTemplate{{NameSuffix: "data", ContainerPath: "/data"}},
		},
	}
	svc := newTestServiceWithCatalog(t, docker, &fakeDesktopAdapter{}, &fakeLaunchpadAdapter{}, fixedPorts(19888), catalog)

	seedOp := Operation{ID: "op-seed", Type: OperationInstall}
	record := svc.buildInstallRecord(catalog[0], seedOp, BindModeLocal, "127.0.0.1", 19888, false)
	record.Image = "example/demo:old"
	record.Status = AppStatusRunning
	record.LastOperationState = OperationSucceeded
	if err := svc.saveInstalled(ctx, record); err != nil {
		t.Fatalf("seed installed app: %v", err)
	}

	updateOp, err := svc.StartAppOperation(ctx, "demo-app", OperationUpdate, OperationRequest{})
	if err != nil {
		t.Fatalf("start update: %v", err)
	}
	if err := svc.RunOperation(ctx, updateOp.ID); err != nil {
		t.Fatalf("run update: %v", err)
	}

	if len(docker.created) != 1 {
		t.Fatalf("created containers = %d, want 1 updated container", len(docker.created))
	}
	if got := docker.created[0].Image; got != "example/demo:new" {
		t.Fatalf("updated container image = %q, want current catalog image", got)
	}
	stored, ok, err := svc.GetInstalled(ctx, "demo-app")
	if err != nil || !ok {
		t.Fatalf("get updated app: ok=%v err=%v", ok, err)
	}
	if stored.Image != "example/demo:new" {
		t.Fatalf("stored image = %q, want current catalog image", stored.Image)
	}
}

func TestUpdateStartFailureRollsBackPreviousRunningContainer(t *testing.T) {
	ctx := context.Background()
	docker := &fakeDockerAdapter{}
	svc := newTestService(t, docker, &fakeDesktopAdapter{}, &fakeLaunchpadAdapter{}, fixedPorts(19334))

	installOp, err := svc.StartInstall(ctx, InstallRequest{AppID: "uptime-kuma", BindMode: BindModeLocal})
	if err != nil {
		t.Fatalf("start install: %v", err)
	}
	if err := svc.RunOperation(ctx, installOp.ID); err != nil {
		t.Fatalf("run install: %v", err)
	}
	docker.startErrors = []error{errors.New("updated container failed to start"), nil}
	updateOp, err := svc.StartAppOperation(ctx, "uptime-kuma", OperationUpdate, OperationRequest{})
	if err != nil {
		t.Fatalf("start update: %v", err)
	}
	if err := svc.RunOperation(ctx, updateOp.ID); err == nil {
		t.Fatal("expected update start failure")
	}
	after, ok, err := svc.GetInstalled(ctx, "uptime-kuma")
	if err != nil || !ok {
		t.Fatalf("get install after update: ok=%v err=%v", ok, err)
	}
	if after.Status != AppStatusRunning {
		t.Fatalf("status after rollback = %s, want running", after.Status)
	}
	if len(docker.created) != 3 {
		t.Fatalf("created containers = %d, want install, failed update, rollback", len(docker.created))
	}
	if docker.removedContainers["aurago-store-uptime-kuma"] < 2 {
		t.Fatalf("expected old and failed updated containers to be removed: %#v", docker.removedContainers)
	}
}

func TestUpdateStoppedAppKeepsItStopped(t *testing.T) {
	ctx := context.Background()
	docker := &fakeDockerAdapter{}
	svc := newTestService(t, docker, &fakeDesktopAdapter{}, &fakeLaunchpadAdapter{}, fixedPorts(19335))

	installOp, err := svc.StartInstall(ctx, InstallRequest{AppID: "node-red", BindMode: BindModeLocal})
	if err != nil {
		t.Fatalf("start install: %v", err)
	}
	if err := svc.RunOperation(ctx, installOp.ID); err != nil {
		t.Fatalf("run install: %v", err)
	}
	stopOp, err := svc.StartAppOperation(ctx, "node-red", OperationStop, OperationRequest{})
	if err != nil {
		t.Fatalf("start stop: %v", err)
	}
	if err := svc.RunOperation(ctx, stopOp.ID); err != nil {
		t.Fatalf("run stop: %v", err)
	}
	startCountBeforeUpdate := len(docker.started)
	updateOp, err := svc.StartAppOperation(ctx, "node-red", OperationUpdate, OperationRequest{})
	if err != nil {
		t.Fatalf("start update: %v", err)
	}
	if err := svc.RunOperation(ctx, updateOp.ID); err != nil {
		t.Fatalf("run update: %v", err)
	}
	after, ok, err := svc.GetInstalled(ctx, "node-red")
	if err != nil || !ok {
		t.Fatalf("get install after update: ok=%v err=%v", ok, err)
	}
	if after.Status != AppStatusStopped {
		t.Fatalf("status after stopped update = %s, want stopped", after.Status)
	}
	if len(docker.started) != startCountBeforeUpdate {
		t.Fatalf("stopped update started container: before=%d after=%d", startCountBeforeUpdate, len(docker.started))
	}
}

func TestUninstallRemovesVolumesOnlyWhenRequested(t *testing.T) {
	ctx := context.Background()
	docker := &fakeDockerAdapter{}
	desktopAdapter := &fakeDesktopAdapter{}
	launchpad := &fakeLaunchpadAdapter{}
	svc := newTestService(t, docker, desktopAdapter, launchpad, fixedPorts(19444))

	installOp, err := svc.StartInstall(ctx, InstallRequest{AppID: "homarr", BindMode: BindModeLocal})
	if err != nil {
		t.Fatalf("start install: %v", err)
	}
	if err := svc.RunOperation(ctx, installOp.ID); err != nil {
		t.Fatalf("run install: %v", err)
	}

	uninstallOp, err := svc.StartAppOperation(ctx, "homarr", OperationUninstall, OperationRequest{DeleteData: false})
	if err != nil {
		t.Fatalf("start uninstall: %v", err)
	}
	if err := svc.RunOperation(ctx, uninstallOp.ID); err != nil {
		t.Fatalf("run uninstall: %v", err)
	}
	if len(docker.removedVolumes) != 0 {
		t.Fatalf("volumes were deleted without delete_data=true: %#v", docker.removedVolumes)
	}

	installOp, err = svc.StartInstall(ctx, InstallRequest{AppID: "homarr", BindMode: BindModeLocal})
	if err != nil {
		t.Fatalf("start reinstall: %v", err)
	}
	if err := svc.RunOperation(ctx, installOp.ID); err != nil {
		t.Fatalf("run reinstall: %v", err)
	}
	uninstallOp, err = svc.StartAppOperation(ctx, "homarr", OperationUninstall, OperationRequest{DeleteData: true})
	if err != nil {
		t.Fatalf("start uninstall with data delete: %v", err)
	}
	if err := svc.RunOperation(ctx, uninstallOp.ID); err != nil {
		t.Fatalf("run uninstall with data delete: %v", err)
	}
	if len(docker.removedVolumes) == 0 {
		t.Fatal("expected named volume deletion with delete_data=true")
	}
	if desktopAdapter.deletedAppID != "store-homarr" {
		t.Fatalf("deleted desktop app = %q", desktopAdapter.deletedAppID)
	}
	if launchpad.deletedID != ManagedLaunchpadLinkID("homarr") {
		t.Fatalf("deleted launchpad id = %q", launchpad.deletedID)
	}
}

func TestUpdateFailureRestoresPreviousRecord(t *testing.T) {
	ctx := context.Background()
	docker := &fakeDockerAdapter{}
	svc := newTestService(t, docker, &fakeDesktopAdapter{}, &fakeLaunchpadAdapter{}, fixedPorts(19555))

	installOp, err := svc.StartInstall(ctx, InstallRequest{AppID: "excalidraw", BindMode: BindModeLocal})
	if err != nil {
		t.Fatalf("start install: %v", err)
	}
	if err := svc.RunOperation(ctx, installOp.ID); err != nil {
		t.Fatalf("run install: %v", err)
	}
	before, ok, err := svc.GetInstalled(ctx, "excalidraw")
	if err != nil || !ok {
		t.Fatalf("get install before update: ok=%v err=%v", ok, err)
	}
	docker.createErrors = []error{errors.New("create failed"), nil}

	updateOp, err := svc.StartAppOperation(ctx, "excalidraw", OperationUpdate, OperationRequest{})
	if err != nil {
		t.Fatalf("start update: %v", err)
	}
	if err := svc.RunOperation(ctx, updateOp.ID); err == nil {
		t.Fatal("expected update failure")
	}
	after, ok, err := svc.GetInstalled(ctx, "excalidraw")
	if err != nil || !ok {
		t.Fatalf("get install after update: ok=%v err=%v", ok, err)
	}
	if after.ContainerName != before.ContainerName || after.HostPort != before.HostPort || after.Status != AppStatusRunning {
		t.Fatalf("record was not restored: before %#v after %#v", before, after)
	}
}

func TestInstallWaitsForContainerReadiness(t *testing.T) {
	ctx := context.Background()
	docker := &fakeDockerAdapter{}
	svc := newTestService(t, docker, &fakeDesktopAdapter{}, &fakeLaunchpadAdapter{}, fixedPorts(19186))

	op, err := svc.StartInstall(ctx, InstallRequest{AppID: "node-red", BindMode: BindModeLocal})
	if err != nil {
		t.Fatalf("start install: %v", err)
	}
	if err := svc.RunOperation(ctx, op.ID); err != nil {
		t.Fatalf("run install: %v", err)
	}
	if docker.inspectCalls == 0 {
		t.Fatal("install did not inspect container readiness before marking running")
	}
}

func TestWaitContainerReadyRejectsUnhealthyContainer(t *testing.T) {
	ctx := context.Background()
	docker := &fakeDockerAdapter{
		inspectState: ContainerState{
			Name:    "aurago-store-node-red",
			Running: true,
			Status:  "running",
			Health:  "unhealthy",
		},
	}
	svc := newTestService(t, docker, &fakeDesktopAdapter{}, &fakeLaunchpadAdapter{}, fixedPorts(19187))
	app := InstalledApp{
		AppID:         "node-red",
		ContainerName: "aurago-store-node-red",
		Protocol:      "tcp",
		HostIP:        "127.0.0.1",
		HostPort:      19187,
	}

	err := svc.waitContainerReady(ctx, app, time.Nanosecond)
	if err == nil || !strings.Contains(err.Error(), "unhealthy") {
		t.Fatalf("waitContainerReady error = %v, want unhealthy", err)
	}
}

func TestGetInstalledRejectsCorruptRuntimeJSON(t *testing.T) {
	ctx := context.Background()
	svc := newTestService(t, &fakeDockerAdapter{}, &fakeDesktopAdapter{}, &fakeLaunchpadAdapter{}, fixedPorts(19188))

	op, err := svc.StartInstall(ctx, InstallRequest{AppID: "node-red", BindMode: BindModeLocal})
	if err != nil {
		t.Fatalf("start install: %v", err)
	}
	if err := svc.RunOperation(ctx, op.ID); err != nil {
		t.Fatalf("run install: %v", err)
	}
	if _, err := svc.db.ExecContext(ctx, `UPDATE desktop_store_apps SET volumes_json = ? WHERE app_id = ?`, `{`, "node-red"); err != nil {
		t.Fatalf("corrupt runtime json: %v", err)
	}

	if _, _, err := svc.GetInstalled(ctx, "node-red"); err == nil || !strings.Contains(err.Error(), "decode desktop store volumes") {
		t.Fatalf("GetInstalled corrupt JSON error = %v, want decode volumes error", err)
	}
}

func TestStorePortAcceptsChecksLocalTCPPort(t *testing.T) {
	ln, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen readiness port: %v", err)
	}
	defer ln.Close()
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			_ = conn.Close()
		}
	}()
	port := ln.Addr().(*net.TCPAddr).Port
	if !storePortAccepts(context.Background(), "0.0.0.0", port) {
		t.Fatal("expected port probe to reach listener through local fallback")
	}
}

func newTestService(t *testing.T, docker DockerAdapter, desktopAdapter DesktopAdapter, launchpad LaunchpadAdapter, ports PortAllocator) *Service {
	t.Helper()
	return newTestServiceWithCatalog(t, docker, desktopAdapter, launchpad, ports, nil)
}

func newTestServiceWithCatalog(t *testing.T, docker DockerAdapter, desktopAdapter DesktopAdapter, launchpad LaunchpadAdapter, ports PortAllocator, catalog []CatalogEntry) *Service {
	t.Helper()
	svc, err := NewService(Config{
		DBPath:        filepath.Join(t.TempDir(), "desktop_store.db"),
		Docker:        docker,
		Desktop:       desktopAdapter,
		Launchpad:     launchpad,
		PortAllocator: ports,
		PortProbe:     func(context.Context, string, int) bool { return true },
		Catalog:       catalog,
	})
	if err != nil {
		t.Fatalf("new service: %v", err)
	}
	if err := svc.Init(context.Background()); err != nil {
		t.Fatalf("init service: %v", err)
	}
	t.Cleanup(func() {
		_ = svc.Close()
	})
	return svc
}

func fixedPorts(values ...int) PortAllocator {
	index := 0
	return func(context.Context, int) (int, error) {
		if index >= len(values) {
			return values[len(values)-1], nil
		}
		value := values[index]
		index++
		return value, nil
	}
}

type fakeDockerAdapter struct {
	pulled            []string
	created           []ContainerSpec
	started           []string
	stopped           []string
	restarted         []string
	removedContainers map[string]int
	removedVolumes    []string
	createErr         error
	createErrors      []error
	startErrors       []error
	inspectCalls      int
	inspectState      ContainerState
	inspectErr        error
}

func (f *fakeDockerAdapter) PullImage(_ context.Context, image string) error {
	f.pulled = append(f.pulled, image)
	return nil
}

func (f *fakeDockerAdapter) CreateContainer(_ context.Context, spec ContainerSpec) (string, error) {
	if len(f.createErrors) > 0 {
		err := f.createErrors[0]
		f.createErrors = f.createErrors[1:]
		if err != nil {
			return "", err
		}
	}
	if f.createErr != nil {
		return "", f.createErr
	}
	f.created = append(f.created, spec)
	return "container-" + spec.Name, nil
}

func (f *fakeDockerAdapter) StartContainer(_ context.Context, name string) error {
	f.started = append(f.started, name)
	if len(f.startErrors) > 0 {
		err := f.startErrors[0]
		f.startErrors = f.startErrors[1:]
		return err
	}
	return nil
}

func (f *fakeDockerAdapter) StopContainer(_ context.Context, name string) error {
	f.stopped = append(f.stopped, name)
	return nil
}

func (f *fakeDockerAdapter) RestartContainer(_ context.Context, name string) error {
	f.restarted = append(f.restarted, name)
	return nil
}

func (f *fakeDockerAdapter) RemoveContainer(_ context.Context, name string, _ bool) error {
	if f.removedContainers == nil {
		f.removedContainers = map[string]int{}
	}
	f.removedContainers[name]++
	return nil
}

func (f *fakeDockerAdapter) RemoveVolume(_ context.Context, name string, _ bool) error {
	f.removedVolumes = append(f.removedVolumes, name)
	return nil
}

func (f *fakeDockerAdapter) InspectContainer(_ context.Context, name string) (ContainerState, error) {
	f.inspectCalls++
	if f.inspectErr != nil {
		return ContainerState{}, f.inspectErr
	}
	if f.inspectState.Name != "" || f.inspectState.Status != "" || f.inspectState.Health != "" {
		if f.inspectState.Name == "" {
			f.inspectState.Name = name
		}
		return f.inspectState, nil
	}
	return ContainerState{Name: name, Running: true, Status: "running"}, nil
}

type fakeDesktopAdapter struct {
	installed     desktop.AppManifest
	files         map[string]string
	dockVisible   *bool
	startVisible  *bool
	shortcutAppID string
	deletedAppID  string
	installErr    error
}

func (f *fakeDesktopAdapter) InstallApp(_ context.Context, manifest desktop.AppManifest, files map[string]string, _ string) error {
	if f.installErr != nil {
		return f.installErr
	}
	f.installed = manifest
	f.files = files
	return nil
}

func (f *fakeDesktopAdapter) SetAppVisibility(_ context.Context, _ string, dockVisible, startVisible *bool, _ string) error {
	f.dockVisible = dockVisible
	f.startVisible = startVisible
	return nil
}

func (f *fakeDesktopAdapter) AddDesktopAppShortcut(_ context.Context, appID, _ string) error {
	f.shortcutAppID = appID
	return nil
}

func (f *fakeDesktopAdapter) DeleteApp(_ context.Context, appID, _ string) error {
	f.deletedAppID = appID
	return nil
}

type fakeLaunchpadAdapter struct {
	upserted  LaunchpadLink
	deletedID string
}

func (f *fakeLaunchpadAdapter) UpsertStoreLink(_ context.Context, link LaunchpadLink) (string, error) {
	f.upserted = link
	return link.ID, nil
}

func (f *fakeLaunchpadAdapter) DeleteStoreLink(_ context.Context, id string) error {
	f.deletedID = id
	return nil
}
