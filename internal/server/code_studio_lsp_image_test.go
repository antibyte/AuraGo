package server

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"testing"
	"time"

	"aurago/internal/dockerutil"
	"aurago/internal/tools"

	"github.com/gorilla/websocket"
)

// codeStudioLSPSample is one small project and the hover its server must answer.
type codeStudioLSPSample struct {
	lang       string            // bridge lang parameter
	languageID string            // LSP languageId of the opened file
	path       string            // the file that is opened and hovered
	needle     string            // hover position: inside its last occurrence
	want       string            // text the hover result must contain
	files      map[string]string // container path -> content
}

var codeStudioLSPSamples = []codeStudioLSPSample{
	{
		lang: "go", languageID: "go", path: "/workspace/golsp/main.go", needle: "Greeting(", want: "Greeting",
		files: map[string]string{
			"/workspace/golsp/go.mod":  "module example.com/golsp\n\ngo 1.27\n",
			"/workspace/golsp/main.go": "package main\n\nimport \"fmt\"\n\n// Greeting returns a friendly greeting.\nfunc Greeting(name string) string {\n\treturn \"hello \" + name\n}\n\nfunc main() {\n\tfmt.Println(Greeting(\"lsp\"))\n}\n",
		},
	},
	{
		lang: "typescript", languageID: "typescript", path: "/workspace/tslsp/index.ts", needle: "greeting(", want: "greeting",
		files: map[string]string{
			"/workspace/tslsp/tsconfig.json": "{\"compilerOptions\": {\"strict\": true, \"noEmit\": true}}\n",
			"/workspace/tslsp/index.ts":      "export function greeting(name: string): string {\n  return \"hello \" + name;\n}\n\nexport const message = greeting(\"lsp\");\n",
		},
	},
	{
		lang: "python", languageID: "python", path: "/workspace/pylsp/main.py", needle: "greeting(", want: "greeting",
		files: map[string]string{
			"/workspace/pylsp/main.py": "def greeting(name: str) -> str:\n    \"\"\"Return a friendly greeting.\"\"\"\n    return \"hello \" + name\n\n\nmessage = greeting(\"lsp\")\n",
		},
	},
	{
		lang: "cpp", languageID: "c", path: "/workspace/clsp/main.c", needle: "add(", want: "add",
		files: map[string]string{
			"/workspace/clsp/main.c": "/* Adds two numbers. */\nint add(int a, int b) {\n    return a + b;\n}\n\nint main(void) {\n    return add(1, 2) == 3 ? 0 : 1;\n}\n",
		},
	},
}

