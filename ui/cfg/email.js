let emailAccountsCache = null;
let emailLoadError = null;
let emailLastSection = null;

async function emailLoadAccounts() {
    try {
        const resp = await fetch('/api/email-accounts');
        if (!resp.ok) {
            const txt = (await resp.text().catch(() => '')).trim();
            throw new Error(txt || resp.statusText || ('HTTP ' + resp.status));
        }
        const data = await resp.json();
        emailAccountsCache = Array.isArray(data) ? data : [];
        emailLoadError = null;
    } catch (e) {
        emailAccountsCache = null;
        emailLoadError = e.message || String(e);
    }
}

function emailRetryLoad() {
    emailAccountsCache = null;
    if (emailLastSection) renderEmailSection(emailLastSection);
}

async function renderEmailSection(section) {
    emailLastSection = section;
    const emailCfg = configData['email'] || {};
    if (emailAccountsCache === null) {
        await emailLoadAccounts();
    }
    const addDisabled = emailAccountsCache === null
        ? ` disabled title="${escapeAttr(t('config.email.load_failed_disabled'))}"` : '';
    let html = `<div class="cfg-section active">
        <div class="section-header">${section.label}</div>
        <div class="section-desc">${section.desc}</div>`;

    html += emailCheatsheetSelect(emailCfg.relay_cheatsheet_id || '');

    html += `<div class="cfg-actions-row">
            <button class="btn-save btn-secondary" onclick="emailAccountAdd()"${addDisabled}>
                ＋ ${t('config.email.new_account')}
            </button>
        </div>
        <div id="email-accounts-list"></div>
        <div id="email-accounts-empty" class="em-empty-state is-hidden">
            ${t('config.email.empty')}
        </div>
    </div>`;
    document.getElementById('content').innerHTML = html;
    attachChangeListeners();
    emailLoadCheatsheets(emailCfg.relay_cheatsheet_id || '');
    emailAccountRenderCards();
}

function emailCheatsheetSelect(selectedID) {
    let html = '<div class="field-group">';
    html += '<div class="field-label">' + t('config.email.relay_cheatsheet_label') + '</div>';
    html += '<div class="field-help">' + t('help.email.relay_cheatsheet_id') + '</div>';
    html += '<select class="field-select" id="email-relay-cheatsheet" data-path="email.relay_cheatsheet_id" data-selected="' + escapeAttr(selectedID || '') + '">';
    html += '<option value="">' + escapeHtml(t('config.email.loading')) + '</option>';
    html += '</select>';
    html += '</div>';
    return html;
}

function emailLoadCheatsheets(selectedID) {
    const select = document.getElementById('email-relay-cheatsheet');
    if (!select) return;
    fetch('/api/cheatsheets')
        .then(r => {
            if (!r.ok) throw new Error(r.statusText || 'Failed to load cheat sheets');
            return r.json();
        })
        .then(sheets => {
            const list = Array.isArray(sheets) ? sheets.slice() : [];
            list.sort((a, b) => String(a.name || '').localeCompare(String(b.name || '')));
            let html = '<option value="">' + escapeHtml(t('config.email.relay_cheatsheet_none')) + '</option>';
            if (selectedID && !list.some(s => s.id === selectedID)) {
                html += '<option value="' + escapeAttr(selectedID) + '">' + escapeHtml(selectedID) + '</option>';
            }
            list.forEach(sheet => {
                html += '<option value="' + escapeAttr(sheet.id) + '">' + escapeHtml(sheet.name || sheet.id) + '</option>';
            });
            if (list.length === 0 && !selectedID) {
                html += '<option value="" disabled>' + escapeHtml(t('config.email.relay_cheatsheet_empty')) + '</option>';
            }
            select.innerHTML = html;
            select.value = selectedID || '';
        })
        .catch(() => {
            select.innerHTML = '<option value="' + escapeAttr(selectedID || '') + '">' + escapeHtml(t('config.email.relay_cheatsheet_error')) + '</option>';
            select.value = selectedID || '';
        });
}

