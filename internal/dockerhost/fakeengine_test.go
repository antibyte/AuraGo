package dockerhost

import (
	"encoding/binary"
	"encoding/pem"
	"io"
	"log"
	"net"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strconv"
	"sync"
	"testing"
	"time"

	"aurago/internal/dockerutil"
	"aurago/internal/testutil"
)

// reAPIPrefix matches the version segment the version transport negotiated, so
// a fake that reports an older API still serves the /v1.45 routes.
var reAPIPrefix = regexp.MustCompile(`^/v1\.\d+/`)

// fakeEngine is an httptest server that speaks the part of the Docker Engine
// API the host registry and the engine functions use. dockerutil's version
// transport probes GET /version first and rewrites "/v1.45/" paths to the
// negotiated version, so the fake answers /version with ApiVersion "1.45" and
// serves every other route under /v1.45/ (any /v1.NN/ request maps onto it).
//
//	fe := newFakeEngine(t)          // plain HTTP; newFakeTLSEngine(t) for HTTPS
//	fe.json("GET", "/containers/json", 200, `[...]`)   // route relative to /v1.45
//	fe.on("POST", "/containers/web/stop", func(w, r) { ... })
//	fe.withFixtures()                // containers, images, volumes, networks, info, one inspect
//	c := fe.client(t)                // *dockerutil.Client against the server
//	fe.requests()                    // "METHOD /path?query" of every request, in order
//
// Unregistered routes answer 404 with an Engine style {"message": ...} body.
// host() and port() give the listener address for Host records; caPEM() the
// server certificate (it is its own CA) of the TLS variant.
type fakeEngine struct {
	server *httptest.Server
	mu     sync.Mutex
	routes map[string]http.HandlerFunc
	log    []string
}

func newFakeEngine(t *testing.T) *fakeEngine {
	t.Helper()
	fe := &fakeEngine{routes: map[string]http.HandlerFunc{}}
	fe.server = testutil.NewHTTPServer(t, http.HandlerFunc(fe.serve))
	t.Cleanup(fe.server.Close)
	fe.defaults()
	return fe
}

func newFakeTLSEngine(t *testing.T) *fakeEngine {
	t.Helper()
	fe := &fakeEngine{routes: map[string]http.HandlerFunc{}}
	fe.server = testutil.NewHTTPSServer(t, http.HandlerFunc(fe.serve))
	fe.server.Config.ErrorLog = log.New(io.Discard, "", 0) // handshake errors of the negative tests
	t.Cleanup(fe.server.Close)
	fe.defaults()
	return fe
}

func (fe *fakeEngine) serve(w http.ResponseWriter, r *http.Request) {
	entry := r.Method + " " + r.URL.Path
	if r.URL.RawQuery != "" {
		entry += "?" + r.URL.RawQuery
	}
	fe.mu.Lock()
	fe.log = append(fe.log, entry)
	handler := fe.routes[r.Method+" "+reAPIPrefix.ReplaceAllString(r.URL.Path, "/v1.45/")]
	fe.mu.Unlock()
	if handler == nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		_, _ = io.WriteString(w, `{"message":"page not found"}`)
		return
	}
	handler(w, r)
}

func (fe *fakeEngine) defaults() {
	fe.setVersion("1.45", "27.3.1")
	fe.json("GET", "/info", http.StatusOK, infoFixture)
}

// setVersion answers GET /version and GET /v1.45/version with the given API
// and engine version.
func (fe *fakeEngine) setVersion(api, engine string) {
	body := `{"Version":"` + engine + `","ApiVersion":"` + api + `","MinAPIVersion":"1.24","Os":"linux","Arch":"amd64"}`
	handler := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, body)
	}
	fe.mu.Lock()
	defer fe.mu.Unlock()
	fe.routes["GET /version"] = handler
	fe.routes["GET /v1.45/version"] = handler
}

