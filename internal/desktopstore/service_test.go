package desktopstore

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"aurago/internal/desktop"
)

func TestDefaultCatalogContainsInitialApps(t *testing.T) {
	catalog := DefaultCatalog()
	if len(catalog) != 17 {
		t.Fatalf("expected 17 catalog apps, got %d", len(catalog))
	}

	expected := map[string]struct {
		image string
		port  int
		icon  string
	}{
		"homarr":              {image: "ghcr.io/homarr-labs/homarr:latest", port: 7575, icon: "home"},
		"n8n":                 {image: "ghcr.io/n8n-io/n8n:latest", port: 5678, icon: "workflow"},
		"node-red":            {image: "ghcr.io/node-red/node-red:latest", port: 1880, icon: "workflow"},
		"open-webui":          {image: "ghcr.io/open-webui/open-webui:main", port: 8080, icon: "chat"},
		"adguard-home":        {image: "adguard/adguardhome", port: 3000, icon: "network"},
		"excalidraw":          {image: "excalidraw/excalidraw:latest", port: 80, icon: "editor"},
		"uptime-kuma":         {image: "ghcr.io/louislam/uptime-kuma:2", port: 3001, icon: "monitor"},
		"olivetin":            {image: "ghcr.io/olivetin/olivetin:latest", port: 1337, icon: "terminal"},
		"bytestash":           {image: "ghcr.io/jordan-dalby/bytestash:latest", port: 5000, icon: "code"},
		"it-tools":            {image: "ghcr.io/corentinth/it-tools:latest", port: 80, icon: "tools"},
		"filebrowser-quantum": {image: "ghcr.io/gtsteffaniak/filebrowser:stable", port: 80, icon: "folder"},
		"stirling-pdf":        {image: "ghcr.io/stirling-tools/stirling-pdf:latest", port: 8080, icon: "pdf"},
		"quakejs-rootless":    {image: "docker.io/awakenedpower/quakejs-rootless:latest", port: 8080, icon: "run"},
		"romm":                {image: "ghcr.io/rommapp/romm:latest", port: 8080, icon: "run"},
		"beszel":              {image: "ghcr.io/henrygd/beszel/beszel:latest", port: 8090, icon: "monitor"},
		"dozzle":              {image: "ghcr.io/amir20/dozzle:latest", port: 8080, icon: "terminal"},
		"code-server":         {image: "ghcr.io/linuxserver/code-server:latest", port: 8443, icon: "code"},
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
		if entry.ID == "uptime-kuma" {
			hasFrameEnv := false
			for _, env := range entry.Env {
				if env == "UPTIME_KUMA_DISABLE_FRAME_SAMEORIGIN=true" {
					hasFrameEnv = true
					break
				}
			}
			if !hasFrameEnv {
				t.Fatalf("uptime-kuma must disable sameorigin frame protection for AuraGo desktop embedding: %#v", entry.Env)
			}
		}
		if entry.ID == "romm" {
			if len(entry.ExtraPorts) != 0 {
				t.Fatalf("romm should expose only its web UI, got extra ports %#v", entry.ExtraPorts)
			}
			wantVolumes := map[string]string{
				"resources":  "/romm/resources",
				"redis-data": "/redis-data",
				"library":    "/romm/library",
				"assets":     "/romm/assets",
				"config":     "/romm/config",
			}
			for _, volume := range entry.Volumes {
				if wantVolumes[volume.NameSuffix] == volume.ContainerPath {
					delete(wantVolumes, volume.NameSuffix)
				}
			}
			if len(wantVolumes) != 0 {
				t.Fatalf("romm missing expected volumes: %#v in %#v", wantVolumes, entry.Volumes)
			}
			for _, key := range []string{"db_password", "db_root_password", "auth_secret_key"} {
				if !catalogSecretKey(entry.GeneratedSecrets, key) {
					t.Fatalf("romm missing generated secret %q in %#v", key, entry.GeneratedSecrets)
				}
			}
			if len(entry.Companions) != 1 {
				t.Fatalf("romm must define a database companion, got %#v", entry.Companions)
			}
			db := entry.Companions[0]
			if db.ID != "db" || db.Name != "RomM MariaDB" || db.Image != "ghcr.io/linuxserver/mariadb:latest" {
				t.Fatalf("romm database companion identity = %#v", db)
			}
			if db.NetworkMode != "aurago-store-romm-net" {
				t.Fatalf("romm database companion network = %q, want private store network", db.NetworkMode)
			}
			if len(db.Volumes) != 1 || db.Volumes[0].ContainerPath != "/config" {
				t.Fatalf("romm database companion volume = %#v", db.Volumes)
			}
			if entry.Metadata["open_external"] != "true" {
				t.Fatalf("romm must open outside the desktop iframe to avoid browser renderer crashes: %#v", entry.Metadata)
			}
		}
		if entry.ID == "olivetin" {
			if len(entry.Volumes) != 0 {
				t.Fatalf("olivetin config must not be hidden in a named volume: %#v", entry.Volumes)
			}
			if len(entry.WorkspaceBinds) != 1 || entry.WorkspaceBinds[0].WorkspacePath != "Shared/OliveTin" || entry.WorkspaceBinds[0].ContainerPath != "/config" {
				t.Fatalf("olivetin workspace bind = %#v, want Shared/OliveTin:/config", entry.WorkspaceBinds)
			}
		}
		if entry.ID == "quakejs-rootless" {
			if entry.Metadata["open_maximized"] != "true" || entry.Metadata["frame_features"] != "game" {
				t.Fatalf("quakejs-rootless must request maximized game-friendly embedding metadata: %#v", entry.Metadata)
			}
		}
		if entry.ID == "dozzle" {
			if len(entry.HostBinds) != 1 || entry.HostBinds[0].HostPath != "/var/run/docker.sock" || !entry.HostBinds[0].ReadOnly {
				t.Fatalf("dozzle must mount Docker socket read-only: %#v", entry.HostBinds)
			}
		}
		if entry.ID == "beszel" {
			if len(entry.Companions) != 1 || entry.Companions[0].ID != "agent" || entry.Companions[0].NetworkMode != "host" {
				t.Fatalf("beszel must define a host-network local agent companion: %#v", entry.Companions)
			}
			if entry.Companions[0].Image != "ghcr.io/henrygd/beszel/beszel-agent:latest" {
				t.Fatalf("beszel agent image = %q", entry.Companions[0].Image)
			}
		}
		if entry.ID == "code-server" {
			if len(entry.GeneratedSecrets) != 1 || entry.GeneratedSecrets[0].Env != "PASSWORD" || !entry.GeneratedSecrets[0].Expose {
				t.Fatalf("code-server must define an exposed generated PASSWORD secret: %#v", entry.GeneratedSecrets)
			}
		}
		delete(expected, entry.ID)
	}
	if len(expected) != 0 {
		t.Fatalf("missing catalog entries: %#v", expected)
	}
}

