# Synth Studio Redesign Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Turn Synth Studio into a spacious mode-based surface with 3D knobs and faders that hides what is not needed.

**Architecture:** Spec: `plans/2026-11-07-synth-studio-redesign-design.md`. Existing `input[type=range]` elements stay the source of truth. A new `synth-studio-controls.js` upgrades them in place (hidden native input + a drawn dial next to it), so every existing handler, `data-*` hook, undo gesture and test keeps working. `synth-studio-workspace.js` becomes a five-mode switch. All new visuals live in one new stylesheet loaded after the old one.

**Tech Stack:** Plain browser JS (IIFE modules on `window`), CSS (tokens, `color-mix`, `:has`), Node `vm` unit tests, Go + rod browser test (`ui/desktop_synth_studio_browser_test.go`).

---

## Rules for every task

- Work in `C:\Users\Andi\Documents\repo\AuraGo`. **Other sessions share this checkout**: stage and commit only the paths named in the task (`git add <paths>`; `git commit -m "…" -- <paths>`). Never `git add -A`.
- Before editing an existing function (`create` in workspace.js, `render`/`refresh` in synth-studio.js, designer `render`), run GitNexus `impact` on it and report the callers (project rule in `CLAUDE.md`). Before each commit run `detect_changes({scope:"all"})`.
- Commit messages end with `Co-Authored-By: Claude Sonnet 5.5 <noreply@anthropic.com>`.
- No external assets (fonts, libs, images from the Internet). Guard: `TestShippedWebAssetsLoadNothingExternal`.
- Scratch scripts go in the scratchpad/disposable folder, not the repo. Use the Write tool for scripts (Bash heredocs mangle backslashes). In Go/JS source use `\x5c` instead of a backslash-`u` sequence.
- Run Go tests with `-timeout` and never kill processes by image name.
- `ui/js/desktop/bundles/main.bundle.js` contains `module-loader.js`; after editing `module-loader.js` run `npm run build:ui` and commit the bundle.

## Deviations from the spec (decided while planning)

1. **Five modes**, not four: Arrange · Sound · Perform · Strudel · Mixer. Performance (chords, scenes, live FX) is its own mode instead of hiding inside Arrange. Export stays in the header (it is already an icon/select row).
2. No header menu collapse: the file actions are already compact icon buttons; the header only gets a visual rework.
3. Collapsed/open state of Sound-Design sections lives per open editor, not in localStorage. Mode and drawer state are persisted (`aurago:synth-studio:view:2`).
4. Double-click reset on a knob clicks the row's existing `data-reset-param` button when there is one.

## File structure

| File | Action | Responsibility |
|---|---|---|
| `ui/js/desktop/apps/synth-studio-controls.js` | create | pure math + range-input upgrade to knob / horizontal fader |
| `ui/css/desktop-app-synth-studio-modern.css` | create | tokens, 3D controls, mode bar, drawer, mode layouts, cards |
| `scripts/test-synth-studio-controls.mjs` | create | node unit test for the math |
| `ui/js/desktop/apps/synth-studio-workspace.js` | rewrite | mode switch, drawer, note-editor height grip |
| `ui/js/desktop/apps/synth-studio.js` | modify | observe controls, pass hooks to workspace |
| `ui/js/desktop/apps/synth-studio-designer.js` | modify | collapsible groups |
| `ui/js/desktop/core/module-loader.js` | modify | register new script + stylesheet |
| `ui/desktop_synth_studio_browser_test.go` | modify | fixture + new/adjusted checks |
| `ui/lang/desktop/*.json` (16) | modify | mode labels |
| `documentation/synth-studio.md`, `plans/2026-11-07-synth-studio-redesign-design.md` | modify | docs / spec sync |

---

### Task 1: Controls math (pure, node-tested)

**Files:**
- Create: `ui/js/desktop/apps/synth-studio-controls.js` (math part only in this task)
- Create: `scripts/test-synth-studio-controls.mjs`
- Modify: `package.json` (script `test:synth-studio`)

- [ ] **Step 1: Write the failing test**

Create `scripts/test-synth-studio-controls.mjs`:

```js
import assert from 'node:assert/strict';
import fs from 'node:fs';
import vm from 'node:vm';

// Knob / fader math. The DOM half of the module is covered by the browser test.
const runtime = { console };
runtime.globalThis = runtime;
runtime.window = runtime;
vm.runInNewContext(fs.readFileSync('ui/js/desktop/apps/synth-studio-controls.js', 'utf8'), runtime, { filename: 'synth-studio-controls.js' });
const C = runtime.SynthStudioControls;
assert.ok(C, 'module exports its namespace');

const unit = { min: 0, max: 1, step: 0.01 };
assert.equal(C.snap(0.4999, unit), 0.5);
assert.equal(C.snap(7, { min: 0, max: 5, step: 1 }), 5);
assert.equal(C.snap(-3, { min: -1, max: 1, step: 0.01 }), -1);
assert.equal(C.snap(0.123, { min: 0, max: 1, step: 0.005 }), 0.125);
assert.equal(C.snap(0.3, { min: 0, max: 1, step: 0 }), 0.3);

assert.equal(C.fraction(0.5, { min: 0, max: 2 }), 0.25);
assert.equal(C.fraction(9, { min: 0, max: 2 }), 1);
assert.equal(C.fraction(1, { min: 1, max: 1 }), 0);

assert.equal(C.angle(0), -135);
assert.equal(C.angle(1), 135);

assert.equal(C.arcPath(0, 0), '');
assert.equal(C.arcPath(0.5, 0.5), '');
assert.equal(C.arcPath(0, 0.5), 'M12.44 43.56A22 22 0 0 1 28.00 6.00');
assert.equal(C.arcPath(0, 1), 'M12.44 43.56A22 22 0 1 1 43.56 43.56');
assert.equal(C.arcPath(0.5, 0), C.arcPath(0, 0.5), 'arc is symmetric in its ends');

assert.equal(C.dragValue(0.5, 100, 200, unit, false), 1);
assert.equal(C.dragValue(0.5, 80, 200, unit, true), 0.6, 'shift drags four times finer');
assert.equal(C.dragValue(0.9, -1000, 200, unit, false), 0);

const kind = (attrs, type = 'range') => C.kindFor({
    type,
    dataset: attrs.dataset || {},
    hasAttribute: name => (attrs.has || []).includes(name),
    matches: () => !!attrs.knob
});
assert.equal(kind({ knob: true }), 'knob');
assert.equal(kind({ has: ['data-track-volume'] }), 'hfader');
assert.equal(kind({ dataset: { ssKind: 'hfader' } }), 'hfader');
assert.equal(kind({ knob: true, dataset: { ssPlain: '' } }), null);
assert.equal(kind({ knob: true }, 'number'), null);
assert.equal(kind({}), null);
console.log('synth-studio controls ok');
```

- [ ] **Step 2: Run it and see it fail**

Run: `node scripts/test-synth-studio-controls.mjs`
Expected: FAIL (`ENOENT … synth-studio-controls.js`).

- [ ] **Step 3: Write the math part of the module**

Create `ui/js/desktop/apps/synth-studio-controls.js`:

```js
(function () {
    'use strict';
    // Knobs and faders for Synth Studio. A range input stays in the DOM as the
    // single source of truth (value, events, keyboard, forms); this module draws
    // a 3D dial next to it and forwards drags/wheel to the input.
    const START = -135, END = 135, CX = 28, CY = 28, R = 22;
    const KNOB_SELECTOR = '[data-param],[data-master-param],[data-designer-param],[data-fx-param]';
    const clamp = (value, low, high) => Math.min(high, Math.max(low, value));
    const decimals = step => (String(step).split('.')[1] || '').length;

    function snap(value, spec) {
        const { min, max, step } = spec;
        if (!(step > 0)) return clamp(value, min, max);
        const n = Math.round((value - min) / step);
        return clamp(Number((min + n * step).toFixed(decimals(step))), min, max);
    }
    function fraction(value, spec) {
        const span = spec.max - spec.min;
        return span > 0 ? clamp((value - spec.min) / span, 0, 1) : 0;
    }
    function angle(f) { return START + (END - START) * f; }
    function polar(deg) {
        const rad = (deg - 90) * Math.PI / 180;
        return [CX + R * Math.cos(rad), CY + R * Math.sin(rad)];
    }
    function arcPath(f0, f1) {
        const a = Math.min(f0, f1), b = Math.max(f0, f1);
        if (b - a < 0.002) return '';
        const [x0, y0] = polar(angle(a)), [x1, y1] = polar(angle(b));
        const large = angle(b) - angle(a) > 180 ? 1 : 0;
        return 'M' + x0.toFixed(2) + ' ' + y0.toFixed(2) + 'A' + R + ' ' + R + ' 0 ' + large + ' 1 ' + x1.toFixed(2) + ' ' + y1.toFixed(2);
    }
    // delta is in pixels, positive = increase. `pixels` is the travel for the full range.
    function dragValue(start, delta, pixels, spec, fine) {
        const span = spec.max - spec.min;
        return snap(start + (delta / (pixels * (fine ? 4 : 1))) * span, spec);
    }
    function kindFor(input) {
        if (!input || input.type !== 'range' || input.dataset.ssPlain !== undefined) return null;
        if (input.dataset.ssKind) return input.dataset.ssKind;
        if (input.hasAttribute('data-track-volume')) return 'hfader';
        return input.matches(KNOB_SELECTOR) ? 'knob' : null;
    }

    window.SynthStudioControls = { snap, fraction, angle, arcPath, dragValue, kindFor };
})();
```

- [ ] **Step 4: Run the test, then add it to the npm script**

Run: `node scripts/test-synth-studio-controls.mjs`
Expected: PASS, prints `synth-studio controls ok`.

In `package.json` change the `test:synth-studio` value to
`"node scripts/test-synth-studio-audio.mjs && node scripts/test-synth-studio-designer.mjs && node scripts/test-synth-studio-controls.mjs"`.

- [ ] **Step 5: Commit**

```bash
git add ui/js/desktop/apps/synth-studio-controls.js scripts/test-synth-studio-controls.mjs package.json
git commit -m "feat(synth-studio): add knob and fader math with unit tests" -m "Co-Authored-By: Claude Sonnet 5.5 <noreply@anthropic.com>" -- ui/js/desktop/apps/synth-studio-controls.js scripts/test-synth-studio-controls.mjs package.json
```

(`package.json` has other uncommitted edits from other sessions: commit it only if `git diff package.json` shows nothing but your one line; otherwise use `git add -p package.json`.)

---

### Task 2: Controls DOM + 3D look + loading

**Files:**
- Modify: `ui/js/desktop/apps/synth-studio-controls.js`
- Create: `ui/css/desktop-app-synth-studio-modern.css`
- Modify: `ui/js/desktop/core/module-loader.js:260-262` (+ rebuilt `ui/js/desktop/bundles/main.bundle.js`)
- Modify: `ui/js/desktop/apps/synth-studio.js` (observe call, `refresh` sync)
- Modify: `ui/desktop_synth_studio_browser_test.go`

Key facts about the app this must respect (verified in `synth-studio.js:236-251`): the root listens for a **bubbling `pointerdown` whose `e.target` is the range input** to start an undo gesture, `window` `pointerup` finishes it (one undo step), `window` `pointercancel` cancels. `parentElement.querySelector('output')` is used to update the number next to the slider, so **the input must stay a child of its label and the dial is inserted right after it**.

- [ ] **Step 1: Add the DOM half to the module**

In `synth-studio-controls.js`, replace the last line (`window.SynthStudioControls = …`) with:

```js
    const spec = input => ({ min: Number(input.min || 0), max: Number(input.max || 100), step: Number(input.step) || 1 });
    const dialOf = input => {
        const next = input.nextElementSibling;
        return next && next.classList.contains('ss-ctl-dial') ? next : null;
    };

    function paint(input, dial) {
        const s = spec(input), f = fraction(Number(input.value), s);
        dial.style.setProperty('--ss-f', f.toFixed(4));
        if (dial.classList.contains('ss-ctl-knob')) {
            dial.style.setProperty('--ss-a', angle(f).toFixed(1) + 'deg');
            const origin = s.min < 0 && s.max > 0 ? fraction(0, s) : 0;
            dial.querySelector('.ss-ctl-arc').setAttribute('d', arcPath(origin, f));
        }
    }

    function bind(input, dial, kind) {
        let drag = null, wheelTimer = 0;
        const horizontal = kind === 'hfader';
        const set = value => {
            const next = snap(value, spec(input));
            if (next === Number(input.value)) return;
            input.value = String(next);
            paint(input, dial);
            input.dispatchEvent(new Event('input', { bubbles: true }));
        };
        const fakeDown = pointerId => input.dispatchEvent(new PointerEvent('pointerdown', { bubbles: true, button: 0, pointerId }));
        dial.addEventListener('pointerdown', event => {
            if (event.button !== 0 || input.disabled) return;
            event.preventDefault();
            try { dial.setPointerCapture(event.pointerId); } catch (_) { /* synthetic pointer ids */ }
            input.focus({ preventScroll: true });
            fakeDown(event.pointerId);
            if (horizontal) {
                const rect = dial.getBoundingClientRect(), s = spec(input);
                const f = clamp((event.clientX - rect.left - 7) / Math.max(1, rect.width - 14), 0, 1);
                set(s.min + f * (s.max - s.min));
            }
            drag = { x: event.clientX, y: event.clientY, value: Number(input.value) };
            dial.classList.add('is-dragging');
        });
        dial.addEventListener('pointermove', event => {
            if (!drag) return;
            const delta = horizontal ? event.clientX - drag.x : drag.y - event.clientY;
            const pixels = horizontal ? Math.max(40, dial.clientWidth - 14) : 160;
            set(dragValue(drag.value, delta, pixels, spec(input), event.shiftKey));
        });
        dial.addEventListener('pointerup', () => {
            if (!drag) return;
            drag = null;
            dial.classList.remove('is-dragging');
            input.dispatchEvent(new Event('change', { bubbles: true }));
        });
        dial.addEventListener('pointercancel', () => {
            drag = null;
            dial.classList.remove('is-dragging');
        });
        // The wheel only acts on the focused control, so scrolling a panel past a
        // knob never changes a sound by accident.
        dial.addEventListener('wheel', event => {
            if (document.activeElement !== input || input.disabled) return;
            event.preventDefault();
            if (!wheelTimer) fakeDown(1);
            const s = spec(input);
            set(Number(input.value) + (event.deltaY < 0 ? 1 : -1) * s.step * (event.shiftKey ? 1 : 5));
            clearTimeout(wheelTimer);
            wheelTimer = setTimeout(() => {
                wheelTimer = 0;
                input.dispatchEvent(new Event('change', { bubbles: true }));
                window.dispatchEvent(new PointerEvent('pointerup'));
            }, 250);
        }, { passive: false });
        dial.addEventListener('dblclick', () => {
            const reset = input.closest('label')?.querySelector('[data-reset-param]');
            if (reset && !reset.disabled) reset.click();
        });
    }

    function upgrade(input) {
        const kind = kindFor(input);
        if (!kind || input.dataset.ssCtl) return null;
        input.dataset.ssCtl = kind;
        input.classList.add('ss-ctl-native');
        const dial = document.createElement('span');
        dial.className = 'ss-ctl-dial ss-ctl-' + kind;
        dial.setAttribute('aria-hidden', 'true');
        dial.innerHTML = kind === 'knob'
            ? '<svg viewBox="0 0 56 56"><path class="ss-ctl-groove" d="' + arcPath(0, 1) + '"/><path class="ss-ctl-arc" d=""/></svg><span class="ss-ctl-cap"></span>'
            : '<span class="ss-ctl-groove-h"></span><span class="ss-ctl-fill"></span><span class="ss-ctl-thumb"></span>';
        input.after(dial);
        paint(input, dial);
        bind(input, dial, kind);
        return dial;
    }
    function upgradeAll(root) {
        root.querySelectorAll('input[type=range]:not([data-ss-ctl])').forEach(upgrade);
    }
    function sync(root) {
        root.querySelectorAll('input[data-ss-ctl]').forEach(input => {
            const dial = dialOf(input);
            if (dial) paint(input, dial);
        });
    }
    // The app re-renders whole panels. One observer upgrades whatever appears.
    function observe(root, signal) {
        upgradeAll(root);
        let queued = false;
        const observer = new MutationObserver(() => {
            if (queued) return;
            queued = true;
            queueMicrotask(() => { queued = false; upgradeAll(root); sync(root); });
        });
        observer.observe(root, { childList: true, subtree: true });
        if (signal && typeof signal.addEventListener === 'function') signal.addEventListener('abort', () => observer.disconnect(), { once: true });
        return observer;
    }

    window.SynthStudioControls = { snap, fraction, angle, arcPath, dragValue, kindFor, upgrade, upgradeAll, sync, observe };
})();
```

