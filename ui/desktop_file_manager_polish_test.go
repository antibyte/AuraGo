package ui

import (
	"path/filepath"
	"strings"
	"testing"
)

// The File Manager polish pass turned the view switch into a segmented control,
// made the column bar sticky and gave the desktop a floating selection action bar.
// File operations stay in the window menu (see TestDesktopWindowMenuSelectiveMigration),
// so the toolbar keeps navigation and view controls only.
func TestFileManagerToolbarExposesViewControls(t *testing.T) {
	t.Parallel()

	render := readDesktopAssetText(t, "js/desktop/file-manager/core-render.js")
	for _, want := range []string{
		`data-action="back"`,
		`data-action="forward"`,
		`data-action="up"`,
		`data-action="search-toggle"`,
		`data-action="view-grid"`,
		`data-action="view-list"`,
		`class="fm-view-switch"`,
		`class="fm-toolbtn fm-view-switch-btn`,
		`fm-breadcrumb-wrap fm-breadcrumb-bar`,
		`aria-pressed="${fm.previewOpen ? 'true' : 'false'}"`,
		`aria-label="${esc(t('desktop.back'))}"`,
	} {
		if !strings.Contains(render, want) {
			t.Fatalf("file manager core-render.js missing toolbar marker %q", want)
		}
	}
	if strings.Count(render, "data-action=\"view-grid\"") != 1 {
		t.Fatal("the grid toggle must stay a single toolbar control")
	}
}

func TestFileManagerSelectionBarReachesDesktop(t *testing.T) {
	t.Parallel()

	render := readDesktopAssetText(t, "js/desktop/file-manager/core-render.js")
	for _, want := range []string{
		"selToolbar.classList.toggle('active', !!(fm.selectionMode && hasSelection))",
		"selToolbar.classList.toggle('has-selection', hasSelection)",
		"hasSelection ? ' has-selection' : ''",
	} {
		if !strings.Contains(render, want) {
			t.Fatalf("file manager core-render.js missing selection bar marker %q", want)
		}
	}

	css := readDesktopAssetText(t, "css/desktop-app-file-manager.css")
	for _, want := range []string{
		".fm-selection-toolbar.has-selection",
		"@media (min-width: 821px)",
		".fm-selection-toolbar.active",
		"@media (max-width: 820px)",
	} {
		if !strings.Contains(css, want) {
			t.Fatalf("file manager CSS missing selection bar rule %q", want)
		}
	}
}

func TestFileManagerListColumnsStayReadable(t *testing.T) {
	t.Parallel()

	css := readDesktopAssetText(t, "css/desktop-app-file-manager.css")
	for _, want := range []string{
		"position: sticky;",
		"--fm-col-width-date, 138px",
		"white-space: nowrap;",
		".fm-list-body {",
		"padding: 0 6px 6px;",
	} {
		if !strings.Contains(css, want) {
			t.Fatalf("file manager CSS missing list column rule %q", want)
		}
	}

	actions := readDesktopAssetText(t, "js/desktop/file-manager/actions-input.js")
	for _, want := range []string{
		"t('desktop.fm.prop_file')",
		"t('desktop.fm.sort_' + sortField)",
	} {
		if !strings.Contains(actions, want) {
			t.Fatalf("file manager actions-input.js missing list column marker %q", want)
		}
	}
}

// Every locale must carry the keys the polished toolbar and the status bar use.
func TestFileManagerPolishKeysExistInEveryLocale(t *testing.T) {
	t.Parallel()

	keys := []string{
		"desktop.fm.new_folder",
		"desktop.fm.new_file",
		"desktop.fm.upload",
		"desktop.fm.refresh",
		"desktop.fm.prop_file",
		"desktop.fm.prop_folder",
		"desktop.fm.sort_name",
		"desktop.fm.sort_size",
		"desktop.fm.sort_date",
		"desktop.fm.sort_type",
		"desktop.fm.sort_asc",
		"desktop.fm.sort_desc",
	}
	for _, lang := range []string{"cs", "da", "de", "el", "en", "es", "fr", "hi", "it", "ja", "nl", "no", "pl", "pt", "sv", "zh"} {
		values, err := readJSONFileMap(filepath.Join("lang", "desktop", lang+".json"))
		if err != nil {
			t.Fatalf("read %s locale: %v", lang, err)
		}
		for _, key := range keys {
			value, _ := values[key].(string)
			if strings.TrimSpace(value) == "" {
				t.Errorf("%s is missing the file manager translation %q", lang, key)
			}
		}
	}
}