func catalogSecretKey(secrets []GeneratedSecret, key string) bool {
	for _, secret := range secrets {
		if secret.Key == key {
			return true
		}
	}
	return false
}

func TestDockerCreatePayloadSupportsMultiPortHostBindsAndHostNetwork(t *testing.T) {
	payload := dockerCreatePayload(ContainerSpec{
		Name:  "aurago-store-demo",
		Image: "ghcr.io/example/demo:latest",
		PortBindings: []PortBinding{
			{ID: "manager", ContainerPort: 3000, Protocol: "tcp", HostIP: "127.0.0.1", HostPort: 19300},
			{ID: "frontend", ContainerPort: 80, Protocol: "tcp", HostIP: "127.0.0.1", HostPort: 19080},
		},
		Volumes: []VolumeBinding{{Name: "aurago_store_demo_data", ContainerPath: "/data"}},
		HostBinds: []HostBinding{{
			HostPath:      "/var/run/docker.sock",
			ContainerPath: "/var/run/docker.sock",
			ReadOnly:      true,
		}},
		NetworkMode: "host",
	})
	hostConfig := payload["HostConfig"].(map[string]any)
	if hostConfig["NetworkMode"] != "host" {
		t.Fatalf("NetworkMode = %#v, want host", hostConfig["NetworkMode"])
	}
	binds := hostConfig["Binds"].([]string)
	if !containsString(binds, "aurago_store_demo_data:/data") {
		t.Fatalf("named volume bind missing from %#v", binds)
	}
	if !containsString(binds, "/var/run/docker.sock:/var/run/docker.sock:ro") {
		t.Fatalf("read-only host bind missing from %#v", binds)
	}
	portBindings := hostConfig["PortBindings"].(map[string]any)
	if _, ok := portBindings["3000/tcp"]; !ok {
		t.Fatalf("manager port missing from %#v", portBindings)
	}
	if _, ok := portBindings["80/tcp"]; !ok {
		t.Fatalf("frontend port missing from %#v", portBindings)
	}
}

func TestDockerCreatePayloadEnablesNoNewPrivileges(t *testing.T) {
	payload := dockerCreatePayload(ContainerSpec{
		Name:  "aurago-store-demo",
		Image: "ghcr.io/example/demo:latest",
		PortBindings: []PortBinding{{
			ContainerPort: 8080,
			Protocol:      "tcp",
			HostIP:        "127.0.0.1",
			HostPort:      18080,
		}},
	})
	hostConfig, ok := payload["HostConfig"].(map[string]any)
	if !ok {
		t.Fatalf("HostConfig missing from payload: %#v", payload)
	}
	securityOpt, ok := hostConfig["SecurityOpt"].([]string)
	if !ok {
		t.Fatalf("SecurityOpt missing or wrong type: %#v", hostConfig["SecurityOpt"])
	}
	if len(securityOpt) != 1 || securityOpt[0] != "no-new-privileges:true" {
		t.Fatalf("SecurityOpt = %#v, want no-new-privileges", securityOpt)
	}
}

