package gamemaker

import (
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
			return nil, "", ErrNotFound
		}
		return nil, "", fmt.Errorf("read game preview file: %w", err)
	}
	contentType := mime.TypeByExtension(strings.ToLower(filepath.Ext(path)))
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	return data, contentType, nil
}

// nilContext avoids retaining request lifetimes for short local DB reads.
type nilContext struct{}

func (nilContext) Deadline() (time.Time, bool) { return time.Time{}, false }
func (nilContext) Done() <-chan struct{}       { return nil }
func (nilContext) Err() error                  { return nil }
func (nilContext) Value(any) any               { return nil }
