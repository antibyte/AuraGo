package gamemaker

import (
	"context"
	"encoding/base64"
	"fmt"
	"math"
	"mime"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"aurago/internal/security"
	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/input"
	"github.com/go-rod/rod/lib/launcher"
	"github.com/go-rod/rod/lib/proto"
)

type PreviewInput struct {
	Type         string  `json:"type"`
	Key          string  `json:"key,omitempty"`
	X            float64 `json:"x,omitempty"`
	Y            float64 `json:"y,omitempty"`
	Button       string  `json:"button,omitempty"`
	Milliseconds int     `json:"milliseconds,omitempty"`
}

type AgentCapture struct {
	ID      string `json:"id"`
	BuildID string `json:"build_id"`
	Image   string `json:"-"`
}

type agentPreview struct {
	browser   *security.EgressBrowser
	cancel    context.CancelFunc
	page      *rod.Page
	server    *http.Server
	url       string
	directory string
	buildID   string
	mu        sync.Mutex
	logs      []Diagnostic
	emitted   int
	captures  []AgentCapture
	pending   []AgentCapture
	requests  map[proto.NetworkRequestID]string
}

// AgentPreview operates a job-owned browser; it never uses the Studio iframe.
func (s *Service) AgentPreview(ctx context.Context, jobID, operation string, actions []PreviewInput) (map[string]any, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	if err := s.CheckJobMutation(ctx, jobID); err != nil {
		return nil, err
	}
	if operation == "close" {
		s.closeAgentPreview(jobID)
		return map[string]any{"status": "closed"}, nil
	}
	s.previewMu.Lock()
	defer s.previewMu.Unlock()
	p := s.agentPreviews[jobID]
	if p != nil {
		select {
		case <-p.browser.Done:
			err := p.browser.Err()
			s.disposeAgentPreview(p)
			delete(s.agentPreviews, jobID)
			if operation != "open" {
				return nil, fmt.Errorf("preview browser stopped: %v; use open to retry", err)
			}
			p = nil
		default:
		}
	}
	if operation == "reload" || operation == "open" && p == nil {
		result := s.BuildJob(ctx, jobID)
		if !result.OK {
			return map[string]any{"status": "build_failed", "result": result}, nil
		}
		stage, err := s.JobDirectory(jobID)
		if err != nil {
			return nil, err
		}
		dir, err := os.MkdirTemp("", "aurago-game-preview-*")
		if err != nil {
			return nil, err
		}
		s.fileMu.Lock()
		err = copyTree(stage, dir, s.opts.MaxFilesPerProject, s.opts.MaxProjectBytes)
		if err == nil {
			var fingerprint string
			fingerprint, err = validationFingerprint(dir)
			if err == nil && fingerprint != result.fingerprint {
				err = fmt.Errorf("draft changed during preview build; reload again")
			}
		}
		s.fileMu.Unlock()
		if err != nil {
			_ = os.RemoveAll(dir)
			return nil, err
		}
		if p == nil {
			p, err = s.newAgentPreview(ctx, jobID)
			if err != nil {
				_ = os.RemoveAll(dir)
				return nil, fmt.Errorf("open preview: %w", err)
			}
			s.agentPreviews[jobID] = p
		}
		if p.page != nil {
			_ = p.page.Close()
		}
		p.mu.Lock()
		old := p.directory
		p.directory, p.buildID, p.logs, p.emitted = dir, result.BuildID, nil, 0
		p.requests = map[proto.NetworkRequestID]string{}
		p.mu.Unlock()
		if old != "" {
			_ = os.RemoveAll(old)
		}
		if result.sourceMap != nil {
			copyMap := *result.sourceMap
			copyMap.root = dir
			result.sourceMap = &copyMap
		}
		if err := s.openAgentPage(ctx, jobID, p, result); err != nil {
			s.disposeAgentPreview(p)
			delete(s.agentPreviews, jobID)
			return nil, fmt.Errorf("load preview: %w", err)
		}
	}
	if p == nil {
		return nil, fmt.Errorf("preview is closed; call open first")
	}
	response := map[string]any{"status": "ok", "build_id": p.buildID}
	switch operation {
	case "open", "reload":
	case "logs":
		p.mu.Lock()
		response["diagnostics"] = append([]Diagnostic(nil), p.logs...)
		p.mu.Unlock()
	case "screenshot":
		quality := 80
		data, err := p.page.Context(ctx).Screenshot(false, &proto.PageCaptureScreenshot{Format: proto.PageCaptureScreenshotFormatJpeg, Quality: &quality})
		if err != nil {
			return nil, p.browser.Wrap(err)
		}
		if len(data) > 2<<20 {
			return nil, fmt.Errorf("preview image exceeds 2 MiB; reduce viewport content")
		}
		capture := AgentCapture{ID: randomID("capture"), BuildID: p.buildID, Image: "data:image/jpeg;base64," + base64.StdEncoding.EncodeToString(data)}
		p.mu.Lock()
		pendingBytes := len(capture.Image)
		for _, previous := range p.pending {
			pendingBytes += len(previous.Image)
		}
		if pendingBytes > 8<<20 {
			p.mu.Unlock()
			return nil, fmt.Errorf("pending preview images exceed 8 MiB; consume images before capturing again")
		}
		p.pending = append(p.pending, capture)
		p.captures = append(p.captures, capture)
		if len(p.captures) > 3 {
			p.captures = append([]AgentCapture(nil), p.captures[len(p.captures)-3:]...)
		}
		p.mu.Unlock()
		response["capture"] = capture
		job, _ := s.GetJob(ctx, jobID)
		_, _ = s.emit(ctx, job.ProjectID, jobID, "agent_capture", map[string]any{"id": capture.ID, "build_id": capture.BuildID, "url": "/api/game-maker/projects/" + job.ProjectID + "/agent-captures/" + jobID + "/" + capture.ID})
	case "input":
		if len(actions) == 0 || len(actions) > 64 {
			return nil, fmt.Errorf("input requires 1–64 actions")
		}
		for _, action := range actions {
			if err := previewInput(ctx, p.page, action); err != nil {
				return nil, p.browser.Wrap(err)
			}
		}
	default:
		return nil, fmt.Errorf("unknown preview operation")
	}
	return response, nil
}

