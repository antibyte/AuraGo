[← Plan index](README.md) · Spec: [plans/2026-11-13-easydrag-w1a-spec.md](../2026-11-13-easydrag-w1a-spec.md)

## Part B: Kit and type scale (FD-09, UI-17)

Spec: `plans/2026-11-13-easydrag-w1a-spec.md` §1 decision 6, §4, §8, §9, §10, §11. Part B starts after Part A has landed on main. The owner decided the type scale: **title 24, head 18, card 16, body 14, meta 12, label 11** (card names and canvas titles 16, summaries, parameter lines and body text 14). Part A names: `--ed-radius-xl` is 12px (`--ed-radius-lg` stays 14px, `--ed-radius` 10px), `--ed-status-ink` is the glyph ink per band, `--ed-status-edge` is a 1px grey ring on discs. It builds `ED.kit`, the bottom-sheet layout of `modal()`, the toast migration, the disclosure migration, the type scale and the two ratchets. Every task ends in one commit. Part B adds no locale keys (the kit reuses `easydrag.ui.close`, `easydrag.ui.undo` and the existing titles), so there is no key file.

**Shell and worktree.** Git Bash. Every command starts with `cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-w1a-kit && …`, because the working directory resets between calls. Branch `feat/easydrag-w1a-kit`, from main after Part A. Go commands carry `GOTOOLCHAIN=go1.27.4` (the go1.27.3 toolchain loses `vet.exe` now and then) and `GOTMPDIR='D:\tmp\go-build'` (all Go packages here are `ui`). Browser tests run with `AURAGO_RUN_BROWSER_SMOKE=1 AURAGO_BROWSER_ARTIFACT_DIR=D:/tmp/easydrag-w1a-shots` and the Bash sandbox off (Chrome cannot start from a test binary under the repo). Never edit, stage or commit in the main checkout `C:/Users/Andi/Documents/repo/AuraGo`.

**Escapes.** The Write and Edit tools turn backslash-u sequences into characters. No code below contains one; keep it so.

**IMPACT command** (used in B.2 to B.4):
```
node C:/Users/Andi/.gitnexus/stores/AuraGo-08496d38b623/checkouts/AuraGo-08496d38b623/run.cjs impact "<name>" --direction upstream --repo C:/Users/Andi/Documents/repo/AuraGo
```
Warn the coordinator on HIGH or CRITICAL. UNKNOWN is unresolved: confirm the callers with `grep -rn`.

**Every commit body ends with** `GitNexus detect-changes unavailable in the worktree; judged by symbols and tests.`, a blank line, then `Co-Authored-By: Claude Sonnet 5.5 <noreply@anthropic.com>`. Write the message file with the Write tool into `C:/disposable/easydrag-w1a/`.

**Facts verified on main 9492395de (before Part A) that the tasks rely on.**

- `ctx.notify` is the **desktop-wide** toast `showDesktopNotification` (`ui/js/desktop/core/sdk-events-bootstrap.js:495`): title and message only, no action, class `vd-toast`, owned by the desktop shell. EasyDrag has 81 `notify(` calls. Only `easydrag-history.js:25-84` has its own in-app toast (`.ed-toast`, test hooks `[data-ed-toast]`, `[data-ed-toast-text]`, `[data-ed-toast-undo]`).
- `modal()` is `easydrag-core.js:439-513`. It builds `.ed-modal-backdrop > .ed-modal` through `innerHTML`; `dialog.el` is the `.ed-modal`. CSS is `desktop-app-easydrag.css` around 1170-1190 (backdrop `place-items: center`, `.ed-modal` width `min(520px,100%)`). Per-dialog widths use `.ed-app .ed-modal--x { width: min(…, 100%) }`.
- `.ed-app` (`easydrag.js:222`) is the one app root with `container-type: inline-size`; editor, home and every dialog live under it.
- The Node harness `miniDom` (`scripts/test-easydrag.mjs:887`) is a stub: `innerHTML` drops text nodes, `hidden` is a plain property, `classList` has add/remove/toggle/contains, `toggleAttribute` exists, `fire(type)` bubbles through `parentNode`, selectors support tag, class, `[attr]`, `[attr="v"]`, descendant and comma lists. The kit is written for that subset (it builds markup with `core.el`, sets texts through `textContent`).
- The easydrag modules are **not** in a UI bundle (`grep -c easydrag scripts/build-ui-bundles.js` is 0), so no `npm run build:ui` and no `node_modules` copy is needed for Part B.
- Stylesheet: 1,752 lines; 110 `font-size` declarations plus 5 `font:` shorthands with a literal size; 68 `border-radius` declarations with a literal length; 16 colour literals outside `--ed-*` token definitions (measured on 9492395de; Part A lowers some).

**Preamble (run once, before Task B.1).**

- [ ] **Step 0.1: verify Part A on main**

```
cd C:/Users/Andi/Documents/repo/AuraGo && git log --oneline -8 && git status --short | head -5
for t in font-title font-head font-card font-body font-meta font-label radius-sm radius-md radius-lg radius-pill space-1 space-6 motion-fast motion-base motion-slow ease-out status-neutral; do printf "%s: " "$t"; grep -c -- "--ed-$t:" ui/css/desktop-app-easydrag.css; done
grep -c "ed-status-disc" ui/css/desktop-app-easydrag.css
grep -n "easydrag-status.js" ui/js/desktop/core/module-loader.js ui/desktop_easydrag_test.go | head
grep -n -- "--ed-status-ink\|--ed-radius-lg:" ui/css/desktop-app-easydrag.css | head
node -e "const vm=require('vm'),fs=require('fs');const b=vm.createContext({window:{},document:{},console});for(const f of ['easydrag-core.js','easydrag-status.js']) vm.runInContext(fs.readFileSync('ui/js/desktop/apps/'+f,'utf8'),b);const s=b.window.EasyDrag.status;console.log(typeof s.disc, typeof s.label, s.VALUES.length)"
```
Expected: Part A's commits at the top of the log, no local changes (the `ui/desktop_ha_switchboard_browser_test.go`, bundle and wallpaper edits of other sessions are not yours; ignore them in the main checkout), every token count `1` (a count of 2 or more is fine when a band redefines it), a `.ed-status-disc` count above 0, `easydrag-status.js` registered in both lists, and `function function 12` from the node line. For each outcome:
  - A token is missing, or `ED.status` lacks `disc`/`label`: Part A is not complete. Stop and tell the coordinator.
  - `--ed-radius-xl` must be 12px and `--ed-radius-lg` 14px (Part A kept the old value of `-lg`). The kit CSS uses `--ed-radius-xl` for callout, toast and sheet. Check with `grep -n -- "--ed-radius-" ui/css/desktop-app-easydrag.css`; if the names differ, use what Part A reports.
  - `--ed-status-ink` must exist (Part A). Read the status lint, `ui/desktop_easydrag_status_test.go`, and note what it flags outside `easydrag-status.js` and its CSS block: B.1 step 1.6 avoids the `ed-status-` class and the `--ed-status-` names in kit CSS by going through two aliases in the token block, so the lint cannot trip on them. If the lint does not look at CSS custom properties, the aliases are harmless.
  - `--ed-font-card` must be 16px and `--ed-font-body` 14px (the owner scale). If Part A defined 15px and 13px, change those two token values first (a one-line edit in the token block, in the B.5 commit) and say so.
  - The module list places `easydrag-status.js` somewhere: B.1 registers the kit right after it.

- [ ] **Step 0.2: create the worktree**

```
cd C:/Users/Andi/Documents/repo/AuraGo && git worktree add .worktrees/easydrag-w1a-kit -b feat/easydrag-w1a-kit main && cd .worktrees/easydrag-w1a-kit && git branch --show-current && git log --oneline -1
```
Expected: `feat/easydrag-w1a-kit` and main's head commit. (Other sessions commit in the main checkout; the worktree is cut from main at this moment.)

- [ ] **Step 0.3: scratch directories**

```
mkdir -p C:/disposable/easydrag-w1a D:/tmp/go-build D:/tmp/easydrag-w1a-shots
```

- [ ] **Step 0.4: baseline T0**

```
cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-w1a-kit && GOTOOLCHAIN=go1.27.4 GOTMPDIR='D:\tmp\go-build' npm run test:easydrag:t0 2>&1 | tail -8
```
Expected: every job `ok`/`PASS`, exit 0 (35-55 s). If a job fails here the failure is not yours: report it before you start.

- [ ] **Step 0.5: baseline of one full gated browser run, with counts**

```
cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-w1a-kit && AURAGO_RUN_BROWSER_SMOKE=1 AURAGO_BROWSER_ARTIFACT_DIR=D:/tmp/easydrag-w1a-shots GOTOOLCHAIN=go1.27.4 GOTMPDIR='D:\tmp\go-build' go test ./ui -run '(?i)easydrag' -count=1 -timeout 30m -v 2>&1 | tee D:/tmp/easydrag-w1a-kit-baseline.txt | grep -c -- "^--- PASS"; grep -c -- "^--- SKIP" D:/tmp/easydrag-w1a-kit-baseline.txt; grep -- "^--- FAIL" D:/tmp/easydrag-w1a-kit-baseline.txt; tail -2 D:/tmp/easydrag-w1a-kit-baseline.txt
```
Run it with the Bash sandbox off; it takes 5-6 minutes. Expected on main before Part A: 41 PASS, 2 SKIP; after Part A the PASS count may be higher. Write down the two counts: they are the gate for B.2 to B.5 (every later run must show the same PASS count plus the tests this part adds, and no FAIL). A FAIL here is not yours: report it first.

---

### Task B.1: `easydrag-kit.js` (disclosure, callout, chip, toast, toolbar) and its CSS

**Files:**
- Create: `ui/js/desktop/apps/easydrag-kit.js`
- Create: `scripts/test-easydrag-kit.mjs`
- Modify: `scripts/test-easydrag.mjs` (one `run` line after line 1141)
- Modify: `ui/js/desktop/core/module-loader.js` (easydrag entry, around 281-322)
- Modify: `ui/desktop_easydrag_test.go:11-16` (`easyDragModules`)
- Modify: `scripts/test-easydrag-extra.mjs:9`, `-extra2.mjs:9-10`, `-extra3.mjs:9-11`, `-extra4.mjs:11-12` (the `MODULES` lists)
- Modify: `ui/css/desktop-app-easydrag.css` (callout rules ~399-408, `.ed-details summary` ~413, toast ~1573-1581, chip variants; new kit block)
- Modify: `ui/js/desktop/apps/easydrag.js:222` (one added line)

**Design.**

- `ED.kit.disclosure({ title, count, open, body, id, className, attrs, headLabel })` returns the element; its head is `button.ed-disclosure-head[aria-expanded][aria-controls]` (a real button, so Enter and Space toggle it with no key code), the body is `.ed-disclosure-body` (hidden when closed). The element form binds its own head. `ED.kit.disclosureMarkup(opts)` returns the same markup as a string for the many call sites that build innerHTML templates; their heads work through one delegated listener, `ED.kit.attach(root)`, called once for `.ed-app` (`easydrag.js`). Helpers: `isOpen(root)`, `setOpen(root, open)`, `setTitle(root, title)`, `setCount(root, count)`, `body(root)`.
- `callout({ status, text, action })`: `.ed-callout[data-status]` with the status disc from `ED.status.disc(status, 18)` in front. Owner decision 6: tinted surface, no border. `status: 'info'` (the default) uses a small own disc, because Part A's twelve values have no neutral "information" glyph (deviation, see the end). The tint is the status colour mixed into the panel; the text stays in `--ed-text`.
- `chip({ text, glyph })`: `.ed-chip.ed-chip--neutral`, neutral ink and a glyph from the `ICONS` set, never a warning token.
- `toast({ status, text, action, host, t... })`: an in-app toast with the status disc and at most one action (see B.3 for the use). Without `host` it wraps `notify` (pass `ed.ctx.notify`): the desktop toast takes a title and message only, so the action is not shown there.
- `toolbar(items)`: `.ed-toolbar` row of `.ed-btn.ed-toolbar-item`, equal height and spacing. W1a uses it nowhere new.

- [ ] **Step 1.1: write the failing Node test**

Create `scripts/test-easydrag-kit.mjs`:

```js
// W1a Part B (FD-09): the kit. disclosure toggles aria-expanded and shows its body, callout carries the
// status disc and no border, chip is neutral, toast carries an action, modal() below 600px gets the
// sheet class. Runs on the miniDom stub of test-easydrag.mjs, which calls run(env); alone this file
// only has to import.
import fs from 'node:fs';
import path from 'node:path';
import vm from 'node:vm';
import { webcrypto } from 'node:crypto';

export async function run({ apps, miniDom, eq }) {
    const dom = miniDom();
    const timers = [];
    const box = vm.createContext({
        window: {}, navigator: { platform: 'Linux' }, crypto: webcrypto, document: dom.document, console, Intl, Date, Set, Map,
        requestAnimationFrame: fn => { fn(); return 1; },
        setTimeout: (fn, ms) => { timers.push({ fn, ms }); return timers.length; },
        clearTimeout: id => { if (timers[id - 1]) timers[id - 1].fn = null; }
    });
    for (const file of ['easydrag-core.js', 'easydrag-kit.js']) vm.runInContext(fs.readFileSync(path.join(apps, file), 'utf8'), box, { filename: file });
    const ED = box.window.EasyDrag;
    ED.status = { disc: (value, size) => '<span class="ed-status-disc" data-status="' + value + '" role="img" aria-label="' + value + '"></span>' };
    const kit = ED.kit;

    // disclosure: closed at first, the head toggles it, the body follows, the head names the body
    const d = kit.disclosure({ title: 'Notes', count: 3, body: '<p class="ed-hint">x</p>' });
    const head = d.querySelector('.ed-disclosure-head'), body = d.querySelector('.ed-disclosure-body');
    eq('kit disclosure starts closed and names its body',
        [head.getAttribute('aria-expanded'), body.getAttribute('hidden') !== null, head.getAttribute('aria-controls') === body.id && body.id !== '', head.localName, kit.isOpen(d)],
        ['false', true, true, 'button', false]);
    eq('kit disclosure shows title and count', [d.querySelector('.ed-disclosure-title').textContent, d.querySelector('.ed-disclosure-count').textContent], ['Notes', '3']);
    head.fire('click');
    eq('kit disclosure opens on a click', [head.getAttribute('aria-expanded'), body.getAttribute('hidden'), kit.isOpen(d)], ['true', null, true]);
    head.fire('click');
    eq('kit disclosure closes on the next click', [head.getAttribute('aria-expanded'), body.getAttribute('hidden') !== null], ['false', true]);
    kit.setOpen(d, true); kit.setTitle(d, 'Other'); kit.setCount(d, '');
    eq('kit disclosure setOpen, setTitle and setCount', [kit.isOpen(d), d.querySelector('.ed-disclosure-title').textContent, d.querySelector('.ed-disclosure-count').getAttribute('hidden') !== null], [true, 'Other', true]);
    eq('kit disclosure open option', kit.isOpen(kit.disclosure({ title: 'a', open: true })), true);

    // the markup form works through one delegated listener on the app root
    const root = new dom.El('div', { class: 'ed-app' });
    kit.attach(root);
    const holder = root.appendChild(new dom.El('div', {}));
    holder.innerHTML = kit.disclosureMarkup({ title: 'A', className: 'ed-x', attrs: { 'data-ed-probe': '' }, open: false, body: '<p>b</p>' });
    const marked = holder.querySelector('.ed-x');
    eq('kit markup form carries class and attribute and starts closed', [!!marked, marked.getAttribute('data-ed-probe'), kit.isOpen(marked)], [true, '', false]);
    marked.querySelector('.ed-disclosure-head').fire('click');
    eq('kit markup form toggles through the delegated listener', kit.isOpen(marked), true);
    const own = kit.disclosure({ title: 'own' });
    root.appendChild(own);
    own.querySelector('.ed-disclosure-head').fire('click');
    eq('kit element form is not toggled twice under an attached root', kit.isOpen(own), true);

    // callout: disc in front, status on the element, no legacy variant class
    const c = kit.callout({ status: 'failed', text: 'Broken', action: { label: 'Retry', onClick: () => { c.fired = true; } } });
    eq('kit callout carries the status disc and the status',
        [c.getAttribute('data-status'), !!c.querySelector('.ed-status-disc'), c.className, c.querySelector('.ed-callout-text').textContent, c.querySelector('.ed-callout-action') !== null],
        ['failed', true, 'ed-callout', 'Broken', true]);
    c.querySelector('.ed-callout-action').fire('click');
    eq('kit callout action runs', c.fired, true);
    const info = kit.callout({ text: 'Hint' });
    eq('kit callout defaults to info with its own disc', [info.getAttribute('data-status'), !!info.querySelector('.ed-callout-disc')], ['info', true]);
    const css = fs.readFileSync(path.join(apps, '../../../css/desktop-app-easydrag.css'), 'utf8').replace(/\r\n/g, '\n');
    const rule = /\n\.ed-callout \{([^}]*)\}/.exec(css);
    eq('kit callout rule has no border and no input-like inset', [!!rule, rule && /border:\s*0/.test(rule[1]), rule && /inset/.test(rule[1])], [true, true, false]);

    // chip: neutral, optional glyph
    const chip = kit.chip({ text: 'sends mail', glyph: 'send' });
    eq('kit chip is neutral with a glyph', [chip.className, chip.querySelector('.ed-chip-text').textContent, !!chip.querySelector('svg')], ['ed-chip ed-chip--neutral', 'sends mail', true]);

    // toolbar: one button per item, order kept
    let picked = '';
    const bar = kit.toolbar([{ id: 'a', label: 'A', onClick: () => { picked += 'a'; } }, null, { id: 'b', label: 'B', icon: 'play', pressed: true, onClick: () => { picked += 'b'; } }]);
    const items = bar.querySelectorAll('.ed-toolbar-item');
    items[0].fire('click'); items[1].fire('click');
    eq('kit toolbar builds equal items', [bar.className, items.length, items[1].getAttribute('aria-pressed'), picked], ['ed-toolbar', 2, 'true', 'ab']);

    // toast: disc, text, one action; the action hides it first and then runs
    const host = new dom.El('div', {});
    let acted = 0, hidden = 0;
    const toast = kit.toast({ status: 'ok', text: 'Gone', host, closeLabel: 'Close', action: { label: 'Undo', onClick: () => { acted++; } }, onHide: () => { hidden++; } });
    const el = host.querySelector('[data-ed-toast]');
    eq('kit toast shows disc, text and action', [el.hidden, !!el.querySelector('.ed-status-disc'), el.querySelector('[data-ed-toast-text]').textContent, el.querySelector('[data-ed-toast-action] span').textContent], [false, true, 'Gone', 'Undo']);
    el.querySelector('[data-ed-toast-action]').fire('click');
    eq('kit toast action hides the toast and runs once', [el.hidden, acted, hidden], [true, 1, 1]);
    toast.show('Again');
    eq('kit toast shows again with a new text and closes after its time', [el.hidden, el.querySelector('[data-ed-toast-text]').textContent, timers.some(t => t.fn && t.ms === 8000)], [false, 'Again', true]);
    el.querySelector('[data-ed-toast-close]').fire('click');
    eq('kit toast close button hides it', el.hidden, true);
    toast.dispose();
    eq('kit toast dispose removes it', host.querySelector('[data-ed-toast]'), null);
    let note = null;
    const none = kit.toast({ status: 'failed', text: 'Failed', notify: n => { note = n; } });
    eq('kit toast without a host wraps notify', [none, note], [null, { title: 'EasyDrag', message: 'Failed', type: 'error' }]);

    // modal(): the sheet class below 600 px of window width, with a grab bar in every dialog
    const open = width => {
        const win = new dom.El('div', { class: 'ed-app' });
        win.clientWidth = width;
        const dialog = ED.core.modal(win, { title: 'T', closeLabel: 'Close', body: '<p>b</p>', actions: [{ id: 'ok', label: 'OK', primary: true }] });
        return dialog;
    };
    const flags = [390, 599, 600, 1280].map(w => open(w).el.classList.contains('ed-sheet'));
    eq('kit modal() is a sheet below 600 px and a dialog from 600 px', flags, [true, true, false, false]);
    eq('kit modal() has a grab bar', !!open(390).el.querySelector('.ed-sheet-grab'), true);
    const noWidth = new dom.El('div', {});
    eq('kit modal() with an unmeasured host stays a dialog', ED.core.modal(noWidth, { title: 'T', body: '' }).el.classList.contains('ed-sheet'), false);
}
```

Register the run in `scripts/test-easydrag.mjs`. Insert after the line `await (await import('./test-easydrag-pack.mjs')).run({ pack: editorHarness.sandbox().ED.pack, eq });` (about 1141):

```js
// ── W1a B: the kit (test-easydrag-kit.mjs) ──
await (await import('./test-easydrag-kit.mjs')).run({ apps, miniDom, eq });
```

Run it (it must fail because the module is missing):

```
cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-w1a-kit && node scripts/test-easydrag.mjs 2>&1 | tail -6
```
Expected: an `ENOENT` for `easydrag-kit.js` and no `all EasyDrag checks passed`.

- [ ] **Step 1.2: write the kit**

Create `ui/js/desktop/apps/easydrag-kit.js`:

```js
// EasyDrag kit (W1a, FD-09): the small parts every surface shares, drawn from the tokens of the
// stylesheet. disclosure replaces the native details element (a button head with aria-expanded and
// aria-controls, so Enter and Space work), callout is the tinted hint with a status disc and no
// border, chip is the neutral label, toast is the in-app message with at most one action, toolbar is a
// row of equal controls. disclosureMarkup feeds the string templates; attach(root) makes the heads of
// that markup work through one delegated click listener (easydrag.js calls it for .ed-app).
(function () {
    'use strict';

    const ED = window.EasyDrag = window.EasyDrag || {};
    // TOAST_MS is how long a toast stays while neither the pointer nor the focus is on it.
    const TOAST_MS = 8000;
    let serial = 0;

    const esc = value => ED.core.esc(value);
    const text = value => (value == null ? '' : String(value));
    const has = value => value != null && value !== '';

    function head(root) { return root ? root.querySelector('.ed-disclosure-head') : null; }
    function body(root) { return root ? root.querySelector('.ed-disclosure-body') : null; }
    function isOpen(root) { const h = head(root); return !!h && h.getAttribute('aria-expanded') === 'true'; }
    function setOpen(root, open) {
        const h = head(root), b = body(root);
        if (!h || !b) return;
        h.setAttribute('aria-expanded', open ? 'true' : 'false');
        b.toggleAttribute('hidden', !open);
    }
    function setTitle(root, title) {
        const node = root && root.querySelector('.ed-disclosure-title');
        if (node) node.textContent = text(title);
    }
    function setCount(root, count) {
        const node = root && root.querySelector('.ed-disclosure-count');
        if (!node) return;
        node.textContent = text(count);
        node.toggleAttribute('hidden', !has(count));
    }
    function toggle(root) { setOpen(root, !isOpen(root)); }

    // disclosureMarkup returns the disclosure as a string: opts {title, count, open, body (markup), id,
    // className, attrs {name: value}, headLabel}. title, count, headLabel and attrs are escaped; body is markup.
    function disclosureMarkup(opts) {
        const o = opts || {};
        const id = o.id || 'ed-disc-' + (++serial);
        const open = !!o.open;
        const attrs = Object.keys(o.attrs || {}).map(name => ' ' + name + '="' + esc(o.attrs[name]) + '"').join('');
        return '<div class="ed-disclosure' + (o.className ? ' ' + esc(o.className) : '') + '"' + attrs + '>' +
            '<button type="button" class="ed-disclosure-head" aria-expanded="' + (open ? 'true' : 'false') + '" aria-controls="' + esc(id) + '"' + (o.headLabel ? ' aria-label="' + esc(o.headLabel) + '"' : '') + '>' +
            '<span class="ed-disclosure-chev" aria-hidden="true"></span><span class="ed-disclosure-title">' + esc(o.title) + '</span>' +
            '<span class="ed-disclosure-count"' + (has(o.count) ? '' : ' hidden') + '>' + esc(o.count) + '</span></button>' +
            '<div class="ed-disclosure-body" id="' + esc(id) + '"' + (open ? '' : ' hidden') + '>' + (o.body || '') + '</div></div>';
    }

    // disclosure returns the element; its head is bound, so it needs no attached root.
    function disclosure(opts) {
        const o = opts || {};
        const root = ED.core.el(disclosureMarkup(o));
        setTitle(root, o.title);
        setCount(root, o.count);
        const h = head(root);
        h.edBound = true;
        h.addEventListener('click', () => toggle(root));
        return root;
    }

    // attach makes the heads of disclosureMarkup work under root (one listener, also for markup that arrives later).
    function attach(root) {
        root.addEventListener('click', event => {
            const h = event.target && event.target.closest ? event.target.closest('.ed-disclosure-head') : null;
            if (h && !h.edBound) toggle(h.parentNode);
        });
    }

    const INFO_DISC = '<span class="ed-callout-disc" aria-hidden="true"><svg viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="2.2" stroke-linecap="round"><path d="M8 7v4.5M8 4.6v.1"/></svg></span>';

    // callout is the hint box: opts {status, text, action: {label, onClick}, role}. status is one of the ED.status
    // values, or 'info' (the default) for a neutral piece of information.
    function callout(opts) {
        const o = opts || {};
        const status = o.status || 'info';
        const lead = status === 'info' || !ED.status ? INFO_DISC : ED.status.disc(status, 18);
        const action = o.action && o.action.label ? '<button type="button" class="ed-callout-action"></button>' : '';
        const el = ED.core.el('<div class="ed-callout" data-status="' + esc(status) + '"' + (o.role ? ' role="' + esc(o.role) + '"' : '') + '>' + lead + '<div class="ed-callout-text"></div>' + action + '</div>');
        el.querySelector('.ed-callout-text').textContent = text(o.text);
        const button = el.querySelector('.ed-callout-action');
        if (button) {
            button.textContent = text(o.action.label);
            if (typeof o.action.onClick === 'function') button.addEventListener('click', o.action.onClick);
        }
        return el;
    }

    // chip is the neutral label: opts {text, glyph (a name of core.ICONS)}.
    function chip(opts) {
        const o = opts || {};
        const el = ED.core.el('<span class="ed-chip ed-chip--neutral">' + (o.glyph ? ED.core.icon(o.glyph) : '') + '<span class="ed-chip-text"></span></span>');
        el.querySelector('.ed-chip-text').textContent = text(o.text);
        return el;
    }

    // toolbar is a row of equal controls: items [{id, label, icon, title, pressed, disabled, iconOnly, onClick}].
    function toolbar(items) {
        const el = ED.core.el('<div class="ed-toolbar" role="group"></div>');
        (items || []).filter(Boolean).forEach(item => {
            const b = ED.core.el('<button type="button" class="ed-btn ed-toolbar-item">' + (item.icon ? ED.core.icon(item.icon) : '') + '<span class="ed-toolbar-label"></span></button>');
            b.querySelector('.ed-toolbar-label').textContent = text(item.label);
            if (item.id) b.setAttribute('data-ed-toolbar', item.id);
            if (item.title) b.setAttribute('title', item.title);
            if (item.iconOnly) b.setAttribute('aria-label', text(item.label));
            if (item.pressed != null) b.setAttribute('aria-pressed', item.pressed ? 'true' : 'false');
            if (item.disabled) b.disabled = true;
            if (typeof item.onClick === 'function') b.addEventListener('click', item.onClick);
            el.appendChild(b);
        });
        return el;
    }

    // toast: opts {status, text, action: {label, icon, onClick}, host, closeLabel, duration (ms, 0 = stays),
    // start (false: stay hidden until show()), onHide({hadFocus}), live, notify}. With a host it builds the
    // in-app toast and returns {el, show(message), hide(), isShown(), dispose()}. Without a host it wraps
    // opts.notify (the desktop notification takes a title and a message and has no action) and returns null.
    // The toast holds while the pointer or the focus is on it. The action hides the toast first and then
    // runs, so the focus goes back before the action acts.
    function toast(opts) {
        const o = opts || {};
        if (!o.host) {
            if (typeof o.notify === 'function') o.notify({ title: o.title || 'EasyDrag', message: text(o.text), type: o.status === 'failed' ? 'error' : undefined });
            return null;
        }
        const lead = o.status && ED.status ? ED.status.disc(o.status, 18) : '';
        const action = o.action && o.action.label ? '<button type="button" class="ed-toast-action" data-ed-toast-action>' + (o.action.icon ? ED.core.icon(o.action.icon) : '') + '<span></span></button>' : '';
        const close = o.closeLabel ? '<button type="button" class="ed-icon-btn" data-ed-toast-close aria-label="' + esc(o.closeLabel) + '" title="' + esc(o.closeLabel) + '">' + ED.core.icon('x') + '</button>' : '';
        const el = ED.core.el('<div class="ed-toast" data-ed-toast' + (o.live ? ' role="status"' : '') + ' hidden>' + lead + '<span class="ed-toast-text" data-ed-toast-text></span>' + action + close + '</div>');
        el.hidden = true;
        if (action) el.querySelector('[data-ed-toast-action] span').textContent = text(o.action.label);
        const duration = o.duration == null ? TOAST_MS : o.duration;
        let timer = 0;
        const arm = () => { clearTimeout(timer); timer = duration > 0 ? setTimeout(hide, duration) : 0; };
        function hide() {
            clearTimeout(timer);
            timer = 0;
            if (el.hidden) return;
            const hadFocus = el.contains(document.activeElement);
            el.hidden = true;
            if (typeof o.onHide === 'function') o.onHide({ hadFocus });
        }
        function show(message) {
            if (message !== undefined) el.querySelector('[data-ed-toast-text]').textContent = text(message);
            el.hidden = false;
            arm();
        }
        el.addEventListener('focusin', () => clearTimeout(timer));
        el.addEventListener('focusout', () => { if (!el.hidden) arm(); });
        el.addEventListener('pointerenter', () => clearTimeout(timer));
        el.addEventListener('pointerleave', () => { if (!el.hidden) arm(); });
        el.addEventListener('click', event => {
            if (event.target.closest('[data-ed-toast-close]')) { hide(); return; }
            if (!event.target.closest('[data-ed-toast-action]')) return;
            hide();
            if (typeof o.action.onClick === 'function') o.action.onClick();
        });
        o.host.appendChild(el);
        if (o.start !== false) show(o.text);
        else el.querySelector('[data-ed-toast-text]').textContent = text(o.text);
        return { el, show, hide, isShown: () => !el.hidden, dispose() { clearTimeout(timer); el.remove(); } };
    }

    ED.kit = { disclosure, disclosureMarkup, attach, isOpen, setOpen, setTitle, setCount, body, callout, chip, toast, toolbar };
})();
```

- [ ] **Step 1.3: register the module in the four places**

In `ui/js/desktop/core/module-loader.js` add `'/js/desktop/apps/easydrag-kit.js',` on its own line right after the line `'/js/desktop/apps/easydrag-status.js',` (the line Part A added; if the status module is not in the list, put it right after `'/js/desktop/apps/easydrag-core.js',`). In `ui/desktop_easydrag_test.go` add `"easydrag-kit.js", ` right after `"easydrag-status.js", ` (or after `"easydrag-core.js", `) in `easyDragModules`. Then the Node harness lists; this command inserts the kit after `'easydrag-core.js'` in every list that loads the core (the kit only needs the core at load, so the position right after it is safe) and prints each file it changed:

