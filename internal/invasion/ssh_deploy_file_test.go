package invasion

import (
	"bytes"
	"encoding/base64"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestSSHDeployFilePublishesPrivateAtomicFiles(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("remote bash behavior is verified on Linux")
	}
	target := filepath.Join(t.TempDir(), "private file's data")
	payload := []byte("binary fixture\x00\xff\n$(must-not-run)\n")
	run := func(script string) error {
		cmd := exec.Command("bash", "-s")
		cmd.Stdin = strings.NewReader(script)
		return cmd.Run()
	}
	if err := run(sshDeployFileScript(target, payload)); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(target)
	if err != nil || !bytes.Equal(got, payload) {
		t.Fatal("deployment bytes changed")
	}
	info, err := os.Stat(target)
	if err != nil || info.Mode().Perm() != 0600 {
		t.Fatal("deployment file is not private")
	}
	broken := strings.Replace(sshDeployFileScript(target, payload), base64.StdEncoding.EncodeToString(payload), "%%%broken%%%", 1)
	if err := run(broken); err == nil {
		t.Fatal("invalid transfer was published")
	}
	got, err = os.ReadFile(target)
	if err != nil || !bytes.Equal(got, payload) {
		t.Fatal("failed transfer replaced last valid file")
	}
	leftovers, _ := filepath.Glob(target + ".tmp.*")
	if len(leftovers) != 0 {
		t.Fatal("failed transfer left temporary secret files")
	}
}
