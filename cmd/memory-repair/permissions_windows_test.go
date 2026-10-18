package main

import (
	"aurago/internal/memory"
	"golang.org/x/sys/windows"
	"io/fs"
	"path/filepath"
	"strings"
	"testing"
)

func TestOfflineEvidenceUsesOwnerOnlyWindowsDACL(t *testing.T) {
	f := newRepairFixture(t)
	if err := memory.PrepareDocumentCorpus(f.vectors); err != nil {
		t.Fatal(err)
	}
	root := filepath.Join(f.root, "reports", "backup")
	if err := backupStores(t.Context(), f.db, f.vectors, root); err != nil {
		t.Fatal(err)
	}
	rehearsal := filepath.Join(f.root, "reports", "rehearsal")
	if err := copyTree(t.Context(), root, rehearsal); err != nil {
		t.Fatal(err)
	}
	if err := writeJSON(filepath.Join(f.root, "reports", "preview.json"), map[string]string{"status": "fixture"}); err != nil {
		t.Fatal(err)
	}
	user, err := windows.GetCurrentProcessToken().GetTokenUser()
	if err != nil {
		t.Fatal(err)
	}
	err = filepath.WalkDir(filepath.Join(f.root, "reports"), func(path string, _ fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		sd, err := windows.GetNamedSecurityInfo(path, windows.SE_FILE_OBJECT, windows.DACL_SECURITY_INFORMATION)
		if err != nil {
			return err
		}
		acl, _, err := sd.DACL()
		if err != nil {
			return err
		}
		if acl == nil || acl.AceCount != 1 || !strings.Contains(sd.String(), "D:P") || !strings.Contains(sd.String(), user.User.Sid.String()) {
			t.Errorf("artifact does not have protected owner-only DACL: %s", path)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
