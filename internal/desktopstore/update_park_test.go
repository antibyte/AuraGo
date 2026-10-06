package desktopstore

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"testing"
)

var errRenameUnsupported = errors.New("rename is not supported by this engine")

var dockerContainerNamePattern = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_.-]+$`)

func runStoreInstall(t *testing.T, svc *Service, appID string) {
	t.Helper()
	op, err := svc.StartInstall(context.Background(), InstallRequest{AppID: appID, BindMode: BindModeLocal})
	if err != nil {
		t.Fatalf("start install %s: %v", appID, err)
	}
	if err := svc.RunOperation(context.Background(), op.ID); err != nil {
		t.Fatalf("run install %s: %v", appID, err)
	}
}

func runStoreOperation(t *testing.T, svc *Service, appID, opType string) error {
	t.Helper()
	op, err := svc.StartAppOperation(context.Background(), appID, opType, OperationRequest{})
	if err != nil {
		t.Fatalf("start %s %s: %v", opType, appID, err)
	}
	return svc.RunOperation(context.Background(), op.ID)
}

// assertEventOrder checks that every wanted event occurs, in this order.
func assertEventOrder(t *testing.T, events []string, ordered ...string) {
	t.Helper()
	last := -1
	for _, want := range ordered {
		index := -1
		for i := last + 1; i < len(events); i++ {
			if events[i] == want {
				index = i
				break
			}
		}
		if index < 0 {
			t.Fatalf("event %q missing after position %d in %#v", want, last, events)
		}
		last = index
	}
}

// envKeys lists the variable names of an env slice, never the values.
func envKeys(env []string) []string {
	keys := make([]string, 0, len(env))
	for _, item := range env {
		key, _, _ := strings.Cut(item, "=")
		keys = append(keys, key)
	}
	return keys
}

func legacyDozzleCatalog() []CatalogEntry {
	catalog := DefaultCatalog()
	for i := range catalog {
		if catalog[i].ID != "dozzle" {
			continue
		}
		catalog[i].Env = nil
		catalog[i].HostBinds = []HostBindTemplate{{HostPath: "/var/run/docker.sock", ContainerPath: "/var/run/docker.sock", ReadOnly: true}}
		catalog[i].Companions = nil
	}
	return catalog
}

func TestParkedContainerNamesCannotCollideWithCatalogContainers(t *testing.T) {
	names := map[string]bool{}
	for _, entry := range DefaultCatalog() {
		names[ContainerName(entry.ID)] = true
		for _, companion := range entry.Companions {
			if !storeAppIDPattern.MatchString(normalizeAppID(companion.ID)) {
				t.Fatalf("%s companion ID %q must use only [a-z0-9-] so parked names stay unique", entry.ID, companion.ID)
			}
			names[CompanionContainerName(entry.ID, companion.ID)] = true
		}
	}
	for name := range names {
		parked := parkedContainerName(name)
		if names[parked] {
			t.Fatalf("parked name %q equals a catalog container name", parked)
		}
		if !dockerContainerNamePattern.MatchString(parked) {
			t.Fatalf("parked name %q is not a valid Docker container name", parked)
		}
	}
}

func TestParkContainerHandlesEveryEngineAnswer(t *testing.T) {
	const name = "aurago-store-demo"
	parked := parkedContainerName(name)
	for _, tc := range []struct {
		label       string
		existing    []string
		renameErr   error
		wantParked  bool
		wantRemoved bool
		wantNames   []string
	}{
		{label: "existing container is parked", existing: []string{name}, wantParked: true, wantNames: []string{parked}},
		{label: "stale parked leftover is replaced", existing: []string{name, parked}, wantParked: true, wantNames: []string{parked}},
		{label: "container parked by an interrupted update is adopted", existing: []string{parked}, wantParked: true, wantNames: []string{parked}},
		{label: "missing container leaves nothing to park", wantNames: []string{}},
		{label: "engine without rename falls back to removal", existing: []string{name}, renameErr: errRenameUnsupported, wantRemoved: true, wantNames: []string{}},
		{label: "rename 404 for an existing container falls back to removal", existing: []string{name}, renameErr: fmt.Errorf("container %s %w", name, errContainerNotFound), wantRemoved: true, wantNames: []string{}},
	} {
		t.Run(tc.label, func(t *testing.T) {
			docker := &fakeDockerAdapter{trackContainers: true, renameErr: tc.renameErr}
			for _, existing := range tc.existing {
				docker.addContainer(existing)
			}
			svc := newTestService(t, docker, &fakeDesktopAdapter{}, &fakeLaunchpadAdapter{}, fixedPorts(19700))
			gotParked, gotRemoved, err := svc.parkContainer(context.Background(), name)
			if err != nil {
				t.Fatalf("parkContainer: %v", err)
			}
			if gotParked != tc.wantParked || gotRemoved != tc.wantRemoved {
				t.Fatalf("parked=%v removed=%v, want parked=%v removed=%v", gotParked, gotRemoved, tc.wantParked, tc.wantRemoved)
			}
			if got := docker.containerNames(); !reflect.DeepEqual(got, tc.wantNames) {
				t.Fatalf("containers afterwards = %v, want %v", got, tc.wantNames)
			}
		})
	}
}

// Backlog "Store update rollback for legacy Dozzle/Beszel": the rollback must
// restore the legacy container itself, because recreating its record fails
// the K10 bind check.
func TestLegacyDozzleUpdateFailureKeepsTheOldContainer(t *testing.T) {
	ctx := context.Background()
	dbPath := filepath.Join(t.TempDir(), "desktop_store.db")
	docker := &fakeDockerAdapter{trackContainers: true, traceLifecycle: true}
	svc := newTestServiceAtPath(t, dbPath, docker, &fakeDesktopAdapter{}, &fakeLaunchpadAdapter{}, fixedPorts(18080), legacyDozzleCatalog())
	runStoreInstall(t, svc, "dozzle")
	legacy, ok, err := svc.GetInstalled(ctx, "dozzle")
	if err != nil || !ok || !hasDockerSocketBind(legacy.HostBinds) {
		t.Fatalf("legacy Dozzle record = %#v, ok=%v err=%v; want a direct socket bind", legacy, ok, err)
	}
	if err := svc.Close(); err != nil {
		t.Fatalf("close legacy service: %v", err)
	}

	docker.events = nil
	docker.created = nil
	docker.enforceCatalogBindTrust = true
	// Update creates: the proxy companion succeeds, the updated Dozzle fails.
	docker.createErrors = []error{nil, errors.New("create updated container failed")}
	svc = newTestServiceAtPath(t, dbPath, docker, &fakeDesktopAdapter{}, &fakeLaunchpadAdapter{}, fixedPorts(18080), nil)

	if err := runStoreOperation(t, svc, "dozzle", OperationUpdate); err == nil {
		t.Fatal("update succeeded, want the create failure")
	}
	assertEventOrder(t, docker.events,
		"create:aurago-store-dozzle-socket-proxy",
		"stop:aurago-store-dozzle",
		"rename:aurago-store-dozzle->aurago-store-dozzle.prev",
		"remove:aurago-store-dozzle-socket-proxy",
		"rename:aurago-store-dozzle.prev->aurago-store-dozzle",
		"start:aurago-store-dozzle",
	)
	for _, spec := range docker.created {
		if spec.Name == "aurago-store-dozzle" {
			t.Fatalf("rollback recreated Dozzle from its record instead of restoring the old container: %#v", spec)
		}
	}
	if got, want := docker.containerNames(), []string{"aurago-store-dozzle"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("containers after the failed update = %v, want only the legacy Dozzle", got)
	}
	stored, ok, err := svc.GetInstalled(ctx, "dozzle")
	if err != nil || !ok {
		t.Fatalf("get Dozzle after the failed update: ok=%v err=%v", ok, err)
	}
	if stored.Status != AppStatusRunning || stored.LastOperationState != OperationFailed || stored.Error != "" {
		t.Fatalf("stored state = status %q operation %q error %q, want running, failed, no rollback error", stored.Status, stored.LastOperationState, stored.Error)
	}
	if stored.ContainerID != legacy.ContainerID || !hasDockerSocketBind(stored.HostBinds) || len(stored.Companions) != 0 || !stored.UpdateRequired {
		t.Fatalf("stored record = %#v, want the unchanged legacy record that still requires the update", stored)
	}
}

func TestLegacyBeszelUpdateFailureKeepsTheOldHubAndAgent(t *testing.T) {
	ctx := context.Background()
	dbPath := filepath.Join(t.TempDir(), "desktop_store.db")
	docker := &fakeDockerAdapter{trackContainers: true, traceLifecycle: true}
	secrets := &fakeSecretStore{data: map[string]string{}}
	svc := newTestServiceAtPathWithSecrets(t, dbPath, docker, &fakeDesktopAdapter{}, &fakeLaunchpadAdapter{}, fixedPorts(18090, 23750), nil, secrets)
	runStoreInstall(t, svc, "beszel")
	app, err := svc.ConfigureBeszelAgent(ctx, "ssh-ed25519 public-key", "agent-token")
	if err != nil {
		t.Fatalf("configure Beszel agent: %v", err)
	}
	// Rewrite the record as a pre-proxy install: the agent mounts the socket
	// directly and no proxy exists.
	var legacyAgent CompanionApp
	for _, companion := range app.Companions {
		if companion.ID == "agent" {
			legacyAgent = companion
		}
	}
	legacyAgent.HostBinds = []HostBinding{{HostPath: "/var/run/docker.sock", ContainerPath: "/var/run/docker.sock", ReadOnly: true}}
	app.Companions = []CompanionApp{legacyAgent}
	if err := svc.saveInstalled(ctx, app); err != nil {
		t.Fatalf("save legacy Beszel record: %v", err)
	}
	if err := docker.RemoveContainer(ctx, "aurago-store-beszel-socket-proxy", true); err != nil {
		t.Fatalf("drop the proxy a legacy install never had: %v", err)
	}
	if err := svc.Close(); err != nil {
		t.Fatalf("close legacy service: %v", err)
	}

	docker.events = nil
	docker.created = nil
	docker.enforceCatalogBindTrust = true
	// Update creates: proxy and agent succeed, the updated hub fails.
	docker.createErrors = []error{nil, nil, errors.New("create updated hub failed")}
	svc = newTestServiceAtPathWithSecrets(t, dbPath, docker, &fakeDesktopAdapter{}, &fakeLaunchpadAdapter{}, fixedPorts(18090, 23751), nil, secrets)

	if err := runStoreOperation(t, svc, "beszel", OperationUpdate); err == nil {
		t.Fatal("update succeeded, want the hub create failure")
	}
	assertEventOrder(t, docker.events,
		"rename:aurago-store-beszel-agent->aurago-store-beszel-agent.prev",
		"rename:aurago-store-beszel->aurago-store-beszel.prev",
		"remove:aurago-store-beszel-agent",
		"remove:aurago-store-beszel-socket-proxy",
		"rename:aurago-store-beszel-agent.prev->aurago-store-beszel-agent",
		"start:aurago-store-beszel-agent",
		"rename:aurago-store-beszel.prev->aurago-store-beszel",
		"start:aurago-store-beszel",
	)
	for _, spec := range docker.created {
		if spec.Name != "aurago-store-beszel-socket-proxy" && hasDockerSocketBind(spec.HostBinds) {
			t.Fatalf("rollback recreated %s with a direct socket bind: %#v", spec.Name, spec)
		}
	}
	if got, want := docker.containerNames(), []string{"aurago-store-beszel", "aurago-store-beszel-agent"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("containers after the failed update = %v, want the legacy hub and agent", got)
	}
	stored, ok, err := svc.GetInstalled(ctx, "beszel")
	if err != nil || !ok {
		t.Fatalf("get Beszel after the failed update: ok=%v err=%v", ok, err)
	}
	if stored.Status != AppStatusRunning || stored.LastOperationState != OperationFailed || stored.Error != "" {
		t.Fatalf("stored state = status %q operation %q error %q, want running, failed, no rollback error", stored.Status, stored.LastOperationState, stored.Error)
	}
	if len(stored.Companions) != 1 || !hasDockerSocketBind(stored.Companions[0].HostBinds) || !stored.UpdateRequired {
		t.Fatalf("stored companions = %#v, want the unchanged legacy agent that still requires the update", stored.Companions)
	}
}

func TestUpdateRestoresParkedContainersWhenTheNewAppFails(t *testing.T) {
	ctx := context.Background()
	docker := &fakeDockerAdapter{trackContainers: true, traceLifecycle: true}
	secrets := &fakeSecretStore{data: map[string]string{}}
	svc := newTestServiceWithSecrets(t, docker, &fakeDesktopAdapter{}, &fakeLaunchpadAdapter{}, fixedPorts(17677), secrets)
	runStoreInstall(t, svc, "romm")
	docker.events = nil
	docker.created = nil
	// Starts: the new database, then the new app fails.
	docker.startErrors = []error{nil, errors.New("updated app start failed")}

	if err := runStoreOperation(t, svc, "romm", OperationUpdate); err == nil {
		t.Fatal("update succeeded, want the app start failure")
	}
	assertEventOrder(t, docker.events,
		"stop:aurago-store-romm-db",
		"rename:aurago-store-romm-db->aurago-store-romm-db.prev",
		"create:aurago-store-romm-db",
		"stop:aurago-store-romm",
		"rename:aurago-store-romm->aurago-store-romm.prev",
		"create:aurago-store-romm",
		"remove:aurago-store-romm",
		"remove:aurago-store-romm-db",
		"rename:aurago-store-romm-db.prev->aurago-store-romm-db",
		"start:aurago-store-romm-db",
		"rename:aurago-store-romm.prev->aurago-store-romm",
		"start:aurago-store-romm",
	)
	if len(docker.created) != 2 {
		t.Fatalf("created %d containers, want only the two replacements: %#v", len(docker.created), docker.created)
	}
	if got, want := docker.containerNames(), []string{"aurago-store-romm", "aurago-store-romm-db"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("containers after the failed update = %v, want %v", got, want)
	}
	stored, _, err := svc.GetInstalled(ctx, "romm")
	if err != nil || stored.Status != AppStatusRunning || stored.LastOperationState != OperationFailed {
		t.Fatalf("stored state = %q/%q (%v), want running/failed", stored.Status, stored.LastOperationState, err)
	}
}

// CompanionApp.Env is not persisted (json:"-"), so a rollback that recreates a
// companion from the stored record creates it with an empty env: RomM's
// MariaDB loses MYSQL_*, the Dozzle proxy loses CONTAINERS=1 and the rest of
// its read-only profile. With rename support the rollback restores the
// previous container itself, which keeps the env it was created with.
func TestFailedUpdateRestoresPreviousCompanionsWithTheirEnv(t *testing.T) {
	for _, tc := range []struct {
		appID     string
		companion string
		wantKeys  []string
		port      int
	}{
		{appID: "romm", companion: "aurago-store-romm-db", wantKeys: []string{"MYSQL_DATABASE", "MYSQL_USER", "MYSQL_PASSWORD", "MYSQL_ROOT_PASSWORD"}, port: 17681},
		{appID: "dozzle", companion: "aurago-store-dozzle-socket-proxy", wantKeys: []string{"CONTAINERS", "EVENTS", "INFO", "POST"}, port: 18081},
	} {
		t.Run(tc.appID, func(t *testing.T) {
			docker := &fakeDockerAdapter{trackContainers: true}
			secrets := &fakeSecretStore{data: map[string]string{}}
			svc := newTestServiceWithSecrets(t, docker, &fakeDesktopAdapter{}, &fakeLaunchpadAdapter{}, fixedPorts(tc.port), secrets)
			runStoreInstall(t, svc, tc.appID)
			installed, ok := docker.containerSpec(tc.companion)
			if !ok {
				t.Fatalf("install did not create %s", tc.companion)
			}
			for _, key := range tc.wantKeys {
				if !containsString(envKeys(installed.Env), key) {
					t.Fatalf("installed %s env keys = %v, want %s", tc.companion, envKeys(installed.Env), key)
				}
			}
			docker.created = nil
			// Starts: the new companion, then the new app fails.
			docker.startErrors = []error{nil, errors.New("updated app start failed")}

			if err := runStoreOperation(t, svc, tc.appID, OperationUpdate); err == nil {
				t.Fatal("update succeeded, want the app start failure")
			}
			restored, ok := docker.containerSpec(tc.companion)
			if !ok {
				t.Fatalf("no %s after the rollback; containers = %v", tc.companion, docker.containerNames())
			}
			if !reflect.DeepEqual(restored.Env, installed.Env) {
				t.Fatalf("restored %s env keys = %v, want the installed env keys %v", tc.companion, envKeys(restored.Env), envKeys(installed.Env))
			}
			for _, spec := range docker.created {
				if spec.Name == tc.companion && len(spec.Env) == 0 {
					t.Fatalf("the update or its rollback created %s with an empty env", tc.companion)
				}
			}
		})
	}
}

func TestUpdateRemovesParkedContainersOnlyAfterTheNewOnesRun(t *testing.T) {
	docker := &fakeDockerAdapter{trackContainers: true, traceLifecycle: true}
	svc := newTestService(t, docker, &fakeDesktopAdapter{}, &fakeLaunchpadAdapter{}, fixedPorts(19336))
	runStoreInstall(t, svc, "uptime-kuma")
	before := docker.created[0]
	docker.events = nil

	if err := runStoreOperation(t, svc, "uptime-kuma", OperationUpdate); err != nil {
		t.Fatalf("run update: %v", err)
	}
	assertEventOrder(t, docker.events,
		"stop:aurago-store-uptime-kuma",
		"rename:aurago-store-uptime-kuma->aurago-store-uptime-kuma.prev",
		"create:aurago-store-uptime-kuma",
		"start:aurago-store-uptime-kuma",
		"remove:aurago-store-uptime-kuma.prev",
	)
	after := docker.created[1]
	if before.PortBindings[0] != after.PortBindings[0] || before.Volumes[0] != after.Volumes[0] {
		t.Fatalf("update changed ports or volumes: before %#v after %#v", before, after)
	}
	if docker.removedContainers["aurago-store-uptime-kuma"] != 0 {
		t.Fatalf("the replacement was removed: %#v", docker.removedContainers)
	}
	if got, want := docker.containerNames(), []string{"aurago-store-uptime-kuma"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("containers after the update = %v, want %v", got, want)
	}
}

func TestUpdateOfAStoppedAppRestoresItStopped(t *testing.T) {
	ctx := context.Background()
	docker := &fakeDockerAdapter{trackContainers: true, traceLifecycle: true}
	svc := newTestService(t, docker, &fakeDesktopAdapter{}, &fakeLaunchpadAdapter{}, fixedPorts(19337))
	runStoreInstall(t, svc, "node-red")
	if err := runStoreOperation(t, svc, "node-red", OperationStop); err != nil {
		t.Fatalf("stop: %v", err)
	}
	docker.events = nil
	docker.createErrors = []error{errors.New("create failed")}

	if err := runStoreOperation(t, svc, "node-red", OperationUpdate); err == nil {
		t.Fatal("update succeeded, want the create failure")
	}
	assertEventOrder(t, docker.events,
		"rename:aurago-store-node-red->aurago-store-node-red.prev",
		"rename:aurago-store-node-red.prev->aurago-store-node-red",
	)
	for _, event := range docker.events {
		if event == "start:aurago-store-node-red" {
			t.Fatalf("the restored container of a stopped app was started: %#v", docker.events)
		}
	}
	stored, _, err := svc.GetInstalled(ctx, "node-red")
	if err != nil || stored.Status != AppStatusStopped || stored.LastOperationState != OperationFailed {
		t.Fatalf("stored state = %q/%q (%v), want stopped/failed", stored.Status, stored.LastOperationState, err)
	}
}

func TestUpdateRecreatesFromTheRecordWhenTheOldContainerIsMissing(t *testing.T) {
	ctx := context.Background()
	docker := &fakeDockerAdapter{trackContainers: true, traceLifecycle: true}
	svc := newTestService(t, docker, &fakeDesktopAdapter{}, &fakeLaunchpadAdapter{}, fixedPorts(19556))
	runStoreInstall(t, svc, "excalidraw")
	if err := docker.RemoveContainer(ctx, "aurago-store-excalidraw", true); err != nil {
		t.Fatal(err)
	}
	docker.events = nil
	docker.created = nil
	docker.createErrors = []error{errors.New("create failed")}

	if err := runStoreOperation(t, svc, "excalidraw", OperationUpdate); err == nil {
		t.Fatal("update succeeded, want the create failure")
	}
	if len(docker.created) != 1 || docker.created[0].Name != "aurago-store-excalidraw" {
		t.Fatalf("created = %#v, want the previous container recreated from its record", docker.created)
	}
	assertEventOrder(t, docker.events, "create:aurago-store-excalidraw", "start:aurago-store-excalidraw")
	stored, _, err := svc.GetInstalled(ctx, "excalidraw")
	if err != nil || stored.Status != AppStatusRunning || stored.Error != "" {
		t.Fatalf("stored state = %q error %q (%v), want running without error", stored.Status, stored.Error, err)
	}
}

// A rollback restores the parked previous containers as they were created; it
// does not recreate them, so the current catalog's hardening is not applied to
// the previous images.
func TestRollbackRestoresParkedContainersWithoutReapplyingHardening(t *testing.T) {
	docker := &fakeDockerAdapter{trackContainers: true, traceLifecycle: true}
	dbPath := filepath.Join(t.TempDir(), "desktop_store.db")
	svc := newTestServiceAtPath(t, dbPath, docker, &fakeDesktopAdapter{}, &fakeLaunchpadAdapter{}, fixedPorts(19605), hardenedDemoCatalog("old", false))
	installHardenedDemo(t, svc)
	if err := svc.Close(); err != nil {
		t.Fatalf("close service: %v", err)
	}
	docker.created = nil
	docker.events = nil
	docker.startErrors = []error{nil, nil, errors.New("updated app start failed")}
	svc = newTestServiceAtPath(t, dbPath, docker, &fakeDesktopAdapter{}, &fakeLaunchpadAdapter{}, fixedPorts(19605), hardenedDemoCatalog("new", true))

	if err := runStoreOperation(t, svc, "hardened-demo", OperationUpdate); err == nil {
		t.Fatal("update succeeded, want the app start failure")
	}
	if len(docker.created) != 3 {
		t.Fatalf("created = %d, want only the three replacements: %#v", len(docker.created), docker.created)
	}
	for i, want := range []string{"example/sidecar:new", "example/plain:new", "example/demo:new"} {
		if docker.created[i].Image != want {
			t.Fatalf("created[%d] image = %q, want %q", i, docker.created[i].Image, want)
		}
	}
	sidecar := CompanionContainerName("hardened-demo", "sidecar")
	plain := CompanionContainerName("hardened-demo", "plain")
	app := ContainerName("hardened-demo")
	assertEventOrder(t, docker.events,
		"rename:"+parkedContainerName(sidecar)+"->"+sidecar,
		"rename:"+parkedContainerName(plain)+"->"+plain,
		"rename:"+parkedContainerName(app)+"->"+app,
		"start:"+app,
	)
	for _, name := range []string{sidecar, plain, app} {
		spec, ok := docker.containerSpec(name)
		if !ok || !strings.HasSuffix(spec.Image, ":old") || spec.Hardening != nil {
			t.Fatalf("%s after the rollback = %#v (ok=%v), want the previous unhardened container", name, spec, ok)
		}
	}
}

func TestUninstallRemovesParkedLeftovers(t *testing.T) {
	docker := &fakeDockerAdapter{trackContainers: true}
	secrets := &fakeSecretStore{data: map[string]string{}}
	svc := newTestServiceWithSecrets(t, docker, &fakeDesktopAdapter{}, &fakeLaunchpadAdapter{}, fixedPorts(17678), secrets)
	runStoreInstall(t, svc, "romm")
	// What an update interrupted by a restart can leave behind.
	docker.addContainer(parkedContainerName("aurago-store-romm"))
	docker.addContainer(parkedContainerName("aurago-store-romm-db"))

	if err := runStoreOperation(t, svc, "romm", OperationUninstall); err != nil {
		t.Fatalf("uninstall: %v", err)
	}
	if got := docker.containerNames(); len(got) != 0 {
		t.Fatalf("containers after uninstall = %v, want none", got)
	}
}
