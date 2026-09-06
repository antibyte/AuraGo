(function () {
    'use strict';

    const packName = (state, id) => state.context.t('game_maker.pack_' + id.replaceAll('-', '_'));
    function selectionText(state) {
        const ids = state.selectedAssetPackIDs || [];
        return ids.length ? state.context.t('game_maker.assets_selected') + ': ' + ids.map(id => packName(state, id)).join(', ')
            : state.context.t('game_maker.assets_automatic');
    }
    function updateSelection(state) {
        state.container.querySelectorAll('[data-gm-asset-selection]').forEach(node => { node.textContent = selectionText(state); });
    }
    function selectionMarkup(state) {
        return `<p class="gm-asset-selection" data-gm-asset-selection>${state.context.esc(selectionText(state))}</p>`;
    }
    function clearSelection(state) {
        state.selectedAssetPackIDs = [];
        updateSelection(state);
    }
    function show(state, helpers) {
        const { esc, t } = state.context;
        helpers.showModal(state, `<section class="gm-modal gm-assets-modal" role="dialog" aria-modal="true" aria-label="${esc(t('game_maker.assets'))}">
            <header><div><span>${esc(t('game_maker.assets_offline'))}</span><h2>${esc(t('game_maker.assets'))}</h2></div>
                <button type="button" data-modal-close aria-label="${esc(t('game_maker.close'))}">×</button></header>
            <div class="gm-assets-toolbar"><input type="search" data-asset-search placeholder="${esc(t('game_maker.assets_search'))}" aria-label="${esc(t('game_maker.assets_search'))}">
                <select data-asset-category aria-label="${esc(t('game_maker.assets_category'))}"><option value="">${esc(t('game_maker.assets_all'))}</option></select></div>
            <div class="gm-assets-workspace"><div class="gm-asset-cards" data-asset-cards role="list"></div>
                <div class="gm-asset-detail" data-asset-detail><p role="status">${esc(t('game_maker.loading'))}</p></div></div>
            ${selectionMarkup(state)}<footer><button type="button" data-clear-selection>${esc(t('game_maker.assets_auto_button'))}</button>
                <button type="button" data-modal-close>${esc(t('game_maker.close'))}</button></footer></section>`, layer => {
            const abort = new AbortController();
            let timer = null, detailID = 0, packs = [];
            const cleanup = () => {
                abort.abort(); clearInterval(timer);
                if (state.assetBrowserCleanup === cleanup) state.assetBrowserCleanup = null;
            };
            state.assetBrowserCleanup = cleanup;
            const current = () => !state.disposed && !abort.signal.aborted;
            const cards = layer.querySelector('[data-asset-cards]'), detail = layer.querySelector('[data-asset-detail]');
            const search = layer.querySelector('[data-asset-search]'), category = layer.querySelector('[data-asset-category]');
            const selected = () => state.selectedAssetPackIDs || [];
            function renderCards() {
                const query = search.value.trim().toLowerCase();
                const visible = packs.filter(pack => (!category.value || category.value === pack.id) &&
                    [packName(state, pack.id), pack.description, ...pack.tags].join(' ').toLowerCase().includes(query));
                cards.innerHTML = visible.map(pack => `<article class="gm-asset-card" role="listitem">
                    <button type="button" data-pack="${esc(pack.id)}"><img src="${esc(state.api.assetPackImageURL(pack.id))}" alt="" loading="lazy">
                        <strong>${esc(packName(state, pack.id))}</strong></button><label><input type="checkbox" aria-label="${esc(packName(state, pack.id) + ': ' + t('game_maker.assets_use_next'))}" data-select-pack="${esc(pack.id)}" ${selected().includes(pack.id) ? 'checked' : ''}>
                        ${esc(t('game_maker.assets_use_next'))}</label></article>`).join('') || `<p>${esc(t('game_maker.assets_no_results'))}</p>`;
            }
            async function showPack(id) {
                clearInterval(timer);
                const requestID = ++detailID;
                detail.innerHTML = `<p role="status">${esc(t('game_maker.loading'))}</p>`;
                try {
                    const pack = await state.api.assetPack(id, { signal: abort.signal });
                    if (!current() || detailID !== requestID) return;
                    detail.innerHTML = `<h3>${esc(packName(state, id))}</h3><p>${esc(pack.description)}</p>
                        <div class="gm-asset-stage is-checker" data-asset-stage><img class="gm-asset-sheet" src="${esc(state.api.assetPackImageURL(id))}" alt="${esc(packName(state, id))}">
                            <canvas width="256" height="256" data-asset-canvas aria-label="${esc(t('game_maker.assets_preview'))}"></canvas></div>
                        <label>${esc(t('game_maker.assets_background'))}<select data-asset-background>
                            <option value="checker">${esc(t('game_maker.assets_checker'))}</option><option value="white">${esc(t('game_maker.assets_white'))}</option>
                            <option value="dark">${esc(t('game_maker.assets_dark'))}</option></select></label>
                        ${(pack.assemblies || []).length ? `<label>${esc(t('game_maker.assets_assembly'))}<select data-asset-assembly><option value="">${esc(t('game_maker.assets_individual'))}</option>${pack.assemblies.map(a => `<option value="${esc(a.id)}">${esc(a.name)}</option>`).join('')}</select></label>` : ''}
                        <label>${esc(t('game_maker.assets_sprite'))}<select data-asset-sprite>${pack.assets.map(asset => `<option value="${esc(asset.id)}">${esc(asset.name)}</option>`).join('')}</select></label>
                        <p data-asset-description></p><label>${esc(t('game_maker.assets_animation'))}<select data-asset-animation><option value="">${esc(t('game_maker.assets_still'))}</option>
                            ${pack.animations.map(a => `<option value="${esc(a.id)}">${esc(a.id)}</option>`).join('')}</select></label>
                        <div class="gm-asset-playback"><button type="button" data-asset-play>${esc(t('game_maker.assets_play'))}</button>
                            <button type="button" data-asset-stop>${esc(t('game_maker.assets_pause'))}</button><span data-asset-frame></span></div>`;
                    const ctx = detail.querySelector('[data-asset-canvas]').getContext('2d');
                    ctx.imageSmoothingEnabled = false;
                    const image = detail.querySelector('.gm-asset-sheet'), spriteSelect = detail.querySelector('[data-asset-sprite]');
                    const animationSelect = detail.querySelector('[data-asset-animation]');
                    const assemblySelect = detail.querySelector('[data-asset-assembly]');
                    let frameIndex = 0;
                    function drawAssembly(elapsed = 0) {
                        const assembly = pack.assemblies?.find(a => a.id === assemblySelect?.value);
                        if (!assembly || !current() || detailID !== requestID || !image.complete || !image.naturalWidth) return;
                        ctx.clearRect(0, 0, 256, 256);
                        const scale = 256 / Math.max(assembly.width, assembly.height);
                        const left = (256 - assembly.width * scale) / 2, top = (256 - assembly.height * scale) / 2;
                        for (const part of assembly.parts) {
                            const animation = pack.animations.find(a => a.id === part.animation_id);
                            let index = part.frame;
                            if (animation) {
                                const sequence = animation.frames.slice();
                                if (animation.yoyo && sequence.length > 2) sequence.push(...sequence.slice(1, -1).reverse());
                                const step = Math.floor(elapsed * animation.frame_rate / 1000);
                                index = sequence[animation.repeat === -1 || step < sequence.length * (animation.repeat + 1)
                                    ? step % sequence.length : sequence.length - 1];
                            }
                            const frame = pack.frames[index];
                            ctx.drawImage(image, frame.x, frame.y, frame.w, frame.h,
                                left + part.x * scale, top + part.y * scale, frame.w * scale, frame.h * scale);
                        }
                        detail.querySelector('[data-asset-frame]').textContent = assembly.width + ' × ' + assembly.height;
                    }
                    function draw(index) {
                        if (assemblySelect?.value) { drawAssembly(); return; }
                        if (!current() || detailID !== requestID || !image.complete || !image.naturalWidth) return;
                        const frame = pack.frames[index];
                        ctx.clearRect(0, 0, 256, 256);
                        ctx.drawImage(image, frame.x, frame.y, frame.w, frame.h, 0, 0, 256, 256);
                        detail.querySelector('[data-asset-frame]').textContent = t('game_maker.assets_frame') + ' ' + index;
                    }
                    function chooseSprite() {
                        clearInterval(timer);
                        if (assemblySelect) assemblySelect.value = '';
                        const asset = pack.assets.find(a => a.id === spriteSelect.value);
                        frameIndex = asset.frames[0];
                        detail.querySelector('[data-asset-description]').textContent = asset.description;
                        animationSelect.value = pack.animations.find(a => a.asset_id === asset.id)?.id || '';
                        draw(frameIndex);
                    }
                    function play() {
                        clearInterval(timer);
                        if (assemblySelect?.value) {
                            drawAssembly();
                            const assembly = pack.assemblies.find(a => a.id === assemblySelect.value);
                            if (!assembly.parts.some(p => p.animation_id)) return;
                            const started = performance.now();
                            timer = setInterval(() => drawAssembly(performance.now() - started), 1000 / 30);
                            return;
                        }
                        const animation = pack.animations.find(a => a.id === animationSelect.value);
                        if (!animation) { draw(frameIndex); return; }
                        spriteSelect.value = animation.asset_id;
                        detail.querySelector('[data-asset-description]').textContent = pack.assets.find(a => a.id === animation.asset_id).description;
                        const sequence = animation.frames.slice();
                        if (animation.yoyo && sequence.length > 2) sequence.push(...sequence.slice(1, -1).reverse());
                        let position = 0, loops = 0;
                        draw(sequence[0]);
                        timer = setInterval(() => {
                            if (!current()) { clearInterval(timer); return; }
                            if (++position >= sequence.length) {
                                if (animation.repeat !== -1 && loops >= animation.repeat) { clearInterval(timer); return; }
                                position = 0; loops++;
                            }
                            draw(sequence[position]);
                        }, 1000 / animation.frame_rate);
                    }
                    image.addEventListener('load', () => draw(frameIndex), { once: true });
                    image.addEventListener('error', () => { if (current() && detailID === requestID) helpers.modalError(layer, t('game_maker.assets_load_failed')); }, { once: true });
                    detail.querySelector('[data-asset-background]').addEventListener('change', event => { detail.querySelector('[data-asset-stage]').className = 'gm-asset-stage is-' + event.target.value; });
                    spriteSelect.addEventListener('change', chooseSprite);
                    animationSelect.addEventListener('change', () => { if (assemblySelect) assemblySelect.value = ''; play(); });
                    assemblySelect?.addEventListener('change', () => {
                        clearInterval(timer);
                        if (!assemblySelect.value) { chooseSprite(); return; }
                        animationSelect.value = '';
                        detail.querySelector('[data-asset-description]').textContent = pack.assemblies.find(a => a.id === assemblySelect.value).description;
                        play();
                    });
                    detail.querySelector('[data-asset-play]').addEventListener('click', play);
                    detail.querySelector('[data-asset-stop]').addEventListener('click', () => clearInterval(timer));
                    chooseSprite();
                    if (assemblySelect) {
                        assemblySelect.value = pack.assemblies[0].id;
                        animationSelect.value = '';
                        detail.querySelector('[data-asset-description]').textContent = pack.assemblies[0].description;
                        drawAssembly();
                    }
                } catch (error) { if (current() && detailID === requestID) helpers.modalError(layer, error.message || t('game_maker.assets_load_failed')); }
            }
            cards.addEventListener('click', event => { const button = event.target.closest('[data-pack]'); if (button) showPack(button.dataset.pack); });
            cards.addEventListener('change', event => {
                const id = event.target.dataset.selectPack;
                if (!id || !packs.some(p => p.id === id)) return;
                state.selectedAssetPackIDs = event.target.checked ? [...new Set([...selected(), id])] : selected().filter(value => value !== id);
                updateSelection(state);
            });
            layer.querySelector('[data-clear-selection]').addEventListener('click', () => { clearSelection(state); renderCards(); });
            search.addEventListener('input', renderCards); category.addEventListener('change', renderCards);
            state.api.assetPacks({ signal: abort.signal }).then(body => {
                if (!current()) return;
                packs = body.packs;
                category.innerHTML += packs.map(p => `<option value="${esc(p.id)}">${esc(packName(state, p.id))}</option>`).join('');
                renderCards(); if (packs.length) showPack(packs[0].id);
            }).catch(error => { if (current()) helpers.modalError(layer, error.message || t('game_maker.assets_load_failed')); });
        });
    }
    window.GameMakerStudioAssets = { show, selectionMarkup, updateSelection, clearSelection };
})();
