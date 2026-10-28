package server

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"aurago/internal/dockerstacks"
)

// dmLiveEngine is a fake Engine for the live routes of the Docker Manager: the
// container list, a few inspect documents, a held event feed, the held log
// stream of any container, the disk usage and (through the terminal fake of
// docker_manager_terminal_test.go) exec create, hijacked start, resize and a
// detached start. The exec calls address the container "web", whatever the path.
type dmLiveEngine struct {
	t    *testing.T
	srv  *httptest.Server
	term *dmTermEngine
	hold chan struct{} // closing it ends the held log follows

	mu         sync.Mutex
	containers string
	logQueries []string
	events     int
	statsCalls int
	resolves   map[string]dmLiveResolved
}

// dmLiveInspect is the inspect answer per container name: running or not, and labels.
var dmLiveInspect = map[string]struct {
	running bool
	labels  map[string]string
}{
	"web":     {true, nil},
	"stopped": {false, nil},
	"helper":  {true, map[string]string{dockerstacks.LabelRole: dockerstacks.RoleRunner}},
}

const dmLiveDF = `{"LayersSize":1000,"Images":[{"Size":1000,"SharedSize":0,"Containers":1}],` +
	`"Containers":[{"State":"running","SizeRw":50}],"Volumes":[{"UsageData":{"RefCount":1,"Size":200}}],"BuildCache":[{"InUse":false,"Shared":false,"Size":30}]}`

func newDMLiveEngine(t *testing.T, containers string) *dmLiveEngine {
	t.Helper()
	e := &dmLiveEngine{t: t, term: &dmTermEngine{}, hold: make(chan struct{}), containers: containers}
	e.srv = httptest.NewServer(http.HandlerFunc(e.serve))
	t.Cleanup(e.srv.Close)
	t.Cleanup(e.srv.CloseClientConnections)
	t.Cleanup(e.release)
	return e
}

func (e *dmLiveEngine) endpoint() string { return "tcp://" + e.srv.Listener.Addr().String() }

func (e *dmLiveEngine) port() int { return e.srv.Listener.Addr().(*net.TCPAddr).Port }

// release ends the held log follows; it can be called more than once.
func (e *dmLiveEngine) release() {
	e.mu.Lock()
	defer e.mu.Unlock()
	select {
	case <-e.hold:
	default:
		close(e.hold)
	}
}

func (e *dmLiveEngine) logRequests() []string {
	e.mu.Lock()
	defer e.mu.Unlock()
	return append([]string(nil), e.logQueries...)
}

func dmMuxFrame(stream byte, text string) []byte {
	header := make([]byte, 8)
	header[0] = stream
	binary.BigEndian.PutUint32(header[4:], uint32(len(text)))
	return append(header, text...)
}

func (e *dmLiveEngine) serve(w http.ResponseWriter, r *http.Request) {
	path := dmTermVersionPrefix.ReplaceAllString(r.URL.Path, "")
	parts := strings.Split(strings.Trim(path, "/"), "/")
	switch {
	case path == "/containers/json":
		e.mu.Lock()
		body := e.containers
		e.mu.Unlock()
		_, _ = io.WriteString(w, body)
	case r.Method == http.MethodGet && path == "/events":
		e.mu.Lock()
		e.events++
		e.mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.(http.Flusher).Flush()
		<-r.Context().Done()
	case r.Method == http.MethodGet && path == "/system/df":
		_, _ = io.WriteString(w, dmLiveDF)
	case r.Method == http.MethodGet && len(parts) == 3 && parts[0] == "containers" && parts[2] == "json":
		e.inspect(w, parts[1])
	case r.Method == http.MethodGet && len(parts) == 3 && parts[0] == "containers" && parts[2] == "logs":
		e.logs(w, r)
	case r.Method == http.MethodGet && len(parts) == 3 && parts[0] == "containers" && parts[2] == "stats":
		e.stats(w)
	case r.Method == http.MethodPost && len(parts) == 3 && parts[0] == "containers" && parts[2] == "exec":
		r.URL.Path = "/containers/web/exec"
		e.term.serve(w, r)
	default:
		e.term.serve(w, r)
	}
}

// dmLiveResolved is what the Engine's inspect answers for one reference the
// way dockerd resolves it: a short ID prefix or a container the list does not
// show. A non-zero status is an Engine failure instead of a document.
type dmLiveResolved struct {
	status  int
	id      string
	name    string
	running bool
	labels  map[string]string
}

// resolve makes the Engine answer the inspect of ref with the document of res,
// before the fixed table dmLiveInspect.
func (e *dmLiveEngine) resolve(ref string, res dmLiveResolved) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.resolves == nil {
		e.resolves = map[string]dmLiveResolved{}
	}
	e.resolves[ref] = res
}

