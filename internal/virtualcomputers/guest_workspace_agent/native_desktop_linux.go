//go:build linux

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
	"unicode/utf8"
)

const nativeDesktopVersion = "1.3.1"
const nativeDesktopRoot = "/run/aurago/native-desktop"
const nativeDesktopBinary = "/usr/local/bin/agent-desktop"
const nativeOutputLimit = 256 * 1024

type desktopRequest struct {
	Operation    string  `json:"operation"`
	ControlToken string  `json:"control_token,omitempty"`
	App          string  `json:"app,omitempty"`
	WindowID     string  `json:"window_id,omitempty"`
	ElementRef   string  `json:"element_ref,omitempty"`
	Root         string  `json:"root,omitempty"`
	Surface      string  `json:"surface,omitempty"`
	Role         string  `json:"role,omitempty"`
	Name         string  `json:"name,omitempty"`
	Text         string  `json:"text,omitempty"`
	Value        string  `json:"value,omitempty"`
	Key          string  `json:"key,omitempty"`
	Predicate    string  `json:"predicate,omitempty"`
	TimeoutMS    int     `json:"timeout_ms,omitempty"`
	MaxDepth     int     `json:"max_depth,omitempty"`
	ImageRef     string  `json:"image_ref,omitempty"`
	Width        int     `json:"width,omitempty"`
	Height       int     `json:"height,omitempty"`
	X            float64 `json:"x,omitempty"`
	Y            float64 `json:"y,omitempty"`
	ToX          float64 `json:"to_x,omitempty"`
	ToY          float64 `json:"to_y,omitempty"`
	DeltaX       float64 `json:"delta_x,omitempty"`
	DeltaY       float64 `json:"delta_y,omitempty"`
}

type desktopBounds struct {
	X      float64 `json:"x"`
	Y      float64 `json:"y"`
	Width  float64 `json:"width"`
	Height float64 `json:"height"`
}
type desktopWindow struct {
	ID        string        `json:"id"`
	Instance  string        `json:"process_instance"`
	Bounds    desktopBounds `json:"bounds"`
	Focused   bool          `json:"is_focused"`
	Visible   bool          `json:"visible"`
	Minimized bool          `json:"minimized"`
}
type desktopImage struct {
	ID        string        `json:"image_ref"`
	Window    desktopWindow `json:"window"`
	Created   time.Time     `json:"created_at"`
	Width     int           `json:"width"`
	Height    int           `json:"height"`
	Committed bool          `json:"-"`
}
type desktopEnvelope struct {
	Version string          `json:"version,omitempty"`
	OK      bool            `json:"ok"`
	Command string          `json:"command,omitempty"`
	Data    json.RawMessage `json:"data,omitempty"`
	Error   json.RawMessage `json:"error,omitempty"`
}
type nativeDesktop struct {
	mu         sync.Mutex
	images     map[string]desktopImage
	refs       map[string]bool
	run        func(context.Context, []string, string) (desktopEnvelope, error)
	screenSize func(context.Context) (int, int, error)
}

func desktopInput(op string) bool {
	switch op {
	case "status", "list_apps", "list_windows", "list_surfaces", "snapshot", "find", "wait", "screenshot_prepare", "screenshot_commit":
		return false
	}
	return true
}

func nativeFailure(code, message string) error {
	return &rpcError{Code: code, Message: message, Data: map[string]interface{}{"disposition": map[string]string{"delivery": "not_delivered", "retry": "safe"}}}
}

