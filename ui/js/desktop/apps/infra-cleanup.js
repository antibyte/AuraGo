// Infra cleanup: the bodies of the library's Cleanup and Ignored sections.
// Cleanup lists the suggestions model.infraCleanup computes (duplicates, Docker
// engines and guests without a host, unreadable names); each has one primary
// action and a dismissal that is remembered in localStorage. Ignored lists the
// ignored sources with Restore. infra-library.js owns the section shells and
// calls render* and onClick; every write goes through the store.
(function () {
    'use strict';

    const DISMISSED_KEY = 'infra.dismissed.v1';

    function create(app) {
        const { store, state, t, esc, model } = app;
        const S = app.safeId;
        let dismissed = model.readDismissed(app.readStorage(DISMISSED_KEY));
        const expanded = new Set(), running = new Set();
        let memoKey = null, memo = [];

        const nameOf = id => store.nodeById(id)?.name || id;
        const items = () => {
            if (!memoKey || memoKey[0] !== state.nodes || memoKey[1] !== state.edges || memoKey[2] !== state.sourceHosts) {
                memo = model.infraCleanup(state.nodes, state.sources, state.sourceHosts, { edges: state.edges });
                memoKey = [state.nodes, state.edges, state.sourceHosts];
            }
            return memo;
        };
        const shown = item => item.nodeIds.some(id => { const node = store.nodeById(id); return !!node && store.matches(node); });
        const visible = () => items().filter(item => !dismissed.has(item.id) && shown(item));
        const dismissedCount = () => items().filter(item => dismissed.has(item.id) && shown(item)).length;
        const saveDismissed = () => app.writeStorage(DISMISSED_KEY, JSON.stringify([...dismissed].slice(-500)));
        const find = id => items().find(item => item.id === id) || null;

        // ---- Cleanup ----------------------------------------------------------------
        function describe(item) {
            if (item.type === 'duplicate') {
                const names = item.nodeIds.map(nameOf).join(', ');
                const text = item.match === 'mac' ? t('desktop.infra_cleanup_duplicate_mac') : t('desktop.infra_cleanup_duplicate');
                return { text: model.fill(text, { names, address: item.value }), primary: t('desktop.infra_cleanup_remove_duplicate'), meta: model.fill(t('desktop.infra_cleanup_keep'), { name: nameOf(item.keepId) }) };
            }
            if (item.type === 'engine-host') return { text: model.fill(t('desktop.infra_cleanup_engine_host'), { count: item.guestIds.length }), primary: t('desktop.infra_assign_engine_host'), dialog: true };
            if (item.type === 'guest-host') return { text: model.fill(t('desktop.infra_cleanup_guest_host'), { name: nameOf(item.nodeId) }), primary: t('desktop.infra_choose_host'), dialog: true };
            return { text: model.fill(t('desktop.infra_cleanup_opaque'), { name: nameOf(item.nodeId) }), primary: t('desktop.infra_rename') };
        }
        function guestRow(id) {
            const node = store.nodeById(id);
            if (!node) return '';
            const name = esc(node.name || node.id);
            return `<li class="infra-library-row" data-library-item="${esc(node.id)}" data-section="cleanup" data-row-kind="plain"><button type="button" class="infra-library-row-main" role="option" aria-selected="${state.selection.has(node.id)}" data-library-select="${esc(node.id)}" tabindex="-1" title="${name}"><img class="infra-library-row-icon" src="${esc(model.iconURL(node.icon))}" alt="" draggable="false"><span class="infra-library-row-text"><span class="infra-library-row-name">${name}</span><span class="infra-library-row-meta">${esc(app.labelForKind(node.kind))}</span></span></button></li>`;
        }
        function itemHTML(item, index) {
            const info = describe(item), write = store.canWrite(), id = esc(item.id), listId = `infra-cleanup-${S}-${index}`;
            let guests = '';
            if (item.type === 'engine-host') {
                const open = expanded.has(item.id);
                const label = open ? t('desktop.infra_cleanup_hide_guests') : model.fill(t('desktop.infra_cleanup_show_guests'), { count: item.guestIds.length });
                guests = `<button type="button" class="infra-library-disclosure infra-cleanup-toggle" data-cleanup-expand="${id}" aria-expanded="${open}" aria-controls="${listId}">${app.glyph('chevron')}<span>${esc(label)}</span></button>`
                    + `<ul class="infra-library-children" id="${listId}" role="listbox" aria-multiselectable="true" aria-label="${esc(label)}"${open ? '' : ' hidden'}>${open ? item.guestIds.map(guestRow).join('') : ''}</ul>`;
            }
            const readOnly = write ? '' : ` disabled title="${esc(t('desktop.infra_read_only'))}"`;
            return `<li class="infra-cleanup-item" data-cleanup="${id}" data-cleanup-type="${esc(item.type)}"><p class="infra-cleanup-text">${esc(info.text)}</p>${info.meta ? `<p class="infra-cleanup-meta">${esc(info.meta)}</p>` : ''}${guests}`
                + `<div class="infra-cleanup-actions"><button type="button" class="infra-button infra-button-primary" data-cleanup-action="${id}"${info.dialog ? ' aria-haspopup="dialog"' : ''}${readOnly}>${esc(info.primary)}</button><button type="button" class="infra-button" data-cleanup-dismiss="${id}">${esc(t('desktop.infra_dismiss'))}</button></div></li>`;
        }
        function renderCleanup({ section, list, count, reset }) {
            const rows = visible(), hiddenCount = dismissedCount();
            list.innerHTML = rows.map(itemHTML).join('');
            count.textContent = String(rows.length);
            reset.hidden = hiddenCount === 0;
            reset.textContent = hiddenCount ? model.fill(t('desktop.infra_cleanup_show_dismissed'), { count: hiddenCount }) : '';
            section.hidden = !rows.length && hiddenCount === 0;
            return rows.length;
        }

        // ---- Ignored -----------------------------------------------------------------
        const sourceName = source => source.name || source.key || source.instance || source.kind;
        function renderIgnored({ section, list, count }) {
            const query = String(state.filters.query || '').trim().toLocaleLowerCase();
            const rows = state.sources.filter(source => source.ignored)
                .filter(source => !query || [source.name, source.key, source.kind, source.instance].some(value => String(value || '').toLocaleLowerCase().includes(query)))
                .sort((a, b) => model.naturalCompare(sourceName(a), sourceName(b)));
            const write = store.canWrite();
            list.innerHTML = rows.map(source => {
                const identity = esc(model.sourceIdentity(source)), name = sourceName(source);
                return `<li class="infra-ignored-row" data-ignored-source="${identity}"><span class="infra-ignored-copy"><strong title="${esc(name)}">${esc(name)}</strong><small>${esc(t(`desktop.infra_source_${source.kind}`))}</small></span><button type="button" class="infra-button" data-restore-source="${identity}" aria-label="${esc(model.fill(t('desktop.infra_restore_source'), { name }))}"${write ? '' : ' disabled'}>${esc(t('desktop.infra_restore'))}</button></li>`;
            }).join('');
            count.textContent = String(rows.length);
            section.hidden = rows.length === 0;
            return rows.length;
        }

        // ---- actions -----------------------------------------------------------------
        async function removeDuplicates(item) {
            const names = item.removeIds.map(nameOf).join(', ');
            const message = model.fill(t('desktop.infra_cleanup_remove_confirm'), { names, keep: nameOf(item.keepId) });
            if (!await app.dialogs.confirmAction(t('desktop.infra_cleanup_remove_duplicate'), message, { confirmLabel: t('desktop.infra_cleanup_remove_duplicate') })) return;
            if (!store.canWrite()) return;
            const removed = await store.removeNodes(item.removeIds, { ignoreSources: true });
            if (removed.length) app.announce(model.fill(t('desktop.infra_removed_ignored'), { count: removed.length }));
        }
        async function run(id, button) {
            const item = find(id);
            if (!item || running.has(id)) return;
            const rect = button.getBoundingClientRect();
            running.add(id);
            try {
                if (item.type === 'opaque') {
                    app.inspector.open(item.nodeId, 'details');
                    app.inspector.focusName();
                } else if (!store.canWrite()) return;
                else if (item.type === 'duplicate') await removeDuplicates(item);
                else if (item.type === 'engine-host') {
                    app.picker.openAssetPicker({
                        anchor: button, rect, label: t('desktop.infra_assign_engine_host'), accept: node => node.kind === 'device', preferred: [item.inferredNodeId].filter(Boolean),
                        onPick: async option => { if (await store.setSourceHost(item.instance, option.id)) app.announce(model.fill(t('desktop.infra_engine_host_assigned'), { name: option.label })); },
                    });
                } else if (item.type === 'guest-host') {
                    const guest = store.nodeById(item.nodeId);
                    if (!guest) return;
                    app.picker.openAssetPicker({
                        anchor: button, rect, label: t('desktop.infra_choose_host'), accept: node => node.id !== guest.id && (node.kind === 'device' || (guest.kind === 'container' && node.kind === 'vm')),
                        onPick: async option => {
                            try {
                                if (await store.dispatch(window.InfraAppStore.commands.createEdge({ from: guest.id, to: option.id, kind: 'runs_on' }))) app.announce(model.fill(t('desktop.infra_host_chosen'), { name: guest.name, host: option.label }));
                            } catch (error) { await store.writeFailed(error); }
                        },
                    });
                }
            } catch (error) { await store.writeFailed(error); }
            finally { running.delete(id); }
        }
        async function restoreSource(identity) {
            const source = state.sources.find(item => item.ignored && model.sourceIdentity(item) === identity);
            if (!source || !store.canWrite()) return;
            if (await store.setSourcesIgnored([source], false)) {
                app.announce(model.fill(t('desktop.infra_restored'), { name: sourceName(source) }));
                app.importer?.reload();
            }
        }

        // onClick answers true when the click belonged to the section bodies.
        function onClick(event, target = event.target) {
            const expand = target.closest('[data-cleanup-expand]');
            if (expand) {
                const id = expand.dataset.cleanupExpand;
                if (expanded.has(id)) expanded.delete(id); else expanded.add(id);
                app.library.render();
                return true;
            }
            const dismiss = target.closest('[data-cleanup-dismiss]');
            if (dismiss) {
                dismissed.add(dismiss.dataset.cleanupDismiss);
                saveDismissed();
                app.library.render();
                return true;
            }
            if (target.closest('[data-action="cleanup-reset"]')) {
                dismissed = new Set();
                app.writeStorage(DISMISSED_KEY, '[]');
                app.library.render();
                return true;
            }
            const action = target.closest('[data-cleanup-action]');
            if (action) { if (!action.disabled) void run(action.dataset.cleanupAction, action); return true; }
            const restore = target.closest('[data-restore-source]');
            if (restore) { if (!restore.disabled) void restoreSource(restore.dataset.restoreSource); return true; }
            return false;
        }

        return { renderCleanup, renderIgnored, onClick, visible, dispose() {} };
    }

    window.InfraAppCleanup = { create };
})();
