package server

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"aurago/internal/credentials"
	"aurago/internal/desktop"
	"aurago/internal/inventory"
	"aurago/internal/remote"
	"aurago/internal/security"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"
)

func codeStudioRemoteTestServer(t *testing.T, f *codeStudioSSHFixture) (*Server, string) {
	t.Helper()
	prev := remote.InsecureHostKey
	remote.InsecureHostKey = false
	t.Cleanup(func() { remote.InsecureHostKey = prev })
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	s := testCodeStudioServerWithFakeCodeContainer(t, 1)
	db, err := inventory.InitDB(filepath.Join(t.TempDir(), "inventory.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := credentials.EnsureSchema(db); err != nil {
		t.Fatal(err)
	}
	vault, err := security.NewVault("0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef", filepath.Join(t.TempDir(), "vault.bin"))
	if err != nil {
		t.Fatal(err)
	}
	_ = vault.WriteSecret("cs/dev", f.password)
	if err := inventory.AddDevice(db, inventory.DeviceRecord{ID: "dev1", Name: "Fixture", Type: "server", Protocol: inventory.ProtocolSSH,
		IPAddress: f.host, Port: f.port, Username: f.user, VaultSecretID: "cs/dev"}); err != nil {
		t.Fatal(err)
	}
	s.InventoryDB, s.Vault = db, vault
	return s, home
}

func postJSON(t *testing.T, handler http.HandlerFunc, path, body string) (*httptest.ResponseRecorder, map[string]any) {
	t.Helper()
	w := httptest.NewRecorder()
	handler(w, httptest.NewRequest(http.MethodPost, path, strings.NewReader(body)))
	var m map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &m)
	return w, m
}

func TestCodeStudioWorkspaceHostKeyContractAndLifecycle(t *testing.T) {
	f := startCodeStudioSSHFixture(t)
	if err := f.adminSFTP(t).MkdirAll("/home/dev/proj"); err != nil {
		t.Fatal(err)
	}
	s, home := codeStudioRemoteTestServer(t, f)
	newH := func() codeStudioHandlers {
		r := newCodeStudioRemote(codeStudioDefaultSSHDialer(s), slog.Default())
		t.Cleanup(r.pool.closeAll)
		return codeStudioHandlers{server: s, docker: &recordingCodeStudioDockerAPI{}, remote: r}
	}
	h := newH()
	fp := ssh.FingerprintSHA256(f.hostKey)
	w, m := postJSON(t, h.handleWorkspaces, "/api/code-studio/workspaces", `{"device_id":"dev1","root":"/home/dev/proj"}`)
	if w.Code != 409 || m["code"] != "unknown_host_key" || m["fingerprint"] != fp || m["key_type"] != "ssh-ed25519" {
		t.Fatal(w.Code, w.Body.String())
	}
	w, m = postJSON(t, h.handleWorkspaces, "/api/code-studio/workspaces", `{"device_id":"dev1","root":"/home/dev/proj","trust_host_key":"SHA256:wrong"}`)
	if w.Code != 409 || m["code"] != "unknown_host_key" {
		t.Fatal(w.Body.String())
	}
	w, m = postJSON(t, h.handleWorkspaces, "/api/code-studio/workspaces", `{"device_id":"dev1","root":"/home/dev/proj","label":"Proj","trust_host_key":"`+fp+`"}`)
	if w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	ws := m["workspace"].(map[string]any)
	id := ws["id"].(string)
	if ws["kind"] != "ssh" || ws["host"] != f.host || ws["user"] != f.user || ws["capabilities"].(map[string]any)["git"] != true {
		t.Fatalf("%v", ws)
	}
	kh, _ := os.ReadFile(filepath.Join(home, ".ssh", "known_hosts"))
	if !strings.Contains(string(kh), f.hostKey.Type()) {
		t.Fatal("host key not stored")
	}
	_, m = postJSON(t, h.handleWorkspaces, "/api/code-studio/workspaces", `{"device_id":"dev1","root":"/home/dev/proj/"}`)
	if m["workspace"].(map[string]any)["id"] != id {
		t.Fatal("upsert created a second entry")
	}
	w, m = postJSON(t, h.handleWorkspaces, "/api/code-studio/workspaces", `{"device_id":"dev1","root":"/nope"}`)
	if w.Code != 400 || m["code"] != "not_a_directory" {
		t.Fatal(w.Body.String())
	}
	w, _ = postJSON(t, h.handleWorkspaceItem, "/api/code-studio/workspaces/"+id+"/connect", ``)
	if w.Code != 200 {
		t.Fatal(w.Body.String())
	}
	lw := httptest.NewRecorder()
	h.handleWorkspaces(lw, httptest.NewRequest(http.MethodGet, "/api/code-studio/workspaces", nil))
	if lw.Code != 200 || strings.Count(lw.Body.String(), `"kind"`) != 2 || strings.Contains(lw.Body.String(), "cs/dev") || strings.Contains(lw.Body.String(), f.password) {
		t.Fatal(lw.Body.String())
	}
	dw := httptest.NewRecorder()
	h.handleWorkspaceItem(dw, httptest.NewRequest(http.MethodDelete, "/api/code-studio/workspaces/"+id, nil))
	if dw.Code != 200 {
		t.Fatal(dw.Body.String())
	}
	dw = httptest.NewRecorder()
	h.handleWorkspaceItem(dw, httptest.NewRequest(http.MethodDelete, "/api/code-studio/workspaces/"+id, nil))
	if dw.Code != 404 {
		t.Fatal(dw.Code)
	}
	// changed host key: no override, not even with trust_host_key
	_, other, _ := ed25519.GenerateKey(rand.Reader)
	otherSigner, _ := ssh.NewSignerFromKey(other)
	_ = os.WriteFile(filepath.Join(home, ".ssh", "known_hosts"), []byte(knownhosts.Line([]string{f.addr}, otherSigner.PublicKey())+"\n"), 0o600)
	w, m = postJSON(t, newH().handleWorkspaces, "/api/code-studio/workspaces", `{"device_id":"dev1","root":"/home/dev/proj","trust_host_key":"`+fp+`"}`)
	if w.Code != 409 || m["code"] != "host_key_mismatch" {
		t.Fatal(w.Body.String())
	}
	w, _ = postJSON(t, newH().handleRemoteTrust, "/api/code-studio/remote/trust", `{"device_id":"dev1","fingerprint":"`+fp+`"}`)
	if w.Code != 409 {
		t.Fatal("trust endpoint overrode a mismatch")
	}
}