// Arguments are constructed here, never accepted as a command or environment.
func desktopArguments(r desktopRequest) ([]string, string, error) {
	if r.TimeoutMS <= 0 || r.TimeoutMS > 30000 {
		r.TimeoutMS = 10000
	}
	for _, value := range []string{r.App, r.WindowID, r.ElementRef, r.Root, r.Surface, r.Role, r.Name, r.Key, r.Predicate} {
		if strings.HasPrefix(value, "-") || strings.ContainsRune(value, 0) || len(value) > 4096 {
			return nil, "", nativeFailure("invalid_params", "invalid desktop selector")
		}
	}
	args := []string{}
	command := strings.ReplaceAll(r.Operation, "_", "-")
	scope := func() {
		if r.App != "" {
			args = append(args, "--app", r.App)
		}
		if r.WindowID != "" {
			args = append(args, "--window-id", r.WindowID)
		}
	}
	ref := func() error {
		if !regexp.MustCompile(`^@[A-Za-z0-9_-]+:e[0-9]+$`).MatchString(r.ElementRef) {
			return nativeFailure("invalid_ref", "a qualified reference from a fresh snapshot is required")
		}
		return nil
	}
	stdin := ""
	switch r.Operation {
	case "list_apps", "list_windows":
		args = append(args, command)
		if r.App != "" && r.Operation == "list_windows" {
			args = append(args, "--app", r.App)
		}
	case "list_surfaces":
		if r.WindowID != "" {
			return nil, "", nativeFailure("invalid_params", "list_surfaces uses app; use snapshot to scope a window")
		}
		args = append(args, command)
		if r.App != "" {
			args = append(args, "--app", r.App)
		}
	case "snapshot", "find":
		args = append(args, command)
		scope()
		if r.Operation == "snapshot" {
			args = append(args, "--compact", "--include-bounds")
			if r.Root != "" {
				args = append(args, "--root", r.Root)
			}
			if r.Surface != "" {
				args = append(args, "--surface", r.Surface)
			}
			depth := r.MaxDepth
			if depth <= 0 || depth > 20 {
				depth = 8
			}
			args = append(args, "--max-depth", strconv.Itoa(depth))
		} else if r.Operation == "find" {
			if r.Root != "" {
				args = append(args, "--root", r.Root)
			}
			if r.Surface != "" {
				args = append(args, "--surface", r.Surface)
			}
			if r.Role != "" {
				args = append(args, "--role", r.Role)
			}
			if r.Name != "" {
				args = append(args, "--name", r.Name)
			}
			args = append(args, "--exact")
		}
	case "launch", "close_app":
		if r.App == "" || strings.ContainsAny(r.App, "/\\\r\n") {
			return nil, "", nativeFailure("invalid_params", "an application name is required")
		}
		args = append(args, command, r.App)
	case "focus_window":
		if r.WindowID == "" {
			return nil, "", nativeFailure("invalid_params", "window_id is required")
		}
		args = append(args, command, "--window-id", r.WindowID)
	case "press":
		if r.WindowID == "" || r.Key == "" {
			return nil, "", nativeFailure("invalid_params", "key and window_id are required")
		}
		args = append(args, "--headed", command, r.Key)
		scope()
	case "wait":
		if err := ref(); err != nil {
			return nil, "", err
		}
		args = append(args, "wait", "--element", r.ElementRef)
		predicate := r.Predicate
		if predicate == "" {
			predicate = "actionable"
		}
		switch predicate {
		case "actionable", "visible", "enabled", "exists":
		default:
			return nil, "", nativeFailure("invalid_params", "unsupported wait predicate")
		}
		args = append(args, "--predicate", predicate, "--timeout", strconv.Itoa(r.TimeoutMS))
	case "click", "right_click", "double_click", "type", "set_value", "select", "toggle", "expand", "collapse", "scroll_to":
		if err := ref(); err != nil {
			return nil, "", err
		}
		if r.Operation == "double_click" {
			args = append(args, "--headed")
		}
		args = append(args, command, r.ElementRef)
		if r.Operation == "type" || r.Operation == "set_value" {
			stdin = r.Text
			if r.Operation == "set_value" {
				stdin = r.Value
			}
			limit := 10000
			if r.Operation == "set_value" {
				limit = 128 * 1024
			}
			if !utf8.ValidString(stdin) || strings.ContainsRune(stdin, 0) || len(stdin) > limit {
				return nil, "", nativeFailure("invalid_params", "text is not valid or exceeds its byte limit")
			}
			args = append(args, "--stdin")
		}
		// Upstream select accepts a visible option label, not text entry or credentials.
		if r.Operation == "select" {
			if !utf8.ValidString(r.Value) || strings.ContainsRune(r.Value, 0) || len(r.Value) > 4096 {
				return nil, "", nativeFailure("invalid_params", "invalid option label")
			}
			args = append(args, "--timeout-ms", strconv.Itoa(r.TimeoutMS), "--", r.Value)
		}
	case "click_xy", "scroll", "drag":
		for _, v := range []float64{r.X, r.Y, r.ToX, r.ToY, r.DeltaX, r.DeltaY} {
			if math.IsNaN(v) || math.IsInf(v, 0) {
				return nil, "", nativeFailure("invalid_coordinates", "coordinates must be finite")
			}
		}
		point := func(x, y float64) string {
			return strconv.FormatFloat(x, 'f', -1, 64) + "," + strconv.FormatFloat(y, 'f', -1, 64)
		}
		args = append(args, "--headed")
		switch r.Operation {
		case "click_xy":
			args = append(args, "mouse-click", "--xy", point(r.X, r.Y))
		case "drag":
			args = append(args, "drag", "--from-xy", point(r.X, r.Y), "--to-xy", point(r.ToX, r.ToY))
		case "scroll":
			if math.Abs(r.DeltaX) > 100 || math.Abs(r.DeltaY) > 100 {
				return nil, "", nativeFailure("invalid_coordinates", "scroll is limited to 100 lines")
			}
			args = append(args, "mouse-wheel", "--x", fmt.Sprint(r.X), "--y", fmt.Sprint(r.Y), "--dx", fmt.Sprint(r.DeltaX), "--dy", fmt.Sprint(r.DeltaY))
		}
	default:
		return nil, "", nativeFailure("unsupported_operation", "unsupported desktop operation")
	}
	// Only commands whose v1.3.1 parser accepts this option receive it.
	switch r.Operation {
	case "snapshot", "find", "list_apps", "list_windows", "click", "double_click", "right_click", "type", "set_value", "toggle", "expand", "collapse", "scroll_to", "drag":
		args = append(args, "--timeout-ms", strconv.Itoa(r.TimeoutMS))
	}
	return args, stdin, nil
}

