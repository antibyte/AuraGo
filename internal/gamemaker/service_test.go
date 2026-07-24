package gamemaker

import (
	"archive/zip"
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

type testRunner struct {
	service *Service
	mutate  func(context.Context, JobRun) error
	block   bool
}

type repairRunner struct {
	service  *Service
	attempts int
	original string
}

func (r *repairRunner) RunGameMakerJob(ctx context.Context, run JobRun) error {
	r.attempts++
	if r.original == "" {
		original, err := r.service.ReadJobFile(ctx, run.Job.ID, "src/main.ts")
		if err != nil {
			return err
		}
		r.original = original
	}
	content := "this is not valid TypeScript <<<"
	if r.attempts > 1 {
		content = r.original + "\n// repaired by validation retry\n"
	}
	if err := r.service.WriteJobFile(ctx, run.Job.ID, "src/main.ts", content); err != nil {
		return err
	}
	if r.attempts > 1 {
		r.service.mu.RLock()
		previewJobID := r.service.previewJobs[run.Project.ID]
		r.service.mu.RUnlock()
		if previewJobID != run.Job.ID {
			return fmt.Errorf("automatic build did not publish staging preview")
		}
	}
	return nil
}

func (r testRunner) RunGameMakerJob(ctx context.Context, run JobRun) error {
	if r.block {
		<-ctx.Done()
		return ctx.Err()
	}
	if r.mutate != nil {
		return r.mutate(ctx, run)
	}
	result := r.service.BuildJob(ctx, run.Job.ID)
	if !result.OK {
		return errors.New(diagnosticsText(result.Diagnostics))
	}
	return nil
}

func newTestService(t *testing.T) *Service {
	t.Helper()
	root := t.TempDir()
	service, err := NewService(Options{
		DBPath:               filepath.Join(root, "data", "game_maker.db"),
		WorkspacePath:        filepath.Join(root, "workspace"),
		Enabled:              true,
		AllowCreate:          true,
		AllowEdit:            true,
		AllowDelete:          true,
		AllowMediaGeneration: true,
		MaxProjects:          10,
		MaxFilesPerProject:   100,
		MaxFileBytes:         2 * 1024 * 1024,
		MaxProjectBytes:      20 * 1024 * 1024,
		JobTimeout:           5 * time.Second,
	})
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}
	service.SetSkillStatus(curatedSkills, true)
	t.Cleanup(func() { _ = service.Close() })
	return service
}

func createTestProject(t *testing.T, service *Service, dimension string) Project {
	t.Helper()
	project, err := service.CreateProject(context.Background(), CreateProjectRequest{
		Name:               "Orbit Garden " + dimension,
		Dimension:          dimension,
		Description:        "Collect stars, avoid hazards, and keep a visible score.",
		ProviderID:         "test",
		Model:              "mock-model",
		UseImageGeneration: true,
		UseMusicGeneration: true,
	})
	if err != nil {
		t.Fatalf("CreateProject: %v", err)
	}
	return project
}

