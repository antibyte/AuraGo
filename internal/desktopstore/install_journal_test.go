package desktopstore

import (
	"context"
	"database/sql"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
	"time"

	"aurago/internal/tools"
)

func storeLabels(appID, companionID string) map[string]string {
	labels := map[string]string{"aurago.desktop_store": "true", "aurago.desktop_store.app_id": appID}
	if companionID != "" {
		labels["aurago.desktop_store.companion"] = companionID
	}
	return labels
}

func envFromSpec(t *testing.T, spec ContainerSpec, key string) string {
	t.Helper()
	value, ok := envValue(spec.Env, key)
	if !ok || value == "" {
		t.Fatalf("%s has no %s", spec.Name, key)
	}
	return value
}

func containsVolume(names []string, volumes []VolumeBinding) bool {
	for _, volume := range volumes {
		if containsString(names, volume.Name) {
			return true
		}
	}
	return false
}

// User decision 5 (F-S5): a container with a target name that the Store did
// not create for this app stops the install before anything is created.
func TestInstallStopsBeforeCreatingAnythingWhenAForeignContainerHasTheName(t *testing.T) {
	for i, tc := range []struct {
		label     string
		appID     string
		container string
		labels    map[string]string
	}{
		{label: "foreign app container", appID: "uptime-kuma", container: "aurago-store-uptime-kuma", labels: map[string]string{"com.example.owner": "someone"}},
		{label: "container of another Store app", appID: "uptime-kuma", container: "aurago-store-uptime-kuma", labels: storeLabels("excalidraw", "")},
		{label: "foreign companion name", appID: "romm", container: "aurago-store-romm-db"},
		{label: "Store app container under a companion name", appID: "romm", container: "aurago-store-romm-db", labels: storeLabels("romm", "")},
	} {
		t.Run(tc.label, func(t *testing.T) {
			ctx := context.Background()
			docker := &fakeDockerAdapter{existingContainers: map[string]map[string]string{tc.container: tc.labels}}
			secrets := &fakeSecretStore{data: map[string]string{}}
			svc := newTestServiceWithSecrets(t, docker, &fakeDesktopAdapter{}, &fakeLaunchpadAdapter{}, fixedPorts(19901+i), secrets)
			op, err := svc.StartInstall(ctx, InstallRequest{AppID: tc.appID, BindMode: BindModeLocal})
			if err != nil {
				t.Fatalf("start install: %v", err)
			}
			err = svc.RunOperation(ctx, op.ID)
			var conflict *ContainerNameConflictError
			if !errors.As(err, &conflict) || conflict.Container != tc.container {
				t.Fatalf("install error = %v, want ContainerNameConflictError for %s", err, tc.container)
			}
			if !strings.Contains(err.Error(), "docker rename "+tc.container+" "+tc.container+"-old") {
				t.Fatalf("install error %q does not name the manual fix", err)
			}
			if len(docker.created)+len(docker.createdNetworks)+len(docker.pulled)+len(docker.removedContainers)+len(docker.removedVolumes)+len(docker.removedNetworks) != 0 {
				t.Fatalf("blocked install touched Docker: created=%v networks=%v pulled=%v removed=%v volumes=%v removedNetworks=%v",
					docker.created, docker.createdNetworks, docker.pulled, docker.removedContainers, docker.removedVolumes, docker.removedNetworks)
			}
			if _, ok := docker.existingContainers[tc.container]; !ok {
				t.Fatal("the foreign container is gone")
			}
			if _, ok, _ := svc.GetInstalled(ctx, tc.appID); ok {
				t.Fatal("blocked install kept a record")
			}
			for key := range secrets.data {
				if strings.HasPrefix(key, "desktop_store_"+tc.appID+"_") {
					t.Fatalf("blocked install kept the secret %s it generated", key)
				}
			}
			stored, err := svc.Operation(ctx, op.ID)
			if err != nil {
				t.Fatalf("load operation: %v", err)
			}
			if stored.Status != OperationFailed || stored.ErrorCode != OperationErrorContainerNameInUse || stored.ErrorParams["name"] != tc.container || !strings.Contains(stored.Error, tc.container) {
				t.Fatalf("operation = %#v, want failed with code %s and the container name", stored, OperationErrorContainerNameInUse)
			}
			if resources, err := svc.installResources(ctx, tc.appID); err != nil || len(resources) != 0 {
				t.Fatalf("install journal after the blocked install = %v (%v), want empty", resources, err)
			}
		})
	}
}

// A container left by an earlier attempt of the same app (its cleanup failed)
// carries the Store labels; the next attempt removes it instead of blocking.
func TestInstallRemovesTheStoreLeftoverOfAnEarlierAttempt(t *testing.T) {
	for i, tc := range []struct {
		appID     string
		container string
		labels    map[string]string
	}{
		{appID: "uptime-kuma", container: "aurago-store-uptime-kuma", labels: storeLabels("uptime-kuma", "")},
		{appID: "romm", container: "aurago-store-romm-db", labels: storeLabels("romm", "db")},
	} {
		t.Run(tc.appID, func(t *testing.T) {
			docker := &fakeDockerAdapter{traceLifecycle: true, existingContainers: map[string]map[string]string{tc.container: tc.labels}}
			secrets := &fakeSecretStore{data: map[string]string{}}
			svc := newTestServiceWithSecrets(t, docker, &fakeDesktopAdapter{}, &fakeLaunchpadAdapter{}, fixedPorts(19911+i), secrets)
			runStoreInstall(t, svc, tc.appID)
			assertEventOrder(t, docker.events, "remove:"+tc.container, "create:"+tc.container)
			if len(docker.removedVolumes) != 0 {
				t.Fatalf("leftover removal deleted volumes: %v", docker.removedVolumes)
			}
		})
	}
}

