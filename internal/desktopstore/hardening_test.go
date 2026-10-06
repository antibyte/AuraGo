package desktopstore

import (
	"context"
	"encoding/json"
	"errors"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// verifiedCatalogHardening lists the catalog images whose hardening passed a
// live check. Add an entry only together with the live evidence.
var verifiedCatalogHardening = map[string]ContainerHardening{
	"arcane/socket-proxy": {CapDrop: []string{"ALL"}},
}

func TestDockerCreatePayloadAppliesOptInHardening(t *testing.T) {
	payload := dockerCreatePayload(ContainerSpec{
		Name:  "aurago-store-demo",
		Image: "ghcr.io/example/demo:latest",
		Hardening: &ContainerHardening{
			CapDrop:        []string{"ALL"},
			CapAdd:         []string{"NET_BIND_SERVICE"},
			ReadonlyRootfs: true,
			Tmpfs:          map[string]string{"/tmp": "rw,noexec,nosuid,size=16m"},
			PidsLimit:      256,
		},
	})
	hostConfig, ok := payload["HostConfig"].(map[string]any)
	if !ok {
		t.Fatalf("HostConfig missing from payload: %#v", payload)
	}
	if got := hostConfig["SecurityOpt"]; !reflect.DeepEqual(got, []string{"no-new-privileges:true"}) {
		t.Fatalf("SecurityOpt = %#v, want no-new-privileges", got)
	}
	if got := hostConfig["CapDrop"]; !reflect.DeepEqual(got, []string{"ALL"}) {
		t.Fatalf("CapDrop = %#v", got)
	}
	if got := hostConfig["CapAdd"]; !reflect.DeepEqual(got, []string{"NET_BIND_SERVICE"}) {
		t.Fatalf("CapAdd = %#v", got)
	}
	if got := hostConfig["ReadonlyRootfs"]; got != true {
		t.Fatalf("ReadonlyRootfs = %#v", got)
	}
	if got := hostConfig["Tmpfs"]; !reflect.DeepEqual(got, map[string]string{"/tmp": "rw,noexec,nosuid,size=16m"}) {
		t.Fatalf("Tmpfs = %#v", got)
	}
	if got := hostConfig["PidsLimit"]; got != int64(256) {
		t.Fatalf("PidsLimit = %#v", got)
	}
}

func TestDockerCreatePayloadWithoutHardeningKeepsDockerDefaults(t *testing.T) {
	payload := dockerCreatePayload(ContainerSpec{
		Name:  "aurago-store-demo",
		Image: "ghcr.io/example/demo:latest",
	})
	hostConfig, ok := payload["HostConfig"].(map[string]any)
	if !ok {
		t.Fatalf("HostConfig missing from payload: %#v", payload)
	}
	for _, key := range []string{"CapDrop", "CapAdd", "ReadonlyRootfs", "Tmpfs", "PidsLimit"} {
		if value, ok := hostConfig[key]; ok {
			t.Fatalf("HostConfig[%q] = %#v, want Docker's default for apps without catalog hardening", key, value)
		}
	}
	if got := hostConfig["SecurityOpt"]; !reflect.DeepEqual(got, []string{"no-new-privileges:true"}) {
		t.Fatalf("SecurityOpt = %#v, want no-new-privileges", got)
	}
}

func TestCatalogHardeningOptInsAreVerifiedOnly(t *testing.T) {
	seen := map[string]bool{}
	check := func(key string, hardening *ContainerHardening) {
		if hardening == nil {
			return
		}
		want, ok := verifiedCatalogHardening[key]
		if !ok || !reflect.DeepEqual(*hardening, want) {
			t.Fatalf("catalog image %s hardening = %#v; opt in only after a live check", key, *hardening)
		}
		seen[key] = true
	}
	for _, entry := range DefaultCatalog() {
		check(entry.ID, entry.Hardening)
		for _, companion := range entry.Companions {
			check(entry.ID+"/"+companion.ID, companion.Hardening)
		}
	}
	for key := range verifiedCatalogHardening {
		if !seen[key] {
			t.Fatalf("verified hardening for %s is missing from the catalog", key)
		}
	}
}

func TestCatalogJSONDoesNotExposeHardening(t *testing.T) {
	raw, err := json.Marshal(DefaultCatalog())
	if err != nil {
		t.Fatalf("marshal catalog: %v", err)
	}
	for _, key := range []string{`"hardening"`, `"cap_drop"`, `"cap_add"`, `"readonly_rootfs"`, `"pids_limit"`} {
		if strings.Contains(string(raw), key) {
			t.Fatalf("catalog JSON exposes %s; hardening must stay internal (json:\"-\")", key)
		}
	}
}

func TestCloneContainerHardeningDoesNotShareCatalogValues(t *testing.T) {
	source := &ContainerHardening{
		CapDrop:        []string{"ALL"},
		CapAdd:         []string{"NET_BIND_SERVICE"},
		ReadonlyRootfs: true,
		Tmpfs:          map[string]string{"/tmp": "rw"},
		PidsLimit:      64,
	}
	copied := cloneContainerHardening(source)
	if !reflect.DeepEqual(copied, source) {
		t.Fatalf("clone = %#v, want an equal copy of %#v", copied, source)
	}
	copied.CapDrop[0] = "NONE"
	copied.CapAdd[0] = "SYS_ADMIN"
	copied.Tmpfs["/tmp"] = "ro"
	if source.CapDrop[0] != "ALL" || source.CapAdd[0] != "NET_BIND_SERVICE" || source.Tmpfs["/tmp"] != "rw" {
		t.Fatalf("clone shares catalog values: %#v", source)
	}
	if cloneContainerHardening(nil) != nil {
		t.Fatal("nil hardening must stay nil")
	}
}

func TestInstallArcaneAppliesOnlyVerifiedHardening(t *testing.T) {
	ctx := context.Background()
	docker := &fakeDockerAdapter{}
	secrets := &fakeSecretStore{data: map[string]string{}}
	svc := newTestServiceWithSecrets(t, docker, &fakeDesktopAdapter{}, &fakeLaunchpadAdapter{}, fixedPorts(13552), secrets)

	op, err := svc.StartInstall(ctx, InstallRequest{AppID: "arcane", BindMode: BindModeLocal})
	if err != nil {
		t.Fatalf("start install: %v", err)
	}
	if err := svc.RunOperation(ctx, op.ID); err != nil {
		t.Fatalf("run install: %v", err)
	}
	if len(docker.created) != 2 {
		t.Fatalf("created containers = %d, want socket proxy companion and arcane", len(docker.created))
	}
	proxy, app := docker.created[0], docker.created[1]
	if proxy.Name != "aurago-store-arcane-socket-proxy" {
		t.Fatalf("first container = %q, want the socket proxy companion", proxy.Name)
	}
	want, verified := verifiedCatalogHardening["arcane/socket-proxy"]
	switch {
	case verified && (proxy.Hardening == nil || !reflect.DeepEqual(*proxy.Hardening, want)):
		t.Fatalf("socket proxy hardening = %#v, want %#v", proxy.Hardening, want)
	case !verified && proxy.Hardening != nil:
		t.Fatalf("socket proxy hardening = %#v, want none", proxy.Hardening)
	}
	if app.Name != "aurago-store-arcane" || app.Hardening != nil {
		t.Fatalf("arcane app %q hardening = %#v, want none", app.Name, app.Hardening)
	}
}

// The tests below use a custom catalog whose app and one companion declare
// hardening, so every create path is pinned independently of which images the
// real catalog has verified.

func demoAppHardening() *ContainerHardening {
	return &ContainerHardening{
		CapDrop:        []string{"ALL"},
		CapAdd:         []string{"NET_BIND_SERVICE"},
		ReadonlyRootfs: true,
		Tmpfs:          map[string]string{"/tmp": "rw,noexec,nosuid,size=16m"},
		PidsLimit:      128,
	}
}

func demoSidecarHardening() *ContainerHardening {
	return &ContainerHardening{CapDrop: []string{"ALL"}}
}

// hardenedDemoCatalog returns an app "hardened-demo" with two companions
// ("sidecar" and "plain") and a second app "plain-demo". With hardened set, the
// app and the sidecar declare hardening; "plain", "plain-demo" and its companion
// never do. version is part of every image tag.
func hardenedDemoCatalog(version string, hardened bool) []CatalogEntry {
	demo := CatalogEntry{
		ID:          "hardened-demo",
		Name:        "Hardened Demo",
		Description: "Demo app for hardening tests.",
		Image:       "example/demo:" + version,
		Icon:        "package",
		LogoSlug:    "demo",
		LogoURL:     "https://example.invalid/demo.png",
		PrimaryPort: PortSpec{ContainerPort: 8080, Protocol: "tcp"},
		Volumes:     []VolumeTemplate{{NameSuffix: "data", ContainerPath: "/data"}},
		Companions: []CompanionTemplate{
			{ID: "sidecar", Name: "Demo Sidecar", Image: "example/sidecar:" + version},
			{ID: "plain", Name: "Demo Plain Sidecar", Image: "example/plain:" + version},
		},
	}
	if hardened {
		demo.Hardening = demoAppHardening()
		demo.Companions[0].Hardening = demoSidecarHardening()
	}
	plain := CatalogEntry{
		ID:          "plain-demo",
		Name:        "Plain Demo",
		Description: "Demo app without hardening.",
		Image:       "example/plain-demo:" + version,
		Icon:        "package",
		LogoSlug:    "demo",
		LogoURL:     "https://example.invalid/demo.png",
		PrimaryPort: PortSpec{ContainerPort: 8081, Protocol: "tcp"},
		Companions: []CompanionTemplate{
			{ID: "sidecar", Name: "Plain Demo Sidecar", Image: "example/plain-demo-sidecar:" + version},
		},
	}
	return []CatalogEntry{demo, plain}
}

func assertSpecHardening(t *testing.T, label string, spec ContainerSpec, wantName string, want *ContainerHardening) {
	t.Helper()
	if spec.Name != wantName {
		t.Fatalf("%s: container = %q, want %q", label, spec.Name, wantName)
	}
	switch {
	case want == nil && spec.Hardening != nil:
		t.Fatalf("%s: %s hardening = %#v, want none", label, spec.Name, spec.Hardening)
	case want != nil && (spec.Hardening == nil || !reflect.DeepEqual(*spec.Hardening, *want)):
		t.Fatalf("%s: %s hardening = %#v, want %#v", label, spec.Name, spec.Hardening, want)
	}
}

func installHardenedDemo(t *testing.T, svc *Service) {
	t.Helper()
	ctx := context.Background()
	op, err := svc.StartInstall(ctx, InstallRequest{AppID: "hardened-demo", BindMode: BindModeLocal})
	if err != nil {
		t.Fatalf("start install: %v", err)
	}
	if err := svc.RunOperation(ctx, op.ID); err != nil {
		t.Fatalf("run install: %v", err)
	}
}

func TestCatalogHardeningResolvesByNormalizedID(t *testing.T) {
	svc := newTestServiceWithCatalog(t, &fakeDockerAdapter{}, &fakeDesktopAdapter{}, &fakeLaunchpadAdapter{}, fixedPorts(19600), hardenedDemoCatalog("new", true))
	for _, tc := range []struct {
		name      string
		appID     string
		companion string
		want      *ContainerHardening
	}{
		{"app", "hardened-demo", "", demoAppHardening()},
		{"app ID is normalised", " Hardened-Demo ", "", demoAppHardening()},
		{"companion", "hardened-demo", "sidecar", demoSidecarHardening()},
		{"companion ID is normalised", "HARDENED-DEMO", " Sidecar ", demoSidecarHardening()},
		{"companion without hardening", "hardened-demo", "plain", nil},
		{"app without hardening", "plain-demo", "", nil},
		{"companion of an app without hardening", "plain-demo", "sidecar", nil},
		{"unknown app", "missing", "", nil},
		{"unknown companion", "hardened-demo", "missing", nil},
		{"companion of an unknown app", "missing", "sidecar", nil},
		{"empty app ID", "", "", nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := svc.catalogHardening(tc.appID, tc.companion)
			switch {
			case tc.want == nil && got != nil:
				t.Fatalf("catalogHardening(%q, %q) = %#v, want nil", tc.appID, tc.companion, got)
			case tc.want != nil && (got == nil || !reflect.DeepEqual(*got, *tc.want)):
				t.Fatalf("catalogHardening(%q, %q) = %#v, want %#v", tc.appID, tc.companion, got, tc.want)
			}
		})
	}
}

