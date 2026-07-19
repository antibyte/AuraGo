package server

import (
	"archive/zip"
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"aurago/internal/agodesk"
	"aurago/internal/config"
	"aurago/internal/memory"
	"aurago/internal/services"
)

func TestAgoDeskKnowledgeArchiveUploadEndToEnd(t *testing.T) {
	s := newAgodeskPairingTestServer(t)
	knowledgeDir := t.TempDir()
	s.Cfg.Auth.SessionSecret = strings.Repeat("knowledge-upload-secret-", 3)
	s.Cfg.Indexing.Enabled = true
	s.Cfg.Indexing.Directories = []config.IndexingDirectory{{
		Path:       knowledgeDir,
		Collection: "agodesk-knowledge-test",
	}}
	s.Cfg.Indexing.Extensions = []string{".txt", ".md", ".json", ".pdf", ".docx", ".odt", ".rtf"}
	s.ShortTermMem = newAgodeskTestMemory(t)
	s.LongTermMem = &knowledgeUploadVectorDB{}
	s.FileIndexer = services.NewFileIndexer(s.Cfg, &s.CfgMu, s.LongTermMem, s.ShortTermMem, s.Logger)

	conn, cleanup, accepted := pairAgodeskTestClient(
		t,
		s,
		"knowledge-archive-upload-token",
		[]string{agodesk.CapabilityKnowledgeArchive},
	)
	defer cleanup()
	t.Cleanup(func() {
		closeAgodeskKnowledgeCoordinatorForTest(s)
	})

	if !agodeskTestContainsString(accepted.AdvertisedCapabilities, agodesk.CapabilityKnowledgeArchive) {
		t.Fatalf("advertised capabilities = %v, want %q", accepted.AdvertisedCapabilities, agodesk.CapabilityKnowledgeArchive)
	}
	if accepted.KnowledgeArchiveLimits == nil {
		t.Fatal("session.accepted is missing knowledge_archive_limits")
	}
	if accepted.KnowledgeArchiveLimits.MaxFileBytes != agodeskKnowledgeMaxFileBytes {
		t.Fatalf("max_file_bytes = %d, want %d", accepted.KnowledgeArchiveLimits.MaxFileBytes, agodeskKnowledgeMaxFileBytes)
	}

	body := []byte("AuraGo knowledge archive upload with semantic content.")
	prepare, err := agodesk.NewEnvelope(agodesk.TypeKnowledgeArchivePrepare, agodesk.KnowledgeArchivePreparePayload{
		SessionID: accepted.SessionID,
		Files: []agodesk.KnowledgeArchivePrepareFile{{
			Filename:  "archive-note.txt",
			MimeType:  "text/plain",
			SizeBytes: int64(len(body)),
			Title:     "Archive Note",
			Tags:      []string{" knowledge ", "Knowledge", "agodesk"},
		}},
	})
	if err != nil {
		t.Fatalf("NewEnvelope prepare: %v", err)
	}
	prepare.ID = "knowledge-prepare-fixed"
	if err := conn.WriteJSON(prepare); err != nil {
		t.Fatalf("write prepare: %v", err)
	}

	preparedEnvelope := readAgodeskTestEnvelope(t, conn)
	if preparedEnvelope.Type != agodesk.TypeKnowledgeArchivePrepared {
		t.Fatalf("prepare response type = %q, want %q", preparedEnvelope.Type, agodesk.TypeKnowledgeArchivePrepared)
	}
	var prepared agodesk.KnowledgeArchivePreparedPayload
	decodeAgodeskTestPayload(t, preparedEnvelope, &prepared)
	if len(prepared.Documents) != 1 || prepared.Documents[0].DocumentID == "" || prepared.Documents[0].UploadURL == "" {
		t.Fatalf("prepared payload = %+v", prepared)
	}
	firstDocumentID := prepared.Documents[0].DocumentID

	if err := conn.WriteJSON(prepare); err != nil {
		t.Fatalf("write idempotent prepare: %v", err)
	}
	replayedEnvelope := readAgodeskTestEnvelope(t, conn)
	var replayed agodesk.KnowledgeArchivePreparedPayload
	decodeAgodeskTestPayload(t, replayedEnvelope, &replayed)
	if len(replayed.Documents) != 1 || replayed.Documents[0].DocumentID != firstDocumentID {
		t.Fatalf("idempotent document_id = %+v, want %q", replayed.Documents, firstDocumentID)
	}

	httpServer := httptest.NewServer(handleAgodeskKnowledgeUpload(s))
	defer httpServer.Close()
	statusCode, responseBody := postAgodeskAttachmentTestUpload(
		t,
		httpServer.URL+prepared.Documents[0].UploadURL,
		agodeskKnowledgeUploadFormField,
		"archive-note.txt",
		body,
	)
	if statusCode != http.StatusCreated {
		t.Fatalf("upload status = %d, want %d; body=%s", statusCode, http.StatusCreated, responseBody)
	}

	if err := conn.SetReadDeadline(time.Now().Add(10 * time.Second)); err != nil {
		t.Fatalf("SetReadDeadline: %v", err)
	}
	states := make(map[string]agodesk.KnowledgeArchiveStatusPayload)
	for len(states) < 3 {
		statusEnvelope := readAgodeskTestEnvelope(t, conn)
		if statusEnvelope.Type != agodesk.TypeKnowledgeArchiveStatus {
			t.Fatalf("status response type = %q, want %q", statusEnvelope.Type, agodesk.TypeKnowledgeArchiveStatus)
		}
		var status agodesk.KnowledgeArchiveStatusPayload
		decodeAgodeskTestPayload(t, statusEnvelope, &status)
		if status.DocumentID != firstDocumentID {
			t.Fatalf("status document_id = %q, want %q", status.DocumentID, firstDocumentID)
		}
		states[status.State] = status
		if status.State == "ready" {
			break
		}
	}
	for _, state := range []string{"uploading", "processing", "ready"} {
		if _, ok := states[state]; !ok {
			t.Fatalf("status states = %v, missing %q", states, state)
		}
	}
	if states["ready"].ChunkCount != 1 {
		t.Fatalf("ready chunk_count = %d, want 1", states["ready"].ChunkCount)
	}

	storedPath := filepath.Join(knowledgeDir, "archive-note.txt")
	if raw, err := os.ReadFile(storedPath); err != nil || string(raw) != string(body) {
		t.Fatalf("stored document = %q, %v", raw, err)
	}
	metadata, err := s.ShortTermMem.GetFileIndexMetadata(storedPath, "agodesk-knowledge-test")
	if err != nil {
		t.Fatalf("GetFileIndexMetadata: %v", err)
	}
	if metadata["archive_document_id"] != firstDocumentID ||
		metadata["archive_title"] != "Archive Note" ||
		metadata["archive_tags"] != "knowledge, agodesk" ||
		metadata["source_channel"] != "agodesk" ||
		metadata["source_device_id"] != accepted.DeviceID {
		t.Fatalf("persisted archive metadata = %#v", metadata)
	}
}

