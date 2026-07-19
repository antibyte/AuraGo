package server

import (
	"bufio"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"aurago/internal/agodesk"
	"aurago/internal/config"
	"aurago/internal/memory"
	"aurago/internal/security"
	"aurago/internal/services"
	"aurago/internal/uid"

	"github.com/gorilla/websocket"
)

const (
	agodeskKnowledgeMaxFileBytes      int64 = 20 << 20
	agodeskKnowledgeMaxFilesPerBatch        = 10
	agodeskKnowledgeMaxFilenameRunes        = 255
	agodeskKnowledgeMaxTitleRunes           = 255
	agodeskKnowledgeMaxTags                 = 32
	agodeskKnowledgeMaxTagRunes             = 64
	agodeskKnowledgePrepareTTL              = 5 * time.Minute
	agodeskKnowledgeTerminalReplayAge       = 24 * time.Hour
	agodeskKnowledgeUploadFormField         = "file"
	agodeskKnowledgeUploadPathPrefix        = "/api/agodesk/knowledge/upload/"
	agodeskKnowledgePartPrefix              = ".agodesk-knowledge-"
)

var (
	errAgodeskKnowledgeUploadExpired = errors.New("knowledge upload reservation expired")
	errAgodeskKnowledgeUploadState   = errors.New("knowledge upload is not prepared")
)

type agodeskKnowledgeUploadResponse struct {
	DocumentID string `json:"document_id"`
	State      string `json:"state"`
	Filename   string `json:"filename"`
	MimeType   string `json:"mime_type"`
	SizeBytes  int64  `json:"size_bytes"`
}

type agodeskKnowledgeHTTPError struct {
	Code       string `json:"code"`
	Message    string `json:"message"`
	DocumentID string `json:"document_id,omitempty"`
}

type agodeskKnowledgeCoordinator struct {
	server    *Server
	ctx       context.Context
	cancel    context.CancelFunc
	jobs      chan string
	wg        sync.WaitGroup
	closeOnce sync.Once
	publishMu sync.Mutex
	queueMu   sync.Mutex
	queued    map[string]struct{}
	activeMu  sync.Mutex
	active    map[string]struct{}
}

type normalizedKnowledgePrepareFile struct {
	Payload     agodesk.KnowledgeArchivePrepareFile
	StoragePath string
}

func newAgodeskKnowledgeCoordinator(s *Server, parent context.Context) *agodeskKnowledgeCoordinator {
	if parent == nil {
		parent = context.Background()
	}
	ctx, cancel := context.WithCancel(parent)
	coordinator := &agodeskKnowledgeCoordinator{
		server: s,
		ctx:    ctx,
		cancel: cancel,
		jobs:   make(chan string, 64),
		queued: make(map[string]struct{}),
		active: make(map[string]struct{}),
	}
	coordinator.wg.Add(2)
	go coordinator.runWorker()
	go coordinator.runMaintenance()
	return coordinator
}

func ensureAgodeskKnowledgeCoordinator(s *Server) *agodeskKnowledgeCoordinator {
	return ensureAgodeskKnowledgeCoordinatorWithContext(s, nil)
}

func configureAgodeskKnowledgeLifecycle(s *Server, parent context.Context) {
	if parent == nil {
		return
	}
	broker := ensureAgodeskDesktopBroker(s)
	if broker == nil {
		return
	}
	broker.knowledgeMu.Lock()
	defer broker.knowledgeMu.Unlock()
	if broker.knowledgeClosed || parent.Err() != nil {
		return
	}
	broker.knowledgeCtx = parent
}

func ensureAgodeskKnowledgeCoordinatorWithContext(s *Server, parent context.Context) *agodeskKnowledgeCoordinator {
	broker := ensureAgodeskDesktopBroker(s)
	if broker == nil {
		return nil
	}
	broker.knowledgeMu.Lock()
	defer broker.knowledgeMu.Unlock()
	if broker.knowledgeClosed {
		return nil
	}
	if parent != nil {
		if parent.Err() != nil {
			return nil
		}
		broker.knowledgeCtx = parent
	}
	rootCtx := broker.knowledgeCtx
	if rootCtx == nil {
		rootCtx = context.Background()
	}
	if rootCtx.Err() != nil {
		return nil
	}
	if broker.knowledge == nil {
		broker.knowledge = newAgodeskKnowledgeCoordinator(s, rootCtx)
	}
	return broker.knowledge
}

func (c *agodeskKnowledgeCoordinator) Close() {
	if c == nil {
		return
	}
	c.closeOnce.Do(func() {
		c.cancel()
		c.wg.Wait()
	})
}

func (c *agodeskKnowledgeCoordinator) enqueue(documentID string) bool {
	if c == nil {
		return false
	}
	documentID = strings.TrimSpace(documentID)
	if documentID == "" {
		return false
	}
	c.queueMu.Lock()
	if _, exists := c.queued[documentID]; exists {
		c.queueMu.Unlock()
		return true
	}
	c.queued[documentID] = struct{}{}
	c.queueMu.Unlock()
	select {
	case c.jobs <- documentID:
		return true
	case <-c.ctx.Done():
		c.queueMu.Lock()
		delete(c.queued, documentID)
		c.queueMu.Unlock()
		return false
	default:
		c.queueMu.Lock()
		delete(c.queued, documentID)
		c.queueMu.Unlock()
		return false
	}
}

func (c *agodeskKnowledgeCoordinator) runWorker() {
	defer c.wg.Done()
	for {
		select {
		case <-c.ctx.Done():
			return
		case documentID := <-c.jobs:
			c.process(documentID)
			c.queueMu.Lock()
			delete(c.queued, documentID)
			c.queueMu.Unlock()
		}
	}
}

func (c *agodeskKnowledgeCoordinator) runMaintenance() {
	defer c.wg.Done()
	c.reconcileUploading()
	c.recoverProcessing()
	c.maintain(time.Now().UTC())
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-c.ctx.Done():
			return
		case now := <-ticker.C:
			c.recoverProcessing()
			c.maintain(now.UTC())
		}
	}
}

func (c *agodeskKnowledgeCoordinator) recoverProcessing() {
	if c == nil || c.server == nil || c.server.ShortTermMem == nil {
		return
	}
	records, err := c.server.ShortTermMem.ListAgoDeskKnowledgeProcessing()
	if err != nil {
		c.server.Logger.Warn("Failed to recover AgoDesk knowledge jobs", "error", scrubAgodeskKnowledgeLogError(err))
		return
	}
	for _, record := range records {
		_ = c.enqueue(record.DocumentID)
	}
}

func (c *agodeskKnowledgeCoordinator) maintain(now time.Time) {
	if c == nil || c.server == nil || c.server.ShortTermMem == nil {
		return
	}
	c.activeMu.Lock()
	expired, err := c.server.ShortTermMem.ExpireAgoDeskKnowledgeDocuments(now, agodesk.ErrorKnowledgeExpired)
	c.activeMu.Unlock()
	if err != nil {
		c.server.Logger.Warn("Failed to expire AgoDesk knowledge uploads", "error", scrubAgodeskKnowledgeLogError(err))
	} else {
		for _, record := range expired {
			c.emit(record)
		}
	}
	c.reconcileUploading()
	if _, err := c.server.ShortTermMem.CleanupAgoDeskKnowledgeDocuments(now, agodesk.ErrorKnowledgeExpired); err != nil {
		c.server.Logger.Warn("Failed to clean up AgoDesk knowledge uploads", "error", scrubAgodeskKnowledgeLogError(err))
	}
	c.cleanupPartFiles(now)
}

