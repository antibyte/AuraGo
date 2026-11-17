package tools

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"aurago/internal/config"
)

func TestWorkspaceAttachmentRetainsOutgoingProtection(t *testing.T) {
	in := c103DefaultInstall(t)
	f, err := OpenWorkspaceAttachment(filepath.Join(in.workspace, "bericht.pdf"), in.cfg)
	if err != nil {
		t.Fatal(err)
	}
	f.Close()
	if f, err := OpenWorkspaceAttachment(filepath.Join(in.docs, "news.pdf"), in.cfg); err == nil {
		f.Close()
		t.Fatal("documents outside workspace accepted")
	}
	link := filepath.Join(in.workspace, "innocent.txt")
	if err := os.Link(filepath.Join(in.data, "vault.bin"), link); err != nil {
		t.Skipf("hardlink unavailable: %v", err)
	}
	if f, err := OpenWorkspaceAttachment(link, in.cfg); err == nil {
		f.Close()
		t.Fatal("vault hardlink accepted")
	}
}

func TestAnsibleRequiresUpdatedSidecarBeforeExecution(t *testing.T) {
	for _, version := range []int{0, 1, 2, 3} {
		t.Run(strconv.Itoa(version), func(t *testing.T) {
			calls := 0
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/status" {
					_ = json.NewEncoder(w).Encode(map[string]int{"security_policy": version})
					return
				}
				calls++
				_, _ = w.Write([]byte(`{}`))
			}))
			defer srv.Close()
			_, _, err := ansibleRequest(AnsibleConfig{URL: srv.URL}, http.MethodPost, "/run/playbook", map[string]bool{"check": true})
			if (err == nil) != (version == 3) || (calls > 0) != (version == 3) {
				t.Fatalf("version %d: calls=%d err=%v", version, calls, err)
			}
		})
	}
}

func TestGatewayBindingDriftAndCookieScheme(t *testing.T) {
	for _, host := range []string{"127.0.0.1", "::1", "192.0.2.1", "0.0.0.0"} {
		if manifestPublishHost(host) != host {
			t.Fatalf("interface changed: %s", host)
		}
	}
	manifest := ManifestSidecarConfig{Host: "127.0.0.1", Port: 2099, HostPort: 2099}
	for _, binding := range []string{"127.0.0.1", "0.0.0.0", "::"} {
		raw := []byte(`{"NetworkSettings":{"Networks":{"net":{}}},"HostConfig":{"PortBindings":{"2099/tcp":[{"HostIp":"` + binding + `","HostPort":"2099"}]}}}`)
		if manifestContainerNeedsRecreateWithAuth(raw, manifest, "net", false) != (binding != "127.0.0.1") {
			t.Fatalf("incorrect binding decision: %s", binding)
		}
	}
	for _, scheme := range []string{"http", "https"} {
		sidecar := OmniRouteSidecarConfig{Host: "127.0.0.1", Port: 20128, HostPort: 20128, BrowserBaseURL: scheme + "://gateway.example", DataVolume: "retained", MemoryMB: 512}
		for _, secure := range []bool{false, true} {
			raw := []byte(`{"Config":{"Env":["NEXT_PUBLIC_BASE_URL=` + sidecar.BrowserBaseURL + `","AUTH_COOKIE_SECURE=` + strconv.FormatBool(secure) + `"]},"HostConfig":{"Memory":536870912,"Binds":["retained:/app/data"],"PortBindings":{"20128/tcp":[{"HostIp":"127.0.0.1","HostPort":"20128"}]}},"NetworkSettings":{"Networks":{"net":{}}}}`)
			if omniRouteContainerNeedsRecreate(raw, sidecar, "net") != (secure != (scheme == "https")) {
				t.Fatalf("incorrect cookie decision: %s/%v", scheme, secure)
			}
		}
	}
}

func TestGatewayRecreationRetainsNamedVolumes(t *testing.T) {
	removed, created := false, false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/version" {
			_, _ = w.Write([]byte(`{"ApiVersion":"1.45","MinAPIVersion":"1.24"}`))
			return
		}
		switch {
		case strings.Contains(r.URL.Path, "/volumes/retained"):
			_, _ = w.Write([]byte(`{"Name":"retained","Driver":"local","Mountpoint":"/var/lib/docker/volumes/retained/_data"}`))
		case r.Method == http.MethodDelete:
			removed = true
			if r.URL.Query().Get("v") != "" || strings.Contains(r.URL.Path, "volumes") {
				t.Error("volume deletion requested")
			}
			w.WriteHeader(http.StatusNoContent)
		case strings.Contains(r.URL.Path, "/containers/create"):
			created = true
			var body struct{ HostConfig struct{ Binds []string } }
			if json.NewDecoder(r.Body).Decode(&body) != nil || len(body.HostConfig.Binds) != 1 || body.HostConfig.Binds[0] != "retained:/app/data" {
				t.Error("volume binding changed")
			}
			w.WriteHeader(http.StatusCreated)
		case r.Method == http.MethodPost:
			w.WriteHeader(http.StatusNoContent)
		default:
			_, _ = w.Write([]byte(`{}`))
		}
	}))
	defer srv.Close()
	err := ensureManifestContainerWithRecreate(context.Background(), DockerConfig{Host: "tcp://" + strings.TrimPrefix(srv.URL, "http://")}, "gateway", "image", func() ([]byte, error) { return []byte(`{"HostConfig":{"Binds":["retained:/app/data"]}}`), nil }, func([]byte) bool { return true })
	if err != nil || !removed || !created {
		t.Fatalf("recreate failed: removed=%v created=%v err=%v", removed, created, err)
	}
}

