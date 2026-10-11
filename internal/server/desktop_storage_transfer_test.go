package server

import "testing"

func TestGogStorageTransferNameValidation(t *testing.T) {
	for _, name := range []string{"", ".", "..", "../outside", "folder/file", "folder\\file", "file\x00name"} {
		if storageTransferNameSafe(name) {
			t.Errorf("accepted unsafe Google child name %q", name)
		}
	}
	for _, name := range []string{"same.txt", "same (2).txt", "München", ".hidden", "a..b"} {
		if !storageTransferNameSafe(name) {
			t.Errorf("rejected safe Google child name %q", name)
		}
	}
}