func (c *agodeskKnowledgeCoordinator) cleanupPartFiles(now time.Time) {
	directory, ok := agodeskKnowledgeDirectory(c.server)
	if !ok {
		return
	}
	entries, err := os.ReadDir(directory.Path)
	if err != nil {
		return
	}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasPrefix(entry.Name(), agodeskKnowledgePartPrefix) || !strings.HasSuffix(entry.Name(), ".part") {
			continue
		}
		if c.partFileBelongsToActiveUpload(entry.Name()) {
			continue
		}
		info, err := entry.Info()
		if err != nil || info.Mode()&os.ModeSymlink != 0 || now.Sub(info.ModTime()) < agodeskKnowledgePrepareTTL {
			continue
		}
		target := filepath.Join(directory.Path, entry.Name())
		if pathStaysWithinDir(directory.Path, target) {
			_ = os.Remove(target)
		}
	}
}

func (c *agodeskKnowledgeCoordinator) claimUpload(documentID string, now time.Time) (*memory.AgoDeskKnowledgeDocument, error) {
	c.activeMu.Lock()
	defer c.activeMu.Unlock()
	if err := c.ctx.Err(); err != nil {
		return nil, err
	}
	record, err := c.server.ShortTermMem.GetAgoDeskKnowledgeDocument(documentID)
	if err != nil {
		return nil, err
	}
	if record == nil || record.Status != memory.AgoDeskKnowledgeStatusPrepared {
		return record, errAgodeskKnowledgeUploadState
	}
	if !record.ExpiresAt.IsZero() && now.After(record.ExpiresAt) {
		failed, failErr := c.server.ShortTermMem.MarkAgoDeskKnowledgeFailed(
			documentID,
			agodesk.ErrorKnowledgeExpired,
			"Knowledge upload reservation expired.",
			now,
		)
		if failErr != nil {
			return nil, failErr
		}
		return failed, errAgodeskKnowledgeUploadExpired
	}
	record, err = c.server.ShortTermMem.MarkAgoDeskKnowledgeUploading(documentID, now)
	if err != nil {
		return nil, err
	}
	c.active[documentID] = struct{}{}
	return record, nil
}

func (c *agodeskKnowledgeCoordinator) releaseUpload(documentID string) {
	c.activeMu.Lock()
	delete(c.active, strings.TrimSpace(documentID))
	c.activeMu.Unlock()
}

func (c *agodeskKnowledgeCoordinator) reconcileUploading() {
	if c == nil || c.server == nil || c.server.ShortTermMem == nil {
		return
	}
	c.activeMu.Lock()
	defer c.activeMu.Unlock()
	records, err := c.server.ShortTermMem.ListAgoDeskKnowledgeUploading()
	if err != nil {
		c.server.Logger.Warn("Failed to inspect orphaned AgoDesk knowledge uploads", "error", scrubAgodeskKnowledgeLogError(err))
		return
	}
	for _, record := range records {
		if _, active := c.active[record.DocumentID]; active {
			continue
		}
		failed, failErr := c.server.ShortTermMem.MarkAgoDeskKnowledgeFailed(
			record.DocumentID,
			agodesk.ErrorKnowledgeExpired,
			"Knowledge upload did not complete.",
			time.Now().UTC(),
		)
		if failErr != nil {
			c.server.Logger.Warn("Failed to expire orphaned AgoDesk knowledge upload", "document_id", record.DocumentID, "error", scrubAgodeskKnowledgeLogError(failErr))
			continue
		}
		c.emit(*failed)
	}
}

func (c *agodeskKnowledgeCoordinator) partFileBelongsToActiveUpload(name string) bool {
	c.activeMu.Lock()
	defer c.activeMu.Unlock()
	for documentID := range c.active {
		if strings.Contains(name, documentID) {
			return true
		}
	}
	return false
}

func (c *agodeskKnowledgeCoordinator) process(documentID string) {
	if c == nil || c.server == nil || c.server.ShortTermMem == nil {
		return
	}
	record, err := c.server.ShortTermMem.GetAgoDeskKnowledgeDocument(documentID)
	if err != nil || record == nil || record.Status != memory.AgoDeskKnowledgeStatusProcessing {
		return
	}
	if err := c.ctx.Err(); err != nil {
		return
	}
	directory, ok := agodeskKnowledgeDirectory(c.server)
	if !ok || c.server.FileIndexer == nil {
		c.failProcessing(*record, "Knowledge indexing is unavailable.")
		return
	}
	resolvedPath, err := filepath.Abs(record.StoragePath)
	if err != nil || !pathStaysWithinDir(directory.Path, resolvedPath) {
		c.failProcessing(*record, "Knowledge document path is no longer valid.")
		return
	}
	if record.Collection != agodeskKnowledgeCollection(directory) {
		c.failProcessing(*record, "Knowledge collection changed before indexing completed.")
		return
	}
	metadata := map[string]string{
		"archive_document_id": record.DocumentID,
		"archive_title":       record.Title,
		"archive_tags":        strings.Join(record.Tags, ", "),
		"source_channel":      "agodesk",
		"source_device_id":    record.OwnerDeviceID,
	}
	result, err := c.server.FileIndexer.IndexFile(c.ctx, directory, resolvedPath, metadata)
	if err != nil {
		if c.ctx.Err() != nil {
			return
		}
		c.server.Logger.Warn(
			"AgoDesk knowledge indexing failed",
			"document_id", record.DocumentID,
			"error", scrubAgodeskKnowledgeLogError(err),
		)
		c.failProcessing(*record, "Knowledge document could not be indexed.")
		return
	}
	if !result.Indexed || result.ChunkCount <= 0 || len(result.DocumentIDs) == 0 {
		c.server.Logger.Warn("AgoDesk knowledge indexing produced no vectors", "document_id", record.DocumentID)
		c.failProcessing(*record, "Knowledge document contains no indexable content.")
		return
	}
	ready, err := c.server.ShortTermMem.MarkAgoDeskKnowledgeReady(record.DocumentID, result.ChunkCount, time.Now().UTC())
	if err != nil {
		c.server.Logger.Warn("Failed to mark AgoDesk knowledge document ready", "document_id", record.DocumentID, "error", scrubAgodeskKnowledgeLogError(err))
		return
	}
	c.emit(*ready)
}

func (c *agodeskKnowledgeCoordinator) failProcessing(record memory.AgoDeskKnowledgeDocument, message string) {
	c.rollbackIndexedDocument(record)
	_ = os.Remove(record.StoragePath)
	failed, err := c.server.ShortTermMem.MarkAgoDeskKnowledgeFailed(
		record.DocumentID,
		agodesk.ErrorKnowledgeIngestFailed,
		sanitizeAgodeskKnowledgeError(message),
		time.Now().UTC(),
	)
	if err != nil {
		c.server.Logger.Warn("Failed to mark AgoDesk knowledge document failed", "document_id", record.DocumentID, "error", scrubAgodeskKnowledgeLogError(err))
		return
	}
	c.emit(*failed)
}

