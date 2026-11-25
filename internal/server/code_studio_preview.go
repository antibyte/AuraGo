package server

import (
	"bytes"
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"io/fs"
	"net/http"
	pathpkg "path"
	"strconv"
	"strings"
	"time"
)

//go:embed code_studio_preview_live.js
var codeStudioPreviewLiveScript []byte

const (
	codeStudioPreviewMaxBytes    = int64(25 << 20)
	codeStudioPreviewPollTimeout = 25 * time.Second
	codeStudioPreviewInternal    = "__aurago/"
	// The iframe carries the same flags. No allow-same-origin: the page gets an opaque origin and can never read
	// AuraGo cookies, storage or API responses, even when it is opened top-level in its own tab.
	codeStudioPreviewCSP = "sandbox allow-scripts allow-forms allow-modals allow-popups allow-downloads; frame-ancestors 'self'"
)

type codeStudioPreviewHandler struct {
	studio   codeStudioHandlers
	versions *codeStudioPreviewVersionRegistry
	// source resolves the workspace a token names; tests replace it with an in-memory fake.
	source      func(ctx context.Context, workspace string) (codeStudioPreviewSource, error)
	now         func() time.Time
	pollTimeout time.Duration
}

func newCodeStudioPreviewHandler(studio codeStudioHandlers) *codeStudioPreviewHandler {
	return &codeStudioPreviewHandler{
		studio:      studio,
		versions:    codeStudioPreviewVersionsFor(studio.server),
		source:      studio.previewSource,
		now:         time.Now,
		pollTimeout: codeStudioPreviewPollTimeout,
	}
}

func setCodeStudioPreviewHeaders(header http.Header) {
	header.Set("Content-Security-Policy", codeStudioPreviewCSP)
	// securityHeadersMiddleware sends X-Frame-Options: DENY for every path; frame-ancestors 'self' replaces it.
	header.Del("X-Frame-Options")
	header.Set("X-Content-Type-Options", "nosniff")
	header.Set("Referrer-Policy", "no-referrer")
	header.Set("Cache-Control", "no-store")
	header.Del("Pragma")
	// The sandboxed page fetches with Origin: null (live.js, module scripts, fonts). The token in the path is the
	// capability and no credentials are sent, so a wildcard is safe.
	header.Set("Access-Control-Allow-Origin", "*")
	header.Set("Cross-Origin-Resource-Policy", "cross-origin")
}