func waitJob(t *testing.T, service *Service, id string) Job {
	t.Helper()
	deadline := time.Now().Add(8 * time.Second)
	for time.Now().Before(deadline) {
		job, err := service.GetJob(context.Background(), id)
		if err == nil && !activeJobStatus(job.Status) {
			return job
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("job %s did not finish", id)
	return Job{}
}

func activeJobStatus(status string) bool {
	switch status {
	case "queued", "planning", "building", "validating", "polishing":
		return true
	default:
		return false
	}
}

func TestSafeRelativePathRejectsCrossPlatformEscapes(t *testing.T) {
	t.Parallel()
	invalid := []string{
		"../escape.ts", `..\escape.ts`, "/etc/passwd", `C:\Windows\win.ini`,
		`C:relative.txt`, `\\server\share\file`, "//server/share/file", "src/\x00bad.ts",
		"vendor/runtime.js", "dist/game.js", ".game-maker/token",
	}
	for _, candidate := range invalid {
		candidate := candidate
		t.Run(strings.ReplaceAll(candidate, "/", "_"), func(t *testing.T) {
			if _, err := safeRelativePath(candidate, false); !errors.Is(err, ErrInvalidPath) {
				t.Fatalf("safeRelativePath(%q) error = %v, want ErrInvalidPath", candidate, err)
			}
		})
	}
	for _, candidate := range []string{"src/main.ts", "assets/hero.png", "game.json", "scenes/menu.ts"} {
		if got, err := safeRelativePath(candidate, false); err != nil || got != candidate {
			t.Fatalf("safeRelativePath(%q) = %q, %v", candidate, got, err)
		}
	}
}

func TestGameMakerPublishes2DAnd3DOfflineExports(t *testing.T) {
	for _, dimension := range []string{"2d", "3d"} {
		t.Run(dimension, func(t *testing.T) {
			service := newTestService(t)
			service.SetRunner(testRunner{service: service})
			project := createTestProject(t, service, dimension)
			job, err := service.StartJob(context.Background(), project.ID, StartJobRequest{})
			if err != nil {
				t.Fatalf("StartJob: %v", err)
			}
			job = waitJob(t, service, job.ID)
			if job.Status != "ready" || job.ResultRevision != 1 {
				t.Fatalf("job = %+v", job)
			}
			published, err := service.GetProject(context.Background(), project.ID)
			if err != nil {
				t.Fatal(err)
			}
			if published.ProjectKey != "Games/"+published.Slug || filepath.IsAbs(published.ProjectKey) {
				t.Fatalf("unsafe public project key %q", published.ProjectKey)
			}
			if dimension == "3d" {
				corePath := filepath.Join(service.opts.WorkspacePath, filepath.FromSlash(published.ProjectKey), "vendor", "three.core.min.js")
				if err := os.Remove(corePath); err != nil {
					t.Fatalf("remove core runtime to simulate legacy revision: %v", err)
				}
				grant, err := service.CreatePreviewGrant(project.ID)
				if err != nil {
					t.Fatalf("CreatePreviewGrant: %v", err)
				}
				core, contentType, err := service.PreviewFile(grant.Token, "vendor/three.core.min.js")
				if err != nil {
					t.Fatalf("legacy runtime preview fallback: %v", err)
				}
				if len(core) == 0 || !strings.Contains(contentType, "javascript") {
					t.Fatalf("legacy runtime preview fallback = %d bytes, %q", len(core), contentType)
				}
			}
			var output bytes.Buffer
			name, err := service.WriteExport(context.Background(), project.ID, &output)
			if err != nil {
				t.Fatalf("WriteExport: %v", err)
			}
			if name != published.Slug+".zip" {
				t.Fatalf("export name = %q", name)
			}
			reader, err := zip.NewReader(bytes.NewReader(output.Bytes()), int64(output.Len()))
			if err != nil {
				t.Fatalf("open export: %v", err)
			}
			entries := map[string]bool{}
			for _, file := range reader.File {
				entries[file.Name] = true
				if strings.Contains(file.Name, ".game-maker") || strings.Contains(file.Name, "token") {
					t.Fatalf("private metadata exported as %q", file.Name)
				}
			}
			runtimeNames := []string{"vendor/phaser-" + PhaserVersion + ".min.js"}
			if dimension == "3d" {
				runtimeNames = []string{
					"vendor/three-" + ThreeVersion + ".module.min.js",
					"vendor/three.core.min.js",
				}
			}
			requiredEntries := []string{"game.json", "index.html", "src/main.ts", "dist/game.js", "THIRD_PARTY_NOTICES.md"}
			requiredEntries = append(requiredEntries, runtimeNames...)
			for _, required := range requiredEntries {
				if !entries[required] {
					t.Errorf("export missing %s", required)
				}
			}
		})
	}
}

func TestUpdatePolicyAppliesPermissionChangesWithoutRestart(t *testing.T) {
	service := newTestService(t)
	service.UpdatePolicy(Policy{})
	if _, err := service.CreateProject(context.Background(), CreateProjectRequest{
		Name: "Disabled", Dimension: "2d", Description: "Must stay disabled.",
	}); !errors.Is(err, ErrDisabled) {
		t.Fatalf("CreateProject with disabled policy error = %v, want ErrDisabled", err)
	}

	service.UpdatePolicy(Policy{Enabled: true, ReadOnly: true, AllowCreate: true})
	if _, err := service.CreateProject(context.Background(), CreateProjectRequest{
		Name: "Read only", Dimension: "2d", Description: "Must stay read only.",
	}); !errors.Is(err, ErrReadOnly) {
		t.Fatalf("CreateProject with read-only policy error = %v, want ErrReadOnly", err)
	}

	service.UpdatePolicy(Policy{Enabled: true, AllowCreate: true})
	if _, err := service.CreateProject(context.Background(), CreateProjectRequest{
		Name: "Live policy", Dimension: "2d", Description: "Permission update works.",
	}); err != nil {
		t.Fatalf("CreateProject after live policy update: %v", err)
	}
}

func TestValidationRetriesRepairBeforePublishing(t *testing.T) {
	service := newTestService(t)
	runner := &repairRunner{service: service}
	service.SetRunner(runner)
	project := createTestProject(t, service, "2d")
	job, err := service.StartJob(context.Background(), project.ID, StartJobRequest{})
	if err != nil {
		t.Fatal(err)
	}
	finished := waitJob(t, service, job.ID)
	if finished.Status != "ready" || finished.ResultRevision != 1 {
		t.Fatalf("repaired job = %+v", finished)
	}
	if runner.attempts != 2 {
		t.Fatalf("runner attempts = %d, want initial build plus one repair", runner.attempts)
	}
}

func TestFailedEditKeepsLastPlayableRevisionAndRestoreDeduplicatesBlobs(t *testing.T) {
	service := newTestService(t)
	service.SetRunner(testRunner{service: service})
	project := createTestProject(t, service, "2d")
	first, err := service.StartJob(context.Background(), project.ID, StartJobRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if job := waitJob(t, service, first.ID); job.Status != "ready" {
		t.Fatalf("first job = %+v", job)
	}
	project, _ = service.GetProject(context.Background(), project.ID)
	publishedMain := filepath.Join(service.opts.WorkspacePath, filepath.FromSlash(project.ProjectKey), "src", "main.ts")
	original, err := os.ReadFile(publishedMain)
	if err != nil {
		t.Fatal(err)
	}

	attempts := 0
	service.SetRunner(testRunner{service: service, mutate: func(ctx context.Context, run JobRun) error {
		attempts++
		return service.WriteJobFile(ctx, run.Job.ID, "src/main.ts", "this is not valid TypeScript <<<")
	}})
	failed, err := service.StartJob(context.Background(), project.ID, StartJobRequest{Prompt: "Break the build"})
	if err != nil {
		t.Fatal(err)
	}
	if job := waitJob(t, service, failed.ID); job.Status != "failed" {
		t.Fatalf("failed job = %+v", job)
	}
	afterFailure, err := os.ReadFile(publishedMain)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(afterFailure, original) {
		t.Fatal("failed staging build changed the published game")
	}
	project, _ = service.GetProject(context.Background(), project.ID)
	if project.CurrentRevision != 1 {
		t.Fatalf("current revision after failed build = %d", project.CurrentRevision)
	}
	if attempts != 4 {
		t.Fatalf("agent attempts = %d, want initial attempt plus three repairs", attempts)
	}

	restored, err := service.RestoreRevision(context.Background(), project.ID, 1)
	if err != nil {
		t.Fatalf("RestoreRevision: %v", err)
	}
	if restored.Number != 2 || restored.Parent != 1 {
		t.Fatalf("restored revision = %+v", restored)
	}
	var blobCount, revisionFileCount int
	if err := service.db.QueryRow(`SELECT COUNT(*) FROM gm_blobs`).Scan(&blobCount); err != nil {
		t.Fatal(err)
	}
	if err := service.db.QueryRow(`SELECT COUNT(*) FROM gm_revision_files WHERE revision_id=?`, restored.ID).Scan(&revisionFileCount); err != nil {
		t.Fatal(err)
	}
	if blobCount != revisionFileCount {
		t.Fatalf("deduplicated blobs = %d, restored files = %d", blobCount, revisionFileCount)
	}
}

func TestCancelLeavesPublishedRevisionUntouched(t *testing.T) {
	service := newTestService(t)
	service.SetRunner(testRunner{service: service})
	project := createTestProject(t, service, "3d")
	first, _ := service.StartJob(context.Background(), project.ID, StartJobRequest{})
	if job := waitJob(t, service, first.ID); job.Status != "ready" {
		t.Fatalf("first job = %+v", job)
	}
	project, _ = service.GetProject(context.Background(), project.ID)
	service.SetRunner(testRunner{service: service, block: true})
	job, err := service.StartJob(context.Background(), project.ID, StartJobRequest{Prompt: "Wait forever"})
	if err != nil {
		t.Fatal(err)
	}
	if err := service.CancelJob(context.Background(), job.ID); err != nil {
		t.Fatal(err)
	}
	if job = waitJob(t, service, job.ID); job.Status != "cancelled" {
		t.Fatalf("cancelled job = %+v", job)
	}
	after, _ := service.GetProject(context.Background(), project.ID)
	if after.CurrentRevision != project.CurrentRevision {
		t.Fatalf("cancel changed revision from %d to %d", project.CurrentRevision, after.CurrentRevision)
	}
}

func TestRestoreAndDeleteRespectGlobalWriterReservation(t *testing.T) {
	service := newTestService(t)
	service.SetRunner(testRunner{service: service})
	project := createTestProject(t, service, "2d")
	initial, err := service.StartJob(context.Background(), project.ID, StartJobRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if finished := waitJob(t, service, initial.ID); finished.Status != "ready" {
		t.Fatalf("initial job = %+v", finished)
	}

	service.SetRunner(testRunner{service: service, block: true})
	active, err := service.StartJob(context.Background(), project.ID, StartJobRequest{Prompt: "Keep the writer busy"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.RestoreRevision(context.Background(), project.ID, 1); !errors.Is(err, ErrBusy) {
		t.Fatalf("RestoreRevision while job active error = %v, want ErrBusy", err)
	}
	if err := service.DeleteProject(context.Background(), project.ID); !errors.Is(err, ErrBusy) {
		t.Fatalf("DeleteProject while job active error = %v, want ErrBusy", err)
	}
	if err := service.CancelJob(context.Background(), active.ID); err != nil {
		t.Fatal(err)
	}
	if finished := waitJob(t, service, active.ID); finished.Status != "cancelled" {
		t.Fatalf("active job after cancellation = %+v", finished)
	}
}

func TestDeleteProjectRestoresFilesWhenLedgerDeleteFails(t *testing.T) {
	service := newTestService(t)
	service.SetRunner(testRunner{service: service})
	project := createTestProject(t, service, "2d")
	job, err := service.StartJob(context.Background(), project.ID, StartJobRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if finished := waitJob(t, service, job.ID); finished.Status != "ready" {
		t.Fatalf("job = %+v", finished)
	}
	project, err = service.GetProject(context.Background(), project.ID)
	if err != nil {
		t.Fatal(err)
	}
	projectDir := filepath.Join(service.opts.WorkspacePath, filepath.FromSlash(project.ProjectKey))
	if _, err := service.db.Exec(`CREATE TRIGGER gm_block_project_delete
		BEFORE DELETE ON gm_projects BEGIN SELECT RAISE(ABORT, 'blocked delete'); END`); err != nil {
		t.Fatal(err)
	}
	if err := service.DeleteProject(context.Background(), project.ID); err == nil {
		t.Fatal("DeleteProject succeeded despite blocking database trigger")
	}
	if _, err := os.Stat(filepath.Join(projectDir, "index.html")); err != nil {
		t.Fatalf("published files were not restored after failed ledger delete: %v", err)
	}
	if _, err := service.GetProject(context.Background(), project.ID); err != nil {
		t.Fatalf("project ledger row disappeared after failed delete: %v", err)
	}
}

func TestDeleteProjectPrunesOnlyUnreferencedBlobs(t *testing.T) {
	service := newTestService(t)
	service.SetRunner(testRunner{service: service})
	var projects []Project
	for i := 0; i < 2; i++ {
		project := createTestProject(t, service, "2d")
		job, err := service.StartJob(context.Background(), project.ID, StartJobRequest{})
		if err != nil {
			t.Fatal(err)
		}
		if finished := waitJob(t, service, job.ID); finished.Status != "ready" {
			t.Fatalf("job = %+v", finished)
		}
		projects = append(projects, project)
	}
	var initial int
	if err := service.db.QueryRow(`SELECT COUNT(*) FROM gm_blobs`).Scan(&initial); err != nil {
		t.Fatal(err)
	}
	if initial == 0 {
		t.Fatal("published projects did not create revision blobs")
	}
	if err := service.DeleteProject(context.Background(), projects[0].ID); err != nil {
		t.Fatal(err)
	}
	var remaining int
	if err := service.db.QueryRow(`SELECT COUNT(*) FROM gm_blobs`).Scan(&remaining); err != nil {
		t.Fatal(err)
	}
	if remaining == 0 {
		t.Fatal("deleting one project removed blobs still referenced by another project")
	}
	if err := service.DeleteProject(context.Background(), projects[1].ID); err != nil {
		t.Fatal(err)
	}
	if err := service.db.QueryRow(`SELECT COUNT(*) FROM gm_blobs`).Scan(&remaining); err != nil {
		t.Fatal(err)
	}
	if remaining != 0 {
		t.Fatalf("orphaned blob records after deleting all projects = %d", remaining)
	}
	orphanData := []byte("unregistered snapshot blob")
	orphanHash := sha256Bytes(orphanData)
	orphanPath := filepath.Join(service.blobDir, orphanHash[:2], orphanHash)
	if err := os.MkdirAll(filepath.Dir(orphanPath), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(orphanPath, orphanData, 0o640); err != nil {
		t.Fatal(err)
	}
	if err := service.pruneOrphanBlobs(context.Background()); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(orphanPath); !os.IsNotExist(err) {
		t.Fatalf("unregistered orphan blob still exists: %v", err)
	}
}

func TestReopenMarksActiveJobsInterrupted(t *testing.T) {
	root := t.TempDir()
	options := Options{
		DBPath: filepath.Join(root, "data", "game_maker.db"), WorkspacePath: filepath.Join(root, "workspace"),
		Enabled: true, AllowCreate: true, AllowEdit: true,
	}
	service, err := NewService(options)
	if err != nil {
		t.Fatal(err)
	}
	service.SetSkillStatus(curatedSkills, true)
	project := createTestProject(t, service, "2d")
	now := time.Now().UTC()
	_, err = service.db.Exec(`INSERT INTO gm_jobs
		(id,project_id,kind,prompt,status,phase,provider_id,model,base_revision,created_at)
		VALUES(?,?,?,?,?,?,?,?,?,?)`, "job_restart", project.ID, "create", "test", "building", "building", "", "", 0, now)
	if err != nil {
		t.Fatal(err)
	}
	if err := service.Close(); err != nil {
		t.Fatal(err)
	}
	reopened, err := NewService(options)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	job, err := reopened.GetJob(context.Background(), "job_restart")
	if err != nil {
		t.Fatal(err)
	}
	if job.Status != "interrupted" || job.Phase != "interrupted" {
		t.Fatalf("job after restart = %+v", job)
	}
}

func TestPreviewTokenIsShortLivedScopedAndPathSafe(t *testing.T) {
	service := newTestService(t)
	service.SetRunner(testRunner{service: service})
	project := createTestProject(t, service, "2d")
	job, _ := service.StartJob(context.Background(), project.ID, StartJobRequest{})
	if finished := waitJob(t, service, job.ID); finished.Status != "ready" {
		t.Fatalf("job = %+v", finished)
	}
	grant, err := service.CreatePreviewGrant(project.ID)
	if err != nil {
		t.Fatal(err)
	}
	if grant.Token == "" || !strings.Contains(grant.URL, grant.Token) ||
		grant.ExpiresAt.Sub(time.Now().UTC()) > 2*time.Minute {
		t.Fatalf("invalid preview grant: %+v", grant)
	}
	data, contentType, err := service.PreviewFile(grant.Token, "index.html")
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(data, []byte(`dist/game.js`)) || !strings.Contains(contentType, "text/html") {
		t.Fatalf("unexpected preview response: %s, %q", data, contentType)
	}
	if _, _, err := service.PreviewFile(grant.Token, "../game_maker.db"); !errors.Is(err, ErrInvalidPath) {
		t.Fatalf("preview traversal error = %v", err)
	}
	service.mu.Lock()
	expired := service.tokens[grant.Token]
	expired.ExpiresAt = time.Now().Add(-time.Second)
	service.tokens[grant.Token] = expired
	service.mu.Unlock()
	if _, _, err := service.PreviewFile(grant.Token, "index.html"); !errors.Is(err, ErrInvalidToken) {
		t.Fatalf("expired token error = %v", err)
	}
}

func TestBundledSkillHashDriftBlocksReadiness(t *testing.T) {
	root := t.TempDir()
	first, err := InstallBundledSkills(root)
	if err != nil {
		t.Fatal(err)
	}
	if !first.Ready || len(first.Skills) != 5 {
		t.Fatalf("first install = %+v", first)
	}
	path := filepath.Join(root, "aurago-game-qa", "SKILL.md")
	if err := os.WriteFile(path, []byte("locally changed"), 0o640); err != nil {
		t.Fatal(err)
	}
	second, err := InstallBundledSkills(root)
	if err != nil {
		t.Fatal(err)
	}
	if second.Ready {
		t.Fatal("hash drift did not block bundled skills")
	}
	var mismatch bool
	for _, skill := range second.Skills {
		if skill.Name == "aurago-game-qa" && skill.Status == "hash_mismatch" {
			mismatch = true
		}
	}
	if !mismatch {
		t.Fatalf("hash mismatch not surfaced: %+v", second.Skills)
	}
}
