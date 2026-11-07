# Goldfish Adventure Runtime

## Purpose

Built-in point-and-click adventure **Bitte nicht das Universum füttern**. The
approved content and acceptance scope lives in
`../../../../../plans/2026-11-06-goldfish-adventure.md`.

## Ownership

- `content*.js` owns authored rooms, actions, inventory, dialogue references,
  puzzle prerequisites and explicit state effects.
- `state.js` owns deterministic transitions and validated, bounded save state.
- `save.js` owns transactional IndexedDB slots, revisions, recovery and limits.
- `renderer.js` owns ImageGen asset loading, Canvas 2D staging and animation.
  `manifest.moments[actionId]` contains bounded beats with actor clip/position,
  optional camera and local effects. Each frame may use its own atlas source.
- `commands.js` (pure, no DOM) owns verb filtering, default actions and the sentence
  line; `speech.js` owns spoken lines, their timing and the speech log; `panel.js`
  owns the SCUMM-style bar (sentence line, verbs, inventory, conversation menus);
  `interactions.js` owns the puzzle menus and save/import/export dialogs.
- `app.js` owns window lifecycle, semantic DOM controls, persistence, travel and
  action wiring, and the menu/journal/hint/settings dialogs. It stays wiring only.
- The runtime owns `ui/css/desktop-app-goldfish-adventure.css`. The coordinator
  owns shared Desktop registration, bundled shell changes and integration checks.
- Art masters/prompts belong to `assets/goldfish-adventure/AGENTS.md`; runtime
  payloads live in `ui/img/goldfish-adventure/`. Story and interface strings use
  separate `ui/lang/goldfish/` and `ui/lang/goldfish-interface/` locale sections.

## Local Contracts

- Expose `window.GoldfishAdventureApp.render/dispose`; each window owns its
  asynchronous work. Pause hidden/minimized/inactive views, release resources on
  dispose, and reject stale completions. Sound starts only after a user gesture.
- `GoldfishContent` is declarative data: stable IDs and localization keys, with
  normalized room/hotspot bounds, prerequisites and explicit effects. No eval,
  dynamic scripts, remote assets or model-generated runtime story.
- `GoldfishState` is the only puzzle-state writer. Validate room reachability,
  ordered applied actions, branch exclusivity, inventory and known IDs. Apply
  rewards/consumption once, then present animation. A skipped scene cannot repeat
  an effect. Repeatable conversations do not repeat rewards.
- Saves use one IndexedDB transaction for revision comparison and write. Twelve
  named current slots plus prior revisions: 128 KiB/state and 4 MiB total payload.
  Never overwrite on conflict or failed import. Keep memory state and offer
  export on storage failure; only completed transactions count as saved.
- Three branch pairs fold back to shared flags; all eight routes support all
  three informed endings. No item-loss dead ends, timed answers or clue hunting
  by pixel. Required clues have text/keyboard-accessible equivalents.
- Serialize captured save snapshots and acknowledge only their state revision.
  A declined failed-save continuation rolls back the full action/room change.
  Refresh named checkpoints at each new commitment and preserve their prior copy.
  The close guard drains queued writes before deciding whether progress is dirty.
- Interaction follows `plans/2026-11-07-goldfish-lucasarts-controls.md`: left click
  walks to a hotspot and runs its default action (the actions without an `item`;
  several open a choice list in the bar), right click / long press / Shift+Enter
  is Ansehen, a verb or an inventory item armed in the bar applies to the next
  click, Enter on a focused hotspot equals left click. Esc skips active speech,
  else cancels an armed verb/item or choice menu, else opens the game menu. Hotspot
  and exit buttons stay semantic, focusable DOM over the canvas. Puzzle prompts are
  conversation menus in the bar; `GoldfishState.apply` alone judges the answer.
- Derive unread results from schema-1 `applied`/`seenLines` using action-time
  conditions. A line counts as read only when it was shown for its full timer or
  the player skipped it (click on the scene or Space on a line); Esc over a queue
  and any click that cancels an ambient queue never mark unread lines. Story queues
  (results, room entry, resume) block the scene until they end; ambient ones (look
  lines, item descriptions, refusals, blocked exits, wrong answers) yield to any
  click. Only story keys may enter `seenLines`: `goldfish.ui.*` lines (refusals,
  item hints) are never recorded. Continue/load/import recover pending result
  dialogue and unseen room entry without replaying rewards. Reading preferences
  (`autoAdvance`, `readSpeed`) live in `localStorage` only, never in the save.
  An explicit current-autosave reload adopts its read revision and unblocks auto;
  manual/prior loads only recover the selected slot's comparison revision.
  Refusing a failed new-game checkpoint restores the previous boundary.
- Coalesce scene/canvas ResizeObserver writes in animation frames; skip unchanged
  sizes and cancel pending callbacks on disposal.
- ImageGen supplies all original scene/character/prop/interface imagery. Native
  text and focus/hotspot indicators remain semantic UI. No placeholder scenery
  may pass final visual acceptance; generated stills are not finished animation.
- Strings use flat `goldfish.*` keys; the application loads the chosen story and
  interface locale before play. Ship genuine translations for all sixteen locales.
- CSS is scoped with `agfish-`; both Desktop themes keep readable controls while
  the authored scene palette stays stable. No native alert/confirm/prompt dialogs.

## Work Guidance

- Keep modules below the existing 1100-line limit; split by responsibility.
- Implementation uses Sol 6.1 at Max; Terra 6.1 owns review/support work.
- Workers own disjoint files and preserve unrelated work. Shared registrations,
  locale integration, guide updates and commits belong to the coordinator.
- Prove the final-quality opening before treating the complete game's playtime,
  frame pacing or animation quality as established.

## Verification

- Syntax, encoding and diff checks precede focused state/save/content tests.
- `node scripts/test-goldfish-app.mjs` checks failed-save rollback, refreshed
  checkpoints, both branches' hints and delayed writes with a real DOM.
- Check all 24 route/ending combinations, malformed/conflicting saves, and
  interrupted animations. Browser tests cover real asset loading, interaction,
  focus, both themes, motion settings and close/reopen lifecycle.
- Validate asset paths/hashes/alpha/atlas bounds and complete localization keys.
- Rebuild and check Desktop bundles after shared shell registration changes.

## Child DOX Index

None.