(The file already ends with `})();`; make sure it appears once, after the new block.)

- [ ] **Step 2: Verify the math test still passes**

Run: `node scripts/test-synth-studio-controls.mjs`
Expected: PASS (module does not touch `document` at load).

- [ ] **Step 3: Create the modern stylesheet with tokens and controls**

Create `ui/css/desktop-app-synth-studio-modern.css`:

```css
/* Synth Studio modern layer: loaded after desktop-app-synth-studio.css.
   Colours come from the existing --ss-* tokens, so every desktop theme and the
   accent colour keep working. Default look = dark teal with an orange record. */
.ss-app {
    --ss-lift: 0 10px 24px rgb(0 0 0 / .32), 0 2px 4px rgb(0 0 0 / .3);
    --ss-edge: inset 0 1px 0 color-mix(in srgb, var(--ss-text) 16%, transparent), inset 0 -1px 0 rgb(0 0 0 / .35);
    --ss-glass: linear-gradient(180deg, color-mix(in srgb, var(--ss-text) 8%, transparent), color-mix(in srgb, var(--ss-text) 2%, transparent));
    --ss-glow: color-mix(in srgb, var(--ss-accent) 65%, transparent);
    --ss-cap-hi: color-mix(in srgb, var(--ss-text) 42%, var(--ss-panel));
    --ss-cap-lo: color-mix(in srgb, #000 60%, var(--ss-panel));
    --ss-hot: #ff9d4a;
}

/* ---- native range hidden, dial drawn ---- */
.ss-app input[type=range].ss-ctl-native {
    position: absolute; width: 1px; height: 1px; min-width: 0; margin: 0; padding: 0;
    opacity: 0; pointer-events: none; overflow: hidden;
}
.ss-ctl-dial { position: relative; display: inline-block; flex: none; user-select: none; touch-action: none; }
.ss-ctl-native:focus-visible + .ss-ctl-dial { outline: 2px solid var(--ss-accent); outline-offset: 3px; }
.ss-ctl-native:disabled + .ss-ctl-dial { opacity: .4; pointer-events: none; }

.ss-ctl-knob { width: 60px; height: 60px; cursor: ns-resize; border-radius: 50%; }
.ss-ctl-knob svg { position: absolute; inset: 0; width: 100%; height: 100%; overflow: visible; }
.ss-ctl-groove { fill: none; stroke: color-mix(in srgb, #000 50%, var(--ss-panel)); stroke-width: 4; stroke-linecap: round; }
.ss-ctl-arc { fill: none; stroke: var(--ss-accent); stroke-width: 4; stroke-linecap: round; filter: drop-shadow(0 0 3px var(--ss-glow)); }
.ss-ctl-cap {
    position: absolute; inset: 11px; border-radius: 50%;
    background: radial-gradient(circle at 34% 26%, var(--ss-cap-hi), var(--ss-cap-lo) 78%);
    box-shadow: 0 5px 9px rgb(0 0 0 / .5), inset 0 1px 1px color-mix(in srgb, #fff 45%, transparent), inset 0 -3px 5px rgb(0 0 0 / .4);
    transform: rotate(var(--ss-a, -135deg));
}
.ss-ctl-cap::after {
    content: ''; position: absolute; left: 50%; top: 4px; width: 3px; height: 10px; margin-left: -1.5px;
    border-radius: 2px; background: var(--ss-accent); box-shadow: 0 0 6px var(--ss-glow);
}
.ss-ctl-knob.is-dragging .ss-ctl-cap { filter: brightness(1.12); }

.ss-ctl-hfader { width: 100%; min-width: 64px; height: 24px; cursor: ew-resize; }
.ss-ctl-groove-h {
    position: absolute; left: 0; right: 0; top: 50%; height: 6px; margin-top: -3px; border-radius: 4px;
    background: color-mix(in srgb, #000 50%, var(--ss-panel)); box-shadow: inset 0 1px 3px rgb(0 0 0 / .6);
}
.ss-ctl-fill {
    position: absolute; left: 0; top: 50%; height: 6px; margin-top: -3px; border-radius: 4px;
    width: calc(7px + var(--ss-f, 0) * (100% - 14px)); background: var(--ss-accent); box-shadow: 0 0 8px var(--ss-glow);
}
.ss-ctl-thumb {
    position: absolute; top: 50%; width: 14px; height: 22px; margin-top: -11px; border-radius: 5px;
    left: calc(var(--ss-f, 0) * (100% - 14px));
    background: linear-gradient(180deg, var(--ss-cap-hi), var(--ss-cap-lo));
    box-shadow: 0 3px 6px rgb(0 0 0 / .5), inset 0 1px 0 color-mix(in srgb, #fff 40%, transparent);
}

/* ---- parameter cells that hold a knob ---- */
.ss-app :is(.ss-parameter, .ss-designer-field):has(.ss-ctl-knob) {
    display: flex; flex-direction: column; align-items: center; gap: 4px; text-align: center;
    padding: 10px 6px 8px; border-radius: 14px;
    background: var(--ss-glass), var(--ss-card); border: 1px solid var(--ss-card-border);
    box-shadow: var(--ss-edge);
}
.ss-app :is(.ss-parameter, .ss-designer-field):has(.ss-ctl-knob) > span:first-child { order: 0; font-size: 11px; color: var(--ss-muted); }
.ss-app :is(.ss-parameter, .ss-designer-field):has(.ss-ctl-knob) .ss-ctl-dial { order: 1; }
.ss-app :is(.ss-parameter, .ss-designer-field):has(.ss-ctl-knob) output {
    order: 2; font: 600 12px/1 var(--vd-mat-mono-font, monospace); color: var(--ss-accent);
}
.ss-app .ss-parameter:has(.ss-ctl-knob) [data-reset-param] {
    order: 3; min-height: 0; padding: 0 6px; font-size: 10px; opacity: 0; color: var(--ss-muted);
}
.ss-app .ss-parameter:has(.ss-ctl-knob):hover [data-reset-param],
.ss-app .ss-parameter:has(.ss-ctl-knob):focus-within [data-reset-param] { opacity: 1; }
.ss-app .ss-param-grid { grid-template-columns: repeat(auto-fill, minmax(96px, 1fr)); gap: 12px; }
```

- [ ] **Step 4: Register script and stylesheet in the module loader**

In `ui/js/desktop/core/module-loader.js` line 261 change
`styles: appStyles('/css/desktop-app-synth-studio.css'),` to
`styles: appStyles('/css/desktop-app-synth-studio.css', '/css/desktop-app-synth-studio-modern.css'),`
and in the `scripts` array of the same entry (line 262) insert `'/js/desktop/apps/synth-studio-controls.js',` directly before `'/js/desktop/apps/synth-studio-workspace.js'`.

Run: `npm run build:ui` then `npm run check:ui`
Expected: both succeed; `git status --short ui/js/desktop/bundles/` shows the rebuilt bundle(s).

- [ ] **Step 5: Start the observer from the app**

In `ui/js/desktop/apps/synth-studio.js`, directly after the line `const root=host.firstElementChild, find=q=>root.querySelector(q);` add:

```js
        window.SynthStudioControls?.observe(root,signal);
```

Then at the end of `function refresh()` (find it with `grep -n "function refresh" ui/js/desktop/apps/synth-studio.js`) add as the last statement before its closing brace:

```js
            window.SynthStudioControls?.sync(root);
```

- [ ] **Step 6: Extend the browser test fixture**

