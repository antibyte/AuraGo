(function () {
    'use strict';

    function create(inst, deps) {
        const { esc, t, responseItems, accountSettingsRecord, showError, setStatus, reloadState, refreshCurrent, localDateTime } = deps;
        function showSettings(inst) {
            inst.host.querySelector('[data-settings]').hidden = false;
            paintSettings(inst);
        }

        async function paintSettings(inst) {
            const root = inst.host.querySelector('[data-settings]');
            root.hidden = false;
            root.innerHTML = `<header class="vd-mailer-settings-head"><div><p class="vd-mailer-eyebrow">${esc(t(inst.ctx, 'settings'))}</p><h2>${esc(t(inst.ctx, 'manage_mail'))}</h2></div><button type="button" class="vd-mailer-button" data-settings-close aria-label="${esc(t(inst.ctx, 'close'))}">×</button></header>
                <nav class="vd-mailer-settings-tabs" role="tablist">${['accounts', 'templates', 'signatures', 'rules'].map(tab => `<button type="button" role="tab" aria-selected="${inst.settingsTab === tab}" data-setting-tab="${tab}">${esc(t(inst.ctx, tab))}</button>`).join('')}</nav>
                <div class="vd-mailer-settings-content" data-settings-content>${esc(t(inst.ctx, 'loading'))}</div>`;
            const content = root.querySelector('[data-settings-content]');
            try {
                if (inst.settingsTab === 'accounts') { paintAccountsSettings(inst, content); return; }
                if (inst.settingsTab === 'templates' || inst.settingsTab === 'signatures') {
                    const data = await (inst.settingsTab === 'templates' ? inst.api.templates() : inst.api.signatures());
                    if (inst.disposed || root.hidden) return;
                    const items = responseItems(data, inst.settingsTab);
                    inst.records = { kind: inst.settingsTab === 'signatures' ? 'signature' : 'template', items };
                    content.innerHTML = `<div class="vd-mailer-settings-toolbar"><p>${esc(t(inst.ctx, inst.settingsTab + '_hint'))}</p><button type="button" class="vd-mailer-button vd-mailer-primary" data-create-record="${inst.records.kind}" ${inst.state.readonly ? 'disabled' : ''}>${esc(t(inst.ctx, 'new_' + inst.records.kind))}</button></div>${items.map(item => `<article class="vd-mailer-record"><div><b>${esc(item.name)}</b><small>${esc(item.subject || item.account_id || '')}</small></div><div><button type="button" data-edit-record="${esc(item.id)}" ${inst.state.readonly ? 'disabled' : ''}>${esc(t(inst.ctx, 'edit'))}</button><button type="button" data-delete-record="${esc(item.id)}" ${inst.state.readonly ? 'disabled' : ''}>${esc(t(inst.ctx, 'delete'))}</button></div></article>`).join('') || `<p class="vd-mailer-empty-copy">${esc(t(inst.ctx, 'no_records'))}</p>`}`;
                    return;
                }
                const data = await inst.api.rules();
                if (inst.disposed || root.hidden) return;
                const items = responseItems(data, 'rules');
                inst.records = { kind: 'rule', items };
                content.innerHTML = `<div class="vd-mailer-settings-toolbar"><p>${esc(t(inst.ctx, 'rules_hint'))}</p><button type="button" class="vd-mailer-button vd-mailer-primary" data-create-record="rule" ${inst.state.readonly ? 'disabled' : ''}>${esc(t(inst.ctx, 'new_rule'))}</button></div>${items.map(rule => `<article class="vd-mailer-record"><div><b>${esc(rule.name)}</b><small>${esc(t(inst.ctx, rule.enabled ? 'rule_enabled' : 'rule_disabled'))}</small></div><div><label class="vd-mailer-inline-check"><input type="checkbox" data-rule-enabled="${esc(rule.id)}" ${rule.enabled ? 'checked' : ''} ${inst.state.readonly ? 'disabled' : ''}><span>${esc(t(inst.ctx, 'enabled'))}</span></label><button type="button" data-edit-record="${esc(rule.id)}" ${inst.state.readonly ? 'disabled' : ''}>${esc(t(inst.ctx, 'edit'))}</button><button type="button" data-delete-record="${esc(rule.id)}" ${inst.state.readonly ? 'disabled' : ''}>${esc(t(inst.ctx, 'delete'))}</button></div></article>`).join('') || `<p class="vd-mailer-empty-copy">${esc(t(inst.ctx, 'no_records'))}</p>`}`;
            } catch (error) { content.innerHTML = `<p class="vd-mailer-inline-error">${esc(t(inst.ctx, 'load_failed'))}</p>`; }
        }

        function paintAccountsSettings(inst, content) {
            const sources = inst.state.sources || [];
            const accounts = inst.state.accounts || [];
            content.innerHTML = `<div class="vd-mailer-settings-toolbar"><label class="vd-mailer-inline-check"><input type="checkbox" data-enable-app ${inst.state.enabled ? 'checked' : ''} ${inst.state.readonly ? 'disabled' : ''}><span>${esc(t(inst.ctx, 'background_sync'))}</span></label><div class="vd-mailer-create-account-actions"><button type="button" class="vd-mailer-button" data-add-account="email" ${inst.state.readonly ? 'disabled' : ''}>${esc(t(inst.ctx, 'add_email_account'))}</button><button type="button" class="vd-mailer-button" data-add-account="google" ${inst.state.readonly ? 'disabled' : ''}>${esc(t(inst.ctx, 'add_google_account'))}</button><button type="button" class="vd-mailer-button" data-add-account="microsoft" ${inst.state.readonly ? 'disabled' : ''}>${esc(t(inst.ctx, 'add_microsoft_account'))}</button></div></div>
                <section class="vd-mailer-source-list"><h3>${esc(t(inst.ctx, 'connect_existing'))}</h3>${sources.map(source => {
                    const caps = new Set(source.capabilities || []);
                    const blocked = !!source.disabled || !caps.has('read');
                    const reasonCode = source.reason || (source.disabled ? 'account_disabled' : !caps.has('read') ? 'mail_read_required' : '');
                    return `<article class="vd-mailer-source ${source.linked ? 'is-linked' : ''}"><div><b>${esc(source.name || source.address)}</b><small>${esc(source.address || source.source || '')} · ${esc(t(inst.ctx, 'source_' + String(source.source || '').toLowerCase()))}${source.protocol ? ` · ${esc(t(inst.ctx, 'receive_protocol'))}: ${esc(source.protocol)}` : ''}</small>${reasonCode ? `<p>${esc(reasonText(inst, reasonCode))}</p>` : ''}<div class="vd-mailer-source-rights"><span>${esc(t(inst.ctx, 'test_receive'))}: ${esc(t(inst.ctx, caps.has('read') ? 'yes' : 'no'))}</span><span>${esc(t(inst.ctx, 'test_send'))}: ${esc(t(inst.ctx, caps.has('send') ? 'yes' : 'no'))}</span><span>${esc(t(inst.ctx, 'agent_access'))}: ${esc(t(inst.ctx, source.agent_access ? 'yes' : 'no'))}</span>${source.read_only ? `<span>${esc(t(inst.ctx, 'account_readonly'))}</span>` : ''}</div></div><div class="vd-mailer-source-actions">${source.reauthorize_supported ? `<button type="button" class="vd-mailer-button" data-reauthorize="${esc(source.id)}" ${inst.state.readonly ? 'disabled' : ''}>${esc(t(inst.ctx, 'reauthorize'))}</button>` : ''}${source.linked ? `<span class="vd-mailer-connected">${esc(t(inst.ctx, 'connected'))}</span>` : `<button type="button" class="vd-mailer-button" data-connect-source="${esc(source.id)}" ${inst.state.readonly || blocked ? 'disabled' : ''}>${esc(t(inst.ctx, 'connect'))}</button>`}</div></article>`;
                }).join('') || `<p class="vd-mailer-empty-copy">${esc(t(inst.ctx, 'no_sources'))}</p>`}</section>
                <section class="vd-mailer-connected-list"><h3>${esc(t(inst.ctx, 'connected_accounts'))}</h3>${accounts.map(account => {
                    const reasonCode = account.sync_error || account.reason || (account.disabled ? 'account_disabled' : '');
                    return `<article class="vd-mailer-account-setting"><div><b>${esc(account.name || account.address)}</b><small>${esc(account.address)} · ${esc(account.source)}</small>${reasonCode ? `<p>${esc(reasonText(inst, reasonCode))}</p>` : ''}</div><div class="vd-mailer-account-setting-controls"><button type="button" data-test-account="${esc(account.id)}" data-direction="receive" ${inst.state.readonly || !account.capabilities?.includes('read') ? 'disabled' : ''}>${esc(t(inst.ctx, 'test_receive'))}</button><button type="button" data-test-account="${esc(account.id)}" data-direction="send" ${inst.state.readonly || !account.capabilities?.includes('send') ? 'disabled' : ''}>${esc(t(inst.ctx, 'test_send'))}</button>${account.reauthorize_supported ? `<button type="button" data-reauthorize="${esc(account.id)}" ${inst.state.readonly ? 'disabled' : ''}>${esc(t(inst.ctx, 'reauthorize'))}</button>` : ''}<label class="vd-mailer-inline-check"><input type="checkbox" data-account-pref="disabled" data-account-id="${esc(account.id)}" ${account.disabled ? 'checked' : ''} ${inst.state.readonly ? 'disabled' : ''}><span>${esc(t(inst.ctx, 'account_disabled'))}</span></label><label class="vd-mailer-inline-check"><input type="checkbox" data-account-pref="read_only" data-account-id="${esc(account.id)}" ${account.read_only ? 'checked' : ''} ${inst.state.readonly ? 'disabled' : ''}><span>${esc(t(inst.ctx, 'account_readonly'))}</span></label>${canConfigureAgentAccess(account) ? `<label class="vd-mailer-inline-check"><input type="checkbox" data-account-pref="agent_access" data-account-id="${esc(account.id)}" ${account.agent_access ? 'checked' : ''} ${inst.state.readonly ? 'disabled' : ''}><span>${esc(t(inst.ctx, 'agent_access'))}</span></label>` : ''}${folderMappingControls(inst, account)}<button type="button" data-disconnect-account="${esc(account.id)}" ${inst.state.readonly ? 'disabled' : ''}>${esc(t(inst.ctx, 'disconnect'))}</button></div></article>`;
                }).join('') || `<p class="vd-mailer-empty-copy">${esc(t(inst.ctx, 'no_accounts'))}</p>`}</section>
                <section class="vd-mailer-oauth-guidance"><h3>${esc(t(inst.ctx, 'oauth_title'))}</h3><p>${esc(t(inst.ctx, 'oauth_byok'))}</p><p>${esc(t(inst.ctx, 'oauth_private'))}</p></section>`;
            content.querySelectorAll('[data-account-pref]').forEach(input => input.addEventListener('change', () => updateAccountPref(inst, input.dataset.accountId, input.dataset.accountPref, input.type === 'checkbox' ? input.checked : input.value)));
            content.querySelectorAll('[data-account-folder-map]').forEach(input => input.addEventListener('change', () => updateAccountPref(inst, input.dataset.accountId, 'folder_map.' + input.dataset.accountFolderMap, input.value)));
            content.onclick = event => {
                const button = event.target.closest('[data-reauthorize]');
                if (button) reauthorizeAccount(inst, button.dataset.reauthorize, button);
            };
        }

        function reasonText(inst, code) {
            if (!code) return '';
            const keys = {
                account_disabled: 'reason_account_disabled', integration_disabled: 'reason_integration_disabled', disabled: 'reason_account_disabled',
                desktop_access_required: 'reason_desktop_access_required', mail_read_required: 'reason_mail_read_required',
                network_disabled: 'reason_network_disabled', missing_scope: 'reason_missing_scope', auth_required: 'reason_auth_required',
                permission_denied: 'reason_permission_denied', offline: 'reason_offline', sync_failed: 'reason_sync_failed', storage_full: 'reason_storage_full'
            };
            return t(inst.ctx, keys[String(code).toLowerCase()] || 'reason_unknown');
        }

        async function reauthorizeAccount(inst, accountId, button) {
            if (inst.state.readonly) return;
            const supported = [...(inst.state.accounts || []), ...(inst.state.sources || [])].some(account => account.id === accountId && account.reauthorize_supported);
            if (!supported) return;
            if (button) button.disabled = true;
            const popup = window.open('about:blank', 'mailer-oauth-' + Date.now(), 'popup,width=640,height=760');
            let handler = null;
            if (popup) {
                handler = event => {
                    if (event.origin !== window.location.origin || event.source !== popup || event.data?.type !== 'aurago-mailer-oauth') return;
                    window.removeEventListener('message', handler);
                    handler = null;
                    if (event.data.status === 'ok') {
                        setStatus(inst, t(inst.ctx, 'account_connected'));
                        reloadState(inst).catch(showError.bind(null, inst));
                    } else setStatus(inst, t(inst.ctx, 'oauth_failed'), true);
                };
                window.addEventListener('message', handler);
            }
            try {
                const auth = await inst.api.startOAuth({ account_id: accountId });
                const authURL = String(auth?.auth_url || '');
                if (!authURL) throw new Error('oauth_start_failed');
                if (popup && !popup.closed) popup.location.href = authURL;
                else {
                    const note = contentMessage(inst, t(inst.ctx, 'oauth_popup_blocked'));
                    const link = document.createElement('a'); link.href = authURL; link.target = '_blank'; link.rel = 'noopener noreferrer'; link.textContent = t(inst.ctx, 'open_authorize_window'); note.append(' ', link);
                }
                setStatus(inst, t(inst.ctx, 'oauth_authorizing'));
            } catch (error) {
                if (handler) window.removeEventListener('message', handler);
                if (popup && !popup.closed) popup.close();
                showError(inst, error);
            } finally { if (button) button.disabled = false; }
        }

        function folderMappingControls(inst, account) {
            const map = accountSettingsRecord(inst.state, account.id).folder_map;
            const folders = inst.state.folders.filter(folder => folder.account_id === account.id);
            const kinds = ['sent', 'drafts', 'archive', 'junk', 'trash'];
            return `<details class="vd-mailer-folder-mapping"><summary>${esc(t(inst.ctx, 'folder_mappings'))}</summary><div>${kinds.map(kind => `<label><span>${esc(t(inst.ctx, 'folder_map_' + kind))}</span><select data-account-folder-map="${kind}" data-account-id="${esc(account.id)}" ${inst.state.readonly || account.read_only ? 'disabled' : ''}><option value="">${esc(t(inst.ctx, 'automatic'))}</option>${folders.map(folder => `<option value="${esc(folder.id)}" ${map[kind] === folder.id ? 'selected' : ''}>${esc(folder.name)}</option>`).join('')}</select></label>`).join('')}</div></details>`;
        }

        async function connectSource(inst, sourceId) {
            if (inst.state.readonly) return;
            try { const result = await inst.api.connect(sourceId); await reloadState(inst); setStatus(inst, result?.connected === false ? t(inst.ctx, 'account_created_connect_required') : t(inst.ctx, 'account_connected')); }
            catch (error) { showError(inst, error); }
        }

        async function createFolder(inst, accountId) {
            const account = inst.state.accounts.find(item => item.id === accountId);
            if (inst.state.readonly || !account || account.read_only || !account.capabilities?.includes('folders_create') || !inst.ctx.promptDialog) return;
            const name = String(await inst.ctx.promptDialog(t(inst.ctx, 'new_folder'), '') || '').trim();
            if (!name) return;
            try { await inst.api.createFolder(accountId, name); inst.accountId = accountId; await reloadState(inst); setStatus(inst, t(inst.ctx, 'folder_created')); }
            catch (error) { showError(inst, error); }
        }

        async function disconnectAccount(inst, accountId) {
            if (inst.state.readonly || !await inst.ctx.confirmDialog(t(inst.ctx, 'disconnect'), t(inst.ctx, 'disconnect_confirm'))) return;
            try { await inst.api.disconnect(accountId); await reloadState(inst); }
            catch (error) { showError(inst, error); }
        }

        async function updateAccountPref(inst, accountId, key, value) {
            if (inst.state.readonly) return;
            const current = accountSettingsRecord(inst.state, accountId);
            const next = Object.assign({}, current);
            if (key.startsWith('folder_map.')) {
                next.folder_map = Object.assign({}, current.folder_map || {});
                const kind = key.slice('folder_map.'.length);
                if (value) next.folder_map[kind] = value; else delete next.folder_map[kind];
            } else next[key] = value;
            try { await inst.api.accountSettings(accountId, next); await reloadState(inst); }
            catch (error) { showError(inst, error); }
        }

        async function testAccount(inst, accountId, direction) {
            if (inst.state.readonly || !['receive', 'send'].includes(direction)) return;
            setStatus(inst, t(inst.ctx, direction === 'receive' ? 'testing_receive' : 'testing_send'));
            try {
                const result = await inst.api.testAccount(accountId, direction);
                const outcome = result[direction] || result.status;
                const errorCode = outcome === 'error' ? result[direction + '_error'] : '';
                const key = errorCode ? 'test_result_error_detail' : 'test_result_' + String(outcome).toLowerCase();
                setStatus(inst, t(inst.ctx, key, { code: String(errorCode || ''), direction: t(inst.ctx, direction === 'receive' ? 'test_receive' : 'test_send') }), outcome === 'error');
            } catch (error) { showError(inst, error); }
        }

        async function toggleEnabled(inst, enabled) {
            if (inst.state.readonly) return;
            try { await inst.api.settings(enabled); inst.state.enabled = enabled; setStatus(inst, t(inst.ctx, enabled ? 'sync_enabled' : 'sync_disabled')); }
            catch (error) { showError(inst, error); }
        }

        function paintAccountForm(inst, type) {
            const content = inst.host.querySelector('[data-settings-content]');
            const googleSources = (inst.state.sources || []).filter(source => /gog|google/i.test(source.source || ''));
            content.innerHTML = `<form class="vd-mailer-account-form" data-account-form><h3>${esc(t(inst.ctx, 'new_account_' + type))}</h3><label><span>${esc(t(inst.ctx, 'account_name'))}</span><input name="name" required maxlength="120"></label><label><span>${esc(t(inst.ctx, 'email_address'))}</span><input name="address" type="email" required autocomplete="email"></label>
                ${type === 'google' ? `<label><span>${esc(t(inst.ctx, 'receive_protocol'))}</span><select name="receive_protocol" data-google-protocol><option value="gmail_api">Gmail API</option><option value="imap">IMAP</option><option value="pop3">POP3</option></select></label><label><span>${esc(t(inst.ctx, 'google_workspace_source'))}</span><select name="google_mode" data-google-mode required><option value="">${esc(t(inst.ctx, 'choose_google_source'))}</option>${googleSources.map(source => `<option value="${esc(source.id)}">${esc(source.name || source.address)}</option>`).join('')}<option value="__new__">${esc(t(inst.ctx, 'google_use_oauth'))}</option></select><small>${esc(t(inst.ctx, 'google_workspace_hint'))}</small><small data-google-protocol-hint hidden>${esc(t(inst.ctx, 'google_mail_protocol_hint'))}</small></label><div class="vd-mailer-form-grid" data-google-oauth hidden><label><span>${esc(t(inst.ctx, 'oauth_client_id'))}</span><input name="oauth_client_id" autocomplete="off"></label><label><span>${esc(t(inst.ctx, 'oauth_client_secret'))}</span><input name="oauth_client_secret" type="password" autocomplete="new-password"></label></div>` : ''}
                ${type === 'microsoft' ? `<label><span>${esc(t(inst.ctx, 'receive_protocol'))}</span><select name="receive_protocol"><option value="imap">IMAP</option><option value="pop3">POP3</option></select></label><div class="vd-mailer-form-grid"><label><span>${esc(t(inst.ctx, 'oauth_client_id'))}</span><input name="oauth_client_id" required autocomplete="off"></label><label><span>${esc(t(inst.ctx, 'oauth_client_secret'))}</span><input name="oauth_client_secret" type="password" required autocomplete="new-password"></label></div><p>${esc(t(inst.ctx, 'microsoft_oauth_hint'))}</p>` : ''}
                ${type === 'email' ? `<div class="vd-mailer-form-grid"><label><span>${esc(t(inst.ctx, 'receive_protocol'))}</span><select name="receive_protocol"><option value="imap">IMAP</option><option value="pop3">POP3</option></select></label><label><span>${esc(t(inst.ctx, 'receive_security'))}</span><select name="receive_tls"><option value="tls">${esc(t(inst.ctx, 'implicit_tls'))}</option><option value="starttls">STARTTLS</option></select></label>
                    <label><span>${esc(t(inst.ctx, 'receive_host'))}</span><input name="imap_host" autocomplete="url"></label><label><span>${esc(t(inst.ctx, 'receive_port'))}</span><input name="imap_port" type="number" min="1" max="65535" value="993"></label>
                    <label><span>${esc(t(inst.ctx, 'send_host'))}</span><input name="smtp_host" required autocomplete="url"></label><label><span>${esc(t(inst.ctx, 'send_port'))}</span><input name="smtp_port" type="number" min="1" max="65535" value="587"></label><label><span>${esc(t(inst.ctx, 'send_security'))}</span><select name="smtp_tls"><option value="starttls">STARTTLS</option><option value="tls">${esc(t(inst.ctx, 'implicit_tls'))}</option></select></label>
                    <label><span>${esc(t(inst.ctx, 'username'))}</span><input name="username" required autocomplete="username"></label><label><span>${esc(t(inst.ctx, 'password'))}</span><input name="password" type="password" autocomplete="new-password"></label>
                    <label><span>${esc(t(inst.ctx, 'smtp_username'))}</span><input name="smtp_username" autocomplete="username"></label><label><span>${esc(t(inst.ctx, 'smtp_password'))}</span><input name="smtp_password" type="password" autocomplete="new-password"></label><label><span>${esc(t(inst.ctx, 'smtp_auth_method'))}</span><select name="smtp_auth_method"><option value="password">${esc(t(inst.ctx, 'password'))}</option><option value="xoauth2">OAuth 2.0</option></select></label></div>` : ''}
                <p class="vd-mailer-secret-note">${esc(t(inst.ctx, 'credential_vault_note'))}</p><div class="vd-mailer-form-actions"><button type="button" class="vd-mailer-button" data-cancel-account-form>${esc(t(inst.ctx, 'cancel'))}</button><button type="button" class="vd-mailer-button vd-mailer-primary" data-save-account data-type="${esc(type)}">${esc(t(inst.ctx, 'save_account'))}</button></div></form>`;
            const mode = content.querySelector('[data-google-mode]');
            if (mode) mode.addEventListener('change', () => {
                const useOAuth = mode.value === '__new__';
                content.querySelector('[data-google-oauth]').hidden = !useOAuth;
                content.querySelector('[data-google-oauth] input[name=oauth_client_id]').required = useOAuth;
                content.querySelector('[data-google-oauth] input[name=oauth_client_secret]').required = useOAuth;
            });
            const googleProtocol = content.querySelector('[data-google-protocol]');
            if (googleProtocol) googleProtocol.addEventListener('change', () => {
                content.querySelector('[data-google-protocol-hint]').hidden = googleProtocol.value === 'gmail_api';
            });
            content.querySelector('[name=name]').focus();
        }

        async function saveAccountForm(inst) {
            const form = inst.host.querySelector('[data-account-form]');
            const type = inst.host.querySelector('[data-save-account]').dataset.type;
            if (!form.reportValidity()) return;
            const data = new FormData(form);
            const addressSlug = slug(String(data.get('address') || ''));
            const payload = { type, id: addressSlug, name: data.get('name'), address: data.get('address') };
            let oauthRequired = type === 'microsoft';
            let authURL = '';
            if (type === 'email') {
                payload.receive_protocol = data.get('receive_protocol'); payload.receive_tls = data.get('receive_tls');
                const host = String(data.get('imap_host') || '');
                if (payload.receive_protocol === 'imap') { payload.imap_host = host; payload.imap_port = Number(data.get('imap_port')); }
                else { payload.pop3_host = host; payload.pop3_port = Number(data.get('imap_port')); }
                payload.smtp_host = data.get('smtp_host'); payload.smtp_port = Number(data.get('smtp_port'));
                payload.smtp_tls = data.get('smtp_tls');
                payload.username = data.get('username'); payload.password = data.get('password');
                payload.smtp_username = data.get('smtp_username'); payload.smtp_password = data.get('smtp_password'); payload.smtp_auth_method = data.get('smtp_auth_method');
            } else if (type === 'google') {
                const protocol = data.get('receive_protocol') || 'gmail_api';
                const mode = data.get('google_mode') || 'existing';
                if (protocol !== 'gmail_api') {
                    payload.receive_protocol = protocol;
                    payload.id = `${addressSlug.slice(0, 43)}-${protocol}`;
                }
                if (mode === '__new__') { oauthRequired = true; payload.oauth_client_id = data.get('oauth_client_id'); payload.oauth_client_secret = data.get('oauth_client_secret'); }
                else payload.gog_account_id = mode;
            } else if (type === 'microsoft') { payload.receive_protocol = data.get('receive_protocol') || 'imap'; payload.oauth_client_id = data.get('oauth_client_id'); payload.oauth_client_secret = data.get('oauth_client_secret'); }
            const popup = oauthRequired ? window.open('about:blank', 'mailer-oauth-' + Date.now(), 'popup,width=640,height=760') : null;
            let oauthHandler = null;
            if (oauthRequired && popup) {
                oauthHandler = event => {
                    if (event.origin !== window.location.origin || event.source !== popup || event.data?.type !== 'aurago-mailer-oauth') return;
                    window.removeEventListener('message', oauthHandler);
                    oauthHandler = null;
                    if (event.data.status === 'ok') {
                        setStatus(inst, t(inst.ctx, 'account_connected'));
                        reloadState(inst).catch(showError.bind(null, inst));
                    } else setStatus(inst, t(inst.ctx, 'oauth_failed'), true);
                };
                window.addEventListener('message', oauthHandler);
            }
            try {
                const created = await inst.api.createAccount(payload);
                form.querySelectorAll('input[type=password]').forEach(input => input.value = '');
                if (oauthRequired) {
                    const auth = await inst.api.startOAuth({ account_id: created.account_id });
                    authURL = auth.auth_url;
                    if (popup && !popup.closed) popup.location.href = auth.auth_url;
                }
                await reloadState(inst);
                if (authURL && (!popup || popup.closed)) {
                    const note = contentMessage(inst, t(inst.ctx, 'oauth_popup_blocked'));
                    const link = document.createElement('a'); link.href = authURL; link.target = '_blank'; link.rel = 'noopener noreferrer'; link.textContent = t(inst.ctx, 'open_authorize_window'); note.append(' ', link);
                }
                if (oauthRequired) setStatus(inst, t(inst.ctx, popup && !popup.closed ? 'oauth_authorizing' : 'account_created_connect_required'));
                else setStatus(inst, created.connected === false ? t(inst.ctx, 'account_created_connect_required') : t(inst.ctx, 'account_created'));
                if (!oauthRequired && created.connected === false && created.connection_error) contentMessage(inst, t(inst.ctx, 'account_connection_error'));
            } catch (error) { if (oauthHandler) window.removeEventListener('message', oauthHandler); if (popup && !popup.closed) popup.close(); form.querySelectorAll('input[type=password]').forEach(input => input.value = ''); showError(inst, error); }
        }

        function contentMessage(inst, message) {
            const node = document.createElement('p'); node.className = 'vd-mailer-oauth-status'; node.setAttribute('role', 'status'); node.textContent = message;
            inst.host.querySelector('[data-settings-content]').prepend(node); return node;
        }

        function slug(value) { return String(value).toLowerCase().replace(/[^a-z0-9]+/g, '-').replace(/^-|-$/g, '').slice(0, 64) || 'mail-account'; }

        function canConfigureAgentAccess(account) {
            const source = String(account.source || '').toLowerCase();
            return ['email', 'gog'].includes(source);
        }

        function editRecord(inst, recordId) {
            const kind = inst.records.kind;
            const item = recordId && typeof recordId === 'object' ? recordId : inst.records.items.find(row => String(row.id) === String(recordId)) || { kind, account_id: inst.accountId, name: '', subject: '', text: '', html: '', conditions: [], actions: [] };
            inst.records.editing = Object.assign({}, item);
            const content = inst.host.querySelector('[data-settings-content]');
            if (kind === 'rule') {
                const condition = item.conditions?.[0] || { field: 'subject', operator: 'contains', value: '' };
                const action = item.actions?.[0] || { kind: 'mark_read', destination_folder_id: '' };
                const selectedFolder = inst.folders.find(folder => String(folder.id) === String(action.destination_folder_id || '') || (action.label && String(folder.provider_id) === String(action.label)));
                content.innerHTML = `<form class="vd-mailer-record-form" data-record-form><h3>${esc(t(inst.ctx, 'rule_editor'))}</h3><label><span>${esc(t(inst.ctx, 'rule_name'))}</span><input name="name" value="${esc(item.name)}" required maxlength="120"></label><label><span>${esc(t(inst.ctx, 'account'))}</span><select name="account_id"><option value="">${esc(t(inst.ctx, 'all_accounts'))}</option>${inst.state.accounts.map(account => `<option value="${esc(account.id)}" ${item.account_id === account.id ? 'selected' : ''}>${esc(account.name || account.address)}</option>`).join('')}</select></label><fieldset><legend>${esc(t(inst.ctx, 'when_message'))}</legend><div class="vd-mailer-form-grid"><select name="field"><option value="from">${esc(t(inst.ctx, 'from'))}</option><option value="to">${esc(t(inst.ctx, 'to'))}</option><option value="subject" selected>${esc(t(inst.ctx, 'subject'))}</option><option value="body">${esc(t(inst.ctx, 'message_body'))}</option><option value="has_attachments">${esc(t(inst.ctx, 'has_attachments'))}</option></select><select name="operator"><option value="contains">${esc(t(inst.ctx, 'contains'))}</option><option value="equals">${esc(t(inst.ctx, 'equals'))}</option><option value="starts_with">${esc(t(inst.ctx, 'starts_with'))}</option></select><input data-condition-text name="condition_value" value="${esc(condition.value)}" required><select data-condition-boolean name="condition_boolean"><option value="true">${esc(t(inst.ctx, 'yes'))}</option><option value="false">${esc(t(inst.ctx, 'no'))}</option></select></div></fieldset><fieldset><legend>${esc(t(inst.ctx, 'then_action'))}</legend><div class="vd-mailer-form-grid"><select name="action"><option value="mark_read">${esc(t(inst.ctx, 'mark_read'))}</option><option value="mark_unread">${esc(t(inst.ctx, 'mark_unread'))}</option><option value="star">${esc(t(inst.ctx, 'star'))}</option><option value="unstar">${esc(t(inst.ctx, 'unstar'))}</option><option value="move">${esc(t(inst.ctx, 'move'))}</option><option value="archive">${esc(t(inst.ctx, 'archive'))}</option><option value="trash">${esc(t(inst.ctx, 'trash'))}</option><option value="label_add">${esc(t(inst.ctx, 'add_label'))}</option><option value="label_remove">${esc(t(inst.ctx, 'remove_label'))}</option><option value="snooze">${esc(t(inst.ctx, 'snooze'))}</option></select><select name="destination_folder_id"><option value="">${esc(t(inst.ctx, 'choose_folder'))}</option>${inst.folders.map(folder => `<option value="${esc(folder.id)}" data-folder-kind="${esc(folder.kind)}" data-provider-id="${esc(folder.provider_id || '')}" data-account-id="${esc(folder.account_id || '')}" ${selectedFolder?.id === folder.id ? 'selected' : ''}>${esc(folder.name)}</option>`).join('')}</select></div><label class="vd-mailer-rule-snooze" data-rule-snooze ${action.kind === 'snooze' ? '' : 'hidden'}><span>${esc(t(inst.ctx, 'snooze_until'))}</span><input name="snooze_until" type="datetime-local" value="${action.snooze_until ? localDateTime(new Date(action.snooze_until)) : ''}"></label></fieldset><div class="vd-mailer-form-actions"><button type="button" class="vd-mailer-button" data-cancel-account-form>${esc(t(inst.ctx, 'cancel'))}</button><button type="button" class="vd-mailer-button" data-rule-preview>${esc(t(inst.ctx, 'preview_rule'))}</button><button type="button" class="vd-mailer-button" data-rule-apply disabled>${esc(t(inst.ctx, 'apply_to_selection'))}</button><button type="button" class="vd-mailer-button vd-mailer-primary" data-save-record>${esc(t(inst.ctx, 'save'))}</button></div><div data-rule-preview-result role="status" aria-live="polite"></div></form>`;
                content.querySelector('[name=field]').value = condition.field || 'subject';
                const syncConditionField = () => {
                    const hasAttachments = content.querySelector('[name=field]').value === 'has_attachments';
                    content.querySelector('[data-condition-text]').hidden = hasAttachments;
                    content.querySelector('[data-condition-text]').required = !hasAttachments;
                    content.querySelector('[data-condition-boolean]').hidden = !hasAttachments;
                    const operator = content.querySelector('[name=operator]');
                    for (const option of operator.options) option.disabled = hasAttachments && option.value !== 'equals';
                    if (hasAttachments) operator.value = 'equals';
                };
                syncConditionField(); content.querySelector('[name=field]').addEventListener('change', syncConditionField);
                content.querySelector('[data-condition-boolean]').value = String(condition.value ?? true);
                content.querySelector('[name=operator]').value = condition.operator || 'contains';
                content.querySelector('[name=action]').value = action.kind;
                const syncRuleAction = () => {
                    const actionKind = content.querySelector('[name=action]').value;
                    content.querySelector('[data-rule-snooze]').hidden = actionKind !== 'snooze';
                    const destination = content.querySelector('[name=destination_folder_id]');
                    const needsFolder = ['move', 'label_add', 'label_remove'].includes(actionKind);
                    const accountID = content.querySelector('[name=account_id]').value;
                    destination.hidden = !needsFolder;
                    destination.disabled = needsFolder && !accountID;
                    destination.title = needsFolder && !accountID ? t(inst.ctx, 'select_account') : '';
                    for (const option of destination.options) {
                        if (!option.value) continue;
                        option.hidden = (['label_add', 'label_remove'].includes(actionKind) && option.dataset.folderKind !== 'label') || (!!accountID && option.dataset.accountId !== accountID);
                    }
                    if (destination.selectedOptions[0]?.hidden) destination.value = '';
                };
                content.querySelector('[name=action]').addEventListener('change', syncRuleAction);
                content.querySelector('[name=account_id]').addEventListener('change', syncRuleAction);
                content.querySelector('[name=destination_folder_id]').addEventListener('change', () => {
                    const selected = content.querySelector('[name=destination_folder_id]').selectedOptions[0];
                    const account = content.querySelector('[name=account_id]');
                    if (selected?.dataset.accountId && !account.value) account.value = selected.dataset.accountId;
                    syncRuleAction();
                });
                content.querySelector('[data-record-form]').addEventListener('input', invalidateRulePreview);
                content.querySelector('[data-record-form]').addEventListener('change', invalidateRulePreview);
                syncRuleAction();
                if (inst.state.readonly) content.querySelectorAll('input,select,textarea,button:not([data-cancel-account-form])').forEach(control => control.disabled = true);
                return;
            }
            content.innerHTML = `<form class="vd-mailer-record-form" data-record-form><h3>${esc(t(inst.ctx, kind === 'signature' ? 'signature_editor' : 'template_editor'))}</h3><label><span>${esc(t(inst.ctx, 'account'))}</span><select name="account_id"><option value="">${esc(t(inst.ctx, 'all_accounts'))}</option>${inst.state.accounts.map(account => `<option value="${esc(account.id)}" ${item.account_id === account.id ? 'selected' : ''}>${esc(account.name || account.address)}</option>`).join('')}</select></label><label><span>${esc(t(inst.ctx, 'name'))}</span><input name="name" value="${esc(item.name)}" required maxlength="120"></label>${kind === 'template' ? `<label><span>${esc(t(inst.ctx, 'subject'))}</span><input name="subject" value="${esc(item.subject)}" maxlength="998"></label>` : ''}<label><span>${esc(t(inst.ctx, 'message_body'))}</span><textarea name="text" rows="8">${esc(item.text || '')}</textarea></label><div class="vd-mailer-form-actions"><button type="button" class="vd-mailer-button" data-cancel-account-form>${esc(t(inst.ctx, 'cancel'))}</button><button type="button" class="vd-mailer-button vd-mailer-primary" data-save-record>${esc(t(inst.ctx, 'save'))}</button></div></form>`;
        }

        function activeRecordForm(inst) {
            const form = inst.host.querySelector('[data-record-form]');
            if (!form) return null;
            const data = new FormData(form);
            const editing = inst.records.editing || {};
            if (inst.records.kind === 'rule') {
                const kind = String(data.get('action') || '');
                const folder = inst.folders.find(item => String(item.id) === String(data.get('destination_folder_id') || ''));
                const action = { kind };
                if (kind === 'move') action.destination_folder_id = folder?.id || '';
                if (kind === 'label_add' || kind === 'label_remove') action.label = folder?.provider_id || folder?.id || '';
                if (kind === 'snooze' && data.get('snooze_until')) action.snooze_until = new Date(data.get('snooze_until')).toISOString();
                return Object.assign({}, editing, {
                    name: data.get('name'), account_id: data.get('account_id'),
                    conditions: [{ field: data.get('field'), operator: data.get('operator'), value: data.get('field') === 'has_attachments' ? String(data.get('condition_boolean') === 'true') : data.get('condition_value') }],
                    actions: [action]
                });
            }
            return Object.assign({}, editing, { kind: inst.records.kind, account_id: data.get('account_id'), name: data.get('name'), subject: data.get('subject') || '', text: data.get('text') || '', html: `<p>${esc(data.get('text') || '').replace(/\n/g, '<br>')}</p>` });
        }

        async function saveRecord(inst) {
            if (inst.state.readonly) return;
            try {
                const record = activeRecordForm(inst);
                const result = inst.records.kind === 'rule' ? await inst.api.saveRule(record) : await inst.api.saveTemplate(record);
                setStatus(inst, t(inst.ctx, 'saved'));
                if (inst.records.kind === 'rule') inst.settingsTab = 'rules';
                await paintSettings(inst);
            } catch (error) { showError(inst, error); }
        }

        async function deleteRecord(inst, recordId) {
            if (inst.state.readonly || !await inst.ctx.confirmDialog(t(inst.ctx, 'delete'), t(inst.ctx, 'delete_record_confirm'))) return;
            const record = inst.records.items.find(item => String(item.id) === String(recordId));
            if (!record) return;
            try {
                if (inst.records.kind === 'rule') await inst.api.deleteRule(record);
                else await inst.api.deleteTemplate(record);
                await paintSettings(inst);
            } catch (error) { showError(inst, error); }
        }

        function toggleRule(inst, ruleId, enabled) {
            if (inst.state.readonly) return;
            const rule = inst.records.items.find(item => item.id === ruleId);
            if (!rule) return;
            inst.api.saveRule(Object.assign({}, rule, { enabled })).then(() => paintSettings(inst)).catch(error => showError(inst, error));
        }

        function ruleReviewSignature(rule) {
            const condition = (rule.conditions || [])[0] || {};
            const action = (rule.actions || [])[0] || {};
            const snooze = action.kind === 'snooze' && action.snooze_until ? new Date(action.snooze_until).toISOString() : '';
            return JSON.stringify({
                id: rule.id || '', revision: Number(rule.revision || 0), account_id: rule.account_id || '', name: rule.name || '',
                condition: { field: condition.field || '', operator: condition.operator || '', value: String(condition.value ?? '') },
                action: { kind: action.kind || '', destination_folder_id: action.kind === 'move' ? action.destination_folder_id || '' : '', label: ['label_add', 'label_remove'].includes(action.kind) ? action.label || '' : '', snooze_until: snooze }
            });
        }

        function invalidateRulePreview(event) {
            const form = event.currentTarget;
            delete form.dataset.previewRevision;
            delete form.dataset.previewSignature;
            delete form.dataset.previewMessages;
            const apply = form.querySelector('[data-rule-apply]');
            if (apply) apply.disabled = true;
        }

        async function selectedMessageIDs(inst, threadIDs) {
            const details = await Promise.all(threadIDs.map(threadID => inst.api.thread(threadID)));
            return Array.from(new Set(details.flatMap(thread => (thread.messages || []).map(message => String(message.id)).filter(Boolean))));
        }

        async function previewRule(inst) {
            const form = inst.host.querySelector('[data-record-form]');
            try {
                const selectedThreads = Array.from(inst.selected);
                const messageIds = await selectedMessageIDs(inst, selectedThreads);
                if (!messageIds.length) { form.querySelector('[data-rule-preview-result]').textContent = t(inst.ctx, 'select_messages_for_rule'); return; }
                let rule = activeRecordForm(inst);
                rule = await inst.api.saveRule(rule);
                inst.records.editing = rule;
                editRecord(inst, rule);
                const currentForm = inst.host.querySelector('[data-record-form]');
                const preview = await inst.api.previewRule(rule, messageIds);
                if (Number(preview.revision) !== Number(rule.revision)) throw new Error(t(inst.ctx, 'request_failed'));
                const items = Array.isArray(preview.items) ? preview.items : [];
                const count = Number.isFinite(Number(preview.match_count)) ? Number(preview.match_count) : items.filter(item => item.matches).length;
                currentForm.dataset.previewRevision = String(preview.revision);
                currentForm.dataset.previewSignature = ruleReviewSignature(rule);
                currentForm.dataset.previewMessages = JSON.stringify(messageIds.slice().sort());
                currentForm.querySelector('[data-rule-apply]').disabled = false;
                currentForm.querySelector('[data-rule-preview-result]').textContent = t(inst.ctx, 'rule_preview_count', { count });
            } catch (error) { showError(inst, error); }
        }

        async function applyRule(inst) {
            const form = inst.host.querySelector('[data-record-form]');
            const rule = activeRecordForm(inst);
            const threadIDs = Array.from(inst.selected);
            try {
                const ids = await selectedMessageIDs(inst, threadIDs);
                const revision = Number(form?.dataset.previewRevision);
                if (!rule?.id || !ids.length || inst.state.readonly || !Number.isInteger(revision) || revision !== Number(rule.revision) || form.dataset.previewSignature !== ruleReviewSignature(rule) || form.dataset.previewMessages !== JSON.stringify(ids.slice().sort())) {
                    if (form) { invalidateRulePreview({ currentTarget: form }); form.querySelector('[data-rule-preview-result]').textContent = t(inst.ctx, 'preview_rule'); }
                    return;
                }
                await inst.api.applyRule(rule.id, ids, revision);
                setStatus(inst, t(inst.ctx, 'rule_applied'));
                await refreshCurrent(inst, true);
            }
            catch (error) { showError(inst, error); }
        }


        return {
            show: () => showSettings(inst),
            paint: () => paintSettings(inst),
            connectSource: sourceId => connectSource(inst, sourceId),
            createFolder: accountId => createFolder(inst, accountId),
            disconnectAccount: accountId => disconnectAccount(inst, accountId),
            testAccount: (accountId, direction) => testAccount(inst, accountId, direction),
            paintAccountForm: type => paintAccountForm(inst, type),
            saveAccountForm: () => saveAccountForm(inst),
            toggleEnabled: enabled => toggleEnabled(inst, enabled),
            saveRecord: () => saveRecord(inst),
            deleteRecord: recordId => deleteRecord(inst, recordId),
            editRecord: recordId => editRecord(inst, recordId),
            toggleRule: (ruleId, enabled) => toggleRule(inst, ruleId, enabled),
            previewRule: () => previewRule(inst),
            applyRule: () => applyRule(inst)
        };
    }

    window.MailerSettings = { create };
})();
