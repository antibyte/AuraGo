package server

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"testing"

	"aurago/internal/security"
	"aurago/internal/tools"
)

// dmFakeEngine stands in for a Docker Engine: it answers the handful of
// endpoints the Docker Manager reads and records every call as
// "METHOD /path" (API version prefix and query stripped).
type dmFakeEngine struct {
	t          *testing.T
	srv        *httptest.Server
	mu         sync.Mutex
	calls      []string
	containers string
	forced     map[string]int
}

var dmEngineVersionPrefix = regexp.MustCompile(`^/v[0-9]+\.[0-9]+`)

func newDMFakeEngine(t *testing.T, containers string) *dmFakeEngine {
	t.Helper()
	e := &dmFakeEngine{t: t, containers: containers, forced: map[string]int{}}
	e.srv = httptest.NewServer(http.HandlerFunc(e.serve))
	t.Cleanup(e.srv.Close)
	return e
}

func (e *dmFakeEngine) port() int { return e.srv.Listener.Addr().(*net.TCPAddr).Port }

func (e *dmFakeEngine) endpoint() string { return "tcp://" + e.srv.Listener.Addr().String() }

func (e *dmFakeEngine) force(call string, status int) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.forced[call] = status
}

func (e *dmFakeEngine) called(call string) bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	for _, got := range e.calls {
		if got == call {
			return true
		}
	}
	return false
}