In `ui/desktop_synth_studio_browser_test.go`:
1. In the `strings.Replace(html, "</head>", …)` call add a second stylesheet: `<link rel="stylesheet" href="/css/desktop-app-synth-studio-modern.css">` right after the existing `<link …synth-studio.css>`.
2. In the `scripts := []string{…}` slice (line 22) insert `"/js/desktop/apps/synth-studio-controls.js"` directly before `"/js/desktop/apps/synth-studio-workspace.js"`.
3. If `github.com/go-rod/rod/lib/proto` is not imported in the file, add it to the import block.

- [ ] **Step 7: Add the knob checks**

Directly after the check named `"clip opens notes and FX rack"` insert:

```go
	check("range sliders are drawn as knobs", `()=>{const i=[...document.querySelectorAll('.ss-inspector [data-param]')];return i.length===8&&i.every(e=>e.classList.contains('ss-ctl-native')&&e.nextElementSibling?.classList.contains('ss-ctl-knob'))}`)
	page.MustElement("[data-work-tab=sound]").MustClick()
	knob := page.MustElement(".ss-inspector [data-param=tone] + .ss-ctl-knob")
	box := knob.MustShape().Box()
	cx, cy := box.X+box.Width/2, box.Y+box.Height/2
	toneBefore := page.MustEval(`()=>synthInstance().project().tracks[0].params.tone`).Num()
	dy := -60.0
	if toneBefore > 0.5 {
		dy = 60.0
	}
	page.Mouse.MustMoveTo(cx, cy)
	page.Mouse.MustDown(proto.InputMouseButtonLeft)
	page.Mouse.MustMoveTo(cx, cy+dy)
	page.Mouse.MustUp(proto.InputMouseButtonLeft)
	check("dragging a knob changes the model", `()=>Math.abs(synthInstance().project().tracks[0].params.tone-`+fmt.Sprint(toneBefore)+`)>.05`)
	check("the knob shows the new value", `()=>{const i=document.querySelector('.ss-inspector [data-param=tone]');return Math.abs(parseFloat(i.nextElementSibling.style.getPropertyValue('--ss-f'))-Number(i.value))<.02}`)
	page.MustElement("[data-action=undo]").MustClick()
	check("a knob drag is one undo step", `()=>Math.abs(synthInstance().project().tracks[0].params.tone-`+fmt.Sprint(toneBefore)+`)<.001`)
	page.MustElement("[data-work-tab=notes]").MustClick()
```

(The existing "parameter gesture commits" check that follows dispatches events on `[data-param=tone]` directly; it keeps working because the native input still exists.)

- [ ] **Step 8: Run the browser test**

Run: `go test ./ui -run TestDesktopSynthStudioBrowser -count=1 -timeout 20m`
Expected: PASS. If a check fails, read the `fixtureErrors` output the helper prints. A hang on a `MustClick` means the target is not visible (not expected in this task).

- [ ] **Step 9: Commit**

```bash
git add ui/js/desktop/apps/synth-studio-controls.js ui/css/desktop-app-synth-studio-modern.css ui/js/desktop/core/module-loader.js ui/js/desktop/bundles ui/js/desktop/apps/synth-studio.js ui/desktop_synth_studio_browser_test.go
git commit -m "feat(synth-studio): draw range sliders as 3D knobs and faders" -m "Co-Authored-By: Claude Sonnet 5.5 <noreply@anthropic.com>" -- ui/js/desktop/apps/synth-studio-controls.js ui/css/desktop-app-synth-studio-modern.css ui/js/desktop/core/module-loader.js ui/js/desktop/bundles ui/js/desktop/apps/synth-studio.js ui/desktop_synth_studio_browser_test.go
```

---

### Task 3: Mode switch replaces the workbench tabs

**Files:**
- Rewrite: `ui/js/desktop/apps/synth-studio-workspace.js`
- Modify: `ui/js/desktop/apps/synth-studio.js` (workspace options, selection hook)
- Modify: `ui/css/desktop-app-synth-studio-modern.css` (append)
- Modify: `ui/desktop_synth_studio_browser_test.go`
- Modify: `ui/lang/desktop/*.json` (Task 6 adds the labels; here the keys fall back to the English key text, so add the English ones now)

Run GitNexus `impact` on `create` (workspace) first. Known callers: `synth-studio.js` (`workspace=window.SynthStudioWorkspace.create(root,{…})`, uses `workspace.panel/performance/strudel/sound/toggleLibrary/refresh/welcome`) and the tour (`workspace.panel(step.panel)` with `notes|sound|performance`). Keep that API.

Behaviour:
- Modes: `notes`(Arrange) · `sound` · `performance`(Perform) · `strudel` · `mix`(Mixer). Mode buttons keep `data-work-tab="<panel>"` so existing tests and the tour keep working; they also get `data-mode`.
- Arrange shows the arrangement; the note editor only when a clip is selected (`setClip(true)`).
- Sound / Perform / Strudel / Mixer hide the arrangement and give their panel the whole area.
- Mixer forces the master panel open (and closes it again when leaving if it opened it).
- The library is a slide-in drawer, closed by default (`view.library`), toggled by `toggleLibrary()`.

- [ ] **Step 1: Replace workspace.js**

Overwrite `ui/js/desktop/apps/synth-studio-workspace.js` with:

```js
(function () {
    'use strict';
    const MODES = ['notes', 'sound', 'performance', 'strudel', 'mix'];
    const MODE_NAME = { notes: 'arrange', sound: 'sound', performance: 'perform', strudel: 'strudel', mix: 'mix' };
    function create(root, options) {
        const { tr, signal } = options, key = 'aurago:synth-studio:view:2';
        const LIMITS = { height: [160, 900] };
        let view = { panel: 'notes', height: 0, library: false };
        try { const saved = JSON.parse(localStorage.getItem(key)); if (saved) view = { ...view, ...saved }; } catch (_) { /* private mode */ }
        const clamp = (value, min, max) => Math.max(min, Math.min(max, Math.round(Number(value) || min)));
        view.height = Math.max(0, Math.min(LIMITS.height[1], Math.round(Number(view.height) || 0)));
        view.library = view.library === true;
        if (!MODES.includes(view.panel)) view.panel = 'notes';
        let hasClip = false, forcedMaster = false;

        const host = root.querySelector('[data-editor-host]'), note = host.querySelector('[data-note-editor]');
        const bar = document.createElement('nav');
        bar.className = 'ss-modes'; bar.setAttribute('role', 'tablist'); bar.setAttribute('aria-label', tr('modes'));
        for (const name of MODES) {
            const button = document.createElement('button');
            button.type = 'button'; button.dataset.workTab = name; button.dataset.mode = MODE_NAME[name];
            button.setAttribute('role', 'tab');
            const mark = document.createElement('span'); mark.className = 'ss-mode-mark'; mark.setAttribute('aria-hidden', 'true');
            const label = document.createElement('span'); label.textContent = tr('mode_' + MODE_NAME[name]);
            button.append(mark, label); bar.append(button);
        }
        root.insertBefore(bar, root.querySelector('.ss-notice'));

        const grip = document.createElement('div'); grip.className = 'ss-workbench-grip'; grip.tabIndex = 0;
        grip.setAttribute('role', 'separator'); grip.setAttribute('aria-orientation', 'horizontal'); grip.setAttribute('aria-label', tr('resize_layout'));
        grip.setAttribute('aria-valuemin', String(LIMITS.height[0])); grip.setAttribute('aria-valuemax', String(LIMITS.height[1]));
        host.insertBefore(grip, note);
        note.dataset.workPanel = 'notes';
        const sound = document.createElement('section'); sound.className = 'ss-sound-workbench'; sound.dataset.workPanel = 'sound';
        sound.append(root.querySelector('[data-inspector]')); host.append(sound);
        const performance = document.createElement('section'); performance.className = 'ss-performance-workbench'; performance.dataset.workPanel = 'performance'; host.append(performance);
        const strudel = document.createElement('section'); strudel.className = 'ss-strudel-workbench'; strudel.dataset.workPanel = 'strudel'; host.append(strudel);

        const save = () => { try { localStorage.setItem(key, JSON.stringify(view)); } catch (_) { /* private mode */ } };
        function workbenchHeight() {
            const available = host.clientHeight || 0;
            const max = Math.max(LIMITS.height[0], available - 170);
            if (view.height > 0) return Math.min(view.height, max);
            return Math.round(Math.min(Math.max(available * .45, LIMITS.height[0]), max));
        }
        function applyLayout() {
            const height = workbenchHeight();
            root.style.setProperty('--ss-workbench-height', height + 'px');
            grip.setAttribute('aria-valuenow', String(height));
        }
        function render() {
            const mode = MODE_NAME[view.panel];
            root.dataset.mode = mode;
            root.classList.toggle('ss-library-open', view.library);
            root.classList.toggle('ss-has-clip', hasClip);
            applyLayout();
            bar.querySelectorAll('button').forEach(b => { const on = b.dataset.workTab === view.panel; b.setAttribute('aria-selected', String(on)); b.tabIndex = on ? 0 : -1; });
            host.querySelectorAll('[data-work-panel]').forEach(p => {
                const shown = p.dataset.workPanel === view.panel && (p.dataset.workPanel !== 'notes' || hasClip);
                p.hidden = !shown; p.setAttribute('role', 'tabpanel');
            });
            grip.hidden = !(view.panel === 'notes' && hasClip);
            const master = root.querySelector('[data-master-panel]');
            if (master) {
                if (view.panel === 'mix') {
                    if (master.hidden) { master.hidden = false; forcedMaster = true; }
                    if (typeof options.onMix === 'function') options.onMix();
                } else if (forcedMaster) { master.hidden = true; forcedMaster = false; }
            }
            root.querySelectorAll('[data-action="library"]').forEach(b => b.setAttribute('aria-expanded', String(view.library)));
        }
        function panel(name) { if (!MODES.includes(name)) return; view.panel = name; render(); save(); }
        bar.addEventListener('click', e => { const b = e.target.closest('[data-work-tab]'); if (b) panel(b.dataset.workTab); }, { signal });
        bar.addEventListener('keydown', e => {
            if (!['ArrowLeft', 'ArrowRight', 'Home', 'End'].includes(e.key)) return;
            e.preventDefault();
            let i = MODES.indexOf(view.panel); if (i < 0) i = 0;
            i = e.key === 'Home' ? 0 : e.key === 'End' ? MODES.length - 1 : (i + (e.key === 'ArrowRight' ? 1 : MODES.length - 1)) % MODES.length;
            panel(MODES[i]); bar.querySelector('[data-work-tab="' + MODES[i] + '"]').focus();
        }, { signal });

        // The note-editor height grip: drag, arrow keys, double click = default.
        let drag = null;
        const setHeight = value => { view.height = clamp(value, LIMITS.height[0], LIMITS.height[1]); render(); };
        grip.addEventListener('pointerdown', e => {
            if (e.button !== 0) return;
            drag = { origin: e.clientY, value: workbenchHeight() };
            grip.setPointerCapture(e.pointerId); grip.classList.add('is-dragging'); e.preventDefault();
        }, { signal });
        grip.addEventListener('pointermove', e => { if (drag) setHeight(drag.value - (e.clientY - drag.origin)); }, { signal });
        grip.addEventListener('pointerup', () => { if (!drag) return; drag = null; grip.classList.remove('is-dragging'); save(); }, { signal });
        grip.addEventListener('pointercancel', () => { if (!drag) return; setHeight(drag.value); drag = null; grip.classList.remove('is-dragging'); }, { signal });
        grip.addEventListener('dblclick', () => { view.height = 0; render(); save(); }, { signal });
        grip.addEventListener('keydown', e => {
            if (!['ArrowUp', 'ArrowDown'].includes(e.key)) return;
            e.preventDefault();
            setHeight(workbenchHeight() + (e.key === 'ArrowUp' ? 1 : -1) * 20 * (e.shiftKey ? 3 : 1)); save();
        }, { signal });

        const welcome = document.createElement('div'); welcome.className = 'ss-welcome';
        const title = document.createElement('strong'); title.textContent = tr('start_loop'); welcome.append(title);
        for (const name of ['electronic', 'chill', 'retro']) { const b = document.createElement('button'); b.dataset.template = name; b.textContent = tr('template_' + name); b.type = 'button'; welcome.append(b); }
        for (const action of ['new', 'open']) { const b = document.createElement('button'); b.dataset.action = action; b.type = 'button'; b.textContent = tr(action); welcome.append(b); }
        if (typeof options.onTour === 'function') { const b = document.createElement('button'); b.dataset.action = 'tour'; b.type = 'button'; b.textContent = tr('tour'); welcome.append(b); }
        root.querySelector('.ss-main').prepend(welcome);
        render();
        if (typeof ResizeObserver === 'function') {
            const observer = new ResizeObserver(() => applyLayout());
            observer.observe(host);
            if (signal && typeof signal.addEventListener === 'function') signal.addEventListener('abort', () => observer.disconnect(), { once: true });
        }
        return {
            panel, performance, sound, strudel, welcome,
            setClip(value) { const next = !!value; if (next !== hasClip) { hasClip = next; render(); } },
            toggleLibrary() { view.library = !view.library; render(); save(); },
            refresh() { welcome.hidden = options.project().tracks.length > 0; welcome.querySelectorAll('[data-template]').forEach(b => b.disabled = options.blocked()); }
        };
    }
    window.SynthStudioWorkspace = { create };
})();
```

- [ ] **Step 2: Wire the app to the new workspace**

In `synth-studio.js`:
1. In the `window.SynthStudioWorkspace.create(root,{…})` call add `onMix:()=>renderMaster(),` after `onTour`.
2. In the `editor=window.SynthStudioEditor.create(…)` options, extend `onSelect` to `onSelect:()=>{workspace?.setClip(!!editor?.selection().clipId);renderInspector();composer?.refresh();stage?.refresh();}` (keep the existing calls, only prepend the `setClip` statement; `workspace`/`editor` are declared with `let`/`const` earlier in the same scope — if `workspace` is not declared before `editor`, the optional chaining covers the first call).
3. After `workspace=window.SynthStudioWorkspace.create(…)` add `workspace.setClip(!!editor.selection().clipId);`.
4. Search for other uses of removed workspace internals: `grep -n "ss-wide\|workTab\|compactLibrary\|panelWidth\|libraryWidth" ui/js/desktop/apps/*.js`. Expected: none outside the old workspace file. Fix any hit.

- [ ] **Step 3: Append the mode layout CSS**

Append to `ui/css/desktop-app-synth-studio-modern.css`:

```css
/* ---- mode bar ---- */
.ss-app .ss-modes {
    display: flex; flex-wrap: wrap; gap: 8px; align-items: center; padding: 10px 16px;
    background: var(--ss-bar); border-bottom: 1px solid var(--ss-line); box-shadow: var(--ss-bar-edge);
}
.ss-app .ss-modes button {
    display: inline-flex; align-items: center; gap: 8px; min-height: 38px; padding: 8px 18px;
    font-size: 14px; font-weight: 600; color: var(--ss-muted); border-radius: 12px;
    border: 1px solid var(--ss-border); background: var(--ss-glass), var(--ss-card);
    box-shadow: var(--ss-edge), 0 2px 5px rgb(0 0 0 / .25);
    transition: color .15s, box-shadow .15s, transform .15s;
}
.ss-app .ss-modes button:hover { color: var(--ss-text); }
.ss-app .ss-modes button[aria-selected=true] {
    color: var(--ss-text); border-color: color-mix(in srgb, var(--ss-accent) 60%, transparent);
    background: linear-gradient(180deg, color-mix(in srgb, var(--ss-accent) 24%, transparent), color-mix(in srgb, var(--ss-accent) 8%, transparent));
    box-shadow: var(--ss-edge), 0 0 14px var(--ss-glow);
}
.ss-mode-mark { width: 8px; height: 8px; border-radius: 50%; background: color-mix(in srgb, var(--ss-muted) 60%, transparent); }
.ss-modes button[aria-selected=true] .ss-mode-mark { background: var(--ss-accent); box-shadow: 0 0 8px var(--ss-glow); }
.ss-app .ss-workbench-tabs { display: none !important; }

/* ---- layout per mode (replaces the old wide/stacked grid) ---- */
.ss-app .ss-workspace { display: block !important; position: relative; }
.ss-app .ss-main { width: 100%; height: 100%; grid-column: auto; }
.ss-app .ss-editor-host { display: flex !important; flex-direction: column; min-height: 0; grid-template: none; }
.ss-app[data-mode=arrange] .ss-arrangement { flex: 1 1 0; min-height: 140px; }
.ss-app[data-mode=arrange] .ss-note-editor { flex: 0 0 var(--ss-workbench-height); max-height: none; }
.ss-app:not([data-mode=arrange]) .ss-arrangement { display: none; }
.ss-app :is(.ss-sound-workbench, .ss-performance-workbench, .ss-strudel-workbench) { flex: 1 1 0; max-height: none; min-height: 0; }
.ss-app[data-mode=mix] .ss-workspace { display: none !important; }
.ss-app[data-mode=mix] .ss-master-panel { flex: 1; overflow: auto; align-content: start; padding: 20px 24px; gap: 20px 28px; }
.ss-app .ss-workbench-grip { flex: 0 0 9px; }

/* ---- library drawer ---- */
.ss-app .ss-splitter-library { display: none !important; }
.ss-app .ss-library {
    position: absolute; left: 0; top: 0; bottom: 0; z-index: 20; display: flex !important;
    width: min(var(--ss-library-width, 280px), 86%); transform: translateX(-102%); visibility: hidden;
    box-shadow: none; background: var(--ss-glass), var(--ss-side);
    transition: transform .22s ease, visibility 0s .22s, box-shadow .22s;
}
.ss-app.ss-library-open .ss-library { transform: none; visibility: visible; box-shadow: var(--ss-lift); transition-delay: 0s; }
@media (prefers-reduced-motion: reduce) { .ss-app .ss-library { transition: none; } }
```