func (s *Service) newAgentPreview(ctx context.Context, jobID string) (*agentPreview, error) {
	s.mu.RLock()
	jobCtx := s.jobContexts[jobID]
	s.mu.RUnlock()
	if jobCtx == nil {
		return nil, fmt.Errorf("job is not active")
	}
	bin, found := launcher.LookPath()
	if !found {
		return nil, fmt.Errorf("Chromium is unavailable; install Chrome or Chromium and retry open")
	}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, err
	}
	p := &agentPreview{url: "http://" + listener.Addr().String() + "/" + randomID("preview") + "/"}
	base, _ := url.Parse(p.url)
	p.server = &http.Server{ReadHeaderTimeout: 5 * time.Second, Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.Host != base.Host || !strings.HasPrefix(r.URL.Path, base.Path) {
			http.NotFound(w, r)
			return
		}
		rel := strings.TrimPrefix(r.URL.Path, base.Path)
		if rel == "" {
			rel = "index.html"
		}
		if strings.HasPrefix(rel, ".") || strings.Contains(rel, "/.") {
			http.NotFound(w, r)
			return
		}
		p.mu.Lock()
		path, _, err := secureJoin(p.directory, rel, true)
		var data []byte
		if err == nil {
			data, err = os.ReadFile(path)
		}
		p.mu.Unlock()
		if err != nil {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Content-Security-Policy", "default-src 'self' data: blob:; script-src 'self' 'unsafe-inline' 'unsafe-eval' blob:; style-src 'self' 'unsafe-inline'; connect-src 'self'; object-src 'none'; base-uri 'none'; form-action 'none'; frame-src 'none'; worker-src 'self' blob:")
		if contentType := mime.TypeByExtension(filepath.Ext(rel)); contentType != "" {
			w.Header().Set("Content-Type", contentType)
		}
		_, _ = w.Write(data)
	})}
	go func() { _ = p.server.Serve(listener) }()
	lifetime := s.opts.JobTimeout
	if deadline, ok := jobCtx.Deadline(); ok {
		lifetime = time.Until(deadline)
	}
	browserCtx, cancel := context.WithCancel(jobCtx)
	p.cancel = cancel
	stopStartup := context.AfterFunc(ctx, cancel)
	p.browser, err = security.LaunchEgressBrowser(browserCtx, security.EgressBrowserOptions{AllowedOrigin: base, Lifetime: lifetime, Bin: bin})
	if !stopStartup() && err == nil {
		err = ctx.Err()
	}
	if err != nil {
		s.disposeAgentPreview(p)
		return nil, err
	}
	router := p.browser.Browser.HijackRequests()
	if err := router.Add("*", "", func(h *rod.Hijack) {
		u := h.Request.URL()
		if u.Scheme == "data" || u.Scheme == "blob" || u.Scheme == base.Scheme && u.Host == base.Host && strings.HasPrefix(u.Path, base.Path) {
			h.ContinueRequest(&proto.FetchContinueRequest{})
		} else {
			h.Response.Fail(proto.NetworkErrorReasonBlockedByClient)
		}
	}); err != nil {
		s.disposeAgentPreview(p)
		return nil, fmt.Errorf("restrict preview requests: %w", err)
	}
	go router.Run()
	go func() { <-jobCtx.Done(); s.closeAgentPreview(jobID) }()
	return p, nil
}

