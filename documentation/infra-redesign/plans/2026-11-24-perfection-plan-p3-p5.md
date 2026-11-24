# Infra perfection plan: packages 3 and 5

Worktree: `C:\Users\Andi\Documents\repo\AuraGo\.worktrees\infra-perfection`. Every line number below was checked against the current files.

## A. Files and headroom

| File | Now | Plan adds (net) |
|---|---|---|
| ui/js/desktop/apps/infra-import.js | 418 | about +45 |
| ui/js/desktop/apps/infra-dialogs.js | 429 | about +40 |
| ui/js/desktop/apps/infra-menus.js | 239 | about +8 |
| ui/js/desktop/apps/infra-cleanup.js | 173 | about +8 |
| ui/js/desktop/apps/infra-store.js | 879 | about +14 |
| ui/js/desktop/apps/infra.js | 729 | about +35 |
| ui/js/desktop/apps/infra-model.js | 951 | about +22 (7 glyphs, sourceLabels, history peek) |
| ui/js/desktop/apps/infra-gestures.js | 918 | 0 (one line changed, optional, see 3.15) |
| ui/css/desktop-app-infra.css | 706 | about +35 / −40 |
| ui/desktop_infra_browser_test.go (fixture) | – | +6 |

Every file stays far below 1100 lines. You do not need a new module.

## Baseline failure: TestDesktopInfraMenusAndShortcutsBrowser

The test is at `ui\desktop_infra_panels_browser_test.go:278`. It checks these things in order:
- the card menu ids are `open,move,connect,unplace,remove,remove-ignore`, then Remove from map and Undo (303–319);
- the place menu ids are `open,rename,move,unplace,remove`, and Rename puts the inspector into rename mode (325–331);
- the edge menus read `edit,remove` (manual connection) and `edit!,remove!` with a hint (source connection) (338–349);
- the empty map menu reads `add,add-place,import,fit` and Import opens the sheet (357–363);
- Shift+F10 on a library row gives `open,move,connect,remove` and opens exactly one menu (368–374);
- `?` typed in the search stays text, and `?` on the map opens the shortcut sheet with 3 sections and more than 20 keycaps (377–394);
- More > Legend gives the counts 5,3,3,5, shows the note, and gives focus back on Escape (397–411);
- filter chips (414–425);
- at 1000 px the bulk bar is not covered (428–441).

The helper `infraPanelsMenu` (line 77) lists **every** `.vd-context-menu [data-context-action]`. The shell (`ui\js\desktop\core\menus-and-routing.js:392–409`) also renders submenu children with `data-context-action="<parent>/<child>"` inside `.vd-context-submenu-popover`.

Reading `infra-menus.js`, I expect the test to fail first at line 325. In the Map, `placeItems` (78–85) adds the `arrange-contents` and `color` submenus, so the helper returns `open,rename,move,unplace,arrange-contents/arrange-connections,…,color/color-none,…,remove`. It would fail again at line 357, because `canvasItems` (112) adds the `arrange-auto` submenu. The expected strings were most likely written before infra-arrange.js added those submenus. I did not run the test, so the developer must confirm the first failing line.

Fix it in the helper so that it counts only top-level items:
```go
return page.MustEval(`()=>[...document.querySelectorAll('.vd-context-menu [data-context-action]')].filter(b=>!b.closest('.vd-context-submenu-popover')).map(button=>button.dataset.contextAction+(button.disabled?'!':'')).join(',')`).Str()
```
With this filter, all five expected strings stay valid, both before and after 3.12. Do this first, in the 3.11/3.12 batch.

## B. Order of work

Package 3:
1. **Store and model first.** Do 3.14, 3.15, the store half of 3.13, and the `createHistory` peek in infra-model.js. These are pure and covered by the Node tests.
2. **3.2** (sourceLabels in infra-model.js and its caller in infra-import.js).
3. **Import sheet: 3.1, 3.3 and the import part of 3.5 together.** All three touch `rowHTML`, `render` and `load` in infra-import.js. Do 5.3 and 5.4 for candidates in the same pass, because 3.1 rewrites the candidate checkbox markup.
4. **Dialogs: 3.4, 3.6, 3.7, 3.8, 3.9 and the dialog part of 3.5 together.** They are all in infra-dialogs.js, and 3.7 needs a `caps` route in infra.js.
5. **Cleanup:** 3.10 and the cleanup part of 3.5 (infra-cleanup.js).
6. **Menus: 3.11, 3.12, the infra.js half of 3.13, and the baseline helper fix.** 3.11 and 3.13 both edit `paintShell` and `paintArrange`.

Package 5, after package 3:
- 5.5 first: it deletes lines that 5.6 would otherwise edit.
- Then 5.6, 5.1, 5.2.
- 5.3 and 5.4 are done in step 3 above.

Cross-package overlaps:
- **2.15 (package 2)** relabels Arrange inside `paintArrange` in infra.js. 3.11 adds a `.infra-menu-label` span; 2.15 must write its label there.
- **Package 4** owns the table rules. 5.3/5.4 rewrite `.infra-col-check input` (CSS 622, 657–658); agree who lands first.
- **Package 1 (1.17)** edits CSS line 151, and 5.6 leaves line 151 alone.
- 3.15 optionally changes one line of infra-gestures.js (package 2).

## C. Items