- [ ] **Step 4: English labels for the new keys**

Add to `ui/lang/desktop/en.json` (flat file, next to `synthStudio.workspace_notes`):

```json
  "synthStudio.modes": "Studio modes",
  "synthStudio.mode_arrange": "Arrange",
  "synthStudio.mode_sound": "Sound design",
  "synthStudio.mode_perform": "Perform",
  "synthStudio.mode_strudel": "Strudel",
  "synthStudio.mode_mix": "Mixer",
```

(All 16 locales get translations in Task 6; until then the other files fall back as before.)

- [ ] **Step 5: Adjust the browser test for the drawer and modes**

In `ui/desktop_synth_studio_browser_test.go`:
1. Before the line that clicks `[data-library-id=lead-saw] [data-insert]` (≈ line 151) and before the line clicking `[data-edit-patch]` (≈ line 187) add `page.MustElement("[data-action=library]").MustClick()` when the drawer is closed. To keep it idempotent, add this helper near `check` and call it instead:

```go
	openLibrary := func() {
		t.Helper()
		if !page.MustEval(`()=>document.querySelector('.ss-app').classList.contains('ss-library-open')`).Bool() {
			page.MustEval(`()=>document.querySelector('[data-action=library]').click()`)
		}
		waitForJSBool(t, page, `()=>getComputedStyle(document.querySelector('.ss-library')).visibility==='visible'`)
	}
```
and call `openLibrary()` before those two `MustClick` lines.
2. Delete the block from the comment `// The wide workbench has to follow the window width` (≈ line 309) through the check `"wide workbench releases on a narrow window"` (≈ line 341); it tests the removed wide layout and splitters. Replace it with:

```go
	// Modes: one at a time, the arrangement only in Arrange, the note editor only with a clip.
	page.MustElement("[data-work-tab=notes]").MustClick()
	check("arrange shows the arrangement", `()=>document.querySelector('.ss-app').dataset.mode==='arrange'&&getComputedStyle(document.querySelector('.ss-arrangement')).display!=='none'`)
	page.MustEval(`()=>synthInstance().editor.select('','')`)
	check("note editor hides without a clip", `()=>document.querySelector('.ss-note-editor').hidden`)
	page.MustEval(`()=>{const p=synthInstance().project(),t=p.tracks[0];synthInstance().editor.select(t.id,t.clips[0].id)}`)
	check("note editor opens with a clip", `()=>!document.querySelector('.ss-note-editor').hidden`)
	page.MustElement("[data-work-tab=sound]").MustClick()
	check("sound mode hides the arrangement and fills the area", `()=>{const a=document.querySelector('.ss-arrangement');const p=document.querySelector('.ss-sound-workbench');return getComputedStyle(a).display==='none'&&!p.hidden&&p.getBoundingClientRect().width>600}`)
	page.MustElement("[data-work-tab=mix]").MustClick()
	check("mixer opens the master panel", `()=>!document.querySelector('[data-master-panel]').hidden&&document.querySelectorAll('[data-master-panel] [data-master-param]').length===14`)
	page.MustElement("[data-work-tab=notes]").MustClick()
	check("leaving the mixer closes what it opened", `()=>document.querySelector('[data-master-panel]').hidden`)
	check("mode is remembered", `()=>JSON.parse(localStorage.getItem('aurago:synth-studio:view:2')).panel==='notes'`)
```
3. At the end (≈ lines 378-379) replace the "compact library opens after desktop collapse" lines with:

```go
	page.MustEval(`()=>{const root=document.querySelector('.ss-app');root.classList.remove('ss-library-open');document.querySelector('[data-action=library]').click();}`)
	waitForJSBool(t, page, `()=>getComputedStyle(document.querySelector('.ss-library')).visibility==='visible'`)
	check("library drawer opens", `()=>document.querySelector('.ss-app').classList.contains('ss-library-open')`)
```
4. Any remaining check that reads the old `view:1` key, `.ss-splitter-*` or `.ss-wide` must be deleted or ported to the new behaviour (`grep -n "view:1\|ss-splitter\|ss-wide\|ss-workbench-tabs" ui/desktop_synth_studio_browser_test.go` must return nothing).

- [ ] **Step 6: Run and fix**

Run: `go test ./ui -run TestDesktopSynthStudioBrowser -count=1 -timeout 20m`
Expected: PASS. Typical fixes: a `MustClick` on something now hidden by its mode (switch mode first), a selector still counting on the tab row.

Also run `node scripts/test-synth-studio-registration.mjs` and `npm run test:synth-studio`. Expected: PASS.

- [ ] **Step 7: Screenshot each mode**

Add a temporary (not committed) loop in the test or use the harness to save `page.MustScreenshot(<scratch>/mode-<name>.png)` for the five modes at 1440×900 and 700×900. Look at them: the mode bar, drawer closed, no horizontal scroll, Mixer fills the area. Fix CSS in the modern file only.

- [ ] **Step 8: Commit**

```bash
git add ui/js/desktop/apps/synth-studio-workspace.js ui/js/desktop/apps/synth-studio.js ui/css/desktop-app-synth-studio-modern.css ui/lang/desktop/en.json ui/desktop_synth_studio_browser_test.go
git commit -m "feat(synth-studio): five-mode workspace with library drawer" -m "Co-Authored-By: Claude Sonnet 5.5 <noreply@anthropic.com>" -- ui/js/desktop/apps/synth-studio-workspace.js ui/js/desktop/apps/synth-studio.js ui/css/desktop-app-synth-studio-modern.css ui/lang/desktop/en.json ui/desktop_synth_studio_browser_test.go
```

---

### Task 4: Collapsible sound-design sections

**Files:**
- Modify: `ui/js/desktop/apps/synth-studio-designer.js` (`createEditor`, `render` ≈ line 295-315)
- Modify: `ui/css/desktop-app-synth-studio-modern.css` (append)
- Modify: `scripts/test-synth-studio-designer.mjs` (exports test) and the browser test

Run `impact` on `createEditor`/`render` first.

- [ ] **Step 1: Failing browser check**

In the browser test, after the check `"library pencil reopens the saved synth"` add:

```go
	check("inactive designer sections start collapsed", `()=>{const g=n=>document.querySelector('[data-designer] details[data-group='+n+']');return g('grp_oscillator').open&&g('grp_filter').open&&g('grp_envelope').open&&!!g('grp_fm')&&!!g('grp_lfo')}`)
	page.MustEval(`()=>{const d=document.querySelector('[data-designer] details[data-group=grp_lfo]');window.lfoWasOpen=d.open;d.querySelector('summary').click();}`)
	check("a section toggles and keeps its state", `()=>new Promise(r=>setTimeout(()=>r(document.querySelector('[data-designer] details[data-group=grp_lfo]').open===!lfoWasOpen),50))`)
```

