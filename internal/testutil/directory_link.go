package testutil

import (
	"os"
	"os/exec"
	"runtime"
	"testing"

	"aurago/internal/sandbox"
)

// DirectoryLink creates a symlink or Windows junction between synthetic fixture directories.
func DirectoryLink(t testing.TB, link, target string) {
	t.Helper()
	if runtime.GOOS != "windows" {
		if err := os.Symlink(target, link); err != nil {
			t.Fatal(err)
		}
		return
	}
	cmd := exec.Command("powershell.exe", "-NoProfile", "-NonInteractive", "-Command", `$ErrorActionPreference = 'Stop'; New-Item -ItemType Junction -Path $env:AURAGO_TEST_LINK -Target $env:AURAGO_TEST_TARGET | Out-Null`)
	cmd.Env = append(sandbox.FilterEnv(os.Environ()), "AURAGO_TEST_LINK="+link, "AURAGO_TEST_TARGET="+target)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("create fixture junction: %v: %s", err, out)
	}
}
