package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
)

// codeStudioGitCmd is the docker exec command execGit builds for the given git arguments.
func codeStudioGitCmd(args ...string) []string {
	return append([]string{"git", "-C", "/workspace", "--literal-pathspecs"}, args...)
}

func TestCodeStudioGitPorcelainKeepsIndexAndWorktree(t *testing.T) {
	got := parseGitPorcelain("MM both.go\x00A  added.go\x00 D gone.go\x00?? new dir/\x00")
	want := []map[string]string{
		{"path": "both.go", "status": "M", "staged": "true", "index": "M", "worktree": "M"},
		{"path": "added.go", "status": "A", "staged": "true", "index": "A", "worktree": " "},
		{"path": "gone.go", "status": "D", "staged": "false", "index": " ", "worktree": "D"},
		{"path": "new dir/", "status": "??", "staged": "false", "index": "?", "worktree": "?"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("porcelain=%#v; want=%#v", got, want)
	}
}

func TestCodeStudioGitStatusReportsRepositoryState(t *testing.T) {
	s := testCodeStudioServerWithFakeCodeContainer(t, 1)
	type statusBody struct {
		IsRepo  *bool               `json:"is_repo"`
		Branch  string              `json:"branch"`
		Changes []map[string]string `json:"changes"`
	}
	run := func(results ...codeStudioExecResult) (statusBody, *recordingCodeStudioDockerAPI, int) {
		t.Helper()
		docker := &recordingCodeStudioDockerAPI{results: results}
		h := codeStudioHandlers{server: s, docker: docker}
		rec := httptest.NewRecorder()
		h.handleGitStatus(rec, httptest.NewRequest(http.MethodGet, "/api/code-studio/git/status", nil))
		var body statusBody
		if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
			t.Fatalf("status body %q: %v", rec.Body.String(), err)
		}
		return body, docker, rec.Code
	}

	body, docker, code := run(codeStudioExecResult{ExitCode: 128, Output: "fatal: not a git repository (or any of the parent directories): .git\n"})
	if code != http.StatusOK || body.IsRepo == nil || *body.IsRepo || len(body.Changes) != 0 || len(docker.commands) != 1 {
		t.Fatalf("missing repository: code=%d body=%+v commands=%q", code, body, docker.commands)
	}

	body, _, code = run(codeStudioExecResult{ExitCode: 128, Output: "fatal: detected dubious ownership in repository at '/workspace'\n"})
	if code != http.StatusOK || body.IsRepo == nil || !*body.IsRepo {
		t.Fatalf("other git failures must not offer init: code=%d body=%+v", code, body)
	}

	body, docker, code = run(codeStudioExecResult{Output: "MM both.go\x00"}, codeStudioExecResult{Output: "main\n"}, codeStudioExecResult{})
	wantChanges := []map[string]string{{"path": "both.go", "status": "M", "staged": "true", "index": "M", "worktree": "M"}}
	if code != http.StatusOK || body.IsRepo == nil || !*body.IsRepo || body.Branch != "main" || !reflect.DeepEqual(body.Changes, wantChanges) {
		t.Fatalf("repository status: code=%d body=%+v", code, body)
	}
	if len(docker.commands) != 3 || !reflect.DeepEqual(docker.commands[2], codeStudioGitCmd("log", codeStudioGitLogFormat, "-n", "10")) {
		t.Fatalf("status log command=%q", docker.commands)
	}
}

func TestCodeStudioGitLogCarriesAuthorAndDate(t *testing.T) {
	output := "0123456789abcdef0123456789abcdef01234567\x1f0123456\x1fAda Lovelace\x1f2026-11-20T10:00:00+01:00\x1fAdd engine\n" +
		"warning: noise without fields\n" +
		"89abcdef0123456789abcdef0123456789abcdef\x1f89abcde\x1fGrace Hopper\x1f2026-11-19T09:00:00Z\x1fFix: a\x1fb\n"
	want := []map[string]string{
		{"hash": "0123456789abcdef0123456789abcdef01234567", "short": "0123456", "author": "Ada Lovelace", "date": "2026-11-20T10:00:00+01:00", "message": "Add engine"},
		{"hash": "89abcdef0123456789abcdef0123456789abcdef", "short": "89abcde", "author": "Grace Hopper", "date": "2026-11-19T09:00:00Z", "message": "Fix: a\x1fb"},
	}
	if got := parseGitLog(output); !reflect.DeepEqual(got, want) {
		t.Fatalf("log=%#v; want=%#v", got, want)
	}
	if got := parseGitLog(""); got == nil || len(got) != 0 {
		t.Fatalf("empty log=%#v", got)
	}

	s := testCodeStudioServerWithFakeCodeContainer(t, 1)
	docker := &recordingCodeStudioDockerAPI{results: []codeStudioExecResult{{Output: output}}}
	h := codeStudioHandlers{server: s, docker: docker}
	rec := httptest.NewRecorder()
	h.handleGitLog(rec, httptest.NewRequest(http.MethodGet, "/api/code-studio/git/log?count=50&path=/workspace/src/app.js", nil))
	if rec.Code != http.StatusOK || !reflect.DeepEqual(docker.commands, [][]string{codeStudioGitCmd("log", codeStudioGitLogFormat, "-n", "50", "--", "src/app.js")}) {
		t.Fatalf("log status=%d commands=%q body=%s", rec.Code, docker.commands, rec.Body.String())
	}
	var body struct{ Log []map[string]string }
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil || !reflect.DeepEqual(body.Log, want) {
		t.Fatalf("log body=%s err=%v", rec.Body.String(), err)
	}

	docker = &recordingCodeStudioDockerAPI{}
	h.docker = docker
	rec = httptest.NewRecorder()
	h.handleGitLog(rec, httptest.NewRequest(http.MethodGet, "/api/code-studio/git/log?path=../etc/passwd", nil))
	if rec.Code != http.StatusBadRequest || len(docker.commands) != 0 {
		t.Fatalf("escaping log path: status=%d commands=%q", rec.Code, docker.commands)
	}
}

