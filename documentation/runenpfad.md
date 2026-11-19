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

- **Rune trails** first appear in Whisperwood. Look for the carved waystone. Touch the seal with the bright ring, then follow the thread and jump through the other seals in order. Short trails have three seals, longer ones have four. You have 12 seconds from the first touch. The ring shows how much time is left, and the counter shows how many seals you have collected. Finishing earns three coins. If time runs out, return to the first seal to try again. You can also head straight for the key and portal.
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

## Characters & Encounters

Six characters live in camps along the road. Walk up to one and a speech bubble appears. Some of them trade with you, give you quests or tips, and remember what you did. Any of them can be struck into a duel.

| Character | Faction | Found on levels | What they offer |
| --- | --- | --- | --- |
| Grimhild, Bog Witch | Coven | 2, 4 and 7 | The glow mushroom quest, potions, secrets and tips, a hex charm, fire rune orbs |
| Aldric, Tower Wizard | Circle | 17, 19, 24 and the finale on 25 | Rune lore and boss weaknesses, storm and frost runes, a duel challenge to strangers in world 4, the finale blessing |
| Fitz, Travelling Trader | Kobolds | 3, 9, 12, 18 and 23, and endless floors 2, 6, 10 and so on | The main shop, haggling, quest items for sale, possible fakes when the kobolds dislike you |
| Krax, Kobold Chief | Kobolds | 8 (guards a gate) and 13 | A toll, a hex charm or a fight to open the world 2 gate, and a return in world 3 |
| Saida, Desert Seer | Circle | 11 and 14 | Paid readings and puzzle hints, a riddle, the sun shard quest, the magnet rune |
| Brass, Clockwork Alchemist | Coven | 21, 23 and the finale on 25 | The gear quest, loose gears, workshop discount tokens, a weak spot in the citadel boss, the finale blessing |

- **Talking.** Stand next to a character and press `E`, or press `Up` while standing still (in the air or while running, `Up` jumps). The world pauses while you talk.
- **Replies.** Each conversation offers two to four replies. Pick one with `1` to `4`, or move with `Up` and `Down` and confirm with `Enter` or `Space`. You can also click. Replies you cannot meet (not enough coins, a missing item, too low a reputation) are hidden.
- **Text.** Lines type out one by one. A key press completes the current line first. With reduced motion, the text appears at once.
- **Leaving.** `Esc` closes a conversation that has a farewell reply. In a trade, `Esc` goes back to the conversation. If there is no way out, `Esc` opens the pause menu.
- **Reputation.** Three factions, Coven (Grimhild, Brass), Circle (Aldric, Saida) and Kobolds (Fitz, Krax), each run from `-5` to `+5` and start at `0`. Replies raise or lower them by one or two, and a floating label shows the change. The conversation shows your standing as a bar.
- **Memory.** Characters remember what you did, across stages and worlds. Grimhild can still hold a grudge from level 4 when you meet her again on level 7. Krax changes his tone once you have paid or beaten him. Each save keeps up to 64 remembered facts.
- **Walking off.** A character may leave the stage after a fight or a bad deal. They stay away for the rest of that stage only.
- **Striking.** Attacking a character in front of you (`J`, `Z` or `Enter`) costs their faction one point and starts a duel. This happens once per character in each stage.

### Quests and trade

