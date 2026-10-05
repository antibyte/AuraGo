package agent

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"aurago/internal/config"
)

const c102PolicyMarker = `<meta http-equiv="Content-Security-Policy" content="default-src 'none'; img-src data:; style-src 'unsafe-inline'; font-src data:">`

func TestDecodeDocumentCreatorArgsReadsBlockRemoteContent(t *testing.T) {
	for _, tc := range []struct {
		name   string
		params map[string]interface{}
		want   bool
	}{
		{"absent", map[string]interface{}{"operation": "html_to_pdf"}, false},
		{"true", map[string]interface{}{"operation": "html_to_pdf", "block_remote_content": true}, true},
		{"false", map[string]interface{}{"operation": "html_to_pdf", "block_remote_content": false}, false},
		{"not a boolean", map[string]interface{}{"operation": "html_to_pdf", "block_remote_content": "true"}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req := decodeDocumentCreatorArgs(ToolCall{Action: "document_creator", Params: tc.params})
			if req.BlockRemoteContent != tc.want {
				t.Fatalf("BlockRemoteContent = %v, want %v", req.BlockRemoteContent, tc.want)
			}
		})
	}
}

func TestDocumentCreatorSchemaOffersBlockRemoteContent(t *testing.T) {
	for _, schema := range builtinToolSchemas(ToolFeatureFlags{DocumentCreatorEnabled: true}) {
		if schema.Function == nil || schema.Function.Name != "document_creator" {
			continue
		}
		parameters, _ := schema.Function.Parameters.(map[string]interface{})
		properties, _ := parameters["properties"].(map[string]interface{})
		property, ok := properties["block_remote_content"].(map[string]interface{})
		if !ok || property["type"] != "boolean" || !strings.Contains(property["description"].(string), "untrusted HTML") {
			t.Fatalf("block_remote_content property = %#v", properties["block_remote_content"])
		}
		return
	}
	t.Fatal("document_creator schema missing")
}

// TestDispatchDocumentCreatorPassesBlockRemoteContent drives the real dispatch case against a
// Gotenberg stand-in and checks the uploaded page.
func TestDispatchDocumentCreatorPassesBlockRemoteContent(t *testing.T) {
	var mu sync.Mutex
	var pages []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseMultipartForm(8 << 20); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		for _, header := range r.MultipartForm.File["files"] {
			if header.Filename != "index.html" {
				continue
			}
			file, err := header.Open()
			if err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			data, _ := io.ReadAll(file)
			file.Close()
			mu.Lock()
			pages = append(pages, string(data))
			mu.Unlock()
		}
		_, _ = w.Write([]byte("%PDF-1.4 fake"))
	}))
	t.Cleanup(server.Close)

	cfg := &config.Config{}
	cfg.Directories.WorkspaceDir = t.TempDir()
	cfg.Tools.DocumentCreator.Enabled = true
	cfg.Tools.DocumentCreator.Backend = "gotenberg"
	cfg.Tools.DocumentCreator.OutputDir = t.TempDir()
	cfg.Tools.DocumentCreator.Gotenberg.URL = server.URL
	const page = `<html><head><title>T</title></head><body><img src="http://192.168.1.1/x.png"></body></html>`

	for _, block := range []bool{false, true} {
		params := map[string]interface{}{"operation": "html_to_pdf", "content": page}
		if block {
			params["block_remote_content"] = true
		}
		out, handled := dispatchExec(context.Background(), ToolCall{Action: "document_creator", Params: params}, &DispatchContext{
			Cfg:    cfg,
			Logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
		})
		if !handled || !strings.Contains(out, `"status":"success"`) {
			t.Fatalf("block=%v: handled=%v out=%s", block, handled, out)
		}
	}

	mu.Lock()
	defer mu.Unlock()
	if len(pages) != 2 {
		t.Fatalf("gotenberg received %d pages, want 2", len(pages))
	}
	if pages[0] != page {
		t.Fatalf("default dispatch changed the page: %q", pages[0])
	}
	if want := `<html><head>` + c102PolicyMarker + `<title>T</title>`; !strings.HasPrefix(pages[1], want) {
		t.Fatalf("blocked dispatch page = %q, want prefix %q", pages[1], want)
	}
}