func TestCatalogHardeningReturnsACopy(t *testing.T) {
	svc := newTestServiceWithCatalog(t, &fakeDockerAdapter{}, &fakeDesktopAdapter{}, &fakeLaunchpadAdapter{}, fixedPorts(19600), hardenedDemoCatalog("new", true))

	app := svc.catalogHardening("hardened-demo", "")
	app.CapDrop[0] = "NONE"
	app.CapAdd[0] = "SYS_ADMIN"
	app.Tmpfs["/tmp"] = "ro"
	app.ReadonlyRootfs = false
	app.PidsLimit = 1
	if got := svc.catalogByID["hardened-demo"].Hardening; got == nil || !reflect.DeepEqual(*got, *demoAppHardening()) {
		t.Fatalf("mutating the returned app hardening changed the catalog: %#v", got)
	}
	if got := svc.catalogHardening("hardened-demo", ""); got == nil || !reflect.DeepEqual(*got, *demoAppHardening()) {
		t.Fatalf("second lookup = %#v, want the unchanged catalog value", got)
	}

	sidecar := svc.catalogHardening("hardened-demo", "sidecar")
	sidecar.CapDrop[0] = "NONE"
	sidecar.CapAdd = append(sidecar.CapAdd, "SYS_ADMIN")
	if got := svc.catalogByID["hardened-demo"].Companions[0].Hardening; got == nil || !reflect.DeepEqual(*got, *demoSidecarHardening()) {
		t.Fatalf("mutating the returned companion hardening changed the catalog: %#v", got)
	}
}

