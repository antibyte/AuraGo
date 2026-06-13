package server

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/url"
	"os"
	pathpkg "path"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"aurago/internal/agodesk"
	"aurago/internal/memory"
	"aurago/internal/uid"

	"github.com/gorilla/websocket"
	"github.com/sashabaranov/go-openai"
)

const (
	agodeskAttachmentMaxFileBytes       = 8 * 1024 * 1024
	agodeskAttachmentMaxFiles           = 5
	agodeskAttachmentMaxTotalBytes      = 24 * 1024 * 1024
	agodeskAttachmentPrepareTTL         = 5 * time.Minute
	agodeskAttachmentUploadFormField    = "file"
	agodeskAttachmentInlineTextMaxBytes = 12 * 1024
)

var agodeskAttachmentBlockPattern = regexp.MustCompile(`(?s)\s*<agodesk_attachments>.*?</agodesk_attachments>\s*`)

type agodeskAttachmentUploadResponse struct {
	AttachmentID string `json:"attachment_id"`
	Status       string `json:"status"`
	Path         string `json:"path"`
	MimeType     string `json:"mime_type"`
	SizeBytes    int64  `json:"size_bytes"`
	SHA256       string `json:"sha256"`
	Filename     string `json:"filename"`
	Kind         string `json:"kind"`
}

type agodeskAttachmentBindingContextKey struct{}

func contextWithAgodeskAttachmentBinding(ctx context.Context, s *Server, conversationID string, records []memory.AgoDeskAttachmentRecord) context.Context {
	if len(records) == 0 || s == nil || s.ShortTermMem == nil {
		return ctx
	}
	ids := attachmentRecordsToIDs(records)
	if len(ids) == 0 {
		return ctx
	}
	return context.WithValue(ctx, agodeskAttachmentBindingContextKey{}, func(messageID int64) error {
		return s.ShortTermMem.BindAgoDeskAttachmentsToMessage(conversationID, messageID, ids)
	})
}

func agodeskAttachmentBindingCallback(ctx context.Context) func(int64) error {
	if ctx == nil {
		return nil
	}
	callback, _ := ctx.Value(agodeskAttachmentBindingContextKey{}).(func(int64) error)
	return callback
}

func agodeskAttachmentLimitsPayload() agodesk.AttachmentLimitsPayload {
	return agodesk.AttachmentLimitsPayload{
		MaxFileBytes:  agodeskAttachmentMaxFileBytes,
		MaxFiles:      agodeskAttachmentMaxFiles,
		MaxTotalBytes: agodeskAttachmentMaxTotalBytes,
		AllowedMime:   []string{"image/*", "text/*", "application/pdf"},
	}
}

func agodeskAttachmentLimitsForAccepted(s *Server, advertised []string) *agodesk.AttachmentLimitsPayload {
	if !agodeskAttachmentUploadsEnabled(s) {
		return nil
	}
	if !agodeskStringSliceContains(advertised, "chat.attachments") && !agodeskStringSliceContains(advertised, "chat.media_upload") {
		return nil
	}
	limits := agodeskAttachmentLimitsPayload()
	return &limits
}

func agodeskAttachmentUploadsEnabled(s *Server) bool {
	if s == nil || s.Cfg == nil {
		return false
	}
	s.CfgMu.RLock()
	workspaceDir := strings.TrimSpace(s.Cfg.Directories.WorkspaceDir)
	secret := strings.TrimSpace(s.Cfg.Auth.SessionSecret)
	s.CfgMu.RUnlock()
	return workspaceDir != "" && secret != ""
}

