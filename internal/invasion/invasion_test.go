package invasion

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func tempDB(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	return filepath.Join(dir, "invasion_test.db")
}

func TestInitDB(t *testing.T) {
	dbPath := tempDB(t)
	db, err := InitDB(dbPath)
	if err != nil {
		t.Fatalf("InitDB failed: %v", err)
	}
	defer db.Close()

	// Verify file was created
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		t.Fatal("database file was not created")
	}

	// Verify tables exist
	for _, table := range []string{"nests", "eggs"} {
		var count int
		err := db.QueryRow("SELECT count(*) FROM sqlite_master WHERE type='table' AND name=?", table).Scan(&count)
		if err != nil {
			t.Fatalf("failed to check table %s: %v", table, err)
		}
		if count != 1 {
			t.Fatalf("table %s not found", table)
		}
	}
}

func TestNestCRUD(t *testing.T) {
	db, err := InitDB(tempDB(t))
	if err != nil {
		t.Fatalf("InitDB: %v", err)
	}
	defer db.Close()

	// Create
	nest := NestRecord{
		Name:       "Test Server",
		Notes:      "Integration test nest",
		AccessType: "ssh",
		Host:       "192.168.1.100",
		Port:       22,
		Username:   "deploy",
		Active:     true,
	}
	id, err := CreateNest(db, nest)
	if err != nil {
		t.Fatalf("CreateNest: %v", err)
	}
	if id == "" {
		t.Fatal("CreateNest returned empty ID")
	}

	// Read
	got, err := GetNest(db, id)
	if err != nil {
		t.Fatalf("GetNest: %v", err)
	}
	if got.Name != "Test Server" {
		t.Errorf("name = %q, want %q", got.Name, "Test Server")
	}
	if got.AccessType != "ssh" {
		t.Errorf("access_type = %q, want %q", got.AccessType, "ssh")
	}
	if got.Host != "192.168.1.100" {
		t.Errorf("host = %q, want %q", got.Host, "192.168.1.100")
	}
	if !got.Active {
		t.Error("expected active = true")
	}
	if got.CreatedAt == "" {
		t.Error("created_at should be set")
	}

	// Update
	got.Name = "Updated Server"
	got.Notes = "Updated notes"
	if err := UpdateNest(db, got); err != nil {
		t.Fatalf("UpdateNest: %v", err)
	}
	updated, _ := GetNest(db, id)
	if updated.Name != "Updated Server" {
		t.Errorf("updated name = %q, want %q", updated.Name, "Updated Server")
	}
	if updated.Notes != "Updated notes" {
		t.Errorf("updated notes = %q, want %q", updated.Notes, "Updated notes")
	}

	// Toggle active
	if err := ToggleNestActive(db, id, false); err != nil {
		t.Fatalf("ToggleNestActive: %v", err)
	}
	toggled, _ := GetNest(db, id)
	if toggled.Active {
		t.Error("expected active = false after toggle")
	}

	// List
	nests, err := ListNests(db)
	if err != nil {
		t.Fatalf("ListNests: %v", err)
	}
	if len(nests) != 1 {
		t.Errorf("ListNests count = %d, want 1", len(nests))
	}

	// ListActive (should be 0 since we toggled off)
	activeNests, err := ListActiveNests(db)
	if err != nil {
		t.Fatalf("ListActiveNests: %v", err)
	}
	if len(activeNests) != 0 {
		t.Errorf("ListActiveNests count = %d, want 0", len(activeNests))
	}

	// GetByName
	byName, err := GetNestByName(db, "updated server") // case-insensitive
	if err != nil {
		t.Fatalf("GetNestByName: %v", err)
	}
	if byName.ID != id {
		t.Errorf("GetNestByName ID = %q, want %q", byName.ID, id)
	}

	// Delete
	if err := DeleteNest(db, id); err != nil {
		t.Fatalf("DeleteNest: %v", err)
	}
	_, err = GetNest(db, id)
	if err == nil {
		t.Fatal("expected error after delete, got nil")
	}
}