type cappedDesktopOutput struct {
	bytes.Buffer
	overflow bool
}

func (b *cappedDesktopOutput) Write(p []byte) (int, error) {
	n := len(p)
	remaining := nativeOutputLimit - b.Len()
	if len(p) > remaining {
		p = p[:remaining]
		b.overflow = true
	}
	_, _ = b.Buffer.Write(p)
	return n, nil
}

func runNativeDesktop(ctx context.Context, args []string, input string) (desktopEnvelope, error) {
	if err := os.MkdirAll(nativeDesktopRoot, 0700); err != nil {
		return desktopEnvelope{}, err
	}
	return runDesktopProcess(ctx, nativeDesktopBinary, args, input)
}

func runDesktopProcess(ctx context.Context, binary string, args []string, input string) (desktopEnvelope, error) {
	ctx, cancel := context.WithTimeout(ctx, 35*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, binary, args...)
	cmd.Env = []string{"PATH=/usr/local/bin:/usr/bin:/bin", "HOME=" + nativeDesktopRoot, "AGENT_DESKTOP_HOME=" + nativeDesktopRoot, "DISPLAY=:0", "NO_AT_BRIDGE=0", "LANG=C.UTF-8", "DBUS_SESSION_BUS_ADDRESS=" + os.Getenv("DBUS_SESSION_BUS_ADDRESS"), "XDG_RUNTIME_DIR=" + nativeDesktopRoot}
	cmd.Stdin = strings.NewReader(input)
	var stdout, stderr cappedDesktopOutput
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	// Upstream releases held X11 input on SIGTERM; allow its two-second release budget.
	cmd.Cancel = func() error {
		if cmd.Process == nil {
			return nil
		}
		return syscall.Kill(-cmd.Process.Pid, syscall.SIGTERM)
	}
	cmd.WaitDelay = 3 * time.Second
	err := cmd.Run()
	if ctx.Err() != nil {
		if cmd.Process != nil {
			// WaitDelay kills the main process; also stop any remaining helpers.
			_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
		}
		return desktopEnvelope{}, &rpcError{Code: "desktop_interrupted", Message: "desktop operation interrupted; observe before another action", Data: map[string]interface{}{"disposition": map[string]string{"delivery": "unknown", "retry": "unsafe"}}}
	}
	if stdout.overflow || stderr.overflow {
		return desktopEnvelope{}, &rpcError{Code: "desktop_output_limit", Message: "observation incomplete; narrow window, surface, root or depth", Data: map[string]interface{}{"complete": false, "disposition": map[string]string{"delivery": "unknown", "retry": "unsafe"}}}
	}
	var result desktopEnvelope
	if json.Unmarshal(stdout.Bytes(), &result) != nil {
		return desktopEnvelope{}, &rpcError{Code: "desktop_invalid_response", Message: "native desktop returned no valid JSON; observe before retrying", Data: map[string]interface{}{"disposition": map[string]string{"delivery": "unknown", "retry": "unsafe"}}}
	}
	if err != nil && result.OK {
		return desktopEnvelope{}, fmt.Errorf("native desktop process failed")
	}
	return result, nil // Preserve the upstream error envelope and its delivery/retry evidence.
}

