package invasion

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestArtifactUploadLifecycleStoresCompletedArtifact(t *testing.T) {
	db, err := InitDB(filepath.Join(t.TempDir(), "invasion.db"))
	if err != nil {
		t.Fatalf("InitDB: %v", err)
	}
	defer db.Close()

	sum := sha256.Sum256([]byte("hello"))
	expectedHash := hex.EncodeToString(sum[:])

	token, artifact, err := CreateArtifactUpload(db, ArtifactUploadRequest{
		NestID:         "nest-1",
		EggID:          "egg-1",
		MissionID:      "mission-1",
		TaskID:         "task-1",
		Filename:       "../report.txt",
		MIMEType:       "text/plain",
		ExpectedSize:   5,
		ExpectedSHA256: expectedHash,
		MetadataJSON:   `{"kind":"test"}`,
		TTL:            time.Minute,
	})
	if err != nil {
		t.Fatalf("CreateArtifactUpload: %v", err)
	}
	if token == "" {
		t.Fatal("expected upload token")
	}
	if artifact.Filename != "report.txt" {
		t.Fatalf("filename = %q, want sanitized report.txt", artifact.Filename)
	}
	if artifact.Status != ArtifactStatusPending {
		t.Fatalf("status = %q, want pending", artifact.Status)
	}

	slot, err := GetArtifactUploadByToken(db, token, time.Now())
	if err != nil {
		t.Fatalf("GetArtifactUploadByToken: %v", err)
	}
	if slot.Artifact.ID != artifact.ID || slot.ExpectedSHA256 != expectedHash {
		t.Fatalf("slot = %#v, artifact = %#v", slot, artifact)
	}

	store := NewArtifactStorage(t.TempDir())
	stored, err := store.Save(slot.Artifact, strings.NewReader("hello"), slot.ExpectedSize, slot.ExpectedSHA256)
	if err != nil {
		t.Fatalf("Save: %v", err)
	}
	if _, err := os.Stat(stored.Path); err != nil {
		t.Fatalf("stored file missing: %v", err)
	}

	if err := CompleteArtifactUpload(db, token, stored.Path, stored.SizeBytes, stored.SHA256, time.Now()); err != nil {
		t.Fatalf("CompleteArtifactUpload: %v", err)
	}

	completed, err := GetArtifact(db, artifact.ID)
	if err != nil {
		t.Fatalf("GetArtifact: %v", err)
	}
	if completed.Status != ArtifactStatusCompleted {
		t.Fatalf("status = %q, want completed", completed.Status)
	}
	if completed.StoragePath != stored.Path || completed.SHA256 != expectedHash || completed.SizeBytes != 5 {
		t.Fatalf("completed artifact = %#v, stored = %#v", completed, stored)
	}
	if completed.WebPath != "/api/invasion/artifacts/"+artifact.ID+"/download" {
		t.Fatalf("web_path = %q", completed.WebPath)
	}
}

func TestArtifactStorageRejectsHashMismatchAndRemovesPartialFile(t *testing.T) {
	store := NewArtifactStorage(t.TempDir())
	artifact := ArtifactRecord{
		ID:       "artifact-1",
		NestID:   "nest-1",
		Filename: "evidence.txt",
	}

	_, err := store.Save(artifact, strings.NewReader("hello"), 5, strings.Repeat("0", 64))
	if err == nil {
		t.Fatal("expected hash mismatch error")
	}

	dir := filepath.Join(store.BaseDir, "nest-1", "artifact-1")
	entries, readErr := os.ReadDir(dir)
	if readErr == nil && len(entries) > 0 {
		t.Fatalf("expected partial files to be removed, found %d entries", len(entries))
	}
}

