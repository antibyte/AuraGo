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
	validationID := ""
	var scenarios []GameScenario
	if s.previewCheck != nil && s.previewCheck.JobID == previewJobID {
		validationID = s.previewCheck.ID
		scenarios = append(scenarios, s.previewCheck.Scenarios...)
	}
	s.tokens[token] = previewToken{Revision: project.CurrentRevision, ProjectID: projectID, JobID: previewJobID, ValidationID: validationID, ExpiresAt: expires}
	playToken := ""
	if project.Variant == "voxel" && project.CurrentRevision > 0 && previewJobID == "" && validationID == "" {
		playToken = randomID("play")
		s.tokens[playToken] = previewToken{Purpose: "play-state", Revision: project.CurrentRevision, ProjectID: projectID, ExpiresAt: time.Now().UTC().Add(24 * time.Hour)}
	}
	for candidate, grant := range s.tokens {
		if time.Now().After(grant.ExpiresAt) {
			delete(s.tokens, candidate)
		}
	}
	s.mu.Unlock()
	return PreviewGrant{Token: token, PlayToken: playToken, Revision: project.CurrentRevision, URL: "/api/game-maker/preview/" + token + "/index.html", ExpiresAt: expires, ValidationID: validationID, Scenarios: scenarios}, nil
}

// PreviewFile validates a token and returns one published project file. The
// caller is responsible for applying the preview CSP and cache headers.
func (s *Service) PreviewFile(token, rawPath string) ([]byte, string, error) {
	s.mu.RLock()
	grant, ok := s.tokens[strings.TrimSpace(token)]
	s.mu.RUnlock()
	if !ok || grant.Purpose != "" || time.Now().After(grant.ExpiresAt) {
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
	if strings.HasPrefix(rawPath, ".") || strings.Contains(rawPath, "/.") || strings.Contains(rawPath, `\.`) {
		return nil, "", ErrInvalidPath
	}
	path, _, err := secureJoin(root, rawPath, true)
	if err != nil {
		return nil, "", err
	}
	data, err := os.ReadFile(path)
	if project.Variant == "voxel" && grant.JobID == "" {
		data, err = s.publishedVoxelFile(nilContext{}, project.ID, grant.Revision, filepath.ToSlash(rawPath))
	}
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
(() => {
  const channel = new URLSearchParams(location.hash.slice(1)).get('gm-channel');
  const send = (type, message = '') => { if (channel) parent.postMessage({source:'aurago-game', channel, type, message:String(message).slice(0,2000)}, '*'); };
  addEventListener('error', event => send(event.message ? 'runtime_error' : 'resource_error', event.message || 'Failed resource: ' + (event.target?.src || event.target?.href || 'unknown')), true);
  addEventListener('unhandledrejection', event => send('runtime_error', event.reason));
  const original = console.error;
  console.error = (...args) => { original.apply(console,args); send('runtime_error',args.join(' ')); };
  if (document.readyState === 'loading') addEventListener('DOMContentLoaded', () => send('ready'), {once:true}); else send('ready');
})();
</script>`

func injectPreviewBoot(data []byte) []byte {
	if bytes.Contains(data, []byte(previewBootMarker)) {
		return data
	}
	html := string(data)
	lower := strings.ToLower(html)
	if idx := strings.Index(lower, "<head"); idx >= 0 {
		if end := strings.IndexByte(lower[idx:], '>'); end >= 0 {
			idx += end + 1
			return []byte(html[:idx] + previewBootScript + html[idx:])
		}
	}
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
