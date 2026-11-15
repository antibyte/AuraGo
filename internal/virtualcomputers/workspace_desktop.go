package virtualcomputers

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"image/png"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"
)

type workspaceGUI struct {
	mu      sync.Mutex
	control sync.Mutex
	serial  chan struct{}
	token   string
	blocked bool
	human   bool
	closed  bool
	cancel  context.CancelFunc
	done    chan struct{}
	image   *workspaceDesktopImage
}
type workspaceDesktopImage struct {
	ref, path, nonce string
	created          time.Time
}

func (m *WorkspaceManager) SetGUIConfigSource(source func() ToolConfig) {
	m.guiMu.Lock()
	m.guiConfig = source
	m.guiMu.Unlock()
}
func (m *WorkspaceManager) liveGUIConfig(fallback ToolConfig) ToolConfig {
	m.guiMu.Lock()
	source := m.guiConfig
	m.guiMu.Unlock()
	if source != nil {
		return source()
	}
	return fallback
}
func (m *WorkspaceManager) guiFor(id string) *workspaceGUI {
	m.guiMu.Lock()
	defer m.guiMu.Unlock()
	if m.gui == nil {
		m.gui = make(map[string]*workspaceGUI)
	}
	if m.gui[id] == nil {
		m.gui[id] = &workspaceGUI{serial: make(chan struct{}, 1)}
	}
	return m.gui[id]
}
func (g *workspaceGUI) clearImageLocked() {
	if g.image != nil {
		_ = os.Remove(g.image.path)
		g.image = nil
	}
}
func hasDesktopControl(w Workspace) bool {
	return slices.Contains(w.Capabilities, "desktop.control.v1")
}

// All UI calls share this lane. Ownership, live permissions and leases are
// checked after admission, so a queued request cannot survive a control change.
func (m *WorkspaceManager) beginGUI(ctx context.Context, cfg ToolConfig, identity WorkspaceIdentity, id string, native, input bool) (context.Context, string, func(), error) {
	g := m.guiFor(id)
	select {
	case g.serial <- struct{}{}:
	case <-ctx.Done():
		return nil, "", nil, ctx.Err()
	}
	g.control.Lock()
	fail := func(err error) (context.Context, string, func(), error) {
		g.control.Unlock()
		<-g.serial
		return nil, "", nil, err
	}
	if err := ctx.Err(); err != nil {
		return fail(err)
	}
	live := m.liveGUIConfig(cfg)
	if err := validateWorkspaceFeature(live, identity); err != nil {
		return fail(err)
	}
	if native && (!cfg.AgentControl.NativeAppsEnabled || !live.AgentControl.NativeAppsEnabled) {
		return fail(WorkspaceRPCError{Code: "native_apps_disabled", Message: "native application control is disabled"})
	}
	w, err := m.ownedWorkspace(ctx, identity, id)
	if err != nil {
		return fail(err)
	}
	if w.State != WorkspaceStateReady || !w.LeaseExpiresAt.After(m.now()) || !w.MaxExpiresAt.After(m.now()) {
		return fail(WorkspaceRPCError{Code: "workspace_expired", Message: "workspace is not ready or its lease expired"})
	}
	human := w.ControlOwner == ControlOwnerHuman && w.ControlLeaseExpiresAt != nil && w.ControlLeaseExpiresAt.After(m.now())
	if human && input {
		return fail(WorkspaceRPCError{Code: "browser_human_control", Message: "workspace input is paused while the user has control"})
	}
	g.mu.Lock()
	if g.closed || g.blocked {
		g.mu.Unlock()
		return fail(WorkspaceRPCError{Code: "desktop_control_changed", Message: "workspace control changed; observe again"})
	}
	callCtx, cancel := context.WithTimeout(ctx, 40*time.Second)
	done := make(chan struct{})
	g.cancel, g.done = cancel, done
	token := g.token
	g.mu.Unlock()
	finish := func() {
		cancel()
		g.mu.Lock()
		g.cancel, g.done = nil, nil
		close(done)
		g.mu.Unlock()
		<-g.serial
	}
	if hasDesktopControl(w) && (token == "" || g.human != human) {
		token, err = newWorkspaceRequestID()
		if err == nil {
			var client *Client
			client, err = m.clientFactory(live)
			if err == nil {
				err = m.transportFactory(client).Call(callCtx, w.MachineID, "desktop.control", map[string]interface{}{"token": token, "blocked": human}, nil)
			}
		}
		if err != nil {
			g.control.Unlock()
			finish()
			return nil, "", nil, err
		}
		g.mu.Lock()
		g.token = token
		g.human = human
		g.clearImageLocked()
		g.mu.Unlock()
	}
	g.control.Unlock()
	if input {
		g.mu.Lock()
		g.clearImageLocked()
		g.mu.Unlock()
	}
	return callCtx, token, finish, nil
}

