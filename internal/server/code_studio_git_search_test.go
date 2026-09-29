package server

import (
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
)

func TestCodeStudioGitPorcelainPreservesPaths(t *testing.T) {
	input := " M main.go\x00M  staged.go\x00 D deleted.go\x00?? -option.txt\x00??  spaced name \x00?? Grüße\n文.txt\x00R  renamed.go\x00old.go\x00 C copied.go\x00original.go\x00?? last.txt\x00"
	want := []map[string]string{
		{"path": "main.go", "status": "M"}, {"path": "staged.go", "status": "M"},
		{"path": "deleted.go", "status": "D"}, {"path": "-option.txt", "status": "??"},
		{"path": " spaced name ", "status": "??"}, {"path": "Grüße\n文.txt", "status": "??"},
		{"path": "renamed.go", "status": "R"}, {"path": "copied.go", "status": "C"},
		{"path": "last.txt", "status": "??"},
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
		want := []string{"-e", query, "--", "/workspace"}
		if !reflect.DeepEqual(cmd[len(cmd)-4:], want) {
			t.Fatalf("query can become an option: %q", cmd)
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
		cmd[len(cmd)-1] = dir
		output, err := exec.Command(grep, cmd[1:]...).CombinedOutput()
		if err != nil || !strings.Contains(string(output), ":1:literal --version and -n") {
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
	if !strings.Contains(docker.commands[0][2], "--porcelain=v1 -z") {
		t.Fatal("status did not request NUL-delimited porcelain")
	}
	rec = httptest.NewRecorder()
	h.handleGitDiff(rec, httptest.NewRequest(http.MethodGet, "/api/code-studio/git/diff?file="+url.QueryEscape(body.Changes[0]["path"]), nil))
	if got := docker.commands[len(docker.commands)-1][2]; !strings.HasSuffix(got, " -- ' spaced name '") {
		t.Fatalf("diff changed the filename: %q", got)
	}
}
