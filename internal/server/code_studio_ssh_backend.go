package server

import (
	"bytes"
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	pathpkg "path"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/pkg/sftp"
)

var errCodeStudioOutsideRoot = errors.New("path is outside the workspace")

// normalizeCodeStudioRemoteRoot accepts absolute POSIX paths and ~ / ~/x (home = sftp Getwd).
func normalizeCodeStudioRemoteRoot(raw, home string) (string, error) {
	p := strings.TrimSpace(strings.ReplaceAll(raw, "\\", "/"))
	if strings.ContainsRune(p, 0) || p == "" {
		return "", errors.New("remote path is required")
	}
	if p == "~" || strings.HasPrefix(p, "~/") {
		if home == "" {
			return "", errors.New("remote home directory is unknown")
		}
		p = home + strings.TrimPrefix(p, "~")
	}
	if !strings.HasPrefix(p, "/") {
		return "", errors.New("remote path must be absolute")
	}
	return pathpkg.Clean(p), nil
}

func codeStudioRemoteInside(root, p string) bool {
	root, p = pathpkg.Clean(root), pathpkg.Clean(p)
	if root == "/" {
		return strings.HasPrefix(p, "/")
	}
	return p == root || strings.HasPrefix(p, root+"/")
}

// codeStudioRemotePath maps /workspace/<rel> to <root>/<rel>.
func codeStudioRemotePath(root, vpath string) (string, error) {
	v, err := sanitizeCodeStudioPath(vpath)
	if err != nil {
		return "", err
	}
	rel := strings.TrimPrefix(strings.TrimPrefix(v, codeStudioWorkspaceRoot), "/")
	if rel == "" {
		return pathpkg.Clean(root), nil
	}
	return pathpkg.Join(root, rel), nil
}

// codeStudioVirtualPath maps <root>/<rel> back to /workspace/<rel>.
func codeStudioVirtualPath(root, remote string) (string, error) {
	root, remote = pathpkg.Clean(root), pathpkg.Clean(remote)
	if !codeStudioRemoteInside(root, remote) {
		return "", errCodeStudioOutsideRoot
	}
	rel := strings.TrimPrefix(strings.TrimPrefix(remote, root), "/")
	if rel == "" {
		return codeStudioWorkspaceRoot, nil
	}
	return codeStudioWorkspaceRoot + "/" + rel, nil
}

func sortCodeStudioFileEntries(entries []codeStudioFileEntry) {
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].Type != entries[j].Type {
			return entries[i].Type == "directory"
		}
		return strings.ToLower(entries[i].Name) < strings.ToLower(entries[j].Name)
	})
}

// codeStudioSSHCommandLine wraps a POSIX script for the user's login shell. A fish
// login shell can alter "\\" inside the single quotes; documented limitation.
func codeStudioSSHCommandLine(script string) string { return "sh -c " + shellQuote(script) }

const codeStudioCapabilitiesScript = `printf 'os=%s\n' "$(uname -s 2>/dev/null)"
printf 'arch=%s\n' "$(uname -m 2>/dev/null)"
printf 'shell=%s\n' "${SHELL:-}"
for t in git grep gofmt gopls pyright-langserver typescript-language-server clangd python3; do
  if command -v "$t" >/dev/null 2>&1; then printf 'has=%s\n' "$t"; fi
done`

func parseCodeStudioCapabilities(output string, now time.Time) codeStudioCapabilities {
	caps := codeStudioCapabilities{Formatters: map[string]bool{"gofmt": false}, ProbedAt: now}
	for _, line := range strings.Split(output, "\n") {
		key, value, ok := strings.Cut(strings.TrimSpace(line), "=")
		if !ok {
			continue
		}
		switch key {
		case "os":
			caps.OS = value
		case "arch":
			caps.Arch = value
		case "shell":
			caps.Shell = value
		case "has":
			switch value {
			case "git":
				caps.Git = true
			case "grep":
				caps.Grep = true
			case "python3":
				caps.Python3 = true
			case "gofmt":
				caps.Formatters["gofmt"] = true
			case "gopls":
				caps.LSP.Go = true
			case "pyright-langserver":
				caps.LSP.Python = true
			case "typescript-language-server":
				caps.LSP.TypeScript = true
			case "clangd":
				caps.LSP.CPP = true
			}
		}
	}
	return caps
}

