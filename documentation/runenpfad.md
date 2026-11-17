# Runenpfad

**Runenpfad** is a built-in 2D action-platformer application for the AuraGo Virtual Desktop. It uses the Game Maker Studio 2D sprite and audio packs directly through `/api/game-maker/asset-packs/` without duplicating binary assets into `ui/`.

## Worlds & Campaign

- **25 Campaign Levels** across **5 Biomes** (`Emerald Woodland`, `Root Caverns`, `Sun-Dust Desert`, `Frost Peaks`, `Clockwork Citadel`).
- Every 5th level (`5`, `10`, `15`, `20`, `25`) culminates in a multi-phase **World Boss** guarding the stage key. Each boss opens with an intro, announces its attacks with visible telegraphs (ground marks, charges, pounds, beams, missiles and bullet rings) and enters a faster, more aggressive **phase 2** once it drops to half health.
- **3-Star Mastery** per level (up to 75 campaign stars):
  1. Complete the level by claiming the key and entering the exit portal.
  2. Collect all 3 hidden stage gems (`red_gem`, `blue_gem`, `green_gem`).
  3. Clear the stage within Par Time or take zero damage.
- **Win card**: lists these three goals and marks the ones you reached, then shows your time, the gems, the coins earned in the stage and the score. Beating your stored time adds a `New best time!` tag.
- **World Map** (`Worlds`): each world tab shows its stars out of 15 or `Locked`, and the tower tab shows your best floor. Stage tiles show the stage number, your stars and best time, and a `World Boss` tag on every fifth stage.
- **Endless Rune Tower**: unlocks procedurally generated stages seeded per floor after completing campaign milestones or selecting it from the World Map (`Levels` dialog).

## Rune Trials (per-level variety)

Every level from `2` onwards carries one deterministic **Rune Trial** that twists how the stage plays. The trial is shown in the HUD (`Rune Trial: …`) and cycles through six variants:

| Trial | Effect |
| --- | --- |
| Swift Foes | Enemies move 40 % faster and gain health |
| Fragile Ground | Crumbling platforms collapse much faster |
| Darkness | Ambient light fades; only the hero's rune keeps glowing |
| Greed | Coins are worth more, but extra enemies lie in wait |
| Storm | Moving platforms race by and the wind pushes the hero back |
| Blessing | More healing potions and hearts in the stage |

Level `1` stays a clean tutorial without a trial. Endless floors pick a trial per floor as well.

## Stage Variety

- **Variety zones** beyond the classic meadow, bridge and crate layouts:
  - `bounce_grove`: bounce mushrooms launch the hero up to high ledges.
  - `updraft_shaft`: wind columns lift the hero.
  - `conveyor_works`: conveyor belts carry a standing hero along.
  - `pendulum_hall`: swinging pendulum saws hurt the hero on their arc.
  - `secret_vault`: cracked walls hide a bonus area and break after enough hits.
  - `lantern_path`: unlit lanterns the hero lights by walking up to them. Lighting a whole group grants a heart. Lit lanterns cast a light pool, and unlit ones glow faintly.
  - `rope_chasm`: climbable ropes across chasms.
- **Classic zone archetypes**: `spike_gallery` (spike traps), `boulder_gallery` (falling stones that drop when the hero enters their lane), `rune_shrine` (a shrine that grants **Rune Power**, double damage for 12 s) and `ambush_den` (stalkers that wait in the dark and only charge when the hero comes close).
- **Timed spike traps**: spikes cycle hidden, rising and then armed for a short window, so you can time your run past them.
- **Ropes and ladders**: climb with the movement keys. Ropes slow horizontal movement more than ladders do, and the hero plays a rear-view climb animation while climbing.
- **Enemy roster** gained three behaviour archetypes: `stalker` (ambush), `sentry_bot` (stationary turret) and `bomber_drone` (rushes the hero and detonates on death, damaging everything nearby).
- **Combo multiplier**: every kill within 2.5 s of the previous one raises the score multiplier.
- **Collectibles** include `star` bonus pieces and extra heart drops granted by the Blessing trial.

## Heroes & Abilities

