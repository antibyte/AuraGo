package server

import (
	"context"
	"net/http"
	"strings"
	"time"

	"aurago/internal/desktop"
)

// codeStudioLocalBackend is today's container workspace. h carries the Docker adapter
// codeContainer installed, so every command vector equals the pre-backend code.
type codeStudioLocalBackend struct {
	h           codeStudioHandlers
	container   *desktop.CodeContainerService
	containerID string
}

var _ codeStudioBackend = (*codeStudioLocalBackend)(nil)

func (h *codeStudioHandlers) backendFor(r *http.Request) (codeStudioBackend, error) {
	id, err := codeStudioWorkspaceID(r)
	if err != nil {
		return nil, err
	}
	if id == codeStudioLocalWorkspaceID {
		return h.localBackend(r.Context())
	}
	return h.remoteBackend(r, id)
}

func (h *codeStudioHandlers) localBackend(ctx context.Context) (codeStudioBackend, error) {
	container, containerID, err := h.codeContainer(ctx, true)
	if err != nil {
		return nil, codeStudioUnavailableError{err}
	}
	return &codeStudioLocalBackend{h: *h, container: container, containerID: containerID}, nil
}

func (b *codeStudioLocalBackend) Info() codeStudioWorkspaceInfo {
	return codeStudioLocalWorkspaceInfo()
}

func (b *codeStudioLocalBackend) pathOp(ctx context.Context, cmd []string) error {
	result, err := b.h.docker.Exec(ctx, b.containerID, cmd, 30*time.Second)
	if err != nil {
		return err
	}
	if result.ExitCode != 0 {
		return codeStudioBadRequest(strings.TrimSpace(result.Output))
	}
	return nil
}

func (b *codeStudioLocalBackend) List(ctx context.Context, vdir string) ([]codeStudioFileEntry, error) {
	result, err := b.h.docker.Exec(ctx, b.containerID, codeStudioPathCommand("list", vdir), 30*time.Second)
	if err != nil {
		return nil, err
	}
	if result.ExitCode != 0 {
		return nil, codeStudioBadRequest(strings.TrimSpace(result.Output))
	}
	return parseCodeStudioFindOutput(result.Output)
}

func (b *codeStudioLocalBackend) Probe(ctx context.Context, vpath string) (codeStudioFileProbe, error) {
	return b.h.probeCodeStudioFile(ctx, b.containerID, vpath)
}

func (b *codeStudioLocalBackend) Read(ctx context.Context, vpath string, max int64) ([]byte, error) {
	return b.h.readCodeStudioFileBytes(ctx, b.containerID, vpath, max)
}

func (b *codeStudioLocalBackend) Write(ctx context.Context, vpath string, content []byte, createOnly bool) error {
	result, err := b.h.writeContainerFile(ctx, b.containerID, vpath, content, createOnly)
	if err != nil {
		return err
	}
	switch {
	case result.ExitCode == codeStudioFileExists:
		return codeStudioExistsError(strings.TrimSpace(result.Output)) // no default text: byte-compatible
	case result.ExitCode != 0:
		return codeStudioBadRequest(strings.TrimSpace(result.Output))
	}
	return nil
}

func (b *codeStudioLocalBackend) Move(ctx context.Context, from, to string) error {
	return b.pathOp(ctx, codeStudioPathCommand("move", from, to))
}

func (b *codeStudioLocalBackend) Delete(ctx context.Context, vpath string) error {
	return b.pathOp(ctx, codeStudioPathCommand("delete", vpath))
}

func (b *codeStudioLocalBackend) Mkdir(ctx context.Context, vpath string) error {
	return b.pathOp(ctx, codeStudioPathCommand("mkdir", vpath))
}

func (b *codeStudioLocalBackend) Search(ctx context.Context, opts codeStudioSearchOptions) ([]codeStudioSearchResult, bool, error) {
	cmd, err := buildCodeStudioSearchCommand(opts)
	if err != nil {
		return nil, false, codeStudioBadRequest(err.Error())
	}
	result, err := b.h.docker.Exec(ctx, b.containerID, cmd, 60*time.Second)
	if err != nil {
		return nil, false, err
	}
	if result.ExitCode > 1 {
		return nil, false, codeStudioBadRequest(strings.TrimSpace(result.Output))
	}
	return parseCodeStudioGrepOutput(result.Output)
}

// codeStudioLocalExecCommand rebuilds the legacy vector: sh -c "cd <cwd> && <command>".
func codeStudioLocalExecCommand(argv []string, vcwd string) []string {
	if vcwd == "" {
		return argv
	}
	line := codeStudioShellJoin(argv)
	if len(argv) == 3 && argv[0] == "sh" && argv[1] == "-c" {
		line = argv[2]
	}
	return []string{"sh", "-c", "cd " + shellQuote(vcwd) + " && " + line}
}

func (b *codeStudioLocalBackend) Exec(ctx context.Context, argv []string, vcwd string, stdin []byte, timeout time.Duration) (codeStudioExecResult, error) {
	cmd := codeStudioLocalExecCommand(argv, vcwd)
	if stdin != nil {
		execer, ok := b.h.docker.(codeStudioStdinExecer)
		if !ok {
			return codeStudioExecResult{}, &codeStudioStatusError{Status: http.StatusNotImplemented,
				Message: "stdin is not supported by this docker client", Err: errCodeStudioUnsupported}
		}
		return execer.ExecWithStdin(ctx, b.containerID, cmd, stdin, timeout)
	}
	return b.h.docker.Exec(ctx, b.containerID, cmd, timeout)
}

func (b *codeStudioLocalBackend) Git(ctx context.Context, timeout time.Duration, args ...string) (codeStudioExecResult, error) {
	return b.h.execGit(ctx, b.containerID, timeout, args...)
}

func (b *codeStudioLocalBackend) OpenTerminal(ctx context.Context, cols, rows int) (containerTerminalSession, error) {
	adapter, ok := b.h.docker.(codeStudioDockerAdapter)
	if !ok {
		return nil, errCodeStudioUnsupported
	}
	session, err := openCodeStudioTerminalSession(ctx, adapter.cfg, b.containerID)
	if err != nil {
		return nil, err
	}
	_ = session.Resize(ctx, cols, rows)
	return session, nil
}

func (b *codeStudioLocalBackend) StartProcess(context.Context, []string, string) (codeStudioProcess, error) {
	return nil, &codeStudioStatusError{Status: http.StatusNotImplemented,
		Message: "the local workspace runs language servers through the container bridge", Err: errCodeStudioUnsupported}
}
