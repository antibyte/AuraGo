package desktop

import (
	"context"
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