func handleAgodeskAttachmentPrepare(s *Server, conn *websocket.Conn, state *agodeskConnectionState, requestID string, payload agodesk.ChatAttachmentPreparePayload) {
	transportSessionID, ok := validateAgodeskTransportSession(s, conn, state, requestID, payload.SessionID, "chat.attachment.prepare")
	if !ok || !validateAgodeskCapability(conn, state, requestID, "chat.media_upload", "chat.attachment.prepare") {
		return
	}
	if s == nil || s.ShortTermMem == nil {
		_ = writeAgodeskErrorLocked(conn, state, requestID, agodesk.ErrorInternal, "short-term memory is not configured")
		return
	}
	if !agodeskAttachmentUploadsEnabled(s) {
		_ = writeAgodeskErrorLocked(conn, state, requestID, agodesk.ErrorUnsupportedCapability, "chat.attachment.prepare requires an active attachment upload service")
		return
	}
	conversationID, ok := resolveAgodeskConversationID(s, conn, state, requestID, transportSessionID, strings.TrimSpace(payload.ConversationID))
	if !ok {
		return
	}
	filename := sanitizeFilename(payload.Filename)
	if filename == "" || filename == "upload.bin" && strings.TrimSpace(payload.Filename) == "" {
		_ = writeAgodeskErrorLocked(conn, state, requestID, agodesk.ErrorAttachmentRejected, "filename is required")
		return
	}
	if isActiveContentExtension(filename) {
		_ = writeAgodeskErrorLocked(conn, state, requestID, agodesk.ErrorAttachmentRejected, "active content attachments are not allowed")
		return
	}
	if payload.SizeBytes <= 0 {
		_ = writeAgodeskErrorLocked(conn, state, requestID, agodesk.ErrorAttachmentRejected, "size_bytes must be positive")
		return
	}
	if payload.SizeBytes > agodeskAttachmentMaxFileBytes {
		_ = writeAgodeskErrorLocked(conn, state, requestID, agodesk.ErrorAttachmentTooLarge, "attachment exceeds max_file_bytes")
		return
	}
	mimeType := normalizeAgodeskAttachmentMime(payload.MimeType)
	if !agodeskAttachmentMimeAllowed(mimeType) {
		_ = writeAgodeskErrorLocked(conn, state, requestID, agodesk.ErrorAttachmentMimeNotAllowed, "mime_type is not allowed")
		return
	}
	expectedSHA := strings.ToLower(strings.TrimSpace(payload.SHA256))
	if expectedSHA != "" {
		if len(expectedSHA) != 64 {
			_ = writeAgodeskErrorLocked(conn, state, requestID, agodesk.ErrorAttachmentRejected, "sha256 must be hex encoded")
			return
		}
		if _, err := hex.DecodeString(expectedSHA); err != nil {
			_ = writeAgodeskErrorLocked(conn, state, requestID, agodesk.ErrorAttachmentRejected, "sha256 must be hex encoded")
			return
		}
	}
	_, _ = s.ShortTermMem.CleanupExpiredAgoDeskAttachments(time.Now().UTC())
	attachmentID := "att-" + uid.New()
	expiresAt := time.Now().UTC().Add(agodeskAttachmentPrepareTTL)
	if err := s.ShortTermMem.PrepareAgoDeskAttachment(memory.AgoDeskAttachmentRecord{
		AttachmentID:       attachmentID,
		TransportSessionID: transportSessionID,
		ConversationID:     conversationID,
		Filename:           filename,
		MimeType:           mimeType,
		Kind:               agodeskAttachmentKind(mimeType, filename),
		DeclaredSizeBytes:  payload.SizeBytes,
		ExpectedSHA256:     expectedSHA,
		ExpiresAt:          expiresAt,
	}); err != nil {
		_ = writeAgodeskErrorLocked(conn, state, requestID, agodesk.ErrorInternal, err.Error())
		return
	}
	uploadURL := signAgodeskMediaAssetPath(s, "/api/agodesk/media/upload/"+url.PathEscape(attachmentID), time.Now())
	_ = writeAgodeskEnvelopeLocked(conn, state, agodesk.TypeChatAttachmentPrepared, agodesk.ChatAttachmentPreparedPayload{
		SessionID:      transportSessionID,
		ConversationID: conversationID,
		PrepareID:      requestID,
		AttachmentID:   attachmentID,
		UploadURL:      uploadURL,
		Method:         http.MethodPost,
		UploadField:    agodeskAttachmentUploadFormField,
		ExpiresAt:      expiresAt.Format(time.RFC3339),
		MaxBytes:       agodeskAttachmentMaxFileBytes,
	})
}