func TestCodeStudioGitBlamePorcelainParser(t *testing.T) {
	commit := strings.Repeat("a", 40)
	zero := strings.Repeat("0", 40)
	porcelain := strings.Join([]string{
		commit + " 1 1 2",
		"author Ada Lovelace",
		"author-mail <ada@example.com>",
		"author-time 1700000000",
		"author-tz +0100",
		"committer Ada Lovelace",
		"committer-mail <ada@example.com>",
		"committer-time 1700000000",
		"committer-tz +0100",
		"summary First commit",
		"boundary",
		"filename src/app.js",
		"\tline one",
		commit + " 2 2",
		"\tline two",
		zero + " 3 3 1",
		"author Not Committed Yet",
		"author-mail <not.committed.yet>",
		"author-time 1700003600",
		"author-tz -0230",
		"committer Not Committed Yet",
		"committer-mail <not.committed.yet>",
		"committer-time 1700003600",
		"committer-tz -0230",
		"summary Version of src/app.js from src/app.js",
		"previous " + commit + " src/app.js",
		"filename src/app.js",
		"\tauthor looks like a key",
		commit + " 4 4 1",
		"\tline four",
		"",
	}, "\n")
	first := codeStudioGitBlameLine{Hash: commit, Short: "aaaaaaa", Author: "Ada Lovelace", Date: "2023-11-14T23:13:20+01:00", Summary: "First commit"}
	lineOf := func(number int, base codeStudioGitBlameLine) codeStudioGitBlameLine {
		base.Line = number
		return base
	}
	want := []codeStudioGitBlameLine{
		lineOf(1, first),
		lineOf(2, first),
		{Line: 3, Hash: zero, Short: "0000000", Author: "Not Committed Yet", Date: "2023-11-14T20:43:20-02:30", Summary: "Version of src/app.js from src/app.js"},
		lineOf(4, first),
	}
	if got := parseGitBlamePorcelain(porcelain); !reflect.DeepEqual(got, want) {
		t.Fatalf("blame=%#v; want=%#v", got, want)
	}
}

func TestCodeStudioGitShowParserAndDiffCap(t *testing.T) {
	full := "0123456789abcdef0123456789abcdef01234567"
	meta := full + "\x1f0123456\x1fAda Lovelace\x1fada@example.com\x1f2026-11-20T10:00:00+01:00\x1fAdd engine\x1fLonger body\nsecond line\n\x1e\nM\tmain.go\nA\tdocs/new file.md\nR100\told.go\tnew.go\nM\t\"tab\\there.txt\"\n"
	got, err := parseGitShowMeta(meta)
	if err != nil {
		t.Fatal(err)
	}
	want := codeStudioGitCommit{
		Hash: full, Short: "0123456", Author: "Ada Lovelace", Email: "ada@example.com",
		Date: "2026-11-20T10:00:00+01:00", Subject: "Add engine", Body: "Longer body\nsecond line",
		Files: []codeStudioGitFileChange{
			{Path: "main.go", Status: "M"},
			{Path: "docs/new file.md", Status: "A"},
			{Path: "new.go", Status: "R"},
			{Path: "tab\there.txt", Status: "M"},
		},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("show=%#v; want=%#v", got, want)
	}
	if _, err := parseGitShowMeta("fatal: bad object\n"); err == nil {
		t.Fatal("unexpected show output must fail")
	}

	big := strings.Repeat("+0123456789abcdef\n", 40000)
	cut, truncated := truncateCodeStudioGitDiff(big, codeStudioGitShowDiffLimit)
	if !truncated || len(cut) > codeStudioGitShowDiffLimit || !strings.HasSuffix(cut, "\n") || !strings.HasPrefix(big, cut) {
		t.Fatalf("cap: truncated=%v len=%d", truncated, len(cut))
	}
	if same, truncated := truncateCodeStudioGitDiff("small\n", codeStudioGitShowDiffLimit); truncated || same != "small\n" {
		t.Fatalf("small diff changed: %q %v", same, truncated)
	}
}