func TestCatalogHardeningAppliesOnInstall(t *testing.T) {
	docker := &fakeDockerAdapter{}
	svc := newTestServiceWithCatalog(t, docker, &fakeDesktopAdapter{}, &fakeLaunchpadAdapter{}, fixedPorts(19601), hardenedDemoCatalog("new", true))

	installHardenedDemo(t, svc)

	if len(docker.created) != 3 {
		t.Fatalf("created containers = %d, want sidecar, plain and app: %#v", len(docker.created), docker.created)
	}
	assertSpecHardening(t, "install", docker.created[0], CompanionContainerName("hardened-demo", "sidecar"), demoSidecarHardening())
	assertSpecHardening(t, "install", docker.created[1], CompanionContainerName("hardened-demo", "plain"), nil)
	assertSpecHardening(t, "install", docker.created[2], ContainerName("hardened-demo"), demoAppHardening())
}

func TestCatalogHardeningAppliesOnUpdate(t *testing.T) {
	ctx := context.Background()
	docker := &fakeDockerAdapter{}
	svc := newTestServiceWithCatalog(t, docker, &fakeDesktopAdapter{}, &fakeLaunchpadAdapter{}, fixedPorts(19602), hardenedDemoCatalog("new", true))
	installHardenedDemo(t, svc)
	docker.created = nil

	op, err := svc.StartAppOperation(ctx, "hardened-demo", OperationUpdate, OperationRequest{})
	if err != nil {
		t.Fatalf("start update: %v", err)
	}
	if err := svc.RunOperation(ctx, op.ID); err != nil {
		t.Fatalf("run update: %v", err)
	}

	if len(docker.created) != 3 {
		t.Fatalf("recreated containers = %d, want sidecar, plain and app: %#v", len(docker.created), docker.created)
	}
	assertSpecHardening(t, "update", docker.created[0], CompanionContainerName("hardened-demo", "sidecar"), demoSidecarHardening())
	assertSpecHardening(t, "update", docker.created[1], CompanionContainerName("hardened-demo", "plain"), nil)
	assertSpecHardening(t, "update", docker.created[2], ContainerName("hardened-demo"), demoAppHardening())
}

