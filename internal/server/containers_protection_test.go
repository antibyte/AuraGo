package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"slices"
	"strings"
	"testing"

	"aurago/internal/dockerutil"
	"aurago/internal/tools"

	"github.com/gorilla/websocket"
)

// replaceContainerProtection makes every terminal/update/remove target report p.
func replaceContainerProtection(p containerProtection) func() {
	old := containerProtectionFor
	containerProtectionFor = func(context.Context, *Server, tools.DockerConfig, string) containerProtection { return p }
	return func() { containerProtectionFor = old }
}

func replaceContainerSelfHostname(hostname string) func() {
	old := containerSelfHostname
	containerSelfHostname = func() (string, error) { return hostname, nil }
	return func() { containerSelfHostname = old }
}

// replaceContainerSelfProcFiles serves /proc fixtures by path; a path that is
// not in files reads as missing, as on a native or Windows host.
func replaceContainerSelfProcFiles(files map[string]string) func() {
	old := containerSelfProcFile
	containerSelfProcFile = func(path string) ([]byte, error) {
		if text, ok := files[path]; ok {
			return []byte(text), nil
		}
		return nil, os.ErrNotExist
	}
	return func() { containerSelfProcFile = old }
}

const (
	selfContainerID  = "4f6c1d0b9a2e8c7f53e1a0b2c4d6e8f0a1b3c5d7e9f1a3b5c7d9e1f3a5b7c9d1"
	otherContainerID = "9e8d7c6b5a4f3e2d1c0b9a8f7e6d5c4b3a2f1e0d9c8b7a6f5e4d3c2b1a0f9e8d"
)

// selfMountinfoFixture is /proc/self/mountinfo of an AuraGo container started
// with a custom compose hostname: the /etc files still come from the
// container's own directory under the Docker data root. The /mnt line mounts
// another container's file elsewhere and must be ignored.
const selfMountinfoFixture = `1520 1351 0:132 / / rw,relatime master:612 - overlay overlay rw,lowerdir=/var/lib/docker/overlay2/l/QX3V:/var/lib/docker/overlay2/l/7KDA,upperdir=/var/lib/docker/overlay2/9f1c2e/diff,workdir=/var/lib/docker/overlay2/9f1c2e/work
1521 1520 0:135 / /proc rw,nosuid,nodev,noexec,relatime - proc proc rw
1522 1520 0:136 / /dev rw,nosuid - tmpfs tmpfs rw,size=65536k,mode=755
1531 1520 8:1 /var/lib/docker/containers/` + selfContainerID + `/resolv.conf /etc/resolv.conf rw,relatime - ext4 /dev/sda1 rw,errors=remount-ro
1532 1520 8:1 /var/lib/docker/containers/` + selfContainerID + `/hostname /etc/hostname rw,relatime - ext4 /dev/sda1 rw,errors=remount-ro
1533 1520 8:1 /var/lib/docker/containers/` + selfContainerID + `/hosts /etc/hosts rw,relatime - ext4 /dev/sda1 rw,errors=remount-ro
1534 1520 8:1 /var/lib/docker/volumes/aurago_data/_data /app/data rw,relatime master:1 - ext4 /dev/sda1 rw,errors=remount-ro
1535 1520 8:1 /var/lib/docker/containers/` + otherContainerID + `/hostname /mnt/other-hostname ro,relatime - ext4 /dev/sda1 rw,errors=remount-ro
`

func replaceContainerEndpointAddresses(addrs ...string) func() {
	old := containerDockerEndpointAddresses
	containerDockerEndpointAddresses = func(context.Context, string) ([]string, error) { return addrs, nil }
	return func() { containerDockerEndpointAddresses = old }
}

// replaceContainerEndpointFailure makes the Docker endpoint lookup fail.
func replaceContainerEndpointFailure() func() {
	old := containerDockerEndpointAddresses
	containerDockerEndpointAddresses = func(context.Context, string) ([]string, error) {
		return nil, fmt.Errorf("lookup docker-proxy: server misbehaving")
	}
	return func() { containerDockerEndpointAddresses = old }
}

// newContainerDockerAPI serves a fake Docker Engine API and enables the Docker
// runtime permission for the duration of the test.
func newContainerDockerAPI(t *testing.T, handler http.HandlerFunc) string {
	t.Helper()
	tools.ConfigureRuntimePermissions(tools.RuntimePermissions{DockerEnabled: true})
	t.Cleanup(tools.ClearRuntimePermissionsForTest)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/version" {
			_, _ = w.Write([]byte(`{"ApiVersion":"1.45","MinAPIVersion":"1.25"}`))
			return
		}
		handler(w, r)
	}))
	t.Cleanup(srv.Close)
	return "tcp://" + strings.TrimPrefix(srv.URL, "http://")
}

