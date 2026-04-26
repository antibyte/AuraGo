package agent

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"aurago/internal/config"
	"aurago/internal/invasion"
)

func TestInvasionLoopbackURLUsesDedicatedLoopbackPort(t *testing.T) {
	cfg := &config.Config{}
	cfg.Server.Port = 8088
	cfg.Server.HTTPS.Enabled = true
	cfg.Server.HTTPS.HTTPSPort = 8443
	cfg.CloudflareTunnel.LoopbackPort = 18080

	got := invasionLoopbackURL(cfg, "/api/invasion/nests/n1/status")
	want := "http://127.0.0.1:18080/api/invasion/nests/n1/status"
	if got != want {
		t.Fatalf("invasionLoopbackURL = %q, want %q", got, want)
	}
}

func TestResolveInvasionTaskNestByEggNamePrefersRunningAssignedNest(t *testing.T) {
	db, err := invasion.InitDB(t.TempDir() + "/invasion.db")
	if err != nil {
		t.Fatalf("InitDB: %v", err)
	}
	defer db.Close()

	eggID, err := invasion.CreateEgg(db, invasion.EggRecord{
		Name:   "Web Scraper",
		Active: true,
	})
	if err != nil {
		t.Fatalf("CreateEgg: %v", err)
	}

	stoppedID, err := invasion.CreateNest(db, invasion.NestRecord{
		Name:        "stopped nest",
		AccessType:  "docker",
		Active:      true,
		EggID:       eggID,
		HatchStatus: "stopped",
	})
	if err != nil {
		t.Fatalf("CreateNest stopped: %v", err)
	}
	runningID, err := invasion.CreateNest(db, invasion.NestRecord{
		Name:        "running nest",
		AccessType:  "docker",
		Active:      true,
		EggID:       eggID,
		HatchStatus: "running",
	})
	if err != nil {
		t.Fatalf("CreateNest running: %v", err)
	}

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	nest, egg, err := resolveInvasionTaskNest(db, ToolCall{EggName: "web scraper"}, logger)
	if err != nil {
		t.Fatalf("resolveInvasionTaskNest: %v", err)
	}
	if nest.ID != runningID {
		t.Fatalf("resolved nest ID = %q, want running nest %q (stopped was %q)", nest.ID, runningID, stoppedID)
	}
	if egg.ID != eggID || egg.Name != "Web Scraper" {
		t.Fatalf("resolved egg = %#v, want Web Scraper %q", egg, eggID)
	}
}

func TestResolveInvasionEggStatusTargetByEggName(t *testing.T) {
	db, err := invasion.InitDB(t.TempDir() + "/invasion.db")
	if err != nil {
		t.Fatalf("InitDB: %v", err)
	}
	defer db.Close()

	eggID, err := invasion.CreateEgg(db, invasion.EggRecord{
		Name:   "Web Scraper",
		Active: true,
	})
	if err != nil {
		t.Fatalf("CreateEgg: %v", err)
	}

	nestID, err := invasion.CreateNest(db, invasion.NestRecord{
		Name:        "agent nest",
		AccessType:  "docker",
		Active:      true,
		EggID:       eggID,
		HatchStatus: "running",
	})
	if err != nil {
		t.Fatalf("CreateNest: %v", err)
	}

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	nest, egg, err := resolveInvasionEggStatusTarget(db, ToolCall{EggName: "web scraper"}, logger)
	if err != nil {
		t.Fatalf("resolveInvasionEggStatusTarget: %v", err)
	}
	if nest.ID != nestID {
		t.Fatalf("resolved nest ID = %q, want %q", nest.ID, nestID)
	}
	if egg.ID != eggID || egg.Name != "Web Scraper" {
		t.Fatalf("resolved egg = %#v, want Web Scraper %q", egg, eggID)
	}
}

func TestResolveInvasionEggStatusTargetByContainerStyleNestID(t *testing.T) {
	db, err := invasion.InitDB(t.TempDir() + "/invasion.db")
	if err != nil {
		t.Fatalf("InitDB: %v", err)
	}
	defer db.Close()

	eggID, err := invasion.CreateEgg(db, invasion.EggRecord{
		Name:   "Web Scraper",
		Active: true,
	})
	if err != nil {
		t.Fatalf("CreateEgg: %v", err)
	}

	nestID, err := invasion.CreateNest(db, invasion.NestRecord{
		Name:        "agent nest",
		AccessType:  "docker",
		Active:      true,
		EggID:       eggID,
		HatchStatus: "running",
	})
	if err != nil {
		t.Fatalf("CreateNest: %v", err)
	}

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	nest, egg, err := resolveInvasionEggStatusTarget(db, ToolCall{NestID: "aurago-egg-" + nestID[:8]}, logger)
	if err != nil {
		t.Fatalf("resolveInvasionEggStatusTarget: %v", err)
	}
	if nest.ID != nestID {
		t.Fatalf("resolved nest ID = %q, want %q", nest.ID, nestID)
	}
	if egg.ID != eggID || egg.Name != "Web Scraper" {
		t.Fatalf("resolved egg = %#v, want Web Scraper %q", egg, eggID)
	}
}