func (e *dmLiveEngine) inspect(w http.ResponseWriter, name string) {
	e.mu.Lock()
	res, resolved := e.resolves[name]
	e.mu.Unlock()
	if resolved {
		if res.status != 0 {
			w.WriteHeader(res.status)
			_, _ = io.WriteString(w, `{"message":"engine failure"}`)
			return
		}
		doc, _ := json.Marshal(map[string]any{
			"Id": res.id, "Name": "/" + res.name,
			"State":  map[string]any{"Running": res.running, "Status": map[bool]string{true: "running", false: "exited"}[res.running]},
			"Config": map[string]any{"Image": "img:1", "Tty": false, "Env": []string{"PATH=/bin"}, "Cmd": []string{"sh"}, "Labels": res.labels},
		})
		_, _ = w.Write(doc)
		return
	}
	info, ok := dmLiveInspect[name]
	if !ok {
		w.WriteHeader(http.StatusNotFound)
		_, _ = io.WriteString(w, `{"message":"No such container: `+name+`"}`)
		return
	}
	labels := info.labels
	if labels == nil {
		labels = map[string]string{}
	}
	doc, _ := json.Marshal(map[string]any{
		"Id": strings.Repeat("a", 64), "Name": "/" + name,
		"State":  map[string]any{"Running": info.running, "Status": map[bool]string{true: "running", false: "exited"}[info.running]},
		"Config": map[string]any{"Image": "img:1", "Tty": false, "Env": []string{"PATH=/bin"}, "Cmd": []string{"sh"}, "Labels": labels},
	})
	_, _ = w.Write(doc)
}

// stats answers every stats request with counters that grow by one call's worth:
// 100 % of one core on 4 CPUs and memory that grows by 10 bytes, so the order of
// points can be told.
func (e *dmLiveEngine) stats(w http.ResponseWriter) {
	e.mu.Lock()
	e.statsCalls++
	n := uint64(e.statsCalls)
	e.mu.Unlock()
	_, _ = fmt.Fprintf(w, `{"read":"2026-10-28T08:00:03.25Z","pids_stats":{"current":5},`+
		`"cpu_stats":{"cpu_usage":{"total_usage":%d},"system_cpu_usage":%d,"online_cpus":4},`+
		`"precpu_stats":{"cpu_usage":{"total_usage":%d},"system_cpu_usage":%d,"online_cpus":4},`+
		`"memory_stats":{"usage":%d,"stats":{"inactive_file":100},"limit":1073741824},`+
		`"networks":{"eth0":{"rx_bytes":10,"tx_bytes":20}}}`, n*1e9, n*4e9, (n-1)*1e9, (n-1)*4e9, 1000+n*10+100)
}

// logs answers the log request of any container with two timestamped lines (one
// per stream) and, for a follow, holds the response open until the client leaves
// or the test releases it.
func (e *dmLiveEngine) logs(w http.ResponseWriter, r *http.Request) {
	e.mu.Lock()
	e.logQueries = append(e.logQueries, r.URL.RawQuery)
	e.mu.Unlock()
	w.Header().Set("Content-Type", "application/vnd.docker.multiplexed-stream")
	_, _ = w.Write(dmMuxFrame(1, "2026-10-28T07:33:29.000000001Z hello from the engine\n"))
	_, _ = w.Write(dmMuxFrame(2, "2026-10-28T07:33:30.000000002Z something went wrong\n"))
	w.(http.Flusher).Flush()
	if q := r.URL.Query().Get("follow"); q == "1" || q == "true" {
		select {
		case <-r.Context().Done():
		case <-e.hold:
		}
	}
}

// dmLiveServer is a Docker Manager server with the real host registry, the real
// hub and a fake local Engine, served over HTTP through handleDockerManager.
type dmLiveServer struct {
	t      *testing.T
	s      *Server
	engine *dmLiveEngine
	url    string
}

func newDMLiveServer(t *testing.T, containers string) *dmLiveServer {
	t.Helper()
	engine := newDMLiveEngine(t, containers)
	s := newDMServer(t, engine.endpoint())
	srv := httptest.NewServer(handleDockerManager(s))
	t.Cleanup(srv.Close)
	t.Cleanup(srv.CloseClientConnections)
	return &dmLiveServer{t: t, s: s, engine: engine, url: srv.URL}
}

// open sends a request (a JSON body when body is not nil) and returns the open
// response as a stream; it is closed when the test ends.
func (ls *dmLiveServer) open(method, path string, body any, headers map[string]string) *dmStream {
	ls.t.Helper()
	var reader io.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			ls.t.Fatal(err)
		}
		reader = bytes.NewReader(raw)
	}
	return dmOpenStream(ls.t, method, ls.url+"/api/docker-manager"+path, reader, headers)
}

// addTCPHost registers another fake Engine as a tcp host and returns its id.
func (ls *dmLiveServer) addTCPHost(name string, engine *dmLiveEngine) string {
	ls.t.Helper()
	raw, _ := json.Marshal(map[string]any{"name": name, "kind": "tcp", "address": "127.0.0.1", "port": engine.port()})
	rec := httptest.NewRecorder()
	handleDockerManager(ls.s)(rec, httptest.NewRequest(http.MethodPost, "/api/docker-manager/hosts", bytes.NewReader(raw)))
	var created struct {
		Host struct {
			ID string `json:"id"`
		} `json:"host"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &created); err != nil || rec.Code != http.StatusCreated || created.Host.ID == "" {
		ls.t.Fatalf("create host %s: %d %s", name, rec.Code, rec.Body.String())
	}
	return created.Host.ID
}
