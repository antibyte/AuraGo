# Runenpfad

**Runenpfad** is a built-in 2D action-platformer application for the AuraGo Virtual Desktop. It uses the Game Maker Studio 2D sprite and audio packs directly through `/api/game-maker/asset-packs/` without duplicating binary assets into `ui/`.

## Worlds & Campaign

- **25 Campaign Levels** across **5 Biomes** (`Emerald Woodland`, `Root Caverns`, `Sun-Dust Desert`, `Frost Peaks`, `Clockwork Citadel`).
- Every 5th level (`5`, `10`, `15`, `20`, `25`) culminates in a multi-phase **World Boss** guarding the stage key.
- **3-Star Mastery** per level (up to 75 campaign stars):
  1. Complete the level by claiming the key and entering the exit portal.
  2. Collect all 3 hidden stage gems (`red_gem`, `blue_gem`, `green_gem`).
  3. Clear the stage within Par Time or take zero damage.
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

- **Zone archetypes** beyond the classic meadow/bridge/crate layouts: `spike_gallery` (animated spike traps), `boulder_gallery` (falling stones that drop when the hero enters their lane), `rune_shrine` (a shrine that grants **Rune Power** — double damage for 12 s) and `ambush_den` (stalkers that wait in the dark and only charge when the hero comes close).
- **Enemy roster** gained three behaviour archetypes: `stalker` (ambush), `sentry_bot` (stationary turret) and `bomber_drone` (rushes the hero and detonates on death, damaging everything nearby).
- **Collectibles** include `star` bonus pieces and extra heart drops granted by the Blessing trial.

## Heroes & Abilities

- **Wächter (Knight)**: higher vitality, melee cleave that breaks crates and deflects enemy projectiles, and a mid-air Ground Slam (`Down + Attack`).
- **Waldläuferin (Ranger)**: higher movement speed, native Double Jump, evasive Dash Roll (`Shift` / `K`), and ranged rune bolts.

## Controls

| Action | Keys |
| --- | --- |
| Move / Climb Ladders | `A` / `D` or `Left` / `Right`, `W` / `S` or `Up` / `Down` |
| Jump / Double Jump | `Space`, `W`, `Up` |
| Attack / Ground Slam | `J`, `Z`, `Enter` (`Down + Attack` mid-air for Knight) |
| Dash Roll (Ranger) | `K`, `Shift`, `X` |
| Healing Potion | `Q`, `H` |
| Pause / Restart Stage | `Esc` / `P`, `R` |

Narrow viewports automatically reveal touch buttons.

## Presentation

- **Biome backdrops** (`biomes.js` + `atmosphere.js`): each world paints its own sky gradient, celestial body, three parallax silhouette layers (rolling ridges, forest crests, crystal shards, dunes, ice spires, clockwork gears), horizon haze, biome weather and a colour grade with vignette.
- **Dynamic 2D lighting**: a light map darkens the world in the caves and the citadel, then campfires, lava, runes, the portal, projectiles and the hero punch holes back out of it.
- **Terrain detail**: fill tiles rotate through neighbouring textures, exposed edges get a sunlit rim and shaded flanks, platforms cast a shadow lip.
- **Effects** (`fx.js`): deterministic particle system with dust, sparks, shards, embers, shockwave rings and rune glyphs, plus screen shake, colour flash and a short impact freeze.

## Audio & Local Saves

- **Audio**: Sound effects and biome ambience are enabled by default (`Sound` toggle in the top bar or Pause menu) and unlock on the first keyboard, mouse, or touch interaction using the `aurago-sounds` pack. Each biome mixes a primary and a secondary ambience bed (e.g. `forest-night` + `space-hum` in the caves), the master bus runs through a limiter, and repeated cues get a small random detune so they never machine-gun.
- **Saves**: Coins earned from enemies, crates, chests, and stage completions can be spent in the **Rune Workshop** (`Shop`) across four persistent upgrade tracks (`Vitality`, `Rune Might`, `Winged Boots`, `Alchemy`). All progress is stored locally in `localStorage` under `aurago.runenpfad.v1`.
