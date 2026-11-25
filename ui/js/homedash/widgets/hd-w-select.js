// HomeDash select widget (W1): segmented (2-3 columns, up to 5 options) and list (picker + listbox popover). Options come from the select_option
// capability, never from the widget; a segmented widget with more options than its width holds (3 at width 1-2, 5 from width 3) renders the list control (Q10). Option and value text is untrusted
// and only ever reaches the DOM through textContent. A click or a listbox pick sends select_option at once (phase discrete); arrow keys on the
// segmented group send phase key (the command client trails 400 ms). Picking the value that is already current sends nothing.
(function () {
    'use strict';
    const E = window.HomeDashEngine;
    if (!E || !E.core || !E.format || !E.prim || !E.widgets || !E.control || !E.input || !E.popover) throw new Error('hd-w-select.js: load hd-core, hd-input, the primitives, hd-widgets, hd-control and hd-popover first');
    const { setAttr } = E.core, F = E.format, C = E.control, CAP = 'select_option';
    const valueOf = e => (e && typeof e.value === 'string' ? e.value : null);
    const flags = (e, off) => ({ status: e ? e.status : 'unavailable', pending: !!(e && e.pending), readonly: !!(e && e.readonly), disabled: off });

    function segmented(api, ctx) {
        let prim = null, sig = null;
        const input = { enabled: ctx.enabled, onBlocked: ctx.blocked, onPick: (id, viaKey) => ctx.pick(id, viaKey ? 'key' : 'discrete') };
        function build(options, label) {
            if (prim) prim.destroy();
            prim = E.prim.segmented({ label, options: options.map(o => ({ id: o, label: o })), input });
            api.body.insertBefore(prim.el, ctx.marker);
        }
        return {
            update(e, options) {
                const label = C.label(api, e), s = JSON.stringify(options);
                if (s !== sig) { sig = s; build(options, label); }   // rebuild only when the option list changes
                prim.set(Object.assign({ value: valueOf(e) }, flags(e, ctx.off(e))));
                setAttr(prim.el, 'aria-label', label || null);
            },
            destroy() { if (prim) prim.destroy(); prim = null; }
        };
    }

    function list(api, ctx) {
        let pop = null, last = null, options = [];
        const paint = () => { if (prim) prim.set(Object.assign({}, last, { expanded: !!pop })); };
        function open() {
            const root = api.body.closest('.hd-root'), e = ctx.cur();
            if (!root || !e) return;
            pop = E.popover.listbox({ root, anchor: prim.el, family: api.family, label: C.label(api, e), options: options.map(o => ({ id: o, label: o })), value: valueOf(e),
                onPick: id => ctx.pick(id, 'discrete'), onClose: () => { pop = null; paint(); } });
            paint();
        }
        const input = { enabled: ctx.enabled, onBlocked: ctx.blocked, onActivate: () => { if (pop) pop.close(true); else open(); }, onHold: () => {} };
        const prim = E.prim.picker({ label: api.widget.title, text: '', input });
        api.body.insertBefore(prim.el, ctx.marker);
        return {
            update(e, opts) {
                options = opts;
                const v = valueOf(e), text = v !== null ? v : e && e.available !== false && e.status !== 'unavailable' && e.text && e.text !== 'unknown' ? e.text : F.DASH;
                last = Object.assign({ text, label: C.label(api, e) }, flags(e, ctx.off(e) || !opts.length));
                paint();
            },
            destroy() { if (pop) pop.close(false); pop = null; prim.destroy(); }
        };
    }

    function create(api) {
        let cur = null, kind = null, part = null, lastView = null;
        const sizeNow = () => { const h = api.body.closest ? api.body.closest('.hd-widget') : null, m = /^(\d+)x/.exec(h ? h.getAttribute('data-hd-size') || '' : ''); return m ? Number(m[1]) : 1; };
        let width = sizeNow();
        const marker = C.armMarker(), ctx = { marker, cur: () => cur,
            enabled: () => C.enabled(api, cur, [CAP]), blocked: () => C.announceBlocked(api, cur, CAP),
            off: e => !e || (!e.readonly && !api.can('primary', CAP)),
            pick(id, phase) { if (!cur || (phase !== 'key' && id === valueOf(cur))) return; api.command('primary', CAP, { option: id }, { phase }); } };
        api.body.appendChild(marker);
        setAttr(api.body, 'data-hd-nodata', api.t('homedash.state.no_data'));
        // Segmented capacity (Q10): 3 options at width 1-2, 5 options from width 3; beyond that the list control renders (each option keeps its 44 px hit target).
        const capacity = () => (width >= 3 ? 5 : 3);
        return {
            resize(size) { const w = size && Number(size.w) > 0 ? Number(size.w) : 1; if (w !== width) { width = w; if (lastView) this.update(lastView); } },
            update(view) {
                lastView = view; const e = view.entities.primary; cur = e || null;
                const options = C.options(e, CAP), want = api.variant === 'list' || options.length > capacity() ? 'list' : 'segmented';
                if (want !== kind) { if (part) part.destroy(); part = want === 'list' ? list(api, ctx) : segmented(api, ctx); kind = want; }
                api.setTone(e && e.tone ? e.tone : null);
                part.update(e, options);
                const v = valueOf(e);
                api.setStatus(C.statusText(api, e, v !== null ? v : e && e.text && e.text !== 'unknown' ? e.text : F.DASH, { cap: CAP }));
            },
            destroy() { if (part) part.destroy(); part = null; marker.remove(); }
        };
    }

    E.widgets.register('select', { bindings: ['primary'], variants: ['segmented', 'list'], create });
})();
