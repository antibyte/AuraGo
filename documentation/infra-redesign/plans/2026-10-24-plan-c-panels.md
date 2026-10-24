# Plan C: Infra panels and flows Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Turn the Plan B shell into the finished Infra workspace: full library sections (To place, On the map with place-row drop targets and `+`, Needs attention, Cleanup, Ignored with Restore), a tabbed inspector (Overview/Details/Sources/Connections) with grouped Details form, sticky save bar, Ctrl S, field-level dirty markers, separate source form, a searchable app-links picker with the MeshCore rule, connection and multi-selection inspectors, a non-modal import sheet with relevance groups and drag-to-canvas batch import, ignore and remove-and-ignore flows, Docker engine host assignment, a sortable tree List view with bulk actions, context menus, filter chips, a legend and a shortcut sheet.

**Architecture:** Plan B's nine IIFE modules stay; Plan C adds five (`infra-picker.js`, `infra-cleanup.js`, `infra-inspector-forms.js`, `infra-import.js`, `infra-menus.js`) and extends the model (pure relevance/cleanup/list/inspector helpers, `unplace` intents), the store (ignore, batch import, source hosts, remove, parent updates, undoable placing) and the shell. Every write still goes through `infra-store.js` with CAS, uncertain-outcome reloads and read-only gating; context menus use the shell's `ctx.showContextMenu`.

**Tech Stack:** Vanilla ES2020 IIFEs, CSS container queries with `--vd-theme-*`/`--ds-*` tokens, Node `assert` + `vm` (`scripts/test-infra-ui.mjs`), Go + rod browser tests (`AURAGO_RUN_BROWSER_SMOKE=1`).

---

## 0. Ground rules for every task

**Worktree:** `C:\Users\Andi\Documents\repo\AuraGo\.worktrees\infra-redesign-ui` (Git Bash: `/c/Users/Andi/Documents/repo/AuraGo/.worktrees/infra-redesign-ui`). Every shell command starts with `cd "/c/Users/Andi/Documents/repo/AuraGo/.worktrees/infra-redesign-ui" &&`. Scratch files go in your session scratchpad (`$SCRATCH`), never in the repo.

**V-core:**
```bash
cd "/c/Users/Andi/Documents/repo/AuraGo/.worktrees/infra-redesign-ui" && node scripts/test-infra-ui.mjs
cd "/c/Users/Andi/Documents/repo/AuraGo/.worktrees/infra-redesign-ui" && go test ./ui -run '(TestVirtualDesktopFirstPartyJSFilesStayBelowLineBudget|TestFrontend_JS_NoLiteralTranslationFallbacks|TestDesktopUsedI18nKeysExistInAllLanguages|TestShippedWebAssetsLoadNothingExternal|TestUIAvoidsCookieConsentClassPrefix)' -count=1
```
**V-browser** (~15 min; trim GOCACHE on D: if space runs out):
```bash
cd "/c/Users/Andi/Documents/repo/AuraGo/.worktrees/infra-redesign-ui" && AURAGO_RUN_BROWSER_SMOKE=1 go test ./ui -run 'TestDesktopInfra' -count=1 -timeout 30m
```
**Syntax check** every JS file you created or edited: `node --check ui/js/desktop/apps/<file>.js`.

**Impact step (before editing an existing function):**
```bash
node C:/Users/Andi/.gitnexus/stores/AuraGo-08496d38b623/checkouts/AuraGo-08496d38b623/run.cjs impact "<Symbol>" --direction upstream --repo "C:\\Users\\Andi\\Documents\\repo\\AuraGo"
graft callers <Symbol>
```
The index is stale: `risk: UNKNOWN` is not an all-clear. Confirm with the `git grep` given in the task, record callers and risk in your notes, and warn before editing on HIGH/CRITICAL.

**Commit step template (path-limited):**
```bash
node C:/Users/Andi/.gitnexus/stores/AuraGo-08496d38b623/checkouts/AuraGo-08496d38b623/run.cjs detect-changes --scope all --repo "C:\\Users\\Andi\\Documents\\repo\\AuraGo"
cd "/c/Users/Andi/Documents/repo/AuraGo/.worktrees/infra-redesign-ui" && git add <new files of this task only> && git commit -m "<subject>" -m "<body>" -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>" -- <every path this task touched>
```
`partial: true`/`truncated: true` is not clean; re-run. Never `git add -A`; other sessions commit in this checkout.

**Code conventions (unchanged from Plan B):** `'\x1f'`-style escapes, never `\u` (the Write/Edit tools decode them; this also applies when touching `sourceStamp`, which contains `'\u001f'` today — rewrite those as `'\x1f'`). Every string is a literal `t('desktop.infra_…')`; placeholders via `model.fill(text, vars)`; no `t('k','fallback')`, no `|| 'x'`, no template-literal keys for new keys (existing families like `infra_source_*`, `infra_edge_*`, `infra_action_*`, `infra_app_*`, `infra_field_*` may stay dynamic). No DOM, `localStorage`, `requestAnimationFrame` or `CSS` at load time. Classes `infra-`, never `cc-`. No external URLs; inline SVG without `xmlns`. JS animation only when `app.animationsEnabled()`. Context-menu items use `label: t('desktop.infra_…')` (not `labelKey`) so the key scan sees them. Every file stays < 1100 lines.

**Plan A contract used here:** `GET /snapshot?include_ignored=1` (sources carry `ignored`), `source_hosts [{kind, instance, node_id, version, inferred_node_id}]`, `Node.host_id/host_inferred`, `GET /candidates` (excludes ignored), `POST /import/batch {items:[{candidate_id, node_id?, parent_id?, placement?:{view,x,y}}]} → {nodes, errors:[{candidate_id, error, message}]}`, `PUT /sources/ignore {sources:[{kind,instance,key}], ignored} → {sources}` (409 `source_linked`), `PUT /source-hosts {kind:"docker", instance, node_id, version} → {source_host}`, `DELETE /nodes/{id}?version=N&ignore_sources=1 → {status, id, ignored:[identity]}`, `PUT /layout` items with `parent_id`, `w/h`, and the amendment **`unplace: true`** (deletes `placements[view]`; `x/y` sent as 0).

## 1. Decisions locked in by this plan

1. **Import is a non-modal sheet** (`[data-import-sheet]`, `role=dialog aria-modal=false`) over the library column, so the canvas stays a drop target. Below 760 px it is full width (drag-to-canvas unavailable there; buttons remain).
2. **All four inspector tab panels are rendered; inactive ones are `hidden`.** Draft inputs always exist in the DOM, so `readProfileLists`, validation and the pinned hooks keep working regardless of the tab.
3. **Name is edited inline in the header.** `<h2>` holds a rename button with the name text (tests read `h2.textContent`); the only name input is `.infra-head-name-input[data-node-field="name"][form=…]`, hidden until rename mode. `focusName()` = Details tab + rename mode.
4. **"Needs a host" is replaced by Cleanup**: Docker guests without host form one `engine-host:docker:<instance>` suggestion per engine (Assign engine host → `PUT /source-hosts`); other hostless VMs/containers get `guest-host:<id>` (Choose host… → manual `runs_on` edge, undoable).
5. **"Ignore" on assets is labelled "Remove and ignore"** (linked sources cannot be ignored; it is `DELETE ?ignore_sources=1`). Discoveries get a plain "Ignore".
6. **Dragging a discovery row carries that row only**; Ctrl/⌘ or Shift at pointerdown carries every checked row. Guests (vm/container) dropped on the map are imported without placement/parent.
7. **Placing from the library becomes undoable** through `unplace` intents, and cards get "Remove from map". Gated by `store.features.unplace` (= Plan A present).
8. **Bulk/menus "Move to place…"**: placed assets whose target region is drawn move inside it in one layout save (reparent command); everything else changes only `parent_id` via node PUT (`setParent` command, undoable).
9. **App bindings live in the draft**, not in the DOM; `readProfileLists` no longer reads `[data-app-target]:checked`.
10. **Picker options carry hooks**: `[data-app-target][data-app][data-target-id]`, `[data-node-option]`, `[data-place-option]`; popover root `[data-picker]`.

**Known limitations (state them in the review handoff):** removals, ignores, imports and engine-host changes are not undoable; place rows are drop targets but not draggable; removing a duplicate deletes its connections; an inferred engine host is not offered for confirmation; an `unplace` sent to a server without the amendment answers 400 (lane turns unsaved).

## 2. File structure and public APIs

| File | Status | Global / responsibility |
|---|---|---|
| `ui/js/desktop/apps/infra-model.js` | modify | `InfraAppModel`: + relevance/cleanup (T1), list/inspector/picker helpers, `unplace` in `mergeIntent`/`layoutItems`/`applyLayoutPatch`, `sourceStamp` ignored flag (T2) |
| `ui/js/desktop/apps/infra-store.js` | modify | `InfraAppStore`: include_ignored, `features.unplace`, new commands and writes (T4) |
| `ui/js/desktop/apps/infra-picker.js` | create | `InfraAppPicker` combobox popover/inline (T6) |
| `ui/js/desktop/apps/infra-library.js` | modify | sections, drop rows, attention, Delete, `+` (T7) |
| `ui/js/desktop/apps/infra-cleanup.js` | create | `InfraAppCleanup` Cleanup + Ignored bodies (T7) |
| `ui/js/desktop/apps/infra-gestures.js` | modify | `moveIntoPlace`, `removeFromMap`, `positionsAt`, place-row and discovery drops, keys (T7, T12, T15) |
| `ui/js/desktop/apps/infra-inspector-forms.js` | create | `InfraAppInspectorForms` Details/Sources forms, drafts, save bar, app picker, icon picker, source edit (T9) |
| `ui/js/desktop/apps/infra-inspector.js` | modify | header, tabs, overview, connections, edge/selection inspectors (T10) |
| `ui/js/desktop/apps/infra-dialogs.js` | modify | edge dialog via picker, `deleteNodes`, `removeAndIgnore`, `removeEdge`; import code removed (T10, T12) |
| `ui/js/desktop/apps/infra-import.js` | create | `InfraAppImport` sheet (T12) |
| `ui/js/desktop/apps/infra-canvas.js` | modify | `createGhost(ids, items)` (T12) |
| `ui/js/desktop/apps/infra-table.js` | modify | sortable tree List view + bulk bar (T14) |
| `ui/js/desktop/apps/infra-menus.js` | create | `InfraAppMenus` context menus, ⋯ node menu, shortcuts, legend (T15) |
| `ui/js/desktop/apps/infra.js` | modify | composition, storage helpers, ledger, routing, chips, ⋯ items (T6, T7, T10, T12, T15) |
| `ui/js/desktop/core/menus-and-routing.js` + `ui/js/desktop/bundles/main.bundle.js` | modify | pass `showContextMenu`, `wireContextMenuBoundary` to Infra (T5) |
| `ui/js/desktop/core/module-loader.js` | modify | register each new file when created |
| `ui/css/desktop-app-infra.css` | modify | Plan C rules appended per task |
| `scripts/test-infra-ui.mjs` | modify | file list, pure + store tests |
| `ui/desktop_infra_browser_test.go` | modify | fixture routes, main test updates (T7, T10, T12, T16) |
| `ui/desktop_infra_panels_browser_test.go` | create | two scenario tests (T16) |
| `ui/lang/desktop/*.json` | modify | batches F–I |
| `ui/js/desktop/apps/AGENTS.md` | modify | Infra bullets (T17) |

Final load order (module-loader and test list): `infra-model.js, infra-store.js, infra-picker.js, infra-canvas.js, infra-gestures.js, infra-library.js, infra-cleanup.js, infra-inspector-forms.js, infra-inspector.js, infra-table.js, infra-dialogs.js, infra-import.js, infra-menus.js, infra.js`. Each task inserts its new file at that position in both lists.

**App context additions** (`infra.js`): `app.picker, app.cleanup, app.forms, app.importer, app.menus`, `app.readStorage(key)`, `app.writeStorage(key, value)` (try/catch), `app.openAppLink(link)` = `ctx.openApp?.(link.app, model.appLinkContext(link))`. Creation order: picker, canvas, table, cleanup, library, forms, inspector, dialogs, importer, menus, gestures; `importer.el` appended last to `[data-body]`.

**Module APIs (new/changed):**
- Store: `reloadSnapshot()→bool`, `replaceNodes(list)→normalized[]`, `setSourcesIgnored(identities, ignored)→bool`, `setSourceHost(instance, nodeId)→bool`, `importBatch(items)→{nodes, errors, uncertain}`, `removeNodes(ids, {ignoreSources})→removedIds`, `updateNodeFields(id, patch)→bool`, `updateParents(pairs)→bool`, `identityKey(source)`; `features.unplace`; events `sources`, `candidates`, `source-hosts`; commands `placeFromLibrary(view, after, label, before)`, `unplace(view, before, label)`, `setParent(changes, label)`.
- Picker: `open({anchor, rect, label, placeholder, source, onPick, onClose})`, `close({restoreFocus})`, `isOpen()`, `attachInline(container, {label, placeholder, source, onPick})→{focus, refresh({clear}), destroy}`, `assetSource(accept, preferred)`, `openAssetPicker({anchor, rect, label, accept, preferred, onPick})`, `openPlacePicker({anchor, rect, exclude, onPick})`, `dispose()`. A `source(query)` returns `{groups:[{id, label, options:[{id, label, meta, icon, attrs}], more}], note}`.
- Cleanup: `renderCleanup({section, list, count, reset})`, `renderIgnored({section, list, count})`, `onClick(event)→bool`, `dispose()`.
- Library: + `setDropTarget(id)`.
- Gestures: + `moveIntoPlace(ids, placeId)→Promise<bool>`, `removeFromMap(ids)`, `positionsAt(count, world)`; `startLibraryDrag(event, ids, {mode:'place'|'move'|'discovery', items, onDragged})`.
- Inspector: + `open(id, tab)`, `showTab(tab, {focus, persist})`, `syncHead(draft)`, `focusEdgeForm()`.
- Forms: `detailsHTML(node, draft)`, `sourcesHTML(node)`, `nodeReviewMarkup(id)`, `afterPaint(node)`, `detach()`, `onClick(e)→bool`, `onFieldEvent(e)`, `onSourceFieldEvent(e)`, `onSubmit(e)→bool`, `onToggle(e)`, `submitDetails()`, `openIconPicker(button)`, `onBusy(key)`, `isBusy()`, `dispose()`.
- Dialogs: `openEdgeDialog(edgeId, fromId, toId)`, `deleteNodes(ids)`, `deleteNode(id)` (= `deleteNodes([id])`), `removeAndIgnore(ids)`, `removeEdge(edgeId)`; `openImport` removed (routes go to `importer.open`).
- Importer: `el, open(), close(), render(), reload(), dropOnCanvas(ids, world, region), onBusy(key), dispose()`.
- Menus: `onContextMenu(event)→bool`, `openForElement(el)`, `openNodeMenu(id, button)`, `openShortcuts()`, `legendHTML()`, `dispose()`.

**New pinned hooks:** `[data-library-drop]`, `[data-action=library-add-place|cleanup-reset|close-import|ignore-selected|link-app|rename|pick-icon|edit-details|open-first-app|bulk-move|bulk-ignore|bulk-remove|bulk-clear|clear-filters|shortcuts|legend|move-selection|ignore-selection|save-edge]`, `[data-cleanup][data-cleanup-action][data-cleanup-dismiss][data-cleanup-expand]`, `[data-ignored-source][data-restore-source]`, `[data-inspector-tab][data-inspector-panel]`, `[data-form-group]`, `[data-field][data-dirty]`, `[data-savebar]`, `[data-app-chips][data-app-binding][data-app-picker]`, `[data-picker][data-picker-input][data-picker-note]`, `[data-node-option][data-place-option]`, `[data-icon-picker][data-pick-icon]`, `[data-import-sheet][data-import-group][data-candidate-row][data-candidate-reason][data-import-one][data-ignore-candidate]`, `[data-sort][data-sort-col][data-row-check][data-table-check-all][data-row-expand][data-bulkbar][data-bulk-count]`, `[data-filter-chips][data-remove-filter]`, `[data-popover="legend"]`. All Plan B §1.5 hooks stay.

---

## Task 0: Baseline and preconditions (no commit)

- [ ] **Step 1:** Confirm Plan B is complete: `ls ui/js/desktop/apps/infra-{store,canvas,gestures,library,inspector,table,dialogs}.js ui/desktop_infra_canvas_browser_test.go` exist and `ui/js/desktop/apps/infra-views.js` does not.
- [ ] **Step 2:** Confirm Plan A is merged: `git grep -n -e desktopInfraImportBatch -e desktopInfraIgnoreSources -e desktopInfraSetSourceHost -- internal/server` and `git grep -n 'json:"unplace' -- internal/infra` are non-empty. If `unplace` is missing, stop and report to the orchestrator (Plan C Task 4 then sets `features.unplace = false` permanently).
- [ ] **Step 3:** Run V-core and V-browser; record results and any pre-existing failure (assertion + line).

---

## Task 1: Model — discovery relevance and cleanup (pure)

**Files:** Modify `ui/js/desktop/apps/infra-model.js`, `scripts/test-infra-ui.mjs`. No existing function changes (only additions and the export object); no impact step needed — note that in your report.