// Installed records do not store hardening, so a catalog change applies on the
// next create: apps installed before an image opted in pick it up on update.
func TestCatalogHardeningReachesAppsInstalledBeforeTheOptIn(t *testing.T) {
	ctx := context.Background()
	docker := &fakeDockerAdapter{}
	dbPath := filepath.Join(t.TempDir(), "desktop_store.db")

	svc := newTestServiceAtPath(t, dbPath, docker, &fakeDesktopAdapter{}, &fakeLaunchpadAdapter{}, fixedPorts(19603), hardenedDemoCatalog("new", false))
	installHardenedDemo(t, svc)
	for _, spec := range docker.created {
		assertSpecHardening(t, "install before opt-in", spec, spec.Name, nil)
	}
	if err := svc.Close(); err != nil {
		t.Fatalf("close service: %v", err)
	}

	docker.created = nil
	svc = newTestServiceAtPath(t, dbPath, docker, &fakeDesktopAdapter{}, &fakeLaunchpadAdapter{}, fixedPorts(19603), hardenedDemoCatalog("new", true))
	op, err := svc.StartAppOperation(ctx, "hardened-demo", OperationUpdate, OperationRequest{})
	if err != nil {
		t.Fatalf("start update: %v", err)
	}
	if err := svc.RunOperation(ctx, op.ID); err != nil {
		t.Fatalf("run update: %v", err)
	}
	if len(docker.created) != 3 {
		t.Fatalf("recreated containers = %d, want sidecar, plain and app: %#v", len(docker.created), docker.created)
	}
	assertSpecHardening(t, "update after opt-in", docker.created[0], CompanionContainerName("hardened-demo", "sidecar"), demoSidecarHardening())
	assertSpecHardening(t, "update after opt-in", docker.created[1], CompanionContainerName("hardened-demo", "plain"), nil)
	assertSpecHardening(t, "update after opt-in", docker.created[2], ContainerName("hardened-demo"), demoAppHardening())
}

