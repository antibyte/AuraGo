package server

import (
	"database/sql"
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

type homepageAnnotationRequest struct {
	// SiteID is accepted for older clients; deployed targets are resolved
	// through PreviewURL, which may belong to any registered site.
	SiteID int64  `json:"site_id"`
	Target string `json:"target"`
	// PreviewURL is the URL the studio iframe shows for a deployed target. It
	// is only a lookup key: the capture uses the registered URL it matches.
	PreviewURL string `json:"preview_url"`
	Path       string `json:"path"`
	Viewport   struct {
		Width  int `json:"width"`
		Height int `json:"height"`
	} `json:"viewport"`
}

// handleHomepageAnnotationSnapshot captures the previewed page for Homepage
// Studio annotations. The client names the target, the previewed URL and a
// relative path; the server captures only a URL it has registered itself.
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
		base, allowed, code := resolveHomepageAnnotationBase(s, strings.TrimSpace(req.Target), req.PreviewURL)
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
// bypass the SSRF guard (local preview only) and an error code. Deployed
// targets resolve previewURL against the URLs the registry holds.
func resolveHomepageAnnotationBase(s *Server, target, previewURL string) (*url.URL, *url.URL, string) {
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
		if !enabled || s.HomepageRegistryDB == nil {
			return nil, nil, "preview_unavailable"
		}
		registered := homepageAnnotationRegisteredURL(s.HomepageRegistryDB, previewURL)
		if registered == nil {
			return nil, nil, "preview_unavailable"
		}
		return registered, nil, ""
	default:
		return nil, nil, "invalid_target"
	}
}

// parseHomepageAnnotationDeployURL accepts an absolute http(s) URL with a host
// and no credentials.
func parseHomepageAnnotationDeployURL(raw string) *url.URL {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || parsed.User != nil || parsed.Hostname() == "" {
		return nil
	}
	scheme := strings.ToLower(parsed.Scheme)
	if scheme != "http" && scheme != "https" {
		return nil
	}
	return parsed
}

// homepageAnnotationURLKey is the comparison form of a deployed URL: lowercase
// scheme and host, default port dropped, trailing slash trimmed, query and
// fragment ignored. It returns "" for URLs that are not usable.
func homepageAnnotationURLKey(raw string) string {
	parsed := parseHomepageAnnotationDeployURL(raw)
	if parsed == nil {
		return ""
	}
	scheme := strings.ToLower(parsed.Scheme)
	host := strings.ToLower(parsed.Hostname())
	port := parsed.Port()
	if (scheme == "http" && port == "80") || (scheme == "https" && port == "443") {
		port = ""
	}
	if port != "" {
		host = net.JoinHostPort(host, port)
	} else if strings.Contains(host, ":") {
		host = "[" + host + "]"
	}
	return scheme + "://" + host + strings.TrimSuffix(parsed.Path, "/")
}

// homepageAnnotationRegisteredURLs lists every deployed URL the registry
// knows about, across all managed sites. These are the URLs Homepage Studio
// can show in its preview, so they are the only ones a capture may target.
func homepageAnnotationRegisteredURLs(db *sql.DB) []string {
	sites, err := tools.ListHomepageManagedSites(db)
	if err != nil {
		return nil
	}
	var urls []string
	for _, site := range sites {
		urls = append(urls, site.LastDeployURL)
		if targets, err := tools.ListHomepageManagedDeployTargets(db, site.ID); err == nil {
			for _, t := range targets {
				urls = append(urls, t.URL)
			}
		}
		if deployments, err := tools.ListHomepageManagedDeployments(db, site.ID, 10); err == nil {
			for _, d := range deployments {
				urls = append(urls, d.URL)
			}
		}
		if observations, err := tools.ListHomepageManagedRemoteObservations(db, site.ID, 10); err == nil {
			for _, o := range observations {
				urls = append(urls, o.URL)
			}
		}
	}
	return urls
}

// homepageAnnotationRegisteredURL returns the registered URL that previewURL
// refers to, with query and fragment dropped, or nil when the registry holds
// no such URL. The client string itself is never used as a capture base.
func homepageAnnotationRegisteredURL(db *sql.DB, previewURL string) *url.URL {
	want := homepageAnnotationURLKey(previewURL)
	if want == "" {
		return nil
	}
	for _, raw := range homepageAnnotationRegisteredURLs(db) {
		if homepageAnnotationURLKey(raw) != want {
			continue
		}
		registered := parseHomepageAnnotationDeployURL(raw)
		registered.RawQuery, registered.Fragment = "", ""
		return registered
	}
	return nil
}

func homepageAnnotationFailure(err error) (int, string) {
	switch {
	case errors.Is(err, tools.ErrHomepageAnnotationBusy):
		return http.StatusConflict, "capture_busy"
	case errors.Is(err, tools.ErrHomepageAnnotationSandbox):
		return http.StatusServiceUnavailable, "browser_sandbox_blocked"
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
