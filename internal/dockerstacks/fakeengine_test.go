package dockerstacks

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path"
	"regexp"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"aurago/internal/dockerhost"
	"aurago/internal/dockerutil"
	"aurago/internal/testutil"
)

// fakeEngine is an httptest server that speaks the part of the Docker Engine
// API the stacks service uses, with state: containers (create, start, inspect,
// logs, kill, remove), images (inspect, pull, save, load, tag), an in-memory
// file tree behind the archive API, volumes and the system df.
//
// dockerutil's version transport probes GET /version first and rewrites
// "/v1.45/" paths to the negotiated version, so the fake answers /version with
// the configured API and serves every other route under /v1.45/ (any /v1.NN/
// request maps onto it).
//
// Helper containers behave by role when they are started:
//
//   - lister: prints the files of the fake file tree that `find` would list and
//     exits 0;
//   - runner: prints runnerLines (each as stdout) and exits with runnerExit, but
//     only after runnerGate is closed when it is set;
//   - anything else: stays running until the test finishes it.
type fakeEngine struct {
	t      *testing.T
	server *httptest.Server
	mux    *http.ServeMux

	mu         sync.Mutex
	api        string
	os, arch   string
	containers map[string]*fakeContainer // by id
	order      []string                  // container ids in creation order
	images     map[string]string         // reference -> image id
	files      map[string][]byte
	modes      map[string]int64
	dirs       map[string]bool
	volumes    []fakeVolume
	log        []string

	pullFails   string // non-empty: POST /images/create answers 500 with this message
	pulled      []string
	loadedBody  []byte
	loadReply   string
	savedRefs   []string
	runnerLines []string
	runnerExit  int
	runnerGate  chan struct{}
	runnerErr   string // written to stderr before the exit
	listerExit  int
	listerErr   string
	listerGate  chan struct{} // non-nil: a lister does not finish until it is closed or the lister is killed
	pullStall   bool          // POST /images/create sends one message and then goes silent
	loadStall   bool          // POST /images/load reads the body and then never answers
	startFails  string        // non-empty: POST .../start of a lister answers 500
	dfStatus    int           // non-zero: GET /system/df answers this status
	putQueries  []string
	nextID      int
}

type fakePort struct {
	IP      string
	Private int
	Public  int
	Proto   string
}

type fakeVolume struct {
	Name, Driver string
	Project      string
	Size         int64
}

type fakeContainer struct {
	id, name string
	image    string
	labels   map[string]string
	spec     map[string]any
	state    string // created | running | exited
	status   string // list status text; derived when empty
	health   string
	exitCode int
	ports    []fakePort
	created  int64
	logs     []byte
	done     chan struct{}
	tty      bool
}

const fakeStacksDir = "/opt/stacks"

func newFakeEngine(t *testing.T) *fakeEngine {
	t.Helper()
	fe := &fakeEngine{
		t:          t,
		api:        "1.45",
		os:         "linux",
		arch:       "amd64",
		containers: map[string]*fakeContainer{},
		images:     map[string]string{},
		files:      map[string][]byte{},
		modes:      map[string]int64{},
		dirs:       map[string]bool{fakeStacksDir: true},
		loadReply:  `{"stream":"Loaded image ID: sha256:loadedimage\n"}` + "\n",
	}
	fe.mux = http.NewServeMux()
	fe.routes()
	fe.server = testutil.NewHTTPServer(t, http.HandlerFunc(fe.serve))
	t.Cleanup(fe.server.Close)
	return fe
}

var reFakeAPIPrefix = regexp.MustCompile(`^/v1\.\d+/`)

func (fe *fakeEngine) serve(w http.ResponseWriter, r *http.Request) {
	entry := r.Method + " " + r.URL.Path
	if r.URL.RawQuery != "" {
		entry += "?" + r.URL.RawQuery
	}
	fe.mu.Lock()
	fe.log = append(fe.log, entry)
	fe.mu.Unlock()
	if r.URL.Path != "/version" {
		r2 := r.Clone(r.Context())
		r2.URL.Path = reFakeAPIPrefix.ReplaceAllString(r.URL.Path, "/v1.45/")
		r = r2
	}
	fe.mux.ServeHTTP(w, r)
}