### 3.1 Compact discovery rows
Verdict: confirmed. The row always renders the bind field and the actions (`rowHTML`, infra-import.js 133–146). An empty section is already `hidden` (183), so that part needs no change.
Touch: infra-import.js → `rowHTML` (133), `render` (186), `focusSnapshot`/`restoreFocus` (148–160), `onClick` (325), `onPointerDown` (374), `prune` (112), `open`/`close` (220/234); CSS 282–294, 591–602.
Do:
1. Add `const opened = new Set(), shut = new Set();` next to `errors`. Clear both in `open()` and `close()`, and prune ids that are gone in `prune()`.
2. Add these helpers:
```js
const startsOpen = (id, group) => group === 'duplicate' || errors.has(id);
const isOpen = (id, group) => (startsOpen(id, group) ? !shut.has(id) : opened.has(id));
function setOpen(id, group, open) { opened.delete(id); shut.delete(id); if (open !== startsOpen(id, group)) (open ? opened : shut).add(id); }
```
3. Change `rowHTML(candidate, group, relevance, index)` (the render map passes `i`). Set `const detailId = `infra-cand-${esc(S)}-${group}-${index}`, open = isOpen(candidate.id, group);`. Markup order:
   - `<li class="infra-candidate" data-candidate-row data-group …>`
   - `<label class="infra-candidate-check"><input type="checkbox" data-candidate=… aria-label=…></label>`. There is no `.infra-checkbox-mark` any more (5.4).
   - the icon
   - `<div class="infra-candidate-copy"><strong/><span>source</span><small>detail</small></div>`. The reason leaves the copy.
   - `<button type="button" class="infra-cmd-icon infra-candidate-toggle" data-candidate-toggle="${id}" aria-expanded="${open}" aria-controls="${detailId}" aria-label="${esc(fill(t('desktop.infra_candidate_options'),{name}))}" title=…>${app.glyph('chevron')}</button>`
   - `<div class="infra-candidate-detail" id="${detailId}" data-candidate-detail${open ? '' : ' hidden'}>` + reason `<small class="infra-candidate-reason" data-candidate-reason>` + the unchanged bind `<label>` + `.infra-candidate-actions` + `</div>`
   - the error `<p data-candidate-error>` stays **outside** the detail.
4. In `onClick`, handle the toggle before `one`. Update the DOM in place without re-rendering, so focus stays on the button:
```js
const disclose = target.closest('[data-candidate-toggle]');
if (disclose) { const row = disclose.closest('[data-candidate-row]'), open = disclose.getAttribute('aria-expanded') !== 'true';
  setOpen(disclose.dataset.candidateToggle, row.dataset.group, open); disclose.setAttribute('aria-expanded', String(open)); row.querySelector('[data-candidate-detail]').hidden = !open; return; }
```
5. Add `'[data-candidate-toggle]'` to `CONTROLS`. In `restoreFocus`, skip hidden controls:
```js
const visible = sel => [...(next?.querySelectorAll(sel) || [])].find(el => !el.disabled && !el.closest('[hidden]'));
const control = visible(snapshot.selector) || visible('[data-candidate-toggle]') || next?.querySelector('[data-candidate]');
```
6. In `onPointerDown`, add `[data-candidate-detail]` to the excluded selector.
7. CSS:
   - `.infra-import-group .infra-candidate { grid-template-columns: auto 34px minmax(0,1fr) auto; padding: 7px 6px 7px 8px; }`
   - `.infra-candidate-copy { display: grid; grid-template-columns: auto minmax(0,1fr); }`
   - `.infra-candidate-copy strong { grid-column: 1 / -1; }`
   - `.infra-candidate-copy > small:not(:empty)::before { content: " · "; }`
   - `.infra-candidate-toggle { width: 32px; height: 32px; }`
   - `.infra-candidate-toggle[aria-expanded="true"] .infra-glyph { transform: rotate(180deg); }`
   - `.infra-candidate-detail { grid-column: 1 / -1; display: grid; gap: var(--ds-space-1); }`
   - In 598, replace `.infra-import-group .infra-bind-field, .infra-candidate-actions` with `.infra-candidate-detail`.
   - Rename 599 to `.infra-candidate-reason`.
   - Add the chevron transform to the reduced-motion transition list (442).

   Expected collapsed height: 34 px icon, or 2 text lines of about 39 px, plus 14 px padding and 2 px border, about 55 px.

Tests:
(a) I expect no existing assertion to break. Every test drives the hooks through JS (`.click()`, `value=`, `dispatchEvent`), and those still work on hidden elements: desktop_infra_browser_test.go 509/513–518, desktop_infra_import_browser_test.go 70–71, 94–98, 193. The mouse drags start on `.infra-candidate-copy strong`, which stays visible (138, 156, 194). The error row at 518 stays visible because the error sits outside the detail.
(b) Add to TestDesktopInfraImportSheetBrowser after line 76 (`want` check):
```
()=>{const r=document.querySelector('[data-candidate-row="reg-printer"]'),d=document.querySelector('[data-candidate-row="fritz-nas"]');return r.getBoundingClientRect().height<=64&&r.querySelector('[data-candidate-detail]').hidden&&r.querySelector('[data-candidate-toggle]').getAttribute('aria-expanded')==='false'&&!d.querySelector('[data-candidate-detail]').hidden}
```
Keys: infra_candidate_options | Optionen für „{name}“ | Options for “{name}”
Risk: focus can land on a hidden control after an import or ignore; step 5 covers that. Rows in the folded "unlikely" group now show their reason only when opened.

### 3.2 Source names
Verdict: adjusted. The API carries no human-readable instance name: `instance` is `docker_<sha256>` (internal/server/desktop_infra_sources.go 481–504), and `Source.name` is the container name. The only name available on the client is the device that runs a Docker engine (`state.sourceHosts[].node_id || inferred_node_id`). Proxmox and FRITZ!Box keep their numbers. `sourceLabels` is used only in infra-import.js (67, 207).
Touch: infra-model.js → `sourceLabels` (719); infra-import.js → `labels` (67, 207), `render` (174).
Do:
1. Change the signature to `sourceLabels(sources, kindLabel, nameOf = () => '')`. For each numbered kind, build `names: Map<kind\x1finstance, name>` from `String(nameOf(kind, instance) || '').trim()`. Keep a name only if it is unique within its kind; otherwise use the number.
2. In `of()`: when a name exists, return `{ text: `${base} · ${short(name)}`, title: `${base} · ${name}` }`, where `short = n => { const c = [...n]; return c.length > 22 ? c.slice(0, 22).join('') + '…' : n; }`. Otherwise keep today's text and title.
3. In infra-import.js, add
```js
const engineName = (kind, instance) => { const host = state.sourceHosts.find(h => h.kind === kind && h.instance === instance); return store.nodeById(host?.node_id || host?.inferred_node_id)?.name || ''; };
```
   Then move `labels = model.sourceLabels(sources, kindLabel, engineName)` to the start of `render()`. The source list is small, and `source-hosts` events already call `importer.render()`.