func (s *Service) openAgentPage(ctx context.Context, jobID string, p *agentPreview, result BuildResult) error {
	page, err := p.browser.Browser.Page(proto.TargetCreateTarget{URL: "about:blank"})
	if err != nil {
		return err
	}
	p.page = page
	if err := page.SetViewport(&proto.EmulationSetDeviceMetricsOverride{Width: 1024, Height: 768, DeviceScaleFactor: 1, Mobile: false}); err != nil {
		return err
	}
	job, _ := s.GetJob(ctx, jobID)
	relative := func(raw string) string { return strings.TrimPrefix(raw, p.url) }
	log := func(message string, frames []RuntimeFrame) {
		message = s.RedactHostPaths(message)
		message = strings.ReplaceAll(message, p.url, "")
		if len(message) > 4096 {
			message = message[:4096]
		}
		d := result.sourceMap.diagnostic(message, frames)
		if d.File == "" && len(frames) > 0 {
			d.File, d.Line, d.Column = frames[0].File, frames[0].Line, frames[0].Column
		}
		p.mu.Lock()
		if p.buildID != result.BuildID {
			p.mu.Unlock()
			return
		}
		p.logs = append(p.logs, d)
		if len(p.logs) > 200 {
			p.logs = append([]Diagnostic(nil), p.logs[len(p.logs)-200:]...)
		}
		p.emitted++
		emit := p.emitted <= 200
		p.mu.Unlock()
		if emit {
			_, _ = s.emit(context.Background(), job.ProjectID, jobID, "preview_diagnostic", map[string]any{"build_id": result.BuildID, "diagnostic": d})
		}
	}
	go page.EachEvent(
		func(e *proto.RuntimeExceptionThrown) {
			d := e.ExceptionDetails
			message := d.Text
			if d.Exception != nil && d.Exception.Description != "" {
				message = d.Exception.Description
			}
			frames := []RuntimeFrame{{File: relative(d.URL), Line: d.LineNumber + 1, Column: d.ColumnNumber + 1}}
			if d.StackTrace != nil {
				for _, f := range d.StackTrace.CallFrames {
					frames = append(frames, RuntimeFrame{relative(f.URL), f.LineNumber + 1, f.ColumnNumber + 1})
				}
			}
			log(message, frames)
		},
		func(e *proto.RuntimeConsoleAPICalled) {
			if e.Type != proto.RuntimeConsoleAPICalledTypeError && e.Type != proto.RuntimeConsoleAPICalledTypeWarning {
				return
			}
			var parts []string
			for _, a := range e.Args {
				text := a.Description
				if text == "" {
					text = a.Value.String()
				}
				parts = append(parts, text)
			}
			log(strings.Join(parts, " "), nil)
		},
		func(e *proto.NetworkRequestWillBeSent) {
			p.mu.Lock()
			if p.buildID == result.BuildID && len(p.requests) < 2048 {
				p.requests[e.RequestID] = relative(e.Request.URL)
			}
			p.mu.Unlock()
		},
		func(e *proto.NetworkLoadingFailed) {
			p.mu.Lock()
			path := p.requests[e.RequestID]
			delete(p.requests, e.RequestID)
			p.mu.Unlock()
			log("Resource failed: "+path+": "+e.ErrorText, nil)
		},
		func(e *proto.NetworkResponseReceived) {
			if e.Response.Status >= 400 {
				log(fmt.Sprintf("Resource HTTP %d: %s", e.Response.Status, relative(e.Response.URL)), nil)
			}
		},
		func(e *proto.NetworkLoadingFinished) { p.mu.Lock(); delete(p.requests, e.RequestID); p.mu.Unlock() },
	)()
	if err := page.Context(ctx).Navigate(p.url + "index.html"); err != nil {
		return err
	}
	return page.Context(ctx).WaitLoad()
}