func decodeContainerResponse(t *testing.T, rec *httptest.ResponseRecorder) map[string]string {
	t.Helper()
	var body map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode %q: %v", rec.Body.String(), err)
	}
	return body
}

func refusingDockerAPI(t *testing.T) string {
	t.Helper()
	return newContainerDockerAPI(t, func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"message":"engine refused"}`, http.StatusConflict)
	})
}

func TestContainerProtectedActionsRequireConfirmation(t *testing.T) {
	s := testContainerServer(true, false)
	s.Cfg.Docker.Host = refusingDockerAPI(t)
	fake := &fakeContainerTerminalBackend{running: true}
	defer replaceContainerTerminalBackend(fake)()
	defer replaceContainerProtection(containerProtection{Owner: "go2rtc"})()

	for _, req := range []*http.Request{
		newContainerTerminalUpgradeRequest("/api/containers/cams/terminal"),
		httptest.NewRequest(http.MethodPost, "/api/containers/cams/update", nil),
		httptest.NewRequest(http.MethodDelete, "/api/containers/cams?force=true", nil),
	} {
		rec := httptest.NewRecorder()
		handleContainerAction(s)(rec, req)
		body := decodeContainerResponse(t, rec)
		if rec.Code != http.StatusConflict || body["code"] != containerCodeConfirmationRequired || body["owner"] != "go2rtc" {
			t.Fatalf("%s %s = %d %v, want 409 %s", req.Method, req.URL, rec.Code, body, containerCodeConfirmationRequired)
		}
		if rec.Header().Get("Upgrade") != "" {
			t.Fatal("protected terminal upgraded without confirmation")
		}
	}
	if fake.createCalls != 0 {
		t.Fatalf("terminal sessions created without confirmation = %d", fake.createCalls)
	}

	// Confirmed update and remove reach the Docker engine, which refuses here.
	for _, req := range []*http.Request{
		httptest.NewRequest(http.MethodPost, "/api/containers/cams/update?confirm=protected", nil),
		httptest.NewRequest(http.MethodDelete, "/api/containers/cams?force=true&confirm=protected", nil),
	} {
		rec := httptest.NewRecorder()
		handleContainerAction(s)(rec, req)
		if !strings.Contains(rec.Body.String(), "engine refused") {
			t.Fatalf("%s %s did not reach Docker: %d %s", req.Method, req.URL, rec.Code, rec.Body.String())
		}
	}

	ts := httptest.NewServer(handleContainerAction(s))
	defer ts.Close()
	conn, _, err := websocket.DefaultDialer.Dial("ws"+ts.URL[len("http"):]+"/api/containers/cams/terminal?confirm=protected", nil)
	if err != nil {
		t.Fatalf("confirmed protected terminal: %v", err)
	}
	_ = conn.Close()
	if fake.createCalls != 1 {
		t.Fatalf("confirmed terminal sessions = %d, want 1", fake.createCalls)
	}
}

func TestContainerUpdateRefusesSelfAndDockerEndpoint(t *testing.T) {
	s := testContainerServer(true, false)
	for _, tc := range []struct {
		name  string
		p     containerProtection
		owner string
	}{
		{"self", containerProtection{Owner: dockerutil.AppOwner, Self: true}, "self"},
		{"docker endpoint", containerProtection{DockerEndpoint: true}, "docker-endpoint"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Cleanup(replaceContainerProtection(tc.p))
			for _, path := range []string{"/api/containers/x/update", "/api/containers/x/update?confirm=protected"} {
				rec := httptest.NewRecorder()
				handleContainerAction(s)(rec, httptest.NewRequest(http.MethodPost, path, nil))
				body := decodeContainerResponse(t, rec)
				if rec.Code != http.StatusConflict || body["code"] != containerCodeSelfUpdateUnsupported || body["owner"] != tc.owner || !strings.Contains(body["message"], "docker compose pull") {
					t.Fatalf("%s %s = %d %v, want 409 %s", tc.name, path, rec.Code, body, containerCodeSelfUpdateUnsupported)
				}
			}
		})
	}
}

// TestContainerConfirmationMessagesNameTheReason pins one message per label so
// an operator sees why a confirmation is needed.
func TestContainerConfirmationMessagesNameTheReason(t *testing.T) {
	s := testContainerServer(true, false)
	for _, tc := range []struct {
		p       containerProtection
		owner   string
		message string
	}{
		{containerProtection{Owner: "go2rtc"}, "go2rtc", "AuraGo manages this container (go2rtc)"},
		{containerProtection{Owner: dockerutil.AppOwner, Self: true}, "self", "AuraGo runs in this container."},
		{containerProtection{DockerEndpoint: true}, "docker-endpoint", "AuraGo reaches Docker through this container"},
		{containerProtection{SharedNetwork: true}, "shared-network", "shares its network namespace"},
		{containerProtection{Unverified: true}, "unverified", "Docker did not answer the ownership check"},
	} {
		t.Run(tc.owner, func(t *testing.T) {
			t.Cleanup(replaceContainerProtection(tc.p))
			rec := httptest.NewRecorder()
			handleContainerAction(s)(rec, httptest.NewRequest(http.MethodDelete, "/api/containers/x", nil))
			body := decodeContainerResponse(t, rec)
			if rec.Code != http.StatusConflict || body["code"] != containerCodeConfirmationRequired || body["owner"] != tc.owner || !strings.Contains(body["message"], tc.message) || !strings.Contains(body["message"], "confirm=protected") {
				t.Fatalf("%s: %d %v, want 409 with %q", tc.owner, rec.Code, body, tc.message)
			}
		})
	}
}

// TestContainerTerminalChecksOriginBeforeProtection: a cross-origin handshake
// is refused before any Docker request or DNS lookup.
func TestContainerTerminalChecksOriginBeforeProtection(t *testing.T) {
	s := testContainerServer(true, false)
	old := containerProtectionFor
	containerProtectionFor = func(context.Context, *Server, tools.DockerConfig, string) containerProtection {
		t.Fatal("a cross-origin terminal request must not consult container protection")
		return containerProtection{}
	}
	t.Cleanup(func() { containerProtectionFor = old })

	rec := httptest.NewRecorder()
	req := newContainerTerminalUpgradeRequest("/api/containers/demo/terminal")
	req.Header.Set("Origin", "http://evil.example")
	handleContainerAction(s)(rec, req)
	if rec.Code != http.StatusForbidden || rec.Header().Get("Upgrade") != "" {
		t.Fatalf("cross-origin terminal = %d (upgrade %q), want 403", rec.Code, rec.Header().Get("Upgrade"))
	}
}

func TestContainerSelfTerminalAndRemoveWorkAfterConfirmation(t *testing.T) {
	s := testContainerServer(true, false)
	s.Cfg.Docker.Host = refusingDockerAPI(t)
	fake := &fakeContainerTerminalBackend{running: true}
	defer replaceContainerTerminalBackend(fake)()
	defer replaceContainerProtection(containerProtection{Owner: dockerutil.AppOwner, Self: true})()

	rec := httptest.NewRecorder()
	handleContainerAction(s)(rec, httptest.NewRequest(http.MethodDelete, "/api/containers/aurago?force=true&confirm=protected", nil))
	if !strings.Contains(rec.Body.String(), "engine refused") {
		t.Fatalf("confirmed remove of the app container did not reach Docker: %d %s", rec.Code, rec.Body.String())
	}

	ts := httptest.NewServer(handleContainerAction(s))
	defer ts.Close()
	conn, _, err := websocket.DefaultDialer.Dial("ws"+ts.URL[len("http"):]+"/api/containers/aurago/terminal?confirm=protected", nil)
	if err != nil {
		t.Fatalf("confirmed shell in the app container: %v", err)
	}
	_ = conn.Close()
	if fake.createCalls != 1 {
		t.Fatalf("terminal sessions = %d, want 1", fake.createCalls)
	}
}

func TestContainerUnprotectedActionsNeedNoConfirmation(t *testing.T) {
	s := testContainerServer(true, false)
	s.Cfg.Docker.Host = refusingDockerAPI(t)
	fake := &fakeContainerTerminalBackend{running: true}
	defer replaceContainerTerminalBackend(fake)()
	defer replaceContainerProtection(containerProtection{})()

	for _, req := range []*http.Request{
		httptest.NewRequest(http.MethodPost, "/api/containers/web/update", nil),
		httptest.NewRequest(http.MethodDelete, "/api/containers/web", nil),
	} {
		rec := httptest.NewRecorder()
		handleContainerAction(s)(rec, req)
		if !strings.Contains(rec.Body.String(), "engine refused") {
			t.Fatalf("%s %s did not reach Docker: %d %s", req.Method, req.URL, rec.Code, rec.Body.String())
		}
	}
	ts := httptest.NewServer(handleContainerAction(s))
	defer ts.Close()
	conn, _, err := websocket.DefaultDialer.Dial("ws"+ts.URL[len("http"):]+"/api/containers/web/terminal", nil)
	if err != nil {
		t.Fatalf("unprotected terminal: %v", err)
	}
	_ = conn.Close()
}

func TestContainerLifecycleActionsNeverConsultProtection(t *testing.T) {
	s := testContainerServer(true, false)
	s.Cfg.Docker.Host = refusingDockerAPI(t)
	old := containerProtectionFor
	containerProtectionFor = func(context.Context, *Server, tools.DockerConfig, string) containerProtection {
		t.Fatal("start/stop/restart/logs/inspect/stats must not consult container protection")
		return containerProtection{}
	}
	defer func() { containerProtectionFor = old }()

	for _, req := range []*http.Request{
		httptest.NewRequest(http.MethodPost, "/api/containers/aurago/start", nil),
		httptest.NewRequest(http.MethodPost, "/api/containers/aurago/stop", nil),
		httptest.NewRequest(http.MethodPost, "/api/containers/aurago/restart", nil),
		httptest.NewRequest(http.MethodGet, "/api/containers/aurago/logs", nil),
		httptest.NewRequest(http.MethodGet, "/api/containers/aurago/inspect", nil),
		httptest.NewRequest(http.MethodGet, "/api/containers/aurago/stats", nil),
	} {
		rec := httptest.NewRecorder()
		handleContainerAction(s)(rec, req)
		if !strings.Contains(rec.Body.String(), "engine refused") {
			t.Fatalf("%s %s did not reach Docker: %d %s", req.Method, req.URL, rec.Code, rec.Body.String())
		}
	}
}

func TestClassifyContainerForActionUsesInspect(t *testing.T) {
	host := newContainerDockerAPI(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/containers/cams/json"):
			_, _ = w.Write([]byte(`{"Id":"aaaaaaaaaaaa1111","Name":"/cams","Config":{"Labels":{"aurago.managed":"go2rtc"}}}`))
		case strings.HasSuffix(r.URL.Path, "/containers/0123456789ab/json"):
			_, _ = w.Write([]byte(`{"Id":"0123456789abcdef","Name":"/aurago","Config":{"Labels":{}}}`))
		case strings.HasSuffix(r.URL.Path, "/containers/proxy/json"):
			_, _ = w.Write([]byte(`{"Id":"bbbbbbbbbbbb2222","Name":"/aurago_docker_proxy","Config":{"Labels":{"com.docker.compose.service":"docker-proxy"}},"NetworkSettings":{"Networks":{"aurago_docker-control":{"IPAddress":"172.18.0.5"}}}}`))
		case strings.HasSuffix(r.URL.Path, "/containers/web/json"):
			_, _ = w.Write([]byte(`{"Id":"cccccccccccc3333","Name":"/web","Config":{"Labels":{}},"NetworkSettings":{"Networks":{"bridge":{"IPAddress":"172.17.0.2"}}}}`))
		case strings.HasSuffix(r.URL.Path, "/containers/missing/json"), strings.HasSuffix(r.URL.Path, "/containers/aurago-boring-garage/json"):
			http.Error(w, `{"message":"No such container"}`, http.StatusNotFound)
		case strings.HasSuffix(r.URL.Path, "/containers/json"):
			// The self proof lists containers to rule out a shared network namespace.
			_, _ = w.Write([]byte(`[
				{"Id":"0123456789abcdef","Names":["/aurago"],"Image":"aurago","State":"running","Status":"Up","Labels":{},"HostConfig":{"NetworkMode":"aurago_default"}},
				{"Id":"cccccccccccc3333","Names":["/web"],"Image":"nginx","State":"running","Status":"Up","Labels":{},"HostConfig":{"NetworkMode":"bridge"}}
			]`))
		default:
			http.Error(w, `{"message":"engine refused"}`, http.StatusConflict)
		}
	})
	defer replaceContainerSelfHostname("0123456789ab")()
	defer replaceContainerSelfProcFiles(nil)()
	defer replaceContainerEndpointAddresses("172.18.0.5")()
	s := testContainerServer(true, false)
	s.Cfg.Runtime.IsDocker = true
	cfg := tools.DockerConfig{Host: host}
	ctx := context.Background()

	for id, want := range map[string]containerProtection{
		"cams":         {Owner: "go2rtc"},
		"0123456789ab": {Owner: dockerutil.AppOwner, Self: true},
		"proxy":        {DockerEndpoint: true},
		"web":          {},
		"missing":      {},
		// A 404 still names the owner of a reserved name, like the error path.
		"aurago-boring-garage": {Owner: dockerutil.BoringGarageOwner},
		"broken":               {Unverified: true},
		"aurago-local-llm":     {Owner: dockerutil.LocalLLMOwner, Unverified: true},
	} {
		if got := classifyContainerForAction(ctx, s, cfg, id); got != want {
			t.Fatalf("classify %q = %+v, want %+v", id, got, want)
		}
	}

	s.Cfg.Runtime.IsDocker = false
	if got := classifyContainerForAction(ctx, s, cfg, "0123456789ab"); got.Self {
		t.Fatal("a native runtime must never prove self")
	}
}

func TestDockerEndpointAddresses(t *testing.T) {
	old := containerEndpointLookup
	containerEndpointLookup = func(_ context.Context, host string) ([]string, error) {
		if host != "docker-proxy" {
			return nil, fmt.Errorf("unexpected lookup %q", host)
		}
		return []string{"172.18.0.5", "127.0.0.1"}, nil
	}
	defer func() { containerEndpointLookup = old }()

	ctx := context.Background()
	for host, want := range map[string][]string{
		"tcp://docker-proxy:2375":        {"172.18.0.5"},
		"docker-proxy:2375":              {"172.18.0.5"},
		"tcp://172.18.0.9:2375":          {"172.18.0.9"},
		"tcp://127.0.0.1:2375":           nil,
		"tcp://localhost:2375":           nil,
		"unix:///var/run/docker.sock":    nil,
		"npipe:////./pipe/docker_engine": nil,
		"":                               nil,
	} {
		got, err := dockerEndpointAddresses(ctx, host)
		if err != nil || !slices.Equal(got, want) {
			t.Fatalf("dockerEndpointAddresses(%q) = %v, %v; want %v", host, got, err, want)
		}
	}
	// A failed lookup is an error, never "no endpoint container".
	if got, err := dockerEndpointAddresses(ctx, "tcp://unresolvable:2375"); err == nil || got != nil {
		t.Fatalf("failed lookup = %v, %v; want an error", got, err)
	}
}

func TestContainerEndpointLookupFailureNeedsConfirmation(t *testing.T) {
	host := newContainerDockerAPI(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/containers/web/json"):
			_, _ = w.Write([]byte(`{"Id":"cccccccccccc3333","Name":"/web","Config":{"Labels":{}}}`))
		case strings.HasSuffix(r.URL.Path, "/containers/cams/json"):
			_, _ = w.Write([]byte(`{"Id":"aaaaaaaaaaaa1111","Name":"/cams","Config":{"Labels":{"aurago.managed":"go2rtc"}}}`))
		case strings.HasSuffix(r.URL.Path, "/containers/json"):
			_, _ = w.Write([]byte(`[
				{"Id":"aaaaaaaaaaaa1111","Names":["/cams"],"Image":"go2rtc","State":"running","Status":"Up","Labels":{"aurago.managed":"go2rtc"}},
				{"Id":"cccccccccccc3333","Names":["/web"],"Image":"nginx","State":"running","Status":"Up","Labels":{}}
			]`))
		default:
			http.NotFound(w, r)
		}
	})
	defer replaceContainerSelfHostname("aurago-host")()
	defer replaceContainerSelfProcFiles(nil)()
	defer replaceContainerEndpointFailure()()
	s := testContainerServer(true, false)
	s.Cfg.Runtime.IsDocker = true
	s.Cfg.Docker.Host = host
	cfg := tools.DockerConfig{Host: host}
	ctx := context.Background()

	// Terminal, update and remove: the endpoint container cannot be ruled out.
	if got, want := classifyContainerForAction(ctx, s, cfg, "web"), (containerProtection{Unverified: true}); got != want {
		t.Fatalf("web with failed endpoint lookup = %+v, want %+v", got, want)
	}
	if got, want := classifyContainerForAction(ctx, s, cfg, "cams"), (containerProtection{Owner: "go2rtc", Unverified: true}); got != want {
		t.Fatalf("cams with failed endpoint lookup = %+v, want %+v", got, want)
	}

	// The list marks no endpoint but keeps every other flag.
	rec := httptest.NewRecorder()
	handleContainersList(s)(rec, httptest.NewRequest(http.MethodGet, "/api/containers", nil))
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"protected_owner":"go2rtc"`) || strings.Contains(rec.Body.String(), "docker_endpoint") {
		t.Fatalf("list with failed endpoint lookup = %d %s", rec.Code, rec.Body.String())
	}
}