// The real retry case: the first attempt fails and its cleanup cannot remove
// the container; the retry is not blocked by that leftover.
func TestRetryIsNotBlockedByTheLeftoverOfAFailedAttempt(t *testing.T) {
	ctx := context.Background()
	docker := &fakeDockerAdapter{
		trackContainers: true,
		startErrors:     []error{errors.New("start failed")},
		removeErrors:    map[string]error{"aurago-store-uptime-kuma": errors.New("engine busy")},
	}
	svc := newTestService(t, docker, &fakeDesktopAdapter{}, &fakeLaunchpadAdapter{}, fixedPorts(19915))
	op, err := svc.StartInstall(ctx, InstallRequest{AppID: "uptime-kuma", BindMode: BindModeLocal})
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.RunOperation(ctx, op.ID); err == nil {
		t.Fatal("first attempt succeeded, want the start failure")
	}
	if got := docker.containerNames(); !reflect.DeepEqual(got, []string{"aurago-store-uptime-kuma"}) {
		t.Fatalf("containers after the failed cleanup = %v, want the leftover", got)
	}
	delete(docker.removeErrors, "aurago-store-uptime-kuma")
	runStoreInstall(t, svc, "uptime-kuma")
	if resources, err := svc.installResources(ctx, "uptime-kuma"); err != nil || len(resources) != 0 {
		t.Fatalf("install journal after the successful retry = %v (%v), want empty", resources, err)
	}
}

// Uninstall without "delete data" keeps volumes, generated secrets and the
// network-independent data; a failed reinstall must not remove them, and the
// next reinstall uses them again.
func TestFailedReinstallKeepsTheDataTheUninstallKept(t *testing.T) {
	ctx := context.Background()
	docker := &fakeDockerAdapter{trackContainers: true}
	secrets := &fakeSecretStore{data: map[string]string{}}
	svc := newTestServiceWithSecrets(t, docker, &fakeDesktopAdapter{}, &fakeLaunchpadAdapter{}, fixedPorts(17690), secrets)
	runStoreInstall(t, svc, "romm")
	installed, ok := docker.containerSpec("aurago-store-romm-db")
	if !ok {
		t.Fatal("install did not create the database")
	}
	password := envFromSpec(t, installed, "MYSQL_PASSWORD")
	app, _, err := svc.GetInstalled(ctx, "romm")
	if err != nil {
		t.Fatal(err)
	}
	retainedVolumes := append([]VolumeBinding(nil), app.Volumes...)
	for _, companion := range app.Companions {
		retainedVolumes = append(retainedVolumes, companion.Volumes...)
	}
	if len(retainedVolumes) == 0 {
		t.Fatal("romm has no volumes to retain")
	}
	if err := runStoreOperation(t, svc, "romm", OperationUninstall); err != nil {
		t.Fatalf("uninstall: %v", err)
	}
	retainedSecrets := map[string]string{}
	for key, value := range secrets.data {
		retainedSecrets[key] = value
	}
	if len(retainedSecrets) == 0 {
		t.Fatal("uninstall without delete_data kept no secrets")
	}

	docker.removedVolumes = nil
	docker.removedContainers = nil
	docker.removedNetworks = nil
	// Starts: the database, then the app fails.
	docker.startErrors = []error{nil, errors.New("start failed")}
	op, err := svc.StartInstall(ctx, InstallRequest{AppID: "romm", BindMode: BindModeLocal})
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.RunOperation(ctx, op.ID); err == nil {
		t.Fatal("reinstall succeeded, want the start failure")
	}
	if containsVolume(docker.removedVolumes, retainedVolumes) {
		t.Fatalf("failed reinstall removed retained volumes: %v", docker.removedVolumes)
	}
	if !reflect.DeepEqual(secrets.data, retainedSecrets) {
		t.Fatalf("failed reinstall changed the retained secrets: keys %v, want %v", mapKeys(secrets.data), mapKeys(retainedSecrets))
	}
	if docker.removedContainers["aurago-store-romm"] == 0 || docker.removedContainers["aurago-store-romm-db"] == 0 {
		t.Fatalf("failed reinstall left its own containers: %v", docker.removedContainers)
	}
	if !containsString(docker.removedNetworks, "aurago-store-romm-net") {
		t.Fatalf("failed reinstall left the network it created: %v", docker.removedNetworks)
	}

	runStoreInstall(t, svc, "romm")
	reinstalled, _ := docker.containerSpec("aurago-store-romm-db")
	if got := envFromSpec(t, reinstalled, "MYSQL_PASSWORD"); got != password {
		t.Fatal("the reinstall did not reuse the retained database password")
	}
}

func mapKeys(values map[string]string) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	return keys
}

func TestFailedReinstallKeepsTheWorkspaceFilesTheUninstallKept(t *testing.T) {
	ctx := context.Background()
	docker := &fakeDockerAdapter{}
	svc := newTestService(t, docker, &fakeDesktopAdapter{}, &fakeLaunchpadAdapter{}, fixedPorts(19920))
	runStoreInstall(t, svc, "olivetin")
	if err := runStoreOperation(t, svc, "olivetin", OperationUninstall); err != nil {
		t.Fatalf("uninstall: %v", err)
	}
	config := filepath.Join(svc.cfg.WorkspaceDir, "Shared", "OliveTin", "config.yaml")
	if err := os.WriteFile(config, []byte("actions:\n  - title: Mine\n"), 0o644); err != nil {
		t.Fatalf("edit the retained OliveTin config: %v", err)
	}
	docker.startErrors = []error{errors.New("start failed")}
	op, err := svc.StartInstall(ctx, InstallRequest{AppID: "olivetin", BindMode: BindModeLocal})
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.RunOperation(ctx, op.ID); err == nil {
		t.Fatal("reinstall succeeded, want the start failure")
	}
	if data, err := os.ReadFile(config); err != nil || string(data) != "actions:\n  - title: Mine\n" {
		t.Fatalf("failed reinstall changed the retained OliveTin config: %q (%v)", data, err)
	}
}

