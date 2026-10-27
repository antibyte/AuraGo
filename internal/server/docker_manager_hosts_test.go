package server

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"aurago/internal/dockerhost"
	"aurago/internal/security"
)

func TestDockerManagerHostCRUD(t *testing.T) {
	f := newDMFixture(t)

	code, body := f.do(http.MethodGet, "/hosts", nil)
	hosts := dmArray(t, body, "hosts")
	if code != http.StatusOK || len(hosts) != 1 || hosts[0]["id"] != "local" || hosts[0]["builtin"] != true || hosts[0]["kind"] != "local" {
		t.Fatalf("initial list = %d %s, want only the builtin local host", code, f.lastRaw)
	}

	code, body = f.do(http.MethodPost, "/hosts", map[string]any{"name": "NAS", "kind": "tcp", "address": "127.0.0.1", "port": f.remote.port()})
	if code != http.StatusCreated {
		t.Fatalf("create = %d %s, want 201", code, f.lastRaw)
	}
	host := dmObject(t, body, "host")
	id := dmString(t, host, "id")
	if !strings.HasPrefix(id, "h_") || host["builtin"] != false || host["plaintext"] != true || host["secret_set"] != false ||
		host["stacks_dir"] != dockerhost.DefaultStacksDir || host["color"] == "" || host["color"] == "blue" {
		t.Fatalf("created host = %s, want defaults filled, plaintext, no secret and a colour other than the local blue", f.lastRaw)
	}

	code, body = f.do(http.MethodPost, "/hosts", map[string]any{"name": " ", "kind": "tcp", "address": "127.0.0.1"})
	dmExpectError(t, f, code, body, http.StatusBadRequest, "invalid_host")
	if body["field"] != "name" {
		t.Fatalf("validation answer = %s, want field name", f.lastRaw)
	}
	code, body = f.do(http.MethodPost, "/hosts", map[string]any{"name": "Again local", "kind": "local"})
	dmExpectError(t, f, code, body, http.StatusBadRequest, "invalid_host")
	// Names are unique ignoring case, and "Local" belongs to the built-in host.
	for _, name := range []string{"nas", "LOCAL"} {
		code, body = f.do(http.MethodPost, "/hosts", map[string]any{"name": name, "kind": "tcp", "address": "127.0.0.1"})
		dmExpectError(t, f, code, body, http.StatusBadRequest, "invalid_host")
		if body["field"] != "name" {
			t.Fatalf("name %q answer = %s, want field name", name, f.lastRaw)
		}
	}

	code, body = f.do(http.MethodPut, "/hosts/"+id, map[string]any{"name": "NAS 2", "kind": "tcp", "address": "127.0.0.1", "port": f.remote.port()})
	host = dmObject(t, body, "host")
	if code != http.StatusOK || host["name"] != "NAS 2" || host["stacks_dir"] != dockerhost.DefaultStacksDir {
		t.Fatalf("update = %d %s, want the new name and the kept stacks_dir", code, f.lastRaw)
	}

	code, body = f.do(http.MethodPut, "/hosts/local", map[string]any{"name": "X", "kind": "tcp", "address": "127.0.0.1"})
	dmExpectError(t, f, code, body, http.StatusConflict, "local_host_fixed")
	code, body = f.do(http.MethodDelete, "/hosts/local", nil)
	dmExpectError(t, f, code, body, http.StatusConflict, "local_host_fixed")
	code, body = f.do(http.MethodDelete, "/hosts/h_000000000000", nil)
	dmExpectError(t, f, code, body, http.StatusNotFound, "host_not_found")

	code, body = f.do(http.MethodDelete, "/hosts/"+id, nil)
	if code != http.StatusOK || body["status"] != "ok" {
		t.Fatalf("delete = %d %s, want 200 ok", code, f.lastRaw)
	}
	_, body = f.do(http.MethodGet, "/hosts", nil)
	if got := len(dmArray(t, body, "hosts")); got != 1 {
		t.Fatalf("list after delete has %d hosts, want only local", got)
	}
}