4. Rewrite the comment above `sourceText` (72–73). Change the AGENTS.md D-fix bullet (line 350) so it says a numbered kind shows the engine host's name when one is known.
Tests:
(a) None. TestDesktopInfraImportSourceChipsBrowser (257, 263) has no source hosts, so it keeps its numbers.
(b) Add to scripts/test-infra-ui.mjs after line 516:
```js
const named = model.sourceLabels(sources, kindLabel, (kind, instance) => (instance === 'b9f3c2d1e8a74f6b' ? 'a-very-long-docker-host-name' : ''));
assert.equal(named.of(sources[0]).text, 'Docker · a-very-long-docker-hos…', 'a named instance shows its host, shortened after 22 characters');
assert.equal(named.of(sources[0]).title, 'Docker · a-very-long-docker-host-name');
assert.equal(named.of(sources[2]).text, 'Docker 1', 'an instance without a name keeps its number');
```
Keys: none.
Risk: a stale engine whose host was removed falls back to a number. That is intended.

### 3.3 Import load failure
Verdict: confirmed. The `catch` at 215 only toasts, and `emptyEl.hidden = loading || shown > 0` (192) then shows "no discoveries".
Touch: infra-import.js → markup (49–53), `load` (199–219), `render` (191–192), `onClick`; CSS next to `.infra-empty-candidates`.
Do:
1. Add markup after the empty block: `<div class="infra-import-error" data-import-error hidden><p data-import-error-text></p><button type="button" class="infra-button" data-action="retry-import">${t('desktop.infra_retry')}</button></div>`. Add `let loadError = ''`.
2. In `load`: on success set `loadError = ''`. In the catch, set `if (!preserve) loadError = t('desktop.infra_candidates_failed');` and keep the existing `store.notify(...)`. That keeps the feedback rule.
3. In `render`, set `errorEl.hidden = loading || !loadError; errorText.textContent = loadError; emptyEl.hidden = loading || !!loadError || shown > 0;`.
4. In `onClick`, handle `[data-action="retry-import"]` with `loadError = ''; loading = true; render(); void load(false);`. Focus the search only if focus was on the retry button.
5. Clear `loadError` in `open()` and `close()`.
6. CSS: `.infra-import-error { display: grid; justify-items: center; gap: var(--ds-space-2); padding: 24px 10px; color: var(--infra-bad-text); text-align: center; }`.
Tests:
(a) None.
(b) Fixture change in ui/desktop_infra_browser_test.go: add the field `failCandidatesLoad int`. In the `/candidates` case (195), add `if f.failCandidatesLoad > 0 { f.failCandidatesLoad--; write(http.StatusInternalServerError, map[string]any{"error": "infra_error"}); return }`; use `break` instead of `return` if `return` does not fit the switch. In TestDesktopInfraImportSourceChipsBrowser, set `fixture.failCandidatesLoad = 1`. After line 249, wait for `!document.querySelector('[data-import-error]').hidden&&document.querySelector('[data-import-empty]').hidden`, then click `[data-action="retry-import"]`. Lines 250 onward run unchanged.
Keys: infra_candidates_failed | Funde konnten nicht geladen werden | Discoveries could not be loaded
Risk: a background `reload()` (preserve) failure must not hide the rows, which is why step 2 guards it.

### 3.4 Create dialog
Verdict: confirmed. The default title is `infra_new_node` ("Infrastruktur-Asset hinzufügen"), the submit is `infra_create_node`, `if (!body.name) return;` (83) is silent, and the h2 has `margin: 0 36px 14px 0` (CSS 266).
Touch: infra-dialogs.js → `openNodeDialog` (70–79), `createNode` (80–96); CSS 266.
Do:
1. Title: `title || t(model.isContainer(kind) ? 'desktop.infra_new_place' : 'desktop.infra_new_asset')`. Both keys exist.
2. Submit: change the **value** of `infra_create_node` in all 16 files to "Erstellen" / "Create". It is used only here.
3. Add `novalidate` to the form. Under the name input add `<p class="infra-field-error" id="infra-new-name-error-${S}" data-name-error role="alert" hidden>${t('desktop.infra_name_required')}</p>`.
4. In the submit handler, before `createNode`:
```js
const input = form.elements.name;
if (!input.value.trim()) { error.hidden = false; input.setAttribute('aria-invalid', 'true'); input.setAttribute('aria-describedby', error.id); input.focus(); return; }
```
   On `input`, clear the error. Leave the guard in `createNode` as it is; the Node tests call it with a Map.
5. Success toast: `t(model.isContainer(body.kind) ? 'desktop.infra_place_created' : 'desktop.infra_node_created')`.
6. CSS 266: `margin: 0 0 14px; padding-right: 32px;`, so the border runs the full width and the text clears the close button. Add `.infra-field-error { margin: 4px 0 0; color: var(--infra-bad-text); font-size: 14px; }`.
Tests:
(a) None. desktop_infra_browser_test.go:502 fills a name.
(b) In TestDesktopInfraAppBrowser before line 502:
```
()=>{const f=document.querySelector('[data-new-node]');f.elements.name.value='   ';f.requestSubmit();return !document.querySelector('[data-name-error]').hidden&&document.activeElement===f.elements.name}
```
Keys:
- infra_create_node (changed) | Erstellen | Create
- infra_name_required | Gib einen Namen ein | Enter a name
- infra_place_created | Ort erstellt | Place created