// A fresh install that fails removes everything it created, as before, but a
// network that existed before the attempt stays.
func TestFailedFreshInstallRemovesOnlyWhatItCreated(t *testing.T) {
	ctx := context.Background()
	docker := &fakeDockerAdapter{trackContainers: true, networks: map[string]bool{"aurago-store-romm-net": true}}
	secrets := &fakeSecretStore{data: map[string]string{}}
	svc := newTestServiceWithSecrets(t, docker, &fakeDesktopAdapter{}, &fakeLaunchpadAdapter{}, fixedPorts(17691), secrets)
	docker.startErrors = []error{nil, errors.New("start failed")}
	op, err := svc.StartInstall(ctx, InstallRequest{AppID: "romm", BindMode: BindModeLocal})
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.RunOperation(ctx, op.ID); err == nil {
		t.Fatal("install succeeded, want the start failure")
	}
	if got := docker.containerNames(); len(got) != 0 {
		t.Fatalf("containers after the failed install = %v, want none", got)
	}
	if len(docker.removedVolumes) == 0 {
		t.Fatal("failed install did not remove the volumes it created")
	}
	if containsString(docker.removedNetworks, "aurago-store-romm-net") || !docker.networks["aurago-store-romm-net"] {
		t.Fatalf("failed install removed the network that existed before: %v", docker.removedNetworks)
	}
	if len(secrets.data) != 0 {
		t.Fatalf("failed install kept the secrets it generated: %v", mapKeys(secrets.data))
	}
	if resources, err := svc.installResources(ctx, "romm"); err != nil || len(resources) != 0 {
		t.Fatalf("install journal after the cleanup = %v (%v), want empty", resources, err)
	}

	// A failed fresh workspace app removes the workspace directory it created.
	runFailedInstall := func(appID string, port int) *Service {
		other := &fakeDockerAdapter{startErrors: []error{errors.New("start failed")}}
		svc := newTestService(t, other, &fakeDesktopAdapter{}, &fakeLaunchpadAdapter{}, fixedPorts(port))
		op, err := svc.StartInstall(ctx, InstallRequest{AppID: appID, BindMode: BindModeLocal})
		if err != nil {
			t.Fatal(err)
		}
		if err := svc.RunOperation(ctx, op.ID); err == nil {
			t.Fatalf("%s install succeeded, want the start failure", appID)
		}
		return svc
	}
	olive := runFailedInstall("olivetin", 19921)
	if _, err := os.Stat(filepath.Join(olive.cfg.WorkspaceDir, "Shared", "OliveTin")); !os.IsNotExist(err) {
		t.Fatalf("failed fresh install kept the workspace directory it created: %v", err)
	}
}

// A cleanup that cannot remove a resource keeps its journal row, so the next
// failed attempt retries the removal instead of adopting the leftover.
func TestInstallCleanupKeepsRowsItCouldNotRemove(t *testing.T) {
	ctx := context.Background()
	docker := &fakeDockerAdapter{trackContainers: true, startErrors: []error{errors.New("start failed")}}
	svc := newTestService(t, docker, &fakeDesktopAdapter{}, &fakeLaunchpadAdapter{}, fixedPorts(19922))
	volume := resolveVolumes(svc.catalogByID["uptime-kuma"])[0].Name
	docker.removeVolumeErrors = map[string]error{volume: errors.New("volume is in use")}
	op, err := svc.StartInstall(ctx, InstallRequest{AppID: "uptime-kuma", BindMode: BindModeLocal})
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.RunOperation(ctx, op.ID); err == nil {
		t.Fatal("install succeeded, want the start failure")
	}
	resources, err := svc.installResources(ctx, "uptime-kuma")
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(resources, []installResource{{Kind: installResourceAttempt, Name: op.ID}, {Kind: installResourceVolume, Name: volume}}) &&
		!reflect.DeepEqual(resources, []installResource{{Kind: installResourceVolume, Name: volume}, {Kind: installResourceAttempt, Name: op.ID}}) {
		t.Fatalf("journal after the partial cleanup = %v, want the attempt and the volume", resources)
	}
	delete(docker.removeVolumeErrors, volume)
	docker.removedVolumes = nil
	docker.startErrors = []error{errors.New("start failed again")}
	second, err := svc.StartInstall(ctx, InstallRequest{AppID: "uptime-kuma", BindMode: BindModeLocal})
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.RunOperation(ctx, second.ID); err == nil {
		t.Fatal("second install succeeded, want the start failure")
	}
	if !containsString(docker.removedVolumes, volume) {
		t.Fatalf("the second cleanup did not retry the volume: %v", docker.removedVolumes)
	}
}