// revokeGUI acknowledges only a guest-side cancellation barrier and the end of
// the host call. Failed acknowledgements leave admission blocked.
func (m *WorkspaceManager) revokeGUI(ctx context.Context, cfg ToolConfig, w Workspace, closeWorkspace bool) (func(), error) {
	g := m.guiFor(w.ID)
	g.control.Lock()
	g.mu.Lock()
	g.blocked = true
	g.closed = g.closed || closeWorkspace
	g.clearImageLocked()
	cancel, done := g.cancel, g.done
	g.mu.Unlock()
	if cancel != nil {
		cancel()
	}
	token, err := newWorkspaceRequestID()
	if err == nil && hasDesktopControl(w) {
		var client *Client
		client, err = m.clientFactory(cfg)
		if err == nil {
			err = m.transportFactory(client).Call(ctx, w.MachineID, "desktop.control", map[string]interface{}{"token": token, "blocked": true}, nil)
			if err != nil && closeWorkspace {
				// Closing may terminate an unreachable guest; deletion also fences input.
				destroyErr := client.DestroyMachine(ctx, w.MachineID)
				if destroyErr == nil || workspaceMachineAlreadyGone(destroyErr) {
					err = nil
				}
			}
		}
	}
	if err == nil && done != nil {
		select {
		case <-done:
		case <-ctx.Done():
			err = ctx.Err()
		}
	}
	if err != nil {
		g.control.Unlock()
		return nil, err
	}
	g.mu.Lock()
	g.token = ""
	g.mu.Unlock() // Next admitted call negotiates a fresh token.
	return func() { g.mu.Lock(); g.blocked = false; g.mu.Unlock(); g.control.Unlock() }, nil
}

// Called after the live config is published, before the save is acknowledged.
func (m *WorkspaceManager) ApplyGUIConfig(ctx context.Context, oldCfg, newCfg ToolConfig) error {
	if newCfg.Enabled && newCfg.ToolGate && newCfg.AgentControl.Enabled && !newCfg.ReadOnly && (!oldCfg.AgentControl.NativeAppsEnabled || newCfg.AgentControl.NativeAppsEnabled) {
		return nil
	}
	m.guiMu.Lock()
	ids := make([]string, 0, len(m.gui))
	for id := range m.gui {
		ids = append(ids, id)
	}
	m.guiMu.Unlock()
	// Revoke every active call immediately, even if the first guest is unreachable.
	for _, id := range ids {
		g := m.guiFor(id)
		g.mu.Lock()
		g.blocked = true
		g.clearImageLocked()
		if g.cancel != nil {
			g.cancel()
		}
		g.mu.Unlock()
	}
	for _, id := range ids {
		w, ok, err := m.ledger.GetWorkspace(ctx, id)
		if err != nil {
			return err
		}
		if !ok || w.State != WorkspaceStateReady {
			continue
		}
		release, err := m.revokeGUI(ctx, oldCfg, w, false)
		if err != nil {
			return err
		}
		release()
	}
	return nil
}

func (m *WorkspaceManager) DesktopStatus(ctx context.Context, cfg ToolConfig, identity WorkspaceIdentity, id string) (map[string]interface{}, error) {
	w, err := m.ownedWorkspace(ctx, identity, id)
	if err != nil {
		return nil, err
	}
	state := map[string]interface{}{"state": "disabled", "version": "1.3.1", "capabilities": []string{}}
	if !cfg.Enabled || !cfg.ToolGate || !cfg.AgentControl.Enabled || !cfg.AgentControl.NativeAppsEnabled || cfg.ReadOnly {
		return state, nil
	}
	if w.Template != "desktop" || !slices.Contains(w.Capabilities, "desktop.native.v1") {
		state["state"] = "repair_required"
		return state, nil
	}
	if w.State != WorkspaceStateReady || !w.LeaseExpiresAt.After(m.now()) || !w.MaxExpiresAt.After(m.now()) {
		state["state"] = "session_unreachable"
		return state, nil
	}
	ctx, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()
	client, err := m.clientFactory(cfg)
	if err != nil {
		return nil, err
	}
	if err = requireWorkspaceControlPlane(ctx, client); err != nil {
		state["state"] = "repair_required"
		return state, nil
	}
	ctx, token, finish, err := m.beginGUI(ctx, cfg, identity, id, true, false)
	if err != nil {
		return nil, err
	}
	defer finish()
	if err = m.transportFactory(client).Call(ctx, w.MachineID, "desktop.status", DesktopActionRequest{ControlToken: token}, &state); err != nil {
		state = map[string]interface{}{"state": "session_unreachable", "version": "1.3.1"}
	}
	return scrubWorkspaceData(state), nil
}

