package gamemaker

import (
	"fmt"
	"os"
	"path/filepath"

	"aurago/internal/webassets"
)

// runtimeFS contains the exact browser runtimes shipped with every exported
// project. The files are sourced from the pinned upstream npm releases.
var runtimeFS = webassets.Namespace("gamemaker")

type runtimeAsset struct {
	embeddedPath string
	projectPath  string
}

// Older published revisions still import the unversioned r185 core.
var legacyThreeRuntimeAssets = []runtimeAsset{
	{embeddedPath: "runtime/three-0.185.1.module.min.js", projectPath: "vendor/three-0.185.1.module.min.js"},
	{embeddedPath: "runtime/three.core.min.js", projectPath: "vendor/three.core.min.js"},
}

func bundledRuntimeAssets(dimension string, variant ...string) []runtimeAsset {
	assets := []runtimeAsset{{
		embeddedPath: "runtime/THIRD_PARTY_NOTICES.md",
		projectPath:  "THIRD_PARTY_NOTICES.md",
	}, {
		embeddedPath: "runtime/game-flow.js",
		projectPath:  "vendor/game-flow.js",
	}, {
		embeddedPath: "runtime/player-ui.js",
		projectPath:  "vendor/player-ui.js",
	}}
	if dimension == "2d" {
		return append(assets, runtimeAsset{
			embeddedPath: "runtime/isometric.js",
			projectPath:  "vendor/isometric.js",
		}, runtimeAsset{
			embeddedPath: "runtime/aurago-effects-2d-1.js",
			projectPath:  "vendor/aurago-effects-2d-1.js",
		}, runtimeAsset{
			embeddedPath: "runtime/phaser-4.2.1.min.js",
			projectPath:  "vendor/phaser-4.2.1.min.js",
		}, runtimeAsset{
			embeddedPath: "runtime/aurago-game-1.js",
			projectPath:  "vendor/aurago-game-1.js",
		}, runtimeAsset{
			embeddedPath: "runtime/scene-builder.js",
			projectPath:  "vendor/scene-builder.js",
		})
	}
	if len(variant) > 0 && variant[0] == "voxel" {
		for _, name := range []string{"voxel-world.js", "voxel-rules.js", "voxel-save.js", "voxel-ui.js", "aurago-voxel-1.js"} {
			assets = append(assets, runtimeAsset{embeddedPath: "runtime/" + name, projectPath: "vendor/" + name})
		}
	}
	return append(assets,
		runtimeAsset{embeddedPath: "runtime/aurago-effects-3d-1.js", projectPath: "vendor/aurago-effects-3d-1.js"},
		runtimeAsset{
			embeddedPath: "runtime/aurago-three-assets-1.js",
			projectPath:  "vendor/aurago-three-assets-1.js",
		},
		runtimeAsset{
			embeddedPath: "runtime/three-0.186.1.module.min.js",
			projectPath:  "vendor/three-0.186.1.module.min.js",
		},
		runtimeAsset{
			embeddedPath: "runtime/three-0.186.1.core.min.js",
			projectPath:  "vendor/three-0.186.1.core.min.js",
		},
		runtimeAsset{embeddedPath: "runtime/scene-builder.js", projectPath: "vendor/scene-builder.js"},
	)
}

func bundledRuntimeFile(dimension, projectPath string) ([]byte, bool, error) {
	assets := bundledRuntimeAssets(dimension)
	if dimension == "3d" {
		assets = append(assets, legacyThreeRuntimeAssets...)
	}
	for _, asset := range assets {
		if asset.projectPath != filepath.ToSlash(projectPath) {
			continue
		}
		data, err := runtimeFS.ReadFile(asset.embeddedPath)
		if err != nil {
			return nil, true, fmt.Errorf("read embedded game runtime %s: %w", asset.embeddedPath, err)
		}
		return data, true, nil
	}
	return nil, false, nil
}

func installRuntime(projectDir, dimension string, variant ...string) error {
	for _, asset := range bundledRuntimeAssets(dimension, variant...) {
		data, err := runtimeFS.ReadFile(asset.embeddedPath)
		if err != nil {
			return fmt.Errorf("read embedded game runtime %s: %w", asset.embeddedPath, err)
		}
		target := filepath.Join(projectDir, filepath.FromSlash(asset.projectPath))
		if err := os.MkdirAll(filepath.Dir(target), 0o750); err != nil {
			return fmt.Errorf("create game runtime directory: %w", err)
		}
		if err := os.WriteFile(target, data, 0o640); err != nil {
			return fmt.Errorf("write embedded game runtime %s: %w", asset.embeddedPath, err)
		}
	}
	return nil
}
