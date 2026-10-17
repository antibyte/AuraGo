package server

import (
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
)

func TestCodeStudioGitBranchGetDoesNotMutate(t *testing.T) {
	s := testCodeStudioServerWithFakeCodeContainer(t, 1)
	for _, target := range []string{
		"/api/code-studio/git/branch?action=create&name=feature",
		"/api/code-studio/git/branch?action=switch&name=main",
	} {
		docker := &recordingCodeStudioDockerAPI{}
		h := codeStudioHandlers{server: s, docker: docker}
		rec := httptest.NewRecorder()
		h.handleGitBranch(rec, httptest.NewRequest(http.MethodGet, target, nil))
		if rec.Code != http.StatusMethodNotAllowed || !strings.Contains(rec.Body.String(), "POST") {
			t.Fatalf("GET %s status=%d body=%s", target, rec.Code, rec.Body.String())
		}
		if len(docker.commands) != 0 {
			t.Fatalf("GET %s ran git: %#v", target, docker.commands)
		}
	}

	docker := &recordingCodeStudioDockerAPI{results: []codeStudioExecResult{{Output: "* main\n"}, {Output: "main\n"}}}
	h := codeStudioHandlers{server: s, docker: docker}
	rec := httptest.NewRecorder()
	h.handleGitBranch(rec, httptest.NewRequest(http.MethodGet, "/api/code-studio/git/branch?action=create-later&name=ignored", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("list status=%d body=%s", rec.Code, rec.Body.String())
	}
	for _, cmd := range docker.commands {
		joined := strings.Join(cmd, " ")
		if strings.Contains(joined, "checkout") {
			t.Fatalf("list ran checkout: %#v", docker.commands)
		}
	}
	if len(docker.commands) != 2 || !strings.Contains(docker.commands[0][2], "git branch -a") {
		t.Fatalf("list commands=%#v", docker.commands)
	}

	docker = &recordingCodeStudioDockerAPI{}
	h = codeStudioHandlers{server: s, docker: docker}
	rec = httptest.NewRecorder()
	h.handleGitBranch(rec, httptest.NewRequest(http.MethodPost, "/api/code-studio/git/branch", strings.NewReader(`{"action":"create","name":"feature/one"}`)))
	if rec.Code != http.StatusOK {
		t.Fatalf("create status=%d body=%s", rec.Code, rec.Body.String())
	}
	want := []string{"git", "-C", "/workspace", "checkout", "-b", "feature/one"}
	if !reflect.DeepEqual(docker.commands[0], want) {
		t.Fatalf("create command=%#v", docker.commands[0])
	}

	docker = &recordingCodeStudioDockerAPI{}
	h = codeStudioHandlers{server: s, docker: docker}
	rec = httptest.NewRecorder()
	h.handleGitBranch(rec, httptest.NewRequest(http.MethodPost, "/api/code-studio/git/branch", strings.NewReader(`{"action":"switch","name":"-main"}`)))
	if rec.Code != http.StatusBadRequest || len(docker.commands) != 0 {
		t.Fatalf("leading dash status=%d commands=%#v", rec.Code, docker.commands)
	}
}

func TestCodeStudioGitCommitStagesOnlyGivenPaths(t *testing.T) {
	s := testCodeStudioServerWithFakeCodeContainer(t, 1)
	docker := &recordingCodeStudioDockerAPI{results: []codeStudioExecResult{{}, {}, {Output: "abc123\n"}}}
	h := codeStudioHandlers{server: s, docker: docker}
	rec := httptest.NewRecorder()
	body := `{"message":"save work","add_all":false,"paths":["main.go","/workspace/sub/a.txt"]}`
	h.handleGitCommit(rec, httptest.NewRequest(http.MethodPost, "/api/code-studio/git/commit", strings.NewReader(body)))
	if rec.Code != http.StatusOK {
		t.Fatalf("commit status=%d body=%s", rec.Code, rec.Body.String())
	}
	want := [][]string{
		{"git", "-C", "/workspace", "add", "--", "main.go"},
		{"git", "-C", "/workspace", "add", "--", "sub/a.txt"},
		{"git", "-C", "/workspace", "commit", "-m", "save work"},
		{"git", "-C", "/workspace", "rev-parse", "--short", "HEAD"},
	}
	if !reflect.DeepEqual(docker.commands, want) {
		t.Fatalf("commands=%#v", docker.commands)
	}

	docker = &recordingCodeStudioDockerAPI{}
	h.docker = docker
	rec = httptest.NewRecorder()
	h.handleGitCommit(rec, httptest.NewRequest(http.MethodPost, "/api/code-studio/git/commit", strings.NewReader(`{"message":"index only"}`)))
	if rec.Code != http.StatusOK || !reflect.DeepEqual(docker.commands[0], []string{"git", "-C", "/workspace", "commit", "-m", "index only"}) {
		t.Fatalf("index commit status=%d commands=%#v body=%s", rec.Code, docker.commands, rec.Body.String())
	}

	docker = &recordingCodeStudioDockerAPI{}
	h.docker = docker
	rec = httptest.NewRecorder()
	h.handleGitCommit(rec, httptest.NewRequest(http.MethodPost, "/api/code-studio/git/commit", strings.NewReader(`{"message":"all","add_all":true,"paths":["main.go"]}`)))
	if rec.Code != http.StatusOK || !reflect.DeepEqual(docker.commands[0], []string{"git", "-C", "/workspace", "add", "-A"}) {
		t.Fatalf("add all commands=%#v", docker.commands)
	}
	for _, cmd := range docker.commands {
		if len(cmd) >= 2 && cmd[len(cmd)-2] == "--" {
			t.Fatalf("add all also staged a path: %#v", docker.commands)
		}
	}

	docker = &recordingCodeStudioDockerAPI{}
	h.docker = docker
	rec = httptest.NewRecorder()
	h.handleGitCommit(rec, httptest.NewRequest(http.MethodPost, "/api/code-studio/git/commit", strings.NewReader(`{"message":"nope","paths":["../etc/passwd"]}`)))
	if rec.Code != http.StatusBadRequest || len(docker.commands) != 0 {
		t.Fatalf("escape status=%d commands=%#v body=%s", rec.Code, docker.commands, rec.Body.String())
	}

	docker = &recordingCodeStudioDockerAPI{results: []codeStudioExecResult{{ExitCode: 128, Output: "Author identity unknown\nPlease tell me who you are.\n"}}}
	h.docker = docker
	rec = httptest.NewRecorder()
	h.handleGitCommit(rec, httptest.NewRequest(http.MethodPost, "/api/code-studio/git/commit", strings.NewReader(`{"message":"needs name"}`)))
	if rec.Code != http.StatusConflict || !strings.Contains(rec.Body.String(), "git_identity_required") {
		t.Fatalf("identity status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestCodeStudioGitIdentityIsLocal(t *testing.T) {
	s := testCodeStudioServerWithFakeCodeContainer(t, 1)
	docker := &recordingCodeStudioDockerAPI{}
	h := codeStudioHandlers{server: s, docker: docker}
	rec := httptest.NewRecorder()
	h.handleGitIdentity(rec, httptest.NewRequest(http.MethodPost, "/api/code-studio/git/identity", strings.NewReader(`{"name":"Ada Lovelace","email":"ada@example.com"}`)))
	if rec.Code != http.StatusOK {
		t.Fatalf("identity status=%d body=%s", rec.Code, rec.Body.String())
	}
	want := [][]string{
		{"git", "-C", "/workspace", "config", "--local", "user.name", "Ada Lovelace"},
		{"git", "-C", "/workspace", "config", "--local", "user.email", "ada@example.com"},
	}
	if !reflect.DeepEqual(docker.commands, want) {
		t.Fatalf("commands=%#v", docker.commands)
	}
	for _, cmd := range docker.commands {
		for _, arg := range cmd {
			if arg == "--global" {
				t.Fatalf("identity used --global: %#v", cmd)
			}
		}
	}

	docker = &recordingCodeStudioDockerAPI{}
	h.docker = docker
	rec = httptest.NewRecorder()
	h.handleGitIdentity(rec, httptest.NewRequest(http.MethodPost, "/api/code-studio/git/identity", strings.NewReader(`{"name":"Ada","email":"ada\n@example.com"}`)))
	if rec.Code != http.StatusBadRequest || len(docker.commands) != 0 {
		t.Fatalf("control char status=%d commands=%#v body=%s", rec.Code, docker.commands, rec.Body.String())
	}
}
