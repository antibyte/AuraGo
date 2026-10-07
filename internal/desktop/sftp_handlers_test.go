package desktop

import (
	"strings"
	"testing"
)

func TestNormalizeSFTPRemotePathTreatsClientRootAsRemoteHome(t *testing.T) {
	t.Parallel()

	cases := map[string]string{
		"/":                    ".",
		`\\`:                   ".",
		"/Documents":           "Documents",
		"/Documents/file.md":   "Documents/file.md",
		"Documents/file.md":    "Documents/file.md",
		"Documents/x":          "Documents/x",
		"/home/user/Documents": "home/user/Documents",
		`Documents\notes\a.md`: "Documents/notes/a.md",
	}
	for raw, want := range cases {
		got, err := normalizeSFTPRemotePath(raw)
		if err != nil {
			t.Fatalf("normalizeSFTPRemotePath(%q): %v", raw, err)
		}
		if got != want {
			t.Fatalf("normalizeSFTPRemotePath(%q) = %q, want %q", raw, got, want)
		}
	}
}

func TestNormalizeSFTPRemotePathRejectsTraversalAndSensitiveAbsolutePaths(t *testing.T) {
	t.Parallel()

	for _, raw := range []string{
		"../.ssh/authorized_keys",
		"/../etc/passwd",
		"/etc/shadow",
		"/root/.ssh/id_rsa",
		"~/.ssh/config",
		"Documents/\x00secret",
	} {
		if got, err := normalizeSFTPRemotePath(raw); err == nil {
			t.Fatalf("normalizeSFTPRemotePath(%q) = %q, want error", raw, got)
		}
	}
}

func TestNormalizeSFTPRemotePathRejectsWindowsDriveAndUNCPaths(t *testing.T) {
	t.Parallel()

	for _, raw := range []string{
		"C:/Windows/System32",
		`C:\Users\x`,
		"c:",
		"/C:/Windows",
		"//server/share",
		`\\server\share`,
		`\\server`,
	} {
		got, err := normalizeSFTPRemotePath(raw)
		if err == nil {
			t.Fatalf("normalizeSFTPRemotePath(%q) = %q, want error", raw, got)
		}
		if !strings.Contains(err.Error(), "Windows or UNC") {
			t.Fatalf("normalizeSFTPRemotePath(%q) error = %v, want the drive/UNC rejection", raw, err)
		}
	}
}
