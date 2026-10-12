package persondetect

import (
	"bytes"
	"errors"
	"fmt"
	"image"
	"image/jpeg"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"

	"golang.org/x/image/draw"
)

const (
	frameFileMode = 0o640 // stored frames and thumbnails: no world access, like the database
	frameTempGlob = ".frame-*.tmp"

	// Windows system error numbers for a full disk. They are only meaningful on
	// Windows; on other systems the same numbers are unrelated errors
	// (Linux: EHOSTDOWN and ENOTEMPTY).
	errorHandleDiskFull syscall.Errno = 39  // ERROR_HANDLE_DISK_FULL
	errorDiskFull       syscall.Errno = 112 // ERROR_DISK_FULL
)

// frameDirectoryReady runs once the directory of a frame exists and before its
// temporary file is created. Tests replace it to remove that directory the way
// EventStore.removeFile does when retention empties it concurrently.
var frameDirectoryReady = func(dir string) {}

// frameRename renames the temporary file of a frame into place; tests replace it
// to simulate a failing rename.
var frameRename = os.Rename

// frameRelPaths returns the slash-separated paths of a frame and its thumbnail
// below the detections root: YYYY/MM/DD/<camera>/<event>_<seq>.jpg (UTC date).
func frameRelPaths(at time.Time, cameraID string, eventID int64, seq int) (string, string) {
	base := path.Join(at.UTC().Format("2006/01/02"), safePathSegment(cameraID), strconv.FormatInt(eventID, 10)+"_"+strconv.Itoa(seq))
	return base + ".jpg", base + "_thumb.jpg"
}

// safePathSegment keeps [a-z0-9_-], lowercases A-Z and replaces everything else.
func safePathSegment(s string) string {
	var b strings.Builder
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9', r == '-', r == '_':
			b.WriteRune(r)
		case r >= 'A' && r <= 'Z':
			b.WriteRune(r + ('a' - 'A'))
		default:
			b.WriteByte('_')
		}
	}
	out := b.String()
	if out == "" {
		return "camera"
	}
	if len(out) > 64 {
		out = out[:64]
	}
	return out
}

// makeThumbnail scales a JPEG to ThumbnailWidth pixels wide (never up) and encodes
// it at ThumbnailQuality. Frames beyond the size limits of Preprocess are refused
// before the decoder allocates for them.
func makeThumbnail(data []byte) ([]byte, error) {
	config, err := jpeg.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("decode frame: %w", err)
	}
	if err := validateFrameSize(config.Width, config.Height); err != nil {
		return nil, fmt.Errorf("decode frame: %w", err)
	}
	src, err := jpeg.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("decode frame: %w", err)
	}
	bounds := src.Bounds()
	w, h := bounds.Dx(), bounds.Dy()
	if w <= 0 || h <= 0 {
		return nil, errors.New("decode frame: empty image")
	}
	tw, th := w, h
	if w > ThumbnailWidth {
		tw = ThumbnailWidth
		th = (h*ThumbnailWidth + w/2) / w
		if th < 1 {
			th = 1
		}
	}
	dst := image.NewRGBA(image.Rect(0, 0, tw, th))
	draw.BiLinear.Scale(dst, dst.Bounds(), src, bounds, draw.Src, nil)
	var out bytes.Buffer
	if err := jpeg.Encode(&out, dst, &jpeg.Options{Quality: ThumbnailQuality}); err != nil {
		return nil, fmt.Errorf("encode thumbnail: %w", err)
	}
	return out.Bytes(), nil
}

// writeImageAtomic writes data below root through a temporary file and a rename:
// files 0640, directories 0750.
//
// EventStore.removeFile deletes the empty directories above a deleted image, and
// retention calls it without the tracker lock, so it can remove the directory of
// a frame between the MkdirAll and the CreateTemp (or the rename) of this write.
// That shows up as a not-exist error; the write is then repeated once, which
// creates the directory again. Any other error, and a second loss, is returned.
func writeImageAtomic(root, rel string, data []byte) error {
	target := filepath.Join(root, filepath.FromSlash(rel))
	if !pathInside(root, target) {
		return fmt.Errorf("path %q escapes the detections root", rel)
	}
	err := writeImageOnce(root, target, data)
	if errors.Is(err, fs.ErrNotExist) {
		err = writeImageOnce(root, target, data)
	}
	return err
}