func TestAttachmentInputRejectsEmptyAndProtectedWorkspace(t *testing.T) {
	cfg := &config.Config{}
	if f, err := OpenWorkspaceAttachment("anything", cfg); err == nil {
		f.Close()
		t.Fatal("empty workspace accepted")
	}
	root := t.TempDir()
	cfg.Directories.WorkspaceDir = root
	cfg.Directories.DataDir = filepath.Join(root, "data")
	for _, name := range []string{"data/vault.bin", "data/private.txt", ".env", "config.yaml", "normal.txt"} {
		path := filepath.Join(root, name)
		if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte("fixture"), 0600); err != nil {
			t.Fatal(err)
		}
		f, err := OpenWorkspaceAttachment(path, cfg)
		if err == nil {
			f.Close()
			t.Fatalf("overbroad workspace accepted: %s", name)
		}
	}
	if f, err := OpenWorkspaceAttachment(root, cfg); err == nil {
		f.Close()
		t.Fatal("directory accepted")
	}
	link := filepath.Join(root, "innocent.txt")
	if err := os.Symlink(filepath.Join(root, "data", "vault.bin"), link); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	if f, err := OpenWorkspaceAttachment(link, cfg); err == nil {
		f.Close()
		t.Fatal("protected symlink accepted")
	}
}

func TestAnsibleInventoryProjectionAndReadOnly(t *testing.T) {
	raw := ansibleInventorySummary(`{"_meta":{"hostvars":{"host":{"ansible_host":"192.0.2.1","ansible_password":"fixture-secret","other":"hidden"}}},"all":{"hosts":["host"],"vars":{"secret":"fixture-secret"}}}`)
	if strings.Contains(raw, "fixture-secret") || strings.Contains(raw, "hidden") || !strings.Contains(raw, "192.0.2.1") {
		t.Fatalf("unsafe projection: %s", raw)
	}
	for _, args := range [][]string{{"site.yml", "--check"}, {"site.yml"}} {
		_, _, err := ansibleRunCmd(AnsibleLocalConfig{ReadOnly: true}, "ansible-playbook", args...)
		if err == nil {
			t.Fatal("read-only playbook accepted")
		}
	}
}

func TestHomepageVercelRejectsUnboundAndMismatchedTargets(t *testing.T) {
	root := t.TempDir()
	writeNetlifyDeployFiles(t, filepath.Join(root, "site"), map[string]string{"index.html": "<html>test</html>"})
	for _, tc := range []struct {
		name, bound, requested, team, response string
		manage                                 bool
		code                                   int
	}{
		{name: "unbound"},
		{name: "different requested ID", bound: "prj_bound", requested: "prj_other"},
		{name: "wrong project", bound: "prj_bound", response: `{"id":"prj_other","accountId":"user_owner"}`},
		{name: "wrong team", bound: "prj_bound", team: "team_owner", response: `{"id":"prj_bound","accountId":"team_other"}`},
		{name: "missing account", bound: "prj_bound", response: `{"id":"prj_bound"}`},
		{name: "name conflict", requested: "new-site", manage: true, code: 409, response: `{"error":{"message":"exists"}}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			posts := 0
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method == http.MethodPost {
					posts++
				}
				if !strings.Contains(r.URL.Path, "/projects") {
					t.Errorf("unexpected request: %s", r.URL.Path)
				}
				if tc.code != 0 {
					w.WriteHeader(tc.code)
				}
				_, _ = w.Write([]byte(tc.response))
			}))
			defer srv.Close()
			oldBase, oldClient, oldExec := vercelBaseURL, vercelHTTPClient, homepageDockerExecInternalFunc
			defer func() { vercelBaseURL, vercelHTTPClient, homepageDockerExecInternalFunc = oldBase, oldClient, oldExec }()
			vercelBaseURL, vercelHTTPClient = srv.URL, srv.Client()
			homepageDockerExecInternalFunc = func(DockerConfig, string, string, string, []string) string {
				t.Error("deployment executed")
				return `{}`
			}
			raw := HomepageDeployVercel(HomepageConfig{WorkspacePath: root}, VercelConfig{Token: "fixture", AllowDeploy: true, AllowProjectManagement: tc.manage, DefaultProjectID: tc.bound, TeamID: tc.team}, "site", ".", tc.requested, "preview", "", "", tc.manage, false, slogDiscard())
			var result map[string]interface{}
			if json.Unmarshal([]byte(raw), &result) != nil || result["status"] != "error" {
				t.Fatalf("target accepted: %s", raw)
			}
			want := 0
			if tc.manage {
				want = 1
			}
			if posts != want {
				t.Fatalf("POST count=%d, want %d", posts, want)
			}
		})
	}
}