// codeStudioSSHGrepArgs: --null (not -Z: BSD grep reads -Z as decompress), -H for single files.
func codeStudioSSHGrepArgs(opts codeStudioSearchOptions, dir string) []string {
	args := []string{"grep", "-r", "-n", "-H", "-I", "-s", "--null"}
	if opts.Regex {
		args = append(args, "-E")
	} else {
		args = append(args, "-F")
	}
	if !opts.CaseSensitive {
		args = append(args, "-i")
	}
	if opts.WholeWord {
		args = append(args, "-w")
	}
	if inc := sanitizeCodeStudioGlob(opts.Include); inc != "" {
		args = append(args, "--include="+inc)
	}
	if exc := sanitizeCodeStudioExclude(opts.Exclude); exc != "" {
		args = append(args, "--exclude="+exc, "--exclude-dir="+exc)
	}
	return append(args, "-m", "201", "-e", opts.Query, "--", dir)
}

// rewriteCodeStudioGrepPaths maps "remote\0rest\n" records to virtual paths; records
// outside root or incomplete are dropped.
func rewriteCodeStudioGrepPaths(root, output string) string {
	var b strings.Builder
	for output != "" {
		name, rest, ok := strings.Cut(output, "\x00")
		if !ok {
			break
		}
		line, tail, ended := strings.Cut(rest, "\n")
		output = tail
		if !ended {
			break
		}
		if v, err := codeStudioVirtualPath(root, name); err == nil {
			b.WriteString(v + "\x00" + line + "\n")
		}
	}
	return b.String()
}

type codeStudioSSHBackend struct {
	rec    codeStudioWorkspaceRecord
	root   string
	pool   *codeStudioSSHPool
	target codeStudioSSHTarget
}

var _ codeStudioBackend = (*codeStudioSSHBackend)(nil)

func newCodeStudioSSHBackend(rec codeStudioWorkspaceRecord, pool *codeStudioSSHPool, target codeStudioSSHTarget) *codeStudioSSHBackend {
	return &codeStudioSSHBackend{rec: rec, root: pathpkg.Clean(rec.Root), pool: pool, target: target}
}

func codeStudioSSHWorkspaceInfo(rec codeStudioWorkspaceRecord, target codeStudioSSHTarget, connected bool) codeStudioWorkspaceInfo {
	info := codeStudioWorkspaceInfo{ID: rec.ID, Kind: codeStudioWorkspaceKindSSH, Label: rec.Label, Root: rec.Root,
		DeviceID: rec.DeviceID, DeviceName: target.Name, Host: target.Host, Port: target.Port, User: target.User,
		Connected: connected, Capabilities: rec.Capabilities}
	if !rec.CreatedAt.IsZero() {
		created := rec.CreatedAt
		info.CreatedAt = &created
	}
	if !rec.LastUsedAt.IsZero() {
		used := rec.LastUsedAt
		info.LastUsedAt = &used
	}
	return info
}

func (b *codeStudioSSHBackend) Info() codeStudioWorkspaceInfo {
	_, connected := b.pool.connected(b.rec.DeviceID)
	return codeStudioSSHWorkspaceInfo(b.rec, b.target, connected)
}

func (b *codeStudioSSHBackend) remote(vpath string) (string, error) {
	p, err := codeStudioRemotePath(b.root, vpath)
	if err != nil {
		return "", codeStudioBadRequest(err.Error())
	}
	return p, nil
}

func (b *codeStudioSSHBackend) sftpDo(ctx context.Context, retry bool, fn func(*codeStudioSSHConn) error) error {
	var retryable func(error) bool
	if retry {
		retryable = codeStudioSSHRetryAlways
	}
	return b.pool.withConn(ctx, b.rec.DeviceID, retryable, fn)
}

func codeStudioSFTPNotExist(err error) bool {
	var status *sftp.StatusError
	return errors.Is(err, os.ErrNotExist) || (errors.As(err, &status) && status.Code == uint32(sftp.ErrSSHFxNoSuchFile))
}

func codeStudioSFTPError(err error, what string) error {
	var status *sftp.StatusError
	switch {
	case err == nil:
		return nil
	case codeStudioSSHBroken(err):
		return &codeStudioRemoteError{Status: http.StatusBadGateway, Code: "ssh_connection_lost", Message: "the SSH connection was lost", Err: err}
	case codeStudioSFTPNotExist(err):
		return codeStudioBadRequest(what + ": no such file or directory")
	case errors.Is(err, os.ErrPermission):
		return codeStudioBadRequest(what + ": permission denied")
	case errors.Is(err, os.ErrExist):
		return codeStudioExistsError(what + ": already exists")
	case errors.As(err, &status):
		return codeStudioBadRequest(what + ": " + status.Error())
	}
	return err
}

