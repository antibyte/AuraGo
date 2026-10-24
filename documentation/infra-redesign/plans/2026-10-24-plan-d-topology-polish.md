# Plan D: Infra topology, polish and docs Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Finish the Infra redesign. This plan adds:
- **Topology lanes:** four labelled lanes behind the cards, one pure `topologyLane(node)` mapping, a connection-order auto layout that draws unplaced assets without saving anything, dragging across lanes, and Arrange for the selection or for every card.
- **Theme QA:** the visual language is checked in the eight review themes, with a committed contrast test and a scratch screenshot runner.
- **Guards:** the CSS token, typography and motion rules become a Node guard.
- **Accessibility:** roving toolbars, tab and combobox structure, a live region that only announces state changes, focus return, and reduced motion.
- **Translations:** terminology and parity are checked across all 16 locales.
- **Docs:** `documentation/infra.md` and both user manuals are rewritten for the new UX.

**Architecture:** No new JS module.
- **Model:** the lane helpers live in `infra-model.js`: `topologyLane`, `laneAt`, `laneBands`, `topologyLayout`, `topologyAutoPositions` and `topologyArrange`. `buildScene` returns `lanes` in Topology.
- **Rendering and input:** `infra-canvas.js` draws `[data-lanes]` behind regions and cards. It feeds auto positions into `buildScene` (memoized) and exposes `setLaneTarget` and `isAutoPlaced`. `infra-gestures.js` uses `topologyArrange` and highlights the target lane while you drag.
- **Accessibility fixes:** they stay inside the existing modules (`infra.js` focus helpers, the store poll notice, dialogs, the import sheet).
- **Guards:** the static checks extend `scripts/test-infra-ui.mjs`. Three new rod tests cover topology, accessibility and theme contrast.
- **Screenshots:** the visual review uses an uncommitted Go test that `go test -overlay` compiles into package `ui`.

**Tech Stack:** Vanilla ES2020 IIFEs; CSS with `--vd-theme-*`/`--vd-*`/`--ds-*` tokens and container queries; Node `assert` and `vm` (`scripts/test-infra-ui.mjs`); Go and go-rod browser tests (`AURAGO_RUN_BROWSER_SMOKE=1`); Markdown docs.

---

## 0. Ground rules for every task

**Worktree:** `C:\Users\Andi\Documents\repo\AuraGo\.worktrees\infra-redesign-ui`, which is `/c/Users/Andi/Documents/repo/AuraGo/.worktrees/infra-redesign-ui` in Git Bash.
- The cwd resets between calls, so start every shell command with `cd "/c/Users/Andi/Documents/repo/AuraGo/.worktrees/infra-redesign-ui" &&`.
- Scratch scripts go in your session scratchpad directory, never in the repo. Below, `$SCRATCH` stands for that absolute path.

**Agent roster (project rules):** maker-agent implements code tasks, test-agent runs verification, docs-agent writes Tasks 12–13 and debug-agent investigates failures. Agents run at their own models with no model overrides. One combined review at the end, Critical and Important findings only.

**Standard verification (V-core):**
```bash
cd "/c/Users/Andi/Documents/repo/AuraGo/.worktrees/infra-redesign-ui" && node scripts/test-infra-ui.mjs
cd "/c/Users/Andi/Documents/repo/AuraGo/.worktrees/infra-redesign-ui" && go test ./ui -run '(TestVirtualDesktopFirstPartyJSFilesStayBelowLineBudget|TestFrontend_JS_NoLiteralTranslationFallbacks|TestDesktopUsedI18nKeysExistInAllLanguages|TestShippedWebAssetsLoadNothingExternal|TestUIAvoidsCookieConsentClassPrefix)' -count=1
```
**Browser verification (V-browser):** about 25 minutes. If Go build space runs out, trim GOCACHE on D: (entries older than 3 days, then older than 1 day).
```bash
cd "/c/Users/Andi/Documents/repo/AuraGo/.worktrees/infra-redesign-ui" && AURAGO_RUN_BROWSER_SMOKE=1 go test ./ui -run 'TestDesktopInfra' -count=1 -timeout 40m
```
**Syntax check** every JS file you created or edited: `node --check ui/js/desktop/apps/<file>.js`.

**Impact step (before editing an existing function):**
```bash
node C:/Users/Andi/.gitnexus/stores/AuraGo-08496d38b623/checkouts/AuraGo-08496d38b623/run.cjs impact "<Symbol>" --direction upstream --repo "C:\\Users\\Andi\\Documents\\repo\\AuraGo"
graft callers <Symbol>
```
- If the tool does not know the repo, use `--repo AuraGo`.
- The index is stale, so expect `risk: UNKNOWN`. That is not an all-clear: confirm with the `git grep` the step gives, and record callers and risk in your task notes.
- Warn before editing on HIGH or CRITICAL.

**Commit step template (path-limited; other sessions commit in this checkout):**
```bash
node C:/Users/Andi/.gitnexus/stores/AuraGo-08496d38b623/checkouts/AuraGo-08496d38b623/run.cjs detect-changes --scope all --repo "C:\\Users\\Andi\\Documents\\repo\\AuraGo"
cd "/c/Users/Andi/Documents/repo/AuraGo/.worktrees/infra-redesign-ui" && git add <new files of this task only> && git commit -m "<subject>" -m "<one-paragraph body>" -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>" -- <every path this task touched>
```
- `partial: true` or `truncated: true` is not a clean result; re-run it.
- Never run `git add -A`. Never use bare `git stash`.

**Code conventions (unchanged from Plans B and C):**
- Escapes and strings:
  - Write `'\x1f'`-style escapes, never `\u` escapes; the Write/Edit tools decode `\u`.
  - Every string is a literal `t('desktop.infra_…')` call. Placeholders go through `model.fill(text, vars)`.
  - Never write `t('k', 'fallback')`, `|| 'x'`, or template-literal keys for new keys.
- Load time: no `document`, `localStorage`, `requestAnimationFrame`, `CSS` or `matchMedia` while a file loads.
- Naming and assets: classes use the `infra-` prefix and never start with `cc-`. No external URLs. Inline SVG has no `xmlns`.
- Motion: JS motion runs only when `app.animationsEnabled()`.
- Go raw strings that hold JS must not contain backticks.
- Size: every JS file stays below 1100 lines.

**Coordinate contract (Plan B, unchanged):**
- An asset placement `(x, y)` is the centre of its 216×64 card (design QA A2; it was 184×60 when this plan was written).
- A place placement `(x, y)` is the top-left corner of its region and `(w, h)` its size: default 360×240, minimum 200×140.
- The drawn region is the stored rectangle grown to contain its children plus 48 px.

**Lane contract (new in this plan; Task 12 documents it):**
- **Bands:** lanes are fixed horizontal world bands, `LANE_H = 480` px each, from y = 0, in this order: `internet`, `network`, `compute`, `clients`.
- **Lane of a card:**
  - A card with a topology placement saved `pinned: true` is in lane `laneAt(y) = clamp(floor(y / 480), 0, 3)`.
  - Any other card (no topology placement, or saved `pinned: false` by Arrange) is in lane `topologyLane(node)`.
- **Auto rows:** auto-laid rows sit at lane top + 96 + row × 96, with columns 240 px apart (`LANE_CELL_W = 240`, card width 216 plus one 24 px grid cell), centred.
- **Auto layout never writes:** assets without a topology placement are drawn at computed slots. Nothing is saved until a card is dragged, moved with the keyboard, or arranged.

**Plan A dependency:**
- Plan A is merged into this branch, so `store.features.layoutParent` and `store.features.unplace` are true against the real server.
- Keep every guard Plans B and C added for older servers.

---

## 1. Decisions locked in by this plan

1. **The auto layout is display-only.** Opening Topology never writes. Every window computes the same deterministic slots from the assets and their physical connections.
2. **Slots are stable:**
   - Every eligible asset gets a slot from one layout over all eligible assets, computed in their type lane. Dragging one card never shifts another.
   - A slot under a placed card moves to the nearest free grid point (`freeSlot`).
   - Eligible means not a place, not a guest, and not a VM or container without a host.
3. **`pinned` means "the user placed it" in Topology.**
   - A drag or keyboard move saves `pinned: true`, and the lane comes from y.
   - Arrange keeps `pinned` as it was. Previously auto cards are saved `pinned: false`, so their lane keeps following their type.
   - "Place automatically" (unplace in Topology) hands a card back to the auto layout.
4. **Lane mapping precedence:** the `network` kind, then the icon, then words in the role, then "has guests" (Compute), else Clients and IoT.
   - The icon sets follow the spec and add `switch`, `vlan` and `wlan` to Network, because they are network gear and segments like `lan`. Reviewers may veto that.
   - Role words split camel case, so `InternetGatewayDevice` reads as "internet gateway device" and lands in Network.
5. **Arrange in Topology:**
   - With a selection, it lays out only the selection, centred on the selection's mean x, around the other cards.
   - Without a selection, it lays out every card.
   - It is one undoable `PUT /layout`. Undoing it unplaces the cards that were auto before.
6. **Lane labels live in world space** inside each lane (14 px, muted pill). They scroll with the map.
7. **The library's To place section is empty in Topology**, because the auto layout draws every placeable asset.
8. **Announcements:**
   - Only state changes speak. A routine poll never changes a live region, even when its data changed.
   - A poll that keeps failing reports its error once. An explicit Refresh always reports.
9. **Focus return:** panels opened from a control give focus back through `app.focusReturnTarget(el)`/`app.restoreFocus(el)`. A menu item hands over to its menu trigger; a vanished element falls back to the map.
10. **Visual QA runner:**
    - The runner is a Go test compiled in through `go test -overlay` from `$SCRATCH`, and is never committed.
    - Its output goes to `reports/infra-ui-review/plan-d/`, which is git-ignored and survives the session, for the reviewer.
    - The committed contrast test covers the same eight themes.
11. **Retired UI terms:**
    - en: node(s), relation(s), relationship(s), candidate(s).
    - de: Knoten, Relation, Beziehung, Kandidat, Behälter.
    - The check covers every used `desktop.infra_*` key; the allowlist is explicit.
    - Unused keys are pruned only when no source references them.

**Known limitations (state them in the review handoff):**
- Lanes have a fixed height. A lane with more than 32 assets widens its rows instead of growing taller.
- Lane labels are not sticky.
- Lane membership is a heuristic; users correct it by dragging.
- The lane is visual only. The library tree and the List view are the accessible lists.
- Hostless VMs and containers appear in Topology only after their host is assigned (Cleanup) or after you place them explicitly.
- The committed theme test and the screenshot review cover 8 of the 14 themes. The other six rely on tokens and the CSS guard.

---

## 2. File structure

| File | Status | Change |
|---|---|---|
| `ui/js/desktop/apps/infra-model.js` | modify | Topology block: lanes, layout, auto positions, arrange. `buildScene` returns `lanes`. `topologyPositions`/`TOPOLOGY_LAYER` removed (Task 4). |
| `ui/js/desktop/apps/infra-canvas.js` | modify | `[data-lanes]` layer, lane patching, memoized auto positions, `setLaneTarget`, `isAutoPlaced`. |
| `ui/js/desktop/apps/infra-gestures.js` | modify | Lane highlight during drag and move mode, lane-move label, `arrangeTopology`, origins from the scene, unplace label. |
| `ui/js/desktop/apps/infra-library.js` | modify | To place is empty in Topology. |
| `ui/js/desktop/apps/infra-menus.js` | modify | The unplace item reads "Place automatically" in Topology. |
| `ui/js/desktop/apps/infra-inspector.js` | modify | Overview: auto-laid fact; no "Place on map" in Topology. |
| `ui/js/desktop/apps/infra.js` | modify | `app.laneLabel`, `app.focusReturnTarget`, `app.restoreFocus`, library sheet focus. |
| `ui/js/desktop/apps/infra-store.js` | modify | A failing poll notifies once. |
| `ui/js/desktop/apps/infra-dialogs.js` | modify | Dialog focus return. |
| `ui/js/desktop/apps/infra-import.js` | modify | Opener through `focusReturnTarget`. |
| `ui/css/desktop-app-infra-canvas.css` | modify | Lane rules. |
| `ui/css/desktop-app-infra*.css` | modify | Fixes the guard and the review find. |
| `scripts/test-infra-ui.mjs` | modify | Topology tests, store test, CSS guard, terminology and parity checks. |
| `ui/desktop_infra_topology_browser_test.go` | create | `TestDesktopInfraTopologyBrowser`, `infraGermanWord`. |
| `ui/desktop_infra_a11y_browser_test.go` | create | `TestDesktopInfraAccessibilityBrowser`, `TestDesktopInfraThemeContrastBrowser`, theme helpers, contrast probe. |
| `ui/lang/desktop/*.json` (16) | modify | Batch J and terminology updates. |
| `ui/js/desktop/apps/AGENTS.md` | modify | Infra bullets, Infra added to the theme bridge list. |
| `documentation/infra.md` | modify | Intro, Build your map, Desktop app architecture, development notes. |
| `documentation/manuals/en/29-infra.md`, `documentation/manuals/de/29-infra.md` | modify | Rewritten for the new UX. |
| `documentation/manuals/{en,de}/README.md`, `documentation/manuals/{en,de}/04-webui.md` | modify | One-line chapter descriptions. |
| `$SCRATCH/infra-visual/*` | scratch | Overlay runner, overlay JSON. Never committed. |

No file is added to the module loader or to the test file list.

---

## Task 0: Baseline and preconditions (no commit)

**Files:** none.

- [ ] **Step 1: Confirm that Plans B and C are complete.**
  ```bash
  cd "/c/Users/Andi/Documents/repo/AuraGo/.worktrees/infra-redesign-ui" && ls ui/js/desktop/apps/infra-{store,canvas,gestures,library,cleanup,picker,inspector,inspector-forms,table,dialogs,import,menus}.js ui/desktop_infra_canvas_browser_test.go ui/desktop_infra_panels_browser_test.go && test ! -e ui/js/desktop/apps/infra-views.js && echo OK
  cd "/c/Users/Andi/Documents/repo/AuraGo/.worktrees/infra-redesign-ui" && git grep -n -e "const naturalCompare" -e "const topologyPositions" -- ui/js/desktop/apps/infra-model.js
  cd "/c/Users/Andi/Documents/repo/AuraGo/.worktrees/infra-redesign-ui" && git grep -n -e "func infraCanvasFixtureNode" -e "func infraPanelsNode" -e "func infraPanelsSource" -e "func infraPanelsAt" -e "func infraPanelsPage" -e "func infraPanelsCenter" -e "func infraFixtureNumber" -e "layoutCalls" -- ui/*_test.go
  ```
  Expected: `OK`, both model lines, and every helper. If something is missing, stop and report to the orchestrator.

- [ ] **Step 2: Confirm that Plan A is merged.**
  ```bash
  cd "/c/Users/Andi/Documents/repo/AuraGo/.worktrees/infra-redesign-ui" && git merge-base --is-ancestor feature/infra-redesign HEAD && echo merged
  cd "/c/Users/Andi/Documents/repo/AuraGo/.worktrees/infra-redesign-ui" && git grep -n 'json:"unplace' -- internal/infra
  ```
  If either check fails, stop and report.

- [ ] **Step 3: Check for parallel work on Plan D paths in the main checkout.** These paths come from project memory.
  ```bash
  git -C "/c/Users/Andi/Documents/repo/AuraGo" status --short -- ui/js/desktop/apps ui/css scripts/test-infra-ui.mjs ui/lang/desktop documentation/infra.md documentation/manuals
  ```
  Record the result. If another session is editing the same Infra files, ask the orchestrator before you continue.

- [ ] **Step 4: Run the baselines.** Run V-core and V-browser and record the results, including any pre-existing failure with its assertion text and line. Also run:
  ```bash
  cd "/c/Users/Andi/Documents/repo/AuraGo/.worktrees/infra-redesign-ui" && wc -l ui/js/desktop/apps/infra*.js
  ```
  `infra-model.js` and `infra-gestures.js` grow in this plan. Each must still be below 1100 lines after Tasks 1 and 4: model about +150, gestures about +45. If a file would reach 1099, stop and report instead of improvising a split.

---

## Task 1: Model: topology lanes, layout, auto positions and arrange (pure)

**Files:**
- Modify: `ui/js/desktop/apps/infra-model.js`
- Modify: `scripts/test-infra-ui.mjs`

- [ ] **Step 1: Impact check** for `buildScene`, using the impact template. Confirm with:
  ```bash
  cd "/c/Users/Andi/Documents/repo/AuraGo/.worktrees/infra-redesign-ui" && git grep -n -e "buildScene" -e "topologyPositions" -e "TOPOLOGY_LAYER" -- ui scripts
  ```
  - Expected callers: `infra-canvas.js` (renderNow), `infra-gestures.js` (arrange uses `topologyPositions`), the model itself and the tests.
  - The `buildScene` change only adds a `lanes` field. In Topology, `bounds` now includes the lanes.
  - `topologyPositions` stays until Task 4.

- [ ] **Step 2: Add the topology block** directly before `window.InfraAppModel = {`, after the Plan C blocks:

```js
    // ---- topology lanes and auto layout (spec §6) --------------------------------
    // Four fixed horizontal world bands, LANE_H px tall, from y = 0. A card the
    // user placed (pinned) belongs to the lane its centre y is in (laneAt);
    // every other card belongs to topologyLane(node).
    const TOPOLOGY_LANES = ['internet', 'network', 'compute', 'clients'];
    const LANE_H = 480, LANE_ROW0 = 96, LANE_ROW_H = 96, LANE_CELL_W = 216, LANE_PAD_X = 240, LANE_MIN_W = 1440;
    const PHYSICAL_EDGES = new Set(['ethernet', 'fiber', 'wifi']);
    const LANE_ICONS = {
        internet: ['internet', 'modem-ont'],
        network: ['router', 'firewall', 'managed-switch', 'switch', 'patch-panel', 'access-point', 'mesh-node', 'lan', 'vlan', 'wlan'],
        compute: ['rack-server', 'tower-server', 'mini-pc', 'sbc', 'nas', 'hypervisor', 'kubernetes'],
    };
    const LANE_ICON_INDEX = new Map(Object.entries(LANE_ICONS).flatMap(([lane, icons]) => icons.map((icon, rank) => [icon, { lane, rank }])));
    const LANE_ROLE_WORDS = [
        ['internet', ['modem', 'ont', 'isp', 'wan', 'dsl']],
        ['network', ['router', 'firewall', 'switch', 'gateway', 'mesh', 'ap', 'access point']],
        ['compute', ['server', 'nas', 'hypervisor', 'proxmox', 'esxi', 'kubernetes', 'k8s']],
    ];
    // roleWords turns "InternetGatewayDevice" or "dsl-modem" into " internet gateway device ".
    const roleWords = role => ` ${String(role || '').replace(/([a-z0-9])([A-Z])/g, '$1 $2').toLowerCase().replace(/[^a-z0-9]+/g, ' ').trim()} `;
    // topologyLane: the network kind, then the icon, then words in the role,
    // then "runs guests"; everything else is a client.
    const topologyLane = (node, { hasGuests = false } = {}) => {
        if (node?.kind === 'network') return 'network';
        const byIcon = LANE_ICON_INDEX.get(node?.icon);
        if (byIcon) return byIcon.lane;
        const words = roleWords(node?.profile?.role);
        for (const [lane, list] of LANE_ROLE_WORDS) if (list.some(word => words.includes(` ${word} `))) return lane;
        return hasGuests ? 'compute' : 'clients';
    };
    const topologyLaneIndex = lane => Math.max(0, TOPOLOGY_LANES.indexOf(lane));
    const laneAt = y => { const n = Number(y); return Number.isFinite(n) ? Math.min(TOPOLOGY_LANES.length - 1, Math.max(0, Math.floor(n / LANE_H))) : TOPOLOGY_LANES.length - 1; };
    // laneBands spans the four lanes across box (the drawn cards) with
    // LANE_PAD_X to spare on both sides, at least LANE_MIN_W wide.
    const laneBands = box => {
        const centre = box ? box.x + box.w / 2 : 0;
        const width = Math.ceil(Math.max(LANE_MIN_W, box ? box.w + LANE_PAD_X * 2 : 0) / GRID) * GRID;
        const x = snap(centre - width / 2);
        return TOPOLOGY_LANES.map((id, index) => ({ id, index, rect: { x, y: index * LANE_H, w: width, h: LANE_H } }));
    };
    const laneRank = node => LANE_ICON_INDEX.get(node?.icon)?.rank ?? 99;
    // topologyLayout puts nodes into their lanes (laneOf → index) in connection
    // order: a breadth-first walk over Ethernet, fiber and Wi-Fi edges, started
    // from the top lane, numbers the nodes, so parents come before children.
    // A lane holds up to maxRows rows of up to maxColumns cards (more widen the
    // rows); every row is centred on centerX. obstacles are rects of cards that
    // stay put; a slot under one moves to the nearest free grid point.
    const topologyLayout = (nodes, edges, { laneOf = node => topologyLaneIndex(topologyLane(node)), obstacles = [], centerX = 0, maxColumns = 8, maxRows = 4 } = {}) => {
        const list = (nodes || []).filter(node => node && node.id);
        const byId = new Map(list.map(node => [node.id, node]));
        const lane = new Map(list.map(node => [node.id, Math.min(TOPOLOGY_LANES.length - 1, Math.max(0, Math.trunc(Number(laneOf(node))) || 0))]));
        const compare = (a, b) => lane.get(a.id) - lane.get(b.id) || laneRank(a) - laneRank(b) || naturalCompare(a.name, b.name) || String(a.id).localeCompare(String(b.id));
        const adjacency = new Map(list.map(node => [node.id, []]));
        for (const edge of edges || []) {
            if (!PHYSICAL_EDGES.has(edge?.kind) || edge.from === edge.to || !byId.has(edge.from) || !byId.has(edge.to)) continue;
            adjacency.get(edge.from).push(byId.get(edge.to));
            adjacency.get(edge.to).push(byId.get(edge.from));
        }
        for (const neighbours of adjacency.values()) neighbours.sort(compare);
        const order = new Map();
        for (const seed of list.slice().sort(compare)) {
            if (order.has(seed.id)) continue;
            order.set(seed.id, order.size);
            const queue = [seed];
            for (let head = 0; head < queue.length; head += 1) {
                for (const next of adjacency.get(queue[head].id)) {
                    if (order.has(next.id)) continue;
                    order.set(next.id, order.size);
                    queue.push(next);
                }
            }
        }
        const lanes = TOPOLOGY_LANES.map(() => []);
        for (const node of list) lanes[lane.get(node.id)].push(node);
        const nudge = (obstacles || []).length > 0, placed = nudge ? obstacles.slice() : [], out = new Map();
        lanes.forEach((members, index) => {
            members.sort((a, b) => order.get(a.id) - order.get(b.id));
            const rows = Math.max(1, Math.min(maxRows, Math.ceil(members.length / maxColumns)));
            const columns = Math.max(1, Math.ceil(members.length / rows));
            members.forEach((node, i) => {
                const row = Math.floor(i / columns), inRow = Math.min(columns, members.length - row * columns);
                const want = { x: snap(centerX - (inRow - 1) * LANE_CELL_W / 2) + (i % columns) * LANE_CELL_W, y: index * LANE_H + LANE_ROW0 + row * LANE_ROW_H };
                const pos = nudge ? freeSlot(want, placed) : want;
                if (nudge) placed.push(cardRect(pos));
                out.set(node.id, { x: pos.x, y: pos.y, pinned: false });
            });
        });
        return out;
    };
    // topologyAutoPositions: where Topology draws assets without a topology
    // placement. Every eligible asset (not a place, not a guest, not a VM or
    // container without a host) gets the slot of one layout over all of them,
    // so the slots only change when assets or connections change and moving
    // one card never shifts another; a slot under a placed card moves aside.
    const topologyAutoPositions = (nodes, edges, { placementOf = () => null, guests = { hostOf: new Map(), guestsOf: new Map() } } = {}) => {
        const drawable = (nodes || []).filter(node => node && !isContainer(node.kind) && !guests.hostOf.has(node.id));
        const eligible = drawable.filter(node => node.kind !== 'vm' && node.kind !== 'container');
        const slots = topologyLayout(eligible, edges, { laneOf: node => topologyLaneIndex(topologyLane(node, { hasGuests: (guests.guestsOf.get(node.id) || []).length > 0 })) });
        const obstacles = drawable.map(node => placementOf(node.id)).filter(p => p && finite(p.x) != null && finite(p.y) != null).map(cardRect);
        const nudge = obstacles.length > 0, out = new Map();
        for (const node of eligible) {
            if (placementOf(node.id)) continue;
            const want = slots.get(node.id);
            const pos = nudge ? freeSlot(want, obstacles) : want;
            if (nudge) obstacles.push(cardRect(pos));
            out.set(node.id, { x: pos.x, y: pos.y, pinned: false });
        }
        return out;
    };
    // topologyArrange plans Arrange in Topology: the selected cards (centred on
    // their mean x, around the other cards) or, without a selection, every
    // card. A card the user placed (pinned) keeps the lane its y is in and
    // stays pinned; the others follow their type and are saved unpinned.
    const topologyArrange = ({ cards = [], selection = new Set(), nodeById, edges = [], guestsOf = new Map(), placementOf = () => null }) => {
        const chosen = cards.filter(card => selection.has(card.id));
        const targets = (chosen.length ? chosen : cards).filter(card => nodeById(card.id));
        const ids = new Set(targets.map(card => card.id));
        const manual = id => { const placement = placementOf(id); return !!placement && placement.pinned !== false; };
        const yOf = new Map(cards.map(card => [card.id, card.y]));
        const laneOf = node => (manual(node.id) ? laneAt(yOf.get(node.id)) : topologyLaneIndex(topologyLane(node, { hasGuests: (guestsOf.get(node.id) || []).length > 0 })));
        const obstacles = chosen.length ? cards.filter(card => !ids.has(card.id)).map(cardRect) : [];
        const centerX = chosen.length ? snap(chosen.reduce((sum, card) => sum + card.x, 0) / chosen.length) : 0;
        const out = new Map();
        for (const [id, pos] of topologyLayout(targets.map(card => nodeById(card.id)), edges, { laneOf, obstacles, centerX })) out.set(id, { x: pos.x, y: pos.y, pinned: manual(id) });
        return out;
    };
```

