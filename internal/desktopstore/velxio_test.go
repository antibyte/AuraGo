package desktopstore

import (
	"context"
	"encoding/json"
	"os/exec"
	"reflect"
	"regexp"
	"strings"
	"testing"
	"time"
)

func velxioCatalogEntry(t *testing.T) CatalogEntry {
	t.Helper()
	for _, candidate := range DefaultCatalog() {
		if candidate.ID == VelxioAppID {
			return candidate
		}
	}
	t.Fatal("velxio missing from the catalog")
	return CatalogEntry{}
}

func TestVelxioCatalogEntry(t *testing.T) {
	entry := velxioCatalogEntry(t)
	if !regexp.MustCompile(`^ghcr\.io/davidmonterocrespo24/velxio@sha256:[0-9a-f]{64}$`).MatchString(entry.Image) {
		t.Fatalf("image %q is not a digest pin of the official image", entry.Image)
	}
	if entry.Category != "dev" || entry.Icon != "code" || entry.LogoSlug != "velxio" || entry.LogoURL != "" || entry.Metadata["open_external"] != "true" {
		t.Fatalf("entry presentation = %#v", entry)
	}
	if entry.PrimaryPort != (PortSpec{ID: "web", Name: "Web UI", ContainerPort: 80, Protocol: "tcp"}) {
		t.Fatalf("primary port = %#v", entry.PrimaryPort)
	}
	wantExtra := []PortSpec{{ID: "agent-mcp", Name: "Agent MCP (loopback only)", ContainerPort: velxioMCPPort, Protocol: "tcp", HostIP: "127.0.0.1"}}
	if !reflect.DeepEqual(entry.ExtraPorts, wantExtra) {
		t.Fatalf("extra ports = %#v", entry.ExtraPorts)
	}
	wantVolumes := []VolumeTemplate{
		{NameSuffix: "data", ContainerPath: "/app/data"},
		{NameSuffix: "arduino15", ContainerPath: "/root/.arduino15"},
		{NameSuffix: "arduino", ContainerPath: "/root/Arduino"},
		{NameSuffix: "ccache", ContainerPath: "/var/cache/ccache"},
		{NameSuffix: "build", ContainerPath: "/var/lib/velxio-build"},
	}
	if !reflect.DeepEqual(entry.Volumes, wantVolumes) {
		t.Fatalf("volumes = %#v", entry.Volumes)
	}
	if !reflect.DeepEqual(entry.Env, []string{"VELXIO_NEWS=off"}) {
		t.Fatalf("env = %#v", entry.Env)
	}
	if len(entry.GeneratedSecrets) != 1 || entry.GeneratedSecrets[0] != (GeneratedSecret{Key: "mcp_token", Env: "AURAGO_VELXIO_MCP_TOKEN", Label: "AuraGo MCP token"}) {
		t.Fatalf("generated secrets = %#v (the token must stay unexposed)", entry.GeneratedSecrets)
	}
	if entry.Hardening != nil || len(entry.HostBinds) != 0 || len(entry.WorkspaceBinds) != 0 || len(entry.Companions) != 0 {
		t.Fatalf("velxio must not opt into hardening, binds or companions: %#v", entry)
	}
	for _, phrase := range []string{"no login", "own browser tab", "7 GB", "first start"} {
		if !strings.Contains(entry.Description, phrase) {
			t.Fatalf("description misses %q: %q", phrase, entry.Description)
		}
	}
	if !HasIntegration(VelxioAppID) {
		t.Fatal("velxio must be an integration app")
	}
	if got := appReadinessTimeoutFor(InstalledApp{AppID: VelxioAppID}); got < 10*time.Minute {
		t.Fatalf("readiness timeout = %s", got)
	}
	if VelxioMCPTokenKey() != "desktop_store_velxio_mcp_token" {
		t.Fatalf("token key = %q", VelxioMCPTokenKey())
	}
}