func TestDockerManagerHostConnectionTest(t *testing.T) {
	f := newDMFixture(t)
	id := f.addRemote("NAS")

	code, body := f.do(http.MethodPost, "/hosts/"+id+"/test", nil)
	status := dmObject(t, dmObject(t, body, "host"), "status")
	if code != http.StatusOK || status["state"] != "ok" || status["engine_version"] != "27.0.1" {
		t.Fatalf("test = %d %s, want state ok and the engine version", code, f.lastRaw)
	}

	f.remote.srv.Close()
	code, body = f.do(http.MethodPost, "/hosts/"+id+"/test", nil)
	status = dmObject(t, dmObject(t, body, "host"), "status")
	if code != http.StatusOK || status["state"] != "error" || status["code"] == "" {
		t.Fatalf("unreachable test = %d %s, want 200 with state error and a reason code", code, f.lastRaw)
	}
	code, body = f.do(http.MethodPost, "/hosts/h_000000000000/test", nil)
	dmExpectError(t, f, code, body, http.StatusNotFound, "host_not_found")
}

func TestDockerManagerDraftTestDoesNotSave(t *testing.T) {
	f := newDMFixture(t)
	code, body := f.do(http.MethodPost, "/hosts/test", map[string]any{"name": "Draft", "kind": "tcp", "address": "127.0.0.1", "port": f.remote.port()})
	if status := dmObject(t, body, "host_status"); code != http.StatusOK || status["state"] != "ok" {
		t.Fatalf("draft test = %d %s, want host_status ok", code, f.lastRaw)
	}
	code, body = f.do(http.MethodPost, "/hosts/test", map[string]any{"name": "Draft", "kind": "tcp", "address": "http://127.0.0.1"})
	dmExpectError(t, f, code, body, http.StatusBadRequest, "invalid_host")
	if body["field"] != "address" {
		t.Fatalf("draft validation = %s, want field address", f.lastRaw)
	}
	_, body = f.do(http.MethodGet, "/hosts", nil)
	if got := len(dmArray(t, body, "hosts")); got != 1 {
		t.Fatalf("a draft test stored a host: %d hosts", got)
	}
}

func TestDockerManagerSecretsAreWriteOnly(t *testing.T) {
	f := newDMFixture(t)
	const secret = "TOPSECRET-KEY-MATERIAL"

	code, body := f.do(http.MethodPost, "/hosts", map[string]any{"name": "edge", "kind": "ssh", "address": "192.0.2.10", "ssh_user": "root", "ssh_secret": secret})
	if code != http.StatusCreated || dmObject(t, body, "host")["secret_set"] != true {
		t.Fatalf("create ssh host = %d %s, want 201 with secret_set", code, f.lastRaw)
	}
	if strings.Contains(f.lastRaw, secret) {
		t.Fatal("the create answer echoes the SSH secret")
	}
	id := dmString(t, body["host"], "id")
	f.do(http.MethodGet, "/hosts", nil)
	if strings.Contains(f.lastRaw, secret) {
		t.Fatal("the host list echoes the SSH secret")
	}
	if got, err := f.s.Vault.ReadSecret(dockerhost.SSHSecretKey(id)); err != nil || got != secret {
		t.Fatalf("vault secret = %q, %v; want the submitted secret", got, err)
	}

	// An edit without a secret (absent or blank) keeps it; a new one replaces it.
	edit := map[string]any{"name": "edge 2", "kind": "ssh", "address": "192.0.2.10", "ssh_user": "root"}
	for _, blank := range []bool{false, true} {
		if blank {
			edit["ssh_secret"] = ""
		}
		code, body = f.do(http.MethodPut, "/hosts/"+id, edit)
		if got, _ := f.s.Vault.ReadSecret(dockerhost.SSHSecretKey(id)); code != http.StatusOK || dmObject(t, body, "host")["secret_set"] != true || got != secret {
			t.Fatalf("update without secret (blank=%v) = %d %s, vault %q; want the secret kept", blank, code, f.lastRaw, got)
		}
	}
	edit["ssh_secret"] = "ROTATED-KEY"
	if code, _ = f.do(http.MethodPut, "/hosts/"+id, edit); code != http.StatusOK {
		t.Fatalf("update with a new secret = %d %s", code, f.lastRaw)
	}
	if got, _ := f.s.Vault.ReadSecret(dockerhost.SSHSecretKey(id)); got != "ROTATED-KEY" {
		t.Fatalf("vault secret after rotation = %q", got)
	}

	// An SSH host needs its secret; a kind that does not use one drops it.
	delete(edit, "ssh_secret")
	edit["clear_secret"] = true
	code, body = f.do(http.MethodPut, "/hosts/"+id, edit)
	dmExpectError(t, f, code, body, http.StatusBadRequest, "invalid_host")
	if body["field"] != "ssh_secret" {
		t.Fatalf("clearing the secret of an ssh host = %s, want field ssh_secret", f.lastRaw)
	}
	code, body = f.do(http.MethodPut, "/hosts/"+id, map[string]any{"name": "edge 2", "kind": "tcp", "address": "192.0.2.10", "clear_secret": true})
	if code != http.StatusOK || dmObject(t, body, "host")["secret_set"] != false {
		t.Fatalf("switch to tcp = %d %s, want secret_set false", code, f.lastRaw)
	}
	if _, err := f.s.Vault.ReadSecret(dockerhost.SSHSecretKey(id)); !errors.Is(err, security.ErrSecretNotFound) {
		t.Fatalf("vault secret after the switch: err = %v, want ErrSecretNotFound", err)
	}

	// Deleting a host removes its vault entries.
	code, body = f.do(http.MethodPost, "/hosts", map[string]any{"name": "edge3", "kind": "ssh", "address": "192.0.2.11", "ssh_user": "root", "ssh_secret": secret})
	if code != http.StatusCreated {
		t.Fatalf("create second ssh host = %d %s", code, f.lastRaw)
	}
	second := dmString(t, body["host"], "id")
	f.do(http.MethodDelete, "/hosts/"+second, nil)
	if _, err := f.s.Vault.ReadSecret(dockerhost.SSHSecretKey(second)); !errors.Is(err, security.ErrSecretNotFound) {
		t.Fatalf("vault secret after delete: err = %v, want ErrSecretNotFound", err)
	}
}