func (c *agodeskKnowledgeCoordinator) rollbackIndexedDocument(record memory.AgoDeskKnowledgeDocument) {
	if c == nil || c.server == nil || c.server.ShortTermMem == nil {
		return
	}
	docIDs, err := c.server.ShortTermMem.GetFileEmbeddingDocIDs(record.StoragePath, record.Collection)
	if err == nil && c.server.LongTermMem != nil {
		for _, docID := range docIDs {
			if deleteErr := c.server.LongTermMem.DeleteDocumentFromCollection(docID, record.Collection); deleteErr != nil {
				c.server.Logger.Warn("Failed to roll back knowledge embedding", "document_id", record.DocumentID, "vector_id", docID, "error", scrubAgodeskKnowledgeLogError(deleteErr))
			}
		}
		_ = c.server.ShortTermMem.DeleteMemoryMetaBatch(docIDs)
	}
	_ = c.server.ShortTermMem.DeleteFileIndex(record.StoragePath, record.Collection)
	_ = c.server.ShortTermMem.DeleteFileIndexMetadata(record.StoragePath, record.Collection)
}

func (c *agodeskKnowledgeCoordinator) emit(record memory.AgoDeskKnowledgeDocument) {
	broker := currentAgodeskDesktopBroker(c.server)
	if broker == nil {
		return
	}
	session := broker.session(record.OwnerDeviceID)
	if session == nil || !session.hasCapability(agodesk.CapabilityKnowledgeArchive) {
		return
	}
	_ = writeAgodeskEnvelopeLocked(session.conn, session.state, agodesk.TypeKnowledgeArchiveStatus, agodeskKnowledgeStatusPayload(record, agodeskSessionTransportID(session)))
}

func replayAgodeskKnowledgeStatuses(s *Server, deviceID string) {
	coordinator := ensureAgodeskKnowledgeCoordinator(s)
	if coordinator == nil || s == nil || s.ShortTermMem == nil {
		return
	}
	records, err := s.ShortTermMem.ListAgoDeskKnowledgeReplay(deviceID, time.Now().UTC().Add(-agodeskKnowledgeTerminalReplayAge))
	if err != nil {
		s.Logger.Warn("Failed to replay AgoDesk knowledge statuses", "device_id", deviceID, "error", err)
		return
	}
	for _, record := range records {
		coordinator.emit(record)
	}
}

func handleAgodeskKnowledgePrepare(s *Server, conn *websocket.Conn, state *agodeskConnectionState, requestID string, payload agodesk.KnowledgeArchivePreparePayload) {
	transportSessionID, ok := validateAgodeskTransportSession(s, conn, state, requestID, payload.SessionID, "knowledge.archive.prepare")
	if !ok || !validateAgodeskCapability(conn, state, requestID, agodesk.CapabilityKnowledgeArchive, "knowledge.archive.prepare") {
		return
	}
	deviceID, paired := agodeskStateDevice(state)
	if !paired || deviceID == "" || agodeskStateReadOnly(state) {
		_ = writeAgodeskErrorLocked(conn, state, requestID, agodesk.ErrorRemoteReadOnly, "Knowledge archive uploads require a writable paired device.")
		return
	}
	coordinator := ensureAgodeskKnowledgeCoordinator(s)
	if coordinator == nil || !agodeskKnowledgeArchiveUploadsEnabled(s) {
		_ = writeAgodeskErrorLocked(conn, state, requestID, agodesk.ErrorUnsupportedCapability, "Knowledge archive uploads are unavailable.")
		return
	}
	directory, ok := agodeskKnowledgeDirectory(s)
	if !ok {
		_ = writeAgodeskErrorLocked(conn, state, requestID, agodesk.ErrorKnowledgeRejected, "Knowledge storage is not configured.")
		return
	}
	normalized, fingerprint, code, message := normalizeAgodeskKnowledgeBatch(s, directory, payload.Files)
	if code != "" {
		_ = writeAgodeskErrorLocked(conn, state, requestID, code, message)
		return
	}

	existing, err := s.ShortTermMem.ListAgoDeskKnowledgeByPrepare(deviceID, requestID)
	if err != nil {
		_ = writeAgodeskErrorLocked(conn, state, requestID, agodesk.ErrorInternal, "Could not inspect the upload reservation.")
		return
	}
	if len(existing) > 0 {
		if existing[0].PrepareFingerprint != fingerprint || len(existing) != len(normalized) {
			_ = writeAgodeskErrorLocked(conn, state, requestID, agodesk.ErrorKnowledgeRejected, "prepare_id was already used for a different batch.")
			return
		}
		if existing[0].Status == memory.AgoDeskKnowledgeStatusPrepared &&
			!existing[0].ExpiresAt.IsZero() &&
			time.Now().UTC().After(existing[0].ExpiresAt) {
			for _, record := range existing {
				if failed, failErr := s.ShortTermMem.MarkAgoDeskKnowledgeFailed(record.DocumentID, agodesk.ErrorKnowledgeExpired, "Knowledge upload reservation expired.", time.Now().UTC()); failErr == nil {
					coordinator.emit(*failed)
				}
			}
			_ = writeAgodeskErrorLocked(conn, state, requestID, agodesk.ErrorKnowledgeExpired, "Knowledge upload reservation expired.")
			return
		}
		if existing[0].Status == memory.AgoDeskKnowledgeStatusFailed && existing[0].ErrorCode == agodesk.ErrorKnowledgeExpired {
			_ = writeAgodeskErrorLocked(conn, state, requestID, agodesk.ErrorKnowledgeExpired, "Knowledge upload reservation expired.")
			return
		}
		sendAgodeskKnowledgePrepared(s, conn, state, transportSessionID, requestID, existing)
		return
	}

	seenPaths := make(map[string]struct{}, len(normalized))
	for _, file := range normalized {
		if _, duplicate := seenPaths[file.StoragePath]; duplicate {
			_ = writeAgodeskErrorLocked(conn, state, requestID, agodesk.ErrorKnowledgeRejected, "The batch contains duplicate filenames.")
			return
		}
		seenPaths[file.StoragePath] = struct{}{}
		if info, statErr := os.Lstat(file.StoragePath); statErr == nil || info != nil {
			_ = writeAgodeskErrorLocked(conn, state, requestID, agodesk.ErrorKnowledgeRejected, "A knowledge file with that name already exists.")
			return
		} else if !os.IsNotExist(statErr) {
			_ = writeAgodeskErrorLocked(conn, state, requestID, agodesk.ErrorKnowledgeRejected, "Knowledge storage is unavailable.")
			return
		}
		reserved, reserveErr := s.ShortTermMem.AgoDeskKnowledgeStoragePathReserved(file.StoragePath)
		if reserveErr != nil {
			_ = writeAgodeskErrorLocked(conn, state, requestID, agodesk.ErrorInternal, "Could not reserve the knowledge filename.")
			return
		}
		if reserved {
			_ = writeAgodeskErrorLocked(conn, state, requestID, agodesk.ErrorKnowledgeRejected, "A knowledge upload with that name already exists.")
			return
		}
	}

	now := time.Now().UTC()
	expiresAt := now.Add(agodeskKnowledgePrepareTTL)
	records := make([]memory.AgoDeskKnowledgeDocument, 0, len(normalized))
	for i, file := range normalized {
		records = append(records, memory.AgoDeskKnowledgeDocument{
			DocumentID:         "kdoc-" + uid.New(),
			PrepareID:          requestID,
			PrepareFingerprint: fingerprint,
			BatchIndex:         i,
			OwnerDeviceID:      deviceID,
			Status:             memory.AgoDeskKnowledgeStatusPrepared,
			Filename:           file.Payload.Filename,
			StoragePath:        file.StoragePath,
			Collection:         agodeskKnowledgeCollection(directory),
			Title:              file.Payload.Title,
			Tags:               file.Payload.Tags,
			DeclaredMime:       file.Payload.MimeType,
			DeclaredSizeBytes:  file.Payload.SizeBytes,
			CreatedAt:          now,
			ExpiresAt:          expiresAt,
		})
	}
	if err := s.ShortTermMem.PrepareAgoDeskKnowledgeBatch(records); err != nil {
		replayed, replayErr := s.ShortTermMem.ListAgoDeskKnowledgeByPrepare(deviceID, requestID)
		if replayErr == nil && len(replayed) == len(normalized) && replayed[0].PrepareFingerprint == fingerprint {
			sendAgodeskKnowledgePrepared(s, conn, state, transportSessionID, requestID, replayed)
			return
		}
		_ = writeAgodeskErrorLocked(conn, state, requestID, agodesk.ErrorKnowledgeRejected, "A knowledge filename is already reserved.")
		return
	}
	sendAgodeskKnowledgePrepared(s, conn, state, transportSessionID, requestID, records)
}