func TestWaitForInvasionTaskResultReturnsCompletedTask(t *testing.T) {
	db, err := invasion.InitDB(t.TempDir() + "/invasion.db")
	if err != nil {
		t.Fatalf("InitDB: %v", err)
	}
	defer db.Close()

	taskID, err := invasion.CreateTask(db, "nest-1", "egg-1", "tell a joke", 0)
	if err != nil {
		t.Fatalf("CreateTask: %v", err)
	}

	go func() {
		time.Sleep(30 * time.Millisecond)
		_ = invasion.UpdateTaskStatus(db, taskID, "completed", "Here is the result", "")
	}()

	task, completed, err := waitForInvasionTaskResult(db, taskID, 500*time.Millisecond, 10*time.Millisecond)
	if err != nil {
		t.Fatalf("waitForInvasionTaskResult: %v", err)
	}
	if !completed {
		t.Fatalf("completed = false, want true; task=%#v", task)
	}
	if task == nil || task.ResultOutput != "Here is the result" || task.Status != "completed" {
		t.Fatalf("task = %#v, want completed result", task)
	}
}

func TestWaitForInvasionTaskResultReturnsPendingTaskOnTimeout(t *testing.T) {
	db, err := invasion.InitDB(t.TempDir() + "/invasion.db")
	if err != nil {
		t.Fatalf("InitDB: %v", err)
	}
	defer db.Close()

	taskID, err := invasion.CreateTask(db, "nest-1", "egg-1", "slow task", 0)
	if err != nil {
		t.Fatalf("CreateTask: %v", err)
	}
	if err := invasion.UpdateTaskStatus(db, taskID, "sent", "", ""); err != nil {
		t.Fatalf("UpdateTaskStatus: %v", err)
	}

	task, completed, err := waitForInvasionTaskResult(db, taskID, 20*time.Millisecond, 5*time.Millisecond)
	if err != nil {
		t.Fatalf("waitForInvasionTaskResult: %v", err)
	}
	if completed {
		t.Fatalf("completed = true, want false")
	}
	if task == nil || task.Status != "sent" {
		t.Fatalf("task = %#v, want sent task", task)
	}
}

func TestInvasionTaskStatusReturnsStoredResult(t *testing.T) {
	db, err := invasion.InitDB(t.TempDir() + "/invasion.db")
	if err != nil {
		t.Fatalf("InitDB: %v", err)
	}
	defer db.Close()

	eggID, err := invasion.CreateEgg(db, invasion.EggRecord{Name: "Web Scraper", Active: true})
	if err != nil {
		t.Fatalf("CreateEgg: %v", err)
	}
	nestID, err := invasion.CreateNest(db, invasion.NestRecord{Name: "Test Nest", AccessType: "docker", Active: true, EggID: eggID})
	if err != nil {
		t.Fatalf("CreateNest: %v", err)
	}
	taskID, err := invasion.CreateTask(db, nestID, eggID, "tell a joke", 0)
	if err != nil {
		t.Fatalf("CreateTask: %v", err)
	}
	if err := invasion.UpdateTaskStatus(db, taskID, "completed", "A short answer", ""); err != nil {
		t.Fatalf("UpdateTaskStatus: %v", err)
	}

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	raw := invasionTaskStatus(db, ToolCall{Operation: "task_status", TaskID: taskID}, logger)
	payload := parseToolOutputJSON(t, raw)

	if payload["status"] != "success" {
		t.Fatalf("status = %v, want success; payload=%v", payload["status"], payload)
	}
	if payload["task_status"] != "completed" {
		t.Fatalf("task_status = %v, want completed; payload=%v", payload["task_status"], payload)
	}
	if payload["result_output"] != "A short answer" {
		t.Fatalf("result_output = %v, want stored output; payload=%v", payload["result_output"], payload)
	}
	if payload["nest_name"] != "Test Nest" || payload["egg_name"] != "Web Scraper" {
		t.Fatalf("names missing from payload: %v", payload)
	}
}

