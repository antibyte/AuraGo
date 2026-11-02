(function () {
    'use strict';

    const M = window.FindItModel;
    const D = window.FindItDialogs;
    const CSV = window.FindItCSV;
    const instances = new Map();
    const inFlightCommands = new Map();
    const SAVED_SEARCHES_KEY = 'aurago.desktop.findit.saved-searches.v1';
    const LIST_MODE_KEY = 'aurago.desktop.findit.list-mode.v1';
    const LIST_COLUMNS_KEY = 'aurago.desktop.findit.list-columns.v1';
    const PENDING_INITIAL_KEY = 'aurago.desktop.findit.pending-initial.v2';
    const PENDING_COMMANDS_KEY = 'aurago.desktop.findit.pending-commands.v1';
    const LEGACY_PENDING_INITIAL_PREFIX = 'aurago.desktop.findit.pending-initial.v1.';
    const MAX_PENDING_COMMANDS = 100;
    const DEFAULT_LIST_COLUMNS = ['part', 'location', 'package', 'available', 'minimum'];
    const NAME = 'FindIt';

    function t(ctx, key, fallback) {
        const id = 'findit.' + key;
        const value = ctx?.t ? ctx.t(id) : '';
        if (value && value !== id) return value;
        return fallback || key.replaceAll('_', ' ');
    }
    function icon(name, size = 18) {
        const paths = {
            search: '<circle cx="10.8" cy="10.8" r="6.5"/><path d="m16 16 4.5 4.5"/>',
            plus: '<path d="M12 5v14M5 12h14"/>',
            pin: '<path d="M20 10c0 5-8 12-8 12S4 15 4 10a8 8 0 1 1 16 0Z"/><circle cx="12" cy="10" r="2.5"/>',
            box: '<path d="m12 3 9 5-9 5-9-5 9-5Z"/><path d="m3 8 9 5 9-5M3 8v9l9 5 9-5V8M12 13v9"/>',
            list: '<path d="M9 6h11M9 12h11M9 18h11M4 6h.01M4 12h.01M4 18h.01"/>',
            grid: '<rect x="3" y="3" width="7" height="7" rx="1"/><rect x="14" y="3" width="7" height="7" rx="1"/><rect x="3" y="14" width="7" height="7" rx="1"/><rect x="14" y="14" width="7" height="7" rx="1"/>',
            qr: '<path d="M3 3h7v7H3zM14 3h7v7h-7zM3 14h7v7H3zM14 14h2v2h-2zM19 14h2v2h-2zM14 19h2v2h-2zM19 19h2v2h-2z"/>',
            camera: '<path d="M4 7h3l1.5-2h7L17 7h3v12H4z"/><circle cx="12" cy="13" r="3.5"/>',
            close: '<path d="m6 6 12 12M18 6 6 18"/>',
            back: '<path d="m15 18-6-6 6-6M9 12h12"/>',
            filter: '<path d="M4 6h16M7 12h10M10 18h4"/>',
            import: '<path d="M12 3v12m-5-5 5 5 5-5M4 20h16"/>',
            export: '<path d="M12 21V9m-5 5 5-5 5 5M4 4h16"/>',
            photo: '<rect x="3" y="4" width="18" height="16" rx="2"/><circle cx="8.5" cy="9" r="1.5"/><path d="m21 15-5-5L5 20"/>',
            down: '<path d="m6 9 6 6 6-6"/>',
            dots: '<circle cx="5" cy="12" r="1" fill="currentColor"/><circle cx="12" cy="12" r="1" fill="currentColor"/><circle cx="19" cy="12" r="1" fill="currentColor"/>',
            edit: '<path d="m4 16-.7 4.7L8 20l11-11-4-4L4 16Z"/><path d="m13.5 6.5 4 4"/>',
            refresh: '<path d="M20 7v5h-5M4 17v-5h5"/><path d="M5.5 9a7 7 0 0 1 11.8-2L20 12M4 12l2.7 5a7 7 0 0 0 11.8-2"/>',
            arrow: '<path d="M5 12h14m-6-6 6 6-6 6"/>',
            check: '<path d="m5 12 4 4L19 6"/>',
            file: '<path d="M6 3h8l5 5v13H6z"/><path d="M14 3v6h5M9 14h7M9 17h7"/>',
            settings: '<circle cx="12" cy="12" r="3"/><path d="m19.4 15 .1.1 1.1 1.8-1.8 3.1-2-.3-.2.1-1.5.9-.6 1.8h-3.6l-.6-1.8-1.5-.9-.2-.1-2 .3-1.8-3.1 1.1-1.8V9l-1.1-1.8 1.8-3.1 2 .3.2-.1 1.5-.9.6-1.8h3.6l.6 1.8 1.5.9.2.1 2-.3 1.8 3.1L19.4 9z"/>',
            barcode: '<path d="M4 5v14M7 5v14M10 5v14M14 5v14M16 5v14M20 5v14"/>'
        };
        return `<svg width="${size}" height="${size}" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">${paths[name] || paths.box}</svg>`;
    }
    const asList = value => M.asArray(value);
    const label = (ctx, value, fallback) => {
        const id = typeof value === 'object' ? String(value?.id || value?.value || '') : String(value || '');
        if (!id) return t(ctx, fallback || 'not_set');
        if (typeof value === 'object' && value.symbol) return t(ctx, 'unit_' + id, value.name || id);
        if (typeof value === 'object' && value.name) return t(ctx, 'category_' + id, value.name);
        return t(ctx, 'category_' + id, id.replaceAll('_', ' '));
    };
    const statusClass = status => ({ draft: 'is-draft', released: 'is-released', completed: 'is-completed', cancelled: 'is-cancelled' }[status] || '');
    const safeLocalStorage = (key, fallback) => { try { return JSON.parse(localStorage.getItem(key) || 'null') ?? fallback; } catch (_) { return fallback; } };
    const storeLocal = (key, value) => { try { localStorage.setItem(key, JSON.stringify(value)); } catch (_) { /* Preferences are optional. */ } };
    const pendingInitialReceiptKey = value => value?.item_id && value?.request_id ? `${value.item_id}:${value.request_id}` : '';
    const freezeValue = value => {
        if (!value || typeof value !== 'object' || Object.isFrozen(value)) return value;
        Object.values(value).forEach(freezeValue);
        return Object.freeze(value);
    };
    const cloneFrozen = value => freezeValue(JSON.parse(JSON.stringify(value)));
    function pendingCommandScope(payload) {
        if (payload?.entity === 'items' && payload.operation === 'stock') {
            const itemId = M.safeId(payload.data?.item_id || payload.id);
            return itemId ? `stock:${itemId}` : '';
        }
        if (payload?.entity === 'projects' && ['reserve', 'release', 'consume'].includes(payload.operation)) {
            const projectId = M.safeId(payload.data?.project_id || payload.id);
            return projectId ? `reservations:${projectId}` : '';
        }
        return '';
    }
    function isDefinitiveCommandRejection(error) {
        const status = Number(error?.status), code = String(error?.code || '');
        return status === 404 && code === 'not_found' || status === 409 && code === 'conflict' || status === 422 && code === 'invalid';
    }
    function readPendingCommands() {
        const records = new Map();
        try {
            const rows = JSON.parse(sessionStorage.getItem(PENDING_COMMANDS_KEY) || '[]');
            for (const row of Array.isArray(rows) ? rows : []) {
                const requestId = typeof row?.request_id === 'string' ? row.request_id : '';
                const payload = row?.payload, scope = pendingCommandScope(payload);
                if (!requestId || payload?.request_id !== requestId || !scope || row.scope !== scope) continue;
                records.set(requestId, { request_id: requestId, payload: cloneFrozen(payload), scope, label: String(row.label || '').slice(0, 256) });
            }
        } catch (_) { /* Pending mutation recovery is unavailable when session storage cannot be read. */ }
        return [...records.values()];
    }
    function readPendingInitialStocks() {
        const records = new Map();
        const add = value => {
            const rows = Array.isArray(value) ? value : value && typeof value === 'object' ? [value] : [];
            for (const row of rows) {
                const itemId = M.safeId(row?.item_id), requestId = typeof row?.request_id === 'string' ? row.request_id : '';
                if (!itemId || !requestId || !row.stock_data || typeof row.stock_data !== 'object') continue;
                const record = { ...row, item_id: itemId, request_id: requestId };
                records.set(pendingInitialReceiptKey(record), record);
            }
        };
        try {
            add(JSON.parse(sessionStorage.getItem(PENDING_INITIAL_KEY) || '[]'));
            const legacyKeys = [];
            for (let index = 0; index < sessionStorage.length; index++) {
                const key = sessionStorage.key(index);
                if (!key?.startsWith(LEGACY_PENDING_INITIAL_PREFIX)) continue;
                legacyKeys.push(key);
                try { add(JSON.parse(sessionStorage.getItem(key) || 'null')); } catch (_) { /* Ignore malformed legacy receipts. */ }
            }
            if (legacyKeys.length) {
                sessionStorage.setItem(PENDING_INITIAL_KEY, JSON.stringify([...records.values()]));
                legacyKeys.forEach(key => sessionStorage.removeItem(key));
            }
        } catch (_) { /* Opening-stock recovery is best-effort when session storage is unavailable. */ }
        return [...records.values()];
    }
    const compactNumber = value => M.fmtQuantity(value ?? '0');
    const noteText = (ctx, error) => {
        const code = String(error?.code || '');
        const reason = String(error?.reason || code);
        const reasonLabels = {
            insufficient_available: 'error_insufficient_stock', reservation_exceeds_need: 'error_reservation_exceeds_need',
            invalid_date: 'error_invalid_date', invalid_quantity: 'invalid_quantity'
        };
        if (reasonLabels[reason]) {
            let message = t(ctx, reasonLabels[reason]);
            const details = error?.details || {};
            if (reason === 'insufficient_available' && details.available != null && details.requested != null) message += ` (${t(ctx, 'available')}: ${M.fmtQuantity(details.available)}, ${t(ctx, 'quantity')}: ${M.fmtQuantity(details.requested)})`;
            if (reason === 'reservation_exceeds_need' && details.remaining_need != null) message += ` (${t(ctx, 'quantity')}: ${M.fmtQuantity(details.remaining_need)})`;
            if (error?.field && M.safeId(error.field)) message += ` · ${t(ctx, 'field_' + error.field, error.field)}`;
            return message;
        }
        const known = {
            readonly: 'readonly_notice', conflict: 'error_conflict', stale: 'error_conflict', not_found: 'error_not_found',
            disabled: 'error_disabled', invalid: 'error_invalid', request_id_conflict: 'error_retry_conflict',
            insufficient_stock: 'error_insufficient_stock', active_reservations: 'release_holds_first',
            unavailable: 'error_generic', policy_changed: 'error_disabled', pending_command: 'pending_command_intro',
            pending_command_queue_full: 'pending_command_queue_full', pending_command_storage_unavailable: 'pending_command_storage_unavailable',
            initial_stock_rejected: 'initial_stock_rejected'
        }[code];
        return t(ctx, known || 'error_generic');
    };
    const unitName = (catalog, id) => asList(catalog?.units).find(value => (typeof value === 'object' ? value.id : value) === id);
    const categoryName = (ctx, category) => label(ctx, category, 'category_generic');

    function render(host, windowId, context) {
        dispose(windowId);
        const ctx = context || {}, abort = new AbortController();
        const requestIDsKey = 'aurago.desktop.findit.requests.v1.' + encodeURIComponent(String(windowId));
        let savedCommandRequests = [];
        try { savedCommandRequests = JSON.parse(sessionStorage.getItem(requestIDsKey) || '[]'); } catch (_) { /* Retry tokens are best-effort. */ }
        const state = {
            host, windowId, ctx, abort, signal: abort.signal, disposed: false,
            status: null, catalog: null, manifest: null, view: 'items', viewMode: safeLocalStorage(LIST_MODE_KEY, 'list') === 'tiles' ? 'tiles' : 'list',
            listColumns: safeLocalStorage(LIST_COLUMNS_KEY, DEFAULT_LIST_COLUMNS),
            queryText: '', offset: 0, limit: M.PAGE_SIZE, total: 0, rows: [], selected: null, selectedId: '', availability: null,
            locations: [], locationId: '', projects: [], project: null, projectShortages: null, templates: [], template: null,
            locationSelected: null,
            category: '', packageFamily: '', filtersOpen: false, onlyAvailable: false, lowStock: false,
            filterTemplateID: '', propertyFilters: {}, propertyFilterOperators: {},
            savedSearches: asList(safeLocalStorage(SAVED_SEARCHES_KEY, [])), busy: false, actionBusy: false, error: '', notice: '',
            mobilePanel: 'list', mobile: false, searchTimer: 0, requestSerial: 0, selectionSerial: 0, csvCandidate: null, printImage: '',
            scanCandidate: '', focusAfterDraw: '', forceReload: false,
            commandRequestIds: new Map(Array.isArray(savedCommandRequests) ? savedCommandRequests.filter(row => Array.isArray(row) && row.length === 2 && typeof row[0] === 'string' && typeof row[1] === 'string').slice(-96) : []),
            pendingCommands: readPendingCommands(), pendingInitialStocks: readPendingInitialStocks(), pendingInitialStock: null
        };
        state.pendingInitialStock = state.pendingInitialStocks[0] || null;
        instances.set(windowId, state);
        const itemFeatures = window.FindItItems.create({ state, host, ctx, tr, canWrite, canPrint, dialogOptions, draw, apiQuery, apiCommand, isDefinitiveCommandRejection, withBusy, loadItems, selectItem, dispatchAction, showNotice, showError, noteText, categoryName, label, compactNumber, unitName, sprite, icon, selectedCategory });
        const projectFeatures = window.FindItProjects.create({ state, host, ctx, tr, canWrite, canPrint, dialogOptions, draw, apiQuery, apiCommand, withBusy, loadItems, loadProjects, loadLocations, selectProject, showNotice, noteText: error => noteText(ctx, error), categoryName, compactNumber, statusClass, sprite, icon });
        const templateFeatures = window.FindItTemplates.create({ state, ctx, tr, canWrite, dialogOptions, draw, apiCommand, withBusy, showNotice, categoryName, icon });
        const workflowFeatures = window.FindItWorkflows.create({ state, host, ctx, tr, canWrite, draw, apiQuery, apiCommand, withBusy, loadItems, loadProjects, selectItem, selectProject, selectLocation, showNotice: (value, error) => showNotice(value, error), noteText: error => noteText(ctx, error), icon });
        state.features = { items: itemFeatures, projects: projectFeatures, templates: templateFeatures, workflows: workflowFeatures };
        const keydown = event => {
            if (event.key === 'Escape' && state.mobilePanel === 'detail' && state.mobile) { state.mobilePanel = 'list'; draw(); }
            if ((event.ctrlKey || event.metaKey) && event.key.toLowerCase() === 'k') {
                const search = host.querySelector('[data-search]'); if (search) { event.preventDefault(); search.focus(); }
            }
            if (event.key === '/' && !/^(INPUT|TEXTAREA|SELECT)$/.test(document.activeElement?.tagName || '')) {
                const search = host.querySelector('[data-search]'); if (search) { event.preventDefault(); search.focus(); }
            }
        };
        host.addEventListener('keydown', keydown, { signal: state.signal });
        const updateMobileWidth = width => {
            const mobile = width > 0 && width <= 760;
            if (mobile === state.mobile) return;
            state.mobile = mobile; draw();
        };
        state.mobile = (host.getBoundingClientRect?.().width || host.clientWidth || 0) <= 760;
        if (typeof ResizeObserver === 'function') {
            state.resizeObserver = new ResizeObserver(entries => { if (!stale()) updateMobileWidth(entries[0]?.contentRect?.width || host.clientWidth); });
            state.resizeObserver.observe(host);
        } else {
            const onResize = () => updateMobileWidth(host.getBoundingClientRect?.().width || host.clientWidth);
            window.addEventListener('resize', onResize);
            state.abort.signal.addEventListener('abort', () => window.removeEventListener('resize', onResize), { once: true });
        }

        function tr(key, fallback) { return t(ctx, key, fallback); }
        function canWrite() { return !!state.status?.can_write && !state.status?.readonly && !ctx.readonly; }
        function canPrint() { return !!state.status?.can_print; }
        function isBusy() { return state.busy || state.actionBusy; }
        function setBusy(value) { state.busy = !!value; }
        function clearNotice() { state.notice = ''; state.error = ''; }
        function showError(error) { state.error = noteText(ctx, error); state.notice = ''; draw(); }
        function showNotice(value, isError = false) { state.notice = isError ? '' : value; state.error = isError ? value : ''; draw(); }
        function isCurrent(serial) { return !state.disposed && !state.signal.aborted && serial === state.requestSerial; }
        function isSelectionCurrent(serial) { return !stale() && serial === state.selectionSerial; }
        function beginRequest() { return ++state.requestSerial; }
        function stale() { return state.disposed || state.signal.aborted; }
        function apiQuery(payload) { return M.query(payload, state.signal); }
        function commandFingerprint(payload) {
            const sort = value => Array.isArray(value) ? value.map(sort) : value && typeof value === 'object' ? Object.fromEntries(Object.keys(value).filter(key => key !== 'request_id').sort().map(key => [key, sort(value[key])])) : value;
            return JSON.stringify(sort(payload));
        }
        function pendingCommands() {
            const records = new Map();
            for (const value of [...readPendingCommands(), ...[...instances.values()].flatMap(instance => instance.pendingCommands || [])]) {
                if (value?.request_id && value?.scope) records.set(value.request_id, value);
            }
            return [...records.values()];
        }
        function pendingInitialStocks() {
            const records = new Map();
            for (const value of [...readPendingInitialStocks(), ...[...instances.values()].flatMap(instance => instance.pendingInitialStocks || [])]) {
                const key = pendingInitialReceiptKey(value); if (key) records.set(key, value);
            }
            return [...records.values()];
        }
        function pendingCommandCount() { return pendingCommands().length + pendingInitialStocks().length; }
        function hasPendingCommandScope(payload, requestId = '') {
            const scope = pendingCommandScope(payload); if (!scope) return false;
            if (pendingCommands().some(value => value.scope === scope && value.request_id !== requestId)) return true;
            return scope.startsWith('stock:') && pendingInitialStocks().some(value => `stock:${value.item_id}` === scope && value.request_id !== requestId);
        }
        function hasPendingCommandIntent(payload) {
            const fingerprint = commandFingerprint(payload);
            if (pendingCommands().some(value => commandFingerprint(value.payload) === fingerprint)) return true;
            return pendingInitialStocks().some(value => commandFingerprint({ operation: 'stock', entity: 'items', id: value.item_id, expected_version: value.item_version, data: value.stock_data }) === fingerprint);
        }
        function persistPendingCommand(record = null, removeRequestId = '') {
            const queue = new Map(pendingCommands().map(value => [value.request_id, value]));
            const replacing = !!record && queue.has(record.request_id);
            if (removeRequestId) queue.delete(removeRequestId);
            if (record) queue.set(record.request_id, record);
            const rows = [...queue.values()];
            if (rows.length > MAX_PENDING_COMMANDS && record && !replacing) return false;
            try { sessionStorage.setItem(PENDING_COMMANDS_KEY, JSON.stringify(rows)); }
            catch (_) { return false; }
            for (const instance of instances.values()) {
                instance.pendingCommands = rows.map(value => ({ ...value, payload: cloneFrozen(value.payload) }));
                instance.draw?.();
            }
            return true;
        }
        function forgetCommandRequest(requestId) {
            for (const [fingerprint, value] of state.commandRequestIds) {
                if (value === requestId) state.commandRequestIds.delete(fingerprint);
            }
            persistCommandRequestIds();
        }
        async function apiCommand(payload, options = {}) {
            const key = commandFingerprint(payload);
            const requestId = payload.request_id || state.commandRequestIds.get(key) || crypto.randomUUID();
            if (!payload.request_id) {
                state.commandRequestIds.delete(key);
                state.commandRequestIds.set(key, requestId);
                while (state.commandRequestIds.size > 96) state.commandRequestIds.delete(state.commandRequestIds.keys().next().value);
                persistCommandRequestIds();
            }
            const command = cloneFrozen({ ...payload, request_id: requestId });
            const scope = options.initialStock ? '' : pendingCommandScope(command);
            let queued = null;
            if (options.replayOnly && !options.initialStock) {
                const exactReceipt = readPendingCommands().find(value => value.request_id === requestId);
                if (!exactReceipt || commandFingerprint(exactReceipt.payload) !== commandFingerprint(command)) {
                    if (!payload.request_id && state.commandRequestIds.get(key) === requestId) forgetCommandRequest(requestId);
                    throw Object.assign(new Error(tr('pending_command_intro')), { code: 'pending_command' });
                }
            }
            if (scope) {
                const existing = pendingCommands().find(value => value.request_id === requestId);
                if (existing && commandFingerprint(existing.payload) !== commandFingerprint(command)) throw Object.assign(new Error(tr('error_retry_conflict')), { code: 'request_id_conflict' });
                if (hasPendingCommandScope(command, requestId)) {
                    if (!payload.request_id && state.commandRequestIds.get(key) === requestId) forgetCommandRequest(requestId);
                    throw Object.assign(new Error(tr('pending_command_intro')), { code: 'pending_command' });
                }
                if (!existing && pendingCommandCount() >= MAX_PENDING_COMMANDS) {
                    if (!payload.request_id && state.commandRequestIds.get(key) === requestId) forgetCommandRequest(requestId);
                    throw Object.assign(new Error(tr('pending_command_queue_full')), { code: 'pending_command_queue_full' });
                }
                queued = existing || { request_id: requestId, payload: command, scope, label: String(options.label || '').slice(0, 256) };
                if (!persistPendingCommand(queued)) {
                    if (!payload.request_id && state.commandRequestIds.get(key) === requestId) forgetCommandRequest(requestId);
                    const full = pendingCommandCount() >= MAX_PENDING_COMMANDS;
                    throw Object.assign(new Error(tr(full ? 'pending_command_queue_full' : 'pending_command_storage_unavailable')), { code: full ? 'pending_command_queue_full' : 'pending_command_storage_unavailable' });
                }
            } else if (options.initialStock) {
                const receipt = readPendingInitialStocks().find(value => value.item_id === command.id && value.request_id === requestId);
                const expected = receipt && { operation: 'stock', entity: 'items', id: receipt.item_id, expected_version: receipt.item_version, request_id: receipt.request_id, data: receipt.stock_data };
                if (!receipt || commandFingerprint(expected) !== commandFingerprint(command)) throw Object.assign(new Error(tr('error_retry_conflict')), { code: 'request_id_conflict' });
                if (hasPendingCommandScope(command, requestId)) throw Object.assign(new Error(tr('pending_command_intro')), { code: 'pending_command' });
            }
            const tracked = !!scope || !!options.initialStock;
            let execution = tracked ? inFlightCommands.get(requestId) : null;
            const ownsExecution = !execution;
            if (!execution) {
                execution = M.command(command, state.signal);
                if (tracked) inFlightCommands.set(requestId, execution);
            }
            try {
                const result = await execution;
                if (scope) { persistPendingCommand(null, requestId); forgetCommandRequest(requestId); }
                else if (!payload.request_id && state.commandRequestIds.get(key) === requestId) { state.commandRequestIds.delete(key); persistCommandRequestIds(); }
                return result;
            } catch (error) {
                if (scope && isDefinitiveCommandRejection(error)) {
                    persistPendingCommand(null, requestId);
                    forgetCommandRequest(requestId);
                }
                throw error;
            } finally {
                if (tracked && ownsExecution && inFlightCommands.get(requestId) === execution) inFlightCommands.delete(requestId);
            }
        }
        function persistPendingInitialStock(record = state.pendingInitialStock, remove = null) {
            const queue = new Map();
            for (const value of [...readPendingInitialStocks(), ...[...instances.values()].flatMap(instance => instance.pendingInitialStocks || [])]) {
                const key = pendingInitialReceiptKey(value); if (key) queue.set(key, value);
            }
            if (remove?.item_id && remove?.request_id) queue.delete(pendingInitialReceiptKey(remove));
            const recordKey = record && pendingInitialReceiptKey(record);
            if (record && !queue.has(recordKey) && pendingCommands().length + queue.size >= MAX_PENDING_COMMANDS) return false;
            if (record) queue.set(recordKey, record);
            const rows = [...queue.values()];
            let persisted = true;
            try { sessionStorage.setItem(PENDING_INITIAL_KEY, JSON.stringify(rows)); } catch (_) { persisted = false; }
            for (const instance of instances.values()) {
                const current = pendingInitialReceiptKey(instance.pendingInitialStock);
                instance.pendingInitialStocks = rows;
                instance.pendingInitialStock = rows.find(value => pendingInitialReceiptKey(value) === current) || rows[0] || null;
                instance.draw?.();
            }
            return persisted;
        }
        function persistCommandRequestIds() {
            try { sessionStorage.setItem(requestIDsKey, JSON.stringify([...state.commandRequestIds])); }
            catch (_) { /* Exact retry is still retained in memory. */ }
        }
        state.persistPendingInitialStock = persistPendingInitialStock;
        state.hasPendingCommandScope = hasPendingCommandScope;
        state.hasPendingCommandIntent = hasPendingCommandIntent;
        state.pendingCommandCount = pendingCommandCount;
        state.hasPendingCommandCapacity = () => pendingCommandCount() < MAX_PENDING_COMMANDS;
        function appTitle() {
            const key = 'desktop.app_findit';
            const value = ctx.t ? ctx.t(key) : '';
            return value && value !== key ? value : NAME;
        }
        function sprite(item, compact = false) { return M.itemImage(item, state.manifest, tr, compact); }
        function selectedCategory() { return state.catalog?.categories?.find(value => (typeof value === 'object' ? value.id : value) === state.category); }
        function pathFrom(root, target) {
            if (root === target) return '';
            const parts = [];
            for (let node = target; node && node !== root; node = node.parentElement) {
                if (!node.parentElement) return '';
                let index = 1;
                for (let sibling = node.previousElementSibling; sibling; sibling = sibling.previousElementSibling) {
                    if (sibling.tagName === node.tagName) index++;
                }
                parts.unshift(`${node.tagName.toLowerCase()}:nth-of-type(${index})`);
            }
            return parts.join(' > ');
        }
        function preserveViewState(oldApp) {
            if (!oldApp) return { focus: '', selection: null, scroll: [] };
            const active = document.activeElement;
            const focus = active && oldApp.contains(active) ? pathFrom(oldApp, active) : '';
            let selection = null;
            if (focus && typeof active.selectionStart === 'number') {
                selection = { start: active.selectionStart, end: active.selectionEnd, direction: active.selectionDirection };
            }
            const scroll = [];
            for (const node of [oldApp, ...oldApp.querySelectorAll('*')]) {
                if (node.scrollTop || node.scrollLeft || node.scrollHeight > node.clientHeight || node.scrollWidth > node.clientWidth) {
                    scroll.push({ path: pathFrom(oldApp, node), top: node.scrollTop, left: node.scrollLeft });
                }
            }
            return { focus, selection, scroll };
        }
        function restoreViewState(app, snapshot) {
            const resolve = path => path ? app.querySelector(path) : app;
            for (const position of snapshot.scroll) {
                const node = resolve(position.path);
                if (node) { node.scrollTop = position.top; node.scrollLeft = position.left; }
            }
            if (snapshot.focus) {
                const node = resolve(snapshot.focus);
                node?.focus({ preventScroll: true });
                if (node && snapshot.selection) {
                    try { node.setSelectionRange(snapshot.selection.start, snapshot.selection.end, snapshot.selection.direction); }
                    catch (_) { /* Selects and buttons do not expose text ranges. */ }
                }
            }
        }

        function renderPendingCommands() {
            if (!state.pendingCommands?.length) return '';
            const actionKey = payload => payload.operation === 'stock'
                ? ({ receive: 'receive_stock', issue: 'issue_stock', transfer: 'transfer', adjust: 'adjust_stock', return: 'return_stock' }[payload.data?.kind] || 'stock_actions')
                : ({ reserve: 'reserve', release: 'release_reserved', consume: 'consume_reserved' }[payload.operation] || 'reserve');
            return `<div class="fi-message is-error fi-pending-commands" role="alert"><span>${M.esc(tr('pending_command_intro'))}</span><div class="fi-pending-command-list">${state.pendingCommands.map(record => `<div class="fi-pending-command-row"><span><b>${M.esc(record.label || record.scope.split(':').at(-1))}</b><small>${M.esc(tr(actionKey(record.payload)))}</small></span><button type="button" class="fi-text-button" data-pending-command="${M.esc(record.request_id)}" ${canWrite() && !isBusy() ? '' : 'disabled'}>${M.esc(tr('retry_exact_command'))}</button></div>`).join('')}</div></div>`;
        }

        function draw() {
            if (stale()) return;
            const app = `<section class="fi-app" data-view="${M.esc(state.view)}" data-mode="${M.esc(state.viewMode)}" data-mobile-panel="${M.esc(state.mobilePanel)}" data-busy="${isBusy()}">
                ${renderHeader()}
                ${state.error ? `<div class="fi-message is-error" role="alert">${icon('close', 16)}<span>${M.esc(state.error)}</span><button type="button" data-action="dismiss-message" aria-label="${M.esc(tr('dismiss'))}">×</button></div>` : ''}
                ${state.notice ? `<div class="fi-message is-success" role="status" aria-live="polite">${icon('check', 16)}<span>${M.esc(state.notice)}</span><button type="button" data-action="dismiss-message" aria-label="${M.esc(tr('dismiss'))}">×</button></div>` : ''}
                ${renderPendingCommands()}
                ${state.pendingInitialStock ? `<div class="fi-message is-error fi-pending-stock" role="alert"><span>${M.esc(tr('initial_stock_pending'))}${state.pendingInitialStock.item_name ? ` · ${M.esc(state.pendingInitialStock.item_name)}` : ''}</span><button type="button" class="fi-text-button" data-action="retry-initial-stock" ${canWrite() ? '' : 'disabled'}>${icon('refresh', 14)}${M.esc(tr('retry_initial_stock'))}</button></div>` : ''}
                ${state.status?.readonly || !canWrite() && state.status?.enabled ? `<div class="fi-readonly-note">${icon('settings', 15)}<span>${M.esc(tr('readonly_notice'))}</span></div>` : ''}
                ${state.status?.enabled ? renderView() : renderActivation()}
                ${renderStatusbar()}
            </section>`;
            const oldApp = host.querySelector('.fi-app');
            const snapshot = preserveViewState(oldApp);
            const template = document.createElement('template');
            template.innerHTML = app;
            const nextApp = template.content.firstElementChild;
            if (oldApp) oldApp.replaceWith(nextApp);
            else host.prepend(nextApp);
            if (isBusy()) {
                const safeWhileBusy = new Set(['app-settings', 'back-list', 'back-locations', 'back-projects', 'back-templates', 'choose-columns', 'clear-filters', 'dismiss-message', 'export-items', 'export-project-csv', 'package-filter', 'print-label', 'print-location-label', 'print-pick-list', 'refresh', 'scan-qr', 'toggle-filters']);
                nextApp.querySelectorAll('button[data-action]').forEach(button => {
                    if (!safeWhileBusy.has(button.dataset.action)) button.disabled = true;
                });
            }
            restoreViewState(nextApp, snapshot);
            if (state.focusAfterDraw) { const focus = nextApp.querySelector(`[data-focus="${CSS.escape(state.focusAfterDraw)}"]`); focus?.focus({ preventScroll: true }); state.focusAfterDraw = ''; }
        }
        state.draw = draw;
        function renderHeader() {
            const nav = [['items', 'workbench'], ['projects', 'projects'], ['locations', 'locations'], ['templates', 'templates']];
            return `<header class="fi-header"><div class="fi-header-main">
                <div class="fi-brand"><span class="fi-brand-mark">${icon('box', 19)}</span><div><strong>${M.esc(appTitle())}</strong><small>${M.esc(tr('subtitle'))}</small></div></div>
                <nav class="fi-primary-nav" aria-label="${M.esc(tr('navigation'))}" role="tablist">${nav.map(([id, key]) => `<button type="button" role="tab" aria-selected="${state.view === id}" class="${state.view === id ? 'is-active' : ''}" data-view="${id}">${M.esc(tr(key))}</button>`).join('')}</nav>
                <div class="fi-header-actions"><button type="button" class="fi-capture-button" data-action="capture" ${canWrite() && !state.pendingInitialStocks?.length ? '' : 'disabled'}>${icon('camera', 16)}<span>${M.esc(tr('capture'))}</span></button>${state.status?.can_manage ? `<button type="button" class="fi-icon-button" data-action="app-settings" aria-label="${M.esc(tr('findit_settings'))}" title="${M.esc(tr('findit_settings'))}">${icon('settings', 17)}</button>` : ''}<button type="button" class="fi-icon-button" data-action="scan-qr" aria-label="${M.esc(tr('scan_qr'))}" title="${M.esc(tr('scan_qr'))}">${icon('qr', 17)}</button><button type="button" class="fi-icon-button" data-action="refresh" aria-label="${M.esc(tr('refresh'))}" title="${M.esc(tr('refresh'))}">${icon('refresh', 17)}</button></div>
            </div>${state.status?.readonly ? `<div class="fi-banner"><span>${M.esc(tr('readonly_banner'))}</span></div>` : ''}</header>`;
        }
        function renderStatusbar() {
            const start = state.total ? state.offset + 1 : 0, end = Math.min(state.offset + state.limit, state.total);
            return `<footer class="fi-statusbar"><span>${M.esc(tr('last_updated'))} ${M.esc(new Intl.DateTimeFormat(undefined, { hour: '2-digit', minute: '2-digit' }).format(new Date()))}</span><span>${M.esc(tr('page_range'))} ${M.esc(`${start}–${end} / ${state.total}`)}</span><span class="fi-statusbar-state"><i></i>${isBusy() ? M.esc(tr('working')) : M.esc(tr('ready'))}</span></footer>`;
        }
        function renderActivation() {
            const enabled = !!state.status?.enabled, canManage = !!state.status?.can_manage;
            return `<main class="fi-activation"><div class="fi-activation-card"><div class="fi-activation-art">${icon('box', 42)}</div><p class="fi-eyebrow">${M.esc(tr('app_label'))}</p><h1>${M.esc(tr(enabled ? 'not_ready' : 'activation_title'))}</h1><p>${M.esc(tr('activation_help'))}</p>
                ${state.status?.ready === false && enabled ? `<div class="fi-message is-error">${M.esc(tr('not_ready'))}</div>` : ''}
                ${canManage ? `<form class="fi-activation-form" data-settings-form><label class="fi-toggle-row"><span><b>${M.esc(tr('enable_findit'))}</b><small>${M.esc(tr('enable_findit_help'))}</small></span><input name="enabled" type="checkbox" ${enabled ? 'checked' : ''}></label><label class="fi-toggle-row"><span><b>${M.esc(tr('agent_access'))}</b><small>${M.esc(tr('agent_access_help'))}</small></span><input name="agent_access" type="checkbox" ${state.status?.agent_access ? 'checked' : ''}></label><label class="fi-toggle-row"><span><b>${M.esc(tr('read_only_mode'))}</b><small>${M.esc(tr('read_only_mode_help'))}</small></span><input name="readonly" type="checkbox" ${state.status?.findit_readonly ?? state.status?.readonly ? 'checked' : ''}></label><button class="fi-primary" type="submit" ${state.busy ? 'disabled' : ''}>${M.esc(tr('save_settings'))}</button></form>` : `<p class="fi-permission-note">${M.esc(tr('admin_activation_note'))}</p>`}
            </div></main>`;
        }
        function renderView() {
            if (state.view === 'projects') return renderProjects();
            if (state.view === 'locations') return renderLocations();
            if (state.view === 'templates') return renderTemplates();
            return renderItems();
        }
        function renderItems() { return itemFeatures.renderItems(); }
        function renderProjects() { return projectFeatures.renderProjects(); }
        function renderLocations() { return projectFeatures.renderLocations(); }
        function renderTemplates() { return templateFeatures.renderTemplates(); }
        async function initialize() {
            draw(); setBusy(true); draw();
            try {
                state.status = await M.getStatus(state.signal);
                if (stale()) return;
                if (state.status.enabled) {
                    const [catalog, manifest] = await Promise.all([
                        M.getCatalog(state.signal),
                        M.loadManifest(state.signal).catch(() => null)
                    ]);
                    if (stale()) return;
                    state.catalog = catalog; state.manifest = manifest;
                    await Promise.all([state.view === 'locations' ? Promise.resolve() : loadLocations(), loadTemplates(), loadViewData()]);
                    const itemId = M.safeId(ctx.item_id);
                    if (itemId) await selectItem(itemId, true);
                    const locationId = M.safeId(ctx.location_id);
                    if (!itemId && locationId) await selectLocation(locationId, true);
                }
            } catch (error) { if (!stale()) state.error = noteText(ctx, error); }
            finally { if (!stale()) { setBusy(false); draw(); } }
        }
        function payloadFilters() {
            const template = asList(state.catalog?.templates).find(value => value.id === state.filterTemplateID);
            return asList(template?.fields).flatMap(field => {
                const raw = state.propertyFilters[field.key];
                if (raw == null || raw === '') return [];
                const value = field.type === 'boolean' ? raw === true || raw === 'true' : raw;
                return [{ field: field.key, operator: ['number', 'date'].includes(field.type) ? (state.propertyFilterOperators[field.key] || 'gte') : 'eq', value }];
            }).slice(0, 8);
        }
        async function searchAll(entity, query = '') {
            const pageSize = 200;
            const first = await apiQuery({ operation: 'search', entity, query, limit: pageSize, offset: 0 });
            const items = asList(first?.items || first?.[entity] || first);
            const total = Number(first?.total) || items.length;
            for (let offset = pageSize; offset < total; offset += pageSize) {
                const page = await apiQuery({ operation: 'search', entity, query, limit: pageSize, offset });
                const next = asList(page?.items || page?.[entity] || page);
                if (!next.length) break;
                items.push(...next);
            }
            return items;
        }
        async function ensureItemTemplate(item, selection) {
            if (!item?.template_id) return null;
            const cached = asList(state.catalog?.templates).find(value => value.id === item.template_id);
            if (cached) return cached;
            try {
                const response = await apiQuery({ operation: 'get', entity: 'templates', id: item.template_id });
                if (!isSelectionCurrent(selection)) return null;
                const template = response?.template || response;
                if (!template?.id || template.id !== item.template_id) return null;
                const merged = new Map(asList(state.catalog?.templates).map(value => [value.id, value]));
                merged.set(template.id, template);
                state.templates = [...merged.values()];
                if (state.catalog) state.catalog.templates = state.templates;
                return template;
            } catch (error) {
                if (error?.name !== 'AbortError' && isSelectionCurrent(selection)) console.warn('FindIt could not load the item template definition', error);
                return null;
            }
        }
        async function loadItems() {
            const serial = beginRequest(); state.busy = !state.rows.length; draw();
            try {
                const body = await apiQuery({ operation: 'search', entity: 'items', query: state.queryText, location_id: state.locationId || undefined,
                    category: state.category || undefined, package: state.packageFamily || undefined, available_only: state.onlyAvailable, low_stock: state.lowStock,
                    template_id: state.filterTemplateID || undefined, filters: payloadFilters(), limit: state.limit, offset: state.offset });
                if (!isCurrent(serial)) return;
                state.rows = asList(body?.items); state.total = Number(body?.total) || 0;
                if (state.selectedId && !state.rows.some(item => item.id === state.selectedId)) { state.selected = null; state.selectedId = ''; state.availability = null; }
            } catch (error) { if (isCurrent(serial) && error?.name !== 'AbortError') state.error = noteText(ctx, error); }
            finally { if (isCurrent(serial)) { state.busy = false; draw(); } }
        }
        async function loadLocations() {
            try {
                const locations = await searchAll('locations');
                if (!stale()) { state.locations = locations; draw(); }
            } catch (error) { if (!stale() && error?.name !== 'AbortError') state.error = noteText(ctx, error); }
        }
        async function loadProjects() {
            const serial = beginRequest(); state.busy = true; draw();
            try {
                const projects = await searchAll('projects', state.queryText);
                if (!isCurrent(serial)) return;
                state.projects = projects;
                if (state.project && !state.projects.some(project => project.id === state.project.id)) state.project = null;
            } catch (error) { if (isCurrent(serial) && error?.name !== 'AbortError') state.error = noteText(ctx, error); }
            finally { if (isCurrent(serial)) { state.busy = false; draw(); } }
        }
        async function loadTemplates() {
            const templates = await searchAll('templates');
            const merged = new Map(asList(state.catalog?.templates).map(value => [value.id, value]));
            for (const value of templates) merged.set(value.id, value);
            state.templates = [...merged.values()].sort((a, b) => String(a.name).localeCompare(String(b.name)));
            if (state.catalog) state.catalog.templates = state.templates;
            draw();
        }
        async function loadViewData() {
            if (!state.status?.enabled) return;
            if (state.view === 'items') return loadItems();
            if (state.view === 'projects') return loadProjects();
            if (state.view === 'locations') return loadLocations();
            if (state.view === 'templates') return loadTemplates();
        }
        async function selectItem(id, fromLaunch = false) {
            const safe = M.safeId(id); if (!safe) return;
            const selection = ++state.selectionSerial;
            state.selectedId = safe; state.mobilePanel = 'detail'; state.busy = true; state.error = ''; draw();
            try {
                const [record, availability] = await Promise.all([
                    apiQuery({ operation: 'get', entity: 'items', id: safe }),
                    apiQuery({ operation: 'availability', entity: 'items', id: safe })
                ]);
                if (!isSelectionCurrent(selection)) return;
                state.selected = record?.item || record;
                state.availability = availability;
                if (fromLaunch && !state.selected?.id) throw Object.assign(new Error(tr('error_not_found')), { code: 'not_found' });
                await ensureItemTemplate(state.selected, selection);
                if (!isSelectionCurrent(selection)) return;
                draw();
            } catch (error) {
                if (isSelectionCurrent(selection)) { state.selected = null; state.selectedId = ''; state.availability = null; state.error = noteText(ctx, error); draw(); }
            } finally { if (isSelectionCurrent(selection)) { state.busy = false; draw(); } }
        }
        state.openItem = selectItem;
        async function selectProject(id) {
            const safe = M.safeId(id); if (!safe) return;
            const selection = ++state.selectionSerial;
            state.project = { id: safe }; state.projectShortages = null; state.mobilePanel = 'detail'; state.busy = true; draw();
            try {
                const [record, shortages] = await Promise.all([
                    apiQuery({ operation: 'get', entity: 'projects', id: safe }),
                    apiQuery({ operation: 'shortages', entity: 'projects', id: safe })
                ]);
                if (!isSelectionCurrent(selection)) return;
                state.project = record?.project || record; state.projectShortages = shortages;
                draw();
            } catch (error) { if (isSelectionCurrent(selection)) { state.error = noteText(ctx, error); draw(); } }
            finally { if (isSelectionCurrent(selection)) { state.busy = false; draw(); } }
        }
        async function selectLocation(id, fromLaunch = false) {
            const safe = M.safeId(id); if (!safe) return;
            const selection = ++state.selectionSerial;
            state.view = 'locations'; state.locationId = safe; state.locationSelected = null; state.mobilePanel = 'detail'; state.busy = true; draw();
            try {
                const record = await apiQuery({ operation: 'get', entity: 'locations', id: safe });
                if (!isSelectionCurrent(selection)) return;
                state.locationSelected = record?.location || record;
                if (fromLaunch && !state.locationSelected?.id) throw Object.assign(new Error(tr('error_not_found')), { code: 'not_found' });
                draw();
            } catch (error) {
                if (isSelectionCurrent(selection)) { state.locationSelected = null; state.locationId = ''; state.error = noteText(ctx, error); draw(); }
            } finally { if (isSelectionCurrent(selection)) { state.busy = false; draw(); } }
        }
        state.openLocation = selectLocation;
        async function selectTemplate(id) {
            state.selectionSerial++;
            const template = asList(state.catalog?.templates).find(value => value.id === id) || asList(state.templates).find(value => value.id === id);
            state.template = template || null; state.mobilePanel = 'detail'; draw();
        }
        async function setView(view) {
            if (!['items', 'projects', 'locations', 'templates'].includes(view) || state.view === view) return;
            state.selectionSerial++;
            state.view = view; state.offset = 0; state.mobilePanel = 'list'; state.queryText = ''; state.error = ''; state.notice = '';
            if (view === 'items') { state.project = null; state.template = null; }
            if (view === 'projects') { state.selected = null; state.selectedId = ''; }
            if (view === 'locations') { state.selected = null; state.selectedId = ''; }
            if (view === 'templates') { state.selected = null; state.selectedId = ''; }
            draw(); await loadViewData();
        }
        async function refresh() {
            clearNotice(); setBusy(true); draw();
            try {
                state.status = await M.getStatus(state.signal);
                if (state.status.enabled) {
                    state.catalog = await M.getCatalog(state.signal);
                    await Promise.all([state.view === 'locations' ? Promise.resolve() : loadLocations(), loadViewData()]);
                    if (state.selectedId) await selectItem(state.selectedId);
                    else if (state.project?.id) await selectProject(state.project.id);
                }
            } catch (error) { if (!stale()) state.error = noteText(ctx, error); }
            finally { if (!stale()) { setBusy(false); draw(); } }
        }
        function dialogOptions(options) { return { host, t: (key, fallback) => tr(String(key).replace(/^findit\./, ''), fallback), signal: state.signal, ...options }; }
        async function editItem(creating = false) { return itemFeatures.editItem(creating); }
        async function editDatasheets() { return itemFeatures.editDatasheets(); }
        async function updateItem(patch) { return itemFeatures.updateItem(patch); }
        async function archiveItem() { return itemFeatures.archiveItem(); }
        async function uploadPhoto(file) { return itemFeatures.uploadPhoto(file); }
        async function stockMove(kind, stock = null) { return itemFeatures.stockMove(kind, stock); }
        async function projectForm(project = null) { return projectFeatures.projectForm(project); }
        async function addProjectLine() { return projectFeatures.addProjectLine(); }
        async function removeProjectLine(lineId) { return projectFeatures.removeProjectLine(lineId); }
        async function updateProject(patch) { return projectFeatures.updateProject(patch); }
        async function projectStatus(status) { return projectFeatures.projectStatus(status); }
        async function reserveLine(lineId) { return projectFeatures.reserveLine(lineId); }
        async function reservationAction(action, reservationId) { return projectFeatures.reservationAction(action, reservationId); }
        async function locationForm(location = null) { return projectFeatures.locationForm(location); }
        async function archiveLocation() { return projectFeatures.archiveLocation(); }
        async function templateForm(template = null) { return templateFeatures.templateForm(template); }
        async function archiveTemplate() { return templateFeatures.archiveTemplate(); }
        async function saveSettings(settings) {
            if (!state.status?.can_manage) return;
            await withBusy(async () => {
                state.status = await M.updateSettings(settings, state.status.config_revision, state.signal);
                state.notice = tr('settings_saved');
                if (state.status.enabled && !state.catalog) {
                    state.catalog = await M.getCatalog(state.signal);
                    state.manifest = await M.loadManifest(state.signal).catch(() => null);
                    await Promise.all([state.view === 'locations' ? Promise.resolve() : loadLocations(), loadTemplates(), loadViewData()]);
                }
                draw();
            });
        }
        async function openSettings() {
            if (!state.status?.can_manage) return;
            const result = await D.openForm(dialogOptions({
                kicker: 'app_label', title: 'findit_settings', description: 'findit_settings_help', submitLabel: 'save_settings',
                fields: [
                    { key: 'enabled', label: 'enable_findit', type: 'checkbox', help: 'enable_findit_help', wide: true },
                    { key: 'agent_access', label: 'agent_access', type: 'checkbox', help: 'agent_access_help', wide: true },
                    { key: 'readonly', label: 'read_only_mode', type: 'checkbox', help: 'read_only_mode_help', wide: true }
                ], values: { enabled: !!state.status.enabled, agent_access: !!state.status.agent_access, readonly: !!(state.status.findit_readonly ?? state.status.readonly) }
            }));
            if (result) await saveSettings(result);
        }
        async function retryInitialStock() {
            const pending = state.pendingInitialStock;
            if (!pending || !canWrite()) return;
            const selection = state.selectionSerial;
            const item = { id: pending.item_id, name: pending.item_name, unit: pending.unit, version: pending.item_version, track_serials: pending.track_serials };
            const stockData = pending.stock_data;
            const location = asList(state.locations).find(value => value.id === stockData.location_id)?.name || stockData.location_id || tr('unknown_location');
            const details = `<dl class="fi-detail-fields fi-retry-stock-summary"><div><dt>${M.esc(tr('part'))}</dt><dd>${M.esc(item.name || item.id)}</dd></div><div><dt>${M.esc(tr('location'))}</dt><dd>${M.esc(location)}</dd></div><div><dt>${M.esc(tr('quantity'))}</dt><dd>${M.esc(M.fmtQuantity(stockData.quantity))} ${M.esc(label(ctx, unitName(state.catalog, item.unit) || item.unit || 'piece'))}</dd></div>${stockData.lot ? `<div><dt>${M.esc(tr('lot'))}</dt><dd>${M.esc(stockData.lot)}</dd></div>` : ''}${stockData.serial ? `<div><dt>${M.esc(tr('serial_short'))}</dt><dd>${M.esc(stockData.serial)}</dd></div>` : ''}</dl>`;
            const confirmed = await D.openConfirm(dialogOptions({
                kicker: 'retry_initial_stock', title: 'retry_initial_stock', description: 'initial_stock_retry_intro',
                contentHTML: details, confirmLabel: 'retry_initial_stock'
            }));
            if (!confirmed) return;
            const exactReceipt = readPendingInitialStocks().find(value => value.item_id === pending.item_id && value.request_id === pending.request_id);
            if (!exactReceipt || commandFingerprint({ operation: 'stock', entity: 'items', id: exactReceipt.item_id, expected_version: exactReceipt.item_version, data: exactReceipt.stock_data }) !== commandFingerprint({ operation: 'stock', entity: 'items', id: pending.item_id, expected_version: pending.item_version, data: pending.stock_data })) return;
            if (!canWrite() || stale()) return;
            await withBusy(async () => {
                try {
                    await apiCommand({ operation: 'stock', entity: 'items', id: item.id, expected_version: pending.item_version, request_id: pending.request_id, data: stockData }, { initialStock: true, replayOnly: true, label: pending.item_name });
                } catch (error) {
                    if (isDefinitiveCommandRejection(error)) {
                        persistPendingInitialStock(null, { item_id: pending.item_id, request_id: pending.request_id });
                        state.error = tr('initial_stock_rejected');
                        state.notice = '';
                        await loadItems(); if (state.selectionSerial === selection && state.view === 'items' && state.selectedId === item.id) await selectItem(item.id);
                        return;
                    }
                    state.error = tr('initial_stock_pending');
                    throw error;
                }
                persistPendingInitialStock(null, { item_id: pending.item_id, request_id: pending.request_id });
                state.notice = tr('initial_stock_saved');
                await loadItems(); if (state.selectionSerial === selection && state.view === 'items' && state.selectedId === item.id) await selectItem(item.id);
            });
        }
        function pendingCommandDetails(record) {
            const payload = record.payload, data = payload.data || {};
            const operationKey = payload.operation === 'stock'
                ? ({ receive: 'receive_stock', issue: 'issue_stock', transfer: 'transfer', adjust: 'adjust_stock', return: 'return_stock' }[data.kind] || 'stock_actions')
                : ({ reserve: 'reserve', release: 'release_reserved', consume: 'consume_reserved' }[payload.operation] || 'reserve');
            const rows = [[tr(payload.operation === 'stock' ? 'part' : 'project'), record.label || data.item_id || data.project_id || payload.id || ''], [tr('stock_actions'), tr(operationKey)]];
            if (data.quantity != null) rows.push([tr('quantity'), M.fmtQuantity(data.quantity)]);
            if (data.location_id) rows.push([tr('location'), asList(state.locations).find(value => value.id === data.location_id)?.name || data.location_id]);
            if (data.target_location_id) rows.push([tr('target_location'), asList(state.locations).find(value => value.id === data.target_location_id)?.name || data.target_location_id]);
            if (data.stock_id) rows.push([tr('stock_record'), data.stock_id]);
            if (data.serial) rows.push([tr('serial_short'), data.serial]);
            return `<dl class="fi-detail-fields fi-retry-stock-summary">${rows.map(([key, value]) => `<div><dt>${M.esc(key)}</dt><dd>${M.esc(value)}</dd></div>`).join('')}</dl>`;
        }
        async function retryPendingCommand(requestId) {
            const pending = state.pendingCommands.find(value => value.request_id === requestId);
            if (!pending || !canWrite() || isBusy()) return;
            const selection = state.selectionSerial;
            const confirmed = await D.openConfirm(dialogOptions({ title: 'pending_command', message: 'pending_command_intro', contentHTML: pendingCommandDetails(pending), confirmLabel: 'retry_exact_command' }));
            if (!confirmed) return;
            const exactCommand = readPendingCommands().find(value => value.request_id === requestId);
            if (!exactCommand || commandFingerprint(exactCommand.payload) !== commandFingerprint(pending.payload) || !canWrite() || stale()) return;
            await withBusy(async () => {
                await apiCommand(pending.payload, { replayOnly: true, label: pending.label });
                state.notice = tr(({ stock: 'movement_saved', reserve: 'reservation_created', release: 'reservation_released', consume: 'stock_consumed' })[pending.payload.operation] || 'item_saved');
                if (pending.payload.operation === 'stock') {
                    await loadItems();
                    const itemId = pending.payload.data?.item_id || pending.payload.id;
                    if (state.selectionSerial === selection && state.view === 'items' && state.selectedId === itemId) await selectItem(itemId);
                } else if (state.view === 'projects') {
                    await loadProjects();
                    const projectId = pending.payload.data?.project_id || pending.payload.id;
                    if (state.selectionSerial === selection && state.project?.id === projectId) await selectProject(projectId);
                }
            });
        }
        async function saveSearch() {
            const result = await D.openForm(dialogOptions({ title: 'save_search', fields: [{ key: 'name', label: 'search_name', type: 'text', required: true }], values: { name: state.queryText || state.category || tr('new_search') }, submitLabel: 'save' }));
            if (!result) return;
            const saved = [...state.savedSearches, { name: result.name.trim(), query: state.queryText, category: state.category, package_family: state.packageFamily, location_id: state.locationId, available_only: state.onlyAvailable, low_stock: state.lowStock, template_id: state.filterTemplateID, property_filters: state.propertyFilters, property_filter_operators: state.propertyFilterOperators }].slice(-20);
            state.savedSearches = saved; storeLocal(SAVED_SEARCHES_KEY, saved); draw();
        }
        function applySavedSearch(index) {
            const saved = state.savedSearches[index]; if (!saved) return;
            state.queryText = String(saved.query || ''); state.category = String(saved.category || ''); state.packageFamily = String(saved.package_family || '');
            state.locationId = M.safeId(saved.location_id) || ''; state.onlyAvailable = !!saved.available_only; state.lowStock = !!saved.low_stock;
            state.filterTemplateID = M.safeId(saved.template_id) || ''; state.propertyFilters = saved.property_filters && typeof saved.property_filters === 'object' ? { ...saved.property_filters } : {};
            state.propertyFilterOperators = saved.property_filter_operators && typeof saved.property_filter_operators === 'object' ? { ...saved.property_filter_operators } : {};
            state.offset = 0; loadItems();
        }
        function removeSavedSearch(index) {
            state.savedSearches = state.savedSearches.filter((_, row) => row !== index); storeLocal(SAVED_SEARCHES_KEY, state.savedSearches); draw();
        }
        async function showItemActions(id) { return itemFeatures.showItemActions(id); }
        async function showStockActions(stockId, locationId) { return itemFeatures.showStockActions(stockId, locationId); }
        function handleClick(event) {
            const target = event.target.closest('button,[data-select-item],[data-select-project],[data-select-location],[data-select-template],[data-page]');
            if (!target || !host.contains(target)) return;
            if (target.dataset.pendingCommand) { void retryPendingCommand(target.dataset.pendingCommand); return; }
            if (target.dataset.view) { void setView(target.dataset.view); return; }
            if (target.dataset.mode) { state.viewMode = target.dataset.mode === 'tiles' ? 'tiles' : 'list'; storeLocal(LIST_MODE_KEY, state.viewMode); draw(); return; }
            if (target.dataset.category != null) { state.category = target.dataset.category; state.offset = 0; void loadItems(); return; }
            if (target.dataset.location != null && state.view === 'items') { state.locationId = target.dataset.location; state.offset = 0; void loadItems(); return; }
            if (target.dataset.selectItem) { void selectItem(target.dataset.selectItem); return; }
            if (target.dataset.selectProject) { void selectProject(target.dataset.selectProject); return; }
            if (target.dataset.selectLocation) { state.locationId = target.dataset.selectLocation; state.locationSelected = state.locations.find(item => item.id === state.locationId) || null; state.mobilePanel = 'detail'; draw(); return; }
            if (target.dataset.selectTemplate) { void selectTemplate(target.dataset.selectTemplate); return; }
            if (target.dataset.page) { state.offset = Math.max(0, state.offset + (target.dataset.page === 'next' ? state.limit : -state.limit)); void loadItems(); return; }
            const removeSaved = event.target.closest('[data-remove-saved]');
            if (removeSaved) { event.stopPropagation(); removeSavedSearch(Number(removeSaved.dataset.removeSaved)); return; }
            if (target.dataset.savedSearch != null) { applySavedSearch(Number(target.dataset.savedSearch)); return; }
            if (target.dataset.clearLocation != null) { state.locationId = ''; void loadItems(); return; }
            if (target.dataset.clearPackage != null) { state.packageFamily = ''; void loadItems(); return; }
            if (target.dataset.clearTemplate != null) { state.filterTemplateID = ''; state.propertyFilters = {}; state.propertyFilterOperators = {}; void loadItems(); return; }
            if (target.dataset.clearProperty != null) { delete state.propertyFilters[target.dataset.clearProperty]; void loadItems(); return; }
            if (target.dataset.clearAvailable != null) { state.onlyAvailable = false; void loadItems(); return; }
            if (target.dataset.clearLowStock != null) { state.lowStock = false; void loadItems(); return; }
            if (target.dataset.itemMenu) { void showItemActions(target.dataset.itemMenu); return; }
            if (target.dataset.stockMenu) { void showStockActions(target.dataset.stockMenu, target.dataset.stockLocation); return; }
            if (target.dataset.removeLine != null) { void removeProjectLine(target.dataset.removeLine); return; }
            if (target.dataset.reserveLine != null) { void reserveLine(target.dataset.reserveLine); return; }
            if (target.dataset.reservationAction) { void reservationAction(target.dataset.reservationAction, target.dataset.reservationId); return; }
            if (target.dataset.action) { void dispatchAction(target.dataset.action); }
        }
        async function dispatchAction(action) {
            clearNotice();
            switch (action) {
                case 'dismiss-message': state.error = ''; state.notice = ''; draw(); break;
                case 'refresh': await refresh(); break;
                case 'app-settings': await openSettings(); break;
                case 'retry-initial-stock': await retryInitialStock(); break;
                case 'capture': await editItem(true); break;
                case 'new-item': await editItem(true); break;
                case 'edit-item': await editItem(false); break;
                case 'archive-item': await archiveItem(); break;
                case 'edit-datasheets': await editDatasheets(); break;
                case 'stock-receive': await stockMove('receive'); break;
                case 'stock-issue': await stockMove('issue'); break;
                case 'stock-transfer': await stockMove('transfer'); break;
                case 'stock-adjust': await stockMove('adjust'); break;
                case 'stock-return': await stockMove('return'); break;
                case 'upload-photo': host.querySelector('[data-photo-input]')?.click(); break;
                case 'print-label': if (canPrint()) await workflowFeatures.printLabel(state.selected); else showNotice(tr('readonly_notice'), true); break;
                case 'print-location-label': if (canPrint()) await workflowFeatures.printLabel(state.locationSelected || state.locations.find(item => item.id === state.locationId), 'location'); else showNotice(tr('readonly_notice'), true); break;
                case 'print-pick-list': if (canPrint()) projectFeatures.printPickList(state.project); else showNotice(tr('readonly_notice'), true); break;
                case 'scan-qr': await workflowFeatures.scanQR(); break;
                case 'toggle-filters': state.filtersOpen = !state.filtersOpen; draw(); break;
                case 'package-filter': state.filtersOpen = true; draw(); host.querySelector('[data-filter-package]')?.focus(); break;
                case 'clear-filters': state.category = ''; state.packageFamily = ''; state.locationId = ''; state.onlyAvailable = false; state.lowStock = false; state.filterTemplateID = ''; state.propertyFilters = {}; state.propertyFilterOperators = {}; state.offset = 0; await loadItems(); break;
                case 'save-search': await saveSearch(); break;
                case 'choose-columns': await itemFeatures.chooseColumns(); break;
                case 'new-location': await locationForm(); break;
                case 'edit-location': await locationForm(state.locations.find(item => item.id === state.locationId)); break;
                case 'archive-location': await archiveLocation(); break;
                case 'new-project': await projectForm(); break;
                case 'edit-project': await projectForm(state.project); break;
                case 'add-project-line': await addProjectLine(); break;
                case 'release-project': await projectStatus('released'); break;
                case 'cancel-project': await projectStatus('cancelled'); break;
                case 'complete-project': await projectStatus('completed'); break;
                case 'reopen-project': await projectStatus('draft'); break;
                case 'new-template': await templateForm(); break;
                case 'edit-template': await templateForm(state.template); break;
                case 'archive-template': await archiveTemplate(); break;
                case 'import-csv': await workflowFeatures.csvWizard('items'); break;
                case 'import-project-csv': await workflowFeatures.csvWizard('project_lines'); break;
                case 'export-items': await workflowFeatures.exportCSV('items'); break;
                case 'export-project-csv': if (state.project) await workflowFeatures.exportCSV('project_lines', state.project.id); break;
                case 'back-list': state.mobilePanel = 'list'; draw(); break;
                case 'back-projects': state.mobilePanel = 'list'; draw(); break;
                case 'back-locations': state.mobilePanel = 'list'; draw(); break;
                case 'back-templates': state.mobilePanel = 'list'; draw(); break;
            }
        }
        function wireEvents() {
            host.addEventListener('click', handleClick, { signal: state.signal });
            host.addEventListener('input', event => {
                if (event.target.matches('[data-search]')) {
                    state.queryText = event.target.value; state.offset = 0; clearTimeout(state.searchTimer);
                    state.searchTimer = setTimeout(() => { if (!stale()) void loadItems(); }, 220);
                } else if (event.target.matches('[data-filter-property]') && event.target.matches('input')) {
                    state.propertyFilters[event.target.dataset.filterProperty] = event.target.value; state.offset = 0; clearTimeout(state.searchTimer);
                    state.searchTimer = setTimeout(() => { if (!stale()) void loadItems(); }, 240);
                }
            }, { signal: state.signal });
            host.addEventListener('change', event => {
                const target = event.target;
                if (target.matches('[data-filter-category]')) { state.category = target.value; state.offset = 0; void loadItems(); }
                else if (target.matches('[data-filter-package]')) { state.packageFamily = target.value; state.offset = 0; void loadItems(); }
                else if (target.matches('[data-filter-template]')) { state.filterTemplateID = target.value; state.propertyFilters = {}; state.propertyFilterOperators = {}; state.offset = 0; draw(); void loadItems(); }
                else if (target.matches('[data-filter-property]')) { state.propertyFilters[target.dataset.filterProperty] = target.value; state.offset = 0; void loadItems(); }
                else if (target.matches('[data-filter-operator]')) { state.propertyFilterOperators[target.dataset.filterOperator] = target.value; state.offset = 0; void loadItems(); }
                else if (target.matches('[data-filter-available]')) { state.onlyAvailable = target.checked; state.offset = 0; void loadItems(); }
                else if (target.matches('[data-filter-low-stock]')) { state.lowStock = target.checked; state.offset = 0; void loadItems(); }
                else if (target.matches('[data-photo-input]') && target.files?.[0]) { void uploadPhoto(target.files[0]).catch(error => showError(error)); target.value = ''; }
            }, { signal: state.signal });
            host.addEventListener('submit', event => {
                if (event.target.matches('[data-settings-form]')) { event.preventDefault(); void saveSettings({ enabled: event.target.elements.enabled.checked, agent_access: event.target.elements.agent_access.checked, readonly: event.target.elements.readonly.checked }).catch(error => showError(error)); }
            }, { signal: state.signal });
            host.addEventListener('keydown', event => {
                const row = event.target.closest('[data-select-item]');
                if (row && (event.key === 'Enter' || event.key === ' ')) { event.preventDefault(); void selectItem(row.dataset.selectItem); return; }
                if (row && ['ArrowDown', 'ArrowUp'].includes(event.key)) {
                    const rows = [...host.querySelectorAll('[data-select-item]')], index = rows.indexOf(row), next = rows[index + (event.key === 'ArrowDown' ? 1 : -1)];
                    if (next) { event.preventDefault(); next.focus(); next.scrollIntoView({ block: 'nearest' }); }
                }
            }, { signal: state.signal });
        }
        async function withBusy(action) {
            if (stale()) return false;
            while ((state.busy || state.actionBusy) && !stale()) await new Promise(resolve => setTimeout(resolve, 16));
            if (stale()) return false;
            state.actionBusy = true; state.busy = true; state.error = ''; draw();
            try { await action(); return true; }
            catch (error) { if (!stale() && error?.name !== 'AbortError') state.error = noteText(ctx, error); return false; }
            finally { if (!stale()) { state.actionBusy = false; state.busy = false; draw(); } }
        }
        wireEvents();
        void initialize();
        state.retryInitialStock = retryInitialStock;
        return state;
    }

    function dispose(windowId) {
        const state = instances.get(windowId); if (!state) return;
        state.disposed = true; clearTimeout(state.searchTimer); state.abort.abort(); state.resizeObserver?.disconnect?.();
        state.features?.workflows?.dispose?.();
        state.host?.replaceChildren(); instances.delete(windowId);
    }
    function open(windowId, context) {
        const state = instances.get(windowId), itemId = M.safeId(context?.item_id), locationId = M.safeId(context?.location_id);
        if (!state) return false;
        if (itemId) return state.openItem?.(itemId) || false;
        if (locationId) return state.openLocation?.(locationId) || false;
        return false;
    }
    window.FindItApp = { render, dispose, open, instances };
})();
