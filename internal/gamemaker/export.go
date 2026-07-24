package gamemaker

import (
	"archive/zip"
	"context"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

func (s *Service) WriteExport(ctx context.Context, projectID string, output io.Writer) (string, error) {
	project, err := s.GetProject(ctx, projectID)
	if err != nil {
		return "", err
	}
	if project.CurrentRevision <= 0 {
		return "", fmt.Errorf("project has no playable revision")
	}
	root := filepath.Join(s.opts.WorkspacePath, filepath.FromSlash(project.ProjectKey))
	archive := zip.NewWriter(output)
	included := make(map[string]bool)
	err = filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		info, err := os.Lstat(path)
		if err != nil {
			return err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("%w: symlink in exported project", ErrInvalidPath)
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		if strings.HasPrefix(rel, ".") || strings.Contains(rel, "/.") {
			return nil
		}
		included[rel] = true
		header, err := zip.FileInfoHeader(info)
		if err != nil {
			return err
		}
		header.Name = rel
		header.Method = zip.Deflate
		writer, err := archive.CreateHeader(header)
		if err != nil {
			return err
		}
		file, err := os.Open(path)
		if err != nil {
			return err
		}
		_, copyErr := io.Copy(writer, file)
		closeErr := file.Close()
		if copyErr != nil {
			return copyErr
		}
		return closeErr
	})
	if err != nil {
		_ = archive.Close()
		return "", fmt.Errorf("build game maker export: %w", err)
	}
	for _, asset := range bundledRuntimeAssets(project.Dimension) {
		if included[asset.projectPath] {
			continue
		}
		if err := ctx.Err(); err != nil {
			_ = archive.Close()
			return "", err
		}
		data, err := runtimeFS.ReadFile(asset.embeddedPath)
		if err != nil {
			_ = archive.Close()
			return "", fmt.Errorf("read exported game runtime %s: %w", asset.embeddedPath, err)
		}
		header := &zip.FileHeader{Name: asset.projectPath, Method: zip.Deflate}
		header.SetMode(0o640)
		writer, err := archive.CreateHeader(header)
		if err != nil {
			_ = archive.Close()
			return "", fmt.Errorf("create exported game runtime %s: %w", asset.projectPath, err)
		}
		if _, err := writer.Write(data); err != nil {
			_ = archive.Close()
			return "", fmt.Errorf("write exported game runtime %s: %w", asset.projectPath, err)
		}
	}
	if err := archive.Close(); err != nil {
		return "", fmt.Errorf("finish game maker export: %w", err)
	}
	return project.Slug + ".zip", nil
}
