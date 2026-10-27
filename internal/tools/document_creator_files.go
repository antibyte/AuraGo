package tools

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"unicode"
	"unicode/utf8"

	"aurago/internal/config"
)

const (
	documentSourceFileLimit  = 32 << 20
	documentSourceTotalLimit = 64 << 20
	documentSourceCountLimit = 20
)

func validateDocumentSourceCount(operation string, count int) error {
	if count > documentSourceCountLimit {
		return fmt.Errorf("source_files allows at most %d files", documentSourceCountLimit)
	}
	if operation == "convert_document" && count != 1 {
		return fmt.Errorf("source_files must contain exactly one file for convert_document")
	}
	if operation == "merge_pdfs" && count < 2 {
		return fmt.Errorf("source_files must contain at least 2 PDF files for merge_pdfs")
	}
	return nil
}

// A nil workspace is used only by backend callers reading already staged files.
func openDocumentSources(ctx context.Context, paths []string, workspace *config.Config) (files []*os.File, err error) {
	defer func() {
		if err != nil {
			closeDocumentSources(files)
		}
	}()
	var total int64
	for _, path := range paths {
		if err = ctx.Err(); err != nil {
			return files, err
		}
		var info os.FileInfo
		if workspace != nil {
			path, err = resolveToolInputPath(path, workspace)
			if err == nil {
				info, err = rootedToolStat(path)
			}
		} else {
			info, err = os.Stat(path)
		}
		if err != nil {
			return files, fmt.Errorf("invalid source path: %w", err)
		}
		if !info.Mode().IsRegular() {
			return files, fmt.Errorf("source_files must contain regular files")
		}
		var file *os.File
		if workspace != nil {
			file, err = rootedToolOpenFile(path, os.O_RDONLY|outgoingOpenNonblock, 0)
		} else {
			file, err = os.OpenFile(path, os.O_RDONLY|outgoingOpenNonblock, 0)
		}
		if err != nil {
			return files, fmt.Errorf("open source file: %w", err)
		}
		files = append(files, file)
		info, err = file.Stat()
		if err != nil {
			return files, fmt.Errorf("stat source file: %w", err)
		}
		if !info.Mode().IsRegular() {
			return files, fmt.Errorf("source_files must contain regular files")
		}
		if info.Size() > documentSourceFileLimit {
			return files, fmt.Errorf("source file exceeds the 32 MiB limit")
		}
		total += info.Size()
		if total > documentSourceTotalLimit {
			return files, fmt.Errorf("source_files exceed the 64 MiB total limit")
		}
	}
	return files, nil
}

func closeDocumentSources(files []*os.File) {
	for _, file := range files {
		_ = file.Close()
	}
}

func copyDocumentSource(ctx context.Context, dst io.Writer, src io.Reader, remaining *int64) error {
	limit := min(int64(documentSourceFileLimit), *remaining)
	n, err := io.Copy(dst, io.LimitReader(mediaConversionContextReader{ctx: ctx, r: src}, limit+1))
	if err != nil {
		return fmt.Errorf("copy source file: %w", err)
	}
	if n > limit {
		return fmt.Errorf("source_files exceed the 32 MiB per-file or 64 MiB total limit")
	}
	*remaining -= n
	return ctx.Err()
}

func readDocumentSources(ctx context.Context, operation string, paths []string) ([][]byte, error) {
	if err := validateDocumentSourceCount(operation, len(paths)); err != nil {
		return nil, err
	}
	files, err := openDocumentSources(ctx, paths, nil)
	if err != nil {
		return nil, err
	}
	defer closeDocumentSources(files)
	remaining := int64(documentSourceTotalLimit)
	data := make([][]byte, 0, len(files))
	for _, file := range files {
		var buf bytes.Buffer
		if err := copyDocumentSource(ctx, &buf, file, &remaining); err != nil {
			return nil, err
		}
		data = append(data, buf.Bytes())
	}
	return data, nil
}

func documentOutputName(filename, ext string) (string, error) {
	if filename == "" {
		filename = defaultDocumentName()
	}
	if strings.ContainsRune(filename, ':') {
		return "", fmt.Errorf("document filename must not contain a drive or NTFS stream")
	}
	for _, r := range filename {
		if unicode.IsControl(r) {
			return "", fmt.Errorf("document filename must not contain control characters")
		}
	}
	filename = filepath.Base(strings.ReplaceAll(filename, "\\", "/"))
	if !utf8.ValidString(filename) || filename == "." || filename == ".." ||
		strings.ContainsAny(filename, `<>:"|?*`) || strings.TrimRight(filename, ". ") != filename {
		return "", fmt.Errorf("invalid document filename")
	}
	base := strings.ToUpper(strings.TrimRight(strings.SplitN(filename, ".", 2)[0], " "))
	switch base {
	case "CON", "PRN", "AUX", "NUL", "CONIN$", "CONOUT$", "CLOCK$":
		return "", fmt.Errorf("reserved document filename")
	}
	if (strings.HasPrefix(base, "COM") || strings.HasPrefix(base, "LPT")) &&
		len([]rune(base)) == 4 && strings.ContainsRune("123456789¹²³", []rune(base)[3]) {
		return "", fmt.Errorf("reserved document filename")
	}
	if !strings.HasSuffix(strings.ToLower(filename), ext) {
		filename += ext
	}
	return filename, nil
}

func saveDocumentOutput(ctx context.Context, data []byte, outputDir, filename, ext string) (string, string, error) {
	if err := ctx.Err(); err != nil {
		return "", "", err
	}
	filename, err := documentOutputName(filename, ext)
	if err != nil {
		return "", "", err
	}
	root, err := os.OpenRoot(outputDir)
	if err != nil {
		return "", "", fmt.Errorf("open output dir: %w", err)
	}
	defer root.Close()
	stem := strings.TrimSuffix(filename, filepath.Ext(filename))
	remainingExt := filepath.Ext(filename)
	for attempt := 0; attempt < 1000; attempt++ {
		if err := ctx.Err(); err != nil {
			return "", "", err
		}
		name := filename
		if attempt > 0 {
			name = fmt.Sprintf("%s-%d%s", stem, attempt, remainingExt)
		}
		file, err := root.OpenFile(name, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0640)
		if errors.Is(err, os.ErrExist) {
			continue
		}
		if err != nil {
			return "", "", fmt.Errorf("create document: %w", err)
		}
		created, statErr := file.Stat()
		_, writeErr := io.Copy(file, mediaConversionContextReader{ctx: ctx, r: bytes.NewReader(data)})
		if writeErr == nil {
			writeErr = file.Sync()
		}
		closeErr := file.Close()
		err = errors.Join(statErr, writeErr, closeErr, ctx.Err())
		if err != nil {
			// Remove only the inode created by this attempt, never a replacement.
			if current, checkErr := root.Lstat(name); checkErr == nil && created != nil && os.SameFile(created, current) {
				_ = root.Remove(name)
			}
			return "", "", fmt.Errorf("write document: %w", err)
		}
		return filepath.Join(outputDir, name), "/files/documents/" + url.PathEscape(name), nil
	}
	return "", "", fmt.Errorf("document filename is occupied in all 1000 variants; choose another filename")
}
