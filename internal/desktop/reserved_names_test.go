package desktop

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestIsWindowsReservedDeviceName(t *testing.T) {
	reserved := []string{
		"CON", "con", "Con.png", "CON.", "CON ", "CON .png", "CON..", "con.tar.gz",
		"PRN", "AUX", "aux.json", "NUL", "NUL.txt", "nul. .txt",
		"COM0", "COM1", "com1.png", "COM9.ora", "LPT0", "LPT1.ora", "lpt9 .png",
		"COM¹", "COM².png", "lpt³.txt", "CONIN$", "conout$.log", "COM1:stream",
	}
	for _, name := range reserved {
		if !isWindowsReservedDeviceName(name) {
			t.Errorf("isWindowsReservedDeviceName(%q) = false, want true", name)
		}
	}
	allowed := []string{
		"", "console.png", "com10.png", "LPT11.ora", "nullable.txt", "auxiliary", "prnt.png",
		"COM", "LPT", "COMA.png", "COM1x.png", "COM⁴.png", "conin.txt", ".con", "x.CON", "my con.png",
	}
	for _, name := range allowed {
		if isWindowsReservedDeviceName(name) {
			t.Errorf("isWindowsReservedDeviceName(%q) = true, want false", name)
		}
	}
}

func TestWorkspaceWritesRejectReservedDeviceNames(t *testing.T) {
	svc := testService(t)
	ctx := context.Background()
	if err := svc.WriteFileBytes(ctx, "source.txt", []byte("data"), SourceUser); err != nil {
		t.Fatal(err)
	}
	expectRejected := func(op, path string, err error) {
		t.Helper()
		if err == nil || !strings.Contains(err.Error(), "reserved device name") {
			t.Errorf("%s %q: err = %v, want reserved device name error", op, path, err)
		}
	}
	for _, path := range []string{
		"COM1.png", "LPT1.ora", "CON.png", "NUL.txt", "aux.json",
		"Com1 .png", "lpt1.", "Pictures/PRN.png", "Pictures/AUX/notes.txt", `Documents\con\readme.md`,
	} {
		expectRejected("write", path, svc.WriteFileBytes(ctx, path, []byte("data"), SourceUser))
	}
	expectRejected("mkdir", "Pictures/COM2", svc.CreateDirectory(ctx, "Pictures/COM2", SourceUser))
	expectRejected("copy", "LPT2.txt", svc.CopyPath(ctx, "source.txt", "LPT2.txt", SourceUser))
	expectRejected("move", "nul/source.txt", svc.MovePath(ctx, "source.txt", "nul/source.txt", SourceUser))

	for _, path := range []string{"console.png", "com10.png", "nullable.txt", "Pictures/auxiliary/notes.txt"} {
		if err := svc.WriteFileBytes(ctx, path, []byte("data"), SourceUser); err != nil {
			t.Errorf("write %q: %v", path, err)
		}
	}
	if err := svc.MovePath(ctx, "source.txt", "Documents/console.txt", SourceUser); err != nil {
		t.Errorf("move to look-alike: %v", err)
	}
}

// setDevicePlatform overrides the GOOS the device-name rule applies for.
func setDevicePlatform(t *testing.T, goos string) {
	t.Helper()
	previous := desktopDevicePlatform
	desktopDevicePlatform = goos
	t.Cleanup(func() { desktopDevicePlatform = previous })
}

func TestDeviceNamesWindowsRefusesExistingEntries(t *testing.T) {
	svc := testService(t)
	ctx := context.Background()
	if runtime.GOOS != "windows" {
		// A Windows host cannot hold such a file; elsewhere it exists so the refusal is not just "missing".
		if err := os.WriteFile(filepath.Join(svc.Config().WorkspaceDir, "aux.json"), []byte("{}"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	setDevicePlatform(t, "windows")
	if _, _, err := svc.ReadFileBytes(ctx, "aux.json"); err == nil || !strings.Contains(err.Error(), "reserved device name") {
		t.Errorf("read aux.json on windows: err = %v, want reserved device name error", err)
	}
}

func TestDeviceNamesLinuxOnlyRefusesCreationAndDestinations(t *testing.T) {
	svc := testService(t)
	ctx := context.Background()
	root := svc.Config().WorkspaceDir
	// A Windows host cannot hold entries named after devices, so the existing-entry checks only run elsewhere.
	existing := runtime.GOOS != "windows"
	if err := os.WriteFile(filepath.Join(root, "source.txt"), []byte("data"), 0o600); err != nil {
		t.Fatal(err)
	}
	if existing {
		if err := os.WriteFile(filepath.Join(root, "aux.json"), []byte("old"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	setDevicePlatform(t, "linux")
	expectRejected := func(op string, err error) {
		t.Helper()
		if err == nil || !strings.Contains(err.Error(), "reserved device name") {
			t.Errorf("%s: err = %v, want reserved device name error", op, err)
		}
	}

	if existing {
		if data, _, err := svc.ReadFileBytes(ctx, "aux.json"); err != nil || string(data) != "old" {
			t.Errorf("read existing aux.json = %q, %v; want old, nil", data, err)
		}
		if err := svc.WriteFileBytes(ctx, "aux.json", []byte("new"), SourceUser); err != nil {
			t.Errorf("write existing aux.json: %v", err)
		}
		// An existing entry is still no valid destination.
		expectRejected("rename onto existing aux.json", svc.MovePath(ctx, "source.txt", "aux.json", SourceUser))
	}
	expectRejected("create con.txt", svc.WriteFileBytes(ctx, "con.txt", []byte("x"), SourceUser))
	expectRejected("create dir", svc.CreateDirectory(ctx, "Pictures/COM2", SourceUser))
	expectRejected("rename to con.txt", svc.MovePath(ctx, "source.txt", "con.txt", SourceUser))
	expectRejected("copy to con.txt", svc.CopyPath(ctx, "source.txt", "con.txt", SourceUser))
}