- [ ] **Step 3: Return the lanes from `buildScene`.** In `buildScene`, replace the final statement `return { cards, cardById, regions, regionRects: rects, regionDepth: depth, lines, bounds: unionRects([...cards.map(cardRect), ...rects.values()]) };` with:

```js
        const drawn = unionRects([...cards.map(cardRect), ...rects.values()]);
        const lanes = view === 'topology' ? laneBands(drawn) : [];
        return { cards, cardById, regions, regionRects: rects, regionDepth: depth, lines, lanes, bounds: lanes.length ? unionRects([drawn, ...lanes.map(lane => lane.rect)]) : drawn };
```

- [ ] **Step 4: Export the new helpers.** Append this line to the export object: `topologyLanes: () => [...TOPOLOGY_LANES], LANE_H, topologyLane, topologyLaneIndex, laneAt, laneBands, topologyLayout, topologyAutoPositions, topologyArrange,`.

- [ ] **Step 5: Add the Node tests.** Insert this block before the final `console.log` of `scripts/test-infra-ui.mjs`:

```js
// ---- topology lanes and auto layout (spec §6) ------------------------------------
{
    const lane = (icon, extra = {}, opts) => model.topologyLane({ kind: 'device', icon, profile: {}, ...extra }, opts);
    assert.deepEqual(plain(model.topologyLanes()), ['internet', 'network', 'compute', 'clients']);
    assert.equal(model.topologyLane({ kind: 'network', icon: 'generic-device', profile: {} }), 'network', 'the network kind is always in the network lane');
    for (const icon of ['internet', 'modem-ont']) assert.equal(lane(icon), 'internet', icon);
    for (const icon of ['router', 'firewall', 'managed-switch', 'switch', 'access-point', 'mesh-node', 'patch-panel', 'lan']) assert.equal(lane(icon), 'network', icon);
    for (const icon of ['rack-server', 'tower-server', 'mini-pc', 'sbc', 'nas', 'hypervisor', 'kubernetes']) assert.equal(lane(icon), 'compute', icon);
    assert.equal(lane('generic-device', { profile: { role: 'InternetGatewayDevice' } }), 'network', 'role words split camel case');
    assert.equal(lane('generic-device', { profile: { role: 'DSL modem' } }), 'internet');
    assert.equal(lane('generic-device', { profile: { role: 'MediaServer' } }), 'compute');
    assert.equal(lane('workstation', {}, { hasGuests: true }), 'compute', 'a device that runs guests is compute');
    assert.equal(lane('router', {}, { hasGuests: true }), 'network', 'an explicit icon wins over guests');
    assert.equal(lane('generic-device', { profile: { role: 'mdns-device' } }), 'clients');
    assert.equal(lane('tv', { profile: { role: 'chromecast' } }), 'clients');
    assert.equal(model.topologyLane({ kind: 'service', icon: 'web-service', profile: {} }), 'clients', 'everything else is a client');
    assert.equal(model.topologyLaneIndex('compute'), 2);
    assert.deepEqual([-50, 0, 479, 480, 1919, 5000, 'x'].map(y => model.laneAt(y)), [0, 0, 0, 1, 3, 3, 3]);
    assert.deepEqual(plain(model.laneBands({ x: -404, y: 66, w: 832, h: 1500 })), [
        { id: 'internet', index: 0, rect: { x: -696, y: 0, w: 1440, h: 480 } },
        { id: 'network', index: 1, rect: { x: -696, y: 480, w: 1440, h: 480 } },
        { id: 'compute', index: 2, rect: { x: -696, y: 960, w: 1440, h: 480 } },
        { id: 'clients', index: 3, rect: { x: -696, y: 1440, w: 1440, h: 480 } },
    ], 'lanes span the cards with 240 px to spare and are at least 1440 px wide');

    const topo = [
        { id: 'inet', kind: 'device', name: 'Internet', icon: 'internet', profile: {} },
        { id: 'ont', kind: 'device', name: 'Fiber ONT', icon: 'modem-ont', profile: {} },
        { id: 'router', kind: 'device', name: 'Edge router', icon: 'router', profile: {} },
        { id: 'switch', kind: 'device', name: 'Core switch', icon: 'managed-switch', profile: {} },
        { id: 'ap', kind: 'device', name: 'Attic access point', icon: 'access-point', profile: {} },
        { id: 'nas', kind: 'device', name: 'Basement NAS', icon: 'nas', profile: {} },
        { id: 'pve', kind: 'device', name: 'Proxmox host', icon: 'generic-device', profile: {} },
        { id: 'laptop', kind: 'device', name: 'Admin laptop', icon: 'laptop', profile: {} },
        { id: 'tv', kind: 'device', name: 'Living room TV', icon: 'tv', profile: {} },
    ];
    const topoEdges = [
        { id: 'e1', kind: 'ethernet', from: 'inet', to: 'ont' }, { id: 'e2', kind: 'fiber', from: 'ont', to: 'router' },
        { id: 'e3', kind: 'ethernet', from: 'router', to: 'switch' }, { id: 'e4', kind: 'ethernet', from: 'switch', to: 'ap' },
        { id: 'e5', kind: 'ethernet', from: 'switch', to: 'nas' }, { id: 'e6', kind: 'ethernet', from: 'pve', to: 'switch' },
        { id: 'e7', kind: 'wifi', from: 'ap', to: 'laptop' }, { id: 'e8', kind: 'ethernet', from: 'switch', to: 'tv' },
        { id: 'e9', kind: 'depends_on', from: 'tv', to: 'nas' },
    ];
    const guestsOf = new Map([['pve', ['vm1']]]);
    const laneOf = node => model.topologyLaneIndex(model.topologyLane(node, { hasGuests: (guestsOf.get(node.id) || []).length > 0 }));
    const asObject = map => Object.fromEntries(plain([...map]));
    const layout = model.topologyLayout(topo, topoEdges, { laneOf });
    assert.deepEqual(asObject(layout), {
        inet: { x: -96, y: 96, pinned: false }, ont: { x: 120, y: 96, pinned: false },
        router: { x: -216, y: 576, pinned: false }, switch: { x: 0, y: 576, pinned: false }, ap: { x: 216, y: 576, pinned: false },
        nas: { x: -96, y: 1056, pinned: false }, pve: { x: 120, y: 1056, pinned: false },
        tv: { x: -96, y: 1536, pinned: false }, laptop: { x: 120, y: 1536, pinned: false },
    }, 'lanes in connection order, parents before children, rows centred');
    assert.deepEqual(asObject(model.topologyLayout(topo.slice().reverse(), topoEdges.slice().reverse(), { laneOf })), asObject(layout), 'input order does not change the layout');
    assert.deepEqual(asObject(model.topologyLayout(topo, topoEdges.slice(0, 8), { laneOf })), asObject(layout), 'dependencies do not order the layout; only Ethernet, fiber and Wi-Fi do');
    const overridden = model.topologyLayout(topo, topoEdges, { laneOf: node => (node.id === 'tv' ? 1 : laneOf(node)) });
    assert.deepEqual(plain(overridden.get('tv')), { x: 336, y: 576, pinned: false }, 'a lane override moves the card and keeps its connection order');
    assert.deepEqual(plain(overridden.get('router')), { x: -312, y: 576, pinned: false });
    assert.deepEqual(plain(overridden.get('laptop')), { x: 0, y: 1536, pinned: false });
    const clients = Array.from({ length: 10 }, (_, i) => ({ id: `c${i}`, kind: 'device', name: `Client ${String(i).padStart(2, '0')}`, icon: 'laptop', profile: {} }));
    const wrapped = model.topologyLayout(clients, []);
    assert.deepEqual(plain(wrapped.get('c4')), { x: 432, y: 1536, pinned: false });
    assert.deepEqual(plain(wrapped.get('c5')), { x: -432, y: 1632, pinned: false }, 'ten cards wrap into two centred rows of five');
    const nudged = model.topologyLayout(topo.slice(0, 2), [topoEdges[0]], { laneOf, obstacles: [model.cardRect({ x: -96, y: 96 })] });
    assert.deepEqual(asObject(nudged), { inet: { x: -96, y: 24, pinned: false }, ont: { x: 120, y: 96, pinned: false } }, 'a slot under a card that stays moves to the nearest free grid point');
    assert.deepEqual(asObject(model.topologyLayout(topo.slice(0, 2), [topoEdges[0]], { laneOf, centerX: 480 })), { inet: { x: 384, y: 96, pinned: false }, ont: { x: 600, y: 96, pinned: false } });

    const autoNodes = [...topo.slice(0, 3),
        { id: 'room', kind: 'room', name: 'Living room', icon: 'room', profile: {} },
        { id: 'vm1', kind: 'vm', name: 'Lab VM', icon: 'vm', host_id: 'router', profile: {} },
        { id: 'ct', kind: 'container', name: 'orphan', icon: 'container', profile: {} }];
    const autoGuests = model.guestIndex(autoNodes, []);
    const far = { ont: { x: 2000, y: 96, pinned: true } };
    const auto = model.topologyAutoPositions(autoNodes, topoEdges.slice(0, 2), { placementOf: id => far[id] || null, guests: autoGuests });
    assert.deepEqual(asObject(auto), { inet: { x: -96, y: 96, pinned: false }, router: { x: 0, y: 576, pinned: false } }, 'placed assets, places, guests and hostless containers get no auto slot');
    const onSlot = { ont: { x: -96, y: 96, pinned: true } };
    assert.deepEqual(plain(model.topologyAutoPositions(autoNodes, topoEdges.slice(0, 2), { placementOf: id => onSlot[id] || null, guests: autoGuests }).get('inet')), { x: -96, y: 24, pinned: false }, 'an auto slot under a placed card moves aside');
    const sceneTopo = model.buildScene({ nodes: autoNodes, edges: topoEdges.slice(0, 2), view: 'topology', placementOf: id => far[id] || auto.get(id) || null, parentOf: () => '', guests: autoGuests });
    assert.deepEqual(plain(sceneTopo.cards.map(card => card.id)), ['inet', 'ont', 'router']);
    assert.deepEqual(plain(sceneTopo.lanes.map(item => item.id)), ['internet', 'network', 'compute', 'clients']);
    assert.deepEqual(plain(sceneTopo.lanes[1].rect), { x: -432, y: 480, w: 2760, h: 480 });
    assert.deepEqual(plain(sceneTopo.bounds), { x: -432, y: 0, w: 2760, h: 1920 }, 'topology bounds include the lanes');
    assert.deepEqual(plain(model.buildScene({ nodes: autoNodes, edges: [], view: 'atlas', placementOf: () => null, parentOf: () => '', guests: autoGuests }).lanes), [], 'the Map view has no lanes');

    const cards = [...layout].map(([id, pos]) => ({ id, x: pos.x, y: id === 'tv' ? 720 : pos.y }));
    const stored = { tv: { x: -96, y: 720, pinned: true }, nas: { x: -96, y: 1056, pinned: false } };
    const nodeById = id => topo.find(node => node.id === id);
    const arranged = model.topologyArrange({ cards, nodeById, edges: topoEdges, guestsOf, placementOf: id => stored[id] || null });
    assert.equal(arranged.size, 9, 'without a selection every card is arranged');
    assert.deepEqual(plain(arranged.get('tv')), { x: 336, y: 576, pinned: true }, 'a dragged card keeps its lane and stays pinned');
    assert.deepEqual(plain(arranged.get('router')), { x: -312, y: 576, pinned: false });
    assert.deepEqual(plain(arranged.get('laptop')), { x: 0, y: 1536, pinned: false });
    assert.deepEqual(plain(arranged.get('nas')), { x: -96, y: 1056, pinned: false }, 'an arranged, unpinned card follows its type');
    const selected = model.topologyArrange({ cards, selection: new Set(['tv', 'laptop']), nodeById, edges: topoEdges, guestsOf, placementOf: id => stored[id] || null });
    assert.deepEqual(asObject(selected), { tv: { x: 24, y: 504, pinned: true }, laptop: { x: 24, y: 1536, pinned: false } }, 'a selection is arranged around its centre and around the other cards');
}
```

- [ ] **Step 6: Verify.**
  - Run `node --check ui/js/desktop/apps/infra-model.js`, then V-core. Expected: everything passes.
  - Run `wc -l ui/js/desktop/apps/infra-model.js`. Expected: below 1100.

- [ ] **Step 7: Commit.**
  - Paths: `ui/js/desktop/apps/infra-model.js`, `scripts/test-infra-ui.mjs`.
  - Subject: `feat(infra): add topology lanes, connection-order layout and topology arrange helpers`.

---

## Task 2: i18n batch J (topology)

**Files:** `ui/lang/desktop/{cs,da,de,el,en,es,fr,hi,it,ja,nl,no,pl,pt,sv,zh}.json`.

- [ ] **Step 1: Write the insertion script** to `$SCRATCH/infra-i18n-add.mjs`:

```js
// Usage: node infra-i18n-add.mjs <repoRoot> <batch.json>
// batch.json: { "en": { "desktop.infra_x": "…" }, "de": { … }, … all 16 locales }
import fs from 'node:fs';
import path from 'node:path';

const [root, batchFile] = process.argv.slice(2);
const batch = JSON.parse(fs.readFileSync(batchFile, 'utf8'));
const locales = ['cs', 'da', 'de', 'el', 'en', 'es', 'fr', 'hi', 'it', 'ja', 'nl', 'no', 'pl', 'pt', 'sv', 'zh'];
const keys = Object.keys(batch.en || {});
if (!keys.length) throw new Error('batch has no English keys');
for (const locale of locales) {
    const entries = batch[locale];
    if (!entries) throw new Error(`batch lacks ${locale}`);
    for (const key of keys) if (typeof entries[key] !== 'string' || !entries[key].trim()) throw new Error(`${locale} lacks ${key}`);
    const file = path.join(root, 'ui/lang/desktop', `${locale}.json`);
    const text = fs.readFileSync(file, 'utf8');
    const eol = text.includes('\r\n') ? '\r\n' : '\n';
    const existing = JSON.parse(text);
    const fresh = keys.filter(key => !(key in existing));
    if (!fresh.length) { console.log(`${locale}: nothing new`); continue; }
    const lines = text.split(eol);
    let anchor = -1;
    lines.forEach((line, index) => { if (/^\s*"desktop\.infra_/.test(line)) anchor = index; });
    if (anchor < 0) throw new Error(`${locale}: no desktop.infra_ anchor line`);
    const hadComma = lines[anchor].trimEnd().endsWith(',');
    if (!hadComma) lines[anchor] = lines[anchor].replace(/\s*$/, ',');
    const added = fresh.map((key, i) => `  ${JSON.stringify(key)}: ${JSON.stringify(entries[key])}${i < fresh.length - 1 || hadComma ? ',' : ''}`);
    lines.splice(anchor + 1, 0, ...added);
    const out = lines.join(eol);
    JSON.parse(out);
    fs.writeFileSync(file, out);
    console.log(`${locale}: +${fresh.length}`);
}
```

- [ ] **Step 2: Write `$SCRATCH/infra-i18n-j.json`** with all 16 locales. English and German are below. Translate the other 14 locales natively:
  - Keep `{name}`, `{lane}` and `{count}` unchanged.
  - Write no English prose; the test rejects identical English text that is longer than 20 characters and contains spaces.
  - "Internet" and "IoT" may stay as they are.

| key | en | de |
|---|---|---|
| `desktop.infra_lane_internet` | Internet | Internet |
| `desktop.infra_lane_network` | Network | Netzwerk |
| `desktop.infra_lane_compute` | Compute | Server und Speicher |
| `desktop.infra_lane_clients` | Clients and IoT | Clients und IoT |
| `desktop.infra_moved_to_lane` | {name} moved to {lane} | {name} nach {lane} verschoben |
| `desktop.infra_topology_auto` | Laid out automatically | Automatisch angeordnet |
| `desktop.infra_topology_auto_place` | Place automatically | Automatisch platzieren |
| `desktop.infra_topology_auto_placed` | {count} assets placed automatically | {count} Assets automatisch platziert |

- [ ] **Step 3: Run the script.**
  ```bash
  cd "/c/Users/Andi/Documents/repo/AuraGo/.worktrees/infra-redesign-ui" && node "$SCRATCH/infra-i18n-add.mjs" . "$SCRATCH/infra-i18n-j.json"
  ```
  Expected: `+8` for each of the 16 locales.
- [ ] **Step 4: Verify** with V-core. `git diff --stat` shows 16 files.
- [ ] **Step 5: Commit.**
  - Paths: `ui/lang/desktop/*.json`.
  - Subject: `i18n(infra): add topology lane and auto layout keys`.

---

## Task 3: Canvas: lane bands and the auto layout on the map

**Files:**
- Modify: `ui/js/desktop/apps/infra-canvas.js`
- Modify: `ui/js/desktop/apps/infra.js`
- Modify: `ui/css/desktop-app-infra-canvas.css`

- [ ] **Step 1: Impact check** for `renderNow`, `create` (InfraAppCanvas) and `dispose` (canvas). Confirm with:
  ```bash
  cd "/c/Users/Andi/Documents/repo/AuraGo/.worktrees/infra-redesign-ui" && git grep -n -e "sceneData()" -e "InfraAppCanvas" -e "app.canvas\." -- ui/js/desktop/apps
  ```
  Expected users: library (sceneData), gestures, inspector, menus, import and infra.js. All changes are additive; scene cards now also include the auto-laid Topology cards.

- [ ] **Step 2: Add the lane label helper in `infra.js`.**
  - Where the `app` context is built, before any module is created, add:
    ```js
            app.laneLabel = lane => ({ internet: t('desktop.infra_lane_internet'), network: t('desktop.infra_lane_network'), compute: t('desktop.infra_lane_compute'), clients: t('desktop.infra_lane_clients') })[lane] || lane;
    ```
  - Every key is a literal `t()` call, so the key scanners see them.

- [ ] **Step 3: Canvas markup and state.** All in `infra-canvas.js`:
  1. In the `holder.innerHTML` template, insert `<div class="infra-lanes" data-lanes aria-hidden="true"></div>` as the first child of `<div class="infra-scene" data-scene>`, before `[data-regions]`.
  2. Change `emptyScene` to `const emptyScene = () => ({ cards: [], cardById: new Map(), regions: [], regionRects: new Map(), regionDepth: new Map(), lines: [], lanes: [], bounds: null });`.
  3. After the line that declares `regionsEl`, add:
     ```js
             const lanesEl = q('[data-lanes]');
             const laneEls = new Map();
             let laneTarget = -1, autoMemo = { nodes: null, edges: null, sig: '', value: new Map() };
     ```

- [ ] **Step 4: Add the topology section** directly above `function renderNow()`:

```js
        // ---- topology lanes and the auto layout ------------------------------------------
        // topologyAuto: computed slots for assets without a topology placement,
        // memoized on the node and edge arrays and the placed positions.
        function topologyAuto() {
            const { nodes, edges } = store.state;
            const placementOf = id => store.effectivePlacement(id, 'topology');
            let sig = '';
            for (const node of nodes) { const p = placementOf(node.id); if (p) sig += `${node.id}:${Math.round(p.x)},${Math.round(p.y)}\x1f`; }
            if (autoMemo.nodes === nodes && autoMemo.edges === edges && autoMemo.sig === sig) return autoMemo.value;
            const value = model.topologyAutoPositions(nodes, edges, { placementOf, guests: store.guests() });
            autoMemo = { nodes, edges, sig, value };
            return value;
        }
        function patchLanes(lanes) {
            const live = new Set();
            for (const lane of lanes) {
                live.add(lane.id);
                let entry = laneEls.get(lane.id);
                if (!entry) {
                    const band = document.createElement('div');
                    band.className = 'infra-lane';
                    band.dataset.lane = lane.id;
                    band.dataset.laneIndex = String(lane.index);
                    band.innerHTML = `<span class="infra-lane-label">${esc(app.laneLabel(lane.id))}</span>`;
                    lanesEl.append(band);
                    entry = { band, index: lane.index, key: '' };
                    laneEls.set(lane.id, entry);
                }
                const r = lane.rect, key = `${r.x},${r.y},${r.w},${r.h}`;
                if (entry.key !== key) {
                    entry.key = key;
                    Object.assign(entry.band.style, { left: px(r.x), top: px(r.y), width: px(r.w), height: px(r.h) });
                }
                entry.band.classList.toggle('is-drop-target', entry.index === laneTarget);
            }
            for (const [id, entry] of laneEls) if (!live.has(id)) { entry.band.remove(); laneEls.delete(id); }
        }
        // setLaneTarget highlights the lane a dragged card would land in (-1 clears).
        function setLaneTarget(index) {
            laneTarget = Number.isInteger(index) ? index : -1;
            for (const entry of laneEls.values()) entry.band.classList.toggle('is-drop-target', entry.index === laneTarget);
        }
        // isAutoPlaced: the card is drawn by the Topology auto layout, not saved.
        const isAutoPlaced = id => app.mapView() === 'topology' && last.cardById.has(id) && !store.effectivePlacement(id, 'topology');
```

- [ ] **Step 5: Feed the auto positions into `renderNow`.** In `renderNow()`:
  1. Replace the `last = model.buildScene({ … placementOf: id => store.effectivePlacement(id, view), … })` statement with:
     ```js
                 const auto = view === 'topology' ? topologyAuto() : null;
                 last = model.buildScene({ nodes: store.state.nodes, edges: store.state.edges, view,
                     placementOf: id => store.effectivePlacement(id, view) || auto?.get(id) || null, parentOf: id => store.effectiveParent(id, view), guests: store.guests() });
     ```
  2. Prefix the patch line with `patchLanes(last.lanes || []);`, so it reads `patchLanes(last.lanes || []); patchRegions(last.regions, force); patchCards(last.cards, force); patchLines(last.lines, force);`.

- [ ] **Step 6: Wire dispose and the API.**
  - In `dispose()`, add `laneEls.clear();` next to the other `clear()` calls.
  - In the returned object, add `setLaneTarget, isAutoPlaced` after `setRovingCard`.

- [ ] **Step 7: Add the lane CSS.** Append to `ui/css/desktop-app-infra-canvas.css`; tokens only:

```css
.infra-lanes { position: absolute; left: 0; top: 0; width: 0; height: 0; pointer-events: none; }
.infra-lane { position: absolute; border-top: 1px solid var(--vd-theme-border); pointer-events: none; }
.infra-lane[data-lane-index="3"] { border-bottom: 1px solid var(--vd-theme-border); }
.infra-lane[data-lane-index="1"], .infra-lane[data-lane-index="3"] { background: color-mix(in srgb, var(--vd-text) 4%, transparent); }
.infra-lane.is-drop-target { background: var(--vd-theme-accent-soft); box-shadow: inset 0 0 0 2px var(--vd-theme-accent-border); }
.infra-lane-label { position: absolute; left: var(--ds-space-3); top: var(--ds-space-3); padding: 2px var(--ds-space-2); border: 1px solid var(--vd-theme-border); border-radius: var(--ds-radius-full); background: var(--vd-theme-panel-bg-strong); color: var(--vd-theme-muted); font: 600 14px/1.4 var(--vd-font-ui); white-space: nowrap; }
```

- [ ] **Step 8: Verify.**
  - Run `node --check` on `infra-canvas.js` and `infra.js`, then V-core.
  - Then run `AURAGO_RUN_BROWSER_SMOKE=1 go test ./ui -run 'TestDesktopInfra(App|CanvasGestures)Browser' -count=1 -timeout 20m`. Both must pass: the main test switches to Topology and back, and the canvas test is unaffected.

- [ ] **Step 9: Commit.**
  - Paths: `ui/js/desktop/apps/infra-canvas.js`, `ui/js/desktop/apps/infra.js`, `ui/css/desktop-app-infra-canvas.css`.
  - Subject: `feat(infra): draw topology lanes and lay out unplaced assets by connection order`.

---

## Task 4: Gestures, library, menus and inspector for Topology; retire `topologyPositions`

**Files:**
- Modify: `ui/js/desktop/apps/infra-gestures.js`
- Modify: `ui/js/desktop/apps/infra-library.js`
- Modify: `ui/js/desktop/apps/infra-menus.js`
- Modify: `ui/js/desktop/apps/infra-inspector.js`
- Modify: `ui/js/desktop/apps/infra-model.js`
- Modify: `scripts/test-infra-ui.mjs`

- [ ] **Step 1: Impact check** for `arrange`, `commitPlan`, `enterMoveMode`, `removeFromMap` (gestures), `computeSections` (library), `assetItems` (menus), `overviewHTML` (inspector) and `topologyPositions` (model). Confirm with:
  ```bash
  cd "/c/Users/Andi/Documents/repo/AuraGo/.worktrees/infra-redesign-ui" && git grep -n -e "topologyPositions" -e "TOPOLOGY_LAYER" -e "function arrange" -e "function commitPlan" -e "function enterMoveMode" -e "function removeFromMap" -e "infra_remove_from_map" -e "place-selected" -e "effectivePlacement" -- ui/js/desktop/apps scripts
  ```
  Record every `effectivePlacement` use in `infra-gestures.js`; Step 3 depends on it.

- [ ] **Step 2: Arrange in Topology** (`infra-gestures.js`).
  1. At the top of `arrange()`, directly after `const view = app.mapView();`, insert `if (view === 'topology') { arrangeTopology(); return; }`.
  2. In the positions line of `arrange()`, replace `view === 'topology' ? model.topologyPositions(targets, obstacles) : model.gridPositions(targets, obstacles, store.state.nodes.length)` with `model.gridPositions(targets, obstacles, store.state.nodes.length)`.
  3. Add this function right after `arrange()`:

```js
        // arrangeTopology re-runs the lane layout for the selection or every card
        // as one undoable layout save; cards Infra laid out before are undone
        // back to the automatic layout (unplace).
        function arrangeTopology() {
            if (!store.canWrite()) return;
            const view = 'topology', scene = canvas.sceneData();
            const positions = model.topologyArrange({ cards: scene.cards, selection: store.state.selection, nodeById: store.nodeById, edges: store.state.edges, guestsOf: store.guests().guestsOf, placementOf: id => store.effectivePlacement(id, view) });
            if (!positions.size) return;
            const before = new Map(), after = new Map();
            let undoable = true;
            for (const [id, pos] of positions) {
                const intent = store.currentIntent(id, view, false) || (store.features.unplace ? { unplace: true } : null);
                if (!intent) undoable = false;
                before.set(id, intent);
                after.set(id, pos);
            }
            const label = t('desktop.infra_arranged');
            const command = window.InfraAppStore.commands.moveNodes(view, before, after, label);
            command.undoable = undoable;
            void store.dispatch(command);
            app.announce(label);
        }
```

- [ ] **Step 3: Origins come from the scene.**
  - Card drag, move mode (`enterMoveMode`) and keyboard nudging must take each card's starting point from `canvas.sceneData().cardById.get(id)` (the drawn position). They must never use `store.effectivePlacement`, because that returns `null` for auto-laid Topology cards.
  - For each Step 1 hit that reads a card's origin through `effectivePlacement`, replace it with `const card = canvas.sceneData().cardById.get(id); if (!card) continue;` (or `return` in a single-id function) and use `card.x`/`card.y`.
  - Leave the region and parent uses in `planDrop`/`planRegionMove` alone; they are Map-only.

- [ ] **Step 4: Highlight the target lane** (`infra-gestures.js`).
  - In the card-drag move frame, right after the existing `canvas.setDropTarget(…)` call, add `if (app.mapView() === 'topology') canvas.setLaneTarget(model.laneAt(positions.get(primaryId).y));`. Use the drag's own names for the position map and the primary id.
  - In move mode, after each arrow step's `canvas.moveTemp(...)`, add the same line for the first moved id.
  - At every end of a card drag and of move mode (commit, cancel, `cancel()`, `dispose()`), add `canvas.setLaneTarget(-1);`.

- [ ] **Step 5: Announce lane moves** (`commitPlan` in `infra-gestures.js`).
  1. Insert at the start of `commitPlan(view, plan, kind)`:
     ```js
                 const scene = canvas.sceneData();
                 const laneMove = (() => {
                     if (view !== 'topology' || kind !== 'move' || plan.moves.length !== 1) return null;
                     const id = plan.moves[0].id, card = scene.cardById.get(id), to = plan.after.get(id);
                     if (!card || !to || model.laneAt(card.y) === model.laneAt(to.y)) return null;
                     return model.fill(t('desktop.infra_moved_to_lane'), { name: store.nodeById(id)?.name || id, lane: app.laneLabel(model.topologyLanes()[model.laneAt(to.y)]) });
                 })();
     ```
  2. Change the label declaration to `const label = laneMove || (<the existing label expression>);`.

- [ ] **Step 6: Unplacing in Topology.**
  - **Label in `removeFromMap(ids)`** (gestures): replace `const label = model.fill(t('desktop.infra_removed_from_map'), { count: before.size });` with:
    ```js
                const label = view === 'topology' ? model.fill(t('desktop.infra_topology_auto_placed'), { count: before.size }) : model.fill(t('desktop.infra_removed_from_map'), { count: before.size });
    ```
  - **Context menu** (`assetItems` in `infra-menus.js`): set the unplace item's label to `app.mapView() === 'topology' ? t('desktop.infra_topology_auto_place') : t('desktop.infra_remove_from_map')`. Keep it hidden unless `store.effectivePlacement(id, app.mapView())` is set; auto-laid cards have none.
  - **Place items** (`placeItems`): change nothing, because places are not drawn in Topology.

- [ ] **Step 7: Library** (`computeSections` in `infra-library.js`). The To place list is empty in Topology:
  - Replace the declaration of the To place list `const toPlace = <filter expression>` with `const toPlace = view === 'topology' ? [] : <the same filter expression>`.
  - Remove the now-dead condition "not a place when `view === 'topology'`" from that filter.

- [ ] **Step 8: Inspector Overview** (`overviewHTML` in `infra-inspector.js`).
  - **Placement fact:** where the `t('desktop.infra_placement')` value is built, use `t('desktop.infra_topology_auto')` when `app.canvas.isAutoPlaced(node.id)`. Otherwise keep the current text (`t('desktop.infra_position')` with x/y, or `t('desktop.infra_unplaced')`).
  - **Place on map:** render `[data-action="place-selected"]` only when `app.mapView() !== 'topology'`.

- [ ] **Step 9: Retire the old helper.**
  - In `infra-model.js`:
    - Delete the `TOPOLOGY_LAYER` constant and the whole `topologyPositions` function.
    - Remove `topologyPositions` from the export object.
    - Keep `gridPositions`, `overlaps`, `gridKey` and `blockedGrid`.
  - In `scripts/test-infra-ui.mjs`, delete the block that starts with `const topology = model.topologyPositions([` and ends with `placedAt('backup', 110, 640);`. This removal is deliberate: the Task 1 topology block replaces the layered-layout pin.
  - Run `git grep -n topologyPositions -- ui scripts`. Expected: no output.

- [ ] **Step 10: Verify.**
  - Run `node --check` on the five JS files, then V-core.
  - Run `wc -l ui/js/desktop/apps/infra-gestures.js ui/js/desktop/apps/infra-model.js`. Both must be below 1100.
  - Run `AURAGO_RUN_BROWSER_SMOKE=1 go test ./ui -run 'TestDesktopInfra(App|CanvasGestures|ImportAndCleanup|InspectorAndList)Browser' -count=1 -timeout 30m`.

- [ ] **Step 11: Commit.**
  - Paths: `ui/js/desktop/apps/infra-gestures.js`, `ui/js/desktop/apps/infra-library.js`, `ui/js/desktop/apps/infra-menus.js`, `ui/js/desktop/apps/infra-inspector.js`, `ui/js/desktop/apps/infra-model.js`, `scripts/test-infra-ui.mjs`.
  - Subject: `feat(infra): arrange topology by lanes, highlight lane drops and retire the layered topology helper`.

---

## Task 5: Topology browser test

**Files:** create `ui/desktop_infra_topology_browser_test.go`.

- [ ] **Step 1: Write the test.**

```go
package ui

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/go-rod/rod/lib/proto"
)

// infraGermanWord reads one desktop locale text; the browser fixture runs in German.
func infraGermanWord(t *testing.T, key string) string {
	t.Helper()
	var words map[string]any
	if err := json.Unmarshal([]byte(readDesktopAssetText(t, "lang/desktop/de.json")), &words); err != nil {
		t.Fatal(err)
	}
	text, _ := words[key].(string)
	if text == "" {
		t.Fatalf("de.json lacks %s", key)
	}
	return text
}

func infraTopologyFixture() *infraAppBrowserFixture {
	fixture := newInfraAppBrowserFixture()
	device := func(id, name, icon string, x, y float64) map[string]any {
		return infraPanelsNode(id, "device", name, icon, "", infraPanelsAt(x, y), nil)
	}
	vm := infraPanelsNode("vm1", "vm", "Lab VM", "vm", "", nil, nil)
	vm["host_id"] = "pve"
	fixture.nodes = []map[string]any{
		infraCanvasFixtureNode("room", "room", "Living room", "room", map[string]any{"x": 1400, "y": 0, "w": 360, "h": 240, "pinned": true}, ""),
		device("inet", "Internet", "internet", 120, 120),
		device("ont", "Fiber ONT", "modem-ont", 360, 120),
		device("router", "Edge router", "router", 600, 120),
		device("switch", "Core switch", "managed-switch", 840, 120),
		device("ap", "Attic access point", "access-point", 1080, 120),
		device("nas", "Basement NAS", "nas", 120, 360),
		device("pve", "Proxmox host", "generic-device", 360, 360),
		device("laptop", "Admin laptop", "laptop", 600, 360),
		device("tv", "Living room TV", "tv", 840, 360),
		vm,
		infraPanelsNode("ct-orphan", "container", "watchtower", "container", "", nil, infraPanelsSource("docker", "engine-b", "w1", "watchtower")),
	}
	edge := func(id, kind, from, to string) map[string]any {
		return map[string]any{"id": id, "version": 1, "kind": kind, "from": from, "to": to}
	}
	fixture.edges = []map[string]any{
		edge("e1", "ethernet", "inet", "ont"), edge("e2", "fiber", "ont", "router"), edge("e3", "ethernet", "router", "switch"),
		edge("e4", "ethernet", "switch", "ap"), edge("e5", "ethernet", "switch", "nas"), edge("e6", "ethernet", "pve", "switch"),
		edge("e7", "wifi", "ap", "laptop"), edge("e8", "ethernet", "switch", "tv"), edge("e9", "depends_on", "tv", "nas"),
	}
	return fixture
}

// TestDesktopInfraTopologyBrowser covers spec §6:
//   - four labelled lanes are drawn behind the cards;
//   - assets without a topology placement are laid out by connection order,
//     and nothing is written;
//   - guests stay nested and places and hostless containers stay out;
//   - dragging into another lane saves a pinned placement;
//   - Arrange keeps that lane while it re-lays everything else in one
//     undoable save.
func TestDesktopInfraTopologyBrowser(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	fixture := infraTopologyFixture()
	page := infraPanelsPage(t, fixture)
	defer page.Close()
	layoutCalls := func() []map[string]any {
		fixture.mu.Lock()
		defer fixture.mu.Unlock()
		return append([]map[string]any(nil), fixture.layoutCalls...)
	}
	waitCalls := func(want int) []map[string]any {
		t.Helper()
		deadline := time.Now().Add(8 * time.Second)
		for {
			if calls := layoutCalls(); len(calls) >= want {
				time.Sleep(250 * time.Millisecond) // a stray extra save would land here
				return layoutCalls()
			}
			if time.Now().After(deadline) {
				t.Fatalf("waited for %d layout saves, got %d", want, len(layoutCalls()))
			}
			time.Sleep(50 * time.Millisecond)
		}
	}
	item := func(call map[string]any, id string) map[string]any {
		nodes, _ := call["nodes"].([]map[string]any)
		for _, node := range nodes {
			if node["id"] == id {
				return node
			}
		}
		return nil
	}

	page.MustEval(`()=>document.querySelector('[data-view="topology"]').click()`)
	page.MustWait(`()=>document.querySelectorAll('[data-lanes] [data-lane]').length===4&&!!document.querySelector('[data-node="tv"]')`)
	page.MustEval(`()=>document.querySelector('[data-action="fit"]').click()`)

	labels, _ := json.Marshal([][2]string{
		{"internet", infraGermanWord(t, "desktop.infra_lane_internet")},
		{"network", infraGermanWord(t, "desktop.infra_lane_network")},
		{"compute", infraGermanWord(t, "desktop.infra_lane_compute")},
		{"clients", infraGermanWord(t, "desktop.infra_lane_clients")},
	})
	if got := page.MustEval(`()=>JSON.stringify([...document.querySelectorAll('[data-lanes] [data-lane]')].map(lane=>[lane.dataset.lane,lane.querySelector('.infra-lane-label').textContent.trim()]))`).Str(); got != string(labels) {
		t.Fatalf("lanes = %s; want %s", got, labels)
	}
	if !page.MustEval(`()=>{const first=document.querySelector('[data-scene]').firstElementChild;return first.matches('[data-lanes]')&&first.getAttribute('aria-hidden')==='true';}`).Bool() {
		t.Fatal("lanes must be the first scene layer, behind the cards, and hidden from assistive technology")
	}

	const wantAuto = `{"inet":"-96px,96px","ont":"120px,96px","router":"-216px,576px","switch":"0px,576px","ap":"216px,576px","nas":"-96px,1056px","pve":"120px,1056px","tv":"-96px,1536px","laptop":"120px,1536px"}`
	if got := page.MustEval(`()=>JSON.stringify(Object.fromEntries(['inet','ont','router','switch','ap','nas','pve','tv','laptop'].map(id=>{const card=document.querySelector('[data-node="'+id+'"]');return [id,card?card.style.left+','+card.style.top:'missing'];})))`).Str(); got != wantAuto {
		t.Fatalf("topology auto layout = %s; want %s", got, wantAuto)
	}
	if got := page.MustEval(`()=>{const lane=document.querySelector('[data-lane="network"]').getBoundingClientRect(),card=document.querySelector('[data-node="router"]').getBoundingClientRect();return JSON.stringify({guest:!!document.querySelector('[data-node="vm1"]'),orphan:!!document.querySelector('[data-node="ct-orphan"]'),place:!!document.querySelector('[data-node="room"]')||document.querySelectorAll('[data-region]').length>0,stack:!!document.querySelector('[data-toggle-host-services="pve"]'),toPlace:document.querySelectorAll('[data-section-list="to-place"] [data-library-item]').length,inLane:card.top>=lane.top&&card.bottom<=lane.bottom});}`).Str(); got != `{"guest":false,"orphan":false,"place":false,"stack":true,"toPlace":0,"inLane":true}` {
		t.Fatalf("topology must nest guests, hide places and hostless containers, and leave nothing to place: %s", got)
	}
	if n := len(layoutCalls()); n != 0 {
		t.Fatalf("opening Topology saved %d layouts; the auto layout must not write", n)
	}

	// Dragging a card into another lane saves a pinned topology placement.
	tx, ty := infraPanelsCenter(page, `[data-node="tv"]`)
	_, ly := infraPanelsCenter(page, `[data-lane="network"]`)
	page.Mouse.MustMoveTo(tx, ty).MustDown(proto.InputMouseButtonLeft)
	page.Mouse.MustMoveTo(tx, (ty+ly)/2)
	page.Mouse.MustMoveTo(tx, ly)
	page.MustWait(`()=>document.querySelector('[data-lane="network"]').classList.contains('is-drop-target')`)
	page.Mouse.MustUp(proto.InputMouseButtonLeft)
	calls := waitCalls(1)
	tv := item(calls[0], "tv")
	if calls[0]["view"] != "topology" || tv == nil || tv["pinned"] != true {
		t.Fatalf("dragging into another lane must save a pinned topology placement: %#v", calls[0])
	}
	droppedY, _ := tv["y"].(float64)
	if droppedY < 480 || droppedY >= 960 || int(droppedY)%24 != 0 {
		t.Fatalf("the card landed at y=%v; want a grid point inside the network lane (480-959)", droppedY)
	}
	page.MustWait(`()=>!document.querySelector('[data-lane].is-drop-target')`)

	// Arrange without a selection: one save, the dragged card keeps its lane.
	page.MustEval(`()=>document.querySelector('[data-action="close-inspector"]')?.click()`)
	page.MustWait(`()=>!document.querySelector('[data-node][aria-pressed="true"]')`)
	page.MustEval(`()=>document.querySelector('[data-action="more"]').click()`)
	page.MustWait(`()=>{const item=document.querySelector('[data-menu="more"] [data-action="arrange"]');return !!item&&!item.disabled&&item.offsetParent!==null;}`)
	page.MustEval(`()=>document.querySelector('[data-menu="more"] [data-action="arrange"]').click()`)
	calls = waitCalls(2)
	arranged := calls[1]
	if nodes, _ := arranged["nodes"].([]map[string]any); len(nodes) != 9 {
		t.Fatalf("Arrange without a selection must save all nine cards in one call: %#v", arranged)
	}
	for id, want := range map[string][3]any{
		"tv":     {float64(336), float64(576), true},
		"router": {float64(-312), float64(576), false},
		"inet":   {float64(-96), float64(96), false},
		"laptop": {float64(0), float64(1536), false},
	} {
		got := item(arranged, id)
		if got == nil || got["x"] != want[0] || got["y"] != want[1] || got["pinned"] != want[2] {
			t.Fatalf("Arrange placed %s at %#v; want x=%v y=%v pinned=%v", id, got, want[0], want[1], want[2])
		}
	}

	// Undo hands the auto cards back to the automatic layout.
	page.MustEval(`()=>document.querySelector('[data-action="undo"]').click()`)
	calls = waitCalls(3)
	undone := calls[2]
	if got := item(undone, "router"); got == nil || got["unplace"] != true {
		t.Fatalf("undoing Arrange must unplace the cards Infra laid out before: %#v", undone)
	}
	if got := item(undone, "tv"); got == nil || got["y"] != droppedY || got["pinned"] != true {
		t.Fatalf("undoing Arrange must put the dragged card back: %#v", undone)
	}
	page.MustWait(`()=>{const card=document.querySelector('[data-node="router"]');return card.style.left==='-216px'&&card.style.top==='576px';}`)
	closeInfraBrowserWindow(t, page)
	requireInfraBrowserClean(t, page)
}
```

- [ ] **Step 2: Verify.**
  - Run `gofmt -l ui/desktop_infra_topology_browser_test.go` (expected: no output) and `go vet ./ui`.
  - Run `AURAGO_RUN_BROWSER_SMOKE=1 go test ./ui -run 'TestDesktopInfraTopologyBrowser' -count=1 -timeout 15m`.
  - If it is flaky, reproduce it with Windows animations on and off (project memory: Windows motion vs browser tests).
