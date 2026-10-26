package server

import (
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
	"unicode"

	"aurago/internal/tools"
)

// homepageAnnotationCapture is the capture engine; tests replace it.
var homepageAnnotationCapture = tools.CaptureHomepageAnnotationSnapshot

// homepageAnnotationDial probes the local preview before a capture.
var homepageAnnotationDial = net.DialTimeout

var homepageAnnotationWidths = map[int]bool{1280: true, 768: true, 390: true}

// homepageAnnotationProviders mirrors the aliases of externalTargetURL in
// ui/js/desktop/apps/homepage-studio-sites.js.
var homepageAnnotationProviders = map[string][]string{
	"vercel":   {"vercel"},
	"netlify":  {"netlify"},
	"here_now": {"here_now"},
	"remote":   {"remote", "sftp", "scp", "ssh"},
}

type homepageAnnotationRequest struct {
	SiteID   int64  `json:"site_id"`
	Target   string `json:"target"`
	Path     string `json:"path"`
	Viewport struct {
		Width  int `json:"width"`
		Height int `json:"height"`
	} `json:"viewport"`
}

// handleHomepageAnnotationSnapshot captures the previewed page for Homepage
// Studio annotations. The client names the target and a relative path; the
// server resolves the URL itself.
func handleHomepageAnnotationSnapshot(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			writeHomepageAnnotationError(w, http.StatusMethodNotAllowed, "method_not_allowed", "Method not allowed")
			return
		}
		var req homepageAnnotationRequest
		if err := decodeDesktopJSON(w, r, &req, 16<<10); err != nil {
			writeHomepageAnnotationError(w, http.StatusBadRequest, "invalid_request", "Invalid JSON")
			return
		}
		pagePath, err := normalizeHomepageAnnotationPath(req.Path)
		if err != nil {
			writeHomepageAnnotationError(w, http.StatusBadRequest, "invalid_path", err.Error())
			return
		}
		if !homepageAnnotationWidths[req.Viewport.Width] || req.Viewport.Height < 400 || req.Viewport.Height > 1600 {
			writeHomepageAnnotationError(w, http.StatusBadRequest, "invalid_viewport", "Viewport is not supported")
			return
		}
		base, allowed, code := resolveHomepageAnnotationBase(s, strings.TrimSpace(req.Target), req.SiteID)
		if code == "invalid_target" {
			writeHomepageAnnotationError(w, http.StatusBadRequest, code, "Target is not supported")
			return
		}
		if code != "" {
			writeHomepageAnnotationError(w, http.StatusNotFound, code, "No preview is reachable for this target")
			return
		}
		snapshot, err := homepageAnnotationCapture(r.Context(), tools.HomepageAnnotationCaptureOptions{
			URL:           homepageAnnotationURL(base, pagePath),
			AllowedOrigin: allowed,
			Width:         req.Viewport.Width,
			Height:        req.Viewport.Height,
		})
		if err != nil {
			status, failure := homepageAnnotationFailure(err)
			if s.Logger != nil {
				s.Logger.Warn("[Homepage] Annotation snapshot failed", "code", failure, "error", err)
			}
			writeHomepageAnnotationError(w, status, failure, "The page could not be captured")
			return
		}
		snapshot.Path = pagePath.String()
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "no-store")
		_ = json.NewEncoder(w).Encode(snapshot)
	}
}

