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

## Rune Workshop, Audio & Local Saves

- **Audio**: Sound effects and biome ambience are enabled by default (`Sound` toggle in the top bar or Pause menu) and unlock on the first keyboard, mouse, or touch interaction using the `aurago-sounds` pack.
- **Saves**: Coins earned from enemies, crates, chests, and stage completions can be spent in the **Rune Workshop** (`Shop`) across four persistent upgrade tracks (`Vitality`, `Rune Might`, `Winged Boots`, `Alchemy`). All progress is stored locally in `localStorage` under `aurago.runenpfad.v1`.
