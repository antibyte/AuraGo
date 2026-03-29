package server

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"

	"aurago/internal/config"
)

func TestHandleBackupImportMasksDecryptionErrors(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	s := &Server{Cfg: cfg, Logger: slog.Default()}

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, err := writer.CreateFormFile("file", "backup.ago")
	if err != nil {
		t.Fatalf("CreateFormFile: %v", err)
	}
	if _, err := part.Write([]byte("AGOEdefinitely-not-valid")); err != nil {
		t.Fatalf("Write: %v", err)
	}
	if err := writer.WriteField("password", "wrong-password"); err != nil {
		t.Fatalf("WriteField: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/backup/import", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	rec := httptest.NewRecorder()

	handleBackupImport(s).ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusBadRequest, rec.Body.String())
	}
	var payload map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if payload["error"] != "decryption_failed" {
		t.Fatalf("error = %q, want decryption_failed", payload["error"])
	}
	if payload["message"] == "" || bytes.Contains([]byte(payload["message"]), []byte("cipher")) {
		t.Fatalf("expected redacted decryption message, got %q", payload["message"])
	}
}