```
cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-w1a-kit && node -e "
const fs=require('fs');
for (const f of ['scripts/test-easydrag-extra.mjs','scripts/test-easydrag-extra2.mjs','scripts/test-easydrag-extra3.mjs','scripts/test-easydrag-extra4.mjs']) {
  let s=fs.readFileSync(f,'utf8'); const n=(s.match(/'easydrag-core\.js', /g)||[]).length;
  s=s.replace(/'easydrag-core\.js', /g, \"'easydrag-core.js', 'easydrag-kit.js', \");
  fs.writeFileSync(f,s); console.log(f, n);
}"
git diff --stat scripts/ | tail -6
```
Expected: `extra.mjs 1`, `extra2.mjs 1`, `extra3.mjs 1`, `extra4.mjs 2` (it has `MODULES` and `UI_MODULES`) and four changed files. If a count is 0 the list formatting differs: open the file, find the list with `grep -n "easydrag-core.js" <file>` and add the entry by hand. Other scripts that load only a few modules (`test-easydrag-transform.mjs`, `-authoring.mjs`, `-child-history.mjs`, `-scenarios.mjs`) are handled in B.4, where the failing run shows which one needs the kit.

Add the delegated listener once: in `ui/js/desktop/apps/easydrag.js` line 222-223, change

```js
        const root = core.el('<div class="ed-app"></div>');
        container.innerHTML = '';
```
to
```js
        const root = core.el('<div class="ed-app"></div>');
        if (ED.kit) ED.kit.attach(root);
        container.innerHTML = '';
```

- [ ] **Step 1.4: run the Node test; it passes**

```
cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-w1a-kit && node scripts/test-easydrag.mjs 2>&1 | grep -i "kit\|FAIL\|all EasyDrag" | tail -8
```
Expected: no `FAIL` line and `all EasyDrag checks passed`. The callout CSS check inside the test still fails until step 1.6 is done: if you run now, expect exactly the line `FAIL kit callout rule has no border and no input-like inset`; that is the next steps' job. (Run step 1.4 again after 1.6.)

- [ ] **Step 1.5: the Go module-order test**

```
cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-w1a-kit && GOTOOLCHAIN=go1.27.4 GOTMPDIR='D:\tmp\go-build' go test ./ui -run 'TestEasyDragAssetsLoadInDependencyOrder' -count=1 2>&1 | tail -3
```
Expected: `ok  	github.com/.../ui`. If it says `easydrag assets miss easydrag-kit.js`, step 1.3 missed a list.

- [ ] **Step 1.6: the CSS**

1. Two aliases in the token block (right after the line that defines `--ed-on-accent`), so the kit rules below never spell a status name: `    --ed-kit-ink: var(--ed-status-ink);` and `    --ed-kit-neutral: var(--ed-status-neutral);`. They are token definitions (the ratchet of B.6 ignores them).
2. Replace the callout rules (the block from `.ed-callout {` to `.ed-callout--warn > .ed-icon { color: var(--ed-warn); }`, about 399-408) with:

```css
/* callout (kit): a tinted surface with a status disc in front and no border, so it is never taken for a field.
   The tint is the status colour mixed into the panel; the text stays in normal ink. The older bare and --warn/--error
   markup takes the same look until it moves to ED.kit.callout. */
.ed-callout {
    --ed-callout-hue: var(--ed-info);
    display: flex; gap: var(--ed-space-3); align-items: flex-start; margin: var(--ed-space-3) 0; padding: var(--ed-space-2) var(--ed-space-3);
    border: 0; border-radius: var(--ed-radius-xl); line-height: 1.45; color: var(--ed-text);
    background: color-mix(in srgb, var(--ed-callout-hue) 14%, var(--ed-panel-strong));
}
.ed-callout > .ed-icon { margin-top: 2px; color: var(--ed-callout-hue); }
.ed-callout p { margin: 4px 0 0; }
.ed-callout--warn, .ed-callout[data-status="uncertain"], .ed-callout[data-status="blocked"] { --ed-callout-hue: var(--ed-warn); }
.ed-callout--error, .ed-callout[data-status="failed"] { --ed-callout-hue: var(--ed-danger); }
.ed-callout[data-status="ok"], .ed-callout[data-status="stopped"] { --ed-callout-hue: var(--ed-ok); }
.ed-callout[data-status="info"], .ed-callout[data-status="running"], .ed-callout[data-status="waiting"] { --ed-callout-hue: var(--ed-info); }
.ed-callout[data-status="skipped"], .ed-callout[data-status="paused"], .ed-callout[data-status="simulated"],
.ed-callout[data-status="not_run"], .ed-callout[data-status="cancelled"] { --ed-callout-hue: var(--ed-kit-neutral); }
.ed-callout > [role="img"], .ed-callout > .ed-callout-disc { flex: none; margin-top: 1px; }
.ed-callout-disc { display: inline-grid; place-items: center; width: 18px; height: 18px; border-radius: var(--ed-radius-pill); background: var(--ed-info); color: var(--ed-kit-ink); }
.ed-callout-disc svg { width: 12px; height: 12px; }
.ed-callout-action { flex: none; min-height: 32px; padding: 0 var(--ed-space-2); border: 0; border-radius: var(--ed-radius-md); background: transparent; color: var(--ed-info-ink); font: inherit; font-weight: 600; cursor: pointer; }
.ed-callout-action:focus-visible { outline: 2px solid var(--ed-focus); outline-offset: 2px; }
```
3. Remove the line `.ed-details summary { cursor: pointer; color: var(--ed-muted); font-weight: 600; margin-top: 10px; }` (~413) and add the kit block right after the `.ed-section-title` line:

```css
/* disclosure (kit): the head is a button, so Enter and Space work; the chevron is drawn with two borders */
.ed-disclosure { margin-top: var(--ed-space-2); }
.ed-disclosure-head {
    display: flex; align-items: center; gap: var(--ed-space-2); width: 100%; min-height: 32px; padding: var(--ed-space-1) var(--ed-space-2);
    border: 0; border-radius: var(--ed-radius-md); background: transparent; color: var(--ed-text); font: inherit; font-size: var(--ed-font-body); text-align: left; cursor: pointer;
}
.ed-disclosure-head:hover { background: var(--ed-control-hover); }
.ed-disclosure-head:focus-visible { outline: 2px solid var(--ed-focus); outline-offset: 2px; }
.ed-disclosure-chev { flex: none; width: 7px; height: 7px; margin: 0 var(--ed-space-1) 0 2px; border-right: 2px solid var(--ed-muted); border-bottom: 2px solid var(--ed-muted); transform: rotate(-45deg); transition: transform var(--ed-motion-fast) var(--ed-ease-out); }
.ed-disclosure-head[aria-expanded="true"] .ed-disclosure-chev { transform: rotate(45deg); }
.ed-disclosure-title { flex: 1 1 auto; min-width: 0; overflow-wrap: anywhere; font-weight: 600; }
.ed-disclosure-count { flex: none; color: var(--ed-muted); font-size: var(--ed-font-meta); font-variant-numeric: tabular-nums; }
.ed-disclosure-body { padding: var(--ed-space-1) var(--ed-space-2) var(--ed-space-2) calc(var(--ed-space-2) + 14px); }
.ed-disclosure-body[hidden] { display: none; }
/* toolbar (kit) */
.ed-toolbar { display: flex; flex-wrap: wrap; align-items: center; gap: var(--ed-space-2); }
.ed-toolbar-item { min-height: 32px; }
```
and after the existing `.ed-chip--risk` line add `.ed-chip--neutral { min-height: 24px; padding: var(--ed-space-1) 10px; background: color-mix(in srgb, var(--ed-text) 10%, var(--ed-panel-strong)); color: var(--ed-text); font-weight: 600; }`.
4. Replace the toast rules (`/* "deleted – Undo" toast …` comment and the `.ed-toast {…}` and `.ed-toast-text` rules, about 1573-1581) with:

```css
/* toast (kit, easydrag-kit.js): above the canvas, below popovers (26) and dialogs (40); no animation */
.ed-toast {
    position: absolute; left: 50%; bottom: var(--ed-space-4); z-index: 20; transform: translateX(-50%);
    display: flex; align-items: center; gap: var(--ed-space-3); max-width: calc(100% - 32px); padding: var(--ed-space-2) var(--ed-space-2) var(--ed-space-2) var(--ed-space-3);
    border-radius: var(--vd-mat-card-radius, var(--ed-radius-xl)); background: var(--ed-panel-strong); color: var(--ed-text);
    border: 1px solid var(--ed-border-strong); box-shadow: var(--ed-shadow-lg);
}
.ed-toast > [role="img"] { flex: none; }
.ed-toast-text { min-width: 0; overflow-wrap: anywhere; font-weight: 600; }
.ed-toast-action { display: inline-flex; align-items: center; gap: var(--ed-space-1); flex: none; min-height: 32px; padding: 0 var(--ed-space-2); border: 0; border-radius: var(--ed-radius-md); background: transparent; color: var(--ed-info-ink); font: inherit; font-weight: 600; cursor: pointer; }
.ed-toast-action .ed-icon { width: 14px; height: 14px; }
.ed-toast-action:hover { background: var(--ed-control-hover); }
.ed-toast-action:focus-visible { outline: 2px solid var(--ed-focus); outline-offset: 2px; }
```
(The `.ed-toast` background and radius keep the material tokens, spec §4.2: material radii keep applying.)

- [ ] **Step 1.7: gates**

```
cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-w1a-kit && node scripts/test-easydrag.mjs 2>&1 | tail -2 && node scripts/test-easydrag-render-baseline.mjs 2>&1 | tail -1 && GOTOOLCHAIN=go1.27.4 GOTMPDIR='D:\tmp\go-build' go test ./ui -run '(?i)(easydrag|ShippedWebAssetsLoadNothingExternal)' -count=1 2>&1 | tail -3 && gofmt -l ui/desktop_easydrag_test.go
```
Expected: `all EasyDrag checks passed`, `PASS render baseline (unchanged)`, `ok`, and no gofmt output. The legacy toast has no user yet on the kit (B.3 moves it), and the old `.ed-toast` markup is unchanged, so only its CSS changed.

- [ ] **Step 1.8: commit**

Write `C:/disposable/easydrag-w1a/msg-b1.txt`:
```
feat(easydrag): add the kit (disclosure, callout, chip, toast, toolbar)

ED.kit in easydrag-kit.js: disclosure (a button head with aria-expanded and
aria-controls, element and markup form, one delegated listener on .ed-app),
callout (tinted surface with a status disc, no border), chip (neutral), toast
(in-app, disc and one action, or a wrapper of notify) and toolbar. The old
.ed-callout rules take the tinted look; the toast CSS moves onto the tokens.
Registered in the loader, the Go module list and the Node harness lists.

GitNexus detect-changes unavailable in the worktree; judged by symbols and tests.

Co-Authored-By: Claude Sonnet 5.5 <noreply@anthropic.com>
```
```
cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-w1a-kit && git add ui/js/desktop/apps/easydrag-kit.js ui/js/desktop/apps/easydrag.js ui/js/desktop/core/module-loader.js ui/desktop_easydrag_test.go ui/css/desktop-app-easydrag.css scripts/test-easydrag-kit.mjs scripts/test-easydrag.mjs scripts/test-easydrag-extra.mjs scripts/test-easydrag-extra2.mjs scripts/test-easydrag-extra3.mjs scripts/test-easydrag-extra4.mjs && git commit -F C:/disposable/easydrag-w1a/msg-b1.txt && git status --short | head -3
```
Expected: one commit, clean status.

---

### Task B.2: the sheet (`modal()` below 600px)

**Files:**
- Modify: `ui/js/desktop/apps/easydrag-core.js:439-513` (`modal`)
- Modify: `ui/css/desktop-app-easydrag.css` (after the `.ed-modal-foot` rules, about 1189)
- Create: `ui/desktop_easydrag_kit_browser_test.go`
- Test: `scripts/test-easydrag-kit.mjs` (already holds the `modal()` checks from B.1; they fail now)

**Design.** `modal()` measures its host (`host.clientWidth`, the `.ed-app` window content) when it opens and toggles `ed-sheet` on `.ed-modal`; with `ResizeObserver` it follows later resizes. Below 600px the dialog is full width, bottom-aligned, rounded on top, with a grab bar. Every dialog gets a `<span class="ed-sheet-grab" aria-hidden="true">` (hidden unless sheet). An unmeasured host (width 0, hidden window, stub DOM) stays a normal dialog. The grab bar is decorative (no swipe gesture in W1a; see the decisions). Callers do not change.

- [ ] **Step 2.1: IMPACT**

```
cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-w1a-kit && node C:/Users/Andi/.gitnexus/stores/AuraGo-08496d38b623/checkouts/AuraGo-08496d38b623/run.cjs impact "modal" --direction upstream --repo C:/Users/Andi/Documents/repo/AuraGo 2>&1 | grep -E '"(risk|impactedCount|status|message)"'
grep -rn "core.modal(\|\.modal(" ui/js/desktop/apps/easydrag-*.js | wc -l
```
Expected: about 30 callers, risk HIGH or UNKNOWN (many). The change is additive: no signature, no return value and no existing DOM node changes, one extra decorative child. Tell the coordinator the risk level; proceed unless a caller reads `dialog.el.firstChild` or `children[0]`: `grep -rn "firstChild\|firstElementChild\|children\[0\]" ui/js/desktop/apps/easydrag-*.js | grep -i "modal\|dialog"` must print nothing relevant.

- [ ] **Step 2.2: run the failing Node test**

```
cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-w1a-kit && node scripts/test-easydrag.mjs 2>&1 | grep -i "FAIL" | head -4
```
Expected: `FAIL kit modal() is a sheet below 600 px and a dialog from 600 px` (got `[false,false,false,false]`) and `FAIL kit modal() has a grab bar`.

- [ ] **Step 2.3: change `modal()`**

In `easydrag-core.js`, add before `let modalCount = 0;` (line 433) the constant:

```js
    // SHEET_BELOW is the window width under which a dialog opens as a bottom sheet (class ed-sheet).
    const SHEET_BELOW = 600;
```
Replace the markup line (449)

```js
        overlay.innerHTML = '<div class="ed-modal ' + esc(opts.className || '') + '" role="dialog" aria-modal="true" aria-labelledby="' + titleId + '">' +
```
with
```js
        overlay.innerHTML = '<div class="ed-modal ' + esc(opts.className || '') + '" role="dialog" aria-modal="true" aria-labelledby="' + titleId + '"><span class="ed-sheet-grab" aria-hidden="true"></span>' +
```
After `const done = new Promise(...)` and the `dialog` object, i.e. right after the line `const dismiss = () => { if (dismissible) dialog.close(cancelId); };` (472) add:

```js
        // The sheet follows the window: below SHEET_BELOW px the dialog is a bottom sheet. An unmeasured host
        // (width 0) keeps the dialog layout.
        const syncSheet = () => { const width = host.clientWidth; dialog.el.classList.toggle('ed-sheet', width > 0 && width < SHEET_BELOW); };
        syncSheet();
        const watch = typeof ResizeObserver === 'function' ? new ResizeObserver(syncSheet) : null;
        if (watch) watch.observe(host);
```
and in `close(result)` (line 461-468), after `closed = true;` add `if (watch) watch.disconnect();`. Because `watch` is declared after `dialog` but `close` runs later, it is initialised by then; to be safe against a `close()` before that line the declaration order above (after `dialog`) is fine, since `close` can only be called after `modal()` returns or from a listener registered below.

- [ ] **Step 2.4: the CSS**

Insert after `.ed-app .ed-modal-foot > .ed-btn > span { overflow-wrap: anywhere; }` (~1189):

```css
/* bottom sheet (easydrag-core.js modal() below 600 px of window width): full width at the bottom, a grab bar on top */
.ed-sheet-grab { display: none; }
.ed-modal-backdrop:has(> .ed-modal.ed-sheet) { place-items: end stretch; padding: 0; }
.ed-app .ed-modal.ed-sheet {
    width: 100%; max-width: none; max-height: 92%; border-radius: var(--ed-radius-xl) var(--ed-radius-xl) 0 0; border-bottom: 0;
    transform: translateY(var(--ed-space-5)); transition: transform var(--ed-motion-base) var(--ed-ease-out);
}
.ed-app .ed-modal-backdrop.is-open .ed-modal.ed-sheet { transform: none; }
.ed-modal.ed-sheet > .ed-sheet-grab { display: block; flex: none; width: 36px; height: 4px; margin: var(--ed-space-2) auto 0; border-radius: var(--ed-radius-pill); background: var(--ed-border-strong); }
```
(`--ed-radius-xl` is the 12px radius of the approved mock; `--ed-radius-lg` is 14px and is not used here.)

