package desktop

import (
	"bytes"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
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

func TestSFTPJSONMutationsRequireAuthorizedDeviceID(t *testing.T) {
	cases := []struct {
		name    string
		handler http.HandlerFunc
		body    string
	}{
		{"mkdir", HandleSFTPMkdir(nil, nil, nil), `{"device_id":"other","path":"docs"}`},
		{"delete", HandleSFTPDelete(nil, nil, nil), `{"device_id":"other","path":"docs/file.txt"}`},
		{"rename", HandleSFTPRename(nil, nil, nil), `{"device_id":"other","old_path":"a","new_path":"b"}`},
		{"copy", HandleSFTPCopy(nil, nil, nil), `{"device_id":"other","src_path":"a","dst_path":"b"}`},
		{"move", HandleSFTPMove(nil, nil, nil), `{"device_id":"other","src_path":"a","dst_path":"b"}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/api/desktop/sftp/"+tc.name+"?device_id=authorized", strings.NewReader(tc.body))
			req.Header.Set("Content-Type", "application/json")
			resp := httptest.NewRecorder()
			tc.handler.ServeHTTP(resp, req)
			if resp.Code != http.StatusBadRequest || !strings.Contains(resp.Body.String(), "device_id mismatch") {
				t.Fatalf("status = %d body = %s", resp.Code, resp.Body.String())
			}
		})
	}
}

func TestSFTPDeviceIDAllowsGuardedAndAdminMultipartCases(t *testing.T) {
	matched := httptest.NewRequest(http.MethodPost, "/?device_id=authorized", nil)
	if !validateSFTPDeviceID(httptest.NewRecorder(), matched, "authorized", true) {
		t.Fatal("matching guarded device rejected")
	}
	adminUpload := httptest.NewRequest(http.MethodPost, "/", nil)
	if !validateSFTPDeviceID(httptest.NewRecorder(), adminUpload, "authorized", false) {
		t.Fatal("multipart upload without query device was rejected")
	}
}

func TestSFTPUploadRejectsMultipartDeviceMismatch(t *testing.T) {
	var body bytes.Buffer
	form := multipart.NewWriter(&body)
	_ = form.WriteField("device_id", "other")
	_ = form.WriteField("remote_path", "docs/file.txt")
	file, err := form.CreateFormFile("file", "file.txt")
	if err != nil {
		t.Fatal(err)
	}
	_, _ = file.Write([]byte("data"))
	if err := form.Close(); err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPost, "/api/desktop/sftp/upload?device_id=authorized", &body)
	req.Header.Set("Content-Type", form.FormDataContentType())
	resp := httptest.NewRecorder()
	HandleSFTPUpload(nil, nil, nil).ServeHTTP(resp, req)
	if resp.Code != http.StatusBadRequest || !strings.Contains(resp.Body.String(), "device_id mismatch") {
		t.Fatalf("status = %d body = %s", resp.Code, resp.Body.String())
	}
}
