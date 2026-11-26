// HomeDash widget fit survey (test fixture, never shipped). Renders every registered widget type x variant at its default and minimum size (and, in the
// extended set, every allowed size up to 6x4) on the cells of a lg (960 px), md (768 px) or sm (390 px) page, in one style family, with realistic long names, and
// measures header text (visible characters, truncation) and text/control overlaps. Driven by ui/desktop_homedash_widget_fit_test.go through window.hdFit.
(function () {
    'use strict';
    const fit = window.hdFit = { ready: false, error: '', errors: [] };
    for (const m of (window.hdFitEarlyErrors || [])) fit.errors.push(String(m));
    window.addEventListener('error', e => { fit.errors.push(String(e.message)); });
    const GAP = 12, BPS = { lg: { width: 960, cols: 12 }, md: { width: 768, cols: 8 }, sm: { width: 390, cols: 4 } };
    const NAMES = { switch: 'Coffee machine', light: 'Reading light', action: 'Movie night', select: 'Living room heater mode', number: 'Living room volume', climate: 'Living room temperature',
        gauge: 'Living room humidity', temperature: 'Living room temperature', sensor: 'Kitchen power consumption', entity: 'Living room temperature' };
    const TEXT = [['title', '.hd-title'], ['sub', '.hd-sub'], ['state', '.hd-state-text'], ['numeral', '.hd-numeral'], ['label', '.hd-push-label']];
    const CONTROLS = '.hd-toggle, .hd-rocker, .hd-push, .hd-slider-thumb, .hd-slider-track, .hd-ct-thumb, .hd-step, .hd-seg-opt, .hd-pick, .hd-more, .hd-ltile, .hd-iconbadge, .hd-spark, .hd-lamp-dot';
    let E = null, catalog = null, t = null, root = null, family = 'glass';
    const frames = n => new Promise(resolve => { const step = k => (k ? requestAnimationFrame(() => step(k - 1)) : resolve()); step(n); });
    const el = (tag, cls, text) => { const n = document.createElement(tag); if (cls) n.className = cls; if (text) n.textContent = text; return n; };
    const clamp = (v, lo, hi) => Math.min(hi, Math.max(lo, v));

    function realView(type, state) {
        const base = E.samples.entities(type, t).primary, view = Object.assign({}, base, { name: NAMES[type] || base.name, area: 'Kitchen' });
        if (state === 'switch') { Object.assign(view, { kind: 'switch', name: NAMES.switch, value: 'on', unit: '', device_class: '', icon_hint: 'power' }); } // the entity tile takes any entity: a switch brings a state text ("On") the sensor does not
        if (state === 'stale') { view.stale = true; view.attributes = Object.assign({}, base.attributes, { updated_at: new Date(Date.now() - 14 * 60000).toISOString() }); }
        return view;
    }
    function build(parent, type, variant, size, px, state, mode) {
        const sample = mode === 'thumb', widget = { id: 'fit-' + type + '-' + variant, type, variant, title: '', bindings: { primary: 'x' }, options: {}, actions: {}, confirm: false };
        const entities = sample ? E.samples.entities(type, t) : Object.assign({}, E.samples.entities(type, t), { primary: realView(type, state) });
        const api = () => ({ widget, t, lang: 'en', entity: b => entities[b] || entities.primary || null, can: () => !sample, command: () => Promise.resolve({ status: 'local' }), moreInfo() {}, history: () => Promise.resolve(E.samples.history(type)), announce() {} });
        const host = E.widgets.host({ widget, size, mode: sample ? 'thumb' : 'view', t, lang: 'en', api, moreLabel: sample ? null : 'More', moreWhen: p => !!p && p.kind === 'light', parent });
        host.el.style.width = px.width + 'px'; host.el.style.height = px.height + 'px';
        host.update(); host.resize({ w: size.w, h: size.h, px });
        return host;
    }

    // ---- measurement ----------------------------------------------------------------------------------------------------------------
    const clipOf = (node, widgetEl) => {
        const r = node.getBoundingClientRect(), cs = getComputedStyle(node);
        let c = { l: r.left + parseFloat(cs.paddingLeft) + parseFloat(cs.borderLeftWidth), r: r.right - parseFloat(cs.paddingRight) - parseFloat(cs.borderRightWidth), t: r.top, b: r.bottom };
        for (let p = node.parentElement; p; p = p.parentElement) {
            const s = getComputedStyle(p);
            if (s.overflowX !== 'visible' || s.overflowY !== 'visible') { const q = p.getBoundingClientRect(); c = { l: Math.max(c.l, q.left), r: Math.min(c.r, q.right), t: Math.max(c.t, q.top), b: Math.min(c.b, q.bottom) }; }
            if (p === widgetEl) break;
        }
        return c;
    };
    let canvas = null;
    const ellipsisWidth = node => { if (!canvas) canvas = document.createElement('canvas').getContext('2d'); const s = getComputedStyle(node); canvas.font = s.fontStyle + ' ' + s.fontWeight + ' ' + s.fontSize + ' ' + s.fontFamily; return canvas.measureText(String.fromCodePoint(0x2026)).width; };
    function textInfo(kind, node, widgetEl) {
        if (!node.getClientRects().length || !node.textContent.trim()) return null;
        const cs = getComputedStyle(node), r = node.getBoundingClientRect();
        if (cs.position === 'absolute' && r.width <= 1.5 && r.height <= 1.5) return null;
        if (cs.visibility === 'hidden' || Number(cs.opacity) === 0) return null;
        const clip = clipOf(node, widgetEl);
        if (clip.r - clip.l <= 2 || clip.b - clip.t <= 2) return null; // clipped away (a visually hidden line)
        const walker = document.createTreeWalker(node, NodeFilter.SHOW_TEXT), rects = [];
        let total = 0;
        for (let n = walker.nextNode(); n; n = walker.nextNode()) {
            const s = n.nodeValue;
            for (let i = 0; i < s.length; i++) { const rg = document.createRange(); rg.setStart(n, i); rg.setEnd(n, i + 1); const cr = rg.getClientRects()[0]; if (!cr || cr.width <= 0) continue; total++; rects.push(cr); }
        }
        if (!total) return null;
        const rows = kind !== 'numeral', fits = c => c.right <= clip.r + 0.5 && c.left >= clip.l - 0.5 && (!rows || (c.bottom - c.height * 0.15 <= clip.b + 1 && c.top + c.height * 0.15 >= clip.t - 1));
        const inset = kind === 'numeral' ? 0.25 : 0.15, allFit = rects.every(fits), ellipsis = cs.textOverflow === 'ellipsis', ellW = !allFit && ellipsis ? ellipsisWidth(node) : 0;
        const shown = allFit ? total : rects.filter(c => fits(c) && c.right <= clip.r - ellW + 0.5).length;
        const vis = (allFit ? rects : rects.filter(c => fits(c) && c.right <= clip.r - ellW + 0.5)), box = { l: Infinity, r: -Infinity, t: Infinity, b: -Infinity };
        for (const c of vis) { box.l = Math.min(box.l, c.left); box.r = Math.max(box.r, c.right); box.t = Math.min(box.t, c.top + c.height * inset); box.b = Math.max(box.b, c.bottom - c.height * inset); } // the ink band: the line box carries leading above and below the glyphs
        if (!vis.length) { box.l = clip.l; box.r = clip.l + ellW; box.t = r.top; box.b = r.bottom; } else if (!allFit && ellipsis) box.r += ellW;
        const widgetR = widgetEl.getBoundingClientRect();
        return { kind, node, text: node.textContent, total, shown, truncated: !allFit, box, outside: box.r > widgetR.right + 0.5 || box.l < widgetR.left - 0.5 || box.b > widgetR.bottom + 0.5 || box.t < widgetR.top - 0.5 };
    }
    const overlap = (a, b, slack) => Math.min(a.r, b.r) - Math.max(a.l, b.l) > 1 + (slack || 0) && Math.min(a.b, b.b) - Math.max(a.t, b.t) > 1 + (slack || 0); // a chart box is loose by a few px, its line rarely reaches the top of the box
    function controlsOf(widgetEl) {
        const wr = widgetEl.getBoundingClientRect(), out = [];
        for (const node of widgetEl.querySelectorAll(CONTROLS)) {
            if (!node.getClientRects().length) continue;
            const probe = node.matches('.hd-spark') ? node.querySelector('.hd-spark-line') || node : node; // a chart counts by its line, not by its box
            const r = probe.getBoundingClientRect(), cs = getComputedStyle(node);
            if (r.width <= 1.5 && r.height <= 1.5) continue;
            if (cs.visibility === 'hidden' || Number(cs.opacity) === 0) continue;
            const l = Math.max(r.left, wr.left), rr = Math.min(r.right, wr.right), tt = Math.max(r.top, wr.top), bb = Math.min(r.bottom, wr.bottom);
            const spill = r.left < wr.left - 2 || r.right > wr.right + 2 || r.top < wr.top - 2 || r.bottom > wr.bottom + 2;
            if (rr - l <= 0 || bb - tt <= 0) { out.push({ node, k: 'spill', box: { l: r.left, r: r.right, t: r.top, b: r.bottom }, w: r.width, h: r.height, spill: true, label: node.getAttribute('class') || '' }); continue; }
            out.push({ spill, label: String(node.getAttribute('class') || '').split(/\s+/)[0], node, k: String(node.getAttribute('class') || '').split(/\s+/)[0] || node.tagName.toLowerCase(), box: { l, r: rr, t: tt, b: bb }, w: Math.round(r.width * 10) / 10, h: Math.round(r.height * 10) / 10 });
        }
        return out;
    }
    function measure(host) {
        const widgetEl = host.el, texts = [], fails = [], wr = widgetEl.getBoundingClientRect(), rel = b => '[' + [b.l - wr.left, b.t - wr.top, b.r - wr.left, b.b - wr.top].map(v => Math.round(v)).join(',') + ']';
        for (const [kind, sel] of TEXT) for (const node of widgetEl.querySelectorAll(sel)) { const info = textInfo(kind, node, widgetEl); if (info) texts.push(info); }
        const controls = controlsOf(widgetEl);
        for (const c of controls) if (c.spill && c.k !== 'hd-slider-thumb' && c.k !== 'hd-ct-thumb') fails.push('outside control ' + (c.label || c.k) + ' ' + rel(c.box));
        for (const x of texts) {
            if (x.kind === 'title' && x.truncated && x.shown < Math.min(x.total, 8)) fails.push('title: ' + x.shown + ' of ' + x.total + ' characters before the ellipsis');
            if ((x.kind === 'title' || x.kind === 'sub' || x.kind === 'state' || x.kind === 'numeral') && x.truncated && x.total > 2 && x.shown <= 2) fails.push('tiny ' + x.kind + ': ' + x.shown + ' of ' + x.total + ' characters');
            if (x.outside) fails.push('outside ' + x.kind + ' leaves the widget box');
            for (const c of controls) if (!c.node.contains(x.node) && !x.node.contains(c.node) && overlap(x.box, c.box, c.k === 'hd-spark' ? 3 : 0)) fails.push('overlap ' + x.kind + ' ' + rel(x.box) + ' with ' + c.k + ' ' + rel(c.box));
        }
        for (let i = 0; i < texts.length; i++) for (let j = i + 1; j < texts.length; j++) { const a = texts[i], b = texts[j]; if (!a.node.contains(b.node) && !b.node.contains(a.node) && overlap(a.box, b.box)) fails.push('overlap ' + a.kind + ' ' + rel(a.box) + ' with ' + b.kind + ' ' + rel(b.box)); }
        const warns = texts.filter(x => x.kind === 'state' && x.truncated && x.shown > 2).map(x => 'state truncated: ' + x.shown + ' of ' + x.total);
        const rnd = v => Math.round(v * 10) / 10;
        return { texts: texts.map(x => ({ kind: x.kind, text: x.text, total: x.total, shown: x.shown, truncated: x.truncated, w: rnd(x.box.r - x.box.l) })), controls: controls.map(c => ({ k: c.k, w: c.w, h: c.h })), fails, warns };
    }

    // ---- cases ----------------------------------------------------------------------------------------------------------------------
    function variantInfo(type, variant) {
        const c = catalog.variant(type, variant) || {}, k = catalog.constraints({ type, variant }), d = c.default_size || { w: 2, h: 1 }, m = c.min_size || { w: 1, h: 1 };
        return { def: { w: d.w, h: d.h }, min: { w: m.w, h: m.h }, max: { w: k.maxW, h: k.maxH } };
    }
    function sizesFor(info, cols, extended) {
        const cl = s => ({ w: clamp(s.w, 1, cols), h: s.h }), out = new Map(), add = (s, tag) => { const key = s.w + 'x' + s.h; if (!out.has(key)) out.set(key, { size: s, tags: [] }); out.get(key).tags.push(tag); };
        add(cl(info.def), 'default'); add(cl(info.min), 'min');
        if (extended) for (let w = info.min.w; w <= Math.min(info.max.w, cols, 6); w++) for (let h = info.min.h; h <= Math.min(info.max.h, 4); h++) add({ w, h }, 'ext');
        return Array.from(out.values());
    }
    const pxFor = (bp, size) => { const b = BPS[bp], colW = (b.width - 2 * GAP - (b.cols - 1) * GAP) / b.cols, rowH = clamp(colW, 56, 180);
        return { width: Math.round((size.w * colW + (size.w - 1) * GAP) * 100) / 100, height: Math.round((size.h * rowH + (size.h - 1) * GAP) * 100) / 100, colW: Math.round(colW * 100) / 100, rowH: Math.round(rowH * 100) / 100 }; };

    // render({family, palette, bp: 'lg'|'md'|'sm'|'thumb', extended}) rebuilds the sheet and returns the tile reports.
    fit.render = async function (o) {
        family = o.family || 'glass';
        for (const [k, v] of Object.entries({ 'data-hd-family': family, 'data-hd-palette': o.palette || 'dark', 'data-hd-scheme': o.palette === 'light' ? 'light' : 'dark', 'data-hd-motion': 'none' })) root.setAttribute(k, v);
        root.replaceChildren();
        const heading = el('h1', 'fit-title', 'widget fit: family=' + family + ' palette=' + (o.palette || 'dark') + ' page=' + o.bp + (o.bp === 'thumb' ? ' (palette thumb, 80 px cells)' : ' ' + BPS[o.bp].width + ' px'));
        const sheet = el('div', 'fit-tiles'), ext = el('div', 'fit-tiles fit-ext');
        root.appendChild(heading); root.appendChild(sheet); root.appendChild(el('h1', 'fit-title fit-ext-title', 'extended sizes (measured, not on the sheet image)')); root.appendChild(ext);
        const jobs = [], hosts = [];
        for (const type of E.widgets.types()) for (const variant of E.widgets.get(type).variants) {
            const info = variantInfo(type, variant);
            if (o.bp === 'thumb') { const size = E.samples.thumbSize(catalog, type, variant); jobs.push({ type, variant, size, px: E.samples.pxOf(size), state: 'on', tags: ['thumb'], mode: 'thumb' }); continue; }
            for (const s of sizesFor(info, BPS[o.bp].cols, o.extended)) for (const state of type === 'entity' ? ['on', 'switch', 'stale'] : ['on', 'stale']) { if (state === 'stale' && s.tags.includes('ext') && s.tags.length === 1) continue; jobs.push({ type, variant, size: s.size, px: pxFor(o.bp, s.size), state, tags: s.tags, mode: 'view' }); }
        }
        for (const j of jobs) {
            const onSheet = !j.tags.includes('ext') || j.tags.length > 1, tile = el('div', 'fit-tile'), label = el('div', 'fit-label'), stage = el('div', 'fit-stage');
            const lead = j.type + '/' + j.variant + (j.state === 'on' ? '' : ' (' + j.state + ')') + ' ' + j.size.w + 'x' + j.size.h + ' ' + j.tags.filter(x => x !== 'ext').join('+');
            label.textContent = lead + ' ' + Math.round(j.px.width) + 'x' + Math.round(j.px.height);
            stage.style.width = j.px.width + 'px'; stage.style.height = j.px.height + 'px';
            tile.appendChild(label); tile.appendChild(stage); (onSheet && j.state !== 'stale' ? sheet : ext).appendChild(tile);
            if (j.state === 'stale') tile.style.display = 'none';
            hosts.push({ j, tile, label, stage });
        }
        for (const h of hosts) { try { h.host = build(h.stage, h.j.type, h.j.variant, h.j.size, h.j.px, h.j.state, h.j.mode); } catch (err) { h.error = String((err && err.message) || err); } }
        await document.fonts.ready; await frames(3);
        // the stale twins are measured in an off-sheet strip so they never show on the image
        const strip = el('div', 'fit-tiles fit-ext'); strip.setAttribute('data-fit-strip', 'true'); root.appendChild(strip);
        for (const h of hosts) if (h.j.state === 'stale') { h.tile.style.display = ''; strip.appendChild(h.tile); }
        await frames(2);
        const tiles = [];
        for (const h of hosts) {
            const rep = { type: h.j.type, variant: h.j.variant, family, bp: o.bp, state: h.j.state, size: h.j.size, px: { width: h.j.px.width, height: h.j.px.height }, tags: h.j.tags, mode: h.j.mode, fails: [], warns: [], texts: [], controls: [] };
            if (h.error) rep.fails.push('render: ' + h.error); else Object.assign(rep, measure(h.host));
            h.tile.setAttribute('data-fit-fail', String(rep.fails.length > 0));
            if (rep.fails.length) h.label.textContent += ' | ' + rep.fails.map(f => f.split(':')[0].replace('overlap ', 'ov ')).join(', ');
            tiles.push(rep);
        }
        await frames(1);
        const r = sheet.getBoundingClientRect(), hr = heading.getBoundingClientRect();
        fit.sheetBox = { x: Math.max(0, hr.left), y: hr.top + window.scrollY, width: root.getBoundingClientRect().width, height: r.bottom + window.scrollY - (hr.top + window.scrollY) + 8 };
        return { tiles, sheet: fit.sheetBox };
    };

    // dump('type/variant/WxH'): every element of the first tile with that id: class, box relative to the widget, display, overflow and the text, for debugging.
    fit.dump = function (spec) {
        const [type, variant, dim, state] = spec.split('/'), want = dim, out = [];
        for (const tile of root.querySelectorAll('.fit-tile')) {
            const w = tile.querySelector('.hd-widget'); if (!w || w.getAttribute('data-hd-type') !== type || w.getAttribute('data-hd-variant') !== variant || w.getAttribute('data-hd-size') !== want) continue;
            if ((state === 'stale') !== /stale/.test(tile.querySelector('.fit-label').textContent)) continue;
            const wr = w.getBoundingClientRect();
            for (const n of w.querySelectorAll('*')) {
                if (n.closest('svg') && n.tagName.toLowerCase() !== 'svg') continue;
                const r = n.getBoundingClientRect(), cs = getComputedStyle(n), cls = n.className && n.className.baseVal === undefined ? n.className : '';
                out.push([n.tagName.toLowerCase() + '.' + String(cls).split(/\s+/).join('.'), [r.left - wr.left, r.top - wr.top, r.width, r.height].map(v => Math.round(v * 10) / 10).join(','), cs.display, cs.position, cs.overflowX, 'pie=' + cs.paddingInlineEnd, 'fs=' + cs.fontSize, (n.children.length ? '' : (n.textContent || '').slice(0, 30))].join(' | '));
            }
            break;
        }
        return out.join(String.fromCodePoint(10));
    };

    async function main() {
        E = window.HomeDashEngine;
        if (!E || !E.widgets || !E.samples || !E.catalog) throw new Error('the HomeDash engine did not load');
        root = document.getElementById('fit-root');
        const words = {};
        for (const l of ['en']) { const r = await fetch('/lang/homedash/' + l + '.json'); if (r.ok) Object.assign(words, await r.json()); }
        t = E.core.makeT(words, { warn() {} });
        fit.setCatalog = raw => { catalog = E.catalog.create(raw); };
        await document.fonts.ready;
        fit.ready = true;
    }
    main().catch(err => { fit.error = String((err && err.message) || err); });
})();
