package cydcreator

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"aurago/internal/fileutil"
)

// The working-copy tools apply the rules of the live project's layout and
// file API (PutLayout, ReadFile, WriteFile, DeleteFile) to the job's working
// copy: os.Root access, IsEditablePath, exact-case app paths, the size and
// count limits, the asset entry rule and the same typed errors.

// LayoutUpdate is the cyd_creator_update_layout answer.
type LayoutUpdate struct {
	Results []OpResult `json:"results"`
	Issues  []Issue    `json:"issues"`
}

// UpdateLayoutForJob applies validated layout ops to the working copy. The
// project itself changes only when the job publishes; ApplyOps is atomic and
// refuses an op that introduces a new validation error (a message box
// without buttons among them). The asset entries follow PutLayout's rule
// (checkLayoutAssets).
func (s *Service) UpdateLayoutForJob(ctx context.Context, jobID string, ops []LayoutOp) (LayoutUpdate, error) {
	run, err := s.jobTool(jobID, ToolUpdateLayout)
	if err != nil {
		return LayoutUpdate{}, err
	}
	if len(ops) == 0 {
		return LayoutUpdate{}, NewError(CodeBadRequest, "send at least one op", nil)
	}
	root, err := os.OpenRoot(run.stage)
	if err != nil {
		return LayoutUpdate{}, s.internalError("update layout", err)
	}
	defer root.Close()
	p, err := readLayout(root)
	if err != nil {
		return LayoutUpdate{}, s.internalError("update layout", err)
	}
	next, results, err := ApplyOps(p, ops, s.ids)
	if err != nil {
		return LayoutUpdate{}, err
	}
	if err := checkLayoutAssets(p.Assets, next.Assets); err != nil {
		return LayoutUpdate{}, err
	}
	data, err := MarshalProject(next)
	if err != nil {
		return LayoutUpdate{}, s.internalError("update layout", err) // a typed CYD_INVALID_LAYOUT passes through
	}
	if _, err := ParseProject(data); err != nil {
		return LayoutUpdate{}, err
	}
	if err := rootWriteFile(ctx, root, ProjectFile, data); err != nil {
		return LayoutUpdate{}, s.internalError("update layout", err)
	}
	run.touch()
	summaries := make([]string, 0, 3)
	for _, r := range results {
		if len(summaries) == 3 {
			break
		}
		summaries = append(summaries, r.Summary)
	}
	text := fmt.Sprintf("%d layout change(s): %s", len(results), strings.Join(summaries, "; "))
	if len(results) > 3 {
		text += "; …"
	}
	s.emitStep(run.projectID, run.jobID, StepLayout, text)
	return LayoutUpdate{Results: results, Issues: validateLayout(next)}, nil
}

// JobFile is the cyd_creator_read_file answer.
type JobFile struct {
	Path      string `json:"path"`
	Content   string `json:"content"`
	SHA256    string `json:"sha256"`
	Generated bool   `json:"generated"`
}

// jobReadonlyPathError is readonlyPathError with the model's raw path clipped
// in the details (it is echoed before any path validation).
func jobReadonlyPathError(rel string) error {
	err := readonlyPathError(rel)
	var typed *Error
	if errors.As(err, &typed) && typed.Details != nil {
		typed.Details["path"] = clipEcho(rel)
	}
	return err
}

// ReadJobFile reads an app file of the working copy, a generated file
// rendered from the working copy, or the working copy's layout file
// (cyd.project.json); the latter two are read-only. App paths are exact-case
// like ReadFile: a letter-case variant is CYD_NOT_FOUND. The agent dispatcher
// pages the content into the tool output budget.
func (s *Service) ReadJobFile(ctx context.Context, jobID, rel string) (JobFile, error) {
	run, err := s.jobTool(jobID, ToolReadFile)
	if err != nil {
		return JobFile{}, err
	}
	notFound := NewError(CodeNotFound, "file not found", map[string]any{"path": clipEcho(rel)})
	switch {
	case rel == ProjectFile:
		root, err := os.OpenRoot(run.stage)
		if err != nil {
			return JobFile{}, s.internalError("read file", err)
		}
		defer root.Close()
		data, err := rootReadFile(root, ProjectFile, MaxLayoutBytes)
		if err != nil {
			return JobFile{}, s.internalError("read file", layoutFileError(ProjectFile, err))
		}
		return JobFile{Path: rel, Content: string(data), SHA256: sha256Hex(data)}, nil
	case IsEditablePath(rel):
		root, err := os.OpenRoot(run.stage)
		if err != nil {
			return JobFile{}, s.internalError("read file", err)
		}
		defer root.Close()
		onDisk, err := appPathOnDisk(root, rel)
		if err != nil {
			return JobFile{}, s.internalError("read file", err)
		}
		if onDisk != rel {
			return JobFile{}, notFound
		}
		data, err := rootReadFile(root, rel, MaxAppFileBytes)
		switch {
		case errors.Is(err, fs.ErrNotExist):
			return JobFile{}, notFound
		case errors.Is(err, errNotRegularFile):
			return JobFile{}, notRegularError(rel)
		case errors.Is(err, errFileTooLarge):
			return JobFile{}, NewError(CodeFileLimit, fmt.Sprintf("%s is larger than %d bytes", rel, MaxAppFileBytes),
				map[string]any{"limit": "file_bytes", "max": MaxAppFileBytes, "path": rel})
		case err != nil:
			return JobFile{}, s.internalError("read file", err)
		}
		return JobFile{Path: rel, Content: string(data), SHA256: sha256Hex(data)}, nil
	case IsGeneratedPath(rel):
		files, err := s.Generate(ctx, run.projectID, run.stage)
		if err != nil {
			return JobFile{}, err
		}
		data, ok := files[rel]
		if !ok {
			return JobFile{}, notFound
		}
		return JobFile{Path: rel, Content: string(data), SHA256: sha256Hex(data), Generated: true}, nil
	}
	return JobFile{}, jobReadonlyPathError(rel)
}

