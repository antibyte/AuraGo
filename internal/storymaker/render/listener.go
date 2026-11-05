package render

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"path"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

const (
	maxReportBytes = 8 << 20
	maxErrorBytes  = 16 << 10
)

// SnapshotFont is one catalog font face as the render page receives it.
type SnapshotFont struct {
	Name   string
	Family string
	Weight int
	// File is the catalog path ("/fonts/Geist-400.woff2"); the listener serves it below <base>/fonts/.
	File string
	Data []byte
	// The fields below describe a fallback face of glyphs.json. A catalog font leaves them empty.
	Fallback     bool
	UnicodeRange string
	WeightRange  string // FontFace descriptor, for example "300 700"
	MaxBytes     int64  // 0 means the page's cap of 8 MiB
}

// Snapshot is everything the render page may read, copied when the job starts so edits during a
// render change nothing.
type Snapshot struct {
	Story []byte
	// CatalogFiles holds catalog.json and every file it names, keyed by the path below
	// ui/img/storymaker/ ("catalog.json", "rig/human.json", "props/lock.json", ...).
	CatalogFiles map[string][]byte
	Fonts        []SnapshotFont
	CodeBundles  map[string][]byte
	HostBundle   []byte
	PlayerBundle []byte
}

// PageParams are the values the boot document carries.
type PageParams struct {
	Width, Height int
	FPS           int
	Transport     string
	FrameLimit    int
	// FrameDeadlineMS is the per-frame render deadline the page may use; a render is not real time,
	// so it is far above the 250 ms of the app preview. A page that does not know the field keeps its
	// own deadline.
	FrameDeadlineMS int
	// Token authorises the page's POSTs (32 hex characters). boot.json hands it out once, before any
	// scene worker exists, so a scene worker that knows the base still cannot forge a report.
	Token string
	// AudioInfo is served as boot.audio_info. nil serves {}.
	AudioInfo json.RawMessage
}

// ReportAnswer is the reply to POST /report.
type ReportAnswer struct {
	Start     bool   `json:"start"`
	FromFrame int    `json:"from_frame,omitempty"`
	Error     string `json:"error,omitempty"`
	Message   string `json:"message,omitempty"`
}

// FrameAnswer is the reply to POST /frame.
type FrameAnswer struct {
	OK      bool   `json:"ok,omitempty"`
	Done    bool   `json:"done,omitempty"`
	Restart bool   `json:"restart,omitempty"`
	Stop    bool   `json:"stop,omitempty"`
	Error   string `json:"error,omitempty"`
	Message string `json:"message,omitempty"`
}

// PageError is a failure the page reports with POST /error.
type PageError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	SceneID string `json:"scene_id,omitempty"`
}

// Hooks connect the listener to the render job.
type Hooks struct {
	// Report receives the compile report JSON.
	Report func(body []byte) ReportAnswer
	// Frame receives one frame; status is the HTTP status of the answer.
	Frame func(n int, contentType string, data []byte) (answer FrameAnswer, status int)
	// Error receives a page failure.
	Error func(e PageError)
}

// Listener is the per-job loopback HTTP server the headless browser talks to. It binds
// 127.0.0.1:0, speaks plain HTTP with its own timeouts, writes no access log and serves nothing
// outside <origin>/<prefix>/.
type Listener struct {
	prefix string
	origin string
	snap   *Snapshot
	params PageParams
	hooks  Hooks
	ln     net.Listener
	srv    *http.Server
	fonts  map[string]*SnapshotFont
	maxFrm int64

	// tokenServed flips on the first GET of boot.json.
	tokenServed atomic.Bool

	statsMu sync.Mutex
	stats   map[string]int64
}

// NewPrefix returns a fresh 128-bit render prefix (32 hex characters).
func NewPrefix() (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(b[:]), nil
}

// StartListener binds a loopback port and starts serving.
func StartListener(prefix string, snap *Snapshot, params PageParams, hooks Hooks) (*Listener, error) {
	if len(prefix) < 32 || strings.ContainsAny(prefix, "/?#%") {
		return nil, errors.New("storymaker: render prefix must be 32 or more safe characters")
	}
	if len(params.Token) < 32 {
		return nil, errors.New("storymaker: render token must have 32 or more characters")
	}
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, fmt.Errorf("storymaker: render listener: %w", err)
	}
	l := &Listener{
		prefix: prefix, origin: "http://" + ln.Addr().String(), snap: snap, params: params, hooks: hooks, ln: ln,
		fonts: map[string]*SnapshotFont{}, stats: map[string]int64{},
		maxFrm: MaxFrameBytes(IngestConfig{Width: params.Width, Height: params.Height, Transport: params.Transport}),
	}
	for i := range snap.Fonts {
		l.fonts[snap.Fonts[i].File] = &snap.Fonts[i]
	}
	l.srv = &http.Server{
		Handler:           http.HandlerFunc(l.serve),
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       2 * time.Minute,
		WriteTimeout:      2 * time.Minute,
		IdleTimeout:       2 * time.Minute,
		MaxHeaderBytes:    16 << 10,
		ErrorLog:          log.New(io.Discard, "", 0), // never log: error texts can carry request paths
	}
	go func() { _ = l.srv.Serve(ln) }()
	return l, nil
}