func TestContainerIsSelfProvenByMountinfoWithCustomHostname(t *testing.T) {
	defer replaceContainerSelfHostname("aurago-host")()
	defer replaceContainerSelfProcFiles(map[string]string{"/proc/self/mountinfo": selfMountinfoFixture, "/proc/self/cgroup": "0::/\n"})()

	if got := ownContainerID(); got != selfContainerID {
		t.Fatalf("ownContainerID = %q, want %q", got, selfContainerID)
	}
	if !containerIsSelf(true, selfContainerID) {
		t.Fatal("custom hostname: the mountinfo container ID must prove self")
	}
	if !containerIsSelf(true, strings.ToUpper(selfContainerID)) {
		t.Fatal("the comparison must ignore case")
	}
	if containerIsSelf(true, otherContainerID) {
		t.Fatal("a container whose file is mounted at /mnt must not count as self")
	}
	if containerIsSelf(false, selfContainerID) {
		t.Fatal("a native runtime must never prove self")
	}
	if containerIsSelf(true, "") {
		t.Fatal("an empty ID must never prove self")
	}
}

func TestOwnContainerIDFallsBackToCgroupV1(t *testing.T) {
	for _, cgroup := range []string{
		"12:memory:/docker/" + selfContainerID + "\n11:cpu,cpuacct:/docker/" + selfContainerID + "\n1:name=systemd:/docker/" + selfContainerID + "\n",
		"9:pids:/system.slice/docker-" + selfContainerID + ".scope\n1:name=systemd:/system.slice/docker-" + selfContainerID + ".scope\n",
	} {
		t.Run(cgroup[:strings.Index(cgroup, ":")], func(t *testing.T) {
			t.Cleanup(replaceContainerSelfProcFiles(map[string]string{"/proc/self/cgroup": cgroup}))
			if got := ownContainerID(); got != selfContainerID {
				t.Fatalf("ownContainerID from cgroup %q = %q, want %q", cgroup, got, selfContainerID)
			}
		})
	}
}

