package tools

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"aurago/internal/security"
)

func TestPaperlessDocumentIDsCannotChangeEndpoint(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { calls.Add(1) }))
	defer srv.Close()
	cfg := PaperlessConfig{URL: srv.URL, APIToken: "fixture-token"}
	for _, id := range []string{"", "0", "-1", "+1", "01", "1/../../tags", "../", "1?x=y", "1#fragment", "%31", "1%2f..", " 1", "9223372036854775808"} {
		for _, out := range []string{PaperlessGet(cfg, id), PaperlessDownload(cfg, id), PaperlessUpdate(cfg, id, "title", "", "", ""), PaperlessDelete(cfg, id)} {
			if !strings.Contains(out, `"status":"error"`) {
				t.Errorf("invalid ID %q accepted: %s", id, out)
			}
		}
	}
	if calls.Load() != 0 {
		t.Fatal("invalid ID made a network request")
	}
	if !validPaperlessDocumentID("123") {
		t.Fatal("valid ID rejected")
	}
}

func TestObsidianCacheTracksVaultRotationAndTLS(t *testing.T) {
	var seen []string
	var mu sync.Mutex
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		seen = append(seen, r.Header.Get("Authorization"))
		mu.Unlock()
		io.WriteString(w, `{"content":"fixture"}`)
	}))
	defer srv.Close()
	v, err := security.NewVault(strings.Repeat("01", 32), filepath.Join(t.TempDir(), "vault.bin"))
	if err != nil {
		t.Fatal(err)
	}
	cfg := testObsidianConfig(t, srv.URL)
	cfg.APIKey = ""
	for _, key := range []string{"fixture-first", "fixture-second"} {
		if err := v.WriteSecret("obsidian_api_key", key); err != nil {
			t.Fatal(err)
		}
		c, err := newObsidianClient(cfg, v)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := c.ReadNote(context.Background(), "fixture.md"); err != nil {
			t.Fatal(err)
		}
	}
	mu.Lock()
	valid := len(seen) == 2 && seen[0] == "Bearer fixture-first" && seen[1] == "Bearer fixture-second"
	mu.Unlock()
	if !valid {
		t.Fatal("credential rotation ignored")
	}
	cfg.InsecureSSL = false
	c, err := newObsidianClient(cfg, v)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := c.ReadNote(context.Background(), "fixture.md"); err == nil {
		t.Fatal("reused insecure client after TLS policy change")
	}
}

func TestCloudStorageDeletionRejectsRootBeforeNetwork(t *testing.T) {
	for _, path := range []string{"", "/", "//", ".", "/./", "folder/..", "\\", "%2f", "%252f", "%2e", "%2e%2e", "a/%2e%2e", " / "} {
		if err := validateCloudDeletePath(path); err == nil {
			t.Errorf("root accepted: %q", path)
		}
		for _, out := range []string{WebDAVDelete(WebDAVConfig{}, path), ExecuteKoofr(KoofrConfig{}, "delete", path, "", "", "", "", ""), (&OneDriveClient{}).deleteItem(path)} {
			if !strings.Contains(out, "root") && !strings.Contains(out, "traversal") {
				t.Errorf("root not rejected before network: %q %s", path, out)
			}
		}
	}
	for _, path := range []string{"/folder/file", "folder/file", "a b"} {
		if err := validateCloudDeletePath(path); err != nil {
			t.Error(err)
		}
	}
}

type interruptedDownload struct{}

func (interruptedDownload) Read(p []byte) (int, error) {
	copy(p, "partial")
	return 7, io.ErrUnexpectedEOF
}
func (interruptedDownload) Close() error { return nil }

func TestOneDriveDownloadRequiresSuccessfulBoundedResponse(t *testing.T) {
	for _, tc := range []struct {
		status int
		body   io.ReadCloser
		want   string
	}{
		{http.StatusForbidden, io.NopCloser(strings.NewReader("denied")), "Download failed"},
		{http.StatusOK, interruptedDownload{}, "could not be read"},
		{http.StatusOK, io.NopCloser(strings.NewReader(strings.Repeat("x", 512*1024))), "truncated&#34;:false"},
		{http.StatusOK, io.NopCloser(strings.NewReader(strings.Repeat("x", 512*1024+1))), "truncated&#34;:true"},
	} {
		out := readOneDriveFileResponse(&http.Response{StatusCode: tc.status, Body: tc.body}, "fixture.txt")
		if !strings.Contains(out, tc.want) {
			t.Fatalf("status %d missing %q in bounded response (length %d)", tc.status, tc.want, len(out))
		}
	}
}
