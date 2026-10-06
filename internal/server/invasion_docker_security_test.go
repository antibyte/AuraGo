package server

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"aurago/internal/config"
	"aurago/internal/invasion"
)

func TestInvasionSecurityHintsFlagActivePlaintextDockerRemoteNests(t *testing.T) {
	db := setupInvasionTestDB(t)
	for _, nest := range []invasion.NestRecord{
		{Name: "alpha", Active: true, DeployMethod: "docker_remote", Host: "10.0.0.5"},
		{Name: "beta", Active: false, DeployMethod: "docker_remote", Host: "10.0.0.6"},
		{Name: "gamma", Active: true, DeployMethod: "ssh", Host: "10.0.0.7"},
		{Name: "delta", Active: true, DeployMethod: "docker_local"},
	} {
		if _, err := invasion.CreateNest(db, nest); err != nil {
			t.Fatal(err)
		}
	}
	hints := invasionSecurityHints(db, nil)
	if len(hints) != 1 || hints[0].ID != "invasion_docker_remote_plaintext" {
		t.Fatalf("hints = %#v, want one invasion_docker_remote_plaintext hint", hints)
	}
	if hints[0].Severity != SevWarning || hints[0].AutoFixable {
		t.Fatalf("hint = %#v, want a non-auto-fixable warning", hints[0])
	}
	if !strings.Contains(hints[0].Description, `"alpha"`) || strings.Contains(hints[0].Description, "beta") {
		t.Fatalf("description = %q, want only the active docker_remote nest", hints[0].Description)
	}
}

func TestInvasionSecurityHintsEmptyWithoutPlaintextNests(t *testing.T) {
	if hints := invasionSecurityHints(nil, nil); len(hints) != 0 {
		t.Fatalf("hints without an invasion DB = %#v, want none", hints)
	}
	db := setupInvasionTestDB(t)
	if _, err := invasion.CreateNest(db, invasion.NestRecord{Name: "ssh-only", Active: true, DeployMethod: "ssh"}); err != nil {
		t.Fatal(err)
	}
	if hints := invasionSecurityHints(db, nil); len(hints) != 0 {
		t.Fatalf("hints = %#v, want none for SSH nests", hints)
	}
}

func TestInvasionSecurityHintsLogsFailedNestCheck(t *testing.T) {
	db := setupInvasionTestDB(t)
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, nil))
	if hints := invasionSecurityHints(db, logger); hints != nil {
		t.Fatalf("hints with a failing nest query = %#v, want nil", hints)
	}
	log := buf.String()
	if !strings.Contains(log, "level=WARN") || !strings.Contains(log, "Invasion nest check for the security hints failed") || !strings.Contains(log, "database is closed") {
		t.Fatalf("log = %q, want a warning with the failed nest query error", log)
	}
}

func TestSecurityHintsEndpointIncludesInvasionHints(t *testing.T) {
	db := setupInvasionTestDB(t)
	if _, err := invasion.CreateNest(db, invasion.NestRecord{Name: "alpha", Active: true, DeployMethod: "docker_remote", Host: "10.0.0.5"}); err != nil {
		t.Fatal(err)
	}
	s := &Server{Cfg: &config.Config{}, InvasionDB: db, Logger: slog.New(slog.NewTextHandler(io.Discard, nil))}
	rec := httptest.NewRecorder()
	handleSecurityHints(s)(rec, httptest.NewRequest(http.MethodGet, "/api/security/hints", nil))
	var body struct {
		Hints []struct {
			ID string `json:"id"`
		} `json:"hints"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode hints: %v", err)
	}
	for _, hint := range body.Hints {
		if hint.ID == "invasion_docker_remote_plaintext" {
			return
		}
	}
	t.Fatalf("hints = %#v, want invasion_docker_remote_plaintext", body.Hints)
}

func TestWarnPlaintextDockerRemoteLogsOnlyDockerRemote(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, nil))
	warnPlaintextDockerRemote(logger, invasion.NestRecord{ID: "n1", Name: "ssh-nest", DeployMethod: "ssh"}, "hatch")
	if buf.Len() != 0 {
		t.Fatalf("SSH nest logged %q, want nothing", buf.String())
	}
	warnPlaintextDockerRemote(logger, invasion.NestRecord{ID: "n2", Name: "remote", DeployMethod: "docker_remote", Host: "10.0.0.5"}, "hatch")
	if !strings.Contains(buf.String(), "unencrypted HTTP") || !strings.Contains(buf.String(), "operation=hatch") {
		t.Fatalf("log = %q, want the plaintext warning with the operation", buf.String())
	}
}
