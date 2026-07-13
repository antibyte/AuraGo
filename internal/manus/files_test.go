package manus

import (
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"testing"
)

func TestResolveUploadPathAllowsRegularWorkspaceFile(t *testing.T) {
	t.Parallel()

	workspace := t.TempDir()
	path := filepath.Join(workspace, "reports", "result.pdf")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("pdf"), 0o600); err != nil {
		t.Fatal(err)
	}
	file, err := ResolveUploadPath(workspace, filepath.Join("reports", "result.pdf"), 20<<20)
	if err != nil {
		t.Fatalf("ResolveUploadPath() error = %v", err)
	}
	if file.Path != path || file.Filename != "result.pdf" || file.Size != 3 {
		t.Fatalf("ResolveUploadPath() = %#v", file)
	}
}

func TestResolveUploadPathRejectsSymlinkEvenWhenTargetStaysInWorkspace(t *testing.T) {
	t.Parallel()

	workspace := t.TempDir()
	target := filepath.Join(workspace, "report.pdf")
	if err := os.WriteFile(target, []byte("pdf"), 0o600); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(workspace, "report-link.pdf")
	if err := os.Symlink(target, link); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	if _, err := ResolveUploadPath(workspace, link, 1024); err == nil {
		t.Fatal("symlink upload was accepted")
	}
}

func TestResolveUploadPathRejectsEscapesSecretsScriptsAndOversize(t *testing.T) {
	t.Parallel()

	workspace := t.TempDir()
	outside := filepath.Join(t.TempDir(), "outside.txt")
	if err := os.WriteFile(outside, []byte("outside"), 0o600); err != nil {
		t.Fatal(err)
	}
	for name, content := range map[string][]byte{
		"config.yaml": []byte("secret"),
		".env":        []byte("secret"),
		"script.py":   []byte("print('x')"),
		"state.db":    []byte("sqlite"),
		"large.txt":   []byte("too large"),
	} {
		if err := os.WriteFile(filepath.Join(workspace, name), content, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	tests := map[string]struct {
		path string
		max  int64
	}{
		"escape":      {path: outside, max: 1024},
		"config":      {path: "config.yaml", max: 1024},
		"environment": {path: ".env", max: 1024},
		"script":      {path: "script.py", max: 1024},
		"database":    {path: "state.db", max: 1024},
		"oversize":    {path: "large.txt", max: 2},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			if _, err := ResolveUploadPath(workspace, tc.path, tc.max); err == nil {
				t.Fatal("error = nil, want rejection")
			}
		})
	}
}

func TestValidateRemoteFileURLRejectsNonHTTPSAndPrivateHosts(t *testing.T) {
	t.Parallel()

	for _, raw := range []string{
		"http://example.com/file.pdf",
		"https://127.0.0.1/file.pdf",
		"https://10.1.2.3/file.pdf",
		"https://[::1]/file.pdf",
		"https://user:pass@example.com/file.pdf",
	} {
		if _, err := ValidateRemoteFileURL(raw); err == nil {
			t.Errorf("ValidateRemoteFileURL(%q) error = nil", raw)
		}
	}
	parsed, err := ValidateRemoteFileURL("https://203.0.113.10/file.pdf")
	if err != nil || parsed.Hostname() != "203.0.113.10" {
		t.Fatalf("public URL rejected: %v, %#v", err, parsed)
	}
}

func TestSafeAttachmentFilenamePreventsTraversal(t *testing.T) {
	t.Parallel()

	if got := SafeAttachmentFilename(`..\..\report?.pdf`); got != "report_.pdf" {
		t.Fatalf("SafeAttachmentFilename() = %q, want report_.pdf", got)
	}
	if got := SafeAttachmentFilename(".."); got != "attachment.bin" {
		t.Fatalf("SafeAttachmentFilename(..) = %q", got)
	}
}

func TestSecureFileClientRejectsRedirectToPrivateHostname(t *testing.T) {
	t.Parallel()

	client, err := NewClient("api-token", ClientConfig{BaseURL: "https://api.manus.ai"})
	if err != nil {
		t.Fatal(err)
	}
	redirectURL, _ := url.Parse("https://localhost/private")
	originalURL, _ := url.Parse("https://203.0.113.10/file")
	redirect := &http.Request{URL: redirectURL}
	redirect = redirect.WithContext(t.Context())
	original := &http.Request{URL: originalURL}
	if err := client.fileHTTPClient.CheckRedirect(redirect, []*http.Request{original}); err == nil {
		t.Fatal("redirect to private hostname was accepted")
	}
}
