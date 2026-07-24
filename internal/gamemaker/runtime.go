package gamemaker

import (
	"embed"
	"fmt"
	"os"
	"path/filepath"
)

// runtimeFS contains the exact browser runtimes shipped with every exported
// project. The files are sourced from the pinned upstream npm releases.
//
//go:embed runtime/*
var runtimeFS embed.FS

type runtimeAsset struct {
	embeddedPath string
	projectPath  string
}

func bundledRuntimeAssets(dimension string) []runtimeAsset {
	assets := []runtimeAsset{{
		embeddedPath: "runtime/THIRD_PARTY_NOTICES.md",
		projectPath:  "THIRD_PARTY_NOTICES.md",
	}}
	if dimension == "2d" {
		return append(assets, runtimeAsset{
			embeddedPath: "runtime/phaser-4.2.1.min.js",
			projectPath:  "vendor/phaser-4.2.1.min.js",
		})
	}
	return append(assets,
		runtimeAsset{
			embeddedPath: "runtime/three-0.185.1.module.min.js",
			projectPath:  "vendor/three-0.185.1.module.min.js",
		},
		runtimeAsset{
			embeddedPath: "runtime/three.core.min.js",
			projectPath:  "vendor/three.core.min.js",
		},
	)
}

func bundledRuntimeFile(dimension, projectPath string) ([]byte, bool, error) {
	for _, asset := range bundledRuntimeAssets(dimension) {
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

func installRuntime(projectDir, dimension string) error {
	for _, asset := range bundledRuntimeAssets(dimension) {
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
