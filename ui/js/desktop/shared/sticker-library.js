(function () {
    'use strict';
    const items = window.AuraStickerCatalog;
    const byId = new Map(items.map(item => [item.id, item]));
    const categories = [...new Set(items.map(item => item.category))];
    const styles = [...new Set(items.map(item => item.style))];
    function url(id, thumbnail) {
        if (!byId.has(id)) throw new Error('unknown_sticker');
        const path = byId.get(id)[thumbnail ? 'thumbnail' : 'image'];
        return window.AuraLazyAssets ? window.AuraLazyAssets.versionedURL(path) : path;
    }
    async function file(id, signal) {
        const response = await fetch(url(id), { credentials: 'same-origin', signal });
        if (!response.ok) throw new Error('sticker_load_failed');
        const blob = await response.blob();
        if (signal) signal.throwIfAborted();
        return new File([blob], 'sticker-' + id + '.png', { type: 'image/png' });
    }
    async function image(id, signal) {
        const objectURL = URL.createObjectURL(await file(id, signal));
        try {
            const img = new Image();
            img.src = objectURL;
            await img.decode();
            if (signal) signal.throwIfAborted();
            if (img.naturalWidth !== 1024 || img.naturalHeight !== 1024) throw new Error('sticker_size');
            return img;
        } finally { URL.revokeObjectURL(objectURL); }
    }
    function mount(host, options) {
        const abort = new AbortController();
        const tr = key => options.t('stickers.' + key);
        let active = false, disabled = false, busy = false, drawn = false;
        host.classList.add('sticker-picker');
        const controls = document.createElement('div'); controls.className = 'sticker-picker-controls';
        const search = document.createElement('input'); search.type = 'search'; search.placeholder = tr('search'); search.setAttribute('aria-label', tr('search'));
        const select = (values, key) => {
            const node = document.createElement('select'); node.setAttribute('aria-label', tr(key));
            node.add(new Option(tr(key), ''));
            values.forEach(value => node.add(new Option(tr(key + '.' + value), value)));
            return node;
        };
        const category = select(categories, 'category'), style = select(styles, 'style');
        controls.append(search, category, style);
        const status = document.createElement('p'); status.className = 'sticker-picker-status'; status.setAttribute('role', 'status');
        const grid = document.createElement('div'); grid.className = 'sticker-picker-grid';
        host.replaceChildren(controls, status, grid);
        function buttons() { grid.querySelectorAll('button').forEach(button => { button.disabled = disabled || busy; }); }
        function render() {
            if (!active) { drawn = false; return; }
            const query = search.value.trim().toLocaleLowerCase();
            const visible = items.filter(item => (!category.value || item.category === category.value) && (!style.value || item.style === style.value) && (!query || [tr('name.' + item.id), tr('category.' + item.category), tr('style.' + item.style)].join(' ').toLocaleLowerCase().includes(query)));
            grid.replaceChildren();
            for (const item of visible) {
                const button = document.createElement('button'); button.type = 'button'; button.dataset.stickerId = item.id;
                const name = tr('name.' + item.id); button.title = name + ' · ' + tr('style.' + item.style); button.setAttribute('aria-label', button.title);
                const img = document.createElement('img'); img.width = 192; img.height = 192; img.alt = ''; img.loading = 'lazy'; img.decoding = 'async'; img.src = url(item.id, true);
                const label = document.createElement('span'); label.textContent = name;
                button.append(img, label); grid.append(button);
            }
            status.textContent = busy ? tr('loading') : visible.length ? tr('count').replace('{{count}}', String(visible.length)) : tr('empty');
            buttons(); drawn = true;
        }
        for (const control of [search, category, style]) control.addEventListener('input', render, { signal: abort.signal });
        grid.addEventListener('error', event => {
            const img = event.target, button = img.closest('[data-sticker-id]');
            if (!button) return;
            button.classList.add('is-unavailable'); img.hidden = true;
            button.title = tr('name.' + button.dataset.stickerId) + ' · ' + tr('failed');
            button.setAttribute('aria-label', button.title);
        }, { capture: true, signal: abort.signal });
        grid.addEventListener('click', async event => {
            const button = event.target.closest('[data-sticker-id]');
            if (!button || !grid.contains(button) || disabled || busy) return;
            busy = true; buttons(); status.textContent = tr('loading'); grid.setAttribute('aria-busy', 'true');
            try { await options.onSelect(button.dataset.stickerId); if (!abort.signal.aborted) status.textContent = ''; }
            catch (error) { if (!abort.signal.aborted && error.name !== 'AbortError') status.textContent = tr('failed'); }
            finally { busy = false; if (!abort.signal.aborted) { buttons(); grid.removeAttribute('aria-busy'); } }
        }, { signal: abort.signal });
        return {
            setActive(value) { active = !!value; if (active && !drawn) render(); },
            setDisabled(value) { disabled = !!value; buttons(); },
            dispose() { abort.abort(); host.replaceChildren(); }
        };
    }
    window.AuraStickerLibrary = Object.freeze({ items, byId, categories, styles, url, file, image, mount });
})();
