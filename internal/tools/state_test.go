package tools

import "testing"

func TestResolveBusyFilePathRejectsInvalidPath(t *testing.T) {
	if _, err := resolveBusyFilePath("\x00busy.lock"); err == nil {
		t.Fatal("expected invalid busy path to return error")
	}
}