`infra_new_node` becomes unused; package 6 can remove it.
Risk: the browser's own required bubble is gone (novalidate), so our message replaces it.

### 3.5 Visible feedback
Verdict: confirmed. These are all the mutations that only call `app.announce`, from a grep over `infra*.js` (package 3 files):

| Where | Result | Toast per rule |
|---|---|---|
| infra-dialogs.js 146 `deleteEdge` (dialog) | connection removed | yes, success |
| infra-dialogs.js 252 `removeEdge` (menu, inspector) | connection removed | yes, success |
| infra-dialogs.js 227 `deleteNodes` partial | partial deletion | yes, info (the refusal already toasted) |
| infra-dialogs.js 241 `removeAndIgnore` partial | partial deletion | yes, info |
| infra-dialogs.js 202 popover create | connection appears where the drag ended | **no**: under the pointer, announce only |
| infra-import.js 275 import partial | failure | yes, error (not named in the spec, but the rule covers "any failure") |
| infra-import.js 299 `ignoreCandidates` | ignore | yes, success |
| infra-cleanup.js 98 `removeDuplicates` | removal and ignore | yes, success |
| infra-cleanup.js 114 engine host | background save | yes, success |
| infra-cleanup.js 123 guest host | background save | yes, success |
| infra-cleanup.js 135 restore | restore | yes, success |

Out of scope (packages 1 and 2): infra-arrange.js 249/330/356, infra-gestures.js (moves under the pointer, modes), infra-inspector.js 301.
Touch: the lines above.
Do:
1. Replace each `app.announce(x)` marked "yes" with `store.notify(x, kind)`. `notify` → `toast()` (infra.js 140), which also announces, so do not call both. Use `store.notify`, not `app.toast`: the Node test's app has no `toast`.
2. Place wording in `deleteNodes`: `const places = list.every(id => model.isContainer(store.nodeById(id).kind));`. Then the title and confirm label become `t(places ? 'desktop.infra_remove_place' : 'desktop.infra_remove_node')`, and a single place removed shows `infra_place_removed`. Also use `infra_remove_place` as the label of the place menu's `remove` item (infra-menus.js 86), keeping id `remove`.
Tests:
(a) scripts/test-infra-ui.mjs:1345: change the expectation to `['desktop.infra_error_infra_readonly', 'desktop.infra_node_removed']`, because the partial deletion now notifies.
(b) Add to the same block: `assert.equal(notices.at(-1), 'desktop.infra_node_removed', 'a partial deletion shows a toast')` (folded into the change above).
Keys:
- infra_remove_place | Ort entfernen | Remove place
- infra_place_removed | Ort entfernt | Place removed
Risk: error toasts never auto-dismiss (infra.js 150). A partial-import error toast stays until dismissed; none of the browser tests that follow clicks with the mouse at its spot.

### 3.6 Backdrop click
Verdict: confirmed (infra-dialogs.js 45). It also closes on clicks in the dialog's own padding, because those also have `target === dialog`.
Touch: infra-dialogs.js → `openDialog` (37–48).
Do:
```js
delete dialog.dataset.touched;
dialog.oninput = () => { dialog.dataset.touched = 'true'; };
dialog.onclick = event => {
  if (event.target !== dialog || dialog.dataset.touched === 'true') return;
  const r = dialog.getBoundingClientRect();
  if (event.clientX < r.left || event.clientX > r.right || event.clientY < r.top || event.clientY > r.bottom) dialog.close();
};
```
Escape and Cancel stay unchanged.
Tests:
(a) None.
(b) In TestDesktopInfraAppBrowser after the 3.4 check: dispatch `input` on the name field, then `d.dispatchEvent(new MouseEvent('click',{bubbles:true,clientX:2,clientY:2}))`, and assert `document.querySelector('[data-dialog]').open`.
Keys: none.
Risk: in the PDF dialog a radio change counts as touched, so the backdrop no longer closes it after a change. That is acceptable.

### 3.7 Read-only while a dialog is open
Verdict: confirmed. `createNode` and `saveEdge` return silently on `!canWrite()` (81, 120).
Touch: infra-dialogs.js → node and edge form markup (72, 110), `onBusy` (408–413), a new `onCaps`; infra.js → `route` `caps` (639–643).
Do:
1. Add `data-write-form` and `<p class="infra-readonly-note" data-dialog-readonly hidden>${t('desktop.infra_dialog_readonly')}</p>` (above `.infra-form-actions`) to `[data-new-node]` and `[data-edge-form]`.
2. Add the function:
```js
function onCaps() { const dialog = dialogEl(); const write = canWrite();
  dialog?.querySelectorAll('[data-write-form]').forEach(form => { form.querySelector('[type="submit"]').disabled = !write || state.busy.has(form.matches('[data-new-node]') ? 'create' : `edge-${form.elements.id?.value || 'new'}`);
    form.querySelector('[data-dialog-readonly]').hidden = write; form.querySelector('[data-delete-edge]')?.toggleAttribute('disabled', !write); });
  popover?.querySelector('[type="submit"]')?.toggleAttribute('disabled', !write); }
```
   In `onBusy`, write `submit.disabled = busy || !canWrite()`. Call `onCaps` at the end of `openDialog` and export it.
3. In infra.js `case 'caps'`, add `dialogs.onCaps();`.
Tests:
(a) None.
(b) Last step of TestDesktopInfraAppBrowser, before `closeInfraBrowserWindow`: open the create dialog, run `document.dispatchEvent(new CustomEvent('aurago:desktop-policy',{detail:{readonly:true}}))`, then assert that the submit is disabled and `[data-dialog-readonly]` is visible. It has to be the last step, because polling pauses afterwards.
Keys: infra_dialog_readonly | Nur Lesezugriff: Speichern ist gerade nicht möglich | Read-only access: saving is not possible right now
Risk: `onBusy` and `onCaps` must agree; both use `busy || !write`.