func (n *nativeDesktop) releaseInput(ctx context.Context) error {
	return releaseDesktopInput(ctx, filepath.Join(nativeDesktopRoot, "agent-desktop-input"), n.call)
}

// v1.3.1 journals held keys/buttons before delivery and removes records only after
// release. Its status/permission probe also recovers abandoned input.
func releaseDesktopInput(ctx context.Context, journal string, run func(context.Context, []string, string) (desktopEnvelope, error)) error {
	entries, err := os.ReadDir(journal)
	if os.IsNotExist(err) || (err == nil && len(entries) == 0) {
		return ctx.Err()
	}
	if err == nil {
		recoveryCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
		_, _ = run(recoveryCtx, []string{"status"}, "")
		cancel()
		entries, err = os.ReadDir(journal)
		if os.IsNotExist(err) || (err == nil && len(entries) == 0) {
			return ctx.Err()
		}
	}
	return &rpcError{Code: "desktop_input_release_unconfirmed", Message: "held input could not be released; restore the display and retry control, or close and recreate the workspace", Data: map[string]interface{}{"disposition": map[string]string{"delivery": "unknown", "retry": "unsafe"}}}
}

func (n *nativeDesktop) reset() { n.mu.Lock(); n.refs = nil; n.images = nil; n.mu.Unlock() }

func (n *nativeDesktop) invalidate() { n.mu.Lock(); n.images = nil; n.mu.Unlock() }
func (n *nativeDesktop) call(ctx context.Context, args []string, input string) (desktopEnvelope, error) {
	if n.run != nil {
		return n.run(ctx, args, input)
	}
	return runNativeDesktop(ctx, args, input)
}
func (n *nativeDesktop) screen(ctx context.Context) (int, int, error) {
	if n.screenSize != nil {
		return n.screenSize(ctx)
	}
	return desktopScreenSize(ctx)
}
func (n *nativeDesktop) window(ctx context.Context, id string) (desktopWindow, error) {
	if id == "" {
		return desktopWindow{}, nativeFailure("window_required", "select one window from list_windows")
	}
	r, err := n.call(ctx, []string{"list-windows", "--timeout-ms", "5000"}, "")
	if err != nil {
		return desktopWindow{}, err
	}
	if !r.OK {
		return desktopWindow{}, &rpcError{Code: "window_unavailable", Message: "window inventory failed", Data: map[string]interface{}{"upstream": r}}
	}
	var windows []desktopWindow
	if json.Unmarshal(r.Data, &windows) != nil {
		return desktopWindow{}, nativeFailure("window_unavailable", "invalid window inventory")
	}
	var found []desktopWindow
	for _, w := range windows {
		if w.ID == id {
			found = append(found, w)
		}
	}
	if len(found) != 1 || !found[0].Visible || found[0].Minimized {
		return desktopWindow{}, nativeFailure("window_unavailable", "window missing, ambiguous or not visible")
	}
	return found[0], nil
}

func desktopScreenSize(ctx context.Context) (int, int, error) {
	cmd := exec.CommandContext(ctx, "/usr/bin/xdpyinfo", "-display", ":0")
	var out cappedDesktopOutput
	cmd.Stdout = &out
	cmd.Stderr = io.Discard
	if err := cmd.Run(); err != nil {
		return 0, 0, err
	}
	m := regexp.MustCompile(`dimensions:\s+(\d+)x(\d+) pixels`).FindStringSubmatch(out.String())
	if len(m) != 3 {
		return 0, 0, fmt.Errorf("display dimensions unavailable")
	}
	w, _ := strconv.Atoi(m[1])
	h, _ := strconv.Atoi(m[2])
	return w, h, nil
}