func sendAgodeskKnowledgePrepared(s *Server, conn *websocket.Conn, state *agodeskConnectionState, sessionID, prepareID string, records []memory.AgoDeskKnowledgeDocument) {
	documents := make([]agodesk.KnowledgeArchivePreparedDocument, 0, len(records))
	for _, record := range records {
		documents = append(documents, agodesk.KnowledgeArchivePreparedDocument{
			DocumentID:   record.DocumentID,
			Filename:     record.Filename,
			UploadURL:    signAgodeskKnowledgeUploadPath(s, record.DocumentID, record.ExpiresAt),
			UploadMethod: http.MethodPost,
			UploadField:  agodeskKnowledgeUploadFormField,
			ExpiresAt:    record.ExpiresAt.UTC().Format(time.RFC3339Nano),
			MaxBytes:     agodeskKnowledgeMaxFileBytes,
		})
	}
	_ = writeAgodeskEnvelopeLocked(conn, state, agodesk.TypeKnowledgeArchivePrepared, agodesk.KnowledgeArchivePreparedPayload{
		SessionID: sessionID,
		PrepareID: prepareID,
		Documents: documents,
	})
}

func handleAgodeskKnowledgeUpload(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.Header().Set("Allow", http.MethodPost)
			writeAgodeskKnowledgeHTTPError(w, http.StatusMethodNotAllowed, agodesk.ErrorKnowledgeRejected, "Method not allowed.", "")
			return
		}
		signatureValid, signatureExpired := verifyAgodeskKnowledgeUploadSignature(s, r, time.Now())
		if signatureExpired {
			writeAgodeskKnowledgeHTTPError(w, http.StatusGone, agodesk.ErrorKnowledgeExpired, "Knowledge upload reservation expired.", "")
			return
		}
		if !signatureValid {
			writeAgodeskKnowledgeHTTPError(w, http.StatusUnauthorized, agodesk.ErrorAuthFailed, "Upload signature is invalid or expired.", "")
			return
		}
		coordinator := ensureAgodeskKnowledgeCoordinator(s)
		if coordinator == nil || s == nil || s.ShortTermMem == nil || !agodeskKnowledgeArchiveUploadsEnabled(s) {
			writeAgodeskKnowledgeHTTPError(w, http.StatusServiceUnavailable, agodesk.ErrorKnowledgeIngestFailed, "Knowledge upload service is unavailable.", "")
			return
		}
		documentID, ok := agodeskKnowledgeDocumentIDFromPath(r.URL.Path)
		if !ok {
			writeAgodeskKnowledgeHTTPError(w, http.StatusNotFound, agodesk.ErrorKnowledgeNotFound, "Knowledge document was not found.", "")
			return
		}
		record, err := s.ShortTermMem.GetAgoDeskKnowledgeDocument(documentID)
		if err != nil {
			writeAgodeskKnowledgeHTTPError(w, http.StatusInternalServerError, agodesk.ErrorInternal, "Knowledge document lookup failed.", documentID)
			return
		}
		if record == nil {
			writeAgodeskKnowledgeHTTPError(w, http.StatusNotFound, agodesk.ErrorKnowledgeNotFound, "Knowledge document was not found.", documentID)
			return
		}
		uploading, err := coordinator.claimUpload(documentID, time.Now().UTC())
		if errors.Is(err, errAgodeskKnowledgeUploadExpired) {
			if uploading != nil {
				coordinator.emit(*uploading)
			}
			writeAgodeskKnowledgeHTTPError(w, http.StatusGone, agodesk.ErrorKnowledgeExpired, "Knowledge upload reservation expired.", documentID)
			return
		}
		if errors.Is(err, errAgodeskKnowledgeUploadState) {
			writeAgodeskKnowledgeHTTPError(w, http.StatusConflict, agodesk.ErrorKnowledgeRejected, "Knowledge document is not awaiting upload.", documentID)
			return
		}
		if err != nil {
			writeAgodeskKnowledgeHTTPError(w, http.StatusConflict, agodesk.ErrorKnowledgeRejected, "Knowledge upload is already in progress.", documentID)
			return
		}
		defer coordinator.releaseUpload(documentID)
		coordinator.emit(*uploading)
		uploadTerminal := false
		uploadFailureCode := agodesk.ErrorKnowledgeRejected
		uploadFailureMessage := "Knowledge upload did not complete."
		defer func() {
			if !uploadTerminal {
				_ = coordinator.failUpload(*uploading, uploadFailureCode, uploadFailureMessage)
			}
		}()

		r.Body = http.MaxBytesReader(w, r.Body, agodeskKnowledgeMaxFileBytes+(1<<20))
		multipartReader, err := r.MultipartReader()
		if err != nil {
			writeAgodeskKnowledgeHTTPError(w, http.StatusBadRequest, agodesk.ErrorKnowledgeRejected, "Invalid multipart upload.", documentID)
			return
		}
		part, err := nextAgodeskKnowledgeFilePart(multipartReader)
		if err != nil {
			writeAgodeskKnowledgeHTTPError(w, http.StatusBadRequest, agodesk.ErrorKnowledgeRejected, err.Error(), documentID)
			return
		}
		defer part.Close()
		uploadFilename, nameErr := normalizeAgodeskKnowledgeFilename(part.FileName())
		if nameErr != nil || uploadFilename != record.Filename {
			uploadTerminal = coordinator.failUpload(*record, agodesk.ErrorKnowledgeRejected, "Uploaded filename does not match the reservation.")
			writeAgodeskKnowledgeHTTPError(w, http.StatusConflict, agodesk.ErrorKnowledgeRejected, "Uploaded filename does not match the reservation.", documentID)
			return
		}
		if err := os.MkdirAll(filepath.Dir(record.StoragePath), 0o750); err != nil {
			uploadFailureCode = agodesk.ErrorKnowledgeIngestFailed
			uploadFailureMessage = "Could not create knowledge storage."
			writeAgodeskKnowledgeHTTPError(w, http.StatusInternalServerError, agodesk.ErrorKnowledgeIngestFailed, "Could not create knowledge storage.", documentID)
			return
		}
		tmp, err := os.CreateTemp(filepath.Dir(record.StoragePath), agodeskKnowledgePartPrefix+documentID+"-*.part")
		if err != nil {
			uploadFailureCode = agodesk.ErrorKnowledgeIngestFailed
			uploadFailureMessage = "Could not create upload file."
			writeAgodeskKnowledgeHTTPError(w, http.StatusInternalServerError, agodesk.ErrorKnowledgeIngestFailed, "Could not create upload file.", documentID)
			return
		}
		tmpPath := tmp.Name()
		removeTmp := true
		defer func() {
			_ = tmp.Close()
			if removeTmp {
				_ = os.Remove(tmpPath)
			}
		}()
		hasher := sha256.New()
		written, copyErr := io.Copy(io.MultiWriter(tmp, hasher), io.LimitReader(part, agodeskKnowledgeMaxFileBytes+1))
		if copyErr != nil {
			writeAgodeskKnowledgeHTTPError(w, http.StatusBadRequest, agodesk.ErrorKnowledgeRejected, "Could not read uploaded document.", documentID)
			return
		}
		if err := part.Close(); err != nil {
			writeAgodeskKnowledgeHTTPError(w, http.StatusBadRequest, agodesk.ErrorKnowledgeRejected, "Could not finish multipart file field.", documentID)
			return
		}
		if extraPart, extraErr := multipartReader.NextPart(); extraErr != io.EOF {
			if extraPart != nil {
				_ = extraPart.Close()
			}
			writeAgodeskKnowledgeHTTPError(w, http.StatusBadRequest, agodesk.ErrorKnowledgeRejected, "Upload must contain exactly one multipart file field.", documentID)
			return
		}
		if written > agodeskKnowledgeMaxFileBytes {
			_ = tmp.Close()
			uploadTerminal = coordinator.failUpload(*record, agodesk.ErrorKnowledgeTooLarge, "Uploaded document exceeds max_file_bytes.")
			writeAgodeskKnowledgeHTTPError(w, http.StatusRequestEntityTooLarge, agodesk.ErrorKnowledgeTooLarge, "Uploaded document exceeds max_file_bytes.", documentID)
			return
		}
		if written != record.DeclaredSizeBytes {
			_ = tmp.Close()
			uploadTerminal = coordinator.failUpload(*record, agodesk.ErrorKnowledgeRejected, "Uploaded document size does not match size_bytes.")
			writeAgodeskKnowledgeHTTPError(w, http.StatusConflict, agodesk.ErrorKnowledgeRejected, "Uploaded document size does not match size_bytes.", documentID)
			return
		}
		if err := tmp.Chmod(0o640); err != nil {
			uploadFailureCode = agodesk.ErrorKnowledgeIngestFailed
			uploadFailureMessage = "Could not secure uploaded document."
			writeAgodeskKnowledgeHTTPError(w, http.StatusInternalServerError, agodesk.ErrorKnowledgeIngestFailed, "Could not secure uploaded document.", documentID)
			return
		}
		if err := tmp.Close(); err != nil {
			uploadFailureCode = agodesk.ErrorKnowledgeIngestFailed
			uploadFailureMessage = "Could not finish uploaded document."
			writeAgodeskKnowledgeHTTPError(w, http.StatusInternalServerError, agodesk.ErrorKnowledgeIngestFailed, "Could not finish uploaded document.", documentID)
			return
		}
		detectedMime, validationErr := validateAgodeskKnowledgeFile(tmpPath, record.Filename, record.DeclaredMime)
		if validationErr != nil {
			if errors.Is(validationErr, services.ErrDocumentArchiveTooLarge) {
				message := "Uploaded document exceeds safe archive extraction limits."
				uploadTerminal = coordinator.failUpload(*record, agodesk.ErrorKnowledgeTooLarge, message)
				writeAgodeskKnowledgeHTTPError(w, http.StatusRequestEntityTooLarge, agodesk.ErrorKnowledgeTooLarge, message, documentID)
				return
			}
			uploadTerminal = coordinator.failUpload(*record, agodesk.ErrorKnowledgeMimeNotAllowed, validationErr.Error())
			writeAgodeskKnowledgeHTTPError(w, http.StatusUnsupportedMediaType, agodesk.ErrorKnowledgeMimeNotAllowed, validationErr.Error(), documentID)
			return
		}

		coordinator.publishMu.Lock()
		err = publishFileNoReplace(tmpPath, record.StoragePath)
		coordinator.publishMu.Unlock()
		if errors.Is(err, errAtomicPublishTargetExists) {
			uploadTerminal = coordinator.failUpload(*record, agodesk.ErrorKnowledgeRejected, "A knowledge file with that name already exists.")
			writeAgodeskKnowledgeHTTPError(w, http.StatusConflict, agodesk.ErrorKnowledgeRejected, "A knowledge file with that name already exists.", documentID)
			return
		}
		if err != nil {
			uploadFailureCode = agodesk.ErrorKnowledgeIngestFailed
			uploadFailureMessage = "Could not publish uploaded document."
			s.Logger.Warn("AgoDesk knowledge publication failed", "document_id", documentID, "error", scrubAgodeskKnowledgeLogError(err))
			writeAgodeskKnowledgeHTTPError(w, http.StatusInternalServerError, agodesk.ErrorKnowledgeIngestFailed, "Could not publish uploaded document.", documentID)
			return
		}
		actualSHA := hex.EncodeToString(hasher.Sum(nil))
		processing, err := s.ShortTermMem.MarkAgoDeskKnowledgeProcessing(documentID, detectedMime, written, actualSHA, time.Now().UTC())
		if err != nil {
			_ = os.Remove(record.StoragePath)
			uploadFailureCode = agodesk.ErrorKnowledgeIngestFailed
			uploadFailureMessage = "Could not start knowledge indexing."
			writeAgodeskKnowledgeHTTPError(w, http.StatusInternalServerError, agodesk.ErrorKnowledgeIngestFailed, "Could not start knowledge indexing.", documentID)
			return
		}
		uploadTerminal = true
		coordinator.emit(*processing)
		if !coordinator.enqueue(documentID) {
			coordinator.failProcessing(*processing, "Knowledge indexing queue is unavailable.")
			writeAgodeskKnowledgeHTTPError(w, http.StatusServiceUnavailable, agodesk.ErrorKnowledgeIngestFailed, "Knowledge indexing queue is unavailable.", documentID)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(agodeskKnowledgeUploadResponse{
			DocumentID: documentID,
			State:      memory.AgoDeskKnowledgeStatusProcessing,
			Filename:   record.Filename,
			MimeType:   detectedMime,
			SizeBytes:  written,
		})
	}
}

