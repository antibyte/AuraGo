// cfg/backup.js — Backup & Restore section module

function backupSetStatus(el, tone, text) {
    if (!el) return;
    el.classList.remove('is-success', 'is-danger', 'is-warning', 'is-muted');
    el.classList.add(tone);
    el.textContent = text;
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
    html += '<div class="section-header">' + section.icon + ' ' + section.label + '</div>';
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
        <div class="backup-note-card">
            <div class="backup-note-text">
                ${t('config.backup.create_desc')}
            </div>
        </div>

        <div class="backup-options-list">
            <label class="backup-option-row">
                <input type="checkbox" id="backup-opt-vectordb" class="backup-check-input">
                <span>${t('config.backup.include_vectordb')}</span>
            </label>
            <label class="backup-option-row">
                <input type="checkbox" id="backup-opt-workdir" class="backup-check-input">
                <span>${t('config.backup.include_workdir')}</span>
            </label>
        </div>

        <div class="field-group backup-field-group">
            <div class="field-label">${t('config.backup.password_label')}</div>
            <div class="field-hint backup-field-hint">${t('config.backup.password_hint')}</div>
            <div class="password-wrap">
                <input type="password" id="backup-password" class="field-input" placeholder="${t('config.backup.encryption_placeholder')}" autocomplete="new-password" oninput="backupUpdateVaultHint()">
                <button type="button" class="password-toggle" onclick="(function(b){var i=document.getElementById('backup-password');i.type=i.type==='password'?'text':'password';b.innerHTML=i.type==='password'?EYE_OPEN_SVG:EYE_CLOSED_SVG;})(this)" title="Toggle visibility">${'<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" class="backup-eye-icon"><path d="M1 12s4-8 11-8 11 8 11 8-4 8-11 8-11-8-11-8z"/><circle cx="12" cy="12" r="3"/></svg>'}</button>
            </div>
            <div id="backup-vault-hint" class="backup-vault-hint is-warning">${t('config.backup.no_password_warning')}</div>
        </div>

        <div class="backup-actions-row">
            <button class="btn-save backup-primary-btn" onclick="backupCreate()" id="backup-create-btn">
                ${t('config.backup.create_download')}
            </button>
            <span id="backup-create-status" class="backup-status-text is-muted"></span>
        </div>
    </div>

    <!-- Import panel -->
    <div class="wh-panel backup-panel" id="backup-panel-import">
        <div class="backup-warning-box">
            ⚠️ ${t('config.backup.import_warning')}
        </div>

        <div class="field-group backup-field-group-file">
            <div class="field-label">${t('config.backup.select_file')}</div>
            <input type="file" id="backup-import-file" accept=".ago" class="field-input backup-file-input">
        </div>

        <div class="field-group backup-field-group">
            <div class="field-label">${t('config.backup.import_password_label')}</div>
            <div class="password-wrap">
                <input type="password" id="backup-import-password" class="field-input" placeholder="${t('config.backup.decryption_placeholder')}" autocomplete="off">
                <button type="button" class="password-toggle" onclick="(function(b){var i=document.getElementById('backup-import-password');i.type=i.type==='password'?'text':'password';b.innerHTML=i.type==='password'?EYE_OPEN_SVG:EYE_CLOSED_SVG;})(this)" title="Toggle visibility">${'<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" class="backup-eye-icon"><path d="M1 12s4-8 11-8 11 8 11 8-4 8-11 8-11-8-11-8z"/><circle cx="12" cy="12" r="3"/></svg>'}</button>
            </div>
        </div>

        <div class="backup-actions-row">
            <button class="btn-save backup-import-btn" onclick="backupImport()" id="backup-import-btn">
                ${t('config.backup.import_button')}
            </button>
            <span id="backup-import-status" class="backup-status-text is-muted"></span>
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

async function backupCreate() {
    const btn = document.getElementById('backup-create-btn');
    const status = document.getElementById('backup-create-status');
    btn.disabled = true;
    backupSetStatus(status, 'is-muted', t('config.backup.creating'));

    const payload = {
        password: document.getElementById('backup-password').value || '',
        include_vectordb: document.getElementById('backup-opt-vectordb').checked,
        include_workdir: document.getElementById('backup-opt-workdir').checked,
    };

    try {
        const resp = await fetch('/api/backup/create', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify(payload)
        });
        if (!resp.ok) {
            const txt = await resp.text();
            throw new Error(txt);
        }
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
        backupSetStatus(status, 'is-success', '✅ ' + t('config.backup.downloaded') + filename);
    } catch (e) {
        backupSetStatus(status, 'is-danger', '❌ ' + e.message);
    } finally {
        btn.disabled = false;
    }
}

async function backupImport() {
    const btn = document.getElementById('backup-import-btn');
    const status = document.getElementById('backup-import-status');
    const result = document.getElementById('backup-import-result');
    const fileInput = document.getElementById('backup-import-file');

    if (!fileInput.files || !fileInput.files[0]) {
        backupSetStatus(status, 'is-warning', t('config.backup.select_file_warning'));
        return;
    }

    btn.disabled = true;
    backupSetStatus(status, 'is-muted', t('config.backup.importing'));
    result.classList.add('is-hidden');

    const fd = new FormData();
    fd.append('file', fileInput.files[0]);
    fd.append('password', document.getElementById('backup-import-password').value || '');

    try {
        const resp = await fetch('/api/backup/import', { method: 'POST', body: fd });
        const data = await resp.json();
        if (!resp.ok || data.error) {
            let msg = data.message || data.error || 'Unknown error';
            if (data.error === 'password_required') msg = t('config.backup.password_required');
            if (data.error === 'decryption_failed') msg = t('config.backup.decryption_failed');
            backupSetStatus(status, 'is-danger', '❌ ' + msg);
        } else {
            backupSetStatus(status, 'is-muted', '');
            result.classList.remove('is-hidden');
            result.innerHTML = `<div class="backup-import-success-box">
                <div class="backup-import-success-title">✅ ${t('config.backup.import_success')}</div>
                <div class="backup-import-success-message">${data.message}</div>
                <button class="btn-save backup-restart-btn" onclick="restartAuraGo()">
                    ↻ ${t('config.backup.restart_now')}
                </button>
            </div>`;
        }
    } catch (e) {
        backupSetStatus(status, 'is-danger', '❌ ' + e.message);
    } finally {
        btn.disabled = false;
    }
}