func TestInvasionListAndReadArtifacts(t *testing.T) {
	dir := t.TempDir()
	db, err := invasion.InitDB(filepath.Join(dir, "invasion.db"))
	if err != nil {
		t.Fatalf("InitDB: %v", err)
	}
	defer db.Close()

	filePath := filepath.Join(dir, "artifact.txt")
	content := "artifact contents"
	if err := os.WriteFile(filePath, []byte(content), 0600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	sum := sha256.Sum256([]byte(content))
	artifactID, err := invasion.RegisterCompletedArtifact(db, invasion.ArtifactRecord{
		NestID:      "nest-1",
		EggID:       "egg-1",
		MissionID:   "mission-1",
		Filename:    "artifact.txt",
		MIMEType:    "text/plain",
		SizeBytes:   int64(len(content)),
		SHA256:      hex.EncodeToString(sum[:]),
		StoragePath: filePath,
	})
	if err != nil {
		t.Fatalf("RegisterCompletedArtifact: %v", err)
	}

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	listRaw := invasionListArtifacts(db, ToolCall{NestID: "nest-1", Limit: 10}, logger)
	listPayload := parseToolOutputJSON(t, listRaw)
	if listPayload["status"] != "success" || int(listPayload["count"].(float64)) != 1 {
		t.Fatalf("unexpected list payload: %v", listPayload)
	}

	readRaw := invasionReadArtifact(db, ToolCall{ArtifactID: artifactID}, logger)
	readPayload := parseToolOutputJSON(t, readRaw)
	if readPayload["status"] != "success" {
		t.Fatalf("unexpected read payload: %v", readPayload)
	}
	if readPayload["content"] != content {
		t.Fatalf("content = %q, want %q", readPayload["content"], content)
	}
}

func TestInvasionReadArtifactRejectsBinary(t *testing.T) {
	dir := t.TempDir()
	db, err := invasion.InitDB(filepath.Join(dir, "invasion.db"))
	if err != nil {
		t.Fatalf("InitDB: %v", err)
	}
	defer db.Close()

	filePath := filepath.Join(dir, "image.png")
	if err := os.WriteFile(filePath, []byte{0x89, 0x50, 0x4e, 0x47}, 0600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	artifactID, err := invasion.RegisterCompletedArtifact(db, invasion.ArtifactRecord{
		NestID:      "nest-1",
		EggID:       "egg-1",
		Filename:    "image.png",
		MIMEType:    "image/png",
		SizeBytes:   4,
		StoragePath: filePath,
	})
	if err != nil {
		t.Fatalf("RegisterCompletedArtifact: %v", err)
	}

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	raw := invasionReadArtifact(db, ToolCall{ArtifactID: artifactID}, logger)
	payload := parseToolOutputJSON(t, raw)
	if payload["status"] != "error" {
		t.Fatalf("status = %v, want error; payload=%v", payload["status"], payload)
	}
	if !strings.Contains(payload["message"].(string), "not text-readable") {
		t.Fatalf("message = %v", payload["message"])
	}
}

func TestInvasionEggOnlyArtifactOperationsRequireEggMode(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	cfg := &config.Config{}

	uploadPayload := parseToolOutputJSON(t, invasionUploadArtifact(cfg, ToolCall{FilePath: "report.txt"}, logger))
	if uploadPayload["status"] != "error" || !strings.Contains(uploadPayload["message"].(string), "egg mode") {
		t.Fatalf("unexpected upload payload: %v", uploadPayload)
	}

	messagePayload := parseToolOutputJSON(t, invasionSendHostMessage(cfg, ToolCall{Body: "hello"}, logger))
	if messagePayload["status"] != "error" || !strings.Contains(messagePayload["message"].(string), "egg mode") {
		t.Fatalf("unexpected message payload: %v", messagePayload)
	}
}

func TestResolveEggArtifactPathStaysInsideWorkspace(t *testing.T) {
	workspace := t.TempDir()
	cfg := &config.Config{}
	cfg.Directories.WorkspaceDir = workspace

	inside := filepath.Join(workspace, "report.txt")
	if err := os.WriteFile(inside, []byte("ok"), 0600); err != nil {
		t.Fatalf("WriteFile inside: %v", err)
	}
	resolved, err := resolveEggArtifactPath(cfg, "report.txt")
	if err != nil {
		t.Fatalf("resolve inside path: %v", err)
	}
	if resolved != inside {
		t.Fatalf("resolved = %q, want %q", resolved, inside)
	}

	outside := filepath.Join(t.TempDir(), "secret.txt")
	if err := os.WriteFile(outside, []byte("nope"), 0600); err != nil {
		t.Fatalf("WriteFile outside: %v", err)
	}
	if _, err := resolveEggArtifactPath(cfg, outside); err == nil {
		t.Fatal("expected outside workspace path to be rejected")
	}
}

func parseToolOutputJSON(t *testing.T, raw string) map[string]interface{} {
	t.Helper()
	const prefix = "Tool Output: "
	if !strings.HasPrefix(raw, prefix) {
		t.Fatalf("raw output %q does not start with %q", raw, prefix)
	}
	var payload map[string]interface{}
	if err := json.Unmarshal([]byte(strings.TrimPrefix(raw, prefix)), &payload); err != nil {
		t.Fatalf("Unmarshal tool output: %v\nraw=%s", err, raw)
	}
	return payload
}