func (e *dmFakeEngine) serve(w http.ResponseWriter, r *http.Request) {
	path := dmEngineVersionPrefix.ReplaceAllString(r.URL.Path, "")
	call := r.Method + " " + path
	e.mu.Lock()
	e.calls = append(e.calls, call)
	status, forced := e.forced[call]
	containers := e.containers
	e.mu.Unlock()
	notFound := func() {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"message":"No such container: missing"}`))
	}
	switch {
	case forced:
		w.WriteHeader(status)
		_, _ = w.Write([]byte(`{"message":"forced answer"}`))
	case path == "/version":
		_, _ = w.Write([]byte(`{"Version":"27.0.1","ApiVersion":"1.45","MinAPIVersion":"1.25","Os":"linux","Arch":"amd64"}`))
	case path == "/info":
		_, _ = w.Write([]byte(`{"Name":"fakehost","ServerVersion":"27.0.1","OperatingSystem":"Debian","OSType":"linux","KernelVersion":"6.1","Architecture":"x86_64","DockerRootDir":"/var/lib/docker","NCPU":4,"MemTotal":8000000000,"Containers":2,"ContainersRunning":1,"ContainersPaused":0,"ContainersStopped":1,"Images":3}`))
	case path == "/containers/json":
		_, _ = w.Write([]byte(containers))
	case strings.HasPrefix(path, "/containers/missing"):
		notFound()
	case r.Method == http.MethodGet && strings.HasSuffix(path, "/logs"):
		name := strings.Split(path, "/")[2]
		payload := []byte("2026-10-26T10:00:00Z hello from " + name + "\n")
		header := make([]byte, 8)
		header[0] = 1
		binary.BigEndian.PutUint32(header[4:], uint32(len(payload)))
		_, _ = io.Copy(w, bytes.NewReader(append(header, payload...)))
	case r.Method == http.MethodGet && strings.HasPrefix(path, "/containers/") && strings.HasSuffix(path, "/json"):
		name := strings.Split(path, "/")[2]
		_, _ = w.Write([]byte(`{"Id":"` + strings.Repeat("a", 64) + `","Name":"/` + name + `","Config":{"Tty":false,"Env":["PATH=/bin"],"Labels":{}},"State":{"Status":"running"}}`))
	case strings.HasPrefix(path, "/containers/"):
		w.WriteHeader(http.StatusNoContent)
	case path == "/images/json":
		_, _ = w.Write([]byte(`[{"Id":"sha256:img1","RepoTags":["nginx:1.27"],"Size":1000,"Created":1700000000},{"Id":"sha256:img2","RepoTags":["ruview:1"],"Size":2000,"Created":1700000100},{"Id":"sha256:old","RepoTags":null,"Size":300,"Created":1600000000}]`))
	case path == "/volumes":
		_, _ = w.Write([]byte(`{"Volumes":[{"Name":"data","Driver":"local","Mountpoint":"/var/lib/docker/volumes/data/_data","CreatedAt":"2026-10-01T00:00:00Z","Labels":null},{"Name":"idle","Driver":"local","Mountpoint":"/var/lib/docker/volumes/idle/_data","CreatedAt":"2026-10-02T00:00:00Z","Labels":null}],"Warnings":null}`))
	case path == "/networks":
		_, _ = w.Write([]byte(`[{"Id":"n1","Name":"bridge","Driver":"bridge","Scope":"local","IPAM":{"Config":[{"Subnet":"172.17.0.0/16"}]}},{"Id":"n2","Name":"appnet","Driver":"bridge","Scope":"local","IPAM":{"Config":[{"Subnet":"172.20.0.0/16"}]}},{"Id":"n3","Name":"idle","Driver":"bridge","Scope":"local","IPAM":{"Config":[]}}]`))
	default:
		http.NotFound(w, r)
	}
}

// dmContainerJSON builds one /containers/json entry.
func dmContainerJSON(id byte, name, imageID, state, network string, labels map[string]string, mounts []map[string]any) map[string]any {
	if labels == nil {
		labels = map[string]string{}
	}
	if mounts == nil {
		mounts = []map[string]any{}
	}
	return map[string]any{
		"Id": strings.Repeat(string(id), 64), "Names": []string{"/" + name}, "Image": name + ":1", "ImageID": imageID,
		"State": state, "Status": state, "Created": 1700000000, "Labels": labels,
		"Ports":  []map[string]any{{"IP": "0.0.0.0", "PrivatePort": 80, "PublicPort": 8080, "Type": "tcp"}},
		"Mounts": mounts, "HostConfig": map[string]any{"NetworkMode": "default"},
		"NetworkSettings": map[string]any{"Networks": map[string]any{network: map[string]any{"IPAddress": "172.17.0.2"}}},
	}
}

func dmContainersJSON(t *testing.T, entries ...map[string]any) string {
	t.Helper()
	raw, err := json.Marshal(entries)
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}

// dmFixture is a server with Docker enabled, the host registry open, a fake
// local engine (docker.host) and a fake remote engine for tcp hosts.
type dmFixture struct {
	t       *testing.T
	s       *Server
	local   *dmFakeEngine
	remote  *dmFakeEngine
	lastRaw string
}

func newDMFixture(t *testing.T) *dmFixture {
	t.Helper()
	local := newDMFakeEngine(t, dmContainersJSON(t,
		dmContainerJSON('a', "web1", "sha256:img1", "running", "bridge",
			map[string]string{"com.docker.compose.project": "shop", "com.docker.compose.service": "web"},
			[]map[string]any{{"Type": "volume", "Name": "data", "Source": "/var/lib/docker/volumes/data/_data", "Destination": "/data"}}),
		dmContainerJSON('b', "rv", "sha256:img2", "exited", "appnet", map[string]string{"aurago.managed": "ruview"}, nil),
	))
	remote := newDMFakeEngine(t, dmContainersJSON(t,
		dmContainerJSON('c', "blog", "sha256:img1", "running", "bridge", nil, nil),
		dmContainerJSON('d', "cams", "sha256:img2", "running", "bridge", map[string]string{"aurago.managed": "ruview"}, nil),
		dmContainerJSON('e', "web1", "sha256:img1", "running", "bridge", nil, nil),
	))
	return &dmFixture{t: t, s: newDMServer(t, local.endpoint()), local: local, remote: remote}
}

// newDMServer is a server with Docker enabled, a vault and the host registry
// (with the stacks service and the live data hub) open on the local Engine at
// endpoint (docker.host). It is closed when the test ends.
func newDMServer(t *testing.T, endpoint string) *Server {
	t.Helper()
	tools.ConfigureRuntimePermissions(tools.RuntimePermissions{DockerEnabled: true})
	t.Cleanup(tools.ClearRuntimePermissionsForTest)

	dir := t.TempDir()
	vault, err := security.NewVault(strings.Repeat("d", 64), filepath.Join(dir, "vault.bin"))
	if err != nil {
		t.Fatalf("NewVault: %v", err)
	}
	s := testContainerServer(true, false)
	s.Cfg.Docker.Host = endpoint
	s.Cfg.Directories.DataDir = dir
	s.Vault = vault
	s.initDockerHosts()
	t.Cleanup(s.closeDockerHosts)
	if s.DockerHosts == nil {
		t.Fatal("initDockerHosts left DockerHosts nil")
	}
	return s
}

// do calls the Docker Manager handler directly and decodes the JSON answer.
func (f *dmFixture) do(method, path string, body any) (int, map[string]any) {
	f.t.Helper()
	var reader io.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			f.t.Fatal(err)
		}
		reader = bytes.NewReader(raw)
	}
	req := httptest.NewRequest(method, "/api/docker-manager"+path, reader)
	rec := httptest.NewRecorder()
	handleDockerManager(f.s)(rec, req)
	f.lastRaw = rec.Body.String()
	var decoded map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &decoded); err != nil {
		f.t.Fatalf("%s %s: answer %q is not JSON: %v", method, path, f.lastRaw, err)
	}
	return rec.Code, decoded
}

// setDocker changes the Docker switches under the config lock.
func (f *dmFixture) setDocker(enabled, readOnly bool) {
	f.s.CfgMu.Lock()
	defer f.s.CfgMu.Unlock()
	f.s.Cfg.Docker.Enabled = enabled
	f.s.Cfg.Docker.ReadOnly = readOnly
}

// addRemote registers the fake remote engine as a plain TCP host and returns its id.
func (f *dmFixture) addRemote(name string) string {
	f.t.Helper()
	code, body := f.do(http.MethodPost, "/hosts", map[string]any{"name": name, "kind": "tcp", "address": "127.0.0.1", "port": f.remote.port()})
	if code != http.StatusCreated {
		f.t.Fatalf("create remote host: %d %s", code, f.lastRaw)
	}
	return dmString(f.t, body["host"], "id")
}

func dmString(t *testing.T, v any, key string) string {
	t.Helper()
	m, ok := v.(map[string]any)
	if !ok {
		t.Fatalf("%v is not an object", v)
	}
	s, ok := m[key].(string)
	if !ok {
		t.Fatalf("member %q of %v is not a string", key, m)
	}
	return s
}

func dmObject(t *testing.T, v any, key string) map[string]any {
	t.Helper()
	m, ok := v.(map[string]any)
	if !ok {
		t.Fatalf("%v is not an object", v)
	}
	child, ok := m[key].(map[string]any)
	if !ok {
		t.Fatalf("member %q of %v is not an object", key, m)
	}
	return child
}

func dmArray(t *testing.T, v any, key string) []map[string]any {
	t.Helper()
	m, ok := v.(map[string]any)
	if !ok {
		t.Fatalf("%v is not an object", v)
	}
	items, ok := m[key].([]any)
	if !ok {
		t.Fatalf("member %q of %v is not an array", key, m)
	}
	out := make([]map[string]any, 0, len(items))
	for _, item := range items {
		obj, ok := item.(map[string]any)
		if !ok {
			t.Fatalf("element %v of %q is not an object", item, key)
		}
		out = append(out, obj)
	}
	return out
}

func dmFind(t *testing.T, items []map[string]any, key, value string) map[string]any {
	t.Helper()
	for _, item := range items {
		if item[key] == value {
			return item
		}
	}
	t.Fatalf("no element with %s=%q in %v", key, value, items)
	return nil
}

func dmExpectError(t *testing.T, f *dmFixture, code int, body map[string]any, wantStatus int, wantCode string) {
	t.Helper()
	if code != wantStatus || body["status"] != "error" || body["code"] != wantCode {
		t.Fatalf("answer = %d %s, want %d error %q", code, f.lastRaw, wantStatus, wantCode)
	}
}