### 3.8 PDF export failure
Verdict: confirmed. A `TypeError("Failed to fetch")` reaches `app.toast(error?.message …)` (394).
Touch: infra-dialogs.js → PDF submit (376–394).
Do: replace 394 with `app.toast(t('desktop.infra_export_pdf_failed'), 'error');`. The server detail (378–383) becomes dead; simplify it to `if (!response.ok) throw Object.assign(new Error('pdf'), { status: response.status });`.
Tests:
(a) None.
(b) In TestDesktopInfraMenusAndShortcutsBrowser after the legend block (411):
   1. Override `window.fetch` so it rejects for `/export/pdf` with `new TypeError('Failed to fetch')`.
   2. Click More > PDF, then `[data-pdf-create]`.
   3. Wait for `[data-toast]` text `=== w('desktop.infra_export_pdf_failed')`.
   4. Restore fetch and close the dialog.
Keys: none (6.2 may drop the full stop of `infra_export_pdf_failed`).
Risk: the server's specific message is lost. The spec asks for that.

### 3.9 Power actions
Verdict: confirmed (infra-dialogs.js 302–308). `t('…status_<x>')` returns the raw key for an unknown status, and the review panel prints the raw status (267).
Touch: infra-dialogs.js → `runAction` (300–309), `openActionMenu` (267).
Do:
```js
const label = t(`desktop.infra_action_${action}`), who = store.nodeById(id)?.name || '';
if (['running', 'accepted'].includes(result.status)) { state.actionBlocks.set(blockKey, { status: result.status, request_id }); store.notify(model.fill(t('desktop.infra_action_toast_running'), { name: who, action: label }), 'info'); … reopen as today; return; }
if (result.status === 'unknown') { …as today… }
const key = { completed: 'desktop.infra_action_toast_done', failed: 'desktop.infra_action_toast_failed' }[result.status];
store.notify(key ? model.fill(t(key), { name: who, action: label }) : model.fill(t('desktop.infra_action_status_generic'), { status: String(result.status || '') }), result.status === 'completed' ? 'success' : 'error');
```
In the review panel, use `t(`desktop.infra_action_status_${status}`)` instead of the raw status. Those keys exist for running, accepted and unknown.
Tests:
(a) None: test-infra-ui.mjs 1357–1378 asserts only the readonly and unavailable notices.
(b) After line 1377: `assert.equal(third.notices.at(-1), 'desktop.infra_action_toast_done')`. Add a fourth setup that answers `{ status: 'weird' }` and expects `'desktop.infra_action_status_generic'`.
Keys:
- infra_action_toast_running | {name}: {action} wird ausgeführt… | {name}: {action} is running…
- infra_action_toast_done | {name}: {action} abgeschlossen | {name}: {action} completed
- infra_action_toast_failed | {name}: {action} fehlgeschlagen | {name}: {action} failed
Risk: running and accepted still block a retry until the review step is passed; only the wording changes.

### 3.10 Restore ignored
Verdict: confirmed. `restoreSource` (infra-cleanup.js 131–138) has no in-flight guard. `setSourcesIgnored` checks no busy key.
Touch: infra-cleanup.js → `restoreSource`, `renderIgnored` (84).
Do:
1. Add `const restoring = new Set();`.
2. In `restoreSource`, start with `if (restoring.has(identity) || state.busy.has('ignore')) return; restoring.add(identity); app.library.render();`. Wrap the request in `try { … } finally { restoring.delete(identity); app.library.render(); }`.
3. In `renderIgnored`, disable the button when `!write || restoring.has(identity)`, and add `aria-busy="true"` while it is restoring.
Tests:
(a) None.
(b) TestDesktopInfraCleanupAndIgnoreBrowser (panels test line 89 onward), at its Restore step: call `b.click(); b.click();` in one eval, then assert that `fixture.ignoreCalls` grew by exactly one `ignored:false` body.
Keys: none.
Risk: none worth noting.

### 3.11 Toolbar menus
Verdict: confirmed. infra.js 73–85 are plain text buttons. In the List, `paintArrange` checks the atlas, and legend is always enabled.
Touch: infra.js → menu markup (73–85), `paintArrange` (267–275), `syncMenus` (405); infra-model.js → `GLYPHS` (569); infra-menus.js → `canvasItems` `fit` (114); CSS 76–78.
Do:
1. Add glyphs to infra-model.js `GLYPHS`:
```js
refresh: '<path d="M13 6a5.2 5.2 0 1 0 .2 3M13 2.5V6H9.5"/>',
arrange: '<rect x="2.5" y="2.5" width="4.5" height="4.5" rx="1"/><rect x="9" y="2.5" width="4.5" height="4.5" rx="1"/><rect x="2.5" y="9" width="4.5" height="4.5" rx="1"/><rect x="9" y="9" width="4.5" height="4.5" rx="1"/>',
keyboard: '<rect x="1.5" y="4" width="13" height="8" rx="1.5"/><path d="M4 7h.01M6.5 7h.01M9 7h.01M11.5 7h.01M5 9.5h6"/>',
legend: '<path d="M6 4.5h7.5M6 8h7.5M6 11.5h7.5"/><circle cx="3" cy="4.5" r=".9" fill="currentColor"/><circle cx="3" cy="8" r=".9" fill="currentColor"/><circle cx="3" cy="11.5" r=".9" fill="currentColor"/>',
file: '<path d="M9.5 2H4.5a1 1 0 0 0-1 1v10a1 1 0 0 0 1 1h7a1 1 0 0 0 1-1V5z"/><path d="M9.5 2v3h3"/>',
place: '<path d="M2 4.5a1 1 0 0 1 1-1h3.2l1.5 1.6H13a1 1 0 0 1 1 1V12a1 1 0 0 1-1 1H3a1 1 0 0 1-1-1z"/>',
import: '<path d="M8 2.5v7.5M4.8 7L8 10.2 11.2 7"/><path d="M2.5 11.5v1a1 1 0 0 0 1 1h9a1 1 0 0 0 1-1v-1"/>',
```
2. Menu markup. Define `const item = (action, g, label, extra = '') => `${glyph(g)}<span class="infra-menu-label">${esc(label)}</span>${extra}``. Add `<div class="infra-menu-sep" role="separator"></div>` between the groups.
   - Add menu: place, import.
   - More menu, in this order: refresh | arrange, legend | shortcuts (with `<kbd class="infra-menu-kbd" aria-hidden="true">?</kbd>`) | export-pdf | `[data-overflow-only]` separator, then undo and redo (glyphs undo and redo).

   Note: legend moves directly after arrange.
