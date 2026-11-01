(function () {
    'use strict';

    const M = window.FindItModel, D = window.FindItDialogs;
    function create(c) {
        const { state, ctx, host, tr, canWrite, canPrint, dialogOptions, draw, apiQuery, apiCommand, withBusy, loadItems, loadProjects, loadLocations, selectProject, showNotice, noteText, categoryName, compactNumber, statusClass, sprite, icon } = c;
        const asList = M.asArray;
        function renderProjects() {
            const rows = asList(state.projects);
            return `<div class="fi-page-toolbar"><div><p class="fi-eyebrow">${M.esc(tr('work_planning'))}</p><h1>${M.esc(tr('projects'))}</h1><p>${M.esc(tr('projects_intro'))}</p></div><div class="fi-page-actions"><button type="button" class="fi-tool-button" data-action="import-project-csv">${icon('import', 16)}${M.esc(tr('import_bom_csv'))}</button><button type="button" class="fi-tool-button" data-action="export-project-csv" ${state.project ? '' : 'disabled'}>${icon('export', 16)}${M.esc(tr('export_bom'))}</button>${canWrite() ? `<button type="button" class="fi-primary" data-action="new-project">${icon('plus', 16)}${M.esc(tr('new_project'))}</button>` : ''}</div></div><div class="fi-project-layout ${state.mobilePanel === 'detail' ? 'fi-show-detail' : ''}"><section class="fi-project-list" aria-label="${M.esc(tr('project_list'))}"><header><strong>${M.esc(tr('project_list'))}</strong><span>${rows.length}</span></header>${rows.length ? rows.map(project => `<button type="button" class="fi-project-row ${state.project?.id === project.id ? 'is-selected' : ''}" data-select-project="${M.esc(project.id)}"><span class="fi-project-symbol">${icon('file', 17)}</span><span class="fi-project-row-copy"><b>${M.esc(project.name)}</b><small>${M.esc(project.description || tr('no_description'))}</small></span><i class="fi-state-pill ${statusClass(project.status)}">${M.esc(tr('status_' + project.status, project.status))}</i></button>`).join('') : `<div class="fi-empty-compact"><p>${M.esc(tr('no_projects'))}</p>${canWrite() ? `<button type="button" class="fi-text-button" data-action="new-project">${icon('plus', 14)}${M.esc(tr('create_project'))}</button>` : ''}</div>`}</section><section class="fi-project-detail">${state.project ? renderProjectDetail(state.project) : `<div class="fi-detail-empty"><div class="fi-detail-empty-icon">${icon('file', 25)}</div><h2>${M.esc(tr('select_project'))}</h2><p>${M.esc(tr('select_project_help'))}</p></div>`}</section></div>`;
        }
        function renderProjectDetail(project) {
            const lines = asList(project.lines), shortages = asList(state.projectShortages?.lines);
            const shortageById = new Map(shortages.map(line => [line.line_id || line.id, line]));
            const reservations = asList(project.reservations || state.projectShortages?.reservations);
            const activeReservations = reservations.filter(reservation => M.quantityCompare(reservation.remaining ?? reservation.remaining_quantity ?? reservation.quantity ?? '0', '0') > 0);
            const closedReservations = reservations.filter(reservation => M.quantityCompare(reservation.remaining ?? reservation.remaining_quantity ?? reservation.quantity ?? '0', '0') <= 0);
            const pickupGroups = groupActiveReservations(project, activeReservations);
            const editable = project.status === 'draft' && canWrite();
            const holdCount = reservations.reduce((total, reservation) => total + (M.quantityCompare(reservation.remaining ?? reservation.remaining_quantity ?? reservation.quantity ?? '0', '0') > 0 ? 1 : 0), 0);
            const statusText = project.status === 'completed' ? tr('completion_does_not_consume') : project.status === 'released' ? tr('released_reservations_note') : '';
            return `<header class="fi-project-detail-head"><button type="button" class="fi-mobile-back" data-action="back-projects">${icon('back', 16)}${M.esc(tr('back_to_projects'))}</button><div><p class="fi-eyebrow">${M.esc(tr('project'))}</p><h2>${M.esc(project.name)}</h2><p>${M.esc(project.description || '')}</p></div><span class="fi-state-pill ${statusClass(project.status)}">${M.esc(tr('status_' + project.status, project.status))}</span><div class="fi-project-menu">${editable ? `<button type="button" class="fi-tool-button" data-action="edit-project">${icon('edit', 15)}${M.esc(tr('edit_project'))}</button><button type="button" class="fi-tool-button" data-action="add-project-line">${icon('plus', 15)}${M.esc(tr('add_line'))}</button>` : ''}${canWrite() && project.status === 'draft' ? `<button type="button" class="fi-primary" data-action="release-project">${icon('check', 15)}${M.esc(tr('release_project'))}</button>` : ''}${canWrite() && project.status === 'released' ? `<button type="button" class="fi-tool-button" data-action="reopen-project" ${holdCount ? 'disabled title="' + M.esc(tr('release_holds_first')) + '"' : ''}>${M.esc(tr('reopen_revision'))}</button><button type="button" class="fi-tool-button" data-action="complete-project" ${holdCount ? 'disabled' : ''}>${M.esc(tr('complete_project'))}</button><button type="button" class="fi-danger-button" data-action="cancel-project">${M.esc(tr('cancel_project'))}</button>` : ''}</div></header>${statusText ? `<div class="fi-project-note">${icon('check', 15)}${M.esc(statusText)}</div>` : ''}
                <section class="fi-bom-section"><header><div><p class="fi-eyebrow">${M.esc(tr('project_plan'))}</p><h3>${M.esc(tr('bill_of_materials'))}</h3></div><div class="fi-bom-actions">${editable ? `<button type="button" class="fi-tool-button" data-action="import-project-csv">${icon('import', 14)}${M.esc(tr('import_bom_csv'))}</button>` : ''}<span>${lines.length} ${M.esc(tr('lines'))}</span></div></header>${lines.length ? `<div class="fi-bom-table-wrap"><table class="fi-bom-table"><thead><tr><th>${M.esc(tr('part'))}</th><th>${M.esc(tr('required'))}</th><th>${M.esc(tr('reserved'))}</th><th>${M.esc(tr('available'))}</th><th>${M.esc(tr('shortage'))}</th><th></th></tr></thead><tbody>${lines.map(line => { const summary = shortageById.get(line.id) || {}; const item = line.item || state.rows.find(row => row.id === line.item_id) || {}; const shortage = summary.shortage ?? '0'; return `<tr><td><div class="fi-bom-part">${sprite(item, true)}<span><b>${M.esc(item.name || line.name || line.item_id)}</b><small>${M.esc(item.part_number || item.manufacturer || line.note || '')}</small></span></div></td><td>${M.esc(compactNumber(summary.required ?? line.quantity))} <small>${M.esc(item.unit || '')}</small></td><td>${M.esc(compactNumber(summary.reserved ?? line.reserved ?? '0'))}</td><td>${M.esc(compactNumber(summary.available ?? '0'))}</td><td><span class="fi-shortage ${M.quantityCompare(shortage, '0') > 0 ? 'is-short' : ''}">${M.esc(compactNumber(shortage))}</span></td><td>${editable ? `<button type="button" data-remove-line="${M.esc(line.id)}" aria-label="${M.esc(tr('remove_line'))}">×</button>` : project.status === 'released' && canWrite() ? `<button type="button" class="fi-text-button" data-reserve-line="${M.esc(line.id)}">${M.esc(tr('reserve'))}</button>` : ''}</td></tr>`; }).join('')}</tbody></table></div>` : `<div class="fi-empty-compact"><p>${M.esc(tr('empty_bom'))}</p>${editable ? `<button type="button" class="fi-primary" data-action="add-project-line">${icon('plus', 15)}${M.esc(tr('add_first_line'))}</button>` : ''}</div>`}</section>
                ${activeReservations.length ? `<section class="fi-reservations-section fi-pick-list"><header><div><p class="fi-eyebrow">${M.esc(tr('stock_holds'))}</p><h3>${M.esc(tr('reservations'))}</h3></div><button type="button" class="fi-tool-button" data-action="print-pick-list" ${canPrint() ? '' : 'disabled'} aria-label="${M.esc(tr('print') + ' ' + tr('stock_holds'))}">${icon('barcode', 14)}${M.esc(tr('print'))}</button></header>${renderPickupGroups(project, pickupGroups)}</section>` : ''}
                ${closedReservations.length ? `<details class="fi-reservation-history"><summary>${M.esc(tr('reservations'))} · ${closedReservations.length}</summary><div class="fi-reservations-list">${closedReservations.map(reservation => { const line = asList(project.lines).find(value => value.id === reservation.line_id); return `<div class="fi-reservation-row"><span class="fi-reservation-marker">${icon('box', 15)}</span><div><b>${M.esc(line?.item_name || reservation.item_name || reservation.part_number || reservation.item_id || tr('reserved_part'))}</b><small>${M.esc(asList(reservation.location_path).join(' › ') || tr('unknown_location'))}</small></div><span>${M.esc(compactNumber(reservation.quantity || '0'))} ${M.esc(reservation.unit || line?.unit || '')}</span><small>${M.esc(tr('stock_consumed'))}: ${M.esc(compactNumber(reservation.consumed || '0'))} · ${M.esc(tr('release'))}: ${M.esc(compactNumber(reservation.released || '0'))}</small></div>`; }).join('')}</div></details>` : ''}`;
        }
        function groupActiveReservations(project, reservations) {
            const lines = new Map(asList(project.lines).map(line => [line.id, line])), groups = new Map();
            for (const reservation of reservations) {
                const path = asList(reservation.location_path).map(String), label = path.join(' › ') || reservation.location_name || tr('unknown_location');
                if (!groups.has(label)) groups.set(label, { label, entries: [] });
                const line = lines.get(reservation.line_id) || {};
                groups.get(label).entries.push({ reservation, line, label: reservation.item_name || line.item_name || line.name || line.part_number || reservation.item_id || tr('reserved_part'), partNumber: reservation.part_number || line.part_number || '', unit: reservation.unit || line.unit || '', quantity: reservation.remaining ?? reservation.remaining_quantity ?? reservation.quantity ?? '0' });
            }
            return [...groups.values()].sort((a, b) => a.label.localeCompare(b.label, undefined, { numeric: true })).map(group => ({ ...group, entries: group.entries.sort((a, b) => a.label.localeCompare(b.label, undefined, { numeric: true })) }));
        }
        function renderPickupGroups(project, groups) {
            return `<div class="fi-pick-groups">${groups.map(group => `<section class="fi-pick-group"><h4>${M.esc(group.label)}<span>${group.entries.length}</span></h4><div>${group.entries.map(entry => { const reservation = entry.reservation; const unit = entry.unit ? tr('unit_' + String(entry.unit).toLowerCase().replace(/[^a-z0-9]+/g, '_'), entry.unit) : ''; return `<div class="fi-pick-row"><span class="fi-reservation-marker">${icon('box', 15)}</span><span><b>${M.esc(entry.label)}</b><small>${M.esc(entry.partNumber || reservation.line_id || '')}</small></span><strong>${M.esc(compactNumber(entry.quantity))} ${M.esc(unit)}</strong>${canWrite() && project.status === 'released' ? `<button type="button" data-reservation-action="release" data-reservation-id="${M.esc(reservation.id)}">${M.esc(tr('release'))}</button><button type="button" data-reservation-action="consume" data-reservation-id="${M.esc(reservation.id)}">${M.esc(tr('consume'))}</button>` : ''}</div>`; }).join('')}</div></section>`).join('')}</div>`;
        }
        function printPickList(project) {
            if (!canPrint() || !project) return;
            const reservations = asList(project.reservations || state.projectShortages?.reservations).filter(reservation => M.quantityCompare(reservation.remaining ?? reservation.remaining_quantity ?? reservation.quantity ?? '0', '0') > 0);
            const groups = groupActiveReservations(project, reservations);
            if (!groups.length) return;
            const popup = window.open('', '_blank');
            if (!popup) return showNotice(tr('print_popup_blocked'), true);
            popup.opener = null;
            popup.addEventListener('load', () => { popup.focus(); popup.print(); }, { once: true });
            popup.document.open();
            popup.document.write(`<!doctype html><html lang="${M.esc(ctx.lang || 'de')}"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>${M.esc(tr('stock_holds'))} · ${M.esc(project.name)}</title><style>@page{size:A4;margin:16mm}*{box-sizing:border-box}body{font:14px/1.45 system-ui,sans-serif;color:#20252a;margin:0}h1{font-size:24px;margin:0 0 4px}p{color:#59636e;margin:0 0 20px}.group{break-inside:avoid;margin:18px 0}.group h2{font-size:15px;padding:8px 0;border-bottom:2px solid #394d48;margin:0 0 5px}.row{display:grid;grid-template-columns:minmax(0,1fr) auto;gap:12px;padding:10px 0;border-bottom:1px solid #d7dcda}.row small{display:block;color:#66716e;font-size:11px}.qty{font-weight:700;white-space:nowrap}@media print{.group{break-inside:avoid}}</style></head><body><h1>${M.esc(project.name)}</h1><p>${M.esc(tr('stock_holds'))} · ${M.esc(new Intl.DateTimeFormat(undefined,{dateStyle:'medium'}).format(new Date()))}</p>${groups.map(group => `<section class="group"><h2>${M.esc(group.label)}</h2>${group.entries.map(entry => { const unit = entry.unit ? tr('unit_' + String(entry.unit).toLowerCase().replace(/[^a-z0-9]+/g, '_'), entry.unit) : ''; return `<div class="row"><span>${M.esc(entry.label)}<small>${M.esc(entry.partNumber)}</small></span><span class="qty">${M.esc(compactNumber(entry.quantity))} ${M.esc(unit)}</span></div>`; }).join('')}</section>`).join('')}</body></html>`);
            popup.document.close();
        }
        function renderLocations() {
            const rows = asList(state.locations), selected = state.locationSelected?.id === state.locationId ? state.locationSelected : rows.find(item => item.id === state.locationId);
            return `<div class="fi-page-toolbar"><div><p class="fi-eyebrow">${M.esc(tr('storage_map'))}</p><h1>${M.esc(tr('locations'))}</h1><p>${M.esc(tr('locations_intro'))}</p></div><div class="fi-page-actions">${canWrite() ? `<button type="button" class="fi-primary" data-action="new-location">${icon('plus', 16)}${M.esc(tr('new_location'))}</button>` : ''}</div></div><div class="fi-entity-layout ${state.mobilePanel === 'detail' ? 'fi-show-detail' : ''}"><section class="fi-entity-list">${rows.length ? rows.map(item => `<button type="button" data-select-location="${M.esc(item.id)}" class="fi-entity-row ${selected?.id === item.id ? 'is-selected' : ''}"><span class="fi-location-mark">${icon('pin', 17)}</span><span><b>${M.esc(item.name)}</b><small>${M.esc(item.kind || tr('storage_location'))}${item.parent_id ? ` · ${M.esc(rows.find(row => row.id === item.parent_id)?.name || '')}` : ''}</small></span><i>${M.esc(String(item.item_count ?? ''))}</i></button>`).join('') : `<div class="fi-empty-compact"><p>${M.esc(tr('no_locations'))}</p></div>`}</section><section class="fi-entity-detail">${selected ? `<button type="button" class="fi-mobile-back" data-action="back-locations">${icon('back', 16)}${M.esc(tr('back_to_list'))}</button><div class="fi-location-hero"><span>${icon('pin', 28)}</span><div><p class="fi-eyebrow">${M.esc(tr('storage_location'))}</p><h2>${M.esc(selected.name)}</h2><p>${M.esc(selected.kind || '')}</p></div></div><div class="fi-detail-toolbar"><button type="button" class="fi-tool-button" data-action="print-location-label" ${canPrint() ? '' : 'disabled'}>${icon('barcode', 15)}${M.esc(tr('print_location_label'))}</button>${canWrite() ? `<button type="button" class="fi-tool-button" data-action="edit-location">${icon('edit', 15)}${M.esc(tr('edit'))}</button><button type="button" class="fi-danger-button" data-action="archive-location">${M.esc(tr('archive_location'))}</button>` : ''}</div><dl class="fi-detail-fields"><div><dt>${M.esc(tr('parent_location'))}</dt><dd>${M.esc(rows.find(row => row.id === selected.parent_id)?.name || tr('root_location'))}</dd></div><div><dt>${M.esc(tr('items_stored'))}</dt><dd>${M.esc(String(selected.item_count ?? '—'))}</dd></div></dl>${selected.description ? `<p class="fi-description">${M.esc(selected.description)}</p>` : ''}</section>` : `<section class="fi-entity-detail"><div class="fi-detail-empty"><div class="fi-detail-empty-icon">${icon('pin', 25)}</div><h2>${M.esc(tr('select_location'))}</h2><p>${M.esc(tr('select_location_help'))}</p></div></section>`}</div>`;
        }
        async function chooseItem() {
            const dialog = document.createElement('dialog'); dialog.className = 'fi-dialog fi-picker-dialog';
            const titleId = 'fi-' + crypto.randomUUID() + '-picker-title';
            dialog.setAttribute('aria-labelledby', titleId);
            const hostWidth = host.clientWidth || host.getBoundingClientRect?.().width || 0;
            dialog.dataset.hostWidth = hostWidth > 0 && hostWidth <= 760 ? 'narrow' : 'wide';
            if (hostWidth > 0) dialog.style.setProperty('--fi-host-width', `${hostWidth}px`);
            dialog.innerHTML = `<form method="dialog" class="fi-dialog-form"><header class="fi-dialog-head"><div><p class="fi-kicker">${M.esc(tr('bill_of_materials'))}</p><h2 id="${M.esc(titleId)}">${M.esc(tr('choose_part'))}</h2></div><button type="button" data-close aria-label="${M.esc(tr('close'))}">×</button></header><div class="fi-dialog-body"><label class="fi-search fi-picker-search">${icon('search', 16)}<input type="search" data-picker-search placeholder="${M.esc(tr('search_parts'))}"></label><div class="fi-picker-results" data-picker-results>${M.esc(tr('loading_parts'))}</div></div><footer class="fi-dialog-foot"><button type="button" data-cancel>${M.esc(tr('cancel'))}</button></footer></form>`;
            host.append(dialog); dialog.showModal();
            let resolve; const promise = new Promise(done => { resolve = done; });
            let timer = 0, controller = null;
            const onAbort = () => close(null);
            const close = value => { if (!dialog.isConnected) return; clearTimeout(timer); controller?.abort(); state.signal.removeEventListener('abort', onAbort); dialog.close(); dialog.remove(); resolve(value || null); };
            dialog.addEventListener('cancel', event => { event.preventDefault(); close(null); }, { once: true });
            dialog.querySelectorAll('[data-close],[data-cancel]').forEach(button => button.addEventListener('click', () => close(null), { once: true }));
            const input = dialog.querySelector('[data-picker-search]'), results = dialog.querySelector('[data-picker-results]');
            const load = async () => {
                clearTimeout(timer); controller?.abort(); controller = new AbortController();
                try {
                    const body = await M.query({ operation: 'search', entity: 'items', query: input.value, limit: 30, offset: 0 }, controller.signal);
                    if (!dialog.isConnected || controller.signal.aborted) return;
                    results.innerHTML = asList(body?.items).map(item => `<button type="button" data-pick-item="${M.esc(item.id)}"><span>${sprite(item, true)}</span><b>${M.esc(item.name)}</b><small>${M.esc(item.manufacturer || '')} ${M.esc(item.part_number || '')}</small></button>`).join('') || `<p>${M.esc(tr('no_match_title'))}</p>`;
                } catch (error) { if (error?.name !== 'AbortError') results.textContent = noteText(error); }
            };
            input.addEventListener('input', () => { timer = setTimeout(load, 180); });
            results.addEventListener('click', event => { const button = event.target.closest('[data-pick-item]'); if (button) close(button.dataset.pickItem); });
            state.signal.addEventListener('abort', onAbort, { once: true });
            if (state.signal.aborted) close(null);
            void load(); input.focus();
            return promise;
        }
        async function projectForm(project = null, retryValues = null) {
            if (!canWrite()) return false;
            const result = await D.openForm(dialogOptions({ title: project ? 'edit_project' : 'new_project', description: 'projects_intro', fields: [
                { key: 'name', label: 'project_name', type: 'text', required: true }, { key: 'description', label: 'description', type: 'textarea', rows: 3, wide: true }
            ], values: retryValues || { name: project?.name || '', description: project?.description || '' }, submitLabel: project ? 'save' : 'create_project' }));
            if (!result) return false;
            let commandAcknowledged = false;
            const saved = await withBusy(async () => {
                const saved = await apiCommand({ operation: project ? 'update' : 'create', entity: 'projects', id: project?.id, expected_version: project?.version,
                    data: { ...result, ...(project ? { lines: asList(project.lines).map(line => ({ id: line.id, item_id: line.item_id, quantity: line.quantity, note: line.note || '' })) } : {}) } });
                commandAcknowledged = true;
                const record = saved?.project || saved; state.project = record; state.notice = project ? tr('project_saved') : tr('project_created');
                await loadProjects(); if (record?.id) await selectProject(record.id);
            });
            if (!saved && !commandAcknowledged && canWrite() && !state.disposed && !state.signal.aborted) return projectForm(project, result);
            return saved;
        }
        async function addProjectLine(retryDraft = null) {
            const project = state.project; if (!project || project.status !== 'draft' || !canWrite()) return;
            const itemId = retryDraft?.itemId || await chooseItem(); if (!itemId) return;
            const result = await D.openForm(dialogOptions({ title: 'add_line', fields: [
                { key: 'quantity', label: 'quantity', type: 'text', required: true, inputmode: 'decimal' },
                { key: 'note', label: 'note', type: 'text', wide: true }
            ], values: retryDraft?.values || { quantity: '1', note: '' }, submitLabel: 'add_line', validate: values =>
                M.quantityUnits(values.quantity) == null || M.quantityCompare(values.quantity, '0') <= 0 ? { message: 'invalid_quantity', field: 'quantity' } : null }));
            if (!result) return false;
            result.quantity = M.normalizeQuantity(result.quantity);
            let commandAcknowledged = false;
            const saved = await updateProject({ lines: [...asList(project.lines), { id: '', item_id: itemId, quantity: result.quantity, note: result.note || '' }] }, () => { commandAcknowledged = true; });
            if (!saved && !commandAcknowledged && canWrite() && !state.disposed && !state.signal.aborted) return addProjectLine({ itemId, values: result });
            return saved;
        }
        async function removeProjectLine(lineId) {
            const project = state.project; if (!project || project.status !== 'draft') return;
            await updateProject({ lines: asList(project.lines).filter(line => line.id !== lineId) });
        }
        async function updateProject(patch, onAcknowledged = null) {
            const project = state.project; if (!project || !canWrite()) return;
            return withBusy(async () => {
                const saved = await apiCommand({ operation: 'update', entity: 'projects', id: project.id, expected_version: project.version,
                    data: { name: project.name, description: project.description || '', ...patch, lines: (patch.lines || asList(project.lines)).map(line => ({ id: line.id, item_id: line.item_id, quantity: line.quantity, note: line.note || '' })) } });
                onAcknowledged?.();
                state.project = saved?.project || saved; state.notice = tr('project_saved'); await loadProjects(); await selectProject(project.id);
            });
        }
        async function projectStatus(status) {
            const project = state.project; if (!project || !canWrite()) return;
            const confirmKey = { released: 'release_project_confirm', cancelled: 'cancel_project_confirm', completed: 'complete_project_confirm', draft: 'reopen_project_confirm' }[status];
            if (!await D.openConfirm(dialogOptions({ title: 'status_' + status, message: confirmKey, confirmLabel: status === 'cancelled' ? 'cancel_project' : status === 'completed' ? 'complete_project' : status === 'draft' ? 'reopen_revision' : 'release_project' }))) return;
            await withBusy(async () => {
                await apiCommand({ operation: 'project_status', entity: 'projects', id: project.id, expected_version: project.version, data: { status } });
                state.notice = status === 'released' ? tr('project_released') : status === 'cancelled' ? tr('project_cancelled') : status === 'completed' ? tr('project_completed') : tr('project_reopened');
                await loadProjects(); await selectProject(project.id);
            });
        }
        async function reserveLine(lineId, retryDraft = null) {
            const project = state.project, line = asList(project?.lines).find(value => value.id === lineId);
            if (!project || !line || !canWrite()) return false;
            const availability = await M.query({ operation: 'availability', entity: 'items', id: line.item_id }, state.signal);
            const allStocks = asList(availability?.stocks || availability?.locations);
            const stocks = allStocks.filter(stock => M.quantityCompare(stock.available ?? stock.quantity ?? '0', '0') > 0);
            if (retryDraft?.stock && !stocks.some(stock => stock.id === retryDraft.stock.id)) {
                const latest = allStocks.find(stock => stock.id === retryDraft.stock.id);
                stocks.unshift({ ...(latest || retryDraft.stock), available: latest?.available ?? '0' });
            }
            if (!stocks.length) return showNotice(tr('no_available_stock'), true);
            const result = await D.openForm(dialogOptions({ title: 'reserve', fields: [
                { key: 'stock_id', label: 'storage_location', type: 'select', required: true, options: stocks.map(stock => ({ value: stock.id, label: `${stock.location_name || stock.location || stock.location_id} · ${compactNumber(stock.available ?? stock.quantity)}` })) },
                { key: 'quantity', label: 'quantity', type: 'text', required: true, inputmode: 'decimal' }
            ], values: retryDraft?.values || { stock_id: stocks[0].id, quantity: line.quantity || '1' }, submitLabel: 'reserve', validate: values =>
                M.quantityUnits(values.quantity) == null || M.quantityCompare(values.quantity, '0') <= 0 ? { message: 'invalid_quantity', field: 'quantity' } : null }));
            if (!result) return false;
            result.quantity = M.normalizeQuantity(result.quantity);
            const selectedStock = stocks.find(stock => stock.id === result.stock_id);
            let commandAcknowledged = false;
            const saved = await withBusy(async () => {
                await apiCommand({ operation: 'reserve', entity: 'projects', id: project.id, expected_version: project.version,
                    data: { project_id: project.id, line_id: line.id, stock_id: result.stock_id, quantity: result.quantity } });
                commandAcknowledged = true;
                state.notice = tr('reservation_created'); await loadProjects(); await selectProject(project.id);
            });
            if (!saved && !commandAcknowledged && canWrite() && !state.disposed && !state.signal.aborted) return reserveLine(lineId, { stock: selectedStock, values: result });
            return saved;
        }
        async function reservationAction(action, reservationId, retryValues = null) {
            const project = state.project;
            const reservation = asList(project?.reservations || state.projectShortages?.reservations).find(item => item.id === reservationId);
            if (!project || !reservation || !canWrite()) return showNotice(tr('reservation_not_found'), true);
            const remaining = String(reservation.remaining ?? reservation.remaining_quantity ?? reservation.quantity ?? '1');
            const result = await D.openForm(dialogOptions({ title: action === 'consume' ? 'consume_reserved' : 'release_reserved', fields: [
                { key: 'quantity', label: 'quantity', type: 'text', required: true, inputmode: 'decimal' }
            ], values: retryValues || { quantity: remaining }, submitLabel: action, validate: values =>
                M.quantityUnits(values.quantity) == null || M.quantityCompare(values.quantity, '0') <= 0 ? { message: 'invalid_quantity', field: 'quantity' } : null }));
            if (!result) return false;
            result.quantity = M.normalizeQuantity(result.quantity);
            let commandAcknowledged = false;
            const saved = await withBusy(async () => {
                await apiCommand({ operation: action, entity: 'projects', id: project.id, expected_version: project.version, data: { reservation_id: reservationId, quantity: result.quantity } });
                commandAcknowledged = true;
                state.notice = action === 'consume' ? tr('stock_consumed') : tr('reservation_released'); await loadProjects(); await selectProject(project.id);
            });
            if (!saved && !commandAcknowledged && canWrite() && !state.disposed && !state.signal.aborted) return reservationAction(action, reservationId, result);
            return saved;
        }
        async function locationForm(location = null, retryValues = null) {
            if (!canWrite()) return false;
            const result = await D.openForm(dialogOptions({ title: location ? 'edit_location' : 'new_location', fields: [
                { key: 'name', label: 'location', type: 'text', required: true },
                { key: 'kind', label: 'location_kind', type: 'select', options: ['room', 'cabinet', 'shelf', 'drawer', 'box', 'rack', 'other'] },
                { key: 'parent_id', label: 'parent_location', type: 'select', allowEmpty: true, emptyLabel: 'root_location', options: asList(state.locations).filter(item => item.id !== location?.id).map(item => ({ value: item.id, label: item.name })) },
                { key: 'description', label: 'description', type: 'textarea', wide: true }
            ], values: retryValues || { name: location?.name || '', kind: location?.kind || 'box', parent_id: location?.parent_id || '', description: location?.description || '' }, submitLabel: location ? 'save' : 'create_location' }));
            if (!result) return false;
            let commandAcknowledged = false;
            const saved = await withBusy(async () => {
                await apiCommand({ operation: location ? 'update' : 'create', entity: 'locations', id: location?.id, expected_version: location?.version, data: result });
                commandAcknowledged = true;
                state.notice = location ? tr('location_saved') : tr('location_created'); await loadLocations(); draw();
            });
            if (!saved && !commandAcknowledged && canWrite() && !state.disposed && !state.signal.aborted) return locationForm(location, result);
            return saved;
        }
        async function archiveLocation() {
            const location = state.locations.find(item => item.id === state.locationId); if (!location) return;
            if (!await D.openConfirm(dialogOptions({ title: 'archive_location', confirmLabel: 'archive' }))) return;
            await withBusy(async () => {
                await apiCommand({ operation: 'archive', entity: 'locations', id: location.id, expected_version: location.version, data: {} });
                state.locationId = ''; state.notice = tr('location_archived'); await loadLocations(); await loadItems();
            });
        }
        return { renderProjects, renderLocations, projectForm, addProjectLine, removeProjectLine, updateProject, projectStatus, reserveLine, reservationAction, locationForm, archiveLocation, printPickList };
    }
    window.FindItProjects = { create };
})();