- [ ] **Step 2.5: the Node test passes**

```
cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-w1a-kit && node scripts/test-easydrag.mjs 2>&1 | grep -i "FAIL\|all EasyDrag"
```
Expected: only `all EasyDrag checks passed`.

- [ ] **Step 2.6: the browser check at 390 px (and the disclosure keyboard)**

Create `ui/desktop_easydrag_kit_browser_test.go`:

```go
package ui

import (
	"testing"

	"github.com/go-rod/rod/lib/input"
)

// TestEasyDragKitBrowser: below 600 px of window width a dialog opens as a bottom sheet (full width, at the
// bottom, a grab bar), from 600 px it stays a dialog, and a kit disclosure opens with Enter and closes with Space.
func TestEasyDragKitBrowser(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	s := startEasyDragSmokeWith(t, easyDragSmokeOptions{Width: 390, Height: 844})
	p := s.page
	s.wait(`()=>!!document.querySelector('.ed-app')`)
	open := `()=>{const root=document.querySelector('.ed-app');window.__kitDialog=EasyDrag.core.modal(root,{title:'Sheet',closeLabel:'Close',body:'<p>Body</p>',actions:[{id:'ok',label:'OK',primary:true}]});return true}`
	p.MustEval(open)
	s.wait(`()=>!!document.querySelector('.ed-modal.ed-sheet') && getComputedStyle(document.querySelector('.ed-modal.ed-sheet')).transform==='none'`)
	got := p.MustEval(`()=>{const m=document.querySelector('.ed-modal.ed-sheet'),a=document.querySelector('.ed-app').getBoundingClientRect(),r=m.getBoundingClientRect(),g=m.querySelector('.ed-sheet-grab').getBoundingClientRect(),cs=getComputedStyle(m);
		return JSON.stringify({full:Math.abs(r.width-a.width)<1.5,bottom:Math.abs(r.bottom-a.bottom)<1.5,grab:g.width>0&&g.height>0,topRound:parseFloat(cs.borderTopLeftRadius)>0,bottomSquare:parseFloat(cs.borderBottomLeftRadius)===0})}`).Str()
	if got != `{"full":true,"bottom":true,"grab":true,"topRound":true,"bottomSquare":true}` {
		t.Errorf("390 px: the dialog is not a bottom sheet: %s", got)
	}
	s.shot("kit-sheet-390")

	// the kit disclosure: Enter opens, Space closes (a real button)
	p.MustEval(`()=>{const d=EasyDrag.kit.disclosure({title:'Notes',count:2,body:'<p>More</p>'});document.querySelector('.ed-modal-body').appendChild(d);d.querySelector('.ed-disclosure-head').focus();return true}`)
	p.Keyboard.MustType(input.Enter)
	s.wait(`()=>document.querySelector('.ed-disclosure-head').getAttribute('aria-expanded')==='true' && !document.querySelector('.ed-disclosure-body').hidden`)
	p.Keyboard.MustType(input.Space)
	s.wait(`()=>document.querySelector('.ed-disclosure-head').getAttribute('aria-expanded')==='false' && document.querySelector('.ed-disclosure-body').hidden`)
	p.MustEval(`()=>window.__kitDialog.close('ok')`)
	s.wait(`()=>!document.querySelector('.ed-modal')`)

	// from 600 px up the same call stays a dialog
	p.MustSetViewport(1280, 800, 1, false)
	p.MustEval(open)
	s.wait(`()=>!!document.querySelector('.ed-modal')`)
	if p.MustEval(`()=>document.querySelector('.ed-modal').classList.contains('ed-sheet')`).Bool() {
		t.Error("1280 px: the dialog opened as a sheet")
	}
	s.failOnPageErrors("kit browser")
}
```
Run it (sandbox off):
```
cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-w1a-kit && AURAGO_RUN_BROWSER_SMOKE=1 AURAGO_BROWSER_ARTIFACT_DIR=D:/tmp/easydrag-w1a-shots GOTOOLCHAIN=go1.27.4 GOTMPDIR='D:\tmp\go-build' go test ./ui -run 'TestEasyDragKitBrowser' -count=1 -v 2>&1 | tail -6
```
Expected: `--- PASS: TestEasyDragKitBrowser`. If it fails with the dialog not being a sheet at 390 px, check `p.MustEval("()=>document.querySelector('.ed-app').clientWidth")`: it must be below 600 (the window follows the viewport in this fixture; if the fixture keeps a fixed window width, open the dialog through `EasyDragApp` at a narrower window instead and say so in the report). Look at `D:/tmp/easydrag-w1a-shots/kit-sheet-390.png`.

- [ ] **Step 2.7: the full gated run (existing dialogs at 390 px change shape)**

```
cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-w1a-kit && AURAGO_RUN_BROWSER_SMOKE=1 AURAGO_BROWSER_ARTIFACT_DIR=D:/tmp/easydrag-w1a-shots GOTOOLCHAIN=go1.27.4 GOTMPDIR='D:\tmp\go-build' go test ./ui -run '(?i)easydrag' -count=1 -timeout 30m -v 2>&1 | tee D:/tmp/easydrag-w1a-kit-b2.txt | grep -- "^--- FAIL"; grep -c -- "^--- PASS" D:/tmp/easydrag-w1a-kit-b2.txt; grep -c -- "^--- SKIP" D:/tmp/easydrag-w1a-kit-b2.txt
```
Expected: no FAIL; PASS = baseline + 1 (the new test), SKIP as in the baseline. A test that opens a dialog at 390 px and measures it (target sizes, overlap, footer position) may now fail because the dialog is a sheet. Fix only a failure whose cause is the sheet geometry (full width, bottom aligned, top radius, the grab bar); for each, update the expectation in that test and name it in the commit body. A failure of another cause is not yours: report it.

- [ ] **Step 2.8: gates and commit**

```
cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-w1a-kit && node scripts/test-easydrag.mjs 2>&1 | tail -1 && node scripts/test-easydrag-render-baseline.mjs 2>&1 | tail -1 && gofmt -l ui/desktop_easydrag_kit_browser_test.go && GOTOOLCHAIN=go1.27.4 GOTMPDIR='D:\tmp\go-build' go vet ./ui 2>&1 | tail -2
```
Expected: `all EasyDrag checks passed`, `PASS render baseline (unchanged)`, no gofmt output, no vet output.

Write `C:/disposable/easydrag-w1a/msg-b2.txt`:
```
feat(easydrag): dialogs open as a bottom sheet below 600px

modal() toggles ed-sheet on the dialog when its host is narrower than 600px
(and follows resizes through ResizeObserver): full width, at the bottom, rounded
on top, with a grab bar. Every dialog carries the hidden grab bar; callers do not
change. TestEasyDragKitBrowser checks the sheet at 390px, the dialog at 1280px
and the Enter/Space keys on the kit disclosure.

GitNexus detect-changes unavailable in the worktree; judged by symbols and tests.

Co-Authored-By: Claude Sonnet 5.5 <noreply@anthropic.com>
```
```
cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-w1a-kit && git add ui/js/desktop/apps/easydrag-core.js ui/css/desktop-app-easydrag.css ui/desktop_easydrag_kit_browser_test.go && git commit -F C:/disposable/easydrag-w1a/msg-b2.txt
```
(`git add` also every browser test file you changed in step 2.7.)

---

### Task B.3: the toast migration

**Files:**
- Modify: `ui/js/desktop/apps/easydrag-history.js:25-31, 49-62, 67-74, 82-98, 101`
- Modify: `scripts/test-easydrag-ux-batch5.mjs:511-515`
- Modify: `ui/desktop_easydrag_ux_browser_test.go:88-91`

**Decision (see the end).** The 81 `notify` calls do **not** change. `ctx.notify` shows the desktop-wide `vd-toast`, which the desktop shell owns and which has no action slot; EasyDrag cannot restyle it without editing the desktop. `ED.kit.toast` therefore has two paths: an in-app toast when it gets a `host` (the undo toast), and a thin wrapper of `notify` when it does not (title and message only). Only the one in-app toast moves.

- [ ] **Step 3.1: IMPACT**

```
cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-w1a-kit && node C:/Users/Andi/.gitnexus/stores/AuraGo-08496d38b623/checkouts/AuraGo-08496d38b623/run.cjs impact "create" --direction upstream --repo C:/Users/Andi/Documents/repo/AuraGo 2>&1 | grep -E '"(risk|impactedCount|status|message)"'; grep -rn "history.create\|ED.history\|hideToast" ui/js/desktop/apps/*.js | cut -c1-140
```
Expected: `create` is ambiguous (many modules define one). The only caller of `ED.history.create` is `easydrag-editor.js:117`; `hideToast` is used only inside `easydrag-history.js`. The returned object keeps `hideToast`. No signature changes.

- [ ] **Step 3.2: update the tests first (they fail on the old hook)**

`scripts/test-easydrag-ux-batch5.mjs` line 515: change `toast.querySelector('[data-ed-toast-undo]').fire('click');` to `toast.querySelector('[data-ed-toast-action]').fire('click');`.
`ui/desktop_easydrag_ux_browser_test.go` line 90: change ``page.MustElement(`[data-ed-toast-undo]`).MustClick()`` to ``page.MustElement(`[data-ed-toast-action]`).MustClick()``.

```
cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-w1a-kit && node scripts/test-easydrag.mjs 2>&1 | grep -i "FAIL" | head -3
```
Expected: a `FAIL b5 U12 …` (the action hook does not exist yet).

- [ ] **Step 3.3: move the toast onto the kit**

In `easydrag-history.js` replace lines 25-31:

```js
        const toast = core.el('<div class="ed-toast" data-ed-toast hidden><span class="ed-toast-text" data-ed-toast-text></span>' +
            '<button type="button" class="ed-btn ed-btn--small" data-ed-toast-undo>' + core.icon('undo') + '<span>' + esc(t('easydrag.ui.undo')) + '</span></button>' +
            '<button type="button" class="ed-icon-btn" data-ed-toast-close aria-label="' + esc(t('easydrag.ui.close')) + '" title="' + esc(t('easydrag.ui.close')) + '">' + core.icon('x') + '</button></div>');
        toast.hidden = true;
        opts.toastHost.appendChild(toast);
        let toastVersion = -1;
        let toastTimer = 0;
```
with
```js
        // toastVersion is the model version right after the deletion: the toast's Undo acts only on that one.
        let toastVersion = -1;
        const toast = ED.kit.toast({
            status: 'ok', host: opts.toastHost, start: false, duration: TOAST_MS, closeLabel: t('easydrag.ui.close'),
            action: { label: t('easydrag.ui.undo'), icon: 'undo', onClick: () => {
                const current = toastVersion !== -1 && ed.model.version === toastVersion;
                toastVersion = -1;
                if (current) opts.undo();
            } },
            onHide: ({ hadFocus }) => { if (hadFocus) opts.focusCanvas(); }
        });
```
Delete `arm()` and `hideToast()` (lines 49-62) and replace them by one line:

```js
        function hideToast() { toast.hide(); toastVersion = -1; }
```
In `showDeleted` (67-74) replace the three lines that set the text, `toastVersion`, `toast.hidden = false` and `arm()`:

```js
            toast.querySelector('[data-ed-toast-text]').textContent = core.stepsText(t, 'easydrag.ui.announce_deleted_one', 'easydrag.ui.announce_deleted_many', names);
            toastVersion = ed.model.version;
            toast.hidden = false;
            arm();
```
with
```js
            toastVersion = ed.model.version;
            toast.show(core.stepsText(t, 'easydrag.ui.announce_deleted_one', 'easydrag.ui.announce_deleted_many', names));
```
Delete the five `toast` listeners (lines 82-92: the `click`, `focusin`, `focusout`, `pointerenter`, `pointerleave` listeners; the kit owns them). In the `model` handler (95) replace `if (!toast.hidden && ed.model.version !== toastVersion) hideToast();` with `if (toast.isShown() && ed.model.version !== toastVersion) hideToast();`. Replace the dispose line (98) `bag.add(() => { clearTimeout(toastTimer); toast.remove(); });` with `bag.add(() => toast.dispose());`. Line 101 stays (`return { sync, hideToast, dispose() { bag.dispose(); } };`).

Then update the module header comment line 2 ("the toast that offers to take a deletion back") by appending ` (ED.kit.toast)`; keep every other comment. `esc` is still used (header buttons), `core.icon` too.

- [ ] **Step 3.4: tests**

```
cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-w1a-kit && node scripts/test-easydrag.mjs 2>&1 | grep -i "FAIL\|all EasyDrag" && node scripts/test-easydrag-render-baseline.mjs 2>&1 | tail -1
AURAGO_RUN_BROWSER_SMOKE=1 AURAGO_BROWSER_ARTIFACT_DIR=D:/tmp/easydrag-w1a-shots GOTOOLCHAIN=go1.27.4 GOTMPDIR='D:\tmp\go-build' go test ./ui -run 'TestEasyDragUX' -count=1 -v 2>&1 | grep -- "^--- \|^ok\|FAIL" | head
```
Expected: `all EasyDrag checks passed`, `PASS render baseline (unchanged)`, and the UX browser test(s) `--- PASS`. If the second command's `-run` finds no test, list the test that contains `ux-batch5-toast` with `grep -ln "ux-batch5-toast" ui/*_test.go` and run it by its name. Look at `D:/tmp/easydrag-w1a-shots/ux-batch5-toast.png`: the toast shows the green disc, the text and the Undo action as a text button.

- [ ] **Step 3.5: commit**

Write `C:/disposable/easydrag-w1a/msg-b3.txt`:
```
refactor(easydrag): the undo toast uses ED.kit.toast

easydrag-history.js builds its deletion toast through ED.kit.toast (status disc,
one Undo action, hold on pointer and focus, close button) instead of its own
markup and timers. The test hook data-ed-toast-undo becomes data-ed-toast-action.
The 81 notify calls stay: ctx.notify is the desktop's own toast, which takes no
action and is not EasyDrag's to restyle.

GitNexus detect-changes unavailable in the worktree; judged by symbols and tests.

Co-Authored-By: Claude Sonnet 5.5 <noreply@anthropic.com>
```
```
cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-w1a-kit && git add ui/js/desktop/apps/easydrag-history.js scripts/test-easydrag-ux-batch5.mjs ui/desktop_easydrag_ux_browser_test.go && git commit -F C:/disposable/easydrag-w1a/msg-b3.txt
```

---

### Task B.4: the disclosure migration

**Files (Modify):** `ui/js/desktop/apps/easydrag-authoring.js:132`, `-publish.js:183`, `-scenarios.js:21,24,61,64,74`, `-testing.js:170,227,255,270,272,283,284,305`, `-transform.js:187-190`, `-detail.js:54,77`, `-child-history.js:158,191`, `-block-fields.js:20,33,230`, `-groups.js:106`, `-home.js:109,122`; `ui/css/desktop-app-easydrag.css` (summary rules); tests: `scripts/test-easydrag-blocks.mjs:36`, `-durable.mjs:67,75-86`, `-extra2.mjs:411`; `ui/desktop_easydrag_blocks_browser_test.go:202-209`, `-durable_browser_test.go:20,96,102-111`, `-batches_browser_test.go:77`, `-metadata_browser_test.go:49`, `-ux_browser_test.go:116,126,173,180,200`.
**Create:** `ui/desktop_easydrag_details_test.go`.

**Line limits.** `home.js` (889) and `detail.js` (878) are in the existing-lines-only group: every replacement below stays on its single line. `testing.js` (336), `transform.js`, `block-fields.js` and the rest are far below 900.

- [ ] **Step 4.1: re-run the survey grep and IMPACT**

