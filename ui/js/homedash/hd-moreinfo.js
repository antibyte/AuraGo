// HomeDash more-info for lights (W1; D1: lights only, no presets, no history). build(o) renders the dialog (static; the lab shows it inline);
// create(o) runs it modal inside .hd-root through E.sheet.trap. Rows follow the entity's capabilities: power (always), brightness (`brightness`),
// colour temperature (`color_temp`, grid anchored at min), colour wheel (`color`) and effect (`effect` with options); a row without its capability is
// hidden. Read-only entities keep the controls at full strength with aria-readonly and the long reason in the foot; unavailable ones are aria-disabled.
// Foot: source, last update and the read-only reason. Entity names and effect names reach the DOM through textContent only.
//   build({family, api, popoverRoot, onClose}) -> {el, update(), focusFirst(), destroy()}; api has the widget-api shape (widget, t, lang,
//   entity('primary'), can, command('primary', cap, args, {phase}), announce). create({root}) -> {open(req), close(), isOpen(), destroy()} with
//   req = {api, family, opener, subscribe(fn) -> unsubscribe}.
(function () {
    'use strict';
    const E = window.HomeDashEngine;
    if (!E || !E.core || !E.format || !E.prim || !E.prim.frame || !E.control || !E.popover || !E.widgets) throw new Error('hd-moreinfo.js: load hd-control.js and hd-popover.js first');
    const { el, setAttr } = E.core, F = E.format, C = E.control, NARROW = 600, POWER = ['toggle', 'turn_on', 'turn_off'];
    const finite = v => typeof v === 'number' && Number.isFinite(v);

    function build(o) {
        const api = o.api, t = api.t, family = o.family || 'glass', life = E.core.bag();
        let cur = null, gone = false, expanded = false;
        const frame = E.prim.frame({ title: '', icon: 'bulb' });
        const close = el('button', { class: 'hd-sheet-close', type: 'button', 'aria-label': t('homedash.moreinfo.close') }, E.icons.icon('close'));
        const dialog = el('div', { class: 'hd-sheet hd-widget', role: 'dialog', 'aria-modal': 'true', 'aria-labelledby': frame.titleId, 'data-kind': 'moreinfo', 'data-hd-family': family, 'data-hd-type': 'light' }, [frame.el, close]);
        life.on(close, 'click', () => { if (o.onClose) o.onClose(); });
        const body = el('div', { class: 'hd-mi' }); frame.body.appendChild(body);
        const popRoot = () => o.popoverRoot || dialog.parentNode;
        const rows = {};
        function row(name, icon, label, ...controls) {
            const r = el('div', { class: 'hd-mi-row', 'data-row': name, hidden: 'true' }, [el('span', { class: 'hd-mi-label' }, [E.icons.icon(icon), el('span', { class: 'hd-mi-label-text', text: label })]), el('div', { class: 'hd-mi-line' }, controls.map(c => c.el))]);
            body.appendChild(r); rows[name] = r; return r;
        }
        const gate = (caps, name) => ({ enabled: () => C.enabled(api, cur, caps), onBlocked: () => C.announceBlocked(api, cur, name) });
        const send = (cmd, args, phase) => api.command('primary', cmd, args, { phase });
        const continuous = (cmd, args) => ({ onInput: v => send(cmd, args(v), 'input'), onCommit: v => send(cmd, args(v), 'commit'), onKey: v => send(cmd, args(v), 'key') });

        const power = E.prim.toggle({ size: 'l', label: t('homedash.control.power'), input: Object.assign(gate(POWER, 'toggle'), { onActivate: () => C.activateOnOff(api, cur) }) });
        const bright = E.prim.slider({ size: 'l', label: t('homedash.control.brightness'), min: 0, max: 100, unit: '%', lang: api.lang,
            input: Object.assign(gate('brightness', 'brightness'), continuous('brightness', v => ({ value: v })), { range: () => C.range(cur, 'brightness', { min: 0, max: 100, step: 1 }) }) });
        const ct = E.prim.ctStrip({ size: 'l', label: t('homedash.control.color_temp'), lang: api.lang,
            input: Object.assign(gate('color_temp', 'color_temp'), continuous('color_temp', v => ({ kelvin: v })), { range: () => C.ctRange(cur) }) });
        const wheel = E.prim.colourWheel({ size: 'l', diameter: 220, label: t('homedash.control.color'),
            input: Object.assign(gate('color', 'color'), continuous('color', v => ({ h: v.h, s: v.s })), { valueText: (h, s) => t('homedash.control.hue_sat', { hue: Math.round(h), sat: Math.round(s) }) }) });
        function openEffects() {
            const host = popRoot(); if (!host || gone) return;
            if (E.popover.isOpen(host)) { E.popover.close(host); return; }
            const list = C.options(cur, 'effect'), attrs = (cur && cur.attributes) || {};
            E.popover.listbox({ root: host, anchor: effect.el, family, label: t('homedash.control.effect'), options: list.map(x => ({ id: x, label: x })), value: attrs.effect,
                onPick: v => api.command('primary', 'effect', { option: v }), onClose: () => { expanded = false; if (!gone) update(); } });
            expanded = true; update();
        }
        const effect = E.prim.picker({ size: 'l', label: t('homedash.control.effect'), input: Object.assign(gate('effect', 'effect'), { onActivate: openEffects }) });
        row('power', 'power', t('homedash.control.power'), power); row('bright', 'brightness', t('homedash.control.brightness'), bright);
        row('ct', 'color-temp', t('homedash.control.color_temp'), ct); row('wheel', 'color', t('homedash.control.color'), wheel); row('effect', 'effect', t('homedash.control.effect'), effect);

        function stateLine(m, status) {
            if (m.dead) return status === 'unknown' ? t('homedash.state.unknown') : t('homedash.state.unavailable');
            if (cur.status === 'error') return cur.error && cur.error.text ? String(cur.error.text) : t('homedash.state.error');
            if (cur.pending) return t('homedash.state.pending');
            const word = m.on ? t('homedash.state.on') : t('homedash.state.off'), line = C.statusText(api, cur, word, { cap: 'toggle', done: true });
            return line === null ? word : line;
        }
        function writeFoot() {
            frame.foot.removeAttribute('hidden'); frame.foot.replaceChildren();
            if (!cur) return;
            const parts = [t('homedash.moreinfo.source', { source: C.sourceName(t, cur.source) })], then = Date.parse(cur.attributes && cur.attributes.updated_at);
            if (Number.isFinite(then)) parts.push(t('homedash.moreinfo.updated', { when: F.relTime(then, Date.now(), api.lang) }));
            frame.foot.appendChild(el('span', { class: 'hd-mi-source', text: parts.join(C.MID) }));
            if (cur.readonly) frame.foot.appendChild(el('span', { class: 'hd-mi-reason', text: C.reasonText(t, C.reasonKey(cur, 'toggle'), true) }));
        }
        function update() {
            if (gone) return;
            cur = api.entity('primary') || null;
            const m = C.lightModel(cur), ro = !!(cur && cur.readonly), attrs = (cur && cur.attributes) || {}, onWord = t('homedash.state.on'), offWord = t('homedash.state.off');
            const status = m.dead ? (cur && cur.status === 'unknown' ? 'unknown' : 'unavailable') : cur.status === 'error' ? 'error' : 'ok';
            const st = caps => ({ status, pending: !!(cur && cur.pending), stale: !!(cur && cur.stale), readonly: ro, disabled: m.dead || (!ro && ![].concat(caps).some(c => api.can('primary', c))) });
            const has = name => !!cur && !!C.cap(cur, name);
            for (const [name, show] of [['power', true], ['bright', has('brightness')], ['ct', has('color_temp')], ['wheel', has('color')], ['effect', has('effect')]]) setAttr(rows[name], 'hidden', !show);
            power.set(Object.assign(st(POWER), { on: m.on }));
            bright.set(Object.assign(st('brightness'), C.range(cur, 'brightness', { min: 0, max: 100, step: 1 }), { value: m.dead ? null : m.on ? m.pct : 0,
                valueText: m.dead ? F.DASH : m.known ? t('homedash.light.valuetext', { value: m.pct, state: onWord }) : m.on ? onWord : offWord }));
            const r = C.ctRange(cur), kelvin = attrs.color_mode === 'color_temp' && finite(attrs.color_temp_kelvin) ? attrs.color_temp_kelvin : null;
            ct.set(Object.assign(st('color_temp'), { min: r.min, max: r.max, value: m.dead ? null : kelvin }));
            const hs = Array.isArray(attrs.hs_color) && finite(attrs.hs_color[0]) && finite(attrs.hs_color[1]) ? attrs.hs_color : null;
            wheel.set(Object.assign(st('color'), { hue: hs ? hs[0] : 0, sat: hs ? hs[1] / 100 : 0, valueText: hs && !m.dead ? t('homedash.control.hue_sat', { hue: Math.round(hs[0]), sat: Math.round(hs[1]) }) : F.DASH }));
            effect.set(Object.assign(st('effect'), { text: typeof attrs.effect === 'string' && attrs.effect && !m.dead ? attrs.effect : F.DASH, expanded }));
            frame.set({ title: api.widget.title || (cur && cur.name) || '', sub: (cur && cur.area) || '', stateText: stateLine(m, status), icon: E.widgets.iconFor(api.widget, cur) || 'bulb' });
            const on = F.isOn(cur);
            setAttr(dialog, 'data-hd-status', status); setAttr(dialog, 'data-hd-on', on === null ? null : String(on)); setAttr(dialog, 'data-hd-pending', !!(cur && cur.pending));
            setAttr(dialog, 'data-hd-readonly', ro); setAttr(dialog, 'data-hd-stale', !!(cur && cur.stale)); setAttr(dialog, 'data-hd-tone', cur && cur.tone ? cur.tone : null);
            writeFoot();
        }
        function destroy() {
            if (gone) return; gone = true; life.dispose();
            const host = popRoot(); if (host && E.popover.isOpen(host)) E.popover.close(host);
            for (const p of [power, bright, ct, wheel, effect]) p.destroy();
            frame.destroy(); dialog.remove();
        }
        return { el: dialog, update, focusFirst() { if (power.el.focus) power.el.focus(); }, destroy };
    }

    // The modal host: one layer per root, open() replaces an open sheet. close() releases the trap, which refocuses the opener.
    function create(o) {
        const root = o.root; let layer = null, inst = null, release = null, unsub = null;
        function close() {
            if (!layer) return;
            const l = layer, i = inst, off = release, un = unsub; layer = inst = release = unsub = null;
            if (un) un(); i.destroy(); l.remove(); off();
        }
        function open(req) {
            close();
            const narrow = root.getBoundingClientRect().width < NARROW;
            layer = el('div', { class: 'hd-sheet-layer', 'data-kind': 'moreinfo', 'data-hd-pop': narrow ? 'sheet' : 'center' });
            inst = build({ family: req.family, api: req.api, popoverRoot: layer, onClose: close });
            layer.appendChild(inst.el); root.appendChild(layer);
            release = E.sheet.trap({ container: root, layer, dialog: inst.el, opener: req.opener, onEscape: close, onBackdrop: close });
            const i = inst; unsub = typeof req.subscribe === 'function' ? req.subscribe(() => { if (inst === i) i.update(); }) : null;
            inst.update(); inst.focusFirst();
        }
        return { open, close, isOpen: () => !!layer, destroy: close };
    }
    E.moreinfo = Object.freeze({ build, create });
})();
