// cfg/backup.js — Backup & Restore section module

// Maps the old tone classes onto the shared result line states.
function backupSetStatus(el, tone, text) {
    if (!el) return;
    const state = { 'is-success': 'success', 'is-danger': 'error', 'is-warning': 'warning', 'is-muted': 'pending' }[tone] || 'pending';
    cfgActionResult.show(el, state, text);
}

function backupSetVaultHintState(hint, hasPassword) {
    if (!hint) return;
    hint.classList.toggle('is-success', hasPassword);
    hint.classList.toggle('is-warning', !hasPassword);
    hint.textContent = hasPassword
        ? t('config.backup.vault_included_hint')
        : t('config.backup.no_password_warning');
}

function renderBackupSection(section) {
    let html = '<div class="cfg-section active">';
    html += '<div class="section-header">' + section.label + '</div>';
    html += '<div class="section-desc">' + section.desc + '</div>';

    html += `
    <!-- Tabs -->
    <div class="wh-tabs backup-tabs">
        <button class="wh-tab active" id="backup-tab-create" onclick="backupShowTab('create')">
            ${t('config.backup.tab_create')}
        </button>
        <button class="wh-tab" id="backup-tab-import" onclick="backupShowTab('import')">
            ${t('config.backup.tab_import')}
        </button>
    </div>

    <!-- Create panel -->
    <div class="wh-panel active backup-panel" id="backup-panel-create">

        <div class="bck-card">
            <div class="bck-card-header">
                <span class="bck-card-icon">💾</span>
                <span class="bck-card-title">${t('config.backup.card_create_title')}</span>
            </div>
            <div class="bck-card-body">
                <div class="backup-note-card">
                    <div class="backup-note-text">
                        ${t('config.backup.create_desc')}
                    </div>
                </div>

                <div class="backup-options-list">
                    <label class="backup-option-row">
                        <input type="checkbox" id="backup-opt-vectordb" class="backup-check-input" aria-describedby="backup-opt-vectordb-help">
                        <span>${t('config.backup.include_vectordb')}<span class="field-help" id="backup-opt-vectordb-help">${t('config.backup.include_vectordb_help')}</span></span>
                    </label>
                    <label class="backup-option-row">
                        <input type="checkbox" id="backup-opt-workdir" class="backup-check-input" aria-describedby="backup-opt-workdir-help">
                        <span>${t('config.backup.include_workdir')}<span class="field-help" id="backup-opt-workdir-help">${t('config.backup.include_workdir_help')}</span></span>
                    </label>
                </div>
            </div>
        </div>

        <div class="bck-card">
            <div class="bck-card-header">
                <span class="bck-card-icon">🔐</span>
                <span class="bck-card-title">${t('config.backup.card_encryption_title')}</span>
            </div>
            <div class="bck-card-body">
                <div class="field-group backup-field-group">
                    <div class="field-label">${t('config.backup.password_label')}</div>
                    <div class="field-help backup-field-hint">${t('config.backup.password_hint')}</div>
                    <div class="password-wrap">
                        <input type="password" id="backup-password" class="field-input" placeholder="${t('config.backup.encryption_placeholder')}" autocomplete="new-password" oninput="backupUpdateVaultHint()">
                        <button type="button" class="password-toggle" onclick="togglePassword(this)" title="${escapeAttr(t('config.backup.toggle_visibility'))}">${EYE_OPEN_SVG}</button>
                    </div>
                    <div id="backup-vault-hint" class="backup-vault-hint is-warning">${t('config.backup.no_password_warning')}</div>
                </div>
            </div>
        </div>

        <div class="backup-actions-row">
            <button class="btn-save backup-primary-btn" onclick="backupCreate()" id="backup-create-btn">
                ${t('config.backup.create_download')}
            </button>
            ${cfgActionResult.html('backup-create-status', 'backup-status-text')}
        </div>
    </div>

    <!-- Import panel -->
    <div class="wh-panel backup-panel" id="backup-panel-import">

        <div class="bck-card">
            <div class="bck-card-header">
                <span class="bck-card-icon">📥</span>
                <span class="bck-card-title">${t('config.backup.card_import_title')}</span>
            </div>
            <div class="bck-card-body">
                <div class="backup-warning-box">
                    ${t('config.backup.import_warning')}
                </div>

                <div class="field-group backup-field-group-file">
                    <div class="field-label">${t('config.backup.select_file')}</div>
                    <div class="field-help backup-field-hint">${t('config.backup.select_file_help')}</div>
                    <input type="file" id="backup-import-file" accept=".ago" class="field-input backup-file-input">
                </div>

                <div class="field-group backup-field-group">
                    <div class="field-label">${t('config.backup.import_password_label')}</div>
                    <div class="field-help backup-field-hint">${t('config.backup.import_password_help')}</div>
                    <div class="password-wrap">
                        <input type="password" id="backup-import-password" class="field-input" placeholder="${t('config.backup.decryption_placeholder')}" autocomplete="off">
                        <button type="button" class="password-toggle" onclick="togglePassword(this)" title="${escapeAttr(t('config.backup.toggle_visibility'))}">${EYE_OPEN_SVG}</button>
                    </div>
                </div>
            </div>
        </div>

        <div class="backup-actions-row">
            <button class="btn-save backup-import-btn" onclick="backupImport()" id="backup-import-btn">
                ${t('config.backup.import_button')}
            </button>
            ${cfgActionResult.html('backup-import-status', 'backup-status-text')}
        </div>

        <div id="backup-import-result" class="backup-import-result is-hidden"></div>
    </div>`;

    html += '</div>';
    document.getElementById('content').innerHTML = html;
}

