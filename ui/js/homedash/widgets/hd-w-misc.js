// HomeDash miscellaneous widgets. M0 registers only the entity tile (icon, numeral for numbers/text, lamp for binary state).
(function () {
    'use strict';
    const E = window.HomeDashEngine;
    if (!E || !E.core || !E.format || !E.prim || !E.widgets) throw new Error('hd-w-misc.js: load hd-core, hd-format, the primitives and hd-widgets first');
    const { el, setAttr } = E.core, F = E.format;
    const DURATION_UNITS = Object.freeze({ s: 1, min: 60, h: 3600, d: 86400 });

    E.widgets.register('entity', {
        bindings: ['primary'],
        variants: ['tile'],
        create(api) {
            const icon = E.prim.iconBadge({ icon: api.widget.icon || 'entity', size: 's' });
            const num = E.prim.numeral({ chars: 5, lang: api.lang });
            const lamp = E.prim.lamp({ label: '' });
            api.body.appendChild(el('div', { class: 'hd-tile' }, [icon.el, num.el, lamp.el]));
            const show = (numeralVisible) => { setAttr(num.el, 'hidden', !numeralVisible); setAttr(lamp.el, 'hidden', numeralVisible); };
            return {
                update(v) {
                    const e = v.entities.primary;
                    // SCR-M2-1: a known icon hint names the badge glyph; duration sensors read as a duration, not as seconds.
                    icon.set({ icon: e && E.icons.has(e.icon_hint) ? e.icon_hint : (api.widget.icon || 'entity') });
                    if (!e) { num.set({ value: null }); show(true); return; }
                    if (typeof e.value === 'number') {
                        const k = e.device_class === 'duration' ? DURATION_UNITS[e.unit || 's'] : 0;
                        if (k) num.set({ text: F.duration(e.value * k, api.lang) }); else num.set({ value: e.value, unit: e.unit });
                        show(true); return;
                    }
                    if (e.value == null) { num.set({ value: null }); show(true); return; }
                    const on = F.isOn(e);
                    if (on !== null) { lamp.set({ on, text: F.stateText(e, api.t), status: e.status, pending: !!e.pending }); show(false); return; }
                    num.set({ text: String(e.value) }); show(true);
                },
                destroy() { icon.destroy(); num.destroy(); lamp.destroy(); }
            };
        }
    });
})();
