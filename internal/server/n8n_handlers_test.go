package server

import (
	"database/sql"
	"errors"
	"io"
	"log/slog"
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
