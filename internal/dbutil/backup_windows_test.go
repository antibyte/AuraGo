package dbutil

import (
	"golang.org/x/sys/windows"
	"testing"
)

func assertBackupPrivate(t *testing.T, path string) {
	t.Helper()
	sd, err := windows.GetNamedSecurityInfo(path, windows.SE_FILE_OBJECT, windows.DACL_SECURITY_INFORMATION)
	if err != nil {
		t.Fatal(err)
	}
	user, err := windows.GetCurrentProcessToken().GetTokenUser()
	if err != nil {
		t.Fatal(err)
	}
	want := "D:P(A;;FA;;;" + user.User.Sid.String() + ")"
	if sd.String() != want && sd.String() != "D:PAI"+want[3:] {
		t.Fatalf("backup ACL = %s, want %s", sd.String(), want)
	}
}
