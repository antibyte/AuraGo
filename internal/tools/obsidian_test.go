package tools

import (
	"aurago/internal/testutil"
	"encoding/json"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"testing"

	"aurago/internal/config"
)

func testObsidianConfig(t *testing.T, serverURL string) config.ObsidianConfig {
	t.Helper()

	parsed, err := url.Parse(serverURL)
	if err != nil {
		t.Fatalf("parse server url: %v", err)
	}
	host, portText, err := net.SplitHostPort(parsed.Host)
	if err != nil {
		t.Fatalf("split host/port: %v", err)
	}
	port, err := strconv.Atoi(portText)
	if err != nil {
		t.Fatalf("atoi port: %v", err)
	}

	return config.ObsidianConfig{
		Enabled:        true,
		Host:           host,
		Port:           port,
		UseHTTPS:       parsed.Scheme == "https",
		InsecureSSL:    true,
		APIKey:         "test-key",
		RequestTimeout: 5,
	}
}

func decodeToolResult(t *testing.T, raw string) map[string]interface{} {
	t.Helper()

	var result map[string]interface{}
	if err := json.Unmarshal([]byte(raw), &result); err != nil {
		t.Fatalf("decode tool result: %v\nraw=%s", err, raw)
	}
	return result
}

func TestObsidianReadOnlyBlocksMutations(t *testing.T) {
	cfg := config.ObsidianConfig{ReadOnly: true}

	tests := map[string]string{
		"create_note":     ObsidianCreateNote(cfg, nil, "note.md", "content", slog.Default()),
		"update_note":     ObsidianUpdateNote(cfg, nil, "note.md", "content", slog.Default()),
		"patch_note":      ObsidianPatchNote(cfg, nil, "note.md", "content", "", "", "append", slog.Default()),
		"delete_note":     ObsidianDeleteNote(cfg, nil, "note.md", slog.Default()),
		"periodic_write":  ObsidianPeriodicNote(cfg, nil, "daily", "content", slog.Default()),
		"execute_command": ObsidianExecuteCommand(cfg, nil, "workspace:save-file", slog.Default()),
	}

	for name, raw := range tests {
		t.Run(name, func(t *testing.T) {
			result := decodeToolResult(t, raw)
			if result["status"] != "error" {
				t.Fatalf("status = %v, raw=%s, want error", result["status"], raw)
			}
			if !strings.Contains(result["message"].(string), "read-only mode") {
				t.Fatalf("message = %q, want read-only denial", result["message"])
			}
		})
	}
}

func TestObsidianUpdateNoteVerifiesWrittenContent(t *testing.T) {
	noteContent := "before"

	server := testutil.NewHTTPSServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer test-key" {
			t.Fatalf("authorization header = %q", got)
		}
		if r.URL.Path != "/vault/note.md" {
			t.Fatalf("unexpected path %q", r.URL.Path)
		}

		switch r.Method {
		case http.MethodGet:
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"path":    "note.md",
				"content": noteContent,
			})
		case http.MethodPut:
			if got := r.Header.Get("Content-Type"); got != "text/plain" {
				t.Fatalf("content-type = %q, want text/plain", got)
			}
			body, _ := io.ReadAll(r.Body)
			noteContent = string(body)
			w.WriteHeader(http.StatusNoContent)
		default:
			t.Fatalf("unexpected method %s", r.Method)
		}
	}))
	defer server.Close()

	cfg := testObsidianConfig(t, server.URL)
	raw := ObsidianUpdateNote(cfg, nil, "note.md", "after", slog.Default())
	result := decodeToolResult(t, raw)

	if result["status"] != "ok" {
		t.Fatalf("status = %v, raw=%s", result["status"], raw)
	}
	if result["verified"] != true {
		t.Fatalf("verified = %v", result["verified"])
	}
	if !strings.Contains(result["content"].(string), "after") {
		t.Fatalf("content = %q, want wrapped updated content", result["content"])
	}
}

func TestObsidianPatchNoteRejectsSilentNoOp(t *testing.T) {
	noteContent := "before"

	server := testutil.NewHTTPSServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer test-key" {
			t.Fatalf("authorization header = %q", got)
		}
		if r.URL.Path != "/vault/note.md" {
			t.Fatalf("unexpected path %q", r.URL.Path)
		}

		switch r.Method {
		case http.MethodGet:
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"path":    "note.md",
				"content": noteContent,
			})
		case http.MethodPatch:
			if got := r.Header.Get("Content-Type"); got != "text/plain" {
				t.Fatalf("content-type = %q, want text/plain", got)
			}
			w.WriteHeader(http.StatusNoContent)
		default:
			t.Fatalf("unexpected method %s", r.Method)
		}
	}))
	defer server.Close()

	cfg := testObsidianConfig(t, server.URL)
	raw := ObsidianPatchNote(cfg, nil, "note.md", "new line", "", "", "append", slog.Default())
	result := decodeToolResult(t, raw)

	if result["status"] != "error" {
		t.Fatalf("status = %v, raw=%s", result["status"], raw)
	}
	if !strings.Contains(result["message"].(string), "verification failed") {
		t.Fatalf("message = %q", result["message"])
	}
}