func (c *agodeskKnowledgeCoordinator) failUpload(record memory.AgoDeskKnowledgeDocument, code, message string) bool {
	failed, err := c.server.ShortTermMem.MarkAgoDeskKnowledgeFailed(record.DocumentID, code, sanitizeAgodeskKnowledgeError(message), time.Now().UTC())
	if err != nil {
		c.server.Logger.Warn("Failed to persist AgoDesk knowledge upload error", "document_id", record.DocumentID, "error", scrubAgodeskKnowledgeLogError(err))
		return false
	}
	c.emit(*failed)
	return true
}

func nextAgodeskKnowledgeFilePart(reader *multipart.Reader) (*multipart.Part, error) {
	for {
		part, err := reader.NextPart()
		if err == io.EOF {
			return nil, fmt.Errorf("Missing multipart file field.")
		}
		if err != nil {
			return nil, fmt.Errorf("Invalid multipart upload.")
		}
		if part.FormName() == agodeskKnowledgeUploadFormField {
			return part, nil
		}
		_ = part.Close()
	}
}

func normalizeAgodeskKnowledgeBatch(s *Server, directory config.IndexingDirectory, files []agodesk.KnowledgeArchivePrepareFile) ([]normalizedKnowledgePrepareFile, string, string, string) {
	if len(files) == 0 {
		return nil, "", agodesk.ErrorKnowledgeRejected, "files must contain at least one document."
	}
	if len(files) > agodeskKnowledgeMaxFilesPerBatch {
		return nil, "", agodesk.ErrorKnowledgeTooLarge, "files exceeds max_files_per_batch."
	}
	root, err := filepath.Abs(directory.Path)
	if err != nil {
		return nil, "", agodesk.ErrorKnowledgeRejected, "Knowledge storage is unavailable."
	}
	normalized := make([]normalizedKnowledgePrepareFile, 0, len(files))
	fingerprintFiles := make([]agodesk.KnowledgeArchivePrepareFile, 0, len(files))
	for _, input := range files {
		filename, err := normalizeAgodeskKnowledgeFilename(input.Filename)
		if err != nil {
			return nil, "", agodesk.ErrorKnowledgeRejected, err.Error()
		}
		if !isAllowedKnowledgeExtension(s, filename) {
			return nil, "", agodesk.ErrorKnowledgeMimeNotAllowed, "File extension is not enabled for knowledge indexing."
		}
		mimeType := normalizeAgodeskKnowledgeMime(input.MimeType)
		if !agodeskKnowledgeMimeAllowed(filename, mimeType) {
			return nil, "", agodesk.ErrorKnowledgeMimeNotAllowed, "mime_type is not allowed for the file extension."
		}
		if input.SizeBytes <= 0 {
			return nil, "", agodesk.ErrorKnowledgeRejected, "size_bytes must be positive."
		}
		if input.SizeBytes > agodeskKnowledgeMaxFileBytes {
			return nil, "", agodesk.ErrorKnowledgeTooLarge, "Document exceeds max_file_bytes."
		}
		title := strings.TrimSpace(input.Title)
		if title == "" {
			title = filename
		}
		if utf8.RuneCountInString(title) > agodeskKnowledgeMaxTitleRunes {
			return nil, "", agodesk.ErrorKnowledgeRejected, "title exceeds 255 characters."
		}
		tags, err := normalizeAgodeskKnowledgeTags(input.Tags)
		if err != nil {
			return nil, "", agodesk.ErrorKnowledgeRejected, err.Error()
		}
		storagePath, err := filepath.Abs(filepath.Join(root, filename))
		if err != nil || !pathStaysWithinDir(root, storagePath) {
			return nil, "", agodesk.ErrorKnowledgeRejected, "Invalid knowledge filename."
		}
		payload := agodesk.KnowledgeArchivePrepareFile{
			Filename:  filename,
			MimeType:  mimeType,
			SizeBytes: input.SizeBytes,
			Title:     title,
			Tags:      tags,
		}
		normalized = append(normalized, normalizedKnowledgePrepareFile{Payload: payload, StoragePath: storagePath})
		fingerprintFiles = append(fingerprintFiles, payload)
	}
	raw, err := json.Marshal(fingerprintFiles)
	if err != nil {
		return nil, "", agodesk.ErrorInternal, "Could not fingerprint the upload batch."
	}
	sum := sha256.Sum256(raw)
	return normalized, hex.EncodeToString(sum[:]), "", ""
}

