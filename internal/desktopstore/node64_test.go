package desktopstore

import (
	"context"
	"reflect"
	"regexp"
	"strings"
	"testing"
)

func node64Entry(t *testing.T) CatalogEntry {
	t.Helper()
	for _, entry := range DefaultCatalog() {
		if entry.ID == "node64" {
			return entry
		}
	}
	t.Fatal("node64 missing from the catalog")
	return CatalogEntry{}
}

func TestNode64CatalogEntry(t *testing.T) {
	entry := node64Entry(t)
	if !regexp.MustCompile(`^ghcr\.io/ipv64net/node64@sha256:[0-9a-f]{64}$`).MatchString(entry.Image) {
		t.Fatalf("image %q is not the digest pin of the official image", entry.Image)
	}
	if entry.Category != "system" || entry.Icon != "monitor" || entry.LogoSlug != "node64" || entry.LogoURL != "" {
		t.Fatalf("presentation = %q/%q/%q/%q", entry.Category, entry.Icon, entry.LogoSlug, entry.LogoURL)
	}
	// The UI sends X-Frame-Options: DENY, so the Desktop must open it in its own tab.
	if entry.Metadata["open_external"] != "true" {
		t.Fatalf("metadata = %#v, want open_external", entry.Metadata)
	}
	if entry.PrimaryPort != (PortSpec{ID: "web", Name: "Web UI", ContainerPort: 8080, Protocol: "tcp"}) {
		t.Fatalf("primary port = %#v", entry.PrimaryPort)
	}
	if len(entry.ExtraPorts) != 0 || len(entry.HostBinds) != 0 || len(entry.WorkspaceBinds) != 0 || len(entry.Companions) != 0 || len(entry.ExtraHosts) != 0 {
		t.Fatalf("node64 publishes only its web port and mounts nothing from the host: %#v", entry)
	}
	if len(entry.Volumes) != 1 || entry.Volumes[0] != (VolumeTemplate{NameSuffix: "data", ContainerPath: "/data"}) {
		t.Fatalf("volumes = %#v", entry.Volumes)
	}
	if !reflect.DeepEqual(entry.Env, []string{"NODE64_LOG_LEVEL=info", "NODE64_ADMIN_USER=admin"}) {
		t.Fatalf("env = %#v", entry.Env)
	}
	if len(entry.GeneratedSecrets) != 1 || entry.GeneratedSecrets[0] != (GeneratedSecret{Key: "admin_password", Env: "NODE64_ADMIN_PASSWORD", Label: "Admin password (user: admin)", Expose: true}) {
		t.Fatalf("generated secrets = %#v", entry.GeneratedSecrets)
	}
	want := verifiedCatalogHardening["node64"]
	if entry.Hardening == nil || !reflect.DeepEqual(*entry.Hardening, want) {
		t.Fatalf("hardening = %#v, want the probed %#v", entry.Hardening, want)
	}
	for _, phrase := range []string{"own browser tab", "53, 80 and 443", "ICMP"} {
		if !strings.Contains(entry.Description, phrase) {
			t.Fatalf("description lacks %q: %q", phrase, entry.Description)
		}
	}
}

func TestInstallNode64CreatesHardenedContainerWithExposedAdminPassword(t *testing.T) {
	ctx := context.Background()
	docker := &fakeDockerAdapter{}
	desktopAdapter := &fakeDesktopAdapter{}
	secrets := &fakeSecretStore{data: map[string]string{}}
	svc := newTestServiceWithSecrets(t, docker, desktopAdapter, &fakeLaunchpadAdapter{}, fixedPorts(18080), secrets)

	op, err := svc.StartInstall(ctx, InstallRequest{AppID: "node64", BindMode: BindModeLocal})
	if err != nil {
		t.Fatalf("start install: %v", err)
	}
	if err := svc.RunOperation(ctx, op.ID); err != nil {
		t.Fatalf("run install: %v", err)
	}
	password := secrets.data["desktop_store_node64_admin_password"]
	if len(password) < 10 {
		t.Fatalf("generated admin password %q is shorter than Node64's 10-character minimum", password)
	}
	if len(docker.created) != 1 {
		t.Fatalf("created containers = %d, want 1", len(docker.created))
	}
	spec := docker.created[0]
	for key, wantValue := range map[string]string{
		"NODE64_ADMIN_USER":     "admin",
		"NODE64_ADMIN_PASSWORD": password,
		"NODE64_LOG_LEVEL":      "info",
	} {
		if got, _ := envValue(spec.Env, key); got != wantValue {
			t.Fatalf("container env %s = %q, want %q", key, got, wantValue)
		}
	}
	if len(spec.PortBindings) != 1 || spec.PortBindings[0].ContainerPort != 8080 || spec.PortBindings[0].HostIP != "127.0.0.1" || spec.PortBindings[0].HostPort != 18080 {
		t.Fatalf("port bindings = %#v, want only the web UI on loopback", spec.PortBindings)
	}
	if len(spec.HostBinds) != 0 || len(spec.Entrypoint) != 0 {
		t.Fatalf("host binds / entrypoint = %#v / %#v", spec.HostBinds, spec.Entrypoint)
	}
	assertVolumeBinding(t, spec.Volumes, "aurago_store_node64_data", "/data")

	// The probed hardening reaches the Docker create payload.
	hostConfig, _ := dockerCreatePayload(spec)["HostConfig"].(map[string]any)
	if hostConfig["ReadonlyRootfs"] != true || !reflect.DeepEqual(hostConfig["Tmpfs"], map[string]string{"/tmp": ""}) ||
		!reflect.DeepEqual(hostConfig["CapDrop"], []string{"ALL"}) || !reflect.DeepEqual(hostConfig["SecurityOpt"], []string{"no-new-privileges:true"}) {
		t.Fatalf("host config = %#v", hostConfig)
	}

	if desktopAdapter.installed.Metadata["open_external"] != "true" || desktopAdapter.installed.Category != "system" {
		t.Fatalf("desktop manifest = %#v", desktopAdapter.installed)
	}
	credentials, err := svc.ExposedCredentials(ctx, "node64")
	if err != nil {
		t.Fatalf("exposed credentials: %v", err)
	}
	if len(credentials) != 1 || credentials[0].Key != "admin_password" || credentials[0].Value != password || credentials[0].Label != "Admin password (user: admin)" {
		t.Fatalf("exposed credentials = %#v", credentials)
	}

	// An update keeps the password the first start already stored in /data.
	update, err := svc.StartAppOperation(ctx, "node64", OperationUpdate, OperationRequest{})
	if err != nil {
		t.Fatalf("start update: %v", err)
	}
	if err := svc.RunOperation(ctx, update.ID); err != nil {
		t.Fatalf("run update: %v", err)
	}
	if got, _ := envValue(docker.created[len(docker.created)-1].Env, "NODE64_ADMIN_PASSWORD"); got != password {
		t.Fatalf("update changed the admin password: %q", got)
	}
}
