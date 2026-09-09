package gamemaker

import (
	"aurago/internal/webassets"
	"os"
)

func init() {
	runtimeFS = webassets.Files{FS: os.DirFS(".")}
	assetPackFS = webassets.Files{FS: os.DirFS(".")}
}