// Origin is http://127.0.0.1:<port>.
func (l *Listener) Origin() string { return l.origin }

// Base is the render base: origin plus the secret prefix, without a trailing slash.
func (l *Listener) Base() string { return l.origin + "/" + l.prefix }

// Close stops the listener and drops its connections.
func (l *Listener) Close() error { return l.srv.Close() }

// RenderCSP is the render page's Content-Security-Policy (spec "Rendering and export", step 3).
func RenderCSP(base string) string {
	return "sandbox allow-scripts; default-src 'none'; script-src " + base + "/js/ blob:; worker-src blob:; img-src blob: data:; font-src blob: data:; connect-src " + base + "/; form-action 'none'; base-uri 'none'; object-src 'none'; frame-ancestors 'none'"
}

// RenderPage is the render page markup (contract section 6).
func RenderPage(base string, width, height int) string {
	b := html.EscapeString(base)
	return `<!doctype html><html data-sm-mode="render" data-sm-base="` + b + `"><head><meta charset="utf-8"></head>` +
		`<body><canvas id="sm-stage" width="` + strconv.Itoa(width) + `" height="` + strconv.Itoa(height) + `"></canvas>` +
		`<script src="` + b + `/js/storymaker/host.bundle.js"></script></body></html>`
}

func (l *Listener) serve(w http.ResponseWriter, r *http.Request) {
	defer func() {
		if rec := recover(); rec != nil && rec != http.ErrAbortHandler {
			http.Error(w, "internal error", http.StatusInternalServerError)
		}
	}()
	// A DNS-rebinding page would reach this port under another Host header.
	if r.Host != strings.TrimPrefix(l.origin, "http://") {
		notFound(w)
		return
	}
	root := "/" + l.prefix + "/"
	if !strings.HasPrefix(r.URL.Path, root) {
		notFound(w)
		return
	}
	rest := strings.TrimPrefix(r.URL.Path, root)
	if rest != "" && path.Clean("/" + rest)[1:] != rest {
		notFound(w)
		return
	}
	l.count(r.Method, rest)
	h := w.Header()
	h.Set("Access-Control-Allow-Origin", "*")
	h.Set("Cache-Control", "no-store")
	h.Set("X-Content-Type-Options", "nosniff")
	h.Set("Referrer-Policy", "no-referrer")
	if r.Method == http.MethodOptions {
		h.Set("Access-Control-Allow-Methods", "GET, POST")
		h.Set("Access-Control-Allow-Headers", "Content-Type, X-SM-Frame")
		h.Set("Access-Control-Max-Age", "600")
		w.WriteHeader(http.StatusNoContent)
		return
	}
	switch {
	case rest == "":
		l.page(w, r)
	case rest == "boot.json":
		l.boot(w, r)
	case rest == "story.json":
		serveBytes(w, r, "application/json; charset=utf-8", l.snap.Story)
	case strings.HasPrefix(rest, "catalog/"):
		l.catalogFile(w, r, strings.TrimPrefix(rest, "catalog/"))
	case strings.HasPrefix(rest, "fonts/"):
		l.font(w, r, "/"+rest)
	case strings.HasPrefix(rest, "scenes/") && strings.HasSuffix(rest, ".js"):
		id := strings.TrimSuffix(strings.TrimPrefix(rest, "scenes/"), ".js")
		if data, ok := l.snap.CodeBundles[id]; ok {
			serveBytes(w, r, "text/javascript; charset=utf-8", data)
			return
		}
		notFound(w)
	case rest == "js/storymaker/host.bundle.js":
		serveBytes(w, r, "text/javascript; charset=utf-8", l.snap.HostBundle)
	case rest == "js/storymaker/player.bundle.js":
		serveBytes(w, r, "text/javascript; charset=utf-8", l.snap.PlayerBundle)
	case rest == "report":
		l.report(w, r)
	case rest == "frame":
		l.frame(w, r)
	case rest == "error":
		l.pageError(w, r)
	default:
		notFound(w)
	}
}

// count records one request under "METHOD route" (never the path itself).
func (l *Listener) count(method, rest string) {
	route := "static"
	switch rest {
	case "":
		route = "page"
	case "report", "frame", "error", "boot.json":
		route = rest
	}
	l.statsMu.Lock()
	l.stats[method+" "+route]++
	l.statsMu.Unlock()
}