func TestAgoDeskKnowledgeArchiveCapabilityAvailability(t *testing.T) {
	s := newAgodeskPairingTestServer(t)
	s.Cfg.Auth.SessionSecret = strings.Repeat("s", 64)
	s.Cfg.Indexing.Enabled = true
	s.Cfg.Indexing.Directories = []config.IndexingDirectory{{Path: t.TempDir()}}
	s.Cfg.Indexing.Extensions = []string{".txt"}
	s.ShortTermMem = newAgodeskTestMemory(t)
	s.LongTermMem = &knowledgeUploadVectorDB{}
	s.FileIndexer = services.NewFileIndexer(s.Cfg, &s.CfgMu, s.LongTermMem, s.ShortTermMem, s.Logger)

	if !agodeskTestContainsString(agodeskServerCapabilities(s), agodesk.CapabilityKnowledgeArchive) {
		t.Fatal("writable server did not advertise knowledge archive uploads")
	}
	if agodeskTestContainsString(agodeskServerCapabilitiesForDevice(s, true), agodesk.CapabilityKnowledgeArchive) {
		t.Fatal("read-only device received knowledge archive upload capability")
	}

	s.Cfg.Indexing.Enabled = false
	if agodeskTestContainsString(agodeskServerCapabilities(s), agodesk.CapabilityKnowledgeArchive) {
		t.Fatal("disabled indexing still advertised knowledge archive uploads")
	}
	s.Cfg.Indexing.Enabled = true
	s.Cfg.Auth.SessionSecret = ""
	if agodeskTestContainsString(agodeskServerCapabilities(s), agodesk.CapabilityKnowledgeArchive) {
		t.Fatal("missing signing secret still advertised knowledge archive uploads")
	}
}