// TestCodeStudioLSPRealImage drives the four language servers of a locally
// built Code Studio image through the real bridge and the real Docker
// adapter. It is opt-in: AURAGO_LSP_IMAGE names the image, for example
// aurago-code-studio:lsp-dev. AURAGO_LSP_DOCKER_HOST overrides the bridge's
// Docker endpoint (default: the OS default, the Docker Desktop pipe on
// Windows). The docker CLI creates and removes the container.
func TestCodeStudioLSPRealImage(t *testing.T) {
	image := strings.TrimSpace(os.Getenv("AURAGO_LSP_IMAGE"))
	if image == "" {
		t.Skip("set AURAGO_LSP_IMAGE=aurago-code-studio:lsp-dev to check the real language servers")
	}
	tools.ConfigureRuntimePermissions(tools.RuntimePermissions{DockerEnabled: true})
	t.Cleanup(tools.ClearRuntimePermissionsForTest)
	tag, err := newContainerTerminalNonce()
	if err != nil {
		t.Fatal(err)
	}
	name := "aurago-lsp-it-" + tag[:12]
	// The same isolation as the managed Code Studio container.
	codeStudioLSPDockerCLI(t, nil, "run", "-d", "--name", name, "--network", "none", "--user", "developer",
		"--security-opt", "no-new-privileges:true", "--cap-drop", "ALL", "--cap-add", "CHOWN", "--cap-add", "FOWNER",
		"--memory", "4096m", "--pids-limit", "1024",
		"-e", "HOME=/home/developer", "-e", "USER=developer", "-e", "GOPATH=/home/developer/go",
		image, "sleep", "infinity")
	t.Cleanup(func() { _ = exec.Command("docker", "rm", "-f", name).Run() })
	for _, sample := range codeStudioLSPSamples {
		for path, content := range sample.files {
			codeStudioLSPDockerCLI(t, strings.NewReader(content), "exec", "-i", name, "sh", "-c", `mkdir -p "$(dirname "$1")" && cat > "$1"`, "sh", path)
		}
	}

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug}))
	h := codeStudioHandlers{
		server: &Server{Logger: logger},
		docker: codeStudioDockerAdapter{cfg: tools.DockerConfig{Host: dockerutil.NormalizeHost(os.Getenv("AURAGO_LSP_DOCKER_HOST"))}, logger: logger},
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		lsp, ok := codeStudioLSPServerFor(r.URL.Query().Get("lang"))
		if !ok {
			http.Error(w, "unknown language server", http.StatusBadRequest)
			return
		}
		conn, err := codeStudioWSUpgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()
		h.serveCodeStudioLSP(r.Context(), conn, h.containerLSPHost(name), lsp)
	}))
	defer srv.Close()

	for _, sample := range codeStudioLSPSamples {
		t.Run(sample.lang, func(t *testing.T) {
			runCodeStudioLSPSample(t, srv.URL, sample)
		})
	}

	// Every socket is closed now: the bridge cleanup must leave no server process.
	deadline := time.Now().Add(20 * time.Second)
	for {
		comms := codeStudioLSPDockerCLI(t, nil, "exec", name, "sh", "-c", `for d in /proc/[0-9]*; do cat "$d/comm" 2>/dev/null; done`)
		left := codeStudioLSPLeftovers(comms)
		if len(left) == 0 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("language server processes survived the bridge cleanup: %v", left)
		}
		time.Sleep(500 * time.Millisecond)
	}
}

// runCodeStudioLSPSample opens one bridge socket and checks initialize and hover.
func runCodeStudioLSPSample(t *testing.T, baseURL string, sample codeStudioLSPSample) {
	conn, _, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(baseURL, "http")+"/?lang="+sample.lang, http.Header{"Origin": {baseURL}})
	if err != nil {
		t.Fatalf("dial %s: %v", sample.lang, err)
	}
	defer conn.Close()
	client := &codeStudioLSPTestClient{t: t, conn: conn, deadline: time.Now().Add(3 * time.Minute)}
	result, rpcErr := client.call("initialize", map[string]interface{}{
		"processId":        nil,
		"rootUri":          "file:///workspace",
		"workspaceFolders": []map[string]string{{"uri": "file:///workspace", "name": "workspace"}},
		"capabilities": map[string]interface{}{
			"textDocument": map[string]interface{}{
				"hover": map[string]interface{}{"contentFormat": []string{"markdown", "plaintext"}},
			},
		},
	})
	if rpcErr != "" {
		t.Fatalf("initialize: %s", rpcErr)
	}
	var initialized struct {
		Capabilities map[string]json.RawMessage `json:"capabilities"`
	}
	if err := json.Unmarshal(result, &initialized); err != nil {
		t.Fatalf("decode initialize result %s: %v", result, err)
	}
	if hover := strings.TrimSpace(string(initialized.Capabilities["hoverProvider"])); hover == "" || hover == "false" || hover == "null" {
		t.Fatalf("initialize result %s has no hoverProvider", result)
	}
	client.notify("initialized", map[string]interface{}{})
	text := sample.files[sample.path]
	uri := "file://" + sample.path
	client.notify("textDocument/didOpen", map[string]interface{}{
		"textDocument": map[string]interface{}{"uri": uri, "languageId": sample.languageID, "version": 1, "text": text},
	})
	line, character := codeStudioLSPPosition(text, sample.needle)
	// A server may answer null or an error until it has loaded the project, so ask again.
	for attempt := 1; ; attempt++ {
		hover, rpcErr := client.call("textDocument/hover", map[string]interface{}{
			"textDocument": map[string]string{"uri": uri},
			"position":     map[string]int{"line": line, "character": character},
		})
		if rpcErr == "" && strings.Contains(string(hover), sample.want) {
			return
		}
		if attempt == 30 {
			t.Fatalf("hover at %d:%d = %s (error %q), want text containing %q", line, character, hover, rpcErr, sample.want)
		}
		time.Sleep(time.Second)
	}
}

