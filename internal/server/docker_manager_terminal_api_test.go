package server

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"aurago/internal/dockerwatch"

	"github.com/gorilla/websocket"
)

// dmTerminalProbe is how a terminal request ended: the HTTP status and the JSON
// answer of a refusal, or a connected WebSocket.
type dmTerminalProbe struct {
	status int
	body   map[string]any
	conn   *websocket.Conn
}

func (ls *dmLiveServer) wsURL(path string) string {
	return "ws" + strings.TrimPrefix(ls.url, "http") + "/api/docker-manager" + path
}

func dmProbeOf(resp *http.Response) dmTerminalProbe {
	raw, _ := io.ReadAll(resp.Body)
	var body map[string]any
	_ = json.Unmarshal(raw, &body)
	return dmTerminalProbe{status: resp.StatusCode, body: body}
}

// handshake asks for the terminal the way the browser does.
func (ls *dmLiveServer) handshake(path string, headers http.Header) dmTerminalProbe {
	ls.t.Helper()
	conn, resp, err := websocket.DefaultDialer.Dial(ls.wsURL(path), headers)
	if err == nil {
		ls.t.Cleanup(func() { _ = conn.Close() })
		return dmTerminalProbe{status: resp.StatusCode, conn: conn}
	}
	if resp == nil {
		ls.t.Fatalf("handshake %s: %v", path, err)
	}
	defer resp.Body.Close()
	return dmProbeOf(resp)
}

// diagnose asks for the same URL without the upgrade headers, as the desktop app
// does after a refused handshake to read the reason.
func (ls *dmLiveServer) diagnose(path string, headers http.Header) dmTerminalProbe {
	ls.t.Helper()
	req, err := http.NewRequest(http.MethodGet, ls.url+"/api/docker-manager"+path, nil)
	if err != nil {
		ls.t.Fatal(err)
	}
	for key, values := range headers {
		req.Header[key] = values
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		ls.t.Fatalf("GET %s: %v", path, err)
	}
	defer resp.Body.Close()
	return dmProbeOf(resp)
}

// dmTerminalSlots counts the stream slots that are free on the host.
func dmTerminalSlots(t *testing.T, hub *dockerwatch.Hub, host string) int {
	t.Helper()
	var releases []func()
	for {
		release, err := hub.Acquire(host)
		if err != nil {
			break
		}
		releases = append(releases, release)
	}
	for _, release := range releases {
		release()
	}
	return len(releases)
}

func TestDockerManagerTerminalExplainsEveryRefusalAsJSONBeforeTheUpgrade(t *testing.T) {
	ls := newDMLiveServer(t, "[]")
	remote := newDMLiveEngine(t, dmContainersJSON(t,
		dmContainerJSON('w', "web", "sha256:img1", "running", "bridge", map[string]string{"aurago.managed": "ruview"}, nil)))
	nas := ls.addTCPHost("NAS", remote)

	hold := func(t *testing.T) {
		for i := 0; i < dockerwatch.MaxStreamsPerHost; i++ {
			release, err := ls.s.DockerWatch.Acquire("local")
			if err != nil {
				t.Fatalf("Acquire %d: %v", i, err)
			}
			t.Cleanup(release)
		}
	}
	protectLocal := func(t *testing.T) {
		t.Cleanup(replaceContainerProtection(containerProtection{Owner: "ruview"}))
	}
	setReadOnly := func(on bool) {
		ls.s.CfgMu.Lock()
		ls.s.Cfg.Docker.ReadOnly = on
		ls.s.CfgMu.Unlock()
	}
	readOnly := func(t *testing.T) {
		setReadOnly(true)
		t.Cleanup(func() { setReadOnly(false) })
	}

	cross := http.Header{"Origin": []string{"http://evil.example"}}
	for _, tc := range []struct {
		name    string
		path    string
		headers http.Header
		setup   func(t *testing.T)
		status  int
		code    string
	}{
		{"read-only", "/hosts/local/containers/web/terminal", nil, readOnly, http.StatusForbidden, "read_only"},
		{"cross-origin", "/hosts/local/containers/web/terminal", cross, nil, http.StatusForbidden, "forbidden_origin"},
		{"unknown host", "/hosts/nowhere/containers/web/terminal", nil, nil, http.StatusNotFound, "host_not_found"},
		{"unknown container", "/hosts/local/containers/ghost/terminal", nil, nil, http.StatusNotFound, "container_not_found"},
		{"stopped container", "/hosts/local/containers/stopped/terminal", nil, nil, http.StatusConflict, "container_not_running"},
		{"helper container", "/hosts/local/containers/helper/terminal", nil, nil, http.StatusConflict, "helper_container"},
		{"protected local container", "/hosts/local/containers/web/terminal", nil, protectLocal, http.StatusConflict, "container_protected_confirmation_required"},
		{"protected remote container", "/hosts/" + nas + "/containers/web/terminal", nil, nil, http.StatusConflict, "container_protected_confirmation_required"},
		{"no stream slot", "/hosts/local/containers/web/terminal", nil, hold, http.StatusTooManyRequests, "too_many_streams"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if tc.setup != nil {
				tc.setup(t)
			}
			for mode, probe := range map[string]func(string, http.Header) dmTerminalProbe{"handshake": ls.handshake, "plain GET": ls.diagnose} {
				got := probe(tc.path, tc.headers)
				if got.status != tc.status || got.body["status"] != "error" || got.body["code"] != tc.code {
					t.Fatalf("%s: answer = %d %v, want %d error %q", mode, got.status, got.body, tc.status, tc.code)
				}
			}
		})
	}
	if t.Failed() {
		return
	}

	// The reason of the protection is named, so the confirmation can say who owns the container.
	if got := ls.diagnose("/hosts/"+nas+"/containers/web/terminal", nil); got.body["owner"] != "ruview" {
		t.Fatalf("remote protection answer = %v, want the owner ruview", got.body)
	}
	if got := ls.diagnose("/hosts/local/containers/helper/terminal", nil); got.body["role"] != "runner" {
		t.Fatalf("helper answer = %v, want the helper role", got.body)
	}
	ls.open(http.MethodPost, "/hosts/local/containers/web/terminal", nil, nil).expectError(http.StatusMethodNotAllowed, "method_not_allowed")
}

