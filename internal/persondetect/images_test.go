package persondetect

import (
	"bytes"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"testing"
	"time"
)

func encodeFrameTestJPEG(t *testing.T, w, h int) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, color.RGBA{R: uint8(x), G: uint8(y), B: 90, A: 255})
		}
	}
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: 85}); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// symlinkOrSkip links link to the directory target. Windows accounts without the
// symlink privilege get a directory junction instead (what a relocated store
// usually is there); when neither can be made the test is skipped.
func symlinkOrSkip(t *testing.T, target, link string) {
	t.Helper()
	err := os.Symlink(target, link)
	if err != nil && runtime.GOOS == "windows" {
		if out, jerr := exec.Command("cmd", "/c", "mklink", "/J", link, target).CombinedOutput(); jerr == nil {
			return
		} else {
			err = fmt.Errorf("%w; junction: %v: %s", err, jerr, strings.TrimSpace(string(out)))
		}
	}
	if err != nil {
		t.Skipf("cannot create a link here: %v", err)
	}
}

// setFrameHooks swaps the two write seams and restores them when the test ends.
func setFrameHooks(t *testing.T, ready func(dir string), rename func(from, to string) error) {
	t.Helper()
	previousReady, previousRename := frameDirectoryReady, frameRename
	t.Cleanup(func() { frameDirectoryReady, frameRename = previousReady, previousRename })
	if ready != nil {
		frameDirectoryReady = ready
	}
	if rename != nil {
		frameRename = rename
	}
}

