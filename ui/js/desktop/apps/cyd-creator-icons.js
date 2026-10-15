// The icon picker uses the shell import seam; its previews use the same RGB565 pixels as firmware.
(function () {
    'use strict';
    const P = window.CYDCreatorParts = window.CYDCreatorParts || {};
    const ID = /^[A-Za-z0-9][A-Za-z0-9_-]{0,63}$/;
    const SIZES = [24, 32, 48, 64];

    function bytes(asset) { return asset.w * asset.h * (asset.encoding === 'a8' ? 1 : asset.alpha ? 3 : 2); }
    function origin(asset) {
        const m = /^1\/(playful|modern|comic|sober)\/(color|mono)\/([a-z_]+)\/(24|32|48|64)\/\d\.png$/.exec((asset || {}).icon || '');
        return m ? { style: m[1], tone: m[2], motif: m[3], size: Number(m[4]) } : null;
    }
    function node(asset, props, assetUrl, color, lazy) {
        const h = P.render.h;
        const p = props || {};
        const motion = ['pulse', 'bob', 'rotate', 'frames'].includes(p.icon_motion) ? p.icon_motion : 'pulse';
        const hasFrames = Array.isArray(p.icon_frames) && p.icon_frames.length === 6 && p.icon_frames.every(id => ID.test(id));
        const animate = !!p.icon_animate && (motion !== 'frames' || hasFrames);
        const frames = animate && motion === 'frames' ? p.icon_frames : [asset.id];
        const mono = asset.encoding === 'a8';
        const tint = /^#[0-9a-fA-F]{6}$/.test(p.icon_tint || '') ? p.icon_tint : color;
        const side = value => Math.max(1, Math.min(2048, Math.round(Number(value)) || 1));
        const style = { width: side(asset.w) + 'px', height: side(asset.h) + 'px', color: tint || 'currentColor' };
        return h('span', { class: 'cydc-icon-art' + (animate ? ' is-animated motion-' + motion : ''), style, 'aria-hidden': 'true' }, frames.map((id, index) => {
            if (!ID.test(id)) return null;
            const src = assetUrl(id);
            // CSS string escaping also covers a versioned URL with quotes or backslashes.
            const url = 'url("' + String(src).replace(/["\\\n\r]/g, c => '\\' + c.charCodeAt(0).toString(16) + ' ') + '")';
            const frameStyle = { '--cc-frame': index, '--cc-delay': (index / 6 - 1) + 's' };
            return mono ? h('span', { class: 'cydc-icon-frame', 'data-cc-mask': lazy ? url : null, style: Object.assign(frameStyle, { 'mask-image': lazy ? 'none' : url, '-webkit-mask-image': lazy ? 'none' : url, background: 'currentColor' }) }) :
                h('img', { class: 'cydc-icon-frame', style: frameStyle, src, alt: '', loading: 'lazy', draggable: 'false' });
        }));
    }

    async function open(d, widgetId, owner, overrides) {
        if (d.readonly() || !d.inst.iconCatalog || !d.inst.importIcon) return;
        const { h, dom, swap } = P.render;
        const iconText = (key, fallback, args) => d.t.or('cyd_creator.icons.' + key, fallback, args);
        const current = () => { const f = P.model.findWidget(d.project(), widgetId); return f && f.widget; };
        const w = current();
        if (!w) return;
        const key = w.type === 'button' ? 'icon_asset' : 'asset';
        const old = (d.project().assets || []).find(a => a.id === (w.props || {})[key]);
        const selection = Object.assign({ style: 'modern', tone: 'color', motif: 'home', size: Math.max(d.screenSize().w, d.screenSize().h) >= 800 ? 48 : 32 }, origin({ icon: (w.props || {}).icon_origin }) || origin(old) || {});
        selection.animated = !!(w.props || {}).icon_animate;
        Object.assign(selection, overrides || {});
        let catalog, category = '', query = '', disposed = false, pending = false, observer;
        const body = dom(h('div', { class: 'cydc-icon-picker' }, iconText('loading', 'Loading icons…')));
        const dialog = P.dialog.open(d.inst.root, {
            owner, title: iconText('title', 'Icon library'), body, className: 'cydc-icon-dialog',
            actions: [{ id: 'cancel', label: iconText('cancel', 'Cancel') }, { id: 'insert', label: iconText('insert', 'Use icon'), kind: 'primary' }],
            async onAction(action) {
                if (action !== 'insert') { dialog.close(); return; }
                if (!catalog || pending || d.readonly() || !current()) return;
                pending = true; dialog.setBusy(true); dialog.setError('');
                const chosen = Object.assign({}, selection);
                try {
                    const result = await d.inst.importIcon(chosen);
                    if (disposed || d.disposed || !current()) return;
                    if (d.readonly()) throw new Error(iconText('locked', 'The project is now read-only.'));
                    const set = { props: { [key]: result.asset.id, icon_animate: chosen.animated, icon_motion: result.motion,
                        icon_origin: '1/' + [chosen.style, chosen.tone, chosen.motif, chosen.size, '0.png'].join('/'),
                        icon_frames: result.frames.length === 6 ? result.frames : null } };
                    if (current().type === 'image') { set.w = chosen.size; set.h = chosen.size; }
                    d.commands.transact('icon', (draft, apply) => {
                        if (!Array.isArray(draft.assets)) draft.assets = [];
                        for (const a of result.assets) if (!draft.assets.some(old => old.id === a.id)) draft.assets.push(P.util.deepClone(a));
                        apply([{ op: 'update_widget', id: widgetId, set }]);
                    });
                    d.announce(iconText('inserted', 'Icon inserted')); dialog.close();
                } catch (err) {
                    if (!disposed) dialog.setError(err && err.code === 'CYD_ASSET_TOO_LARGE' ? iconText('budget_error', 'Not enough image memory. Choose a smaller size or a still image.') : P.errors.message(d.t, err));
                } finally { pending = false; if (!disposed) dialog.setBusy(false); }
            }
        });
        dialog.promise.then(() => { disposed = true; if (observer) observer.disconnect(); });
        const name = id => iconText('motif.' + id, id);
        const path = (id, frame) => {
            const url = '/img/cyd-icons/' + [selection.style, selection.tone, id, selection.size, frame + '.png'].join('/');
            return window.AuraLazyAssets && typeof window.AuraLazyAssets.versionedURL === 'function' ? window.AuraLazyAssets.versionedURL(url) : url;
        };
        const preview = (motif, active) => {
            const a = { id: motif.id, w: selection.size, h: selection.size, encoding: selection.tone === 'mono' ? 'a8' : '' };
            const frames = Array.from({ length: 6 }, (_, i) => 'f_' + i);
            const motion = selection.style === 'sober' && motif.motion !== 'frames' ? 'pulse' : motif.motion;
            return node(a, { icon_animate: active && selection.animated, icon_motion: motion, icon_frames: frames }, id => path(motif.id, id.startsWith('f_') ? Number(id.slice(2)) : 0), (d.theme().tokens || {}).text, !active);
        };
        function render() {
            if (disposed || !catalog) return;
            if (observer) observer.disconnect();
            const motif = catalog.motifs.find(m => m.id === selection.motif) || catalog.motifs[0];
            const sizeBytes = selection.size ** 2 * (selection.tone === 'mono' ? 1 : 3) * (selection.animated && motif.motion === 'frames' ? 6 : 1);
            const used = (d.project().assets || []).reduce((n, a) => n + bytes(a), 0);
            const budget = Number((d.board() || {}).flash_mb) >= 8 ? 4 * 1024 * 1024 : 256 * 1024;
            const select = (label, key, values, value, change) => h('label', { class: 'cydc-field' }, label, h('select', {
                'data-cc-focus': 'icons:' + key, 'data-cc-icon-filter': key, value,
                on: { change(e) { change(e.target.value); render(); } }
            }, values.map(([v, text]) => h('option', { value: v, selected: String(v) === String(value) }, text))));
            const visible = catalog.motifs.filter(m => (!category || m.category === category) && (name(m.id) + ' ' + m.id).toLocaleLowerCase().includes(query.toLocaleLowerCase()));
            swap(body, h('div', {},
                h('div', { class: 'cydc-icon-filters' },
                    h('label', { class: 'cydc-field' }, iconText('search', 'Search icons'), h('input', { type: 'search', value: query, autofocus: true, 'data-cc-focus': 'icons:search', on: { input(e) { query = e.target.value; render(); } } })),
                    select(iconText('style', 'Style'), 'style', catalog.styles.map(s => [s, iconText('style_' + s, s)]), selection.style, v => { selection.style = v; }),
                    select(iconText('tone', 'Colour'), 'tone', catalog.tones.map(s => [s, iconText('tone_' + s, s)]), selection.tone, v => { selection.tone = v; }),
                    select(iconText('animation', 'Animation'), 'animated', [['false', iconText('still', 'Still image')], ['true', iconText('animated', 'Animated')]], String(selection.animated), v => { selection.animated = v === 'true'; }),
                    select(iconText('size', 'Size'), 'size', SIZES.map(s => [s, s + ' px']), selection.size, v => { selection.size = Number(v); }),
                    select(iconText('category', 'Category'), 'category', [['', iconText('all', 'All')]].concat(Array.from(new Set(catalog.motifs.map(m => m.category))).map(c => [c, iconText('category_' + c, c)])), category, v => { category = v; })),
                h('div', { class: 'cydc-icon-detail' },
                    h('div', { class: 'cydc-icon-samples' }, h('span', { class: 'cydc-icon-sample is-light' }, preview(motif, true)), h('span', { class: 'cydc-icon-sample is-dark' }, preview(motif, true))),
                    h('div', {}, h('strong', {}, name(motif.id)), h('p', {}, iconText('pixels', '{size} × {size} device pixels · RGB565 / A8', { size: selection.size })),
                        h('p', {}, iconText('memory', '{icon} KiB · project {used} / {budget} KiB', { icon: (sizeBytes / 1024).toFixed(2), used: (used / 1024).toFixed(1), budget: budget / 1024 })),
                        h('small', {}, iconText('memory_hint', 'Shared images count once. The final size is checked on import.')))),
                h('div', { class: 'cydc-icon-grid', 'aria-label': iconText('title', 'Icon library') }, visible.map(m => h('button', {
                    type: 'button', class: 'cydc-icon-choice', 'aria-pressed': m.id === selection.motif ? 'true' : 'false', 'data-cc-icon': m.id, 'data-cc-focus': 'icon:' + m.id,
                    on: { click() { selection.motif = m.id; render(); } }
                }, preview(m, false), h('span', {}, name(m.id))))),
                visible.length ? null : h('p', { role: 'status' }, iconText('empty', 'No matching icons'))));
            const loadMask = el => { el.style.maskImage = el.dataset.ccMask; el.style.webkitMaskImage = el.dataset.ccMask; };
            if (window.IntersectionObserver) observer = new IntersectionObserver(entries => {
                for (const entry of entries) if (entry.isIntersecting) { loadMask(entry.target); observer.unobserve(entry.target); }
            }, { root: body.querySelector('.cydc-icon-grid'), rootMargin: '64px' });
            for (const el of body.querySelectorAll('[data-cc-mask]')) {
                if (observer) observer.observe(el); else loadMask(el);
            }
        }
        try { catalog = await d.inst.iconCatalog(); render(); }
        catch (err) { if (!disposed) { dialog.setError(P.errors.message(d.t, err)); body.replaceChildren(); } }
    }
    P.iconLibrary = { open, node, bytes, origin };
})();