// Rollback recreates the previous record's image with the current catalog's
// hardening (the record stores none). The first three specs are the failed
// update, the last three are restorePreviousCompanions and rollbackPrevious.
func TestCatalogHardeningAppliesOnRollbackWithCurrentCatalog(t *testing.T) {
	ctx := context.Background()
	docker := &fakeDockerAdapter{}
	dbPath := filepath.Join(t.TempDir(), "desktop_store.db")

	svc := newTestServiceAtPath(t, dbPath, docker, &fakeDesktopAdapter{}, &fakeLaunchpadAdapter{}, fixedPorts(19604), hardenedDemoCatalog("old", false))
	installHardenedDemo(t, svc)
	if err := svc.Close(); err != nil {
		t.Fatalf("close service: %v", err)
	}

	docker.created = nil
	docker.events = nil
	// Starts: sidecar, plain, then the updated app fails; the rollback starts
	// sidecar, plain and the previous app again.
	docker.startErrors = []error{nil, nil, errors.New("updated app start failed")}
	svc = newTestServiceAtPath(t, dbPath, docker, &fakeDesktopAdapter{}, &fakeLaunchpadAdapter{}, fixedPorts(19604), hardenedDemoCatalog("new", true))
	op, err := svc.StartAppOperation(ctx, "hardened-demo", OperationUpdate, OperationRequest{})
	if err != nil {
		t.Fatalf("start update: %v", err)
	}
	if err := svc.RunOperation(ctx, op.ID); err == nil {
		t.Fatal("run update succeeded, want the app start failure")
	}

	if len(docker.created) != 6 {
		t.Fatalf("created containers = %d, want 3 for the update and 3 for the rollback: %#v", len(docker.created), docker.created)
	}
	sidecarName := CompanionContainerName("hardened-demo", "sidecar")
	plainName := CompanionContainerName("hardened-demo", "plain")
	appName := ContainerName("hardened-demo")
	wantImages := []string{"example/sidecar:new", "example/plain:new", "example/demo:new", "example/sidecar:old", "example/plain:old", "example/demo:old"}
	for i, want := range wantImages {
		if docker.created[i].Image != want {
			t.Fatalf("created[%d] image = %q, want %q (update first, rollback of the previous images after)", i, docker.created[i].Image, want)
		}
	}
	assertSpecHardening(t, "failed update", docker.created[0], sidecarName, demoSidecarHardening())
	assertSpecHardening(t, "failed update", docker.created[1], plainName, nil)
	assertSpecHardening(t, "failed update", docker.created[2], appName, demoAppHardening())
	assertSpecHardening(t, "rollback companions", docker.created[3], sidecarName, demoSidecarHardening())
	assertSpecHardening(t, "rollback companions", docker.created[4], plainName, nil)
	assertSpecHardening(t, "rollback app", docker.created[5], appName, demoAppHardening())
}

