package server

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"time"

	"aurago/internal/agent"
	"aurago/internal/config"
	"aurago/internal/detective"
	"aurago/internal/tools"
	"aurago/internal/virtualcomputers"
)

// The background browser has no session-owner check of its own. Restrict it to
// IDs returned by this run and clean up only those resources.
type detectiveResources struct {
	mu                sync.Mutex
	browsers          map[string]bool
	workspaces        map[string]bool
	openingBrowsers   int
	openingWorkspaces int
}

func newDetectiveResources() *detectiveResources {
	return &detectiveResources{browsers: map[string]bool{}, workspaces: map[string]bool{}}
}
func (d *detectiveResources) authorize(tc agent.ToolCall) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	if tc.Action == "browser_automation" {
		if tc.Operation == "create_session" {
			if len(d.browsers)+d.openingBrowsers >= 2 {
				return errors.New("close an existing research browser before opening another")
			}
			d.openingBrowsers++
			return nil
		}
		id, _ := tc.Params["session_id"].(string)
		if !d.browsers[id] {
			return errors.New("browser session was not created by this research run")
		}
		if tc.Operation == "screenshot" {
			return errors.New("use the workspace browser for case-scoped screenshots")
		}
	}
	if tc.Action == "virtual_workspace" && tc.Operation == "open" {
		if len(d.workspaces)+d.openingWorkspaces >= 1 {
			return errors.New("reuse the existing research workspace")
		}
		if volume, _ := tc.Params["volume_id"].(string); volume != "" {
			return errors.New("research opens a clean workspace without private volumes")
		}
		d.openingWorkspaces++
	}
	return nil
}
func (d *detectiveResources) observe(tc agent.ToolCall, out string) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if tc.Action == "browser_automation" && tc.Operation == "create_session" {
		d.openingBrowsers = max(0, d.openingBrowsers-1)
	}
	if tc.Action == "virtual_workspace" && tc.Operation == "open" {
		d.openingWorkspaces = max(0, d.openingWorkspaces-1)
	}
	var result map[string]any
	if json.Unmarshal([]byte(strings.TrimSpace(strings.TrimPrefix(out, "Tool Output:"))), &result) != nil {
		return
	}
	status, _ := result["status"].(string)
	if status != "ok" && status != "success" {
		return
	}
	if tc.Action == "browser_automation" {
		id, _ := result["session_id"].(string)
		if tc.Operation == "create_session" && id != "" {
			d.browsers[id] = true
		}
		if tc.Operation == "close_session" {
			if id == "" {
				id, _ = tc.Params["session_id"].(string)
			}
			delete(d.browsers, id)
		}
	}
	if tc.Action == "virtual_workspace" && (tc.Operation == "open" || tc.Operation == "get") {
		data, _ := result["data"].(map[string]any)
		if data == nil {
			data = result
		}
		workspace, _ := data["workspace"].(map[string]any)
		id, _ := workspace["id"].(string)
		if id != "" {
			d.workspaces[id] = true
		}
	}
}
func (d *detectiveResources) close(s *Server, cfg *config.Config, job *detective.Session) {
	d.mu.Lock()
	defer d.mu.Unlock()
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	for id := range d.browsers {
		tools.ExecuteBrowserAutomation(ctx, cfg, tools.BrowserAutomationRequest{Operation: "close_session", SessionID: id}, s.Logger)
	}
	// Preserve a visible workspace while the user handles a challenge; its normal
	// bounded lease still applies. Subsequent runs may find it through their owner.
	c, _ := job.Snapshot()
	if c.Run.Status == "waiting_for_user" {
		return
	}
	identity := virtualcomputers.WorkspaceIdentity{SessionID: "detective-" + job.CaseID, Actor: "agent"}
	for id := range d.workspaces {
		tools.ExecuteVirtualWorkspace(ctx, cfg, identity, map[string]any{"operation": "close", "workspace_id": id})
	}
}
