package memory

import (
	"testing"
	"unicode/utf8"
)

func TestTruncateUTF8BytesPreservesCodePoints(t *testing.T) {
	value := "界" + string(make([]byte, 20))
	got := truncateUTF8Bytes(value, 4)
	if !utf8.ValidString(got) {
		t.Fatalf("invalid UTF-8: %q", got)
	}
	if len(got) > 4 {
		t.Fatalf("len = %d, want <= 4", len(got))
	}
}