func assertNoFrameTemporaries(t *testing.T, root string) {
	t.Helper()
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err == nil && strings.HasPrefix(d.Name(), ".frame-") {
			t.Errorf("temporary file left behind: %s", p)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestFrameRelPathsUseTheUTCDateAndASafeCameraSegment(t *testing.T) {
	cest := time.FixedZone("CEST", 2*3600)
	rel, thumb := frameRelPaths(time.Date(2026, 10, 12, 1, 30, 0, 0, cest), "front-door", 42, 3)
	if rel != "2026/10/11/front-door/42_3.jpg" || thumb != "2026/10/11/front-door/42_3_thumb.jpg" {
		t.Fatalf("paths = %q %q", rel, thumb)
	}
	rel, _ = frameRelPaths(time.Date(2026, 10, 11, 12, 0, 0, 0, time.UTC), "../Front Door!", 1, 1)
	if rel != "2026/10/11/___front_door_/1_1.jpg" {
		t.Fatalf("unsafe camera id path = %q", rel)
	}
}

func TestMakeThumbnailScalesToWidthWithoutUpscaling(t *testing.T) {
	thumb, err := makeThumbnail(encodeFrameTestJPEG(t, 640, 360))
	if err != nil {
		t.Fatal(err)
	}
	cfg, err := jpeg.DecodeConfig(bytes.NewReader(thumb))
	if err != nil || cfg.Width != ThumbnailWidth || cfg.Height != 180 {
		t.Fatalf("thumbnail = %dx%d, %v", cfg.Width, cfg.Height, err)
	}
	small, err := makeThumbnail(encodeFrameTestJPEG(t, 200, 100))
	if err != nil {
		t.Fatal(err)
	}
	if cfg, _ := jpeg.DecodeConfig(bytes.NewReader(small)); cfg.Width != 200 || cfg.Height != 100 {
		t.Fatalf("small frame thumbnail = %dx%d, want no upscaling", cfg.Width, cfg.Height)
	}
	if _, err := makeThumbnail([]byte("not a jpeg")); err == nil {
		t.Fatal("garbage accepted")
	}
}

func TestMakeThumbnailRefusesFramesTheDetectorWouldRefuse(t *testing.T) {
	// A tiny file whose header claims a frame beyond the shared size limits must
	// fail before the decoder allocates for it.
	wide := encodeFrameTestJPEG(t, maxFrameSide+1, 1)
	if _, err := makeThumbnail(wide); err == nil || !strings.Contains(err.Error(), "not supported") {
		t.Fatalf("oversized frame: err = %v", err)
	}
}

func TestWriteFramePairStoresTheOriginalAtomically(t *testing.T) {
	root := t.TempDir()
	data := encodeFrameTestJPEG(t, 640, 360)
	size, err := writeFramePair(root, "2026/10/11/cam/1_1.jpg", "2026/10/11/cam/1_1_thumb.jpg", data)
	if err != nil {
		t.Fatalf("writeFramePair: %v", err)
	}
	dir := filepath.Join(root, "2026", "10", "11", "cam")
	stored, err := os.ReadFile(filepath.Join(dir, "1_1.jpg"))
	if err != nil || !bytes.Equal(stored, data) {
		t.Fatalf("frame must be stored as received: %v", err)
	}
	thumbInfo, err := os.Stat(filepath.Join(dir, "1_1_thumb.jpg"))
	if err != nil || size != int64(len(data))+thumbInfo.Size() {
		t.Fatalf("size = %d, frame %d + thumb %v (%v)", size, len(data), thumbInfo, err)
	}
	if runtime.GOOS != "windows" {
		for _, name := range []string{"1_1.jpg", "1_1_thumb.jpg"} {
			if info, _ := os.Stat(filepath.Join(dir, name)); info.Mode().Perm() != 0o640 {
				t.Fatalf("%s mode = %v, want 0640", name, info.Mode().Perm())
			}
		}
		// Every directory below the root, never wider than 0750 (a strict umask may narrow it).
		for _, d := range []string{filepath.Join(root, "2026"), filepath.Join(root, "2026", "10"), filepath.Join(root, "2026", "10", "11"), dir} {
			if info, _ := os.Stat(d); info.Mode().Perm()&^0o750 != 0 {
				t.Fatalf("directory %s mode = %v, want at most 0750", d, info.Mode().Perm())
			}
		}
	}
	assertNoFrameTemporaries(t, root)
	if err := writeImageAtomic(root, "../escape.jpg", data); err == nil {
		t.Fatal("a path outside the root was written")
	}
}

func TestWriteFramePairRemovesTheFrameWhenTheThumbnailCannotBeWritten(t *testing.T) {
	root := t.TempDir()
	data := encodeFrameTestJPEG(t, 640, 360)
	setFrameHooks(t, nil, func(from, to string) error {
		if strings.HasSuffix(to, "_thumb.jpg") {
			return &os.LinkError{Op: "rename", Old: from, New: to, Err: syscall.ENOSPC}
		}
		return os.Rename(from, to)
	})
	size, err := writeFramePair(root, "2026/10/11/cam/1_1.jpg", "2026/10/11/cam/1_1_thumb.jpg", data)
	if err == nil || size != 0 {
		t.Fatalf("writeFramePair = %d, %v; want a failure", size, err)
	}
	if !isNoSpace(err) {
		t.Fatalf("the disk-full cause must survive the pair write: %v", err)
	}
	dir := filepath.Join(root, "2026", "10", "11", "cam")
	if entries, _ := os.ReadDir(dir); len(entries) != 0 {
		names := []string{}
		for _, e := range entries {
			names = append(names, e.Name())
		}
		t.Fatalf("a half-written pair was left behind: %v", names)
	}
}

func TestWriteImageAtomicRecreatesADirectoryRetentionRemovedMeanwhile(t *testing.T) {
	st := newTestEventStore(t)
	root := st.Root()
	const first, second = "2026/10/11/cam/1_1.jpg", "2026/10/11/cam/2_1.jpg"
	if err := writeImageAtomic(root, first, []byte("one")); err != nil {
		t.Fatal(err)
	}
	// Retention deletes the last file of an event and, with it, the now empty
	// day directories, exactly between MkdirAll and CreateTemp of the next write.
	calls := 0
	setFrameHooks(t, func(string) {
		calls++
		if calls == 1 {
			st.removeFile(first)
		}
	}, nil)
	if err := writeImageAtomic(root, second, []byte("two")); err != nil {
		t.Fatalf("writeImageAtomic lost against removeFile: %v", err)
	}
	if calls != 2 {
		t.Fatalf("directory prepared %d times, want 2 (one retry)", calls)
	}
	if got, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(second))); err != nil || string(got) != "two" {
		t.Fatalf("second frame = %q, %v", got, err)
	}
	if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(first))); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("first frame should be gone: %v", err)
	}
	assertNoFrameTemporaries(t, root)
}