func previewInput(ctx context.Context, page *rod.Page, action PreviewInput) error {
	page = page.Context(ctx)
	if math.IsNaN(action.X) || math.IsInf(action.X, 0) || math.IsNaN(action.Y) || math.IsInf(action.Y, 0) || action.X < 0 || action.X > 1024 || action.Y < 0 || action.Y > 768 {
		return fmt.Errorf("mouse coordinates must be inside the 1024×768 viewport")
	}
	button := proto.InputMouseButton(action.Button)
	if button == "" {
		button = proto.InputMouseButtonLeft
	}
	if button != proto.InputMouseButtonLeft && button != proto.InputMouseButtonRight && button != proto.InputMouseButtonMiddle {
		return fmt.Errorf("invalid mouse button")
	}
	var key input.Key
	if strings.HasPrefix(action.Type, "key_") {
		keys := map[string]input.Key{"ArrowLeft": input.ArrowLeft, "ArrowRight": input.ArrowRight, "ArrowUp": input.ArrowUp, "ArrowDown": input.ArrowDown, "Space": input.Space, "Enter": input.Enter, "Escape": input.Escape, "Tab": input.Tab, "Shift": input.ShiftLeft, "Control": input.ControlLeft, "Alt": input.AltLeft, "Backspace": input.Backspace}
		var ok bool
		key, ok = keys[action.Key]
		if !ok {
			if len(action.Key) != 1 || action.Key[0] < 32 || action.Key[0] > 126 {
				return fmt.Errorf("unsupported browser key %q", action.Key)
			}
			key = input.Key(action.Key[0])
		}
	}
	switch action.Type {
	case "key_down":
		return page.Keyboard.Press(key)
	case "key_up":
		return page.Keyboard.Release(key)
	case "key_press":
		if err := page.Keyboard.Press(key); err != nil {
			return err
		}
		return page.Keyboard.Release(key)
	case "mouse_move":
		return page.Mouse.MoveTo(proto.Point{X: action.X, Y: action.Y})
	case "mouse_down":
		return page.Mouse.Down(button, 1)
	case "mouse_up":
		return page.Mouse.Up(button, 1)
	case "click":
		if err := page.Mouse.MoveTo(proto.Point{X: action.X, Y: action.Y}); err != nil {
			return err
		}
		return page.Mouse.Click(button, 1)
	case "wait":
		if action.Milliseconds < 0 || action.Milliseconds > 10000 {
			return fmt.Errorf("wait must be 0–10000 milliseconds")
		}
		timer := time.NewTimer(time.Duration(action.Milliseconds) * time.Millisecond)
		defer timer.Stop()
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-timer.C:
			return nil
		}
	default:
		return fmt.Errorf("unknown input action")
	}
}

func (s *Service) TakeAgentCaptures(jobID string) []AgentCapture {
	s.previewMu.Lock()
	defer s.previewMu.Unlock()
	if p := s.agentPreviews[jobID]; p != nil {
		p.mu.Lock()
		defer p.mu.Unlock()
		captures := p.pending
		p.pending = nil
		return captures
	}
	return nil
}

func (s *Service) AgentCaptureImage(ctx context.Context, projectID, jobID, captureID string) ([]byte, error) {
	job, err := s.GetJob(ctx, jobID)
	if err != nil || job.ProjectID != projectID {
		return nil, ErrNotFound
	}
	s.previewMu.Lock()
	defer s.previewMu.Unlock()
	if p := s.agentPreviews[jobID]; p != nil {
		p.mu.Lock()
		defer p.mu.Unlock()
		for _, c := range p.captures {
			if c.ID == captureID {
				return base64.StdEncoding.DecodeString(strings.TrimPrefix(c.Image, "data:image/jpeg;base64,"))
			}
		}
	}
	return nil, ErrNotFound
}

func (s *Service) closeAgentPreview(jobID string) {
	s.previewMu.Lock()
	defer s.previewMu.Unlock()
	if p := s.agentPreviews[jobID]; p != nil {
		s.disposeAgentPreview(p)
		delete(s.agentPreviews, jobID)
	}
}

func (s *Service) disposeAgentPreview(p *agentPreview) {
	if p.cancel != nil {
		p.cancel()
	}
	if p.browser != nil {
		_ = p.browser.Close()
	}
	if p.server != nil {
		_ = p.server.Close()
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.directory != "" {
		_ = os.RemoveAll(p.directory)
	}
	p.captures, p.pending = nil, nil
}
