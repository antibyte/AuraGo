package gamemaker

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	"math"
	"mime"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"aurago/internal/security"
	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/input"
	"github.com/go-rod/rod/lib/launcher"
	"github.com/go-rod/rod/lib/proto"
	"golang.org/x/image/draw"
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
	browser      *security.EgressBrowser
	cancel       context.CancelFunc
	page         *rod.Page
	server       *http.Server
	url          string
	directory    string
	dimension    string
	buildID      string
	width        int
	height       int
	mobile       bool
	touchEnabled bool
	mu           sync.Mutex
	logs         []Diagnostic
	emitted      int
	captures     []AgentCapture
	pending      []AgentCapture
	requests     map[proto.NetworkRequestID]string
}

func parsePreviewViewport(spec string) (width, height int, mobile, specified bool, err error) {
	trimmed := strings.ToLower(strings.TrimSpace(spec))
	if trimmed == "" {
		return 0, 0, false, false, nil
	}
	switch trimmed {
	case "landscape", "desktop":
		return 1024, 768, false, true, nil
	case "hd", "widescreen":
		return 1280, 720, false, true, nil
	case "portrait", "mobile":
		return 430, 932, true, true, nil
	case "tablet":
		return 768, 1024, true, true, nil
	}
	normalized := strings.ReplaceAll(trimmed, "×", "x")
	parts := strings.Split(normalized, "x")
	if len(parts) != 2 {
		return 0, 0, false, false, fmt.Errorf("invalid viewport %q; use landscape, hd, portrait, tablet or WIDTHxHEIGHT (320..1920)", spec)
	}
	w, errW := strconv.Atoi(strings.TrimSpace(parts[0]))
	h, errH := strconv.Atoi(strings.TrimSpace(parts[1]))
	if errW != nil || errH != nil || w < 320 || w > 1920 || h < 320 || h > 1920 {
		return 0, 0, false, false, fmt.Errorf("viewport dimensions must be 320..1920")
	}
	return w, h, w < h, true, nil
}

func applyPreviewViewport(ctx context.Context, page *rod.Page, width, height int, mobile bool) error {
	if page == nil {
		return nil
	}
	pctx := page.Context(ctx)
	if err := pctx.SetViewport(&proto.EmulationSetDeviceMetricsOverride{
		Width:             width,
		Height:            height,
		DeviceScaleFactor: 1,
		Mobile:            mobile,
	}); err != nil {
		return err
	}
	if mobile {
		_ = proto.EmulationSetTouchEmulationEnabled{Enabled: true}.Call(pctx)
	}
	return nil
}