function emailAccountRenderCards() {
    const wrap = document.getElementById('email-accounts-list');
    const empty = document.getElementById('email-accounts-empty');
    if (!wrap) return;
    if (emailAccountsCache === null) {
        wrap.innerHTML = `<div class="wh-notice mcp-notice-danger" role="alert">
            <span>⚠️</span>
            <div>
                <strong>${escapeHtml(t('config.email.load_failed'))}</strong><br>
                <small>${escapeHtml(emailLoadError || '')}</small>
            </div>
            <button type="button" class="btn-save btn-secondary" onclick="emailRetryLoad()">${escapeHtml(t('config.email.retry'))}</button>
        </div>`;
        if (empty) empty.classList.add('is-hidden');
        return;
    }
    if (emailAccountsCache.length === 0) {
        wrap.innerHTML = '';
        if (empty) empty.classList.remove('is-hidden');
        return;
    }
    if (empty) empty.classList.add('is-hidden');

    let html = '';
    emailAccountsCache.forEach((a, idx) => {
        const watchBadge = a.watch_enabled
            ? `<span class="em-badge em-badge-success">👁️ Watcher</span>`
            : '';
        const enabledBadge = (a.enabled === false)
            ? `<span class="em-badge em-badge-danger">⏸ ${t('config.email.disabled')}</span>`
            : `<span class="em-badge em-badge-success">✔ ${t('config.email.enabled')}</span>`;
        const sendBadge = (a.allow_sending === false)
            ? `<span class="em-badge em-badge-warning">📖 ${t('config.email.read_only')}</span>`
            : '';
        const maskedPw = a.password === '••••••••'
            ? `<span class="em-pw-set">✔ ${t('config.email.password_set')}</span>`
            : (a.password ? `<span class="em-pw-set">✔ ${t('config.email.password_set')}</span>` : '<span class="em-pw-unset">—</span>');

        html += `
        <div class="em-card" data-idx="${idx}">
            <div class="em-card-header">
                <div class="em-card-title">
                    ${escapeAttr(a.name || a.id)}${enabledBadge}${sendBadge}${watchBadge}
                    <span class="em-card-id">ID: ${escapeAttr(a.id)}</span>
                </div>
                <div class="em-card-actions">
                    <button onclick="emailAccountEdit(${idx})" class="em-card-btn em-card-btn-edit" title="${t('config.email.edit_tooltip')}">✏️</button>
                    <button onclick="emailAccountDelete(${idx})" class="em-card-btn em-card-btn-delete" title="${t('config.email.delete_tooltip')}">🗑️</button>
                </div>
            </div>
            <div class="em-card-grid">
                <div><span class="em-grid-label">IMAP:</span> ${escapeAttr(a.imap_host || '—')}:${a.imap_port || '—'}</div>
                <div><span class="em-grid-label">SMTP:</span> ${escapeAttr(a.smtp_host || '—')}:${a.smtp_port || '—'}</div>
                <div><span class="em-grid-label">${t('config.email.user')}:</span> ${escapeAttr(a.username || '—')}</div>
                <div><span class="em-grid-label">${t('config.email.password')}:</span> ${maskedPw}</div>
                <div><span class="em-grid-label">${t('config.email.from_label')}:</span> ${escapeAttr(a.from_address || '—')}</div>
                <div><span class="em-grid-label">${t('config.email.folder')}:</span> ${escapeAttr(a.watch_folder || 'INBOX')}</div>
            </div>
        </div>`;
    });
    wrap.innerHTML = html;
}

