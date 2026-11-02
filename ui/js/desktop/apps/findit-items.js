(function () {
    'use strict';

    const M = window.FindItModel, D = window.FindItDialogs;
    const COLUMN_CHOICES = [
        ['part', 'part'], ['manufacturer', 'manufacturer'], ['part_number', 'part_number'], ['category', 'category'], ['template', 'template'],
        ['location', 'location'], ['package', 'package'], ['on_hand', 'on_hand'], ['reserved', 'reserved'], ['available', 'available'], ['minimum', 'minimum']
    ];
    const COLUMN_STORAGE_KEY = 'aurago.desktop.findit.list-columns.v1';
    const DEFAULT_COLUMNS = ['part', 'location', 'package', 'available', 'minimum'];
    function create(c) {
        const { state, ctx, host, tr, canWrite, canPrint, dialogOptions, draw, apiQuery, apiCommand, isDefinitiveCommandRejection, withBusy, loadItems, selectItem, dispatchAction, showNotice, showError, noteText, categoryName, label, compactNumber, unitName, sprite, icon, selectedCategory } = c;
        const asList = M.asArray;
        const stale = () => state.disposed || state.signal.aborted;
        const validColumns = new Set(COLUMN_CHOICES.map(([key]) => key));
        const configuredColumns = (Array.isArray(state.listColumns) ? state.listColumns : DEFAULT_COLUMNS).filter(key => validColumns.has(key));
        state.listColumns = ['part', ...new Set(configuredColumns.filter(key => key !== 'part'))];
        function renderItems() {
            const categories = asList(state.catalog?.categories);
            const filtered = state.rows;
            const hasFilters = state.category || state.packageFamily || state.onlyAvailable || state.lowStock || state.filterTemplateID || Object.values(state.propertyFilters).some(value => value !== '' && value != null);
            return `<div class="fi-toolbar"><label class="fi-search">${icon('search', 17)}<input data-search type="search" value="${M.esc(state.queryText)}" placeholder="${M.esc(tr('search_placeholder'))}" aria-label="${M.esc(tr('search'))}" autocomplete="off"><kbd>⌘ K</kbd></label><button type="button" class="fi-tool-button ${state.filtersOpen || hasFilters ? 'is-selected' : ''}" data-action="toggle-filters">${icon('filter', 16)}<span>${M.esc(tr('filters'))}</span></button><button type="button" class="fi-tool-button" data-action="choose-columns" title="${M.esc(tr('choose_columns'))}">${icon('list', 16)}<span>${M.esc(tr('columns'))}</span></button><button type="button" class="fi-tool-button" data-action="save-search" ${state.queryText || hasFilters ? '' : 'disabled'} title="${M.esc(tr('save_current_search'))}">${icon('plus', 16)}<span>${M.esc(tr('save_search'))}</span></button><div class="fi-view-switch" role="group" aria-label="${M.esc(tr('view_mode'))}"><button type="button" data-mode="list" aria-pressed="${state.viewMode === 'list'}" aria-label="${M.esc(tr('list_view'))}">${icon('list', 16)}</button><button type="button" data-mode="tiles" aria-pressed="${state.viewMode === 'tiles'}" aria-label="${M.esc(tr('tile_view'))}">${icon('grid', 16)}</button></div>${canWrite() ? `<button type="button" class="fi-primary" data-action="new-item" ${state.pendingInitialStocks?.length ? 'disabled' : ''}>${icon('plus', 16)}${M.esc(tr('new_item'))}</button>` : ''}</div>
                ${state.filtersOpen ? renderFilterPanel(categories) : ''}
                ${state.savedSearches.length ? `<div class="fi-saved-searches"><span>${M.esc(tr('saved_searches'))}</span>${state.savedSearches.map((item, index) => `<button type="button" data-saved-search="${index}" title="${M.esc(item.name)}">${M.esc(item.name)}<span data-remove-saved="${index}" aria-label="${M.esc(tr('remove_saved_search'))}">×</span></button>`).join('')}</div>` : ''}
                <div class="fi-workspace ${state.mobilePanel === 'detail' ? 'fi-show-detail' : ''}">
                <aside class="fi-library-panel"><div class="fi-library-head"><div><span class="fi-eyebrow">${M.esc(tr('inventory'))}</span><strong>${M.esc(tr('browse'))}</strong></div><span class="fi-count-badge">${M.esc(String(state.total))}</span></div><div class="fi-library-section"><span class="fi-section-label">${M.esc(tr('categories'))}</span><button type="button" data-category="" class="fi-library-link ${!state.category ? 'is-active' : ''}"><span>${icon('box', 15)}${M.esc(tr('all_parts'))}</span><span>${M.esc(String(state.total))}</span></button>${categories.map(cat => { const id = typeof cat === 'object' ? cat.id : cat; return `<button type="button" data-category="${M.esc(id)}" class="fi-library-link ${state.category === id ? 'is-active' : ''}"><span>${M.categoryIcon(id, state.manifest)}${M.esc(categoryName(ctx, cat))}</span><span>${M.esc(String(cat?.count ?? ''))}</span></button>`; }).join('')}</div><div class="fi-library-section"><div class="fi-section-label-row"><span class="fi-section-label">${M.esc(tr('locations'))}</span><button type="button" data-action="new-location" aria-label="${M.esc(tr('new_location'))}" title="${M.esc(tr('new_location'))}" ${canWrite() ? '' : 'disabled'}>${icon('plus', 14)}</button></div><button type="button" class="fi-library-link ${!state.locationId ? 'is-active' : ''}" data-location=""><span>${icon('pin', 15)}${M.esc(tr('all_locations'))}</span></button>${renderLocationTree()}</div><div class="fi-library-bottom">${canWrite() ? `<button type="button" class="fi-library-link" data-action="import-csv">${icon('import', 15)}${M.esc(tr('import_csv'))}</button>` : ''}<button type="button" class="fi-library-link" data-action="export-items">${icon('export', 15)}${M.esc(tr('export_items'))}</button></div></aside>
                    <section class="fi-results-panel" aria-label="${M.esc(tr('parts'))}"><header class="fi-results-head"><div><p class="fi-eyebrow">${M.esc(state.category ? categoryName(ctx, selectedCategory()) : tr('all_parts'))}</p><h1>${M.esc(tr('parts'))}</h1></div><div class="fi-results-tools"><span>${M.esc(`${state.total} ${tr('items_count')}`)}</span><button type="button" class="fi-select" data-action="package-filter"><span>${M.esc(state.packageFamily ? M.packageLabel({ package_family: state.packageFamily }, state.catalog, tr) : tr('all_packages'))}</span>${icon('down', 14)}</button></div></header>${renderFilterChips()}
                    ${state.busy && !filtered.length ? renderLoading() : filtered.length ? (state.viewMode === 'tiles' ? renderItemTiles(filtered) : renderItemList(filtered)) : renderEmptyItems()}
                    ${renderPagination()}</section>
                    <aside class="fi-detail-panel ${state.selected ? 'has-selection' : ''}" aria-label="${M.esc(tr('item_details'))}">${state.selected ? renderItemDetail(state.selected) : renderNoSelection()}</aside>
                </div>`;
        }
        function renderFilterPanel(categories) {
            const familyOptions = asList(state.catalog?.package_families).map(value => { const id = typeof value === 'object' ? value.id : value; return { value: id, label: tr('package_' + String(id).toLowerCase().replace(/[^a-z0-9]+/g, '_'), typeof value === 'object' ? value.name || id : String(id).toUpperCase()) }; });
            const templates = asList(state.catalog?.templates || state.templates);
            const template = templates.find(value => value.id === state.filterTemplateID);
            const propertyFields = asList(template?.fields).map(field => {
                const label = template.built_in ? tr('field_' + field.key, field.label || field.key) : field.label || field.key;
                const unit = field.unit ? ` <small>${M.esc(tr('unit_' + field.unit.toLowerCase().replace(/[^a-z0-9]+/g, '_'), field.unit))}</small>` : '';
                let control;
                if (field.type === 'boolean') control = `<select data-filter-property="${M.esc(field.key)}"><option value="">${M.esc(tr('any_value'))}</option><option value="true" ${state.propertyFilters[field.key] === true || state.propertyFilters[field.key] === 'true' ? 'selected' : ''}>${M.esc(tr('yes'))}</option><option value="false" ${state.propertyFilters[field.key] === false || state.propertyFilters[field.key] === 'false' ? 'selected' : ''}>${M.esc(tr('no'))}</option></select>`;
                else if (field.type === 'enum') control = `<select data-filter-property="${M.esc(field.key)}"><option value="">${M.esc(tr('any_value'))}</option>${asList(field.options).map(value => `<option value="${M.esc(value)}" ${String(state.propertyFilters[field.key] ?? '') === String(value) ? 'selected' : ''}>${M.esc(tr('enum_' + String(value).toLowerCase().replace(/[^a-z0-9]+/g, '_'), value))}</option>`).join('')}</select>`;
                else control = `<div class="fi-typed-filter-control">${['number', 'date'].includes(field.type) ? `<select aria-label="${M.esc(tr('comparison'))}" data-filter-operator="${M.esc(field.key)}">${[['gte','at_least'],['lte','at_most'],['eq','exactly']].map(([op,key]) => `<option value="${op}" ${(state.propertyFilterOperators[field.key] || 'gte') === op ? 'selected' : ''}>${M.esc(tr(key))}</option>`).join('')}</select>` : ''}<input type="${field.type === 'number' || field.type === 'date' ? field.type : 'search'}" ${field.type === 'number' ? 'step="any"' : ''} data-filter-property="${M.esc(field.key)}" value="${M.esc(state.propertyFilters[field.key] ?? '')}" aria-label="${M.esc(label)}"></div>`;
                return `<label class="fi-property-filter"><span>${M.esc(label)}${unit}</span>${control}</label>`;
            }).join('');
            return `<section class="fi-filter-panel" aria-label="${M.esc(tr('filters'))}"><label>${M.esc(tr('category'))}<select data-filter-category><option value="">${M.esc(tr('all_categories'))}</option>${categories.map(value => { const id = typeof value === 'object' ? value.id : value; return `<option value="${M.esc(id)}" ${state.category === id ? 'selected' : ''}>${M.esc(categoryName(ctx, value))}</option>`; }).join('')}</select></label><label>${M.esc(tr('package_family'))}<select data-filter-package><option value="">${M.esc(tr('all_packages'))}</option>${familyOptions.map(value => `<option value="${M.esc(value.value)}" ${state.packageFamily === value.value ? 'selected' : ''}>${M.esc(value.label)}</option>`).join('')}</select></label><label>${M.esc(tr('template'))}<select data-filter-template><option value="">${M.esc(tr('all_templates'))}</option>${templates.map(value => `<option value="${M.esc(value.id)}" ${state.filterTemplateID === value.id ? 'selected' : ''}>${M.esc(value.built_in ? tr('template_' + value.id, value.name) : value.name)}</option>`).join('')}</select></label>${propertyFields ? `<div class="fi-property-filters"><p>${M.esc(tr('technical_filters'))}</p>${propertyFields}</div>` : ''}<label class="fi-filter-check"><input type="checkbox" data-filter-available ${state.onlyAvailable ? 'checked' : ''}><span>${M.esc(tr('available_only'))}</span></label><label class="fi-filter-check"><input type="checkbox" data-filter-low-stock ${state.lowStock ? 'checked' : ''}><span>${M.esc(tr('low_stock_only'))}</span></label><button type="button" class="fi-text-button" data-action="clear-filters">${M.esc(tr('clear_filters'))}</button></section>`;
        }
        function renderFilterChips() {
            const chips = [];
            if (state.locationId) chips.push(`<button type="button" data-clear-location>${icon('pin', 13)}${M.esc(state.locations.find(item => item.id === state.locationId)?.name || tr('location'))} ×</button>`);
            if (state.packageFamily) chips.push(`<button type="button" data-clear-package>${M.esc(M.packageLabel({ package_family: state.packageFamily }, state.catalog, tr))} ×</button>`);
            if (state.filterTemplateID) chips.push(`<button type="button" data-clear-template>${M.esc(tr('template_' + state.filterTemplateID, state.filterTemplateID))} ×</button>`);
            for (const [key, value] of Object.entries(state.propertyFilters)) if (value !== '' && value != null) chips.push(`<button type="button" data-clear-property="${M.esc(key)}">${M.esc(key)}: ${M.esc(String(value))} ×</button>`);
            if (state.onlyAvailable) chips.push(`<button type="button" data-clear-available>${M.esc(tr('available_only'))} ×</button>`);
            if (state.lowStock) chips.push(`<button type="button" data-clear-low-stock>${M.esc(tr('low_stock_only'))} ×</button>`);
            return chips.length ? `<div class="fi-filter-chips">${chips.join('')}</div>` : '';
        }
        function renderLocationTree() {
            const items = asList(state.locations), children = new Map();
            for (const item of items) {
                const parentId = item.parent_id || '';
                if (!children.has(parentId)) children.set(parentId, []);
                children.get(parentId).push(item);
            }
            const renderChildren = (parentId, depth, visited) => asList(children.get(parentId)).map(item => {
                if (visited.has(item.id)) return '';
                const next = new Set(visited); next.add(item.id);
                const hasChildren = asList(children.get(item.id)).length > 0;
                return `<button type="button" data-location="${M.esc(item.id)}" class="fi-library-link fi-location-link ${state.locationId === item.id ? 'is-active' : ''}" style="--fi-indent:${Math.min(depth, 4)}" title="${M.esc(item.name)}"><span>${icon('pin', 14)}<span>${M.esc(item.name)}</span></span>${hasChildren ? `<i>${asList(children.get(item.id)).length}</i>` : ''}</button>${renderChildren(item.id, depth + 1, next)}`;
            }).join('');
            return items.length ? renderChildren('', 0, new Set()) : `<p class="fi-sidebar-empty">${M.esc(tr('no_locations'))}</p>`;
        }
        function itemLocationSummary(item) {
            const paths = [...new Set(asList(item.stock_locations).map(location => asList(location.path).map(String).filter(Boolean).join(' › ')).filter(Boolean))];
            const fallback = item.location_name || item.primary_location || item.availability?.location_name || '';
            return { label: paths.length > 1 ? `${paths[0]} +${paths.length - 1}` : paths[0] || fallback || '—', title: paths.join('\n') || fallback };
        }
        function belowMinimum(available, minimum) {
            return M.quantityCompare(minimum, '0') > 0 && M.quantityCompare(available, minimum) < 0;
        }
        function renderItemList(rows) {
            const columns = COLUMN_CHOICES.filter(([key]) => state.listColumns.includes(key));
            return `<div class="fi-list-scroll"><table class="fi-item-table"><thead><tr>${columns.map(([, labelKey]) => `<th>${M.esc(tr(labelKey))}</th>`).join('')}<th></th></tr></thead><tbody>${rows.map(item => {
                const available = item.availability?.available ?? item.available ?? item.stock_on_hand ?? '0';
                const onHand = item.availability?.on_hand ?? item.on_hand ?? item.stock_on_hand ?? '0';
                const reserved = item.availability?.reserved ?? item.reserved ?? '0';
                const minimum = item.minimum_stock ?? '0';
                const isLow = belowMinimum(available, minimum);
                const location = itemLocationSummary(item);
                const template = asList(state.catalog?.templates).find(value => value.id === item.template_id);
                const values = {
                    part: `<div class="fi-table-part">${sprite(item, true)}<div><strong>${M.esc(item.name)}</strong><span>${M.esc(item.manufacturer || '')}${item.part_number ? ` · ${M.esc(item.part_number)}` : ''}</span></div></div>`,
                    manufacturer: M.esc(item.manufacturer || '—'), part_number: M.esc(item.part_number || '—'), category: M.esc(categoryName(ctx, item.category)),
                    template: M.esc(template?.built_in ? tr('template_' + template.id, template.name) : template?.name || '—'),
                    location: `<span title="${M.esc(location.title)}">${M.esc(location.label)}</span>`, package: `<span class="fi-package-badge">${M.esc(M.packageLabel(item, state.catalog, tr) || '—')}</span>`,
                    on_hand: M.esc(compactNumber(onHand)) + ` <small>${M.esc(itemUnitLabel(item))}</small>`,
                    reserved: M.esc(compactNumber(reserved)) + ` <small>${M.esc(itemUnitLabel(item))}</small>`,
                    available: `<span class="fi-stock-value ${isLow ? 'is-low' : ''}">${M.esc(compactNumber(available))} <small>${M.esc(itemUnitLabel(item))}</small></span>`,
                    minimum: M.esc(compactNumber(minimum)) + ` <small>${M.esc(itemUnitLabel(item))}</small>`
                };
                return `<tr data-select-item="${M.esc(item.id)}" tabindex="0" aria-selected="${state.selectedId === item.id}">${columns.map(([key]) => `<td>${values[key]}</td>`).join('')}<td><button type="button" class="fi-row-more" data-item-menu="${M.esc(item.id)}" aria-label="${M.esc(tr('more_actions'))}">${icon('dots', 17)}</button></td></tr>`;
            }).join('')}</tbody></table></div>`;
        }
        async function chooseColumns() {
            const values = Object.fromEntries(COLUMN_CHOICES.map(([key]) => [key, state.listColumns.includes(key)]));
            const fields = COLUMN_CHOICES.map(([key, labelKey]) => ({ key, label: labelKey, type: 'checkbox', disabled: key === 'part', wide: true }));
            const result = await D.openForm(dialogOptions({ title: 'choose_columns', description: 'column_visibility_help', fields, values, submitLabel: 'save' }));
            if (!result) return;
            state.listColumns = ['part', ...COLUMN_CHOICES.map(([key]) => key).filter(key => key !== 'part' && result[key])];
            try { localStorage.setItem(COLUMN_STORAGE_KEY, JSON.stringify(state.listColumns)); } catch (_) { /* Column choices are optional preferences. */ }
            draw();
        }
        function renderItemTiles(rows) {
            return `<div class="fi-item-grid">${rows.map(item => {
                const available = item.availability?.available ?? item.available ?? item.stock_on_hand ?? '0';
                const isLow = belowMinimum(available, item.minimum_stock ?? '0');
                const location = itemLocationSummary(item);
                return `<button type="button" class="fi-item-card ${state.selectedId === item.id ? 'is-selected' : ''}" data-select-item="${M.esc(item.id)}" aria-pressed="${state.selectedId === item.id}"><span class="fi-card-image">${sprite(item)}</span><span class="fi-card-copy"><span class="fi-eyebrow">${M.esc(categoryName(ctx, item.category))}</span><strong>${M.esc(item.name)}</strong><small>${M.esc(item.manufacturer || item.part_number || ' ')}</small><small class="fi-card-location" title="${M.esc(location.title)}">${M.esc(location.label)}</small><span class="fi-card-bottom"><span class="fi-package-badge">${M.esc(M.packageLabel(item, state.catalog, tr) || '—')}</span><b class="${isLow ? 'is-low' : ''}">${M.esc(compactNumber(available))} ${M.esc(itemUnitLabel(item))}</b></span></span></button>`;
            }).join('')}</div>`;
        }
        function renderEmptyItems() {
            const query = state.queryText || state.category || state.locationId || state.packageFamily;
            return `<div class="fi-empty-state"><div>${icon(query ? 'search' : 'box', 28)}</div><h2>${M.esc(tr(query ? 'no_match_title' : 'empty_inventory_title'))}</h2><p>${M.esc(tr(query ? 'no_match_help' : 'empty_inventory_help'))}</p>${canWrite() ? `<button type="button" class="fi-primary" data-action="new-item" ${state.pendingInitialStocks?.length ? 'disabled' : ''}>${icon('plus', 16)}${M.esc(tr('new_item'))}</button>` : ''}</div>`;
        }
        function renderLoading() { return `<div class="fi-loading"><i></i>${M.esc(tr('loading_parts'))}</div>`; }
        function renderPagination() {
            if (state.total <= state.limit) return '';
            const current = Math.floor(state.offset / state.limit) + 1, pages = Math.ceil(state.total / state.limit);
            return `<nav class="fi-pagination" aria-label="${M.esc(tr('pagination'))}"><button type="button" data-page="prev" ${state.offset === 0 ? 'disabled' : ''}>${icon('back', 14)}${M.esc(tr('previous'))}</button><span>${M.esc(tr('page_of'))} ${M.esc(`${current} / ${pages}`)}</span><button type="button" data-page="next" ${current >= pages ? 'disabled' : ''}>${M.esc(tr('next'))}${icon('arrow', 14)}</button></nav>`;
        }
        function renderNoSelection() {
            return `<div class="fi-detail-empty"><div class="fi-detail-empty-icon">${icon('box', 26)}</div><h2>${M.esc(tr('select_part'))}</h2><p>${M.esc(tr('select_part_help'))}</p><kbd>↑ ↓ ${M.esc(tr('to_browse'))}</kbd></div>`;
        }
        function renderItemDetail(item) {
            const rows = asList(state.availability?.stocks || state.availability?.locations || item.stocks);
            const datasheets = asList(item.datasheet_urls || item.datasheets);
            const movements = asList(item.movements);
            const photosrc = item.image_id ? M.imageURL(item.image_id, false) : '';
            const template = asList(state.catalog?.templates).find(value => value.id === item.template_id);
            const templateName = template?.built_in ? tr('template_' + template.id, template.name) : template?.name;
            return `<div class="fi-detail-scroll"><header class="fi-detail-head"><button type="button" class="fi-mobile-back" data-action="back-list">${icon('back', 16)}${M.esc(tr('back_to_list'))}</button><div class="fi-detail-art">${photosrc ? `<img src="${M.esc(photosrc)}" alt="${M.esc(tr('item_photo'))}">` : sprite(item)}</div><div class="fi-detail-title"><p class="fi-eyebrow">${M.esc(categoryName(ctx, item.category))}</p><h2>${M.esc(item.name)}</h2><p>${M.esc([item.manufacturer, item.part_number].filter(Boolean).join(' · '))}</p></div><div class="fi-detail-actions"><button type="button" data-action="print-label" title="${M.esc(tr('print_label'))}" aria-label="${M.esc(tr('print_label'))}" ${canPrint() ? '' : 'disabled'}>${icon('barcode', 17)}</button><button type="button" data-action="scan-qr" title="${M.esc(tr('scan_qr'))}" aria-label="${M.esc(tr('scan_qr'))}">${icon('qr', 17)}</button><button type="button" data-action="item-menu" aria-label="${M.esc(tr('more_actions'))}">${icon('dots', 17)}</button></div></header>
                <div class="fi-detail-quickstats"><div><span>${M.esc(tr('on_hand'))}</span><strong>${M.esc(compactNumber(state.availability?.on_hand ?? state.availability?.stock_on_hand ?? item.availability?.on_hand ?? item.stock_on_hand ?? '0'))}<small>${M.esc(itemUnitLabel(item))}</small></strong></div><div><span>${M.esc(tr('reserved'))}</span><strong>${M.esc(compactNumber(state.availability?.reserved ?? item.availability?.reserved ?? '0'))}<small>${M.esc(itemUnitLabel(item))}</small></strong></div><div><span>${M.esc(tr('available'))}</span><strong>${M.esc(compactNumber(state.availability?.available ?? item.availability?.available ?? '0'))}<small>${M.esc(itemUnitLabel(item))}</small></strong></div></div>
                ${canWrite() ? `<div class="fi-detail-toolbar"><button type="button" class="fi-primary" data-action="stock-receive">${icon('plus', 15)}${M.esc(tr('receive_stock'))}</button><button type="button" class="fi-tool-button" data-action="stock-issue">${M.esc(tr('issue_stock'))}</button><button type="button" class="fi-tool-button" data-action="upload-photo">${icon('photo', 15)}${M.esc(tr('add_photo'))}</button><input type="file" accept="image/png,image/jpeg,image/webp,.png,.jpg,.jpeg,.webp" capture="environment" data-photo-input hidden></div>` : ''}
                <section class="fi-detail-section"><header><h3>${M.esc(tr('stock_by_location'))}</h3><span>${M.esc(String(rows.length))}</span></header>${rows.length ? `<div class="fi-stock-list">${rows.map(stock => `<div class="fi-stock-row"><span class="fi-stock-pin">${icon('pin', 15)}</span><div><strong>${M.esc(asList(stock.location_path).join(' › ') || stock.location_name || stock.location || state.locations.find(location => location.id === stock.location_id)?.name || tr('unknown_location'))}</strong><small>${M.esc([stock.lot ? `${tr('lot')}: ${stock.lot}` : '', stock.condition ? tr('condition_' + stock.condition, stock.condition) : '', stock.serial ? `${tr('serial_short')} ${stock.serial}` : ''].filter(Boolean).join(' · ') || tr('stock_unit'))}</small></div><b>${M.esc(compactNumber(stock.available ?? stock.quantity ?? '0'))}<small>${M.esc(itemUnitLabel(item))}</small></b>${canWrite() ? `<button type="button" data-stock-menu="${M.esc(stock.id)}" data-stock-location="${M.esc(stock.location_id || '')}" aria-label="${M.esc(tr('stock_actions'))}">${icon('dots', 16)}</button>` : ''}</div>`).join('')}</div>` : `<p class="fi-inline-empty">${M.esc(tr('no_stock_records'))}</p>`}</section>
                <section class="fi-detail-section"><header><h3>${M.esc(tr('part_details'))}</h3>${canWrite() ? `<button type="button" class="fi-text-button" data-action="edit-item">${icon('edit', 14)}${M.esc(tr('edit'))}</button>` : ''}</header><dl class="fi-detail-fields"><div><dt>${M.esc(tr('manufacturer'))}</dt><dd>${M.esc(item.manufacturer || '—')}</dd></div><div><dt>${M.esc(tr('part_number'))}</dt><dd>${M.esc(item.part_number || '—')}</dd></div><div><dt>${M.esc(tr('template'))}</dt><dd>${M.esc(templateName || '—')}</dd></div><div><dt>${M.esc(tr('package_housing'))}</dt><dd>${M.esc(M.packageLabel(item, state.catalog, tr) || '—')}</dd></div>${item.pitch_mm != null ? `<div><dt>${M.esc(tr('pitch'))}</dt><dd>${M.esc(String(item.pitch_mm))} mm</dd></div>` : ''}${item.width_mm != null ? `<div><dt>${M.esc(tr('dimensions'))}</dt><dd>${M.esc([item.length_mm, item.width_mm, item.height_mm].filter(value => value != null).join(' × '))} mm</dd></div>` : ''}${renderProperties(item, template)}</dl>${item.description ? `<p class="fi-description">${M.esc(item.description)}</p>` : ''}${asList(item.tags).length ? `<div class="fi-tags">${asList(item.tags).map(tag => `<span>${M.esc(tag)}</span>`).join('')}</div>` : ''}</section>
                <section class="fi-detail-section"><header><h3>${M.esc(tr('datasheets'))}</h3>${canWrite() ? `<button type="button" class="fi-text-button" data-action="edit-datasheets">${icon('plus', 14)}${M.esc(tr('add_datasheet'))}</button>` : ''}</header>${datasheets.length ? `<ul class="fi-datasheet-list">${datasheets.map(url => `<li><a href="${M.esc(safeURL(url))}" target="_blank" rel="noopener noreferrer">${icon('file', 14)}<span>${M.esc(url)}</span>${icon('arrow', 13)}</a></li>`).join('')}</ul>` : `<p class="fi-inline-empty">${M.esc(tr('no_datasheets'))}</p>`}</section>
                <section class="fi-detail-section"><header><h3>${M.esc(tr('movement_history'))}</h3><span>${M.esc(String(movements.length))}</span></header>${movements.length ? `<ol class="fi-movement-list">${movements.map(movement => renderMovement(item, movement)).join('')}</ol>` : `<p class="fi-inline-empty">${M.esc(tr('no_movements'))}</p>`}</section>
                <small class="fi-updated">${M.esc(tr('updated'))} ${M.esc(M.date(item.updated_at) || '—')}</small></div>`;
        }
        function renderMovement(item, movement) {
            const kindLabels = { receive: 'receive_stock', issue: 'issue_stock', transfer: 'transfer', adjust: 'adjust_stock', return: 'return_stock', consume: 'consume' };
            const path = (ids, fallback) => asList(ids).length ? asList(ids).join(' › ') : locationPath(fallback).join(' › ');
            const source = path(movement.source_location_path, movement.source_location_id), target = path(movement.target_location_path, movement.target_location_id);
            const route = [source ? `${tr('source_location')}: ${source}` : '', target ? `${tr('target_location')}: ${target}` : ''].filter(Boolean).join(' · ');
            const metadata = [movement.lot ? `${tr('lot')}: ${movement.lot}` : '', movement.serial ? `${tr('serial_short')}: ${movement.serial}` : '', movement.note || ''].filter(Boolean).join(' · ');
            return `<li data-movement-kind="${M.esc(movement.kind)}"><div><b>${M.esc(tr(kindLabels[movement.kind] || 'movement'))}</b><time>${M.esc(M.date(movement.created_at) || '—')}</time></div><strong>${M.esc(compactNumber(movement.quantity))} ${M.esc(itemUnitLabel(item))}</strong>${route ? `<small>${M.esc(route)}</small>` : ''}${metadata ? `<small>${M.esc(metadata)}</small>` : ''}</li>`;
        }
        function safeURL(value) {
            try { const url = new URL(String(value || '')); return ['http:', 'https:'].includes(url.protocol) ? url.href : '#'; }
            catch (_) { return '#'; }
        }
        function renderProperties(item, template) {
            const properties = item.properties || {};
            const fields = template ? asList(template.fields) : Object.keys(properties).map(key => ({ key, type: 'text' }));
            return fields.map(field => {
                const value = properties[field.key];
                if (value == null || value === '') return '';
                const display = typeof value === 'boolean' ? (value ? tr('yes') : tr('no')) : field.type === 'enum' ? tr('enum_' + String(value).toLowerCase().replace(/[^a-z0-9]+/g, '_'), String(value)) : String(value);
                const label = !template ? field.key : template.built_in ? tr('field_' + field.key, field.label || field.key) : field.label || field.key;
                return `<div><dt>${M.esc(label)}</dt><dd>${M.esc(display)}${field.unit ? ` ${M.esc(tr('unit_' + field.unit.toLowerCase().replace(/[^a-z0-9]+/g, '_'), field.unit))}` : ''}</dd></div>`;
            }).join('');
        }
        function fieldOptions(values, key, prefix) { return asList(values).map(value => { const id = typeof value === 'object' ? value.id : value; const fallback = typeof value === 'object' ? value.label || value.name || id : String(value); return { value: id, label: tr(prefix + String(id).toLowerCase().replace(/[^a-z0-9]+/g, '_'), fallback) }; }); }
        function categoryOptions() { return fieldOptions(state.catalog?.categories || [], 'id', 'category_'); }
        function packageOptions() { return fieldOptions(state.catalog?.package_families || [], 'id', 'package_'); }
        function unitOptions() { return fieldOptions(state.catalog?.units || [], 'id', 'unit_'); }
        function templateOptions(currentID = '') {
            return asList(state.catalog?.templates).filter(value => !value.archived || value.id === currentID).map(value => ({
                value: value.id, label: value.built_in ? tr('template_' + value.id, value.name) : value.name,
                ...(value.archived ? { disabled: true } : {})
            }));
        }
        function itemFields(item = {}, template = null, creating = false) {
            const fields = [
                { key: 'name', label: 'item_name', type: 'text', required: true, wide: true },
                { key: 'description', label: 'description', type: 'textarea', wide: true, rows: 2 },
                { key: 'category', label: 'category', type: 'select', required: true, options: categoryOptions() },
                { key: 'template_id', label: 'template', type: 'select', allowEmpty: true, emptyLabel: 'not_set', options: templateOptions(item.template_id || '') },
                { key: 'manufacturer', label: 'manufacturer', type: 'text' },
                { key: 'part_number', label: 'part_number', type: 'text' },
                { key: 'unit', label: 'stock_unit', type: 'select', required: true, options: unitOptions() },
                { key: 'minimum_stock', label: 'minimum_stock', type: 'text', inputmode: 'decimal' },
                { key: 'package_family', label: 'package_housing', type: 'select', allowEmpty: true, emptyLabel: 'not_set', options: packageOptions() },
                { key: 'package_size', label: 'package', type: 'text', advanced: true },
                { key: 'icon_id', label: 'icon', type: 'select', allowEmpty: true, emptyLabel: 'automatic_icon', advanced: true, options: [...(state.manifest?.icons?.keys?.() || [])].map(id => ({ value: id, label: tr('category_' + id, id.replaceAll('_', ' ')) })) },
                { key: 'pin_count', label: 'pin_count', type: 'number', min: 0, step: 1, advanced: true },
                { key: 'pitch_mm', label: 'field_pitch_mm', type: 'number', min: 0, step: 'any', advanced: true },
                { key: 'length_mm', label: 'field_length_mm', type: 'number', min: 0, step: 'any', advanced: true },
                { key: 'width_mm', label: 'field_width_mm', type: 'number', min: 0, step: 'any', advanced: true },
                { key: 'height_mm', label: 'field_height_mm', type: 'number', min: 0, step: 'any', advanced: true },
                { key: 'size_system', label: 'size_system', type: 'select', allowEmpty: true, emptyLabel: 'not_set', options: ['imperial', 'metric'], advanced: true },
                { key: 'tags', label: 'tags', type: 'textarea', valueType: 'csv', wide: true, rows: 2, advanced: true },
                { key: 'datasheet_urls', label: 'datasheets', type: 'textarea', valueType: 'lines', wide: true, rows: 2, advanced: true },
                { key: 'track_serials', label: 'track_serials', type: 'checkbox', wide: true }
            ];
            if (creating) {
                fields.push({ key: 'initial_quantity', label: 'initial_quantity', type: 'text', inputmode: 'decimal' });
                fields.push({ key: 'initial_location_id', label: 'initial_location', type: 'select', allowEmpty: true, emptyLabel: 'select_location', options: asList(state.locations).map(value => ({ value: value.id, label: value.name })) });
                fields.push({ key: 'initial_lot', label: 'lot', type: 'text' });
                fields.push({ key: 'initial_serial', label: 'serial_short', type: 'text' });
            }
            for (const field of asList(template?.fields)) {
                fields.push({ key: 'property:' + field.key, label: template?.built_in ? tr('field_' + field.key, field.label || field.key) : field.label || field.key, type: field.type === 'number' || field.type === 'date' ? field.type : field.type === 'boolean' || field.type === 'enum' ? 'select' : 'text', required: !!field.required, allowEmpty: !field.required, emptyLabel: 'not_set', unit: field.unit, options: field.type === 'boolean' ? [{ value: 'true', label: tr('yes') }, { value: 'false', label: tr('no') }] : asList(field.options).map(value => ({ value, label: tr('enum_' + String(value).toLowerCase().replace(/[^a-z0-9]+/g, '_'), value) })), min: field.min, max: field.max, wide: field.type === 'text', advanced: !field.required });
            }
            return fields;
        }
        function itemFormValues(item = {}) {
            const values = {
                name: item.name || '', description: item.description || '', category: item.category || '', template_id: item.template_id || '',
                manufacturer: item.manufacturer || '', part_number: item.part_number || '', unit: item.unit || asList(state.catalog?.units)[0]?.id || asList(state.catalog?.units)[0] || 'piece',
                minimum_stock: item.minimum_stock ?? '', package_family: item.package_family || '', pin_count: item.pin_count ?? '', pitch_mm: item.pitch_mm ?? '',
                length_mm: item.length_mm ?? '', width_mm: item.width_mm ?? '', height_mm: item.height_mm ?? '', package_size: item.package_size || '', size_system: item.size_system || '',
                tags: asList(item.tags).join(', '), datasheet_urls: asList(item.datasheet_urls).join('\n'), icon_id: item.icon_id || '', track_serials: !!item.track_serials
            };
            for (const [key, value] of Object.entries(item.properties || {})) values['property:' + key] = typeof value === 'boolean' ? String(value) : value;
            return values;
        }
        function itemData(values, template, current = {}) {
            const properties = {};
            const fieldTypes = new Map(asList(template?.fields).map(field => [field.key, field.type]));
            for (const [key, value] of Object.entries(values)) {
                if (!key.startsWith('property:') || value === '' || value == null) continue;
                const fieldKey = key.slice('property:'.length);
                properties[fieldKey] = fieldTypes.get(fieldKey) === 'boolean' ? value === true || value === 'true' : value;
            }
            const data = {
                name: values.name, description: values.description || '', category: values.category, template_id: values.template_id || '',
                manufacturer: values.manufacturer || '', part_number: values.part_number || '', unit: values.unit || 'piece', minimum_stock: values.minimum_stock || '0',
                package_family: values.package_family || '', pin_count: values.pin_count === '' ? null : values.pin_count,
                pitch_mm: values.pitch_mm === '' ? null : values.pitch_mm, length_mm: values.length_mm === '' ? null : values.length_mm,
                width_mm: values.width_mm === '' ? null : values.width_mm, height_mm: values.height_mm === '' ? null : values.height_mm,
                package_size: values.package_size ?? current.package_size ?? '', size_system: values.size_system || '', tags: values.tags || [], datasheet_urls: values.datasheet_urls || [],
                image_id: values.image_id ?? current.image_id ?? '', icon_id: values.icon_id || '', track_serials: !!values.track_serials,
                properties
            };
            return data;
        }
        async function editItem(creating = false, defaults = {}) {
            if (creating && state.pendingInitialStocks?.length) { showNotice(tr('initial_stock_pending'), true); return; }
            const current = creating ? {} : state.selected; if (!creating && !current) return;
            let template = asList(state.catalog?.templates).find(value => value.id === (current.template_id || defaults.template_id || ''));
            if (!creating && current.template_id && !template) {
                try {
                    const record = await apiQuery({ operation: 'get', entity: 'templates', id: current.template_id });
                    template = record?.template || record;
                    if (!template?.id || template.id !== current.template_id) throw new Error(tr('error_not_found'));
                    state.catalog.templates = [...asList(state.catalog.templates), template];
                    state.templates = [...asList(state.templates).filter(value => value.id !== template.id), template];
                } catch (error) { showError(error); return; }
            }
            const initialValues = { ...itemFormValues(current), ...defaults };
            if (Array.isArray(initialValues.tags)) initialValues.tags = initialValues.tags.join(', ');
            if (Array.isArray(initialValues.datasheet_urls)) initialValues.datasheet_urls = initialValues.datasheet_urls.join('\n');
            let autoCategory = creating && !initialValues.category;
            let suggestedCategory = initialValues.category || categoryOptions()[0]?.value || '';
            const result = await D.openForm(dialogOptions({
                kicker: creating ? 'quick_capture' : 'item_record', title: creating ? 'new_item' : 'edit_item',
                description: 'item_form_intro', fields: itemFields(current, template, creating), submitLabel: creating ? 'create_item' : 'save',
                secondarySubmitLabel: creating ? 'save_and_next' : undefined, values: initialValues,
                fieldsWatch: 'template_id', fieldsFor: templateId => itemFields(current, asList(state.catalog?.templates).find(value => value.id === templateId), creating),
                onFieldChange: name => { if (name === 'category') autoCategory = false; },
                valuesFor: (templateId, values) => {
                    if (!autoCategory || values.category !== suggestedCategory) return null;
                    const selectedTemplate = asList(state.catalog?.templates).find(value => value.id === templateId);
                    if (!selectedTemplate?.category) return null;
                    suggestedCategory = selectedTemplate.category;
                    return { category: suggestedCategory };
                },
                validate: values => {
                    if (values.package_size && !values.size_system) return { message: 'error_invalid', field: 'size_system' };
                    if (values.minimum_stock !== '' && (M.quantityUnits(values.minimum_stock) == null || M.quantityCompare(values.minimum_stock, '0') < 0)) return { message: 'invalid_quantity', field: 'minimum_stock' };
                    if (creating && values.initial_quantity !== '') {
                        if (M.quantityUnits(values.initial_quantity) == null || M.quantityCompare(values.initial_quantity, '0') <= 0) return { message: 'invalid_quantity', field: 'initial_quantity' };
                        if (values.unit === 'piece' && !M.wholeQuantity(values.initial_quantity)) return { message: 'whole_piece_quantity', field: 'initial_quantity' };
                        if (!values.initial_location_id) return { message: 'initial_location_required', field: 'initial_location_id' };
                    }
                    if (creating && values.track_serials) {
                        if (values.unit !== 'piece') return { message: 'serial_initial_stock_rules', field: 'unit' };
                        if (values.initial_quantity !== '' && M.quantityCompare(values.initial_quantity, '1') !== 0) return { message: 'serial_initial_stock_rules', field: 'initial_quantity' };
                        if (values.initial_quantity !== '' && !String(values.initial_serial || '').trim()) return { message: 'serial_initial_stock_rules', field: 'initial_serial' };
                    }
                    return null;
                }
            }));
            if (!result) return;
            const addAnother = creating && result.__action === 'next'; delete result.__action;
            if (result.minimum_stock && M.quantityUnits(result.minimum_stock) == null) return showNotice(tr('invalid_quantity'), true);
            if (creating && result.initial_quantity && M.quantityUnits(result.initial_quantity) == null) return showNotice(tr('invalid_quantity'), true);
            if (creating && result.initial_quantity && M.quantityCompare(result.initial_quantity, '0') > 0 && !result.initial_location_id) return showNotice(tr('initial_location_required'), true);
            if (creating && result.initial_quantity && result.unit === 'piece' && !M.wholeQuantity(result.initial_quantity)) return showNotice(tr('whole_piece_quantity'), true);
            result.minimum_stock = result.minimum_stock ? M.normalizeQuantity(result.minimum_stock) : '0';
            if (creating && result.initial_quantity) result.initial_quantity = M.normalizeQuantity(result.initial_quantity);
            if (creating && result.initial_quantity && M.quantityCompare(result.initial_quantity, '0') > 0 && !state.hasPendingCommandCapacity?.()) return showNotice(tr('pending_command_queue_full'), true);
            const data = itemData(result, asList(state.catalog?.templates).find(value => value.id === result.template_id), current);
            const nextDefaults = { template_id: result.template_id || '', category: result.category || '', unit: result.unit || 'piece', initial_location_id: result.initial_location_id || '' };
            let acknowledged = false, initialStockFailed = false;
            const savedSuccessfully = await withBusy(async () => {
                const saved = await apiCommand({ operation: creating ? 'create' : 'update', entity: 'items', id: creating ? undefined : current.id, expected_version: current.version, data });
                acknowledged = true;
                const item = saved?.item || saved;
                const id = M.safeId(item?.id || saved?.id);
                if (creating && result.initial_quantity && M.quantityCompare(result.initial_quantity, '0') > 0 && result.initial_location_id && id) {
                    const stockData = {
                        kind: 'receive', item_id: id, location_id: result.initial_location_id, quantity: result.initial_quantity,
                        lot: result.initial_lot || '', serial: result.initial_serial || '', condition: 'new', note: ''
                    };
                    const pending = { item_id: id, item_name: item.name || data.name, item_version: item.version, unit: item.unit || data.unit,
                        track_serials: !!item.track_serials, stock_data: stockData, request_id: crypto.randomUUID() };
                    state.pendingInitialStock = pending;
                    if (!state.persistPendingInitialStock?.(pending)) {
                        initialStockFailed = true;
                        state.persistPendingInitialStock?.(null, { item_id: id, request_id: pending.request_id });
                        state.selectedId = id; state.notice = '';
                        await loadItems(); await selectItem(id);
                        state.error = state.pendingCommandCount?.() >= 100 ? tr('pending_command_queue_full') : tr('pending_command_storage_unavailable');
                        return;
                    }
                    try {
                        await apiCommand({ operation: 'stock', entity: 'items', id, expected_version: item.version, request_id: pending.request_id, data: stockData }, { initialStock: true, label: pending.item_name });
                        state.persistPendingInitialStock?.(null, { item_id: id, request_id: pending.request_id });
                    } catch (error) {
                        if (isDefinitiveCommandRejection(error)) {
                            initialStockFailed = true;
                            state.persistPendingInitialStock?.(null, { item_id: id, request_id: pending.request_id });
                            state.selectedId = id; state.notice = '';
                            await loadItems(); await selectItem(id);
                            state.error = tr('initial_stock_rejected');
                            return;
                        }
                        state.selectedId = id; state.notice = '';
                        await loadItems(); await selectItem(id);
                        state.error = tr('initial_stock_pending');
                        return;
                    }
                }
                state.notice = creating ? tr('item_created') : tr('item_saved');
                state.selectedId = id; await loadItems();
                if (id) await selectItem(id);
            });
            if (!savedSuccessfully && !acknowledged && !state.pendingInitialStock && !stale()) { await editItem(creating, result); return; }
            if (savedSuccessfully && addAnother && !state.pendingInitialStock && !initialStockFailed) await editItem(true, nextDefaults);
        }
        async function editDatasheets(defaults = null) {
            if (!state.selected) return;
            const result = await D.openForm(dialogOptions({ title: 'datasheets', fields: [{ key: 'urls', label: 'datasheets', type: 'textarea', valueType: 'lines', rows: 7, wide: true }], values: { urls: defaults ?? asList(state.selected.datasheet_urls).join('\n') }, submitLabel: 'save' }));
            if (!result) return;
            const item = state.selected;
            let acknowledged = false;
            const saved = await withBusy(async () => {
                const data = itemUpdateData(item, { datasheet_urls: result.urls });
                const updated = await apiCommand({ operation: 'update', entity: 'items', id: item.id, expected_version: item.version, data });
                acknowledged = true;
                state.selected = updated?.item || updated; state.selectedId = state.selected.id;
                state.notice = tr('item_saved'); await loadItems(); await selectItem(state.selectedId);
            });
            if (!saved && !acknowledged && !stale()) await editDatasheets(result.urls.join('\n'));
        }
        async function updateItem(patch) {
            const item = state.selected; if (!item || !canWrite()) return;
            await withBusy(async () => {
                const data = itemUpdateData(item, patch);
                const saved = await apiCommand({ operation: 'update', entity: 'items', id: item.id, expected_version: item.version, data });
                state.selected = saved?.item || saved; state.selectedId = state.selected.id;
                state.notice = tr('item_saved'); await loadItems(); await selectItem(state.selectedId);
            });
        }
        async function archiveItem() {
            const item = state.selected; if (!item || !canWrite()) return;
            if (!await D.openConfirm(dialogOptions({ title: 'archive', confirmLabel: 'archive' }))) return;
            await withBusy(async () => {
                await apiCommand({ operation: 'archive', entity: 'items', id: item.id, expected_version: item.version, data: {} });
                state.selected = null; state.selectedId = ''; state.availability = null; state.notice = tr('item_archived'); await loadItems();
            });
        }
        async function uploadPhoto(file) {
            const item = state.selected; if (!item || !file || !canWrite()) return;
            if (!/^image\/(png|jpeg|webp)$/.test(file.type) || file.size > 10 * 1024 * 1024) return showNotice(tr('photo_invalid'), true);
            await withBusy(async () => {
                const result = await M.uploadImage(file, state.signal);
                const saved = await apiCommand({ operation: 'update', entity: 'items', id: item.id, expected_version: item.version, data: itemUpdateData(item, { image_id: result.id }) });
                state.selected = saved?.item || saved;
                state.notice = tr('photo_saved'); await loadItems(); await selectItem(item.id);
            });
        }
        function itemUpdateData(item, patch = {}) {
            return {
                name: item.name || '', description: item.description || '', category: item.category || 'other', template_id: item.template_id || '',
                manufacturer: item.manufacturer || '', part_number: item.part_number || '', unit: item.unit || 'piece', minimum_stock: item.minimum_stock || '0',
                package_family: item.package_family || '', pin_count: Number(item.pin_count) || 0, pitch_mm: item.pitch_mm ?? null,
                length_mm: item.length_mm ?? null, width_mm: item.width_mm ?? null, height_mm: item.height_mm ?? null,
                package_size: item.package_size || '', size_system: item.size_system || '', tags: asList(item.tags), datasheet_urls: asList(item.datasheet_urls),
                image_id: item.image_id || '', icon_id: item.icon_id || '', track_serials: !!item.track_serials, properties: item.properties || {}, ...patch
            };
        }
        function locationPath(id) {
            const byId = new Map(asList(state.locations).map(location => [location.id, location]));
            const names = [], seen = new Set(); let current = byId.get(id);
            while (current && !seen.has(current.id) && names.length < 64) { seen.add(current.id); names.push(current.name); current = byId.get(current.parent_id); }
            return names.reverse();
        }
        function stockChoices(kind, item) {
            const rows = new Map(asList(state.availability?.stocks).map(stock => [stock.id, { ...stock }]));
            if (kind === 'return' || kind === 'adjust') {
                const seen = new Set();
                for (const movement of asList(state.selected?.movements)) {
                    if (!movement.stock_id || seen.has(movement.stock_id)) continue;
                    seen.add(movement.stock_id);
                    if (rows.has(movement.stock_id) || M.quantityCompare(movement.quantity, '0') >= 0 || kind === 'return' && movement.kind !== 'issue') continue;
                    rows.set(movement.stock_id, { id: movement.stock_id, item_id: item.id, location_id: movement.source_location_id || movement.target_location_id || '', quantity: '0', available: '0', reserved: '0', lot: movement.lot || '', condition: movement.condition || 'unknown', serial: movement.serial || '', version: 0 });
                }
            }
            return [...rows.values()].filter(stock => {
                if (kind === 'issue' || kind === 'transfer') return M.quantityCompare(stock.available ?? stock.quantity ?? '0', '0') > 0;
                if (kind === 'return' && item.track_serials) return M.quantityCompare(stock.quantity ?? '0', '0') === 0;
                return true;
            }).sort((a, b) => locationPath(a.location_id).join(' / ').localeCompare(locationPath(b.location_id).join(' / ')) || String(a.lot || '').localeCompare(String(b.lot || '')) || String(a.serial || '').localeCompare(String(b.serial || '')));
        }
        function stockChoiceLabel(stock, item = state.selected) {
            const location = locationPath(stock.location_id).join(' › ') || stock.location || tr('unknown_location');
            const attributes = [stock.lot ? `${tr('lot')}: ${stock.lot}` : '', tr('condition_' + String(stock.condition || 'unknown'), stock.condition || tr('not_set')), stock.serial ? `${tr('serial_short')}: ${stock.serial}` : ''].filter(Boolean);
            attributes.push(`${tr('available')}: ${M.fmtQuantity(stock.available ?? stock.quantity ?? '0')} ${itemUnitLabel(item)}`);
            return `${location} · ${attributes.join(' · ')}`;
        }
        function itemUnitLabel(item) { return label(ctx, unitName(state.catalog, item?.unit) || item?.unit || 'piece'); }
        async function stockMove(kind, stock = null, retryValues = {}, retryExact = false) {
            const item = state.selected; if (!item || !canWrite()) return;
            const commandScope = { operation: 'stock', entity: 'items', id: item.id, data: { item_id: item.id } };
            if (!retryExact && state.hasPendingCommandScope?.(commandScope)) return showNotice(tr('pending_command_intro'), true);
            const selection = state.selectionSerial, availability = state.availability;
            const isCurrentSelection = () => !stale() && state.selectionSerial === selection && state.selectedId === item.id;
            const stockRequired = ['issue', 'transfer', 'adjust', 'return'].includes(kind);
            const choices = stockRequired ? stockChoices(kind, item) : [];
            if (stockRequired && !choices.length) return showNotice(tr('no_stock_records'), true);
            const preset = stock && choices.find(value => value.id === stock.id) || choices[0];
            const locations = asList(state.locations);
            const locationOptions = locations.map(value => ({ value: value.id, label: locationPath(value.id).join(' › ') || value.name }));
            const conditionOptions = ['new', 'used', 'refurbished', 'damaged', 'unknown'].map(value => ({ value, label: tr('condition_' + value, value) }));
            const fieldsFor = stockId => {
                const selected = choices.find(value => value.id === stockId) || preset;
                return [
                    ...(stockRequired ? [{ key: 'stock_id', label: 'stock_record', type: 'select', required: true, disabled: retryExact, options: choices.map(value => ({ value: value.id, label: stockChoiceLabel(value, item) })) }] : []),
                    ...(kind === 'receive' ? [{ key: 'location_id', label: 'location', type: 'select', required: true, disabled: retryExact, options: locationOptions }] : []),
                    ...(kind === 'transfer' ? [{ key: 'target_location_id', label: 'target_location', type: 'select', required: true, disabled: retryExact, options: locationOptions.filter(value => value.value !== selected?.location_id) }] : []),
                    { key: 'quantity', label: 'quantity', type: 'text', required: true, inputmode: 'decimal', disabled: retryExact },
                    ...(kind === 'receive' ? [
                        { key: 'lot', label: 'lot', type: 'text', disabled: retryExact },
                        ...(item.track_serials ? [{ key: 'serial', label: 'serial_short', type: 'text', required: true, disabled: retryExact }] : []),
                        { key: 'condition', label: 'condition', type: 'select', disabled: retryExact, options: conditionOptions }
                    ] : []),
                    { key: 'note', label: 'description', type: 'text', wide: true, disabled: retryExact }
                ];
            };
            const initialStock = preset;
            const values = { stock_id: retryValues.stock_id || initialStock?.id || '', location_id: retryValues.location_id || initialStock?.location_id || locations[0]?.id || '', target_location_id: retryValues.target_location_id || locations.find(value => value.id !== initialStock?.location_id)?.id || '', quantity: retryValues.quantity || (kind === 'adjust' ? initialStock?.quantity || '0' : item.track_serials && kind === 'return' ? '1' : ''), lot: retryValues.lot || '', serial: retryValues.serial || '', condition: retryValues.condition || 'new', note: retryValues.note || '' };
            const fields = fieldsFor(values.stock_id);
            const titleKeys = { receive: 'receive_stock', issue: 'issue_stock', transfer: 'transfer', adjust: 'adjust_stock', return: 'return_stock' };
            const result = await D.openForm(dialogOptions({ title: titleKeys[kind] || 'stock_record', description: retryExact ? 'pending_command_intro' : 'stock_form_help', fields, fieldsWatch: kind === 'transfer' ? 'stock_id' : undefined, fieldsFor: kind === 'transfer' ? fieldsFor : undefined,
                values, submitLabel: retryExact ? 'retry_exact_command' : 'save_movement', validate: formValues => {
                    const compare = M.quantityCompare(formValues.quantity, '0');
                    if (M.quantityUnits(formValues.quantity) == null || (kind === 'adjust' ? compare < 0 : compare <= 0)) return { message: kind === 'adjust' ? 'invalid_adjustment' : 'invalid_quantity', field: 'quantity' };
                    if (item.unit === 'piece' && !M.wholeQuantity(formValues.quantity)) return { message: 'whole_piece_quantity', field: 'quantity' };
                    const chosen = stockRequired ? choices.find(value => value.id === formValues.stock_id) : null;
                    if (stockRequired && !chosen) return { message: 'no_stock_records', field: 'stock_id' };
                    if ((kind === 'issue' || kind === 'transfer') && M.quantityCompare(formValues.quantity, chosen.available ?? chosen.quantity ?? '0') > 0) return { message: 'error_insufficient_stock', field: 'quantity' };
                    if (kind === 'adjust' && M.quantityCompare(formValues.quantity, chosen.reserved ?? '0') < 0) return { message: 'release_holds_first', field: 'quantity' };
                    return null;
                } }));
            if (!result) return;
            if (!retryExact && !isCurrentSelection()) { showNotice(tr('error_conflict'), true); return; }
            if (M.quantityUnits(result.quantity) == null || (kind === 'adjust' ? M.quantityCompare(result.quantity, '0') < 0 : M.quantityCompare(result.quantity, '0') <= 0)) return showNotice(tr(kind === 'adjust' ? 'invalid_adjustment' : 'invalid_quantity'), true);
            if (item.unit === 'piece' && !M.wholeQuantity(result.quantity)) return showNotice(tr('whole_piece_quantity'), true);
            const amount = M.normalizeQuantity(result.quantity), selected = stockRequired ? choices.find(value => value.id === result.stock_id) : null;
            if (stockRequired && !selected) return showNotice(tr('no_stock_records'), true);
            if ((kind === 'issue' || kind === 'transfer') && M.quantityCompare(amount, selected.available ?? selected.quantity ?? '0') > 0) return showNotice(tr('error_insufficient_stock'), true);
            if (kind === 'adjust' && M.quantityCompare(amount, selected.reserved ?? '0') < 0) return showNotice(tr('release_holds_first'), true);
            const locationId = selected?.location_id || result.location_id || '';
            const beforeTotal = availability?.on_hand ?? '0'; let afterTotal = beforeTotal;
            if (kind === 'receive' || kind === 'return') afterTotal = M.quantityAdd(beforeTotal, amount) ?? beforeTotal;
            else if (kind === 'issue') afterTotal = M.quantityAdd(beforeTotal, '-' + amount) ?? beforeTotal;
            else if (kind === 'adjust') afterTotal = M.quantityAdd(M.quantityAdd(beforeTotal, '-' + (selected.quantity || '0')) || beforeTotal, amount) || beforeTotal;
            let beforeRow = selected?.quantity || '0', afterRow = beforeRow;
            if (kind === 'receive') { beforeRow = asList(availability?.stocks).filter(value => value.location_id === locationId).reduce((sum, value) => M.quantityAdd(sum, value.quantity || '0') || sum, '0'); afterRow = M.quantityAdd(beforeRow, amount) || beforeRow; }
            else if (kind === 'issue') afterRow = M.quantityAdd(beforeRow, '-' + amount) || beforeRow;
            else if (kind === 'return') afterRow = M.quantityAdd(beforeRow, amount) || beforeRow;
            else if (kind === 'adjust') afterRow = amount;
            const targetId = result.target_location_id || '';
            let impact = `<div class="fi-stock-impact"><div><span>${M.esc(tr('on_hand'))}</span><strong>${M.esc(M.fmtQuantity(beforeTotal))} → ${M.esc(M.fmtQuantity(afterTotal))} ${M.esc(itemUnitLabel(item))}</strong></div>`;
            if (kind === 'transfer') {
                const targetBefore = asList(availability?.stocks).filter(value => value.location_id === targetId).reduce((sum, value) => M.quantityAdd(sum, value.quantity || '0') || sum, '0');
                const targetAfter = M.quantityAdd(targetBefore, amount) || targetBefore;
                impact += `<div><span>${M.esc(tr('source_location'))}</span><strong>${M.esc(M.fmtQuantity(beforeRow))} → ${M.esc(M.fmtQuantity(M.quantityAdd(beforeRow, '-' + amount) || beforeRow))} ${M.esc(itemUnitLabel(item))}</strong><small>${M.esc(locationPath(locationId).join(' › '))}</small></div><div><span>${M.esc(tr('target_location'))}</span><strong>${M.esc(M.fmtQuantity(targetBefore))} → ${M.esc(M.fmtQuantity(targetAfter))} ${M.esc(itemUnitLabel(item))}</strong><small>${M.esc(locationPath(targetId).join(' › '))}</small></div>`;
            } else impact += `<div><span>${M.esc(kind === 'receive' ? tr('location') : tr('stock_record'))}</span><strong>${M.esc(M.fmtQuantity(beforeRow))} → ${M.esc(M.fmtQuantity(afterRow))} ${M.esc(itemUnitLabel(item))}</strong><small>${M.esc(locationPath(locationId).join(' › '))}</small></div>`;
            impact += '</div>';
            const confirmed = await D.openConfirm(dialogOptions({ title: 'confirm_stock_change', message: 'stock_impact_help', contentHTML: impact, confirmLabel: 'save_movement' }));
            if (!confirmed) return;
            if (!retryExact && !isCurrentSelection()) { showNotice(tr('error_conflict'), true); return; }
            let acknowledged = false;
            let commandError = null;
            const stockData = {
                kind, item_id: item.id, stock_id: selected?.id || '', location_id: locationId, target_location_id: targetId,
                quantity: amount, lot: selected?.lot || result.lot || '', serial: selected?.serial || result.serial || '', condition: selected?.condition || result.condition || 'new', note: result.note || '',
                ...(selected?.version != null ? { stock_version: selected.version } : {})
            };
            const completed = await withBusy(async () => {
                try { await apiCommand({ operation: 'stock', entity: 'items', id: item.id, expected_version: item.version, data: stockData }, { replayOnly: retryExact, label: item.name }); }
                catch (error) { commandError = error; throw error; }
                acknowledged = true;
                state.notice = tr('movement_saved'); await loadItems();
                if (state.selectionSerial === selection) await selectItem(item.id);
            });
            if (completed || acknowledged) return;
            if (commandError?.code === 'conflict' && isCurrentSelection()) {
                await selectItem(item.id);
                showNotice(noteText(ctx, commandError), true);
                return;
            }
            if (!retryExact && isCurrentSelection() && state.hasPendingCommandIntent?.({ operation: 'stock', entity: 'items', id: item.id, expected_version: item.version, data: stockData })) {
                await stockMove(kind, selected, result, true);
                return;
            }
            if (!retryExact && isCurrentSelection() && !state.hasPendingCommandScope?.(commandScope)) await stockMove(kind, selected, result);
        }
        async function showItemActions(id) {
            if (id !== state.selectedId) await selectItem(id);
            if (!state.selected) return;
            const menu = D.openInfo(dialogOptions({ title: 'more_actions', html: `<div class="fi-action-list">${[
                ['edit-item', 'edit_item', 'edit'], ['stock-receive', 'receive_stock', 'plus'], ['stock-issue', 'issue_stock', 'export'], ['print-label', 'print_label', 'barcode'], ['archive-item', 'archive', 'close']
            ].filter(([action]) => action === 'print-label' ? canPrint() : canWrite()).map(([action, key, symbol]) => `<button type="button" data-menu-action="${action}">${icon(symbol, 16)}${M.esc(tr(key))}</button>`).join('')}</div>` }));
            menu.element.addEventListener('click', event => {
                const button = event.target.closest('[data-menu-action]'); if (!button) return;
                event.stopPropagation(); const action = button.dataset.menuAction; menu.close(); void dispatchAction(action).catch(error => showError(error));
            });
        }
        async function showStockActions(stockId, locationId) {
            const stocks = asList(state.availability?.stocks || state.availability?.locations || state.selected?.stocks);
            const stock = stocks.find(item => item.id === stockId);
            if (!stock) return;
            const menu = D.openInfo(dialogOptions({ title: 'stock_actions', html: `<div class="fi-action-list"><button type="button" data-menu-action="stock-transfer">${icon('pin', 16)}${M.esc(tr('transfer'))}</button><button type="button" data-menu-action="stock-adjust">${icon('edit', 16)}${M.esc(tr('adjust_stock'))}</button><button type="button" data-menu-action="stock-return">${icon('refresh', 16)}${M.esc(tr('return_stock'))}</button></div>` }));
            menu.element.addEventListener('click', event => {
                const button = event.target.closest('[data-menu-action]'); if (!button) return;
                event.stopPropagation(); const action = button.dataset.menuAction.replace('stock-', ''); menu.close(); void stockMove(action, { ...stock, location_id: locationId }).catch(error => showError(error));
            });
        }

        return { renderItems, renderItemDetail, editItem, editDatasheets, updateItem, archiveItem, uploadPhoto, stockMove, showItemActions, showStockActions, chooseColumns };
    }
    window.FindItItems = { create };
})();