func TestWriteFramePairSurvivesRetentionRemovingTheCameraDirectory(t *testing.T) {
	st := newTestEventStore(t)
	root := st.Root()
	old, oldThumb := frameRelPaths(time.Date(2026, 10, 11, 8, 0, 0, 0, time.UTC), "cam", 1, 1)
	for _, rel := range []string{old, oldThumb} {
		if err := writeImageAtomic(root, rel, []byte("old")); err != nil {
			t.Fatal(err)
		}
	}
	data := encodeFrameTestJPEG(t, 320, 180)
	calls := 0
	setFrameHooks(t, func(string) {
		calls++
		// The first preparation of each write meets a retention pass that
		// deletes the old event's files and the directories above them.
		if calls == 1 {
			st.removeFile(old)
			st.removeFile(oldThumb)
		}
	}, nil)
	rel, thumbRel := frameRelPaths(time.Date(2026, 10, 11, 9, 0, 0, 0, time.UTC), "cam", 2, 1)
	size, err := writeFramePair(root, rel, thumbRel, data)
	if err != nil || size <= int64(len(data)) {
		t.Fatalf("writeFramePair = %d, %v", size, err)
	}
	for _, name := range []string{rel, thumbRel} {
		if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(name))); err != nil {
			t.Fatalf("%s missing: %v", name, err)
		}
	}
	assertNoFrameTemporaries(t, root)
}

func TestWriteImageAtomicRetriesOnlyOnce(t *testing.T) {
	root := t.TempDir()
	calls := 0
	setFrameHooks(t, func(dir string) {
		calls++
		_ = os.Remove(dir) // every attempt loses the race
	}, nil)
	err := writeImageAtomic(root, "2026/10/11/cam/1_1.jpg", []byte("x"))
	if !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("err = %v, want the not-exist error after the single retry", err)
	}
	if calls != 2 {
		t.Fatalf("directory prepared %d times, want exactly 2", calls)
	}
	assertNoFrameTemporaries(t, root)
}

func TestWriteImageAtomicRetriesARenameThatFindsNothing(t *testing.T) {
	root := t.TempDir()
	renames := 0
	setFrameHooks(t, nil, func(from, to string) error {
		renames++
		if renames == 1 {
			return &os.LinkError{Op: "rename", Old: from, New: to, Err: syscall.ENOENT}
		}
		return os.Rename(from, to)
	})
	if err := writeImageAtomic(root, "2026/10/11/cam/1_1.jpg", []byte("x")); err != nil {
		t.Fatal(err)
	}
	if renames != 2 {
		t.Fatalf("rename attempted %d times, want 2", renames)
	}
	if got, err := os.ReadFile(filepath.Join(root, "2026", "10", "11", "cam", "1_1.jpg")); err != nil || string(got) != "x" {
		t.Fatalf("file = %q, %v", got, err)
	}
	assertNoFrameTemporaries(t, root)
}

func TestWriteImageAtomicDoesNotRetryOtherFailures(t *testing.T) {
	root := t.TempDir()
	renames := 0
	setFrameHooks(t, nil, func(from, to string) error {
		renames++
		return &os.LinkError{Op: "rename", Old: from, New: to, Err: syscall.ENOSPC}
	})
	err := writeImageAtomic(root, "2026/10/11/cam/1_1.jpg", []byte("x"))
	if !isNoSpace(err) || renames != 1 {
		t.Fatalf("err = %v after %d renames, want the disk-full error after one", err, renames)
	}
	assertNoFrameTemporaries(t, root)
}

func TestWriteImageAtomicRefusesADirectorySymlinkedOutsideTheRoot(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "2026", "10", "11"), 0o750); err != nil {
		t.Fatal(err)
	}
	symlinkOrSkip(t, outside, filepath.Join(root, "2026", "10", "11", "cam"))
	if err := writeImageAtomic(root, "2026/10/11/cam/1_1.jpg", []byte("x")); err == nil {
		t.Fatal("a write through a symlinked directory left the root")
	}
	if entries, _ := os.ReadDir(outside); len(entries) != 0 {
		t.Fatalf("something was written outside the root: %v", entries)
	}
}

