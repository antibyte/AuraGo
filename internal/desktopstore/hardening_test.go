package desktopstore

import (
	"context"
	"encoding/json"
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
	source := &ContainerHardening{CapDrop: []string{"ALL"}, Tmpfs: map[string]string{"/tmp": "rw"}}
	copied := cloneContainerHardening(source)
	copied.CapDrop[0] = "NONE"
	copied.Tmpfs["/tmp"] = "ro"
	if source.CapDrop[0] != "ALL" || source.Tmpfs["/tmp"] != "rw" {
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