- **Quests.** Grimhild wants three glow mushrooms from the woodland and the caverns, Saida three sun shards from the desert, and Brass three clockwork gears from the citadel. Walk over an item to pick it up; the counter shows `n/3`. Bring the items back to finish the quest.
- **Rewards.** Grimhild gives a healing potion, a tip about the stage's hidden gems and `+1` coven. Saida gives a magnet rune and `+1` circle. Brass gives two workshop discount tokens and `+1` coven.
- **Buying items.** While a quest is active, Fitz sells one quest item at a time for 20, 25 or 30 coins. Brass sells loose gears for 20 coins each.
- **Trade panel.** Open it from a character's trade reply and buy with the number keys. Each price moves with your reputation with that faction: up to 30 % cheaper at `+5`, up to 30 % dearer at `-5`.
- **Potions and runes.** A healing potion (Grimhild, Fitz, Brass) and a rune orb for the current stage (fire from Grimhild, storm from Aldric, magnet from Saida, frost from Brass; Fitz sells one kind per stage).
- **Lucky amulet.** Sold by Fitz and Krax. Every coin in your next stage is worth one more.
- **Discount token.** Sold by Fitz and Brass. The next Rune Workshop upgrade costs 30 % less. Each token pays for one upgrade.
- **Map charm.** Fitz and the seer sell it. For the rest of the stage, glowing rings mark the gems on screen and arrows at the screen edge point to the ones you have not found yet.
- **Rune attune.** Aldric sells it (up to 2 at a time). The next rune you pick up lasts 1.5 times longer, and one charm is used up.
- **Haggling.** Fitz sometimes agrees to 15 % off for the rest of the stage. You can ask once per stage.
- **Fakes.** When the kobolds think badly of you (`-2` or lower), Fitz may sell a fake: you pay and get nothing. You find out when you close the conversation. Confront him with "That was a fake!" to get half the price back, and the kobolds lose one point.

### Close-up duels

A duel moves the fight into a close-up arena. The screen wipes into a flat floor with bigger sprites, against the biome's painted backdrop. The character you fight has its own pattern of attacks and its own music. Each attack is announced first, so you can roll away from it, or block it where blocking works.

| Action | Keys |
| --- | --- |
| Move | `A` / `D` or `Left` / `Right` |
| Jump | `Space`, `W`, `Up` |
| Attack (Knight: three-hit combo, Ranger: bolts) | `J`, `Z`, `Enter` |
| Block (on the ground, facing the attack) | hold `Down` (or `S`) |
| Dash roll | `K`, `Shift`, `X` |
| Rune special (once per duel, needs a rune active when the duel starts) | `E` |

- **Rune specials.** Fire does 10 damage and pierces. Frost freezes the character for 1.6 seconds. Storm does 8 damage that cannot be blocked. Magnet clears incoming shots.
- **Potions** do nothing in a duel.
- **Attacks.** Hex bolts, toad curse clouds and broom dives (Grimhild); arcane missiles, a teleport strike and a rune shield that blocks your hits (Aldric); dagger flurries, bomb tosses and a smoke escape that reappears on your other side (Fitz); club smashes, war cries that summon kobolds and charges (Krax); sand vortices, mirage clones and sun beams (Saida); acid flasks, clockwork minions and steam vents (Brass).
- **Phase two.** Below half health, a character enrages for a moment, then attacks faster and more often.
- **Staggering.** Three quick hits make a character stagger.

### Duel stakes

- **Winning.** The first win against each character pays coins: Grimhild 20, Aldric 30, Fitz 15, Krax 25, Saida 20, Brass 30. Later wins pay nothing. The character then reacts in conversation.
- **Losing.** You lose one heart and return to the level with two seconds of invulnerability. A duel never ends the run.
- **No fleeing.** Once a duel starts, you fight until one of you falls. The arena walls hold you, and restarting, the level select, switching heroes and resetting progress are locked. `Esc` pauses the game, and resuming continues the fight.

### World 5 finale

- **Before the boss.** On level 25, Aldric and Brass meet you before the citadel boss. Their final blessing depends on how the circle and the coven see you. It gives coins, a rune, a heart or a potion, once per save.
- **Weak spot.** If you ask Brass how to stop the mech while your coven standing is `+2` or higher, and you have finished his gear quest, he reveals a loose seam. The boss then starts with a fifth less health.
- **Epilogue.** Beating the citadel boss on level 25 shows an epilogue line, friendly, feared or neutral, based on your reputation. Each faction at `+2` or higher pays 50 coins, once per save.

## Controls

