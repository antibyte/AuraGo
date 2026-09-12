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
		Description: "Plans the game loop, coordinates implementation phases, and keeps the result playable.",
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
		Description: "Validates load state, canvas readiness, runtime errors, frame rate, controls, and game feel.",
		Source:      "AuraGo clean-room synthesis; TinySwords concepts only",
		Commit:      TinySwordsCommit,
		License:     "AuraGo original; no TinySwords code or assets",
	},
}

type SkillInstallResult struct {
	Skills []SkillInfo
	Ready  bool
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

// PhaseGuidance uses only the embedded, reviewed source, never a project file or
// a locally replaced skill. Startup still verifies the curated skill registry.
func PhaseGuidance(stage, dimension string) string {
	if stage == "planning" {
		return `Choose a supported game base and describe the player objective and 1–12 features with set_design. Optional helper fields belong inside design.mechanics, for example "mechanics":{"outcomes":["won","lost"],"lives":3}; blocks and events also belong there. Omit these helpers for continuous play or implement custom rules in source. Supply exact asset role/pack/asset IDs from search_assets where customization is needed. Guided 3D bases provide catalog-checked default models when assets is empty. Resolve user-selected models into the appropriate roles. The server fills version, geometry, controls and validation defaults. On failure resubmit only the incorrect design fields (arrays replace whole); at most two corrections. For edits read the existing plan and affected source, keep the existing base and working behavior. Never write/import in planning. Accepted designs end this phase.`
	}
	common := `Implement the accepted design in the installed source. Start with src/main.ts; retain common.ts lifecycle and the generated role bindings. Read only relevant line ranges; use replace with the returned sha256 for local changes. Writes return written and build.ok separately: fix compiler errors at their source location before validation. Keep existing art and behavior in repair jobs. Validate once after a complete change; missing browser observations cannot pass. Keep one game clock, clear input on blur, reset all gameplay state on restart and release resources on pagehide. Offline local runtime only; no remote imports, eval or network services.`
	if dimension == "2d" {
		common += ` Phaser 4: subclass GameScene using setup(), step(deltaSeconds), action(), tick(), paintHUD(); never override update/create. Assign this.player. body(x,y,w,h,color,fixed,role) binds planned artwork automatically; roles include player, enemy, projectile, item, obstacle and goal. Pass actual physics GameObjects to collider/overlap, not wrapper records. Use fixed=false for moving bodies. Neither Arcade body type has setPosition; use object.setPosition and body.reset(x,y), or updateFromGameObject for static proxies. Use persistent groups for spawned objects and register overlap once. Never load a library sheet as a single image. Use scope=full.`
	} else {
		common += ` Guided 3D bases install startGame(config) in main.ts and a reusable common.ts. Edit the small config (goal/speed/duration, objects and roles) or the specific game rules in common.ts; do not rewrite the renderer. All selected model manifests and roles are wired automatically. FPS arms/weapon share one camera group and metadata bindings. Use scope=full for guided bases; free-code template three supports startup checks only. Unsupported requested mechanics require real source implementation, not a prose claim.`
	}
	if stage == "repair" {
		common += ` Repair only the supplied diagnostics, then validate and end the turn.`
	}
	return common
}

func equalSHA256(left, right []byte) bool {
	leftHash := sha256.Sum256(left)
	rightHash := sha256.Sum256(right)
	return hex.EncodeToString(leftHash[:]) == hex.EncodeToString(rightHash[:])
}