func TestAgoDeskKnowledgeArchiveSecurityValidation(t *testing.T) {
	s := newAgodeskPairingTestServer(t)
	s.Cfg.Auth.SessionSecret = strings.Repeat("s", 64)
	s.Cfg.Indexing.Enabled = true
	s.Cfg.Indexing.Directories = []config.IndexingDirectory{{Path: t.TempDir()}}
	s.Cfg.Indexing.Extensions = []string{".txt", ".pdf"}
	s.ShortTermMem = newAgodeskTestMemory(t)
	s.LongTermMem = &knowledgeUploadVectorDB{}
	s.FileIndexer = services.NewFileIndexer(s.Cfg, &s.CfgMu, s.LongTermMem, s.ShortTermMem, s.Logger)

	directory := s.Cfg.Indexing.Directories[0]
	if _, _, code, _ := normalizeAgodeskKnowledgeBatch(s, directory, []agodesk.KnowledgeArchivePrepareFile{{
		Filename:  "../escape.txt",
		MimeType:  "text/plain",
		SizeBytes: 1,
	}}); code != agodesk.ErrorKnowledgeRejected {
		t.Fatalf("traversal error code = %q, want %q", code, agodesk.ErrorKnowledgeRejected)
	}
	if _, _, code, _ := normalizeAgodeskKnowledgeBatch(s, directory, []agodesk.KnowledgeArchivePrepareFile{{
		Filename:  "spoof.pdf",
		MimeType:  "text/plain",
		SizeBytes: 1,
	}}); code != agodesk.ErrorKnowledgeMimeNotAllowed {
		t.Fatalf("MIME mismatch error code = %q, want %q", code, agodesk.ErrorKnowledgeMimeNotAllowed)
	}

	fakePDF := filepath.Join(t.TempDir(), "spoof.pdf")
	if err := os.WriteFile(fakePDF, []byte("not a PDF"), 0o600); err != nil {
		t.Fatalf("WriteFile fake PDF: %v", err)
	}
	if _, err := validateAgodeskKnowledgeFile(fakePDF, "spoof.pdf", "application/pdf"); err == nil {
		t.Fatal("content-spoofed PDF was accepted")
	}
	disguisedExecutable := filepath.Join(t.TempDir(), "manual.txt")
	if err := os.WriteFile(disguisedExecutable, []byte{'M', 'Z', 0x90, 0x00}, 0o600); err != nil {
		t.Fatalf("WriteFile disguised executable: %v", err)
	}
	if _, err := validateAgodeskKnowledgeFile(disguisedExecutable, "manual.txt", "text/plain"); err == nil {
		t.Fatal("executable disguised as text was accepted")
	}

	expiredURL := signAgodeskKnowledgeUploadPath(s, "kdoc-expired", time.Now().Add(-time.Minute))
	req := httptest.NewRequest(http.MethodPost, expiredURL, nil)
	if verifyAgodeskMediaAssetSignature(s, req, time.Now()) {
		t.Fatal("expired knowledge upload signature was accepted")
	}
	rec := httptest.NewRecorder()
	handleAgodeskKnowledgeUpload(s).ServeHTTP(rec, req)
	if rec.Code != http.StatusGone || !strings.Contains(rec.Body.String(), agodesk.ErrorKnowledgeExpired) {
		t.Fatalf("expired upload response = %d %s, want 410 %s", rec.Code, rec.Body.String(), agodesk.ErrorKnowledgeExpired)
	}
	if !isAuthBypassed("/api/agodesk/knowledge/upload/kdoc-example") {
		t.Fatal("signed knowledge upload route is not exempt from browser session auth")
	}
	if isAuthBypassed("/api/knowledge/upload") {
		t.Fatal("existing knowledge API unexpectedly bypasses browser session auth")
	}
}

