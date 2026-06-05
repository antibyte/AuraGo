package obsidian

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"aurago/internal/config"
)

func TestNewClientRejectsSSRFMetadataHost(t *testing.T) {
	_, err := NewClient(config.ObsidianConfig{
		Enabled: true,
		Host:    "169.254.169.254",
		APIKey:  "secret",
	}, nil)
	if err == nil || !strings.Contains(err.Error(), "SSRF") {
		t.Fatalf("expected SSRF rejection for metadata host, got %v", err)
	}
}

func TestNewClientRejectsURLHost(t *testing.T) {
	_, err := NewClient(config.ObsidianConfig{
		Enabled: true,
		Host:    "http://127.0.0.1/admin",
		APIKey:  "secret",
	}, nil)
	if err == nil || !strings.Contains(err.Error(), "not a URL") {
		t.Fatalf("expected URL host rejection, got %v", err)
	}
}

func TestReadNoteBuildsRequest(t *testing.T) {
	var gotPath, gotAuth, gotAccept string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.EscapedPath()
		gotAuth = r.Header.Get("Authorization")
		gotAccept = r.Header.Get("Accept")
		if r.Method != http.MethodGet {
			t.Fatalf("method = %s, want GET", r.Method)
		}
		_ = json.NewEncoder(w).Encode(NoteJSON{
			Content: "hello",
			Tags:    []string{"tag"},
		})
	}))
	defer server.Close()

	client := &Client{
		baseURL:    server.URL,
		apiKey:     "secret",
		httpClient: server.Client(),
	}

	note, err := client.ReadNote(context.Background(), "folder/My Note.md")
	if err != nil {
		t.Fatalf("ReadNote returned error: %v", err)
	}
	if note.Content != "hello" || note.Path != "folder/My Note.md" {
		t.Fatalf("unexpected note: %+v", note)
	}
	if gotPath != "/vault/folder%2FMy%20Note.md" {
		t.Fatalf("path = %q, want encoded vault path", gotPath)
	}
	if gotAuth != "Bearer secret" {
		t.Fatalf("Authorization = %q", gotAuth)
	}
	if gotAccept != "application/vnd.olrapi.note+json" {
		t.Fatalf("Accept = %q", gotAccept)
	}
}
