// Infra import sheet: a non-modal panel over the library column that lists the
// discoveries (GET /candidates) in relevance groups (recommended, possible
// duplicates, probably not homelab), imports them in batches (POST
// /import/batch), ignores them (PUT /sources/ignore) and lets a row be dragged
// onto the map to import and place it in one step. The canvas stays a drop
// target while the sheet is open. Every write goes through the store.
(function () {
    'use strict';

    const GROUPS = ['recommended', 'duplicate', 'unlikely'];
    const GUEST_KINDS = new Set(['vm', 'container']);
    const CONTROLS = ['[data-candidate]', '[data-bind]', '[data-import-one]', '[data-ignore-candidate]'];

    function create(app) {
        const { store, state, t, esc, model, root } = app;
        const S = app.safeId;
        const canWrite = () => store.canWrite();
        const titles = { recommended: t('desktop.infra_import_recommended'), duplicate: t('desktop.infra_import_duplicates'), unlikely: t('desktop.infra_import_unlikely') };
        const sections = GROUPS.map(group => `<section class="infra-import-group" data-import-group="${group}" hidden><h3 class="infra-import-group-head"><button type="button" class="infra-library-toggle" data-import-group-toggle="${group}" aria-expanded="${group !== 'unlikely'}" aria-controls="infra-import-${esc(S)}-${group}"><span class="infra-library-title">${esc(titles[group])}</span><span class="infra-library-count" data-import-group-count="${group}">0</span></button><input type="checkbox" data-import-check-group="${group}" aria-label="${esc(model.fill(t('desktop.infra_select_group'), { group: titles[group] }))}"></h3><ul class="infra-candidate-list" id="infra-import-${esc(S)}-${group}" data-import-list="${group}"${group === 'unlikely' ? ' hidden' : ''}></ul></section>`).join('');
        const holder = document.createElement('div');
        holder.innerHTML = `<section class="infra-import-sheet" data-import-sheet role="dialog" aria-modal="false" aria-labelledby="infra-import-title-${esc(S)}" hidden>
  <header class="infra-sheet-head"><h2 id="infra-import-title-${esc(S)}">${esc(t('desktop.infra_import_title'))}</h2><button type="button" class="infra-cmd-icon" data-action="close-import" aria-label="${esc(t('desktop.close'))}" title="${esc(t('desktop.close'))}">${app.glyph('close')}</button></header>
  <p class="infra-sheet-intro">${esc(t('desktop.infra_import_sheet_hint'))}</p>
  <div class="infra-candidate-source-tabs" data-candidate-sources></div>
  <div class="infra-import-search"><input type="search" data-import-search aria-label="${esc(t('desktop.infra_import_search'))}" placeholder="${esc(t('desktop.infra_import_search'))}" autocomplete="off"></div>
  <div class="infra-import-groups" data-import-groups>
    <p class="infra-import-loading" data-import-loading>${esc(t('desktop.infra_loading_candidates'))}</p>
    <div class="infra-empty-candidates" data-import-empty hidden><strong>${esc(t('desktop.infra_no_candidates'))}</strong><p>${esc(t('desktop.infra_no_candidates_hint'))}</p></div>
    ${sections}
  </div>
  <div class="infra-sheet-foot"><span data-import-count></span><button type="button" class="infra-button" data-action="ignore-selected" disabled>${esc(t('desktop.infra_ignore_selected'))}</button><button type="button" class="infra-button infra-button-primary" data-import disabled>${esc(t('desktop.infra_import_selected'))}</button></div>
</section>`;
        const el = holder.firstElementChild;
        const $ = selector => el.querySelector(selector);
        const search = $('[data-import-search]'), groupsEl = $('[data-import-groups]'), loadingEl = $('[data-import-loading]'), emptyEl = $('[data-import-empty]');
        const sourcesEl = $('[data-candidate-sources]'), countEl = $('[data-import-count]'), importButton = $('[data-import]'), ignoreButton = $('[data-action="ignore-selected"]');
        const groupEl = Object.fromEntries(GROUPS.map(group => [group, $(`[data-import-group="${group}"]`)]));
        const listEl = Object.fromEntries(GROUPS.map(group => [group, $(`[data-import-list="${group}"]`)]));
        const countOf = Object.fromEntries(GROUPS.map(group => [group, $(`[data-import-group-count="${group}"]`)]));
        const checkOf = Object.fromEntries(GROUPS.map(group => [group, $(`[data-import-check-group="${group}"]`)]));
        const toggleOf = Object.fromEntries(GROUPS.map(group => [group, $(`[data-import-group-toggle="${group}"]`)]));
        const errors = new Map(), collapsed = new Set(['unlikely']);
        let owner = '', opener = null, loading = false, sources = [], searchFrame = 0, suppressClick = false, suppressTimer = 0, disposed = false;
        let memoKey = null, memo = null, byIdKey = null, byIdMap = new Map(), suggestKey = null, suggestions = new Map(), pendingFocus = null;

        // ---- data ----------------------------------------------------------------------------------
        function groups() {
            if (!memoKey || memoKey[0] !== state.candidates || memoKey[1] !== state.nodes || memoKey[2] !== state.sourceHosts) {
                memo = model.groupDiscoveries(state.candidates, state.nodes, { sourceHosts: state.sourceHosts });
                memoKey = [state.candidates, state.nodes, state.sourceHosts];
            }
            return memo;
        }
        function candidateById(id) {
            if (byIdKey !== state.candidates) { byIdMap = new Map(state.candidates.map(candidate => [candidate.id, candidate])); byIdKey = state.candidates; }
            return byIdMap.get(id) || null;
        }
        // Suggestions are ranked once per discovery and node list; a thousand rows would otherwise rank on every paint.
        function suggestionsFor(candidate) {
            if (suggestKey !== state.nodes) { suggestions = new Map(); suggestKey = state.nodes; }
            if (!suggestions.has(candidate.id)) suggestions.set(candidate.id, model.linkSuggestions(candidate, state.nodes));
            return suggestions.get(candidate.id);
        }
        const nameOfCandidate = candidate => candidate?.node?.name || t('desktop.infra_unnamed_candidate');
        const matches = (candidate, query) => !query || [candidate.node?.name, candidate.node?.profile?.ip_address, candidate.node?.profile?.mac_address, candidate.source?.name, candidate.source?.instance, candidate.source?.kind]
            .some(value => String(value || '').toLocaleLowerCase().includes(query));
        const shownIn = group => { const query = search.value.trim().toLocaleLowerCase(); return groups()[group].filter(candidate => matches(candidate, query)); };
        const selectedItems = () => GROUPS.flatMap(group => groups()[group]).filter(candidate => state.importSelection.has(candidate.id))
            .map(candidate => ({ candidate_id: candidate.id, ...(state.importBindings.get(candidate.id) ? { node_id: state.importBindings.get(candidate.id) } : {}) }));
        // prune forgets selections, bindings and errors of discoveries that are gone.
        function prune() {
            if (loading && !state.candidates.length) return;
            const present = new Set(state.candidates.map(candidate => candidate.id));
            for (const id of [...state.importSelection]) if (!present.has(id)) state.importSelection.delete(id);
            for (const id of [...state.importBindings.keys()]) if (!present.has(id) || !store.nodeById(state.importBindings.get(id))) state.importBindings.delete(id);
            for (const id of [...errors.keys()]) if (!present.has(id)) errors.delete(id);
        }

        // ---- rendering --------------------------------------------------------------------------------
        function reasonTexts(candidate, relevance) {
            const duplicate = relevance?.duplicateOf;
            const other = duplicate ? (duplicate.nodeId ? store.nodeById(duplicate.nodeId)?.name : nameOfCandidate(candidateById(duplicate.candidateId))) : '';
            return (relevance?.reasons || []).map(reason => {
                if (reason === 'duplicate_ip') return model.fill(t('desktop.infra_relevance_duplicate_ip'), { name: other || '' });
                if (reason === 'duplicate_mac') return model.fill(t('desktop.infra_relevance_duplicate_mac'), { name: other || '' });
                if (reason === 'generic_discovery') return t('desktop.infra_relevance_generic_discovery');
                if (reason === 'inactive') return t('desktop.infra_relevance_inactive');
                return reason === 'link_local' ? t('desktop.infra_relevance_link_local') : '';
            }).filter(Boolean).join(' · ');
        }
        const optionHTML = (id, selected) => `<option value="${esc(id)}"${selected ? ' selected' : ''}>${esc(store.nodeById(id)?.name || id)}</option>`;
        function rowHTML(candidate, group, relevance) {
            const node = candidate.node || {}, source = candidate.source || {}, id = esc(candidate.id), name = nameOfCandidate(candidate);
            const write = canWrite(), busy = state.busy.has('import') || state.busy.has('ignore'), error = errors.get(candidate.id) || '';
            const bound = state.importBindings.get(candidate.id) || '';
            const suggested = [...new Set([bound, ...suggestionsFor(candidate)].filter(Boolean))].filter(nodeId => store.nodeById(nodeId));
            const draggable = write && store.features.layoutParent ? ' data-draggable="true"' : '';
            const reasons = reasonTexts(candidate, relevance), detail = [node.profile?.vendor, node.profile?.model, node.profile?.ip_address].filter(Boolean).join(' · ');
            const disabled = write && !busy ? '' : ' disabled';
            return `<li class="infra-candidate" data-candidate-row="${id}" data-group="${group}"${draggable}><label class="infra-candidate-check"><input type="checkbox" data-candidate="${id}" aria-label="${esc(model.fill(t('desktop.infra_select_candidate'), { name }))}"${state.importSelection.has(candidate.id) ? ' checked' : ''}><span class="infra-checkbox-mark"></span></label>`
                + `<div class="infra-candidate-icon">${app.icon(node.icon)}</div><div class="infra-candidate-copy"><strong title="${esc(name)}">${esc(name)}</strong><span>${esc(t(`desktop.infra_source_${source.kind}`))} · ${esc(source.name || source.instance || '')}</span><small>${esc(detail)}</small>${reasons ? `<small class="infra-candidate-reason" data-candidate-reason>${esc(reasons)}</small>` : ''}</div>`
                + `<label class="infra-field infra-bind-field"><span>${esc(t('desktop.infra_link_existing'))}</span><select data-bind="${id}"${write ? '' : ' disabled'}><option value="">${esc(t('desktop.infra_import_new'))}</option>${suggested.length ? `<optgroup label="${esc(t('desktop.infra_link_suggested'))}">${suggested.map(nodeId => optionHTML(nodeId, nodeId === bound)).join('')}</optgroup>` : ''}</select></label>`
                + `<div class="infra-candidate-actions"><button type="button" class="infra-button" data-import-one="${id}"${disabled}>${esc(t('desktop.infra_import_one'))}</button><button type="button" class="infra-button" data-ignore-candidate="${id}"${disabled}>${esc(t('desktop.infra_ignore'))}</button></div>`
                + `<p class="infra-candidate-error" data-candidate-error role="alert"${error ? '' : ' hidden'}>${esc(error)}</p></li>`;
        }
        // The control that has focus moves to the same control of the row that takes its place.
        function focusSnapshot() {
            const active = document.activeElement, row = active && el.contains(active) ? active.closest?.('[data-candidate-row]') : null;
            if (!row) return null;
            const selector = CONTROLS.find(item => active.matches(item));
            return selector ? { id: row.dataset.candidateRow, group: row.dataset.group, selector, index: [...row.parentElement.children].indexOf(row) } : null;
        }
        function restoreFocus(snapshot) {
            if (!snapshot) return;
            const same = [...listEl[snapshot.group].querySelectorAll('[data-candidate-row]')].find(row => row.dataset.candidateRow === snapshot.id);
            const next = same || listEl[snapshot.group].children[snapshot.index] || null;
            const control = next?.querySelector(`${snapshot.selector}:not(:disabled)`) || next?.querySelector('[data-candidate]');
            (control || search).focus({ preventScroll: true });
        }
        function syncGroupChecks() {
            for (const group of GROUPS) {
                const rows = shownIn(group), selected = rows.filter(candidate => state.importSelection.has(candidate.id)).length;
                checkOf[group].checked = rows.length > 0 && selected === rows.length;
                checkOf[group].indeterminate = selected > 0 && selected < rows.length;
            }
        }
        function updateCount() {
            const count = state.importSelection.size, busy = state.busy.has('import') || state.busy.has('ignore'), write = canWrite();
            countEl.textContent = model.fill(t('desktop.infra_selected_count'), { count });
            for (const button of [importButton, ignoreButton]) button.disabled = !count || !write || busy;
            for (const button of el.querySelectorAll('[data-import-one], [data-ignore-candidate]')) button.disabled = !write || busy;
        }
        function render() {
            if (disposed || el.hidden) return;
            prune();
            const snapshot = pendingFocus || focusSnapshot(), grouped = groups();
            pendingFocus = null;
            let shown = 0;
            for (const group of GROUPS) {
                const rows = shownIn(group);
                shown += rows.length;
                groupEl[group].hidden = rows.length === 0;
                countOf[group].textContent = String(rows.length);
                // A folded group is built when it opens.
                listEl[group].innerHTML = collapsed.has(group) ? '' : rows.map(candidate => rowHTML(candidate, group, grouped.relevance.get(candidate.id))).join('');
                listEl[group].hidden = collapsed.has(group);
                toggleOf[group].setAttribute('aria-expanded', String(!collapsed.has(group)));
            }
            sourcesEl.innerHTML = sources.filter(source => !source.ignored).map(source => `<span class="infra-source-chip" data-state="${esc(source.state || 'unknown')}"><i></i>${esc(t(`desktop.infra_source_${source.kind}`))} · ${esc(source.name || source.instance || source.kind)}</span>`).join('');
            loadingEl.hidden = !loading;
            emptyEl.hidden = loading || shown > 0;
            syncGroupChecks();
            updateCount();
            restoreFocus(snapshot);
        }

        // ---- loading ------------------------------------------------------------------------------------
        async function load(preserve) {
            const mine = owner, keep = preserve ? new Set(state.importSelection) : new Set();
            if (!preserve) loading = true;
            try {
                const result = await store.request('/candidates');
                if (disposed || !mine || owner !== mine) return;
                state.candidates = Array.isArray(result?.candidates) ? result.candidates : [];
                sources = Array.isArray(result?.sources) ? result.sources : [];
                const grouped = groups();
                state.importSelection.clear();
                for (const candidate of preserve ? state.candidates.filter(item => keep.has(item.id)) : grouped.recommended) state.importSelection.add(candidate.id);
                for (const candidate of grouped.duplicate) {
                    const id = grouped.relevance.get(candidate.id)?.duplicateOf?.nodeId;
                    if (id && !state.importBindings.has(candidate.id) && model.bindableFor(candidate, store.nodeById(id))) state.importBindings.set(candidate.id, id);
                }
            } catch (error) { if (error?.name !== 'AbortError') store.notify(store.messageFor(error), 'error'); }
            finally {
                if (owner === mine) { loading = false; render(); }
            }
        }
        function open() {
            if (!canWrite() || disposed) return;
            if (state.importOpen) { search.focus(); return; }
            opener = document.activeElement;
            state.importOpen = true;
            state.importDialogOwner = owner = `sheet-${++state.dialogGeneration}`;
            state.candidates = []; state.importSelection.clear(); state.importBindings.clear();
            errors.clear(); sources = []; search.value = ''; loading = true;
            el.hidden = false;
            root.dataset.importOpen = 'true';
            render();
            search.focus();
            void load(false);
        }
        function close({ restoreFocus: focus = true } = {}) {
            if (!state.importOpen && el.hidden) return;
            cancelAnimationFrame(searchFrame); searchFrame = 0;
            el.hidden = true;
            delete root.dataset.importOpen;
            state.importOpen = false; state.importDialogOwner = ''; owner = ''; pendingFocus = null;
            state.candidates = []; state.importSelection.clear(); state.importBindings.clear(); errors.clear();
            if (focus && opener?.isConnected && typeof opener.focus === 'function') opener.focus({ preventScroll: true });
            opener = null;
        }
        const reload = () => { if (state.importOpen) void load(true); };

        // ---- importing and ignoring ----------------------------------------------------------------------------
        // The ported one-by-one import for servers without POST /import/batch.
        async function importSequential(items) {
            const result = { nodes: [], errors: [], uncertain: false };
            if (!canWrite()) return result;
            store.setBusy('import', true);
            try {
                for (const item of items) {
                    try {
                        const answer = await store.request('/import', { method: 'POST', body: { candidate_id: item.candidate_id, ...(item.node_id ? { node_id: item.node_id } : {}) } });
                        const node = answer?.node || answer;
                        if (node?.id) result.nodes.push(...store.replaceNodes([node]));
                        state.candidates = state.candidates.filter(candidate => candidate.id !== item.candidate_id);
                    } catch (error) {
                        if (state.disposed || error?.name === 'AbortError') break;
                        if (store.uncertainMutation(error)) { result.uncertain = true; await store.uncertainReload(error); break; }
                        result.errors.push({ candidate_id: item.candidate_id, error: String(error?.body?.error || 'infra_error'), message: store.messageFor(error) });
                    }
                }
            } finally { store.setBusy('import', false); }
            return result;
        }
        // settle reports an import result: rows that left stop being selected, failures keep their row and
        // their message, and the sheet closes after a complete import when asked to.
        function settle(result, items, mine, { closeWhenDone = false } = {}) {
            if (disposed || state.disposed) return;
            const failed = new Map(result.errors.map(item => [item.candidate_id, item.message]));
            const imported = result.uncertain ? 0 : items.filter(item => !failed.has(item.candidate_id) && !state.candidates.some(candidate => candidate.id === item.candidate_id)).length;
            if (imported) store.notify(model.countText(imported, t('desktop.infra_imported_one'), t('desktop.infra_imported')), 'success');
            if (failed.size) app.announce(model.countText(failed.size, t('desktop.infra_import_partial_one'), t('desktop.infra_import_partial')));
            if (!mine || owner !== mine) return;
            for (const item of items) errors.delete(item.candidate_id);
            for (const [id, message] of failed) errors.set(id, message);
            if (result.uncertain) { void load(true); return; }
            if (closeWhenDone && !failed.size) { close(); return; }
            render();
        }
        async function runImport(items, options) {
            if (!canWrite() || !items.length || state.busy.has('import')) return;
            const mine = owner;
            const result = store.features.layoutParent ? await store.importBatch(items) : await importSequential(items);
            settle(result, items, mine, options);
        }
        const importSelected = () => runImport(selectedItems(), { closeWhenDone: true });
        function importOne(id) {
            const bound = state.importBindings.get(id);
            return runImport([{ candidate_id: id, ...(bound ? { node_id: bound } : {}) }], {});
        }
        async function ignoreCandidates(ids) {
            const list = ids.map(candidateById).filter(Boolean);
            if (!list.length || !canWrite() || state.busy.has('ignore')) return;
            const mine = owner;
            if (!await store.setSourcesIgnored(list.map(candidate => candidate.source), true)) return;
            app.announce(model.countText(list.length, t('desktop.infra_ignored_count_one'), t('desktop.infra_ignored_count')));
            if (owner !== mine) return;
            for (const candidate of list) { state.importSelection.delete(candidate.id); state.importBindings.delete(candidate.id); errors.delete(candidate.id); }
            render();
        }
        // dropOnCanvas imports the dragged discoveries and places them around the drop point; guests are
        // imported without a placement and join their host.
        async function dropOnCanvas(ids, world, region) {
            if (!canWrite() || !world || !state.importOpen) return false;
            const view = app.mapView(), list = ids.map(candidateById).filter(Boolean);
            if (!list.length) return false;
            const placeable = list.filter(candidate => !GUEST_KINDS.has(candidate.node?.kind));
            const points = app.gestures.positionsAt(placeable.length || 1, world);
            const parent = region && view === 'atlas' && store.features.layoutParent ? region : '';
            let next = 0;
            const items = list.map(candidate => {
                const item = { candidate_id: candidate.id, ...(state.importBindings.get(candidate.id) ? { node_id: state.importBindings.get(candidate.id) } : {}) };
                if (GUEST_KINDS.has(candidate.node?.kind)) return item;
                const point = points[next++];
                return { ...item, placement: { view, x: point.x, y: point.y }, ...(parent ? { parent_id: parent } : {}) };
            });
            const mine = owner;
            const result = await store.importBatch(items);
            settle(result, items, mine);
            const done = result.nodes.map(node => node.id);
            if (done.length) store.select(done);
            return done.length > 0;
        }

        // ---- events ------------------------------------------------------------------------------------------------
        function visibleIds() { return [...el.querySelectorAll('[data-candidate-row]')].map(row => row.dataset.candidateRow); }
        const onClick = event => {
            if (suppressClick) { suppressClick = false; return; }
            const target = event.target;
            if (target.closest('[data-action="close-import"]')) { close(); return; }
            const toggle = target.closest('[data-import-group-toggle]');
            if (toggle) {
                const group = toggle.dataset.importGroupToggle;
                if (collapsed.has(group)) collapsed.delete(group); else collapsed.add(group);
                render();
                return;
            }
            const one = target.closest('[data-import-one]');
            if (one) { if (!one.disabled) { pendingFocus = focusSnapshot(); void importOne(one.dataset.importOne); } return; }
            const ignore = target.closest('[data-ignore-candidate]');
            if (ignore) { if (!ignore.disabled) { pendingFocus = focusSnapshot(); void ignoreCandidates([ignore.dataset.ignoreCandidate]); } return; }
            if (target.closest('[data-action="ignore-selected"]')) { if (!ignoreButton.disabled) void ignoreCandidates([...state.importSelection]); return; }
            if (target.closest('[data-import]') && !importButton.disabled) void importSelected();
        };
        const onChange = event => {
            const box = event.target.closest?.('[data-candidate]');
            if (box) {
                if (box.checked) state.importSelection.add(box.dataset.candidate); else state.importSelection.delete(box.dataset.candidate);
                syncGroupChecks(); updateCount();
                return;
            }
            const groupBox = event.target.closest?.('[data-import-check-group]');
            if (groupBox) {
                for (const candidate of shownIn(groupBox.dataset.importCheckGroup)) { if (groupBox.checked) state.importSelection.add(candidate.id); else state.importSelection.delete(candidate.id); }
                for (const check of listEl[groupBox.dataset.importCheckGroup].querySelectorAll('[data-candidate]')) check.checked = groupBox.checked;
                syncGroupChecks(); updateCount();
                return;
            }
            const bind = event.target.closest?.('[data-bind]');
            if (bind) { if (bind.value) state.importBindings.set(bind.dataset.bind, bind.value); else state.importBindings.delete(bind.dataset.bind); }
        };
        // The long list of every other asset is only built when the control is used.
        const fillBinding = event => {
            const select = event.target.closest?.('[data-bind]');
            if (!select || select.dataset.full === 'true') return;
            const candidate = candidateById(select.dataset.bind);
            if (!candidate) return;
            const have = new Set([...select.options].map(option => option.value));
            const rest = state.nodes.filter(node => !have.has(node.id) && model.bindableFor(candidate, node)).sort((a, b) => model.naturalCompare(a.name, b.name));
            select.dataset.full = 'true';
            if (rest.length) select.insertAdjacentHTML('beforeend', `<optgroup label="${esc(t('desktop.infra_link_all'))}">${rest.map(node => optionHTML(node.id, false)).join('')}</optgroup>`);
        };
        const onPointerDown = event => {
            fillBinding(event);
            const row = event.target.closest?.('[data-candidate-row][data-draggable="true"]');
            if (!row || event.button !== 0 || event.target.closest('input, select, button, label, textarea, a')) return;
            const id = row.dataset.candidateRow, multi = (event.ctrlKey || event.metaKey || event.shiftKey) && state.importSelection.has(id);
            const ids = multi ? visibleIds().filter(item => state.importSelection.has(item)) : [id];
            const items = ids.map(candidateById).filter(Boolean).map(candidate => ({ icon: candidate.node?.icon, name: nameOfCandidate(candidate), guest: GUEST_KINDS.has(candidate.node?.kind) }));
            app.gestures.startLibraryDrag(event, ids, {
                mode: 'discovery', items,
                onDragged: () => { suppressClick = true; clearTimeout(suppressTimer); suppressTimer = setTimeout(() => { suppressClick = false; }, 0); },
            });
        };
        const onInput = event => {
            if (event.target !== search || searchFrame) return;
            searchFrame = requestAnimationFrame(() => { searchFrame = 0; render(); });
        };
        const onKeyDown = event => {
            if (event.key !== 'Escape' || root.classList.contains('is-dragging')) return;
            event.preventDefault(); event.stopPropagation();
            close();
        };
        el.addEventListener('click', onClick);
        el.addEventListener('change', onChange);
        el.addEventListener('focusin', fillBinding);
        el.addEventListener('pointerdown', onPointerDown);
        el.addEventListener('input', onInput);
        el.addEventListener('keydown', onKeyDown);

        const onBusy = key => { if (key === 'import' || key === 'ignore') updateCount(); };
        function dispose() {
            if (disposed) return;
            close({ restoreFocus: false });
            disposed = true;
            clearTimeout(suppressTimer);
            el.removeEventListener('click', onClick);
            el.removeEventListener('change', onChange);
            el.removeEventListener('focusin', fillBinding);
            el.removeEventListener('pointerdown', onPointerDown);
            el.removeEventListener('input', onInput);
            el.removeEventListener('keydown', onKeyDown);
            el.remove();
        }

        return { el, open, close, render, reload, dropOnCanvas, onBusy, dispose };
    }

    window.InfraAppImport = { create };
})();
