// HomeDash light widget. Variants: dial (H3 hero, dimmer knob), slider (.hd-lrow), fader (.hd-lcol) and tile (.hd-ltile). brightness_pct (0-100) is the
// truth for brightness (D6); range, keys and commands follow the capability. Every variant offers more-info (hold, Shift+Enter, ContextMenu; the
// visible button is the host's). Unavailable entities read the en dash, never "Off". Commands go through api.command with a phase (P1 rules 6-7).
(function () {
    'use strict';
    const E = window.HomeDashEngine;
    if (!E || !E.core || !E.format || !E.prim || !E.widgets || !E.control || !E.input) throw new Error('hd-w-light.js: load hd-core, hd-format, hd-input, the primitives, hd-widgets and hd-control first');
    const { setAttr, setVars, el } = E.core, F = E.format, C = E.control, MID_DOT = C.MID;
    const POWER = ['toggle', 'turn_on', 'turn_off'], ANY = POWER.concat('brightness');

    // Shared by every variant: the last view, the brightness model, the input contracts and the readout words.
    function controller(api) {
        const st = { cur: null };
        const lm = () => C.lightModel(st.cur), rng = () => C.range(st.cur, 'brightness', { min: 0, max: 100, step: 1 });
        const canDim = () => !!st.cur && st.cur.available !== false && !st.cur.readonly && api.can('primary', 'brightness');
        const send = (v, phase) => { if (canDim()) api.command('primary', 'brightness', { value: v }, { phase }); };
        const hooks = { input: () => {}, commit: () => {}, key: () => {} };   // a variant adds its live readout here
        const dimInput = { range: rng, enabled: () => C.enabled(api, st.cur, ['brightness']), onBlocked: () => C.announceBlocked(api, st.cur, 'brightness'),
            onInput: v => { hooks.input(v); send(v, 'input'); }, onCommit: v => { hooks.commit(v); send(v, 'commit'); }, onKey: v => { hooks.key(v); send(v, 'key'); },
            onToggle: () => C.activateOnOff(api, st.cur), onHold: () => C.hold(api) };
        const powerInput = { enabled: () => C.enabled(api, st.cur, POWER), onBlocked: () => C.announceBlocked(api, st.cur, 'toggle'),
            onActivate: () => C.tap(api, st.cur, () => C.activateOnOff(api, st.cur)), onHold: () => C.hold(api) };
        // One update pass: the model of the entity plus the words every variant shows.
        function model(e) {
            st.cur = e || null;
            const m = C.lightModel(e), onWord = api.t('homedash.state.on'), offWord = api.t('homedash.state.off'), word = m.on ? onWord : offWord;
            return { e, m, onWord, offWord, word, known: m.known, pct: m.pct, line: m.known ? onWord + MID_DOT + m.pct + ' %' : word,
                valueText: m.dead ? F.DASH : m.known ? api.t('homedash.light.valuetext', { value: m.pct, state: onWord }) : word,
                off: !e || (!e.readonly && !POWER.some(c => api.can('primary', c))), dimOff: !e || (!e.readonly && !api.can('primary', 'brightness')) };
        }
        const flags = (x, disabled) => ({ status: x.e ? x.e.status : 'unavailable', pending: !!(x.e && x.e.pending), readonly: !!(x.e && x.e.readonly), disabled });
        const status = (x, cap) => api.setStatus(C.statusText(api, x.e, x.line, { cap }));
        const named = (x, key) => { const n = C.label(api, x.e); return n ? (key ? n + ': ' + api.t(key) : n) : null; };
        const readout = x => (x.m.dead ? { value: null } : x.known ? { value: x.pct } : { text: x.word });
        return { st, lm, rng, canDim, send, hooks, dimInput, powerInput, model, flags, status, named, readout };
    }

    function dial(api) {
        const k = controller(api), inst = api.family === 'instrument', tact = api.family === 'tactile', glass = api.family === 'glass';
        const prim = E.prim.knob({ label: api.widget.title, variant: 'ring', min: 0, max: 100, unit: '%', decimals: 0, ticks: 10, legends: true,
            readout: inst ? 'seg7' : 'numeral', quantize: inst ? 40 : 0, gaps: inst ? 40 : 0, lead: inst, glow: glass || inst,
            dots: tact ? { count: 21, r: 94, size: 4.5, lit: true } : null, hardware: tact, word: true, handleShadow: glass, input: k.dimInput });
        api.body.appendChild(prim.el); api.body.appendChild(C.armMarker());
        setAttr(api.body, 'data-hd-nodata', api.t('homedash.state.no_data'));
        const readout = prim.el.querySelector('.hd-dial-readout');
        let lastPct = null;
        return {
            update(view) {
                const e = view.entities.primary, x = k.model(e), m = x.m;
                // No live data (no entity, unavailable or unknown): an empty readout and the handle at the arc start, never the word "Off".
                // On without a usable brightness (null, 0 or not a number): dial full, word "On", no percentage anywhere.
                api.setTone(e && e.tone ? e.tone : null);
                const r = k.rng(); prim.set(Object.assign({ value: m.pct, min: r.min, max: r.max, word: m.dead || x.known ? null : x.word, valueText: x.valueText }, k.flags(x, x.dimOff)));
                setAttr(prim.el, 'aria-label', k.named(x, 'homedash.control.brightness'));
                if (readout && lastPct !== null && lastPct !== m.pct) setAttr(readout, 'data-hd-tick', readout.getAttribute('data-hd-tick') === 'a' ? 'b' : 'a');
                lastPct = m.pct;
                k.status(x, 'brightness');
            },
            destroy() { prim.destroy(); }
        };
    }

    // slider (.hd-lrow: power chip, slider, numeral) and fader (.hd-lcol: numeral, vertical slider, power chip).
    function line(api, vertical) {
        const k = controller(api), chip = E.prim.pushButton({ variant: 'chip', icon: 'bulb', toggle: true, label: '', input: k.powerInput });
        const slider = E.prim.slider({ variant: vertical ? 'v' : 'h', min: 0, max: 100, unit: '%', decimals: 0, input: k.dimInput }), num = E.prim.numeral({ chars: 4, unit: '%', decimals: 0 });
        const live = v => num.set({ value: v });
        k.hooks.input = k.hooks.commit = k.hooks.key = live;
        const box = el('div', { class: vertical ? 'hd-lcol' : 'hd-lrow' }, vertical ? [num.el, slider.el, chip.el] : [chip.el, slider.el, num.el]);
        api.body.appendChild(box); api.body.appendChild(C.armMarker());
        setAttr(api.body, 'data-hd-nodata', api.t('homedash.state.no_data'));
        return {
            update(view) {
                const e = view.entities.primary, x = k.model(e), m = x.m;
                api.setTone(e && e.tone ? e.tone : null);
                chip.set(Object.assign({ on: m.on, icon: E.widgets.iconFor(api.widget, e) || 'bulb', text: '' }, k.flags(x, x.off)));
                slider.set(Object.assign({ value: m.pct, min: k.rng().min, max: k.rng().max, valueText: x.valueText }, k.flags(x, x.dimOff)));
                num.set(k.readout(x));
                setAttr(chip.el, 'aria-label', k.named(x, 'homedash.control.power')); setAttr(slider.el, 'aria-label', k.named(x, 'homedash.control.brightness'));
                k.status(x, 'brightness');
            },
            destroy() { chip.destroy(); slider.destroy(); num.destroy(); box.remove(); }
        };
    }

    // tile (.hd-ltile): a wash from the left (--hd-value), an icon badge and the numeral. Tap powers, a horizontal drag dims (when the light can),
    // hold or Shift+Enter opens more-info. role=slider (dimmer) when dimmable, else role=switch.
    function tile(api) {
        const k = controller(api), I = E.input, badge = E.prim.iconBadge({ icon: 'bulb' }), num = E.prim.numeral({ chars: 4, unit: '%', decimals: 0 });
        const node = el('div', { class: 'hd-ltile', tabindex: 0 }, [el('span', { class: 'hd-ltile-wash', 'aria-hidden': 'true' }), badge.el, num.el]);
        api.body.appendChild(node); api.body.appendChild(C.armMarker());
        setAttr(api.body, 'data-hd-nodata', api.t('homedash.state.no_data'));
        let shown = null, dragging = false, last = null;
        const paint = pct => { shown = pct; setVars(node, { '--hd-value': (pct === null ? 0 : Math.min(1, Math.max(0, pct / 100))).toFixed(4) }); setAttr(node, 'data-hd-empty', pct === null); };
        const live = v => { paint(v); num.set({ value: v }); setAttr(node, 'aria-valuenow', v); setAttr(node, 'aria-valuetext', api.t('homedash.light.valuetext', { value: v, state: api.t('homedash.state.on') })); };
        const gate = { enabled: () => C.enabled(api, k.st.cur, ANY), onBlocked: () => C.announceBlocked(api, k.st.cur, 'toggle'), onHold: () => C.hold(api) };
        const at = p => I.valueAt(I.fraction(p.x, p.y, p.rect, 'x'), k.rng());
        const offG = I.gesture(node, Object.assign({}, gate, { onTap: () => C.tap(api, k.st.cur, () => C.activateOnOff(api, k.st.cur)),
            onDragStart: () => { dragging = true; },
            onDrag: p => { if (!k.canDim()) return; last = at(p); live(last); k.send(last, 'input'); },
            onDragEnd: (p, s, cancelled) => { dragging = false; if (!k.canDim()) return; const v = last === null ? at(p) : last; last = null; live(v); k.send(v, 'commit'); } }));
        const offK = I.keys(node, Object.assign({}, gate, { onToggle: () => C.activateOnOff(api, k.st.cur), range: k.rng, get: () => shown,
            onKey: v => { if (!k.canDim()) return; live(v); k.send(v, 'key'); } }));
        setAttr(node, 'data-hd-input', 'drag');
        return {
            update(view) {
                const e = view.entities.primary, x = k.model(e), m = x.m, dim = m.dimmable;
                api.setTone(e && e.tone ? e.tone : null);
                E.prim.util.applyState(node, k.flags(x, x.off && x.dimOff), { interactive: true });
                setAttr(node, 'data-hd-on', m.on);
                if (!dragging) paint(m.pct);
                badge.set({ icon: E.widgets.iconFor(api.widget, e) || 'bulb' }); num.set(k.readout(x));
                setAttr(node, 'role', dim ? 'slider' : 'switch'); setAttr(node, 'aria-label', k.named(x, null));
                setAttr(node, 'aria-roledescription', dim ? api.t('homedash.control.dimmer') : null); setAttr(node, 'aria-keyshortcuts', dim ? 'Enter Space' : null);
                setAttr(node, 'aria-checked', dim ? null : String(m.on));
                const r = k.rng();
                setAttr(node, 'aria-valuemin', dim ? r.min : null); setAttr(node, 'aria-valuemax', dim ? r.max : null);
                setAttr(node, 'aria-valuenow', dim && !m.dead ? m.pct : null); setAttr(node, 'aria-valuetext', dim ? x.valueText : null);
                k.status(x, dim ? 'brightness' : 'toggle');
            },
            destroy() { offG(); offK(); badge.destroy(); num.destroy(); node.remove(); }
        };
    }

    E.widgets.register('light', {
        bindings: ['primary'],
        variants: ['slider', 'fader', 'dial', 'tile'],
        create(api) {
            if (api.variant === 'dial') return dial(api);
            if (api.variant === 'tile') return tile(api);
            return line(api, api.variant === 'fader');
        }
    });
})();