func TestFramesWorkBelowASymlinkedRoot(t *testing.T) {
	// A detections root that is a link to another disk is a supported setup.
	realRoot := t.TempDir()
	linkRoot := filepath.Join(t.TempDir(), "detections")
	symlinkOrSkip(t, realRoot, linkRoot)

	data := encodeFrameTestJPEG(t, 640, 360)
	rel, thumbRel := frameRelPaths(time.Date(2026, 10, 11, 9, 0, 0, 0, time.UTC), "cam", 7, 1)
	if _, err := writeFramePair(linkRoot, rel, thumbRel, data); err != nil {
		t.Fatalf("writeFramePair below a symlinked root: %v", err)
	}
	if _, err := os.Stat(filepath.Join(realRoot, filepath.FromSlash(rel))); err != nil {
		t.Fatalf("the frame did not land in the link's target: %v", err)
	}
	got, err := resolveInside(linkRoot, rel)
	if err != nil {
		t.Fatalf("resolveInside below a symlinked root: %v", err)
	}
	gotInfo, err1 := os.Stat(got)
	wantInfo, err2 := os.Stat(filepath.Join(realRoot, filepath.FromSlash(rel)))
	if err1 != nil || err2 != nil || !os.SameFile(gotInfo, wantInfo) {
		t.Fatalf("resolveInside = %q is not the stored frame (%v, %v)", got, err1, err2)
	}
	if !pathInside(linkRoot, got) {
		t.Fatalf("resolveInside = %q, want a path below the root %q", got, linkRoot)
	}
	if _, err := resolveInside(linkRoot, thumbRel); err != nil {
		t.Fatalf("thumbnail: %v", err)
	}

	// Links inside the tree may not lead out of the real root. The tree holds
	// none of its own, so even one that stays inside is refused.
	outside := t.TempDir()
	if err := os.WriteFile(filepath.Join(outside, "secret.jpg"), []byte("secret"), 0o600); err != nil {
		t.Fatal(err)
	}
	symlinkOrSkip(t, outside, filepath.Join(realRoot, "leak"))
	if _, err := resolveInside(linkRoot, "leak/secret.jpg"); err == nil {
		t.Fatal("a directory link leading out of the real root was accepted")
	}
	symlinkOrSkip(t, filepath.Join(realRoot, "2026"), filepath.Join(realRoot, "alias"))
	if _, err := resolveInside(linkRoot, "alias/10/11/cam/7_1.jpg"); err == nil {
		t.Fatal("a directory link inside the tree was followed")
	}
	if err := writeImageAtomic(linkRoot, "leak/new.jpg", []byte("x")); err == nil {
		t.Fatal("a write through that directory symlink was accepted")
	}
	if _, err := os.Stat(filepath.Join(outside, "new.jpg")); err == nil {
		t.Fatal("the file was written outside the root")
	}
}

func TestResolveInsideRejectsEscapes(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "a"), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "a", "f.jpg"), []byte("x"), 0o640); err != nil {
		t.Fatal(err)
	}
	got, err := resolveInside(root, "a/f.jpg")
	if err != nil || filepath.Base(got) != "f.jpg" {
		t.Fatalf("resolveInside = %q, %v", got, err)
	}
	outside := filepath.Join(t.TempDir(), "secret.jpg")
	if err := os.WriteFile(outside, []byte("secret"), 0o600); err != nil {
		t.Fatal(err)
	}
	rejected := []string{"../outside.jpg", "a/missing.jpg", "a", "a/../../x.jpg", "", ".", "/", filepath.ToSlash(outside)}
	for _, rel := range rejected {
		if _, err := resolveInside(root, rel); err == nil {
			t.Fatalf("resolveInside(%q) accepted", rel)
		}
	}
	if err := os.Symlink(outside, filepath.Join(root, "a", "link.jpg")); err != nil {
		t.Logf("symlink assertion skipped, cannot create a symlink here: %v", err)
	} else if _, err := resolveInside(root, "a/link.jpg"); err == nil {
		t.Fatal("a symlink pointing outside the root was accepted")
	}
}

