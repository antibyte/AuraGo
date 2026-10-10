package layerling

import (
	"archive/zip"
	"bytes"
	"context"
	"fmt"
	"io"
	"io/fs"
	"path"
	"strings"
)

// ValidateArchive bounds both declared and actually decoded CAD archive content.
// Legacy JSON .lyl projects are validated by the pinned upstream importer.
func ValidateArchive(ctx context.Context, name string, data []byte) error {
	ext := strings.ToLower(path.Ext(name))
	if ext != ".3mf" && (ext != ".lyl" || !bytes.HasPrefix(data, []byte("PK"))) {
		return nil
	}
	r, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return fmt.Errorf("invalid CAD archive: %w", err)
	}
	if len(r.File) > 10000 {
		return fmt.Errorf("CAD archive exceeds 10000 entries")
	}
	const limit = 256 << 20
	remaining := int64(limit)
	seen := map[string]bool{}
	for _, f := range r.File {
		if err := ctx.Err(); err != nil {
			return err
		}
		n := strings.TrimSuffix(f.Name, "/")
		key := strings.ToLower(n)
		if !fs.ValidPath(n) || strings.ContainsAny(n, "\\:\x00") || seen[key] || f.Mode()&fs.ModeSymlink != 0 || (!f.Mode().IsRegular() && !f.FileInfo().IsDir()) {
			return fmt.Errorf("invalid CAD archive entry")
		}
		seen[key] = true
		if f.UncompressedSize64 > uint64(remaining) {
			return fmt.Errorf("CAD archive exceeds 256 MiB expansion limit")
		}
		reader, err := f.Open()
		if err != nil {
			return fmt.Errorf("open CAD archive entry: %w", err)
		}
		count, readErr := io.Copy(io.Discard, io.LimitReader(reader, remaining+1))
		closeErr := reader.Close()
		if readErr != nil || closeErr != nil || count > remaining || uint64(count) != f.UncompressedSize64 {
			return fmt.Errorf("invalid or oversized CAD archive content")
		}
		remaining -= count
	}
	return nil
}
