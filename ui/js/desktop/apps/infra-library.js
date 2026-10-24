// Infra library: the docked left panel with the To place, On the map and
// Needs a host sections, its collapse rail and the width splitter. Rows are
// rebuilt per section (rAF-coalesced) and keep scroll position, focus and the
// roving tabindex. Dragging a To place row is driven by infra-gestures.js
// (startLibraryDrag), which calls suppressNextClick() after a real drag.
(function () {
    'use strict';

    const LIB_MIN = 220, LIB_MAX = 400, LIB_STEP = 16;
    const FOCUS_ATTRS = { librarySelect: 'data-library-select', place: 'data-place', libraryExpand: 'data-library-expand' };
    const clampWidth = value => Math.min(LIB_MAX, Math.max(LIB_MIN, Math.round(Number.isFinite(Number(value)) ? Number(value) : 264)));

    function create(app) {
        const { store, state, t, esc, model, root } = app;
        const S = app.safeId;
        const byName = (a, b) => String(a.name).localeCompare(String(b.name));
        const sectionHTML = (id, title, emptyText) => `<section class="infra-library-section" data-library-section="${id}"${id === 'needs-host' ? ' hidden' : ''}>
      <h2 class="infra-library-heading"><button type="button" class="infra-library-toggle" data-section-toggle="${id}" aria-expanded="true" aria-controls="infra-lib-${id}-${S}"><span class="infra-library-title">${esc(title)}</span><span class="infra-library-count" data-section-count="${id}">0</span></button></h2>
      <ul class="infra-library-list" id="infra-lib-${id}-${S}" data-section-list="${id}"></ul>${emptyText ? `
      <p class="infra-library-empty" data-section-empty="${id}" hidden>${esc(emptyText)}</p>` : ''}
    </section>`;
        const holder = document.createElement('div');
        holder.innerHTML = `<aside class="infra-library" data-library id="infra-library-${S}" aria-label="${esc(t('desktop.infra_library'))}">
  <div class="infra-library-scroll" data-library-scroll>
    ${sectionHTML('to-place', t('desktop.infra_to_place'), t('desktop.infra_everything_placed'))}
    ${sectionHTML('on-map', t('desktop.infra_on_map'), t('desktop.infra_library_empty_map'))}
    ${sectionHTML('needs-host', t('desktop.infra_needs_host'), '')}
  </div>
  <div class="infra-library-rail" data-library-rail hidden><button type="button" class="infra-cmd-icon" data-action="expand-library" aria-label="${esc(t('desktop.infra_expand_library'))}" title="${esc(t('desktop.infra_expand_library'))}">${app.glyph('library')}</button></div>
</aside>
<div class="infra-splitter" data-splitter="library" role="separator" aria-orientation="vertical" aria-controls="infra-library-${S}" aria-valuemin="${LIB_MIN}" aria-valuemax="${LIB_MAX}" aria-valuenow="264" aria-label="${esc(t('desktop.infra_resize_library'))}" tabindex="0"></div>`;
        const el = holder.firstElementChild, splitter = holder.lastElementChild;
        const scrollEl = el.querySelector('[data-library-scroll]'), railEl = el.querySelector('[data-library-rail]');
        const listOf = id => el.querySelector(`[data-section-list="${id}"]`);
        const countOf = id => el.querySelector(`[data-section-count="${id}"]`);
        const emptyOf = id => el.querySelector(`[data-section-empty="${id}"]`);
        const sectionEl = id => el.querySelector(`[data-library-section="${id}"]`);

        const collapsedSections = new Set(), expandedHosts = new Set();
        let rovingId = '', suppressClick = false, suppressTimer = 0, frameId = 0, disposed = false, width = clampWidth(app.prefs?.libraryWidth ?? 264), drag = null;

        // ---- sections -------------------------------------------------------------------------
        function computeSections() {
            const view = app.mapView(), guests = store.guests(), filtersOn = model.filtersActive(state.filters);
            const placementOf = id => store.effectivePlacement(id, view);
            const parentOf = id => store.effectiveParent(id, view);
            const toPlace = [], needsHost = [];
            for (const node of state.nodes) {
                if (guests.hostOf.has(node.id) || placementOf(node.id) || !store.matches(node)) continue;
                if (node.kind === 'vm' || node.kind === 'container') needsHost.push(node);
                else if (view !== 'topology' || !model.isContainer(node.kind)) toPlace.push(node);
            }
            toPlace.sort(byName); needsHost.sort(byName);
            const scene = model.buildScene({ nodes: state.nodes, edges: state.edges, view, placementOf, parentOf, guests });
            const regionIds = new Set(scene.regions.map(region => region.id));
            const buckets = new Map();
            const bucket = key => { let found = buckets.get(key); if (!found) { found = { regions: [], cards: [] }; buckets.set(key, found); } return found; };
            const bucketKey = id => { const parent = parentOf(id); return regionIds.has(parent) && parent !== id ? parent : ''; };
            for (const region of scene.regions) bucket(bucketKey(region.id)).regions.push(store.nodeById(region.id));
            for (const card of scene.cards) bucket(bucketKey(card.id)).cards.push(store.nodeById(card.id));
            const prune = row => {
                const children = row.children.map(prune).filter(Boolean);
                if (filtersOn && !store.matches(row.node) && !children.length) return null;
                return { ...row, children };
            };
            const visited = new Set();
            const cardRow = node => ({ node, kind: 'card', children: (scene.cardById.get(node.id)?.guests || []).map(id => store.nodeById(id)).filter(Boolean).sort(byName).map(guest => ({ node: guest, kind: 'guest', children: [] })) });
            const regionRow = node => {
                visited.add(node.id);
                const own = buckets.get(node.id) || { regions: [], cards: [] };
                const regions = own.regions.filter(item => item && !visited.has(item.id)).sort(byName).map(regionRow);
                return { node, kind: 'region', children: [...regions, ...own.cards.filter(Boolean).sort(byName).map(cardRow)] };
            };
            const top = bucket('');
            const roots = top.regions.filter(Boolean).sort(byName).map(regionRow);
            for (const region of scene.regions) if (!visited.has(region.id)) roots.push(regionRow(store.nodeById(region.id))); // a parent cycle still shows its places
            roots.push(...top.cards.filter(Boolean).sort(byName).map(cardRow));
            return {
                'to-place': toPlace.map(node => ({ node, kind: 'plain', children: [] })),
                'on-map': roots.map(prune).filter(Boolean),
                'needs-host': needsHost.map(node => ({ node, kind: 'plain', children: [] })),
            };
        }
        const countRows = rows => rows.reduce((total, row) => total + 1 + countRows(row.children), 0);

        // ---- rows -----------------------------------------------------------------------------------
        function rowHTML(row, sectionId) {
            const node = row.node, id = esc(node.id), name = esc(node.name || node.id);
            const meta = [app.labelForKind(node.kind), node.profile?.ip_address].filter(Boolean).join(' · ');
            const draggable = sectionId === 'to-place' && store.canWrite() ? ' data-draggable="true"' : '';
            let action = '';
            if (sectionId === 'to-place') {
                const label = store.canWrite() ? model.fill(t('desktop.infra_place_asset'), { name: node.name || node.id }) : t('desktop.infra_read_only');
                action = `<button type="button" class="infra-library-place" data-place="${id}" tabindex="-1" aria-label="${esc(label)}" title="${esc(label)}"${store.canWrite() ? '' : ' disabled'}>${app.glyph('plus')}</button>`;
            }
            const hasGuests = row.kind === 'card' && row.children.length > 0, open = hasGuests && expandedHosts.has(node.id);
            if (hasGuests) {
                const label = model.fill(open ? t('desktop.infra_hide_guests') : t('desktop.infra_show_guests'), { name: node.name || node.id });
                action = `<button type="button" class="infra-library-disclosure" data-library-expand="${id}" tabindex="-1" aria-expanded="${open}" aria-label="${esc(label)}">${app.glyph('chevron')}</button>`;
            }
            const kids = row.kind === 'card' ? (open ? row.children : []) : row.children;
            return `<li class="infra-library-row" data-library-item="${id}" data-section="${sectionId}" data-row-kind="${esc(row.kind)}"${draggable} aria-selected="${state.selection.has(node.id)}">`
                + `<button type="button" class="infra-library-row-main" data-library-select="${id}" tabindex="-1" title="${name}"><img class="infra-library-row-icon" src="${esc(model.iconURL(node.icon))}" alt="" draggable="false">`
                + `<span class="infra-library-row-text"><span class="infra-library-row-name">${name}</span><span class="infra-library-row-meta">${esc(meta)}</span></span></button>${action}`
                + (kids.length ? `<ul class="infra-library-children">${kids.map(kid => rowHTML(kid, sectionId)).join('')}</ul>` : '') + '</li>';
        }

        // ---- rendering ----------------------------------------------------------------------------
        const visibleButtons = () => [...el.querySelectorAll('[data-library-select]')].filter(button => !button.closest('[hidden]'));
        function applyRoving() {
            const buttons = visibleButtons();
            const target = buttons.find(button => button.dataset.librarySelect === rovingId) || buttons[0] || null;
            for (const button of el.querySelectorAll('[data-library-select]')) button.tabIndex = button === target ? 0 : -1;
            rovingId = target ? target.dataset.librarySelect : '';
        }
        const findByAttr = (attr, value) => [...el.querySelectorAll(`[${FOCUS_ATTRS[attr]}]`)].find(node => node.dataset[attr] === value) || null;
        function applyCollapsed() {
            const collapsed = root.dataset.libraryCollapsed === 'true';
            scrollEl.hidden = collapsed; railEl.hidden = !collapsed;
            el.classList.toggle('is-collapsed', collapsed);
        }
        function renderNow() {
            if (disposed) return;
            if (frameId) { cancelAnimationFrame(frameId); frameId = 0; }
            applyCollapsed();
            const sections = computeSections(), filtersOn = model.filtersActive(state.filters);
            const active = document.activeElement && el.contains(document.activeElement) ? document.activeElement : null;
            const focusKey = active ? Object.keys(FOCUS_ATTRS).find(attr => active.dataset[attr] !== undefined) : '';
            const focusValue = focusKey ? active.dataset[focusKey] : '';
            const scrollTop = scrollEl.scrollTop;
            for (const id of Object.keys(sections)) {
                const rows = sections[id], list = listOf(id);
                list.innerHTML = rows.map(row => rowHTML(row, id)).join('');
                list.hidden = collapsedSections.has(id);
                el.querySelector(`[data-section-toggle="${id}"]`).setAttribute('aria-expanded', String(!collapsedSections.has(id)));
                countOf(id).textContent = String(countRows(rows));
                const empty = emptyOf(id);
                if (empty) empty.hidden = rows.length > 0 || filtersOn;
            }
            sectionEl('needs-host').hidden = sections['needs-host'].length === 0;
            applyRoving();
            scrollEl.scrollTop = scrollTop;
            if (focusKey) (findByAttr(focusKey, focusValue) || findByAttr('librarySelect', rovingId))?.focus({ preventScroll: true });
        }
        function render() {
            if (disposed || frameId) return;
            frameId = requestAnimationFrame(() => { frameId = 0; renderNow(); });
        }
        function renderSelection() {
            for (const row of el.querySelectorAll('[data-library-item]')) row.setAttribute('aria-selected', String(state.selection.has(row.dataset.libraryItem)));
        }
        // reveal opens On the map, unfolds the host that holds id (or id itself when
        // it is a host, for "+N more") and puts focus on the row.
        function reveal(id) {
            if (root.dataset.libraryCollapsed === 'true') app.setLibraryCollapsed?.(false);
            collapsedSections.delete('on-map');
            const host = store.guests().hostOf.get(id);
            if (host) expandedHosts.add(host);
            if ((store.guests().guestsOf.get(id) || []).length) expandedHosts.add(id);
            renderNow();
            const button = findByAttr('librarySelect', id);
            if (!button) return;
            rovingId = id; applyRoving();
            button.scrollIntoView({ block: 'nearest' });
            button.focus({ preventScroll: true });
        }
        function suppressNextClick() {
            suppressClick = true;
            clearTimeout(suppressTimer);
            suppressTimer = setTimeout(() => { suppressClick = false; }, 50);
        }

        // ---- behaviour --------------------------------------------------------------------------------
        function selectRow(button, event) {
            const id = button.dataset.librarySelect, item = button.closest('[data-library-item]');
            store.select([id], { toggle: event.shiftKey || event.ctrlKey || event.metaKey });
            if (item?.dataset.section !== 'on-map') return;
            const host = store.guests().hostOf.get(id);
            if (host && !state.expandedHosts.has(host)) store.toggleHost(host);
            app.canvas?.ensureVisible(id);
        }
        // A library drag captures the pointer on the panel, which retargets the click that follows
        // to the panel itself; look for the row under the pointer instead.
        function clickTarget(event) {
            if (event.target !== el || event.detail === 0) return event.target;
            const hit = document.elementFromPoint(event.clientX, event.clientY);
            return hit && el.contains(hit) ? hit : event.target;
        }
        const onClick = event => {
            if (suppressClick) { suppressClick = false; return; }
            const target = clickTarget(event);
            const toggle = target.closest('[data-section-toggle]');
            if (toggle) {
                const id = toggle.dataset.sectionToggle;
                if (collapsedSections.has(id)) collapsedSections.delete(id); else collapsedSections.add(id);
                toggle.setAttribute('aria-expanded', String(!collapsedSections.has(id)));
                listOf(id).hidden = collapsedSections.has(id);
                applyRoving();
                return;
            }
            const place = target.closest('[data-place]');
            if (place) { if (!place.disabled) void app.gestures.placeAtCenter([place.dataset.place], { moveMode: true }); return; }
            const expand = target.closest('[data-library-expand]');
            if (expand) {
                const id = expand.dataset.libraryExpand;
                if (expandedHosts.has(id)) expandedHosts.delete(id); else expandedHosts.add(id);
                renderNow();
                return;
            }
            const select = target.closest('[data-library-select]');
            if (select) { selectRow(select, event); return; }
            if (target.closest('[data-action="expand-library"]')) app.setLibraryCollapsed?.(false);
        };
        const onPointerDown = event => {
            if (event.button !== 0 || event.target.closest('[data-place]')) return;
            const row = event.target.closest('[data-library-item][data-draggable="true"]');
            if (!row) return;
            const id = row.dataset.libraryItem;
            const placeable = new Set([...el.querySelectorAll('[data-library-item][data-section="to-place"]')].map(item => item.dataset.libraryItem));
            const selected = [...state.selection].filter(item => placeable.has(item));
            app.gestures.startLibraryDrag(event, state.selection.has(id) && selected.length ? selected : [id]);
        };
        const onKeyDown = event => {
            const target = event.target.closest('[data-library-select], [data-place], [data-library-expand]');
            if (!target || event.altKey || event.ctrlKey || event.metaKey) return;
            const item = target.closest('[data-library-item]');
            if (!item) return;
            const main = item.querySelector(':scope > [data-library-select]');
            const buttons = visibleButtons(), index = buttons.indexOf(main);
            const go = next => { if (!next) return; event.preventDefault(); rovingId = next.dataset.librarySelect; applyRoving(); next.focus(); };
            const placeButton = item.querySelector(':scope > [data-place]:not(:disabled)'), expandButton = item.querySelector(':scope > [data-library-expand]');
            switch (event.key) {
                case 'ArrowDown': go(buttons[index + 1]); break;
                case 'ArrowUp': go(buttons[index - 1]); break;
                case 'Home': go(buttons[0]); break;
                case 'End': go(buttons[buttons.length - 1]); break;
                case 'ArrowRight':
                    if (target === main && placeButton) { event.preventDefault(); placeButton.focus(); }
                    else if (target === main && expandButton && expandButton.getAttribute('aria-expanded') !== 'true') { event.preventDefault(); expandButton.click(); }
                    break;
                case 'ArrowLeft':
                    if (target !== main) { event.preventDefault(); main.focus(); }
                    else if (expandButton && expandButton.getAttribute('aria-expanded') === 'true') { event.preventDefault(); expandButton.click(); findByAttr('librarySelect', item.dataset.libraryItem)?.focus(); }
                    break;
                case 'p': case 'P':
                    if (target === main && item.dataset.section === 'to-place' && store.canWrite()) { event.preventDefault(); void app.gestures.placeAtCenter([item.dataset.libraryItem], { moveMode: true }); }
                    break;
                default: break;
            }
        };
        const onFocusIn = event => {
            const button = event.target.closest('[data-library-select]');
            if (!button || button.dataset.librarySelect === rovingId) return;
            rovingId = button.dataset.librarySelect; applyRoving();
        };
        el.addEventListener('click', onClick);
        el.addEventListener('pointerdown', onPointerDown);
        el.addEventListener('keydown', onKeyDown);
        el.addEventListener('focusin', onFocusIn);

        // ---- splitter ----------------------------------------------------------------------------------
        function setWidth(next, { persist = false } = {}) {
            width = clampWidth(next);
            root.style.setProperty('--infra-library-w', `${width}px`);
            splitter.setAttribute('aria-valuenow', String(width));
            if (persist) app.setPref?.('libraryWidth', width);
        }
        const onSplitDown = event => {
            if (event.button !== 0) return;
            event.preventDefault();
            drag = { x: event.clientX, width };
            try { splitter.setPointerCapture?.(event.pointerId); } catch (_) { /* the pointer is already gone */ }
            root.classList.add('is-resizing');
        };
        const onSplitMove = event => { if (drag) setWidth(drag.width + event.clientX - drag.x); };
        const onSplitEnd = () => {
            if (!drag) return;
            drag = null;
            root.classList.remove('is-resizing');
            app.setPref?.('libraryWidth', width);
        };
        const onSplitKey = event => {
            if (event.key === 'Enter') { event.preventDefault(); app.setLibraryCollapsed?.(root.dataset.libraryCollapsed !== 'true'); return; }
            const next = { ArrowLeft: width - LIB_STEP, ArrowRight: width + LIB_STEP, Home: LIB_MIN, End: LIB_MAX }[event.key];
            if (next === undefined) return;
            event.preventDefault();
            setWidth(next, { persist: true });
        };
        splitter.addEventListener('pointerdown', onSplitDown);
        splitter.addEventListener('pointermove', onSplitMove);
        splitter.addEventListener('pointerup', onSplitEnd);
        splitter.addEventListener('pointercancel', onSplitEnd);
        splitter.addEventListener('lostpointercapture', onSplitEnd);
        splitter.addEventListener('keydown', onSplitKey);
        setWidth(width);

        function dispose() {
            if (disposed) return;
            disposed = true;
            if (frameId) cancelAnimationFrame(frameId);
            clearTimeout(suppressTimer);
            root.classList.remove('is-resizing');
            el.remove(); splitter.remove();
        }

        return { el, splitter, render, renderNow, renderSelection, reveal, suppressNextClick, setWidth, applyCollapsed, dispose };
    }

    window.InfraAppLibrary = { create };
})();
