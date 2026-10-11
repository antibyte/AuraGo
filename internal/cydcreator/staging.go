package cydcreator

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"
)

// stagingDirName is the workspace folder that holds job working copies
// (<workspace>/.cyd-creator-staging/<job_id>/) and restore stages.
const stagingDirName = ".cyd-creator-staging"

const (
	maxStagingFiles = 1000
	maxStagingBytes = 64 << 20
)

func (s *Service) stagingRoot() string { return filepath.Join(s.workspaceDir, stagingDirName) }

// stagingPath is the working copy of a job.
func (s *Service) stagingPath(jobID string) string { return filepath.Join(s.stagingRoot(), jobID) }

// stagingRel is the staging_dir column value (workspace-relative, slash form).
func stagingRel(jobID string) string { return stagingDirName + "/" + jobID }

// copyTree copies the regular files of src into the new directory dst.
// Symlinks and other special files are refused; the copy is bounded.
func copyTree(src, dst string) error {
	srcRoot, err := os.OpenRoot(src)
	if err != nil {
		return fmt.Errorf("open copy source: %w", err)
	}
	defer srcRoot.Close()
	if err := os.MkdirAll(filepath.Dir(dst), 0o750); err != nil {
		return err
	}
	if err := os.Mkdir(dst, 0o750); err != nil {
		return fmt.Errorf("create copy target: %w", err)
	}
	dstRoot, err := os.OpenRoot(dst)
	if err != nil {
		return err
	}
	defer dstRoot.Close()
	files, total := 0, int64(0)
	return fs.WalkDir(srcRoot.FS(), ".", func(rel string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if rel == "." {
			return nil
		}
		if strings.HasPrefix(path.Base(rel), ".cyd-write-") {
			return nil // an interrupted atomic write, never project content
		}
		switch {
		case d.IsDir():
			return dstRoot.Mkdir(filepath.FromSlash(rel), 0o750)
		case !d.Type().IsRegular():
			return fmt.Errorf("%s is not a regular file", rel)
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		files++
		total += info.Size()
		if files > maxStagingFiles || total > maxStagingBytes {
			return errors.New("the project is too large to copy")
		}
		in, err := srcRoot.Open(filepath.FromSlash(rel))
		if err != nil {
			return err
		}
		defer in.Close()
		out, err := dstRoot.OpenFile(filepath.FromSlash(rel), os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o640)
		if err != nil {
			return err
		}
		// Copy at most the counted size; a file that grew meanwhile fails the
		// copy instead of exceeding the bound.
		n, err := io.Copy(out, io.LimitReader(in, info.Size()+1))
		if err == nil && n != info.Size() {
			err = fmt.Errorf("%s changed while it was copied", rel)
		}
		if err != nil {
			_ = out.Close()
			return err
		}
		return out.Close()
	})
}

// removeStaging deletes a job's working copy (only well-formed job ids, so
// the path never leaves the staging folder).
func (s *Service) removeStaging(jobID string) {
	if !validElementID(jobID) {
		return
	}
	if err := os.RemoveAll(s.stagingPath(jobID)); err != nil {
		s.logger.Warn("CYD Creator working copy could not be removed", "job_id", jobID, "error", err)
	}
}

func dirExists(p string) bool {
	info, err := os.Stat(p)
	return err == nil && info.IsDir()
}
