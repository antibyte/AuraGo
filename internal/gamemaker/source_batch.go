package gamemaker

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"
)

type SourceReplacement struct {
	Path           string  `json:"path"`
	ExpectedSHA256 string  `json:"expected_sha256"`
	OldText        string  `json:"old_text"`
	NewText        *string `json:"new_text"`
}

type SourceBatchWrite struct {
	Written bool            `json:"written"`
	Files   []SourceVersion `json:"files"`
	Build   BuildResult     `json:"build"`
}

type SourceVersion struct {
	Path   string `json:"path"`
	SHA256 string `json:"sha256"`
}

// ReplaceJobFiles checks every precondition before changing any existing file.
// It publishes source under the file/build locks, rolls back IO failures, then
// builds once. Preview evidence is never produced for a half-applied edit.
func (s *Service) ReplaceJobFiles(ctx context.Context, jobID string, edits []SourceReplacement) (SourceBatchWrite, error) {
	s.fileMu.Lock()
	defer s.fileMu.Unlock()
	if len(edits) < 1 || len(edits) > 8 {
		return SourceBatchWrite{}, fmt.Errorf("replace_many requires 1–8 edits with distinct existing paths")
	}
	if err := s.CheckJobMutation(ctx, jobID); err != nil {
		return SourceBatchWrite{}, err
	}
	stage, err := s.JobDirectory(jobID)
	if err != nil {
		return SourceBatchWrite{}, err
	}
	type pending struct{ path, rel, before, after, temp string }
	files := make([]pending, 0, len(edits))
	seen := map[string]bool{}
	var growth int64
	for _, edit := range edits {
		path, rel, err := secureJoin(stage, edit.Path, false)
		if err != nil {
			return SourceBatchWrite{}, err
		}
		key := strings.ToLower(rel)
		if seen[key] {
			return SourceBatchWrite{}, fmt.Errorf("replace_many: repeated path %s; combine its changes into one unique block", rel)
		}
		seen[key] = true
		if edit.OldText == "" || edit.NewText == nil || edit.ExpectedSHA256 == "" {
			return SourceBatchWrite{}, fmt.Errorf("%s: require old_text, explicit new_text and expected_sha256", rel)
		}
		before, err := s.ReadJobFile(ctx, jobID, rel)
		if err != nil {
			return SourceBatchWrite{}, err
		}
		if sourceHash(before) != edit.ExpectedSHA256 {
			return SourceBatchWrite{}, fmt.Errorf("source_conflict: %s changed; read it again; no files were written", rel)
		}
		if n := strings.Count(before, edit.OldText); n != 1 {
			return SourceBatchWrite{}, fmt.Errorf("%s: old_text matches %d locations; no files were written", rel, n)
		}
		after := strings.Replace(before, edit.OldText, *edit.NewText, 1)
		if !utf8.ValidString(after) || strings.ContainsRune(after, 0) {
			return SourceBatchWrite{}, fmt.Errorf("%s: source must be UTF-8 text", rel)
		}
		if int64(len(after)) > s.opts.MaxFileBytes {
			return SourceBatchWrite{}, fmt.Errorf("%s: file exceeds configured limit", rel)
		}
		if err := validateBuilderSource(stage, rel, after); err != nil {
			return SourceBatchWrite{}, err
		}
		if err := s.validateScriptAssetImports(ctx, jobID, rel, after); err != nil {
			return SourceBatchWrite{}, err
		}
		growth += int64(len(after) - len(before))
		files = append(files, pending{path: path, rel: rel, before: before, after: after})
	}
	if err := validateTreeLimits(stage, s.opts.MaxFilesPerProject, s.opts.MaxProjectBytes-growth); err != nil {
		return SourceBatchWrite{}, err
	}
	defer func() {
		for _, f := range files {
			if f.temp != "" {
				_ = os.Remove(f.temp)
			}
		}
	}()
	for i := range files {
		f, err := os.CreateTemp(filepath.Dir(files[i].path), ".gm-batch-*")
		if err != nil {
			return SourceBatchWrite{}, fmt.Errorf("prepare source edit: %w", err)
		}
		files[i].temp = f.Name()
		_, writeErr := f.WriteString(files[i].after)
		syncErr := f.Sync()
		closeErr := f.Close()
		if err := errors.Join(writeErr, syncErr, closeErr); err != nil {
			return SourceBatchWrite{}, fmt.Errorf("prepare source edit: %w", err)
		}
	}
	commitErr := func() error {
		s.buildMu.Lock()
		defer s.buildMu.Unlock()
		if err := ctx.Err(); err != nil {
			return err
		}
		if err := s.CheckJobMutation(ctx, jobID); err != nil {
			return err
		}
		for _, f := range files {
			current, err := os.ReadFile(f.path)
			if err != nil {
				return err
			}
			if string(current) != f.before {
				return fmt.Errorf("source_conflict: %s changed before commit; no files were written", f.rel)
			}
		}
		for i, f := range files {
			if err := os.Rename(f.temp, f.path); err != nil {
				// Roll back successful renames before releasing the build lock.
				var rollbackErr error
				for _, previous := range files[:i] {
					rollbackErr = errors.Join(rollbackErr, os.WriteFile(previous.path, []byte(previous.before), 0o640))
				}
				if rollbackErr != nil {
					s.mu.Lock()
					s.previewCheck = nil
					s.mu.Unlock()
				}
				return fmt.Errorf("commit source edits: %w", errors.Join(err, rollbackErr))
			}
		}
		return nil
	}()
	if commitErr != nil {
		return SourceBatchWrite{}, commitErr
	}
	job, _ := s.GetJob(ctx, jobID)
	for _, f := range files {
		_, _ = s.emit(ctx, job.ProjectID, jobID, "file_changed", map[string]any{"path": f.rel})
	}
	result := SourceBatchWrite{Written: true, Build: s.BuildJob(ctx, jobID)}
	result.Build.Diagnostics = result.Build.Diagnostics[:min(12, len(result.Build.Diagnostics))]
	for _, f := range files {
		current, err := s.ReadJobFile(ctx, jobID, f.rel)
		if err != nil {
			return result, err
		}
		result.Files = append(result.Files, SourceVersion{Path: f.rel, SHA256: sourceHash(current)})
	}
	return result, nil
}
