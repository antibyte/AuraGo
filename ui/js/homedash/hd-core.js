// HomeDash engine core: DOM builders (no innerHTML, no style attributes), emitter, rAF coalescer, AbortController bag,
// ids, clone, motion and typing checks, translator. Creates window.HomeDashEngine; the other engine files extend it.
(function () {
    'use strict';
    const E = window.HomeDashEngine = window.HomeDashEngine || {};
    const SVG_NS = 'http://www.w3.org/2000/svg';
    let counter = 0;
    function setVars(node, vars) {
        for (const [name, value] of Object.entries(vars || {})) {
            if (!name.startsWith('--')) throw new Error('hd-core: setVars writes custom properties only, got ' + name);
            if (value === null || value === undefined) node.style.removeProperty(name); else node.style.setProperty(name, String(value));
        }
        return node;
    }
    function setAttr(node, name, value) {
        if (name === 'style') throw new Error('hd-core: no style attributes; use setVars');
        if (value === null || value === undefined || value === false) node.removeAttribute(name);
        else node.setAttribute(name, value === true ? 'true' : String(value));
        return node;
    }
    function applyAttrs(node, attrs) {
        for (const [key, value] of Object.entries(attrs || {})) {
            if (key === 'text') node.textContent = value == null ? '' : String(value);
            else if (key === 'vars') setVars(node, value); else setAttr(node, key, value);
        }
        return node;
    }
    function append(node, children) {
        for (const child of [].concat(children == null ? [] : children)) {
            if (child === null || child === undefined || child === false) continue;
            node.appendChild(typeof child === 'string' || typeof child === 'number' ? document.createTextNode(String(child)) : child);
        }
        return node;
    }
    const el = (tag, attrs, children) => append(applyAttrs(document.createElement(tag), attrs), children);
    const svg = (tag, attrs, children) => append(applyAttrs(document.createElementNS(SVG_NS, tag), attrs), children);
    function emitter() {
        const map = new Map();
        const api = {
            on(type, fn) { if (!map.has(type)) map.set(type, new Set()); map.get(type).add(fn); return () => api.off(type, fn); },
            off(type, fn) { const s = map.get(type); if (s) s.delete(fn); },
            emit(type, payload) { for (const fn of [...(map.get(type) || [])]) fn(payload); }
        };
        return api;
    }
    function frame(fn, opts) {
        const raf = (opts && opts.raf) || (typeof requestAnimationFrame === 'function' ? requestAnimationFrame : cb => setTimeout(cb, 16));
        const caf = (opts && opts.caf) || (typeof cancelAnimationFrame === 'function' ? cancelAnimationFrame : clearTimeout);
        let handle = null;
        const schedule = () => { if (handle === null) handle = raf(() => { handle = null; fn(); }); };
        schedule.cancel = () => { if (handle !== null) { caf(handle); handle = null; } };
        schedule.pending = () => handle !== null;
        return schedule;
    }
    function bag() {
        const controller = new AbortController(), disposers = [];
        return {
            signal: controller.signal,
            on(target, type, fn, options) { target.addEventListener(type, fn, Object.assign({}, options, { signal: controller.signal })); },
            add(fn) { disposers.push(fn); return fn; },
            dispose() { if (controller.signal.aborted) return; controller.abort(); while (disposers.length) { try { disposers.pop()(); } catch (err) { console.error(err); } } }
        };
    }
    const clamp = (v, lo, hi) => Math.min(hi, Math.max(lo, v));
    const uid = prefix => (prefix || 'hd') + '-' + (++counter).toString(36);
    function randomId(prefix, length) {
        const n = length || 12, abc = 'abcdefghijklmnopqrstuvwxyz0123456789', bytes = new Uint8Array(n);
        if (typeof crypto !== 'undefined' && crypto.getRandomValues) crypto.getRandomValues(bytes); else for (let i = 0; i < n; i++) bytes[i] = Math.floor(Math.random() * 256);
        let out = ''; for (const b of bytes) out += abc[b % abc.length];
        return (prefix ? prefix + '_' : '') + out;
    }
    const clone = v => (v === undefined ? undefined : JSON.parse(JSON.stringify(v)));
    function motionReduced(root) {
        const mode = root && root.getAttribute ? root.getAttribute('data-hd-motion') : null;
        if (mode === 'reduced' || mode === 'none') return true;
        if (mode === 'full') return false;
        return typeof matchMedia === 'function' && matchMedia('(prefers-reduced-motion: reduce)').matches;
    }
    function makeT(dict, options) {
        const warn = (options && options.warn) || (m => console.warn(m)), warned = new Set();
        return function translate(key, vars) {
            const has = dict && Object.prototype.hasOwnProperty.call(dict, key) && dict[key] != null;
            if (!has) { if (!warned.has(key)) { warned.add(key); warn('HomeDash: missing translation ' + key); } return key; }
            let text = String(dict[key]);
            for (const [name, value] of Object.entries(vars || {})) text = text.split('{{' + name + '}}').join(String(value));
            return text;
        };
    }
    function isTyping(target) {
        if (!target || target.nodeType !== 1) return false;
        if (target.isContentEditable) return true;
        const tag = String(target.tagName || '').toLowerCase();
        if (tag === 'textarea' || tag === 'select') return true;
        if (tag !== 'input') return false;
        return !['checkbox', 'radio', 'button', 'submit', 'reset', 'range', 'color', 'file', 'image'].includes(String(target.getAttribute('type') || 'text').toLowerCase());
    }
    E.core = Object.freeze({ SVG_NS, el, svg, append, applyAttrs, setVars, setAttr, emitter, frame, bag, clamp, uid, randomId, clone, motionReduced, makeT, isTyping });
    E.el = el; E.svg = svg;
})();