func (fe *fakeEngine) routes() {
	m := fe.mux
	m.HandleFunc("GET /version", fe.handleVersion)
	m.HandleFunc("GET /v1.45/version", fe.handleVersion)
	m.HandleFunc("GET /v1.45/containers/json", fe.handleList)
	m.HandleFunc("POST /v1.45/containers/create", fe.handleCreate)
	m.HandleFunc("POST /v1.45/containers/{id}/start", fe.handleStart)
	m.HandleFunc("POST /v1.45/containers/{id}/kill", fe.handleKill)
	m.HandleFunc("GET /v1.45/containers/{id}/json", fe.handleInspect)
	m.HandleFunc("DELETE /v1.45/containers/{id}", fe.handleRemove)
	m.HandleFunc("GET /v1.45/containers/{id}/logs", fe.handleLogs)
	m.HandleFunc("GET /v1.45/containers/{id}/archive", fe.handleArchiveGet)
	m.HandleFunc("PUT /v1.45/containers/{id}/archive", fe.handleArchivePut)
	m.HandleFunc("GET /v1.45/images/{ref}/json", fe.handleImageInspect)
	m.HandleFunc("POST /v1.45/images/create", fe.handleImagePull)
	m.HandleFunc("GET /v1.45/images/{ref}/get", fe.handleImageSave)
	m.HandleFunc("POST /v1.45/images/load", fe.handleImageLoad)
	m.HandleFunc("POST /v1.45/images/{ref}/tag", fe.handleImageTag)
	m.HandleFunc("GET /v1.45/volumes", fe.handleVolumes)
	m.HandleFunc("GET /v1.45/system/df", fe.handleDF)
}

func fakeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

func fakeMessage(w http.ResponseWriter, status int, format string, args ...any) {
	fakeJSON(w, status, map[string]string{"message": fmt.Sprintf(format, args...)})
}

func (fe *fakeEngine) handleVersion(w http.ResponseWriter, r *http.Request) {
	fe.mu.Lock()
	defer fe.mu.Unlock()
	fakeJSON(w, http.StatusOK, map[string]string{
		"Version": "27.3.1", "ApiVersion": fe.api, "MinAPIVersion": "1.24", "Os": fe.os, "Arch": fe.arch,
	})
}

// ---- test-side helpers -------------------------------------------------------

func (fe *fakeEngine) host() dockerhost.Host {
	return dockerhost.Host{ID: "h_fake", Name: "fake", Kind: dockerhost.KindTCP, Address: "127.0.0.1", Port: 1,
		StacksDir: fakeStacksDir, HostSocketPath: dockerhost.DefaultHostSocketPath, Color: "blue"}
}

func (fe *fakeEngine) client(t *testing.T) *dockerutil.Client {
	t.Helper()
	port := fe.server.Listener.Addr().String()
	c := dockerutil.NewClient("tcp://"+port, 5*time.Second)
	t.Cleanup(c.CloseIdleConnections)
	return c
}

func (fe *fakeEngine) requests() []string {
	fe.mu.Lock()
	defer fe.mu.Unlock()
	return slices.Clone(fe.log)
}

// requestCount counts the logged requests that start with prefix ("POST /containers/create").
func (fe *fakeEngine) requestCount(prefix string) int {
	n := 0
	for _, r := range fe.requests() {
		method, rest, _ := strings.Cut(r, " ")
		rest = reFakeAPIPrefix.ReplaceAllString(rest, "/")
		if strings.HasPrefix(method+" "+rest, prefix) {
			n++
		}
	}
	return n
}

func (fe *fakeEngine) setVersion(api, os, arch string) {
	fe.mu.Lock()
	defer fe.mu.Unlock()
	fe.api, fe.os, fe.arch = api, os, arch
}

func (fe *fakeEngine) addImage(ref, id string) {
	fe.mu.Lock()
	defer fe.mu.Unlock()
	fe.images[ref] = id
}

func (fe *fakeEngine) hasImage(ref string) bool {
	fe.mu.Lock()
	defer fe.mu.Unlock()
	_, ok := fe.images[ref]
	return ok
}

// prepared gives the fake a host that is already prepared: the runner image and
// a matching IO container.
func (fe *fakeEngine) prepared() {
	fe.addImage(RunnerImage, "sha256:runnerimage")
	h := fe.host()
	fe.addContainer(IOContainerName, ioSpec(RunnerImage, h), "created", 0)
}

func (fe *fakeEngine) putFile(p, content string) {
	fe.mu.Lock()
	defer fe.mu.Unlock()
	fe.files[p] = []byte(content)
}

func (fe *fakeEngine) file(p string) (string, bool) {
	fe.mu.Lock()
	defer fe.mu.Unlock()
	b, ok := fe.files[p]
	return string(b), ok
}

