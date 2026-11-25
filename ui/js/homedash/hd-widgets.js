// HomeDash widget registry and frame host (contract C3). register(type, {bindings, variants, create(api)}) stores a widget type;
// host({...}) builds div.hd-widget around a frame primitive, resolves the style family, wires the frozen api and keeps the state line.
(function () {
    'use strict';
    const E = window.HomeDashEngine;
    if (!E || !E.core || !E.format || !E.icons || !E.prim || !E.prim.frame) throw new Error('hd-widgets.js: load hd-core, hd-format, hd-icons and hd-prim-displays first');
    const { el, setAttr, bag } = E.core, F = E.format;
    const registry = new Map();
    const FACTORY_FNS = ['entity', 'can', 'command', 'moreInfo', 'history', 'announce'];
    const STATUSES = ['ok', 'unavailable', 'unknown', 'error'];
    const noop = () => {};
    // Shared by the hero modules: staleText(api, e) is "Updated N min ago" in whole minutes, or null when updated_at does not parse.
    const heroStatus = Object.freeze({
        staleText(api, e) {
            const then = Date.parse(e && e.attributes && e.attributes.updated_at);
            if (!Number.isFinite(then)) return null;
            const minutes = Math.max(1, Math.floor(Math.max(0, Date.now() - then) / 60000));
            return api.t('homedash.state.updated_ago', { age: F.duration(minutes * 60, api.lang) });
        }
    });

    function register(type, def) {
        if (typeof type !== 'string' || !/^[a-z][a-z0-9_]*$/.test(type)) throw new Error('hd-widgets: invalid widget type ' + type);
        if (!def || !Array.isArray(def.bindings) || !def.bindings.every(b => typeof b === 'string')) throw new Error('hd-widgets: bindings must be an array of strings');
        if (!Array.isArray(def.variants) || !def.variants.length || !def.variants.every(v => typeof v === 'string')) throw new Error('hd-widgets: variants must be a non-empty array of strings');
        if (typeof def.create !== 'function') throw new Error('hd-widgets: create must be a function');
        if (registry.has(type)) throw new Error('widget type ' + type + ' is already registered');
        registry.set(type, Object.freeze({ bindings: Object.freeze(def.bindings.slice()), variants: Object.freeze(def.variants.slice()), create: def.create }));
    }
    const get = type => registry.get(type) || null;
    const types = () => Array.from(registry.keys());

    function resolveFamily(widget, parent) {
        if (widget.family) return String(widget.family);
        const holder = parent && typeof parent.closest === 'function' ? parent.closest('[data-hd-family]') : null;
        return (holder ? holder.getAttribute('data-hd-family') : '') || 'glass';
    }
    const sizeText = size => String((size && size.w) || 1) + 'x' + String((size && size.h) || 1);

    function host(opts) {
        const o = opts || {}, widget = o.widget || {}, def = registry.get(widget.type);
        if (!def) throw new Error('hd-widgets: unknown widget type ' + widget.type);
        const t = typeof o.t === 'function' ? o.t : k => k, lang = o.lang || 'en';
        let mode = o.mode || 'view', destroyed = false, toneOverride, statusText = null;
        const variant = def.variants.includes(widget.variant) ? widget.variant : def.variants[0];
        const family = resolveFamily(widget, o.parent);
        const frame = E.prim.frame({ title: '', icon: widget.icon || null, more: o.moreLabel || null });
        const wrapper = el('div', { class: 'hd-widget', 'data-hd-type': widget.type, 'data-hd-variant': variant, 'data-hd-family': family, 'data-hd-size': sizeText(o.size),
            'data-hd-status': 'ok', role: 'group', 'aria-labelledby': frame.titleId }, frame.el);
        if (o.parent) o.parent.appendChild(wrapper);

        const resources = bag();
        const abandon = () => { resources.dispose(); frame.destroy(); wrapper.remove(); };
        let fns;
        try {
            fns = typeof o.api === 'function' ? o.api({ widget, mode, signal: resources.signal }) : null;
            for (const name of FACTORY_FNS) if (!fns || typeof fns[name] !== 'function') throw new Error('hd-widgets: api.' + name + ' missing');
        } catch (err) { abandon(); throw err; }

        const api = Object.freeze(Object.defineProperties({
            body: frame.body, widget, variant, family, t, fmt: F, prim: E.prim, icons: E.icons, signal: resources.signal, lang,
            entity: fns.entity, can: fns.can, command: fns.command, moreInfo: fns.moreInfo, history: fns.history, announce: fns.announce,
            setTone(tone) { toneOverride = tone == null ? undefined : tone; writeTone(currentPrimary()); },
            setStatus(text) { statusText = text == null ? null : String(text); writeState(currentPrimary()); }
        }, { mode: { get: () => mode, enumerable: true } }));

        const currentPrimary = () => (def.bindings.length ? fns.entity(def.bindings[0]) || null : null);
        function writeTone(p) { setAttr(wrapper, 'data-hd-tone', toneOverride ?? (p && p.tone)); }
        function writeState(p) {
            const w = p ? F.stateWord(p) : '';
            const stateText = ['unavailable', 'unknown', 'error', 'pending'].includes(w) ? t(F.STATE_PREFIX + w) : statusText !== null ? statusText : (p ? F.stateText(p, t) : '');
            frame.set({ title: widget.title || (p && p.name) || '', stateText });
        }
        let instance;
        try {
            instance = def.create(api);
            if (!instance || typeof instance.update !== 'function') throw new Error('hd-widgets: create must return an object with update');
        } catch (err) { abandon(); throw err; }
        const inst = { update: instance.update.bind(instance), resize: typeof instance.resize === 'function' ? instance.resize.bind(instance) : noop, destroy: typeof instance.destroy === 'function' ? instance.destroy.bind(instance) : noop };

        function update() {
            if (destroyed) return;
            const entities = {};
            for (const b of def.bindings) entities[b] = fns.entity(b) || null;
            const p = def.bindings.length ? entities[def.bindings[0]] : null;
            setAttr(wrapper, 'data-hd-status', !p ? 'ok' : p.available === false ? 'unavailable' : STATUSES.includes(p.status) ? p.status : 'ok');
            const on = F.isOn(p); setAttr(wrapper, 'data-hd-on', on === null ? null : String(on));
            setAttr(wrapper, 'data-hd-pending', !!(p && p.pending)); setAttr(wrapper, 'data-hd-stale', !!(p && p.stale)); setAttr(wrapper, 'data-hd-readonly', !!(p && p.readonly));
            writeTone(p); writeState(p);
            inst.update({ mode, entities });
        }
        function applyMode() {
            const inert = mode === 'design' || mode === 'thumb';
            setAttr(frame.body, 'inert', inert ? '' : null);
            setAttr(frame.el, 'inert', mode === 'thumb' ? '' : null);
            setAttr(wrapper, 'tabindex', mode === 'design' ? '0' : null);
            setAttr(wrapper, 'aria-hidden', mode === 'thumb' ? 'true' : null);
            setAttr(wrapper, 'data-hd-motion', mode === 'thumb' ? 'none' : null);
        }
        function setMode(m) { if (destroyed) return; mode = m; applyMode(); update(); }
        function resize(size) { if (destroyed) return; setAttr(wrapper, 'data-hd-size', sizeText(size)); inst.resize(size); }
        function destroy() {
            if (destroyed) return; destroyed = true;
            resources.dispose(); inst.destroy(); frame.destroy(); wrapper.remove();
        }
        applyMode(); update();
        return { el: wrapper, body: frame.body, family, update, resize, setMode, destroy };
    }

    // A computed colour string as [r, g, b] (0-255), or null when it does not resolve. rgb()/rgba() are parsed directly; anything else a browser
    // can compute (oklch(), color(srgb ...), color-mix(), named colours) goes through a 1x1 canvas, which normalises it to sRGB. An invalid or
    // fully transparent string leaves the transparent default and reads as null.
    const RGB = /^rgba?\(\s*(\d+(?:\.\d+)?)[,\s]+(\d+(?:\.\d+)?)[,\s]+(\d+(?:\.\d+)?)/;
    function resolveColour(doc, value) {
        const text = String(value || '').trim();
        if (!text) return null;
        const m = RGB.exec(text);
        if (m) return [Number(m[1]), Number(m[2]), Number(m[3])];
        try {
            const canvas = doc.createElement('canvas'), g = typeof canvas.getContext === 'function' ? canvas.getContext('2d', { willReadFrequently: true }) : null;
            if (!g) return null;
            canvas.width = canvas.height = 1; g.clearRect(0, 0, 1, 1); g.fillStyle = 'rgba(0, 0, 0, 0)'; g.fillStyle = text; g.fillRect(0, 0, 1, 1);
            const d = g.getImageData(0, 0, 1, 1).data;
            return d[3] > 0 ? [d[0], d[1], d[2]] : null;
        } catch (err) { return null; }
    }

    // Resolves the host's look into two derived values on `root`: --hd-hud (E.colour.hudFor of the resolved accent; removed when the accent does
    // not resolve) and data-hd-corners="square" when the theme's card radius is 0. Called by the hosts after a palette or theme change, never at load.
    function applyLook(root) {
        if (!root || root.nodeType !== 1 || !E.colour || typeof E.colour.hudFor !== 'function') return;
        const doc = root.ownerDocument, view = (doc && doc.defaultView) || (typeof window !== 'undefined' ? window : null);
        if (!view || typeof view.getComputedStyle !== 'function') return;
        let probe = null;
        try {
            probe = el('span', { 'aria-hidden': 'true' }); probe.style.setProperty('display', 'none'); probe.style.setProperty('color', 'var(--hd-accent)');
            root.appendChild(probe);
            const rgb = resolveColour(doc, view.getComputedStyle(probe).color);
            if (rgb) root.style.setProperty('--hd-hud', E.colour.hudFor(E.colour.rgbToHex(rgb)));
            else if (typeof root.style.removeProperty === 'function') root.style.removeProperty('--hd-hud');
            const radius = String(view.getComputedStyle(root).getPropertyValue('--vd-mat-card-radius') || ''), nums = radius.match(/-?\d*\.?\d+/g);
            setAttr(root, 'data-hd-corners', nums && nums.every(n => Number(n) === 0) ? 'square' : null);
        } finally { if (probe) probe.remove(); }
    }

    E.widgets = Object.freeze({ register, get, types, host, applyLook, heroStatus });
})();
