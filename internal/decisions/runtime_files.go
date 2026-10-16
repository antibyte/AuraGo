package decisions

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func validateRuntimeFiles(ctx context.Context, directory string, manifest runtimeManifest) error {
	if len(manifest.Files) == 0 || len(manifest.Files) > 1024 {
		return fmt.Errorf("invalid runtime file inventory")
	}
	seen := map[string]bool{}
	foundBinary := false
	for _, entry := range manifest.Files {
		// Release packages are flat, including DLLs and uniquely named licenses.
		if entry.File == "" || filepath.Base(entry.File) != entry.File || strings.ContainsAny(entry.File, "/\\:") || strings.HasSuffix(entry.File, ".") || strings.HasSuffix(entry.File, " ") || entry.File == "." || entry.File == ".." || entry.File == "release-manifest.json" || entry.File == ".complete.json" || entry.Size <= 0 || !validSHA256(entry.SHA256) || seen[strings.ToLower(entry.File)] {
			return fmt.Errorf("invalid runtime file entry")
		}
		seen[strings.ToLower(entry.File)] = true
		path := filepath.Join(directory, entry.File)
		info, err := os.Lstat(path)
		if err != nil || !info.Mode().IsRegular() || info.Size() != entry.Size {
			return fmt.Errorf("runtime file size or type mismatch")
		}
		digest, err := fileSHA256(ctx, path)
		if err != nil {
			return err
		}
		if !strings.EqualFold(digest, entry.SHA256) {
			return fmt.Errorf("runtime file digest mismatch")
		}
		if entry.File == manifest.Artifact.File {
			foundBinary = entry.Size == manifest.Artifact.SizeBytes && strings.EqualFold(entry.SHA256, manifest.Artifact.SHA256)
		}
	}
	if !foundBinary {
		return fmt.Errorf("runtime inventory omits executable")
	}
	entries, err := os.ReadDir(directory)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if entry.Name() != "release-manifest.json" && entry.Name() != ".complete.json" && !seen[strings.ToLower(entry.Name())] {
			return fmt.Errorf("unexpected runtime file")
		}
	}
	return ctx.Err()
}
