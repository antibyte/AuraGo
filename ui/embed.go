// Package ui exposes the source tree to frontend regression tests. Production
// servers use internal/webassets and do not link this package or embed this tree.
package ui

import (
	"os"
	"path/filepath"
	"runtime"

	"aurago/internal/webassets"
)

var Content = sourceFiles()

func sourceFiles() webassets.Files {
	_, file, _, _ := runtime.Caller(0)
	return webassets.Files{FS: os.DirFS(filepath.Dir(file))}
}