func normalizeAgodeskKnowledgeFilename(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" || value == "." || value == ".." || utf8.RuneCountInString(value) > agodeskKnowledgeMaxFilenameRunes {
		return "", fmt.Errorf("filename must contain 1 to 255 characters.")
	}
	if filepath.Base(value) != value || strings.ContainsAny(value, `/\:*?"<>|`) || strings.HasSuffix(value, ".") || strings.HasSuffix(value, " ") {
		return "", fmt.Errorf("filename contains unsupported path characters.")
	}
	for _, r := range value {
		if r < 32 || r == 127 {
			return "", fmt.Errorf("filename contains control characters.")
		}
	}
	return value, nil
}

func normalizeAgodeskKnowledgeTags(values []string) ([]string, error) {
	if len(values) > agodeskKnowledgeMaxTags {
		return nil, fmt.Errorf("tags exceeds 32 entries.")
	}
	seen := make(map[string]struct{}, len(values))
	tags := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if utf8.RuneCountInString(value) > agodeskKnowledgeMaxTagRunes {
			return nil, fmt.Errorf("tag exceeds 64 characters.")
		}
		key := strings.ToLower(value)
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		tags = append(tags, value)
	}
	return tags, nil
}

func normalizeAgodeskKnowledgeMime(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	if parsed, _, err := mime.ParseMediaType(value); err == nil {
		return parsed
	}
	return value
}

func agodeskKnowledgeMimeAllowed(filename, mimeType string) bool {
	ext := strings.ToLower(filepath.Ext(filename))
	mimeType = normalizeAgodeskKnowledgeMime(mimeType)
	switch ext {
	case ".pdf":
		return mimeType == "application/pdf"
	case ".docx":
		return mimeType == "application/vnd.openxmlformats-officedocument.wordprocessingml.document"
	case ".xlsx":
		return mimeType == "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"
	case ".pptx":
		return mimeType == "application/vnd.openxmlformats-officedocument.presentationml.presentation"
	case ".odt":
		return mimeType == "application/vnd.oasis.opendocument.text"
	case ".ods":
		return mimeType == "application/vnd.oasis.opendocument.spreadsheet"
	case ".odp":
		return mimeType == "application/vnd.oasis.opendocument.presentation"
	case ".rtf":
		return mimeType == "application/rtf" || mimeType == "text/rtf"
	case ".json":
		return mimeType == "application/json" || mimeType == "text/json" || mimeType == "text/plain"
	default:
		return strings.HasPrefix(mimeType, "text/")
	}
}

