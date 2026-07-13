package manus

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (fn roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return fn(req)
}

func TestClientListsCatalogsAndProjectSkills(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v2/project.list":
			_, _ = w.Write([]byte(`{"ok":true,"data":[{"id":"project-1","name":"Home Lab","instruction":"external"}]}`))
		case "/v2/connector.list":
			_, _ = w.Write([]byte(`{"ok":true,"data":[{"id":"connector-1","name":"GitHub","type":"builtin"}]}`))
		case "/v2/skill.list":
			if r.URL.Query().Get("project_id") != "project-1" {
				t.Fatalf("project_id = %q", r.URL.Query().Get("project_id"))
			}
			_, _ = w.Write([]byte(`{"ok":true,"data":[{"id":"skill-1","name":"Research","owner_type":"official"}]}`))
		default:
			t.Fatalf("unexpected catalog path %q", r.URL.Path)
		}
	}))
	defer server.Close()
	client, _ := NewClient("secret", ClientConfig{BaseURL: server.URL})

	projects, err := client.ListProjects(context.Background())
	if err != nil || len(projects) != 1 || projects[0].ID != "project-1" {
		t.Fatalf("ListProjects() = %#v, %v", projects, err)
	}
	connectors, err := client.ListConnectors(context.Background())
	if err != nil || len(connectors) != 1 || connectors[0].ID != "connector-1" {
		t.Fatalf("ListConnectors() = %#v, %v", connectors, err)
	}
	skills, err := client.ListSkills(context.Background(), "project-1")
	if err != nil || len(skills) != 1 || skills[0].ID != "skill-1" {
		t.Fatalf("ListSkills() = %#v, %v", skills, err)
	}
}

func TestClientUploadsValidatedLocalFileWithoutLeakingAPIKey(t *testing.T) {
	t.Parallel()

	workspace := t.TempDir()
	path := filepath.Join(workspace, "report.pdf")
	if err := os.WriteFile(path, []byte("payload"), 0o600); err != nil {
		t.Fatal(err)
	}
	apiServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v2/file.upload" {
			t.Fatalf("path = %q", r.URL.Path)
		}
		_, _ = w.Write([]byte(`{"ok":true,"file":{"id":"file-1","filename":"report.pdf","status":"pending"},"upload_url":"https://203.0.113.10/upload"}`))
	}))
	defer apiServer.Close()

	fileClient := &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.Method != http.MethodPut || req.URL.String() != "https://203.0.113.10/upload" {
			t.Fatalf("upload request = %s %s", req.Method, req.URL)
		}
		if req.Header.Get("x-manus-api-key") != "" {
			t.Fatal("Manus API key leaked to presigned upload host")
		}
		body, _ := io.ReadAll(req.Body)
		if string(body) != "payload" {
			t.Fatalf("upload body = %q", body)
		}
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(bytes.NewReader(nil)), Header: make(http.Header)}, nil
	})}
	client, _ := NewClient("secret", ClientConfig{BaseURL: apiServer.URL, FileHTTPClient: fileClient})
	local, err := ResolveUploadPath(workspace, "report.pdf", 1024)
	if err != nil {
		t.Fatal(err)
	}
	file, err := client.UploadLocalFile(context.Background(), local)
	if err != nil {
		t.Fatalf("UploadLocalFile() error = %v", err)
	}
	if file.ID != "file-1" {
		t.Fatalf("UploadLocalFile() = %#v", file)
	}
}

func TestClientDownloadsAttachmentBytes(t *testing.T) {
	t.Parallel()

	fileClient := &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode:    http.StatusOK,
			Body:          io.NopCloser(strings.NewReader("downloaded")),
			ContentLength: 10,
			Header:        make(http.Header),
		}, nil
	})}
	client, _ := NewClient("secret", ClientConfig{BaseURL: "https://api.manus.ai", FileHTTPClient: fileClient})
	payload, err := client.DownloadAttachment(context.Background(), TaskAttachment{
		Filename: `../result?.txt`, URL: "https://203.0.113.11/download",
	}, 1024)
	if err != nil {
		t.Fatalf("DownloadAttachment() error = %v", err)
	}
	if string(payload) != "downloaded" {
		t.Fatalf("downloaded content = %q", payload)
	}
}
