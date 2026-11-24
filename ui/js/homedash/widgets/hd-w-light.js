// HomeDash light widget. Hero: dial (H3, dimmer knob). slider, fader and tile are placeholders until their milestone.
// Command sending is not wired in M1: the knob primitive exposes no input callback, so the widget only displays state.
(function () {
    'use strict';
    const E = window.HomeDashEngine;
    if (!E || !E.core || !E.format || !E.prim || !E.widgets) throw new Error('hd-w-light.js: load hd-core, hd-format, the primitives and hd-widgets first');
    const { setAttr } = E.core, F = E.format, MID_DOT = ' ' + String.fromCodePoint(0xb7) + ' ';
    const HOST_WORDS = ['unavailable', 'unknown', 'error'];

    // Status text per module rule 5: null lets the host words win (no entity, unavailable, unknown, error or pending).
    function statusText(api, e, hero) {
        if (!e || e.available === false || HOST_WORDS.includes(e.status) || e.pending) return null;
        if (e.readonly) return hero + MID_DOT + api.t('homedash.state.readonly');
        if (!api.can('primary', 'brightness')) return api.t('homedash.state.blocked');
        if (e.stale) { const old = E.widgets.heroStatus.staleText(api, e); if (old) return old; }
        return hero;
    }

    function dial(api) {
        const inst = api.family === 'instrument', tact = api.family === 'tactile', glass = api.family === 'glass';
        const prim = E.prim.knob({ label: api.widget.title, variant: 'ring', min: 0, max: 100, unit: '%', decimals: 0, ticks: 10, legends: true,
            readout: inst ? 'seg7' : 'numeral', quantize: inst ? 40 : 0, gaps: inst ? 40 : 0, lead: inst, glow: glass || inst,
            dots: tact ? { count: 21, r: 94, size: 4.5, lit: true } : null, hardware: tact, word: true, handleShadow: glass });
        api.body.appendChild(prim.el);
        setAttr(api.body, 'data-hd-nodata', api.t('homedash.state.no_data'));
        const readout = prim.el.querySelector('.hd-dial-readout');
        let lastPct = null;
        return {
            update(view) {
                const e = view.entities.primary, on = !!e && F.isOn(e) === true, raw = e && e.attributes ? Number(e.attributes.brightness) : NaN;
                // On without a usable brightness (null, 0 or not a number): dial full, word "On", no percentage anywhere.
                const known = on && Number.isFinite(raw) && raw > 0;
                const pct = known ? Math.min(100, Math.max(1, Math.round(raw / 2.55))) : on ? 100 : 0;
                const onWord = api.t('homedash.state.on'), offWord = api.t('homedash.state.off');
                api.setTone(e && e.tone ? e.tone : null);
                prim.set({ value: pct, word: known ? null : on ? onWord : offWord, status: e ? e.status : 'unavailable', pending: !!(e && e.pending),
                    readonly: !!(e && e.readonly), disabled: !e || !api.can('primary', 'brightness'),
                    valueText: known ? api.t('homedash.light.valuetext', { value: pct, state: onWord }) : on ? onWord : offWord });
                if (readout && lastPct !== null && lastPct !== pct) setAttr(readout, 'data-hd-tick', readout.getAttribute('data-hd-tick') === 'a' ? 'b' : 'a');
                lastPct = pct;
                api.setStatus(statusText(api, e, known ? onWord + MID_DOT + pct + ' %' : on ? onWord : offWord));
            },
            destroy() { prim.destroy(); }
        };
    }

    E.widgets.register('light', {
        bindings: ['primary'],
        variants: ['slider', 'fader', 'dial', 'tile'],
        create(api) {
            if (api.variant === 'dial') return dial(api);
            return { update() { api.setStatus(null); } };
        }
    });
})();