func TestEggCRUD(t *testing.T) {
	db, err := InitDB(tempDB(t))
	if err != nil {
		t.Fatalf("InitDB: %v", err)
	}
	defer db.Close()

	// Create
	egg := EggRecord{
		Name:        "Analytics Agent",
		Description: "Processes analytics data",
		Model:       "gpt-4o-mini",
		Provider:    "openrouter",
		Active:      true,
	}
	id, err := CreateEgg(db, egg)
	if err != nil {
		t.Fatalf("CreateEgg: %v", err)
	}
	if id == "" {
		t.Fatal("CreateEgg returned empty ID")
	}

	// Read
	got, err := GetEgg(db, id)
	if err != nil {
		t.Fatalf("GetEgg: %v", err)
	}
	if got.Name != "Analytics Agent" {
		t.Errorf("name = %q, want %q", got.Name, "Analytics Agent")
	}
	if got.Model != "gpt-4o-mini" {
		t.Errorf("model = %q, want %q", got.Model, "gpt-4o-mini")
	}
	if !got.Active {
		t.Error("expected active = true")
	}

	// Update
	got.Description = "Updated description"
	got.Model = "claude-3.5-sonnet"
	if err := UpdateEgg(db, got); err != nil {
		t.Fatalf("UpdateEgg: %v", err)
	}
	updated, _ := GetEgg(db, id)
	if updated.Description != "Updated description" {
		t.Errorf("description = %q, want %q", updated.Description, "Updated description")
	}
	if updated.Model != "claude-3.5-sonnet" {
		t.Errorf("model = %q, want %q", updated.Model, "claude-3.5-sonnet")
	}

	// Toggle
	if err := ToggleEggActive(db, id, false); err != nil {
		t.Fatalf("ToggleEggActive: %v", err)
	}
	toggled, _ := GetEgg(db, id)
	if toggled.Active {
		t.Error("expected active = false")
	}

	// List
	eggs, err := ListEggs(db)
	if err != nil {
		t.Fatalf("ListEggs: %v", err)
	}
	if len(eggs) != 1 {
		t.Errorf("ListEggs count = %d, want 1", len(eggs))
	}

	// Delete
	if err := DeleteEgg(db, id); err != nil {
		t.Fatalf("DeleteEgg: %v", err)
	}
	_, err = GetEgg(db, id)
	if err == nil {
		t.Fatal("expected error after delete, got nil")
	}
}

func TestDeleteNonExistent(t *testing.T) {
	db, err := InitDB(tempDB(t))
	if err != nil {
		t.Fatalf("InitDB: %v", err)
	}
	defer db.Close()

	if err := DeleteNest(db, "nonexistent"); err == nil {
		t.Error("expected error deleting non-existent nest")
	}
	if err := DeleteEgg(db, "nonexistent"); err == nil {
		t.Error("expected error deleting non-existent egg")
	}
}

func TestNestEggAssignment(t *testing.T) {
	db, err := InitDB(tempDB(t))
	if err != nil {
		t.Fatalf("InitDB: %v", err)
	}
	defer db.Close()

	eggID, _ := CreateEgg(db, EggRecord{Name: "Worker", Active: true})
	nestID, _ := CreateNest(db, NestRecord{Name: "Server", AccessType: "ssh", Host: "10.0.0.1", Port: 22, Active: true})

	// Assign egg to nest
	nest, _ := GetNest(db, nestID)
	nest.EggID = eggID
	if err := UpdateNest(db, nest); err != nil {
		t.Fatalf("assign egg: %v", err)
	}

	updated, _ := GetNest(db, nestID)
	if updated.EggID != eggID {
		t.Errorf("egg_id = %q, want %q", updated.EggID, eggID)
	}
}

// ── Task CRUD tests ─────────────────────────────────────────────────────────