func (p *codeStudioPreviewHandler) handleServe(w http.ResponseWriter, r *http.Request) {
	setCodeStudioPreviewHeaders(w.Header())
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.Header().Set("Allow", "GET, HEAD")
		writeCodeStudioPreviewPage(w, r, http.StatusMethodNotAllowed, "Method not allowed", "The preview is read-only.")
		return
	}
	token, rel, slash := splitCodeStudioPreviewPath(r.URL.Path)
	key, err := p.studio.server.codeStudioPreviewSigningKey()
	if err != nil {
		writeCodeStudioPreviewPage(w, r, http.StatusServiceUnavailable, "Preview unavailable", "The preview key is not available.")
		return
	}
	claims, ok := parseCodeStudioPreviewToken(key, token, p.now())
	if !ok {
		writeCodeStudioPreviewPage(w, r, http.StatusUnauthorized, "Preview link expired", "This preview link has expired. Open the preview again from Code Studio.")
		return
	}
	if !slash {
		codeStudioPreviewRedirect(w, r, r.URL.EscapedPath()+"/")
		return
	}
	base := codeStudioPreviewPrefix + token + "/"
	switch rel {
	case codeStudioPreviewInternal + "live.js":
		w.Header().Set("Content-Type", "text/javascript; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		if r.Method == http.MethodGet {
			_, _ = w.Write(codeStudioPreviewLiveScript)
		}
		return
	case codeStudioPreviewInternal + "version":
		p.serveVersion(w, r, claims.Workspace)
		return
	}
	if strings.HasPrefix(rel, codeStudioPreviewInternal) {
		writeCodeStudioPreviewPage(w, r, http.StatusNotFound, "Not found", rel)
		return
	}
	directory := rel == "" || strings.HasSuffix(rel, "/")
	vpath, err := codeStudioPreviewVirtualPath(rel)
	if err != nil {
		writeCodeStudioPreviewPage(w, r, http.StatusBadRequest, "Invalid path", err.Error())
		return
	}
	if directory {
		vpath = pathpkg.Join(vpath, "index.html")
	}
	source, err := p.source(r.Context(), claims.Workspace)
	if err != nil {
		if errors.Is(err, errCodeStudioPreviewWorkspaceNotFound) {
			writeCodeStudioPreviewPage(w, r, http.StatusNotFound, "Workspace not found", "This workspace is no longer open in Code Studio.")
		} else {
			writeCodeStudioPreviewPage(w, r, http.StatusServiceUnavailable, "Preview unavailable", err.Error())
		}
		return
	}
	probe, err := source.Probe(r.Context(), vpath)
	if err != nil && !directory {
		// A folder without its trailing slash: redirect so relative links inside index.html resolve in the folder.
		if _, indexErr := source.Probe(r.Context(), pathpkg.Join(vpath, "index.html")); indexErr == nil {
			codeStudioPreviewRedirect(w, r, r.URL.EscapedPath()+"/")
			return
		}
	}
	display := strings.TrimPrefix(vpath, codeStudioWorkspaceRoot+"/")
	if err != nil {
		if codeStudioPreviewMissing(err) {
			writeCodeStudioPreviewPage(w, r, http.StatusNotFound, "File not found", display+" does not exist in this workspace.")
		} else {
			writeCodeStudioPreviewPage(w, r, http.StatusBadGateway, "Preview unavailable", err.Error())
		}
		return
	}
	if probe.Size > codeStudioPreviewMaxBytes {
		writeCodeStudioPreviewPage(w, r, http.StatusRequestEntityTooLarge, "File too large", display+" is larger than 25 MB.")
		return
	}
	contentType := codeStudioPreviewContentType(probe.Path)
	if r.Method == http.MethodHead {
		if contentType == "" {
			contentType = "application/octet-stream"
		}
		w.Header().Set("Content-Type", contentType)
		w.WriteHeader(http.StatusOK)
		return
	}
	// The version is taken before the read: a write in between makes live.js reload once more, never miss it.
	version := p.versions.Current(claims.Workspace)
	content, err := source.Read(r.Context(), probe.Path, codeStudioPreviewMaxBytes)
	if err != nil {
		writeCodeStudioPreviewPage(w, r, http.StatusBadGateway, "Preview unavailable", err.Error())
		return
	}
	if contentType == "" {
		contentType = http.DetectContentType(content)
	}
	content = injectCodeStudioPreviewLiveScript(content, contentType, base, version)
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Content-Length", strconv.Itoa(len(content)))
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(content)
}

func (p *codeStudioPreviewHandler) serveVersion(w http.ResponseWriter, r *http.Request, workspace string) {
	w.Header().Set("Content-Type", "application/json")
	if r.Method == http.MethodHead {
		w.WriteHeader(http.StatusOK)
		return
	}
	since, _ := strconv.ParseUint(r.URL.Query().Get("since"), 10, 64)
	version, paths, full := p.versions.Wait(r.Context(), workspace, since, p.pollTimeout)
	if paths == nil {
		paths = []string{}
	}
	_ = json.NewEncoder(w).Encode(map[string]interface{}{"version": version, "paths": paths, "full": full})
}

func codeStudioPreviewMissing(err error) bool {
	var rejected codeStudioProbeRejection
	return errors.As(err, &rejected) || errors.Is(err, fs.ErrNotExist)
}

func codeStudioPreviewRedirect(w http.ResponseWriter, r *http.Request, target string) {
	if r.URL.RawQuery != "" {
		target += "?" + r.URL.RawQuery
	}
	w.Header().Set("Location", target)
	w.WriteHeader(http.StatusTemporaryRedirect)
}