- [ ] **Step 3: Commit.**
  - Paths: `ui/desktop_infra_topology_browser_test.go`.
  - Subject: `test(infra): cover topology lanes, auto layout, lane drops and topology arrange`.

---

## Task 6: CSS token, typography and motion guard

**Files:**
- Modify: `scripts/test-infra-ui.mjs`
- Modify: `ui/css/desktop-app-infra.css`, `ui/css/desktop-app-infra-canvas.css` (only what the guard flags)
- Modify: `ui/js/desktop/apps/AGENTS.md` (theme bridge list)

- [ ] **Step 1: Replace the CSS guard.**
  - In `scripts/test-infra-ui.mjs`, delete the existing guard loop that starts with `for (const file of ['desktop-app-infra.css', 'desktop-app-infra-canvas.css']) {` (Plan B's two-file list) and runs to its closing `}`.
  - Insert this block in its place:

```js
// ---- CSS tokens, typography and motion (spec §12) --------------------------------
{
    const cssDir = path.join(root, 'ui/css');
    const infraCss = fs.readdirSync(cssDir).filter(file => /^desktop-app-infra.*\.css$/.test(file)).sort();
    assert.ok(infraCss.length >= 2, `expected the Infra stylesheets, found ${infraCss.join(', ')}`);
    const stripMixes = text => text.replace(/color-mix\((?:[^()]|\([^()]*\))*\)/g, 'color-mix()');
    const NAMED = /(^|[\s,(])(white|black|red|green|blue|gray|grey|silver|orange|yellow|purple|pink|navy|teal|maroon|olive|lime|aqua|fuchsia)(?=$|[\s,)!;])/i;
    const COLOR_PROPS = /^(color|background|background-color|border|border-(top|right|bottom|left)|border(-(top|right|bottom|left))?-color|outline|outline-color|fill|stroke|box-shadow|text-shadow|caret-color|accent-color|text-decoration-color|column-rule)$/;
    const MOTION_PROPS = /^(transition|transition-property|transition-duration|animation|animation-name|animation-duration)$/;
    // leafRules: every rule that holds declarations, with the at-rules around it.
    const leafRules = css => {
        const rules = [], stack = [];
        let buf = '';
        for (const ch of css) {
            if (ch === '{') { stack.push(buf.trim()); buf = ''; }
            else if (ch === '}') { const head = stack.pop() || ''; if (buf.trim()) rules.push({ selector: head, body: buf.trim(), context: [...stack] }); buf = ''; }
            else buf += ch;
        }
        return rules;
    };
    const splitSelectors = selector => {
        const parts = [];
        let depth = 0, current = '';
        for (const ch of selector) {
            if (ch === '(') depth += 1;
            if (ch === ')') depth -= 1;
            if (ch === ',' && depth === 0) { parts.push(current.trim()); current = ''; } else current += ch;
        }
        if (current.trim()) parts.push(current.trim());
        return parts;
    };
    for (const file of infraCss) {
        const css = fs.readFileSync(path.join(cssDir, file), 'utf8').replace(/\/\*[\s\S]*?\*\//g, '');
        const outsideMixes = stripMixes(css);
        assert.deepEqual(outsideMixes.match(/#[0-9a-fA-F]{3,8}\b/g) || [], [], `${file} uses hex colours outside color-mix()`);
        assert.deepEqual(outsideMixes.match(/\b(?:rgba?|hsla?|hwb|lab|lch|oklab|oklch)\(\s*[\d.]/g) || [], [], `${file} uses literal colour functions instead of tokens`);
        assert.ok(!/var\(--vd-font[,)]/.test(css), `${file} uses --vd-font instead of --vd-font-ui`);
        assert.ok(!/copper/i.test(css), `${file} still uses the copper accent`);
        assert.ok(!/\[data-theme[=\]]/.test(css), `${file} keys rules on a theme id; use the --vd-theme-* tokens`);
        const named = [], labels = [], ungated = [];
        for (const rule of leafRules(css).filter(item => !item.context.some(head => head.startsWith('@keyframes')))) {
            for (const [, prop, rawValue] of rule.body.matchAll(/([a-z-]+)\s*:\s*([^;]+)/g)) {
                const name = prop.toLowerCase(), value = rawValue.trim();
                if (name.startsWith('--')) continue;
                if (COLOR_PROPS.test(name) && NAMED.test(stripMixes(value))) named.push(`${rule.selector} { ${name}: ${value} }`);
                if (name === 'text-transform' && /uppercase/i.test(value)) labels.push(`${rule.selector} { text-transform: uppercase }`);
                if ((name === 'font' || name === 'font-family') && /monospace/i.test(value)) labels.push(`${rule.selector} { ${name}: ${value} }`);
                if (MOTION_PROPS.test(name) && !/^(none|0m?s)\b/i.test(value)) {
                    const gated = rule.context.some(head => /prefers-reduced-motion\s*:\s*no-preference/.test(head)) && splitSelectors(rule.selector).every(part => part.includes(':not([data-animations="false"])'));
                    if (!gated) ungated.push(`${rule.selector} { ${name} }`);
                }
            }
        }
        assert.deepEqual(named, [], `${file} uses named colours instead of tokens`);
        assert.deepEqual(labels, [], `${file} has uppercase or monospace labels; use 13 px/600 sentence case with --vd-font-ui`);
        assert.deepEqual(ungated, [], `${file} has motion outside @media (prefers-reduced-motion: no-preference) with body:not([data-animations="false"])`);
    }
}
```

- [ ] **Step 2: Run the guard.** Run `node scripts/test-infra-ui.mjs` and fix every reported line in the stylesheets, following these rules:
  - **Colours:** replace each colour with the matching token from spec §12:
    - surfaces: `--vd-theme-app-bg`, `--vd-theme-panel-bg(-strong)`, `--vd-theme-control-bg/hover`, `--vd-theme-overlay-bg`;
    - lines: `--vd-theme-border(-strong)`;
    - text: `--vd-text`, `--vd-theme-muted`;
    - accent: `--vd-accent`, `--vd-accent-contrast`, `--vd-theme-accent-soft/border`;
    - status: `--ds-color-success/warning/danger/info-fg`.
    A tint is `color-mix(in srgb, <token> N%, transparent)`.
  - **Labels:** an uppercase or monospace label becomes `font: 600 13px/1.3 var(--vd-font-ui); color: var(--vd-theme-muted); text-transform: none; letter-spacing: 0;`. Numbers that need alignment use `font-variant-numeric: tabular-nums` instead of monospace.
  - **Motion:** wrap ungated motion as `@media (prefers-reduced-motion: no-preference) { body:not([data-animations="false"]) <selector> { … } }`, with the static parts left outside.
  - **Theme ids:** a `[data-theme…]` rule is deleted, and its effect is expressed through tokens. If no token can express it, record it as a theme-token follow-up for the handoff; do not keep the rule.

- [ ] **Step 3: Add Infra to the theme bridge contract.**
  - Run `git ls-files -v ui/js/desktop/apps/AGENTS.md`. It must start with `H `; otherwise run `git update-index --no-assume-unchanged ui/js/desktop/apps/AGENTS.md`.
  - In `### App theme bridge contract`, change the first bullet's list `…, Nasscad and Mailer read` to `…, Nasscad, Mailer and Infra read`.

- [ ] **Step 4: Verify.**
  - Run V-core.
  - Run `AURAGO_RUN_BROWSER_SMOKE=1 go test ./ui -run 'TestDesktopInfraAppBrowser' -count=1 -timeout 20m`. It covers the readable-layout floors after the label changes.

- [ ] **Step 5: Commit.**
  - Paths: `scripts/test-infra-ui.mjs`, `ui/css/desktop-app-infra.css`, `ui/css/desktop-app-infra-canvas.css`, `ui/js/desktop/apps/AGENTS.md`.
  - Subject: `test(infra): guard colour tokens, label typography and gated motion in the Infra stylesheets`.

---

## Task 7: Accessibility fixes

**Files:**
- Modify: `ui/js/desktop/apps/infra-store.js`
- Modify: `ui/js/desktop/apps/infra.js`
- Modify: `ui/js/desktop/apps/infra-dialogs.js`
- Modify: `ui/js/desktop/apps/infra-import.js`
- Modify: `scripts/test-infra-ui.mjs`

- [ ] **Step 1: Impact check** for `loadSnapshot` (store), `openDialog` (dialogs), `open`/`close` (InfraAppImport), and the `toggle-library` branch, scrim handler and `paintShell` in `infra.js`. Confirm with:
  ```bash
  cd "/c/Users/Andi/Documents/repo/AuraGo/.worktrees/infra-redesign-ui" && git grep -n -e "function loadSnapshot" -e "function openDialog" -e "toggle-library" -e "data-scrim" -e "librarySheet" -e "activeElement" -- ui/js/desktop/apps
  ```

- [ ] **Step 2: Report a failing poll once** (`infra-store.js`, `loadSnapshot`). In the `catch` block, replace `} else { notify(messageFor(error), 'error'); setSync('error'); }` with:
  ```js
                    } else {
                        // A poll that keeps failing reports once; an explicit refresh always reports.
                        if (explicit || state.sync !== 'error') notify(messageFor(error), 'error');
                        setSync('error');
                    }
  ```

- [ ] **Step 3: Add the focus helpers** to `infra.js`. Add them where `app` is built, after `app.laneLabel`:
  ```js
          // focusReturnTarget: where focus goes back when a panel opened from el
          // closes. A menu item hands over to its menu's trigger; anything outside
          // the window hands over to the map.
          app.focusReturnTarget = el => {
              if (!el || el === document.body || !root.contains(el)) return q('[data-stage]');
              const menu = el.closest('[role="menu"], [data-popover]');
              if (menu && menu.id) return root.querySelector(`[aria-controls="${CSS.escape(menu.id)}"]`) || q('[data-stage]');
              return el;
          };
          app.restoreFocus = el => {
              const target = el && el.isConnected && !el.closest('[hidden]') ? el : q('[data-stage]');
              if (target && target.isConnected) target.focus({ preventScroll: true });
          };
  ```

- [ ] **Step 4: Manage focus for the narrow library sheet** (`infra.js`).
  1. Add these two functions next to the root click handler:
     ```js
             function openLibrarySheet() {
                 root.dataset.librarySheet = 'open';
                 q('[data-action="toggle-library"]')?.setAttribute('aria-expanded', 'true');
                 (app.library.el.querySelector('[data-library-select][tabindex="0"]') || app.library.el.querySelector('[data-section-toggle]'))?.focus({ preventScroll: true });
             }
             function closeLibrarySheet({ restoreFocus = true } = {}) {
                 if (root.dataset.librarySheet !== 'open') return;
                 root.dataset.librarySheet = 'closed';
                 const toggle = q('[data-action="toggle-library"]');
                 toggle?.setAttribute('aria-expanded', 'false');
                 if (restoreFocus) toggle?.focus({ preventScroll: true });
             }
     ```
  2. In the `toggle-library` action, the `< 760 px` branch becomes `if (root.dataset.librarySheet === 'open') closeLibrarySheet(); else { app.inspector.close({ restoreFocus: false }); openLibrarySheet(); }`.
  3. The scrim click handler calls `closeLibrarySheet()` instead of setting `data-library-sheet` directly.
  4. Where an inspector opening at < 760 px closes the library sheet (the `selection` routing), call `closeLibrarySheet({ restoreFocus: false })`.
  5. Bind once on `app.library.el`, and remove the listener in `dispose`:
     ```js
             const onLibraryKey = event => {
                 if (event.key !== 'Escape' || root.dataset.librarySheet !== 'open' || event.target.closest('input, select, textarea, [contenteditable="true"]')) return;
                 event.preventDefault(); event.stopPropagation();
                 closeLibrarySheet();
             };
             app.library.el.addEventListener('keydown', onLibraryKey);
     ```
  6. In `paintShell()`, keep the toggle's state honest at every width: `q('[data-action="toggle-library"]')?.setAttribute('aria-expanded', String(root.clientWidth < 760 ? root.dataset.librarySheet === 'open' : root.dataset.libraryCollapsed !== 'true'));`.

- [ ] **Step 5: Return focus after a dialog closes** (`infra-dialogs.js`).
  - Add `let dialogOpener = null;` to the module state.
  - Bind this once in `create(app)` on the `[data-dialog]` element, and remove the listener in `dispose()`:
    ```js
            const onDialogClose = () => { const target = dialogOpener; dialogOpener = null; app.restoreFocus(target); };
            dialogEl.addEventListener('close', onDialogClose);
    ```
    Use the module's own variable name for the dialog element.
  - In `openDialog(...)`, before `showModal()`, add `if (!dialogEl.open) dialogOpener = app.focusReturnTarget(document.activeElement);`.

- [ ] **Step 6: Give the import sheet a reachable opener** (`infra-import.js`).
  - In `open()`, replace the statement that remembers `document.activeElement` with `opener = app.focusReturnTarget(document.activeElement);`. Use the module's existing variable name.
  - In `close()`, replace the focus call with `app.restoreFocus(opener);`.

- [ ] **Step 7: Audit the live regions and toolbars.** No code change is expected; record the output in your notes.
  ```bash
  cd "/c/Users/Andi/Documents/repo/AuraGo/.worktrees/infra-redesign-ui" && git grep -n -e 'aria-live' -e 'role="status"' -e 'role="alert"' -e 'role="log"' -e 'role="toolbar"' -- ui/js/desktop/apps/infra*.js
  ```
  - **Live regions:** expected only `[data-announce]` (`aria-live="polite"`), `[data-layout-chip]` and `.infra-draft-banner` (`role="status"`), plus the inline error paragraphs (`role="alert"`).
    - `[data-live]` must have neither attribute.
    - Any other live region whose text a poll can change is a defect: remove its live role and announce state changes through `app.announce` instead.
  - **Toolbars:** every `role="toolbar"` must have a roving tab stop. Arrow keys, Home and End must move it the way the canvas tools do (`rovePrimary` in `infra-canvas.js`). If one is missing, implement the same pattern there.

- [ ] **Step 8: Add the store test.** Insert before the final `console.log` of `scripts/test-infra-ui.mjs`:
  ```js
  // ---- store: a failing poll reports once ------------------------------------------
  {
      let fail = true;
      const notices = [], spoken = [];
      const api = async url => {
          if (!String(url).includes('/snapshot')) throw Object.assign(new Error('not_found'), { status: 404, body: { error: 'not_found' } });
          if (fail) throw Object.assign(new Error('provider_unavailable'), { status: 503, body: { error: 'provider_unavailable' } });
          return { nodes: [], edges: [], sources: [], source_hosts: [], capabilities: { write: true } };
      };
      const store = window.InfraAppStore.create({ ctx: { api }, t: key => key });
      store.subscribe(change => {
          if (change.type === 'notice' && change.kind === 'error') notices.push(change.message);
          if (change.type === 'sync' && change.announce) spoken.push(change.announce);
      });
      store.applySnapshot({ nodes: [], edges: [], sources: [], source_hosts: [], capabilities: { write: true } });
      store.state.sync = 'live';
      store.state.visible = true;
      await store.loadSnapshot(false);
      await store.loadSnapshot(false);
      assert.equal(notices.length, 1, 'a poll that keeps failing reports its error once');
      assert.deepEqual(spoken, ['desktop.infra_sync_stale']);
      await store.loadSnapshot(true);
      assert.equal(notices.length, 2, 'an explicit refresh always reports its failure');
      fail = false;
      await store.loadSnapshot(false);
      assert.deepEqual(spoken, ['desktop.infra_sync_stale', 'desktop.infra_sync_recovered']);
      store.dispose();
  }
  ```

- [ ] **Step 9: Verify.**
  - Run `node --check` on the four JS files, then V-core. `node scripts/test-infra-ui.mjs` must exit promptly; a hang means an uncleared poll timer.
  - Run `AURAGO_RUN_BROWSER_SMOKE=1 go test ./ui -run 'TestDesktopInfra(App|ImportAndCleanup|InspectorAndList|UnsavedDraftClose)Browser' -count=1 -timeout 30m`.

- [ ] **Step 10: Commit.**
  - Paths: `ui/js/desktop/apps/infra-store.js`, `ui/js/desktop/apps/infra.js`, `ui/js/desktop/apps/infra-dialogs.js`, `ui/js/desktop/apps/infra-import.js`, `scripts/test-infra-ui.mjs`, plus any toolbar file from Step 7.
  - Subject: `fix(infra): report failing polls once and return focus from sheets, dialogs and the library`.

---

## Task 8: Accessibility and theme contrast browser tests

**Files:** create `ui/desktop_infra_a11y_browser_test.go`.

- [ ] **Step 1: Write the tests.**

```go
package ui

import (
	"net/http"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/input"
	"github.com/go-rod/rod/lib/proto"
)

// infraThemeConfig is one desktop theme setup for the Infra theme checks.
type infraThemeConfig struct {
	name, theme, setting, mode, scheme string
}

// infraReviewThemes are the themes of the visual review (spec §13).
var infraReviewThemes = []infraThemeConfig{
	{"standard", "standard", "", "", "dark"},
	{"fruity-light", "fruity", "appearance.fruity_mode", "light", "light"},
	{"fruity-dark", "fruity", "appearance.fruity_mode", "dark", "dark"},
	{"minimal-light", "minimal", "appearance.minimal_mode", "light", "light"},
	{"minimal-dark", "minimal", "appearance.minimal_mode", "dark", "dark"},
	{"orbit", "orbit", "", "", "dark"},
	{"windows-xp", "windows_xp", "", "", "light"},
	{"amiga", "amiga", "", "", "light"},
}

func applyInfraTheme(page *rod.Page, c infraThemeConfig) {
	page.MustEval(`(theme,setting,mode,scheme)=>{const s=auditTest.state.bootstrap.settings;s['appearance.theme']=theme;if(setting)s[setting]=mode;document.documentElement.dataset.theme=scheme;auditTest.applyDesktopSettings();}`, c.theme, c.setting, c.mode, c.scheme)
	page.MustWait(`(theme,scheme)=>document.body.dataset.theme===theme&&document.body.dataset.scheme===scheme`, c.theme, c.scheme)
}

// infraContrastProbeJS installs window.infraProbe: WCAG contrast of an
// element's text against its composited background, font-size floors and the
// focus ring of the focused element. Any CSS colour syntax is resolved by
// painting it on a 1x1 canvas.
const infraContrastProbeJS = `()=>{
	const canvas=document.createElement('canvas');canvas.width=canvas.height=1;
	const ctx=canvas.getContext('2d',{willReadFrequently:true});
	const rgba=value=>{ctx.clearRect(0,0,1,1);ctx.fillStyle='rgba(0,0,0,0)';ctx.fillStyle=value;ctx.fillRect(0,0,1,1);const d=ctx.getImageData(0,0,1,1).data;return [d[0],d[1],d[2],d[3]/255];};
	const over=(top,bottom)=>{const a=top[3]+bottom[3]*(1-top[3]);if(!a)return [0,0,0,0];return [0,1,2].map(i=>(top[i]*top[3]+bottom[i]*bottom[3]*(1-top[3]))/a).concat([a]);};
	const background=el=>{const layers=[];for(let node=el;node&&node.nodeType===1;node=node.parentElement){const c=rgba(getComputedStyle(node).backgroundColor);if(c[3]>0){layers.push(c);if(c[3]>=0.99)break;}}let out=[255,255,255,1];for(let i=layers.length-1;i>=0;i-=1)out=over(layers[i],out);return out;};
	const lum=c=>{const f=v=>{v/=255;return v<=0.03928?v/12.92:Math.pow((v+0.055)/1.055,2.4);};return 0.2126*f(c[0])+0.7152*f(c[1])+0.0722*f(c[2]);};
	const ratio=(a,b)=>{const x=lum(a),y=lum(b);return (Math.max(x,y)+0.05)/(Math.min(x,y)+0.05);};
	const opacity=el=>{let o=1;for(let node=el;node&&node.nodeType===1;node=node.parentElement)o*=Number(getComputedStyle(node).opacity);return o;};
	const contrast=el=>{const bg=background(el);return Math.round(ratio(over(rgba(getComputedStyle(el).color),bg),bg)*100)/100;};
	window.infraProbe={
		contrast,
		check(pairs){return pairs.flatMap(([label,selector,min])=>{const el=document.querySelector(selector);if(!el||!el.getClientRects().length)return [label+': missing'];if(opacity(el)<0.99)return [label+': faded'];const value=contrast(el);return value>=min?[]:[label+': '+value];});},
		floors(list){return list.flatMap(([label,selector,min])=>{const el=document.querySelector(selector);if(!el)return [label+': missing'];const size=parseFloat(getComputedStyle(el).fontSize);return size>=min?[]:[label+': '+size+'px'];});},
		focusRing(el){if(!el||el===document.body)return {ok:false,reason:'nothing focused'};const style=getComputedStyle(el);const width=parseFloat(style.outlineWidth)||0;if(style.outlineStyle==='none'||width<1)return {ok:false,reason:'outline '+style.outlineStyle+' '+style.outlineWidth,tag:el.tagName};const bg=background(el.parentElement);const value=Math.round(ratio(over(rgba(style.outlineColor),bg),bg)*100)/100;return {ok:value>=3,ratio:value,tag:el.tagName};},
	};
	return true;
}`

// infraFlakySnapshot answers GET /snapshot with 503 while fail is set.
type infraFlakySnapshot struct {
	base http.Handler
	fail atomic.Bool
}

func (h *infraFlakySnapshot) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if h.fail.Load() && r.URL.Path == "/api/desktop/infra/snapshot" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusServiceUnavailable)
		_, _ = w.Write([]byte(`{"error":"provider_unavailable"}`))
		return
	}
	h.base.ServeHTTP(w, r)
}

func infraA11yFixture() *infraAppBrowserFixture {
	fixture := newInfraAppBrowserFixture()
	fixture.nodes = []map[string]any{
		infraCanvasFixtureNode("room-1", "room", "Living room", "room", map[string]any{"x": 400, "y": 120, "w": 480, "h": 320, "pinned": true}, ""),
		infraCanvasFixtureNode("dev-a", "device", "Edge router", "router", map[string]any{"x": 160, "y": 200, "pinned": true}, ""),
		infraCanvasFixtureNode("dev-b", "device", "Core switch", "managed-switch", map[string]any{"x": 160, "y": 420, "pinned": true}, ""),
		infraCanvasFixtureNode("dev-c", "device", "Media box", "mini-pc", map[string]any{"x": 640, "y": 300, "pinned": true}, "room-1"),
		infraCanvasFixtureNode("dev-d", "device", "Cold NAS", "nas", nil, ""),
	}
	fixture.edges = []map[string]any{{"id": "edge-ab", "version": 1, "from": "dev-a", "to": "dev-b", "kind": "ethernet"}}
	return fixture
}

// infraA11yPage opens Infra at 1500x970 and records every scheduled 30 s poll
// in window.infraPolls, so a test can run a poll without waiting for it.
func infraA11yPage(t *testing.T, handler http.Handler) *rod.Page {
	t.Helper()
	page := infraBrowserPageWithFixture(t, handler)
	page.MustSetViewport(1920, 1200, 1, false)
	page.MustEval(`()=>{
		Object.keys(localStorage).filter(key=>key.startsWith('infra.')).forEach(key=>localStorage.removeItem(key));
		window.infraPolls=[];
		const set=window.setTimeout.bind(window);
		window.setTimeout=(fn,delay,...args)=>{if(Number(delay)===30000)window.infraPolls.push(fn);return set(fn,delay,...args);};
	}`)
	openInfraBrowserApp(page)
	page.MustWait(`()=>!!document.querySelector('[data-nodes] [data-node]')`)
	page.MustEval(`()=>{const win=document.querySelector('.vd-window[data-app-id="infra"]');win.classList.remove('maximized','vd-mobile-forced-maximized');win.style.right='auto';win.style.width='1500px';win.style.height='970px';win.style.left='210px';win.style.top='70px';}`)
	page.MustWait(`()=>document.querySelector('.infra-app').clientWidth>1000`)
	page.MustEval(`()=>document.querySelector('[data-action="fit"]').click()`)
	return page
}

func runInfraPoll(page *rod.Page) {
	page.MustEval(`async()=>{const poll=window.infraPolls.at(-1);if(!poll)throw new Error('no 30 s poll was scheduled');await poll();}`)
}

// TestDesktopInfraAccessibilityBrowser covers spec §5.3:
//   - the map is one tab stop and toolbars rove;
//   - tablist structure, and the combobox pattern with focus return;
//   - focus returns after the inspector, the shortcut dialog, the import
//     sheet and the narrow library sheet;
//   - routine polls never announce, while a lost and recovered sync is
//     announced once each;
//   - reduced motion and Window animations off stop every Infra motion.
func TestDesktopInfraAccessibilityBrowser(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	fixture := infraA11yFixture()
	flaky := &infraFlakySnapshot{base: fixture}
	page := infraA11yPage(t, flaky)
	defer page.Close()
	focused := func(selector string) bool {
		return page.MustEval(`selector=>document.activeElement===document.querySelector(selector)`, selector).Bool()
	}

	// One tab stop for the map, one per toolbar, and arrows rove.
	if got := page.MustEval(`()=>{const stage=document.querySelector('[data-stage]');return [stage,...stage.querySelectorAll('button,[tabindex]')].filter(el=>el.tabIndex>=0&&!el.disabled).length;}`).Int(); got != 1 {
		t.Fatalf("the map must be a single tab stop, found %d", got)
	}
	if got := page.MustEval(`()=>JSON.stringify([...document.querySelectorAll('.infra-app [role="toolbar"]')].map(bar=>[...bar.querySelectorAll('button')].filter(button=>!button.disabled&&button.getClientRects().length&&button.tabIndex===0).length).filter(count=>count!==1))`).Str(); got != "[]" {
		t.Fatalf("every toolbar needs exactly one tab stop: %s", got)
	}
	page.MustEval(`()=>document.querySelector('[data-tools] button[tabindex="0"]').focus()`)
	first := page.MustEval(`()=>{const el=document.activeElement;return el.dataset.tool||el.dataset.action;}`).Str()
	page.Keyboard.MustType(input.ArrowRight)
	if !page.MustEval(`(first)=>{const el=document.activeElement;return (el.dataset.tool||el.dataset.action)!==first&&el.tabIndex===0&&!!el.closest('[data-tools]')&&[...document.querySelectorAll('[data-tools] button')].filter(button=>button.tabIndex===0).length===1;}`, first).Bool() {
		t.Fatal("ArrowRight must move the toolbar's single tab stop to the next tool")
	}
	page.Keyboard.MustType(input.End)
	if !page.MustEval(`()=>document.activeElement===[...document.querySelectorAll('[data-tools] button:not(:disabled)')].at(-1)`).Bool() {
		t.Fatal("End must reach the last tool")
	}
	page.Keyboard.MustType(input.Home)
	if !page.MustEval(`()=>document.activeElement===document.querySelector('[data-tools] button:not(:disabled)')`).Bool() {
		t.Fatal("Home must reach the first tool")
	}

	// Tablist structure.
	page.MustEval(`()=>{const card=document.querySelector('[data-node="dev-a"]');card.click();card.focus();}`)
	page.MustWait(`()=>!document.querySelector('[data-inspector]').hidden&&!!document.querySelector('[data-inspector] [role="tablist"]')`)
	if got := page.MustEval(`()=>{const problems=[];for(const list of document.querySelectorAll('.infra-app [role="tablist"]')){const tabs=[...list.querySelectorAll('[role="tab"]')];const stops=tabs.filter(tab=>tab.tabIndex===0),chosen=tabs.filter(tab=>tab.getAttribute('aria-selected')==='true');if(stops.length!==1||chosen.length!==1||stops[0]!==chosen[0])problems.push('roving');for(const tab of tabs){const panel=document.getElementById(tab.getAttribute('aria-controls'));if(!panel||panel.getAttribute('role')!=='tabpanel'||panel.getAttribute('aria-labelledby')!==tab.id)problems.push('link '+tab.id);else if(panel.hidden===(tab.getAttribute('aria-selected')==='true'))problems.push('visibility '+tab.id);}}return JSON.stringify(problems);}`).Str(); got != "[]" {
		t.Fatalf("inspector tabs break the tab pattern: %s", got)
	}

	// Combobox: ARIA 1.2 structure, active option moves, Escape gives focus back.
	page.MustEval(`()=>document.querySelector('[data-inspector-tab="connections"]').click()`)
	page.MustEval(`()=>{const add=document.querySelector('[data-inspector-panel="connections"] [data-action="add-edge"]');add.focus();add.click();}`)
	page.MustWait(`()=>!!document.querySelector('[data-picker] [data-picker-input]')`)
	if got := page.MustEval(`()=>{const input=document.querySelector('[data-picker] [data-picker-input]'),list=document.getElementById(input.getAttribute('aria-controls')),active=document.getElementById(input.getAttribute('aria-activedescendant'));return JSON.stringify({role:input.getAttribute('role'),expanded:input.getAttribute('aria-expanded'),list:list&&list.getAttribute('role'),active:active&&active.getAttribute('role'),selected:active&&active.getAttribute('aria-selected'),focused:document.activeElement===input});}`).Str(); got != `{"role":"combobox","expanded":"true","list":"listbox","active":"option","selected":"true","focused":true}` {
		t.Fatalf("the asset picker is not an ARIA combobox: %s", got)
	}
	before := page.MustEval(`()=>document.querySelector('[data-picker] [data-picker-input]').getAttribute('aria-activedescendant')`).Str()
	page.Keyboard.MustType(input.ArrowDown)
	if after := page.MustEval(`()=>document.querySelector('[data-picker] [data-picker-input]').getAttribute('aria-activedescendant')`).Str(); after == before || after == "" {
		t.Fatalf("ArrowDown must move the active option (before %q, after %q)", before, after)
	}
	page.Keyboard.MustType(input.Escape)
	page.MustWait(`()=>!document.querySelector('[data-picker]')`)
	if !focused(`[data-inspector-panel="connections"] [data-action="add-edge"]`) {
		t.Fatal("closing the picker must give focus back to Add connection")
	}

	// The inspector gives focus back to the card.
	page.MustEval(`()=>document.querySelector('[data-inspector-tab="overview"]').focus()`)
	page.Keyboard.MustType(input.Escape)
	page.MustWait(`()=>document.querySelector('[data-inspector]').hidden`)
	if !focused(`[data-node="dev-a"]`) {
		t.Fatal("closing the inspector must give focus back to the asset card")
	}

	// The shortcut dialog gives focus back to where it was opened.
	page.MustEval(`()=>document.querySelector('[data-node="dev-a"]').dispatchEvent(new KeyboardEvent('keydown',{key:'?',bubbles:true}))`)
	page.MustWait(`()=>document.querySelector('[data-dialog]').open`)
	page.Keyboard.MustType(input.Escape)
	page.MustWait(`()=>!document.querySelector('[data-dialog]').open`)
	if !focused(`[data-node="dev-a"]`) {
		t.Fatal("closing the shortcut sheet must give focus back to where it was opened")
	}

	// The import sheet, opened from the Add menu, gives focus back to the menu button.
	page.MustEval(`()=>{const trigger=document.querySelector('[data-action="add-menu"]');trigger.focus();trigger.click();}`)
	page.MustWait(`()=>!document.querySelector('[data-menu="add"]').hidden`)
	page.MustEval(`()=>document.querySelector('[data-menu="add"] [data-action="import"]').click()`)
	page.MustWait(`()=>!document.querySelector('[data-import-sheet]').hidden`)
	page.MustEval(`()=>document.querySelector('[data-import-search]').focus()`)
	page.Keyboard.MustType(input.Escape)
	page.MustWait(`()=>document.querySelector('[data-import-sheet]').hidden`)
	if !focused(`[data-action="add-menu"]`) {
		t.Fatal("closing the import sheet must give focus back to the Add menu button")
	}

	// The narrow library sheet takes focus and gives it back on Escape.
	page.MustEval(`()=>{const win=document.querySelector('.vd-window[data-app-id="infra"]');win.style.width='430px';win.style.height='700px';}`)
	page.MustWait(`()=>document.querySelector('.infra-app').clientWidth<500&&getComputedStyle(document.querySelector('.infra-library')).display==='none'`)
	page.MustEval(`()=>{const toggle=document.querySelector('[data-action="toggle-library"]');toggle.focus();toggle.click();}`)
	page.MustWait(`()=>document.querySelector('.infra-app').dataset.librarySheet==='open'&&document.querySelector('[data-library]').contains(document.activeElement)`)
	page.Keyboard.MustType(input.Escape)
	page.MustWait(`()=>document.querySelector('.infra-app').dataset.librarySheet==='closed'`)
	if !focused(`[data-action="toggle-library"]`) || page.MustEval(`()=>document.querySelector('[data-action="toggle-library"]').getAttribute('aria-expanded')`).Str() != "false" {
		t.Fatal("Escape must close the library sheet, report it collapsed and give focus back to the library button")
	}
	page.MustEval(`()=>{const win=document.querySelector('.vd-window[data-app-id="infra"]');win.style.width='1500px';win.style.height='970px';}`)
	page.MustWait(`()=>document.querySelector('.infra-app').clientWidth>1000`)

	// Live regions: routine polls stay silent; losing and regaining sync speak once each.
	if page.MustEval(`()=>!!document.querySelector('[data-live]').closest('[aria-live]:not([aria-live="off"]),[role="status"],[role="alert"]')`).Bool() {
		t.Fatal("the sync indicator must not be a live region")
	}
	page.MustEval(`()=>{
		const app=document.querySelector('.infra-app');window.infraLive=[];
		const regionOf=node=>{const el=node&&node.nodeType===1?node:node&&node.parentElement;return el&&el.closest('[aria-live]:not([aria-live="off"]),[role="status"],[role="alert"],[role="log"]');};
		new MutationObserver(records=>{const touched=new Set();for(const record of records){const region=regionOf(record.target);if(region&&app.contains(region)&&!region.closest('[hidden]'))touched.add(region);}for(const region of touched){const text=region.textContent.trim();if(text)window.infraLive.push(text);}}).observe(app,{subtree:true,childList:true,characterData:true,attributes:true,attributeFilter:['hidden']});
	}`)
	page.MustEval(`()=>document.querySelector('[data-node="dev-a"]').click()`)
	page.MustWait(`()=>!document.querySelector('[data-inspector]').hidden`)
	page.MustEval(`()=>document.querySelector('[data-stage]').focus()`) // focus outside the inspector, so polls repaint it
	time.Sleep(300 * time.Millisecond)
	page.MustEval(`()=>{window.infraLive.length=0;}`)
	runInfraPoll(page)
	runInfraPoll(page)
	fixture.mu.Lock()
	for _, node := range fixture.nodes {
		if node["id"] == "dev-a" {
			node["name"] = "Edge router (renamed elsewhere)"
			node["version"] = infraFixtureNumber(node["version"]) + 1
		}
	}
	fixture.mu.Unlock()
	runInfraPoll(page)
	page.MustWait(`()=>document.querySelector('[data-inspector] h2').textContent.includes('renamed elsewhere')`)
	time.Sleep(300 * time.Millisecond)
	if got := page.MustEval(`()=>JSON.stringify(window.infraLive)`).Str(); got != "[]" {
		t.Fatalf("routine polls must not announce anything: %s", got)
	}
	stale, recovered := infraGermanWord(t, "desktop.infra_sync_stale"), infraGermanWord(t, "desktop.infra_sync_recovered")
	flaky.fail.Store(true)
	runInfraPoll(page)
	page.MustWait(`()=>document.querySelector('[data-live]').dataset.sync==='error'`)
	page.MustWait(`(text)=>window.infraLive.includes(text)`, stale)
	time.Sleep(300 * time.Millisecond)
	spoken := page.MustEval(`()=>window.infraLive.length`).Int()
	runInfraPoll(page)
	time.Sleep(400 * time.Millisecond)
	if got := page.MustEval(`()=>window.infraLive.length`).Int(); got != spoken {
		t.Fatalf("a poll that keeps failing must not announce again: %s", page.MustEval(`()=>JSON.stringify(window.infraLive)`).Str())
	}
	flaky.fail.Store(false)
	runInfraPoll(page)
	page.MustWait(`(text)=>window.infraLive.at(-1)===text`, recovered)

	// Motion: reduced motion and Window animations off stop every Infra motion.
	setMotion := func(media string, animations bool) {
		t.Helper()
		if err := (proto.EmulationSetEmulatedMedia{Features: []*proto.EmulationMediaFeature{{Name: "prefers-reduced-motion", Value: media}}}).Call(page); err != nil {
			t.Fatal(err)
		}
		page.MustEval(`(on)=>{auditTest.state.bootstrap.settings['windows.animations']=on?'true':'false';auditTest.applyDesktopSettings();}`, animations)
		page.MustWait(`(on)=>document.body.dataset.animations===(on?'true':'false')`, animations)
	}
	motion := func() string {
		return page.MustEval(`async()=>{
			const scene=document.querySelector('[data-scene]');let animated=false;
			const watch=new MutationObserver(()=>{if(scene.classList.contains('is-animating'))animated=true;});
			watch.observe(scene,{attributes:true,attributeFilter:['class']});
			document.querySelector('[data-node="dev-b"]').click();
			document.querySelector('[data-action="zoom-in"]').click();
			document.querySelector('[data-action="fit"]').click();
			await new Promise(resolve=>setTimeout(resolve,120));
			watch.disconnect();
			const any=value=>value.split(',').some(part=>parseFloat(part)>0);
			const moving=['.infra-inspector','.infra-library','.infra-scrim','.infra-card','.infra-scene'].filter(selector=>{const el=document.querySelector(selector);if(!el)return false;const style=getComputedStyle(el);return any(style.transitionDuration)||(style.animationName!=='none'&&any(style.animationDuration));});
			return JSON.stringify({animated,moving});
		}`).Str()
	}
	setMotion("no-preference", true)
	if got := motion(); !strings.Contains(got, `"animated":true`) || !strings.Contains(got, ".infra-card") {
		t.Fatalf("test control: with motion allowed, Fit should animate and cards should transition: %s", got)
	}
	setMotion("reduce", true)
	if got := motion(); got != `{"animated":false,"moving":[]}` {
		t.Fatalf("prefers-reduced-motion must stop every Infra transition and animation: %s", got)
	}
	setMotion("no-preference", false)
	if got := motion(); got != `{"animated":false,"moving":[]}` {
		t.Fatalf("Window animations off must stop every Infra transition and animation: %s", got)
	}
	closeInfraBrowserWindow(t, page)
	requireInfraBrowserClean(t, page)
}

// TestDesktopInfraThemeContrastBrowser checks the review themes at 1500x970:
// text contrast of every key surface, the text floors, no overflow in the
// command bar, library and inspector, a visible focus ring on a card, and a
// readable lane label in Topology.
func TestDesktopInfraThemeContrastBrowser(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	page := infraA11yPage(t, infraA11yFixture())
	defer page.Close()
	page.MustEval(infraContrastProbeJS)
	page.MustEval(`()=>document.querySelector('[data-node="dev-a"]').click()`)
	page.MustWait(`()=>!document.querySelector('[data-inspector]').hidden&&!!document.querySelector('[data-inspector] .infra-facts dt')`)
	atlasPairs := [][]any{
		{"card name", `[data-node="dev-a"] .infra-card-name`, 4.5},
		{"card meta", `[data-node="dev-a"] .infra-card-meta`, 4.5},
		{"region name", `[data-region="room-1"] .infra-region-name`, 4.5},
		{"library row", `.infra-library-row-name`, 4.5},
		{"library meta", `.infra-library-row-meta`, 4.5},
		{"library section", `.infra-library-title`, 4.5},
		{"view switch", `.infra-view-switch button[aria-pressed="false"]`, 4.5},
		{"primary button", `.infra-split .infra-button-primary`, 4.5},
		{"inspector title", `[data-inspector] h2`, 4.5},
		{"inspector tab", `[data-inspector] [role="tab"][aria-selected="false"]`, 4.5},
		{"inspector fact", `[data-inspector] .infra-facts dt`, 4.5},
		{"visible count", `[data-visible-count]`, 4.5},
	}
	floors := [][]any{
		{"card name", `.infra-card-name`, 16}, {"card meta", `.infra-card-meta`, 14}, {"library row", `.infra-library-row-name`, 14},
		{"library section", `.infra-library-title`, 13}, {"view switch", `.infra-view-switch button`, 14}, {"inspector tab", `[data-inspector] [role="tab"]`, 14},
	}
	for _, theme := range infraReviewThemes {
		applyInfraTheme(page, theme)
		page.MustEval(`()=>document.querySelector('[data-view="atlas"]').click()`)
		page.MustWait(`()=>!!document.querySelector('[data-region="room-1"] .infra-region-name')`)
		if got := page.MustEval(`(pairs,floors)=>JSON.stringify({contrast:infraProbe.check(pairs),floors:infraProbe.floors(floors),overflow:['.infra-commandbar','.infra-library','.infra-inspector'].filter(selector=>{const el=document.querySelector(selector);return el&&el.getClientRects().length&&el.scrollWidth>el.clientWidth+1;})})`, atlasPairs, floors).Str(); got != `{"contrast":[],"floors":[],"overflow":[]}` {
			t.Fatalf("%s: %s", theme.name, got)
		}
		page.MustEval(`()=>document.querySelector('[data-stage]').focus()`)
		page.Keyboard.MustType(input.ArrowRight)
		if ring := page.MustEval(`()=>JSON.stringify(infraProbe.focusRing(document.activeElement))`).Str(); !strings.Contains(ring, `"ok":true`) {
			t.Fatalf("%s: the focused card has no visible focus ring: %s", theme.name, ring)
		}
		page.MustEval(`()=>document.querySelector('[data-view="topology"]').click()`)
		page.MustWait(`()=>!!document.querySelector('[data-lane="network"] .infra-lane-label')&&!!document.querySelector('[data-node="dev-a"]')`)
		lanePairs := [][]any{{"lane label", `[data-lane="network"] .infra-lane-label`, 4.5}, {"card in a lane", `[data-node="dev-a"] .infra-card-name`, 4.5}}
		if got := page.MustEval(`(pairs,floors)=>JSON.stringify(infraProbe.check(pairs).concat(infraProbe.floors(floors)))`, lanePairs, [][]any{{"lane label", `.infra-lane-label`, 14}}).Str(); got != "[]" {
			t.Fatalf("%s topology: %s", theme.name, got)
		}
	}
	closeInfraBrowserWindow(t, page)
	requireInfraBrowserClean(t, page)
}
```

- [ ] **Step 2: Run and fix.**
  - Run `gofmt -l ui/desktop_infra_a11y_browser_test.go` (expected: no output) and `go vet ./ui`.
  - Run `AURAGO_RUN_BROWSER_SMOKE=1 go test ./ui -run 'TestDesktopInfra(Accessibility|ThemeContrast)Browser' -count=1 -timeout 20m`.
  - **Contrast or floor failure:** fix it in `desktop-app-infra*.css` with tokens. Small muted text on a panel that misses 4.5:1 uses `color-mix(in srgb, var(--vd-theme-muted) 60%, var(--vd-text))`. Re-run the guard (V-core) after every CSS change.
  - **Missing selector:** align the test to the Plan B/C hook that exists only when the hook was renamed. The meaning of the check must stay the same; record the change.
  - **Focus or live region failure:** fix the module from Task 7. Never weaken an assertion.
  - **Flaky test:** reproduce it with Windows animations on and off.
- [ ] **Step 3: Commit.**
  - Paths: `ui/desktop_infra_a11y_browser_test.go`, plus every CSS or JS file Step 2 fixed.
  - Subject: `test(infra): cover keyboard, focus, live region and motion rules and contrast in the review themes`.

---

## Task 9: i18n review: terminology, parity and unused keys

**Files:**
- Modify: `scripts/test-infra-ui.mjs`
- Modify: `ui/lang/desktop/*.json` (16)

- [ ] **Step 1: Add the terminology and parity check.** In `scripts/test-infra-ui.mjs`, insert this directly after the existing "A locale must not be filled with the English source text" loop:

```js
// ---- terminology (spec §3) and full locale parity ----------------------------------
{
    const german = JSON.parse(fs.readFileSync(path.join(root, 'ui/lang/desktop/de.json'), 'utf8'));
    const RETIRED = { en: /\b(nodes?|relations?|relationships?|candidates?)\b/i, de: /(knoten|relation|beziehung|kandidat|behälter)/i };
    // Keys whose text names a real thing that happens to use a retired word.
    const RETIRED_ALLOW = new Map([
        ['desktop.infra_icon_mesh_node', 'the mesh node device type, not the old asset term'],
    ]);
    const PAIRS = [
        [/\bassets?\b/i, /Asset/, 'asset → Asset'],
        [/\bconnections?\b/i, /Verbindung/, 'connection → Verbindung'],
        [/\bdiscover(y|ies)\b/i, /Fund|Funde/, 'discovery → Fund'],
        [/\bguests?\b/i, /Gast|Gäste/, 'guest → Gast'],
    ];
    // Keys whose German text is right without the paired term (state the reason).
    const PAIR_ALLOW = new Map([]);
    const prefixes = [...source.matchAll(/`(desktop\.infra_[a-z0-9_]*)\$\{/g), ...source.matchAll(/['"](desktop\.infra_[a-z0-9_]*)['"]\s*\+/g)].map(match => match[1]);
    const used = Object.keys(english).filter(key => key.startsWith('desktop.infra_') && (required.has(key) || prefixes.some(prefix => key.startsWith(prefix))));
    const retired = [], mismatched = [];
    for (const key of used) {
        if (!RETIRED_ALLOW.has(key)) {
            if (RETIRED.en.test(english[key] || '')) retired.push(`en ${key}: ${english[key]}`);
            if (RETIRED.de.test(german[key] || '')) retired.push(`de ${key}: ${german[key]}`);
        }
        if (PAIR_ALLOW.has(key)) continue;
        for (const [en, de, rule] of PAIRS) if (en.test(english[key] || '') && !de.test(german[key] || '')) mismatched.push(`${rule} ${key}: ${english[key]} / ${german[key]}`);
    }
    assert.deepEqual(retired, [], `retired UI terms (spec §3):\n${retired.join('\n')}`);
    assert.deepEqual(mismatched, [], `en/de terminology differs (spec §3):\n${mismatched.join('\n')}`);
    const infraKeys = Object.keys(english).filter(key => key.startsWith('desktop.infra_'));
    for (const locale of locales) {
        const dictionary = JSON.parse(fs.readFileSync(path.join(root, `ui/lang/desktop/${locale}.json`), 'utf8'));
        const missing = infraKeys.filter(key => typeof dictionary[key] !== 'string' || !dictionary[key].trim());
        assert.deepEqual(missing, [], `${locale}.json lacks Infra keys present in en.json: ${missing.join(', ')}`);
    }
}
```

- [ ] **Step 2: Find and prune unused keys.**
  1. Write `$SCRATCH/infra-i18n-orphans.mjs`:
     ```js
     // Usage: node infra-i18n-orphans.mjs <repoRoot>
     // Prints desktop.infra_* keys of en.json that no UI or server source references.
     import fs from 'node:fs';
     import path from 'node:path';
     import { execFileSync } from 'node:child_process';
     const [root] = process.argv.slice(2);
     const english = JSON.parse(fs.readFileSync(path.join(root, 'ui/lang/desktop/en.json'), 'utf8'));
     const keys = Object.keys(english).filter(key => key.startsWith('desktop.infra_'));
     const files = execFileSync('git', ['-C', root, 'ls-files', '--', 'ui/js', 'ui/*.html', 'internal', 'cmd'], { encoding: 'utf8' }).split('\n').filter(file => /\.(js|mjs|html|go)$/.test(file) && !file.includes('/bundles/'));
     const text = files.map(file => fs.readFileSync(path.join(root, file), 'utf8')).join('\n');
     const prefixes = [...text.matchAll(/`(desktop\.infra_[a-z0-9_]*)\$\{/g), ...text.matchAll(/['"](desktop\.infra_[a-z0-9_]*)['"]\s*\+/g)].map(match => match[1]);
     const orphans = keys.filter(key => !text.includes(`'${key}'`) && !text.includes(`"${key}"`) && !text.includes(`\`${key}\``) && !prefixes.some(prefix => key.startsWith(prefix)));
     console.log(orphans.join('\n'));
     console.error(`${orphans.length} of ${keys.length} Infra keys look unused`);
     ```
  2. Run it from the worktree root:
     ```bash
     node "$SCRATCH/infra-i18n-orphans.mjs" . > "$SCRATCH/infra-orphans.txt"
     ```
  3. For every listed key, run `git grep -n -F "<key>" -- ':!ui/lang'`. Also grep its prefix up to the last `_`, which catches dynamic families (for example `infra_kind_`).
  4. Keep in `$SCRATCH/infra-orphans.txt` only the keys with no hit at all. When in doubt, keep the key in the locale files.
  5. Write `$SCRATCH/infra-i18n-remove.mjs`:
     ```js
     // Usage: node infra-i18n-remove.mjs <repoRoot> <keys.txt>   (one key per line)
     import fs from 'node:fs';
     import path from 'node:path';
     const [root, listFile] = process.argv.slice(2);
     const keys = fs.readFileSync(listFile, 'utf8').split(/\r?\n/).map(line => line.trim()).filter(Boolean);
     if (keys.some(key => !key.startsWith('desktop.infra_'))) throw new Error('only desktop.infra_* keys may be removed');
     const locales = ['cs', 'da', 'de', 'el', 'en', 'es', 'fr', 'hi', 'it', 'ja', 'nl', 'no', 'pl', 'pt', 'sv', 'zh'];
     const prefixes = keys.map(key => `  ${JSON.stringify(key)}: `);
     for (const locale of locales) {
         const file = path.join(root, 'ui/lang/desktop', `${locale}.json`);
         const text = fs.readFileSync(file, 'utf8');
         const eol = text.includes('\r\n') ? '\r\n' : '\n';
         const before = text.split(eol);
         const lines = before.filter(line => !prefixes.some(prefix => line.startsWith(prefix)));
         let close = lines.length - 1;
         while (close >= 0 && lines[close].trim() !== '}') close -= 1;
         let last = close - 1;
         while (last >= 0 && !lines[last].trim()) last -= 1;
         if (last >= 0) lines[last] = lines[last].replace(/,\s*$/, '');
         const out = lines.join(eol);
         const parsed = JSON.parse(out);
         for (const key of keys) if (key in parsed) throw new Error(`${locale}: ${key} survived`);
         fs.writeFileSync(file, out);
         console.log(`${locale}: -${before.length - lines.length}`);
     }
     ```
  6. Run `node "$SCRATCH/infra-i18n-remove.mjs" . "$SCRATCH/infra-orphans.txt"`. Every locale must report the same count.

- [ ] **Step 3: Update the retired-term texts.**
  1. Run `node scripts/test-infra-ui.mjs` and collect every key it lists.
  2. Write `$SCRATCH/infra-i18n-set.mjs`:
     ```js
     // Usage: node infra-i18n-set.mjs <repoRoot> <values.json>
     // values.json: { "en": { "desktop.infra_x": "new text" }, "de": {...}, … all 16 locales }
     // Replaces the value of existing keys in place; keeps order, indentation and EOL.
     import fs from 'node:fs';
     import path from 'node:path';
     const [root, file] = process.argv.slice(2);
     const batch = JSON.parse(fs.readFileSync(file, 'utf8'));
     const locales = ['cs', 'da', 'de', 'el', 'en', 'es', 'fr', 'hi', 'it', 'ja', 'nl', 'no', 'pl', 'pt', 'sv', 'zh'];
     const keys = Object.keys(batch.en || {});
     if (!keys.length) throw new Error('no English keys');
     for (const locale of locales) {
         const entries = batch[locale];
         if (!entries) throw new Error(`batch lacks ${locale}`);
         const target = path.join(root, 'ui/lang/desktop', `${locale}.json`);
         const text = fs.readFileSync(target, 'utf8');
         const eol = text.includes('\r\n') ? '\r\n' : '\n';
         const lines = text.split(eol);
         let changed = 0;
         for (const key of keys) {
             const value = entries[key];
             if (typeof value !== 'string' || !value.trim()) throw new Error(`${locale} lacks ${key}`);
             const prefix = `  ${JSON.stringify(key)}: `;
             const index = lines.findIndex(line => line.startsWith(prefix));
             if (index < 0) throw new Error(`${locale}: ${key} not found; add it with infra-i18n-add.mjs`);
             const next = `${prefix}${JSON.stringify(value)}${lines[index].trimEnd().endsWith(',') ? ',' : ''}`;
             if (next !== lines[index]) { lines[index] = next; changed += 1; }
         }
         const out = lines.join(eol);
         JSON.parse(out);
         fs.writeFileSync(target, out);
         console.log(`${locale}: ${changed} updated`);
     }
     ```
  3. Write `$SCRATCH/infra-i18n-terms.json` with every flagged key still in use, in all 16 locales.
     - Use the en/de values below for the keys that appear here.
     - For any other flagged key, write en and de with the spec §3 terms: asset/Asset, place/Ort, connection/Verbindung, discovery/Fund, guest/Gast, source/Quelle.
     - Translate all of them natively for the other 14 locales, using each locale's established word for these concepts (Step 4 prints them).

| key | en | de |
|---|---|---|
| `desktop.infra_new_node` | New asset | Neues Asset |
| `desktop.infra_add_node` | Add asset | Asset hinzufügen |
| `desktop.infra_create_node` | Create asset | Asset erstellen |
| `desktop.infra_node_created` | Asset created | Asset erstellt |
| `desktop.infra_node_removed` | Asset removed from Infra | Asset aus Infra entfernt |
| `desktop.infra_import_new` | Import as new asset | Als neues Asset importieren |
| `desktop.infra_unnamed_candidate` | Unnamed discovery | Unbenannter Fund |
| `desktop.infra_no_candidates` | No new discoveries | Keine neuen Funde |
| `desktop.infra_import_hint` | Review each discovery, then import it as a new asset or link it to one you already manage. | Prüfe jeden Fund und importiere ihn als neues Asset oder verknüpfe ihn mit einem Asset, das du schon verwaltest. |
| `desktop.infra_relations` | Connections | Verbindungen |
| `desktop.infra_add_relation` | Add connection | Verbindung hinzufügen |
| `desktop.infra_edit_relation` | Edit connection | Verbindung bearbeiten |
| `desktop.infra_no_relations` | No connections yet. | Noch keine Verbindungen. |
| `desktop.infra_relation_kind` | Connection type | Verbindungsart |
| `desktop.infra_relation_saved` | Connection saved | Verbindung gespeichert |
| `desktop.infra_remove_relation` | Remove connection | Verbindung entfernen |
| `desktop.infra_remove_relation_confirm` | Remove this connection from Infra? | Diese Verbindung aus Infra entfernen? |
| `desktop.infra_error_invalid_node_id` | The asset identifier is invalid. | Die Asset-ID ist ungültig. |
| `desktop.infra_error_invalid_candidate_id` | Select a valid discovery. | Wähle einen gültigen Fund. |
| `desktop.infra_error_invalid_edge_id` | The connection identifier is invalid. | Die Verbindungs-ID ist ungültig. |
| `desktop.infra_error_source_linked` | This source is already linked to an Infra asset. | Diese Quelle ist bereits mit einem Infra-Asset verknüpft. |

  4. Run `node "$SCRATCH/infra-i18n-set.mjs" . "$SCRATCH/infra-i18n-terms.json"`. Every locale must report the same count.
  5. Resolve each remaining pairing failure:
     - Retranslate it (preferred).
     - If the German is correct without the term, add the key to `PAIR_ALLOW` with a one-line reason. An example is a sentence where "asset" names a source's own resource.

- [ ] **Step 4: Review the terms per locale.**
  1. Write `$SCRATCH/infra-i18n-concepts.mjs`:
     ```js
     // Usage: node infra-i18n-concepts.mjs <repoRoot>
     // Prints, per locale, the Infra texts of each concept so one term can be checked throughout.
     import fs from 'node:fs';
     import path from 'node:path';
     const [root] = process.argv.slice(2);
     const locales = ['cs', 'da', 'de', 'el', 'en', 'es', 'fr', 'hi', 'it', 'ja', 'nl', 'no', 'pl', 'pt', 'sv', 'zh'];
     const concepts = {
         asset: ['desktop.infra_new_asset', 'desktop.infra_link_all', 'desktop.infra_pick_asset', 'desktop.infra_remove_selection_confirm', 'desktop.infra_import_new'],
         place: ['desktop.infra_new_place', 'desktop.infra_place', 'desktop.infra_no_place', 'desktop.infra_move_to_place', 'desktop.infra_pick_place'],
         connection: ['desktop.infra_connection', 'desktop.infra_connection_new', 'desktop.infra_add_connection', 'desktop.infra_tab_connections', 'desktop.infra_remove_connection'],
         discovery: ['desktop.infra_import_discoveries', 'desktop.infra_import_sheet_hint', 'desktop.infra_ignored_count', 'desktop.infra_unnamed_candidate', 'desktop.infra_no_candidates'],
         guest: ['desktop.infra_guest_count', 'desktop.infra_fact_guests', 'desktop.infra_show_guests', 'desktop.infra_cleanup_engine_host'],
         lane: ['desktop.infra_lane_internet', 'desktop.infra_lane_network', 'desktop.infra_lane_compute', 'desktop.infra_lane_clients'],
     };
     for (const locale of locales) {
         const dictionary = JSON.parse(fs.readFileSync(path.join(root, 'ui/lang/desktop', `${locale}.json`), 'utf8'));
         console.log(`\n## ${locale}`);
         for (const [concept, keys] of Object.entries(concepts)) console.log(`${concept}: ${keys.map(key => dictionary[key] ?? '<missing>').join(' | ')}`);
     }
     ```
  2. Run it. For every locale, check that each concept uses one word throughout.
  3. Fix inconsistent locales with `infra-i18n-set.mjs`. That script needs all 16 locales, so pass the unchanged values for the other locales.

- [ ] **Step 5: Verify.** V-core: the terminology check, parity and "no English prose" must all pass.
- [ ] **Step 6: Commit.**
  - Paths: `scripts/test-infra-ui.mjs`, `ui/lang/desktop/*.json`.
  - Subject: `i18n(infra): retire node, relation and candidate wording and keep Infra keys at parity in 16 locales`.

---

## Task 10: Visual QA with a scratch screenshot runner (no commit)

**Files:** `$SCRATCH/infra-visual/infra_visual_scratch_test.go`, `$SCRATCH/infra-visual/overlay.json`. Output goes to `reports/infra-ui-review/plan-d/`, which is git-ignored. Nothing in this task is committed.

- [ ] **Step 1: Write the runner** to `$SCRATCH/infra-visual/infra_visual_scratch_test.go`. It reuses the fixture helpers from Plans B and C and the Task 8 theme helpers:

```go
package ui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/go-rod/rod/lib/input"
)

// Scratch only (Plan D Task 10): compiled into package ui through
// `go test -overlay`; never copy this file into the repository.

const scratchInfraMetricsJS = `()=>{
	const q=selector=>document.querySelector(selector),app=q('.infra-app');
	const shown=el=>!!el&&el.getClientRects().length>0&&getComputedStyle(el).visibility!=='hidden';
	const box=app.getBoundingClientRect(),canvas=q('[data-canvas]'),inspector=q('[data-inspector]'),bar=q('.infra-commandbar');
	const faded=el=>{let o=1;for(let node=el;node&&node.nodeType===1;node=node.parentElement)o*=Number(getComputedStyle(node).opacity);return o<0.99;};
	const texts=[...app.querySelectorAll('*')].filter(el=>shown(el)&&!faded(el)&&[...el.childNodes].some(node=>node.nodeType===3&&node.textContent.trim()));
	const sectionLabel='.infra-library-title,.infra-library-count,.infra-picker-heading,.infra-form-group-changed,.infra-candidate-reason,.infra-sort,.infra-region-count,.infra-legend h3,.infra-shortcuts h3,.infra-detail-kind,.infra-picker-meta,.infra-picker-note,.infra-card-badge';
	const floor=el=>el.matches('.infra-card-name')?16:el.closest('[data-scene]')?(el.matches('.infra-card-meta,.infra-lane-label,.infra-region-name,.infra-guest-chip span,.infra-guest-more')?14:el.matches('.infra-card-badge')?13:0):el.matches(sectionLabel)?13:14;
	const describe=el=>(el.getAttribute('class')||el.tagName.toLowerCase()).split(' ')[0]+' «'+el.textContent.trim().slice(0,32)+'»';
	const small=texts.filter(el=>parseFloat(getComputedStyle(el).fontSize)<floor(el)).slice(0,24).map(el=>describe(el)+' '+getComputedStyle(el).fontSize);
	const lowContrast=texts.filter(el=>!el.closest('[data-scene] svg')).map(el=>[el,infraProbe.contrast(el)]).filter(pair=>pair[1]<4.5).slice(0,24).map(pair=>describe(pair[0])+' '+pair[1]);
	const overflow=['.infra-commandbar','.infra-library','.infra-inspector','[data-import-sheet]','.infra-table-scroll','.infra-tools','[data-dialog]'].filter(selector=>{const el=q(selector);return shown(el)&&el.scrollWidth>el.clientWidth+1;});
	return JSON.stringify({
		app:[Math.round(box.width),Math.round(box.height)],
		canvasShare:shown(canvas)?Math.round(canvas.getBoundingClientRect().height/box.height*100)/100:null,
		inspector:shown(inspector)?getComputedStyle(inspector).position:'closed',
		scrim:getComputedStyle(q('[data-scrim]')).display,
		library:getComputedStyle(q('.infra-library')).display,
		commandbarWraps:bar.scrollHeight>bar.clientHeight+1,
		overflow:overflow,small:small,lowContrast:lowContrast
	});
}`

func scratchInfraVisualFixture() *infraAppBrowserFixture {
	f := newInfraAppBrowserFixture()
	f.failCandidateOnce = ""
	reg := func(key, name string) map[string]any { return infraPanelsSource("registry", "inventory", key, name) }
	place := func(id, kind, name string, x, y, w, h float64, parent string) map[string]any {
		return infraCanvasFixtureNode(id, kind, name, kind, map[string]any{"x": x, "y": y, "w": w, "h": h, "pinned": true}, parent)
	}
	device := func(id, name, icon, ip string, x, y float64, parent string, source map[string]any) map[string]any {
		node := infraPanelsNode(id, "device", name, icon, ip, infraPanelsAt(x, y), source)
		if parent != "" {
			node["parent_id"] = parent
		}
		return node
	}
	f.nodes = []map[string]any{
		place("site", "site", "Home", -240, -120, 1560, 900, ""),
		place("room", "room", "Basement", 600, 0, 600, 420, "site"),
		place("rack", "rack", "Rack 1", 648, 72, 288, 312, "room"),
		device("inet", "Internet", "internet", "", 0, 0, "site", nil),
		device("ont", "Fiber ONT", "modem-ont", "192.168.1.1", 0, 144, "site", reg("ont", "Fiber ONT")),
		device("router", "Edge router", "router", "192.168.6.1", 0, 288, "site", reg("router", "Edge router")),
		device("switch", "Core switch", "managed-switch", "192.168.6.2", 264, 288, "site", reg("switch", "Core switch")),
		device("ap", "Attic access point", "access-point", "192.168.6.3", 264, 456, "site", reg("ap", "Attic access point")),
		device("pve", "Proxmox host with a long descriptive name", "hypervisor", "192.168.6.10", 792, 168, "rack", reg("pve", "Proxmox host")),
		device("nas", "Basement NAS", "nas", "192.168.6.20", 792, 312, "rack", reg("nas", "Basement NAS")),
		device("laptop", "Admin laptop", "laptop", "192.168.6.50", 264, 624, "site", nil),
		device("tv", "Living room TV", "tv", "192.168.6.30", 0, 624, "site", reg("tv", "Living room TV")),
		infraPanelsNode("tv-uuid", "device", "Chromecast-3c1d2e4f5a6b7c8d", "tv", "192.168.6.30", nil, reg("tv-uuid", "Chromecast-3c1d2e4f5a6b7c8d")),
	}
	for i, name := range []string{"plex", "radarr", "sonarr", "grafana", "home-assistant", "mosquitto", "nginx", "postgres", "redis", "uptime-kuma"} {
		guest := infraPanelsNode(fmt.Sprintf("ct-%d", i), "container", name, "container", "", nil, infraPanelsSource("docker", "engine-a", fmt.Sprintf("c%d", i), name))
		guest["host_id"] = "pve"
		guest["power"] = "running"
		if i%4 == 0 {
			guest["power"] = "stopped"
		}
		f.nodes = append(f.nodes, guest)
	}
	f.nodes = append(f.nodes, infraPanelsNode("ct-orphan", "container", "watchtower", "container", "", nil, infraPanelsSource("docker", "engine-b", "w1", "watchtower")))
	edge := func(id, kind, from, to string, extra map[string]any) map[string]any {
		e := map[string]any{"id": id, "version": 1, "kind": kind, "from": from, "to": to}
		for key, value := range extra {
			e[key] = value
		}
		return e
	}
	f.edges = []map[string]any{
		edge("e1", "ethernet", "inet", "ont", nil), edge("e2", "fiber", "ont", "router", map[string]any{"label": "Uplink"}),
		edge("e3", "ethernet", "router", "switch", map[string]any{"from_port": "LAN 1", "to_port": "eth0", "speed": "2.5 Gbps"}),
		edge("e4", "ethernet", "switch", "ap", nil), edge("e5", "ethernet", "switch", "pve", nil), edge("e6", "ethernet", "switch", "nas", nil),
		edge("e7", "wifi", "ap", "laptop", map[string]any{"label": "Home Wi-Fi"}), edge("e8", "ethernet", "switch", "tv", nil),
		edge("e9", "depends_on", "pve", "nas", nil),
	}
	discovery := func(id string, source map[string]any, kind, name, icon string, profile map[string]any) map[string]any {
		profile["interfaces"] = []any{}
		return map[string]any{"id": id, "source": source, "node": map[string]any{"kind": kind, "name": name, "icon": icon, "profile": profile}}
	}
	f.candidates = []map[string]any{
		discovery("cand-printer", reg("dev-printer", "Office printer"), "device", "Office printer", "printer", map[string]any{"ip_address": "192.168.6.40", "vendor": "Brother", "model": "HL-L2350DW"}),
		discovery("cand-switch", infraPanelsSource("fritzbox", "fritz-a", "fritz-switch", "Core switch"), "device", "Core switch", "generic-device", map[string]any{"ip_address": "192.168.6.2", "vendor": "AVM FRITZ!Box"}),
		discovery("cand-speaker", reg("dev-speaker", "Kitchen speaker"), "device", "Kitchen speaker", "generic-device", map[string]any{"role": "mdns-device", "tags": []any{"network-scan"}}),
	}
	f.sources = []map[string]any{{"kind": "registry", "instance": "inventory", "key": "old-nas", "name": "Old NAS", "state": "current", "observed": map[string]any{"name": "Old NAS"}, "ignored": true}}
	return f
}

func TestInfraVisualScratch(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	out := os.Getenv("AURAGO_INFRA_VISUAL_DIR")
	if out == "" {
		t.Skip("AURAGO_INFRA_VISUAL_DIR is not set")
	}
	only := os.Getenv("AURAGO_INFRA_VISUAL_ONLY") // comma list of theme names; empty means all
	if err := os.MkdirAll(out, 0o755); err != nil {
		t.Fatal(err)
	}
	metrics, err := os.Create(filepath.Join(out, "metrics.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	defer metrics.Close()
	page := infraBrowserPageWithFixture(t, scratchInfraVisualFixture())
	defer page.Close()
	page.MustSetViewport(1920, 1200, 1, false)
	page.MustEval(`()=>Object.keys(localStorage).filter(key=>key.startsWith('infra.')).forEach(key=>localStorage.removeItem(key))`)
	openInfraBrowserApp(page)
	page.MustWait(`()=>!!document.querySelector('[data-nodes] [data-node]')`)
	page.MustEval(infraContrastProbeJS)
	shoot := func(theme, size, scene string) {
		time.Sleep(350 * time.Millisecond)
		name := fmt.Sprintf("%s-%s-%s.png", theme, size, scene)
		if err := os.WriteFile(filepath.Join(out, name), page.MustElement(`.vd-window[data-app-id="infra"]`).MustScreenshot(), 0o644); err != nil {
			t.Fatal(err)
		}
		fmt.Fprintf(metrics, "{\"shot\":%q,\"metrics\":%s}\n", name, page.MustEval(scratchInfraMetricsJS).Str())
	}
	reset := `()=>{document.querySelector('[data-action="close-import"]')?.click();if(document.querySelector('.infra-app').dataset.librarySheet==='open')document.querySelector('[data-action="toggle-library"]').click();document.querySelector('[data-inspector]:not([hidden]) [data-action="close-inspector"]')?.click();}`
	sizes := []struct {
		name string
		w, h int
	}{{"1500x970", 1500, 970}, {"1000x700", 1000, 700}, {"430x700", 430, 700}}
	for _, theme := range infraReviewThemes {
		if only != "" && !strings.Contains(","+only+",", ","+theme.name+",") {
			continue
		}
		applyInfraTheme(page, theme)
		for _, size := range sizes {
			page.MustEval(`(w,h)=>{const win=document.querySelector('.vd-window[data-app-id="infra"]');win.classList.remove('maximized','vd-mobile-forced-maximized');win.style.right='auto';win.style.left='120px';win.style.top='60px';win.style.width=w+'px';win.style.height=h+'px';}`, size.w, size.h)
			page.MustWait(`(w)=>Math.abs(document.querySelector('.vd-window[data-app-id="infra"]').getBoundingClientRect().width-w)<2`, size.w)
			for _, view := range []string{"atlas", "topology"} {
				page.MustEval(reset)
				page.MustEval(`(view)=>document.querySelector('[data-view="'+view+'"]').click()`, view)
				page.MustWait(`(view)=>document.querySelector('[data-view="'+view+'"]').getAttribute('aria-pressed')==='true'`, view)
				page.MustEval(`()=>document.querySelector('[data-action="fit"]').click()`)
				shoot(theme.name, size.name, map[string]string{"atlas": "map", "topology": "topology"}[view])
			}
			page.MustEval(reset)
			page.MustEval(`()=>document.querySelector('[data-view="atlas"]').click()`)
			page.MustEval(`()=>{document.querySelector('[data-node="pve"]').click();const badge=document.querySelector('[data-toggle-host-services="pve"]');if(badge&&badge.getAttribute('aria-expanded')==='false')badge.click();}`)
			page.MustWait(`()=>!document.querySelector('[data-inspector]').hidden`)
			shoot(theme.name, size.name, "inspector")
			page.MustEval(reset)
			page.MustEval(`()=>document.querySelector('[data-stage]').focus()`)
			page.Keyboard.MustType(input.ArrowRight)
			shoot(theme.name, size.name, "focus")
			page.MustEval(reset)
			page.MustEval(`()=>document.querySelector('[data-action="import"]').click()`)
			page.MustWait(`()=>!document.querySelector('[data-import-sheet]').hidden&&!!document.querySelector('[data-candidate-row]')`)
			shoot(theme.name, size.name, "import")
			page.MustEval(reset)
			page.MustEval(`()=>document.querySelector('[data-view="table"]').click()`)
			page.MustWait(`()=>document.querySelectorAll('[data-table-body] [data-row]').length>0`)
			shoot(theme.name, size.name, "list")
			if size.w < 760 {
				page.MustEval(`()=>document.querySelector('[data-view="atlas"]').click()`)
				page.MustEval(`()=>document.querySelector('[data-action="toggle-library"]').click()`)
				page.MustWait(`()=>document.querySelector('.infra-app').dataset.librarySheet==='open'`)
				shoot(theme.name, size.name, "library-sheet")
			}
		}
	}
	closeInfraBrowserWindow(t, page)
	requireInfraBrowserClean(t, page)
}
```

- [ ] **Step 2: Write the overlay** to `$SCRATCH/infra-visual/overlay.json`.
  - Replace `<SCRATCH>` with the absolute scratchpad path, using forward slashes.
  ```json
  {"Replace":{"C:/Users/Andi/Documents/repo/AuraGo/.worktrees/infra-redesign-ui/ui/infra_visual_scratch_test.go":"<SCRATCH>/infra-visual/infra_visual_scratch_test.go"}}
  ```

- [ ] **Step 3: Run the runner.** It takes about 8 minutes for all themes. Use `AURAGO_INFRA_VISUAL_ONLY=standard,amiga` for a partial run.
  ```bash
  cd "/c/Users/Andi/Documents/repo/AuraGo/.worktrees/infra-redesign-ui" && AURAGO_RUN_BROWSER_SMOKE=1 AURAGO_INFRA_VISUAL_DIR="$PWD/reports/infra-ui-review/plan-d" go test ./ui -overlay "$SCRATCH/infra-visual/overlay.json" -run '^TestInfraVisualScratch$' -count=1 -timeout 30m
  ```
  Then confirm nothing leaked into the repo with `git status --short ui/`. It must not list `infra_visual_scratch_test.go`.

- [ ] **Step 4: Review the screenshots against this checklist.**
  - **What to read:**
    - Read every `1500x970` PNG with the Read tool (8 themes × 6 scenes).
    - At `1000x700` and `430x700`, read Standard, Fruity dark, Minimal light, Windows XP and Amiga.
    - Also read every shot whose `metrics.jsonl` line has a non-empty `overflow`, `small` or `lowContrast`, `commandbarWraps: true`, or a `canvasShare` below 0.75 at 1500x970.
  - **Checklist** (spec section in brackets):
    1. **Contrast through tokens [§12]:** text is legible on every surface; muted text stays muted but readable; no light-on-light or dark-on-dark patches in either scheme; the `lowContrast` list is empty or each entry is decorative.
    2. **No hard-coded colours [§12]:** nothing keeps a colour from another theme, such as white cards in dark themes, dark panels in Fruity light, or copper anywhere. Status colours read as success, warning and danger in every theme.
    3. **Focus rings [§5.3]:** the `focus` shots show a clearly visible ring on the focused card in every theme, including Amiga's dashed ring.
    4. **Text floors [§12]:** chrome text is at least 14 px, section labels 13 px and card names 16 px (`small` list).
    5. **No overflow [§4]:**
       - the command bar is one line at every width (labels collapse to icons first);
       - library, inspector, import sheet and List do not scroll sideways;
       - long names ellipsize (see `pve`).
    6. **Canvas share [§4, §13]:** at 1500x970 the canvas is at least 75% of the app height.
    7. **Responsive panels [§4]:**
       - 1500: the inspector is docked (`inspector: static`/grid) and there is no scrim.
       - 1000: the inspector is an overlay drawer (`absolute`) with a visible scrim and close button, and the library is docked.
       - 430: library and inspector are sheets opened from the command bar, and every action stays reachable. The `library-sheet` shot shows the sheet over the map.
    8. **Topology [§6]:**
       - lanes are distinct from the canvas without overpowering it;
       - labels are readable;
       - cards sit inside their lanes;
       - guests ride in the `pve` stack;
       - `ct-orphan` and places are absent.
    9. **Map [§5.1]:**
       - regions are visibly nested (site › room › rack) with readable headers;
       - connection lines are visible in dark themes;
       - fiber, Wi-Fi and dependency strokes are distinguishable;
       - the guest shelf is legible.
    10. **Import sheet [§9.2]:** the three groups and their reasons are readable, and the sheet does not cover the drop region at 1500.
    11. **Themes:** Orbit glass panels stay legible; Windows XP and Amiga chrome around the app does not clash with Infra panels.
  - **How to record findings:**
    - Record each finding as one tab-separated line in `reports/infra-ui-review/plan-d/findings.tsv`, with the header line `id	theme	size	scene	png	selector	observed	expected	class`.
    - `class` is one of:
      - `infra-css`: fix in `desktop-app-infra*.css`;
      - `infra-js`: fix in an Infra module;
      - `i18n`: a text problem;
      - `theme-token`: a token in a skin or bridge is wrong for every app, so it goes to the handoff as a follow-up and is not fixed here;
      - `spec-question`: ask the orchestrator.

- [ ] **Step 5: Plan the fix batches.**
  - Group the `infra-css`, `infra-js` and `i18n` findings into batches of at most 8, by file.
  - Add one Task 11 instance per batch to your task list (D-fix-1, D-fix-2, …). Each lists its finding ids.
  - If there are no findings, skip Task 11 and note that in the handoff.

---

## Task 11: Fix batch D-fix-N (repeat once per batch from Task 10)

**Files:** only the Infra files the batch's findings name, plus `reports/infra-ui-review/plan-d/findings.tsv` (git-ignored).

- [ ] **Step 1: Impact check.** Run it for every JS function you will edit, using the impact template. CSS-only batches skip this step; note that in your task notes.
- [ ] **Step 2: Fix each finding.**
  - **CSS:**
    - Use tokens only, under the Task 6 guard rules.
    - Never add `[data-theme…]` selectors.
    - Prefer changing the shared rule over adding a narrower override.
    - A size or overflow fix keeps the floors: 14 px chrome, 13 px section labels, 16 px card names.
  - **JS:** keep every Plan B and C hook and contract. Add no new strings unless the finding needs one. A new string goes through `$SCRATCH/infra-i18n-add.mjs` with all 16 locales; write the script from Task 2 Step 1 again if your scratchpad is new.
  - **Resolution:** mark each finding fixed in `findings.tsv` by appending `fixed in D-fix-N`.
- [ ] **Step 3: Verify.**
  - Run V-core.
  - Run `AURAGO_RUN_BROWSER_SMOKE=1 go test ./ui -run 'TestDesktopInfra(App|ThemeContrast|Accessibility)Browser' -count=1 -timeout 25m`.
  - Re-run the Task 10 runner for the affected themes (`AURAGO_INFRA_VISUAL_ONLY=<names>`) and read the affected scenes again.
- [ ] **Step 4: Commit.**
  - Paths: the fixed Infra files.
  - Subject: `fix(infra): visual review batch N – <short summary>`.
  - The body lists the finding ids.

---

## Task 12: `documentation/infra.md` (docs-agent)

**Files:** `documentation/infra.md`.

- [ ] **Step 1: Check the file and the API section.**
  1. Run `git ls-files -v documentation/infra.md`. It must start with `H `; otherwise run `git update-index --no-assume-unchanged documentation/infra.md`.
  2. Cross-check the API section that Plan A wrote; it stays unless something below is missing.
     ```bash
     cd "/c/Users/Andi/Documents/repo/AuraGo/.worktrees/infra-redesign-ui" && grep -n -e "unplace" -e "import/batch" -e "sources/ignore" -e "source-hosts" -e "edge_exists" -e "include_ignored" -e "host_inferred" documentation/infra.md
     cd "/c/Users/Andi/Documents/repo/AuraGo/.worktrees/infra-redesign-ui" && git grep -n -E '"/(snapshot|candidates|targets|nodes|import|import/batch|sources/ignore|source-hosts|edges|layout)' -- internal/server/desktop_infra*.go
     ```
     Every route in the second output must have a row in the API table. The `PUT /layout` row must mention `parent_id`, `w`/`h` and `unplace`. Add only what is missing, in the existing table style.

- [ ] **Step 2: Update the opening.** In the intro paragraph, replace `Atlas, topology and inventory show the same entries.` with `The Map, Topology and List views show the same assets.`.

- [ ] **Step 3: Rewrite `## Build your map`.** Replace the whole section up to `## Source edits and actions` with the text below. Before writing, verify every bold label against `ui/lang/desktop/en.json` (keys in brackets; do not copy the brackets).

```markdown
## Build your map

Infra never adds an asset by itself. **Import discoveries…** [`desktop.infra_import_discoveries`], in the **Add**
menu and on the empty map, opens a sheet beside the map. It groups what the configured sources see into
**Recommended** [`desktop.infra_import_recommended`], **Possible duplicates** [`desktop.infra_import_duplicates`]
and **Probably not homelab** [`desktop.infra_import_unlikely`], each row with its reason. Import a row, link it to
an existing asset, ignore it, or drag it onto the map to import and place it in one step. Linking is always
explicit; matching names and addresses are suggestions, never identity. Manual device assets create a registry
device with protocol `none`; assign SSH/VNC and credentials deliberately through the existing registry and
credential UI. VMs, containers, services, places and network segments do not create registry devices. A VM or
container can be explicitly linked to an existing registry device to select its Quick Connect target.

Discovered Registry and FRITZ!Box devices with blank/default names or only a hexadecimal identifier, UUID or MAC
address are not offered as discoveries. Give them a recognizable name in their source to make them available. This
filter also hides untouched, unplaced imports with these names. Registry data and saved assets remain stored;
edited, placed, grouped and connected assets remain visible.

Sites, rooms, racks and groups are places. On the map a placed place is a region: dropping an asset into it makes
the place its parent, dragging the asset out clears the parent, and the region grows to contain its assets. VMs and
containers are shown inside their host: a Proxmox guest through its `runs_on` connection, a Docker container
through the device assigned to its engine. **Cleanup** [`desktop.infra_cleanup`] asks for that device when Infra
cannot infer it. The Topology view draws four lanes (Internet, Network, Compute, Clients and IoT) and lays out
assets without a topology position by connection order; dragging a card into another lane keeps it there, and
**Arrange** [`desktop.infra_arrange`] re-runs the layout. The List view is a sortable tree of the same assets.

Draw a connection by dragging a card's handle onto another card. Ethernet, fiber, Wi-Fi, network membership,
hosting, dependencies and power stay distinct; a shared subnet does not imply a cable. Layout changes are saved per
view and can be undone and redone. Refreshing source data preserves positions. Cleanup also suggests duplicate
assets and unreadable names; **Remove and ignore** [`desktop.infra_remove_and_ignore`] deletes an asset and ignores
its sources, so they are not discovered again until they are restored under **Ignored** [`desktop.infra_ignored`].

The details panel separates **Save in Infra** [`desktop.infra_save_in_infra`] (Details tab, sticky save bar,
Ctrl+S) from the source edit on the Sources tab. Local profiles hold hardware details, network interfaces,
CIDR/VLAN, location, management links, notes and custom fields. Blank information remains unknown. Saving local
details never changes an existing registry device or its Quick Connect destination; use the source editor for those
changes. Failed saves and version conflicts keep the draft rather than silently overwriting another editor's
changes. Closing a window with unsaved local, source or layout changes asks whether to discard them or keep editing.

App links are chosen in a searchable picker: Quick Connect selects a registry device without connecting or
replacing the current session, Network Cameras a configured camera, Software Store an installed app and MeshCore a
confirmed conversation. MeshCore conversations are offered only for radio or mesh assets, or once two characters
are typed. Opening these targets does not dispatch power actions.
```

- [ ] **Step 4: Add `## Desktop app architecture`** directly before `## Local assets and development`:

```markdown
## Desktop app architecture

The Infra window is plain JavaScript in `ui/js/desktop/apps/`, loaded in this order by
`ui/js/desktop/core/module-loader.js`. Every file is an IIFE that sets one global, stays below 1,100 lines and
touches no DOM while loading, so `scripts/test-infra-ui.mjs` can run all of them in a Node `vm`.

| File | Global | Responsibility |
| --- | --- | --- |
| `infra-model.js` | `InfraAppModel` | Pure helpers: viewport math, grid snapping, free slots, regions and drop rules, guests, the filter predicate, the scene builder, layout intents and history, topology lanes and auto layout, discovery relevance, cleanup, List sorting |
| `infra-store.js` | `InfraAppStore` | State, API client, polling and visibility, CAS writes, the layout queue with one conflict rebase, undo and redo, ignore, batch import, engine hosts |
| `infra-picker.js` | `InfraAppPicker` | Searchable combobox for app links, places and assets |
| `infra-canvas.js` | `InfraAppCanvas` | Keyed rendering of cards, host stacks, regions, connections, topology lanes and the minimap |
| `infra-gestures.js` | `InfraAppGestures` | Pointer and keyboard input: pan, zoom, drag and drop, marquee, connect, move mode, Arrange, shortcuts |
| `infra-library.js` | `InfraAppLibrary` | Left panel sections and drag sources |
| `infra-cleanup.js` | `InfraAppCleanup` | Cleanup and Ignored sections |
| `infra-inspector-forms.js` | `InfraAppInspectorForms` | Details and Sources forms, drafts, save bar, app link picker |
| `infra-inspector.js` | `InfraAppInspector` | Details panel header, tabs, connection and multi-selection views |
| `infra-table.js` | `InfraAppTable` | List view: virtualized, sortable tree with bulk actions |
| `infra-dialogs.js` | `InfraAppDialogs` | Dialogs, confirmations, the connection popover |
| `infra-import.js` | `InfraAppImport` | Import sheet with relevance groups |
| `infra-menus.js` | `InfraAppMenus` | Context menus, shortcut sheet, legend |
| `infra.js` | `InfraApp` | Composition root: `render(host, windowId, ctx)`, `dispose(windowId)`, command bar, event routing |

The styles are `ui/css/desktop-app-infra.css` (shell, panels, dialogs) and `ui/css/desktop-app-infra-canvas.css`
(map). They use desktop theme tokens only (`--vd-theme-*`, `--vd-*`, `--ds-*`). The guard in
`scripts/test-infra-ui.mjs` rejects:
- literal colours outside `color-mix()`, and named colours;
- theme-id selectors;
- `--vd-font`;
- uppercase and monospace labels;
- motion outside `@media (prefers-reduced-motion: no-preference)` combined with the desktop *Window animations*
  gate.

### Coordinates

- World coordinates are CSS pixels at zoom 1. `[data-scene]` carries the only transform, `translate(x, y) scale(z)`, with z from 0.25 to 2.
- An asset placement `(x, y)` is the centre of its 184 × 60 px card.
- A place placement (site, room, rack, group) `(x, y)` is the top-left corner of its region, and `w`/`h` is its size.
  - Unset size: 360 × 240.
  - The app resizes to at least 200 × 140; the server accepts 120–20,000.
  - The drawn region is that rectangle grown to contain its children plus 48 px.
- Drops snap to a 24 px grid. A drop onto another card moves to the nearest free grid slot.
- The Map (`atlas`) and Topology (`topology`) views have separate placements. Places are drawn only on the map.

### Topology lanes

| Lane | Index | World y | Assets |
| --- | --- | --- | --- |
| Internet | 0 | 0–479 | icons `internet`, `modem-ont`; role words modem, ONT, ISP, WAN, DSL |
| Network | 1 | 480–959 | kind `network`; icons `router`, `firewall`, `managed-switch`, `switch`, `patch-panel`, `access-point`, `mesh-node`, `lan`, `vlan`, `wlan`; role words router, firewall, switch, gateway, mesh, AP |
| Compute | 2 | 960–1439 | icons `rack-server`, `tower-server`, `mini-pc`, `sbc`, `nas`, `hypervisor`, `kubernetes`; role words server, NAS, hypervisor, Proxmox, ESXi, Kubernetes, k8s; any asset that runs guests |
| Clients and IoT | 3 | 1440–1919 | everything else |

- `topologyLane(node)` is the one mapping. It checks the `network` kind, then the icon, then words in the role, then whether the asset runs guests.
- A card the user placed in Topology (saved `pinned: true`) is in the lane its y falls in (`floor(y / 480)`, clamped to 0–3). That is how dragging into another lane overrides the mapping.
- Assets without a topology placement are drawn at computed slots and nothing is saved until a card is dragged, moved with the keyboard or arranged.
  - The slots depend only on the assets and their connections, so every window shows the same layout and moving one card never shifts another.
  - Places, guests, and VMs or containers without a host get no slot.
- The auto layout walks Ethernet, fiber and Wi-Fi connections breadth-first from the Internet lane and numbers the assets, so parents come before children.
  - Each lane shows its assets in that order, in rows of up to 8 cards and up to 4 rows, each row centred.
- Arrange re-runs the layout for the selection (centred on it, around the other cards) or, without a selection, for every card.
  - Dragged cards keep their lane and stay pinned; the others are saved `pinned: false`.
  - Undoing Arrange returns previously auto-laid cards to the automatic layout.

### Layout saves and undo

- **Commands:** every position, parent or size change is a command dispatched to the store.
- **One save per view:** the store keeps one `PUT /layout` in flight per view and merges moves made meanwhile into the next request.
- **Conflicts:**
  - A 409 reloads the snapshot and resends the same intents onto the fresh versions once.
  - A second conflict leaves the cards marked unsaved behind **Positions not saved · Retry**.
  - A conflict clears that view's undo history.
- **Undo depth:** undo and redo hold 50 steps per view. Placing from the library and Arrange are undone with `unplace` layout items, which restore the previous parent.
- **Unknown fields:** node drafts never gain fields the snapshot did not send. Layout items carry `parent_id`, `w`, `h` and `unplace` only when the snapshot has `source_hosts`.

### Browser storage

Each window keeps view preferences, never inventory data, in `localStorage`:
- `infra.layout.v1`: library and details panel width, collapsed library, minimap;
- `infra.viewport.v1.atlas` and `infra.viewport.v1.topology`: centre and zoom;
- `infra.inspector.v1`: selected tab per kind;
- `infra.table.v1`: List sort;
- `infra.dismissed.v1`: dismissed Cleanup suggestions.

### Accessibility

- The map is one tab stop (`role="application"`); arrow keys move between cards. The library tree and the List view are the accessible lists of the same assets.
- Toolbars use one roving tab stop, the details panel tabs follow the tab pattern, and pickers are ARIA 1.2 comboboxes.
- One polite live region announces state changes only: selection, placement, save outcome, and sync lost or back. A poll never announces, and a poll that keeps failing reports the error once.
- Closing a panel, sheet, picker or dialog gives focus back to the control that opened it.
- Motion follows `prefers-reduced-motion` and the desktop *Window animations* setting.
```

- [ ] **Step 5: Update `## Local assets and development`.**
  - Keep the assets paragraph and the AGENTS link.
  - Replace the "Run the focused Infra package…" sentence with:
    ```markdown
    Before changing this flow, run `node scripts/test-infra-ui.mjs` (pure helpers, store, CSS guard, translation
    coverage and terminology), `node scripts/test-infra-assets.mjs`, the focused Infra package and server tests
    (`go test ./internal/infra/... -count=1`, `go test ./internal/server/ -run Infra -count=1`), the UI guards and
    `AURAGO_RUN_BROWSER_SMOKE=1 go test ./ui -run TestDesktopInfra -count=1 -timeout 40m`. The browser suite covers
    the map, gestures, panels, topology, accessibility and contrast in the review themes.
    ```
  - Keep the final sentence about live-provider acceptance.

- [ ] **Step 6: Verify.**
  - `grep -n -i -e "atlas" -e "inventory list" -e "candidate" documentation/infra.md` may only show the API terms (`atlas` as a view id, `/candidates`, `candidate_id`) and the "Persistence and privacy" wording about the inventory database.
  - Check every relative link.
- [ ] **Step 7: Commit.**
  - Paths: `documentation/infra.md`.
  - Subject: `docs(infra): describe the canvas UX, frontend modules, coordinates and topology lanes`.

---

## Task 13: User manuals en + de (docs-agent)

**Files:**
- `documentation/manuals/en/29-infra.md`, `documentation/manuals/de/29-infra.md`
- `documentation/manuals/en/README.md:130`, `documentation/manuals/de/README.md:130`
- `documentation/manuals/en/04-webui.md:246`, `documentation/manuals/de/04-webui.md:246`

**Rules for the docs-agent:**
- **Audience:** end users who administer AuraGo. Write plain, second-person instructions in the existing tone: the German manual uses "du".
- **Keep:** the title line, intro scope sentences, the operator-reference link, the `## Table of Contents` list style, the `---` separators, tables, the "See also" line and the Previous/Next chapter footer.
- **Labels:**
  - Quote every UI label exactly as `ui/lang/desktop/en.json` or `de.json` has it. Look each one up by the key named below, and never invent a label.
  - Call the inspector "details panel" (en) and "Detailbereich" (de); its accessible name is `desktop.infra_details`.
- **Terms (spec §3):**
  - en: asset, place, connection, discovery, guest, source; views Map, Topology, List; panel Library.
  - de: Asset, Ort, Verbindung, Fund (plural Funde), Gast (plural Gäste), Quelle; views Karte, Topologie, Liste; panel Bibliothek.
  - Never use node, relation, candidate, atlas, inventory list, Knoten, Beziehung, Kandidat, Behälter or Atlas for the UI.
- **Facts:** describe only behaviour the code has. When in doubt, check the screenshots in `reports/infra-ui-review/plan-d/` or the browser tests.

- [ ] **Step 1: Rewrite `en/29-infra.md`** with this table of contents and content:
  1. **What You Need and Switching It On**: unchanged.
  2. **First Steps**:
     - Open Infra from the start menu, **System** category.
     - The empty map shows `desktop.infra_empty_title` with its three steps (`desktop.infra_empty_step_sources`, `…_import`, `…_drag`) and the buttons `desktop.infra_import_discoveries` and `desktop.infra_new_asset`.
     - Start with the import.
  3. **The Infra Window**:
     - **Command bar:** view switch (`desktop.infra_view_map`, `desktop.infra_topology`, `desktop.infra_view_list`), search (`desktop.infra_search_jump`, Ctrl+K), filters (`desktop.infra_filters`) with removable chips, the sync dot (`desktop.infra_sync_live|syncing|error|paused`), undo and redo, the **Add** split button (`desktop.infra_new_asset`; menu `desktop.infra_new_place`, `desktop.infra_import_discoveries`) and **⋯** (`desktop.infra_refresh`, `desktop.infra_arrange`, `desktop.infra_shortcuts`, `desktop.infra_legend`).
     - **Library sections:** `desktop.infra_to_place`, `desktop.infra_on_map`, `desktop.infra_attention`, `desktop.infra_cleanup`, `desktop.infra_ignored`.
     - **Map:** the tools (`desktop.infra_tool_select|hand|connect`), zoom, `desktop.infra_zoom_fit` and the minimap (`desktop.infra_minimap`).
     - **Details panel:** opens on selection and closes with Esc or its close button.
     - **Narrow windows:** the library button in the command bar opens the library as a sheet, and the details panel slides over the map.
     - **Layout preferences:** library width, minimap and zoom are remembered in this browser.
  4. **Import Discoveries**:
     - **The sheet** (`desktop.infra_import_title`) opens beside the map. It groups discoveries into `desktop.infra_import_recommended` (pre-selected), `desktop.infra_import_duplicates` and `desktop.infra_import_unlikely`. Each row shows its reason (the `desktop.infra_relevance_*` texts).
     - **Row actions:** `desktop.infra_import_one`, `desktop.infra_ignore`, `desktop.infra_link_existing` / `desktop.infra_import_new`, and `desktop.infra_import_selected` / `desktop.infra_ignore_selected`.
     - **Drag to import:** dragging a discovery onto the map imports and places it, inside a place when dropped into one. VMs and containers are imported into their host.
     - **Keep:** the sources table, the hidden-unreadable-names rule and "Other integrations do not feed Infra".
     - **Manual asset:** `desktop.infra_new_asset` asks for name, type and icon. A manual device creates a registry row with protocol `none`.
  5. **Build the Map**:
     - **Placing:** drag rows from `desktop.infra_to_place` onto the map, or press P on a row. Drop into a place (the region shows `desktop.infra_drop_into`); drag out to clear the place.
     - **Places:** move a region by its header and resize it from its edges. `desktop.infra_new_place` creates one, as does the **+** next to `desktop.infra_on_map`.
     - **Guests:** the badge on a host card opens its guest shelf.
     - **Connections:**
       - Hover a card and drag one of its handles onto another card, or use the Connect tool (C).
       - The popover (`desktop.infra_connection_new`) suggests Ethernet, or Wi-Fi for access points and mesh nodes.
       - The connection types are the `desktop.infra_edge_*` values.
     - **Selection:** Shift-click or Shift-drag selects several assets; drag them together or press M to move them with the arrow keys.
     - **Undo:** undo and redo hold 50 steps per view.
     - **Context menu:** right-click or Shift+F10 opens it: Open, `desktop.infra_move_to_place`, `desktop.infra_connect_to`, Open in app, `desktop.infra_remove_from_map`, `desktop.infra_remove_node`, `desktop.infra_remove_and_ignore`.
     - **Positions not saved:** `desktop.infra_layout_unsaved` with `desktop.infra_retry` means a save failed after one automatic retry.
  6. **Topology and List**:
     - **Lanes:** the four lanes (`desktop.infra_lane_*`) and which assets land where, in plain words (the mapping table from Task 12).
     - **Auto layout:** assets without a topology position are laid out automatically in connection order. Dragging a card into another lane keeps it there.
     - **Arrange:** re-lays the selection or every card. Moved cards keep their lane.
     - **Reset a card:** `desktop.infra_topology_auto_place` in the context menu hands it back to the automatic layout. Guests stay inside their host.
     - **List:** click a column to sort it. Hosts expand to their guests. Ticking rows opens the bulk bar (`desktop.infra_move_to_place`, `desktop.infra_remove_and_ignore`, `desktop.infra_remove_node`, `desktop.infra_clear_selection`).
  7. **Cleanup and Ignore**:
     - **Attention:** `desktop.infra_attention` lists stale, unknown and removed sources and unsaved work.
     - **Cleanup suggestions:**
       - possible duplicates (`desktop.infra_cleanup_remove_duplicate`);
       - Docker guests without a host (`desktop.infra_assign_engine_host`);
       - other guests without a host (`desktop.infra_choose_host`);
       - unreadable names (`desktop.infra_rename`).
     - **Dismissing:** `desktop.infra_dismiss`, then `desktop.infra_cleanup_show_dismissed` to bring suggestions back.
     - **Remove and ignore:** `desktop.infra_remove_and_ignore` deletes the asset and ignores its sources. `desktop.infra_ignored` lists them with `desktop.infra_restore`.
     - **Linked sources:** a source that is still linked cannot be ignored (`desktop.infra_error_source_linked`).
  8. **Edit Assets and Save**:
     - **Header:** icon (`desktop.infra_change_icon`), click the name to rename it, place breadcrumb, state and power, and the primary action (`desktop.infra_open_in_app` or `desktop.infra_edit`).
     - **Tabs:** `desktop.infra_tab_overview|details|sources|connections`.
     - **Details groups:** `desktop.infra_identity`, `desktop.infra_group_network`, `desktop.infra_group_hardware`, `desktop.infra_group_links`, `desktop.infra_notes`, `desktop.infra_custom_fields`. Empty groups start closed.
     - **Save bar:** `desktop.infra_unsaved_changes` with `desktop.infra_discard` and `desktop.infra_save_in_infra` (or Ctrl+S). Changed fields are marked.
     - **App links:** `desktop.infra_link_app` opens a search. MeshCore conversations appear only for radio and mesh assets, or after two typed characters.
     - **Conflicts:** keep the existing paragraph, with `desktop.infra_node_compare` and `desktop.infra_reapply_draft`.
     - **Remove from Infra:** keep the existing paragraph.
  9. **Source Edits**: unchanged content. The `desktop.infra_edit_source` button is on the **Sources** tab, and the form saves with Enter or its save button.
  10. **Power Actions**: unchanged content. The actions are on each source row of the **Sources** tab.
  11. **Keyboard Shortcuts**: this table (Ctrl is ⌘ on a Mac):

| Keys | Action |
|---|---|
| Ctrl+K | Search or jump to an asset |
| Ctrl+Z / Ctrl+Shift+Z (or Ctrl+Y) | Undo / redo |
| Ctrl+S | Save the open details |
| Esc | Cancel or close |
| ? | Show the shortcut list |
| Arrow keys | Move between assets on the map |
| Enter | Open the details |
| Space | Add the asset to the selection or remove it |
| M | Move the selection: arrow keys 24 px, with Shift 8 px; Enter saves, Esc cancels |
| P | Place a library row at the centre of the map |
| Ctrl+A | Select all visible assets |
| Delete | Remove the selection from Infra |
| Shift+F10 or Menu key | Open the context menu |
| F | Fit the map to the window |
| + / - / 0 | Zoom in / out / to 100 % |
| V / H / C | Select, pan or connect tool |
| Shift+drag | Select an area |
| Space+drag, middle mouse button | Pan the map |
| Ctrl+wheel, pinch | Zoom at the pointer |

  12. **Good to Know**:
      - Keep the existing bullets.
      - Add: positions are shared and saved per view, the automatic topology layout is the same for everyone, and layout preferences stay in this browser.
      - Keep the limits table.
  13. **Privacy and Safety**:
      - Keep the existing text.
      - Add: ignored sources stay stored, so they are not discovered again.
      - Add: view preferences (panel widths, zoom, chosen tab, List sort, dismissed suggestions) are stored only in the browser.
  14. **When Something Goes Wrong**:
      - Keep each row whose message still exists in en.json; grep the value to check.
      - Replace "No import candidates found" with `desktop.infra_no_candidates`.
      - Add rows for:
        - `desktop.infra_layout_unsaved` (click Retry; if it keeps failing, Refresh);
        - `desktop.infra_sync_stale` (Infra shows the last state and retries on its own);
        - `desktop.infra_error_edge_exists`;
        - `desktop.infra_write_outcome_unknown`;
        - `desktop.infra_error_source_linked` (unlink or remove the asset instead).

- [ ] **Step 2: Rewrite `de/29-infra.md`** with the same structure and facts, using these German headings:
  1. Was du brauchst und wie du es einschaltest
  2. Erste Schritte
  3. Das Infra-Fenster
  4. Funde importieren
  5. Die Karte aufbauen
  6. Topologie und Liste
  7. Aufräumen und Ignorieren
  8. Assets bearbeiten und speichern
  9. Quellenänderungen
  10. Energieaktionen
  11. Tastenkürzel
  12. Gut zu wissen
  13. Datenschutz und Sicherheit
  14. Wenn etwas nicht klappt

  The table-of-contents anchors keep the umlauts (for example `#assets-bearbeiten-und-speichern`, `#aufräumen-und-ignorieren`). All labels come from `de.json`. This is the shortcut table (Strg is ⌘ on a Mac):

| Tasten | Aktion |
|---|---|
| Strg+K | Asset suchen oder anspringen |
| Strg+Z / Strg+Umschalt+Z (oder Strg+Y) | Rückgängig machen / wiederholen |
| Strg+S | Geöffnete Details speichern |
| Esc | Abbrechen oder schließen |
| ? | Liste der Tastenkürzel anzeigen |
| Pfeiltasten | Zwischen Assets auf der Karte wechseln |
| Eingabe | Details öffnen |
| Leertaste | Asset zur Auswahl hinzufügen oder entfernen |
| M | Auswahl verschieben: Pfeiltasten 24 px, mit Umschalt 8 px; Eingabe speichert, Esc bricht ab |
| P | Bibliothekszeile in der Kartenmitte platzieren |
| Strg+A | Alle sichtbaren Assets auswählen |
| Entf | Auswahl aus Infra entfernen |
| Umschalt+F10 oder Menütaste | Kontextmenü öffnen |
| F | Karte an das Fenster anpassen |
| + / - / 0 | Vergrößern / verkleinern / auf 100 % |
| V / H / C | Werkzeug Auswählen, Verschieben oder Verbinden |
| Umschalt+Ziehen | Einen Bereich auswählen |
| Leertaste+Ziehen, mittlere Maustaste | Karte verschieben |
| Strg+Mausrad, Zwei-Finger-Zoom | Am Mauszeiger zoomen |

- [ ] **Step 3: Update the one-line descriptions.**
  - `en/README.md:130`: "The shared homelab atlas: map, topology and list views with live power readings".
  - `de/README.md:130`: "Der gemeinsame Homelab-Atlas: Karten-, Topologie- und Listenansicht mit Live-Stromwerten".
  - `en/04-webui.md:246` and `de/04-webui.md:246`: in the Infra row, replace "atlas, topology and inventory views" with "map, topology and list views". In German, replace "Atlas-, Topologie- und Inventar-Ansicht" with "Karten-, Topologie- und Listenansicht".

- [ ] **Step 4: Verify.**
  ```bash
  cd "/c/Users/Andi/Documents/repo/AuraGo/.worktrees/infra-redesign-ui" && grep -n -i -w -e "node" -e "nodes" -e "relation" -e "relations" -e "candidate" -e "candidates" -e "inventory list" documentation/manuals/en/29-infra.md
  cd "/c/Users/Andi/Documents/repo/AuraGo/.worktrees/infra-redesign-ui" && grep -n -i -e "knoten" -e "beziehung" -e "kandidat" -e "behälter" documentation/manuals/de/29-infra.md
  ```
  Expected: no output from either command, except "Mesh node" as a device type. Click-check the table-of-contents anchors in a Markdown preview, or compare them to GitHub's slug rules: lowercase, spaces become `-`, punctuation is dropped, umlauts are kept.
- [ ] **Step 5: Commit.**
  - Paths: the six manual files.
  - Subject: `docs(manuals): rewrite the Infra chapter for the map, topology lanes, import sheet and cleanup`.

---

## Task 14: Final verification, contract notes, handoff and landing checklist

**Files:** `ui/js/desktop/apps/AGENTS.md`.

- [ ] **Step 1: Append to the `## Infra` section** of `ui/js/desktop/apps/AGENTS.md`. Check for assume-unchanged as in Task 6 Step 3 first.
  ```markdown
  - Topology (Plan D): four fixed lanes of `InfraAppModel.LANE_H` (480) world px from y = 0 in `topologyLanes()` order,
    drawn first in `[data-scene]` as `[data-lanes]`. `topologyLane(node, {hasGuests})` is the only lane mapping.
    Assets without a topology placement are drawn at `topologyAutoPositions` slots and never saved until dragged,
    moved or arranged; a dragged card is saved `pinned: true` and keeps the lane of its y (`laneAt`), and Arrange
    (`topologyArrange`) preserves `pinned`. Gestures take card origins from `canvas.sceneData()`, never from
    `store.effectivePlacement` (auto cards have none).
  - Styles use tokens only. `scripts/test-infra-ui.mjs` rejects hex, rgb or hsl outside `color-mix()`, named colours,
    `[data-theme…]` selectors, `--vd-font`, and uppercase or monospace labels. Every `transition`/`animation` sits in
    `@media (prefers-reduced-motion: no-preference)` with `body:not([data-animations="false"])`, and JS motion asks
    `app.animationsEnabled()`.
  - Announcements: only `[data-announce]` (via `app.announce`) and state-change regions speak; a poll never changes
    a live region, and a poll that keeps failing reports once. Panels opened from a control give focus back through
    `app.focusReturnTarget`/`app.restoreFocus` (a menu item hands over to its menu trigger).
  ```
  Commit with paths `ui/js/desktop/apps/AGENTS.md` and subject `docs(infra): describe topology lanes, style guard and announcement rules`.

- [ ] **Step 2: Run everything.** Record each result.
  ```bash
  cd "/c/Users/Andi/Documents/repo/AuraGo/.worktrees/infra-redesign-ui" && node scripts/test-infra-ui.mjs && node scripts/test-infra-assets.mjs
  cd "/c/Users/Andi/Documents/repo/AuraGo/.worktrees/infra-redesign-ui" && go test ./ui -run '(TestVirtualDesktopFirstPartyJSFilesStayBelowLineBudget|TestFrontend_JS_NoLiteralTranslationFallbacks|TestDesktopUsedI18nKeysExistInAllLanguages|TestShippedWebAssetsLoadNothingExternal|TestUIAvoidsCookieConsentClassPrefix)' -count=1
  cd "/c/Users/Andi/Documents/repo/AuraGo/.worktrees/infra-redesign-ui" && go test ./internal/infra/... -count=1 && go test ./internal/server/ -run 'Infra' -count=1
  cd "/c/Users/Andi/Documents/repo/AuraGo/.worktrees/infra-redesign-ui" && gofmt -l ui internal/infra internal/server && go vet ./ui
  cd "/c/Users/Andi/Documents/repo/AuraGo/.worktrees/infra-redesign-ui" && npm run check:ui
  cd "/c/Users/Andi/Documents/repo/AuraGo/.worktrees/infra-redesign-ui" && AURAGO_RUN_BROWSER_SMOKE=1 go test ./ui -run 'TestDesktopInfra' -count=1 -timeout 40m
  cd "/c/Users/Andi/Documents/repo/AuraGo/.worktrees/infra-redesign-ui" && wc -l ui/js/desktop/apps/infra*.js
  ```
  - `npm run check:ui` must print `Checked …` for every bundle. This plan changes no bundled file; if the check fails because another session moved a shared source, rebuild with `node scripts/build-ui-bundles.js` and commit the bundle as its own path-limited commit.
  - `gofmt -l` must list nothing this branch touched.
  - Every Infra JS file must be below 1100 lines.
  - If a browser test fails only sometimes, reproduce it with Windows animations on and off, and hand it to debug-agent.

- [ ] **Step 3: Run the GitNexus checks.**
  ```bash
  node C:/Users/Andi/.gitnexus/stores/AuraGo-08496d38b623/checkouts/AuraGo-08496d38b623/run.cjs detect-changes --scope all --repo "C:\\Users\\Andi\\Documents\\repo\\AuraGo"
  node C:/Users/Andi/.gitnexus/stores/AuraGo-08496d38b623/checkouts/AuraGo-08496d38b623/run.cjs detect-changes --scope compare --base-ref "main" --repo "C:\\Users\\Andi\\Documents\\repo\\AuraGo"
  ```
  - `partial: true` or `truncated: true` is not clean; re-run, and if it stays partial, say so in the handoff.
  - Expected scope of the compare against `main`: `internal/infra`, `internal/server/desktop_infra*`, `ui/js/desktop/apps/infra*`, `ui/js/desktop/core/module-loader.js`, `ui/js/desktop/core/menus-and-routing.js`, `ui/js/desktop/bundles/main.bundle.js`, `ui/css/desktop-app-infra*`, `ui/lang/desktop/*`, `ui/desktop_infra*_test.go`, `scripts/test-infra-ui.mjs`, `ui/js/desktop/apps/AGENTS.md`, `internal/infra/AGENTS.md`, `documentation/infra.md`, `documentation/manuals/**`, `documentation/infra-redesign/**`.

- [ ] **Step 4: Hand off for the single combined review** (Critical and Important only). Include:
  - the Task 0 baseline notes;
  - every HIGH or CRITICAL impact warning;
  - the §1 decisions and known limitations;
  - the line counts;
  - the detect-changes summaries;
  - `reports/infra-ui-review/plan-d/` (screenshots, `metrics.jsonl`, `findings.tsv` with each finding's resolution);
  - the `theme-token` follow-ups;
  - the list of pruned i18n keys.

- [ ] **Step 5: Landing checklist.** Describe it in the handoff; do not execute it. Landing needs the orchestrator's or the user's go-ahead after the review.
  1. **Preconditions:**
     - the combined review has no open Critical or Important finding;
     - `feature/infra-redesign` (Plan A) has passed its own review and its security review of the new endpoints;
     - `git -C "/c/Users/Andi/Documents/repo/AuraGo" branch --show-current` and `git worktree list --porcelain` show where `main` is checked out;
     - `git -C <main checkout> status --short -- <the compare paths above>` shows no foreign dirty file. If one is dirty, wait or ask the user.
  2. **Land the backend first.** In `.worktrees/infra-redesign`:
     - `git rebase main`, then `go test ./internal/infra/... -count=1 && go test ./internal/server/ -run Infra -count=1`;
     - then `git -C <main checkout> merge --ff-only feature/infra-redesign`;
     - or, when `main` is checked out nowhere, `git fetch . feature/infra-redesign:main` from the worktree.
  3. **Rebase the UI branch.** In `.worktrees/infra-redesign-ui`:
     - run `git rebase main` (the backend commits already on `main` drop out);
     - resolve any `main.bundle.js` conflict by regenerating it with `node scripts/build-ui-bundles.js`;
     - then run Step 2 again in full, including `npm run check:ui` and V-browser.
  4. **Fast-forward main:** `git -C <main checkout> merge --ff-only feature/infra-redesign-ui`, or `git fetch . feature/infra-redesign-ui:main` when `main` is checked out nowhere. Never use a plumbing write into another session's dirty checkout.
  5. **After landing:**
     - check `git -C <main checkout> log --oneline -3`;
     - do not push; pushing and the aurago-test deploy (binary deploy, user restart, live acceptance with the real 108-asset inventory) are user follow-ups;
     - remove the worktrees and branches only when the user confirms.
  6. **Memory:** the orchestrator updates the project memory with the landed SHA and the open follow-ups.

---

## Spec coverage (self-review)

| Spec item | Task |
|---|---|
| §6 lanes behind cards, labelled, alternating tint via tokens | 3 (canvas, CSS), 5 |
| §6 `topologyLane(node)` from kind, icon, role, guests | 1 |
| §6 drag into another lane stores the placement; lane derived from y | 1 (`laneAt`, `topologyArrange`), 4, 5 |
| §6 auto layout per lane by connection order, centred | 1 (`topologyLayout`, `topologyAutoPositions`), 3, 5 |
| §6 Arrange for the selection or everything | 1 (`topologyArrange`), 4, 5 |
| §6 regions hidden, guests nested | 1 (`buildScene`), 5 |
| §5.3 one tab stop, roving toolbars, tabs, combobox | 7, 8 |
| §5.3 live region only for state changes | 7, 8 |
| §4/§5.3 focus return from inspector and sheets | 7, 8 |
| §12 tokens only, `--vd-font-ui`, no uppercase monospace labels | 6 |
| §12 motion behind reduced motion and `data-animations` | 6, 8 |
| §12 looks native in light and dark themes, contrast | 8, 10, 11 |
| §4 responsive rules, canvas ≥ 75 % at 1500×970 | 10 (checklist), Plan B test |
| §13 visual review at three sizes in the review themes | 10, 11 |
| §13 browser suite updates and new scenarios | 5, 8 |
| §3 terminology, 16-locale translations review | 2, 9 |
| §14 `documentation/infra.md` (modules, coordinate contract, API cross-check) | 12 |
| §14 manuals `documentation/manuals/{de,en}/29-infra.md` | 13 |
| Final verification, GitNexus, landing | 14 |