// AgentPreview operates a job-owned browser; it never uses the Studio iframe.
func (s *Service) AgentPreview(ctx context.Context, jobID, operation string, actions []PreviewInput, viewport ...string) (map[string]any, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	if err := s.CheckJobMutation(ctx, jobID); err != nil {
		return nil, err
	}
	var vw, vh int
	var vmobile, vspecified bool
	if len(viewport) > 0 && strings.TrimSpace(viewport[0]) != "" {
		var err error
		vw, vh, vmobile, vspecified, err = parsePreviewViewport(viewport[0])
		if err != nil {
			return nil, err
		}
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
	reloaded := false
	if operation == "reload" || (operation == "open" && p == nil) {
		result := s.BuildJob(ctx, jobID)
		if !result.OK {
			return map[string]any{"status": "build_failed", "result": result}, nil
		}
		project, _, _ := s.ProjectForJob(ctx, jobID)
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
			p, err = s.newAgentPreview(ctx, jobID, project.Dimension)
			if err != nil {
				_ = os.RemoveAll(dir)
				return nil, fmt.Errorf("open preview: %w", err)
			}
			s.agentPreviews[jobID] = p
		}
		if vspecified {
			p.width, p.height, p.mobile = vw, vh, vmobile
		}
		if p.page != nil {
			_ = p.page.Close()
		}
		p.mu.Lock()
		old := p.directory
		p.directory, p.dimension, p.buildID, p.logs, p.emitted = dir, project.Dimension, result.BuildID, nil, 0
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
		reloaded = true
	}
	if p == nil {
		return nil, fmt.Errorf("preview is closed; call open first")
	}
	if vspecified && !reloaded {
		if err := applyPreviewViewport(ctx, p.page, vw, vh, vmobile); err != nil {
			return nil, p.browser.Wrap(err)
		}
		p.width, p.height, p.mobile = vw, vh, vmobile
		if vmobile {
			p.touchEnabled = true
		}
	}
	response := map[string]any{
		"status":   "ok",
		"build_id": p.buildID,
		"viewport": map[string]any{"width": p.width, "height": p.height, "mobile": p.mobile},
	}
	switch operation {
	case "open", "reload":
	case "logs":
		p.mu.Lock()
		response["diagnostics"] = append([]Diagnostic(nil), p.logs...)
		p.mu.Unlock()
	case "screenshot":
		data, err := capturePreviewJPEG(ctx, p.page)
		if err != nil {
			return nil, p.browser.Wrap(err)
		}
		capture, err := s.recordAgentCapture(ctx, jobID, p, data)
		if err != nil {
			return nil, err
		}
		response["capture"] = capture
	case "input":
		if len(actions) == 0 || len(actions) > 64 {
			return nil, fmt.Errorf("input requires 1–64 actions")
		}
		captureCount := 0
		for _, a := range actions {
			if a.Type == "capture" {
				captureCount++
			}
		}
		if captureCount > 6 {
			return nil, fmt.Errorf("input supports at most 6 capture actions per call")
		}
		var frames [][]byte
		for _, action := range actions {
			if action.Type == "capture" {
				frame, err := capturePreviewJPEG(ctx, p.page)
				if err != nil {
					return nil, p.browser.Wrap(err)
				}
				frames = append(frames, frame)
				continue
			}
			if (strings.HasPrefix(action.Type, "touch_") || action.Type == "tap") && !p.touchEnabled {
				_ = proto.EmulationSetTouchEmulationEnabled{Enabled: true}.Call(p.page.Context(ctx))
				p.touchEnabled = true
			}
			if err := previewInput(ctx, p.page, action, p.width, p.height); err != nil {
				return nil, p.browser.Wrap(err)
			}
		}
		if len(frames) == 1 {
			capture, err := s.recordAgentCapture(ctx, jobID, p, frames[0])
			if err != nil {
				return nil, err
			}
			response["capture"] = capture
			response["capture_count"] = 1
		} else if len(frames) > 1 {
			sheet, err := stitchContactSheet(frames)
			if err != nil {
				return nil, err
			}
			capture, err := s.recordAgentCapture(ctx, jobID, p, sheet)
			if err != nil {
				return nil, err
			}
			response["capture"] = capture
			response["capture_count"] = len(frames)
			response["contact_sheet"] = true
		}
	case "probe":
		probe, err := s.probeAgentPage(ctx, p.page)
		if err != nil {
			return nil, p.browser.Wrap(err)
		}
		p.mu.Lock()
		logCount := len(p.logs)
		p.mu.Unlock()
		response["probe"] = probe
		response["log_count"] = logCount
	default:
		return nil, fmt.Errorf("unknown preview operation")
	}
	return response, nil
}

func capturePreviewJPEG(ctx context.Context, page *rod.Page) ([]byte, error) {
	quality := 80
	return page.Context(ctx).Screenshot(false, &proto.PageCaptureScreenshot{
		Format:  proto.PageCaptureScreenshotFormatJpeg,
		Quality: &quality,
	})
}

func (s *Service) recordAgentCapture(ctx context.Context, jobID string, p *agentPreview, data []byte) (AgentCapture, error) {
	if len(data) > 2<<20 {
		return AgentCapture{}, fmt.Errorf("preview image exceeds 2 MiB; reduce viewport content")
	}
	capture := AgentCapture{ID: randomID("capture"), BuildID: p.buildID, Image: "data:image/jpeg;base64," + base64.StdEncoding.EncodeToString(data)}
	p.mu.Lock()
	pendingBytes := len(capture.Image)
	for _, previous := range p.pending {
		pendingBytes += len(previous.Image)
	}
	if pendingBytes > 8<<20 {
		p.mu.Unlock()
		return AgentCapture{}, fmt.Errorf("pending preview images exceed 8 MiB; consume images before capturing again")
	}
	p.pending = append(p.pending, capture)
	p.captures = append(p.captures, capture)
	if len(p.captures) > 3 {
		p.captures = append([]AgentCapture(nil), p.captures[len(p.captures)-3:]...)
	}
	p.mu.Unlock()
	job, _ := s.GetJob(ctx, jobID)
	_, _ = s.emit(ctx, job.ProjectID, jobID, "agent_capture", map[string]any{"id": capture.ID, "build_id": capture.BuildID, "url": "/api/game-maker/projects/" + job.ProjectID + "/agent-captures/" + jobID + "/" + capture.ID})
	return capture, nil
}

