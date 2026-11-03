package desktop

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"encoding/binary"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestArchiveKindDetectsExtensionsAndMagic(t *testing.T) {
	if kindFromName("notes.tar.gz") != archiveTarGz || kindFromName("notes.tgz") != archiveTarGz {
		t.Fatal("tar.gz name")
	}
	if kindFromName("disk.img") != archiveProbe || kindFromName("root.ext4") != archiveExt {
		t.Fatal("image name")
	}
	header := make([]byte, 0x500)
	binary.LittleEndian.PutUint16(header[0x438:], 0xEF53)
	if kindFromMagic(header) != archiveExt {
		t.Fatal("ext magic")
	}
	copy(header[:4], []byte("hsqs"))
	if kindFromMagic(header[:4]) != archiveSquashfs {
		t.Fatal("squashfs magic")
	}
	if got := kindFromMagic([]byte{0x37, 0x7A, 0xBC, 0xAF, 0x27, 0x1C}); got != archive7z {
		t.Fatalf("7z magic = %s", got)
	}
	kind, err := detectArchiveKind("plain.gz", bytes.NewReader(gzipBytes(t, []byte("ustar-not-really"))))
	if err != nil || kind != archiveGz {
		t.Fatalf("bare gzip = %s %v", kind, err)
	}
	var tarBuf bytes.Buffer
	tw := tar.NewWriter(&tarBuf)
	if err := tw.WriteHeader(&tar.Header{Name: "a.txt", Mode: 0o644, Size: 1, ModTime: time.Unix(0, 0)}); err != nil {
		t.Fatal(err)
	}
	if _, err := tw.Write([]byte("a")); err != nil {
		t.Fatal(err)
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	kind, err = detectArchiveKind("notes.gz", bytes.NewReader(gzipBytes(t, tarBuf.Bytes())))
	if err != nil || kind != archiveTarGz {
		t.Fatalf("gzip tar sniff = %s %v", kind, err)
	}
}

func TestTarGzRoundTripListExtractAndPreview(t *testing.T) {
	svc := testService(t)
	ctx := context.Background()
	if err := svc.WriteFileBytes(ctx, "docs/readme.txt", []byte("hello tar"), SourceUser); err != nil {
		t.Fatal(err)
	}
	if err := svc.CreateArchive(ctx, []string{"docs/readme.txt"}, "notes.tar.gz", SourceUser); err != nil {
		t.Fatal(err)
	}
	listing, err := svc.ListArchive(ctx, "notes.tar.gz")
	if err != nil {
		t.Fatal(err)
	}
	if listing.Format != "tar.gz" || len(listing.Entries) != 1 || listing.Entries[0].Name != "readme.txt" {
		t.Fatalf("listing = %#v", listing)
	}
	entry, err := svc.ReadArchiveEntry(ctx, "notes.tar.gz", "readme.txt")
	if err != nil || string(entry.Data) != "hello tar" {
		t.Fatalf("preview = %q %v", entry.Data, err)
	}
	if err := svc.ExtractArchive(ctx, "notes.tar.gz", "out", SourceUser); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(filepath.Join(svc.Config().WorkspaceDir, "out", "readme.txt"))
	if err != nil || string(got) != "hello tar" {
		t.Fatalf("extracted %q %v", got, err)
	}
}

func TestTarExtractRejectsSlipAndSymlinkBeforeWrite(t *testing.T) {
	svc := testService(t)
	ctx := context.Background()
	if err := svc.WriteFileBytes(ctx, "target/keep.txt", []byte("original"), SourceUser); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"../target/keep.txt", "link"} {
		var buf bytes.Buffer
		writer := tar.NewWriter(&buf)
		hdr := &tar.Header{Name: "ok.txt", Mode: 0o644, Size: 2, ModTime: time.Unix(0, 0)}
		if err := writer.WriteHeader(hdr); err != nil {
			t.Fatal(err)
		}
		if _, err := writer.Write([]byte("ok")); err != nil {
			t.Fatal(err)
		}
		if name == "link" {
			if err := writer.WriteHeader(&tar.Header{Name: "link", Mode: 0o777, Typeflag: tar.TypeSymlink, Linkname: "ok.txt", ModTime: time.Unix(0, 0)}); err != nil {
				t.Fatal(err)
			}
		} else {
			if err := writer.WriteHeader(&tar.Header{Name: name, Mode: 0o644, Size: 3, ModTime: time.Unix(0, 0)}); err != nil {
				t.Fatal(err)
			}
			if _, err := writer.Write([]byte("bad")); err != nil {
				t.Fatal(err)
			}
		}
		if err := writer.Close(); err != nil {
			t.Fatal(err)
		}
		if err := svc.WriteFileBytes(ctx, "bad.tar", buf.Bytes(), SourceUser); err != nil {
			t.Fatal(err)
		}
		if err := svc.ExtractArchive(ctx, "bad.tar", "target", SourceUser); err == nil {
			t.Fatalf("accepted %s", name)
		}
		content, err := os.ReadFile(filepath.Join(svc.Config().WorkspaceDir, "target", "keep.txt"))
		if err != nil || string(content) != "original" {
			t.Fatalf("%s modified destination %q %v", name, content, err)
		}
		if _, err := os.Stat(filepath.Join(svc.Config().WorkspaceDir, "target", "ok.txt")); !os.IsNotExist(err) {
			t.Fatalf("%s published a file before rejection: %v", name, err)
		}
	}
}