func TestIsNoSpace(t *testing.T) {
	for _, err := range []error{
		&os.PathError{Op: "write", Path: "f.jpg", Err: syscall.ENOSPC},
		errors.New("write f.jpg: no space left on device"),
		fmt.Errorf("wrap: %w", errors.New("There is not enough space on the disk.")),
	} {
		if !isNoSpace(err) {
			t.Fatalf("isNoSpace(%v) = false", err)
		}
	}
	for _, err := range []error{nil, os.ErrPermission, errors.New("broken pipe")} {
		if isNoSpace(err) {
			t.Fatalf("isNoSpace(%v) = true", err)
		}
	}
}

func TestIsNoSpaceReadsTheMessageOnlyWhenThereIsNoErrorNumber(t *testing.T) {
	// A phrase in a path must not turn another failure into "disk full".
	for _, err := range []error{
		&os.PathError{Op: "open", Path: "/srv/no space left/detections/x.jpg", Err: syscall.EACCES},
		&os.PathError{Op: "open", Path: `D:\Disk quota exceeded\detections\2026`, Err: syscall.ENOENT},
		fmt.Errorf("store frame: %w", &os.LinkError{Op: "rename", Old: "/not enough space on the disk/a", New: "b", Err: syscall.EPERM}),
	} {
		if isNoSpace(err) {
			t.Errorf("isNoSpace(%v) = true", err)
		}
	}
	// Without an error number the English message is the only hint left.
	for _, err := range []error{
		errors.New("write x.jpg: no space left on device"),
		errors.New("There is not enough space on the disk."),
		errors.New("write x.jpg: Disk quota exceeded"),
	} {
		if !isNoSpace(err) {
			t.Errorf("isNoSpace(%v) = false", err)
		}
	}
}

func TestSafePathSegmentAvoidsReservedWindowsDeviceNames(t *testing.T) {
	// Writing "nul" or "com1" fails on Windows (a device path), so the segment is
	// changed on every platform to keep stored paths portable.
	cases := map[string]string{
		"nul": "nul_", "NUL": "nul_", "Con": "con_", "prn": "prn_", "AUX": "aux_",
		"com1": "com1_", "COM9": "com9_", "lpt0": "lpt0_", "Lpt7": "lpt7_",
		// not reserved: other lengths, other digits, other characters
		"cam": "cam", "null": "null", "console": "console", "com": "com", "com10": "com10",
		"lpt": "lpt", "comx": "comx", "nul1": "nul1", "my-nul": "my-nul", "con_": "con_",
		// the dot becomes an underscore, so the Windows "con.txt" case never occurs
		"con.txt": "con_txt",
	}
	for in, want := range cases {
		if got := safePathSegment(in); got != want {
			t.Errorf("safePathSegment(%q) = %q, want %q", in, got, want)
		}
	}
	rel, thumb := frameRelPaths(time.Date(2026, 10, 11, 9, 0, 0, 0, time.UTC), "nul", 1, 1)
	if rel != "2026/10/11/nul_/1_1.jpg" || thumb != "2026/10/11/nul_/1_1_thumb.jpg" {
		t.Fatalf("paths = %q %q", rel, thumb)
	}
	// And it really can be written on this platform.
	root := t.TempDir()
	if _, err := writeFramePair(root, rel, thumb, encodeFrameTestJPEG(t, 320, 180)); err != nil {
		t.Fatalf("a frame of camera %q cannot be stored: %v", "nul", err)
	}
	if _, err := resolveInside(root, rel); err != nil {
		t.Fatalf("resolveInside: %v", err)
	}
}

func TestSafePathSegmentLimitsLengthAndNeverReturnsEmpty(t *testing.T) {
	if got := safePathSegment(""); got != "camera" {
		t.Fatalf("empty id = %q, want camera", got)
	}
	long := strings.Repeat("x", 300)
	if got := safePathSegment(long); got != strings.Repeat("x", 64) {
		t.Fatalf("long id = %d chars", len(got))
	}
	exact := strings.Repeat("a", 64)
	if got := safePathSegment(exact); got != exact {
		t.Fatalf("64 characters changed: %q", got)
	}
	if got := safePathSegment(strings.Repeat("ä", 100)); got != strings.Repeat("_", 64) {
		t.Fatalf("non-ASCII id = %q", got)
	}
}