func TestTaskCRUD(t *testing.T) {
	db, err := InitDB(tempDB(t))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	// Create
	id, err := CreateTask(db, "nest-1", "egg-1", "run backup", 300)
	if err != nil {
		t.Fatalf("CreateTask: %v", err)
	}
	if id == "" {
		t.Fatal("expected non-empty task ID")
	}

	// Read
	task, err := GetTaskByID(db, id)
	if err != nil {
		t.Fatalf("GetTaskByID: %v", err)
	}
	if task.Status != "pending" {
		t.Errorf("status = %q, want pending", task.Status)
	}
	if task.NestID != "nest-1" {
		t.Errorf("nest_id = %q", task.NestID)
	}
	if task.Description != "run backup" {
		t.Errorf("description = %q", task.Description)
	}
	if task.Timeout != 300 {
		t.Errorf("timeout = %d", task.Timeout)
	}

	// Update to sent
	if err := UpdateTaskStatus(db, id, "sent", "", ""); err != nil {
		t.Fatal(err)
	}
	task, _ = GetTaskByID(db, id)
	if task.Status != "sent" {
		t.Errorf("status = %q, want sent", task.Status)
	}
	if task.SentAt == "" {
		t.Error("sent_at should be set after marking sent")
	}

	// Update to acked
	if err := UpdateTaskStatus(db, id, "acked", "", ""); err != nil {
		t.Fatal(err)
	}
	task, _ = GetTaskByID(db, id)
	if task.Status != "acked" {
		t.Errorf("status = %q, want acked", task.Status)
	}

	// Update to completed
	if err := UpdateTaskStatus(db, id, "completed", "backup done", ""); err != nil {
		t.Fatal(err)
	}
	task, _ = GetTaskByID(db, id)
	if task.Status != "completed" {
		t.Errorf("status = %q, want completed", task.Status)
	}
	if task.ResultOutput != "backup done" {
		t.Errorf("output = %q", task.ResultOutput)
	}
	if task.CompletedAt == "" {
		t.Error("completed_at should be set")
	}
}

func TestGetPendingTasks(t *testing.T) {
	db, err := InitDB(tempDB(t))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	id1, _ := CreateTask(db, "nest-1", "egg-1", "task 1", 0)
	id2, _ := CreateTask(db, "nest-1", "egg-1", "task 2", 0)
	CreateTask(db, "nest-2", "egg-2", "task 3", 0) // different nest

	// Mark one as sent (still recoverable)
	UpdateTaskStatus(db, id1, "sent", "", "")

	pending, err := GetPendingTasks(db, "nest-1")
	if err != nil {
		t.Fatal(err)
	}
	if len(pending) != 2 {
		t.Fatalf("pending count = %d, want 2", len(pending))
	}

	// Complete one
	UpdateTaskStatus(db, id2, "completed", "done", "")
	pending, _ = GetPendingTasks(db, "nest-1")
	if len(pending) != 1 {
		t.Errorf("pending after complete = %d, want 1", len(pending))
	}
}