function emailAccountShowModal(title, data, onSave) {
    const existing = document.getElementById('email-modal-overlay');
    if (existing) existing.remove();

    const overlay = document.createElement('div');
    overlay.id = 'email-modal-overlay';
    overlay.className = 'modal-overlay open active';
    overlay.onclick = (e) => { if (e.target === overlay) overlay.remove(); };

    overlay.innerHTML = `
    <div class="modal-card email-modal-card" onclick="event.stopPropagation()">
        <div class="modal-header">
            <h2>${title}</h2>
            <button type="button" class="modal-close" onclick="document.getElementById('email-modal-overlay').remove()" aria-label="${t('config.email.cancel')}">&times;</button>
        </div>
        <div class="field-group">
            <div class="field-label">${t('config.email.field_id')}</div>
            <div class="field-help">${t('config.email.id_help')}</div>
            <input class="field-input${data._editMode ? ' is-disabled' : ''}" id="ea-id" value="${escapeAttr(data.id || '')}" placeholder="personal" ${data._editMode ? 'disabled' : ''}>
        </div>
        <div class="field-group">
            <div class="field-label">${t('config.email.field_name')}</div>
            <div class="field-help">${t('config.email.name_help')}</div>
            <input class="field-input" id="ea-name" value="${escapeAttr(data.name || '')}" placeholder="${t('config.email.display_name')}">
        </div>

        <div class="em-section-divider">
            <div class="em-section-subtitle">⚙️ ${t('config.email.account_settings')}</div>
        </div>
        <div class="field-group em-toggle-group">
            <div class="em-flex-1">
                <div class="field-label em-toggle-label-mb">${t('config.email.account_enabled_label')}</div>
                <div class="field-help em-toggle-help-m0">${t('config.email.account_enabled_help')}</div>
            </div>
            <label class="toggle-switch">
                <input type="checkbox" id="ea-enabled" ${(data.enabled !== false) ? 'checked' : ''}>
                <span class="toggle-slider"></span>
            </label>
        </div>
        <div class="field-group em-toggle-group">
            <div class="em-flex-1">
                <div class="field-label em-toggle-label-mb">${t('config.email.allow_sending_label')}</div>
                <div class="field-help em-toggle-help-m0">${t('config.email.allow_sending_help')}</div>
            </div>
            <label class="toggle-switch">
                <input type="checkbox" id="ea-allow-sending" ${(data.allow_sending !== false) ? 'checked' : ''}>
                <span class="toggle-slider"></span>
            </label>
        </div>

        <div class="em-section-divider">
            <div class="em-section-subtitle">📥 ${t('config.email.imap_receive')}</div>
        </div>
        <div class="field-group">
            <div class="field-label">${t('config.email.imap_host')}</div>
            <div class="field-help">${t('config.email.imap_host_help')}</div>
            <input class="field-input" id="ea-imap-host" value="${escapeAttr(data.imap_host || '')}" placeholder="imap.example.com">
        </div>
        <div class="field-group">
            <div class="field-label">${t('config.email.imap_port')}</div>
            <div class="field-help">${t('config.email.imap_port_help')}</div>
            <input class="field-input" id="ea-imap-port" type="number" value="${data.imap_port || 993}" placeholder="993">
        </div>

        <div class="em-section-divider">
            <div class="em-section-subtitle">📤 ${t('config.email.smtp_send')}</div>
        </div>
        <div class="field-group">
            <div class="field-label">${t('config.email.smtp_host')}</div>
            <div class="field-help">${t('config.email.smtp_host_help')}</div>
            <input class="field-input" id="ea-smtp-host" value="${escapeAttr(data.smtp_host || '')}" placeholder="smtp.example.com">
        </div>
        <div class="field-group">
            <div class="field-label">${t('config.email.smtp_port')}</div>
            <div class="field-help">${t('config.email.smtp_port_help')}</div>
            <input class="field-input" id="ea-smtp-port" type="number" value="${data.smtp_port || 587}" placeholder="587">
        </div>

        <div class="em-section-divider">
            <div class="em-section-subtitle">🔑 ${t('config.email.credentials')}</div>
        </div>
        <div class="field-group">
            <div class="field-label">${t('config.email.username')}</div>
            <div class="field-help">${t('config.email.username_help')}</div>
            <input class="field-input" id="ea-username" value="${escapeAttr(data.username || '')}" placeholder="user@example.com">
        </div>
        <div class="field-group">
            <div class="field-label">${t('config.email.password')}</div>
            <div class="field-help">${t('config.email.password_help')}</div>
            <div class="password-wrap">
                <input class="field-input" id="ea-password" type="password" value="${escapeAttr(cfgSecretValue(data.password))}" placeholder="${escapeAttr(cfgSecretPlaceholder(data.password, ''))}" autocomplete="off">
                <button type="button" class="password-toggle" data-visible="false" onclick="togglePassword(this)">${EYE_OPEN_SVG}</button>
            </div>
            ${cfgIsMaskedSecret(data.password) ? `<div class="em-pw-hint">${t('config.email.keep_password')}</div>` : ''}
        </div>
        <div class="field-group">
            <div class="field-label">${t('config.email.from_address')}</div>
            <div class="field-help">${t('config.email.from_help')}</div>
            <input class="field-input" id="ea-from" value="${escapeAttr(data.from_address || '')}" placeholder="user@example.com">
        </div>

        <div class="em-section-divider">
            <div class="em-section-subtitle">👁️ ${t('config.email.inbox_watcher')}</div>
        </div>
        <div class="field-group em-toggle-group">
            <div class="field-label em-toggle-label-mb">${t('config.email.watch_enabled')}</div>
            <div class="field-help em-toggle-help-m0">${t('config.email.watch_enabled_help')}</div>
            <label class="toggle-switch">
                <input type="checkbox" id="ea-watch-enabled" ${data.watch_enabled ? 'checked' : ''}>
                <span class="toggle-slider"></span>
            </label>
        </div>
        <div class="field-group">
            <div class="field-label">${t('config.email.watch_folder')}</div>
            <div class="field-help">${t('config.email.watch_folder_help')}</div>
            <input class="field-input" id="ea-watch-folder" value="${escapeAttr(data.watch_folder || 'INBOX')}" placeholder="INBOX">
        </div>
        <div class="field-group">
            <div class="field-label">${t('config.email.interval_seconds')}</div>
            <div class="field-help">${t('config.email.interval_seconds_help')}</div>
            <input class="field-input" id="ea-watch-interval" type="number" value="${data.watch_interval_seconds || 120}" placeholder="120">
        </div>

        ${cfgActionResult.html('ea-test-result')}
        <div id="ea-save-error" class="wh-notice mcp-notice-danger is-hidden" role="alert"></div>
        <div class="modal-actions">
            <button type="button" class="btn btn-secondary" onclick="document.getElementById('email-modal-overlay').remove()">
                ${t('config.email.cancel')}
            </button>
            <button type="button" class="btn-save adg-test-btn" id="ea-test-btn" data-config-action-draft="true">
                🔌 ${t('config.email.test_btn')}
            </button>
            <button type="button" class="btn-save" id="ea-save-btn">
                ${t('config.email.save')}
            </button>
        </div>
    </div>`;
    document.body.appendChild(overlay);

    document.getElementById('ea-test-btn').onclick = () => emailAccountTestFromModal(data);

    document.getElementById('ea-save-btn').onclick = async () => {
        const errBox = document.getElementById('ea-save-error');
        const saveBtn = document.getElementById('ea-save-btn');
        const showErr = msg => { errBox.textContent = msg; errBox.classList.remove('is-hidden'); };
        errBox.classList.add('is-hidden');
        const id = document.getElementById('ea-id').value.trim().toLowerCase().replace(/[^a-z0-9_-]/g, '');
        const name = document.getElementById('ea-name').value.trim();
        const enabled = document.getElementById('ea-enabled').checked;
        const allow_sending = document.getElementById('ea-allow-sending').checked;
        const imap_host = document.getElementById('ea-imap-host').value.trim();
        const imap_port = parseInt(document.getElementById('ea-imap-port').value, 10) || 993;
        const smtp_host = document.getElementById('ea-smtp-host').value.trim();
        const smtp_port = parseInt(document.getElementById('ea-smtp-port').value, 10) || 587;
        const username = document.getElementById('ea-username').value.trim();
        let password = document.getElementById('ea-password').value.trim();
        if (!password && cfgIsMaskedSecret(data.password)) password = CFG_MASKED_SECRET;
        const from_address = document.getElementById('ea-from').value.trim();
        const watch_enabled = document.getElementById('ea-watch-enabled').checked;
        const watch_folder = document.getElementById('ea-watch-folder').value.trim() || 'INBOX';
        const watch_interval_seconds = parseInt(document.getElementById('ea-watch-interval').value, 10) || 120;

        if (!id) { showErr(t('config.email.id_empty')); return; }
        if (!imap_host && !smtp_host) { showErr(t('config.email.host_required')); return; }

        const entry = { id, name: name || id, enabled, allow_sending, imap_host, imap_port, smtp_host, smtp_port, username, password, from_address, watch_enabled, watch_folder, watch_interval_seconds };
        saveBtn.disabled = true;
        const res = await onSave(entry);
        saveBtn.disabled = false;
        if (res && res.ok === false) {
            showErr(res.error || t('config.common.error'));
            return;
        }
        overlay.remove();
    };

    setTimeout(() => {
        const focus = data._editMode ? document.getElementById('ea-name') : document.getElementById('ea-id');
        if (focus) focus.focus();
    }, 50);
}

