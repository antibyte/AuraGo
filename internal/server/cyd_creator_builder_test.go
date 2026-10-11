package server

import (
	"archive/tar"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"
	"unicode/utf8"

	"aurago/internal/cydcreator"
	"aurago/internal/dockerutil"
	"aurago/internal/tools"
)

// fakeCYDEngine emulates the Docker Engine endpoints the CYD builder uses.
// Uploaded files land in files (container path → content); cyd-build reads
// "<work>/stub.json" {"exit":N,"lines":K} and writes out/ artefacts.
type fakeCYDEngine struct {
	t            *testing.T
	mu           sync.Mutex
	imagePresent bool
	imageStatus  int // when set, GET /images/{name}/json answers this status
	pullBody     string
	container    map[string]any // create spec, nil when absent
	running      bool
	labels       map[string]string
	volumes      []string
	files        map[string][]byte
	execs        map[string]*fakeExec
	execOrder    [][]string
	stops        int
	starts       int
	removes      int
	nextExec     int
	failStart    bool          // POST /start answers 500
	archiveGone  bool          // GET /archive answers 404 "No such container"
	inspectFail  bool          // GET /containers/{name}/json answers 500
	blockBuild   chan struct{} // when set, cyd-build blocks until it is closed or the request ends
	blockUpload  chan struct{} // when set, PUT /archive closes uploadSeen and blocks until the request ends
	uploadSeen   chan struct{}
}

type fakeExec struct {
	cmd  []string
	exit int
}

func newFakeCYDEngine(t *testing.T) (*fakeCYDEngine, tools.DockerConfig) {
	t.Helper()
	e := &fakeCYDEngine{t: t, imagePresent: true, files: map[string][]byte{}, execs: map[string]*fakeExec{}}
	srv := httptest.NewServer(http.HandlerFunc(e.serve))
	t.Cleanup(srv.Close)
	tools.ConfigureRuntimePermissions(tools.RuntimePermissions{DockerEnabled: true})
	t.Cleanup(tools.ClearRuntimePermissionsForTest)
	return e, tools.DockerConfig{Host: "tcp://" + strings.TrimPrefix(srv.URL, "http://")}
}

