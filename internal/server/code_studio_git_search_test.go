package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestCodeStudioGitPorcelainPreservesPaths(t *testing.T) {
	input := " M main.go\x00M  staged.go\x00 D deleted.go\x00?? -option.txt\x00??  spaced name \x00?? Grüße\n文.txt\x00R  renamed.go\x00old.go\x00 C copied.go\x00original.go\x00?? last.txt\x00"
	want := []map[string]string{
		{"path": "main.go", "status": "M", "staged": "false"}, {"path": "staged.go", "status": "M", "staged": "true"},
		{"path": "deleted.go", "status": "D", "staged": "false"}, {"path": "-option.txt", "status": "??", "staged": "false"},
		{"path": " spaced name ", "status": "??", "staged": "false"}, {"path": "Grüße\n文.txt", "status": "??", "staged": "false"},
		{"path": "renamed.go", "status": "R", "staged": "true"}, {"path": "copied.go", "status": "C", "staged": "false"},
		{"path": "last.txt", "status": "??", "staged": "false"},
	}
	if got := parseGitPorcelain(input); !reflect.DeepEqual(got, want) {
		t.Fatalf("parsed changes=%#v; want=%#v", got, want)
	}
	if got := parseGitPorcelain("R  incomplete\x00"); len(got) != 0 {
		t.Fatalf("incomplete rename should not expose a misleading path: %#v", got)
	}
}

func TestCodeStudioSearchLeadingDashIsLiteral(t *testing.T) {
	for _, query := range []string{"--version", "-n", "--help", "pattern"} {
		cmd, err := buildCodeStudioSearchCommand(codeStudioSearchOptions{Query: query, Path: "/workspace"})
		if err != nil {
			t.Fatal(err)
		}
		var options struct{ Args []string }
		if err := json.Unmarshal([]byte(cmd[len(cmd)-1]), &options); err != nil {
			t.Fatal(err)
		}
		want := []string{"-e", query, "--"}
		if !reflect.DeepEqual(options.Args[len(options.Args)-3:], want) {
			t.Fatalf("query can become an option: %q", options.Args)
		}
	}
}

func TestCodeStudioSearchLeadingDashWithGrep(t *testing.T) {
	grep, err := exec.LookPath("grep")
	if err != nil {
		t.Skip("grep unavailable")
	}
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "source.txt"), []byte("literal --version and -n\n"), 0600); err != nil {
		t.Fatal(err)
	}
	for _, query := range []string{"--version", "-n"} {
		cmd, err := buildCodeStudioSearchCommand(codeStudioSearchOptions{Query: query, Path: "/workspace"})
		if err != nil {
			t.Fatal(err)
		}
		var options struct{ Args []string }
		if err := json.Unmarshal([]byte(cmd[len(cmd)-1]), &options); err != nil {
			t.Fatal(err)
		}
		args := append(options.Args[1:], filepath.Join(dir, "source.txt"))
		output, err := exec.Command(grep, args...).CombinedOutput()
		if err != nil || !strings.Contains(string(output), "1:literal --version and -n") {
			t.Fatalf("literal grep failed: %v: %s", err, output)
		}
	}
}

func TestCodeStudioGitStatusAndDiffUseExactPaths(t *testing.T) {
	s := testCodeStudioServerWithFakeCodeContainer(t, 1)
	docker := &recordingCodeStudioDockerAPI{results: []codeStudioExecResult{{Output: " M  spaced name \x00"}, {Output: "main\n"}, {Output: "abc change\n"}}}
	h := codeStudioHandlers{server: s, docker: docker}
	rec := httptest.NewRecorder()
	h.handleGitStatus(rec, httptest.NewRequest(http.MethodGet, "/api/code-studio/git/status", nil))
	var body struct{ Changes []map[string]string }
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil || rec.Code != http.StatusOK || len(body.Changes) != 1 || body.Changes[0]["path"] != " spaced name " {
		t.Fatalf("git status changed the path: status=%d body=%s error=%v", rec.Code, rec.Body.String(), err)
	}
	if !strings.Contains(strings.Join(docker.commands[0], " "), "--porcelain=v1 -z") {
		t.Fatal("status did not request NUL-delimited porcelain")
	}
	rec = httptest.NewRecorder()
	h.handleGitDiff(rec, httptest.NewRequest(http.MethodGet, "/api/code-studio/git/diff?file="+url.QueryEscape(body.Changes[0]["path"]), nil))
	if got := docker.commands[len(docker.commands)-1]; !reflect.DeepEqual(got[len(got)-2:], []string{"--", " spaced name "}) {
		t.Fatalf("diff changed the filename: %q", got)
	}
}