- [ ] **Step 1: Add this block** directly before `window.InfraAppModel = {` (after Plan B's helpers):

```js
    // ---- relevance and cleanup (spec §9.2) -------------------------------------
    // Only devices and VMs are compared by address: containers share bridge
    // addresses and places have none.
    const DUPLICATE_KINDS = new Set(['device', 'vm']);
    const GENERIC_ROLES = new Set(['mdns-device', 'dial', 'wfadevice', 'basic']);
    const SCAN_TAGS = new Set(['network-scan', 'full-scan']);
    const SOURCE_RANK = { registry: 0, proxmox: 1, docker: 2, fritzbox: 3 };
    const DEFAULT_NAMES = new Set(['', 'unnamed device', 'unnamed candidate', 'fritz!box host', 'unknown', 'device']);
    const OPAQUE_NAMES = [
        /^[0-9a-f]{8,}$/i,
        /^(uuid:)?[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i,
        /^([0-9a-f]{2}[:-]){5}[0-9a-f]{2}$/i,
        /^\d{1,3}(\.\d{1,3}){3}$/,
        /^[0-9a-f]{0,4}(:[0-9a-f]{0,4}){2,7}$/i,
        /^[a-z][a-z0-9 _-]{0,24}[-_ ][0-9a-f]{12,}$/i,
    ];
    const naturalCompare = (a, b) => String(a ?? '').localeCompare(String(b ?? ''), undefined, { numeric: true, sensitivity: 'base' });
    // isOpaqueName: blank/default names, hex ids, UUIDs, MAC or IP literals and a
    // short word followed by a long hex run (mDNS instance names).
    const isOpaqueName = name => { const value = String(name ?? '').trim(); return DEFAULT_NAMES.has(value.toLowerCase()) || OPAQUE_NAMES.some(pattern => pattern.test(value)); };
    const normalizeIP = value => { const ip = String(value ?? '').trim().toLowerCase(); return !ip || ip === '0.0.0.0' || ip === '::' || ip === '::1' || ip.startsWith('127.') ? '' : ip; };
    const normalizeMAC = value => { const mac = String(value ?? '').trim().toLowerCase().replaceAll('-', ':'); return /^([0-9a-f]{2}:){5}[0-9a-f]{2}$/.test(mac) && mac !== '00:00:00:00:00:00' ? mac : ''; };
    const isLinkLocalIPv6 = ip => /^fe[89ab][0-9a-f]:/i.test(ip);
    const addressesOf = node => {
        const profile = node?.profile || {}, interfaces = Array.isArray(profile.interfaces) ? profile.interfaces : [];
        const ips = new Set([profile.ip_address, ...interfaces.map(item => item?.ip_address)].map(normalizeIP).filter(Boolean));
        const macs = new Set([profile.mac_address, ...interfaces.map(item => item?.mac_address)].map(normalizeMAC).filter(Boolean));
        return { ips: [...ips], macs: [...macs] };
    };
    const isGenericDiscovery = (node, sourceKind) => {
        const profile = node?.profile || {};
        return sourceKind === 'registry' && GENERIC_ROLES.has(String(profile.role || '').trim().toLowerCase())
            && (profile.tags || []).every(tag => SCAN_TAGS.has(String(tag).trim().toLowerCase()))
            && !String(profile.vendor || '').trim() && !String(profile.model || '').trim();
    };
    // A FRITZ!Box names itself as vendor of every host it sees; that says nothing.
    const detailLength = entry => (entry.sourceKind === 'fritzbox' ? 0 : String(entry.node?.profile?.vendor || '').trim().length) + String(entry.node?.profile?.model || '').trim().length;
    // compareSpecificity sorts {id, node, sourceKind} most descriptive first.
    const compareSpecificity = (a, b) => Number(isOpaqueName(a.node?.name)) - Number(isOpaqueName(b.node?.name))
        || detailLength(b) - detailLength(a)
        || (SOURCE_RANK[a.sourceKind] ?? 9) - (SOURCE_RANK[b.sourceKind] ?? 9)
        || Number(isGenericDiscovery(a.node, a.sourceKind)) - Number(isGenericDiscovery(b.node, b.sourceKind))
        || String(a.node?.name || '').length - String(b.node?.name || '').length
        || String(a.id).localeCompare(String(b.id));
    const addressIndex = entries => {
        const byIp = new Map(), byMac = new Map();
        for (const entry of entries) {
            if (!DUPLICATE_KINDS.has(entry.node?.kind)) continue;
            const { ips, macs } = addressesOf(entry.node);
            for (const ip of ips) byIp.set(ip, [...(byIp.get(ip) || []), entry]);
            for (const mac of macs) byMac.set(mac, [...(byMac.get(mac) || []), entry]);
        }
        return { byIp, byMac };
    };
    const candidateEntry = candidate => ({ id: candidate.id, node: candidate.node || {}, sourceKind: candidate.source?.kind || '' });
    const relevanceIndex = (nodes = [], candidates = []) => ({
        nodes: addressIndex((nodes || []).map(node => ({ id: node.id, node }))),
        candidates: addressIndex((candidates || []).filter(Boolean).map(candidateEntry)),
    });
    // discoveryRelevance: an existing asset wins over every discovery; among
    // discoveries sharing an address the most specific one is recommended.
    const discoveryRelevance = (candidate, nodes = [], candidates = [], { sourceHosts = [], index = null } = {}) => {
        const lookup = index || relevanceIndex(nodes, candidates);
        const node = candidate?.node || {}, source = candidate?.source || {};
        const own = addressesOf(node), reasons = [];
        let duplicateOf = null;
        if (DUPLICATE_KINDS.has(node.kind)) {
            for (const [field, match, values] of [['byMac', 'mac', own.macs], ['byIp', 'ip', own.ips]]) {
                const hit = values.map(value => (lookup.nodes[field].get(value) || [])[0]).find(Boolean);
                if (hit) { duplicateOf = { nodeId: hit.id, match }; break; }
            }
            if (!duplicateOf) {
                const peers = new Map();
                for (const [field, match, values] of [['byMac', 'mac', own.macs], ['byIp', 'ip', own.ips]]) {
                    for (const value of values) for (const entry of lookup.candidates[field].get(value) || []) if (entry.id !== candidate.id && !peers.has(entry.id)) peers.set(entry.id, { entry, match });
                }
                if (peers.size) {
                    const best = [candidateEntry(candidate), ...[...peers.values()].map(peer => peer.entry)].sort(compareSpecificity)[0];
                    if (best.id !== candidate.id) duplicateOf = { candidateId: best.id, match: peers.get(best.id).match };
                }
            }
        }
        if (duplicateOf) reasons.push(duplicateOf.match === 'mac' ? 'duplicate_mac' : 'duplicate_ip');
        if (isGenericDiscovery(node, source.kind)) reasons.push('generic_discovery');
        const power = String(source.observed?.power || '');
        const mapped = (sourceHosts || []).some(host => host?.kind === 'docker' && host.instance === source.instance && host.node_id);
        if ((source.kind === 'fritzbox' && power === 'stopped') || (source.kind === 'docker' && power === 'stopped' && !mapped)) reasons.push('inactive');
        if (own.ips.length && own.ips.every(isLinkLocalIPv6)) reasons.push('link_local');
        return { group: duplicateOf ? 'duplicate' : reasons.length ? 'unlikely' : 'recommended', reasons, ...(duplicateOf ? { duplicateOf } : {}) };
    };
    // groupDiscoveries returns the import sheet groups (sorted by name) and every relevance.
    const groupDiscoveries = (candidates = [], nodes = [], { sourceHosts = [] } = {}) => {
        const index = relevanceIndex(nodes, candidates), relevance = new Map(), groups = { recommended: [], duplicate: [], unlikely: [] };
        for (const candidate of candidates || []) {
            if (!candidate?.id) continue;
            const result = discoveryRelevance(candidate, nodes, candidates, { sourceHosts, index });
            relevance.set(candidate.id, result);
            groups[result.group].push(candidate);
        }
        for (const list of Object.values(groups)) list.sort((a, b) => naturalCompare(a.node?.name, b.node?.name) || String(a.id).localeCompare(String(b.id)));
        return { ...groups, relevance };
    };
    const LINKABLE_REGISTRY_KINDS = new Set(['device', 'vm', 'container']);
    // bindableFor: never a place or network; registry devices only onto devices,
    // VMs and containers without another registry device.
    const bindableFor = (candidate, node) => {
        if (!node || isContainer(node.kind) || node.kind === 'network') return false;
        if (candidate?.source?.kind !== 'registry') return true;
        return LINKABLE_REGISTRY_KINDS.has(node.kind) && (!node.registry_id || node.registry_id === candidate.registry_id);
    };
    const normalizedName = value => String(value ?? '').toLocaleLowerCase().replace(/[^a-z0-9]+/g, ' ').trim();
    // linkSuggestions ranks assets a discovery probably is: same MAC, address, name.
    const linkSuggestions = (candidate, nodes = [], limit = 5) => {
        const own = addressesOf(candidate?.node), name = normalizedName(candidate?.node?.name), scored = [];
        for (const node of nodes || []) {
            if (!bindableFor(candidate, node)) continue;
            const theirs = addressesOf(node);
            const score = (own.macs.some(mac => theirs.macs.includes(mac)) ? 4 : 0) + (own.ips.some(ip => theirs.ips.includes(ip)) ? 2 : 0) + (name && normalizedName(node.name) === name ? 1 : 0);
            if (score) scored.push({ id: node.id, name: node.name, score });
        }
        return scored.sort((a, b) => b.score - a.score || naturalCompare(a.name, b.name)).slice(0, limit).map(item => item.id);
    };
    const bestSourceKind = node => (node.sources || []).map(source => source.kind).sort((a, b) => (SOURCE_RANK[a] ?? 9) - (SOURCE_RANK[b] ?? 9))[0] || '';
    // A placed, connected, grouped or manual asset is the one a user invested in.
    const investment = (node, edgeCount) => Object.keys(node.placements || {}).length * 2 + (edgeCount.get(node.id) || 0) + (node.parent_id ? 1 : 0) + ((node.sources || []).length ? 0 : 3);
    const isUntouchedImport = node => (node.sources || []).some(source => [source.name, source.observed?.name].includes(node.name));
    // infraCleanup: duplicates by MAC or address, Docker guests whose engine has
    // no host, other hostless guests, untouched opaque names. `sources` is kept
    // for the spec signature.
    const infraCleanup = (nodes = [], sources = [], sourceHosts = [], { edges = [], dismissed = new Set() } = {}) => {
        const list = (nodes || []).filter(Boolean), edgeCount = new Map();
        for (const edge of edges || []) for (const id of [edge.from, edge.to]) edgeCount.set(id, (edgeCount.get(id) || 0) + 1);
        const rank = (a, b) => investment(b, edgeCount) - investment(a, edgeCount) || compareSpecificity({ id: a.id, node: a, sourceKind: bestSourceKind(a) }, { id: b.id, node: b, sourceKind: bestSourceKind(b) });
        const byName = (a, b) => naturalCompare(a.name, b.name) || String(a.id).localeCompare(String(b.id));
        const duplicates = [], seenGroups = new Set(), inDuplicate = new Set();
        const index = addressIndex(list.map(node => ({ id: node.id, node })));
        for (const [field, match] of [['byMac', 'mac'], ['byIp', 'ip']]) {
            for (const [value, entries] of [...index[field]].sort((a, b) => naturalCompare(a[0], b[0]))) {
                const members = [...new Set(entries.map(entry => entry.id))];
                if (members.length < 2) continue;
                const key = members.slice().sort().join(',');
                if (seenGroups.has(key)) continue;
                seenGroups.add(key);
                const ranked = members.map(id => list.find(node => node.id === id)).sort(rank);
                ranked.forEach(node => inDuplicate.add(node.id));
                duplicates.push({ id: `duplicate:${match}:${value}:${key}`, type: 'duplicate', match, value, keepId: ranked[0].id, removeIds: ranked.slice(1).map(node => node.id), nodeIds: ranked.map(node => node.id) });
            }
        }
        const { hostOf } = guestIndex(list, edges);
        const engines = new Map(), guestHosts = [];
        for (const node of list.filter(item => (item.kind === 'vm' || item.kind === 'container') && !hostOf.has(item.id)).sort(byName)) {
            const docker = (node.sources || []).find(source => source.kind === 'docker' && source.instance);
            if (docker) engines.set(docker.instance, [...(engines.get(docker.instance) || []), node.id]);
            else guestHosts.push({ id: `guest-host:${node.id}`, type: 'guest-host', nodeId: node.id, nodeIds: [node.id] });
        }
        const engineHosts = [...engines].sort((a, b) => naturalCompare(a[0], b[0])).map(([instance, guestIds]) => {
            const entry = (sourceHosts || []).find(host => host?.kind === 'docker' && host.instance === instance) || {};
            return { id: `engine-host:docker:${instance}`, type: 'engine-host', kind: 'docker', instance, version: Number(entry.version) || 0, inferredNodeId: String(entry.inferred_node_id || ''), guestIds, nodeIds: guestIds };
        });
        const opaque = list.filter(node => node.kind === 'device' && !inDuplicate.has(node.id) && isOpaqueName(node.name) && isUntouchedImport(node)
            && (node.sources || []).some(source => source.kind === 'registry' || source.kind === 'fritzbox')).sort(byName)
            .map(node => ({ id: `opaque:${node.id}`, type: 'opaque', nodeId: node.id, nodeIds: [node.id] }));
        return [...duplicates, ...engineHosts, ...guestHosts, ...opaque].filter(item => !dismissed.has(item.id));
    };
```

- [ ] **Step 2: Export** — append to the export object: `naturalCompare, isOpaqueName, addressesOf, isGenericDiscovery, compareSpecificity, relevanceIndex, discoveryRelevance, groupDiscoveries, bindableFor, linkSuggestions, infraCleanup,`.

- [ ] **Step 3: Tests** — insert before the final `console.log` in `scripts/test-infra-ui.mjs`:

```js
// ---- discovery relevance and cleanup -----------------------------------------
{
    const cand = (id, kind, name, profile = {}, source = {}) => ({ id, node: { kind, name, icon: 'generic-device', profile: { tags: [], interfaces: [], ...profile } }, source: { kind: 'registry', instance: 'inventory', key: id, state: 'current', observed: {}, ...source } });
    for (const name of ['3f2a9c1e7b6d', 'b1c2d3e4-f5a6-4b7c-8d9e-0f1a2b3c4d5e', 'AA-BB-CC-DD-EE-FF', '192.168.6.30', 'Chromecast-3c1d2e4f5a6b7c8d', 'Unnamed device']) assert.equal(model.isOpaqueName(name), true, name);
    for (const name of ['Living room TV', 'TheBrain']) assert.equal(model.isOpaqueName(name), false, name);
    const nas = cand('cand-nas', 'device', 'Basement NAS', { ip_address: '192.168.6.20', role: 'server', vendor: 'Synology' });
    const brain = cand('cand-brain', 'device', 'TheBrain', { ip_address: '192.168.6.1', role: 'InternetGatewayDevice' });
    const media = cand('cand-brain-media', 'device', 'TheBrain Mediaserver', { ip_address: '192.168.6.1', role: 'MediaServer' });
    const castNamed = cand('cand-cast', 'device', 'Living room TV', { ip_address: '192.168.6.30', role: 'chromecast' });
    const castUuid = cand('cand-cast-uuid', 'device', 'Chromecast-3c1d2e4f5a6b7c8d', { ip_address: '192.168.6.30', role: 'mdns-device', tags: ['network-scan'] });
    const speaker = cand('cand-speaker', 'device', 'Kitchen speaker', { role: 'mdns-device', tags: ['network-scan', 'full-scan'] });
    const phone = cand('cand-phone', 'device', 'Old phone', { ip_address: '192.168.6.77', vendor: 'AVM FRITZ!Box' }, { kind: 'fritzbox', instance: 'fritz', observed: { power: 'stopped' } });
    const fritzSwitch = cand('cand-switch', 'device', 'Core switch', { ip_address: '192.168.6.2', vendor: 'AVM FRITZ!Box' }, { kind: 'fritzbox', instance: 'fritz', observed: { power: 'running' } });
    const linkLocal = cand('cand-ll', 'device', 'Printer', { ip_address: 'fe80::1c2b:3aff:fe4d:5e6f', role: 'printer' });
    const stopped = cand('cand-ct', 'container', 'legacy-db', {}, { kind: 'docker', instance: 'engine-a', observed: { power: 'stopped' } });
    const nodes = [{ id: 'core-switch', kind: 'device', name: 'Core switch', profile: { ip_address: '192.168.6.2' } }];
    const all = [nas, brain, media, castNamed, castUuid, speaker, phone, fritzSwitch, linkLocal, stopped];
    const rel = candidate => plain(model.discoveryRelevance(candidate, nodes, all));
    assert.deepEqual(rel(nas), { group: 'recommended', reasons: [] });
    assert.deepEqual(rel(brain), { group: 'recommended', reasons: [] }, 'the shorter base name wins an equally detailed pair');
    assert.deepEqual(rel(media), { group: 'duplicate', reasons: ['duplicate_ip'], duplicateOf: { candidateId: 'cand-brain', match: 'ip' } });
    assert.deepEqual(rel(castNamed), { group: 'recommended', reasons: [] });
    assert.deepEqual(rel(castUuid), { group: 'duplicate', reasons: ['duplicate_ip', 'generic_discovery'], duplicateOf: { candidateId: 'cand-cast', match: 'ip' } }, 'the opaque mDNS twin of a Chromecast is the duplicate');
    assert.deepEqual(rel(speaker), { group: 'unlikely', reasons: ['generic_discovery'] });
    assert.deepEqual(rel(phone), { group: 'unlikely', reasons: ['inactive'] });
    assert.deepEqual(rel(fritzSwitch), { group: 'duplicate', reasons: ['duplicate_ip'], duplicateOf: { nodeId: 'core-switch', match: 'ip' } }, 'an existing asset wins over every discovery');
    assert.deepEqual(rel(linkLocal), { group: 'unlikely', reasons: ['link_local'] });
    assert.deepEqual(rel(stopped), { group: 'unlikely', reasons: ['inactive'] });
    assert.deepEqual(plain(model.discoveryRelevance(stopped, nodes, all, { sourceHosts: [{ kind: 'docker', instance: 'engine-a', node_id: 'host-1', version: 1 }] })), { group: 'recommended', reasons: [] }, 'a mapped engine keeps stopped containers relevant');
    const groups = model.groupDiscoveries(all, nodes);
    assert.deepEqual(plain(groups.recommended.map(c => c.id)), ['cand-nas', 'cand-cast', 'cand-brain']);
    assert.deepEqual(plain(groups.duplicate.map(c => c.id)), ['cand-cast-uuid', 'cand-switch', 'cand-brain-media']);
    assert.deepEqual(plain(groups.unlikely.map(c => c.id)), ['cand-speaker', 'cand-ct', 'cand-phone', 'cand-ll']);
    assert.equal(groups.relevance.get('cand-switch').duplicateOf.nodeId, 'core-switch');
    const linkNodes = [
        { id: 'manual-1', kind: 'device', name: 'Atlas gateway', profile: {} },
        { id: 'sw', kind: 'device', name: 'Core switch', profile: { ip_address: '192.168.6.2' } },
        { id: 'sw-reg', kind: 'device', name: 'Switch', registry_id: 'other-device', profile: { ip_address: '192.168.6.2' } },
        { id: 'room', kind: 'room', name: 'Core switch', profile: {} },
    ];
    assert.deepEqual(plain(model.linkSuggestions(fritzSwitch, linkNodes)), ['sw', 'sw-reg'], 'address matches first; places are never link targets');
    const atlas = { ...cand('registry-candidate', 'device', 'Atlas gateway', { ip_address: '192.0.2.9' }), registry_id: 'registry-device-9' };
    assert.deepEqual(plain(model.linkSuggestions(atlas, linkNodes)), ['manual-1'], 'registry discoveries skip assets with another registry device');
    assert.equal(model.bindableFor(atlas, { kind: 'service', name: 'x' }), false);
    const src = (kind, name, extra = {}) => ({ kind, name, observed: { name }, ...extra });
    const cleanupNodes = [
        { id: 'tv', kind: 'device', name: 'Living room TV', placements: { atlas: { x: 0, y: 0 } }, profile: { ip_address: '192.168.6.30' }, sources: [src('registry', 'Living room TV')] },
        { id: 'tv-uuid', kind: 'device', name: 'Chromecast-3c1d2e4f5a6b7c8d', placements: {}, profile: { ip_address: '192.168.6.30' }, sources: [src('registry', 'Chromecast-3c1d2e4f5a6b7c8d')] },
        { id: 'blob', kind: 'device', name: '3f2a9c1e7b6d', placements: {}, profile: {}, sources: [src('fritzbox', '3f2a9c1e7b6d')] },
        { id: 'renamed', kind: 'device', name: 'a1b2c3d4e5f6', placements: {}, profile: {}, sources: [src('registry', 'Garage sensor')] },
        { id: 'ct-1', kind: 'container', name: 'radarr', placements: {}, profile: {}, sources: [{ kind: 'docker', instance: 'engine-a', key: 'c1' }] },
        { id: 'ct-2', kind: 'container', name: 'sonarr', placements: {}, profile: {}, sources: [{ kind: 'docker', instance: 'engine-a', key: 'c2' }] },
        { id: 'ct-hosted', kind: 'container', name: 'plex', host_id: 'host', placements: {}, profile: {}, sources: [{ kind: 'docker', instance: 'engine-a', key: 'c3' }] },
        { id: 'host', kind: 'device', name: 'Docker host', placements: {}, profile: {}, sources: [] },
        { id: 'vm-1', kind: 'vm', name: 'Lab VM', placements: {}, profile: {}, sources: [] },
    ];
    const suggestions = plain(model.infraCleanup(cleanupNodes, [], [{ kind: 'docker', instance: 'engine-a', node_id: '', version: 0, inferred_node_id: '' }], { edges: [] }));
    assert.deepEqual(suggestions.map(item => item.id), ['duplicate:ip:192.168.6.30:tv,tv-uuid', 'engine-host:docker:engine-a', 'guest-host:vm-1', 'opaque:blob'], 'renamed opaque assets and hosted guests make no suggestion');
    assert.deepEqual(suggestions[0], { id: 'duplicate:ip:192.168.6.30:tv,tv-uuid', type: 'duplicate', match: 'ip', value: '192.168.6.30', keepId: 'tv', removeIds: ['tv-uuid'], nodeIds: ['tv', 'tv-uuid'] });
    assert.deepEqual(suggestions[1], { id: 'engine-host:docker:engine-a', type: 'engine-host', kind: 'docker', instance: 'engine-a', version: 0, inferredNodeId: '', guestIds: ['ct-1', 'ct-2'], nodeIds: ['ct-1', 'ct-2'] });
    assert.deepEqual(suggestions[2], { id: 'guest-host:vm-1', type: 'guest-host', nodeId: 'vm-1', nodeIds: ['vm-1'] });
    assert.deepEqual(suggestions[3], { id: 'opaque:blob', type: 'opaque', nodeId: 'blob', nodeIds: ['blob'] });
    const again = plain(model.infraCleanup(cleanupNodes, [], [{ kind: 'docker', instance: 'engine-a', node_id: '', version: 4, inferred_node_id: 'host' }], { edges: [], dismissed: new Set(['opaque:blob']) }));
    assert.deepEqual(again.map(item => item.id), ['duplicate:ip:192.168.6.30:tv,tv-uuid', 'engine-host:docker:engine-a', 'guest-host:vm-1'], 'dismissed suggestions stay hidden');
    assert.equal(again[1].version, 4);
    assert.equal(again[1].inferredNodeId, 'host');
    assert.ok(!plain(model.infraCleanup(cleanupNodes, [], [], { edges: [{ id: 'e', kind: 'runs_on', from: 'vm-1', to: 'host' }] })).some(item => item.id === 'guest-host:vm-1'), 'a runs_on edge gives a guest its host');
}
```

- [ ] **Step 4: Verify:** `node --check ui/js/desktop/apps/infra-model.js`, V-core.
- [ ] **Step 5: Commit** paths `ui/js/desktop/apps/infra-model.js scripts/test-infra-ui.mjs`; subject `feat(infra): add discovery relevance and cleanup helpers`.

---

## Task 2: Model — List, inspector and picker helpers; `unplace` intents

**Files:** `ui/js/desktop/apps/infra-model.js`, `scripts/test-infra-ui.mjs`.

- [ ] **Step 1: Impact** for `mergeIntent`, `layoutItems`, `applyLayoutPatch`, `sourceStamp`; confirm with `git grep -n -e mergeIntent -e layoutItems -e applyLayoutPatch -e sourceStamp -- ui scripts`. Expected: `infra-store.js` (effectiveIntent, queueLayout, moveToUnsaved, sendBatch, applyLayoutResult), `infra-model.js` (snapshotStamp), tests. All changes are backward compatible for intents without `unplace`.

- [ ] **Step 2: Replace `mergeIntent`** with:
```js
    // mergeIntent: unplace wins over a position, a later position cancels an
    // earlier unplace, a parent change survives both.
    const mergeIntent = (earlier, later) => {
        const parent = later && later.parent_id !== undefined ? { parent_id: later.parent_id } : earlier && earlier.parent_id !== undefined ? { parent_id: earlier.parent_id } : {};
        if (later?.unplace) return { unplace: true, ...parent };
        const out = { ...(earlier || {}), ...(later || {}), ...parent };
        if (later && later.x !== undefined) delete out.unplace;
        return out;
    };
```
- [ ] **Step 3: Replace `layoutItems`** with:
```js
    const layoutItems = (intents, nodesById, { parent = false, view = 'atlas', unplace = parent } = {}) => {
        const items = [], missing = [], skipped = [];
        for (const [id, intent] of intents) {
            const node = nodesById.get(id);
            if (!node) { missing.push(id); continue; }
            if (intent?.unplace) {
                // An old server rejects unknown fields, so unplace stays local there.
                if (!unplace) { skipped.push(id); continue; }
                const item = { id, version: node.version, x: 0, y: 0, pinned: false, unplace: true };
                if (parent && intent.parent_id !== undefined) item.parent_id = String(intent.parent_id);
                items.push(item);
                continue;
            }
            const item = { id, version: node.version, x: Math.round(Number(intent.x)), y: Math.round(Number(intent.y)), pinned: intent.pinned !== false };
            if (parent && intent.parent_id !== undefined) item.parent_id = String(intent.parent_id);
            if (parent && isContainer(node.kind)) {
                const stored = node.placements?.[view] || {};
                const w = finite(intent.w) ?? finite(stored.w), h = finite(intent.h) ?? finite(stored.h);
                if (w != null && h != null) { item.w = Math.round(w); item.h = Math.round(h); }
            }
            items.push(item);
        }
        return { items, missing, skipped };
    };
```
- [ ] **Step 4: In `applyLayoutPatch`** replace the placements line and Plan B's parent line with:
```js
            if (row.placements && typeof row.placements === 'object') {
                // A full node states every view; a missing view was unplaced.
                if (row.placements[view]) node.placements[view] = row.placements[view]; else delete node.placements[view];
                node.parent_id = row.parent_id || '';
            } else node.placements[view] = { x: row.x, y: row.y, pinned: !!row.pinned };
```
- [ ] **Step 5: In `sourceStamp`** use `[source?.kind || '', source?.instance || '', source?.key || '', source?.state || '', source?.observed?.power || '', source?.ignored ? 'ignored' : ''].join('\x1f')`.

- [ ] **Step 6: Add this block** after the Task 1 block:
```js
    // ---- List view, inspector and picker helpers -------------------------------
    const TABLE_SORTS = ['name', 'kind', 'place', 'address', 'source', 'status', 'power'];
    const STATUS_ORDER = { removed: 0, stale: 1, unknown: 2, current: 3, manual: 4 };
    const POWER_ORDER = { running: 0, paused: 1, stopped: 2, unknown: 3 };
    const ipv4Number = value => { const match = /^(\d{1,3})\.(\d{1,3})\.(\d{1,3})\.(\d{1,3})$/.exec(String(value ?? '').trim()); return match ? match.slice(1).reduce((sum, part) => sum * 256 + Number(part), 0) : null; };
    const compareAddress = (a, b) => { const x = ipv4Number(a), y = ipv4Number(b); if (x !== null && y !== null) return x - y; if (x !== null) return -1; if (y !== null) return 1; return naturalCompare(a, b); };
    const tableSortValue = (node, key, ctx) => {
        if (key === 'kind') return ctx.labelOf(node.kind);
        if (key === 'place') return ctx.placeOf(node.id);
        if (key === 'address') return node.profile?.ip_address || node.profile?.mac_address || '';
        if (key === 'source') return ctx.sourceOf(node);
        if (key === 'status') return STATUS_ORDER[sourceState(node)];
        if (key === 'power') return Object.prototype.hasOwnProperty.call(POWER_ORDER, node.power || '') ? POWER_ORDER[node.power] : null;
        return node.name;
    };
    // tableComparator: empty values last in both directions; ties by name, then id.
    const tableComparator = (key = 'name', dir = 'asc', ctx = {}) => {
        const sign = dir === 'desc' ? -1 : 1;
        const context = { labelOf: kind => kind, placeOf: () => '', sourceOf: node => (node.sources || []).map(source => source.kind).join(','), ...ctx };
        const empty = value => value === null || value === undefined || value === '';
        return (a, b) => {
            const x = tableSortValue(a, key, context), y = tableSortValue(b, key, context);
            if (empty(x) || empty(y)) { if (empty(x) && empty(y)) return naturalCompare(a.name, b.name) || String(a.id).localeCompare(String(b.id)); return empty(x) ? 1 : -1; }
            const primary = key === 'address' ? compareAddress(x, y) : typeof x === 'number' && typeof y === 'number' ? x - y : naturalCompare(x, y);
            return primary * sign || naturalCompare(a.name, b.name) || String(a.id).localeCompare(String(b.id));
        };
    };
    // tableRows flattens List rows: top-level assets sorted, guests below their
    // host when expanded. While filtering, a non-matching host above a match is
    // a context row and opens automatically.
    const tableRows = (nodes, { hostOf = new Map(), guestsOf = new Map(), compare = () => 0, matches = () => true, expanded = new Set(), filtering = false } = {}) => {
        const byId = new Map((nodes || []).map(node => [node.id, node])), memo = new Map();
        const visible = (id, trail = new Set()) => {
            if (memo.has(id)) return memo.get(id);
            if (trail.has(id)) return false;
            trail.add(id);
            const node = byId.get(id);
            const result = !!node && (matches(node) || (guestsOf.get(id) || []).some(guest => visible(guest, trail)));
            trail.delete(id); memo.set(id, result);
            return result;
        };
        const rows = [], emitted = new Set();
        const emit = (node, depth) => {
            if (emitted.has(node.id)) return;
            emitted.add(node.id);
            const guests = (guestsOf.get(node.id) || []).map(id => byId.get(id)).filter(Boolean);
            const shown = filtering ? guests.filter(guest => visible(guest.id)) : guests;
            const open = guests.length > 0 && (expanded.has(node.id) || (filtering && shown.length > 0));
            rows.push({ id: node.id, depth, guests: guests.length, expanded: open, context: filtering && !matches(node) });
            if (open) for (const guest of shown.slice().sort(compare)) emit(guest, depth + 1);
        };
        for (const node of (nodes || []).filter(item => !byId.has(hostOf.get(item.id))).sort(compare)) if (!filtering || visible(node.id)) emit(node, 0);
        return rows;
    };
    const INSPECTOR_TABS = ['overview', 'details', 'sources', 'connections'];
    const parseJSON = raw => { try { const data = raw ? JSON.parse(raw) : {}; return data && typeof data === 'object' ? data : {}; } catch (_) { return {}; } };
    const readTablePrefs = raw => { const data = parseJSON(raw); return { sort: TABLE_SORTS.includes(data.sort) ? data.sort : 'name', dir: data.dir === 'desc' ? 'desc' : 'asc' }; };
    const readInspectorPrefs = raw => {
        const data = parseJSON(raw), tabs = {};
        if (data.tabs && typeof data.tabs === 'object') for (const [kind, tab] of Object.entries(data.tabs)) if (/^[a-z]{1,16}$/.test(kind) && INSPECTOR_TABS.includes(tab)) tabs[kind] = tab;
        return { tabs };
    };
    const inspectorTab = (prefs, kind) => (INSPECTOR_TABS.includes(prefs?.tabs?.[kind]) ? prefs.tabs[kind] : 'overview');
    const withInspectorTab = (prefs, kind, tab) => ({ tabs: { ...(prefs?.tabs || {}), ...(INSPECTOR_TABS.includes(tab) && /^[a-z]{1,16}$/.test(kind) ? { [kind]: tab } : {}) } });
    const readDismissed = raw => { let list; try { list = JSON.parse(raw || '[]'); } catch (_) { list = []; } return new Set((Array.isArray(list) ? list : []).filter(item => typeof item === 'string').slice(-500)); };
    const blankValue = value => value === undefined || value === null || value === '' || value === 0 || (Array.isArray(value) && !value.length) || (typeof value === 'object' && value !== null && !Array.isArray(value) && !Object.keys(value).length);
    const sameValue = (a, b) => (blankValue(a) && blankValue(b)) || JSON.stringify(a) === JSON.stringify(b);
    // draftDiff lists changed fields: name, kind, icon, parent_id, profile.<key>.
    const draftDiff = (node, draft) => {
        const changed = new Set();
        if (!node || !draft) return changed;
        for (const key of ['name', 'kind', 'icon', 'parent_id']) if (!sameValue(node[key], draft[key])) changed.add(key);
        for (const key of new Set([...Object.keys(node.profile || {}), ...Object.keys(draft.profile || {})])) if (!sameValue(node.profile?.[key], draft.profile?.[key])) changed.add(`profile.${key}`);
        return changed;
    };
    const FORM_GROUPS = {
        identity: ['name', 'kind', 'icon', 'parent_id', 'profile.role', 'profile.tags'],
        network: ['profile.ip_address', 'profile.mac_address', 'profile.cidr', 'profile.vlan', 'profile.interfaces'],
        hardware: ['profile.vendor', 'profile.model', 'profile.serial', 'profile.os', 'profile.firmware', 'profile.cpu', 'profile.ram', 'profile.storage', 'profile.rack_position'],
        links: ['profile.links', 'profile.app_bindings'],
        notes: ['profile.notes'],
        custom: ['profile.custom'],
    };
    const fieldValue = (node, path) => (path.startsWith('profile.') ? node?.profile?.[path.slice(8)] : node?.[path]);
    const formGroupFill = node => Object.fromEntries(Object.entries(FORM_GROUPS).map(([group, fields]) => [group, group === 'identity' || fields.some(path => !blankValue(fieldValue(node, path)))]));
    const groupOfField = path => Object.entries(FORM_GROUPS).find(([, fields]) => fields.includes(path))?.[0] || (path.startsWith('profile.') ? 'custom' : 'identity');
    const MESH_ICONS = new Set(['mesh-node', 'radio-gateway']);
    const APP_ORDER = ['network-cameras', 'software-store', 'meshcore'];
    const isMeshAsset = node => MESH_ICONS.has(node?.icon) || /mesh/i.test(String(node?.profile?.role || '')) || (node?.profile?.tags || []).some(tag => /mesh/i.test(String(tag)));
    const tokensOf = value => new Set(String(value || '').toLocaleLowerCase().split(/[^\p{L}\p{N}]+/u).filter(token => token.length >= 3));
    const targetScore = (node, target) => {
        const label = String(target.label || '').toLocaleLowerCase(), id = String(target.target_id || '').toLocaleLowerCase();
        const ip = String(node?.profile?.ip_address || '').trim().toLocaleLowerCase();
        let score = ip && (label.includes(ip) || id.includes(ip)) ? 3 : 0;
        const theirs = tokensOf(target.label);
        for (const token of tokensOf(node?.name)) if (theirs.has(token)) score += 2;
        return score;
    };
    // appTargetOptions: unbound targets matching the query, suggestions first,
    // then one group per app. MeshCore only for radio/mesh assets or a query of
    // two characters or more.
    const appTargetOptions = (node, targets, query = '', { limit = 50 } = {}) => {
        const q = String(query || '').trim().toLocaleLowerCase();
        const bound = new Set((node?.profile?.app_bindings || []).map(item => `${item.app}\x00${item.target_id}`));
        const meshAllowed = isMeshAsset(node) || q.length >= 2;
        let hiddenMesh = 0;
        const pool = [];
        for (const target of targets || []) {
            if (!target?.app || !target.target_id || bound.has(`${target.app}\x00${target.target_id}`)) continue;
            if (target.app === 'meshcore' && !meshAllowed) { hiddenMesh += 1; continue; }
            const label = String(target.label || target.target_id);
            if (q && !label.toLocaleLowerCase().includes(q) && !String(target.target_id).toLocaleLowerCase().includes(q)) continue;
            pool.push({ app: target.app, target_id: String(target.target_id), label, score: targetScore(node, target) });
        }
        const byLabel = (a, b) => naturalCompare(a.label, b.label) || a.target_id.localeCompare(b.target_id);
        const suggested = pool.filter(option => option.score > 0).sort((a, b) => b.score - a.score || byLabel(a, b)).slice(0, 5);
        const taken = new Set(suggested.map(option => `${option.app}\x00${option.target_id}`));
        const groups = suggested.length ? [{ id: 'suggested', options: suggested }] : [];
        for (const app of APP_ORDER) {
            const list = pool.filter(option => option.app === app && !taken.has(`${option.app}\x00${option.target_id}`)).sort(byLabel);
            if (list.length) groups.push({ id: app, options: list.slice(0, limit), more: Math.max(0, list.length - limit) });
        }
        return { groups, hiddenMesh };
    };
    const appLinkContext = link => ({ 'quick-connect': { device_id: link?.target_id }, 'network-cameras': { camera_id: link?.target_id }, 'software-store': { app_id: link?.target_id }, meshcore: { conversation_id: link?.target_id } }[link?.app] || { target_id: link?.target_id });
    // placePath: place names from the outermost place down to id's parent (or id).
    const placePath = (id, nodeById, { includeSelf = false } = {}) => {
        const path = [], seen = new Set();
        let current = includeSelf ? id : nodeById(id)?.parent_id || '';
        while (current && !seen.has(current)) { seen.add(current); const node = nodeById(current); if (!node) break; path.unshift({ id: node.id, name: node.name }); current = node.parent_id || ''; }
        return path;
    };
    const isDescendant = (id, ancestorId, parentOf) => { const seen = new Set(); for (let current = parentOf(id); current && !seen.has(current); current = parentOf(current)) { if (current === ancestorId) return true; seen.add(current); } return false; };
    const ATTENTION_STATES = new Set(['stale', 'unknown', 'removed']);
    // attentionItems: any linked source that is not current, and unfinished writes.
    const attentionItems = (nodes = [], { unsaved = new Set(), reviews = new Set(), sourceBlocked = new Set(), actionBlocked = new Set() } = {}) => (nodes || []).map(node => {
        const reasons = [];
        for (const source of node.sources || []) if (!source.ignored && ATTENTION_STATES.has(source.state)) reasons.push({ code: `source_${source.state}`, source: source.kind });
        if (unsaved.has(node.id)) reasons.push({ code: 'layout_unsaved' });
        if (reviews.has(node.id)) reasons.push({ code: 'draft_conflict' });
        if (sourceBlocked.has(node.id)) reasons.push({ code: 'source_review' });
        if (actionBlocked.has(node.id)) reasons.push({ code: 'action_review' });
        return { node, reasons };
    }).filter(item => item.reasons.length).sort((a, b) => naturalCompare(a.node.name, b.node.name) || String(a.node.id).localeCompare(String(b.node.id))).map(item => ({ id: item.node.id, reasons: item.reasons }));
```
- [ ] **Step 7: Export** `TABLE_SORTS, tableComparator, tableRows, readTablePrefs, readInspectorPrefs, inspectorTab, withInspectorTab, readDismissed, draftDiff, formGroupFill, groupOfField, isMeshAsset, appTargetOptions, appLinkContext, placePath, isDescendant, attentionItems,`.

- [ ] **Step 8: Tests** (before the final `console.log`):
```js
// ---- List view, inspector and picker helpers -------------------------------------
{
    const rows = [
        { id: 'b', name: 'Beta', kind: 'device', power: 'running', profile: { ip_address: '192.168.6.10' }, sources: [{ kind: 'registry', state: 'current' }] },
        { id: 'a', name: 'alpha', kind: 'vm', power: 'stopped', profile: { ip_address: '192.168.6.9' }, sources: [] },
        { id: 'c', name: 'Gamma 10', kind: 'device', profile: {}, sources: [{ kind: 'docker', state: 'stale' }] },
        { id: 'd', name: 'Gamma 9', kind: 'device', profile: { mac_address: 'aa:bb:cc:dd:ee:ff' }, sources: [{ kind: 'fritzbox', state: 'removed' }] },
    ];
    const ids = (key, dir, ctx) => plain(rows.slice().sort(model.tableComparator(key, dir, ctx)).map(row => row.id));
    assert.deepEqual(ids('name', 'asc'), ['a', 'b', 'd', 'c'], 'names sort naturally, case-insensitive');
    assert.deepEqual(ids('name', 'desc'), ['c', 'd', 'b', 'a']);
    assert.deepEqual(ids('address', 'asc'), ['a', 'b', 'd', 'c'], 'IPv4 numerically, other addresses after, empty last');
    assert.deepEqual(ids('address', 'desc'), ['d', 'b', 'a', 'c'], 'empty stays last in both directions');
    assert.deepEqual(ids('status', 'asc'), ['d', 'c', 'b', 'a'], 'problems first');
    assert.deepEqual(ids('power', 'asc'), ['b', 'a', 'd', 'c']);
    assert.deepEqual(ids('kind', 'asc'), ['b', 'd', 'c', 'a']);
    assert.deepEqual(ids('place', 'asc', { placeOf: id => ({ c: 'Basement', a: 'Attic' })[id] || '' }), ['a', 'c', 'b', 'd']);
    const tree = [{ id: 'host', name: 'Host', kind: 'device' }, { id: 'g2', name: 'web', kind: 'container' }, { id: 'g1', name: 'db', kind: 'container' }, { id: 'solo', name: 'Alone', kind: 'device' }, { id: 'nested', name: 'inner', kind: 'service' }];
    const hostOf = new Map([['g1', 'host'], ['g2', 'host'], ['nested', 'g1']]), guestsOf = new Map([['host', ['g2', 'g1']], ['g1', ['nested']]]);
    const flat = opts => plain(model.tableRows(tree, { hostOf, guestsOf, compare: model.tableComparator('name', 'asc'), ...opts }).map(row => `${row.id}:${row.depth}${row.expanded ? '+' : ''}${row.context ? '~' : ''}`));
    assert.deepEqual(flat({}), ['solo:0', 'host:0'], 'guests stay folded under their host');
    assert.deepEqual(flat({ expanded: new Set(['host']) }), ['solo:0', 'host:0+', 'g1:1', 'g2:1']);
    assert.deepEqual(flat({ expanded: new Set(['host', 'g1']) }), ['solo:0', 'host:0+', 'g1:1+', 'nested:2', 'g2:1']);
    assert.deepEqual(flat({ matches: node => node.id === 'nested', filtering: true }), ['host:0+~', 'g1:1+~', 'nested:2'], 'a matching guest shows its hosts as context rows');
    const targets = [
        { app: 'network-cameras', target_id: 'stream-front-door', label: 'Front door camera' },
        { app: 'software-store', target_id: 'net-tools', label: 'Network Tools' },
        { app: 'meshcore', target_id: 'conv-ipt', label: 'Iptingen Room Server' },
        { app: 'meshcore', target_id: 'conv-lab', label: 'Lab mesh conversation' },
    ];
    const options = (node, query) => { const result = model.appTargetOptions(node, targets, query); return plain({ groups: result.groups.map(group => [group.id, group.options.map(option => option.target_id)]), hiddenMesh: result.hiddenMesh }); };
    const tv = { id: 'tv', name: 'Living room TV', icon: 'tv', profile: { app_bindings: [] } };
    assert.deepEqual(options(tv, ''), { groups: [['network-cameras', ['stream-front-door']], ['software-store', ['net-tools']]], hiddenMesh: 2 }, 'a TV never sees MeshCore by default');
    assert.deepEqual(options(tv, 'ip'), { groups: [['suggested', ['conv-ipt']]], hiddenMesh: 0 }, 'two typed characters search MeshCore too');
    const relay = { id: 'relay', name: 'Garden relay', icon: 'mesh-node', profile: { app_bindings: [{ app: 'meshcore', target_id: 'conv-lab' }] } };
    assert.deepEqual(options(relay, ''), { groups: [['network-cameras', ['stream-front-door']], ['software-store', ['net-tools']], ['meshcore', ['conv-ipt']]], hiddenMesh: 0 }, 'mesh devices see MeshCore; bound targets are not offered again');
    assert.equal(model.appTargetOptions({ name: 'Node', icon: 'router', profile: { tags: ['Mesh backbone'] } }, targets, '').hiddenMesh, 0);
    assert.equal(model.isMeshAsset({ icon: 'radio-gateway', profile: {} }), true);
    assert.equal(model.isMeshAsset({ icon: 'tv', profile: { role: 'chromecast' } }), false);
    assert.equal(model.appTargetOptions({ name: 'Front door', icon: 'camera', profile: {} }, targets, '').groups[0].id, 'suggested');
    const prefs = model.readInspectorPrefs('{"tabs":{"device":"details","vm":"nope","Bad Kind":"sources"}}');
    assert.deepEqual(plain(prefs), { tabs: { device: 'details' } });
    assert.equal(model.inspectorTab(prefs, 'device'), 'details');
    assert.equal(model.inspectorTab(prefs, 'room'), 'overview');
    assert.deepEqual(plain(model.withInspectorTab(prefs, 'room', 'connections')), { tabs: { device: 'details', room: 'connections' } });
    assert.deepEqual(plain(model.readInspectorPrefs('not json')), { tabs: {} });
    assert.deepEqual(plain(model.readTablePrefs('{"sort":"address","dir":"desc"}')), { sort: 'address', dir: 'desc' });
    assert.deepEqual(plain(model.readTablePrefs('{"sort":"secret"}')), { sort: 'name', dir: 'asc' });
    assert.deepEqual([...model.readDismissed('["a",3,"b"]')], ['a', 'b']);
    const stored = { id: 'n', name: 'NAS', kind: 'device', icon: 'nas', parent_id: '', profile: { vendor: '', vlan: 0, interfaces: [], tags: ['lab'], custom: {} } };
    const draft = { ...model.clone(stored), name: 'Big NAS', profile: { ...model.clone(stored.profile), vendor: 'Synology', vlan: '', interfaces: [{ name: 'eth0' }] } };
    assert.deepEqual([...model.draftDiff(stored, draft)].sort(), ['name', 'profile.interfaces', 'profile.vendor'], 'blank values compare equal');
    assert.deepEqual(plain(model.formGroupFill(stored)), { identity: true, network: false, hardware: false, links: false, notes: false, custom: false });
    assert.deepEqual(plain(model.formGroupFill(draft)), { identity: true, network: true, hardware: true, links: false, notes: false, custom: false });
    assert.equal(model.groupOfField('profile.mac_address'), 'network');
    assert.deepEqual(plain(model.attentionItems([
        { id: 'ok', name: 'Fine', sources: [{ kind: 'docker', state: 'current' }] },
        { id: 'stale', name: 'Old', sources: [{ kind: 'docker', state: 'current' }, { kind: 'proxmox', state: 'stale' }] },
        { id: 'moved', name: 'Moved', sources: [] },
    ], { unsaved: new Set(['moved']) })), [{ id: 'moved', reasons: [{ code: 'layout_unsaved' }] }, { id: 'stale', reasons: [{ code: 'source_stale', source: 'proxmox' }] }], 'any non-current source counts');
    const places = { home: { id: 'home', name: 'Home', parent_id: '' }, base: { id: 'base', name: 'Basement', parent_id: 'home' }, rack: { id: 'rack', name: 'Rack 1', parent_id: 'base' } };
    assert.deepEqual(plain(model.placePath('rack', id => places[id]).map(item => item.name)), ['Home', 'Basement']);
    assert.deepEqual(plain(model.placePath('rack', id => places[id], { includeSelf: true }).map(item => item.name)), ['Home', 'Basement', 'Rack 1']);
    assert.equal(model.isDescendant('rack', 'home', id => places[id]?.parent_id || ''), true);
    assert.equal(model.isDescendant('home', 'rack', id => places[id]?.parent_id || ''), false);
    assert.deepEqual(plain(model.appLinkContext({ app: 'meshcore', target_id: 'c1' })), { conversation_id: 'c1' });
    assert.deepEqual(plain(model.mergeIntent({ x: 1, y: 2, parent_id: 'room' }, { unplace: true })), { unplace: true, parent_id: 'room' });
    assert.deepEqual(plain(model.mergeIntent({ unplace: true }, { x: 5, y: 6, pinned: true })), { x: 5, y: 6, pinned: true }, 'a later position cancels unplace');
    const unplaceNodes = new Map([['a', { id: 'a', kind: 'device', version: 2, placements: { atlas: { x: 1, y: 1 } } }]]);
    assert.deepEqual(plain(model.layoutItems(new Map([['a', { unplace: true }]]), unplaceNodes, { parent: true }).items), [{ id: 'a', version: 2, x: 0, y: 0, pinned: false, unplace: true }]);
    assert.deepEqual(plain(model.layoutItems(new Map([['a', { unplace: true }]]), unplaceNodes, { parent: false }).skipped), ['a'], 'an old server never receives unplace');
    const placedNode = [{ id: 'a', version: 2, parent_id: '', placements: { atlas: { x: 1, y: 1 }, topology: { x: 3, y: 3 } } }];
    model.applyLayoutPatch(placedNode, [{ id: 'a', version: 3, placements: { topology: { x: 3, y: 3 } } }], 'atlas');
    assert.equal(placedNode[0].placements.atlas, undefined, 'a full node without the view was unplaced');
    assert.equal(placedNode[0].placements.topology.x, 3);
}
```
- [ ] **Step 9: Verify** `node --check`, V-core (Plan B's model and store tests must still pass).
- [ ] **Step 10: Commit** paths `ui/js/desktop/apps/infra-model.js scripts/test-infra-ui.mjs`; subject `feat(infra): add list, inspector and picker helpers and unplace intents`.

---

## Task 3: i18n batch F (store, picker, library, cleanup)

**Files:** `ui/lang/desktop/*.json` (16).

- [ ] **Step 1:** Ensure `$SCRATCH/infra-i18n-add.mjs` exists; if not, write it exactly as in Plan B Task 2 Step 1 (reads `{locale: {key: text}}` for all 16 locales, refuses missing/empty entries, inserts new keys after the last `"desktop.infra_` line keeping the file's EOL, validates JSON).
- [ ] **Step 2:** Write `$SCRATCH/infra-i18n-f.json` with all 16 locales. Translate the 14 other locales natively, keep placeholders, no English prose (the test rejects identical English text longer than 20 characters with spaces).

| key | en | de |
|---|---|---|
| `desktop.infra_write_outcome_unknown` | Infra could not confirm the change. The latest state was reloaded; check it before you try again. | Infra konnte die Änderung nicht bestätigen. Der aktuelle Stand wurde neu geladen; prüfe ihn, bevor du es erneut versuchst. |
| `desktop.infra_picker_suggested` | Suggestions | Vorschläge |
| `desktop.infra_picker_more` | {count} more · keep typing to narrow the list | {count} weitere · tippe weiter, um die Liste einzugrenzen |
| `desktop.infra_picker_no_match` | Nothing matches | Keine Treffer |
| `desktop.infra_link_all` | All assets | Alle Assets |
| `desktop.infra_pick_asset` | Search assets | Assets suchen |
| `desktop.infra_pick_place` | Search places | Orte suchen |
| `desktop.infra_place` | Place | Ort |
| `desktop.infra_no_place` | No place | Kein Ort |
| `desktop.infra_move_to_place` | Move to place… | An einen Ort verschieben… |
| `desktop.infra_moved_to_place` | {count} assets moved to {place} | {count} Assets nach {place} verschoben |
| `desktop.infra_moved_out_of_place` | {count} assets no longer belong to a place | {count} Assets gehören zu keinem Ort mehr |
| `desktop.infra_cleanup` | Cleanup | Aufräumen |
| `desktop.infra_ignored` | Ignored | Ignoriert |
| `desktop.infra_restore` | Restore | Wiederherstellen |
| `desktop.infra_restore_source` | Restore {name} | {name} wiederherstellen |
| `desktop.infra_restored` | {name} can be discovered again | {name} kann wieder gefunden werden |
| `desktop.infra_dismiss` | Dismiss | Ausblenden |
| `desktop.infra_cleanup_show_dismissed` | Show {count} dismissed suggestions | {count} ausgeblendete Vorschläge anzeigen |
| `desktop.infra_cleanup_duplicate` | {names} share the address {address}. | {names} haben dieselbe Adresse {address}. |
| `desktop.infra_cleanup_duplicate_mac` | {names} share the MAC address {address}. | {names} haben dieselbe MAC-Adresse {address}. |
| `desktop.infra_cleanup_keep` | Keeps {name} | Behält {name} |
| `desktop.infra_cleanup_remove_duplicate` | Remove duplicate and ignore | Duplikat entfernen und ignorieren |
| `desktop.infra_cleanup_remove_confirm` | Remove {names} from Infra and ignore their sources? {keep} stays. Connections of the removed assets are deleted. | {names} aus Infra entfernen und ihre Quellen ignorieren? {keep} bleibt erhalten. Verbindungen der entfernten Assets werden gelöscht. |
| `desktop.infra_cleanup_opaque` | {name} has no readable name. | {name} hat keinen lesbaren Namen. |
| `desktop.infra_rename` | Rename | Umbenennen |
| `desktop.infra_cleanup_engine_host` | {count} Docker guests have no host. Which asset runs this Docker engine? | {count} Docker-Gäste haben keinen Host. Welches Asset betreibt diese Docker-Engine? |
| `desktop.infra_assign_engine_host` | Assign engine host | Engine-Host zuweisen |
| `desktop.infra_engine_host_assigned` | {name} now runs this Docker engine | {name} betreibt jetzt diese Docker-Engine |
| `desktop.infra_cleanup_guest_host` | {name} has no host. | {name} hat keinen Host. |
| `desktop.infra_choose_host` | Choose host… | Host wählen… |
| `desktop.infra_host_chosen` | {name} now runs on {host} | {name} läuft jetzt auf {host} |
| `desktop.infra_cleanup_show_guests` | Show {count} guests | {count} Gäste anzeigen |
| `desktop.infra_cleanup_hide_guests` | Hide guests | Gäste ausblenden |
| `desktop.infra_attention_source_stale` | {source} data is stale | Daten von {source} sind veraltet |
| `desktop.infra_attention_source_unknown` | {source} state is unknown | Zustand von {source} ist unbekannt |
| `desktop.infra_attention_source_removed` | {source} no longer reports it | {source} meldet es nicht mehr |
| `desktop.infra_attention_layout_unsaved` | Position not saved | Position nicht gespeichert |
| `desktop.infra_attention_draft_conflict` | Draft conflicts with a newer version | Entwurf kollidiert mit einer neueren Version |
| `desktop.infra_attention_source_review` | Source change needs a review | Quellenänderung muss geprüft werden |
| `desktop.infra_attention_action_review` | Action outcome needs a review | Ergebnis der Aktion muss geprüft werden |
| `desktop.infra_remove_and_ignore` | Remove and ignore | Entfernen und ignorieren |
| `desktop.infra_removed_ignored` | {count} assets removed and their sources ignored | {count} Assets entfernt und ihre Quellen ignoriert |
| `desktop.infra_removed_many` | {count} assets removed from Infra | {count} Assets aus Infra entfernt |

- [ ] **Step 3:** `node "$SCRATCH/infra-i18n-add.mjs" . "$SCRATCH/infra-i18n-f.json"` → `+44` per locale.
- [ ] **Step 4:** V-core; `git diff --stat` shows 16 files.
- [ ] **Step 5: Commit** paths `ui/lang/desktop/*.json`; subject `i18n(infra): add cleanup, ignore and picker keys`.

---

## Task 4: Store — ignore, batch import, source hosts, removal, parents, undoable placing

**Files:** `ui/js/desktop/apps/infra-store.js`, `scripts/test-infra-ui.mjs`.

- [ ] **Step 1: Impact** for `create` (InfraAppStore), `applySnapshot`, `loadSnapshot`, `refreshSnapshot`, `effectivePlacement`, `placeFromLibrary`; `git grep -n -e "InfraAppStore" -e "placeFromLibrary" -e "effectivePlacement" -- ui scripts`. Expected: `infra.js`, `infra-gestures.js` (commitPlan, planDrop), `infra-canvas.js`, `infra-library.js`, tests. Risk MEDIUM (central state).

- [ ] **Step 2: Edits**
  - `features` init: `{ layoutParent: false, unplace: false }`; in `applySnapshot` after `features.layoutParent = …` add `features.unplace = features.layoutParent;` and compute `const stamp = model.snapshotStamp(state.nodes, state.edges, state.sources, state.caps) + '\x1d' + JSON.stringify(state.sourceHosts);`.
  - `loadSnapshot`: request `` `/snapshot?${explicit ? 'refresh=1&' : ''}include_ignored=1` ``. `refreshSnapshot`: `request('/snapshot?include_ignored=1')`.
  - `effectivePlacement`: after `if (!intent) return base;` add `if (intent.unplace) return null;`.
  - `commands`: replace `placeFromLibrary` and add `unplace`, `setParent`:
```js
        placeFromLibrary: (view, after, label = '', before = new Map()) => {
            const command = layoutCommand('PlaceFromLibrary', view, before, after, label);
            // Undoable only when every placed id has a prior intent (unplace).
            command.undoable = after.size > 0 && [...after.keys()].every(id => !!before.get(id));
            return command;
        },
        unplace: (view, before, label = '') => layoutCommand('Unplace', view, before, new Map([...before.keys()].map(id => [id, { unplace: true }])), label),
        setParent: (changes, label = '') => ({
            type: 'SetParent', label, undoable: true, changes: changes.map(change => ({ id: change.id, before: change.before || '', after: change.after || '' })),
            async apply(store) { return store.updateParents(this.changes.map(change => [change.id, change.after])); },
            async revert(store) { return store.updateParents(this.changes.map(change => [change.id, change.before])); },
        }),
```
  - Add these functions inside `create` (after the edges section) and export them in `api` together with `reloadSnapshot: refreshSnapshot`:
```js
        // ---- sources, discoveries, removal and parents (Plan C) ---------------------
        const identityKey = source => `${source?.kind || ''}\x00${source?.instance || ''}\x00${source?.key || ''}`;
        function mergeSources(rows) {
            const byKey = new Map(state.sources.map(source => [identityKey(source), source]));
            for (const row of rows) byKey.set(identityKey(row), { ...(byKey.get(identityKey(row)) || {}), ...row });
            state.sources = [...byKey.values()];
        }
        const uncertainReload = async error => { await refreshSnapshot(); notify(t('desktop.infra_write_outcome_unknown'), 'error'); return error; };
        function replaceNodes(list) {
            const at = new Map(state.nodes.map((node, index) => [node.id, index])), nodes = state.nodes.slice(), out = [];
            for (const raw of list || []) {
                if (!raw?.id) continue;
                const normalized = model.normalizeNode(raw);
                if (at.has(normalized.id)) nodes[at.get(normalized.id)] = normalized; else { at.set(normalized.id, nodes.length); nodes.push(normalized); }
                if (!state.dirty.has(normalized.id)) state.drafts.set(normalized.id, model.clone(normalized));
                out.push(normalized);
            }
            if (out.length) { state.nodes = nodes; emit({ type: 'nodes' }); }
            return out;
        }
        // setSourcesIgnored ignores (or restores) sources in chunks of 1000 and
        // drops ignored discoveries from the import sheet.
        async function setSourcesIgnored(identities, ignored) {
            const items = (identities || []).filter(item => item?.kind && item.key).map(item => ({ kind: item.kind, instance: item.instance || '', key: item.key }));
            if (!items.length || !canWrite()) return false;
            setBusy('ignore', true);
            try {
                for (let start = 0; start < items.length; start += 1000) {
                    const chunk = items.slice(start, start + 1000);
                    const result = await request('/sources/ignore', { method: 'PUT', body: { sources: chunk, ignored: !!ignored } });
                    mergeSources(Array.isArray(result?.sources) ? result.sources : chunk.map(item => ({ ...item, ignored: !!ignored })));
                }
                const gone = new Set(items.map(identityKey));
                if (ignored) state.candidates = state.candidates.filter(candidate => !gone.has(identityKey(candidate.source)));
                emit({ type: 'sources' }); emit({ type: 'candidates' });
                return true;
            } catch (error) {
                if (error?.name === 'AbortError') return false;
                if (uncertainMutation(error)) await uncertainReload(error); else notify(messageFor(error), 'error');
                return false;
            } finally { setBusy('ignore', false); }
        }
        // setSourceHost assigns ('' clears) the device that runs a Docker engine
        // with the stored version, then reloads because host_id is a projection.
        async function setSourceHost(instance, nodeId) {
            const key = `source-host-${instance}`;
            if (!canWrite() || !instance || state.busy.has(key)) return false;
            const entry = state.sourceHosts.find(item => item.kind === 'docker' && item.instance === instance);
            setBusy(key, true);
            try {
                const result = await request('/source-hosts', { method: 'PUT', body: { kind: 'docker', instance, node_id: nodeId || '', version: Number(entry?.version) || 0 } });
                const saved = result?.source_host;
                if (saved?.instance) { state.sourceHosts = [...state.sourceHosts.filter(item => !(item.kind === saved.kind && item.instance === saved.instance)), { ...(entry || {}), ...saved }]; emit({ type: 'source-hosts' }); }
                await refreshSnapshot();
                return true;
            } catch (error) {
                if (error?.name === 'AbortError') return false;
                if (error?.status === 409) { await refreshSnapshot(); notify(messageFor(error), 'error'); }
                else if (uncertainMutation(error)) await uncertainReload(error);
                else notify(messageFor(error), 'error');
                return false;
            } finally { setBusy(key, false); }
        }
        // importBatch posts items in chunks of 500 and answers {nodes, errors,
        // uncertain}; imported discoveries leave state.candidates.
        async function importBatch(items) {
            const result = { nodes: [], errors: [], uncertain: false };
            if (!canWrite() || !items?.length) return result;
            setBusy('import', true);
            try {
                for (let start = 0; start < items.length; start += 500) {
                    const chunk = items.slice(start, start + 500);
                    let answer;
                    try { answer = await request('/import/batch', { method: 'POST', body: { items: chunk } }); }
                    catch (error) {
                        if (state.disposed) return result;
                        if (uncertainMutation(error)) { result.uncertain = true; await uncertainReload(error); return result; }
                        for (const item of chunk) result.errors.push({ candidate_id: item.candidate_id, error: String(error?.body?.error || 'infra_error'), message: messageFor(error) });
                        continue;
                    }
                    result.nodes.push(...replaceNodes(Array.isArray(answer?.nodes) ? answer.nodes : []));
                    for (const failure of Array.isArray(answer?.errors) ? answer.errors : []) result.errors.push({ candidate_id: String(failure.candidate_id || ''), error: String(failure.error || 'infra_error'), message: messageFor({ body: { error: failure.error } }) });
                }
                const failed = new Set(result.errors.map(item => item.candidate_id));
                const done = new Set(items.map(item => item.candidate_id).filter(id => !failed.has(id)));
                state.candidates = state.candidates.filter(candidate => !done.has(candidate.id));
                emit({ type: 'candidates' });
                return result;
            } finally { setBusy('import', false); }
        }
        // removeNodes deletes one by one (each with its version) and stops at the
        // first refusal; ignoreSources also ignores the linked sources.
        async function removeNodes(ids, { ignoreSources = false } = {}) {
            const removed = [];
            if (!canWrite()) return removed;
            setBusy('remove', true);
            try {
                for (const id of ids || []) {
                    const node = nodeById(id);
                    if (!node) continue;
                    const linked = new Map((node.sources || []).map(source => [identityKey(source), source]));
                    try {
                        const result = await request(`/nodes/${encodeURIComponent(id)}?version=${encodeURIComponent(node.version)}${ignoreSources ? '&ignore_sources=1' : ''}`, { method: 'DELETE' });
                        if (ignoreSources) mergeSources((Array.isArray(result?.ignored) ? result.ignored : []).map(identity => ({ ...(linked.get(identityKey(identity)) || {}), ...identity, ignored: true })));
                        removeNode(id);
                        removed.push(id);
                    } catch (error) {
                        if (state.disposed) break;
                        if (uncertainMutation(error)) await uncertainReload(error); else notify(messageFor(error), 'error');
                        break;
                    }
                }
                if (ignoreSources && removed.length) emit({ type: 'sources' });
                return removed;
            } finally { setBusy('remove', false); }
        }
        // updateNodeFields writes top-level fields of the stored node (not the
        // draft); one 409 reloads and retries; a dirty draft follows the version.
        async function updateNodeFields(id, patch) {
            for (let attempt = 0; attempt < 2; attempt += 1) {
                const node = nodeById(id);
                if (!node || !canWrite()) return false;
                try {
                    const result = await request(`/nodes/${encodeURIComponent(id)}`, { method: 'PUT', body: { ...model.clone(node), ...patch, id } });
                    const updated = result?.node || result;
                    if (!updated?.id) { await refreshSnapshot(); return true; }
                    const draft = state.drafts.get(id), dirty = state.dirty.has(id);
                    replaceNode(updated);
                    if (dirty && draft && draft.version === node.version) {
                        draft.version = updated.version;
                        for (const key of Object.keys(patch)) if (JSON.stringify(draft[key]) === JSON.stringify(node[key])) draft[key] = updated[key] ?? patch[key];
                    }
                    return true;
                } catch (error) {
                    if (error?.status === 409 && attempt === 0 && await refreshSnapshot()) continue;
                    if (error?.name === 'AbortError') return false;
                    if (uncertainMutation(error)) await uncertainReload(error); else notify(messageFor(error), 'error');
                    return false;
                }
            }
            return false;
        }
        async function updateParents(pairs) { for (const [id, parent] of pairs) if (!await updateNodeFields(id, { parent_id: parent || '' })) return false; return true; }
```
  - Document the new events in the comment above `emit`: `sources`, `candidates`, `source-hosts`.

- [ ] **Step 3: Tests** (before the final `console.log`):
```js
// ---- store: ignore, batch import, removal, parents, unplace undo ------------------
{
    const InfraStore = window.InfraAppStore;
    const node = (id, extra = {}) => ({ id, version: 1, kind: 'device', name: id, icon: 'router', profile: {}, placements: { atlas: { x: 0, y: 0, pinned: true } }, sources: [], ...extra });
    const recorder = routes => {
        const calls = [];
        const api = async (url, options = {}) => {
            const [pathPart, query = ''] = String(url).replace('/api/desktop/infra', '').split('?');
            const method = options.method || 'GET';
            const body = typeof options.body === 'string' ? JSON.parse(options.body) : undefined;
            calls.push({ route: `${method} ${pathPart}`, query, body });
            const handler = routes[`${method} ${pathPart}`] || routes[`${method} ${pathPart.replace(/\/nodes\/[^/]+$/, '/nodes/:id')}`];
            if (!handler) throw Object.assign(new Error('not_found'), { status: 404, body: { error: 'not_found' } });
            return handler(body, { query, path: pathPart });
        };
        return { api, calls };
    };
    const snap = (nodes, extra = {}) => ({ nodes, edges: [], sources: [], source_hosts: [], capabilities: { write: true }, ...extra });
    const newStore = api => InfraStore.create({ ctx: { api }, t: key => key });
    {
        const { api, calls } = recorder({ 'GET /snapshot': () => snap([node('a')], { sources: [{ kind: 'registry', instance: 'inventory', key: 'k1', name: 'TV', state: 'current', ignored: true }] }) });
        const store = newStore(api);
        assert.equal(await store.reloadSnapshot(), true);
        assert.match(calls[0].query, /(^|&)include_ignored=1(&|$)/, 'snapshots ask for ignored sources');
        assert.equal(store.state.sources[0].ignored, true);
        assert.equal(store.features.unplace, true);
        store.dispose();
    }
    {
        const identity = { kind: 'registry', instance: 'inventory', key: 'k1' };
        const { api, calls } = recorder({ 'PUT /sources/ignore': body => ({ sources: body.sources.map(item => ({ ...item, name: 'TV', state: 'current', ignored: body.ignored })) }) });
        const store = newStore(api);
        store.applySnapshot(snap([node('a')]));
        store.state.candidates = [{ id: 'cand-1', source: { ...identity, state: 'current' }, node: { kind: 'device', name: 'TV' } }];
        const events = [];
        store.subscribe(change => events.push(change.type));
        assert.equal(await store.setSourcesIgnored([identity], true), true);
        assert.deepEqual(plain(calls[0].body), { sources: [identity], ignored: true });
        assert.equal(store.state.candidates.length, 0);
        assert.equal(store.state.sources.find(source => source.key === 'k1').ignored, true);
        assert.ok(events.includes('sources') && events.includes('candidates'));
        assert.equal(await store.setSourcesIgnored([identity], false), true);
        assert.equal(store.state.sources.find(source => source.key === 'k1').ignored, false, 'restoring updates the same row');
        store.dispose();
    }
    {
        const { api, calls } = recorder({ 'POST /import/batch': body => ({ nodes: [node('imported-1', { parent_id: 'room', placements: { atlas: { x: 48, y: 72, pinned: true } } })], errors: [{ candidate_id: body.items[1].candidate_id, error: 'source_stale', message: 'x' }] }) });
        const store = newStore(api);
        store.applySnapshot(snap([node('room', { kind: 'room' })]));
        store.state.candidates = [{ id: 'c1', source: {}, node: {} }, { id: 'c2', source: {}, node: {} }];
        const result = await store.importBatch([{ candidate_id: 'c1', parent_id: 'room', placement: { view: 'atlas', x: 48, y: 72 } }, { candidate_id: 'c2' }]);
        assert.deepEqual(plain(calls[0].body.items[0]), { candidate_id: 'c1', parent_id: 'room', placement: { view: 'atlas', x: 48, y: 72 } });
        assert.deepEqual(plain(result.nodes.map(item => item.id)), ['imported-1']);
        assert.deepEqual(plain(result.errors.map(item => [item.candidate_id, item.error])), [['c2', 'source_stale']]);
        assert.equal(store.nodeById('imported-1').parent_id, 'room');
        assert.deepEqual(plain(store.state.candidates.map(item => item.id)), ['c2'], 'a failed discovery stays');
        store.dispose();
    }
    {
        const source = { kind: 'registry', instance: 'inventory', key: 'k9', name: 'Old NAS', state: 'current' };
        const { api, calls } = recorder({ 'DELETE /nodes/:id': (_, { path }) => ({ status: 'deleted', id: path.split('/').pop(), ignored: [{ kind: 'registry', instance: 'inventory', key: 'k9' }] }) });
        const store = newStore(api);
        store.applySnapshot(snap([node('nas', { version: 3, sources: [source] }), node('other')]));
        assert.deepEqual(plain(await store.removeNodes(['nas'], { ignoreSources: true })), ['nas']);
        assert.equal(calls[0].route, 'DELETE /nodes/nas');
        assert.equal(calls[0].query, 'version=3&ignore_sources=1');
        assert.equal(store.nodeById('nas'), undefined);
        assert.deepEqual(plain(store.state.sources.filter(item => item.ignored).map(item => [item.key, item.name])), [['k9', 'Old NAS']]);
        store.dispose();
    }
    {
        const puts = [];
        const { api } = recorder({ 'PUT /nodes/:id': body => { puts.push([body.id, body.parent_id, body.version]); return { node: { ...body, version: body.version + 1 } }; } });
        const store = newStore(api);
        store.applySnapshot(snap([node('a', { placements: {} }), node('room', { kind: 'room' })]));
        store.state.drafts.set('a', { ...store.nodeById('a'), name: 'Draft' });
        store.state.dirty.add('a');
        assert.equal(await store.dispatch(InfraStore.commands.setParent([{ id: 'a', before: '', after: 'room' }], 'moved')), true);
        assert.equal(store.nodeById('a').parent_id, 'room');
        assert.equal(store.state.drafts.get('a').version, 2, 'a dirty draft follows the version');
        assert.equal(store.state.drafts.get('a').parent_id, 'room');
        assert.equal(store.state.drafts.get('a').name, 'Draft');
        assert.equal(await store.undo('atlas'), true);
        assert.equal(store.nodeById('a').parent_id, '');
        assert.deepEqual(puts, [['a', 'room', 1], ['a', '', 2]]);
        store.dispose();
    }
    {
        const { api, calls } = recorder({ 'PUT /layout': body => ({ nodes: body.nodes.map(item => node(item.id, { version: item.version + 1, placements: item.unplace ? {} : { atlas: { x: item.x, y: item.y, pinned: true } } })) }) });
        const store = newStore(api);
        store.applySnapshot(snap([node('a', { placements: {} })]));
        const after = new Map([['a', { x: 96, y: 48, pinned: true }]]);
        const command = InfraStore.commands.placeFromLibrary('atlas', after, 'placed', new Map([['a', { unplace: true }]]));
        assert.equal(command.undoable, true);
        await store.dispatch(command);
        await store.whenLayoutIdle('atlas');
        assert.equal(store.effectivePlacement('a', 'atlas').x, 96);
        assert.equal(await store.undo('atlas'), true);
        assert.equal(store.effectivePlacement('a', 'atlas'), null, 'undo hides the card at once');
        await store.whenLayoutIdle('atlas');
        assert.equal(store.nodeById('a').placements.atlas, undefined);
        assert.deepEqual(plain(calls.map(call => call.body.nodes[0])), [{ id: 'a', version: 1, x: 96, y: 48, pinned: true }, { id: 'a', version: 2, x: 0, y: 0, pinned: false, unplace: true }]);
        assert.equal(InfraStore.commands.placeFromLibrary('atlas', after, '', new Map()).undoable, false);
        store.dispose();
    }
    {
        const { api, calls } = recorder({
            'PUT /source-hosts': body => ({ source_host: { ...body, version: body.version + 1 } }),
            'GET /snapshot': () => snap([node('ct', { kind: 'container', host_id: 'host' }), node('host')], { source_hosts: [{ kind: 'docker', instance: 'engine-a', node_id: 'host', version: 5 }] }),
        });
        const store = newStore(api);
        store.applySnapshot(snap([node('ct', { kind: 'container' }), node('host')], { source_hosts: [{ kind: 'docker', instance: 'engine-a', node_id: '', version: 4, inferred_node_id: '' }] }));
        assert.equal(await store.setSourceHost('engine-a', 'host'), true);
        assert.deepEqual(plain(calls[0].body), { kind: 'docker', instance: 'engine-a', node_id: 'host', version: 4 });
        assert.equal(store.guests().hostOf.get('ct'), 'host');
        store.dispose();
    }
}
```
- [ ] **Step 4: Verify** `node --check ui/js/desktop/apps/infra-store.js`, V-core (exits promptly), then `AURAGO_RUN_BROWSER_SMOKE=1 go test ./ui -run 'TestDesktopInfra(App|CanvasGestures)Browser' -count=1 -timeout 20m`.
- [ ] **Step 5: Commit** paths `ui/js/desktop/apps/infra-store.js scripts/test-infra-ui.mjs`; subject `feat(infra): add ignore, batch import, engine host, removal and undoable placing to the store`.

---

## Task 5: Shell passes context-menu helpers to Infra

**Files:** `ui/js/desktop/core/menus-and-routing.js`, `ui/js/desktop/bundles/main.bundle.js` (generated).

- [ ] **Step 1: Impact** for `renderAppContent` (fan-in is high, expect HIGH/CRITICAL — record the warning; the change only adds two keys to the Infra branch's context object). `git grep -n "InfraApp.render" -- ui/js/desktop/core` → `menus-and-routing.js:1644-1645`.
- [ ] **Step 2:** In the `appId === 'infra'` branch replace `esc, t, api, openApp, setWindowMenus, clearWindowMenus, confirmDialog, readonly: desktopReadonly(),` with `esc, t, api, openApp, setWindowMenus, clearWindowMenus, showContextMenu, wireContextMenuBoundary, confirmDialog, readonly: desktopReadonly(),`.
- [ ] **Step 3:** `node scripts/build-ui-bundles.js` then `node scripts/build-ui-bundles.js --check` (must print `Checked …` for every bundle).
- [ ] **Step 4:** V-core.
- [ ] **Step 5: Commit** paths `ui/js/desktop/core/menus-and-routing.js ui/js/desktop/bundles/main.bundle.js`; subject `feat(desktop): give Infra the shell context menu helpers`.

---

## Task 6: Picker module

**Files:** create `ui/js/desktop/apps/infra-picker.js`; modify `ui/js/desktop/apps/infra.js` (create picker, storage helpers, `openAppLink`, dispose), `ui/js/desktop/core/module-loader.js`, `scripts/test-infra-ui.mjs` (file list), `ui/css/desktop-app-infra.css`.

- [ ] **Step 1: Impact** for `render`/`dispose` (InfraApp) and the module-loader `infra` entry; `git grep -n "infra-store.js" -- ui scripts`.
- [ ] **Step 2: Create `infra-picker.js`:**
```js
// Searchable combobox for Infra (app links, places, hosts, connection ends).
// ARIA 1.2 combobox: focus stays in the input, aria-activedescendant names the
// active option. Popover mode floats in the shell layer; inline mode lives in a form.
(function () {
    'use strict';

    function create(app) {
        const { store, t, esc, model } = app;
        let counter = 0, popover = null;
        const inlines = new Set();
        function optionsMarkup(prefix, result) {
            const flat = [];
            const html = (result.groups || []).filter(group => group.options.length).map((group, g) => {
                const heading = `${prefix}-g${g}`;
                const options = group.options.map(option => {
                    const index = flat.length, id = `${prefix}-o${index}`;
                    flat.push({ ...option, domId: id });
                    const attrs = Object.entries(option.attrs || {}).map(([key, value]) => ` ${key}="${esc(value)}"`).join('');
                    return `<div class="infra-picker-option" role="option" id="${id}" aria-selected="false" data-picker-option="${index}"${attrs}>${option.icon ? app.icon(option.icon) : ''}<span class="infra-picker-label">${esc(option.label)}</span>${option.meta ? `<small class="infra-picker-meta">${esc(option.meta)}</small>` : ''}</div>`;
                }).join('');
                const more = group.more ? `<div class="infra-picker-more">${esc(model.fill(t('desktop.infra_picker_more'), { count: group.more }))}</div>` : '';
                return `<div class="infra-picker-group" role="group" aria-labelledby="${heading}"><div class="infra-picker-heading" id="${heading}">${esc(group.label)}</div>${options}${more}</div>`;
            }).join('');
            return { html, flat };
        }
        function controller({ input, list, note, empty, source, onPick, onEscape }) {
            let flat = [], active = -1;
            const setActive = next => {
                active = next;
                for (const option of list.querySelectorAll('[data-picker-option]')) { const on = Number(option.dataset.pickerOption) === next; option.classList.toggle('is-active', on); option.setAttribute('aria-selected', String(on)); }
                const current = flat[next];
                input.setAttribute('aria-activedescendant', current ? current.domId : '');
                if (current) list.querySelector(`[id="${current.domId}"]`)?.scrollIntoView?.({ block: 'nearest' });
            };
            const refresh = () => {
                const result = source(input.value) || { groups: [] };
                const built = optionsMarkup(list.id, result);
                flat = built.flat; list.innerHTML = built.html;
                empty.hidden = flat.length > 0;
                note.textContent = result.note || ''; note.hidden = !result.note;
                setActive(flat.length ? 0 : -1);
            };
            const pick = index => { const option = flat[index]; if (option) onPick(option); };
            const onKey = event => {
                if (event.key === 'ArrowDown' || event.key === 'ArrowUp') { event.preventDefault(); if (flat.length) setActive((active + (event.key === 'ArrowDown' ? 1 : -1) + flat.length) % flat.length); }
                else if (event.key === 'Enter') { event.preventDefault(); if (active >= 0) pick(active); } // never submits the surrounding form
                else if (event.key === 'Escape') { event.preventDefault(); event.stopPropagation(); onEscape(); }
            };
            const onInput = () => refresh();
            const keepFocus = event => { if (event.target.closest('[data-picker-option]')) event.preventDefault(); };
            const onClick = event => { const option = event.target.closest('[data-picker-option]'); if (option) pick(Number(option.dataset.pickerOption)); };
            input.addEventListener('keydown', onKey); input.addEventListener('input', onInput);
            list.addEventListener('pointerdown', keepFocus); list.addEventListener('mousedown', keepFocus); list.addEventListener('click', onClick);
            return { refresh, destroy() { input.removeEventListener('keydown', onKey); input.removeEventListener('input', onInput); list.removeEventListener('pointerdown', keepFocus); list.removeEventListener('mousedown', keepFocus); list.removeEventListener('click', onClick); } };
        }
        const skeleton = (id, label, placeholder, expanded) => `<input type="text" class="infra-picker-input" data-picker-input role="combobox" aria-expanded="${expanded}" aria-controls="${id}" aria-autocomplete="list" aria-activedescendant="" aria-label="${esc(label)}" placeholder="${esc(placeholder)}" autocomplete="off" spellcheck="false"><div class="infra-picker-list" id="${id}" role="listbox" aria-label="${esc(label)}" data-picker-list ${expanded ? '' : 'hidden'}></div><p class="infra-picker-note" data-picker-note hidden></p><p class="infra-picker-empty" data-picker-empty hidden>${esc(t('desktop.infra_picker_no_match'))}</p>`;
        function place(el, rect) {
            const box = app.root.getBoundingClientRect(), width = el.offsetWidth || 320, below = rect.bottom - box.top + 6;
            el.style.left = `${Math.max(8, Math.min(rect.left - box.left, box.width - width - 8))}px`;
            el.style.top = `${below}px`;
            if (below + el.offsetHeight > box.height - 8) el.style.top = `${Math.max(8, rect.top - box.top - el.offsetHeight - 6)}px`;
        }
        // open shows a popover under anchor (or under rect when the anchor may be repainted).
        function open({ anchor = null, rect = null, label, placeholder = '', source, onPick, onClose = () => {} }) {
            close({ restoreFocus: false });
            const layer = app.q('[data-layer]');
            if (!layer) return;
            const id = `infra-picker-${app.safeId}-${++counter}`, el = document.createElement('div');
            el.className = 'infra-popover infra-picker'; el.dataset.picker = '';
            el.setAttribute('role', 'dialog'); el.setAttribute('aria-label', label);
            el.innerHTML = skeleton(id, label, placeholder || label, true);
            layer.append(el);
            const input = el.querySelector('[data-picker-input]');
            const ctl = controller({ input, list: el.querySelector('[data-picker-list]'), note: el.querySelector('[data-picker-note]'), empty: el.querySelector('[data-picker-empty]'), source, onPick: option => { close(); onPick(option); }, onEscape: () => close() });
            ctl.refresh();
            place(el, rect || anchor?.getBoundingClientRect() || app.root.getBoundingClientRect());
            const outside = event => { if (!el.contains(event.target)) close({ restoreFocus: false }); };
            const focusout = event => { if (event.relatedTarget && !el.contains(event.relatedTarget)) close({ restoreFocus: false }); };
            document.addEventListener('pointerdown', outside, true);
            el.addEventListener('focusout', focusout);
            popover = { el, ctl, anchor, onClose, outside, focusout };
            input.focus({ preventScroll: true });
        }
        function close({ restoreFocus = true } = {}) {
            if (!popover) return;
            const { el, ctl, anchor, onClose, outside, focusout } = popover;
            popover = null;
            document.removeEventListener('pointerdown', outside, true);
            el.removeEventListener('focusout', focusout);
            ctl.destroy(); el.remove();
            if (restoreFocus && anchor?.isConnected) anchor.focus({ preventScroll: true });
            onClose();
        }
        // attachInline turns container into an inline combobox: {focus, refresh({clear}), destroy}.
        function attachInline(container, { label, placeholder = '', source, onPick }) {
            const id = `infra-picker-${app.safeId}-${++counter}`;
            container.classList.add('infra-picker', 'infra-picker-inline'); container.dataset.picker = '';
            container.innerHTML = skeleton(id, label, placeholder || label, false);
            const input = container.querySelector('[data-picker-input]'), list = container.querySelector('[data-picker-list]');
            const note = container.querySelector('[data-picker-note]'), empty = container.querySelector('[data-picker-empty]');
            const setOpen = isOpen => { list.hidden = !isOpen; input.setAttribute('aria-expanded', String(isOpen)); if (!isOpen) { input.setAttribute('aria-activedescendant', ''); note.hidden = true; empty.hidden = true; } };
            const ctl = controller({ input, list, note, empty, source, onPick: option => { onPick(option); input.value = ''; ctl.refresh(); input.focus({ preventScroll: true }); }, onEscape: () => setOpen(false) });
            const show = () => { if (list.hidden) { setOpen(true); ctl.refresh(); } };
            const onKey = event => { if (event.key === 'ArrowDown') show(); };
            const onFocusOut = event => { if (!container.contains(event.relatedTarget)) setOpen(false); };
            input.addEventListener('focus', show); input.addEventListener('input', show); input.addEventListener('keydown', onKey);
            container.addEventListener('focusout', onFocusOut);
            const handle = {
                focus() { input.focus({ preventScroll: true }); show(); },
                refresh({ clear = false } = {}) { if (clear) input.value = ''; if (!list.hidden) ctl.refresh(); },
                destroy() { ctl.destroy(); input.removeEventListener('focus', show); input.removeEventListener('input', show); input.removeEventListener('keydown', onKey); container.removeEventListener('focusout', onFocusOut); inlines.delete(handle); },
            };
            inlines.add(handle);
            return handle;
        }
        const nodeOption = node => ({ id: node.id, label: node.name, meta: [app.labelForKind(node.kind), node.profile?.ip_address].filter(Boolean).join(' · '), icon: node.icon, attrs: { 'data-node-option': node.id } });
        function assetSource(accept, preferred = []) {
            return query => {
                const q = String(query || '').trim().toLocaleLowerCase();
                const nodes = store.state.nodes.filter(node => accept(node) && (!q || store.searchableText(node).includes(q)));
                const first = preferred.map(id => nodes.find(node => node.id === id)).filter(Boolean);
                const rest = nodes.filter(node => !preferred.includes(node.id)).sort((a, b) => model.naturalCompare(a.name, b.name));
                const groups = first.length ? [{ id: 'suggested', label: t('desktop.infra_picker_suggested'), options: first.map(nodeOption) }] : [];
                groups.push({ id: 'all', label: t('desktop.infra_link_all'), options: rest.slice(0, 100).map(nodeOption), more: Math.max(0, rest.length - 100) });
                return { groups };
            };
        }
        const openAssetPicker = ({ anchor = null, rect = null, label, accept, preferred = [], onPick }) => open({ anchor, rect, label, placeholder: t('desktop.infra_pick_asset'), source: assetSource(accept, preferred), onPick });
        // openPlacePicker offers "No place" plus every place except exclude and its descendants.
        function openPlacePicker({ anchor = null, rect = null, exclude = [], onPick }) {
            const parentOf = id => store.nodeById(id)?.parent_id || '';
            const blocked = node => exclude.includes(node.id) || exclude.some(id => model.isDescendant(node.id, id, parentOf));
            open({ anchor, rect, label: t('desktop.infra_move_to_place'), placeholder: t('desktop.infra_pick_place'), onPick: option => onPick(option.id), source: query => {
                const q = String(query || '').trim().toLocaleLowerCase();
                const places = store.state.nodes.filter(node => model.isContainer(node.kind) && !blocked(node))
                    .map(node => ({ node, path: model.placePath(node.id, store.nodeById, { includeSelf: true }).map(item => item.name).join(' › ') }))
                    .filter(item => !q || item.path.toLocaleLowerCase().includes(q)).sort((a, b) => model.naturalCompare(a.path, b.path));
                const options = [...(q ? [] : [{ id: '', label: t('desktop.infra_no_place'), attrs: { 'data-place-option': '' } }]), ...places.slice(0, 100).map(item => ({ id: item.node.id, label: item.path, icon: item.node.icon, attrs: { 'data-place-option': item.node.id } }))];
                return { groups: [{ id: 'places', label: t('desktop.infra_place'), options, more: Math.max(0, places.length - 100) }] };
            } });
        }
        return { open, close, isOpen: () => !!popover, attachInline, assetSource, openAssetPicker, openPlacePicker, dispose() { close({ restoreFocus: false }); for (const handle of [...inlines]) handle.destroy(); } };
    }

    window.InfraAppPicker = { create };
})();
```
- [ ] **Step 3: `infra.js`:** before creating modules add `app.readStorage = key => { try { return window.localStorage.getItem(key); } catch (_) { return null; } }; app.writeStorage = (key, value) => { try { window.localStorage.setItem(key, value); } catch (_) { /* storage disabled */ } }; app.openAppLink = link => ctx.openApp?.(link.app, model.appLinkContext(link));`, then `app.picker = window.InfraAppPicker.create(app);` as the first module; `dispose` calls `app.picker.dispose()` first. On `caps` routing also call `app.picker.close({ restoreFocus: false })`.
- [ ] **Step 4:** Register `/js/desktop/apps/infra-picker.js` after `infra-store.js` in `module-loader.js` and `'infra-picker.js'` in the test list.
- [ ] **Step 5: CSS** (append to `desktop-app-infra.css`):
```css
.infra-picker { display: grid; gap: var(--ds-space-2); width: 320px; max-height: min(420px, 70vh); padding: var(--ds-space-2); }
.infra-picker-inline { position: relative; width: 100%; max-height: none; padding: 0; }
.infra-picker-input { width: 100%; height: 36px; padding: 0 var(--ds-space-2); border: 1px solid var(--vd-theme-border); border-radius: var(--ds-radius-sm); background: var(--vd-theme-control-bg); color: var(--vd-text); font-size: 14px; }
.infra-picker-input:focus { border-color: var(--ds-color-border-focus); outline: none; }
.infra-picker-list { display: grid; gap: 2px; max-height: 320px; overflow: auto; }
.infra-picker-inline .infra-picker-list { position: absolute; left: 0; right: 0; top: calc(100% + 4px); z-index: 12; padding: var(--ds-space-1); border: 1px solid var(--vd-theme-border); border-radius: var(--vd-theme-radius-panel); background: var(--vd-theme-panel-bg-strong); box-shadow: var(--vd-theme-shadow-modal); }
.infra-picker-heading { padding: var(--ds-space-1) var(--ds-space-2) 2px; color: var(--vd-theme-muted); font-size: 13px; font-weight: 600; }
.infra-picker-option { display: grid; grid-template-columns: 24px minmax(0, 1fr) auto; align-items: center; gap: var(--ds-space-2); min-height: 36px; padding: 2px var(--ds-space-2); border-radius: var(--ds-radius-sm); font-size: 14px; cursor: pointer; }
.infra-picker-option:not(:has(img)) { grid-template-columns: minmax(0, 1fr) auto; }
.infra-picker-option img { width: 24px; height: 24px; object-fit: contain; }
.infra-picker-option:hover, .infra-picker-option.is-active { background: var(--vd-theme-control-hover); }
.infra-picker-option.is-active { box-shadow: inset 0 0 0 1px var(--vd-theme-accent-border); }
.infra-picker-label, .infra-picker-meta { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.infra-picker-meta, .infra-picker-note, .infra-picker-more, .infra-picker-empty { margin: 0; color: var(--vd-theme-muted); font-size: 13px; }
.infra-picker-note, .infra-picker-more, .infra-picker-empty { padding: 0 var(--ds-space-2); }
@media (pointer: coarse) { .infra-picker-option { min-height: 44px; } }
```
- [ ] **Step 6: Verify** `node --check` both files, V-core.
- [ ] **Step 7: Commit** paths `ui/js/desktop/apps/infra-picker.js ui/js/desktop/apps/infra.js ui/js/desktop/core/module-loader.js scripts/test-infra-ui.mjs ui/css/desktop-app-infra.css`; subject `feat(infra): add searchable picker combobox`.

---

## Task 7: Library sections, Cleanup module, place-row drops

**Files:** create `ui/js/desktop/apps/infra-cleanup.js`; modify `infra-library.js`, `infra-gestures.js`, `infra.js`, `module-loader.js`, `scripts/test-infra-ui.mjs`, `ui/css/desktop-app-infra.css`, `ui/desktop_infra_browser_test.go`.

- [ ] **Step 1: Impact** for library `create`/`render`/`computeSections`, gestures `planDrop`/`commitPlan`/`startLibraryDrag`, infra.js `paintShell` and the routing function; `git grep -n -e "needs-host" -e "startLibraryDrag" -e "commitPlan" -e "suppressNextClick" -- ui`.

- [ ] **Step 2: Create `infra-cleanup.js`** (`InfraAppCleanup.create(app)`):
  - Constants/state: `DISMISSED_KEY = 'infra.dismissed.v1'`; `dismissed = model.readDismissed(app.readStorage(DISMISSED_KEY))`; `expanded = new Set()`; `memo` keyed by `[state.nodes, state.edges, state.sourceHosts]` identity holding `model.infraCleanup(state.nodes, state.sources, state.sourceHosts, { edges: state.edges })`.
  - `visible()` = memo items not dismissed with some `nodeIds` node matching `store.matches`; `dismissedCount()`.
  - Text: duplicate → `model.fill(t(item.match === 'mac' ? 'desktop.infra_cleanup_duplicate_mac' : 'desktop.infra_cleanup_duplicate'), { names: nodeIds.map(name).join(', '), address: item.value })`; engine-host → `desktop.infra_cleanup_engine_host` `{count: guestIds.length}`; guest-host → `desktop.infra_cleanup_guest_host`; opaque → `desktop.infra_cleanup_opaque`. Primary labels: `infra_cleanup_remove_duplicate`, `infra_assign_engine_host`, `infra_choose_host`, `infra_rename`.
  - Item markup (`index` from the list):
```html
<li class="infra-cleanup-item" data-cleanup="{id}" data-cleanup-type="{type}">
  <p class="infra-cleanup-text">{text}</p>
  [duplicate: <p class="infra-cleanup-meta">{fill(t desktop.infra_cleanup_keep, {name: keep})}</p>]
  [engine-host: <button type="button" class="infra-library-disclosure infra-cleanup-toggle" data-cleanup-expand="{id}" aria-expanded="{open}" aria-controls="infra-cleanup-{S}-{index}">{glyph chevron}<span>{open ? t hide_guests : fill(t show_guests,{count})}</span></button>
   <ul class="infra-library-children" id="infra-cleanup-{S}-{index}" {hidden unless open}>{open: per guest <li class="infra-library-row" data-library-item="{gid}" data-section="cleanup" aria-selected="{sel}"><button type="button" class="infra-library-row-main" data-library-select="{gid}" tabindex="-1" title="{name}"><img class="infra-library-row-icon" src="{iconURL}" alt="" draggable="false"><span class="infra-library-row-text"><span class="infra-library-row-name">{name}</span><span class="infra-library-row-meta">{kind label}</span></span></button></li>}</ul>]
  <div class="infra-cleanup-actions"><button type="button" class="infra-button infra-button-primary" data-cleanup-action="{id}" [aria-haspopup="dialog" for engine-host/guest-host] [disabled title=read_only when !canWrite]>{primary}</button><button type="button" class="infra-button" data-cleanup-dismiss="{id}">{t desktop.infra_dismiss}</button></div>
</li>
```
  - `renderCleanup({section, list, count, reset})`: list innerHTML, count text, `reset.hidden = dismissedCount()===0`, reset text `fill(t('desktop.infra_cleanup_show_dismissed'), {count})`, `section.hidden = !items.length && !dismissedCount()`.
  - `renderIgnored({section, list, count})`: rows = `state.sources.filter(s => s.ignored)` filtered by the search query on name/key/kind, sorted by `naturalCompare(name||key)`; row `<li class="infra-ignored-row" data-ignored-source="{model.sourceIdentity(source)}"><span class="infra-ignored-copy"><strong title="{name}">{name}</strong><small>{t desktop.infra_source_<kind>}</small></span><button type="button" class="infra-button" data-restore-source="{same id}" aria-label="{fill(t restore_source,{name})}" {disabled !canWrite}>{t desktop.infra_restore}</button></li>`; `section.hidden = rows.length === 0`.
  - `onClick(event)` (returns true when handled): `[data-cleanup-expand]` toggles `expanded` and calls `app.library.render()`; `[data-cleanup-dismiss]` adds to `dismissed`, writes `JSON.stringify([...dismissed].slice(-500))`, renders; `[data-action="cleanup-reset"]` clears dismissed (writes `'[]'`), renders; `[data-cleanup-action]` → `run(id, button)`; `[data-restore-source]` → `restoreSource(id)`.
  - `run(id, button)` (capture `rect = button.getBoundingClientRect()` first; wrap each write branch in try/catch → `store.notify(store.messageFor(error), 'error')`):
    - duplicate: `await app.dialogs.confirmAction(t('desktop.infra_cleanup_remove_duplicate'), model.fill(t('desktop.infra_cleanup_remove_confirm'), { names, keep }), { confirmLabel: t('desktop.infra_cleanup_remove_duplicate') })`; then `store.removeNodes(item.removeIds, { ignoreSources: true })`; announce `fill(t('desktop.infra_removed_ignored'), {count})`.
    - opaque: `app.inspector.open(item.nodeId, 'details'); app.inspector.focusName();` (until Task 10 lands, `open` is `store.select([id])`).
    - engine-host: `app.picker.openAssetPicker({ anchor: button, rect, label: t('desktop.infra_assign_engine_host'), accept: node => node.kind === 'device', preferred: [item.inferredNodeId].filter(Boolean), onPick: async option => { if (await store.setSourceHost(item.instance, option.id)) app.announce(model.fill(t('desktop.infra_engine_host_assigned'), { name: option.label })); } })`.
    - guest-host: accept `node.id !== guestId && (node.kind === 'device' || (guest.kind === 'container' && node.kind === 'vm'))`; onPick dispatches `InfraAppStore.commands.createEdge({ from: guestId, to: option.id, kind: 'runs_on' })` and announces `fill(t('desktop.infra_host_chosen'), {name, host})`.
  - `restoreSource(id)`: find ignored source by `model.sourceIdentity`; `store.setSourcesIgnored([source], false)` → announce `fill(t('desktop.infra_restored'), {name})`, then `app.importer?.reload()`.
- [ ] **Step 3: Library changes** (`infra-library.js`):
  - Replace the `needs-host` section with three sections built once, in this order after On the map: `attention` (title `t('desktop.infra_attention')`, list `<ul class="infra-library-list" data-section-list="attention">`, section `hidden` initially), `cleanup` (title `t('desktop.infra_cleanup')`, `<ul class="infra-cleanup-list" data-section-list="cleanup"></ul><button type="button" class="infra-library-reset" data-action="cleanup-reset" hidden></button>`), `ignored` (title `t('desktop.infra_ignored')`, toggle `aria-expanded="false"`, list `hidden`). Same heading/toggle/count markup as Plan B.
  - On the map heading: wrap as `<h2 class="infra-library-heading"><button …toggle…></button><button type="button" class="infra-library-add" data-action="library-add-place" aria-label="{t desktop.infra_new_place}" title="…" {disabled !canWrite}>{glyph plus}</button></h2>`; click → `app.dialogs.openNodeDialog('room', { title: t('desktop.infra_new_place') })`.
  - `computeSections`: drop `needsHost` (To place keeps excluding hostless vm/container). Rows: place-kind rows (To place and On the map) get `data-library-drop="{id}"` when `store.canWrite()`; On-the-map asset rows that are not places and not guests get `data-draggable="true"` when `canWrite`.
  - Attention rows: `items = model.attentionItems(state.nodes.filter(store.matches), { unsaved, reviews: new Set(state.nodeReviews.keys()), sourceBlocked: state.sourceBlocked, actionBlocked })` with `unsaved` = nodes whose `store.layoutState(id, 'atlas'|'topology') === 'unsaved'` and `actionBlocked` = first `'\x00'` segment of every `state.actionBlocks` key. Row = Plan B row markup with `data-section="attention"`, meta `<span class="infra-library-row-meta infra-attention-reason">{first reason text}{more: ' · ' + fill(t desktop.infra_more_guests,{count})}</span>` and `data-attention-tab` on `[data-library-select]`: `source_*`, `source_review`, `action_review` → `sources`; `draft_conflict` → `details`; `layout_unsaved` → `overview`. Reason texts: `fill(t('desktop.infra_attention_source_stale'|…unknown|…removed), { source: t(\`desktop.infra_source_${kind}\`) })`, `t('desktop.infra_attention_layout_unsaved')`, `…draft_conflict`, `…source_review`, `…action_review`.
  - `render()` calls `app.cleanup.renderCleanup(…)` and `renderIgnored(…)` with the section elements; counts include filters.
  - Click delegation: first `if (app.cleanup.onClick(event)) return;`; `[data-library-select]` with `data-attention-tab` → `app.inspector.open ? app.inspector.open(id, tab) : store.select([id])` plus `app.canvas.ensureVisible(id)` when placed.
  - Pointerdown on `[data-draggable="true"]`: `ids` = selected rows of the same `data-section` if this row is selected, else `[id]`; `app.gestures.startLibraryDrag(event, ids, { mode: row.dataset.section === 'on-map' ? 'move' : 'place' })`.
  - Keydown on `[data-library-select]`: `Delete`/`Backspace` (canWrite, asset exists) → `app.dialogs.deleteNodes(state.selection.has(id) && state.selection.size > 1 ? [...state.selection] : [id])` (until Task 10, `deleteNodes` is Plan B's).
  - Focus restore after a rebuild also covers `[data-cleanup-action]`, `[data-cleanup-dismiss]`, `[data-cleanup-expand]`, `[data-restore-source]` by attribute value; when the element is gone, focus the first `[data-cleanup-action]` in the section, else the section toggle.
  - New API `setDropTarget(id)`: toggles `is-drop-target` on `[data-library-drop="{CSS.escape(id)}"]`, `''` clears.
- [ ] **Step 4: Gestures changes** (`infra-gestures.js`):
  - In `planDrop` replace `before.set(id, store.currentIntent(id, view, 'parent_id' in parentChange));` with `before.set(id, store.currentIntent(id, view, 'parent_id' in parentChange) || (store.features.unplace ? { unplace: true, ...('parent_id' in parentChange ? { parent_id: current } : {}) } : null));`.
  - In `commitPlan` use `commands.placeFromLibrary(view, plan.after, label, plan.before)`.
  - Viewport pointerdown: return immediately when `event.button === 2`.
  - `startLibraryDrag(event, ids, { mode = 'place', items = null, onDragged = () => app.library.suppressNextClick() } = {})`. In the move frame, before the canvas test: `const dropRow = mode !== 'discovery' ? hit?.closest?.('[data-library-drop]') : null; const placeTarget = dropRow && app.library.el.contains(dropRow) && !ids.includes(dropRow.dataset.libraryDrop) ? dropRow.dataset.libraryDrop : '';` → `app.library.setDropTarget(placeTarget)`; with a target, `canvas.setDropTarget('')` and `ghost.move(x, y, { scale: 1, label: model.fill(t('desktop.infra_drop_into'), { name: store.nodeById(placeTarget)?.name || '' }) })`; remember `drag.placeTarget`. Replace the "Up, over the canvas" branch with `void finishLibraryDrop(ids, { mode, placeTarget: drag.placeTarget, world: over ? world : null, region })`; on every end also `app.library.setDropTarget('')`; after a real drag call `onDragged()`.
  - Add and export:
```js
        // finishLibraryDrop commits a library drag that ended over the map or on a place row.
        function finishLibraryDrop(ids, { mode = 'place', placeTarget = '', world = null, region = '' } = {}) {
            const view = app.mapView();
            if (placeTarget) return moveIntoPlace(ids, placeTarget);
            if (!world) return Promise.resolve(false);
            const points = ids.length === 1 ? [world] : model.compactGrid(ids.length, world);
            const plan = planDrop(view, new Map(ids.map((id, index) => [id, points[index]])), { nudge: true });
            commitPlan(view, plan, mode === 'move' ? 'move' : 'place');
            store.select(ids);
            return Promise.resolve(true);
        }
        // positionsAt returns free card centres around a world point.
        function positionsAt(count, world) {
            const obstacles = canvas.sceneData().cards.map(model.cardRect);
            const points = count === 1 ? [world] : model.compactGrid(count, world);
            return points.map(point => { const pos = model.freeSlot(point, obstacles); obstacles.push(model.cardRect(pos)); return { x: Math.round(pos.x), y: Math.round(pos.y) }; });
        }
        // moveIntoPlace puts assets into a place ('' = out of every place): placed
        // assets with a drawn target move inside it in one layout save, the rest
        // only change parent_id (node update).
        function moveIntoPlace(ids, placeId) {
            if (!store.canWrite()) return Promise.resolve(false);
            const view = app.mapView(), scene = canvas.sceneData(), parentOf = id => store.effectiveParent(id, view);
            const valid = ids.filter(id => store.nodeById(id) && id !== placeId && !(placeId && model.isDescendant(placeId, id, parentOf)) && parentOf(id) !== placeId);
            if (!valid.length) return Promise.resolve(false);
            const target = placeId ? scene.regionRects.get(placeId) : null, layoutIds = [], parentIds = [];
            for (const id of valid) {
                const node = store.nodeById(id);
                if (view === 'atlas' && store.features.layoutParent && scene.cardById.has(id) && !model.isContainer(node.kind) && (!placeId || target)) layoutIds.push(id); else parentIds.push(id);
            }
            const name = id => store.nodeById(id)?.name || id;
            const label = placeId ? model.fill(t('desktop.infra_moved_to_place'), { count: valid.length, place: name(placeId) }) : model.fill(t('desktop.infra_moved_out_of_place'), { count: valid.length });
            const { commands } = window.InfraAppStore, work = [];
            if (layoutIds.length) {
                const moving = new Set(layoutIds), obstacles = scene.cards.filter(card => !moving.has(card.id)).map(model.cardRect);
                const first = scene.cardById.get(layoutIds[0]), from = scene.regionRects.get(parentOf(layoutIds[0]));
                const centre = target ? { x: target.x + target.w / 2, y: target.y + target.h / 2 } : from ? { x: from.x + from.w + model.REGION_PAD + model.CARD_W / 2, y: first.y } : { x: first.x, y: first.y };
                const wants = model.compactGrid(layoutIds.length, centre), before = new Map(), after = new Map(), regionIds = new Set(scene.regionRects.keys());
                const placementOf = id => after.get(id) || store.effectivePlacement(id, view);
                layoutIds.forEach((id, index) => {
                    const pos = model.freeSlot(wants[index], obstacles);
                    obstacles.push(model.cardRect(pos));
                    before.set(id, store.currentIntent(id, view, true));
                    after.set(id, { x: Math.round(pos.x), y: Math.round(pos.y), pinned: true, parent_id: placeId });
                    if (!placeId) return;
                    for (const [regionId, rect] of model.growRegions(placeId, model.cardRect(pos), { ids: regionIds, parentOf, placementOf })) {
                        if (!before.has(regionId)) before.set(regionId, store.currentIntent(regionId, view, false));
                        after.set(regionId, { ...(after.get(regionId) || {}), x: Math.round(rect.x), y: Math.round(rect.y), w: Math.round(rect.w), h: Math.round(rect.h), pinned: true });
                    }
                });
                work.push(store.dispatch(commands.reparent(view, before, after, label)));
            }
            if (parentIds.length) work.push(store.dispatch(commands.setParent(parentIds.map(id => ({ id, before: store.nodeById(id).parent_id || '', after: placeId })), label)));
            app.announce(label);
            return Promise.all(work).then(results => results.every(Boolean)).catch(error => { store.notify(store.messageFor(error), 'error'); return false; });
        }
        // removeFromMap unplaces assets in the current map view (undoable).
        function removeFromMap(ids) {
            const view = app.mapView(), before = new Map();
            for (const id of ids) { const intent = store.currentIntent(id, view, false); if (intent) before.set(id, intent); }
            if (!before.size || !store.canWrite() || !store.features.unplace) return Promise.resolve(false);
            const label = model.fill(t('desktop.infra_removed_from_map'), { count: before.size });
            app.announce(label);
            return store.dispatch(window.InfraAppStore.commands.unplace(view, before, label));
        }
```
  (`desktop.infra_removed_from_map` arrives in batch I; until Task 13, add it to batch F instead: run Task 3's script again with a one-key JSON before this task, or move the key into batch F before running it. **Do the latter**: add `desktop.infra_removed_from_map` | {count} assets removed from the map | {count} Assets von der Karte genommen to batch F; batch I then omits it.)
- [ ] **Step 5: `infra.js`:** create `app.cleanup = window.InfraAppCleanup.create(app)` before the library; `paintShell` ledger counts `state.sources.filter(source => !source.ignored)`; routing adds `sources` → `library.render(); paintShell();`, `source-hosts` → `library.render(); canvas.render();`, and `layout-status` also calls `library.render()`. Register `infra-cleanup.js` after `infra-library.js` in loader and test list.
- [ ] **Step 6: CSS:**
```css
.infra-library-heading { display: flex; align-items: center; gap: 2px; }
.infra-library-heading .infra-library-toggle { flex: 1; min-width: 0; }
.infra-library-add { width: 32px; height: 32px; display: grid; place-items: center; border: 1px solid transparent; border-radius: var(--ds-radius-sm); background: transparent; color: var(--vd-theme-muted); cursor: pointer; }
.infra-library-add:hover:not(:disabled) { background: var(--vd-theme-control-hover); color: var(--vd-text); }
.infra-library-row.is-drop-target > .infra-library-row-main { background: var(--vd-theme-accent-soft); box-shadow: inset 0 0 0 2px var(--vd-accent); }
.infra-attention-reason { color: var(--ds-color-warning-fg); }
.infra-cleanup-list { display: grid; gap: var(--ds-space-2); margin: var(--ds-space-1) 0 0; padding: 0; list-style: none; }
.infra-cleanup-item { display: grid; gap: var(--ds-space-2); padding: var(--ds-space-2) var(--ds-space-3); border: 1px solid var(--vd-theme-border); border-radius: var(--ds-radius-md); background: var(--vd-theme-panel-bg-strong); }
.infra-cleanup-text, .infra-cleanup-meta { margin: 0; font-size: 14px; overflow-wrap: anywhere; }
.infra-cleanup-meta { color: var(--vd-theme-muted); }
.infra-cleanup-actions { display: flex; flex-wrap: wrap; gap: var(--ds-space-2); }
.infra-cleanup-actions .infra-button { height: 32px; }
.infra-library-disclosure.infra-cleanup-toggle { width: auto; justify-self: start; display: inline-flex; gap: var(--ds-space-1); padding: 0 var(--ds-space-2); font-size: 14px; }
.infra-ignored-row { display: grid; grid-template-columns: minmax(0, 1fr) auto; align-items: center; gap: var(--ds-space-2); min-height: 44px; padding: 2px var(--ds-space-2); }
.infra-ignored-copy { display: grid; min-width: 0; }
.infra-ignored-copy strong, .infra-ignored-copy small { overflow: hidden; font-size: 14px; text-overflow: ellipsis; white-space: nowrap; }
.infra-ignored-copy small { color: var(--vd-theme-muted); }
.infra-library-reset { justify-self: start; margin: var(--ds-space-1) var(--ds-space-2); border: 0; background: transparent; color: var(--vd-accent); font-size: 14px; text-decoration: underline; cursor: pointer; }
```
- [ ] **Step 7: Main browser test update (conscious):** in `TestDesktopInfraAppBrowser` replace Plan B's `page.MustWait(\`()=>!!document.querySelector('[data-library-item="imported-2"]')\`)` with `page.MustWait(\`()=>!!document.querySelector('[data-cleanup="engine-host:docker:engine-a"]')\`)` — the hostless Docker guest is now a Cleanup suggestion.
- [ ] **Step 8: Verify** `node --check` (cleanup, library, gestures, infra), V-core, V-browser.
- [ ] **Step 9: Commit** paths: the six JS files, `ui/js/desktop/core/module-loader.js`, `scripts/test-infra-ui.mjs`, `ui/css/desktop-app-infra.css`, `ui/desktop_infra_browser_test.go`, `ui/lang/desktop/*.json` (only if Step 4's key was added here); subject `feat(infra): add attention, cleanup and ignored library sections with place-row drops`.

---

## Task 8: i18n batch G (inspector)

Same procedure as Task 3 with `$SCRATCH/infra-i18n-g.json` (expect `+36`). Commit subject `i18n(infra): add inspector tab, form and connection keys`.

| key | en | de |
|---|---|---|
| `desktop.infra_tab_overview` | Overview | Übersicht |
| `desktop.infra_tab_details` | Details | Details |
| `desktop.infra_tab_sources` | Sources | Quellen |
| `desktop.infra_tab_connections` | Connections | Verbindungen |
| `desktop.infra_inspector_tabs` | Asset sections | Asset-Bereiche |
| `desktop.infra_change_icon` | Change icon | Symbol ändern |
| `desktop.infra_icon_search` | Search icons | Symbole suchen |
| `desktop.infra_edit` | Edit | Bearbeiten |
| `desktop.infra_open` | Open | Öffnen |
| `desktop.infra_open_in_app` | Open in {app} | In {app} öffnen |
| `desktop.infra_fact_hardware` | Vendor and model | Hersteller und Modell |
| `desktop.infra_fact_host` | Runs on | Läuft auf |
| `desktop.infra_fact_guests` | Guests | Gäste |
| `desktop.infra_fact_apps` | Linked apps | Verknüpfte Apps |
| `desktop.infra_fact_last_seen` | Last seen | Zuletzt gesehen |
| `desktop.infra_host_inferred` | inferred | vermutet |
| `desktop.infra_connections_summary` | {count} connections | {count} Verbindungen |
| `desktop.infra_group_network` | Network | Netzwerk |
| `desktop.infra_group_hardware` | Hardware | Hardware |
| `desktop.infra_group_links` | Links and apps | Links und Apps |
| `desktop.infra_group_add` | Add | Hinzufügen |
| `desktop.infra_group_changed` | {count} changed | {count} geändert |
| `desktop.infra_field_changed` | Changed | Geändert |
| `desktop.infra_unsaved_changes` | Unsaved changes | Ungespeicherte Änderungen |
| `desktop.infra_save_in_infra` | Save in Infra | In Infra speichern |
| `desktop.infra_link_app` | Link an app… | App verknüpfen… |
| `desktop.infra_link_app_search` | Search apps | Apps suchen |
| `desktop.infra_picker_mesh_hidden` | Type at least two characters to search {count} MeshCore conversations. | Gib mindestens zwei Zeichen ein, um {count} MeshCore-Unterhaltungen zu durchsuchen. |
| `desktop.infra_remove_app_link` | Remove link to {name} | Verknüpfung mit {name} entfernen |
| `desktop.infra_no_connections` | No connections yet | Noch keine Verbindungen |
| `desktop.infra_add_connection` | Add connection | Verbindung hinzufügen |
| `desktop.infra_connect_to` | Connect to… | Verbinden mit… |
| `desktop.infra_connection_saved` | Connection saved | Verbindung gespeichert |
| `desktop.infra_connection_readonly` | Reported by a source and read-only | Von einer Quelle gemeldet und schreibgeschützt |
| `desktop.infra_remove_ignore_confirm` | Remove {name} from Infra and ignore its sources? It comes back as a discovery only after you restore it under Ignored. | {name} aus Infra entfernen und seine Quellen ignorieren? Es erscheint erst wieder als Fund, wenn du es unter Ignoriert wiederherstellst. |
| `desktop.infra_remove_ignore_confirm_many` | Remove {count} assets from Infra and ignore their sources? | {count} Assets aus Infra entfernen und ihre Quellen ignorieren? |

---

## Task 9: Inspector forms module (created, wired in Task 10)

**Files:** create `ui/js/desktop/apps/infra-inspector-forms.js`; register it (loader + test list) after `infra-cleanup.js`. Copying code is not an edit; no impact step (record that).

- [ ] **Step 1: Create the module** `InfraAppInspectorForms.create(app)` with `const { store, state, t, esc, model } = app; const S = app.safeId;` and constants copied from Plan B's inspector: `KINDS`, `SOURCE_KINDS`, `SOURCE_EDIT_FIELDS`, `profileFields` is not needed.
- [ ] **Step 2: Move verbatim from `infra-inspector.js`** (the Plan B port): `reviewValue`, `nodeReviewRows`, `reviewDiffMarkup`, `sourceReviewMarkup`, `nodeReviewMarkup`, `inputFor`, `listEditor`, `customEditor`, `sourceFieldMarkup`, `sourceMarkup`, `validateDetailForm`, `readProfileLists`, `saveNode`, `loadSourceEdit`, `sourceEditPending`, `sourceFieldValue`, `reapplySourceDraft`, `reapplyNodeDraft`, `reviewSourceEdit`, `saveSourceEdit`, the list add/remove handler and the `updateSourceDraft` closure. Edits while copying:
  - `listEditor` root gets `data-field="profile.{family}"`; `customEditor` root gets `data-field="profile.custom"`.
  - `readProfileLists`: delete the `draft.profile.app_bindings = …[data-app-target]:checked…` line (bindings live in the draft).
  - `saveNode`: after the success branch and the conflict/uncertain branches call `app.inspector.paint()`.
  - `loadSourceEdit`: before painting call `app.inspector.showTab('sources')`, then `app.inspector.paint()`.
  - Everything that read `q('[data-inspector]')` reads `app.inspector.el`.
- [ ] **Step 3: New code** (contracts; implement exactly):
  - `groupOpen = new Map()` keyed `` `${nodeId}:${group}` ``; `onToggle(event)` (inspector binds `toggle` in capture) stores `event.target.open` for `[data-form-group]`.
  - `detailsHTML(node, draft)` returns `<form class="infra-detail-form" id="infra-detail-form-${S}" data-detail-form>` + six `<details class="infra-form-group" data-form-group="{identity|network|hardware|links|notes|custom}" {open}>` groups + save bar or `<p class="infra-readonly-note">{t read_only}</p>` + `</form>`. A group is open when `groupOpen` says so, else when `model.formGroupFill(draft)[group]`. Summary: `<summary class="infra-form-group-head"><span class="infra-form-group-title">{title}</span>[empty && canWrite: <span class="infra-form-group-add" aria-hidden="true">{glyph plus}{t desktop.infra_group_add}</span>]<span class="infra-form-group-changed" data-group-changed hidden></span></summary><div class="infra-form-group-body">…</div>`. Titles: `t('desktop.infra_identity')`, `t('desktop.infra_group_network')`, `t('desktop.infra_group_hardware')`, `t('desktop.infra_group_links')`, `t('desktop.infra_notes')`, `t('desktop.infra_custom_fields')`. Bodies:
    - identity: kind select `[data-node-field="kind"]` and icon select `[data-node-field="icon"]` in `.infra-field-row`; place select `[data-node-field="parent_id"]` labelled `t('desktop.infra_place')` with `<option value="">{t desktop.infra_no_place}</option>` and every place except the node and its descendants, label = `placePath(includeSelf).join(' › ')`; `role` input; tags input (comma list, placeholder `t('desktop.infra_tags_hint')`). Each field label has `data-field="kind|icon|parent_id|profile.role|profile.tags"`.
    - network: `ip_address`, `mac_address` in `.infra-fields-grid`; for `kind === 'network'` the ported CIDR/VLAN segment (labels get `data-field="profile.cidr|profile.vlan"`); interfaces list editor.
    - hardware: `vendor, model, serial, os, firmware, cpu, ram, storage, rack_position` in `.infra-fields-grid`, each `<label class="infra-field" data-field="profile.{key}"><span>{t desktop.infra_field_<key>}</span><input data-profile="{key}" …></label>`.
    - links: links list editor + `appLinksHTML(node, draft)` (below).
    - notes: `<label class="infra-field" data-field="profile.notes"><span>{t desktop.infra_notes}</span><textarea data-profile="notes" rows="4">…</textarea></label>`.
    - custom: `customEditor`.
    - Save bar (canWrite only): `<div class="infra-savebar" data-savebar {hidden unless dirty}>[dirty: <p class="infra-draft-banner" role="status"><i aria-hidden="true"></i><span>{t desktop.infra_unsaved_changes}</span></p>]<span class="infra-flex-spacer"></span><button type="button" class="infra-button" data-action="revert-node">{t desktop.infra_discard}</button><button type="submit" class="infra-button infra-button-primary" {disabled unless dirty && !busy}>{busy ? t saving : t desktop.infra_save_in_infra}</button></div>`.
  - `appLinksHTML(node, draft)`: `<div class="infra-list-editor" data-list-editor="app-bindings" data-field="profile.app_bindings"><div class="infra-list-heading"><span>{t desktop.infra_application_links}</span></div><ul class="infra-app-chips" data-app-chips>{chipsHTML(draft)}</ul>{loading/empty/error hint as ported}{targetsError: reload-targets button}{canWrite && targets: <button type="button" class="infra-button" data-action="link-app">{glyph plus}<span>{t desktop.infra_link_app}</span></button><div class="infra-app-picker" data-app-picker hidden></div>}</div>`. Chip: `<li class="infra-app-chip[ is-unavailable]" data-app-binding="{app}:{target_id}"><span class="infra-app-chip-text"><strong>{label or target_id · t target_unavailable}</strong><small>{t desktop.infra_app_<app>}</small></span>[available: <button type="button" data-open-binding="{app}" data-target-id="{id}">{t desktop.infra_open}</button>][canWrite: <button type="button" data-remove-binding="{app}" data-target-id="{id}" aria-label="{fill(t remove_app_link,{name: label})}" title="…">{glyph close}</button>]</li>`.
  - `openAppPicker()` (on `[data-action="link-app"]`): open the enclosing `<details>` and record `groupOpen`; unhide `[data-app-picker]`; create once per paint `inline = app.picker.attachInline(box, { label: t('desktop.infra_link_app'), placeholder: t('desktop.infra_link_app_search'), source: appSource, onPick: option => addBinding(option.attrs['data-app'], option.attrs['data-target-id']) })`; `inline.focus()`. `appSource(query)` maps `model.appTargetOptions({ ...node, name: draft.name, icon: draft.icon, profile: draft.profile }, state.appTargets, query)` to picker groups (label `t('desktop.infra_picker_suggested')` or `t(\`desktop.infra_app_${id.replaceAll('-', '_')}\`)`), options `{ id: app:target, label, meta: app label, attrs: { 'data-app-target': '', 'data-app': app, 'data-target-id': target_id } }`, `note = hiddenMesh ? model.fill(t('desktop.infra_picker_mesh_hidden'), { count: hiddenMesh }) : ''`. `addBinding` appends `{app, target_id}` to `draft.profile.app_bindings` if absent, `markDirty`, re-renders `[data-app-chips]`, `inline.refresh({ clear: true })`. `[data-remove-binding]` removes it, `markDirty`, re-renders chips, focuses the next chip's remove button or `[data-action="link-app"]`. `[data-open-binding]` → `app.openAppLink({ app, target_id })`.
  - `onFieldEvent(event)`: ignore when `!store.canWrite()`, when the target has `data-source-field`, or has none of `data-node-field|data-profile|data-list-family`. Update the draft exactly like the ported `updateDraft` (tags split by comma, `vlan` number or 0, `readProfileLists(form, draft, id)`), then `markDirty(id)`; kind change → `app.inspector.paint()`; name/icon → `app.inspector.syncHead(draft)`.
  - `markDirty(id)`: `state.dirty.add(id)`, bump `state.draftRevision`, `syncDirty()`.
  - `syncDirty()` (no repaint): savebar `hidden = !dirty`; insert the banner once when dirty, remove it when clean; submit `disabled = !dirty || busy || !validateDetailForm(form, draft, id)`; Details tab `.infra-tab-dirty` and `[data-dirty-label]` `hidden = !dirty`; for every `[data-field]` set/remove `data-dirty="true"` from `model.draftDiff(node, draft)` and add/remove `<span class="infra-sr-only" data-dirty-text> · {t desktop.infra_field_changed}</span>` inside the label's first `span`; each `[data-form-group]` `[data-group-changed]` shows `fill(t('desktop.infra_group_changed'), {count})` (counts via `model.groupOfField`), hidden at 0.
  - `submitDetails()`: if the form is invalid (`[...form.elements].find(el => !el.checkValidity())`), `app.inspector.showTab('details')`, open the field's `<details>`, call `app.inspector.startRename()` when it is the header name input, then `reportValidity()`; else `form.requestSubmit()`.
  - `onSubmit(event)` (returns true when handled): `[data-detail-form]` → `preventDefault`, `saveNode(id)`; `[data-source-form]` → `preventDefault`, `saveSourceEdit(id)`.
  - `sourcesHTML(node)`: `<section class="infra-sources-panel">{sources ? <div class="infra-source-list">{sourceMarkup per source}</div> : <p class="infra-hint">{t manual_only}</p>}{sourceEdit: <form class="infra-source-edit infra-source-form" data-source-form>{heading, draft banners, sourceReviewMarkup, fields, reason, blocked banner as ported}<div class="infra-form-actions"><button type="button" class="infra-button" data-action="close-source-edit">{t cancel}</button>{review button}<button type="submit" class="infra-button infra-button-primary" data-action="save-source" {disabled as ported}>…</button></div></form>}</section>`. No click listener on save-source any more (submit only).
  - `onClick(event)` (returns true when handled): `[data-list-add]`/`[data-list-remove]` (ported handler; additionally set `groupOpen` true for that group), `revert-node` (ported), `link-app`, chip buttons, `[data-edit-source]` → `loadSourceEdit`, `[data-source-action]` → `app.dialogs.openActionMenu(node.id, source)` by `data-source-identity`, `close-source-edit`, `review-source`, `reapply-source-draft`, `reapply-node-draft`, `reload-targets` → `store.loadTargets(true)`.
  - `onSourceFieldEvent(event)` = ported `updateSourceDraft` for `[data-source-field]`.
  - `openIconPicker(button)`: popover `<div class="infra-popover infra-icon-picker" data-icon-picker role="dialog" aria-label="{t desktop.infra_change_icon}"><input type="search" data-icon-search aria-label="{t desktop.infra_icon_search}" placeholder="…" autocomplete="off"><div class="infra-icon-grid" role="listbox" aria-label="{t change_icon}" data-icon-grid>{per icon: <button type="button" class="infra-icon-option" role="option" data-pick-icon="{id}" aria-selected="{current}" tabindex="{0 current, else -1}" title="{label}" aria-label="{label}">{app.icon(id)}</button>}</div></div>` in `[data-layer]`, placed under the button like the picker. Keys in the grid: ArrowLeft/Right ±1, ArrowUp/Down ±8 over visible options (roving tabindex), Home/End, Enter/Space pick, Escape close and focus the button; ArrowDown in the search focuses the first visible option; search hides options whose label/id do not contain the text; outside pointerdown closes. Pick → `draft.icon = id`, sync `[data-node-field="icon"]`, `markDirty`, `app.inspector.syncHead(draft)`, close, focus the button.
  - `afterPaint(node)`: destroy the previous inline picker; `syncDirty()`. `detach()`: destroy inline picker, close icon picker. `isBusy()`: inline listbox open or icon picker open. `onBusy(key)`: ported source-save and `node-*` submit updates (submit text `t('desktop.infra_save_in_infra')`).
- [ ] **Step 4: CSS** (append):
```css
.infra-form-group { border: 1px solid var(--vd-theme-border); border-radius: var(--ds-radius-md); background: var(--vd-theme-panel-bg-strong); }
.infra-form-group + .infra-form-group { margin-top: var(--ds-space-2); }
.infra-form-group-head { display: flex; align-items: center; gap: var(--ds-space-2); min-height: 40px; padding: 0 var(--ds-space-3); font-size: 14px; font-weight: 600; list-style: none; cursor: pointer; }
.infra-form-group-head::-webkit-details-marker { display: none; }
.infra-form-group-head::before { content: ''; width: 7px; height: 7px; border-right: 2px solid var(--vd-theme-muted); border-bottom: 2px solid var(--vd-theme-muted); transform: rotate(-45deg); }
.infra-form-group[open] > .infra-form-group-head::before { transform: rotate(45deg); }
.infra-form-group-title { flex: 1; }
.infra-form-group-add { display: inline-flex; align-items: center; gap: 4px; color: var(--vd-accent); }
.infra-form-group-changed { color: var(--ds-color-warning-fg); font-size: 13px; }
.infra-form-group-body { display: grid; gap: var(--ds-space-2); padding: 0 var(--ds-space-3) var(--ds-space-3); }
.infra-field[data-dirty="true"] > span:first-child::before, .infra-list-editor[data-dirty="true"] .infra-list-heading > span:first-child::before { content: ''; display: inline-block; width: 6px; height: 6px; margin-right: 6px; border-radius: 50%; background: var(--ds-color-warning-fg); vertical-align: middle; }
.infra-field[data-dirty="true"] :is(input, select, textarea) { border-color: color-mix(in srgb, var(--ds-color-warning-fg) 60%, var(--vd-theme-border)); }
.infra-savebar { position: sticky; bottom: 0; z-index: 2; display: flex; align-items: center; gap: var(--ds-space-2); margin: var(--ds-space-3) calc(-1 * var(--ds-space-3)) calc(-1 * var(--ds-space-3)); padding: var(--ds-space-2) var(--ds-space-3); border-top: 1px solid var(--vd-theme-border); background: var(--vd-theme-panel-bg-strong); box-shadow: var(--vd-theme-shadow-panel); }
.infra-savebar .infra-draft-banner { margin: 0; }
.infra-app-chips { display: grid; gap: var(--ds-space-1); margin: 0; padding: 0; list-style: none; }
.infra-app-chip { display: grid; grid-template-columns: minmax(0, 1fr) auto auto; align-items: center; gap: var(--ds-space-1); min-height: 40px; padding: 2px 4px 2px var(--ds-space-2); border: 1px solid var(--vd-theme-border); border-radius: var(--ds-radius-full); background: var(--vd-theme-control-bg); }
.infra-app-chip.is-unavailable { border-style: dashed; }
.infra-app-chip-text { display: grid; min-width: 0; }
.infra-app-chip-text strong, .infra-app-chip-text small { overflow: hidden; font-size: 14px; text-overflow: ellipsis; white-space: nowrap; }
.infra-app-chip-text small { color: var(--vd-theme-muted); font-size: 13px; }
.infra-app-chip button { height: 30px; padding: 0 var(--ds-space-2); border: 0; border-radius: var(--ds-radius-full); background: transparent; font-size: 14px; cursor: pointer; }
.infra-app-chip button:hover { background: var(--vd-theme-control-hover); }
.infra-icon-picker { display: grid; gap: var(--ds-space-2); width: 340px; padding: var(--ds-space-2); }
.infra-icon-picker input { height: 34px; padding: 0 var(--ds-space-2); border: 1px solid var(--vd-theme-border); border-radius: var(--ds-radius-sm); background: var(--vd-theme-control-bg); color: var(--vd-text); font-size: 14px; }
.infra-icon-grid { display: grid; grid-template-columns: repeat(8, 36px); gap: 4px; max-height: 260px; overflow: auto; }
.infra-icon-option { width: 36px; height: 36px; display: grid; place-items: center; padding: 0; border: 1px solid transparent; border-radius: var(--ds-radius-sm); background: transparent; cursor: pointer; }
.infra-icon-option img { width: 30px; height: 30px; object-fit: contain; }
.infra-icon-option:hover { background: var(--vd-theme-control-hover); }
.infra-icon-option[aria-selected="true"] { border-color: var(--vd-accent); background: var(--vd-theme-accent-soft); }
```
- [ ] **Step 5: Verify** `node --check`, V-core (keys of batches F/G only).
- [ ] **Step 6: Commit** paths `ui/js/desktop/apps/infra-inspector-forms.js ui/js/desktop/core/module-loader.js scripts/test-infra-ui.mjs ui/css/desktop-app-infra.css`; subject `feat(infra): add grouped details form, save bar, app link picker and source form module`.

---

## Task 10: Inspector shell (header, tabs, overview, connections, edge and selection inspectors)

**Files:** `infra-inspector.js`, `infra-dialogs.js`, `infra.js`, `ui/css/desktop-app-infra.css`, `ui/desktop_infra_browser_test.go`.

- [ ] **Step 1: Impact** for inspector `paint`, `close`, `saveDraft`, `focusName`, `onBusy`, `paintIfIdle`; dialogs `openEdgeDialog`, `deleteNode`, `deleteNodes`; `git grep -n -e "inspector\." -e "openEdgeDialog" -e "deleteNode" -- ui/js/desktop/apps`.
- [ ] **Step 2: Rewrite inspector rendering.** Delete the code moved in Task 9 and import it via `const forms = app.forms` (infra.js creates `app.forms = window.InfraAppInspectorForms.create(app)` before the inspector). State: `TABS = ['overview','details','sources','connections']`, `PREFS_KEY = 'infra.inspector.v1'`, `prefs = model.readInspectorPrefs(app.readStorage(PREFS_KEY))`, `activeTab`, `pendingTab`, `lastId`, `renaming`, `renameBefore`, `edgeDirty`.
  - `paint()`: node → `paintNode`; `state.selectedEdge` → `paintEdge`; `selection.size > 1` → `paintSelection`; else `forms.detach()`, hide/empty panel, `inspectorOpen=false`, `lastId=''`.
  - `paintNode(node)`: draft as before; new node → `activeTab = pendingTab || model.inspectorTab(prefs, node.kind)`, same node with `pendingTab` → that tab; clear `pendingTab`. Markup:
```html
<div class="infra-inspector-top">
  <header class="infra-inspector-head" data-inspector-head>
    <button type="button" class="infra-head-icon" data-action="pick-icon" aria-haspopup="dialog" aria-label="{t desktop.infra_change_icon}" title="…" {disabled !canWrite}>{icon(draft.icon)}</button>
    <div class="infra-head-main">
      <h2 class="infra-head-title" id="infra-inspector-title-${S}">{canWrite ? <button type="button" class="infra-head-name" data-action="rename" title="{t desktop.infra_rename}">{draft.name}</button> : <span class="infra-head-name">{node.name}</span>}</h2>
      <input class="infra-head-name-input" data-node-field="name" form="infra-detail-form-${S}" value="{draft.name}" maxlength="120" required autocomplete="off" aria-label="{t desktop.infra_name}" hidden {readonly !canWrite}>
      <p class="infra-head-meta"><span>{kind label}</span>{crumbs: <span aria-hidden="true">·</span><nav class="infra-crumbs" aria-label="{t desktop.infra_place}"><ol>{<li><button type="button" data-crumb="{id}">{name}</button></li> per placePath}</ol></nav>}</p>
      <p class="infra-head-pills"><span class="infra-state-pill" data-state="{state}"><i></i>{status}</span>{power pill}{first app link ? <button type="button" class="infra-button infra-button-primary" data-action="open-first-app" data-open-app="{app}" data-target-id="{id}">{fill(t open_in_app,{app: label})}</button> : <button type="button" class="infra-button" data-action="edit-details">{t desktop.infra_edit}</button>}</p>
    </div>
    <div class="infra-head-actions"><button type="button" class="infra-cmd-icon" data-action="node-more" aria-haspopup="menu" aria-label="{t desktop.infra_more_actions}" title="…">{glyph more}</button><button type="button" class="infra-cmd-icon infra-detail-close" data-action="close-inspector" aria-label="{t desktop.close}" title="…">{glyph close}</button></div>
  </header>
  <div class="infra-tabs" role="tablist" aria-label="{t desktop.infra_inspector_tabs}" data-inspector-tabs>
    <button type="button" role="tab" id="infra-tab-${S}-overview" data-inspector-tab="overview" aria-controls="infra-panel-${S}-overview" aria-selected="{bool}" tabindex="{0|-1}">{t desktop.infra_tab_overview}</button>
    <button … details>{t desktop.infra_tab_details}<i class="infra-tab-dirty" aria-hidden="true" {hidden unless dirty}></i><span class="infra-sr-only" data-dirty-label {hidden unless dirty}>, {t desktop.infra_unsaved_changes}</span></button>
    <button … sources>{t desktop.infra_tab_sources}<span class="infra-tab-count">{sources.length}</span></button>
    <button … connections>{t desktop.infra_tab_connections}<span class="infra-tab-count">{edges.length}</span></button>
  </div>
</div>
{forms.nodeReviewMarkup(node.id)}
<div class="infra-tabpanel" role="tabpanel" id="infra-panel-${S}-{tab}" aria-labelledby="infra-tab-${S}-{tab}" data-inspector-panel="{tab}" {hidden unless active}>…</div> ×4
```
  Panels: overview = `overviewHTML(node)`, details = `forms.detailsHTML(node, draft)`, sources = `forms.sourcesHTML(node)`, connections = `connectionsHTML(node)`. Then `el.hidden=false`, `inspectorOpen=true`, `aria-labelledby` to the title, `forms.afterPaint(node)`.
  - `overviewHTML(node)`: `<dl class="infra-facts">` with `t('desktop.infra_address')` (ip or —), `t('desktop.infra_field_mac_address')`, `t('desktop.infra_fact_hardware')` (vendor · model), `t('desktop.infra_place')` (path or `t('desktop.infra_no_place')`), `t('desktop.infra_placement')` (`t('desktop.infra_position')` with x/y or `t('desktop.infra_unplaced')`), host (`t('desktop.infra_fact_host')`: `<button data-inspector-select="{host}">{name}</button>` + `' · ' + t('desktop.infra_host_inferred')` when `node.host_inferred`), guests (`t('desktop.infra_fact_guests')`: `<ul class="infra-overview-guests">` first 8 `[data-inspector-select]` buttons + `[data-guest-more="{id}"]` with `fill(t desktop.infra_more_guests,{count})`), `t('desktop.infra_tab_connections')` (`fill(t connections_summary)`), `t('desktop.infra_fact_last_seen')` (max `fetched_at` → `toLocaleString()`, else `t('desktop.infra_no_observation')`). Below: `t('desktop.infra_fact_apps')` heading with the ported `[data-open-app]` buttons when `app_links` exist; the Plan B `[data-action="place-selected"]` button (unplaced, not a guest, canWrite) and `<p class="infra-hint">{t desktop.infra_move_hint}</p>`; the ported service-components block for `kind === 'service'`.
  - `connectionsHTML(node)`: heading `<div class="infra-form-heading"><h3>{t tab_connections} <span>{n}</span></h3>{canWrite: <button type="button" class="infra-button" data-action="add-edge" aria-haspopup="dialog">{glyph plus}{t desktop.infra_add_connection}</button>}</div>`; rows `<div class="infra-relation-row" data-edge-row="{id}"><span class="infra-edge-kind" data-edge-kind="{kind}" aria-hidden="true"></span><span>{t edge_kind}</span><button type="button" class="infra-relation-other" data-inspector-select="{other}">{other name}</button><small>{model.edgeCaption(edge)}</small><button type="button" data-edit-edge="{id}" aria-label="{t edit_connection}" title="…" {disabled !canWrite || edge.source}>✎</button><button type="button" data-remove-edge="{id}" aria-label="{t remove_connection}" title="…" {disabled same}>{glyph close}</button></div>`; empty `<p class="infra-hint">{t desktop.infra_no_connections}</p>`.
  - `paintEdge(edge)`: head like Plan B (connection glyph, `{t connection} · {edge kind}`, `<h2 id=…>{from} → {to}</h2>`, close); body `<form class="infra-edge-inline" data-edge-inline-form="{id}">` with `<dl class="infra-facts">` from/to `[data-inspector-select]` buttons; when `edge.source || !canWrite`: facts for kind/ports/speed/label and (source only) `<p class="infra-hint">{t desktop.infra_connection_readonly}</p>`; otherwise kind select (`EDGE_KINDS`, label `t('desktop.infra_connection_kind')`), from_port/to_port row, speed/label row, `<p class="infra-connect-error" data-edge-error role="alert" hidden></p>`, actions `[data-action="delete-edge"]` (danger) and `<button type="submit" class="infra-button infra-button-primary" data-action="save-edge" disabled>{t desktop.save}</button>`. Input enables save and sets `edgeDirty`; submit: duplicate `(from,to,kind)` among other edges → inline `t('desktop.infra_error_edge_exists')`; else `store.updateEdge(id, { id, version, from, to, kind, …non-empty label/from_port/to_port/speed })` → announce `t('desktop.infra_connection_saved')`, `edgeDirty=false`; errors inline with `store.messageFor`. `focusEdgeForm()` focuses the kind select.
  - `paintSelection()`: head `<h2 id=…>{fill(t selected_count)}</h2>` + close; `<ul class="infra-selection-kinds">` (kind label + `<b>count</b>`), `<ul class="infra-selection-names">` first 8 `[data-inspector-select]` buttons, `+N` hint; canWrite actions: `[data-action="move-selection"]` (`t move_to_place`, `aria-haspopup="dialog"`), `[data-action="arrange-selection"]` (disabled in table view), `[data-action="delete-selection"]` (`t remove_node`), `[data-action="ignore-selection"]` (`t remove_and_ignore`).
  - `showTab(tab, {focus, persist})`: update `aria-selected`, `tabindex`, panel `hidden`; `persist` writes `model.withInspectorTab(prefs, node.kind, tab)` to `PREFS_KEY`. `open(id, tab='')`: if already the single selection, `showTab(tab)`; else `pendingTab = tab; store.select([id])`. `syncHead(draft)`: update rename button text and head icon `src`.
  - Rename: `startRename()` hides the button, shows and selects the input (stores `renameBefore`); keydown on the input: Enter → `preventDefault`, end; Escape → `stopPropagation`, restore `renameBefore` and dispatch `input`, end; focusout from the input ends without focusing. `focusName()` = `showTab('details')` + `startRename()`.
  - Delegated listeners on `el` (bound once): `click` → `forms.onClick(event)` first, then: `[data-inspector-tab]` (persist), `rename`, `pick-icon` → `forms.openIconPicker`, `[data-crumb]` and `[data-inspector-select]` → `store.select([id])` + `app.canvas.ensureVisible(id)`, `[data-open-app]` → `app.openAppLink`, `edit-details` → Details tab + focus kind select, `node-more` → `app.menus ? app.menus.openNodeMenu(id, button) : app.dialogs.openNodeActions(id)`, `close-inspector`, `[data-guest-more]` → `app.library.reveal`, `place-selected`, `add-edge` → `app.picker.openAssetPicker({ anchor: button, label: t('desktop.infra_connect_to'), accept: other => other.id !== id, onPick: option => app.dialogs.openEdgeDialog('', id, option.id) })`, `[data-edit-edge]` → `openEdgeDialog(edgeId)`, `[data-remove-edge]`/`delete-edge` → `app.dialogs.removeEdge`, `move-selection` → `app.picker.openPlacePicker({ anchor: button, exclude: ids, onPick: placeId => app.gestures.moveIntoPlace(ids, placeId) })`, `arrange-selection`, `delete-selection` → `deleteNodes`, `ignore-selection` → `removeAndIgnore`. `input`/`change` → `forms.onFieldEvent` + `forms.onSourceFieldEvent` + edge form. `submit` → `forms.onSubmit` else edge form. `keydown` → tab keys (`ArrowRight/ArrowLeft` wrap, `Home`, `End`, all with `preventDefault` and `showTab(…, {focus:true, persist:true})`) and rename keys. `toggle` (capture) → `forms.onToggle`.
  - Busy: `isBusy()` = dirty/source-dirty for the selected node, focus inside `el`, `app.picker.isOpen()`, `forms.isBusy()` or `edgeDirty`. `paintIfIdle`/`flushStale` use it.
  - `saveDraft()` (Ctrl S): Sources tab with a source draft → `el.querySelector('[data-source-form]')?.requestSubmit()`; else if the node is dirty → `forms.submitDetails()`.
  - API returns `{ el, paint, paintIfIdle, flushStale, onBusy, saveDraft, focusName, startRename, close, open, showTab, syncHead, focusEdgeForm, dispose }`; `onBusy` also forwards to `forms.onBusy`; `dispose` calls `forms.dispose()`.
- [ ] **Step 3: Dialogs** (`infra-dialogs.js`):
  - `openEdgeDialog(edgeId = '', fromId = state.selected, toId = '')`: new edge without `toId` → `app.picker.openAssetPicker({ anchor: document.activeElement, label: t('desktop.infra_connect_to'), accept: node => node.id !== fromId, onPick: option => openEdgeDialog('', fromId, option.id) })` and return. In the form markup replace both endpoint `<select>`s with `<input type="hidden" name="from" value="{from}"><input type="hidden" name="to" value="{to}"><p class="infra-connect-ends">{from name} → {to name}</p>`; kind default `edge?.kind || model.defaultEdgeKind(fromNode, toNode)`.
  - Replace `deleteNode`/`deleteNodes` and add `removeAndIgnore`, `removeEdge`:
```js
        async function deleteNodes(ids) {
            const list = (ids || []).filter(id => store.nodeById(id));
            if (!list.length || !store.canWrite()) return false;
            const message = list.length === 1 ? model.fill(t('desktop.infra_remove_node_confirm'), { name: store.nodeById(list[0]).name }) : model.fill(t('desktop.infra_remove_selection_confirm'), { count: list.length });
            if (!await confirmAction(t('desktop.infra_remove_node'), message, { confirmLabel: t('desktop.infra_remove_node') })) return false;
            const removed = await store.removeNodes(list);
            if (removed.length) app.announce(removed.length === 1 ? t('desktop.infra_node_removed') : model.fill(t('desktop.infra_removed_many'), { count: removed.length }));
            return removed.length === list.length;
        }
        const deleteNode = id => deleteNodes([id]);
        async function removeAndIgnore(ids) {
            const list = (ids || []).filter(id => store.nodeById(id));
            if (!list.length || !store.canWrite()) return false;
            const message = list.length === 1 ? model.fill(t('desktop.infra_remove_ignore_confirm'), { name: store.nodeById(list[0]).name }) : model.fill(t('desktop.infra_remove_ignore_confirm_many'), { count: list.length });
            if (!await confirmAction(t('desktop.infra_remove_and_ignore'), message, { confirmLabel: t('desktop.infra_remove_and_ignore') })) return false;
            const removed = await store.removeNodes(list, { ignoreSources: true });
            if (removed.length) app.announce(model.fill(t('desktop.infra_removed_ignored'), { count: removed.length }));
            return removed.length === list.length;
        }
        async function removeEdge(edgeId) {
            const edge = state.edges.find(item => item.id === edgeId);
            if (!edge || edge.source || !store.canWrite()) return false;
            if (!await confirmAction(t('desktop.infra_remove_connection'), t('desktop.infra_remove_relation_confirm'), { confirmLabel: t('desktop.infra_remove_connection') })) return false;
            try { const ok = await store.dispatch(window.InfraAppStore.commands.deleteEdge(edge)); if (ok) app.announce(t('desktop.infra_connection_removed')); return ok; }
            catch (error) { store.notify(store.messageFor(error), 'error'); return false; }
        }
```
- [ ] **Step 4: CSS** (append):
```css
.infra-inspector-top { position: sticky; top: 0; z-index: 3; border-bottom: 1px solid var(--vd-theme-border); background: var(--vd-theme-panel-bg); }
.infra-inspector-head { display: grid; grid-template-columns: 44px minmax(0, 1fr) auto; align-items: start; gap: var(--ds-space-2); padding: var(--ds-space-3) var(--ds-space-3) var(--ds-space-2); }
.infra-head-icon { width: 44px; height: 44px; display: grid; place-items: center; padding: 0; border: 1px solid var(--vd-theme-border); border-radius: var(--ds-radius-md); background: var(--vd-theme-control-bg); cursor: pointer; }
.infra-head-icon img { width: 36px; height: 36px; object-fit: contain; }
.infra-head-main { display: grid; gap: 2px; min-width: 0; }
.infra-head-title { margin: 0; min-width: 0; font-size: 18px; font-weight: 650; line-height: 1.3; }
.infra-head-name { display: block; max-width: 100%; overflow: hidden; margin: 0 -4px; padding: 0 4px; border: 1px solid transparent; border-radius: var(--ds-radius-sm); background: transparent; font: inherit; text-align: left; text-overflow: ellipsis; white-space: nowrap; }
button.infra-head-name { cursor: text; }
button.infra-head-name:hover { border-color: var(--vd-theme-border); }
.infra-head-name-input { width: 100%; height: 32px; padding: 0 var(--ds-space-2); border: 1px solid var(--ds-color-border-focus); border-radius: var(--ds-radius-sm); background: var(--vd-theme-control-bg); color: var(--vd-text); font-size: 16px; font-weight: 600; }
.infra-head-meta { display: flex; flex-wrap: wrap; align-items: center; gap: 4px; margin: 0; color: var(--vd-theme-muted); font-size: 14px; }
.infra-crumbs ol { display: flex; flex-wrap: wrap; margin: 0; padding: 0; list-style: none; }
.infra-crumbs li + li::before { content: '›'; margin: 0 4px; }
.infra-crumbs button { padding: 0; border: 0; background: transparent; color: var(--vd-accent); font-size: 14px; cursor: pointer; }
.infra-head-pills { display: flex; flex-wrap: wrap; align-items: center; gap: var(--ds-space-1); margin: 2px 0 0; }
.infra-head-pills .infra-button { height: 30px; margin-left: auto; }
.infra-head-actions { display: flex; gap: 2px; }
.infra-tabs { display: flex; gap: 2px; padding: 0 var(--ds-space-2); overflow-x: auto; scrollbar-width: none; }
.infra-tabs [role="tab"] { display: inline-flex; flex: 0 0 auto; align-items: center; gap: 6px; height: 36px; padding: 0 var(--ds-space-2); border: 0; border-bottom: 2px solid transparent; background: transparent; color: var(--vd-theme-muted); font-size: 14px; font-weight: 600; cursor: pointer; }
.infra-tabs [role="tab"][aria-selected="true"] { border-bottom-color: var(--vd-accent); color: var(--vd-text); }
.infra-tab-count { padding: 1px 6px; border-radius: var(--ds-radius-full); background: var(--vd-theme-control-bg); font-size: 12px; font-variant-numeric: tabular-nums; }
.infra-tab-dirty { width: 8px; height: 8px; border-radius: 50%; background: var(--ds-color-warning-fg); }
.infra-tabpanel { display: grid; gap: var(--ds-space-3); padding: var(--ds-space-3); }
.infra-selection-kinds, .infra-selection-names, .infra-overview-guests { display: flex; flex-wrap: wrap; gap: var(--ds-space-1); margin: 0; padding: 0; list-style: none; }
.infra-selection-kinds li { padding: 2px var(--ds-space-2); border-radius: var(--ds-radius-full); background: var(--vd-theme-control-bg); font-size: 14px; }
.infra-selection-names button, .infra-overview-guests button, .infra-facts dd button, .infra-relation-other { display: inline-flex; align-items: center; gap: 6px; max-width: 100%; min-height: 30px; padding: 0 var(--ds-space-2); border: 1px solid var(--vd-theme-border); border-radius: var(--ds-radius-full); background: var(--vd-theme-control-bg); font-size: 14px; cursor: pointer; }
.infra-selection-names img, .infra-overview-guests img { width: 20px; height: 20px; object-fit: contain; }
.infra-selection-actions, .infra-edge-inline { display: flex; flex-wrap: wrap; gap: var(--ds-space-2); }
.infra-edge-inline { display: grid; }
```
- [ ] **Step 5: Main browser test updates (conscious):**
  - App binding (old lines 400–402): replace the checkbox lines with
```go
	page.MustWait(`()=>!!document.querySelector('[data-action="link-app"]')`)
	page.MustEval(`()=>document.querySelector('[data-action="link-app"]').click()`)
	page.MustWait(`()=>!!document.querySelector('[data-app-picker] [data-app-target][data-app="software-store"]')`)
	page.MustEval(`()=>document.querySelector('[data-app-picker] [data-app-target][data-app="software-store"]').click()`)
	page.MustWait(`()=>!!document.querySelector('[data-app-binding="software-store:net-tools"]')`)
	page.MustEval(`()=>{
		const iface=document.querySelector('[data-list-add="interfaces"]');iface.click();
		const name=document.querySelector('[data-list-row="interfaces"] [data-key="name"]');name.value='eth0';name.dispatchEvent(new Event('input',{bubbles:true}));
		const address=document.querySelector('[data-list-row="interfaces"] [data-key="ip_address"]');address.value='192.0.2.9';address.dispatchEvent(new Event('input',{bubbles:true}));
		document.querySelector('[data-detail-form] button[type="submit"]').click();
	}`)
```
  - Relation (old lines 409–412): replace with
```go
	page.MustEval(`()=>document.querySelector('[data-action="add-edge"]').click()`)
	page.MustWait(`()=>!!document.querySelector('[data-picker] [data-node-option="imported-2"]')`)
	page.MustEval(`()=>document.querySelector('[data-picker] [data-node-option="imported-2"]').click()`)
	page.MustWait(`()=>!!document.querySelector('[data-edge-form]')`)
	page.MustEval(`()=>{const form=document.querySelector('[data-edge-form]');if(form.elements.from.value!=='manual-1'||form.elements.to.value!=='imported-2')throw new Error('connection ends were not prefilled');form.elements.kind.value='depends_on';form.requestSubmit();}`)
```
- [ ] **Step 6: Verify** `node --check` (inspector, forms, dialogs, infra), V-core, V-browser (main, regression and canvas suites must pass; the regression suites run unchanged).
- [ ] **Step 7: Commit** paths `ui/js/desktop/apps/infra-inspector.js ui/js/desktop/apps/infra-inspector-forms.js ui/js/desktop/apps/infra-dialogs.js ui/js/desktop/apps/infra.js ui/css/desktop-app-infra.css ui/desktop_infra_browser_test.go`; subject `feat(infra): tabbed inspector with inline name, connection and selection inspectors`.

---

## Task 11: i18n batch H (import)

Same procedure (`$SCRATCH/infra-i18n-h.json`, `+18`). Subject `i18n(infra): add import sheet and relevance keys`.

| key | en | de |
|---|---|---|
| `desktop.infra_import_sheet_hint` | Drag a discovery onto the map to import and place it in one step. | Ziehe einen Fund auf die Karte, um ihn in einem Schritt zu importieren und zu platzieren. |
| `desktop.infra_import_recommended` | Recommended | Empfohlen |
| `desktop.infra_import_duplicates` | Possible duplicates | Mögliche Duplikate |
| `desktop.infra_import_unlikely` | Probably not homelab | Wahrscheinlich kein Homelab |
| `desktop.infra_import_one` | Import | Importieren |
| `desktop.infra_ignore` | Ignore | Ignorieren |
| `desktop.infra_ignore_selected` | Ignore selected | Auswahl ignorieren |
| `desktop.infra_ignored_count` | {count} discoveries ignored | {count} Funde ignoriert |
| `desktop.infra_select_group` | Select all in {group} | Alle in {group} auswählen |
| `desktop.infra_import_search` | Filter discoveries | Funde filtern |
| `desktop.infra_relevance_duplicate_ip` | Same address as {name} | Gleiche Adresse wie {name} |
| `desktop.infra_relevance_duplicate_mac` | Same MAC address as {name} | Gleiche MAC-Adresse wie {name} |
| `desktop.infra_relevance_generic_discovery` | Generic network scan result without vendor or model | Allgemeiner Netzwerkscan-Fund ohne Hersteller oder Modell |
| `desktop.infra_relevance_inactive` | Not active right now | Gerade nicht aktiv |
| `desktop.infra_relevance_link_local` | Only a link-local IPv6 address | Nur eine link-lokale IPv6-Adresse |
| `desktop.infra_link_suggested` | Suggested assets | Vorgeschlagene Assets |
| `desktop.infra_drop_guest` | Imported into its host | Wird in seinen Host importiert |
| `desktop.infra_import_partial` | {count} discoveries could not be imported | {count} Funde konnten nicht importiert werden |

---

## Task 12: Import sheet, discovery drag, batch fixture

**Files:** create `ui/js/desktop/apps/infra-import.js`; modify `infra-dialogs.js` (remove `loadCandidates`, `paintImportDialog`, `updateImportCount`, `importSelected`, `openImport` and their `onBusy('import')` branch), `infra-gestures.js`, `infra-canvas.js`, `infra.js`, loader, test list, CSS, `ui/desktop_infra_browser_test.go`.

- [ ] **Step 1: Impact** for dialogs `openImport`/`importSelected`, canvas `createGhost`, gestures `startLibraryDrag`/`finishLibraryDrop`, fixture `ServeHTTP` and `snapshot`; `git grep -n -e "openImport" -e "createGhost" -e "snapshot()" -- ui`.
- [ ] **Step 2: Canvas:** `createGhost(ids, items = null)` — content from `items[0]` (`{icon, name}`) and `+N` from `(items || ids).length` when given, else as Plan B.
- [ ] **Step 3: Gestures:** in `finishLibraryDrop` add first: `if (mode === 'discovery') return world && app.importer ? app.importer.dropOnCanvas(ids, world, view === 'atlas' ? region : '') : Promise.resolve(false);`; pass `items` to `canvas.createGhost(ids, items)`; ghost label for discovery = `t('desktop.infra_drop_guest')` when every dragged candidate is `vm`/`container`, else the Plan B labels.
- [ ] **Step 4: Create `infra-import.js`** (`InfraAppImport.create(app)`), `GROUPS = ['recommended', 'duplicate', 'unlikely']`. Markup (built once, `el.hidden = true`):
```html
<section class="infra-import-sheet" data-import-sheet role="dialog" aria-modal="false" aria-labelledby="infra-import-title-${S}" hidden>
  <header class="infra-sheet-head"><h2 id="infra-import-title-${S}">{t desktop.infra_import_title}</h2><button type="button" class="infra-cmd-icon" data-action="close-import" aria-label="{t desktop.close}" title="…">{glyph close}</button></header>
  <p class="infra-sheet-intro">{t desktop.infra_import_sheet_hint}</p>
  <div class="infra-candidate-source-tabs" data-candidate-sources></div>
  <div class="infra-import-search"><input type="search" data-import-search aria-label="{t desktop.infra_import_search}" placeholder="…" autocomplete="off"></div>
  <div class="infra-import-groups" data-import-groups>
    <p class="infra-import-loading" data-import-loading>{t desktop.infra_loading_candidates}</p>
    <div class="infra-empty-candidates" data-import-empty hidden><strong>{t desktop.infra_no_candidates}</strong><p>{t desktop.infra_no_candidates_hint}</p></div>
    {per group: <section class="infra-import-group" data-import-group="{g}" hidden><h3 class="infra-import-group-head"><button type="button" class="infra-library-toggle" data-import-group-toggle="{g}" aria-expanded="{g!=='unlikely'}" aria-controls="infra-import-${S}-{g}"><span class="infra-library-title">{title}</span><span class="infra-library-count" data-import-group-count="{g}">0</span></button><input type="checkbox" data-import-check-group="{g}" aria-label="{fill(t select_group,{group: title})}"></h3><ul class="infra-candidate-list" id="infra-import-${S}-{g}" data-import-list="{g}" {hidden for unlikely}></ul></section>}
  </div>
  <div class="infra-sheet-foot"><span data-import-count></span><button type="button" class="infra-button" data-action="ignore-selected" disabled>{t desktop.infra_ignore_selected}</button><button type="button" class="infra-button infra-button-primary" data-import disabled>{t desktop.infra_import_selected}</button></div>
</section>
```
  Titles: `t('desktop.infra_import_recommended')`, `t('desktop.infra_import_duplicates')`, `t('desktop.infra_import_unlikely')`. Row:
```html
<li class="infra-candidate" data-candidate-row="{id}" data-group="{group}" [data-draggable="true" when canWrite && features.layoutParent]>
  <label class="infra-candidate-check"><input type="checkbox" data-candidate="{id}" aria-label="{fill(t select_candidate,{name})}" {checked}><span class="infra-checkbox-mark"></span></label>
  <div class="infra-candidate-icon">{icon}</div>
  <div class="infra-candidate-copy"><strong>{name or t unnamed_candidate}</strong><span>{t desktop.infra_source_<kind>} · {source name|instance}</span><small>{vendor · model · ip}</small>[reasons: <small class="infra-candidate-reason" data-candidate-reason>{reason texts joined ' · '}</small>]</div>
  <label class="infra-field infra-bind-field"><span>{t desktop.infra_link_existing}</span><select data-bind="{id}"><option value="">{t desktop.infra_import_new}</option>[<optgroup label="{t desktop.infra_link_suggested}">{binding (if set) + model.linkSuggestions}</optgroup>]</select></label>
  <div class="infra-candidate-actions"><button type="button" class="infra-button" data-import-one="{id}">{t desktop.infra_import_one}</button><button type="button" class="infra-button" data-ignore-candidate="{id}">{t desktop.infra_ignore}</button></div>
  <p class="infra-candidate-error" data-candidate-error role="alert" {hidden unless error}>{message}</p>
</li>
```
  Reason texts: `duplicate_ip`/`duplicate_mac` → `fill(t('desktop.infra_relevance_duplicate_ip'|'…_mac'), { name })` (name of `duplicateOf.nodeId` node or `duplicateOf.candidateId` discovery), `generic_discovery`, `inactive`, `link_local` → their `desktop.infra_relevance_*` texts. Behaviour contracts:
  - `open()`: needs `canWrite`; remember `document.activeElement`; `state.importOpen = true`, `state.importDialogOwner = \`sheet-${++state.dialogGeneration}\``; clear candidates, selection, bindings, row errors, search; unhide; `root.dataset.importOpen='true'`; focus search; `load(false)`. `close()`: hide, reset `importOpen`/owner/candidates, focus the opener. Escape (keydown inside, `stopPropagation`) closes.
  - `load(preserve)`: `store.request('/candidates')`; ignore when the owner changed; keep `result.sources` locally for the source chips (never overwrite `state.sources`); groups from `model.groupDiscoveries(state.candidates, state.nodes, { sourceHosts: state.sourceHosts })` (memo by those array identities); initial selection = Recommended ids (or the preserved ids); initial bindings: for duplicates with `duplicateOf.nodeId` where `model.bindableFor(candidate, node)`.
  - `render()`: per group filter by search (name, ip, source name/kind); hide empty groups; counts; group checkbox `checked`/`indeterminate`; `[data-import-empty]` when not loading and nothing shown; `updateCount()` sets `[data-import-count]` (`fill(t selected_count)`) and disables `[data-import]`/`ignore-selected` when nothing selected, `!canWrite` or busy `import`/`ignore`. After a row disappears, focus the same control of the next row in its group, else the search field.
  - Change: `[data-candidate]` toggles `state.importSelection`; `[data-import-check-group]` selects/deselects the group's visible rows; `[data-bind]` sets/deletes `state.importBindings`. `focusin`/`pointerdown` on a `[data-bind]` without `data-full` appends `<optgroup label="{t desktop.infra_link_all}">` with all other bindable nodes sorted by name and sets `data-full="true"`. Search input re-renders once per animation frame.
  - `[data-import]`: items in display order `{candidate_id, node_id?}`; without `store.features.layoutParent` run the ported sequential `/import` loop; else `settle(await store.importBatch(items), owner, { closeWhenDone: true })`. `[data-import-one]` → batch of one. `settle`: drop selection/bindings of candidates that left; row errors from `result.errors`; `store.notify(fill(t imported,{count}), 'success')` when nodes were imported; `app.announce(fill(t('desktop.infra_import_partial'), {count}))` on errors; `uncertain` → `load(true)`; close when `closeWhenDone` and no errors; else `render()`. Import never changes the map selection.
  - `[data-ignore-candidate]`/`ignore-selected` → `store.setSourcesIgnored(sources, true)`, drop from selection, announce `fill(t('desktop.infra_ignored_count'), {count})`, render.
  - Pointerdown on `[data-candidate-row][data-draggable="true"]` (button 0, not on input/select/button/label): `ids = (event.ctrlKey || event.metaKey || event.shiftKey) && row checked ? checked visible ids : [id]`; `app.gestures.startLibraryDrag(event, ids, { mode: 'discovery', items: ids.map(…{icon, name}), onDragged: () => { suppressClick = true; setTimeout(() => { suppressClick = false; }, 0); } })`. Clicks are ignored while `suppressClick`.
  - `dropOnCanvas(ids, world, region)`: candidates of those ids; placeable = not `vm`/`container`; `points = app.gestures.positionsAt(placeable.length || 1, world)`; `parent = region && view === 'atlas' && store.features.layoutParent ? region : ''`; items `{candidate_id, node_id?, placement: {view, x, y}, parent_id?}` (no placement/parent for guests); `settle(result, owner)`; select the imported node ids and announce `fill(t imported,{count})`.
  - `reload()`: `if (state.importOpen) load(true)`. `onBusy(key)` for `import`/`ignore` → `updateCount()`. `dispose()` closes without focus.
- [ ] **Step 5: `infra.js`:** create `app.importer` after dialogs, append `importer.el` to `[data-body]`; route `import`, `import-empty` and the window-menu Import item to `importer.open()`; root click delegation also ignores clicks inside `[data-import-sheet]`; routing `candidates` → `importer.render()`, `sources`/`caps` also `importer.render()`, `busy` → `importer.onBusy(key)`; dispose calls `importer.dispose()`. Register `infra-import.js` after `infra-dialogs.js`.
- [ ] **Step 6: CSS:**
```css
.infra-import-sheet { position: absolute; top: 0; bottom: 0; left: 0; z-index: 9; display: flex; flex-direction: column; width: max(var(--infra-library-w), 380px); max-width: 100%; border-right: 1px solid var(--vd-theme-border); background: var(--vd-theme-panel-bg); box-shadow: var(--vd-theme-shadow-modal); }
.infra-sheet-head { display: flex; align-items: center; gap: var(--ds-space-2); padding: var(--ds-space-2) var(--ds-space-2) var(--ds-space-2) var(--ds-space-3); border-bottom: 1px solid var(--vd-theme-border); }
.infra-sheet-head h2 { flex: 1; margin: 0; font-size: 16px; font-weight: 650; }
.infra-sheet-intro { margin: 0; padding: var(--ds-space-2) var(--ds-space-3) 0; color: var(--vd-theme-muted); font-size: 14px; }
.infra-import-search { padding: var(--ds-space-2) var(--ds-space-3); }
.infra-import-search input { width: 100%; height: 34px; padding: 0 var(--ds-space-2); border: 1px solid var(--vd-theme-border); border-radius: var(--ds-radius-sm); background: var(--vd-theme-control-bg); color: var(--vd-text); font-size: 14px; }
.infra-import-groups { flex: 1; min-height: 0; overflow: auto; padding: 0 var(--ds-space-2) var(--ds-space-2); }
.infra-import-group + .infra-import-group { margin-top: var(--ds-space-3); }
.infra-import-group-head { display: flex; align-items: center; gap: var(--ds-space-1); margin: 0; }
.infra-import-group-head .infra-library-toggle { flex: 1; }
.infra-import-group .infra-candidate-list { display: grid; gap: var(--ds-space-1); margin: var(--ds-space-1) 0 0; padding: 0; list-style: none; }
.infra-import-group .infra-candidate { display: grid; grid-template-columns: auto 32px minmax(0, 1fr); align-items: center; gap: var(--ds-space-1) var(--ds-space-2); padding: var(--ds-space-2); border: 1px solid var(--vd-theme-border); border-radius: var(--ds-radius-md); background: var(--vd-theme-panel-bg-strong); touch-action: pan-y; }
.infra-import-group .infra-candidate[data-draggable="true"] { cursor: grab; }
.infra-import-group .infra-bind-field, .infra-candidate-actions, .infra-candidate-error { grid-column: 1 / -1; }
.infra-candidate-reason { color: var(--ds-color-warning-fg); font-size: 13px; }
.infra-candidate-actions { display: flex; flex-wrap: wrap; gap: var(--ds-space-1); }
.infra-candidate-actions .infra-button { height: 30px; }
.infra-candidate-error { margin: 0; color: var(--ds-color-danger-fg); font-size: 14px; }
.infra-sheet-foot { display: flex; flex-wrap: wrap; align-items: center; gap: var(--ds-space-2); padding: var(--ds-space-2) var(--ds-space-3); border-top: 1px solid var(--vd-theme-border); }
.infra-sheet-foot [data-import-count] { flex: 1; color: var(--vd-theme-muted); font-size: 14px; }
@container infra (max-width: 759px) { .infra-import-sheet { width: 100%; } }
@media (prefers-reduced-motion: no-preference) { body:not([data-animations="false"]) .infra-import-sheet:not([hidden]) { animation: infra-sheet-in var(--ds-motion-duration-base) var(--ds-motion-ease-decel); } }
@keyframes infra-sheet-in { from { opacity: 0; transform: translateX(-12px); } }
```
- [ ] **Step 7: Fixture** (`ui/desktop_infra_browser_test.go`): add struct fields `sources []map[string]any`, `ignoredCandidates map[string]bool`, `batchCalls []map[string]any`, `ignoreCalls []map[string]any`; initialise `ignoredCandidates: map[string]bool{}`. Replace `snapshot()` with:
```go
func (f *infraAppBrowserFixture) snapshot(includeIgnored bool) map[string]any {
	sources := make([]map[string]any, 0, len(f.sources))
	for _, source := range f.sources {
		if source["ignored"] == true && !includeIgnored {
			continue
		}
		sources = append(sources, source)
	}
	return map[string]any{"nodes": f.nodes, "edges": f.edges, "sources": sources, "source_hosts": []any{}, "capabilities": f.capabilities}
}

func infraFixtureSameSource(raw any, identity map[string]any) bool {
	source, _ := raw.(map[string]any)
	return source != nil && source["kind"] == identity["kind"] && source["instance"] == identity["instance"] && source["key"] == identity["key"]
}

// sourceRow finds an observed source row, adding one from a discovery when needed.
func (f *infraAppBrowserFixture) sourceRow(identity map[string]any) map[string]any {
	for _, source := range f.sources {
		if infraFixtureSameSource(source, identity) {
			return source
		}
	}
	for _, candidate := range f.candidates {
		if infraFixtureSameSource(candidate["source"], identity) {
			row := cloneInfraBrowserNode(candidate["source"].(map[string]any))
			delete(row, "profile")
			f.sources = append(f.sources, row)
			return row
		}
	}
	return nil
}

// importCandidate mirrors POST /import for one discovery: node, status, error code.
func (f *infraAppBrowserFixture) importCandidate(candidateID, nodeID string) (map[string]any, int, string) {
	if f.failCandidateOnce == candidateID {
		f.failCandidateOnce = ""
		return nil, http.StatusUnprocessableEntity, "fixture_retryable_candidate"
	}
	var candidate map[string]any
	for _, item := range f.candidates {
		if item["id"] == candidateID && !f.consumed[candidateID] && !f.ignoredCandidates[candidateID] {
			candidate = item
			break
		}
	}
	if candidate == nil {
		return nil, http.StatusNotFound, "not_found"
	}
	source := candidate["source"].(map[string]any)
	if nodeID != "" {
		var target map[string]any
		for _, item := range f.nodes {
			if item["id"] == nodeID {
				target = item
				break
			}
		}
		if target == nil {
			return nil, http.StatusNotFound, "not_found"
		}
		f.consumed[candidateID] = true
		sources, _ := target["sources"].([]any)
		target["sources"] = append(sources, source)
		if source["kind"] == "registry" {
			// keep the existing registry block of the old /import case verbatim here
		}
		return target, http.StatusOK, ""
	}
	f.consumed[candidateID] = true
	// keep the existing "create imported-N node" block of the old /import case verbatim here
	return node, http.StatusCreated, ""
}
```
  (Move the two existing blocks from the old `/import` case into the marked places unchanged, except `target["sources"]` uses the safe `sources` slice above.) In `ServeHTTP`: `/snapshot` → `write(http.StatusOK, f.snapshot(r.URL.Query().Get("include_ignored") == "1"))`; `/candidates` also skips `f.ignoredCandidates[id]`; `/import` becomes:
```go
		node, status, code := f.importCandidate(request.CandidateID, request.NodeID)
		if code != "" {
			write(status, map[string]string{"error": code})
			return
		}
		write(status, map[string]any{"node": node})
```
  Add cases:
```go
	case path == "/import/batch" && r.Method == http.MethodPost:
		var payload struct {
			Items []map[string]any `json:"items"`
		}
		if !decode(&payload) || len(payload.Items) == 0 || len(payload.Items) > 500 {
			write(http.StatusBadRequest, map[string]string{"error": "invalid_request"})
			return
		}
		f.batchCalls = append(f.batchCalls, map[string]any{"items": payload.Items})
		nodes, failures := []map[string]any{}, []map[string]any{}
		for _, item := range payload.Items {
			candidateID, _ := item["candidate_id"].(string)
			nodeID, _ := item["node_id"].(string)
			node, _, code := f.importCandidate(candidateID, nodeID)
			if code != "" {
				failures = append(failures, map[string]any{"candidate_id": candidateID, "error": code, "message": code})
				continue
			}
			if parent, ok := item["parent_id"].(string); ok {
				if parent == "" {
					delete(node, "parent_id")
				} else {
					node["parent_id"] = parent
				}
			}
			if placement, ok := item["placement"].(map[string]any); ok {
				placements, _ := node["placements"].(map[string]any)
				if placements == nil {
					placements = map[string]any{}
					node["placements"] = placements
				}
				view, _ := placement["view"].(string)
				placements[view] = map[string]any{"x": placement["x"], "y": placement["y"], "pinned": true}
			}
			nodes = append(nodes, node)
		}
		write(http.StatusOK, map[string]any{"nodes": nodes, "errors": failures})
	case path == "/sources/ignore" && r.Method == http.MethodPut:
		var payload struct {
			Sources []map[string]any `json:"sources"`
			Ignored *bool            `json:"ignored"`
		}
		if !decode(&payload) || payload.Ignored == nil || len(payload.Sources) == 0 {
			write(http.StatusBadRequest, map[string]string{"error": "invalid_request"})
			return
		}
		f.ignoreCalls = append(f.ignoreCalls, map[string]any{"sources": payload.Sources, "ignored": *payload.Ignored})
		rows := make([]map[string]any, 0, len(payload.Sources))
		for _, identity := range payload.Sources {
			row := f.sourceRow(identity)
			if row == nil {
				write(http.StatusNotFound, map[string]string{"error": "not_found"})
				return
			}
			rows = append(rows, row)
		}
		for index, identity := range payload.Sources {
			rows[index]["ignored"] = *payload.Ignored
			for _, candidate := range f.candidates {
				if infraFixtureSameSource(candidate["source"], identity) {
					f.ignoredCandidates[candidate["id"].(string)] = *payload.Ignored
				}
			}
		}
		write(http.StatusOK, map[string]any{"sources": rows})
```
  Update every other `f.snapshot()` caller to `f.snapshot(false)`.
- [ ] **Step 8: Verify** `node --check`, `go vet ./ui`, V-core, V-browser (the main test now imports through `/import/batch`: first click imports the registry link and keeps `docker-candidate` checked with its inline error, the second click imports it).
- [ ] **Step 9: Commit** paths `ui/js/desktop/apps/infra-import.js ui/js/desktop/apps/infra-dialogs.js ui/js/desktop/apps/infra-gestures.js ui/js/desktop/apps/infra-canvas.js ui/js/desktop/apps/infra.js ui/js/desktop/core/module-loader.js scripts/test-infra-ui.mjs ui/css/desktop-app-infra.css ui/desktop_infra_browser_test.go`; subject `feat(infra): import sheet with relevance groups, ignore and drag-to-map batch import`.

---

## Task 13: i18n batch I (List, menus, command bar, shortcuts, legend)

Same procedure (`$SCRATCH/infra-i18n-i.json`, `+44`; `desktop.infra_removed_from_map` already landed in batch F). Subject `i18n(infra): add list, menu, shortcut and legend keys`.

| key | en | de |
|---|---|---|
| `desktop.infra_sort_by` | Sort by {column} | Nach {column} sortieren |
| `desktop.infra_select_row` | Select {name} | {name} auswählen |
| `desktop.infra_select_all_rows` | Select all {count} rows | Alle {count} Zeilen auswählen |
| `desktop.infra_bulk_actions` | Actions for the selection | Aktionen für die Auswahl |
| `desktop.infra_clear_selection` | Clear selection | Auswahl aufheben |
| `desktop.infra_remove_from_map` | Remove from map | Von der Karte nehmen |
| `desktop.infra_shortcuts` | Keyboard shortcuts | Tastenkürzel |
| `desktop.infra_legend` | Legend | Legende |
| `desktop.infra_legend_status` | Source status | Quellenstatus |
| `desktop.infra_legend_power` | Power state | Betriebszustand |
| `desktop.infra_clear_filters` | Clear filters | Filter löschen |
| `desktop.infra_remove_filter` | Remove filter {filter} | Filter {filter} entfernen |
| `desktop.infra_active_filters` | Active filters | Aktive Filter |
| `desktop.infra_keys_general` | General | Allgemein |
| `desktop.infra_keys_pointer` | Mouse and touch | Maus und Touch |
| `desktop.infra_key_search` | Search or jump to an asset | Asset suchen oder anspringen |
| `desktop.infra_key_undo` | Undo or redo | Rückgängig machen oder wiederholen |
| `desktop.infra_key_save` | Save the open details | Geöffnete Details speichern |
| `desktop.infra_key_escape` | Cancel or close | Abbrechen oder schließen |
| `desktop.infra_key_help` | Show this list | Diese Liste anzeigen |
| `desktop.infra_key_arrows` | Move between assets | Zwischen Assets wechseln |
| `desktop.infra_key_enter` | Open details or toggle the selection | Details öffnen oder Auswahl umschalten |
| `desktop.infra_key_move` | Move the selection; Enter saves, Escape cancels | Auswahl verschieben; Eingabe speichert, Escape bricht ab |
| `desktop.infra_key_place` | Place a library row at the map centre | Bibliothekszeile in der Kartenmitte platzieren |
| `desktop.infra_key_select_all` | Select all visible assets | Alle sichtbaren Assets auswählen |
| `desktop.infra_key_delete` | Remove the selection from Infra | Auswahl aus Infra entfernen |
| `desktop.infra_key_menu` | Open the context menu | Kontextmenü öffnen |
| `desktop.infra_key_fit` | Fit the map to the window | Karte an das Fenster anpassen |
| `desktop.infra_key_zoom` | Zoom in, zoom out or reset to 100 % | Vergrößern, verkleinern oder auf 100 % zurücksetzen |
| `desktop.infra_key_tools` | Select, pan or connect tool | Werkzeug Auswählen, Verschieben oder Verbinden |
| `desktop.infra_key_marquee` | Select an area | Einen Bereich auswählen |
| `desktop.infra_key_pan` | Pan the map | Karte verschieben |
| `desktop.infra_key_wheel_zoom` | Zoom at the pointer | Am Mauszeiger zoomen |
| `desktop.infra_key_drag` | drag | ziehen |
| `desktop.infra_key_wheel` | wheel | Mausrad |
| `desktop.infra_kbd_ctrl` | Ctrl | Strg |
| `desktop.infra_kbd_shift` | Shift | Umschalt |
| `desktop.infra_kbd_enter` | Enter | Eingabe |
| `desktop.infra_kbd_space` | Space | Leertaste |
| `desktop.infra_kbd_esc` | Esc | Esc |
| `desktop.infra_kbd_delete` | Delete | Entf |

(Remove the `desktop.infra_remove_from_map` row here if it was already added in batch F; the script reports `nothing new` for existing keys anyway.)

---

## Task 14: List view upgrade

**Files:** `ui/js/desktop/apps/infra-table.js`, `ui/css/desktop-app-infra.css`.

- [ ] **Step 1: Impact** for table `create`/`render`; `git grep -n "data-table-body\|app.table" -- ui`.
- [ ] **Step 2: Rewrite `infra-table.js`** with this contract:
  - Prefs `infra.table.v1` via `model.readTablePrefs`; labels with literal calls: `{ name: t('desktop.infra_name'), kind: t('desktop.infra_type'), place: t('desktop.infra_place'), address: t('desktop.infra_address'), hardware: t('desktop.infra_group_hardware'), source: t('desktop.infra_source'), status: t('desktop.infra_status'), power: t('desktop.infra_column_power') }`.
  - Markup: `<section class="infra-table-view" data-table-view hidden aria-label="{t inventory}"><div class="infra-table-head"><h2>{t inventory}</h2><span data-table-total></span></div><div class="infra-bulkbar" data-bulkbar role="group" aria-label="{t desktop.infra_bulk_actions}" hidden><span class="infra-bulkbar-count" data-bulk-count></span><button type="button" class="infra-button" data-action="bulk-move" aria-haspopup="dialog">{t move_to_place}</button><button type="button" class="infra-button infra-danger-button" data-action="bulk-ignore">{t remove_and_ignore}</button><button type="button" class="infra-button infra-danger-button" data-action="bulk-remove">{t remove_node}</button><button type="button" class="infra-button" data-action="bulk-clear">{t desktop.infra_clear_selection}</button></div><div class="infra-table-scroll" data-table-scroll tabindex="0"><table class="infra-table"><thead><tr><th class="infra-col-check" scope="col"><input type="checkbox" data-table-check-all aria-label="…"></th>{per column: <th scope="col" data-sort-col="{key}" [aria-sort="ascending|descending" only on the sorted column]>{sortable (all but hardware): <button type="button" class="infra-sort" data-sort="{key}" title="{fill(t sort_by,{column})}">{label}<span class="infra-sort-arrow" aria-hidden="true"></span></button> : label}</th>}</tr></thead><tbody data-table-body></tbody></table></div></section>`.
  - Rows: memo keyed by `[state.nodes, state.edges, JSON.stringify(state.filters), prefs.sort, prefs.dir, expandedRows sorted]` → `model.tableRows(state.nodes, { …store.guests(), compare: model.tableComparator(prefs.sort, prefs.dir, { labelOf: app.labelForKind, placeOf: id => parent name, sourceOf: node => translated source kinds joined ', ' }), matches: store.matches, expanded: expandedRows, filtering: model.filtersActive(state.filters) })`; total = matching node count → `[data-table-total]` `fill(t asset_count)`.
  - `render()`: `model.virtualSlice(rows, scrollTop, clientHeight || 600)` (row height 54), spacers `colspan="9"`; row markup `<tr data-row="{id}" data-depth="{d}" class="[is-selected][ is-context]"><td class="infra-col-check"><input type="checkbox" data-row-check="{id}" aria-label="{fill(t select_row,{name})}" {checked}></td><td class="infra-col-name"><div class="infra-table-name-cell" style="--infra-depth:{d}">{guests ? <button type="button" class="infra-library-disclosure" data-row-expand="{id}" aria-expanded="{bool}" aria-label="{fill(t show_guests|hide_guests,{name})}">{glyph chevron}</button> : <span class="infra-disclosure-spacer" aria-hidden="true"></span>}<button type="button" class="infra-table-name" data-select="{id}" aria-current="{bool}">{icon}<span>{name}</span>{guests ? <small class="infra-library-count">{n}</small>}</button></div></td><td>{kind}</td><td>{place or —}</td><td class="infra-col-address">{ip||mac||—}</td><td>{vendor · model||—}</td><td>{sources||t manual}</td><td>{state pill}</td><td>{power pill or —}</td></tr>`. Restore focus to the same `[data-select|data-row-check|data-row-expand]` after a rebuild. Head: `aria-sort`, check-all `checked`/`indeterminate` over the rows list and its `aria-label` `fill(t select_all_rows, {count})`. Bulk bar: hidden when `state.selection.size === 0`, count `fill(t selected_count)`, buttons disabled with read-only title when `!canWrite` (clear stays enabled).
  - Events: `[data-sort]` (same key toggles dir, new key asc; persist); `[data-row-check]` click (Shift with an anchor selects the range additively, else `store.select([id], { toggle: true })`, anchor = id); check-all (all selected → `clearSelection`, else `store.select(rowIds)`); `[data-row-expand]` toggles; `[data-select]` as Plan B; bulk: move → `app.picker.openPlacePicker({ anchor: button, exclude: ids, onPick: placeId => app.gestures.moveIntoPlace(ids, placeId) })`, ignore → `app.dialogs.removeAndIgnore(ids)`, remove → `app.dialogs.deleteNodes(ids)`, clear → `store.clearSelection()`; keydown `Delete`/`Backspace` inside tbody (not typing) → `deleteNodes(selection or row id)`; passive scroll → render.
- [ ] **Step 3: CSS:**
```css
.infra-table { width: 100%; border-collapse: separate; border-spacing: 0; font-size: 14px; }
.infra-table thead th { position: sticky; top: 0; z-index: 1; height: 40px; padding: 0 var(--ds-space-2); border-bottom: 1px solid var(--vd-theme-border); background: var(--vd-theme-panel-bg); text-align: left; white-space: nowrap; }
.infra-sort { display: inline-flex; align-items: center; gap: 4px; height: 32px; padding: 0 4px; border: 0; border-radius: var(--ds-radius-sm); background: transparent; color: var(--vd-theme-muted); font: 600 13px/1 var(--vd-font-ui); cursor: pointer; }
.infra-sort:hover, th[aria-sort] .infra-sort { color: var(--vd-text); }
.infra-sort-arrow { width: 0; height: 0; border-left: 4px solid transparent; border-right: 4px solid transparent; opacity: 0; }
th[aria-sort="ascending"] .infra-sort-arrow { border-bottom: 5px solid currentColor; opacity: 1; }
th[aria-sort="descending"] .infra-sort-arrow { border-top: 5px solid currentColor; opacity: 1; }
.infra-table tbody tr[data-row] { height: 54px; }
.infra-table tbody td { max-width: 240px; overflow: hidden; padding: 0 var(--ds-space-2); border-bottom: 1px solid var(--vd-theme-border); text-overflow: ellipsis; white-space: nowrap; }
.infra-table tbody tr.is-selected td { background: var(--vd-theme-accent-soft); }
.infra-table tbody tr.is-context td { color: var(--vd-theme-muted); }
.infra-col-check { width: 40px; text-align: center; }
.infra-col-check input { width: 18px; height: 18px; accent-color: var(--vd-accent); }
.infra-table-name-cell { display: flex; align-items: center; gap: 4px; padding-left: calc(var(--infra-depth, 0) * var(--ds-space-5)); }
.infra-disclosure-spacer { flex: 0 0 auto; width: 32px; }
.infra-col-address { font-variant-numeric: tabular-nums; }
.infra-bulkbar { display: flex; flex-wrap: wrap; align-items: center; gap: var(--ds-space-2); padding: var(--ds-space-2) var(--ds-space-3); border-bottom: 1px solid var(--vd-theme-border); background: var(--vd-theme-accent-soft); }
.infra-bulkbar-count { flex: 1; font-weight: 600; }
```
- [ ] **Step 4: Verify** `node --check`, V-core, `AURAGO_RUN_BROWSER_SMOKE=1 go test ./ui -run 'TestDesktopInfraAppBrowser' -count=1 -timeout 20m` (1000-row virtualization, `[data-row="fixture-node-0002"] [data-select]`).
- [ ] **Step 5: Commit** paths `ui/js/desktop/apps/infra-table.js ui/css/desktop-app-infra.css`; subject `feat(infra): sortable tree list view with multi-select bulk actions`.

---

## Task 15: Menus, command bar chips, legend, shortcuts, keys

**Files:** create `ui/js/desktop/apps/infra-menus.js`; modify `infra.js`, `infra-gestures.js`, `infra-inspector.js` (node-more now always menus), loader, test list, CSS.

- [ ] **Step 1: Impact** for infra.js `paintShell`, `openMenu`, root click and keydown handlers; gestures `onRootKeydown`.
- [ ] **Step 2: Create `infra-menus.js`** (`InfraAppMenus.create(app)`); `show(x, y, items)` returns false without `ctx.showContextMenu`. Item builders (writes disabled with `disabledHint: t('desktop.infra_read_only')` when `!store.canWrite()`; ids become `data-context-action`):
  - `assetItems(id, anchor)` — `ids` = whole selection when `id` is in a multi-selection, else `[id]`; capture `rect = anchor.getBoundingClientRect()`: `open` (single; `app.inspector.open(id, 'overview')`, `label: t('desktop.infra_open')`, icon `info`), `move` (`t('desktop.infra_move_to_place')`, icon `folder`, `app.picker.openPlacePicker({ anchor, rect, exclude: ids, onPick: placeId => app.gestures.moveIntoPlace(ids, placeId) })`), `connect` (single; `t('desktop.infra_connect_to')`, icon `link`, asset picker → `app.dialogs.openEdgeDialog('', id, option.id)`), `app-<n>` per `app_links` (single; `model.fill(t('desktop.infra_open_in_app'), { app: t(\`desktop.infra_app_${app.replaceAll('-', '_')}\`) })` → `app.openAppLink(link)`), `unplace` (`t('desktop.infra_remove_from_map')`, hidden unless placed in `app.mapView()`, `features.unplace` and not table view → `app.gestures.removeFromMap(ids)`), separator, `remove` (`t('desktop.infra_remove_node')`, icon `trash` → `app.dialogs.deleteNodes(ids)`), `remove-ignore` (`t('desktop.infra_remove_and_ignore')`, hidden unless any id has sources → `app.dialogs.removeAndIgnore(ids)`).
  - `placeItems(id, anchor)`: open, `rename` (`t('desktop.infra_rename')` → `inspector.open(id, 'details'); inspector.focusName()`), move (exclude `[id]`), unplace, separator, remove.
  - `edgeItems(edgeId)`: `edit` (`t('desktop.infra_edit_connection')` → `store.selectEdge(id); app.inspector.focusEdgeForm()`), `remove` (`t('desktop.infra_remove_connection')` → `app.dialogs.removeEdge(id)`), both disabled for provider edges (`disabledHint: t('desktop.infra_connection_readonly')`).
  - `canvasItems()`: `add` (`t('desktop.infra_new_asset')`), `add-place` (`t('desktop.infra_new_place')`), `import` (`t('desktop.infra_import_discoveries')` → `app.importer.open()`), separator, `fit` (`t('desktop.infra_zoom_fit')`).
  - `sourceActionItems(node)` (control only): per allowed source action `label: \`${t(\`desktop.infra_action_${action}\`)} · ${t(\`desktop.infra_source_${source.kind}\`)}\``, disabled while blocked, → `app.dialogs.runAction(node.id, source, action)`; prefixed by a separator.
  - `onContextMenu(event)`: false for editable targets or while `state.interacting`; resolution order `[data-node]` in the viewport, `[data-region-header]`, `[data-edge]` (select it first), `[data-hosted-service]`, `[data-library-item]`/`[data-row]` (place kinds → placeItems), else empty `[data-stage]` → canvasItems. Select the target first when it is not selected. Shows at the pointer.
  - `openForElement(el)` (Shift+F10 / ContextMenu key): same resolution, shown at `rect.left + min(24, width/2), rect.bottom`.
  - `openNodeMenu(id, button)`: falls back to `app.dialogs.openNodeActions(id)` without `showContextMenu`; else asset/place items for `[id]` plus `sourceActionItems`, at the button's bottom-left.
  - `openShortcuts()`: `app.dialogs.openDialog(t('desktop.infra_shortcuts'), html)` where `html` is `<div class="infra-shortcuts">` with three `<section><h3>…</h3><dl>` lists (`<dt>` of `<kbd>` keys, `<dd>` text): General (`t('desktop.infra_keys_general')`): Ctrl+K search, Ctrl+Z / Ctrl+Shift+Z undo, Ctrl+S save, Esc, `?` help; Map (`t('desktop.infra_view_map')`): ←↑→↓ arrows, Enter / Space, M, P, Ctrl+A, Delete, Shift+F10 menu, F, + - 0, V H C; Mouse and touch (`t('desktop.infra_keys_pointer')`): Shift + `t('desktop.infra_key_drag')` marquee, Space + drag pan, Ctrl + `t('desktop.infra_key_wheel')` zoom. Modifier and key names come from `t('desktop.infra_kbd_ctrl|shift|enter|space|esc|delete')`; descriptions from the `desktop.infra_key_*` keys.
  - `legendHTML()`: `<section><h3>{t legend_status}</h3><ul>` five `<li><span class="infra-legend-ring" data-state="{state}" aria-hidden="true">{● ◐ ○ ✕ or empty}</span>{app.statusLabel(state)}</li>` for current/stale/unknown/removed/manual; `<section><h3>{t legend_power}</h3>` running/paused/stopped `<i class="infra-legend-dot" data-power="…" aria-hidden="true"></i>{t(model.powerLabel(power))}`; `<section><h3>{t tab_connections}</h3>` ethernet/fiber/wifi/depends_on/powered_by `<svg class="infra-legend-line" data-kind="{kind}" viewBox="0 0 40 8" width="40" height="8" aria-hidden="true" focusable="false"><line x1="2" y1="4" x2="38" y2="4"/></svg>{t edge_kind}`.
- [ ] **Step 3: `infra.js`:**
  - `if (typeof ctx.wireContextMenuBoundary === 'function') ctx.wireContextMenuBoundary(root, { onContextMenu: event => app.menus.onContextMenu(event) }); else root.addEventListener('contextmenu', event => { if (app.menus.onContextMenu(event)) { event.preventDefault(); event.stopPropagation(); } });` (bound on the fresh `.infra-app` root).
  - `[data-menu="more"]`: insert after Arrange `<button type="button" role="menuitem" data-action="shortcuts" aria-keyshortcuts="Shift+?">{t desktop.infra_shortcuts}</button><button type="button" role="menuitem" data-action="legend" aria-haspopup="dialog">{t desktop.infra_legend}</button>`. Layer gets `<div class="infra-popover infra-legend" id="infra-legend-${S}" data-popover="legend" role="dialog" aria-label="{t desktop.infra_legend}" tabindex="-1" hidden>{menus.legendHTML()}</div>`; `openMenu` focuses the popover itself when it has no menu items; `legend` opens it aligned to `[data-action="more"]`.
  - Filters popover: append `<button type="button" class="infra-button" data-action="clear-filters" disabled>{t desktop.infra_clear_filters}</button>`. Command bar: after `.infra-search` add `<div class="infra-filter-chips" data-filter-chips role="group" aria-label="{t desktop.infra_active_filters}" hidden></div>`. `paintFilterChips()` (from `paintShell`, signature-cached) renders one `<button type="button" class="infra-filter-chip" data-remove-filter="{kind|source|power}" aria-label="{fill(t remove_filter,{filter: label})}" title="…"><span>{label}</span>{glyph close}</button>` per active filter, label `` `${t('desktop.infra_type')}: ${labelForKind}` ``, `` `${t('desktop.infra_source')}: ${t(\`desktop.infra_source_${source}\`)}` ``, `` `${t('desktop.infra_power_filter')}: …` `` (`t('desktop.infra_power_none')` or `t(model.powerLabel(power))`); toggles `clear-filters` disabled. Root actions: `[data-remove-filter]` → `store.setFilters({ [key]: '' })`, reset the matching select, focus the next chip or `[data-search]`; `clear-filters` → reset all three; `shortcuts` → `menus.openShortcuts()`; `legend` → `openMenu('legend', moreTrigger)`.
  - Create `app.menus` after the importer; dispose it.
- [ ] **Step 4: Gestures `onRootKeydown` additions** (skip typing targets and open `[data-dialog]`): `event.key === '?'` → `app.menus.openShortcuts()`; `event.key === 'ContextMenu' || (event.shiftKey && event.key === 'F10')` → `preventDefault`, `app.menus.openForElement(document.activeElement)`.
- [ ] **Step 5: Inspector:** `node-more` calls `app.menus.openNodeMenu(id, button)`.
- [ ] **Step 6: CSS:**
```css
.infra-filter-chips { display: flex; flex: 0 1 auto; gap: 4px; min-width: 0; max-width: 360px; overflow: hidden; }
.infra-filter-chip { display: inline-flex; flex: 0 0 auto; align-items: center; gap: 4px; height: 28px; padding: 0 4px 0 var(--ds-space-2); border: 1px solid var(--vd-theme-accent-border); border-radius: var(--ds-radius-full); background: var(--vd-theme-accent-soft); color: var(--vd-text); font-size: 13px; font-weight: 600; cursor: pointer; }
@container infra (max-width: 1099px) { .infra-filter-chips { display: none !important; } }
.infra-legend { display: grid; gap: var(--ds-space-3); width: 280px; padding: var(--ds-space-3); }
.infra-legend h3, .infra-shortcuts h3 { margin: 0 0 var(--ds-space-1); color: var(--vd-theme-muted); font-size: 13px; font-weight: 600; }
.infra-legend ul { display: grid; gap: 6px; margin: 0; padding: 0; list-style: none; font-size: 14px; }
.infra-legend li { display: flex; align-items: center; gap: var(--ds-space-2); }
.infra-legend-ring { width: 18px; height: 18px; display: grid; place-items: center; border-radius: 50%; box-shadow: 0 0 0 2px var(--infra-state, var(--vd-theme-border-strong)); color: var(--infra-state, var(--vd-theme-muted)); font-size: 11px; }
.infra-legend-ring[data-state="current"] { --infra-state: var(--ds-color-success-fg); }
.infra-legend-ring[data-state="stale"] { --infra-state: var(--ds-color-warning-fg); }
.infra-legend-ring[data-state="removed"] { --infra-state: var(--ds-color-danger-fg); }
.infra-legend-ring[data-state="unknown"] { --infra-state: var(--vd-theme-muted); }
.infra-legend-dot { width: 8px; height: 8px; margin: 0 5px; border-radius: 50%; background: var(--vd-theme-muted); }
.infra-legend-dot[data-power="running"] { background: var(--ds-color-success-fg); }
.infra-legend-dot[data-power="paused"] { background: var(--ds-color-warning-fg); }
.infra-legend-dot[data-power="stopped"] { background: var(--ds-color-danger-fg); }
.infra-legend-line line { stroke: color-mix(in srgb, var(--vd-theme-muted) 70%, var(--vd-text)); stroke-width: 2; }
.infra-legend-line[data-kind="wifi"] line { stroke-dasharray: 6 5; }
.infra-legend-line[data-kind="fiber"] line { stroke: var(--ds-color-info-fg); stroke-width: 4; }
.infra-legend-line[data-kind="depends_on"] line { stroke-dasharray: 1.5 4; stroke-linecap: round; }
.infra-legend-line[data-kind="powered_by"] line { stroke: var(--ds-color-warning-fg); }
.infra-shortcuts { display: grid; gap: var(--ds-space-3); }
.infra-shortcuts dl { display: grid; grid-template-columns: max-content minmax(0, 1fr); gap: 6px var(--ds-space-3); margin: 0; font-size: 14px; }
.infra-shortcuts dt { display: flex; flex-wrap: wrap; align-items: center; gap: 4px; }
.infra-shortcuts dd { margin: 0; }
.infra-shortcuts kbd { padding: 2px 6px; border: 1px solid var(--vd-theme-border); border-bottom-width: 2px; border-radius: var(--ds-radius-xs); background: var(--vd-theme-control-bg); font: 600 13px/1.4 var(--vd-font-ui); }
```
- [ ] **Step 7: Verify** `node --check`, V-core, V-browser.
- [ ] **Step 8: Commit** paths `ui/js/desktop/apps/infra-menus.js ui/js/desktop/apps/infra.js ui/js/desktop/apps/infra-gestures.js ui/js/desktop/apps/infra-inspector.js ui/js/desktop/core/module-loader.js scripts/test-infra-ui.mjs ui/css/desktop-app-infra.css`; subject `feat(infra): context menus, filter chips, legend and shortcut sheet`.

---

## Task 16: Fixture routes and panel scenario browser tests

**Files:** `ui/desktop_infra_browser_test.go`, create `ui/desktop_infra_panels_browser_test.go`.

- [ ] **Step 1: Impact** for `ServeHTTP` (infraAppBrowserFixture) and `snapshot`; `git grep -n "newInfraAppBrowserFixture\|infraCanvasFixtureNode" -- ui`.
- [ ] **Step 2: Fixture additions:** fields `sourceHosts []map[string]any`, `sourceHostCalls []map[string]any`, `deleteCalls []string`, `nodePuts []map[string]any`, `targets []map[string]any` (initialised to the three current `/targets` entries; the `/targets` case answers `{"targets": f.targets}`). `snapshot()` answers `"source_hosts": f.sourceHosts` (empty slice when nil). PUT `/nodes/{id}` appends the decoded body to `f.nodePuts`. In the `/layout` loop use `if change["unplace"] == true { delete(placements, payload.View) } else { placements[payload.View] = placement }`. New cases:
```go
	case strings.HasPrefix(path, "/nodes/") && r.Method == http.MethodDelete:
		id := strings.TrimPrefix(path, "/nodes/")
		f.deleteCalls = append(f.deleteCalls, id+"?"+r.URL.RawQuery)
		index := -1
		for i, node := range f.nodes {
			if node["id"] == id {
				index = i
				break
			}
		}
		if index < 0 {
			write(http.StatusNotFound, map[string]string{"error": "not_found"})
			return
		}
		node := f.nodes[index]
		if r.URL.Query().Get("version") != fmt.Sprint(infraFixtureNumber(node["version"])) {
			write(http.StatusConflict, map[string]string{"error": "conflict"})
			return
		}
		for _, other := range f.nodes {
			if other["parent_id"] == id {
				write(http.StatusConflict, map[string]string{"error": "has_children"})
				return
			}
		}
		response := map[string]any{"status": "deleted", "id": id}
		if r.URL.Query().Get("ignore_sources") == "1" {
			ignored := []any{}
			sources, _ := node["sources"].([]any)
			for _, raw := range sources {
				source, _ := raw.(map[string]any)
				if source == nil {
					continue
				}
				row := cloneInfraBrowserNode(source)
				delete(row, "profile")
				row["ignored"] = true
				f.sources = append(f.sources, row)
				ignored = append(ignored, map[string]any{"kind": source["kind"], "instance": source["instance"], "key": source["key"]})
			}
			response["ignored"] = ignored
		}
		f.nodes = append(append([]map[string]any{}, f.nodes[:index]...), f.nodes[index+1:]...)
		kept := make([]map[string]any, 0, len(f.edges))
		for _, edge := range f.edges {
			if edge["from"] != id && edge["to"] != id {
				kept = append(kept, edge)
			}
		}
		f.edges = kept
		write(http.StatusOK, response)
	case path == "/source-hosts" && r.Method == http.MethodPut:
		var payload struct {
			Kind     string  `json:"kind"`
			Instance string  `json:"instance"`
			NodeID   string  `json:"node_id"`
			Version  float64 `json:"version"`
		}
		if !decode(&payload) || payload.Kind != "docker" || payload.Instance == "" {
			write(http.StatusBadRequest, map[string]string{"error": "invalid_request"})
			return
		}
		f.sourceHostCalls = append(f.sourceHostCalls, map[string]any{"instance": payload.Instance, "node_id": payload.NodeID, "version": payload.Version})
		var entry map[string]any
		for _, host := range f.sourceHosts {
			if host["instance"] == payload.Instance {
				entry = host
			}
		}
		current := 0.0
		if entry != nil {
			current = infraFixtureNumber(entry["version"])
		}
		if payload.Version != current {
			write(http.StatusConflict, map[string]string{"error": "conflict"})
			return
		}
		if entry == nil {
			entry = map[string]any{"kind": "docker", "instance": payload.Instance}
			f.sourceHosts = append(f.sourceHosts, entry)
		}
		entry["node_id"], entry["version"] = payload.NodeID, current+1
		// Mirror the server projection: the engine's containers now run on the node.
		for _, node := range f.nodes {
			sources, _ := node["sources"].([]any)
			for _, raw := range sources {
				source, _ := raw.(map[string]any)
				if node["kind"] == "container" && source != nil && source["kind"] == "docker" && source["instance"] == payload.Instance {
					if payload.NodeID == "" {
						delete(node, "host_id")
					} else {
						node["host_id"] = payload.NodeID
					}
				}
			}
		}
		write(http.StatusOK, map[string]any{"source_host": entry})
```
- [ ] **Step 3: Create `ui/desktop_infra_panels_browser_test.go`:**
```go
package ui

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/input"
	"github.com/go-rod/rod/lib/proto"
)

func infraPanelsSource(kind, instance, key, name string) map[string]any {
	return map[string]any{"kind": kind, "instance": instance, "key": key, "name": name, "state": "current", "observed": map[string]any{"name": name}}
}

func infraPanelsNode(id, kind, name, icon, ip string, placement, source map[string]any) map[string]any {
	node := infraCanvasFixtureNode(id, kind, name, icon, placement, "")
	if ip != "" {
		node["profile"].(map[string]any)["ip_address"] = ip
	}
	if source != nil {
		node["sources"] = []any{source}
	}
	return node
}

func infraPanelsAt(x, y float64) map[string]any { return map[string]any{"x": x, "y": y, "pinned": true} }

func infraPanelsPage(t *testing.T, fixture *infraAppBrowserFixture) *rod.Page {
	t.Helper()
	page := infraBrowserPageWithFixture(t, fixture)
	page.MustSetViewport(1920, 1200, 1, false)
	page.MustEval(`()=>Object.keys(localStorage).filter(key=>key.startsWith('infra.')).forEach(key=>localStorage.removeItem(key))`)
	openInfraBrowserApp(page)
	page.MustWait(`()=>!!document.querySelector('[data-nodes] [data-node]')`)
	page.MustEval(`()=>{const win=document.querySelector('.vd-window[data-app-id="infra"]');win.classList.remove('maximized','vd-mobile-forced-maximized');win.style.right='auto';win.style.width='1500px';win.style.height='970px';win.style.left='210px';win.style.top='70px';}`)
	page.MustWait(`()=>document.querySelector('.infra-app').clientWidth>1000`)
	page.MustEval(`()=>document.querySelector('[data-action="fit"]').click()`)
	return page
}

func infraPanelsCenter(page *rod.Page, selector string) (float64, float64) {
	point := page.MustEval(`selector=>{const r=document.querySelector(selector).getBoundingClientRect();return [r.left+r.width/2,r.top+r.height/2];}`, selector).Arr()
	return point[0].Num(), point[1].Num()
}

func infraPanelsWait(t *testing.T, fixture *infraAppBrowserFixture, what string, ready func() bool) {
	t.Helper()
	deadline := time.Now().Add(8 * time.Second)
	for {
		fixture.mu.Lock()
		ok := ready()
		fixture.mu.Unlock()
		if ok {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for %s", what)
		}
		time.Sleep(50 * time.Millisecond)
	}
}

func infraPanelsConfirm(page *rod.Page) {
	page.MustWait(`()=>!!document.querySelector('.vd-modal [data-choice="discard"]')`)
	page.MustEval(`()=>document.querySelector('.vd-modal [data-choice="discard"]').click()`)
}

// TestDesktopInfraImportAndCleanupBrowser covers engine host assignment,
// remove-and-ignore from a context menu, the duplicate cleanup, Restore, the
// relevance-grouped import sheet, ignoring a discovery and dragging one into a region.
func TestDesktopInfraImportAndCleanupBrowser(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	fixture := newInfraAppBrowserFixture()
	fixture.failCandidateOnce = ""
	registry := func(key, name string) map[string]any { return infraPanelsSource("registry", "inventory", key, name) }
	fixture.nodes = []map[string]any{
		infraCanvasFixtureNode("room-1", "room", "Living room", "room", map[string]any{"x": 400, "y": 120, "w": 480, "h": 320, "pinned": true}, ""),
		infraPanelsNode("core-switch", "device", "Core switch", "managed-switch", "192.168.6.2", infraPanelsAt(160, 200), registry("dev-switch", "Core switch")),
		infraPanelsNode("tv", "device", "Living room TV", "tv", "192.168.6.30", infraPanelsAt(160, 380), registry("dev-tv", "Living room TV")),
		infraPanelsNode("tv-uuid", "device", "Chromecast-3c1d2e4f5a6b7c8d", "tv", "192.168.6.30", nil, registry("dev-tv-uuid", "Chromecast-3c1d2e4f5a6b7c8d")),
		infraPanelsNode("old-nas", "device", "Old NAS", "nas", "192.168.6.21", infraPanelsAt(1100, 380), registry("dev-old-nas", "Old NAS")),
		infraPanelsNode("host-1", "device", "Docker host", "tower-server", "192.168.6.10", infraPanelsAt(1100, 200), nil),
		infraPanelsNode("ct-1", "container", "radarr", "container", "", nil, infraPanelsSource("docker", "engine-a", "c1", "radarr")),
		infraPanelsNode("ct-2", "container", "sonarr", "container", "", nil, infraPanelsSource("docker", "engine-a", "c2", "sonarr")),
	}
	discovery := func(id string, source map[string]any, kind, name, icon string, profile map[string]any) map[string]any {
		profile["interfaces"] = []any{}
		return map[string]any{"id": id, "source": source, "node": map[string]any{"kind": kind, "name": name, "icon": icon, "profile": profile}}
	}
	fritz := func(key, name, power string) map[string]any {
		source := infraPanelsSource("fritzbox", "fritz-a", key, name)
		source["observed"].(map[string]any)["power"] = power
		return source
	}
	fixture.candidates = []map[string]any{
		discovery("cand-nas", registry("dev-nas", "Basement NAS"), "device", "Basement NAS", "nas", map[string]any{"ip_address": "192.168.6.20", "role": "server", "vendor": "Synology"}),
		discovery("cand-brain", registry("dev-brain", "TheBrain"), "device", "TheBrain", "router", map[string]any{"ip_address": "192.168.6.1", "role": "InternetGatewayDevice"}),
		discovery("cand-brain-media", registry("dev-brain-media", "TheBrain Mediaserver"), "device", "TheBrain Mediaserver", "media-server", map[string]any{"ip_address": "192.168.6.1", "role": "MediaServer"}),
		discovery("cand-switch", fritz("fritz-switch", "Core switch", "running"), "device", "Core switch", "generic-device", map[string]any{"ip_address": "192.168.6.2", "vendor": "AVM FRITZ!Box"}),
		discovery("cand-speaker", registry("dev-speaker", "Kitchen speaker"), "device", "Kitchen speaker", "generic-device", map[string]any{"role": "mdns-device", "tags": []any{"network-scan"}}),
		discovery("cand-phone", fritz("fritz-phone", "Old phone", "stopped"), "device", "Old phone", "smartphone", map[string]any{"ip_address": "192.168.6.77", "vendor": "AVM FRITZ!Box"}),
	}
	page := infraPanelsPage(t, fixture)
	defer page.Close()

	page.MustWait(`()=>!!document.querySelector('[data-cleanup^="duplicate:ip:192.168.6.30:"]')&&!!document.querySelector('[data-cleanup="engine-host:docker:engine-a"]')`)
	if !page.MustEval(`()=>document.querySelector('[data-cleanup^="duplicate:ip:192.168.6.30:"]').textContent.includes('Living room TV')`).Bool() {
		t.Fatal("the duplicate suggestion must name the asset it keeps")
	}

	// Assign the Docker engine host.
	page.MustEval(`()=>document.querySelector('[data-cleanup="engine-host:docker:engine-a"] [data-cleanup-action]').click()`)
	page.MustElement(`[data-picker] [data-picker-input]`).MustInput("Docker")
	page.MustWait(`()=>!!document.querySelector('[data-picker] [data-node-option="host-1"]')`)
	page.MustEval(`()=>document.querySelector('[data-picker] [data-node-option="host-1"]').click()`)
	infraPanelsWait(t, fixture, "the engine host write", func() bool { return len(fixture.sourceHostCalls) == 1 })
	fixture.mu.Lock()
	hostCall := fixture.sourceHostCalls[0]
	fixture.mu.Unlock()
	if hostCall["instance"] != "engine-a" || hostCall["node_id"] != "host-1" || hostCall["version"] != float64(0) {
		t.Fatalf("engine host write = %#v", hostCall)
	}
	page.MustWait(`()=>!document.querySelector('[data-cleanup="engine-host:docker:engine-a"]')&&!!document.querySelector('[data-toggle-host-services="host-1"]')`)

	// Remove and ignore through the card context menu.
	nx, ny := infraPanelsCenter(page, `[data-node="old-nas"]`)
	page.Mouse.MustMoveTo(nx, ny)
	page.Mouse.MustClick(proto.InputMouseButtonRight)
	page.MustWait(`()=>!!document.querySelector('.vd-context-menu [data-context-action="remove-ignore"]')`)
	page.MustEval(`()=>document.querySelector('.vd-context-menu [data-context-action="remove-ignore"]').click()`)
	infraPanelsConfirm(page)
	infraPanelsWait(t, fixture, "remove and ignore", func() bool { return len(fixture.deleteCalls) == 1 })
	fixture.mu.Lock()
	removal := fixture.deleteCalls[0]
	fixture.mu.Unlock()
	if removal != "old-nas?version=1&ignore_sources=1" {
		t.Fatalf("remove and ignore sent %q", removal)
	}
	oldNas := infraAppSourceID("registry", "inventory", "dev-old-nas")
	page.MustWait(fmt.Sprintf(`()=>!document.querySelector('[data-node="old-nas"]')&&!!document.querySelector('[data-ignored-source="%s"]')`, oldNas))

	// Remove the duplicate from Cleanup.
	page.MustEval(`()=>document.querySelector('[data-cleanup^="duplicate:ip:192.168.6.30:"] [data-cleanup-action]').click()`)
	infraPanelsConfirm(page)
	infraPanelsWait(t, fixture, "duplicate removal", func() bool { return len(fixture.deleteCalls) == 2 })
	fixture.mu.Lock()
	duplicate := fixture.deleteCalls[1]
	fixture.mu.Unlock()
	if duplicate != "tv-uuid?version=1&ignore_sources=1" {
		t.Fatalf("cleanup removed %q; want the opaque twin with its sources ignored", duplicate)
	}
	page.MustWait(`()=>!document.querySelector('[data-cleanup^="duplicate:"]')&&!!document.querySelector('[data-node="tv"]')&&document.querySelectorAll('[data-ignored-source]').length===2`)

	// Restore from Ignored.
	page.MustEval(fmt.Sprintf(`()=>document.querySelector('[data-restore-source="%s"]').click()`, oldNas))
	infraPanelsWait(t, fixture, "restore", func() bool { return len(fixture.ignoreCalls) == 1 })
	fixture.mu.Lock()
	restore := fixture.ignoreCalls[0]
	fixture.mu.Unlock()
	if restore["ignored"] != false {
		t.Fatalf("restore sent %#v", restore)
	}
	page.MustWait(fmt.Sprintf(`()=>!document.querySelector('[data-ignored-source="%s"]')`, oldNas))

	// The import sheet groups discoveries by relevance.
	page.MustEval(`()=>document.querySelector('[data-action="close-inspector"]')?.click()`)
	page.MustEval(`()=>document.querySelector('[data-action="import"]').click()`)
	page.MustWait(`()=>!!document.querySelector('[data-import-group="recommended"] [data-candidate-row="cand-nas"]')`)
	groups := page.MustEval(`()=>JSON.stringify(Object.fromEntries(['recommended','duplicate','unlikely'].map(group=>[group,[...document.querySelectorAll('[data-import-group="'+group+'"] [data-candidate-row]')].map(row=>row.dataset.candidateRow+(row.querySelector('[data-candidate]').checked?'+':''))])))`).Str()
	if groups != `{"recommended":["cand-nas+","cand-brain+"],"duplicate":["cand-switch","cand-brain-media"],"unlikely":["cand-speaker","cand-phone"]}` {
		t.Fatalf("import sheet groups = %s", groups)
	}
	if !page.MustEval(`()=>document.querySelector('[data-bind="cand-switch"]').value==='core-switch'&&document.querySelector('[data-candidate-row="cand-brain-media"] [data-candidate-reason]').textContent.includes('TheBrain')`).Bool() {
		t.Fatal("a duplicate must link to its asset and explain which discovery it repeats")
	}

	// Ignore a generic discovery.
	page.MustEval(`()=>document.querySelector('[data-ignore-candidate="cand-speaker"]').click()`)
	infraPanelsWait(t, fixture, "ignoring a discovery", func() bool { return len(fixture.ignoreCalls) == 2 })
	fixture.mu.Lock()
	ignore := fixture.ignoreCalls[1]
	fixture.mu.Unlock()
	if sources, _ := ignore["sources"].([]map[string]any); ignore["ignored"] != true || len(sources) != 1 || sources[0]["key"] != "dev-speaker" {
		t.Fatalf("ignore sent %#v", ignore)
	}
	page.MustWait(`()=>!document.querySelector('[data-candidate-row="cand-speaker"]')`)

	// Drag a discovery into the region: one batch call with parent and placement.
	rx, ry := infraPanelsCenter(page, `[data-candidate-row="cand-nas"] .infra-candidate-copy strong`)
	gx, gy := infraPanelsCenter(page, `[data-region="room-1"]`)
	if page.MustEval(`(x,y)=>{const r=document.querySelector('[data-import-sheet]').getBoundingClientRect();return x>=r.left&&x<=r.right&&y>=r.top&&y<=r.bottom;}`, gx, gy).Bool() {
		t.Fatal("the import sheet covers the drop region")
	}
	page.Mouse.MustMoveTo(rx, ry).MustDown(proto.InputMouseButtonLeft)
	page.Mouse.MustMoveTo((rx+gx)/2, (ry+gy)/2)
	page.Mouse.MustMoveTo(gx, gy)
	page.Mouse.MustUp(proto.InputMouseButtonLeft)
	infraPanelsWait(t, fixture, "the batch import", func() bool { return len(fixture.batchCalls) == 1 })
	fixture.mu.Lock()
	items, _ := fixture.batchCalls[0]["items"].([]map[string]any)
	fixture.mu.Unlock()
	if len(items) != 1 || items[0]["candidate_id"] != "cand-nas" || items[0]["parent_id"] != "room-1" {
		t.Fatalf("dropping a discovery into a region sent %#v", items)
	}
	if placement, _ := items[0]["placement"].(map[string]any); placement == nil || placement["view"] != "atlas" {
		t.Fatalf("the batch item lacks an atlas placement: %#v", items[0])
	}
	page.MustWait(`()=>!document.querySelector('[data-candidate-row="cand-nas"]')&&!!document.querySelector('[data-node="imported-1"]')`)
	page.MustEval(`()=>document.querySelector('[data-action="close-import"]').click()`)
	closeInfraBrowserWindow(t, page)
	requireInfraBrowserClean(t, page)
}

// TestDesktopInfraInspectorAndListBrowser covers the tab keyboard pattern, the
// app link picker's MeshCore rule, the sticky save bar with Ctrl S, List sorting
// and bulk remove-and-ignore.
func TestDesktopInfraInspectorAndListBrowser(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	fixture := newInfraAppBrowserFixture()
	registry := func(key, name string) map[string]any { return infraPanelsSource("registry", "inventory", key, name) }
	fixture.nodes = []map[string]any{
		infraPanelsNode("tv", "device", "Living room TV", "tv", "192.168.6.30", infraPanelsAt(160, 200), registry("dev-tv", "Living room TV")),
		infraPanelsNode("relay", "device", "Garden relay", "mesh-node", "", infraPanelsAt(400, 200), nil),
		infraPanelsNode("alpha", "device", "Alpha box", "mini-pc", "192.168.6.9", infraPanelsAt(160, 400), registry("dev-alpha", "Alpha box")),
		infraPanelsNode("mid", "device", "Middle box", "mini-pc", "192.168.6.10", infraPanelsAt(400, 400), nil),
		infraPanelsNode("zeta", "device", "Zeta box", "mini-pc", "192.168.6.40", infraPanelsAt(640, 400), registry("dev-zeta", "Zeta box")),
	}
	fixture.targets = append(fixture.targets, map[string]any{"app": "meshcore", "target_id": "conv-ipt", "label": "Iptingen Room Server"})
	page := infraPanelsPage(t, fixture)
	defer page.Close()

	page.MustEval(`()=>document.querySelector('[data-node="tv"]').click()`)
	page.MustWait(`()=>document.querySelector('[data-inspector-tab="overview"]')?.getAttribute('aria-selected')==='true'`)
	page.MustEval(`()=>document.querySelector('[data-inspector-tab="overview"]').focus()`)
	activeTab := func() string {
		return page.MustEval(`()=>{const el=document.activeElement,tab=el?.dataset?.inspectorTab||'';const panel=document.querySelector('[data-inspector-panel="'+tab+'"]');return tab+(panel&&!panel.hidden&&el.getAttribute('aria-selected')==='true'?'':'!');}`).Str()
	}
	for _, step := range []struct {
		key  input.Key
		want string
	}{{input.ArrowRight, "details"}, {input.End, "connections"}, {input.Home, "overview"}, {input.ArrowLeft, "connections"}} {
		page.Keyboard.MustType(step.key)
		if got := activeTab(); got != step.want {
			t.Fatalf("tab key moved to %q; want %q", got, step.want)
		}
	}
	page.MustEval(`()=>document.querySelector('[data-inspector-tab="details"]').click()`)

	// A TV is offered no MeshCore conversations until two characters are typed.
	page.MustEval(`()=>document.querySelector('[data-action="link-app"]').click()`)
	page.MustWait(`()=>!!document.querySelector('[data-app-picker] [data-app-target][data-app="software-store"]')`)
	if got := page.MustEval(`()=>JSON.stringify({mesh:document.querySelectorAll('[data-app-picker] [data-app-target][data-app="meshcore"]').length,note:document.querySelector('[data-app-picker] [data-picker-note]').textContent})`).Str(); !strings.Contains(got, `"mesh":0`) || !strings.Contains(got, "2") {
		t.Fatalf("a TV must not see MeshCore before typing: %s", got)
	}
	page.MustElement(`[data-app-picker] [data-picker-input]`).MustInput("Ip")
	page.MustWait(`()=>!!document.querySelector('[data-app-picker] [data-app-target][data-app="meshcore"][data-target-id="conv-ipt"]')`)
	page.Keyboard.MustType(input.Escape)

	// A mesh node sees them at once; the tab choice persisted for the kind.
	page.MustEval(`()=>document.querySelector('[data-node="relay"]').click()`)
	page.MustWait(`()=>document.querySelector('[data-inspector] h2')?.textContent==='Garden relay'&&document.querySelector('[data-inspector-tab="details"]').getAttribute('aria-selected')==='true'`)
	page.MustEval(`()=>document.querySelector('[data-action="link-app"]').click()`)
	page.MustWait(`()=>!!document.querySelector('[data-app-picker] [data-app-target][data-app="meshcore"][data-target-id="conv-ipt"]')`)
	page.MustEval(`()=>document.querySelector('[data-app-picker] [data-app-target][data-target-id="conv-ipt"]').click()`)
	page.MustWait(`()=>!!document.querySelector('[data-app-binding="meshcore:conv-ipt"]')&&!document.querySelector('[data-savebar]').hidden&&!!document.querySelector('[data-savebar] .infra-draft-banner')`)

	// Field markers and Ctrl S.
	page.MustEval(`()=>{const field=document.querySelector('[data-profile="vendor"]');field.closest('details').open=true;field.value='Heltec';field.dispatchEvent(new Event('input',{bubbles:true}));field.focus();}`)
	page.MustWait(`()=>document.querySelector('[data-field="profile.vendor"]')?.dataset.dirty==='true'&&!document.querySelector('[data-inspector-tab="details"] .infra-tab-dirty').hidden`)
	page.KeyActions().Press(input.ControlLeft).Type('s').MustDo()
	infraPanelsWait(t, fixture, "the Ctrl S save", func() bool { return len(fixture.nodePuts) == 1 })
	fixture.mu.Lock()
	saved := fixture.nodePuts[0]
	fixture.mu.Unlock()
	profile, _ := saved["profile"].(map[string]any)
	bindings, _ := profile["app_bindings"].([]any)
	if saved["id"] != "relay" || profile["vendor"] != "Heltec" || len(bindings) != 1 {
		t.Fatalf("Ctrl S saved %#v", saved)
	}
	page.MustWait(`()=>document.querySelector('[data-savebar]').hidden&&!document.querySelector('.infra-draft-banner')`)

	// List view sorting.
	page.MustEval(`()=>document.querySelector('[data-view="table"]').click()`)
	page.MustWait(`()=>document.querySelectorAll('[data-table-body] [data-row]').length===5`)
	order := func() string {
		return page.MustEval(`()=>[...document.querySelectorAll('[data-table-body] [data-row]')].map(row=>row.dataset.row).join(',')`).Str()
	}
	if got := order(); got != "alpha,relay,tv,mid,zeta" {
		t.Fatalf("default List order = %s", got)
	}
	page.MustEval(`()=>document.querySelector('[data-sort="name"]').click()`)
	page.MustWait(`()=>document.querySelector('th[data-sort-col="name"]').getAttribute('aria-sort')==='descending'`)
	if got := order(); got != "zeta,mid,tv,relay,alpha" {
		t.Fatalf("descending name order = %s", got)
	}
	page.MustEval(`()=>document.querySelector('[data-sort="address"]').click()`)
	page.MustWait(`()=>document.querySelector('th[data-sort-col="address"]').getAttribute('aria-sort')==='ascending'&&!document.querySelector('th[data-sort-col="name"]').hasAttribute('aria-sort')`)
	if got := order(); got != "alpha,mid,tv,zeta,relay" {
		t.Fatalf("address order = %s; want numeric IPv4 with the address-less asset last", got)
	}

	// Bulk remove and ignore.
	page.MustEval(`()=>{document.querySelector('[data-row-check="alpha"]').click();document.querySelector('[data-row-check="zeta"]').click();}`)
	page.MustWait(`()=>!document.querySelector('[data-bulkbar]').hidden&&document.querySelector('[data-bulk-count]').textContent.includes('2')`)
	page.MustEval(`()=>document.querySelector('[data-action="bulk-ignore"]').click()`)
	infraPanelsConfirm(page)
	infraPanelsWait(t, fixture, "bulk remove and ignore", func() bool { return len(fixture.deleteCalls) == 2 })
	fixture.mu.Lock()
	calls := strings.Join(fixture.deleteCalls, " ")
	fixture.mu.Unlock()
	if !strings.Contains(calls, "alpha?version=1&ignore_sources=1") || !strings.Contains(calls, "zeta?version=1&ignore_sources=1") {
		t.Fatalf("bulk remove and ignore sent %s", calls)
	}
	page.MustWait(`()=>document.querySelectorAll('[data-table-body] [data-row]').length===3`)
	closeInfraBrowserWindow(t, page)
	requireInfraBrowserClean(t, page)
}
```
- [ ] **Step 4: Verify** `go vet ./ui`; `AURAGO_RUN_BROWSER_SMOKE=1 go test ./ui -run 'TestDesktopInfra(ImportAndCleanup|InspectorAndList)Browser' -count=1 -timeout 20m`; if flaky, reproduce with Windows animations on and off; then V-browser.
- [ ] **Step 5: Commit** paths `ui/desktop_infra_browser_test.go ui/desktop_infra_panels_browser_test.go`; subject `test(infra): cover import groups, cleanup, ignore, app picker, save bar and list bulk actions`.

---

## Task 17: Contract notes, final verification, handoff

**Files:** `ui/js/desktop/apps/AGENTS.md`.

- [ ] **Step 1:** `git ls-files -v ui/js/desktop/apps/AGENTS.md` must start with `H ` (else `git update-index --no-assume-unchanged`). Append to the `## Infra` section:
```markdown
- Plan C modules: `infra-picker.js` (ARIA combobox, popover and inline; options carry `data-app-target`,
  `data-node-option`, `data-place-option`), `infra-cleanup.js` (Cleanup/Ignored from `InfraAppModel.infraCleanup`,
  dismissals in `infra.dismissed.v1`), `infra-inspector-forms.js` (Details/Sources forms; all four tab panels are
  rendered and inactive ones hidden; app bindings live in the draft), `infra-import.js` (non-modal import sheet,
  `POST /import/batch`, `discoveryRelevance` groups), `infra-menus.js` (context menus through `ctx.showContextMenu`).
- Snapshots always ask `?include_ignored=1`; ignored sources never count in the ledger and appear only under
  Ignored. Ignore, remove-and-ignore, engine hosts and batch import go through the store, which reloads the
  snapshot and says so on an uncertain outcome. `unplace` intents make library placing undoable (only when the
  snapshot has `source_hosts`). Inspector tab per kind persists in `infra.inspector.v1`, List sort in `infra.table.v1`.
```
- [ ] **Step 2:** V-core, `node scripts/test-infra-assets.mjs`, `node scripts/build-ui-bundles.js --check`, V-browser, `wc -l ui/js/desktop/apps/infra*.js` (each < 1100; record the numbers).
- [ ] **Step 3:** `node C:/Users/Andi/.gitnexus/stores/AuraGo-08496d38b623/checkouts/AuraGo-08496d38b623/run.cjs detect-changes --scope compare --base-ref "main" --repo "C:\\Users\\Andi\\Documents\\repo\\AuraGo"`; attach the summary (partial/truncated is not clean).
- [ ] **Step 4: Commit** path `ui/js/desktop/apps/AGENTS.md`; subject `docs(infra): describe panel, import and cleanup module contracts`.
- [ ] **Step 5: Handoff** for the single combined review (Critical/Important only): Task 0 notes, every HIGH/CRITICAL impact warning (Task 5 `renderAppContent`), the known limitations from §1, line counts, screenshots in `reports/infra-ui-review/`.

---

## Spec coverage (self-review)

| Spec item | Task |
|---|---|
| §7 To place / On the map, place-row drop targets, section `+` | 7 |
| §7 Needs attention (any source, failed saves) | 2, 7 |
| §7 Cleanup with one primary action + Dismiss (`infra.dismissed.v1`) | 1, 7 |
| §7 Ignored with Restore | 4, 7 |
| §8 header (icon picker, inline name, breadcrumb, pills, quick action, ⋯) | 9, 10, 15 |
| §8 tabs with arrow keys, persisted per kind | 2, 10, 16 |
| §8 grouped Details, collapsed empty groups, save bar, Ctrl S, dirty markers | 2, 9, 10, 16 |
| §8 separate `<form data-source-form>` | 9 |
| §8 app links picker with MeshCore rule | 2, 6, 9, 16 |
| §8 connection inspector, multi-selection summary and bulk actions | 10 |
| §9.1 guests without host → Cleanup, Assign engine host (`PUT /source-hosts`) | 1, 4, 7, 16 |
| §9.2 `discoveryRelevance`, `infraCleanup`, import sheet groups, Import/Link/Ignore, drag to map with placement + parent | 1, 4, 12, 16 |
| §9.3 ignore (`PUT /sources/ignore`), remove and ignore (`DELETE ?ignore_sources=1`) | 4, 10, 15, 16 |
| List view: sortable columns, place column, guest tree, checkboxes + bulk bar, 54 px rows | 2, 14, 16 |
| Context menus via `showContextMenu` on cards, regions, edges, library and table rows | 5, 15, 16 |
| Filter popover + removable chips, legend, shortcut sheet (`?`) | 15 |
| "Remove from Infra" confirm (Delete key on canvas, library, List) | 7, 10, 14 |
| Plan A amendment `unplace` (undo placing, Remove from map) | 2, 4, 7, 15 |
| 16 locales, no literal fallbacks, tokens only, reduced motion, no external assets, no `cc-` | 3, 8, 11, 13, every CSS step |