// WriteJobFile writes an app file of the working copy with the rules of an
// unconditional WriteFile (src/app/** only, UTF-8, 256 KiB per file, 64
// files, 2 MiB, exact-case paths, never over a directory, link or device).
func (s *Service) WriteJobFile(ctx context.Context, jobID, rel, content string) (string, error) {
	run, err := s.jobTool(jobID, ToolWriteFile)
	if err != nil {
		return "", err
	}
	data := []byte(content)
	if !IsEditablePath(rel) {
		return "", jobReadonlyPathError(rel)
	}
	if !utf8.Valid(data) || bytes.IndexByte(data, 0) >= 0 {
		return "", NewError(CodeBadRequest, "source files must be UTF-8 text", map[string]any{"path": rel})
	}
	if len(data) > MaxAppFileBytes {
		return "", NewError(CodeFileLimit, fmt.Sprintf("a source file may have at most %d bytes", MaxAppFileBytes),
			map[string]any{"limit": "file_bytes", "max": MaxAppFileBytes, "path": rel})
	}
	root, err := os.OpenRoot(run.stage)
	if err != nil {
		return "", s.internalError("write file", err)
	}
	defer root.Close()
	files, total, err := listAppFiles(root)
	if err != nil {
		return "", s.internalError("write file", err)
	}
	// Checked before the existence test: on case-insensitive file systems the
	// other spelling would otherwise be overwritten.
	for _, f := range files {
		if f.Path != rel && strings.EqualFold(f.Path, rel) {
			return "", NewError(CodeFileConflict, "a file with the same name in different letter case exists",
				map[string]any{"path": rel, "existing": f.Path, "sha256": ""})
		}
	}
	cur, err := currentAppFile(root, rel)
	if err != nil {
		return "", s.internalError("write file", err)
	}
	if cur.alias != "" {
		return "", NewError(CodeFileConflict, "a file or folder with the same name in different letter case exists",
			map[string]any{"path": rel, "existing": cur.alias, "sha256": ""})
	}
	if cur.exists && !cur.regular {
		return "", notRegularError(rel)
	}
	if !cur.exists && len(files)+1 > MaxAppFiles {
		return "", NewError(CodeFileLimit, fmt.Sprintf("a project may have at most %d source files", MaxAppFiles),
			map[string]any{"limit": "files", "max": MaxAppFiles, "path": rel})
	}
	if total-cur.size+int64(len(data)) > MaxAppTotalBytes {
		return "", NewError(CodeFileLimit, fmt.Sprintf("all source files together may have at most %d bytes", MaxAppTotalBytes),
			map[string]any{"limit": "total_bytes", "max": MaxAppTotalBytes, "path": rel})
	}
	if err := rootWriteFile(ctx, root, rel, data); err != nil {
		return "", s.internalError("write file", err)
	}
	run.touch()
	s.emitStep(run.projectID, run.jobID, StepFile, fmt.Sprintf("Wrote %s (%d lines)", rel, strings.Count(content, "\n")+1))
	return sha256Hex(data), nil
}

// DeleteJobFile removes an app file from the working copy (exact-case path,
// regular files only, like DeleteFile) and the folders it leaves empty.
func (s *Service) DeleteJobFile(ctx context.Context, jobID, rel string) error {
	run, err := s.jobTool(jobID, ToolDeleteFile)
	if err != nil {
		return err
	}
	if !IsEditablePath(rel) {
		return jobReadonlyPathError(rel)
	}
	root, err := os.OpenRoot(run.stage)
	if err != nil {
		return s.internalError("delete file", err)
	}
	defer root.Close()
	cur, err := currentAppFile(root, rel)
	if err != nil {
		return s.internalError("delete file", err)
	}
	if !cur.exists {
		return NewError(CodeNotFound, "file not found", map[string]any{"path": rel})
	}
	if !cur.regular {
		return notRegularError(rel)
	}
	if err := fileutil.PublishContext(ctx, func() error { return root.Remove(filepath.FromSlash(rel)) }); err != nil {
		return s.internalError("delete file", err)
	}
	for dir := path.Dir(rel); dir != AppDir && strings.HasPrefix(dir, AppDir+"/"); dir = path.Dir(dir) {
		if root.Remove(filepath.FromSlash(dir)) != nil {
			break
		}
	}
	run.touch()
	s.emitStep(run.projectID, run.jobID, StepFile, "Deleted "+rel)
	return nil
}
