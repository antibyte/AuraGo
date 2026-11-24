// HomeDash switch widget. Heroes: pill (H1, toggle with decor) and rocker (H2). push and tile are placeholders until their milestone.
// Command sending is not wired in M1: the toggle and rocker primitives expose no input callback, so the widgets only display state.
(function () {
    'use strict';
    const E = window.HomeDashEngine;
    if (!E || !E.core || !E.format || !E.prim || !E.widgets) throw new Error('hd-w-switch.js: load hd-core, hd-format, the primitives and hd-widgets first');
    const { setAttr } = E.core, F = E.format, MID_DOT = ' ' + String.fromCodePoint(0xb7) + ' ';
    const HOST_WORDS = ['unavailable', 'unknown', 'error'];

    // Status text per module rule 5: null lets the host words win (no entity, unavailable, unknown, error or pending).
    function statusText(api, e, word) {
        if (!e || e.available === false || HOST_WORDS.includes(e.status) || e.pending) return null;
        if (e.readonly) return word + MID_DOT + api.t('homedash.state.readonly');
        if (!api.can('primary', 'toggle')) return api.t('homedash.state.blocked');
        if (e.stale) { const old = E.widgets.heroStatus.staleText(api, e); if (old) return old; }
        return word;
    }

    function hero(api, build) {
        const prim = build(), node = prim.el;
        api.body.appendChild(node);
        setAttr(api.body, 'data-hd-nodata', api.t('homedash.state.no_data'));
        let lastOn = null;
        return {
            update(view) {
                const e = view.entities.primary, on = !!e && F.isOn(e) === true;
                api.setTone(e && e.tone ? e.tone : null);
                prim.set({ on, status: e ? e.status : 'unavailable', pending: !!(e && e.pending), readonly: !!(e && e.readonly), disabled: !e || !api.can('primary', 'toggle') });
                if (lastOn !== null && lastOn !== on) setAttr(node, 'data-hd-tick', node.getAttribute('data-hd-tick') === 'a' ? 'b' : 'a');
                lastOn = on;
                api.setStatus(statusText(api, e, api.t(on ? 'homedash.state.on' : 'homedash.state.off')));
            },
            destroy() { prim.destroy(); }
        };
    }

    E.widgets.register('switch', {
        bindings: ['primary'],
        variants: ['pill', 'rocker', 'push', 'tile'],
        create(api) {
            if (api.variant === 'pill') return hero(api, () => E.prim.toggle({ label: api.widget.title, decor: true }));
            if (api.variant === 'rocker') return hero(api, () => E.prim.rocker({ label: api.widget.title }));
            return { update() { api.setStatus(null); } };
        }
    });
})();