3. `paintArrange`:
   - `const list = state.view === 'table'; const blocked = list || …existing…;`
   - set the label on `button.querySelector('.infra-menu-label') || button`;
   - title: `list ? t('desktop.infra_map_only') : (existing topology title)`.
   - Also handle `const legend = q('[data-menu="more"] [data-action="legend"]'); legend.disabled = list; legend.title = list ? t('desktop.infra_map_only') : '';`.
4. In `syncMenus` Arrange, set `disabled: … || state.view === 'table'`.
5. In the canvas context menu `fit` item, add `shortcut: 'F'` (see D).
6. CSS:
   - `.infra-menu [role="menuitem"] { gap: var(--ds-space-2); }`
   - `.infra-menu [role="menuitem"] .infra-glyph { color: var(--vd-theme-muted); }`
   - `.infra-menu-label { flex: 1; min-width: 0; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }`
   - `.infra-menu-kbd { padding: 1px 6px; border: 1px solid var(--infra-line); border-radius: var(--ds-radius-xs); color: var(--vd-theme-muted); font: 600 12px/1.3 var(--vd-font-ui); }`
   - `.infra-menu-sep { height: 1px; margin: var(--ds-space-1) var(--ds-space-2); background: var(--infra-line); }`

   Keyboard navigation (infra.js 448) already skips non-menuitems.
Tests:
(a) desktop_infra_topology_browser_test.go:187 compares `textContent.trim()` with the label. That still passes, because SVG has no text. Also apply the baseline helper fix (see "Baseline failure").
(b) TestDesktopInfraMenusAndShortcutsBrowser, after the List switch (431): open More and assert `arrange` and `legend` are `.disabled` with `title===w('desktop.infra_map_only')`, and that `[data-menu="more"] [data-action="shortcuts"] .infra-menu-kbd` reads `?`. Also add the new glyph names to the explicit list in test-infra-ui.mjs:2060, because infra.js calls a bare `glyph()` that the regex does not see.
Keys: infra_map_only | Nur in Karte und Topologie | Only in Map and Topology
Risk: 2.15 also writes the Arrange label; both must use `.infra-menu-label`.

### 3.12 Card context menu
Verdict: confirmed (infra-menus.js 54, 65–66; power actions exist only in `openNodeMenu` 186). The shell supports `icon`, `shortcut`/`shortcutHint`, `disabled` + `disabledHint`, `danger`, `items`, and separators (`menus-and-routing.js` 31–40, 387–410). Icons from the shell's MINI_SYMBOLS list always render: play, stop, refresh, eye, eye-off, x.
Touch: infra-menus.js → `assetItems` (47–68), `placeItems` (74), `sourceActionItems` (118–128), `openNodeMenu` (186).
Do:
1. Label `open` (both builders): `t('desktop.infra_show_details')`. Keep id `open` and icon `info`.
2. Add `const POWER_ICONS = { start: 'play', stop: 'stop', restart: 'refresh', reboot: 'refresh', shutdown: 'stop', wake: 'play' };` and use `icon: POWER_ICONS[action] || 'settings'` in `sourceActionItems`.
3. In `assetItems`, after the unplace item: `if (single) items.push(...sourceActionItems(node, anchor));`. It brings its own leading separator and returns nothing without `canControl`.
4. Remove item: `shortcut: t('desktop.infra_kbd_delete')` (assets and places). Remove-and-ignore item: `icon: 'eye-off'`.
5. In `openNodeMenu`: use `const items = itemsFor(id, button, { inspector: true });`, and append `sourceActionItems` only when `model.isContainer(node.kind)`, so assets do not get the actions twice.
Tests:
(a) None: the id strings at panels test 303/325/368 are unchanged (dev-a has only a registry source, so no power group).
(b) At panels test 303, while the card menu is open:
```
()=>{const m=document.querySelector('.vd-context-menu'),icon=a=>m.querySelector('[data-context-action="'+a+'"] .vd-context-icon').innerHTML;return m.querySelector('[data-context-action="open"] .vd-context-label').textContent===w('desktop.infra_show_details')&&m.querySelector('[data-context-action="remove"] .vd-context-shortcut')?.textContent===w('desktop.infra_kbd_delete')&&icon('remove')!==icon('remove-ignore')}
```
Keys: infra_show_details | Details anzeigen | Show details
Risk: the shell's read-only guard does not know Infra ids; the existing `write()` wrapper still disables writes.

