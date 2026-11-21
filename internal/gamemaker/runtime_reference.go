package gamemaker

import "strings"

// runtimeReference is the complete public API of one installed helper version.
// Members lists every documented definition; tests require each to exist in
// the template that declares the version, so the text cannot drift silently.
type runtimeReference struct {
	Text    string
	Members []string
	// Source names the embedded runtime that defines the members when the
	// template only re-exports it (voxel-common.ts).
	Source string
}

// runtimeReferences is keyed by the AURAGO_RUNTIME_API descriptor version of a
// verified installed common.ts. Unverified helpers are read from source.
var runtimeReferences = map[string]runtimeReference{
	"phaser-2": {Text: phaserRuntimeReference, Members: []string{
		"start", "setupScene", "setup", "step", "action", "tick", "paintHUD", "player", "state", "elapsed",
		"levelIndex", "inputKeys", "flow", "hud", "body", "configureLevels", "nextLevel", "restartGame",
		"configureWorld", "setCheckpoint", "damagePlayer", "feedback", "end", "assetRoles",
	}},
	"three-4": {Text: threeRuntimeReference, Members: []string{
		"startGame", "isDown", "hasLineOfSight", "damagePlayer", "setCheckpoint", "levelIndex", "nextLevel",
		"event", "win", "lose", "reset", "dispose",
	}},
	"voxel-1": {Text: voxelRuntimeReference, Source: "runtime/aurago-voxel-1.js", Members: []string{
		"startVoxelGame", "objective", "setup", "step", "action", "getBlock", "setBlock", "transaction",
		"count", "craft", "damagePlayer", "event", "dispose",
	}},
}

const voxelRuntimeReference = `Voxel API (installed helper voxel-1; src/common.ts re-exports the runtime). This is the complete public contract: do not read vendor files to rediscover it.
Entry: src/main.ts imports definition from './voxel.json' and {startVoxelGame} from './common' and calls startVoxelGame(definition, hooks) once. The runtime owns the clock, meshing, physics, input, inventory and crafting UI, enemies, start/pause, touch controls and saves. Most requests need no hooks: the definition in src/voxel.json is the implementation.
Hooks (all optional):
- objective: short start-screen text.
- setup(api): after the world and the save have loaded.
- step(api, dt): every simulated frame; dt is in seconds.
- action(api, target): primary action on the aimed block or enemy; return false to suppress the default mine/attack.
- dispose(api): remove your own scene additions.
api:
- scene, camera, renderer (Three.js). event(name, point): effects and sound only.
- getBlock(x, y, z) returns the block id (0 is air). setBlock(x, y, z, id) returns true when the cell changed; y = 0 and cells inside the player are refused.
- transaction(take, give) with {itemId: count} maps commits both sides atomically and returns true or false. count(itemId) returns the inventory total. craft(recipeId) returns true when crafted.
- damagePlayer(amount): no effect in creative mode.
- player: read-only snapshot {x, y, z, yaw, pitch, health, ...}. progress: read-only counters keyed "collect:<item>", "craft:<item>", "place:<item>" and "defeat".
- dispose().
Use these APIs to mutate the grid or inventory so meshing, collision, UI and saves stay synchronized.`

const phaserRuntimeReference = `Phaser GameScene API (installed helper phaser-2 in src/common.ts). This is the complete public contract: do not read common.ts or vendor files to rediscover it.
Entry: src/main.ts imports {GameScene, start} from './common', declares class Main extends GameScene {...} and ends with start(Main, gravity). gravity is the Arcade y gravity (0 top-down, about 1000 for platformers). The logical canvas defaults to 960×540 with FIT scaling.
Hooks (override these; never override preload, create or update):
- setup(): build the stage. Keep "if (this.setupScene()) return;" first when scene.json may hold nodes. Assign this.player. Reset custom cooldowns, spawn clocks and per-run flags here: restarts reuse the instance, so class field initializers do not run again.
- step(dt): per-frame gameplay. dt is in SECONDS (at most 0.05). Call super.step(dt) to keep built-in arrow/WASD movement and jumping.
- action(): primary action, called on every SPACE press. When overriding, increment this.state.actions yourself or call super.action().
- tick(): once per second while playing.
- paintHUD(): compact live status only, via this.hud.setText(...).
Fields:
- this.player: the controlled object followed by the camera and default movement; assign it in setup().
- this.state: shared HUD and flow counters: score, actions, hits, spawns, turns, ticks, pickup_events, lives, health, goal_remaining, outcome (0 playing, 1 won, 2 lost), ended.
- this.elapsed: MILLISECONDS since the stage started; it resets on restart. Compare cooldowns in milliseconds.
- this.levelIndex: current stage (0-based); choose the layout from it in setup().
- this.inputKeys: down(...names) held, pressed(name) newly pressed, vector() gives {x,y} in -1..1 from arrows and WASD. Keys: LEFT RIGHT UP DOWN W A S D SPACE R P ESC ENTER. R restarts, P pauses and ESC ends the game. Other keys are not registered: use this.input.keyboard.addKey('F') for them.
- this.flow.message(text, seconds): short on-screen notice. this.hud is the HUD text object.
Helpers:
- body(x, y, w, h, color, fixed = false, role = ''): rectangle with an Arcade body (static when fixed) and optional artwork for role (player, enemy, item, projectile, goal, obstacle or custom); returns the GameObject while keeping the collider at w×h.
- configureLevels([{id, title}, ...]) declares stages; nextLevel() advances after a win; restartGame() returns to stage 0.
- configureWorld(width, height, follow = true): world and camera bounds, camera follows this.player.
- setCheckpoint(x, y); damagePlayer() consumes a life on real harmful contact with feedback and a protected respawn, ends the game at 0 lives and returns true when damage applied. Set this.state.lives in setup() for life-based games.
- feedback(name, object = this.player, material = 'flesh'): effect and sound for hit, pickup, shot, jump, land, damage, death, win, lose, interact and similar events. feedback('pickup', item) also increments this.state.pickup_events.
- end(won): result screen with Continue or Restart.
- assetRoles(prefix): configured asset roles named prefix or prefix_*.
Physics: pass GameObjects returned by body() to this.physics.add.collider/overlap; register groups and overlaps once in setup(); movers use fixed = false. Arcade bodies have no setPosition: use object.setPosition(x, y) plus object.body.reset(x, y), or body.updateFromGameObject() for static bodies.
Lifecycle & state: hits = this.state.hits, incremented inside collision callbacks; pickup_events = feedback('pickup', ...) calls; player position lives on this.player; configureLevels sets the stage list.`