func TestAgoDeskKnowledgeArchiveLimitReturnsTooLarge(t *testing.T) {
	s := newAgodeskPairingTestServer(t)
	knowledgeDir := t.TempDir()
	s.Cfg.Auth.SessionSecret = strings.Repeat("s", 64)
	s.Cfg.Indexing.Enabled = true
	s.Cfg.Indexing.Directories = []config.IndexingDirectory{{Path: knowledgeDir}}
	s.Cfg.Indexing.Extensions = []string{".docx"}
	s.ShortTermMem = newAgodeskTestMemory(t)
	s.LongTermMem = &knowledgeUploadVectorDB{}
	s.FileIndexer = services.NewFileIndexer(s.Cfg, &s.CfgMu, s.LongTermMem, s.ShortTermMem, s.Logger)

	var archive bytes.Buffer
	writer := zip.NewWriter(&archive)
	member, err := writer.Create("word/document.xml")
	if err != nil {
		t.Fatalf("create DOCX member: %v", err)
	}
	if _, err := member.Write(make([]byte, (8<<20)+1)); err != nil {
		t.Fatalf("write DOCX member: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close DOCX: %v", err)
	}
	body := archive.Bytes()
	now := time.Now().UTC()
	record := memory.AgoDeskKnowledgeDocument{
		DocumentID:         "kdoc-archive-limit",
		PrepareID:          "prepare-archive-limit",
		PrepareFingerprint: "fingerprint",
		OwnerDeviceID:      "device-limit",
		Filename:           "oversized.docx",
		StoragePath:        filepath.Join(knowledgeDir, "oversized.docx"),
		Collection:         services.IndexerCollection,
		Title:              "Oversized",
		DeclaredMime:       "application/vnd.openxmlformats-officedocument.wordprocessingml.document",
		DeclaredSizeBytes:  int64(len(body)),
		CreatedAt:          now,
		ExpiresAt:          now.Add(agodeskKnowledgePrepareTTL),
	}
	if err := s.ShortTermMem.PrepareAgoDeskKnowledgeBatch([]memory.AgoDeskKnowledgeDocument{record}); err != nil {
		t.Fatalf("PrepareAgoDeskKnowledgeBatch: %v", err)
	}
	t.Cleanup(func() { closeAgodeskKnowledgeCoordinatorForTest(s) })
	httpServer := httptest.NewServer(handleAgodeskKnowledgeUpload(s))
	defer httpServer.Close()

	uploadURL := signAgodeskKnowledgeUploadPath(s, record.DocumentID, record.ExpiresAt)
	statusCode, responseBody := postAgodeskAttachmentTestUpload(
		t,
		httpServer.URL+uploadURL,
		agodeskKnowledgeUploadFormField,
		record.Filename,
		body,
	)
	if statusCode != http.StatusRequestEntityTooLarge || !strings.Contains(responseBody, agodesk.ErrorKnowledgeTooLarge) {
		t.Fatalf("upload response = %d %s, want 413 %s", statusCode, responseBody, agodesk.ErrorKnowledgeTooLarge)
	}
	if _, err := os.Stat(record.StoragePath); !os.IsNotExist(err) {
		t.Fatalf("oversized document was published: %v", err)
	}
	got, err := s.ShortTermMem.GetAgoDeskKnowledgeDocument(record.DocumentID)
	if err != nil {
		t.Fatalf("GetAgoDeskKnowledgeDocument: %v", err)
	}
	if got == nil || got.Status != memory.AgoDeskKnowledgeStatusFailed || got.ErrorCode != agodesk.ErrorKnowledgeTooLarge {
		t.Fatalf("oversized document status = %+v", got)
	}
}