// codeStudioLSPTestClient is a minimal JSON-RPC client on a bridge socket.
type codeStudioLSPTestClient struct {
	t        *testing.T
	conn     *websocket.Conn
	nextID   int
	deadline time.Time
}

func (c *codeStudioLSPTestClient) send(msg map[string]interface{}) {
	c.t.Helper()
	msg["jsonrpc"] = "2.0"
	data, err := json.Marshal(msg)
	if err != nil {
		c.t.Fatalf("encode %v: %v", msg["method"], err)
	}
	if err := c.conn.WriteMessage(websocket.TextMessage, data); err != nil {
		c.t.Fatalf("send %v: %v", msg["method"], err)
	}
}

func (c *codeStudioLSPTestClient) notify(method string, params interface{}) {
	c.t.Helper()
	c.send(map[string]interface{}{"method": method, "params": params})
}

// call sends a request and returns its result, or the error message of an
// error response. Server requests that arrive meanwhile get an empty answer
// (workspace/configuration one null per item); notifications are skipped.
func (c *codeStudioLSPTestClient) call(method string, params interface{}) (json.RawMessage, string) {
	c.t.Helper()
	c.nextID++
	id := strconv.Itoa(c.nextID)
	c.send(map[string]interface{}{"id": c.nextID, "method": method, "params": params})
	for {
		_ = c.conn.SetReadDeadline(c.deadline)
		_, data, err := c.conn.ReadMessage()
		if err != nil {
			c.t.Fatalf("%s: read: %v", method, err)
		}
		var msg struct {
			ID     json.RawMessage `json:"id"`
			Method string          `json:"method"`
			Params json.RawMessage `json:"params"`
			Result json.RawMessage `json:"result"`
			Error  *struct {
				Code    int    `json:"code"`
				Message string `json:"message"`
			} `json:"error"`
		}
		if err := json.Unmarshal(data, &msg); err != nil {
			c.t.Fatalf("%s: decode %s: %v", method, data, err)
		}
		if msg.Method != "" {
			if len(msg.ID) > 0 {
				c.answer(msg.ID, msg.Method, msg.Params)
			}
			continue
		}
		if string(msg.ID) != id {
			continue
		}
		if msg.Error != nil {
			return nil, strconv.Itoa(msg.Error.Code) + " " + msg.Error.Message
		}
		if len(msg.Result) == 0 {
			msg.Result = json.RawMessage("null")
		}
		return msg.Result, ""
	}
}

func (c *codeStudioLSPTestClient) answer(id json.RawMessage, method string, params json.RawMessage) {
	c.t.Helper()
	var result interface{}
	if method == "workspace/configuration" {
		var request struct {
			Items []json.RawMessage `json:"items"`
		}
		_ = json.Unmarshal(params, &request)
		result = make([]interface{}, len(request.Items))
	}
	c.send(map[string]interface{}{"id": id, "result": result})
}

// codeStudioLSPPosition returns the LSP position (0-based line, UTF-16
// column) of the second character of the last occurrence of needle. The
// samples are ASCII.
func codeStudioLSPPosition(text, needle string) (int, int) {
	index := strings.LastIndex(text, needle)
	before := text[:index]
	return strings.Count(before, "\n"), index - (strings.LastIndex(before, "\n") + 1) + 1
}

// codeStudioLSPLeftovers lists process names that belong to a language
// server: the servers, their node runtimes and gopls's go children.
func codeStudioLSPLeftovers(comms string) []string {
	var left []string
	for _, line := range strings.Split(comms, "\n") {
		comm := strings.TrimSpace(line)
		switch {
		case comm == "gopls", comm == "go", comm == "node", strings.HasPrefix(comm, "clangd"),
			strings.HasPrefix(comm, "typescript"), strings.HasPrefix(comm, "pyright"), strings.HasPrefix(comm, "tsserver"):
			left = append(left, comm)
		}
	}
	return left
}

// codeStudioLSPDockerCLI runs the docker CLI and fails the test on an error.
func codeStudioLSPDockerCLI(t *testing.T, stdin io.Reader, args ...string) string {
	t.Helper()
	cmd := exec.Command("docker", args...)
	cmd.Stdin = stdin
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("docker %s: %v\n%s", strings.Join(args, " "), err, out)
	}
	return string(out)
}