// writeImageOnce is one attempt of writeImageAtomic.
func writeImageOnce(root, target string, data []byte) error {
	dir := filepath.Dir(target)
	if err := os.MkdirAll(dir, detectionsRootMode); err != nil {
		return err
	}
	// The root may be a link to another disk; a link inside the tree must not
	// lead out of it.
	if dir != filepath.Clean(root) {
		if err := requireNoLinksBelow(root, dir, false); err != nil {
			return err
		}
	}
	frameDirectoryReady(dir)
	tmp, err := os.CreateTemp(dir, frameTempGlob)
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmpName)
		return err
	}
	if err := tmp.Chmod(frameFileMode); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmpName)
		return err
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmpName)
		return err
	}
	if err := frameRename(tmpName, target); err != nil {
		_ = os.Remove(tmpName)
		return err
	}
	return nil
}

// requireNoLinksBelow checks that target lies below root and that every element
// between them is a real directory (the last one a regular file when wantFile
// is set): no symbolic link and no other reparse point may redirect a path out of
// the tree. A Windows junction is os.ModeIrregular to Go, so that bit counts as
// a link too. The frame tree is created by this package and never contains
// links, which makes any link in it foreign.
//
// root itself is the trusted base and may be a link: relocating the images to
// another disk is a supported setup. It is deliberately not resolved with
// filepath.EvalSymlinks, which since Go 1.23 does not follow a Windows junction
// and fails for every path that runs through one, so a junction root would
// make all frames unreadable.
func requireNoLinksBelow(root, target string, wantFile bool) error {
	if !pathInside(root, target) {
		return fmt.Errorf("path %q escapes the detections root", target)
	}
	rel, err := filepath.Rel(filepath.Clean(root), filepath.Clean(target))
	if err != nil {
		return err
	}
	current := filepath.Clean(root)
	parts := strings.Split(rel, string(filepath.Separator))
	for i, part := range parts {
		current = filepath.Join(current, part)
		info, err := os.Lstat(current)
		if err != nil {
			return err
		}
		if info.Mode()&(os.ModeSymlink|os.ModeIrregular) != 0 {
			return fmt.Errorf("path %q passes through a link inside the detections root", target)
		}
		switch last := i == len(parts)-1; {
		case last && wantFile && !info.Mode().IsRegular():
			return fmt.Errorf("path %q is not a regular file", target)
		case (!last || !wantFile) && !info.IsDir():
			return fmt.Errorf("path %q is not a directory", current)
		}
	}
	return nil
}

// writeFramePair stores a frame exactly as received plus its thumbnail and returns
// the bytes written.
func writeFramePair(root, rel, thumbRel string, data []byte) (int64, error) {
	thumb, err := makeThumbnail(data)
	if err != nil {
		return 0, err
	}
	if err := writeImageAtomic(root, rel, data); err != nil {
		return 0, err
	}
	if err := writeImageAtomic(root, thumbRel, thumb); err != nil {
		_ = os.Remove(filepath.Join(root, filepath.FromSlash(rel)))
		return 0, err
	}
	return int64(len(data) + len(thumb)), nil
}

// resolveInside returns the absolute path of a regular file below root. The
// path may not pass through a link inside the tree, so it cannot leave root; root
// itself may be a link to another disk and is followed.
func resolveInside(root, rel string) (string, error) {
	target := filepath.Join(root, filepath.FromSlash(rel))
	if err := requireNoLinksBelow(root, target, true); err != nil {
		return "", fmt.Errorf("path %q: %w", rel, err)
	}
	return target, nil
}

// isNoSpace reports whether err means the disk is full: ENOSPC or EDQUOT on
// Unix systems, ERROR_DISK_FULL or ERROR_HANDLE_DISK_FULL on Windows. The error
// numbers are decisive (the system messages are localized); the English
// messages are a fallback for errors that lost their number.
func isNoSpace(err error) bool {
	if err == nil {
		return false
	}
	var errno syscall.Errno
	if errors.As(err, &errno) {
		if errno == syscall.ENOSPC || errno == syscall.EDQUOT {
			return true
		}
		if runtime.GOOS == "windows" && (errno == errorDiskFull || errno == errorHandleDiskFull) {
			return true
		}
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "no space left") ||
		strings.Contains(msg, "not enough space on the disk") ||
		strings.Contains(msg, "disk quota exceeded")
}