func (s *Service) probeAgentPage(ctx context.Context, page *rod.Page) (map[string]any, error) {
	res, err := page.Context(ctx).Eval(`() => {
		const out = { title: document.title || '', hud: '', canvasCount: 0, canvas: null, debug: null, debugError: '' };
		const hud = document.getElementById('hud') || document.querySelector('[data-hud="true"], .aurago-player-hud');
		if (hud) out.hud = String(hud.textContent || '').trim().slice(0, 512);
		const canvases = document.querySelectorAll('canvas');
		out.canvasCount = canvases.length;
		if (canvases.length > 0) {
			const c = canvases[0];
			const r = c.getBoundingClientRect();
			out.canvas = { width: c.width, height: c.height, clientWidth: Math.round(r.width), clientHeight: Math.round(r.height) };
		}
		try {
			const raw = typeof window.__gameDebug === 'function' ? window.__gameDebug() : window.__gameDebug;
			if (raw !== undefined) {
				const encoded = JSON.stringify(raw);
				if (encoded !== undefined) {
					out.debug = encoded.length > 16384 ? encoded.slice(0, 16384) + '...(truncated)' : JSON.parse(encoded);
				}
			}
		} catch (e) {
			out.debugError = String(e && e.message ? e.message : e).slice(0, 512);
		}
		return out;
	}`)
	if err != nil {
		return nil, err
	}
	raw, err := res.Value.MarshalJSON()
	if err != nil {
		return nil, err
	}
	redacted := s.RedactHostPaths(string(raw))
	var parsed map[string]any
	if err := json.Unmarshal([]byte(redacted), &parsed); err != nil {
		return map[string]any{"raw": redacted}, nil
	}
	return parsed, nil
}

func stitchContactSheet(frames [][]byte) ([]byte, error) {
	if len(frames) == 0 {
		return nil, fmt.Errorf("no frames to stitch")
	}
	decoded := make([]image.Image, 0, len(frames))
	for i, raw := range frames {
		img, err := jpeg.Decode(bytes.NewReader(raw))
		if err != nil {
			return nil, fmt.Errorf("decode capture frame %d: %w", i+1, err)
		}
		decoded = append(decoded, img)
	}
	cols := 2
	if len(decoded) == 3 || len(decoded) >= 5 {
		cols = 3
	}
	rows := (len(decoded) + cols - 1) / cols
	const gap = 8
	srcW := max(1, decoded[0].Bounds().Dx())
	srcH := max(1, decoded[0].Bounds().Dy())
	panelW := min(srcW, max(160, (1280-gap*(cols+1))/cols))
	panelH := max(90, panelW*srcH/srcW)
	canvasW := cols*panelW + (cols+1)*gap
	canvasH := rows*panelH + (rows+1)*gap
	sheet := image.NewRGBA(image.Rect(0, 0, canvasW, canvasH))
	bg := color.RGBA{R: 11, G: 18, B: 32, A: 255}
	draw.Draw(sheet, sheet.Bounds(), &image.Uniform{C: bg}, image.Point{}, draw.Src)

	for i, src := range decoded {
		col := i % cols
		row := i / cols
		ox := gap + col*(panelW+gap)
		oy := gap + row*(panelH+gap)
		dstRect := image.Rect(ox, oy, ox+panelW, oy+panelH)
		draw.ApproxBiLinear.Scale(sheet, dstRect, src, src.Bounds(), draw.Over, nil)
		drawFrameBadge(sheet, ox+6, oy+6, i+1)
	}

	for _, quality := range []int{80, 65, 50} {
		var buf bytes.Buffer
		if err := jpeg.Encode(&buf, sheet, &jpeg.Options{Quality: quality}); err != nil {
			return nil, fmt.Errorf("encode contact sheet: %w", err)
		}
		if buf.Len() <= 2<<20 || quality == 50 {
			return buf.Bytes(), nil
		}
	}
	return nil, fmt.Errorf("contact sheet exceeded size limit")
}

// 3x5 pixel bitmaps for digits 1..9 to label contact sheet panels cleanly.
var frameDigitGlyphs = map[int][5]uint8{
	1: {0b010, 0b110, 0b010, 0b010, 0b111},
	2: {0b111, 0b001, 0b111, 0b100, 0b111},
	3: {0b111, 0b001, 0b111, 0b001, 0b111},
	4: {0b101, 0b101, 0b111, 0b001, 0b001},
	5: {0b111, 0b100, 0b111, 0b001, 0b111},
	6: {0b111, 0b100, 0b111, 0b101, 0b111},
}

