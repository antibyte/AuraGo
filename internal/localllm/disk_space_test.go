package localllm

import (
	"reflect"
	"testing"

	"aurago/internal/fileutil"
)

func TestAvailableDiskBytesUsesSharedFreeSpaceProbe(t *testing.T) {
	if reflect.ValueOf(availableDiskBytes).Pointer() != reflect.ValueOf(fileutil.FreeDiskBytes).Pointer() {
		t.Fatal("availableDiskBytes must default to fileutil.FreeDiskBytes")
	}
}