### 3.13 Undo labels
Verdict: confirmed. infra.js 289–291 uses the static `infra_undo` / `infra_redo`. Command `label`s are announcement sentences, so map the command `type` to a word instead.
Touch: infra-model.js `createHistory` (529–541); infra-store.js `api` (864–874); infra.js `paintShell` (286–291).
Do:
1. In `createHistory`, add `peekUndo: () => undo.at(-1) ?? null, peekRedo: () => redo.at(-1) ?? null`.
2. In the store, add `const nextHistory = (view, dir) => (dir === 'undo' ? histories[view]?.peekUndo() : histories[view]?.peekRedo())?.type || '';` and export it.
3. In infra.js:
```js
const HISTORY_WORDS = { MoveNodes: 'desktop.infra_history_move', Reparent: 'desktop.infra_history_move', SetParent: 'desktop.infra_history_move', ResizePlace: 'desktop.infra_history_resize', PlaceFromLibrary: 'desktop.infra_history_place', CreateEdge: 'desktop.infra_history_connect', DeleteEdge: 'desktop.infra_remove_connection' };
const historyWord = (type, view) => t(type === 'Unplace' ? (view === 'topology' ? 'desktop.infra_topology_auto_place' : 'desktop.infra_remove_from_map') : HISTORY_WORDS[type] || 'desktop.infra_history_change');
const historyTitle = (dir, view) => model.fill(t(`desktop.infra_${dir}_named`), { action: historyWord(store.nextHistory(view, dir), view), keys: dir === 'undo' ? `${t('desktop.infra_kbd_ctrl')}+Z` : `${t('desktop.infra_kbd_ctrl')}+${t('desktop.infra_kbd_shift')}+Z` });
```
   In `paintShell`: `undo.title = canUndo ? historyTitle('undo', history) : t('desktop.infra_undo_none'); undo.setAttribute('aria-label', canUndo ? undo.title : t('desktop.infra_undo'));`. Do the same for redo.
Tests:
(a) None (no test pins these titles).
(b) In test-infra-ui.mjs, in the block at 611 after the `dispatch`: `assert.equal(store.nextHistory('atlas', 'undo'), 'Reparent', 'the undo button can name the step');`.
Keys:
- infra_undo_named | Rückgängig: {action} ({keys}) | Undo: {action} ({keys})
- infra_redo_named | Wiederholen: {action} ({keys}) | Redo: {action} ({keys})
- infra_history_move | Verschieben | Move
- infra_history_resize | Größe ändern | Resize
- infra_history_place | Platzieren | Place
- infra_history_connect | Verbindung erstellen | Create connection
- infra_history_change | Änderung | Change
Risk: the title repaints only on `history` and `view`; both already call `paintShell`.

### 3.14 Undo history cleared
Verdict: confirmed (infra-store.js 549–550).
Touch: infra-store.js → `flush` (547–550).
Do:
```js
const had = histories[view].canUndo() || histories[view].canRedo();
histories[view].clear(); emit({ type: 'history' });
if (had) notify(t('desktop.infra_history_cleared'), 'info');
```
Tests:
(a) None.
(b) In the block at test-infra-ui.mjs:611, subscribe to notices before the `dispatch` and assert they include `'desktop.infra_history_cleared'`.
Keys: infra_history_cleared | Frühere Schritte lassen sich nicht mehr rückgängig machen | Earlier steps can no longer be undone
Risk: none worth noting.

### 3.15 Undo announcement
Verdict: confirmed with a narrower cause. `step()` (796) returns `true` for an optimistic entry without checking anything. When every asset of the entry is gone (deleted locally or by a poll), the queued intents send nothing (`layoutItems` skips unknown ids), yet infra-gestures.js 868 announces "Rückgängig gemacht". The toolbar button (infra.js 514–515) announces nothing at all.
Touch: infra-store.js → `step` (796); infra.js → a new `stepHistory`, `onRootClick` 514–515, `syncMenus` 399–400; optionally infra-gestures.js 868.
Do:
1. In `step`:
```js
if (entry.optimistic) { const intents = forward ? entry.after : entry.before;
  if (![...intents.keys()].some(id => nodeById(id))) { emit({ type: 'history' }); return false; } // nothing left to change: the entry is dropped
  void (forward ? entry.apply(api) : entry.revert(api)); done(); emit({ type: 'history' }); return true; }
```
2. In infra.js: `async function stepHistory(dir) { const view = mapView(), type = store.nextHistory(view, dir); if (await (dir === 'undo' ? store.undo(view) : store.redo(view))) announce(model.fill(t(`desktop.infra_${dir === 'undo' ? 'undone' : 'redone'}_named`), { action: historyWord(type, view) })); }`. Expose it as `app.stepHistory`. Use it for the toolbar, the overflow menu and the window menu.
3. Optional (package 2 file): change gestures 868 to `void app.stepHistory(redo ? 'redo' : 'undo');`. Without that change, gestures is already correct after step 1, but its announcement does not name the command.
Tests:
(a) None.
(b) New block in test-infra-ui.mjs: `dispatch(moveNodes(...'a'...))`, then `store.removeNode('a')`, then `assert.equal(await store.undo('atlas'), false); assert.equal(store.canUndo('atlas'), false);`.
Keys:
- infra_undone_named | Rückgängig gemacht: {action} | Undone: {action}
- infra_redone_named | Wiederholt: {action} | Redone: {action}
Risk: an entry with some assets still present undoes as today.

### 5.1 Filter popover
Verdict: confirmed (CSS 79–81: flex with `space-between`, select `max-width: 200px`).
Touch: CSS `.infra-filters` (79), `.infra-select-label` (80–81), 676.
Do:
- `.infra-filters { grid-template-columns: max-content 200px; column-gap: var(--ds-space-3); }`
- `.infra-select-label { display: grid; grid-column: 1 / -1; grid-template-columns: subgrid; align-items: center; }`
- `.infra-select-label select { width: 100%; max-width: none; }`
- `.infra-filters [data-action="clear-filters"] { grid-column: 1 / -1; justify-self: end; }`
Tests:
(a) None.
(b) Panels test at 414, after opening filters:
```
()=>{const r=[...document.querySelectorAll('[data-popover="filters"] select')].map(s=>s.getBoundingClientRect());return new Set(r.map(x=>Math.round(x.left))).size===1&&new Set(r.map(x=>Math.round(x.width))).size===1}
```
Keys: none.
Risk: the popover gets wider; `openMenu` (435) already clamps it inside the window.

