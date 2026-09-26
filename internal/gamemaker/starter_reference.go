package gamemaker

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"sort"
	"strings"
)

// StarterReferences uses compact APIs only when the entire installed helper
// equals the accepted plan's generated source. A version comment is insufficient.
func (s *Service) StarterReferences(ctx context.Context, jobID string, plan *GamePlan) (map[string]any, error) {
	files := map[string]string{}
	for _, path := range []string{"src/common.ts", "src/scene.json", "src/mechanics.json"} {
		content, err := s.ReadJobFile(ctx, jobID, path)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return nil, err
		}
		if len(content) > 96000 {
			return nil, fmt.Errorf("starter reference %s exceeds 96000 bytes", path)
		}
		files[path] = content
	}
	return starterReferences(plan, files), nil
}

func starterReferences(plan *GamePlan, files map[string]string) map[string]any {
	result := map[string]any{"mode": "full", "files": files}
	if plan == nil {
		return result
	}
	expected, err := gameTemplateSources(*plan)
	common, exists := files["src/common.ts"]
	if err != nil || !exists || common != string(expected["common.ts"]) {
		return result
	}
	// This descriptor is now bound to the complete verified template.
	var descriptor map[string]any
	lines := strings.Split(string(expected["common.ts"]), "\n")
	for _, line := range lines[:min(8, len(lines))] {
		if strings.HasPrefix(line, runtimeContractPrefix) {
			_ = json.Unmarshal([]byte(strings.TrimPrefix(line, runtimeContractPrefix)), &descriptor)
			break
		}
	}
	if descriptor == nil {
		return result
	}
	api := phaserStarterAPI
	if guided3D(plan.Template) || plan.Template == "three" {
		api = threeStarterAPI
	}
	compact := map[string]any{"src/common.ts": map[string]any{"sha256": sourceHash(common), "api": descriptor, "reference": api}}
	for _, path := range []string{"src/scene.json", "src/mechanics.json"} {
		if content, ok := files[path]; ok {
			var value any
			if json.Unmarshal([]byte(content), &value) != nil {
				return result
			}
			structure := starterStructure(value, 0)
			encoded, _ := json.Marshal(structure)
			if len(encoded) > 12000 {
				structure = map[string]any{"details_omitted": true, "reason": "structure exceeds 12000 bytes", "root": starterStructure(value, 3)}
				root, _ := json.Marshal(structure)
				if len(root) > 12000 {
					structure = map[string]any{"details_omitted": true, "reason": "structure exceeds 12000 bytes"}
				}
			}
			compact[path] = map[string]any{"sha256": sourceHash(content), "bytes": len(content), "structure": structure}
		}
	}
	return map[string]any{"mode": "verified_template_api", "references": compact}
}