```
cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-w1a-kit && grep -n "<details\|<summary\|</details\|</summary" ui/js/desktop/apps/easydrag*.js | cut -c1-110
for s in jsonTree createResults; do node C:/Users/Andi/.gitnexus/stores/AuraGo-08496d38b623/checkouts/AuraGo-08496d38b623/run.cjs impact "$s" --direction upstream --repo C:/Users/Andi/Documents/repo/AuraGo 2>&1 | grep -E '"(risk|impactedCount|message)"'; done
```
Expected: exactly the 16 sites of the list above (authoring 132; block-fields 20, 33, 230; child-history 158, 191; detail 54 (a comment), 77; groups 106; home 109, 122; publish 183; scenarios 21; testing 170, 255; transform 187, 189, 190), and a possible extra hit on a line Part A changed. **Every hit that is not in this list is a surface the survey missed: migrate it the same way** (`ED.kit.disclosureMarkup` for a string template, `ED.kit.disclosure` when the code needs the element). `jsonTree` and `createResults` are LOW; none changes a signature or return type (still a string and an element). Each file defines `const ED = window.EasyDrag` at the top; confirm with `grep -L "const ED = window.EasyDrag" ui/js/desktop/apps/easydrag-{authoring,publish,scenarios,testing,transform,detail,child-history,block-fields,groups,home}.js`, which must print nothing.

Each site below is an exact replacement of the quoted fragment. Apply each with the Edit tool (`old_string` must be unique; each fragment is). Line numbers are those of main before Part A; after Part A re-locate with the grep above.

**Site 1, `authoring.js:132`** (publish preview diff)
Old:
```
(result.diff ? '<details class="ed-details"><summary>' + esc(t('easydrag.ui.publish_changes')) + '</summary><pre class="ed-code ed-output-json">' + esc(JSON.stringify(result.diff.details || [], null, 2)) + '</pre></details>' : '')
```
New:
```
(result.diff ? ED.kit.disclosureMarkup({ title: t('easydrag.ui.publish_changes'), body: '<pre class="ed-code ed-output-json">' + esc(JSON.stringify(result.diff.details || [], null, 2)) + '</pre>' }) : '')
```

**Site 2, `publish.js:183`** (warnings)
Old:
```
body += '<details class="ed-details"><summary>' + esc(t('easydrag.ui.publish_warnings', { count: warnings.length })) + '</summary>' + issuesMarkup(warnings) + '</details>';
```
New:
```
body += ED.kit.disclosureMarkup({ title: t('easydrag.ui.publish_warnings', { count: warnings.length }), body: issuesMarkup(warnings) });
```

**Site 3, `scenarios.js:21, 24, 61, 64, 74`** (the sample JSON; the old element is kept, so the element form is used)
Line 21 old: `const advanced = core.el('<details class="ed-details"><summary>' + esc(t('easydrag.ui.sample_json')) + '</summary></details>');` New: `const advanced = ED.kit.disclosure({ title: t('easydrag.ui.sample_json') });`
Line 24 old: `advanced.appendChild(originalLabel);` New: `ED.kit.body(advanced).appendChild(originalLabel);`
Lines 61, 64 and 74: each `advanced.open = true;` becomes `ED.kit.setOpen(advanced, true);` (line 61: `... host.innerHTML = ''; advanced.open = true; return; }`, line 64: `if (!keys.length) advanced.open = true;`, line 74: `... } catch (_) { advanced.open = true; }`).

**Site 4, `testing.js:170`** (simulation fixtures). Old fragment (inside the long `panel` string, starting right after `…simulation_hint')) + '</p>`):
```
<details class="ed-details" data-ed-fixtures-advanced><summary>' + esc(t('easydrag.ui.simulation_fixtures')) + '</summary><label class="ed-label">' + esc(t('easydrag.ui.simulation_fixtures')) + '<textarea class="ed-input ed-code" spellcheck="false" rows="7" data-ed-simulation-fixtures>{}</textarea></label><p class="ed-hint">' + esc(t('easydrag.ui.simulation_fixture_hint')) + '</p><ul class="ed-simulation-nodes">' + ed.model.doc.nodes.filter(n => !(ed.model.info(n.type) || {}).trigger).map(n => '<li><strong>' + esc(n.label || n.type) + '</strong> <code>' + esc(n.id) + '</code></li>').join('') + '</ul></details></section>');
```
New:
```
' + ED.kit.disclosureMarkup({ attrs: { 'data-ed-fixtures-advanced': '' }, title: t('easydrag.ui.simulation_fixtures'), body: '<label class="ed-label">' + esc(t('easydrag.ui.simulation_fixtures')) + '<textarea class="ed-input ed-code" spellcheck="false" rows="7" data-ed-simulation-fixtures>{}</textarea></label><p class="ed-hint">' + esc(t('easydrag.ui.simulation_fixture_hint')) + '</p><ul class="ed-simulation-nodes">' + ed.model.doc.nodes.filter(n => !(ed.model.info(n.type) || {}).trigger).map(n => '<li><strong>' + esc(n.label || n.type) + '</strong> <code>' + esc(n.id) + '</code></li>').join('') + '</ul>' }) + '</section>');
```
(The old fragment starts with `<details` after an open quote, the new one closes that quote first with `' + `; check the line still reads `…simulation_hint')) + '</p>' + ED.kit.disclosureMarkup(…`: the old text before the fragment ends in `'</p>`, so the unchanged prefix plus the new first characters `' + ED.kit…` gives `'</p>' + ED.kit…`.)
Line 227 old: `panel.querySelector('[data-ed-fixtures-advanced]').open = true;` New: `ED.kit.setOpen(panel.querySelector('[data-ed-fixtures-advanced]'), true);`

**Site 5, `testing.js:255`** (results panel; element form, because the code sets `hidden`, the title and the open state)
Old: `const el = core.el('<details class="ed-run-results" hidden><summary></summary><div class="ed-run-results-body"></div></details>');`
New: `const el = ED.kit.disclosure({ className: 'ed-run-results', body: '<div class="ed-run-results-body"></div>' }); el.hidden = true;`
Line 270 old: `if (!durableTouched && !durablePhaseKnown) el.open = status?.state === 'blocked';` New: `if (!durableTouched && !durablePhaseKnown) ED.kit.setOpen(el, status?.state === 'blocked');`
Line 272 old: `el.querySelector('summary').textContent = t('easydrag.ui.durable_title') + ' · ' + ED.durable.label(t, run);` New: `ED.kit.setTitle(el, t('easydrag.ui.durable_title') + ' · ' + ED.durable.label(t, run));`
Line 283 old: `el.open = simulation || run.status === 'error';` New: `ED.kit.setOpen(el, simulation || run.status === 'error');`
Line 284 old (after Part A the status word is `ED.status.word(...)`; replace only the `el.querySelector('summary').textContent = ` prefix and the closing `;`): `el.querySelector('summary').textContent = (simulation ? t('easydrag.ui.simulation_result') : t('easydrag.ui.run_diagnosis')) + ' · ' + <STATUS-WORD>;` New: `ED.kit.setTitle(el, (simulation ? t('easydrag.ui.simulation_result') : t('easydrag.ui.run_diagnosis')) + ' · ' + <STATUS-WORD>);` where `<STATUS-WORD>` is whatever text is there (today `core.tr(t, 'easydrag.ui.status_' + run.status, run.status)`).
Line 305 old: `if (event.target.closest('summary')) durableTouched = true;` New: `if (event.target.closest('.ed-disclosure-head')) durableTouched = true;`

**Site 6, `transform.js:187, 189, 190`** (the three disclosures of the transform dialog; the sample one gets a hook for the browser test)
Line 187 old:
```
'<details class="ed-details"' + (hasSample ? '' : ' open') + '><summary>' + esc(t('easydrag.ui.transform_sample')) + '</summary><p class="ed-hint">' + esc(t('easydrag.ui.transform_sample_hint')) + '</p><textarea class="ed-input ed-code" rows="4" aria-label="' + esc(t('easydrag.ui.transform_sample')) + '" data-ed-transform-sample></textarea></details>',
```
New:
```
ED.kit.disclosureMarkup({ attrs: { 'data-ed-transform-disclosure': 'sample' }, open: !hasSample, title: t('easydrag.ui.transform_sample'), body: '<p class="ed-hint">' + esc(t('easydrag.ui.transform_sample_hint')) + '</p><textarea class="ed-input ed-code" rows="4" aria-label="' + esc(t('easydrag.ui.transform_sample')) + '" data-ed-transform-sample></textarea>' }),
```
Line 189 old: `'<details class="ed-details"><summary>' + esc(t('easydrag.ui.transform_notes')) + '</summary><p class="ed-hint">' + esc(t('easydrag.ui.transform_limit')) + '</p></details>',`
New: `ED.kit.disclosureMarkup({ title: t('easydrag.ui.transform_notes'), body: '<p class="ed-hint">' + esc(t('easydrag.ui.transform_limit')) + '</p>' }),`
Line 190 old: `'<details class="ed-details"><summary>' + esc(t('easydrag.ui.transform_expression')) + '</summary><pre class="ed-code ed-output-json" data-ed-transform-expression></pre></details><p class="ed-error" role="alert" hidden></p>'`
New: `ED.kit.disclosureMarkup({ title: t('easydrag.ui.transform_expression'), body: '<pre class="ed-code ed-output-json" data-ed-transform-expression></pre>' }) + '<p class="ed-error" role="alert" hidden></p>'`

**Site 7, `detail.js:54, 77`** (the JSON tree; the rule "open the levels above depth 2" stays as `open: d < 2`)
Line 54 comment old: `// jsonTree renders a value as nested <details>. A list or object shows 200 entries and` New: `// jsonTree renders a value as nested disclosures. A list or object shows 200 entries and` (the Go test of step 4.5 also reads comments).
Line 77 old: `return '<details' + (d < 2 ? ' open' : '') + '><summary>' + summary + '</summary><ul>' + body + '</ul></details>';`
New: `return ED.kit.disclosureMarkup({ className: 'ed-json-node', open: d < 2, title: summary, body: '<ul>' + body + '</ul>' });`

**Site 8, `child-history.js:158`** (the call path; the head keeps its own accessible name)
Old:
```
nav += '<details class="ed-child-path"><summary aria-label="' + esc(t('easydrag.ui.child_history_navigation')) + '">' + esc(t('easydrag.ui.child_history_calls')) + '</summary><button type="button" class="ed-btn" data-ed-child-root>' + esc(t('easydrag.ui.child_history_calls')) + '</button>' + chain.map(p => '<button type="button" class="ed-btn" data-ed-child-frame="' + p.invocation_id + '"' + (p === f ? ' aria-current="location"' : '') + '>' + esc(callName(p) + ' · ' + frameName(v, p)) + '</button>').join('') + '</details>';
```
New:
```
nav += ED.kit.disclosureMarkup({ className: 'ed-child-path', headLabel: t('easydrag.ui.child_history_navigation'), title: t('easydrag.ui.child_history_calls'), body: '<button type="button" class="ed-btn" data-ed-child-root>' + esc(t('easydrag.ui.child_history_calls')) + '</button>' + chain.map(p => '<button type="button" class="ed-btn" data-ed-child-frame="' + p.invocation_id + '"' + (p === f ? ' aria-current="location"' : '') + '>' + esc(callName(p) + ' · ' + frameName(v, p)) + '</button>').join('') });
```
**Site 9, `child-history.js:191`** tail. Old fragment: `<details class="ed-child-help"><summary>' + esc(t('easydrag.ui.child_history_values_hint')) + '</summary><p>' + esc(t('easydrag.ui.child_history_time_hint')) + '</p></details>';`
New fragment: `' + ED.kit.disclosureMarkup({ className: 'ed-child-help', title: t('easydrag.ui.child_history_values_hint'), body: '<p>' + esc(t('easydrag.ui.child_history_time_hint')) + '</p>' });` (the text before it ends in `+ body + '`, so the line reads `… + body + ED.kit.disclosureMarkup(…);`: the quote before `<details` is closed by the new first `'`; remove the dangling ` + '` if your edit left one: the final line must end `… + body + ED.kit.disclosureMarkup({ … });`.)

**Site 10, `block-fields.js:20`**
Old fragment: `(options.requireComplete ? '<details class="ed-details"><summary>' + ed.esc(ed.t('easydrag.ui.transform_notes')) + '</summary><p class="ed-hint">' + ed.esc(ed.t('easydrag.ui.block_sample_limit')) + '</p></details>' : '')`
New fragment: `(options.requireComplete ? ED.kit.disclosureMarkup({ title: ed.t('easydrag.ui.transform_notes'), body: '<p class="ed-hint">' + ed.esc(ed.t('easydrag.ui.block_sample_limit')) + '</p>' }) : '')`
The whole expression sits inside `core.el('<div class="ed-block-values">' + (…) + '<div data-ed-block-value-form>…')`, so the new fragment is a drop-in for the old one.
**Site 11, `block-fields.js:33`**
Old: `host.innerHTML = !keys.length ? '' : '<details class="ed-details" open><summary>' + ed.esc(ed.t('easydrag.ui.block_unknown_bindings')) + '</summary><ul>' + keys.map((name, index) => '<li><code>' + ed.esc(name) + '</code><pre class="ed-code ed-output-json">' + ed.esc(preview(data[name])) + '</pre><button type="button" class="ed-btn" data-ed-block-unknown-remove="' + index + '"' + (ed.readonly || ed.runView ? ' disabled' : '') + '>' + ed.esc(ed.t('easydrag.ui.delete')) + '</button></li>').join('') + '</ul></details>';`
New: `host.innerHTML = !keys.length ? '' : ED.kit.disclosureMarkup({ open: true, title: ed.t('easydrag.ui.block_unknown_bindings'), body: '<ul>' + keys.map((name, index) => '<li><code>' + ed.esc(name) + '</code><pre class="ed-code ed-output-json">' + ed.esc(preview(data[name])) + '</pre><button type="button" class="ed-btn" data-ed-block-unknown-remove="' + index + '"' + (ed.readonly || ed.runView ? ' disabled' : '') + '>' + ed.esc(ed.t('easydrag.ui.delete')) + '</button></li>').join('') + '</ul>' });`
**Site 12, `block-fields.js:230`**
Old fragment: `core.el('<details class="ed-details" data-ed-nested-call-help><summary>' + ed.esc(ed.t('easydrag.ui.block_nested_title')) + '</summary><p class="ed-hint">' + ed.esc(ed.t('easydrag.ui.block_nested_hint')) + '</p></details>')`
New fragment: `ED.kit.disclosure({ attrs: { 'data-ed-nested-call-help': '' }, title: ed.t('easydrag.ui.block_nested_title'), body: '<p class="ed-hint">' + ed.esc(ed.t('easydrag.ui.block_nested_hint')) + '</p>' })`

**Site 13, `groups.js:106`**
Old fragment: `<details class="ed-group-help"><summary>' + esc(t('easydrag.ui.group_limit')) + '</summary><p class="ed-hint">' + esc(t('easydrag.ui.group_lock_hint')) + '</p></details>' +`
New fragment: `' + ED.kit.disclosureMarkup({ className: 'ed-group-help', title: t('easydrag.ui.group_limit'), body: '<p class="ed-hint">' + esc(t('easydrag.ui.group_lock_hint')) + '</p>' }) +` (it follows `…data-ed-group-view="lock"></button></div>`; the old closing `</div>` stays inside the preceding string, so the line reads `…</button></div>' + ED.kit.disclosureMarkup({…}) +`.)

