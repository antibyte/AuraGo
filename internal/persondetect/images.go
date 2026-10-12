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
	"regexp"
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

	// errorAccessDenied is ERROR_ACCESS_DENIED (5), which Windows answers for a
	// mkdir below a directory whose deletion is still pending. Elsewhere errno 5
	// is EIO.
	errorAccessDenied syscall.Errno = 5
)

// reservedDeviceName matches the names Windows reserves for devices (CON, PRN,
// AUX, NUL, COM0-9, LPT0-9). A directory with such a name cannot be created
// there; it is only ever matched against an already lowercased segment.
var reservedDeviceName = regexp.MustCompile(`^(con|prn|aux|nul|com[0-9]|lpt[0-9])$`)

// frameDirectoryReady runs once the directory of a frame exists and before its
// temporary file is created. Tests replace it to remove that directory the way
// EventStore.removeFile does when retention empties it concurrently.
var frameDirectoryReady = func(dir string) {}

// frameRename renames the temporary file of a frame into place; tests replace it
// to simulate a failing rename.
var frameRename = os.Rename

// frameMkdir creates one directory of a frame's path; tests replace it to
// simulate the failure Windows reports for a directory pending deletion.
var frameMkdir = os.Mkdir

// frameRelPaths returns the slash-separated paths of a frame and its thumbnail
// below the detections root: YYYY/MM/DD/<camera>/<event>_<seq>.jpg (UTC date).
func frameRelPaths(at time.Time, cameraID string, eventID int64, seq int) (string, string) {
	base := path.Join(at.UTC().Format("2006/01/02"), safePathSegment(cameraID), strconv.FormatInt(eventID, 10)+"_"+strconv.Itoa(seq))
	return base + ".jpg", base + "_thumb.jpg"
}

// safePathSegment keeps [a-z0-9_-], lowercases A-Z and replaces everything else.
// The result is at most 64 bytes, "camera" for an empty id, and never a reserved
// Windows device name: "nul" becomes "nul_" on every platform, so a stored path
// stays valid when the data directory moves to Windows.
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
	if reservedDeviceName.MatchString(out) {
		out += "_"
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
// There is no fsync, on purpose. Temp file plus rename makes a frame visible
// atomically and keeps a crash from leaving a half-written file under the final
// name; a power loss on a journaling file system such as ext4 can still leave a
// zero-length file, which is acceptable for event images. Two fsyncs per frame
// would cost far more than they are worth on SD-card hosts.
//
// EventStore.removeFile deletes the empty directories above a deleted image, and
// retention calls it without the tracker lock, so it can remove the directory of
// a frame between the creation of the directory and the CreateTemp (or the
// rename) of this write. That shows up as a not-exist error (on Windows, for a
// directory that is still pending deletion, as access denied while creating the
// directory); the write is then repeated once, which creates the directory
// again. Any other error, and a second loss, is returned.
func writeImageAtomic(root, rel string, data []byte) error {
	target := filepath.Join(root, filepath.FromSlash(rel))
	if !pathInside(root, target) {
		return fmt.Errorf("path %q escapes the detections root", rel)
	}
	retry, err := writeImageOnce(root, target, data)
	if err != nil && retry {
		_, err = writeImageOnce(root, target, data)
	}
	return err
}

// writeImageOnce is one attempt of writeImageAtomic; retry tells whether its
// error is the kind a concurrent directory removal causes.
func writeImageOnce(root, target string, data []byte) (retry bool, err error) {
	dir := filepath.Dir(target)
	if err := makeDirsBelow(root, dir); err != nil {
		return retryableFrameError(err, true, runtime.GOOS), err
	}
	frameDirectoryReady(dir)
	tmp, err := os.CreateTemp(dir, frameTempGlob)
	if err != nil {
		return retryableFrameError(err, false, runtime.GOOS), err
	}
	tmpName := tmp.Name()
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmpName)
		return false, err
	}
	if err := tmp.Chmod(frameFileMode); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmpName)
		return false, err
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmpName)
		return false, err
	}
	if err := frameRename(tmpName, target); err != nil {
		_ = os.Remove(tmpName)
		return retryableFrameError(err, false, runtime.GOOS), err
	}
	return false, nil
}

