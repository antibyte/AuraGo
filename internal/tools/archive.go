// Package tools – archive: create, extract, and list ZIP and TAR.GZ archives.
package tools

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// archiveResult is the JSON payload returned by ExecuteArchive.
type archiveResult struct {
	Status  string   `json:"status"`
	Message string   `json:"message,omitempty"`
	Files   []string `json:"files,omitempty"`
	Total   int      `json:"total,omitempty"`
}

// maxExtractSize is the cumulative uncompressed data limit (1 GB).
const maxExtractSize int64 = 1 << 30

// maxArchiveEntries limits the number of entries to prevent zip bomb-style attacks.
const maxArchiveEntries = 100000

// maxCompressionRatio is the maximum allowed uncompressed/compressed size ratio.
// A ratio above this is suspicious and may indicate a zip bomb (e.g., highly compressed data).
// 1000:1 is a conservative limit that allows legitimate highly-compressed files while
// blocking extreme cases like 1000000:1 decompression bombs.
const maxCompressionRatio = 1000

// ExecuteArchive handles archive operations: create, extract, list.
//
//	operation  – "create", "extract", or "list"
//	archivePath – path to the archive file (source for extract/list, target for create)
//	targetDir   – extraction target dir (extract) or source dir (create)
//	sourceFiles – JSON-encoded array of specific file paths (create only; "" = use targetDir)
//	format      – "zip" or "tar.gz" (create only; extract/list auto-detect from extension)
func ExecuteArchive(workspaceDir, operation, archivePath, targetDir, sourceFiles, format string) string {
	encode := func(r archiveResult) string {
		b, _ := json.Marshal(r)
		return string(b)
	}

	if workspaceDir != "" {
		if archivePath != "" {
			resolved, err := secureResolve(workspaceDir, archivePath)
			if err != nil {
				return encode(archiveResult{Status: "error", Message: fmt.Sprintf("invalid archive path: %v", err)})
			}
			archivePath = resolved
		}
		if targetDir != "" {
			resolved, err := secureResolve(workspaceDir, targetDir)
			if err != nil {
				return encode(archiveResult{Status: "error", Message: fmt.Sprintf("invalid target directory: %v", err)})
			}
			targetDir = resolved
		}
	}

	operation = strings.ToLower(strings.TrimSpace(operation))
	switch operation {
	case "create":
		return encode(archiveCreate(archivePath, targetDir, sourceFiles, format))
	case "extract":
		return encode(archiveExtract(archivePath, targetDir))
	case "list":
		return encode(archiveList(archivePath))
	default:
		return encode(archiveResult{Status: "error", Message: "operation must be 'create', 'extract', or 'list'"})
	}
}

// ── Create ───────────────────────────────────────────────────────────────────

func archiveCreate(archivePath, sourceDir, sourceFilesJSON, format string) archiveResult {
	if archivePath == "" {
		return archiveResult{Status: "error", Message: "archive path is required"}
	}
	if sourceDir == "" && sourceFilesJSON == "" {
		return archiveResult{Status: "error", Message: "target_dir or source_files is required for create"}
	}

	// Determine format
	if format == "" {
		format = detectArchiveFormat(archivePath)
	}
	format = strings.ToLower(strings.TrimSpace(format))

	// Collect files to add
	files, err := collectFiles(sourceDir, sourceFilesJSON)
	if err != nil {
		return archiveResult{Status: "error", Message: fmt.Sprintf("collect files: %v", err)}
	}
	if len(files) == 0 {
		return archiveResult{Status: "error", Message: "no files found to archive"}
	}

	// Ensure output directory exists
	if dir := filepath.Dir(archivePath); dir != "" {
		if err := os.MkdirAll(dir, 0o750); err != nil {
			return archiveResult{Status: "error", Message: fmt.Sprintf("mkdir: %v", err)}
		}
	}

	switch format {
	case "zip":
		if err := createZip(archivePath, files, sourceDir); err != nil {
			return archiveResult{Status: "error", Message: fmt.Sprintf("create zip: %v", err)}
		}
	case "tar.gz", "tgz", "tar":
		if err := createTarGz(archivePath, files, sourceDir); err != nil {
			return archiveResult{Status: "error", Message: fmt.Sprintf("create tar.gz: %v", err)}
		}
	default:
		return archiveResult{Status: "error", Message: "format must be 'zip' or 'tar.gz'"}
	}

	names := make([]string, 0, len(files))
	for _, f := range files {
		names = append(names, f)
	}
	return archiveResult{Status: "success", Message: fmt.Sprintf("created %s with %d file(s)", archivePath, len(files)), Files: names, Total: len(files)}
}

