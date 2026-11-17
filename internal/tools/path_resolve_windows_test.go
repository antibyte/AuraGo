//go:build windows

package tools

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestSecureResolveFinalPathResolvesWindowsJunctions(t *testing.T) {
	base := t.TempDir()
	workspace, outside := filepath.Join(base, "workspace"), filepath.Join(base, "outside")
	for _, dir := range []string{workspace, outside} {
		if err := os.Mkdir(dir, 0700); err != nil {
			t.Fatal(err)
		}
	}
	marker := filepath.Join(outside, "marker.txt")
	if err := os.WriteFile(marker, []byte("outside"), 0600); err != nil {
		t.Fatal(err)
	}
	junction := filepath.Join(workspace, "junction")
	if output, err := exec.Command("cmd.exe", "/c", "mklink", "/J", junction, outside).CombinedOutput(); err != nil {
		t.Fatalf("create NTFS junction: %v: %s", err, output)
	}
	t.Cleanup(func() { _ = os.Remove(junction) })
	for _, suffix := range []string{"", "marker.txt", "new/folder/file.txt"} {
		requested := filepath.Join(junction, suffix)
		got, err := secureResolveFinalPath(requested)
		if err != nil || !strings.EqualFold(got, filepath.Join(outside, suffix)) {
			t.Fatalf("%s resolved to %q: %v", requested, got, err)
		}
		if dockerBindWithinWorkspace(workspace, requested) {
			t.Fatalf("junction escape accepted as workspace bind: %s", requested)
		}
	}
	if err := os.Remove(marker); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(outside); err != nil {
		t.Fatal(err)
	}
	if _, err := secureResolveFinalPath(junction); err == nil {
		t.Fatal("broken junction silently accepted")
	}
}
