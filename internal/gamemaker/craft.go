package gamemaker

// Craft guides carry game-design direction that the technical phase guidance
// leaves out. They are part of the prepared prompt profile (the curated skill
// packages are registered, but their text is not injected into Studio runs).
// User requests always take precedence over these defaults.

// DesignCraftGuide steers planning toward a distinct experience without
// imposing combat, progression or an ending on requests that do not ask for them.
const DesignCraftGuide = `Game design brief: plan for fun, not only validity. Decide and write concretely before set_design:
- Hook: one distinctive idea that shapes play (for example "the lantern reveals paths but drains while lit"), not a genre label.
- Stages: when the request calls for advancing levels or areas, list 2–4 entries in design.stages; each names its layout and one new mechanic, route, hazard or enemy behavior. Raise difficulty only for challenge games. Waves or escalation inside one arena belong in features. A single-board game or open world may omit stages.
- Challenge: when the request asks for danger, combat or a scored challenge, describe concrete obstacles and how they behave; use at least two distinct behaviors when the game supports them (for example patrol plus timed hazard). Static decoration is not a challenge. Do not add enemies or hazards to a peaceful request.
- Feel: controls respond immediately and important interactions get clear feedback. Use hit effects only when the game has hits; peaceful interactions can use animation, sound or UI feedback.
- Rewards and progression: specify pickups, unlocks, secrets or other rewards when they fit the request. A peaceful sandbox may instead focus on building, exploration or creative expression.
- Ending: define an ending and result/restart flow when the request asks for a finite challenge. Do not force a boss, escape, final puzzle, score chase, timer or ending into continuous play or a peaceful sandbox.
Write requested features as concrete sentences ("Stage 2 ridge: moving platforms and a charging boar"), never adjectives such as fun or exciting. Declare only what you will build: every feature and stage becomes an obligation that validation and repair hold you to. The user's explicit wishes override these defaults.`

// BuildCraftGuide keeps building and repair accountable to the accepted design.
const BuildCraftGuide = `Delivering the design:
- Implement every accepted feature and every stage; a feature that exists only in the plan is a defect. Before the final validation reread the accepted plan and add what is missing.
- Stages: if the accepted design has stages, each needs its own layout and new challenge (2D configureLevels plus layouts chosen by this.levelIndex in setup, or scene levels with level_id nodes; 3D config.levels with different objects). Validation counts distinct stages; cloned or renamed layouts do not count.
- Layouts: place objects deliberately (routes, cover, sightlines, safe spots, secrets), not in straight lines or regular grids; vary spacing and heights.
- Challenge behavior: implement the planned enemy and hazard behaviors, with readable reactions in step(); raise difficulty only across planned challenge stages. For peaceful designs, do not add enemies, combat, timers, lives, forced failure or an ending unless the accepted plan requests them; make the planned exploration, building and interactions work.
- Feel: give clear feedback for the interactions in the accepted design. Hits and deaths need feedback and recovery only when the game includes them; keep a readable compact HUD.
- Controls: RIGHT/D moves toward screen right, LEFT/A toward screen left, W/UP up in 2D or away from the camera in 3D. Free Three.js code derives movement from the camera's right/forward vectors. Validation fails swapped controls.`
