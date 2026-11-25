package server

import (
	"context"
	"errors"
	"fmt"
	"net/http"
)

// codeStudioPreviewSource is the read side of a workspace backend. codeStudioBackend has the same two methods, so
// every backend can serve previews without an adapter.
type codeStudioPreviewSource interface {
	Probe(ctx context.Context, vpath string) (codeStudioFileProbe, error)
	Read(ctx context.Context, vpath string, max int64) ([]byte, error)
}

var errCodeStudioPreviewWorkspaceNotFound = errors.New("workspace_not_found")

// previewWorkspaceID resolves the workspace of an HTTP request with the same gate as backendFor (SSH workspaces need
// a session or desktop:admin token), without starting the local container. A missing workspace is
// reported as errCodeStudioWorkspaceNotFound by backendFor and stays matchable through errors.Is; the
// errCodeStudioPreviewWorkspaceNotFound sentinel is added for callers that only know the preview contract.
func (h codeStudioHandlers) previewWorkspaceID(r *http.Request) (string, error) {
	id, err := codeStudioWorkspaceID(r)
	if err != nil {
		return "", err
	}
	if id == codeStudioLocalWorkspaceID {
		// No container lookup: issuing a link only names the workspace, the serve route reads it later.
		return id, nil
	}
	if !codeStudioRemoteCallerAllowed(h.server, r) {
		return "", codeStudioRemoteScopeError()
	}
	backend, err := h.codeStudioBackendByID(r.Context(), id)
	if err != nil {
		return "", mapCodeStudioPreviewNotFound(err)
	}
	return backend.Info().ID, nil
}

// previewSource looks a workspace up by the id inside a preview token. It does not dial and does not run the
// remote-caller gate: the signed token is the authorisation, issued only to callers that passed the gate.
func (h codeStudioHandlers) previewSource(ctx context.Context, wsID string) (codeStudioPreviewSource, error) {
	backend, err := h.codeStudioBackendByID(ctx, wsID)
	if err != nil {
		return nil, mapCodeStudioPreviewNotFound(err)
	}
	return backend, nil
}

func mapCodeStudioPreviewNotFound(err error) error {
	if errors.Is(err, errCodeStudioWorkspaceNotFound) {
		return fmt.Errorf("%w: %w", errCodeStudioPreviewWorkspaceNotFound, err)
	}
	return err
}
