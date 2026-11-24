// HomeDash climate widget. Hero: dial (H4, thermostat knob with steppers and a mode bar). stepper and scale are placeholders until their milestone.
// Command sending is not wired in M1: the knob, steppers and mode bar expose no input callback, so the widget only displays state.
(function () {
    'use strict';
    const E = window.HomeDashEngine;
    if (!E || !E.core || !E.format || !E.prim || !E.widgets) throw new Error('hd-w-climate.js: load hd-core, hd-format, the primitives and hd-widgets first');
    const { el, setAttr } = E.core, F = E.format, MID_DOT = ' ' + String.fromCodePoint(0xb7) + ' ';
    const DEG_C = String.fromCodePoint(0xb0) + 'C', HOST_WORDS = ['unavailable', 'unknown', 'error'];
    const ACTIONS = ['heating', 'cooling', 'idle'], MODES = ['off', 'heat', 'cool', 'auto'];
    // Defaults of the Home Assistant climate platform, used until the entity reports its range.
    const FALLBACK_MIN = 7, FALLBACK_MAX = 35;

    // Status text per module rule 5: null lets the host words win (no entity, unavailable, unknown, error or pending).
    function statusText(api, e, word) {
        if (!e || e.available === false || HOST_WORDS.includes(e.status) || e.pending) return null;
        if (e.readonly) return word + MID_DOT + api.t('homedash.state.readonly');
        if (!api.can('primary', 'set_temperature')) return api.t('homedash.state.blocked');
        if (e.stale) {
            const then = Date.parse(e.attributes && e.attributes.updated_at), age = Number.isFinite(then) ? F.duration(Math.max(0, (Date.now() - then) / 1000), api.lang) : '';
            return api.t('homedash.state.updated_ago', { age });
        }
        return word;
    }
    const num = v => (v === null || v === undefined || v === '' ? NaN : Number(v));
    const finiteOr = (v, fallback) => (Number.isFinite(num(v)) ? num(v) : fallback);

    function dial(api) {
        const inst = api.family === 'instrument', tact = api.family === 'tactile', glass = api.family === 'glass', t = api.t;
        const first = api.entity('primary'), attrs = (first && first.attributes) || {};
        let min = finiteOr(attrs.min_temp, FALLBACK_MIN), max = finiteOr(attrs.max_temp, FALLBACK_MAX);
        if (!(max > min)) { min = FALLBACK_MIN; max = FALLBACK_MAX; }
        const knob = api.prim.knob({ label: api.widget.title, variant: 'ring', min, max, step: 0.5, decimals: 1, unit: DEG_C, activeSegment: true, legends: true,
            ticks: inst || tact ? (max - min) * 2 : 0, ticksMajor: tact ? (max - min) / 5 : 0, numerals: tact ? 6 : 0,
            dots: glass ? { count: max - min + 1, r: 74, size: 1.4, lit: false } : null, hardware: tact, needle: tact, handleShadow: glass,
            readout: inst ? 'seg7' : 'numeral', extra: true, subReadout: inst ? 'seg7' : 'numeral', subIcon: 'thermometer',
            captions: inst ? [t('homedash.climate.target'), t('homedash.climate.current')] : null, lang: api.lang });
        const lower = api.prim.pushButton({ label: t('homedash.climate.lower'), icon: 'minus', dir: -1, variant: 'chip' });
        const raise = api.prim.pushButton({ label: t('homedash.climate.raise'), icon: 'plus', dir: 1, variant: 'chip' });
        const modeLabels = { off: t('homedash.climate.mode_off'), heat: t('homedash.climate.mode_heat'), cool: t('homedash.climate.mode_cool'), auto: t('homedash.climate.mode_auto') };
        const seg = api.prim.segmented({ label: api.widget.title, rotary: tact, options: [{ id: 'off', icon: 'power', label: modeLabels.off }, { id: 'heat', icon: 'flame', label: modeLabels.heat },
            { id: 'cool', icon: 'snowflake', label: modeLabels.cool }, { id: 'auto', icon: 'mode-auto', label: modeLabels.auto }] });
        const box = el('div', { class: 'hd-climate-dialbox' }, [knob.el, el('div', { class: 'hd-climate-steps' }, [lower.el, raise.el])]);
        // Tactile only: the action lamp sits inside the dialbox after the knob; CSS positions it beside the readout plate.
        const lamp = tact ? api.prim.lamp({ label: t('homedash.climate.idle') }) : null;
        if (lamp) box.insertBefore(lamp.el, box.lastChild);
        api.body.appendChild(box); api.body.appendChild(seg.el);
        setAttr(api.body, 'data-hd-nodata', t('homedash.state.no_data'));
        const all = [knob, lower, raise, seg].concat(lamp ? [lamp] : []);
        return {
            update(view) {
                const e = view.entities.primary, a = (e && e.attributes) || {}, mode = e && MODES.includes(e.value) ? e.value : null;
                const target = e ? num(a.temperature) : NaN, current = e ? num(a.current_temperature) : NaN;
                const action = mode === 'off' ? 'off' : ACTIONS.includes(a.hvac_action) ? a.hvac_action : 'idle';
                const actionWord = action === 'off' ? t('homedash.climate.mode_off') : t('homedash.climate.' + (action === 'heating' ? 'heating_to' : action === 'cooling' ? 'cooling_to' : 'idle'));
                const lo = finiteOr(a.min_temp, min), hi = finiteOr(a.max_temp, max), common = { status: e ? e.status : 'unavailable', pending: !!(e && e.pending), readonly: !!(e && e.readonly) };
                const disabledTemp = !e || !api.can('primary', 'set_temperature');
                setAttr(api.body, 'data-hd-action', e ? action : null);
                api.setTone(e && e.tone ? e.tone : null);
                knob.set(Object.assign({ value: Number.isFinite(target) ? target : null, current: Number.isFinite(current) ? current : null, min: hi > lo ? lo : min, max: hi > lo ? hi : max,
                    action: actionWord, sub: Number.isFinite(current) ? current : null, disabled: disabledTemp,
                    valueText: Number.isFinite(target) ? t('homedash.climate.valuetext', { value: F.valueText(target, { decimals: 1, lang: api.lang, unit: DEG_C }), action: actionWord }) : F.DASH }, common));
                for (const b of [lower, raise]) b.set(Object.assign({ disabled: disabledTemp || mode === 'off' }, common));
                seg.set(Object.assign({ value: mode, disabled: !e || !api.can('primary', 'set_hvac_mode') }, common));
                if (lamp) lamp.set({ on: action === 'heating' || action === 'cooling', text: actionWord, status: common.status });
                api.setStatus(statusText(api, e, mode ? modeLabels[mode] : e && e.value ? String(e.value) : ''));
            },
            destroy() { for (const p of all) p.destroy(); box.remove(); seg.el.remove(); }
        };
    }

    E.widgets.register('climate', {
        bindings: ['primary'],
        variants: ['dial', 'stepper', 'scale'],
        create(api) {
            if (api.variant === 'dial') return dial(api);
            return { update() { api.setStatus(null); } };
        }
    });
})();