func TestWriteImageAtomicCreatesNothingBehindALinkedAncestor(t *testing.T) {
	// MkdirAll would follow the link and build the rest of the path in its
	// target before any check could refuse the write.
	for _, level := range []string{"2026", "2026/10", "2026/10/11", "2026/10/11/cam"} {
		t.Run(strings.ReplaceAll(level, "/", "_"), func(t *testing.T) {
			root, outside := t.TempDir(), t.TempDir()
			link := filepath.Join(root, filepath.FromSlash(level))
			if err := os.MkdirAll(filepath.Dir(link), 0o750); err != nil {
				t.Fatal(err)
			}
			symlinkOrSkip(t, outside, link)
			if err := writeImageAtomic(root, "2026/10/11/cam/1_1.jpg", []byte("x")); err == nil {
				t.Fatal("a write below a linked directory was accepted")
			}
			if tree := listTree(outside); len(tree) != 0 {
				t.Fatalf("something was created behind the link: %v", tree)
			}
			data := encodeFrameTestJPEG(t, 320, 180)
			if _, err := writeFramePair(root, "2026/10/11/cam/2_1.jpg", "2026/10/11/cam/2_1_thumb.jpg", data); err == nil {
				t.Fatal("a frame pair below a linked directory was accepted")
			}
			if tree := listTree(outside); len(tree) != 0 {
				t.Fatalf("a frame pair created something behind the link: %v", tree)
			}
		})
	}
}

func TestRemoveFileDoesNotFollowALinkedParent(t *testing.T) {
	for _, level := range []string{"2026", "2026/10/11/cam"} {
		t.Run(strings.ReplaceAll(level, "/", "_"), func(t *testing.T) {
			st := newTestEventStore(t)
			outside := t.TempDir()
			if err := os.MkdirAll(filepath.Join(outside, "10", "11", "cam"), 0o750); err != nil {
				t.Fatal(err)
			}
			victim := filepath.Join(outside, "10", "11", "cam", "1_1.jpg")
			if err := os.WriteFile(victim, []byte("precious"), 0o640); err != nil {
				t.Fatal(err)
			}
			link := filepath.Join(st.Root(), filepath.FromSlash(level))
			if err := os.MkdirAll(filepath.Dir(link), 0o750); err != nil {
				t.Fatal(err)
			}
			symlinkOrSkip(t, outside, link)
			if level == "2026/10/11/cam" {
				victim = filepath.Join(outside, "1_1.jpg")
				if err := os.WriteFile(victim, []byte("precious"), 0o640); err != nil {
					t.Fatal(err)
				}
			}
			before := listTree(outside)
			st.removeFile("2026/10/11/cam/1_1.jpg")
			if got, err := os.ReadFile(victim); err != nil || string(got) != "precious" {
				t.Fatalf("a file behind a link was deleted: %q, %v", got, err)
			}
			if after := listTree(outside); strings.Join(after, "|") != strings.Join(before, "|") {
				t.Fatalf("the tree behind the link changed:\nbefore %v\nafter  %v", before, after)
			}
			if _, err := os.Lstat(link); err != nil {
				t.Fatalf("the link itself was removed by the prune: %v", err)
			}
		})
	}
}

func TestRemoveFileNeverPrunesALinkEvenWhenItIsTheParent(t *testing.T) {
	st := newTestEventStore(t)
	outside := t.TempDir() // empty: an unguarded prune would remove the link, even rmdir the target
	link := filepath.Join(st.Root(), "2026")
	symlinkOrSkip(t, outside, link)
	st.removeFile("2026/x.jpg")
	if _, err := os.Lstat(link); err != nil {
		t.Fatalf("the link was removed: %v", err)
	}
	if _, err := os.Stat(outside); err != nil {
		t.Fatalf("the link target was removed: %v", err)
	}
}

