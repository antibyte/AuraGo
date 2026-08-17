package server

import (
	"bufio"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"aurago/internal/config"
)

func testDesktopLogServer(t *testing.T, logDir string) *Server {
	t.Helper()
	cfg := &config.Config{}
	cfg.Logging.LogDir = logDir
	return &Server{Cfg: cfg}
}

func writeDesktopLogFixture(t *testing.T, dir, name, content string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
	return path
}

func TestParseSlogLineExtractsFieldsAndAttrs(t *testing.T) {
	rec, ok := parseSlogLine(`time=2026-08-16T20:00:00.000Z level=INFO msg="Loading environment" path=.env table=users`)
	if !ok {
		t.Fatal("expected slog line to parse")
	}
	if rec.Time != "2026-08-16T20:00:00.000Z" || rec.Level != "INFO" || rec.Msg != "Loading environment" {
		t.Fatalf("parsed core fields = %+v", rec)
	}
	if rec.Attrs["path"] != ".env" || rec.Attrs["table"] != "users" {
		t.Fatalf("parsed attrs = %#v", rec.Attrs)
	}
}

func TestParseSlogLineBestEffortForAccessLog(t *testing.T) {
	raw := `127.0.0.1 - GET /api/desktop/apps 200`
	rec, ok := parseSlogLine(raw)
	if ok {
		t.Fatal("access log should not count as structured slog")
	}
	if rec.Raw != raw || rec.Level != "" {
		t.Fatalf("best-effort record = %+v", rec)
	}
}

func TestDesktopLogFilesListsDirAndSkipsMissing(t *testing.T) {
	dir := t.TempDir()
	writeDesktopLogFixture(t, dir, "aurago.log", "time=2026-08-16T20:00:00Z level=INFO msg=\"boot\"\n")
	writeDesktopLogFixture(t, dir, "web_access.log", "GET / 200\n")
	if err := os.Mkdir(filepath.Join(dir, "nested"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	s := testDesktopLogServer(t, dir)
	req := httptest.NewRequest(http.MethodGet, "/api/desktop/logs/files", nil)
	rec := httptest.NewRecorder()
	handleDesktopLogFiles(s).ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("files status = %d body %s", rec.Code, rec.Body.String())
	}
	var body struct {
		Files  []desktopLogFileInfo `json:"files"`
		LogDir string               `json:"log_dir"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body.LogDir != dir {
		t.Fatalf("log_dir = %q, want %q", body.LogDir, dir)
	}
	if len(body.Files) != 2 {
		t.Fatalf("files = %#v, want 2 regular files", body.Files)
	}

	missing := testDesktopLogServer(t, filepath.Join(dir, "missing-dir"))
	emptyRec := httptest.NewRecorder()
	handleDesktopLogFiles(missing).ServeHTTP(emptyRec, httptest.NewRequest(http.MethodGet, "/api/desktop/logs/files", nil))
	if emptyRec.Code != http.StatusOK {
		t.Fatalf("missing dir status = %d", emptyRec.Code)
	}
	var emptyBody struct {
		Files []desktopLogFileInfo `json:"files"`
	}
	if err := json.Unmarshal(emptyRec.Body.Bytes(), &emptyBody); err != nil {
		t.Fatalf("decode empty: %v", err)
	}
	if len(emptyBody.Files) != 0 {
		t.Fatalf("missing dir files = %#v", emptyBody.Files)
	}
}

func TestDesktopLogTailForwardAndFromEnd(t *testing.T) {
	dir := t.TempDir()
	var b strings.Builder
	for i := 1; i <= 8; i++ {
		b.WriteString("time=2026-08-16T20:00:0")
		b.WriteByte(byte('0' + i))
		b.WriteString(`Z level=INFO msg="line `)
		b.WriteByte(byte('0' + i))
		b.WriteString("\"\n")
	}
	writeDesktopLogFixture(t, dir, "aurago.log", b.String())
	s := testDesktopLogServer(t, dir)

	forward := httptest.NewRecorder()
	handleDesktopLogTail(s).ServeHTTP(forward, httptest.NewRequest(http.MethodGet, "/api/desktop/logs/tail?file=aurago.log&lines=3&offset=0", nil))
	if forward.Code != http.StatusOK {
		t.Fatalf("forward tail status = %d body %s", forward.Code, forward.Body.String())
	}
	var forwardBody struct {
		Lines []desktopLogRecord `json:"lines"`
		EOF   int64              `json:"eof_offset"`
	}
	if err := json.Unmarshal(forward.Body.Bytes(), &forwardBody); err != nil {
		t.Fatalf("decode forward: %v", err)
	}
	if len(forwardBody.Lines) != 3 || forwardBody.Lines[0].LineNo != 1 || !strings.Contains(forwardBody.Lines[0].Msg, "line 1") {
		t.Fatalf("forward lines = %#v", forwardBody.Lines)
	}

	fromEnd := httptest.NewRecorder()
	handleDesktopLogTail(s).ServeHTTP(fromEnd, httptest.NewRequest(http.MethodGet, "/api/desktop/logs/tail?file=aurago.log&lines=2", nil))
	if fromEnd.Code != http.StatusOK {
		t.Fatalf("from-end tail status = %d body %s", fromEnd.Code, fromEnd.Body.String())
	}
	var endBody struct {
		Lines []desktopLogRecord `json:"lines"`
	}
	if err := json.Unmarshal(fromEnd.Body.Bytes(), &endBody); err != nil {
		t.Fatalf("decode end: %v", err)
	}
	if len(endBody.Lines) != 2 || !strings.Contains(endBody.Lines[1].Msg, "line 8") {
		t.Fatalf("from-end lines = %#v", endBody.Lines)
	}
}

func TestDesktopLogRejectsPathTraversal(t *testing.T) {
	dir := t.TempDir()
	writeDesktopLogFixture(t, dir, "aurago.log", "ok\n")
	s := testDesktopLogServer(t, dir)
	for _, file := range []string{"../aurago.log", `..\aurago.log`, "/tmp/aurago.log", "nested/secret.log"} {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/api/desktop/logs/tail?file="+file, nil)
		handleDesktopLogTail(s).ServeHTTP(rec, req)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("file %q status = %d, want 400 body %s", file, rec.Code, rec.Body.String())
		}
	}
}

