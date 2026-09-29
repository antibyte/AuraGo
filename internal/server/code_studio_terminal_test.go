package server

import (
	"context"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

type terminalTestDocker struct {
	fakeCodeStudioDockerAPI
	commands []string
}

func (d *terminalTestDocker) Exec(_ context.Context, _ string, cmd []string, _ time.Duration) (codeStudioExecResult, error) {
	command := cmd[2]
	d.commands = append(d.commands, command)
	if strings.HasSuffix(command, " && pwd -P") {
		for _, dir := range []string{"/workspace", "/workspace/src", "/workspace/with space"} {
			if command == "cd "+shellQuote(dir)+" && pwd -P" {
				return codeStudioExecResult{Output: dir + "\n"}, nil
			}
		}
		if command == "cd '/workspace/escape' && pwd -P" {
			return codeStudioExecResult{Output: "/etc\n"}, nil
		}
		return codeStudioExecResult{ExitCode: 1, Output: "no such directory\n"}, nil
	}
	return codeStudioExecResult{Output: "ok\n"}, nil
}

func TestCodeStudioTerminalInputAndDirectories(t *testing.T) {
	for _, tc := range []struct {
		name, cwd, wantCWD string
		frames, commands   []string
	}{
		{"absolute", "/workspace/src", "/workspace", []string{"cd /workspace\rpwd\r"}, []string{"cd '/workspace' && pwd -P", "cd '/workspace' && pwd"}},
		{"parent", "/workspace/src", "/workspace", []string{"cd ..\npwd\n"}, []string{"cd '/workspace' && pwd -P", "cd '/workspace' && pwd"}},
		{"default", "/workspace/src", "/workspace", []string{"cd\npwd\n"}, []string{"cd '/workspace' && pwd -P", "cd '/workspace' && pwd"}},
		{"quoted", "/workspace", "/workspace/with space", []string{"cd 'with space'\r\npwd\n"}, []string{"cd '/workspace/with space' && pwd -P", "cd '/workspace/with space' && pwd"}},
		{"missing", "/workspace", "/workspace", []string{"cd missing\npwd\n"}, []string{"cd '/workspace/missing' && pwd -P", "cd '/workspace' && pwd"}},
		{"escape", "/workspace", "/workspace", []string{"cd /etc\ncd ..\npwd\n"}, []string{"cd '/workspace' && pwd"}},
		{"symlink escape", "/workspace", "/workspace", []string{"cd escape\npwd\n"}, []string{"cd '/workspace/escape' && pwd -P", "cd '/workspace' && pwd"}},
		{"compound", "/workspace/src", "/workspace/src", []string{"cd /workspace && pwd\n"}, []string{"cd '/workspace/src' && cd /workspace && pwd"}},
		{"batch and split CRLF", "/workspace", "/workspace", []string{"echo one\necho tw", "o\r", "\necho three\r"}, []string{"cd '/workspace' && echo one", "cd '/workspace' && echo two", "cd '/workspace' && echo three"}},
		{"backspace", "/workspace", "/workspace", []string{"echo ü\bö\n"}, []string{"cd '/workspace' && echo ö"}},
		{"split UTF8", "/workspace", "/workspace", []string{"echo \xc3", "\xbc\n"}, []string{"cd '/workspace' && echo ü"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			docker := &terminalTestDocker{}
			h := codeStudioHandlers{docker: docker}
			session := newCodeStudioTerminalSession(tc.cwd)
			for _, input := range tc.frames {
				for {
					line, _, complete, err := session.consume(input)
					input = ""
					if err != nil {
						t.Fatal(err)
					}
					if !complete {
						break
					}
					h.executeTerminalLine(context.Background(), "test", session, line)
				}
			}
			if session.cwd != tc.wantCWD || !reflect.DeepEqual(docker.commands, tc.commands) {
				t.Fatalf("cwd=%q commands=%q; want cwd=%q commands=%q", session.cwd, docker.commands, tc.wantCWD, tc.commands)
			}
		})
	}
}

func TestCodeStudioTerminalBoundsAndCancellation(t *testing.T) {
	session := newCodeStudioTerminalSession("/workspace")
	if _, _, _, err := session.consume(strings.Repeat("x", codeStudioTerminalInputLimit)); err != nil {
		t.Fatal(err)
	}
	if _, _, _, err := session.consume("x"); err == nil {
		t.Fatal("input limit did not include the incomplete line")
	}
	docker := &terminalTestDocker{}
	h := codeStudioHandlers{docker: docker}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	h.executeTerminalLine(ctx, "test", session, "echo must-not-run")
	if len(docker.commands) != 0 {
		t.Fatal("cancelled terminal executed another command")
	}
}

func TestCodeStudioTerminalWebSocketDrainsPastedLines(t *testing.T) {
	docker := &terminalTestDocker{}
	h := codeStudioHandlers{docker: docker}
	finished := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer close(finished)
		conn, err := codeStudioWSUpgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()
		h.runLineTerminal(r.Context(), conn, "test")
	}))
	defer srv.Close()
	conn, _, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(srv.URL, "http"), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	_ = conn.SetReadDeadline(time.Now().Add(3 * time.Second))
	if err := conn.WriteMessage(websocket.TextMessage, []byte("cd src\npwd\necho done\r\npartial")); err != nil {
		t.Fatal(err)
	}
	var output strings.Builder
	for !strings.HasSuffix(output.String(), "/workspace/src $ partial") {
		_, data, err := conn.ReadMessage()
		if err != nil {
			t.Fatalf("pasted commands did not finish: %v; output=%q", err, output.String())
		}
		output.Write(data)
	}
	_ = conn.Close()
	<-finished
	want := []string{"cd '/workspace/src' && pwd -P", "cd '/workspace/src' && pwd", "cd '/workspace/src' && echo done"}
	if !reflect.DeepEqual(docker.commands, want) {
		t.Fatalf("commands=%q want=%q", docker.commands, want)
	}
}
