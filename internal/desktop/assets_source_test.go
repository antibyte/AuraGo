package desktop

import (
	"aurago/internal/webassets"
	"os"
)

// Tests opt into source fixtures; production has no source-tree fallback.
var defaultPetSpritesheet, _ = os.ReadFile("pets_assets/openpets-default/spritesheet.webp")

func init() {
	bundledAppAssets = webassets.Files{FS: os.DirFS(".")}
	petAssets = webassets.Files{FS: os.DirFS(".")}
}
