package tools

import (
	"encoding/base64"
	"io"
	"mime"
	"mime/multipart"
	"net/mail"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"aurago/internal/config"
)

var emailTestTime = time.Date(2026, 10, 4, 9, 30, 0, 0, time.UTC)

func TestBuildEmailMessageWithoutAttachmentsIsUnchanged(t *testing.T) {
	got := buildEmailMessage("a@example.com", "b@example.com", "Hallo", "Text", emailTestTime, nil, "")
	want := "From: a@example.com\r\nTo: b@example.com\r\nSubject: =?UTF-8?B?SGFsbG8=?=\r\n" +
		"Date: Sun, 04 Oct 2026 09:30:00 +0000\r\nMIME-Version: 1.0\r\n" +
		"Content-Type: text/plain; charset=UTF-8\r\nContent-Transfer-Encoding: 8bit\r\n\r\nText"
	if got != want {
		t.Fatalf("message changed:\n%q\nwant\n%q", got, want)
	}
}

func TestBuildEmailMessageWithAttachmentIsMultipart(t *testing.T) {
	att := []EmailAttachment{{Name: "Bericht ä.pdf", ContentType: "application/pdf", Data: []byte("%PDF-1.4 \x00\x01 binary")}}
	raw := buildEmailMessage("a@example.com", "b@example.com", "Bericht", "Siehe Anhang", emailTestTime, att, "aurago-test-boundary")
	msg, err := mail.ReadMessage(strings.NewReader(raw))
	if err != nil {
		t.Fatalf("ReadMessage: %v", err)
	}
	mediaType, params, err := mime.ParseMediaType(msg.Header.Get("Content-Type"))
	if err != nil || mediaType != "multipart/mixed" || params["boundary"] != "aurago-test-boundary" {
		t.Fatalf("content type = %q %v %v", mediaType, params, err)
	}
	reader := multipart.NewReader(msg.Body, params["boundary"])
	text, err := reader.NextPart()
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(text)
	if strings.TrimSpace(string(body)) != "Siehe Anhang" {
		t.Fatalf("text part = %q", body)
	}
	file, err := reader.NextPart()
	if err != nil {
		t.Fatal(err)
	}
	if file.FileName() != "Bericht ä.pdf" {
		t.Fatalf("file name = %q", file.FileName())
	}
	encoded, _ := io.ReadAll(file)
	decoded, err := base64.StdEncoding.DecodeString(strings.ReplaceAll(string(encoded), "\r\n", ""))
	if err != nil || string(decoded) != "%PDF-1.4 \x00\x01 binary" {
		t.Fatalf("attachment = %q, %v", decoded, err)
	}
	for _, line := range strings.Split(string(encoded), "\r\n") {
		if len(line) > 76 {
			t.Fatalf("base64 line longer than 76 characters: %d", len(line))
		}
	}
}

func TestLoadEmailAttachmentsChecksPathsAndSize(t *testing.T) {
	workspace := t.TempDir()
	cfg := &config.Config{}
	cfg.Directories.WorkspaceDir = workspace
	small := filepath.Join(workspace, "a.txt")
	if err := os.WriteFile(small, []byte("hallo"), 0o600); err != nil {
		t.Fatal(err)
	}
	files, err := LoadEmailAttachments(cfg, []string{small})
	if err != nil || len(files) != 1 || files[0].Name != "a.txt" || string(files[0].Data) != "hallo" ||
		!strings.HasPrefix(files[0].ContentType, "text/plain") {
		t.Fatalf("files = %+v, %v", files, err)
	}
	if _, err := LoadEmailAttachments(cfg, []string{"../etc/passwd"}); err == nil {
		t.Fatal("paths outside the workspace must be rejected")
	}
	big := filepath.Join(workspace, "big.bin")
	if err := os.WriteFile(big, make([]byte, emailMaxAttachmentBytes+1), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadEmailAttachments(cfg, []string{big}); err == nil {
		t.Fatal("attachments over the size limit must be rejected")
	}
	many := make([]string, emailMaxAttachments+1)
	for i := range many {
		many[i] = small
	}
	if _, err := LoadEmailAttachments(cfg, many); err == nil {
		t.Fatal("too many attachments must be rejected")
	}
}
