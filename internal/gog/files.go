package gog

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// Files carries already-authorized streams, never caller-chosen CLI paths.
type Files struct {
	Input  io.Reader
	Output io.Writer
	Name   string
}

func (f *Files) stage(dir string) (string, error) {
	if f == nil || f.Input == nil {
		return "", fmt.Errorf("input_file_required")
	}
	ext := strings.ToLower(filepath.Ext(f.Name))
	if len(ext) > 12 || strings.ContainsAny(ext, "/\\\x00") {
		ext = ""
	}
	p := filepath.Join(dir, "input"+ext)
	out, e := os.OpenFile(p, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if e != nil {
		return "", e
	}
	n, e := io.Copy(out, io.LimitReader(f.Input, MaxFile+1))
	closeErr := out.Close()
	if e != nil {
		return "", e
	}
	if closeErr != nil {
		return "", closeErr
	}
	if n > MaxFile {
		return "", fmt.Errorf("file_too_large")
	}
	return p, nil
}
func (f *Files) collect(dir string) error {
	if f.Output == nil {
		return fmt.Errorf("output_required")
	}
	r, e := os.OpenRoot(dir)
	if e != nil {
		return e
	}
	defer r.Close()
	info, e := r.Lstat("output")
	if e != nil || !info.Mode().IsRegular() || info.Size() > MaxFile {
		return fmt.Errorf("unsafe_output")
	}
	file, e := r.Open("output")
	if e != nil {
		return e
	}
	defer file.Close()
	n, e := io.Copy(f.Output, io.LimitReader(file, MaxFile+1))
	if n > MaxFile {
		return fmt.Errorf("file_too_large")
	}
	return e
}
