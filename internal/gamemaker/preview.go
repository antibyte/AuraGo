package gamemaker

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"mime"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func (s *Service) CreatePreviewGrant(projectID string) (PreviewGrant, error) {
	project, err := s.GetProject(nilContext{}, projectID)
	if err != nil {
		return PreviewGrant{}, err
	}
	s.mu.RLock()
	previewJobID := s.previewJobs[projectID]
	s.mu.RUnlock()
	if project.CurrentRevision <= 0 && previewJobID == "" {
		return PreviewGrant{}, fmt.Errorf("project has no playable revision")
	}
	var raw [32]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return PreviewGrant{}, fmt.Errorf("create preview token: %w", err)
	}
	token := hex.EncodeToString(raw[:])
	expires := time.Now().UTC().Add(2 * time.Minute)
	s.mu.Lock()
	s.tokens[token] = previewToken{ProjectID: projectID, JobID: previewJobID, ExpiresAt: expires}
	for candidate, grant := range s.tokens {
		if time.Now().After(grant.ExpiresAt) {
			delete(s.tokens, candidate)
		}
	}
	s.mu.Unlock()
	return PreviewGrant{Token: token, URL: "/api/game-maker/preview/" + token + "/index.html", ExpiresAt: expires}, nil
}

// PreviewFile validates a token and returns one published project file. The
// caller is responsible for applying the preview CSP and cache headers.
func (s *Service) PreviewFile(token, rawPath string) ([]byte, string, error) {
	s.mu.RLock()
	grant, ok := s.tokens[strings.TrimSpace(token)]
	s.mu.RUnlock()
	if !ok || time.Now().After(grant.ExpiresAt) {
		return nil, "", ErrInvalidToken
	}
	project, err := s.GetProject(nilContext{}, grant.ProjectID)
	if err != nil {
		return nil, "", err
	}
	root := filepath.Join(s.opts.WorkspacePath, filepath.FromSlash(project.ProjectKey))
	if grant.JobID != "" {
		s.mu.RLock()
		currentJobID := s.previewJobs[grant.ProjectID]
		s.mu.RUnlock()
		if currentJobID == grant.JobID {
			root = filepath.Join(s.stagingDir, grant.JobID)
		}
	}
	if rawPath == "" || strings.HasSuffix(rawPath, "/") {
		rawPath += "index.html"
	}
	path, _, err := secureJoin(root, rawPath, true)
	if err != nil {
		return nil, "", err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			if fallback, bundled, fallbackErr := bundledRuntimeFile(project.Dimension, rawPath); bundled {
				if fallbackErr != nil {
					return nil, "", fallbackErr
				}
				contentType := mime.TypeByExtension(strings.ToLower(filepath.Ext(rawPath)))
				if contentType == "" {
					contentType = "application/octet-stream"
				}
				return fallback, contentType, nil
			}
			return nil, "", ErrNotFound
		}
		return nil, "", fmt.Errorf("read game preview file: %w", err)
	}
	contentType := mime.TypeByExtension(strings.ToLower(filepath.Ext(path)))
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	if isPreviewHTML(rawPath, path, contentType) {
		data = injectPreviewBoot(data)
		if contentType == "application/octet-stream" || contentType == "" {
			contentType = "text/html; charset=utf-8"
		}
	}
	return data, contentType, nil
}

func isPreviewHTML(rawPath, absPath, contentType string) bool {
	name := strings.ToLower(filepath.Base(strings.TrimSpace(rawPath)))
	if name == "" {
		name = strings.ToLower(filepath.Base(absPath))
	}
	if name == "index.html" || strings.HasSuffix(name, ".html") {
		return true
	}
	return strings.Contains(strings.ToLower(contentType), "text/html")
}

// previewBootScript is injected into every HTML preview response so agent-rewritten
// index.html files still clear stale Loading pills and notify the parent studio.
// The parent validates source, channel, and event.source; "*" is required because
// the sandboxed iframe has an opaque origin.
const previewBootMarker = `data-aurago-preview-boot`

const previewBootScript = `<script ` + previewBootMarker + `>
(function () {
  if (window.__AURAGO_PREVIEW_BOOT__) return;
  window.__AURAGO_PREVIEW_BOOT__ = true;
  function isLoadingText(value) {
    return /^loading[.…]*(?:\s*preview)?[.…\s]*$/i.test(String(value || "").trim());
  }
  function hideStaleLoading() {
    var nodes = document.querySelectorAll("#hud, [data-loading], .loading, .gm-loading");
    for (var i = 0; i < nodes.length; i++) {
      var el = nodes[i];
      if (!el) continue;
      var text = String(el.textContent || "").trim();
      if (isLoadingText(text)) {
        el.setAttribute("data-aurago-hid-loading", "1");
        el.hidden = true;
        el.style.display = "none";
        continue;
      }
      // Restore HUD labels the boot previously hid once the game sets real text.
      if (text && el.getAttribute("data-aurago-hid-loading") === "1") {
        el.removeAttribute("data-aurago-hid-loading");
        el.hidden = false;
        el.style.display = "";
      }
    }
  }
  var channel = "";
  try {
    channel = new URLSearchParams(location.hash.replace(/^#/, "")).get("gm-channel") || "";
  } catch (_) {}
  var readySent = false;
  function notifyReady() {
    if (readySent || !channel || !document.querySelector("canvas")) return;
    readySent = true;
    try {
      parent.postMessage({ source: "aurago-game", type: "ready", channel: channel, canvas: true, boot: true }, "*");
    } catch (_) {}
  }
  function tick() {
    hideStaleLoading();
    notifyReady();
  }
  if (document.readyState === "loading") {
    document.addEventListener("DOMContentLoaded", tick, { once: true });
  } else {
    tick();
  }
  try {
    new MutationObserver(tick).observe(document.documentElement, { childList: true, subtree: true, characterData: true });
  } catch (_) {}
  setTimeout(tick, 250);
  setTimeout(tick, 1000);
  setTimeout(function () { hideStaleLoading(); notifyReady(); }, 4000);
})();
</script>`

func injectPreviewBoot(data []byte) []byte {
	if bytes.Contains(data, []byte(previewBootMarker)) {
		return data
	}
	html := string(data)
	lower := strings.ToLower(html)
	if idx := strings.LastIndex(lower, "</body>"); idx >= 0 {
		return []byte(html[:idx] + previewBootScript + html[idx:])
	}
	if idx := strings.LastIndex(lower, "</html>"); idx >= 0 {
		return []byte(html[:idx] + previewBootScript + html[idx:])
	}
	return append(data, []byte(previewBootScript)...)
}

// nilContext avoids retaining request lifetimes for short local DB reads.
type nilContext struct{}

func (nilContext) Deadline() (time.Time, bool) { return time.Time{}, false }
func (nilContext) Done() <-chan struct{}       { return nil }
func (nilContext) Err() error                  { return nil }
func (nilContext) Value(any) any               { return nil }
