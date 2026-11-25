package server

import (
	"context"
	"errors"
	"strings"
	"time"

	"aurago/internal/tools"
)

// codeStudioAgentWorkspaces gives the code_studio_workspace agent tool
// (internal/tools) the saved SSH workspaces through the Code Studio backends.
// The local workspace is refused: the agent uses virtual_desktop_files there.
//
// Gate: the agent acts for the signed-in user of a desktop chat turn, so the
// remote-caller gate of the HTTP API (codeStudioRemoteCallerAllowed, which asks
// bearer tokens for desktop:admin) does not apply here. The tool itself applies
// the gates of virtual_desktop_files: desktop enabled, allow_agent_control,
// tools.virtual_desktop.enabled and read-only (Limits). The lookup never dials.
type codeStudioAgentWorkspaces struct{ s *Server }

var _ tools.CodeStudioWorkspaceProvider = codeStudioAgentWorkspaces{}

func (p codeStudioAgentWorkspaces) Workspace(ctx context.Context, id string) (tools.CodeStudioWorkspace, error) {
	id = strings.TrimSpace(id)
	if id == "" || id == codeStudioLocalWorkspaceID {
		return nil, tools.ErrCodeStudioWorkspaceLocal
	}
	if p.s == nil {
		return nil, tools.ErrCodeStudioWorkspaceNotFound
	}
	h := newCodeStudioHandlers(p.s)
	backend, err := h.codeStudioBackendByID(ctx, id)
	if errors.Is(err, errCodeStudioWorkspaceNotFound) {
		return nil, tools.ErrCodeStudioWorkspaceNotFound
	}
	if err != nil {
		return nil, err
	}
	info := backend.Info()
	if info.Kind != codeStudioWorkspaceKindSSH {
		return nil, tools.ErrCodeStudioWorkspaceLocal
	}
	return codeStudioAgentWorkspace{backend: backend, info: info}, nil
}

// Changed feeds the workspace change hook: the preview version and anything
// else that watches workspace writes. No paths means "anything may have changed".
func (p codeStudioAgentWorkspaces) Changed(id string, vpaths ...string) {
	if p.s != nil {
		newCodeStudioHandlers(p.s).notifyWorkspaceChange(id, vpaths...)
	}
}

func (p codeStudioAgentWorkspaces) Limits() tools.CodeStudioWorkspaceLimits {
	limits := tools.CodeStudioWorkspaceLimits{MaxFileBytes: newCodeStudioHandlers(p.s).maxFileSizeBytes()}
	if p.s != nil && p.s.Cfg != nil {
		p.s.CfgMu.RLock()
		limits.ReadOnly = p.s.Cfg.VirtualDesktop.ReadOnly
		p.s.CfgMu.RUnlock()
	}
	return limits
}

// codeStudioAgentWorkspace adapts one backend. Every path is sanitised again
// here; the backend's own confinement stays the guard behind it.
type codeStudioAgentWorkspace struct {
	backend codeStudioBackend
	info    codeStudioWorkspaceInfo
}

func (w codeStudioAgentWorkspace) Info() tools.CodeStudioWorkspaceInfo {
	return tools.CodeStudioWorkspaceInfo{ID: w.info.ID, Label: w.info.Label, Host: codeStudioWorkspaceDisplayHost(w.info), Root: w.info.Root}
}

func (w codeStudioAgentWorkspace) List(ctx context.Context, vdir string) ([]tools.CodeStudioWorkspaceEntry, error) {
	clean, err := sanitizeCodeStudioPath(vdir)
	if err != nil {
		return nil, err
	}
	entries, err := w.backend.List(ctx, clean)
	if err != nil {
		return nil, err
	}
	out := make([]tools.CodeStudioWorkspaceEntry, 0, len(entries))
	for _, e := range entries {
		out = append(out, tools.CodeStudioWorkspaceEntry{Name: e.Name, Path: e.Path, Type: e.Type, Size: e.Size, Modified: e.Modified})
	}
	return out, nil
}

func (w codeStudioAgentWorkspace) Read(ctx context.Context, vpath string, max int64) ([]byte, error) {
	clean, err := sanitizeCodeStudioPath(vpath)
	if err != nil {
		return nil, err
	}
	return w.backend.Read(ctx, clean, max)
}

func (w codeStudioAgentWorkspace) Write(ctx context.Context, vpath string, content []byte) error {
	clean, err := sanitizeCodeStudioPath(vpath)
	if err != nil {
		return err
	}
	if clean == codeStudioWorkspaceRoot {
		return errors.New("cannot write the workspace root")
	}
	return w.backend.Write(ctx, clean, content, false)
}

func (w codeStudioAgentWorkspace) Search(ctx context.Context, opts tools.CodeStudioWorkspaceSearch) ([]tools.CodeStudioWorkspaceMatch, bool, error) {
	clean, err := sanitizeCodeStudioPath(opts.Path)
	if err != nil {
		return nil, false, err
	}
	results, truncated, err := w.backend.Search(ctx, codeStudioSearchOptions{
		Query: opts.Query, Path: clean, CaseSensitive: opts.CaseSensitive, Regex: opts.Regex,
		WholeWord: opts.WholeWord, Include: opts.Include, Exclude: opts.Exclude,
	})
	if err != nil {
		return nil, false, err
	}
	out := make([]tools.CodeStudioWorkspaceMatch, 0, len(results))
	for _, r := range results {
		out = append(out, tools.CodeStudioWorkspaceMatch{Path: r.Path, Line: r.Line, Preview: r.Preview})
	}
	return out, truncated, nil
}

func (w codeStudioAgentWorkspace) Exec(ctx context.Context, command, vcwd string, timeout time.Duration) (tools.CodeStudioWorkspaceExecResult, error) {
	clean, err := sanitizeCodeStudioPath(vcwd)
	if err != nil {
		return tools.CodeStudioWorkspaceExecResult{}, err
	}
	result, err := w.backend.Exec(ctx, []string{"/bin/sh", "-c", command}, clean, nil, timeout)
	if err != nil {
		return tools.CodeStudioWorkspaceExecResult{}, err
	}
	return tools.CodeStudioWorkspaceExecResult{ExitCode: result.ExitCode, Output: result.Output}, nil
}
