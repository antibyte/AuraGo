package desktop

import (
	"bytes"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestNormalizeSFTPRemotePathTreatsClientRootAsRemoteHome(t *testing.T) {
	t.Parallel()

	cases := map[string]string{
		"/":                  ".",
		`\\`:                 ".",
		"/Documents":         "Documents",
		"/Documents/file.md": "Documents/file.md",
		"Documents/file.md":  "Documents/file.md",
	}
	for raw, want := range cases {
		got, err := normalizeSFTPRemotePath(raw)
		if err != nil {
			t.Fatalf("normalizeSFTPRemotePath(%q): %v", raw, err)
		}
		if got != want {
			t.Fatalf("normalizeSFTPRemotePath(%q) = %q, want %q", raw, got, want)
		}
	}
}

func TestNormalizeSFTPRemotePathRejectsTraversalAndSensitiveAbsolutePaths(t *testing.T) {
	t.Parallel()

	for _, raw := range []string{
		"../.ssh/authorized_keys",
		"/../etc/passwd",
		"/etc/shadow",
		"/root/.ssh/id_rsa",
		"~/.ssh/config",
		"Documents/\x00secret",
	} {
		if got, err := normalizeSFTPRemotePath(raw); err == nil {
			t.Fatalf("normalizeSFTPRemotePath(%q) = %q, want error", raw, got)
		}
	}
}

func TestHandleSFTPUploadRejectsGuardedDeviceMismatch(t *testing.T) {
	t.Parallel()

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	if err := writer.WriteField("device_id", "device-b"); err != nil {
		t.Fatalf("write device_id field: %v", err)
	}
	if err := writer.WriteField("remote_path", "Documents/file.txt"); err != nil {
		t.Fatalf("write remote_path field: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close multipart writer: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/desktop/sftp/upload?device_id=device-a", &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	rec := httptest.NewRecorder()
	HandleSFTPUpload(nil, nil, nil)(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusBadRequest, rec.Body.String())
	}
}
