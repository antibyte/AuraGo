package server

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"aurago/internal/invasion"
	"aurago/internal/security"
)

func failNestUpdates(t *testing.T, s *Server) {
	t.Helper()
	if _, err := s.InvasionDB.Exec(`CREATE TRIGGER nests_update_fails BEFORE UPDATE ON nests BEGIN SELECT RAISE(ABORT, 'simulated update failure'); END`); err != nil {
		t.Fatal(err)
	}
}

func tlsPut(extra map[string]any) map[string]any {
	body := map[string]any{"name": "tls-nest", "access_type": "docker", "host": "10.0.0.5", "port": 2376, "active": true,
		"deploy_method": "docker_remote", "target_arch": "linux/amd64", "route": "direct"}
	for k, v := range extra {
		body[k] = v
	}
	return body
}

func TestUpdateNestRestoresPreviousDockerTLSMaterialWhenDBUpdateFails(t *testing.T) {
	s := newInvasionTLSTestServer(t)
	id, ca, cert, key := createMutualTLSNest(t, s)
	otherCA, _, _ := testDockerTLSPEMs(t)
	failNestUpdates(t, s)
	rec := invasionTLSRequest(t, handleInvasionNest(s), http.MethodPut, "/api/invasion/nests/"+id, tlsPut(map[string]any{"docker_tls": "tls", "docker_tls_ca": otherCA}))
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", rec.Code)
	}
	if n, _ := invasion.GetNest(s.InvasionDB, id); n.DockerTLS != "mtls" {
		t.Fatalf("mode = %q, want mtls", n.DockerTLS)
	}
	want := invasion.DockerTLSMaterial{CA: strings.TrimSpace(ca), Cert: strings.TrimSpace(cert), Key: strings.TrimSpace(key)}
	if got, err := s.loadNestDockerTLS(id); err != nil || got != want {
		t.Fatalf("material = %+v, %v; want the mtls material back", got, err)
	}
}

func TestUpdateNestDropsNewDockerTLSMaterialWhenDBUpdateFailsOnAPlainNest(t *testing.T) {
	s := newInvasionTLSTestServer(t)
	created := invasionTLSRequest(t, handleInvasionNests(s), http.MethodPost, "/api/invasion/nests", map[string]any{
		"name": "plain", "access_type": "docker", "host": "10.0.0.5", "deploy_method": "docker_remote", "active": true})
	var body struct {
		ID string `json:"id"`
	}
	if created.Code != http.StatusOK || json.Unmarshal(created.Body.Bytes(), &body) != nil {
		t.Fatalf("create: %d %s", created.Code, created.Body.String())
	}
	ca, _, _ := testDockerTLSPEMs(t)
	failNestUpdates(t, s)
	rec := invasionTLSRequest(t, handleInvasionNest(s), http.MethodPut, "/api/invasion/nests/"+body.ID, tlsPut(map[string]any{"docker_tls": "tls", "docker_tls_ca": ca}))
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", rec.Code)
	}
	if n, _ := invasion.GetNest(s.InvasionDB, body.ID); n.DockerTLS != "" {
		t.Fatalf("mode = %q, want plain", n.DockerTLS)
	}
	if _, err := s.Vault.ReadSecret(invasion.DockerTLSVaultKey(body.ID)); err != security.ErrSecretNotFound {
		t.Fatalf("vault read = %v, want the new entry removed again", err)
	}
}

func TestUpdateNestRestoresUnreadableDockerTLSMaterialWhenDBUpdateFails(t *testing.T) {
	s := newInvasionTLSTestServer(t)
	ca, _, _ := testDockerTLSPEMs(t)
	id := createServerTLSNest(t, s, ca)
	const unreadable = "{not json"
	if err := s.Vault.WriteSecret(invasion.DockerTLSVaultKey(id), unreadable); err != nil {
		t.Fatal(err)
	}
	newCA, _, _ := testDockerTLSPEMs(t)
	failNestUpdates(t, s)
	rec := invasionTLSRequest(t, handleInvasionNest(s), http.MethodPut, "/api/invasion/nests/"+id, tlsPut(map[string]any{"docker_tls": "tls", "docker_tls_ca": newCA}))
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", rec.Code)
	}
	if raw, _ := s.Vault.ReadSecret(invasion.DockerTLSVaultKey(id)); raw != unreadable {
		t.Fatal("a failed update left the new CA in place of the stored entry")
	}
}

// Requested by the K17 re-review; passes before the change and pins it.
func TestUpdateNestFailedEarlyLeftoverDeleteKeepsThePlainNest(t *testing.T) {
	vaultPath := filepath.Join(t.TempDir(), "vault.bin")
	vault, err := security.NewVault(strings.Repeat("b", 64), vaultPath)
	if err != nil {
		t.Fatal(err)
	}
	s := &Server{InvasionDB: setupInvasionTestDB(t), Vault: vault, Logger: slog.New(slog.NewTextHandler(io.Discard, nil))}
	created := invasionTLSRequest(t, handleInvasionNests(s), http.MethodPost, "/api/invasion/nests", map[string]any{
		"name": "plain", "access_type": "docker", "host": "10.0.0.5", "deploy_method": "docker_remote", "active": true})
	var body struct {
		ID string `json:"id"`
	}
	if created.Code != http.StatusOK || json.Unmarshal(created.Body.Bytes(), &body) != nil {
		t.Fatalf("create: %d %s", created.Code, created.Body.String())
	}
	if err := os.WriteFile(vaultPath, []byte("not a vault"), 0o600); err != nil {
		t.Fatal(err)
	}
	rec := invasionTLSRequest(t, handleInvasionNest(s), http.MethodPut, "/api/invasion/nests/"+body.ID, tlsPut(map[string]any{"name": "plain", "docker_tls": "tls"}))
	if rec.Code != http.StatusInternalServerError || !strings.Contains(rec.Body.String(), "Failed to remove stale nest TLS material") {
		t.Fatalf("status = %d, body %s; want 500 from the failed leftover delete", rec.Code, rec.Body.String())
	}
	if n, _ := invasion.GetNest(s.InvasionDB, body.ID); n.DockerTLS != "" || n.Port != 2375 {
		t.Fatalf("nest = mode %q port %d, want it unchanged (plain, 2375)", n.DockerTLS, n.Port)
	}
}
