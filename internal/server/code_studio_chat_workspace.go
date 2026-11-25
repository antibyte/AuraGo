package server

import (
	"context"
	"errors"
	"fmt"
	pathpkg "path"
	"regexp"
	"strings"
)

// codeStudioChatWorkspace is the SSH workspace of a Code Studio chat turn,
// looked up by the server from desktopChatContext.WorkspaceID. Label, host
// and folder never come from the request.
type codeStudioChatWorkspace struct {
	ID      string
	Label   string
	Host    string
	Root    string
	Missing bool // the id names no saved SSH workspace
}

// Saved workspace ids are random and url-safe; anything else is not echoed into the prompt.
var codeStudioChatWorkspaceIDPattern = regexp.MustCompile(`^[A-Za-z0-9_-]{1,64}$`)

// codeStudioChatWorkspaceLookup resolves a saved workspace without dialing; tests replace it.
var codeStudioChatWorkspaceLookup = func(ctx context.Context, s *Server, id string) (codeStudioWorkspaceInfo, error) {
	h := newCodeStudioHandlers(s)
	backend, err := h.codeStudioBackendByID(ctx, id)
	if err != nil {
		return codeStudioWorkspaceInfo{}, err
	}
	return backend.Info(), nil
}

// resolveCodeStudioChatWorkspace returns nil for the local workspace and for
// chats outside Code Studio. Like the agent tool, it does not apply the HTTP
// remote-caller gate: the chat endpoint already authenticated the user, and
// the workspace data it exposes is only a label, host and folder.
func resolveCodeStudioChatWorkspace(ctx context.Context, s *Server, chat desktopChatContext) *codeStudioChatWorkspace {
	if s == nil || !strings.EqualFold(strings.TrimSpace(chat.Source), "code-studio") {
		return nil
	}
	id := strings.TrimSpace(chat.WorkspaceID)
	if id == "" || id == codeStudioLocalWorkspaceID {
		return nil
	}
	if !codeStudioChatWorkspaceIDPattern.MatchString(id) {
		return &codeStudioChatWorkspace{Missing: true}
	}
	info, err := codeStudioChatWorkspaceLookup(ctx, s, id)
	if err == nil && info.Kind != codeStudioWorkspaceKindSSH {
		return nil
	}
	if err != nil {
		if !errors.Is(err, errCodeStudioWorkspaceNotFound) && s.Logger != nil {
			s.Logger.Warn("code studio chat workspace lookup failed", "workspace_id", id, "error", err)
		}
		return &codeStudioChatWorkspace{ID: id, Missing: true}
	}
	label := strings.TrimSpace(info.Label)
	if label == "" {
		label = pathpkg.Base(info.Root)
	}
	return &codeStudioChatWorkspace{ID: info.ID, Label: label, Host: codeStudioWorkspaceDisplayHost(info), Root: info.Root}
}

// buildCodeStudioRemoteWorkspacePrompt replaces the local Code Studio routing
// for an SSH workspace.
func buildCodeStudioRemoteWorkspacePrompt(ws codeStudioChatWorkspace) string {
	var b strings.Builder
	if ws.Missing {
		if ws.ID != "" {
			fmt.Fprintf(&b, "\nThe Code Studio window names the SSH workspace %q, which AuraGo cannot find (removed, or its saved data is gone).", ws.ID)
		} else {
			b.WriteString("\nThe Code Studio window names an SSH workspace that AuraGo cannot find.")
		}
		b.WriteString(" Do not read or change files for it with any tool; tell the user to reopen the folder with Code Studio's workspace switcher.")
		return b.String()
	}
	b.WriteString("\nThis Code Studio window works in a remote SSH workspace, not in the virtual desktop workspace; this replaces the Code Studio and /workspace guidance above.")
	fmt.Fprintf(&b, "\nUse code_studio_workspace with workspace_id %q for every file and command action in it: list_files, read_file, read_file_excerpt, search, write_file, patch_file and run_command.", ws.ID)
	b.WriteString(" Paths are relative to the remote folder; a /workspace/<path> of this context is path <path> of code_studio_workspace.")
	b.WriteString(" Never use virtual_desktop_files, virtual_desktop_apps open_in_app, file_editor, filesystem, execute_shell or remote_execution for these paths; they do not reach this folder.")
	b.WriteString("\nFor changes prefer search or read_file_excerpt, then patch_file with a find text that occurs exactly once; use write_file only with complete new content. Code Studio reloads open files after your writes, so do not open them in an app and do not paste the changed code into your reply; name the changed files in one or two sentences.")
	b.WriteString("\nRemote workspace:\n")
	b.WriteString(desktopExternalData("code_studio_remote_workspace", "Label: "+ws.Label+"\nHost: "+ws.Host+"\nFolder: "+ws.Root, 1024))
	return b.String()
}