func TestDockerManagerHostListStartsBackgroundTests(t *testing.T) {
	f := newDMFixture(t)
	id := f.addRemote("NAS")
	f.do(http.MethodGet, "/hosts", nil)
	f.s.dockerStatus.wg.Wait()
	_, body := f.do(http.MethodGet, "/hosts", nil)
	host := dmFind(t, dmArray(t, body, "hosts"), "id", id)
	if dmObject(t, host, "status")["state"] != "ok" {
		t.Fatalf("host after a list round trip = %s, want state ok from the background test", f.lastRaw)
	}
}

// The host list tells the UI whether docker.readonly is on. It is read per request, so the actions are hidden before
// the first refusal and come back when the admin lifts the switch.
func TestDockerManagerHostListReportsDockerReadonly(t *testing.T) {
	f := newDMFixture(t)
	id := f.addRemote("NAS")
	for _, readOnly := range []bool{false, true, false} {
		f.setDocker(true, readOnly)
		code, body := f.do(http.MethodGet, "/hosts", nil)
		if code != http.StatusOK || body["docker_readonly"] != readOnly {
			t.Fatalf("docker.readonly = %v: host list = %d %s, want docker_readonly %v next to hosts", readOnly, code, f.lastRaw, readOnly)
		}
		if len(dmArray(t, body, "hosts")) != 2 {
			t.Fatalf("docker.readonly = %v: host list lost its hosts: %s", readOnly, f.lastRaw)
		}
		code, body = f.do(http.MethodPost, "/hosts/"+id+"/containers/web1/start", nil)
		if readOnly {
			dmExpectError(t, f, code, body, http.StatusForbidden, "read_only")
		} else if code == http.StatusForbidden {
			t.Fatalf("docker.readonly = false: start was refused: %s", f.lastRaw)
		}
	}
}

func TestDockerManagerRejectsMalformedBodies(t *testing.T) {
	f := newDMFixture(t)
	for _, path := range []string{"/hosts", "/hosts/test"} {
		req := httptest.NewRequest(http.MethodPost, "/api/docker-manager"+path, strings.NewReader("{not json"))
		rec := httptest.NewRecorder()
		handleDockerManager(f.s)(rec, req)
		if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), `"bad_request"`) {
			t.Fatalf("POST %s with a malformed body = %d %s, want 400 bad_request", path, rec.Code, rec.Body.String())
		}
	}
}