func TestGetTasksByNest(t *testing.T) {
	db, err := InitDB(tempDB(t))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	CreateTask(db, "nest-1", "egg-1", "task A", 0)
	CreateTask(db, "nest-1", "egg-1", "task B", 0)
	CreateTask(db, "nest-2", "egg-2", "task C", 0)

	tasks, err := GetTasksByNest(db, "nest-1", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(tasks) != 2 {
		t.Errorf("task count = %d, want 2", len(tasks))
	}
}

func TestCleanupOldTasks(t *testing.T) {
	db, err := InitDB(tempDB(t))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	id1, _ := CreateTask(db, "nest-1", "egg-1", "old task", 0)
	id2, _ := CreateTask(db, "nest-1", "egg-1", "new task", 0)

	// Mark both completed
	UpdateTaskStatus(db, id1, "completed", "done", "")
	UpdateTaskStatus(db, id2, "completed", "done", "")

	// Backdate id1 by modifying completed_at directly
	_, err = db.Exec(`UPDATE invasion_tasks SET completed_at=? WHERE id=?`,
		time.Now().Add(-8*24*time.Hour).UTC().Format(time.RFC3339), id1)
	if err != nil {
		t.Fatal(err)
	}

	n, err := CleanupOldTasks(db, 7*24*time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Errorf("cleaned = %d, want 1", n)
	}

	// Verify id2 still exists
	_, err = GetTaskByID(db, id2)
	if err != nil {
		t.Error("new task should still exist after cleanup")
	}
	_, err = GetTaskByID(db, id1)
	if err == nil {
		t.Error("old task should be deleted after cleanup")
	}
}

func TestTaskNotFound(t *testing.T) {
	db, err := InitDB(tempDB(t))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	_, err = GetTaskByID(db, "nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent task")
	}
}

func TestInitDB_TasksTable(t *testing.T) {
	db, err := InitDB(tempDB(t))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	var count int
	err = db.QueryRow("SELECT count(*) FROM sqlite_master WHERE type='table' AND name='invasion_tasks'").Scan(&count)
	if err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Error("invasion_tasks table not found")
	}
}

// ── Deployment history tests ────────────────────────────────────────────────

func TestInitDB_DeploymentHistoryTable(t *testing.T) {
	db, err := InitDB(tempDB(t))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	var count int
	err = db.QueryRow("SELECT count(*) FROM sqlite_master WHERE type='table' AND name='deployment_history'").Scan(&count)
	if err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Error("deployment_history table not found")
	}
}

func TestDeploymentCRUD(t *testing.T) {
	db, err := InitDB(tempDB(t))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	// Create
	id, err := CreateDeployment(db, "nest-1", "egg-1", "ssh", "abc123", "def456")
	if err != nil {
		t.Fatalf("CreateDeployment: %v", err)
	}
	if id == "" {
		t.Fatal("expected non-empty deployment ID")
	}

	// Read
	d, err := GetDeployment(db, id)
	if err != nil {
		t.Fatalf("GetDeployment: %v", err)
	}
	if d.Status != "started" {
		t.Errorf("status = %q, want started", d.Status)
	}
	if d.NestID != "nest-1" {
		t.Errorf("nest_id = %q", d.NestID)
	}
	if d.BinaryHash != "abc123" {
		t.Errorf("binary_hash = %q", d.BinaryHash)
	}
	if d.ConfigHash != "def456" {
		t.Errorf("config_hash = %q", d.ConfigHash)
	}
	if d.DeployMethod != "ssh" {
		t.Errorf("deploy_method = %q", d.DeployMethod)
	}

	// Update to deployed
	if err := UpdateDeploymentStatus(db, id, "deployed"); err != nil {
		t.Fatal(err)
	}
	d, _ = GetDeployment(db, id)
	if d.Status != "deployed" {
		t.Errorf("status = %q, want deployed", d.Status)
	}
	if d.DeployedAt == "" {
		t.Error("deployed_at should be set")
	}

	// Update to verified
	if err := UpdateDeploymentStatus(db, id, "verified"); err != nil {
		t.Fatal(err)
	}
	d, _ = GetDeployment(db, id)
	if d.Status != "verified" {
		t.Errorf("status = %q, want verified", d.Status)
	}
	if d.VerifiedAt == "" {
		t.Error("verified_at should be set")
	}
}

