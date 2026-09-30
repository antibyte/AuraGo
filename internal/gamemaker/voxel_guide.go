package gamemaker

// voxelDefinitionRules apply while planning and while editing the definition.
const voxelDefinitionRules = `## Voxel definition
- Voxel v1 is a finite first-person sandbox driven by src/voxel.json. The mode, survival (default) or creative, is chosen from the request and fixed during play. Size defaults to [96,48,96], at most [128,64,128], in multiples of 16; terrain is flat, hills or island with a bounded seed.
- Preserve valid defaults unless the request changes them. For an explicitly peaceful request, remove all enemies; keep the wood→stone→metal mining and crafting progression unless asked to change it.
- At most 64 block/item/recipe identities and 24 enemies. IDs and materials are storage identities. Plan acceptance and file edits validate the same definition.
- The definition is an implementation: keep concrete requested recipes, enemies and goals executable, not prose or renamed labels. Optional goals use collect/craft/place with an item, or defeat without an item. No forced timer, combat or victory for peaceful sandboxes.`

// voxelGoalExample shows the goal shape inside the definition; tests validate
// it against the default palette in both modes.
const voxelGoalExample = `"goals":[{"id":"shelter","kind":"place","item":"planks","count":20},{"id":"tools","kind":"craft","item":"stone_tool","count":1}]`

// VoxelPlanningGuide is the voxel planning contract.
const VoxelPlanningGuide = voxelDefinitionRules + `
## Voxel planning
- Use base="voxel" and send design.voxel as a JSON-encoded string in native set_design; schema 5 is server-owned. Start from inspect.design_example.voxel for palette, items, recipes and enemies.
- Goals are optional entries of the definition, at most 8, for example ` + voxelGoalExample + `. id is lowercase snake_case; kind is collect, craft or place with an existing item id, or defeat without item; count is 1–10000 and must be reachable with the declared world and recipes.
- Built-in checks already measure movement, jumping, mining, crafting, placing, pause and combat; omit scenarios for them.
- Features the definition cannot express are implemented later with hooks in main.ts.`

// VoxelRuntimeGuide is the voxel editing contract, shared by building, repair
// and tool-free source generation.
const VoxelRuntimeGuide = voxelDefinitionRules + `
## Voxel runtime
- Edit src/voxel.json with its current sha256, then check build.ok and validate. Invented mechanics in features must be implemented with hooks.
- main.ts imports definition from './voxel.json' and startVoxelGame from './common'. startVoxelGame(definition,{objective,setup(api),step(api,dtSeconds),action(api,target),dispose(api)}) owns the clock, mesh/physics, grid rays, resource/tool requirements, atomic inventory/crafting, enemies, start/pause, touch controls and persistence.
- action returning false suppresses the default mine/attack. Do not add another input or frame loop.
- api.getBlock(x,y,z), setBlock(x,y,z,id), transaction(take,give), count(item), craft(recipe), damagePlayer(amount), player/progress snapshots, scene, camera, renderer, event(name,point), dispose(). Only these APIs change the grid or inventory; never edit observations or counters.
- Scene decoration may use the existing model helper and must be disposed by the hook. World state, saves and chunk arrays never belong in prompts. Keep the 1 m voxel scale and bounded first-person controls.
- The intro precedes Start; compact status/hotbar during play. Desktop WASD/mouse/Space, primary mine/attack, secondary place, I inventory/crafting; E/F are keyboard alternatives for primary/secondary. Touch has a movement stick, look gesture and contextual actions. Inventory and pause freeze simulation.
- Studio saves use only the trusted host; do not add network or browser storage code. Exports already use IndexedDB.`