- **Wächter (Knight)**: higher vitality, melee cleave that breaks crates and deflects enemy projectiles, and a mid-air Ground Slam (`Down + Attack`).
- **Waldläuferin (Ranger)**: higher movement speed, native Double Jump, evasive Dash Roll (`Shift` / `K`), and ranged rune bolts.

Switch heroes with the `Hero` button in the top bar or in the Rune Workshop. Switching restarts the current stage, so once a run has progress the button asks you to click a second time to confirm. On the win card the switch applies to the next stage.

## Controls

| Action | Keys |
| --- | --- |
| Move / Climb Ladders and Ropes | `A` / `D` or `Left` / `Right`, `W` / `S` or `Up` / `Down` |
| Jump / Double Jump | `Space`, `W`, `Up` |
| Attack / Ground Slam | `J`, `Z`, `Enter` (`Down + Attack` mid-air for Knight) |
| Dash Roll (Ranger) | `K`, `Shift`, `X` |
| Healing Potion | `Q`, `H` |
| Pause / Restart Stage | `Esc` / `P`, `R` |

The Pause menu shows the main keys as a legend. Narrow viewports automatically reveal touch buttons.

## Presentation

- **Biome backdrops** (`biomes.js`, `backdrops.js`, `backdrops-biomes.js`, `atmosphere.js`): each world paints its own sky gradient, celestial body, several parallax silhouette layers (rolling ridges, forest crests, crystal shards, dunes, ice spires, clockwork gears), horizon haze, biome weather and a colour grade with vignette. The backdrop layers are rendered once and cached.
- **Dynamic 2D lighting**: a light map darkens the world in the caves and the citadel, then campfires, lava, runes, the portal, lanterns, projectiles and the hero punch holes back out of it.
- **Terrain detail**: fill tiles rotate through neighbouring textures, exposed edges get a sunlit rim and shaded flanks, platforms cast a shadow lip.
- **Decor** (`decor.js`): torches, lanterns, ledge caps, floating motes, swaying grass, shimmering lava and water ripples.
- **Character animation** (`actors.js`): the hero and enemies squash and stretch, dashes leave afterimages, hits flash, and defeated enemies dissolve.
- **Effects** (`fx.js`): deterministic particle system with dust, sparks, shards, embers, shockwave rings and rune glyphs, plus screen shake, colour flash and a short impact freeze.
- **GPU effects** (`postfx.js`): where WebGL2 is available, a post-processing pass adds bloom, god rays, shockwave distortion, heat haze, chromatic aberration, a per-biome colour grade, vignette and film grain. Switch it off with the `Graphics` button in the Pause menu. If the GPU pass cannot start, the game draws the plain 2D canvas instead.

## Audio & Local Saves

- **Sound effects and ambience**: enabled by default (`Sound` toggle in the top bar or Pause menu). They unlock on the first keyboard, mouse, or touch interaction using the `aurago-sounds` pack. Each biome mixes a primary and a secondary ambience bed (e.g. `forest-night` + `space-hum` in the caves), the master bus runs through a limiter, and repeated cues get a small random detune so they never machine-gun.
- **Adaptive music**: procedural music is enabled by default and can be switched off with the `Music on` / `Music off` button in the Pause menu. Each world has its own theme, and boss stages play a heavier boss theme.
- **Pause menu**: `Continue`, `Restart Stage`, `Sound on` / `Sound off`, `Music on` / `Music off`, `Graphics: GPU effects` / `Graphics: classic canvas`, `Reset Progress`.
- **Saves**: Coins earned from enemies, crates, chests, and stage completions can be spent in the **Rune Workshop** (`Shop`) across four persistent upgrade tracks (`Vitality`, `Rune Might`, `Winged Boots`, `Alchemy`); each track shows its level as five pips. All progress is stored locally in `localStorage` under `aurago.runenpfad.v1`, including your sound, music and graphics preferences.

## Recent Fixes

- Fixed a crash caused by the bomber enemy.
- Fixed the Fragile Ground trial so crumbling platforms behave as described.
- Fixed Rune Power for the Ranger.
- Fixed coins being saved mid-level.
- Fixed the Darkness trial so its effect is visible.
- Fixed saved progress being replaced by a fresh profile every time the game was opened.
- Pickup and combat labels that float over the stage now appear in your language.
