package desktop

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"os"
	"path/filepath"
	"strings"

	_ "golang.org/x/image/bmp"
	_ "golang.org/x/image/webp"
)

const (
	customWallpaperPrefix       = "file:"
	customWallpaperMaxBytes     = int64(20 << 20)
	customWallpaperMaxDimension = 8192
	customWallpaperMaxPixels    = int64(16_000_000)
	customWallpaperMaxPathBytes = 1024
	customWallpaperMaxPathDepth = 64
)

// WallpaperError carries a stable, non-sensitive code for HTTP/UI handling.
type WallpaperError struct {
	Code  string
	cause error
}

func (e *WallpaperError) Error() string {
	if e == nil || e.Code == "" {
		return "wallpaper_error"
	}
	return e.Code
}

func (e *WallpaperError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.cause
}

// CustomWallpaper contains the exact bytes validated by ReadCustomWallpaper.
// Callers serving the image must write these bytes rather than reopening Path.
type CustomWallpaper struct {
	Path     string
	Bytes    []byte
	MIMEType string
	Width    int
	Height   int
}

type customWallpaperRoot struct {
	roots []*os.Root
	file  *os.File
}

func (h *customWallpaperRoot) close() {
	if h == nil {
		return
	}
	if h.file != nil {
		_ = h.file.Close()
	}
	for i := len(h.roots) - 1; i >= 0; i-- {
		_ = h.roots[i].Close()
	}
}

func wallpaperError(code string, cause error) error {
	return &WallpaperError{Code: code, cause: cause}
}

func customWallpaperPath(raw string) (string, error) {
	if raw == "" || len(raw) > customWallpaperMaxPathBytes || strings.ContainsAny(raw, "\\:\x00") {
		return "", wallpaperError("wallpaper_invalid_path", nil)
	}
	if strings.HasPrefix(raw, "/") || filepath.IsAbs(raw) {
		return "", wallpaperError("wallpaper_invalid_path", nil)
	}
	parts := strings.Split(raw, "/")
	if len(parts) > customWallpaperMaxPathDepth {
		return "", wallpaperError("wallpaper_invalid_path", nil)
	}
	for _, part := range parts {
		if part == "" || part == "." || part == ".." {
			return "", wallpaperError("wallpaper_invalid_path", nil)
		}
	}
	return strings.Join(parts, "/"), nil
}

func customWallpaperFormat(path string) (format, mimeType string, ok bool) {
	base := path
	if idx := strings.LastIndexByte(base, '/'); idx >= 0 {
		base = base[idx+1:]
	}
	ext := strings.ToLower(filepath.Ext(base))
	switch ext {
	case ".png":
		return "png", "image/png", true
	case ".jpg", ".jpeg":
		return "jpeg", "image/jpeg", true
	case ".webp":
		return "webp", "image/webp", true
	case ".bmp":
		return "bmp", "image/bmp", true
	default:
		return "", "", false
	}
}

func customWallpaperDimensionsAllowed(width, height int) bool {
	if width <= 0 || height <= 0 || width > customWallpaperMaxDimension || height > customWallpaperMaxDimension {
		return false
	}
	return int64(width)*int64(height) <= customWallpaperMaxPixels
}

func openCustomWallpaperRoot(workspace, relative string) (*customWallpaperRoot, error) {
	root, err := os.OpenRoot(workspace)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, wallpaperError("wallpaper_not_found", err)
		}
		return nil, wallpaperError("wallpaper_read_failed", err)
	}
	handle := &customWallpaperRoot{roots: []*os.Root{root}}
	fail := func(err error) (*customWallpaperRoot, error) {
		handle.close()
		return nil, err
	}
	parts := strings.Split(relative, "/")
	parent := root
	for _, part := range parts[:len(parts)-1] {
		before, err := parent.Lstat(filepath.FromSlash(part))
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				return fail(wallpaperError("wallpaper_not_found", err))
			}
			return fail(wallpaperError("wallpaper_read_failed", err))
		}
		if before.Mode()&os.ModeSymlink != 0 || !before.IsDir() {
			return fail(wallpaperError("wallpaper_invalid_path", nil))
		}
		child, err := parent.OpenRoot(filepath.FromSlash(part))
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				return fail(wallpaperError("wallpaper_not_found", err))
			}
			return fail(wallpaperError("wallpaper_read_failed", err))
		}
		handle.roots = append(handle.roots, child)
		opened, statErr := child.Stat(".")
		after, lstatErr := parent.Lstat(filepath.FromSlash(part))
		if statErr != nil || lstatErr != nil || after.Mode()&os.ModeSymlink != 0 || !os.SameFile(before, opened) || !os.SameFile(before, after) {
			if errors.Is(lstatErr, os.ErrNotExist) {
				return fail(wallpaperError("wallpaper_not_found", lstatErr))
			}
			return fail(wallpaperError("wallpaper_changed", firstWallpaperError(statErr, lstatErr)))
		}
		parent = child
	}
	name := filepath.FromSlash(parts[len(parts)-1])
	before, err := parent.Lstat(name)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return fail(wallpaperError("wallpaper_not_found", err))
		}
		return fail(wallpaperError("wallpaper_read_failed", err))
	}
	if before.Mode()&os.ModeSymlink != 0 || !before.Mode().IsRegular() {
		return fail(wallpaperError("wallpaper_invalid_path", nil))
	}
	file, err := parent.OpenFile(name, os.O_RDONLY, 0)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return fail(wallpaperError("wallpaper_not_found", err))
		}
		return fail(wallpaperError("wallpaper_read_failed", err))
	}
	handle.file = file
	opened, statErr := file.Stat()
	after, lstatErr := parent.Lstat(name)
	if statErr != nil || lstatErr != nil || after.Mode()&os.ModeSymlink != 0 || !opened.Mode().IsRegular() || !os.SameFile(before, opened) || !os.SameFile(before, after) {
		if errors.Is(lstatErr, os.ErrNotExist) {
			return fail(wallpaperError("wallpaper_not_found", lstatErr))
		}
		return fail(wallpaperError("wallpaper_changed", firstWallpaperError(statErr, lstatErr)))
	}
	return handle, nil
}