func (n *nativeDesktop) perform(ctx context.Context, r desktopRequest) (interface{}, error) {
	if r.Operation == "status" {
		if _, err := os.Stat(nativeDesktopBinary); err != nil {
			return map[string]interface{}{"state": "repair_required", "version": nativeDesktopVersion}, nil
		}
		if os.Getenv("DBUS_SESSION_BUS_ADDRESS") == "" {
			return map[string]interface{}{"state": "session_unreachable", "version": nativeDesktopVersion}, nil
		}
		result, err := n.call(ctx, []string{"status"}, "")
		if err != nil {
			return nil, err
		}
		var health struct {
			Version     string `json:"version"`
			Permissions struct {
				Accessibility struct {
					State string `json:"state"`
				} `json:"accessibility"`
			} `json:"permissions"`
		}
		_ = json.Unmarshal(result.Data, &health)
		state := "ready"
		if !result.OK || health.Version != nativeDesktopVersion || health.Permissions.Accessibility.State != "granted" {
			state = "session_unreachable"
		}
		return map[string]interface{}{"state": state, "version": nativeDesktopVersion, "capabilities": []string{"desktop.native.v1"}, "diagnostic": result}, nil
	}
	if r.Operation == "screenshot_prepare" {
		w, err := n.window(ctx, r.WindowID)
		if err != nil {
			return nil, err
		}
		width, height, err := n.screen(ctx)
		if err != nil {
			return nil, err
		}
		id, err := randomID()
		if err != nil {
			return nil, err
		}
		image := desktopImage{ID: id, Window: w, Created: time.Now().UTC(), Width: width, Height: height}
		n.mu.Lock()
		n.images = map[string]desktopImage{id: image}
		n.mu.Unlock()
		return image, nil
	}
	if r.Operation == "screenshot_commit" || r.Operation == "click_xy" || r.Operation == "scroll" || r.Operation == "drag" {
		n.mu.Lock()
		image, ok := n.images[r.ImageRef]
		n.mu.Unlock()
		if !ok || time.Since(image.Created) > 30*time.Second || r.WindowID != image.Window.ID {
			return nil, nativeFailure("stale_image", "take and analyze a fresh screenshot of this window")
		}
		w, err := n.window(ctx, r.WindowID)
		if err != nil {
			return nil, err
		}
		width, height, err := n.screen(ctx)
		if err != nil {
			return nil, err
		}
		if time.Since(image.Created) > 30*time.Second || w.Bounds != image.Window.Bounds || w.Instance != image.Window.Instance || width != image.Width || height != image.Height {
			return nil, nativeFailure("stale_image", "window geometry or display changed; take a fresh screenshot")
		}
		if r.Operation == "screenshot_commit" {
			if r.Width != width || r.Height != height {
				return nil, nativeFailure("image_size_mismatch", "VM screenshot does not match the visible display")
			}
			image.Committed = true
			n.mu.Lock()
			n.images[r.ImageRef] = image
			n.mu.Unlock()
			return image, nil
		}
		inside := func(x, y float64) bool {
			return x >= 0 && y >= 0 && x < float64(width) && y < float64(height) && x >= w.Bounds.X && y >= w.Bounds.Y && x < w.Bounds.X+w.Bounds.Width && y < w.Bounds.Y+w.Bounds.Height
		}
		if !image.Committed || !w.Focused || !inside(r.X, r.Y) || (r.Operation == "drag" && !inside(r.ToX, r.ToY)) {
			return nil, nativeFailure("invalid_coordinates", "target must be focused and coordinates must stay inside the captured window")
		}
	}
	if r.ElementRef != "" || r.Root != "" {
		n.mu.Lock()
		known := (r.ElementRef == "" || n.refs[strings.Split(r.ElementRef, ":")[0]]) && (r.Root == "" || n.refs[strings.Split(r.Root, ":")[0]])
		n.mu.Unlock()
		if !known {
			return nil, nativeFailure("stale_ref", "observe again after a control change")
		}
	}
	args, input, err := desktopArguments(r)
	if err != nil {
		return nil, err
	}
	if desktopInput(r.Operation) {
		n.invalidate()
	}
	result, err := n.call(ctx, args, input)
	if err == nil && result.OK && (r.Operation == "snapshot" || r.Operation == "find") {
		var meta struct {
			SnapshotID string `json:"snapshot_id"`
		}
		_ = json.Unmarshal(result.Data, &meta)
		n.mu.Lock()
		if n.refs == nil {
			n.refs = make(map[string]bool)
		}
		if len(n.refs) > 64 {
			n.refs = make(map[string]bool)
		}
		if meta.SnapshotID != "" {
			n.refs["@"+meta.SnapshotID] = true
		}
		n.mu.Unlock()
	}
	return result, err
}