func TestInstallJournalIsClearedAfterSuccessAndOnUninstall(t *testing.T) {
	ctx := context.Background()
	svc := newTestService(t, &fakeDockerAdapter{}, &fakeDesktopAdapter{}, &fakeLaunchpadAdapter{}, fixedPorts(19923))
	runStoreInstall(t, svc, "uptime-kuma")
	if resources, err := svc.installResources(ctx, "uptime-kuma"); err != nil || len(resources) != 0 {
		t.Fatalf("journal after a successful install = %v (%v), want empty", resources, err)
	}
	if err := svc.recordInstallResource(ctx, "uptime-kuma", installResourceVolume, "stale"); err != nil {
		t.Fatal(err)
	}
	if err := runStoreOperation(t, svc, "uptime-kuma", OperationUninstall); err != nil {
		t.Fatalf("uninstall: %v", err)
	}
	if resources, err := svc.installResources(ctx, "uptime-kuma"); err != nil || len(resources) != 0 {
		t.Fatalf("journal after uninstall = %v (%v), want empty", resources, err)
	}
}

// An install interrupted by a restart is cleaned up from its journal: what the
// attempt created goes, a volume it found already existing stays.
func TestInterruptedInstallRecoveryRemovesOnlyJournaledResources(t *testing.T) {
	ctx := context.Background()
	dbPath := filepath.Join(t.TempDir(), "desktop_store.db")
	svc := newTestServiceAtPath(t, dbPath, &fakeDockerAdapter{}, &fakeDesktopAdapter{}, &fakeLaunchpadAdapter{}, fixedPorts(19924), nil)
	op, err := svc.StartInstall(ctx, InstallRequest{AppID: "node-red", BindMode: BindModeLocal})
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.updateOperation(ctx, op.ID, OperationRunning, "running", ""); err != nil {
		t.Fatal(err)
	}
	record := svc.buildInstallRecord(svc.catalogByID["node-red"], op, BindModeLocal, "127.0.0.1", 19924, false)
	if len(record.Volumes) == 0 {
		t.Fatal("node-red has no volume")
	}
	retained := record.Volumes[0].Name
	for _, item := range []installResource{
		{installResourceAttempt, op.ID},
		{installResourceContainer, record.ContainerName},
		{installResourceVolume, "created-by-the-attempt"},
	} {
		if err := svc.recordInstallResource(ctx, "node-red", item.Kind, item.Name); err != nil {
			t.Fatal(err)
		}
	}
	if err := svc.saveInstalled(ctx, record); err != nil {
		t.Fatal(err)
	}
	if err := svc.Close(); err != nil {
		t.Fatal(err)
	}

	// The container the interrupted attempt created carries its Store labels.
	recoveryDocker := &fakeDockerAdapter{existingContainers: map[string]map[string]string{record.ContainerName: storeLabels("node-red", "")}}
	recovered := newTestServiceAtPath(t, dbPath, recoveryDocker, &fakeDesktopAdapter{}, &fakeLaunchpadAdapter{}, fixedPorts(19925), nil)
	deadline := time.Now().Add(2 * time.Second)
	for {
		recoveryDocker.cleanupMu.Lock()
		done := containsString(recoveryDocker.removedVolumes, "created-by-the-attempt")
		removedRetained := containsString(recoveryDocker.removedVolumes, retained)
		removedContainer := recoveryDocker.removedContainers[record.ContainerName]
		recoveryDocker.cleanupMu.Unlock()
		if done {
			if removedRetained || removedContainer == 0 {
				t.Fatalf("recovery removed container=%d retained volume=%v, want the container only", removedContainer, removedRetained)
			}
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("recovery did not remove the journaled volume")
		}
		time.Sleep(5 * time.Millisecond)
	}
	if _, ok, err := recovered.GetInstalled(ctx, "node-red"); err != nil || ok {
		t.Fatalf("recovery kept the installing record: ok=%v err=%v", ok, err)
	}
	waitForJournal(t, recovered, "node-red", nil)
}

// waitForJournal waits until the app's journal holds exactly want (the
// interrupted-install recovery updates it in the background).
func waitForJournal(t *testing.T, svc *Service, appID string, want []installResource) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for {
		got, err := svc.installResources(context.Background(), appID)
		if err == nil && len(got) == len(want) && (len(want) == 0 || reflect.DeepEqual(sortedResources(got), sortedResources(want))) {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("journal of %s = %v (%v), want %v", appID, got, err, want)
		}
		time.Sleep(5 * time.Millisecond)
	}
}

func sortedResources(items []installResource) []installResource {
	out := append([]installResource(nil), items...)
	sort.Slice(out, func(i, j int) bool {
		if out[i].Kind != out[j].Kind {
			return out[i].Kind < out[j].Kind
		}
		return out[i].Name < out[j].Name
	})
	return out
}

