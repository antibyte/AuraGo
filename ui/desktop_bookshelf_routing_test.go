package ui

import (
	"os"
	"strings"
	"testing"
)

func TestDesktopBookshelfAssociations(t *testing.T) {
	read := func(path string) string {
		t.Helper()
		body, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		return string(body)
	}

	const automatic = `\.(epub|mobi|azw|azw3|fb2(?:\.zip)?|cbz|cbr|lit|pdb|lrf)$/i`
	for _, path := range []string{"js/desktop/apps/editor-filemenu.js", "js/desktop/core/spotlight-runtime.js"} {
		source := read(path)
		if !strings.Contains(source, automatic) {
			t.Errorf("%s must automatically open only native Bookshelf formats", path)
		}
		if strings.Contains(source, `\.(epub|pdf`) || strings.Contains(source, `cbr|rtf|docx|odt`) {
			t.Errorf("%s routes document/PDF formats to Bookshelf by default", path)
		}
		if !strings.Contains(source, `associationExtension === 'fb2.zip'`) {
			t.Errorf("%s must keep .fb2.zip defaults separate from generic .zip files", path)
		}
	}

	for _, path := range []string{"js/desktop/core/menus-and-routing.js", "js/desktop/file-manager/actions-operations.js"} {
		source := read(path)
		for _, format := range []string{`fb2(?:\.zip)?`, `txt(?:\.gz)?`, `djvu`, `lit`, `pdb`, `lrf`} {
			if !strings.Contains(source, format) {
				t.Errorf("%s is missing the explicit Bookshelf Open with route for %s", path, format)
			}
		}
	}

	menus := read("js/desktop/core/menus-and-routing.js")
	if !strings.Contains(menus, `? 'fb2.zip' : /\.txt\.gz$/i.test(String(name || '')) ? 'txt.gz' : desktopFileExtension(name)`) {
		t.Error("Open with must keep compound book formats separate from generic ZIP/GZIP defaults")
	}

	app := read("js/desktop/apps/bookshelf.js")
	if strings.Count(app, "        shell();") != 1 || strings.Index(app, "        shell();") > strings.Index(app, "        const root = host.querySelector('.bs-app');") {
		t.Error("Bookshelf must render its shell before querying the app root")
	}
}
