package server

import (
	"database/sql"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"aurago/internal/config"
	"aurago/internal/sqlconnections"
)

func newN8nToolTestServer(t *testing.T) *Server {
	t.Helper()

	cfg := &config.Config{}
	cfg.Directories.ToolsDir = t.TempDir()
	cfg.Directories.SkillsDir = t.TempDir()
	cfg.N8n.Enabled = true
	cfg.SQLConnections.Enabled = true

	return &Server{
		Cfg:    cfg,
		Logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
	}
}

func TestBuildFeatureFlagsRequiresSQLRuntimeForN8n(t *testing.T) {
	t.Parallel()

	s := newN8nToolTestServer(t)
	if buildFeatureFlags(s).SQLConnectionsEnabled {
		t.Fatal("expected SQL connections to stay disabled without db and pool")
	}

	s.SQLConnectionsDB = &sql.DB{}
	s.SQLConnectionPool = &sqlconnections.ConnectionPool{}
	if !buildFeatureFlags(s).SQLConnectionsEnabled {
		t.Fatal("expected SQL connections to be enabled when db and pool are available")
	}
}

func TestN8nToolAvailableRequiresSQLRuntime(t *testing.T) {
	t.Parallel()

	s := newN8nToolTestServer(t)
	if n8nToolAvailable(s, "sql_query", nil) {
		t.Fatal("sql_query should not be available without runtime SQL dependencies")
	}

	s.SQLConnectionsDB = &sql.DB{}
	s.SQLConnectionPool = &sqlconnections.ConnectionPool{}
	if !n8nToolAvailable(s, "sql_query", nil) {
		t.Fatal("sql_query should be available when runtime SQL dependencies exist")
	}
	if n8nToolAvailable(s, "nonexistent_tool", nil) {
		t.Fatal("unexpected availability for unknown tool")
	}
	if n8nToolAvailable(s, "sql_query", []string{"filesystem"}) {
		t.Fatal("sql_query should respect explicit allowed tool filtering")
	}
}

func TestGenerateSessionIDReturnsErrorWhenRandomFails(t *testing.T) {
	prev := n8nRandRead
	n8nRandRead = func([]byte) (int, error) {
		return 0, errors.New("entropy unavailable")
	}
	defer func() { n8nRandRead = prev }()

	if _, err := generateSessionID(); err == nil {
		t.Fatal("expected random failure to be returned")
	}
}

func TestMaskN8nTokenHandlesShortValues(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		token string
		want  string
	}{
		{name: "empty", token: "", want: ""},
		{name: "short", token: "short", want: "•••••"},
		{name: "exact eight", token: "12345678", want: "••••••••"},
		{name: "long", token: "n8n_1234567890", want: "n8n_••••••••7890"},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := maskN8nToken(tt.token); got != tt.want {
				t.Fatalf("maskN8nToken() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestN8nReadJSONRejectsTrailingDocuments(t *testing.T) {
	t.Parallel()

	req := httptest.NewRequest(http.MethodPost, "/api/n8n/chat", strings.NewReader(`{"message":"one"}{"message":"two"}`))
	rec := httptest.NewRecorder()
	var body n8nChatRequest
	if err := n8nReadJSON(rec, req, &body); err == nil {
		t.Fatal("expected trailing JSON document to be rejected")
	}
}

func TestN8nEffectiveAllowedTools(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		global  []string
		request []string
		want    []string
	}{
		{name: "request only", request: []string{"shell"}, want: []string{"shell"}},
		{name: "global only", global: []string{"shell"}, want: []string{"shell"}},
		{name: "intersection", global: []string{"shell", "http_request"}, request: []string{"http_request", "python"}, want: []string{"http_request"}},
		{name: "no overlap", global: []string{"shell"}, request: []string{"python"}, want: []string{}},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := n8nEffectiveAllowedTools(tt.global, tt.request)
			if len(got) != len(tt.want) {
				t.Fatalf("len = %d, want %d (%v)", len(got), len(tt.want), got)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Fatalf("got %v, want %v", got, tt.want)
				}
			}
		})
	}
}
