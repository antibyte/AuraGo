package agentmail

import (
	"context"
	"encoding/base64"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
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

func TestDownloadAttachmentUsesUncredentialedHTTPSCDNAndCapsBody(t *testing.T) {
	const apiKey = "agentmail-api-secret"
	var cdnAuthorization string
	client := &http.Client{Transport: attachmentRoundTripFunc(func(r *http.Request) (*http.Response, error) {
		header := make(http.Header)
		if r.URL.Host == "api.agentmail.com" {
			if got := r.Header.Get("Authorization"); got != "Bearer "+apiKey {
				t.Errorf("AgentMail API authorization = %q", got)
			}
			header.Set("Content-Type", "application/json")
			body := `{"attachment_id":"att-1","filename":"report.txt","download_url":"https://8.8.8.8/signed/file?signature=fixture"}`
			return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(body)), Header: header}, nil
		}
		cdnAuthorization = r.Header.Get("Authorization")
		if r.URL.Host != "8.8.8.8" || r.URL.Path != "/signed/file" || r.URL.Query().Get("signature") != "fixture" {
			t.Errorf("signed CDN URL changed: %s?%s", r.URL.Path, r.URL.RawQuery)
		}
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader("download")), Header: header}, nil
	})}
	c, err := NewClient(ClientConfig{BaseURL: "https://api.agentmail.com", APIKey: apiKey, HTTPClient: client, DisableRetries: true})
	if err != nil {
		t.Fatal(err)
	}
	c.attachmentHTTPClient = attachmentRedirectPolicy(client)
	data, meta, err := c.DownloadAttachment(context.Background(), "inbox", "message", "att-1", 16)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "download" || meta.Filename != "report.txt" {
		t.Fatalf("download = %q, metadata = %#v", data, meta)
	}
	if cdnAuthorization != "" {
		t.Fatalf("AgentMail API credential reached CDN: %q", cdnAuthorization)
	}

	if _, _, err := c.DownloadAttachment(context.Background(), "inbox", "message", "att-1", 3); err == nil {
		t.Fatal("oversized CDN body was accepted")
	}
}

func TestAttachmentDownloadURLRejectsNonPublicOrUnsafeTargets(t *testing.T) {
	t.Setenv("AURAGO_SSRF_ALLOW_LOOPBACK", "0")
	for _, raw := range []string{
		"http://cdn.example.test/file",
		"https://user:pass@cdn.example.test/file",
		"https://cdn.example.test/file#fragment",
		"https://127.0.0.1/private",
	} {
		t.Run(strings.ReplaceAll(raw, "/", "_"), func(t *testing.T) {
			if _, err := attachmentDownloadURL(raw, nil); err == nil {
				t.Fatalf("unsafe attachment URL was accepted: %s", raw)
			}
		})
	}
}

func TestAttachmentRedirectRequiresPublicHTTPS(t *testing.T) {
	client := attachmentRedirectPolicy(&http.Client{})
	for _, target := range []string{"http://8.8.8.8/private", "https://127.0.0.1/private"} {
		to, err := url.Parse(target)
		if err != nil {
			t.Fatal(err)
		}
		from, _ := url.Parse("https://8.8.8.8/signed/file")
		req := &http.Request{URL: to, Header: make(http.Header)}
		if err := client.CheckRedirect(req, []*http.Request{{URL: from}}); err == nil {
			t.Fatalf("unsafe redirect target was accepted: %s", target)
		}
	}
}

type attachmentRoundTripFunc func(*http.Request) (*http.Response, error)

func (f attachmentRoundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