func TestAgoDeskKnowledgeActiveUploadSurvivesPrepareExpiryAndPartCleanup(t *testing.T) {
	s := newAgodeskPairingTestServer(t)
	knowledgeDir := t.TempDir()
	s.Cfg.Auth.SessionSecret = strings.Repeat("s", 64)
	s.Cfg.Indexing.Enabled = true
	s.Cfg.Indexing.Directories = []config.IndexingDirectory{{Path: knowledgeDir}}
	s.Cfg.Indexing.Extensions = []string{".txt"}
	s.ShortTermMem = newAgodeskTestMemory(t)
	s.LongTermMem = &knowledgeUploadVectorDB{}
	s.FileIndexer = services.NewFileIndexer(s.Cfg, &s.CfgMu, s.LongTermMem, s.ShortTermMem, s.Logger)

	now := time.Now().UTC()
	record := memory.AgoDeskKnowledgeDocument{
		DocumentID:         "kdoc-active-cleanup",
		PrepareID:          "prepare-active-cleanup",
		PrepareFingerprint: "fingerprint",
		OwnerDeviceID:      "device-active",
		Filename:           "active.txt",
		StoragePath:        filepath.Join(knowledgeDir, "active.txt"),
		Collection:         services.IndexerCollection,
		Title:              "Active",
		DeclaredMime:       "text/plain",
		DeclaredSizeBytes:  1,
		CreatedAt:          now.Add(-10 * time.Minute),
		ExpiresAt:          now.Add(-time.Minute),
	}
	if err := s.ShortTermMem.PrepareAgoDeskKnowledgeBatch([]memory.AgoDeskKnowledgeDocument{record}); err != nil {
		t.Fatalf("PrepareAgoDeskKnowledgeBatch: %v", err)
	}
	coordinator := ensureAgodeskKnowledgeCoordinator(s)
	t.Cleanup(func() { closeAgodeskKnowledgeCoordinatorForTest(s) })
	if coordinator == nil {
		t.Fatal("knowledge coordinator was not created")
	}
	if _, err := coordinator.claimUpload(record.DocumentID, record.ExpiresAt.Add(-time.Minute)); err != nil {
		t.Fatalf("claimUpload: %v", err)
	}

	partPath := filepath.Join(knowledgeDir, agodeskKnowledgePartPrefix+record.DocumentID+"-stream.part")
	if err := os.WriteFile(partPath, []byte("x"), 0o600); err != nil {
		t.Fatalf("WriteFile part: %v", err)
	}
	old := now.Add(-2 * agodeskKnowledgePrepareTTL)
	if err := os.Chtimes(partPath, old, old); err != nil {
		t.Fatalf("Chtimes part: %v", err)
	}
	coordinator.maintain(now)
	got, err := s.ShortTermMem.GetAgoDeskKnowledgeDocument(record.DocumentID)
	if err != nil {
		t.Fatalf("GetAgoDeskKnowledgeDocument active: %v", err)
	}
	if got == nil || got.Status != memory.AgoDeskKnowledgeStatusUploading {
		t.Fatalf("active document = %+v, want uploading", got)
	}
	if _, err := os.Stat(partPath); err != nil {
		t.Fatalf("active part file was removed: %v", err)
	}
	retryURL := signAgodeskKnowledgeUploadPath(s, record.DocumentID, now.Add(time.Minute))
	retryRequest := httptest.NewRequest(http.MethodPost, retryURL, nil)
	retryResponse := httptest.NewRecorder()
	handleAgodeskKnowledgeUpload(s).ServeHTTP(retryResponse, retryRequest)
	if retryResponse.Code != http.StatusConflict {
		t.Fatalf("active upload retry status = %d, want %d; body=%s", retryResponse.Code, http.StatusConflict, retryResponse.Body.String())
	}
	got, err = s.ShortTermMem.GetAgoDeskKnowledgeDocument(record.DocumentID)
	if err != nil {
		t.Fatalf("GetAgoDeskKnowledgeDocument after retry: %v", err)
	}
	if got == nil || got.Status != memory.AgoDeskKnowledgeStatusUploading {
		t.Fatalf("active retry changed document state: %+v", got)
	}

	coordinator.releaseUpload(record.DocumentID)
	coordinator.reconcileUploading()
	got, err = s.ShortTermMem.GetAgoDeskKnowledgeDocument(record.DocumentID)
	if err != nil {
		t.Fatalf("GetAgoDeskKnowledgeDocument orphaned: %v", err)
	}
	if got == nil || got.Status != memory.AgoDeskKnowledgeStatusFailed || got.ErrorCode != agodesk.ErrorKnowledgeExpired {
		t.Fatalf("orphaned document = %+v, want failed/%s", got, agodesk.ErrorKnowledgeExpired)
	}
	coordinator.cleanupPartFiles(now)
	if _, err := os.Stat(partPath); !os.IsNotExist(err) {
		t.Fatalf("orphaned part file still exists: %v", err)
	}
}