func (b *codeStudioSSHBackend) realRoot(c *codeStudioSSHConn) (string, error) {
	c.rootsMu.Lock()
	defer c.rootsMu.Unlock()
	if real, ok := c.realRoots[b.root]; ok {
		return real, nil
	}
	real, err := c.sftp.RealPath(b.root)
	if err != nil {
		return "", codeStudioSFTPError(err, "workspace root")
	}
	c.realRoots[b.root] = pathpkg.Clean(real)
	return c.realRoots[b.root], nil
}

// confine is a UX guard, NOT a security boundary: the same SSH user has a full shell in
// the terminal and exec. It stops editor operations from silently following a symlink
// out of the workspace. The nearest existing ancestor is resolved with RealPath.
func (b *codeStudioSSHBackend) confine(c *codeStudioSSHConn, remote string) error {
	realRoot, err := b.realRoot(c)
	if err != nil {
		return err
	}
	for p := remote; ; p = pathpkg.Dir(p) {
		real, err := c.sftp.RealPath(p)
		if err == nil {
			if !codeStudioRemoteInside(realRoot, real) {
				return codeStudioProbeRejection{message: "path is outside the workspace"}
			}
			return nil
		}
		if !codeStudioSFTPNotExist(err) {
			return codeStudioSFTPError(err, p)
		}
		if p == b.root || p == "/" {
			return nil
		}
	}
}

func (b *codeStudioSSHBackend) List(ctx context.Context, vdir string) ([]codeStudioFileEntry, error) {
	dir, err := b.remote(vdir)
	if err != nil {
		return nil, err
	}
	var infos []os.FileInfo
	err = b.sftpDo(ctx, true, func(c *codeStudioSSHConn) error {
		if err := b.confine(c, dir); err != nil {
			return err
		}
		list, err := c.sftp.ReadDir(dir)
		infos = list
		return codeStudioSFTPError(err, vdir)
	})
	if err != nil {
		return nil, err
	}
	entries := make([]codeStudioFileEntry, 0, len(infos))
	for _, info := range infos {
		kind := "file" // ReadDir is lstat-based: a symlinked dir is a file, as locally
		if info.IsDir() {
			kind = "directory"
		}
		entries = append(entries, codeStudioFileEntry{Name: info.Name(), Path: pathpkg.Join(vdir, info.Name()),
			Type: kind, Size: info.Size(), Modified: info.ModTime().UTC()})
	}
	sortCodeStudioFileEntries(entries)
	return entries, nil
}

func (b *codeStudioSSHBackend) Probe(ctx context.Context, vpath string) (codeStudioFileProbe, error) {
	target, err := b.remote(vpath)
	if err != nil {
		return codeStudioFileProbe{}, codeStudioProbeRejection{message: err.Error()}
	}
	unavailable := codeStudioProbeRejection{message: "file is outside the workspace or unavailable"}
	var probe codeStudioFileProbe
	err = b.sftpDo(ctx, true, func(c *codeStudioSSHConn) error {
		realRoot, err := b.realRoot(c)
		if err != nil {
			return err
		}
		real, err := c.sftp.RealPath(target)
		if err != nil {
			if codeStudioSFTPNotExist(err) {
				return unavailable
			}
			return codeStudioSFTPError(err, vpath)
		}
		virtual, err := codeStudioVirtualPath(realRoot, real)
		if err != nil {
			return unavailable
		}
		info, err := c.sftp.Stat(real)
		if err != nil {
			if codeStudioSFTPNotExist(err) {
				return unavailable
			}
			return codeStudioSFTPError(err, vpath)
		}
		if !info.Mode().IsRegular() {
			return codeStudioProbeRejection{message: "not a regular file"}
		}
		probe = codeStudioFileProbe{Path: virtual, Size: info.Size(), Modified: info.ModTime().UTC()}
		return nil
	})
	return probe, err
}

