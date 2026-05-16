package agentmail

import (
	"encoding/base64"
	"os"
	"path/filepath"
	"testing"
)

func TestPrepareAttachmentsAcceptsWorkspacePathAndBase64(t *testing.T) {
	t.Parallel()

	workspace := t.TempDir()
	reportPath := filepath.Join(workspace, "reports", "report.txt")
	if err := os.MkdirAll(filepath.Dir(reportPath), 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	if err := os.WriteFile(reportPath, []byte("hello"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	got, err := PrepareAttachments(workspace, 1, []AttachmentInput{
		{Path: filepath.Join("reports", "report.txt"), ContentType: "text/plain"},
		{Filename: "inline.txt", ContentType: "text/plain", Base64: base64.StdEncoding.EncodeToString([]byte("inline"))},
	})
	if err != nil {
		t.Fatalf("PrepareAttachments() error = %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("len = %d, want 2", len(got))
	}
	if got[0].Filename != "report.txt" || got[0].ContentBase64 != "aGVsbG8=" {
		t.Fatalf("unexpected file attachment: %+v", got[0])
	}
	if got[1].Filename != "inline.txt" || got[1].ContentBase64 != "aW5saW5l" {
		t.Fatalf("unexpected inline attachment: %+v", got[1])
	}
}

func TestPrepareAttachmentsRejectsOutsideWorkspace(t *testing.T) {
	t.Parallel()

	workspace := t.TempDir()
	outsideDir := t.TempDir()
	outsidePath := filepath.Join(outsideDir, "secret.txt")
	if err := os.WriteFile(outsidePath, []byte("secret"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	if _, err := PrepareAttachments(workspace, 1, []AttachmentInput{{Path: outsidePath}}); err == nil {
		t.Fatal("PrepareAttachments() expected error for outside path")
	}
}

func TestPrepareAttachmentsRejectsOversizedInput(t *testing.T) {
	t.Parallel()

	workspace := t.TempDir()
	path := filepath.Join(workspace, "large.bin")
	if err := os.WriteFile(path, []byte("toolarge"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	if _, err := PrepareAttachments(workspace, 0, []AttachmentInput{{Path: path}}); err == nil {
		t.Fatal("PrepareAttachments() expected error when file exceeds zero MB limit")
	}
}