func firstWallpaperError(first, second error) error {
	if first != nil {
		return first
	}
	return second
}

func readCustomWallpaperBytes(ctx context.Context, file *os.File, size int64) ([]byte, error) {
	if size < 0 || size > customWallpaperMaxBytes {
		return nil, wallpaperError("wallpaper_too_large", nil)
	}
	data := make([]byte, 0, int(size))
	chunk := make([]byte, 32*1024)
	for {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		count, err := file.Read(chunk)
		if count > 0 {
			if int64(len(data))+int64(count) > customWallpaperMaxBytes {
				return nil, wallpaperError("wallpaper_too_large", nil)
			}
			data = append(data, chunk[:count]...)
		}
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return nil, wallpaperError("wallpaper_read_failed", err)
		}
		if count == 0 {
			return nil, wallpaperError("wallpaper_read_failed", io.ErrNoProgress)
		}
	}
	return data, nil
}

// ReadCustomWallpaper opens and fully validates an image from the desktop workspace.
// The returned bytes are the exact validated snapshot and can be served without reopening
// the workspace path.
func (s *Service) ReadCustomWallpaper(ctx context.Context, rawPath string) (CustomWallpaper, error) {
	if err := s.ensureReady(ctx); err != nil {
		return CustomWallpaper{}, err
	}
	relative, err := customWallpaperPath(rawPath)
	if err != nil {
		return CustomWallpaper{}, err
	}
	wantFormat, mimeType, ok := customWallpaperFormat(relative)
	if !ok {
		return CustomWallpaper{}, wallpaperError("wallpaper_unsupported_format", nil)
	}
	root, err := openCustomWallpaperRoot(s.Config().WorkspaceDir, relative)
	if err != nil {
		return CustomWallpaper{}, err
	}
	defer root.close()
	info, err := root.file.Stat()
	if err != nil {
		return CustomWallpaper{}, wallpaperError("wallpaper_read_failed", err)
	}
	if !info.Mode().IsRegular() {
		return CustomWallpaper{}, wallpaperError("wallpaper_invalid_path", nil)
	}
	if info.Size() > customWallpaperMaxBytes {
		return CustomWallpaper{}, wallpaperError("wallpaper_too_large", nil)
	}
	data, err := readCustomWallpaperBytes(ctx, root.file, info.Size())
	if err != nil {
		return CustomWallpaper{}, err
	}
	if err := ctx.Err(); err != nil {
		return CustomWallpaper{}, err
	}
	config, format, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return CustomWallpaper{}, wallpaperError("wallpaper_invalid_image", err)
	}
	if format != wantFormat {
		return CustomWallpaper{}, wallpaperError("wallpaper_format_mismatch", nil)
	}
	if config.Width <= 0 || config.Height <= 0 || config.Width > customWallpaperMaxDimension || config.Height > customWallpaperMaxDimension {
		return CustomWallpaper{}, wallpaperError("wallpaper_dimensions_exceeded", nil)
	}
	if !customWallpaperDimensionsAllowed(config.Width, config.Height) {
		return CustomWallpaper{}, wallpaperError("wallpaper_pixel_limit", nil)
	}
	decoded, decodedFormat, err := image.Decode(bytes.NewReader(data))
	if err != nil || decodedFormat != wantFormat {
		return CustomWallpaper{}, wallpaperError("wallpaper_invalid_image", err)
	}
	bounds := decoded.Bounds()
	if bounds.Dx() != config.Width || bounds.Dy() != config.Height {
		return CustomWallpaper{}, wallpaperError("wallpaper_invalid_image", fmt.Errorf("decoded image dimensions changed"))
	}
	if err := ctx.Err(); err != nil {
		return CustomWallpaper{}, err
	}
	return CustomWallpaper{Path: relative, Bytes: data, MIMEType: mimeType, Width: config.Width, Height: config.Height}, nil
}

// ValidateWallpaperValue validates changed custom wallpaper settings before persistence.
// Built-in wallpaper IDs are validated by the existing setting-definition path.
func (s *Service) ValidateWallpaperValue(ctx context.Context, value string) error {
	if !strings.HasPrefix(value, customWallpaperPrefix) {
		return nil
	}
	_, err := s.ReadCustomWallpaper(ctx, strings.TrimPrefix(value, customWallpaperPrefix))
	return err
}