func (b *codeStudioSSHBackend) Read(ctx context.Context, vpath string, max int64) ([]byte, error) {
	target, err := b.remote(vpath)
	if err != nil {
		return nil, err
	}
	var data []byte
	err = b.sftpDo(ctx, true, func(c *codeStudioSSHConn) error {
		if err := b.confine(c, target); err != nil {
			return err
		}
		f, err := c.sftp.Open(target)
		if err != nil {
			return codeStudioSFTPError(err, vpath)
		}
		defer f.Close()
		if data, err = io.ReadAll(io.LimitReader(f, max+1)); err != nil {
			return codeStudioSFTPError(err, vpath)
		}
		if int64(len(data)) > max {
			return &codeStudioStatusError{Status: http.StatusRequestEntityTooLarge, Message: "file exceeds configured maximum size"}
		}
		return nil
	})
	return data, err
}

func (b *codeStudioSSHBackend) Write(ctx context.Context, vpath string, content []byte, createOnly bool) error {
	target, err := b.remote(vpath)
	if err != nil {
		return err
	}
	if target == b.root {
		return codeStudioBadRequest("cannot write the workspace root")
	}
	parent := pathpkg.Dir(target)
	return b.sftpDo(ctx, !createOnly, func(c *codeStudioSSHConn) error {
		if err := b.confine(c, parent); err != nil {
			return err
		}
		if err := c.sftp.MkdirAll(parent); err != nil {
			return codeStudioSFTPError(err, vpath)
		}
		if err := b.confine(c, parent); err != nil {
			return err
		}
		mode, existed := os.FileMode(0o644), false
		info, err := c.sftp.Lstat(target)
		switch {
		case err == nil:
			if createOnly {
				return codeStudioExistsError("file already exists")
			}
			if !info.Mode().IsRegular() {
				return codeStudioBadRequest("target must be a regular file, not a symlink or directory")
			}
			mode, existed = info.Mode().Perm(), true
		case !codeStudioSFTPNotExist(err):
			return codeStudioSFTPError(err, vpath)
		}
		if createOnly {
			return codeStudioSFTPWriteExclusive(c.sftp, target, content, mode)
		}
		return codeStudioSFTPReplace(c.sftp, parent, target, content, mode, existed)
	})
}

func codeStudioSFTPWriteExclusive(client *sftp.Client, target string, content []byte, mode os.FileMode) error {
	f, err := client.OpenFile(target, os.O_WRONLY|os.O_CREATE|os.O_EXCL)
	if err != nil {
		if errors.Is(err, os.ErrExist) {
			return codeStudioExistsError("file already exists")
		}
		return codeStudioSFTPError(err, target)
	}
	if _, err := f.Write(content); err != nil {
		_ = f.Close()
		_ = client.Remove(target)
		return codeStudioSFTPError(err, target)
	}
	if err := f.Close(); err != nil {
		_ = client.Remove(target)
		return codeStudioSFTPError(err, target)
	}
	_ = client.Chmod(target, mode) // best effort
	return nil
}

// codeStudioSFTPReplace writes a temporary sibling and publishes it with posix-rename
// (fallback: remove + rename); the old file's mode is kept.
func codeStudioSFTPReplace(client *sftp.Client, parent, target string, content []byte, mode os.FileMode, existed bool) error {
	temp := pathpkg.Join(parent, ".aurago-save-"+strings.ToLower(rand.Text()))
	f, err := client.OpenFile(temp, os.O_WRONLY|os.O_CREATE|os.O_EXCL)
	if err != nil {
		return codeStudioSFTPError(err, target)
	}
	published := false
	defer func() {
		if !published {
			_ = client.Remove(temp)
		}
	}()
	if _, err := f.Write(content); err != nil {
		_ = f.Close()
		return codeStudioSFTPError(err, target)
	}
	if err := f.Close(); err != nil {
		return codeStudioSFTPError(err, target)
	}
	_ = client.Chmod(temp, mode)
	if err := client.PosixRename(temp, target); err != nil {
		if existed {
			if err := client.Remove(target); err != nil && !codeStudioSFTPNotExist(err) {
				return codeStudioSFTPError(err, target)
			}
		}
		if err := client.Rename(temp, target); err != nil {
			return codeStudioSFTPError(err, target)
		}
	}
	published = true
	return nil
}