// The page lives under the preview CSP sandbox too; inline styles are allowed there, scripts are not needed.
const codeStudioPreviewPageTemplate = `<!doctype html><html lang="en"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>%s</title><style>:root{color-scheme:light dark;font-family:system-ui,sans-serif}body{margin:0;min-height:100vh;display:grid;place-items:center;background:Canvas;color:CanvasText}main{max-width:28rem;padding:2rem;text-align:center}h1{font-size:1.15rem;margin:0 0 .5rem}p{margin:0;opacity:.75;line-height:1.5;overflow-wrap:anywhere}</style></head><body><main><h1>%s</h1><p>%s</p></main></body></html>`

func writeCodeStudioPreviewPage(w http.ResponseWriter, r *http.Request, status int, title, detail string) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	if r.Method == http.MethodHead {
		return
	}
	escapedTitle := html.EscapeString(title)
	_, _ = fmt.Fprintf(w, codeStudioPreviewPageTemplate, escapedTitle, escapedTitle, html.EscapeString(detail))
}

// A fixed table instead of mime.TypeByExtension: the Windows registry maps .js to text/plain on some machines,
// which nosniff would then refuse to execute.
var codeStudioPreviewTypes = map[string]string{
	".html": "text/html; charset=utf-8", ".htm": "text/html; charset=utf-8",
	".css": "text/css; charset=utf-8", ".js": "text/javascript; charset=utf-8", ".mjs": "text/javascript; charset=utf-8",
	".json": "application/json", ".map": "application/json", ".webmanifest": "application/manifest+json",
	".svg": "image/svg+xml", ".png": "image/png", ".jpg": "image/jpeg", ".jpeg": "image/jpeg", ".gif": "image/gif",
	".webp": "image/webp", ".avif": "image/avif", ".ico": "image/x-icon", ".bmp": "image/bmp",
	".woff": "font/woff", ".woff2": "font/woff2", ".ttf": "font/ttf", ".otf": "font/otf",
	".wasm": "application/wasm", ".mp3": "audio/mpeg", ".wav": "audio/wav", ".ogg": "audio/ogg",
	".mp4": "video/mp4", ".webm": "video/webm", ".txt": "text/plain; charset=utf-8", ".md": "text/plain; charset=utf-8",
	".csv": "text/csv; charset=utf-8", ".xml": "application/xml", ".pdf": "application/pdf",
}

func codeStudioPreviewContentType(vpath string) string {
	return codeStudioPreviewTypes[strings.ToLower(pathpkg.Ext(vpath))]
}

// injectCodeStudioPreviewLiveScript adds live.js to HTML (right after <head>, so it also sees errors of the page's
// own scripts; else before </body>; else at the end) and to SVG documents (before </svg>).
func injectCodeStudioPreviewLiveScript(content []byte, contentType, base string, version uint64) []byte {
	src := base + codeStudioPreviewInternal + "live.js"
	versionText := strconv.FormatUint(version, 10)
	switch {
	case strings.HasPrefix(contentType, "text/html"):
		tag := []byte(`<script src="` + src + `" data-aurago-version="` + versionText + `"></script>`)
		if at := codeStudioPreviewHeadEnd(content); at >= 0 {
			return codeStudioPreviewSplice(content, at, tag)
		}
		if at := codeStudioPreviewLastIndexFold(content, "</body>"); at >= 0 {
			return codeStudioPreviewSplice(content, at, tag)
		}
		return append(append([]byte{}, content...), tag...)
	case strings.HasPrefix(contentType, "image/svg+xml"):
		tag := []byte(`<script xmlns="http://www.w3.org/2000/svg" href="` + src + `" data-aurago-version="` + versionText + `"></script>`)
		if at := codeStudioPreviewLastIndexFold(content, "</svg>"); at >= 0 {
			return codeStudioPreviewSplice(content, at, tag)
		}
	}
	return content
}