const threeRuntimeReference = `Three.js startGame API (installed helper three-4 in src/common.ts). This is the complete public contract: do not read common.ts or vendor files to rediscover it.
Entry: src/main.ts imports {startGame} from './common' and calls startGame(config) once. The helper owns renderer, camera, player, input, clock, HUD, start/pause/result UI, presentation and disposal.
config:
- mode: fps, exploration, transport, flight or space (camera and control preset). goal: count to win. speed: metres per second. duration: seconds, 0 disables the countdown. lives: default 3.
- objects: [{role, at: [x, y, z], scale?, blocksShots?}] using configured asset roles. Built-in behaviour: item (collected within 1.5 m) and cargo (2 m, then carried) raise score, hits and pickup_events; goal collects like an item in flight and completes a cargo delivery in transport; enemy is hit by the primary action in fps and space (hits, score) and attacks the player within combat range; tree, building, obstacle and terrain are scenery that can block shots.
- levels: [{id, title, objects?, goal?, worldBounds?, mode?}, ...]; each entry overrides config for its stage. Each stage can define its own layout and objective.
- worldBounds: {min: [x, y, z], max: [x, y, z]}. combat: {enemyRange, enemyDamage, enemyCooldown}. playerUI: {movement, action, instructions?}.
- Hooks: setup(api) after assets load; step(dt, api) every frame with dt in seconds; action(api) on the primary action, return false to cancel the built-in action or true after a real custom interaction; reset(api) on restart; dispose(api) on teardown.
api:
- scene, camera, player (a THREE.Object3D), renderer, presentation; ended; builder (scene builder or null).
- input.isDown(key): held key by lowercase KeyboardEvent.key, such as 'w', ' ' for Space or 'arrowleft'. R restarts, P pauses, F reloads in fps and Escape ends the game.
- state: read-only snapshot {score, hits, actions, lives, health, ammo, pickup_events, outcome, ended, ...}.
- damagePlayer(amount = 25): real harmful contact only; scene-driven games own damage through scene rules instead.
- setCheckpoint([x, y, z]); hasLineOfSight(from, to) returns false when cover blocks the line.
- event(name, point?): feedback effects and sound only; it never changes counters.
- levelIndex (current stage), nextLevel(), win(), lose(), reset(), dispose().
State & roles: built-in role behaviour and scene-builder rules update score, hits and pickup_events automatically for config.objects roles; custom Three.js meshes and rules can also be added freely in setup/step/action.
Movement is camera-relative: D/RIGHT moves toward screen right and W/UP away from the camera.`

// CompactRuntimeReference bounds a complete runtime reference for small-model
// runs. Only whole lines are kept. Cutting from the tail used to drop the
// Lifecycle, state and feedback lines that weak models need most, so selection
// now follows importance while the emitted lines keep their original order.
const compactRuntimeReferenceTail = "\n- Cheat sheet only: read bounded src/common.ts ranges for the remaining members."

var compactReferencePriorities = []string{
	"Lifecycle & state:",
	"feedback(",
	"State & roles:",
	"this.state:",
	"this.player:",
	"step(dt)",
	"action()",
	"body(",
	"Entry:",
	"Hooks (",
}

func CompactRuntimeReference(reference string, maxBytes int) string {
	reference = strings.TrimSpace(reference)
	if maxBytes <= 0 {
		maxBytes = 2000
	}
	if len(reference) <= maxBytes {
		return reference
	}
	budget := maxBytes - len(compactRuntimeReferenceTail)
	if budget < 0 {
		budget = 0
	}
	lines := strings.Split(reference, "\n")
	selected := make([]bool, len(lines))
	used := 0
	take := func(index int) bool {
		if selected[index] {
			return true
		}
		next := len(lines[index])
		if used > 0 {
			next++
		}
		if used+next > budget {
			return false
		}
		selected[index] = true
		used += next
		return true
	}
	for _, keyword := range compactReferencePriorities {
		for index, line := range lines {
			if strings.Contains(line, keyword) {
				_ = take(index)
			}
		}
	}
	for index := range lines {
		_ = take(index)
	}
	var sheet strings.Builder
	for index, line := range lines {
		if !selected[index] {
			continue
		}
		if sheet.Len() > 0 {
			sheet.WriteByte('\n')
		}
		sheet.WriteString(line)
	}
	return sheet.String() + compactRuntimeReferenceTail
}
