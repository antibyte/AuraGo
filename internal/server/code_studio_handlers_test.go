package server

import (
	"bytes"
	"context"
	"encoding/binary"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"aurago/internal/config"
)

func TestSanitizeCodeStudioPathRejectsTraversalAndEscapesWorkspace(t *testing.T) {
	tests := []struct {
		name    string
		raw     string
		want    string
		wantErr bool
	}{
		{name: "empty is workspace root", raw: "", want: "/workspace"},
		{name: "valid nested path", raw: "/workspace/src/main.go", want: "/workspace/src/main.go"},
		{name: "backslashes normalize", raw: `\workspace\src\main.go`, want: "/workspace/src/main.go"},
		{name: "reject traversal", raw: "/workspace/../etc/passwd", wantErr: true},
		{name: "reject outside workspace", raw: "/etc/passwd", wantErr: true},
		{name: "reject null byte", raw: "/workspace/main.go\x00", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := sanitizeCodeStudioPath(tt.raw)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("sanitizeCodeStudioPath(%q) returned nil error", tt.raw)
				}
				return
			}
			if err != nil {
				t.Fatalf("sanitizeCodeStudioPath(%q): %v", tt.raw, err)
			}
			if got != tt.want {
				t.Fatalf("sanitizeCodeStudioPath(%q) = %q, want %q", tt.raw, got, tt.want)
			}
		})
	}
}

func TestParseCodeStudioFindOutputSortsFoldersFirst(t *testing.T) {
	output := strings.Join([]string{
		"f|12|1710000001.0000000000|/workspace/main.go",
		"d|4096|1710000000.0000000000|/workspace/src",
		"f|8|1710000002.0000000000|/workspace/README.md",
	}, "\n") + "\n"

	entries, err := parseCodeStudioFindOutput(output)
	if err != nil {
		t.Fatalf("parseCodeStudioFindOutput: %v", err)
	}
	if len(entries) != 3 {
		t.Fatalf("len(entries) = %d, want 3", len(entries))
	}
	if entries[0].Name != "src" || entries[0].Type != "directory" {
		t.Fatalf("first entry = %#v, want src directory", entries[0])
	}
	if entries[1].Name != "main.go" || entries[2].Name != "README.md" {
		t.Fatalf("file order = %q, %q", entries[1].Name, entries[2].Name)
	}
	if entries[1].Modified.IsZero() {
		t.Fatal("modified timestamp was not parsed")
	}
}

func TestNormalizeCodeStudioExecTimeoutCapsAt300Seconds(t *testing.T) {
	if got := normalizeCodeStudioExecTimeout(999); got != 300*time.Second {
		t.Fatalf("timeout = %s, want 300s", got)
	}
	if got := normalizeCodeStudioExecTimeout(0); got != 30*time.Second {
		t.Fatalf("default timeout = %s, want 30s", got)
	}
}

func TestDemuxDockerAttachStream(t *testing.T) {
	var raw bytes.Buffer
	writeAttachFrame(t, &raw, 1, []byte("hello\n"))
	writeAttachFrame(t, &raw, 2, []byte("err\n"))

	frames, err := demuxDockerAttachStream(&raw)
	if err != nil {
		t.Fatalf("demuxDockerAttachStream: %v", err)
	}
	if len(frames) != 2 {
		t.Fatalf("len(frames) = %d, want 2", len(frames))
	}
	if frames[0].Stream != 1 || string(frames[0].Payload) != "hello\n" {
		t.Fatalf("stdout frame = %#v", frames[0])
	}
	if frames[1].Stream != 2 || string(frames[1].Payload) != "err\n" {
		t.Fatalf("stderr frame = %#v", frames[1])
	}
}

func TestCodeStudioTerminalRejectsUnauthenticatedBeforeUpgrade(t *testing.T) {
	s := testCodeStudioServer(t)
	handler := codeStudioHandlers{server: s, docker: fakeCodeStudioDockerAPI{}}
	req := httptest.NewRequest(http.MethodGet, "/api/code-studio/terminal", nil)
	req.Header.Set("Connection", "Upgrade")
	req.Header.Set("Upgrade", "websocket")
	req.Header.Set("Sec-Websocket-Version", "13")
	req.Header.Set("Sec-Websocket-Key", "dGhlIHNhbXBsZSBub25jZQ==")
	rec := httptest.NewRecorder()

	handler.handleTerminal(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
	if rec.Header().Get("Upgrade") != "" {
		t.Fatalf("handler upgraded unauthenticated websocket")
	}
}

func writeAttachFrame(t *testing.T, buf *bytes.Buffer, stream byte, payload []byte) {
	t.Helper()
	header := make([]byte, 8)
	header[0] = stream
	binary.BigEndian.PutUint32(header[4:8], uint32(len(payload)))
	buf.Write(header)
	buf.Write(payload)
}

func testCodeStudioServer(t *testing.T) *Server {
	t.Helper()
	cfg := &config.Config{}
	cfg.Auth.Enabled = true
	cfg.Auth.SessionSecret = "test-secret"
	cfg.Auth.PasswordHash = "present"
	cfg.VirtualDesktop.Enabled = true
	cfg.VirtualDesktop.CodeStudio.Enabled = true
	cfg.VirtualDesktop.WorkspaceDir = filepath.Join(t.TempDir(), "desktop")
	cfg.SQLite.VirtualDesktopPath = filepath.Join(t.TempDir(), "desktop.db")
	return &Server{Cfg: cfg, Logger: slog.Default()}
}

type fakeCodeStudioDockerAPI struct{}

func (fakeCodeStudioDockerAPI) Exec(ctx context.Context, containerID string, cmd []string, timeout time.Duration) (codeStudioExecResult, error) {
	return codeStudioExecResult{}, nil
}

func (fakeCodeStudioDockerAPI) CreateTerminalExec(ctx context.Context, containerID string, cols, rows int) (string, error) {
	return "exec-id", nil
}

func (fakeCodeStudioDockerAPI) StartExec(ctx context.Context, execID string) ([]byte, error) {
	return nil, nil
}

func (fakeCodeStudioDockerAPI) ResizeExec(ctx context.Context, execID string, cols, rows int) error {
	return nil
}