// codeStudioPreviewHeadEnd returns the offset just after the first <head> start tag, or -1. "<header" does not count.
func codeStudioPreviewHeadEnd(content []byte) int {
	for from := 0; from < len(content); {
		at := codeStudioPreviewIndexFold(content[from:], "<head")
		if at < 0 {
			return -1
		}
		next := from + at + len("<head")
		if next < len(content) {
			switch content[next] {
			case '>', ' ', '\t', '\n', '\r', '/':
				if end := bytes.IndexByte(content[next:], '>'); end >= 0 {
					return next + end + 1
				}
				return -1
			}
		}
		from = next
	}
	return -1
}

func codeStudioPreviewEqualFold(window []byte, needle string) bool {
	for i := 0; i < len(needle); i++ {
		c := window[i]
		if 'A' <= c && c <= 'Z' {
			c += 'a' - 'A'
		}
		if c != needle[i] {
			return false
		}
	}
	return true
}

// ASCII-only folding keeps byte offsets stable (bytes.ToLower may change the length of non-ASCII text).
func codeStudioPreviewIndexFold(haystack []byte, needle string) int {
	for i := 0; i+len(needle) <= len(haystack); i++ {
		if codeStudioPreviewEqualFold(haystack[i:i+len(needle)], needle) {
			return i
		}
	}
	return -1
}

func codeStudioPreviewLastIndexFold(haystack []byte, needle string) int {
	for i := len(haystack) - len(needle); i >= 0; i-- {
		if codeStudioPreviewEqualFold(haystack[i:i+len(needle)], needle) {
			return i
		}
	}
	return -1
}

func codeStudioPreviewSplice(content []byte, at int, insert []byte) []byte {
	out := make([]byte, 0, len(content)+len(insert))
	out = append(out, content[:at]...)
	out = append(out, insert...)
	return append(out, content[at:]...)
}

func registerCodeStudioPreviewRoutes(mux *http.ServeMux, studio codeStudioHandlers) {
	preview := newCodeStudioPreviewHandler(studio)
	mux.HandleFunc("/api/code-studio/preview", preview.handleIssue)
	mux.HandleFunc(codeStudioPreviewPrefix, preview.handleServe)
}

// handleIssue hands out the workspace-scoped preview token. It is a read: a read-only desktop still previews, so
// the operation is desktopRead even though the method is POST. The workspace is resolved through backendFor, which
// applies the remote-caller gate for SSH workspaces.
func (p *codeStudioPreviewHandler) handleIssue(w http.ResponseWriter, r *http.Request) {
	if !requireDesktopOperation(p.studio.server, w, r, desktopScopeRead, desktopRead) {
		return
	}
	if r.Method != http.MethodPost {
		jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var body struct {
		Path string `json:"path"`
	}
	if err := decodeJSONRequest(w, r, &body, p.studio.server); err != nil {
		jsonError(w, "Invalid JSON", jsonDecodeStatus(err))
		return
	}
	vpath, err := sanitizeCodeStudioPath(body.Path)
	if err != nil {
		jsonError(w, err.Error(), http.StatusBadRequest)
		return
	}
	workspace, err := p.studio.previewWorkspaceID(r)
	if err != nil {
		writeCodeStudioBackendError(w, err)
		return
	}
	key, err := p.studio.server.codeStudioPreviewSigningKey()
	if err != nil {
		jsonError(w, err.Error(), http.StatusServiceUnavailable)
		return
	}
	token, expires, err := issueCodeStudioPreviewToken(key, workspace, p.now())
	if err != nil {
		jsonError(w, err.Error(), http.StatusBadRequest)
		return
	}
	base := codeStudioPreviewPrefix + token + "/"
	rel := strings.TrimPrefix(strings.TrimPrefix(vpath, codeStudioWorkspaceRoot), "/")
	writeJSON(w, map[string]interface{}{
		"status":     "ok",
		"url":        base + codeStudioPreviewEscapePath(rel),
		"base":       base,
		"token":      token,
		"path":       vpath,
		"workspace":  workspace,
		"expires_at": expires.UTC().Format(time.RFC3339),
		"version":    p.versions.Current(workspace),
	})
}
