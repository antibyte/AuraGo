package server

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"aurago/internal/desktop"
)

// Remote language servers. In an SSH workspace the bridge runs the command of
// codeStudioLSPServerFor as a stdio process of the workspace backend
// (codeStudioBackend.StartProcess, one SSH session of the pooled connection),
// with the same framing and close codes as in the container. It differs in
// four ways: plain sh with the usual per-user install dirs on PATH, the same
// session tag for the cleanup, URI rewriting between file:///workspace and the
// remote folder, and a fresh install probe per start. The cached capability can
// be stale: the user may install gopls after connecting.

const (
	// codeStudioRemoteLSPPath puts the per-user install dirs (go install,
	// pip --user / pipx, npm prefix, cargo) and /usr/local in front of the
	// PATH of a non-login SSH exec. Probe and start share it.
	codeStudioRemoteLSPPath = `export PATH="$HOME/go/bin:$HOME/.local/bin:$HOME/.npm-global/bin:$HOME/.cargo/bin:/usr/local/go/bin:/usr/local/bin:$PATH"`
	// codeStudioRemoteLSPMissingExit is the probe's exit code for a missing
	// server binary (the 44 in codeStudioRemoteLSPProbeScript).
	codeStudioRemoteLSPMissingExit = 44
	// codeStudioRemoteLSPProbeScript runs in the workspace folder: it prints
	// the symlink-resolved folder, then checks the binary given as $1.
	codeStudioRemoteLSPProbeScript = codeStudioRemoteLSPPath + "\npwd -P || exit 3\ncommand -v \"$1\" >/dev/null || exit 44"
	// codeStudioRemoteLSPStartScript tags the session ($1) and replaces the
	// shell with the server command ($2...).
	codeStudioRemoteLSPStartScript = codeStudioRemoteLSPPath + "\nexport GOTOOLCHAIN=local\nexport " + codeStudioLSPSessionEnv + "=\"$1\"\nshift\nexec \"$@\""
	// codeStudioRemoteLSPEndScript is codeStudioLSPEndScript on hosts with
	// /proc. Elsewhere (macOS) stdin EOF and the session kill end the server.
	codeStudioRemoteLSPEndScript = "[ -d /proc ] || exit 0\n" + codeStudioLSPEndScript
)

type codeStudioRemoteLSPHost struct {
	backend  codeStudioBackend
	info     codeStudioWorkspaceInfo
	realRoot string // pwd -P of the folder, set by Installed
}

var _ codeStudioLSPHost = (*codeStudioRemoteLSPHost)(nil)

func newCodeStudioRemoteLSPHost(backend codeStudioBackend) *codeStudioRemoteLSPHost {
	return &codeStudioRemoteLSPHost{backend: backend, info: backend.Info()}
}

func (h *codeStudioRemoteLSPHost) Installed(ctx context.Context, binary string) (bool, error) {
	result, err := h.backend.Exec(ctx, []string{"/bin/sh", "-c", codeStudioRemoteLSPProbeScript, "sh", binary}, codeStudioWorkspaceRoot, nil, codeStudioLSPCheckTimeout)
	if err != nil {
		return false, err
	}
	if line, _, _ := strings.Cut(result.Output, "\n"); strings.HasPrefix(strings.TrimSpace(line), "/") {
		h.realRoot = strings.TrimSpace(line)
	}
	switch result.ExitCode {
	case 0:
		return true, nil
	case codeStudioRemoteLSPMissingExit:
		return false, nil
	}
	return false, fmt.Errorf("language server probe on %s failed (exit %d): %s", codeStudioWorkspaceDisplayHost(h.info), result.ExitCode, strings.TrimSpace(result.Output))
}

func (h *codeStudioRemoteLSPHost) Start(ctx context.Context, cmd []string, nonce string, stderr io.Writer) (codeStudioLSPStream, io.Reader, error) {
	argv := append([]string{"/bin/sh", "-c", codeStudioRemoteLSPStartScript, "aurago-lsp", nonce}, cmd...)
	proc, err := h.backend.StartProcess(ctx, argv, codeStudioWorkspaceRoot)
	if err != nil {
		return nil, nil, err
	}
	stream := newCodeStudioRemoteLSPStream(proc, stderr)
	return stream, stream, nil
}

// End runs the tag cleanup. Its exit code is not checked: a vanished process
// makes kill fail, which is the normal case.
func (h *codeStudioRemoteLSPHost) End(ctx context.Context, nonce string) error {
	if nonce == "" {
		return errors.New("language server session has no end tag")
	}
	_, err := h.backend.Exec(ctx, []string{"/bin/sh", "-c", codeStudioRemoteLSPEndScript, "aurago-end-lsp", nonce}, codeStudioWorkspaceRoot, nil, containerTerminalEndTimeout)
	return err
}

func (h *codeStudioRemoteLSPHost) NotInstalled(binary string) string {
	return binary + " is not installed on " + codeStudioWorkspaceDisplayHost(h.info)
}

func (h *codeStudioRemoteLSPHost) Rewriter() *codeStudioLSPURIRewriter {
	return newCodeStudioLSPURIRewriter(h.info.Root, h.realRoot)
}

// codeStudioRemoteLSPStream is a backend process as a codeStudioLSPStream: Read
// is stdout, Write is stdin. Stderr is drained into the bridge's debug tail.
type codeStudioRemoteLSPStream struct {
	codeStudioProcess
}

func newCodeStudioRemoteLSPStream(proc codeStudioProcess, stderr io.Writer) *codeStudioRemoteLSPStream {
	if stderr == nil {
		stderr = io.Discard
	}
	if source := proc.Stderr(); source != nil {
		go func() { _, _ = io.Copy(stderr, source) }()
	}
	return &codeStudioRemoteLSPStream{codeStudioProcess: proc}
}

// codeStudioWorkspaceDisplayHost is user@host of a workspace for messages and prompts.
func codeStudioWorkspaceDisplayHost(info codeStudioWorkspaceInfo) string {
	host := strings.TrimSpace(info.Host)
	if user := strings.TrimSpace(info.User); user != "" && host != "" {
		return user + "@" + host
	}
	if host != "" {
		return host
	}
	return strings.TrimSpace(info.DeviceName)
}

// handleRemoteLSP is handleLSP for an SSH workspace (?ws=): the same auth,
// origin, method and language checks, then the workspace's backend instead of
// the container (no Docker admission, no container hold). backendFor applies
// the remote-caller gate: bearer tokens need desktop:admin for SSH workspaces.
func (h codeStudioHandlers) handleRemoteLSP(w http.ResponseWriter, r *http.Request) {
	if !h.isAuthenticated(r) {
		jsonError(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	if !desktop.SameHostWebSocketOrigin(r) {
		jsonError(w, "forbidden websocket origin", http.StatusForbidden)
		return
	}
	if r.Method != http.MethodGet {
		jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	lsp, ok := codeStudioLSPServerFor(r.URL.Query().Get("lang"))
	if !ok {
		jsonError(w, "unknown language server", http.StatusBadRequest)
		return
	}
	backend, err := h.backendFor(r)
	if err != nil {
		writeCodeStudioBackendError(w, err)
		return
	}
	if backend.Info().Kind != codeStudioWorkspaceKindSSH {
		writeCodeStudioBackendError(w, codeStudioWorkspaceNotFound())
		return
	}
	conn, err := codeStudioWSUpgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	defer conn.Close()
	h.serveCodeStudioLSP(r.Context(), conn, newCodeStudioRemoteLSPHost(backend), lsp)
}