// on registers a handler for a route relative to /v1.45.
func (fe *fakeEngine) on(method, route string, handler http.HandlerFunc) {
	fe.mu.Lock()
	defer fe.mu.Unlock()
	fe.routes[method+" /v1.45"+route] = handler
}

func (fe *fakeEngine) json(method, route string, status int, body string) {
	fe.on(method, route, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, _ = io.WriteString(w, body)
	})
}

func (fe *fakeEngine) requests() []string {
	fe.mu.Lock()
	defer fe.mu.Unlock()
	return append([]string(nil), fe.log...)
}

func (fe *fakeEngine) host() string { return "127.0.0.1" }

func (fe *fakeEngine) port() int {
	return fe.server.Listener.Addr().(*net.TCPAddr).Port
}

func (fe *fakeEngine) caPEM() string {
	return string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: fe.server.Certificate().Raw}))
}

func (fe *fakeEngine) client(t *testing.T) *dockerutil.Client {
	t.Helper()
	c := dockerutil.NewClient("tcp://127.0.0.1:"+strconv.Itoa(fe.port()), 5*time.Second)
	t.Cleanup(c.CloseIdleConnections)
	return c
}

func (fe *fakeEngine) withFixtures() {
	fe.json("GET", "/containers/json", http.StatusOK, containersFixture)
	fe.json("GET", "/images/json", http.StatusOK, imagesFixture)
	fe.json("GET", "/volumes", http.StatusOK, volumesFixture)
	fe.json("GET", "/networks", http.StatusOK, networksFixture)
	fe.json("GET", "/containers/solo/json", http.StatusOK, inspectFixture)
}

// muxFrame is one frame of the multiplexed log stream: stream 1 is stdout,
// stream 2 stderr.
func muxFrame(stream byte, text string) []byte {
	header := make([]byte, 8)
	header[0] = stream
	binary.BigEndian.PutUint32(header[4:], uint32(len(text)))
	return append(header, text...)
}

const infoFixture = `{"Name":"nas","ServerVersion":"27.3.1","OperatingSystem":"Debian GNU/Linux 12 (bookworm)","OSType":"linux",` +
	`"KernelVersion":"6.1.0-26-amd64","Architecture":"x86_64","DockerRootDir":"/var/lib/docker","NCPU":4,"MemTotal":8589934592,` +
	`"Containers":3,"ContainersRunning":2,"ContainersPaused":0,"ContainersStopped":1,"Images":4}`

// Three containers: shop-web-1 and shop-db-1 belong to the compose project
// "shop", solo is a single exited container. The web container carries a
// credential-like label and a bind mount of a private host directory.
const containersFixture = `[
{"Id":"aaaaaaaaaaaa1111111111111111111111111111111111111111111111111111","Names":["/shop-web-1"],"Image":"nginx:1.27","ImageID":"sha256:img-nginx",
 "Created":1700000000,"State":"running","Status":"Up 2 hours (healthy)",
 "Ports":[{"IP":"::","PrivatePort":80,"PublicPort":8080,"Type":"tcp"},{"IP":"0.0.0.0","PrivatePort":80,"PublicPort":8080,"Type":"tcp"},{"PrivatePort":443,"Type":"tcp"}],
 "Labels":{"com.docker.compose.project":"shop","com.docker.compose.service":"web","com.docker.compose.project.working_dir":"/opt/stacks/shop","com.example.db_password":"label-secret"},
 "NetworkSettings":{"Networks":{"shop_default":{},"bridge":{}}},
 "Mounts":[{"Type":"bind","Source":"/srv/secret-dir/html","Destination":"/usr/share/nginx/html"},{"Type":"volume","Name":"shop_data","Source":"/var/lib/docker/volumes/shop_data/_data","Destination":"/data"}]},
{"Id":"bbbbbbbbbbbb2222222222222222222222222222222222222222222222222222","Names":["/shop-db-1"],"Image":"postgres:16","ImageID":"sha256:img-postgres",
 "Created":1700000100,"State":"running","Status":"Up 5 seconds (health: starting)","Ports":[],
 "Labels":{"com.docker.compose.project":"shop","com.docker.compose.service":"db"},
 "NetworkSettings":{"Networks":{"shop_default":{}}},
 "Mounts":[{"Type":"volume","Name":"shop_pg","Source":"/var/lib/docker/volumes/shop_pg/_data","Destination":"/var/lib/postgresql/data"}]},
{"Id":"cccccccccccc3333333333333333333333333333333333333333333333333333","Names":["/solo"],"Image":"redis:7","ImageID":"sha256:img-redis",
 "Created":1700000200,"State":"exited","Status":"Exited (1) 3 minutes ago","Ports":null,"Labels":null,
 "NetworkSettings":{"Networks":{"bridge":{}}},"Mounts":null}
]`

