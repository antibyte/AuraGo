// HomeDash clipboard (frontend 5.5, M4 contract C11). encode(doc, pageId, ids) is the text kept at localStorage['homedash.clipboard.v1']:
// {v:1, widgets:[{widget (no id), lgSize:{w,h}}]} in lg reading order. decode(text, {catalog, pages}) treats the text as untrusted (another window,
// tab or a hand edit): at most 256 KiB, v 1, at most 120 widgets, each rebuilt from known fields only (sanitizeWidget) and sized within its variant.
// channel({storage, win}) wraps the storage: write() tells this realm's listeners at once, the 'storage' event tells other tabs.
(function () {
    'use strict';
    const E = window.HomeDashEngine;
    if (!E || !E.core || !E.grid) throw new Error('hd-clipboard.js: load hd-core.js and hd-grid.js first');
    const { clone, clamp } = E.core, KEY = 'homedash.clipboard.v1', MAX_BYTES = 262144, MAX_WIDGETS = 120;
    const SOURCE = /^[a-z][a-z0-9_]{1,15}$/, ICON = /^[a-z0-9-]{1,48}$/, HEX = /^#[0-9a-f]{6}$/, DO = ['default', 'toggle', 'more_info', 'navigate', 'none'];
    const NATIVE = /^[\p{L}\p{M}\p{N}\p{P}\p{S}]+$/u; // Go IsPrint without the space; bidi controls are Cf
    // Entity id grammar of internal/homedash/ids.go ParseEntityID: source (2-16) ':' native (1-200 bytes, printable, no space, no bidi control); the native part may hold ':'.
    function entityId(s) { if (typeof s !== 'string') return false; const i = s.indexOf(':'); if (i < 0 || !SOURCE.test(s.slice(0, i))) return false; const n = s.slice(i + 1); if (!n) return false; let bytes = 0;
        for (const ch of n) { const c = ch.codePointAt(0); bytes += c < 0x80 ? 1 : c < 0x800 ? 2 : c < 0x10000 ? 3 : 4; if (bytes > 200) return false; } return NATIVE.test(n); }
    const isObj = v => v !== null && typeof v === 'object' && !Array.isArray(v);
    // Mirrors the server's text rules (sanitize.go ContainsControl: unicode.IsControl = C0, DEL, C1, plus bidi controls; ContainsURL): control and bidi
    // characters are dropped, a text that still carries a URL or script scheme becomes '' (the server answers url_not_allowed for it).
    const URLISH = ['://', 'javascript:', 'data:', 'vbscript:'];
    function text(v, max) { if (typeof v !== 'string') return ''; let out = '';
        for (const ch of v) { const c = ch.codePointAt(0); if (c < 0x20 || (c >= 0x7f && c <= 0x9f) || c === 0x61c || c === 0x200e || c === 0x200f || (c >= 0x202a && c <= 0x202e) || (c >= 0x2066 && c <= 0x2069)) continue; out += ch; }
        out = Array.from(out).slice(0, max).join(''); const low = out.toLowerCase();
        return URLISH.some(m => low.includes(m)) || low.trim().startsWith('//') ? '' : out; }
    function encode(doc, pageId, ids) {
        const page = doc && Array.isArray(doc.pages) ? doc.pages.find(p => p.id === pageId) : null; if (!page) return '';
        const want = new Set(ids || []), order = E.grid.sortReading(page.layouts.lg.filter(i => want.has(i.id)));
        const widgets = order.map(i => { const w = page.widgets.find(x => x.id === i.id); if (!w) return null; const c = clone(w); delete c.id; return { widget: c, lgSize: { w: i.w, h: i.h } }; }).filter(Boolean).slice(0, MAX_WIDGETS);
        return widgets.length ? JSON.stringify({ v: 1, widgets }) : '';
    }
    function option(s, val, pages) {
        switch (s.type) {
            case 'bool': return typeof val === 'boolean' ? val : undefined;
            case 'int': case 'number': {
                if (val === null && s.default === null) return null; if (typeof val !== 'number' || !Number.isFinite(val)) return undefined;
                const n = s.type === 'int' ? Math.round(val) : val; return clamp(n, Number.isFinite(s.min) ? s.min : -1e12, Number.isFinite(s.max) ? s.max : 1e12); }
            case 'string': return typeof val === 'string' ? text(val, Math.min(s.max_len || 200, 200)) : undefined;
            case 'enum': return typeof val === 'string' && Array.isArray(s.enum) && s.enum.includes(val) ? val : undefined;
            case 'color': return val === '' ? '' : typeof val === 'string' && HEX.test(val.toLowerCase()) ? val.toLowerCase() : undefined;
            case 'page': return typeof val === 'string' && pages && pages.has(val) ? val : undefined;
            default: return undefined;
        }
    }
    function sanitizeWidget(raw, catalog, o) {
        const opts = o || {}; if (!isObj(raw) || typeof raw.type !== 'string' || !catalog || !catalog.has(raw.type)) return null;
        const type = raw.type, spec = catalog.widget(type), v = catalog.variant(type, typeof raw.variant === 'string' ? raw.variant : ''); if (!v) return null;
        const out = { type, variant: v.id, title: text(raw.title, 80), bindings: {}, options: {}, actions: {}, confirm: raw.confirm === true };
        if (typeof raw.family === 'string' && catalog.families().includes(raw.family)) out.family = raw.family;
        if (typeof raw.icon === 'string' && ICON.test(raw.icon)) out.icon = raw.icon;
        if (isObj(raw.bindings)) for (const b of spec.bindings || []) { const id = b && raw.bindings[b.name]; if (entityId(id)) out.bindings[b.name] = id; }
        if (isObj(raw.options)) for (const s of catalog.optionSpecs(type)) { if (!Object.prototype.hasOwnProperty.call(raw.options, s.key)) continue; const val = option(s, raw.options[s.key], opts.pages); if (val !== undefined) out.options[s.key] = val; }
        if (isObj(raw.actions)) for (const k of ['tap', 'hold']) {
            const a = raw.actions[k]; if (!isObj(a) || !DO.includes(a.do)) continue;
            if (a.do === 'navigate') { if (typeof a.page === 'string' && opts.pages && opts.pages.has(a.page)) out.actions[k] = { do: 'navigate', page: a.page }; continue; }
            if (a.do === 'toggle' && !catalog.canToggle(type)) continue; out.actions[k] = { do: a.do };
        }
        return out;
    }
    function decode(input, o) {
        const opts = o || {}, catalog = opts.catalog; if (typeof input !== 'string' || !input || input.length > MAX_BYTES || !catalog) return [];
        let data; try { data = JSON.parse(input); } catch (err) { return []; }
        if (!isObj(data) || data.v !== 1 || !Array.isArray(data.widgets)) return [];
        const out = [], int = (v, d) => (Number.isFinite(Number(v)) ? Math.round(Number(v)) : d);
        for (const e of data.widgets.slice(0, MAX_WIDGETS)) {
            if (!isObj(e)) continue; const widget = sanitizeWidget(e.widget, catalog, opts); if (!widget) continue;
            const c = catalog.constraints(widget), d = catalog.defaultSize(widget), s = isObj(e.lgSize) ? e.lgSize : {};
            out.push({ widget, size: { w: clamp(int(s.w, d.w), c.minW, Math.min(c.maxW, 12)), h: clamp(int(s.h, d.h), c.minH, c.maxH) } });
        }
        return out;
    }
    function channel(o) {
        const opts = o || {}, store = opts.storage || null, win = opts.win || null, listeners = new Set();
        const read = () => { try { return store ? store.getItem(KEY) : null; } catch (err) { return null; } };
        const tell = v => { for (const fn of [...listeners]) { try { fn(v); } catch (err) { console.error(err); } } };
        const onStorage = ev => { if (ev && (ev.key === KEY || ev.key === null)) tell(read()); };
        if (win && typeof win.addEventListener === 'function') win.addEventListener('storage', onStorage);
        return Object.freeze({
            read, has: () => !!read(),
            write(value) { if (!store || typeof value !== 'string' || !value || value.length > MAX_BYTES) return false; try { store.setItem(KEY, value); } catch (err) { return false; } tell(value); return true; },
            onChange(fn) { listeners.add(fn); return () => listeners.delete(fn); },
            dispose() { listeners.clear(); if (win && typeof win.removeEventListener === 'function') win.removeEventListener('storage', onStorage); }
        });
    }
    E.clipboard = Object.freeze({ KEY, MAX_BYTES, MAX_WIDGETS, encode, decode, sanitizeWidget, channel });
})();