func TestArtifactStorageKeepsPathsInsideBaseDir(t *testing.T) {
	base := t.TempDir()
	store := NewArtifactStorage(base)
	stored, err := store.Save(ArtifactRecord{
		ID:       "../artifact-1",
		NestID:   "../nest-1",
		Filename: "../evidence.txt",
	}, strings.NewReader("hello"), 5, "")
	if err != nil {
		t.Fatalf("Save: %v", err)
	}
	rel, err := filepath.Rel(base, stored.Path)
	if err != nil {
		t.Fatalf("Rel: %v", err)
	}
	if strings.HasPrefix(rel, "..") || filepath.IsAbs(rel) {
		t.Fatalf("stored path escaped base dir: %q", stored.Path)
	}
}

func TestEggMessageRateLimitAndDedup(t *testing.T) {
	db, err := InitDB(filepath.Join(t.TempDir(), "invasion.db"))
	if err != nil {
		t.Fatalf("InitDB: %v", err)
	}
	defer db.Close()

	now := time.Now().UTC()
	policy := EggMessageRatePolicy{Burst: 2, Window: time.Minute}
	base := EggMessageRecord{
		NestID:          "nest-1",
		EggID:           "egg-1",
		Severity:        "warning",
		Title:           "Artifact ready",
		Body:            "New evidence was produced",
		DedupKey:        "artifact-ready",
		WakeupRequested: true,
	}

	first, err := RecordEggMessage(db, base, policy, now)
	if err != nil {
		t.Fatalf("RecordEggMessage first: %v", err)
	}
	if !first.WakeupAllowed {
		t.Fatalf("first WakeupAllowed = false, want true")
	}

	second := base
	second.DedupKey = "artifact-ready-2"
	secondMsg, err := RecordEggMessage(db, second, policy, now.Add(time.Second))
	if err != nil {
		t.Fatalf("RecordEggMessage second: %v", err)
	}
	if !secondMsg.WakeupAllowed {
		t.Fatalf("second WakeupAllowed = false, want true")
	}

	third := base
	third.DedupKey = "artifact-ready-3"
	thirdMsg, err := RecordEggMessage(db, third, policy, now.Add(2*time.Second))
	if err != nil {
		t.Fatalf("RecordEggMessage third: %v", err)
	}
	if thirdMsg.WakeupAllowed {
		t.Fatalf("third WakeupAllowed = true, want rate-limited false")
	}

	duplicate, err := RecordEggMessage(db, base, policy, now.Add(3*time.Second))
	if err != nil {
		t.Fatalf("RecordEggMessage duplicate: %v", err)
	}
	if duplicate.ID != first.ID {
		t.Fatalf("duplicate ID = %q, want first ID %q", duplicate.ID, first.ID)
	}
}

func TestEggMessageListAndAcknowledge(t *testing.T) {
	db, err := InitDB(filepath.Join(t.TempDir(), "invasion.db"))
	if err != nil {
		t.Fatalf("InitDB: %v", err)
	}
	defer db.Close()

	msg, err := RecordEggMessage(db, EggMessageRecord{
		NestID:      "nest-1",
		EggID:       "egg-1",
		Title:       "Report ready",
		ArtifactIDs: []string{"artifact-1"},
	}, EggMessageRatePolicy{}, time.Now())
	if err != nil {
		t.Fatalf("RecordEggMessage: %v", err)
	}
	if err := AcknowledgeEggMessage(db, msg.ID, time.Now()); err != nil {
		t.Fatalf("AcknowledgeEggMessage: %v", err)
	}
	messages, err := ListEggMessages(db, EggMessageFilter{NestID: "nest-1", Limit: 10})
	if err != nil {
		t.Fatalf("ListEggMessages: %v", err)
	}
	if len(messages) != 1 {
		t.Fatalf("len(messages) = %d, want 1", len(messages))
	}
	if messages[0].AcknowledgedAt == "" {
		t.Fatalf("AcknowledgedAt should be set: %#v", messages[0])
	}
	if len(messages[0].ArtifactIDs) != 1 || messages[0].ArtifactIDs[0] != "artifact-1" {
		t.Fatalf("ArtifactIDs = %#v, want artifact-1", messages[0].ArtifactIDs)
	}
}