func sha256HexForTest(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func frame(stream byte, text string) []byte {
	hdr := make([]byte, 8)
	hdr[0] = stream
	binary.BigEndian.PutUint32(hdr[4:], uint32(len(text)))
	return append(hdr, text...)
}

func (e *fakeCYDEngine) serve(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path == "/version" {
		_, _ = io.WriteString(w, `{"ApiVersion":"1.45"}`)
		return
	}
	p := r.URL.Path
	if strings.HasPrefix(p, "/v") {
		p = p[strings.Index(p[1:], "/")+1:]
	}
	e.mu.Lock()
	defer e.mu.Unlock() // runExec may release and retake it while cyd-build blocks
	switch {
	case r.Method == http.MethodGet && strings.HasPrefix(p, "/images/") && strings.HasSuffix(p, "/json"):
		if e.imageStatus != 0 {
			w.WriteHeader(e.imageStatus)
			_, _ = io.WriteString(w, `{"message":"refused by the proxy at unix:///var/run/docker.sock"}`)
			return
		}
		if !e.imagePresent {
			w.WriteHeader(http.StatusNotFound)
			_, _ = io.WriteString(w, `{"message":"No such image"}`)
			return
		}
		_, _ = io.WriteString(w, `{"Id":"sha256:abc"}`)
	case r.Method == http.MethodPost && p == "/images/create":
		e.imagePresent = true
		_, _ = io.WriteString(w, e.pullBody)
	case r.Method == http.MethodPost && p == "/volumes/create":
		var body struct {
			Name   string            `json:"Name"`
			Labels map[string]string `json:"Labels"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		if body.Labels["aurago.managed"] != "cyd-creator" {
			e.t.Errorf("volume %s labels = %v", body.Name, body.Labels)
		}
		e.volumes = append(e.volumes, body.Name)
		w.WriteHeader(http.StatusCreated)
		_, _ = io.WriteString(w, `{}`)
	case r.Method == http.MethodGet && p == "/containers/aurago-cyd-builder/json", r.Method == http.MethodGet && p == "/containers/ctr1/json":
		if e.inspectFail {
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = io.WriteString(w, `{"message":"engine busy"}`)
			return
		}
		if e.container == nil {
			w.WriteHeader(http.StatusNotFound)
			_, _ = io.WriteString(w, `{"message":"No such container"}`)
			return
		}
		data, _ := json.Marshal(map[string]any{"Id": "ctr1", "State": map[string]any{"Running": e.running}, "Config": map[string]any{"Labels": e.labels}})
		_, _ = w.Write(data)
	case r.Method == http.MethodPost && p == "/containers/create":
		if r.URL.Query().Get("name") != "aurago-cyd-builder" {
			e.t.Errorf("container name = %q", r.URL.Query().Get("name"))
		}
		var spec map[string]any
		_ = json.NewDecoder(r.Body).Decode(&spec)
		e.container = spec
		e.labels = map[string]string{}
		for k, v := range spec["Labels"].(map[string]any) {
			e.labels[k] = v.(string)
		}
		w.WriteHeader(http.StatusCreated)
		_, _ = io.WriteString(w, `{"Id":"ctr1"}`)
	case r.Method == http.MethodDelete && p == "/containers/ctr1":
		if e.container == nil {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		e.container, e.running = nil, false
		e.removes++
		w.WriteHeader(http.StatusNoContent)
	case r.Method == http.MethodPost && p == "/containers/ctr1/start":
		if e.failStart {
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = io.WriteString(w, `{"message":"OCI runtime create failed: /var/lib/docker/overlay2/x: permission denied"}`)
			return
		}
		e.running = true
		e.starts++
		w.WriteHeader(http.StatusNoContent)
	case r.Method == http.MethodPost && p == "/containers/ctr1/stop":
		e.running = false
		e.stops++
		w.WriteHeader(http.StatusNoContent)
	case r.Method == http.MethodPut && p == "/containers/ctr1/archive":
		if r.URL.Query().Get("path") != "/work" || r.Header.Get("Content-Type") != "application/x-tar" {
			e.t.Errorf("archive upload %s (%s)", r.URL.RawQuery, r.Header.Get("Content-Type"))
		}
		if block := e.blockUpload; block != nil {
			close(e.uploadSeen)
			e.mu.Unlock()
			select {
			case <-block:
			case <-r.Context().Done():
			}
			e.mu.Lock()
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		tr := tar.NewReader(r.Body)
		for {
			hdr, err := tr.Next()
			if err != nil {
				break
			}
			if hdr.Uid != 1000 || hdr.Gid != 1000 {
				e.t.Errorf("%s owned by %d:%d", hdr.Name, hdr.Uid, hdr.Gid)
			}
			if hdr.Typeflag == tar.TypeReg {
				data, _ := io.ReadAll(tr)
				e.files["/work/"+hdr.Name] = data
			}
		}
		w.WriteHeader(http.StatusOK)
	case r.Method == http.MethodGet && p == "/containers/ctr1/archive":
		dir := r.URL.Query().Get("path")
		if e.archiveGone {
			w.WriteHeader(http.StatusNotFound)
			_, _ = io.WriteString(w, `{"message":"No such container: ctr1"}`)
			return
		}
		// Like the Engine: entries are named below the base name of the
		// requested path ("out/..."), starting with its directory entry.
		var buf bytes.Buffer
		tw := tar.NewWriter(&buf)
		_ = tw.WriteHeader(&tar.Header{Name: "out/", Typeflag: tar.TypeDir, Mode: 0o755})
		found := false
		for name, data := range e.files {
			if strings.HasPrefix(name, dir+"/") {
				found = true
				_ = tw.WriteHeader(&tar.Header{Name: "out/" + strings.TrimPrefix(name, dir+"/"), Typeflag: tar.TypeReg, Mode: 0o644, Size: int64(len(data))})
				_, _ = tw.Write(data)
			}
		}
		_ = tw.Close()
		if !found {
			w.WriteHeader(http.StatusNotFound)
			_, _ = fmt.Fprintf(w, `{"message":"Could not find the file %s in container ctr1"}`, dir)
			return
		}
		_, _ = w.Write(buf.Bytes())
	case r.Method == http.MethodPost && p == "/containers/ctr1/exec":
		var body struct {
			Cmd []string `json:"Cmd"`
			Tty bool     `json:"Tty"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		if body.Tty {
			e.t.Error("exec must not allocate a TTY")
		}
		e.nextExec++
		id := fmt.Sprintf("exec%d", e.nextExec)
		e.execs[id] = &fakeExec{cmd: body.Cmd}
		e.execOrder = append(e.execOrder, body.Cmd)
		w.WriteHeader(http.StatusCreated)
		_, _ = fmt.Fprintf(w, `{"Id":%q}`, id)
	case r.Method == http.MethodPost && strings.HasPrefix(p, "/exec/") && strings.HasSuffix(p, "/start"):
		ex := e.execs[strings.TrimSuffix(strings.TrimPrefix(p, "/exec/"), "/start")]
		e.runExec(w, r, ex)
	case r.Method == http.MethodGet && strings.HasPrefix(p, "/exec/") && strings.HasSuffix(p, "/json"):
		ex := e.execs[strings.TrimSuffix(strings.TrimPrefix(p, "/exec/"), "/json")]
		_, _ = fmt.Fprintf(w, `{"Running":false,"ExitCode":%d}`, ex.exit)
	default:
		e.t.Errorf("unexpected Docker request %s %s", r.Method, r.URL.String())
		w.WriteHeader(http.StatusNotFound)
	}
}

// runExec answers an exec start; it is called with e.mu held and may release it.
func (e *fakeCYDEngine) runExec(w http.ResponseWriter, r *http.Request, ex *fakeExec) {
	w.Header().Set("Content-Type", "application/vnd.docker.multiplexed-stream")
	switch ex.cmd[0] {
	case "cyd-build":
		work := ex.cmd[2]
		var stub struct {
			Exit     int  `json:"exit"`
			Lines    int  `json:"lines"`
			BadFrame bool `json:"badframe"`
		}
		_ = json.Unmarshal(e.files[work+"/stub.json"], &stub)
		_, _ = w.Write(frame(1, "Processing "+ex.cmd[4]+"\n"))
		if stub.BadFrame {
			_, _ = w.Write(frame(0, "not an output stream\n"))
			return
		}
		for i := 0; i < stub.Lines; i++ {
			_, _ = w.Write(frame(1, fmt.Sprintf("line %d\n", i)))
		}
		_, _ = w.Write(frame(2, "src/app/app_main.cpp:3:1: warning: partial "))
		_, _ = w.Write(frame(2, "line joined\n"))
		w.(http.Flusher).Flush()
		if block := e.blockBuild; block != nil {
			e.mu.Unlock()
			select {
			case <-block:
			case <-r.Context().Done():
			}
			e.mu.Lock()
		}
		ex.exit = stub.Exit
		if stub.Exit == 0 || stub.Exit == 3 {
			result := map[string]any{"ok": stub.Exit == 0, "exit": stub.Exit, "env": ex.cmd[4], "chip": "esp32", "duration_ms": 10,
				"flash": map[string]int{"used": 100, "total": 200}, "ram": map[string]int{"used": 1, "total": 2}, "files": map[string]any{}}
			if stub.Exit == 0 {
				for _, name := range []string{"merged.bin", "firmware.bin", "firmware.elf"} {
					e.files[work+"/out/"+name] = []byte(name + "-content")
				}
			}
			data, _ := json.Marshal(result)
			e.files[work+"/out/result.json"] = data
		}
	case "cyd-addr2line":
		elf := e.files[ex.cmd[2]]
		if string(elf) != "ELF" || ex.cmd[4] != "esp32s3" {
			e.t.Errorf("addr2line got elf %q chip %q", elf, ex.cmd[4])
		}
		frames := []map[string]any{}
		for _, addr := range ex.cmd[5:] {
			frames = append(frames, map[string]any{"address": addr, "function": "app_loop()", "file": "src/app/app_main.cpp", "line": 42})
		}
		data, _ := json.Marshal(frames)
		_, _ = w.Write(frame(1, string(data)+"\n"))
	case "rm":
		for name := range e.files {
			if strings.HasPrefix(name, ex.cmd[3]+"/") {
				delete(e.files, name)
			}
		}
	case "sh":
	default:
		e.t.Errorf("unexpected exec %v", ex.cmd)
	}
}

func newTestCYDBuilder(cfg tools.DockerConfig, idle time.Duration) *cydDockerBuilder {
	b := newCYDDockerBuilder(cfg, cydBuilderSettings{Image: "ghcr.io/antibyte/aurago-cyd-builder:1", MemoryMB: 2048, CPUCores: 2, IdleStop: idle},
		slog.New(slog.NewTextHandler(io.Discard, nil)))
	b.stopGrace = 0
	return b
}

func stubTree(exit, lines int) map[string][]byte {
	return map[string][]byte{
		"stub.json":            []byte(fmt.Sprintf(`{"exit":%d,"lines":%d}`, exit, lines)),
		"src/app/app_main.cpp": []byte("void app_setup() {}\n"),
		"platformio.ini":       []byte("[env:esp32-2432s028r]\n"),
	}
}

func TestCYDBuilderBuildCreatesAHardenedContainerAndStreamsTheLog(t *testing.T) {
	engine, cfg := newFakeCYDEngine(t)
	b := newTestCYDBuilder(cfg, 0)
	var lines []string
	out, err := b.Build(context.Background(), cydcreator.BuildRequest{BuildID: "ccb_0001", Env: "esp32-2432s028r",
		CacheKey: "ccp_1-esp32-2432s028r", Tree: stubTree(0, 3), Timeout: time.Minute}, func(line string) { lines = append(lines, line) })
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	if out.ExitCode != 0 || string(out.Files["merged.bin"]) != "merged.bin-content" || len(out.Files) != 3 ||
		out.Result.Env != "esp32-2432s028r" || out.Result.Flash.Used != 100 {
		t.Fatalf("output = %+v", out)
	}
	wantLines := []string{"Processing esp32-2432s028r", "line 0", "line 1", "line 2", "src/app/app_main.cpp:3:1: warning: partial line joined"}
	if !slices.Equal(lines, wantLines) {
		t.Fatalf("log lines = %q, want %q", lines, wantLines)
	}
	if out.Log != strings.Join(wantLines, "\n")+"\n" {
		t.Fatalf("log = %q", out.Log)
	}
	engine.mu.Lock()
	defer engine.mu.Unlock()
	spec := engine.container
	host := spec["HostConfig"].(map[string]any)
	checks := map[string]bool{
		"user 1000":       spec["User"] == "1000:1000",
		"sleep infinity":  fmt.Sprint(spec["Cmd"]) == "[sleep infinity]",
		"network none":    host["NetworkMode"] == "none",
		"cap drop":        fmt.Sprint(host["CapDrop"]) == "[ALL]",
		"no new privs":    fmt.Sprint(host["SecurityOpt"]) == "[no-new-privileges:true]",
		"readonly rootfs": host["ReadonlyRootfs"] == true,
		"tmpfs":           fmt.Sprint(host["Tmpfs"]) == "map[/tmp:rw,exec,nosuid,nodev,size=1g]",
		"memory":          host["Memory"] == float64(2048<<20) && host["MemorySwap"] == float64(2048<<20),
		"cpus":            host["NanoCpus"] == float64(2e9),
		"pids":            host["PidsLimit"] == float64(512),
		"init":            host["Init"] == true,
		"home":            fmt.Sprint(spec["Env"]) == "[HOME=/tmp]",
		"managed label":   engine.labels["aurago.managed"] == "cyd-creator" && engine.labels["aurago.fingerprint"] != "",
		"volumes":         strings.Contains(fmt.Sprint(host["Mounts"]), "Source:aurago-cyd-cache Target:/cache") && strings.Contains(fmt.Sprint(host["Mounts"]), "Source:aurago-cyd-work Target:/work"),
		// Pin the exact spec: any added key (Privileged, Binds, PidMode,
		// IpcMode, CapAdd, Devices, ...) or a changed value fails here.
		"exact mounts":      fmt.Sprint(host["Mounts"]) == "[map[Source:aurago-cyd-cache Target:/cache Type:volume] map[Source:aurago-cyd-work Target:/work Type:volume]]",
		"restart policy":    fmt.Sprint(host["RestartPolicy"]) == "map[Name:no]",
		"exact host keys":   strings.Join(sortedAnyKeys(host), ",") == "CapDrop,Init,Memory,MemorySwap,Mounts,NanoCpus,NetworkMode,PidsLimit,ReadonlyRootfs,RestartPolicy,SecurityOpt,Tmpfs",
		"exact spec keys":   strings.Join(sortedAnyKeys(spec), ",") == "Cmd,Env,HostConfig,Image,Labels,User,WorkingDir",
		"image and workdir": spec["Image"] == "ghcr.io/antibyte/aurago-cyd-builder:1" && spec["WorkingDir"] == "/work",
	}
	for name, ok := range checks {
		if !ok {
			t.Errorf("container spec lacks %s: %v", name, spec)
		}
	}
	if !slices.Equal(engine.volumes, []string{"aurago-cyd-cache", "aurago-cyd-work"}) {
		t.Errorf("volumes = %v", engine.volumes)
	}
	var build []string
	for _, cmd := range engine.execOrder {
		if cmd[0] == "cyd-build" {
			build = cmd
		}
	}
	if !slices.Equal(build, []string{"cyd-build", "--work", "/work/ccb_0001", "--env", "esp32-2432s028r", "--build-id", "ccb_0001", "--cache-key", "ccp_1-esp32-2432s028r"}) {
		t.Errorf("cyd-build command = %v", build)
	}
	for name := range engine.files {
		if strings.HasPrefix(name, "/work/ccb_0001/") {
			t.Errorf("work directory was not removed: %s", name)
		}
	}
}

func TestCYDBuilderExitCodesAndMissingArtefacts(t *testing.T) {
	_, cfg := newFakeCYDEngine(t)
	b := newTestCYDBuilder(cfg, 0)
	for _, exit := range []int{1, 2, 3} {
		out, err := b.Build(context.Background(), cydcreator.BuildRequest{BuildID: fmt.Sprintf("ccb_%04d", exit), Env: "esp32-2432s028r",
			CacheKey: "k", Tree: stubTree(exit, 0)}, nil)
		if err != nil || out.ExitCode != exit || len(out.Files) != 0 {
			t.Fatalf("exit %d: out %+v, %v", exit, out, err)
		}
		if exit == 3 && out.Result.Exit != 3 {
			t.Fatalf("exit 3 must keep result.json: %+v", out.Result)
		}
	}
}

func TestCYDBuilderReusesAndRecreatesTheContainer(t *testing.T) {
	engine, cfg := newFakeCYDEngine(t)
	b := newTestCYDBuilder(cfg, 0)
	req := cydcreator.BuildRequest{BuildID: "ccb_0001", Env: "esp32-2432s028r", CacheKey: "k", Tree: stubTree(0, 0)}
	if _, err := b.Build(context.Background(), req, nil); err != nil {
		t.Fatal(err)
	}
	engine.mu.Lock()
	engine.running = false
	engine.mu.Unlock()
	req.BuildID = "ccb_0002"
	if _, err := b.Build(context.Background(), req, nil); err != nil {
		t.Fatal(err)
	}
	engine.mu.Lock()
	if engine.removes != 0 || engine.starts != 2 {
		t.Fatalf("stopped container: removes %d starts %d, want a restart only", engine.removes, engine.starts)
	}
	engine.mu.Unlock()
	b.settings.MemoryMB = 4096
	req.BuildID = "ccb_0003"
	if _, err := b.Build(context.Background(), req, nil); err != nil {
		t.Fatal(err)
	}
	engine.mu.Lock()
	defer engine.mu.Unlock()
	if engine.removes != 1 {
		t.Fatalf("changed settings did not recreate the container (removes %d)", engine.removes)
	}
}

func TestCYDBuilderCancelStopsTheContainer(t *testing.T) {
	engine, cfg := newFakeCYDEngine(t)
	engine.blockBuild = make(chan struct{})
	defer close(engine.blockBuild)
	b := newTestCYDBuilder(cfg, 0)
	ctx, cancel := context.WithCancel(context.Background())
	started := make(chan struct{})
	var once sync.Once
	errCh := make(chan error, 1)
	go func() {
		_, err := b.Build(ctx, cydcreator.BuildRequest{BuildID: "ccb_0009", Env: "esp32-2432s028r", CacheKey: "k", Tree: stubTree(0, 0)},
			func(string) { once.Do(func() { close(started) }) })
		errCh <- err
	}()
	<-started
	cancel()
	select {
	case err := <-errCh:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("Build after cancel = %v", err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("Build did not return after cancel")
	}
	engine.mu.Lock()
	defer engine.mu.Unlock()
	if engine.stops != 1 || engine.running {
		t.Fatalf("stops = %d running = %v, want the container stopped", engine.stops, engine.running)
	}
	if engine.removes != 1 || engine.container != nil {
		t.Fatalf("removes = %d, want the cancelled build's container removed", engine.removes)
	}
}

func TestCYDBuilderIdleStop(t *testing.T) {
	engine, cfg := newFakeCYDEngine(t)
	b := newTestCYDBuilder(cfg, 50*time.Millisecond)
	if _, err := b.Build(context.Background(), cydcreator.BuildRequest{BuildID: "ccb_0001", Env: "esp32-2432s028r", CacheKey: "k", Tree: stubTree(0, 0)}, nil); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		engine.mu.Lock()
		stopped := engine.stops == 1 && !engine.running
		if engine.removes != 0 {
			engine.mu.Unlock()
			t.Fatalf("the idle stop removed the container (removes %d), want a stop only", engine.removes)
		}
		engine.mu.Unlock()
		if stopped {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("the idle container was not stopped")
}

func TestCYDBuilderStatusPullAndMissingImage(t *testing.T) {
	engine, cfg := newFakeCYDEngine(t)
	engine.imagePresent = false
	engine.pullBody = `{"status":"Pulling from antibyte/aurago-cyd-builder","id":"1"}` + "\n" +
		`{"status":"Downloading","id":"l1","progressDetail":{"current":50,"total":100}}` + "\n" +
		`{"status":"Download complete","id":"l1"}` + "\n"
	b := newTestCYDBuilder(cfg, 0)
	st := b.Status(context.Background())
	if !st.DockerAvailable || st.Present || st.Image != "ghcr.io/antibyte/aurago-cyd-builder:1" {
		t.Fatalf("status before pull = %+v", st)
	}
	_, err := b.Build(context.Background(), cydcreator.BuildRequest{BuildID: "ccb_0001", Env: "esp32-2432s028r", CacheKey: "k", Tree: stubTree(0, 0)}, nil)
	if cydcreator.ErrorCode(err) != cydcreator.CodeImageMissing {
		t.Fatalf("build without image = %v, want CYD_IMAGE_MISSING", err)
	}
	if err := b.Pull(context.Background()); err != nil {
		t.Fatalf("Pull: %v", err)
	}
	if st := b.Status(context.Background()); !st.Present || st.Pulling || st.Error != "" {
		t.Fatalf("status after pull = %+v", st)
	}
	engine.mu.Lock()
	engine.pullBody = `{"errorDetail":{"message":"unauthorized"},"error":"unauthorized"}` + "\n"
	engine.mu.Unlock()
	err = b.Pull(context.Background())
	if cydcreator.ErrorCode(err) != cydcreator.CodeImagePullFailed || !strings.Contains(err.Error(), "unauthorized") {
		t.Fatalf("failed pull = %v", err)
	}
	if st := b.Status(context.Background()); !strings.Contains(st.Error, "unauthorized") {
		t.Fatalf("status keeps the pull error: %+v", st)
	}
	tools.ConfigureRuntimePermissions(tools.RuntimePermissions{DockerEnabled: true, DockerReadOnly: true})
	if st := b.Status(context.Background()); st.DockerAvailable || !strings.Contains(st.Error, "read-only") {
		t.Fatalf("status under docker.read_only = %+v", st)
	}
}

func TestCYDPullProgressFraction(t *testing.T) {
	var got []float64
	p := &cydPullProgress{report: func(f float64) { got = append(got, f) }}
	_, _ = p.Write([]byte(`{"status":"Downloading","id":"a","progressDetail":{"current":25,"total":100}}` + "\n" +
		`{"status":"Downloading","id":"b","progressDetail":{"current":0,"total":100}}` + "\n{\"status\":\"Download"))
	_, _ = p.Write([]byte(` complete","id":"a"}` + "\n"))
	if len(got) != 3 || got[0] != 0.25 || got[1] != 0.125 || got[2] != 0.5 {
		t.Fatalf("progress = %v, want [0.25 0.125 0.5]", got)
	}
}

func TestCYDBuilderAddr2Line(t *testing.T) {
	_, cfg := newFakeCYDEngine(t)
	b := newTestCYDBuilder(cfg, 0)
	frames, err := b.Addr2Line(context.Background(), cydcreator.Addr2LineRequest{BuildID: "ccb_1", Chip: "esp32s3", ELF: []byte("ELF"),
		Addresses: []string{"0x400d1234", "0x400d5678"}})
	if err != nil || len(frames) != 2 || frames[1].Address != "0x400d5678" || frames[0].Line != 42 {
		t.Fatalf("frames = %+v, %v", frames, err)
	}
	if _, err := b.Addr2Line(context.Background(), cydcreator.Addr2LineRequest{Chip: "avr"}); cydcreator.ErrorCode(err) != cydcreator.CodeBadRequest {
		t.Fatalf("unknown chip = %v", err)
	}
}

func TestDemuxDockerStreamHandlesRawOutput(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if err := demuxDockerStream(strings.NewReader("plain tty output\n"), &stdout, &stderr); err != nil || stdout.String() != "plain tty output\n" {
		t.Fatalf("raw stream = %q, %v", stdout.String(), err)
	}
	stdout.Reset()
	stream := append(frame(1, "out"), frame(2, "err")...)
	if err := demuxDockerStream(bytes.NewReader(stream), &stdout, &stderr); err != nil || stdout.String() != "out" || stderr.String() != "err" {
		t.Fatalf("multiplexed = %q/%q, %v", stdout.String(), stderr.String(), err)
	}
}

func TestCYDBuilderExtractArtifactsKeepsFonts(t *testing.T) {
	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)
	for name, data := range map[string]string{
		"out/merged.bin": "m", "out/result.json": `{"ok":true}`, "out/fonts/fonts.json": "{}",
		"out/fonts/ui_font_montserrat_14.c": "// font",
	} {
		_ = tw.WriteHeader(&tar.Header{Name: name, Typeflag: tar.TypeReg, Mode: 0o644, Size: int64(len(data))})
		_, _ = tw.Write([]byte(data))
	}
	_ = tw.Close()
	files, result, err := extractArtifacts(buf.Bytes())
	if err != nil || string(result) != `{"ok":true}` {
		t.Fatalf("extract = %v, %q", err, result)
	}
	keys := make([]string, 0, len(files))
	for k := range files {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	if strings.Join(keys, ",") != "fonts/fonts.json,fonts/ui_font_montserrat_14.c,merged.bin" {
		t.Fatalf("files = %v", keys)
	}
}

func TestBuildTarRejectsEscapingPaths(t *testing.T) {
	for _, bad := range []string{"../x", "/etc/passwd", "a/../../b", ".", ""} {
		if _, err := buildTar("ccb_1", map[string][]byte{bad: nil}); err == nil {
			t.Errorf("path %q was accepted", bad)
		}
	}
}

// artefactEntry is one entry of a test GET /archive tar. size > 0 writes only
// the header with that size and ends the archive there (the reader must refuse
// it before it reads any content).
type artefactEntry struct {
	name, data, link string
	typ              byte
	size             int64
}

func artefactTar(t *testing.T, entries ...artefactEntry) []byte {
	t.Helper()
	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)
	for _, e := range entries {
		typ := e.typ
		if typ == 0 {
			typ = tar.TypeReg
		}
		hdr := &tar.Header{Name: e.name, Typeflag: typ, Mode: 0o644, Linkname: e.link}
		if typ == tar.TypeReg {
			hdr.Size = int64(len(e.data))
		}
		if e.size > 0 {
			hdr.Size = e.size
		}
		if err := tw.WriteHeader(hdr); err != nil {
			t.Fatalf("tar header %q: %v", e.name, err)
		}
		if e.size > 0 {
			return buf.Bytes()
		}
		if _, err := tw.Write([]byte(e.data)); err != nil {
			t.Fatalf("tar data %q: %v", e.name, err)
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func sortedKeys(files map[string][]byte) []string {
	keys := make([]string, 0, len(files))
	for k := range files {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	return keys
}

func TestExtractArtifactsAcceptsTheEngineLayout(t *testing.T) {
	files, result, err := extractArtifacts(artefactTar(t,
		artefactEntry{name: "out/", typ: tar.TypeDir},
		artefactEntry{name: "out/merged.bin", data: "m"},
		artefactEntry{name: "out/firmware.bin", data: "f"},
		artefactEntry{name: "out/firmware.elf", data: "e"},
		artefactEntry{name: "out/result.json", data: `{"ok":true}`},
		artefactEntry{name: "out/build.log", data: "log"},
		artefactEntry{name: "out/fonts/", typ: tar.TypeDir},
		artefactEntry{name: "out/fonts/fonts.json", data: "{}"},
		artefactEntry{name: "out/fonts/ui_font_montserrat_14.c", data: "// font"},
	))
	if err != nil || string(result) != `{"ok":true}` {
		t.Fatalf("extract = %v, %q", err, result)
	}
	if got := strings.Join(sortedKeys(files), ","); got != "firmware.bin,firmware.elf,fonts/fonts.json,fonts/ui_font_montserrat_14.c,merged.bin" {
		t.Fatalf("files = %s (build.log is not kept: the streamed log is authoritative)", got)
	}
}

func TestExtractArtifactsRejectsUnsafeArchives(t *testing.T) {
	cases := map[string][]artefactEntry{
		"symlink":            {{name: "out/firmware.bin", typ: tar.TypeSymlink, link: "/etc/passwd"}},
		"hardlink":           {{name: "out/merged.bin", data: "m"}, {name: "out/firmware.bin", typ: tar.TypeLink, link: "out/merged.bin"}},
		"char device":        {{name: "out/firmware.elf", typ: tar.TypeChar}},
		"block device":       {{name: "out/firmware.elf", typ: tar.TypeBlock}},
		"fifo":               {{name: "out/result.json", typ: tar.TypeFifo}},
		"absolute":           {{name: "/out/merged.bin", data: "m"}},
		"dotdot inside":      {{name: "out/../merged.bin", data: "m"}},
		"leading dotdot":     {{name: "../out/merged.bin", data: "m"}},
		"dot segment":        {{name: "out/./merged.bin", data: "m"}},
		"double slash":       {{name: "out//merged.bin", data: "m"}},
		"backslash":          {{name: "out\\merged.bin", data: "m"}},
		"duplicate file":     {{name: "out/merged.bin", data: "a"}, {name: "out/merged.bin", data: "b"}},
		"duplicate dir":      {{name: "out/", typ: tar.TypeDir}, {name: "out/", typ: tar.TypeDir}},
		"nested binary":      {{name: "out/sub/firmware.bin", data: "f"}},
		"nested dir":         {{name: "out/sub/", typ: tar.TypeDir}},
		"nested font dir":    {{name: "out/fonts/x/", typ: tar.TypeDir}},
		"other root":         {{name: "a/fonts/ui_font_montserrat_14.c", data: "f"}},
		"no root":            {{name: "merged.bin", data: "m"}},
		"unknown file":       {{name: "out/other.txt", data: "x"}},
		"unknown font file":  {{name: "out/fonts/evil.sh", data: "rm -rf /"}},
		"font outside fonts": {{name: "out/ui_font_montserrat_14.c", data: "f"}},
		"root as file":       {{name: "out", data: "x"}},
	}
	for name, entries := range cases {
		files, result, err := extractArtifacts(artefactTar(t, entries...))
		if err == nil {
			t.Errorf("%s: accepted (files %v, result %q)", name, sortedKeys(files), result)
		}
	}
}

func TestExtractArtifactsCapsEveryFileBeforeReadingIt(t *testing.T) {
	cases := map[string]artefactEntry{
		"merged.bin":   {name: "out/merged.bin", size: cydBuilderMaxImageBytes + 1},
		"firmware.bin": {name: "out/firmware.bin", size: cydBuilderMaxImageBytes + 1},
		"firmware.elf": {name: "out/firmware.elf", size: cydcreator.MaxELFArtifactBytes + 1},
		"font":         {name: "out/fonts/ui_font_montserrat_14.c", size: cydcreator.MaxFontArtifactBytes + 1},
		"fonts.json":   {name: "out/fonts/fonts.json", size: cydcreator.MaxFontArtifactBytes + 1},
		"result.json":  {name: "out/result.json", size: cydBuilderMaxResultJSON + 1},
		"build.log":    {name: "out/build.log", size: cydcreator.MaxBuildLogBytes + 1},
	}
	for name, entry := range cases {
		// The archive ends after the header: only a check of the declared
		// size (not a read) can produce the limit error.
		if _, _, err := extractArtifacts(artefactTar(t, entry)); err == nil || !strings.Contains(err.Error(), "limit") {
			t.Errorf("%s above its cap = %v, want a limit error", name, err)
		}
	}
	limits := cydDefaultArtifactLimits
	limits.total = 10
	two := artefactTar(t, artefactEntry{name: "out/merged.bin", data: "123456"}, artefactEntry{name: "out/firmware.bin", data: "123456"})
	if _, _, err := extractArtifactsWithin(two, limits); err == nil || !strings.Contains(err.Error(), "limit") {
		t.Errorf("total above its cap = %v, want a limit error", err)
	}
	limits = cydDefaultArtifactLimits
	limits.entries = 2
	three := artefactTar(t, artefactEntry{name: "out/", typ: tar.TypeDir}, artefactEntry{name: "out/merged.bin", data: "m"},
		artefactEntry{name: "out/firmware.bin", data: "f"})
	if _, _, err := extractArtifactsWithin(three, limits); err == nil || !strings.Contains(err.Error(), "limit") {
		t.Errorf("entries above the cap = %v, want a limit error", err)
	}
}

func TestCYDBuilderArtifactLimitsFollowTheService(t *testing.T) {
	l := cydDefaultArtifactLimits
	if l.elf != cydcreator.MaxELFArtifactBytes || l.font != cydcreator.MaxFontArtifactBytes || l.log != cydcreator.MaxBuildLogBytes ||
		l.total != cydBuilderMaxArtifacts || l.image != cydBuilderMaxImageBytes || l.result != cydBuilderMaxResultJSON {
		t.Fatalf("limits = %+v", l)
	}
	boards, err := cydcreator.Boards()
	if err != nil {
		t.Fatal(err)
	}
	for _, board := range boards {
		if int64(board.FlashMB)<<20 > l.image {
			t.Errorf("board %s has %d MB flash, above the image cap %d", board.ID, board.FlashMB, l.image)
		}
	}
}

type repeatByteReader byte

func (r repeatByteReader) Read(p []byte) (int, error) {
	for i := range p {
		p[i] = byte(r)
	}
	return len(p), nil
}

func TestDemuxDockerStreamRejectsBadFrames(t *testing.T) {
	// A complete frame one byte above the cap: only the size check refuses it.
	oversized := frame(1, strings.Repeat("x", cydBuilderMaxFrame+1))
	padded := frame(1, "x")
	padded[2] = 1
	cases := map[string][]byte{
		"stdin first":       frame(0, "in"),
		"stream 3 first":    frame(3, "x"),
		"stdin later":       append(frame(1, "ok"), frame(0, "in")...),
		"stream 7 later":    append(frame(2, "ok"), frame(7, "x")...),
		"bad padding later": append(frame(1, "ok"), padded...),
		"oversized first":   oversized,
		"oversized later":   append(frame(1, "ok"), oversized...),
		"truncated frame":   frame(1, "abc")[:10],
		"truncated header":  append(frame(1, "ok"), 1, 0, 0),
	}
	for name, stream := range cases {
		if err := demuxDockerStream(bytes.NewReader(stream), io.Discard, io.Discard); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
	var stdout bytes.Buffer
	if err := demuxDockerStream(strings.NewReader("hi\n"), &stdout, io.Discard); err != nil || stdout.String() != "hi\n" {
		t.Fatalf("short raw output = %q, %v", stdout.String(), err)
	}
	var n countingWriter
	if err := demuxDockerStream(io.LimitReader(repeatByteReader('a'), cydBuilderMaxRawOutput), &n, io.Discard); err != nil || n != cydBuilderMaxRawOutput {
		t.Fatalf("raw output at the cap = %d bytes, %v", n, err)
	}
	if err := demuxDockerStream(io.LimitReader(repeatByteReader('a'), cydBuilderMaxRawOutput+1), io.Discard, io.Discard); err == nil {
		t.Fatal("raw output above the cap was accepted")
	}
}

type countingWriter int64

func (c *countingWriter) Write(p []byte) (int, error) {
	*c += countingWriter(len(p))
	return len(p), nil
}

func TestCYDLineWriterBoundsLongLines(t *testing.T) {
	var lines []string
	w := &cydLineWriter{emit: func(s string) { lines = append(lines, s) }}
	_, _ = w.Write(bytes.Repeat([]byte("a"), 3*cydBuilderMaxLogLine+5))
	if len(w.partial) > cydBuilderMaxLogLine {
		t.Fatalf("pending line holds %d bytes", len(w.partial))
	}
	_, _ = w.Write([]byte("b\r\nnext"))
	w.flush()
	want := []string{strings.Repeat("a", cydBuilderMaxLogLine), strings.Repeat("a", cydBuilderMaxLogLine),
		strings.Repeat("a", cydBuilderMaxLogLine), "aaaaab", "next"}
	if !slices.Equal(lines, want) {
		t.Fatalf("lines = %d %q...", len(lines), lines[len(lines)-1])
	}
}

func TestCYDBuilderTimeoutRemovesTheContainer(t *testing.T) {
	engine, cfg := newFakeCYDEngine(t)
	engine.blockBuild = make(chan struct{})
	defer close(engine.blockBuild)
	b := newTestCYDBuilder(cfg, 0)
	_, err := b.Build(context.Background(), cydcreator.BuildRequest{BuildID: "ccb_0010", Env: "esp32-2432s028r", CacheKey: "k",
		Tree: stubTree(0, 0), Timeout: 300 * time.Millisecond}, nil)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("Build after its timeout = %v", err)
	}
	engine.mu.Lock()
	defer engine.mu.Unlock()
	if engine.stops != 1 || engine.removes != 1 || engine.container != nil {
		t.Fatalf("stops %d removes %d, want the container stopped and removed", engine.stops, engine.removes)
	}
}

func TestCYDBuilderBrokenStreamRemovesTheContainer(t *testing.T) {
	engine, cfg := newFakeCYDEngine(t)
	b := newTestCYDBuilder(cfg, 0)
	tree := stubTree(0, 0)
	tree["stub.json"] = []byte(`{"exit":0,"badframe":true}`)
	_, err := b.Build(context.Background(), cydcreator.BuildRequest{BuildID: "ccb_0011", Env: "esp32-2432s028r", CacheKey: "k", Tree: tree}, nil)
	if err == nil || errors.Is(err, context.Canceled) {
		t.Fatalf("Build with a stdin frame = %v, want a stream error", err)
	}
	engine.mu.Lock()
	defer engine.mu.Unlock()
	if engine.removes != 1 || engine.container != nil {
		t.Fatalf("removes %d: cyd-build may still run, the container must go", engine.removes)
	}
}

func TestCYDBuilderMissingContainerIsNotAnEmptyResult(t *testing.T) {
	engine, cfg := newFakeCYDEngine(t)
	engine.archiveGone = true
	b := newTestCYDBuilder(cfg, 0)
	out, err := b.Build(context.Background(), cydcreator.BuildRequest{BuildID: "ccb_0012", Env: "esp32-2432s028r", CacheKey: "k", Tree: stubTree(0, 0)}, nil)
	if err == nil {
		t.Fatalf("Build whose container vanished = %+v, want an error", out)
	}
}

func TestCYDBuilderFailedStartRemovesTheContainer(t *testing.T) {
	engine, cfg := newFakeCYDEngine(t)
	engine.failStart = true
	b := newTestCYDBuilder(cfg, 0)
	if _, err := b.Build(context.Background(), cydcreator.BuildRequest{BuildID: "ccb_0013", Env: "esp32-2432s028r", CacheKey: "k", Tree: stubTree(0, 0)}, nil); err == nil {
		t.Fatal("Build with a container that cannot start succeeded")
	}
	engine.mu.Lock()
	defer engine.mu.Unlock()
	if engine.removes != 1 || engine.container != nil {
		t.Fatalf("removes %d, want the unstartable container removed", engine.removes)
	}
}

func TestCYDBuilderCloseRemovesTheContainer(t *testing.T) {
	engine, cfg := newFakeCYDEngine(t)
	b := newTestCYDBuilder(cfg, time.Hour)
	if _, err := b.Build(context.Background(), cydcreator.BuildRequest{BuildID: "ccb_0014", Env: "esp32-2432s028r", CacheKey: "k", Tree: stubTree(0, 0)}, nil); err != nil {
		t.Fatal(err)
	}
	b.Close()
	engine.mu.Lock()
	removed := engine.removes == 1 && engine.container == nil
	engine.mu.Unlock()
	if !removed {
		t.Fatal("Close left the builder container behind")
	}
	_, err := b.Build(context.Background(), cydcreator.BuildRequest{BuildID: "ccb_0015", Env: "esp32-2432s028r", CacheKey: "k", Tree: stubTree(0, 0)}, nil)
	if cydcreator.ErrorCode(err) != cydcreator.CodeDockerUnavailable {
		t.Fatalf("Build after Close = %v", err)
	}
}

func TestCYDBuilderErrorsStayGeneric(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := ln.Addr().String()
	_ = ln.Close()
	tools.ConfigureRuntimePermissions(tools.RuntimePermissions{DockerEnabled: true})
	t.Cleanup(tools.ClearRuntimePermissionsForTest)
	b := newTestCYDBuilder(tools.DockerConfig{Host: "tcp://" + addr}, 0)
	if st := b.Status(context.Background()); st.DockerAvailable || st.Error != cydDockerUnreachable {
		t.Fatalf("status of an unreachable engine = %+v", st)
	}
	_, err = b.Build(context.Background(), cydcreator.BuildRequest{BuildID: "ccb_0016", Env: "esp32-2432s028r", CacheKey: "k", Tree: stubTree(0, 0)}, nil)
	if cydcreator.ErrorCode(err) != cydcreator.CodeDockerUnavailable || strings.Contains(err.Error(), "127.0.0.1") {
		t.Fatalf("build on an unreachable engine = %v", err)
	}

	engine, cfg := newFakeCYDEngine(t)
	engine.imageStatus = http.StatusForbidden
	b = newTestCYDBuilder(cfg, 0)
	st := b.Status(context.Background())
	if st.DockerAvailable || !strings.Contains(st.Error, "HTTP 403") || strings.Contains(st.Error, "docker.sock") {
		t.Fatalf("status with a refusing engine = %+v", st)
	}

	engine.mu.Lock()
	engine.imageStatus = 0
	engine.pullBody = `{"error":"Get \"https://ghcr.io/v2/\": dial tcp: lookup ghcr.io on 127.0.0.11:53: no such host"}` + "\n"
	engine.mu.Unlock()
	err = b.Pull(context.Background())
	if cydcreator.ErrorCode(err) != cydcreator.CodeImagePullFailed || !strings.Contains(err.Error(), "not reachable") || strings.Contains(err.Error(), "127.0.0.11") {
		t.Fatalf("pull with a resolver failure = %v", err)
	}
	if st := b.Status(context.Background()); strings.Contains(st.Error, "127.0.0.11") || !strings.Contains(st.Error, "not reachable") {
		t.Fatalf("status after the resolver failure = %+v", st)
	}
}

func TestCYDBuilderDefaultsMissingLimits(t *testing.T) {
	b := newCYDDockerBuilder(tools.DockerConfig{}, cydBuilderSettings{Image: "img:1"}, nil)
	host := b.containerSpec()["HostConfig"].(map[string]any)
	if host["Memory"] != int64(2048<<20) || host["MemorySwap"] != int64(2048<<20) || host["NanoCpus"] != int64(2e9) {
		t.Fatalf("host config without settings = %v", host)
	}
}

func sortedAnyKeys(m map[string]any) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	return keys
}

// presetContainer puts an existing aurago-cyd-builder container into the
// fake engine, as left behind by an earlier process or someone else.
func (e *fakeCYDEngine) presetContainer(labels map[string]string, running bool) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.container = map[string]any{"Labels": labels}
	e.labels = labels
	e.running = running
}

func TestCYDBuilderDiscardsARunningContainerOfAnEarlierProcess(t *testing.T) {
	engine, cfg := newFakeCYDEngine(t)
	b := newTestCYDBuilder(cfg, 0)
	engine.presetContainer(dockerutil.ManagedLabels("cyd-creator", "cyd-creator", "builder", b.fingerprint()), true)
	req := cydcreator.BuildRequest{BuildID: "ccb_0020", Env: "esp32-2432s028r", CacheKey: "k", Tree: stubTree(0, 0)}
	if _, err := b.Build(context.Background(), req, nil); err != nil {
		t.Fatal(err)
	}
	engine.mu.Lock()
	if engine.stops != 1 || engine.removes != 1 || engine.starts != 1 || engine.container == nil {
		engine.mu.Unlock()
		t.Fatalf("stops %d removes %d starts %d: the first use must discard the running container and create a fresh one",
			engine.stops, engine.removes, engine.starts)
	}
	engine.mu.Unlock()
	req.BuildID = "ccb_0021"
	if _, err := b.Build(context.Background(), req, nil); err != nil {
		t.Fatal(err)
	}
	engine.mu.Lock()
	defer engine.mu.Unlock()
	if engine.removes != 1 || engine.starts != 1 {
		t.Fatalf("removes %d starts %d: this process's own container must be reused", engine.removes, engine.starts)
	}
}

func TestCYDBuilderLeavesAForeignContainerAlone(t *testing.T) {
	engine, cfg := newFakeCYDEngine(t)
	engine.presetContainer(map[string]string{"com.example.app": "mine"}, true)
	b := newTestCYDBuilder(cfg, 0)
	_, err := b.Build(context.Background(), cydcreator.BuildRequest{BuildID: "ccb_0022", Env: "esp32-2432s028r", CacheKey: "k", Tree: stubTree(0, 0)}, nil)
	if cydcreator.ErrorCode(err) != cydcreator.CodeDockerUnavailable || strings.Contains(err.Error(), "ctr1") {
		t.Fatalf("build next to a foreign container = %v", err)
	}
	b.Close()
	engine.mu.Lock()
	defer engine.mu.Unlock()
	if engine.removes != 0 || engine.stops != 0 || !engine.running || len(engine.execOrder) != 0 {
		t.Fatalf("removes %d stops %d running %v execs %v: a foreign container must not be touched",
			engine.removes, engine.stops, engine.running, engine.execOrder)
	}
}

func TestCYDBuilderRearmsTheIdleStopAfterAFailedAcquire(t *testing.T) {
	engine, cfg := newFakeCYDEngine(t)
	b := newTestCYDBuilder(cfg, time.Hour)
	defer b.Close()
	req := cydcreator.BuildRequest{BuildID: "ccb_0023", Env: "esp32-2432s028r", CacheKey: "k", Tree: stubTree(0, 0)}
	if _, err := b.Build(context.Background(), req, nil); err != nil {
		t.Fatal(err)
	}
	engine.mu.Lock()
	engine.inspectFail = true
	engine.mu.Unlock()
	req.BuildID = "ccb_0024"
	if _, err := b.Build(context.Background(), req, nil); err == nil {
		t.Fatal("Build with a failing container inspect succeeded")
	}
	b.mu.Lock()
	armed, busy := b.idleTimer != nil, b.busy
	b.mu.Unlock()
	if !armed || busy != 0 {
		t.Fatalf("idle timer armed %v busy %d after a failed acquire, want the idle stop re-armed", armed, busy)
	}
}

func TestCYDBuilderReturnsTheContextErrorDuringAcquireAndUpload(t *testing.T) {
	engine, cfg := newFakeCYDEngine(t)
	b := newTestCYDBuilder(cfg, 0)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := b.Build(ctx, cydcreator.BuildRequest{BuildID: "ccb_0025", Env: "esp32-2432s028r", CacheKey: "k", Tree: stubTree(0, 0)}, nil)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Build with an ended context during acquire = %v", err)
	}

	engine.mu.Lock()
	engine.blockUpload, engine.uploadSeen = make(chan struct{}), make(chan struct{})
	engine.mu.Unlock()
	defer close(engine.blockUpload)
	ctx, cancel = context.WithCancel(context.Background())
	defer cancel()
	errCh := make(chan error, 1)
	go func() {
		_, err := b.Build(ctx, cydcreator.BuildRequest{BuildID: "ccb_0026", Env: "esp32-2432s028r", CacheKey: "k", Tree: stubTree(0, 0)}, nil)
		errCh <- err
	}()
	select {
	case <-engine.uploadSeen:
	case <-time.After(10 * time.Second):
		t.Fatal("the upload never started")
	}
	cancel()
	select {
	case err := <-errCh:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("Build cancelled during the upload = %v", err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("Build did not return after cancel during the upload")
	}
	engine.mu.Lock()
	defer engine.mu.Unlock()
	for _, cmd := range engine.execOrder {
		if cmd[0] == "cyd-build" {
			t.Fatalf("cyd-build ran after the upload was cancelled: %v", engine.execOrder)
		}
	}
}

func TestCYDBuilderValidatesContainerArguments(t *testing.T) {
	engine, cfg := newFakeCYDEngine(t)
	b := newTestCYDBuilder(cfg, 0)
	for _, req := range []cydcreator.BuildRequest{
		{BuildID: "ccb_0027", Env: "--clean", CacheKey: "k"},
		{BuildID: "ccb_0027", Env: "e", CacheKey: "k"},
		{BuildID: "ccb_0027", Env: "ESP32", CacheKey: "k"},
		{BuildID: "ccb_0027", Env: "esp32-2432s028r", CacheKey: "../k"},
		{BuildID: "ccb_0027", Env: "esp32-2432s028r", CacheKey: "-k"},
		{BuildID: "ccb_0027", Env: "esp32-2432s028r", CacheKey: strings.Repeat("k", 65)},
		{BuildID: "../x", Env: "esp32-2432s028r", CacheKey: "k"},
	} {
		if _, err := b.Build(context.Background(), req, nil); err == nil {
			t.Errorf("Build(%q, %q, %q) was accepted", req.BuildID, req.Env, req.CacheKey)
		}
	}
	for _, addrs := range [][]string{
		nil,
		{"0x123456789"},
		{"400d1234"},
		{"0x400d1234; rm -rf /"},
		{"0x400d1234", "--elf=/etc/passwd"},
		slices.Repeat([]string{"0x400d1234"}, 65),
	} {
		_, err := b.Addr2Line(context.Background(), cydcreator.Addr2LineRequest{BuildID: "ccb_1", Chip: "esp32", ELF: []byte("ELF"), Addresses: addrs})
		if cydcreator.ErrorCode(err) != cydcreator.CodeBadRequest {
			t.Errorf("Addr2Line(%q) = %v, want CYD_BAD_REQUEST", addrs, err)
		}
	}
	engine.mu.Lock()
	defer engine.mu.Unlock()
	if engine.container != nil || len(engine.execOrder) != 0 {
		t.Fatalf("invalid arguments reached Docker: container %v execs %v", engine.container != nil, engine.execOrder)
	}
}

func TestCYDBuildLogEndsWithAMarkerAtTheCap(t *testing.T) {
	var l cydBuildLog
	line := strings.Repeat("é", 1000) // 2000 bytes, multi-byte runes
	for i := 0; i < cydcreator.MaxBuildLogBytes/len(line)+10; i++ {
		l.add(line)
	}
	text := l.buf.String()
	if len(text) > cydcreator.MaxBuildLogBytes || !strings.HasSuffix(text, cydBuildLogTruncated) || !utf8.ValidString(text) || !l.truncated {
		t.Fatalf("log of %d bytes, marker %v, valid %v", len(text), strings.HasSuffix(text, cydBuildLogTruncated), utf8.ValidString(text))
	}
	l.add("late line")
	if l.buf.String() != text {
		t.Fatal("a truncated log still grows")
	}
}

func TestCYDLineWriterKeepsRunesWhole(t *testing.T) {
	var lines []string
	w := &cydLineWriter{emit: func(s string) { lines = append(lines, s) }}
	_, _ = w.Write([]byte(strings.Repeat("a", cydBuilderMaxLogLine-1) + "é" + "b\n"))
	if len(lines) != 2 || lines[0] != strings.Repeat("a", cydBuilderMaxLogLine-1) || lines[1] != "éb" {
		t.Fatalf("lines = %d, last %q", len(lines), lines[len(lines)-1])
	}
	lines = nil
	_, _ = w.Write([]byte(strings.Repeat("€", 3000) + "\n")) // 9000 bytes of 3-byte runes
	total := 0
	for _, s := range lines {
		if !utf8.ValidString(s) || len(s) > cydBuilderMaxLogLine {
			t.Fatalf("chunk of %d bytes, valid %v", len(s), utf8.ValidString(s))
		}
		total += len(s)
	}
	if total != 9000 {
		t.Fatalf("chunks hold %d bytes, want 9000", total)
	}
	lines = nil
	_, _ = w.Write(append(bytes.Repeat([]byte{0x80}, 5000), '\n')) // no UTF-8 at all: cut at the limit
	if len(lines) != 2 || len(lines[0]) != cydBuilderMaxLogLine || len(lines[1]) != 5000-cydBuilderMaxLogLine {
		t.Fatalf("invalid bytes split into %d lines", len(lines))
	}
}

func TestCYDPullReasonStaysGeneric(t *testing.T) {
	cases := []struct {
		status    int
		text      string
		want, not string
	}{
		{0, "unauthorized: authentication required", "unauthorized", ""},
		{404, "pull access denied for x, repository does not exist or may require 'docker login': denied: requested access to the resource is denied", "unauthorized", ""},
		{500, "open /var/lib/docker/tmp/GetImageBlob123: permission denied", "permission denied", "unauthorized"},
		{0, "manifest unknown: manifest unknown", "does not exist", ""},
		{403, "<html><body><h1>403 Forbidden</h1></body></html>", "socket proxy", ""},
		{0, "write /var/lib/docker/overlay2/x: no space left on device", "disk space", ""},
	}
	for _, c := range cases {
		got := cydPullReason(c.status, c.text)
		if !strings.Contains(got, c.want) || (c.not != "" && strings.Contains(got, c.not)) || strings.Contains(got, "/var/lib") {
			t.Errorf("cydPullReason(%d, %q) = %q, want %q", c.status, c.text, got, c.want)
		}
	}
}
