// EasyDrag start page: flow cards with mini preview, search and filters, card menu,
// template gallery, import and "new flow".
(function () {
    'use strict';

    const ED = window.EasyDrag = window.EasyDrag || {};
    const FILTER_KEY = 'aurago.easydrag.home.filter';
    const LIBRARY_KEY = 'aurago.easydrag.library';
    let libraryChanges;
    const FILTERS = ['all', 'active', 'inactive', 'drafts', 'errors'];
    function preferences(value) {
        const source = value && typeof value === 'object' ? value : {};
        const id = text => typeof text === 'string' && /^[A-Za-z0-9_-]{1,64}$/.test(text);
        const filters = Array.isArray(source.filters) ? source.filters : [];
        return {
            view: source.view === 'list' ? 'list' : 'cards',
            favorites: Array.from(new Set((Array.isArray(source.favorites) ? source.favorites : []).filter(id))).slice(0, 1000),
            filters: filters.filter(item => item && id(item.id) && typeof item.name === 'string' && item.name.trim() && Array.from(item.name).length <= 80).slice(0, 20).map(item => ({
                id: item.id, name: item.name.trim(), query: typeof item.query === 'string' ? Array.from(item.query).slice(0, 4000).join('') : '',
                filter: FILTERS.includes(item.filter) ? item.filter : 'all', sort: ['updated', 'last_run', 'name'].includes(item.sort) ? item.sort : 'updated', favorites: item.favorites === true,
                ...(item.folder === ':none' || id(item.folder) ? { folder: item.folder } : {}),
                ...(typeof item.tag === 'string' && item.tag && Array.from(item.tag).length <= 40 ? { tag: item.tag } : {})
            }))
        };
    }
    // IMPORT_MAX_BYTES is the server's request-body limit for flow documents (4 MiB). The document
    // itself may be at most 2 MiB (flows.MaxDocumentBytes); the server answers a larger one with
    // FLOW_TOO_LARGE.
    const IMPORT_MAX_BYTES = 4 * 1024 * 1024;
    const pauseOps = ED.libraryPause;
    const PAUSE_LIMIT = pauseOps.LIMIT;

    function previewSVG(nodes, esc, edges) {
        if (!nodes || !nodes.length) return '<svg viewBox="0 0 240 110" aria-hidden="true"><rect class="ed-mini-empty" x="94" y="39" width="52" height="32" rx="8"></rect></svg>';
        const xs = nodes.map(n => n.x);
        const ys = nodes.map(n => n.y);
        const minX = Math.min(...xs); const maxX = Math.max(...xs) + 232;
        const minY = Math.min(...ys); const maxY = Math.max(...ys) + 72;
        const scale = Math.min(220 / Math.max(maxX - minX, 1), 90 / Math.max(maxY - minY, 1), 0.35);
        const ox = (240 - (maxX - minX) * scale) / 2;
        const oy = (110 - (maxY - minY) * scale) / 2;
        const pathBetween = (p, n) => {
            const x1 = ox + (p.x - minX + 232) * scale; const y1 = oy + (p.y - minY + 36) * scale;
            const x2 = ox + (n.x - minX) * scale; const y2 = oy + (n.y - minY + 36) * scale;
            const dx = Math.max(6, Math.abs(x2 - x1) / 2);
            return '<path d="M' + x1.toFixed(1) + ' ' + y1.toFixed(1) + ' C' + (x1 + dx).toFixed(1) + ' ' + y1.toFixed(1) + ' ' + (x2 - dx).toFixed(1) + ' ' + y2.toFixed(1) + ' ' + x2.toFixed(1) + ' ' + y2.toFixed(1) + '"></path>';
        };
        let lines = '';
        if (Array.isArray(edges) && edges.length) {
            lines = edges.map(pair => {
                const a = Array.isArray(pair) ? nodes[pair[0]] : null;
                const b = Array.isArray(pair) ? nodes[pair[1]] : null;
                return a && b ? pathBetween(a, b) : '';
            }).join('');
        } else {
            const sorted = nodes.slice().sort((a, b) => a.x - b.x);
            lines = sorted.slice(1).map((n, i) => pathBetween(sorted[i], n)).join('');
        }
        const rects = nodes.map(n => '<rect data-cat="' + esc(String(n.category).startsWith('tool:') ? 'tool' : n.category) + '" x="' + (ox + (n.x - minX) * scale).toFixed(1) + '" y="' + (oy + (n.y - minY) * scale).toFixed(1) +
            '" width="' + (232 * scale).toFixed(1) + '" height="' + (72 * scale).toFixed(1) + '" rx="' + (10 * scale + 2).toFixed(1) + '"></rect>').join('');
        return '<svg viewBox="0 0 240 110" aria-hidden="true"><g class="ed-mini-wires">' + lines + '</g>' + rects + '</svg>';
    }

    // create builds the start page. opts.focusFlow: the flow the editor just left; its card takes
    // the focus once the list is loaded, if New flow still has it (showHome focused New flow).
    function create(app, opts) {
        const core = ED.core;
        if (!libraryChanges) libraryChanges = core.emitter();
        const { t, esc, ctx } = app;
        const bag = core.bag();
        let flows = [];
        let templates = [];
        let filter = core.storage.get(FILTER_KEY, 'all');
        let sortBy = 'updated';
        let query = '';
        let templateQuery = '';
        let templateCategory = '';
        let focusFlow = (opts && opts.focusFlow) || '';
        let prefs = preferences(core.storage.get(LIBRARY_KEY, {})), favoritesOnly = false, selectedFilter = '', volatilePrefs = false;
        let folders = [], foldersError = null, organizationFolder = '', organizationTag = '';
        let folderMap = new Map(), tagSet = new Set(), folderOptions = '', tagOptions = '', installedFolders = '', installedTags = '';
        let privacyUpdate = false;
        const selectedFlows = new Map();
        const destroyed = new AbortController();
        let activePause = null, pauseSequence = 0;
        const hasOrganization = !!(ED.metadata && app.api.folders && app.api.library);

        const el = core.el('<div class="ed-home">' +
            '<header class="ed-home-hero"><div class="ed-home-brand"><span class="ed-logo" aria-hidden="true">' + core.icon('git-branch') + '</span>' +
            '<div><h1>EasyDrag</h1><p>' + esc(t('easydrag.ui.home_tagline')) + '</p></div></div>' +
            '<div class="ed-home-actions"><button type="button" class="ed-btn" data-ed-import>' + core.icon('upload') + '<span>' + esc(t('easydrag.ui.home_import')) + '</span></button>' +
            '<button type="button" class="ed-btn" data-ed-pack-import>' + core.icon('upload') + '<span>' + esc(t('easydrag.ui.pack_import')) + '</span></button>' +
            '<button type="button" class="ed-btn" data-ed-approval-inbox>' + core.icon('check') + '<span>' + esc(t('easydrag.ui.approval_inbox')) + '</span></button>' +
            '<button type="button" class="ed-btn" data-ed-authoring>' + core.icon('sparkles') + '<span>' + esc(t('easydrag.ui.authoring_title')) + '</span></button>' +
            '<button type="button" class="ed-btn ed-btn--primary" data-ed-new>' + core.icon('plus') + '<span>' + esc(t('easydrag.ui.home_new')) + '</span></button>' +
            '<input type="file" accept=".json,.easydrag.json,application/json" hidden data-ed-import-file>' +
            '<input type="file" accept=".easydrag-pack.json,.json,application/json" hidden data-ed-pack-import-file></div></header>' +
            '<div class="ed-pack-status" data-ed-pack-status hidden role="status" aria-live="polite"></div>' +
            '<div class="ed-home-toolbar"><div class="ed-search">' + core.icon('search') + '<input type="search" maxlength="4000" placeholder="' + esc(t('easydrag.ui.home_search')) + '" aria-label="' + esc(t('easydrag.ui.home_search')) + '" enterkeyhint="search" inputmode="search" data-ed-home-search></div>' +
            '<div class="ed-seg ed-seg--small" role="radiogroup" aria-label="' + esc(t('easydrag.ui.home_filter')) + '">' +
            ['all', 'active', 'inactive', 'drafts', 'errors'].map(f => '<button type="button" role="radio" data-ed-filter="' + f + '" aria-checked="' + (f === filter) + '">' + esc(t('easydrag.ui.home_filter_' + f)) + '</button>').join('') + '</div>' +
            '<select class="ed-input ed-input--small ed-home-sort" aria-label="' + esc(t('easydrag.ui.home_sort')) + '" data-ed-home-sort>' +
            '<option value="updated">' + esc(t('easydrag.ui.home_sort_updated')) + '</option>' +
            '<option value="last_run">' + esc(t('easydrag.ui.home_sort_last_run')) + '</option>' +
            '<option value="name">' + esc(t('easydrag.ui.home_sort_name')) + '</option></select></div>' +
            '<section class="ed-home-section"><div class="ed-library-heading"><h2>' + esc(t('easydrag.ui.home_flows')) + '</h2><div class="ed-library-tools" data-ed-library-tools>' +
            '<div class="ed-seg ed-seg--small" role="radiogroup" aria-label="' + esc(t('easydrag.ui.library_view')) + '"><button type="button" role="radio" data-ed-library-view="cards" aria-checked="true">' + core.icon('grid') + esc(t('easydrag.ui.library_cards')) + '</button><button type="button" role="radio" data-ed-library-view="list" aria-checked="false">' + core.icon('list') + esc(t('easydrag.ui.library_list')) + '</button></div>' +
            '<button type="button" class="ed-btn" aria-pressed="false" data-ed-library-favorites>' + core.icon('star') + esc(t('easydrag.ui.library_favorites')) + '</button>' +
            '<details class="ed-details ed-library-saved"><summary>' + esc(t('easydrag.ui.library_saved_filters')) + '</summary><div class="ed-library-presets"><select class="ed-input" data-ed-library-filter aria-label="' + esc(t('easydrag.ui.library_saved_filters')) + '"></select><button type="button" class="ed-btn" data-ed-library-save>' + core.icon('plus') + esc(t('easydrag.ui.library_filter_save')) + '</button><button type="button" class="ed-icon-btn" data-ed-library-delete disabled aria-label="' + esc(t('easydrag.ui.library_filter_delete')) + '" title="' + esc(t('easydrag.ui.library_filter_delete')) + '">' + core.icon('trash') + '</button></div></details></div></div>' +
            '<div class="ed-library-selection" data-ed-bulk-selection role="group" aria-label="' + esc(t('easydrag.ui.home_bulk_review')) + '" hidden><span data-ed-bulk-count aria-live="polite"></span><button type="button" class="ed-btn" data-ed-bulk-review>' + core.icon('pause') + '<span>' + esc(t('easydrag.ui.home_bulk_review')) + '</span></button><button type="button" class="ed-btn ed-btn--ghost" data-ed-bulk-clear>' + esc(t('easydrag.ui.home_bulk_clear')) + '</button></div>' +
            '<p class="ed-hint" data-ed-library-local>' + esc(t('easydrag.ui.library_local')) + '</p><div class="ed-flow-grid"></div></section>' +
            '<section class="ed-home-section"><h2>' + esc(t('easydrag.ui.home_templates')) + '</h2><p class="ed-hint">' + esc(t('easydrag.ui.home_templates_hint')) + '</p>' +
            '<div class="ed-recipe-intro" data-ed-recipe-intro hidden><h2>' + esc(t('easydrag.ui.recipe_intro_title')) + '</h2><p>' + esc(t('easydrag.ui.recipe_intro_hint')) + '</p><div class="ed-recipe-steps">' + ['build', 'test', 'result'].map(id => '<span>' + esc(t('easydrag.ui.recipe_step_' + id)) + '</span>').join('') + '</div></div>' +
            '<div class="ed-recipe-tools"><div class="ed-search">' + core.icon('search') + '<input type="search" data-ed-template-search aria-label="' + esc(t('easydrag.ui.template_search')) + '" placeholder="' + esc(t('easydrag.ui.template_search')) + '"></div><select class="ed-input" data-ed-template-category aria-label="' + esc(t('easydrag.ui.template_category')) + '"><option value="">' + esc(t('easydrag.ui.template_all')) + '</option></select></div><div class="ed-template-grid"></div></section>' +
            '<div class="ed-sr-only" aria-live="polite" data-ed-home-live></div>' +
            '</div>');
        const grid = el.querySelector('.ed-flow-grid');
        const approvalUI = ED.approvals && ED.approvals.createHome(app, el);
        if (approvalUI) bag.add(() => approvalUI.dispose());
        const packUI = ED.pack && ED.pack.createHome(app, el, { canWrite: () => mayChange(), onChanged: () => reload() });
        if (packUI) bag.add(() => packUI.dispose());
        const organization = hasOrganization ? core.el('<details class="ed-details ed-library-organization"><summary>' + esc(t('easydrag.ui.organization_filters')) + '</summary><div class="ed-library-organization-controls"><label class="ed-label">' + esc(t('easydrag.ui.folder')) + '<select class="ed-input" data-ed-folder-filter></select></label><label class="ed-label">' + esc(t('easydrag.ui.tags')) + '<select class="ed-input" data-ed-tag-filter></select></label><button type="button" class="ed-btn" data-ed-folders-manage>' + core.icon('folder') + esc(t('easydrag.ui.folders_manage')) + '</button></div><p class="ed-hint" data-ed-folder-error hidden></p></details>') : null;
        if (organization) grid.parentElement.insertBefore(organization, grid);
        if (ED.blocks && app.openBlock) {
            const button = core.el('<button type="button" class="ed-btn ed-block-library-link">' + core.icon('grid') + esc(t('easydrag.ui.blocks_title')) + '</button>');
            button.addEventListener('click', () => app.openHome({ section: 'blocks' }));
            el.insertBefore(button, el.querySelector('.ed-home-toolbar'));
        }
        const tplGrid = el.querySelector('.ed-template-grid');
        const runtime = ED.durable && ED.durable.supported(app) ? ED.durable.observe(Object.assign({}, app, { root: el })) : null;
        if (runtime) { el.insertBefore(runtime.el, el.querySelector('.ed-home-toolbar')); bag.add(() => runtime.dispose()); }
        // The grid is no live region (every refresh would read all cards); meaningful changes
        // are announced on their own (a deleted flow).
        const voice = core.announcer(el.querySelector('[data-ed-home-live]'));

        function changePreferences(change) {
            const next = volatilePrefs ? preferences(prefs) : preferences(core.storage.get(LIBRARY_KEY, prefs));
            if (change(next) === false) return false;
            prefs = preferences(next);
            const saved = core.storage.set(LIBRARY_KEY, prefs);
            if (!saved && !volatilePrefs) ctx.notify({ title: 'EasyDrag', message: t('easydrag.ui.library_preferences_unavailable'), type: 'error' });
            volatilePrefs = !saved;
            if (saved) libraryChanges.emit('change', el);
            return true;
        }

        function metadataEnv() { return { root: el, api: app.api, ctx, t, esc, readonly: !!app.readonly, canWrite: mayChange,
            onAuthLost(err) { flows = []; templates = []; folders = []; clearSelection(); loadError = err; staleError = null; listed = false; cacheOrganization(); render(); reportLoaded(); },
            onLibraryRead(id, library, nextFolders) { const flow = flows.find(row => row.id === id); if (flow) flow.library = library; folders = nextFolders; cacheOrganization(); render(); }
        }; }
        function metadataOf(flow) { return flow.library || { folder_id: null, tags: [], revision: 0 }; }
        function cacheOrganization() {
            const nextFolders = new Map(folders.map(row => [row.id, row]));
            const nextTags = new Set(flows.flatMap(flow => { const meta = metadataOf(flow); return meta.tags_redacted || meta.tags_editable === false ? [] : meta.tags || []; }));
            if (loadError || flows.some(flow => metadataOf(flow).tags_redacted || metadataOf(flow).tags_editable === false) && Array.from(tagSet).some(tag => !nextTags.has(tag)) || folders.some(row => row.redacted && folderMap.get(row.id)?.name !== row.name)) privacyUpdate = true;
            folderMap = nextFolders; tagSet = nextTags;
            folderOptions = '<option value="">' + esc(t('easydrag.ui.folders_all')) + '</option><option value=":none">' + esc(t('easydrag.ui.folder_none')) + '</option>' + folders.map(row => '<option value="' + esc(row.id) + '">' + esc(row.name) + '</option>').join('');
            tagOptions = '<option value="">' + esc(t('easydrag.ui.tags_all')) + '</option>' + Array.from(tagSet).sort((a, b) => a.localeCompare(b)).map(tag => '<option value="' + esc('tag:' + tag) + '">' + esc(tag) + '</option>').join('');
        }
        function organizationControls() {
            if (!organization) return;
            organization.hidden = !!loadError;
            const folderSelect = organization.querySelector('[data-ed-folder-filter]');
            const nextFolders = folderOptions + (organizationFolder && organizationFolder !== ':none' && !folderMap.has(organizationFolder) ? '<option value="' + esc(organizationFolder) + '">' + esc(t('easydrag.ui.folder_missing')) + '</option>' : '');
            if (nextFolders !== installedFolders && (privacyUpdate || document.activeElement !== folderSelect)) { folderSelect.innerHTML = nextFolders; installedFolders = nextFolders; }
            if (privacyUpdate || document.activeElement !== folderSelect) folderSelect.value = organizationFolder;
            const tagSelect = organization.querySelector('[data-ed-tag-filter]'), missingTag = organizationTag && !tagSet.has(organizationTag), nextTags = tagOptions + (missingTag ? '<option value=":unavailable">' + esc(t('easydrag.ui.tags_missing')) + '</option>' : '');
            const hideOldTag = missingTag && tagSelect.value === 'tag:' + organizationTag;
            if (nextTags !== installedTags && (privacyUpdate || hideOldTag || document.activeElement !== tagSelect)) { tagSelect.innerHTML = nextTags; installedTags = nextTags; }
            if (privacyUpdate || hideOldTag || document.activeElement !== tagSelect) tagSelect.value = missingTag ? ':unavailable' : organizationTag ? 'tag:' + organizationTag : '';
            privacyUpdate = false;
            const error = organization.querySelector('[data-ed-folder-error]'); error.hidden = !foldersError; error.textContent = foldersError ? core.errorText(t, foldersError) : '';
            organization.querySelector('[data-ed-folders-manage]').disabled = !mayChange();
        }
        function metadataMarkup(flow) {
            if (!hasOrganization) return '';
            const meta = metadataOf(flow), folder = folderMap.get(meta.folder_id), tags = meta.tags || [];
            const parts = (meta.folder_id ? [folder ? folder.name : t('easydrag.ui.folder_missing')] : []).concat(tags);
            return !parts.length ? '' : '<p class="ed-flow-organization" title="' + esc(parts.join(' · ')) + '">' + esc(parts.slice(0, 3).join(' · ') + (parts.length > 3 ? ' …' : '')) + '</p>';
        }

        function libraryControls() {
            grid.classList.toggle('is-list', prefs.view === 'list');
            el.querySelector('[data-ed-library-tools]').hidden = !!loadError;
            el.querySelector('[data-ed-library-local]').hidden = !!loadError;
            el.querySelectorAll('[data-ed-library-view]').forEach(button => button.setAttribute('aria-checked', String(button.dataset.edLibraryView === prefs.view)));
            el.querySelector('[data-ed-library-favorites]').setAttribute('aria-pressed', String(favoritesOnly));
            const select = el.querySelector('[data-ed-library-filter]');
            const options = '<option value="">' + esc(t('easydrag.ui.library_current_filter')) + '</option>' + prefs.filters.map(item => '<option value="' + esc(item.id) + '">' + esc(item.name) + '</option>').join('');
            if (select.innerHTML !== options) select.innerHTML = options;
            select.value = selectedFilter;
            el.querySelector('[data-ed-library-delete]').disabled = !selectedFilter;
            selectionControls();
        }

        function selectionControls() {
            const controls = el.querySelector('[data-ed-bulk-selection]');
            if (!controls) return;
            controls.hidden = !selectedFlows.size || !!loadError;
            controls.querySelector('[data-ed-bulk-count]').textContent = t('easydrag.ui.home_bulk_selected', { count: selectedFlows.size });
            const locked = !mayChange() || !!activePause;
            controls.querySelector('[data-ed-bulk-review]').disabled = locked || !selectedFlows.size;
            controls.querySelector('[data-ed-bulk-clear]').disabled = !!activePause;
            grid.querySelectorAll('[data-ed-select-flow]').forEach(input => {
                const selected = selectedFlows.has(input.dataset.edSelectFlow);
                input.checked = selected;
                input.disabled = locked || (!selected && selectedFlows.size >= PAUSE_LIMIT);
                const card = input.closest('[data-ed-flow]');
                if (card) card.classList.toggle('is-bulk-selected', selected);
            });
        }

        function clearSelection() {
            if (!selectedFlows.size) return;
            selectedFlows.clear();
            selectionControls();
        }

        function pauseRowMarkup(row, final) {
            const state = ['checking', 'enabled', 'disabled', 'missing', 'mismatch', 'unavailable'].includes(row.state) ? row.state : 'unavailable';
            const label = final && row.outcome && ['confirmed', 'save_uncertain', 'active', 'unresolved', 'not_attempted'].includes(row.outcome)
                ? t('easydrag.ui.home_bulk_outcome_' + row.outcome) : t('easydrag.ui.home_bulk_' + (state === 'checking' ? 'loading' : 'status_' + (row.outcome && ['disabled', 'missing', 'mismatch', 'unavailable'].includes(row.outcome) ? row.outcome : state)));
            return '<li class="ed-bulk-pause-row" data-state="' + state + '"><span class="ed-bulk-pause-name">' + esc(row.name) + '</span><span class="ed-bulk-pause-state">' + esc(label) + '</span></li>';
        }

        function pauseDialogBody(rows, options) {
            const opts = options || {};
            const note = opts.note ? '<p class="ed-hint" data-ed-bulk-state-note>' + esc(t(opts.note)) + '</p>' : '';
            const progress = opts.progress ? '<p class="ed-bulk-pause-progress" role="status" aria-live="polite">' + esc(t('easydrag.ui.home_bulk_progress', opts.progress)) + '</p>' : '';
            const summary = opts.summary ? '<p class="ed-bulk-pause-summary" role="status">' + esc(t(opts.summary)) + '</p>' : '';
            const list = '<ul class="ed-bulk-pause-list">' + rows.map(row => pauseRowMarkup(row, !!opts.final)).join('') + '</ul>';
            if (opts.final) return summary + list + note;
            return '<p class="ed-bulk-pause-intro">' + esc(t('easydrag.ui.home_bulk_intro')) + '</p>' + summary + progress + note + list;
        }

        async function reviewPause() {
            if (!mayChange() || activePause || !selectedFlows.size || selectedFlows.size > PAUSE_LIMIT) return;
            const opener = el.querySelector('[data-ed-bulk-review]');
            const restorePauseFocus = () => {
                const anotherModalOpen = Array.from(document.querySelectorAll('.ed-modal-backdrop:not(.is-closing)')).some(overlay => !dialog || !overlay.contains(dialog.el));
                if (anotherModalOpen) return;
                const openerVisible = opener && opener.isConnected && !opener.disabled && opener.offsetParent !== null && !opener.closest('[hidden]');
                if (openerVisible) return;
                const active = document.activeElement;
                const activeInClosingDialog = operation && operation.closed && dialog && dialog.el.contains(active);
                if (active === opener || activeInClosingDialog || !active || active === document.body || !active.isConnected || active.closest('[hidden]')) {
                    const search = el.querySelector('[data-ed-home-search]');
                    if (search && !search.disabled) search.focus();
                    else el.querySelector('[data-ed-home-retry]')?.focus();
                }
            };
            const operation = { id: ++pauseSequence, cancel: new AbortController(), closed: false, running: true, executing: false };
            activePause = operation;
            selectionControls();
            const targets = Array.from(selectedFlows.values(), row => ({ id: row.id, name: row.name }));
            async function runPause() {
                try {
                    const result = await pauseOps.execute(app.api, operation.preview.rows, {
                        cancelSignal: operation.cancel.signal, destroySignal: destroyed.signal, cancelled: () => operation.closed,
                        canWrite: mayChange,
                        onProgress(current, total, target) {
                            if (operation.closed || gone || operation.id !== pauseSequence) return;
                            dialog.body.innerHTML = pauseDialogBody(operation.preview.rows, { progress: { current, total, name: target.name } });
                        }
                    });
                    if (!operation.closed && !gone && operation.id === pauseSequence) {
                        dialog.body.innerHTML = pauseDialogBody(result.rows, { final: true, note: 'easydrag.ui.home_bulk_state_note', summary: result.permissionLost ? 'easydrag.ui.home_bulk_permission_lost' : result.stopped ? 'easydrag.ui.home_bulk_stopped' : 'easydrag.ui.home_bulk_results' });
                        const close = dialog.el.querySelector('[data-ed-action="cancel"]');
                        if (close) close.querySelector('span').textContent = t('easydrag.ui.close');
                    }
                } catch (err) {
                    if (!operation.closed && !gone && operation.id === pauseSequence) {
                        dialog.body.innerHTML = pauseDialogBody(operation.preview.rows.map(row => ({ ...row, outcome: 'unresolved' })), { final: true, note: 'easydrag.ui.home_bulk_state_note', summary: 'easydrag.ui.home_bulk_stopped' });
                    }
                } finally {
                    operation.running = false;
                    if (operation.executing && !gone && !destroyed.signal.aborted) {
                        operation.refreshPromise = reload();
                        operation.refreshPromise.then(() => { if (operation.closed && !gone) restorePauseFocus(); }, () => { if (operation.closed && !gone) restorePauseFocus(); });
                    }
                    if (activePause === operation) activePause = null;
                    if (!operation.closed && !gone) selectionControls();
                    if (operation.closed && !gone) restorePauseFocus();
                }
            }
            const dialog = core.modal(el, {
                title: t('easydrag.ui.home_bulk_title'), closeLabel: t('easydrag.ui.close'), className: 'ed-modal--bulk-pause',
                body: pauseDialogBody(targets.map(row => ({ ...row, state: 'checking' })), { summary: 'easydrag.ui.home_bulk_loading' }),
                actions: [{ id: 'cancel', label: t('easydrag.ui.cancel') }], cancel: 'cancel',
                onAction: action => {
                    if (action !== 'confirm') return true;
                    if (operation.executing || !operation.preview || !mayChange()) return false;
                    operation.executing = true;
                    operation.running = true;
                    const confirm = dialog.el.querySelector('[data-ed-action="confirm"]');
                    if (confirm) confirm.hidden = true;
                    const cancel = dialog.el.querySelector('[data-ed-action="cancel"]');
                    if (cancel) setTimeout(() => { if (!operation.closed && !gone && !cancel.disabled) cancel.focus(); }, 0);
                    runPause();
                    return false;
                }
            });
            operation.dialog = dialog;
            dialog.done.then(() => {
                operation.closed = true;
                operation.cancel.abort();
                if (!operation.running && activePause === operation) activePause = null;
                if (!gone) {
                    selectionControls();
                    restorePauseFocus();
                }
            });
            try {
                const preview = await pauseOps.review(app.api, targets, {
                    signal: operation.cancel.signal, destroySignal: destroyed.signal,
                    onProgress(current, total, target) {
                        if (operation.closed || gone || operation.id !== pauseSequence) return;
                        dialog.body.innerHTML = pauseDialogBody(targets.map(row => ({ ...row, state: 'checking' })), { progress: { current, total, name: target.name } });
                    }
                });
                operation.preview = preview;
                if (!operation.closed && !gone && operation.id === pauseSequence) {
                    dialog.body.innerHTML = pauseDialogBody(preview.rows, { summary: preview.permissionLost ? 'easydrag.ui.home_bulk_permission_lost' : '' });
                    const candidates = preview.rows.filter(row => row.state === 'enabled').length;
                    const footer = dialog.el.querySelector('.ed-modal-foot');
                    if (candidates && !preview.permissionLost && footer) {
                        const confirm = core.el('<button type="button" class="ed-btn ed-btn--primary" data-ed-action="confirm"><span>' + esc(t('easydrag.ui.home_bulk_confirm')) + '</span></button>');
                        footer.insertBefore(confirm, footer.firstChild);
                    }
                }
            } finally {
                operation.running = false;
                if (operation.closed && activePause === operation) activePause = null;
                if (!operation.closed && !gone) selectionControls();
            }
        }

        function toggleFavorite(id) {
            if (!flows.some(flow => flow.id === id) || loadError) return;
            if (!changePreferences(next => {
                const at = next.favorites.indexOf(id);
                if (at >= 0) next.favorites.splice(at, 1);
                else if (next.favorites.length < 1000) next.favorites.push(id);
                else { ctx.notify({ title: 'EasyDrag', message: t('easydrag.ui.library_preferences_limit'), type: 'error' }); return false; }
            })) return;
            render();
            const button = Array.from(grid.querySelectorAll('[data-ed-favorite]')).find(item => item.dataset.edFavorite === id);
            (button || el.querySelector('[data-ed-library-favorites]')).focus();
        }

        async function saveFilter() {
            if (gone || loadError) return;
            if (organizationTag && !tagSet.has(organizationTag)) { ctx.notify({ title: 'EasyDrag', message: t('easydrag.ui.tags_missing'), type: 'error' }); return; }
            searchUpdate.cancel();
            const input = await ctx.promptDialog(t('easydrag.ui.library_filter_save'), '');
            if (input == null || gone || loadError) return;
            const name = input.trim();
            if (!name || Array.from(name).length > 80) { ctx.notify({ title: 'EasyDrag', message: t('easydrag.ui.library_filter_name_invalid'), type: 'error' }); return; }
            query = el.querySelector('[data-ed-home-search]').value;
            changePreferences(next => {
                const previous = next.filters.find(item => item.name.toLocaleLowerCase() === name.toLocaleLowerCase());
                if (!previous && next.filters.length >= 20) { ctx.notify({ title: 'EasyDrag', message: t('easydrag.ui.library_preferences_limit'), type: 'error' }); return false; }
                const item = { id: previous ? previous.id : core.randomID('filter_', 10), name, query, filter, sort: sortBy, favorites: favoritesOnly };
                if (organizationFolder) item.folder = organizationFolder;
                if (organizationTag) item.tag = organizationTag;
                if (previous) next.filters.splice(next.filters.indexOf(previous), 1, item); else next.filters.push(item);
                selectedFilter = item.id;
            });
            render();
        }

        function triggerLabels(f) {
            const seen = new Set();
            return (f.triggers || []).map(type => (app.catalog && app.catalog.types.get(type) || {}).label || type).filter(l => !seen.has(l) && seen.add(l));
        }

        // statusBadge uses the words of Mission Control: state_draft reads "Not published yet".
        function statusBadge(f) {
            if (!f.published) return '<span class="ed-chip ed-chip--muted">' + esc(t('easydrag.ui.state_draft')) + '</span>';
            if (f.has_unpublished_changes) return '<span class="ed-chip ed-chip--accent">' + esc(t('easydrag.ui.state_changes')) + '</span>';
            return '<span class="ed-chip ed-chip--ok">' + esc(t('easydrag.ui.state_published')) + '</span>';
        }

        // liveBadge: the published version no longer passes the server's current checks (its
        // start-up check); publishing again clears it.
        function liveBadge(f) {
            if (!f.published || !Array.isArray(f.live_issues) || !f.live_issues.length) return '';
            const hint = t('easydrag.ui.live_outdated_hint');
            return '<span class="ed-chip ed-chip--risk" data-ed-live-outdated title="' + esc(hint) + '">' + esc(t('easydrag.ui.state_live_outdated')) +
                '<span class="ed-sr-only"> ' + esc(hint) + '</span></span>';
        }

        function lastRun(f) {
            const r = f.last_run;
            if (!r) return '<span class="ed-muted">' + esc(t('easydrag.ui.home_never_ran')) + '</span>';
            return '<span class="ed-run-dot ed-run-dot--' + esc(r.status) + '"></span><span>' + esc(ED.durable ? ED.durable.label(t, r) : core.tr(t, 'easydrag.ui.status_' + r.status, r.status)) + ' · ' + esc(core.fmt.relative(r.started_at)) + (r.durable ? ' · ' + esc(t('easydrag.ui.durable_title')) : '') + '</span>';
        }

        function visibleFlows() {
            const q = query.trim().toLowerCase();
            const list = flows.filter(f => {
                const organization = metadataOf(f);
                if (organizationFolder === ':none' && organization.folder_id || organizationFolder && organizationFolder !== ':none' && organization.folder_id !== organizationFolder) return false;
                if (organizationTag && !(organization.tags || []).includes(organizationTag)) return false;
                if (favoritesOnly && !prefs.favorites.includes(f.id)) return false;
                // Names, descriptions and trigger labels already on the card. No catalog refetch.
                if (q && !String(f.name).toLowerCase().includes(q) && !String(f.description || '').toLowerCase().includes(q) &&
                    !triggerLabels(f).some(label => String(label).toLowerCase().includes(q))) return false;
                if (filter === 'active') return f.enabled;
                // "Paused" (the stored filter id stays "inactive"): published and switched off. A
                // flow that was never published is not paused; it shows under All.
                if (filter === 'inactive') return f.published && !f.enabled;
                if (filter === 'drafts') return !f.published || !!f.has_unpublished_changes;
                // Cancelled is not an error. Only last_run.status "error" counts.
                if (filter === 'errors') return f.last_run && f.last_run.status === 'error';
                return true;
            });
            if (sortBy === 'name') {
                list.sort((a, b) => String(a.name || '').localeCompare(String(b.name || '')));
            } else if (sortBy === 'last_run') {
                list.sort((a, b) => {
                    const ta = a.last_run && a.last_run.started_at ? Date.parse(a.last_run.started_at) || 0 : 0;
                    const tb = b.last_run && b.last_run.started_at ? Date.parse(b.last_run.started_at) || 0 : 0;
                    return tb - ta;
                });
            }
            return list;
        }

        // loadError replaces the grid: first load never succeeded, or a later answer withdrew the
        // list (FLOWS_DISABLED, or the session can no longer read). staleError keeps the last list
        // in memory only. Nothing is written to storage for either.
        let loadError = null;
        let staleError = null, importSelectionGeneration = 0;
        let timersRunning = true;  // GET /flows timers_running; false while Date & time triggers do not fire
        let listed = false;
        let spoken = '';
        // templateError is the failure of the last template load while no templates are known; the
        // template section shows it with its own Try again (retryTemplates).
        let templateError = null;
        let templateSeq = 0;

        function errorIdentity(err) {
            const body = err && err.body || {};
            return String(body.code || body.error || '').toLowerCase();
        }

        // authLoss: the session or the read scope is gone. Every 401 and every 403 counts,
        // whatever its code. desktop_readonly is the exception: the list stays, changes do not.
        function authLoss(err) {
            if (!err) return false;
            const status = Number(err.status) || 0;
            const code = errorIdentity(err);
            if (code === 'desktop_readonly') return false;
            if (status === 401 || status === 403) return true;
            return code === 'unauthorized' || code === 'forbidden' || code === 'permission_denied' || code === 'desktop_scope_required';
        }

        function writesRevoked(err) { return errorIdentity(err) === 'desktop_readonly'; }

        // createOff: no create, import, duplicate, delete, toggle or run. Read-only open stays.
        function createOff() {
            return !!app.readonly || !!staleError || core.errorCode(loadError) === 'FLOWS_DISABLED' || authLoss(loadError) || writesRevoked(loadError);
        }

        function blockReason() {
            if (!staleError) return '';
            return t('easydrag.ui.home_stale_locked', { reason: core.errorText(t, staleError) });
        }

        // errorCard offers the actions of the window's lock card (easydrag.js showError): "Open
        // settings" for FLOWS_DISABLED, and "Try again" (a new list request) for every error.
        function errorCard(err) {
            const retry = '<button type="button" class="ed-btn ed-btn--primary" data-ed-home-retry>' + core.icon('refresh') + '<span>' + esc(t('easydrag.ui.retry')) + '</span></button>';
            return core.errorCode(err) === 'FLOWS_DISABLED'
                ? '<div class="ed-home-empty" role="status">' + core.icon('lock') + '<h3>' + esc(t('easydrag.ui.disabled_title')) + '</h3><p>' + esc(t('easydrag.ui.disabled_text')) + '</p>' +
                    '<div class="ed-row"><button type="button" class="ed-btn" data-ed-home-settings>' + core.icon('settings') + '<span>' + esc(t('easydrag.ui.open_settings')) + '</span></button>' + retry + '</div></div>'
                : '<p class="ed-error" role="alert">' + esc(core.errorText(t, err)) + '</p><div class="ed-row ed-home-retry">' + retry + '</div>';
        }

        function staleBanner() {
            return '<div class="ed-home-stale" role="status"><h3>' + esc(t('easydrag.ui.home_stale_title')) + '</h3><p>' +
                esc(t('easydrag.ui.home_stale_text', { reason: core.errorText(t, staleError) })) + '</p>' +
                '<button type="button" class="ed-btn" data-ed-home-retry>' + core.icon('refresh') + '<span>' + esc(t('easydrag.ui.retry')) + '</span></button></div>';
        }

        // timersBanner: Date & time triggers do not fire (the server keeps retrying to start).
        function timersBanner() {
            if (timersRunning) return '';
            return '<div class="ed-home-stale" role="status" data-ed-timers-off><h3>' + esc(t('easydrag.ui.timers_stopped_title')) + '</h3><p>' +
                esc(t('easydrag.ui.timers_stopped_text')) + '</p></div>';
        }

        function emptyCard() {
            const off = createOff();
            const dis = off ? ' disabled' : '';
            const why = off && blockReason() ? ' title="' + esc(blockReason()) + '"' : '';
            return '<div class="ed-home-empty"><h3>' + esc(t('easydrag.ui.home_empty_title')) + '</h3><p>' + esc(t('easydrag.ui.home_empty_text')) + '</p>' +
                '<div class="ed-empty-actions">' +
                '<button type="button" class="ed-btn" data-ed-home-templates' + dis + why + '>' + core.icon('sparkles') + '<span>' + esc(t('easydrag.ui.home_empty_template')) + '</span></button>' +
                '<button type="button" class="ed-btn ed-btn--primary" data-ed-new' + dis + why + '>' + core.icon('plus') + '<span>' + esc(t('easydrag.ui.home_empty_create')) + '</span></button>' +
                '<button type="button" class="ed-btn" data-ed-import' + dis + why + '>' + core.icon('upload') + '<span>' + esc(t('easydrag.ui.home_import')) + '</span></button>' +
                '</div></div>';
        }

        function noMatchCard() {
            let actions = '';
            if (query.trim()) actions += '<button type="button" class="ed-btn" data-ed-home-clear-search>' + esc(t('easydrag.ui.home_clear_search')) + '</button>';
            if (filter !== 'all' || favoritesOnly || organizationFolder || organizationTag) actions += '<button type="button" class="ed-btn" data-ed-home-clear-filter>' + esc(t('easydrag.ui.home_clear_filter')) + '</button>';
            return '<div class="ed-home-empty"><p class="ed-hint">' + esc(t('easydrag.ui.home_no_match')) + '</p><div class="ed-empty-actions">' + actions + '</div></div>';
        }

        function speak(mode, text) {
            if (spoken === mode) return;
            spoken = mode;
            const live = el.querySelector('[data-ed-home-live]');
            if (live) live.textContent = text;
        }

        // reportLoaded tells the shell the list has settled, so the window menu can follow createOff.
        function reportLoaded() {
            if (opts && typeof opts.onLoaded === 'function') opts.onLoaded();
        }

        // cardName is a flow card's accessible name (U7): its name and the state the card shows
        // (state, Paused, a failed last run).
        function cardName(f) {
            const parts = [f.name, !f.published ? t('easydrag.ui.state_draft') : f.has_unpublished_changes ? t('easydrag.ui.state_changes') : t('easydrag.ui.state_published')];
            if (f.published && !f.enabled) parts.push(t('easydrag.ui.state_inactive'));
            if (f.last_run && f.last_run.status === 'error') parts.push(t('easydrag.ui.status_error'));
            return parts.join(', ');
        }

        function render() {
            const list = visibleFlows();
            const visiblePublished = new Set(list.filter(flow => flow.published).map(flow => flow.id));
            for (const id of selectedFlows.keys()) if (!visiblePublished.has(id)) selectedFlows.delete(id);
            libraryControls();
            organizationControls();
            const off = createOff();
            const banner = (!loadError && staleError ? staleBanner() : '') + (!loadError ? timersBanner() : '');
            if (loadError) {
                grid.innerHTML = errorCard(loadError);
                speak(core.errorCode(loadError) === 'FLOWS_DISABLED' ? 'disabled' : 'error', core.errorCode(loadError) === 'FLOWS_DISABLED' ? t('easydrag.ui.disabled_title') : core.errorText(t, loadError));
            } else if (!flows.length) {
                grid.innerHTML = banner + emptyCard();
                speak(staleError ? 'stale-empty' : 'empty', staleError ? t('easydrag.ui.home_stale_title') : t('easydrag.ui.home_empty_title'));
            } else if (!list.length) {
                grid.innerHTML = banner + noMatchCard();
                speak(staleError ? 'stale-none' : 'none', t('easydrag.ui.home_no_match'));
            } else {
                grid.innerHTML = banner + list.map(f =>
                    '<article class="ed-flow-card' + (f.enabled ? ' is-active' : '') + (f.published && !off ? ' is-bulk-selectable' : '') + (selectedFlows.has(f.id) ? ' is-bulk-selected' : '') + '" data-ed-flow="' + esc(f.id) + '" tabindex="0" aria-label="' + esc(cardName(f)) + '">' +
                    (f.published && !off ? '<label class="ed-flow-select" data-ed-select-label="' + esc(f.id) + '"><input type="checkbox" data-ed-select-flow="' + esc(f.id) + '" aria-label="' + esc(t('easydrag.ui.home_bulk_select_flow', { name: f.name })) + '"' + (selectedFlows.has(f.id) ? ' checked' : '') + '><span class="ed-sr-only">' + esc(t('easydrag.ui.home_bulk_select_flow', { name: f.name })) + '</span></label>' : '') +
                    '<div class="ed-flow-preview">' + previewSVG(f.preview, esc, f.preview_edges) + '</div>' +
                    '<div class="ed-flow-info"><h3>' + esc(f.name) + '</h3><p class="ed-flow-triggers">' + core.icon('bolt') + '<span>' + esc(triggerLabels(f).join(' · ') || t('easydrag.ui.home_no_trigger')) + '</span></p>' +
                    '<div class="ed-flow-meta">' + statusBadge(f) + liveBadge(f) + '<span class="ed-flow-run">' + lastRun(f) + '</span></div>' + metadataMarkup(f) + '</div>' +
                    '<div class="ed-flow-card-actions">' +
                    '<button type="button" class="ed-icon-btn ed-flow-favorite" data-ed-favorite="' + esc(f.id) + '" aria-pressed="' + prefs.favorites.includes(f.id) + '" aria-label="' + esc(t(prefs.favorites.includes(f.id) ? 'easydrag.ui.library_favorite_remove' : 'easydrag.ui.library_favorite_add', { name: f.name })) + '" title="' + esc(t(prefs.favorites.includes(f.id) ? 'easydrag.ui.library_favorite_remove' : 'easydrag.ui.library_favorite_add', { name: f.name })) + '">' + core.icon('star') + '</button>' +
                    (f.published && !off ? '<button type="button" class="ed-switch ed-switch--small" role="switch" aria-checked="' + !!f.enabled + '" data-ed-toggle="' + esc(f.id) + '" aria-label="' + esc(t('easydrag.ui.home_active_named', { name: f.name })) + '" title="' + esc(t('easydrag.ui.active')) + '"><span></span></button>' : '') +
                    '<button type="button" class="ed-icon-btn" data-ed-card-menu="' + esc(f.id) + '" aria-label="' + esc(t('easydrag.ui.more')) + '">' + core.icon('dots') + '</button></div></article>').join('');
                speak(staleError ? 'stale' : 'ready', staleError ? t('easydrag.ui.home_stale_title') : t('easydrag.ui.home_flows'));
            }
            tplGrid.classList.toggle('is-disabled', off);
            // Templates that could not be loaded say so here, with their own Try again; the flows above stay.
            if (templateError && !templates.length && !loadError) {
                tplGrid.innerHTML = '<p class="ed-error">' + esc(t('easydrag.ui.home_templates_failed', { reason: core.errorText(t, templateError) })) + '</p>' +
                    '<div class="ed-row"><button type="button" class="ed-btn" data-ed-templates-retry>' + core.icon('refresh') + '<span>' + esc(t('easydrag.ui.retry')) + '</span></button></div>';
            } else tplGrid.innerHTML = templates.filter(tp => (!templateCategory || (tp.categories || []).includes(templateCategory)) &&
                (!templateQuery || (tp.name + ' ' + tp.description).toLocaleLowerCase().includes(templateQuery))).map(tp =>
                '<article class="ed-template-card" data-ed-template="' + esc(tp.id) + '" tabindex="' + (off ? '-1' : '0') + '" role="button"' + (off ? ' aria-disabled="true"' : '') + ' aria-label="' + esc(tp.name) + '">' +
                '<div class="ed-template-cats">' + (tp.categories || []).map(c => '<span class="ed-cat-dot" data-cat="' + esc(c) + '"></span>').join('') + '</div>' +
                '<h3>' + esc(tp.name) + '</h3><p>' + esc(tp.description) + '</p><dl class="ed-recipe-meta"><div><dt>' + esc(t('easydrag.ui.recipe_requirements')) + '</dt><dd>' + esc(t(tp.local_only ? 'easydrag.ui.recipe_local' : 'easydrag.ui.recipe_setup_review')) + '</dd></div>' +
                (tp.effects_known ? (tp.effects || []).length ? '<div><dt>' + esc(t('easydrag.ui.recipe_effects')) + '</dt><dd>' + (tp.effects || []).map(effect => esc(core.tr(t, 'easydrag.ui.effect_' + effect, effect))).join(' · ') + '</dd></div>' : '' : '<div><dd>' + esc(t('easydrag.ui.effects_unchecked')) + '</dd></div>') + '</dl><span class="ed-template-use">' + esc(t('easydrag.ui.home_use_template')) + core.icon('chevron-right') + '</span></article>').join('') || '<p class="ed-hint">' + esc(t('easydrag.ui.input_none')) + '</p>';
            el.querySelector('[data-ed-recipe-intro]').hidden = !!loadError || !templates.some(tp => tp.id === 'local_first_result' && tp.local_only);
            const categories = el.querySelector('[data-ed-template-category]');
            const categoryOptions = '<option value="">' + esc(t('easydrag.ui.template_all')) + '</option>' + Array.from(new Set(templates.flatMap(tp => tp.categories || []))).sort().map(c => '<option value="' + esc(c) + '">' + esc(core.tr(t, 'easydrag.category.' + c, c)) + '</option>').join('');
            if (categories.innerHTML !== categoryOptions) { categories.innerHTML = categoryOptions; categories.value = templateCategory; }
            el.querySelectorAll('[data-ed-new]').forEach(b => { b.disabled = off; });
            el.querySelectorAll('[data-ed-import], [data-ed-pack-import]').forEach(b => { b.disabled = off; });
            el.querySelectorAll('[data-ed-authoring]').forEach(b => { b.disabled = off; });
            el.querySelector('[data-ed-home-search]').disabled = !!loadError;
            const sortSel = el.querySelector('[data-ed-home-sort]');
            if (sortSel) sortSel.disabled = !!loadError;
            el.querySelectorAll('[data-ed-filter]').forEach(b => { b.disabled = !!loadError; });
            selectionControls();
        }

        // loadSeq numbers the list requests: only the newest answer is shown, none after dispose.
        let loadSeq = 0;

        // reload fetches the flow list and, once, the templates. Each answer stands on its own
        // (allSettled): templates that fail leave the flows listed and show their own Try again in
        // the template section. A list that fails is shown and not retried by itself: the next
        // flows_changed or the user (Reload) asks again. Flows switched off (FLOWS_DISABLED,
        // possible at any time) show a calm state instead of an error.
        async function reload() {
            reloadSoon.cancel();
            const mine = ++loadSeq;
            const [listAnswer, tplAnswer, folderAnswer] = await Promise.allSettled([app.api.list(), templates.length ? Promise.resolve({ templates }) : app.api.templates(), hasOrganization ? app.api.folders() : Promise.resolve({ folders: [] })]);
            if (mine !== loadSeq) return;
            if (folderAnswer.status === 'rejected' && (Number(folderAnswer.reason?.status) === 401 || Number(folderAnswer.reason?.status) === 403)) {
                metadataEnv().onAuthLost(folderAnswer.reason);
                return;
            }
            if (listAnswer.status === 'rejected') {
                const err = listAnswer.reason;
                // A refresh that fails keeps the last authoritative list and blocks changes.
                // The first load, FLOWS_DISABLED and a lost session do not keep protected cards.
                if (authLoss(err)) {
                    folders = [];
                    flows = [];
                    clearSelection();
                    templates = [];
                    templateError = null;
                    listed = false;
                    staleError = null;
                    loadError = err;
                } else if (core.errorCode(err) === 'FLOWS_DISABLED' || !listed) {
                    flows = [];
                    clearSelection();
                    listed = false;
                    staleError = null;
                    loadError = err;
                } else {
                    staleError = err;
                    loadError = null;
                }
                cacheOrganization();
                render();
                reportLoaded();
                return;
            }
            const list = listAnswer.value || {};
            loadError = null;
            staleError = null;
            listed = true;
            flows = list.flows || [];
            if (folderAnswer.status === 'fulfilled') { folders = folderAnswer.value.folders || []; foldersError = null; }
            else { foldersError = folderAnswer.reason; if (authLoss(foldersError)) folders = []; }
            cacheOrganization();
            timersRunning = list.timers_running !== false;
            if (tplAnswer.status === 'fulfilled') {
                templates = (tplAnswer.value && tplAnswer.value.templates) || [];
                templateError = null;
            } else if (!templates.length) {
                templateError = tplAnswer.reason;
            }
            render();
            reportLoaded();
            focusLeftFlow();
        }

        // retryTemplates loads the templates again (their Try again). Focus that was on the old button
        // goes to the new Try again, or to the first template.
        async function retryTemplates() {
            const mine = ++templateSeq;
            const hadFocus = tplGrid.contains(document.activeElement);
            let next = null;
            let failure = null;
            try { next = await app.api.templates(); } catch (err) { failure = err; }
            if (gone || mine !== templateSeq) return;
            if (failure) templateError = failure;
            else { templates = (next && next.templates) || []; templateError = null; }
            render();
            const target = tplGrid.querySelector('[data-ed-templates-retry]') || tplGrid.querySelector('[data-ed-template]');
            if (hadFocus && target && typeof target.focus === 'function') target.focus();
        }
        // Every autosave broadcasts flows_changed "saved", and every live run's end "run_finished";
        // listing parses every flow on the server: those refreshes wait until they pause for 1.5 s.
        const reloadSoon = core.debounce(reload, 1500);

        // focusLeftFlow moves the focus to the card of the flow the editor just left, once, and only
        // from New flow (where showHome put it) or from nowhere: never away from a control the user
        // chose meanwhile.
        function focusLeftFlow() {
            const id = focusFlow;
            focusFlow = '';
            if (!id) return;
            const card = Array.from(grid.querySelectorAll('[data-ed-flow]')).find(c => c.dataset.edFlow === id);
            const active = document.activeElement;
            const add = el.querySelector('.ed-home-hero [data-ed-new]');
            if (card && (!active || active === document.body || active === add)) card.focus();
        }

        // retry is "Try again" on an error card. The new grid replaces the button, so the focus goes
        // to the new "Try again" or, once the flows are back, to the first card. It moves only when
        // it was lost with the old button (on the body, or on the removed button): never away from
        // a control the user moved to meanwhile, here or in another window.
        async function retry() {
            await reload();
            const active = document.activeElement;
            if (active && active !== document.body && active.isConnected !== false) return;
            const next = grid.querySelector('[data-ed-home-retry]') || grid.querySelector('[data-ed-flow]') || el.querySelector('[data-ed-new]');
            if (next && !next.disabled) next.focus();
        }

        // creating allows one create at a time (a held Enter on a template card, a double click).
        // gone is dispose. A confirmation, file read or duplicate fetch can outlive the authority
        // it started with, so the mutation checks again at the request.
        let creating = false;
        let gone = false;
        const runsPending = new Set();
        function mayChange() { return !gone && !createOff(); }
        const authoring = ED.authoring && ED.authoring.createNew(app, el, mayChange);
        if (authoring) bag.add(() => authoring.dispose());

        async function create(body, openOptions) {
            if (!mayChange() || creating) return;
            creating = true;
            try {
                const res = await app.api.create(body);
                app.openFlow(res.flow.id, openOptions);
            } catch (err) {
                ctx.notify({ title: 'EasyDrag', message: core.errorText(t, err), type: 'error' });
            } finally {
                creating = false;
            }
        }

        async function newFlow() {
            if (createOff()) return;
            const name = await ctx.promptDialog(t('easydrag.ui.home_new_title'), t('easydrag.ui.new_flow_name'));
            if (name === null || name === undefined) return;
            create({ name: String(name).trim() || t('easydrag.ui.new_flow_name') });
        }

        // Pack suffixes are classified before parsing. Generic JSON is read once (up to the 32 MiB
        // pack cap) so a format marker cannot fall through to legacy flow creation.
        async function importFile(file, packOnly) {
            const selection = ++importSelectionGeneration;
            const current = () => selection === importSelectionGeneration && !gone && mayChange();
            if (!mayChange() || !file) return;
            if (packUI) {
                const result = await packUI.readImportFile(file);
                if (!current() || !result || result.kind === 'cancelled') return;
                if (result.kind === 'pack') { packUI.review(result.pack, result.name); return; }
                if (result.kind === 'pack-invalid' || result.kind === 'pack-too-large' || result.kind === 'pack-unsupported') {
                    const key = result.kind === 'pack-invalid' ? 'pack_invalid' : result.kind === 'pack-too-large' ? 'pack_too_large' : 'error_flow_pack_unsupported';
                    ctx.notify({ title: t('easydrag.ui.pack_import'), message: t('easydrag.ui.' + key), type: 'error' });
                    return;
                }
                if (result.kind === 'legacy-too-large') {
                    const key = packOnly ? 'pack_too_large' : 'error_flow_too_large';
                    ctx.notify({ title: t(packOnly ? 'easydrag.ui.pack_import' : 'easydrag.ui.home_import'), message: t('easydrag.ui.' + key), type: 'error' });
                    return;
                }
                if (packOnly) {
                    ctx.notify({ title: t('easydrag.ui.pack_import'), message: t('easydrag.ui.pack_invalid'), type: 'error' });
                    return;
                }
                if (result.kind === 'legacy-invalid') {
                    ctx.notify({ title: t('easydrag.ui.home_import'), message: t('easydrag.ui.import_invalid'), type: 'error' });
                    return;
                }
                create({ import: result.doc });
                return;
            }
            if (Number(file.size) > IMPORT_MAX_BYTES) {
                ctx.notify({ title: t('easydrag.ui.home_import'), message: t('easydrag.ui.error_flow_too_large'), type: 'error' });
                return;
            }
            let doc;
            try { doc = JSON.parse(await file.text()); } catch (err) {
                if (current()) ctx.notify({ title: t('easydrag.ui.home_import'), message: t('easydrag.ui.import_invalid'), type: 'error' });
                return;
            }
            if (!current()) return;
            create({ import: doc });
        }

        function download(id, name) {
            const a = document.createElement('a');
            a.href = app.api.exportUrl(id);
            a.download = (name || 'flow') + '.easydrag.json';
            document.body.appendChild(a);
            a.click();
            a.remove();
        }

        // duplicating holds the flows whose copy is being made: a double click copies once.
        const duplicating = new Set();
        async function duplicate(id) {
            if (!mayChange() || duplicating.has(id)) return;
            duplicating.add(id);
            try {
                const res = await app.api.get(id);
                if (!mayChange()) return;
                const doc = res.flow.draft;
                doc.name = t('easydrag.ui.copy_of', { name: doc.name });
                await create({ import: doc });
            } catch (err) {
                if (!gone) ctx.notify({ title: 'EasyDrag', message: core.errorText(t, err), type: 'error' });
            } finally {
                duplicating.delete(id);
            }
        }

        async function remove(f) {
            if (!mayChange() || !f) return;
            const ok = await ctx.confirmDialog(t('easydrag.ui.delete_title'), t('easydrag.ui.delete_text', { name: f.name }));
            if (!ok || !mayChange()) return;
            try { await app.api.remove(f.id); } catch (err) { if (!gone) ctx.notify({ title: 'EasyDrag', message: core.errorText(t, err), type: 'error' }); return; }
            // The flow's local leftovers go with it: its emergency copy, its view, its confirmed
            // test effects and its last test trigger.
            core.forgetFlow(f.id);
            voice.announce(t('easydrag.ui.home_flow_deleted', { name: f.name }));
            reload();
        }

        // runNow allows one request per flow until that request settles. A failure is not sent again.
        async function runNow(f) {
            if (!mayChange() || !f || !f.published || !f.enabled || runsPending.has(f.id)) return;
            runsPending.add(f.id);
            try {
                const res = await app.api.runNow(f.id);
                // Concurrency "skip" with a run still going starts nothing (202 {status: "skipped"}).
                const skipped = !!(res && res.status === 'skipped' && !res.run_id);
                if (!gone) ctx.notify({ title: f.name, message: t(skipped ? 'easydrag.ui.run_now_skipped' : 'easydrag.ui.run_started') });
            } catch (err) {
                if (!gone) ctx.notify({ title: f.name, message: core.errorText(t, err), type: 'error' });
            } finally {
                runsPending.delete(f.id);
            }
        }

        async function toggle(f, button) {
            if (createOff() || !f || !button) return;
            const on = button.getAttribute('aria-checked') !== 'true';
            button.setAttribute('aria-checked', String(on));
            try { await app.api.setEnabled(f.id, on); f.enabled = on; render(); } catch (err) {
                button.setAttribute('aria-checked', String(!on));
                ctx.notify({ title: f.name, message: core.errorText(t, err), type: 'error' });
            }
        }

        function cardMenu(f, x, y) {
            if (!f || typeof ctx.showContextMenu !== 'function') return;
            const locked = createOff();
            // Run now needs a published flow that is switched on (a paused one is refused).
            const paused = !!f.published && !f.enabled;
            const why = blockReason();
            ctx.showContextMenu(x, y, [
                { icon: 'edit', label: t('easydrag.ui.home_open'), action: () => { clearSelection(); app.openFlow(f.id); } },
                ...(hasOrganization ? [{ icon: 'folder', label: t('easydrag.ui.organize'), disabled: locked, disabledHint: why, action: () => ED.metadata.organization(metadataEnv(), f.id, reload) }] : []),
                { icon: 'play', label: t('easydrag.ui.run_now'), disabled: locked || !f.published || paused || runsPending.has(f.id), disabledHint: why || (paused && !app.readonly ? t('easydrag.ui.error_flow_disabled') : ''), action: () => runNow(f) },
                { icon: 'copy', label: t('easydrag.ui.home_duplicate'), disabled: locked, disabledHint: why, action: () => duplicate(f.id) },
                { icon: 'download', label: t('easydrag.ui.home_export'), action: () => download(f.id, f.name) },
                { icon: 'download', label: t('easydrag.ui.pack_export'), action: () => packUI && packUI.exportStored(f.id, f.name) },
                { icon: 'list', label: t('easydrag.ui.home_mission_control'), action: () => ctx.openApp && ctx.openApp('mission-control') },
                { separator: true },
                { icon: 'trash', label: t('easydrag.ui.home_delete'), disabled: locked, disabledHint: why, action: () => remove(f) }
            ]);
        }

        // focusTemplates moves to the existing template section. It never creates a flow.
        function focusTemplates() {
            const section = tplGrid.parentElement;
            if (section && typeof section.scrollIntoView === 'function') section.scrollIntoView({ block: 'start' });
            const head = section && section.querySelector('h2');
            const card = !createOff() ? tplGrid.querySelector('[data-ed-template]') : null;
            const target = card || head || section;
            if (target === head && head && typeof head.setAttribute === 'function') head.setAttribute('tabindex', '-1');
            if (target && typeof target.focus === 'function') target.focus();
        }

        function clearSearch() {
            searchUpdate.cancel();
            clearSelection();
            selectedFilter = '';
            query = '';
            const input = el.querySelector('[data-ed-home-search]');
            if (input) {
                input.value = '';
                if (typeof input.focus === 'function') input.focus();
            }
            render();
        }

        function clearFilter() {
            clearSelection();
            selectedFilter = ''; favoritesOnly = false;
            organizationFolder = ''; organizationTag = '';
            filter = 'all';
            core.storage.set(FILTER_KEY, filter);
            el.querySelectorAll('[data-ed-filter]').forEach(b => b.setAttribute('aria-checked', String(b.dataset.edFilter === 'all')));
            render();
            const all = el.querySelector('[data-ed-filter="all"]');
            if (all && typeof all.focus === 'function') all.focus();
        }

        bag.listen(el, 'click', (event) => {
            if (event.target.closest('[data-ed-select-label], [data-ed-select-flow]')) { event.stopPropagation(); return; }
            if (loadError && event.target.closest('[data-ed-library-tools]')) return;
            if (event.target.closest('[data-ed-bulk-review]')) { reviewPause(); return; }
            if (event.target.closest('[data-ed-bulk-clear]')) { clearSelection(); return; }
            const view = event.target.closest('[data-ed-library-view]');
            if (view) { changePreferences(next => { next.view = view.dataset.edLibraryView; }); libraryControls(); return; }
            if (event.target.closest('[data-ed-library-favorites]')) { clearSelection(); favoritesOnly = !favoritesOnly; selectedFilter = ''; render(); return; }
            if (event.target.closest('[data-ed-library-save]')) { saveFilter(); return; }
            if (event.target.closest('[data-ed-library-delete]')) { changePreferences(next => { next.filters = next.filters.filter(item => item.id !== selectedFilter); }); selectedFilter = ''; libraryControls(); return; }
            const favorite = event.target.closest('[data-ed-favorite]');
            if (favorite) { toggleFavorite(favorite.dataset.edFavorite); return; }
            if (event.target.closest('[data-ed-home-settings]')) { core.openFlowSettings(); return; }
            if (event.target.closest('[data-ed-templates-retry]')) { retryTemplates(); return; }
            if (event.target.closest('[data-ed-home-retry]')) { retry(); return; }
            if (event.target.closest('[data-ed-home-templates]')) { focusTemplates(); return; }
            if (event.target.closest('[data-ed-home-clear-search]')) { clearSearch(); return; }
            if (event.target.closest('[data-ed-home-clear-filter]')) { clearFilter(); return; }
            if (event.target.closest('[data-ed-new]')) { newFlow(); return; }
            if (event.target.closest('[data-ed-authoring]')) { if (authoring) authoring.open(); return; }
            if (event.target.closest('[data-ed-pack-import]')) { el.querySelector('[data-ed-pack-import-file]').click(); return; }
            if (event.target.closest('[data-ed-import]')) { el.querySelector('[data-ed-import-file]').click(); return; }
            const f = event.target.closest('[data-ed-filter]');
            if (f) {
                clearSelection();
                selectedFilter = '';
                filter = f.dataset.edFilter;
                core.storage.set(FILTER_KEY, filter);
                el.querySelectorAll('[data-ed-filter]').forEach(b => b.setAttribute('aria-checked', String(b === f)));
                render();
                return;
            }
            const tg = event.target.closest('[data-ed-toggle]');
            if (tg) { event.stopPropagation(); toggle(flows.find(x => x.id === tg.dataset.edToggle), tg); return; }
            const menu = event.target.closest('[data-ed-card-menu]');
            if (menu) { const r = menu.getBoundingClientRect(); cardMenu(flows.find(x => x.id === menu.dataset.edCardMenu), r.left, r.bottom); return; }
            const tpl = event.target.closest('[data-ed-template]');
            if (tpl) { if (!createOff()) create({ template: tpl.dataset.edTemplate }); return; }
            const card = event.target.closest('[data-ed-flow]');
            if (card) { clearSelection(); app.openFlow(card.dataset.edFlow); }
        });
        bag.listen(el, 'change', event => {
            const input = event.target?.closest?.('[data-ed-select-flow]');
            if (!input) return;
            event.stopPropagation();
            const flow = flows.find(row => row.id === input.dataset.edSelectFlow && row.published);
            if (!input.checked) selectedFlows.delete(input.dataset.edSelectFlow);
            else if (!mayChange() || !flow) input.checked = false;
            else if (selectedFlows.size >= PAUSE_LIMIT && !selectedFlows.has(flow.id)) {
                input.checked = false;
                voice.announce(t('easydrag.ui.home_bulk_limit'));
            } else selectedFlows.set(flow.id, { id: flow.id, name: flow.name });
            selectionControls();
        });
        bag.listen(el.querySelector('[data-ed-template-search]'), 'input', event => { templateQuery = event.target.value.trim().toLocaleLowerCase(); render(); });
        bag.listen(el.querySelector('[data-ed-template-category]'), 'change', event => { templateCategory = event.target.value; render(); });
        bag.listen(el, 'keydown', (event) => {
            if (event.repeat || (event.key !== 'Enter' && event.key !== ' ')) return;
            const card = event.target.closest('[data-ed-flow], [data-ed-template]');
            if (!card || event.target !== card) return;
            event.preventDefault();
            card.click();
        });
        bag.listen(el, 'contextmenu', (event) => {
            const card = event.target.closest('[data-ed-flow]');
            if (!card) return;
            event.preventDefault();
            cardMenu(flows.find(x => x.id === card.dataset.edFlow), event.clientX, event.clientY);
        });
        const searchUpdate = core.debounce(event => { query = event.target.value; selectedFilter = ''; render(); }, 120);
        bag.listen(el.querySelector('[data-ed-home-search]'), 'input', event => { clearSelection(); searchUpdate(event); });
        bag.add(() => searchUpdate.cancel());
        bag.listen(el.querySelector('[data-ed-library-filter]'), 'change', event => {
            const saved = prefs.filters.find(item => item.id === event.target.value);
            clearSelection();
            selectedFilter = saved ? saved.id : '';
            if (!saved) return;
            searchUpdate.cancel();
            query = saved.query; filter = saved.filter; sortBy = saved.sort; favoritesOnly = saved.favorites;
            organizationFolder = saved.folder || ''; organizationTag = saved.tag || '';
            el.querySelector('[data-ed-home-search]').value = query;
            el.querySelector('[data-ed-home-sort]').value = sortBy;
            el.querySelectorAll('[data-ed-filter]').forEach(button => button.setAttribute('aria-checked', String(button.dataset.edFilter === filter)));
            core.storage.set(FILTER_KEY, filter); render();
        });
        bag.listen(el.querySelector('[data-ed-home-sort]'), 'change', (event) => {
            sortBy = event.target.value || 'updated';
            selectedFilter = '';
            render();
        });
        bag.listen(el.querySelector('[data-ed-import-file]'), 'change', (event) => {
            const file = event.target.files && event.target.files[0];
            event.target.value = '';
            if (file) importFile(file);
            else importSelectionGeneration++;
        });
        bag.listen(el.querySelector('[data-ed-pack-import-file]'), 'change', (event) => {
            const file = event.target.files && event.target.files[0];
            event.target.value = '';
            if (file) importFile(file, true);
            else importSelectionGeneration++;
        });
        bag.listen(el, 'dragover', (event) => {
            if (createOff()) return;
            const dt = event.dataTransfer;
            if (!dt || !Array.from(dt.types || []).includes('Files')) return;
            event.preventDefault();
            dt.dropEffect = 'copy';
            el.classList.add('is-drop-target');
        });
        bag.listen(el, 'dragleave', (event) => {
            if (!el.contains(event.relatedTarget)) el.classList.remove('is-drop-target');
        });
        bag.listen(el, 'drop', (event) => {
            el.classList.remove('is-drop-target');
            if (createOff()) return;
            const file = event.dataTransfer && event.dataTransfer.files && event.dataTransfer.files[0];
            if (!file) return;
            event.preventDefault();
            importFile(file);
        });
        bag.listen(document, 'aurago:flows-changed', (event) => {
            const d = event.detail || {};
            if (d.reason === 'version_note') return;
            // A flow deleted elsewhere leaves nothing behind in this browser either.
            if (d.reason === 'deleted' && d.flow_id) core.forgetFlow(d.flow_id);
            if (activePause && activePause.executing) return;
            if (d.reason === 'saved' || d.reason === 'run_finished') reloadSoon(); else reload();
        });
        bag.add(() => { loadSeq++; reloadSoon.cancel(); voice.cancel(); });
        function refreshPreferences() {
            if (volatilePrefs) return;
            prefs = preferences(core.storage.get(LIBRARY_KEY, {}));
            if (!prefs.filters.some(item => item.id === selectedFilter && item.query === query && item.filter === filter && item.sort === sortBy && item.favorites === favoritesOnly && (item.folder || '') === organizationFolder && (item.tag || '') === organizationTag)) selectedFilter = '';
            render();
        }
        bag.add(libraryChanges.on('change', owner => { if (owner !== el) refreshPreferences(); }));
        bag.listen(window, 'storage', event => {
            if (event.key === LIBRARY_KEY || event.key === null) refreshPreferences();
        });
        if (organization) {
            const frame = core.frame(organizationControls); bag.add(() => frame.cancel());
            bag.listen(organization.querySelector('[data-ed-folder-filter]'), 'change', event => { clearSelection(); organizationFolder = event.target.value; selectedFilter = ''; render(); });
            bag.listen(organization.querySelector('[data-ed-tag-filter]'), 'change', event => { clearSelection(); if (event.target.value === '' || event.target.value.startsWith('tag:')) organizationTag = event.target.value ? event.target.value.slice(4) : ''; selectedFilter = ''; render(); });
            bag.listen(organization.querySelector('[data-ed-folder-filter]'), 'blur', () => frame.request());
            bag.listen(organization.querySelector('[data-ed-tag-filter]'), 'blur', () => frame.request());
            bag.listen(organization.querySelector('[data-ed-folders-manage]'), 'click', () => ED.metadata.folders(metadataEnv(), rows => { folders = rows; cacheOrganization(); render(); }));
        }

        cacheOrganization();
        render();
        reload();
        if (runtime) runtime.refresh();
        return { el, reload, createOff, dispose() { gone = true; pauseSequence++; destroyed.abort(); if (activePause) activePause.cancel.abort(); if (ED.metadata) ED.metadata.dispose(el); bag.dispose(); el.remove(); } };
    }

    ED.home = { create, previewSVG, pauseOps };
})();
