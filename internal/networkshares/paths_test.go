package networkshares

import (
	"os"
	"path/filepath"
	"testing"
)

func TestValidateConfiguredRootsRejectsRelativeEmptyAndDuplicate(t *testing.T) {
	root := t.TempDir()
	cases := [][]string{
		{""},
		{"relative"},
		{root, root},
	}
	for _, roots := range cases {
		if err := ValidateConfiguredRoots(roots); err == nil {
			t.Fatalf("ValidateConfiguredRoots(%q) succeeded", roots)
		}
	}
}

func TestCanonicalAllowedPathRejectsSymlinkEscape(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	link := filepath.Join(root, "escape")
	if err := os.Symlink(outside, link); err != nil {
		t.Skipf("symlink creation is unavailable: %v", err)
	}
	if _, err := canonicalAllowedPath(link, []string{root}); ErrorCode(err) != ErrorOutsideRoot {
		t.Fatalf("canonicalAllowedPath error = %v, code=%q", err, ErrorCode(err))
	}
}

func TestRootStatusesKeepMissingRootConfiguredButUnavailable(t *testing.T) {
	root := filepath.Join(t.TempDir(), "missing")
	statuses := rootStatuses([]string{root})
	if len(statuses) != 1 || statuses[0].Available || statuses[0].Path != root || statuses[0].Reason == "" {
		t.Fatalf("root statuses = %+v", statuses)
	}
}

func TestCanonicalClientRejectsWildcardsAndCanonicalizesCIDR(t *testing.T) {
	if _, ok := CanonicalClient("*"); ok {
		t.Fatal("wildcard client unexpectedly accepted")
	}
	if got, ok := CanonicalClient("192.0.2.42/24"); !ok || got != "192.0.2.0/24" {
		t.Fatalf("CanonicalClient = %q, %v", got, ok)
	}
}