func TestAgoDeskKnowledgeCoordinatorFailsOrphanedUploadOnStart(t *testing.T) {
	s := newAgodeskPairingTestServer(t)
	knowledgeDir := t.TempDir()
	s.Cfg.Auth.SessionSecret = strings.Repeat("s", 64)
	s.Cfg.Indexing.Enabled = true
	s.Cfg.Indexing.Directories = []config.IndexingDirectory{{Path: knowledgeDir}}
	s.Cfg.Indexing.Extensions = []string{".txt"}
	s.ShortTermMem = newAgodeskTestMemory(t)
	s.LongTermMem = &knowledgeUploadVectorDB{}
	s.FileIndexer = services.NewFileIndexer(s.Cfg, &s.CfgMu, s.LongTermMem, s.ShortTermMem, s.Logger)

	now := time.Now().UTC()
	record := memory.AgoDeskKnowledgeDocument{
		DocumentID:         "kdoc-orphaned-start",
		PrepareID:          "prepare-orphaned-start",
		PrepareFingerprint: "fingerprint",
		OwnerDeviceID:      "device-orphaned",
		Filename:           "orphaned.txt",
		StoragePath:        filepath.Join(knowledgeDir, "orphaned.txt"),
		Collection:         services.IndexerCollection,
		Title:              "Orphaned",
		DeclaredMime:       "text/plain",
		DeclaredSizeBytes:  1,
		CreatedAt:          now,
		ExpiresAt:          now.Add(agodeskKnowledgePrepareTTL),
	}
	if err := s.ShortTermMem.PrepareAgoDeskKnowledgeBatch([]memory.AgoDeskKnowledgeDocument{record}); err != nil {
		t.Fatalf("PrepareAgoDeskKnowledgeBatch: %v", err)
	}
	if _, err := s.ShortTermMem.MarkAgoDeskKnowledgeUploading(record.DocumentID, now); err != nil {
		t.Fatalf("MarkAgoDeskKnowledgeUploading: %v", err)
	}
	if ensureAgodeskKnowledgeCoordinator(s) == nil {
		t.Fatal("knowledge coordinator was not created")
	}
	t.Cleanup(func() { closeAgodeskKnowledgeCoordinatorForTest(s) })

	deadline := time.Now().Add(2 * time.Second)
	for {
		got, err := s.ShortTermMem.GetAgoDeskKnowledgeDocument(record.DocumentID)
		if err != nil {
			t.Fatalf("GetAgoDeskKnowledgeDocument: %v", err)
		}
		if got != nil && got.Status == memory.AgoDeskKnowledgeStatusFailed {
			if got.ErrorCode != agodesk.ErrorKnowledgeExpired {
				t.Fatalf("orphaned error code = %q, want %q", got.ErrorCode, agodesk.ErrorKnowledgeExpired)
			}
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("orphaned upload did not fail: %+v", got)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func TestAgoDeskKnowledgeExpiryAndUploadClaimAreSerialized(t *testing.T) {
	s := newAgodeskPairingTestServer(t)
	knowledgeDir := t.TempDir()
	s.Cfg.Indexing.Enabled = true
	s.Cfg.Indexing.Directories = []config.IndexingDirectory{{Path: knowledgeDir}}
	s.ShortTermMem = newAgodeskTestMemory(t)
	coordinator := &agodeskKnowledgeCoordinator{
		server: s,
		ctx:    context.Background(),
		active: make(map[string]struct{}),
	}

	for attempt := 0; attempt < 50; attempt++ {
		now := time.Now().UTC()
		documentID := "kdoc-expiry-race-" + strconv.Itoa(attempt)
		record := memory.AgoDeskKnowledgeDocument{
			DocumentID:         documentID,
			PrepareID:          "prepare-expiry-race-" + strconv.Itoa(attempt),
			PrepareFingerprint: "fingerprint",
			OwnerDeviceID:      "device-race",
			Filename:           "race-" + strconv.Itoa(attempt) + ".txt",
			StoragePath:        filepath.Join(knowledgeDir, "race-"+strconv.Itoa(attempt)+".txt"),
			Collection:         services.IndexerCollection,
			Title:              "Race",
			DeclaredMime:       "text/plain",
			DeclaredSizeBytes:  1,
			CreatedAt:          now.Add(-10 * time.Minute),
			ExpiresAt:          now.Add(-time.Millisecond),
		}
		if err := s.ShortTermMem.PrepareAgoDeskKnowledgeBatch([]memory.AgoDeskKnowledgeDocument{record}); err != nil {
			t.Fatalf("attempt %d PrepareAgoDeskKnowledgeBatch: %v", attempt, err)
		}

		start := make(chan struct{})
		claimResult := make(chan error, 1)
		var wait sync.WaitGroup
		wait.Add(2)
		go func() {
			defer wait.Done()
			<-start
			_, err := coordinator.claimUpload(documentID, now.Add(-2*time.Millisecond))
			claimResult <- err
		}()
		go func() {
			defer wait.Done()
			<-start
			coordinator.maintain(now)
		}()
		close(start)
		wait.Wait()
		claimErr := <-claimResult

		got, err := s.ShortTermMem.GetAgoDeskKnowledgeDocument(documentID)
		if err != nil {
			t.Fatalf("attempt %d GetAgoDeskKnowledgeDocument: %v", attempt, err)
		}
		if claimErr == nil {
			if got == nil || got.Status != memory.AgoDeskKnowledgeStatusUploading {
				t.Fatalf("attempt %d successful claim raced into state %+v", attempt, got)
			}
			coordinator.releaseUpload(documentID)
			if _, err := s.ShortTermMem.MarkAgoDeskKnowledgeFailed(documentID, agodesk.ErrorKnowledgeExpired, "test cleanup", now); err != nil {
				t.Fatalf("attempt %d cleanup failed: %v", attempt, err)
			}
			continue
		}
		if got == nil || got.Status != memory.AgoDeskKnowledgeStatusFailed {
			t.Fatalf("attempt %d rejected claim left state %+v; error=%v", attempt, got, claimErr)
		}
	}
}

func TestAgoDeskKnowledgeShutdownPreventsCoordinatorRecreation(t *testing.T) {
	s := newAgodeskPairingTestServer(t)
	rootCtx, cancel := context.WithCancel(context.Background())
	defer cancel()
	configureAgodeskKnowledgeLifecycle(s, rootCtx)
	if ensureAgodeskKnowledgeCoordinator(s) == nil {
		t.Fatal("knowledge coordinator was not created")
	}

	s.closeRuntimeResources()
	if coordinator := ensureAgodeskKnowledgeCoordinator(s); coordinator != nil {
		coordinator.Close()
		t.Fatal("knowledge coordinator was recreated after shutdown")
	}
	broker := currentAgodeskDesktopBroker(s)
	if broker == nil {
		t.Fatal("AgoDesk broker is missing")
	}
	broker.knowledgeMu.Lock()
	defer broker.knowledgeMu.Unlock()
	if !broker.knowledgeClosed || broker.knowledge != nil {
		t.Fatalf("knowledge lifecycle after shutdown: closed=%v coordinator=%p", broker.knowledgeClosed, broker.knowledge)
	}
}

func TestAgoDeskKnowledgeProcessingRejectsWhitespaceWithoutVectors(t *testing.T) {
	s := newAgodeskPairingTestServer(t)
	knowledgeDir := t.TempDir()
	s.Cfg.Auth.SessionSecret = strings.Repeat("s", 64)
	s.Cfg.Indexing.Enabled = true
	s.Cfg.Indexing.Directories = []config.IndexingDirectory{{Path: knowledgeDir}}
	s.Cfg.Indexing.Extensions = []string{".txt"}
	s.ShortTermMem = newAgodeskTestMemory(t)
	s.LongTermMem = &knowledgeUploadVectorDB{}
	s.FileIndexer = services.NewFileIndexer(s.Cfg, &s.CfgMu, s.LongTermMem, s.ShortTermMem, s.Logger)

	now := time.Now().UTC()
	path := filepath.Join(knowledgeDir, "whitespace.txt")
	body := []byte(" \r\n\t ")
	if err := os.WriteFile(path, body, 0o640); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	record := memory.AgoDeskKnowledgeDocument{
		DocumentID:         "kdoc-whitespace",
		PrepareID:          "prepare-whitespace",
		PrepareFingerprint: "fingerprint",
		OwnerDeviceID:      "device-whitespace",
		Filename:           filepath.Base(path),
		StoragePath:        path,
		Collection:         services.IndexerCollection,
		Title:              "Whitespace",
		DeclaredMime:       "text/plain",
		DeclaredSizeBytes:  int64(len(body)),
		CreatedAt:          now,
		ExpiresAt:          now.Add(agodeskKnowledgePrepareTTL),
	}
	if err := s.ShortTermMem.PrepareAgoDeskKnowledgeBatch([]memory.AgoDeskKnowledgeDocument{record}); err != nil {
		t.Fatalf("PrepareAgoDeskKnowledgeBatch: %v", err)
	}
	if _, err := s.ShortTermMem.MarkAgoDeskKnowledgeUploading(record.DocumentID, now); err != nil {
		t.Fatalf("MarkAgoDeskKnowledgeUploading: %v", err)
	}
	if _, err := s.ShortTermMem.MarkAgoDeskKnowledgeProcessing(record.DocumentID, "text/plain", int64(len(body)), "hash", now); err != nil {
		t.Fatalf("MarkAgoDeskKnowledgeProcessing: %v", err)
	}
	coordinator := &agodeskKnowledgeCoordinator{server: s, ctx: context.Background()}
	coordinator.process(record.DocumentID)

	got, err := s.ShortTermMem.GetAgoDeskKnowledgeDocument(record.DocumentID)
	if err != nil {
		t.Fatalf("GetAgoDeskKnowledgeDocument: %v", err)
	}
	if got == nil || got.Status != memory.AgoDeskKnowledgeStatusFailed || got.ErrorCode != agodesk.ErrorKnowledgeIngestFailed {
		t.Fatalf("whitespace document = %+v, want failed/%s", got, agodesk.ErrorKnowledgeIngestFailed)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("failed document file still exists: %v", err)
	}
}

func TestSanitizeAgoDeskKnowledgeErrorHidesInternalDetails(t *testing.T) {
	for _, message := range []string{
		`provider failed for C:\AuraGo\data\knowledge\private.txt`,
		"provider failed for /srv/aurago/data/knowledge/private.txt",
		"api_key=sk-test-secret-value",
		"upstream https://provider.invalid/v1 returned diagnostics",
	} {
		if got := sanitizeAgodeskKnowledgeError(message); got != "Knowledge ingest failed." {
			t.Fatalf("sanitizeAgodeskKnowledgeError(%q) = %q", message, got)
		}
	}
	const controlled = "Uploaded document exceeds safe archive extraction limits."
	if got := sanitizeAgodeskKnowledgeError(controlled); got != controlled {
		t.Fatalf("controlled error = %q, want %q", got, controlled)
	}
}

func closeAgodeskKnowledgeCoordinatorForTest(s *Server) {
	broker := currentAgodeskDesktopBroker(s)
	if broker == nil {
		return
	}
	broker.knowledgeMu.Lock()
	coordinator := broker.knowledge
	broker.knowledge = nil
	broker.knowledgeMu.Unlock()
	if coordinator != nil {
		coordinator.Close()
	}
}