// Counts returns how many requests each "METHOD route" received, for diagnostics.
func (l *Listener) Counts() map[string]int64 {
	l.statsMu.Lock()
	defer l.statsMu.Unlock()
	out := make(map[string]int64, len(l.stats))
	for k, v := range l.stats {
		out[k] = v
	}
	return out
}

func notFound(w http.ResponseWriter) {
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusNotFound)
}

func methodNotAllowed(w http.ResponseWriter, allow string) {
	w.Header().Set("Allow", allow)
	w.WriteHeader(http.StatusMethodNotAllowed)
}

func serveBytes(w http.ResponseWriter, r *http.Request, contentType string, data []byte) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		methodNotAllowed(w, "GET, HEAD, OPTIONS")
		return
	}
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Content-Length", strconv.Itoa(len(data)))
	w.WriteHeader(http.StatusOK)
	if r.Method == http.MethodGet {
		_, _ = w.Write(data)
	}
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	data, err := json.Marshal(v)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Content-Length", strconv.Itoa(len(data)))
	w.WriteHeader(status)
	_, _ = w.Write(data)
}

func (l *Listener) page(w http.ResponseWriter, r *http.Request) {
	h := w.Header()
	h.Set("Content-Security-Policy", RenderCSP(l.Base()))
	serveBytes(w, r, "text/html; charset=utf-8", []byte(RenderPage(l.Base(), l.params.Width, l.params.Height)))
}

type bootFont struct {
	Name         string `json:"name"`
	Family       string `json:"family"`
	Weight       int    `json:"weight"`
	URL          string `json:"url"`
	Fallback     bool   `json:"fallback,omitempty"`
	UnicodeRange string `json:"unicode_range,omitempty"`
	WeightRange  string `json:"weight_range,omitempty"`
	MaxBytes     int64  `json:"max_bytes,omitempty"`
}

type bootDoc struct {
	Mode            string            `json:"mode"`
	FPS             int               `json:"fps"`
	Width           int               `json:"width"`
	Height          int               `json:"height"`
	Transport       string            `json:"transport"`
	FrameLimit      int               `json:"frame_limit"`
	FrameDeadlineMS int               `json:"frame_deadline_ms,omitempty"`
	StoryURL        string            `json:"story_url"`
	CatalogURL      string            `json:"catalog_url"`
	AssetsBase      string            `json:"assets_base"`
	CodeBundles     map[string]string `json:"code_bundles"`
	Fonts           []bootFont        `json:"fonts"`
	AudioInfo       json.RawMessage   `json:"audio_info"`
	Token           string            `json:"token,omitempty"`
}

func (l *Listener) boot(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		methodNotAllowed(w, "GET, HEAD, OPTIONS")
		return
	}
	base := l.Base()
	audioInfo := l.params.AudioInfo
	if len(audioInfo) == 0 {
		audioInfo = json.RawMessage(`{}`)
	}
	doc := bootDoc{
		Mode: "render", FPS: l.params.FPS, Width: l.params.Width, Height: l.params.Height, Transport: l.params.Transport,
		FrameLimit: l.params.FrameLimit, FrameDeadlineMS: l.params.FrameDeadlineMS, StoryURL: base + "/story.json", CatalogURL: base + "/catalog/catalog.json",
		AssetsBase: base + "/catalog/", CodeBundles: map[string]string{}, Fonts: []bootFont{}, AudioInfo: audioInfo,
	}
	ids := make([]string, 0, len(l.snap.CodeBundles))
	for id := range l.snap.CodeBundles {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		doc.CodeBundles[id] = base + "/scenes/" + url.PathEscape(id) + ".js"
	}
	for _, f := range l.snap.Fonts {
		doc.Fonts = append(doc.Fonts, bootFont{Name: f.Name, Family: f.Family, Weight: f.Weight, URL: base + f.File, Fallback: f.Fallback, UnicodeRange: f.UnicodeRange, WeightRange: f.WeightRange, MaxBytes: f.MaxBytes})
	}
	// The first GET takes the token. A HEAD never does, and the page makes that GET before any worker exists.
	if r.Method == http.MethodGet && !l.tokenServed.Swap(true) {
		doc.Token = l.params.Token
	}
	writeJSON(w, http.StatusOK, doc)
}

func (l *Listener) catalogFile(w http.ResponseWriter, r *http.Request, rel string) {
	data, ok := l.snap.CatalogFiles[rel]
	if !ok {
		notFound(w)
		return
	}
	serveBytes(w, r, "application/json; charset=utf-8", data)
}

