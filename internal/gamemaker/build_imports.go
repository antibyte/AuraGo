package gamemaker

import (
	"fmt"
	"path/filepath"

	"github.com/evanw/esbuild/pkg/api"
)

func projectImportBounds(projectDir string) api.Plugin {
	return api.Plugin{Name: "project-import-bounds", Setup: func(build api.PluginBuild) {
		// Run before compatibility loaders so none can read an escaped import.
		build.OnLoad(api.OnLoadOptions{Filter: ".*", Namespace: "file"}, func(args api.OnLoadArgs) (api.OnLoadResult, error) {
			rel, err := filepath.Rel(projectDir, args.Path)
			if err == nil {
				_, _, err = secureJoin(projectDir, rel, false)
			}
			if err != nil {
				return api.OnLoadResult{}, fmt.Errorf("imports must stay inside the game project: %w", ErrInvalidPath)
			}
			return api.OnLoadResult{}, nil
		})
	}}
}
