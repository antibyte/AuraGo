// Infra library: the docked left panel with the To place, On the map, Needs
// attention, Cleanup and Ignored sections, its collapse rail and the width
// splitter. Rows are rebuilt per section (rAF-coalesced) and keep scroll
// position, focus and the roving tabindex. To place and Needs attention are
// listboxes, On the map is a tree (the select button of a row is the option or
// treeitem and carries aria-selected). Dragging a row is driven by
// infra-gestures.js (startLibraryDrag), which calls suppressNextClick() after a
// real drag; place rows are drop targets (setDropTarget). The Cleanup and
// Ignored bodies come from infra-cleanup.js.
(function () {
    'use strict';

    const LIB_MIN = 220, LIB_MAX = 400, LIB_STEP = 16;
    const FOCUS_ATTRS = { librarySelect: 'data-library-select', place: 'data-place', libraryExpand: 'data-library-expand', cleanupAction: 'data-cleanup-action', cleanupDismiss: 'data-cleanup-dismiss', cleanupExpand: 'data-cleanup-expand', restoreSource: 'data-restore-source' };
    const CLEANUP_FOCUS = new Set(['cleanupAction', 'cleanupDismiss', 'cleanupExpand', 'restoreSource']);
    const ATTENTION_TABS = { source_stale: 'sources', source_unknown: 'sources', source_removed: 'sources', source_review: 'sources', action_review: 'sources', draft_conflict: 'details', layout_unsaved: 'overview' };
    const clampWidth = value => Math.min(LIB_MAX, Math.max(LIB_MIN, Math.round(Number.isFinite(Number(value)) ? Number(value) : 264)));

    function create(app) {
        const { store, state, t, esc, model, root } = app;
        const S = app.safeId;
        const byName = (a, b) => String(a.name).localeCompare(String(b.name));
        const sectionHTML = (id, title, { emptyText = '', role = '', hidden = false, listClass = 'infra-library-list', head = '', after = '' } = {}) => `<section class="infra-library-section" data-library-section="${id}"${hidden ? ' hidden' : ''}>
      <h2 class="infra-library-heading"><button type="button" class="infra-library-toggle" data-section-toggle="${id}" aria-expanded="true" aria-controls="infra-lib-${id}-${S}"><span class="infra-library-title">${esc(title)}</span><span class="infra-library-count" data-section-count="${id}">0</span></button>${head}</h2>
      <ul class="${listClass}" id="infra-lib-${id}-${S}" data-section-list="${id}"${role ? ` role="${role}" aria-multiselectable="true"` : ''} aria-label="${esc(title)}"></ul>${after}${emptyText ? `
      <p class="infra-library-empty" data-section-empty="${id}" hidden>${esc(emptyText)}</p>` : ''}
    </section>`;
        const addLabel = esc(t('desktop.infra_new_place'));
        const holder = document.createElement('div');
        holder.innerHTML = `<aside class="infra-library" data-library id="infra-library-${S}" aria-label="${esc(t('desktop.infra_library'))}">
  <div class="infra-library-scroll" data-library-scroll>
    ${sectionHTML('to-place', t('desktop.infra_to_place'), { emptyText: t('desktop.infra_everything_placed'), role: 'listbox' })}
    ${sectionHTML('on-map', t('desktop.infra_on_map'), { emptyText: t('desktop.infra_library_empty_map'), role: 'tree', head: `<button type="button" class="infra-library-add" data-action="library-add-place" aria-label="${addLabel}" title="${addLabel}">${app.glyph('plus')}</button>` })}
    ${sectionHTML('attention', t('desktop.infra_attention'), { role: 'listbox', hidden: true })}
    ${sectionHTML('cleanup', t('desktop.infra_cleanup'), { listClass: 'infra-cleanup-list', hidden: true, after: '<button type="button" class="infra-library-reset" data-action="cleanup-reset" hidden></button>' })}
    ${sectionHTML('ignored', t('desktop.infra_ignored'), { listClass: 'infra-ignored-list', hidden: true })}
  </div>
  <div class="infra-library-rail" data-library-rail hidden><button type="button" class="infra-cmd-icon" data-action="expand-library" aria-label="${esc(t('desktop.infra_expand_library'))}" title="${esc(t('desktop.infra_expand_library'))}">${app.glyph('library')}</button></div>
</aside>
<div class="infra-splitter" data-splitter="library" role="separator" aria-orientation="vertical" aria-controls="infra-library-${S}" aria-valuemin="${LIB_MIN}" aria-valuemax="${LIB_MAX}" aria-valuenow="264" aria-label="${esc(t('desktop.infra_resize_library'))}" tabindex="0"></div>`;
        const el = holder.firstElementChild, splitter = holder.lastElementChild;
        const scrollEl = el.querySelector('[data-library-scroll]'), railEl = el.querySelector('[data-library-rail]');
        const addButton = el.querySelector('[data-action="library-add-place"]'), resetButton = el.querySelector('[data-action="cleanup-reset"]');
        const listOf = id => el.querySelector(`[data-section-list="${id}"]`);
        const countOf = id => el.querySelector(`[data-section-count="${id}"]`);
        const titleOf = id => el.querySelector(`[data-section-toggle="${id}"] .infra-library-title`);
        const emptyOf = id => el.querySelector(`[data-section-empty="${id}"]`);
        const sectionEl = id => el.querySelector(`[data-library-section="${id}"]`);

        const collapsedSections = new Set(['ignored']), expandedHosts = new Set();
        let rovingId = '', suppressClick = false, suppressTimer = 0, frameId = 0, disposed = false, width = clampWidth(app.prefs?.libraryWidth ?? 264), drag = null, dropEl = null, resetShown = false;

        // ---- sections -------------------------------------------------------------------------
        function computeSections() {
            const view = app.mapView(), guests = store.guests(), filtersOn = model.filtersActive(state.filters);
            // Topology draws every placeable asset at an automatic slot, so they count as on the map there.
            const auto = view === 'topology' ? app.canvas.autoPositions() : null;
            const placementOf = id => store.effectivePlacement(id, view) || auto?.get(id) || null;
            const parentOf = id => store.effectiveParent(id, view);
            const toPlace = [];
            if (view !== 'topology') for (const node of state.nodes) { // nothing is left to place in Topology
                if (guests.hostOf.has(node.id) || placementOf(node.id) || !store.matches(node)) continue;
                if (node.kind === 'vm' || node.kind === 'container') continue; // a guest without a host is a Cleanup suggestion
                toPlace.push(node);
            }
            toPlace.sort(byName);
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
            const unsaved = new Set(state.nodes.filter(node => ['atlas', 'topology'].some(layoutView => store.layoutState(node.id, layoutView) === 'unsaved')).map(node => node.id));
            const actionBlocked = new Set([...state.actionBlocks.keys()].map(key => String(key).split('\x00')[0]));
            const attention = model.attentionItems(state.nodes.filter(store.matches), { unsaved, reviews: new Set(state.nodeReviews.keys()), sourceBlocked: state.sourceBlocked, actionBlocked })
                .map(item => ({ node: store.nodeById(item.id), kind: 'plain', children: [], reasons: item.reasons })).filter(row => row.node);
            return {
                'to-place': toPlace.map(node => ({ node, kind: 'plain', children: [] })),
                'on-map': roots.map(prune).filter(Boolean),
                attention,
            };
        }
        const countRows = rows => rows.reduce((total, row) => total + 1 + countRows(row.children), 0);
        const reasonText = reason => {
            const source = reason.source ? t(`desktop.infra_source_${reason.source}`) : '';
            switch (reason.code) {
                case 'source_stale': return model.fill(t('desktop.infra_attention_source_stale'), { source });
                case 'source_unknown': return model.fill(t('desktop.infra_attention_source_unknown'), { source });
                case 'source_removed': return model.fill(t('desktop.infra_attention_source_removed'), { source });
                case 'layout_unsaved': return t('desktop.infra_attention_layout_unsaved');
                case 'draft_conflict': return t('desktop.infra_attention_draft_conflict');
                case 'source_review': return t('desktop.infra_attention_source_review');
                default: return t('desktop.infra_attention_action_review');
            }
        };

        // ---- rows -----------------------------------------------------------------------------------
        function rowHTML(row, sectionId, level = 1) {
            const node = row.node, id = esc(node.id), name = esc(node.name || node.id), write = store.canWrite();
            const tree = sectionId === 'on-map';
            let meta = esc([app.labelForKind(node.kind), node.profile?.ip_address].filter(Boolean).join(' · ')), metaClass = 'infra-library-row-meta';
            if (row.reasons) {
                metaClass += ' infra-attention-reason';
                meta = esc(reasonText(row.reasons[0]) + (row.reasons.length > 1 ? ` · ${model.fill(t('desktop.infra_more_guests'), { count: row.reasons.length - 1 })}` : ''));
            }
            const asset = sectionId === 'to-place' || (tree && row.kind === 'card');
            const draggable = write && asset ? ' data-draggable="true"' : '';
            const drop = write && model.isContainer(node.kind) && (sectionId === 'to-place' || tree) ? ` data-library-drop="${id}"` : '';
            let action = '';
            if (sectionId === 'to-place') {
                const label = write ? model.fill(t('desktop.infra_place_asset'), { name: node.name || node.id }) : t('desktop.infra_read_only');
                action = `<button type="button" class="infra-library-place" data-place="${id}" tabindex="-1" aria-hidden="true" title="${esc(label)}"${write ? '' : ' disabled'}>${app.glyph('plus')}</button>`;
            }
            const hasGuests = tree && row.kind === 'card' && row.children.length > 0, open = hasGuests && expandedHosts.has(node.id);
            if (hasGuests) action = `<button type="button" class="infra-library-disclosure" data-library-expand="${id}" tabindex="-1" aria-hidden="true" aria-expanded="${open}">${app.glyph('chevron')}</button>`;
            const kids = row.kind === 'card' ? (open ? row.children : []) : row.children;
            const treeAttrs = tree ? ` aria-level="${level}"${hasGuests ? ` aria-expanded="${open}"` : row.kind === 'region' && row.children.length ? ' aria-expanded="true"' : ''}` : '';
            const attention = row.reasons ? ` data-attention-tab="${ATTENTION_TABS[row.reasons[0].code] || 'overview'}"` : '';
            const shortcut = sectionId === 'to-place' && write ? ' aria-keyshortcuts="P"' : '';
            return `<li class="infra-library-row" role="none" data-library-item="${id}" data-section="${sectionId}" data-row-kind="${esc(row.kind)}"${draggable}${drop}>`
                + `<button type="button" class="infra-library-row-main" role="${tree ? 'treeitem' : 'option'}" aria-selected="${state.selection.has(node.id)}"${treeAttrs}${shortcut} data-library-select="${id}"${attention} tabindex="-1" title="${name}"><img class="infra-library-row-icon" src="${esc(model.iconURL(node.icon))}" alt="" draggable="false">`
                + `<span class="infra-library-row-text"><span class="infra-library-row-name">${name}</span><span class="${metaClass}">${meta}</span></span></button>${action}`
                + (kids.length ? `<ul class="infra-library-children" role="group">${kids.map(kid => rowHTML(kid, sectionId, level + 1)).join('')}</ul>` : '') + '</li>';
        }

        // ---- rendering ----------------------------------------------------------------------------
        const visibleButtons = () => [...el.querySelectorAll('[data-library-select]')].filter(button => !button.closest('[hidden]'));
        function applyRoving() {
            const buttons = visibleButtons();
            const target = buttons.find(button => button.dataset.librarySelect === rovingId) || buttons[0] || null;
            for (const button of el.querySelectorAll('[data-library-select]')) button.tabIndex = button === target ? 0 : -1;
            rovingId = target ? target.dataset.librarySelect : '';
        }
        // findByAttr looks inside one section when it is given: an asset can sit in several sections.
        const findByAttr = (attr, value, sectionId = '') => [...(sectionId && sectionEl(sectionId) || el).querySelectorAll(`[${FOCUS_ATTRS[attr]}]`)].find(node => node.dataset[attr] === value) || null;
        function applyCollapsed() {
            const collapsed = root.dataset.libraryCollapsed === 'true';
            scrollEl.hidden = collapsed; railEl.hidden = !collapsed;
            el.classList.toggle('is-collapsed', collapsed);
        }
        function applyFold(id) {
            const folded = collapsedSections.has(id);
            listOf(id).hidden = folded;
            el.querySelector(`[data-section-toggle="${id}"]`).setAttribute('aria-expanded', String(!folded));
            if (id === 'cleanup') resetButton.hidden = folded || !resetShown;
        }
        function restoreFocus(focusKey, focusValue, sectionId) {
            let target = findByAttr(focusKey, focusValue, sectionId);
            if (!target && CLEANUP_FOCUS.has(focusKey)) {
                const section = sectionEl(sectionId);
                target = section?.querySelector('[data-cleanup-action]') || section?.querySelector('[data-section-toggle]') || null;
            }
            (target || findByAttr('librarySelect', rovingId, sectionId) || findByAttr('librarySelect', rovingId))?.focus({ preventScroll: true });
        }
        function renderNow() {
            if (disposed) return;
            if (frameId) { cancelAnimationFrame(frameId); frameId = 0; }
            applyCollapsed();
            const sections = computeSections(), filtersOn = model.filtersActive(state.filters);
            const active = document.activeElement && el.contains(document.activeElement) ? document.activeElement : null;
            const focusKey = active ? Object.keys(FOCUS_ATTRS).find(attr => active.dataset[attr] !== undefined) : '';
            const focusValue = focusKey ? active.dataset[focusKey] : '';
            const focusSection = active?.closest('[data-library-section]')?.dataset.librarySection || '';
            const scrollTop = scrollEl.scrollTop;
            // Topology draws every placeable asset by itself: nothing is left to place, and the placed section is "In topology".
            const topology = app.mapView() === 'topology';
            sectionEl('to-place').hidden = topology;
            const placedTitle = topology ? t('desktop.infra_in_topology') : t('desktop.infra_on_map');
            const placedEmpty = topology ? t('desktop.infra_library_empty_topology') : t('desktop.infra_library_empty_map');
            if (titleOf('on-map').textContent !== placedTitle) { titleOf('on-map').textContent = placedTitle; listOf('on-map').setAttribute('aria-label', placedTitle); }
            for (const id of Object.keys(sections)) {
                const rows = sections[id], list = listOf(id);
                list.innerHTML = rows.map(row => rowHTML(row, id)).join('');
                countOf(id).textContent = String(countRows(rows));
                const empty = emptyOf(id);
                if (empty) {
                    // Under a query or filter an empty section is "No matches", not a bare header with a 0.
                    empty.hidden = rows.length > 0;
                    empty.textContent = filtersOn ? t('desktop.infra_no_matches') : id === 'on-map' ? placedEmpty : t('desktop.infra_everything_placed');
                }
                applyFold(id);
            }
            sectionEl('attention').hidden = sections.attention.length === 0;
            addButton.disabled = !store.canWrite();
            const cleanup = app.cleanup.renderCleanup({ section: sectionEl('cleanup'), list: listOf('cleanup'), count: countOf('cleanup'), reset: resetButton });
            resetShown = !resetButton.hidden;
            app.cleanup.renderIgnored({ section: sectionEl('ignored'), list: listOf('ignored'), count: countOf('ignored') });
            applyFold('cleanup'); applyFold('ignored');
            applyRoving();
            scrollEl.scrollTop = scrollTop;
            if (focusKey) restoreFocus(focusKey, focusValue, focusSection);
            return cleanup;
        }
        function render() {
            if (disposed || frameId) return;
            frameId = requestAnimationFrame(() => { frameId = 0; renderNow(); });
        }
        function renderSelection() {
            for (const button of el.querySelectorAll('[data-library-select]')) button.setAttribute('aria-selected', String(state.selection.has(button.dataset.librarySelect)));
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
            const button = [...el.querySelectorAll('[data-section="on-map"] [data-library-select]')].find(item => item.dataset.librarySelect === id) || findByAttr('librarySelect', id);
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
        // setDropTarget highlights the place row a library drag is over ('' clears).
        function setDropTarget(id) {
            const next = id ? [...el.querySelectorAll('[data-library-drop]')].find(row => row.dataset.libraryDrop === id) || null : null;
            if (next === dropEl) return;
            dropEl?.classList.remove('is-drop-target');
            next?.classList.add('is-drop-target');
            dropEl = next;
        }

        // ---- behaviour --------------------------------------------------------------------------------
        function selectRow(button, event) {
            const id = button.dataset.librarySelect, item = button.closest('[data-library-item]');
            if (button.dataset.attentionTab && !(event.shiftKey || event.ctrlKey || event.metaKey)) {
                app.inspector.open(id, button.dataset.attentionTab);
                app.canvas?.ensureVisible(id);
                return;
            }
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
            if (app.cleanup.onClick(event, target)) return;
            const toggle = target.closest('[data-section-toggle]');
            if (toggle) {
                const id = toggle.dataset.sectionToggle;
                if (collapsedSections.has(id)) collapsedSections.delete(id); else collapsedSections.add(id);
                applyFold(id);
                applyRoving();
                return;
            }
            if (target.closest('[data-action="library-add-place"]')) { if (!addButton.disabled) app.dialogs.openNodeDialog('room', { title: t('desktop.infra_new_place') }); return; }
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
            if (event.button !== 0 || event.target.closest('[data-place], [data-library-expand]')) return;
            const row = event.target.closest('[data-library-item][data-draggable="true"]');
            if (!row) return;
            const id = row.dataset.libraryItem, section = row.dataset.section;
            const draggable = new Set([...el.querySelectorAll('[data-library-item][data-draggable="true"]')].filter(item => item.dataset.section === section).map(item => item.dataset.libraryItem));
            const selected = [...state.selection].filter(item => draggable.has(item));
            app.gestures.startLibraryDrag(event, state.selection.has(id) && selected.length ? selected : [id], { mode: section === 'on-map' ? 'move' : 'place' });
        };
        const onKeyDown = event => {
            const target = event.target.closest('[data-library-select]');
            if (!target || event.altKey || event.ctrlKey || event.metaKey) return;
            const item = target.closest('[data-library-item]');
            if (!item) return;
            const buttons = visibleButtons(), index = buttons.indexOf(target);
            const go = next => { if (!next) return; event.preventDefault(); rovingId = next.dataset.librarySelect; applyRoving(); next.focus(); };
            const expandButton = item.querySelector(':scope > [data-library-expand]');
            const expanded = target.getAttribute('aria-expanded');
            switch (event.key) {
                case 'ArrowDown': go(buttons[index + 1]); break;
                case 'ArrowUp': go(buttons[index - 1]); break;
                case 'Home': go(buttons[0]); break;
                case 'End': go(buttons[buttons.length - 1]); break;
                case 'ArrowRight':
                    if (expanded === 'false' && expandButton) { event.preventDefault(); expandButton.click(); }
                    else if (expanded === 'true') go(item.querySelector(':scope > ul [data-library-select]'));
                    break;
                case 'ArrowLeft':
                    if (expanded === 'true' && expandButton) { event.preventDefault(); expandButton.click(); findByAttr('librarySelect', item.dataset.libraryItem, item.dataset.section)?.focus(); }
                    else if (target.getAttribute('role') === 'treeitem') go(item.parentElement?.closest('[data-library-item]')?.querySelector(':scope > [data-library-select]'));
                    break;
                case 'p': case 'P':
                    if (item.dataset.section === 'to-place' && store.canWrite()) { event.preventDefault(); void app.gestures.placeAtCenter([item.dataset.libraryItem], { moveMode: true }); }
                    break;
                case 'Delete': case 'Backspace':
                    if (store.canWrite() && store.nodeById(item.dataset.libraryItem)) {
                        const id = item.dataset.libraryItem;
                        event.preventDefault();
                        void app.dialogs.deleteNodes(state.selection.has(id) && state.selection.size > 1 ? [...state.selection] : [id]);
                    }
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

        return { el, splitter, render, renderNow, renderSelection, reveal, suppressNextClick, setDropTarget, setWidth, applyCollapsed, dispose };
    }

    window.InfraAppLibrary = { create };
})();