// TestOwnContainerIDReadsADedicatedContainersMount: when Docker's containers
// directory is its own filesystem, the mount root starts at /<id>/.
func TestOwnContainerIDReadsADedicatedContainersMount(t *testing.T) {
	t.Cleanup(replaceContainerSelfProcFiles(map[string]string{"/proc/self/mountinfo": "1531 1520 8:17 /" + selfContainerID + "/resolv.conf /etc/resolv.conf rw - ext4 /dev/sdb1 rw\n" +
		"1532 1520 8:17 /" + selfContainerID + "/hostname /etc/hostname rw - ext4 /dev/sdb1 rw\n"}))
	if got := ownContainerID(); got != selfContainerID {
		t.Fatalf("ownContainerID = %q, want %q", got, selfContainerID)
	}
}

func TestOwnContainerIDRefusesAmbiguousOrMissingSignals(t *testing.T) {
	for name, files := range map[string]map[string]string{
		"no proc files (native or Windows)": nil,
		"cgroup v2 private namespace":       {"/proc/self/mountinfo": "1520 1351 0:132 / / rw - overlay overlay rw\n", "/proc/self/cgroup": "0::/\n"},
		"conflicting /etc mounts": {"/proc/self/mountinfo": "1531 1520 8:1 /var/lib/docker/containers/" + selfContainerID + "/hostname /etc/hostname rw - ext4 /dev/sda1 rw\n" +
			"1532 1520 8:1 /var/lib/docker/containers/" + otherContainerID + "/hosts /etc/hosts rw - ext4 /dev/sda1 rw\n"},
		"foreign file outside /etc only": {"/proc/self/mountinfo": "1535 1520 8:1 /var/lib/docker/containers/" + otherContainerID + "/hostname /mnt/other-hostname ro - ext4 /dev/sda1 rw\n"},
		"short or non-hex ID":            {"/proc/self/mountinfo": "1531 1520 8:1 /var/lib/docker/containers/4f6c1d0b9a2e/hostname /etc/hostname rw - ext4 /dev/sda1 rw\n", "/proc/self/cgroup": "12:memory:/docker/not-a-container-id\n"},
		"conflicting cgroup lines":       {"/proc/self/cgroup": "12:memory:/docker/" + selfContainerID + "\n11:pids:/docker/" + otherContainerID + "\n"},
	} {
		t.Run(name, func(t *testing.T) {
			t.Cleanup(replaceContainerSelfProcFiles(files))
			if got := ownContainerID(); got != "" {
				t.Fatalf("%s: ownContainerID = %q, want none", name, got)
			}
		})
	}
}