func TestCodeStudioGitFailuresAreNotRepositoryData(t *testing.T) {
	s := testCodeStudioServerWithFakeCodeContainer(t, 1)
	failure := codeStudioExecResult{ExitCode: 128, Output: "fatal: not a git repository\n"}
	for _, route := range []string{"status", "branch", "log", "diff"} {
		t.Run(route, func(t *testing.T) {
			docker := &recordingCodeStudioDockerAPI{results: []codeStudioExecResult{failure}}
			h := codeStudioHandlers{server: s, docker: docker}
			handlers := map[string]http.HandlerFunc{"status": h.handleGitStatus, "branch": h.handleGitBranch, "log": h.handleGitLog, "diff": h.handleGitDiff}
			rec := httptest.NewRecorder()
			handlers[route](rec, httptest.NewRequest(http.MethodGet, "/api/code-studio/git/"+route, nil))
			if route == "diff" {
				if rec.Code != http.StatusBadRequest {
					t.Fatalf("failed diff: %d %s", rec.Code, rec.Body.String())
				}
			} else if rec.Code != http.StatusOK || strings.Contains(rec.Body.String(), "fatal:") {
				t.Fatalf("error exposed as repository data: %d %s", rec.Code, rec.Body.String())
			}
			if len(docker.commands) != 1 {
				t.Fatalf("commands after failed repository read: %q", docker.commands)
			}
		})
	}
	docker := &recordingCodeStudioDockerAPI{results: []codeStudioExecResult{{}, {Output: "main\n"}, {ExitCode: 128, Output: "fatal: no commits yet\n"}}}
	h := codeStudioHandlers{server: s, docker: docker}
	rec := httptest.NewRecorder()
	h.handleGitStatus(rec, httptest.NewRequest(http.MethodGet, "/api/code-studio/git/status", nil))
	var status struct {
		Branch string
		Log    []map[string]string
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &status); err != nil || status.Branch != "main" || len(status.Log) != 0 {
		t.Fatalf("new repository status: %s, %v", rec.Body.String(), err)
	}
}

func TestCodeStudioGitUsesLiteralPathspecs(t *testing.T) {
	git, err := exec.LookPath("git")
	if err != nil {
		t.Skip("git unavailable")
	}
	dir := t.TempDir()
	if output, err := exec.Command(git, "init", dir).CombinedOutput(); err != nil {
		t.Fatalf("git init: %v: %s", err, output)
	}
	for _, name := range []string{"a[1].txt", "a1.txt"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(name), 0600); err != nil {
			t.Fatal(err)
		}
	}
	docker := &recordingCodeStudioDockerAPI{}
	h := codeStudioHandlers{docker: docker}
	if _, err := h.execGit(context.Background(), "container", time.Second, "add", "--", "a[1].txt"); err != nil {
		t.Fatal(err)
	}
	command := docker.commands[0]
	command[2] = dir
	if output, err := exec.Command(git, command[1:]...).CombinedOutput(); err != nil {
		t.Fatalf("literal add: %v: %s", err, output)
	}
	output, err := exec.Command(git, "-C", dir, "ls-files", "-z").CombinedOutput()
	if err != nil || string(output) != "a[1].txt\x00" {
		t.Fatalf("staged another file through pathspec expansion: %q, %v", output, err)
	}
}
