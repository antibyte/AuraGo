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

func TestCodeStudioGitStageAndUnstageCommands(t *testing.T) {
	s := testCodeStudioServerWithFakeCodeContainer(t, 1)
	docker := &recordingCodeStudioDockerAPI{}
	h := codeStudioHandlers{server: s, docker: docker}
	post := func(handler http.HandlerFunc, route, body string) *httptest.ResponseRecorder {
		t.Helper()
		rec := httptest.NewRecorder()
		handler(rec, httptest.NewRequest(http.MethodPost, route, strings.NewReader(body)))
		return rec
	}

	rec := post(h.handleGitStage, "/api/code-studio/git/stage", `{"paths":["main.go","/workspace/sub/a b.txt","main.go","-n"]}`)
	if rec.Code != http.StatusOK || !reflect.DeepEqual(docker.commands, [][]string{codeStudioGitCmd("add", "--", "main.go", "sub/a b.txt", "-n")}) {
		t.Fatalf("stage status=%d commands=%q body=%s", rec.Code, docker.commands, rec.Body.String())
	}
	for _, body := range []string{`{"paths":[]}`, `{"paths":["../etc/passwd"]}`, `{"paths":["/etc/passwd"]}`, `{"paths":["/workspace"]}`, `not json`} {
		docker = &recordingCodeStudioDockerAPI{}
		h.docker = docker
		if rec := post(h.handleGitStage, "/api/code-studio/git/stage", body); rec.Code != http.StatusBadRequest || len(docker.commands) != 0 {
			t.Fatalf("stage %s: status=%d commands=%q", body, rec.Code, docker.commands)
		}
	}
	rec = httptest.NewRecorder()
	h.handleGitStage(rec, httptest.NewRequest(http.MethodGet, "/api/code-studio/git/stage", nil))
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("GET stage status=%d", rec.Code)
	}

	docker = &recordingCodeStudioDockerAPI{results: []codeStudioExecResult{{Output: "0123456789abcdef0123456789abcdef01234567\n"}}}
	h.docker = docker
	if rec := post(h.handleGitUnstage, "/api/code-studio/git/unstage", `{"paths":["main.go"]}`); rec.Code != http.StatusOK || !reflect.DeepEqual(docker.commands, [][]string{
		codeStudioGitCmd("rev-parse", "--verify", "-q", "HEAD"),
		codeStudioGitCmd("restore", "--staged", "--", "main.go"),
	}) {
		t.Fatalf("unstage status=%d commands=%q", rec.Code, docker.commands)
	}

	docker = &recordingCodeStudioDockerAPI{results: []codeStudioExecResult{{ExitCode: 1}}}
	h.docker = docker
	if rec := post(h.handleGitUnstage, "/api/code-studio/git/unstage", `{"paths":["main.go"]}`); rec.Code != http.StatusOK || !reflect.DeepEqual(docker.commands, [][]string{
		codeStudioGitCmd("rev-parse", "--verify", "-q", "HEAD"),
		codeStudioGitCmd("rm", "--cached", "-r", "-q", "--", "main.go"),
	}) {
		t.Fatalf("unstage without HEAD status=%d commands=%q", rec.Code, docker.commands)
	}
}

