package media

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestSaveAttachmentReaderSanitizesAndNeverOverwrites(t *testing.T) {
	dest := t.TempDir()
	now := time.Date(2026, 10, 31, 12, 0, 0, 0, time.UTC)

	first, err := saveAttachmentReader(context.Background(), strings.NewReader("first"), "../../evil name.pdf", dest, now)
	if err != nil {
		t.Fatalf("saveAttachmentReader: %v", err)
	}
	if filepath.Dir(first) != dest {
		t.Fatalf("the file left the destination: %q", first)
	}
	name := filepath.Base(first)
	if name != "20261031_120000_evil_name.pdf" || strings.Contains(name, "..") {
		t.Fatalf("stored name = %q, want 20261031_120000_evil_name.pdf", name)
	}

	// The same name in the same second must not replace the first file.
	second, err := saveAttachmentReader(context.Background(), strings.NewReader("second"), "../../evil name.pdf", dest, now)
	if err != nil {
		t.Fatalf("second save: %v", err)
	}
	if got := filepath.Base(second); got != "20261031_120000_evil_name_1.pdf" {
		t.Fatalf("second stored name = %q, want a _1 suffix before the extension", got)
	}
	for path, want := range map[string]string{first: "first", second: "second"} {
		if data, err := os.ReadFile(path); err != nil || string(data) != want {
			t.Fatalf("%s holds %q (%v), want %q", filepath.Base(path), data, err, want)
		}
	}

	// _1 to _9 are free to use; the next collision fails and leaves the others alone.
	for i := 0; i < 8; i++ {
		if _, err := saveAttachmentReader(context.Background(), strings.NewReader("x"), "../../evil name.pdf", dest, now); err != nil {
			t.Fatalf("save %d: %v", i+3, err)
		}
	}
	if _, err := saveAttachmentReader(context.Background(), strings.NewReader("x"), "../../evil name.pdf", dest, now); err == nil {
		t.Fatal("the eleventh save with one name succeeded")
	}
	if data, _ := os.ReadFile(first); string(data) != "first" {
		t.Fatalf("a failed save changed the first file: %q", data)
	}

	// A name with nothing left falls back to file.bin; the exported entry point works too.
	path, err := SaveAttachmentReaderContext(context.Background(), strings.NewReader("x"), "..", dest)
	if err != nil || !strings.HasSuffix(filepath.Base(path), "_file.bin") {
		t.Fatalf("SaveAttachmentReaderContext(..) = %q, %v, want a _file.bin name", path, err)
	}
}
