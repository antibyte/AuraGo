let emailAccountsCache = null;

async function renderEmailSection(section) {
    if (emailAccountsCache === null) {
        try {
            const resp = await fetch('/api/email-accounts');
            emailAccountsCache = resp.ok ? await resp.json() : [];
        } catch (_) { emailAccountsCache = []; }
    }
    let html = `<div class="cfg-section active">
        <div class="section-header">${section.icon} ${section.label}</div>
        <div class="section-desc">${section.desc}</div>`;

    html += `<div class="em-action-row">
            <button class="btn-save cfg-save-btn-sm" onclick="emailAccountAdd()">
                ＋ ${t('config.email.new_account')}
            </button>
        </div>
        <div id="email-accounts-list"></div>
        <div id="email-accounts-empty" class="em-empty-state is-hidden">
            ${t('config.email.empty')}
        </div>
    </div>`;
    document.getElementById('content').innerHTML = html;
    emailAccountRenderCards();
}

function emailAccountRenderCards() {
    const wrap = document.getElementById('email-accounts-list');
    const empty = document.getElementById('email-accounts-empty');
    if (!wrap) return;
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
    overlay.className = 'em-modal-overlay';
    overlay.onclick = (e) => { if (e.target === overlay) overlay.remove(); };

    overlay.innerHTML = `
    <div class="em-modal-box" onclick="event.stopPropagation()">
        <div class="em-modal-header">
            <div class="em-modal-title">${title}</div>
            <button onclick="document.getElementById('email-modal-overlay').remove()" class="em-modal-close">✕</button>
        </div>
        <div class="field-group">
            <div class="field-label">${t('config.email.field_id')}</div>
            <div class="field-help">${t('config.email.id_help')}</div>
            <input class="field-input${data._editMode ? ' is-disabled' : ''}" id="ea-id" value="${escapeAttr(data.id || '')}" placeholder="personal" ${data._editMode ? 'disabled' : ''}>
        </div>
        <div class="field-group">
            <div class="field-label">${t('config.email.field_name')}</div>
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
            <input class="field-input" id="ea-username" value="${escapeAttr(data.username || '')}" placeholder="user@example.com">
        </div>
        <div class="field-group">
            <div class="field-label">${t('config.email.password')}</div>
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
            <label class="toggle-switch">
                <input type="checkbox" id="ea-watch-enabled" ${data.watch_enabled ? 'checked' : ''}>
                <span class="toggle-slider"></span>
            </label>
        </div>
        <div class="field-group">
            <div class="field-label">${t('config.email.watch_folder')}</div>
            <input class="field-input" id="ea-watch-folder" value="${escapeAttr(data.watch_folder || 'INBOX')}" placeholder="INBOX">
        </div>
        <div class="field-group">
            <div class="field-label">${t('config.email.interval_seconds')}</div>
            <input class="field-input" id="ea-watch-interval" type="number" value="${data.watch_interval_seconds || 120}" placeholder="120">
        </div>

        <div class="em-modal-footer">
            <button class="btn-save em-btn-cancel" onclick="document.getElementById('email-modal-overlay').remove()">
                ${t('config.email.cancel')}
            </button>
            <button class="btn-save em-btn-save" id="ea-save-btn">
                ${t('config.email.save')}
            </button>
        </div>
    </div>`;
    document.body.appendChild(overlay);

    document.getElementById('ea-save-btn').onclick = () => {
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

        if (!id) { showToast(t('config.email.id_empty'), 'warn'); return; }
        if (!imap_host && !smtp_host) { showToast(t('config.email.host_required'), 'warn'); return; }

        const entry = { id, name: name || id, enabled, allow_sending, imap_host, imap_port, smtp_host, smtp_port, username, password, from_address, watch_enabled, watch_folder, watch_interval_seconds };
        onSave(entry);
        overlay.remove();
    };

    setTimeout(() => {
        const focus = data._editMode ? document.getElementById('ea-name') : document.getElementById('ea-id');
        if (focus) focus.focus();
    }, 50);
}

function emailAccountAdd() {
    emailAccountShowModal(
        t('config.email.new_account'),
        {},
        async (entry) => {
            if (emailAccountsCache.some(a => a.id === entry.id)) {
                showToast(t('config.email.id_exists'), 'warn');
                return;
            }
            emailAccountsCache.push(entry);
            await emailAccountSave();
        }
    );
}

function emailAccountEdit(idx) {
    const a = { ...emailAccountsCache[idx], _editMode: true };
    emailAccountShowModal(
        t('config.email.edit_account'),
        a,
        async (entry) => {
            emailAccountsCache[idx] = entry;
            await emailAccountSave();
        }
    );
}

async function emailAccountDelete(idx) {
    const a = emailAccountsCache[idx];
    if (!await showConfirm(t('config.email.delete_confirm', {name: a.name || a.id}))) return;
    emailAccountsCache.splice(idx, 1);
    await emailAccountSave();
}

async function emailAccountSave() {
    try {
        const resp = await fetch('/api/email-accounts', {
            method: 'PUT',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify(emailAccountsCache)
        });
        if (!resp.ok) {
            const txt = await resp.text();
            showToast(txt || t('config.common.error'), 'error');
            return;
        }
        const reload = await fetch('/api/email-accounts');
        if (reload.ok) emailAccountsCache = await reload.json();
        emailAccountRenderCards();
    } catch (e) {
        showToast(e.message || t('config.common.error'), 'error');
    }
}
