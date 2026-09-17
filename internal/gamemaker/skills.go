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
const gameplayExperienceGuidance = `Specify feedback, recovery/result, world/camera and progression in features. Hits need visible/audio responses; death needs recovery or clear defeat; endings need restart/continue controls. Exploration/traversal normally spans several viewports with landmarks and discoveries. Provide distinct areas, stages or evolving challenges; a deliberate single-board/arena is valid. Do not force combat/timers into peaceful ideas. A startup pass or HUD counter proves no game quality.`

// PhaseGuidance uses only the embedded, reviewed source, never a project file or
// a locally replaced skill. Startup still verifies the curated skill registry.
func PhaseGuidance(stage, dimension string) string {
	if stage == "planning" {
		return gameplayExperienceGuidance + ` Choose a supported game base and describe the player objective and 1–12 features with set_design. Optional helper fields belong inside design.mechanics, for example "mechanics":{"outcomes":["won","lost"],"lives":3}; blocks and events also belong there. Omit these helpers for continuous play or implement custom rules in source. Supply exact asset role/pack/asset IDs from search_assets where customization is needed. Guided 3D bases provide catalog-checked default models when assets is empty. Only fps/exploration/transport/flight/space accept settings (goal/speed/duration). For three and all 2D bases omit settings; omitting it or sending settings:null clears incompatible draft settings after an error. Keep the requested base and describe custom tuning in features/source or supported mechanics.blocks[].params. Resolve user-selected models into the appropriate roles. The server fills version, geometry, controls and validation defaults. On failure resubmit only the incorrect design fields (arrays replace whole); at most two corrections. For edits read the existing plan and affected source, keep the existing base and working behavior. Never write/import in planning. Accepted designs end this phase.`
	}
	common := gameplayExperienceGuidance + ` Implement the accepted design in src/main.ts. Keep common.ts lifecycle and asset bindings. Read relevant lines; replace with returned sha256. Check written and build.ok, fix source diagnostics, then validate once. Preserve working behavior in repairs. One game clock; clear input on blur, reset on restart, dispose on pagehide. Local runtime only; no remote imports/eval/services.`
	if dimension == "2d" {
		common += ` Phaser 4: subclass GameScene with setup(), step(dt), action(), tick(), paintHUD(); never override update/create. Assign this.player; preserve jump in action and call super.step(dt) when extending movement. Spawn the whole collider above ground. A stomp proves target damage/removal, not game outcome. body(x,y,w,h,color,fixed,role) fits/follows/animates art; its collider stays w/h. Match the visible footprint, not a square box. Never add another sprite to it; manual art requires an empty role and no super.setup creating another player. Roles: player/enemy/projectile/item/obstacle/goal. Use real GameObjects in collider/overlap and fixed=false for movers. Neither Arcade body type has setPosition: use object.setPosition plus body.reset(x,y), or updateFromGameObject for static proxies. Register persistent groups/overlaps once; never load a sheet as one image. New helpers: configureLevels([{id,title},...]) and levelIndex select distinct layouts in setup; configureWorld(width,height) after creating player enables camera follow. setCheckpoint(x,y), damagePlayer() consume a life on real harmful contact with protected respawn; builder health rules remain their own authority. feedback(name,object,material) supplies contact feedback; end(won) shows result/Continue, restartGame() resets campaign. Scene JSON levels also work. Use scope=full.`
	} else {
		common += ` Guided 3D: startGame(config) in main.ts uses common.ts lifecycle; keep renderer, asset bindings and FPS arms/weapon group. New config.levels:[{id,title,objects,goal,...},...] or scene.json levels provide distinct stages; config.worldBounds:{min:[x,y,z],max:[x,y,z]} controls authored world bounds. config.lives, api.levelIndex, api.setCheckpoint([x,y,z]), api.damagePlayer(amount), api.event(name,point), api.win()/lose() provide recovery/feedback/result/Continue. Builder health rules own scene-driven damage; feedback never manufactures damage. Implement custom mechanics in hooks/source. Keep fire() and observeTargets() on the same aimRay(caster), including read-only aim_ray origin/direction (observer x/y=ground, z=altitude). Tests use normal inputs; never invent hits/verdicts. Use scope=full for guided bases; free three has startup checks only, gameplay stays unverified.`
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
