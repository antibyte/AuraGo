// Infra canvas: the pan/zoom world of the Map and Topology views. Keyed
// patching of region, card, host-stack and connection elements, the one scene
// transform, the minimap and the overlays (empty state, no match, unsaved chip,
// tools). It owns no gestures: infra-gestures.js drives it through moveTemp,
// the highlight setters and createGhost.
//
// Wiring contract for infra.js:
//  - call render() on snapshot, layout, nodes, edges, expanded, selection,
//    layout-status and filters events, render({force: true}) on view, caps and
//    manifest events, and applyTool() on tool events;
//  - this module handles its own [data-tool], zoom-in/out/reset, fit and
//    toggle-minimap clicks; infra.js must not act on the zoom, fit and
//    minimap actions, and handles import-empty, add-empty, clear-search and
//    retry-layout itself;
//  - app.setPref('minimap', bool) is called after the minimap toggle changes,
//    app.prefs.minimap === false starts it hidden.
(function () {
    'use strict';

    const STATUS_GLYPH = { current: '●', stale: '◐', unknown: '○', removed: '✕', manual: '' };
    const CONNECT_HANDLES = ['n', 'e', 's', 'w'];
    const RESIZE_HANDLES = ['nw', 'n', 'ne', 'e', 'se', 's', 'sw', 'w'];
    const MAX_CHIPS = 8;
    const MM = { w: 200, h: 130 };
    const px = value => `${value}px`;
    const round1 = value => Math.round(value * 10) / 10;
    const emptyScene = () => ({ cards: [], cardById: new Map(), regions: [], regionRects: new Map(), regionDepth: new Map(), lines: [], bounds: null });
    // frame coalesces many requests into one requestAnimationFrame callback.
    const frame = fn => {
        let id = 0;
        return {
            request() { if (!id) id = requestAnimationFrame(() => { id = 0; fn(); }); },
            cancel() { if (id) cancelAnimationFrame(id); id = 0; },
        };
    };

    function create(app) {
        const { model, store, t, esc } = app;
        const S = app.safeId;
        const holder = document.createElement('div');
        holder.innerHTML = `<section class="infra-canvas" data-canvas aria-label="${esc(t('desktop.infra_map_label'))}">
  <div class="infra-viewport" data-stage tabindex="0" role="application" data-tool="select" aria-roledescription="${esc(t('desktop.infra_map_role'))}" aria-label="${esc(t('desktop.infra_map_label'))}" aria-describedby="infra-canvas-help-${S}">
    <div class="infra-canvas-grid" aria-hidden="true"></div>
    <div class="infra-scene" data-scene>
      <div class="infra-regions" data-regions></div>
      <svg class="infra-edges" data-edges width="1" height="1" viewBox="0 0 1 1" overflow="visible" aria-hidden="true" focusable="false"><g data-edge-layer></g><path class="infra-edge-preview" data-edge-preview d=""></path></svg>
      <div class="infra-nodes" data-nodes></div>
    </div>
    <div class="infra-marquee" data-marquee hidden></div>
  </div>
  <div class="infra-canvas-empty" data-empty hidden>
    <img src="${esc(model.assetURL('empty-atlas'))}" alt="" draggable="false">
    <h2>${esc(t('desktop.infra_empty_title'))}</h2>
    <ol><li>${esc(t('desktop.infra_empty_step_sources'))}</li><li>${esc(t('desktop.infra_empty_step_import'))}</li><li>${esc(t('desktop.infra_empty_step_drag'))}</li></ol>
    <div class="infra-empty-actions">
      <button type="button" class="infra-button infra-button-primary" data-action="import-empty" disabled>${esc(t('desktop.infra_import_discoveries'))}</button>
      <button type="button" class="infra-button" data-action="add-empty" disabled>${esc(t('desktop.infra_new_asset'))}</button>
    </div>
  </div>
  <div class="infra-canvas-nomatch" data-no-match hidden><strong>${esc(t('desktop.infra_no_match'))}</strong>
    <button type="button" class="infra-button" data-action="clear-search">${esc(t('desktop.infra_clear_search'))}</button></div>
  <div class="infra-canvas-chip" data-layout-chip role="status" hidden>
    <span>${esc(t('desktop.infra_layout_unsaved'))}</span><button type="button" class="infra-button" data-action="retry-layout" disabled>${esc(t('desktop.infra_retry'))}</button></div>
  <div class="infra-tools" role="toolbar" aria-label="${esc(t('desktop.infra_tools'))}" data-tools>
    <button type="button" data-tool="select" aria-pressed="true" tabindex="0" aria-label="${esc(t('desktop.infra_tool_select'))}" title="${esc(t('desktop.infra_tool_select'))}">${model.glyph('select')}</button>
    <button type="button" data-tool="hand" aria-pressed="false" tabindex="-1" aria-label="${esc(t('desktop.infra_tool_hand'))}" title="${esc(t('desktop.infra_tool_hand'))}">${model.glyph('hand')}</button>
    <button type="button" data-tool="connect" aria-pressed="false" tabindex="-1" aria-label="${esc(t('desktop.infra_tool_connect'))}" title="${esc(t('desktop.infra_tool_connect'))}">${model.glyph('connect')}</button>
    <span class="infra-tools-sep" aria-hidden="true"></span>
    <button type="button" data-action="zoom-out" tabindex="-1" aria-label="${esc(t('desktop.infra_zoom_out'))}" title="${esc(t('desktop.infra_zoom_out'))}">${model.glyph('minus')}</button>
    <button type="button" class="infra-zoom-value" data-action="zoom-reset" tabindex="-1" title="${esc(t('desktop.infra_zoom_reset'))}"><span data-zoom>100%</span></button>
    <button type="button" data-action="zoom-in" tabindex="-1" aria-label="${esc(t('desktop.infra_zoom_in'))}" title="${esc(t('desktop.infra_zoom_in'))}">${model.glyph('plus')}</button>
    <button type="button" data-action="fit" tabindex="-1" aria-label="${esc(t('desktop.infra_zoom_fit'))}" title="${esc(t('desktop.infra_zoom_fit'))}">${model.glyph('fit')}</button>
  </div>
  <figure class="infra-minimap" data-minimap>
    <figcaption class="infra-minimap-caption"><span data-visible-count></span>
      <button type="button" class="infra-cmd-icon" data-action="toggle-minimap" aria-pressed="true" aria-label="${esc(t('desktop.infra_minimap'))}" title="${esc(t('desktop.infra_minimap'))}">${model.glyph('minimap')}</button></figcaption>
    <svg class="infra-minimap-map" data-minimap-map viewBox="0 0 ${MM.w} ${MM.h}" role="img" aria-label="${esc(t('desktop.infra_minimap'))}"><g data-minimap-regions></g><g data-minimap-cards></g><rect class="infra-minimap-view" data-minimap-view rx="2"></rect></svg>
  </figure>
  <p class="infra-sr-only" id="infra-canvas-help-${S}">${esc(t('desktop.infra_map_keys'))}</p>
</section>`;
        const el = holder.firstElementChild;
        const q = selector => el.querySelector(selector);
        const viewport = q('[data-stage]'), scene = q('[data-scene]'), gridEl = q('.infra-canvas-grid');
        const regionsEl = q('[data-regions]'), nodesEl = q('[data-nodes]'), edgesSvg = q('[data-edges]');
        const edgeLayer = q('[data-edge-layer]'), edgePreview = q('[data-edge-preview]'), marquee = q('[data-marquee]');
        const emptyEl = q('[data-empty]'), noMatchEl = q('[data-no-match]'), chipEl = q('[data-layout-chip]');
        const toolsEl = q('[data-tools]'), zoomText = q('[data-zoom]'), zoomButton = q('[data-action="zoom-reset"]');
        const minimapEl = q('[data-minimap]'), minimapSvg = q('[data-minimap-map]'), minimapView = q('[data-minimap-view]');
        const minimapRegions = q('[data-minimap-regions]'), minimapCards = q('[data-minimap-cards]');
        const visibleCount = q('[data-visible-count]'), minimapToggle = q('[data-action="toggle-minimap"]');
        const retryButton = q('[data-action="retry-layout"]');

        const views = { atlas: null, topology: null }, initialized = { atlas: false, topology: false };
        // viewSize is the viewport size each view was last placed for: a resize keeps the world point at the
        // centre of that size, even when the view was set between the layout change and the observer callback.
        const viewSize = { atlas: null, topology: null };
        const cardEls = new Map(), regionEls = new Map(), lineEls = new Map();
        const temp = { cards: new Set(), regions: new Set(), lines: new Set() };
        const ghosts = new Set();
        let last = emptyScene(), linesByEnd = new Map(), lineGeom = new Map();
        let forceNext = false, mapDirty = false, revision = 0, drawnRevision = -1;
        let pctShown = '', rovingId = '', dropTargetId = '', connectTargetId = '', lastSize = null, animTimer = 0, persistTimer = 0, disposed = false;
        const persistKeys = new Set();
        let mmTransform = null, mmFrozen = null, mmDragging = false, mmSelected = new Set();
        const mmCardEls = new Map();

        const curKey = () => app.mapView();
        const getView = () => views[curKey()] || { x: 0, y: 0, zoom: 1 };
        const size = () => ({ w: viewport.clientWidth, h: viewport.clientHeight });
        const safeOf = id => String(id).replace(/[^a-zA-Z0-9_-]/g, '');
        const cardHalf = { w: model.CARD_W / 2, h: model.CARD_H / 2 };
        const minimapOn = () => minimapToggle.getAttribute('aria-pressed') !== 'false';
        const kindLabel = kind => app.labelForKind(kind);
        const cardMeta = node => node.profile?.ip_address || node.profile?.role || kindLabel(node.kind);
        const powerOf = node => (node && model.powerLabel(node.power) ? node.power : '');
        const guestNodesOf = card => card.guests.map(id => store.nodeById(id)).filter(Boolean);
        const setFlag = (entry, key, value, apply) => { if (entry.flags[key] !== value) { entry.flags[key] = value; apply(value); } };

        // ---- view transform ----------------------------------------------------
        const applyFrame = frame(paintView);
        const minimapFrame = frame(drawMinimap);
        const renderFrame = frame(() => renderNow());

        function paintView() {
            if (disposed) return;
            const v = getView();
            scene.style.transform = `translate(${v.x}px, ${v.y}px) scale(${v.zoom})`;
            gridEl.style.backgroundSize = `${model.GRID * v.zoom}px ${model.GRID * v.zoom}px`;
            gridEl.style.backgroundPosition = `${v.x}px ${v.y}px`;
            viewport.classList.toggle('is-zoom-low', v.zoom < 0.75);
            const pct = `${Math.round(v.zoom * 100)}%`;
            if (pctShown !== pct) {
                pctShown = pct;
                zoomText.textContent = pct;
                zoomButton.setAttribute('aria-label', `${t('desktop.infra_zoom_reset')} · ${pct}`);
            }
            drawMinimap();
        }
        function persistNow() {
            clearTimeout(persistTimer); persistTimer = 0;
            const sz = size();
            if (sz.w > 0 && sz.h > 0) {
                for (const key of persistKeys) {
                    if (!views[key]) continue;
                    try { localStorage.setItem(`infra.viewport.v1.${key}`, JSON.stringify(model.viewCenter(views[key], sz))); } catch (_) { /* storage may be unavailable */ }
                }
            }
            persistKeys.clear();
        }
        function persistSoon() { persistKeys.add(curKey()); clearTimeout(persistTimer); persistTimer = setTimeout(persistNow, 400); }
        function setView(next, { animate = false } = {}) {
            const finite = value => (Number.isFinite(value) ? value : 0);
            views[curKey()] = { x: finite(next?.x), y: finite(next?.y), zoom: model.clampZoom(next?.zoom) };
            const placedFor = size();
            if (placedFor.w > 0 && placedFor.h > 0) viewSize[curKey()] = placedFor;
            if (animate && app.animationsEnabled()) {
                scene.classList.add('is-animating');
                clearTimeout(animTimer);
                animTimer = setTimeout(() => scene.classList.remove('is-animating'), 220);
            } else if (scene.classList.contains('is-animating')) { scene.classList.remove('is-animating'); clearTimeout(animTimer); }
            applyFrame.request(); persistSoon();
        }
        function visibleHits(view, sz) {
            const vis = { x: -view.x / view.zoom, y: -view.y / view.zoom, w: sz.w / view.zoom, h: sz.h / view.zoom };
            return last.cards.some(card => model.intersects(model.cardRect(card), vis)) || [...last.regionRects.values()].some(rect => model.intersects(rect, vis));
        }
        // ensureInitialView runs once per view: restore the stored viewport when it
        // still shows something, else fit everything. It waits for the first
        // snapshot and a laid-out viewport.
        function ensureInitialView() {
            const key = curKey(), sz = size();
            if (initialized[key] || sz.w <= 0 || sz.h <= 0 || !store.state.refreshAt) return;
            initialized[key] = true;
            let stored = null;
            try { stored = model.viewFromCenter(JSON.parse(localStorage.getItem(`infra.viewport.v1.${key}`) || 'null'), sz); } catch (_) { stored = null; }
            if (stored && visibleHits(stored, sz)) setView(stored);
            else setView(model.fitView(last.bounds, sz, 48, 1));
        }
        function zoomBy(factor, point) {
            const v = getView(), sz = size();
            setView(model.zoomAt(v, v.zoom * factor, point || { x: sz.w / 2, y: sz.h / 2 }));
        }
        function zoomTo(zoom) { zoomBy(model.clampZoom(zoom) / getView().zoom); }
        function fit({ animate = false } = {}) {
            if (!last.bounds) renderNow();
            setView(model.fitView(last.bounds, size(), 48, 1), { animate });
        }
        // worldRect is the rectangle of a card, a region or (via its host) a guest.
        function worldRect(id) {
            const card = last.cardById.get(id) || last.cardById.get(store.guests().hostOf.get(id));
            if (card) return model.cardRect(card);
            return last.regionRects.get(id) || null;
        }
        function centerOn(id, { animate = false } = {}) {
            const rect = worldRect(id);
            if (!rect) return;
            const v = getView(), sz = size(), zoom = Math.max(v.zoom, 0.8);
            setView({ x: sz.w / 2 - (rect.x + rect.w / 2) * zoom, y: sz.h / 2 - (rect.y + rect.h / 2) * zoom, zoom }, { animate });
        }
        function ensureVisible(id) {
            const rect = worldRect(id);
            if (!rect) return;
            const v = getView(), sz = size();
            const a = model.toScreen(v, { x: rect.x, y: rect.y }), b = model.toScreen(v, { x: rect.x + rect.w, y: rect.y + rect.h });
            if (a.x < 0 || a.y < 0 || b.x > sz.w || b.y > sz.h) centerOn(id, { animate: true });
        }
        function clientToWorld(clientX, clientY) {
            const rect = viewport.getBoundingClientRect();
            return model.toWorld(getView(), { x: clientX - rect.left, y: clientY - rect.top });
        }

        // ---- cards and host stacks ----------------------------------------------
        function cardSig(node, card, guestList) {
            const guestsSig = guestList.map(guest => `${guest.id}:${guest.name}:${guest.power || ''}`).join(',');
            return [node.name, node.kind, model.iconURL(node.icon), model.sourceState(node), node.power || '', cardMeta(node), guestsSig, store.state.expandedHosts.has(node.id), store.canWrite()].join('\x1f');
        }
        function cardHTML(node, guestList) {
            const id = esc(node.id), name = esc(node.name || node.id), safe = safeOf(node.id);
            const state = model.sourceState(node), power = powerOf(node), expanded = store.state.expandedHosts.has(node.id);
            const label = [node.name || node.id, kindLabel(node.kind), app.statusLabel(state), power ? t(model.powerLabel(power)) : '',
                guestList.length ? model.countText(guestList.length, t('desktop.infra_guest_count_one'), t('desktop.infra_guest_count')) : ''].filter(Boolean).join(', ');
            const handles = store.canWrite() ? CONNECT_HANDLES.map(side => `<span class="infra-card-handle" data-connect-handle="${side}" aria-hidden="true"></span>`).join('') : '';
            let html = `<button type="button" class="infra-card" data-node="${id}" data-kind="${esc(node.kind)}" data-state="${esc(state)}"${power ? ` data-power="${power}"` : ''} tabindex="-1" aria-pressed="false" aria-label="${esc(label)}" title="${name}">`
                // The status glyph and the power dot ride on the icon, so the name keeps the whole text column.
                + `<span class="infra-card-icon">${app.icon(node.icon)}${STATUS_GLYPH[state] ? `<span class="infra-card-status" aria-hidden="true">${STATUS_GLYPH[state]}</span>` : ''}${power ? `<i class="infra-card-power" data-power="${power}" aria-hidden="true"></i>` : ''}</span>`
                + `<span class="infra-card-text"><strong class="infra-card-name">${name}</strong><small class="infra-card-meta">${esc(cardMeta(node))}</small></span>`
                + handles + '</button>';
            if (!guestList.length) return html;
            const running = guestList.filter(guest => guest.power === 'running').length;
            const toggleLabel = expanded ? t('desktop.infra_hide_guests') : t('desktop.infra_show_guests');
            const sorted = guestList.slice().sort((a, b) => String(a.name).localeCompare(String(b.name)));
            // "3 guests · 2 running", "1 guest · stopped"; the tooltip names the first five guests.
            const summary = guestList.length === 1
                ? (running ? t('desktop.infra_guests_badge_one') : t('desktop.infra_guests_badge_stopped_one'))
                : (running ? t('desktop.infra_guests_badge') : t('desktop.infra_guests_badge_stopped'));
            const names = sorted.slice(0, 5).map(guest => guest.name || guest.id).join(', ') + (sorted.length > 5 ? ` ${model.fill(t('desktop.infra_more_guests'), { count: sorted.length - 5 })}` : '');
            html += `<button type="button" class="infra-card-badge" data-toggle-host-services="${id}" tabindex="-1" aria-expanded="${expanded}" aria-controls="infra-guests-${S}-${safe}" aria-label="${esc(model.fill(toggleLabel, { name: node.name || node.id }))}" title="${esc(model.fill(t('desktop.infra_guests_badge_title'), { names }))}">`
                + `<span aria-hidden="true">▣</span> ${esc(model.fill(summary, { count: guestList.length, running }))}</button>`;
            const chips = sorted.slice(0, MAX_CHIPS).map(guest => `<button type="button" class="infra-guest-chip" data-hosted-service="${esc(guest.id)}" data-power="${esc(powerOf(guest))}" tabindex="-1" title="${esc(guest.name)}"><i aria-hidden="true"></i><span>${esc(guest.name)}</span></button>`).join('');
            const more = sorted.length > MAX_CHIPS ? `<button type="button" class="infra-guest-more" data-guest-more="${id}" tabindex="-1">${esc(model.fill(t('desktop.infra_more_guests'), { count: sorted.length - MAX_CHIPS }))}</button>` : '';
            return `${html}<div class="infra-guest-shelf" id="infra-guests-${S}-${safe}" data-guest-shelf="${id}" role="group" aria-label="${esc(model.fill(t('desktop.infra_guests_of'), { name: node.name || node.id }))}"${expanded ? '' : ' hidden'}>${chips}${more}</div>`;
        }
        // partsOf indexes the top-level elements of parsed card markup by node id.
        function partsOf(children, into) {
            const slot = id => { if (!into.has(id)) into.set(id, {}); return into.get(id); };
            for (const part of children) {
                if (part.dataset.node !== undefined) slot(part.dataset.node).card = part;
                else if (part.dataset.toggleHostServices !== undefined) slot(part.dataset.toggleHostServices).badge = part;
                else if (part.dataset.guestShelf !== undefined) slot(part.dataset.guestShelf).shelf = part;
            }
            return into;
        }
        function writeCard(entry, x, y) {
            const left = px(x), top = px(y);
            for (const part of [entry.card, entry.badge, entry.shelf]) if (part) { part.style.left = left; part.style.top = top; }
        }
        function placeCard(entry, x, y) {
            if (entry.x === x && entry.y === y) return;
            writeCard(entry, x, y);
            entry.x = x; entry.y = y; mapDirty = true;
        }
        function swapPart(entry, key, next, anchor) {
            const old = entry[key];
            if (old && next) old.replaceWith(next);
            else if (old) old.remove();
            else if (next) anchor.after(next);
            entry[key] = next || null;
        }
        function replaceCard(entry, card, node, guestList, sig) {
            const holderEl = document.createElement('template');
            holderEl.innerHTML = cardHTML(node, guestList);
            const parts = partsOf(holderEl.content.children, new Map()).get(node.id);
            const focused = document.activeElement === entry.card;
            parts.card.tabIndex = entry.card.tabIndex;
            entry.card.replaceWith(parts.card);
            entry.card = parts.card;
            swapPart(entry, 'badge', parts.badge, entry.card);
            swapPart(entry, 'shelf', parts.shelf, entry.badge || entry.card);
            Object.assign(entry, { sig, x: null, y: null, flags: {} });
            placeCard(entry, card.x, card.y);
            if (focused) entry.card.focus({ preventScroll: true });
        }
        function patchCards(cards, force) {
            const fresh = [], seen = new Set();
            for (const card of cards) {
                const node = store.nodeById(card.id);
                if (!node) continue;
                seen.add(card.id);
                const guestList = guestNodesOf(card), sig = cardSig(node, card, guestList), entry = cardEls.get(card.id);
                if (!entry) fresh.push({ card, node, guestList, sig });
                else if (force || entry.sig !== sig) replaceCard(entry, card, node, guestList, sig);
                else placeCard(entry, card.x, card.y);
            }
            for (const [id, entry] of cardEls) {
                if (seen.has(id)) continue;
                entry.card.remove(); entry.badge?.remove(); entry.shelf?.remove();
                cardEls.delete(id); mapDirty = true;
                if (id === rovingId) setRovingCard('');
            }
            if (!fresh.length) return;
            const batch = document.createElement('template');
            batch.innerHTML = fresh.map(item => cardHTML(item.node, item.guestList)).join('');
            const parts = partsOf(batch.content.children, new Map());
            for (const item of fresh) {
                const entry = { ...parts.get(item.card.id), sig: item.sig, x: null, y: null, flags: {} };
                if (item.card.id === rovingId) entry.card.tabIndex = 0;
                cardEls.set(item.card.id, entry);
                placeCard(entry, item.card.x, item.card.y);
            }
            nodesEl.append(batch.content);
        }

        // ---- regions --------------------------------------------------------------
        function regionHTML(node, depth) {
            const id = esc(node.id), count = store.childrenOf(node.id).length;
            const label = model.countText(count, t('desktop.infra_region_label_one'), t('desktop.infra_region_label'), { name: node.name || node.id, kind: kindLabel(node.kind) });
            const handles = store.canWrite() && store.features.layoutParent ? RESIZE_HANDLES.map(side => `<span class="infra-region-handle" data-resize="${side}" aria-hidden="true"></span>`).join('') : '';
            return `<div class="infra-region" data-region="${id}" data-kind="${esc(node.kind)}" data-depth="${depth}">`
                + `<button type="button" class="infra-region-header" data-region-header="${id}" tabindex="-1" aria-pressed="false" aria-label="${esc(label)}">`
                + `<img class="infra-region-icon" src="${esc(model.iconURL(node.icon))}" alt="" draggable="false"><span class="infra-region-name">${esc(node.name || node.id)}</span><span class="infra-region-count" aria-hidden="true">${count}</span></button>`
                + `<span class="infra-region-drop" data-region-drop aria-hidden="true"></span>${handles}</div>`;
        }
        function regionSig(node, depth) {
            return [node.name, model.iconURL(node.icon), node.kind, store.childrenOf(node.id).length, store.canWrite() && store.features.layoutParent, depth].join('\x1f');
        }
        function writeRegion(entry, rect) {
            const s = entry.el.style;
            s.left = px(rect.x); s.top = px(rect.y); s.width = px(rect.w); s.height = px(rect.h);
        }
        function placeRegion(entry, rect) {
            const r = entry.rect;
            if (r && r.x === rect.x && r.y === rect.y && r.w === rect.w && r.h === rect.h) return;
            writeRegion(entry, rect);
            entry.rect = { ...rect }; mapDirty = true;
        }
        function indexRegion(entry, el) { entry.el = el; entry.header = el.querySelector('[data-region-header]'); entry.drop = el.querySelector('[data-region-drop]'); return entry; }
        function patchRegions(regions, force) {
            const seen = new Set(), fresh = [];
            for (const region of regions) {
                const node = store.nodeById(region.id);
                if (!node) continue;
                seen.add(region.id);
                const sig = regionSig(node, region.depth), entry = regionEls.get(region.id);
                if (!entry) { fresh.push({ region, node, sig }); continue; }
                if (force || entry.sig !== sig) {
                    const tpl = document.createElement('template');
                    tpl.innerHTML = regionHTML(node, region.depth);
                    const focused = document.activeElement === entry.header, roving = entry.header.tabIndex;
                    const next = tpl.content.firstElementChild;
                    entry.el.replaceWith(next);
                    indexRegion(entry, next);
                    entry.header.tabIndex = roving;
                    Object.assign(entry, { sig, rect: null, flags: {} });
                    if (dropTargetId === region.id) applyDropTarget(region.id);
                    if (focused) entry.header.focus({ preventScroll: true });
                }
                placeRegion(entry, region.rect);
            }
            for (const [id, entry] of regionEls) {
                if (seen.has(id)) continue;
                entry.el.remove(); regionEls.delete(id); mapDirty = true;
                if (id === rovingId) setRovingCard('');
            }
            if (fresh.length) {
                const batch = document.createElement('template');
                batch.innerHTML = fresh.map(item => regionHTML(item.node, item.region.depth)).join('');
                const els = [...batch.content.children];
                fresh.forEach((item, i) => {
                    const entry = indexRegion({ sig: item.sig, rect: null, flags: {} }, els[i]);
                    if (item.region.id === rovingId) entry.header.tabIndex = 0;
                    regionEls.set(item.region.id, entry);
                    placeRegion(entry, item.region.rect);
                });
                regionsEl.append(batch.content);
            }
            // DOM order follows the scene: shallow regions first so nested ones paint above.
            let cursor = regionsEl.firstElementChild;
            for (const region of regions) {
                const entry = regionEls.get(region.id);
                if (!entry) continue;
                if (entry.el === cursor) cursor = cursor.nextElementSibling; else regionsEl.insertBefore(entry.el, cursor);
            }
        }

        // ---- connections ------------------------------------------------------------
        function parseLine(markup) {
            const svg = document.createElementNS(edgesSvg.namespaceURI, 'svg');
            svg.innerHTML = markup;
            return svg.firstElementChild;
        }
        function lineMarkup(edge, caption, title) {
            return `<g class="infra-edge" data-edge="${esc(edge.id)}" data-kind="${esc(edge.kind)}"><path class="infra-edge-hit" d=""/><path class="infra-edge-line" d=""/>`
                + (edge.kind === 'fiber' ? '<path class="infra-edge-line infra-edge-core" d=""/>' : '')
                + (caption ? `<text class="infra-edge-label" x="0" y="0">${esc(caption)}</text>` : '') + `<title>${esc(title)}</title></g>`;
        }
        function applyLine(entry, seg) {
            const d = `M${round1(seg.x1)} ${round1(seg.y1)}L${round1(seg.x2)} ${round1(seg.y2)}`;
            if (d !== entry.d) {
                entry.hit.setAttribute('d', d); entry.line.setAttribute('d', d); entry.core?.setAttribute('d', d);
                entry.d = d;
            }
            if (!entry.label) return;
            const mx = round1((seg.x1 + seg.x2) / 2), my = round1((seg.y1 + seg.y2) / 2);
            if (entry.lx !== mx || entry.ly !== my) { entry.label.setAttribute('x', mx); entry.label.setAttribute('y', my); entry.lx = mx; entry.ly = my; }
        }
        function shapeOf(id, cards, regions) {
            const card = cards?.get(id) || last.cardById.get(id);
            if (card) return { x: card.x, y: card.y, half: cardHalf };
            const r = regions?.get(id) || last.regionRects.get(id);
            return r ? { x: r.x + r.w / 2, y: r.y + r.h / 2, half: { w: r.w / 2, h: r.h / 2 } } : null;
        }
        function redrawLine(lineId, cards, regions) {
            const geom = lineGeom.get(lineId), entry = lineEls.get(lineId);
            if (!geom || !entry) return;
            const a = shapeOf(geom.from, cards, regions), b = shapeOf(geom.to, cards, regions);
            if (a && b) applyLine(entry, model.edgeSegment(a, b, a.half, b.half));
        }
        function patchLines(lines, force) {
            const edgeById = new Map(store.state.edges.map(edge => [edge.id, edge]));
            const seen = new Set();
            linesByEnd = new Map(); lineGeom = new Map();
            for (const line of lines) {
                const edge = edgeById.get(line.id);
                if (!edge) continue;
                seen.add(line.id);
                const caption = edge.label || model.edgeCaption(edge);
                const title = `${t(`desktop.infra_edge_${edge.kind}`)}${caption ? ` · ${caption}` : ''}`;
                const sig = [edge.kind, caption, title].join('\x1f');
                let entry = lineEls.get(line.id);
                if (!entry || force || entry.sig !== sig) {
                    const g = parseLine(lineMarkup(edge, caption, title));
                    if (entry) entry.g.replaceWith(g); else edgeLayer.appendChild(g);
                    entry = { g, hit: g.querySelector('.infra-edge-hit'), line: g.querySelector('.infra-edge-line:not(.infra-edge-core)'), core: g.querySelector('.infra-edge-core'), label: g.querySelector('.infra-edge-label'), sig, d: '', lx: null, ly: null, flags: {} };
                    lineEls.set(line.id, entry);
                }
                lineGeom.set(line.id, { from: line.from, to: line.to });
                for (const end of [line.from, line.to]) (linesByEnd.get(end) || linesByEnd.set(end, new Set()).get(end)).add(line.id);
                applyLine(entry, line);
            }
            for (const [id, entry] of lineEls) if (!seen.has(id)) { entry.g.remove(); lineEls.delete(id); }
        }

        // ---- states and overlays ------------------------------------------------------
        function paintStates() {
            const view = app.mapView(), selection = store.state.selection, filtersOn = model.filtersActive(store.state.filters);
            let shown = 0;
            for (const card of last.cards) {
                const entry = cardEls.get(card.id);
                if (!entry) continue;
                const guestList = filtersOn ? guestNodesOf(card) : [];
                const dim = filtersOn && !(store.matches(store.nodeById(card.id)) || guestList.some(guest => store.matches(guest)));
                if (!dim) shown += 1;
                const layout = store.layoutState(card.id, view);
                setFlag(entry, 'pressed', selection.has(card.id), on => entry.card.setAttribute('aria-pressed', on ? 'true' : 'false'));
                setFlag(entry, 'dim', dim, on => entry.card.classList.toggle('is-dimmed', on));
                setFlag(entry, 'saving', layout === 'saving', on => entry.card.classList.toggle('is-saving', on));
                setFlag(entry, 'unsaved', layout === 'unsaved', on => entry.card.classList.toggle('is-unsaved', on));
                if (entry.shelf && !entry.shelf.hidden) {
                    for (const chip of entry.shelf.querySelectorAll('[data-hosted-service]')) {
                        const guest = store.nodeById(chip.dataset.hostedService);
                        chip.classList.toggle('is-selected', selection.has(chip.dataset.hostedService));
                        chip.classList.toggle('is-dimmed', filtersOn && !!guest && !store.matches(guest));
                    }
                }
            }
            for (const [id, entry] of regionEls) {
                const on = selection.has(id);
                setFlag(entry, 'pressed', on, value => { entry.header.setAttribute('aria-pressed', value ? 'true' : 'false'); entry.el.classList.toggle('is-selected', value); });
            }
            for (const [id, entry] of lineEls) setFlag(entry, 'selected', store.state.selectedEdge === id, on => entry.g.classList.toggle('is-selected', on));
            return shown;
        }
        function paintOverlays(shown) {
            const filtersOn = model.filtersActive(store.state.filters), view = app.mapView(), canWrite = store.canWrite();
            visibleCount.textContent = t('desktop.infra_visible_count').replace('{count}', shown);
            emptyEl.hidden = !(store.state.refreshAt && !last.cards.length && !last.regions.length);
            for (const button of emptyEl.querySelectorAll('button')) button.disabled = !canWrite;
            noMatchEl.hidden = !(filtersOn && last.cards.length && shown === 0);
            const unsaved = store.unsavedCount(view);
            chipEl.hidden = unsaved === 0;
            retryButton.disabled = unsaved === 0 || !canWrite || store.state.busy.has('layout');
        }
        function renderNow() {
            if (disposed || store.state.view === 'table' || !viewport.isConnected) return;
            const view = app.mapView(), force = forceNext;
            forceNext = false;
            last = model.buildScene({ nodes: store.state.nodes, edges: store.state.edges, view,
                placementOf: id => store.effectivePlacement(id, view), parentOf: id => store.effectiveParent(id, view), guests: store.guests() });
            patchRegions(last.regions, force); patchCards(last.cards, force); patchLines(last.lines, force);
            if (mapDirty) { mapDirty = false; revision += 1; }
            paintOverlays(paintStates()); ensureInitialView();
            viewport.classList.toggle('is-readonly', !store.canWrite());
            applyTool(); applyFrame.request();
        }
        function render({ force = false } = {}) {
            if (force) forceNext = true;
            renderFrame.request();
        }

        // ---- minimap -------------------------------------------------------------------
        function rebuildMinimap() {
            const regionRects = last.regions.map(region => `<rect class="region" x="${round1(region.rect.x)}" y="${round1(region.rect.y)}" width="${round1(region.rect.w)}" height="${round1(region.rect.h)}"/>`).join('');
            const cardRects = last.cards.map(card => `<rect class="card" x="${card.x - model.CARD_W / 2}" y="${card.y - model.CARD_H / 2}" width="${model.CARD_W}" height="${model.CARD_H}"/>`).join('');
            minimapRegions.innerHTML = regionRects;
            minimapCards.innerHTML = cardRects;
            mmCardEls.clear(); mmSelected = new Set();
            last.cards.forEach((card, i) => mmCardEls.set(card.id, minimapCards.children[i]));
            drawnRevision = revision;
        }
        function drawMinimap() {
            if (disposed || !minimapOn() || store.state.view === 'table') return;
            const v = getView(), sz = size();
            const visible = sz.w > 0 && sz.h > 0 ? { x: -v.x / v.zoom, y: -v.y / v.zoom, w: sz.w / v.zoom, h: sz.h / v.zoom } : null;
            if (drawnRevision !== revision) rebuildMinimap();
            const tr = mmDragging && mmFrozen ? mmFrozen : model.minimapTransform(model.unionRects([last.bounds, visible]), MM);
            mmTransform = tr;
            const transform = `translate(${round1(tr.x)} ${round1(tr.y)}) scale(${tr.scale})`;
            if (minimapCards.getAttribute('transform') !== transform) { minimapCards.setAttribute('transform', transform); minimapRegions.setAttribute('transform', transform); }
            const selection = store.state.selection;
            for (const id of mmSelected) if (!selection.has(id)) { mmCardEls.get(id)?.classList.remove('is-selected'); mmSelected.delete(id); }
            for (const id of selection) if (!mmSelected.has(id) && mmCardEls.has(id)) { mmCardEls.get(id).classList.add('is-selected'); mmSelected.add(id); }
            if (!visible) return;
            minimapView.setAttribute('x', round1(visible.x * tr.scale + tr.x)); minimapView.setAttribute('y', round1(visible.y * tr.scale + tr.y));
            minimapView.setAttribute('width', round1(visible.w * tr.scale)); minimapView.setAttribute('height', round1(visible.h * tr.scale));
        }
        function minimapJump(event) {
            const rect = minimapSvg.getBoundingClientRect(), tr = mmFrozen || mmTransform;
            if (!tr || !tr.scale || !rect.width || !rect.height) return;
            const wx = ((event.clientX - rect.left) * MM.w / rect.width - tr.x) / tr.scale, wy = ((event.clientY - rect.top) * MM.h / rect.height - tr.y) / tr.scale;
            const v = getView(), sz = size();
            setView({ x: sz.w / 2 - wx * v.zoom, y: sz.h / 2 - wy * v.zoom, zoom: v.zoom });
        }
        const onMinimapDown = event => {
            if (event.button !== 0) return;
            mmDragging = true; mmFrozen = mmTransform;
            minimapSvg.setPointerCapture?.(event.pointerId);
            event.preventDefault();
            minimapJump(event);
        };
        const onMinimapMove = event => { if (mmDragging) minimapJump(event); };
        const onMinimapEnd = () => { mmDragging = false; mmFrozen = null; minimapFrame.request(); };
        minimapSvg.addEventListener('pointerdown', onMinimapDown);
        minimapSvg.addEventListener('pointermove', onMinimapMove);
        minimapSvg.addEventListener('pointerup', onMinimapEnd);
        minimapSvg.addEventListener('pointercancel', onMinimapEnd);
        minimapSvg.addEventListener('lostpointercapture', onMinimapEnd);
        function setMinimapVisible(on) {
            minimapToggle.setAttribute('aria-pressed', on ? 'true' : 'false');
            if (on) minimapSvg.removeAttribute('hidden'); else minimapSvg.setAttribute('hidden', '');
            minimapEl.classList.toggle('is-collapsed', !on);
            if (on) { drawnRevision = -1; minimapFrame.request(); }
        }
        if (app.prefs && app.prefs.minimap === false) setMinimapVisible(false);

        // ---- tools -----------------------------------------------------------------------
        const toolButtons = [...toolsEl.querySelectorAll('button[data-tool]')];
        function applyTool() {
            let tool = store.state.tool;
            const canConnect = store.canWrite();
            toolsEl.querySelector('button[data-tool="connect"]').disabled = !canConnect;
            if (tool === 'connect' && !canConnect) { store.setTool('select'); tool = store.state.tool; }
            viewport.dataset.tool = tool;
            for (const button of toolButtons) button.setAttribute('aria-pressed', button.dataset.tool === tool ? 'true' : 'false');
        }
        const onClick = event => {
            const button = event.target.closest('button');
            if (!button || button.disabled || !button.closest('[data-tools], [data-minimap]')) return;
            if (button.dataset.tool) { store.setTool(button.dataset.tool); return; }
            switch (button.dataset.action) {
                case 'zoom-in': zoomBy(1.2); break;
                case 'zoom-out': zoomBy(1 / 1.2); break;
                case 'zoom-reset': zoomTo(1); break;
                case 'fit': fit({ animate: true }); break;
                case 'toggle-minimap': {
                    const on = !minimapOn();
                    setMinimapVisible(on);
                    app.setPref?.('minimap', on);
                    break;
                }
                default: break;
            }
        };
        // rovePrimary makes button the toolbar's one tab stop.
        const rovePrimary = target => { for (const button of toolsEl.querySelectorAll('button')) button.tabIndex = button === target ? 0 : -1; };
        const onToolsKey = event => {
            const buttons = [...toolsEl.querySelectorAll('button:not(:disabled)')], index = buttons.indexOf(event.target);
            if (index < 0) return;
            const next = { ArrowRight: (index + 1) % buttons.length, ArrowLeft: (index + buttons.length - 1) % buttons.length, Home: 0, End: buttons.length - 1 }[event.key];
            if (next === undefined) return;
            event.preventDefault();
            rovePrimary(buttons[next]);
            buttons[next].focus();
        };
        const onToolsFocus = event => { const target = event.target.closest('button'); if (target) rovePrimary(target); };
        el.addEventListener('click', onClick);
        toolsEl.addEventListener('keydown', onToolsKey);
        toolsEl.addEventListener('focusin', onToolsFocus);

        // ---- temporary positions, highlights and the drag ghost -----------------------------
        function moveTemp(cards = new Map(), regions = new Map()) {
            const affected = new Set(), lines = new Set();
            for (const [id, pos] of cards) {
                const entry = cardEls.get(id);
                if (!entry) continue;
                writeCard(entry, pos.x, pos.y); temp.cards.add(id); affected.add(id);
            }
            for (const [id, rect] of regions) {
                const entry = regionEls.get(id);
                if (!entry) continue;
                writeRegion(entry, rect); temp.regions.add(id); affected.add(id);
            }
            for (const id of affected) for (const lineId of linesByEnd.get(id) || []) lines.add(lineId);
            for (const lineId of lines) { temp.lines.add(lineId); redrawLine(lineId, cards, regions); }
        }
        function clearTemp() {
            for (const id of temp.cards) {
                const entry = cardEls.get(id), card = last.cardById.get(id);
                if (entry && card) writeCard(entry, card.x, card.y);
            }
            for (const id of temp.regions) {
                const entry = regionEls.get(id), rect = last.regionRects.get(id);
                if (entry && rect) writeRegion(entry, rect);
            }
            for (const lineId of temp.lines) redrawLine(lineId);
            temp.cards.clear(); temp.regions.clear(); temp.lines.clear();
        }
        function applyDropTarget(id) {
            const entry = regionEls.get(id);
            if (!entry) return;
            entry.el.classList.add('is-drop-target');
            const node = store.nodeById(id);
            entry.drop.textContent = model.fill(t('desktop.infra_drop_into'), { name: node?.name || id });
        }
        function setDropTarget(id) {
            if (id === dropTargetId) return;
            const previous = regionEls.get(dropTargetId);
            if (previous) { previous.el.classList.remove('is-drop-target'); previous.drop.textContent = ''; }
            dropTargetId = id || '';
            if (dropTargetId) applyDropTarget(dropTargetId);
        }
        function setConnectTarget(id) {
            if (id === connectTargetId) return;
            (cardEls.get(connectTargetId)?.card || regionEls.get(connectTargetId)?.el)?.classList.remove('is-connect-target');
            connectTargetId = id || '';
            (cardEls.get(connectTargetId)?.card || regionEls.get(connectTargetId)?.el)?.classList.add('is-connect-target');
        }
        function showEdgePreview(fromId, point) {
            const from = point ? shapeOf(fromId) : null;
            if (!from) { edgePreview.setAttribute('d', ''); return; }
            const seg = model.edgeSegment(from, point, from.half, { w: 0, h: 0 });
            edgePreview.setAttribute('d', `M${round1(seg.x1)} ${round1(seg.y1)}L${round1(seg.x2)} ${round1(seg.y2)}`);
        }
        function showMarquee(rect) {
            marquee.hidden = !rect;
            if (!rect) return;
            marquee.style.left = px(rect.x); marquee.style.top = px(rect.y); marquee.style.width = px(rect.w); marquee.style.height = px(rect.h);
        }
        function setRovingCard(id) {
            const pick = key => cardEls.get(key)?.card || regionEls.get(key)?.header || null;
            const target = id ? pick(id) : null, previous = rovingId ? pick(rovingId) : null;
            if (previous && previous !== target) previous.tabIndex = -1;
            if (target) { target.tabIndex = 0; rovingId = id; viewport.tabIndex = -1; }
            else { rovingId = ''; viewport.tabIndex = 0; }
        }
        // createGhost makes the drag ghost of ids; items ({icon, name}) describe dragged things that are not
        // assets yet (discoveries).
        function createGhost(ids, items = null) {
            const first = items?.length ? items[0] : store.nodeById(ids[0]), total = (items?.length ? items : ids).length;
            const ghost = document.createElement('div');
            ghost.className = 'infra-drag-ghost';
            ghost.setAttribute('aria-hidden', 'true');
            ghost.innerHTML = `<span class="infra-ghost-icon">${first ? app.icon(first.icon) : ''}</span><span class="infra-ghost-text"><strong>${esc(first?.name || '')}</strong>${total > 1 ? `<small>+${total - 1}</small>` : ''}</span>`;
            app.root.appendChild(ghost);
            ghosts.add(ghost);
            return {
                el: ghost,
                move(clientX, clientY, { scale = 1, label = '', invalid = false } = {}) {
                    const rect = app.root.getBoundingClientRect();
                    ghost.style.transform = `translate(${clientX - rect.left + 8}px, ${clientY - rect.top + 8}px) scale(${scale})`;
                    ghost.dataset.dropLabel = label;
                    ghost.classList.toggle('is-invalid', !!invalid);
                },
                remove() { ghost.remove(); ghosts.delete(ghost); },
            };
        }
        function cardEl(id) {
            const entry = cardEls.get(id);
            if (entry) return entry.card;
            const host = cardEls.get(store.guests().hostOf.get(id));
            return host?.shelf ? [...host.shelf.querySelectorAll('[data-hosted-service]')].find(chip => chip.dataset.hostedService === id) || null : null;
        }
        const regionEl = id => regionEls.get(id)?.el || null;

        // ---- size changes and teardown ------------------------------------------------------------
        const observer = typeof ResizeObserver === 'function' ? new ResizeObserver(() => {
            const now = size();
            if (now.w > 0 && now.h > 0) {
                for (const key of Object.keys(views)) {
                    const was = viewSize[key] || lastSize;
                    if (!views[key] || !was || was.w <= 0 || was.h <= 0) continue;
                    if (now.w !== was.w || now.h !== was.h) { views[key] = model.viewFromCenter(model.viewCenter(views[key], was), now) || views[key]; applyFrame.request(); }
                    viewSize[key] = now;
                }
            }
            lastSize = now;
            if (!initialized[curKey()]) renderFrame.request(); else minimapFrame.request();
        }) : null;
        observer?.observe(viewport);
        function dispose() {
            if (disposed) return;
            persistNow();
            disposed = true;
            observer?.disconnect();
            for (const f of [applyFrame, minimapFrame, renderFrame]) f.cancel();
            clearTimeout(animTimer); clearTimeout(persistTimer);
            for (const ghost of ghosts) ghost.remove();
            ghosts.clear();
            cardEls.clear(); regionEls.clear(); lineEls.clear(); mmCardEls.clear();
            el.remove();
        }

        return {
            el, viewport, render, renderNow, sceneData: () => last,
            getView, setView, fit, zoomBy, zoomTo, centerOn, ensureVisible, clientToWorld, size,
            cardEl, regionEl, moveTemp, clearTemp,
            setDropTarget, setConnectTarget, showEdgePreview, showMarquee, createGhost,
            setRovingCard, applyTool, dispose,
        };
    }

    window.InfraAppCanvas = { create };
})();
