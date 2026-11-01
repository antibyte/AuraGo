package media

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
	"unicode/utf8"
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

// File names come from a remote party. On Windows ":" starts an alternate data stream and
// "*?" cannot be created; trailing dots and spaces are dropped by the file system; very long
// names fail; U+202E reverses the displayed extension.
func TestSanitizeFileNameMakesSenderNamesSafeEverywhere(t *testing.T) {
	rlo := string(rune(0x202e))
	zeroWidth := string(rune(0x200b))
	long := strings.Repeat("a", 250) + ".pdf"
	multibyte := strings.Repeat("é", 200) + ".docx"
	bigExt := "a." + strings.Repeat("x", 300)

	for _, tc := range []struct {
		name, in, want string
	}{
		{"plain", "report.pdf", "report.pdf"},
		{"spaces stay for the naming policy", "my report.pdf", "my report.pdf"},
		{"path traversal", "../../evil.pdf", "evil.pdf"},
		{"windows path", `C:\dir\sub\file.txt`, "file.txt"},
		{"alternate data stream", "report.pdf:Zone.Identifier", "report.pdf_Zone.Identifier"},
		{"wildcards", "x*y?.pdf", "x_y_.pdf"},
		{"every reserved character", `a<b>c:d"e|f.txt`, "a_b_c_d_e_f.txt"},
		{"control characters", "a\tb\x00c" + string(rune(0x85)) + "d.txt", "a_b_c_d.txt"},
		{"right-to-left override", "invoice" + rlo + "fdp.exe", "invoice_fdp.exe"},
		{"zero-width space", "a" + zeroWidth + "b.txt", "a_b.txt"},
		{"invalid UTF-8", "bad\xffname.txt", "bad_name.txt"},
		{"trailing dots and spaces", "report.pdf. . ", "report.pdf"},
		{"only dots", "...", ""},
		{"dot dot", "..", ""},
		{"empty", "", ""},
		{"only a separator", "dir/", ""},
		{"long ASCII keeps the extension", long, strings.Repeat("a", 96) + ".pdf"},
		{"long multibyte cuts on a rune boundary", multibyte, strings.Repeat("é", 47) + ".docx"},
		{"huge extension is cut with the name", bigExt, "a." + strings.Repeat("x", 98)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := SanitizeFileName(tc.in)
			if got != tc.want {
				t.Fatalf("SanitizeFileName(%q) = %q, want %q", tc.in, got, tc.want)
			}
			if len(got) > maxSanitizedNameBytes || !utf8.ValidString(got) {
				t.Fatalf("result %q is %d bytes, valid UTF-8 = %v", got, len(got), utf8.ValidString(got))
			}
			if again := SanitizeFileName(got); again != got {
				t.Fatalf("not idempotent: %q then %q", got, again)
			}
		})
	}

	// The stored name carries no stream separator, so one visible file holds the content.
	dest := t.TempDir()
	now := time.Date(2026, 10, 31, 12, 0, 0, 0, time.UTC)
	path, err := saveAttachmentReader(context.Background(), strings.NewReader("data"), "report.pdf:Zone.Identifier", dest, now)
	if err != nil {
		t.Fatalf("saveAttachmentReader: %v", err)
	}
	if got := filepath.Base(path); got != "20261031_120000_report.pdf_Zone.Identifier" {
		t.Fatalf("stored name = %q", got)
	}
	if entries, _ := os.ReadDir(dest); len(entries) != 1 {
		t.Fatalf("%d files in the destination, want 1", len(entries))
	}
}