func TestCreateArchiveRejectsUncreatableExtension(t *testing.T) {
	svc := testService(t)
	ctx := context.Background()
	if err := svc.WriteFileBytes(ctx, "a.txt", []byte("a"), SourceUser); err != nil {
		t.Fatal(err)
	}
	err := svc.CreateArchive(ctx, []string{"a.txt"}, "out.7z", SourceUser)
	if err == nil || !strings.Contains(err.Error(), "not supported") {
		t.Fatal(err)
	}
}

func TestExtImageReportsMissingHostTool(t *testing.T) {
	svc := testService(t)
	ctx := withArchiveTools(context.Background(), archiveTools{})
	data := make([]byte, 0x500)
	binary.LittleEndian.PutUint16(data[0x438:], 0xEF53)
	if err := svc.WriteFileBytes(context.Background(), "disk.img", data, SourceUser); err != nil {
		t.Fatal(err)
	}
	_, err := svc.ListArchive(ctx, "disk.img")
	if err == nil || !strings.Contains(err.Error(), "debugfs") {
		t.Fatal(err)
	}
}

func TestParse7zTechnicalList(t *testing.T) {
	text := "Path = notes.7z\nType = 7z\n\nPath = readme.txt\nSize = 5\nPacked Size = 7\nModified = 2020-01-02 03:04:05\nAttributes = A_ -rw-r--r--\nFolder = -\n\nPath = link\nSize = 0\nAttributes = _ lrwxrwxrwx\nFolder = -\n"
	members, err := parse7zList(text)
	if err != nil || len(members) != 2 || members[0].Name != "readme.txt" || members[0].Size != 5 || members[1].Link != true {
		t.Fatalf("%#v %v", members, err)
	}
	if _, err := parse7zList("Path = secret\nEncrypted = +\n"); !errors.Is(err, errArchiveEncrypted) {
		t.Fatal(err)
	}
}

func TestCreateTarXzRoundTrip(t *testing.T) {
	svc := testService(t)
	ctx := context.Background()
	if err := svc.WriteFileBytes(ctx, "note.txt", []byte("xz"), SourceUser); err != nil {
		t.Fatal(err)
	}
	if err := svc.CreateArchive(ctx, []string{"note.txt"}, "note.tar.xz", SourceUser); err != nil {
		t.Fatal(err)
	}
	listing, err := svc.ListArchive(ctx, "note.tar.xz")
	if err != nil || listing.Format != "tar.xz" || len(listing.Entries) != 1 {
		t.Fatalf("%#v %v", listing, err)
	}
}

func TestTarHardLinkCopiesEarlierFile(t *testing.T) {
	svc := testService(t)
	ctx := context.Background()
	var buf bytes.Buffer
	writer := tar.NewWriter(&buf)
	if err := writer.WriteHeader(&tar.Header{Name: "hello.txt", Mode: 0o644, Size: 5, ModTime: time.Unix(0, 0)}); err != nil {
		t.Fatal(err)
	}
	if _, err := writer.Write([]byte("hello")); err != nil {
		t.Fatal(err)
	}
	if err := writer.WriteHeader(&tar.Header{Name: "copy.txt", Mode: 0o644, Typeflag: tar.TypeLink, Linkname: "hello.txt", ModTime: time.Unix(0, 0)}); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	if err := svc.WriteFileBytes(ctx, "links.tar", buf.Bytes(), SourceUser); err != nil {
		t.Fatal(err)
	}
	if err := svc.ExtractArchive(ctx, "links.tar", "out", SourceUser); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"hello.txt", "copy.txt"} {
		got, err := os.ReadFile(filepath.Join(svc.Config().WorkspaceDir, "out", name))
		if err != nil || string(got) != "hello" {
			t.Fatalf("%s %q %v", name, got, err)
		}
	}
}

func TestTarHardLinkToMissingTargetIsRejected(t *testing.T) {
	svc := testService(t)
	ctx := context.Background()
	var buf bytes.Buffer
	writer := tar.NewWriter(&buf)
	if err := writer.WriteHeader(&tar.Header{Name: "copy.txt", Mode: 0o644, Typeflag: tar.TypeLink, Linkname: "missing.txt", ModTime: time.Unix(0, 0)}); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	if err := svc.WriteFileBytes(ctx, "links.tar", buf.Bytes(), SourceUser); err != nil {
		t.Fatal(err)
	}
	if err := svc.ExtractArchive(ctx, "links.tar", "out", SourceUser); err == nil {
		t.Fatal("accepted a dangling hard link")
	}
	if _, err := os.Stat(filepath.Join(svc.Config().WorkspaceDir, "out", "copy.txt")); !os.IsNotExist(err) {
		t.Fatalf("published dangling hard link: %v", err)
	}
}