func TestInstallVelxioWiresMCPLauncherAndKeepsTokenOutOfTheRecord(t *testing.T) {
	ctx := context.Background()
	docker := &fakeDockerAdapter{}
	secrets := &fakeSecretStore{data: map[string]string{}}
	s := newTestServiceWithSecrets(t, docker, &fakeDesktopAdapter{}, &fakeLaunchpadAdapter{}, fixedPorts(18080, 18002), secrets)
	op, err := s.StartInstall(ctx, InstallRequest{AppID: VelxioAppID, BindMode: BindModeLAN})
	if err != nil {
		t.Fatalf("start install: %v", err)
	}
	if err := s.RunOperation(ctx, op.ID); err != nil {
		t.Fatalf("run install: %v", err)
	}
	if len(docker.created) != 1 {
		t.Fatalf("created containers = %d", len(docker.created))
	}
	token := secrets.data[VelxioMCPTokenKey()]
	if !regexp.MustCompile(`^[0-9a-f]{32,}$`).MatchString(token) {
		t.Fatalf("vault token = %q", token)
	}
	checkSpec := func(spec ContainerSpec) {
		t.Helper()
		if !reflect.DeepEqual(spec.Entrypoint, []string{"/bin/sh", "-c", velxioWrapper, "aurago-velxio"}) {
			t.Fatalf("entrypoint = %#v", spec.Entrypoint)
		}
		for key, want := range map[string]string{
			"AURAGO_VELXIO_MCP_TOKEN": token,
			"AURAGO_VELXIO_MCP_PY":    velxioMCPLauncher,
			"AURAGO_VELXIO_MCP_PORT":  "8002",
			"VELXIO_NEWS":             "off",
		} {
			if got, _ := envValue(spec.Env, key); got != want {
				t.Fatalf("env %s = %q, want %q", key, got, want)
			}
		}
		var web, mcp *PortBinding
		for i := range spec.PortBindings {
			switch spec.PortBindings[i].ID {
			case "web":
				web = &spec.PortBindings[i]
			case "agent-mcp":
				mcp = &spec.PortBindings[i]
			}
		}
		if web == nil || web.HostIP != "0.0.0.0" || web.HostPort != 18080 || web.ContainerPort != 80 {
			t.Fatalf("web binding = %#v", web)
		}
		if mcp == nil || mcp.HostIP != "127.0.0.1" || mcp.HostPort != 18002 || mcp.ContainerPort != 8002 {
			t.Fatalf("agent-mcp binding = %#v", mcp)
		}
	}
	spec := docker.created[0]
	checkSpec(spec)
	for suffix, path := range map[string]string{"data": "/app/data", "arduino15": "/root/.arduino15", "arduino": "/root/Arduino", "ccache": "/var/cache/ccache", "build": "/var/lib/velxio-build"} {
		assertVolumeBinding(t, spec.Volumes, "aurago_store_velxio_"+suffix, path)
	}
	app, ok, err := s.GetInstalled(ctx, VelxioAppID)
	if err != nil || !ok {
		t.Fatalf("installed record: ok=%v err=%v", ok, err)
	}
	if len(app.SecretRefs) != 1 || app.SecretRefs[0].Expose {
		t.Fatalf("secret refs = %#v", app.SecretRefs)
	}
	public, _ := json.Marshal(app)
	var envJSON, refsJSON string
	if err := s.db.QueryRow(`SELECT env_json, secret_refs_json FROM desktop_store_apps WHERE app_id = ?`, VelxioAppID).Scan(&envJSON, &refsJSON); err != nil {
		t.Fatal(err)
	}
	for name, text := range map[string]string{"public JSON": string(public), "env_json": envJSON, "secret_refs_json": refsJSON} {
		if strings.Contains(text, token) || strings.Contains(text, "AURAGO_VELXIO_MCP_PY") {
			t.Fatalf("%s leaks the token or the launcher: %s", name, text)
		}
	}
	if _, _, err := s.OpenURL(ctx, VelxioAppID, "", false, "", "agent-mcp"); err == nil {
		t.Fatal("the agent MCP port must not be openable")
	}

	upd, err := s.StartAppOperation(ctx, VelxioAppID, OperationUpdate, OperationRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if err := s.RunOperation(ctx, upd.ID); err != nil {
		t.Fatalf("update: %v", err)
	}
	if len(docker.created) != 2 {
		t.Fatalf("created containers = %d, want install + update", len(docker.created))
	}
	if secrets.data[VelxioMCPTokenKey()] != token {
		t.Fatal("update changed the vault token")
	}
	checkSpec(docker.created[1])
}

func TestVelxioLauncherScriptAndWrapper(t *testing.T) {
	for _, want := range []string{`python -I -c "$AURAGO_VELXIO_MCP_PY"`, `unset AURAGO_VELXIO_MCP_PY AURAGO_VELXIO_MCP_TOKEN`, `exec /app/entrypoint.sh "$@"`} {
		if !strings.Contains(velxioWrapper, want) {
			t.Fatalf("wrapper misses %q", want)
		}
	}
	for _, want := range []string{
		"hmac.compare_digest", "enable_dns_rebinding_protection=False", "streamable_http_app()", "stateless_http = True",
		"http://127.0.0.1:8001/health", "AURAGO_VELXIO_MCP_TOKEN", "AURAGO_VELXIO_MCP_PORT", "time.sleep(3600)",
	} {
		if !strings.Contains(velxioMCPLauncher, want) {
			t.Fatalf("launcher misses %q", want)
		}
	}
	if strings.Contains(velxioMCPLauncher, "sys.exit(0)") {
		t.Fatal("a missing token must idle, not exit (the restart loop would spin)")
	}
	py, err := exec.LookPath("python3")
	if err != nil {
		if py, err = exec.LookPath("python"); err != nil {
			t.Skip("no python on PATH")
		}
	}
	cmd := exec.Command(py, "-I", "-c", "import ast,sys; ast.parse(sys.stdin.read())")
	cmd.Stdin = strings.NewReader(velxioMCPLauncher)
	if out, err := cmd.CombinedOutput(); err != nil {
		if strings.Contains(string(out), "SyntaxError") {
			t.Fatalf("launcher does not parse: %s", out)
		}
		t.Skipf("python is not usable here: %v %s", err, out)
	}
}

func TestCatalogPinnedPortHostMustBeLoopback(t *testing.T) {
	entry := func(hostIP string) CatalogEntry {
		return CatalogEntry{
			ID: "pin-test", Name: "Pin", Image: "example/pin:1",
			PrimaryPort: PortSpec{ID: "web", ContainerPort: 80, Protocol: "tcp"},
			ExtraPorts:  []PortSpec{{ID: "agent-x", ContainerPort: 90, Protocol: "tcp", HostIP: hostIP}},
		}
	}
	build := func(hostIP string) error {
		_, err := NewService(Config{
			DBPath:       t.TempDir() + "/pin.db",
			WorkspaceDir: t.TempDir(),
			Docker:       &fakeDockerAdapter{},
			Catalog:      []CatalogEntry{entry(hostIP)},
		})
		return err
	}
	if err := build("0.0.0.0"); err == nil || !strings.Contains(err.Error(), "loopback") {
		t.Fatalf("0.0.0.0 pin error = %v", err)
	}
	if err := build("127.0.0.1"); err != nil {
		t.Fatalf("127.0.0.1 pin rejected: %v", err)
	}
}