function backupUpdateVaultHint() {
    const pw = document.getElementById('backup-password');
    const hint = document.getElementById('backup-vault-hint');
    if (!pw || !hint) return;
    backupSetVaultHintState(hint, Boolean(pw.value));
}

function backupShowTab(tab) {
    ['create', 'import'].forEach(tabId => {
        document.getElementById('backup-tab-' + tabId).classList.toggle('active', tabId === tab);
        document.getElementById('backup-panel-' + tabId).classList.toggle('active', tabId === tab);
    });
}

function backupCreate() {
    const btn = document.getElementById('backup-create-btn');

    const payload = {
        password: document.getElementById('backup-password').value || '',
        include_vectordb: document.getElementById('backup-opt-vectordb').checked,
        include_workdir: document.getElementById('backup-opt-workdir').checked,
    };

    // The archive is a binary download: run() saves it and hands the result line a small body.
    return cfgActionResult(btn, async () => {
        const resp = await fetch('/api/backup/create', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify(payload)
        });
        if (!resp.ok) return resp;
        const disposition = resp.headers.get('Content-Disposition') || '';
        const fnMatch = disposition.match(/filename="([^"]+)"/);
        const filename = fnMatch ? fnMatch[1] : 'backup.ago';
        const blob = await resp.blob();
        const url = URL.createObjectURL(blob);
        const a = document.createElement('a');
        a.href = url;
        a.download = filename;
        document.body.appendChild(a);
        a.click();
        document.body.removeChild(a);
        URL.revokeObjectURL(url);
        return { ok: true, filename: filename };
    }, {
        target: 'backup-create-status',
        pending: t('config.backup.creating'),
        success: outcome => t('config.backup.status_success') + t('config.backup.downloaded') + (outcome.body && outcome.body.filename ? outcome.body.filename : ''),
        failure: t('config.backup.status_error')
    });
}

// Server error codes of the import endpoint, shown as readable reasons.
function backupImportReason(body) {
    if (!body) return '';
    if (body.error === 'password_required') return t('config.backup.password_required');
    if (body.error === 'decryption_failed') return t('config.backup.decryption_failed');
    return body.message || body.error || t('config.backup.unknown_error');
}

async function backupImport() {
    const btn = document.getElementById('backup-import-btn');
    const status = document.getElementById('backup-import-status');
    const result = document.getElementById('backup-import-result');
    const fileInput = document.getElementById('backup-import-file');

    if (!fileInput.files || !fileInput.files[0]) {
        backupSetStatus(status, 'is-warning', t('config.backup.select_file_warning'));
        return { skipped: 'no-file' };
    }

    if (!(await showConfirm(t('config.backup.confirm_import_title'), t('config.backup.confirm_import_message', { name: fileInput.files[0].name })))) return { skipped: 'cancelled' };

    result.classList.add('is-hidden');

    const fd = new FormData();
    fd.append('file', fileInput.files[0]);
    fd.append('password', document.getElementById('backup-import-password').value || '');

    return cfgActionResult(btn, () => fetch('/api/backup/import', { method: 'POST', body: fd }), {
        target: 'backup-import-status',
        pending: t('config.backup.importing'),
        success: t('config.backup.import_success'),
        failure: t('config.backup.status_error'),
        reasonText: body => backupImportReason(body && (body.error || body.message) ? body : null),
        onSuccess: outcome => {
            const data = outcome.body || {};
            result.classList.remove('is-hidden');
            result.innerHTML = `<div class="backup-import-success-box">
                <div class="backup-import-success-title">${t('config.backup.status_success')} ${t('config.backup.import_success')}</div>
                <div class="backup-import-success-message">${escapeHtml(data.message || '')}</div>
                <button class="btn-save backup-restart-btn" onclick="restartAuraGo()">
                    ↻ ${t('config.backup.restart_now')}
                </button>
            </div>`;
        }
    });
}