### 5.2 Layers
Verdict: confirmed. `.infra-toasts` has z-index 25 (CSS 386); `.infra-layer` has 30 (74).
Touch: CSS 386.
Do: set `z-index: 31`. The modal `<dialog>` lives in the top layer and is not affected.
Tests:
(a) None. workspace_polish test 93 checks position only.
(b) In test-infra-ui.mjs next to the canvas CSS block (520): read `ui/css/desktop-app-infra.css` with the same `cssRule` pattern, and assert z-index of `.infra-toasts` > z-index of `.infra-layer`.
Keys: none.
Risk: a toast can now cover an open menu. That is intended.

### 5.3 Small targets
Verdict: adjusted. The status reset is already 26 px high and at least 28 px wide (CSS 59, 405), so it meets 24 px and needs no change. The discovery checkbox label is 22 px wide (282), and the List row checkbox is 18×18 (658); both are real.
Touch: CSS 282, 589, 658 (done together with 5.4).
Do: `.infra-candidate-check { width: 24px; height: 32px; }`. The 24×24 input itself comes from 5.4.
Tests: see 5.4.
Keys: none.
Risk: 658 is a package 4 rule; coordinate.

### 5.4 Checkboxes
Verdict: adjusted. The library renders no checkboxes (grep of `type="checkbox"` in infra*.js: import 42/141, table 26/72, PDF 357). The three remaining looks are: native with accent-color (group head 589, List 658) and a custom mark span (candidate 284–286).
Touch: infra-import.js 141 (done in 3.1); CSS 283–286, 589, 622, 658.
Do:
1. Delete 283–286 (`.infra-candidate-check input` and `.infra-checkbox-mark`).
2. Add one rule set:
```css
.infra-app :is(.infra-candidate-check, .infra-import-group-head, .infra-col-check) input[type="checkbox"] { appearance: none; display: inline-grid; place-items: center; width: 24px; height: 24px; margin: 0; border: 0; background: none; cursor: pointer; }
.infra-app :is(.infra-candidate-check, .infra-import-group-head, .infra-col-check) input[type="checkbox"]::before { content: ""; grid-area: 1 / 1; width: 18px; height: 18px; border: 1px solid var(--vd-mat-field-border, var(--vd-theme-border)); border-radius: var(--ds-radius-xs); background: var(--vd-mat-field-color, var(--vd-theme-app-bg)); }
… :is(:checked, :indeterminate)::before { border-color: var(--vd-accent); background: var(--vd-accent); }
… ::after { content: ""; grid-area: 1 / 1; width: 5px; height: 9px; margin-top: -2px; border: solid var(--vd-accent-contrast); border-width: 0 2px 2px 0; rotate: 45deg; opacity: 0; }
… :checked::after { opacity: 1; }
… :indeterminate::after { opacity: 1; width: 8px; height: 0; margin: 0; border-width: 0 0 2px; rotate: none; }
… :disabled { opacity: .45; cursor: default; }
```
3. Reduce 589 to `margin: 0 var(--ds-space-1)` and delete 658. Line 622 is covered by the new disabled rule.
Tests:
(a) None. Grep confirms no test references `.infra-checkbox-mark`. Every test toggles `[data-candidate]` through JS.
(b) TestDesktopInfraImportSheetBrowser after line 76:
```
()=>{const c=document.querySelector('[data-candidate="reg-printer"]'),g=document.querySelector('[data-import-check-group="recommended"]'),r=c.getBoundingClientRect();return r.width>=24&&r.height>=24&&getComputedStyle(c,'::before').width===getComputedStyle(g,'::before').width}
```
Keys: none.
Risk: the focus ring now draws around the 24 px box. That is acceptable.

### 5.5 Dead rules
Verdict: confirmed. A grep for all seven class names over `ui` (everything except *.css: js, html, go, bundles) and over `scripts` finds no match, and nothing builds these class names dynamically.
Touch: CSS.
Do:
- 143–144: drop `.infra-detail-actions button`, `.infra-relations button` and `.infra-quick-place button` from both selector lists.
- Delete 161–170 (target list and options), 209 (`.infra-detail-more`), 228–229 (quick-place), 252–254 (relations) and 320–327 (hosted-services).
- 215: drop `, .infra-relations h3`.
- 232: drop `, .infra-detail-actions > button`.
Tests: none.
Keys: none.
Risk: none.

### 5.6 Radius literals
Verdict: adjusted. The Standard tokens are xs 4, sm 6, md 10, lg 14 (desktop-base.css 121–126).
Touch: CSS 210, 318.
Do:
- Replace `6px` with `var(--ds-radius-sm)` at 210 and 318.
- 284 (4px), 209 (5px), 323 (9px) and 324 (6px) disappear with 5.4 and 5.5.
- Leave 151, 189 and 220 (5px), 267 and 287 (7px) and 264 (11px): no token has the same value.
Tests: none.
Keys: none.
Risk: none.

## D. Open questions

1. **3.2 instance names:** no API field holds an instance name, and the brief rules out route changes. The plan uses the Docker engine's host device name and keeps numbers for Proxmox and FRITZ!Box. If "aurago-test" in the review was something else, it needs a server field, which is out of scope.
2. **3.11 "F" hint:** the toolbar menus have no Fit command. I put `F` on the map context menu's Fit item and recommend against adding a Fit item to "More".
3. **3.11 disabled items:** they keep `disabled` plus a `title`, as `paintArrange` does today. Keyboard users skip them and so never hear the reason. Switching to `aria-disabled` would keep them focusable, but it changes the menu's keyboard contract; I recommend leaving it.
4. **3.9 "Wird ausgeführt…":** I read it as the toast for a `running` or `accepted` answer. If it was meant as the button label while the request is in flight, add that in `onBusy` with the same key.
5. **3.5 partial import:** it gets an `error` toast, which does not auto-dismiss, although the rows already show inline errors. If that is too loud, use `info`.
6. **3.1 unlikely rows:** their reason now shows only when the row is opened. The alternative is to show the reason as the meta line of collapsed rows in that group.
7. **Unused key:** `infra_new_node` becomes unused. Package 6 should delete it from all 16 files if the i18n guard flags unused keys.