Run: `go test ./ui -run TestDesktopSynthStudioBrowser -count=1 -timeout 20m`
Expected: FAIL at "inactive designer sections start collapsed" (no `details`).

- [ ] **Step 2: Implement**

In `synth-studio-designer.js`, inside `createEditor` right before `function fieldMarkup(field)` add:

```js
        // A section is open when it shapes the sound; neutral ones start closed.
        // What the user opens or closes wins for as long as this editor lives.
        const userOpen = new Map();
        const ACTIVE = {
            grp_sub_noise: p => Number(p.sub) > 0 || Number(p.noise) > 0,
            grp_fm: p => !!p.mod,
            grp_pitch: p => Math.abs(Number(p.pitchEnv) - 1) > 0.001,
            grp_lfo: p => Number(p.lfo && p.lfo.depth) > 0
        };
        function groupOpen(group) {
            if (userOpen.has(group.labelKey)) return userOpen.get(group.labelKey);
            const test = ACTIVE[group.labelKey];
            return test ? test(state.patch) : true;
        }
```

Replace the `const groups = FIELD_GROUPS.map(group => `<section …`).join("");` line in `render()` with:

```js
            const groups = FIELD_GROUPS.map(group => `<details class="ss-designer-group" data-group="${esc(group.labelKey)}"${groupOpen(group) ? " open" : ""}><summary><h4>${esc(tr(group.labelKey))}</h4><span class="ss-group-chevron" aria-hidden="true"></span></summary><div class="ss-param-grid">${group.fields.map(fieldMarkup).join("")}</div></details>`).join("");
```

After the existing `listen(host, "click", …)` handlers add:

```js
        listen(host, "click", event => {
            const summary = event.target.closest("summary");
            if (!summary) return;
            const details = summary.parentElement;
            // The browser toggles `open` after the click handlers ran.
            setTimeout(() => userOpen.set(details.dataset.group, details.open), 0);
        });
```

- [ ] **Step 3: CSS**

Append to the modern stylesheet:

```css
.ss-app .ss-designer-group {
    margin: 0 0 12px; border-radius: 16px; overflow: hidden;
    background: var(--ss-glass), var(--ss-card); border: 1px solid var(--ss-card-border);
    box-shadow: var(--ss-edge), var(--ss-lift);
}
.ss-app .ss-designer-group > summary {
    display: flex; align-items: center; justify-content: space-between; gap: 8px;
    padding: 12px 16px; cursor: pointer; list-style: none;
}
.ss-app .ss-designer-group > summary::-webkit-details-marker { display: none; }
.ss-app .ss-designer-group > summary h4 { margin: 0; }
.ss-group-chevron {
    width: 8px; height: 8px; border-right: 2px solid var(--ss-muted); border-bottom: 2px solid var(--ss-muted);
    transform: rotate(-45deg); transition: transform .18s;
}
.ss-designer-group[open] .ss-group-chevron { transform: rotate(45deg); }
.ss-app .ss-designer-group > .ss-param-grid { padding: 4px 16px 16px; }
.ss-app .ss-designer-group > summary:focus-visible { outline: 2px solid var(--ss-accent); outline-offset: -2px; }
```

- [ ] **Step 4: Run**

Run: `go test ./ui -run TestDesktopSynthStudioBrowser -count=1 -timeout 20m` and `npm run test:synth-studio`
Expected: PASS. If `grp_lfo` is open in the fixture because the patch has LFO depth, pick another closed group in the test (`grp_fm`) rather than weakening the assertion.

- [ ] **Step 5: Commit**

```bash
git add ui/js/desktop/apps/synth-studio-designer.js ui/css/desktop-app-synth-studio-modern.css ui/desktop_synth_studio_browser_test.go
git commit -m "feat(synth-studio): collapse inactive sound-design sections" -m "Co-Authored-By: Claude Sonnet 5.5 <noreply@anthropic.com>" -- ui/js/desktop/apps/synth-studio-designer.js ui/css/desktop-app-synth-studio-modern.css ui/desktop_synth_studio_browser_test.go
```

---

### Task 5: Glass look for header, transport, inspector, mixer strips

**Files:**
- Modify: `ui/css/desktop-app-synth-studio-modern.css` (append)
- Modify: `ui/desktop_synth_studio_browser_test.go` (contrast/visibility smoke)

CSS only; no JS. The goal is the reference look: raised round transport buttons with a glowing Play, orange Record, big BPM readout, glass cards in the inspector, FX slots as cards, roomy spacing.

- [ ] **Step 1: Append the styles**

```css
/* ---- header and transport ---- */
.ss-app .ss-header { padding: 12px 18px; gap: 16px; background: var(--ss-glass), var(--ss-bar); }
.ss-app .ss-brand-mark { box-shadow: 0 0 14px var(--ss-glow), var(--ss-edge); }
.ss-app .ss-transport { gap: 14px; }
.ss-app .ss-transport-buttons { display: flex; gap: 10px; align-items: center; }
.ss-app .ss-transport-buttons button {
    width: 42px; height: 42px; padding: 0; border-radius: 50%; font-size: 15px;
    border: 1px solid var(--ss-border);
    background: radial-gradient(circle at 35% 25%, var(--ss-cap-hi), var(--ss-cap-lo) 80%);
    box-shadow: 0 4px 9px rgb(0 0 0 / .45), inset 0 1px 1px color-mix(in srgb, #fff 40%, transparent);
}
.ss-app .ss-transport-buttons button:active { transform: translateY(1px); box-shadow: 0 1px 3px rgb(0 0 0 / .5), inset 0 2px 4px rgb(0 0 0 / .4); }
.ss-app .ss-transport-buttons .ss-play { width: 52px; height: 52px; font-size: 19px; color: var(--ss-accent); }
.ss-app .ss-transport-buttons .ss-play.is-active { box-shadow: 0 0 18px var(--ss-glow), 0 4px 9px rgb(0 0 0 / .45); }
.ss-app .ss-transport-buttons .ss-record { color: var(--ss-hot); }
.ss-app .ss-transport-buttons .ss-record.is-recording { box-shadow: 0 0 18px color-mix(in srgb, var(--ss-hot) 70%, transparent); }
.ss-app .ss-position {
    font: 700 20px/1 var(--vd-mat-mono-font, monospace); letter-spacing: 1px; padding: 8px 14px; border-radius: 10px;
    background: color-mix(in srgb, #000 40%, var(--ss-panel)); box-shadow: inset 0 2px 5px rgb(0 0 0 / .55); color: var(--ss-accent);
}
.ss-app .ss-tempo input { width: 76px; font: 700 18px/1 var(--vd-mat-mono-font, monospace); text-align: center; }

/* ---- inspector and FX cards ---- */
.ss-app .ss-inspector { padding: 20px 22px; }
.ss-app .ss-sound-workbench .ss-param-grid { grid-template-columns: repeat(auto-fill, minmax(96px, 1fr)); gap: 14px; }
.ss-app .ss-fx-slot {
    margin: 0 0 14px; padding: 12px 14px; border-radius: 16px;
    background: var(--ss-glass), var(--ss-card); border: 1px solid var(--ss-card-border); box-shadow: var(--ss-edge), var(--ss-lift);
}
.ss-app .ss-fx-params { display: grid; grid-template-columns: repeat(auto-fill, minmax(96px, 1fr)); gap: 12px; margin-top: 10px; }
.ss-app .ss-master-panel .ss-parameter:has(.ss-ctl-knob) { min-width: 0; }

/* ---- track mixer fader ---- */
.ss-app .ss-track-head .ss-ctl-hfader { margin-top: 4px; }

/* ---- spacing for the empty states and the welcome card ---- */
.ss-app .ss-welcome { padding: 18px 22px; border-radius: 18px; background: var(--ss-glass), var(--ss-card); box-shadow: var(--ss-edge), var(--ss-lift); }
```

- [ ] **Step 2: Look at it, then iterate**

Screenshot the five modes (see Task 3 Step 7) at 1440×900 and check against the reference image: raised knobs with glowing ring, glass cards, bright Play, orange Record, no clipped labels, nothing overflowing at 700 px width. Fix in the modern CSS only. Also run the all-theme smoke: `go test ./ui -run TestDesktopThemeTokenContrastBrowser -count=1 -timeout 20m` (existing guard; expected PASS) and switch two themes by hand in the harness to confirm the knobs recolour.

