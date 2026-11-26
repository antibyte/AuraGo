// HomeDash sample states (M4 part 2, D5): one sample entity view per registered widget type (the values of ui/testdata/homedash-lab/*-fixtures.js,
// names through literal keys), deterministic history, and thumb(): a widget host in 'thumb' mode at its thumb size on 56 px cells, scaled into a
// fixed box with --hd-e-tw/-th/-ts. A thumb never sends a command (thumb mode, can() false) and never subscribes; entity(binding) may give a real view.
(function () {
    'use strict';
    const E = window.HomeDashEngine;
    if (!E || !E.core || !E.widgets || !E.catalog) throw new Error('hd-samples.js: load hd-widgets.js and hd-catalog.js first');
    const { el, setVars, clamp, uid } = E.core, DEG_C = String.fromCodePoint(0xb0) + 'C', CELL = 56, GAP = 8;
    const cap = (name, extra) => Object.freeze(Object.assign({ name, allowed: true }, extra || {}));
    function view(id, kind, name, extra) {
        const x = extra || {};
        return Object.freeze(Object.assign({ id, source: 'sample', kind, name, area: '', unit: '', device_class: '', icon_hint: '', value: null, text: '', available: true, status: 'ok', readonly: false,
            readonlyReason: '', serverReadonly: false, capabilities: [], pending: false, stale: false, error: null, tone: null }, x, { attributes: Object.freeze(Object.assign({ updated_at: new Date().toISOString() }, x.attributes)) }));
    }
    const TABLE = Object.freeze({
        switch: t => ({ primary: view('sample:switch', 'switch', t('homedash.palette.sample_lamp'), { value: 'on', icon_hint: 'power', capabilities: [cap('toggle'), cap('turn_on'), cap('turn_off')] }) }),
        light: t => ({ primary: view('sample:light', 'light', t('homedash.palette.sample_light'), { value: 'on', icon_hint: 'bulb', attributes: { brightness_pct: 64, color_mode: 'brightness', supported_color_modes: ['brightness'] },
            capabilities: [cap('toggle'), cap('turn_on'), cap('turn_off'), cap('brightness', { min: 0, max: 100, step: 1 })] }) }),
        action: t => ({ primary: view('sample:scene', 'scene', t('homedash.palette.sample_scene'), { value: '2026-11-24T20:15:00+00:00', icon_hint: 'scene', capabilities: [cap('activate')] }) }),
        select: t => { const options = [t('homedash.palette.sample_low'), t('homedash.palette.sample_mid'), t('homedash.palette.sample_high')];
            return { primary: view('sample:select', 'select', t('homedash.palette.sample_mode'), { value: options[1], icon_hint: 'list', attributes: { options }, capabilities: [cap('select_option', { options })] }) }; },
        number: t => ({ primary: view('sample:number', 'number', t('homedash.palette.sample_volume'), { value: 65, icon_hint: 'number', attributes: { min: 0, max: 100, step: 5, mode: 'slider' }, capabilities: [cap('set_value', { min: 0, max: 100, step: 5 })] }) }),
        climate: t => ({ primary: view('sample:climate', 'climate', t('homedash.palette.sample_room'), { value: 'heat', unit: DEG_C, attributes: { current_temperature: 19.5, temperature: 21.5, min_temp: 5, max_temp: 30, target_temp_step: 0.5,
            hvac_action: 'heating', hvac_modes: ['off', 'heat', 'cool', 'auto'] }, capabilities: [cap('set_temperature', { min: 5, max: 30, step: 0.5 }), cap('hvac_mode', { options: ['off', 'heat', 'cool', 'auto'] })] }) }),
        gauge: t => ({ primary: view('sample:gauge', 'sensor', t('homedash.palette.sample_cpu'), { value: 42, unit: '%', capabilities: [cap('numeric'), cap('history')] }) }),
        temperature: t => ({ primary: view('sample:temperature', 'sensor', t('homedash.palette.sample_room'), { value: 21.4, unit: DEG_C, device_class: 'temperature', capabilities: [cap('numeric'), cap('history')] }),
            humidity: view('sample:humidity', 'sensor', t('homedash.palette.sample_humidity'), { value: 48, unit: '%', device_class: 'humidity', capabilities: [cap('numeric')] }) }),
        sensor: t => ({ primary: view('sample:sensor', 'sensor', t('homedash.palette.sample_power'), { value: 412, unit: 'W', device_class: 'power', icon_hint: 'plug', capabilities: [cap('numeric'), cap('history')] }) }),
        entity: t => ({ primary: view('sample:entity', 'sensor', t('homedash.palette.sample_outside'), { value: 7.5, unit: DEG_C, device_class: 'temperature', capabilities: [cap('numeric')] }) })
    });
    const SERIES = Object.freeze({ sensor: [380, 220, 120], gauge: [42, 15, 6], temperature: [21.2, 1.4, 0.3] });
    const entities = (type, t) => (TABLE[type] ? TABLE[type](t) : { primary: view('sample:' + type, 'generic', t('homedash.widget.' + type + '.name')) });
    function history(type, now) { const p = SERIES[type], end = Number.isFinite(now) ? now : Date.now(); if (!p) return { points: [], source: 'none' }; const points = [];
        for (let i = 0; i < 48; i++) points.push([end - (47 - i) * 1800000, Math.round((p[0] + p[1] * Math.sin(i / 48 * Math.PI * 2 - 1.9) + p[2] * Math.sin(i * 0.9)) * 10) / 10]);
        return { points, source: 'ring' }; }
    // Thumb size: the variant's default size capped to 2x2 for legibility, never below its minimum.
    function thumbSize(catalog, type, variant) { const v = catalog.variant(type, variant) || {}, d = v.default_size || { w: 2, h: 1 }, m = v.min_size || { w: 1, h: 1 };
        return { w: clamp(d.w, m.w, Math.max(m.w, 2)), h: clamp(d.h, m.h, Math.max(m.h, 2)) }; }
    const pxOf = s => ({ width: s.w * CELL + (s.w - 1) * GAP, height: s.h * CELL + (s.h - 1) * GAP });
    const scaleFor = (px, box) => Math.round(Math.min(1, box.width / px.width, box.height / px.height) * 1000) / 1000;
    function thumb(o) {
        const w = o.widget, size = o.size || thumbSize(o.catalog, w.type, w.variant), px = pxOf(size), samples = entities(w.type, o.t);
        const node = el('div', { class: 'hd-thumb', 'aria-hidden': 'true' }); setVars(node, { '--hd-e-tw': px.width + 'px', '--hd-e-th': px.height + 'px', '--hd-e-ts': scaleFor(px, o.box) });
        const entity = b => (typeof o.entity === 'function' && o.entity(b)) || samples[b] || null;
        const api = () => ({ widget: w, t: o.t, lang: o.lang, entity, can: () => false, command: () => Promise.resolve({ status: 'refused' }), moreInfo: () => false, history: () => Promise.resolve(history(w.type)), announce: () => {} });
        let host = null;
        try { host = E.widgets.host({ widget: Object.assign({ id: uid('hd-thumb') }, w), mode: 'thumb', size, t: o.t, lang: o.lang, parent: node, api }); host.resize({ w: size.w, h: size.h, px }); }
        catch (err) { host = null; node.replaceChildren(E.icons.icon('entity')); node.setAttribute('data-hd-thumb', 'none'); }
        if (o.parent) o.parent.appendChild(node);
        return Object.freeze({ el: node, update() { if (host) host.update(); }, destroy() { if (host) host.destroy(); host = null; node.remove(); } });
    }
    E.samples = Object.freeze({ CELL, GAP, entities, history, thumbSize, pxOf, scaleFor, thumb });
})();
