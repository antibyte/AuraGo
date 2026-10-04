package deployer

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"aurago/internal/dockerutil"
)

func TestDisabledStartupPreservesJournalAndExplicitRecoveryNeverStarts(t *testing.T) {
	manifest := validManifest()
	fake := newFakeDocker(t, manifest)
	fake.addContainer(&fakeContainer{ID: "backup", Name: "aurago-speech-lab-gateway-rollback", Image: manifest.Images.Gateway,
		Labels: dockerutil.ManagedLabels(OwnerLabel, "speech-lab", "gateway", "old"), Running: true})
	cfg := managedSpeechLabConfig(fake.server.URL)
	cfg.Enabled = false
	cfg.Deployment.AutoStart, cfg.Deployment.AutoUpdate = true, true
	dir := t.TempDir()
	manager := fake.manager(t, cfg, dir)
	manager.state = State{SchemaVersion: 2, State: "error", ContainerIDs: []string{"backup"}, Transaction: &DeploymentTransaction{
		ID: "tx", Phase: "rollback_pending", PreviousState: "ready", PreviousContainerIDs: []string{"backup"},
		Backups: []ContainerBackup{{ID: "backup", StableName: "aurago-speech-lab-gateway", WasRunning: true}},
	}}
	if err := manager.persist(); err != nil {
		t.Fatal(err)
	}
	manager = fake.manager(t, cfg, dir)
	if err := manager.AutoStart(context.Background()); err != nil {
		t.Fatal(err)
	}
	if !manager.PublicStatus().RecoveryPending || fake.find("backup").Name != "aurago-speech-lab-gateway-rollback" {
		t.Fatal("disabled startup consumed recovery journal")
	}
	if err := manager.recoverTransaction(context.Background(), manager.operationSnapshot()); err != nil {
		t.Fatal(err)
	}
	if state := manager.Status(); state.Transaction != nil || state.State != "stopped" || fake.find("backup").Running {
		t.Fatalf("disabled cleanup reactivated stack or failed to finish: %#v", state)
	}
	if err := manager.containerAction(context.Background(), manager.operationSnapshot(), http.MethodPost, "/containers/backup/start"); err == nil {
		t.Fatal("disabled container start accepted")
	}
}

func TestRecoveryCancelsInFlightDockerRequestAndRetainsJournal(t *testing.T) {
	started := make(chan struct{}, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case started <- struct{}{}:
		default:
		}
		<-r.Context().Done()
	}))
	defer server.Close()
	fake := newFakeDocker(t, validManifest())
	manager := fake.manager(t, managedSpeechLabConfig(server.URL), "")
	manager.docker = dockerutil.NewClient("tcp://"+strings.TrimPrefix(server.URL, "http://"), time.Minute)
	manager.state.Transaction = &DeploymentTransaction{ID: "tx", Phase: "rollback_pending", NewContainerIDs: []string{"new"}}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- manager.recoverTransaction(ctx, manager.operationSnapshot()) }()
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("recovery did not start")
	}
	cancel()
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("recovery error = %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("recovery ignored cancellation")
	}
	if manager.Status().Transaction == nil {
		t.Fatal("cancelled recovery discarded journal")
	}
}