// F-S5 review, minor 6: the recovery keeps the rows whose removal failed (and
// the attempt marker), so the next failed attempt's cleanup retries them.
func TestInterruptedInstallRecoveryKeepsRowsItCouldNotRemove(t *testing.T) {
	ctx := context.Background()
	dbPath := filepath.Join(t.TempDir(), "desktop_store.db")
	svc := newTestServiceAtPath(t, dbPath, &fakeDockerAdapter{}, &fakeDesktopAdapter{}, &fakeLaunchpadAdapter{}, fixedPorts(19927), nil)
	op, err := svc.StartInstall(ctx, InstallRequest{AppID: "node-red", BindMode: BindModeLocal})
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.updateOperation(ctx, op.ID, OperationRunning, "running", ""); err != nil {
		t.Fatal(err)
	}
	record := svc.buildInstallRecord(svc.catalogByID["node-red"], op, BindModeLocal, "127.0.0.1", 19927, false)
	for _, item := range []installResource{
		{installResourceAttempt, op.ID},
		{installResourceContainer, record.ContainerName},
		{installResourceVolume, "created-by-the-attempt"},
	} {
		if err := svc.recordInstallResource(ctx, "node-red", item.Kind, item.Name); err != nil {
			t.Fatal(err)
		}
	}
	if err := svc.saveInstalled(ctx, record); err != nil {
		t.Fatal(err)
	}
	if err := svc.Close(); err != nil {
		t.Fatal(err)
	}

	recoveryDocker := &fakeDockerAdapter{
		existingContainers: map[string]map[string]string{record.ContainerName: storeLabels("node-red", "")},
		removeErrors:       map[string]error{record.ContainerName: errors.New("engine busy")},
	}
	recovered := newTestServiceAtPath(t, dbPath, recoveryDocker, &fakeDesktopAdapter{}, &fakeLaunchpadAdapter{}, fixedPorts(19928), nil)
	waitForJournal(t, recovered, "node-red", []installResource{
		{installResourceAttempt, op.ID},
		{installResourceContainer, record.ContainerName},
	})
	if _, ok := recoveryDocker.existingContainers[record.ContainerName]; !ok {
		t.Fatal("the container whose removal failed is gone")
	}
}

// F-S5 review, important 2: the preflight journals a free name before the
// create; a foreign container that takes the name in between (here during
// the image pull) is never removed by the failed install's cleanup.
func TestFailedInstallNeverRemovesAForeignContainerThatTookTheName(t *testing.T) {
	ctx := context.Background()
	for i, tracked := range []bool{false, true} {
		docker := &fakeDockerAdapter{trackContainers: tracked}
		const name = "aurago-store-uptime-kuma"
		docker.pullHook = func(string) {
			// A foreign container appears after the preflight.
			if tracked {
				docker.addContainer(name)
				return
			}
			docker.existingContainers = map[string]map[string]string{name: {"com.example.owner": "someone"}}
			docker.createErr = errors.New("create container " + name + ": name already in use")
		}
		svc := newTestService(t, docker, &fakeDesktopAdapter{}, &fakeLaunchpadAdapter{}, fixedPorts(19930+i))
		op, err := svc.StartInstall(ctx, InstallRequest{AppID: "uptime-kuma", BindMode: BindModeLocal})
		if err != nil {
			t.Fatal(err)
		}
		if err := svc.RunOperation(ctx, op.ID); err == nil || !strings.Contains(err.Error(), "name already in use") {
			t.Fatalf("tracked=%v: install error = %v, want the name conflict of the create", tracked, err)
		}
		if docker.removedContainers[name] != 0 {
			t.Fatalf("tracked=%v: the failed install removed the foreign container: %v", tracked, docker.removedContainers)
		}
		if _, found, _ := docker.FindContainer(ctx, name); !found {
			t.Fatalf("tracked=%v: the foreign container is gone", tracked)
		}
		if len(docker.removedVolumes) == 0 {
			t.Fatalf("tracked=%v: the failed install kept the volume it created", tracked)
		}
		if resources, err := svc.installResources(ctx, "uptime-kuma"); err != nil || len(resources) != 0 {
			t.Fatalf("tracked=%v: journal = %v (%v), want empty", tracked, resources, err)
		}
	}
}

// A journaled container name whose lookup fails stays in the journal.
func TestInstallCleanupKeepsAContainerRowItCouldNotCheck(t *testing.T) {
	ctx := context.Background()
	docker := &fakeDockerAdapter{startErrors: []error{errors.New("start failed")}}
	svc := newTestService(t, docker, &fakeDesktopAdapter{}, &fakeLaunchpadAdapter{}, fixedPorts(19933))
	docker.findErrors = map[string]error{}
	docker.pullHook = func(string) {
		docker.findErrors["aurago-store-uptime-kuma"] = errors.New("engine busy")
	}
	op, err := svc.StartInstall(ctx, InstallRequest{AppID: "uptime-kuma", BindMode: BindModeLocal})
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.RunOperation(ctx, op.ID); err == nil {
		t.Fatal("install succeeded, want the start failure")
	}
	if docker.removedContainers["aurago-store-uptime-kuma"] != 0 {
		t.Fatal("removed a container whose lookup failed")
	}
	resources, err := svc.installResources(ctx, "uptime-kuma")
	if err != nil {
		t.Fatal(err)
	}
	want := []installResource{{installResourceAttempt, op.ID}, {installResourceContainer, "aurago-store-uptime-kuma"}}
	if !reflect.DeepEqual(sortedResources(resources), sortedResources(want)) {
		t.Fatalf("journal = %v, want %v", resources, want)
	}
}

