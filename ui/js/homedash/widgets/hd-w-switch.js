// HomeDash switch widget. Heroes: pill (H1, toggle with decor) and rocker (H2); W1 variants: push (chip) and tile. Every variant is a
// role=switch button with the input contract (P1): one activation sends turn_on/turn_off from the shown state, toggle only as fallback (Q5);
// hold opens more-info for light entities only (rule 11); a read-only entity announces its long reason and sends nothing.
(function () {
    'use strict';
    const E = window.HomeDashEngine;
    if (!E || !E.core || !E.format || !E.prim || !E.widgets || !E.control || !E.input) throw new Error('hd-w-switch.js: load hd-core, hd-format, hd-input, the primitives, hd-widgets and hd-control first');
    const { setAttr } = E.core, F = E.format, C = E.control;
    const CAPS = ['toggle', 'turn_on', 'turn_off'];

    // build(input) returns the primitive for the variant; the widget owns the shared input, the state line and the arm marker.
    function control(api, build, o) {
        let cur = null;
        const input = {
            enabled: () => C.enabled(api, cur, CAPS),
            onBlocked: () => C.announceBlocked(api, cur, 'toggle'),
            onActivate: () => C.tap(api, cur, () => C.activateOnOff(api, cur)),
            onHold: () => { if (cur && cur.kind === 'light') C.hold(api); }   // more-info is for light entities only (rule 11)
        };
        const prim = build(input), node = prim.el;
        api.body.appendChild(node); api.body.appendChild(C.armMarker());
        setAttr(api.body, 'data-hd-nodata', api.t('homedash.state.no_data'));
        let lastOn = null;
        return {
            update(view) {
                const e = view.entities.primary; cur = e || null;
                const on = !!e && F.isOn(e) === true, word = api.t(on ? 'homedash.state.on' : 'homedash.state.off');
                api.setTone(e && e.tone ? e.tone : null);
                const state = { on, status: e ? e.status : 'unavailable', pending: !!(e && e.pending), readonly: !!(e && e.readonly),
                    disabled: !e || (!e.readonly && !CAPS.some(c => api.can('primary', c))) };   // a read-only entity is shown at full strength (lock badge), not dimmed like a disabled one
                if (o.glyph) { state.icon = E.widgets.iconFor(api.widget, e) || 'power'; state.text = o.text ? word : ''; }
                prim.set(state);
                setAttr(node, 'aria-label', C.label(api, e) || null);
                if (lastOn !== null && lastOn !== on) setAttr(node, 'data-hd-tick', node.getAttribute('data-hd-tick') === 'a' ? 'b' : 'a');
                lastOn = on;
                api.setStatus(C.statusText(api, e, word, { cap: 'toggle' }));
            },
            destroy() { prim.destroy(); }
        };
    }

    E.widgets.register('switch', {
        bindings: ['primary'],
        variants: ['pill', 'rocker', 'push', 'tile'],
        create(api) {
            const label = api.widget.title;
            if (api.variant === 'pill') return control(api, input => E.prim.toggle({ label, decor: true, input }), {});
            if (api.variant === 'rocker') return control(api, input => E.prim.rocker({ label, input }), {});
            if (api.variant === 'tile') return control(api, input => E.prim.pushButton({ variant: 'tile', icon: 'power', toggle: true, label: '', input }), { glyph: true, text: false });
            return control(api, input => E.prim.pushButton({ variant: 'chip', icon: 'power', toggle: true, label: '', input }), { glyph: true, text: true });
        }
    });
})();
