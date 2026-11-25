// HomeDash action button (W1): scene and script activate, button and input_button press. A momentary command: a 202 without `expected` confirms at
// once and the check glyph shows (e.confirmed). Sensitive capabilities and widget confirm:true arm for 4 s in the command client (rule 8); the widget
// only calls api.command and shows the armed text from the view. A read-only entity keeps full strength, announces its long reason and sends nothing.
(function () {
    'use strict';
    const E = window.HomeDashEngine;
    if (!E || !E.core || !E.prim || !E.widgets || !E.control || !E.input) throw new Error('hd-w-action.js: load hd-core, hd-input, the primitives, hd-widgets and hd-control first');
    const { setAttr } = E.core, C = E.control;
    const cmdOf = e => (e && e.kind === 'button' ? 'press' : 'activate');
    const verbKey = e => (e && e.kind === 'button' ? 'homedash.action.press' : e && e.kind === 'script' ? 'homedash.action.run' : 'homedash.action.activate');
    const kindIcon = e => (e && (e.kind === 'script' || e.kind === 'button') ? e.kind : 'scene');

    function create(api) {
        let cur = null;
        const input = {
            enabled: () => C.enabled(api, cur, [cmdOf(cur)]),
            onBlocked: () => C.announceBlocked(api, cur, cmdOf(cur)),
            onActivate: () => C.tap(api, cur, () => { if (cur) api.command('primary', cmdOf(cur), {}); }),
            onHold: () => {}   // more-info is for lights only (rule 11)
        };
        const prim = E.prim.pushButton({ variant: 'chip', icon: 'scene', toggle: false, label: api.t('homedash.action.activate'), input }), node = prim.el;
        api.body.appendChild(node); api.body.appendChild(C.armMarker());
        setAttr(api.body, 'data-hd-nodata', api.t('homedash.state.no_data'));
        return {
            update(view) {
                const e = view.entities.primary; cur = e || null;
                api.setTone(e && e.tone ? e.tone : null);
                const word = e ? api.t('homedash.kind.' + (e.kind === 'script' ? 'script' : e.kind === 'button' ? 'button' : 'scene')) : '';
                prim.set({ text: api.t(verbKey(e)), icon: e && e.confirmed ? 'check' : E.widgets.iconFor(api.widget, e) || kindIcon(e), on: false,
                    status: e ? e.status : 'unavailable', pending: !!(e && e.pending), readonly: !!(e && e.readonly), disabled: !e || (!e.readonly && !api.can('primary', cmdOf(e))) });
                setAttr(node, 'aria-label', C.label(api, e) || null);
                api.setStatus(C.statusText(api, e, word, { cap: cmdOf(e), done: true }));
            },
            destroy() { prim.destroy(); }
        };
    }

    E.widgets.register('action', { bindings: ['primary'], variants: ['button'], create });
})();