func handleAgodeskAttachmentUpload(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.Header().Set("Allow", http.MethodPost)
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if !verifyAgodeskMediaAssetSignature(s, r, time.Now()) {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		if s == nil || s.ShortTermMem == nil || !agodeskAttachmentUploadsEnabled(s) {
			http.Error(w, "attachment upload service unavailable", http.StatusServiceUnavailable)
			return
		}
		attachmentID, ok := agodeskAttachmentIDFromUploadPath(r.URL.Path)
		if !ok {
			http.NotFound(w, r)
			return
		}
		record, err := s.ShortTermMem.GetAgoDeskAttachment(attachmentID)
		if err != nil {
			http.Error(w, "attachment lookup failed", http.StatusInternalServerError)
			return
		}
		if record == nil {
			http.Error(w, "attachment not found", http.StatusNotFound)
			return
		}
		if record.Status != memory.AgoDeskAttachmentStatusPrepared {
			http.Error(w, "attachment is not prepared", http.StatusConflict)
			return
		}
		if !record.ExpiresAt.IsZero() && time.Now().UTC().After(record.ExpiresAt) {
			http.Error(w, "attachment prepare expired", http.StatusGone)
			return
		}
		r.Body = http.MaxBytesReader(w, r.Body, agodeskAttachmentMaxFileBytes+1024*1024)
		if err := r.ParseMultipartForm(agodeskAttachmentMaxFileBytes + 1024*1024); err != nil {
			http.Error(w, "invalid multipart upload", http.StatusBadRequest)
			return
		}
		file, header, err := r.FormFile(agodeskAttachmentUploadFormField)
		if err != nil {
			http.Error(w, "missing file field", http.StatusBadRequest)
			return
		}
		defer file.Close()
		data, err := io.ReadAll(io.LimitReader(file, agodeskAttachmentMaxFileBytes+1))
		if err != nil {
			http.Error(w, "read upload failed", http.StatusBadRequest)
			return
		}
		if len(data) == 0 {
			http.Error(w, "empty upload", http.StatusBadRequest)
			return
		}
		if int64(len(data)) > agodeskAttachmentMaxFileBytes {
			http.Error(w, "attachment too large", http.StatusRequestEntityTooLarge)
			return
		}
		if record.DeclaredSizeBytes > 0 && int64(len(data)) != record.DeclaredSizeBytes {
			http.Error(w, "attachment size mismatch", http.StatusBadRequest)
			return
		}
		sum := sha256.Sum256(data)
		actualSHA := hex.EncodeToString(sum[:])
		if record.ExpectedSHA256 != "" && !strings.EqualFold(record.ExpectedSHA256, actualSHA) {
			http.Error(w, "attachment sha256 mismatch", http.StatusBadRequest)
			return
		}
		filename := sanitizeFilename(header.Filename)
		if filename == "upload.bin" && strings.TrimSpace(record.Filename) != "" {
			filename = record.Filename
		}
		if isActiveContentExtension(filename) {
			http.Error(w, "active content attachments are not allowed", http.StatusBadRequest)
			return
		}
		mimeType := normalizeAgodeskAttachmentMime(record.MimeType)
		if uploadType := normalizeAgodeskAttachmentMime(header.Header.Get("Content-Type")); uploadType != "" {
			if uploadType != "application/octet-stream" {
				mimeType = uploadType
			}
		}
		sniffed := normalizeAgodeskAttachmentMime(http.DetectContentType(data[:min(len(data), 512)]))
		if !agodeskAttachmentMimeAllowed(mimeType) || !agodeskAttachmentMimeCompatible(mimeType, sniffed) {
			http.Error(w, "attachment mime type is not allowed", http.StatusBadRequest)
			return
		}
		kind := agodeskAttachmentKind(mimeType, filename)
		relPath := agodeskAttachmentRelativePath(record.ConversationID, attachmentID, filename)
		absPath, ok := agodeskAttachmentAbsolutePath(s, relPath)
		if !ok {
			http.Error(w, "attachment storage unavailable", http.StatusServiceUnavailable)
			return
		}
		if err := os.MkdirAll(filepath.Dir(absPath), 0o755); err != nil {
			http.Error(w, "create attachment directory failed", http.StatusInternalServerError)
			return
		}
		tmp, err := os.CreateTemp(filepath.Dir(absPath), ".upload-*.tmp")
		if err != nil {
			http.Error(w, "create temp upload failed", http.StatusInternalServerError)
			return
		}
		tmpName := tmp.Name()
		cleanupTmp := true
		defer func() {
			if cleanupTmp {
				_ = os.Remove(tmpName)
			}
		}()
		if _, err := tmp.Write(data); err != nil {
			_ = tmp.Close()
			http.Error(w, "write upload failed", http.StatusInternalServerError)
			return
		}
		if err := tmp.Chmod(0o600); err != nil {
			_ = tmp.Close()
			http.Error(w, "chmod upload failed", http.StatusInternalServerError)
			return
		}
		if err := tmp.Close(); err != nil {
			http.Error(w, "close upload failed", http.StatusInternalServerError)
			return
		}
		if err := os.Rename(tmpName, absPath); err != nil {
			http.Error(w, "store upload failed", http.StatusInternalServerError)
			return
		}
		cleanupTmp = false
		updated, err := s.ShortTermMem.MarkAgoDeskAttachmentUploaded(attachmentID, int64(len(data)), actualSHA, relPath, kind, mimeType)
		if err != nil {
			http.Error(w, "mark upload failed", http.StatusInternalServerError)
			return
		}
		item := agodeskChatAttachmentItem(s, *updated)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(agodeskAttachmentUploadResponse{
			AttachmentID: item.AttachmentID,
			Status:       "ready",
			Path:         item.Path,
			MimeType:     item.MimeType,
			SizeBytes:    item.SizeBytes,
			SHA256:       item.SHA256,
			Filename:     item.Filename,
			Kind:         item.Kind,
		})
	}
}