**Site 14, `home.js:109`**
Old fragment: `<details class="ed-details ed-library-saved"><summary>' + esc(t('easydrag.ui.library_saved_filters')) + '</summary><div class="ed-library-presets"><select class="ed-input" data-ed-library-filter aria-label="' + esc(t('easydrag.ui.library_saved_filters')) + '"></select><button type="button" class="ed-btn" data-ed-library-save>' + core.icon('plus') + esc(t('easydrag.ui.library_filter_save')) + '</button><button type="button" class="ed-icon-btn" data-ed-library-delete disabled aria-label="' + esc(t('easydrag.ui.library_filter_delete')) + '" title="' + esc(t('easydrag.ui.library_filter_delete')) + '">' + core.icon('trash') + '</button></div></details></div></div>' +`
New fragment: `' + ED.kit.disclosureMarkup({ className: 'ed-library-saved', title: t('easydrag.ui.library_saved_filters'), body: '<div class="ed-library-presets"><select class="ed-input" data-ed-library-filter aria-label="' + esc(t('easydrag.ui.library_saved_filters')) + '"></select><button type="button" class="ed-btn" data-ed-library-save>' + core.icon('plus') + esc(t('easydrag.ui.library_filter_save')) + '</button><button type="button" class="ed-icon-btn" data-ed-library-delete disabled aria-label="' + esc(t('easydrag.ui.library_filter_delete')) + '" title="' + esc(t('easydrag.ui.library_filter_delete')) + '">' + core.icon('trash') + '</button></div>' }) + '</div></div>' +`
**Site 15, `home.js:122`**
Old: `const organization = hasOrganization ? core.el('<details class="ed-details ed-library-organization"><summary>' + esc(t('easydrag.ui.organization_filters')) + '</summary><div class="ed-library-organization-controls"><label class="ed-label">' + esc(t('easydrag.ui.folder')) + '<select class="ed-input" data-ed-folder-filter></select></label><label class="ed-label">' + esc(t('easydrag.ui.tags')) + '<select class="ed-input" data-ed-tag-filter></select></label><button type="button" class="ed-btn" data-ed-folders-manage>' + core.icon('folder') + esc(t('easydrag.ui.folders_manage')) + '</button></div><p class="ed-hint" data-ed-folder-error hidden></p></details>') : null;`
New: `const organization = hasOrganization ? ED.kit.disclosure({ className: 'ed-library-organization', title: t('easydrag.ui.organization_filters'), body: '<div class="ed-library-organization-controls"><label class="ed-label">' + esc(t('easydrag.ui.folder')) + '<select class="ed-input" data-ed-folder-filter></select></label><label class="ed-label">' + esc(t('easydrag.ui.tags')) + '<select class="ed-input" data-ed-tag-filter></select></label><button type="button" class="ed-btn" data-ed-folders-manage>' + core.icon('folder') + esc(t('easydrag.ui.folders_manage')) + '</button></div><p class="ed-hint" data-ed-folder-error hidden></p>' }) : null;`