func TestGetLastSuccessfulDeployment(t *testing.T) {
	db, err := InitDB(tempDB(t))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	// No deployments yet
	_, err = GetLastSuccessfulDeployment(db, "nest-1")
	if err == nil {
		t.Error("expected error when no deployments exist")
	}

	// Create a failed one
	id1, _ := CreateDeployment(db, "nest-1", "egg-1", "ssh", "h1", "c1")
	_ = UpdateDeploymentStatus(db, id1, "failed")

	// Still no successful
	_, err = GetLastSuccessfulDeployment(db, "nest-1")
	if err == nil {
		t.Error("expected error when only failed deployments exist")
	}

	// Create a verified one
	id2, _ := CreateDeployment(db, "nest-1", "egg-1", "ssh", "h2", "c2")
	_ = UpdateDeploymentStatus(db, id2, "verified")

	last, err := GetLastSuccessfulDeployment(db, "nest-1")
	if err != nil {
		t.Fatalf("GetLastSuccessfulDeployment: %v", err)
	}
	if last.ID != id2 {
		t.Errorf("last success = %q, want %q", last.ID, id2)
	}
}

func TestDeploymentHistory(t *testing.T) {
	db, err := InitDB(tempDB(t))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	CreateDeployment(db, "nest-1", "egg-1", "ssh", "h1", "c1")
	CreateDeployment(db, "nest-1", "egg-1", "ssh", "h2", "c2")
	CreateDeployment(db, "nest-2", "egg-2", "docker_remote", "h3", "c3")

	history, err := GetDeploymentHistory(db, "nest-1", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(history) != 2 {
		t.Errorf("history count = %d, want 2", len(history))
	}
	// Newest first
	if history[0].BinaryHash != "h2" {
		t.Errorf("first entry hash = %q, want h2", history[0].BinaryHash)
	}
}

func TestDeploymentRollbackStatus(t *testing.T) {
	db, err := InitDB(tempDB(t))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	id, _ := CreateDeployment(db, "nest-1", "egg-1", "ssh", "h1", "c1")
	_ = UpdateDeploymentStatus(db, id, "verified")

	// Roll back
	if err := UpdateDeploymentStatus(db, id, "rolled_back"); err != nil {
		t.Fatal(err)
	}
	d, _ := GetDeployment(db, id)
	if d.Status != "rolled_back" {
		t.Errorf("status = %q, want rolled_back", d.Status)
	}
	if d.RolledBackAt == "" {
		t.Error("rolled_back_at should be set")
	}
}

func TestCleanupOldDeployments(t *testing.T) {
	db, err := InitDB(tempDB(t))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	id1, _ := CreateDeployment(db, "nest-1", "egg-1", "ssh", "h1", "c1")
	id2, _ := CreateDeployment(db, "nest-1", "egg-1", "ssh", "h2", "c2")

	// Backdate id1 by 31 days
	_, err = db.Exec(`UPDATE deployment_history SET created_at=? WHERE id=?`,
		time.Now().Add(-31*24*time.Hour).UTC().Format(time.RFC3339), id1)
	if err != nil {
		t.Fatal(err)
	}

	n, err := CleanupOldDeployments(db, 30*24*time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Errorf("cleaned = %d, want 1", n)
	}

	// id2 still exists
	_, err = GetDeployment(db, id2)
	if err != nil {
		t.Error("recent deployment should still exist")
	}
	// id1 deleted
	_, err = GetDeployment(db, id1)
	if err == nil {
		t.Error("old deployment should be deleted")
	}
}

// ── Safe Config Revision tests ──────────────────────────────────────────────

func TestSafeConfigRevision_CRUD(t *testing.T) {
	db, err := InitDB(tempDB(t))
	if err != nil {
		t.Fatalf("InitDB: %v", err)
	}
	defer db.Close()

	nest := NestRecord{Name: "Rev Test", AccessType: "ssh", Host: "10.0.0.1", Port: 22, Active: true}
	nestID, err := CreateNest(db, nest)
	if err != nil {
		t.Fatalf("CreateNest: %v", err)
	}

	egg := EggRecord{Name: "Rev Egg", Model: "gpt-4", Active: true}
	eggID, err := CreateEgg(db, egg)
	if err != nil {
		t.Fatalf("CreateEgg: %v", err)
	}

	patchJSON := `{"model":"gpt-4o"}`
	configHash := HashConfigYAML([]byte("test: yaml"))

	// Create
	revID, err := CreateSafeConfigRevision(db, nestID, eggID, patchJSON, configHash, "safe_reconfigure")
	if err != nil {
		t.Fatalf("CreateSafeConfigRevision: %v", err)
	}
	if revID == "" {
		t.Fatal("expected non-empty revision ID")
	}

	// Get
	rev, err := GetSafeConfigRevision(db, revID)
	if err != nil {
		t.Fatalf("GetSafeConfigRevision: %v", err)
	}
	if rev.NestID != nestID {
		t.Errorf("nest_id = %q, want %q", rev.NestID, nestID)
	}
	if rev.EggID != eggID {
		t.Errorf("egg_id = %q, want %q", rev.EggID, eggID)
	}
	if rev.RevisionNumber != 1 {
		t.Errorf("revision_number = %d, want 1", rev.RevisionNumber)
	}
	if rev.Status != "pending" {
		t.Errorf("status = %q, want pending", rev.Status)
	}
	if rev.Source != "safe_reconfigure" {
		t.Errorf("source = %q, want safe_reconfigure", rev.Source)
	}
	if rev.ConfigHash != configHash {
		t.Errorf("config_hash = %q, want %q", rev.ConfigHash, configHash)
	}

	// Update to applying
	if err := UpdateSafeConfigRevisionStatus(db, revID, "applying", ""); err != nil {
		t.Fatalf("UpdateSafeConfigRevisionStatus applying: %v", err)
	}
	rev, _ = GetSafeConfigRevision(db, revID)
	if rev.Status != "applying" {
		t.Errorf("status = %q, want applying", rev.Status)
	}

	// Update to applied
	if err := UpdateSafeConfigRevisionStatus(db, revID, "applied", ""); err != nil {
		t.Fatalf("UpdateSafeConfigRevisionStatus applied: %v", err)
	}
	rev, _ = GetSafeConfigRevision(db, revID)
	if rev.Status != "applied" {
		t.Errorf("status = %q, want applied", rev.Status)
	}
	if rev.AppliedAt == "" {
		t.Error("applied_at should be set")
	}

	// GetLatestAppliedRevision
	latest, err := GetLatestAppliedRevision(db, nestID)
	if err != nil {
		t.Fatalf("GetLatestAppliedRevision: %v", err)
	}
	if latest == nil || latest.ID != revID {
		t.Errorf("latest applied = %+v, want ID %s", latest, revID)
	}

	// Create second revision
	revID2, _ := CreateSafeConfigRevision(db, nestID, eggID, `{"model":"gpt-5"}`, "hash2", "safe_reconfigure")
	rev2, _ := GetSafeConfigRevision(db, revID2)
	if rev2.RevisionNumber != 2 {
		t.Errorf("second revision_number = %d, want 2", rev2.RevisionNumber)
	}

	// List
	revs, err := ListSafeConfigRevisions(db, nestID, 10)
	if err != nil {
		t.Fatalf("ListSafeConfigRevisions: %v", err)
	}
	if len(revs) != 2 {
		t.Errorf("len(revs) = %d, want 2", len(revs))
	}
	// Newest first
	if revs[0].RevisionNumber != 2 {
		t.Errorf("first rev number = %d, want 2", revs[0].RevisionNumber)
	}

	// Update to failed
	if err := UpdateSafeConfigRevisionStatus(db, revID2, "failed", "test error"); err != nil {
		t.Fatalf("UpdateSafeConfigRevisionStatus failed: %v", err)
	}
	rev2, _ = GetSafeConfigRevision(db, revID2)
	if rev2.ErrorMessage != "test error" {
		t.Errorf("error_message = %q, want 'test error'", rev2.ErrorMessage)
	}
}

func TestSafeConfigRevision_NestConfigRev(t *testing.T) {
	db, err := InitDB(tempDB(t))
	if err != nil {
		t.Fatalf("InitDB: %v", err)
	}
	defer db.Close()

	nest := NestRecord{Name: "CfgRev Test", AccessType: "ssh", Host: "10.0.0.2", Port: 22, Active: true}
	nestID, _ := CreateNest(db, nest)

	// Update config rev
	if err := UpdateNestConfigRev(db, nestID, "rev-desired-1", "rev-applied-1"); err != nil {
		t.Fatalf("UpdateNestConfigRev: %v", err)
	}

	got, _ := GetNest(db, nestID)
	if got.DesiredConfigRev != "rev-desired-1" {
		t.Errorf("desired_config_rev = %q, want 'rev-desired-1'", got.DesiredConfigRev)
	}
	if got.AppliedConfigRev != "rev-applied-1" {
		t.Errorf("applied_config_rev = %q, want 'rev-applied-1'", got.AppliedConfigRev)
	}
}

func TestValidateSafeConfigPatch(t *testing.T) {
	// Valid: empty patch
	if err := ValidateSafeConfigPatch(SafeConfigPatch{}); err != nil {
		t.Errorf("empty patch should be valid: %v", err)
	}

	// Valid: model only
	model := "gpt-4o"
	if err := ValidateSafeConfigPatch(SafeConfigPatch{Model: &model}); err != nil {
		t.Errorf("model-only patch should be valid: %v", err)
	}

	// Invalid: inherit_llm=true with provider
	inheritTrue := true
	provider := "openai"
	if err := ValidateSafeConfigPatch(SafeConfigPatch{InheritLLM: &inheritTrue, Provider: &provider}); err == nil {
		t.Error("inherit_llm=true with provider should fail validation")
	}

	// Invalid: inherit_llm=true with base_url
	baseURL := "https://api.example.com"
	if err := ValidateSafeConfigPatch(SafeConfigPatch{InheritLLM: &inheritTrue, BaseURL: &baseURL}); err == nil {
		t.Error("inherit_llm=true with base_url should fail validation")
	}

	// Invalid: unknown tool
	if err := ValidateSafeConfigPatch(SafeConfigPatch{AllowedTools: []string{"shell", "dangerous_tool"}}); err == nil {
		t.Error("unknown tool should fail validation")
	}

	// Valid: known tools
	if err := ValidateSafeConfigPatch(SafeConfigPatch{AllowedTools: []string{"shell", "python"}}); err != nil {
		t.Errorf("known tools should be valid: %v", err)
	}

	// Valid: inherit_llm=false with provider
	inheritFalse := false
	if err := ValidateSafeConfigPatch(SafeConfigPatch{InheritLLM: &inheritFalse, Provider: &provider}); err != nil {
		t.Errorf("inherit_llm=false with provider should be valid: %v", err)
	}
}

func TestHashConfigYAML(t *testing.T) {
	h1 := HashConfigYAML([]byte("test: value"))
	h2 := HashConfigYAML([]byte("test: value"))
	if h1 != h2 {
		t.Error("same input should produce same hash")
	}
	h3 := HashConfigYAML([]byte("test: other"))
	if h1 == h3 {
		t.Error("different input should produce different hash")
	}
	if len(h1) != 64 {
		t.Errorf("hash length = %d, want 64 (SHA-256 hex)", len(h1))
	}
}

func TestPatchToJSON_JSONToPatch(t *testing.T) {
	model := "gpt-4o"
	patch := SafeConfigPatch{Model: &model}

	jsonStr, err := PatchToJSON(patch)
	if err != nil {
		t.Fatalf("PatchToJSON: %v", err)
	}

	got, err := JSONToPatch(jsonStr)
	if err != nil {
		t.Fatalf("JSONToPatch: %v", err)
	}
	if got.Model == nil || *got.Model != "gpt-4o" {
		t.Errorf("model = %+v, want 'gpt-4o'", got.Model)
	}
}