func TestToolsDockerAdapterFindsContainersVolumesAndNetworks(t *testing.T) {
	tools.ConfigureRuntimePermissions(tools.RuntimePermissions{DockerEnabled: true})
	t.Cleanup(tools.ClearRuntimePermissionsForTest)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet && r.URL.Path == "/version" {
			w.Header().Set("Content-Type", "application/json")
			_, _ = io.WriteString(w, `{"ApiVersion":"1.45","MinAPIVersion":"1.25"}`)
			return
		}
		if r.Method != http.MethodGet {
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
		}
		switch r.URL.Path {
		case "/v1.45/containers/aurago-store-demo/json":
			w.Header().Set("Content-Type", "application/json")
			_, _ = io.WriteString(w, `{"Name":"/aurago-store-demo","Config":{"Labels":{"aurago.desktop_store":"true","aurago.desktop_store.app_id":"demo"}},"State":{"Running":true,"Status":"running"}}`)
		case "/v1.45/volumes/aurago_store_demo_data", "/v1.45/networks/aurago-store-demo-net":
			w.Header().Set("Content-Type", "application/json")
			_, _ = io.WriteString(w, `{}`)
		case "/v1.45/volumes/broken":
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = io.WriteString(w, `{"message":"engine says no"}`)
		default:
			w.WriteHeader(http.StatusNotFound)
			_, _ = io.WriteString(w, `{"message":"no such object"}`)
		}
	}))
	defer server.Close()
	adapter := NewToolsDockerAdapter("tcp://"+strings.TrimPrefix(server.URL, "http://"), "", nil)
	ctx := context.Background()

	state, found, err := adapter.FindContainer(ctx, "aurago-store-demo")
	if err != nil || !found || !isStoreLeftover(state.Labels, "demo", "") {
		t.Fatalf("FindContainer = %#v found=%v err=%v, want the labelled container", state, found, err)
	}
	if _, found, err := adapter.FindContainer(ctx, "aurago-store-missing"); err != nil || found {
		t.Fatalf("missing container: found=%v err=%v", found, err)
	}
	for name, want := range map[string]bool{"aurago_store_demo_data": true, "aurago_store_missing": false} {
		if got, err := adapter.VolumeExists(ctx, name); err != nil || got != want {
			t.Fatalf("VolumeExists(%s) = %v, %v; want %v", name, got, err, want)
		}
	}
	if _, err := adapter.VolumeExists(ctx, "broken"); err == nil || !strings.Contains(err.Error(), "engine says no") {
		t.Fatalf("VolumeExists(broken) error = %v", err)
	}
	for name, want := range map[string]bool{"aurago-store-demo-net": true, "aurago-store-missing-net": false} {
		if got, err := adapter.NetworkExists(ctx, name); err != nil || got != want {
			t.Fatalf("NetworkExists(%s) = %v, %v; want %v", name, got, err, want)
		}
	}
}

func TestStoreLeftoverNeedsTheExactAppAndCompanionLabels(t *testing.T) {
	for _, tc := range []struct {
		labels      map[string]string
		appID       string
		companionID string
		want        bool
	}{
		{storeLabels("romm", ""), "romm", "", true},
		{storeLabels("romm", "db"), "romm", "db", true},
		{storeLabels("romm", "db"), "romm", "", false},
		{storeLabels("romm", ""), "romm", "db", false},
		{storeLabels("excalidraw", ""), "romm", "", false},
		{map[string]string{"aurago.desktop_store.app_id": "romm"}, "romm", "", false},
		{nil, "romm", "", false},
	} {
		if got := isStoreLeftover(tc.labels, tc.appID, tc.companionID); got != tc.want {
			t.Fatalf("isStoreLeftover(%v, %q, %q) = %v, want %v", tc.labels, tc.appID, tc.companionID, got, tc.want)
		}
	}
}

// A name an earlier failed attempt journaled can since belong to a foreign
// container; the blocked install must not remove it.
func TestBlockedInstallNeverRemovesJournaledContainers(t *testing.T) {
	ctx := context.Background()
	docker := &fakeDockerAdapter{existingContainers: map[string]map[string]string{"aurago-store-uptime-kuma": nil}}
	svc := newTestService(t, docker, &fakeDesktopAdapter{}, &fakeLaunchpadAdapter{}, fixedPorts(19926))
	for _, item := range []installResource{{installResourceAttempt, "op-earlier"}, {installResourceContainer, "aurago-store-uptime-kuma"}} {
		if err := svc.recordInstallResource(ctx, "uptime-kuma", item.Kind, item.Name); err != nil {
			t.Fatal(err)
		}
	}
	op, err := svc.StartInstall(ctx, InstallRequest{AppID: "uptime-kuma", BindMode: BindModeLocal})
	if err != nil {
		t.Fatal(err)
	}
	var conflict *ContainerNameConflictError
	if err := svc.RunOperation(ctx, op.ID); !errors.As(err, &conflict) {
		t.Fatalf("install error = %v, want the name conflict", err)
	}
	if len(docker.removedContainers) != 0 || len(docker.stopped) != 0 {
		t.Fatalf("blocked install touched containers: removed=%v stopped=%v", docker.removedContainers, docker.stopped)
	}
}