func TestDockerManagerTerminalPlainGETNeverCreatesAnExecOrKeepsASlot(t *testing.T) {
	ls := newDMLiveServer(t, "[]")
	got := ls.diagnose("/hosts/local/containers/web/terminal", nil)
	if got.status != http.StatusBadRequest || got.body["code"] != "websocket_required" {
		t.Fatalf("plain GET = %d %v, want 400 websocket_required once every check has passed", got.status, got.body)
	}
	if execs := ls.engine.term.find(http.MethodPost, "/containers/web/exec"); len(execs) != 0 {
		t.Fatalf("a plain GET created %d exec(s)", len(execs))
	}
	if free := dmTerminalSlots(t, ls.s.DockerWatch, "local"); free != dockerwatch.MaxStreamsPerHost {
		t.Fatalf("%d of %d stream slots are free after a plain GET, want all", free, dockerwatch.MaxStreamsPerHost)
	}
}

func TestDockerManagerTerminalServesTheSharedProtocolOnAnyHost(t *testing.T) {
	ls := newDMLiveServer(t, "[]")
	remote := newDMLiveEngine(t, dmContainersJSON(t,
		dmContainerJSON('w', "web", "sha256:img1", "running", "bridge", map[string]string{"aurago.managed": "ruview"}, nil)))
	nas := ls.addTCPHost("NAS", remote)

	roundTrip := func(conn *websocket.Conn) {
		t.Helper()
		readText := func(n int) string {
			var got []byte
			_ = conn.SetReadDeadline(time.Now().Add(5 * time.Second))
			for len(got) < n {
				_, payload, err := conn.ReadMessage()
				if err != nil {
					t.Fatalf("read: %v (got %q)", err, got)
				}
				got = append(got, payload...)
			}
			return string(got)
		}
		if got := readText(len("ready$ ")); got != "ready$ " {
			t.Fatalf("prompt = %q", got)
		}
		if err := conn.WriteMessage(websocket.BinaryMessage, []byte("pwd\r")); err != nil {
			t.Fatal(err)
		}
		if got := readText(4); got != "pwd\r" {
			t.Fatalf("echo = %q", got)
		}
	}

	// The local host: the whole protocol, and the slot is held while the session runs.
	local := ls.handshake("/hosts/local/containers/web/terminal", nil)
	if local.conn == nil || local.status != http.StatusSwitchingProtocols {
		t.Fatalf("handshake = %+v, want a WebSocket", local)
	}
	roundTrip(local.conn)
	if free := dmTerminalSlots(t, ls.s.DockerWatch, "local"); free != dockerwatch.MaxStreamsPerHost-1 {
		t.Fatalf("%d stream slots free during a session, want %d", free, dockerwatch.MaxStreamsPerHost-1)
	}
	if err := local.conn.WriteMessage(websocket.TextMessage, []byte(`{"type":"resize","cols":90,"rows":20}`)); err != nil {
		t.Fatal(err)
	}
	dmTermWaitFor(t, "the resize", func() bool {
		rs := ls.engine.term.find(http.MethodPost, "/exec/exec-1/resize")
		return len(rs) == 2 && rs[1].query == "h=20&w=90"
	})
	if err := local.conn.WriteMessage(websocket.TextMessage, []byte(`{"type":"end"}`)); err != nil {
		t.Fatal(err)
	}
	dmTermWaitFor(t, "the end helper", func() bool { return len(ls.engine.term.find(http.MethodPost, "/exec/exec-2/start")) == 1 })
	_ = local.conn.Close()
	dmTermWaitFor(t, "the exec stream to close", func() bool { return ls.engine.term.hijack.Load() == 1 })
	dmTermWaitFor(t, "the stream slot to come back", func() bool {
		return dmTerminalSlots(t, ls.s.DockerWatch, "local") == dockerwatch.MaxStreamsPerHost
	})

	// A protected container of a remote host needs the confirmation; with it the exec runs on that host's Engine.
	remoteWS := ls.handshake("/hosts/"+nas+"/containers/web/terminal?confirm=protected", nil)
	if remoteWS.conn == nil {
		t.Fatalf("confirmed handshake = %+v, want a WebSocket", remoteWS)
	}
	roundTrip(remoteWS.conn)
	if n := len(remote.term.find(http.MethodPost, "/containers/web/exec")); n != 1 {
		t.Fatalf("%d exec(s) on the remote Engine, want 1", n)
	}

	// The same for a protected container of the local host.
	t.Cleanup(replaceContainerProtection(containerProtection{Owner: "ruview"}))
	if got := ls.handshake("/hosts/local/containers/web/terminal", nil); got.status != http.StatusConflict {
		t.Fatalf("unconfirmed protected handshake = %d, want 409", got.status)
	}
	confirmed := ls.handshake("/hosts/local/containers/web/terminal?confirm=protected", nil)
	if confirmed.conn == nil {
		t.Fatalf("confirmed protected handshake = %+v, want a WebSocket", confirmed)
	}
}