function emailAccountAdd() {
    if (emailAccountsCache === null) { showToast(t('config.email.load_failed_disabled'), 'warn'); return; }
    emailAccountShowModal(
        t('config.email.new_account'),
        {},
        async (entry) => {
            if (emailAccountsCache === null) return { ok: false, error: t('config.email.load_failed_disabled') };
            if (emailAccountsCache.some(a => a.id === entry.id)) {
                return { ok: false, error: t('config.email.id_exists') };
            }
            return emailAccountSave(emailAccountsCache.concat([entry]));
        }
    );
}

function emailAccountEdit(idx) {
    if (emailAccountsCache === null) return;
    const a = { ...emailAccountsCache[idx], _editMode: true };
    emailAccountShowModal(
        t('config.email.edit_account'),
        a,
        async (entry) => {
            if (emailAccountsCache === null) return { ok: false, error: t('config.email.load_failed_disabled') };
            const next = emailAccountsCache.slice();
            next[idx] = entry;
            return emailAccountSave(next);
        }
    );
}

async function emailAccountDelete(idx) {
    if (emailAccountsCache === null) return;
    const a = emailAccountsCache[idx];
    if (!await showConfirm(t('config.email.delete_confirm', {name: a.name || a.id}))) return;
    const next = emailAccountsCache.slice();
    next.splice(idx, 1);
    const res = await emailAccountSave(next);
    if (!res.ok) showToast(res.error, 'error');
}