// createPreJournalStoreDB writes a Store database with the schema before F-S5
// (no install journal, no operation error code columns) and some data.
func createPreJournalStoreDB(t *testing.T, dbPath string) {
	t.Helper()
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	for _, stmt := range []string{
		`CREATE TABLE desktop_store_apps (
			app_id TEXT PRIMARY KEY,
			desktop_app_id TEXT NOT NULL,
			launchpad_link_id TEXT,
			container_name TEXT NOT NULL,
			container_id TEXT,
			image TEXT NOT NULL,
			status TEXT NOT NULL,
			error TEXT NOT NULL DEFAULT '',
			bind_mode TEXT NOT NULL,
			host_ip TEXT NOT NULL,
			host_port INTEGER NOT NULL,
			container_port INTEGER NOT NULL,
			protocol TEXT NOT NULL,
			tailscale_enabled INTEGER NOT NULL DEFAULT 0,
			tailscale_status TEXT NOT NULL DEFAULT 'disabled',
			tailscale_port INTEGER NOT NULL DEFAULT 0,
			logo_path TEXT NOT NULL DEFAULT '',
			ports_json TEXT NOT NULL DEFAULT '[]',
			volumes_json TEXT NOT NULL DEFAULT '[]',
			host_binds_json TEXT NOT NULL DEFAULT '[]',
			env_json TEXT NOT NULL DEFAULT '[]',
			extra_hosts_json TEXT NOT NULL DEFAULT '[]',
			secret_refs_json TEXT NOT NULL DEFAULT '[]',
			companions_json TEXT NOT NULL DEFAULT '[]',
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL,
			last_operation_id TEXT NOT NULL DEFAULT '',
			last_operation_type TEXT NOT NULL DEFAULT '',
			last_operation_state TEXT NOT NULL DEFAULT ''
		)`,
		`CREATE TABLE desktop_store_operations (
			id TEXT PRIMARY KEY,
			type TEXT NOT NULL,
			app_id TEXT NOT NULL,
			status TEXT NOT NULL,
			message TEXT NOT NULL DEFAULT '',
			error TEXT NOT NULL DEFAULT '',
			request_json TEXT NOT NULL DEFAULT '{}',
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL,
			completed_at TEXT
		)`,
		`CREATE INDEX idx_desktop_store_operations_app ON desktop_store_operations(app_id, created_at)`,
		`INSERT INTO desktop_store_apps(app_id, desktop_app_id, launchpad_link_id, container_name, container_id, image, status,
			bind_mode, host_ip, host_port, container_port, protocol, volumes_json, created_at, updated_at,
			last_operation_id, last_operation_type, last_operation_state)
			VALUES('excalidraw', 'store-excalidraw', 'store-excalidraw', 'aurago-store-excalidraw', 'old-id', 'excalidraw/excalidraw:latest', 'running',
			'local', '127.0.0.1', 19941, 80, 'tcp', '[]', '2026-05-22T10:00:00Z', '2026-05-22T10:00:00Z', 'op-old-install', 'install', 'succeeded')`,
		`INSERT INTO desktop_store_operations(id, type, app_id, status, message, error, request_json, created_at, updated_at, completed_at)
			VALUES('op-old-install', 'install', 'excalidraw', 'succeeded', 'completed', '', '{}', '2026-05-22T10:00:00Z', '2026-05-22T10:01:00Z', '2026-05-22T10:01:00Z')`,
		`INSERT INTO desktop_store_operations(id, type, app_id, status, message, error, request_json, created_at, updated_at, completed_at)
			VALUES('op-old-failed', 'install', 'n8n', 'failed', '', 'pull image failed', '{}', '2026-05-23T10:00:00Z', '2026-05-23T10:01:00Z', '2026-05-23T10:01:00Z')`,
	} {
		if _, err := db.Exec(stmt); err != nil {
			t.Fatalf("pre-journal schema: %v", err)
		}
	}
}

func sqliteTableExists(t *testing.T, dbPath, table string) bool {
	t.Helper()
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	var count int
	if err := db.QueryRow(`SELECT count(*) FROM sqlite_master WHERE type = 'table' AND name = ?`, table).Scan(&count); err != nil {
		t.Fatal(err)
	}
	return count > 0
}

// F-S5 review, important 3: a Store database from before F-S5 migrates, keeps
// its data, gets a backup first, and the journaled install works on it.
func TestStoreMigrationFromThePreJournalSchema(t *testing.T) {
	ctx := context.Background()
	dbPath := filepath.Join(t.TempDir(), "desktop_store.db")
	createPreJournalStoreDB(t, dbPath)
	docker := &fakeDockerAdapter{trackContainers: true}
	svc := newTestServiceAtPath(t, dbPath, docker, &fakeDesktopAdapter{}, &fakeLaunchpadAdapter{}, fixedPorts(19942), nil)

	old, err := svc.Operation(ctx, "op-old-failed")
	if err != nil {
		t.Fatalf("old operation after the migration: %v", err)
	}
	if old.Status != OperationFailed || old.Error != "pull image failed" || old.ErrorCode != "" || old.ErrorParams != nil {
		t.Fatalf("old operation = %#v, want the failed row with an empty error code", old)
	}
	app, ok, err := svc.GetInstalled(ctx, "excalidraw")
	if err != nil || !ok || app.Status != AppStatusRunning || app.ContainerID != "old-id" {
		t.Fatalf("old app after the migration = %#v ok=%v err=%v", app, ok, err)
	}

	backup := dbPath + storeMigrationBackupSuffix
	if _, err := os.Stat(backup); err != nil {
		t.Fatalf("no backup before the migration: %v", err)
	}
	if sqliteTableExists(t, backup, "desktop_store_install_resources") || !sqliteTableExists(t, backup, "desktop_store_operations") {
		t.Fatal("the backup is not the pre-migration database")
	}

	// A journaled install works on the migrated database: a failure cleans up
	// what it created, a retry succeeds.
	docker.startErrors = []error{errors.New("start failed")}
	op, err := svc.StartInstall(ctx, InstallRequest{AppID: "uptime-kuma", BindMode: BindModeLocal})
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.RunOperation(ctx, op.ID); err == nil {
		t.Fatal("install succeeded, want the start failure")
	}
	if got := docker.containerNames(); len(got) != 0 || len(docker.removedVolumes) == 0 {
		t.Fatalf("after the failed install: containers %v, removed volumes %v", got, docker.removedVolumes)
	}
	runStoreInstall(t, svc, "uptime-kuma")
	if resources, err := svc.installResources(ctx, "uptime-kuma"); err != nil || len(resources) != 0 {
		t.Fatalf("journal after the install = %v (%v)", resources, err)
	}

	// An up-to-date database takes no new backup.
	if err := svc.Close(); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(backup); err != nil {
		t.Fatal(err)
	}
	newTestServiceAtPath(t, dbPath, docker, &fakeDesktopAdapter{}, &fakeLaunchpadAdapter{}, fixedPorts(19943), nil)
	if _, err := os.Stat(backup); !os.IsNotExist(err) {
		t.Fatalf("an up-to-date database wrote a backup: %v", err)
	}
}

