package ui

import (
	"strings"
	"testing"
)

func TestDesktopArchivePredicatesShareOnePattern(t *testing.T) {
	t.Parallel()
	const pattern = `/\.(?:tar\.(?:gz|bz2|xz|zst|lzma|lz)|tgz|tbz2?|txz|tzst|zip|7z|rar|tar|gz|bz2|xz|zst|lzma|lz|cab|iso|wim|arj|lzh|lha|cpio|deb|rpm|xar|img|raw|ext[234]|squashfs|sqsh|sfs|btrfs|xfs|erofs|cramfs|qcow2?|vmdk|vhdx?|vdi|dmg|udf)$/i`
	for _, file := range []string{
		"js/desktop/apps/zipper.js",
		"js/desktop/core/desktop-foundation.js",
		"js/desktop/file-manager/actions-input.js",
	} {
		source := readDesktopAssetText(t, file)
		if !strings.Contains(source, pattern) {
			t.Fatalf("%s drifted from the shared archive pattern", file)
		}
	}
	editor := readDesktopAssetText(t, "js/desktop/apps/editor-filemenu.js")
	bookshelf := strings.Index(editor, `fb2(?:\.zip)?`)
	archive := strings.Index(editor, "desktopIsArchiveFile")
	if bookshelf < 0 || archive < 0 || bookshelf > archive {
		t.Fatal("bookshelf routing must stay ahead of Zipper")
	}
	spotlight := readDesktopAssetText(t, "js/desktop/core/spotlight-runtime.js")
	if strings.Index(spotlight, `fb2(?:\.zip)?`) > strings.Index(spotlight, "desktopIsArchiveFile") {
		t.Fatal("spotlight must open bookshelf formats before Zipper")
	}
	zipper := readDesktopAssetText(t, "js/desktop/apps/zipper.js")
	if !strings.Contains(zipper, "function isCreatableArchive") || strings.Contains(zipper, "return isOpenableArchive(value)") {
		t.Fatal("create dialog must not keep read-only archive extensions")
	}
}