func collectFiles(sourceDir, sourceFilesJSON string) ([]string, error) {
	if sourceFilesJSON != "" {
		var paths []string
		if err := json.Unmarshal([]byte(sourceFilesJSON), &paths); err != nil {
			return nil, fmt.Errorf("parse source_files JSON: %w", err)
		}
		// Reject absolute paths and path traversal attempts in source_files
		for _, p := range paths {
			if filepath.IsAbs(p) {
				return nil, fmt.Errorf("absolute paths are not allowed in source_files: %q", p)
			}
			cleaned := filepath.Clean(p)
			if cleaned == ".." || strings.HasPrefix(cleaned, ".."+string(filepath.Separator)) {
				return nil, fmt.Errorf("path traversal not allowed in source_files: %q", p)
			}
		}
		return paths, nil
	}
	var files []string
	err := filepath.WalkDir(sourceDir, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() {
			files = append(files, p)
		}
		return nil
	})
	return files, err
}

func createZip(archivePath string, files []string, baseDir string) error {
	f, err := os.Create(archivePath)
	if err != nil {
		return err
	}
	defer f.Close()

	w := zip.NewWriter(f)
	defer w.Close()

	for _, fpath := range files {
		if err := addToZip(w, fpath, baseDir); err != nil {
			return fmt.Errorf("add %s: %w", fpath, err)
		}
	}
	return nil
}

func addToZip(w *zip.Writer, fpath, baseDir string) error {
	info, err := os.Stat(fpath)
	if err != nil {
		return err
	}
	header, err := zip.FileInfoHeader(info)
	if err != nil {
		return err
	}
	rel, err := filepath.Rel(baseDir, fpath)
	if err != nil {
		rel = filepath.Base(fpath)
	}
	header.Name = filepath.ToSlash(rel)
	header.Method = zip.Deflate

	writer, err := w.CreateHeader(header)
	if err != nil {
		return err
	}
	file, err := os.Open(fpath)
	if err != nil {
		return err
	}
	defer file.Close()
	_, err = io.Copy(writer, file)
	return err
}

func createTarGz(archivePath string, files []string, baseDir string) error {
	f, err := os.Create(archivePath)
	if err != nil {
		return err
	}
	defer f.Close()

	gzw := gzip.NewWriter(f)
	defer gzw.Close()

	tw := tar.NewWriter(gzw)
	defer tw.Close()

	for _, fpath := range files {
		if err := addToTar(tw, fpath, baseDir); err != nil {
			return fmt.Errorf("add %s: %w", fpath, err)
		}
	}
	return nil
}

func addToTar(tw *tar.Writer, fpath, baseDir string) error {
	info, err := os.Stat(fpath)
	if err != nil {
		return err
	}
	rel, err := filepath.Rel(baseDir, fpath)
	if err != nil {
		rel = filepath.Base(fpath)
	}
	header, err := tar.FileInfoHeader(info, "")
	if err != nil {
		return err
	}
	header.Name = filepath.ToSlash(rel)

	if err := tw.WriteHeader(header); err != nil {
		return err
	}
	file, err := os.Open(fpath)
	if err != nil {
		return err
	}
	defer file.Close()
	_, err = io.Copy(tw, file)
	return err
}

// ── Extract ──────────────────────────────────────────────────────────────────

func archiveExtract(archivePath, targetDir string) archiveResult {
	if archivePath == "" {
		return archiveResult{Status: "error", Message: "archive path is required"}
	}
	if targetDir == "" {
		targetDir = "."
	}

	format := detectArchiveFormat(archivePath)
	switch format {
	case "zip":
		files, err := extractZip(archivePath, targetDir)
		if err != nil {
			return archiveResult{Status: "error", Message: fmt.Sprintf("extract zip: %v", err)}
		}
		return archiveResult{Status: "success", Message: fmt.Sprintf("extracted %d file(s) to %s", len(files), targetDir), Files: files, Total: len(files)}
	case "tar.gz":
		files, err := extractTarGz(archivePath, targetDir)
		if err != nil {
			return archiveResult{Status: "error", Message: fmt.Sprintf("extract tar.gz: %v", err)}
		}
		return archiveResult{Status: "success", Message: fmt.Sprintf("extracted %d file(s) to %s", len(files), targetDir), Files: files, Total: len(files)}
	default:
		return archiveResult{Status: "error", Message: "unsupported archive format. Supported: .zip, .tar.gz, .tgz"}
	}
}