func drawFrameBadge(img *image.RGBA, x, y, number int) {
	const scale = 3
	badgeW, badgeH := 20, 22
	badgeRect := image.Rect(x, y, x+badgeW, y+badgeH).Intersect(img.Bounds())
	draw.Draw(img, badgeRect, &image.Uniform{C: color.RGBA{R: 15, G: 23, B: 42, A: 235}}, image.Point{}, draw.Over)
	glyph, ok := frameDigitGlyphs[number]
	if !ok {
		return
	}
	fg := color.RGBA{R: 56, G: 189, B: 248, A: 255}
	gx, gy := x+5, y+4
	for row := 0; row < 5; row++ {
		bits := glyph[row]
		for col := 0; col < 3; col++ {
			if bits&(1<<(2-col)) != 0 {
				pxRect := image.Rect(gx+col*scale, gy+row*scale, gx+(col+1)*scale, gy+(row+1)*scale).Intersect(img.Bounds())
				draw.Draw(img, pxRect, &image.Uniform{C: fg}, image.Point{}, draw.Src)
			}
		}
	}
}

func (s *Service) newAgentPreview(ctx context.Context, jobID, dimension string) (*agentPreview, error) {
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
	p := &agentPreview{
		url:       "http://" + listener.Addr().String() + "/" + randomID("preview") + "/",
		dimension: dimension,
		width:     1024,
		height:    768,
	}
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
		dim := p.dimension
		p.mu.Unlock()
		if err != nil {
			if os.IsNotExist(err) {
				if fallback, bundled, fallbackErr := bundledRuntimeFileAny(dim, rel); bundled {
					if fallbackErr != nil {
						http.NotFound(w, r)
						return
					}
					data = fallback
					err = nil
				}
			}
		}
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
	if p.width <= 0 {
		p.width = 1024
	}
	if p.height <= 0 {
		p.height = 768
	}
	if err := applyPreviewViewport(ctx, page, p.width, p.height, p.mobile); err != nil {
		return err
	}
	if p.mobile {
		p.touchEnabled = true
	} else {
		p.touchEnabled = false
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

func previewInput(ctx context.Context, page *rod.Page, action PreviewInput, width, height int) error {
	page = page.Context(ctx)
	if width <= 0 {
		width = 1024
	}
	if height <= 0 {
		height = 768
	}
	needsCoords := strings.HasPrefix(action.Type, "mouse_") || action.Type == "click" ||
		action.Type == "touch_start" || action.Type == "touch_move" || action.Type == "tap"
	if needsCoords {
		if math.IsNaN(action.X) || math.IsInf(action.X, 0) || math.IsNaN(action.Y) || math.IsInf(action.Y, 0) ||
			action.X < 0 || action.X > float64(width) || action.Y < 0 || action.Y > float64(height) {
			return fmt.Errorf("coordinates must be inside the %d×%d viewport", width, height)
		}
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
	touchID := 0.0
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
	case "touch_start":
		return proto.InputDispatchTouchEvent{
			Type:        proto.InputDispatchTouchEventTypeTouchStart,
			TouchPoints: []*proto.InputTouchPoint{{X: action.X, Y: action.Y, ID: &touchID}},
		}.Call(page)
	case "touch_move":
		return proto.InputDispatchTouchEvent{
			Type:        proto.InputDispatchTouchEventTypeTouchMove,
			TouchPoints: []*proto.InputTouchPoint{{X: action.X, Y: action.Y, ID: &touchID}},
		}.Call(page)
	case "touch_end":
		return proto.InputDispatchTouchEvent{
			Type:        proto.InputDispatchTouchEventTypeTouchEnd,
			TouchPoints: []*proto.InputTouchPoint{},
		}.Call(page)
	case "tap":
		if err := (proto.InputDispatchTouchEvent{
			Type:        proto.InputDispatchTouchEventTypeTouchStart,
			TouchPoints: []*proto.InputTouchPoint{{X: action.X, Y: action.Y, ID: &touchID}},
		}).Call(page); err != nil {
			return err
		}
		return proto.InputDispatchTouchEvent{
			Type:        proto.InputDispatchTouchEventTypeTouchEnd,
			TouchPoints: []*proto.InputTouchPoint{},
		}.Call(page)
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