| Action | Keys |
| --- | --- |
| Move / Climb Ladders and Ropes | `A` / `D` or `Left` / `Right`, `W` / `S` or `Up` / `Down` |
| Jump / Double Jump | `Space`, `W`, `Up` |
| Attack / Ground Slam | `J`, `Z`, `Enter` (`Down + Attack` mid-air for Knight) |
| Dash Roll (Ranger) | `K`, `Shift`, `X` |
| Healing Potion | `Q`, `H` |
| Pause / Restart Stage | `Esc` / `P`, `R` |
| Talk to a character | `E`, or `Up` while standing still |
| Choose a reply | `1` to `4`, or `Up` / `Down` and `Enter` |
| Duel: block | hold `Down` |
| Duel: rune special | `E` |

The Pause menu shows the main keys as a legend. Narrow viewports automatically reveal touch buttons.

## Presentation

- **Biome backdrops** (`biomes.js`, `backdrops.js`, `backdrops-biomes.js`, `atmosphere.js`): each world paints its own sky gradient, celestial body, several parallax silhouette layers (rolling ridges, forest crests, crystal shards, dunes, ice spires, clockwork gears), horizon haze, biome weather and a colour grade with vignette. The backdrop layers are rendered once and cached.
- **Dynamic 2D lighting**: a light map darkens the world in the caves and the citadel, then campfires, lava, runes, the portal, lanterns, projectiles and the hero punch holes back out of it.
- **Terrain detail**: grass and snow form continuous ledges. Below them, loose stones break up the soil texture and a depth gradient darkens the lower layers.
- **Decor** (`decor.js`): torches, lanterns, ledge caps, floating motes, swaying grass, shimmering lava and water ripples.
- **Character animation** (`actors.js`): the hero and enemies squash and stretch, dashes leave afterimages, hits flash, and defeated enemies dissolve.
- **Effects** (`fx.js`): deterministic particle system with dust, sparks, shards, embers, shockwave rings and rune glyphs, plus screen shake, colour flash and a short impact freeze.
- **GPU effects** (`postfx.js`): where WebGL2 is available, a post-processing pass adds bloom, god rays, shockwave distortion, heat haze, chromatic aberration, a per-biome colour grade, vignette and film grain. Switch it off with the `Graphics` button in the Pause menu. If the GPU pass cannot start, the game draws the plain 2D canvas instead.

## Audio & Local Saves

- **Rune trails and landings**: each seal plays a note from the current biome's scale. The last seal resolves the phrase. A landing uses the biome's ground sound and the hero's downward speed. These cues follow the Sound switch.
- **Sound effects and ambience**: enabled by default (`Sound` toggle in the top bar or Pause menu). They unlock on the first keyboard, mouse, or touch interaction using the `aurago-sounds` pack. Each biome mixes a primary and a secondary ambience bed (e.g. `forest-night` + `space-hum` in the caves), the master bus runs through a limiter, and repeated cues get a small random detune so they never machine-gun.
- **Adaptive music**: procedural music is enabled by default and can be switched off with the `Music on` / `Music off` button in the Pause menu. Each world has its own theme, and boss stages play a heavier boss theme. A duel with a character plays that character's own tense loop.
- **Pause menu**: `Continue`, `Restart Stage`, `Sound on` / `Sound off`, `Music on` / `Music off`, `Graphics: GPU effects` / `Graphics: classic canvas`, `Reset Progress`.
- **Saves**: Coins earned from enemies, crates, chests, and stage completions can be spent in the **Rune Workshop** (`Shop`) across four persistent upgrade tracks (`Vitality`, `Rune Might`, `Winged Boots`, `Alchemy`); each track shows its level as five pips. All progress is stored locally in `localStorage` under `aurago.runenpfad.v1`, including your sound, music and graphics preferences, and your standing with the characters (reputation, what they remember, quests and items).

## Recent Fixes

- Fixed a crash caused by the bomber enemy.
- Fixed the Fragile Ground trial so crumbling platforms behave as described.
- Fixed Rune Power for the Ranger.
- Fixed coins being saved mid-level.
- Fixed the Darkness trial so its effect is visible.
- Fixed saved progress being replaced by a fresh profile every time the game was opened.
- Pickup and combat labels that float over the stage now appear in your language.
