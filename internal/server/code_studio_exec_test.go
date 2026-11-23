package server

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"aurago/internal/tools"
)

// stdinRecordingCodeStudioDockerAPI records the stdin payloads next to the commands.
type stdinRecordingCodeStudioDockerAPI struct {
	recordingCodeStudioDockerAPI
	stdins [][]byte
}

func (f *stdinRecordingCodeStudioDockerAPI) ExecWithStdin(ctx context.Context, containerID string, cmd []string, stdin []byte, timeout time.Duration) (codeStudioExecResult, error) {
	f.stdins = append(f.stdins, append([]byte(nil), stdin...))
	return f.Exec(ctx, containerID, cmd, timeout)
}

type codeStudioExecResponse struct {
	ExitCode  int    `json:"exit_code"`
	Output    string `json:"output"`
	Truncated bool   `json:"truncated"`
}

func postCodeStudioExec(t *testing.T, h codeStudioHandlers, body string) (*httptest.ResponseRecorder, codeStudioExecResponse) {
	t.Helper()
	rec := httptest.NewRecorder()
	h.handleExec(rec, httptest.NewRequest(http.MethodPost, "/api/code-studio/exec", strings.NewReader(body)))
	var resp codeStudioExecResponse
	if rec.Code == http.StatusOK {
		if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
			t.Fatalf("decode exec response: %v; body=%s", err, rec.Body.String())
		}
	}
	return rec, resp
}

func TestCodeStudioExecPipesStdinAndRejectsOversizedInput(t *testing.T) {
	s := testCodeStudioServerWithFakeCodeContainer(t, 1)
	docker := &stdinRecordingCodeStudioDockerAPI{recordingCodeStudioDockerAPI: recordingCodeStudioDockerAPI{results: []codeStudioExecResult{{Output: "package main\n"}}}}
	h := codeStudioHandlers{server: s, docker: docker}

	rec, resp := postCodeStudioExec(t, h, `{"args":["gofmt"],"stdin":"package  main\n"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("stdin exec status=%d body=%s", rec.Code, rec.Body.String())
	}
	if len(docker.stdins) != 1 || string(docker.stdins[0]) != "package  main\n" {
		t.Fatalf("stdin = %q, want the request payload", docker.stdins)
	}
	if !reflect.DeepEqual(docker.commands, [][]string{{"gofmt"}}) {
		t.Fatalf("commands = %#v", docker.commands)
	}
	if resp.Output != "package main\n" || resp.Truncated {
		t.Fatalf("response = %+v", resp)
	}

	payload, err := json.Marshal(map[string]interface{}{"args": []string{"gofmt"}, "stdin": strings.Repeat("a", 1<<20+1)})
	if err != nil {
		t.Fatal(err)
	}
	rec, _ = postCodeStudioExec(t, h, string(payload))
	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("oversized stdin status=%d body=%s", rec.Code, rec.Body.String())
	}
	if len(docker.commands) != 1 || len(docker.stdins) != 1 {
		t.Fatalf("oversized stdin reached docker: commands=%#v", docker.commands)
	}

	rec, _ = postCodeStudioExec(t, h, `{"args":["true"]}`)
	if rec.Code != http.StatusOK || len(docker.stdins) != 1 || len(docker.commands) != 2 {
		t.Fatalf("exec without stdin: status=%d stdins=%d commands=%#v", rec.Code, len(docker.stdins), docker.commands)
	}
}

func TestCodeStudioExecCapsOutputAtFourMiB(t *testing.T) {
	s := testCodeStudioServerWithFakeCodeContainer(t, 1)
	big := strings.Repeat("x", codeStudioExecOutputLimit-1) + "é" + "tail"
	docker := &recordingCodeStudioDockerAPI{results: []codeStudioExecResult{{Output: big}, {Output: "small"}}}
	h := codeStudioHandlers{server: s, docker: docker}

	rec, resp := postCodeStudioExec(t, h, `{"command":"cat big.log"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String()[:200])
	}
	if !resp.Truncated || len(resp.Output) != codeStudioExecOutputLimit-1 {
		t.Fatalf("truncated=%v len=%d, want the output cut before the split rune", resp.Truncated, len(resp.Output))
	}

	rec, resp = postCodeStudioExec(t, h, `{"command":"echo small"}`)
	if rec.Code != http.StatusOK || resp.Truncated || resp.Output != "small" {
		t.Fatalf("small output: status=%d response=%+v", rec.Code, resp)
	}
}

func TestCodeStudioExecWithStdinStreamsThroughDocker(t *testing.T) {
	var mu sync.Mutex
	var created map[string]interface{}
	var received []byte
	host := newContainerDockerAPI(t, func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(io.LimitReader(r.Body, 1<<20))
		switch {
		case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/containers/code-1/exec"):
			mu.Lock()
			_ = json.Unmarshal(body, &created)
			mu.Unlock()
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{"Id":"fmt-1"}`))
		case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/exec/fmt-1/start"):
			conn, err := hijackCodeStudioTTY(w)
			if err != nil {
				return
			}
			defer conn.Close()
			input, _ := io.ReadAll(conn)
			mu.Lock()
			received = input
			mu.Unlock()
			var frames bytes.Buffer
			writeAttachFrame(t, &frames, 1, bytes.ToUpper(input))
			writeAttachFrame(t, &frames, 2, []byte("warning\n"))
			_, _ = conn.Write(frames.Bytes())
		case r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/exec/fmt-1/json"):
			_, _ = w.Write([]byte(`{"Running":false,"ExitCode":3}`))
		default:
			http.Error(w, "unexpected "+r.Method+" "+r.URL.Path, http.StatusConflict)
		}
	})
	adapter := codeStudioDockerAdapter{cfg: tools.DockerConfig{Host: host}}

	result, err := adapter.ExecWithStdin(context.Background(), "code-1", []string{"gofmt"}, []byte("package main\n"), 5*time.Second)
	if err != nil {
		t.Fatalf("ExecWithStdin: %v", err)
	}
	mu.Lock()
	defer mu.Unlock()
	if string(received) != "package main\n" {
		t.Fatalf("engine received stdin %q", received)
	}
	if result.Output != "PACKAGE MAIN\nwarning\n" || result.ExitCode != 3 {
		t.Fatalf("result = %+v", result)
	}
	if created["AttachStdin"] != true || created["Tty"] != false {
		t.Fatalf("exec create = %v, want AttachStdin without a TTY", created)
	}
}
