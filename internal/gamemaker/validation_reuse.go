package gamemaker

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"
)

// Include source, assets, generated output, runtime and the internal plan. A
// matching bundle alone does not prove that the accepted plan is unchanged.
func validationFingerprint(stage string) (string, error) {
	hash := sha256.New()
	err := filepath.WalkDir(stage, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.Type()&os.ModeSymlink != 0 {
			return ErrInvalidPath
		}
		if entry.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(stage, path)
		if err != nil {
			return err
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() {
			return ErrInvalidPath
		}
		fmt.Fprintf(hash, "%s\x00%d\x00", filepath.ToSlash(rel), info.Size())
		file, err := os.Open(path)
		if err != nil {
			return err
		}
		_, err = io.Copy(hash, file)
		closeErr := file.Close()
		if err != nil {
			return err
		}
		return closeErr
	})
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%x", hash.Sum(nil)), nil
}

func (s *Service) reusableValidation(ctx context.Context, jobID, scope string) (BuildResult, bool) {
	if ctx.Err() != nil || s.CheckJobMutation(ctx, jobID) != nil {
		return BuildResult{}, false
	}
	s.fileMu.Lock()
	defer s.fileMu.Unlock()
	s.buildMu.Lock()
	defer s.buildMu.Unlock()
	stage, err := s.JobDirectory(jobID)
	if err != nil {
		return BuildResult{}, false
	}
	job, err := s.GetJob(ctx, jobID)
	if err != nil {
		return BuildResult{}, false
	}
	fingerprint, err := validationFingerprint(stage)
	if err != nil {
		return BuildResult{}, false
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	last := s.lastValidation[jobID]
	if last == nil || !last.OK || last.TargetedChecks || last.validationScope != scope || last.fingerprint == "" || last.fingerprint != fingerprint || last.RuntimeStatus != "passed" {
		return BuildResult{}, false
	}
	check := last.check
	if check == nil || check != s.previewCheck || check.JobID != jobID || s.activeJobID != jobID || len(check.Diagnostics) > 0 || check.ReadyAt.IsZero() || time.Since(check.ReadyAt) < 3*time.Second {
		return BuildResult{}, false
	}
	grant, ok := s.tokens[check.BoundToken]
	if !ok || grant.ProjectID != job.ProjectID || grant.JobID != jobID || grant.ValidationID != check.ID || time.Now().After(grant.ExpiresAt) {
		return BuildResult{}, false
	}
	encoded, _ := json.Marshal(check.Scenarios)
	if last.scenarioFingerprint == "" || last.scenarioFingerprint != sourceHash(string(encoded)) {
		return BuildResult{}, false
	}
	if scope != "startup" && (last.GameplayStatus != "passed" || !check.GameplayReceived || len(check.Scenarios) == 0) {
		return BuildResult{}, false
	}
	return *last, true
}

// Explicit tool validation always runs afresh. Only the orchestrator can reuse
// the complete current result when the agent just ran the same required scope.
func (s *Service) validateForPublication(ctx context.Context, jobID, scope string) BuildResult {
	if result, ok := s.reusableValidation(ctx, jobID, scope); ok {
		return result
	}
	return s.ValidateJobScope(ctx, jobID, scope)
}

func (s *Service) RemainingRepairs(jobID string) int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return max(0, min(3, 4-s.validationFailures[jobID]))
}
