// Infra dialogs: the modal host (confirm, new asset or place, import, edge
// form, source actions, node actions, delete) and the non-modal connection
// popover that opens when a connect drag is released on another asset. The
// shell (infra.js) provides the <dialog data-dialog> element; every write goes
// through the store.
(function () {
    'use strict';

    const KINDS = ['device', 'vm', 'container', 'service', 'network', 'site', 'room', 'rack', 'group'];
    const DEFAULT_ICONS = { device: 'generic-device', vm: 'vm', container: 'container', service: 'generic-app', network: 'lan', site: 'site', room: 'room', rack: 'rack', group: 'group' };
    const EDGE_KINDS = ['ethernet', 'fiber', 'wifi', 'member_of', 'runs_on', 'depends_on', 'powered_by'];
    const CAUTION_ACTIONS = ['stop', 'restart', 'shutdown', 'reboot'];

    function create(app) {
        const { store, state, t, esc, model, q, root, ctx } = app;
        const { iconId, iconLabelKey, sourceActions, sourceIdentity } = model;
        const S = app.safeId;
        const ICON_LIST = model.iconIds();
        const canWrite = () => store.canWrite();
        const commands = () => window.InfraAppStore.commands;
        const dialogEl = () => q('[data-dialog]');
        const ownsDialog = owner => !!owner && dialogEl()?.dataset.infraOwner === owner;
        const closeOwnedDialog = owner => { const dialog = dialogEl(); if (dialog?.open && ownsDialog(owner)) dialog.close(); };
        const edgeExists = fields => state.edges.some(edge => edge.from === fields.from && edge.to === fields.to && edge.kind === fields.kind);

        // ---- the modal host --------------------------------------------------------------
        function openDialog(title, content, onRender) {
            if (state.disposed || !root.isConnected) return null;
            const dialog = dialogEl();
            if (!dialog) return null;
            if (state.importOpen && dialog.dataset.infraOwner === state.importDialogOwner) { state.importOpen = false; state.importDialogOwner = ''; }
            dialog.dataset.infraOwner = String(++state.dialogGeneration);
            q('[data-dialog-content]').innerHTML = `<h2 id="infra-dialog-title-${esc(S)}">${esc(title)}</h2>${content}`;
            if (!dialog.open) dialog.showModal();
            dialog.querySelector('.infra-dialog-close')?.addEventListener('click', event => { event.preventDefault(); dialog.close(); }, { once: true });
            dialog.onclick = event => { if (event.target === dialog) dialog.close(); };
            onRender?.(dialog);
            return dialog;
        }
        async function confirmAction(title, message, { withOwner = false, confirmLabel } = {}) {
            const initialOwner = dialogEl()?.dataset.infraOwner || '';
            if (typeof ctx.showConfirmModal === 'function') {
                const confirmed = !!(await ctx.showConfirmModal(title, message, confirmLabel));
                return withOwner ? { confirmed, owner: initialOwner } : confirmed;
            }
            return new Promise(resolve => {
                const dialog = openDialog(title, `<p class="infra-dialog-intro">${esc(message)}</p><div class="infra-form-actions"><button type="button" data-answer="no">${esc(t('desktop.cancel'))}</button><button type="button" data-answer="yes" class="infra-button-primary">${esc(confirmLabel || t('desktop.ok'))}</button></div>`, dlg => {
                    const owner = dlg.dataset.infraOwner;
                    let done = false;
                    const finish = confirmed => { if (done) return; done = true; resolve(withOwner ? { confirmed, owner } : confirmed); };
                    dlg.querySelector('[data-answer="no"]').addEventListener('click', () => { finish(false); dlg.close(); });
                    dlg.querySelector('[data-answer="yes"]').addEventListener('click', () => { finish(true); dlg.close(); });
                    dlg.addEventListener('close', () => finish(false), { once: true });
                });
                if (!dialog) { resolve(withOwner ? { confirmed: false, owner: initialOwner } : false); return; }
                dialog.querySelector('[data-answer="no"]')?.focus();
            });
        }

        // ---- new asset or place ---------------------------------------------------------------
        function openNodeDialog(kind = 'device', { title } = {}) {
            if (!canWrite()) return;
            openDialog(title || t('desktop.infra_new_node'), `<form data-new-node><label class="infra-field"><span>${esc(t('desktop.infra_name'))}</span><input name="name" maxlength="120" required autofocus></label><div class="infra-field-row"><label class="infra-field"><span>${esc(t('desktop.infra_type'))}</span><select name="kind" data-kind-select>${KINDS.map(value => `<option value="${value}" ${value === kind ? 'selected' : ''}>${esc(app.labelForKind(value))}</option>`).join('')}</select></label><label class="infra-field"><span>${esc(t('desktop.infra_icon'))}</span><select name="icon" data-icon-select>${ICON_LIST.map(value => `<option value="${value}" ${value === (DEFAULT_ICONS[kind] || 'generic-device') ? 'selected' : ''}>${esc(t(iconLabelKey(value)))}</option>`).join('')}</select></label></div><div class="infra-form-actions"><button type="button" data-close>${esc(t('desktop.cancel'))}</button><button class="infra-button-primary" type="submit">${esc(t('desktop.infra_create_node'))}</button></div></form>`, dialog => {
                dialog.querySelector('[data-close]').addEventListener('click', () => dialog.close());
                const kindSelect = dialog.querySelector('[data-kind-select]'), iconSelect = dialog.querySelector('[data-icon-select]');
                kindSelect.addEventListener('change', () => { if (iconSelect.dataset.userChosen !== 'true') iconSelect.value = DEFAULT_ICONS[kindSelect.value] || 'generic-device'; });
                iconSelect.addEventListener('change', () => { iconSelect.dataset.userChosen = 'true'; });
                dialog.querySelector('[data-new-node]').addEventListener('submit', event => { event.preventDefault(); void createNode(new FormData(event.currentTarget), dialog.dataset.infraOwner); });
            });
        }
        async function createNode(form, owner) {
            if (!canWrite() || state.busy.has('create')) return;
            const body = { id: '', version: 0, kind: form.get('kind'), name: String(form.get('name') || '').trim(), icon: iconId(form.get('icon')), profile: { tags: [], interfaces: [], links: [], app_bindings: [], custom: {} }, placements: {}, sources: [] };
            if (!body.name) return;
            store.setBusy('create', true);
            try {
                const result = await store.request('/nodes', { method: 'POST', body });
                const node = result.node || result;
                if (node.id) {
                    const created = store.replaceNode(node);
                    if (ownsDialog(owner)) store.select([created.id]);
                    closeOwnedDialog(owner);
                } else await store.loadSnapshot();
                store.notify(t('desktop.infra_node_created'), 'success');
            } catch (error) { store.notify(store.messageFor(error), 'error'); }
            finally { store.setBusy('create', false); }
        }

        // ---- import -------------------------------------------------------------------------------
        async function loadCandidates(preserveSelection = false, owner = state.importDialogOwner) {
            try {
                const selected = preserveSelection ? new Set(state.importSelection) : new Set();
                const result = await store.request('/candidates');
                if (state.disposed || !owner || state.importDialogOwner !== owner || !ownsDialog(owner)) return;
                state.candidates = result.candidates || []; state.sources = result.sources || state.sources;
                state.importSelection = preserveSelection ? new Set(state.candidates.filter(candidate => selected.has(candidate.id)).map(candidate => candidate.id)) : new Set(state.candidates.map(candidate => candidate.id));
                paintImportDialog();
            } catch (error) { if (error.name !== 'AbortError') store.notify(store.messageFor(error), 'error'); }
        }
        function paintImportDialog() {
            const dialog = dialogEl();
            if (state.disposed || !state.importOpen || !dialog?.open || !ownsDialog(state.importDialogOwner)) return;
            const rows = state.candidates.map(candidate => {
                const node = candidate.node || {};
                const selectedNode = state.importBindings.get(candidate.id) || '';
                const bindableNodes = candidate.source?.kind === 'registry' ? state.nodes.filter(existing => ['device', 'vm', 'container'].includes(existing.kind)) : state.nodes;
                const candidateName = node.name || t('desktop.infra_unnamed_candidate');
                const checkboxLabel = t('desktop.infra_select_candidate').replace('{name}', candidateName);
                return `<article class="infra-candidate"><label class="infra-candidate-check"><input type="checkbox" data-candidate="${esc(candidate.id)}" aria-label="${esc(checkboxLabel)}" ${state.importSelection.has(candidate.id) ? 'checked' : ''}><span class="infra-checkbox-mark"></span></label><div class="infra-candidate-icon">${app.icon(node.icon)}</div><div class="infra-candidate-copy"><strong>${esc(candidateName)}</strong><span>${esc(t(`desktop.infra_source_${candidate.source?.kind}`))} · ${esc(candidate.source?.name || candidate.source?.instance || '')}</span><small>${esc([node.profile?.vendor, node.profile?.model, node.profile?.ip_address].filter(Boolean).join(' · '))}</small></div><span class="infra-state-pill" data-state="${esc(candidate.source?.state || 'unknown')}"><i></i>${esc(app.statusLabel(candidate.source?.state || 'unknown'))}</span><label class="infra-field infra-bind-field"><span>${esc(t('desktop.infra_link_existing'))}</span><select data-bind="${esc(candidate.id)}"><option value="">${esc(t('desktop.infra_import_new'))}</option>${bindableNodes.map(existing => `<option value="${esc(existing.id)}" ${selectedNode === existing.id ? 'selected' : ''}>${esc(existing.name)}</option>`).join('')}</select></label></article>`;
            }).join('');
            dialog.querySelector('[data-candidate-sources]').innerHTML = state.sources.map(source => `<span class="infra-source-chip" data-state="${esc(source.state)}"><i></i>${esc(t(`desktop.infra_source_${source.kind}`))} · ${esc(source.name || source.instance || source.kind)}</span>`).join('');
            dialog.querySelector('.infra-candidate-list').innerHTML = rows || `<div class="infra-empty-candidates"><span aria-hidden="true">⌁</span><strong>${esc(t('desktop.infra_no_candidates'))}</strong><p>${esc(t('desktop.infra_no_candidates_hint'))}</p></div>`;
            dialog.querySelectorAll('[data-candidate]').forEach(box => box.addEventListener('change', () => { if (box.checked) state.importSelection.add(box.dataset.candidate); else state.importSelection.delete(box.dataset.candidate); updateImportCount(dialog); }));
            dialog.querySelectorAll('[data-bind]').forEach(select => select.addEventListener('change', () => state.importBindings.set(select.dataset.bind, select.value)));
            updateImportCount(dialog);
        }
        function updateImportCount(dialog) {
            const count = state.importSelection.size;
            const label = dialog.querySelector('[data-import-count]');
            if (label) label.textContent = t('desktop.infra_selected_count').replace('{count}', count);
            const button = dialog.querySelector('[data-import]');
            if (button) button.disabled = !count || !canWrite() || state.busy.has('import');
        }
        async function importSelected() {
            if (!canWrite() || !state.importSelection.size || state.busy.has('import')) return;
            const owner = state.importDialogOwner;
            store.setBusy('import', true);
            const ids = [...state.importSelection];
            try {
                for (const candidate_id of ids) {
                    await store.request('/import', { method: 'POST', body: { candidate_id, ...(state.importBindings.get(candidate_id) ? { node_id: state.importBindings.get(candidate_id) } : {}) } });
                    state.importSelection.delete(candidate_id); state.importBindings.delete(candidate_id);
                    state.candidates = state.candidates.filter(candidate => candidate.id !== candidate_id);
                    paintImportDialog();
                }
                closeOwnedDialog(owner);
                if (state.importDialogOwner === owner) { state.importOpen = false; state.importDialogOwner = ''; }
                state.importSelection.clear(); state.importBindings.clear();
                store.notify(t('desktop.infra_imported').replace('{count}', ids.length), 'success');
                await store.loadSnapshot();
            } catch (error) {
                if (error.name !== 'AbortError') store.notify(store.messageFor(error), 'error');
                if (!state.disposed && state.importDialogOwner === owner) await loadCandidates(true, owner);
                paintImportDialog();
            } finally { store.setBusy('import', false); }
        }
        function openImport() {
            if (!canWrite()) return;
            state.importOpen = true;
            state.candidates = []; state.importSelection.clear(); state.importBindings.clear();
            const dialog = openDialog(t('desktop.infra_import_title'), `<p class="infra-dialog-intro">${esc(t('desktop.infra_import_hint'))}</p><div class="infra-candidate-source-tabs" data-candidate-sources></div><div class="infra-candidate-list"><div class="infra-import-loading">${esc(t('desktop.infra_loading_candidates'))}</div></div><div class="infra-form-actions infra-dialog-actions"><span data-import-count></span><button type="button" data-close>${esc(t('desktop.cancel'))}</button><button type="button" data-import class="infra-button-primary" disabled>${esc(t('desktop.infra_import_selected'))}</button></div>`, dlg => {
                dlg.querySelector('[data-close]').addEventListener('click', () => dlg.close());
                dlg.querySelector('[data-import]').addEventListener('click', () => void importSelected());
            });
            state.importDialogOwner = dialog?.dataset.infraOwner || '';
            if (dialog && !dialog.dataset.importCloseBound) {
                dialog.dataset.importCloseBound = 'true';
                dialog.addEventListener('close', () => {
                    const owner = dialog.dataset.infraOwner;
                    if (state.importDialogOwner === owner) { state.candidates = []; state.importOpen = false; state.importDialogOwner = ''; }
                });
            }
            void loadCandidates();
        }

        // ---- connections --------------------------------------------------------------------------------
        function openEdgeDialog(edgeId = '', fromId = state.selected, toId = '') {
            if (!canWrite()) return;
            const edge = state.edges.find(item => item.id === edgeId);
            const options = state.nodes.map(node => `<option value="${esc(node.id)}">${esc(node.name)}</option>`).join('');
            openDialog(edge ? t('desktop.infra_edit_relation') : t('desktop.infra_add_relation'), `<form data-edge-form><input type="hidden" name="id" value="${esc(edge?.id || '')}"><input type="hidden" name="version" value="${esc(edge?.version || 0)}"><label class="infra-field"><span>${esc(t('desktop.infra_relation_from'))}</span><select name="from">${options}</select></label><label class="infra-field"><span>${esc(t('desktop.infra_relation_kind'))}</span><select name="kind">${EDGE_KINDS.map(kind => `<option value="${kind}">${esc(t(`desktop.infra_edge_${kind}`))}</option>`).join('')}</select></label><label class="infra-field"><span>${esc(t('desktop.infra_relation_to'))}</span><select name="to">${options}</select></label><div class="infra-field-row"><label class="infra-field"><span>${esc(t('desktop.infra_relation_from_port'))}</span><input name="from_port"></label><label class="infra-field"><span>${esc(t('desktop.infra_relation_to_port'))}</span><input name="to_port"></label></div><div class="infra-field-row"><label class="infra-field"><span>${esc(t('desktop.infra_relation_speed'))}</span><input name="speed"></label><label class="infra-field"><span>${esc(t('desktop.infra_relation_label'))}</span><input name="label"></label></div><div class="infra-form-actions">${edge ? `<button type="button" data-delete-edge class="infra-danger-button">${esc(t('desktop.infra_remove_relation'))}</button>` : ''}<span class="infra-flex-spacer"></span><button type="button" data-close>${esc(t('desktop.cancel'))}</button><button type="submit" class="infra-button-primary">${esc(t('desktop.save'))}</button></div></form>`, dlg => {
                const form = dlg.querySelector('[data-edge-form]');
                form.elements.from.value = edge?.from || fromId || '';
                form.elements.to.value = edge?.to || toId || '';
                form.elements.kind.value = edge?.kind || 'ethernet';
                for (const key of ['label', 'from_port', 'to_port', 'speed']) form.elements[key].value = edge?.[key] || '';
                dlg.querySelector('[data-close]').addEventListener('click', () => dlg.close());
                form.addEventListener('submit', event => { event.preventDefault(); void saveEdge(new FormData(form), dlg.dataset.infraOwner); });
                dlg.querySelector('[data-delete-edge]')?.addEventListener('click', () => void deleteEdge(edge, dlg.dataset.infraOwner));
            });
        }
        async function saveEdge(form, owner) {
            if (!canWrite()) return;
            const id = String(form.get('id') || '');
            const fields = { from: String(form.get('from') || ''), to: String(form.get('to') || ''), kind: String(form.get('kind') || '') };
            for (const key of ['label', 'from_port', 'to_port', 'speed']) if (form.get(key)) fields[key] = String(form.get(key));
            if (!fields.from || !fields.to || fields.from === fields.to) { store.notify(t('desktop.infra_edge_self_error'), 'error'); return; }
            if (!id && edgeExists(fields)) { store.notify(t('desktop.infra_error_edge_exists'), 'error'); return; }
            const busyKey = `edge-${id || 'new'}`;
            if (state.busy.has(busyKey)) return;
            store.setBusy(busyKey, true);
            try {
                if (id) await store.updateEdge(id, { id, version: Number(form.get('version') || 0), ...fields });
                else if (!await store.dispatch(commands().createEdge(fields))) return;
                closeOwnedDialog(owner);
                store.notify(t('desktop.infra_relation_saved'), 'success');
            } catch (error) { store.notify(store.messageFor(error), 'error'); }
            finally { store.setBusy(busyKey, false); }
        }
        async function deleteEdge(edge, owner) {
            if (!edge || !canWrite()) return;
            const confirmation = await confirmAction(t('desktop.infra_remove_relation'), t('desktop.infra_remove_relation_confirm'), { withOwner: true });
            if (!confirmation.confirmed) return;
            try {
                if (await store.dispatch(commands().deleteEdge(edge))) { closeOwnedDialog(confirmation.owner || owner); app.announce(t('desktop.infra_connection_removed')); }
            } catch (error) { store.notify(store.messageFor(error), 'error'); }
        }

        // The popover opens at the release point of a connect drag.
        let popover = null, popoverCleanup = null, popoverFrom = '';
        function closeConnectionPopover({ restoreFocus = true } = {}) {
            if (!popover) return;
            popoverCleanup?.();
            popover.remove();
            const from = popoverFrom;
            popover = null; popoverCleanup = null; popoverFrom = '';
            if (restoreFocus) app.canvas?.cardEl(from)?.focus({ preventScroll: true });
        }
        function openConnectionPopover({ from, to, clientX, clientY }) {
            closeConnectionPopover({ restoreFocus: false });
            const fromNode = store.nodeById(from), toNode = store.nodeById(to);
            if (!canWrite() || !fromNode || !toNode || from === to || !app.canvas) return;
            const holder = document.createElement('div');
            holder.innerHTML = `<form class="infra-connect-popover" data-connect-popover role="dialog" aria-labelledby="infra-connect-title-${S}">
  <h3 id="infra-connect-title-${S}">${esc(t('desktop.infra_connection_new'))}</h3>
  <p class="infra-connect-ends">${esc(fromNode.name)} → ${esc(toNode.name)}</p>
  <label class="infra-field"><span>${esc(t('desktop.infra_connection_kind'))}</span><select name="kind" data-connect-kind>${EDGE_KINDS.map(kind => `<option value="${kind}" ${kind === model.defaultEdgeKind(fromNode, toNode) ? 'selected' : ''}>${esc(t(`desktop.infra_edge_${kind}`))}</option>`).join('')}</select></label>
  <label class="infra-field"><span>${esc(t('desktop.infra_relation_label'))}</span><input name="label" maxlength="128" autocomplete="off"></label>
  <div class="infra-field-row"><label class="infra-field"><span>${esc(t('desktop.infra_relation_from_port'))}</span><input name="from_port" maxlength="64"></label><label class="infra-field"><span>${esc(t('desktop.infra_relation_to_port'))}</span><input name="to_port" maxlength="64"></label></div>
  <p class="infra-connect-error" data-connect-error role="alert" hidden></p>
  <div class="infra-form-actions"><button type="button" class="infra-button" data-action="cancel-connection">${esc(t('desktop.cancel'))}</button><button type="submit" class="infra-button infra-button-primary" data-action="create-connection">${esc(t('desktop.infra_connection_create'))}</button></div>
</form>`;
            const form = holder.firstElementChild;
            const errorEl = form.querySelector('[data-connect-error]'), submit = form.querySelector('[type="submit"]');
            const showError = text => { errorEl.textContent = text; errorEl.hidden = !text; };
            popover = form; popoverFrom = from;
            const host = app.canvas.el;
            host.appendChild(form);
            const bounds = host.getBoundingClientRect(), width = form.offsetWidth || 300, height = form.offsetHeight || 260;
            form.style.left = `${Math.max(8, Math.min(clientX - bounds.left, bounds.width - width - 8))}px`;
            form.style.top = `${Math.max(8, Math.min(clientY - bounds.top, bounds.height - height - 8))}px`;
            const onDown = event => { if (!form.contains(event.target)) closeConnectionPopover(); };
            document.addEventListener('pointerdown', onDown, true);
            popoverCleanup = () => document.removeEventListener('pointerdown', onDown, true);
            form.addEventListener('keydown', event => {
                if (event.key !== 'Escape') return;
                event.preventDefault(); event.stopPropagation();
                closeConnectionPopover();
            });
            form.querySelector('[data-action="cancel-connection"]').addEventListener('click', () => closeConnectionPopover());
            form.addEventListener('submit', async event => {
                event.preventDefault();
                const data = new FormData(form);
                const fields = { from, to, kind: String(data.get('kind') || '') };
                for (const key of ['label', 'from_port', 'to_port']) { const value = String(data.get(key) || '').trim(); if (value) fields[key] = value; }
                if (edgeExists(fields)) { showError(t('desktop.infra_error_edge_exists')); return; }
                submit.disabled = true; showError('');
                try {
                    const ok = await store.dispatch(commands().createEdge(fields));
                    if (popover !== form) return;
                    if (ok) { closeConnectionPopover(); app.announce(t('desktop.infra_connection_created')); return; }
                    showError(t('desktop.infra_error_infra_error'));
                } catch (error) {
                    if (popover === form) showError(store.messageFor(error));
                } finally { if (popover === form) submit.disabled = false; }
            });
            form.querySelector('[data-connect-kind]').focus();
        }

        // ---- delete -------------------------------------------------------------------------------------------
        async function removeRemote(node) {
            await store.request(`/nodes/${encodeURIComponent(node.id)}?version=${encodeURIComponent(node.version)}`, { method: 'DELETE' });
            store.removeNode(node.id);
        }
        async function deleteNode(id) {
            const node = store.nodeById(id);
            if (!node || !canWrite()) return;
            if (!await confirmAction(t('desktop.infra_remove_node'), t('desktop.infra_remove_node_confirm').replace('{name}', node.name))) return;
            try { await removeRemote(node); store.notify(t('desktop.infra_node_removed'), 'success'); }
            catch (error) { store.notify(store.messageFor(error), 'error'); }
        }
        // deleteNodes removes assets one by one, places last, and stops at the first error.
        async function deleteNodes(ids) {
            const nodes = ids.map(id => store.nodeById(id)).filter(Boolean);
            if (!nodes.length || !canWrite()) return;
            if (nodes.length === 1) { await deleteNode(nodes[0].id); return; }
            if (!await confirmAction(t('desktop.infra_remove_node'), t('desktop.infra_remove_selection_confirm').replace('{count}', nodes.length))) return;
            const ordered = [...nodes.filter(node => !model.isContainer(node.kind)), ...nodes.filter(node => model.isContainer(node.kind))];
            for (const node of ordered) {
                const current = store.nodeById(node.id);
                if (!current) continue;
                try { await removeRemote(current); }
                catch (error) { store.notify(store.messageFor(error), 'error'); return; }
            }
            store.notify(t('desktop.infra_node_removed'), 'success');
        }

        // ---- source actions -------------------------------------------------------------------------------------
        function openActionMenu(id, source) {
            const node = store.nodeById(id);
            if (!node || !store.canControl()) return null;
            const sourceDescriptor = typeof source === 'string' ? (node.sources || []).find(item => item.kind === source) : source;
            if (!sourceDescriptor) return null;
            const actions = sourceActions(sourceDescriptor.kind).filter(action => store.allowedAction(node, action, sourceDescriptor));
            if (!actions.length) return null;
            const blockedKey = action => store.actionBlockKey(id, sourceDescriptor, action);
            const blocked = actions.filter(action => state.actionBlocks.has(blockedKey(action)));
            return openDialog(t('desktop.infra_source_actions'), `<p class="infra-dialog-intro">${esc(t('desktop.infra_action_warning'))}</p><div class="infra-action-list">${actions.map(action => `<button type="button" data-run-action="${esc(action)}" data-action-node="${esc(id)}" data-source-identity="${esc(sourceIdentity(sourceDescriptor))}" class="${CAUTION_ACTIONS.includes(action) ? 'is-caution' : ''}" ${state.actionBlocks.has(blockedKey(action)) ? 'disabled' : ''}><span>${esc(t(`desktop.infra_action_${action}`))}</span><small>${esc(t(`desktop.infra_action_help_${action}`))}</small><b aria-hidden="true">›</b></button>`).join('')}</div>${blocked.length ? `<section class="infra-action-review" role="status"><strong>${esc(t('desktop.infra_action_review_required'))}</strong>${blocked.map(action => `<div><span>${esc(t(`desktop.infra_action_${action}`))} · ${esc(state.actionBlocks.get(blockedKey(action))?.status || 'unknown')}</span><button type="button" data-action-rearm="${esc(action)}">${esc(t('desktop.infra_action_rearm'))}</button></div>`).join('')}</section>` : ''}`, dlg => {
                dlg.querySelectorAll('[data-run-action]').forEach(button => button.addEventListener('click', () => void runAction(id, sourceDescriptor, button.dataset.runAction, dlg)));
                dlg.querySelectorAll('[data-action-rearm]').forEach(button => button.addEventListener('click', () => {
                    state.actionBlocks.delete(blockedKey(button.dataset.actionRearm));
                    store.notify(t('desktop.infra_action_rearmed'), 'info');
                    openActionMenu(id, sourceDescriptor);
                }));
            });
        }
        async function runAction(id, source, action, dialog = null) {
            const node = store.nodeById(id);
            const sourceDescriptor = typeof source === 'string' ? (node?.sources || []).find(item => item.kind === source) : source;
            const blockKey = sourceDescriptor && store.actionBlockKey(id, sourceDescriptor, action);
            const busyKey = `action-${id}-${sourceDescriptor ? sourceIdentity(sourceDescriptor) : ''}-${action}`;
            let dialogOwner = dialog?.dataset.infraOwner || dialogEl()?.dataset.infraOwner || '';
            let reviewAllowed = !dialog;
            if (!sourceDescriptor || !store.allowedAction(node, action, sourceDescriptor) || state.actionBlocks.has(blockKey) || state.busy.has(busyKey)) return;
            store.setBusy(busyKey, true);
            const request_id = window.crypto?.randomUUID?.() || `${Date.now()}-${Math.random().toString(36).slice(2)}`;
            try {
                if (CAUTION_ACTIONS.includes(action)) {
                    const confirmation = await confirmAction(t(`desktop.infra_action_${action}`), t('desktop.infra_confirm_action').replace('{action}', t(`desktop.infra_action_${action}`)).replace('{name}', store.nodeById(id)?.name || ''), { withOwner: true });
                    if (!confirmation.confirmed) return;
                    dialogOwner = confirmation.owner || dialogOwner;
                    reviewAllowed = true;
                }
                if (state.disposed || !root.isConnected) return;
                const result = await store.request(`/nodes/${encodeURIComponent(id)}/actions`, { method: 'POST', body: { source: sourceDescriptor.kind, action, request_id, confirmed: true } });
                if (state.disposed || !root.isConnected) return;
                if (['running', 'accepted', 'unknown'].includes(result.status)) {
                    state.actionBlocks.set(blockKey, { status: result.status, request_id });
                    store.notify(result.status === 'unknown' ? t('desktop.infra_action_outcome_unknown') : t('desktop.infra_action_review_required'), result.status === 'unknown' ? 'error' : 'info');
                    if (ownsDialog(dialogOwner) && (!dialog || dialog.open || reviewAllowed)) openActionMenu(id, sourceDescriptor);
                    return;
                }
                store.notify(t(`desktop.infra_action_status_${result.status}`).replace('{status}', result.status), result.status === 'failed' || result.status === 'unknown' ? 'error' : 'success');
                closeOwnedDialog(dialogOwner);
            } catch (error) {
                if (state.disposed || !root.isConnected) return;
                if (store.uncertainMutation(error)) {
                    state.actionBlocks.set(blockKey, { status: 'unknown', request_id });
                    store.notify(t('desktop.infra_action_outcome_unknown'), 'error');
                    if (ownsDialog(dialogOwner) && (!dialog || dialog.open || reviewAllowed)) openActionMenu(id, sourceDescriptor);
                } else store.notify(store.messageFor(error), 'error');
            } finally { store.setBusy(busyKey, false); }
        }
        function openNodeActions(id) {
            const node = store.nodeById(id);
            if (!node) return null;
            const sourceButtons = store.canControl() ? (node.sources || []).flatMap(source => sourceActions(source.kind).filter(action => store.allowedAction(node, action, source)).map(action => `<button type="button" data-act-source="${esc(sourceIdentity(source))}" data-act="${esc(action)}"><span>${esc(t(`desktop.infra_action_${action}`))} · ${esc(t(`desktop.infra_source_${source.kind}`))}</span><b>›</b></button>`)).join('') : '';
            const content = `<div class="infra-action-list">${canWrite() ? `<button type="button" data-act="edge"><span>${esc(t('desktop.infra_add_relation'))}</span><b>›</b></button><button type="button" data-act="remove" class="is-caution"><span>${esc(t('desktop.infra_remove_node'))}</span><b>›</b></button>` : ''}${sourceButtons}</div>`;
            return openDialog(node.name, content, dlg => {
                dlg.querySelector('[data-act="edge"]')?.addEventListener('click', () => { dlg.close(); openEdgeDialog('', id); });
                dlg.querySelector('[data-act="remove"]')?.addEventListener('click', () => { dlg.close(); void deleteNode(id); });
                dlg.querySelectorAll('[data-act-source]').forEach(button => button.addEventListener('click', () => {
                    dlg.close();
                    const source = (store.nodeById(id)?.sources || []).find(item => sourceIdentity(item) === button.dataset.actSource);
                    if (source) void runAction(id, source, button.dataset.act);
                }));
            });
        }

        // ---- busy keys and teardown -------------------------------------------------------------------------------
        function onBusy(key) {
            const busy = state.busy.has(key);
            if (key === 'create') { const submit = q('[data-new-node] [type="submit"]'); if (submit) submit.disabled = busy; }
            if (key === 'import') { const submit = q('[data-import]'); if (submit) submit.disabled = busy || !state.importSelection.size || !canWrite(); }
            if (key.startsWith('edge-')) { const submit = q('[data-edge-form] [type="submit"]'); if (submit) submit.disabled = busy; }
            if (key.startsWith('action-')) dialogEl()?.querySelectorAll('[data-run-action]').forEach(button => { button.disabled = busy || state.actionBlocks.has(store.actionBlockKey(button.dataset.actionNode, button.dataset.sourceIdentity, button.dataset.runAction)); });
        }
        function dispose() {
            closeConnectionPopover({ restoreFocus: false });
            const dialog = dialogEl();
            if (dialog?.open) dialog.close();
        }

        return {
            openDialog, confirmAction, openNodeDialog, openImport, openEdgeDialog,
            deleteNode, deleteNodes, openActionMenu, runAction, openNodeActions,
            openConnectionPopover, closeConnectionPopover, onBusy, ownsDialog, closeOwnedDialog, dispose,
        };
    }

    window.InfraAppDialogs = { create };
})();