// ConfigureBeszelAgent creates its companion outside the install and update
// flows, so it needs its own coverage. Install also creates the Beszel Docker
// socket proxy, which gets a loopback host port of its own (the second
// allocated port; reusing the hub's port is a conflict), and keeps only the
// hardening the real catalog has verified for it.
func TestCatalogHardeningAppliesToBeszelAgentCompanion(t *testing.T) {
	ctx := context.Background()
	docker := &fakeDockerAdapter{}
	secrets := &fakeSecretStore{data: map[string]string{}}
	catalog := DefaultCatalog()
	for i := range catalog {
		if catalog[i].ID != "beszel" {
			continue
		}
		for j := range catalog[i].Companions {
			if catalog[i].Companions[j].ID == "agent" {
				catalog[i].Companions[j].Hardening = demoSidecarHardening()
			}
		}
	}
	dbPath := filepath.Join(t.TempDir(), "desktop_store.db")
	svc := newTestServiceAtPathWithSecrets(t, dbPath, docker, &fakeDesktopAdapter{}, &fakeLaunchpadAdapter{}, fixedPorts(18091, 23751), catalog, secrets)

	op, err := svc.StartInstall(ctx, InstallRequest{AppID: "beszel", BindMode: BindModeLocal})
	if err != nil {
		t.Fatalf("start install: %v", err)
	}
	if err := svc.RunOperation(ctx, op.ID); err != nil {
		t.Fatalf("run install: %v", err)
	}
	if _, err := svc.ConfigureBeszelAgent(ctx, "ssh-ed25519 public-key", "agent-token"); err != nil {
		t.Fatalf("configure beszel agent: %v", err)
	}

	if len(docker.created) != 3 {
		t.Fatalf("created containers = %d, want socket proxy, hub and agent: %#v", len(docker.created), docker.created)
	}
	var proxyHardening *ContainerHardening
	if verified, ok := verifiedCatalogHardening["beszel/socket-proxy"]; ok {
		proxyHardening = &verified
	}
	assertSpecHardening(t, "beszel socket proxy", docker.created[0], CompanionContainerName("beszel", "socket-proxy"), proxyHardening)
	assertSpecHardening(t, "beszel hub", docker.created[1], ContainerName("beszel"), nil)
	assertSpecHardening(t, "beszel agent", docker.created[2], CompanionContainerName("beszel", "agent"), demoSidecarHardening())
}
