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

// ExecuteArchive handles archive operations: create, extract, list.
//
//	operation  – "create", "extract", or "list"
//	archivePath – path to the archive file (source for extract/list, target for create)
//	targetDir   – extraction target dir (extract) or source dir (create)
//	sourceFiles – JSON-encoded array of specific file paths (create only; "" = use targetDir)
//	format      – "zip" or "tar.gz" (create only; extract/list auto-detect from extension)
func ExecuteArchive(operation, archivePath, targetDir, sourceFiles, format string) string {
	encode := func(r archiveResult) string {
		b, _ := json.Marshal(r)
		return string(b)
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

	if err := os.MkdirAll(targetDir, 0o750); err != nil {
		return nil, err
	}

	var files []string
	var totalBytes int64
	for _, f := range r.File {
		name := filepath.FromSlash(f.Name)
		// Path traversal protection
		dest := filepath.Join(targetDir, name)
		if !strings.HasPrefix(filepath.Clean(dest), filepath.Clean(targetDir)) {
			return nil, fmt.Errorf("illegal path in archive (path traversal): %s", f.Name)
		}

		if f.FileInfo().IsDir() {
			if err := os.MkdirAll(dest, 0o750); err != nil {
				return nil, err
			}
			continue
		}

		// Size limit check
		totalBytes += int64(f.UncompressedSize64)
		if totalBytes > maxExtractSize {
			return nil, fmt.Errorf("archive exceeds maximum uncompressed size limit (%d bytes)", maxExtractSize)
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

	gzr, err := gzip.NewReader(f)
	if err != nil {
		return nil, fmt.Errorf("gzip reader: %w", err)
	}
	defer gzr.Close()

	tr := tar.NewReader(gzr)
	if err := os.MkdirAll(targetDir, 0o750); err != nil {
		return nil, err
	}

	var files []string
	var totalBytes int64
	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}

		name := filepath.FromSlash(header.Name)
		dest := filepath.Join(targetDir, name)
		if !strings.HasPrefix(filepath.Clean(dest), filepath.Clean(targetDir)) {
			return nil, fmt.Errorf("illegal path in archive (path traversal): %s", header.Name)
		}

		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(dest, 0o750); err != nil {
				return nil, err
			}
		case tar.TypeReg:
			totalBytes += header.Size
			if totalBytes > maxExtractSize {
				return nil, fmt.Errorf("archive exceeds maximum uncompressed size limit (%d bytes)", maxExtractSize)
			}
			if err := os.MkdirAll(filepath.Dir(dest), 0o750); err != nil {
				return nil, err
			}
			outFile, err := os.OpenFile(dest, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, os.FileMode(header.Mode))
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
	return files, nil
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