const imagesFixture = `[
{"Id":"sha256:img-nginx","RepoTags":["nginx:1.27","nginx:latest"],"RepoDigests":["nginx@sha256:aa"],"Created":1690000000,"Size":187000000,"Containers":-1},
{"Id":"sha256:img-postgres","RepoTags":["postgres:16"],"RepoDigests":["postgres@sha256:bb"],"Created":1690000100,"Size":430000000,"Containers":-1},
{"Id":"sha256:img-redis","RepoTags":["redis:7"],"RepoDigests":["redis@sha256:cc"],"Created":1690000200,"Size":117000000,"Containers":-1},
{"Id":"sha256:img-old","RepoTags":["<none>:<none>"],"RepoDigests":null,"Created":1600000000,"Size":50000000,"Containers":-1}
]`

const volumesFixture = `{"Volumes":[
{"Name":"shop_data","Driver":"local","Mountpoint":"/var/lib/docker/volumes/shop_data/_data","CreatedAt":"2024-01-01T10:00:00Z","Labels":{"com.docker.compose.project":"shop"}},
{"Name":"shop_pg","Driver":"local","Mountpoint":"/var/lib/docker/volumes/shop_pg/_data","CreatedAt":"2024-01-01T10:00:01Z","Labels":null},
{"Name":"orphan","Driver":"local","Mountpoint":"/var/lib/docker/volumes/orphan/_data","CreatedAt":"2024-02-01T10:00:00Z","Labels":{}}
],"Warnings":null}`

const networksFixture = `[
{"Id":"net-bridge","Name":"bridge","Driver":"bridge","Scope":"local","IPAM":{"Driver":"default","Config":[{"Subnet":"172.17.0.0/16","Gateway":"172.17.0.1"}]}},
{"Id":"net-host","Name":"host","Driver":"host","Scope":"local","IPAM":{"Driver":"default","Config":[]}},
{"Id":"net-none","Name":"none","Driver":"null","Scope":"local","IPAM":{"Driver":"default","Config":null}},
{"Id":"net-shop","Name":"shop_default","Driver":"bridge","Scope":"local","IPAM":{"Driver":"default","Config":[{"Subnet":"172.20.0.0/16"},{"Subnet":"fd00:1::/64"}]}}
]`

// inspectFixture is the inspect document of the container "solo": secret
// environment values, a credential flag in the command line, a secret label and
// a bind mount of a private host directory.
const inspectFixture = `{"Id":"cccccccccccc3333333333333333333333333333333333333333333333333333","Name":"/solo",` +
	`"State":{"Status":"exited","ExitCode":1},"Mounts":[{"Type":"bind","Source":"/srv/secret-dir","Destination":"/data","Mode":"ro","RW":false}],` +
	`"Config":{"Image":"redis:7","Tty":false,"Env":["REDIS_PASSWORD=hunter2","PATH=/usr/bin"],"Cmd":["redis-server","--requirepass","cmd-secret"],` +
	`"Labels":{"com.example.api_token":"label-secret","com.docker.compose.service":"cache"}},"HostConfig":{"NetworkMode":"bridge"}}`
