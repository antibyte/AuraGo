// HomeDash climate widget. Hero: dial (H4, thermostat knob with steppers and a mode bar). All three variants send commands through api.command with the phases of the other W1
// controls: the target as set_temperature {value} (a knob or scale drag sends input and commit, a tap on the ring sends commit, keys and the plus and minus keys send key, which
// trails 400 ms and coalesces) and, on the dial, the mode bar as hvac_mode {option} (discrete for a click, key for arrow keys). Range and step come from the capability, then
// the entity attributes (min_temp, max_temp, target_temp_step). A read-only entity, a blocked capability, design mode (api.can is false) and the off mode send nothing.
// stepper and scale (G13) are the minus, value and plus keys and the vertical scale version of the same target; both show the current temperature and the mode word.
(function () {
    'use strict';
    const E = window.HomeDashEngine;
    if (!E || !E.core || !E.format || !E.prim || !E.widgets || !E.control || !E.input) throw new Error('hd-w-climate.js: load hd-core, hd-format, hd-input, the primitives, hd-widgets and hd-control first');
    const { el, setAttr, setVars } = E.core, F = E.format, MID_DOT = ' ' + String.fromCodePoint(0xb7) + ' ';
    const DEG_C = String.fromCodePoint(0xb0) + 'C', HOST_WORDS = ['unavailable', 'unknown', 'error'];
    const ACTIONS = ['heating', 'cooling', 'idle'], MODES = ['off', 'heat', 'cool', 'auto'];
    // Defaults of the Home Assistant climate platform, used until the entity reports its range.
    const FALLBACK_MIN = 7, FALLBACK_MAX = 35;

    // Status text per module rule 5: null lets the host words win (no entity, unavailable, unknown, error or pending).
    function statusText(api, e, word) {
        if (!e || e.available === false || HOST_WORDS.includes(e.status) || e.pending) return null;
        if (e.readonly) return word + MID_DOT + api.t('homedash.state.readonly');
        if (!api.can('primary', 'set_temperature')) return api.t('homedash.state.blocked');
        if (e.stale) { const old = E.widgets.heroStatus.staleText(api, e); if (old) return old; }
        return word;
    }
    const num_ = v => (v === null || v === undefined || v === '' ? NaN : Number(v)), num = num_;
    const finiteOr = (v, fallback) => (Number.isFinite(num_(v)) ? num_(v) : fallback);
    const C = E.control, CAP = 'set_temperature', CAP_MODE = 'hvac_mode';
    // Range and step of the target: the capability first (C.range), then the attributes of the entity, then the platform defaults. One source for the knob, the keys and the scale.
    function limitsOf(e) {
        const a = (e && e.attributes) || {}, lo = finiteOr(a.min_temp, FALLBACK_MIN), hi = finiteOr(a.max_temp, FALLBACK_MAX), ok = hi > lo;
        const r = C.range(e, CAP, { min: ok ? lo : FALLBACK_MIN, max: ok ? hi : FALLBACK_MAX, step: finiteOr(a.target_temp_step, 0.5) });
        return r.max > r.min && r.step > 0 ? r : { min: ok ? lo : FALLBACK_MIN, max: ok ? hi : FALLBACK_MAX, step: 0.5 };
    }
    const isDead = e => !e || e.available === false || e.status === 'unavailable' || e.status === 'unknown';
    const modeOf = e => (!isDead(e) && MODES.includes(e.value) ? e.value : null);
    // The target can be sent: a live, writable entity whose capability is open and that is not switched off (the mode bar turns it on again).
    const settable = (api, e) => !!e && C.enabled(api, e, [CAP]) && modeOf(e) !== 'off';
    const sendTarget = (api, e, v, phase) => { if (e && e.available !== false && !e.readonly && modeOf(e) !== 'off' && api.can('primary', CAP)) api.command('primary', CAP, { value: v }, { phase }); };
    function blockedTarget(api, e) { if (e && !e.readonly && e.available !== false && modeOf(e) === 'off' && api.can('primary', CAP)) api.announce(api.t('homedash.climate.mode_off')); else C.announceBlocked(api, e, CAP); }
    // The modes of the mode bar the entity offers (capability options, else the hvac_modes attribute); none listed: all four.
    function modesOf(e) {
        const list = C.options(e, CAP_MODE), attr = e && e.attributes && Array.isArray(e.attributes.hvac_modes) ? e.attributes.hvac_modes.filter(x => typeof x === 'string') : [];
        const have = list.length ? list : attr, ids = MODES.filter(m => !have.length || have.includes(m));
        return ids.length ? ids : MODES.slice();
    }

    function dial(api) {
        const inst = api.family === 'instrument', tact = api.family === 'tactile', glass = api.family === 'glass', t = api.t;
        let cur = null, limits = limitsOf(null), shown = null, nowTemp = null, range = { min: limits.min, max: limits.max };
        const inputRange = () => ({ min: range.min, max: range.max, step: limits.step });
        const targetInput = { enabled: () => settable(api, cur), onBlocked: () => blockedTarget(api, cur), range: inputRange, onHold: () => C.hold(api),
            onInput: v => sendTarget(api, cur, v, 'input'), onCommit: v => sendTarget(api, cur, v, 'commit'), onKey: v => sendTarget(api, cur, v, 'key') };
        function makeKnob(min, max) {
            return api.prim.knob({ label: api.widget.title, variant: 'ring', min, max, step: limits.step, decimals: 1, unit: DEG_C, activeSegment: true, legends: true,
                ticks: inst || tact ? (max - min) * 2 : 0, ticksMajor: tact ? (max - min) / 5 : 0, numerals: tact ? 6 : 0,
                dots: glass ? { count: max - min + 1, r: 74, size: 1.4, lit: false } : null, hardware: tact, needle: tact, handleShadow: glass,
                readout: inst ? 'seg7' : 'numeral', extra: true, subReadout: inst ? 'seg7' : 'numeral', subIcon: 'thermometer',
                captions: inst ? [t('homedash.climate.target'), t('homedash.climate.current')] : null, lang: api.lang, input: targetInput });
        }
        let knob = makeKnob(range.min, range.max);
        const lower = api.prim.pushButton({ label: t('homedash.climate.lower'), icon: 'minus', dir: -1, variant: 'chip' });
        const raise = api.prim.pushButton({ label: t('homedash.climate.raise'), icon: 'plus', dir: 1, variant: 'chip' });
        // The plus and minus keys: one step per click, repeating while held; the value snaps to the entity step inside its range and a limit sends nothing.
        const step = dir => {
            const v = E.input.keyValue(dir > 0 ? 'ArrowUp' : 'ArrowDown', shown !== null ? shown : nowTemp, inputRange());
            if (v === null || v === shown) return; shown = v; sendTarget(api, cur, v, 'key');
        };
        const offSteps = [[lower, -1], [raise, 1]].map(([b, dir]) => E.input.repeat(b.el, { enabled: targetInput.enabled, onBlocked: targetInput.onBlocked, onStep: () => step(dir) }));
        const modeLabels = { off: t('homedash.climate.mode_off'), heat: t('homedash.climate.mode_heat'), cool: t('homedash.climate.mode_cool'), auto: t('homedash.climate.mode_auto') };
        const modeIcons = { off: 'power', heat: 'flame', cool: 'snowflake', auto: 'mode-auto' };
        const modeInput = { enabled: () => !!cur && C.enabled(api, cur, [CAP_MODE]), onBlocked: () => C.announceBlocked(api, cur, CAP_MODE),
            onPick: (id, viaKey) => {
                if (!cur || (!viaKey && id === modeOf(cur))) return;
                if (cur.available !== false && !cur.readonly && api.can('primary', CAP_MODE)) api.command('primary', CAP_MODE, { option: id }, { phase: viaKey ? 'key' : 'discrete' });
            } };
        const makeSeg = ids => api.prim.segmented({ label: api.widget.title, rotary: false, input: modeInput, options: ids.map(id => ({ id, icon: modeIcons[id], label: modeLabels[id] })) });
        let modes = modesOf(null), seg = makeSeg(modes);
        const box = el('div', { class: 'hd-climate-dialbox' }, [knob.el, el('div', { class: 'hd-climate-steps' }, [lower.el, raise.el])]);
        // Tactile only: the action lamp sits inside the dialbox after the knob; CSS positions it beside the readout plate.
        const lamp = tact ? api.prim.lamp({ label: t('homedash.climate.idle') }) : null;
        if (lamp) box.insertBefore(lamp.el, box.lastChild);
        api.body.appendChild(box); api.body.appendChild(seg.el);
        setAttr(api.body, 'data-hd-nodata', t('homedash.state.no_data'));
        const others = [lower, raise].concat(lamp ? [lamp] : []);
        return {
            update(view) {
                const e = view.entities.primary, a = (e && e.attributes) || {}; cur = e || null;
                // No live data (no entity, unavailable or unknown): no old values, no action word, no marker, no selected mode.
                const dead = isDead(e), mode = modeOf(e);
                const target = dead ? NaN : num(a.temperature), current = dead ? NaN : num(a.current_temperature);
                const action = dead ? null : mode === 'off' ? 'off' : ACTIONS.includes(a.hvac_action) ? a.hvac_action : 'idle';
                const actionWord = dead ? '' : action === 'off' ? t('homedash.climate.mode_off') : t('homedash.climate.' + (action === 'heating' ? 'heating_to' : action === 'cooling' ? 'cooling_to' : 'idle'));
                const lim = dead ? limits : limitsOf(e), common = { status: e ? e.status : 'unavailable', pending: !!(e && e.pending), readonly: !!(e && e.readonly) };
                const disabledTemp = !e || (!e.readonly && (!api.can('primary', CAP) || mode === 'off')); // read-only: full strength, the lock badge and aria-disabled tell
                limits = lim; shown = Number.isFinite(target) ? target : null; nowTemp = Number.isFinite(current) ? current : null;
                if (lim.min !== range.min || lim.max !== range.max) {
                    // A live entity reports another range than the knob was built with: rebuild it in place, first child of the dialbox. A documented exception to module rule 3 (ticks,
                    // numerals and dots are drawn from the build range). A dead entity never gets here: it keeps the last range, so the knob does not fall back to the platform defaults.
                    knob.destroy(); range = { min: lim.min, max: lim.max }; knob = makeKnob(range.min, range.max); box.insertBefore(knob.el, box.firstChild);
                }
                if (!dead) {
                    // The mode bar shows the modes the entity offers; another list rebuilds it in place (a dead entity keeps the last bar).
                    const next = modesOf(e);
                    if (next.join() !== modes.join()) { const old = seg; modes = next; seg = makeSeg(modes); box.parentNode.insertBefore(seg.el, old.el); old.destroy(); }
                }
                setAttr(api.body, 'data-hd-action', action);
                api.setTone(e && e.tone ? e.tone : null);
                knob.set(Object.assign({ value: shown, current: nowTemp, min: range.min, max: range.max,
                    action: actionWord, sub: nowTemp, disabled: disabledTemp,
                    valueText: shown !== null ? t('homedash.climate.valuetext', { value: F.valueText(target, { decimals: 1, lang: api.lang, unit: DEG_C }), action: actionWord }) : F.DASH }, common));
                for (const b of [lower, raise]) b.set(Object.assign({ disabled: disabledTemp }, common));
                seg.set(Object.assign({ value: mode, disabled: !e || (!e.readonly && !api.can('primary', CAP_MODE)) }, common));
                if (lamp) lamp.set({ on: action === 'heating' || action === 'cooling', text: actionWord, status: common.status });
                api.setStatus(statusText(api, e, mode ? modeLabels[mode] : e && e.value ? String(e.value) : ''));
            },
            destroy() { for (const off of offSteps) off(); knob.destroy(); seg.destroy(); for (const p of others) p.destroy(); box.remove(); seg.el.remove(); }
        };
    }

    // ---- climate/stepper and climate/scale: the target temperature as keys or as a vertical scale ----
    function setpoint(api, vertical) {
        const t = api.t;
        let cur = null, live = () => {}, decimals = 1;
        const rng = () => limitsOf(cur);
        const send = (v, phase) => sendTarget(api, cur, v, phase);
        const gate = { enabled: () => settable(api, cur), onBlocked: () => blockedTarget(api, cur), range: rng };
        const modeLabels = { off: t('homedash.climate.mode_off'), heat: t('homedash.climate.mode_heat'), cool: t('homedash.climate.mode_cool'), auto: t('homedash.climate.mode_auto') };
        const now = E.prim.numeral({ chars: 4, decimals: 1, unit: 'C', degree: true, lang: api.lang });
        const nowBox = el('span', { class: 'hd-cnow' }, [E.icons.icon('thermometer'), now.el]);
        let prim, num = null, mark = null, box;
        if (vertical) {
            prim = E.prim.slider({ variant: 'v', min: FALLBACK_MIN, max: FALLBACK_MAX, unit: DEG_C, decimals: 1, input: Object.assign({}, gate, { onHold: () => {},
                onInput: v => { live(v); send(v, 'input'); }, onCommit: v => { live(v); send(v, 'commit'); }, onKey: v => { live(v); send(v, 'key'); } }) });
            num = E.prim.numeral({ chars: 4, decimals: 1, unit: 'C', degree: true, lang: api.lang });
            mark = el('span', { class: 'hd-cscale-now', 'aria-hidden': 'true', hidden: '' });
            prim.el.appendChild(mark);
            box = el('div', { class: 'hd-lcol hd-cscale' }, [num.el, prim.el, nowBox]);
            live = v => num.set({ value: v, decimals });
        } else {
            prim = E.prim.stepper({ label: api.widget.title, min: FALLBACK_MIN, max: FALLBACK_MAX, unit: DEG_C, decimals: 1, decLabel: t('homedash.climate.lower'), incLabel: t('homedash.climate.raise'),
                input: Object.assign({}, gate, { onStep: v => send(v, 'key') }) });
            box = el('div', { class: 'hd-cstep' }, [prim.el, nowBox]);
        }
        const marker = C.armMarker();
        api.body.appendChild(box); api.body.appendChild(marker);
        setAttr(api.body, 'data-hd-nodata', t('homedash.state.no_data'));
        return {
            update(view) {
                const e = view.entities.primary, a = (e && e.attributes) || {}; cur = e || null;
                const dead = !e || e.available === false || e.status === 'unavailable' || e.status === 'unknown';
                const mode = !dead && MODES.includes(e.value) ? e.value : null, target = dead ? NaN : num_(a.temperature), current = dead ? NaN : num_(a.current_temperature);
                const action = dead ? null : mode === 'off' ? 'off' : ACTIONS.includes(a.hvac_action) ? a.hvac_action : 'idle', r = rng(); decimals = C.decimalsOf(r.step);
                const common = { status: e ? e.status : 'unavailable', pending: !!(e && e.pending), readonly: !!(e && e.readonly) };
                const off = !e || (!e.readonly && (!api.can('primary', CAP) || mode === 'off'));
                setAttr(api.body, 'data-hd-action', action); api.setTone(e && e.tone ? e.tone : null);
                const shown = Number.isFinite(target) ? target : null;
                prim.set(Object.assign({ value: shown, min: r.min, max: r.max, decimals, unit: DEG_C, disabled: off,
                    valueText: shown !== null ? t('homedash.climate.valuetext', { value: F.valueText(target, { decimals, lang: api.lang, unit: DEG_C }), action: mode ? modeLabels[mode] : '' }) : F.DASH }, common));
                const n = C.label(api, e);
                if (num) { num.set({ value: shown, decimals }); setAttr(prim.el, 'aria-label', n ? n + ': ' + t('homedash.climate.target') : null); } else setAttr(prim.el, 'aria-label', n || null);
                now.set({ value: Number.isFinite(current) ? current : null, decimals: 1 });
                setAttr(nowBox, 'hidden', Number.isFinite(current) ? null : '');
                if (mark) { setAttr(mark, 'hidden', Number.isFinite(current) && !dead ? null : ''); if (Number.isFinite(current)) setVars(mark, { '--hd-cur': Math.min(1, Math.max(0, (current - r.min) / (r.max - r.min))).toFixed(4) }); }
                const word = [mode ? modeLabels[mode] : e && e.value ? String(e.value) : '', Number.isFinite(current) ? F.valueText(current, { decimals: 1, lang: api.lang, unit: DEG_C }) : ''].filter(Boolean).join(MID_DOT);
                api.setStatus(statusText(api, e, word));
            },
            destroy() { prim.destroy(); now.destroy(); if (num) num.destroy(); box.remove(); marker.remove(); }
        };
    }

    E.widgets.register('climate', {
        bindings: ['primary'],
        variants: ['dial', 'stepper', 'scale'],
        create(api) {
            if (api.variant === 'dial') return dial(api);
            return setpoint(api, api.variant === 'scale');
        }
    });
})();