function emailAccountCollectModalFields(data) {
    const id = document.getElementById('ea-id').value.trim().toLowerCase().replace(/[^a-z0-9_-]/g, '');
    const imap_host = document.getElementById('ea-imap-host').value.trim();
    const imap_port = parseInt(document.getElementById('ea-imap-port').value, 10) || 993;
    const smtp_host = document.getElementById('ea-smtp-host').value.trim();
    const smtp_port = parseInt(document.getElementById('ea-smtp-port').value, 10) || 587;
    const username = document.getElementById('ea-username').value.trim();
    let password = document.getElementById('ea-password').value.trim();
    if (!password && cfgIsMaskedSecret(data.password)) password = CFG_MASKED_SECRET;
    return { id, imap_host, imap_port, smtp_host, smtp_port, username, password };
}

// Tests the account as typed in the modal (nothing is saved). partial = warning.
async function emailAccountTestFromModal(data) {
    const fields = emailAccountCollectModalFields(data);
    if (!fields.username) {
        cfgActionResult.show('ea-test-result', 'error', t('config.email.test_credentials_required'));
        return { skipped: 'invalid' };
    }
    if (!fields.imap_host && !fields.smtp_host) {
        cfgActionResult.show('ea-test-result', 'error', t('config.email.host_required'));
        return { skipped: 'invalid' };
    }
    return cfgActionResult(document.getElementById('ea-test-btn'), () => fetch('/api/email-accounts/test', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(fields)
    }), {
        target: 'ea-test-result',
        pending: t('config.email.testing'),
        success: outcome => outcome.reason || t('config.email.test_ok'),
        warning: outcome => outcome.reason ? '' : t('config.email.test_partial'),
        failure: t('config.email.test_fail')
    });
}

async function emailAccountSave(nextAccounts) {
    if (emailAccountsCache === null || !Array.isArray(nextAccounts)) {
        return { ok: false, error: t('config.email.load_failed_disabled') };
    }
    try {
        const resp = await fetch('/api/email-accounts', {
            method: 'PUT',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify(nextAccounts)
        });
        if (!resp.ok) {
            const txt = (await resp.text().catch(() => '')).trim();
            return { ok: false, error: txt || t('config.common.error') };
        }
        emailAccountsCache = nextAccounts;
        try {
            const reload = await fetch('/api/email-accounts');
            if (reload.ok) emailAccountsCache = await reload.json();
        } catch (_) { /* keep saved list */ }
        emailAccountRenderCards();
        showToast(t('config.email.saved'), 'success');
        return { ok: true };
    } catch (e) {
        return { ok: false, error: e.message || t('config.common.error') };
    }
}