Sites 1-15 plus the comment (detail 54) cover every hit of the survey (the survey's `testing.js 170 and 255` are sites 4 and 5, `transform.js 187-190` is site 6, `child-history.js 158 and 191` are sites 8 and 9).

- [ ] **Step 4.2: prove that no old form remains, then fix the CSS**

```
cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-w1a-kit && grep -n -i "<details\|</details\|<summary\|</summary\|closest('details')\|querySelector('summary')\|\.open = \|\.open=" ui/js/desktop/apps/easydrag*.js | grep -v "window.open\|\.open(" | cut -c1-120
```
Expected: no line. (`ed.groups.open(` and `window.open(` are function calls and are filtered.) Then edit the CSS:
- Delete `.ed-run-results > summary { … }` (~947) and add below `.ed-run-results-body …`:
```css
.ed-run-results.ed-disclosure { margin-top: 0; }
.ed-run-results > .ed-disclosure-head { padding: 12px 14px; }
.ed-run-results > .ed-disclosure-body { padding: 0; }
```
- Replace `.ed-app .ed-child-path summary { … }` (~1666) by `.ed-app .ed-child-path > .ed-disclosure-head { min-height: 44px; padding: 10px 4px; box-sizing: border-box; color: var(--ed-muted); font-size: 12px; line-height: 20px; }` (B.5 turns the 12px into the token) and `.ed-app .ed-child-help summary { min-height: 44px; cursor: pointer; }` (~1678) by `.ed-app .ed-child-help > .ed-disclosure-head { min-height: 44px; }`. In `.ed-app .ed-child-path .ed-btn { display: block; width: 100%; }` nothing changes.
- Add next to the `.ed-json-tree` rules (~966): `.ed-json-tree .ed-json-node > .ed-disclosure-head { min-height: 24px; padding: 0 var(--ed-space-1); font-family: inherit; font-size: inherit; font-weight: 400; }` and `.ed-json-tree .ed-json-node { margin-top: 0; }` and `.ed-json-tree .ed-json-node > .ed-disclosure-body { padding: 0; }`.
- Add `.ed-library-saved > .ed-disclosure-body, .ed-library-organization > .ed-disclosure-body { padding-left: var(--ed-space-2); }` after `.ed-library-saved { flex: 1 1 100%; }` (~503).
```
grep -n "summary" ui/css/desktop-app-easydrag.css | grep -v "node-summary\|bulk-pause-summary"
```
Expected: no line.

- [ ] **Step 4.3: update the Node tests**

`scripts/test-easydrag-blocks.mjs:36`, replace
```
eq('blocks sample limits remain available in collapsed preview notes', [form.el.querySelector('.ed-details').getAttribute('open'), !!form.el.querySelector('.ed-details .ed-hint')], [null, true]);
```
with
```
eq('blocks sample limits remain available in collapsed preview notes', [h.ED.kit.isOpen(form.el.querySelector('.ed-disclosure')), !!form.el.querySelector('.ed-disclosure .ed-hint')], [false, true]);
```
`scripts/test-easydrag-extra2.mjs:411`: replace `(tree.match(/<details/g) || []).length < 100` with `(tree.match(/class="ed-disclosure /g) || []).length < 100`.
`scripts/test-easydrag-durable.mjs`: line 67 replace `.ed-run-results summary').textContent` with `.ed-run-results .ed-disclosure-title').textContent`. Lines 75-83 become (a click on the head now toggles, so the explicit `open` assignments go):
```js
        const results = f.editor.el.querySelector('.ed-run-results'), summary = results.querySelector('.ed-disclosure-head'), kit = f.h.ED.kit;
        eq('durable ordinary waiting status is initially collapsed', kit.isOpen(results), false);
        summary.fire('click');
        f.state.phase.generation++; f.editor.ed.bus.emit('refresh-durable-run'); await settle();
        eq('durable a metadata refresh retains the users expanded status', kit.isOpen(results), true);
        summary.fire('click');
        f.state.phase.state = 'running'; f.state.run.status = 'running';
        f.editor.ed.bus.emit('refresh-durable-run'); await settle();
        eq('durable a later phase retains the users collapsed status', kit.isOpen(results), false);
```
Line 86: replace `blocked.editor.el.querySelector('.ed-run-results').open, true` with `blocked.h.ED.kit.isOpen(blocked.editor.el.querySelector('.ed-run-results')), true`.

Run; the scripts that load single modules may now lack the kit:
```
cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-w1a-kit && node scripts/test-easydrag.mjs 2>&1 | grep -i "FAIL\|TypeError\|all EasyDrag" | head -8; for f in transform authoring child-history scenarios samples test-mode groups; do node scripts/test-easydrag-$f.mjs 2>&1 | tail -1 | cut -c1-120; done
```
Expected: `all EasyDrag checks passed` and each single script ends without an error. A `TypeError: Cannot read properties of undefined (reading 'disclosureMarkup')` names a script whose module list lacks the kit: add `'easydrag-kit.js'` right after its `'easydrag-core.js'` entry (`grep -n "easydrag-core.js" scripts/<file>`), run again. Likewise `FAIL` lines in the groups, nested, scenarios or flagship checks name an assertion that read `details`: fix it to the `.ed-disclosure` form shown above (the markup is the only change).

- [ ] **Step 4.4: update the browser tests**

| File:line | Old | New |
|---|---|---|
| `desktop_easydrag_blocks_browser_test.go:202` | `.ed-details');return notes && !notes.open && …` | `.ed-disclosure');return notes && notes.querySelector('.ed-disclosure-head').getAttribute('aria-expanded')==='false' && …` |
| `…blocks…:206, 209` | `` `[data-ed-block-inputs] .ed-details summary` `` | `` `[data-ed-block-inputs] .ed-disclosure-head` `` |
| `…blocks…:207` | `.ed-details').open` | `.ed-disclosure-head').getAttribute('aria-expanded')==='true'` |
| `…durable…:20` | `return p.open && p.textContent…` | `return p.querySelector('.ed-disclosure-head').getAttribute('aria-expanded')==='true' && p.textContent…` |
| `…durable…:96, 103, 109, 111` | `document.querySelector('.ed-run-results').open` (negated at 96 and 111) | `document.querySelector('.ed-run-results > .ed-disclosure-head').getAttribute('aria-expanded')==='true'` (keep the `!` where it was) |
| `…durable…:102, 110` | `` `.ed-run-results summary` `` | `` `.ed-run-results > .ed-disclosure-head` `` |
| `…batches…:77` | `…details=area.closest('details');if(details && !details.open)details.querySelector('summary').click();…` | `…box=area.closest('.ed-disclosure');if(box && box.querySelector('.ed-disclosure-head').getAttribute('aria-expanded')!=='true')box.querySelector('.ed-disclosure-head').click();…` |
| `…metadata…:49` | `` `.ed-library-organization > summary` `` | `` `.ed-library-organization > .ed-disclosure-head` `` |
| `…ux…:116, 126` | `` `.ed-library-saved > summary` `` | `` `.ed-library-saved > .ed-disclosure-head` `` |
| `…ux…:173` | `document.querySelector('.ed-run-results').open &&` | `document.querySelector('.ed-run-results > .ed-disclosure-head').getAttribute('aria-expanded')==='true' &&` |
| `…ux…:180` | `` `.ed-run-results > summary` `` | `` `.ed-run-results > .ed-disclosure-head` `` |
| `…ux…:200` | `` `.ed-modal--transform details:first-of-type > summary` `` | `` `.ed-modal--transform [data-ed-transform-disclosure="sample"] > .ed-disclosure-head` `` |

The durable tests in `.ed-run-results` read the title as `p.textContent.split(title).length===2`: the kit's head holds the title once, so that still holds. Find any other test that reads `.open` on these elements:
```
grep -n "\.open\b\|summary\|ed-details\|details" ui/*easydrag*_browser_test.go ui/desktop_*easydrag*_test.go | grep -v "node-summary\|data-ed-.*summary\|ed-approval-summary\|ed-batch-summary\|Summary \|summary:" | cut -c1-140
```
Expected after your edits: no line that still addresses a `details` or a `summary` element.

- [ ] **Step 4.5: the Go test (no native details)**

Create `ui/desktop_easydrag_details_test.go`:

```go
package ui

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

var easyDragNativeDisclosure = regexp.MustCompile(`(?i)<\s*/?\s*(details|summary)\b`)

// TestEasyDragHasNoNativeDetails: every disclosure in EasyDrag is ED.kit.disclosure (a button head with
// aria-expanded), never a native details/summary element, in any easydrag*.js (comments included).
func TestEasyDragHasNoNativeDetails(t *testing.T) {
	files, err := filepath.Glob("js/desktop/apps/easydrag*.js")
	if err != nil || len(files) < 40 {
		t.Fatalf("found %d easydrag scripts (%v): the glob is wrong", len(files), err)
	}
	for _, file := range files {
		data, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		for i, line := range strings.Split(string(data), "\n") {
			if easyDragNativeDisclosure.MatchString(line) {
				t.Errorf("%s:%d uses a native details/summary element: use ED.kit.disclosure or ED.kit.disclosureMarkup", file, i+1)
			}
		}
	}
}
```
Check that it fails first on a clean copy of main? It would: run it against the main checkout's files is not allowed to edit, so prove the failure by temporarily keeping one site unmigrated is unnecessary; instead run it now and expect PASS, then `git stash`-free proof: `git show main:ui/js/desktop/apps/easydrag-home.js | grep -c "<details"` prints 2 (the old form was there).
```
cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-w1a-kit && git show main:ui/js/desktop/apps/easydrag-home.js | grep -c "<details" && gofmt -l ui/desktop_easydrag_details_test.go && GOTOOLCHAIN=go1.27.4 GOTMPDIR='D:\tmp\go-build' go test ./ui -run 'TestEasyDragHasNoNativeDetails' -count=1 -v 2>&1 | tail -3
```
Expected: `2`, no gofmt output, `--- PASS`.

- [ ] **Step 4.6: gates, the full gated run, commit**

```
cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-w1a-kit && node scripts/test-easydrag.mjs 2>&1 | tail -1 && node scripts/test-easydrag-render-baseline.mjs 2>&1 | tail -1 && AURAGO_RUN_BROWSER_SMOKE=1 AURAGO_BROWSER_ARTIFACT_DIR=D:/tmp/easydrag-w1a-shots GOTOOLCHAIN=go1.27.4 GOTMPDIR='D:\tmp\go-build' go test ./ui -run '(?i)easydrag' -count=1 -timeout 30m -v 2>&1 | tee D:/tmp/easydrag-w1a-kit-b4.txt | grep -- "^--- FAIL"; grep -c -- "^--- PASS" D:/tmp/easydrag-w1a-kit-b4.txt
```
Expected: `all EasyDrag checks passed`, `PASS render baseline (unchanged)`, no FAIL, PASS = baseline + 2 (`TestEasyDragKitBrowser`, `TestEasyDragHasNoNativeDetails`), and `TestEasyDragTargetSizes` still PASS (the heads are at least 32px; the JSON-tree heads are 24px). A failure whose message names a disclosure selector means a test was missed in step 4.4: fix it as above. Look at one screenshot of each: the test panel with its disclosures, the publish dialog, the transform dialog.

Write `C:/disposable/easydrag-w1a/msg-b4.txt`:
```
refactor(easydrag): every details element becomes ED.kit.disclosure

The 15 native details/summary sites of the EasyDrag scripts (authoring, publish,
scenarios, testing x2, transform x3, the detail JSON tree, child-history x2,
block-fields x3, groups, home x2) use the kit disclosure: a button head with
aria-expanded and aria-controls, a chevron, an optional count. The JSON tree
still opens the levels above depth 2. The summary CSS goes; the Node and browser
tests that opened details or read .open address the disclosure head.
TestEasyDragHasNoNativeDetails keeps it that way.

GitNexus detect-changes unavailable in the worktree; judged by symbols and tests.

Co-Authored-By: Claude Sonnet 5.5 <noreply@anthropic.com>
```
`git add` every file changed in this task by name (`git status --short` lists them; stage exactly those), then `git commit -F C:/disposable/easydrag-w1a/msg-b4.txt`.

---

### Task B.5: the type scale (every `font-size` becomes `var(--ed-font-*)`)

**Files:**
- Modify: `ui/css/desktop-app-easydrag.css` (about 110 `font-size` declarations and 4-5 `font:` shorthands)
- Create: `ui/desktop_easydrag_scale_test.go`
- Create (outside the repo): `C:/disposable/easydrag-w1a/css-fontsize-apply.mjs`

**The rule (owner scale 24 / 18 / 16 / 14 / 12 / 11).** 10 and 11px go to label (11px); a label that sits on the canvas goes to meta (12px). 12px goes to meta. 12.5, 13 and 14px go to body (14px). 15 and 16px go to card (16px). 17 and 20px go to head (18px). 24px goes to title. Principle 1 is the reason for 16 and 14: a card name is at least 13.8px and a parameter line at least 12px at the fit zoom of 0.86 (16 x 0.86 = 13.76, 14 x 0.86 = 12.04), so the legibility test thresholds do **not** change and `TestDesktopEasyDragLegibilityBrowser` stays green. A declaration is decided by its use; the departures from the plain value rule:
1. **Card names and canvas titles** are card (16): `.ed-node-label`, `.ed-detail-label`, `.ed-title-input`, `.ed-modal-title`, and the 15px headings `.ed-flow-info h3` and `.ed-child-history h3` (+1px).
2. **Summaries, parameter lines and body text** are body (14): `.ed-node-summary`, error and field-issue text, and everything that was 13 or 12.5px (so the app root, `.ed-app`, becomes 14px). The 14px small headings (`.ed-template-card h3`, `.ed-outline-head h3`, `.ed-publish-change h4`, `.ed-run-diagnosis h3, h4`, `.ed-drawer-head h3`) stay at body (14), no move; they differ from text by weight.
3. **15px `.ed-result-preview p`** is content the user reads, not a heading: body (14, -1px).
4. **16px form fields**, desktop and touch, are card (16): no move, and iOS Safari does not zoom into a focused field (it zooms below 16px). No field goes to 18.

No label on the canvas needs the meta exception today: `.ed-node-label`, `.ed-node-summary`, `.ed-port-label`, `.ed-node-error`, `.ed-zoom-value`, `.ed-port-tip`, `.ed-connect-hint` and `.ed-pill text` are 12px or more. `.ed-quick-group` (10px, the quick-add popover) is mapped to label; if `easyDragS0SmallTextJS` (the existing check "no text under `.ed-canvas` below 12px") reports it, add it to `CANVAS_LABELS` in the script and run the script again.

**Expected result (on main before Part A; the script prints the real numbers).** 114 declarations: title 1, head 3, card 9, body 26, meta 53, label 22. **Moves larger than 1px: four.** `.ed-recipe-intro h2` 20 to 18 (-2); `.ed-home-sort`, `.ed-version-meta` and `.ed-port-tip` 12.5 to 14 (+1.5). Moves of 1px: 13 to 14 (14 declarations, the root among them), 17 to 18 (2), 15 to 16 (2), 15 to 14 (1), 10 to 11 (4). Unchanged: every 16px, 14px, 12px and 11px declaration (about 80). The root text grows from 13 to 14px, so panels become a little taller and wider; that is the layout shift of spec §11.

- [ ] **Step 5.1: the failing Go test**

Create `ui/desktop_easydrag_scale_test.go`:

```go
package ui

import (
	"regexp"
	"strings"
	"testing"
	"unicode"
)

var (
	easyDragCSSComment = regexp.MustCompile(`(?s)/\*.*?\*/`)
	easyDragFontToken  = regexp.MustCompile(`^var\(--ed-font-(title|head|card|body|meta|label)\)$`)
	easyDragVarCall    = regexp.MustCompile(`var\((?:[^()]|\([^()]*\))*\)`)
	easyDragLengthLit  = regexp.MustCompile(`(^|[\s/])\d+(\.\d+)?(px|rem|em|pt|%)`)
)

// easyDragCSSDecl is one declaration of the stylesheet with its selector (or at-rule prelude) and its first line.
type easyDragCSSDecl struct {
	Line                  int
	Selector, Prop, Value string
}

// easyDragCSSDeclarations lists every declaration of css. Comments are blanked (the line numbers stay).
func easyDragCSSDeclarations(css string) []easyDragCSSDecl {
	css = easyDragCSSComment.ReplaceAllStringFunc(css, func(m string) string {
		return strings.Map(func(r rune) rune {
			if r == '\n' {
				return r
			}
			return ' '
		}, m)
	})
	var out []easyDragCSSDecl
	var stack []string
	var cur strings.Builder
	line, curLine, started := 1, 1, false
	flush := func() {
		seg := strings.TrimSpace(cur.String())
		cur.Reset()
		started = false
		if i := strings.Index(seg, ":"); i > 0 && len(stack) > 0 {
			out = append(out, easyDragCSSDecl{curLine, stack[len(stack)-1], strings.TrimSpace(seg[:i]), strings.TrimSpace(seg[i+1:])})
		}
	}
	for _, r := range css {
		switch r {
		case '{':
			stack = append(stack, strings.Join(strings.Fields(cur.String()), " "))
			cur.Reset()
			started = false
		case '}':
			flush()
			if len(stack) > 0 {
				stack = stack[:len(stack)-1]
			}
		case ';':
			flush()
		default:
			if !started && !unicode.IsSpace(r) {
				started, curLine = true, line
			}
			cur.WriteRune(r)
		}
		if r == '\n' {
			line++
		}
	}
	return out
}

// TestEasyDragFontSizesUseTheScale: every font-size is one of the six tokens (title, head, card, body, meta,
// label), and a font shorthand carries no literal size.
func TestEasyDragFontSizesUseTheScale(t *testing.T) {
	css := strings.ReplaceAll(readDesktopAssetText(t, "css/desktop-app-easydrag.css"), "\r\n", "\n")
	sizes := 0
	for _, d := range easyDragCSSDeclarations(css) {
		switch d.Prop {
		case "font-size":
			sizes++
			if !easyDragFontToken.MatchString(d.Value) {
				t.Errorf("line %d: %s { font-size: %s }: want var(--ed-font-title|head|card|body|meta|label)", d.Line, d.Selector, d.Value)
			}
		case "font":
			if lit := easyDragLengthLit.FindString(easyDragVarCall.ReplaceAllString(d.Value, "")); lit != "" {
				t.Errorf("line %d: %s { font: %s } has a literal size: use var(--ed-font-*) (and a unitless line height)", d.Line, d.Selector, d.Value)
			}
		}
	}
	if sizes < 90 {
		t.Fatalf("only %d font-size declarations found: the scan is broken", sizes)
	}
}
```
Run it:
```
cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-w1a-kit && gofmt -l ui/desktop_easydrag_scale_test.go; GOTOOLCHAIN=go1.27.4 GOTMPDIR='D:\tmp\go-build' go test ./ui -run 'TestEasyDragFontSizesUseTheScale' -count=1 2>&1 | grep -c "font-size:\|font:"; GOTOOLCHAIN=go1.27.4 GOTMPDIR='D:\tmp\go-build' go test ./ui -run 'TestEasyDragFontSizesUseTheScale' -count=1 2>&1 | head -3
```
Expected: no gofmt output; the count is about 110-115 (one error line per declaration, Go prints them all); the head shows `line N: .ed-hint { font-size: 12px }: want var(…)`. If it says `the scan is broken`, the stylesheet is not found: `readDesktopAssetText` takes a path under `ui/`.

- [ ] **Step 5.2: write the migration script**

Write `C:/disposable/easydrag-w1a/css-fontsize-apply.mjs` with the Write tool (the script is the table: the mapping rules are its constants):

```js
#!/usr/bin/env node
// W1a B.5: maps every literal font size of desktop-app-easydrag.css to a var(--ed-font-*) token.
//   node css-fontsize-apply.mjs --list    print the table, change nothing
//   node css-fontsize-apply.mjs --write   rewrite the stylesheet and print the table
// Run with the worktree as the current directory. A size without a rule makes it exit 1 (nothing is written).
import fs from 'node:fs';

const FILE = 'ui/css/desktop-app-easydrag.css';
const PX = { title: 24, head: 18, card: 16, body: 14, meta: 12, label: 11 };
// the plain value rule (owner scale 24 / 18 / 16 / 14 / 12 / 11)
const BY_VALUE = { '10': 'label', '11': 'label', '12': 'meta', '12.5': 'body', '13': 'body', '14': 'body', '15': 'card', '16': 'card', '17': 'head', '20': 'head', '24': 'title' };
// 15px that is content the user reads, not a card name: body
const BODY_TEXT_15 = new Set(['.ed-result-preview p']);
// text on the canvas that the value rule would send to label: meta (principle 1, never below 12px)
const CANVAS_LABELS = new Set([]);

function tokenFor(value, selector) {
    if (value === '15' && BODY_TEXT_15.has(selector)) return 'body';
    const token = BY_VALUE[value];
    if (token === 'label' && CANVAS_LABELS.has(selector)) return 'meta';
    return token;
}

const text = fs.readFileSync(FILE, 'utf8');
const crlf = text.includes('\r\n');
const css = text.replace(/\r\n/g, '\n');
const blank = css.replace(/\/\*[\s\S]*?\*\//g, m => m.replace(/[^\n]/g, ' '));
const edits = [], problems = [];
let stack = [], segStart = 0, started = false, cur = '';
const lineAt = pos => css.slice(0, pos).split('\n').length;
function flush(end) {
    const seg = cur;
    cur = ''; const base = segStart; started = false;
    const selector = stack[stack.length - 1] || '';
    let m = /(^|[\s])font-size\s*:\s*([^;}]+)/.exec(seg);
    if (m) {
        const value = m[2].trim();
        const num = /^(\d+(?:\.\d+)?)px$/.exec(value);
        const at = base + m.index + m[0].length - m[2].length;
        if (!num) { if (!/^var\(--ed-font-/.test(value)) problems.push(lineAt(base) + ' ' + selector + ' font-size: ' + value); }
        else {
            const token = tokenFor(num[1], selector);
            if (!token) problems.push(lineAt(base) + ' ' + selector + ' font-size: ' + value + ' has no rule');
            else edits.push({ start: at, end: at + value.length, old: value, token, selector, line: lineAt(base), kind: 'font-size' });
        }
        return;
    }
    m = /(^|[\s])font\s*:\s*([^;}]+)/.exec(seg);
    if (m) {
        const val = m[2];
        const stripped = val.replace(/var\((?:[^()]|\([^()]*\))*\)/g, s => ' '.repeat(s.length));
        const num = /(\d+(?:\.\d+)?)px/.exec(stripped);
        if (!num) return;
        const token = tokenFor(num[1], selector);
        if (!token) { problems.push(lineAt(base) + ' ' + selector + ' font: ' + val.trim() + ' has no rule'); return; }
        const at = base + m.index + m[0].length - val.length + num.index;
        edits.push({ start: at, end: at + num[0].length, old: num[0], token, selector, line: lineAt(base), kind: 'font' });
    }
}
for (let i = 0; i < blank.length; i++) {
    const ch = blank[i];
    if (ch === '{') { stack.push(cur.split(/\s+/).filter(Boolean).join(' ')); cur = ''; started = false; }
    else if (ch === '}') { flush(i); stack.pop(); }
    else if (ch === ';') flush(i);
    else { if (!started && !/\s/.test(ch)) { started = true; segStart = i; } cur += ch; }
}
const rows = edits.map(e => {
    const from = parseFloat(e.old), to = PX[e.token];
    return { line: e.line, selector: e.selector, from, token: e.token, to, move: to - from, kind: e.kind };
});
const per = {};
rows.forEach(r => { per[r.token] = (per[r.token] || 0) + 1; });
rows.forEach(r => console.log(r.line + '\t' + r.from + 'px\t' + r.token + '\t' + (r.move === 0 ? '' : (r.move > 0 ? '+' : '') + r.move) + (Math.abs(r.move) > 1 ? '\tMOVE>1px' : '') + '\t' + r.kind + '\t' + r.selector.slice(0, 90)));
console.log('TOTAL', rows.length, JSON.stringify(per));
console.log('MOVES>1px', rows.filter(r => Math.abs(r.move) > 1).length);
if (problems.length) { console.error('UNMAPPED:\n' + problems.join('\n')); process.exit(1); }
if (process.argv.includes('--write')) {
    let out = css;
    edits.sort((a, b) => b.start - a.start).forEach(e => { out = out.slice(0, e.start) + 'var(--ed-font-' + e.token + ')' + out.slice(e.end); });
    fs.writeFileSync(FILE, crlf ? out.replace(/\n/g, '\r\n') : out);
    console.log('written ' + edits.length + ' edits to ' + FILE);
}
```

- [ ] **Step 5.3: list the table, save it, check the numbers**

```
cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-w1a-kit && node C:/disposable/easydrag-w1a/css-fontsize-apply.mjs --list > D:/tmp/easydrag-w1a-fontsize-table.tsv; echo exit=$?; tail -3 D:/tmp/easydrag-w1a-fontsize-table.tsv; grep -c "MOVE>1px" D:/tmp/easydrag-w1a-fontsize-table.tsv; grep "MOVE>1px" D:/tmp/easydrag-w1a-fontsize-table.tsv | cut -c1-120
```
Expected: `exit=0`, `TOTAL ~114 {…}` close to the figures above (a few fewer after Part A removed the status families), `MOVES>1px 4`, and four `MOVE>1px` rows: `.ed-recipe-intro h2` (20px, head, -2) and `.ed-home-sort`, `.ed-version-meta`, `.ed-port-tip` (12.5px, body, +1.5). If the script prints `UNMAPPED`, a size outside the eleven values exists (Part A may add one): decide it by use with the rule above, add its selector to the matching set, and run again. Record the table and the four moves in your report.

- [ ] **Step 5.4: apply**

```
cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-w1a-kit && node C:/disposable/easydrag-w1a/css-fontsize-apply.mjs --write | tail -3 && git diff --stat ui/css/desktop-app-easydrag.css && grep -c "font-size: [0-9]" ui/css/desktop-app-easydrag.css; grep -n "font: [0-9]" ui/css/desktop-app-easydrag.css | head -3
GOTOOLCHAIN=go1.27.4 GOTMPDIR='D:\tmp\go-build' go test ./ui -run 'TestEasyDragFontSizesUseTheScale' -count=1 2>&1 | tail -2
```
Expected: `written ~114 edits`, the two grep counts print `0` and nothing, `ok`. Check one `font:` rewrite by eye: `grep -n "font: 600 var(--ed-font-label)" ui/css/desktop-app-easydrag.css` shows `font: 600 var(--ed-font-label)/1 var(--ds-font-mono, ui-monospace, monospace);` (valid CSS: `var()` is allowed as the size in the shorthand).

- [ ] **Step 5.5: the legibility test stays as it is**

The thresholds in `ui/desktop_easydrag_legibility_browser_test.go:212-219` (names 13.76, summaries 12.04, small text 10.32 at the 0.86 fit floor) and `READABLE_ZOOM` in `easydrag-canvas.js` do not change: card 16 and body 14 keep exactly those rendered sizes. Run it now:
```
cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-w1a-kit && AURAGO_RUN_BROWSER_SMOKE=1 AURAGO_BROWSER_ARTIFACT_DIR=D:/tmp/easydrag-w1a-shots GOTOOLCHAIN=go1.27.4 GOTMPDIR='D:\tmp\go-build' go test ./ui -run 'TestDesktopEasyDragLegibilityBrowser' -count=1 -v 2>&1 | grep -- "^--- \|^ok\|FAIL"
```
Expected: `--- PASS`. If `grep -rn "func TestDesktopEasyDragLegibilityBrowser" ui/` prints nothing, find the test that holds `s0LegibleAtFit` with `grep -ln "s0LegibleAtFit" ui/*_test.go` and run its test names instead. A failure means a canvas declaration ended below its old size (check `.ed-node-label`, `.ed-node-summary`, `.ed-port-label`, `.ed-node-error` in the table: they must read 16, 14, 12, 12); fix the mapping, never the threshold. `easydrag-canvas.js` is not touched by this task (it must not grow).

- [ ] **Step 5.6: Node gates, then the full gated run**

```
cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-w1a-kit && node scripts/test-easydrag.mjs 2>&1 | tail -1 && node scripts/test-easydrag-render-baseline.mjs 2>&1 | tail -1
AURAGO_RUN_BROWSER_SMOKE=1 AURAGO_BROWSER_ARTIFACT_DIR=D:/tmp/easydrag-w1a-shots GOTOOLCHAIN=go1.27.4 GOTMPDIR='D:\tmp\go-build' go test ./ui -run '(?i)easydrag' -count=1 -timeout 30m -v 2>&1 | tee D:/tmp/easydrag-w1a-kit-b5.txt | grep -- "^--- FAIL"; grep -c -- "^--- PASS" D:/tmp/easydrag-w1a-kit-b5.txt
```
Expected: `all EasyDrag checks passed`; the render baseline reads `PASS render baseline (unchanged)` (it counts render work on a stub DOM, so a font size cannot change it; if it differs, something else changed: do not update it, find the cause); PASS = the B.4 count + 1 (`TestEasyDragFontSizesUseTheScale`) and no FAIL.

Browser tests that fail now fail for a font-size cause or for another. Handle each:
- A layout test that measures a size, a width or a position of text that moved (title-input height, a card caption, a wrapped line, a clipped label): if the failure message names an element in the table of step 5.3 and the measured value moved by the size change, update the expectation in that test and write the test, old value and new value in the commit body. Never loosen a check that is not about the text size.
- `easyDragS0SmallTextJS` ("no canvas text under 12px") fails: the element it names is a label on the canvas; add its selector to `CANVAS_LABELS`, run `--write` is not repeatable on the already-rewritten file, so edit that one declaration to `var(--ed-font-meta)` by hand and add the selector to the script for the record.
- A failure in the legibility test: never lower a threshold; see step 5.5.
- Any failure whose cause is not a font size: not yours; report it with the test name.
Look at screenshots of the editor and the start page (`D:/tmp/easydrag-w1a-shots/`) before and after (take the `before` set from the B.4 run) in one theme and read the diff by eye: headings 14 to 15, node names 16 to 15, summaries 14 to 13.

- [ ] **Step 5.7: gofmt, vet, commit**

```
cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-w1a-kit && gofmt -l ui/desktop_easydrag_scale_test.go && GOTOOLCHAIN=go1.27.4 GOTMPDIR='D:\tmp\go-build' go vet ./ui 2>&1 | tail -2
```
Expected: no output. Write `C:/disposable/easydrag-w1a/msg-b5.txt` (fill the three moves and the changed tests from your table and from step 5.6):
```
feat(easydrag): every font size is one of six tokens

All font-size declarations and the font shorthands of desktop-app-easydrag.css
use var(--ed-font-title|head|card|body|meta|label): 24, 18, 16, 14, 12 and 11px.
Moves of more than 1px: .ed-recipe-intro h2 20 to 18px, and .ed-home-sort,
.ed-version-meta and .ed-port-tip 12.5 to 14px. Card names stay at 16px and
summaries at 14px, so the legibility thresholds (13.76 and 12.04 at the 0.86 fit
floor) are unchanged; the 16px fields stay at 16px (no iOS zoom).
TestEasyDragFontSizesUseTheScale keeps it at six sizes. The render baseline is
unchanged.

GitNexus detect-changes unavailable in the worktree; judged by symbols and tests.

Co-Authored-By: Claude Sonnet 5.5 <noreply@anthropic.com>
```
```
cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-w1a-kit && git add ui/css/desktop-app-easydrag.css ui/desktop_easydrag_scale_test.go && git commit -F C:/disposable/easydrag-w1a/msg-b5.txt
```
(Stage also every browser test file you updated for a font-size cause.) Confirm `wc -l ui/js/desktop/apps/easydrag-canvas.js` still prints 928 or fewer.

---

### Task B.6: the ratchets (radius literals, colour literals)

**Files:**
- Create: `ui/desktop_easydrag_ratchet_test.go`

**Design.** A Go test counts, in `desktop-app-easydrag.css`:
- **radius literals:** declarations of `border-radius` and the four corner longhands whose value contains a non-zero length literal (`4px`, `50%`, a fallback inside `var(--vd-mat-btn-radius, 999px)` included), so a literal fallback is counted until it becomes a `--ed-radius-*` token;
- **colour literals:** occurrences of `#hex`, `rgb(`, `rgba(`, `hsl(`, `hsla(`, `white` and `black` in values.

Both exclude **token definitions**: every declaration whose property starts with `--ed-` (the token block and its light and Cyberpunk bands, which sit in four places, lines 6-140, 143-180, 181-212 and 212+ today). A single marker comment cannot bound them, because the token definitions are spread over four rules; the property name is the robust boundary (a new colour in a `--ed-*` token is a token, by definition, and is reviewed as one). The test requires the numbers to match exactly: a count above the ceiling fails ("a new literal; use a token"), a count below it fails too ("lower the ceiling in the same commit"), so the numbers can only fall.

- [ ] **Step 6.1: write the test with the measured starting numbers**

Create `ui/desktop_easydrag_ratchet_test.go`:

```go
package ui

import (
	"regexp"
	"strings"
	"testing"
)

// The ceilings only fall. A change that lowers a count lowers its ceiling in the same commit; a change that
// raises one is refused (use a --ed-radius-* token or a status/theme colour token instead of a literal).
const (
	easyDragRadiusLiteralCeiling = 68
	easyDragColourLiteralCeiling = 16
)

var (
	easyDragRadiusProp = regexp.MustCompile(`^border(-(top|bottom)-(left|right))?-radius$`)
	easyDragRadiusLit  = regexp.MustCompile(`(^|[^\w.-])(\d*\.)?[1-9]\d*(\.\d+)?(px|%|em|rem)\b|(^|[^\w.-])0\.\d+(px|em|rem)\b`)
	easyDragColourLit  = regexp.MustCompile(`#[0-9a-fA-F]{3,8}\b|\b(?:rgba?|hsla?)\(|\b(?:white|black)\b`)
)

