package gamemaker

// Craft guides carry game-design direction that the technical phase guidance
// leaves out. They are part of the prepared prompt profile (the curated skill
// packages are registered, but their text is not injected into Studio runs).
// User requests always take precedence over these defaults.

// DesignCraftGuide steers planning toward a distinct, escalating experience.
const DesignCraftGuide = `Game design brief: plan for fun, not only validity. Decide and write concretely before set_design:
- Hook: one distinctive idea that shapes play (for example "the lantern reveals paths but drains while lit"), not a genre label.
- Stages: for levels or areas the player advances through, list 2–4 entries in design.stages; each names its layout and ONE new element (enemy behavior, hazard, mechanic or route) and raises difficulty. Waves or escalating rules inside one arena belong in features. An explicitly single-board game may omit stages.
- Opposition: at least two different behaviors (patrol, chase, shoot, swoop, timed hazard, moving platform). Static decoration is not a challenge.
- Feel: every contact gets visible and audible feedback (flash, knockback, particles, sound, shake); controls respond immediately.
- Rewards: pickups, combos, power-ups, unlocks or secrets worth a detour.
- Finale: a boss, escape, final puzzle or score chase, then a result screen with restart/continue.
Write each as a concrete feature sentence ("Stage 2 ridge: moving platforms and a charging boar"), never adjectives such as fun or exciting. Declare only what you will build: every feature and stage becomes an obligation that validation and repair hold you to. The user's explicit wishes override these defaults.`

// BuildCraftGuide keeps building and repair accountable to the accepted design.
const BuildCraftGuide = `Delivering the design:
- Implement every accepted feature and every stage; a feature that exists only in the plan is a defect. Before the final validation reread the accepted plan and add what is missing.
- Stages: each stage needs its own layout and new challenge (2D configureLevels plus layouts chosen by this.levelIndex in setup, or scene levels with level_id nodes; 3D config.levels with different objects). Validation counts distinct stages; cloned or renamed layouts do not count.
- Layouts: place objects deliberately (routes, cover, sightlines, safe spots, secrets), not in straight lines or regular grids; vary spacing and heights.
- Opposition moves and reacts in step(): patrols, chases, shots or timed hazards; difficulty rises per stage.
- Feel: feedback on every hit, pickup and death, brief invulnerability after damage, a readable compact HUD.
- Controls: RIGHT/D moves toward screen right, LEFT/A toward screen left, W/UP up in 2D or away from the camera in 3D. Free Three.js code derives movement from the camera's right/forward vectors. Validation fails swapped controls.`