func TestCodeStudioGitDiscardSplitsUntrackedFromTracked(t *testing.T) {
	s := testCodeStudioServerWithFakeCodeContainer(t, 1)
	docker := &recordingCodeStudioDockerAPI{results: []codeStudioExecResult{{Output: " M main.go\x00?? new.txt\x00?? build/\x00"}}}
	h := codeStudioHandlers{server: s, docker: docker}
	rec := httptest.NewRecorder()
	h.handleGitDiscard(rec, httptest.NewRequest(http.MethodPost, "/api/code-studio/git/discard", strings.NewReader(`{"paths":["main.go","new.txt","build/"]}`)))
	want := [][]string{
		codeStudioGitCmd("status", "--porcelain=v1", "-z", "--", "main.go", "new.txt", "build"),
		codeStudioGitCmd("clean", "-f", "-q", "--", "new.txt"),
		codeStudioGitCmd("clean", "-f", "-q", "-d", "--", "build"),
		codeStudioGitCmd("restore", "--worktree", "--", "main.go"),
	}
	if rec.Code != http.StatusOK || !reflect.DeepEqual(docker.commands, want) {
		t.Fatalf("discard status=%d commands=%q body=%s", rec.Code, docker.commands, rec.Body.String())
	}

	for _, body := range []string{`{"paths":["/workspace"]}`, `{"paths":["."]}`, `{"paths":["../outside.txt"]}`} {
		docker = &recordingCodeStudioDockerAPI{}
		h.docker = docker
		rec = httptest.NewRecorder()
		h.handleGitDiscard(rec, httptest.NewRequest(http.MethodPost, "/api/code-studio/git/discard", strings.NewReader(body)))
		if rec.Code != http.StatusBadRequest || len(docker.commands) != 0 {
			t.Fatalf("discard %s: status=%d commands=%q", body, rec.Code, docker.commands)
		}
	}

	docker = &recordingCodeStudioDockerAPI{results: []codeStudioExecResult{{Output: " M main.go\x00"}, {ExitCode: 1, Output: "error: pathspec 'main.go' did not match\n"}}}
	h.docker = docker
	rec = httptest.NewRecorder()
	h.handleGitDiscard(rec, httptest.NewRequest(http.MethodPost, "/api/code-studio/git/discard", strings.NewReader(`{"paths":["main.go"]}`)))
	if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), "did not match") {
		t.Fatalf("git error must be returned verbatim: status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestCodeStudioGitInitOnlyOutsideRepository(t *testing.T) {
	s := testCodeStudioServerWithFakeCodeContainer(t, 1)
	probe := codeStudioGitCmd("rev-parse", "--is-inside-work-tree")
	cases := []struct {
		name     string
		result   codeStudioExecResult
		wantCode int
		wantCmds [][]string
	}{
		{"already a repository", codeStudioExecResult{Output: "true\n"}, http.StatusConflict, [][]string{probe}},
		{"no repository", codeStudioExecResult{ExitCode: 128, Output: "fatal: not a git repository (or any of the parent directories): .git\n"}, http.StatusOK, [][]string{probe, codeStudioGitCmd("init")}},
		{"other failure", codeStudioExecResult{ExitCode: 128, Output: "fatal: detected dubious ownership in repository at '/workspace'\n"}, http.StatusBadRequest, [][]string{probe}},
	}
	for _, tc := range cases {
		docker := &recordingCodeStudioDockerAPI{results: []codeStudioExecResult{tc.result}}
		h := codeStudioHandlers{server: s, docker: docker}
		rec := httptest.NewRecorder()
		h.handleGitInit(rec, httptest.NewRequest(http.MethodPost, "/api/code-studio/git/init", nil))
		if rec.Code != tc.wantCode || !reflect.DeepEqual(docker.commands, tc.wantCmds) {
			t.Fatalf("%s: status=%d commands=%q body=%s", tc.name, rec.Code, docker.commands, rec.Body.String())
		}
	}
}

func TestCodeStudioGitWorkbenchWritesRespectReadOnly(t *testing.T) {
	s := testCodeStudioServerWithFakeCodeContainer(t, 1)
	s.CfgMu.Lock()
	s.Cfg.VirtualDesktop.ReadOnly = true
	s.CfgMu.Unlock()
	docker := &recordingCodeStudioDockerAPI{}
	h := codeStudioHandlers{server: s, docker: docker}
	for route, handler := range map[string]http.HandlerFunc{
		"stage": h.handleGitStage, "unstage": h.handleGitUnstage, "discard": h.handleGitDiscard, "init": h.handleGitInit,
	} {
		rec := httptest.NewRecorder()
		handler(rec, httptest.NewRequest(http.MethodPost, "/api/code-studio/git/"+route, strings.NewReader(`{"paths":["main.go"]}`)))
		if rec.Code != http.StatusForbidden {
			t.Fatalf("read-only %s: status=%d body=%s", route, rec.Code, rec.Body.String())
		}
	}
	if len(docker.commands) != 0 {
		t.Fatalf("read-only writes ran git: %q", docker.commands)
	}
}

func TestCodeStudioGitShowHandler(t *testing.T) {
	s := testCodeStudioServerWithFakeCodeContainer(t, 1)
	for _, query := range []string{"", "abc", "HEAD", "-p", "zzzz", "abcd%3Bid", "0123456789abcdef0123456789abcdef012345678"} {
		docker := &recordingCodeStudioDockerAPI{}
		h := codeStudioHandlers{server: s, docker: docker}
		rec := httptest.NewRecorder()
		h.handleGitShow(rec, httptest.NewRequest(http.MethodGet, "/api/code-studio/git/show?hash="+query, nil))
		if rec.Code != http.StatusBadRequest || len(docker.commands) != 0 {
			t.Fatalf("hash %q: status=%d commands=%q", query, rec.Code, docker.commands)
		}
	}

	full := "0123456789abcdef0123456789abcdef01234567"
	meta := full + "\x1f0123456\x1fAda Lovelace\x1fada@example.com\x1f2026-11-20T10:00:00+01:00\x1fAdd engine\x1fLonger body\n\x1e\nM\tmain.go\n"
	docker := &recordingCodeStudioDockerAPI{results: []codeStudioExecResult{{Output: meta}, {Output: "\ndiff --git a/main.go b/main.go\n+new\n"}}}
	h := codeStudioHandlers{server: s, docker: docker}
	rec := httptest.NewRecorder()
	h.handleGitShow(rec, httptest.NewRequest(http.MethodGet, "/api/code-studio/git/show?hash=0123456", nil))
	wantCommands := [][]string{
		codeStudioGitCmd("-c", "core.quotePath=false", "show", "--no-color", "--name-status", codeStudioGitShowFormat, "0123456", "--"),
		codeStudioGitCmd("show", "--no-color", "--format=", "--patch", "0123456", "--"),
	}
	if rec.Code != http.StatusOK || !reflect.DeepEqual(docker.commands, wantCommands) {
		t.Fatalf("show status=%d commands=%q body=%s", rec.Code, docker.commands, rec.Body.String())
	}
	var body struct {
		Hash      string
		Short     string
		Email     string
		Subject   string
		Body      string
		Diff      string
		Files     []codeStudioGitFileChange
		Truncated bool
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Hash != full || body.Short != "0123456" || body.Email != "ada@example.com" || body.Subject != "Add engine" || body.Body != "Longer body" ||
		body.Diff != "diff --git a/main.go b/main.go\n+new\n" || body.Truncated || !reflect.DeepEqual(body.Files, []codeStudioGitFileChange{{Path: "main.go", Status: "M"}}) {
		t.Fatalf("show body=%+v", body)
	}

	docker = &recordingCodeStudioDockerAPI{results: []codeStudioExecResult{{ExitCode: 128, Output: "fatal: bad object 0123456\n"}}}
	h.docker = docker
	rec = httptest.NewRecorder()
	h.handleGitShow(rec, httptest.NewRequest(http.MethodGet, "/api/code-studio/git/show?hash=0123456", nil))
	if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), "bad object") || len(docker.commands) != 1 {
		t.Fatalf("unknown commit: status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestCodeStudioGitBlameHandler(t *testing.T) {
	s := testCodeStudioServerWithFakeCodeContainer(t, 1)
	for _, query := range []string{"", "../etc/passwd", "/etc/passwd", "/workspace"} {
		docker := &recordingCodeStudioDockerAPI{}
		h := codeStudioHandlers{server: s, docker: docker}
		rec := httptest.NewRecorder()
		h.handleGitBlame(rec, httptest.NewRequest(http.MethodGet, "/api/code-studio/git/blame?path="+query, nil))
		if rec.Code != http.StatusBadRequest || len(docker.commands) != 0 {
			t.Fatalf("blame path %q: status=%d commands=%q", query, rec.Code, docker.commands)
		}
	}

	commit := strings.Repeat("b", 40)
	porcelain := commit + " 1 1 1\nauthor Ada Lovelace\nauthor-time 1700000000\nauthor-tz +0000\nsummary First commit\nfilename src/app.js\n\tline one\n"
	docker := &recordingCodeStudioDockerAPI{results: []codeStudioExecResult{{Output: porcelain}}}
	h := codeStudioHandlers{server: s, docker: docker}
	rec := httptest.NewRecorder()
	h.handleGitBlame(rec, httptest.NewRequest(http.MethodGet, "/api/code-studio/git/blame?path=/workspace/src/app.js", nil))
	if rec.Code != http.StatusOK || !reflect.DeepEqual(docker.commands, [][]string{codeStudioGitCmd("blame", "--porcelain", "--", "src/app.js")}) {
		t.Fatalf("blame status=%d commands=%q body=%s", rec.Code, docker.commands, rec.Body.String())
	}
	var body struct{ Lines []codeStudioGitBlameLine }
	want := []codeStudioGitBlameLine{{Line: 1, Hash: commit, Short: "bbbbbbb", Author: "Ada Lovelace", Date: "2023-11-14T22:13:20Z", Summary: "First commit"}}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil || !reflect.DeepEqual(body.Lines, want) {
		t.Fatalf("blame body=%s err=%v", rec.Body.String(), err)
	}

	docker = &recordingCodeStudioDockerAPI{results: []codeStudioExecResult{{ExitCode: 128, Output: "fatal: no such path 'src/new.js' in HEAD\n"}}}
	h.docker = docker
	rec = httptest.NewRecorder()
	h.handleGitBlame(rec, httptest.NewRequest(http.MethodGet, "/api/code-studio/git/blame?path=src/new.js", nil))
	if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), "no such path") {
		t.Fatalf("blame failure: status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestCodeStudioGitCommitStagedOnlyAndBranchDelete(t *testing.T) {
	s := testCodeStudioServerWithFakeCodeContainer(t, 1)
	docker := &recordingCodeStudioDockerAPI{results: []codeStudioExecResult{{}, {Output: "abc1234\n"}}}
	h := codeStudioHandlers{server: s, docker: docker}
	rec := httptest.NewRecorder()
	h.handleGitCommit(rec, httptest.NewRequest(http.MethodPost, "/api/code-studio/git/commit", strings.NewReader(`{"message":"index only","staged_only":true,"paths":["main.go"]}`)))
	if rec.Code != http.StatusOK || !reflect.DeepEqual(docker.commands, [][]string{
		codeStudioGitCmd("commit", "-m", "index only"),
		codeStudioGitCmd("rev-parse", "--short", "HEAD"),
	}) {
		t.Fatalf("staged-only commit status=%d commands=%q", rec.Code, docker.commands)
	}

	docker = &recordingCodeStudioDockerAPI{}
	h.docker = docker
	rec = httptest.NewRecorder()
	h.handleGitCommit(rec, httptest.NewRequest(http.MethodPost, "/api/code-studio/git/commit", strings.NewReader(`{"message":"both","staged_only":true,"add_all":true}`)))
	if rec.Code != http.StatusBadRequest || len(docker.commands) != 0 {
		t.Fatalf("staged_only with add_all: status=%d commands=%q", rec.Code, docker.commands)
	}

	docker = &recordingCodeStudioDockerAPI{results: []codeStudioExecResult{{Output: "main\n"}, {}, {Output: "* main\n"}, {Output: "main\n"}}}
	h.docker = docker
	rec = httptest.NewRecorder()
	h.handleGitBranch(rec, httptest.NewRequest(http.MethodPost, "/api/code-studio/git/branch", strings.NewReader(`{"action":"delete","name":"feature/old"}`)))
	if rec.Code != http.StatusOK || len(docker.commands) < 2 || !reflect.DeepEqual(docker.commands[:2], [][]string{
		codeStudioGitCmd("branch", "--show-current"),
		codeStudioGitCmd("branch", "-d", "feature/old"),
	}) {
		t.Fatalf("branch delete status=%d commands=%q body=%s", rec.Code, docker.commands, rec.Body.String())
	}

	docker = &recordingCodeStudioDockerAPI{results: []codeStudioExecResult{{Output: "main\n"}}}
	h.docker = docker
	rec = httptest.NewRecorder()
	h.handleGitBranch(rec, httptest.NewRequest(http.MethodPost, "/api/code-studio/git/branch", strings.NewReader(`{"action":"delete","name":"main"}`)))
	if rec.Code != http.StatusConflict || len(docker.commands) != 1 {
		t.Fatalf("current branch delete: status=%d commands=%q", rec.Code, docker.commands)
	}

	docker = &recordingCodeStudioDockerAPI{}
	h.docker = docker
	rec = httptest.NewRecorder()
	h.handleGitBranch(rec, httptest.NewRequest(http.MethodGet, "/api/code-studio/git/branch?action=delete&name=feature/old", nil))
	if rec.Code != http.StatusMethodNotAllowed || len(docker.commands) != 0 {
		t.Fatalf("GET delete: status=%d commands=%q", rec.Code, docker.commands)
	}
}