// normalizeHomepageAnnotationPath accepts only a relative URL path with an
// optional query; the fragment is dropped.
func normalizeHomepageAnnotationPath(raw string) (*url.URL, error) {
	value := strings.TrimSpace(raw)
	if value == "" {
		value = "/"
	}
	if len(value) > 512 {
		return nil, fmt.Errorf("path is too long")
	}
	if !strings.HasPrefix(value, "/") || strings.HasPrefix(value, "//") || strings.Contains(value, `\`) {
		return nil, fmt.Errorf("path must start with a single /")
	}
	for _, r := range value {
		if unicode.IsControl(r) {
			return nil, fmt.Errorf("path contains control characters")
		}
	}
	parsed, err := url.Parse(value)
	if err != nil || parsed.Scheme != "" || parsed.Host != "" || parsed.User != nil {
		return nil, fmt.Errorf("path is not a relative URL path")
	}
	parsed.Fragment = ""
	return parsed, nil
}

func homepageAnnotationURL(base, pagePath *url.URL) *url.URL {
	target := *base
	target.Path = strings.TrimSuffix(base.Path, "/") + pagePath.Path
	target.RawPath = ""
	target.RawQuery = pagePath.RawQuery
	target.Fragment = ""
	return &target
}

// resolveHomepageAnnotationBase returns the page base URL, the origin that may
// bypass the SSRF guard (local preview only) and an error code.
func resolveHomepageAnnotationBase(s *Server, target string, siteID int64) (*url.URL, *url.URL, string) {
	s.CfgMu.RLock()
	enabled := s.Cfg != nil && s.Cfg.Homepage.Enabled
	port := 0
	if s.Cfg != nil {
		port = s.Cfg.Homepage.WebServerPort
	}
	s.CfgMu.RUnlock()

	switch target {
	case "", "local":
		if !enabled {
			return nil, nil, "preview_unavailable"
		}
		if port <= 0 {
			port = 8080
		}
		addr := net.JoinHostPort("127.0.0.1", strconv.Itoa(port))
		conn, err := homepageAnnotationDial("tcp", addr, time.Second)
		if err != nil {
			return nil, nil, "preview_unavailable"
		}
		_ = conn.Close()
		return &url.URL{Scheme: "http", Host: addr, Path: "/"}, &url.URL{Scheme: "http", Host: addr}, ""
	case "vercel", "netlify", "here_now", "remote":
		if !enabled || siteID <= 0 || s.HomepageRegistryDB == nil {
			return nil, nil, "preview_unavailable"
		}
		site, err := tools.GetHomepageManagedSite(s.HomepageRegistryDB, siteID)
		if err != nil {
			return nil, nil, "preview_unavailable"
		}
		raw := homepageAnnotationDeployURL(target, site)
		parsed, err := url.Parse(raw)
		if raw == "" || err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" {
			return nil, nil, "preview_unavailable"
		}
		parsed.RawQuery, parsed.Fragment = "", ""
		return parsed, nil, ""
	default:
		return nil, nil, "invalid_target"
	}
}

// homepageAnnotationDeployURL picks the provider's own target first, then a
// matching deployment, then any non-local target, then the last deploy URL.
func homepageAnnotationDeployURL(target string, site tools.HomepageManagedSite) string {
	matches := func(provider string) bool {
		for _, name := range homepageAnnotationProviders[target] {
			if strings.EqualFold(strings.TrimSpace(provider), name) {
				return true
			}
		}
		return false
	}
	for _, t := range site.DeployTargets {
		if matches(t.Provider) && strings.TrimSpace(t.URL) != "" {
			return strings.TrimSpace(t.URL)
		}
	}
	for _, d := range site.Deployments {
		if matches(d.Provider) && strings.TrimSpace(d.URL) != "" {
			return strings.TrimSpace(d.URL)
		}
	}
	for _, t := range site.DeployTargets {
		if !strings.EqualFold(strings.TrimSpace(t.Provider), "local") && strings.TrimSpace(t.URL) != "" {
			return strings.TrimSpace(t.URL)
		}
	}
	return strings.TrimSpace(site.LastDeployURL)
}

func homepageAnnotationFailure(err error) (int, string) {
	switch {
	case errors.Is(err, tools.ErrHomepageAnnotationBusy):
		return http.StatusConflict, "capture_busy"
	case errors.Is(err, tools.ErrHomepageAnnotationBrowser):
		return http.StatusServiceUnavailable, "browser_unavailable"
	case errors.Is(err, tools.ErrHomepageAnnotationTimeout):
		return http.StatusGatewayTimeout, "capture_timeout"
	case errors.Is(err, tools.ErrHomepageAnnotationBlocked):
		return http.StatusUnprocessableEntity, "capture_blocked"
	default:
		return http.StatusBadGateway, "capture_failed"
	}
}

// writeHomepageAnnotationError writes {"error","code"} so the studio can
// localize the message.
func writeHomepageAnnotationError(w http.ResponseWriter, status int, code, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": message, "code": code})
}