func extractZip(archivePath, targetDir string) ([]string, error) {
	r, err := zip.OpenReader(archivePath)
	if err != nil {
		return nil, err
	}
	defer r.Close()

	// Entry count limit to prevent zip bombs
	if len(r.File) > maxArchiveEntries {
		return nil, fmt.Errorf("archive contains too many entries (%d), maximum allowed is %d", len(r.File), maxArchiveEntries)
	}

	if err := os.MkdirAll(targetDir, 0o750); err != nil {
		return nil, err
	}
	baseDir, err := archiveBaseDir(targetDir)
	if err != nil {
		return nil, err
	}

	var files []string
	var totalBytes int64
	for _, f := range r.File {
		name := filepath.FromSlash(f.Name)
		dest, err := archiveSafeDestination(baseDir, name)
		if err != nil {
			return nil, fmt.Errorf("illegal path in archive: %s", f.Name)
		}
		if f.Mode()&os.ModeSymlink != 0 {
			return nil, fmt.Errorf("symlink entries are not allowed in archives: %s", f.Name)
		}

		if f.FileInfo().IsDir() {
			if err := archiveEnsureSafePath(baseDir, dest, true); err != nil {
				return nil, err
			}
			if err := os.MkdirAll(dest, 0o750); err != nil {
				return nil, err
			}
			continue
		}

		// Compression ratio check to detect zip bombs
		if f.CompressedSize64 > 0 && f.UncompressedSize64 > 0 {
			ratio := f.UncompressedSize64 / f.CompressedSize64
			if ratio > maxCompressionRatio {
				return nil, fmt.Errorf("archive contains entry with extreme compression ratio (%d:1), suspected zip bomb", ratio)
			}
		}

		// Size limit check
		if f.UncompressedSize64 > uint64(maxExtractSize) {
			return nil, fmt.Errorf("archive exceeds maximum uncompressed size limit (%d bytes)", maxExtractSize)
		}
		totalBytes += int64(f.UncompressedSize64)
		if totalBytes > maxExtractSize {
			return nil, fmt.Errorf("archive exceeds maximum uncompressed size limit (%d bytes)", maxExtractSize)
		}

		if err := archiveEnsureSafePath(baseDir, dest, false); err != nil {
			return nil, err
		}
		if err := os.MkdirAll(filepath.Dir(dest), 0o750); err != nil {
			return nil, err
		}

		rc, err := f.Open()
		if err != nil {
			return nil, err
		}
		outFile, err := os.OpenFile(dest, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, f.Mode())
		if err != nil {
			rc.Close()
			return nil, err
		}
		written, err := io.Copy(outFile, io.LimitReader(rc, maxExtractSize-totalBytes+int64(f.UncompressedSize64)))
		rc.Close()
		outFile.Close()
		if err != nil {
			return nil, err
		}
		_ = written
		files = append(files, name)
	}
	return files, nil
}

func extractTarGz(archivePath, targetDir string) ([]string, error) {
	f, err := os.Open(archivePath)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	// Get compressed file size for ratio calculation
	compressedInfo, err := f.Stat()
	if err != nil {
		return nil, fmt.Errorf("stat archive: %w", err)
	}
	compressedSize := compressedInfo.Size()

	gzr, err := gzip.NewReader(f)
	if err != nil {
		return nil, fmt.Errorf("gzip reader: %w", err)
	}
	defer gzr.Close()

	tr := tar.NewReader(gzr)
	if err := os.MkdirAll(targetDir, 0o750); err != nil {
		return nil, err
	}
	baseDir, err := archiveBaseDir(targetDir)
	if err != nil {
		return nil, err
	}

	var files []string
	var totalBytes int64
	var entryCount int
	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}

		// Entry count limit to prevent zip bombs
		entryCount++
		if entryCount > maxArchiveEntries {
			return nil, fmt.Errorf("archive contains too many entries (%d), maximum allowed is %d", entryCount, maxArchiveEntries)
		}

		name := filepath.FromSlash(header.Name)
		dest, err := archiveSafeDestination(baseDir, name)
		if err != nil {
			return nil, fmt.Errorf("illegal path in archive: %s", header.Name)
		}

		switch header.Typeflag {
		case tar.TypeDir:
			if err := archiveEnsureSafePath(baseDir, dest, true); err != nil {
				return nil, err
			}
			if err := os.MkdirAll(dest, 0o750); err != nil {
				return nil, err
			}
		case tar.TypeSymlink, tar.TypeLink:
			return nil, fmt.Errorf("symlink entries are not allowed in archives: %s", header.Name)
		case tar.TypeReg:
			totalBytes += header.Size
			if totalBytes > maxExtractSize {
				return nil, fmt.Errorf("archive exceeds maximum uncompressed size limit (%d bytes)", maxExtractSize)
			}
			if err := archiveEnsureSafePath(baseDir, dest, false); err != nil {
				return nil, err
			}
			if err := os.MkdirAll(filepath.Dir(dest), 0o750); err != nil {
				return nil, err
			}
			outFile, err := os.OpenFile(dest, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, os.FileMode(header.Mode&07777))
			if err != nil {
				return nil, err
			}
			if _, err := io.Copy(outFile, io.LimitReader(tr, header.Size)); err != nil {
				outFile.Close()
				return nil, err
			}
			outFile.Close()
			files = append(files, name)
		}
	}

	// Final compression ratio check for tar.gz (total uncompressed vs compressed)
	// This catches zip-bomb style archives where the entire stream decompresses to massive size
	if compressedSize > 0 && totalBytes > 0 {
		// Use int64 division carefully to avoid overflow
		ratio := totalBytes
		if ratio/Int64ToInt(compressedSize) > maxCompressionRatio {
			return nil, fmt.Errorf("archive has extreme compression ratio (%d:1), suspected decompression bomb", ratio/Int64ToInt(compressedSize))
		}
	}
	return files, nil
}