func validateAgodeskKnowledgeFile(path, filename, declaredMime string) (string, error) {
	ext := strings.ToLower(filepath.Ext(filename))
	if !agodeskKnowledgeMimeAllowed(filename, declaredMime) {
		return "", fmt.Errorf("Uploaded MIME type is not allowed for the file extension.")
	}
	executable, err := hasAgodeskKnowledgeExecutableMagic(path)
	if err != nil {
		return "", fmt.Errorf("Uploaded file could not be inspected.")
	}
	if executable {
		return "", fmt.Errorf("Executable files are not allowed in the knowledge archive.")
	}
	switch ext {
	case ".pdf":
		header, err := readAgodeskKnowledgePrefix(path, 5)
		if err != nil || string(header) != "%PDF-" {
			return "", fmt.Errorf("Uploaded file is not a valid PDF.")
		}
		return "application/pdf", nil
	case ".docx":
		if err := validateAgodeskKnowledgeZIP(path, "word/document.xml", ""); err != nil {
			if errors.Is(err, services.ErrDocumentArchiveTooLarge) {
				return "", err
			}
			return "", fmt.Errorf("Uploaded file is not a valid DOCX document.")
		}
		return "application/vnd.openxmlformats-officedocument.wordprocessingml.document", nil
	case ".xlsx":
		if err := validateAgodeskKnowledgeZIP(path, "xl/workbook.xml", ""); err != nil {
			if errors.Is(err, services.ErrDocumentArchiveTooLarge) {
				return "", err
			}
			return "", fmt.Errorf("Uploaded file is not a valid XLSX workbook.")
		}
		return "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet", nil
	case ".pptx":
		if err := validateAgodeskKnowledgeZIP(path, "ppt/presentation.xml", ""); err != nil {
			if errors.Is(err, services.ErrDocumentArchiveTooLarge) {
				return "", err
			}
			return "", fmt.Errorf("Uploaded file is not a valid PPTX presentation.")
		}
		return "application/vnd.openxmlformats-officedocument.presentationml.presentation", nil
	case ".odt":
		if err := validateAgodeskKnowledgeZIP(path, "content.xml", "application/vnd.oasis.opendocument.text"); err != nil {
			if errors.Is(err, services.ErrDocumentArchiveTooLarge) {
				return "", err
			}
			return "", fmt.Errorf("Uploaded file is not a valid ODT document.")
		}
		return "application/vnd.oasis.opendocument.text", nil
	case ".ods":
		if err := validateAgodeskKnowledgeZIP(path, "content.xml", "application/vnd.oasis.opendocument.spreadsheet"); err != nil {
			if errors.Is(err, services.ErrDocumentArchiveTooLarge) {
				return "", err
			}
			return "", fmt.Errorf("Uploaded file is not a valid ODS spreadsheet.")
		}
		return "application/vnd.oasis.opendocument.spreadsheet", nil
	case ".odp":
		if err := validateAgodeskKnowledgeZIP(path, "content.xml", "application/vnd.oasis.opendocument.presentation"); err != nil {
			if errors.Is(err, services.ErrDocumentArchiveTooLarge) {
				return "", err
			}
			return "", fmt.Errorf("Uploaded file is not a valid ODP presentation.")
		}
		return "application/vnd.oasis.opendocument.presentation", nil
	case ".rtf":
		header, err := readAgodeskKnowledgePrefix(path, 5)
		if err != nil || !strings.HasPrefix(string(header), `{\rtf`) {
			return "", fmt.Errorf("Uploaded file is not a valid RTF document.")
		}
		return "application/rtf", nil
	default:
		if err := validateAgodeskKnowledgeText(path); err != nil {
			return "", fmt.Errorf("Uploaded file is not valid UTF-8 text.")
		}
		if ext == ".json" {
			raw, err := os.ReadFile(path)
			if err != nil || !json.Valid(raw) {
				return "", fmt.Errorf("Uploaded file is not valid JSON.")
			}
			return "application/json", nil
		}
		return "text/plain", nil
	}
}

func hasAgodeskKnowledgeExecutableMagic(path string) (bool, error) {
	header, err := readAgodeskKnowledgePrefix(path, 4)
	if err != nil {
		return false, err
	}
	if len(header) >= 2 && string(header[:2]) == "MZ" {
		return true, nil
	}
	if len(header) < 4 {
		return false, nil
	}
	switch [4]byte(header[:4]) {
	case [4]byte{0x7f, 'E', 'L', 'F'},
		[4]byte{0x00, 'a', 's', 'm'},
		[4]byte{0xca, 0xfe, 0xba, 0xbe},
		[4]byte{0xbe, 0xba, 0xfe, 0xca},
		[4]byte{0xfe, 0xed, 0xfa, 0xce},
		[4]byte{0xfe, 0xed, 0xfa, 0xcf},
		[4]byte{0xce, 0xfa, 0xed, 0xfe},
		[4]byte{0xcf, 0xfa, 0xed, 0xfe}:
		return true, nil
	default:
		return false, nil
	}
}

func validateAgodeskKnowledgeText(path string) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()
	reader := bufio.NewReader(file)
	for {
		r, size, err := reader.ReadRune()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
		if r == utf8.RuneError && size == 1 {
			return fmt.Errorf("invalid UTF-8")
		}
	}
}

func readAgodeskKnowledgePrefix(path string, count int64) ([]byte, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	return io.ReadAll(io.LimitReader(file, count))
}

func validateAgodeskKnowledgeZIP(path, requiredEntry, requiredMimetype string) error {
	return services.ValidateDocumentArchive(path, requiredEntry, requiredMimetype)
}

func agodeskKnowledgeArchiveUploadsEnabled(s *Server) bool {
	if s == nil || s.Cfg == nil || s.ShortTermMem == nil || s.FileIndexer == nil || s.LongTermMem == nil || s.LongTermMem.IsDisabled() {
		return false
	}
	s.CfgMu.RLock()
	enabled := s.Cfg.Indexing.Enabled &&
		len(s.Cfg.Indexing.Directories) > 0 &&
		strings.TrimSpace(s.Cfg.Indexing.Directories[0].Path) != "" &&
		strings.TrimSpace(s.Cfg.Auth.SessionSecret) != ""
	s.CfgMu.RUnlock()
	return enabled
}

func agodeskKnowledgeDirectory(s *Server) (config.IndexingDirectory, bool) {
	if s == nil || s.Cfg == nil {
		return config.IndexingDirectory{}, false
	}
	s.CfgMu.RLock()
	defer s.CfgMu.RUnlock()
	if !s.Cfg.Indexing.Enabled || len(s.Cfg.Indexing.Directories) == 0 {
		return config.IndexingDirectory{}, false
	}
	directory := s.Cfg.Indexing.Directories[0]
	directory.Path = strings.TrimSpace(directory.Path)
	if directory.Path == "" {
		return config.IndexingDirectory{}, false
	}
	return directory, true
}

