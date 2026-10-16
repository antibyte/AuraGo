package smbclient

import (
	"os"
	"testing"
	"time"
)

func TestValidateCoordinatesRequiresConfiguredShareAndCredentials(t *testing.T) {
	tests := []struct {
		name string
		cfg  Config
		want bool
	}{
		{name: "complete named user", cfg: Config{Host: "nas.lan", Share: "library", Username: "reader", Domain: "LAB", Password: "secret"}, want: false},
		{name: "guest fallback forbidden", cfg: Config{Host: "nas.lan", Share: "library", Password: "secret"}, want: true},
		{name: "empty password forbidden", cfg: Config{Host: "nas.lan", Share: "library", Username: "reader"}, want: true},
		{name: "share required", cfg: Config{Host: "nas.lan", Username: "reader", Password: "secret"}, want: true},
		{name: "host port forbidden", cfg: Config{Host: "nas.lan:445", Share: "library", Username: "reader", Password: "secret"}, want: true},
		{name: "host path forbidden", cfg: Config{Host: "nas.lan/library", Share: "library", Username: "reader", Password: "secret"}, want: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := validateCoordinates(test.cfg)
			if (err != nil) != test.want {
				t.Fatalf("validateCoordinates() error = %v, want error %v", err, test.want)
			}
		})
	}
}

func TestCleanPathRejectsTraversalAndNormalizesSharePaths(t *testing.T) {
	for _, input := range []string{"../secret.epub", "folder/../../secret.epub", `/absolute/book.epub`, `folder\\book.epub`, "C:/book.epub", "a\x00b"} {
		if _, err := cleanPath(input, false); err == nil {
			t.Errorf("cleanPath(%q) accepted unsafe path", input)
		}
	}
	got, err := cleanPath("books/fiction/../book.epub", false)
	if err != nil || got != "books/book.epub" {
		t.Fatalf("cleanPath() = %q, %v; want books/book.epub", got, err)
	}
	if got := smbPath("books/fiction/book.epub"); got != `books\fiction\book.epub` {
		t.Fatalf("smbPath() = %q", got)
	}
	if got, err := cleanPath(".", true); err != nil || got != "" {
		t.Fatalf("cleanPath root = %q, %v", got, err)
	}
}

func TestSMBEntryCarriesSafeMetadataOnly(t *testing.T) {
	entry := fromFileInfo("folder/book.epub", fakeFileInfo{name: "book.epub", size: 42, modified: time.Unix(100, 0)})
	if entry.Path != "folder/book.epub" || entry.Name != "book.epub" || entry.Size != 42 || entry.IsDir {
		t.Fatalf("fromFileInfo() = %#v", entry)
	}
}

type fakeFileInfo struct {
	name     string
	size     int64
	modified time.Time
}

func (f fakeFileInfo) Name() string       { return f.name }
func (f fakeFileInfo) Size() int64        { return f.size }
func (fakeFileInfo) Mode() os.FileMode    { return 0o600 }
func (f fakeFileInfo) ModTime() time.Time { return f.modified }
func (fakeFileInfo) IsDir() bool          { return false }
func (fakeFileInfo) Sys() any             { return nil }
