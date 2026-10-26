(function () {
    'use strict';
    const instances = new Map();
    const PAGE_SIZE = 40;
    // Pure rendering helpers live in mailer-views.js (loaded before this file).
    const safeMailHTML = (...args) => window.MailerViews.safeMailHTML(...args);
    const displayName = value => window.MailerViews.displayName(value);
    const parseSender = value => window.MailerViews.parseSender(value);
    const formatDate = (value, timezone) => window.MailerViews.formatDate(value, timezone);
    const formatBytes = size => window.MailerViews.formatBytes(size);
    const avatar = (name, extraClass) => window.MailerViews.avatar(name, extraClass);
    const emptyState = (options) => window.MailerViews.emptyState(esc, options);
    const skeleton = rows => window.MailerViews.skeleton(rows);
    function esc(value) {
        return String(value == null ? '' : value).replace(/[&<>"']/g, ch => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' }[ch]));
    }
    function t(ctx, key, vars) {
        return ctx.t('mailer.' + key, vars);
    }
    function iconSVG(name) {
        const shapes = {
            archive: '<path d="M3 5h14v3H3zM4 8v8h12V8M8 11h4"/>',
            junk: '<path d="M5 17V3l11 2-3 4 3 4-11 2"/>',
            trash: '<path d="M4 6h12M8 6V4h4v2M6 6l1 11h6l1-11M9 9v5M11 9v5"/>',
            restore: '<path d="M4 8V4m0 4h4M4 8a6.5 6.5 0 1 1-1 5"/>',
            delete: '<path d="m5 5 10 10M15 5 5 15"/>',
            read: '<path d="M3 5h14v10H3zM3 6l7 5 7-5M7 3h8"/>',
            snooze: '<circle cx="10" cy="10" r="7"/><path d="M10 6v4l3 2"/>',
            search: '<circle cx="8.5" cy="8.5" r="5.5"/><path d="m13 13 4 4"/>',
            refresh: '<path d="M16 7V3l-2 2a7 7 0 1 0 2 9M16 3l-2 2"/>',
            compose: '<path d="M11 4H4v12h12V9M9 11l7-7 2 2-7 7-3 1z"/>',
            folder: '<path d="M2.5 5.5h6l1.5 2h7.5v8h-15z"/>',
            inbox: '<path d="M3 5h14v11H3zM3 11h4l1 2h4l1-2h4"/>',
            outbox: '<path d="M4 15h12V6H4zM8 9l2-2 2 2M10 7v6"/>',
            drafts: '<path d="M4 3h8l4 4v10H4zM12 3v4h4M7 11h6M7 14h4"/>',
            star: '<path d="m10 2 2.3 5 5.4.7-4 3.8 1 5.5-4.7-2.7-4.7 2.7 1-5.5-4-3.8L7.7 7z"/>',
            label: '<path d="M3 4h8l6 6-7 7-7-6zM7 8h.01"/>',
            reply: '<path d="M8 5 3 9l5 4M3 9h9a5 5 0 0 1 5 5v1"/>',
            replyAll: '<path d="M6 4 1 8l5 4M1 8h8a4 4 0 0 1 4 4v2M11 4l4 4-4 4"/>',
            forward: '<path d="m12 5 5 4-5 4M17 9H8a5 5 0 0 0-5 5v1"/>',
            print: '<path d="M6 8V3h8v5M6 8h8a2 2 0 0 1 2 2v4h-3M6 14H4v-4a2 2 0 0 1 2-2M7 12h6v5H7z"/>',
            download: '<path d="M10 3v9M6 9l4 4 4-4M4 16h12"/>',
            back: '<path d="M12 4 6 10l6 6M6 10h11"/>'
        };
        return `<svg class="vd-mailer-icon" viewBox="0 0 20 20" aria-hidden="true" focusable="false" fill="none" stroke="currentColor" stroke-width="1.75" stroke-linecap="round" stroke-linejoin="round">${shapes[name] || shapes.folder}</svg>`;
    }

    function apiRoot() { return '/api/desktop/mailer'; }

    function responseItems(value, key) {
        if (Array.isArray(value)) return value;
        if (value && Array.isArray(value.items)) return value.items;
        return value && Array.isArray(value[key]) ? value[key] : [];
    }

    function localPrefs() {
        try { return JSON.parse(localStorage.getItem('aurago.desktop.mailer.prefs') || '{}'); } catch (_) { return {}; }
    }

    function savePrefs(prefs) {
        try { localStorage.setItem('aurago.desktop.mailer.prefs', JSON.stringify(prefs)); } catch (_) {}
    }

    function render(host, windowId, ctx, launchContext) {
        dispose(windowId);
        if (launchContext && launchContext.compose) return window.MailerCompose.render(host, windowId, ctx, launchContext);
        const inst = {
            windowId, host, ctx, api: window.MailerAPI.create(ctx.api), state: null, folders: [], threads: [], drafts: [], outbox: [],
            view: localPrefs().view || 'inbox', accountId: localPrefs().accountId || '', folderId: '', query: '', nextCursor: '', outboxCursor: '', outboxHasMore: false, loadingMore: false,
            selected: new Set(), activeThread: null, activeThreadData: null, ruleRunsByMessage: Object.create(null), loadGeneration: 0, searchTimer: 0, remoteSearch: false,
            refreshTimer: 0, disposed: false, listeners: [], abort: null, remoteImages: new Set(), inlineImageURLs: new Set(), readerRenderGeneration: 0, readerAbort: null, selectedLabel: '',
            resizeState: null, settingsTab: 'accounts', records: {}, eventHandler: null, footerMessage: ''
        };
        inst.settings = window.MailerSettings.create(inst, {
            esc, t, responseItems, accountSettingsRecord, showError, setStatus, reloadState, refreshCurrent, localDateTime
        });
        instances.set(windowId, inst);
        host.innerHTML = `<section class="vd-mailer ${ctx.readonly ? 'is-readonly' : ''}" data-mailer-app>
            <header class="vd-mailer-topbar"><div class="vd-mailer-brand"><span class="vd-mailer-logo" aria-hidden="true">${ctx.iconMarkup ? ctx.iconMarkup('mail', '✉', 'vd-mailer-logo-icon', 24) : '✉'}</span><div class="vd-mailer-brand-text"><p class="vd-mailer-eyebrow">AuraGo Desktop</p><h1>Mailer</h1></div></div>
                <label class="vd-mailer-account-picker"><span>${esc(t(ctx, 'account'))}</span><select data-account-picker aria-label="${esc(t(ctx, 'account'))}"></select></label>
                <label class="vd-mailer-search"><span aria-hidden="true">${iconSVG('search')}</span><input type="search" data-search placeholder="${esc(t(ctx, 'search_placeholder'))}" aria-label="${esc(t(ctx, 'search'))}" autocomplete="off"><button type="button" data-clear-search aria-label="${esc(t(ctx, 'clear_search'))}" hidden>×</button></label>
                <div class="vd-mailer-top-actions"><button type="button" class="vd-mailer-button" data-provider-search hidden>${esc(t(ctx, 'search_provider'))}</button><button type="button" class="vd-mailer-button" data-refresh aria-label="${esc(t(ctx, 'refresh'))}" title="${esc(t(ctx, 'refresh'))}">${iconSVG('refresh')}</button><button type="button" class="vd-mailer-button" data-open-settings>${esc(t(ctx, 'settings'))}</button><button type="button" class="vd-mailer-button vd-mailer-primary" data-compose><span>${iconSVG('compose')}</span>${esc(t(ctx, 'compose'))}</button></div>
            </header>
            <div class="vd-mailer-readonly-banner" data-readonly-banner hidden role="status">${esc(t(ctx, 'readonly'))}</div>
            <div class="vd-mailer-workspace" data-mailer-workspace aria-busy="true">
                <nav class="vd-mailer-folders" data-folder-nav aria-label="${esc(t(ctx, 'folders'))}"></nav>
                <div class="vd-mailer-resizer" data-resizer="side" role="separator" aria-orientation="vertical" tabindex="0" aria-label="${esc(t(ctx, 'resize_folder_panel'))}"></div>
                <section class="vd-mailer-list-panel" aria-label="${esc(t(ctx, 'message_list'))}">
                <div class="vd-mailer-list-toolbar"><label class="vd-mailer-select-all"><input type="checkbox" data-select-all aria-label="${esc(t(ctx, 'select_all'))}" ${ctx.readonly ? 'disabled' : ''}><span data-selection-count>${esc(t(ctx, 'no_selection'))}</span></label>
                        <div class="vd-mailer-list-actions"><button type="button" data-action="archive" title="${esc(t(ctx, 'archive'))}" aria-label="${esc(t(ctx, 'archive'))}">${iconSVG('archive')}</button><button type="button" data-action="junk" title="${esc(t(ctx, 'junk'))}" aria-label="${esc(t(ctx, 'junk'))}">${iconSVG('junk')}</button><button type="button" data-action="trash" title="${esc(t(ctx, 'trash'))}" aria-label="${esc(t(ctx, 'trash'))}">${iconSVG('trash')}</button><button type="button" data-action="restore" title="${esc(t(ctx, 'restore'))}" aria-label="${esc(t(ctx, 'restore'))}">${iconSVG('restore')}</button><button type="button" data-action="delete" title="${esc(t(ctx, 'delete'))}" aria-label="${esc(t(ctx, 'delete'))}">${iconSVG('delete')}</button><button type="button" data-action="mark_read" title="${esc(t(ctx, 'mark_read'))}" aria-label="${esc(t(ctx, 'mark_read'))}">${iconSVG('read')}</button>
                            <select data-move-folder aria-label="${esc(t(ctx, 'move_to'))}"><option value="">${esc(t(ctx, 'move_to'))}</option></select><select data-label-select aria-label="${esc(t(ctx, 'add_label'))}"><option value="">${esc(t(ctx, 'add_label'))}</option></select><button type="button" data-action="snooze" title="${esc(t(ctx, 'snooze'))}" aria-label="${esc(t(ctx, 'snooze'))}">${iconSVG('snooze')}</button></div>
                    </div>
                    <div class="vd-mailer-items" data-thread-list aria-live="polite"></div>
                    <div class="vd-mailer-list-footer"><span data-search-notice></span><span data-list-count></span><button type="button" class="vd-mailer-button" data-load-more hidden>${esc(t(ctx, 'load_more'))}</button></div>
                </section>
                <div class="vd-mailer-resizer" data-resizer="list" role="separator" aria-orientation="vertical" tabindex="0" aria-label="${esc(t(ctx, 'resize_message_panel'))}"></div>
                <section class="vd-mailer-reader" data-reader aria-label="${esc(t(ctx, 'message_reader'))}"><div class="vd-mailer-empty-reader"><span class="vd-mailer-empty-badge" aria-hidden="true">✉</span><p>${esc(t(ctx, 'select_message'))}</p></div></section>
            </div>
            <div class="vd-mailer-status" data-status role="status" aria-live="polite"></div>
            <section class="vd-mailer-settings" data-settings hidden aria-label="${esc(t(ctx, 'settings'))}"></section>
        </section>`;
        inst.mailerRoot = host.querySelector('[data-mailer-app]');
        bind(inst);
        restoreSizes(inst);
        subscribe(inst);
        loadState(inst).catch(error => errorState(inst, error));
        inst.refreshTimer = setInterval(() => {
            if (!document.hidden && !inst.disposed && !host.closest('.vd-window')?.classList.contains('minimized')) refreshCurrent(inst, true);
        }, 90000);
        ctx.registerWindowCleanup?.(windowId, () => dispose(windowId));
        return inst;
    }

    function bind(inst) {
        const host = inst.host;
        const listen = (target, type, handler, options) => { target.addEventListener(type, handler, options); inst.listeners.push([target, type, handler, options]); };
        listen(host, 'input', event => {
            if (event.target.matches('[data-search]')) {
                inst.query = event.target.value.trim();
                host.querySelector('[data-clear-search]').hidden = !inst.query;
                syncProviderSearchButton(inst);
                clearTimeout(inst.searchTimer);
                inst.searchTimer = setTimeout(() => loadCurrent(inst), 260);
            }
        });
        listen(host, 'change', event => {
            if (event.target.matches('[data-account-picker]')) {
                inst.accountId = event.target.value;
                inst.remoteSearch = false;
                if (inst.view === 'folder') { inst.view = 'inbox'; inst.folderId = ''; }
                inst.activeThread = null;
                inst.selected.clear();
                savePrefs(Object.assign(localPrefs(), { accountId: inst.accountId }));
                loadFolders(inst).then(() => { paintFolders(inst); paintMoveLabels(inst); loadCurrent(inst); });
            }
            if (event.target.matches('[data-select-all]')) {
                const checked = event.target.checked;
                inst.threads.forEach(item => checked ? inst.selected.add(item.id) : inst.selected.delete(item.id));
                paintList(inst);
            }
            if (event.target.matches('[data-move-folder]') && event.target.value) runBulkAction(inst, 'move', { destination_folder_id: event.target.value });
            if (event.target.matches('[data-label-select]') && event.target.value) runBulkAction(inst, 'label_add', { label_id: event.target.value });
        });
        listen(host, 'click', event => {
            const view = event.target.closest('[data-view]');
            if (view) {
                inst.view = view.dataset.view;
                inst.remoteSearch = false;
                if (view.dataset.accountId) inst.accountId = view.dataset.accountId;
                else inst.accountId = '';
                if (inst.view === 'label') inst.selectedLabel = view.dataset.labelId || '';
                inst.folderId = ''; inst.selected.clear(); inst.activeThread = null; inst.activeThreadData = null;
                inst.mailerRoot.classList.remove('has-active-reader');
                const picker = host.querySelector('[data-account-picker]'); if (picker) picker.value = inst.accountId;
                savePrefs(Object.assign(localPrefs(), { accountId: inst.accountId, view: inst.view })); paintFolders(inst);
                loadFolders(inst).then(() => loadCurrent(inst)); return;
            }
            const folder = event.target.closest('[data-folder-id]');
            if (folder) { inst.view = 'folder'; inst.remoteSearch = false; inst.accountId = folder.dataset.accountId || inst.accountId; inst.folderId = folder.dataset.folderId; inst.activeThread = null; inst.activeThreadData = null; inst.selected.clear(); inst.mailerRoot.classList.remove('has-active-reader'); paintFolders(inst); loadFolders(inst).then(() => loadCurrent(inst)); return; }
            const toggle = event.target.closest('[data-select-thread]');
            if (toggle) { event.stopPropagation(); inst.selected.has(toggle.value) ? inst.selected.delete(toggle.value) : inst.selected.add(toggle.value); paintList(inst); return; }
            const star = event.target.closest('[data-star-thread]');
            if (star) { event.stopPropagation(); runOneAction(inst, star.dataset.starThread, star.getAttribute('aria-pressed') === 'true' ? 'unstar' : 'star'); return; }
            const item = event.target.closest('[data-thread-id]');
            if (item) { openThread(inst, item.dataset.threadId); return; }
            const action = event.target.closest('[data-action]');
            if (action) { runToolbarAction(inst, action.dataset.action); return; }
            if (event.target.closest('[data-refresh]')) { refreshCurrent(inst, false); return; }
            if (event.target.closest('[data-provider-search]')) { inst.remoteSearch = !inst.remoteSearch; syncProviderSearchButton(inst); loadCurrent(inst); return; }
            if (event.target.closest('[data-clear-search]')) { host.querySelector('[data-search]').value = ''; inst.query = ''; inst.remoteSearch = false; host.querySelector('[data-clear-search]').hidden = true; syncProviderSearchButton(inst); loadCurrent(inst); return; }
            if (event.target.closest('[data-compose]')) { openCompose(inst); return; }
            if (event.target.closest('[data-open-settings]')) { inst.settings.show(); return; }
            if (event.target.closest('[data-settings-close]')) { host.querySelector('[data-settings]').hidden = true; return; }
            const settingTab = event.target.closest('[data-setting-tab]');
            if (settingTab) { inst.settingsTab = settingTab.dataset.settingTab; inst.settings.paint(); return; }
            if (event.target.closest('[data-create-folder]')) { inst.settings.createFolder(event.target.closest('[data-create-folder]').dataset.createFolder); return; }
            if (event.target.closest('[data-cancel-account-form]')) { inst.settings.paint(); return; }
            if (event.target.closest('[data-load-more]')) { if ((inst.view === 'outbox' ? inst.outboxHasMore : inst.nextCursor) && !inst.loadingMore) loadCurrent(inst, true); return; }
            const remoteImages = event.target.closest('[data-load-images]');
            if (remoteImages) { inst.remoteImages.add(inst.activeThread); paintReader(inst); return; }
            if (event.target.closest('[data-reader-back]')) { clearInlineImages(inst); inst.activeThread = null; inst.activeThreadData = null; inst.mailerRoot.classList.remove('has-active-reader'); host.querySelector('[data-reader]').innerHTML = `<div class="vd-mailer-empty-reader"><span class="vd-mailer-empty-badge" aria-hidden="true">✉</span><p>${esc(t(inst.ctx, 'select_message'))}</p></div>`; return; }
            if (event.target.closest('[data-remove-label]')) { const button = event.target.closest('[data-remove-label]'); runOneAction(inst, inst.activeThread, 'label_remove', { label_id: button.dataset.removeLabel }); return; }
            if (event.target.closest('[data-reader-action]')) { const button = event.target.closest('[data-reader-action]'); readerAction(inst, button.dataset.readerAction, button); return; }
            if (event.target.closest('[data-eml]')) { const message = findMessage(inst, event.target.closest('[data-eml]').dataset.eml); if (message) window.open(inst.api.emlURL(message.id), '_blank', 'noopener'); return; }
            const retryRuleRuns = event.target.closest('[data-retry-rule-runs]');
            if (retryRuleRuns) { loadRuleRuns(inst, inst.activeThread, [findMessage(inst, retryRuleRuns.dataset.retryRuleRuns)].filter(Boolean)); return; }
            const retryRuleRun = event.target.closest('[data-retry-rule-run]');
            if (retryRuleRun) { retryMessageRule(inst, retryRuleRun.dataset.messageId, retryRuleRun.dataset.retryRuleRun, retryRuleRun.dataset.runStatus); return; }
            if (event.target.closest('[data-retry]')) { retryOutbox(inst, event.target.closest('[data-retry]').dataset.retry); return; }
            if (event.target.closest('[data-cancel-send]')) { cancelOutbox(inst, event.target.closest('[data-cancel-send]').dataset.cancelSend); return; }
            if (event.target.closest('[data-connect-source]')) { inst.settings.connectSource(event.target.closest('[data-connect-source]').dataset.connectSource); return; }
            if (event.target.closest('[data-disconnect-account]')) { inst.settings.disconnectAccount(event.target.closest('[data-disconnect-account]').dataset.disconnectAccount); return; }
            if (event.target.closest('[data-test-account]')) { inst.settings.testAccount(event.target.closest('[data-test-account]').dataset.testAccount, event.target.closest('[data-test-account]').dataset.direction); return; }
            if (event.target.closest('[data-add-account]')) { inst.settings.paintAccountForm(event.target.closest('[data-add-account]').dataset.type); return; }
            if (event.target.closest('[data-save-account]')) { inst.settings.saveAccountForm(); return; }
            if (event.target.closest('[data-enable-app]')) { inst.settings.toggleEnabled(event.target.closest('[data-enable-app]').checked); return; }
            if (event.target.closest('[data-save-record]')) { inst.settings.saveRecord(); return; }
            if (event.target.closest('[data-delete-record]')) { inst.settings.deleteRecord(event.target.closest('[data-delete-record]').dataset.deleteRecord); return; }
            if (event.target.closest('[data-create-record]')) { inst.settings.editRecord(event.target.closest('[data-create-record]').dataset.createRecord); return; }
            if (event.target.closest('[data-edit-record]')) { inst.settings.editRecord(event.target.closest('[data-edit-record]').dataset.editRecord); return; }
            if (event.target.closest('[data-edit-draft]')) {
                const draft = inst.drafts.find(item => item.id === event.target.closest('[data-edit-draft]').dataset.editDraft);
                if (draft) openCompose(inst, { draft, accountId: draft.account_id });
                return;
            }
            if (event.target.closest('[data-rule-preview]')) { inst.settings.previewRule(); return; }
            if (event.target.closest('[data-rule-apply]')) { inst.settings.applyRule(); return; }
            if (event.target.closest('[data-rule-enabled]')) { inst.settings.toggleRule(event.target.closest('[data-rule-enabled]').dataset.ruleEnabled, event.target.closest('[data-rule-enabled]').checked); return; }
            if (event.target.closest('[data-snooze-submit]')) { submitSnooze(inst); return; }
            if (event.target.closest('[data-close-snooze]')) { closeSnooze(inst); return; }
        });
        listen(host, 'keydown', event => {
            const separator = event.target.closest('[data-resizer]');
            if (separator && ['ArrowLeft', 'ArrowRight'].includes(event.key)) {
                event.preventDefault(); resizeKeyboard(inst, separator.dataset.resizer, event.key === 'ArrowRight' ? 24 : -24);
            }
            if ((event.ctrlKey || event.metaKey) && event.key.toLowerCase() === 'f') { event.preventDefault(); host.querySelector('[data-search]').focus(); }
            if (event.key === 'Escape' && !host.querySelector('[data-settings]').hidden) host.querySelector('[data-settings]').hidden = true;
            const row = event.target.closest('[data-thread-id]');
            if (row && ['Enter', ' '].includes(event.key) && !event.target.closest('button,input,a,select')) { event.preventDefault(); openThread(inst, row.dataset.threadId); }
        });
        for (const handle of host.querySelectorAll('[data-resizer]')) {
            listen(handle, 'pointerdown', event => startResize(inst, event, handle.dataset.resizer));
        }
    }

    async function loadState(inst) {
        const response = await inst.api.state();
        if (inst.disposed) return;
        inst.state = response;
        inst.state.accounts = response.accounts || [];
        inst.state.sources = response.sources || [];
        inst.state.account_settings = response.account_settings || {};
        for (const account of inst.state.accounts) Object.assign(account, accountSettingsRecord(inst.state, account.id));
        inst.state.folders = response.folders || [];
        inst.state.labels = normalizeLabels(response.labels || inst.state.folders.filter(folder => folder.kind === 'label'));
        inst.state.readonly = !!(inst.ctx.readonly || response.readonly);
        inst.host.querySelector('[data-compose]').disabled = inst.state.readonly;
        inst.host.querySelector('[data-readonly-banner]').hidden = !inst.state.readonly;
        paintAccountPicker(inst);
        paintFolders(inst);
        paintMoveLabels(inst);
        await loadFolders(inst);
        paintFolders(inst);
        await loadCurrent(inst);
    }

    function paintAccountPicker(inst) {
        const select = inst.host.querySelector('[data-account-picker]');
        const accounts = inst.state.accounts || [];
        if (inst.accountId && !accounts.some(account => account.id === inst.accountId)) inst.accountId = '';
        select.innerHTML = `<option value="">${esc(t(inst.ctx, 'all_accounts'))}</option>` + accounts.map(account => `<option value="${esc(account.id)}" ${account.id === inst.accountId ? 'selected' : ''}>${esc(account.name || account.address)}</option>`).join('');
    }

    async function loadFolders(inst) {
        if (!inst.state) return;
        if (inst.accountId && !inst.state.folders.some(folder => folder.account_id === inst.accountId)) {
            try { inst.state.folders = responseItems(await inst.api.request('/accounts/' + encodeURIComponent(inst.accountId) + '/folders'), 'folders'); }
            catch (_) { inst.state.folders = []; }
        }
        inst.folders = inst.state.folders.filter(folder => !inst.accountId || folder.account_id === inst.accountId);
    }

    function normalizeLabels(labels) {
        return (labels || []).map(label => Object.assign({}, label, {
            folder_id: label.folder_id || label.id,
            id: label.provider_id || label.id
        }));
    }

    function paintFolders(inst) {
        const root = inst.host.querySelector('[data-folder-nav]');
        const count = kind => {
            const found = inst.folders.find(folder => folder.kind === kind);
            return found && Number(found.unread_count) ? `<span>${Number(found.unread_count)}</span>` : '';
        };
        const inboxCount = accountId => {
            const found = inst.folders.find(folder => folder.account_id === accountId && folder.kind === 'inbox');
            return found && Number(found.unread_count) ? `<small>${Number(found.unread_count)}</small>` : '';
        };
        const views = [
            ['inbox', 'inbox', count('inbox')], ['unread', 'unread', ''], ['starred', 'starred', ''], ['snoozed', 'snoozed', ''],
            ['drafts', 'drafts', String(inst.drafts.length || '')], ['outbox', 'outbox', String(inst.outbox.filter(item => ['scheduled', 'sending', 'unknown', 'failed'].includes(item.status)).length || '')]
        ];
        root.innerHTML = `<div class="vd-mailer-folder-group"><p>${esc(t(inst.ctx, 'mailboxes'))}</p>${views.map(([id, label, badge]) => `<button type="button" data-view="${id}" class="vd-mailer-folder ${inst.view === id ? 'is-active' : ''}" aria-current="${inst.view === id ? 'page' : 'false'}"><span class="vd-mailer-folder-icon" aria-hidden="true">${folderGlyph(id)}</span><span>${esc(t(inst.ctx, label))}</span>${badge ? `<small>${esc(badge)}</small>` : ''}</button>`).join('')}</div>
            <div class="vd-mailer-folder-group"><p>${esc(t(inst.ctx, 'accounts'))}</p>${(inst.state.accounts || []).map(account => `<div class="vd-mailer-account-folder-set"><p class="vd-mailer-account-label">${esc(account.name || account.address)}</p><button type="button" data-view="inbox" data-account-id="${esc(account.id)}" class="vd-mailer-folder ${account.id === inst.accountId && inst.view === 'inbox' ? 'is-active' : ''}"><span class="vd-mailer-folder-icon" aria-hidden="true">${folderGlyph('inbox')}</span><span>${esc(t(inst.ctx, 'inbox'))}</span>${inboxCount(account.id)}</button>${inst.folders.filter(folder => folder.account_id === account.id && folder.kind !== 'inbox').map(folder => `<button type="button" data-folder-id="${esc(folder.id)}" data-account-id="${esc(folder.account_id)}" class="vd-mailer-folder vd-mailer-subfolder ${inst.view === 'folder' && folder.id === inst.folderId ? 'is-active' : ''}"><span class="vd-mailer-folder-icon" aria-hidden="true">${folderGlyph(folder.kind)}</span><span>${esc(folder.name)}</span>${Number(folder.unread_count) ? `<small>${Number(folder.unread_count)}</small>` : ''}</button>`).join('')}${account.capabilities?.includes('folders_create') && !inst.state.readonly && !account.read_only ? `<button type="button" class="vd-mailer-new-folder" data-create-folder="${esc(account.id)}">＋ ${esc(t(inst.ctx, 'new_folder'))}</button>` : ''}</div>`).join('')}</div>
            ${labelsForAccount(inst).length ? `<div class="vd-mailer-folder-group"><p>${esc(t(inst.ctx, 'labels'))}</p>${labelsForAccount(inst).map(label => `<button type="button" data-folder-id="${esc(label.folder_id)}" data-account-id="${esc(label.account_id)}" class="vd-mailer-folder vd-mailer-subfolder"><span class="vd-mailer-label-dot" style="--label-color:${safeColor(label.color)}"></span><span>${esc(label.name || label.id)}</span></button>`).join('')}</div>` : ''}`;
    }

    function folderGlyph(kind) {
        return iconSVG(({ inbox: 'inbox', sent: 'outbox', drafts: 'drafts', archive: 'archive', trash: 'trash', junk: 'junk', spam: 'junk', snoozed: 'snooze', label: 'label' })[kind] || 'folder');
    }

    function safeColor(value) { return /^#[0-9a-f]{3,8}$/i.test(String(value || '')) ? value : '#8d9bb0'; }

    function labelsForAccount(inst) {
        return inst.accountId ? (inst.state.labels || []).filter(label => String(label.account_id) === String(inst.accountId)) : [];
    }

    function paintMoveLabels(inst) {
        const move = inst.host.querySelector('[data-move-folder]');
        move.innerHTML = `<option value="">${esc(t(inst.ctx, 'move_to'))}</option>` + inst.folders.map(folder => `<option value="${esc(folder.id)}">${esc(folder.name)}</option>`).join('');
        const labels = inst.host.querySelector('[data-label-select]');
        const available = labelsForAccount(inst);
        labels.disabled = !inst.accountId || inst.state.readonly;
        labels.title = inst.accountId ? '' : t(inst.ctx, 'select_account');
        labels.innerHTML = `<option value="">${esc(t(inst.ctx, 'add_label'))}</option>` + available.map(label => `<option value="${esc(label.id || label.name)}">${esc(label.name || label.id)}</option>`).join('');
    }

    function queryFor(inst, append) {
        const params = { limit: PAGE_SIZE, query: inst.query };
        if (append && inst.nextCursor) params.cursor = inst.nextCursor;
        if (inst.remoteSearch) { delete params.query; params.q = inst.query; params.remote = 'true'; }
        if (inst.accountId) params.account_id = inst.accountId;
        if (inst.view === 'folder') params.folder_id = inst.folderId;
        else if (['inbox', 'unread', 'starred', 'snoozed'].includes(inst.view)) params.view = inst.view;
        else if (inst.view === 'label') params.folder_id = inst.selectedLabel;
        if (inst.remoteSearch) {
            const folderId = providerSearchFolder(inst);
            if (folderId) { params.folder_id = folderId; delete params.view; }
        }
        return params;
    }

    function providerSearchFolder(inst) {
        if (!inst.accountId) return '';
        if (inst.view === 'folder' && inst.folderId) return inst.folderId;
        if (inst.view !== 'inbox') return '';
        const inbox = (inst.folders || []).find(folder => folder.kind === 'inbox' && (!folder.account_id || folder.account_id === inst.accountId));
        return inbox ? inbox.id : '';
    }

    function providerSearchEligible(inst) {
        const account = inst.state?.accounts?.find(item => item.id === inst.accountId);
        if (!inst.query || !inst.accountId || !providerSearchFolder(inst) || !account || (inst.view !== 'folder' && inst.view !== 'inbox')) return false;
        const source = String(account.source || '').toLowerCase();
        return account.protocol === 'imap' || ['gog', 'google', 'google_workspace', 'gmail'].includes(source) || (account.capabilities || []).includes('search_remote');
    }

    function syncProviderSearchButton(inst) {
        const button = inst.host.querySelector('[data-provider-search]');
        if (!button) return;
        button.hidden = !providerSearchEligible(inst);
        button.setAttribute('aria-pressed', inst.remoteSearch ? 'true' : 'false');
        button.disabled = !!inst.loadingRemote;
    }

    function updateSearchNotice(inst, result) {
        const notice = inst.host.querySelector('[data-search-notice]');
        if (!notice) return;
        if (!inst.query) { notice.textContent = ''; return; }
        if (result?.search_scope === 'provider') {
            notice.textContent = result.partial ? t(inst.ctx, 'provider_search_partial') : t(inst.ctx, 'provider_search_notice');
        } else notice.textContent = t(inst.ctx, 'local_search_notice');
    }

    async function loadCurrent(inst, append) {
        if (!inst.state || inst.disposed) return;
        if (inst.abort) inst.abort.abort();
        const controller = new AbortController();
        inst.abort = controller;
        const generation = ++inst.loadGeneration;
        const list = inst.host.querySelector('[data-thread-list]');
        inst.loadingRemote = inst.remoteSearch;
        inst.loadingMore = !!append;
        if (!append) inst.nextCursor = '';
        syncProviderSearchButton(inst);
        const more = inst.host.querySelector('[data-load-more]');
        if (more) more.disabled = !!append;
        if (!append) list.innerHTML = skeleton(6);
        if (!append && inst.view === 'outbox') { inst.outbox = []; inst.outboxCursor = ''; inst.outboxHasMore = false; }
        inst.host.querySelector('[data-mailer-workspace]').setAttribute('aria-busy', 'true');
        try {
            if (['drafts', 'outbox'].includes(inst.view)) {
                const result = inst.view === 'drafts' ? await inst.api.drafts() : await inst.api.outbox(Object.assign({ limit: 50 }, append && inst.outboxCursor ? { cursor: inst.outboxCursor } : {}));
                if (generation !== inst.loadGeneration || inst.disposed) return;
                if (inst.view === 'drafts') inst.drafts = responseItems(result, 'drafts'); else {
                    const rows = responseItems(result, 'items');
                    inst.outbox = append ? inst.outbox.concat(rows) : rows;
                    inst.outboxCursor = String(result?.next_cursor || '');
                    inst.outboxHasMore = !!result?.has_more && !!inst.outboxCursor;
                }
                paintFolders(inst); paintSpecialList(inst);
                return;
            }
            const result = await inst.api.threads(Object.assign(queryFor(inst, append), { signal: controller.signal }));
            if (generation !== inst.loadGeneration || inst.disposed) return;
            updateSearchNotice(inst, result);
            const rows = responseItems(result, 'items');
            inst.threads = append ? inst.threads.concat(rows) : rows;
            inst.nextCursor = String(result?.next_cursor || '');
            paintList(inst);
            if (inst.activeThread && !append) await loadOpenThread(inst, inst.activeThread, false);
        } catch (error) {
            if (error.name !== 'AbortError' && generation === inst.loadGeneration) {
                updateSearchNotice(inst, null);
                list.innerHTML = emptyState({ icon: '!', title: t(inst.ctx, 'load_failed'), action: 'refresh', actionLabel: t(inst.ctx, 'retry') });
            }
        } finally {
            inst.loadingRemote = false;
            inst.loadingMore = false;
            syncProviderSearchButton(inst);
            if (more) more.disabled = inst.view === 'outbox' ? !inst.outboxHasMore || inst.loadingMore : !inst.nextCursor || inst.loadingMore;
            if (generation === inst.loadGeneration) inst.host.querySelector('[data-mailer-workspace]').setAttribute('aria-busy', 'false');
        }
    }

    function paintList(inst) {
        const root = inst.host.querySelector('[data-thread-list]');
        const items = inst.threads;
        const selectAll = inst.host.querySelector('[data-select-all]');
        selectAll.disabled = inst.state.readonly;
        selectAll.checked = items.length > 0 && items.every(item => inst.selected.has(item.id));
        inst.host.querySelector('[data-selection-count]').textContent = inst.selected.size ? t(inst.ctx, 'selected_count', { count: inst.selected.size }) : t(inst.ctx, 'no_selection');
        const move = inst.host.querySelector('[data-move-folder]');
        const label = inst.host.querySelector('[data-label-select]');
        const actionCaps = { archive: 'archive', junk: 'junk', trash: 'trash', restore: 'restore', delete: 'delete', mark_read: 'read_state' };
        inst.host.querySelectorAll('[data-action]').forEach(button => {
            const capability = actionCaps[button.dataset.action];
            button.disabled = capability ? !selectedHasCapability(inst, capability) : !hasMutableSelection(inst);
        });
        move.disabled = !selectedHasCapability(inst, 'move');
        label.disabled = !selectedHasCapability(inst, 'labels_write');
        root.innerHTML = items.length ? items.map(item => {
            const account = accountForThread(inst, item);
            const accountName = !inst.accountId && account ? (account.name || account.address) : '';
            const senderName = threadIdentity(inst, item) || t(inst.ctx, 'unknown_sender');
            return `<article class="vd-mailer-thread ${(item.unread_count || item.unread) ? 'is-unread' : ''} ${item.id === inst.activeThread ? 'is-selected' : ''}" data-thread-id="${esc(item.id)}" tabindex="0" role="button" aria-label="${esc(t(inst.ctx, 'open_message_label', { subject: item.subject || t(inst.ctx, 'no_subject'), from: senderName }))}" aria-current="${item.id === inst.activeThread ? 'true' : 'false'}">
            <label class="vd-mailer-thread-check" aria-label="${esc(t(inst.ctx, 'select_message'))}"><input type="checkbox" data-select-thread value="${esc(item.id)}" ${inst.selected.has(item.id) ? 'checked' : ''}></label>
            <button type="button" class="vd-mailer-star" data-star-thread="${esc(item.id)}" aria-label="${esc(item.starred ? t(inst.ctx, 'unstar') : t(inst.ctx, 'star'))}" aria-pressed="${item.starred ? 'true' : 'false'}" ${!canThreadCapability(inst, item, 'star') ? 'disabled' : ''}>${iconSVG('star')}</button>
            ${avatar(senderName, 'vd-mailer-avatar--row')}
            <div class="vd-mailer-thread-copy"><div class="vd-mailer-thread-line"><b title="${esc(senderName)}">${esc(senderName)}</b>${accountName ? `<small class="vd-mailer-thread-account">${esc(accountName)}</small>` : ''}<time>${esc(formatDate(item.last_message_at || item.date, inst.state.timezone))}</time></div><strong>${esc(item.subject || t(inst.ctx, 'no_subject'))}</strong><p>${esc(item.preview || item.snippet || '')}</p>${item.attachment_count ? `<span class="vd-mailer-attachment-count">📎 ${Number(item.attachment_count)}</span>` : ''}</div>
            </article>`;
        }).join('') : emptyState({ icon: inst.view === 'inbox' ? '✉' : '⌕', title: t(inst.ctx, 'empty_title'), hint: t(inst.ctx, 'empty_view_' + inst.view) });
        inst.host.querySelector('[data-list-count]').textContent = t(inst.ctx, 'messages_count', { count: items.length });
        const more = inst.host.querySelector('[data-load-more]');
        more.hidden = !inst.nextCursor;
        more.disabled = !inst.nextCursor || inst.loadingMore;
        if (!items.length && !inst.activeThread) inst.host.querySelector('[data-reader]').innerHTML = `<div class="vd-mailer-empty-reader"><span class="vd-mailer-empty-badge" aria-hidden="true">✉</span><p>${esc(t(inst.ctx, 'select_message'))}</p></div>`;
    }

    function paintSpecialList(inst) {
        const root = inst.host.querySelector('[data-thread-list]');
        if (inst.view === 'drafts') {
            root.innerHTML = inst.drafts.length ? inst.drafts.map(draft => `<article class="vd-mailer-thread vd-mailer-draft-item" data-draft-id="${esc(draft.id)}"><button type="button" data-edit-draft="${esc(draft.id)}"><b>${esc((draft.to || []).join(', ') || t(inst.ctx, 'no_recipients'))}</b><strong>${esc(draft.subject || t(inst.ctx, 'no_subject'))}</strong><p>${esc(draft.text || '')}</p></button></article>`).join('') : emptyHTML(inst, 'empty_view_drafts');
        } else {
            root.innerHTML = inst.outbox.length ? inst.outbox.map(item => `<article class="vd-mailer-thread"><div class="vd-mailer-outbox-item"><b>${esc(item.status === 'unknown' ? t(inst.ctx, 'outbox_unknown') : t(inst.ctx, 'outbox_' + item.status))}</b><time>${esc(formatDate(item.updated_at || item.created_at, inst.state.timezone))}</time>${outboxDiagnostics(inst, item)}${item.status === 'failed' ? `<button type="button" class="vd-mailer-button" data-retry="${esc(item.id)}">${esc(t(inst.ctx, 'retry_send'))}</button>` : ''}${['scheduled', 'sending'].includes(item.status) ? `<button type="button" class="vd-mailer-button" data-cancel-send="${esc(item.id)}">${esc(t(inst.ctx, 'cancel_send'))}</button>` : ''}</div></article>`).join('') : emptyHTML(inst, 'empty_view_outbox');
        }
        inst.host.querySelector('[data-selection-count]').textContent = t(inst.ctx, 'no_selection');
        inst.host.querySelector('[data-select-all]').disabled = true;
        inst.host.querySelectorAll('[data-action], [data-move-folder], [data-label-select]').forEach(button => button.disabled = true);
        inst.host.querySelector('[data-list-count]').textContent = t(inst.ctx, 'messages_count', { count: inst.view === 'drafts' ? inst.drafts.length : inst.outbox.length });
        const more = inst.host.querySelector('[data-load-more]');
        more.hidden = inst.view !== 'outbox' || !inst.outboxHasMore;
        more.disabled = inst.view !== 'outbox' || !inst.outboxHasMore || inst.loadingMore;
    }

    function outboxDiagnostics(inst, item) {
        const error = item.error || item.error_code;
        const sendDetail = error ? t(inst.ctx, 'outbox_error_detail', { code: String(error) }) : item.remote_id || '';
        const appendDetail = item.append_error ? `${t(inst.ctx, 'action_failed')}: ${String(item.append_error)}` : '';
        return [sendDetail, appendDetail].filter(Boolean).map(detail => `<p>${esc(detail)}</p>`).join('');
    }

    function emptyHTML(inst, key) { return emptyState({ icon: '✉', title: t(inst.ctx, 'empty_title'), hint: t(inst.ctx, key) }); }

    async function openThread(inst, threadId) {
        clearInlineImages(inst);
        inst.activeThread = threadId;
        inst.remoteImages.delete(threadId);
        inst.mailerRoot.classList.add('has-active-reader');
        inst.host.querySelector('[data-reader]').innerHTML = `<div class="vd-mailer-reader-loading" role="status">${esc(t(inst.ctx, 'loading_message'))}</div>`;
        inst.host.querySelectorAll('[data-thread-id]').forEach(node => node.classList.toggle('is-selected', node.dataset.threadId === threadId));
        await loadOpenThread(inst, threadId, true);
    }

    async function loadOpenThread(inst, threadId, markRead) {
        const current = inst.loadGeneration;
        try {
            const listThread = inst.threads.find(item => item.id === threadId) || {};
            let readConfirmed = false;
            if (markRead && canThreadCapability(inst, listThread, 'read_state')) {
                try { await inst.api.read(threadId, true); readConfirmed = true; }
                catch (_) { setStatus(inst, t(inst.ctx, 'action_failed'), true); }
            }
            const thread = await inst.api.thread(threadId);
            if (inst.disposed || inst.activeThread !== threadId) return;
            inst.activeThreadData = Object.assign({ id: threadId }, thread);
            const item = inst.threads.find(row => row.id === threadId);
            if (item && readConfirmed) { item.unread_count = 0; item.unread = false; }
            paintReader(inst);
            loadRuleRuns(inst, threadId, thread.messages || []);
            if (markRead) paintList(inst);
        } catch (_) {
            if (current === inst.loadGeneration) inst.host.querySelector('[data-reader]').innerHTML = `<div class="vd-mailer-empty"><span class="vd-mailer-empty-badge" aria-hidden="true">!</span><b>${esc(t(inst.ctx, 'message_load_failed'))}</b><button type="button" class="vd-mailer-button" data-reader-action="reload">${esc(t(inst.ctx, 'retry'))}</button></div>`;
        }
    }

    // readerActionButton renders one action of the reader toolbar. Response
    // actions keep their label, utility and file actions collapse into an
    // icon with a tooltip, so the reader header never becomes a wall of
    // full-text buttons.
    function readerActionButton(inst, action, labelKey, glyph, iconOnly, primary, danger) {
        const label = t(inst.ctx, labelKey);
        const classes = ['vd-mailer-reader-action'];
        if (iconOnly) classes.push('vd-mailer-reader-action--icon');
        if (primary) classes.push('vd-mailer-primary');
        if (danger) classes.push('vd-mailer-danger');
        const attributes = iconOnly ? ` title="${esc(label)}" aria-label="${esc(label)}"` : '';
        return `<button type="button" class="${classes.join(' ')}" data-reader-action="${action}"${attributes}>${glyph}${iconOnly ? '' : `<span>${esc(label)}</span>`}</button>`;
    }

    function paintReader(inst) {
        const thread = inst.activeThreadData;
        const root = inst.host.querySelector('[data-reader]');
        if (!thread) return;
        const messages = thread.messages || [];
        const first = messages[0] || {};
        const current = messages[messages.length - 1] || first;
        const trashView = inst.view === 'folder' && inst.folders.find(folder => folder.id === inst.folderId)?.kind === 'trash';
        const canSend = canThreadCapability(inst, thread, 'send');
        const canArchive = canThreadCapability(inst, thread, 'archive');
        const canTrash = canThreadCapability(inst, thread, 'trash');
        const canRestore = canThreadCapability(inst, thread, 'restore');
        const canLabel = canThreadCapability(inst, thread, 'labels_write');
        const folderAction = trashView
            ? (canRestore ? readerActionButton(inst, 'restore', 'restore', iconSVG('restore'), true) : '')
            : (canArchive ? readerActionButton(inst, 'archive', 'archive', iconSVG('archive'), true) : '');
        const responseGroup = canSend
            ? `<span class="vd-mailer-reader-action-group">${readerActionButton(inst, 'reply', 'reply', iconSVG('reply'), false, true)}${readerActionButton(inst, 'reply_all', 'reply_all', iconSVG('replyAll'), false)}${readerActionButton(inst, 'forward', 'forward', iconSVG('forward'), false)}</span>`
            : '';
        const utilityGroup = `<span class="vd-mailer-reader-action-group">${readerActionButton(inst, 'print', 'print', iconSVG('print'), true)}${readerActionButton(inst, 'eml', 'download_eml', iconSVG('download'), true)}</span>`;
        const fileGroup = folderAction || canTrash || canDeleteThread(inst, thread)
            ? `<span class="vd-mailer-reader-action-group vd-mailer-reader-action-group--files">${folderAction}${canTrash ? readerActionButton(inst, 'trash', 'trash', iconSVG('trash'), true) : ''}${canDeleteThread(inst, thread) ? readerActionButton(inst, 'delete', 'delete', iconSVG('delete'), true, false, true) : ''}</span>`
            : '';
        const toolbar = `<div class="vd-mailer-reader-actions"><button type="button" class="vd-mailer-reader-back" data-reader-back>${iconSVG('back')}<span>${esc(t(inst.ctx, 'back_to_list'))}</span></button>${responseGroup}${utilityGroup}${fileGroup}</div>`;
        const threadLabels = thread.labels || current.labels || [];
        const knownLabelIDs = new Set((inst.state.labels || []).map(label => String(label.id)));
        const visibleLabels = threadLabels.filter(label => knownLabelIDs.has(String(typeof label === 'string' ? label : label.id)));
        // The reader has room for the thread roster, the list row does not.
        const participantCount = (thread.participants || []).filter(Boolean).length;
        const participantChip = participantCount > 2 ? `<span class="vd-mailer-participant-chip">${esc(t(inst.ctx, 'participants_count', { count: participantCount }))}</span>` : '';
        root.innerHTML = `<header class="vd-mailer-reader-head"><div class="vd-mailer-reader-subject"><h2>${esc(thread.subject || t(inst.ctx, 'no_subject'))}</h2><p>${participantChip}${visibleLabels.map(label => { const id = typeof label === 'string' ? label : label.id; const known = inst.state.labels.find(item => String(item.id) === String(id)); return `<span class="vd-mailer-label-chip">${esc(known?.name || id)}${canLabel ? `<button type="button" data-remove-label="${esc(id)}" aria-label="${esc(t(inst.ctx, 'remove_label'))}">×</button>` : ''}</span>`; }).join('')}</p></div>${toolbar}</header>
            <div class="vd-mailer-message-stack">${messages.map((message, index) => renderMessage(inst, message, index, messages.length)).join('')}</div>`;
        paintMessageFrames(inst);
    }

    function accountForThread(inst, thread) { return (inst.state.accounts || []).find(account => account.id === thread.account_id); }
    // threadIdentity resolves the person a list row belongs to. Received
    // mail shows the sender, sent mail the recipient, so the avatar letters
    // and the row label always name the other party. The participant list
    // of a thread mixes recipients and the sender and therefore never
    // describes one person.
    function threadIdentity(inst, item) {
        const folder = (inst.folders || []).find(entry => entry.id === item.folder_id);
        const recipients = (item.to || []).filter(Boolean);
        if (folder && folder.kind === 'sent' && recipients.length) return displayName(recipients[0]);
        const participants = (item.participants || []).filter(Boolean);
        return displayName(item.from || participants[participants.length - 1] || '');
    }
    function threadReadOnly(inst, thread) { return !!accountForThread(inst, thread)?.read_only; }
    function canThreadCapability(inst, thread, capability) {
        const account = accountForThread(inst, thread);
        return !!account && !inst.state.readonly && !account.disabled && !account.read_only && (account.capabilities || []).includes(capability);
    }
    function hasMutableSelection(inst) {
        const selected = inst.threads.filter(item => inst.selected.has(item.id));
        return selected.length > 0 && !inst.state.readonly && selected.every(item => !threadReadOnly(inst, item));
    }
    function selectedHasCapability(inst, capability) {
        const selected = inst.threads.filter(item => inst.selected.has(item.id));
        return selected.length > 0 && selected.every(item => canThreadCapability(inst, item, capability));
    }
    function canDeleteThread(inst, thread) { return canThreadCapability(inst, thread, 'delete'); }

    function renderMessage(inst, message, index, total) {
        const remoteEnabled = inst.remoteImages.has(inst.activeThread);
        const attachments = message.attachments || [];
        const hasRemoteImages = !!message.html && messageHasRemoteImages(message.html);
        const readErrorKey = message.read_error === 'message_unavailable' || message.read_error === 'message_unparsed' ? message.read_error : (message.read_error ? 'message_unavailable' : '');
        const notices = `${readErrorKey ? `<p class="vd-mailer-message-notice">${esc(t(inst.ctx, readErrorKey))}</p>` : ''}${message.html_skipped ? `<p class="vd-mailer-message-notice">${esc(t(inst.ctx, 'html_skipped'))}</p>` : ''}`;
        const body = message.html ? `<div class="vd-mailer-html-wrap"><iframe sandbox="allow-popups allow-popups-to-escape-sandbox" referrerpolicy="no-referrer" title="${esc(t(inst.ctx, 'email_content'))}" data-email-frame="${esc(message.id)}"></iframe></div>` : `<pre class="vd-mailer-plain-body">${esc(message.text || '')}</pre>`;
        const sender = parseSender(message.from || t(inst.ctx, 'unknown_sender'));
        const recipientLine = [message.to, message.cc?.length ? 'Cc: ' + message.cc.join(', ') : ''].flat().filter(Boolean).join(' · ');
        return `<article class="vd-mailer-message"><header class="vd-mailer-message-head">${avatar(sender.name, 'vd-mailer-avatar--message')}<div class="vd-mailer-message-meta"><b>${esc(sender.name)}</b>${sender.address ? `<details class="vd-mailer-sender-address"><summary>${esc(t(inst.ctx, 'email_address'))}</summary><span>${esc(sender.address)}</span></details>` : ''}<span>${esc(recipientLine)}</span></div><time>${esc(formatDate(message.date, inst.state.timezone))}</time>${canThreadCapability(inst, inst.activeThreadData || {}, 'star') ? `<button type="button" class="vd-mailer-reader-star" data-reader-action="${message.starred ? 'unstar' : 'star'}" data-message-id="${esc(message.id)}" aria-label="${esc(t(inst.ctx, message.starred ? 'unstar' : 'star'))}" aria-pressed="${message.starred ? 'true' : 'false'}">${message.starred ? '★' : '☆'}</button>` : ''}</header>
            ${hasRemoteImages ? `<div class="vd-mailer-remote-warning"><span>${esc(t(inst.ctx, remoteEnabled ? 'remote_images_loaded' : 'remote_images_blocked'))}</span>${remoteEnabled ? '' : `<button type="button" data-load-images>${esc(t(inst.ctx, 'load_remote_images'))}</button>`}</div>` : ''}
            ${notices}
            <div class="vd-mailer-message-body">${body}</div>
            ${attachments.length ? `<ul class="vd-mailer-message-attachments" aria-label="${esc(t(inst.ctx, 'attachments'))}">${attachments.map(item => `<li><a href="${esc(inst.api.attachmentURL(message.id, item.id))}" download="${esc(item.filename)}">📎 ${esc(item.filename)} <small>${formatBytes(item.size)}</small></a></li>`).join('')}</ul>` : ''}
            <div class="vd-mailer-rule-runs" data-rule-runs-for="${esc(message.id)}"></div>
            <footer><button type="button" data-eml="${esc(message.id)}">${iconSVG('download')}<span>${esc(t(inst.ctx, 'download_message_eml'))}</span></button>${index + 1 < total ? `<span>${esc(t(inst.ctx, 'quoted_message'))}</span>` : ''}</footer></article>`;
    }

    function paintRuleRunsForMessage(inst, messageId) {
        const root = inst.host.querySelector(`[data-rule-runs-for="${cssEscape(messageId)}"]`);
        if (!root) return;
        const state = inst.ruleRunsByMessage[String(messageId)] || {};
        if (state.loading) {
            root.innerHTML = `<p role="status">${esc(t(inst.ctx, 'rule_runs_loading'))}</p>`;
            return;
        }
        if (state.error) {
            root.innerHTML = `<p>${esc(t(inst.ctx, 'rule_runs_unavailable'))}</p><button type="button" class="vd-mailer-button" data-retry-rule-runs="${esc(messageId)}">${esc(t(inst.ctx, 'retry'))}</button>`;
            return;
        }
        const items = state.items || [];
        if (!items.length) { root.innerHTML = ''; return; }
        root.innerHTML = `<section aria-label="${esc(t(inst.ctx, 'rule_runs_title'))}"><h3>${esc(t(inst.ctx, 'rule_runs_title'))}</h3><ul>${items.map(run => {
            const runStatus = String(run.status);
            const statusKey = ({ pending: 'rule_run_pending', executing: 'rule_run_executing', unknown: 'rule_run_unknown', failed: 'rule_run_failed', completed: 'rule_run_completed', skipped: 'rule_run_skipped' })[runStatus] || 'rule_run_unknown';
            const actionCount = Number(run.action_count) || 0;
            const actionNumber = runStatus === 'completed' ? actionCount : Math.min(actionCount || Number.MAX_SAFE_INTEGER, (Number(run.action_index) || 0) + 1);
            const progress = actionCount && runStatus !== 'skipped' ? `<small>${esc(t(inst.ctx, 'rule_run_action_progress', { current: actionNumber, total: actionCount }))}</small>` : '';
            const explanationKey = ({ unknown: 'rule_run_unknown_help', failed: 'rule_run_failed_help' })[runStatus];
            const explanation = explanationKey ? `<p class="vd-mailer-rule-run-error">${esc(t(inst.ctx, explanationKey))}</p>` : '';
            const diagnostic = run.error ? `<details class="vd-mailer-rule-run-diagnostic"><summary>${esc(t(inst.ctx, 'rule_run_error_details'))}</summary><p>${esc(run.error)}</p></details>` : '';
            const retry = ['failed', 'unknown'].includes(String(run.status)) && !inst.state.readonly
                ? `<button type="button" class="vd-mailer-button" data-retry-rule-run="${esc(run.rule_id)}" data-message-id="${esc(messageId)}" data-run-status="${esc(run.status)}">${esc(t(inst.ctx, 'retry'))}</button>` : '';
            return `<li data-rule-id="${esc(run.rule_id)}"><b>${esc(t(inst.ctx, statusKey))}</b>${progress}${explanation}${diagnostic}${retry}</li>`;
        }).join('')}</ul></section>`;
    }

    async function loadRuleRuns(inst, threadId, messages) {
        if (!threadId || !messages.length || typeof inst.api.ruleRuns !== 'function') return;
        const targets = messages.filter(message => message.id);
        for (const message of targets) {
            const id = String(message.id);
            const state = inst.ruleRunsByMessage[id] || (inst.ruleRunsByMessage[id] = {});
            state.loading = true;
            state.error = false;
            paintRuleRunsForMessage(inst, id);
        }
        await Promise.all(targets.map(async message => {
            const id = String(message.id);
            const state = inst.ruleRunsByMessage[id];
            try {
                const response = await inst.api.ruleRuns(id);
                state.items = responseItems(response, 'items');
                state.error = false;
            } catch (_) { state.error = true; }
            finally {
                state.loading = false;
                if (!inst.disposed && inst.activeThread === threadId) paintRuleRunsForMessage(inst, id);
            }
        }));
    }

    async function retryMessageRule(inst, messageId, ruleId, status) {
        if (inst.state.readonly || !messageId || !ruleId) return;
        if (status === 'unknown') {
            const accepted = await inst.ctx.confirmDialog(t(inst.ctx, 'rule_run_retry_unknown_title'), t(inst.ctx, 'rule_run_retry_unknown_confirm'));
            if (!accepted || inst.disposed) return;
        }
        try {
            await inst.api.retryRuleRun(messageId, ruleId);
            setStatus(inst, t(inst.ctx, 'rule_run_retry_queued'));
            const threadId = inst.activeThread;
            const messages = inst.activeThreadData?.messages || [];
            await loadRuleRuns(inst, threadId, messages);
        } catch (error) { showError(inst, error); }
    }

    function messageHasRemoteImages(html) {
        const template = document.createElement('template');
        template.innerHTML = String(html || '');
        return Array.from(template.content.querySelectorAll('img[src]')).some(image => /^(?:https?:)?\/\//i.test((image.getAttribute('src') || '').trim()));
    }

    function findMessage(inst, messageId) { return inst.activeThreadData?.messages?.find(item => String(item.id) === String(messageId)); }

    function paintMessageFrames(inst) {
        const generation = clearInlineImages(inst);
        inst.readerAbort = new AbortController();
        const messages = inst.activeThreadData?.messages || [];
        for (const message of messages) {
            if (!message.html) continue;
            const frame = inst.host.querySelector(`[data-email-frame="${cssEscape(message.id)}"]`);
            if (frame) frame.srcdoc = safeMailHTML(message.html, inst.remoteImages.has(inst.activeThread), null, t(inst.ctx, 'quoted_message'));
        }
        Promise.all(messages.map(message => loadInlineImages(inst, message, inst.readerAbort.signal))).then(results => {
            const urls = results.flatMap(result => Array.from(result.urls));
            if (inst.disposed || generation !== inst.readerRenderGeneration) {
                urls.forEach(url => URL.revokeObjectURL(url));
                return;
            }
            for (const url of urls) inst.inlineImageURLs.add(url);
            for (const result of results) {
                if (!result.message.html) continue;
                const frame = inst.host.querySelector(`[data-email-frame="${cssEscape(result.message.id)}"]`);
                if (frame && result.images.size > 0) frame.srcdoc = safeMailHTML(result.message.html, inst.remoteImages.has(inst.activeThread), result.images, t(inst.ctx, 'quoted_message'));
            }
        }).catch(() => {});
    }

    function clearInlineImages(inst) {
        inst.readerRenderGeneration++;
        if (inst.readerAbort) inst.readerAbort.abort();
        inst.readerAbort = null;
        for (const url of inst.inlineImageURLs || []) URL.revokeObjectURL(url);
        inst.inlineImageURLs?.clear();
        return inst.readerRenderGeneration;
    }

    const INLINE_IMAGE_TYPES = new Set(['image/png', 'image/jpeg', 'image/gif', 'image/webp']);

    function normalizeContentID(value) { return window.MailerViews.normalizeContentID(value); }

    async function loadInlineImages(inst, message, signal) {
        const images = new Map();
        const urls = new Set();
        if (!message.html) return { message, images, urls };
        const source = document.createElement('template');
        source.innerHTML = String(message.html);
        const referenced = new Set(Array.from(source.content.querySelectorAll('img[src]'))
            .map(image => image.getAttribute('src') || '')
            .filter(src => /^cid:/i.test(src))
            .map(normalizeContentID));
        if (!referenced.size) return { message, images, urls };
        for (const attachment of message.attachments || []) {
            const contentID = normalizeContentID(attachment.content_id);
            const declaredType = String(attachment.content_type || '').split(';')[0].trim().toLowerCase();
            if (!contentID || !referenced.has(contentID) || !INLINE_IMAGE_TYPES.has(declaredType)) continue;
            try {
                const response = await fetch(inst.api.attachmentURL(message.id, attachment.id), { credentials: 'same-origin', cache: 'no-store', signal });
                if (!response.ok) continue;
                const responseType = String(response.headers.get('content-type') || '').split(';')[0].trim().toLowerCase();
                if (responseType && responseType !== 'application/octet-stream' && !INLINE_IMAGE_TYPES.has(responseType)) continue;
                const blob = await response.blob();
                if (blob.size > 8 * 1024 * 1024) continue;
                const url = URL.createObjectURL(new Blob([blob], { type: declaredType }));
                images.set(contentID, url);
                urls.add(url);
            } catch (_) {
                if (signal.aborted) break;
            }
        }
        return { message, images, urls };
    }

    function cssEscape(value) { return window.CSS && CSS.escape ? CSS.escape(String(value)) : String(value).replace(/[^a-zA-Z0-9_-]/g, '\\$&'); }

    async function readerAction(inst, action, button) {
        if (action === 'reload') { loadOpenThread(inst, inst.activeThread, false); return; }
        if (action === 'reply' || action === 'reply_all' || action === 'forward') {
            if (!canThreadCapability(inst, inst.activeThreadData || {}, 'send')) return;
            openCompose(inst, { replyType: action, thread: inst.activeThreadData }); return;
        }
        if (action === 'print') { printThread(inst); return; }
        if (action === 'eml') { const message = inst.activeThreadData?.messages?.slice(-1)[0]; if (message) window.open(inst.api.emlURL(message.id), '_blank', 'noopener'); return; }
        if (['star', 'unstar'].includes(action)) {
            const messageId = button?.dataset.messageId;
            if (messageId && canThreadCapability(inst, inst.activeThreadData || {}, 'star')) inst.api.action(inst.activeThread, { action }).then(() => loadOpenThread(inst, inst.activeThread, false)).catch(error => showError(inst, error));
            return;
        }
        if (inst.state.readonly) return;
        const required = { archive: 'archive', trash: 'trash', restore: 'restore', delete: 'delete', label_add: 'labels_write', label_remove: 'labels_write', mark_read: 'read_state', mark_unread: 'read_state' }[action];
        if (required && !canThreadCapability(inst, inst.activeThreadData || {}, required)) return;
        if (action === 'delete' && !canDeleteThread(inst, inst.activeThreadData || {})) return;
        if (action === 'delete' && !await inst.ctx.confirmDialog(t(inst.ctx, 'delete'), t(inst.ctx, 'delete_confirm'))) return;
        if (['archive', 'trash', 'restore', 'delete', 'mark_read', 'mark_unread'].includes(action)) runOneAction(inst, inst.activeThread, action);
    }

    function printThread(inst) {
        const thread = inst.activeThreadData;
        if (!thread) return;
        const content = thread.messages.map(message => `<article><h2>${esc(message.subject || thread.subject || '')}</h2><p><b>${esc(message.from || '')}</b> · ${esc(formatDate(message.date, inst.state.timezone))}</p>${message.html ? safeMailHTML(message.html, false, null, t(inst.ctx, 'quoted_message'), false).match(/<body>([\s\S]*)<\/body>/i)?.[1] || esc(message.text) : `<pre>${esc(message.text || '')}</pre>`}</article>`).join('<hr>');
        const popup = window.open('', '_blank');
        if (!popup) { setStatus(inst, t(inst.ctx, 'print_blocked'), true); return; }
        popup.opener = null;
        popup.onload = () => { try { popup.print(); } catch (_) {} };
        popup.document.write(`<!doctype html><html><head><meta charset="utf-8"><meta http-equiv="Content-Security-Policy" content="default-src 'none'; img-src 'none'; style-src 'unsafe-inline'; script-src 'none'; form-action 'none'; base-uri 'none'"><title>${esc(thread.subject || t(inst.ctx, 'title'))}</title><style>body{font:15px/1.6 system-ui,sans-serif;max-width:850px;margin:40px auto;color:#1d2632}img{max-width:100%;height:auto}pre{white-space:pre-wrap}</style></head><body>${content}</body></html>`);
        popup.document.close();
    }

    function openCompose(inst, options) {
        if (inst.state.readonly) return;
        options = options || {};
        let draft = options.draft || null;
        const thread = options.thread || (options.replyType ? inst.activeThreadData : null);
        let forwardMessage = null;
        let assistContext = '';
        if (!draft && thread) {
            const messages = thread.messages || [];
            const last = messages[messages.length - 1] || {};
            const account = inst.state.accounts.find(item => item.id === thread.account_id);
            const own = account?.address || '';
            const ownLower = own.toLowerCase();
            const validRecipient = value => {
                const address = mailboxAddress(value);
                return /^[^\s<>@,;]+@[^\s<>@,;]+\.[^\s<>@,;]+$/.test(address) ? address : '';
            };
            const replyToValues = Array.isArray(last.reply_to) ? last.reply_to : String(last.reply_to || '').split(/[;,]+/);
            const replyToAddresses = replyToValues.map(validRecipient).filter(Boolean);
            const replyAddresses = [...new Set((replyToAddresses.length ? replyToAddresses : [validRecipient(last.from || '')]).filter(Boolean))];
            const toAddresses = options.replyType === 'reply_all' ? [...new Set([...replyAddresses, ...(last.to || []).map(validRecipient).filter(Boolean)])] : replyAddresses;
            const allTo = toAddresses.filter(address => address.toLowerCase() !== ownLower);
            const allToLower = new Set(allTo.map(address => address.toLowerCase()));
            const allCc = options.replyType === 'reply_all' ? [...new Set((last.cc || []).map(validRecipient).filter(address => address && address.toLowerCase() !== ownLower && !allToLower.has(address.toLowerCase())))] : [];
            const prefix = options.replyType === 'forward' ? 'Fwd: ' : 'Re: ';
            const original = messageText(last);
            const quoted = original.split('\n').map(line => '> ' + line).join('\n');
            if (options.replyType === 'reply' || options.replyType === 'reply_all') assistContext = messageText(last).slice(0, 6000);
            if (options.replyType === 'forward') forwardMessage = last;
            const references = Array.isArray(last.references) ? last.references.slice() : String(last.references || '').split(/\s+/).filter(Boolean);
            if (last.message_id && !references.includes(last.message_id)) references.push(last.message_id);
            draft = { account_id: thread.account_id, to: options.replyType === 'forward' ? [] : allTo, cc: options.replyType === 'forward' ? [] : allCc, bcc: [], subject: (options.replyType === 'forward' ? /^fwd:/i : /^re:/i).test(thread.subject || '') ? thread.subject : prefix + (thread.subject || ''), text: options.replyType === 'forward' ? `\n\n---------- ${t(inst.ctx, 'forwarded_message')} ----------\n${quoted}` : `\n\n${quoted}`, html: '', in_reply_to: options.replyType === 'forward' ? '' : last.message_id || '', references: options.replyType === 'forward' ? [] : references, reply_remote_id: account?.source === 'agentmail' && ['reply', 'reply_all'].includes(options.replyType) ? last.remote_id || '' : '' };
        }
        let accountId = draft?.account_id || thread?.account_id || options.accountId || inst.accountId || '';
        if (!draft && !thread) {
            const preferred = inst.state.accounts.find(account => account.id === accountId && canThreadCapability(inst, { account_id: account.id }, 'send'));
            const account = preferred || inst.state.accounts.find(item => canThreadCapability(inst, { account_id: item.id }, 'send'));
            if (!account) { setStatus(inst, t(inst.ctx, 'no_accounts'), true); return; }
            accountId = account.id;
        }
        if (!canThreadCapability(inst, { account_id: accountId }, 'send')) {
            setStatus(inst, t(inst.ctx, 'reason_missing_scope'), true);
            return;
        }
        inst.ctx.openApp?.('mailer', { forceNew: true, compose: true, draft: draft || undefined, accountId, replyType: options.replyType, forwardMessage: forwardMessage || undefined, assistContext, state: inst.state });
    }

    function messageText(message) {
        if (message.text) return String(message.text);
        if (!message.html) return '';
        const safe = window.DOMPurify ? window.DOMPurify.sanitize(String(message.html), { FORBID_TAGS: ['script', 'style', 'iframe', 'object', 'embed', 'svg', 'math'] }) : String(message.html).replace(/<[^>]*>/g, ' ');
        const doc = new DOMParser().parseFromString(safe, 'text/html');
        return String(doc.body.textContent || '').replace(/\s+/g, ' ').trim();
    }

    function mailboxAddress(value) {
        const text = String(value || '').trim();
        const bracketed = text.match(/<([^<>\s]+@[^<>\s]+)>/);
        return bracketed ? bracketed[1] : text;
    }

    async function runToolbarAction(inst, action) {
        if (action === 'snooze') { openSnooze(inst); return; }
        if (action === 'junk') return runBulkAction(inst, 'junk');
        if (action === 'delete' && !canDeleteSelected(inst)) return;
        if (action === 'delete' && !await inst.ctx.confirmDialog(t(inst.ctx, 'delete'), t(inst.ctx, 'delete_confirm'))) return;
        runBulkAction(inst, action);
    }

    function canDeleteSelected(inst) {
        return selectedHasCapability(inst, 'delete');
    }

    function selectedReadOnly(inst) { return inst.threads.some(item => inst.selected.has(item.id) && inst.state.accounts.find(account => account.id === item.account_id)?.read_only); }

    async function runOneAction(inst, threadId, action, extra) {
        const thread = inst.activeThreadData?.id === threadId ? inst.activeThreadData : inst.threads.find(item => item.id === threadId);
        const required = { archive: 'archive', junk: 'junk', trash: 'trash', restore: 'restore', delete: 'delete', mark_read: 'read_state', mark_unread: 'read_state', star: 'star', unstar: 'star', move: 'move', label_add: 'labels_write', label_remove: 'labels_write' }[action];
        if (inst.state.readonly || !thread || required && !canThreadCapability(inst, thread, required) || !required && threadReadOnly(inst, thread)) return;
        try {
            await inst.api.action(threadId, Object.assign({ action }, extra || {}));
            await refreshCurrent(inst, true);
        } catch (error) {
            if (error?.status === 409 || error?.body?.code === 'partial_action' || error?.body?.error === 'partial_action') {
                await refreshCurrent(inst, true);
                setStatus(inst, t(inst.ctx, 'partial_action'), true);
                return;
            }
            showError(inst, error);
        }
    }

    async function runBulkAction(inst, action, extra) {
        const required = { archive: 'archive', junk: 'junk', trash: 'trash', restore: 'restore', delete: 'delete', mark_read: 'read_state', mark_unread: 'read_state', star: 'star', unstar: 'star', move: 'move', label_add: 'labels_write', label_remove: 'labels_write' }[action];
        if (inst.state.readonly || !inst.selected.size || (required ? !selectedHasCapability(inst, required) : !hasMutableSelection(inst))) return;
        const ids = Array.from(inst.selected);
        if (action === 'snooze') return openSnooze(inst);
        try {
            await Promise.all(ids.map(threadId => inst.api.action(threadId, Object.assign({ action }, extra || {}))));
            inst.selected.clear();
            if (inst.activeThread && ids.includes(inst.activeThread) && ['archive', 'trash', 'move', 'delete'].includes(action)) {
                inst.activeThread = null; inst.activeThreadData = null;
            }
            await refreshCurrent(inst, true);
            setStatus(inst, t(inst.ctx, 'action_complete'));
        } catch (error) {
            if (error?.status === 409 || error?.body?.code === 'partial_action' || error?.body?.error === 'partial_action') {
                await refreshCurrent(inst, true);
                setStatus(inst, t(inst.ctx, 'partial_action'), true);
                return;
            }
            showError(inst, error);
        }
    }

    function openSnooze(inst) {
        if (inst.state.readonly || !inst.selected.size || selectedReadOnly(inst)) return;
        const until = new Date(Date.now() + 86400000);
        inst.host.querySelector('[data-status]').innerHTML = `<span class="vd-mailer-snooze-dialog" role="group" aria-label="${esc(t(inst.ctx, 'snooze'))}"><label>${esc(t(inst.ctx, 'snooze_until'))}<input type="datetime-local" data-snooze-time value="${localDateTime(until)}"></label><button type="button" class="vd-mailer-button" data-snooze-submit>${esc(t(inst.ctx, 'snooze'))}</button><button type="button" class="vd-mailer-button" data-close-snooze>${esc(t(inst.ctx, 'cancel'))}</button></span>`;
        inst.host.querySelector('[data-snooze-time]').focus();
    }

    function localDateTime(date) { const d = new Date(date.getTime() - date.getTimezoneOffset() * 60000); return d.toISOString().slice(0, 16); }

    function closeSnooze(inst) { inst.host.querySelector('[data-status]').textContent = ''; }

    async function submitSnooze(inst) {
        const until = inst.host.querySelector('[data-snooze-time]').value;
        if (!until || new Date(until) <= new Date()) { setStatus(inst, t(inst.ctx, 'snooze_future_required'), true); return; }
        const ids = Array.from(inst.selected);
        try {
            for (const threadId of ids) {
                const detail = await inst.api.thread(threadId);
                for (const message of detail.messages || []) await inst.api.snooze(message.id, new Date(until).toISOString());
            }
            inst.selected.clear(); closeSnooze(inst); await refreshCurrent(inst, true);
        } catch (error) { showError(inst, error); }
    }

    async function refreshCurrent(inst, quiet) {
        if (!inst.state || inst.disposed) return;
        if (inst.view === 'drafts' || inst.view === 'outbox') return loadCurrent(inst);
        await loadCurrent(inst);
        if (!quiet) setStatus(inst, t(inst.ctx, 'refreshed'));
    }

    function setStatus(inst, message, isError) {
        const node = inst.host.querySelector('[data-status]');
        if (!node) return;
        node.textContent = message;
        node.classList.toggle('is-error', !!isError);
    }

    function showError(inst, error) {
        const key = error?.status === 403 ? 'readonly' : 'action_failed';
        setStatus(inst, t(inst.ctx, key), true);
    }

    function errorState(inst) {
        const root = inst.host.querySelector('[data-thread-list]');
        root.innerHTML = emptyState({ icon: '!', title: t(inst.ctx, 'load_failed'), action: 'refresh', actionLabel: t(inst.ctx, 'retry') });
        inst.host.querySelector('[data-mailer-workspace]').setAttribute('aria-busy', 'false');
    }

    function subscribe(inst) {
        const handler = event => {
            const envelope = event && (event.detail || event) || {};
            if (envelope.type !== 'mailer_changed') return;
            const change = envelope.payload || {};
            const accountId = change.account_id || '';
            if (change.type === 'rule_run' && inst.activeThreadData?.messages?.some(message => String(message.id) === String(change.id))) {
                const message = inst.activeThreadData.messages.find(item => String(item.id) === String(change.id));
                loadRuleRuns(inst, inst.activeThread, [message]);
            }
            if (change.type === 'account_sync' && (!inst.accountId || !accountId || accountId === inst.accountId)) refreshAccountSyncStatus(inst, change);
            if (!accountId || !inst.accountId || accountId === inst.accountId) refreshCurrent(inst, true);
        };
        inst.eventHandler = handler;
        if (window.AuraSSE?.on) window.AuraSSE.on('virtual_desktop_event', handler);
        else {
            const domHandler = event => handler(event.detail);
            inst.listeners.push([document, 'aurago:mailer-change', domHandler]);
            document.addEventListener('aurago:mailer-change', domHandler);
        }
    }

    async function refreshAccountSyncStatus(inst, change) {
        try {
            const latest = await inst.api.state();
            if (inst.disposed) return;
            inst.state.accounts = latest.accounts || inst.state.accounts || [];
            inst.state.sources = latest.sources || inst.state.sources || [];
            inst.state.account_settings = latest.account_settings || inst.state.account_settings || {};
            for (const account of inst.state.accounts) Object.assign(account, accountSettingsRecord(inst.state, account.id));
            // A background sync event must not open the settings overlay.
            if (!inst.host.querySelector('[data-settings]').hidden) await inst.settings.paint();
            const account = inst.state.accounts.find(item => item.id === change.account_id);
            const code = account?.sync_error || (change.status === 'error' ? 'sync_failed' : '');
            if (code) setStatus(inst, inst.settings.reasonText(code), true);
            else if (change.status === 'ok') setStatus(inst, t(inst.ctx, 'refreshed'));
        } catch (error) { showError(inst, error); }
    }

    async function retryOutbox(inst, id) {
        const item = inst.outbox.find(entry => String(entry.id) === String(id));
        if (!item || item.status !== 'failed' || inst.state.readonly) return;
        try { await inst.api.retrySend(id); await loadCurrent(inst); } catch (error) { showError(inst, error); }
    }

    async function cancelOutbox(inst, id) {
        const item = inst.outbox.find(entry => String(entry.id) === String(id));
        if (!item || !['scheduled', 'sending'].includes(item.status) || inst.state.readonly) return;
        try { await inst.api.cancelSend(id); await loadCurrent(inst); } catch (error) { showError(inst, error); }
    }

    async function reloadState(inst) {
        inst.state = await inst.api.state();
        inst.state.accounts = inst.state.accounts || []; inst.state.sources = inst.state.sources || []; inst.state.folders = inst.state.folders || []; inst.state.labels = normalizeLabels(inst.state.labels || inst.state.folders.filter(folder => folder.kind === 'label'));
        inst.state.account_settings = inst.state.account_settings || {};
        for (const account of inst.state.accounts) Object.assign(account, accountSettingsRecord(inst.state, account.id));
        inst.state.readonly = !!(inst.ctx.readonly || inst.state.readonly);
        inst.host.querySelector('[data-compose]').disabled = inst.state.readonly;
        paintAccountPicker(inst); await loadFolders(inst); paintFolders(inst); paintMoveLabels(inst); await inst.settings.paint(); await loadCurrent(inst);
    }

    function accountSettingsRecord(state, accountId) {
        const value = state.account_settings || {};
        const record = Array.isArray(value) ? value.find(item => item.account_id === accountId) : value[accountId];
        return { disabled: !!record?.disabled, read_only: !!record?.read_only, agent_access: !!record?.agent_access, signature_id: record?.signature_id || '', folder_map: Object.assign({}, record?.folder_map || {}) };
    }

    async function reloadStateQuiet(inst) {
        try { await reloadState(inst); } catch (error) { showError(inst, error); }
    }

    function startResize(inst, event, which) {
        if (event.button !== 0) return;
        event.preventDefault();
        const workspace = inst.host.querySelector('[data-mailer-workspace]');
        const computed = getComputedStyle(workspace);
        const vars = {
            side: parseInt(computed.getPropertyValue('--mail-side')) || 230,
            list: parseInt(computed.getPropertyValue('--mail-list')) || 360
        };
        inst.resizeState = { pointerId: event.pointerId, which, startX: event.clientX, original: vars[which], handle: event.currentTarget };
        event.currentTarget.setPointerCapture(event.pointerId);
        const move = current => {
            if (!inst.resizeState || current.pointerId !== inst.resizeState.pointerId) return;
            const delta = current.clientX - inst.resizeState.startX;
            const next = clamp(inst.resizeState.original + delta, which === 'side' ? 170 : 260, which === 'side' ? 340 : 600);
            workspace.style.setProperty(which === 'side' ? '--mail-side' : '--mail-list', next + 'px');
        };
        const end = current => {
            if (!inst.resizeState || current.pointerId !== inst.resizeState.pointerId) return;
            const endValue = parseInt(getComputedStyle(workspace).getPropertyValue(which === 'side' ? '--mail-side' : '--mail-list'));
            savePrefs(Object.assign(localPrefs(), { [which]: endValue }));
            document.removeEventListener('pointermove', move); document.removeEventListener('pointerup', end); inst.resizeState = null;
        };
        document.addEventListener('pointermove', move); document.addEventListener('pointerup', end);
    }

    function resizeKeyboard(inst, which, delta) {
        const workspace = inst.host.querySelector('[data-mailer-workspace]');
        const key = which === 'side' ? 'side' : 'list';
        const prop = key === 'side' ? '--mail-side' : '--mail-list';
        const current = parseInt(getComputedStyle(workspace).getPropertyValue(prop)) || (key === 'side' ? 230 : 360);
        const next = clamp(current + delta, key === 'side' ? 170 : 260, key === 'side' ? 340 : 600);
        workspace.style.setProperty(prop, next + 'px');
        savePrefs(Object.assign(localPrefs(), { [key]: next }));
    }

    function restoreSizes(inst) {
        const prefs = localPrefs();
        const workspace = inst.host.querySelector('[data-mailer-workspace]');
        if (prefs.side) workspace.style.setProperty('--mail-side', clamp(prefs.side, 170, 340) + 'px');
        if (prefs.list) workspace.style.setProperty('--mail-list', clamp(prefs.list, 260, 600) + 'px');
    }

    function clamp(value, min, max) { return Math.max(min, Math.min(max, Number(value) || min)); }

    function dispose(windowId) {
        const inst = instances.get(windowId);
        if (!inst) { window.MailerCompose.dispose(windowId); return; }
        inst.disposed = true;
        clearInlineImages(inst);
        clearInterval(inst.refreshTimer); clearTimeout(inst.searchTimer);
        if (inst.abort) inst.abort.abort();
        inst.listeners.forEach(([target, type, handler, options]) => target.removeEventListener(type, handler, options));
        if (inst.eventHandler && window.AuraSSE?.off) window.AuraSSE.off('virtual_desktop_event', inst.eventHandler);
        instances.delete(windowId);
        window.MailerCompose.dispose(windowId);
    }

    window.MailerApp = { render, dispose, safeMailHTML };
})();
