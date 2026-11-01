(function () {
    'use strict';

    const M = window.FindItModel, D = window.FindItDialogs, CSV = window.FindItCSV;
    const esc = M.esc;
    function create(c) {
        const { host, state, tr, canWrite, draw, apiQuery, apiCommand, withBusy, loadItems, loadProjects, selectItem, showNotice, noteText, icon } = c;
        const activeDialog = new Set();
        function close(dialog) { if (!dialog?.isConnected) return; if (dialog.open) dialog.close(); dialog.remove(); activeDialog.delete(dialog); }
        function dialog(title, body, actions = '') {
            const node = document.createElement('dialog'); node.className = 'fi-dialog fi-csv-dialog';
            node.innerHTML = `<div class="fi-dialog-form"><header class="fi-dialog-head"><div><p class="fi-kicker">${esc(tr('inventory_import'))}</p><h2>${esc(tr(title))}</h2></div><button type="button" data-close aria-label="${esc(tr('close'))}">×</button></header><div class="fi-dialog-body" data-dialog-body>${body}</div>${actions ? `<footer class="fi-dialog-foot" data-dialog-actions>${actions}</footer>` : ''}</div>`;
            host.append(node); node.showModal(); activeDialog.add(node);
            node.addEventListener('cancel', event => { event.preventDefault(); close(node); }, { once: true });
            node.querySelector('[data-close]').addEventListener('click', () => close(node), { once: true });
            return node;
        }
        function actionButton(action, text, primary = false, disabled = false) {
            return `<button type="button" data-csv-action="${esc(action)}" class="${primary ? 'fi-primary' : ''}" ${disabled ? 'disabled' : ''}>${esc(tr(text))}</button>`;
        }
        async function csvWizard(entity) {
            if (!canWrite()) return;
            if (entity === 'project_lines' && !state.projects.length) await loadProjects();
            const isProject = entity === 'project_lines', projectId = isProject ? state.project?.id || '' : '';
            const body = `<div class="fi-csv-stepper"><span class="is-current">1 <b>${esc(tr('choose_file'))}</b></span><span>2 <b>${esc(tr('map_columns'))}</b></span><span>3 <b>${esc(tr('review_rows'))}</b></span></div><p class="fi-dialog-copy">${esc(tr('csv_import_intro'))}</p><label class="fi-form-field"><span>${esc(tr('csv_file'))}</span><input type="file" data-csv-file accept=".csv,text/csv" required><small>${esc(tr('csv_file_help'))}</small></label>${isProject ? `<label class="fi-form-field"><span>${esc(tr('target_project'))}</span><select data-csv-project><option value="">${esc(tr('choose_project'))}</option>${M.asArray(state.projects).filter(project => project.status === 'draft').map(project => `<option value="${esc(project.id)}" ${project.id === projectId ? 'selected' : ''}>${esc(project.name)}</option>`).join('')}</select><small>${esc(tr('csv_project_help'))}</small></label>` : ''}<p class="fi-form-error" data-csv-error role="alert" hidden></p>`;
            const node = dialog('import_csv', body, `${actionButton('cancel', 'cancel')}${actionButton('next', 'continue', true, true)}`);
            const bodyNode = node.querySelector('[data-dialog-body]');
            let file = null, csvText = '', parsed = null, mapping = {}, preview = null, currentEntity = entity, currentProject = projectId;
            let resolutions = {}, skipRows = new Set(), previewPage = 0, projectVersion;
            const requestAbort = new AbortController();
            const stop = () => requestAbort.abort();
            state.signal.addEventListener('abort', stop, { once: true });
            node.addEventListener('close', () => { stop(); state.signal.removeEventListener('abort', stop); }, { once: true });
            node.addEventListener('click', async event => {
                event.stopPropagation();
                const actionButtonNode = event.target.closest('[data-csv-action]'); if (!actionButtonNode) return;
                const action = actionButtonNode.dataset.csvAction;
                const error = node.querySelector('[data-csv-error]') || createErrorNode(bodyNode);
                error.hidden = true; error.textContent = '';
                if (action === 'cancel') { close(node); return; }
                if (action === 'previous' || action === 'next-page') { previewPage += action === 'previous' ? -1 : 1; renderPreview(); return; }
                if (action === 'mapping') { renderMapping(); return; }
                if (action === 'next' && !parsed) {
                    file = node.querySelector('[data-csv-file]')?.files?.[0];
                    if (currentEntity === 'project_lines') currentProject = node.querySelector('[data-csv-project]')?.value || '';
                    if (!file) { error.textContent = tr('choose_csv_file_error'); error.hidden = false; return; }
                    if (file.size > 10 * 1024 * 1024) { error.textContent = tr('csv_too_large'); error.hidden = false; return; }
                    if (currentEntity === 'project_lines' && !currentProject) { error.textContent = tr('choose_project_error'); error.hidden = false; return; }
                    try {
                        csvText = new TextDecoder('utf-8', { fatal: true }).decode(await file.arrayBuffer());
                    } catch (_) { error.textContent = tr('csv_parse_error'); error.hidden = false; return; }
                    try { parsed = CSV.parse(csvText); }
                    catch (reason) { error.textContent = tr('csv_parse_error') + ' ' + tr('csv_error_' + String(reason.message || reason)); error.hidden = false; return; }
                    mapping = CSV.guessMapping(parsed.headers, currentEntity);
                    renderMapping(); return;
                }
                if (action === 'preview') {
                    mapping = Object.fromEntries([...bodyNode.querySelectorAll('[data-csv-map]')].filter(select => select.value).map(select => [select.dataset.csvMap, select.value]));
                    const required = currentEntity === 'items' ? ['name'] : ['quantity'];
                    if (required.some(field => !mapping[field])) { error.textContent = tr('mapping_required_error'); error.hidden = false; return; }
                    actionButtonNode.disabled = true; actionButtonNode.textContent = tr('working');
                    try {
                        if (currentProject) projectVersion = (await apiQuery({ operation: 'get', entity: 'projects', id: currentProject })).version;
                        preview = await M.csvPreview({ entity: currentEntity, csv: csvText, mapping, project_id: currentProject || undefined, expected_version: projectVersion }, requestAbort.signal);
                        if (!node.isConnected) return;
                        resolutions = {}; skipRows = new Set(); previewPage = 0; renderPreview();
                    } catch (reason) { error.textContent = noteText(reason); error.hidden = false; actionButtonNode.disabled = false; actionButtonNode.textContent = tr('preview_import'); }
                    return;
                }
                if (action === 'import') {
                    const rows = M.asArray(preview?.rows);
                    const unresolved = rows.filter(row => row.errors?.length && !row.resolved_item_id && !resolutions[row.row] && !skipRows.has(Number(row.row)));
                    if (unresolved.length) { error.textContent = tr('resolve_or_skip_rows'); error.hidden = false; return; }
                    if (!rows.some(row => !skipRows.has(Number(row.row)))) { error.textContent = tr('nothing_to_import'); error.hidden = false; return; }
                    actionButtonNode.disabled = true; actionButtonNode.textContent = tr('working');
                    try {
                        const payload = { entity: currentEntity, csv: csvText, mapping, project_id: currentProject || undefined,
                            expected_version: projectVersion, resolutions, skip_rows: [...skipRows].sort((a, b) => a - b) };
                        const digest = await crypto.subtle.digest('SHA-256', new TextEncoder().encode(JSON.stringify(payload)));
                        const receiptKey = 'aurago.findit.csv-retry.' + [...new Uint8Array(digest)].map(byte => byte.toString(16).padStart(2, '0')).join('');
                        let requestId = state.csvImportRetry?.key === receiptKey ? state.csvImportRetry.id : '';
                        try { requestId ||= sessionStorage.getItem(receiptKey); } catch (_) { /* Memory receipt remains available. */ }
                        requestId ||= crypto.randomUUID(); state.csvImportRetry = { key: receiptKey, id: requestId };
                        try { sessionStorage.setItem(receiptKey, requestId); } catch (_) { /* Storage may be unavailable. */ }
                        const result = await M.csvImport({ ...payload, request_id: requestId }, requestAbort.signal);
                        try { sessionStorage.removeItem(receiptKey); } catch (_) { /* Optional receipt storage. */ }
                        state.csvImportRetry = null;
                        if (!node.isConnected) return;
                        close(node);
                        showNotice(`${tr('csv_rows_count')}: ${String(result?.imported ?? result?.created ?? '')}`);
                        if (currentEntity === 'items') await loadItems();
                        else { await loadProjects(); await c.selectProject(currentProject); }
                    } catch (reason) { error.textContent = noteText(reason); error.hidden = false; actionButtonNode.disabled = false; actionButtonNode.textContent = tr('import_rows'); }
                }
            });
            node.addEventListener('change', event => {
                event.stopPropagation();
                if (event.target.matches('[data-csv-file]')) node.querySelector('[data-csv-action="next"]').disabled = !event.target.files?.length;
                if (event.target.matches('[data-csv-resolution]')) { resolutions[event.target.dataset.csvResolution] = event.target.value; }
                if (event.target.matches('[data-csv-skip]')) {
                    const row = Number(event.target.dataset.csvSkip); event.target.checked ? skipRows.add(row) : skipRows.delete(row);
                    if (event.target.checked) delete resolutions[row];
                }
            });
            state.signal.addEventListener('abort', () => close(node), { once: true });
            function createErrorNode(parent) {
                const error = document.createElement('p'); error.className = 'fi-form-error'; error.dataset.csvError = ''; error.setAttribute('role', 'alert'); parent.append(error); return error;
            }
            function renderMapping() {
                const keys = CSV.mappingFields(currentEntity), required = currentEntity === 'items' ? ['name'] : ['quantity'];
                const headers = [`<option value="">${esc(tr('skip_column'))}</option>`, ...parsed.headers.map(header => `<option value="${esc(header)}">${esc(header)}</option>`)].join('');
                bodyNode.innerHTML = `<div class="fi-csv-stepper"><span>1 <b>${esc(tr('choose_file'))}</b></span><span class="is-current">2 <b>${esc(tr('map_columns'))}</b></span><span>3 <b>${esc(tr('review_rows'))}</b></span></div><p class="fi-dialog-copy">${esc(tr('column_mapping_help'))}</p><div class="fi-mapping-grid">${keys.map(key => `<label>${esc(tr('field_' + key, key.replaceAll('_', ' ')))}${required.includes(key) ? ' *' : ''}<select data-csv-map="${esc(key)}">${headers}</select></label>`).join('')}</div><p class="fi-csv-count">${esc(tr('csv_rows_count'))}: ${esc(String(parsed.rows.length))}</p><p class="fi-form-error" data-csv-error role="alert" hidden></p>`;
                for (const [key, header] of Object.entries(mapping)) { const select = bodyNode.querySelector(`[data-csv-map="${CSS.escape(key)}"]`); if (select) select.value = header; }
                node.querySelector('[data-dialog-actions]').innerHTML = `${actionButton('cancel', 'cancel')}${actionButton('preview', 'preview_import', true)}`;
            }
            function renderPreview() {
                const rows = M.asArray(preview?.rows);
                const good = Number(preview?.valid_count) || 0, bad = Number(preview?.invalid_count) || 0;
                previewPage = Math.max(0, Math.min(previewPage, Math.ceil(rows.length / 100) - 1));
                const rowHtml = rows.slice(previewPage * 100, previewPage * 100 + 100).map(row => {
                    const candidateOptions = M.asArray(row.candidates).map(candidate => `<option value="${esc(candidate.id)}" ${resolutions[row.row] === candidate.id || row.resolved_item_id === candidate.id ? 'selected' : ''}>${esc(candidate.name || candidate.part_number || candidate.id)}${candidate.part_number ? ` · ${esc(candidate.part_number)}` : ''}</option>`).join('');
                    return `<tr class="${row.errors?.length ? 'is-invalid' : 'is-valid'}"><td>${esc(String(row.row))}</td><td>${esc(row.data?.name || row.data?.part_number || row.data?.item_id || '')}</td><td>${esc(row.data?.quantity || '')}</td><td>${row.errors?.length ? `<span>${esc(M.asArray(row.errors).map(code => tr('csv_error_' + code, code)).join(', '))}</span>${candidateOptions ? `<select data-csv-resolution="${esc(String(row.row))}"><option value="">${esc(tr('choose_match'))}</option>${candidateOptions}</select>` : ''}` : `<span class="fi-valid-mark">${esc(tr('valid'))}</span>`}</td><td><label class="fi-skip-row"><input type="checkbox" data-csv-skip="${esc(String(row.row))}" ${skipRows.has(Number(row.row)) ? 'checked' : ''}><span>${esc(tr('skip'))}</span></label></td></tr>`;
                }).join('');
                bodyNode.innerHTML = `<div class="fi-csv-stepper"><span>1 <b>${esc(tr('choose_file'))}</b></span><span>2 <b>${esc(tr('map_columns'))}</b></span><span class="is-current">3 <b>${esc(tr('review_rows'))}</b></span></div><div class="fi-csv-summary"><span class="is-good"><b>${good}</b>${esc(tr('valid_rows'))}</span><span class="is-bad"><b>${bad}</b>${esc(tr('rows_need_attention'))}</span><small>${esc(tr('all_or_nothing_note'))}</small></div><div class="fi-csv-table-wrap"><table class="fi-csv-table"><thead><tr><th>#</th><th>${esc(tr('part'))}</th><th>${esc(tr('quantity'))}</th><th>${esc(tr('review'))}</th><th>${esc(tr('skip'))}</th></tr></thead><tbody>${rowHtml}</tbody></table></div>${rows.length > 100 ? `<p class="fi-csv-count">${esc(tr('csv_preview_limit', '100'))}</p>` : ''}<p class="fi-form-error" data-csv-error role="alert" hidden></p>`;
                const oldLimit = bodyNode.querySelector('.fi-csv-count'); if (oldLimit) oldLimit.remove();
                if (rows.length > 100) {
                    const paging = document.createElement('nav'); paging.className = 'fi-detail-toolbar'; paging.setAttribute('aria-label', tr('review_rows'));
                    paging.innerHTML = `${actionButton('previous', 'previous_page', false, previewPage === 0)}<span>${previewPage * 100 + 1}–${Math.min(rows.length, (previewPage + 1) * 100)} / ${rows.length}</span>${actionButton('next-page', 'next_page', false, (previewPage + 1) * 100 >= rows.length)}`;
                    bodyNode.append(paging);
                }
                node.querySelector('[data-dialog-actions]').innerHTML = `${actionButton('cancel', 'cancel')}${actionButton('mapping', 'map_columns')}${actionButton('import', 'import_rows', true, !rows.length)}`;
            }
        }
        async function exportCSV(entity, projectId = '') {
            try {
                const response = await M.csvExport(entity, projectId, state.signal), blob = await response.blob();
                const url = URL.createObjectURL(blob), anchor = document.createElement('a');
                anchor.href = url; anchor.download = response.headers.get('Content-Disposition')?.match(/filename="?([^";]+)"?/i)?.[1] || `findit-${entity}.csv`;
                anchor.click(); setTimeout(() => URL.revokeObjectURL(url), 2000);
                showNotice(tr('csv_exported'));
            } catch (error) { showNotice(noteText(error), true); }
        }
        function deepLink(idValue, entity = 'item') {
            const id = M.safeId(idValue); if (!id || !['item', 'location'].includes(entity)) return '';
            const url = new URL(window.location.href); url.search = ''; url.searchParams.set('app', 'findit'); url.searchParams.set(entity, id); url.hash = '';
            return url.href;
        }
        function findFindItLink(value) {
            try {
                const url = new URL(String(value || ''), window.location.href);
                if (!['http:', 'https:'].includes(url.protocol) || url.origin !== window.location.origin || url.username || url.password || url.pathname !== window.location.pathname || url.hash) return '';
                const entries = [...url.searchParams.entries()], apps = url.searchParams.getAll('app'), items = url.searchParams.getAll('item'), locations = url.searchParams.getAll('location');
                if (entries.length !== 2 || apps.length !== 1 || apps[0] !== 'findit' || (items.length + locations.length) !== 1) return '';
                const item = M.safeId(items[0]), location = M.safeId(locations[0]);
                if (!!item === !!location) return '';
                return item ? { entity: 'item', id: item } : { entity: 'location', id: location };
            } catch (_) { return ''; }
        }
        async function loadQR() {
            if (!window.AuraQRDialog?.open) await window.AuraDesktopModules?.loadAppAssets('qr');
            if (!window.AuraQRDialog?.open) throw new Error(tr('qr_unavailable'));
        }
        function shortCode(id, entity) { return (entity === 'location' ? 'L-' : 'I-') + String(id).slice(0, 13).toUpperCase(); }
        async function scanQR() {
            const chooser = D.openInfo({ host, t: tr, title: 'scan_qr', html: `<p>${esc(tr('scan_method_help'))}</p><div class="fi-action-list"><button type="button" data-scan-method="camera">${icon('qr', 18)} ${esc(tr('scan_camera_upload'))}</button><button type="button" data-scan-method="manual">${icon('edit', 18)} ${esc(tr('enter_identifier'))}</button></div>` });
            state.signal.addEventListener('abort', chooser.close, { once: true });
            chooser.element.addEventListener('click', event => {
                const method = event.target.closest('[data-scan-method]')?.dataset.scanMethod; if (!method) return;
                chooser.close(); state.signal.removeEventListener('abort', chooser.close);
                if (method === 'manual') void enterIdentifier(); else void scanImageQR();
            });
        }
        async function enterIdentifier() {
            const input = await D.openForm({ host, t: tr, signal: state.signal, title: 'enter_identifier', description: 'identifier_help', fields: [{ key: 'identifier', label: 'identifier', type: 'text', required: true, wide: true }], submitLabel: 'search' });
            if (!input || state.disposed) return;
            try {
                const raw = input.identifier.trim(), target = findFindItLink(raw);
                if (target) { showScanTarget(target, raw); return; }
                const match = /^([IL])-([a-f0-9]{8}-[a-f0-9]{4})$/i.exec(raw);
                if (!match) { showNotice(tr('invalid_identifier'), true); return; }
                const entity = match[1].toUpperCase() === 'L' ? 'location' : 'item';
                const found = await apiQuery({ operation: 'search', entity: entity + 's', query: 'id:' + match[2].toLowerCase(), limit: 200 });
                if (!found.total) { showNotice(tr('error_not_found'), true); return; }
                if (found.total === 1) { showScanTarget({ entity, id: found.items[0].id }, found.items[0].name); return; }
                const candidates = D.openInfo({ host, t: tr, title: 'choose_match', html: `<div class="fi-action-list">${found.items.map(item => `<button type="button" data-candidate="${esc(item.id)}">${esc(item.name)} · ${esc(item.id)}</button>`).join('')}</div>` });
                state.signal.addEventListener('abort', candidates.close, { once: true });
                candidates.element.addEventListener('click', event => { const id = event.target.closest('[data-candidate]')?.dataset.candidate; if (!id) return; candidates.close(); showScanTarget({ entity, id }, raw); });
            } catch (error) { if (error?.name !== 'AbortError') showNotice(noteText(error), true); }
        }
        function showScanTarget(target, text) {
            if (state.disposed) return;
            const detail = target ? `<div class="fi-scan-result">${icon('check', 20)}<div><b>${esc(tr('findit_label_found'))}</b><small>${esc(tr(target.entity === 'item' ? 'scan_item_confirm' : 'scan_location_confirm'))}</small></div></div><p>${esc(text)}</p><button type="button" class="fi-primary" data-scan-open="${esc(target.id)}" data-scan-entity="${esc(target.entity)}">${esc(tr(target.entity === 'item' ? 'open_item' : 'open_location'))}</button>` : `<div class="fi-scan-result is-plain">${icon('qr', 20)}<div><b>${esc(tr('qr_text_found'))}</b><small>${esc(tr('qr_text_passive'))}</small></div></div><pre class="fi-scan-text">${esc(text)}</pre>`;
            const info = D.openInfo({ host, t: tr, title: 'scan_result', html: detail });
            state.signal.addEventListener('abort', info.close, { once: true });
            info.element.addEventListener('click', event => {
                const button = event.target.closest('[data-scan-open]'); if (!button) return;
                const id = M.safeId(button.dataset.scanOpen), entity = button.dataset.scanEntity; info.close(); state.signal.removeEventListener('abort', info.close);
                if (!id) return;
                if (entity === 'location') void c.selectLocation(id, true); else void selectItem(id, true);
            });
        }
        async function scanImageQR() {
            try {
                await loadQR(); let scan = null;
                const qr = window.AuraQRDialog.open({ host, windowId: state.windowId, t: c.ctx.t, signal: state.signal, initialTab: 'scan',
                    isCurrent: () => !state.disposed, onScan: result => { scan = result; } });
                await qr.closed;
                if (!scan) return;
                showScanTarget(findFindItLink(scan.text), scan.text);
            } catch (error) { if (error?.name !== 'AbortError') showNotice(noteText(error), true); }
        }
        async function printLabel(item, entity = 'item') {
            if (!item?.id || !['item', 'location'].includes(entity)) return;
            if (!state.status?.can_print) { showNotice(tr('readonly_notice'), true); return; }
            try {
                await loadQR(); let imageBlob = null;
                const url = deepLink(item.id, entity);
                const labelText = entity === 'location' ? item.name : (item.part_number || item.name);
                const handle = window.AuraQRDialog.open({ host, windowId: state.windowId, t: c.ctx.t, signal: state.signal, isCurrent: () => !state.disposed,
                    initialSpec: { schema_version: 1, content: { type: 'url', url }, frame: { position: 'bottom', text: labelText, font: 'sans', font_size: 18, text_color: '#182027', background_color: '#ffffff', border_color: '#d9dfe2', padding: 12 }, output: { format: 'png' } },
                    onPreview: result => { imageBlob = result.blob; } });
                await handle.closed;
                if (!imageBlob || state.disposed) return;
                printDialog(item, URL.createObjectURL(imageBlob), entity);
            } catch (error) { if (error?.name !== 'AbortError') showNotice(noteText(error), true); }
        }
        function printDialog(item, imageUrl, entity) {
            const node = document.createElement('dialog'); node.className = 'fi-dialog fi-print-dialog';
            node.innerHTML = `<form method="dialog" class="fi-dialog-form"><header class="fi-dialog-head"><div><p class="fi-kicker">${esc(tr('item_label'))}</p><h2>${esc(tr('print_label'))}</h2></div><button type="button" data-close aria-label="${esc(tr('close'))}">×</button></header><div class="fi-dialog-body"><div class="fi-print-preview" data-print-preview><img src="${esc(imageUrl)}" alt=""><div><b>${esc(item.name)}</b><small>${esc([item.manufacturer, item.part_number, item.package_family ? M.packageLabel(item, state.catalog, tr) : ''].filter(Boolean).join(' · '))}</small></div></div><fieldset class="fi-print-sizes"><legend>${esc(tr('paper_size'))}</legend><label><input type="radio" name="print-size" value="label" checked><span><b>50 × 30 mm</b><small>${esc(tr('single_label'))}</small></span></label><label><input type="radio" name="print-size" value="a4"><span><b>A4</b><small>${esc(tr('a4_sheet'))}</small></span></label></fieldset><p class="fi-dialog-copy">${esc(tr('a4_repeat_help'))}</p></div><footer class="fi-dialog-foot"><button type="button" data-close>${esc(tr('cancel'))}</button><button type="button" class="fi-primary" data-print>${icon('barcode', 15)}${esc(tr('print'))}</button></footer></form>`;
            host.append(node); node.showModal();
            const code = shortCode(item.id, entity), codeLine = document.createElement('small');
            codeLine.textContent = code; node.querySelector('[data-print-preview] > div').append(codeLine);
            let released = false;
            const release = () => { if (released) return; released = true; if (node.isConnected) { node.close(); node.remove(); } URL.revokeObjectURL(imageUrl); state.signal.removeEventListener('abort', release); };
            node.querySelectorAll('[data-close]').forEach(button => button.addEventListener('click', release, { once: true }));
            node.addEventListener('cancel', event => { event.preventDefault(); release(); }, { once: true });
            node.querySelector('[data-print]').addEventListener('click', async event => {
                const size = node.querySelector('input[name="print-size"]:checked')?.value || 'label';
                const popup = window.open('', '_blank', 'width=780,height=640');
                if (!popup) { showNotice(tr('print_popup_blocked'), true); return; }
                event.currentTarget.disabled = true;
                const image = esc(imageUrl), title = esc(item.name), sub = esc(code + ' · ' + (entity === 'location' ? (item.path || []).join(' / ') : [item.manufacturer, item.part_number, item.package_family ? M.packageLabel(item, state.catalog, tr) : ''].filter(Boolean).join(' · ')));
                const sheet = size === 'a4';
                const labels = sheet ? Array.from({ length: 27 }, () => `<article class="label"><img src="${image}" alt=""><b>${title}</b><small>${sub}</small></article>`).join('') : `<article class="label"><img src="${image}" alt=""><b>${title}</b><small>${sub}</small></article>`;
                popup.document.open(); popup.document.write(`<!doctype html><html><head><meta charset="utf-8"><title>${title}</title><style>@page{size:${sheet ? 'A4 portrait' : '50mm 30mm'};margin:${sheet ? '5mm' : '0'}}*{box-sizing:border-box}html,body{margin:0;padding:0;font:8pt/1.15 system-ui,sans-serif;color:#172027}.labels{display:${sheet ? 'grid' : 'block'};grid-template-columns:repeat(3,50mm);grid-auto-rows:30mm;gap:2mm;justify-content:center;align-content:start}.label{width:50mm;height:30mm;border:0.25mm solid #9ca5a7;border-radius:1mm;padding:1.5mm;display:grid;grid-template-columns:20mm 1fr;grid-template-rows:1fr auto;align-items:center;gap:1mm;overflow:hidden}.label img{grid-row:1/3;width:19mm;height:19mm;object-fit:contain}.label b{font-size:7pt;max-height:12mm;overflow:hidden}.label small{font-size:5.5pt;max-height:7mm;overflow:hidden;overflow-wrap:anywhere}@media print{body{print-color-adjust:exact;-webkit-print-color-adjust:exact}}</style></head><body><main class="labels">${labels}</main></body></html>`); popup.document.close();
                try {
                    await Promise.all([...popup.document.images].map(image => image.decode()));
                    if (!state.disposed && !popup.closed) { popup.focus(); popup.print(); }
                    release();
                } catch (_) { popup.close(); if (!state.disposed) showNotice(tr('error_generic'), true); release(); }
            });
            state.signal.addEventListener('abort', release, { once: true });
        }
        function dispose() { activeDialog.forEach(close); }
        return { csvWizard, exportCSV, deepLink, findFindItLink, scanQR, printLabel, dispose };
    }
    window.FindItWorkflows = { create };
})();
