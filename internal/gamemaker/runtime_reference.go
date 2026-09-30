package gamemaker

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

// runtimeReferences is keyed by the AURAGO_RUNTIME_API descriptor version of an
// installed common.ts. Legacy or custom helpers have no entry and are read.
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
Only these APIs change the grid or inventory. Checks read real block, inventory and player evidence, so never fake counters or add another input or frame loop.`

const phaserRuntimeReference = `Phaser GameScene API (installed helper phaser-2 in src/common.ts). This is the complete public contract: do not read common.ts or vendor files to rediscover it.
Entry: src/main.ts imports {GameScene, start} from './common', declares class Main extends GameScene {...} and ends with start(Main, gravity). gravity is the Arcade y gravity (0 top-down, about 1000 for platformers). The logical canvas keeps the plan width and height.
Hooks (override these; never override preload, create or update):
- setup(): build the stage. Keep "if (this.setupScene()) return;" first when scene.json may hold nodes. Assign this.player. Reset custom cooldowns, spawn clocks and per-run flags here: restarts reuse the instance, so class field initializers do not run again.
- step(dt): per-frame gameplay. dt is in SECONDS (at most 0.05). Call super.step(dt) to keep built-in arrow/WASD movement and jumping.
- action(): primary action, called on every SPACE press. When overriding, increment this.state.actions yourself or call super.action().
- tick(): once per second while playing.
- paintHUD(): compact live status only, via this.hud.setText(...).
Fields:
- this.player: the controlled object; the validator binds to it, so assign it in setup().
- this.state: counters the validator reads: score, actions, hits, spawns, turns, ticks, lives, health, goal_remaining, outcome (0 playing, 1 won, 2 lost), ended. Change them only when the real event happens.
- this.elapsed: MILLISECONDS since the stage started; it resets on restart. Compare cooldowns in milliseconds.
- this.levelIndex: current stage (0-based); choose the layout from it in setup().
- this.inputKeys: down(...names) held, pressed(name) newly pressed, vector() gives {x,y} in -1..1 from arrows and WASD. Keys: LEFT RIGHT UP DOWN W A S D SPACE R P ESC ENTER. R restarts, P pauses and ESC ends the game. Other keys are not registered: use this.input.keyboard.addKey('F') for them.
- this.flow.message(text, seconds): short on-screen notice. this.hud is the HUD text object.
Helpers:
- body(x, y, w, h, color, fixed = false, role = ''): rectangle with an Arcade body (static when fixed) and the planned artwork for role; returns the GameObject. The collider stays w×h. role makes it a validation target: player, enemy, item, projectile, goal, obstacle or a custom role. Set object.__gmRole on objects created otherwise.
- configureLevels([{id, title}, ...]) declares stages; nextLevel() advances after a win; restartGame() returns to stage 0.
- configureWorld(width, height, follow = true): world and camera bounds, camera follows this.player.
- setCheckpoint(x, y); damagePlayer() consumes a life on real harmful contact with feedback and a protected respawn, ends the game at 0 lives and returns true when damage applied. Set this.state.lives in setup() for life-based games.
- feedback(name, object = this.player, material = 'flesh'): effect and sound for hit, pickup, shot, jump, land, damage, death, win, lose, interact and similar events. The validator counts feedback('pickup', item) as pickup_events.
- end(won): result screen with Continue or Restart.
- assetRoles(prefix): planned asset roles named prefix or prefix_*.
Physics: pass GameObjects returned by body() to this.physics.add.collider/overlap; register groups and overlaps once in setup(); movers use fixed = false. Arcade bodies have no setPosition: use object.setPosition(x, y) plus object.body.reset(x, y), or body.updateFromGameObject() for static bodies.
Checks: hits = this.state.hits, incremented inside the real collision callback; pickup_events = feedback('pickup', ...) calls; player_x/player_y come from this.player; stage_count = configured levels.`

const threeRuntimeReference = `Three.js startGame API (installed helper three-4 in src/common.ts). This is the complete public contract: do not read common.ts or vendor files to rediscover it.
Entry: src/main.ts imports {startGame} from './common' and calls startGame(config) once. The helper owns renderer, camera, player, input, clock, HUD, start/pause/result UI, presentation and disposal.
config:
- mode: fps, exploration, transport, flight or space (the plan base). goal: count to win. speed: metres per second. duration: seconds, 0 disables the countdown. lives: default 3.
- objects: [{role, at: [x, y, z], scale?, blocksShots?}] using planned asset roles. Built-in behaviour: item (collected within 1.5 m) and cargo (2 m, then carried) raise score, hits and pickup_events; goal collects like an item in flight and completes a cargo delivery in transport; enemy is hit by the primary action in fps and space (hits, score) and attacks the player within combat range; tree, building, obstacle and terrain are scenery that can block shots.
- levels: [{id, title, objects?, goal?, worldBounds?, mode?}, ...]; each entry overrides config for its stage. Only stages with a distinct layout count.
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
Counters: score, hits and pickup_events change only through the built-in role behaviour above or scene-builder rules; custom code cannot add to them. Give every checked target a real config.objects role.
Movement is camera-relative: D/RIGHT moves toward screen right and W/UP away from the camera.`