func TestTarDuplicateDirectoryExtracts(t *testing.T) {
	svc := testService(t)
	ctx := context.Background()
	var buf bytes.Buffer
	writer := tar.NewWriter(&buf)
	for _, name := range []string{"sub", "sub/"} {
		if err := writer.WriteHeader(&tar.Header{Name: name, Mode: 0o755, Typeflag: tar.TypeDir, ModTime: time.Unix(0, 0)}); err != nil {
			t.Fatal(err)
		}
	}
	if err := writer.WriteHeader(&tar.Header{Name: "sub/a.txt", Mode: 0o644, Size: 1, ModTime: time.Unix(0, 0)}); err != nil {
		t.Fatal(err)
	}
	if _, err := writer.Write([]byte("a")); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	if err := svc.WriteFileBytes(ctx, "dirs.tar", buf.Bytes(), SourceUser); err != nil {
		t.Fatal(err)
	}
	if err := svc.ExtractArchive(ctx, "dirs.tar", "out", SourceUser); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(filepath.Join(svc.Config().WorkspaceDir, "out", "sub", "a.txt"))
	if err != nil || string(got) != "a" {
		t.Fatalf("%q %v", got, err)
	}
}

func TestTarPreviewStopsBeforeLaterMember(t *testing.T) {
	var buf bytes.Buffer
	writer := tar.NewWriter(&buf)
	if err := writer.WriteHeader(&tar.Header{Name: "a.txt", Mode: 0o644, Size: 1, ModTime: time.Unix(0, 0)}); err != nil {
		t.Fatal(err)
	}
	if _, err := writer.Write([]byte("a")); err != nil {
		t.Fatal(err)
	}
	if err := writer.WriteHeader(&tar.Header{Name: "b.txt", Mode: 0o644, Size: archiveExpansionLimit, ModTime: time.Unix(0, 0)}); err != nil {
		t.Fatal(err)
	}
	entry, err := readNativePreview(context.Background(), bytes.NewReader(buf.Bytes()), archiveTar, "x.tar", "a.txt", 50)
	if err != nil || string(entry.Data) != "a" {
		t.Fatalf("%q %v", entry.Data, err)
	}
}

func TestMembersFromTreeSkipsOutsideHardLink(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "ok.txt"), []byte("ok"), 0o644); err != nil {
		t.Fatal(err)
	}
	outside := filepath.Join(t.TempDir(), "secret")
	if err := os.WriteFile(outside, []byte("secret"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Link(outside, filepath.Join(root, "leak.txt")); err != nil {
		t.Skip(err)
	}
	members, err := membersFromTree(root, true)
	if err != nil {
		t.Fatal(err)
	}
	var sawOK, sawLeak bool
	for _, member := range members {
		switch member.Name {
		case "ok.txt":
			sawOK = true
			data, readErr := member.Read(10)
			if readErr != nil || string(data) != "ok" {
				t.Fatalf("ok %q %v", data, readErr)
			}
		case "leak.txt":
			sawLeak = true
			if !member.Link || member.Read != nil {
				t.Fatalf("outside hard link was readable: %#v", member)
			}
		default:
			t.Fatalf("unexpected member %s", member.Name)
		}
	}
	if !sawOK || !sawLeak {
		t.Fatalf("members = %#v", members)
	}
}

func TestMembersFromTreeDoesNotFollowDirectoryLink(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	if err := os.WriteFile(filepath.Join(outside, "secret.txt"), []byte("secret"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(root, "linked")); err != nil {
		t.Skip(err)
	}
	members, err := membersFromTree(root, true)
	if err != nil {
		t.Fatal(err)
	}
	if len(members) != 1 || members[0].Name != "linked" || !members[0].Link || members[0].Read != nil {
		t.Fatalf("%#v", members)
	}
}

func TestRunArchiveCommandAcceptsWarningExit(t *testing.T) {
	name := "sh"
	args := []string{"-c", "exit 1"}
	if runtime.GOOS == "windows" {
		name = "cmd"
		args = []string{"/c", "exit", "1"}
	}
	if _, err := runArchiveCommand(context.Background(), "", 64, false, true, name, args...); err != nil {
		t.Fatal(err)
	}
	if _, err := runArchiveCommand(context.Background(), "", 64, false, false, name, args...); err == nil {
		t.Fatal("expected a non-warning failure")
	}
}

func TestClassifyToolErrorDoesNotTreatPathAsEncrypted(t *testing.T) {
	if errors.Is(classifyToolError(`cannot open C:\encrypted\disk.img`, nil), errArchiveEncrypted) {
		t.Fatal("path looked encrypted")
	}
	if !errors.Is(classifyToolError("Cannot open encrypted archive. Wrong password?", nil), errArchiveEncrypted) {
		t.Fatal("encrypted archive was accepted")
	}
}

func gzipBytes(t *testing.T, payload []byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	writer := gzip.NewWriter(&buf)
	if _, err := writer.Write(payload); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}