func (m *WorkspaceManager) DesktopAction(ctx context.Context, cfg ToolConfig, identity WorkspaceIdentity, id string, r DesktopActionRequest) (map[string]interface{}, error) {
	if strings.TrimSpace(id) == "" {
		return nil, WorkspaceRPCError{Code: "workspace_id_required", Message: "workspace_id is required"}
	}
	if !slices.Contains(DesktopOperations, r.Operation) {
		return nil, WorkspaceRPCError{Code: "unsupported_operation", Message: "unsupported virtual_computer_ui operation"}
	}
	if !cfg.AgentControl.NativeAppsEnabled {
		return nil, WorkspaceRPCError{Code: "native_apps_disabled", Message: "native application control is disabled"}
	}
	w, client, transport, err := m.readyWorkspace(ctx, cfg, identity, id)
	if err != nil {
		return nil, err
	}
	if w.Template != "desktop" {
		return nil, WorkspaceRPCError{Code: "desktop_unavailable", Message: "native applications require a desktop workspace"}
	}
	if !slices.Contains(w.Capabilities, "desktop.native.v1") {
		return nil, WorkspaceRPCError{Code: "desktop_upgrade_required", Message: "run Install/Repair and create a new workspace; existing VMs retain their image"}
	}
	ctx, token, finish, err := m.beginGUI(ctx, cfg, identity, id, true, DesktopInput(r.Operation))
	if err != nil {
		return nil, err
	}
	defer finish()
	r.ControlToken = token
	var result map[string]interface{}
	if r.Operation == "screenshot" {
		result, err = m.captureDesktop(ctx, cfg, w, client, transport, r)
	} else {
		err = transport.Call(ctx, w.MachineID, "desktop."+r.Operation, r, &result)
	}
	outcome := "completed"
	if err != nil {
		outcome = "error"
	}
	if ok, exists := result["ok"].(bool); exists && !ok {
		outcome = "error"
	}
	_ = m.recordEvent(context.WithoutCancel(ctx), w.ID, "desktop_action", "Native application operation", map[string]interface{}{"operation": r.Operation, "outcome": outcome})
	if err == nil {
		m.touchBestEffort(ctx, cfg, client, w)
	}
	return scrubWorkspaceData(result), err
}

func (m *WorkspaceManager) captureDesktop(ctx context.Context, cfg ToolConfig, w Workspace, client *Client, transport WorkspaceTransport, r DesktopActionRequest) (map[string]interface{}, error) {
	started := m.now()
	var observation struct {
		Ref    string `json:"image_ref"`
		Width  int    `json:"width"`
		Height int    `json:"height"`
	}
	if err := transport.Call(ctx, w.MachineID, "desktop.screenshot_prepare", r, &observation); err != nil {
		return nil, err
	}
	screenshot, err := client.Screenshot(ctx, w.MachineID)
	if err != nil {
		return nil, err
	}
	if len(screenshot.Base64) > 12*1024*1024 {
		return nil, fmt.Errorf("desktop screenshot exceeds size limit")
	}
	data, err := base64.StdEncoding.DecodeString(screenshot.Base64)
	if err != nil {
		return nil, err
	}
	size, err := png.DecodeConfig(bytes.NewReader(data))
	if err != nil || size.Width < 1 || size.Height < 1 || size.Width*size.Height > 16*1024*1024 {
		return nil, fmt.Errorf("invalid or oversized desktop PNG")
	}
	r.ImageRef, r.Width, r.Height = observation.Ref, size.Width, size.Height
	if observation.Ref == "" {
		return nil, fmt.Errorf("desktop capture omitted its reference")
	}
	if err := transport.Call(ctx, w.MachineID, "desktop.screenshot_commit", r, nil); err != nil {
		return nil, err
	}
	if cfg.NativeArtifactDir == "" {
		return nil, fmt.Errorf("private desktop artifact directory is unavailable")
	}
	if err := os.MkdirAll(cfg.NativeArtifactDir, 0700); err != nil {
		return nil, err
	}
	file, err := os.CreateTemp(cfg.NativeArtifactDir, "capture-*.png")
	if err != nil {
		return nil, err
	}
	name := file.Name()
	if _, err = file.Write(data); err == nil {
		err = file.Close()
	} else {
		_ = file.Close()
	}
	if err != nil {
		_ = os.Remove(name)
		return nil, err
	}
	g := m.guiFor(w.ID)
	g.mu.Lock()
	g.clearImageLocked()
	g.image = &workspaceDesktopImage{ref: observation.Ref, path: name, nonce: w.InstanceNonce, created: started}
	g.mu.Unlock()
	time.AfterFunc(30*time.Second, func() {
		g.mu.Lock()
		defer g.mu.Unlock()
		if g.image != nil && g.image.path == name {
			g.clearImageLocked()
		}
	})
	return map[string]interface{}{"image_ref": observation.Ref, "file_path": "native-desktop:" + observation.Ref, "window_id": r.WindowID, "width": size.Width, "height": size.Height, "expires_in_seconds": 30, "next": "analyze_image with this private file_path, then act once and observe again"}, nil
}