func TestInstallByteStashGeneratesAndPreservesJWTSecret(t *testing.T) {
	ctx := context.Background()
	docker := &fakeDockerAdapter{}
	svc := newTestService(t, docker, &fakeDesktopAdapter{}, &fakeLaunchpadAdapter{}, fixedPorts(19190))

	installOp, err := svc.StartInstall(ctx, InstallRequest{AppID: "bytestash", BindMode: BindModeLocal})
	if err != nil {
		t.Fatalf("start install: %v", err)
	}
	if err := svc.RunOperation(ctx, installOp.ID); err != nil {
		t.Fatalf("run install: %v", err)
	}
	before, ok, err := svc.GetInstalled(ctx, "bytestash")
	if err != nil || !ok {
		t.Fatalf("get installed bytestash: ok=%v err=%v", ok, err)
	}
	secret, ok := envValue(before.Env, "JWT_SECRET")
	if !ok || len(secret) != 64 {
		t.Fatalf("installed bytestash JWT_SECRET missing or wrong length: %#v", before.Env)
	}
	if len(docker.created) != 1 || docker.created[0].Image != "ghcr.io/jordan-dalby/bytestash:latest" {
		t.Fatalf("unexpected bytestash container spec: %#v", docker.created)
	}
	if len(docker.created[0].Volumes) != 1 || docker.created[0].Volumes[0].ContainerPath != "/data/snippets" {
		t.Fatalf("unexpected bytestash volumes: %#v", docker.created[0].Volumes)
	}

	updateOp, err := svc.StartAppOperation(ctx, "bytestash", OperationUpdate, OperationRequest{})
	if err != nil {
		t.Fatalf("start update: %v", err)
	}
	if err := svc.RunOperation(ctx, updateOp.ID); err != nil {
		t.Fatalf("run update: %v", err)
	}

	after, ok, err := svc.GetInstalled(ctx, "bytestash")
	if err != nil || !ok {
		t.Fatalf("get updated bytestash: ok=%v err=%v", ok, err)
	}
	updatedSecret, ok := envValue(after.Env, "JWT_SECRET")
	if !ok || updatedSecret != secret {
		t.Fatalf("bytestash JWT_SECRET after update = %q/%v, want original %q", updatedSecret, ok, secret)
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

func TestInstallOperationCopiesCatalogMetadataToDesktopManifest(t *testing.T) {
	ctx := context.Background()
	docker := &fakeDockerAdapter{}
	desktopAdapter := &fakeDesktopAdapter{}
	svc := newTestService(t, docker, desktopAdapter, &fakeLaunchpadAdapter{}, fixedPorts(18088))

	op, err := svc.StartInstall(ctx, InstallRequest{AppID: "quakejs-rootless", BindMode: BindModeLocal})
	if err != nil {
		t.Fatalf("start install: %v", err)
	}
	if err := svc.RunOperation(ctx, op.ID); err != nil {
		t.Fatalf("run install: %v", err)
	}
	if desktopAdapter.installed.Metadata["open_maximized"] != "true" {
		t.Fatalf("desktop manifest did not preserve open_maximized metadata: %#v", desktopAdapter.installed.Metadata)
	}
	if desktopAdapter.installed.Metadata["frame_features"] != "game" {
		t.Fatalf("desktop manifest did not preserve frame_features metadata: %#v", desktopAdapter.installed.Metadata)
	}
}

func TestInstallRomMCreatesDatabaseCompanionNetworkAndSecrets(t *testing.T) {
	ctx := context.Background()
	docker := &fakeDockerAdapter{}
	secrets := &fakeSecretStore{data: map[string]string{}}
	svc := newTestServiceWithSecrets(t, docker, &fakeDesktopAdapter{}, &fakeLaunchpadAdapter{}, fixedPorts(17676), secrets)

	op, err := svc.StartInstall(ctx, InstallRequest{AppID: "romm", BindMode: BindModeLocal})
	if err != nil {
		t.Fatalf("start install: %v", err)
	}
	if err := svc.RunOperation(ctx, op.ID); err != nil {
		t.Fatalf("run install: %v", err)
	}

	stored, ok, err := svc.GetInstalled(ctx, "romm")
	if err != nil || !ok {
		t.Fatalf("get installed romm: ok=%v err=%v", ok, err)
	}
	if stored.HostPort != 17676 || stored.ContainerPort != 8080 {
		t.Fatalf("primary port = %d:%d, want 17676:8080", stored.HostPort, stored.ContainerPort)
	}
	assertPortBinding(t, stored.Ports, "web", 8080, 17676)
	if len(stored.Companions) != 1 || stored.Companions[0].ID != "db" || stored.Companions[0].Status != AppStatusRunning {
		t.Fatalf("romm database companion not persisted as running: %#v", stored.Companions)
	}
	if !containsString(docker.createdNetworks, "aurago-store-romm-net") {
		t.Fatalf("romm private network not created: %#v", docker.createdNetworks)
	}
	if len(docker.created) != 2 {
		t.Fatalf("created containers = %d, want database companion and romm", len(docker.created))
	}
	db := docker.created[0]
	app := docker.created[1]
	if db.Name != "aurago-store-romm-db" || db.Image != "ghcr.io/linuxserver/mariadb:latest" {
		t.Fatalf("romm database container = %#v", db)
	}
	if app.Name != "aurago-store-romm" || app.Image != "ghcr.io/rommapp/romm:latest" {
		t.Fatalf("romm app container = %#v", app)
	}
	if db.NetworkMode != "aurago-store-romm-net" || app.NetworkMode != "aurago-store-romm-net" {
		t.Fatalf("romm containers must use private network, db=%q app=%q", db.NetworkMode, app.NetworkMode)
	}
	if !containsString(docker.events, "start:aurago-store-romm-db") || !containsString(docker.events, "start:aurago-store-romm") {
		t.Fatalf("romm containers were not started: %#v", docker.events)
	}
	if indexOfString(docker.events, "start:aurago-store-romm-db") > indexOfString(docker.events, "start:aurago-store-romm") {
		t.Fatalf("romm database must start before app: %#v", docker.events)
	}
	dbPassword, ok := secrets.data["desktop_store_romm_db_password"]
	if !ok || len(dbPassword) < 24 {
		t.Fatalf("romm database password secret missing: %#v", secrets.data)
	}
	rootPassword, ok := secrets.data["desktop_store_romm_db_root_password"]
	if !ok || len(rootPassword) < 24 {
		t.Fatalf("romm database root password secret missing: %#v", secrets.data)
	}
	authSecret, ok := secrets.data["desktop_store_romm_auth_secret_key"]
	if !ok || len(authSecret) < 24 {
		t.Fatalf("romm auth secret missing: %#v", secrets.data)
	}
	if !containsString(app.Env, "DB_HOST=aurago-store-romm-db") || !containsString(app.Env, "DB_PASSWD="+dbPassword) || !containsString(app.Env, "ROMM_AUTH_SECRET_KEY="+authSecret) {
		t.Fatalf("romm app env missing database or auth secrets: %#v", app.Env)
	}
	if !containsString(db.Env, "MYSQL_PASSWORD="+dbPassword) || !containsString(db.Env, "MYSQL_ROOT_PASSWORD="+rootPassword) {
		t.Fatalf("romm database env missing generated credentials: %#v", db.Env)
	}
	assertVolumeBinding(t, app.Volumes, "aurago_store_romm_resources", "/romm/resources")
	assertVolumeBinding(t, app.Volumes, "aurago_store_romm_library", "/romm/library")
	assertVolumeBinding(t, db.Volumes, "aurago_store_romm_db", "/config")
}

func TestUninstallRomMDeleteDataRemovesCompanionVolumeSecretsAndNetwork(t *testing.T) {
	ctx := context.Background()
	docker := &fakeDockerAdapter{}
	secrets := &fakeSecretStore{data: map[string]string{}}
	svc := newTestServiceWithSecrets(t, docker, &fakeDesktopAdapter{}, &fakeLaunchpadAdapter{}, fixedPorts(17676), secrets)

	op, err := svc.StartInstall(ctx, InstallRequest{AppID: "romm", BindMode: BindModeLocal})
	if err != nil {
		t.Fatalf("start install: %v", err)
	}
	if err := svc.RunOperation(ctx, op.ID); err != nil {
		t.Fatalf("run install: %v", err)
	}

	delOp, err := svc.StartAppOperation(ctx, "romm", OperationUninstall, OperationRequest{DeleteData: true})
	if err != nil {
		t.Fatalf("start uninstall: %v", err)
	}
	if err := svc.RunOperation(ctx, delOp.ID); err != nil {
		t.Fatalf("run uninstall: %v", err)
	}
	for _, volume := range []string{"aurago_store_romm_resources", "aurago_store_romm_redis-data", "aurago_store_romm_library", "aurago_store_romm_assets", "aurago_store_romm_config", "aurago_store_romm_db"} {
		if !containsString(docker.removedVolumes, volume) {
			t.Fatalf("romm volume %s was not removed: %#v", volume, docker.removedVolumes)
		}
	}
	if !containsString(docker.removedNetworks, "aurago-store-romm-net") {
		t.Fatalf("romm private network was not removed: %#v", docker.removedNetworks)
	}
	for _, key := range []string{"desktop_store_romm_db_password", "desktop_store_romm_db_root_password", "desktop_store_romm_auth_secret_key"} {
		if _, ok := secrets.data[key]; ok {
			t.Fatalf("romm secret %s was not deleted: %#v", key, secrets.data)
		}
	}
}

func TestUpdateRomMRecreatesDatabaseCompanionFromVaultSecrets(t *testing.T) {
	ctx := context.Background()
	docker := &fakeDockerAdapter{}
	secrets := &fakeSecretStore{data: map[string]string{}}
	dbPath := filepath.Join(t.TempDir(), "desktop_store.db")
	svc := newTestServiceAtPathWithSecrets(t, dbPath, docker, &fakeDesktopAdapter{}, &fakeLaunchpadAdapter{}, fixedPorts(17676), nil, secrets)

	op, err := svc.StartInstall(ctx, InstallRequest{AppID: "romm", BindMode: BindModeLocal})
	if err != nil {
		t.Fatalf("start install: %v", err)
	}
	if err := svc.RunOperation(ctx, op.ID); err != nil {
		t.Fatalf("run install: %v", err)
	}
	dbPassword := secrets.data["desktop_store_romm_db_password"]
	rootPassword := secrets.data["desktop_store_romm_db_root_password"]
	if err := svc.Close(); err != nil {
		t.Fatalf("close service: %v", err)
	}

	docker.created = nil
	docker.events = nil
	svc = newTestServiceAtPathWithSecrets(t, dbPath, docker, &fakeDesktopAdapter{}, &fakeLaunchpadAdapter{}, fixedPorts(17676), nil, secrets)

	updateOp, err := svc.StartAppOperation(ctx, "romm", OperationUpdate, OperationRequest{})
	if err != nil {
		t.Fatalf("start update: %v", err)
	}
	if err := svc.RunOperation(ctx, updateOp.ID); err != nil {
		t.Fatalf("run update: %v", err)
	}
	if len(docker.created) != 2 {
		t.Fatalf("created containers = %d, want recreated db companion and app: %#v", len(docker.created), docker.created)
	}
	db := docker.created[0]
	if db.Name != "aurago-store-romm-db" {
		t.Fatalf("first recreated container = %#v, want RomM database", db)
	}
	if !containsString(db.Env, "MYSQL_PASSWORD="+dbPassword) || !containsString(db.Env, "MYSQL_ROOT_PASSWORD="+rootPassword) {
		t.Fatalf("recreated romm database env did not use preserved vault secrets: %#v", db.Env)
	}
}

func TestInstallDozzleUsesReadOnlyDockerSocketBindAndDeleteDataOnlyRemovesVolumes(t *testing.T) {
	ctx := context.Background()
	docker := &fakeDockerAdapter{}
	svc := newTestService(t, docker, &fakeDesktopAdapter{}, &fakeLaunchpadAdapter{}, fixedPorts(18080))

	op, err := svc.StartInstall(ctx, InstallRequest{AppID: "dozzle", BindMode: BindModeLocal})
	if err != nil {
		t.Fatalf("start install: %v", err)
	}
	if err := svc.RunOperation(ctx, op.ID); err != nil {
		t.Fatalf("run install: %v", err)
	}
	stored, ok, err := svc.GetInstalled(ctx, "dozzle")
	if err != nil || !ok {
		t.Fatalf("get installed dozzle: ok=%v err=%v", ok, err)
	}
	if len(stored.HostBinds) != 1 || !stored.HostBinds[0].ReadOnly {
		t.Fatalf("stored host binds = %#v, want read-only Docker socket bind", stored.HostBinds)
	}
	if len(docker.created) != 1 || len(docker.created[0].HostBinds) != 1 || !docker.created[0].HostBinds[0].ReadOnly {
		t.Fatalf("created dozzle host binds = %#v", docker.created)
	}

	delOp, err := svc.StartAppOperation(ctx, "dozzle", OperationUninstall, OperationRequest{DeleteData: true})
	if err != nil {
		t.Fatalf("start uninstall: %v", err)
	}
	if err := svc.RunOperation(ctx, delOp.ID); err != nil {
		t.Fatalf("run uninstall: %v", err)
	}
	if containsString(docker.removedVolumes, "/var/run/docker.sock") {
		t.Fatalf("host bind was treated as removable volume: %#v", docker.removedVolumes)
	}
	if !containsString(docker.removedVolumes, "aurago_store_dozzle_data") {
		t.Fatalf("dozzle data volume was not removed: %#v", docker.removedVolumes)
	}
}

func TestInstallCodeServerGeneratesVaultPasswordAndPreservesItOnUpdate(t *testing.T) {
	ctx := context.Background()
	docker := &fakeDockerAdapter{}
	secrets := &fakeSecretStore{data: map[string]string{}}
	svc := newTestServiceWithSecrets(t, docker, &fakeDesktopAdapter{}, &fakeLaunchpadAdapter{}, fixedPorts(18443), secrets)

	op, err := svc.StartInstall(ctx, InstallRequest{AppID: "code-server", BindMode: BindModeLocal})
	if err != nil {
		t.Fatalf("start install: %v", err)
	}
	if err := svc.RunOperation(ctx, op.ID); err != nil {
		t.Fatalf("run install: %v", err)
	}
	stored, ok, err := svc.GetInstalled(ctx, "code-server")
	if err != nil || !ok {
		t.Fatalf("get installed code-server: ok=%v err=%v", ok, err)
	}
	password, ok := secrets.data["desktop_store_code-server_password"]
	if !ok || len(password) < 24 {
		t.Fatalf("generated code-server password missing or too short: %#v", secrets.data)
	}
	if got, ok := envValue(stored.Env, "PASSWORD"); !ok || got != password {
		t.Fatalf("stored env PASSWORD = %q/%v, want vault password", got, ok)
	}
	if len(stored.SecretRefs) != 1 || !stored.SecretRefs[0].Expose {
		t.Fatalf("stored secret refs = %#v, want exposed password ref", stored.SecretRefs)
	}
	data, err := json.Marshal(stored)
	if err != nil {
		t.Fatalf("marshal installed app: %v", err)
	}
	if strings.Contains(string(data), password) || strings.Contains(string(data), `"env"`) || strings.Contains(string(data), "desktop_store_code-server_password") {
		t.Fatalf("runtime secret leaked in installed app JSON: %s", string(data))
	}

	updateOp, err := svc.StartAppOperation(ctx, "code-server", OperationUpdate, OperationRequest{})
	if err != nil {
		t.Fatalf("start update: %v", err)
	}
	if err := svc.RunOperation(ctx, updateOp.ID); err != nil {
		t.Fatalf("run update: %v", err)
	}
	updated, ok, err := svc.GetInstalled(ctx, "code-server")
	if err != nil || !ok {
		t.Fatalf("get updated code-server: ok=%v err=%v", ok, err)
	}
	if got, _ := envValue(updated.Env, "PASSWORD"); got != password {
		t.Fatalf("code-server password changed on update: %q want %q", got, password)
	}
}

func TestConfigureBeszelAgentCreatesHostNetworkCompanionWithVaultSecrets(t *testing.T) {
	ctx := context.Background()
	docker := &fakeDockerAdapter{}
	secrets := &fakeSecretStore{data: map[string]string{}}
	svc := newTestServiceWithSecrets(t, docker, &fakeDesktopAdapter{}, &fakeLaunchpadAdapter{}, fixedPorts(18090), secrets)

	op, err := svc.StartInstall(ctx, InstallRequest{AppID: "beszel", BindMode: BindModeLocal})
	if err != nil {
		t.Fatalf("start install: %v", err)
	}
	if err := svc.RunOperation(ctx, op.ID); err != nil {
		t.Fatalf("run install: %v", err)
	}
	app, err := svc.ConfigureBeszelAgent(ctx, "ssh-ed25519 public-key", "agent-token")
	if err != nil {
		t.Fatalf("configure beszel agent: %v", err)
	}
	if secrets.data["desktop_store_beszel_agent_key"] != "ssh-ed25519 public-key" || secrets.data["desktop_store_beszel_agent_token"] != "agent-token" {
		t.Fatalf("beszel agent secrets not stored in vault: %#v", secrets.data)
	}
	if len(app.Companions) != 1 || app.Companions[0].ID != "agent" || app.Companions[0].Status != AppStatusRunning {
		t.Fatalf("beszel companion not persisted as running: %#v", app.Companions)
	}
	if len(docker.created) != 2 {
		t.Fatalf("created containers = %d, want hub and agent", len(docker.created))
	}
	agent := docker.created[1]
	if agent.Name != "aurago-store-beszel-agent" || agent.Image != "ghcr.io/henrygd/beszel/beszel-agent:latest" {
		t.Fatalf("agent container identity = %#v", agent)
	}
	if agent.NetworkMode != "host" {
		t.Fatalf("agent network mode = %q, want host", agent.NetworkMode)
	}
	if !containsString(agent.Env, "LISTEN=/beszel_socket/beszel.sock") || !containsString(agent.Env, "HUB_URL=http://localhost:18090") {
		t.Fatalf("agent env missing socket listen or hub URL: %#v", agent.Env)
	}
	if !containsString(agent.Env, "KEY=ssh-ed25519 public-key") || !containsString(agent.Env, "TOKEN=agent-token") {
		t.Fatalf("agent env missing vault secrets: %#v", agent.Env)
	}
	if len(agent.HostBinds) != 1 || agent.HostBinds[0].HostPath != "/var/run/docker.sock" || !agent.HostBinds[0].ReadOnly {
		t.Fatalf("agent Docker socket bind = %#v", agent.HostBinds)
	}
	assertVolumeBinding(t, agent.Volumes, "aurago_store_beszel_socket", "/beszel_socket")
	assertVolumeBinding(t, agent.Volumes, "aurago_store_beszel_agent_data", "/var/lib/beszel-agent")
}

func TestInstallOliveTinMountsEditableWorkspaceConfigBeforeStart(t *testing.T) {
	ctx := context.Background()
	docker := &fakeDockerAdapter{}
	svc := newTestService(t, docker, &fakeDesktopAdapter{}, &fakeLaunchpadAdapter{}, fixedPorts(19189))

	op, err := svc.StartInstall(ctx, InstallRequest{AppID: "olivetin", BindMode: BindModeLocal})
	if err != nil {
		t.Fatalf("start install: %v", err)
	}
	if err := svc.RunOperation(ctx, op.ID); err != nil {
		t.Fatalf("run install: %v", err)
	}

	if len(docker.created) != 1 {
		t.Fatalf("created containers = %d, want 1", len(docker.created))
	}
	spec := docker.created[0]
	if spec.Image != "ghcr.io/olivetin/olivetin:latest" {
		t.Fatalf("olivetin image = %q", spec.Image)
	}
	if len(spec.Volumes) != 0 {
		t.Fatalf("olivetin should not use a hidden named config volume: %#v", spec.Volumes)
	}
	workspaceConfigDir := filepath.Join(svc.cfg.WorkspaceDir, "Shared", "OliveTin")
	if len(spec.HostBinds) != 1 || spec.HostBinds[0].HostPath != workspaceConfigDir || spec.HostBinds[0].ContainerPath != "/config" || !spec.HostBinds[0].Managed {
		t.Fatalf("unexpected olivetin host binds: %#v, want managed workspace config bind", spec.HostBinds)
	}
	seededConfigPath := filepath.Join(workspaceConfigDir, "config.yaml")
	seededConfig, err := os.ReadFile(seededConfigPath)
	if err != nil {
		t.Fatalf("read seeded olivetin config: %v", err)
	}
	if !strings.Contains(string(seededConfig), `title: "Hello world!"`) {
		t.Fatalf("olivetin default config not seeded: %s", string(seededConfig))
	}
	if _, copied := docker.copiedFiles["aurago-store-olivetin:/config"]; copied {
		t.Fatalf("olivetin workspace config should be seeded on host, not docker-copied: %#v", docker.copiedFiles)
	}
	createIndex := indexOfString(docker.events, "create:aurago-store-olivetin")
	startIndex := indexOfString(docker.events, "start:aurago-store-olivetin")
	if createIndex > startIndex {
		t.Fatalf("olivetin config mount must exist before start: %#v", docker.events)
	}
}

func TestInstallOliveTinKeepsExistingWorkspaceConfig(t *testing.T) {
	ctx := context.Background()
	docker := &fakeDockerAdapter{}
	svc := newTestService(t, docker, &fakeDesktopAdapter{}, &fakeLaunchpadAdapter{}, fixedPorts(19189))
	workspaceConfigDir := filepath.Join(svc.cfg.WorkspaceDir, "Shared", "OliveTin")
	if err := os.MkdirAll(workspaceConfigDir, 0o755); err != nil {
		t.Fatalf("create existing olivetin config dir: %v", err)
	}
	customConfigPath := filepath.Join(workspaceConfigDir, "config.yaml")
	if err := os.WriteFile(customConfigPath, []byte("actions:\n  - title: Existing\n"), 0o644); err != nil {
		t.Fatalf("write existing olivetin config: %v", err)
	}

	op, err := svc.StartInstall(ctx, InstallRequest{AppID: "olivetin", BindMode: BindModeLocal})
	if err != nil {
		t.Fatalf("start install: %v", err)
	}
	if err := svc.RunOperation(ctx, op.ID); err != nil {
		t.Fatalf("run install: %v", err)
	}

	kept, err := os.ReadFile(customConfigPath)
	if err != nil {
		t.Fatalf("read existing olivetin config: %v", err)
	}
	if string(kept) != "actions:\n  - title: Existing\n" {
		t.Fatalf("existing olivetin config was overwritten: %q", string(kept))
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

func TestInstallReplacesFailedInstallingRecord(t *testing.T) {
	ctx := context.Background()
	docker := &fakeDockerAdapter{}
	desktopAdapter := &fakeDesktopAdapter{}
	launchpad := &fakeLaunchpadAdapter{}
	svc := newTestService(t, docker, desktopAdapter, launchpad, fixedPorts(19184))

	staleOp, err := svc.StartInstall(ctx, InstallRequest{AppID: "node-red", BindMode: BindModeLocal})
	if err != nil {
		t.Fatalf("start stale install: %v", err)
	}
	if err := svc.updateOperation(ctx, staleOp.ID, OperationFailed, "", "interrupted before cleanup"); err != nil {
		t.Fatalf("mark stale operation failed: %v", err)
	}
	record := svc.buildInstallRecord(svc.catalogByID["node-red"], staleOp, BindModeLocal, "127.0.0.1", 19184, false)
	record.LastOperationState = OperationRunning
	if err := svc.saveInstalled(ctx, record); err != nil {
		t.Fatalf("seed failed installing record: %v", err)
	}

	op, err := svc.StartInstall(ctx, InstallRequest{AppID: "node-red", BindMode: BindModeLocal})
	if err != nil {
		t.Fatalf("start replacement install: %v", err)
	}
	if err := svc.RunOperation(ctx, op.ID); err != nil {
		t.Fatalf("run replacement install: %v", err)
	}
	stored, ok, err := svc.GetInstalled(ctx, "node-red")
	if err != nil || !ok {
		t.Fatalf("get replacement install: ok=%v err=%v", ok, err)
	}
	if stored.Status != AppStatusRunning || stored.LastOperationID != op.ID || stored.LastOperationState != OperationSucceeded {
		t.Fatalf("replacement record not running from new operation: %#v", stored)
	}
	if docker.removedContainers["aurago-store-node-red"] == 0 {
		t.Fatalf("stale container cleanup not attempted: %#v", docker.removedContainers)
	}
	if len(docker.removedVolumes) == 0 {
		t.Fatal("stale volumes were not cleaned before replacement install")
	}
}

func TestInitRecoversInterruptedInstallingOperation(t *testing.T) {
	ctx := context.Background()
	dbPath := filepath.Join(t.TempDir(), "desktop_store.db")
	docker := &fakeDockerAdapter{}
	desktopAdapter := &fakeDesktopAdapter{}
	launchpad := &fakeLaunchpadAdapter{}
	svc := newTestServiceAtPath(t, dbPath, docker, desktopAdapter, launchpad, fixedPorts(19187), nil)

	op, err := svc.StartInstall(ctx, InstallRequest{AppID: "node-red", BindMode: BindModeLocal})
	if err != nil {
		t.Fatalf("start install: %v", err)
	}
	if err := svc.updateOperation(ctx, op.ID, OperationRunning, "running", ""); err != nil {
		t.Fatalf("mark operation running: %v", err)
	}
	record := svc.buildInstallRecord(svc.catalogByID["node-red"], op, BindModeLocal, "127.0.0.1", 19187, false)
	if err := svc.saveInstalled(ctx, record); err != nil {
		t.Fatalf("seed installing record: %v", err)
	}
	if err := svc.Close(); err != nil {
		t.Fatalf("close first service: %v", err)
	}

	recoveryDocker := &fakeDockerAdapter{}
	recoveredSvc := newTestServiceAtPath(t, dbPath, recoveryDocker, &fakeDesktopAdapter{}, &fakeLaunchpadAdapter{}, fixedPorts(19188), nil)
	recoveredOp, err := recoveredSvc.Operation(ctx, op.ID)
	if err != nil {
		t.Fatalf("load recovered operation: %v", err)
	}
	if recoveredOp.Status != OperationFailed || recoveredOp.CompletedAt == nil {
		t.Fatalf("interrupted operation not failed on startup: %#v", recoveredOp)
	}
	if _, ok, err := recoveredSvc.GetInstalled(ctx, "node-red"); err != nil || ok {
		t.Fatalf("installing record should be removed on startup recovery: ok=%v err=%v", ok, err)
	}
	if recoveryDocker.removedContainers["aurago-store-node-red"] == 0 {
		t.Fatalf("startup recovery did not clean container: %#v", recoveryDocker.removedContainers)
	}
	if len(recoveryDocker.removedVolumes) == 0 {
		t.Fatal("startup recovery did not clean volumes")
	}
}

func TestInitClearsStaleActiveOperationMarkerForRunningApp(t *testing.T) {
	ctx := context.Background()
	dbPath := filepath.Join(t.TempDir(), "desktop_store.db")
	svc := newTestServiceAtPath(t, dbPath, &fakeDockerAdapter{}, &fakeDesktopAdapter{}, &fakeLaunchpadAdapter{}, fixedPorts(17575), nil)

	op, err := svc.StartInstall(ctx, InstallRequest{AppID: "homarr", BindMode: BindModeLocal})
	if err != nil {
		t.Fatalf("start install: %v", err)
	}
	if err := svc.updateOperation(ctx, op.ID, OperationRunning, "running", ""); err != nil {
		t.Fatalf("mark operation running: %v", err)
	}
	record := svc.buildInstallRecord(svc.catalogByID["homarr"], op, BindModeLocal, "127.0.0.1", 17575, false)
	record.Status = AppStatusRunning
	record.LastOperationState = OperationRunning
	if err := svc.saveInstalled(ctx, record); err != nil {
		t.Fatalf("seed running record with stale operation marker: %v", err)
	}
	if err := svc.Close(); err != nil {
		t.Fatalf("close first service: %v", err)
	}

	recoveredSvc := newTestServiceAtPath(t, dbPath, &fakeDockerAdapter{}, &fakeDesktopAdapter{}, &fakeLaunchpadAdapter{}, fixedPorts(17576), nil)
	stored, ok, err := recoveredSvc.GetInstalled(ctx, "homarr")
	if err != nil || !ok {
		t.Fatalf("get recovered app: ok=%v err=%v", ok, err)
	}
	if stored.Status != AppStatusRunning {
		t.Fatalf("running app status changed during metadata recovery: %#v", stored)
	}
	if stored.LastOperationState != OperationFailed {
		t.Fatalf("stale active operation marker = %q, want %q", stored.LastOperationState, OperationFailed)
	}
}

func TestStartInstallRejectsConcurrentOperationForSameApp(t *testing.T) {
	ctx := context.Background()
	svc := newTestService(t, &fakeDockerAdapter{}, &fakeDesktopAdapter{}, &fakeLaunchpadAdapter{}, fixedPorts(19185))

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
	svc := newTestService(t, &fakeDockerAdapter{}, &fakeDesktopAdapter{}, &fakeLaunchpadAdapter{}, fixedPorts(19186))

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

func TestUpdateOperationRefreshesCatalogEnv(t *testing.T) {
	ctx := context.Background()
	docker := &fakeDockerAdapter{}
	catalog := []CatalogEntry{
		{
			ID:          "demo-app",
			Name:        "Demo App",
			Description: "Demo app for env update tests.",
			Image:       "example/demo:new",
			Icon:        "package",
			LogoSlug:    "demo",
			LogoURL:     "https://example.invalid/demo.png",
			PrimaryPort: PortSpec{ContainerPort: 8080, Protocol: "tcp"},
			Volumes:     []VolumeTemplate{{NameSuffix: "data", ContainerPath: "/data"}},
			Env:         []string{"DEMO_SETTING=new"},
		},
	}
	svc := newTestServiceWithCatalog(t, docker, &fakeDesktopAdapter{}, &fakeLaunchpadAdapter{}, fixedPorts(19889), catalog)

	seedOp := Operation{ID: "op-seed", Type: OperationInstall}
	record := svc.buildInstallRecord(catalog[0], seedOp, BindModeLocal, "127.0.0.1", 19889, false)
	record.Env = []string{"DEMO_SETTING=old"}
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
	if got := strings.Join(docker.created[0].Env, ","); got != "DEMO_SETTING=new" {
		t.Fatalf("updated container env = %q, want current catalog env", got)
	}
	stored, ok, err := svc.GetInstalled(ctx, "demo-app")
	if err != nil || !ok {
		t.Fatalf("get updated app: ok=%v err=%v", ok, err)
	}
	if got := strings.Join(stored.Env, ","); got != "DEMO_SETTING=new" {
		t.Fatalf("stored env = %q, want current catalog env", got)
	}
}

func TestUpdateOperationPreservesHomarrSecretEnv(t *testing.T) {
	ctx := context.Background()
	docker := &fakeDockerAdapter{}
	svc := newTestService(t, docker, &fakeDesktopAdapter{}, &fakeLaunchpadAdapter{}, fixedPorts(19339))

	installOp, err := svc.StartInstall(ctx, InstallRequest{AppID: "homarr", BindMode: BindModeLocal})
	if err != nil {
		t.Fatalf("start install: %v", err)
	}
	if err := svc.RunOperation(ctx, installOp.ID); err != nil {
		t.Fatalf("run install: %v", err)
	}
	before, ok, err := svc.GetInstalled(ctx, "homarr")
	if err != nil || !ok {
		t.Fatalf("get installed homarr: ok=%v err=%v", ok, err)
	}
	secret, ok := envValue(before.Env, "SECRET_ENCRYPTION_KEY")
	if !ok || secret == "" {
		t.Fatalf("installed homarr secret env missing: %#v", before.Env)
	}

	updateOp, err := svc.StartAppOperation(ctx, "homarr", OperationUpdate, OperationRequest{})
	if err != nil {
		t.Fatalf("start update: %v", err)
	}
	if err := svc.RunOperation(ctx, updateOp.ID); err != nil {
		t.Fatalf("run update: %v", err)
	}

	after, ok, err := svc.GetInstalled(ctx, "homarr")
	if err != nil || !ok {
		t.Fatalf("get updated homarr: ok=%v err=%v", ok, err)
	}
	updatedSecret, ok := envValue(after.Env, "SECRET_ENCRYPTION_KEY")
	if !ok || updatedSecret != secret {
		t.Fatalf("homarr secret after update = %q/%v, want original %q", updatedSecret, ok, secret)
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

func TestUninstallIgnoresMissingDesktopApp(t *testing.T) {
	ctx := context.Background()
	docker := &fakeDockerAdapter{}
	desktopAdapter := &fakeDesktopAdapter{}
	svc := newTestService(t, docker, desktopAdapter, &fakeLaunchpadAdapter{}, fixedPorts(19445))

	installOp, err := svc.StartInstall(ctx, InstallRequest{AppID: "node-red", BindMode: BindModeLocal})
	if err != nil {
		t.Fatalf("start install: %v", err)
	}
	if err := svc.RunOperation(ctx, installOp.ID); err != nil {
		t.Fatalf("run install: %v", err)
	}
	desktopAdapter.deleteErr = errors.New("desktop app not found")

	uninstallOp, err := svc.StartAppOperation(ctx, "node-red", OperationUninstall, OperationRequest{DeleteData: false})
	if err != nil {
		t.Fatalf("start uninstall: %v", err)
	}
	if err := svc.RunOperation(ctx, uninstallOp.ID); err != nil {
		t.Fatalf("run uninstall: %v", err)
	}
	if _, ok, err := svc.GetInstalled(ctx, "node-red"); err != nil || ok {
		t.Fatalf("install record should be removed despite missing desktop app: ok=%v err=%v", ok, err)
	}
	if desktopAdapter.deletedAppID != "store-node-red" {
		t.Fatalf("deleted desktop app = %q", desktopAdapter.deletedAppID)
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
	return newTestServiceAtPath(t, filepath.Join(t.TempDir(), "desktop_store.db"), docker, desktopAdapter, launchpad, ports, nil)
}

func newTestServiceWithCatalog(t *testing.T, docker DockerAdapter, desktopAdapter DesktopAdapter, launchpad LaunchpadAdapter, ports PortAllocator, catalog []CatalogEntry) *Service {
	t.Helper()
	return newTestServiceAtPath(t, filepath.Join(t.TempDir(), "desktop_store.db"), docker, desktopAdapter, launchpad, ports, catalog)
}

func newTestServiceWithSecrets(t *testing.T, docker DockerAdapter, desktopAdapter DesktopAdapter, launchpad LaunchpadAdapter, ports PortAllocator, secrets SecretStore) *Service {
	t.Helper()
	return newTestServiceAtPathWithSecrets(t, filepath.Join(t.TempDir(), "desktop_store.db"), docker, desktopAdapter, launchpad, ports, nil, secrets)
}

func newTestServiceAtPath(t *testing.T, dbPath string, docker DockerAdapter, desktopAdapter DesktopAdapter, launchpad LaunchpadAdapter, ports PortAllocator, catalog []CatalogEntry) *Service {
	t.Helper()
	return newTestServiceAtPathWithSecrets(t, dbPath, docker, desktopAdapter, launchpad, ports, catalog, nil)
}

func newTestServiceAtPathWithSecrets(t *testing.T, dbPath string, docker DockerAdapter, desktopAdapter DesktopAdapter, launchpad LaunchpadAdapter, ports PortAllocator, catalog []CatalogEntry, secrets SecretStore) *Service {
	t.Helper()
	svc, err := NewService(Config{
		DBPath:        dbPath,
		WorkspaceDir:  filepath.Join(t.TempDir(), "workspace"),
		Docker:        docker,
		Desktop:       desktopAdapter,
		Launchpad:     launchpad,
		PortAllocator: ports,
		PortProbe:     func(context.Context, string, int) bool { return true },
		Catalog:       catalog,
		Secrets:       secrets,
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

func assertPortBinding(t *testing.T, ports []PortBinding, id string, containerPort, hostPort int) {
	t.Helper()
	for _, port := range ports {
		if port.ID == id {
			if port.ContainerPort != containerPort || port.HostPort != hostPort {
				t.Fatalf("port %s = %#v, want container %d host %d", id, port, containerPort, hostPort)
			}
			return
		}
	}
	t.Fatalf("port %s missing from %#v", id, ports)
}

func assertVolumeBinding(t *testing.T, volumes []VolumeBinding, name, path string) {
	t.Helper()
	for _, volume := range volumes {
		if volume.Name == name && volume.ContainerPath == path {
			return
		}
	}
	t.Fatalf("volume %s:%s missing from %#v", name, path, volumes)
}

func containsString(items []string, want string) bool {
	for _, item := range items {
		if item == want {
			return true
		}
	}
	return false
}

func indexOfString(items []string, want string) int {
	for i, item := range items {
		if item == want {
			return i
		}
	}
	return len(items)
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
	createdNetworks   []string
	copiedFiles       map[string]map[string]string
	events            []string
	started           []string
	stopped           []string
	restarted         []string
	removedContainers map[string]int
	removedVolumes    []string
	removedNetworks   []string
	createErr         error
	createErrors      []error
	startErrors       []error
	inspectCalls      int
	inspectState      ContainerState
	inspectErr        error
}

type fakeSecretStore struct {
	data map[string]string
}

func (f *fakeSecretStore) ReadSecret(key string) (string, error) {
	if f.data == nil {
		f.data = map[string]string{}
	}
	value, ok := f.data[key]
	if !ok {
		return "", errors.New("secret not found")
	}
	return value, nil
}

func (f *fakeSecretStore) WriteSecret(key, value string) error {
	if f.data == nil {
		f.data = map[string]string{}
	}
	f.data[key] = value
	return nil
}

func (f *fakeSecretStore) DeleteSecret(key string) error {
	if f.data == nil {
		f.data = map[string]string{}
	}
	delete(f.data, key)
	return nil
}

func (f *fakeDockerAdapter) PullImage(_ context.Context, image string) error {
	f.pulled = append(f.pulled, image)
	f.events = append(f.events, "pull:"+image)
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
	f.events = append(f.events, "create:"+spec.Name)
	return "container-" + spec.Name, nil
}

func (f *fakeDockerAdapter) CopyToContainer(_ context.Context, containerName, destDir string, files map[string]string) error {
	if f.copiedFiles == nil {
		f.copiedFiles = map[string]map[string]string{}
	}
	key := containerName + ":" + destDir
	f.copiedFiles[key] = map[string]string{}
	for name, content := range files {
		f.copiedFiles[key][name] = content
	}
	f.events = append(f.events, "copy:"+key)
	return nil
}

func (f *fakeDockerAdapter) StartContainer(_ context.Context, name string) error {
	f.started = append(f.started, name)
	f.events = append(f.events, "start:"+name)
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

func (f *fakeDockerAdapter) CreateNetwork(_ context.Context, name string) error {
	f.createdNetworks = append(f.createdNetworks, name)
	f.events = append(f.events, "network:"+name)
	return nil
}

func (f *fakeDockerAdapter) RemoveNetwork(_ context.Context, name string) error {
	f.removedNetworks = append(f.removedNetworks, name)
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
	deleteErr     error
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
	if f.deleteErr != nil {
		return f.deleteErr
	}
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