// retryableFrameError reports whether err from a frame write is what losing a
// race against the pruning of empty directories looks like: a missing file or
// directory anywhere, and, on Windows only while the directory is being created,
// access denied. Access denied from the final file (CreateTemp, rename) is a real
// permission problem and is never retried.
func retryableFrameError(err error, creatingDirectory bool, goos string) bool {
	if errors.Is(err, fs.ErrNotExist) {
		return true
	}
	var errno syscall.Errno
	return creatingDirectory && goos == "windows" && errors.As(err, &errno) && errno == errorAccessDenied
}

// makeDirsBelow creates dir and the directories above it below root one element
// at a time, and refuses to continue at the first element that is a link or not
// a directory. os.MkdirAll must not be used here: it follows a link at any
// level, so a link at <root>/2026 would have the rest of the path built behind it
// (outside the root) before any check could refuse the write. root itself is the
// trusted base: it is created when missing and may be a link.
func makeDirsBelow(root, dir string) error {
	root, dir = filepath.Clean(root), filepath.Clean(dir)
	if err := os.MkdirAll(root, detectionsRootMode); err != nil {
		return err
	}
	if dir == root {
		return nil
	}
	if !pathInside(root, dir) {
		return fmt.Errorf("directory %q escapes the detections root", dir)
	}
	rel, err := filepath.Rel(root, dir)
	if err != nil {
		return err
	}
	current := root
	for _, part := range strings.Split(rel, string(filepath.Separator)) {
		current = filepath.Join(current, part)
		if err := frameMkdir(current, detectionsRootMode); err != nil && !errors.Is(err, fs.ErrExist) {
			return err
		}
		info, err := os.Lstat(current)
		if err != nil {
			return err
		}
		if err := checkElement(current, info, true); err != nil {
			return err
		}
	}
	return nil
}

// checkElement judges one existing element (name) below the image root from its Lstat
// result: a directory when wantDir is set, else a regular file.
//
// A directory must not be a symlink and must have the directory bit. Go reports
// symlinks and Windows junctions without ModeDir, so both fail that test, while
// reparse-point directories that are not links (OneDrive placeholders, tiering
// stores) keep ModeDir together with ModeIrregular and are allowed. A file must
// be regular, which excludes every kind of link and reparse point.
func checkElement(name string, info fs.FileInfo, wantDir bool) error {
	if wantDir {
		if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
			return fmt.Errorf("%q is a link or not a directory", name)
		}
		return nil
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("%q is not a regular file", name)
	}
	return nil
}

// requireNoLinksBelow checks that target lies below root and that every element
// between them is a real directory (the last one a regular file when wantFile is
// set): no symbolic link and no junction may redirect a path out of the tree. The
// frame tree is created by this package and never contains links, which makes
// any link in it foreign.
//
// root itself is the trusted base and may be a link: relocating the images to
// another disk is a supported setup. It is deliberately not resolved with
// filepath.EvalSymlinks, which since Go 1.23 does not follow a Windows junction
// and fails for every path that runs through one, so a junction root would
// make all frames unreadable.
//
// Threat model: the root is 0750, owned by the AuraGo user, and the go2rtc
// container mounts it read-only. The walk guards against stray links (restored
// backups, relocated subdirectories, tampered database paths), not against a
// concurrent attacker who could already act as the AuraGo user. A time-of-check
// to time-of-use window remains between the walk and the later open, and is
// accepted. os.Root would be the race-free alternative if that ever matters.
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
		if err := checkElement(current, info, !(wantFile && i == len(parts)-1)); err != nil {
			return err
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
// number is decisive (system messages are localized, and a path or an unrelated
// failure may contain the same words); the English messages are a fallback only
// for errors that carry no error number at all.
func isNoSpace(err error) bool {
	if err == nil {
		return false
	}
	var errno syscall.Errno
	if errors.As(err, &errno) {
		if errno == syscall.ENOSPC || errno == syscall.EDQUOT {
			return true
		}
		return runtime.GOOS == "windows" && (errno == errorDiskFull || errno == errorHandleDiskFull)
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "no space left") ||
		strings.Contains(msg, "not enough space on the disk") ||
		strings.Contains(msg, "disk quota exceeded")
}