// easyDragRatchetCounts counts the literal radii and the literal colours outside the token definitions
// (every declaration of a --ed-* custom property is a token; the token bands are spread over four rules).
func easyDragRatchetCounts(css string) (radius, colour int) {
	for _, d := range easyDragCSSDeclarations(css) {
		if strings.HasPrefix(d.Prop, "--ed-") {
			continue
		}
		if easyDragRadiusProp.MatchString(d.Prop) && easyDragRadiusLit.MatchString(d.Value) {
			radius++
		}
		colour += len(easyDragColourLit.FindAllString(d.Value, -1))
	}
	return radius, colour
}

func TestEasyDragLiteralRatchets(t *testing.T) {
	css := strings.ReplaceAll(readDesktopAssetText(t, "css/desktop-app-easydrag.css"), "\r\n", "\n")
	radius, colour := easyDragRatchetCounts(css)
	check := func(name string, have, ceiling int, instead string) {
		switch {
		case have > ceiling:
			t.Errorf("%s: %d, ceiling %d: a new literal; use %s", name, have, ceiling, instead)
		case have < ceiling:
			t.Errorf("%s: %d, ceiling %d: lower the ceiling to %d in this commit", name, have, ceiling, have)
		}
	}
	check("literal border-radius declarations", radius, easyDragRadiusLiteralCeiling, "var(--ed-radius-sm|md|lg|pill) (a theme material radius keeps its var(--vd-mat-*-radius, var(--ed-radius-*)) form)")
	check("colour literals outside the --ed-* tokens", colour, easyDragColourLiteralCeiling, "a token (--ed-*) or a colour-mix() of tokens")
	// the counter must see the real stylesheet: guard against a scan that finds nothing
	if radius+colour == 0 {
		t.Fatal("the ratchet scan found no literal at all: easyDragCSSDeclarations is broken")
	}
}

// TestEasyDragRatchetCountsOnSamples pins the counting rules.
func TestEasyDragRatchetCountsOnSamples(t *testing.T) {
	r, c := easyDragRatchetCounts(`.a { border-radius: 8px; color: #fff; } .b { border-radius: var(--ed-radius-md); border-radius: 0; }
.c { --ed-x: #123456; border-top-left-radius: var(--vd-mat-btn-radius, 999px); background: color-mix(in srgb, var(--ed-ok) 12%, white); }
@media (pointer: coarse) { .d { border-radius: 50%; box-shadow: 0 0 0 1px rgba(0,0,0,.3), inset 0 0 black; } }`)
	if r != 3 || c != 4 {
		t.Fatalf("samples: radius %d (want 3: 8px, the 999px fallback, 50%%), colour %d (want 4: #fff, white, rgba(, black)", r, c)
	}
}
```

- [ ] **Step 6.2: run it, set the ceilings to today's counts**

```
cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-w1a-kit && gofmt -l ui/desktop_easydrag_ratchet_test.go; GOTOOLCHAIN=go1.27.4 GOTMPDIR='D:\tmp\go-build' go test ./ui -run 'TestEasyDragRatchetCountsOnSamples|TestEasyDragLiteralRatchets' -count=1 2>&1 | head -6
```
Expected on main (before Part A): the samples test passes and `TestEasyDragLiteralRatchets` passes with 68 and 16. After Part A (it removes the ad-hoc status families, which carry literal radii and `white`) and after B.1 to B.5, the real counts are lower: the output then reads, for example, `literal border-radius declarations: 62, ceiling 68: lower the ceiling to 62 in this commit` and `colour literals …: 14, ceiling 16: lower the ceiling to 14 …`. **At this first run only**, set each constant to the number the message names (the counts are measured, not estimated; the estimate is about 60-68 radius declarations and 10-16 colour literals), run again:
```
GOTOOLCHAIN=go1.27.4 GOTMPDIR='D:\tmp\go-build' go test ./ui -run 'TestEasyDragRatchetCountsOnSamples|TestEasyDragLiteralRatchets' -count=1 -v 2>&1 | grep -- "^--- \|^ok"
```
Expected: both `--- PASS`. If a message says a count is **above** 68 or 16 after Part A, Part A added literals (for example a status ink hex): report it to the coordinator and set the ceiling to the measured number; do not hide the literal.
Prove the ratchet bites: add `.zz { border-radius: 7px; color: #abc; }` at the end of the stylesheet, run the test (expect two failing lines containing `a new literal`), then remove the two lines again (`git diff --stat ui/css` must show no change).

- [ ] **Step 6.3: the whole T0, gates, commit**

```
cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-w1a-kit && GOTOOLCHAIN=go1.27.4 GOTMPDIR='D:\tmp\go-build' go vet ./ui 2>&1 | tail -2; GOTOOLCHAIN=go1.27.4 GOTMPDIR='D:\tmp\go-build' npm run test:easydrag:t0 2>&1 | tail -8
```
Expected: no vet output; T0 all jobs ok (this includes the new Node kit test and the three new Go tests, since they match `scripts/test-easydrag*.mjs` and `(?i)easydrag`). Then the final full gated run:
```
AURAGO_RUN_BROWSER_SMOKE=1 AURAGO_BROWSER_ARTIFACT_DIR=D:/tmp/easydrag-w1a-shots GOTOOLCHAIN=go1.27.4 GOTMPDIR='D:\tmp\go-build' go test ./ui -run '(?i)easydrag' -count=1 -timeout 30m -v 2>&1 | tee D:/tmp/easydrag-w1a-kit-final.txt | grep -- "^--- FAIL"; grep -c -- "^--- PASS" D:/tmp/easydrag-w1a-kit-final.txt; grep -c -- "^--- SKIP" D:/tmp/easydrag-w1a-kit-final.txt
```
Expected: no FAIL; PASS = baseline + 4 new tests (`TestEasyDragKitBrowser`, `TestEasyDragHasNoNativeDetails`, `TestEasyDragFontSizesUseTheScale`, `TestEasyDragLiteralRatchets`; the samples test is a fifth, so +5 if it prints as its own `--- PASS`); SKIP as the baseline.

Write `C:/disposable/easydrag-w1a/msg-b6.txt` (put the measured ceilings in):
```
test(easydrag): ratchets for literal radii and colours

TestEasyDragLiteralRatchets counts literal border-radius declarations (a literal
fallback inside var() included) and colour literals (hex, rgb, rgba, hsl, hsla,
white, black) outside the --ed-* token definitions, and requires the checked-in
ceilings (<RADIUS> and <COLOUR>) to match: a rise is refused, a fall must lower
the ceiling in the same commit.

GitNexus detect-changes unavailable in the worktree; judged by symbols and tests.

Co-Authored-By: Claude Sonnet 5.5 <noreply@anthropic.com>
```
(Replace `<RADIUS>` and `<COLOUR>` with the two numbers.)
```
cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-w1a-kit && git add ui/desktop_easydrag_ratchet_test.go && git commit -F C:/disposable/easydrag-w1a/msg-b6.txt && git log --oneline main..HEAD
```
Expected: six commits (B.1 to B.6).

---

### Coverage: spec item to task

| Spec item | Task |
|---|---|
| §4.3 `disclosure` (chevron, count right, `aria-expanded`/`aria-controls`, Enter/Space, no `<details>`) | B.1 (code, Node test), B.2 (browser keyboard check) |
| §4.3 `callout` (tinted surface with disc, no border, text in normal ink, at most one action); owner decision 6 | B.1 (code, CSS, Node test incl. the border rule) |
| §4.3 `chip` (neutral, glyph) | B.1 |
| §4.3 `toast` (wraps notify, disc, one action; undo toast of `easydrag-history.js:25-84` moves) | B.1, B.3 |
| §4.3 `toolbar` (built, used nowhere new) | B.1 |
| §4.3 `sheet`: `modal()` below 600px, grab bar, callers unchanged | B.2 (Node test for the class, browser check at 390px) |
| §4.4 disclosure migration (every `<details>`; JSON tree keeps depth rule) | B.4 (15 sites + comment) |
| §4.1 type scale, §10 "every font-size is one of six tokens" | B.5 (script, table, Go test) |
| §4.2 radius and colour ratchets | B.6 |
| §4.5 Go: no `<details>`/`<summary>`; font-size rule; two ratchets | B.4, B.5, B.6 |
| §4.5 Node: disclosure toggles, callout disc and no border, `modal()` sheet class | B.1, B.2 |
| §4.5 browser: publish/test/diagnosis screens keep passing | B.4 (tests updated, full gated run) |
| §9 module registration (loader, `easyDragModules`, harness lists) | B.1 |
| §10 "Callouts are tinted surfaces with a disc. A dialog is a bottom sheet below 600px." | B.1, B.2 |
| §11 type scale moves layouts; render baseline and layout tests updated only for font-size causes | B.5 |

### Deviations from the spec

1. **The ~81 `notify` calls are not wrapped to change their look.** `ctx.notify` is the desktop-wide `vd-toast` (`sdk-events-bootstrap.js:495`): title and message, no action, owned by the desktop shell. Spec §4.3 says `toast` "wraps `ed.ctx.notify`". Part B implements both paths in `ED.kit.toast`: with a `host` it builds the in-app toast (used by the undo toast), without one it calls the `notify` function it is given. Restyling every notify toast needs a change in the desktop shell, which is outside W1a.
2. **`callout` accepts `status: 'info'` (the default) in addition to Part A's twelve values.** The twelve values have no neutral "information" glyph (blue is `running` and `waiting`, whose glyphs mean something else), and the owner-approved mock shows a blue "i" disc. The info disc is a small own element (`.ed-callout-disc`, decorative), the other statuses use `ED.status.disc`.
3. **Extra kit API beyond the contract:** `disclosureMarkup`, `attach`, `isOpen`, `setOpen`, `setTitle`, `setCount`, `body`, and the options `className`, `attrs`, `headLabel` of `disclosure`. The 15 sites build string templates; without a markup form each would need a rewrite into DOM code. The contract names (`disclosure`, `callout`, `chip`, `toast`, `toolbar`) are exactly as specified.
4. **The token-block boundary is the property name, not a marker comment.** The token definitions sit in four places (the main rule and three theme bands). The ratchet ignores every `--ed-*` custom-property declaration; a marker pair would have to bracket four disjoint rules.
5. **The radius ratchet counts a literal fallback inside `var(--vd-mat-*-radius, 12px)`.** Today 68 declarations; counting only top-level literals would give 56 and would hide the fallbacks that should become tokens.
6. **The render baseline is expected unchanged**, not updated: it counts render work on a stub DOM, so neither the font sizes nor the disclosure markup can change it. The brief expected it to change; the plan treats a difference as a defect to explain.

### Decisions needed

Resolved by the coordinator and the owner: the type scale is 24/18/16/14/12/11 (legibility thresholds stay; the touch fields stay at 16px); Part A names `--ed-radius-xl` (12px), `--ed-status-ink`, `--ed-status-edge`; D3 (the `ok` disc on the undo toast), D4 (decorative grab bar), D5 (`notify` toasts keep the desktop look) and the callout `info` status are accepted. Open: none.

For the record:

- **D3. The undo toast status.** The deletion toast shows the green `ok` disc ("done"). If the owner prefers no disc on an undo, pass no `status`. Recommendation: keep `ok`, it matches the approved mock.
- **D4. The grab bar is decorative.** No swipe-down gesture is added in W1a. Recommendation: leave it; a gesture belongs to the touch work of W1e.
- **D5. Whether to restyle the desktop toast.** To give the 81 `notify` calls the new look the desktop's `.vd-toast` needs the disc and the tint; that is a desktop-shell change. Recommendation: not in W1a; log it for the Wave 1 sessions.
