package server

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"aurago/internal/agodesk"
	"aurago/internal/config"
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