func TestRemoveFilePrunesEmptyParentsWhetherOrNotTheFileIsStillThere(t *testing.T) {
	st := newTestEventStore(t)
	root := st.Root()
	exists := func(rel string) bool {
		_, err := os.Lstat(filepath.Join(root, filepath.FromSlash(rel)))
		return err == nil
	}
	// the file exists: it goes, and so do the directories it leaves empty
	if err := writeImageAtomic(root, "2026/10/11/cam/1_1.jpg", []byte("x")); err != nil {
		t.Fatal(err)
	}
	st.removeFile("2026/10/11/cam/1_1.jpg")
	if exists("2026") {
		t.Fatal("empty parents of a deleted file survived")
	}
	if !exists(".") {
		t.Fatal("the root was removed")
	}
	// the file is already gone but its directories are empty
	if err := os.MkdirAll(filepath.Join(root, "2026", "10", "11", "cam"), 0o750); err != nil {
		t.Fatal(err)
	}
	st.removeFile("2026/10/11/cam/2_1.jpg")
	if exists("2026") {
		t.Fatal("empty parents of a missing file survived")
	}
	// the directory of the file is gone as well, the ones above it are empty
	if err := os.MkdirAll(filepath.Join(root, "2026", "10"), 0o750); err != nil {
		t.Fatal(err)
	}
	st.removeFile("2026/10/11/cam/3_1.jpg")
	if exists("2026") {
		t.Fatal("empty ancestors of a missing directory survived")
	}
	// a sibling keeps its directories
	if err := writeImageAtomic(root, "2026/10/11/cam/4_1.jpg", []byte("keep")); err != nil {
		t.Fatal(err)
	}
	if err := writeImageAtomic(root, "2026/10/11/cam/5_1.jpg", []byte("go")); err != nil {
		t.Fatal(err)
	}
	st.removeFile("2026/10/11/cam/5_1.jpg")
	if !exists("2026/10/11/cam/4_1.jpg") || exists("2026/10/11/cam/5_1.jpg") {
		t.Fatal("removeFile touched the wrong files")
	}
	// paths that leave the root are ignored
	outside := filepath.Join(t.TempDir(), "victim.jpg")
	if err := os.WriteFile(outside, []byte("x"), 0o640); err != nil {
		t.Fatal(err)
	}
	rel, err := filepath.Rel(root, outside)
	if err == nil {
		st.removeFile(filepath.ToSlash(rel))
	}
	if _, err := os.Stat(outside); err != nil {
		t.Fatalf("a file outside the root was deleted: %v", err)
	}
}

// listTree lists every path below dir (slash separated, sorted by the walk).
func listTree(dir string) []string {
	var out []string
	_ = filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil || p == dir {
			return nil
		}
		rel, _ := filepath.Rel(dir, p)
		out = append(out, filepath.ToSlash(rel))
		return nil
	})
	return out
}

func TestIsNoSpaceRecognisesTheDiskFullCodesOfEachPlatform(t *testing.T) {
	wrapped := func(errno syscall.Errno) error {
		return fmt.Errorf("store frame: %w", &os.LinkError{Op: "rename", Old: "a", New: "b", Err: errno})
	}
	// Linux, macOS and the BSDs: no space left on device, and an exceeded quota.
	for name, errno := range map[string]syscall.Errno{"ENOSPC": syscall.ENOSPC, "EDQUOT": syscall.EDQUOT} {
		if !isNoSpace(wrapped(errno)) {
			t.Errorf("%s (%d) not recognised", name, uintptr(errno))
		}
	}
	// Windows: ERROR_DISK_FULL (112) and ERROR_HANDLE_DISK_FULL (39) are real
	// system error numbers there. Elsewhere the same numbers mean something else
	// (Linux: EHOSTDOWN and ENOTEMPTY) and must not count as a full disk.
	for name, errno := range map[string]syscall.Errno{"ERROR_DISK_FULL": 112, "ERROR_HANDLE_DISK_FULL": 39} {
		got := isNoSpace(wrapped(errno))
		if want := runtime.GOOS == "windows"; got != want {
			t.Errorf("%s (%d): isNoSpace = %v on %s, want %v", name, uintptr(errno), got, runtime.GOOS, want)
		}
	}
	// Other errors that disk code returns every day are not a full disk.
	for name, errno := range map[string]syscall.Errno{"ENOENT": syscall.ENOENT, "EACCES": syscall.EACCES, "EEXIST": syscall.EEXIST} {
		if isNoSpace(wrapped(errno)) {
			t.Errorf("%s counted as a full disk", name)
		}
	}
}
