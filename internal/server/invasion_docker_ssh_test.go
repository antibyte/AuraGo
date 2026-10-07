package server

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"aurago/internal/invasion"
	"aurago/internal/security"
)

func TestCreateDockerSSHNestDefaultsToSSHPort(t *testing.T) {
	s := newInvasionTLSTestServer(t)
	rec := invasionTLSRequest(t, handleInvasionNests(s), http.MethodPost, "/api/invasion/nests", map[string]any{
		"name": "ssh-docker", "access_type": "docker", "host": "10.0.0.5", "username": "deploy",
		"secret": "pw", "deploy_method": "docker_ssh", "active": true,
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("create status = %d, body %s", rec.Code, rec.Body.String())
	}
	var created struct {
		ID   string `json:"id"`
		Port int    `json:"port"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}
	if created.Port != 22 {
		t.Fatalf("port = %d, want the SSH default 22", created.Port)
	}
	if nest, _ := invasion.GetNest(s.InvasionDB, created.ID); nest.DeployMethod != "docker_ssh" {
		t.Fatalf("deploy method = %q, want docker_ssh", nest.DeployMethod)
	}
}

func TestUpdateNestAcceptsDockerSSH(t *testing.T) {
	s := newInvasionTLSTestServer(t)
	id, err := invasion.CreateNest(s.InvasionDB, invasion.NestRecord{Name: "nest", Active: true, DeployMethod: "ssh", Host: "10.0.0.5", Port: 22})
	if err != nil {
		t.Fatal(err)
	}
	rec := invasionTLSRequest(t, handleInvasionNest(s), http.MethodPut, "/api/invasion/nests/"+id, map[string]any{
		"name": "nest", "access_type": "ssh", "host": "10.0.0.5", "port": 22, "active": true,
		"deploy_method": "docker_ssh", "target_arch": "linux/amd64", "route": "direct",
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("update status = %d, body %s", rec.Code, rec.Body.String())
	}
}

func TestValidateNestConnectionRequiresSecretForDockerSSH(t *testing.T) {
	err := validateNestConnection(invasion.NestRecord{DeployMethod: "docker_ssh", Host: "10.0.0.5"}, &Server{})
	if err == nil || !strings.Contains(err.Error(), "no SSH secret") {
		t.Fatalf("validate = %v, want the missing SSH secret error", err)
	}
}

func TestValidateNestConnectionSecretRequirementPerMethod(t *testing.T) {
	// The ssh method keeps requiring a secret.
	if err := validateNestConnection(invasion.NestRecord{DeployMethod: "ssh", Host: "10.0.0.5"}, &Server{}); err == nil || !strings.Contains(err.Error(), "no SSH secret") {
		t.Fatalf("ssh validate = %v, want the missing SSH secret error", err)
	}
	// docker_remote still needs no secret: it goes straight to the Engine API
	// (a closed local port here, so the check fails fast with a connection error).
	err := validateNestConnection(invasion.NestRecord{ID: "12345678-0000-0000-0000-000000000000", DeployMethod: "docker_remote", Host: "127.0.0.1", Port: 1}, &Server{})
	if err == nil || strings.Contains(err.Error(), "no SSH secret") {
		t.Fatalf("docker_remote validate = %v, want an Engine connection error, not the SSH secret check", err)
	}
}

func TestCreateDockerSSHNestRejectsDockerTLS(t *testing.T) {
	ca, _, _ := testDockerTLSPEMs(t)
	for name, fields := range map[string]map[string]any{
		"tls mode": {"docker_tls": "tls"},
		"CA only":  {"docker_tls_ca": ca},
	} {
		s := newInvasionTLSTestServer(t)
		body := map[string]any{"name": "nest", "access_type": "ssh", "host": "10.0.0.5", "secret": "pw", "deploy_method": "docker_ssh", "active": true}
		for k, v := range fields {
			body[k] = v
		}
		rec := invasionTLSRequest(t, handleInvasionNests(s), http.MethodPost, "/api/invasion/nests", body)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("%s: status = %d, want 400 (body %s)", name, rec.Code, rec.Body.String())
		}
	}
}

func TestInvasionTransportSecretGivesDockerSSHItsSSHSecret(t *testing.T) {
	s := newInvasionTLSTestServer(t)
	if err := s.Vault.WriteSecret("nest_ssh_docker", "ssh-key"); err != nil {
		t.Fatal(err)
	}
	// Even a leftover TLS mode must not swap the SSH credential for TLS material.
	for _, mode := range []string{invasion.DockerTLSOff, invasion.DockerTLSMutual} {
		nest := invasion.NestRecord{ID: "12345678-0000-0000-0000-000000000000", DeployMethod: "docker_ssh", DockerTLS: mode, VaultSecretID: "nest_ssh_docker"}
		got, err := s.invasionTransportSecret(nest)
		if err != nil || string(got) != "ssh-key" {
			t.Fatalf("docker_tls %q: credential = %q, %v; want the nest's SSH secret", mode, got, err)
		}
	}
}

func TestUpdateTLSDockerRemoteNestToDockerSSHEndsTLS(t *testing.T) {
	for name, extra := range map[string]map[string]any{
		"current UI":   {"docker_tls": ""},
		"older client": {},
	} {
		s := newInvasionTLSTestServer(t)
		id, _, _, _ := createMutualTLSNest(t, s)
		body := map[string]any{
			"name": "tls-nest", "access_type": "ssh", "host": "10.0.0.5", "port": 22, "username": "deploy", "secret": "ssh-key",
			"active": true, "deploy_method": "docker_ssh", "target_arch": "linux/amd64", "route": "direct",
		}
		for k, v := range extra {
			body[k] = v
		}
		rec := invasionTLSRequest(t, handleInvasionNest(s), http.MethodPut, "/api/invasion/nests/"+id, body)
		if rec.Code != http.StatusOK {
			t.Fatalf("%s: update status = %d, body %s", name, rec.Code, rec.Body.String())
		}
		nest, err := invasion.GetNest(s.InvasionDB, id)
		if err != nil || nest.DeployMethod != "docker_ssh" || nest.DockerTLS != invasion.DockerTLSOff {
			t.Fatalf("%s: nest = %q/%q, %v; want docker_ssh without Docker TLS", name, nest.DeployMethod, nest.DockerTLS, err)
		}
		if _, err := s.Vault.ReadSecret(invasion.DockerTLSVaultKey(id)); err != security.ErrSecretNotFound {
			t.Fatalf("%s: TLS vault read = %v, want the material removed", name, err)
		}
		if got, err := s.invasionTransportSecret(nest); err != nil || string(got) != "ssh-key" {
			t.Fatalf("%s: credential = %q, %v; want the SSH secret", name, got, err)
		}
	}
}

func TestInvasionSecurityHintsPlaintextAdviceNamesBothSSHMethodsAndTheListener(t *testing.T) {
	db := setupInvasionTestDB(t)
	if _, err := invasion.CreateNest(db, invasion.NestRecord{Name: "plain", Active: true, DeployMethod: "docker_remote", Host: "10.0.0.5", Port: 2375}); err != nil {
		t.Fatal(err)
	}
	hints := invasionSecurityHints(db, nil)
	if len(hints) != 1 {
		t.Fatalf("hints = %#v, want the plaintext hint", hints)
	}
	for _, want := range []string{"Docker TLS", "Docker (via SSH)", "or SSH", "-H tcp://", "remove that listener on the target host"} {
		if !strings.Contains(hints[0].Description, want) {
			t.Fatalf("hint description %q lacks %q", hints[0].Description, want)
		}
	}
}

func TestInvasionSecurityHintsSkipDockerSSHNests(t *testing.T) {
	db := setupInvasionTestDB(t)
	if _, err := invasion.CreateNest(db, invasion.NestRecord{Name: "ssh-docker", Active: true, DeployMethod: "docker_ssh", Host: "10.0.0.5", Port: 22}); err != nil {
		t.Fatal(err)
	}
	if hints := invasionSecurityHints(db, nil); len(hints) != 0 {
		t.Fatalf("hints = %#v, want none for a docker_ssh nest", hints)
	}
}