func agodeskKnowledgeCollection(directory config.IndexingDirectory) string {
	if collection := strings.TrimSpace(directory.Collection); collection != "" {
		return collection
	}
	return services.IndexerCollection
}

func agodeskKnowledgeArchiveLimitsPayload(s *Server) agodesk.KnowledgeArchiveLimitsPayload {
	return agodesk.KnowledgeArchiveLimitsPayload{
		MaxFileBytes:        agodeskKnowledgeMaxFileBytes,
		MaxFilesPerBatch:    agodeskKnowledgeMaxFilesPerBatch,
		AllowedMimePrefixes: agodeskKnowledgeAllowedMimePrefixes(s),
	}
}

func agodeskKnowledgeArchiveLimitsForAccepted(s *Server, advertised []string) *agodesk.KnowledgeArchiveLimitsPayload {
	if !agodeskKnowledgeArchiveUploadsEnabled(s) || !agodeskStringSliceContains(advertised, agodesk.CapabilityKnowledgeArchive) {
		return nil
	}
	limits := agodeskKnowledgeArchiveLimitsPayload(s)
	return &limits
}

func agodeskKnowledgeAllowedMimePrefixes(s *Server) []string {
	var extensions []string
	if s != nil && s.Cfg != nil {
		s.CfgMu.RLock()
		extensions = append(extensions, s.Cfg.Indexing.Extensions...)
		s.CfgMu.RUnlock()
	}
	if len(extensions) == 0 {
		extensions = append(extensions, defaultKnowledgeExtensions...)
	}
	values := map[string]struct{}{}
	for _, ext := range extensions {
		switch strings.ToLower(strings.TrimSpace(ext)) {
		case ".pdf":
			values["application/pdf"] = struct{}{}
		case ".docx", ".xlsx", ".pptx":
			values["application/vnd.openxmlformats-officedocument"] = struct{}{}
		case ".odt", ".ods", ".odp":
			values["application/vnd.oasis.opendocument"] = struct{}{}
		case ".rtf":
			values["application/rtf"] = struct{}{}
			values["text/rtf"] = struct{}{}
		case ".json":
			values["application/json"] = struct{}{}
			values["text/"] = struct{}{}
		default:
			values["text/"] = struct{}{}
		}
	}
	out := make([]string, 0, len(values))
	for value := range values {
		out = append(out, value)
	}
	sort.Strings(out)
	return out
}

func signAgodeskKnowledgeUploadPath(s *Server, documentID string, expiresAt time.Time) string {
	documentID = strings.TrimSpace(documentID)
	secret := agodeskMediaAssetSigningSecret(s)
	if documentID == "" || secret == "" || expiresAt.IsZero() {
		return ""
	}
	pathValue := agodeskKnowledgeUploadPathPrefix + url.PathEscape(documentID)
	parsed, err := url.Parse(pathValue)
	if err != nil {
		return ""
	}
	query := parsed.Query()
	query.Set(agodeskMediaAssetExpParam, fmt.Sprintf("%d", expiresAt.Unix()))
	query.Del(agodeskMediaAssetSigParam)
	signature := agodeskMediaAssetSignature(secret, agodeskMediaAssetSignatureMaterial(parsed.EscapedPath(), query))
	query.Set(agodeskMediaAssetSigParam, signature)
	parsed.RawQuery = query.Encode()
	return parsed.String()
}

func verifyAgodeskKnowledgeUploadSignature(s *Server, r *http.Request, now time.Time) (valid bool, expired bool) {
	secret := agodeskMediaAssetSigningSecret(s)
	if secret == "" || r == nil || r.URL == nil {
		return false, false
	}
	query := r.URL.Query()
	expiresRaw := strings.TrimSpace(query.Get(agodeskMediaAssetExpParam))
	signature := strings.TrimSpace(query.Get(agodeskMediaAssetSigParam))
	if expiresRaw == "" || signature == "" {
		return false, false
	}
	expiresUnix, err := strconv.ParseInt(expiresRaw, 10, 64)
	if err != nil || expiresUnix <= 0 {
		return false, false
	}
	query.Del(agodeskMediaAssetSigParam)
	expected := agodeskMediaAssetSignature(secret, agodeskMediaAssetSignatureMaterial(r.URL.EscapedPath(), query))
	if !hmac.Equal([]byte(strings.ToLower(signature)), []byte(expected)) {
		return false, false
	}
	if now.Unix() > expiresUnix {
		return false, true
	}
	return true, false
}

func agodeskKnowledgeDocumentIDFromPath(pathValue string) (string, bool) {
	escaped := strings.TrimPrefix(pathValue, agodeskKnowledgeUploadPathPrefix)
	if escaped == "" || escaped == pathValue {
		return "", false
	}
	documentID, err := url.PathUnescape(escaped)
	if err != nil || !strings.HasPrefix(documentID, "kdoc-") ||
		strings.ContainsAny(documentID, `/\`) || strings.Contains(documentID, "..") || strings.ContainsRune(documentID, '\x00') {
		return "", false
	}
	return documentID, true
}

func agodeskKnowledgeStatusPayload(record memory.AgoDeskKnowledgeDocument, sessionID string) agodesk.KnowledgeArchiveStatusPayload {
	return agodesk.KnowledgeArchiveStatusPayload{
		SessionID:  sessionID,
		DocumentID: record.DocumentID,
		State:      record.Status,
		Title:      record.Title,
		ChunkCount: record.ChunkCount,
		ErrorCode:  record.ErrorCode,
		Error:      record.ErrorMessage,
	}
}

func writeAgodeskKnowledgeHTTPError(w http.ResponseWriter, status int, code, message, documentID string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(agodeskKnowledgeHTTPError{
		Code:       code,
		Message:    message,
		DocumentID: documentID,
	})
}

func sanitizeAgodeskKnowledgeError(message string) string {
	message = strings.TrimSpace(message)
	if message == "" {
		return "Knowledge ingest failed."
	}
	scrubbed := strings.TrimSpace(security.RedactSensitiveInfo(security.Scrub(message)))
	if scrubbed == "" || scrubbed != message || strings.Contains(strings.ToLower(scrubbed), "[redacted]") ||
		containsAgodeskKnowledgeInternalLocation(scrubbed) {
		return "Knowledge ingest failed."
	}
	if len(message) > 512 {
		message = message[:512]
	}
	return message
}

func scrubAgodeskKnowledgeLogError(err error) string {
	if err == nil {
		return ""
	}
	return security.RedactSensitiveInfo(security.Scrub(err.Error()))
}

func containsAgodeskKnowledgeInternalLocation(message string) bool {
	lower := strings.ToLower(message)
	if strings.Contains(lower, "://") || strings.Contains(message, `:\`) || strings.Contains(message, `\\`) {
		return true
	}
	for _, field := range strings.Fields(message) {
		candidate := strings.Trim(field, `"'(),:;[]{}<>`)
		if filepath.IsAbs(candidate) || strings.HasPrefix(candidate, "/") {
			return true
		}
	}
	return false
}