// Explicit structural summaries with omission counts are never partial source.
func starterStructure(value any, depth int) any {
	switch v := value.(type) {
	case map[string]any:
		keys := make([]string, 0, len(v))
		for key := range v {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		if depth >= 2 {
			return map[string]any{"keys": keys[:min(16, len(keys))], "key_count": len(keys)}
		}
		fields := map[string]any{}
		for _, key := range keys[:min(16, len(keys))] {
			fields[key] = starterStructure(v[key], depth+1)
		}
		return map[string]any{"fields": fields, "omitted_fields": max(0, len(keys)-16)}
	case []any:
		items := make([]any, 0, min(8, len(v)))
		if depth < 3 {
			for _, item := range v[:min(8, len(v))] {
				items = append(items, starterStructure(item, depth+1))
			}
		}
		return map[string]any{"count": len(v), "sample": items, "omitted_items": len(v) - len(items)}
	case string:
		if len(v) > 256 {
			return map[string]any{"string_bytes": len(v), "content_omitted": true}
		}
	}
	return value
}

const phaserStarterAPI = `Import {GameScene,start} from './common'; declare const Phaser:any. class Main extends GameScene; start(Main, gravity=0).
Inherit preload/create/update: they own assets, inputs, physics pause, restart, shutdown, test binding, HUD and sprite following. Implement setup(), step(dtSeconds), action(), tick() once per second and paintHUD(). setupScene() returns true when scene nodes created the player; return early then or use super.setup(). Default step moves the existing player; super.action() preserves builder actions and the normal action count.
Fields: player (physics GameObject), inputKeys.vector()->{x,y}, inputKeys.pressed(key), state, hud (screen-fixed compact status only), builder, levelIndex, elapsed (milliseconds). playerUIOptions in setup(): {objective,mode,movement:'full'|'horizontal'|'none',action:'jump'|'fire'|'interact'|'launch'|'boost'|false,instructions?}. Keep the shared once-before-play Start card, pause menu and device-specific touch layout. Do not draw a permanent instruction card or desktop button row. Use this.add/physics/time/cameras from Phaser.Scene. body(x,y,w,h,color,fixed=false,role='') returns a physics GameObject with separately following planned artwork. Never duplicate artwork or pass its body wrapper to collider/overlap; use persistent physics groups for repeated spawns. Dynamic objects move with object.body.setVelocity; static bodies use updateFromGameObject after moving the object. assetRoles(prefix) lists exact planned roles.
configureWorld(width,height,follow=true), configureLevels([{id,title,...}]), setCheckpoint(x,y), damagePlayer() (lives/checkpoint/invulnerability; false when ignored), end(won=false), nextLevel() (after victory only), restartGame(). feedback(name,object=player,material='flesh') is presentation; real contacts/rules own gameplay counters. state includes score, actions, hits, spawns, lives, goal_remaining, outcome, ended and event counters. Builder owns scene outcomes/damage. Do not manufacture test evidence. Keep scene/mechanics loaded by the helper. Reset custom cooldowns/timestamps (e.g. lastShot=-1000) and flags in setup: scene.restart() reuses the instance, so class initializers do not rerun when elapsed resets. Clean resources on scene shutdown.`

const threeStarterAPI = `Import {startGame} from './common'. startGame(config) returns api and owns loading, the single frame loop, controls, collision/cover, render, sound, reset, stage changes and disposal. Keep these helpers intact. Units: seconds, metres, radians.
Config: mode ('fps','exploration','transport','flight','space','three'), objective, goal, speed, duration, objects:[{role,at:[x,y,z],scale?,blocksShots?}], worldBounds:{min:[x,y,z],max:[x,y,z]}, levels:[{id,title,...configOverrides}], combat:{enemyRange,enemyDamage,enemyCooldown}. Roles refer to accepted, imported models. Scene nodes override the role-object starter; scene/mechanics/presentation are loaded by the helper.
Hooks: setup(api) after assets load, step(dtSeconds,api) adds rules after normal movement/simulation, action(api) returns false to override primary input, reset(api), hud(api) returns compact status only, dispose(api). config.playerUI can customize {movement:'full'|'horizontal'|'none',action:'fire'|'interact'|'boost'|false,instructions?}. Objective/instructions belong to the shared Start/pause card, never the running HUD. Keep desktop free of movement/action buttons; touch has a left stick, right actions and drag-look for FPS. Store custom state in your closure and reset it in reset; use setup for resources needing loaded assets. Do not add another frame/input loop.
api.scene, camera, player (THREE.Group), renderer, presentation, builder, input.isDown(key), ended, levelIndex; api.state is a snapshot in non-scene mode (writing it cannot change the game). api.damagePlayer(amount), setCheckpoint([x,y,z]), hasLineOfSight(from:[x,y,z],to:[x,y,z]), event(name,point?), win(), lose(), reset(), nextLevel(), dispose(). Builder owns scene health/contact outcomes. Real contacts own pickup/hit counters; presentation is not gameplay evidence. Keep resources you add under game ownership and dispose them. Use the supplied model API guide for extra visuals.`