func agodeskAttachmentIDFromUploadPath(pathValue string) (string, bool) {
	rel := strings.TrimPrefix(pathValue, "/api/agodesk/media/upload/")
	if rel == "" || rel == pathValue {
		return "", false
	}
	id, err := url.PathUnescape(rel)
	if err != nil {
		return "", false
	}
	id = strings.TrimSpace(id)
	if id == "" || strings.Contains(id, "/") || strings.Contains(id, `\`) || strings.Contains(id, "..") || strings.Contains(id, "\x00") {
		return "", false
	}
	return id, true
}

func agodeskResolveChatAttachments(s *Server, state *agodeskConnectionState, transportSessionID, conversationID string, refs []agodesk.ChatAttachmentItem) ([]memory.AgoDeskAttachmentRecord, []agodesk.ChatAttachmentItem, string, string) {
	if len(refs) == 0 {
		return nil, nil, "", ""
	}
	if len(refs) > agodeskAttachmentMaxFiles {
		return nil, nil, agodesk.ErrorAttachmentTooLarge, "too many attachments"
	}
	if s == nil || s.ShortTermMem == nil {
		return nil, nil, agodesk.ErrorInternal, "short-term memory is not configured"
	}
	now := time.Now().UTC()
	seen := map[string]struct{}{}
	var total int64
	records := make([]memory.AgoDeskAttachmentRecord, 0, len(refs))
	items := make([]agodesk.ChatAttachmentItem, 0, len(refs))
	for _, ref := range refs {
		attachmentID := strings.TrimSpace(ref.AttachmentID)
		if attachmentID == "" {
			return nil, nil, agodesk.ErrorAttachmentNotFound, "attachment_id is required"
		}
		if _, ok := seen[attachmentID]; ok {
			return nil, nil, agodesk.ErrorAttachmentRejected, "duplicate attachment_id"
		}
		seen[attachmentID] = struct{}{}
		record, err := s.ShortTermMem.GetAgoDeskAttachment(attachmentID)
		if err != nil {
			return nil, nil, agodesk.ErrorInternal, err.Error()
		}
		if record == nil {
			return nil, nil, agodesk.ErrorAttachmentNotFound, "attachment was not found"
		}
		if record.Status != memory.AgoDeskAttachmentStatusUploaded {
			return nil, nil, agodesk.ErrorAttachmentNotReady, "attachment is not ready"
		}
		if !record.ExpiresAt.IsZero() && now.After(record.ExpiresAt) {
			return nil, nil, agodesk.ErrorAttachmentExpired, "attachment upload expired"
		}
		if record.TransportSessionID != strings.TrimSpace(transportSessionID) || record.ConversationID != strings.TrimSpace(conversationID) {
			return nil, nil, agodesk.ErrorAttachmentRejected, "attachment belongs to a different session"
		}
		total += record.SizeBytes
		if total > agodeskAttachmentMaxTotalBytes {
			return nil, nil, agodesk.ErrorAttachmentTooLarge, "attachments exceed max_total_bytes"
		}
		records = append(records, *record)
		items = append(items, agodeskChatAttachmentItem(s, *record))
	}
	return records, items, "", ""
}

func buildAgodeskMessageWithAttachments(s *Server, message string, records []memory.AgoDeskAttachmentRecord) string {
	message = strings.TrimSpace(message)
	if len(records) == 0 {
		return message
	}
	if message == "" {
		message = "Please review the attached file(s)."
	}
	var b strings.Builder
	b.WriteString(message)
	b.WriteString("\n\n<agodesk_attachments>\n")
	b.WriteString("These files were explicitly uploaded through AgoDesk. Use agent_path for file operations; filename is display-only and may not exist in the working directory.\n")
	for _, record := range records {
		agentPath := agodeskAttachmentAgentPath(record.RelativePath)
		b.WriteString("- ")
		b.WriteString("attachment_id: ")
		b.WriteString(strings.TrimSpace(record.AttachmentID))
		b.WriteString("\n  filename: ")
		b.WriteString(strings.TrimSpace(record.Filename))
		b.WriteString("\n  mime_type: ")
		b.WriteString(strings.TrimSpace(record.MimeType))
		b.WriteString("\n  size_bytes: ")
		b.WriteString(fmt.Sprintf("%d", record.SizeBytes))
		if agentPath != "" {
			b.WriteString("\n  agent_path: ")
			b.WriteString(agentPath)
		}
		b.WriteByte('\n')
		if strings.HasPrefix(record.MimeType, "text/") {
			if text := agodeskAttachmentInlineText(s, record.RelativePath); text != "" {
				b.WriteString(desktopExternalData("agodesk_attachment_text:"+record.Filename, text, agodeskAttachmentInlineTextMaxBytes))
				b.WriteByte('\n')
			}
		}
	}
	b.WriteString("</agodesk_attachments>")
	return b.String()
}

func agodeskMessagesWithAttachmentContext(s *Server, messages []memory.HistoryMessage, skipMessageID int64) []memory.HistoryMessage {
	if s == nil || s.ShortTermMem == nil || len(messages) == 0 {
		return messages
	}
	ids := make([]int64, 0, len(messages))
	for _, msg := range messages {
		if msg.ID > 0 && msg.ID != skipMessageID && !msg.IsInternal && msg.Role == openai.ChatMessageRoleUser {
			ids = append(ids, msg.ID)
		}
	}
	if len(ids) == 0 {
		return messages
	}
	recordsByMessage, err := s.ShortTermMem.ListAgoDeskAttachmentsForMessages(ids)
	if err != nil {
		if s.Logger != nil {
			s.Logger.Warn("Failed to load agodesk attachments for agent context", "error", err)
		}
		return messages
	}
	if len(recordsByMessage) == 0 {
		return messages
	}
	out := append([]memory.HistoryMessage(nil), messages...)
	for i := range out {
		records := recordsByMessage[out[i].ID]
		if len(records) == 0 {
			continue
		}
		visible := stripAgodeskAttachmentBlock(out[i].Content)
		content := buildAgodeskMessageWithAttachments(s, visible, records)
		out[i].Content = content
		out[i].MultiContent = nil
		out[i].ChatCompletionMessage.Content = content
		out[i].ChatCompletionMessage.MultiContent = nil
	}
	return out
}

func stripAgodeskAttachmentBlock(content string) string {
	out := agodeskAttachmentBlockPattern.ReplaceAllString(content, "")
	return strings.TrimSpace(out)
}

func agodeskAttachmentInlineText(s *Server, relPath string) string {
	absPath, ok := agodeskAttachmentAbsolutePath(s, relPath)
	if !ok {
		return ""
	}
	data, err := os.ReadFile(absPath)
	if err != nil {
		return ""
	}
	text := strings.TrimSpace(string(data))
	if text == "" {
		return ""
	}
	if len(text) > agodeskAttachmentInlineTextMaxBytes {
		text = text[:agodeskAttachmentInlineTextMaxBytes] + "\n[truncated]"
	}
	return text
}

func agodeskChatAttachmentItem(s *Server, record memory.AgoDeskAttachmentRecord) agodesk.ChatAttachmentItem {
	pathValue := ""
	if strings.TrimSpace(record.RelativePath) != "" {
		mediaRel := strings.TrimPrefix(filepath.ToSlash(record.RelativePath), "attachments/")
		pathValue = "/api/agodesk/media/attachments/" + pathpkg.Join(mediaRel)
		pathValue = signAgodeskMediaAssetPath(s, pathValue, time.Now())
	}
	return agodesk.ChatAttachmentItem{
		AttachmentID: strings.TrimSpace(record.AttachmentID),
		Kind:         strings.TrimSpace(record.Kind),
		Path:         pathValue,
		MimeType:     strings.TrimSpace(record.MimeType),
		Filename:     strings.TrimSpace(record.Filename),
		SizeBytes:    record.SizeBytes,
		SHA256:       strings.TrimSpace(record.SHA256),
	}
}

func agodeskAttachmentRelativePath(conversationID, attachmentID, filename string) string {
	return filepath.ToSlash(filepath.Join(
		"attachments",
		"agodesk",
		sanitizeFilename(conversationID),
		sanitizeFilename(attachmentID),
		sanitizeFilename(filename),
	))
}

func agodeskAttachmentAgentPath(relativePath string) string {
	relativePath = strings.TrimPrefix(filepath.ToSlash(strings.TrimSpace(relativePath)), "/")
	if relativePath == "" {
		return ""
	}
	return "agent_workspace/workdir/" + relativePath
}

func agodeskAttachmentAbsolutePath(s *Server, relativePath string) (string, bool) {
	if s == nil || s.Cfg == nil {
		return "", false
	}
	s.CfgMu.RLock()
	workspaceDir := strings.TrimSpace(s.Cfg.Directories.WorkspaceDir)
	s.CfgMu.RUnlock()
	if workspaceDir == "" {
		return "", false
	}
	relativePath = filepath.FromSlash(strings.TrimPrefix(filepath.ToSlash(strings.TrimSpace(relativePath)), "/"))
	if relativePath == "" || strings.HasPrefix(relativePath, "..") || strings.Contains(relativePath, "\x00") {
		return "", false
	}
	if !strings.HasPrefix(filepath.ToSlash(relativePath), "attachments/") {
		return "", false
	}
	target := filepath.Join(workspaceDir, relativePath)
	if !pathStaysWithinDir(workspaceDir, target) {
		return "", false
	}
	return target, true
}

func normalizeAgodeskAttachmentMime(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	if value == "" {
		return ""
	}
	if parsed, _, err := mime.ParseMediaType(value); err == nil {
		value = parsed
	}
	return value
}

func agodeskAttachmentMimeAllowed(mimeType string) bool {
	mimeType = normalizeAgodeskAttachmentMime(mimeType)
	return strings.HasPrefix(mimeType, "image/") ||
		strings.HasPrefix(mimeType, "text/") ||
		mimeType == "application/pdf"
}

func agodeskAttachmentMimeCompatible(declared, sniffed string) bool {
	declared = normalizeAgodeskAttachmentMime(declared)
	sniffed = normalizeAgodeskAttachmentMime(sniffed)
	if !agodeskAttachmentMimeAllowed(declared) {
		return false
	}
	if sniffed == "" {
		return true
	}
	if !agodeskAttachmentMimeAllowed(sniffed) {
		return false
	}
	if declared == sniffed {
		return true
	}
	if strings.HasPrefix(declared, "text/") && strings.HasPrefix(sniffed, "text/") {
		return true
	}
	if strings.HasPrefix(declared, "image/") && strings.HasPrefix(sniffed, "image/") {
		return true
	}
	return false
}

func agodeskAttachmentKind(mimeType, filename string) string {
	mimeType = normalizeAgodeskAttachmentMime(mimeType)
	switch {
	case strings.HasPrefix(mimeType, "image/"):
		return "image"
	case strings.HasPrefix(mimeType, "text/"):
		return "text"
	case mimeType == "application/pdf":
		return "document"
	case strings.HasPrefix(mimeType, "audio/"):
		return "audio"
	case strings.HasPrefix(mimeType, "video/"):
		return "video"
	default:
		switch strings.ToLower(filepath.Ext(filename)) {
		case ".png", ".jpg", ".jpeg", ".gif", ".webp", ".bmp":
			return "image"
		case ".txt", ".md", ".csv", ".json", ".log":
			return "text"
		case ".pdf":
			return "document"
		default:
			return "binary"
		}
	}
}

func attachmentRecordsToIDs(records []memory.AgoDeskAttachmentRecord) []string {
	ids := make([]string, 0, len(records))
	for _, record := range records {
		if id := strings.TrimSpace(record.AttachmentID); id != "" {
			ids = append(ids, id)
		}
	}
	return ids
}

func agodeskStringSliceContains(values []string, want string) bool {
	want = strings.TrimSpace(want)
	for _, value := range values {
		if strings.TrimSpace(value) == want {
			return true
		}
	}
	return false
}