func TestStoreMigrationBackupRules(t *testing.T) {
	t.Run("fresh database takes no backup", func(t *testing.T) {
		dbPath := filepath.Join(t.TempDir(), "desktop_store.db")
		newTestServiceAtPath(t, dbPath, &fakeDockerAdapter{}, &fakeDesktopAdapter{}, &fakeLaunchpadAdapter{}, fixedPorts(19944), nil)
		if _, err := os.Stat(dbPath + storeMigrationBackupSuffix); !os.IsNotExist(err) {
			t.Fatalf("fresh database wrote a backup: %v", err)
		}
	})
	t.Run("an existing backup is never overwritten", func(t *testing.T) {
		dbPath := filepath.Join(t.TempDir(), "desktop_store.db")
		createPreJournalStoreDB(t, dbPath)
		backup := dbPath + storeMigrationBackupSuffix
		if err := os.WriteFile(backup, []byte("earlier backup"), 0o600); err != nil {
			t.Fatal(err)
		}
		svc := newTestServiceAtPath(t, dbPath, &fakeDockerAdapter{}, &fakeDesktopAdapter{}, &fakeLaunchpadAdapter{}, fixedPorts(19945), nil)
		if data, err := os.ReadFile(backup); err != nil || string(data) != "earlier backup" {
			t.Fatalf("existing backup = %q (%v), want it unchanged", data, err)
		}
		if _, err := svc.Operation(context.Background(), "op-old-failed"); err != nil {
			t.Fatalf("migration did not run: %v", err)
		}
	})
	t.Run("a failed backup still migrates", func(t *testing.T) {
		dbPath := filepath.Join(t.TempDir(), "desktop_store.db")
		createPreJournalStoreDB(t, dbPath)
		original := storeMigrationBackup
		t.Cleanup(func() { storeMigrationBackup = original })
		calls := 0
		storeMigrationBackup = func(context.Context, *sql.DB, string) error {
			calls++
			return errors.New("disk full")
		}
		svc := newTestServiceAtPath(t, dbPath, &fakeDockerAdapter{}, &fakeDesktopAdapter{}, &fakeLaunchpadAdapter{}, fixedPorts(19946), nil)
		if calls != 1 {
			t.Fatalf("backup attempts = %d, want 1", calls)
		}
		if op, err := svc.Operation(context.Background(), "op-old-failed"); err != nil || op.ErrorCode != "" {
			t.Fatalf("migration did not run: %#v (%v)", op, err)
		}
		if !sqliteTableExists(t, dbPath, "desktop_store_install_resources") {
			t.Fatal("the install journal table is missing")
		}
	})
}

// F-S5 review, minor 5: a poll must never see a failed operation without its
// error code. A trigger records every row state in which the status is
// already failed but the code of a blocked install is still missing.
func TestFailedOperationStatusAndErrorCodeAreWrittenTogether(t *testing.T) {
	ctx := context.Background()
	docker := &fakeDockerAdapter{existingContainers: map[string]map[string]string{"aurago-store-uptime-kuma": nil}}
	svc := newTestService(t, docker, &fakeDesktopAdapter{}, &fakeLaunchpadAdapter{}, fixedPorts(19950))
	for _, stmt := range []string{
		`CREATE TABLE fs5_failed_without_code (id TEXT)`,
		`CREATE TRIGGER fs5_failed_without_code_probe AFTER UPDATE ON desktop_store_operations
			WHEN NEW.status = 'failed' AND NEW.error_code = '' AND NEW.error LIKE '%already exists%'
			BEGIN INSERT INTO fs5_failed_without_code(id) VALUES (NEW.id); END`,
	} {
		if _, err := svc.db.ExecContext(ctx, stmt); err != nil {
			t.Fatal(err)
		}
	}
	op, err := svc.StartInstall(ctx, InstallRequest{AppID: "uptime-kuma", BindMode: BindModeLocal})
	if err != nil {
		t.Fatal(err)
	}
	var conflict *ContainerNameConflictError
	if err := svc.RunOperation(ctx, op.ID); !errors.As(err, &conflict) {
		t.Fatalf("install error = %v, want the name conflict", err)
	}
	var seen int
	if err := svc.db.QueryRowContext(ctx, `SELECT count(*) FROM fs5_failed_without_code`).Scan(&seen); err != nil {
		t.Fatal(err)
	}
	if seen != 0 {
		t.Fatalf("the operation was visible %d time(s) as failed without its error code", seen)
	}
	stored, err := svc.Operation(ctx, op.ID)
	if err != nil || stored.Status != OperationFailed || stored.ErrorCode != OperationErrorContainerNameInUse || stored.CompletedAt == nil {
		t.Fatalf("operation = %#v (%v)", stored, err)
	}
	// An error without a code stores an empty code.
	docker.existingContainers = nil
	docker.startErrors = []error{errors.New("start failed")}
	second, err := svc.StartInstall(ctx, InstallRequest{AppID: "uptime-kuma", BindMode: BindModeLocal})
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.RunOperation(ctx, second.ID); err == nil {
		t.Fatal("install succeeded, want the start failure")
	}
	if stored, err := svc.Operation(ctx, second.ID); err != nil || stored.Status != OperationFailed || stored.ErrorCode != "" || stored.Error == "" {
		t.Fatalf("operation = %#v (%v), want failed without a code", stored, err)
	}
}
