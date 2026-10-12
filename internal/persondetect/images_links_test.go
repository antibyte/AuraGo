package persondetect

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"testing"
	"time"
)

// syntheticInfo is a fs.FileInfo with a chosen mode, to classify what Lstat
// reports on platforms or accounts that cannot create the real thing.
type syntheticInfo struct{ mode fs.FileMode }

func (s syntheticInfo) Name() string       { return "x" }
func (s syntheticInfo) Size() int64        { return 0 }
func (s syntheticInfo) Mode() fs.FileMode  { return s.mode }
func (s syntheticInfo) ModTime() time.Time { return time.Time{} }
func (s syntheticInfo) IsDir() bool        { return s.mode.IsDir() }
func (s syntheticInfo) Sys() any           { return nil }

func TestCheckElementClassifiesWhatLstatReports(t *testing.T) {
	const (
		dir      = fs.ModeDir | 0o750
		file     = fs.FileMode(0o640)
		symlink  = fs.ModeSymlink | 0o777
		junction = fs.ModeIrregular | 0o666 // Go reports a Windows junction like this: no ModeDir
		// placeholder directories of OneDrive or tiering stores are reparse points
		// that Go reports as directories carrying ModeIrregular.
		placeholderDir = fs.ModeDir | fs.ModeIrregular | 0o777
	)
	cases := []struct {
		name    string
		mode    fs.FileMode
		wantDir bool
		ok      bool
		message string
	}{
		{"directory", dir, true, true, ""},
		{"placeholder directory", placeholderDir, true, true, ""},
		{"symlink as directory", symlink, true, false, "link or not a directory"},
		{"junction as directory", junction, true, false, "link or not a directory"},
		{"symlink flagged as directory", fs.ModeSymlink | fs.ModeDir, true, false, "link or not a directory"},
		{"file as directory", file, true, false, "link or not a directory"},

		{"regular file", file, false, true, ""},
		{"irregular file", junction, false, false, "not a regular file"},
		{"placeholder file", fs.ModeIrregular | 0o666, false, false, "not a regular file"},
		{"symlink as file", symlink, false, false, "not a regular file"},
		{"directory as file", dir, false, false, "not a regular file"},
		{"socket as file", fs.ModeSocket | 0o666, false, false, "not a regular file"},
	}
	for _, c := range cases {
		err := checkElement("some/path", syntheticInfo{c.mode}, c.wantDir)
		if c.ok != (err == nil) {
			t.Errorf("%s: err = %v, want ok=%v", c.name, err, c.ok)
			continue
		}
		if err != nil && !strings.Contains(err.Error(), c.message) {
			t.Errorf("%s: err = %q, want it to say %q", c.name, err, c.message)
		}
	}
}

func TestRetryableFrameErrorKnowsTheLostRaces(t *testing.T) {
	notExist := &os.PathError{Op: "open", Path: "x", Err: syscall.ENOENT}
	denied := &os.PathError{Op: "mkdir", Path: "x", Err: errorAccessDenied}
	for _, goos := range []string{"windows", "linux", "darwin"} {
		for _, creating := range []bool{true, false} {
			if !retryableFrameError(notExist, creating, goos) {
				t.Errorf("not-exist (%s, creating=%v) must be retried", goos, creating)
			}
			if retryableFrameError(&os.PathError{Op: "write", Err: syscall.ENOSPC}, creating, goos) {
				t.Errorf("disk full (%s, creating=%v) must not be retried", goos, creating)
			}
		}
	}
	if !retryableFrameError(denied, true, "windows") {
		t.Error("access denied while creating a directory on Windows (a directory pending deletion) must be retried")
	}
	if retryableFrameError(denied, false, "windows") {
		t.Error("access denied on the final file must not be retried")
	}
	if retryableFrameError(denied, true, "linux") || retryableFrameError(denied, true, "darwin") {
		t.Error("errno 5 outside Windows is EIO and must not be retried")
	}
	if !retryableFrameError(errors.Join(errors.New("context"), denied), true, "windows") {
		t.Error("wrapped access denied must be recognised")
	}
}

func setFrameMkdir(t *testing.T, fn func(name string, perm os.FileMode) error) {
	t.Helper()
	previous := frameMkdir
	t.Cleanup(func() { frameMkdir = previous })
	frameMkdir = fn
}

func TestWriteImageAtomicRetriesADirectoryPendingDeletionOnWindows(t *testing.T) {
	root := t.TempDir()
	calls := 0
	setFrameMkdir(t, func(name string, perm os.FileMode) error {
		calls++
		if calls == 1 {
			// Windows answers a mkdir below a directory whose deletion is pending with ERROR_ACCESS_DENIED.
			return &os.PathError{Op: "mkdir", Path: name, Err: errorAccessDenied}
		}
		return os.Mkdir(name, perm)
	})
	err := writeImageAtomic(root, "2026/10/11/cam/1_1.jpg", []byte("x"))
	if runtime.GOOS == "windows" {
		if err != nil {
			t.Fatalf("the single retry did not rescue the write: %v", err)
		}
		if _, err := os.Stat(filepath.Join(root, "2026", "10", "11", "cam", "1_1.jpg")); err != nil {
			t.Fatal(err)
		}
		return
	}
	// errno 5 is EIO elsewhere: a real I/O error, never a race.
	if !errors.Is(err, errorAccessDenied) || calls != 1 {
		t.Fatalf("err = %v after %d mkdir calls, want the error after 1", err, calls)
	}
}

func TestWriteImageAtomicDoesNotRetryAccessDeniedOnTheFinalFile(t *testing.T) {
	root := t.TempDir()
	renames := 0
	setFrameHooks(t, nil, func(from, to string) error {
		renames++
		return &os.LinkError{Op: "rename", Old: from, New: to, Err: errorAccessDenied}
	})
	err := writeImageAtomic(root, "2026/10/11/cam/1_1.jpg", []byte("x"))
	if !errors.Is(err, errorAccessDenied) || renames != 1 {
		t.Fatalf("err = %v after %d renames, want the access error after exactly 1", err, renames)
	}
	assertNoFrameTemporaries(t, root)
}