func (b *codeStudioSSHBackend) Move(ctx context.Context, from, to string) error {
	src, err := b.remote(from)
	if err != nil {
		return err
	}
	dst, err := b.remote(to)
	if err != nil {
		return err
	}
	if src == b.root {
		return codeStudioBadRequest("cannot change workspace root")
	}
	if dst == b.root {
		return codeStudioBadRequest("cannot replace workspace root")
	}
	return b.sftpDo(ctx, false, func(c *codeStudioSSHConn) error {
		if err := b.confine(c, pathpkg.Dir(src)); err != nil {
			return err
		}
		if err := c.sftp.MkdirAll(pathpkg.Dir(dst)); err != nil {
			return codeStudioSFTPError(err, to)
		}
		if err := b.confine(c, pathpkg.Dir(dst)); err != nil {
			return err
		}
		if err := c.sftp.PosixRename(src, dst); err != nil {
			if err := c.sftp.Rename(src, dst); err != nil {
				return codeStudioSFTPError(err, from)
			}
		}
		return nil
	})
}

func (b *codeStudioSSHBackend) Delete(ctx context.Context, vpath string) error {
	target, err := b.remote(vpath)
	if err != nil {
		return err
	}
	if target == b.root {
		return codeStudioBadRequest("cannot change workspace root")
	}
	return b.sftpDo(ctx, false, func(c *codeStudioSSHConn) error {
		if err := b.confine(c, pathpkg.Dir(target)); err != nil {
			return err
		}
		return codeStudioSFTPRemoveAll(c.sftp, target)
	})
}

// codeStudioSFTPRemoveAll never follows symlinks (Lstat / lstat-based ReadDir).
func codeStudioSFTPRemoveAll(client *sftp.Client, p string) error {
	info, err := client.Lstat(p)
	if err != nil {
		return codeStudioSFTPError(err, p)
	}
	if !info.IsDir() {
		return codeStudioSFTPError(client.Remove(p), p)
	}
	children, err := client.ReadDir(p)
	if err != nil {
		return codeStudioSFTPError(err, p)
	}
	for _, child := range children {
		if err := codeStudioSFTPRemoveAll(client, pathpkg.Join(p, child.Name())); err != nil {
			return err
		}
	}
	return codeStudioSFTPError(client.RemoveDirectory(p), p)
}

func (b *codeStudioSSHBackend) Mkdir(ctx context.Context, vpath string) error {
	target, err := b.remote(vpath)
	if err != nil {
		return err
	}
	return b.sftpDo(ctx, true, func(c *codeStudioSSHConn) error {
		if err := b.confine(c, target); err != nil {
			return err
		}
		return codeStudioSFTPError(c.sftp.MkdirAll(target), vpath)
	})
}

// codeStudioCappedBuffer keeps at most limit bytes and drains the rest (thread-safe:
// stdout and stderr may share it).
type codeStudioCappedBuffer struct {
	mu    sync.Mutex
	buf   []byte
	limit int
}

func newCodeStudioCappedBuffer(limit int) *codeStudioCappedBuffer {
	return &codeStudioCappedBuffer{limit: limit}
}

func (c *codeStudioCappedBuffer) Write(p []byte) (int, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if room := c.limit - len(c.buf); room > 0 {
		c.buf = append(c.buf, p[:min(room, len(p))]...)
	}
	return len(p), nil
}

func (c *codeStudioCappedBuffer) String() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return string(c.buf)
}

// codeStudioRecordBuffer stops (closes full) after maxRecords lines or maxBytes.
type codeStudioRecordBuffer struct {
	mu                   sync.Mutex
	buf                  []byte
	records              int
	maxRecords, maxBytes int
	full                 chan struct{}
	once                 sync.Once
}

func newCodeStudioRecordBuffer(maxRecords, maxBytes int) *codeStudioRecordBuffer {
	return &codeStudioRecordBuffer{maxRecords: maxRecords, maxBytes: maxBytes, full: make(chan struct{})}
}

func (r *codeStudioRecordBuffer) Write(p []byte) (int, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, ch := range p {
		if r.records >= r.maxRecords || len(r.buf) >= r.maxBytes {
			r.once.Do(func() { close(r.full) })
			break
		}
		r.buf = append(r.buf, ch)
		if ch == '\n' {
			r.records++
		}
	}
	return len(p), nil
}

func (r *codeStudioRecordBuffer) Full() bool {
	select {
	case <-r.full:
		return true
	default:
		return false
	}
}

// Complete returns the output up to the last full record.
func (r *codeStudioRecordBuffer) Complete() string {
	r.mu.Lock()
	defer r.mu.Unlock()
	if i := bytes.LastIndexByte(r.buf, '\n'); i >= 0 {
		return string(r.buf[:i+1])
	}
	return ""
}