func TestDesktopLogUnknownFileRejected(t *testing.T) {
	dir := t.TempDir()
	writeDesktopLogFixture(t, dir, "aurago.log", "ok\n")
	s := testDesktopLogServer(t, dir)
	rec := httptest.NewRecorder()
	handleDesktopLogTail(s).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/desktop/logs/tail?file=secret.txt", nil))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("unknown file status = %d, want 400", rec.Code)
	}
}

func TestDesktopLogSearchMatchesAndLevels(t *testing.T) {
	dir := t.TempDir()
	writeDesktopLogFixture(t, dir, "aurago.log", strings.Join([]string{
		`time=2026-08-16T20:00:01Z level=INFO msg="boot complete"`,
		`time=2026-08-16T20:00:02Z level=ERROR msg="disk full" path=/data`,
		`time=2026-08-16T20:00:03Z level=WARN msg="retry later"`,
	}, "\n")+"\n")
	s := testDesktopLogServer(t, dir)
	rec := httptest.NewRecorder()
	handleDesktopLogSearch(s).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/desktop/logs/search?file=aurago.log&q=disk&levels=ERROR", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("search status = %d body %s", rec.Code, rec.Body.String())
	}
	var body struct {
		Matches []struct {
			LineNo int                `json:"line_no"`
			Record desktopLogRecord   `json:"record"`
			Before []desktopLogRecord `json:"before"`
		} `json:"matches"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(body.Matches) != 1 || body.Matches[0].LineNo != 2 || body.Matches[0].Record.Level != "ERROR" {
		t.Fatalf("matches = %#v", body.Matches)
	}
	if len(body.Matches[0].Before) != 1 || !strings.Contains(body.Matches[0].Before[0].Msg, "boot") {
		t.Fatalf("before context = %#v", body.Matches[0].Before)
	}
}

func TestDesktopLogDownloadContentTypeAndReadonly(t *testing.T) {
	dir := t.TempDir()
	writeDesktopLogFixture(t, dir, "aurago.log", "hello-log\n")
	s := testDesktopLogServer(t, dir)
	rec := httptest.NewRecorder()
	handleDesktopLogDownload(s).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/desktop/logs/download?file=aurago.log", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("download status = %d body %s", rec.Code, rec.Body.String())
	}
	if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "text/plain") {
		t.Fatalf("Content-Type = %q", ct)
	}
	if disp := rec.Header().Get("Content-Disposition"); !strings.Contains(disp, "attachment") || !strings.Contains(disp, "aurago.log") {
		t.Fatalf("Content-Disposition = %q", disp)
	}
	if rec.Body.String() != "hello-log\n" {
		t.Fatalf("download body = %q", rec.Body.String())
	}

	s.Cfg.VirtualDesktop.ReadOnly = true
	blocked := httptest.NewRecorder()
	handleDesktopLogDownload(s).ServeHTTP(blocked, httptest.NewRequest(http.MethodGet, "/api/desktop/logs/download?file=aurago.log", nil))
	if blocked.Code != http.StatusForbidden {
		t.Fatalf("readonly download status = %d, want 403", blocked.Code)
	}
}

func TestDesktopLogStreamEmitsLineAndTruncation(t *testing.T) {
	dir := t.TempDir()
	path := writeDesktopLogFixture(t, dir, "aurago.log", "time=2026-08-16T20:00:00Z level=INFO msg=\"start\"\n")
	s := testDesktopLogServer(t, dir)
	srv := httptest.NewServer(handleDesktopLogStream(s))
	defer srv.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, srv.URL+"?file=aurago.log&since="+strconv.FormatInt(info.Size(), 10), nil)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("do: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("stream status = %d", resp.StatusCode)
	}
	if ct := resp.Header.Get("Content-Type"); !strings.Contains(ct, "text/event-stream") {
		t.Fatalf("Content-Type = %q", ct)
	}

	reader := bufio.NewReader(resp.Body)
	events := make(chan string, 16)
	go func() {
		for {
			line, readErr := reader.ReadString('\n')
			if readErr != nil {
				return
			}
			line = strings.TrimSpace(line)
			if strings.HasPrefix(line, "data: ") {
				select {
				case events <- strings.TrimPrefix(line, "data: "):
				case <-ctx.Done():
					return
				}
			}
		}
	}()

	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatalf("open append: %v", err)
	}
	if _, err := f.WriteString("time=2026-08-16T20:00:01Z level=WARN msg=\"appended\"\n"); err != nil {
		t.Fatalf("append: %v", err)
	}
	_ = f.Close()

	gotLine := waitDesktopLogEvent(t, events, "log_line", 3*time.Second)
	if !strings.Contains(gotLine, "appended") {
		t.Fatalf("log_line payload = %s", gotLine)
	}

	if err := os.WriteFile(path, []byte("time=2026-08-16T20:01:00Z level=INFO msg=\"reset\"\n"), 0o644); err != nil {
		t.Fatalf("truncate rewrite: %v", err)
	}
	gotTrunc := waitDesktopLogEvent(t, events, "log_truncated", 3*time.Second)
	if !strings.Contains(gotTrunc, "log_truncated") {
		t.Fatalf("truncation event = %s", gotTrunc)
	}
}

func waitDesktopLogEvent(t *testing.T, events <-chan string, typ string, timeout time.Duration) string {
	t.Helper()
	deadline := time.After(timeout)
	for {
		select {
		case <-deadline:
			t.Fatalf("timed out waiting for %s", typ)
			return ""
		case msg := <-events:
			if strings.Contains(msg, `"type":"`+typ+`"`) {
				return msg
			}
		}
	}
}
