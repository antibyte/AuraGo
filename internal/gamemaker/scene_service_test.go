package gamemaker

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

type sceneBlockingRunner struct {
	service *Service
	started chan struct{}
}

type nullSceneRunner struct {
	service *Service
	started chan struct{}
}

func (r nullSceneRunner) RunGameMakerJob(ctx context.Context, run JobRun) error {
	if run.Stage == "planning" {
		return r.service.SetPlan(ctx, run.Job.ID, ExampleGamePlan(run.Project))
	}
	if run.Stage != "building" {
		return nil
	}
	stage, err := r.service.JobDirectory(run.Job.ID)
	if err != nil {
		return err
	}
	path := filepath.Join(stage, filepath.FromSlash(SceneFilePath))
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return err
	}
	if err := os.WriteFile(path, []byte("null"), 0o640); err != nil {
		return err
	}
	close(r.started)
	<-ctx.Done()
	return ctx.Err()
}

func (r sceneBlockingRunner) RunGameMakerJob(ctx context.Context, run JobRun) error {
	if run.Stage == "planning" {
		return r.service.SetPlan(ctx, run.Job.ID, ExampleGamePlan(run.Project))
	}
	if run.Stage != "building" {
		return nil
	}
	data, err := MarshalSceneJSON(sceneFixture())
	if err != nil {
		return err
	}
	stage, err := r.service.JobDirectory(run.Job.ID)
	if err != nil {
		return err
	}
	path := filepath.Join(stage, filepath.FromSlash(SceneFilePath))
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return err
	}
	if err := os.WriteFile(path, data, 0o640); err != nil {
		return err
	}
	close(r.started)
	<-ctx.Done()
	return ctx.Err()
}

func TestSceneServiceUsesHashConditionalAtomicMutations(t *testing.T) {
	service := newTestService(t)
	project := createTestProject(t, service, "2d")
	started := make(chan struct{})
	service.SetRunner(sceneBlockingRunner{service: service, started: started})
	job, err := service.StartJob(context.Background(), project.ID, StartJobRequest{})
	if err != nil {
		t.Fatal(err)
	}
	select {
	case <-started:
	case <-time.After(5 * time.Second):
		t.Fatal("scene test job did not reach its active building stage")
	}
	inspected, err := service.InspectScene(context.Background(), job.ID)
	if err != nil {
		t.Fatal(err)
	}
	data, err := MarshalSceneJSON(inspected.Scene)
	if err != nil {
		t.Fatal(err)
	}
	patch, _ := json.Marshal(ScenePatch{Nodes: []SceneNode{{ID: "added", Kind: "decorative", Position: Vec3{10, 10, 0}}}})
	if _, err := service.PatchSceneJSON(context.Background(), job.ID, inspected.SHA256, append(patch, []byte(" {}")...)); err == nil {
		t.Fatal("trailing patch JSON was accepted")
	}
	if _, err := service.PatchSceneJSON(context.Background(), job.ID, inspected.SHA256, patch); err != nil {
		t.Fatalf("conditional patch: %v", err)
	}
	patched, err := service.InspectScene(context.Background(), job.ID)
	if err != nil {
		t.Fatal(err)
	}
	if patched.SHA256 == "" || patched.CurrentSHA256 != patched.SHA256 || patched.ProposedSHA256 != "" {
		t.Fatalf("inspect hashes = current %q proposed %q sha %q", patched.CurrentSHA256, patched.ProposedSHA256, patched.SHA256)
	}
	patchedData, err := MarshalSceneJSON(patched.Scene)
	if err != nil {
		t.Fatal(err)
	}
	dry, err := service.SetSceneJSON(context.Background(), job.ID, patched.SHA256, patchedData, true)
	if err != nil {
		t.Fatalf("dry-run set: %v", err)
	}
	if dry.SHA256 != patched.SHA256 || dry.CurrentSHA256 != patched.SHA256 || dry.ProposedSHA256 == "" {
		t.Fatalf("dry-run hashes = current %q proposed %q sha %q", dry.CurrentSHA256, dry.ProposedSHA256, dry.SHA256)
	}
	_, staleErr := service.SetSceneJSON(context.Background(), job.ID, inspected.SHA256, data, true)
	if staleErr == nil {
		t.Fatal("stale hash was accepted after patch")
	}
	if !errors.Is(staleErr, ErrSceneConflict) {
		t.Fatalf("stale hash error = %v", staleErr)
	}
	stage, err := service.JobDirectory(job.ID)
	if err != nil {
		t.Fatal(err)
	}
	corrupt := []byte("{invalid scene")
	if err := os.WriteFile(filepath.Join(stage, filepath.FromSlash(SceneFilePath)), corrupt, 0600); err != nil {
		t.Fatal(err)
	}
	broken, err := service.InspectScene(context.Background(), job.ID)
	if err == nil || broken.SHA256 != sceneHash(corrupt) {
		t.Fatal("invalid scene inspection must retain repair precondition", err)
	}
	repaired, err := service.SetSceneJSON(context.Background(), job.ID, broken.SHA256, patchedData)
	if err != nil || !repaired.Written {
		t.Fatal("hash-guarded full scene repair", err)
	}
	if repaired.Changes == nil || repaired.Changes.Counts["added"] == 0 {
		t.Fatal("repair omitted concrete change summary")
	}

	if err := service.CancelJob(context.Background(), job.ID); err != nil {
		t.Fatal(err)
	}
}

func TestSceneServiceTreatsLegacyNullAsCreateOnly(t *testing.T) {
	service := newTestService(t)
	project := createTestProject(t, service, "2d")
	started := make(chan struct{})
	service.SetRunner(nullSceneRunner{service: service, started: started})
	job, err := service.StartJob(context.Background(), project.ID, StartJobRequest{})
	if err != nil {
		t.Fatal(err)
	}
	select {
	case <-started:
	case <-time.After(5 * time.Second):
		t.Fatal("null scene test job did not reach its active building stage")
	}
	inspected, err := service.InspectScene(context.Background(), job.ID)
	if err != nil {
		t.Fatal(err)
	}
	if inspected.Exists || inspected.SHA256 != "" || inspected.CurrentSHA256 != "" {
		t.Fatalf("legacy null should be absent: exists=%v sha=%q current=%q", inspected.Exists, inspected.SHA256, inspected.CurrentSHA256)
	}
	data, err := MarshalSceneJSON(sceneFixture())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.SetSceneJSON(context.Background(), job.ID, inspected.SHA256, data); err != nil {
		t.Fatalf("create-only set after null inspect: %v", err)
	}
	if err := service.CancelJob(context.Background(), job.ID); err != nil {
		t.Fatal(err)
	}
}