func (l *Listener) font(w http.ResponseWriter, r *http.Request, file string) {
	f, ok := l.fonts[file]
	if !ok {
		notFound(w)
		return
	}
	serveBytes(w, r, fontContentType(file), f.Data)
}

// fontContentType picks the type by extension. The Noto fallback faces are TTF and OTF.
func fontContentType(file string) string {
	switch strings.ToLower(path.Ext(file)) {
	case ".ttf":
		return "font/ttf"
	case ".otf":
		return "font/otf"
	}
	return "font/woff2"
}

// badTokenAnswer is the body of a 403. A wrong or missing token calls no hook and consumes nothing.
var badTokenAnswer = map[string]string{"error": "bad_token"}

// withoutSecrets removes the render prefix and the report token from a text the page wrote, before the text reaches the
// job record. A worker's error text can name a URL under the base, and the prefix in that URL is the read credential for
// the story and the catalog. Call it before clip, because a clip could leave a part of a secret. An empty secret changes
// nothing.
func withoutSecrets(text, prefix, token string) string {
	return strings.ReplaceAll(strings.ReplaceAll(text, prefix, ""), token, "")
}

// tokenOK compares the t query parameter with the job's token in constant time.
func (l *Listener) tokenOK(r *http.Request) bool {
	return subtle.ConstantTimeCompare([]byte(r.URL.Query().Get("t")), []byte(l.params.Token)) == 1
}

func (l *Listener) report(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w, "POST, OPTIONS")
		return
	}
	if !l.tokenOK(r) {
		writeJSON(w, http.StatusForbidden, badTokenAnswer)
		return
	}
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, maxReportBytes))
	if err != nil {
		writeJSON(w, http.StatusRequestEntityTooLarge, ReportAnswer{Error: "report_too_large", Message: "The compile report is larger than 8 MiB."})
		return
	}
	if l.hooks.Report == nil {
		notFound(w)
		return
	}
	writeJSON(w, http.StatusOK, l.hooks.Report(body))
}

func (l *Listener) frame(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w, "POST, OPTIONS")
		return
	}
	if !l.tokenOK(r) {
		writeJSON(w, http.StatusForbidden, FrameAnswer{Stop: true, Error: "bad_token"})
		return
	}
	// The frame number is the X-SM-Frame header or, for a CORS-simple request that needs no preflight,
	// the n query parameter.
	number := r.Header.Get("X-SM-Frame")
	if number == "" {
		number = r.URL.Query().Get("n")
	}
	n, err := strconv.Atoi(number)
	if err != nil || n < 0 {
		writeJSON(w, http.StatusBadRequest, FrameAnswer{Stop: true, Error: "frame_header", Message: "A frame needs its number in X-SM-Frame or ?n=."})
		l.fail("frame_header", "A frame arrived without a valid frame number.")
		return
	}
	if l.hooks.Frame == nil {
		notFound(w)
		return
	}
	data, err := io.ReadAll(http.MaxBytesReader(w, r.Body, l.maxFrm))
	if err != nil {
		var tooBig *http.MaxBytesError
		if errors.As(err, &tooBig) {
			writeJSON(w, http.StatusRequestEntityTooLarge, FrameAnswer{Stop: true, Error: "frame_too_large", Message: fmt.Sprintf("A frame is larger than %d bytes.", l.maxFrm)})
			l.fail("frame_too_large", fmt.Sprintf("Frame %d is larger than %d bytes.", n, l.maxFrm))
			return
		}
		writeJSON(w, http.StatusBadRequest, FrameAnswer{Stop: true, Error: "frame_read", Message: "The frame could not be read."})
		l.fail("frame_read", "A frame could not be read.")
		return
	}
	answer, status := l.hooks.Frame(n, r.Header.Get("Content-Type"), data)
	writeJSON(w, status, answer)
}

// fail reports an ingest violation that the frame hook never saw.
func (l *Listener) fail(code, message string) {
	if l.hooks.Error != nil {
		l.hooks.Error(PageError{Code: code, Message: message})
	}
}

func (l *Listener) pageError(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w, "POST, OPTIONS")
		return
	}
	if !l.tokenOK(r) {
		writeJSON(w, http.StatusForbidden, badTokenAnswer)
		return
	}
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, maxErrorBytes))
	var e PageError
	if err != nil || json.Unmarshal(body, &e) != nil || e.Code == "" {
		e = PageError{Code: "render_failed", Message: "The page reported an unreadable error."}
	}
	e.Code, e.Message, e.SceneID = clip(e.Code, 40), clip(withoutSecrets(e.Message, l.prefix, l.params.Token), 600), clip(withoutSecrets(e.SceneID, l.prefix, l.params.Token), 60)
	if l.hooks.Error != nil {
		l.hooks.Error(e)
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}
