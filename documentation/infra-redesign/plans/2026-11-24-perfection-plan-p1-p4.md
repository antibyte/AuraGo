# Infra perfection plan: packages 1 and 4

I read the spec, AGENTS.md lines 251 to 463, all four source files in full, the parts of infra-model.js, infra-store.js and infra.js they call, internal/infra/validate.go, and the tests named in the brief. Nothing was edited. All paths are under `C:\Users\Andi\Documents\repo\AuraGo\.worktrees\infra-perfection`.

The project CLAUDE.md requires a GitNexus `impact` run before editing any function. For these edits that means: `paintNode`, `paint`, `connectionsHTML`, `overviewHTML`, `copyText`, `syncDirty`, `validateDetailForm`, `submitDetails`, `saveNodeNow`, `rowMarkup`, `tableSortValue` and `tableComparator`.

Baseline failure: `TestDesktopInfraMenusAndShortcutsBrowser` already fails on unchanged code (44 of 45 TestDesktopInfra* tests pass). Do not attribute that failure to these items. Only item 1.16 adds an assertion to that test.

## A. Files and headroom

| File | Now | Added by this plan (estimate) | After |
|---|---|---|---|
| ui/js/desktop/apps/infra-inspector.js | 461 | +80 | ~541 |
| ui/js/desktop/apps/infra-inspector-forms.js | 718 | +65 | ~783 |
| ui/js/desktop/apps/infra-table.js | 229 | +85 | ~314 |
| ui/js/desktop/apps/infra-model.js | 951 | +20 (validators, hardware and status sort) | ~971 |
| ui/css/desktop-app-infra.css | 707 | about +40 net (the container-query rules at 632–636 go) | ~747 |
| ui/lang/desktop/*.json (16 files) | — | 13 keys each | — |
| ui/js/desktop/apps/AGENTS.md | — | update lines 266–268, 326, 434–437, 440–442 | — |

- Every infra*.js file stays well below 1100 lines. No new module is needed.
- I did not find the CSS line limit of the UI line budget guard. Check it before you start.
- `infra-model.js` is outside the "Files" list of both packages, but items 1.10, 4.4 and 4.6 need pure helpers there. The alternative would be duplicating logic into the UI modules.

## B. Order of work

**Package 1**
1. CSS only: 1.3, 1.17, 1.18.
2. Header (`nodeMarkup` and the header CSS): 1.1 and 1.2 together.
3. Overview (`overviewHTML`, `copyText`): 1.8, 1.9, 1.12 and 1.13 together.
4. Connections (`connectionsHTML`): 1.5, 1.6 and 1.7 together.
5. Panel paint: 1.11 and 1.15 together. Both change `paint()` and `paintNode()`.
6. Forms: 1.10, after 1.11. Both touch `saveNodeNow` and `submitDetails`.
7. Then 1.14, 1.16 and 1.4.

**Package 4**
1. Model first: 4.4 and 4.6 (sort values).
2. `rowMarkup`: 4.5, 4.6, 4.7 and the `data-col` attributes of 4.1, together.
3. Column layout: 4.1.
4. Events: 4.2 and 4.3 together (`onClick`, `onDblClick`, keydown, roving tab stop).

**Cross-package**
- After 4.2 every row click opens the inspector. That path goes through 1.11 (focus restore) and 1.15 (draft-kept toast).
- `inspector.close()` (line 320) focuses `[data-row] [data-select]`. This still works with `tabindex="-1"`.

## C. Items

### 1.1 Tabs fit
Verdict: confirmed. The font-size floor of 14 px stays: `desktop_infra_a11y_browser_test.go:363` pins `[role="tab"]` at 14 px or more, so the fix is padding and badge size only.
Touch: desktop-app-infra.css -> `.infra-inspector` (380), `.infra-tabs` (550–556); infra-inspector.js -> `showTab` (211–228), `create`, `dispose`.
Do:
1. Width budget: the labels are 258 px together in German and cannot shrink.
   - Set `.infra-tabs { gap: 0; padding: 4px 4px 0; scrollbar-width: none; }` and add `.infra-tabs::-webkit-scrollbar { display: none; }`.
   - Set the tab to `padding: 0 5px; gap: 4px`.
   - Set `.infra-tab-count { min-width: 16px; padding: 0 4px; border: 0; font-size: 11px; line-height: 16px; text-align: center; }`.
   - Add `scrollbar-width: thin; scrollbar-color: var(--vd-theme-border) transparent;` to `.infra-inspector`.
   - My estimate is 334 px needed against 336 px available. Measure at 360 px in de and en, and go to `padding: 0 4px` if it still overflows.
2. Fade CSS. The masks use only an opaque token and `transparent`:
   ```css
   .infra-tabs[data-fade-end]:not([data-fade-start]) { mask-image: linear-gradient(to right, var(--vd-text) calc(100% - 24px), transparent); }
   .infra-tabs[data-fade-start]:not([data-fade-end]) { mask-image: linear-gradient(to left, var(--vd-text) calc(100% - 24px), transparent); }
   .infra-tabs[data-fade-start][data-fade-end] { mask-image: linear-gradient(to right, transparent, var(--vd-text) 24px, var(--vd-text) calc(100% - 24px), transparent); }
   ```
3. Fade JS in infra-inspector.js:
   ```js
   const paintTabFade = () => { const tabs = $('[data-inspector-tabs]'); if (!tabs) return; const max = tabs.scrollWidth - tabs.clientWidth; tabs.toggleAttribute('data-fade-start', tabs.scrollLeft > 1); tabs.toggleAttribute('data-fade-end', tabs.scrollLeft < max - 1); };
   ```
   - Call it at the end of `showTab`.
   - Add `el.addEventListener('scroll', onScroll, true)` with `onScroll = e => { if (e.target.matches?.('[data-inspector-tabs]')) paintTabFade(); }`.
   - Add `const tabResize = typeof ResizeObserver === 'function' ? new ResizeObserver(paintTabFade) : null; tabResize?.observe(el);`. This only runs inside `create()`, so nothing touches the DOM at load time. Disconnect it and remove the listener in `dispose`.
4. In `showTab`, reveal the active tab with a 24 px margin so the fade never covers it: `const pad = 24; if (rect.left < bounds.left + pad) tabs.scrollLeft -= bounds.left + pad - rect.left; else if (rect.right > bounds.right - pad) tabs.scrollLeft += rect.right - (bounds.right - pad);`. `scrollLeft` clamps at both ends.

Tests: (a) none break. `TestDesktopInfraInspectorTabVisibilityBrowser` keeps passing because End scrolls to the maximum. (b) In `desktop_infra_header_action_browser_test.go`, inside the locale loop (after line 62), add for `de` and `en` only: `const s=document.querySelector('[data-inspector-tabs]');return s.scrollWidth<=s.clientWidth+1&&getComputedStyle(s).scrollbarWidth==='none'`.
Keys: none.
Risk: a two-digit count can overflow by a few px in German; it then shows the fade (the spec allows this). The mask also clips the focus ring at the very edge.

### 1.2 Header action
Verdict: confirmed (line 114 renders the full sentence as a primary block inside `.infra-head-pills`).
Touch: infra-inspector.js -> `nodeMarkup` (113–115, 129); desktop-app-infra.css (545–548).
Do:
1. Take `${quick}` out of `.infra-head-pills`. Render it after the pills as `<div class="infra-head-cta">${quick}</div>`, and only when `quick` is not empty.
2. App variant:
   ```js
   const full = model.fill(t('desktop.infra_open_in_app'), { app: appLabel(link.app) });
   `<button type="button" class="infra-button infra-button-primary" data-action="open-first-app" data-open-app="…" data-target-id="…" title="${esc(full)}" aria-label="${esc(full)}"><span>${esc(appLabel(link.app))}</span></button>`
   ```
   The Edit variant gets the same `<span>` wrapper.
3. CSS: delete lines 545–548. Add:
   ```css
   .infra-head-cta { display: flex; margin-top: 6px; min-width: 0; }
   .infra-head-cta .infra-button { height: 30px; max-width: 100%; padding: 0 12px; line-height: 1.25; }
   .infra-head-cta .infra-button > span { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
   @media (pointer: coarse) { .infra-head-cta .infra-button { min-height: 44px; } }
   ```

Tests: (a) `desktop_infra_header_action_browser_test.go`:
- Lines 34 and 128–129: change the selectors to `.infra-head-cta`.
- Lines 85–90: `want` becomes the app label. Also return and compare `aria:button.getAttribute('aria-label')` against the full sentence.
- Lines 82–84 and 93–96: fail when `got.Lines > 1` instead of requiring that at least one locale wraps.
- Line 11's comment and the AGENTS.md note at 440–442 change accordingly.

(b) Covered by the lines-equals-1 check over all 16 locales.
Keys: none (reuses `infra_app_*` and `infra_open_in_app`).
Risk: a very long app label in some locale ellipsizes; the full text stays in title and aria-label.

### 1.3 Breadcrumb
Verdict: confirmed (line 541 uses `li + li::before`).
Touch: desktop-app-infra.css (540–543).
Do: replace line 541 with:
```css
.infra-crumbs li { display: inline-flex; align-items: center; white-space: nowrap; }
.infra-crumbs li:not(:last-child)::after { content: '›' / ''; margin: 0 4px; }
```
Tests: none.
Keys: none.
Risk: a single very long place name can still overflow (it never could break before either).

### 1.4 Source cards
Verdict: confirmed (the grid has 3 columns, so a sixth child lands in row 2, column 1).
Touch: infra-inspector-forms.js -> `sourceMarkup` (433); desktop-app-infra.css (237, 247).
Do:
1. Markup: wrap the pills: `…</div><span class="infra-source-pills">${power}<span class="infra-state-pill" …>…</span></span><small class="infra-source-time">…</small>${actionBar}`.
2. CSS:
   ```css
   .infra-source-pills { display: flex; flex-wrap: wrap; justify-content: flex-end; gap: 4px; min-width: 0; max-width: 100%; }
   .infra-source-time { grid-column: 2 / -1; }
   ```
   Change line 247's selector to `.infra-source-time` and set the grid to `28px minmax(0, 1fr) minmax(0, max-content)`.

Tests: none. `desktop_infra_browser_test.go:530–539` only counts rows and finds `[data-source-action]`.
Keys: none.
Risk: low.

### 1.5 Connection direction
Verdict: confirmed (line 97 always uses the forward label).
Touch: infra-inspector.js -> `connectionsHTML` (93–103) and the service components in `overviewHTML` (87).
Do:
1. Add a module constant `const DIRECTED = new Set(['runs_on', 'depends_on', 'powered_by', 'member_of']);` and a helper `const edgeLabel = (edge, nodeId) => t(`desktop.infra_edge_${edge.kind}${DIRECTED.has(edge.kind) && edge.to === nodeId && edge.from !== nodeId ? '_inverse' : ''}`);`.
2. Use it at line 97 and line 87.

Tests: none break. (b) No new browser assertion; the toast/key guards cover the new keys. Optional: one Node assertion is not possible, because the helper is closure-local.
Keys:
- infra_edge_runs_on_inverse | Hostet | Hosts
- infra_edge_depends_on_inverse | Benötigt von | Required by
- infra_edge_powered_by_inverse | Versorgt | Powers
- infra_edge_member_of_inverse | Enthält | Contains

Risk: none beyond wording.

### 1.6 Source-reported connections
Verdict: confirmed (line 96: `locked` disables both buttons for `edge.source` and for read-only windows).
Touch: infra-inspector.js -> `connectionsHTML` (96–100); CSS (569, 576).
Do:
1. Render the edit and remove buttons only when `write && !edge.source`.
2. When `edge.source` is set:
   ```js
   const known = ['registry', 'docker', 'proxmox', 'fritzbox'].includes(edge.source);
   const note = model.fill(t('desktop.infra_edge_reported_by'), { source: known ? t(`desktop.infra_source_${edge.source}`) : edge.source });
   ```
   Append it to the caption line: `<small class="infra-relation-caption">${esc([caption, note].filter(Boolean).join(' · '))}</small>`.
3. A read-only window shows no buttons and no note.
4. Delete the now-unused `.infra-relation-row button:disabled` rule (576).

Tests: none. No test reads `[data-edit-edge]`.
Keys: infra_edge_reported_by | Von {source} gemeldet | Reported by {source}
Risk: keyboard users in read-only windows lose two disabled stops (intended).

### 1.7 Connections heading
Verdict: confirmed (line 102).
Touch: infra-inspector.js -> `connectionsHTML` (102); CSS.
Do:
1. Replace the heading block with `<h3 class="infra-sr-only">${esc(t('desktop.infra_tab_connections'))} ${edges.length}</h3>`.
2. When the window is writable, add `<div class="infra-relation-toolbar">${add button unchanged}</div>`, then the rows.
3. CSS: `.infra-relation-toolbar { display: flex; justify-content: flex-start; }` and `.infra-tabpanel .infra-relation-row + .infra-relation-row { margin-top: 0; }` (the panel grid gap already spaces the rows).

Tests: none. `desktop_infra_a11y_browser_test.go:191` still finds `[data-inspector-panel="connections"] [data-action="add-edge"]`.
Keys: none.
Risk: low.

### 1.8 Guests in the overview
Verdict: confirmed (63–66 and CSS 337–338 right-align the chips).
Touch: infra-inspector.js -> `overviewHTML` (63–66, 89); CSS (338, 561–566).
Do:
1. Take the guests out of `facts` and build a block:
   ```js
   const guestChip = guest => { const key = model.powerLabel(guest.power); return `<li><button type="button" data-inspector-select="${esc(guest.id)}"><i class="infra-guest-power" data-power="${esc(guest.power || '')}" aria-hidden="true"></i>${app.icon(guest.icon)}<span>${esc(guest.name)}</span>${key ? `<span class="infra-sr-only">, ${esc(t(key))}</span>` : ''}</button></li>`; };
   const guestsBlock = guests.length ? `<div class="infra-overview-block"><h3 class="infra-overview-heading">${esc(t('desktop.infra_fact_guests'))}</h3><ul class="infra-overview-guests">${guests.slice(0, LIST_LIMIT).map(guestChip).join('')}${more}</ul></div>` : '';
   ```
   `more` stays as it is today (`data-guest-more`, which calls `library.reveal`).
2. Return it right after the `<dl>`: `…</dl>${guestsBlock}${specs}…`.
3. CSS: delete line 338. Add:
   ```css
   .infra-overview-guests { justify-content: flex-start; }
   .infra-guest-power { width: 8px; height: 8px; flex: 0 0 auto; border-radius: 50%; background: var(--vd-theme-muted); }
   .infra-guest-power[data-power="running"] { background: var(--infra-ok); }
   .infra-guest-power[data-power="stopped"] { background: var(--infra-bad); }
   .infra-guest-power[data-power="paused"] { background: var(--infra-warn); }
   ```

Tests: none break (`fact()` in the overview test does not read guests). (b) Optional; skip under lean rules.
Keys: none.
Risk: low.

### 1.9 Fact labels
Verdict: confirmed (line 52 uses `infra_fact_hardware`, "Hersteller und Modell").
Touch: infra-inspector.js (52); CSS (336).
Do:
1. Use `t('desktop.infra_group_hardware')` ("Hardware" exists in all locales).
2. Add `white-space: nowrap; overflow: hidden; text-overflow: ellipsis;` to `.infra-facts-card dt` (on a flex dt, put the ellipsis on an inner span, or make the dt `display: block` with `align-self: center`).
3. `infra_fact_hardware` becomes unused. Remove it from the 16 files only if the i18n guard flags unused keys; otherwise leave it for package 6.

Tests: none.
Keys: none.
Risk: none.

### 1.10 Validation
Verdict: confirmed. Line 265 disables Save on invalid. `pattern`, `type="url"` and `min`/`max` rely on native bubbles. IP, MAC and VLAN are not checked.

Server rules from validate.go, and what the client must allow:

| Field | Server rule | Client rule |
|---|---|---|
| IP | `net.ParseIP`, at most 64 chars (lines 299–307) | same; IPv4 leading zeros are rejected by Go too |
| MAC | `net.ParseMAC`: 6, 8 or 20 octets with `:` or `-`, or dotted `xxxx.xxxx.xxxx` | all ParseMAC forms, so not stricter |
| VLAN | 0 to 4094, 0 = unknown | empty, 0 or 1 to 4094 |
| CIDR | `netip.ParsePrefix` | existing `validCIDR` |
| Custom key | `^[a-z][a-z0-9_.-]{0,63}$` | same pattern; the client allows uppercase today, so the server rejects later |
| Interface VLAN | text up to 64 chars | not checked |

Touch: infra-model.js (after `validHTTPLink`, line 75, and the export at 948); infra-inspector-forms.js -> `listEditor`/`customEditor` field attrs (94, 97, 204), the network group (199), `detailsHTML` form tag (219), `syncDirty` (265), `validateDetailForm` (228–239), `submitDetails` (316–326), `saveNodeNow` (338, 356), `revert-node` (678); infra-inspector.js -> `onFocusOut` (404).
Do:
1. Model:
   ```js
   const IPV4_PART = '(25[0-5]|2[0-4]\\d|1\\d\\d|[1-9]?\\d)', IPV4 = new RegExp(`^${IPV4_PART}(\\.${IPV4_PART}){3}$`);
   const validIP = value => { const text = String(value ?? ''); if (!text) return true; if (text.length > 64) return false; if (IPV4.test(text)) return true; if (!text.includes(':') || /[^0-9a-f:.]/i.test(text)) return false; try { return new URL(`http://[${text}]/`).hostname.length > 0; } catch (_) { return false; } };
   const validMAC = value => { const text = String(value ?? ''); if (!text) return true; if (/^[0-9a-f]{2}([:-])[0-9a-f]{2}(?:\1[0-9a-f]{2})*$/i.test(text)) return [6, 8, 20].includes((text.length + 1) / 3); return /^[0-9a-f]{4}(?:\.[0-9a-f]{4})+$/i.test(text) && [6, 8, 20].includes(2 * (text.length + 1) / 5); };
   const validVLAN = value => value === '' || (/^\d{1,4}$/.test(String(value)) && Number(value) <= 4094);
   const validCustomKey = key => /^[a-z][a-z0-9_.-]{0,63}$/.test(key);
   ```
   Export all four. Write the backslashes as shown: the template uses `\\d` inside a JS string. Use the Write tool carefully (see the memory note on `\u` escapes; there are none here).
2. Markup:
   - Add `novalidate` to the detail `<form>`. Without it, clicking the now-enabled Save runs native validation and never fires `submit`.
   - Custom key attrs become `required maxlength="64"` (drop `pattern`).
   - The link url becomes `type="text" inputmode="url"`.
   - Drop `min="1" max="4094"` from the VLAN input; native `rangeUnderflow` would reject 0, which the server accepts.
   - Add `data-action="save-node"` to the submit button (used by 1.11).
3. Line 265 becomes `submit.disabled = !dirty || busy;`.
4. `validateDetailForm` sets every message with `setCustomValidity`, so `validationMessage` is always ours and in the app language:
   ```js
   const RULES = [
     ['[data-profile="cidr"]', i => validCIDR(i.value) ? '' : 'invalid_cidr'],
     ['[data-profile="vlan"]', i => !i.validity.badInput && validVLAN(i.value) ? '' : 'invalid_vlan'],
     ['[data-profile="ip_address"], [data-list-family="interfaces"][data-key="ip_address"]', i => validIP(i.value) ? '' : 'invalid_ip'],
     ['[data-profile="mac_address"], [data-list-family="interfaces"][data-key="mac_address"]', i => validMAC(i.value) ? '' : 'invalid_mac'],
     ['[data-list-family="links"][data-key="label"], [data-list-family="interfaces"][data-key="name"]', i => i.value.trim() ? '' : 'field_required'],
     ['[data-list-family="links"][data-key="url"]', i => validHTTPLink(i.value) ? '' : 'invalid_link'],
   ];
   ```
   - Loop: `for (const [sel, rule] of RULES) for (const i of form.querySelectorAll(sel)) { const k = rule(i); i.setCustomValidity(k ? t(`desktop.infra_${k}`) : ''); }`.
   - Custom keys: trimmed empty -> `field_required`; `!validCustomKey` -> `invalid_custom_key`; duplicate -> `duplicate_custom_key`.
   - Then `paintFieldErrors(form)` and `return form.checkValidity()`.
5. Message display state: `let shown = { id: '', keys: new Set(), all: false };`.
   - `fieldKey(i) = i.dataset.profile || (i.dataset.listFamily ? `${i.dataset.listFamily}:${i.dataset.index}:${i.dataset.key}` : '')`.
   - `paintFieldErrors(form)`:
     - If `shown.id !== state.selected`, reset `shown` first.
     - Loop over `form.elements`; skip anything without a key or outside `.infra-field`.
     - Set `id = `infra-field-error-${S}-${key.replace(/[^\w-]/g, '-')}``.
     - When `(shown.all || shown.keys.has(key)) && !i.validity.valid`: create or update `<p class="infra-field-error" id="${id}">` appended to `i.closest('.infra-field')`, with `textContent = i.validationMessage`, and set `aria-invalid="true"` and `aria-describedby=id`.
     - Otherwise remove the note and both attributes.
6. Blur: export `onFieldBlur(event)`. When the target is inside `[data-detail-form]` and has a key, add the key to `shown.keys` and call `validateDetailForm(form)`. Call it from the inspector's `onFocusOut` (line 404) before the rename check.
7. Save attempt, `submitDetails`:
   ```js
   shown.all = true;
   if (validateDetailForm(form)) { form.requestSubmit(); return; }
   ```
   Otherwise find the first invalid element in `form.elements`, as today: `showTab('details')`, open its group (keep 322–324), `bad.focus()`. Remove `reportValidity()`.
   `saveNodeNow` line 338 already routes invalid forms there and saves nothing.
8. After a successful save (line 356) and on discard (line 678), set `shown = { id: '', keys: new Set(), all: false }`.
9. CSS:
   ```css
   .infra-field-error { margin: 0; color: var(--infra-bad-text); font-size: 13px; }
   .infra-field :is(input, select, textarea)[aria-invalid="true"] { border-color: var(--infra-bad-ring); }
   ```

Tests: (a) `desktop_infra_browser_test.go:559–561`: still valid (a clean form leaves Save disabled), now through `novalidate`. `scripts/test-infra-ui.mjs:40–43` unchanged.
(b) Two additions:
- test-infra-ui.mjs after line 43:
  - IP: `validIP('2001:db8::1')`, `'::ffff:10.0.0.1'` and `''` are true; `'01.2.3.4'` and `'fe80::1%eth0'` are false.
  - MAC: `validMAC('AA-BB-CC-DD-EE-FF')`, `'0000.5e00.5301'` and an 8-octet form are true; `'aa:bb-cc:dd:ee:ff'` and `'aa:bb:cc:dd:ee'` are false.
  - VLAN: `validVLAN('0')` and `'4094'` are true; `'4095'` and `'1.5'` are false.
  - `validCustomKey('Rack')` is false.
- Browser: `desktop_infra_panels_browser_test.go` before line 222, on "Garden relay":
  1. Set `[data-profile="ip_address"]` to `nope`, dispatch input, then blur.
  2. Assert `aria-invalid="true"` and that the described element's text equals `fixtureWords['desktop.infra_invalid_ip']`, and that `[data-detail-form] [type="submit"]` is not disabled.
  3. Click Save. Assert `nodePuts` is still 0 and `activeElement` is that input.
  4. Clear the value back to `''`.

Keys:
- infra_invalid_ip | Gib eine gültige IPv4- oder IPv6-Adresse ein. | Enter a valid IPv4 or IPv6 address.
- infra_invalid_mac | Gib eine MAC-Adresse wie aa:bb:cc:dd:ee:ff ein. | Enter a MAC address such as aa:bb:cc:dd:ee:ff.
- infra_invalid_vlan | Gib eine VLAN-ID von 1 bis 4094 ein oder lass das Feld leer. | Enter a VLAN ID from 1 to 4094, or leave the field empty.
- infra_invalid_custom_key | Beginne mit einem Kleinbuchstaben und verwende nur a–z, 0–9, Punkt, Bindestrich und Unterstrich. | Start with a lowercase letter and use only a–z, 0–9, dot, hyphen and underscore.
- infra_field_required | Fülle dieses Feld aus. | Fill in this field.

These are field messages, not toasts, so they keep the full stop like `infra_invalid_cidr`.
Risk: `saveAllDrafts` (window-close save) validates only the selected asset's form, as today. An empty custom row that was valid-by-omission before is now flagged as required, which matches today's native `required`.

### 1.11 Focus after Save and Discard
Verdict: confirmed. `paintNode` replaces `innerHTML` (151) after save (365) and discard (678). Also, the Save button turns disabled while busy, and Chrome's focus fixup then moves focus to the body before the paint.
Touch: infra-inspector.js -> `paintNode` (140–156), the API (457); infra-inspector-forms.js -> `saveNodeNow` (343), `revert-node` (678).
Do:
1. In the inspector:
   ```js
   const FOCUS_ATTRS = ['data-action', 'data-inspector-tab', 'data-profile', 'data-node-field', 'data-list-family', 'data-index', 'data-key', 'data-list-add', 'data-list-remove', 'data-jump-tab', 'data-copy-value', 'data-inspector-select', 'data-crumb'];
   let pendingFocus = null;
   const focusKey = node => (node && el.contains(node) ? { selector: FOCUS_ATTRS.filter(a => node.hasAttribute(a)).map(a => `[${a}="${CSS.escape(node.getAttribute(a))}"]`).join('') } : null);
   const rememberFocus = () => { pendingFocus = focusKey(document.activeElement); };
   ```
2. In `paintNode`, before `innerHTML`: `const focus = same ? (pendingFocus || focusKey(document.activeElement)) : null; pendingFocus = null;`.
3. After `forms.afterPaint(node)`:
   ```js
   if (focus) { const target = focus.selector && el.querySelector(focus.selector); if (target && target.offsetParent !== null && !target.disabled) target.focus({ preventScroll: true }); else $('[data-inspector-tab][aria-selected="true"]')?.focus({ preventScroll: true }); }
   ```
4. Export `rememberFocus`. Call `app.inspector.rememberFocus()` in `saveNodeNow` right before `store.setBusy(...)` (line 343), and in `revert-node` before `paint()`.

Tests: none break. The callers that focus after paint (`editList` at 401–402, the kind change at 306) still win because they run later. (b) In the panels test after line 234 (Ctrl S): `document.activeElement===document.querySelector('[data-profile="vendor"]')`.
Keys: none.
Risk: a focused element inside a closed `<details>` falls back to the tab (intended).

### 1.12 Copy feedback
Verdict: confirmed (lines 326–339 toast "copied" even after a catch).
Touch: infra-inspector.js -> `copyText` (324–340).
Do:
```js
let ok = false;
try { if (navigator.clipboard?.writeText) { await navigator.clipboard.writeText(value); ok = true; } } catch (_) { /* blocked */ }
if (!ok) { app.toast(t('desktop.infra_copy_failed'), 'error'); return; }
```
Leave the rest unchanged.

Tests: (b) `desktop_infra_overview_browser_test.go` before line 49:
```js
Object.defineProperty(navigator,'clipboard',{configurable:true,value:{writeText:()=>Promise.reject(new Error('blocked'))}})
```
Then click the first `[data-inspector-panel="overview"] [data-copy-value]` (I assume fixture-node-0003 has an IP; I did not verify). Expect `[data-toast] span` text `infra_copy_failed` and no `data-copied`.
Keys: infra_copy_failed | Kopieren nicht möglich | Could not copy
Risk: in headless runs without clipboard access the success path is never exercised (no test relies on it).

### 1.13 Move hint
Verdict: confirmed (line 89 renders it unconditionally).
Touch: infra-inspector.js -> `overviewHTML` (89).
Do: render the tip only when `state.view !== 'table' && canWrite()`. The 'view' and 'caps' events already call `inspector.paint()` (infra.js 635 and 642).
Tests: (b) In the overview test, after the placed-asset check: click `[data-view="table"]` and assert `!document.querySelector('[data-inspector-panel="overview"] .infra-overview-tip')`.
Keys: none.
Risk: none.

### 1.14 Icon picker
Verdict: confirmed (lines 610–613).
Touch: infra-inspector-forms.js -> `openIconPicker` (601, 610–613).
Do:
1. Append `<p class="infra-picker-empty" data-icon-empty hidden>${esc(t('desktop.infra_icon_no_match'))}</p>` after the grid.
2. At the end of the input handler: `box.querySelector('[data-icon-empty]').hidden = visible().length > 0;`.

Tests: none.
Keys: infra_icon_no_match | Kein Symbol passt | No icon matches
Risk: none.

### 1.15 Leaving a dirty draft
Verdict: confirmed (`select` and `clearSelection` keep drafts silently; infra-store.js 395–404).
Touch: infra-inspector.js -> `paint` (198–208).
Do: add `const draftNoticed = new Set();` and put this at the top of `paint()`, after the disposed guard:
```js
const leaving = lastId, next = state.view === 'table' && state.selectionOrigin === 'check' ? '' : state.selected;
for (const id of draftNoticed) if (!state.dirty.has(id) && !state.sourceDirty.has(id)) draftNoticed.delete(id);
if (leaving && leaving !== next && (state.dirty.has(leaving) || state.sourceDirty.has(leaving)) && !draftNoticed.has(leaving) && store.nodeById(leaving)) {
    draftNoticed.add(leaving);
    app.toast(model.fill(t('desktop.infra_draft_kept'), { name: state.drafts.get(leaving)?.name || store.nodeById(leaving).name }), 'info');
}
```
`app.toast` also announces (infra.js 151), which satisfies the feedback rule.

Tests: (b) Panels test after line 223:
1. Click `[data-node="tv"]`. Wait for a `[data-toast] span` equal to `fixtureWords['desktop.infra_draft_kept'].replace('{name}','Garden relay')`.
2. Click `[data-node="relay"]`, wait for `h2==='Garden relay'`, focus `[data-profile="vendor"]`.
3. Ctrl S continues as before.

Keys: infra_draft_kept | Entwurf für {name} bleibt erhalten | Draft for {name} is kept
Risk: the window-close flow (`TestDesktopInfraUnsavedDraftCloseBrowser`) may now also show this toast as the panel hides. It is harmless, but check it there.

### 1.16 Inline connection form
Verdict: confirmed (line 175 has Remove and Save only).
Touch: infra-inspector.js -> `paintEdge` (175), `onEdgeInput` (280–286), `onClick` switch (369–388).
Do:
1. Before Save add `<button type="button" class="infra-button" data-action="cancel-edge" disabled>${esc(t('desktop.cancel'))}</button>`.
2. `onEdgeInput` enables both `[data-action="save-edge"]` and `[data-action="cancel-edge"]`.
3. Add `case 'cancel-edge': edgeDirty = false; paint(); focusEdgeForm(); break;`. This re-renders from the stored edge, so it also picks up a version that a poll changed meanwhile.

Tests: (b) `desktop_infra_panels_browser_test.go` after line 342:
1. Change `[name="label"]` and dispatch input.
2. Click cancel-edge.
3. Assert the label value equals the stored one, Save is disabled, and `fixture.edgePuts` (if that counter exists; I did not verify its name) is unchanged.

This test already fails at baseline. Confirm the new lines run before its failing step, or add them elsewhere.
Keys: none (reuses `desktop.cancel`).
Risk: low.

### 1.17 Small targets and states
Verdict: confirmed (CSS 342, 156, 346, 542; summary is missing from the focus rule at 15).
Touch: desktop-app-infra.css.
Do:
- `.infra-fact-copy` width and height 28px.
- `.infra-list-remove` width and min-height 28px; `.infra-list-row` padding-right 40px.
- `.infra-fact-link` min-height 28px.
- `.infra-crumbs button` min-height 24px.
- Hover for the list-editor buttons:
  ```css
  .infra-list-heading button:hover:not(:disabled), .infra-list-remove:hover:not(:disabled) { border-color: color-mix(in srgb, var(--vd-accent) 44%, var(--vd-mat-btn-border, var(--vd-theme-border))); background: var(--vd-mat-btn-hover, var(--vd-theme-control-hover)); }
  ```
  Add both selectors to the transition list at line 443.
- Summary focus ring:
  ```css
  .infra-app summary:focus-visible { outline: var(--ds-focus-ring-width) solid var(--ds-color-border-focus); outline-offset: calc(var(--ds-focus-ring-width) * -1); }
  ```
  The offset is negative because `.infra-form-group` clips with `overflow: hidden`.

Tests: none.
Keys: none.
Risk: none.

### 1.18 Type scale
Verdict: confirmed (CSS 208 is 17px; 220 and 222 are 15px; 271 is 15px for dialogs).
Touch: desktop-app-infra.css 208, 220, 222, 271, 538.
Do:
- `.infra-detail-head h2` 18px.
- `.infra-field` input/select/textarea and textarea font 14px.
- Line 271 (dialog fields) 14px. That line is a package 3 file area; tell that agent.
- `.infra-head-name-input` 16px -> 18px, so the rename input matches the title (see D4).

Tests: none. The a11y floors stay satisfied.
Keys: none.
Risk: none.

### 4.1 No sideways overflow
Verdict: confirmed. The sum of the fixed widths is the measured 1131 px. The overflow comes from header content: `th` is `nowrap` with no overflow clipping, and "Betriebszustand" in a 75 px column spills, which gives scrollWidth 1203. The container queries also measure the view, which is 26 px wider than the table area, and fixed px cannot fit every locale.
Touch: infra-table.js -> `heads` (20–22), `rowMarkup` (72–76), new `layoutColumns`; CSS 614, 623–636.
Do:
1. Header markup: `<th scope="col" data-col="${key}" data-sort-col="${key}"><button …><span class="infra-sort-label">${label}</span><span class="infra-sort-arrow" …></span></button></th>`.
2. Every non-check `td` gets `data-col="kind|place|address|hardware|source|status|power"`. The name `td` keeps `infra-col-name` and adds `data-col="name"`.
3. JS:
   ```js
   const BASE = { kind: 150, place: 150, address: 120, hardware: 130, source: 130, status: 120, power: 75 };
   const DROP = ['hardware', 'power', 'source', 'place', 'kind', 'address', 'status'], NAME_MIN = 170, CHECK_W = 40;
   let widths = null, hiddenCols = '';
   const table = el.querySelector('.infra-table');
   function layoutColumns() {
       const room = scroll.clientWidth;
       if (!room) return;
       if (!widths) { const label = key => el.querySelector(`[data-sort="${key}"] .infra-sort-label`); if (!label('kind')?.offsetParent) return; widths = Object.fromEntries(Object.entries(BASE).map(([key, base]) => [key, Math.max(base, Math.ceil(label(key).scrollWidth) + 37)])); for (const [key, value] of Object.entries(widths)) table.style.setProperty(`--infra-col-${key}`, `${value}px`); }
       const shown = new Set(Object.keys(BASE)), need = () => CHECK_W + NAME_MIN + [...shown].reduce((sum, key) => sum + widths[key], 0);
       for (const key of DROP) { if (need() <= room) break; shown.delete(key); }
       const next = Object.keys(BASE).filter(key => !shown.has(key)).join(' ');
       if (next !== hiddenCols) { hiddenCols = next; table.dataset.hiddenCols = next; }
   }
   const resize = typeof ResizeObserver === 'function' ? new ResizeObserver(layoutColumns) : null;
   resize?.observe(scroll);
   ```
   - The 37 px is th padding 16 + button padding 8 + gap 4 + arrow 8 + 1 for rounding.
   - Disconnect the observer in `dispose`. Also call `layoutColumns()` at the end of `render()`; it costs nothing once the widths are known.
4. CSS:
   - Line 614: `min-width: 0`.
   - Lines 625–631: `width: var(--infra-col-kind, 150px)` and so on per column.
   - Delete 632–636.
   - Add one hide rule per column: `.infra-table[data-hidden-cols~="hardware"] [data-col="hardware"] { display: none; }`.
   - Add `.infra-table thead th { overflow: hidden; }`, `.infra-sort { max-width: 100%; }` and `.infra-sort-label { overflow: hidden; text-overflow: ellipsis; }`.

Tests: (a) none break. `desktop_infra_list_browser_test.go:98` reads `td[3]` (Place), which stays shown at that width. Update AGENTS.md 434–437.
(b) List test after line 36:
- For `[1157, 900, 640, 360]`, set `document.querySelector('[data-table-view]').style.width=W+'px'`.
- Wait one animation frame, then assert `scroll.scrollWidth<=scroll.clientWidth+1` and every visible `.infra-sort-label` has `scrollWidth<=clientWidth+1`.
- Reset the width afterwards.
- Run it with German labels, loaded before the app opens. I did not verify how `TestDesktopInfraWorkspacePolishLocalesBrowser` does that.

Keys: none.
Risk: the widths are measured once, at the first visible paint. A locale change without recreating the app keeps the old widths (I assume the app is recreated on a language change; not verified).

### 4.2 Row click
Verdict: confirmed (179–183 handle only `[data-select]`; 203–207 ignore unplaced assets).
Touch: infra-table.js -> `onClick` (179–183), `onDblClick` (203–212).
Do:
1. Replace the `[data-select]` branch with:
   ```js
   const row = target.closest('[data-row]');
   if (row && !target.closest('a, .infra-col-check, [data-row-expand]')) { store.select([row.dataset.row]); return; }
   ```
2. `onDblClick`:
   ```js
   const row = event.target.closest('[data-row]'); if (!row || event.target.closest('a, .infra-col-check, [data-row-expand]')) return;
   const id = row.dataset.row, hostId = …;
   if (!placed) { if (state.selected !== id || state.selection.size !== 1) store.select([id]); return; }
   ```
   The map jump stays as today.
3. Suppress text selection on double click: `el.addEventListener('mousedown', e => { if (e.detail > 1 && e.target.closest('[data-row]')) e.preventDefault(); })`. Remove it in `dispose`.
4. AGENTS.md line 326: "Clicking an asset's row still opens its panel."

Tests: (b) List test after line 54:
1. Click `[data-row="fixture-node-0005"] td[data-col="kind"]`.
2. Wait until `!document.querySelector('[data-inspector]').hidden` and the h2 equals that asset's name.
3. Click `[data-action="close-inspector"]` so the checkbox steps that follow start clean.

Keys: none.
Risk: clicking a row while a check-origin selection exists replaces that selection. This matches today's name click.

### 4.3 Row keys
Verdict: confirmed (three focusable controls per row, `tabindex="0"` on the scroller at line 26, no arrow handling).
Touch: infra-table.js -> markup (26, 69, 72–73), `render`, `onKeyDown` (195–202), new `focusRow`, a keyup and a focusin listener.
Do:
1. Remove `tabindex="0"` from `[data-table-scroll]`.
2. Add `let rovingId = '';`. In `render`: `const stop = slice.rows.some(r => r.id === rovingId) ? rovingId : slice.rows[0]?.id || '';`. Pass `stop` to `rowMarkup`:
   - the name button gets `tabindex="${node.id === stop ? 0 : -1}"`;
   - the checkbox and the expander get `tabindex="-1"`.
3. `body.addEventListener('focusin', e => { const row = e.target.closest?.('[data-row]'); if (!row) return; rovingId = row.dataset.row; for (const b of body.querySelectorAll('[data-select]')) b.tabIndex = b.dataset.select === rovingId ? 0 : -1; });`
4. `focusRow(id)`:
   - `rovingId = id; const index = list().rows.findIndex(r => r.id === id), head = el.querySelector('thead').offsetHeight, top = head + index * 54;`
   - `if (top < scroll.scrollTop + head) scroll.scrollTop = index * 54; else if (top + 54 > scroll.scrollTop + scroll.clientHeight) scroll.scrollTop = top + 54 - scroll.clientHeight;`
   - `render(); body.querySelector(`[data-row="${CSS.escape(id)}"] [data-select]`)?.focus({ preventScroll: true });`
5. In `onKeyDown`, before the Delete branch, for a row target with no modifier keys and no typing target:
   - `ArrowDown`/`ArrowUp`/`Home`/`End` -> `focusRow(rows[clamp(index)].id)`.
   - `ArrowRight` on a closed host and `ArrowLeft` on an open host click its `[data-row-expand]`. `ArrowLeft` on a guest focuses its host. The spec does not ask for this, but without it the expander has no keyboard path.
   - `' '` when the target is not `[data-row-check]`: `preventDefault()`, then `box.dispatchEvent(new MouseEvent('click', { bubbles: true, cancelable: true, shiftKey: event.shiftKey }))`.
   - `'Enter'` when the target is not a button: `store.select([id])`. On the name button the native click already selects.
6. `body.addEventListener('keyup', e => { if (e.key === ' ' && e.target.matches?.('[data-select]')) e.preventDefault(); })`. This stops the button's own Space activation (I believe preventing keydown alone is not reliable in Chrome; not verified). Remove all new listeners in `dispose`.

Tests: (a) `desktop_infra_list_browser_test.go:66–70`: focus `[data-row="fixture-node-0010"] [data-select]` instead of the checkbox. Keep Space and the count of 5. The focus assertion becomes `activeElement.dataset.select==='fixture-node-0010'`.
(b) Same place:
1. Press ArrowDown and assert the focus moved to the next row's `[data-select]`.
2. Assert `[...document.querySelectorAll('[data-table-body] button,[data-table-body] input')].filter(e=>e.tabIndex>=0).length===1`.
3. Press ArrowUp to return before line 74.

Keys: none.
Risk: Shift+Tab out of the body now skips the checkboxes (intended). The context menu still finds `[data-select]` (infra-menus.js 144).

### 4.4 Hardware sort
Verdict: confirmed (line 20–22 special case; `TABLE_SORTS` lacks it, infra-model.js 778).
Touch: infra-table.js (20–22); infra-model.js 778 and 783–791.
Do:
1. Drop the `key === 'hardware'` branch so every header gets a sort button.
2. Add `'hardware'` to `TABLE_SORTS`.
3. In `tableSortValue`: `if (key === 'hardware') return [node.profile?.vendor, node.profile?.model].filter(Boolean).join(' · ');`.

Tests: (b) test-infra-ui.mjs:
- Give rows b and c profiles with `vendor: 'Synology'` and `vendor: 'Asus'` respectively.
- Assert `ids('hardware','asc')` starts `['c','b', …]`, with the empty ones last.
- Assert `readTablePrefs('{"sort":"hardware"}').sort === 'hardware'`.

Keys: none.
Risk: none.

### 4.5 Truncated cells
Verdict: confirmed. The name already has a title (line 73) and Place has one (74). Hardware and Source have none (75); Address has none either.
Touch: infra-table.js -> `rowMarkup` (74–75).
Do: compute `hw` and `src` once. Add `title="${esc(text)}"` to the Hardware, Source and Address `td`s when the text is not '—'.
Tests: none.
Keys: none.
Risk: none.

### 4.6 Status column
Verdict: confirmed (line 76 renders the source-state pill, including "Manuell").
Touch: infra-table.js -> `rowMarkup` (76); infra-model.js -> `STATUS_ORDER`/`tableSortValue` (779, 788); CSS.
Do:
1. Cell:
   ```js
   const warn = nodeState === 'stale' || nodeState === 'removed', stateText = app.statusLabel(nodeState);
   `<td data-col="status"><span class="infra-status-cell">${live ? `<span class="infra-live-pill" data-live="${esc(live)}" title="${esc(liveLabel)}"><i></i><span>${esc(liveLabel)}</span></span>` : '<span aria-hidden="true">—</span>'}${warn ? `<span class="infra-status-warn" data-state="${esc(nodeState)}" title="${esc(stateText)}"><span aria-hidden="true">!</span><span class="infra-sr-only">${esc(stateText)}</span></span>` : ''}</span></td>`
   ```
2. Keep `data-state` on the `tr` (it drives the row tone at line 645). Keep the name cell's `.infra-sr-only` live label: `desktop_infra_live_browser_test.go:51` reads the first `.infra-sr-only` in the row.
3. CSS:
   ```css
   .infra-status-cell { display: flex; align-items: center; gap: 4px; min-width: 0; }
   .infra-status-cell .infra-live-pill { min-width: 0; overflow: hidden; }
   .infra-status-cell .infra-live-pill > span { overflow: hidden; text-overflow: ellipsis; }
   .infra-status-warn { flex: 0 0 auto; width: 20px; height: 20px; display: inline-grid; place-items: center; border-radius: var(--ds-radius-full); font: 700 12px/1 var(--vd-font-ui); }
   .infra-status-warn[data-state="stale"] { border: 1px solid color-mix(in srgb, var(--infra-warn) 36%, var(--infra-line)); background: color-mix(in srgb, var(--infra-warn) 12%, var(--infra-card-color)); color: var(--infra-warn-text); }
   ```
   Add the same pattern with `--infra-bad` for `removed`.
4. Model: replace `STATUS_ORDER` with `const REACH_ORDER = { live: 0, offline: 1, unknown: 2 };` and:
   ```js
   if (key === 'status') { const live = liveState(node); return live ? REACH_ORDER[live] * 2 + (['stale', 'removed'].includes(sourceState(node)) ? 0 : 1) : null; }
   ```
   No signal gives null, so those assets sort last in both directions; within one state, warned assets come first.

Tests: (a) `scripts/test-infra-ui.mjs:823` pins the old problems-first order.
- Add `live: 'live'` to b, `'offline'` to a and `'live'` to c (c is stale), nothing to d.
- Expect `ids('status','asc')` to be `['c','b','a','d']`, and add `ids('status','desc')` as `['a','b','c','d']`.

(b) Covered by that change.
Keys: none (reuses `infra_live_*` and `infra_state_*`).
Risk: in German "Nicht erreichbar" ellipsizes in a 120 px column; the full text stays in the title (see D3).

### 4.7 Expander gutter
Verdict: confirmed (32 px disclosure or spacer + 4 px gap + 8 px td padding + a 9 px name gap).
Touch: CSS 659–660 and a new scoped rule.
Do:
```css
.infra-table td.infra-col-name { padding-left: 4px; }
.infra-table-name-cell { gap: 0; }
.infra-table-name-cell .infra-library-disclosure { width: 24px; height: 32px; }
.infra-disclosure-spacer { width: 24px; }
```
Keep the depth indent.

Tests: none.
Keys: none.
Risk: the expander is 24 px wide on touch. If 5.3 wants more, give it `min-height: 44px` on coarse pointers there.

## D. Open questions

1. **1.10 MAC rule.** The spec says "six octets, `:` or `-`", but `net.ParseMAC` also accepts 8 and 20 octets and the dotted `xxxx.xxxx.xxxx` form. A six-octet-only check would be stricter than the server, which breaks the rule. I recommend matching ParseMAC exactly, as planned; the message names the common form.
2. **1.10 VLAN 0.** The server accepts 0 as "unknown". I accept empty or 0, which means the native `min="1"` has to go. The message still says "1 bis 4094 oder leer".
3. **4.6 wording and width.** The spec writes "Offline", but the German key `infra_live_offline` is "Nicht erreichbar". I recommend keeping the key (package 6 owns wording), using the compact "!" warning pill and ellipsizing the reach text with a title. A text warning pill ("Veraltet") will not fit 120 px next to it.
4. **1.18 rename input.** "14 px inputs" could also cover the header rename input. I recommend 18 px so the title does not jump while renaming.
5. **4.3 Left/Right.** The spec does not ask for Left and Right on hosts. Without them, removing the expander's tab stop leaves no keyboard way to open guests, so I added them.
6. **Package boundaries.** 1.10, 4.4 and 4.6 change `infra-model.js`, which is outside both packages' file lists. 1.18 touches the dialog field rule at line 271, which belongs to package 3.
7. **Unverified:**
   - whether the browser fixture exposes an edge PUT counter (1.16);
   - whether fixture-node-0003 has a copyable IP (1.12);
   - how a test loads a locale before the app opens (4.1);
   - Chrome's thin scrollbar width (1.1). Measure it instead of trusting my 334/336 px estimate.