// Opaque image handles are session/mission scoped and never registered as public media.
func (m *WorkspaceManager) ResolveDesktopImage(ctx context.Context, identity WorkspaceIdentity, handle string) (string, error) {
	cfg := m.liveGUIConfig(m.configSnapshot())
	if err := validateWorkspaceFeature(cfg, identity); err != nil {
		return "", err
	}
	if !cfg.AgentControl.NativeAppsEnabled {
		return "", fmt.Errorf("native application control is disabled")
	}
	ref := strings.TrimPrefix(handle, "native-desktop:")
	m.guiMu.Lock()
	states := make(map[string]*workspaceGUI, len(m.gui))
	for id, g := range m.gui {
		states[id] = g
	}
	m.guiMu.Unlock()
	for id, g := range states {
		g.mu.Lock()
		var image workspaceDesktopImage
		if g.image != nil {
			image = *g.image
		}
		g.mu.Unlock()
		if image.ref != ref || image.path == "" {
			continue
		}
		w, err := m.ownedWorkspace(ctx, identity, id)
		if err != nil {
			return "", err
		}
		if image.nonce != w.InstanceNonce || m.now().Sub(image.created) > 30*time.Second || w.State != WorkspaceStateReady || !w.LeaseExpiresAt.After(m.now()) || !w.MaxExpiresAt.After(m.now()) {
			return "", fmt.Errorf("desktop screenshot expired")
		}
		return filepath.Abs(image.path)
	}
	return "", fmt.Errorf("private desktop screenshot is unavailable; take a fresh observation")
}

func ExecuteDesktopTool(ctx context.Context, cfg ToolConfig, identity WorkspaceIdentity, args map[string]interface{}) string {
	if err := validateDesktopToolFields(args); err != nil {
		return workspaceToolResult("", nil, err)
	}
	m := DefaultWorkspaceManager()
	if m == nil {
		return workspaceToolResult("", nil, fmt.Errorf("workspace manager unavailable"))
	}
	// Select only the published fields. A tool caller cannot set the control token,
	// instance, capture dimensions, owner or mission.
	allowed := []string{"operation", "app", "window_id", "element_ref", "root", "surface", "role", "name", "text", "value", "key", "predicate", "timeout_ms", "max_depth", "image_ref", "x", "y", "to_x", "to_y", "delta_x", "delta_y"}
	params := map[string]interface{}{}
	for _, key := range allowed {
		if v, ok := args[key]; ok {
			params[key] = v
		}
	}
	data, err := json.Marshal(params)
	var r DesktopActionRequest
	if err == nil {
		err = json.Unmarshal(data, &r)
	}
	if err != nil {
		return workspaceToolResult("", nil, err)
	}
	result, err := m.DesktopAction(ctx, cfg, identity, toolString(args, "workspace_id"), r)
	if ok, exists := result["ok"].(bool); err == nil && exists && !ok {
		upstream, _ := result["error"].(map[string]interface{})
		code, _ := upstream["code"].(string)
		message, _ := upstream["message"].(string)
		err = WorkspaceRPCError{Code: code, Message: message, Data: result}
	}
	return workspaceToolResult("native application operation completed", result, err)
}

func validateDesktopToolFields(args map[string]interface{}) error {
	required := []string{"workspace_id", "operation"}
	switch toolString(args, "operation") {
	case "launch", "close_app":
		required = append(required, "app")
	case "focus_window", "screenshot":
		required = append(required, "window_id")
	case "press":
		required = append(required, "window_id", "key")
	case "type":
		required = append(required, "element_ref", "text")
	case "set_value", "select":
		required = append(required, "element_ref", "value")
	case "click", "double_click", "right_click", "toggle", "expand", "collapse", "scroll_to", "wait":
		required = append(required, "element_ref")
	case "click_xy", "scroll", "drag":
		required = append(required, "window_id", "image_ref", "x", "y")
		if toolString(args, "operation") == "drag" {
			required = append(required, "to_x", "to_y")
		}
		if toolString(args, "operation") == "scroll" && args["delta_x"] == nil && args["delta_y"] == nil {
			return fmt.Errorf("scroll requires delta_x or delta_y")
		}
	}
	for _, key := range required {
		if value, ok := args[key]; !ok || value == nil {
			return fmt.Errorf("%s is required for this operation", key)
		}
	}
	return nil
}