func (b *codeStudioSSHBackend) run(ctx context.Context, script string, stdin []byte, stdout, stderr io.Writer, stop <-chan struct{}) (int, error) {
	code := -1
	err := b.pool.withConn(ctx, b.rec.DeviceID, codeStudioSSHRetrySessionOpen, func(c *codeStudioSSHConn) error {
		var err error
		code, err = c.run(ctx, codeStudioSSHCommandLine(script), stdin, stdout, stderr, stop)
		return err
	})
	return code, err
}

func (b *codeStudioSSHBackend) Exec(ctx context.Context, argv []string, vcwd string, stdin []byte, timeout time.Duration) (codeStudioExecResult, error) {
	if len(argv) == 0 || strings.TrimSpace(argv[0]) == "" {
		return codeStudioExecResult{}, codeStudioBadRequest("command is required")
	}
	if timeout <= 0 || timeout > codeStudioMaxExecTime {
		timeout = codeStudioMaxExecTime
	}
	cwd := b.root
	if vcwd != "" {
		p, err := b.remote(vcwd)
		if err != nil {
			return codeStudioExecResult{}, err
		}
		cwd = p
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	out := newCodeStudioCappedBuffer(codeStudioExecOutputLimit + 1)
	code, err := b.run(ctx, "cd "+shellQuote(cwd)+" && exec "+codeStudioShellJoin(argv), stdin, out, out, nil)
	if err != nil {
		if ctx.Err() != nil {
			return codeStudioExecResult{}, fmt.Errorf("code studio exec: %w", ctx.Err())
		}
		return codeStudioExecResult{}, err
	}
	return codeStudioExecResult{ExitCode: code, Output: out.String()}, nil
}

// Git pins English messages (codeStudioGitNotRepository / identity checks parse them).
func (b *codeStudioSSHBackend) Git(ctx context.Context, timeout time.Duration, args ...string) (codeStudioExecResult, error) {
	argv := append([]string{"env", "LC_ALL=C", "GIT_TERMINAL_PROMPT=0", "git", "-C", b.root, "--literal-pathspecs"}, args...)
	return b.Exec(ctx, argv, "", nil, timeout)
}

func (b *codeStudioSSHBackend) Search(ctx context.Context, opts codeStudioSearchOptions) ([]codeStudioSearchResult, bool, error) {
	if strings.TrimSpace(opts.Query) == "" {
		return nil, false, codeStudioBadRequest("search query is required")
	}
	dir, err := b.remote(opts.Path)
	if err != nil {
		return nil, false, err
	}
	ctx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()
	out := newCodeStudioRecordBuffer(codeStudioSearchResultLimit+1, 16<<20)
	errOut := newCodeStudioCappedBuffer(64 << 10)
	code, err := b.run(ctx, codeStudioShellJoin(codeStudioSSHGrepArgs(opts, dir)), nil, out, errOut, out.full)
	if err != nil && !out.Full() {
		return nil, false, err
	}
	text := out.Complete()
	if code == 127 {
		return nil, false, codeStudioBadRequest("grep is not installed on " + b.target.Host)
	}
	if code > 1 && text == "" && !out.Full() {
		msg := strings.TrimSpace(errOut.String())
		if msg == "" {
			msg = "search failed"
		}
		return nil, false, codeStudioBadRequest(msg)
	}
	results, truncated, err := parseCodeStudioGrepOutput(rewriteCodeStudioGrepPaths(b.root, text))
	if err != nil {
		return nil, false, err
	}
	return results, truncated || out.Full(), nil
}

func (b *codeStudioSSHBackend) probeCapabilities(ctx context.Context) (*codeStudioCapabilities, error) {
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	out := newCodeStudioCappedBuffer(64 << 10)
	code, err := b.run(ctx, codeStudioCapabilitiesScript, nil, out, io.Discard, nil)
	if err != nil {
		return nil, err
	}
	if code != 0 {
		return nil, fmt.Errorf("capability probe exited with %d", code)
	}
	caps := parseCodeStudioCapabilities(out.String(), time.Now().UTC())
	return &caps, nil
}

// ---- TEMPORARY STUBS (Task 12), replaced in Task 13 ----

func (b *codeStudioSSHBackend) OpenTerminal(context.Context, int, int) (containerTerminalSession, error) {
	return nil, errCodeStudioUnsupported
}

func (b *codeStudioSSHBackend) StartProcess(context.Context, []string, string) (codeStudioProcess, error) {
	return nil, errCodeStudioUnsupported
}