func TestClassifyContainerForActionProvesSelfWithCustomHostname(t *testing.T) {
	host := newContainerDockerAPI(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/containers/aurago/json"):
			_, _ = w.Write([]byte(`{"Id":"` + selfContainerID + `","Name":"/aurago","Config":{"Hostname":"aurago-host","Labels":{}}}`))
		case strings.HasSuffix(r.URL.Path, "/containers/twin/json"):
			_, _ = w.Write([]byte(`{"Id":"` + otherContainerID + `","Name":"/aurago","Config":{"Labels":{}}}`))
		case strings.HasSuffix(r.URL.Path, "/containers/json"):
			_, _ = w.Write([]byte(`[
				{"Id":"` + selfContainerID + `","Names":["/aurago"],"Image":"aurago","State":"running","Status":"Up","Labels":{},"HostConfig":{"NetworkMode":"aurago_default"}}
			]`))
		default:
			http.NotFound(w, r)
		}
	})
	defer replaceContainerSelfHostname("aurago-host")()
	defer replaceContainerSelfProcFiles(map[string]string{"/proc/self/mountinfo": selfMountinfoFixture})()
	defer replaceContainerEndpointAddresses()()
	s := testContainerServer(true, false)
	s.Cfg.Runtime.IsDocker = true
	cfg := tools.DockerConfig{Host: host}
	ctx := context.Background()

	if got, want := classifyContainerForAction(ctx, s, cfg, "aurago"), (containerProtection{Owner: dockerutil.AppOwner, Self: true}); got != want {
		t.Fatalf("own container with custom hostname = %+v, want %+v", got, want)
	}
	// Another container named like the app (e.g. on a second host's engine)
	// keeps the confirmation fallback; it is never refused by name.
	if got, want := classifyContainerForAction(ctx, s, cfg, "twin"), (containerProtection{Owner: dockerutil.AppOwner}); got != want {
		t.Fatalf("other aurago-named container = %+v, want %+v", got, want)
	}
}