func TestCodeStudioRemoteDevicesCreateAndList(t *testing.T) {
	f := startCodeStudioSSHFixture(t)
	s, _ := codeStudioRemoteTestServer(t, f)
	h := codeStudioHandlers{server: s, docker: &recordingCodeStudioDockerAPI{}}
	w, m := postJSON(t, h.handleRemoteDevices, "/api/code-studio/remote/devices", `{"name":"Lab box","host":"10.0.0.5","port":2222,"username":"ana","auth":"password","secret":"s3cret"}`)
	if w.Code != 201 {
		t.Fatal(w.Code, w.Body.String())
	}
	dev := m["device"].(map[string]any)
	id := dev["id"].(string)
	if dev["host"] != "10.0.0.5" || dev["port"].(float64) != 2222 || dev["username"] != "ana" || dev["has_secret"] != true || strings.Contains(w.Body.String(), "s3cret") {
		t.Fatal(w.Body.String())
	}
	access, err := desktop.ResolveSSHDevice(s.InventoryDB, s.Vault, id)
	if err != nil || string(access.Secret) != "s3cret" || access.Username != "ana" || access.Port != 2222 {
		t.Fatalf("%+v %v", access, err)
	}
	w, m = postJSON(t, h.handleRemoteDevices, "/api/code-studio/remote/devices", `{"name":"K","host":"h","username":"u","auth":"key","secret":"not a key"}`)
	if w.Code != 400 || m["code"] != "invalid_private_key" {
		t.Fatal(w.Body.String())
	}
	lw := httptest.NewRecorder()
	h.handleRemoteDevices(lw, httptest.NewRequest(http.MethodGet, "/api/code-studio/remote/devices", nil))
	if lw.Code != 200 || !strings.Contains(lw.Body.String(), `"Lab box"`) || !strings.Contains(lw.Body.String(), `"Fixture"`) || strings.Contains(lw.Body.String(), "vault") {
		t.Fatal(lw.Body.String())
	}
}
