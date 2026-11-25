// HomeDash number widget (W1): stepper (minus, value, plus) and slider (.hd-nrow: slider and numeral). The range comes from the set_value capability
// (min, max, step; defaults 0, 100, 1), the readout decimals from the step and the unit from the entity. The primitives take min, max, decimals and
// unit with every set(), so a changed range repaints in place (no rebuild). Stepper steps and keys send set_value with phase key (the command client
// trails 400 ms, rule 7); a slider drag sends input then commit. At a limit nothing is sent. The widget only shows e.value; confirmation is the client's.
(function () {
    'use strict';
    const E = window.HomeDashEngine;
    if (!E || !E.core || !E.format || !E.prim || !E.widgets || !E.control || !E.input) throw new Error('hd-w-number.js: load hd-core, hd-input, the primitives, hd-widgets and hd-control first');
    const { setAttr, el } = E.core, F = E.format, C = E.control, CAP = 'set_value', DEFAULT = { min: 0, max: 100, step: 1 };

    function create(api) {
        const slider = api.variant === 'slider';
        let cur = null, live = () => {};
        const rng = () => C.range(cur, CAP, DEFAULT);
        const send = (v, phase) => { if (cur && cur.available !== false && !cur.readonly && api.can('primary', CAP)) api.command('primary', CAP, { value: v }, { phase }); };
        const gate = { enabled: () => C.enabled(api, cur, [CAP]), onBlocked: () => C.announceBlocked(api, cur, CAP), range: rng };
        const label = api.widget.title;
        let prim, num = null, box = null;
        if (slider) {
            prim = E.prim.slider({ variant: 'h', label, min: 0, max: 100, input: Object.assign({}, gate, { onHold: () => {},
                onInput: v => { live(v); send(v, 'input'); }, onCommit: v => { live(v); send(v, 'commit'); }, onKey: v => { live(v); send(v, 'key'); } }) });
            num = E.prim.numeral({ chars: 5 });
            box = el('div', { class: 'hd-nrow' }, [prim.el, num.el]);
        } else {
            prim = E.prim.stepper({ label, min: 0, max: 100, decLabel: api.t('homedash.control.decrease'), incLabel: api.t('homedash.control.increase'),
                input: Object.assign({}, gate, { onStep: v => send(v, 'key') }) });
        }
        const marker = C.armMarker();
        api.body.appendChild(box || prim.el); api.body.appendChild(marker);
        setAttr(api.body, 'data-hd-nodata', api.t('homedash.state.no_data'));
        let fmt = { decimals: 0, unit: '' };
        live = v => { if (num) num.set({ value: v, decimals: fmt.decimals, unit: fmt.unit }); };
        return {
            update(view) {
                const e = view.entities.primary; cur = e || null;
                const r = rng(), dead = !e || e.available === false || e.status === 'unavailable' || e.status === 'unknown', v = !dead && typeof e.value === 'number' && Number.isFinite(e.value) ? e.value : null;
                fmt = { decimals: C.decimalsOf(r.step), unit: (e && e.unit) || '' };
                api.setTone(e && e.tone ? e.tone : null);
                prim.set({ label: C.label(api, e) || undefined, value: v, min: r.min, max: r.max, decimals: fmt.decimals, unit: fmt.unit, status: e ? e.status : 'unavailable', pending: !!(e && e.pending), readonly: !!(e && e.readonly),
                    disabled: !e || (!e.readonly && !api.can('primary', CAP)) });
                if (num) num.set({ value: v, decimals: fmt.decimals, unit: fmt.unit });
                setAttr(prim.el, 'aria-label', C.label(api, e) || null);
                api.setStatus(C.statusText(api, e, F.valueText(v, { decimals: fmt.decimals, unit: fmt.unit, lang: api.lang }), { cap: CAP }));
            },
            destroy() { prim.destroy(); if (num) num.destroy(); if (box) box.remove(); marker.remove(); }
        };
    }

    E.widgets.register('number', { bindings: ['primary'], variants: ['stepper', 'slider'], create });
})();
