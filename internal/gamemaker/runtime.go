package gamemaker

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

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

func readRuntimeAsset(embeddedPath string) ([]byte, error) {
	if embeddedPath == "runtime/zzfx.js" {
		return []byte(zzfxRuntimeJS), nil
	}
	return runtimeFS.ReadFile(embeddedPath)
}

func bundledRuntimeAssets(dimension string, variant ...string) []runtimeAsset {
	assets := []runtimeAsset{{
		embeddedPath: "runtime/THIRD_PARTY_NOTICES.md",
		projectPath:  "THIRD_PARTY_NOTICES.md",
	}, {
		embeddedPath: "runtime/zzfx.js",
		projectPath:  "vendor/zzfx.js",
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

func bundledRuntimeFile(dimension, projectPath string, variant ...string) ([]byte, bool, error) {
	assets := bundledRuntimeAssets(dimension, variant...)
	if dimension == "3d" {
		assets = append(assets, legacyThreeRuntimeAssets...)
	}
	target := filepath.ToSlash(projectPath)
	for _, asset := range assets {
		if asset.projectPath != target {
			continue
		}
		data, err := readRuntimeAsset(asset.embeddedPath)
		if err != nil {
			return nil, true, fmt.Errorf("read embedded game runtime %s: %w", asset.embeddedPath, err)
		}
		return data, true, nil
	}
	return nil, false, nil
}

func bundledRuntimeFileAny(preferredDimension, projectPath string) ([]byte, bool, error) {
	if preferredDimension == "" {
		preferredDimension = "2d"
	}
	if data, found, err := bundledRuntimeFile(preferredDimension, projectPath); found {
		return data, true, err
	}
	other := "3d"
	if preferredDimension == "3d" {
		other = "2d"
	}
	if data, found, err := bundledRuntimeFile(other, projectPath); found {
		return data, true, err
	}
	return bundledRuntimeFile("3d", projectPath, "voxel")
}

func installRuntime(projectDir, dimension string, variant ...string) error {
	return installRuntimeAssets(projectDir, bundledRuntimeAssets(dimension, variant...), false)
}

func installRuntimeAssets(projectDir string, assets []runtimeAsset, preserveExisting bool) error {
	for _, asset := range assets {
		data, err := readRuntimeAsset(asset.embeddedPath)
		if err != nil {
			return fmt.Errorf("read embedded game runtime %s: %w", asset.embeddedPath, err)
		}
		target := filepath.Join(projectDir, filepath.FromSlash(asset.projectPath))
		if preserveExisting {
			existing, err := os.ReadFile(target)
			switch {
			case err == nil:
				if !bytes.Equal(existing, data) {
					return fmt.Errorf("library file conflicts with existing %s", asset.projectPath)
				}
				continue
			case !os.IsNotExist(err):
				return fmt.Errorf("read existing library file %s: %w", asset.projectPath, err)
			}
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o750); err != nil {
			return fmt.Errorf("create game runtime directory: %w", err)
		}
		if err := os.WriteFile(target, data, 0o640); err != nil {
			return fmt.Errorf("write embedded game runtime %s: %w", asset.embeddedPath, err)
		}
	}
	return nil
}

// AvailableLibraries returns concise metadata about local vendor libraries that
// can be installed into a project via add_library without overwriting source.
func AvailableLibraries() []map[string]any {
	return []map[string]any{
		{
			"id":      "phaser",
			"files":   []string{"vendor/phaser-4.2.1.min.js", "vendor/aurago-game-1.js", "vendor/aurago-effects-2d-1.js", "vendor/player-ui.js", "vendor/game-flow.js", "vendor/isometric.js", "vendor/scene-builder.js", "vendor/zzfx.js"},
			"exports": "Phaser global (4.2.1); aurago-game-1.js: preloadPack, registerAnimations, createAsset, createAssembly, fitVisual, playAction, setFacing, createInputs; player-ui.js: createPlayerUI; game-flow.js: createGameFlow; aurago-effects-2d-1.js: createPresentation, createPhaserAdapter; zzfx.js: zzfx, playSfx, sfxPresets",
			"usage":   `<script src="vendor/phaser-4.2.1.min.js"></script> and import { preloadPack, createAsset, createInputs } from './vendor/aurago-game-1.js';`,
		},
		{
			"id":      "three",
			"files":   []string{"vendor/three-0.186.1.module.min.js", "vendor/three-0.186.1.core.min.js", "vendor/aurago-three-assets-1.js", "vendor/aurago-effects-3d-1.js", "vendor/player-ui.js", "vendor/game-flow.js", "vendor/scene-builder.js", "vendor/zzfx.js"},
			"exports": "Three.js r186 ES module; aurago-three-assets-1.js: THREE, loadAsset, createInstance, playAction, updateInstance, attachToSocket, setPart, disposeInstance, releaseAsset; aurago-effects-3d-1.js: createPresentation, createThreeAdapter; player-ui.js: createPlayerUI; game-flow.js: createGameFlow; zzfx.js: zzfx, playSfx, sfxPresets",
			"usage":   `import * as THREE from './vendor/three-0.186.1.module.min.js'; import * as A from './vendor/aurago-three-assets-1.js';`,
		},
		{
			"id":      "voxel",
			"files":   []string{"vendor/aurago-voxel-1.js", "vendor/voxel-world.js", "vendor/voxel-rules.js", "vendor/voxel-save.js", "vendor/voxel-ui.js", "vendor/three-0.186.1.module.min.js", "vendor/three-0.186.1.core.min.js", "vendor/zzfx.js"},
			"exports": "aurago-voxel-1.js: startVoxelGame",
			"usage":   `import { startVoxelGame } from './vendor/aurago-voxel-1.js';`,
		},
		{
			"id":      "effects-2d",
			"files":   []string{"vendor/aurago-effects-2d-1.js", "vendor/player-ui.js", "vendor/game-flow.js", "vendor/zzfx.js"},
			"exports": "aurago-effects-2d-1.js: createPresentation, createPhaserAdapter; player-ui.js: createPlayerUI; game-flow.js: createGameFlow; zzfx.js: zzfx, playSfx, sfxPresets",
			"usage":   `import { createPresentation, createPhaserAdapter } from './vendor/aurago-effects-2d-1.js';`,
		},
		{
			"id":      "effects-3d",
			"files":   []string{"vendor/aurago-effects-3d-1.js", "vendor/player-ui.js", "vendor/game-flow.js", "vendor/zzfx.js"},
			"exports": "aurago-effects-3d-1.js: createPresentation, createThreeAdapter; player-ui.js: createPlayerUI; game-flow.js: createGameFlow; zzfx.js: zzfx, playSfx, sfxPresets",
			"usage":   `import { createPresentation, createThreeAdapter } from './vendor/aurago-effects-3d-1.js';`,
		},
		{
			"id":      "zzfx",
			"files":   []string{"vendor/zzfx.js"},
			"exports": "zzfx, playSfx, sfxPresets, getAudioContext",
			"usage":   `import { playSfx, zzfx, sfxPresets } from './vendor/zzfx.js';`,
		},
		{
			"id":      "all",
			"files":   []string{"All bundled 2D, 3D, Voxel, UI, effects and ZzFX runtime files under vendor/"},
			"exports": "Complete offline runtime suite",
			"usage":   "Import any vendor/*.js module directly from HTML/JS or src/main.ts",
		},
	}
}

func resolveLibraryBundle(library string) (string, []runtimeAsset, string, error) {
	key := strings.ToLower(strings.TrimSpace(library))
	notices := runtimeAsset{embeddedPath: "runtime/THIRD_PARTY_NOTICES.md", projectPath: "THIRD_PARTY_NOTICES.md"}
	zzfx := runtimeAsset{embeddedPath: "runtime/zzfx.js", projectPath: "vendor/zzfx.js"}
	flow := runtimeAsset{embeddedPath: "runtime/game-flow.js", projectPath: "vendor/game-flow.js"}
	playerUI := runtimeAsset{embeddedPath: "runtime/player-ui.js", projectPath: "vendor/player-ui.js"}
	switch key {
	case "phaser":
		return key, bundledRuntimeAssets("2d"), `<script src="vendor/phaser-4.2.1.min.js"></script> (for index.html) and import { preloadPack, registerAnimations, createAsset, createInputs } from '../vendor/aurago-game-1.js'; (from src/*.ts) or './vendor/aurago-game-1.js' (from root)`, nil
	case "three":
		return key, bundledRuntimeAssets("3d"), `import * as THREE from '../vendor/three-0.186.1.module.min.js'; import * as A from '../vendor/aurago-three-assets-1.js'; (use './vendor/...' from index.html)`, nil
	case "voxel":
		return key, bundledRuntimeAssets("3d", "voxel"), `import { startVoxelGame } from '../vendor/aurago-voxel-1.js';`, nil
	case "effects-2d":
		return key, []runtimeAsset{notices, zzfx, flow, playerUI, {embeddedPath: "runtime/aurago-effects-2d-1.js", projectPath: "vendor/aurago-effects-2d-1.js"}}, `import { createPresentation, createPhaserAdapter } from '../vendor/aurago-effects-2d-1.js';`, nil
	case "effects-3d":
		return key, []runtimeAsset{notices, zzfx, flow, playerUI, {embeddedPath: "runtime/aurago-effects-3d-1.js", projectPath: "vendor/aurago-effects-3d-1.js"}}, `import { createPresentation, createThreeAdapter } from '../vendor/aurago-effects-3d-1.js';`, nil
	case "zzfx":
		return key, []runtimeAsset{notices, zzfx}, `import { playSfx, zzfx, sfxPresets } from './vendor/zzfx.js'; (or '../vendor/zzfx.js' from src/)`, nil
	case "all":
		seen := map[string]bool{}
		var combined []runtimeAsset
		for _, list := range [][]runtimeAsset{bundledRuntimeAssets("2d"), bundledRuntimeAssets("3d", "voxel")} {
			for _, a := range list {
				if !seen[a.projectPath] {
					seen[a.projectPath] = true
					combined = append(combined, a)
				}
			}
		}
		return key, combined, "All bundled vendor/*.js runtimes installed; import relative to index.html or src/.", nil
	default:
		return "", nil, "", fmt.Errorf("unknown library %q; choose phaser, three, voxel, effects-2d, effects-3d, zzfx or all", library)
	}
}

// AddLibrary installs bundled runtime files without replacing existing files.
func (s *Service) AddLibrary(ctx context.Context, jobID, library string) (map[string]any, error) {
	key, assets, usage, err := resolveLibraryBundle(library)
	if err != nil {
		return nil, err
	}
	s.fileMu.Lock()
	defer s.fileMu.Unlock()
	s.buildMu.Lock()
	defer s.buildMu.Unlock()
	if err := s.CheckJobMutation(ctx, jobID); err != nil {
		return nil, err
	}
	project, _, err := s.ProjectForJob(ctx, jobID)
	if err != nil {
		return nil, err
	}
	stage, err := s.JobDirectory(jobID)
	if err != nil {
		return nil, err
	}
	candidate, err := os.MkdirTemp(s.stagingDir, ".library-*")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(candidate)
	if err := copyTree(stage, candidate, s.opts.MaxFilesPerProject, s.opts.MaxProjectBytes); err != nil {
		return nil, err
	}
	if err := installRuntimeAssets(candidate, assets, true); err != nil {
		return nil, err
	}
	if err := validateTreeLimits(candidate, s.opts.MaxFilesPerProject, s.opts.MaxProjectBytes); err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	backup := candidate + "-previous"
	if err := os.Rename(stage, backup); err != nil {
		return nil, err
	}
	if err := os.Rename(candidate, stage); err != nil {
		if rollback := os.Rename(backup, stage); rollback != nil {
			return nil, fmt.Errorf("library commit failed: %w; restore failed: %v", err, rollback)
		}
		return nil, err
	}
	_ = os.RemoveAll(backup)
	installed := make([]string, 0, len(assets))
	for _, asset := range assets {
		installed = append(installed, asset.projectPath)
	}
	_, _ = s.emit(ctx, project.ID, jobID, "file_changed", map[string]any{"path": "vendor", "library": key})
	return map[string]any{
		"library":         key,
		"installed_files": installed,
		"usage":           usage,
		"note":            "Existing index.html, game.json and src/ files were preserved.",
	}, nil
}
