package bookshelfconvert

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestConversionFailsClosedWithoutRuntime(t *testing.T) {
	if ok, _ := Capability(""); ok {
		t.Fatal("unconfigured runtime enabled")
	}
	if err := Convert(context.Background(), "", "source.docx", "output.epub"); !errors.Is(err, ErrUnavailable) {
		t.Fatal(err)
	}
	if _, err := Version(context.Background(), ""); !errors.Is(err, ErrUnavailable) {
		t.Fatal(err)
	}
}

func TestConverterVersionOutputIsBounded(t *testing.T) {
	var output versionOutput
	if _, err := output.Write([]byte(strings.Repeat("a", 4096))); err != nil {
		t.Fatal(err)
	}
	if _, err := output.Write([]byte("b")); err == nil || output.Len() != 4096 {
		t.Fatal("unbounded converter output")
	}
}

func TestConversionSandboxHasNoHostDataOrNetwork(t *testing.T) {
	args := sandboxArgs("/opt/calibre/ebook-convert", "/private/job/input.docx", "/private/job/output", ".docx")
	joined := strings.Join(args, "\n")
	for _, required := range []string{"--unshare-all", "--die-with-parent", "--new-session", "--clearenv", "--cap-drop\nALL", "--ro-bind\n/private/job/input.docx\n/input/book.docx", "--as=1073741824", "--cpu=120", "--fsize=268435456"} {
		if !strings.Contains(joined, required) {
			t.Errorf("missing %s", required)
		}
	}
	for _, forbidden := range []string{"--share-net", "--ro-bind\n/\n/", "--bind\n/home", "--bind\n/tmp", "--unshare-user-try"} {
		if strings.Contains(joined, forbidden) {
			t.Errorf("unsafe argument: %s", forbidden)
		}
	}
	if args[len(args)-3] != "/opt/calibre/ebook-convert" || args[len(args)-1] != "/output/book.epub" {
		t.Fatal("converter invocation changed")
	}
}