- [ ] **Step 3: Commit**

```bash
git add ui/css/desktop-app-synth-studio-modern.css
git commit -m "style(synth-studio): glass header, transport and card look" -m "Co-Authored-By: Claude Sonnet 5.5 <noreply@anthropic.com>" -- ui/css/desktop-app-synth-studio-modern.css
```

---

### Task 6: Translations (16 locales)

**Files:**
- Modify: `ui/lang/desktop/{cs,da,de,el,en,es,fr,hi,it,ja,nl,no,pl,pt,sv,zh}.json`

- [ ] **Step 1: Write the insert script**

Write `<scratchpad>/add-synth-modes.mjs` (Write tool; not in the repo):

```js
import fs from 'node:fs';
const dir = 'C:/Users/Andi/Documents/repo/AuraGo/ui/lang/desktop/';
const T = {
  en: ['Studio modes', 'Arrange', 'Sound design', 'Perform', 'Strudel', 'Mixer'],
  de: ['Studio-Modi', 'Arrangieren', 'Sound-Design', 'Spielen', 'Strudel', 'Mixer'],
  zh: ['工作室模式', '编排', '音色设计', '演奏', 'Strudel', '混音'],
  sv: ['Studioläge', 'Arrangera', 'Ljuddesign', 'Spela', 'Strudel', 'Mixer'],
  pt: ['Modos do estúdio', 'Arranjar', 'Design de som', 'Tocar', 'Strudel', 'Mixer'],
  pl: ['Tryby studia', 'Aranżacja', 'Projekt dźwięku', 'Wykonanie', 'Strudel', 'Mikser'],
  no: ['Studiomoduser', 'Arranger', 'Lyddesign', 'Spill', 'Strudel', 'Mikser'],
  nl: ['Studiomodi', 'Arrangeren', 'Geluidsontwerp', 'Spelen', 'Strudel', 'Mixer'],
  ja: ['スタジオモード', 'アレンジ', 'サウンドデザイン', 'パフォーム', 'Strudel', 'ミキサー'],
  it: ['Modalità studio', 'Arrangia', 'Sound design', 'Suona', 'Strudel', 'Mixer'],
  hi: ['स्टूडियो मोड', 'व्यवस्था', 'साउंड डिज़ाइन', 'प्रदर्शन', 'Strudel', 'मिक्सर'],
  fr: ['Modes du studio', 'Arranger', 'Design sonore', 'Jouer', 'Strudel', 'Mixeur'],
  es: ['Modos del estudio', 'Arreglar', 'Diseño de sonido', 'Tocar', 'Strudel', 'Mezclador'],
  el: ['Λειτουργίες στούντιο', 'Διάταξη', 'Σχεδιασμός ήχου', 'Παίξιμο', 'Strudel', 'Μίξερ'],
  da: ['Studietilstande', 'Arranger', 'Lyddesign', 'Spil', 'Strudel', 'Mixer'],
  cs: ['Režimy studia', 'Aranžmá', 'Zvukový design', 'Hra', 'Strudel', 'Mixážní pult'],
};
const keys = ['modes', 'mode_arrange', 'mode_sound', 'mode_perform', 'mode_strudel', 'mode_mix'];
for (const [lang, values] of Object.entries(T)) {
  const file = dir + lang + '.json';
  const raw = fs.readFileSync(file, 'utf8');
  const data = JSON.parse(raw);
  keys.forEach((k, i) => { data['synthStudio.' + k] = values[i]; });
  const eol = raw.includes('\r\n') ? '\r\n' : '\n';
  fs.writeFileSync(file, JSON.stringify(data, null, 2).replace(/\n/g, eol) + (raw.endsWith('\n') ? eol : ''));
}
```

- [ ] **Step 2: Run it and check the diff is additive**

Run: `node <scratchpad>/add-synth-modes.mjs` then `git diff --stat ui/lang/desktop/`
Expected: 16 files, each with ≈ 6 inserted lines and **0 deleted** lines (if lines were reformatted or deleted, the files use another style: `git checkout` them and insert the six lines textually after `"synthStudio.workspace_notes"` instead). `en.json` already has the keys from Task 3 (the script overwrites them with identical values).

- [ ] **Step 3: Run the locale guards**

Run: `grep -ln "lang/desktop" ui/*_test.go` and run the matching tests with `go test ./ui -run '<TestNames>' -count=1`; also `node scripts/test-ui-regressions.mjs`.
Expected: PASS (all 16 files carry the same keys).

- [ ] **Step 4: Commit**

```bash
git add ui/lang/desktop
git commit -m "i18n(synth-studio): translate the studio mode labels" -m "Co-Authored-By: Claude Sonnet 5.5 <noreply@anthropic.com>" -- ui/lang/desktop
```

---

### Task 7: Docs, spec sync, full verification

**Files:**
- Modify: `documentation/synth-studio.md`
- Modify: `plans/2026-11-07-synth-studio-redesign-design.md`

- [ ] **Step 1: Update the manual**

In `documentation/synth-studio.md` replace the paragraph that describes the workbench tabs/layout with a section "Modes and controls":
- Five modes (Arrange, Sound, Perform, Strudel, Mixer), what each shows, and that the note editor appears when a clip is selected.
- The library drawer (button in the header, closed by default).
- Knobs: drag up/down, Shift = fine, mouse wheel while focused, arrow keys, double-click = reset where a reset exists; faders for track volume.
- Sound-design sections: inactive ones start closed.
- Mode and drawer state are remembered in this browser.
(`grep -n "workbench\|tab" documentation/synth-studio.md` lists the sentences to rewrite; keep the Strudel section as is.)

- [ ] **Step 2: Sync the spec with the deviations**

In the spec replace "four modes" lines with the five modes, drop the header-menu line, and replace the "persisted collapsed sections" sentence per the deviations list at the top of this plan.

- [ ] **Step 3: Full verification**

Run, expecting all green:
- `npm run test:synth-studio`
- `node scripts/test-synth-studio-registration.mjs && node scripts/test-synth-studio-recording.mjs`
- `npm run check:ui`
- `go test ./ui -run 'TestDesktopSynthStudioBrowser|TestShippedWebAssetsLoadNothingExternal|TestDesktopThemeTokenContrastBrowser' -count=1 -timeout 30m`
- `go test ./internal/server -run SynthStudio -count=1`
Then GitNexus `detect_changes({scope:"all"})` and confirm only Synth Studio files changed.

- [ ] **Step 4: Commit**

```bash
git add documentation/synth-studio.md plans/2026-11-07-synth-studio-redesign-design.md
git commit -m "docs(synth-studio): describe modes, knobs and the library drawer" -m "Co-Authored-By: Claude Sonnet 5.5 <noreply@anthropic.com>" -- documentation/synth-studio.md plans/2026-11-07-synth-studio-redesign-design.md
```

---

## Self-review (spec coverage)

- Modes (spec §1): Task 3 (five modes; deviation recorded). Drawers: Task 3. Persisted state: Task 3 (`view:2`).
- Intelligent hiding (§2): note editor conditional (Task 3), sections collapsed (Task 4), arrangement hidden outside Arrange (Task 3), empty states keep the existing messages.
- 3D controls (§3): Tasks 1, 2 (knob, hfader, focus, wheel, keys, reset), glass/tokens Tasks 2, 5. Toggle switches: not part of this plan (checkboxes keep their accent-colour look); add as follow-up if wanted.
- Migration order (§4): controls → shell → sound design → look → i18n/docs.
- Testing (§6): Tasks 1–4 and 7.
- Type/name consistency: `SynthStudioControls.{snap,fraction,angle,arcPath,dragValue,kindFor,upgrade,upgradeAll,sync,observe}`; workspace API `{panel,performance,sound,strudel,welcome,setClip,toggleLibrary,refresh}`; classes `ss-ctl-native`, `ss-ctl-dial`, `ss-ctl-knob`, `ss-ctl-hfader`, `ss-modes`, `ss-library-open`, `ss-has-clip`; storage key `aurago:synth-studio:view:2`.
- Risks to watch while executing: hidden-element `MustClick` hangs in the browser test; Windows `prefers-reduced-motion` flipping browser tests; other sessions committing in the same checkout (stage by path only).
