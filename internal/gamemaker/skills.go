package gamemaker

import (
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"sort"
)

const (
	ThreeSkillsCommit  = "7221c1f4a6d2ae189a4d85d058d24f3228499d46"
	PhaserSkillsCommit = "41be1e462bc600064e498cba370bfa8c5c055a22"
	TinySwordsCommit   = "f59f1dca8bf461227c9b5d856764e1e90d8b8e90"
)

//go:embed skills/*/SKILL.md
var bundledSkills embed.FS

var curatedSkills = []SkillInfo{
	{
		Name:        "aurago-game-maker-director",
		Description: "Supports free development with files, builds and independent preview tools.",
		Source:      "AuraGo synthesis",
		Commit:      ThreeSkillsCommit + " + " + PhaserSkillsCommit,
		License:     "MIT",
	},
	{
		Name:        "aurago-phaser4-gameplay",
		Description: "Phaser 4 scene, input, physics, camera, animation, and responsive gameplay guidance.",
		Source:      "phaserjs/phaser skills",
		Commit:      PhaserSkillsCommit,
		License:     "MIT",
	},
	{
		Name:        "aurago-threejs-gameplay",
		Description: "Three.js gameplay, rendering, controls, UI, performance, and debugging guidance.",
		Source:      "majidmanzarpour/threejs-game-skills",
		Commit:      ThreeSkillsCommit,
		License:     "MIT",
	},
	{
		Name:        "aurago-game-assets",
		Description: "Selects AI-generated or procedural images, music, textures, UI art, and Web Audio effects.",
		Source:      "AuraGo synthesis",
		Commit:      PhaserSkillsCommit + " + " + ThreeSkillsCommit,
		License:     "MIT",
	},
	{
		Name:        "aurago-game-qa",
		Description: "Optional browser feedback and gameplay inspection guidance.",
		Source:      "AuraGo clean-room synthesis; TinySwords concepts only",
		Commit:      TinySwordsCommit,
		License:     "AuraGo original; no TinySwords code or assets",
	},
}

type SkillInstallResult struct {
	Skills []SkillInfo
	Ready  bool
}

// BundledSkillMarkdown returns trusted compile-time bytes for a curated package.
// All currently bundled packages contain exactly this one file.
func BundledSkillMarkdown(name string) ([]byte, error) {
	for _, skill := range curatedSkills {
		if skill.Name == name {
			return bundledSkills.ReadFile("skills/" + name + "/SKILL.md")
		}
	}
	return nil, fmt.Errorf("unknown bundled game maker skill %q", name)
}

// InstallBundledSkills installs missing system-managed packages and repairs
// drifted ones by overwriting them with the bundled version. The curated
// skills are fully managed by AuraGo, so local edits are discarded on startup
// (self-healing) instead of blocking Game Maker jobs.
func InstallBundledSkills(root string) (SkillInstallResult, error) {
	result := SkillInstallResult{Ready: true}
	for _, definition := range curatedSkills {
		data, err := bundledSkills.ReadFile("skills/" + definition.Name + "/SKILL.md")
		if err != nil {
			return result, fmt.Errorf("read bundled game maker skill %s: %w", definition.Name, err)
		}
		dir := filepath.Join(root, definition.Name)
		path := filepath.Join(dir, "SKILL.md")
		status := "installed"
		existing, readErr := os.ReadFile(path)
		switch {
		case os.IsNotExist(readErr):
			if err := os.MkdirAll(dir, 0o750); err != nil {
				return result, fmt.Errorf("create bundled game maker skill directory: %w", err)
			}
			if err := os.WriteFile(path, data, 0o640); err != nil {
				return result, fmt.Errorf("install bundled game maker skill: %w", err)
			}
		case readErr != nil:
			return result, fmt.Errorf("inspect bundled game maker skill: %w", readErr)
		case !equalSHA256(existing, data):
			if err := os.WriteFile(path, data, 0o640); err != nil {
				return result, fmt.Errorf("repair bundled game maker skill %s: %w", definition.Name, err)
			}
			status = "updated"
		default:
			status = "verified"
		}
		definition.Status = status
		result.Skills = append(result.Skills, definition)
	}
	sort.Slice(result.Skills, func(i, j int) bool { return result.Skills[i].Name < result.Skills[j].Name })
	return result, nil
}

func CuratedSkillNames() []string {
	names := make([]string, 0, len(curatedSkills))
	for _, skill := range curatedSkills {
		names = append(names, skill.Name)
	}
	sort.Strings(names)
	return names
}

// Keep the same concise quality contract in planning and implementation context.
func equalSHA256(left, right []byte) bool {
	leftHash := sha256.Sum256(left)
	rightHash := sha256.Sum256(right)
	return hex.EncodeToString(leftHash[:]) == hex.EncodeToString(rightHash[:])
}
