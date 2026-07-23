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

func installRuntime(projectDir, dimension string) error {
	vendorDir := filepath.Join(projectDir, "vendor")
	if err := os.MkdirAll(vendorDir, 0o750); err != nil {
		return fmt.Errorf("create game runtime directory: %w", err)
	}
	files := []string{"runtime/THIRD_PARTY_NOTICES.md"}
	if dimension == "2d" {
		files = append(files, "runtime/phaser-4.2.1.min.js")
	} else {
		files = append(files, "runtime/three-0.185.1.module.min.js")
	}
	for _, name := range files {
		data, err := runtimeFS.ReadFile(name)
		if err != nil {
			return fmt.Errorf("read embedded game runtime %s: %w", name, err)
		}
		target := filepath.Join(vendorDir, filepath.Base(name))
		if filepath.Base(name) == "THIRD_PARTY_NOTICES.md" {
			target = filepath.Join(projectDir, "THIRD_PARTY_NOTICES.md")
		}
		if err := os.WriteFile(target, data, 0o640); err != nil {
			return fmt.Errorf("write embedded game runtime %s: %w", name, err)
		}
	}
	return nil
}