// Int64ToInt safely converts int64 to int for ratio calculations
func Int64ToInt(v int64) int64 {
	if v > 1<<30 {
		return 1 << 30 // cap at reasonable maximum to prevent overflow
	}
	return v
}

// ── List ─────────────────────────────────────────────────────────────────────

func archiveList(archivePath string) archiveResult {
	if archivePath == "" {
		return archiveResult{Status: "error", Message: "archive path is required"}
	}

	format := detectArchiveFormat(archivePath)
	switch format {
	case "zip":
		return listZip(archivePath)
	case "tar.gz":
		return listTarGz(archivePath)
	default:
		return archiveResult{Status: "error", Message: "unsupported archive format"}
	}
}

func listZip(archivePath string) archiveResult {
	r, err := zip.OpenReader(archivePath)
	if err != nil {
		return archiveResult{Status: "error", Message: fmt.Sprintf("open zip: %v", err)}
	}
	defer r.Close()

	var names []string
	for _, f := range r.File {
		names = append(names, f.Name)
	}
	return archiveResult{Status: "success", Files: names, Total: len(names), Message: fmt.Sprintf("%d entries in %s", len(names), archivePath)}
}

func listTarGz(archivePath string) archiveResult {
	f, err := os.Open(archivePath)
	if err != nil {
		return archiveResult{Status: "error", Message: fmt.Sprintf("open: %v", err)}
	}
	defer f.Close()

	gzr, err := gzip.NewReader(f)
	if err != nil {
		return archiveResult{Status: "error", Message: fmt.Sprintf("gzip reader: %v", err)}
	}
	defer gzr.Close()

	tr := tar.NewReader(gzr)
	var names []string
	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return archiveResult{Status: "error", Message: fmt.Sprintf("read tar: %v", err)}
		}
		names = append(names, header.Name)
	}
	return archiveResult{Status: "success", Files: names, Total: len(names), Message: fmt.Sprintf("%d entries in %s", len(names), archivePath)}
}

// ── Helpers ──────────────────────────────────────────────────────────────────

func detectArchiveFormat(path string) string {
	lower := strings.ToLower(path)
	if strings.HasSuffix(lower, ".tar.gz") || strings.HasSuffix(lower, ".tgz") {
		return "tar.gz"
	}
	if strings.HasSuffix(lower, ".zip") {
		return "zip"
	}
	return ""
}

func archiveBaseDir(targetDir string) (string, error) {
	absBase, err := filepath.Abs(targetDir)
	if err != nil {
		return "", err
	}
	if resolved, err := filepath.EvalSymlinks(absBase); err == nil {
		absBase = resolved
	}
	return filepath.Clean(absBase), nil
}

func archiveSafeDestination(baseDir, name string) (string, error) {
	dest := filepath.Clean(filepath.Join(baseDir, name))
	rel, err := filepath.Rel(baseDir, dest)
	if err != nil {
		return "", err
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
		return "", fmt.Errorf("path traversal")
	}
	return dest, nil
}

func archiveEnsureSafePath(baseDir, dest string, dirTarget bool) error {
	rel, err := filepath.Rel(baseDir, dest)
	if err != nil {
		return err
	}
	if rel == "." && dirTarget {
		return nil
	}
	parts := strings.Split(filepath.Clean(rel), string(os.PathSeparator))
	current := baseDir
	limit := len(parts)
	if !dirTarget {
		limit--
	}
	for i := 0; i < limit; i++ {
		current = filepath.Join(current, parts[i])
		info, err := os.Lstat(current)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("refusing to extract through symlink path: %s", current)
		}
		if !info.IsDir() {
			return fmt.Errorf("refusing to extract through non-directory path: %s", current)
		}
	}
	if !dirTarget {
		if info, err := os.Lstat(dest); err == nil && info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("refusing to overwrite symlink target: %s", dest)
		}
	}
	return nil
}