func (fe *fakeEngine) mode(p string) int64 {
	fe.mu.Lock()
	defer fe.mu.Unlock()
	return fe.modes[p]
}

// addContainer registers a container with the payload it would have been created from.
func (fe *fakeEngine) addContainer(name string, spec containerSpec, state string, exit int) *fakeContainer {
	raw, _ := json.Marshal(spec)
	var generic map[string]any
	_ = json.Unmarshal(raw, &generic)
	fe.mu.Lock()
	defer fe.mu.Unlock()
	fc := fe.newContainerLocked(name, generic)
	fc.state, fc.exitCode = state, exit
	if state == "exited" {
		close(fc.done)
	}
	return fc
}

// addProjectContainer registers a compose project container of the fake host.
func (fe *fakeEngine) addProjectContainer(name, project, service, state, status, health string, created int64, ports ...fakePort) *fakeContainer {
	fe.mu.Lock()
	defer fe.mu.Unlock()
	fc := fe.newContainerLocked(name, map[string]any{"Image": service + ":latest"})
	fc.labels = map[string]string{
		"com.docker.compose.project":             project,
		"com.docker.compose.service":             service,
		"com.docker.compose.project.working_dir": path.Join(fakeStacksDir, project),
	}
	fc.image, fc.state, fc.status, fc.health, fc.created, fc.ports = service+":latest", state, status, health, created, ports
	if state == "exited" {
		fc.exitCode = 1
		close(fc.done)
	}
	return fc
}

func (fe *fakeEngine) container(name string) *fakeContainer {
	fe.mu.Lock()
	defer fe.mu.Unlock()
	for _, fc := range fe.containers {
		if fc.name == name {
			return fc
		}
	}
	return nil
}

func (fe *fakeEngine) containersWithRole(role string) []*fakeContainer {
	fe.mu.Lock()
	defer fe.mu.Unlock()
	var out []*fakeContainer
	for _, id := range fe.order {
		if fc := fe.containers[id]; fc != nil && fc.labels[LabelRole] == role {
			out = append(out, fc)
		}
	}
	return out
}

func (fe *fakeEngine) specOf(fc *fakeContainer) map[string]any {
	fe.mu.Lock()
	defer fe.mu.Unlock()
	return fc.spec
}

func (fe *fakeEngine) newContainerLocked(name string, spec map[string]any) *fakeContainer {
	fe.nextID++
	id := fmt.Sprintf("%012x%052x", fe.nextID, fe.nextID)
	labels := map[string]string{}
	if raw, ok := spec["Labels"].(map[string]any); ok {
		for k, v := range raw {
			labels[k], _ = v.(string)
		}
	}
	image, _ := spec["Image"].(string)
	fc := &fakeContainer{id: id, name: name, image: image, labels: labels, spec: spec, state: "created",
		created: time.Now().Unix(), done: make(chan struct{})}
	fe.containers[id] = fc
	fe.order = append(fe.order, id)
	return fc
}

func (fe *fakeEngine) lookupLocked(ref string) *fakeContainer {
	if fc := fe.containers[ref]; fc != nil {
		return fc
	}
	for _, fc := range fe.containers {
		if fc.name == ref || (len(ref) >= 4 && strings.HasPrefix(fc.id, ref)) {
			return fc
		}
	}
	return nil
}

// emit appends text to the container's log (stream 1 = stdout, 2 = stderr).
func (fe *fakeEngine) emit(fc *fakeContainer, stream byte, text string) {
	fe.mu.Lock()
	defer fe.mu.Unlock()
	header := make([]byte, 8)
	header[0] = stream
	binary.BigEndian.PutUint32(header[4:], uint32(len(text)))
	fc.logs = append(append(fc.logs, header...), text...)
}

// finish marks a running container as exited.
func (fe *fakeEngine) finish(fc *fakeContainer, exit int) {
	fe.mu.Lock()
	defer fe.mu.Unlock()
	if fc.state == "exited" {
		return
	}
	fc.state, fc.exitCode = "exited", exit
	close(fc.done)
}

func (fe *fakeEngine) isRunning(fc *fakeContainer) bool {
	fe.mu.Lock()
	defer fe.mu.Unlock()
	return fc.state == "running"
}

// setWorkingDir sets the compose working directory label of a project container.
func (fe *fakeEngine) setWorkingDir(fc *fakeContainer, dir string) {
	fe.mu.Lock()
	defer fe.mu.Unlock()
	fc.labels["com.docker.compose.project.working_dir"] = dir
}
