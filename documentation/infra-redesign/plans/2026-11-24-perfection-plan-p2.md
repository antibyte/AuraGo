# Infra perfection plan: package 2

Worktree: `C:\Users\Andi\Documents\repo\AuraGo\.worktrees\infra-perfection`. Paths below are relative to it. Baseline: `TestDesktopInfraMenusAndShortcutsBrowser` already fails on unchanged code. That failure does not come from any item here.

## A. Files and headroom

| File | Now | Added (est.) | After |
|---|---|---|---|
| ui/js/desktop/apps/infra-canvas.js | 931 | +70 | ~1001 |
| ui/js/desktop/apps/infra-gestures.js | 918 | +67 | ~985 |
| ui/js/desktop/apps/infra.js | 729 | +6 | ~735 |
| ui/js/desktop/apps/infra-inspector.js | 461 | +1 | 462 |
| ui/js/desktop/apps/infra-library.js | 418 | 0 (one argument changes) | 418 |
| ui/js/desktop/apps/infra-menus.js | 239 | 0 (one label changes) | 239 |
| ui/js/desktop/apps/infra-lanes.js | 248 | 0 (icons move between lists) | 248 |
| ui/css/desktop-app-infra-canvas.css | 211 | +12 | ~223 |
| ui/lang/desktop/*.json (16 files) | – | 9 keys each | – |

**No new module is needed.** Both big files stay about 100 lines under 1100. The edge auto-pan helper (`edgePan`, about 20 lines) lives in infra-gestures.js. The reveal helper and the mode banner live in infra-canvas.js, because they need the canvas DOM and `setView`. If a later package needs room, `edgePan` and `slotsAt` are the first things to move to an `infra-reveal.js`. That file would load right after infra-gestures.js in both the module-loader list (`ui/js/desktop/core/module-loader.js:180`) and the `files` list in `scripts/test-infra-ui.mjs:8`.

Node harness trap: `scripts/test-infra-ui.mjs:1500` builds a fake canvas with only `viewport, sceneData, cardEl, regionEl, moveTemp, clearTemp, setDropTarget, setLaneTarget, renderNow, getView`. Every new canvas call from gestures (`setMode`, `showDropSlot`, `ensureVisible`, `zoomStep`) must use optional chaining (`canvas.setMode?.(...)`), or the harness throws.

## B. Order of work

1. **2.1 + 2.2** (one reveal helper): canvas `uncovered()` and `reveal()`; `gestures.active()`; the selection route in infra.js; gestures `onFocusIn`.
2. **2.3 + 2.4 + 2.5 together**: all three change `startLibraryDrag` (paint and onEnd). 2.5 also changes `startCardDrag.compute`, `startMarquee.update` and `startConnect`.
3. **2.10 + 2.11 + 2.13 together**: the banner text "Esc beendet" is only true once 2.11 lands, and 2.13 relies on the banner for the P path.
4. **2.8 + 2.9**: the tools cluster in canvas.
5. **2.6, 2.7, 2.12, 2.17**: rendering and CSS.
6. **2.14, 2.15, 2.16**.

Cross-package dependencies:
- 2.14 relies on the 3.5 toast inside `dialogs.removeEdge`.
- 2.15 edits `paintArrange` in infra.js and `canvasItems` in infra-menus.js, which 3.11 also rewrites (glyphs and separators). Land one, then rebase the other. 3.11 must not set `textContent` over the label 2.15 computes.

## C. Items

### 2.1 Selection stays visible
Verdict: confirmed. The test that pins the map staying still is `ui/desktop_infra_topology_polish_browser_test.go:145-153` (leftmost card, 1500x970 window, docked inspector).

Touch: infra-canvas.js -> new `uncovered()` and `reveal()` next to `ensureVisible` (258); infra.js -> `route` case `'selection'` (631-634); infra-gestures.js -> return object (914).

Do:
1. **How the uncovered rectangle is found.** Use layout offsets, not bounding rects, so the panel-in animation's transform does not skew the result. The canvas section, the overlay inspector (absolute below 1100 px), the narrow library sheet (absolute below 760 px) and the import sheet (always absolute) all have `.infra-body` (`position: relative`) as their offsetParent. A docked inspector or library is not absolute, so it is skipped: the viewport has already shrunk for it.
   ```js
   const PAD = 24;
   // uncovered: the part of the viewport no overlay panel covers (overlay inspector, narrow library sheet, import sheet), viewport px.
   function uncovered() {
       const sz = size(), box = { left: 0, right: sz.w, top: 0, bottom: sz.h };
       for (const panel of [app.inspector?.el, app.library?.el, app.importer?.el]) {
           if (!panel || panel.hidden || !panel.offsetWidth || panel.offsetParent !== el.offsetParent || getComputedStyle(panel).position !== 'absolute') continue;
           const l = panel.offsetLeft - el.offsetLeft, r = l + panel.offsetWidth;
           if (r <= box.left || l >= box.right) continue;
           if (l <= box.left + 1) box.left = Math.max(box.left, r); else box.right = Math.min(box.right, l);
       }
       return box;
   }
   ```
2. **The reveal helper.** It moves the view by the smallest distance that gives 24 px of clearance. With `sideways`, it only acts when the card is actually cut, and only horizontally. Panels are full height, so no vertical move is needed.
   ```js
   function reveal(id, { sideways = false } = {}) {
       const region = last.regionRects.get(id), rect = region ? { x: region.x, y: region.y, w: Math.min(region.w, 320), h: 50 } : worldRect(id);
       const box = uncovered();
       if (!rect || box.right - box.left < 120) return false;
       const v = getView(), a = model.toScreen(v, rect), b = model.toScreen(v, { x: rect.x + rect.w, y: rect.y + rect.h });
       if (sideways && a.x >= box.left && b.x <= box.right) return false;
       const shift = (lo, hi, min, max) => (hi - lo > max - min - 2 * PAD ? (min + max - lo - hi) / 2 : lo < min + PAD ? min + PAD - lo : hi > max - PAD ? max - PAD - hi : 0);
       const dx = shift(a.x, b.x, box.left, box.right), dy = sideways ? 0 : shift(a.y, b.y, box.top, box.bottom);
       if (!dx && !dy) return false;
       setView({ x: v.x + dx, y: v.y + dy, zoom: v.zoom }, { animate: true });
       return true;
   }
   function ensureVisible(id) { reveal(id); }
   ```
   `setView({animate:true})` already uses the 200 ms `--ds-motion-duration-base` transition, and is instant when `app.animationsEnabled()` is false. Export `reveal`. Keep `ensureVisible`: the library, inspector and import callers keep working and now respect the panels.
3. Gestures: add `active: () => !!(gesture || moveMode || pinch)` to the returned object. Without it, a card drag (which selects at 276 before `beginInteraction`) or a marquee would pan the map mid-gesture.
4. infra.js selection case: as the **last** line, after `closeLibrarySheet`:
   `if (state.view !== 'table' && state.selected && !state.interacting && !gestures.active()) canvas.reveal(state.selected, { sideways: true });`
   This runs synchronously. `size()` reads `clientWidth`, which forces layout, so the narrower docked viewport is already known. `setView` stores the new `viewSize`, so the ResizeObserver adds no shift afterwards.

Tests:
(a) topology_polish 145-153 keeps passing: after fit the leftmost card has 48 px of clearance. Change only the comment at 145 to "…leaves the map where it is unless the card would be covered (2.1)".
(b) Add the new assertion in the same test after line 155:
- Mark the card with the largest `getBoundingClientRect().right` as `data-rightmost` and click it with the mouse.
- Wait for `inspectorOpen==='true'`, then call `infraPanelsCenter(page,"[data-rightmost]")`, which waits for the pan.
- Assert `r.right <= stage.right - 23 && r.left >= stage.left`, where stage is the `[data-stage]` rect.
- Press Escape and wait until the inspector is closed.

Keys: none.
Risk: other browser tests that click a card near the right edge and then reuse stale coordinates. Run the full `TestDesktopInfra*` suite.

### 2.2 Focus stays visible
Verdict: confirmed. Gestures `onFocusIn` (561-573) only sets the roving item. Arrow keys reveal through `moveFocus` -> `canvas.ensureVisible(next)` (498).

Touch: infra-gestures.js -> `onFocusIn` (571-572).

Do:
1. Replace line 572 with:
   ```js
   if (item) { const id = item.dataset.node || item.dataset.regionHeader; canvas.setRovingCard(id); if (target.matches?.(':focus-visible')) canvas.ensureVisible?.(id); }
   ```
   A pointer click does not match `:focus-visible` on buttons, so clicks never pan. Region headers reveal their header box: `reveal` handles region ids, see 2.1 step 2.
2. Keep the explicit `ensureVisible` in `moveFocus`. A second call finds nothing to shift.

Tests:
(a) none.
(b) Same test, after the 2.1 block:
- Dispatch `WheelEvent('wheel',{deltaX:-3000})` on `[data-stage]` so the cards leave the view.
- Focus `[data-tools] [data-tool="select"]`.
- Press Shift+Tab (`page.Keyboard.MustPress(input.ShiftLeft)`, `MustType(input.Tab)`, `MustRelease`).
- Settle, then assert `document.activeElement.matches('[data-node]')` and that its rect lies inside the `[data-stage]` rect.

Keys: none.
Risk: low. A programmatic focus right after a key press counts as focus-visible, which is the intended case.

### 2.3 Library drag preview
Verdict: confirmed. Ghost offset `+8` and `transform-origin: 0 0` (canvas 876, css 183); unclamped `scale: zoom` (gestures 715). The drop lands at the pointer with no snap, and `freeSlot` only nudges when blocked (751-752).

Touch: infra-canvas.js -> `createGhost.move` (874-876), scene markup (52-53), new `showDropSlot`; infra-gestures.js -> `startLibraryDrag` paint (707-717), onEnd, `finishLibraryDrop` (751), `positionsAt` (758-762); css 183, 188.

Do:
1. Ghost: `ghost.style.transform = \`translate(${clientX - rect.left}px, ${clientY - rect.top}px) translate(-50%, -50%) scale(${scale})\`;`. CSS 183: change `transform-origin: 0 0` to `50% 50%`. CSS 188: the `::after` label gets `left: 50%; transform: translateX(-50%)`.
2. Gestures paint: `scale: onMap ? Math.min(1.25, Math.max(0.5, canvas.getView().zoom)) : 1`.
3. New shared helper in gestures. It replaces the body of `positionsAt`, which import's `dropOnCanvas` uses, so the outline matches the landing everywhere.
   ```js
   // slotsAt: where count cards dropped at a world point land: snapped, spread by compactGrid, nudged into free slots (cards in moving do not block).
   function slotsAt(count, world, moving = null) {
       const obstacles = canvas.sceneData().cards.filter(card => !moving?.has(card.id)).map(model.cardRect), at = { x: model.snap(world.x), y: model.snap(world.y) };
       return (count === 1 ? [at] : model.compactGrid(count, at)).map(point => { const pos = model.freeSlot(point, obstacles); obstacles.push(model.cardRect(pos)); return { x: Math.round(pos.x), y: Math.round(pos.y) }; });
   }
   const positionsAt = (count, world) => slotsAt(count, world);
   ```
   `finishLibraryDrop` 751: `const points = slotsAt(ids.length, world, new Set(ids));`
4. Canvas markup: put `<div class="infra-drop-slot" data-drop-slot hidden></div>` inside `[data-scene]` between the `svg` and `[data-nodes]`. Add:
   `function showDropSlot(pos) { slotEl.hidden = !pos; if (pos) { slotEl.style.left = px(pos.x); slotEl.style.top = px(pos.y); } }` and export it.
5. Paint: `canvas.showDropSlot?.(onMap && !guestsOnly ? slotsAt(ids.length, info.world, mode === 'move' ? new Set(ids) : null)[0] : null);`. In onEnd: `canvas.showDropSlot?.(null)`.
6. CSS:
   `.infra-drop-slot { position: absolute; width: 216px; height: 64px; box-sizing: border-box; transform: translate(-50%, -50%); border: var(--infra-ring, 2px) dashed var(--vd-accent); border-radius: var(--vd-mat-card-radius, var(--ds-radius-md)); background: color-mix(in srgb, var(--vd-accent) 10%, transparent); pointer-events: none; }`

Tests:
(a) none known. Library drops are now snapped to the 24 px grid; any assertion that pins an unsnapped drop x/y must take the snapped value. I could not check every library-drop assertion.
(b) `ui/desktop_infra_import_browser_test.go:142`: after the wait, assert that the ghost rect centre is within 1 px of the last mouse point and that `[data-drop-slot]` is not hidden.

Keys: none.
Risk: per-frame cost of `slotsAt` on very large maps (it filters every card each frame). Acceptable below about 1000 cards.

### 2.4 Invalid drop
Verdict: confirmed. `invalid = !placeTarget && !over && !onLibrary` (714). `target` is the library element (`event.currentTarget`, library 311), so the whole sidebar counts as valid, and a drop there does nothing silently (738-739).

Touch: infra-gestures.js -> `startLibraryDrag` (705, 714, 738-739).

Do:
1. `const invalid = !info.placeTarget && !info.over;`. Remove `onLibrary` from `probe`.
2. onEnd: after the two `finishLibraryDrop` branches add `else app.announce(t('desktop.infra_drop_cancelled'));`. This covers Escape (root keydown aborts the gesture), a cancel, and a release off the map. Announcement only, per the feedback rule.

Tests:
(a) none.
(b) import test, after line 142: move the mouse back over `[data-import-sheet]`, wait for `.infra-drag-ghost.is-invalid`, release, and wait for `[data-announce]` text `=== w('desktop.infra_drop_cancelled')`.

Keys: `desktop.infra_drop_cancelled | Ziehen abgebrochen | Drag cancelled`
Risk: the ghost is red for the first frames while it is still over the library. That is now intended.

### 2.5 Edge auto-pan
Verdict: confirmed. There is no auto-pan, and card drag and marquee compute from screen deltas against a fixed view (264, 237-240).

Touch: infra-gestures.js -> new `edgePan` after `begin` (80); `startCardDrag`, `startMarquee`, `startConnect`, `startLibraryDrag`.

Do:
1. The helper. It arms only after the pointer has been well inside the viewport once, so crossing the band from the library does not pan.
   ```js
   const EDGE_BAND = 36, EDGE_SPEED = 18;
   // edgePan: inside the 36 px band at a viewport edge, pan towards it every frame (faster nearer the edge), then step().
   function edgePan(step) {
       let point = null, id = 0, armed = false;
       const speed = d => (d >= 0 && d < EDGE_BAND ? Math.ceil(EDGE_SPEED * (1 - d / EDGE_BAND)) : 0);
       const tick = () => {
           id = 0;
           if (!point) return;
           const r = viewport.getBoundingClientRect(), d = [point.x - r.left, r.right - point.x, point.y - r.top, r.bottom - point.y];
           if (!armed) { armed = d.every(v => v >= EDGE_BAND); return; }
           const dx = speed(d[0]) - speed(d[1]), dy = speed(d[2]) - speed(d[3]);
           if (!dx && !dy) return;
           const v = canvas.getView(); canvas.setView({ x: v.x + dx, y: v.y + dy, zoom: v.zoom });
           step(); id = requestAnimationFrame(tick);
       };
       return { update(ev) { point = { x: ev.clientX, y: ev.clientY }; if (!id) id = requestAnimationFrame(tick); }, stop() { point = null; if (id) cancelAnimationFrame(id); id = 0; } };
   }
   ```
2. `startCardDrag`:
   - Keep `v0 = canvas.getView()` at the start.
   - `compute` subtracts the pan: `const v = canvas.getView(), dx = model.snap(o.x + (ev.clientX - sx - (v.x - v0.x)) / zoom) - o.x` (same for y).
   - `const pan = edgePan(() => { if (lastEv) { compute(lastEv); paint.request(); } })`. In move: `lastEv = ev; pan.update(ev)`. In onEnd: `pan.stop()` first.
3. `startMarquee`: anchor the start in world space. `const startWorld = model.toWorld(canvas.getView(), { x: sx - rect.left, y: sy - rect.top })`, and in `update`: `const a = model.toScreen(canvas.getView(), startWorld)`. The pan step calls `update(lastEv)`.
4. `startConnect`: move the move body into `const show = ev => {…}`. The pan step calls `show(lastEv)`.
5. `startLibraryDrag`: the pan step is `paint.request()`, because `probe` uses `clientToWorld`. Call `pan.update(ev)` after `started`, and `pan.stop()` in onEnd.

Tests:
(a) none.
(b) `ui/desktop_infra_canvas_browser_test.go`, after the redo block (~145):
- Press on a card and move to `stage.right - 8` with the button still down.
- Wait about 300 ms, then assert the scene's `translate` x has decreased.
- Release, and assert the saved x is greater than the drop x would have been without the pan.

Keys: none.
Risk: drags in existing tests that end inside a 36 px band now pan a few frames. rod moves are instant, so the effect is small.

### 2.6 Compact names
Verdict: confirmed: the compact name uses `display:-webkit-box; -webkit-line-clamp:2` (css 70).

Measured problem (my arithmetic from `zoomLevels`, not observed live): at 47 % the counter-scaled one-line chip is about 35 world px tall, while the gap down to a card at the minimum 72 px pitch is 8 px. So "one line" alone cannot meet "never taller than the space". Adjusted: one line, plus the existing A5 pattern (hide what would clash).

Touch: css 63 (comment), 69-71; infra-canvas.js -> `paintLevels` (177-196), `renderNow` (675), new `paintNameClash`.

Do:
1. CSS 69: `width: 232px`. CSS 70: replace `white-space: normal; text-overflow: clip; display: -webkit-box; -webkit-box-orient: vertical; -webkit-line-clamp: 2;` with `display: block; white-space: nowrap; text-overflow: ellipsis;`. The base `max-width: calc(100% / k)` now caps the scaled chip at 232 world px, which is below the 240 px column pitch. Delete line 71 (the tiny rule, now redundant).
2. Add CSS:
   `.infra-viewport.is-compact .infra-card.is-name-clash:not(:hover, :focus-visible, [aria-pressed="true"]) .infra-card-name { visibility: hidden; }`
   `.infra-viewport.is-compact .infra-card:is(:hover, :focus-visible) { z-index: 3; }` (z-index only, no transform).
3. Canvas: store `nameK = levels.name` in `paintLevels` and call `paintNameClash()` inside the changed-key branch. Also call it in `renderNow` after `patchCards`. Read all widths first, then write:
   ```js
   // A compact name that would run into another card hides until its card is hovered, focused or selected (as A5 labels do).
   function paintNameClash() {
       const on = cardForm === 'compact', rects = on ? last.cards.map(model.cardRect) : [];
       const widths = on ? last.cards.map(card => Math.min(232, (cardEls.get(card.id)?.card.querySelector('.infra-card-name')?.offsetWidth || 232) * nameK)) : [];
       last.cards.forEach((card, i) => {
           const entry = cardEls.get(card.id);
           if (!entry) return;
           const box = on ? { x: card.x - widths[i] / 2, y: card.y + model.CARD_H / 2 + 4, w: widths[i], h: 22 * nameK } : null;
           setFlag(entry, 'nameClash', !!box && rects.some(r => model.intersects(box, r)), v => entry.card.classList.toggle('is-name-clash', v));
       });
   }
   ```
   22 = 16 px × 1.25 line height + 2 px padding. The full name is already the card's `title` (canvas 282).

Tests:
(a) `ui/desktop_infra_polish_browser_test.go:100-103` pins `"lines":"2"`. Change the probe to `lines:getComputedStyle(card.querySelector('.infra-card-name')).whiteSpace`, the want to `"lines":"nowrap"`, and the message to "…shows one-line names…".
(b) Same test, 28 % probe (85-91): add `clash: names.filter(el=>getComputedStyle(el).visibility!=='hidden').some(el=>{const r=el.getBoundingClientRect(),own=el.closest('.infra-card');return [...document.querySelectorAll('[data-nodes] .infra-card')].some(c=>c!==own&&hit(r,c.getBoundingClientRect()));})` and expect `"clash":false`. The topology `namesProbe` tests stay valid: lanes are 144 px apart.

Keys: none.
Risk: O(n²) per render in compact form. Hidden names on tight stacks are a deliberate trade-off, see D.

### 2.7 Guest badge and connection labels
Verdict: confirmed: `blockersOf` lists card rects only (canvas 472-475). The badge hangs from card.y+22 to card.y+46, which is 14 px below the box.

Touch: infra-canvas.js -> `blockersOf` (472-475).

Do:
1. Change it to:
   ```js
   // A host's guest badge (translate 22 px, 24 px high) hangs 14 px below its card and blocks labels too (2.7).
   return () => rects || (rects = last.cards.map(card => { const r = model.cardRect(tempCards?.get(card.id) || card); return card.guests.length ? { ...r, h: r.h + 14 } : r; }));
   ```
   This is conservative: it uses the full card width, since the badge is narrower than the card.

Tests:
(a) `ui/desktop_infra_polish_browser_test.go:223` may change if a fixture host's badge now covers the fibre label. I could not verify the fixture's guests. If the want changes, every row must still have `overlap:false`.
(b) Same probe (216): extend `cards` to `'[data-nodes] [data-node], [data-nodes] .infra-card-badge'`, so a visible label over a badge fails `overlap`.

Keys: none.
Risk: a few labels near hosts now hide until hover.

### 2.8 Zoom limits
Verdict: confirmed: `zoomBy` clamps silently and the buttons are never disabled (canvas 237-241, 757-774).

Touch: infra-canvas.js -> refs (94), `paintView` (149-164), `setView` (209), new `zoomStep`; infra-gestures.js -> keys (541-542).

Do:
1. Add refs `zoomInButton`, `zoomOutButton`.
2. In `paintView`, every frame, write only when the flags change. Do not tie this to `pctShown`: the last step to 25 % or 200 % can keep the same rounded percentage.
   ```js
   const atMax = model.clampZoom(v.zoom * 1.2) <= v.zoom, atMin = model.clampZoom(v.zoom / 1.2) >= v.zoom, lk = `${atMax}|${atMin}`;
   if (lk !== limitsKey) { limitsKey = lk; for (const [b, at, key, lim] of [[zoomInButton, atMax, 'desktop.infra_zoom_in', 'desktop.infra_zoom_limit_max'], [zoomOutButton, atMin, 'desktop.infra_zoom_out', 'desktop.infra_zoom_limit_min']]) { if (at && document.activeElement === b) { rovePrimary(zoomButton); zoomButton.focus({ preventScroll: true }); } b.disabled = at; b.title = t(at ? lim : key); } }
   ```
3. Keyboard step:
   ```js
   function zoomStep(dir) {
       const before = getView().zoom; zoomBy(dir > 0 ? 1.2 : 1 / 1.2);
       if (getView().zoom !== before) return;
       const which = dir > 0 ? 'max' : 'min';
       if (limitSaid !== which) { limitSaid = which; app.announce(t(`desktop.infra_zoom_limit_${which}`)); }
   }
   ```
   In `setView`, reset `limitSaid = ''` when the zoom changes. Export `zoomStep`.
4. Gestures 541-542: `canvas.zoomStep(1)` and `canvas.zoomStep(-1)`.

Tests:
(a) none. `.click()` on a disabled button is a no-op, so the 8 clicks at `ui/desktop_infra_a11y_browser_test.go:388` and polish 79-82 still work.
(b) polish 82: click zoom-out twice more, wait for `infraZoomOf()===0.25`, and assert `[data-action="zoom-out"]` is disabled with `title===fixtureWords['desktop.infra_zoom_limit_min']`.

Keys:
`desktop.infra_zoom_limit_max | Größte Zoomstufe erreicht (200 %) | Largest zoom reached (200%)`
`desktop.infra_zoom_limit_min | Kleinste Zoomstufe erreicht (25 %) | Smallest zoom reached (25%)`

Risk: focus loss when the focused button is disabled; handled in step 2.

### 2.9 Minimap switch
Verdict: confirmed (canvas 703-704, 766-770).

Touch: infra-canvas.js -> `onClick` `'toggle-minimap'` (766-771).

Do:
1. After `app.setPref`: `if (on && model.sceneFits(last.bounds, getView(), size())) app.toast(t('desktop.infra_minimap_fits'), 'info');`. Switching it off stays visible through the pressed style, so it needs no toast.

Tests:
(a) none. Polish 186-189 runs where the minimap is shown, so the scene does not fit and no toast appears.
(b) Same test: click `fit`, click the toggle twice (off, then on), and wait for the `[data-toast] span` text `=== fixtureWords['desktop.infra_minimap_fits']`.

Keys: `desktop.infra_minimap_fits | Die ganze Karte ist sichtbar. Die Übersichtskarte erscheint beim Hineinzoomen | The whole map is visible. The overview map appears when you zoom in`
Risk: none.

### 2.10 Mode banner
Verdict: confirmed: move mode is only announced (624), and the Connect tool has no hint.

Touch: infra-canvas.js -> markup after the chip (70), `applyTool` (749-756), new `setMode`; infra-gestures.js -> `enterMoveMode` (623), `finishMoveMode` (647); css.

Do:
1. Markup, after `[data-layout-chip]`: `<div class="infra-mode-banner" data-mode-banner aria-hidden="true" hidden></div>`. It is aria-hidden because the move modes are already announced.
2. Canvas:
   ```js
   let modeKind = '';
   function paintMode() { const text = modeKind === 'move' ? t('desktop.infra_mode_move') : store.state.tool === 'connect' && store.canWrite() ? t('desktop.infra_mode_connect') : ''; bannerEl.hidden = !text; if (bannerEl.textContent !== text) bannerEl.textContent = text; }
   function setMode(kind) { modeKind = kind || ''; paintMode(); }
   ```
   Call `paintMode()` at the end of `applyTool`, and export `setMode`.
3. Gestures: `canvas.setMode?.('move')` after `store.beginInteraction()` in `enterMoveMode`, and `canvas.setMode?.('')` in `finishMoveMode` next to `clearTemp`.
4. CSS:
   `.infra-mode-banner { position: absolute; top: var(--ds-space-3); left: 50%; z-index: 7; transform: translateX(-50%); max-width: calc(100% - var(--ds-space-6)); overflow: hidden; padding: 4px var(--ds-space-3); border: 1px solid var(--vd-theme-accent-border); border-radius: var(--ds-radius-full); background: var(--vd-theme-panel-bg-strong); -webkit-backdrop-filter: var(--vd-theme-blur, blur(12px)); backdrop-filter: var(--vd-theme-blur, blur(12px)); box-shadow: var(--vd-theme-shadow-panel); color: var(--vd-text); font: 600 13px/1.5 var(--vd-font-ui); text-overflow: ellipsis; white-space: nowrap; pointer-events: none; }`
   `.infra-canvas-chip:not([hidden]) ~ .infra-mode-banner { top: calc(var(--ds-space-3) + 48px); }`

Tests:
(a) none.
(b) `ui/desktop_infra_canvas_browser_test.go:190`: split `MustType` so that after `KeyM` the test asserts `[data-mode-banner]` is visible with the move text and has `pointerEvents==='none'`. After Enter, assert it is hidden.

Keys:
`desktop.infra_mode_connect | Von Asset zu Asset ziehen · Esc beendet | Drag from asset to asset · Esc ends`
`desktop.infra_mode_move | Mit Pfeiltasten verschieben · Enter übernimmt · Esc bricht ab | Move with the arrow keys · Enter applies · Esc cancels`

Risk: an overlay inspector can cover the banner in narrow windows (cosmetic).

### 2.11 Escape and tools
Verdict: confirmed (gestures 514-517).

Touch: infra-gestures.js -> `onKeyDown` (514-517).

Do:
1. Replace with:
   ```js
   if (key === 'Escape') {
       if (gesture) { gesture.abort(); event.preventDefault(); }
       else if (state.tool !== 'select') { event.preventDefault(); store.setTool('select'); }
       else store.clearSelection();
       return;
   }
   ```

Tests:
(a) none. topology_polish 154 presses Escape with the Select tool, so it still clears the selection.
(b) `scripts/test-infra-ui.mjs`, after the move-mode block (~1535): `w = world({nodes:[card('a',0,0)], selection:['a'], focus:'a'})`; `w.store.setTool('connect'); w.press('Escape')` -> tool `'select'`, selection size 1; `w.press('Escape')` -> selection size 0.

Keys: none.
Risk: none.

### 2.12 Connect cursor
Verdict: adjusted. `grabbing` while dragging a card already exists (`.infra-app.is-dragging *`, css 195; the class is set at gestures 281). Only the crosshair is missing (card `cursor: default`, css 38).

Touch: desktop-app-infra-canvas.css after line 13.

Do:
1. `.infra-viewport[data-tool="connect"]:not(.is-readonly) .infra-card { cursor: crosshair; }`
2. `.infra-viewport[data-tool="hand"] :is(.infra-card, .infra-region-header) { cursor: grab; }` (the Hand tool pans over cards too).

Tests:
(a) none.
(b) canvas test next to the 2.10 check: click `[data-tool="connect"]`, assert `getComputedStyle(dev-a card).cursor==='crosshair'`, then click `[data-tool="select"]`.

Keys: none.
Risk: none.

### 2.13 Placing with the plus button
Verdict: confirmed. Library 292 calls `placeAtCenter(..., { moveMode: true })` on a pointer click, and the next pointer press cancels the move (gestures 410 -> 654 announce).

Touch: infra-library.js -> 292; infra-inspector.js -> 375.

Do:
1. Library 292: `placeAtCenter([place.dataset.place])` with no move mode. The "+" is `tabindex=-1 aria-hidden`, so it is pointer-only. P (336) keeps `{ moveMode: true }` and now shows the 2.10 banner.
2. Inspector 375: `{ moveMode: event.detail === 0 }`. Keyboard activation of a button has `detail` 0.

Tests:
(a) none found that pins move mode after "+".
(b) canvas test after the drop at line 123, or the library test: `document.querySelector('[data-place="…"]').click()` on another To-place row, then assert the card exists, `[data-mode-banner]` is hidden, and `state` shows no interaction (`.infra-app` has no `is-dragging`).

Keys: none.
Risk: none.

### 2.14 Delete on a connection
Verdict: confirmed (gestures 532-539 only look at the selection or the focused item).

Touch: infra-gestures.js -> `onKeyDown` Delete case (532-539).

Do:
1. After `if (!store.canWrite()) break;` add `if (!state.selection.size && state.selectedEdge) { event.preventDefault(); void app.dialogs.removeEdge(state.selectedEdge); break; }`. `selectEdge` empties the selection (store 403), and the edge wins over a merely focused card. `removeEdge` keeps its confirm dialog and gets the 3.5 toast.

Tests:
(a) none.
(b) Node harness: add `app` to the object `world` returns (1508). Then: `w.store.state.edges=[{id:'e1',from:'a',to:'b',kind:'ethernet'}]; w.store.selectEdge('e1'); const removed=[]; w.app.dialogs.removeEdge=id=>{removed.push(id);return Promise.resolve(true);}; w.press('Delete')` -> `removed` equals `['e1']`.

Keys: none (the toast text belongs to 3.5).
Risk: until 3.5 lands, the result is only announced.

### 2.15 Arrange scope
Verdict: confirmed. gestures 830-837 decide the scope. Labels: infra.js 268 (`paintArrange`), 405 (window menu), inspector 192, and menus 112 (canvas context menu `arrange_auto` in the Map).

Touch: infra-gestures.js -> new `arrangeScope`, `arrange` (830-837); infra.js -> `paintArrange` 268, `syncMenus` 405, selection case 632; infra-inspector.js 192; infra-menus.js 112.

Do:
1. Gestures:
   ```js
   // arrangeScope: what Arrange acts on: 'topology' (hand back to auto), 'selection' (several drawn cards of one parent) or 'map'.
   function arrangeScope() {
       const view = app.mapView();
       if (view === 'topology') return 'topology';
       const scene = canvas.sceneData(), picked = [...state.selection].filter(id => scene.cardById.has(id));
       return picked.length > 1 && new Set(picked.map(id => store.effectiveParent(id, view))).size === 1 ? 'selection' : 'map';
   }
   ```
   `arrange()` uses it: `void app.arrange.arrange('connections', arrangeScope() === 'selection' ? { ids: picked } : {})`. Export `arrangeScope`.
2. infra.js: `const ARRANGE_KEYS = { topology: 'desktop.infra_arrange_auto', selection: 'desktop.infra_arrange_selection', map: 'desktop.infra_arrange_map' };` and `app.arrangeLabelKey = () => ARRANGE_KEYS[gestures.arrangeScope()]`. `paintArrange` uses `t(app.arrangeLabelKey())`, and `syncMenus` uses `labelKey: app.arrangeLabelKey()`. Add `syncMenus()` to the selection case.
3. Inspector 192: `t(app.arrangeLabelKey())` instead of the ternary.
4. Menus 112: `label: t(app.mapView() === 'topology' ? 'desktop.infra_arrange_auto' : 'desktop.infra_arrange_map')`. This always arranges the whole map. The asset menu (62) already reads "Auswahl anordnen".

Tests:
(a) `ui/desktop_infra_topology_browser_test.go:187` stays (Topology keeps `arrange_auto`).
(b) `ui/desktop_infra_arrange_browser_test.go`: with nothing selected the more-menu `[data-action="arrange"]` text equals `infra_arrange_map`; after selecting two cards of one parent it equals `infra_arrange_selection`.

Keys: `desktop.infra_arrange_map | Ganze Karte anordnen | Arrange whole map` (reuses the existing `infra_arrange_selection`).
Risk: merge conflict with 3.11 (see B).

### 2.16 Topology lanes
Verdict: confirmed. `cloud-service` is a real icon id (model line 5) but appears in no lane list, so it falls to clients. `sbc` sits in compute (lanes 27), pinned by `scripts/test-infra-ui.mjs:1781`. The AGENTS.md lane notes (A21) name servers, storage, NVR, database, UPS and PDU, not single-board computers, so they give no reason to keep it there.

Touch: infra-lanes.js -> `LANE_ICONS` (24-28); ui/js/desktop/apps/AGENTS.md lane bullet (~361).

Do:
1. `internet: ['internet', 'modem-ont', 'cloud-service']`.
2. Remove `'sbc'` from compute. Do not add it to clients: it then falls through to role words and "has guests" (a Pi running containers stays compute, a bare sensor Pi becomes a client), and to clients otherwise.
3. AGENTS.md: "…modem-ont and cloud-service internet; a single-board computer goes by its role or guests".

Tests:
(a) mjs 1781: remove `'sbc'` from the server loop.
(b) mjs after 1784: `assert.equal(lane('cloud-service'), 'internet'); assert.equal(lane('sbc'), 'clients'); assert.equal(lane('sbc', {}, { hasGuests: true }), 'compute');`. The crossing counts (1908-1909) are unaffected: the lab fixture has no sbc or cloud-service.

Keys: none.
Risk: saved Topology placements of such assets keep their lane (pinned cards follow their y).

### 2.17 Connection hover
Verdict: confirmed. css 25 changes stroke and width on hover; only opacity transitions (204).

Touch: desktop-app-infra-canvas.css, inside the reduced-motion block (after 204).

Do:
1. `body:not([data-animations="false"]) .infra-edge-line { transition: stroke var(--ds-motion-duration-fast) var(--ds-motion-ease-standard), stroke-width var(--ds-motion-duration-fast) var(--ds-motion-ease-standard); }` (120 ms; `d` is not listed, so drags stay instant).

Tests: (a) none. (b) none.
Keys: none.
Risk: none.

## D. Open questions

1. **2.6 can only be met by hiding.** With the 12 px name floor and the 72 px minimum pitch, no one-line name fits below a card. I recommend one line with ellipsis, plus `is-name-clash`: hidden until hover, focus or selection, the same rule A5 uses for edge labels. The alternative, putting the name inside the box, would need the A24 icon floor lowered in `model.iconScale`.
2. **2.3 changes landing positions.** "After grid snap" implies snapping, but library drops never snapped. The plan snaps them, through the shared `slotsAt` that import also uses, so the outline always equals the landing. If you would rather keep unsnapped drops, drop the `model.snap` in `slotsAt`; the outline still matches.
3. **2.1 also pans for the window edge.** A selected card already cut by the window edge counts as covered and pans; this keeps the rule to one simple test. Cards with at least 0 px of clearance stay still, so the pinned test holds.
4. **The expanded guest shelf (2.7)** also hides labels. It is not in the spec, so I left it out. Its height is unknown without measuring it.
5. **2.10 banner is aria-hidden.** Move modes are already announced; choosing the Connect tool only changes `aria-pressed`. Add an announcement of `infra_mode_connect` in `applyTool` if you want parity.
6. **AGENTS.md drift:** the documented load order omits `infra-arrange.js`, which loads after infra-gestures.js in both lists. Fix the doc while there. The invariant "opening the inspector … moves nothing" (D-fix-1 bullet) should gain "unless the selected card would be covered (2.1)".