func TestAdminContainerListAddsProtectionFlags(t *testing.T) {
	host := newContainerDockerAPI(t, func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasSuffix(r.URL.Path, "/containers/json") {
			http.NotFound(w, r)
			return
		}
		_, _ = w.Write([]byte(`[
			{"Id":"0123456789abcdef","Names":["/aurago"],"Image":"ghcr.io/antibyte/aurago:latest","State":"running","Status":"Up 1 hour","Labels":{}},
			{"Id":"bbbbbbbbbbbb2222","Names":["/aurago_docker_proxy"],"Image":"tecnativa/docker-socket-proxy","State":"running","Status":"Up 1 hour","Labels":{"com.docker.compose.service":"docker-proxy"},"NetworkSettings":{"Networks":{"aurago_docker-control":{"IPAddress":"172.18.0.5"}}}},
			{"Id":"aaaaaaaaaaaa1111","Names":["/cams"],"Image":"alexxit/go2rtc","State":"running","Status":"Up 1 hour","Labels":{"aurago.managed":"go2rtc"}},
			{"Id":"cccccccccccc3333","Names":["/web"],"Image":"nginx:1","State":"exited","Status":"Exited (0)","Labels":{}}
		]`))
	})
	defer replaceContainerSelfHostname("0123456789ab")()
	defer replaceContainerSelfProcFiles(nil)()
	defer replaceContainerEndpointAddresses("172.18.0.5")()
	s := testContainerServer(true, false)
	s.Cfg.Runtime.IsDocker = true
	s.Cfg.Docker.Host = host

	rec := httptest.NewRecorder()
	handleContainersList(s)(rec, httptest.NewRequest(http.MethodGet, "/api/containers", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("list status = %d; body=%s", rec.Code, rec.Body.String())
	}
	var body struct {
		Status     string                   `json:"status"`
		Count      int                      `json:"count"`
		Containers []map[string]interface{} `json:"containers"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil || body.Status != "ok" || body.Count != 4 {
		t.Fatalf("list body = %s (%v)", rec.Body.String(), err)
	}
	want := map[string]map[string]interface{}{
		"0123456789ab": {"protected_owner": dockerutil.AppOwner, "self": true},
		"bbbbbbbbbbbb": {"docker_endpoint": true},
		"aaaaaaaaaaaa": {"protected_owner": "go2rtc"},
		"cccccccccccc": {},
	}
	for _, c := range body.Containers {
		id, _ := c["id"].(string)
		expect, ok := want[id]
		if !ok {
			t.Fatalf("unexpected container %v", c)
		}
		for _, key := range []string{"protected_owner", "self", "docker_endpoint", "shared_network"} {
			if c[key] != expect[key] {
				t.Fatalf("%s %s = %v, want %v", id, key, c[key], expect[key])
			}
		}
		for _, key := range []string{"names", "image", "state", "status"} {
			if _, ok := c[key]; !ok {
				t.Fatalf("%s lost list field %s: %v", id, key, c)
			}
		}
		for _, key := range []string{"labels", "Labels", "FullID", "full_id", "NetworkIPs", "network_ips", "NetworkMode", "network_mode"} {
			if _, leaked := c[key]; leaked {
				t.Fatalf("%s exposes internal field %s", id, key)
			}
		}
	}
}

// TestContainerSelfSignalsIgnoreASharedNetworkNamespace covers AuraGo behind a
// network sidecar (network_mode container:/service:, e.g. Tailscale or
// Gluetun): Docker gives AuraGo the provider's hostname and /etc files, so both
// self signals name the provider. Neither container is then refused as self;
// both need a confirmation instead.
func TestContainerSelfSignalsIgnoreASharedNetworkNamespace(t *testing.T) {
	const providerID, appID = selfContainerID, otherContainerID
	for _, tc := range []struct {
		name     string
		mode     string
		hostname string
		proc     map[string]string
	}{
		{"full ID, mountinfo", "container:" + providerID, "aurago-host", map[string]string{"/proc/self/mountinfo": selfMountinfoFixture}},
		{"short ID, default hostname", "container:" + providerID[:12], providerID[:12], nil},
		{"name, mountinfo", "container:tailscale", "aurago-host", map[string]string{"/proc/self/mountinfo": selfMountinfoFixture}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			host := newContainerDockerAPI(t, func(w http.ResponseWriter, r *http.Request) {
				switch {
				case strings.HasSuffix(r.URL.Path, "/containers/tailscale/json"):
					_, _ = w.Write([]byte(`{"Id":"` + providerID + `","Name":"/tailscale","Config":{"Labels":{}},"HostConfig":{"NetworkMode":"bridge"}}`))
				case strings.HasSuffix(r.URL.Path, "/containers/aurago/json"):
					_, _ = w.Write([]byte(`{"Id":"` + appID + `","Name":"/aurago","Config":{"Labels":{}},"HostConfig":{"NetworkMode":"` + tc.mode + `"}}`))
				case strings.HasSuffix(r.URL.Path, "/containers/json"):
					_, _ = w.Write([]byte(`[
						{"Id":"` + providerID + `","Names":["/tailscale"],"Image":"tailscale/tailscale","State":"running","Status":"Up","Labels":{},"HostConfig":{"NetworkMode":"bridge"}},
						{"Id":"` + appID + `","Names":["/aurago"],"Image":"aurago","State":"running","Status":"Up","Labels":{},"HostConfig":{"NetworkMode":"` + tc.mode + `"}}
					]`))
				default:
					http.Error(w, `{"message":"engine refused"}`, http.StatusConflict)
				}
			})
			t.Cleanup(replaceContainerSelfHostname(tc.hostname))
			t.Cleanup(replaceContainerSelfProcFiles(tc.proc))
			t.Cleanup(replaceContainerEndpointAddresses())
			s := testContainerServer(true, false)
			s.Cfg.Runtime.IsDocker = true
			s.Cfg.Docker.Host = host
			cfg := tools.DockerConfig{Host: host}
			ctx := context.Background()

			if got, want := classifyContainerForAction(ctx, s, cfg, "tailscale"), (containerProtection{SharedNetwork: true}); got != want {
				t.Fatalf("network provider = %+v, want %+v", got, want)
			}
			if got, want := classifyContainerForAction(ctx, s, cfg, "aurago"), (containerProtection{Owner: dockerutil.AppOwner, SharedNetwork: true}); got != want {
				t.Fatalf("app container joining the provider = %+v, want %+v", got, want)
			}

			// The provider's update needs a confirmation and is not refused.
			rec := httptest.NewRecorder()
			handleContainerAction(s)(rec, httptest.NewRequest(http.MethodPost, "/api/containers/tailscale/update", nil))
			body := decodeContainerResponse(t, rec)
			if rec.Code != http.StatusConflict || body["code"] != containerCodeConfirmationRequired || body["owner"] != "shared-network" {
				t.Fatalf("provider update = %d %v, want 409 %s", rec.Code, body, containerCodeConfirmationRequired)
			}
			rec = httptest.NewRecorder()
			handleContainerAction(s)(rec, httptest.NewRequest(http.MethodPost, "/api/containers/tailscale/update?confirm=protected", nil))
			if strings.Contains(rec.Body.String(), containerCodeSelfUpdateUnsupported) || strings.Contains(rec.Body.String(), containerCodeConfirmationRequired) {
				t.Fatalf("confirmed provider update was refused: %d %s", rec.Code, rec.Body.String())
			}

			rec = httptest.NewRecorder()
			handleContainersList(s)(rec, httptest.NewRequest(http.MethodGet, "/api/containers", nil))
			var list struct {
				Containers []map[string]interface{} `json:"containers"`
			}
			if err := json.Unmarshal(rec.Body.Bytes(), &list); err != nil || len(list.Containers) != 2 {
				t.Fatalf("list = %s (%v)", rec.Body.String(), err)
			}
			for _, c := range list.Containers {
				if c["self"] != nil || c["shared_network"] != true {
					t.Fatalf("list entry %v: want shared_network and no self", c)
				}
			}
		})
	}
}

// TestContainerSelfNeedsTheListToRuleOutASharedNamespace: when the signals
// name the target but Docker does not answer the list, self stays unproven.
func TestContainerSelfNeedsTheListToRuleOutASharedNamespace(t *testing.T) {
	host := newContainerDockerAPI(t, func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/containers/aurago/json") {
			_, _ = w.Write([]byte(`{"Id":"` + selfContainerID + `","Name":"/aurago","Config":{"Labels":{}}}`))
			return
		}
		http.Error(w, `{"message":"engine refused"}`, http.StatusConflict)
	})
	t.Cleanup(replaceContainerSelfHostname(selfContainerID[:12]))
	t.Cleanup(replaceContainerSelfProcFiles(nil))
	t.Cleanup(replaceContainerEndpointAddresses())
	s := testContainerServer(true, false)
	s.Cfg.Runtime.IsDocker = true

	if got, want := classifyContainerForAction(context.Background(), s, tools.DockerConfig{Host: host}, "aurago"), (containerProtection{Owner: dockerutil.AppOwner, Unverified: true}); got != want {
		t.Fatalf("self without a list answer = %+v, want %+v", got, want)
	}
}
